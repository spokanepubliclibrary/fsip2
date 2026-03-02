package health

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

// Server represents the health check HTTP server
type Server struct {
	host   string
	port   int
	logger *zap.Logger
	server *http.Server
}

// NewServer creates a new health check server
func NewServer(port int, logger *zap.Logger) *Server {
	return &Server{
		port:   port,
		logger: logger,
	}
}

// NewServerWithHost creates a new health check server bound to a specific host address
func NewServerWithHost(host string, port int, logger *zap.Logger) *Server {
	return &Server{
		host:   host,
		port:   port,
		logger: logger,
	}
}

// Start starts the health check server
func (s *Server) Start(ctx context.Context) error {
	mux := http.NewServeMux()

	// Health check endpoint
	mux.HandleFunc("/admin/health", s.healthHandler)

	// Readiness endpoint
	mux.HandleFunc("/admin/ready", s.readyHandler)

	// Prometheus metrics endpoint
	mux.Handle("/metrics", promhttp.Handler())

	// Create server
	s.server = &http.Server{
		Addr:         fmt.Sprintf("%s:%d", s.host, s.port),
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	s.logger.Info("Health check server starting",
		zap.Int("port", s.port),
	)

	// Start server
	if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("health check server error: %w", err)
	}

	return nil
}

// Stop stops the health check server
func (s *Server) Stop(ctx context.Context) error {
	if s.server == nil {
		return nil
	}

	s.logger.Info("Stopping health check server...")

	if err := s.server.Shutdown(ctx); err != nil {
		return fmt.Errorf("failed to shutdown health check server: %w", err)
	}

	s.logger.Info("Health check server stopped")
	return nil
}

// healthHandler handles health check requests
func (s *Server) healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

// readyHandler handles readiness check requests
func (s *Server) readyHandler(w http.ResponseWriter, r *http.Request) {
	// Can add more sophisticated readiness checks here
	// For now, just return OK
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Ready"))
}
