package protocol

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// This is an architectural guard, not a behavioral test.
//
// The sanitization bug that motivated the protocol builders survived for so long
// because of a structural problem rather than a logic error: alongside the safe
// builders in this package, a large set of hand-rolled response builders in
// internal/handlers and internal/server wrote SIP2 fields by raw formatting,
// bypassing helpers.SanitizeFieldValue entirely. Control characters (the real
// reported case was a call number "741.5 KHAN\n741.5 KHAN") break SIP2's
// line-oriented framing and crash self-check clients, and a literal field
// delimiter inside a value injects a spurious field separator that silently
// shifts every following field.
//
// Converting those 45 call sites fixed the instances. This test fixes the class:
// it scans every non-test Go file in the repository and fails if a new raw field
// write appears anywhere. It lives in internal/sip2/protocol rather than
// internal/handlers for three reasons:
//
//  1. The policy it enforces belongs to this package. BuildPrefixedField and its
//     siblings ARE the policy; the guard is the compile-time-ish counterpart of
//     their doc comments, and a reader who opens delimiter.go to ask "may I
//     format a field by hand?" finds the answer in the same directory.
//  2. Its scope is the whole repository, not the handlers. internal/server
//     /connection.go was the second offender, and the third one could appear in
//     any package. Filing the guard under internal/handlers would wrongly imply
//     handlers are the only place that matters.
//  3. internal/handlers already owns the complementary test,
//     raw_field_sanitization_test.go, which drives real handler paths with dirty
//     FOLIO data and asserts on the built wire message. That test proves the
//     current call sites behave; this one proves no new call site can appear.
//     Keeping behavior and structure in separate packages keeps both readable.

// sipFieldCode matches a field delimiter followed by a two-character SIP2 field
// code, e.g. the "|AE" in "...|AE%s|AF%s". Used to split a format string into the
// per-field segments that each code owns.
var sipFieldCode = regexp.MustCompile(`\|[A-Z0-9]{2}`)

// formatVerb matches a real fmt verb, e.g. %s, %d, %.2f, %-10s, %q.
//
// The space flag (as in "% d") is deliberately excluded from the flag class, and
// escaped "%%" is stripped before this is applied (see hasFormatVerb). Both
// guard against the same false positive: ordinary SIP2 screen-message prose that
// happens to contain a percent sign. Without those two measures "|AE100%%
// complete" scans as the verb "% c" and "|AF50% off" as "% o", so a legitimate
// literal-only screen message would be reported as a raw field write. The cost
// is a blind spot for the space-flag verbs alone, which no call site here uses
// and which would in practice be accompanied by an ordinary verb anyway.
var formatVerb = regexp.MustCompile(`%[-+#0'\[\]*.0-9]*[a-zA-Z]`)

// hasFormatVerb reports whether s interpolates a value. Escaped "%%" is removed
// first, exactly as fmt treats it: a literal percent sign, not a verb.
func hasFormatVerb(s string) bool {
	return formatVerb.MatchString(strings.ReplaceAll(s, "%%", ""))
}

// rawFieldCodePrefix matches a string literal that BEGINS with a delimiter plus a
// two-character field code, e.g. "|AO" or "|AFError: ". Used for the
// concatenation form: such a literal is harmless on its own and only becomes a
// raw field write when a non-literal operand is concatenated onto it. Matching on
// the prefix rather than the whole literal means "|AFError: " + err.Error() is
// caught as well as the bare "|AO" + institutionID.
var rawFieldCodePrefix = regexp.MustCompile(`^\|[A-Z0-9]{2}`)

// rawSprintfFieldWrites returns one entry per raw field write in a printf format
// string: the segment a field code owns, from the code up to the next field code
// or the end of the string, for every segment that contains a format verb.
//
// Splitting at field-code boundaries rather than demanding the verb sit flush
// against the code is what separates a raw write from a literal-only write
// without leaving a gap. "|AE%s" and "|AFError: %s" both interpolate external
// data into a field and are both reported; "|AFSession ended successfully" and
// "|BLY" interpolate nothing, so there is no value to sanitize and they are not.
// Text before the first field code (the fixed-length header, e.g. the "30NN  %s"
// in "30NN  %s|AO%s") is positional rather than delimiter-framed and is ignored.
func rawSprintfFieldWrites(format string) []string {
	bounds := sipFieldCode.FindAllStringIndex(format, -1)

	var out []string
	for i, b := range bounds {
		end := len(format)
		if i+1 < len(bounds) {
			end = bounds[i+1][0]
		}
		segment := format[b[0]:end]
		if hasFormatVerb(segment) {
			out = append(out, segment)
		}
	}
	return out
}

// approvedRawFieldWrite lists the raw writes that are allowed to remain.
//
// The key is file + the exact matched text, never a whole file: a NEW raw write
// in fee_paid.go or base.go is still a failure. Each entry is a float formatted
// with %.2f, and strconv.FormatFloat can only ever emit digits, '.', '-', '+',
// 'e', 'E', or the words Inf/NaN. None of those is a control character and none
// is the field delimiter, so these three cannot carry the payloads this guard
// exists to stop. Converting them would mean formatting the float first and
// passing the string to BuildPrefixedField, which adds a sanitize pass that is
// provably a no-op.
var approvedRawFieldWrite = map[string]string{
	"internal/handlers/fee_paid.go\x00|FA%.2f": "FA (amount still owed) is a %.2f of a float64 — cannot carry a control character or a delimiter",
	"internal/handlers/fee_paid.go\x00|FG%.2f": "FG (amount applied) is a %.2f of a float64 — cannot carry a control character or a delimiter",
	"internal/handlers/base.go\x00|BV%.2f":     "BV (fee amount) is a %.2f of a float64 — cannot carry a control character or a delimiter",
}

// trackedUnconvertedRawFieldWrite is NOT an approval list. It records raw field
// writes that this guard has found, that ARE genuine bugs, and that are awaiting
// conversion by the owner of the surrounding code.
//
// The distinction from approvedRawFieldWrite matters and the two must not be
// merged: entries above are provably safe and permanent, entries here are known
// defects held open so the guard can be landed without masking them. Every run
// logs them (see the t.Log below) so they cannot quietly become the status quo.
//
// Remove an entry — do not extend this list — when the site is converted to
// protocol.BuildPrefixedField. Adding a new entry here is never the right fix
// for new code.
//
// The list is currently EMPTY, and that is the goal state: every raw field write
// in the repository is either converted or provably safe (see
// approvedRawFieldWrite). The map and its staleness check are kept so a future
// find can be held open the same way rather than silently approved.
//
// The three entries that used to live here shared one shape, and it is the shape
// the original conversion sweep missed: the last-resort fallback taken when the
// ResponseBuilder itself returns an error (renew.go buildRenewResponseWithError
// and buildRenewResponse, renew_all.go buildErrorResponseWithMessage). Each
// interpolated untrusted client-supplied identifiers and wrapped error text,
// which is precisely the data least worth trusting, and each was overlooked
// because its format string does not BEGIN with the delimiter — the field codes
// sit mid-string after the fixed-length header, so a grep anchored on `"|` never
// saw them. That is the hole this guard closes. All three now build their
// data-carrying fields with protocol.BuildPrefixedField; their fixed-length
// headers and positional timestamps are emitted verbatim, as they must be.
var trackedUnconvertedRawFieldWrite = map[string]string{}

// skippedDirs hold no production wire path.
//
// "tests" is skipped for the same reason _test.go files are, and the reason is
// worth stating because the directory name alone does not make it obvious:
// tests/testutil/sip2.go is ordinary (non-_test.go) source, yet every raw field
// write in it builds a SIP2 *request* fixture that a fake self-check unit sends
// INTO the server (e.g. fmt.Sprintf("9300|CN%s|CO%s\r", user, pass)). Those are
// inputs to the parser, not outgoing responses, and sanitizing them would defeat
// their purpose — several tests exist precisely to feed dirty values in. The
// guard's subject is the outbound serialization boundary, so test fixtures are
// out of scope while every package that can answer a client stays in scope.
var skippedDirs = map[string]bool{
	".git":         true,
	".github":      true,
	"vendor":       true,
	"node_modules": true,
	"bin":          true,
	"dist":         true,
	"tests":        true,
}

type rawFieldFinding struct {
	file    string // repo-relative, forward slashes
	line    int
	matched string // the offending literal text, e.g. "|AO%s" or "|AO"
	form    string // human-readable description of which bad pattern matched
	context string // the surrounding source expression, trimmed
}

func (f rawFieldFinding) key() string {
	return f.file + "\x00" + f.matched
}

// TestNoRawSIP2FieldWrites fails if any Go file in the repository formats a SIP2
// variable field by hand instead of going through the protocol builders.
func TestNoRawSIP2FieldWrites(t *testing.T) {
	root := repoRoot(t)

	var findings []rawFieldFinding

	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if skippedDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		// Only first-party Go source. _test.go files are skipped: tests
		// legitimately construct raw wire messages as fixtures and expectations.
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		rel = filepath.ToSlash(rel)

		findings = append(findings, scanGoFile(t, path, rel)...)
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", root, err)
	}

	// Separate the findings into the three buckets.
	var unapproved []rawFieldFinding
	var tracked []rawFieldFinding
	seenApproved := make(map[string]bool)
	seenTracked := make(map[string]bool)

	for _, f := range findings {
		switch {
		case approvedRawFieldWrite[f.key()] != "":
			seenApproved[f.key()] = true
		case trackedUnconvertedRawFieldWrite[f.key()] != "":
			seenTracked[f.key()] = true
			tracked = append(tracked, f)
		default:
			unapproved = append(unapproved, f)
		}
	}

	if len(unapproved) > 0 {
		sort.Slice(unapproved, func(i, j int) bool {
			if unapproved[i].file != unapproved[j].file {
				return unapproved[i].file < unapproved[j].file
			}
			return unapproved[i].line < unapproved[j].line
		})

		var b strings.Builder
		fmt.Fprintf(&b, "found %d raw SIP2 field write(s) that bypass sanitization:\n\n", len(unapproved))
		for _, f := range unapproved {
			fmt.Fprintf(&b, "  %s:%d: %s\n", f.file, f.line, f.form)
			fmt.Fprintf(&b, "      offending text: %s\n", f.matched)
			if f.context != "" {
				fmt.Fprintf(&b, "      in: %s\n", f.context)
			}
			fmt.Fprintln(&b)
		}
		b.WriteString("A raw field write sends the value straight to the wire, so an embedded\n")
		b.WriteString("control character breaks SIP2 message framing (and crashes self-check\n")
		b.WriteString("clients), and an embedded field delimiter injects a spurious field\n")
		b.WriteString("separator that shifts every following field.\n\n")
		b.WriteString("Use protocol.BuildPrefixedField instead, which sanitizes the value:\n\n")
		b.WriteString("    -   response += fmt.Sprintf(\"|AE%s\", personalName)\n")
		b.WriteString("    -   content  += \"|AE\" + personalName\n")
		b.WriteString("    +   response += protocol.BuildPrefixedField(\"AE\", personalName, \"|\")\n\n")
		b.WriteString("BuildPrefixedField emits delimiter-then-code and always emits, matching\n")
		b.WriteString("what these raw writes do; see internal/sip2/protocol/delimiter.go.\n\n")
		b.WriteString("If the value provably cannot carry a control character or a delimiter\n")
		b.WriteString("(e.g. a %.2f of a float64), add it to approvedRawFieldWrite in this file\n")
		b.WriteString("with the reason — keyed on file + exact format string, never a whole file.")

		t.Error(b.String())
	}

	// A stale approval is itself a defect: it means the allowlist is describing
	// code that no longer exists, so it is no longer protecting anything.
	for key, reason := range approvedRawFieldWrite {
		if !seenApproved[key] {
			file, matched, _ := strings.Cut(key, "\x00")
			t.Errorf("approvedRawFieldWrite entry is stale: %s no longer contains %q (%s). Remove the entry.",
				file, matched, reason)
		}
	}

	// A stale tracking entry is a defect for the mirror-image reason: it claims a
	// known bug still exists at a site that no longer has one, so the next reader
	// cannot tell the open defects from the fixed ones.
	for key, reason := range trackedUnconvertedRawFieldWrite {
		if !seenTracked[key] {
			file, matched, _ := strings.Cut(key, "\x00")
			t.Errorf("trackedUnconvertedRawFieldWrite entry is stale: %s no longer contains %q (%s). Remove the entry.",
				file, matched, reason)
		}
	}

	// Surface the known-defect list on every run so it stays visible.
	if len(tracked) > 0 {
		sort.Slice(tracked, func(i, j int) bool { return tracked[i].line < tracked[j].line })
		var b strings.Builder
		fmt.Fprintf(&b, "%d raw SIP2 field write(s) are KNOWN DEFECTS awaiting conversion "+
			"(tracked, not approved):\n", len(tracked))
		for _, f := range tracked {
			fmt.Fprintf(&b, "  %s:%d: %s — %s\n",
				f.file, f.line, f.matched, trackedUnconvertedRawFieldWrite[f.key()])
		}
		b.WriteString("Convert these to protocol.BuildPrefixedField and delete their entries " +
			"from trackedUnconvertedRawFieldWrite.")
		t.Log(b.String())
	}
}

// scanGoFile parses one file and reports the raw field writes in it.
//
// Parsing with go/ast rather than scanning lines of text is what makes the
// comment problem disappear: delimiter.go's doc comment for BuildPrefixedField
// deliberately quotes both bad patterns as examples of what not to write
// (`fmt.Sprintf("|XX%s", v)` and `content += "|XX" + v`), and a textual grep
// trips on it. Comments are not expressions, so they are never visited here; the
// parser is also called WITHOUT parser.ParseComments, so they are not even
// retained in the AST. The same is true of the explanatory text in this file.
func scanGoFile(t *testing.T, path, rel string) []rawFieldFinding {
	t.Helper()

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		t.Fatalf("parsing %s: %v", rel, err)
	}

	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", rel, err)
	}
	lines := strings.Split(strings.ReplaceAll(string(src), "\r\n", "\n"), "\n")

	lineText := func(pos token.Pos) (int, string) {
		n := fset.Position(pos).Line
		if n >= 1 && n <= len(lines) {
			return n, strings.TrimSpace(lines[n-1])
		}
		return n, ""
	}

	seen := make(map[string]bool)
	var out []rawFieldFinding

	add := func(pos token.Pos, matched, form string) {
		line, text := lineText(pos)
		f := rawFieldFinding{file: rel, line: line, matched: matched, form: form, context: text}
		dedupe := fmt.Sprintf("%d\x00%s", line, matched)
		if seen[dedupe] {
			return
		}
		seen[dedupe] = true
		out = append(out, f)
	}

	ast.Inspect(file, func(n ast.Node) bool {
		switch node := n.(type) {

		// Pattern 1: the fmt.Sprintf form used throughout internal/handlers,
		// e.g. fmt.Sprintf("|AE%s", personalName). Any Sprintf-like call is
		// checked, so Fprintf/Appendf variants are covered too.
		case *ast.CallExpr:
			if !isPrintfLikeCall(node) || len(node.Args) == 0 {
				return true
			}
			format, ok := stringLit(node.Args[0])
			if !ok {
				return true
			}
			for _, m := range rawSprintfFieldWrites(format) {
				add(node.Args[0].Pos(), m,
					`fmt.Sprintf-style raw field write: a format string interpolates a value into a "|XX" field`)
			}

		// Pattern 2: the plain-concatenation form used in
		// internal/server/connection.go, e.g. content += "|AE" + personalName.
		// The whole + chain is flattened so that a long expression such as
		// a + "|AO" + x + "|AA" + y is checked pairwise: a "|XX" literal is a
		// violation exactly when the operand concatenated onto it is not itself
		// a string literal. That is the distinction between interpolating
		// external data and writing a constant like "|BLY".
		case *ast.BinaryExpr:
			if node.Op != token.ADD {
				return true
			}
			operands := flattenConcat(node)
			for i, op := range operands {
				lit, ok := stringLit(op)
				if !ok || !rawFieldCodePrefix.MatchString(lit) {
					continue
				}
				if i+1 >= len(operands) {
					continue // trailing constant; nothing is interpolated
				}
				if _, nextIsLiteral := stringLit(operands[i+1]); nextIsLiteral {
					continue // "|AF" + "literal text" is still constant
				}
				add(op.Pos(), lit,
					`concatenation raw field write: a variable is concatenated directly onto a "|XX" field code literal`)
			}
		}
		return true
	})

	return out
}

// flattenConcat turns a left-associative chain of string concatenations into the
// ordered list of its operands, so adjacency can be inspected.
func flattenConcat(e ast.Expr) []ast.Expr {
	if p, ok := e.(*ast.ParenExpr); ok {
		return flattenConcat(p.X)
	}
	be, ok := e.(*ast.BinaryExpr)
	if !ok || be.Op != token.ADD {
		return []ast.Expr{e}
	}
	return append(flattenConcat(be.X), flattenConcat(be.Y)...)
}

// stringLit reports whether e is an untyped string literal and returns its value.
// Anything else — an identifier, a call, a selector — counts as interpolated data.
func stringLit(e ast.Expr) (string, bool) {
	if p, ok := e.(*ast.ParenExpr); ok {
		return stringLit(p.X)
	}
	lit, ok := e.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	v, err := strconv.Unquote(lit.Value)
	if err != nil {
		return "", false
	}
	return v, true
}

// isPrintfLikeCall reports whether the call is a *printf whose first argument is
// the format string (Sprintf, Printf, Appendf) or has the format string first
// after a writer (Fprintf) — in the latter case the check below simply does not
// match, which is harmless.
func isPrintfLikeCall(call *ast.CallExpr) bool {
	switch fun := call.Fun.(type) {
	case *ast.SelectorExpr:
		return strings.HasSuffix(fun.Sel.Name, "printf") || strings.HasSuffix(fun.Sel.Name, "Sprintf") ||
			strings.HasSuffix(fun.Sel.Name, "Printf") || strings.HasSuffix(fun.Sel.Name, "Appendf")
	case *ast.Ident:
		return strings.HasSuffix(fun.Name, "printf") || strings.HasSuffix(fun.Name, "Sprintf")
	}
	return false
}

// repoRoot walks up from the test's working directory to the directory holding
// go.mod, so the guard scans the whole module no matter which package it is in.
func repoRoot(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("could not locate go.mod above %s", dir)
		}
		dir = parent
	}
}
