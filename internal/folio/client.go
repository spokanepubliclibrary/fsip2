package folio

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/spokanepubliclibrary/fsip2/internal/logging"
	"go.uber.org/zap"
)

// Package-level logger for client operations (can be set via SetClientLogger)
var clientLogger *zap.Logger

// SetClientLogger sets the logger for HTTP client operations
func SetClientLogger(logger *zap.Logger) {
	clientLogger = logger
}

var (
	// sharedHTTPClient is a package-level shared HTTP client with connection pooling
	// This prevents creating new TCP connections for every request, improving performance
	sharedHTTPClient *http.Client
	clientOnce       sync.Once
)

// initSharedHTTPClient initializes the shared HTTP client with optimized settings
func initSharedHTTPClient() {
	clientOnce.Do(func() {
		transport := &http.Transport{
			MaxIdleConns:        100,              // Max idle connections across all hosts
			MaxIdleConnsPerHost: 10,               // Max idle connections per host
			IdleConnTimeout:     90 * time.Second, // How long idle connections stay open
			DisableCompression:  false,            // Enable compression
			ForceAttemptHTTP2:   true,             // Attempt HTTP/2
		}

		sharedHTTPClient = &http.Client{
			Transport: transport,
			Timeout:   30 * time.Second, // Default timeout (can be overridden per instance)
		}
	})
}

// Client is a base HTTP client for FOLIO API calls
type Client struct {
	httpClient *http.Client
	baseURL    string
	tenant     string
	timeout    time.Duration
	logger     *zap.Logger
}

// NewClient creates a new FOLIO API client using a shared HTTP client with connection pooling
func NewClient(baseURL, tenant string) *Client {
	// Initialize shared client on first use
	initSharedHTTPClient()

	return &Client{
		httpClient: sharedHTTPClient,
		baseURL:    baseURL,
		tenant:     tenant,
		timeout:    30 * time.Second,
		logger:     clientLogger,
	}
}

// SetTimeout sets the per-request timeout for this client instance.
// On each request, a context.WithTimeout deadline of this duration is applied,
// cancelling the request if it does not complete within the allotted time.
// This does not modify the shared HTTP client; the deadline is enforced via context.
func (c *Client) SetTimeout(timeout time.Duration) {
	c.timeout = timeout
}

// Get performs a GET request
func (c *Client) Get(ctx context.Context, path string, token string, result interface{}) error {
	return c.doRequest(ctx, "GET", path, token, nil, result)
}

// Post performs a POST request
func (c *Client) Post(ctx context.Context, path string, token string, body interface{}, result interface{}) error {
	return c.doRequest(ctx, "POST", path, token, body, result)
}

// Put performs a PUT request
func (c *Client) Put(ctx context.Context, path string, token string, body interface{}, result interface{}) error {
	return c.doRequest(ctx, "PUT", path, token, body, result)
}

// Delete performs a DELETE request
func (c *Client) Delete(ctx context.Context, path string, token string) error {
	return c.doRequest(ctx, "DELETE", path, token, nil, nil)
}

// PostWithTextPlainAccept performs a POST request with Accept: text/plain header
func (c *Client) PostWithTextPlainAccept(ctx context.Context, path string, token string, body interface{}) error {
	return c.doRequestWithCustomAccept(ctx, "POST", path, token, body, nil, "text/plain")
}

// PutWithTextPlainAccept performs a PUT request with Accept: text/plain header
func (c *Client) PutWithTextPlainAccept(ctx context.Context, path string, token string, body interface{}) error {
	return c.doRequestWithCustomAccept(ctx, "PUT", path, token, body, nil, "text/plain")
}

// doRequest performs an HTTP request with proper headers
func (c *Client) doRequest(ctx context.Context, method, path, token string, body interface{}, result interface{}) error {
	return c.doRequestWithCustomAccept(ctx, method, path, token, body, result, "application/json")
}

// doRequestWithCustomAccept performs an HTTP request with a custom Accept header
func (c *Client) doRequestWithCustomAccept(ctx context.Context, method, path, token string, body interface{}, result interface{}, acceptHeader string) error {
	url := c.baseURL + path

	// Apply client timeout using context
	// This allows per-instance timeout configuration while using a shared HTTP client
	if c.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.timeout)
		defer cancel()
	}

	var bodyReader io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewBuffer(jsonData)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set required headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", acceptHeader)
	req.Header.Set("X-Okapi-Tenant", c.tenant)

	// Add authentication token if provided
	if token != "" {
		req.Header.Set("X-Okapi-Token", token)
	}

	// Log the outbound request (debug only)
	if c.logger != nil {
		c.logger.Debug("FOLIO API request",
			logging.TypeField(logging.TypeFolioRequest),
			zap.String("method", method),
			zap.String("url", url),
			zap.String("tenant", c.tenant),
		)
	}

	// Perform the request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	// Check for HTTP errors
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if c.logger != nil {
			c.logger.Debug("FOLIO API response",
				logging.TypeField(logging.TypeFolioResponse),
				zap.String("method", method),
				zap.String("url", url),
				zap.Int("status_code", resp.StatusCode),
				zap.Int("response_bytes", len(respBody)),
			)
		}
		return &HTTPError{
			StatusCode: resp.StatusCode,
			Status:     resp.Status,
			Body:       string(respBody),
			URL:        url,
			Method:     method,
		}
	}

	// Log the successful response (debug only)
	if c.logger != nil {
		c.logger.Debug("FOLIO API response",
			logging.TypeField(logging.TypeFolioResponse),
			zap.String("method", method),
			zap.String("url", url),
			zap.Int("status_code", resp.StatusCode),
			zap.Int("response_bytes", len(respBody)),
		)
	}

	// Parse response if result is provided (only for JSON responses)
	if result != nil && len(respBody) > 0 && acceptHeader == "application/json" {
		if err := json.Unmarshal(respBody, result); err != nil {
			return fmt.Errorf("failed to parse response: %w", err)
		}
	}

	return nil
}

// HTTPError represents an HTTP error response
type HTTPError struct {
	StatusCode int
	Status     string
	Body       string
	URL        string
	Method     string
}

// Error implements the error interface
func (e *HTTPError) Error() string {
	return fmt.Sprintf("HTTP %d: %s [%s %s]\n%s", e.StatusCode, e.Status, e.Method, e.URL, e.Body)
}

// IsNotFound checks if the error is a 404 Not Found
func (e *HTTPError) IsNotFound() bool {
	return e.StatusCode == http.StatusNotFound
}

// FolioErrorResponse represents a FOLIO error response structure
type FolioErrorResponse struct {
	Message    string                   `json:"message"`
	Type       string                   `json:"type"`
	Code       string                   `json:"code"`
	Parameters []map[string]interface{} `json:"parameters"`
}

// ParseErrorMessage extracts a user-friendly error message from the HTTPError
func (e *HTTPError) ParseErrorMessage() string {
	if e.Body == "" {
		return "Unknown error"
	}

	// Try to parse as FOLIO error response
	var folioError FolioErrorResponse
	if err := json.Unmarshal([]byte(e.Body), &folioError); err == nil && folioError.Message != "" {
		return folioError.Message
	}

	// Try to parse as errors array format
	var errorsResponse struct {
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal([]byte(e.Body), &errorsResponse); err == nil && len(errorsResponse.Errors) > 0 && errorsResponse.Errors[0].Message != "" {
		return errorsResponse.Errors[0].Message
	}

	// Fallback: return first 200 characters of body
	if len(e.Body) > 200 {
		return e.Body[:200] + "..."
	}
	return e.Body
}

// IsUnauthorized checks if the error is a 401 Unauthorized
func (e *HTTPError) IsUnauthorized() bool {
	return e.StatusCode == http.StatusUnauthorized
}

// IsForbidden checks if the error is a 403 Forbidden
func (e *HTTPError) IsForbidden() bool {
	return e.StatusCode == http.StatusForbidden
}

// IsBadRequest checks if the error is a 400 Bad Request
func (e *HTTPError) IsBadRequest() bool {
	return e.StatusCode == http.StatusBadRequest
}

// IsServerError checks if the error is a 5xx server error
func (e *HTTPError) IsServerError() bool {
	return e.StatusCode >= 500 && e.StatusCode < 600
}
