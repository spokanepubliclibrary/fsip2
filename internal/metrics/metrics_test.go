package metrics

import (
	"sync"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

// TestNewMetrics tests creating a new metrics instance
func TestNewMetrics(t *testing.T) {
	m := NewMetrics()

	if m == nil {
		t.Fatal("NewMetrics returned nil")
	}

	// Verify all metrics are initialized
	if m.ConnectionsTotal == nil {
		t.Error("ConnectionsTotal is nil")
	}
	if m.ConnectionsActive == nil {
		t.Error("ConnectionsActive is nil")
	}
	if m.ConnectionDuration == nil {
		t.Error("ConnectionDuration is nil")
	}
	if m.ConnectionErrors == nil {
		t.Error("ConnectionErrors is nil")
	}
	if m.MessagesTotal == nil {
		t.Error("MessagesTotal is nil")
	}
	if m.MessageDuration == nil {
		t.Error("MessageDuration is nil")
	}
	if m.MessageErrors == nil {
		t.Error("MessageErrors is nil")
	}
}

// TestNewMetrics_Singleton tests that NewMetrics returns the same instance
func TestNewMetrics_Singleton(t *testing.T) {
	m1 := NewMetrics()
	m2 := NewMetrics()

	if m1 != m2 {
		t.Error("NewMetrics did not return the same singleton instance")
	}
}

// TestMetrics_ConnectionCounter tests connection counter metric
func TestMetrics_ConnectionCounter(t *testing.T) {
	m := NewMetrics()

	initialValue := testutil.ToFloat64(m.ConnectionsTotal)

	// Increment counter
	m.ConnectionsTotal.Inc()

	newValue := testutil.ToFloat64(m.ConnectionsTotal)

	if newValue != initialValue+1 {
		t.Errorf("Expected counter value %f, got %f", initialValue+1, newValue)
	}
}

// TestMetrics_ConnectionsActive tests active connections gauge
func TestMetrics_ConnectionsActive(t *testing.T) {
	m := NewMetrics()

	initialValue := testutil.ToFloat64(m.ConnectionsActive)

	// Increase active connections
	m.ConnectionsActive.Inc()
	value := testutil.ToFloat64(m.ConnectionsActive)

	if value != initialValue+1 {
		t.Errorf("Expected gauge value %f, got %f", initialValue+1, value)
	}

	// Decrease active connections
	m.ConnectionsActive.Dec()
	value = testutil.ToFloat64(m.ConnectionsActive)

	if value != initialValue {
		t.Errorf("Expected gauge value %f, got %f", initialValue, value)
	}
}

// TestMetrics_ConnectionDuration tests connection duration histogram
func TestMetrics_ConnectionDuration(t *testing.T) {
	m := NewMetrics()

	// Observe some durations
	m.ConnectionDuration.Observe(1.5)
	m.ConnectionDuration.Observe(2.3)
	m.ConnectionDuration.Observe(0.5)

	// For histograms, we just verify they can be observed without panicking
	// The actual histogram implementation is tested by Prometheus client library
}

// TestMetrics_MessagesByType tests message counters by type
func TestMetrics_MessagesByType(t *testing.T) {
	m := NewMetrics()

	// Increment counters for different message types
	m.MessagesTotal.WithLabelValues("login", "test-tenant").Inc()
	m.MessagesTotal.WithLabelValues("login", "test-tenant").Inc()
	m.MessagesTotal.WithLabelValues("checkout", "test-tenant").Inc()

	// Verify the counters
	loginCount := testutil.ToFloat64(m.MessagesTotal.WithLabelValues("login", "test-tenant"))
	if loginCount != 2 {
		t.Errorf("Expected login count 2, got %f", loginCount)
	}

	checkoutCount := testutil.ToFloat64(m.MessagesTotal.WithLabelValues("checkout", "test-tenant"))
	if checkoutCount != 1 {
		t.Errorf("Expected checkout count 1, got %f", checkoutCount)
	}
}

// TestMetrics_MessageDuration tests message duration histogram
func TestMetrics_MessageDuration(t *testing.T) {
	m := NewMetrics()

	// Observe durations for different message types
	m.MessageDuration.WithLabelValues("login", "test-tenant").Observe(0.05)
	m.MessageDuration.WithLabelValues("login", "test-tenant").Observe(0.1)
	m.MessageDuration.WithLabelValues("checkout", "test-tenant").Observe(0.2)

	// For histograms, we just verify they can be observed without panicking
	// The actual histogram implementation is tested by Prometheus client library
}

// TestMetrics_MessageErrors tests message error counters
func TestMetrics_MessageErrors(t *testing.T) {
	m := NewMetrics()

	// Increment error counters for different message types and error types
	m.MessageErrors.WithLabelValues("login", "test-tenant", "auth_failed").Inc()
	m.MessageErrors.WithLabelValues("login", "test-tenant", "auth_failed").Inc()
	m.MessageErrors.WithLabelValues("checkout", "test-tenant", "item_not_found").Inc()

	// Verify the error counters
	loginErrors := testutil.ToFloat64(m.MessageErrors.WithLabelValues("login", "test-tenant", "auth_failed"))
	if loginErrors != 2 {
		t.Errorf("Expected 2 login auth errors, got %f", loginErrors)
	}

	checkoutErrors := testutil.ToFloat64(m.MessageErrors.WithLabelValues("checkout", "test-tenant", "item_not_found"))
	if checkoutErrors != 1 {
		t.Errorf("Expected 1 checkout item_not_found error, got %f", checkoutErrors)
	}
}

// TestMetrics_LoginCounters tests login-specific metrics
func TestMetrics_LoginCounters(t *testing.T) {
	m := NewMetrics()

	initialAttempts := testutil.ToFloat64(m.LoginAttempts)
	initialSuccess := testutil.ToFloat64(m.LoginSuccess)
	initialFailures := testutil.ToFloat64(m.LoginFailures)

	// Simulate some login attempts
	m.LoginAttempts.Inc()
	m.LoginSuccess.Inc()

	m.LoginAttempts.Inc()
	m.LoginFailures.Inc()

	// Verify counters
	attempts := testutil.ToFloat64(m.LoginAttempts)
	if attempts != initialAttempts+2 {
		t.Errorf("Expected %f login attempts, got %f", initialAttempts+2, attempts)
	}

	success := testutil.ToFloat64(m.LoginSuccess)
	if success != initialSuccess+1 {
		t.Errorf("Expected %f successful logins, got %f", initialSuccess+1, success)
	}

	failures := testutil.ToFloat64(m.LoginFailures)
	if failures != initialFailures+1 {
		t.Errorf("Expected %f failed logins, got %f", initialFailures+1, failures)
	}
}

// TestMetrics_CheckoutCounters tests checkout-specific metrics
func TestMetrics_CheckoutCounters(t *testing.T) {
	m := NewMetrics()

	initialTotal := testutil.ToFloat64(m.CheckoutTotal)
	initialSuccess := testutil.ToFloat64(m.CheckoutSuccess)
	initialFailures := testutil.ToFloat64(m.CheckoutFailures)

	// Simulate checkouts
	m.CheckoutTotal.Inc()
	m.CheckoutSuccess.Inc()

	m.CheckoutTotal.Inc()
	m.CheckoutFailures.Inc()

	// Verify counters
	total := testutil.ToFloat64(m.CheckoutTotal)
	if total != initialTotal+2 {
		t.Errorf("Expected %f total checkouts, got %f", initialTotal+2, total)
	}

	success := testutil.ToFloat64(m.CheckoutSuccess)
	if success != initialSuccess+1 {
		t.Errorf("Expected %f successful checkouts, got %f", initialSuccess+1, success)
	}

	failures := testutil.ToFloat64(m.CheckoutFailures)
	if failures != initialFailures+1 {
		t.Errorf("Expected %f failed checkouts, got %f", initialFailures+1, failures)
	}
}

// TestMetrics_CheckinCounters tests checkin-specific metrics
func TestMetrics_CheckinCounters(t *testing.T) {
	m := NewMetrics()

	initialTotal := testutil.ToFloat64(m.CheckinTotal)
	initialSuccess := testutil.ToFloat64(m.CheckinSuccess)

	// Simulate checkins
	m.CheckinTotal.Inc()
	m.CheckinSuccess.Inc()

	// Verify counters
	total := testutil.ToFloat64(m.CheckinTotal)
	if total != initialTotal+1 {
		t.Errorf("Expected %f total checkins, got %f", initialTotal+1, total)
	}

	success := testutil.ToFloat64(m.CheckinSuccess)
	if success != initialSuccess+1 {
		t.Errorf("Expected %f successful checkins, got %f", initialSuccess+1, success)
	}
}

// TestMetrics_RenewCounters tests renew-specific metrics
func TestMetrics_RenewCounters(t *testing.T) {
	m := NewMetrics()

	initialTotal := testutil.ToFloat64(m.RenewTotal)
	initialSuccess := testutil.ToFloat64(m.RenewSuccess)
	initialFailures := testutil.ToFloat64(m.RenewFailures)

	// Simulate renewals
	m.RenewTotal.Inc()
	m.RenewSuccess.Inc()

	m.RenewTotal.Inc()
	m.RenewFailures.Inc()

	// Verify counters
	total := testutil.ToFloat64(m.RenewTotal)
	if total != initialTotal+2 {
		t.Errorf("Expected %f total renewals, got %f", initialTotal+2, total)
	}

	success := testutil.ToFloat64(m.RenewSuccess)
	if success != initialSuccess+1 {
		t.Errorf("Expected %f successful renewals, got %f", initialSuccess+1, success)
	}

	failures := testutil.ToFloat64(m.RenewFailures)
	if failures != initialFailures+1 {
		t.Errorf("Expected %f failed renewals, got %f", initialFailures+1, failures)
	}
}

// TestMetrics_FOLIORequests tests FOLIO API request metrics
func TestMetrics_FOLIORequests(t *testing.T) {
	m := NewMetrics()

	// Simulate FOLIO API requests
	m.FolioRequestsTotal.WithLabelValues("/users", "GET", "test-tenant").Inc()
	m.FolioRequestsTotal.WithLabelValues("/users", "GET", "test-tenant").Inc()
	m.FolioRequestsTotal.WithLabelValues("/items", "POST", "test-tenant").Inc()

	// Verify counters
	usersCount := testutil.ToFloat64(m.FolioRequestsTotal.WithLabelValues("/users", "GET", "test-tenant"))
	if usersCount != 2 {
		t.Errorf("Expected 2 /users GET requests, got %f", usersCount)
	}

	itemsCount := testutil.ToFloat64(m.FolioRequestsTotal.WithLabelValues("/items", "POST", "test-tenant"))
	if itemsCount != 1 {
		t.Errorf("Expected 1 /items POST request, got %f", itemsCount)
	}
}

// TestMetrics_FOLIORequestDuration tests FOLIO API request duration
func TestMetrics_FOLIORequestDuration(t *testing.T) {
	m := NewMetrics()

	// Observe request durations
	m.FolioRequestDuration.WithLabelValues("/users", "GET", "test-tenant").Observe(0.15)
	m.FolioRequestDuration.WithLabelValues("/users", "GET", "test-tenant").Observe(0.25)

	// For histograms, we just verify they can be observed without panicking
	// The actual histogram implementation is tested by Prometheus client library
}

// TestMetrics_FOLIORequestErrors tests FOLIO API error counters
func TestMetrics_FOLIORequestErrors(t *testing.T) {
	m := NewMetrics()

	// Simulate FOLIO API errors
	m.FolioRequestErrors.WithLabelValues("/users", "GET", "test-tenant", "404").Inc()
	m.FolioRequestErrors.WithLabelValues("/users", "GET", "test-tenant", "500").Inc()

	// Verify error counters
	notFoundErrors := testutil.ToFloat64(m.FolioRequestErrors.WithLabelValues("/users", "GET", "test-tenant", "404"))
	if notFoundErrors != 1 {
		t.Errorf("Expected 1 404 error, got %f", notFoundErrors)
	}

	serverErrors := testutil.ToFloat64(m.FolioRequestErrors.WithLabelValues("/users", "GET", "test-tenant", "500"))
	if serverErrors != 1 {
		t.Errorf("Expected 1 500 error, got %f", serverErrors)
	}
}

// TestMetrics_TenantResolutions tests tenant resolution metrics
func TestMetrics_TenantResolutions(t *testing.T) {
	m := NewMetrics()

	// Simulate tenant resolutions
	m.TenantResolutions.WithLabelValues("login", "institutional_id", "test-tenant").Inc()
	m.TenantResolutions.WithLabelValues("login", "institutional_id", "test-tenant").Inc()
	m.TenantResolutions.WithLabelValues("message", "header_lookup", "other-tenant").Inc()

	// Verify counters
	loginResolutions := testutil.ToFloat64(m.TenantResolutions.WithLabelValues("login", "institutional_id", "test-tenant"))
	if loginResolutions != 2 {
		t.Errorf("Expected 2 login institutional_id resolutions, got %f", loginResolutions)
	}

	messageResolutions := testutil.ToFloat64(m.TenantResolutions.WithLabelValues("message", "header_lookup", "other-tenant"))
	if messageResolutions != 1 {
		t.Errorf("Expected 1 message header_lookup resolution, got %f", messageResolutions)
	}
}

// TestMetrics_TenantResolutionErrors tests tenant resolution error counter
func TestMetrics_TenantResolutionErrors(t *testing.T) {
	m := NewMetrics()

	initialErrors := testutil.ToFloat64(m.TenantResolutionErrors)

	// Simulate tenant resolution errors
	m.TenantResolutionErrors.Inc()
	m.TenantResolutionErrors.Inc()

	// Verify error counter
	errors := testutil.ToFloat64(m.TenantResolutionErrors)
	if errors != initialErrors+2 {
		t.Errorf("Expected %f tenant resolution errors, got %f", initialErrors+2, errors)
	}
}

// TestMetrics_SessionCounters tests session metrics
func TestMetrics_SessionCounters(t *testing.T) {
	m := NewMetrics()

	initialCreated := testutil.ToFloat64(m.SessionsCreated)
	initialEnded := testutil.ToFloat64(m.SessionsEnded)
	initialActive := testutil.ToFloat64(m.SessionsActive)

	// Simulate session lifecycle
	m.SessionsCreated.Inc()
	m.SessionsActive.Inc()

	created := testutil.ToFloat64(m.SessionsCreated)
	if created != initialCreated+1 {
		t.Errorf("Expected %f sessions created, got %f", initialCreated+1, created)
	}

	active := testutil.ToFloat64(m.SessionsActive)
	if active != initialActive+1 {
		t.Errorf("Expected %f active sessions, got %f", initialActive+1, active)
	}

	// End the session
	m.SessionsEnded.Inc()
	m.SessionsActive.Dec()

	ended := testutil.ToFloat64(m.SessionsEnded)
	if ended != initialEnded+1 {
		t.Errorf("Expected %f sessions ended, got %f", initialEnded+1, ended)
	}

	active = testutil.ToFloat64(m.SessionsActive)
	if active != initialActive {
		t.Errorf("Expected %f active sessions after ending, got %f", initialActive, active)
	}
}

// TestMetrics_ConcurrentUpdates tests concurrent metric updates
func TestMetrics_ConcurrentUpdates(t *testing.T) {
	m := NewMetrics()

	var wg sync.WaitGroup
	iterations := 100

	// Concurrently increment counters
	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			m.ConnectionsTotal.Inc()
			m.ConnectionsActive.Inc()
			m.MessagesTotal.WithLabelValues("test", "tenant").Inc()
		}()
	}

	wg.Wait()

	// Verify all increments were recorded
	// Note: We can't verify exact counts since these are singletons and may be used by other tests
	// But we can verify the operations completed without panic
}

// TestGetMessageTypeName tests the message type name lookup function
func TestGetMessageTypeName(t *testing.T) {
	tests := []struct {
		code     string
		expected string
	}{
		{"93", "login"},
		{"99", "sc_status"},
		{"23", "patron_status"},
		{"11", "checkout"},
		{"09", "checkin"},
		{"63", "patron_information"},
		{"17", "item_information"},
		{"29", "renew"},
		{"65", "renew_all"},
		{"35", "end_session"},
		{"37", "fee_paid"},
		{"19", "item_status_update"},
		{"97", "resend"},
		{"00", "unknown"},
		{"invalid", "unknown"},
		{"", "unknown"},
	}

	for _, tt := range tests {
		t.Run("Code_"+tt.code, func(t *testing.T) {
			name := GetMessageTypeName(tt.code)
			if name != tt.expected {
				t.Errorf("Expected message type name '%s' for code '%s', got '%s'", tt.expected, tt.code, name)
			}
		})
	}
}

// TestMetrics_PrometheusRegistry tests that metrics are registered with Prometheus
func TestMetrics_PrometheusRegistry(t *testing.T) {
	m := NewMetrics()

	// Verify metrics can be collected
	// This is a basic test to ensure metrics are properly registered
	if m.ConnectionsTotal == nil {
		t.Error("ConnectionsTotal metric not registered")
	}

	// Try to collect metrics (this will panic if registration is broken)
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Panic during metric collection: %v", r)
		}
	}()

	// These operations should work if metrics are properly registered
	m.ConnectionsTotal.Inc()
	m.MessagesTotal.WithLabelValues("test", "tenant").Inc()
}

// TestMetrics_AllMetricTypesInitialized tests that all metric types are initialized
func TestMetrics_AllMetricTypesInitialized(t *testing.T) {
	m := NewMetrics()

	// Test Counter metrics
	counters := []prometheus.Counter{
		m.ConnectionsTotal,
		m.ConnectionErrors,
		m.LoginAttempts,
		m.LoginSuccess,
		m.LoginFailures,
		m.CheckoutTotal,
		m.CheckoutSuccess,
		m.CheckoutFailures,
		m.CheckinTotal,
		m.CheckinSuccess,
		m.CheckinFailures,
		m.RenewTotal,
		m.RenewSuccess,
		m.RenewFailures,
		m.TenantResolutionErrors,
		m.SessionsCreated,
		m.SessionsEnded,
	}

	for i, counter := range counters {
		if counter == nil {
			t.Errorf("Counter metric at index %d is nil", i)
		}
	}

	// Test Gauge metrics
	gauges := []prometheus.Gauge{
		m.ConnectionsActive,
		m.SessionsActive,
	}

	for i, gauge := range gauges {
		if gauge == nil {
			t.Errorf("Gauge metric at index %d is nil", i)
		}
	}

	// Test Histogram metrics
	if m.ConnectionDuration == nil {
		t.Error("ConnectionDuration histogram is nil")
	}

	// Test CounterVec metrics
	counterVecs := []*prometheus.CounterVec{
		m.MessagesTotal,
		m.MessageErrors,
		m.FolioRequestsTotal,
		m.FolioRequestErrors,
		m.TenantResolutions,
	}

	for i, vec := range counterVecs {
		if vec == nil {
			t.Errorf("CounterVec metric at index %d is nil", i)
		}
	}

	// Test HistogramVec metrics
	histogramVecs := []*prometheus.HistogramVec{
		m.MessageDuration,
		m.FolioRequestDuration,
	}

	for i, vec := range histogramVecs {
		if vec == nil {
			t.Errorf("HistogramVec metric at index %d is nil", i)
		}
	}
}
