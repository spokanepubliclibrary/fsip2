// Package handlers implements SIP2 protocol message handlers for fsip2.
//
// This package provides handler implementations for all 13 supported SIP2 message types,
// bridging SIP2 protocol messages from library self-service kiosks with FOLIO library
// management system APIs.
//
// # Architecture
//
// All handlers extend BaseHandler which provides common functionality:
//   - Session management
//   - FOLIO API client creation
//   - Patron verification
//   - Field validation
//   - Response building
//
// Each handler implements the Handler interface with a Handle() method that:
//  1. Parses the SIP2 request message
//  2. Validates required fields
//  3. Makes necessary FOLIO API calls
//  4. Builds and returns a SIP2 response message
//
// # Supported Message Types
//
//   - Message 23/24: Patron Status Request/Response
//   - Message 63/64: Patron Information Request/Response
//   - Message 09/10: Checkin Request/Response
//   - Message 11/12: Checkout Request/Response
//   - Message 29/30: Renew Request/Response
//   - Message 65/66: Renew All Request/Response
//   - Message 17/18: Item Information Request/Response
//   - Message 19/20: Item Status Update Request/Response
//   - Message 37/38: Fee Paid Request/Response
//   - Message 93/94: Login Request/Response
//   - Message 35/36: End Patron Session Request/Response
//   - Message 99/98: SC Status Request/ACS Status Response
//   - Message 97/96: Resend Request/Response
//
// # Error Handling
//
// Handlers return SIP2-compliant error responses when:
//   - Required fields are missing
//   - FOLIO API calls fail
//   - Patron verification fails
//   - Items are not found or unavailable
//   - Permissions are denied
//
// # Performance
//
// Handlers are optimized for performance with:
//   - HTTP client connection pooling (via folio package)
//   - Parallel API calls for independent operations (e.g., checkin handler)
//   - Cached patron status and name formatting (buildPatronStatusString, formatPatronName)
//   - Efficient response building using ResponseBuilder pattern
//
// # Testing
//
// Each handler has corresponding unit tests validating:
//   - Successful operation scenarios
//   - Error handling paths
//   - Field validation logic
//   - Response format compliance
//   - Session state management
package handlers
