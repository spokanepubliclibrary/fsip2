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
		input    string
		maxLen   int
		expected string
	}{
		{"hello", 10, "hello"},
		{"hello world", 5, "hello"},
		{"test", 4, "test"},
		{"", 5, ""},
	}

	for _, tc := range testCases {
		result := TruncateString(tc.input, tc.maxLen)
		if result != tc.expected {
			t.Errorf("TruncateString(%s, %d) = %s, expected %s", tc.input, tc.maxLen, result, tc.expected)
		}
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
		input    string
		expected string
	}{
		{"hello", "hello"},
		{"hello\x00world", "helloworld"},
		{"test\n\ttab", "test\n\ttab"},
		{"control\x01char", "controlchar"},
	}

	for _, tc := range testCases {
		result := SanitizeString(tc.input)
		if result != tc.expected {
			t.Errorf("SanitizeString(%q) = %q, expected %q", tc.input, result, tc.expected)
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

