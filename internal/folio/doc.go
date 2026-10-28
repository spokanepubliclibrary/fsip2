// Package folio provides a client library for interacting with FOLIO library management system APIs.
//
// This package implements HTTP clients for various FOLIO modules including patron management,
// circulation, inventory, and fees. It handles authentication, error handling, and provides
// strongly-typed Go structs for FOLIO API requests and responses.
//
// # Architecture
//
// The package is organized into several specialized clients:
//   - Client: Base HTTP client with authentication and request handling
//   - PatronClient: User authentication, patron lookup, and block management
//   - CirculationClient: Checkout, checkin, renewal, loan, and request operations
//   - InventoryClient: Item, holding, instance, location, and material type lookups
//   - FeesClient: Account and payment operations
//
// # Authentication
//
// Kiosk authentication (AuthClient) works the same on Eureka and Okapi Poppy+:
//  1. Login posts username/password to /authn/login-with-expiry. The access and refresh
//     tokens arrive as folioAccessToken / folioRefreshToken Set-Cookie headers; the body
//     carries their absolute expirations.
//  2. The access token is sent as X-Okapi-Token on every subsequent request.
//  3. Refresh posts only the folioRefreshToken cookie to /authn/refresh and receives a new
//     pair. Refresh tokens rotate, so both stored tokens are replaced together.
//  4. If refresh fails or the refresh token has expired, the caller logs in again.
//  5. Logout posts the folioRefreshToken and folioAccessToken cookies to /authn/logout,
//     which ends only the Keycloak session that refresh token belongs to. It refuses (no
//     request) an empty, whitespace or non-JWT-shaped refresh token, or an empty access
//     token: on Eureka an access token with a blank refresh token ends EVERY session of
//     the user. The access token must be current (the gateway rejects an expired one), so
//     callers refresh first; handlers.LogoutTokens does that and sends the request to the
//     tenant that issued the pair.
//
// Tokens are owned per SIP connection by types.Session; there is no shared token cache.
// The /authn/* calls never send X-Okapi-Token (PostWithCookies with an empty token).
//
// # HTTP Client Pooling
//
// The package uses a shared HTTP client with connection pooling:
//   - Singleton http.Client initialized once (sync.Once)
//   - Connection pool configuration:
//     * MaxIdleConns: 100
//     * MaxIdleConnsPerHost: 10
//     * IdleConnTimeout: 90 seconds
//   - Reuses TCP connections across all FOLIO API calls
//   - Reduces connection overhead by 20-30%
//   - HTTP/2 support enabled for multiplexing
//
// # Error Handling
//
// The package provides rich error handling:
//
//   - HTTPError: Structured error with status code, URL, and response body
//   - IsNotFound(): Checks for 404 errors
//   - IsUnauthorized(): Checks for 401 errors
//   - IsForbidden(): Checks for 403 errors
//   - IsBadRequest(): Checks for 400 errors
//   - IsPermissionError(): Checks for permission-related errors
//
// FOLIO error responses are parsed and included in the error message.
//
// # Context Support
//
// All API methods accept context.Context for:
//   - Request cancellation
//   - Timeout enforcement
//   - Deadline propagation
//   - Graceful shutdown support
//
// # Request/Response Flow
//
//  1. Client creates HTTP request with context
//  2. Sets required Okapi headers (x-okapi-tenant, x-okapi-token, x-okapi-url)
//  3. Marshals request body to JSON (if applicable)
//  4. Sends request via shared HTTP client
//  5. Reads response body
//  6. Checks for HTTP errors (4xx, 5xx)
//  7. Unmarshals response JSON to Go struct
//  8. Returns typed result or error
//
// # Performance Optimizations
//
// The client is optimized with:
//   - HTTP connection pooling (shared client)
//   - Context timeouts (default: 30 seconds)
//   - Parallel API calls in handlers (e.g., checkin handler)
//   - Efficient JSON marshaling/unmarshaling
//   - Minimal memory allocations
//
// # FOLIO API Modules
//
// PatronClient (/users, /bl-users, /automated-patron-blocks, /manualblocks):
//   - GetUserByBarcode: Lookup patron by barcode
//   - GetUserByID: Lookup patron by UUID
//   - Authenticate: Verify patron credentials
//   - GetAutomatedBlocksByUserId: Get system-generated blocks
//   - GetManualBlocksByUserId: Get staff-created blocks
//
// CirculationClient (/circulation):
//   - PostCheckout: Check out an item to a patron
//   - PostCheckin: Check in an item
//   - PostRenew: Renew a loan
//   - GetLoansByUserId: Get patron's active loans
//   - GetRequestsByUserId: Get patron's requests (holds)
//   - GetRequestsByItem: Get requests for a specific item
//
// InventoryClient (/inventory, /holdings-storage, /instance-storage):
//   - GetItemByBarcode: Lookup item by barcode
//   - GetItemByID: Lookup item by UUID
//   - GetHoldingsByID: Get holdings record
//   - GetInstanceByID: Get instance (title) record
//   - GetLocationByID: Get location details
//   - GetMaterialTypeByID: Get material type details
//   - GetServicePointByID: Get service point details
//
// FeesClient (/accounts):
//   - GetAccountsByUserId: Get patron's fees/fines
//   - PostPayment: Record a payment
//
// # Usage Example
//
//	// Create clients
//	patronClient := folio.NewPatronClient(okapiURL, tenant)
//	circClient := folio.NewCirculationClient(okapiURL, tenant)
//
//	// Authenticate patron
//	token, err := patronClient.Authenticate(ctx, username, password)
//	if err != nil {
//	    return fmt.Errorf("authentication failed: %w", err)
//	}
//
//	// Lookup user
//	user, err := patronClient.GetUserByBarcode(ctx, token, "12345678")
//	if err != nil {
//	    if folio.IsNotFound(err) {
//	        return errors.New("patron not found")
//	    }
//	    return err
//	}
//
//	// Checkout item
//	loan, err := circClient.PostCheckout(ctx, token, &folio.CheckoutRequest{
//	    ItemBarcode:  "ITEM123",
//	    UserBarcode:  "12345678",
//	    ServicePointId: spID,
//	})
package folio
