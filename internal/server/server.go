package server

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/spokanepubliclibrary/fsip2/internal/config"
	"github.com/spokanepubliclibrary/fsip2/internal/handlers"
	"github.com/spokanepubliclibrary/fsip2/internal/helpers"
	"github.com/spokanepubliclibrary/fsip2/internal/metrics"
	"github.com/spokanepubliclibrary/fsip2/internal/sip2/parser"
	"github.com/spokanepubliclibrary/fsip2/internal/tenant"
	"github.com/spokanepubliclibrary/fsip2/internal/types"
	"go.uber.org/zap"
)

// Server represents the SIP2 TCP server
type Server struct {
	config        *config.Config
	logger        *zap.Logger
	listener      net.Listener
	tenantService *tenant.Service
	handlers      map[parser.MessageCode]MessageHandler
	metrics       *metrics.Metrics

	// Connection tracking
	activeConnections int64
	totalConnections  int64

	// Lifecycle
	mu        sync.RWMutex
	isRunning bool
	wg        sync.WaitGroup
}

// NewServer creates a new SIP2 server
func NewServer(cfg *config.Config, logger *zap.Logger) (*Server, error) {
	// Create tenant service
	tenantService := tenant.NewService(cfg)

	// Initialize metrics
	m := metrics.NewMetrics()

	return &Server{
		config:        cfg,
		logger:        logger,
		tenantService: tenantService,
		handlers:      make(map[parser.MessageCode]MessageHandler),
		metrics:       m,
		isRunning:     false,
	}, nil
}

// RegisterHandler registers a message handler
func (s *Server) RegisterHandler(code parser.MessageCode, handler MessageHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handlers[code] = handler
}

// RegisterAllHandlers registers all SIP2 message handlers
func (s *Server) RegisterAllHandlers() {
	// Get default tenant config for handler initialization
	// Handlers will use session-specific tenant configs at runtime
	defaultTenantConfig := &config.TenantConfig{
		Tenant:   "default",
		OkapiURL: s.config.OkapiURL,
	}

	// Register all SIP2 message handlers
	s.RegisterHandler(parser.LoginRequest, handlers.NewLoginHandler(s.logger, defaultTenantConfig))
	s.RegisterHandler(parser.SCStatus, handlers.NewSCStatusHandler(s.logger, defaultTenantConfig))
	s.RegisterHandler(parser.PatronStatusRequest, handlers.NewPatronStatusHandler(s.logger, defaultTenantConfig))
	s.RegisterHandler(parser.CheckoutRequest, handlers.NewCheckoutHandler(s.logger, defaultTenantConfig))
	s.RegisterHandler(parser.CheckinRequest, handlers.NewCheckinHandler(s.logger, defaultTenantConfig))
	s.RegisterHandler(parser.PatronInformationRequest, handlers.NewPatronInformationHandler(s.logger, defaultTenantConfig))
	s.RegisterHandler(parser.ItemInformationRequest, handlers.NewItemInformationHandler(s.logger, defaultTenantConfig))
	s.RegisterHandler(parser.RenewRequest, handlers.NewRenewHandler(s.logger, defaultTenantConfig))
	s.RegisterHandler(parser.RenewAllRequest, handlers.NewRenewAllHandler(s.logger, defaultTenantConfig))
	s.RegisterHandler(parser.EndPatronSessionRequest, handlers.NewEndSessionHandler(s.logger, defaultTenantConfig))
	s.RegisterHandler(parser.FeePaidRequest, handlers.NewFeePaidHandler(s.logger, defaultTenantConfig))
	s.RegisterHandler(parser.ItemStatusUpdateRequest, handlers.NewItemStatusUpdateHandler(s.logger, defaultTenantConfig))
	s.RegisterHandler(parser.RequestSCResend, handlers.NewResendHandler(s.logger, defaultTenantConfig))
	s.RegisterHandler(parser.RequestACSResend, handlers.NewResendHandler(s.logger, defaultTenantConfig))

	s.logger.Info("All SIP2 message handlers registered",
		zap.Int("handler_count", len(s.handlers)),
	)
}

// Start starts the SIP2 server
func (s *Server) Start(ctx context.Context) error {
	s.mu.Lock()
	if s.isRunning {
		s.mu.Unlock()
		return fmt.Errorf("server is already running")
	}
	s.isRunning = true
	s.mu.Unlock()

	// Create listener
	addr := fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)

	var listener net.Listener
	var err error

	// Check if TLS is enabled
	if s.config.TLS != nil && s.config.TLS.Enabled {
		tlsConfig, err := LoadTLSConfig(s.config.TLS.CertFile, s.config.TLS.KeyFile)
		if err != nil {
			return fmt.Errorf("failed to load TLS config: %w", err)
		}

		listener, err = tls.Listen("tcp", addr, tlsConfig)
		if err != nil {
			return fmt.Errorf("failed to start TLS listener: %w", err)
		}

		s.logger.Info("TLS enabled", zap.String("cert", s.config.TLS.CertFile))
	} else {
		listener, err = net.Listen("tcp", addr)
		if err != nil {
			return fmt.Errorf("failed to start listener: %w", err)
		}
	}

	s.listener = listener

	s.logger.Info("SIP2 server started",
		zap.String("address", addr),
		zap.Int("port", s.config.Port),
	)

	// Accept connections
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			conn, err := listener.Accept()
			if err != nil {
				// Check if server is stopping
				s.mu.RLock()
				running := s.isRunning
				s.mu.RUnlock()

				if !running {
					return nil
				}

				s.logger.Error("Failed to accept connection", zap.Error(err))
				continue
			}

			// Increment connection counters
			atomic.AddInt64(&s.activeConnections, 1)
			atomic.AddInt64(&s.totalConnections, 1)

			// Update metrics
			s.metrics.ConnectionsTotal.Inc()
			s.metrics.ConnectionsActive.Inc()

			// Handle connection in goroutine
			s.wg.Add(1)
			go func() {
				defer s.wg.Done()
				defer atomic.AddInt64(&s.activeConnections, -1)
				defer s.metrics.ConnectionsActive.Dec()

				startTime := time.Now()
				if err := s.handleConnection(ctx, conn); err != nil {
					s.logger.Error("Connection error",
						zap.String("remote", conn.RemoteAddr().String()),
						zap.Error(err),
					)
					s.metrics.ConnectionErrors.Inc()
				}
				s.metrics.ConnectionDuration.Observe(time.Since(startTime).Seconds())
			}()
		}
	}
}

// handleConnection handles a single client connection
func (s *Server) handleConnection(ctx context.Context, conn net.Conn) error {
	// Extract connection info
	clientIP, err := helpers.ExtractIPFromAddr(conn.RemoteAddr())
	if err != nil {
		clientIP = conn.RemoteAddr().String()
	}

	clientPort, err := helpers.ExtractPortFromAddr(conn.RemoteAddr())
	if err != nil {
		clientPort = 0
	}

	serverPort, err := helpers.ExtractPortFromAddr(conn.LocalAddr())
	if err != nil {
		serverPort = s.config.Port
	}

	s.logger.Info("New connection",
		zap.String("client_ip", clientIP),
		zap.Int("client_port", clientPort),
		zap.Int("server_port", serverPort),
	)

	// Resolve tenant at CONNECT phase
	tenantConfig, err := s.tenantService.ResolveAtConnect(ctx, clientIP, clientPort, serverPort)
	if err != nil {
		s.logger.Error("Failed to resolve tenant", zap.Error(err))
		return err
	}

	s.logger.Info("Tenant resolved",
		zap.String("tenant", tenantConfig.Tenant),
		zap.String("client_ip", clientIP),
	)

	// Create session
	sessionID := helpers.GenerateID()
	session := types.NewSession(sessionID, tenantConfig)

	// Update session metrics
	s.metrics.SessionsCreated.Inc()
	s.metrics.SessionsActive.Inc()
	defer s.metrics.SessionsActive.Dec()

	// Create connection handler
	connHandler := NewConnection(
		conn,
		session,
		s.tenantService,
		s.handlers,
		s,
	)

	// Handle the connection
	return connHandler.Handle(ctx)
}

// Stop stops the SIP2 server
func (s *Server) Stop(ctx context.Context) error {
	s.mu.Lock()
	if !s.isRunning {
		s.mu.Unlock()
		return nil
	}
	s.isRunning = false
	s.mu.Unlock()

	s.logger.Info("Stopping SIP2 server...")

	// Close listener
	if s.listener != nil {
		if err := s.listener.Close(); err != nil {
			s.logger.Error("Error closing listener", zap.Error(err))
		}
	}

	// Wait for all connections to close (with timeout)
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		s.logger.Info("All connections closed")
	case <-time.After(30 * time.Second):
		s.logger.Warn("Timeout waiting for connections to close")
	case <-ctx.Done():
		s.logger.Warn("Context cancelled while waiting for connections")
	}

	s.logger.Info("SIP2 server stopped")
	return nil
}

// IsRunning returns whether the server is currently running
func (s *Server) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.isRunning
}

// GetActiveConnections returns the number of active connections
func (s *Server) GetActiveConnections() int64 {
	return atomic.LoadInt64(&s.activeConnections)
}

// GetTotalConnections returns the total number of connections handled
func (s *Server) GetTotalConnections() int64 {
	return atomic.LoadInt64(&s.totalConnections)
}

// GetTenantService returns the tenant service
func (s *Server) GetTenantService() *tenant.Service {
	return s.tenantService
}

// GetConfig returns the server configuration
func (s *Server) GetConfig() *config.Config {
	return s.config
}

// GetLogger returns the server logger
func (s *Server) GetLogger() *zap.Logger {
	return s.logger
}

// GetMetrics returns the server metrics
func (s *Server) GetMetrics() *metrics.Metrics {
	return s.metrics
}
