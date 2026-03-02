package metrics

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// Global singleton metrics instance
	globalMetrics *Metrics
	metricsOnce   sync.Once
)

// Metrics holds all Prometheus metrics for the SIP2 server
type Metrics struct {
	// Connection metrics
	ConnectionsTotal   prometheus.Counter
	ConnectionsActive  prometheus.Gauge
	ConnectionDuration prometheus.Histogram
	ConnectionErrors   prometheus.Counter

	// Message metrics
	MessagesTotal   *prometheus.CounterVec
	MessageDuration *prometheus.HistogramVec
	MessageErrors   *prometheus.CounterVec

	// Handler-specific metrics
	LoginAttempts prometheus.Counter
	LoginSuccess  prometheus.Counter
	LoginFailures prometheus.Counter

	CheckoutTotal    prometheus.Counter
	CheckoutSuccess  prometheus.Counter
	CheckoutFailures prometheus.Counter

	CheckinTotal    prometheus.Counter
	CheckinSuccess  prometheus.Counter
	CheckinFailures prometheus.Counter

	RenewTotal    prometheus.Counter
	RenewSuccess  prometheus.Counter
	RenewFailures prometheus.Counter

	// FOLIO API metrics
	FolioRequestsTotal   *prometheus.CounterVec
	FolioRequestDuration *prometheus.HistogramVec
	FolioRequestErrors   *prometheus.CounterVec

	// Tenant metrics
	TenantResolutions      *prometheus.CounterVec
	TenantResolutionErrors prometheus.Counter

	// Session metrics
	SessionsCreated prometheus.Counter
	SessionsEnded   prometheus.Counter
	SessionsActive  prometheus.Gauge
}

// NewMetrics creates and registers all Prometheus metrics
// Uses sync.Once to ensure metrics are only registered once globally
func NewMetrics() *Metrics {
	metricsOnce.Do(func() {
		globalMetrics = &Metrics{
			// Connection metrics
			ConnectionsTotal: promauto.NewCounter(prometheus.CounterOpts{
				Name: "fsip2_connections_total",
				Help: "Total number of SIP2 connections established",
			}),
			ConnectionsActive: promauto.NewGauge(prometheus.GaugeOpts{
				Name: "fsip2_connections_active",
				Help: "Current number of active SIP2 connections",
			}),
			ConnectionDuration: promauto.NewHistogram(prometheus.HistogramOpts{
				Name:    "fsip2_connection_duration_seconds",
				Help:    "Duration of SIP2 connections in seconds",
				Buckets: prometheus.DefBuckets,
			}),
			ConnectionErrors: promauto.NewCounter(prometheus.CounterOpts{
				Name: "fsip2_connection_errors_total",
				Help: "Total number of SIP2 connection errors",
			}),

			// Message metrics
			MessagesTotal: promauto.NewCounterVec(
				prometheus.CounterOpts{
					Name: "fsip2_messages_total",
					Help: "Total number of SIP2 messages processed by message type",
				},
				[]string{"message_type", "tenant"},
			),
			MessageDuration: promauto.NewHistogramVec(
				prometheus.HistogramOpts{
					Name:    "fsip2_message_duration_seconds",
					Help:    "Duration of SIP2 message processing in seconds",
					Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5},
				},
				[]string{"message_type", "tenant"},
			),
			MessageErrors: promauto.NewCounterVec(
				prometheus.CounterOpts{
					Name: "fsip2_message_errors_total",
					Help: "Total number of SIP2 message errors by message type",
				},
				[]string{"message_type", "tenant", "error_type"},
			),

			// Handler-specific metrics
			LoginAttempts: promauto.NewCounter(prometheus.CounterOpts{
				Name: "fsip2_login_attempts_total",
				Help: "Total number of login attempts",
			}),
			LoginSuccess: promauto.NewCounter(prometheus.CounterOpts{
				Name: "fsip2_login_success_total",
				Help: "Total number of successful logins",
			}),
			LoginFailures: promauto.NewCounter(prometheus.CounterOpts{
				Name: "fsip2_login_failures_total",
				Help: "Total number of failed logins",
			}),

			CheckoutTotal: promauto.NewCounter(prometheus.CounterOpts{
				Name: "fsip2_checkout_total",
				Help: "Total number of checkout requests",
			}),
			CheckoutSuccess: promauto.NewCounter(prometheus.CounterOpts{
				Name: "fsip2_checkout_success_total",
				Help: "Total number of successful checkouts",
			}),
			CheckoutFailures: promauto.NewCounter(prometheus.CounterOpts{
				Name: "fsip2_checkout_failures_total",
				Help: "Total number of failed checkouts",
			}),

			CheckinTotal: promauto.NewCounter(prometheus.CounterOpts{
				Name: "fsip2_checkin_total",
				Help: "Total number of checkin requests",
			}),
			CheckinSuccess: promauto.NewCounter(prometheus.CounterOpts{
				Name: "fsip2_checkin_success_total",
				Help: "Total number of successful checkins",
			}),
			CheckinFailures: promauto.NewCounter(prometheus.CounterOpts{
				Name: "fsip2_checkin_failures_total",
				Help: "Total number of failed checkins",
			}),

			RenewTotal: promauto.NewCounter(prometheus.CounterOpts{
				Name: "fsip2_renew_total",
				Help: "Total number of renewal requests",
			}),
			RenewSuccess: promauto.NewCounter(prometheus.CounterOpts{
				Name: "fsip2_renew_success_total",
				Help: "Total number of successful renewals",
			}),
			RenewFailures: promauto.NewCounter(prometheus.CounterOpts{
				Name: "fsip2_renew_failures_total",
				Help: "Total number of failed renewals",
			}),

			// FOLIO API metrics
			FolioRequestsTotal: promauto.NewCounterVec(
				prometheus.CounterOpts{
					Name: "folio_requests_total",
					Help: "Total number of FOLIO API requests by endpoint",
				},
				[]string{"endpoint", "method", "tenant"},
			),
			FolioRequestDuration: promauto.NewHistogramVec(
				prometheus.HistogramOpts{
					Name:    "folio_request_duration_seconds",
					Help:    "Duration of FOLIO API requests in seconds",
					Buckets: []float64{0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
				},
				[]string{"endpoint", "method", "tenant"},
			),
			FolioRequestErrors: promauto.NewCounterVec(
				prometheus.CounterOpts{
					Name: "folio_request_errors_total",
					Help: "Total number of FOLIO API errors by endpoint",
				},
				[]string{"endpoint", "method", "tenant", "status_code"},
			),

			// Tenant metrics
			TenantResolutions: promauto.NewCounterVec(
				prometheus.CounterOpts{
					Name: "fsip2_tenant_resolutions_total",
					Help: "Total number of tenant resolutions by phase and resolver",
				},
				[]string{"phase", "resolver", "tenant"},
			),
			TenantResolutionErrors: promauto.NewCounter(prometheus.CounterOpts{
				Name: "fsip2_tenant_resolution_errors_total",
				Help: "Total number of tenant resolution errors",
			}),

			// Session metrics
			SessionsCreated: promauto.NewCounter(prometheus.CounterOpts{
				Name: "fsip2_sessions_created_total",
				Help: "Total number of SIP2 sessions created",
			}),
			SessionsEnded: promauto.NewCounter(prometheus.CounterOpts{
				Name: "fsip2_sessions_ended_total",
				Help: "Total number of SIP2 sessions ended",
			}),
			SessionsActive: promauto.NewGauge(prometheus.GaugeOpts{
				Name: "fsip2_sessions_active",
				Help: "Current number of active SIP2 sessions",
			}),
		}
	})
	return globalMetrics
}

// GetMessageTypeName returns a human-readable name for a message type code
func GetMessageTypeName(code string) string {
	messageTypes := map[string]string{
		"93": "login",
		"99": "sc_status",
		"23": "patron_status",
		"11": "checkout",
		"09": "checkin",
		"63": "patron_information",
		"17": "item_information",
		"29": "renew",
		"65": "renew_all",
		"35": "end_session",
		"37": "fee_paid",
		"19": "item_status_update",
		"97": "resend",
	}

	if name, ok := messageTypes[code]; ok {
		return name
	}
	return "unknown"
}
