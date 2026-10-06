package protocol

import (
	"reflect"
	"strings"
	"testing"
)

func TestConvertDelimiter(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"CR escape", "\\r", "\r"},
		{"LF escape", "\\n", "\n"},
		{"CRLF escape", "\\r\\n", "\r\n"},
		{"tab escape", "\\t", "\t"},
		{"pipe (no conversion)", "|", "|"},
		{"empty string", "", ""},
		{"no escapes", "hello", "hello"},
		{"mixed", "\\r\\n|", "\r\n|"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ConvertDelimiter(tt.input)
			if got != tt.want {
				t.Errorf("ConvertDelimiter(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestGetMessageDelimiterBytes(t *testing.T) {
	tests := []struct {
		input string
		want  []byte
	}{
		{"\\r", []byte("\r")},
		{"\\n", []byte("\n")},
		{"\\r\\n", []byte("\r\n")},
		{"|", []byte("|")},
		{"", []byte("")},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := GetMessageDelimiterBytes(tt.input)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetMessageDelimiterBytes(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestGetFieldDelimiterBytes(t *testing.T) {
	tests := []struct {
		input string
		want  []byte
	}{
		{"|", []byte("|")},
		{"^", []byte("^")},
		{"", []byte("")},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := GetFieldDelimiterBytes(tt.input)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetFieldDelimiterBytes(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestSplitFields(t *testing.T) {
	tests := []struct {
		name      string
		message   string
		delimiter string
		want      []string
	}{
		{
			name:      "pipe delimiter",
			message:   "AA1234|AB5678|",
			delimiter: "|",
			want:      []string{"AA1234", "AB5678", ""},
		},
		{
			name:      "no delimiter found",
			message:   "AA1234",
			delimiter: "|",
			want:      []string{"AA1234"},
		},
		{
			name:      "empty message",
			message:   "",
			delimiter: "|",
			want:      []string{""},
		},
		{
			name:      "caret delimiter",
			message:   "AA1234^AB5678",
			delimiter: "^",
			want:      []string{"AA1234", "AB5678"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SplitFields(tt.message, tt.delimiter)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("SplitFields() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseFields(t *testing.T) {
	tests := []struct {
		name      string
		message   string
		delimiter string
		want      map[string]string
	}{
		{
			name:      "basic fields",
			message:   "AA1234567|AB9876543|",
			delimiter: "|",
			want:      map[string]string{"AA": "1234567", "AB": "9876543"},
		},
		{
			name:      "empty value field",
			message:   "AA|",
			delimiter: "|",
			want:      map[string]string{"AA": ""},
		},
		{
			name:      "single char part skipped",
			message:   "A|AA123|",
			delimiter: "|",
			want:      map[string]string{"AA": "123"},
		},
		{
			name:      "empty message",
			message:   "",
			delimiter: "|",
			want:      map[string]string{},
		},
		{
			name:      "multiple fields",
			message:   "AA123|BV5.00|CW20240315|",
			delimiter: "|",
			want:      map[string]string{"AA": "123", "BV": "5.00", "CW": "20240315"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseFields(tt.message, tt.delimiter)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseFields() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseMultiValueFields(t *testing.T) {
	tests := []struct {
		name      string
		message   string
		delimiter string
		want      map[string][]string
	}{
		{
			name:      "single value per field",
			message:   "AA1234|AB5678|",
			delimiter: "|",
			want:      map[string][]string{"AA": {"1234"}, "AB": {"5678"}},
		},
		{
			name:      "duplicate field codes",
			message:   "AS001|AS002|AS003|",
			delimiter: "|",
			want:      map[string][]string{"AS": {"001", "002", "003"}},
		},
		{
			name:      "empty message",
			message:   "",
			delimiter: "|",
			want:      map[string][]string{},
		},
		{
			name:      "single char parts skipped",
			message:   "A|AA123|",
			delimiter: "|",
			want:      map[string][]string{"AA": {"123"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseMultiValueFields(tt.message, tt.delimiter)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseMultiValueFields() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetField(t *testing.T) {
	fields := map[string]string{
		"AA": "1234567",
		"AB": "9876543",
	}

	tests := []struct {
		code string
		want string
	}{
		{"AA", "1234567"},
		{"AB", "9876543"},
		{"XX", ""},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			got := GetField(fields, tt.code)
			if got != tt.want {
				t.Errorf("GetField(%q) = %q, want %q", tt.code, got, tt.want)
			}
		})
	}
}

func TestGetMultiValueField(t *testing.T) {
	fields := map[string][]string{
		"AS": {"001", "002", "003"},
		"AA": {"123"},
	}

	tests := []struct {
		code string
		want []string
	}{
		{"AS", []string{"001", "002", "003"}},
		{"AA", []string{"123"}},
		{"XX", []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			got := GetMultiValueField(fields, tt.code)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetMultiValueField(%q) = %v, want %v", tt.code, got, tt.want)
			}
		})
	}
}

func TestBuildField(t *testing.T) {
	tests := []struct {
		name      string
		fieldCode string
		value     string
		delimiter string
		want      string
	}{
		{"normal field", "AA", "1234567", "|", "AA1234567|"},
		{"empty value returns empty", "AA", "", "|", ""},
		{"custom delimiter", "BV", "5.00", "^", "BV5.00^"},
		// Sanitization: FOLIO call numbers can carry embedded control characters,
		// which would break SIP2's line-oriented framing.
		{"embedded newline collapses to space", "AB", "741.5 KHAN\n741.5 KHAN", "|", "AB741.5 KHAN 741.5 KHAN|"},
		{"leading newline trimmed", "AB", "\n741.5 KHAN", "|", "AB741.5 KHAN|"},
		{"trailing newline trimmed", "AB", "741.5 KHAN\n", "|", "AB741.5 KHAN|"},
		{"embedded CRLF collapses to space", "AB", "part one\r\npart two", "|", "ABpart one part two|"},
		{"control run plus adjacent spaces collapses", "AB", "a \n b", "|", "ABa b|"},
		// Regression guard: sanitizing must happen BEFORE the empty check, so a
		// value that is nothing but control characters omits the field entirely
		// instead of emitting a raw control character into the message.
		{"only LF omits field", "AB", "\n", "|", ""},
		{"only CRLF omits field", "AB", "\r\n", "|", ""},
		{"only NUL/US/DEL omits field", "AB", "\x00\x1f\x7f", "|", ""},
		{"only control chars and spaces omits field", "AB", " \n\t ", "|", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildField(tt.fieldCode, tt.value, tt.delimiter)
			if got != tt.want {
				t.Errorf("BuildField(%q, %q, %q) = %q, want %q", tt.fieldCode, tt.value, tt.delimiter, got, tt.want)
			}
		})
	}
}

func TestBuildFixedField(t *testing.T) {
	tests := []struct {
		name   string
		value  string
		length int
		want   string
	}{
		{"exact length", "Hello", 5, "Hello"},
		{"shorter — pads with spaces", "Hi", 5, "Hi   "},
		{"longer — truncates", "Hello World", 5, "Hello"},
		{"empty — all spaces", "", 3, "   "},
		// Sanitization must be strict 1:1 (SanitizeFixedWidthString): each
		// control char becomes exactly one space so no character position shifts.
		{"embedded newline becomes one space", "AB\nCD", 5, "AB CD"},
		{"embedded newline then truncates", "AB\nCDEF", 5, "AB CD"},
		{"patron-status pattern not shifted", "Y \x00Y", 4, "Y  Y"},
		{"patron-status pattern padded not shifted", "Y\x00 Y", 14, "Y  Y          "},
		// No trimming: real leading/trailing spaces are semantically meaningful.
		{"leading spaces preserved", "  Y", 5, "  Y  "},
		{"leading spaces with control char preserved", "  \nY", 5, "   Y "},
		{"all control chars become spaces", "\n\r\t", 3, "   "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildFixedField(tt.value, tt.length)
			if got != tt.want {
				t.Errorf("BuildFixedField(%q, %d) = %q, want %q", tt.value, tt.length, got, tt.want)
			}
			if len(got) != tt.length {
				t.Errorf("BuildFixedField(%q, %d) length = %d, want %d", tt.value, tt.length, len(got), tt.length)
			}
		})
	}
}

// TestBuildRequiredField covers the always-emit builder used for SIP2 fields
// that must be included "even if blank" (the checkin response AQ, AP, CK, CH,
// CS, CV, CT). The cases below are deliberately the behavioral opposite of
// TestBuildField for empty and control-char-only input: that one omits the
// field, this one still emits it, blank.
func TestBuildRequiredField(t *testing.T) {
	tests := []struct {
		name      string
		fieldCode string
		value     string
		delimiter string
		want      string
	}{
		{"with value", "CS", "741.5 KHAN", "|", "CS741.5 KHAN|"},
		// Sanitization: control chars must never reach the wire.
		{"interior newline collapses to space", "CS", "741.5 KHAN\n741.5 KHAN", "|", "CS741.5 KHAN 741.5 KHAN|"},
		{"leading newline yields no leading space", "CS", "\n741.5 KHAN", "|", "CS741.5 KHAN|"},
		{"trailing CRLF yields no trailing space", "CS", "741.5 KHAN\r\n", "|", "CS741.5 KHAN|"},
		{"mixed control chars", "CS", "a\rb\nc", "|", "CSa b c|"},
		// The whole point of this builder: the field stays PRESENT even when
		// the value is (or sanitizes to) empty. BuildField would return "".
		{"empty value still emits field", "CS", "", "|", "CS|"},
		{"only LF still emits field", "CS", "\n", "|", "CS|"},
		{"only CRLF still emits field", "CS", "\r\n", "|", "CS|"},
		{"only control chars still emits field", "CS", "\n\n\x01\x02", "|", "CS|"},
		{"NUL unit-sep DEL still emits field", "CS", "\x00\x1f\x7f", "|", "CS|"},
		// Identity invariant: clean values pass through byte-identically,
		// including pre-existing edge whitespace.
		{"clean value byte-identical", "CT", "Reference Desk", "|", "CTReference Desk|"},
		{"clean value with edge spaces preserved", "CT", "  Main  Library  ", "|", "CT  Main  Library  |"},
		{"clean punctuation byte-identical", "CS", "PS3525.A84 G7 2004", "|", "CSPS3525.A84 G7 2004|"},
		{"alternate delimiter honored", "CV", "01", "\x7c", "CV01\x7c"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildRequiredField(tt.fieldCode, tt.value, tt.delimiter)
			if got != tt.want {
				t.Errorf("BuildRequiredField(%q, %q, %q) = %q, want %q", tt.fieldCode, tt.value, tt.delimiter, got, tt.want)
			}
			// Unconditional structural guarantee: the field is always present.
			if got == "" {
				t.Errorf("BuildRequiredField(%q, %q, %q) returned empty; this builder must always emit the field", tt.fieldCode, tt.value, tt.delimiter)
			}
			if !strings.HasPrefix(got, tt.fieldCode) {
				t.Errorf("BuildRequiredField(%q, %q, %q) = %q, want prefix %q", tt.fieldCode, tt.value, tt.delimiter, got, tt.fieldCode)
			}
			if !strings.HasSuffix(got, tt.delimiter) {
				t.Errorf("BuildRequiredField(%q, %q, %q) = %q, want suffix %q", tt.fieldCode, tt.value, tt.delimiter, got, tt.delimiter)
			}
		})
	}
}

// TestBuildRequiredFieldVsBuildField pins the contract difference between the
// two builders so nobody later "simplifies" them into one.
func TestBuildRequiredFieldVsBuildField(t *testing.T) {
	for _, value := range []string{"", "\n", "\r\n", "\n\n\x01\x02", " \n\t "} {
		t.Run(value, func(t *testing.T) {
			if got := BuildField("CS", value, "|"); got != "" {
				t.Errorf("BuildField(%q) = %q, want \"\" (omits when empty)", value, got)
			}
			if got := BuildRequiredField("CS", value, "|"); got != "CS|" {
				t.Errorf("BuildRequiredField(%q) = %q, want %q (always emits)", value, got, "CS|")
			}
		})
	}
}

// TestBuiltFieldsNeverContainLineBreaks is the framing guard: SIP2 is
// line-oriented, so no built field may ever carry a bare CR or LF through to
// the wire regardless of what FOLIO returned.
func TestBuiltFieldsNeverContainLineBreaks(t *testing.T) {
	dirty := []string{
		"741.5 KHAN\n741.5 KHAN",
		"\n741.5 KHAN",
		"741.5 KHAN\r\n",
		"a\rb\nc",
		"\n",
		"\r\n",
		"\x00\x1f\x7f",
	}

	for _, value := range dirty {
		t.Run(value, func(t *testing.T) {
			for name, got := range map[string]string{
				"BuildField":         BuildField("AB", value, "|"),
				"BuildRequiredField": BuildRequiredField("AB", value, "|"),
				"BuildFixedField":    BuildFixedField(value, 24),
				"BuildPrefixedField": BuildPrefixedField("AB", value, "|"),
			} {
				if strings.ContainsAny(got, "\r\n") {
					t.Errorf("%s(%q) = %q, contains CR or LF", name, value, got)
				}
			}
		})
	}
}

// TestBuildPrefixedField covers the delimiter-FIRST shape used by the
// hand-rolled response builders in internal/handlers and internal/server.
func TestBuildPrefixedField(t *testing.T) {
	testCases := []struct {
		name      string
		fieldCode string
		value     string
		delimiter string
		want      string
	}{
		// Output shape: delimiter, then code, then value - the opposite of
		// BuildField's code + value + delimiter.
		{"simple value", "AE", "John Doe", "|", "|AEJohn Doe"},
		{"numeric value", "BV", "0.00", "|", "|BV0.00"},

		// ALWAYS emits, which is the behavioural opposite of BuildField.
		{"empty value still emits", "AE", "", "|", "|AE"},
		{"whitespace-only value still emits", "AE", " ", "|", "|AE "},
		{"control-chars-only value still emits", "AE", "\n\r", "|", "|AE"},
		{"NUL and DEL only still emits", "AE", "\x00\x7f", "|", "|AE"},

		// Embedded delimiter is neutralized, so no spurious field separator is
		// injected into the value.
		{"embedded delimiter neutralized", "AJ", "741.5|KHAN", "|", "|AJ741.5 KHAN"},
		{"multiple embedded delimiters", "AJ", "A|B|C", "|", "|AJA B C"},
		{"multi-char delimiter neutralized", "AE", "A||B", "||", "||AEA B"},

		// Control characters still follow SanitizeString's policy.
		{"interior newline collapsed", "AJ", "741.5 KHAN\n741.5 KHAN", "|", "|AJ741.5 KHAN 741.5 KHAN"},
		{"delimiter adjacent to newline", "AJ", "741.5|\nKHAN", "|", "|AJ741.5 KHAN"},

		// Clean values pass through byte-identical.
		{"clean value byte-identical", "AF", "Café résumé", "|", "|AFCafé résumé"},
		{"clean value with interior double space", "AF", "A  B", "|", "|AFA  B"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := BuildPrefixedField(tc.fieldCode, tc.value, tc.delimiter); got != tc.want {
				t.Errorf("BuildPrefixedField(%q, %q, %q) = %q, want %q",
					tc.fieldCode, tc.value, tc.delimiter, got, tc.want)
			}
		})
	}
}

// TestBuildPrefixedFieldVsBuildField pins the emit-policy contrast: for values
// that sanitize to empty, BuildField omits the field entirely while
// BuildPrefixedField still emits the delimiter and code. Field presence at
// BuildPrefixedField's call sites is governed by the surrounding if statements,
// not by the builder.
func TestBuildPrefixedFieldVsBuildField(t *testing.T) {
	for _, value := range []string{"", "\n", "\r\n", "\n\n\x01\x02", " \n\t "} {
		t.Run(value, func(t *testing.T) {
			if got := BuildField("AE", value, "|"); got != "" {
				t.Errorf("BuildField(%q) = %q, want \"\" (omits when empty)", value, got)
			}
			if got := BuildPrefixedField("AE", value, "|"); got != "|AE" {
				t.Errorf("BuildPrefixedField(%q) = %q, want %q (always emits)", value, got, "|AE")
			}
		})
	}
}

// TestBuiltFieldsNeutralizeEmbeddedDelimiter is the field-injection guard: the
// field delimiter is a printable character (default "|"), so it falls outside
// the control-character sanitizer and must be neutralized separately. A value
// carrying a delimiter would otherwise shift every following field.
func TestBuiltFieldsNeutralizeEmbeddedDelimiter(t *testing.T) {
	testCases := []struct {
		name  string
		value string
		delim string

		wantField    string
		wantRequired string
		wantPrefixed string
	}{
		{
			name: "call number with pipe", value: "741.5|KHAN", delim: "|",
			wantField: "AJ741.5 KHAN|", wantRequired: "AJ741.5 KHAN|", wantPrefixed: "|AJ741.5 KHAN",
		},
		{
			name: "multiple pipes", value: "A|B|C", delim: "|",
			wantField: "AJA B C|", wantRequired: "AJA B C|", wantPrefixed: "|AJA B C",
		},
		{
			name: "pipe next to newline", value: "A|\nB", delim: "|",
			wantField: "AJA B|", wantRequired: "AJA B|", wantPrefixed: "|AJA B",
		},
		{
			name: "value is only the delimiter", value: "|", delim: "|",
			wantField: "AJ |", wantRequired: "AJ |", wantPrefixed: "|AJ ",
		},
		{
			name: "multi-char delimiter", value: "A||B", delim: "||",
			wantField: "AJA B||", wantRequired: "AJA B||", wantPrefixed: "||AJA B",
		},
		{
			name: "non-pipe delimiter", value: "A^B", delim: "^",
			wantField: "AJA B^", wantRequired: "AJA B^", wantPrefixed: "^AJA B",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := BuildField("AJ", tc.value, tc.delim); got != tc.wantField {
				t.Errorf("BuildField(%q, %q) = %q, want %q", tc.value, tc.delim, got, tc.wantField)
			}
			if got := BuildRequiredField("AJ", tc.value, tc.delim); got != tc.wantRequired {
				t.Errorf("BuildRequiredField(%q, %q) = %q, want %q", tc.value, tc.delim, got, tc.wantRequired)
			}
			if got := BuildPrefixedField("AJ", tc.value, tc.delim); got != tc.wantPrefixed {
				t.Errorf("BuildPrefixedField(%q, %q) = %q, want %q", tc.value, tc.delim, got, tc.wantPrefixed)
			}

			// Each builder contributes exactly one structural delimiter.
			for name, got := range map[string]string{
				"BuildField":         BuildField("AJ", tc.value, tc.delim),
				"BuildRequiredField": BuildRequiredField("AJ", tc.value, tc.delim),
				"BuildPrefixedField": BuildPrefixedField("AJ", tc.value, tc.delim),
			} {
				if n := strings.Count(got, tc.delim); n != 1 {
					t.Errorf("%s(%q, %q) = %q, contains %d delimiters, want exactly 1",
						name, tc.value, tc.delim, got, n)
				}
			}
		})
	}
}

func TestBuildYNField(t *testing.T) {
	if got := BuildYNField(true); got != "Y" {
		t.Errorf("BuildYNField(true) = %q, want %q", got, "Y")
	}
	if got := BuildYNField(false); got != "N" {
		t.Errorf("BuildYNField(false) = %q, want %q", got, "N")
	}
}

func TestParseYNField(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"Y", true},
		{"y", true},
		{"N", false},
		{"n", false},
		{"", false},
		{"yes", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ParseYNField(tt.input)
			if got != tt.want {
				t.Errorf("ParseYNField(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
