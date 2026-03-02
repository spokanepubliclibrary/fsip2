package protocol

import (
	"testing"

	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/unicode"
)

func TestGetEncoder(t *testing.T) {
	tests := []struct {
		charset string
		want    interface{}
		wantErr bool
	}{
		{"IBM850", charmap.CodePage850, false},
		{"ISO-8859-1", charmap.ISO8859_1, false},
		{"UTF-8", unicode.UTF8, false},
		{"IBM437", charmap.CodePage437, false},
		{"Windows-1252", charmap.Windows1252, false},
		{"invalid", nil, true},
		{"", nil, true},
		{"utf8", nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.charset, func(t *testing.T) {
			enc, err := GetEncoder(tt.charset)
			if tt.wantErr {
				if err == nil {
					t.Errorf("GetEncoder(%q) expected error, got nil", tt.charset)
				}
				return
			}
			if err != nil {
				t.Fatalf("GetEncoder(%q) unexpected error: %v", tt.charset, err)
			}
			if enc == nil {
				t.Errorf("GetEncoder(%q) returned nil encoder", tt.charset)
			}
		})
	}
}

func TestEncodeString(t *testing.T) {
	tests := []struct {
		name    string
		s       string
		charset string
		wantErr bool
	}{
		{"empty string UTF-8", "", "UTF-8", false},
		{"ASCII UTF-8", "Hello", "UTF-8", false},
		{"ASCII IBM850", "Hello World", "IBM850", false},
		{"ASCII ISO-8859-1", "Hello", "ISO-8859-1", false},
		{"unsupported charset", "Hello", "invalid", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := EncodeString(tt.s, tt.charset)
			if tt.wantErr {
				if err == nil {
					t.Errorf("EncodeString(%q, %q) expected error, got nil", tt.s, tt.charset)
				}
				return
			}
			if err != nil {
				t.Fatalf("EncodeString(%q, %q) unexpected error: %v", tt.s, tt.charset, err)
			}
			if tt.s != "" && len(b) == 0 {
				t.Errorf("EncodeString(%q, %q) returned empty bytes", tt.s, tt.charset)
			}
		})
	}
}

func TestEncodeDecodeRoundTrip(t *testing.T) {
	charsets := []string{"UTF-8", "IBM850", "ISO-8859-1", "IBM437", "Windows-1252"}
	input := "Hello World 123"

	for _, charset := range charsets {
		t.Run(charset, func(t *testing.T) {
			encoded, err := EncodeString(input, charset)
			if err != nil {
				t.Fatalf("EncodeString: %v", err)
			}

			decoded, err := DecodeBytes(encoded, charset)
			if err != nil {
				t.Fatalf("DecodeBytes: %v", err)
			}

			if decoded != input {
				t.Errorf("round-trip failed: got %q, want %q", decoded, input)
			}
		})
	}
}

func TestDecodeBytes(t *testing.T) {
	tests := []struct {
		name    string
		b       []byte
		charset string
		wantErr bool
	}{
		{"empty bytes UTF-8", []byte{}, "UTF-8", false},
		{"ASCII UTF-8", []byte("Hello"), "UTF-8", false},
		{"ASCII IBM850", []byte("Hello"), "IBM850", false},
		{"unsupported charset", []byte("Hello"), "invalid", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := DecodeBytes(tt.b, tt.charset)
			if tt.wantErr {
				if err == nil {
					t.Errorf("DecodeBytes expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("DecodeBytes unexpected error: %v", err)
			}
			_ = s
		})
	}
}

func TestSupportedCharsets(t *testing.T) {
	charsets := SupportedCharsets()
	if len(charsets) == 0 {
		t.Fatal("SupportedCharsets returned empty list")
	}

	expected := []string{"IBM850", "ISO-8859-1", "UTF-8", "IBM437", "Windows-1252"}
	if len(charsets) != len(expected) {
		t.Errorf("SupportedCharsets len = %d, want %d", len(charsets), len(expected))
	}

	for _, exp := range expected {
		found := false
		for _, c := range charsets {
			if c == exp {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("SupportedCharsets missing %q", exp)
		}
	}
}

func TestIsCharsetSupported(t *testing.T) {
	tests := []struct {
		charset string
		want    bool
	}{
		{"IBM850", true},
		{"ISO-8859-1", true},
		{"UTF-8", true},
		{"IBM437", true},
		{"Windows-1252", true},
		{"invalid", false},
		{"", false},
		{"utf-8", false},
	}

	for _, tt := range tests {
		t.Run(tt.charset, func(t *testing.T) {
			got := IsCharsetSupported(tt.charset)
			if got != tt.want {
				t.Errorf("IsCharsetSupported(%q) = %v, want %v", tt.charset, got, tt.want)
			}
		})
	}
}
