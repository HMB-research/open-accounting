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
    "email": "finance@acme.example",
    "inventory_issue_costing_method": "WEIGHTED_AVERAGE",
    "inventory_valuation_method": "FIFO"
  }
}
```

`period_lock_date` is returned on tenant reads, but it is no longer mutable through the generic tenant settings endpoint. Use the explicit period close/reopen endpoints below so changes are audited. Inventory policy settings are `inventory_issue_costing_method` (`LOT`, `WEIGHTED_AVERAGE`, or `STANDARD_COST`) and `inventory_valuation_method` (`STANDARD_COST`, `WEIGHTED_AVERAGE`, or `FIFO`); friendly aliases such as `lot`, `weighted-average`, `standard-cost`, and `fifo` are accepted and stored canonically.

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

### List Migration Provider Presets

```http
GET /tenants/{tenantId}/migration/provider-presets
Authorization: Bearer <token>
```

Returns the supported migration CSV provider presets. Each preset includes a label, description, supported file-kind count, total preset alias count, required column groups by file kind, and sorted sample header mappings such as `konto -> code` for Merit exports. The endpoint backs `oa migration presets` and the dashboard migration workbench provider selector so operators can confirm vendor mapping coverage before validating or executing a cutover bundle.

### Validate Migration Bundle

```http
POST /tenants/{tenantId}/migration/validate
Authorization: Bearer <token>
Content-Type: application/json
```

```json
{
  "e_invoice_contact_mode": "supplier",
  "provider_preset": "generic",
  "files": [
    {
      "kind": "contacts",
      "file_name": "contacts.csv",
      "csv_content": "contact_code,name\nCUST-1,Customer One\n"
    },
    {
      "kind": "invoices",
      "file_name": "invoices.csv",
      "csv_content": "invoice_number,invoice_type,contact_code,issue_date,line_description,quantity,unit_price,vat_rate\nINV-1,SALES,CUST-1,2026-05-30,Work,1,100,22\n"
    },
    {
      "kind": "e_invoices",
      "file_name": "supplier-einvoices.xml",
      "xml_content": "<?xml version=\"1.0\" encoding=\"UTF-8\"?><E_Invoice>...</E_Invoice>"
    }
  ]
}
```

Returns a non-mutating cutover report with required-column checks, duplicate business-identifier checks, grouped-document header and preserved-ID consistency checks, row-value preflight checks, imported invoice `amount_paid` checks against computed invoice totals and explicit statuses, cross-file reference issues, payment bank-account default-currency consistency, bank-transaction source-account omitted-currency consistency, payment allocation consistency checks against imported invoice CSV and e-invoice XML totals, imported invoice `amount_paid` plus same-bundle allocations, currencies, payment direction based on invoice type, invoice status, fixed-asset source-invoice consistency for purchase invoice type, same-field supplier/contact identity, purchase-date ordering, and purchase-cost totals, and cost-allocation journal-line amount and percentage total consistency plus row-level amount/percentage agreement against historical journal debit or credit amounts and a 100 percent split limit, plus grouped `remediation_actions` for supported migration CSV files and Estonian e-invoice XML bundles. Supported `kind` values are `accounts`, `contacts`, `employees`, `expenses`, `invoices`, `e_invoices`, `payments`, `bank_accounts`, `bank_transactions`, `payroll_history`, `leave_balances`, `tsd_history`, `kmd_history`, `quotes`, `orders`, `recurring_invoices`, `cost_centers`, `cost_allocations`, `product_categories`, `warehouses`, `products`, `stock_adjustments`, `fixed_assets`, `opening_balances`, and `journal_entries`. CSV files use `csv_content`; `e_invoices` files use `xml_content`. `provider_preset` defaults to `generic`; `merit`, `smartaccounts`, and `directo` add provider-style CSV header aliases for accounts, contacts, expenses, invoices, payments, bank data, cost centers, cost allocations, product categories, warehouses, products, stock adjustments, fixed assets, opening balances, historical journals, employees, payroll history, leave balances, TSD/KMD history, and related commercial documents before the same canonical validation rules run, including provider-specific preserved journal-line ID aliases used by cost-allocation cutovers. Merit Palk-style `Month6` and similar combined period columns are expanded into canonical year/month fields during validation; SmartAccounts payroll and TSD presets also expand separate year/month headers such as `payroll_year`/`payroll_month` and `pay_period_year`/`pay_period_month` plus common exported tax amount labels. Generic bank-account validation also accepts importer-compatible account-number aliases such as `iban`, `bank_account`, `account_no`, and `account`; payment `bank_account` rows are checked against bank-account files, including currency consistency with omitted payment currency treated as EUR, when both are in the same bundle. Bank-transaction `source_account` rows are checked against bank-account files; when statement currency is supplied it must match the referenced bank account, and omitted statement currency is accepted like the importer. Payment rows that reference same-bundle invoice CSV or e-invoice XML rows by `invoice_id` or `invoice_number` are also checked for ambiguous invoice-number matches, allocation totals that exceed the imported invoice total, invoice CSV `amount_paid` plus same-bundle payment allocations that would exceed the imported invoice total after import, payment currencies that do not match the imported invoice currency, `payment_type` direction mismatches against the imported invoice type, payment dates before imported invoice issue dates, and imported invoice CSV statuses that are not allocatable. `e_invoice_invoice_type` can be set to `SALES`, `PURCHASE`, or `CREDIT_NOTE` when validation and execution planning should use an e-invoice type override instead of XML inference. Migration remediation actions include `code`, `severity`, `scope`, `owner_role`, `workspace_queue`, stable `assignment_key`, `priority`, `due_in_days`, `message`, `action`, optional file/kind/field/target-kind context, `issue_count`, optional `ui_path`, and a suggested CLI command so accountant review and cutover runbooks can assign blockers without parsing every raw issue.
When a payment allocation and its same-bundle imported invoice CSV or e-invoice XML target both provide the same contact reference field, validation also rejects mismatched contact values before allocation totals are accumulated; customer-mode sales e-invoices compare payments against the buyer party. When both sides provide parseable dates, validation also rejects payments dated before the imported invoice issue date. Same-bundle imported invoice CSV targets with `DRAFT` or `VOIDED` status reject payment allocations before allocation totals are accumulated.
Stock adjustments that reference same-bundle products by `product_code` must target tracked `GOODS` products; service products and products with `track_inventory=false` are rejected before import execution.

### Plan Migration Execution

```http
POST /tenants/{tenantId}/migration/execution-plan
Authorization: Bearer <token>
Content-Type: application/json
```

```json
{
  "provider_preset": "smartaccounts",
  "bank_transaction_account_id": "bank-account-id",
  "opening_balance_entry_date": "2026-01-01",
  "files": [
    {
      "kind": "accounts",
      "file_name": "accounts.csv",
      "csv_content": "code,name,account_type\n1000,Cash,ASSET\n"
    },
    {
      "kind": "bank_transactions",
      "file_name": "bank.csv",
      "csv_content": "date,amount\n2026-05-31,100\n"
    }
  ]
}
```

Returns the same validation report plus a deterministic `steps` array for executing validated cutover imports in dependency order. Each step includes `status`, `kind`, `file_name`, optional `depends_on`, optional `context_fields`, `api_method`, `api_path`, and `cli_command`. `READY` steps can be imported directly through the listed route or command, `NEEDS_CONTEXT` steps require extra request data such as `bank_transaction_account_id` or `opening_balance_entry_date`, and `BLOCKED` steps require fixing validation remediation actions before import. The top-level `summary` reports validation readiness, full plan readiness, ready step count, missing-context count, and blocked step count.

### Execute Migration

```http
POST /tenants/{tenantId}/migration/execute
Authorization: Bearer <token>
Content-Type: application/json
```

```json
{
  "confirm": true,
  "provider_preset": "smartaccounts",
  "bank_transaction_account_id": "bank-account-id",
  "bank_transaction_format": "auto",
  "opening_balance_entry_date": "2026-01-01",
  "files": [
    {
      "kind": "accounts",
      "file_name": "accounts.csv",
      "csv_content": "code,name,account_type\n1000,Cash,ASSET\n"
    }
  ]
}
```

Validates the same bundle, builds the same execution plan, and returns a persisted `id` plus `summary`, `plan`, `steps`, and `remediation_actions` run object. With `confirm: false`, the endpoint is non-mutating and saves planned steps with `needs_confirmation`. With `confirm: true`, all plan steps must be `READY`; otherwise the response is `409` with a saved blocked run and missing-context or remediation details. Confirmed ready runs execute the ordered imports server-side through the existing tenant-scoped import services, save running and terminal snapshots, mark each step `SUCCEEDED` or `FAILED`, and return `400` with the failed run when any import fails. When `provider_preset` is `merit`, `smartaccounts`, or `directo`, server-side execution rewrites CSV headers with the same provider-specific file-kind aliases used by preflight before handing each file to its import service; this covers inventory and fixed-asset aliases whose raw labels can differ or conflict across providers. Passing `resume_from_run` or `resume_from_run_id` marks matching previously `SUCCEEDED` steps as already complete, preserves their response payloads, sets `resumed` and `resumed_step_count`, and imports only the remaining planned steps. When `resume_from_run_id` points to a saved run with a stored execution request, the endpoint merges the saved bundle files and execution context before planning, so confirmation-ready saved dry runs can be executed from the CLI or accountant workspace without resupplying local files.

```http
GET /tenants/{tenantId}/migration/execution-runs?status=succeeded&limit=10
Authorization: Bearer <token>
```

Lists saved migration execution run snapshots for the tenant. Optional `status` filters by run status, and `limit` accepts values from 1 to 200.

```http
GET /tenants/{tenantId}/migration/execution-runs/{runId}
Authorization: Bearer <token>
```

Returns one saved migration execution run snapshot. Saved runs include the persisted run ID, tenant metadata, timestamps, summary counters, execution plan, step outcomes, response payloads, and remediation actions.

```http
GET /tenants/{tenantId}/migration/execution-runs/{runId}/events?interval_ms=1000&max_events=20
Authorization: Bearer <token>
Accept: text/event-stream
```

Streams saved migration execution run snapshots as Server-Sent Events. Each event payload is a `MigrationExecutionRunEvent` with `type`, `sequence`, and `run`; `type` is `snapshot` while the run is active and `complete` when the saved run reaches `succeeded`, `failed`, `blocked`, or `needs_confirmation`. `interval_ms` accepts values from 1 to 30000 and defaults to 1000. `max_events` accepts values from 1 to 1000 and defaults to 100.

Account row validation checks required codes/names, optional preserved `id` or `account_id` UUIDs, and importer-supported `account_type` aliases or uppercase enum values before import. Contact row validation checks required names, contact type aliases, payment terms, country-code length, and credit-limit decimal values before import. Employee master row validation checks required first/last names and start dates, importer-supported employment-type and boolean aliases, optional end-date ordering, basic-exemption and funded-pension rates, positive base salaries, and salary effective dates before import. Payroll-history row validation checks period bounds, payroll and payment statuses, payment and paid dates, required positive gross salary, non-negative tax/deduction/employer-cost amounts, duplicate employee rows inside each payroll period, and consistent status, payment date, and notes inside each payroll period before import. Leave-balance row validation checks importer-compatible year bounds, `absence_type_id` UUID syntax, duplicate employee/absence-type rows inside each year, and non-negative entitled, carryover, used, and pending day values before import. TSD-history row validation checks importer-compatible periods, declaration statuses, submitted dates, required positive gross payment, non-negative tax/pension amounts, duplicate employee rows inside each TSD period, and consistent status, submitted date, and EMTA reference inside each TSD period before import. KMD-history row validation checks importer-compatible years, months, declaration statuses, submitted dates, required row codes, duplicate row codes inside each declaration period, tax-base or tax-amount decimals, optional VAT totals, and consistent status, submitted date, output VAT, and input VAT inside each KMD period before import. Invoice CSV validation requires `invoice_type` and `due_date` because invoice imports group rows by `invoice_number` plus `invoice_type` and require explicit due dates; grouped invoice, quote, order, and recurring-invoice rows must keep header-level fields such as dates, contacts, currency, status, and template settings consistent across their line rows, and preserved invoice/quote UUIDs must not be reused by another grouped document. Commercial document row validation checks invoice/quote/order/recurring dates, due/valid-until/end-date ordering, quantities, prices, discounts, VAT rates, exchange rates, statuses, invoice VAT treatment, amount paid, recurring frequencies, recurring counters, and recurring boolean settings before import. Product-category, product, warehouse, and cost-center row validation checks category names and importer-compatible parent ordering, product names, types, prices, VAT and stock thresholds, inventory/active flags, statuses, lead times, warehouse codes/names, cost-center codes/names, budgets, budget periods, and warehouse default/active flags before import. Product, warehouse, and cost-center master imports generate new UUIDs and preserve codes, so same-bundle downstream references should use `product_code`, `warehouse_code`, or `cost_center_code`. Stock-adjustment validation checks product and warehouse identifiers, strict decimal quantity, nonzero signed adjustments, non-negative unit costs, and YYYY-MM-DD expiry dates while recognizing optional lot metadata columns including `lot_number`, `serial_number`, and `expiry_date` plus common aliases such as `batch`, `serial`, and `expiration_date`; serialized stock rows require quantity `1` or `-1`, and duplicate serial numbers for the same product are rejected in the same import file. `description` is accepted as a `reason` alias. Fixed-asset row validation checks required names, purchase dates, positive purchase costs, supported statuses, depreciation methods, useful-life months, residual values, accumulated depreciation, book-value consistency, disposal methods, and disposal proceeds before import. Cost-allocation row validation checks cost-center references, journal line UUIDs, positive allocation amounts, allocation percentages, and YYYY-MM-DD allocation dates before import; cost-allocation `cost_center_id` values are existing UUIDs while `cost_center_code` is the same-bundle lookup path, and `description` is accepted as a `notes` alias. Bank-account row validation checks required names/account numbers, optional default/active booleans, and currency code format before import.

Payment validation checks `payment_type`, payment date format, contact ID or contact identity references, positive payment and exchange-rate amounts, three-letter currency code syntax, allocation references, over-allocation, same-bundle invoice/e-invoice allocation totals, currencies, payment direction, and same-field invoice-contact consistency for imported invoice CSV and e-invoice XML allocations, and same-bundle `bank_account` references with currency consistency before import; omitted payment currency is treated as EUR for the bank-account comparison. Provider presets also canonicalize common payment currency aliases such as Merit/Directo `valuuta` and `valuuta_kood`, SmartAccounts `currency_code`, and `payment_currency` before the same currency rules run. Expense validation checks required row values, contact ID or contact identity references, expense/payment account ID or code presence, same-bundle account-code type consistency, positive amount and exchange-rate values, three-letter currency code syntax, supported statuses, receipt flags, rejected-expense reasons, and status timestamps before import. Product validation checks product account ID/code references and rejects same-bundle chart references where sales accounts are not `REVENUE`, purchase accounts are not `EXPENSE`, or inventory accounts are not `ASSET`. Recurring-invoice validation checks line account IDs and rejects same-bundle preserved account references where line accounts are not `REVENUE`. Fixed-asset validation checks asset/depreciation account ID/code references and rejects same-bundle chart references where asset accounts are not `ASSET`, depreciation expense accounts are not `EXPENSE`, or accumulated depreciation accounts are not `ASSET`. Cost-allocation validation checks same-bundle journal-line references, rejects allocation totals that exceed the referenced historical journal line debit or credit amount, rejects allocation percentage totals above 100 percent for one journal line, and rejects rows where `amount` does not match the supplied `allocation_percentage` of the journal line amount. Opening-balance validation requires account codes plus both debit and credit columns, rejects negative, zero, or dual-sided rows, checks the file's debit and credit totals balance, and normalizes provider amount aliases such as `deebetsumma`, `kreeditsumma`, `algsaldo_deebet`, `algsaldo_kreedit`, `opening_debit`, and `opening_credit`. Historical journal validation recognizes importer and provider aliases including voucher number, posting date, optional preserved journal line IDs, debit/credit amount, currency, and exchange rate, then checks `source_id` and `line_id` UUID syntax, account codes, grouped entry references for one date, at least two lines, nonzero amounts, and balanced base-currency totals. Bank-transaction validation checks transaction dates and decimal amounts before import, while allowing negative statement amounts for outflows. Account validation accepts optional `id` or `account_id` UUIDs for preserved cutover account IDs. Contact validation accepts optional `id` or `contact_id` UUIDs for preserved cutover IDs. Invoice and quote validation accepts optional preserved UUIDs, requires them to be valid where supplied, and rejects reuse across different grouped documents. `e_invoice_contact_mode` controls which XML party is checked against same-bundle contacts: `supplier` validates the seller party and is the default for purchase/supplier bill cutovers, `customer` validates the buyer party for outbound sales e-invoice cutovers, and `both` validates both parties for stricter mixed bundles. `e_invoice_invoice_type` controls the effective XML invoice type used by validation and execution planning when operators intentionally import outbound sales or credit e-invoice files.

Cost-allocation validation checks required `journal_entry_line_id`, `amount`, `allocation_date`, and `cost_center_id` or `cost_center_code` columns, `journal_entry_line_id` and `cost_center_id` UUID syntax, same-bundle cost-center-code references when a cost center file is included, and preserved journal line references when a historical journal file supplies line IDs in the same bundle. Product validation also checks category IDs as UUIDs/preserved product category IDs or category names as names, sale, purchase, and inventory account ID fields as UUIDs/preserved account IDs, account-code fields as account codes with same-bundle `REVENUE`/`EXPENSE`/`ASSET` type consistency, and `supplier_id` or supplier identity fields (`supplier_code`, `supplier_reg_code`, `supplier_vat_number`, `supplier_email`, or `supplier_name`) against contacts when those related files are included. Commercial document reference validation checks quote, order, and recurring-invoice contact IDs, line `product_id` values as UUIDs, and line `product_code` values against product files when both are included; recurring-invoice line `account_id` values are validated as UUIDs and against same-bundle preserved account IDs when an accounts file is included, and same-bundle account rows must resolve to `REVENUE` accounts. Order validation checks `quote_id` values against same-bundle quote IDs when a quotes file is included. Fixed-asset validation checks supplier IDs or supplier identity fields against contacts, invoice IDs against invoice files, fixed-asset source invoices as purchase invoices with matching supplier/contact identity where both rows provide the same field, source-invoice purchase dates not before imported invoice issue dates, source-invoice purchase-cost totals not exceeding imported invoice totals, and asset, depreciation expense, and accumulated depreciation account ID fields as UUIDs/preserved account IDs or account-code fields as account codes with same-bundle `ASSET`/`EXPENSE`/`ASSET` type consistency when account files are included. Opening-balance validation requires account codes plus both debit and credit columns, rejects negative, zero, or dual-sided rows, checks the file's debit and credit totals balance, and accepts provider opening-balance account/amount aliases including Merit/Directo `konto_kood`, `deebetsumma`, `kreeditsumma`, `algsaldo_deebet`, `algsaldo_kreedit`, and SmartAccounts `gl_account_no`, `opening_debit`, `opening_credit`. Historical journal validation accepts importer and provider aliases such as voucher number, posting date, optional preserved `line_id`/`journal_entry_line_id`, Merit `kanne_rea_id`/`kande_rea_id`, SmartAccounts `entry_line_id`/`transaction_line_id`, debit/credit amount, currency, and exchange rate, then checks `source_id` and `line_id` UUID syntax, account codes, and that each entry reference has one date, at least two lines, nonzero amounts, and balanced base-currency totals.

When the related files are present in the same bundle, the validator also checks references such as commercial documents by contact ID/code/registry/VAT/email/name, e-invoice seller and/or buyer parties to contacts according to `e_invoice_contact_mode`, payments to contacts by ID/code/registry/VAT/email/name and to CSV or XML invoices by ID or number with allocation total, currency, payment-direction consistency, payment-date ordering against invoice issue dates, and same-field imported-invoice/e-invoice contact consistency, expenses to contacts by ID/code/registry/VAT/email/name plus expense/payment accounts and account types, employee master rows for importer-compatible dates, employment settings, salary values, and tax settings, payroll-history rows for importer-compatible periods, statuses, dates, amounts, and duplicate employee-period keys, leave-balance rows for importer-compatible years, day totals, and duplicate employee/year/absence-type keys, TSD-history rows for importer-compatible periods, statuses, dates, amounts, and duplicate employee-period keys, KMD-history rows for importer-compatible periods, statuses, dates, amounts, same-period VAT totals, and duplicate period row codes, payroll/leave/TSD rows to employees by employee number, personal code, email, full name, or complete first and last name, account parent codes, and bank-account ledger account ID fields as preserved account UUIDs and ledger account-code fields as account codes, bank-account master rows for required names/account numbers, currency codes, and optional boolean flags, bank-statement source accounts and currencies to bank accounts, order quote IDs to imported quote IDs, fixed-asset supplier identity fields to contacts, fixed-asset invoice IDs to imported invoices, fixed-asset source-invoice purchase type, same-field supplier/contact consistency, purchase-date ordering, and purchase-cost totals, fixed-asset account ID/code references to accounts plus asset/depreciation account-type consistency, product category IDs as preserved category UUIDs or product category names as names, product account ID/code references to accounts plus sales/purchase/inventory account-type consistency, product supplier identity fields to contacts, stock `product_id`/`warehouse_id` values as UUIDs and stock product/warehouse codes against same-bundle product and warehouse files, cost centers to parent cost centers, cost allocations to cost centers, product category parent IDs as preserved category UUIDs or parent names as names, and opening balances or journals to accounts. Hierarchy files also reject self-parent rows before import.

Bank-account GL account preflight rejects same-bundle `gl_account_id` or `gl_account_code` references where the linked cash/ledger account is not an `ASSET` account.

### List Recent Journal Entries

```http
GET /tenants/{tenantId}/journal-entries?limit=50
Authorization: Bearer <token>
```

- returns the most recent journal entries with their lines
- `limit` defaults to `50` and is capped at `200`

### Document Attachments

Document attachments currently support `invoice`, `journal_entry`, `payment`, `bank_transaction`, `asset`, `expense`, `quote`, `order`, `year_end_close`, `leave_record`, `tsd_declaration`, and `kmd_declaration` entities.

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
- supported `entity_type` values currently include `invoice`, `journal_entry`, `payment`, `bank_transaction`, `asset`, `expense`, `quote`, `order`, `year_end_close`, `leave_record`, `tsd_declaration`, and `kmd_declaration`
- supported `document_type` values currently include `supporting_document`, `receipt`, `reconciliation_evidence`, `contract`, `asset_record`, `tax_support`, `close_pack`, and `other`
- uploads start in `PENDING` review status and can carry optional retention metadata
- set either `retention_until=YYYY-MM-DD` or `retention_years=N` up to `100`; `retention_years` derives `retention_until` from the upload date, and the two fields cannot be combined
- pass `replaces_document_id=<documentId>` with an optional `replacement_note` when uploading corrected evidence; the replaced document is retained for audit and marked `SUPERSEDED`

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

Returns one result per requested entity ID with `compliant`, document-status counts, document-type counts, rule-level accepted counts, and `violations` for missing or unapproved evidence. Non-compliant results also include `remediation_actions` with stable document evidence codes such as `document_evidence_missing`, `document_evidence_unapproved`, and `document_evidence_policy_violation`, plus severity, owner role, workspace queue, stable assignment key, priority, due window, entity/document context, UI path, and suggested CLI command fields. Omit `document_types` in a rule to allow any supported document type; `min_count` defaults to `1`.

#### Retention Review

```http
GET /tenants/{tenantId}/documents/retention?as_of=2027-03-01&horizon_days=45&include_missing=true
Authorization: Bearer <token>
```

Returns tenant-wide retention administration data for documents with `retention_until` on or before the cutoff date. `include_missing=true` also includes documents without retention metadata. The response includes expired, due-soon, missing-retention, pending-review, rejected, and total counts plus the matching documents. It also includes `reminder_actions`, an automation-friendly queue with one action per expired retention date, due-soon retention date, missing retention date, pending review, or rejected document. `remediation_actions` exposes the same retention and review follow-up as accountant-assigned actions with codes such as `document_retention_expired`, `document_retention_due_soon`, `document_retention_missing`, `document_review_pending`, and `document_review_rejected`, including severity, owner role, workspace queue, stable assignment key, priority, due window, UI path, and CLI command fields.

#### Purge Expired Disposed Documents

```http
POST /tenants/{tenantId}/documents/purge
Authorization: Bearer <token>
Content-Type: application/json

{
  "as_of": "2027-03-01",
  "dry_run": true,
  "limit": 100
}
```

Runs the retention purge planner. `dry_run` defaults to `true` when omitted. Only documents whose `retention_until` is on or before `as_of`, whose lifecycle is `DISPOSED`, and which are not under legal hold are eligible. Dry-run responses list eligible and skipped candidates without deleting files. Executed purges remove the stored file and document row through the same delete path as single-document deletion; documents under legal hold or not yet disposed are reported with `skip_reason` values such as `legal_hold` and `not_disposed`.

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

#### Update Lifecycle Status

```http
PATCH /tenants/{tenantId}/documents/{documentId}/lifecycle
Authorization: Bearer <token>
Content-Type: application/json

{
  "lifecycle_status": "ARCHIVED",
  "lifecycle_note": "Retention reviewed and archived for audit"
}
```

Lifecycle statuses are `ACTIVE`, `SUPERSEDED`, `ARCHIVED`, and `DISPOSED`. `ARCHIVED` and `DISPOSED` require `lifecycle_note`, and `SUPERSEDED` requires `superseded_by_document_id` pointing at a replacement document for the same entity and document type. Superseded and disposed documents remain listed for audit but no longer satisfy evidence-policy counts.

#### Update Legal Hold

```http
PATCH /tenants/{tenantId}/documents/{documentId}/legal-hold
Authorization: Bearer <token>
Content-Type: application/json

{
  "legal_hold": true,
  "note": "Litigation hold for supplier dispute"
}
```

Set `legal_hold` to `false` with a release note to remove the hold. Active legal hold blocks disposal lifecycle changes, replacement supersession, and hard deletion while preserving the document in audit and evidence-policy views.

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
Account `parent_id` values are optional on create and update, but must be valid UUIDs when supplied.

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
  "csv_content": "invoice_id,invoice_number,invoice_type,contact_code,issue_date,due_date,status,amount_paid,reference,notes,line_description,quantity,unit,unit_price,discount_percent,vat_rate,vat_treatment,product_code\n11111111-1111-1111-1111-111111111111,INV-EXT-001,SALES,CUST-001,2026-02-01,2026-02-15,SENT,0,PO-12345,Imported migration invoice,Implementation work,1,hour,100.00,0,22,standard,SERV-001\n22222222-2222-2222-2222-222222222222,BILL-RC-001,PURCHASE,SUP-001,2026-02-01,2026-02-15,SENT,0,,Reverse-charge supplier invoice,EU service,1,hour,100.00,0,22,reverse_charge,EU-SERV"
}
```

Rows are grouped by `invoice_number` and `invoice_type`. Contacts are resolved by the first populated contact identifier in this priority order:

- `contact_code`
- `contact_reg_code`
- `contact_email`
- `contact_name`

Invoice line product references may use `product_id` or `product_code`; `product_id` values must be valid UUIDs, and `sku` and `item_code` are accepted as `product_code` aliases.

Invoice imports accept an optional valid UUID in `id` or `invoice_id`. Supplied invoice IDs are preserved, must be consistent across grouped rows, and are skipped when the ID already exists. Preserved IDs let payment imports and migration preflight target imported invoices by `invoice_id`.

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

Supported header aliases include `code` / `account_code` / `number`, `name` / `account_name`, `account_type` / `type` / `category`, and `parent_code` / `parent` / `parent_account`.

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

**Note:** Debits must equal credits in base currency. Line `currency` defaults to `EUR`, omitted or zero `exchange_rate` defaults to `1`, and non-zero exchange rates must be positive. Optional `source_id` values must be valid UUIDs. When `requires_evidence` is true, posting is blocked until the journal entry has at least one approved `supporting_document`, `receipt`, or `tax_support` document attached.

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

The import creates a journal entry and posts it immediately. It accepts the same account/debit/credit provider aliases validated by migration preflight, including Merit and Directo `konto_kood`, `deebetsumma`, `kreeditsumma`, `algsaldo_deebet`, and `algsaldo_kreedit`, plus SmartAccounts `gl_account_no`, `opening_debit`, and `opening_credit`, so provider-preset execution can pass through the original opening-balance CSV. If the tenant period is locked for the chosen date, the API returns `409 Conflict`.

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

Rows are grouped by `entry_reference`; each group must have one `entry_date`, at least two lines, known `account_code` values, and balanced debit/credit totals. The import endpoint accepts the same Merit, SmartAccounts, and Directo historical-journal provider aliases validated by migration preflight, including `kanne_nr`, `kuupaev`, `kanne_rea_id`, `entry_no`, `transaction_date`, `entry_line_id`, `account_no`, `number`, `rea_id`, `konto`, `deebet`, `kreedit`, `valuuta`, and `kurss`, so provider-preset execution can pass through the original historical-journal CSV. Locked-period groups are skipped with row errors in the import result.

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
Contact create and update requests may set `default_account_id`; when supplied it must be a valid UUID.

### Import Contacts

```http
POST /tenants/{tenantId}/contacts/import
Authorization: Bearer <token>
Content-Type: application/json

{
  "file_name": "contacts.csv",
  "csv_content": "contact_id,name,type,email,payment_terms_days\n11111111-1111-1111-1111-111111111111,Northwind OU,CUSTOMER,ap@northwind.example,14\n,Supply Partner,SUPPLIER,purchases@supply.example,30\n"
}
```

Supported header aliases include `id` / `contact_id`, `name` / `company` / `company_name`, `type` / `role`, `payment_terms_days` / `payment_days` / `terms_days`, `country` / `country_code`, `vat_number` / `vat` / `vat_no`, `phone` / `telephone`, `address` / `street` / `street_address`, and `postal_code` / `postcode` / `zip` / `zip_code`. `credit_limit` accepts comma decimals such as `1500,50` and thousands separators such as `1,500.50`. Optional imported IDs must be valid UUIDs and are preserved so cutover files can refer to contacts through `contact_id` or `supplier_id`.

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

Required columns are `first_name`, `last_name`, and `start_date`. Optional columns include `employee_number`, `personal_code`, `email`, `phone`, `address`, `bank_account`, `end_date`, `position`, `department`, `employment_type`, `apply_basic_exemption`, `basic_exemption_amount`, `funded_pension_rate`, `base_salary`, `salary_effective_from`, and `is_active`.

Supported header aliases include `number`, `employee_no`, or `employee_id` for `employee_number`; `firstname` or `given_name` for `first_name`; `lastname`, `surname`, or `family_name` for `last_name`; `isikukood` for `personal_code`; `telephone` for `phone`; `iban` for `bank_account`; `employment_start` for `start_date`; `employment_end` for `end_date`; `title` for `position`; `team` for `department`; `type` for `employment_type`; `basic_exemption` for `apply_basic_exemption`; `pension_rate` for `funded_pension_rate`; `salary` or `gross_salary` for `base_salary`; `effective_from` for `salary_effective_from`; and `active` for `is_active`.

Dates can use `YYYY-MM-DD`, RFC3339 timestamps, or `DD.MM.YYYY`. Boolean fields accept common migration values such as `true`/`false`, `yes`/`no`, `1`/`0`, and Estonian `ja`/`ei`. Decimal fields accept comma decimals such as `4500,50`. Duplicate employees are detected by employee number, personal code, email, or the same full name plus start date. If `base_salary` is provided, it must be positive; if `salary_effective_from` is omitted, the employee start date is used.

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

Payroll run responses include `remediation_actions` with `code`, `severity`, `scope`, `owner_role`, `workspace_queue`, stable `assignment_key`, `priority`, `due_in_days`, `message`, `action`, `period`, optional entity context, UI path, and CLI command fields for accountant follow-up on draft calculation, missing payment dates, zero-payslip runs, approval, TSD generation, paid-run declaration follow-up, and declared payroll archive evidence.

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

### Update Payroll Payment Date

```http
PATCH /tenants/{tenantId}/payroll-runs/{runId}/payment-date
Authorization: Bearer <token>
Content-Type: application/json

{
  "payment_date": "2026-03-31T00:00:00Z"
}
```

Sets or corrects the intended salary payment date on draft, calculated, approved, or paid payroll runs. Declared payroll runs reject payment-date changes. The returned payroll run includes refreshed remediation actions, so `payroll_payment_date_missing` clears immediately when the date is set.

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
- `name`
- `first_name` + `last_name`

Accepted payroll-history aliases include `year` or `payroll_year` for `period_year`, `month` or `payroll_month` for `period_month`, `run_status` for `status`, `pay_date` for `payment_date`, `employee_no` or `employee_id` for `employee_number`, `isikukood` for `personal_code`, `gross` for `gross_salary`, `unemployment_employee` or `unemployment_insurance_ee` for `unemployment_insurance_employee`, `pension` for `funded_pension`, `net` for `net_salary`, `unemployment_employer` or `unemployment_insurance_er` for `unemployment_insurance_employer`, and `employer_cost` for `total_employer_cost`. The SmartAccounts migration provider preset also accepts `pay_period_year`, `pay_period_month`, `payroll_status`, `paid_date`, `taxable_amount`, `employee_unemployment_amount`, `employer_unemployment_amount`, and `pension_amount`.

Supported statuses:

- `APPROVED`
- `PAID`
- `DECLARED`

`status` defaults to `PAID` when omitted. `payment_status` defaults to `PENDING` for `APPROVED` runs and `PAID` for `PAID`/`DECLARED` runs.

Dates use the same `YYYY-MM-DD`, RFC3339, or `DD.MM.YYYY` formats as employee import. Decimal fields accept comma decimals and must be zero or greater, except `gross_salary`, which must be greater than zero. `payment_status` accepts `PENDING`, `PAID`, `CANCELLED`, or `CANCELED`. If `taxable_income`, `net_salary`, or `total_employer_cost` is omitted, the importer derives it from the supplied gross salary and deduction/tax columns. Existing payroll periods are not overwritten; rows for periods that already have payroll runs are skipped and returned as row errors. Migration preflight also reports duplicate employee rows inside the same payroll period before import.

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
- one absence type identifier: `absence_type_code`, `absence_type`, or a UUID `absence_type_id`
- at least one employee identifier per row

Employee matching supports the same identifiers as historical payroll import. Absence types can be matched by code, name, Estonian name, or id. If `entitled_days` is omitted, the absence type default is used. `carryover_days`, `used_days`, and `pending_days` default to zero. Migration preflight reports duplicate employee + absence type rows inside the same year before import.

Accepted leave-balance aliases include `period_year` for `year`, `employee_no` or `employee_id` for `employee_number`, `isikukood` for `personal_code`, `absence_code`, `leave_type_code`, or `type_code` for `absence_type_code`, `absence_type_name`, `leave_type`, `leave_type_name`, or `type` for `absence_type`, `entitlement` or `annual_entitlement` for `entitled_days`, `carry_over_days` or `carried_forward_days` for `carryover_days`, `taken_days` for `used_days`, and `reserved_days` for `pending_days`. Day values accept comma decimals and must be zero or greater.

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

Employee matching supports the same identifiers as historical payroll import. Supported statuses are `DRAFT`, `SUBMITTED`, `ACCEPTED`, and `REJECTED`; `FILED` aliases to `SUBMITTED`, and `APPROVED` or `CONFIRMED` alias to `ACCEPTED`. Status defaults to `DRAFT` when omitted. Existing TSD declaration periods are skipped rather than overwritten. `payment_type` defaults to `10` when omitted.

Accepted TSD-history aliases include `declaration_year`, `tsd_year`, or `year` for `period_year`, `declaration_month`, `tsd_month`, or `month` for `period_month`, `declaration_status` for `status`, `submitted_date` or `submission_date` for `submitted_at`, `emta_ref` or `submission_reference` for `emta_reference`, `employee_no` or `employee_id` for `employee_number`, `isikukood` for `personal_code`, `e_mail` for `email`, `payment_code` or `tsd_payment_type` for `payment_type`, `gross_salary` or `gross` for `gross_payment`, `basic_exemption_applied` for `basic_exemption`, `taxable_income` for `taxable_amount`, `unemployment_employer` or `unemployment_insurance_er` for `unemployment_insurance_employer`, `unemployment_employee` or `unemployment_insurance_ee` for `unemployment_insurance_employee`, and `pension` for `funded_pension`. The SmartAccounts migration provider preset also accepts `pay_period_year`, `pay_period_month`, `filing_date`, `payment_kind`, `basic_exemption_amount`, `tax_free_amount`, `employee_unemployment_amount`, `employer_unemployment_amount`, and `pension_amount`. Dates and decimals use the same formats as payroll history; `gross_payment` must be greater than zero, and tax/pension amounts must be zero or greater. Migration preflight checks the same period, status, submitted-date, gross-payment, tax amount, duplicate employee-period keys, and same-period metadata rules before import.

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

Imports one CSV row per quote line and groups rows by `quote_number`. Required columns are `quote_number`, `quote_date`, a contact identifier (`contact_id`, `contact_code`, `contact_reg_code`, `contact_email`, or `contact_name`), `line_description`, `quantity`, `unit_price`, and `vat_rate`. Optional columns include `id` or `quote_id` for a valid UUID to preserve during cutover, `valid_until`, `status`, `currency`, `exchange_rate`, `notes`, `unit`, `discount_percent`, and `product_id` or `product_code`; direct `contact_id` and `product_id` values must be valid UUIDs, and `sku` and `item_code` are accepted as `product_code` aliases. Duplicate quote numbers or imported IDs are skipped.

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

Imports one CSV row per order line and groups rows by `order_number`. Required columns are `order_number`, `order_date`, a contact identifier (`contact_id`, `contact_code`, `contact_reg_code`, `contact_email`, or `contact_name`), `line_description`, `quantity`, `unit_price`, and `vat_rate`. Optional columns include `expected_delivery`, `status`, `currency`, `exchange_rate`, `notes`, `quote_id`, `unit`, `discount_percent`, and `product_id` or `product_code`; direct `contact_id`, `product_id`, and `quote_id` values must be valid UUIDs, and `quote_id` can point at a quote UUID preserved by a quote import `id` or `quote_id` column. `sku` and `item_code` are accepted as `product_code` aliases. Duplicate order numbers are skipped.

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

Imports one CSV row per recurring template line and groups rows by `name`. Required columns are `name`, `frequency`, `start_date`, a contact identifier (`contact_id`, `contact_code`, `contact_reg_code`, `contact_email`, or `contact_name`), `line_description`, `quantity`, `unit_price`, and `vat_rate`. Optional columns include `invoice_type`, `currency`, `end_date`, `next_generation_date`, `payment_terms_days`, `reference`, `notes`, active/generation/email settings, `unit`, `discount_percent`, `account_id`, and `product_id` or `product_code`; direct `contact_id`, `product_id`, and `account_id` values must be valid UUIDs, and `sku` and `item_code` are accepted as `product_code` aliases. In migration bundle preflight, recurring line `account_id` values can reference preserved account IDs from the same accounts file. Duplicate template names are skipped.

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

Statuses are `DRAFT`, `SUBMITTED`, `APPROVED`, `REJECTED`, and `POSTED`. Expense responses include `remediation_actions` so accountant queues can surface receipt, approval, rejection, posting, and archive follow-up without parsing status text. Each action includes `code`, `severity`, `scope`, `owner_role`, `workspace_queue`, stable `assignment_key`, `priority`, `due_in_days`, `message`, `action`, expense context, `ui_path`, and a suggested `cli_command`.

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

Expense imports create unposted expense claims and return row-level errors for invalid rows. ID columns such as `employee_id`, `contact_id`, `expense_account_id`, and `payment_account_id` must be valid UUIDs. Contact references can also use `contact_code`, `contact_reg_code`, `contact_vat_number`, `contact_email`, or `contact_name`; customer/supplier aliases map onto the same fields. Account references can also use chart codes via `expense_account_code`/`payment_account_code`. Migration preflight rejects same-bundle expense rows where `expense_account_code` does not reference an `EXPENSE` account or `payment_account_code` does not reference an `ASSET` or `LIABILITY` account. Supported imported statuses are `DRAFT`, `SUBMITTED`, `APPROVED`, and `REJECTED`; `POSTED` must be reached through the normal approval/posting workflow so ledger entries are created consistently. If the tenant period is locked, locked expense-date rows are skipped in the import result.

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

Then approve the document with `POST /tenants/{tenantId}/documents/{documentId}/review`. Expenses with `requires_receipt=true` reject approval and posting until at least one linked `receipt` document is approved. Receipt-backed draft and submitted expense responses include remediation actions such as `expense_receipt_required` and `expense_receipt_approval_required`, pointing operators to document upload or review-queue commands.

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

Lifecycle responses expose status-specific remediation actions:

- `expense_submit_for_approval` for draft claims
- `expense_approve_or_reject` for submitted claims
- `expense_post_to_ledger` for approved claims
- `expense_rejection_review` for rejected claims
- `expense_posted_archive` for posted claims

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

Required CSV columns are `name`, `purchase_date`, and `purchase_cost`. Optional columns include `asset_number`, `category_id`, `category_name`, `status`, `description`, `supplier_id`, supplier identity columns (`supplier_code`, `supplier_reg_code`, `supplier_vat_number`, `supplier_email`, `supplier_name`), `invoice_id`, serial/location fields, depreciation settings, `accumulated_depreciation`, `book_value`, `last_depreciation_date`, disposal fields, and asset/depreciation account IDs or account codes. ID columns must be valid UUIDs; account-code columns are `asset_account_code`, `depreciation_expense_account_code`, and `accumulated_depreciation_account_code`, and migration preflight rejects same-bundle references where those accounts are not `ASSET`, `EXPENSE`, and `ASSET` respectively. Supplier identity values resolve through contacts before storing the resolved supplier UUID, and invalid, missing, or ambiguous supplier references are returned as row-level errors. Omitted asset numbers are generated; supplied asset numbers are preserved and checked for duplicates.

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

Create category payloads accept `name`, optional `description`, and optional `parent_id`; direct `parent_id` values must be valid UUIDs.

```http
POST /tenants/{tenantId}/product-categories/import
Authorization: Bearer <token>
Content-Type: application/json

{
  "file_name": "categories.csv",
  "csv_content": "name,description,parent_name\nParts,Spare parts,\nFasteners,Bolts and screws,Parts\n"
}
```

Category CSV imports require `name`. Optional columns include `id`, `category_id`, or `product_category_id` to preserve UUIDs for cutover references, plus `description`, `parent_id`, `parent_category_id`, and `parent_name`; parent ID columns must be valid UUIDs, while parent names can reference existing categories or earlier rows in the same import.

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

Malformed `category_id` values return `400 Bad Request`.

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
  "sale_account_id": "uuid",
  "purchase_account_id": "uuid",
  "inventory_account_id": "uuid",
  "supplier_id": "uuid",
  "track_inventory": true
}
```

`sales_price` is required. If `code` is omitted, the service generates one. Optional `category_id`, account ID, and `supplier_id` fields must be valid UUIDs when supplied.

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

Required CSV columns are `name` and `sales_price`. Optional columns include `code`, `product_type`, `category_id`, `category_name`, `description`, `unit`, purchase/VAT/reorder prices, account IDs or account-code columns, `track_inventory`, `status` or `is_active`, `barcode`, `supplier_id`, supplier identity columns (`supplier_code`, `supplier_reg_code`, `supplier_vat_number`, `supplier_email`, `supplier_name`), and `lead_time_days`. `category_id`, direct account ID, and `supplier_id` values must be valid UUIDs for existing or already imported records; supplier identity values resolve through contacts before storing the resolved supplier UUID. Invalid or missing category IDs, malformed ID values, and unknown or ambiguous supplier references are returned as row-level errors. Account-code columns are `sale_account_code`, `purchase_account_code`, and `inventory_account_code`; migration preflight rejects same-bundle product rows where those account references resolve to non-`REVENUE`, non-`EXPENSE`, or non-`ASSET` account types respectively. Product imports generate new UUIDs rather than preserving `id` or `product_id` values. Omitted codes are generated; supplied codes are preserved, checked for duplicates, and used as the same-bundle lookup key for product lines and stock. Use inventory stock adjustment commands or APIs after product import to load opening quantities.

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

Returns tracked `GOODS` stock valuation. `method` accepts `standard-cost`, `weighted-average`, or `fifo` and overrides the tenant `inventory_valuation_method` policy, which defaults to `STANDARD_COST`. Costed methods fall back to purchase price when no usable movement costs exist. The response includes product and warehouse labels, on-hand/reserved/available quantities, line value, and report totals.

### Inventory Subledger Reconciliation

```http
GET /tenants/{tenantId}/inventory/subledger-reconciliation
GET /tenants/{tenantId}/inventory/subledger-reconciliation?warehouse_id={warehouseId}
GET /tenants/{tenantId}/inventory/subledger-reconciliation?method=weighted-average
GET /tenants/{tenantId}/inventory/subledger-reconciliation?as_of_date=2026-03-31
Authorization: Bearer <token>
```

Compares valued tracked `GOODS` stock to posted general-ledger balances by each product's configured inventory asset account. `method` uses the same `standard-cost`, `weighted-average`, or `fifo` valuation behavior as inventory valuation and falls back to the tenant `inventory_valuation_method` policy when omitted. `as_of_date` controls the GL balance cutoff. The response includes account-level subledger value, GL balance, difference, readiness, and stock-line exceptions for missing, unknown, or non-asset inventory account mappings.

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

Warehouse CSV imports require `code` and `name`. Optional columns include `address`, `is_default`, `status`, and `is_active`; `status` accepts `ACTIVE` or `INACTIVE`. Warehouse imports generate new UUIDs rather than preserving `id` or `warehouse_id` values. Supplied codes are preserved, checked for duplicates, and used as the same-bundle lookup key for stock.

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

`product_id` and `warehouse_id` must be valid UUIDs. `quantity` is signed: positive quantities add stock and negative quantities remove stock. Adjustments update both the product total stock and the selected warehouse stock level; reductions cannot drive that warehouse below zero or below reserved quantity. `lot_number`, `serial_number`, and `expiry_date` are optional movement metadata fields; `expiry_date` must use `YYYY-MM-DD`.

```http
POST /tenants/{tenantId}/inventory/stock-import
Authorization: Bearer <token>
Content-Type: application/json

{
  "file_name": "stock.csv",
  "csv_content": "product_code,warehouse_code,quantity,unit_cost,lot_number,serial_number,expiry_date,reason\nPRD-001,MAIN,1,10.50,LOT-2026-01,SN-001,2027-01-31,Opening stock\n"
}
```

Stock CSV imports require `quantity`, a product identifier (`product_id` or `product_code`), and a warehouse identifier (`warehouse_id` or `warehouse_code`). Direct `product_id` and `warehouse_id` values must be valid UUIDs. When a migration bundle includes product or warehouse master files, stock rows must use `product_code` or `warehouse_code` for same-bundle references because product and warehouse import IDs are generated. Quantities are signed adjustments; use positive quantities for opening stock or inbound counts and negative quantities for reductions. Optional lot metadata columns are `lot_number`, `serial_number`, and `expiry_date`; serialized stock rows require quantity `1` or `-1`, and duplicate serial numbers for the same product are skipped as row errors. Aliases include `lot`, `batch`, `serial`, `expiration_date`, and `description` for `reason`.

```http
POST /tenants/{tenantId}/inventory/issue
Authorization: Bearer <token>
Content-Type: application/json

{
  "product_id": "uuid",
  "warehouse_id": "uuid",
  "quantity": "2",
  "costing_method": "weighted-average",
  "lot_number": "LOT-2026-01",
  "serial_number": "SN-001",
  "expiry_date": "2027-01-31",
  "reference": "Invoice INV-001",
  "source_type": "SALES_INVOICE",
  "source_id": "uuid",
  "reason": "Shipment",
  "cost_of_goods_sold_account_id": "uuid",
  "inventory_account_id": "uuid",
  "post_to_ledger": true
}
```

`product_id`, `warehouse_id`, optional `source_id`, and optional account IDs must be valid UUIDs. Issues require a positive quantity and enough available warehouse stock. Optional lot metadata consumes only a matching tracked lot/serial/expiry position after tracked and unallocated reservations; without metadata, issue allocation consumes available tracked lots in deterministic expiry/lot/serial order before any untracked remainder. `costing_method` accepts `lot`, `weighted-average`, or `standard-cost` and overrides the tenant `inventory_issue_costing_method` policy, which defaults to `LOT`; all methods preserve the physical lot allocation while changing the unit cost applied to the outbound movement and accounting lines. The response includes costed outbound movements, the normalized costing method, weighted issue cost, the updated stock level, and optional accounting-ready lines that debit cost of goods sold and credit inventory when COGS and inventory asset accounts are available. When `post_to_ledger` is true, those lines are created and posted as a journal entry in the same database transaction as the stock issue, so a ledger posting failure rolls back the stock movements and stock-level updates; the response includes the posted journal entry ID and number. The service validates supplied COGS accounts as `EXPENSE` and inventory accounts as `ASSET` when accounting account metadata is available.

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

`product_id`, `from_warehouse_id`, and `to_warehouse_id` must be valid UUIDs. Transfers require a positive quantity and enough available stock in the source warehouse. `lot_number`, `serial_number`, and `expiry_date` are optional movement metadata fields; `expiry_date` must use `YYYY-MM-DD`. When lot metadata is supplied, the matching source lot/serial/expiry position must have enough quantity for the transfer. Successful transfers create an outbound movement for the source warehouse, an inbound movement for the destination warehouse, copy any lot metadata to both movements, carry the source lot cost when a matching cost layer exists, fall back to weighted-average or purchase price when needed, and update both warehouse stock levels without changing total product stock.

```http
POST /tenants/{tenantId}/inventory/reserve
Authorization: Bearer <token>
Content-Type: application/json

{
  "product_id": "uuid",
  "warehouse_id": "uuid",
  "quantity": "2",
  "lot_number": "LOT-2026-01",
  "expiry_date": "2027-01-31",
  "reason": "Sales order allocation"
}
```

`product_id` and `warehouse_id` must be valid UUIDs. Reservations require a positive quantity and sufficient warehouse available stock. Optional `lot_number`, `serial_number`, and `expiry_date` fields reserve against a matching tracked lot/serial/expiry position; explicit tracked-lot reservations require enough matching availability after existing tracked and unallocated reservations. When tracking metadata is omitted, the service allocates available tracked lots automatically before leaving any remainder as warehouse-level reserved stock. A successful reservation increases `reserved_qty` and decreases `available_qty` without changing on-hand quantity or product total stock.

```http
POST /tenants/{tenantId}/inventory/release
Authorization: Bearer <token>
Content-Type: application/json

{
  "product_id": "uuid",
  "warehouse_id": "uuid",
  "quantity": "1",
  "lot_number": "LOT-2026-01",
  "expiry_date": "2027-01-31",
  "reason": "Order canceled"
}
```

`product_id` and `warehouse_id` must be valid UUIDs. Releases require a positive quantity no greater than current reserved stock. Optional `lot_number`, `serial_number`, and `expiry_date` fields release the matching tracked reservation and reject over-release for that lot; releases without tracking metadata consume tracked reservation rows first and then release any remaining warehouse-level reservation. A successful release decreases `reserved_qty` and increases `available_qty` without changing on-hand quantity or product total stock.

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
  "parent_id": "cost-center-uuid",
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

Cost center create and update payloads accept optional `parent_id`, which must be a valid UUID when supplied. Cost center CSV imports require `code` and `name`. Optional columns include `description`, `parent_id`, `parent_code`, `budget_amount`, `budget_period`, `status`, and `is_active`; `parent_id` must be an existing cost-center UUID, while `parent_code` can reference existing cost centers or earlier rows in the same import. The import endpoint accepts the same Merit, SmartAccounts, and Directo provider aliases validated by migration preflight, including `kulukoha_kood`, `cost_center_no`, `department_no`, `objekt`, `objekti_kood`, `nimi`, and `ylemobjekt`, so provider-preset execution can pass through original cost-center CSVs.

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

Cost allocations assign a positive journal-entry-line amount to a cost center for budget-vs-actual and cost-center reporting. Listing supports optional `cost_center_id`, `journal_entry_line_id`, `start_date`, and `end_date` filters; ID filters must be valid UUIDs, and returned rows include joined `cost_center_code` and `cost_center_name` when available. Direct create payloads require UUID `cost_center_id` and `journal_entry_line_id` values. Cost allocation CSV imports require a UUID `journal_entry_line_id`, `amount`, `allocation_date`, and either an existing UUID in `cost_center_id` or a resolvable `cost_center_code`; optional columns include `allocation_percentage` and `notes`, with `description` accepted as a `notes` alias. The import endpoint accepts provider aliases such as `kulukoht`, `cost_center_no`, `entry_line_id`, `transaction_line_id`, `kanne_rea_id`, `rea_id`, `summa`, `allocated_amount`, `jaotuse_protsent`, `posting_date`, `kuupaev`, and `selgitus`, so provider-preset execution can pass through original allocation CSVs. Historical journal imports can preserve per-row `line_id` or `journal_entry_line_id` values so later cost-allocation rows can target those imported journal lines. Migration preflight rejects same-bundle cost-allocation amounts above the referenced historical journal line debit or credit amount, allocation percentages above 100 percent per journal line, and rows whose `amount` disagrees with `allocation_percentage` for that journal line amount. Cost-center master CSV imports generate new UUIDs and preserve codes for downstream lookup. Import responses include processed, imported, skipped, and row-level error counts.

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

Payment CSV imports require `payment_type`, `payment_date`, and `amount`. Optional columns include `payment_number`, `contact_id`, contact identity columns (`contact_code`, `contact_reg_code`, `contact_vat_number`, `contact_email`, `contact_name`), `currency`, `exchange_rate`, `payment_method`, `bank_account`, `reference`, `notes`, `invoice_id`, `invoice_number`, and `allocation_amount`. `contact_id` and direct `invoice_id` values must be valid UUIDs. `customer_id` and `supplier_id` are accepted as `contact_id`, customer/supplier code and name columns map to contact identity fields, `method` as `payment_method`, `description` as `notes`, and `invoice_no` as `invoice_number`. Omitted payment numbers are generated; supplied numbers are preserved and checked for duplicates. Contact identity values resolve through contacts before storing the resolved contact UUID. Payment allocations can target `invoice_id` directly, including UUIDs preserved by invoice import, or resolve `invoice_number` through the tenant invoice list.

### Export SEPA Payment XML

```http
POST /tenants/{tenantId}/payments/sepa-export
Authorization: Bearer <token>
Content-Type: application/json
Accept: application/xml

{
  "message_id": "MSG-20260331",
  "payment_info_id": "PMT-INF-20260331",
  "creation_date_time": "2026-03-31T12:00:00Z",
  "debtor_name": "Example OU",
  "debtor_iban": "EE382200221020145685",
  "debtor_bic": "HABAEE2X",
  "execution_date": "2026-04-01",
  "batch_booking": false,
  "charge_bearer": "SLEV",
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

Returns ISO 20022 `pain.001.001.03` SEPA credit-transfer XML for manual bank upload. The exporter validates debtor and creditor IBAN checksums, optional BIC format, positive EUR amounts, and `YYYY-MM-DD` execution dates. Optional `payment_info_id` overrides the payment-info block ID; when omitted it defaults from the message ID. Optional `creation_date_time` must be RFC3339 and is normalized to UTC. `batch_booking` defaults to `true` when omitted. `charge_bearer` defaults to `SLEV`, and `SLEV` is the only accepted value for SEPA credit transfers. It does not submit payments directly to a bank.

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

Template types are `INVOICE_SEND`, `QUOTE_SEND`, `ORDER_CONFIRM`, `PAYMENT_RECEIPT`, `OVERDUE_REMINDER`, and `DOCUMENT_RETENTION_REMINDER`. Scheduled document retention reminder delivery uses the tenant settings `email` value as the recipient and the tenant SMTP configuration for delivery, with startup-configured retry and escalation attempt thresholds.

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
  "reviewer_sign_off": false,
  "inventory_valuation_method": "fifo"
}
```

Fiscal-year close requests require `reviewer_sign_off=true`, approved `close_pack` evidence attached to the `year_end_close` entity ID returned by year-end status or pack, and an inventory costing review with no blocking exception lines when inventory is configured. `inventory_valuation_method` accepts `standard-cost`, `weighted-average`, or `fifo` for fiscal-year close review and overrides the tenant `inventory_valuation_method` policy, which defaults to `STANDARD_COST`. Reopen requests require a note. The API rejects fiscal-year reopen requests after year-end carry-forward has been posted.

### Year-End Carry-Forward

```http
GET /tenants/{tenantId}/year-end-close-status?period_end_date=2025-12-31&inventory_valuation_method=fifo
GET /tenants/{tenantId}/year-end-close-pack?period_end_date=2025-12-31&inventory_valuation_method=fifo
GET /tenants/{tenantId}/year-end-close-audit-evidence?period_end_date=2025-12-31&inventory_valuation_method=fifo
GET /tenants/{tenantId}/year-end-close-audit-archive?period_end_date=2025-12-31&inventory_valuation_method=fifo
POST /tenants/{tenantId}/year-end-carry-forward
POST /tenants/{tenantId}/year-end-carry-forward/reverse
Authorization: Bearer <token>
```

The close pack returns the readiness status plus year-end trial balance, balance sheet, and income statement for the selected fiscal year. Status, pack, audit evidence, and audit archive responses include `inventory_costing_review` when inventory is configured. The review summarizes the explicit `inventory_valuation_method` when supplied, otherwise the tenant valuation policy, plus totals, blocking exception counts for negative quantities/availability/value and missing costs, and whether inventory costing is ready; blocking exceptions make carry-forward readiness false. Year-end status responses also include `remediation_actions`, a machine-readable close plan with `code`, `severity`, `scope`, `owner_role`, `workspace_queue`, stable `assignment_key`, `priority`, `due_in_days`, `message`, `action`, and optional `entity_type`, `entity_id`, `ui_path`, and `cli_command` fields for unresolved close, evidence, ledger, and inventory blockers. The audit evidence endpoint adds the close-pack evidence-policy result and attached close-pack document metadata. The audit archive endpoint returns that evidence as a ZIP with a JSON manifest and attached close-pack files. Year-end carry-forward requires the fiscal year to be closed, the same approved close-pack evidence to be present, and inventory costing review to be ready when inventory is configured.

Create carry-forward:

```json
{
  "period_end_date": "2025-12-31",
  "inventory_valuation_method": "fifo"
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

Create and update `gl_account_id` values must be valid UUIDs. Update supports `name`, `bank_name`, `swift_code`, `gl_account_id`, `is_active`, and `is_default`.

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

Bank account imports require `name` and `account_number`, can link the cash/ledger account by UUID `gl_account_id` or by `gl_account_code`, skip duplicate account numbers when `skip_duplicates` is true, and report invalid or duplicate rows without creating placeholder accounts.

When an accounts file is included in the same migration bundle, bank account import preflight rejects linked GL accounts that are not `ASSET` accounts.

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

Rules can be bank-account scoped or tenant-wide by omitting `bank_account_id`; supplied `bank_account_id` values must be valid UUIDs. `match_field` supports `DESCRIPTION`, `REFERENCE`, `COUNTERPARTY_NAME`, and `COUNTERPARTY_ACCOUNT`. `priority` runs lowest first, `min_confidence` can only raise the CLI/API auto-match threshold for matching transactions, `max_date_diff_days` narrows the payment date window, and `require_exact_amount` filters candidate payments before scoring.

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

`format` supports `auto`, `generic`, `lhv`, `camt053`, and `lhv-camt`. `auto` detects LHV Internet Bank CSV and ISO 20022 camt.053 XML before falling back to generic headers. The LHV CSV mapper follows the 2026 Internet Bank account statement columns documented by LHV. The camt.053 mapper is covered by the current LHV Connect Account Statement `Statement data` sample, and `lhv-camt` remains accepted as an LHV compatibility alias. LHV, camt.053, and generic mappers preserve statement account and currency metadata when present; import rejects rows whose `source_account` or `currency` does not match the selected bank account.

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
- transaction reads, lists, and review responses include `remediation_actions` with stable codes such as `bank_evidence_required`, `bank_ready_to_match`, `bank_transaction_unmatched`, `bank_transaction_reconciliation_pending`, and `bank_transaction_reconciled_archive`, plus severity, owner role, workspace queue, stable assignment key, priority, due window, entity context, UI path, and suggested CLI command fields for accountant workspace follow-up

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
Admin registry and plugin endpoints require a tenant-scoped access token whose current tenant membership is `owner` or `admin`; the API rechecks current membership instead of trusting stale role claims.

### Admin Plugins

```http
GET /admin/plugins
GET /admin/plugins/search?q=vat
GET /admin/plugins/permissions
POST /admin/plugins/install
GET /admin/plugins/{id}
POST /admin/plugins/{id}/enable
POST /admin/plugins/{id}/disable
GET /admin/plugins/{id}/runtime
POST /admin/plugins/{id}/runtime/restart
DELETE /admin/plugins/{id}
Authorization: Bearer <token>
```

Install requests accept `repository_url`. Enable requests accept `granted_permissions`.
Runtime status returns lifecycle, health, crash/backoff, restart count, and last error/output fields. Runtime restart is only supported for supervised `backend.runtime: package` plugins.

### Tenant Plugins

```http
GET /tenants/{tenantId}/plugins
POST /tenants/{tenantId}/plugins/{pluginId}/enable
POST /tenants/{tenantId}/plugins/{pluginId}/disable
GET /tenants/{tenantId}/plugins/{pluginId}/settings
PUT /tenants/{tenantId}/plugins/{pluginId}/settings
GET /tenants/{tenantId}/plugins/{pluginId}/runtime/*
POST /tenants/{tenantId}/plugins/{pluginId}/runtime/*
PUT /tenants/{tenantId}/plugins/{pluginId}/runtime/*
PATCH /tenants/{tenantId}/plugins/{pluginId}/runtime/*
DELETE /tenants/{tenantId}/plugins/{pluginId}/runtime/*
Authorization: Bearer <token>
```

Tenant plugin enable and settings update requests accept arbitrary plugin-specific JSON settings.
Tenant plugin list responses include each plugin manifest. Frontend slot entries may declare safe `card`, `link`, or `action` runtime metadata with `label`, `description`, internal `path`, `badge`, and `order` fields. Backend hook and route declarations can run through an out-of-process HTTP runtime when the manifest declares `backend.runtime: http` and a loopback `backend.base_url`, or through a supervised package runtime when the manifest declares `backend.runtime: package`; tenant routes are exposed under `/api/v1/tenants/{tenantID}/plugins/{pluginID}/runtime/...` after tenant enablement.
Tenant runtime routes support `GET`, `POST`, `PUT`, `PATCH`, and `DELETE`. The API preserves the raw query string, forwards the request body to the manifest-declared handler path, strips authorization and hop-by-hop request headers before proxying, and returns the plugin runtime status code, response headers, and raw response body without wrapping successful responses in an Open Accounting envelope. Runtime route calls require normal tenant access for the current user and the plugin must be enabled for that tenant.

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

Returns generated KMD declarations for the tenant. KMD declaration responses include `remediation_actions` with `code`, `severity`, `scope`, `owner_role`, `workspace_queue`, stable `assignment_key`, `priority`, `due_in_days`, `message`, `action`, `period`, optional entity context, UI path, and CLI command fields for accountant follow-up on draft payable/refund/zero declarations, empty VAT periods, submitted declarations awaiting acceptance, missing submission timestamps, and accepted declarations that should be archived with supporting evidence.

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

The generated declaration response includes the same KMD `remediation_actions` array as other KMD declaration responses.

### Generate KMD INF Report

```http
GET /tenants/{tenantId}/tax/kmd/{year}/{month}/inf?threshold=1000
Authorization: Bearer <token>
```

Generates KMD INF A/B appendix rows from domestic VAT-bearing sales and purchase invoices whose partner-period taxable total reaches the threshold excluding VAT. The default threshold is `1000`. Report responses include `remediation_actions` with `code`, `severity`, `scope`, `owner_role`, `workspace_queue`, stable `assignment_key`, `priority`, `due_in_days`, `message`, `action`, `period`, report entity context, `ui_path`, and a suggested `cli_command` for threshold-row review or empty-report evidence retention.

### Mark KMD Submitted

```http
POST /tenants/{tenantId}/tax/kmd/{year}/{month}/submit
Authorization: Bearer <token>
```

Marks an existing KMD declaration as submitted to e-MTA and records the current server timestamp in `submitted_at`. Marking a KMD declaration submitted requires an approved `tax_support` or `supporting_document` attached to the `kmd_declaration` entity with the declaration ID as `entity_id`. Missing or pending evidence returns `409 Conflict` and leaves the declaration in draft status. Returns `{ "status": "submitted" }` when the transition succeeds.

### Mark KMD Accepted

```http
POST /tenants/{tenantId}/tax/kmd/{year}/{month}/accept
Authorization: Bearer <token>
```

Marks an existing KMD declaration as accepted by e-MTA. Marking a KMD declaration accepted requires an approved `tax_support` or `supporting_document` attached to the `kmd_declaration` entity with the declaration ID as `entity_id`. Missing or pending evidence returns `409 Conflict` and leaves the declaration status unchanged. Returns `{ "status": "accepted" }` when the transition succeeds and powers the accountant workspace KMD acceptance assignment action.

### Generate EU VAT OSS Report

```http
GET /tenants/{tenantId}/tax/eu-vat/oss?year=2026&quarter=1
Authorization: Bearer <token>
```

Returns quarterly EU VAT One Stop Shop report totals grouped by destination member state and VAT rate from non-Estonian EU sales invoice lines. The report uses base-currency invoice amounts and excludes contacts with VAT numbers by default. Add `include_b2b=true` for a reconciliation view that includes VAT-registered contacts. Report responses include `remediation_actions` with the same tax-report follow-up fields for manual OSS filing review, filing evidence retention, or empty-quarter confirmation.

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

Imports incumbent-system KMD history without overwriting existing declaration periods. Required CSV columns are `year`, `month`, and `row_code`; each row must include `tax_base` or `tax_amount`. Optional `status`, `submitted_at`, `description`, `total_output_vat`, and `total_input_vat` fields must use importer-compatible values, row codes must not repeat inside the same declaration period during migration preflight, and period-level status/submission/VAT-total values must stay consistent across rows in the same declaration period.

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

TSD declaration responses include `remediation_actions` for accountant follow-up: empty rows/totals, draft export/submission, submitted declarations awaiting acceptance, missing submission timestamps, rejected declaration review, accepted declaration archiving, and unsupported status review. Each action includes `code`, `severity`, `scope`, `owner_role`, `workspace_queue`, stable `assignment_key`, `priority`, `due_in_days`, `message`, `action`, optional entity/period context, UI path, and a suggested CLI command.

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

Marking a TSD declaration submitted requires an approved `tax_support` or `supporting_document` attached to the `tsd_declaration` entity with the declaration ID as `entity_id`. Missing or pending evidence returns `409 Conflict` and leaves the declaration in draft status.

### Mark TSD Accepted

```http
POST /tenants/{tenantId}/tsd/{year}/{month}/accept
Authorization: Bearer <token>
```

Marking a TSD declaration accepted requires an approved `tax_support` or `supporting_document` attached to the `tsd_declaration` entity with the declaration ID as `entity_id`. Missing or pending evidence returns `409 Conflict` and leaves the declaration status unchanged.

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
