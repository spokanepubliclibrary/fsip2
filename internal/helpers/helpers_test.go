package helpers

import (
	"net"
	"testing"
)

// mockAddr implements net.Addr with a fixed network/address string.
// Used to drive the string-fallback branch in ExtractIPFromAddr/ExtractPortFromAddr.
type mockAddr struct {
	network string
	address string
}

func (m *mockAddr) Network() string { return m.network }
func (m *mockAddr) String() string  { return m.address }

// Tests for ip.go

func TestExtractIPFromAddr_TCP(t *testing.T) {
	addr, _ := net.ResolveTCPAddr("tcp", "192.168.1.1:8080")
	ip, err := ExtractIPFromAddr(addr)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if ip != "192.168.1.1" {
		t.Errorf("Expected IP '192.168.1.1', got '%s'", ip)
	}
}

func TestExtractIPFromAddr_Nil(t *testing.T) {
	_, err := ExtractIPFromAddr(nil)

	if err == nil {
		t.Error("Expected error for nil address")
	}
}

func TestExtractIPFromAddr_UDP(t *testing.T) {
	addr, _ := net.ResolveUDPAddr("udp", "10.0.0.5:5353")
	ip, err := ExtractIPFromAddr(addr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ip != "10.0.0.5" {
		t.Errorf("got %s, want 10.0.0.5", ip)
	}
}

func TestExtractIPFromAddr_FallbackSuccess(t *testing.T) {
	addr := &mockAddr{network: "custom", address: "172.16.0.1:7777"}
	ip, err := ExtractIPFromAddr(addr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ip != "172.16.0.1" {
		t.Errorf("got %s, want 172.16.0.1", ip)
	}
}

func TestExtractIPFromAddr_FallbackError(t *testing.T) {
	addr := &mockAddr{network: "custom", address: "no-port-here"}
	_, err := ExtractIPFromAddr(addr)
	if err == nil {
		t.Error("expected error for unparseable address")
	}
}

func TestExtractPortFromAddr_TCP(t *testing.T) {
	addr, _ := net.ResolveTCPAddr("tcp", "192.168.1.1:8080")
	port, err := ExtractPortFromAddr(addr)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if port != 8080 {
		t.Errorf("Expected port 8080, got %d", port)
	}
}

func TestExtractPortFromAddr_Nil(t *testing.T) {
	_, err := ExtractPortFromAddr(nil)

	if err == nil {
		t.Error("Expected error for nil address")
	}
}

func TestExtractPortFromAddr_UDP(t *testing.T) {
	addr, _ := net.ResolveUDPAddr("udp", "10.0.0.5:5353")
	port, err := ExtractPortFromAddr(addr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if port != 5353 {
		t.Errorf("got %d, want 5353", port)
	}
}

func TestExtractPortFromAddr_FallbackSuccess(t *testing.T) {
	addr := &mockAddr{network: "custom", address: "172.16.0.1:7777"}
	port, err := ExtractPortFromAddr(addr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if port != 7777 {
		t.Errorf("got %d, want 7777", port)
	}
}

func TestExtractPortFromAddr_FallbackError(t *testing.T) {
	addr := &mockAddr{network: "custom", address: "no-port-here"}
	_, err := ExtractPortFromAddr(addr)
	if err == nil {
		t.Error("expected error for unparseable address")
	}
}

func TestIsIPv4(t *testing.T) {
	testCases := []struct {
		ip       string
		expected bool
	}{
		{"192.168.1.1", true},
		{"127.0.0.1", true},
		{"10.0.0.1", true},
		{"2001:db8::1", false},
		{"::1", false},
		{"invalid", false},
		{"", false},
	}

	for _, tc := range testCases {
		t.Run(tc.ip, func(t *testing.T) {
			result := IsIPv4(tc.ip)
			if result != tc.expected {
				t.Errorf("IsIPv4(%s) = %v, expected %v", tc.ip, result, tc.expected)
			}
		})
	}
}

func TestIsIPv6(t *testing.T) {
	testCases := []struct {
		ip       string
		expected bool
	}{
		{"192.168.1.1", false},
		{"127.0.0.1", false},
		{"2001:db8::1", true},
		{"::1", true},
		{"fe80::1", true},
		{"invalid", false},
		{"", false},
	}

	for _, tc := range testCases {
		t.Run(tc.ip, func(t *testing.T) {
			result := IsIPv6(tc.ip)
			if result != tc.expected {
				t.Errorf("IsIPv6(%s) = %v, expected %v", tc.ip, result, tc.expected)
			}
		})
	}
}

func TestNormalizeIP(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{"127.0.0.1", "127.0.0.1"},
		{"::1", "::1"},
		{"invalid", "invalid"},
		{"", ""},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			result := NormalizeIP(tc.input)
			if result != tc.expected {
				t.Errorf("NormalizeIP(%s) = %s, expected %s", tc.input, result, tc.expected)
			}
		})
	}
}

func TestIsLocalhost(t *testing.T) {
	testCases := []struct {
		ip       string
		expected bool
	}{
		{"127.0.0.1", true},
		{"::1", true},
		{"127.0.0.2", true},
		{"192.168.1.1", false},
		{"10.0.0.1", false},
	}

	for _, tc := range testCases {
		t.Run(tc.ip, func(t *testing.T) {
			result := IsLocalhost(tc.ip)
			if result != tc.expected {
				t.Errorf("IsLocalhost(%s) = %v, expected %v", tc.ip, result, tc.expected)
			}
		})
	}
}

func TestIsPrivateIP(t *testing.T) {
	testCases := []struct {
		ip       string
		expected bool
	}{
		{"10.0.0.1", true},
		{"172.16.0.1", true},
		{"192.168.1.1", true},
		{"127.0.0.1", true},
		{"8.8.8.8", false},
		{"1.1.1.1", false},
		{"::1", true},
		{"invalid", false},
	}

	for _, tc := range testCases {
		t.Run(tc.ip, func(t *testing.T) {
			result := IsPrivateIP(tc.ip)
			if result != tc.expected {
				t.Errorf("IsPrivateIP(%s) = %v, expected %v", tc.ip, result, tc.expected)
			}
		})
	}
}

// Tests for utils.go

func TestGenerateID(t *testing.T) {
	id1 := GenerateID()
	id2 := GenerateID()

	if id1 == "" {
		t.Error("GenerateID should not return empty string")
	}

	if id1 == id2 {
		t.Error("GenerateID should generate unique IDs")
	}

	// Should be hex encoded (32 characters for 16 bytes)
	if len(id1) != 32 {
		t.Errorf("Expected ID length 32, got %d", len(id1))
	}
}

func TestTruncateString(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		maxLen   int
		expected string
	}{
		{"shorter than maxLen", "hello", 10, "hello"},
		{"plain ASCII truncation", "hello world", 5, "hello"},
		{"exact boundary ASCII", "test", 4, "test"},
		{"empty string", "", 5, ""},
		{"maxLen zero", "hello", 0, ""},
		// Same contract asserted by the handlers package: truncation lands on a
		// rune boundary, never mid-character.
		{"multi-byte truncated on rune boundary", "Café résumé naïve 日本語 中文", 20, "Café résumé naïve 日本"},
		// 8 runes but 24 bytes: under maxLen runes, over maxLen bytes, so it must
		// come back intact rather than being sliced by byte offset.
		{"under maxLen runes but over maxLen bytes", "日本語 中文 한국어", 10, "日本語 中文 한국어"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := TruncateString(tc.input, tc.maxLen)
			if result != tc.expected {
				t.Errorf("TruncateString(%q, %d) = %q, expected %q", tc.input, tc.maxLen, result, tc.expected)
			}
			if len([]rune(result)) > tc.maxLen {
				t.Errorf("TruncateString(%q, %d) returned %d runes, exceeds maxLen", tc.input, tc.maxLen, len([]rune(result)))
			}
		})
	}
}

func TestPadRight(t *testing.T) {
	testCases := []struct {
		input    string
		length   int
		expected string
	}{
		{"hello", 10, "hello     "},
		{"test", 4, "test"},
		{"toolong", 3, "too"},
	}

	for _, tc := range testCases {
		result := PadRight(tc.input, tc.length)
		if result != tc.expected {
			t.Errorf("PadRight(%s, %d) = %q, expected %q", tc.input, tc.length, result, tc.expected)
		}
	}
}

func TestPadLeft(t *testing.T) {
	testCases := []struct {
		input    string
		length   int
		expected string
	}{
		{"hello", 10, "     hello"},
		{"test", 4, "test"},
		{"toolong", 3, "too"},
	}

	for _, tc := range testCases {
		result := PadLeft(tc.input, tc.length)
		if result != tc.expected {
			t.Errorf("PadLeft(%s, %d) = %q, expected %q", tc.input, tc.length, result, tc.expected)
		}
	}
}

func TestSanitizeString(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{"clean text unchanged", "clean text", "clean text"},
		{"no control chars at all", "hello", "hello"},
		{"embedded newline in call number", "741.5 KHAN\n741.5 KHAN", "741.5 KHAN 741.5 KHAN"},
		{"leading newline", "\n741.5 KHAN", "741.5 KHAN"},
		{"control char with adjacent spaces", "A \n B", "A B"},
		{"crlf is one run", "A\r\nB", "A B"},
		{"null byte", "A\x00B", "A B"},
		{"real double space preserved", "A  B", "A  B"},
		{"embedded tab", "test\ttab", "test tab"},
		{"tab with adjacent spaces", "test \t tab", "test tab"},
		{"other control char", "control\x01char", "control char"},
		{"del char", "control\x7fchar", "control char"},
		{"trailing control chars", "abc\n\r", "abc"},
		{"trailing control char with space", "abc \n", "abc"},
		{"several separate runs", "a\nb\tc\rd", "a b c d"},
		{"multiple runs with spaces", "a \n b  c\t\td", "a b  c d"},
		{"empty string", "", ""},
		{"only control chars", "\n\r\t\x00", ""},
		{"only spaces and control chars", " \n ", ""},
		{"multi-byte utf8 preserved", "café ünïcode 日本語", "café ünïcode 日本語"},
		{"multi-byte utf8 with control char", "café\nünïcode", "café ünïcode"},
		{"accented multi-byte around control char", "Café\nrésumé", "Café résumé"},
		{"trailing newline only", "741.5 KHAN\n", "741.5 KHAN"},
		{"leading control char with space", "\n abc", "abc"},
		{"only control chars, two", "\n\n", ""},
		{"trailing space preserved, no control chars", "abc ", "abc "},
		{"leading space preserved, no control chars", " abc", " abc"},
		{"spaces only, no control chars, preserved", "  ", "  "},
		{"edge spaces preserved when no control chars", "  A  B  ", "  A  B  "},
	}

	for _, tc := range testCases {
		result := SanitizeString(tc.input)
		if result != tc.expected {
			t.Errorf("%s: SanitizeString(%q) = %q, expected %q", tc.name, tc.input, result, tc.expected)
		}
	}
}

// TestSanitizeString_NoControlCharsIsIdentity asserts the core invariant of
// SanitizeString: input containing no control characters is returned
// byte-identical, so values that were never part of the control-character bug
// (including ones with meaningful leading or trailing spaces) are untouched.
func TestSanitizeString_NoControlCharsIsIdentity(t *testing.T) {
	inputs := []string{
		"",
		" ",
		"  ",
		"      ",
		"abc",
		"abc ",
		" abc",
		" abc ",
		"  A  B  ",
		"A  B",
		"This is a very long title that exceeds sixty characters and ",
		"741.5 KHAN",
		"café ünïcode 日本語",
		" café ünïcode 日本語 ",
		"Café résumé",
	}

	for _, in := range inputs {
		if got := SanitizeString(in); got != in {
			t.Errorf("SanitizeString(%q) = %q, expected the input unchanged", in, got)
		}
	}
}

func TestSanitizeFieldValue(t *testing.T) {
	testCases := []struct {
		name  string
		value string
		delim string
		want  string
	}{
		// Delimiter neutralization: replaced with a space, never stripped, so
		// tokens that were visually distinct stay distinct.
		{"delimiter in the middle", "741.5|KHAN", "|", "741.5 KHAN"},
		{"multiple delimiter occurrences", "A|B|C", "|", "A B C"},
		{"consecutive delimiters", "A||B", "|", "A  B"},
		{"delimiter at start", "|AB", "|", " AB"},
		{"delimiter at end", "AB|", "|", "AB "},
		{"value is only the delimiter", "|", "|", " "},
		{"delimiter with unicode value", "café|日本語", "|", "café 日本語"},

		// Delimiter pass runs BEFORE SanitizeString, so the space it introduces
		// is absorbed by the adjacent control-character run (not "A  B").
		{"delimiter plus control char", "A|\nB", "|", "A B"},
		{"delimiter adjacent to CRLF", "741.5|KHAN\r\n741.5 KHAN", "|", "741.5 KHAN 741.5 KHAN"},

		// Multi-character delimiters.
		{"multi-char delimiter", "A||B", "||", "A B"},
		{"multi-char delimiter leaves single char alone", "A|B", "||", "A|B"},
		{"CRLF delimiter", "A\r\nB", "\r\n", "A B"},

		// A delimiter that is itself a control character: both passes apply.
		{"CR delimiter", "A\rB", "\r", "A B"},
		{"CR delimiter leaves LF to SanitizeString", "A\r\nB", "\r", "A B"},

		// No delimiter and no control characters: byte-identical.
		{"clean value", "741.5 KHAN", "|", "741.5 KHAN"},
		{"clean value with interior double space", "A  B", "|", "A  B"},
		{"clean value with edge spaces", " abc ", "|", " abc "},
		{"clean unicode value", "Café résumé", "|", "Café résumé"},
		{"empty value", "", "|", ""},

		// Control characters alone still follow SanitizeString's policy.
		{"control chars only", "\n\r", "|", ""},
		{"interior control run", "741.5 KHAN\n741.5 KHAN", "|", "741.5 KHAN 741.5 KHAN"},

		// REGRESSION: an empty delimiter must be a no-op for the delimiter pass.
		// strings.ReplaceAll(s, "", " ") inserts a space between every character,
		// which would corrupt every field of every message.
		{"empty delimiter is a no-op", "ABC", "", "ABC"},
		{"empty delimiter does not space out a sentence", "741.5 KHAN", "", "741.5 KHAN"},
		{"empty delimiter preserves pipes", "A|B|C", "", "A|B|C"},
		{"empty delimiter still sanitizes control chars", "A\nB", "", "A B"},
		{"empty delimiter with unicode", "café ünïcode 日本語", "", "café ünïcode 日本語"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := SanitizeFieldValue(tc.value, tc.delim); got != tc.want {
				t.Errorf("SanitizeFieldValue(%q, %q) = %q, want %q", tc.value, tc.delim, got, tc.want)
			}
		})
	}
}

// TestSanitizeFieldValue_EmptyDelimiterNeverExpands is an explicit guard against
// the strings.ReplaceAll(s, "", " ") trap: with an empty delimiter the result
// must never be longer than SanitizeString's own output.
func TestSanitizeFieldValue_EmptyDelimiterNeverExpands(t *testing.T) {
	inputs := []string{
		"",
		"A",
		"ABC",
		"741.5 KHAN",
		"A|B|C",
		"Café résumé",
		"A\nB",
	}

	for _, in := range inputs {
		want := SanitizeString(in)
		if got := SanitizeFieldValue(in, ""); got != want {
			t.Errorf("SanitizeFieldValue(%q, \"\") = %q, want %q (must equal SanitizeString)", in, got, want)
		}
	}
}

// TestSanitizeFieldValue_CleanValueIsIdentity documents the invariant: a value
// with no control characters and no occurrence of the configured delimiter is
// returned byte-identical.
func TestSanitizeFieldValue_CleanValueIsIdentity(t *testing.T) {
	inputs := []string{
		"",
		" ",
		"  ",
		"abc",
		"abc ",
		" abc",
		" abc ",
		"  A  B  ",
		"A  B",
		"741.5 KHAN",
		"café ünïcode 日本語",
		"Café résumé",
	}

	for _, in := range inputs {
		for _, delim := range []string{"|", "||", "\r\n", "\r", "^", ""} {
			if got := SanitizeFieldValue(in, delim); got != in {
				t.Errorf("SanitizeFieldValue(%q, %q) = %q, expected the input unchanged", in, delim, got)
			}
		}
	}
}

func TestSanitizeFixedWidthString(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{"clean text unchanged", "clean text", "clean text"},
		{"newline becomes space", "741.5 KHAN\n741.5 KHAN", "741.5 KHAN 741.5 KHAN"},
		{"leading newline becomes space", "\n741.5 KHAN", " 741.5 KHAN"},
		{"crlf becomes two spaces", "A\r\nB", "A  B"},
		{"null byte", "A\x00B", "A B"},
		{"tab", "A\tB", "A B"},
		{"del char", "A\x7fB", "A B"},
		{"patron status block preserved", "Y             ", "Y             "},
		{"patron status block with control char", "Y    \n       ", "Y            "},
		{"leading and trailing spaces preserved", "  mid  ", "  mid  "},
		{"spaces not collapsed", "A  B", "A  B"},
		{"all control chars", "\n\r\t\x00", "    "},
		{"empty string", "", ""},
		{"multi-byte utf8 preserved", "café ünïcode", "café ünïcode"},
	}

	for _, tc := range testCases {
		result := SanitizeFixedWidthString(tc.input)
		if result != tc.expected {
			t.Errorf("%s: SanitizeFixedWidthString(%q) = %q, expected %q", tc.name, tc.input, result, tc.expected)
		}
		if len(result) != len(tc.input) {
			t.Errorf("%s: SanitizeFixedWidthString(%q) changed length: got %d, expected %d",
				tc.name, tc.input, len(result), len(tc.input))
		}
	}
}

func TestCoalesceString(t *testing.T) {
	testCases := []struct {
		values   []string
		expected string
	}{
		{[]string{"", "", "first"}, "first"},
		{[]string{"first", "second"}, "first"},
		{[]string{"", "", ""}, ""},
		{[]string{"value"}, "value"},
	}

	for _, tc := range testCases {
		result := CoalesceString(tc.values...)
		if result != tc.expected {
			t.Errorf("CoalesceString(%v) = %s, expected %s", tc.values, result, tc.expected)
		}
	}
}

func TestDefaultString(t *testing.T) {
	testCases := []struct {
		value        string
		defaultValue string
		expected     string
	}{
		{"", "default", "default"},
		{"value", "default", "value"},
		{"", "", ""},
	}

	for _, tc := range testCases {
		result := DefaultString(tc.value, tc.defaultValue)
		if result != tc.expected {
			t.Errorf("DefaultString(%s, %s) = %s, expected %s", tc.value, tc.defaultValue, result, tc.expected)
		}
	}
}

func TestBoolToYN(t *testing.T) {
	if BoolToYN(true) != "Y" {
		t.Error("BoolToYN(true) should return 'Y'")
	}

	if BoolToYN(false) != "N" {
		t.Error("BoolToYN(false) should return 'N'")
	}
}

func TestYNToBool(t *testing.T) {
	testCases := []struct {
		input    string
		expected bool
	}{
		{"Y", true},
		{"y", true},
		{"N", false},
		{"n", false},
		{"", false},
		{"invalid", false},
	}

	for _, tc := range testCases {
		result := YNToBool(tc.input)
		if result != tc.expected {
			t.Errorf("YNToBool(%s) = %v, expected %v", tc.input, result, tc.expected)
		}
	}
}

func TestContains(t *testing.T) {
	slice := []string{"apple", "banana", "cherry"}

	if !Contains(slice, "banana") {
		t.Error("Contains should return true for 'banana'")
	}

	if Contains(slice, "orange") {
		t.Error("Contains should return false for 'orange'")
	}

	if Contains([]string{}, "test") {
		t.Error("Contains should return false for empty slice")
	}
}

func TestFormatCurrency(t *testing.T) {
	testCases := []struct {
		amount   float64
		expected string
	}{
		{10.5, "10.50"},
		{0.0, "0.00"},
		{123.456, "123.46"},
		{-5.25, "-5.25"},
	}

	for _, tc := range testCases {
		result := FormatCurrency(tc.amount)
		if result != tc.expected {
			t.Errorf("FormatCurrency(%f) = %s, expected %s", tc.amount, result, tc.expected)
		}
	}
}

