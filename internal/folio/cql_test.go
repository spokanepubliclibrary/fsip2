package folio

import "testing"

func TestCQLQuote(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "plain", input: "123456", want: `"123456"`},
		{name: "spaces", input: "abc 123", want: `"abc 123"`},
		{name: "quotes", input: `ab"c`, want: `"ab\"c"`},
		{name: "quote injection", input: `x" or barcode=*`, want: `"x\" or barcode=\*"`},
		{name: "asterisk", input: "*", want: `"\*"`},
		{name: "question mark", input: "12?4", want: `"12\?4"`},
		{name: "caret", input: "^123", want: `"\^123"`},
		{name: "backslash", input: `a\b`, want: `"a\\b"`},
		{name: "trailing backslash", input: `abc\`, want: `"abc\\"`},
		{name: "backslash before special", input: `\*`, want: `"\\\*"`},
		{name: "empty", input: "", want: `""`},
		{name: "unicode", input: "Müller-東京", want: `"Müller-東京"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := cqlQuote(tt.input); got != tt.want {
				t.Errorf("cqlQuote(%q) = %s, want %s", tt.input, got, tt.want)
			}
		})
	}
}

func TestIsUUID(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{name: "v1", input: "c232b9a0-5c4e-11ef-8f3a-0242ac120002", want: true},
		{name: "v2", input: "000003e8-5c4e-21ef-9f00-325096b39f47", want: true},
		{name: "v3", input: "6fa459ea-ee8a-3ca4-894e-db77e160355e", want: true},
		{name: "v4", input: "2b2c1e6a-8f3d-4c1a-a5e7-3d9b0f6c4e21", want: true},
		{name: "v5", input: "886313e1-3b8a-5372-9b90-0c9aee199e5d", want: true},
		{name: "upper case", input: "2B2C1E6A-8F3D-4C1A-A5E7-3D9B0F6C4E21", want: true},
		{name: "mixed case", input: "2b2C1e6A-8f3D-4c1A-B5e7-3D9b0F6c4E21", want: true},
		{name: "variant 8", input: "2b2c1e6a-8f3d-4c1a-85e7-3d9b0f6c4e21", want: true},
		{name: "variant 9", input: "2b2c1e6a-8f3d-4c1a-95e7-3d9b0f6c4e21", want: true},
		{name: "version 0", input: "2b2c1e6a-8f3d-0c1a-a5e7-3d9b0f6c4e21", want: false},
		{name: "version 6", input: "2b2c1e6a-8f3d-6c1a-a5e7-3d9b0f6c4e21", want: false},
		{name: "version 7", input: "2b2c1e6a-8f3d-7c1a-a5e7-3d9b0f6c4e21", want: false},
		{name: "version 8", input: "2b2c1e6a-8f3d-8c1a-a5e7-3d9b0f6c4e21", want: false},
		{name: "version 9", input: "2b2c1e6a-8f3d-9c1a-a5e7-3d9b0f6c4e21", want: false},
		{name: "variant c", input: "2b2c1e6a-8f3d-4c1a-c5e7-3d9b0f6c4e21", want: false},
		{name: "variant d", input: "2b2c1e6a-8f3d-4c1a-d5e7-3d9b0f6c4e21", want: false},
		{name: "variant e", input: "2b2c1e6a-8f3d-4c1a-e5e7-3d9b0f6c4e21", want: false},
		{name: "variant f", input: "2b2c1e6a-8f3d-4c1a-f5e7-3d9b0f6c4e21", want: false},
		{name: "nil uuid", input: "00000000-0000-0000-0000-000000000000", want: false},
		{name: "braces", input: "{2b2c1e6a-8f3d-4c1a-a5e7-3d9b0f6c4e21}", want: false},
		{name: "urn prefix", input: "urn:uuid:2b2c1e6a-8f3d-4c1a-a5e7-3d9b0f6c4e21", want: false},
		{name: "no hyphens", input: "2b2c1e6a8f3d4c1aa5e73d9b0f6c4e21", want: false},
		{name: "leading space", input: " 2b2c1e6a-8f3d-4c1a-a5e7-3d9b0f6c4e21", want: false},
		{name: "trailing space", input: "2b2c1e6a-8f3d-4c1a-a5e7-3d9b0f6c4e21 ", want: false},
		{name: "trailing newline", input: "2b2c1e6a-8f3d-4c1a-a5e7-3d9b0f6c4e21\n", want: false},
		{name: "non-hex", input: "2b2c1e6a-8f3d-4c1a-a5e7-3d9b0f6c4e2g", want: false},
		{name: "asterisk", input: "*", want: false},
		{name: "quote injection", input: `x" or id=="y`, want: false},
		{name: "empty", input: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isUUID(tt.input); got != tt.want {
				t.Errorf("isUUID(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
