package config

import (
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Config represents the main application configuration
type Config struct {
	Port                int            `yaml:"port"`
	Host                string         `yaml:"host,omitempty"` // Bind address (e.g., "127.0.0.1"); empty = all interfaces
	OkapiURL            string         `yaml:"okapiUrl"`
	HealthCheckPort     int            `yaml:"healthCheckPort"`
	TokenCacheCapacity  int            `yaml:"tokenCacheCapacity"`
	ScanPeriod          int            `yaml:"scanPeriod"` // milliseconds
	LogLevel            string         `yaml:"logLevel"`
	TenantConfigSources []ConfigSource `yaml:"tenantConfigSources"`
	TLS                 *TLSConfig     `yaml:"tls,omitempty"`

	// Runtime tenant configurations (loaded from sources)
	Tenants map[string]*TenantConfig `yaml:"-"`
}

// ConfigSource represents a source for tenant configuration
type ConfigSource struct {
	Type   string `yaml:"type"` // file, http, s3
	Path   string `yaml:"path,omitempty"`
	URL    string `yaml:"url,omitempty"`
	Bucket string `yaml:"bucket,omitempty"`
	Key    string `yaml:"key,omitempty"`
	Region string `yaml:"region,omitempty"`
}

// TLSConfig represents TLS/SSL configuration
type TLSConfig struct {
	Enabled  bool   `yaml:"enabled"`
	CertFile string `yaml:"certFile"`
	KeyFile  string `yaml:"keyFile"`
}

// TenantConfig represents configuration for a specific tenant
type TenantConfig struct {
	Tenant                string `yaml:"tenant"`
	ErrorDetectionEnabled bool   `yaml:"errorDetectionEnabled"`
	MessageDelimiter      string `yaml:"messageDelimiter"`
	FieldDelimiter        string `yaml:"fieldDelimiter"`
	Charset               string `yaml:"charset"`
	Timezone              string `yaml:"timezone"`
	LogLevel              string `yaml:"logLevel"`
	OkapiURL              string `yaml:"okapiUrl"`
	OkapiTenant           string `yaml:"okapiTenant"`

	// Multi-tenant configurations
	SCTenants []SCTenantConfig `yaml:"scTenants,omitempty"`

	// SIP2 message support
	SupportedMessages []MessageSupport `yaml:"supportedMessages"`

	// FOLIO-specific settings
	PatronPasswordVerificationRequired bool              `yaml:"patronPasswordVerificationRequired"`
	UsePinForPatronVerification        bool              `yaml:"usePinForPatronVerification"`
	InvalidCheckinStatuses             string            `yaml:"invalidCheckinStatuses"`
	ClaimedReturnedResolution          string            `yaml:"claimedReturnedResolution"` // How to resolve claimed returned items during checkin: patron, library, none
	StatusUpdateOk                     bool              `yaml:"statusUpdateOk"`
	OfflineOk                          bool              `yaml:"offlineOk"`
	TimeoutPeriod                      int               `yaml:"timeoutPeriod,omitempty"`  // Timeout in seconds for ACS Status Response (000-999, default: 30)
	RetriesAllowed                     int               `yaml:"retriesAllowed,omitempty"` // Number of retries allowed for ACS Status Response (000-999, default: 3)
	Currency                           string            `yaml:"currency"`                           // Currency code (e.g., USD, EUR)
	CirculationStatusMapping           map[string]string `yaml:"circulationStatusMapping,omitempty"` // FOLIO status -> SIP2 code
	RenewAllMaxItems                   int               `yaml:"renewAllMaxItems,omitempty"`         // Maximum items to process in renew all (default: 50)

	// Fee/Fine payment settings
	AcceptBulkPayment bool   `yaml:"acceptBulkPayment"` // Enable bulk payment fallback when account ID not found/provided (default: false)
	PaymentMethod     string `yaml:"paymentMethod"`     // Payment method for fee/fine payments (default: "Credit card")
	NotifyPatron      bool   `yaml:"notifyPatron"`      // Notify patron of payment via email/SMS (default: false)

	// Rolling renewal settings
	RollingRenewals *RollingRenewalConfig `yaml:"rollingRenewals,omitempty"`

	// Patron custom fields configuration
	PatronCustomFields *PatronCustomFieldsConfig `yaml:"patronCustomFields,omitempty"`
}

// RollingRenewalConfig represents configuration for automatic patron account renewal
type RollingRenewalConfig struct {
	Enabled             bool     `yaml:"enabled"`                       // Master switch for rolling renewals (default: false)
	RenewWithin         string   `yaml:"renewWithin"`                   // Renew if expiration is within this period (e.g., "6M", "30D", "1Y")
	ExtendFor           string   `yaml:"extendFor"`                     // Extend expiration to this period from today (e.g., "6Y", "12M")
	ExtendExpired       bool     `yaml:"extendExpired"`                 // If true: extend expired accounts from today; If false: skip expired accounts (default: false)
	ExtendExpiredLimits string   `yaml:"extendExpiredLimits,omitempty"` // Do not renew if expiration is more than this period in the past (e.g., "6M"); blank = no limit
	DryRun              bool     `yaml:"dryRun"`                        // If true: log what would happen without updating records (default: false)
	SelectPatrons       bool     `yaml:"selectPatrons"`                 // If true: only renew patron groups in AllowedPatrons list (default: false)
	AllowedPatrons      []string `yaml:"allowedPatrons,omitempty"`      // List of patron group UUIDs allowed for renewal (only used if SelectPatrons is true)
}

// PatronCustomFieldsConfig represents configuration for custom fields in patron information response (64)
type PatronCustomFieldsConfig struct {
	Enabled bool                 `yaml:"enabled"` // Master switch - if false, no custom fields are included
	Fields  []CustomFieldMapping `yaml:"fields"`  // Field mappings (SA-SZ)
}

// CustomFieldMapping represents a single custom field mapping
type CustomFieldMapping struct {
	Code           string `yaml:"code"`                     // SIP2 field code (SA-SZ only)
	Source         string `yaml:"source"`                   // Key in user.customFields from FOLIO
	Type           string `yaml:"type"`                     // Data type: string, boolean, array
	ArrayDelimiter string `yaml:"arrayDelimiter,omitempty"` // How to join array elements (default: ",")
	MaxLength      int    `yaml:"maxLength,omitempty"`      // Maximum length of value (default: 60)
}

// SCTenantConfig represents a sub-tenant configuration for multi-tenancy
type SCTenantConfig struct {
	Tenant           string   `yaml:"tenant"`
	SCSubnet         string   `yaml:"scSubnet,omitempty"`
	Port             int      `yaml:"port,omitempty"`
	LocationCodes    []string `yaml:"locationCodes,omitempty"`
	UsernamePrefixes []string `yaml:"usernamePrefixes,omitempty"`
}

// MessageSupport represents support configuration for a SIP2 message
type MessageSupport struct {
	Code    string               `yaml:"code"`
	Enabled bool                 `yaml:"enabled"`
	Fields  []FieldConfiguration `yaml:"fields,omitempty"`
}

// FieldConfiguration represents field-level configuration for message responses
type FieldConfiguration struct {
	Code    string `yaml:"code"`
	Enabled bool   `yaml:"enabled"`
}

// Load loads configuration from a YAML file
func Load(configFile string) (*Config, error) {
	data, err := os.ReadFile(configFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Set defaults
	if cfg.Port == 0 {
		cfg.Port = 6443
	}
	if cfg.HealthCheckPort == 0 {
		cfg.HealthCheckPort = 8081
	}
	if cfg.TokenCacheCapacity == 0 {
		cfg.TokenCacheCapacity = 100
	}
	if cfg.ScanPeriod == 0 {
		cfg.ScanPeriod = 300000 // 5 minutes
	}
	if cfg.LogLevel == "" {
		cfg.LogLevel = "info"
	}

	// Initialize tenant map
	cfg.Tenants = make(map[string]*TenantConfig)

	// Load tenant configurations from sources
	if err := cfg.loadTenantConfigs(); err != nil {
		return nil, fmt.Errorf("failed to load tenant configs: %w", err)
	}

	return &cfg, nil
}

// loadTenantConfigs loads tenant configurations from all configured sources
func (c *Config) loadTenantConfigs() error {
	for _, source := range c.TenantConfigSources {
		var loader TenantConfigLoader

		switch source.Type {
		case "file":
			loader = &FileLoader{Path: source.Path}
		case "http":
			loader = &HTTPLoader{URL: source.URL}
		case "s3":
			loader = &S3Loader{
				Bucket: source.Bucket,
				Key:    source.Key,
				Region: source.Region,
			}
		default:
			return fmt.Errorf("unsupported config source type: %s", source.Type)
		}

		tenantCfg, err := loader.Load()
		if err != nil {
			return fmt.Errorf("failed to load config from %s: %w", source.Type, err)
		}

		// Add to tenant map
		c.Tenants[tenantCfg.Tenant] = tenantCfg

		// Also add SC tenants
		for _, scTenant := range tenantCfg.SCTenants {
			if scTenant.Tenant != "" {
				// Create a copy with overridden tenant name
				scCfg := *tenantCfg
				scCfg.Tenant = scTenant.Tenant
				c.Tenants[scTenant.Tenant] = &scCfg
			}
		}
	}

	return nil
}

// Validate validates the bootstrap configuration
// Returns error if the configuration is invalid
func (c *Config) Validate() error {
	// Validate Port
	if c.Port < 1 || c.Port > 65535 {
		return fmt.Errorf("invalid port: %d (must be 1-65535)", c.Port)
	}

	// Validate HealthCheckPort
	if c.HealthCheckPort < 1 || c.HealthCheckPort > 65535 {
		return fmt.Errorf("invalid health check port: %d (must be 1-65535)", c.HealthCheckPort)
	}

	// Validate OkapiURL is not empty
	if c.OkapiURL == "" {
		return fmt.Errorf("OkapiURL is required")
	}

	// Validate OkapiURL is a valid URL
	if _, err := url.Parse(c.OkapiURL); err != nil {
		return fmt.Errorf("invalid OkapiURL: %w", err)
	}

	// Validate TokenCacheCapacity is positive
	if c.TokenCacheCapacity < 1 {
		return fmt.Errorf("token cache capacity must be positive (got %d)", c.TokenCacheCapacity)
	}

	// Validate ScanPeriod is non-negative
	if c.ScanPeriod < 0 {
		return fmt.Errorf("scan period must be non-negative (got %d)", c.ScanPeriod)
	}

	// Validate LogLevel is a known value
	validLogLevels := map[string]bool{
		"debug": true,
		"info":  true,
		"warn":  true,
		"error": true,
	}
	if !validLogLevels[strings.ToLower(c.LogLevel)] {
		return fmt.Errorf("invalid log level: %s (must be debug, info, warn, or error)", c.LogLevel)
	}

	// Validate TLS configuration if enabled
	if c.TLS != nil && c.TLS.Enabled {
		if c.TLS.CertFile == "" {
			return fmt.Errorf("TLS cert file is required when TLS is enabled")
		}
		if c.TLS.KeyFile == "" {
			return fmt.Errorf("TLS key file is required when TLS is enabled")
		}
	}

	return nil
}

// GetScanPeriod returns the scan period as a time.Duration
func (c *Config) GetScanPeriod() time.Duration {
	return time.Duration(c.ScanPeriod) * time.Millisecond
}

// IsMessageSupported checks if a message code is supported for a tenant
func (tc *TenantConfig) IsMessageSupported(code string) bool {
	for _, msg := range tc.SupportedMessages {
		if msg.Code == code && msg.Enabled {
			return true
		}
	}
	return false
}

// GetMessageDelimiterBytes returns the message delimiter as bytes
func (tc *TenantConfig) GetMessageDelimiterBytes() []byte {
	switch tc.MessageDelimiter {
	case "\\r":
		return []byte("\r")
	case "\\n":
		return []byte("\n")
	case "\\r\\n":
		return []byte("\r\n")
	default:
		return []byte(tc.MessageDelimiter)
	}
}

// GetFieldDelimiterBytes returns the field delimiter as bytes
func (tc *TenantConfig) GetFieldDelimiterBytes() []byte {
	return []byte(tc.FieldDelimiter)
}

// IsFieldEnabled checks if a field is enabled for a specific message
// Returns true if the field is enabled, false if disabled or not configured
func (tc *TenantConfig) IsFieldEnabled(messageCode, fieldCode string) bool {
	for _, msg := range tc.SupportedMessages {
		if msg.Code == messageCode {
			// If no field configuration exists, assume all fields are enabled by default
			if len(msg.Fields) == 0 {
				return true
			}

			// Check if field is explicitly configured
			for _, field := range msg.Fields {
				if field.Code == fieldCode {
					return field.Enabled
				}
			}

			// Field not in configuration, default to enabled
			return true
		}
	}

	// Message not found, default to enabled
	return true
}

// MapCirculationStatus maps a FOLIO item status to a SIP2 circulation status code
// Returns the mapped SIP2 code, or "01" (Other) as default if not found
func (tc *TenantConfig) MapCirculationStatus(folioStatus string) string {
	// If no mapping is configured, use default mappings
	if len(tc.CirculationStatusMapping) == 0 {
		return getDefaultCirculationStatusMapping(folioStatus)
	}

	// Check configured mapping
	if sip2Code, ok := tc.CirculationStatusMapping[folioStatus]; ok {
		return sip2Code
	}

	// Check for default fallback in mapping
	if defaultCode, ok := tc.CirculationStatusMapping["default"]; ok {
		return defaultCode
	}

	// Ultimate fallback
	return "01" // Other
}

// getDefaultCirculationStatusMapping provides default FOLIO status to SIP2 code mappings
func getDefaultCirculationStatusMapping(folioStatus string) string {
	defaults := map[string]string{
		"Available":        "03",
		"Checked out":      "04",
		"In process":       "06",
		"Awaiting pickup":  "08",
		"In transit":       "10",
		"Claimed returned": "11",
		"Lost and paid":    "12",
		"Aged to lost":     "12",
		"Declared lost":    "12",
		"Missing":          "13",
		"Withdrawn":        "01",
		"On order":         "02",
		"Paged":            "08",
	}

	if code, ok := defaults[folioStatus]; ok {
		return code
	}

	return "01" // Other (default)
}

// GetRenewAllMaxItems returns the maximum items to process in renew all
// Returns the configured value, or 50 as default if not set
func (tc *TenantConfig) GetRenewAllMaxItems() int {
	if tc.RenewAllMaxItems <= 0 {
		return 50 // Default limit
	}
	return tc.RenewAllMaxItems
}

// GetPaymentMethod returns the payment method for fee/fine payments
// Returns the configured value, or "Credit card" as default if not set
func (tc *TenantConfig) GetPaymentMethod() string {
	if tc.PaymentMethod == "" {
		return "Credit card" // Default payment method
	}
	return tc.PaymentMethod
}

// GetAcceptBulkPayment returns whether bulk payment fallback is enabled
// Returns the configured value (defaults to false)
func (tc *TenantConfig) GetAcceptBulkPayment() bool {
	return tc.AcceptBulkPayment
}

// GetNotifyPatron returns whether to notify patrons of fee/fine payments
// Returns the configured value (defaults to false)
func (tc *TenantConfig) GetNotifyPatron() bool {
	return tc.NotifyPatron
}

// GetTimeoutPeriod returns the timeout period formatted as a 3-digit string
// Returns the configured value, or "030" (30 seconds) as default if not set
func (tc *TenantConfig) GetTimeoutPeriod() string {
	timeout := tc.TimeoutPeriod
	if timeout <= 0 {
		timeout = 30 // Default: 30 seconds
	}
	if timeout > 999 {
		timeout = 999 // Maximum allowed value
	}
	return fmt.Sprintf("%03d", timeout)
}

// GetRetriesAllowed returns the retries allowed formatted as a 3-digit string
// Returns the configured value, or "003" (3 retries) as default if not set
func (tc *TenantConfig) GetRetriesAllowed() string {
	retries := tc.RetriesAllowed
	if retries <= 0 {
		retries = 3 // Default: 3 retries
	}
	if retries > 999 {
		retries = 999 // Maximum allowed value
	}
	return fmt.Sprintf("%03d", retries)
}

// BuildSupportedMessages builds the BX field (supported messages) from the supportedMessages configuration
// Returns a 16-character string where each position represents support for a specific SIP2 message:
// Position 1: Patron Status Request (23)
// Position 2: Checkout (11)
// Position 3: Checkin (09)
// Position 4: Block Patron (01) - Always N (not implemented)
// Position 5: SC/ACS Status (99)
// Position 6: Request SC/ACS Resend (97)
// Position 7: Login (93)
// Position 8: Patron Information (63)
// Position 9: End Patron Session (35)
// Position 10: Fee Paid (37)
// Position 11: Item Information (17)
// Position 12: Item Status Update (19)
// Position 13: Patron Enable (25) - Always N (not implemented)
// Position 14: Hold (15)
// Position 15: Renew (29)
// Position 16: Renew All (65)
func (tc *TenantConfig) BuildSupportedMessages() string {
	// Message code to position mapping
	messagePositions := map[string]int{
		"23": 0,  // Patron Status Request
		"11": 1,  // Checkout
		"09": 2,  // Checkin
		"01": 3,  // Block Patron (always N - not implemented)
		"99": 4,  // SC/ACS Status
		"97": 5,  // Request SC/ACS Resend
		"93": 6,  // Login
		"63": 7,  // Patron Information
		"35": 8,  // End Patron Session
		"37": 9,  // Fee Paid
		"17": 10, // Item Information
		"19": 11, // Item Status Update
		"25": 12, // Patron Enable (always N - not implemented)
		"15": 13, // Hold
		"29": 14, // Renew
		"65": 15, // Renew All
	}

	// Initialize all positions to 'N'
	result := make([]byte, 16)
	for i := range result {
		result[i] = 'N'
	}

	// Set positions based on supportedMessages configuration
	for _, msg := range tc.SupportedMessages {
		if pos, exists := messagePositions[msg.Code]; exists {
			// Always keep positions 3 (Block Patron) and 12 (Patron Enable) as 'N'
			// These are not implemented in the service
			if pos == 3 || pos == 12 {
				result[pos] = 'N'
			} else if msg.Enabled {
				result[pos] = 'Y'
			} else {
				result[pos] = 'N'
			}
		}
	}

	return string(result)
}

// IsRollingRenewalEnabled returns whether rolling renewals are enabled for this tenant
func (tc *TenantConfig) IsRollingRenewalEnabled() bool {
	return tc.RollingRenewals != nil && tc.RollingRenewals.Enabled
}

// GetRollingRenewalConfig returns the rolling renewal configuration
// Returns nil if rolling renewals are not configured
func (tc *TenantConfig) GetRollingRenewalConfig() *RollingRenewalConfig {
	return tc.RollingRenewals
}

// Validate validates the RollingRenewalConfig
// Returns error if the configuration is invalid
func (rr *RollingRenewalConfig) Validate() error {
	if !rr.Enabled {
		// If disabled, no need to validate other fields
		return nil
	}

	// Validate renewWithin duration
	if rr.RenewWithin == "" {
		return fmt.Errorf("renewWithin is required when rolling renewals are enabled")
	}
	if _, err := ParseDuration(rr.RenewWithin); err != nil {
		return fmt.Errorf("invalid renewWithin duration: %w", err)
	}

	// Validate extendFor duration
	if rr.ExtendFor == "" {
		return fmt.Errorf("extendFor is required when rolling renewals are enabled")
	}
	if _, err := ParseDuration(rr.ExtendFor); err != nil {
		return fmt.Errorf("invalid extendFor duration: %w", err)
	}

	// Validate extendExpiredLimits duration (optional field)
	if rr.ExtendExpiredLimits != "" {
		if _, err := ParseDuration(rr.ExtendExpiredLimits); err != nil {
			return fmt.Errorf("invalid extendExpiredLimits duration: %w", err)
		}
	}

	// Validate allowedPatrons if selectPatrons is enabled
	if rr.SelectPatrons && len(rr.AllowedPatrons) == 0 {
		return fmt.Errorf("allowedPatrons cannot be empty when selectPatrons is true")
	}

	return nil
}

// ValidateRollingRenewals validates the rolling renewal configuration for this tenant
// Returns error if the configuration is invalid
func (tc *TenantConfig) ValidateRollingRenewals() error {
	if tc.RollingRenewals == nil {
		// No rolling renewals configured, nothing to validate
		return nil
	}

	return tc.RollingRenewals.Validate()
}

// ValidatePatronCustomFields validates the patron custom fields configuration
// Returns error if the configuration is invalid
func (tc *TenantConfig) ValidatePatronCustomFields() error {
	if tc.PatronCustomFields == nil {
		// No custom fields configured, nothing to validate
		return nil
	}

	return tc.PatronCustomFields.Validate()
}

// Validate validates the PatronCustomFieldsConfig
// Returns error if the configuration is invalid
func (pcf *PatronCustomFieldsConfig) Validate() error {
	if !pcf.Enabled {
		// If disabled, no need to validate fields
		return nil
	}

	if len(pcf.Fields) == 0 {
		// No fields configured, but enabled - this is allowed (no-op)
		return nil
	}

	// Track field codes to detect duplicates
	seenCodes := make(map[string]bool)

	for i, field := range pcf.Fields {
		// Validate field code (must be SA-SZ)
		if !isValidCustomFieldCode(field.Code) {
			return fmt.Errorf("invalid field code '%s' at index %d: must be SA-SZ", field.Code, i)
		}

		// Check for duplicate codes (case-insensitive)
		codeUpper := strings.ToUpper(field.Code)
		if seenCodes[codeUpper] {
			return fmt.Errorf("duplicate field code '%s' at index %d", field.Code, i)
		}
		seenCodes[codeUpper] = true

		// Validate source field is not empty
		if field.Source == "" {
			return fmt.Errorf("source field is required for field code '%s' at index %d", field.Code, i)
		}

		// Validate type
		if !isValidCustomFieldType(field.Type) {
			return fmt.Errorf("invalid type '%s' for field code '%s' at index %d: must be string, boolean, or array", field.Type, field.Code, i)
		}

		// Set default array delimiter if not specified
		if field.Type == "array" && field.ArrayDelimiter == "" {
			pcf.Fields[i].ArrayDelimiter = ","
		}

		// Set default max length if not specified
		if field.MaxLength <= 0 {
			pcf.Fields[i].MaxLength = 60 // Default max length
		}
	}

	return nil
}

// isValidCustomFieldCode checks if the field code is valid (SA-SZ, case-insensitive)
func isValidCustomFieldCode(code string) bool {
	if len(code) != 2 {
		return false
	}

	// First character must be 'S' or 's'
	if code[0] != 'S' && code[0] != 's' {
		return false
	}

	// Second character must be A-Z or a-z
	secondChar := code[1]
	if (secondChar >= 'A' && secondChar <= 'Z') || (secondChar >= 'a' && secondChar <= 'z') {
		return true
	}

	return false
}

// isValidCustomFieldType checks if the type is valid
func isValidCustomFieldType(fieldType string) bool {
	switch fieldType {
	case "string", "boolean", "array":
		return true
	default:
		return false
	}
}

// GetClaimedReturnedResolution returns the normalized claimed returned resolution value
// Valid values: "patron", "library", "none"
// Returns "none" for empty, invalid, or not configured values
func (tc *TenantConfig) GetClaimedReturnedResolution() string {
	switch strings.ToLower(tc.ClaimedReturnedResolution) {
	case "patron":
		return "patron"
	case "library":
		return "library"
	case "none", "":
		return "none"
	default:
		return "none" // Default to none for invalid values
	}
}

// MapClaimedReturnedResolutionToFOLIO maps the configuration value to FOLIO API format
// Returns the FOLIO-compatible string for the claimedReturnedResolution field
// Returns empty string if the resolution is "none" or not configured
func (tc *TenantConfig) MapClaimedReturnedResolutionToFOLIO() string {
	switch tc.GetClaimedReturnedResolution() {
	case "patron":
		return "Returned by patron"
	case "library":
		return "Found by library"
	default:
		return ""
	}
}
