package logging

import (
	"testing"

	"go.uber.org/zap/zapcore"
)

func TestTypeField(t *testing.T) {
	tests := []struct {
		logType       LogType
		expectedValue string
	}{
		{TypeApplication, "application"},
		{TypeSIPRequest, "sip_request"},
		{TypeSIPResponse, "sip_response"},
		{TypeFolioRequest, "folio_request"},
		{TypeFolioResponse, "folio_response"},
	}

	for _, tt := range tests {
		t.Run(string(tt.logType), func(t *testing.T) {
			field := TypeField(tt.logType)

			if field.Key != "type" {
				t.Errorf("expected key %q, got %q", "type", field.Key)
			}
			if field.Type != zapcore.StringType {
				t.Errorf("expected field type String, got %v", field.Type)
			}
			if field.String != tt.expectedValue {
				t.Errorf("expected value %q, got %q", tt.expectedValue, field.String)
			}
		})
	}
}
