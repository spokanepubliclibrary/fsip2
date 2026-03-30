package server

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"strings"
	"time"

	"github.com/spokanepubliclibrary/fsip2/internal/helpers"
	"github.com/spokanepubliclibrary/fsip2/internal/logging"
	"github.com/spokanepubliclibrary/fsip2/internal/sip2/builder"
	"github.com/spokanepubliclibrary/fsip2/internal/sip2/parser"
	"github.com/spokanepubliclibrary/fsip2/internal/tenant"
	"github.com/spokanepubliclibrary/fsip2/internal/types"
	"go.uber.org/zap"
)

// Connection represents a single SIP2 client connection
type Connection struct {
	conn          net.Conn
	session       *types.Session
	parser        *parser.Parser
	builder       *builder.ResponseBuilder
	tenantService *tenant.Service
	handlers      map[parser.MessageCode]MessageHandler
	server        *Server
}

// MessageHandler is an interface for handling SIP2 messages
type MessageHandler interface {
	Handle(ctx context.Context, msg *parser.Message, session *types.Session) (string, error)
}

// NewConnection creates a new connection handler
func NewConnection(
	conn net.Conn,
	session *types.Session,
	tenantService *tenant.Service,
	handlers map[parser.MessageCode]MessageHandler,
	server *Server,
) *Connection {
	return &Connection{
		conn:          conn,
		session:       session,
		parser:        parser.NewParser(session.TenantConfig),
		builder:       builder.NewResponseBuilder(session.TenantConfig),
		tenantService: tenantService,
		handlers:      handlers,
		server:        server,
	}
}

// Handle processes the connection
func (c *Connection) Handle(ctx context.Context) error {
	defer c.conn.Close()

	// Set read timeout
	if err := c.conn.SetReadDeadline(time.Now().Add(5 * time.Minute)); err != nil {
		return fmt.Errorf("failed to set read deadline: %w", err)
	}

	reader := bufio.NewReader(c.conn)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			// Read message until delimiter
			rawMessage, err := c.readMessage(reader)
			if err != nil {
				if err == io.EOF {
					return nil // Client closed connection
				}
				return fmt.Errorf("failed to read message: %w", err)
			}

			// Process message
			response, err := c.processMessage(ctx, rawMessage)
			if err != nil {
				// Log error but continue (don't close connection on single message error)
				c.server.logger.Error("Failed to process message", logging.TypeField(logging.TypeApplication), zap.Error(err), zap.String("message", rawMessage))

				// Try to send an error response to the client instead of silently continuing
				errorResponse := c.buildErrorResponse(rawMessage, err)
				if errorResponse != "" {
					if sendErr := c.sendMessage(errorResponse); sendErr != nil {
						c.server.logger.Error("Failed to send error response", logging.TypeField(logging.TypeApplication), zap.Error(sendErr))
					}
				}
				continue
			}

			// Send response
			if err := c.sendMessage(response); err != nil {
				return fmt.Errorf("failed to send response: %w", err)
			}

			// Reset read deadline
			if err := c.conn.SetReadDeadline(time.Now().Add(5 * time.Minute)); err != nil {
				return fmt.Errorf("failed to reset read deadline: %w", err)
			}
		}
	}
}

// readMessage reads a SIP2 message from the connection until the configured delimiter is found.
//
// This function implements a byte-by-byte reading algorithm to support configurable delimiters
// of arbitrary length (single-byte like "|" or multi-byte like "||" or "\r\n").
//
// Algorithm:
//  1. Create a 1-byte buffer for reading individual bytes
//  2. Read bytes one at a time and append to message buffer
//  3. After each byte, check if the last N bytes match the delimiter
//     (where N is the delimiter length)
//  4. Once delimiter is found, remove it from the message and return
//
// Delimiter Detection:
//   - For single-byte delimiters (e.g., "|"), checks last 1 byte
//   - For multi-byte delimiters (e.g., "||"), checks last 2 bytes
//   - Uses string comparison on the tail of the message buffer
//
// Performance Note:
//
//	This byte-by-byte approach creates many small allocations (1 per byte + append reallocations).
//	For typical SIP2 messages (100-500 bytes), this is acceptable given the low message rate
//	from self-service kiosks (typically <10 messages/sec). A future optimization could use
//	a pre-allocated buffer with ReadBytes() or Scanner, but would need careful handling of
//	multi-byte delimiters that don't align with newlines.
//
// Example:
//
//	Message: "2300019700101    084625AO|AA12345|" with delimiter "|"
//	Reads: '2' '3' '0' '0' ... 'A' 'A' '1' '2' '3' '4' '5' '|'
//	Returns: "2300019700101    084625AO|AA12345" (delimiter removed)
func (c *Connection) readMessage(reader *bufio.Reader) (string, error) {
	delimiter := c.session.TenantConfig.GetMessageDelimiterBytes()

	var message []byte
	buf := make([]byte, 1) // Single-byte buffer for reading one byte at a time

	// Read bytes one at a time until we find the delimiter
	for {
		n, err := reader.Read(buf)
		if err != nil {
			return "", err
		}

		if n > 0 {
			// Append the byte to our message buffer
			message = append(message, buf[0])

			// Check if the last N bytes of the message match the delimiter
			// Only check once we have enough bytes (message length >= delimiter length)
			if len(message) >= len(delimiter) {
				// Extract the tail of the message with length equal to delimiter
				tail := message[len(message)-len(delimiter):]

				// Compare tail with delimiter
				if string(tail) == string(delimiter) {
					// Found delimiter! Remove it from message and stop reading
					message = message[:len(message)-len(delimiter)]
					break
				}
			}
		}
	}

	receivedMsg := string(message)

	// Log received message with directional indicator (if allowed by tenant log level)
	messageCode := logging.ExtractMessageCode(receivedMsg)
	logLevel := c.session.TenantConfig.LogLevel
	if logging.ShouldLogMessage(messageCode, logLevel) {
		obfuscatedMsg := logging.ObfuscateMessage(receivedMsg, messageCode, logLevel)
		c.server.logger.Info("SIP2 message received",
			logging.TypeField(logging.TypeSIPRequest),
			zap.String("message", obfuscatedMsg),
			zap.String("message_code", messageCode),
			zap.String("session_id", c.session.ID),
		)
	}

	return receivedMsg, nil
}

// processMessage processes a SIP2 message and returns the response
func (c *Connection) processMessage(ctx context.Context, rawMessage string) (string, error) {
	// Update session activity
	c.session.UpdateActivity()

	// Parse the message
	msg, err := c.parser.Parse(rawMessage)
	if err != nil {
		return "", fmt.Errorf("failed to parse message: %w", err)
	}

	// Validate message is supported
	if err := c.parser.ValidateMessage(msg); err != nil {
		return "", fmt.Errorf("message validation failed: %w", err)
	}

	// Special handling for LOGIN message - may change tenant
	if msg.Code == parser.LoginRequest {
		if err := c.handleLoginTenantResolution(ctx, msg); err != nil {
			// Log error but continue with current tenant
		}
	}

	// Find handler for this message type
	handler, ok := c.handlers[msg.Code]
	if !ok {
		c.server.logger.Error("No handler found for message type",
			logging.TypeField(logging.TypeApplication),
			zap.String("message_code", string(msg.Code)),
			zap.String("tenant", c.session.TenantConfig.Tenant))
		return "", fmt.Errorf("no handler for message type: %s", msg.Code)
	}

	// Track message metrics
	metrics := c.server.GetMetrics()
	messageType := string(msg.Code)
	tenant := c.session.TenantConfig.Tenant

	// Log handler invocation
	c.server.logger.Info("Invoking message handler",
		logging.TypeField(logging.TypeApplication),
		zap.String("message_code", string(msg.Code)),
		zap.String("tenant", tenant),
		zap.String("session_id", c.session.ID))

	// Increment message counter
	metrics.MessagesTotal.WithLabelValues(messageType, tenant).Inc()

	// Track message duration
	startTime := time.Now()
	defer func() {
		metrics.MessageDuration.WithLabelValues(messageType, tenant).Observe(time.Since(startTime).Seconds())
	}()

	// Handle the message
	response, err := handler.Handle(ctx, msg, c.session)
	if err != nil {
		// Track error
		metrics.MessageErrors.WithLabelValues(messageType, tenant, "handler_error").Inc()
		c.server.logger.Error("Handler returned error",
			logging.TypeField(logging.TypeApplication),
			zap.String("message_code", string(msg.Code)),
			zap.String("tenant", tenant),
			zap.Error(err))
		return "", fmt.Errorf("handler error: %w", err)
	}

	// Log successful handler response
	c.server.logger.Info("Handler generated response",
		logging.TypeField(logging.TypeApplication),
		zap.String("message_code", string(msg.Code)),
		zap.String("tenant", tenant),
		zap.Int("response_length", len(response)))

	return response, nil
}

// handleLoginTenantResolution handles tenant resolution during LOGIN
func (c *Connection) handleLoginTenantResolution(ctx context.Context, msg *parser.Message) error {
	// Extract login fields
	username := msg.GetField(parser.LoginUserID)
	locationCode := msg.GetField(parser.LocationCode)

	// Attempt LOGIN phase resolution
	newTenant, err := c.tenantService.ResolveAtLogin(
		ctx,
		username,
		locationCode,
		c.session.TenantConfig,
	)
	if err != nil {
		return err
	}

	// Update session tenant if changed
	if newTenant != nil && newTenant.Tenant != c.session.TenantConfig.Tenant {
		c.session.UpdateTenant(newTenant)
		// Recreate parser and builder with new tenant config
		c.parser = parser.NewParser(newTenant)
		c.builder = builder.NewResponseBuilder(newTenant)
	}

	return nil
}

// sendMessage sends a message to the client
func (c *Connection) sendMessage(message string) error {
	// Log sent message with directional indicator (if allowed by tenant log level)
	messageCode := logging.ExtractMessageCode(message)
	logLevel := c.session.TenantConfig.LogLevel
	if logging.ShouldLogMessage(messageCode, logLevel) {
		obfuscatedMsg := logging.ObfuscateMessage(message, messageCode, logLevel)
		c.server.logger.Info("SIP2 message sent",
			logging.TypeField(logging.TypeSIPResponse),
			zap.String("message", obfuscatedMsg),
			zap.String("message_code", messageCode),
			zap.String("session_id", c.session.ID),
		)
	}

	// Add delimiter if not present
	hadDelimiter := strings.HasSuffix(message, c.session.TenantConfig.MessageDelimiter)
	if !hadDelimiter {
		message += c.session.TenantConfig.MessageDelimiter
	}
	messageBytes := []byte(message)

	// Log response format details
	c.server.logger.Debug("Sending response to client",
		logging.TypeField(logging.TypeApplication),
		zap.Int("message_length", len(message)),
		zap.Int("bytes_length", len(messageBytes)),
		zap.Bool("had_delimiter", hadDelimiter),
		zap.String("delimiter", c.session.TenantConfig.MessageDelimiter),
		zap.String("session_id", c.session.ID))

	// Write to connection
	_, err := c.conn.Write(messageBytes)
	if err != nil {
		c.server.logger.Error("Failed to write message to connection",
			logging.TypeField(logging.TypeApplication),
			zap.Error(err),
			zap.String("session_id", c.session.ID))
		return fmt.Errorf("failed to write message: %w", err)
	}

	c.server.logger.Debug("Response sent successfully",
		logging.TypeField(logging.TypeApplication),
		zap.Int("bytes_written", len(messageBytes)),
		zap.String("session_id", c.session.ID))

	return nil
}

// Close closes the connection
func (c *Connection) Close() error {
	return c.conn.Close()
}

// GetRemoteAddr returns the remote address
func (c *Connection) GetRemoteAddr() string {
	return c.conn.RemoteAddr().String()
}

// GetLocalAddr returns the local address
func (c *Connection) GetLocalAddr() string {
	return c.conn.LocalAddr().String()
}

// buildErrorResponse builds a generic error response for a failed message
func (c *Connection) buildErrorResponse(rawMessage string, processingError error) string {
	// Try to parse the message to get the message code
	msg, err := c.parser.Parse(rawMessage)
	if err != nil {
		// Can't parse the message, can't send a proper response
		c.server.logger.Warn("Cannot build error response - failed to parse message",
			logging.TypeField(logging.TypeApplication),
			zap.Error(err),
			zap.String("raw_message", rawMessage))
		return ""
	}

	// Get the appropriate response code for this request
	responseCode := msg.Code.GetResponseCode()
	if responseCode == "" {
		// No response code mapping exists
		c.server.logger.Warn("Cannot build error response - no response code mapping",
			logging.TypeField(logging.TypeApplication),
			zap.String("request_code", string(msg.Code)))
		return ""
	}

	// Build a generic error response based on the message type
	// The response format varies by message type, but we'll build a minimal valid response
	var content string
	errorMsg := processingError.Error()
	if len(errorMsg) > 255 {
		errorMsg = errorMsg[:255] // Truncate if too long
	}

	// Add basic fields that most responses need
	institutionID := msg.GetField(parser.InstitutionID)
	patronID := msg.GetField(parser.PatronIdentifier)
	itemID := msg.GetField(parser.ItemIdentifier)

	// Build content based on message type - add required fields first, then error message
	switch responseCode {
	case parser.CheckoutResponse, parser.CheckinResponse, parser.RenewResponse:
		// Format: 12<ok><renewal ok><magnetic media><desensitize><transaction date>
		// or 10<ok><resensitize><magnetic media><alert><transaction date>
		// We set ok=0 (N) to indicate failure
		content = "0"                   // OK flag = N (not ok)
		if responseCode == parser.CheckoutResponse {
			content += "UU"             // Renewal OK = U (unknown), Magnetic media = U (unknown)
		} else if responseCode == parser.CheckinResponse {
			content += "UU"             // Resensitize = U, Magnetic media = U
		} else {
			content += "UU"             // Renewal OK = U, Magnetic media = U
		}
		content += time.Now().Format("20060102    150405") // Transaction date
	case parser.PatronStatusResponse, parser.PatronInformationResponse:
		// Format: 24<patron status><language><transaction date>
		content = "YYYYYYYYYYYYYY" // All blocks/flags set to Y (blocked)
		content += "000"            // Language = 000 (unknown)
		content += time.Now().Format("20060102    150405") // Transaction date
	default:
		// For other message types, just use transaction date
		content = time.Now().Format("20060102    150405")
	}

	// Add institution ID if available
	if institutionID != "" {
		content += "|AO" + institutionID
	}

	// Add patron identifier if available
	if patronID != "" {
		content += "|AA" + patronID
	}

	// Add item identifier if available
	if itemID != "" {
		content += "|AB" + itemID
	}

	// Add error message to screen message field (AF)
	content += "|AF" + errorMsg

	// Use builder to add sequence number, checksum, and delimiter
	response, err := c.builder.Build(responseCode, content, msg.SequenceNumber)
	if err != nil {
		c.server.logger.Error("Failed to build error response",
			logging.TypeField(logging.TypeApplication),
			zap.Error(err),
			zap.String("response_code", string(responseCode)))
		return ""
	}

	return response
}

// GetClientIP returns the client IP address
func (c *Connection) GetClientIP() (string, error) {
	return helpers.ExtractIPFromAddr(c.conn.RemoteAddr())
}

// GetClientPort returns the client port
func (c *Connection) GetClientPort() (int, error) {
	return helpers.ExtractPortFromAddr(c.conn.RemoteAddr())
}

// GetServerPort returns the server port
func (c *Connection) GetServerPort() (int, error) {
	return helpers.ExtractPortFromAddr(c.conn.LocalAddr())
}
