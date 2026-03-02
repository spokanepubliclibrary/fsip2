package localization

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
)

// Messages represents localized message strings
type Messages struct {
	// Screen messages for SIP2 responses
	CheckoutSuccess          string `json:"checkout_success"`
	CheckoutFailed           string `json:"checkout_failed"`
	CheckinSuccess           string `json:"checkin_success"`
	CheckinFailed            string `json:"checkin_failed"`
	RenewalSuccess           string `json:"renewal_success"`
	RenewalFailed            string `json:"renewal_failed"`
	PatronNotFound           string `json:"patron_not_found"`
	ItemNotFound             string `json:"item_not_found"`
	ItemFound                string `json:"item_found"`
	LoginSuccess             string `json:"login_success"`
	LoginFailed              string `json:"login_failed"`
	SessionEnded             string `json:"session_ended"`
	SessionEndFailed         string `json:"session_end_failed"`
	PaymentAccepted          string `json:"payment_accepted"`
	PaymentFailed            string `json:"payment_failed"`
	ItemPropertiesUpdated    string `json:"item_properties_updated"`
	ItemPropertiesUpdateFail string `json:"item_properties_update_failed"`
	RenewAllSuccess          string `json:"renew_all_success"`
	RenewAllNoItems          string `json:"renew_all_no_items"`

	// Error messages
	InvalidPatron        string `json:"invalid_patron"`
	InvalidItem          string `json:"invalid_item"`
	PatronBlocked        string `json:"patron_blocked"`
	ItemNotAvailable     string `json:"item_not_available"`
	RenewalNotPermitted  string `json:"renewal_not_permitted"`
	CheckoutNotPermitted string `json:"checkout_not_permitted"`
	HoldNotPermitted     string `json:"hold_not_permitted"`
	InvalidPassword      string `json:"invalid_password"`
	ServiceUnavailable   string `json:"service_unavailable"`

	// Validation messages
	RequiredFieldMissing string `json:"required_field_missing"`
	InvalidFieldValue    string `json:"invalid_field_value"`
}

// Localizer manages localized messages
type Localizer struct {
	messages    map[string]*Messages // language code -> messages
	mu          sync.RWMutex
	defaultLang string // default language
}

// NewLocalizer creates a new localizer with default language
func NewLocalizer(defaultLang string) *Localizer {
	return &Localizer{
		messages:    make(map[string]*Messages),
		defaultLang: defaultLang,
	}
}

// LoadMessages loads messages from a JSON file for a specific language
func (l *Localizer) LoadMessages(lang string, filePath string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read localization file: %w", err)
	}

	var messages Messages
	if err := json.Unmarshal(data, &messages); err != nil {
		return fmt.Errorf("failed to parse localization file: %w", err)
	}

	l.messages[lang] = &messages
	return nil
}

// LoadMessagesFromString loads messages from a JSON string
func (l *Localizer) LoadMessagesFromString(lang string, jsonData string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	var messages Messages
	if err := json.Unmarshal([]byte(jsonData), &messages); err != nil {
		return fmt.Errorf("failed to parse localization data: %w", err)
	}

	l.messages[lang] = &messages
	return nil
}

// GetMessages returns messages for the specified language, or default if not found
func (l *Localizer) GetMessages(lang string) *Messages {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if messages, ok := l.messages[lang]; ok {
		return messages
	}

	// Fall back to default language
	if messages, ok := l.messages[l.defaultLang]; ok {
		return messages
	}

	// Return empty messages if nothing found
	return &Messages{}
}

// SetDefaultLanguage sets the default language
func (l *Localizer) SetDefaultLanguage(lang string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.defaultLang = lang
}

// GetDefaultLanguage returns the default language
func (l *Localizer) GetDefaultLanguage() string {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.defaultLang
}

// SupportedLanguages returns a list of loaded languages
func (l *Localizer) SupportedLanguages() []string {
	l.mu.RLock()
	defer l.mu.RUnlock()

	langs := make([]string, 0, len(l.messages))
	for lang := range l.messages {
		langs = append(langs, lang)
	}
	return langs
}
