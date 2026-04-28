// Package config manages configuration loading, validation, and hot-reload for fsip2.
//
// This package provides centralized configuration management with support for:
//   - YAML/JSON configuration files
//   - Multi-tenant configurations
//   - Environment variable overrides
//   - Configuration validation
//   - Hot-reload (automatic configuration updates without restart)
//   - Per-tenant SIP2 protocol customization
//
// # Configuration Structure
//
// The configuration hierarchy:
//
//	Config
//	├── Server (port, TLS settings)
//	├── Health (health check endpoint configuration)
//	├── Logging (log level, format)
//	├── Metrics (Prometheus endpoint)
//	├── ScanPeriod (hot-reload interval)
//	└── Tenants (map of tenant configurations)
//	    └── TenantConfig
//	        ├── OkapiURL (FOLIO API endpoint)
//	        ├── Tenant (FOLIO tenant ID)
//	        ├── SupplicantLoginTime (authentication settings)
//	        ├── SupportedMessages (enabled SIP2 message types)
//	        ├── MessageDelimiter (message terminator)
//	        ├── FieldDelimiter (field separator)
//	        ├── ErrorDetectionEnabled (checksum validation)
//	        ├── CharacterSet (encoding)
//	        ├── EnabledFields (optional field filtering per message type)
//	        ├── CirculationStatusMap (FOLIO → SIP2 item status mapping)
//	        ├── RollingRenewal (automatic renewal service settings)
//	        └── RenewAllMaxItems (maximum items for renew-all operation)
//
// # Multi-tenancy
//
// The configuration supports multiple FOLIO tenants, each with independent settings.
// Tenants are resolved at runtime using one of four resolution strategies:
//   - SC terminal tenant: Based on SC login terminal username
//   - Location code tenant: Based on CP field in LOGIN message
//   - Patron ID tenant: Based on AA field patron barcode
//   - Username tenant: Based on CO login username
//
// Example multi-tenant configuration:
//
//	tenants:
//	  TENANT1:
//	    okapiUrl: https://folio1.example.com
//	    tenant: tenant1
//	    supportedMessages: "09|11|17|23|29|35|37|63|65|93|97|99"
//	  TENANT2:
//	    okapiUrl: https://folio2.example.com
//	    tenant: tenant2
//	    supportedMessages: "09|11|23|63|93|99"
//
// # Hot-Reload
//
// Configuration hot-reload allows updating settings without restarting the server:
//   - Periodic scanning of config file controlled by the scanPeriod field
//   - scanPeriod is specified in milliseconds in the YAML config file
//   - GetScanPeriod() converts the raw integer to a time.Duration
//   - Default (if scanPeriod is unset or 0): 300000 ms (5 minutes)
//   - Minimum effective value: 1000 ms (values below this are treated as default)
//   - Automatic reload when file modification time changes
//   - Thread-safe configuration updates (RWMutex)
//   - Validation before applying new configuration
//   - Rollback on validation errors
//   - Logging of successful/failed reloads
//
// Hot-reload is useful for:
//   - Updating tenant credentials
//   - Enabling/disabling message types
//   - Adjusting timeouts or limits
//   - Modifying field mappings
//
// # Configuration Loading
//
//  1. Load YAML/JSON configuration file
//  2. Parse into Config struct
//  3. Validate configuration structure
//  4. Apply defaults for optional fields
//  5. Validate tenant configurations
//  6. Cache tenant map for fast lookups
//  7. Start hot-reload watcher (if enabled)
//
// # Field Filtering
//
// TenantConfig.IsFieldEnabled() allows per-tenant, per-message control over optional fields:
//
//	// In configuration:
//	enabledFields:
//	  "12":  # Checkout response
//	    - "BF"  # Currency type
//	    - "BH"  # Patron currency
//	  "64":  # Patron information response
//	    - "BV"  # Fee limit
//	    - "CC"  # Fee amount
//
// This enables/disables specific SIP2 fields based on kiosk vendor requirements.
//
// # Circulation Status Mapping
//
// The CirculationStatusMap translates FOLIO item statuses to SIP2 2-character codes:
//   - "01": On order
//   - "02": Available
//   - "03": Charged (checked out)
//   - "04": Charged, not to be recalled
//   - "05": In process
//   - "06": Recalled
//   - "07": Waiting on hold shelf
//   - "08": Waiting to be reshelved
//   - "09": In transit
//   - "10": Claimed returned
//   - "11": Lost
//   - "12": Missing
//   - "13": In repair
//
// Example mapping:
//
//	circulationStatusMap:
//	  "Available": "02"
//	  "Checked out": "03"
//	  "On order": "01"
//	  "In transit": "09"
//
// # Configuration Validation
//
// The package validates:
//   - Required fields are present
//   - Port numbers are valid (1-65535)
//   - URLs are well-formed
//   - Message type codes are valid SIP2 types
//   - Field codes are valid SIP2 field identifiers
//   - Circulation status codes are valid (01-13)
//   - Tenant IDs are unique
//   - Delimiters are non-empty
//
// # Usage Example
//
//	// Load configuration
//	cfg, err := config.Load("config.yaml")
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Start hot-reload
//	ctx, cancel := context.WithCancel(context.Background())
//	defer cancel()
//	go config.StartHotReload(ctx, cfg, "config.yaml", logger)
//
//	// Access tenant configuration
//	tenantCfg := cfg.Tenants["TENANT1"]
//	okapiURL := tenantCfg.OkapiURL
//
//	// Check if message type is supported
//	if !tenantCfg.IsMessageSupported("23") {
//	    return errors.New("message type not supported")
//	}
//
//	// Check if field is enabled for message type
//	if tenantCfg.IsFieldEnabled("12", "BF") {
//	    // Include currency type in checkout response
//	}
//
//	// Map FOLIO status to SIP2 code
//	sip2Status := tenantCfg.MapCirculationStatus("Checked out") // Returns "03"
package config
