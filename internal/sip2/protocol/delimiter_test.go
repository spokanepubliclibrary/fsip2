package protocol

import (
	"reflect"
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

func TestBuildOptionalField(t *testing.T) {
	tests := []struct {
		name      string
		fieldCode string
		value     string
		delimiter string
		want      string
	}{
		{"with value", "AA", "1234", "|", "AA1234|"},
		{"empty value", "AA", "", "|", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildOptionalField(tt.fieldCode, tt.value, tt.delimiter)
			if got != tt.want {
				t.Errorf("BuildOptionalField(%q, %q, %q) = %q, want %q", tt.fieldCode, tt.value, tt.delimiter, got, tt.want)
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
