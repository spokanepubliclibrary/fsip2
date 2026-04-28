package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/spokanepubliclibrary/fsip2/internal/config"
	"github.com/spokanepubliclibrary/fsip2/internal/folio"
	"github.com/spokanepubliclibrary/fsip2/internal/health"
	"github.com/spokanepubliclibrary/fsip2/internal/logging"
	"github.com/spokanepubliclibrary/fsip2/internal/server"
	"go.uber.org/zap"
)

var (
	version   = "1.0.0"
	buildDate = "unknown"
	gitCommit = "unknown"
)

func main() {
	var configFile string
	var logFile string

	rootCmd := &cobra.Command{
		Use:   "fsip2",
		Short: "FOLIO Edge SIP2 Server",
		Long:  "A bridge application connecting self-service library kiosks with the FOLIO library management system via SIP2 protocol",
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(configFile, logFile)
		},
	}

	rootCmd.Flags().StringVarP(&configFile, "config", "c", "", "Configuration file path (required)")
	rootCmd.MarkFlagRequired("config")
	rootCmd.Flags().StringVarP(&logFile, "log", "l", "", "Log file path (optional, logs to stdout if not specified)")

	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("fsip2 version %s\n", version)
			fmt.Printf("Build date: %s\n", buildDate)
			fmt.Printf("Git commit: %s\n", gitCommit)
		},
	}

	rootCmd.AddCommand(versionCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run(configFile string, logFile string) error {
	// Load configuration
	cfg, err := config.Load(configFile)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	// Initialize logger
	logger, err := logging.NewLogger(cfg.LogLevel, logFile)
	if err != nil {
		return fmt.Errorf("failed to initialize logger: %w", err)
	}
	defer logger.Sync()

	// Set auth logger for token expiration debugging (Phase 1.1)
	folio.SetAuthLogger(logger)
	// Set client logger for FOLIO HTTP request/response debug logging (Phase 4)
	folio.SetClientLogger(logger)

	logger.Info("Starting fsip2",
		logging.TypeField(logging.TypeApplication),
		zap.String("version", version),
		zap.Int("port", cfg.Port),
		zap.String("okapi_url", cfg.OkapiURL),
	)

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start health check server
	healthServer := health.NewServer(cfg.HealthCheckPort, logger)
	go func() {
		if err := healthServer.Start(ctx); err != nil {
			logger.Error("Health check server error", logging.TypeField(logging.TypeApplication), zap.Error(err))
		}
	}()

	// Create and start SIP2 server
	sip2Server, err := server.NewServer(cfg, logger)
	if err != nil {
		return fmt.Errorf("failed to create server: %w", err)
	}

	// Register all SIP2 message handlers
	sip2Server.RegisterAllHandlers()

	// Start configuration reloader if tenant config sources are defined
	if len(cfg.TenantConfigSources) > 0 {
		reloader := config.NewReloader(cfg, logger, func(updated *config.Config) {
			logger.Info("Configuration reloaded",
				logging.TypeField(logging.TypeApplication),
				zap.Int("tenant_count", len(updated.GetTenants())),
			)
			sip2Server.GetTenantService().Reinitialize(updated)
		})
		if err := reloader.Start(ctx); err != nil {
			return fmt.Errorf("failed to start config reloader: %w", err)
		}
		logger.Debug("Configuration reloader started",
			logging.TypeField(logging.TypeApplication),
			zap.Duration("scan_period", cfg.GetScanPeriod()),
			zap.Int("sources", len(cfg.TenantConfigSources)),
		)
		defer reloader.Stop()
	}

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	errChan := make(chan error, 1)
	go func() {
		errChan <- sip2Server.Start(ctx)
	}()

	// Wait for shutdown signal or error
	select {
	case <-sigChan:
		logger.Info("Received shutdown signal, gracefully stopping...", logging.TypeField(logging.TypeApplication))
		cancel()
		if err := sip2Server.Stop(ctx); err != nil {
			logger.Error("Error during shutdown", logging.TypeField(logging.TypeApplication), zap.Error(err))
			return err
		}
		logger.Info("Server stopped successfully", logging.TypeField(logging.TypeApplication))
		return nil
	case err := <-errChan:
		if err != nil {
			return fmt.Errorf("server error: %w", err)
		}
		return nil
	}
}
