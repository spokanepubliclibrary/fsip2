package folio

import (
	"regexp"
	"strings"
)

// cqlEscaper escapes the characters that are special inside a quoted CQL term.
// The backslash is listed first so it is escaped before any escape sequences
// are introduced (strings.Replacer is single-pass, so earlier output is never
// re-escaped).
var cqlEscaper = strings.NewReplacer(
	`\`, `\\`,
	`"`, `\"`,
	`*`, `\*`,
	`?`, `\?`,
	`^`, `\^`,
)

// cqlQuote returns v as a quoted CQL term that matches v literally.
//
// Per the CQL masking rules, inside a quoted term "*" and "?" are wildcards
// and "^" is an anchor, a double quote ends the term, and a backslash escapes
// the next character. Each of these is escaped with a backslash, and the
// result is wrapped in double quotes, so values supplied by a device (such as
// barcodes) cannot change the meaning of the query.
//
// Gate L2-V2 (2026-11-02) confirmed against FOLIO /users and /inventory/items
// that \*, \?, \^, \" and \\ inside a quoted term are treated as literals.
func cqlQuote(v string) string {
	return `"` + cqlEscaper.Replace(v) + `"`
}

// uuidPattern is copied verbatim from FOLIO's uuid.schema (raml-util), the
// schema used for IDs such as the accountIds of /accounts-bulk/pay. It accepts
// any letter case, UUID versions 1-5 and variants 8, 9, a and b, with no
// surrounding whitespace. uuid.Parse is deliberately not used: it is looser
// (any version, braces, "urn:uuid:" prefix).
var uuidPattern = regexp.MustCompile(`^[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-[1-5][a-fA-F0-9]{3}-[89abAB][a-fA-F0-9]{3}-[a-fA-F0-9]{12}$`)

// isUUID reports whether v is a UUID that FOLIO would accept. Device-supplied
// FOLIO IDs are checked with it before they reach a query or URL path.
func isUUID(v string) bool {
	return uuidPattern.MatchString(v)
}
