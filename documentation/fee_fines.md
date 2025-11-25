# Fee/Fine Management in fsip2

This document explains how fee/fine information is retrieved and payments are processed using the SIP2 protocol.

## Overview

The fee/fine feature consists of two main operations:
1. **Retrieving fee/fine information** via Patron Information (63/64 messages)
2. **Accepting payments** via Fee Paid (37/38 messages)

## Retrieving Fee/Fine Information (Patron Information - Message 64)

### How It Works

When a Patron Information request (63) is received, the server:

1. **Authenticates the patron** using system credentials (from Login 93 message)
2. **Retrieves open fee/fine accounts** from FOLIO that are:
   - Open (not closed)
   - Not suspended claim returned status
3. **Includes fee/fine data in the response** based on the summary field

### Response Format

The Patron Information response (64) includes fee/fine information in multiple ways:

#### Fixed Field Counts (Always Included)

The fine items count (position 61-64) **always** reflects the actual number of open fee/fine accounts, regardless of what the summary field requests. This follows the SIP2 specification requirement that counts must always be accurate.

```
64<patron_status><language><transaction_date><hold_items_count>
  <overdue_items_count><charged_items_count><fine_items_count>...
```

#### Variable Fields

**BV** - Total Outstanding Balance (always included when accounts exist)
- Total remaining balance across all open accounts
- Format: `BV<amount>` (e.g., `BV6.95`)

**BH** - Currency Type (always included when accounts exist)
- Currency code from tenant configuration (defaults to USD)
- Format: `BH<currency>` (e.g., `BHUSD`)

**AV** - Fine Item Details (conditional)
- Only included when summary field position 3 is 'Y'
- Repeatable field - one per open account
- Format: `AV<accountID> <remaining balance> "<Fee/Fine type>" <Instance title>`

Example:
```
AV4d20dfef-6290-4b65-be2c-69fbe6556e75 6.95 "Lost item fee" Harry Potter
```

### AV Field Format Specification

```
AV<accountID> <remaining> "<feeFineType>" <title>
```

Components:
- **accountID**: FOLIO account UUID (used in payment messages)
- **remaining**: Remaining balance formatted as decimal (e.g., `6.95`)
- **feeFineType**: Always quoted, contains the type from FOLIO (e.g., `"Lost item fee"`)
- **title**: Instance title from FOLIO, truncated to 60 characters if needed

### Parsing AV Fields with Regex

Vendors can use these regex patterns to extract information from the AV field:

**Include fee/fine Type:**
```regex
^\s?(?<id>\S*)\s+(?<price>\d+\.\d{2})\s+"(?<type>[^"]+)"\s+(?<title>.+)$
```

**Exclude fee/fine type (used by MK Selfchecks):**
```regex
^\s?(?<id>\S+)\s+(?<price>\d+\.\d{2})\s+"(?:[^"]+)"\s+(?<title>.+)$
```

### Implementation Details

See [patron_information.go:254-268](../../internal/handlers/patron_information.go#L254-L268) for account retrieval and [patron_information.go:590-609](../../internal/handlers/patron_information.go#L590-L609) for AV field construction.

---

## Accepting Payments (Fee Paid - Message 37)

### How It Works

The Fee Paid handler supports two payment modes:

1. **Single Account Payment** - Payment applied to a specific account
2. **Bulk Payment** - Payment distributed across all eligible accounts

### Payment Request Format

```
37<transaction_date><fixed_fields>|BV<amount>|AO<institution_id>|AA<patron_barcode>|AD<patron_password>|CG<account_id>|AY<sequence>AZ<checksum>
```

#### Required Fields

- **BV** - Fee Amount: Amount to pay (must be positive)
- **AO** - Institution ID: Service point UUID
- **AA** - Patron Identifier: Patron barcode
- **CG** - Fee Identifier (optional): FOLIO account UUID from AV field

#### Optional Fields

- **AD** - Patron Password: PIN verification if required by tenant config
- **BK** - Transaction ID: External transaction identifier

### Configuration Options

The following tenant configuration options control payment behavior:

- **paymentMethod**: Method of payment (e.g., "Cash", "Credit Card")
- **notifyPatron**: Whether to send notification to patron (boolean)
- **acceptBulkPayment**: Whether to allow bulk payment mode (boolean)

### Single Account Payment

When a **CG field** (account ID) is provided:

1. **Validates** patron credentials if required
2. **Checks account eligibility**:
   - Account must exist
   - Account must be open
   - Account must not be suspended claim returned
3. **Applies full payment** to the specified account
4. **Fallback behavior**: If account not eligible and `acceptBulkPayment` is enabled, falls back to bulk payment

See [fee_paid.go:212-269](../../internal/handlers/fee_paid.go#L212-L269) for single payment implementation.

### Bulk Payment

When **no CG field** is provided OR when single payment falls back:

1. **Retrieves all eligible accounts** for the patron
2. **Distributes payment evenly** across all accounts:
   - Base amount = `floor(total_amount / account_count * 100) / 100`
   - Remainder goes to the last account
3. **Processes each payment** individually
4. **Partial success handling**: Continues processing even if some payments fail

Example distribution for $10.00 across 3 accounts:
- Account 1: $3.33
- Account 2: $3.33
- Account 3: $3.34 (receives remainder)

See [fee_paid.go:271-365](../../internal/handlers/fee_paid.go#L271-L365) for bulk payment implementation.

### Payment Response Format (Message 38)

#### Success Response

```
38Y<transaction_date>|AO<institution_id>|AA<patron_barcode>|CG<account_id>|FA<remaining_balance>|FC<payment_date>|FE<feefine_id>|FG<amount_applied>|AF<screen_message>|AY<sequence>AZ<checksum>
```

**Repeatable Fields** (one set per successful payment):
- **CG** - Account ID: FOLIO account UUID payment was applied to
- **FA** - Remaining Balance: Updated remaining balance after payment
- **FC** - Payment Date: Date/time payment was applied (format: `YYYYMMDD    HHMMSS`)
- **FE** - Fee/Fine Identifier: FOLIO feeFineId
- **FG** - Amount Applied: Amount applied to this specific account

**Screen Messages**:
- Single payment: `AFPayment accepted`
- Bulk payment (all succeeded): `AFBulk payment applied`
- Bulk payment (partial failure): `AFBulk payment applied - see staff for details`

Example successful response:
```
38Y20240315    142530|AO7c5abc9f-f3d7-4856-b8d7-058557855bf7|AA112233|CG4d20dfef-6290-4b65-be2c-69fbe6556e75|FA0.00|FC20240315    142530|FE8a1a3c5e-9b2d-4e6f-a1c3-5e7f9a1b3c5d|FG6.95|AFPayment accepted|AY2AZDDD3
```

#### Error Response

```
38N<transaction_date>|AO<institution_id>|AA<patron_barcode>|AF<error_message>|AY<sequence>AZ<checksum>
```

Common error messages:
- `"Validation failed"` - Missing required fields
- `"Invalid fee amount"` - Amount is zero, negative, or not a valid number
- `"Authentication failed"` - System authentication token unavailable
- `"No fee/fine could be found by ID"` - Account ID not found or not eligible
- `"Account ID required"` - No CG field provided and bulk payment disabled
- `"Payment failed - see staff for details"` - All payment attempts failed

### Payment Flow Diagram

```
Fee Paid Request (37)
         |
         v
    CG provided?
    /          \
   YES          NO
   |            |
   v            v
Check Account   acceptBulkPayment?
Eligibility     /              \
   |           YES              NO
   v           |                |
Eligible?      v                v
/      \    Bulk Payment    Return Error
YES     NO     |
|       |      v
|       |   Distribute
|       |   Across All
|       |   Accounts
|       |      |
|       +------+
|              |
v              v
Apply      Partial Success
Payment    Handling
|              |
v              v
Return 38Y Response
```

## Configuration Example

```yaml
tenant:
  - id: "diku"
    payment_method: "SIP2 Online"
    notify_patron: false
    accept_bulk_payment: true
    currency: "USD"
```

## Error Handling

The payment system includes comprehensive error handling:

1. **Validation errors**: Invalid amounts, missing fields
2. **Authentication errors**: Failed patron verification
3. **Account errors**: Account not found, not eligible
4. **Partial failures**: Some accounts succeed in bulk payment
5. **Complete failures**: No payments succeeded

All errors are logged with structured logging and return appropriate 38N responses with descriptive messages.
