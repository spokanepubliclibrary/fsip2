package config

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"
)

// Reloader handles periodic reloading of configuration
type Reloader struct {
	config    *Config
	loaders   []TenantConfigLoader
	interval  time.Duration
	onChange  func(*Config)
	stopCh    chan struct{}
	stoppedCh chan struct{}
	mu        sync.RWMutex
	isRunning bool
}

// NewReloader creates a new configuration reloader
func NewReloader(cfg *Config, onChange func(*Config)) *Reloader {
	return &Reloader{
		config:    cfg,
		interval:  cfg.GetScanPeriod(),
		onChange:  onChange,
		stopCh:    make(chan struct{}),
		stoppedCh: make(chan struct{}),
	}
}

// Start starts the configuration reloader
func (r *Reloader) Start(ctx context.Context) error {
	r.mu.Lock()
	if r.isRunning {
		r.mu.Unlock()
		return fmt.Errorf("reloader is already running")
	}
	r.isRunning = true
	r.mu.Unlock()

	// Initialize loaders from config sources
	if err := r.initializeLoaders(); err != nil {
		r.mu.Lock()
		r.isRunning = false
		r.mu.Unlock()
		return fmt.Errorf("failed to initialize loaders: %w", err)
	}

	// Start reload loop
	go r.reloadLoop(ctx)

	return nil
}

// Stop stops the configuration reloader
func (r *Reloader) Stop() {
	r.mu.Lock()
	if !r.isRunning {
		r.mu.Unlock()
		return
	}
	r.mu.Unlock()

	close(r.stopCh)
	<-r.stoppedCh

	r.mu.Lock()
	r.isRunning = false
	r.mu.Unlock()
}

// IsRunning returns whether the reloader is currently running
func (r *Reloader) IsRunning() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.isRunning
}

// initializeLoaders creates loaders for all configured sources
func (r *Reloader) initializeLoaders() error {
	r.loaders = []TenantConfigLoader{}

	for _, source := range r.config.TenantConfigSources {
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

		r.loaders = append(r.loaders, loader)
	}

	return nil
}

// reloadLoop runs the periodic reload loop
func (r *Reloader) reloadLoop(ctx context.Context) {
	defer close(r.stoppedCh)

	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-r.stopCh:
			return
		case <-ticker.C:
			if err := r.reload(); err != nil {
				// Log error but continue (don't stop reloader on error)
				// In a real implementation, this would use the logger
				continue
			}
		}
	}
}

// reload performs a configuration reload
func (r *Reloader) reload() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Load tenant configs from all sources
	newTenants := make(map[string]*TenantConfig)
	var newSCTenants []SCTenantConfig

	for _, loader := range r.loaders {
		tenantCfgs, scTenants, err := loader.Load()
		if err != nil {
			// Log warning but continue with other sources
			continue
		}

		for _, tenantCfg := range tenantCfgs {
			newTenants[tenantCfg.Tenant] = tenantCfg
		}

		newSCTenants = append(newSCTenants, scTenants...)
	}

	// Check if configuration has changed
	if r.hasChanged(newTenants) {
		// Update configuration
		r.config.Tenants = newTenants
		r.config.SCTenants = newSCTenants

		// Call onChange callback if provided
		if r.onChange != nil {
			r.onChange(r.config)
		}
	}

	return nil
}

// configChange represents a single configuration change
type configChange struct {
	tenant   string
	field    string
	oldValue string
	newValue string
}

// deepCompareTenants performs a deep comparison of two TenantConfig structs
// Returns true if they are equal, false otherwise
func deepCompareTenants(old, new *TenantConfig) bool {
	return reflect.DeepEqual(old, new)
}

// findConfigChanges compares old and new tenant configurations and returns a list of changes
func findConfigChanges(oldTenants, newTenants map[string]*TenantConfig) []configChange {
	var changes []configChange

	// Find added tenants
	for tenantName := range newTenants {
		if _, exists := oldTenants[tenantName]; !exists {
			changes = append(changes, configChange{
				tenant:   tenantName,
				field:    "tenant",
				oldValue: "",
				newValue: "added",
			})
		}
	}

	// Find removed tenants
	for tenantName := range oldTenants {
		if _, exists := newTenants[tenantName]; !exists {
			changes = append(changes, configChange{
				tenant:   tenantName,
				field:    "tenant",
				oldValue: "removed",
				newValue: "",
			})
		}
	}

	// Find modified tenants
	for tenantName, newCfg := range newTenants {
		if oldCfg, exists := oldTenants[tenantName]; exists {
			if !deepCompareTenants(oldCfg, newCfg) {
				// Tenant exists in both but has changes
				// Compare individual fields to identify what changed
				changes = append(changes, compareFields(tenantName, oldCfg, newCfg)...)
			}
		}
	}

	return changes
}

// compareFields compares individual fields between two tenant configs
func compareFields(tenantName string, old, new *TenantConfig) []configChange {
	var changes []configChange

	if old.ErrorDetectionEnabled != new.ErrorDetectionEnabled {
		changes = append(changes, configChange{
			tenant:   tenantName,
			field:    "ErrorDetectionEnabled",
			oldValue: fmt.Sprintf("%t", old.ErrorDetectionEnabled),
			newValue: fmt.Sprintf("%t", new.ErrorDetectionEnabled),
		})
	}

	if old.MessageDelimiter != new.MessageDelimiter {
		changes = append(changes, configChange{
			tenant:   tenantName,
			field:    "MessageDelimiter",
			oldValue: escapeDelimiter(old.MessageDelimiter),
			newValue: escapeDelimiter(new.MessageDelimiter),
		})
	}

	if old.FieldDelimiter != new.FieldDelimiter {
		changes = append(changes, configChange{
			tenant:   tenantName,
			field:    "FieldDelimiter",
			oldValue: escapeDelimiter(old.FieldDelimiter),
			newValue: escapeDelimiter(new.FieldDelimiter),
		})
	}

	if old.Charset != new.Charset {
		changes = append(changes, configChange{
			tenant:   tenantName,
			field:    "Charset",
			oldValue: old.Charset,
			newValue: new.Charset,
		})
	}

	if old.Timezone != new.Timezone {
		changes = append(changes, configChange{
			tenant:   tenantName,
			field:    "Timezone",
			oldValue: old.Timezone,
			newValue: new.Timezone,
		})
	}

	if old.LogLevel != new.LogLevel {
		changes = append(changes, configChange{
			tenant:   tenantName,
			field:    "LogLevel",
			oldValue: old.LogLevel,
			newValue: new.LogLevel,
		})
	}

	if old.OkapiURL != new.OkapiURL {
		changes = append(changes, configChange{
			tenant:   tenantName,
			field:    "OkapiURL",
			oldValue: old.OkapiURL,
			newValue: new.OkapiURL,
		})
	}

	if old.OkapiTenant != new.OkapiTenant {
		changes = append(changes, configChange{
			tenant:   tenantName,
			field:    "OkapiTenant",
			oldValue: old.OkapiTenant,
			newValue: new.OkapiTenant,
		})
	}

	if old.PatronPasswordVerificationRequired != new.PatronPasswordVerificationRequired {
		changes = append(changes, configChange{
			tenant:   tenantName,
			field:    "PatronPasswordVerificationRequired",
			oldValue: fmt.Sprintf("%t", old.PatronPasswordVerificationRequired),
			newValue: fmt.Sprintf("%t", new.PatronPasswordVerificationRequired),
		})
	}

	if old.UsePinForPatronVerification != new.UsePinForPatronVerification {
		changes = append(changes, configChange{
			tenant:   tenantName,
			field:    "UsePinForPatronVerification",
			oldValue: fmt.Sprintf("%t", old.UsePinForPatronVerification),
			newValue: fmt.Sprintf("%t", new.UsePinForPatronVerification),
		})
	}

	if old.InvalidCheckinStatuses != new.InvalidCheckinStatuses {
		changes = append(changes, configChange{
			tenant:   tenantName,
			field:    "InvalidCheckinStatuses",
			oldValue: old.InvalidCheckinStatuses,
			newValue: new.InvalidCheckinStatuses,
		})
	}

	if old.ClaimedReturnedResolution != new.ClaimedReturnedResolution {
		changes = append(changes, configChange{
			tenant:   tenantName,
			field:    "ClaimedReturnedResolution",
			oldValue: old.ClaimedReturnedResolution,
			newValue: new.ClaimedReturnedResolution,
		})
	}

	if old.StatusUpdateOk != new.StatusUpdateOk {
		changes = append(changes, configChange{
			tenant:   tenantName,
			field:    "StatusUpdateOk",
			oldValue: fmt.Sprintf("%t", old.StatusUpdateOk),
			newValue: fmt.Sprintf("%t", new.StatusUpdateOk),
		})
	}

	if old.OfflineOk != new.OfflineOk {
		changes = append(changes, configChange{
			tenant:   tenantName,
			field:    "OfflineOk",
			oldValue: fmt.Sprintf("%t", old.OfflineOk),
			newValue: fmt.Sprintf("%t", new.OfflineOk),
		})
	}

	if old.TimeoutPeriod != new.TimeoutPeriod {
		changes = append(changes, configChange{
			tenant:   tenantName,
			field:    "TimeoutPeriod",
			oldValue: fmt.Sprintf("%d", old.TimeoutPeriod),
			newValue: fmt.Sprintf("%d", new.TimeoutPeriod),
		})
	}

	if old.RetriesAllowed != new.RetriesAllowed {
		changes = append(changes, configChange{
			tenant:   tenantName,
			field:    "RetriesAllowed",
			oldValue: fmt.Sprintf("%d", old.RetriesAllowed),
			newValue: fmt.Sprintf("%d", new.RetriesAllowed),
		})
	}

	if old.Currency != new.Currency {
		changes = append(changes, configChange{
			tenant:   tenantName,
			field:    "Currency",
			oldValue: old.Currency,
			newValue: new.Currency,
		})
	}

	if old.RenewAllMaxItems != new.RenewAllMaxItems {
		changes = append(changes, configChange{
			tenant:   tenantName,
			field:    "RenewAllMaxItems",
			oldValue: fmt.Sprintf("%d", old.RenewAllMaxItems),
			newValue: fmt.Sprintf("%d", new.RenewAllMaxItems),
		})
	}

	if old.AcceptBulkPayment != new.AcceptBulkPayment {
		changes = append(changes, configChange{
			tenant:   tenantName,
			field:    "AcceptBulkPayment",
			oldValue: fmt.Sprintf("%t", old.AcceptBulkPayment),
			newValue: fmt.Sprintf("%t", new.AcceptBulkPayment),
		})
	}

	if old.PaymentMethod != new.PaymentMethod {
		changes = append(changes, configChange{
			tenant:   tenantName,
			field:    "PaymentMethod",
			oldValue: old.PaymentMethod,
			newValue: new.PaymentMethod,
		})
	}

	if old.NotifyPatron != new.NotifyPatron {
		changes = append(changes, configChange{
			tenant:   tenantName,
			field:    "NotifyPatron",
			oldValue: fmt.Sprintf("%t", old.NotifyPatron),
			newValue: fmt.Sprintf("%t", new.NotifyPatron),
		})
	}

	// Compare complex nested structures using deep equal
	if !reflect.DeepEqual(old.SupportedMessages, new.SupportedMessages) {
		changes = append(changes, configChange{
			tenant:   tenantName,
			field:    "SupportedMessages",
			oldValue: fmt.Sprintf("%d messages", len(old.SupportedMessages)),
			newValue: fmt.Sprintf("%d messages", len(new.SupportedMessages)),
		})
	}

	if !reflect.DeepEqual(old.CirculationStatusMapping, new.CirculationStatusMapping) {
		changes = append(changes, configChange{
			tenant:   tenantName,
			field:    "CirculationStatusMapping",
			oldValue: fmt.Sprintf("%d mappings", len(old.CirculationStatusMapping)),
			newValue: fmt.Sprintf("%d mappings", len(new.CirculationStatusMapping)),
		})
	}

	if !reflect.DeepEqual(old.RollingRenewals, new.RollingRenewals) {
		changes = append(changes, configChange{
			tenant:   tenantName,
			field:    "RollingRenewals",
			oldValue: "changed",
			newValue: "changed",
		})
	}

	if !reflect.DeepEqual(old.PatronCustomFields, new.PatronCustomFields) {
		changes = append(changes, configChange{
			tenant:   tenantName,
			field:    "PatronCustomFields",
			oldValue: "changed",
			newValue: "changed",
		})
	}

	return changes
}

// escapeDelimiter converts delimiter strings to readable format for logging
func escapeDelimiter(s string) string {
	s = strings.ReplaceAll(s, "\r", "\\r")
	s = strings.ReplaceAll(s, "\n", "\\n")
	s = strings.ReplaceAll(s, "\t", "\\t")
	return s
}

// hasChanged checks if the tenant configuration has changed
func (r *Reloader) hasChanged(newTenants map[string]*TenantConfig) bool {
	// Find all configuration changes
	changes := findConfigChanges(r.config.Tenants, newTenants)

	// Log changes if any were found
	if len(changes) > 0 {
		r.logConfigChanges(changes)
		return true
	}

	return false
}

// logConfigChanges logs all configuration changes
func (r *Reloader) logConfigChanges(changes []configChange) {
	// In production, this would use the actual logger
	// For now, we just prepare the log messages

	for _, change := range changes {
		if change.field == "tenant" {
			if change.newValue == "added" {
				// Log: Tenant 'xxx' added
				_ = fmt.Sprintf("Tenant '%s' added", change.tenant)
			} else if change.oldValue == "removed" {
				// Log: Tenant 'xxx' removed
				_ = fmt.Sprintf("Tenant '%s' removed", change.tenant)
			}
		} else {
			// Log: Tenant 'xxx' configuration changed: FieldName 'oldValue' → 'newValue'
			_ = fmt.Sprintf("Tenant '%s' configuration changed: %s '%s' → '%s'",
				change.tenant, change.field, change.oldValue, change.newValue)
		}
	}
}

// TriggerReload manually triggers a configuration reload
func (r *Reloader) TriggerReload() error {
	return r.reload()
}

// GetCurrentConfig returns the current configuration
func (r *Reloader) GetCurrentConfig() *Config {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.config
}

// UpdateInterval updates the reload interval
func (r *Reloader) UpdateInterval(interval time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.interval = interval
}
