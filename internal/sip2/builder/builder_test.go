package builder

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/spokanepubliclibrary/fsip2/internal/config"
)

func TestBuildRenewAllResponse(t *testing.T) {
	cfg := &config.TenantConfig{
		ErrorDetectionEnabled: false,
		MessageDelimiter:      "\r",
		FieldDelimiter:        "|",
		Timezone:              "America/New_York",
	}

	builder := NewResponseBuilder(cfg)

	tests := []struct {
		name           string
		ok             bool
		renewedCount   int
		unrenewedCount int
		institutionID  string
		patronID       string
		renewedItems   []string
		unrenewedItems []string
		screenMessage  []string
		wantPrefix     string // Expected message code
		wantOk         string // Expected ok field
	}{
		{
			name:           "Successful renewal with items",
			ok:             true,
			renewedCount:   2,
			unrenewedCount: 1,
			institutionID:  "test-inst",
			patronID:       "patron123",
			renewedItems:   []string{"ITEM001", "ITEM002"},
			unrenewedItems: []string{"ITEM003"},
			screenMessage:  []string{"Some items could not be renewed"},
			wantPrefix:     "66",
			wantOk:         "Y",
		},
		{
			name:           "All renewals failed",
			ok:             false,
			renewedCount:   0,
			unrenewedCount: 3,
			institutionID:  "test-inst",
			patronID:       "patron123",
			renewedItems:   []string{},
			unrenewedItems: []string{"ITEM001", "ITEM002", "ITEM003"},
			screenMessage:  []string{"No items could be renewed"},
			wantPrefix:     "66",
			wantOk:         "N",
		},
		{
			name:           "All renewals successful",
			ok:             true,
			renewedCount:   5,
			unrenewedCount: 0,
			institutionID:  "test-inst",
			patronID:       "patron123",
			renewedItems:   []string{"ITEM001", "ITEM002", "ITEM003", "ITEM004", "ITEM005"},
			unrenewedItems: []string{},
			screenMessage:  []string{},
			wantPrefix:     "66",
			wantOk:         "Y",
		},
		{
			name:           "No loans to renew",
			ok:             false,
			renewedCount:   0,
			unrenewedCount: 0,
			institutionID:  "test-inst",
			patronID:       "patron123",
			renewedItems:   []string{},
			unrenewedItems: []string{},
			screenMessage:  []string{"Renewal failed"},
			wantPrefix:     "66",
			wantOk:         "N",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transactionDate := time.Date(2025, 1, 10, 14, 30, 0, 0, time.UTC)

			response, err := builder.BuildRenewAllResponse(
				tt.ok,
				tt.renewedCount,
				tt.unrenewedCount,
				transactionDate,
				tt.institutionID,
				tt.patronID,
				tt.renewedItems,
				tt.unrenewedItems,
				tt.screenMessage,
				"0",
			)

			if err != nil {
				t.Fatalf("BuildRenewAllResponse() error = %v", err)
			}

			// Verify message code
			if !strings.HasPrefix(response, tt.wantPrefix) {
				t.Errorf("Expected response to start with %s, got: %s", tt.wantPrefix, response[:10])
			}

			// Verify ok field (position 2)
			if len(response) > 2 && string(response[2]) != tt.wantOk {
				t.Errorf("Expected ok field = %s, got: %s", tt.wantOk, string(response[2]))
			}

			// Verify renewed count (positions 3-6)
			expectedRenewedCount := formatCount(tt.renewedCount)
			if len(response) > 6 && response[3:7] != expectedRenewedCount {
				t.Errorf("Expected renewed count = %s, got: %s", expectedRenewedCount, response[3:7])
			}

			// Verify unrenewed count (positions 7-10)
			expectedUnrenewedCount := formatCount(tt.unrenewedCount)
			if len(response) > 10 && response[7:11] != expectedUnrenewedCount {
				t.Errorf("Expected unrenewed count = %s, got: %s", expectedUnrenewedCount, response[7:11])
			}

			// Verify institution ID field (first variable field has no leading delimiter)
			if !strings.Contains(response, "AO"+tt.institutionID+"|") {
				t.Errorf("Expected institution ID AO%s| in response", tt.institutionID)
			}

			// Verify patron ID field
			if !strings.Contains(response, "|AA"+tt.patronID+"|") {
				t.Errorf("Expected patron ID |AA%s| in response", tt.patronID)
			}

			// Verify renewed items count (BM field)
			if len(tt.renewedItems) > 0 {
				expectedCount := fmt.Sprintf("BM%04d", len(tt.renewedItems))
				if !strings.Contains(response, expectedCount) {
					t.Errorf("Expected renewed count %s in response, got: %s", expectedCount, response)
				}
			}

			// Verify unrenewed items count (BN field)
			if len(tt.unrenewedItems) > 0 {
				expectedCount := fmt.Sprintf("BN%04d", len(tt.unrenewedItems))
				if !strings.Contains(response, expectedCount) {
					t.Errorf("Expected unrenewed count %s in response, got: %s", expectedCount, response)
				}
			}

			// Verify screen messages (AF fields)
			for _, msg := range tt.screenMessage {
				if !strings.Contains(response, "|AF"+msg) {
					t.Errorf("Expected screen message |AF%s in response", msg)
				}
			}

			// Verify message delimiter at end
			if !strings.HasSuffix(response, cfg.MessageDelimiter) {
				t.Errorf("Expected response to end with message delimiter %q", cfg.MessageDelimiter)
			}
		})
	}
}

func TestBuildRenewAllResponse_WithErrorDetection(t *testing.T) {
	cfg := &config.TenantConfig{
		ErrorDetectionEnabled: true,
		MessageDelimiter:      "\r",
		FieldDelimiter:        "|",
		Charset:               "UTF-8",
		Timezone:              "America/New_York",
	}

	builder := NewResponseBuilder(cfg)

	transactionDate := time.Date(2025, 1, 10, 14, 30, 0, 0, time.UTC)

	response, err := builder.BuildRenewAllResponse(
		true,
		2,
		1,
		transactionDate,
		"test-inst",
		"patron123",
		[]string{"ITEM001", "ITEM002"},
		[]string{"ITEM003"},
		[]string{},
		"5",
	)

	if err != nil {
		t.Fatalf("BuildRenewAllResponse() error = %v", err)
	}

	// Verify AY (sequence number) field is present
	if !strings.Contains(response, "AY5") {
		t.Errorf("Expected sequence number AY5 in response with error detection enabled")
	}

	// Verify AZ (checksum) field is present (no delimiter before it)
	if !strings.Contains(response, "AZ") {
		t.Errorf("Expected checksum AZ field in response with error detection enabled")
	}
}

func TestBuildRenewAllResponse_EmptyItems(t *testing.T) {
	cfg := &config.TenantConfig{
		ErrorDetectionEnabled: false,
		MessageDelimiter:      "\r",
		FieldDelimiter:        "|",
		Timezone:              "America/New_York",
	}

	builder := NewResponseBuilder(cfg)
	transactionDate := time.Date(2025, 1, 10, 14, 30, 0, 0, time.UTC)

	response, err := builder.BuildRenewAllResponse(
		false,
		0,
		0,
		transactionDate,
		"test-inst",
		"patron123",
		[]string{},
		[]string{},
		[]string{"Renewal failed"},
		"0",
	)

	if err != nil {
		t.Fatalf("BuildRenewAllResponse() error = %v", err)
	}

	// Verify BM and BN fields show 0000 when there are no items
	if !strings.Contains(response, "BM0000") {
		t.Errorf("Expected BM0000 when renewedItems is empty")
	}

	if !strings.Contains(response, "BN0000") {
		t.Errorf("Expected BN0000 when unrenewedItems is empty")
	}

	// Verify counts are both 0000 (positions 3-10: renewed+unrenewed)
	if !strings.Contains(response, "66N00000000") {
		t.Errorf("Expected response to start with 66N00000000 (ok=N, renewed=0000, unrenewed=0000)")
	}
}

// Helper function to format count as 4-digit string
func formatCount(count int) string {
	if count < 0 {
		count = 0
	}
	if count > 9999 {
		count = 9999
	}
	return padLeft(count, 4)
}

// Helper function to pad integer with leading zeros
func padLeft(n int, width int) string {
	s := ""
	for i := 0; i < width; i++ {
		s += "0"
	}
	num := ""
	if n == 0 {
		num = "0"
	} else {
		for n > 0 {
			num = string(rune('0'+(n%10))) + num
			n /= 10
		}
	}
	if len(num) >= width {
		return num
	}
	return s[:width-len(num)] + num
}
