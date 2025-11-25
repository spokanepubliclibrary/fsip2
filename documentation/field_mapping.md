# SIP2 Field Mapping Documentation

This document provides a comprehensive reference for all supported SIP2 messages, their field mappings to FOLIO data, configuration options, and implementation details.

## Table of Contents

- [Message 09/10 - Checkin](#message-0910---checkin)
- [Message 11/12 - Checkout](#message-1112---checkout)
- [Message 17/18 - Item Information](#message-1718---item-information)
- [Message 19 - Item Status Update](#message-19---item-status-update)
- [Message 23/24 - Patron Status](#message-2324---patron-status)
- [Message 29/30 - Renew](#message-2930---renew)
- [Message 35/36 - End Patron Session](#message-3536---end-patron-session)
- [Message 37/38 - Fee Paid](#message-3738---fee-paid)
- [Message 63/64 - Patron Information](#message-6364---patron-information)
- [Message 65/66 - Renew All](#message-6566---renew-all)
- [Message 93/94 - Login](#message-9394---login)
- [Message 97/96 - Request Resend](#message-9796---request-resend)
- [Message 99/98 - SC/ACS Status](#message-9998---scacs-status)

---

## Message 09/10 - Checkin

**Request: Checkin (09)**
**Response: Checkin Response (10)**

### Request Fields (09)

| Field | Code | Required | Description | Source |
|-------|------|----------|-------------|--------|
| No Block | Fixed | Yes | Checkout blocked flag (Y/N) | Request message |
| Transaction Date | Fixed | Yes | Date/time of transaction | Request message |
| Return Date | Fixed | Yes | Date/time of return | Request message |
| Current Location | AP | Yes | Service point ID for checkin | Request message |
| Institution ID | AO | Yes | Institution identifier | Request message |
| Item Identifier | AB | Yes | Item barcode | Request message |
| Terminal Password | AC | No | Terminal password | Request message |

### Response Fields (10)

| Field | Code | Required | Configurable | Description | FOLIO Source |
|-------|------|----------|--------------|-------------|--------------|
| OK | Fixed | Yes | No | Checkin success (1) or failure (0) | Checkin API response |
| Resensitize | Fixed | Yes | No | Resensitize flag (Y/N/U) | Hardcoded: Y |
| Magnetic Media | Fixed | Yes | No | Magnetic media flag (Y/N/U) | Hardcoded: U |
| Alert | Fixed | Yes | No | Alert flag (Y/N) | Calculated from alert type |
| Transaction Date | Fixed | Yes | No | Date/time of transaction | Current timestamp |
| Institution ID | AO | Yes | No | Institution identifier | Request echo |
| Item Identifier | AB | Yes | No | Item barcode | Request echo |
| Permanent Location | AQ | Yes | No | Permanent location name | Item → Location → Name |
| Current Location | AP | Yes | No | Current location (service point) | Request echo |
| Title Identifier | AJ | Yes | No | Instance title (truncated to 60 chars) | Item → Holdings → Instance → Title |
| Media Type | CK | Yes | No | SIP2 media type code | Item → MaterialType → Mapped |
| Material Type | CH | Yes | No | FOLIO material type name | Item → MaterialType → Name |
| Call Number | CS | Yes | No | Effective call number | Item → EffectiveCallNumberComponents |
| Alert Type | CV | Yes | No | Alert type code (see below) | Calculated from item status and requests |
| Destination Location | CT | Yes | No | Routing/pickup service point | Request → PickupServicePoint or InTransitDestination |
| Sort Bin | CL | No | No | Sort bin identifier | Not implemented |
| Patron Identifier | AA | No | No | Patron barcode (if on hold) | Request → Requester → Barcode |
| Item Properties | BV | No | No | Item properties | Not implemented |
| Checkin Notes | AG | No | No | Item checkin notes (repeatable) | Item → CirculationNotes (Check in notes) |
| Hold Shelf Expiration | CM | No | Yes | Hold shelf expiration date | Request → HoldShelfExpirationDate |
| Requestor Name | DA | No | Yes | Patron name who placed hold | Request → Requester → LastName, FirstName |
| Screen Message | AF | No | No | Messages for display | "Checkin successful" or error messages |
| Print Line | AG | No | No | Print messages | Not implemented |

### Alert Type Calculation (CV Field)

| Code | Description | Logic |
|------|-------------|-------|
| 01 | Hold exists, not in transit | Item has open hold/recall but Status ≠ "In transit" |
| 02 | In transit with hold/recall | Status = "In transit" AND has open hold/recall |
| 04 | In transit only | Status = "In transit" with no holds/recalls |
| (blank) | No alert | No transit, no holds |

### Configuration Options

```yaml
supportedMessages:
  - code: "09"
    enabled: true
    fields:
      - code: "CM"  # Hold Shelf Expiration Date
        enabled: true
      - code: "DA"  # Requestor Name
        enabled: true
```

### Data Flow

1. Validate required fields (AO, AB, AP)
2. Call FOLIO Checkin API (`/circulation/check-in-by-barcode`)
3. Fetch item details by barcode (parallel goroutines):
   - Item → Location (for AQ field)
   - Item → MaterialType (for CH/CK fields)
   - Item → Holdings → Instance (for AJ field)
   - Item → Requests (for CV, CT, CM, DA fields)
4. Calculate alert type based on status and requests
5. Determine routing location from pickup service point or in-transit destination
6. Build response with all fields

### Special Handling

- **Claimed Returned Items**: Configurable resolution (`claimedReturnedResolution`):
  - `patron`: Resolve as "Returned by patron"
  - `library`: Resolve as "Found by library"
  - `none`: Block checkin with error message
- **Service Point Requirement**: AP field (Current Location) is required for FOLIO checkin
- **Parallel Data Fetching**: Uses goroutines to fetch independent data in parallel for performance

---

## Message 11/12 - Checkout

**Request: Checkout (11)**
**Response: Checkout Response (12)**

### Request Fields (11)

| Field | Code | Required | Description | Source |
|-------|------|----------|-------------|--------|
| SC Renewal Policy | Fixed | Yes | Renewal policy flag (Y/N) | Request message |
| No Block | Fixed | Yes | Checkout blocked flag (Y/N) | Request message |
| Transaction Date | Fixed | Yes | Date/time of transaction | Request message |
| NB Due Date | Fixed | No | No block due date | Request message |
| Institution ID | AO | Yes | Institution identifier | Request message |
| Patron Identifier | AA | Yes | Patron barcode | Request message |
| Item Identifier | AB | Yes | Item barcode | Request message |
| Terminal Password | AC | No | Terminal password | Request message |

### Response Fields (12)

| Field | Code | Required | Configurable | Description | FOLIO Source |
|-------|------|----------|--------------|-------------|--------------|
| OK | Fixed | Yes | No | Checkout success (1) or failure (0) | Checkout API response |
| Renewal OK | Fixed | Yes | No | Renewal allowed (Y/N) | Hardcoded: N |
| Magnetic Media | Fixed | Yes | No | Magnetic media flag (Y/N/U) | Hardcoded: U |
| Desensitize | Fixed | Yes | No | Desensitize flag (Y/N/U) | Hardcoded: U |
| Transaction Date | Fixed | Yes | No | Date/time of transaction | Current timestamp |
| Institution ID | AO | Yes | No | Institution identifier | Request echo |
| Patron Identifier | AA | Yes | No | Patron barcode | Request echo |
| Item Identifier | AB | Yes | No | Item barcode | Request echo |
| Title Identifier | AJ | Yes | No | Instance title | Loan → Item → Holdings → Instance → Title |
| Due Date | AH | No | No | Due date/time | Loan → DueDate |
| Fee Type | BT | No | No | Fee type code | Hardcoded: 01 (Other) |
| Security Inhibit | CI | No | No | Security inhibit flag (Y/N) | Not implemented |
| Currency Type | BH | No | No | Currency code | Configuration: currency |
| Fee Amount | BV | No | No | Fee amount | Not implemented |
| Media Type | CK | No | No | SIP2 media type code | Item → MaterialType → Mapped |
| Item Properties | BV | No | No | Item properties | Not implemented |
| Transaction ID | BK | No | No | Transaction identifier | Loan → ID |
| Screen Message | AF | No | No | Messages for display | Success/error messages |
| Print Line | AG | No | No | Print messages | Not implemented |

### Configuration Options

```yaml
supportedMessages:
  - code: "11"
    enabled: true
currency: "USD"
```

---

## Message 17/18 - Item Information

**Request: Item Information (17)**
**Response: Item Information Response (18)**

### Overview

Item Information (17/18) provides detailed information about library items. This implementation supports two distinct lookup modes:

1. **Item-Level Lookup**: Using item barcode (standard SIP2 usage)
2. **Instance-Level Lookup**: Using FOLIO instance UUID (bibliographic record only)

The system automatically detects the lookup mode based on the AB field format.

### Request Fields (17)

| Field | Code | Required | Description | Source |
|-------|------|----------|-------------|--------|
| Transaction Date | Fixed | Yes | Date/time of transaction (18 chars) | Request message |
| Institution ID | AO | Yes | Institution identifier | Request message |
| Item Identifier | AB | Yes | Item barcode OR instance UUID | Request message |

#### AB Field Format Detection

The handler uses regex pattern matching to detect the identifier type:

- **UUID Format**: `^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`
  - Example: `5bf370e0-8cca-4d9c-82e4-5a6a3ec0b6e0`
  - Triggers instance-level lookup
- **Barcode Format**: Anything not matching UUID pattern
  - Example: `31221234567890`
  - Triggers item-level lookup

**Implementation**: [item_information.go:100-123](../internal/handlers/item_information.go#L100-L123)

### Response Fields (18) - Item-Level Lookup

When AB contains an item barcode, the response includes both item-level and bibliographic fields:

| Field | Code | Required | Configurable | Max Length | Description | FOLIO Source | Configuration Check |
|-------|------|----------|--------------|------------|-------------|--------------|---------------------|
| **Fixed Fields** |
| Circulation Status | Fixed | Yes | Via Mapping | 2 chars | SIP2 circulation status code | Item → Status → Mapped via config | `circulationStatusMapping` |
| Security Marker | Fixed | Yes | No | 2 chars | Security marker code | Hardcoded: 00 | N/A |
| Fee Type | Fixed | Yes | No | 2 chars | Fee type code | Hardcoded: 01 (Other) | N/A |
| Transaction Date | Fixed | Yes | No | 18 chars | Date/time of transaction | Current timestamp | N/A |
| **Required Variable Fields** |
| Institution ID | AO | Yes | No | Variable | Institution identifier | Request echo | N/A |
| Item Identifier | AB | Yes | No | Variable | Item barcode (echoed) | Request echo | N/A |
| **Item-Level Fields (Configurable)** |
| Title Identifier | AJ | No | Yes | 60 chars | Instance title (truncated) | Item → Holdings → Instance → Title | `IsFieldEnabled("17", "AJ")` |
| Permanent Location | AQ | No | Yes | Variable | Permanent location name | Item → Location → Name | `IsFieldEnabled("17", "AQ")` |
| Current Location | AP | No | Yes | Variable | Current location name | Item → Location → Name | `IsFieldEnabled("17", "AP")` |
| Due Date | AH | No | Yes | 18 chars | Due date (if checked out) | Loan → DueDate | `IsFieldEnabled("17", "AH")` |
| Media Type | CK | No | Yes | 3 chars | SIP2 media type code | Item → MaterialType → Mapped | `IsFieldEnabled("17", "CK")` |
| Material Type | CH | No | Yes | Variable | FOLIO material type name | Item → MaterialType → Name | `IsFieldEnabled("17", "CH")` |
| Call Number | CS | No | Yes | Variable | Effective call number | Item → EffectiveCallNumberComponents | `IsFieldEnabled("17", "CS")` |
| Routing Location | CT | No | Yes | Variable | In-transit destination | Item → InTransitDestinationServicePoint → Name | `IsFieldEnabled("17", "CT")` |
| Hold Queue Length | CF | No | Yes | 4 chars | Number of open requests | Count of Requests with Status = "Open*" | `IsFieldEnabled("17", "CF")` |
| **Bibliographic Fields (Configurable)** |
| Primary Contributor | EA | No | Yes | Variable | Primary author/contributor | Instance → Contributors (Primary=true) → Name | `IsFieldEnabled("17", "EA")` |
| Work Description | DE | No | Yes | 255 chars | Summary/description | Instance → Notes (Type=Summary) → Note | `IsFieldEnabled("17", "DE")` |
| ISBN Identifier | IN | No | Yes | Variable | ISBN (repeatable) | Instance → Identifiers (Type=ISBN) → Value | `IsFieldEnabled("17", "IN")` |
| Other Standard ID | NB | No | Yes | Variable | UPC/other IDs (repeatable) | Instance → Identifiers (Type=UPC) → Value | `IsFieldEnabled("17", "NB")` |
| **Hold-Related Fields (Configurable)** |
| Hold Shelf Expiration | CM | No | Yes | 18 chars | Hold expiration date | Request (Awaiting Pickup) → HoldShelfExpirationDate | `IsFieldEnabled("17", "CM")` |
| Requestor Barcode | CY | No | Yes | Variable | Patron barcode of requestor | Request (Awaiting Pickup) → Requester → Barcode | `IsFieldEnabled("17", "CY")` |
| Requestor Name | DA | No | Yes | Variable | Patron name of requestor | Request (Awaiting Pickup) → Requester → LastName, FirstName | `IsFieldEnabled("17", "DA")` |
| **Messages** |
| Screen Message | AF | No | No | Variable | Status messages | "Item found" or "Item not found" | N/A |
| Print Line | AG | No | No | Variable | Print messages | Not implemented | N/A |

### Response Fields (18) - Instance-Level Lookup

When AB contains an instance UUID, the response includes only bibliographic fields:

| Field | Code | Required | Configurable | Description | FOLIO Source |
|-------|------|----------|--------------|-------------|--------------|
| **Fixed Fields** |
| Circulation Status | Fixed | Yes | No | Always "01" (Other) | Hardcoded for instances |
| Security Marker | Fixed | Yes | No | Always "00" | Hardcoded |
| Fee Type | Fixed | Yes | No | Always "01" (Other) | Hardcoded |
| Transaction Date | Fixed | Yes | No | Current timestamp | System |
| **Required Variable Fields** |
| Institution ID | AO | Yes | No | Institution identifier | Request echo |
| Item Identifier | AB | Yes | No | Instance UUID (echoed) | Request echo |
| **Bibliographic Fields (Configurable)** |
| Title Identifier | AJ | No | Yes | Instance title (60 chars) | Instance → Title |
| Primary Contributor | EA | No | Yes | Primary contributor | Instance → Contributors (Primary=true) → Name |
| Work Description | DE | No | Yes | Summary (255 chars) | Instance → Notes (Type=Summary) → Note |
| ISBN Identifier | IN | No | Yes | ISBN (repeatable) | Instance → Identifiers (Type=ISBN) → Value |
| Other Standard ID | NB | No | Yes | UPC/other (repeatable) | Instance → Identifiers (Type=UPC) → Value |
| **Item-Level Fields** |
| All item fields | Various | No | No | Always blank for instances | N/A |
| **Messages** |
| Screen Message | AF | No | No | Status message | "Instance-level information only (no item data)" or "Instance not found" |
| Hold Queue Length | CF | No | No | Always "0000" | No request lookup for instances |

### Circulation Status Mapping

The circulation status (2-char fixed field) is mapped from FOLIO item status using configuration:

**Default Mappings** (if not configured):

| FOLIO Status | SIP2 Code | Description |
|--------------|-----------|-------------|
| Available | 03 | Available |
| Checked out | 04 | Charged |
| In process | 06 | In process |
| Awaiting pickup | 08 | Waiting on hold shelf |
| In transit | 10 | In transit between library locations |
| Claimed returned | 11 | Claimed returned by borrower |
| Lost and paid | 12 | Lost |
| Aged to lost | 12 | Lost |
| Declared lost | 12 | Lost |
| Missing | 13 | Missing |
| Withdrawn | 01 | Other |
| On order | 02 | On order |
| Paged | 08 | Waiting on hold shelf |
| (default) | 01 | Other |

**Configuration Override**:

```yaml
circulationStatusMapping:
  "Available": "03"
  "Checked out": "04"
  "In transit": "10"
  "Awaiting pickup": "08"
  "default": "01"  # Fallback for unmapped statuses
```

**Implementation**: [config.go:278-323](../internal/config/config.go#L278-L323)

### Configuration Options

Field-level configuration controls which fields are included in the response:

```yaml
supportedMessages:
  - code: "17"
    enabled: true
    fields:
      # Item metadata fields
      - code: "AJ"  # Title
        enabled: true
      - code: "AQ"  # Permanent location
        enabled: true
      - code: "AP"  # Current location
        enabled: true
      - code: "AH"  # Due date
        enabled: true
      - code: "CK"  # Media type (SIP2 code)
        enabled: true
      - code: "CH"  # Material type (FOLIO name)
        enabled: true
      - code: "CS"  # Call number
        enabled: true
      - code: "CT"  # Routing location
        enabled: true
      - code: "CF"  # Hold queue length
        enabled: true

      # Bibliographic fields
      - code: "EA"  # Primary contributor
        enabled: true
      - code: "DE"  # Work description/summary
        enabled: true
      - code: "IN"  # ISBN
        enabled: true
      - code: "NB"  # Other standard identifier (UPC)
        enabled: true

      # Hold-related fields
      - code: "CM"  # Hold shelf expiration
        enabled: true
      - code: "CY"  # Requestor barcode
        enabled: true
      - code: "DA"  # Requestor name
        enabled: true

# Circulation status mapping
circulationStatusMapping:
  "Available": "03"
  "Checked out": "04"
  "Awaiting pickup": "08"
  "In transit": "10"
  "default": "01"
```

### Data Flow - Item-Level Lookup

**Handler**: [item_information.go:69-266](../internal/handlers/item_information.go#L69-L266)

1. **Validate Request** (Lines 73-79)
   - Check required fields: AO (Institution ID), AB (Item Identifier)

2. **Authenticate** (Lines 91-95)
   - Get FOLIO authentication token from session

3. **Detect Identifier Type** (Lines 100-123)
   - Check if AB matches UUID pattern
   - Route to instance or item lookup path

4. **Item Barcode Path** (Lines 125-137)
   - Fetch item by barcode: `GET /item-storage/items?query=barcode=={barcode}`

5. **Fetch Title** (Lines 144-165) - *Conditional on AJ field enabled*
   - Follow chain: Item → HoldingsRecordID → Holdings → InstanceID → Instance → Title
   - Truncate to 60 characters

6. **Fetch Due Date** (Lines 167-180) - *Conditional on AH field enabled AND item checked out*
   - Get loans by item ID: `GET /circulation/loans?query=itemId=={itemId}`
   - Extract DueDate from first open loan

7. **Fetch Requests** (Lines 182-197) - *Conditional on CF, CM, CY, or DA fields enabled*
   - Get requests by item: `GET /circulation/requests?query=itemId=={itemId}`
   - Used for hold queue length and hold-related fields

8. **Extract Hold Information** (Lines 199-230) - *From requests collection*
   - Find first request with status "Open - Awaiting pickup"
   - Extract:
     - CM: HoldShelfExpirationDate
     - CY: Requester → Barcode
     - DA: Requester → LastName, FirstName (formatted as "LastName, FirstName")

9. **Determine Routing Location** (Lines 232-261) - *Conditional on CT field enabled*
   - Check if item has `InTransitDestinationServicePointID`
   - Fetch service point: `GET /service-points/{id}`
   - Use service point name as routing location

10. **Build Response** (Lines 274-328)
    - Prepare all field data via `prepareItemResponseData()`
    - Use `ResponseBuilder.BuildItemInformationResponse()`
    - Apply field configuration to include/exclude fields

### Data Flow - Instance-Level Lookup

**Handler**: [item_information.go:100-122](../internal/handlers/item_information.go#L100-L122)

1. **Detect UUID Format** (Line 101)
   - Regex match: `^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`

2. **Fetch Instance** (Lines 107-114)
   - Direct lookup: `GET /instance-storage/instances/{uuid}`
   - No item-level data fetched

3. **Build Response** (Lines 427-477)
   - Include only bibliographic fields
   - No item-level fields (locations, circulation, holds)
   - Circulation status always "01" (Other)
   - Hold queue length always "0000"
   - Screen message: "Instance-level information only (no item data)"

### Identifier Type UUIDs

The following FOLIO identifier type UUIDs are used:

| Type | UUID | Usage |
|------|------|-------|
| ISBN | `8261054f-be78-422d-bd51-4ed9f33c3422` | IN field |
| UPC (Other standard identifier) | `2e8b3b6c-0e7d-4e48-bca2-b0b23b376af5` | NB field |
| UPC (Other standard identifier) | `1795ea23-6856-48a5-a772-f356e16a8a6c` | NB field |
| Summary Note | `10e2e11b-450f-45c8-b09b-0f819999966e` | DE field |

**Implementation**: [item_information.go:531-569](../internal/handlers/item_information.go#L531-L569)

### String Truncation

Certain fields are truncated to meet SIP2 specifications:

- **AJ (Title)**: 60 characters
- **DE (Work Description)**: 255 characters

Truncation preserves UTF-8 multibyte characters correctly.

**Implementation**: [item_information.go:421-432](../internal/handlers/item_information.go#L421-L432)

### Example Responses

#### Item-Level Response (All Fields Enabled)

```
18030120250125    143000AO|AB31221234567890|AJHarry Potter and the Sorcerer's Stone|AQMain Library - Fiction|APMain Library - Fiction|AH20250215    235900|CK001|CHBook|PZFIC ROW|CTNorth Branch|CF0001|EARowling, J.K.|DEA young wizard begins his magical education.|IN9780439708180|CM20250130    235900|CY21234567890|DASmith, John|AFItem found|
```

#### Instance-Level Response (UUID Lookup)

```
18010120250125    143000AO|AB5bf370e0-8cca-4d9c-82e4-5a6a3ec0b6e0|AJHarry Potter and the Sorcerer's Stone|EARowling, J.K.|DEA young wizard begins his magical education.|IN9780439708180|CF0000|AFInstance-level information only (no item data)|
```

#### Item Not Found Response

```
18010120250125    143000AO|AB99999999999999|ABItem not found|
```

### Error Handling

| Condition | Response | Circulation Status | Screen Message |
|-----------|----------|-------------------|----------------|
| Item barcode not found | Returns response | 01 (Other) | "Item not found" |
| Instance UUID not found | Returns response | 01 (Other) | "Instance not found" |
| Authentication failure | Returns response | 01 (Other) | "Item not found" |
| Holdings fetch fails | Omits title | Status from item | "Item found" |
| Instance fetch fails | Omits bibliographic data | Status from item | "Item found" |
| Requests fetch fails | Omits hold data | Status from item | "Item found" |

**Note**: The service never returns error messages via SIP2 error response codes. All errors result in a valid 18 response with appropriate fields omitted.

### Performance Optimizations

- **Conditional API Calls**: Only fetches data for enabled fields
  - Title fetched only if `IsFieldEnabled("17", "AJ")` = true
  - Due date fetched only if `IsFieldEnabled("17", "AH")` = true AND item is checked out
  - Requests fetched only if any of CF, CM, CY, DA fields are enabled
- **Single Request Check**: Requests are fetched once and used for multiple fields (CF, CM, CY, DA, CT)
- **Early Return**: Instance-level lookup skips all item-level API calls

---

## Message 19 - Item Status Update

**Request: Item Status Update (19)**
**Response: Item Status Update Response (20)**

### Request Fields (19)

| Field | Code | Required | Description | Source |
|-------|------|----------|-------------|--------|
| Transaction Date | Fixed | Yes | Date/time of transaction | Request message |
| Institution ID | AO | Yes | Institution identifier | Request message |
| Item Identifier | AB | Yes | Item barcode | Request message |
| Terminal Password | AC | No | Terminal password | Request message |
| Item Properties | CH | No | Item properties | Request message |

### Response Fields (20)

| Field | Code | Required | Description | FOLIO Source |
|-------|------|----------|-------------|--------------|
| Item Properties OK | Fixed | Yes | Properties updated (Y/N) | Not implemented - always N |
| Transaction Date | Fixed | Yes | Date/time of transaction | Current timestamp |
| Institution ID | AO | Yes | Institution identifier | Request echo |
| Item Identifier | AB | Yes | Item barcode | Request echo |
| Title Identifier | AJ | No | Instance title | Not implemented |
| Item Properties | CH | No | Item properties | Not implemented |
| Screen Message | AF | No | Messages for display | "Item status update not supported" |

**Note**: Item Status Update is not currently implemented and always returns failure.

---

## Message 23/24 - Patron Status

**Request: Patron Status (23)**
**Response: Patron Status Response (24)**

### Request Fields (23)

| Field | Code | Required | Description | Source |
|-------|------|----------|-------------|--------|
| Language | Fixed | Yes | Language code (3 chars) | Request message |
| Transaction Date | Fixed | Yes | Date/time of transaction | Request message |
| Institution ID | AO | Yes | Institution identifier | Request message |
| Patron Identifier | AA | Yes | Patron barcode | Request message |
| Terminal Password | AC | No | Terminal password | Request message |
| Patron Password | AD | No | Patron password/PIN | Request message |

### Response Fields (24)

| Field | Code | Required | Description | FOLIO Source |
|-------|------|----------|-------------|--------------|
| Patron Status | Fixed | Yes | 14-character status flags | Calculated from user record |
| Language | Fixed | Yes | Language code (3 chars) | Request echo or "000" |
| Transaction Date | Fixed | Yes | Date/time of transaction | Current timestamp |
| Institution ID | AO | Yes | Institution identifier | Request echo |
| Patron Identifier | AA | Yes | Patron barcode | Request echo |
| Personal Name | AE | Yes | Patron full name | User → Personal → LastName, FirstName |
| Valid Patron | BL | No | Patron valid (Y/N) | User → Active |
| Valid Patron Password | CQ | No | Password valid (Y/N) | PIN verification result |
| Currency Type | BH | No | Currency code | Configuration: currency |
| Fee Amount | BV | No | Total outstanding fees | Sum of open accounts |
| Screen Message | AF | No | Messages for display | Status messages |
| Print Line | AG | No | Print messages | Not implemented |

### Patron Status Flags (14 characters)

Each position is 'Y' (true) or ' ' (space, false):

| Position | Flag | Description | FOLIO Source |
|----------|------|-------------|--------------|
| 1 | Charge Denied | Cannot checkout | User → Blocks or manual blocks |
| 2 | Renewal Denied | Cannot renew | User → Blocks or manual blocks |
| 3 | Recall Denied | Cannot recall | Not implemented |
| 4 | Hold Denied | Cannot place holds | User → Blocks or manual blocks |
| 5 | Card Lost | Card reported lost | Not implemented |
| 6 | Too Many Charged | Too many items charged | Not implemented |
| 7 | Too Many Overdue | Too many overdue items | Calculated from loans |
| 8 | Too Many Renewals | Renewal limit reached | Not implemented |
| 9 | Too Many Claims | Too many claims returned | Not implemented |
| 10 | Too Many Lost | Too many lost items | Not implemented |
| 11 | Excessive Fines | Outstanding fines too high | Calculated from accounts |
| 12 | Excessive Fees | Outstanding fees too high | Calculated from accounts |
| 13 | Recall Overdue | Recall is overdue | Not implemented |
| 14 | Too Many Billed | Too many billed items | Not implemented |

---

## Message 29/30 - Renew

**Request: Renew (29)**
**Response: Renew Response (30)**

### Request Fields (29)

| Field | Code | Required | Description | Source |
|-------|------|----------|-------------|--------|
| Third Party Allowed | Fixed | Yes | Third party allowed (Y/N) | Request message |
| No Block | Fixed | Yes | Renew blocked flag (Y/N) | Request message |
| Transaction Date | Fixed | Yes | Date/time of transaction | Request message |
| NB Due Date | Fixed | No | No block due date | Request message |
| Institution ID | AO | Yes | Institution identifier | Request message |
| Patron Identifier | AA | Yes | Patron barcode | Request message |
| Item Identifier | AB | Yes | Item barcode | Request message |
| Terminal Password | AC | No | Terminal password | Request message |
| Patron Password | AD | No | Patron password/PIN | Request message |
| Fee Acknowledged | BO | No | Fee acknowledged (Y/N) | Request message |

### Response Fields (30)

| Field | Code | Required | Description | FOLIO Source |
|-------|------|----------|-------------|--------------|
| OK | Fixed | Yes | Renew success (1) or failure (0) | Renewal API response |
| Renewal OK | Fixed | Yes | Renewal allowed (Y/N) | Renewal API response |
| Magnetic Media | Fixed | Yes | Magnetic media (Y/N/U) | Hardcoded: U |
| Desensitize | Fixed | Yes | Desensitize (Y/N/U) | Hardcoded: U |
| Transaction Date | Fixed | Yes | Date/time of transaction | Current timestamp |
| Institution ID | AO | Yes | Institution identifier | Request echo |
| Patron Identifier | AA | Yes | Patron barcode | Request echo |
| Item Identifier | AB | Yes | Item barcode | Request echo |
| Title Identifier | AJ | No | Instance UUID | Loan → Item → Holdings → InstanceID |
| Due Date | AH | No | New due date | Loan → DueDate (updated) |
| Screen Message | AF | No | Messages for display | Success/error messages |
| Print Line | AG | No | Print messages | Not implemented |

---

## Message 35/36 - End Patron Session

**Request: End Patron Session (35)**
**Response: End Session Response (36)**

### Request Fields (35)

| Field | Code | Required | Description | Source |
|-------|------|----------|-------------|--------|
| Transaction Date | Fixed | Yes | Date/time of transaction | Request message |
| Institution ID | AO | Yes | Institution identifier | Request message |
| Patron Identifier | AA | Yes | Patron barcode | Request message |
| Terminal Password | AC | No | Terminal password | Request message |
| Patron Password | AD | No | Patron password/PIN | Request message |

### Response Fields (36)

| Field | Code | Required | Description | FOLIO Source |
|-------|------|----------|-------------|--------------|
| End Session | Fixed | Yes | End session (Y/N) | Hardcoded: Y |
| Transaction Date | Fixed | Yes | Date/time of transaction | Current timestamp |
| Institution ID | AO | Yes | Institution identifier | Request echo |
| Patron Identifier | AA | Yes | Patron barcode | Request echo |
| Screen Message | AF | No | Messages for display | "Session ended" |
| Print Line | AG | No | Print messages | Not implemented |

**Note**: This message does not perform any server-side session cleanup. It simply acknowledges the client's request to end the session.

---

## Message 37/38 - Fee Paid

**Request: Fee Paid (37)**
**Response: Fee Paid Response (38)**

### Request Fields (37)

| Field | Code | Required | Description | Source |
|-------|------|----------|-------------|--------|
| Transaction Date | Fixed | Yes | Date/time of transaction | Request message |
| Fee Type | Fixed | Yes | Fee type code (2 chars) | Request message |
| Payment Type | Fixed | Yes | Payment type code (2 chars) | Request message |
| Currency Type | Fixed | Yes | Currency code (3 chars) | Request message |
| Fee Amount | BV | Yes | Payment amount | Request message |
| Institution ID | AO | Yes | Institution identifier | Request message |
| Patron Identifier | AA | Yes | Patron barcode | Request message |
| Terminal Password | AC | No | Terminal password | Request message |
| Patron Password | AD | No | Patron password/PIN | Request message |
| Fee Identifier | CG | No | Account ID | Request message |
| Transaction ID | BK | No | Transaction identifier | Request message |

### Response Fields (38)

| Field | Code | Required | Description | FOLIO Source |
|-------|------|----------|-------------|--------------|
| Payment Accepted | Fixed | Yes | Payment accepted (Y/N) | Payment API response |
| Transaction Date | Fixed | Yes | Date/time of transaction | Current timestamp |
| Institution ID | AO | Yes | Institution identifier | Request echo |
| Patron Identifier | AA | Yes | Patron barcode | Request echo |
| Transaction ID | BK | No | FOLIO payment ID | Payment → ID |
| Screen Message | AF | No | Messages for display | Success/error messages |
| Print Line | AG | No | Print messages | Not implemented |

### Additional details about fee/fine messages
See [documentation/fee_fines.md](documentation/fee_fines.md) for additional reference. 

### Configuration Options

```yaml
acceptBulkPayment: false    # Allow payment without account ID (distributes across all accounts)
paymentMethod: "Credit card"  # Default payment method
notifyPatron: false          # Send email/SMS notification
```

### Payment Logic

1. **With Fee Identifier (CG)**: Pay specific account
   - Use CG field as account ID
   - Call `/accounts/{accountId}/pay` with amount

2. **Without Fee Identifier**: Bulk payment (if enabled)
   - Requires `acceptBulkPayment: true` in configuration
   - Distributes payment across all open accounts
   - Calls `/accounts/{userId}/pay` with amount

---

## Message 63/64 - Patron Information

**Request: Patron Information (63)**
**Response: Patron Information Response (64)**

### Request Fields (63)

| Field | Code | Required | Description | Source |
|-------|------|----------|-------------|--------|
| Language | Fixed | Yes | Language code (3 chars) | Request message |
| Transaction Date | Fixed | Yes | Date/time of transaction | Request message |
| Summary | Fixed | Yes | Summary flags (10 chars) | Request message |
| Institution ID | AO | Yes | Institution identifier | Request message |
| Patron Identifier | AA | Yes | Patron barcode | Request message |
| Terminal Password | AC | No | Terminal password | Request message |
| Patron Password | AD | No | Patron password/PIN | Request message |
| Start Item | BP | No | Start item (pagination) | Request message |
| End Item | BQ | No | End item (pagination) | Request message |

### Summary Field (10 characters)

Each position controls inclusion of detail fields (Y/N):

| Position | Flag | Variable Fields Affected | Description |
|----------|------|-------------------------|-------------|
| 1 | Hold items | AS | Include hold item barcodes |
| 2 | Overdue items | AT | Include overdue item barcodes |
| 3 | Charged items | AU | Include charged item barcodes |
| 4 | Fine items | AV | Include fine item details |
| 5 | Recall items | BU | Include recall item barcodes |
| 6 | Unavailable holds | CD | Include unavailable hold barcodes |
| 7-10 | (Reserved) | N/A | Not used |

**Important**: Summary flags only control detail fields. Counts in fixed fields are ALWAYS accurate regardless of summary flags.

### Response Fields (64)

| Field | Code | Required | Configurable | Description | FOLIO Source |
|-------|------|----------|--------------|-------------|--------------|
| **Fixed Fields** |
| Patron Status | Fixed | Yes | No | 14-character status flags | Calculated from user/blocks |
| Language | Fixed | Yes | No | Language code (3 chars) | Request echo or "000" |
| Transaction Date | Fixed | Yes | No | Date/time of transaction | Current timestamp |
| Hold Items Count | Fixed | Yes | No | Number of available holds | Count of requests (status=Awaiting pickup) |
| Overdue Items Count | Fixed | Yes | No | Number of overdue items | Count of overdue loans |
| Charged Items Count | Fixed | Yes | No | Number of charged items | Count of open loans |
| Fine Items Count | Fixed | Yes | No | Number of items with fines | Count of open accounts |
| Recall Items Count | Fixed | Yes | No | Number of recall items | Always 0000 (not implemented) |
| Unavailable Holds Count | Fixed | Yes | No | Number of unfilled holds | Count of requests (status ≠ Awaiting pickup) |
| **Required Variable Fields** |
| Institution ID | AO | Yes | No | Institution identifier | Request echo |
| Patron Identifier | AA | Yes | No | Patron barcode | Request echo |
| Personal Name | AE | Yes | No | Patron full name | User → Personal → LastName, FirstName |
| **Optional Fields** |
| Hold Items Limit | BZ | No | No | Hold limit | Not implemented (0000) |
| Overdue Items Limit | CA | No | No | Overdue limit | Not implemented (0000) |
| Charged Items Limit | CB | No | No | Charged limit | Not implemented (0000) |
| Valid Patron | BL | No | No | Patron valid (Y/N) | User → Active |
| Valid Patron Password | CQ | No | No | Password valid (Y/N) | PIN verification result |
| Currency Type | BH | No | No | Currency code | Configuration: currency |
| Fee Amount | BV | No | No | Total outstanding fees | Sum of accounts → Amount - Paid |
| Fee Limit | CC | No | No | Fee limit | Not implemented |
| **Item Detail Fields (Controlled by Summary)** |
| Hold Items | AS | No | Summary[0] | Hold item barcodes (repeatable) | Request → Item → Barcode |
| Overdue Items | AT | No | Summary[1] | Overdue item barcodes (repeatable) | Loan → Item → Barcode (where overdue) |
| Charged Items | AU | No | Summary[2] | Charged item barcodes (repeatable) | Loan → Item → Barcode |
| Fine Items | AV | No | Summary[3] | Fine item barcodes (repeatable) | Account → Item → Barcode |
| Recall Items | BU | No | Summary[4] | Recall item barcodes (repeatable) | Not implemented |
| Unavailable Holds | CD | No | Summary[5] | Unavailable hold barcodes (repeatable) | Request → Item → Barcode (where not awaiting pickup) |
| **Patron Details** |
| Home Address | BD | No | No | Patron address | User → Personal → Addresses (primary) |
| Email Address | BE | No | No | Email address | User → Personal → Email |
| Home Phone | BF | No | No | Home phone number | User → Personal → Phone |
| **Messages** |
| Screen Message | AF | No | No | Messages for display | Status messages |
| Print Line | AG | No | No | Print messages | Not implemented |


### Additional details about fee/fine messages
See [documentation/fee_fines.md](documentation/fee_fines.md) for additional reference. 

### Custom Fields (SA-SZ)

Patron custom fields can be configured to map FOLIO user custom fields to SIP2 fields SA through SZ:

```yaml
patronCustomFields:
  enabled: true
  fields:
    - code: "SA"
      source: "department"
      type: "string"
      maxLength: 60
    - code: "SB"
      source: "studentId"
      type: "string"
      maxLength: 20
    - code: "SC"
      source: "tags"
      type: "array"
      arrayDelimiter: ","
      maxLength: 100
```

**Implementation**: Custom field mapping extracts values from `User.CustomFields` and includes them in the response if enabled.

---

## Message 65/66 - Renew All

**Request: Renew All (65)**
**Response: Renew All Response (66)**

### Request Fields (65)

| Field | Code | Required | Description | Source |
|-------|------|----------|-------------|--------|
| Transaction Date | Fixed | Yes | Date/time of transaction | Request message |
| Institution ID | AO | Yes | Institution identifier | Request message |
| Patron Identifier | AA | Yes | Patron barcode | Request message |
| Terminal Password | AC | No | Terminal password | Request message |
| Patron Password | AD | No | Patron password/PIN | Request message |
| Fee Acknowledged | BO | No | Fee acknowledged (Y/N) | Request message |

### Response Fields (66)

| Field | Code | Required | Description | FOLIO Source |
|-------|------|----------|-------------|--------------|
| OK | Fixed | Yes | Overall success (Y/N) | All renewals successful |
| Renewed Count | Fixed | Yes | Number of renewed items (4 digits) | Count of successful renewals |
| Unrenewed Count | Fixed | Yes | Number of unrenewed items (4 digits) | Count of failed renewals |
| Transaction Date | Fixed | Yes | Date/time of transaction | Current timestamp |
| Institution ID | AO | Yes | Institution identifier | Request echo |
| Patron Identifier | AA | Yes | Patron barcode | Request echo |
| Renewed Items | BM | No | Renewed item barcodes (repeatable) | Loan → Item → Barcode (successful) |
| Unrenewed Items | BN | No | Unrenewed item barcodes (repeatable) | Loan → Item → Barcode (failed) |
| Screen Message | AF | No | Messages for display | Status messages |
| Print Line | AG | No | Print messages | Not implemented |

### Configuration Options

```yaml
renewAllMaxItems: 50  # Maximum items to attempt renewal (default: 50)
```

### Renewal Logic

1. Fetch all open loans for patron
2. Limit to `renewAllMaxItems` loans
3. Attempt renewal for each loan via `/circulation/renew-by-id`
4. Track successful and failed renewals
5. Return counts and barcode lists

---

## Message 93/94 - Login

**Request: Login (93)**
**Response: Login Response (94)**

### Request Fields (93)

| Field | Code | Required | Description | Source |
|-------|------|----------|-------------|--------|
| UID Algorithm | Fixed | Yes | User ID algorithm (1 char) | Request message |
| PWD Algorithm | Fixed | Yes | Password algorithm (1 char) | Request message |
| Login User ID | CN | Yes | Login username | Request message |
| Login Password | CO | Yes | Login password | Request message |
| Location Code | CP | No | Location code | Request message |

### Response Fields (94)

| Field | Code | Required | Description | FOLIO Source |
|-------|------|----------|-------------|--------------|
| OK | Fixed | Yes | Login success (1) or failure (0) | Authentication result |

### Authentication Flow

1. Validate username and password format
2. Call FOLIO authentication: `POST /authn/login`
   - Body: `{"username": "...", "password": "..."}`
3. Store authentication token in session
4. Return success (1) or failure (0)

**Important**: The 93 message establishes system-level authentication. Subsequent patron operations use this token for FOLIO API calls.

---

## Message 97/96 - Request Resend

**Request: Request SC Resend (97)**
**Response: Request SC/ACS Resend (96)**

### Request Fields (97)

No variable fields. This is a simple 2-character message: `97`

### Response Fields (96)

No fields. The ACS responds by resending the last message sent to the SC.

**Implementation**: Not fully implemented. Currently returns success without resending.

---

## Message 99/98 - SC/ACS Status

**Request: SC Status (99)**
**Response: ACS Status (98)**

### Request Fields (99)

| Field | Code | Required | Description | Source |
|-------|------|----------|-------------|--------|
| Status Code | Fixed | Yes | SC status code (1 char) | Request message |
| Max Print Width | Fixed | Yes | Maximum print width (3 digits) | Request message |
| Protocol Version | Fixed | Yes | Protocol version (4 chars) | Request message |

### Response Fields (98)

| Field | Code | Required | Description | FOLIO Source |
|-------|------|----------|-------------|--------------|
| Online Status | Fixed | Yes | ACS online (Y/N) | Hardcoded: Y |
| Checkin OK | Fixed | Yes | Checkin allowed (Y/N) | Configuration or Y |
| Checkout OK | Fixed | Yes | Checkout allowed (Y/N) | Configuration or Y |
| ACS Renewal Policy | Fixed | Yes | ACS renewal policy (Y/N) | Hardcoded: N |
| Status Update OK | Fixed | Yes | Status update allowed (Y/N) | Configuration: statusUpdateOk |
| Offline OK | Fixed | Yes | Offline allowed (Y/N) | Configuration: offlineOk |
| Timeout Period | Fixed | Yes | Timeout period (3 digits) | Configuration: timeoutPeriod (default: 030) |
| Retries Allowed | Fixed | Yes | Retries allowed (3 digits) | Configuration: retriesAllowed (default: 003) |
| Date/Time Sync | Fixed | Yes | Current date/time | Current timestamp |
| Protocol Version | Fixed | Yes | Protocol version | "2.00" |
| Institution ID | AO | Yes | Institution identifier | Configuration or request |
| Library Name | AM | No | Library name | Configuration |
| Supported Messages | BX | Yes | 16-character message support string | Configuration: supportedMessages |
| Terminal Location | AN | No | Terminal location | Configuration |
| Screen Message | AF | No | Messages for display | Status messages |
| Print Line | AG | No | Print messages | Not implemented |

### Supported Messages Field (BX)

16-character string indicating support for each message type (Y/N):

| Position | Message | Code | Description |
|----------|---------|------|-------------|
| 1 | Patron Status Request | 23 | Patron status lookup |
| 2 | Checkout | 11 | Item checkout |
| 3 | Checkin | 09 | Item checkin |
| 4 | Block Patron | 01 | Block patron (not implemented) |
| 5 | SC/ACS Status | 99 | Status request |
| 6 | Request Resend | 97 | Resend last message |
| 7 | Login | 93 | System authentication |
| 8 | Patron Information | 63 | Detailed patron info |
| 9 | End Patron Session | 35 | End session |
| 10 | Fee Paid | 37 | Fee/fine payment |
| 11 | Item Information | 17 | Item lookup |
| 12 | Item Status Update | 19 | Update item status |
| 13 | Patron Enable | 25 | Enable patron (not implemented) |
| 14 | Hold | 15 | Place hold |
| 15 | Renew | 29 | Renew item |
| 16 | Renew All | 65 | Renew all items |

**Configuration**:

```yaml
supportedMessages:
  - code: "09"
    enabled: true
  - code: "11"
    enabled: true
  - code: "17"
    enabled: true
  # ... etc
```

The BX field is automatically built from this configuration using `TenantConfig.BuildSupportedMessages()`.

---

## Global Configuration Options

### Error Detection

```yaml
errorDetectionEnabled: true  # Enable AY/AZ checksum fields
```

When enabled, all responses include:
- `AY{seq}`: Sequence number (echoed from request)
- `AZ{checksum}`: 4-character checksum

### Delimiters

```yaml
fieldDelimiter: "|"          # Field delimiter
messageDelimiter: "\\r"      # Message delimiter (\r, \n, or \r\n)
```

### Character Encoding

```yaml
charset: "UTF-8"             # Character encoding (UTF-8, ASCII, ISO-8859-1)
```

### Timezone

```yaml
timezone: "America/Los_Angeles"  # Timezone for date/time formatting
```

All SIP2 date/time fields use format: `YYYYMMDD    HHMMSS` (18 characters, 4-space separator)

### Currency

```yaml
currency: "USD"              # Currency code (3 characters)
```

Used in BH fields across multiple messages.

### Patron Verification

```yaml
patronPasswordVerificationRequired: true  # Require patron password/PIN
usePinForPatronVerification: true        # Use PIN instead of password
```

Controls whether AD field (Patron Password) is verified for patron operations.

### Multi-Tenancy

```yaml
scTenants:
  - tenant: "tenant1"
    scSubnet: "192.168.1.0/24"
    port: 6443
    locationCodes:
      - "MAIN"
      - "BRANCH"
    usernamePrefixes:
      - "sip_"
```

Supports multiple sub-tenants with different configurations based on subnet, port, location, or username.

---

## Appendix: Field Code Reference

### Fixed-Length Fields

| Field | Length | Description |
|-------|--------|-------------|
| Patron Status | 14 chars | 14 Y/N flags for patron status |
| Language | 3 chars | ISO 639-2 language code (e.g., "000" = unknown, "eng" = English) |
| Transaction Date | 18 chars | YYYYMMDD    HHMMSS (4 spaces between) |
| Summary | 10 chars | 10 Y/N flags for patron info detail control |
| Circulation Status | 2 chars | SIP2 circulation status code (00-99) |
| Security Marker | 2 chars | Security marker code (00-99) |
| Fee Type | 2 chars | Fee type code (01-99) |

### Variable-Length Field Codes

| Code | Field Name | Description | Max Length |
|------|------------|-------------|------------|
| AA | Patron Identifier | Patron barcode | Variable |
| AB | Item Identifier | Item barcode or instance UUID | Variable |
| AC | Terminal Password | Terminal password | Variable |
| AD | Patron Password | Patron password/PIN | Variable |
| AE | Personal Name | Patron full name | Variable |
| AF | Screen Message | Messages for display (repeatable) | Variable |
| AG | Print Line | Print messages (repeatable) | Variable |
| AH | Due Date | Due date/time (18 chars) | 18 |
| AJ | Title Identifier | Instance title | 60 (truncated) |
| AM | Library Name | Library name | Variable |
| AN | Terminal Location | Terminal location | Variable |
| AO | Institution ID | Institution identifier | Variable |
| AP | Current Location | Current location/service point | Variable |
| AQ | Permanent Location | Permanent location | Variable |
| AS | Hold Items | Hold item barcodes (repeatable) | Variable |
| AT | Overdue Items | Overdue item barcodes (repeatable) | Variable |
| AU | Charged Items | Charged item barcodes (repeatable) | Variable |
| AV | Fine Items | Fine item details (repeatable) | Variable |
| BD | Home Address | Patron address | Variable |
| BE | Email Address | Patron email | Variable |
| BF | Home Phone | Patron phone | Variable |
| BH | Currency Type | Currency code (3 chars) | 3 |
| BK | Transaction ID | Transaction identifier | Variable |
| BL | Valid Patron | Valid patron flag (Y/N) | 1 |
| BM | Renewed Items | Renewed item barcodes (repeatable) | Variable |
| BN | Unrenewed Items | Unrenewed item barcodes (repeatable) | Variable |
| BO | Fee Acknowledged | Fee acknowledged (Y/N) | 1 |
| BT | Fee Type | Fee type code | 2 |
| BU | Recall Items | Recall item barcodes (repeatable) | Variable |
| BV | Fee Amount | Fee amount | Variable |
| BX | Supported Messages | 16-char message support flags | 16 |
| BZ | Hold Items Limit | Hold items limit | 4 |
| CA | Overdue Items Limit | Overdue items limit | 4 |
| CB | Charged Items Limit | Charged items limit | 4 |
| CC | Fee Limit | Fee limit | Variable |
| CD | Unavailable Holds | Unavailable hold barcodes (repeatable) | Variable |
| CF | Hold Queue Length | Hold queue length | 4 |
| CG | Fee Identifier | Account ID | Variable |
| CH | Material Type | FOLIO material type name | Variable |
| CI | Security Inhibit | Security inhibit flag (Y/N) | 1 |
| CK | Media Type | SIP2 media type code | 3 |
| CL | Sort Bin | Sort bin identifier | Variable |
| CM | Hold Shelf Expiration / Security Marker | Hold expiration date or security marker | Variable |
| CN | Login User ID | Login username | Variable |
| CO | Login Password | Login password | Variable |
| CP | Location Code | Location code | Variable |
| CQ | Valid Patron Password | Valid password flag (Y/N) | 1 |
| CS | Call Number | Call number | Variable |
| CT | Destination Location / Routing Location | Pickup/routing service point | Variable |
| CV | Alert Type | Alert type code | 2 |
| CY | Requestor Barcode | Requestor patron barcode | Variable |
| DA | Requestor Name / Transaction Date | Requestor name or transaction date | Variable |
| DE | Work Description | Summary/description | 255 (truncated) |
| EA | Primary Contributor | Primary author/contributor | Variable |
| IN | ISBN Identifier | ISBN (repeatable) | Variable |
| NB | Other Standard ID | UPC/other IDs (repeatable) | Variable |
| SA-SZ | Custom Fields | Patron custom fields (configurable) | Variable (max: 60 default) |

---

## Appendix: FOLIO API Endpoints Used

| Endpoint | Method | Purpose | Used By Messages |
|----------|--------|---------|------------------|
| `/authn/login` | POST | System authentication | 93 |
| `/bl/users/by-barcode/{barcode}` | GET | Get user by barcode | 23, 63, 64 |
| `/users/{id}` | GET | Get user by ID | 63, 64 |
| `/circulation/check-in-by-barcode` | POST | Checkin item | 09 |
| `/circulation/check-out-by-barcode` | POST | Checkout item | 11 |
| `/circulation/renew-by-barcode` | POST | Renew item | 29 |
| `/circulation/renew-by-id` | POST | Renew item by ID | 65 |
| `/circulation/loans` | GET | Get loans | 17, 29, 63, 65 |
| `/circulation/requests` | GET | Get requests | 09, 17 |
| `/item-storage/items` | GET | Get item by barcode | 09, 11, 17, 29 |
| `/item-storage/items/{id}` | GET | Get item by ID | 63 |
| `/holdings-storage/holdings/{id}` | GET | Get holdings record | 09, 11, 17, 29 |
| `/instance-storage/instances/{id}` | GET | Get instance | 09, 11, 17, 29 |
| `/locations/{id}` | GET | Get location | 09 |
| `/material-types/{id}` | GET | Get material type | 09 |
| `/service-points/{id}` | GET | Get service point | 09, 17 |
| `/groups/{id}` | GET | Get patron group | 63 |
| `/accounts` | GET | Get accounts (fines/fees) | 23, 37, 63 |
| `/accounts/{id}/pay` | POST | Pay account | 37 |

---

## Document Version

**Version**: 1.0
**Last Updated**: 2025-11-25
**Maintained By**: FSIP2 Team
