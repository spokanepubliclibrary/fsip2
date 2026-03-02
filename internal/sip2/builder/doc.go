// Package builder implements SIP2 protocol response message construction for fsip2.
//
// This package provides a builder pattern for constructing SIP2 protocol response messages
// that are sent back to library self-service kiosks. It handles field formatting, delimiter
// insertion, checksum calculation, and message assembly according to the SIP2 specification.
//
// # Architecture
//
// The ResponseBuilder provides dedicated methods for each SIP2 response message type:
//   - BuildCheckoutResponse (message 12)
//   - BuildCheckinResponse (message 10)
//   - BuildRenewResponse (message 30)
//   - BuildRenewAllResponse (message 66)
//   - BuildPatronStatusResponse (message 24)
//   - BuildPatronInformationResponse (message 64)
//   - BuildItemInformationResponse (message 18)
//   - BuildItemStatusUpdateResponse (message 20)
//   - BuildFeePaidResponse (message 38)
//   - BuildLoginResponse (message 94)
//   - BuildEndSessionResponse (message 36)
//   - BuildACSStatusResponse (message 98)
//   - BuildResendResponse (message 96)
//
// # Response Building Flow
//
//  1. Call appropriate Build*Response() method with response data
//  2. Builder constructs message with fixed-length fields
//  3. Builder adds variable-length fields with delimiters
//  4. Builder applies tenant-specific field filtering (IsFieldEnabled)
//  5. Builder adds checksum if error detection is enabled
//  6. Build() returns complete SIP2 response message as []byte
//
// # Field Construction
//
// The builder uses helper functions from the protocol package:
//   - protocol.BuildField(code, value, delimiter) - adds field with delimiter
//   - protocol.BuildOptionalField(code, value, delimiter) - adds field only if value is not empty
//   - protocol.BuildYNField(bool) - converts boolean to "Y" or "N"
//   - protocol.FormatSIP2DateTime(time, timezone) - formats time in SIP2 format (YYYYMMDD    HHMMSS)
//
// # Field Filtering
//
// Tenant configurations can enable/disable specific fields per message type using IsFieldEnabled().
// This allows customization of responses based on kiosk vendor requirements.
//
// Example:
//
//	if tenantConfig.IsFieldEnabled("12", "BF") {
//	    // Include patron currency type (BF) field in checkout response
//	}
//
// # Checksum Calculation
//
// When error detection is enabled (ErrorDetectionEnabled = true), the builder:
//   - Appends sequence number field: AY<seq>
//   - Calculates checksum using CalculateChecksum()
//   - Appends checksum field: AZ<checksum>
//
// The checksum algorithm ensures message integrity during transmission.
//
// # SIP2 Response Format
//
// SIP2 responses follow this structure:
//
//	<2-char type><fixed fields><variable fields><checksum>
//
// Example checkout response (message 12):
//
//	121NNY19700101    084625AO|AB123456|AJ|AA12345678|BK|
//	\__/\_/\_________/\__________________________________/
//	 |   |      |                     |
//	 |   |      |                     Variable-length fields
//	 |   |      Transaction date (fixed field)
//	 |   ok, renewal ok, magnetic media flags (fixed fields)
//	 Message type (12 = Checkout Response)
//
// # Performance
//
// The builder is optimized for:
//   - String concatenation efficiency (minimal allocations)
//   - Conditional field inclusion (only include needed fields)
//   - Reusable builder instances (NewResponseBuilder can be called per-session)
//
// # Thread Safety
//
// ResponseBuilder instances are NOT thread-safe. Create a new instance per request
// or protect with appropriate synchronization.
//
// # Usage Example
//
//	builder := NewResponseBuilder(tenantConfig)
//	response := builder.BuildCheckoutResponse(CheckoutResponseData{
//	    Ok: true,
//	    RenewalOk: true,
//	    MagneticMedia: false,
//	    Desensitize: false,
//	    TransactionDate: time.Now(),
//	    InstitutionID: "MYLIB",
//	    PatronID: "12345678",
//	    ItemID: "123456",
//	    TitleID: "The Great Gatsby",
//	    DueDate: dueDate,
//	    // ... other fields
//	})
//	return response, nil
package builder
