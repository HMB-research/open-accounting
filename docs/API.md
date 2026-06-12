# API Reference

Complete API reference for Open Accounting. Interactive documentation available at `/swagger/` when running the server.

## Base URL

```
http://localhost:8080/api/v1
```

Operational endpoints are mounted outside the versioned `/api/v1` prefix:

```http
GET /health
GET /api/demo/status?user=1
POST /api/demo/reset
POST /api/demo/reset?user=1
```

`/health` returns plain text `OK`. The demo endpoints are available only when demo mode is enabled and require the `X-Demo-Secret` header. `GET /api/demo/status` requires a `user` query parameter. `POST /api/demo/reset` resets all demo users unless `user` is set to a specific demo user number.

## Authentication

All endpoints (except `/auth/*`, `/invitations/*`, and demo reset/status endpoints when enabled) require a Bearer token:

```bash
curl -H "Authorization: Bearer <access_token-or-api-token>" \
     https://api.example.com/api/v1/me
```

Bearer auth supports two token types:

- JWT access tokens from `/auth/login` and `/auth/refresh`
- tenant-scoped API tokens created under `/tenants/{tenantId}/api-tokens`

Refresh tokens are accepted only by `/auth/refresh`; they cannot authorize API requests. Access tokens cannot be used as refresh tokens. Suspended tenant memberships cannot use tenant endpoints, and tenant-scoped API tokens stop validating while their owning membership is suspended.

### Register

Create a new user account.

```http
POST /auth/register
Content-Type: application/json

{
  "email": "user@example.com",
  "password": "securepassword",
  "name": "John Doe"
}
```

**Response (201 Created):**

```json
{
  "id": "uuid",
  "email": "user@example.com",
  "name": "John Doe"
}
```

### Login

Authenticate and receive JWT tokens.

```http
POST /auth/login
Content-Type: application/json

{
  "email": "user@example.com",
  "password": "securepassword",
  "tenant_id": "uuid"  // Optional: login directly to a tenant
}
```

**Response (200 OK):**

```json
{
  "access_token": "eyJhbGc...",
  "refresh_token": "eyJhbGc...",
  "token_type": "Bearer",
  "expires_in": 900,
  "user": {
    "id": "uuid",
    "email": "user@example.com",
    "name": "John Doe"
  }
}
```

### Refresh Token

Exchange a refresh token for a new access token. The token must be a JWT refresh token issued by `/auth/login`; access tokens are rejected here.

```http
POST /auth/refresh
Content-Type: application/json

{
  "refresh_token": "eyJhbGc...",
  "tenant_id": "uuid"  // Optional: switch tenant context
}
```

The response includes a replacement `refresh_token`; store the new refresh token and discard the old one.

### Logout

Revoke a refresh token session.

```http
POST /auth/logout
Content-Type: application/json

{
  "refresh_token": "eyJhbGc..."
}
```

### Change Password

Change the authenticated user's password. The current password must match, the new password must be at least 8 characters, and active refresh-token sessions are revoked after a successful change.

```http
PUT /auth/password
Authorization: Bearer <access-token-or-api-token>
Content-Type: application/json

{
  "current_password": "old-password",
  "new_password": "new-password"
}
```

### Password Reset

Request a one-time password reset token. The endpoint returns the same accepted response whether the account exists or not. Token issuance is throttled for repeated active-account requests. In production, configure `PASSWORD_RESET_BASE_URL` plus `PASSWORD_RESET_SMTP_*` settings to send the token by email; `PASSWORD_RESET_EXPOSE_TOKEN=true` is intended only for local/development automation.

```http
POST /auth/password-reset/request
Content-Type: application/json

{
  "email": "user@example.com"
}
```

Confirm the reset with the one-time token and a new password. The token must be unused and unexpired, the new password must be at least 8 characters, and active refresh-token sessions are revoked after a successful reset.

```http
POST /auth/password-reset/confirm
Content-Type: application/json

{
  "token": "reset-token",
  "new_password": "new-password"
}
```

### Auth Sessions

List and revoke refresh-token sessions for the authenticated user.

```http
GET /auth/sessions
Authorization: Bearer <access-token-or-api-token>
```

Pass `include_inactive=true` to include revoked and expired sessions.

```http
DELETE /auth/sessions/{sessionID}
Authorization: Bearer <access-token-or-api-token>
```

Use the collection delete endpoint to revoke every active refresh-token session for the authenticated user.

```http
DELETE /auth/sessions
Authorization: Bearer <access-token-or-api-token>
```

### Security Events

List recent auth security audit events where the authenticated user is the actor or target.

```http
GET /auth/security-events?limit=50
Authorization: Bearer <access-token-or-api-token>
```

---

## API Tokens

Tenant-scoped API tokens are intended for CLI and automation usage. They are valid only for the tenant path they were created for, and only while the owning user's tenant membership is active.

### List API Tokens

```http
GET /tenants/{tenantId}/api-tokens
Authorization: Bearer <jwt-or-api-token>
```

### Create API Token

```http
POST /tenants/{tenantId}/api-tokens
Authorization: Bearer <jwt-or-api-token>
Content-Type: application/json

{
  "name": "CI automation",
  "expires_at": "2026-06-01T00:00:00Z"  // Optional
}
```

**Response (201 Created):**

```json
{
  "token": "oa_...",
  "api_token": {
    "id": "uuid",
    "tenant_id": "uuid",
    "user_id": "uuid",
    "name": "CI automation",
    "token_prefix": "oa_1234abcd5678",
    "expires_at": "2026-06-01T00:00:00Z",
    "created_at": "2026-03-12T00:00:00Z"
  }
}
```

The raw `token` value is returned only once at creation time. Token creation is recorded in auth security events.

### Revoke API Token

```http
DELETE /tenants/{tenantId}/api-tokens/{tokenId}
Authorization: Bearer <jwt-or-api-token>
```

Revoking a token records an auth security event.

---

## User Endpoints

### Get Current User

```http
GET /me
Authorization: Bearer <token>
```

**Response:**

```json
{
  "id": "uuid",
  "email": "user@example.com",
  "name": "John Doe",
  "created_at": "2025-01-01T00:00:00Z"
}
```

### List User's Tenants

```http
GET /me/tenants
Authorization: Bearer <token>
```

**Response:**

```json
[
  {
    "tenant": {
      "id": "uuid",
      "name": "My Company",
      "slug": "my-company"
    },
    "role": "owner",
    "is_default": true
  }
]
```

---

## Tenant Endpoints

### Create Tenant

```http
POST /tenants
Authorization: Bearer <token>
Content-Type: application/json

{
  "name": "Acme Corp",
  "slug": "acme-corp",
  "settings": {
    "default_currency": "EUR",
    "country_code": "EE",
    "timezone": "Europe/Tallinn"
  }
}
```

### Get Tenant

```http
GET /tenants/{tenantId}
Authorization: Bearer <token>
```

### Update Tenant

```http
PUT /tenants/{tenantId}
Authorization: Bearer <token>
Content-Type: application/json

{
  "name": "Acme Corp",
  "settings": {
    "email": "finance@acme.example"
  }
}
```

`period_lock_date` is returned on tenant reads, but it is no longer mutable through the generic tenant settings endpoint. Use the explicit period close/reopen endpoints below so changes are audited.

### Complete Onboarding

```http
POST /tenants/{tenantId}/complete-onboarding
Authorization: Bearer <token>
```

### List Period Close Events

```http
GET /tenants/{tenantId}/period-close-events?limit=20
Authorization: Bearer <token>
```

Returns recent close and reopen events for the tenant.

### Close Period

```http
POST /tenants/{tenantId}/period-close
Authorization: Bearer <token>
Content-Type: application/json

{
  "period_end_date": "2026-01-31",
  "note": "Month-end checks completed",
  "reviewer_sign_off": false
}
```

- `period_end_date` must be `YYYY-MM-DD`
- the date must be the last day of a month
- `reviewer_sign_off` is required when the selected period is the fiscal year-end
- only roles with close permissions can perform this action

### Reopen Period

```http
POST /tenants/{tenantId}/period-reopen
Authorization: Bearer <token>
Content-Type: application/json

{
  "period_end_date": "2026-01-31",
  "note": "Accrual correction required"
}
```

- `note` is required
- reopen restores the previous lock state for that period instead of guessing from the date alone
- reopening a fiscal year is rejected once a year-end carry-forward journal has already been posted for that year

### Year-End Close Status

```http
GET /tenants/{tenantId}/year-end-close-status?period_end_date=2025-12-31
GET /tenants/{tenantId}/year-end-close-pack?period_end_date=2025-12-31
GET /tenants/{tenantId}/year-end-close-audit-evidence?period_end_date=2025-12-31
GET /tenants/{tenantId}/year-end-close-audit-archive?period_end_date=2025-12-31
Authorization: Bearer <token>
```

Returns the fiscal-year readiness summary for the selected date, including:

- fiscal-year start and end dates
- whether the selected date matches the fiscal year-end
- whether the tenant is currently locked through that date
- whether revenue/expense activity exists for the year
- retained-earnings account mapping
- whether a carry-forward journal already exists
- the `close_pack_evidence_entity_id` used for required close-pack evidence
- approved close-pack evidence compliance when document evidence is configured

The close pack endpoint adds the year-end trial balance, balance sheet, and income statement to the readiness summary. The audit evidence endpoint returns that close pack plus the close-pack evidence-policy result and attached close-pack document metadata for reviewer/auditor handoff. The audit archive endpoint returns a ZIP with `manifest.json` plus attached close-pack document files.

Before a fiscal-year close with reviewer sign-off, upload and approve at least one `close_pack` document against the returned `year_end_close` evidence entity. The same approved close-pack evidence is also required before posting year-end carry-forward.

### Post Year-End Carry-Forward

```http
POST /tenants/{tenantId}/year-end-carry-forward
Authorization: Bearer <token>
Content-Type: application/json

{
  "period_end_date": "2025-12-31"
}
```

- only roles with close permissions can perform this action
- the fiscal year must already be closed through the selected year-end
- a carry-forward cannot be posted twice for the same fiscal year
- the journal entry is posted on the first day of the next fiscal year using `source_type = YEAR_END_CARRY_FORWARD`

### Reverse Year-End Carry-Forward

```http
POST /tenants/{tenantId}/year-end-carry-forward/reverse
Authorization: Bearer <token>
Content-Type: application/json

{
  "period_end_date": "2025-12-31",
  "reason": "Late supplier accrual"
}
```

- only roles with close permissions can perform this action
- `reason` is required
- the selected date must match the fiscal year-end
- an existing posted carry-forward must exist for the fiscal year
- the original carry-forward is voided and a posted reversal journal is created on the original carry-forward date using `source_type = YEAR_END_CARRY_FORWARD_REVERSAL`

### Period Lock Behavior

When `settings.period_lock_date` is set, core write paths reject back-dated operations on or before the lock date with `409 Conflict`.

This currently applies to:

- journal entry create, post, and void
- invoice create and void
- payment creation
- payment creation from bank transactions
- opening-balance import

Invoice import also enforces the lock, but because it is a bulk operation, locked invoice rows are returned as row errors in the import summary instead of failing the whole request with `409 Conflict`.

### Validate Migration Bundle

```http
POST /tenants/{tenantId}/migration/validate
Authorization: Bearer <token>
Content-Type: application/json
```

```json
{
  "files": [
    {
      "kind": "contacts",
      "file_name": "contacts.csv",
      "csv_content": "contact_code,name\nCUST-1,Customer One\n"
    },
    {
      "kind": "invoices",
      "file_name": "invoices.csv",
      "csv_content": "invoice_number,contact_code,issue_date,line_description,quantity,unit_price,vat_rate\nINV-1,CUST-1,2026-05-30,Work,1,100,22\n"
    },
    {
      "kind": "e_invoices",
      "file_name": "supplier-einvoices.xml",
      "xml_content": "<?xml version=\"1.0\" encoding=\"UTF-8\"?><E_Invoice>...</E_Invoice>"
    }
  ]
}
```

Returns a non-mutating cutover report with required-column checks and cross-file reference issues for supported migration CSV files plus Estonian e-invoice XML bundles. Supported `kind` values are `accounts`, `contacts`, `employees`, `expenses`, `invoices`, `e_invoices`, `payments`, `bank_accounts`, `bank_transactions`, `payroll_history`, `leave_balances`, `tsd_history`, `kmd_history`, `quotes`, `orders`, `recurring_invoices`, `cost_centers`, `cost_allocations`, `product_categories`, `warehouses`, `products`, `stock_adjustments`, `fixed_assets`, `opening_balances`, and `journal_entries`. CSV files use `csv_content`; `e_invoices` files use `xml_content`. Cost-allocation validation checks required `journal_entry_line_id`, `amount`, `allocation_date`, and `cost_center_id` or `cost_center_code` columns, plus same-bundle cost-center references when a cost center file is included. Product validation checks category IDs or names and sale, purchase, and inventory account IDs or codes when those related files are included. Commercial document validation checks invoice, quote, order, and recurring-invoice line `product_id` or `product_code` values against product files when both are included. Fixed-asset validation checks asset, depreciation expense, and accumulated depreciation account IDs or codes against account files when both are included. Stock-adjustment validation recognizes optional lot metadata columns including `lot_number`, `serial_number`, and `expiry_date` plus common aliases such as `batch`, `serial`, and `expiration_date`.

When the related files are present in the same bundle, the validator also checks references such as commercial documents and e-invoice suppliers to contacts, payments to CSV or XML invoices by ID or number, payroll/leave/TSD rows to employees, account parent codes and expense account/payment account codes to accounts, fixed-asset accounting codes to accounts, product category and account references to product categories and accounts, stock rows to products and warehouses, cost centers to parent cost centers, cost allocations to cost centers, product categories to parent categories, and opening balances or journals to accounts. Hierarchy files also reject self-parent rows before import.

### List Recent Journal Entries

```http
GET /tenants/{tenantId}/journal-entries?limit=50
Authorization: Bearer <token>
```

- returns the most recent journal entries with their lines
- `limit` defaults to `50` and is capped at `200`

### Document Attachments

Document attachments currently support `invoice`, `journal_entry`, `payment`, `bank_transaction`, `asset`, `expense`, `quote`, `order`, and `year_end_close` entities.

#### List Documents

```http
GET /tenants/{tenantId}/documents?entity_type=invoice&entity_id=<uuid>
Authorization: Bearer <token>
```

#### Upload Document

```http
POST /tenants/{tenantId}/documents
Authorization: Bearer <token>
Content-Type: multipart/form-data

entity_type=payment
entity_id=<uuid>
document_type=receipt
notes=Matched%20to%20bank%20statement
retention_years=7
file=<binary>
```

- accepts PDFs, images, CSV files, text files, and similar supporting records
- maximum file size is `10 MB`
- supported `entity_type` values currently include `invoice`, `journal_entry`, `payment`, `bank_transaction`, `asset`, `expense`, `quote`, `order`, `year_end_close`, and `leave_record`
- supported `document_type` values currently include `supporting_document`, `receipt`, `reconciliation_evidence`, `contract`, `asset_record`, `tax_support`, `close_pack`, and `other`
- uploads start in `PENDING` review status and can carry optional retention metadata
- set either `retention_until=YYYY-MM-DD` or `retention_years=N` up to `100`; `retention_years` derives `retention_until` from the upload date, and the two fields cannot be combined

#### Download Document

```http
GET /tenants/{tenantId}/documents/{documentId}/download
Authorization: Bearer <token>
```

#### Review Summary

```http
POST /tenants/{tenantId}/documents/review-summary
Authorization: Bearer <token>
Content-Type: application/json

{
  "entity_type": "payment",
  "entity_ids": ["<uuid>", "<uuid>"]
}
```

#### Review Queue

```http
GET /tenants/{tenantId}/documents/review-queue?entity_type=year_end_close&document_type=close_pack&review_status=PENDING&limit=50
Authorization: Bearer <token>
```

Returns a tenant-wide document reviewer queue with optional `entity_type`, `document_type`, `review_status`, and `limit` filters. `review_status` defaults to `PENDING`; use `ALL` to include every review state. This is the reviewer-facing queue for close-pack approvals outside the company settings workflow.

#### Evidence Policy

```http
POST /tenants/{tenantId}/documents/evidence-policy
Authorization: Bearer <token>
Content-Type: application/json

{
  "entity_type": "payment",
  "entity_ids": ["<uuid>", "<uuid>"],
  "rules": [
    {
      "document_types": ["receipt"],
      "min_count": 1,
      "require_approved": true
    }
  ]
}
```

Returns one result per requested entity ID with `compliant`, document-status counts, document-type counts, rule-level accepted counts, and `violations` for missing or unapproved evidence. Omit `document_types` in a rule to allow any supported document type; `min_count` defaults to `1`.

#### Retention Review

```http
GET /tenants/{tenantId}/documents/retention?as_of=2027-03-01&horizon_days=45&include_missing=true
Authorization: Bearer <token>
```

Returns tenant-wide retention administration data for documents with `retention_until` on or before the cutoff date. `include_missing=true` also includes documents without retention metadata. The response includes expired, due-soon, missing-retention, pending-review, rejected, and total counts plus the matching documents.

#### Update Retention Metadata

```http
PATCH /tenants/{tenantId}/documents/{documentId}/retention
Authorization: Bearer <token>
Content-Type: application/json

{
  "retention_until": "2028-03-31"
}
```

Use `{"clear_retention": true}` to clear retention metadata. `retention_until` uses `YYYY-MM-DD` and cannot be sent together with `clear_retention`.

#### Mark Document Reviewed

```http
POST /tenants/{tenantId}/documents/{documentId}/mark-reviewed
Authorization: Bearer <token>
```

#### Review Document

```http
POST /tenants/{tenantId}/documents/{documentId}/review
Authorization: Bearer <token>
Content-Type: application/json

{
  "review_status": "APPROVED",
  "review_note": "Evidence accepted"
}
```

Review statuses are `REVIEWED`, `APPROVED`, and `REJECTED`. Rejected documents require `review_note`; review summaries return approved and rejected counts in addition to pending and reviewed totals.

#### Delete Document

```http
DELETE /tenants/{tenantId}/documents/{documentId}
Authorization: Bearer <token>
```

---

## Accounts (Chart of Accounts)

### List Accounts

```http
GET /tenants/{tenantId}/accounts
Authorization: Bearer <token>
```

**Query Parameters:**

- `active_only` (bool): Filter for active accounts

**Response:**

```json
[
  {
    "id": "uuid",
    "code": "1000",
    "name": "Cash",
    "account_type": "ASSET",
    "parent_id": null,
    "is_active": true
  }
]
```

### Account Hierarchy

```http
GET /tenants/{tenantId}/accounts/hierarchy
Authorization: Bearer <token>
```

Returns a flattened parent-child chart of accounts ordered by account code. Each row includes `depth`, `path`, `parent_code`, and `has_children` fields for grouped chart views. Supports `active_only=true`.

### Create Account

```http
POST /tenants/{tenantId}/accounts
Authorization: Bearer <token>
Content-Type: application/json

{
  "code": "1010",
  "name": "Petty Cash",
  "account_type": "ASSET",
  "parent_id": "uuid",  // Optional
  "description": "Office petty cash"
}
```

### Get Account

```http
GET /tenants/{tenantId}/accounts/{accountId}
Authorization: Bearer <token>
```

### Update Account

```http
PUT /tenants/{tenantId}/accounts/{accountId}
Authorization: Bearer <token>
Content-Type: application/json

{
  "code": "1010",
  "name": "Petty Cash",
  "account_type": "ASSET",
  "parent_id": "uuid",
  "description": "Office petty cash"
}
```

System accounts cannot be updated. `code`, `name`, and `account_type` are required.

### Delete Account

```http
DELETE /tenants/{tenantId}/accounts/{accountId}
Authorization: Bearer <token>
```

Delete deactivates a custom account by returning it with `is_active=false`, preserving ledger history. System accounts cannot be deleted.

### Import Invoices

```http
POST /tenants/{tenantId}/invoices/import
Authorization: Bearer <token>
Content-Type: application/json

{
  "file_name": "invoices.csv",
  "csv_content": "invoice_number,invoice_type,contact_code,issue_date,due_date,status,amount_paid,reference,notes,line_description,quantity,unit,unit_price,discount_percent,vat_rate,vat_treatment,product_code\nINV-EXT-001,SALES,CUST-001,2026-02-01,2026-02-15,SENT,0,PO-12345,Imported migration invoice,Implementation work,1,hour,100.00,0,22,standard,SERV-001\nBILL-RC-001,PURCHASE,SUP-001,2026-02-01,2026-02-15,SENT,0,,Reverse-charge supplier invoice,EU service,1,hour,100.00,0,22,reverse_charge,EU-SERV"
}
```

Rows are grouped by `invoice_number` and `invoice_type`. Contacts are resolved by the first populated contact identifier in this priority order:

- `contact_code`
- `contact_reg_code`
- `contact_email`
- `contact_name`

Invoice line product references may use `product_id` or `product_code`; `sku` and `item_code` are accepted as `product_code` aliases.

**Response (200 OK):**

```json
{
  "file_name": "invoices.csv",
  "rows_processed": 2,
  "invoices_created": 1,
  "lines_imported": 2,
  "rows_skipped": 0
}
```

### Import Estonian E-Invoice XML

```http
POST /tenants/{tenantId}/invoices/import-einvoice
Authorization: Bearer <token>
Content-Type: application/json

{
  "file_name": "supplier-einvoice.xml",
  "invoice_type": "PURCHASE",
  "xml_content": "<?xml version=\"1.0\" encoding=\"UTF-8\"?><E_Invoice>...</E_Invoice>"
}
```

Imports local Estonian e-invoice XML files using the official `E_Invoice` structure. If `invoice_type` is omitted, debit invoices import as `PURCHASE`; credit invoices import as `CREDIT_NOTE`. Use `invoice_type: "SALES"` only when importing an outbound sales e-invoice file. Contacts are matched from the e-invoice party block by registry code, VAT number, email, or name. Direct operator-network send/receive is not covered by this endpoint.

**Response (200 OK):**

```json
{
  "file_name": "supplier-einvoice.xml",
  "rows_processed": 1,
  "invoices_created": 1,
  "lines_imported": 2,
  "rows_skipped": 0
}
```

**Account Types:** `ASSET`, `LIABILITY`, `EQUITY`, `REVENUE`, `EXPENSE`

### Import Accounts

```http
POST /tenants/{tenantId}/accounts/import
Authorization: Bearer <token>
Content-Type: application/json

{
  "file_name": "accounts.csv",
  "csv_content": "code,name,account_type\n1000,Cash,ASSET\n4000,Sales Revenue,REVENUE\n"
}
```

Supported header aliases include `code` / `account_code`, `name` / `account_name`, and `account_type` / `type`.

---

## Journal Entries

### List Journal Entries

```http
GET /tenants/{tenantId}/journal-entries?limit=50
Authorization: Bearer <token>
```

`limit` defaults to `50` and is capped at `200`.

### Get Journal Entry

```http
GET /tenants/{tenantId}/journal-entries/{entryId}
Authorization: Bearer <token>
```

### Create Journal Entry

```http
POST /tenants/{tenantId}/journal-entries
Authorization: Bearer <token>
Content-Type: application/json

{
  "entry_date": "2025-01-15",
  "description": "Office supplies purchase",
  "reference": "INV-001",
  "requires_evidence": true,
  "lines": [
    {
      "account_id": "uuid",
      "debit_amount": "100.00",
      "credit_amount": "0.00",
      "currency": "USD",
      "exchange_rate": "0.92",
      "description": "Office supplies"
    },
    {
      "account_id": "uuid",
      "debit_amount": "0.00",
      "credit_amount": "100.00",
      "currency": "USD",
      "exchange_rate": "0.92",
      "description": "Payment from cash"
    }
  ]
}
```

**Note:** Debits must equal credits in base currency. Line `currency` defaults to `EUR`, omitted or zero `exchange_rate` defaults to `1`, and non-zero exchange rates must be positive. When `requires_evidence` is true, posting is blocked until the journal entry has at least one approved `supporting_document`, `receipt`, or `tax_support` document attached.

### Post Journal Entry

Finalize a draft entry (makes it immutable). Entries marked `requires_evidence` must pass approved journal-entry evidence policy first.

```http
POST /tenants/{tenantId}/journal-entries/{entryId}/post
Authorization: Bearer <token>
```

### Void Journal Entry

Creates a reversal entry.

```http
POST /tenants/{tenantId}/journal-entries/{entryId}/void
Authorization: Bearer <token>
Content-Type: application/json

{
  "reason": "Duplicate entry"
}
```

### Journal Entry Templates

Reusable balanced journal templates reduce repeated manual accruals and adjustments.

```http
GET /tenants/{tenantId}/journal-entry-templates?active_only=true
GET /tenants/{tenantId}/journal-entry-templates/{templateId}
POST /tenants/{tenantId}/journal-entry-templates
POST /tenants/{tenantId}/journal-entry-templates/{templateId}/apply
POST /tenants/{tenantId}/journal-entry-templates/{templateId}/generate
POST /tenants/{tenantId}/journal-entry-templates/generate-due
Authorization: Bearer <token>
```

Create templates with the same line format as manual journal entries, including optional `currency` and positive `exchange_rate` for foreign-currency accruals:

```json
{
  "name": "Monthly rent accrual",
  "description": "Monthly rent accrual",
  "reference": "RENT",
  "requires_evidence": false,
  "frequency": "MONTHLY",
  "start_date": "2026-04-30T00:00:00Z",
  "lines": [
    {
      "account_id": "expense-account-id",
      "description": "Rent expense",
      "debit_amount": "500.00"
    },
    {
      "account_id": "accrual-account-id",
      "description": "Accrued rent",
      "credit_amount": "500.00"
    }
  ]
}
```

Omit `frequency` for an on-demand template. Recurring frequencies are `WEEKLY`, `BIWEEKLY`, `MONTHLY`, `QUARTERLY`, and `YEARLY`; `start_date` is required for recurring templates and becomes the first `next_generation_date` unless an explicit `next_generation_date` is supplied.

Apply a template to create a draft or posted journal entry without advancing recurring schedule metadata:

```json
{
  "entry_date": "2026-04-30T00:00:00Z",
  "description": "April rent accrual",
  "reference": "RENT-APR",
  "post": true
}
```

Generate recurring templates when the schedule should advance:

```json
{
  "entry_date": "2026-04-30T00:00:00Z",
  "post": true
}
```

For all due templates:

```json
{
  "as_of_date": "2026-05-31T00:00:00Z",
  "post": false
}
```

Templates marked `requires_evidence` can be applied or generated as drafts, but cannot be auto-posted because the generated entry needs approved evidence before posting. Due generation returns one result per template and reports per-template errors, including closed-period conflicts.

### Import Opening Balances

```http
POST /tenants/{tenantId}/journal-entries/import-opening-balances
Authorization: Bearer <token>
Content-Type: application/json

{
  "file_name": "opening-balances.csv",
  "entry_date": "2026-01-01",
  "reference": "OB-2026",
  "description": "Opening balances",
  "csv_content": "account_code,debit,credit,description\n1000,1500.00,0,Cash opening balance\n3000,0,1500.00,Owner equity opening balance\n"
}
```

The import creates a journal entry and posts it immediately. If the tenant period is locked for the chosen date, the API returns `409 Conflict`.

### Import Historical Journal Entries

```http
POST /tenants/{tenantId}/journal-entries/import
Authorization: Bearer <token>
Content-Type: application/json

{
  "file_name": "journal-entries.csv",
  "source_type": "LEGACY_GL",
  "post_entries": true,
  "csv_content": "entry_reference,entry_date,entry_description,account_code,line_description,debit,credit\nLEG-001,2026-03-31,Imported sale,1000,Cash received,100.00,0\nLEG-001,2026-03-31,Imported sale,4000,Revenue,0,100.00\n"
}
```

Rows are grouped by `entry_reference`; each group must have one `entry_date`, at least two lines, known `account_code` values, and balanced debit/credit totals. Locked-period groups are skipped with row errors in the import result.

---

## Contacts

### List Contacts

```http
GET /tenants/{tenantId}/contacts
Authorization: Bearer <token>
```

**Query Parameters:**

- `type` (string): `CUSTOMER`, `SUPPLIER`, or `BOTH`
- `active_only` (boolean): Return active contacts only
- `search` (string): Search by name or email

### Create Contact

```http
POST /tenants/{tenantId}/contacts
Authorization: Bearer <token>
Content-Type: application/json

{
  "name": "ABC Supplier",
  "contact_type": "SUPPLIER",
  "email": "contact@abc.com",
  "phone": "+372 555 1234",
  "vat_number": "EE123456789",
  "address_line1": "123 Main St",
  "city": "Tallinn",
  "country_code": "EE",
  "payment_terms_days": 30
}
```

### Get Contact

```http
GET /tenants/{tenantId}/contacts/{contactId}
Authorization: Bearer <token>
```

### Update Contact

```http
PUT /tenants/{tenantId}/contacts/{contactId}
Authorization: Bearer <token>
Content-Type: application/json

{
  "email": "billing@example.com",
  "payment_terms_days": 30,
  "is_active": true
}
```

### Delete Contact

```http
DELETE /tenants/{tenantId}/contacts/{contactId}
Authorization: Bearer <token>
```

Contacts are soft-deleted by deactivating them.

### Import Contacts

```http
POST /tenants/{tenantId}/contacts/import
Authorization: Bearer <token>
Content-Type: application/json

{
  "file_name": "contacts.csv",
  "csv_content": "name,type,email,payment_terms_days\nNorthwind OU,CUSTOMER,ap@northwind.example,14\nSupply Partner,SUPPLIER,purchases@supply.example,30\n"
}
```

Supported header aliases include `name` / `company_name`, `type`, `payment_terms_days` / `payment_days`, and standard contact metadata such as `email`, `phone`, `reg_code`, and `vat_number`.

---

## Payroll

### List Employees

```http
GET /tenants/{tenantId}/employees
Authorization: Bearer <token>
```

**Query Parameters:**

- `active_only` (boolean): return only active employees

### Create Employee

```http
POST /tenants/{tenantId}/employees
Authorization: Bearer <token>
Content-Type: application/json

{
  "employee_number": "EMP-001",
  "first_name": "Mari",
  "last_name": "Maasikas",
  "personal_code": "49001010001",
  "email": "mari@example.com",
  "start_date": "2026-01-15T00:00:00Z",
  "employment_type": "FULL_TIME",
  "apply_basic_exemption": true,
  "basic_exemption_amount": "700.00",
  "funded_pension_rate": "0.02"
}
```

### Get Employee

```http
GET /tenants/{tenantId}/employees/{employeeId}
Authorization: Bearer <token>
```

### Update Employee

```http
PUT /tenants/{tenantId}/employees/{employeeId}
Authorization: Bearer <token>
Content-Type: application/json

{
  "department": "Finance",
  "apply_basic_exemption": false,
  "is_active": true
}
```

### Set Base Salary

```http
POST /tenants/{tenantId}/employees/{employeeId}/salary
Authorization: Bearer <token>
Content-Type: application/json

{
  "amount": "3200.00",
  "effective_from": "2026-03-01T00:00:00Z"
}
```

### Add Salary Component

```http
POST /tenants/{tenantId}/employees/{employeeId}/salary-components
Authorization: Bearer <token>
Content-Type: application/json

{
  "component_type": "SECONDARY_EMPLOYMENT",
  "name": "Evening contract",
  "amount": "600.00",
  "is_taxable": true,
  "is_recurring": true,
  "effective_from": "2026-03-01T00:00:00Z",
  "effective_to": "2026-08-31T00:00:00Z"
}
```

Supported earning component types are `SECONDARY_EMPLOYMENT`, `BONUS`, `COMMISSION`, `BENEFIT`, and `BASE_SALARY`. Active recurring components are included in payroll gross salary.

### List Salary Components

```http
GET /tenants/{tenantId}/employees/{employeeId}/salary-components?active_on=2026-03-15
Authorization: Bearer <token>
```

**Query Parameters:**

- `active_on` (date, optional): return only components active on the given `YYYY-MM-DD` date

### Import Employees

```http
POST /tenants/{tenantId}/employees/import
Authorization: Bearer <token>
Content-Type: application/json

{
  "file_name": "employees.csv",
  "csv_content": "employee_number,first_name,last_name,personal_code,email,start_date,employment_type,apply_basic_exemption,basic_exemption_amount,funded_pension_rate,base_salary,salary_effective_from\nEMP-001,Mari,Maasikas,49001010001,mari@example.com,2026-01-15,FULL_TIME,true,700.00,0.02,3200.00,2026-01-15\n"
}
```

Supported header aliases include `employee_number` / `employee_no`, `personal_code` / `isikukood`, `employment_type` / `type`, `base_salary` / `salary`, and `salary_effective_from` / `effective_from`.

**Response (200 OK):**

```json
{
  "file_name": "employees.csv",
  "rows_processed": 1,
  "employees_created": 1,
  "salaries_created": 1,
  "rows_skipped": 0
}
```

The importer creates employee records first and, when `base_salary` is provided, also creates a recurring base salary component in the same request.

### List Payroll Runs

```http
GET /tenants/{tenantId}/payroll-runs
Authorization: Bearer <token>
```

**Query Parameters:**

- `year` (integer): optional period-year filter

### Create Payroll Run

```http
POST /tenants/{tenantId}/payroll-runs
Authorization: Bearer <token>
Content-Type: application/json

{
  "period_year": 2026,
  "period_month": 3,
  "payment_date": "2026-03-31T00:00:00Z",
  "notes": "March payroll"
}
```

### Get Payroll Run

```http
GET /tenants/{tenantId}/payroll-runs/{runId}
Authorization: Bearer <token>
```

### Calculate Payroll Run

```http
POST /tenants/{tenantId}/payroll-runs/{runId}/calculate
Authorization: Bearer <token>
```

Calculates payslips for active employees that have salary setup.

### Process Payroll Run

```http
POST /tenants/{tenantId}/payroll-runs/{runId}/process
Authorization: Bearer <token>
Content-Type: application/json

{
  "approve": true
}
```

Bulk-calculates payslips for all active employees that have salary setup. When `approve` is true, the same request also approves the calculated run for payment and tax declaration workflows.

### Approve Payroll Run

```http
POST /tenants/{tenantId}/payroll-runs/{runId}/approve
Authorization: Bearer <token>
```

Approves a calculated payroll run for payment and tax declaration workflows.

### List Payroll Run Payslips

```http
GET /tenants/{tenantId}/payroll-runs/{runId}/payslips
Authorization: Bearer <token>
```

### Download Payslip PDF

```http
GET /tenants/{tenantId}/payroll-runs/{runId}/payslips/{payslipId}/pdf
Authorization: Bearer <token>
```

Returns a generated PDF for one employee payslip in the payroll run.

### Payroll Tax Preview

```http
POST /tenants/{tenantId}/payroll/tax-preview
Authorization: Bearer <token>
Content-Type: application/json

{
  "gross_salary": "3200.00",
  "apply_basic_exemption": true,
  "basic_exemption_amount": "700.00",
  "funded_pension_rate": "0.02"
}
```

### Import Historical Payroll

```http
POST /tenants/{tenantId}/payroll-runs/import-history
Authorization: Bearer <token>
Content-Type: application/json

{
  "file_name": "payroll-history.csv",
  "csv_content": "period_year,period_month,status,payment_date,notes,employee_number,gross_salary,income_tax,unemployment_insurance_employee,funded_pension,other_deductions,net_salary,social_tax,unemployment_insurance_employer,total_employer_cost,basic_exemption_applied,payment_status,paid_at\n2025,12,PAID,2026-01-05,Imported December payroll,EMP-001,3200.00,550.00,51.20,64.00,0.00,2534.80,1056.00,25.60,4281.60,50.00,PAID,2026-01-05\n"
}
```

Each CSV row represents one employee payslip. Rows are grouped into a single payroll run by `period_year` + `period_month`.

Required columns:

- `period_year`
- `period_month`
- `gross_salary`
- at least one employee identifier per row

Employee matching supports:

- `employee_number`
- `personal_code`
- `email`
- `first_name` + `last_name`

Supported statuses:

- `APPROVED`
- `PAID`
- `DECLARED`

`status` defaults to `PAID` when omitted. `payment_status` defaults to `PENDING` for `APPROVED` runs and `PAID` for `PAID`/`DECLARED` runs.

If `taxable_income`, `net_salary`, or `total_employer_cost` is omitted, the importer derives it from the supplied gross salary and deduction/tax columns. Existing payroll periods are not overwritten; rows for periods that already have payroll runs are skipped and returned as row errors.

This importer records historical payroll runs and payslips only. Use the leave-balance and TSD history importers for those separate migration records. Accounting journal entries and incumbent-system audit logs remain separate cutover work.

**Response (200 OK):**

```json
{
  "file_name": "payroll-history.csv",
  "rows_processed": 1,
  "payroll_runs_created": 1,
  "payslips_created": 1,
  "rows_skipped": 0
}
```

### Import Leave Balances

```http
POST /tenants/{tenantId}/leave-balances/import
Authorization: Bearer <token>
Content-Type: application/json

{
  "file_name": "leave-balances.csv",
  "csv_content": "year,employee_number,absence_type_code,entitled_days,carryover_days,used_days,pending_days,notes\n2025,EMP-001,ANNUAL_LEAVE,28,2,4,0,Imported leave balance\n"
}
```

The importer creates or updates leave balances by employee + absence type + year.

Required columns:

- `year`
- one absence type identifier: `absence_type_code`, `absence_type`, or `absence_type_id`
- at least one employee identifier per row

Employee matching supports the same identifiers as historical payroll import. Absence types can be matched by code, name, Estonian name, or id. If `entitled_days` is omitted, the absence type default is used. `carryover_days`, `used_days`, and `pending_days` default to zero.

**Response (200 OK):**

```json
{
  "file_name": "leave-balances.csv",
  "rows_processed": 1,
  "leave_balances_created": 1,
  "leave_balances_updated": 0,
  "rows_skipped": 0
}
```

### Import Historical TSD

```http
POST /tenants/{tenantId}/tsd/import-history
Authorization: Bearer <token>
Content-Type: application/json

{
  "file_name": "tsd-history.csv",
  "csv_content": "year,month,status,submitted_at,emta_reference,employee_number,gross_payment,basic_exemption,taxable_amount,income_tax,social_tax,unemployment_insurance_employer,unemployment_insurance_employee,funded_pension\n2025,12,ACCEPTED,2026-01-10,EMTA-2025-12,EMP-001,3200.00,50.00,3150.00,693.00,1056.00,25.60,51.20,64.00\n"
}
```

Each CSV row represents one employee row in a historical TSD declaration. Rows are grouped into declarations by `year` + `month`.

Required columns:

- `year`
- `month`
- `gross_payment`
- at least one employee identifier per row

Employee matching supports the same identifiers as historical payroll import. Supported statuses are `DRAFT`, `SUBMITTED`, `ACCEPTED`, and `REJECTED`; status defaults to `DRAFT` when omitted. Existing TSD declaration periods are skipped rather than overwritten. `payment_type` defaults to `10` when omitted, and `gross_salary` is accepted as an alias for `gross_payment`.

**Response (200 OK):**

```json
{
  "file_name": "tsd-history.csv",
  "rows_processed": 1,
  "declarations_created": 1,
  "rows_imported": 1,
  "rows_skipped": 0
}
```

---

## Leave Management

### Absence Types

```http
GET /tenants/{tenantId}/absence-types?active_only=true
GET /tenants/{tenantId}/absence-types/{typeId}
Authorization: Bearer <token>
```

Absence types define paid or unpaid leave categories, default entitlement days, carryover caps, document requirements, and Estonian reporting codes.

### Leave Balances

```http
GET /tenants/{tenantId}/employees/{employeeId}/leave-balances?year=2026
GET /tenants/{tenantId}/employees/{employeeId}/leave-balances/{year}
PUT /tenants/{tenantId}/employees/{employeeId}/leave-balances/{year}/{typeId}
POST /tenants/{tenantId}/employees/{employeeId}/leave-balances/{year}/initialize
POST /tenants/{tenantId}/leave-balances/import
Authorization: Bearer <token>
```

Update a leave balance:

```json
{
  "entitled_days": "28.00",
  "carryover_days": "2.00",
  "notes": "Imported balance correction"
}
```

Initialize creates missing balances for the employee and year using active absence types. Import uses the CSV payload documented in the Payroll section.

### Leave Records

```http
GET /tenants/{tenantId}/leave-records?employee_id=uuid&year=2026
POST /tenants/{tenantId}/leave-records
GET /tenants/{tenantId}/leave-records/{recordId}
POST /tenants/{tenantId}/leave-records/{recordId}/approve
POST /tenants/{tenantId}/leave-records/{recordId}/reject
POST /tenants/{tenantId}/leave-records/{recordId}/cancel
Authorization: Bearer <token>
```

Create a leave record:

```json
{
  "employee_id": "uuid",
  "absence_type_id": "uuid",
  "start_date": "2026-07-01T00:00:00Z",
  "end_date": "2026-07-05T00:00:00Z",
  "total_days": "5.00",
  "working_days": "3.00",
  "document_number": "DOC-1",
  "document_date": "2026-06-30T00:00:00Z",
  "notes": "Summer leave"
}
```

If the absence type has `requires_document=true`, approving a leave record requires at least one approved `supporting_document` or `tax_support` document attached to the `leave_record` entity. Missing or pending evidence returns `409 Conflict`.

Reject a leave record:

```json
{
  "reason": "Staffing shortage"
}
```

Leave record statuses are `PENDING`, `APPROVED`, `REJECTED`, and `CANCELLED`.

---

## Invoices

### List Invoices

```http
GET /tenants/{tenantId}/invoices
Authorization: Bearer <token>
```

**Query Parameters:**

- `type` (string): `SALES`, `PURCHASE`, or `CREDIT_NOTE`
- `status` (string): `DRAFT`, `SENT`, `PARTIALLY_PAID`, `PAID`, `OVERDUE`, `VOIDED`
- `contact_id` (uuid): Filter by contact
- `from_date` (date): Filter from issue date, `YYYY-MM-DD`
- `to_date` (date): Filter to issue date, `YYYY-MM-DD`
- `search` (string): Search invoice numbers and references

### Create Invoice

Use `invoice_type: "SALES"` for customer sales invoices, `invoice_type: "PURCHASE"` for supplier bills, and `invoice_type: "CREDIT_NOTE"` for credit notes. Purchase invoice lines can carry `account_id` for the expense or asset account used by downstream accounting. Set line `vat_treatment` to `REVERSE_CHARGE` when VAT is self-assessed; the invoice total excludes VAT while the VAT rate is retained for KMD reporting.

```http
POST /tenants/{tenantId}/invoices
Authorization: Bearer <token>
Content-Type: application/json

{
  "invoice_type": "SALES",
  "contact_id": "uuid",
  "issue_date": "2025-01-15",
  "due_date": "2025-01-29",
  "currency": "EUR",
  "lines": [
    {
      "description": "Consulting services",
      "quantity": 10,
      "unit_price": "100.00",
      "vat_rate": "22.00",
      "vat_treatment": "STANDARD"
    }
  ]
}
```

### Get Invoice

```http
GET /tenants/{tenantId}/invoices/{invoiceId}
Authorization: Bearer <token>
```

### Download Invoice PDF

```http
GET /tenants/{tenantId}/invoices/{invoiceId}/pdf
Authorization: Bearer <token>
```

Returns `application/pdf` file.

### Send Invoice

```http
POST /tenants/{tenantId}/invoices/{invoiceId}/send
Authorization: Bearer <token>
```

Draft purchase invoices require at least one approved `receipt`, `supporting_document`, or `tax_support` document attached to the `invoice` entity before they can be sent or emailed. Missing or pending evidence returns `409 Conflict`.

### Void Invoice

```http
POST /tenants/{tenantId}/invoices/{invoiceId}/void
Authorization: Bearer <token>
```

---

## Quotes

### List Quotes

```http
GET /tenants/{tenantId}/quotes
Authorization: Bearer <token>
```

**Query Parameters:**

- `status` (string): `DRAFT`, `SENT`, `ACCEPTED`, `REJECTED`, `EXPIRED`, or `CONVERTED`
- `contact_id` (uuid): Filter by contact
- `from_date` (date): Filter from quote date, `YYYY-MM-DD`
- `to_date` (date): Filter to quote date, `YYYY-MM-DD`
- `search` (string): Search quote numbers and related text

### Create Quote

```http
POST /tenants/{tenantId}/quotes
Authorization: Bearer <token>
Content-Type: application/json

{
  "contact_id": "uuid",
  "quote_date": "2026-03-15T00:00:00Z",
  "valid_until": "2026-04-15T00:00:00Z",
  "currency": "EUR",
  "exchange_rate": "1",
  "notes": "March offer",
  "lines": [
    {
      "description": "Consulting services",
      "quantity": "2",
      "unit": "hour",
      "unit_price": "100.00",
      "discount_percent": "0",
      "vat_rate": "22.00"
    }
  ]
}
```

### Import Quotes

```http
POST /tenants/{tenantId}/quotes/import
Authorization: Bearer <token>
Content-Type: application/json
```

Imports one CSV row per quote line and groups rows by `quote_number`. Required columns are `quote_number`, `quote_date`, a contact identifier (`contact_id`, `contact_code`, `contact_reg_code`, `contact_email`, or `contact_name`), `line_description`, `quantity`, `unit_price`, and `vat_rate`. Optional columns include `valid_until`, `status`, `currency`, `exchange_rate`, `notes`, `unit`, `discount_percent`, and `product_id` or `product_code`; `sku` and `item_code` are accepted as `product_code` aliases. Duplicate quote numbers are skipped.

### Get Quote

```http
GET /tenants/{tenantId}/quotes/{quoteId}
Authorization: Bearer <token>
```

### Download Quote PDF

```http
GET /tenants/{tenantId}/quotes/{quoteId}/pdf
Authorization: Bearer <token>
```

Returns `application/pdf` with an attachment filename based on the quote number, for example `quote-QUO-00001.pdf`.

### Email Quote

```http
POST /tenants/{tenantId}/quotes/{quoteId}/email
Authorization: Bearer <token>
Content-Type: application/json

{
  "recipient_email": "billing@example.com",
  "recipient_name": "Acme",
  "subject": "Quote QUO-00001",
  "message": "Please see attached quote.",
  "attach_pdf": true,
  "require_approved_evidence": true
}
```

Sends the quote with the `QUOTE_SEND` template, optionally attaches the generated PDF, and records an email log linked to the quote. Draft quotes are marked `SENT` after a successful send. When `require_approved_evidence` is true, the quote must have at least one approved `contract` or `supporting_document` document attached to the `quote` entity or the endpoint returns `409 Conflict`.

### Update Quote

```http
PUT /tenants/{tenantId}/quotes/{quoteId}
Authorization: Bearer <token>
Content-Type: application/json
```

Draft quotes can be updated with the same editable fields used by `Create Quote`.

### Delete Quote

```http
DELETE /tenants/{tenantId}/quotes/{quoteId}
Authorization: Bearer <token>
```

### Quote Lifecycle

```http
POST /tenants/{tenantId}/quotes/{quoteId}/send
POST /tenants/{tenantId}/quotes/{quoteId}/accept
POST /tenants/{tenantId}/quotes/{quoteId}/reject
Authorization: Bearer <token>
```

`POST /tenants/{tenantId}/quotes/{quoteId}/send` accepts an optional JSON body `{"require_approved_evidence": true}`. When set, the quote must have at least one approved `contract` or `supporting_document` document attached to the `quote` entity or the endpoint returns `409 Conflict`.

### Convert Quote to Invoice

```http
POST /tenants/{tenantId}/quotes/{quoteId}/convert-to-invoice
Authorization: Bearer <token>
Content-Type: application/json

{
  "issue_date": "2026-03-20T00:00:00Z",
  "due_date": "2026-04-03T00:00:00Z",
  "notes": "Invoice notes"
}
```

Creates a draft sales invoice from an accepted quote, sets the invoice reference to the quote number, and marks the quote `CONVERTED`.

---

## Orders

### List Orders

```http
GET /tenants/{tenantId}/orders
Authorization: Bearer <token>
```

**Query Parameters:**

- `status` (string): `PENDING`, `CONFIRMED`, `PROCESSING`, `SHIPPED`, `DELIVERED`, or `CANCELED`
- `contact_id` (uuid): Filter by contact
- `from_date` (date): Filter from order date, `YYYY-MM-DD`
- `to_date` (date): Filter to order date, `YYYY-MM-DD`
- `search` (string): Search order numbers and related text

### Create Order

```http
POST /tenants/{tenantId}/orders
Authorization: Bearer <token>
Content-Type: application/json

{
  "contact_id": "uuid",
  "order_date": "2026-03-15T00:00:00Z",
  "expected_delivery": "2026-03-22T00:00:00Z",
  "currency": "EUR",
  "exchange_rate": "1",
  "quote_id": "uuid",
  "notes": "March order",
  "lines": [
    {
      "description": "Consulting services",
      "quantity": "2",
      "unit": "hour",
      "unit_price": "100.00",
      "discount_percent": "0",
      "vat_rate": "22.00"
    }
  ]
}
```

### Import Orders

```http
POST /tenants/{tenantId}/orders/import
Authorization: Bearer <token>
Content-Type: application/json
```

Imports one CSV row per order line and groups rows by `order_number`. Required columns are `order_number`, `order_date`, a contact identifier (`contact_id`, `contact_code`, `contact_reg_code`, `contact_email`, or `contact_name`), `line_description`, `quantity`, `unit_price`, and `vat_rate`. Optional columns include `expected_delivery`, `status`, `currency`, `exchange_rate`, `notes`, `quote_id`, `unit`, `discount_percent`, and `product_id` or `product_code`; `sku` and `item_code` are accepted as `product_code` aliases. Duplicate order numbers are skipped.

### Get Order

```http
GET /tenants/{tenantId}/orders/{orderId}
Authorization: Bearer <token>
```

### Download Order PDF

```http
GET /tenants/{tenantId}/orders/{orderId}/pdf
Authorization: Bearer <token>
```

Returns `application/pdf` with an attachment filename based on the order number, for example `order-ORD-00001.pdf`.

### Email Order

```http
POST /tenants/{tenantId}/orders/{orderId}/email
Authorization: Bearer <token>
Content-Type: application/json

{
  "recipient_email": "billing@example.com",
  "recipient_name": "Acme",
  "subject": "Order ORD-00001",
  "message": "Please see attached order confirmation.",
  "attach_pdf": true,
  "require_approved_evidence": true
}
```

Sends the order confirmation with the `ORDER_CONFIRM` template, optionally attaches the generated PDF, and records an email log linked to the order. Pending orders are marked `CONFIRMED` after a successful send. When `require_approved_evidence` is true, the order must have at least one approved `contract` or `supporting_document` document attached to the `order` entity or the endpoint returns `409 Conflict`.

### Check Order Stock

```http
GET /tenants/{tenantId}/orders/{orderId}/stock-check
GET /tenants/{tenantId}/orders/{orderId}/stock-check?warehouse_id={warehouseId}
Authorization: Bearer <token>
```

Returns a non-mutating fulfillment readiness check for each order line. Product lines linked to tracked goods report `AVAILABLE` or `SHORTAGE`; repeated lines for the same product consume the same available quantity cumulatively inside the check. Service, free-text, and other non-tracked lines report `NOT_TRACKED`; deleted or missing product references report `PRODUCT_NOT_FOUND`. Omitting `warehouse_id` sums available stock across all warehouses.

### List Order Stock Reservations

```http
GET /tenants/{tenantId}/orders/{orderId}/stock-reservations
Authorization: Bearer <token>
```

Lists the persisted order-level stock reservations by product and warehouse. These records are updated by `reserve-stock` and `release-stock` and are separate from the aggregate warehouse `reserved_qty` total.

### Get Order Pick List

```http
GET /tenants/{tenantId}/orders/{orderId}/pick-list?warehouse_id={warehouseId}
Authorization: Bearer <token>
```

Builds a warehouse pick list from persisted order-level reservations. Repeated lines for the same product consume the reserved quantity cumulatively. Tracked product lines report `READY`, `SHORTAGE`, or `UNRESERVED`; service and free-text lines report `NOT_TRACKED`.

### Reserve Order Stock

```http
POST /tenants/{tenantId}/orders/{orderId}/reserve-stock
Authorization: Bearer <token>
Content-Type: application/json

{
  "warehouse_id": "uuid",
  "reason": "Pick list"
}
```

Runs the order stock check for the selected warehouse and, only when every tracked product line is available, reserves the cumulative tracked goods quantities for the order. The mutation increases warehouse `reserved_qty` and decreases `available_qty`; it does not ship stock or create a per-order allocation ledger.

### Release Order Stock

```http
POST /tenants/{tenantId}/orders/{orderId}/release-stock
Authorization: Bearer <token>
Content-Type: application/json

{
  "warehouse_id": "uuid",
  "reason": "Order canceled"
}
```

Releases the order's cumulative tracked goods quantities from the selected warehouse's reserved stock back to available stock. The release requires enough warehouse-level reserved quantity for each tracked product.

### Update Order

```http
PUT /tenants/{tenantId}/orders/{orderId}
Authorization: Bearer <token>
Content-Type: application/json
```

Pending or confirmed orders can be updated with the same editable fields used by `Create Order`, except `quote_id`.

### Delete Order

```http
DELETE /tenants/{tenantId}/orders/{orderId}
Authorization: Bearer <token>
```

### Order Lifecycle

```http
POST /tenants/{tenantId}/orders/{orderId}/confirm
POST /tenants/{tenantId}/orders/{orderId}/process
POST /tenants/{tenantId}/orders/{orderId}/ship
POST /tenants/{tenantId}/orders/{orderId}/deliver
POST /tenants/{tenantId}/orders/{orderId}/cancel
Authorization: Bearer <token>
```

`POST /tenants/{tenantId}/orders/{orderId}/confirm` accepts an optional JSON body `{"require_approved_evidence": true}`. When set, the order must have at least one approved `contract` or `supporting_document` document attached to the `order` entity or the endpoint returns `409 Conflict`.

### Convert Order to Invoice

```http
POST /tenants/{tenantId}/orders/{orderId}/convert-to-invoice
Authorization: Bearer <token>
Content-Type: application/json

{
  "issue_date": "2026-03-24T00:00:00Z",
  "due_date": "2026-04-07T00:00:00Z",
  "notes": "Invoice from delivered order"
}
```

Creates a draft sales invoice from a delivered order, copies the order lines, uses the order number as the invoice reference, and stores the created invoice id in `converted_to_invoice_id`. `issue_date`, `due_date`, and `notes` are optional; the API defaults the issue date to now, the due date to 14 days after the issue date, and notes to the order notes. Orders must be `DELIVERED` and not already converted.

---

## Recurring Invoices

### List Recurring Invoices

```http
GET /tenants/{tenantId}/recurring-invoices?active_only=true
Authorization: Bearer <token>
```

### Create Recurring Invoice

```http
POST /tenants/{tenantId}/recurring-invoices
Authorization: Bearer <token>
Content-Type: application/json

{
  "name": "Monthly retainer",
  "contact_id": "uuid",
  "invoice_type": "SALES",
  "currency": "EUR",
  "frequency": "MONTHLY",
  "start_date": "2026-03-15T00:00:00Z",
  "end_date": "2026-12-31T00:00:00Z",
  "payment_terms_days": 21,
  "reference": "RET-1",
  "send_email_on_generation": true,
  "attach_pdf_to_email": true,
  "lines": [
    {
      "description": "Consulting services",
      "quantity": "2",
      "unit": "hour",
      "unit_price": "100.00",
      "discount_percent": "0",
      "vat_rate": "22.00"
    }
  ]
}
```

Frequencies are `WEEKLY`, `BIWEEKLY`, `MONTHLY`, `QUARTERLY`, and `YEARLY`. `attach_pdf_to_email` defaults to true when omitted.

### Import Recurring Invoices

```http
POST /tenants/{tenantId}/recurring-invoices/import
Authorization: Bearer <token>
Content-Type: application/json

{
  "file_name": "recurring-invoices.csv",
  "csv_content": "name,contact_code,frequency,start_date,line_description,quantity,unit_price,vat_rate\nMonthly Retainer,CUST-1,MONTHLY,2026-03-15,Consulting,1,100,22\n"
}
```

Imports one CSV row per recurring template line and groups rows by `name`. Required columns are `name`, `frequency`, `start_date`, a contact identifier (`contact_id`, `contact_code`, `contact_reg_code`, `contact_email`, or `contact_name`), `line_description`, `quantity`, `unit_price`, and `vat_rate`. Optional columns include `invoice_type`, `currency`, `end_date`, `next_generation_date`, `payment_terms_days`, `reference`, `notes`, active/generation/email settings, `unit`, `discount_percent`, `account_id`, and `product_id` or `product_code`; `sku` and `item_code` are accepted as `product_code` aliases. Duplicate template names are skipped.

### Create From Existing Invoice

```http
POST /tenants/{tenantId}/recurring-invoices/from-invoice/{invoiceId}
Authorization: Bearer <token>
Content-Type: application/json

{
  "name": "Repeat invoice",
  "frequency": "QUARTERLY",
  "start_date": "2026-04-01T00:00:00Z",
  "payment_terms_days": 14
}
```

### Get, Update, and Delete Recurring Invoice

```http
GET /tenants/{tenantId}/recurring-invoices/{recurringId}
PUT /tenants/{tenantId}/recurring-invoices/{recurringId}
DELETE /tenants/{tenantId}/recurring-invoices/{recurringId}
Authorization: Bearer <token>
```

Update payloads accept optional `name`, `contact_id`, `frequency`, `end_date`, `payment_terms_days`, `reference`, `notes`, `lines`, and email configuration fields.

### Recurring Invoice Lifecycle

```http
POST /tenants/{tenantId}/recurring-invoices/{recurringId}/pause
POST /tenants/{tenantId}/recurring-invoices/{recurringId}/resume
POST /tenants/{tenantId}/recurring-invoices/{recurringId}/generate
POST /tenants/{tenantId}/recurring-invoices/generate-due
Authorization: Bearer <token>
```

Manual generation returns the generated invoice id and invoice number. `generate-due` processes every due active recurring invoice for the tenant.

---

## Expenses

### List Expenses

```http
GET /tenants/{tenantId}/expenses?status=SUBMITTED&limit=50
Authorization: Bearer <token>
```

Statuses are `DRAFT`, `SUBMITTED`, `APPROVED`, `REJECTED`, and `POSTED`.

### Create Expense

```http
POST /tenants/{tenantId}/expenses
Authorization: Bearer <token>
Content-Type: application/json

{
  "expense_date": "2026-05-30T00:00:00Z",
  "merchant": "Office Store",
  "description": "Printer toner",
  "employee_id": "uuid",
  "contact_id": "uuid",
  "expense_account_id": "uuid",
  "payment_account_id": "uuid",
  "amount": "120.50",
  "currency": "EUR",
  "exchange_rate": "1",
  "requires_receipt": true
}
```

`expense_account_id` must point to an `EXPENSE` account when posted. `payment_account_id` must point to an `ASSET` or `LIABILITY` account and is credited by the posted journal entry.

### Import Expenses

```http
POST /tenants/{tenantId}/expenses/import
Authorization: Bearer <token>
Content-Type: application/json
```

```json
{
  "file_name": "expenses.csv",
  "csv_content": "expense_number,expense_date,merchant,description,expense_account_code,payment_account_code,amount,status\nEXP-LEG-1,2026-05-30,Office Store,Toner,5500,1000,120.50,DRAFT\n"
}
```

Expense imports create unposted expense claims and return row-level errors for invalid rows. Account references can use `expense_account_id`/`payment_account_id` or chart codes via `expense_account_code`/`payment_account_code`. Supported imported statuses are `DRAFT`, `SUBMITTED`, `APPROVED`, and `REJECTED`; `POSTED` must be reached through the normal approval/posting workflow so ledger entries are created consistently. If the tenant period is locked, locked expense-date rows are skipped in the import result.

### Receipt Evidence

Receipt-backed expenses use the document attachment API:

```http
POST /tenants/{tenantId}/documents
Content-Type: multipart/form-data

entity_type=expense
entity_id=<expenseId>
document_type=receipt
file=<binary>
```

Then approve the document with `POST /tenants/{tenantId}/documents/{documentId}/review`. Expenses with `requires_receipt=true` reject approval and posting until at least one linked `receipt` document is approved.

### Lifecycle

```http
GET /tenants/{tenantId}/expenses/{expenseId}
POST /tenants/{tenantId}/expenses/{expenseId}/submit
POST /tenants/{tenantId}/expenses/{expenseId}/approve
POST /tenants/{tenantId}/expenses/{expenseId}/reject
POST /tenants/{tenantId}/expenses/{expenseId}/post
Authorization: Bearer <token>
```

Reject payloads require a reason:

```json
{
  "reason": "Need project code"
}
```

Posting creates and posts a balanced journal entry with source type `EXPENSE`, using the expense account as the debit line and the payment/reimbursement account as the credit line.

---

## Fixed Assets

### Asset Categories

```http
GET /tenants/{tenantId}/asset-categories
POST /tenants/{tenantId}/asset-categories
GET /tenants/{tenantId}/asset-categories/{categoryId}
DELETE /tenants/{tenantId}/asset-categories/{categoryId}
Authorization: Bearer <token>
```

Create category payloads accept `name`, `description`, `depreciation_method`, `default_useful_life_months`, `default_residual_value_percent`, and optional account IDs.
When a new asset references a category, omitted depreciation method, useful life, residual value, asset account, depreciation expense account, and accumulated depreciation account values inherit from that category.
Asset updates preserve omitted category/account values; changing `category_id` without explicit depreciation/account overrides applies the new category defaults.

### List Assets

```http
GET /tenants/{tenantId}/assets
Authorization: Bearer <token>
```

**Query Parameters:**

- `status` (string): `DRAFT`, `ACTIVE`, `DISPOSED`, or `SOLD`
- `category_id` (uuid): Filter by asset category
- `search` (string): Search asset name or asset number

### Create Asset

```http
POST /tenants/{tenantId}/assets
Authorization: Bearer <token>
Content-Type: application/json

{
  "name": "Laptop",
  "category_id": "uuid",
  "purchase_date": "2026-03-15T00:00:00Z",
  "purchase_cost": "1200.00",
  "supplier_id": "uuid",
  "serial_number": "SN-1",
  "location": "Tallinn",
  "depreciation_method": "STRAIGHT_LINE",
  "useful_life_months": 36,
  "residual_value": "100.00",
  "depreciation_start_date": "2026-04-01T00:00:00Z"
}
```

### Import Assets

```http
POST /tenants/{tenantId}/assets/import
Authorization: Bearer <token>
Content-Type: application/json

{
  "file_name": "assets.csv",
  "csv_content": "asset_number,name,status,purchase_date,purchase_cost,accumulated_depreciation,book_value\nLEG-001,Laptop,ACTIVE,2025-01-10,1200.00,300.00,900.00\n"
}
```

Required CSV columns are `name`, `purchase_date`, and `purchase_cost`. Optional columns include `asset_number`, `category_id`, `category_name`, `status`, `description`, supplier/invoice IDs, serial/location fields, depreciation settings, `accumulated_depreciation`, `book_value`, `last_depreciation_date`, disposal fields, and asset/depreciation account IDs or account codes. Account-code columns are `asset_account_code`, `depreciation_expense_account_code`, and `accumulated_depreciation_account_code`. Omitted asset numbers are generated; supplied asset numbers are preserved and checked for duplicates.

### Get, Update, and Delete Asset

```http
GET /tenants/{tenantId}/assets/{assetId}
PUT /tenants/{tenantId}/assets/{assetId}
DELETE /tenants/{tenantId}/assets/{assetId}
Authorization: Bearer <token>
```

### Asset Lifecycle

```http
POST /tenants/{tenantId}/assets/{assetId}/activate
POST /tenants/{tenantId}/assets/{assetId}/dispose
Authorization: Bearer <token>
```

Activating a draft asset requires at least one approved `asset_record`, `receipt`, or `contract` document attached to the `asset` entity. Missing or pending asset evidence returns `409 Conflict` so operators can upload and approve the acquisition record before the asset enters the depreciation workflow.

Dispose payloads include `disposal_date`, `disposal_method`, optional `disposal_proceeds`, optional `disposal_notes`, optional `disposal_proceeds_account_id`, and optional `disposal_gain_loss_account_id`.
Disposing or selling an active asset requires at least one approved `supporting_document` or `contract` document attached to the `asset` entity. Missing or pending disposal evidence returns `409 Conflict`; successful disposal persists the date, method, proceeds, notes, and `disposal_journal_entry_id`. Disposal requires asset and accumulated-depreciation account links, creates and posts a balanced `ASSET_DISPOSAL` journal entry that clears cost and accumulated depreciation, records proceeds to `disposal_proceeds_account_id`, and posts any gain or loss to `disposal_gain_loss_account_id`; gain accounts must be `REVENUE`, and loss accounts must be `EXPENSE`.

### Depreciation

```http
POST /tenants/{tenantId}/assets/{assetId}/depreciation
GET /tenants/{tenantId}/assets/{assetId}/depreciation
Authorization: Bearer <token>
```

Recording depreciation uses the current month according to the server-side service. The asset must have both `depreciation_expense_account_id` and `accumulated_depreciation_account_id`; recording depreciation creates and posts a balanced `ASSET_DEPRECIATION` journal entry and returns its ID as `journal_entry_id` on the depreciation entry. Missing or invalid accounts are rejected as an invalid asset accounting configuration.

---

## Inventory

### Product Categories

```http
GET /tenants/{tenantId}/product-categories
POST /tenants/{tenantId}/product-categories
POST /tenants/{tenantId}/product-categories/import
GET /tenants/{tenantId}/product-categories/{categoryId}
DELETE /tenants/{tenantId}/product-categories/{categoryId}
Authorization: Bearer <token>
```

Create category payloads accept `name`, optional `description`, and optional `parent_id`.

```http
POST /tenants/{tenantId}/product-categories/import
Authorization: Bearer <token>
Content-Type: application/json

{
  "file_name": "categories.csv",
  "csv_content": "name,description,parent_name\nParts,Spare parts,\nFasteners,Bolts and screws,Parts\n"
}
```

Category CSV imports require `name`. Optional columns include `description`, `parent_id`, and `parent_name`; parent names can reference existing categories or earlier rows in the same import.

### List Products

```http
GET /tenants/{tenantId}/products
Authorization: Bearer <token>
```

**Query Parameters:**

- `product_type` (string): `GOODS` or `SERVICE`
- `status` (string): `ACTIVE` or `INACTIVE`
- `category_id` (uuid): Filter by product category
- `search` (string): Search product code or name
- `low_stock` (boolean): Return products below reorder threshold

### Create Product

```http
POST /tenants/{tenantId}/products
Authorization: Bearer <token>
Content-Type: application/json

{
  "code": "PRD-001",
  "name": "Widget",
  "description": "Inventory item",
  "product_type": "GOODS",
  "category_id": "uuid",
  "unit": "pcs",
  "purchase_price": "10.50",
  "sales_price": "15.00",
  "vat_rate": "22.00",
  "min_stock_level": "5",
  "reorder_point": "7",
  "track_inventory": true
}
```

`sales_price` is required. If `code` is omitted, the service generates one.

### Import Products

```http
POST /tenants/{tenantId}/products/import
Authorization: Bearer <token>
Content-Type: application/json

{
  "file_name": "products.csv",
  "csv_content": "code,name,product_type,category_name,sales_price,purchase_price,vat_rate,track_inventory\nPRD-001,Widget,GOODS,Parts,15.00,10.50,22,true\n"
}
```

Required CSV columns are `name` and `sales_price`. Optional columns include `code`, `product_type`, `category_id`, `category_name`, `description`, `unit`, purchase/VAT/reorder prices, account IDs or account-code columns, `track_inventory`, `status` or `is_active`, `barcode`, `supplier_id`, and `lead_time_days`. Account-code columns are `sale_account_code`, `purchase_account_code`, and `inventory_account_code`. Omitted codes are generated; supplied codes are preserved and checked for duplicates. Use inventory stock adjustment commands or APIs after product import to load opening quantities.

### Get, Update, and Delete Product

```http
GET /tenants/{tenantId}/products/{productId}
PUT /tenants/{tenantId}/products/{productId}
DELETE /tenants/{tenantId}/products/{productId}
Authorization: Bearer <token>
```

Update product payloads accept the editable product fields plus `is_active`.

### Product Stock Levels and Movements

```http
GET /tenants/{tenantId}/products/{productId}/stock-levels
GET /tenants/{tenantId}/products/{productId}/movements
Authorization: Bearer <token>
```

### Inventory Valuation

```http
GET /tenants/{tenantId}/inventory/valuation
GET /tenants/{tenantId}/inventory/valuation?warehouse_id={warehouseId}
GET /tenants/{tenantId}/inventory/valuation?method=weighted-average
GET /tenants/{tenantId}/inventory/valuation?method=fifo
Authorization: Bearer <token>
```

Returns tracked `GOODS` stock valuation. `method` accepts `standard-cost` (default, using each product `purchase_price`), `weighted-average` (using costed inbound stock movements), or `fifo` (valuing current quantity from newest remaining inbound layers). Costed methods fall back to purchase price when no usable movement costs exist. The response includes product and warehouse labels, on-hand/reserved/available quantities, line value, and report totals.

### Inventory Lot Report

```http
GET /tenants/{tenantId}/inventory/lots
GET /tenants/{tenantId}/inventory/lots?product_id={productId}
GET /tenants/{tenantId}/inventory/lots?warehouse_id={warehouseId}
GET /tenants/{tenantId}/inventory/lots?include_empty=true
Authorization: Bearer <token>
```

Returns tracked `GOODS` stock grouped by product, warehouse, lot number, serial number, and expiry date from inventory movement metadata. The report includes weighted unit cost per lot position, inventory value, last movement date, and report totals. By default only positive on-hand positions are returned; `include_empty=true` includes zero or negative positions for exhausted or corrective lots.

### Warehouses

```http
GET /tenants/{tenantId}/warehouses?active_only=true
POST /tenants/{tenantId}/warehouses
POST /tenants/{tenantId}/warehouses/import
GET /tenants/{tenantId}/warehouses/{warehouseId}
PUT /tenants/{tenantId}/warehouses/{warehouseId}
DELETE /tenants/{tenantId}/warehouses/{warehouseId}
Authorization: Bearer <token>
```

Create warehouse payloads accept `code`, `name`, optional `address`, and `is_default`. Update payloads accept `name`, optional `address`, `is_default`, and `is_active`.

```http
POST /tenants/{tenantId}/warehouses/import
Authorization: Bearer <token>
Content-Type: application/json

{
  "file_name": "warehouses.csv",
  "csv_content": "code,name,address,is_default,status\nMAIN,Main warehouse,Tallinn,true,ACTIVE\n"
}
```

Warehouse CSV imports require `code` and `name`. Optional columns include `address`, `is_default`, `status`, and `is_active`; `status` accepts `ACTIVE` or `INACTIVE`. Supplied codes are preserved and checked for duplicates.

### Stock Operations

```http
POST /tenants/{tenantId}/inventory/adjust
Authorization: Bearer <token>
Content-Type: application/json

{
  "product_id": "uuid",
  "warehouse_id": "uuid",
  "quantity": "-2",
  "unit_cost": "10.50",
  "lot_number": "LOT-2026-01",
  "serial_number": "SN-001",
  "expiry_date": "2027-01-31",
  "reason": "Cycle count"
}
```

`quantity` is signed: positive quantities add stock and negative quantities remove stock. Adjustments update both the product total stock and the selected warehouse stock level; reductions cannot drive that warehouse below zero or below reserved quantity. `lot_number`, `serial_number`, and `expiry_date` are optional movement metadata fields; `expiry_date` must use `YYYY-MM-DD`.

```http
POST /tenants/{tenantId}/inventory/stock-import
Authorization: Bearer <token>
Content-Type: application/json

{
  "file_name": "stock.csv",
  "csv_content": "product_code,warehouse_code,quantity,unit_cost,lot_number,serial_number,expiry_date,reason\nPRD-001,MAIN,12,10.50,LOT-2026-01,SN-001,2027-01-31,Opening stock\n"
}
```

Stock CSV imports require `quantity`, a product identifier (`product_id` or `product_code`), and a warehouse identifier (`warehouse_id` or `warehouse_code`). Quantities are signed adjustments; use positive quantities for opening stock or inbound counts and negative quantities for reductions. Optional lot metadata columns are `lot_number`, `serial_number`, and `expiry_date`; aliases include `lot`, `batch`, `serial`, and `expiration_date`.

```http
POST /tenants/{tenantId}/inventory/transfer
Authorization: Bearer <token>
Content-Type: application/json

{
  "product_id": "uuid",
  "from_warehouse_id": "uuid",
  "to_warehouse_id": "uuid",
  "quantity": "3",
  "lot_number": "LOT-2026-01",
  "serial_number": "SN-001",
  "expiry_date": "2027-01-31",
  "notes": "Move to branch"
}
```

Transfers require a positive quantity and enough available stock in the source warehouse. `lot_number`, `serial_number`, and `expiry_date` are optional movement metadata fields; `expiry_date` must use `YYYY-MM-DD`. Successful transfers create an outbound movement for the source warehouse, an inbound movement for the destination warehouse, copy any lot metadata to both movements, and update both warehouse stock levels without changing total product stock.

```http
POST /tenants/{tenantId}/inventory/reserve
Authorization: Bearer <token>
Content-Type: application/json

{
  "product_id": "uuid",
  "warehouse_id": "uuid",
  "quantity": "2",
  "reason": "Sales order allocation"
}
```

Reservations require a positive quantity and sufficient warehouse available stock. A successful reservation increases `reserved_qty` and decreases `available_qty` without changing on-hand quantity or product total stock.

```http
POST /tenants/{tenantId}/inventory/release
Authorization: Bearer <token>
Content-Type: application/json

{
  "product_id": "uuid",
  "warehouse_id": "uuid",
  "quantity": "1",
  "reason": "Order canceled"
}
```

Releases require a positive quantity no greater than current reserved stock. A successful release decreases `reserved_qty` and increases `available_qty` without changing on-hand quantity or product total stock.

---

## Cost Centers

### List Cost Centers

```http
GET /tenants/{tenantId}/cost-centers?active_only=true
Authorization: Bearer <token>
```

### Create Cost Center

```http
POST /tenants/{tenantId}/cost-centers
POST /tenants/{tenantId}/cost-centers/import
Authorization: Bearer <token>
Content-Type: application/json

{
  "code": "CC001",
  "name": "Sales",
  "description": "Sales team",
  "is_active": true,
  "budget_amount": "1000.00",
  "budget_period": "MONTHLY"
}
```

```http
POST /tenants/{tenantId}/cost-centers/import
Authorization: Bearer <token>
Content-Type: application/json

{
  "file_name": "cost-centers.csv",
  "csv_content": "code,name,parent_code,budget_amount,budget_period\nSALES,Sales,,1000.00,MONTHLY\nONLINE,Online sales,SALES,500.00,MONTHLY\n"
}
```

Cost center CSV imports require `code` and `name`. Optional columns include `description`, `parent_id`, `parent_code`, `budget_amount`, `budget_period`, `status`, and `is_active`; parent codes can reference existing cost centers or rows in the same import.

### Cost Allocations

```http
GET /tenants/{tenantId}/cost-centers/allocations?cost_center_id={costCenterId}&start_date=2026-03-01&end_date=2026-03-31
POST /tenants/{tenantId}/cost-centers/allocations
POST /tenants/{tenantId}/cost-centers/allocations/import
Authorization: Bearer <token>
Content-Type: application/json

{
  "cost_center_id": "cc-uuid",
  "journal_entry_line_id": "journal-line-uuid",
  "amount": "125.50",
  "allocation_percentage": "50.00",
  "allocation_date": "2026-03-20T00:00:00Z",
  "notes": "Shared office expense"
}
```

```http
POST /tenants/{tenantId}/cost-centers/allocations/import
Authorization: Bearer <token>
Content-Type: application/json

{
  "file_name": "cost-allocations.csv",
  "csv_content": "cost_center_code,journal_entry_line_id,amount,allocation_percentage,allocation_date,notes\nSALES,journal-line-uuid,125.50,50,2026-03-20,Shared office expense\n"
}
```

Cost allocations assign a positive journal-entry-line amount to a cost center for budget-vs-actual and cost-center reporting. Listing supports optional `cost_center_id`, `journal_entry_line_id`, `start_date`, and `end_date` filters; returned rows include joined `cost_center_code` and `cost_center_name` when available. Cost allocation CSV imports require `journal_entry_line_id`, `amount`, `allocation_date`, and either `cost_center_id` or `cost_center_code`; optional columns include `allocation_percentage` and `notes`. Import responses include processed, imported, skipped, and row-level error counts.

### Get, Update, and Delete Cost Center

```http
GET /tenants/{tenantId}/cost-centers/{costCenterId}
PUT /tenants/{tenantId}/cost-centers/{costCenterId}
DELETE /tenants/{tenantId}/cost-centers/{costCenterId}
Authorization: Bearer <token>
```

### Cost Center Report

```http
GET /tenants/{tenantId}/cost-centers/report?start_date=2026-03-01&end_date=2026-03-31&format=csv
Authorization: Bearer <token>
```

Report `format` defaults to JSON and also supports `csv`, `xlsx`, and `pdf`.

Budget periods are `MONTHLY`, `QUARTERLY`, and `ANNUAL`.

---

## Analytics

```http
GET /tenants/{tenantId}/analytics/dashboard
GET /tenants/{tenantId}/analytics/revenue-expense?months=12
GET /tenants/{tenantId}/analytics/cash-flow?months=12
GET /tenants/{tenantId}/analytics/activity?limit=10
Authorization: Bearer <token>
```

Analytics endpoints return dashboard totals, monthly chart arrays, and recent activity items for the tenant.

---

## Payments

### List Payments

```http
GET /tenants/{tenantId}/payments
Authorization: Bearer <token>
```

**Query Parameters:**

- `type` (string): `RECEIVED` or `MADE`
- `method` (string): Filter by payment method
- `contact_id` (uuid): Filter by contact
- `from_date` (date): Filter from payment date, `YYYY-MM-DD`
- `to_date` (date): Filter to payment date, `YYYY-MM-DD`

### Create Payment

```http
POST /tenants/{tenantId}/payments
POST /tenants/{tenantId}/payments/import
Authorization: Bearer <token>
Content-Type: application/json

{
  "payment_type": "RECEIVED",
  "contact_id": "uuid",
  "payment_date": "2025-01-20T00:00:00Z",
  "amount": "1220.00",
  "currency": "EUR",
  "exchange_rate": "1",
  "payment_method": "BANK_TRANSFER",
  "bank_account": "EE471000001020145685",
  "reference": "BANK-001"
}
```

**Payment Types:** `RECEIVED`, `MADE`

The request may include `allocations` with `invoice_id` and `amount` entries to allocate the payment while creating it.

```http
POST /tenants/{tenantId}/payments/import
Authorization: Bearer <token>
Content-Type: application/json

{
  "file_name": "payments.csv",
  "csv_content": "payment_number,payment_type,payment_date,amount,currency,invoice_number,allocation_amount\nPAY-001,RECEIVED,2026-03-15,1220.00,EUR,INV-001,1220.00\n"
}
```

Payment CSV imports require `payment_type`, `payment_date`, and `amount`. Optional columns include `payment_number`, `contact_id`, `currency`, `exchange_rate`, `payment_method`, `bank_account`, `reference`, `notes`, `invoice_id`, `invoice_number`, and `allocation_amount`. Omitted payment numbers are generated; supplied numbers are preserved and checked for duplicates. Payment allocations can target `invoice_id` directly or resolve `invoice_number` through the tenant invoice list.

### Export SEPA Payment XML

```http
POST /tenants/{tenantId}/payments/sepa-export
Authorization: Bearer <token>
Content-Type: application/json
Accept: application/xml

{
  "message_id": "MSG-20260331",
  "debtor_name": "Example OU",
  "debtor_iban": "EE382200221020145685",
  "debtor_bic": "HABAEE2X",
  "execution_date": "2026-04-01",
  "lines": [
    {
      "end_to_end_id": "INV-1001",
      "creditor_name": "Supplier AS",
      "creditor_iban": "EE471000001020145685",
      "amount": "125.50",
      "remittance": "Invoice INV-1001"
    }
  ]
}
```

Returns ISO 20022 `pain.001.001.03` SEPA credit-transfer XML for manual bank upload. The exporter validates debtor and creditor IBAN checksums, optional BIC format, positive EUR amounts, and `YYYY-MM-DD` execution dates. It does not submit payments directly to a bank.

### Get Payment

```http
GET /tenants/{tenantId}/payments/{paymentId}
Authorization: Bearer <token>
```

### Allocate Payment to Invoice

```http
POST /tenants/{tenantId}/payments/{paymentId}/allocate
Authorization: Bearer <token>
Content-Type: application/json

{
  "invoice_id": "uuid",
  "amount": "1220.00"
}
```

### Reverse Payment

```http
POST /tenants/{tenantId}/payments/{paymentId}/reverse
Authorization: Bearer <token>
Content-Type: application/json

{
  "payment_date": "2026-03-20T00:00:00Z",
  "reason": "Duplicate bank import",
  "reference": "REVERSAL-PMT-00001",
  "notes": "Correcting duplicate bank statement import"
}
```

Creates an auditable offsetting payment instead of deleting the original payment. The original payment is marked with `reversed_by_payment_id`, `reversed_at`, `reversed_by`, and `reversal_reason`; the offsetting payment points back with `reversal_of_payment_id`. Allocated payments mirror the original allocations and reduce the related invoices' paid amounts through the payment workflow.

### Get Unallocated Payments

```http
GET /tenants/{tenantId}/payments/unallocated?type=RECEIVED
Authorization: Bearer <token>
```

---

## Payment Reminders

### Overdue Invoices

```http
GET /tenants/{tenantId}/invoices/overdue
Authorization: Bearer <token>
```

Returns overdue invoices with outstanding amount, contact details, days overdue, and prior reminder count.

### Send Reminder

```http
POST /tenants/{tenantId}/invoices/reminders
Authorization: Bearer <token>
Content-Type: application/json

{
  "invoice_id": "uuid",
  "message": "Please pay"
}
```

### Send Bulk Reminders

```http
POST /tenants/{tenantId}/invoices/reminders/bulk
Authorization: Bearer <token>
Content-Type: application/json

{
  "invoice_ids": ["uuid-1", "uuid-2"],
  "message": "Please pay"
}
```

### Invoice Reminder History

```http
GET /tenants/{tenantId}/invoices/{invoiceId}/reminders
Authorization: Bearer <token>
```

### Reminder Rules

```http
GET /tenants/{tenantId}/reminder-rules
POST /tenants/{tenantId}/reminder-rules
GET /tenants/{tenantId}/reminder-rules/{ruleId}
PUT /tenants/{tenantId}/reminder-rules/{ruleId}
DELETE /tenants/{tenantId}/reminder-rules/{ruleId}
POST /tenants/{tenantId}/reminder-rules/trigger
Authorization: Bearer <token>
```

Create a reminder rule:

```json
{
  "name": "Seven days overdue",
  "trigger_type": "AFTER_DUE",
  "days_offset": 7,
  "email_template_type": "OVERDUE_REMINDER",
  "is_active": true
}
```

Trigger types are `BEFORE_DUE`, `ON_DUE`, and `AFTER_DUE`. Rule updates support `name`, `email_template_type`, and `is_active`.

---

## Email

### SMTP Settings

```http
GET /tenants/{tenantId}/settings/smtp
PUT /tenants/{tenantId}/settings/smtp
POST /tenants/{tenantId}/settings/smtp/test
Authorization: Bearer <token>
```

Update SMTP settings:

```json
{
  "smtp_host": "smtp.example.com",
  "smtp_port": 587,
  "smtp_username": "robot",
  "smtp_password": "secret",
  "smtp_from_email": "billing@example.com",
  "smtp_from_name": "Billing",
  "smtp_use_tls": true
}
```

Test SMTP settings:

```json
{
  "recipient_email": "you@example.com"
}
```

### Templates and Log

```http
GET /tenants/{tenantId}/email-templates
PUT /tenants/{tenantId}/email-templates/{templateType}
GET /tenants/{tenantId}/email-log?limit=50
Authorization: Bearer <token>
```

Template types are `INVOICE_SEND`, `QUOTE_SEND`, `ORDER_CONFIRM`, `PAYMENT_RECEIPT`, and `OVERDUE_REMINDER`.

Update an email template:

```json
{
  "subject": "Reminder for {{.InvoiceNumber}}",
  "body_html": "<p>Please pay {{.InvoiceNumber}}</p>",
  "body_text": "Please pay {{.InvoiceNumber}}",
  "is_active": true
}
```

### Send Email

```http
POST /tenants/{tenantId}/invoices/{invoiceId}/email
POST /tenants/{tenantId}/quotes/{quoteId}/email
POST /tenants/{tenantId}/orders/{orderId}/email
POST /tenants/{tenantId}/payments/{paymentId}/email-receipt
Authorization: Bearer <token>
```

Send an invoice email:

```json
{
  "recipient_email": "billing@example.com",
  "recipient_name": "Acme",
  "subject": "Invoice INV-00001",
  "message": "Please see attached.",
  "attach_pdf": true
}
```

Quote and order emails use the same recipient, subject, message, and `attach_pdf` fields. Add `"require_approved_evidence": true` to require approved `contract` or `supporting_document` evidence before sending. A successful quote email marks draft quotes as `SENT`; a successful order email marks pending orders as `CONFIRMED`.

Payment receipt emails use the same recipient, subject, and message fields without `attach_pdf`. Add `"require_approved_evidence": true` to require at least one approved `receipt`, `supporting_document`, or `tax_support` document attached to the payment before sending; missing approved evidence returns `409 Conflict`.

Draft purchase-invoice emails use the same approved invoice-evidence rule as `POST /tenants/{tenantId}/invoices/{invoiceId}/send`.

---

## Interest

### Settings

```http
GET /tenants/{tenantId}/settings/interest
PUT /tenants/{tenantId}/settings/interest
Authorization: Bearer <token>
```

Update late-payment interest settings with the daily rate:

```json
{
  "rate": 0.0005
}
```

### Calculations

```http
GET /tenants/{tenantId}/invoices/overdue-with-interest
GET /tenants/{tenantId}/invoices/{invoiceId}/interest
GET /tenants/{tenantId}/invoices/{invoiceId}/interest/history
Authorization: Bearer <token>
```

The calculation response includes the invoice number, due date, days overdue, outstanding amount, daily interest, total interest, total with interest, and currency.

---

## Period Close and Year-End

### Period Close Events

```http
GET /tenants/{tenantId}/period-close-events?limit=20
POST /tenants/{tenantId}/period-close
POST /tenants/{tenantId}/period-reopen
Authorization: Bearer <token>
```

Close or reopen request:

```json
{
  "period_end_date": "2026-03-31",
  "note": "March close",
  "reviewer_sign_off": false
}
```

Fiscal-year close requests require `reviewer_sign_off=true` and approved `close_pack` evidence attached to the `year_end_close` entity ID returned by year-end status or pack. Reopen requests require a note. The API rejects fiscal-year reopen requests after year-end carry-forward has been posted.

### Year-End Carry-Forward

```http
GET /tenants/{tenantId}/year-end-close-status?period_end_date=2025-12-31
GET /tenants/{tenantId}/year-end-close-pack?period_end_date=2025-12-31
GET /tenants/{tenantId}/year-end-close-audit-evidence?period_end_date=2025-12-31
GET /tenants/{tenantId}/year-end-close-audit-archive?period_end_date=2025-12-31
POST /tenants/{tenantId}/year-end-carry-forward
POST /tenants/{tenantId}/year-end-carry-forward/reverse
Authorization: Bearer <token>
```

The close pack returns the readiness status plus year-end trial balance, balance sheet, and income statement for the selected fiscal year. The audit evidence endpoint adds the close-pack evidence-policy result and attached close-pack document metadata. The audit archive endpoint returns that evidence as a ZIP with a JSON manifest and attached close-pack files. Year-end carry-forward requires the fiscal year to be closed and the same approved close-pack evidence to be present.

Create carry-forward:

```json
{
  "period_end_date": "2025-12-31"
}
```

Reverse carry-forward:

```json
{
  "period_end_date": "2025-12-31",
  "reason": "Late supplier accrual"
}
```

---

## Banking

### Bank Accounts

```http
GET /tenants/{tenantId}/bank-accounts?active_only=true
POST /tenants/{tenantId}/bank-accounts
POST /tenants/{tenantId}/bank-accounts/import
GET /tenants/{tenantId}/bank-accounts/{accountId}
PUT /tenants/{tenantId}/bank-accounts/{accountId}
DELETE /tenants/{tenantId}/bank-accounts/{accountId}
Authorization: Bearer <token>
```

Create a bank account:

```json
{
  "name": "Main bank",
  "account_number": "EE471000001020145685",
  "bank_name": "LHV",
  "swift_code": "LHVBEE22",
  "currency": "EUR",
  "gl_account_id": "uuid",
  "is_default": true
}
```

Update supports `name`, `bank_name`, `swift_code`, `gl_account_id`, `is_active`, and `is_default`.

Import bank account master data:

```json
{
  "file_name": "bank-accounts.csv",
  "skip_duplicates": true,
  "rows": [
    {
      "name": "Main bank",
      "account_number": "EE471000001020145685",
      "bank_name": "LHV",
      "swift_code": "LHVBEE22",
      "currency": "EUR",
      "gl_account_code": "1000",
      "is_default": "true",
      "is_active": "true"
    }
  ]
}
```

Bank account imports require `name` and `account_number`, can link the cash/ledger account by `gl_account_id` or `gl_account_code`, skip duplicate account numbers when `skip_duplicates` is true, and report invalid or duplicate rows without creating placeholder accounts.

### Bank Auto-Match Rules

```http
GET /tenants/{tenantId}/bank-match-rules?bank_account_id={accountId}&active_only=true&include_global=true
POST /tenants/{tenantId}/bank-match-rules
GET /tenants/{tenantId}/bank-match-rules/{ruleId}
PUT /tenants/{tenantId}/bank-match-rules/{ruleId}
DELETE /tenants/{tenantId}/bank-match-rules/{ruleId}
Authorization: Bearer <token>
```

Create a rule:

```json
{
  "bank_account_id": "uuid",
  "name": "Stripe receipts",
  "priority": 10,
  "match_field": "DESCRIPTION",
  "pattern": "stripe",
  "min_confidence": 0.85,
  "max_date_diff_days": 3,
  "require_exact_amount": true,
  "is_active": true
}
```

Rules can be bank-account scoped or tenant-wide by omitting `bank_account_id`. `match_field` supports `DESCRIPTION`, `REFERENCE`, `COUNTERPARTY_NAME`, and `COUNTERPARTY_ACCOUNT`. `priority` runs lowest first, `min_confidence` can only raise the CLI/API auto-match threshold for matching transactions, `max_date_diff_days` narrows the payment date window, and `require_exact_amount` filters candidate payments before scoring.

Update supports `bank_account_id`, `clear_bank_account`, `name`, `priority`, `match_field`, `pattern`, `min_confidence`, `max_date_diff_days`, `require_exact_amount`, and `is_active`.

### Bank Transactions

```http
GET /tenants/{tenantId}/bank-accounts/{accountId}/transactions?status=UNMATCHED&from_date=2026-03-01&to_date=2026-03-31
POST /tenants/{tenantId}/bank-accounts/{accountId}/import
GET /tenants/{tenantId}/bank-accounts/{accountId}/import-history
GET /tenants/{tenantId}/bank-transactions/{transactionId}
GET /tenants/{tenantId}/bank-transactions/{transactionId}/suggestions
POST /tenants/{tenantId}/bank-transactions/{transactionId}/match
POST /tenants/{tenantId}/bank-transactions/{transactionId}/unmatch
POST /tenants/{tenantId}/bank-transactions/{transactionId}/review
POST /tenants/{tenantId}/bank-transactions/{transactionId}/create-payment
POST /tenants/{tenantId}/bank-accounts/{accountId}/auto-match?min_confidence=0.70
Authorization: Bearer <token>
```

Transaction statuses are `UNMATCHED`, `MATCHED`, and `RECONCILED`.

Import raw statement content with the shared bank mappers:

```json
{
  "file_name": "lhv-bank.csv",
  "format": "lhv",
  "csv_content": "Client account;Document number;Date;Beneficiary's/remitter's account;Beneficiary's/remitter's name;Debit/Credit (D/C);Amount;Reference number;Archival ID;Details;Currency;Personal identification code or registry code;Beneficiary's/remitter's bank's BIC;Payment initiator's name;Entry reference;Account service provider's reference\nEE457700771000676899;123;2026-03-15;EE111;Acme;C;100,00;REF-1;202603150001;Client payment;EUR;12345678;LHVBEE22;;ENTRY-1;ext-1\n",
  "skip_duplicates": true
}
```

`format` supports `auto`, `generic`, `lhv`, and `lhv-camt`. `auto` detects LHV Internet Bank CSV and LHV Connect camt.053 XML before falling back to generic headers. The LHV CSV mapper follows the 2026 Internet Bank account statement columns documented by LHV. The LHV camt.053 mapper is covered by the current LHV Connect Account Statement `Statement data` sample. LHV and generic mappers preserve statement account and currency metadata when present; import rejects rows whose `source_account` or `currency` does not match the selected bank account.

Pre-parsed transaction rows are also supported for clients that normalize statements before calling the API:

```json
{
  "file_name": "bank.csv",
  "transactions": [
    {
      "date": "2026-03-15",
      "value_date": "2026-03-16",
      "amount": "100.00",
      "currency": "EUR",
      "source_account": "EE457700771000676899",
      "description": "Client payment",
      "reference": "REF-1",
      "counterparty_name": "Acme",
      "counterparty_account": "EE111",
      "external_id": "bank-ext-1"
    }
  ],
  "skip_duplicates": true
}
```

Match a transaction to a payment:

```json
{
  "payment_id": "uuid"
}
```

Review an unmatched transaction:

```json
{
  "follow_up_status": "EVIDENCE_REQUIRED",
  "review_note": "Request signed receipt from the client"
}
```

- at least one of `follow_up_status` or `review_note` is required
- `follow_up_status` supports `NONE`, `EVIDENCE_REQUIRED`, and `READY_TO_MATCH`
- successful updates stamp `reviewed_by` and `reviewed_at` on the transaction
- transaction reads and lists return review metadata alongside match and reconciliation ids

### Bank Reconciliations

```http
GET /tenants/{tenantId}/bank-accounts/{accountId}/reconciliations
POST /tenants/{tenantId}/bank-accounts/{accountId}/reconciliation
GET /tenants/{tenantId}/reconciliations/{reconciliationId}
POST /tenants/{tenantId}/reconciliations/{reconciliationId}/complete
Authorization: Bearer <token>
```

Create a reconciliation:

```json
{
  "statement_date": "2026-03-31",
  "opening_balance": "0.00",
  "closing_balance": "100.00"
}
```

Reconciliation statuses are `IN_PROGRESS` and `COMPLETED`. Completing a reconciliation checks document evidence for matched transactions that an accountant has marked `EVIDENCE_REQUIRED`. Each flagged transaction must have at least one approved `reconciliation_evidence` document attached as a `bank_transaction` document, otherwise completion returns `409 Conflict`.

---

## Plugins

### Admin Registries

```http
GET /admin/plugin-registries
POST /admin/plugin-registries
DELETE /admin/plugin-registries/{id}
POST /admin/plugin-registries/{id}/sync
Authorization: Bearer <token>
```

`POST /admin/plugin-registries` accepts `name`, `url`, and optional `description`.

### Admin Plugins

```http
GET /admin/plugins
GET /admin/plugins/search?q=vat
GET /admin/plugins/permissions
POST /admin/plugins/install
GET /admin/plugins/{id}
POST /admin/plugins/{id}/enable
POST /admin/plugins/{id}/disable
DELETE /admin/plugins/{id}
Authorization: Bearer <token>
```

Install requests accept `repository_url`. Enable requests accept `granted_permissions`.

### Tenant Plugins

```http
GET /tenants/{tenantId}/plugins
POST /tenants/{tenantId}/plugins/{pluginId}/enable
POST /tenants/{tenantId}/plugins/{pluginId}/disable
GET /tenants/{tenantId}/plugins/{pluginId}/settings
PUT /tenants/{tenantId}/plugins/{pluginId}/settings
Authorization: Bearer <token>
```

Tenant plugin enable and settings update requests accept arbitrary plugin-specific JSON settings.

---

## User Management

### Invite User

```http
POST /tenants/{tenantId}/invitations
Authorization: Bearer <token>
Content-Type: application/json

{
  "email": "newuser@example.com",
  "role": "accountant"
}
```

**Roles:** `admin`, `accountant`, `viewer`

### List Invitations

```http
GET /tenants/{tenantId}/invitations
Authorization: Bearer <token>
```

### Revoke Invitation

```http
DELETE /tenants/{tenantId}/invitations/{invitationId}
Authorization: Bearer <token>
```

### Get Invitation (Public)

```http
GET /invitations/{token}
```

### Accept Invitation (Public)

```http
POST /invitations/accept
Content-Type: application/json

{
  "token": "invitation-token",
  "password": "newpassword",  // Required for new users
  "name": "New User"          // Required for new users
}
```

### List Tenant Users

```http
GET /tenants/{tenantId}/users
Authorization: Bearer <token>
```

### List Tenant Audit Events

```http
GET /tenants/{tenantId}/audit-events?limit=50
Authorization: Bearer <token>
```

Requires an owner or admin role. Returns recent tenant administration audit events for user role changes, user status changes, user session/API-token revocations, user removals, invitation creation, and invitation revocation. `limit` defaults to `50` and must be between `1` and `200`.

### List Tenant User Sessions

```http
GET /tenants/{tenantId}/users/{userId}/sessions
Authorization: Bearer <token>
```

Pass `include_inactive=true` to include revoked and expired refresh-token sessions. Requires an owner or admin role, and the target user must belong to the tenant.

### List Tenant User API Tokens

```http
GET /tenants/{tenantId}/users/{userId}/api-tokens
Authorization: Bearer <token>
```

Requires an owner or admin role. Returns active API tokens owned by the target tenant user.

### Revoke Tenant User API Token

```http
DELETE /tenants/{tenantId}/users/{userId}/api-tokens/{tokenId}
Authorization: Bearer <token>
```

Requires an owner or admin role. Revokes one active API token owned by the target tenant user and records auth and tenant audit events.

### List Tenant User Security Events

```http
GET /tenants/{tenantId}/users/{userId}/security-events?limit=50
Authorization: Bearer <token>
```

Requires an owner or admin role. Returns recent auth security events where the target tenant user is actor or target. `limit` defaults to `50` and must be between `1` and `200`.

### Update Tenant User Status

```http
PUT /tenants/{tenantId}/users/{userId}/status
Authorization: Bearer <token>
Content-Type: application/json

{
  "is_active": false
}
```

Requires an owner or admin role. Users cannot suspend or restore themselves, and owner memberships cannot be suspended. Suspended users cannot log in, refresh tokens, use existing tenant-scoped access tokens, or validate tenant-scoped API tokens for that tenant; suspending a user revokes active refresh sessions and records tenant and auth security audit events.

### Revoke Tenant User Session

```http
DELETE /tenants/{tenantId}/users/{userId}/sessions/{sessionId}
Authorization: Bearer <token>
```

Requires an owner or admin role. The revocation is recorded in both auth security events and tenant audit events.

### Revoke All Tenant User Sessions

```http
DELETE /tenants/{tenantId}/users/{userId}/sessions
Authorization: Bearer <token>
```

Requires an owner or admin role. Revokes every active refresh-token session for the target tenant user and records auth and tenant audit events.

### Remove Tenant User

```http
DELETE /tenants/{tenantId}/users/{userId}
Authorization: Bearer <token>
```

Requires an owner or admin role. Users cannot remove themselves, and owner memberships cannot be removed through this endpoint.

### Update User Role

```http
PUT /tenants/{tenantId}/users/{userId}/role
Authorization: Bearer <token>
Content-Type: application/json

{
  "role": "admin"
}
```

Requires an owner or admin role. Assignable roles are `admin`, `accountant`, and `viewer`; `owner` is creation-only and cannot be granted with this endpoint. Users cannot update their own role.

---

## Reports

### Trial Balance

```http
GET /tenants/{tenantId}/reports/trial-balance
Authorization: Bearer <token>
```

**Query Parameters:**

- `as_of_date` (string): Date in YYYY-MM-DD format
- `format` (string): `json` (default), `csv`, `xlsx`, or `pdf`

### Account Balance

```http
GET /tenants/{tenantId}/reports/account-balance/{accountId}
Authorization: Bearer <token>
```

**Query Parameters:**

- `as_of_date` (string): Date in YYYY-MM-DD format
- `format` (string): `json` (default), `csv`, `xlsx`, or `pdf`

### Balance Sheet

```http
GET /tenants/{tenantId}/reports/balance-sheet
Authorization: Bearer <token>
```

**Query Parameters:**

- `as_of` (string): Date in YYYY-MM-DD format
- `format` (string): `json` (default), `csv`, `xlsx`, or `pdf`

### Income Statement

```http
GET /tenants/{tenantId}/reports/income-statement
Authorization: Bearer <token>
```

**Query Parameters:**

- `start` (string, required): Start date in YYYY-MM-DD format
- `end` (string, required): End date in YYYY-MM-DD format
- `format` (string): `json` (default), `csv`, `xlsx`, or `pdf`

### Consolidated Financial Report

```http
GET /tenants/{tenantId}/reports/consolidated?tenant_ids=tenant-a,tenant-b&as_of=2026-12-31&start=2026-01-01&end=2026-12-31
Authorization: Bearer <token>
```

Combines trial balance, balance sheet, and income statement totals across selected tenant IDs the caller can view. Tenant-scoped API tokens are limited to their own tenant.

**Query Parameters:**

- `tenant_ids` (string): Comma-separated tenant IDs to consolidate; defaults to `{tenantId}`
- `tenant_id` (string, repeatable): Alternative repeated tenant selector
- `as_of` or `as_of_date` (string): Balance sheet and trial balance date in YYYY-MM-DD format
- `start` (string): Income statement start date; defaults to Jan 1 of the `as_of` year
- `end` (string): Income statement end date; defaults to `as_of`

### Annual Report

```http
GET /tenants/{tenantId}/reports/annual
Authorization: Bearer <token>
```

Builds a fiscal-year annual report pack from the year-end close readiness, trial balance, balance sheet, income statement, and cash-flow statement.

**Query Parameters:**

- `period_end_date` (string, required): Fiscal year-end date in YYYY-MM-DD format
- `cash_flow_method` (string): `direct` (default) or `indirect`

### Cash Flow Statement

```http
GET /tenants/{tenantId}/reports/cash-flow
Authorization: Bearer <token>
```

**Query Parameters:**

- `start_date` (string, required): Start date in YYYY-MM-DD format
- `end_date` (string, required): End date in YYYY-MM-DD format
- `method` (string): `direct` (default) or `indirect`. Indirect operating cash flow starts with net income and adjusts for depreciation/amortization plus receivables, inventory, and payables deltas.
- `operating_accounts`, `investing_accounts`, `financing_accounts` (string): Comma-separated account codes to force into a cash-flow section for this request.
- `format` (string): `json` (default), `csv`, `xlsx`, or `pdf`

### Cash Flow Mapping

```http
GET /tenants/{tenantId}/reports/cash-flow/mapping
PUT /tenants/{tenantId}/reports/cash-flow/mapping
Authorization: Bearer <token>
```

Stores tenant-level account-code mappings under tenant settings. Request-level cash-flow query parameters still take precedence for one-off reports.

**PUT Body:**

```json
{
  "operating_account_codes": ["PREPAY"],
  "investing_account_codes": ["CAPEX-1"],
  "financing_account_codes": ["FOUNDERS"]
}
```

### Receivables Aging

```http
GET /tenants/{tenantId}/reports/aging/receivables
Authorization: Bearer <token>
```

**Query Parameters:**

- `format` (string): `json` (default), `csv`, `xlsx`, or `pdf`

### Payables Aging

```http
GET /tenants/{tenantId}/reports/aging/payables
Authorization: Bearer <token>
```

**Query Parameters:**

- `format` (string): `json` (default), `csv`, `xlsx`, or `pdf`

### Balance Confirmation Summary

```http
GET /tenants/{tenantId}/reports/balance-confirmations
Authorization: Bearer <token>
```

**Query Parameters:**

- `type` (string, required): `RECEIVABLE` or `PAYABLE`
- `as_of_date` (string, required): Date in YYYY-MM-DD format
- `format` (string): `json` (default), `csv`, `xlsx`, or `pdf`

### Contact Balance Confirmation

```http
GET /tenants/{tenantId}/reports/balance-confirmations/{contactId}
Authorization: Bearer <token>
```

**Query Parameters:**

- `type` (string, required): `RECEIVABLE` or `PAYABLE`
- `as_of_date` (string, required): Date in YYYY-MM-DD format
- `format` (string): `json` (default), `csv`, `xlsx`, or `pdf`

### Contact Statement

```http
GET /tenants/{tenantId}/reports/contact-statements/{contactId}
Authorization: Bearer <token>
```

Returns one customer or supplier statement with opening balance, period invoices, period payments, and closing balance.

**Query Parameters:**

- `type` (string, required): `RECEIVABLE` for customer statements or `PAYABLE` for supplier statements
- `start_date` (string, required): Start date in YYYY-MM-DD format
- `end_date` (string, required): End date in YYYY-MM-DD format
- `format` (string): `json` (default), `csv`, `xlsx`, or `pdf`

### Sales Margin

```http
GET /tenants/{tenantId}/reports/sales-margin
Authorization: Bearer <token>
```

Returns sales invoice line revenue, estimated product cost from product purchase prices, customer rollups, margin, and margin percent for a period.

**Query Parameters:**

- `start_date` (string, required): Start date in YYYY-MM-DD format
- `end_date` (string, required): End date in YYYY-MM-DD format
- `format` (string): `json` (default), `csv`, `xlsx`, or `pdf`

### Customer Profitability

```http
GET /tenants/{tenantId}/reports/customer-profitability
Authorization: Bearer <token>
```

Returns customer-level revenue, estimated product cost from product purchase prices, profit, profit percent, and supporting sales invoice line detail for a period.

**Query Parameters:**

- `start_date` (string, required): Start date in YYYY-MM-DD format
- `end_date` (string, required): End date in YYYY-MM-DD format
- `format` (string): `json` (default), `csv`, `xlsx`, or `pdf`

### Budget vs Actual

```http
GET /tenants/{tenantId}/reports/budget-vs-actual
Authorization: Bearer <token>
```

Returns cost-center budget, actual expense, budget-used percentage, and over-budget flags for the requested period.

**Query Parameters:**

- `start_date` (string): Start date in YYYY-MM-DD format. Defaults to the start of the current year.
- `end_date` (string): End date in YYYY-MM-DD format. Defaults to today.
- `format` (string): `json` (default), `csv`, `xlsx`, or `pdf`

---

## Estonian and EU Tax (KMD/OSS)

### List KMD Declarations

```http
GET /tenants/{tenantId}/tax/kmd
Authorization: Bearer <token>
```

Returns generated KMD declarations for the tenant.

### Generate KMD Declaration

```http
POST /tenants/{tenantId}/tax/kmd
Authorization: Bearer <token>
Content-Type: application/json

{
  "year": 2025,
  "month": 1
}
```

### Generate KMD INF Report

```http
GET /tenants/{tenantId}/tax/kmd/{year}/{month}/inf?threshold=1000
Authorization: Bearer <token>
```

Generates KMD INF A/B appendix rows from domestic VAT-bearing sales and purchase invoices whose partner-period taxable total reaches the threshold excluding VAT. The default threshold is `1000`.

### Generate EU VAT OSS Report

```http
GET /tenants/{tenantId}/tax/eu-vat/oss?year=2026&quarter=1
Authorization: Bearer <token>
```

Returns quarterly EU VAT One Stop Shop report totals grouped by destination member state and VAT rate from non-Estonian EU sales invoice lines. The report uses base-currency invoice amounts and excludes contacts with VAT numbers by default. Add `include_b2b=true` for a reconciliation view that includes VAT-registered contacts.

### Import Historical KMD Declarations

```http
POST /tenants/{tenantId}/tax/kmd/import-history
Authorization: Bearer <token>
Content-Type: application/json

{
  "file_name": "kmd-history.csv",
  "csv_content": "year,month,status,submitted_at,row_code,description,tax_base,tax_amount\n2025,12,ACCEPTED,2026-01-20,1,Taxable sales,1000.00,220.00\n2025,12,ACCEPTED,2026-01-20,4,Input VAT,363.64,80.00\n"
}
```

Imports incumbent-system KMD history without overwriting existing declaration periods. Required CSV columns are `year`, `month`, and `row_code`; `tax_base`, `tax_amount`, `status`, `submitted_at`, `description`, `total_output_vat`, and `total_input_vat` are optional migration fields.

### Export KMD to XML

```http
GET /tenants/{tenantId}/tax/kmd/{year}/{month}/xml
Authorization: Bearer <token>
```

Returns `application/xml` file compatible with Estonian e-MTA.

---

## Payroll Tax (TSD)

### List TSD Declarations

```http
GET /tenants/{tenantId}/tsd
Authorization: Bearer <token>
```

Optional query parameters:

| Parameter | Type | Description |
|-----------|------|-------------|
| `year`    | int  | Filter declarations by period year |
| `month`   | int  | Filter declarations by period month (`1`-`12`) |

### Get TSD Declaration

```http
GET /tenants/{tenantId}/tsd/{year}/{month}
Authorization: Bearer <token>
```

### Generate TSD From Payroll Run

```http
POST /tenants/{tenantId}/payroll-runs/{runId}/tsd
Authorization: Bearer <token>
```

The payroll run must be approved or paid before TSD generation.

### Export TSD to XML

```http
GET /tenants/{tenantId}/tsd/{year}/{month}/xml
Authorization: Bearer <token>
```

Returns `application/xml` for e-MTA import.

### Export TSD to CSV

```http
GET /tenants/{tenantId}/tsd/{year}/{month}/csv
Authorization: Bearer <token>
```

Returns `text/csv` for review or import tooling.

### Import Historical TSD

```http
POST /tenants/{tenantId}/tsd/import-history
Authorization: Bearer <token>
Content-Type: application/json
```

Imports incumbent-system TSD declaration history without overwriting existing declaration periods. The CSV payload and response shape are documented in the Payroll section.

### Mark TSD Submitted

```http
POST /tenants/{tenantId}/tsd/{year}/{month}/submit
Authorization: Bearer <token>
Content-Type: application/json

{
  "emta_reference": "EMTA-123"
}
```

### Mark TSD Accepted

```http
POST /tenants/{tenantId}/tsd/{year}/{month}/accept
Authorization: Bearer <token>
```

### Mark TSD Rejected

```http
POST /tenants/{tenantId}/tsd/{year}/{month}/reject
Authorization: Bearer <token>
```

---

## Webhooks

### List Supported Events

```http
GET /tenants/{tenantId}/webhooks/events
Authorization: Bearer <token>
```

Returns event names such as `invoice.created`, `payment.received`, `journal_entry.posted`, `expense.approved`, `bank_transaction.matched`, `payroll.approved`, and `webhook.test`.

### Create Webhook Endpoint

```http
POST /tenants/{tenantId}/webhooks
Authorization: Bearer <token>
Content-Type: application/json

{
  "name": "CRM notifications",
  "url": "https://crm.example.com/open-accounting/webhook",
  "events": ["invoice.created", "payment.received"],
  "secret": "shared-hmac-secret",
  "is_active": true
}
```

Endpoint responses include `secret_set` but never return the secret value. Deliveries are signed with `X-Open-Accounting-Signature: sha256=<hmac>` when a secret is configured, and include event, event ID, and tenant ID headers.

### Manage Webhook Endpoints

```http
GET /tenants/{tenantId}/webhooks?active_only=true
GET /tenants/{tenantId}/webhooks/{webhookId}
PUT /tenants/{tenantId}/webhooks/{webhookId}
DELETE /tenants/{tenantId}/webhooks/{webhookId}
```

`PUT` accepts the same mutable fields as create. Set `secret` to an empty string to clear the signing secret.

### Test And Audit Deliveries

```http
POST /tenants/{tenantId}/webhooks/{webhookId}/test
Authorization: Bearer <token>
Content-Type: application/json

{
  "event_type": "webhook.test",
  "payload": {"source": "manual-check"}
}
```

```http
GET /tenants/{tenantId}/webhooks/{webhookId}/deliveries?limit=50
Authorization: Bearer <token>
```

Delivery history records status, HTTP status code, response body excerpt, error text, event ID/type, and the request body sent.

---

## Error Responses

All errors return JSON with an `error` field:

```json
{
  "error": "Description of what went wrong"
}
```

### Common Status Codes

| Code | Meaning                              |
| ---- | ------------------------------------ |
| 400  | Bad Request - Invalid input          |
| 401  | Unauthorized - Missing/invalid token |
| 403  | Forbidden - Insufficient permissions |
| 404  | Not Found - Resource doesn't exist   |
| 500  | Internal Server Error                |
