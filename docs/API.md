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

Refresh tokens are accepted only by `/auth/refresh`; they cannot authorize API requests. Access tokens cannot be used as refresh tokens.

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
- reopening a fiscal year is rejected once a year-end carry-forward journal has already been posted for that year

### Year-End Close Status

```http
GET /tenants/{tenantId}/year-end-close-status?period_end_date=2025-12-31
Authorization: Bearer <token>
```

Returns the fiscal-year readiness summary for the selected date, including:

- fiscal-year start and end dates
- whether the selected date matches the fiscal year-end
- whether the tenant is currently locked through that date
- whether revenue/expense activity exists for the year
- retained-earnings account mapping
- whether a carry-forward journal already exists

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

### List Recent Journal Entries

```http
GET /tenants/{tenantId}/journal-entries?limit=50
Authorization: Bearer <token>
```

- returns the most recent journal entries with their lines
- `limit` defaults to `50` and is capped at `200`

### Document Attachments

Document attachments currently support `invoice`, `journal_entry`, `payment`, `bank_transaction`, and `asset` entities.

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
retention_until=2027-03-31
file=<binary>
```

- accepts PDFs, images, CSV files, text files, and similar supporting records
- maximum file size is `10 MB`
- supported `document_type` values currently include `supporting_document`, `receipt`, `reconciliation_evidence`, `contract`, `asset_record`, `tax_support`, and `other`
- uploads start in `PENDING` review status and can carry optional retention metadata

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

### Payroll Tax Preview

```http
POST /tenants/{tenantId}/payroll/tax-preview
Authorization: Bearer <token>
Content-Type: application/json

{
  "gross_salary": "3200.00",
  "apply_basic_exemption": true,
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

This importer records historical payroll runs and payslips only. It does not import leave balances, tax declaration submission history, accounting journal entries, or incumbent-system audit logs.

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

### Get Quote

```http
GET /tenants/{tenantId}/quotes/{quoteId}
Authorization: Bearer <token>
```

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

### Get Order

```http
GET /tenants/{tenantId}/orders/{orderId}
Authorization: Bearer <token>
```

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

Dispose payloads include `disposal_date`, `disposal_method`, optional `disposal_proceeds`, and optional `disposal_notes`.

### Depreciation

```http
POST /tenants/{tenantId}/assets/{assetId}/depreciation
GET /tenants/{tenantId}/assets/{assetId}/depreciation
Authorization: Bearer <token>
```

Recording depreciation uses the current month according to the server-side service.

---

## Inventory

### Product Categories

```http
GET /tenants/{tenantId}/product-categories
POST /tenants/{tenantId}/product-categories
GET /tenants/{tenantId}/product-categories/{categoryId}
DELETE /tenants/{tenantId}/product-categories/{categoryId}
Authorization: Bearer <token>
```

Create category payloads accept `name`, optional `description`, and optional `parent_id`.

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

### Warehouses

```http
GET /tenants/{tenantId}/warehouses?active_only=true
POST /tenants/{tenantId}/warehouses
GET /tenants/{tenantId}/warehouses/{warehouseId}
PUT /tenants/{tenantId}/warehouses/{warehouseId}
DELETE /tenants/{tenantId}/warehouses/{warehouseId}
Authorization: Bearer <token>
```

Create warehouse payloads accept `code`, `name`, optional `address`, and `is_default`. Update payloads accept `name`, optional `address`, `is_default`, and `is_active`.

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
  "reason": "Cycle count"
}
```

`quantity` is signed: positive quantities add stock and negative quantities remove stock.

```http
POST /tenants/{tenantId}/inventory/transfer
Authorization: Bearer <token>
Content-Type: application/json

{
  "product_id": "uuid",
  "from_warehouse_id": "uuid",
  "to_warehouse_id": "uuid",
  "quantity": "3",
  "notes": "Move to branch"
}
```

Transfers require a positive quantity.

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

### Get, Update, and Delete Cost Center

```http
GET /tenants/{tenantId}/cost-centers/{costCenterId}
PUT /tenants/{tenantId}/cost-centers/{costCenterId}
DELETE /tenants/{tenantId}/cost-centers/{costCenterId}
Authorization: Bearer <token>
```

### Cost Center Report

```http
GET /tenants/{tenantId}/cost-centers/report?start_date=2026-03-01&end_date=2026-03-31
Authorization: Bearer <token>
```

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

Template types are `INVOICE_SEND`, `PAYMENT_RECEIPT`, and `OVERDUE_REMINDER`.

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

Payment receipt emails use the same recipient, subject, and message fields without `attach_pdf`.

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
  "note": "March close"
}
```

Reopen requests require a note. The API rejects fiscal-year reopen requests after year-end carry-forward has been posted.

### Year-End Carry-Forward

```http
GET /tenants/{tenantId}/year-end-close-status?period_end_date=2025-12-31
POST /tenants/{tenantId}/year-end-carry-forward
POST /tenants/{tenantId}/year-end-carry-forward/reverse
Authorization: Bearer <token>
```

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

Import pre-parsed transactions:

```json
{
  "file_name": "bank.csv",
  "transactions": [
    {
      "date": "2026-03-15",
      "value_date": "2026-03-16",
      "amount": "100.00",
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

Reconciliation statuses are `IN_PROGRESS` and `COMPLETED`.

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

### Remove Tenant User

```http
DELETE /tenants/{tenantId}/users/{userId}
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
- `format` (string): `json` (default), `csv`, `xlsx`, or `pdf`

### Account Balance

```http
GET /tenants/{tenantId}/reports/account-balance/{accountId}
Authorization: Bearer <token>
```

**Query Parameters:**
- `as_of_date` (string): Date in YYYY-MM-DD format

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

### Cash Flow Statement

```http
GET /tenants/{tenantId}/reports/cash-flow
Authorization: Bearer <token>
```

**Query Parameters:**
- `start_date` (string, required): Start date in YYYY-MM-DD format
- `end_date` (string, required): End date in YYYY-MM-DD format
- `format` (string): `json` (default), `csv`, `xlsx`, or `pdf`

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

---

## Estonian Tax (KMD)

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

### Mark TSD Submitted

```http
POST /tenants/{tenantId}/tsd/{year}/{month}/submit
Authorization: Bearer <token>
Content-Type: application/json

{
  "emta_reference": "EMTA-123"
}
```

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
