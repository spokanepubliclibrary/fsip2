// Package server implements the TCP server for fsip2 SIP2 protocol handling.
//
// This package provides a multi-connection TCP server that listens for incoming
// SIP2 protocol connections from library self-service kiosks, manages connection
// lifecycle, and dispatches messages to appropriate handlers.
//
// # Architecture
//
// The server consists of:
//   - Server: Main TCP listener managing connections and handlers
//   - Connection: Per-client connection handler managing message flow
//   - Session: Per-connection state including tenant configuration and authentication
//
// # Connection Lifecycle
//
//  1. Client connects to TCP port (default: 6443)
//  2. Server creates Connection instance with new Session
//  3. Connection reads SIP2 messages using configurable delimiter
//  4. Each message is parsed and dispatched to registered handler
//  5. Handler processes message and returns SIP2 response
//  6. Response is sent back to client
//  7. Connection continues until client disconnects or timeout occurs
//  8. Connection cleanup: close socket, decrement metrics, signal wait group
//
// # Multi-tenancy
//
// The server supports multiple FOLIO tenants through configurable tenant resolution:
//   - SC Terminal tenant: Identified by SC/login terminal username
//   - Institution ID tenant: Identified by AO field in messages
//   - Patron ID tenant: Identified by AA field in messages
//   - Username tenant: Identified by CO login username
//
// Each connection resolves its tenant during login (message 93) and caches the
// TenantConfig in the session for subsequent requests.
//
// # Connection Management
//
// Connections are managed with:
//   - Goroutine per connection (one per kiosk)
//   - Read timeouts (5 minutes, reset after each message)
//   - Graceful shutdown with 30-second timeout
//   - Wait groups to track active connections
//   - Atomic counters for active/total connection metrics
//
// # Message Reading
//
// The connection reads messages using a delimiter-based algorithm:
//   - Reads bytes until configured message delimiter is found
//   - Supports single-byte and multi-byte delimiters
//   - Maximum message size enforced (default: 10KB)
//   - Read timeout enforced to prevent hanging connections
//
// # Handler Registration
//
// Handlers are registered by message type (e.g., "23" for Patron Status):
//
//	server.RegisterHandler("23", patronStatusHandler)
//	server.RegisterHandler("11", checkoutHandler)
//	// ... etc.
//
// The RegisterAllHandlers() function registers all standard SIP2 message handlers.
//
// # TLS Support
//
// The server supports TLS encryption when configured:
//   - Configurable certificate and key files
//   - Automatic TLS listener setup
//   - Secure communication with kiosks
//
// # Metrics
//
// The server tracks operational metrics via Prometheus:
//   - Active connections (gauge)
//   - Total connections (counter)
//   - Messages received per type (counter)
//   - Message processing duration (histogram)
//   - Handler errors (counter)
//
// # Graceful Shutdown
//
// The server implements graceful shutdown:
//   - Stop accepting new connections
//   - Wait up to 30 seconds for active connections to complete
//   - Force close remaining connections after timeout
//   - Wait for all goroutines to finish via wait groups
//   - Clean up resources (close listeners, clear state)
//
// # Error Handling
//
// Connection errors are handled gracefully:
//   - EOF: Client disconnected normally
//   - Timeout: Client inactive, connection closed
//   - Parse errors: Error response sent to client
//   - Handler errors: Error response sent to client
//   - All errors logged with structured logging
//
// # Performance
//
// The server is optimized for:
//   - Concurrent connection handling (goroutines)
//   - Efficient message reading (buffered reads)
//   - Connection pooling for FOLIO API calls
//   - Low memory allocation per message
//   - Prometheus metrics with minimal overhead
//
// # Usage Example
//
//	// Create server
//	srv := NewServer(config, logger, metricsCollector)
//
//	// Register handlers
//	srv.RegisterAllHandlers()
//
//	// Start server (blocks until shutdown)
//	ctx, cancel := context.WithCancel(context.Background())
//	defer cancel()
//
//	if err := srv.Start(ctx); err != nil {
//	    log.Fatal(err)
//	}
package server
