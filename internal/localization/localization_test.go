package localization

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

const testMessagesJSON = `{
	"checkout_success": "Checkout successful",
	"checkout_failed": "Checkout failed",
	"checkin_success": "Checkin successful",
	"checkin_failed": "Checkin failed",
	"renewal_success": "Renewal successful",
	"renewal_failed": "Renewal failed",
	"patron_not_found": "Patron not found",
	"item_not_found": "Item not found",
	"item_found": "Item found",
	"login_success": "Login successful",
	"login_failed": "Login failed",
	"session_ended": "Session ended",
	"session_end_failed": "Session end failed",
	"payment_accepted": "Payment accepted",
	"payment_failed": "Payment failed",
	"item_properties_updated": "Item properties updated",
	"item_properties_update_failed": "Item properties update failed",
	"renew_all_success": "Renew all successful",
	"renew_all_no_items": "No items to renew",
	"invalid_patron": "Invalid patron",
	"invalid_item": "Invalid item",
	"patron_blocked": "Patron blocked",
	"item_not_available": "Item not available",
	"renewal_not_permitted": "Renewal not permitted",
	"checkout_not_permitted": "Checkout not permitted",
	"hold_not_permitted": "Hold not permitted",
	"invalid_password": "Invalid password",
	"service_unavailable": "Service unavailable",
	"required_field_missing": "Required field missing",
	"invalid_field_value": "Invalid field value"
}`

func TestNewLocalizer(t *testing.T) {
	l := NewLocalizer("en")
	if l == nil {
		t.Fatal("NewLocalizer returned nil")
	}
	if l.GetDefaultLanguage() != "en" {
		t.Errorf("default language = %q, want %q", l.GetDefaultLanguage(), "en")
	}
}

func TestLoadMessagesFromString(t *testing.T) {
	l := NewLocalizer("en")

	if err := l.LoadMessagesFromString("en", testMessagesJSON); err != nil {
		t.Fatalf("LoadMessagesFromString: %v", err)
	}

	msgs := l.GetMessages("en")
	if msgs.CheckoutSuccess != "Checkout successful" {
		t.Errorf("CheckoutSuccess = %q, want %q", msgs.CheckoutSuccess, "Checkout successful")
	}
	if msgs.PatronNotFound != "Patron not found" {
		t.Errorf("PatronNotFound = %q, want %q", msgs.PatronNotFound, "Patron not found")
	}
}

func TestLoadMessagesFromString_InvalidJSON(t *testing.T) {
	l := NewLocalizer("en")
	err := l.LoadMessagesFromString("en", `{invalid json}`)
	if err == nil {
		t.Error("expected error for invalid JSON, got nil")
	}
}

func TestLoadMessages_FromFile(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "en.json")
	if err := os.WriteFile(filePath, []byte(testMessagesJSON), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	l := NewLocalizer("en")
	if err := l.LoadMessages("en", filePath); err != nil {
		t.Fatalf("LoadMessages: %v", err)
	}

	msgs := l.GetMessages("en")
	if msgs.LoginSuccess != "Login successful" {
		t.Errorf("LoginSuccess = %q, want %q", msgs.LoginSuccess, "Login successful")
	}
}

func TestLoadMessages_FileNotFound(t *testing.T) {
	l := NewLocalizer("en")
	err := l.LoadMessages("en", "/nonexistent/path/en.json")
	if err == nil {
		t.Error("expected error for missing file, got nil")
	}
}

func TestLoadMessages_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(filePath, []byte(`{bad json}`), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	l := NewLocalizer("en")
	err := l.LoadMessages("en", filePath)
	if err == nil {
		t.Error("expected error for invalid JSON file, got nil")
	}
}

func TestGetMessages_FallbackToDefault(t *testing.T) {
	l := NewLocalizer("en")
	if err := l.LoadMessagesFromString("en", testMessagesJSON); err != nil {
		t.Fatalf("LoadMessagesFromString: %v", err)
	}

	// Request a language that doesn't exist — should fall back to "en"
	msgs := l.GetMessages("fr")
	if msgs.CheckoutSuccess != "Checkout successful" {
		t.Errorf("fallback CheckoutSuccess = %q, want %q", msgs.CheckoutSuccess, "Checkout successful")
	}
}

func TestGetMessages_NeitherLangNorDefault(t *testing.T) {
	l := NewLocalizer("en")
	// No languages loaded at all — should return empty Messages
	msgs := l.GetMessages("fr")
	if msgs == nil {
		t.Fatal("GetMessages returned nil")
	}
	if msgs.CheckoutSuccess != "" {
		t.Errorf("expected empty messages, got %q", msgs.CheckoutSuccess)
	}
}

func TestSetDefaultLanguage(t *testing.T) {
	l := NewLocalizer("en")
	l.SetDefaultLanguage("fr")
	if l.GetDefaultLanguage() != "fr" {
		t.Errorf("default language = %q, want %q", l.GetDefaultLanguage(), "fr")
	}
}

func TestSupportedLanguages(t *testing.T) {
	l := NewLocalizer("en")

	// Initially empty
	langs := l.SupportedLanguages()
	if len(langs) != 0 {
		t.Errorf("expected 0 languages, got %d", len(langs))
	}

	// Load two languages
	if err := l.LoadMessagesFromString("en", testMessagesJSON); err != nil {
		t.Fatalf("LoadMessagesFromString en: %v", err)
	}
	if err := l.LoadMessagesFromString("fr", testMessagesJSON); err != nil {
		t.Fatalf("LoadMessagesFromString fr: %v", err)
	}

	langs = l.SupportedLanguages()
	if len(langs) != 2 {
		t.Errorf("expected 2 languages, got %d: %v", len(langs), langs)
	}

	sort.Strings(langs)
	if langs[0] != "en" || langs[1] != "fr" {
		t.Errorf("unexpected languages: %v", langs)
	}
}

func TestMultipleLanguages(t *testing.T) {
	frMessages := `{"checkout_success": "Emprunt réussi"}`

	l := NewLocalizer("en")
	if err := l.LoadMessagesFromString("en", testMessagesJSON); err != nil {
		t.Fatalf("LoadMessagesFromString en: %v", err)
	}
	if err := l.LoadMessagesFromString("fr", frMessages); err != nil {
		t.Fatalf("LoadMessagesFromString fr: %v", err)
	}

	enMsgs := l.GetMessages("en")
	if enMsgs.CheckoutSuccess != "Checkout successful" {
		t.Errorf("en CheckoutSuccess = %q", enMsgs.CheckoutSuccess)
	}

	frMsgs := l.GetMessages("fr")
	if frMsgs.CheckoutSuccess != "Emprunt réussi" {
		t.Errorf("fr CheckoutSuccess = %q", frMsgs.CheckoutSuccess)
	}
}
