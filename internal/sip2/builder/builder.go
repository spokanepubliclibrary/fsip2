package builder

import (
	"fmt"
	"strings"
	"time"

	"github.com/spokanepubliclibrary/fsip2/internal/config"
	"github.com/spokanepubliclibrary/fsip2/internal/sip2/parser"
	"github.com/spokanepubliclibrary/fsip2/internal/sip2/protocol"
)

// ResponseBuilder builds SIP2 response messages
type ResponseBuilder struct {
	config *config.TenantConfig
}

// NewResponseBuilder creates a new response builder
func NewResponseBuilder(cfg *config.TenantConfig) *ResponseBuilder {
	return &ResponseBuilder{
		config: cfg,
	}
}

// Build builds a complete SIP2 response message with optional checksum
// Build builds a complete SIP2 response message with optional checksum
func (b *ResponseBuilder) Build(code parser.MessageCode, content string, sequenceNumber string) (string, error) {
	// Start with message code
	response := string(code) + content

	// Add error detection fields if enabled OR if sequence number is provided
	if b.config.ErrorDetectionEnabled || sequenceNumber != "" {
		// Prepare content for checksum calculation (without trailing delimiter)
		checksumContent := content
		if strings.HasSuffix(checksumContent, b.config.FieldDelimiter) {
			checksumContent = strings.TrimSuffix(checksumContent, b.config.FieldDelimiter)
		}

		// Add AY field to response (delimiter already at end of content)
		if !strings.HasSuffix(response, b.config.FieldDelimiter) {
			response += b.config.FieldDelimiter
		}
		response += "AY" + sequenceNumber

		// Add checksum if error detection is enabled
		if b.config.ErrorDetectionEnabled {
			encoder, err := protocol.GetEncoder(b.config.Charset)
			if err != nil {
				return "", fmt.Errorf("failed to get encoder: %w", err)
			}

			checksum, err := parser.CalculateChecksum(string(code)+checksumContent, sequenceNumber, b.config.FieldDelimiter, encoder)
			if err != nil {
				return "", fmt.Errorf("failed to calculate checksum: %w", err)
			}

			response += "AZ" + checksum
		}
	}

	// Add message delimiter
	response += b.config.MessageDelimiter

	return response, nil
}



// BuildLoginResponse builds a login response (94)
func (b *ResponseBuilder) BuildLoginResponse(ok bool, sequenceNumber string) (string, error) {
	status := "0"
	if ok {
		status = "1"
	}

	return b.Build(parser.LoginResponse, status, sequenceNumber)
}

// BuildPatronStatusResponse builds a patron status response (24)
func (b *ResponseBuilder) BuildPatronStatusResponse(
	patronStatus string, // 14-character status
	language string,
	transactionDate time.Time,
	institutionID string,
	patronID string,
	personalName string,
	validPatron bool,
	validPatronPassword bool,
	currencyType string,
	feeAmount string,
	sequenceNumber string,
) (string, error) {
	// Fixed fields
	content := patronStatus // 14 characters
	content += language     // 3 characters
	content += protocol.FormatSIP2DateTime(transactionDate, b.config.Timezone)

	// Variable fields
	delimiter := b.config.FieldDelimiter

	content += protocol.BuildField(string(parser.InstitutionID), institutionID, delimiter)
	content += protocol.BuildField(string(parser.PatronIdentifier), patronID, delimiter)
	content += protocol.BuildField(string(parser.PersonalName), personalName, delimiter)

	if validPatron {
		content += protocol.BuildField(string(parser.ValidPatron), "Y", delimiter)
	}

	if validPatronPassword {
		content += protocol.BuildField(string(parser.ValidPatronPassword), "Y", delimiter)
	}

	if currencyType != "" {
		content += protocol.BuildField(string(parser.CurrencyType), currencyType, delimiter)
	}

	if feeAmount != "" {
		content += protocol.BuildField(string(parser.FeeAmount), feeAmount, delimiter)
	}

	return b.Build(parser.PatronStatusResponse, content, sequenceNumber)
}

// BuildCheckoutResponse builds a checkout response (12)
func (b *ResponseBuilder) BuildCheckoutResponse(
	ok bool,
	renewalOK bool,
	magneticMedia bool,
	desensitize bool,
	transactionDate time.Time,
	institutionID string,
	patronID string,
	itemID string,
	titleID string,
	dueDate time.Time,
	feeType string,
	securityInhibit bool,
	currencyType string,
	feeAmount string,
	mediaType string,
	itemProperties string,
	transactionID string,
	screenMessage []string,
	printLine []string,
	sequenceNumber string,
) (string, error) {
	// Fixed fields
	// ok field: 1 digit (0 or 1), not Y/N per SIP2 spec
	var content string
	if ok {
		content = "1"
	} else {
		content = "0"
	}
	content += protocol.BuildYNField(renewalOK)
	// magneticMedia and desensitize: if false, use 'U' for unknown instead of 'N'
	if magneticMedia {
		content += "Y"
	} else {
		content += "U"
	}
	if desensitize {
		content += "Y"
	} else {
		content += "U"
	}
	content += protocol.FormatSIP2DateTime(transactionDate, b.config.Timezone)

	// Variable fields
	delimiter := b.config.FieldDelimiter

	content += protocol.BuildField(string(parser.InstitutionID), institutionID, delimiter)
	content += protocol.BuildField(string(parser.PatronIdentifier), patronID, delimiter)
	content += protocol.BuildField(string(parser.ItemIdentifier), itemID, delimiter)
	content += protocol.BuildField(string(parser.TitleIdentifier), titleID, delimiter)

	if !dueDate.IsZero() {
		content += protocol.BuildField(string(parser.DueDate), protocol.FormatSIP2DateTime(dueDate, b.config.Timezone), delimiter)
	}

	content += protocol.BuildOptionalField(string(parser.FeeType), feeType, delimiter)

	if securityInhibit {
		content += "CI" + "Y" + delimiter
	}

	content += protocol.BuildOptionalField(string(parser.CurrencyType), currencyType, delimiter)
	content += protocol.BuildOptionalField(string(parser.FeeAmount), feeAmount, delimiter)
	content += protocol.BuildOptionalField(string(parser.MediaType), mediaType, delimiter)
	content += protocol.BuildOptionalField("BV", itemProperties, delimiter) // Item properties
	content += protocol.BuildOptionalField(string(parser.TransactionID), transactionID, delimiter)

	// Add screen messages
	for _, msg := range screenMessage {
		content += protocol.BuildField(string(parser.ScreenMessage), msg, delimiter)
	}

	// Add print lines
	for _, line := range printLine {
		content += protocol.BuildField(string(parser.PrintLine), line, delimiter)
	}

	return b.Build(parser.CheckoutResponse, content, sequenceNumber)
}

// BuildRenewResponse builds a renew response (30)
func (b *ResponseBuilder) BuildRenewResponse(
	ok bool,
	renewalOK bool,
	magneticMedia bool,
	desensitize bool,
	transactionDate time.Time,
	institutionID string,
	patronID string,
	itemID string,
	titleID string,
	dueDate time.Time,
	screenMessage []string,
	printLine []string,
	sequenceNumber string,
) (string, error) {
	// Fixed fields
	// ok field: 1 digit (0 or 1), not Y/N per SIP2 spec
	var content string
	if ok {
		content = "1"
	} else {
		content = "0"
	}
	content += protocol.BuildYNField(renewalOK)
	// magneticMedia and desensitize: if false, use 'U' for unknown instead of 'N'
	if magneticMedia {
		content += "Y"
	} else {
		content += "U"
	}
	if desensitize {
		content += "Y"
	} else {
		content += "U"
	}
	content += protocol.FormatSIP2DateTime(transactionDate, b.config.Timezone)

	// Variable fields
	delimiter := b.config.FieldDelimiter

	content += protocol.BuildField(string(parser.InstitutionID), institutionID, delimiter)
	content += protocol.BuildField(string(parser.PatronIdentifier), patronID, delimiter)
	content += protocol.BuildField(string(parser.ItemIdentifier), itemID, delimiter)

	// AJ field - Title Identifier (Instance UUID in our implementation)
	if titleID != "" {
		content += protocol.BuildField(string(parser.TitleIdentifier), titleID, delimiter)
	}

	// AH field - Due date (only if renewal was successful)
	if !dueDate.IsZero() {
		content += protocol.BuildField(string(parser.DueDate), protocol.FormatSIP2DateTime(dueDate, b.config.Timezone), delimiter)
	}

	// Add screen messages
	for _, msg := range screenMessage {
		content += protocol.BuildField(string(parser.ScreenMessage), msg, delimiter)
	}

	// Add print lines
	for _, line := range printLine {
		content += protocol.BuildField(string(parser.PrintLine), line, delimiter)
	}

	return b.Build(parser.RenewResponse, content, sequenceNumber)
}

// BuildCheckinResponse builds a checkin response (10)
func (b *ResponseBuilder) BuildCheckinResponse(
	ok bool,
	resensitize bool,
	magneticMedia bool,
	alert bool,
	transactionDate time.Time,
	institutionID string,
	itemID string,
	permanentLocation string,
	currentLocation string,
	titleID string,
	materialType string,
	mediaType string,
	callNumber string,
	alertType string,
	destinationLocation string,
	sortBin string,
	patronID string,
	itemProperties string,
	checkinNotes []string,
	holdShelfExpiration string,
	requestorName string,
	screenMessage []string,
	printLine []string,
	sequenceNumber string,
) (string, error) {
	// Fixed fields
	// ok field: 1 digit (0 or 1), not Y/N per SIP2 spec
	var content string
	if ok {
		content = "1"
	} else {
		content = "0"
	}
	content += protocol.BuildYNField(resensitize)
	content += protocol.BuildYNField(magneticMedia)
	content += protocol.BuildYNField(alert)
	content += protocol.FormatSIP2DateTime(transactionDate, b.config.Timezone)

	// Variable fields
	delimiter := b.config.FieldDelimiter

	// Required fields
	content += protocol.BuildField(string(parser.InstitutionID), institutionID, delimiter)
	content += protocol.BuildField(string(parser.ItemIdentifier), itemID, delimiter)

	// AQ - Permanent location (always include, even if blank per SIP2 spec)
	content += string(parser.PermanentLocation) + permanentLocation + delimiter

	// AP - Current location (always include, even if blank)
	content += string(parser.CurrentLocation) + currentLocation + delimiter

	// AJ - Title identifier (always include, with fallback to item barcode)
	if titleID != "" {
		content += protocol.BuildField(string(parser.TitleIdentifier), titleID, delimiter)
	} else {
		content += protocol.BuildField(string(parser.TitleIdentifier), itemID, delimiter)
	}

	// CK - Media type (SIP2 code) (always include, even if blank)
	content += string(parser.MediaType) + mediaType + delimiter

	// CH - Material type (plain text) (always include, even if blank)
	content += string(parser.CurrentItemType) + materialType + delimiter

	// CS - Call number (always include, even if blank)
	content += string(parser.CallNumber) + callNumber + delimiter

	// CV - Alert type (always include, even if blank)
	content += string(parser.AlertType) + alertType + delimiter

	// CT - Destination/routing location (always include, even if blank)
	content += string(parser.PickupServicePoint) + destinationLocation + delimiter

	// CL - Sort bin (optional)
	content += protocol.BuildOptionalField("CL", sortBin, delimiter)

	// AA - Patron identifier (optional)
	content += protocol.BuildOptionalField(string(parser.PatronIdentifier), patronID, delimiter)

	// BV - Item properties (optional)
	content += protocol.BuildOptionalField("BV", itemProperties, delimiter)

	// AG - Checkin notes (optional, repeatable)
	for _, note := range checkinNotes {
		content += protocol.BuildField("AG", note, delimiter)
	}

	// CM - Hold shelf expiration date (configurable, omit if disabled or not present)
	if b.config.IsFieldEnabled("09", "CM") && holdShelfExpiration != "" {
		content += protocol.BuildField(string(parser.HoldShelfExpirationDate), holdShelfExpiration, delimiter)
	}

	// DA - Requestor name (configurable, omit if disabled or not present)
	if b.config.IsFieldEnabled("09", "DA") && requestorName != "" {
		content += protocol.BuildField(string(parser.TransactionDate), requestorName, delimiter)
	}

	// Add screen messages
	for _, msg := range screenMessage {
		content += protocol.BuildField(string(parser.ScreenMessage), msg, delimiter)
	}

	// Add print lines
	for _, line := range printLine {
		content += protocol.BuildField(string(parser.PrintLine), line, delimiter)
	}

	return b.Build(parser.CheckinResponse, content, sequenceNumber)
}

// BuildPatronInformationResponse builds a patron information response (64)
func (b *ResponseBuilder) BuildPatronInformationResponse(
	patronStatus string, // 14 characters
	language string,
	transactionDate time.Time,
	holdItemsCount int,
	overdueItemsCount int,
	chargedItemsCount int,
	fineItemsCount int,
	recallItemsCount int,
	unavailableHoldsCount int,
	institutionID string,
	patronID string,
	personalName string,
	holdItemsLimit int,
	overdueItemsLimit int,
	chargedItemsLimit int,
	validPatron bool,
	validPatronPassword bool,
	currencyType string,
	feeAmount string,
	feeLimit string,
	holdItems []string,
	overdueItems []string,
	chargedItems []string,
	fineItems []string,
	recallItems []string,
	unavailableHolds []string,
	homeAddress string,
	emailAddress string,
	homePhoneNumber string,
	screenMessage []string,
	printLine []string,
	sequenceNumber string,
) (string, error) {
	// Fixed fields
	content := patronStatus // 14 characters
	content += language     // 3 characters
	content += protocol.FormatSIP2DateTime(transactionDate, b.config.Timezone)
	content += fmt.Sprintf("%04d", holdItemsCount)
	content += fmt.Sprintf("%04d", overdueItemsCount)
	content += fmt.Sprintf("%04d", chargedItemsCount)
	content += fmt.Sprintf("%04d", fineItemsCount)
	content += fmt.Sprintf("%04d", recallItemsCount)
	content += fmt.Sprintf("%04d", unavailableHoldsCount)

	// Variable fields
	delimiter := b.config.FieldDelimiter

	content += protocol.BuildField(string(parser.InstitutionID), institutionID, delimiter)
	content += protocol.BuildField(string(parser.PatronIdentifier), patronID, delimiter)
	content += protocol.BuildField(string(parser.PersonalName), personalName, delimiter)

	if holdItemsLimit > 0 {
		content += protocol.BuildField("BZ", fmt.Sprintf("%04d", holdItemsLimit), delimiter)
	}
	if overdueItemsLimit > 0 {
		content += protocol.BuildField("CA", fmt.Sprintf("%04d", overdueItemsLimit), delimiter)
	}
	if chargedItemsLimit > 0 {
		content += protocol.BuildField("CB", fmt.Sprintf("%04d", chargedItemsLimit), delimiter)
	}

	if validPatron {
		content += protocol.BuildField(string(parser.ValidPatron), "Y", delimiter)
	}
	if validPatronPassword {
		content += protocol.BuildField(string(parser.ValidPatronPassword), "Y", delimiter)
	}

	content += protocol.BuildOptionalField(string(parser.CurrencyType), currencyType, delimiter)
	content += protocol.BuildOptionalField(string(parser.FeeAmount), feeAmount, delimiter)
	content += protocol.BuildOptionalField("CC", feeLimit, delimiter)

	// Add item lists
	for _, item := range holdItems {
		content += protocol.BuildField("AS", item, delimiter)
	}
	for _, item := range overdueItems {
		content += protocol.BuildField("AT", item, delimiter)
	}
	for _, item := range chargedItems {
		content += protocol.BuildField("AU", item, delimiter)
	}
	for _, item := range fineItems {
		content += protocol.BuildField("AV", item, delimiter)
	}
	for _, item := range recallItems {
		content += protocol.BuildField("BU", item, delimiter)
	}
	for _, item := range unavailableHolds {
		content += protocol.BuildField("CD", item, delimiter)
	}

	// Add patron details
	content += protocol.BuildOptionalField(string(parser.HomeAddress), homeAddress, delimiter)
	content += protocol.BuildOptionalField(string(parser.EmailAddress), emailAddress, delimiter)
	content += protocol.BuildOptionalField(string(parser.HomePhoneNumber), homePhoneNumber, delimiter)

	// Add screen messages
	for _, msg := range screenMessage {
		content += protocol.BuildField(string(parser.ScreenMessage), msg, delimiter)
	}

	// Add print lines
	for _, line := range printLine {
		content += protocol.BuildField(string(parser.PrintLine), line, delimiter)
	}

	return b.Build(parser.PatronInformationResponse, content, sequenceNumber)
}

// BuildACSStatusResponse builds an ACS status response (98)
func (b *ResponseBuilder) BuildACSStatusResponse(
	onlineStatus bool,
	checkinOK bool,
	checkoutOK bool,
	acsRenewalPolicy bool,
	statusUpdateOK bool,
	offlineOK bool,
	timeoutPeriod int,
	retriesAllowed int,
	dateTimeSync time.Time,
	protocolVersion string,
	institutionID string,
	libraryName string,
	supportedMessages string,
	terminalLocation string,
	screenMessage []string,
	printLine []string,
	sequenceNumber string,
) (string, error) {
	// Fixed fields
	content := protocol.BuildYNField(onlineStatus)
	content += protocol.BuildYNField(checkinOK)
	content += protocol.BuildYNField(checkoutOK)
	content += protocol.BuildYNField(acsRenewalPolicy)
	content += protocol.BuildYNField(statusUpdateOK)
	content += protocol.BuildYNField(offlineOK)
	content += fmt.Sprintf("%03d", timeoutPeriod)
	content += fmt.Sprintf("%03d", retriesAllowed)
	content += protocol.FormatSIP2DateTime(dateTimeSync, b.config.Timezone)
	content += protocolVersion // "2.00"

	// Variable fields
	delimiter := b.config.FieldDelimiter

	content += protocol.BuildField(string(parser.InstitutionID), institutionID, delimiter)
	content += protocol.BuildOptionalField(string(parser.LibraryName), libraryName, delimiter)
	content += protocol.BuildField("BX", supportedMessages, delimiter) // Supported messages
	content += protocol.BuildOptionalField(string(parser.TerminalLocation), terminalLocation, delimiter)

	// Add screen messages
	for _, msg := range screenMessage {
		content += protocol.BuildField(string(parser.ScreenMessage), msg, delimiter)
	}

	// Add print lines
	for _, line := range printLine {
		content += protocol.BuildField(string(parser.PrintLine), line, delimiter)
	}

	return b.Build(parser.ACSStatus, content, sequenceNumber)
}

// BuildSupportedMessagesString builds the supported messages string for ACS status
// Format: Y/N for each message type in order
func BuildSupportedMessagesString(supportedMessages []parser.MessageCode) string {
	// Define all possible message codes in order
	allMessages := []parser.MessageCode{
		parser.PatronStatusRequest,
		parser.CheckoutRequest,
		parser.CheckinRequest,
		parser.BlockPatron,
		parser.SCStatus,
		parser.RequestACSResend,
		parser.LoginRequest,
		parser.PatronInformationRequest,
		parser.EndPatronSessionRequest,
		parser.FeePaidRequest,
		parser.ItemInformationRequest,
		parser.ItemStatusUpdateRequest,
		parser.PatronEnableRequest,
		parser.HoldRequest,
		parser.RenewRequest,
		parser.RenewAllRequest,
	}

	var result strings.Builder
	for _, msg := range allMessages {
		supported := false
		for _, supported_msg := range supportedMessages {
			if msg == supported_msg {
				supported = true
				break
			}
		}
		if supported {
			result.WriteString("Y")
		} else {
			result.WriteString("N")
		}
	}

	return result.String()
}

// BuildItemInformationResponse builds an item information response (18)
func (b *ResponseBuilder) BuildItemInformationResponse(
	circulationStatus string,
	securityMarker string,
	feeType string,
	transactionDate time.Time,
	institutionID string,
	itemID string,
	title string,
	permanentLocation string,
	currentLocation string,
	dueDate string,
	mediaType string,
	materialType string,
	callNumber string,
	routingLocation string,
	holdQueueLength string,
	primaryContributor string,
	workDescription string,
	isbns []string,
	upcs []string,
	holdShelfExpiration string,
	requestorBarcode string,
	requestorName string,
	screenMessage []string,
	printLine []string,
	sequenceNumber string,
) (string, error) {
	// Fixed fields
	content := circulationStatus                                               // 2 characters
	content += securityMarker                                                  // 2 characters
	content += feeType                                                         // 2 characters
	content += protocol.FormatSIP2DateTime(transactionDate, b.config.Timezone) // 18 characters

	// Variable fields
	delimiter := b.config.FieldDelimiter

	// DEBUG: Ensure delimiter is not empty (fallback to pipe)
	if delimiter == "" {
		delimiter = "|"
	}

	// Required fields
	content += protocol.BuildField(string(parser.InstitutionID), institutionID, delimiter)
	content += protocol.BuildField(string(parser.ItemIdentifier), itemID, delimiter)

	// AJ - Title identifier (configurable)
	if b.config.IsFieldEnabled("17", "AJ") {
		content += protocol.BuildField(string(parser.TitleIdentifier), title, delimiter)
	}

	// AQ - Permanent location (configurable)
	if b.config.IsFieldEnabled("17", "AQ") {
		content += protocol.BuildField(string(parser.PermanentLocation), permanentLocation, delimiter)
	}

	// AP - Current location (configurable)
	if b.config.IsFieldEnabled("17", "AP") {
		content += protocol.BuildField(string(parser.CurrentLocation), currentLocation, delimiter)
	}

	// AH - Due date (configurable, conditional on checkout)
	if b.config.IsFieldEnabled("17", "AH") && dueDate != "" {
		content += protocol.BuildField(string(parser.DueDate), dueDate, delimiter)
	}

	// CK - Media type (configurable)
	if b.config.IsFieldEnabled("17", "CK") {
		content += protocol.BuildField(string(parser.MediaType), mediaType, delimiter)
	}

	// CH - Material type (configurable)
	if b.config.IsFieldEnabled("17", "CH") {
		content += protocol.BuildField(string(parser.CurrentItemType), materialType, delimiter)
	} else {
		// DEBUG: Log when CH is disabled
		// fmt.Printf("DEBUG: CH field disabled, skipping materialType=%s\n", materialType)
	}

	// CS - Call number (configurable)
	if b.config.IsFieldEnabled("17", "CS") {
		content += protocol.BuildField(string(parser.CallNumber), callNumber, delimiter)
	}

	// CT - Routing location (configurable)
	if b.config.IsFieldEnabled("17", "CT") {
		content += protocol.BuildField(string(parser.PickupServicePoint), routingLocation, delimiter)
	}

	// CF - Hold queue length (configurable)
	if b.config.IsFieldEnabled("17", "CF") {
		content += protocol.BuildField(string(parser.HoldQueueLength), holdQueueLength, delimiter)
	}

	// EA - Primary contributor (configurable, omit if disabled or not present)
	if b.config.IsFieldEnabled("17", "EA") && primaryContributor != "" {
		content += protocol.BuildField(string(parser.PrimaryContributor), primaryContributor, delimiter)
	}

	// DE - Work description (configurable, omit if disabled or not present)
	if b.config.IsFieldEnabled("17", "DE") && workDescription != "" {
		content += protocol.BuildField(string(parser.WorkDescription), workDescription, delimiter)
	}

	// IN - ISBNs (configurable, repeatable, omit if disabled)
	if b.config.IsFieldEnabled("17", "IN") {
		for _, isbn := range isbns {
			content += protocol.BuildField(string(parser.ISBNIdentifier), isbn, delimiter)
		}
	}

	// NB - UPCs (configurable, repeatable, omit if disabled)
	if b.config.IsFieldEnabled("17", "NB") {
		for _, upc := range upcs {
			content += protocol.BuildField(string(parser.OtherStandardID), upc, delimiter)
		}
	}

	// CM - Hold shelf expiration date (configurable, omit if disabled or not present)
	if b.config.IsFieldEnabled("17", "CM") && holdShelfExpiration != "" {
		content += protocol.BuildField(string(parser.HoldShelfExpirationDate), holdShelfExpiration, delimiter)
	}

	// CY - Requestor barcode (configurable, omit if disabled or not present)
	if b.config.IsFieldEnabled("17", "CY") && requestorBarcode != "" {
		content += protocol.BuildField(string(parser.RequestorBarcode), requestorBarcode, delimiter)
	}

	// DA - Requestor name (configurable, omit if disabled or not present)
	if b.config.IsFieldEnabled("17", "DA") && requestorName != "" {
		content += protocol.BuildField(string(parser.TransactionDate), requestorName, delimiter)
	}

	// Add screen messages
	for _, msg := range screenMessage {
		content += protocol.BuildField(string(parser.ScreenMessage), msg, delimiter)
	}

	// Add print lines
	for _, line := range printLine {
		content += protocol.BuildField(string(parser.PrintLine), line, delimiter)
	}

	return b.Build(parser.ItemInformationResponse, content, sequenceNumber)
}

// BuildRenewAllResponse builds a renew all response (66)
func (b *ResponseBuilder) BuildRenewAllResponse(
	ok bool,
	renewedCount int,
	unrenewedCount int,
	transactionDate time.Time,
	institutionID string,
	patronID string,
	renewedItems []string,
	unrenewedItems []string,
	screenMessage []string,
	sequenceNumber string,
) (string, error) {
	// Fixed fields
	content := protocol.BuildYNField(ok)
	content += fmt.Sprintf("%04d", renewedCount)
	content += fmt.Sprintf("%04d", unrenewedCount)
	content += protocol.FormatSIP2DateTime(transactionDate, b.config.Timezone)

	// Variable fields
	delimiter := b.config.FieldDelimiter

	content += protocol.BuildField(string(parser.InstitutionID), institutionID, delimiter)
	content += protocol.BuildField(string(parser.PatronIdentifier), patronID, delimiter)

	// BM - Renewed items count (required)
	content += protocol.BuildField("BM", fmt.Sprintf("%04d", renewedCount), delimiter)

	// BN - Unrenewed items count (required, always included even if zero)
	content += protocol.BuildField("BN", fmt.Sprintf("%04d", unrenewedCount), delimiter)

	// Add screen messages
	for _, msg := range screenMessage {
		content += protocol.BuildField(string(parser.ScreenMessage), msg, delimiter)
	}

	return b.Build(parser.RenewAllResponse, content, sequenceNumber)
}

// BuildPatronStatusString builds the 14-character patron status string
// Each character is Y (yes) or a space (no) for each status flag
func BuildPatronStatusString(
	chargePrivilegesDenied bool,
	renewalPrivilegesDenied bool,
	recallPrivilegesDenied bool,
	holdPrivilegesDenied bool,
	cardReportedLost bool,
	tooManyItemsCharged bool,
	tooManyItemsOverdue bool,
	tooManyRenewals bool,
	tooManyClaimsReturned bool,
	tooManyItemsLost bool,
	excessiveOutstandingFines bool,
	excessiveOutstandingFees bool,
	recallOverdue bool,
	tooManyItemsBilled bool,
) string {
	status := make([]byte, 14)
	for i := range status {
		status[i] = ' '
	}

	if chargePrivilegesDenied {
		status[0] = 'Y'
	}
	if renewalPrivilegesDenied {
		status[1] = 'Y'
	}
	if recallPrivilegesDenied {
		status[2] = 'Y'
	}
	if holdPrivilegesDenied {
		status[3] = 'Y'
	}
	if cardReportedLost {
		status[4] = 'Y'
	}
	if tooManyItemsCharged {
		status[5] = 'Y'
	}
	if tooManyItemsOverdue {
		status[6] = 'Y'
	}
	if tooManyRenewals {
		status[7] = 'Y'
	}
	if tooManyClaimsReturned {
		status[8] = 'Y'
	}
	if tooManyItemsLost {
		status[9] = 'Y'
	}
	if excessiveOutstandingFines {
		status[10] = 'Y'
	}
	if excessiveOutstandingFees {
		status[11] = 'Y'
	}
	if recallOverdue {
		status[12] = 'Y'
	}
	if tooManyItemsBilled {
		status[13] = 'Y'
	}

	return string(status)
}
