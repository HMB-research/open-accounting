# API Reference

Complete API reference for Open Accounting. Interactive documentation available at `/swagger/` when running the server.

## Base URL

```
http://localhost:8080/api/v1
```

## Authentication

All endpoints (except `/auth/*`, `/invitations/*`, and demo reset/status endpoints when enabled) require a Bearer token:

```bash
curl -H "Authorization: Bearer <access_token-or-api-token>" \
     https://api.example.com/api/v1/me
```

Bearer auth supports two token types:
- JWT access tokens from `/auth/login` and `/auth/refresh`
- tenant-scoped API tokens created under `/tenants/{tenantId}/api-tokens`

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

Exchange refresh token for new access token.

```http
POST /auth/refresh
Content-Type: application/json

{
  "refresh_token": "eyJhbGc...",
  "tenant_id": "uuid"  // Optional: switch tenant context
}
```

---

## API Tokens

Tenant-scoped API tokens are intended for CLI and automation usage. They are valid only for the tenant path they were created for.

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

The raw `token` value is returned only once at creation time.

### Revoke API Token

```http
DELETE /tenants/{tenantId}/api-tokens/{tokenId}
Authorization: Bearer <jwt-or-api-token>
```

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
  "note": "Month-end checks completed"
}
```

- `period_end_date` must be `YYYY-MM-DD`
- the date must be the last day of a month
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

### Period Lock Behavior

When `settings.period_lock_date` is set, core write paths reject back-dated operations on or before the lock date with `409 Conflict`.

This currently applies to:
- journal entry create, post, and void
- invoice create and void
- payment creation
- payment creation from bank transactions
- opening-balance import

Invoice import also enforces the lock, but because it is a bulk operation, locked invoice rows are returned as row errors in the import summary instead of failing the whole request with `409 Conflict`.

### List Recent Journal Entries

```http
GET /tenants/{tenantId}/journal-entries?limit=50
Authorization: Bearer <token>
```

- returns the most recent journal entries with their lines
- `limit` defaults to `50` and is capped at `200`

### Document Attachments

Document attachments currently support `invoice`, `journal_entry`, and `payment` entities.

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
file=<binary>
```

- accepts PDFs, images, CSV files, text files, and similar supporting records
- maximum file size is `10 MB`

#### Download Document

```http
GET /tenants/{tenantId}/documents/{documentId}/download
Authorization: Bearer <token>
```

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

### Import Invoices

```http
POST /tenants/{tenantId}/invoices/import
Authorization: Bearer <token>
Content-Type: application/json

{
  "file_name": "invoices.csv",
  "csv_content": "invoice_number,invoice_type,contact_code,issue_date,due_date,status,amount_paid,reference,notes,line_description,quantity,unit,unit_price,discount_percent,vat_rate\nINV-EXT-001,SALES,CUST-001,2026-02-01,2026-02-15,SENT,0,PO-12345,Imported migration invoice,Implementation work,1,hour,100.00,0,22\nINV-EXT-001,SALES,CUST-001,2026-02-01,2026-02-15,SENT,0,PO-12345,Imported migration invoice,Support retainer,1,month,50.00,0,22"
}
```

Rows are grouped by `invoice_number` and `invoice_type`. Contacts are resolved by the first populated contact identifier in this priority order:
- `contact_code`
- `contact_reg_code`
- `contact_email`
- `contact_name`

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

### Create Journal Entry

```http
POST /tenants/{tenantId}/journal-entries
Authorization: Bearer <token>
Content-Type: application/json

{
  "entry_date": "2025-01-15",
  "description": "Office supplies purchase",
  "reference": "INV-001",
  "lines": [
    {
      "account_id": "uuid",
      "debit_amount": "100.00",
      "credit_amount": "0.00",
      "description": "Office supplies"
    },
    {
      "account_id": "uuid",
      "debit_amount": "0.00",
      "credit_amount": "100.00",
      "description": "Payment from cash"
    }
  ]
}
```

**Note:** Debits must equal credits.

### Post Journal Entry

Finalize a draft entry (makes it immutable).

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

---

## Contacts

### List Contacts

```http
GET /tenants/{tenantId}/contacts
Authorization: Bearer <token>
```

**Query Parameters:**
- `type` (string): `CUSTOMER`, `SUPPLIER`, or `BOTH`
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

## Invoices

### List Invoices

```http
GET /tenants/{tenantId}/invoices
Authorization: Bearer <token>
```

**Query Parameters:**
- `type` (string): `SALES` or `PURCHASE`
- `status` (string): `DRAFT`, `SENT`, `PAID`, `VOID`
- `contact_id` (uuid): Filter by contact

### Create Invoice

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
      "vat_rate": "22.00"
    }
  ]
}
```

### Download Invoice PDF

```http
GET /tenants/{tenantId}/invoices/{invoiceId}/pdf
Authorization: Bearer <token>
```

Returns `application/pdf` file.

---

## Payments

### Create Payment

```http
POST /tenants/{tenantId}/payments
Authorization: Bearer <token>
Content-Type: application/json

{
  "payment_type": "RECEIVED",
  "contact_id": "uuid",
  "account_id": "uuid",
  "amount": "1220.00",
  "payment_date": "2025-01-20",
  "reference": "BANK-001"
}
```

**Payment Types:** `RECEIVED`, `MADE`

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

### Update User Role

```http
PUT /tenants/{tenantId}/users/{userId}/role
Authorization: Bearer <token>
Content-Type: application/json

{
  "role": "admin"
}
```

---

## Reports

### Trial Balance

```http
GET /tenants/{tenantId}/reports/trial-balance
Authorization: Bearer <token>
```

**Query Parameters:**
- `as_of_date` (string): Date in YYYY-MM-DD format

### Account Balance

```http
GET /tenants/{tenantId}/reports/account-balance/{accountId}
Authorization: Bearer <token>
```

### Receivables Aging

```http
GET /tenants/{tenantId}/reports/aging/receivables
Authorization: Bearer <token>
```

---

## Estonian Tax (KMD)

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

### Export KMD to XML

```http
GET /tenants/{tenantId}/tax/kmd/{year}/{month}/xml
Authorization: Bearer <token>
```

Returns `application/xml` file compatible with Estonian e-MTA.

---

## Error Responses

All errors return JSON with an `error` field:

```json
{
  "error": "Description of what went wrong"
}
```

### Common Status Codes

| Code | Meaning |
|------|---------|
| 400 | Bad Request - Invalid input |
| 401 | Unauthorized - Missing/invalid token |
| 403 | Forbidden - Insufficient permissions |
| 404 | Not Found - Resource doesn't exist |
| 500 | Internal Server Error |
