# CLI Guide

The repository includes a Go CLI at `cmd/oa` for scriptable reads and mutations against the API.

The CLI uses a tenant-scoped API token for normal operation:

- `auth init` uses email/password once to bootstrap a token
- the CLI creates a tenant-scoped API token through the API
- later reads and writes use that stored API token, not the login password

## Build or run

```bash
go run ./cmd/oa help
go build -o oa ./cmd/oa
./oa help
```

## Coverage gate

The CLI package is expected to stay at 100% statement coverage. The backend CI
gate enforces this from the same coverage profile with:

```bash
make test-backend-coverage
```

Before changing `cmd/oa`, run the focused CLI-only gate:

```bash
make test-cli-coverage
```

The focused target writes `coverage-cli.out`, fails if any `cmd/oa` function is below 100.0%, and is cleaned by `make clean`.

## Operational checks

```bash
go run ./cmd/oa health --base-url http://localhost:8080
go run ./cmd/oa demo status --base-url http://localhost:8080 --secret <demo-secret> --user 1
go run ./cmd/oa demo reset --base-url http://localhost:8080 --secret <demo-secret> --user 1
```

`demo status` requires `--user 1` through `--user 4` and prints the API's raw JSON status payload. `demo reset` accepts `--user 1` through `--user 4` for a single seeded demo user; omit `--user` to reset all demo users. When `--secret` is provided, the CLI sends it as the `X-Demo-Secret` header expected by the demo endpoints.
`health` calls the public `/health` endpoint at `--base-url` and does not require a configured API token.

## Operator backup commands

These local operator commands wrap the backup scripts in `scripts/`. Run them from the repository root, or set `OA_SCRIPT_DIR` to the directory containing the scripts when using a built binary.

```bash
go run ./cmd/oa ops backup create --backup-dir ./backups --retention-days 30 --dry-run
go run ./cmd/oa ops backup health --backup-dir ./backups --max-age-hours 26 --status-file /var/lib/node_exporter/textfile_collector/openaccounting_backup.prom
go run ./cmd/oa ops backup offsite-sync --backup-dir ./backups --s3-uri s3://company-backups/open-accounting/prod --dry-run
go run ./cmd/oa ops backup restore-drill --backup ./backups/openaccounting_20260528T120000Z.dump --restore-url postgres://user:pass@localhost:5432/openaccounting_restore_drill?sslmode=disable --dry-run
```

`ops backup create` delegates to `db-backup.sh`, which requires `DATABASE_URL` unless `--database-url` is passed. `ops backup restore-drill` requires a separate disposable restore database and refuses to restore into the source URL when `DATABASE_URL` or `--source-url` matches. `ops backup offsite-sync` requires exactly one destination: `--s3-uri` or `--rclone-remote`.
The CLI trims and forwards only provided backup flags to the matching local script; `OA_SCRIPT_DIR` can point at a custom scripts directory, otherwise the CLI searches for `scripts/` from the current working tree.

## Bootstrap a token

```bash
go run ./cmd/oa auth register \
  --base-url http://localhost:8080 \
  --email you@example.com \
  --password 'your-password' \
  --name "Your Name"

go run ./cmd/oa auth login \
  --base-url http://localhost:8080 \
  --email you@example.com \
  --password 'your-password'

go run ./cmd/oa auth init \
  --base-url http://localhost:8080 \
  --email you@example.com \
  --password 'your-password'
```

Useful options:

```bash
# Select a tenant explicitly when the user belongs to multiple tenants
go run ./cmd/oa auth init \
  --base-url http://localhost:8080 \
  --email you@example.com \
  --password 'your-password' \
  --tenant demo1

# Avoid putting the password in shell history
printf '%s\n' 'your-password' | go run ./cmd/oa auth init \
  --base-url http://localhost:8080 \
  --email you@example.com \
  --password-stdin

# Customize token name and lifetime
go run ./cmd/oa auth init \
  --base-url http://localhost:8080 \
  --email you@example.com \
  --password 'your-password' \
  --token-name "Laptop automation" \
  --expires-in-days 30
```

If your user belongs to multiple tenants, pass `--tenant` with the tenant id, slug, or name. The raw API token is shown once when it is created.

## Local config and overrides

The token is stored under your OS user config directory. Typical paths are:

```text
Linux: ~/.config/open-accounting/config.json
macOS: ~/Library/Application Support/open-accounting/config.json
```

If `XDG_CONFIG_HOME` is set, the CLI stores the config under that directory instead.

Environment overrides:

```text
OA_BASE_URL
OA_API_TOKEN
OA_TENANT_ID
OA_SCRIPT_DIR
```

`OA_API_TOKEN` is useful for CI or automation where you do not want to persist local config.

## Inspect auth state

```bash
go run ./cmd/oa auth status
go run ./cmd/oa auth tenants
go run ./cmd/oa auth login --email you@example.com --password 'your-password' --tenant-id <tenant-id>
go run ./cmd/oa auth refresh --refresh-token <refresh-token> --tenant-id <tenant-id>
go run ./cmd/oa auth request-password-reset --email you@example.com
go run ./cmd/oa auth reset-password --token <reset-token> --new-password <new>
printf '%s\n' '<new-password>' | go run ./cmd/oa auth reset-password --token <reset-token> --password-stdin
go run ./cmd/oa auth sessions
go run ./cmd/oa auth sessions --include-inactive
go run ./cmd/oa auth security-events
go run ./cmd/oa auth security-events --limit 100 --json
go run ./cmd/oa auth revoke-session --id <session-id>
go run ./cmd/oa auth revoke-all-sessions
go run ./cmd/oa auth change-password --current-password <old> --new-password <new>
printf '%s\n%s\n' '<old-password>' '<new-password>' | go run ./cmd/oa auth change-password --passwords-stdin
go run ./cmd/oa auth logout --refresh-token <refresh-token>
go run ./cmd/oa auth logout
```

`auth login` and `auth refresh` print short-lived JWT tokens. `auth refresh` returns a replacement refresh token and revokes the presented refresh session. `auth request-password-reset` starts the non-enumerating account recovery flow; production reset tokens are emailed through the server's `PASSWORD_RESET_SMTP_*` settings, while local/dev servers may expose the token when `PASSWORD_RESET_EXPOSE_TOKEN=true`. `auth reset-password` consumes a one-time reset token, stores the new password, and revokes active refresh sessions. `auth sessions` lists refresh sessions for the current user, and `auth security-events` lists recent auth audit events where the current user is actor or target. `auth revoke-session` revokes one session by id, and `auth revoke-all-sessions` revokes every active refresh session for the user. `auth change-password` verifies the current password, stores the new password, and revokes active refresh sessions. `auth logout --refresh-token` revokes that refresh session on the server before removing local CLI config. The normal automation flow still uses `auth init`, which stores a tenant-scoped API token. For automation, use `--json` on token-producing and listing auth commands; `auth init` trims the tenant selector and token name before creating the stored API token, and `--password-stdin`/`--passwords-stdin` read newline-terminated secrets from standard input without persisting them in shell history.

## Tenant administration

```bash
go run ./cmd/oa tenant get
go run ./cmd/oa tenant create \
  --name "Acme Corp" \
  --slug acme-corp \
  --settings-json '{"default_currency":"EUR","country_code":"EE","timezone":"Europe/Tallinn"}'
go run ./cmd/oa tenant update \
  --name "Acme Finance" \
  --settings-json '{"email":"finance@acme.example","timezone":"Europe/Tallinn"}'
go run ./cmd/oa tenant complete-onboarding
go run ./cmd/oa tenant audit-events --limit 50
```

Use `--id <tenant-id>` on `tenant get`, `tenant update`, `tenant complete-onboarding`, and `tenant audit-events` to target a tenant other than the configured one. Use `--settings-file ./tenant-settings.json` instead of `--settings-json` for larger settings payloads. Use `--json` on tenant create/get/update/onboarding/audit commands for automation; tenant IDs and create/update names/slugs are trimmed before API requests.

## Tenant users and invitations

```bash
go run ./cmd/oa users list
go run ./cmd/oa users update-role --id <user-id> --role accountant
go run ./cmd/oa users set-status --id <user-id> --active false
go run ./cmd/oa users sessions --id <user-id>
go run ./cmd/oa users sessions --id <user-id> --include-inactive
go run ./cmd/oa users api-tokens --id <user-id>
go run ./cmd/oa users security-events --id <user-id> --limit 50
go run ./cmd/oa users revoke-api-token --id <user-id> --token-id <token-id>
go run ./cmd/oa users revoke-session --id <user-id> --session-id <session-id>
go run ./cmd/oa users revoke-all-sessions --id <user-id>
go run ./cmd/oa users remove --id <user-id>

go run ./cmd/oa invitations list
go run ./cmd/oa invitations create --email newuser@example.com --role viewer
go run ./cmd/oa invitations revoke --id <invitation-id>
go run ./cmd/oa invitations get --token <invitation-token> --base-url http://localhost:8080
go run ./cmd/oa invitations accept \
  --token <invitation-token> \
  --name "New User" \
  --password 'new-password' \
  --base-url http://localhost:8080
```

`users update-role` accepts `admin`, `accountant`, or `viewer`. The `owner` role is assigned only at tenant creation and cannot be granted through the CLI role-update flow. `users set-status --active false` suspends tenant access without deleting the membership, revokes active refresh sessions, and blocks existing tenant-scoped access/API-token use; `--active true` restores tenant login/refresh/API-token access. `users sessions`, `users api-tokens`, `users security-events`, and the tenant-user revocation commands require a tenant admin or owner and record audit events when sessions or API tokens are revoked. Use `--json` on invitation list, create, get, and accept commands for automation. Public invitation get/accept commands can use `--base-url`, `OA_BASE_URL`, or saved config for the API URL. Use `--password-stdin` on `invitations accept` to avoid placing a new-user password in shell history.

## Plugins

```bash
go run ./cmd/oa plugins list --json
go run ./cmd/oa plugins enable --id <plugin-id> --settings-json '{"enabled":true}'
go run ./cmd/oa plugins enable --id <plugin-id> --settings-file ./plugin-settings.json
go run ./cmd/oa plugins settings get --id <plugin-id>
go run ./cmd/oa plugins settings update --id <plugin-id> --settings-file ./plugin-settings.json
go run ./cmd/oa plugins disable --id <plugin-id>

go run ./cmd/oa admin registries list
go run ./cmd/oa admin registries create --name Official --url https://plugins.example.com
go run ./cmd/oa admin registries sync --id <registry-id>
go run ./cmd/oa admin registries delete --id <registry-id>

go run ./cmd/oa admin plugins list
go run ./cmd/oa admin plugins search --q vat
go run ./cmd/oa admin plugins permissions
go run ./cmd/oa admin plugins install --repository-url https://github.com/example/plugin
go run ./cmd/oa admin plugins get --id <plugin-id>
go run ./cmd/oa admin plugins enable --id <plugin-id> --permission invoices:read
go run ./cmd/oa admin plugins disable --id <plugin-id>
go run ./cmd/oa admin plugins uninstall --id <plugin-id>
```

Tenant plugin `enable` and `settings update` accept exactly one settings source: `--settings-json` for inline JSON or `--settings-file` for a JSON object on disk.
Admin plugin commands use the saved API token from `auth init`; `admin registries` and `admin plugin-registries` are equivalent aliases. Use `--permission` repeatedly when enabling an instance-level plugin with multiple permissions.

## Webhooks

```bash
go run ./cmd/oa webhooks events
go run ./cmd/oa webhooks list --active-only
go run ./cmd/oa webhooks create \
  --name "CRM notifications" \
  --url https://crm.example.com/open-accounting/webhook \
  --events invoice.created,payment.received \
  --secret "$WEBHOOK_SECRET"
go run ./cmd/oa webhooks get --id <webhook-id>
go run ./cmd/oa webhooks update --id <webhook-id> --events invoice.created,webhook.test --active true
go run ./cmd/oa webhooks deliveries --id <webhook-id> --limit 20
go run ./cmd/oa webhooks test --id <webhook-id> --event webhook.test --payload-json '{"source":"cli"}'
go run ./cmd/oa webhooks delete --id <webhook-id>
```

Webhook deliveries are signed with `X-Open-Accounting-Signature: sha256=<hmac>` when a secret is set. Delivery requests also include `X-Open-Accounting-Event`, `X-Open-Accounting-Event-ID`, and `X-Open-Accounting-Tenant-ID`.

## Expenses

```bash
go run ./cmd/oa expenses list --status SUBMITTED --limit 25
go run ./cmd/oa expenses create \
  --merchant "Office Store" \
  --description "Printer toner" \
  --expense-date 2026-05-30 \
  --employee-id <employee-id> \
  --contact-id <supplier-id> \
  --expense-account-id <expense-account-id> \
  --payment-account-id <cash-or-liability-account-id> \
  --amount 120.50 \
  --requires-receipt=true
go run ./cmd/oa expenses import --file ./expenses.csv
go run ./cmd/oa expenses get --id <expense-id>
go run ./cmd/oa documents upload --entity-type expense --entity-id <expense-id> --file ./receipt.pdf --document-type receipt
go run ./cmd/oa documents review --id <document-id> --status APPROVED --note "Receipt accepted"
go run ./cmd/oa expenses submit --id <expense-id>
go run ./cmd/oa expenses approve --id <expense-id>
go run ./cmd/oa expenses reject --id <expense-id> --reason "Need project code"
go run ./cmd/oa expenses post --id <expense-id>
```

Expense statuses are `DRAFT`, `SUBMITTED`, `APPROVED`, `REJECTED`, and `POSTED`. Posting an approved expense creates and posts a balanced journal entry using `--expense-account-id` as the debit line and `--payment-account-id` as the credit line. Receipt-backed expenses require at least one approved `receipt` document linked with `entity-type expense` before approval or posting. Expense CSV imports require `expense_date`, `merchant`, `amount`, and either `expense_account_id`/`payment_account_id` or `expense_account_code`/`payment_account_code`; optional columns include `expense_number`, `description`, `employee_id`, `contact_id`, `currency`, `exchange_rate`, `requires_receipt`, `status`, `submitted_at`, `approved_at`, `rejected_at`, and `rejection_reason`. ID columns must be valid UUIDs; account-code columns remain supported for chart-of-accounts lookups. Imported `POSTED` rows are rejected so posting still creates ledger entries through the workflow.

## Manage API tokens

```bash
go run ./cmd/oa tokens list
go run ./cmd/oa tokens create --name "CI automation" --expires-in-days 90
go run ./cmd/oa tokens revoke --id <token-id>
```

`tokens create` returns the raw token once. Store it immediately if you need to use it outside the CLI config flow. Creating or revoking a token records an auth security event for the user.

## Migration validation

```bash
go run ./cmd/oa migration validate \
  --accounts ./accounts.csv \
  --contacts ./contacts.csv \
  --employees ./employees.csv \
  --expenses ./expenses.csv \
  --invoices ./invoices.csv \
  --e-invoices ./supplier-einvoices.xml \
  --e-invoice-contact-mode supplier \
  --provider-preset generic \
  --payments ./payments.csv \
  --bank-accounts ./bank-accounts.csv \
  --bank-transactions ./bank-transactions.csv \
  --payroll-history ./payroll-history.csv \
  --leave-balances ./leave-balances.csv \
  --tsd-history ./tsd-history.csv \
  --kmd-history ./kmd-history.csv \
  --quotes ./quotes.csv \
  --orders ./orders.csv \
  --recurring-invoices ./recurring-invoices.csv \
  --cost-centers ./cost-centers.csv \
  --cost-allocations ./cost-allocations.csv \
  --product-categories ./product-categories.csv \
  --warehouses ./warehouses.csv \
  --products ./products.csv \
  --stock ./stock.csv \
  --fixed-assets ./fixed-assets.csv \
  --opening-balances ./opening-balances.csv \
  --journal ./journal-entries.csv
go run ./cmd/oa migration validate --provider-preset merit --contacts ./contacts.csv --e-invoices ./sales-einvoices.xml --e-invoice-contact-mode customer --json
```

`migration validate` is a non-mutating cutover preflight. It checks required CSV column groups, duplicate business identifiers, grouped-document header and preserved-ID consistency, row-value errors, Estonian e-invoice XML payloads, and same-bundle cross-file references for accounts, contacts, employees, expenses, invoices, e-invoices, payments, bank accounts, bank transactions, payroll history, leave balances, TSD history, KMD history, quotes, orders, recurring invoice templates, cost centers, cost allocations, product categories, warehouses, products, stock adjustments, fixed assets, opening balances, and historical journal entries before you run the individual import commands. Use `--provider-preset generic`, `--provider-preset merit`, or `--provider-preset smartaccounts` to select CSV header aliases before canonical validation runs; provider presets include employee, payroll-history, leave-balance, and TSD-history aliases, including combined period fields such as Merit Palk `Month6`.

Account row validation checks required codes/names, optional preserved `id` or `account_id` UUIDs, and importer-supported `account_type` aliases or uppercase enum values before import. Contact row validation checks required names, contact type aliases, payment terms, country-code length, and credit-limit decimal values before import. Employee master row validation checks required first/last names and start dates, importer-supported employment-type and boolean aliases, optional end-date ordering, basic-exemption and funded-pension rates, positive base salaries, and salary effective dates before import. Payroll-history row validation checks period bounds, payroll and payment statuses, payment and paid dates, required positive gross salary, non-negative tax/deduction/employer-cost amounts, duplicate employee rows inside each payroll period, and consistent status, payment date, and notes inside each payroll period before import. Leave-balance row validation checks importer-compatible year bounds, `absence_type_id` UUID syntax, duplicate employee/absence-type rows inside each year, and non-negative entitled, carryover, used, and pending day values before import. TSD-history row validation checks importer-compatible periods, declaration statuses, submitted dates, required positive gross payment, non-negative tax/pension amounts, duplicate employee rows inside each TSD period, and consistent status, submitted date, and EMTA reference inside each TSD period before import. KMD-history row validation checks importer-compatible years, months, declaration statuses, submitted dates, required row codes, duplicate row codes inside each declaration period, tax-base or tax-amount decimals, optional VAT totals, and consistent status, submitted date, output VAT, and input VAT inside each KMD period before import. Invoice CSV validation requires `invoice_type` and `due_date` because invoice imports group line rows by `invoice_number` plus `invoice_type` and require explicit due dates; grouped invoice, quote, order, and recurring-invoice rows must keep header-level fields such as dates, contacts, currency, status, and template settings consistent, and preserved invoice/quote UUIDs must not be reused by another grouped document. Commercial document row validation checks invoice/quote/order/recurring dates, due/valid-until/end-date ordering, quantities, prices, discounts, VAT rates, exchange rates, statuses, invoice VAT treatment, amount paid, recurring frequencies, recurring counters, and recurring boolean settings before import. Product-category, product, warehouse, and cost-center row validation checks category names and importer-compatible parent ordering, product names, types, prices, VAT and stock thresholds, inventory/active flags, statuses, lead times, warehouse codes/names, cost-center codes/names, budgets, budget periods, and warehouse default/active flags before import. Product, warehouse, and cost-center master imports generate new UUIDs and preserve codes, so same-bundle downstream references should use `product_code`, `warehouse_code`, or `cost_center_code`. Stock-adjustment validation checks product and warehouse identifiers, strict decimal quantity, nonzero signed adjustments, non-negative unit costs, and YYYY-MM-DD expiry dates while recognizing optional lot metadata columns such as `lot_number`, `serial_number`, and `expiry_date`, plus aliases including `batch`, `serial`, and `expiration_date`; `description` is accepted as a `reason` alias. Fixed-asset row validation checks required names, purchase dates, positive purchase costs, supported statuses, depreciation methods, useful-life months, residual values, accumulated depreciation, book-value consistency, disposal methods, and disposal proceeds before import. Cost-allocation row validation checks cost-center references, journal line UUIDs, positive allocation amounts, allocation percentages, and YYYY-MM-DD allocation dates before import; cost-allocation `cost_center_id` values are existing UUIDs while `cost_center_code` is the same-bundle lookup path, and `description` is accepted as a `notes` alias. Bank-account row validation checks required names/account numbers, optional default/active booleans, and currency code format before import.

Account validation also checks `parent_code` against the accounts file and hierarchy imports reject self-parent rows before import. Contact validation accepts optional `id` or `contact_id` UUIDs so preserved contact IDs can be referenced by later cutover files, and checks duplicate contact IDs, codes, registry codes, VAT numbers, emails, and names inside the same bundle. Invoice and quote validation accepts optional preserved UUIDs, requires them to be valid where supplied, and rejects reuse across different grouped documents. E-invoice validation defaults to `--e-invoice-contact-mode supplier`, which checks the seller party against contacts for purchase/supplier bill cutovers; use `customer` to check the buyer party for outbound sales e-invoice cutovers, or `both` to require both parties in stricter mixed bundles. Payment validation checks `payment_type`, `payment_date`, positive payment/exchange/allocation amounts, allocation references, over-allocation, `contact_id` as a UUID/preserved contact ID against contacts, and `invoice_id` or `invoice_number` against invoice files when those files are included in the same bundle, including `id` or `invoice_id` UUIDs preserved by invoice import. Expense validation checks required row values, expense/payment account ID or code presence, positive amount and exchange-rate values, supported statuses, receipt flags, rejected-expense reasons, status timestamps, `contact_id` as a UUID/preserved contact ID against contacts, account code fields against accounts when related files are included, and account ID fields as UUIDs that can target preserved account IDs. Recurring-invoice validation checks line-level `account_id` values as UUIDs and against preserved account IDs when an accounts file is included. Bank-account validation checks required `name` and `account_number` row values, optional `is_default` and `is_active` booleans, currency code format, and `gl_account_id` as a UUID or `gl_account_code` as an account code against the accounts file when both are included. Bank-transaction validation checks required transaction dates and decimal amounts, allows negative statement amounts for outflows, recognizes generic and LHV-style statement account headers such as `source_account`, `client_account`, `account_number`, and `bank_account`, then checks matching bank account numbers and currencies when a bank-accounts file is included.

Employee master validation checks importer-compatible dates, employment settings, salary values, and tax settings before import. Payroll-history validation checks importer-compatible periods, statuses, dates, amounts, duplicate employee-period keys, and same-period consistency before import. Leave-balance validation checks importer-compatible years, `absence_type_id` UUID syntax, day totals, and duplicate employee/year/absence-type keys before import. TSD-history validation checks importer-compatible periods, statuses, dates, amounts, duplicate employee-period keys, and same-period consistency before import. KMD-history validation checks importer-compatible periods, statuses, dates, row amounts, duplicate period row codes, and same-period VAT-total consistency before import. Payroll, leave-balance, and TSD validation require an employee identifier: `employee_number`, `personal_code`, `email`, `name`, or both `first_name` and `last_name`, and check employee references when an employees file is included. Product validation also checks category IDs as UUIDs/preserved product category IDs or category names as category names, sale, purchase, and inventory account ID fields as UUIDs/preserved account IDs, account-code fields as account codes, and `supplier_id` as a UUID/preserved contact ID against contacts when those related files are included. Commercial document reference validation checks quote, order, and recurring-invoice `contact_id` values as UUIDs/preserved contact IDs against contacts, invoice/quote/order/recurring-invoice line `product_id` values as UUIDs, line `product_code` values against product files when both are included, and order `quote_id` values against same-bundle quote IDs when a quotes file is included. Fixed-asset validation checks `supplier_id` as a UUID/preserved contact ID against contacts, `invoice_id` against invoice files, and asset, depreciation expense, and accumulated depreciation account ID fields as UUIDs/preserved account IDs or account-code fields as account codes when account files are included. Opening-balance validation requires account codes plus both debit and credit columns, rejects negative, zero, or dual-sided rows, and checks the file's debit and credit totals balance. Historical journal validation recognizes importer aliases including voucher number, posting date, debit/credit amount, and exchange rate, then checks `source_id` UUID syntax, account codes, grouped entry references for one date, at least two lines, nonzero amounts, and balanced base-currency totals. Cost-allocation validation checks required `journal_entry_line_id`, `amount`, `allocation_date`, and `cost_center_id` or `cost_center_code` columns, `journal_entry_line_id` UUID syntax, plus same-bundle cost-center references when a cost center file is included.

## Accounts

```bash
go run ./cmd/oa accounts list
go run ./cmd/oa accounts list --active-only
go run ./cmd/oa accounts hierarchy --active-only
go run ./cmd/oa accounts create --code 1100 --name Cash --type ASSET
go run ./cmd/oa accounts get --id <account-id>
go run ./cmd/oa accounts update --id <account-id> --code 1110 --name "Cash on hand" --type ASSET
go run ./cmd/oa accounts delete --id <account-id>
go run ./cmd/oa accounts import --file ./accounts.csv
```

`accounts hierarchy` shows the parent-child chart of accounts using existing `parent_id` relationships. `accounts update` and `accounts delete` only apply to custom accounts; system accounts are immutable, and delete deactivates the account instead of removing ledger history. Account CSV imports can preserve optional `id`, `account_id`, or `account_uuid` UUIDs and set parent accounts with `parent_code`; importer-compatible aliases include `number` for `code`, `category` for `account_type`, and `parent` or `parent_account` for `parent_code`.

## Contacts

```bash
go run ./cmd/oa contacts list
go run ./cmd/oa contacts list --type CUSTOMER --search Nordic
go run ./cmd/oa contacts create --name "New Customer" --type CUSTOMER --email customer@example.com
go run ./cmd/oa contacts get --id <contact-id>
go run ./cmd/oa contacts update --id <contact-id> --email billing@example.com --payment-terms-days 30
go run ./cmd/oa contacts delete --id <contact-id>
go run ./cmd/oa contacts import --file ./contacts.csv
```

Use `--json` on contacts list/create/get/update/delete/import commands for automation. Contact CSV import accepts optional `id` or `contact_id` UUID columns and preserves those IDs for cutover references from `contact_id` or `supplier_id` fields. Importer-compatible aliases include `company` or `company_name` for `name`, `type` or `role` for `contact_type`, `vat` or `vat_no` for `vat_number`, `telephone` for `phone`, `address`, `street`, or `street_address` for `address_line1`, `postcode`, `zip`, or `zip_code` for `postal_code`, `country` for `country_code`, and `payment_days` or `terms_days` for `payment_terms_days`. `credit_limit` accepts comma decimals such as `1500,50` and thousands separators such as `1,500.50`. Contact list filters and create/update fields are trimmed before API requests; contact type and country code inputs are normalized to uppercase.

## Employees

```bash
go run ./cmd/oa employees list
go run ./cmd/oa employees list --active-only
go run ./cmd/oa employees create \
  --employee-number EMP-001 \
  --first-name Mari \
  --last-name Maasikas \
  --start-date 2026-01-15 \
  --employment-type FULL_TIME
go run ./cmd/oa employees get --id <employee-id>
go run ./cmd/oa employees update --id <employee-id> --department Finance --active true
go run ./cmd/oa employees set-salary --id <employee-id> --amount 3200.00 --effective-from 2026-03-01
go run ./cmd/oa employees add-salary-component --id <employee-id> --type SECONDARY_EMPLOYMENT --name "Evening contract" --amount 600.00 --effective-from 2026-03-01
go run ./cmd/oa employees salary-components --id <employee-id> --active-on 2026-03-15
go run ./cmd/oa employees import --file ./employees.csv
```

## Payroll runs

```bash
go run ./cmd/oa payroll runs list
go run ./cmd/oa payroll runs list --year 2026
go run ./cmd/oa payroll runs list --json
go run ./cmd/oa payroll runs create --year 2026 --month 3 --payment-date 2026-03-31
go run ./cmd/oa payroll runs get --id <payroll-run-id>
go run ./cmd/oa payroll runs calculate --id <payroll-run-id>
go run ./cmd/oa payroll runs process --id <payroll-run-id> --approve
go run ./cmd/oa payroll runs approve --id <payroll-run-id>
go run ./cmd/oa payroll runs payslips --id <payroll-run-id>
go run ./cmd/oa payroll runs payslip-pdf --run-id <payroll-run-id> --payslip-id <payslip-id> --output ./payslip.pdf
go run ./cmd/oa payroll tax-preview --gross-salary 3200.00
```

Use `payroll runs calculate` after employee salary setup, then `payroll runs approve` before TSD generation. Use `payroll runs process --approve` to bulk-calculate all active employees in a draft run and approve it in one request. `payroll runs create` accepts an optional `--payment-date`; when omitted, the API receives no payment date. Payroll run IDs are trimmed before API requests. Use `--json` on read and mutation commands when scripting, including list/create/get/calculate/process/approve/payslips. `payroll runs payslip-pdf` downloads a generated PDF for one payslip; pass `--output` to write a file, or omit it to stream the PDF bytes to stdout.

## Payroll migration imports

```bash
go run ./cmd/oa payroll import-history --file ./payroll-history.csv
go run ./cmd/oa payroll import-history --file ./payroll-history.csv --json
go run ./cmd/oa payroll import-leave-balances --file ./leave-balances.csv
```

Import employees first so payroll history rows can match existing employees by `employee_number`, `personal_code`, `email`, `name`, or `first_name` + `last_name`. Existing payroll periods are skipped rather than overwritten; migration preflight reports duplicate employee rows inside the same payroll period. Use `--json` when you need row-level import errors for automation.

Leave balance imports create or update balances by employee + absence type + year. Match absence types with `absence_type_code`, `absence_type`, or a UUID `absence_type_id`; migration preflight reports duplicate employee + absence type rows inside the same year.

## Leave management

```bash
go run ./cmd/oa leave absence-types list --active-only
go run ./cmd/oa leave absence-types get --id <absence-type-id>

go run ./cmd/oa leave balances list --employee-id <employee-id> --year 2026
go run ./cmd/oa leave balances by-year --employee-id <employee-id> --year 2026
go run ./cmd/oa leave balances update \
  --employee-id <employee-id> \
  --absence-type-id <absence-type-id> \
  --year 2026 \
  --entitled-days 28 \
  --carryover-days 2 \
  --notes "Imported balance correction"
go run ./cmd/oa leave balances initialize --employee-id <employee-id> --year 2026
go run ./cmd/oa leave balances import --file ./leave-balances.csv

go run ./cmd/oa leave records list --employee-id <employee-id> --year 2026
go run ./cmd/oa leave records create \
  --employee-id <employee-id> \
  --absence-type-id <absence-type-id> \
  --start-date 2026-07-01 \
  --end-date 2026-07-05 \
  --total-days 5 \
  --working-days 3 \
  --notes "Summer leave"
go run ./cmd/oa leave records get --id <leave-record-id>
go run ./cmd/oa documents upload --entity-type leave_record --entity-id <leave-record-id> --file ./medical-certificate.pdf --document-type supporting_document
go run ./cmd/oa documents review --id <document-id> --status APPROVED --note "Leave evidence accepted"
go run ./cmd/oa leave records approve --id <leave-record-id>
go run ./cmd/oa leave records reject --id <leave-record-id> --reason "Staffing shortage"
go run ./cmd/oa leave records cancel --id <leave-record-id>
```

Use `--json` on leave-management reads and mutations for automation. Leave commands trim identifiers and dates before sending API requests, validate required IDs, years, positive day counts, and non-negative balance adjustments locally, and surface API errors without falling back to legacy client-side behavior. Leave record statuses are `PENDING`, `APPROVED`, `REJECTED`, and `CANCELLED`. Absence types marked `requires_document=true` block approval until the leave record has at least one approved `supporting_document` or `tax_support` document attached with `--entity-type leave_record`.

## TSD declarations

```bash
go run ./cmd/oa tsd list
go run ./cmd/oa tsd list --year 2026 --month 3
go run ./cmd/oa tsd get --year 2026 --month 3
go run ./cmd/oa tsd generate --run-id <payroll-run-id>
go run ./cmd/oa tsd export-xml --year 2026 --month 3 --output ./tsd-2026-03.xml
go run ./cmd/oa tsd export-csv --year 2026 --month 3 --output ./tsd-2026-03.csv
go run ./cmd/oa tsd import-history --file ./tsd-history.csv
go run ./cmd/oa tsd import-history --file ./tsd-history.csv --json
go run ./cmd/oa tsd mark-submitted --year 2026 --month 3 --emta-reference EMTA-123
go run ./cmd/oa tsd mark-accepted --year 2026 --month 3
go run ./cmd/oa tsd mark-rejected --year 2026 --month 3
```

`tsd list` accepts optional `--year` and `--month` filters; `--month` must be between 1 and 12 when provided. TSD period commands require `--year` and `--month`; `--month` must be between 1 and 12. Omit `--output` on export commands to write the raw XML or CSV to stdout. Use `--json` on list/get/generate/import-history/mark-submitted/mark-accepted/mark-rejected for automation.

Historical TSD imports use one CSV row per employee declaration row and group rows by `year` + `month`. Required columns are `year`, `month`, `gross_payment`, and an employee identifier (`employee_number`, `personal_code`, `email`, `name`, or `first_name` + `last_name`). Optional columns include `status`, `submitted_at`, `emta_reference`, `payment_type`, `basic_exemption`, `taxable_amount`, `income_tax`, `social_tax`, `unemployment_insurance_employer`, `unemployment_insurance_employee`, and `funded_pension`. Migration preflight checks period bounds, supported statuses, submitted-date format, positive gross payment, non-negative tax/pension amounts, duplicate employee-period rows, and consistent status, submitted date, and EMTA reference inside each TSD period. Existing TSD declaration periods are skipped instead of overwritten.

## KMD declarations

```bash
go run ./cmd/oa tax kmd list
go run ./cmd/oa tax kmd generate --year 2026 --month 3
go run ./cmd/oa tax kmd inf --year 2026 --month 3
go run ./cmd/oa tax kmd inf --year 2026 --month 3 --threshold 1000 --json
go run ./cmd/oa tax kmd import-history --file ./kmd-history.csv
go run ./cmd/oa tax kmd import-history --file ./kmd-history.csv --json
go run ./cmd/oa tax kmd export-xml --year 2026 --month 3 --output ./kmd-2026-03.xml
go run ./cmd/oa tax oss report --year 2026 --quarter 1
go run ./cmd/oa tax oss report --year 2026 --quarter 1 --include-b2b --json
```

KMD period commands require `--year` and `--month`; `--month` must be between 1 and 12. Use `--json` on `list`, `generate`, `inf`, and `import-history` for automation.

Historical KMD import expects `year`, `month`, and `row_code` columns, plus `tax_base` or `tax_amount` on each row. Optional `status`, `submitted_at`, `description`, `total_output_vat`, and `total_input_vat` columns are validated by `migration validate`, including duplicate period row codes, same-period status, submitted date, output VAT, and input VAT consistency. Existing declaration periods are skipped instead of overwritten.

KMD INF generation returns A-part sales and B-part purchase invoice rows for domestic VAT-bearing invoices whose partner-period taxable total reaches the threshold excluding VAT. The default threshold is `1000`; pass `--threshold` only with a positive decimal value when overriding it.

KMD export writes e-MTA XML. Omit `--output` to stream the XML to stdout.

EU VAT OSS reporting groups non-Estonian EU sales invoice lines by destination country and VAT rate for quarterly manual filing support. By default it excludes contacts with VAT numbers to focus on B2C OSS rows; add `--include-b2b` only when you need a reconciliation view that includes VAT-registered contacts.

## Invoices

```bash
go run ./cmd/oa invoices list
go run ./cmd/oa invoices list --type SALES --status DRAFT --from 2026-03-01 --to 2026-03-31
go run ./cmd/oa invoices create \
  --type SALES \
  --contact-id <contact-id> \
  --issue-date 2026-03-15 \
  --due-date 2026-03-29 \
  --reference PO-123 \
  --line "description=Consulting,quantity=2,unit=hour,unit_price=100.00,vat_rate=22.00"
go run ./cmd/oa invoices create \
  --type PURCHASE \
  --contact-id <supplier-id> \
  --issue-date 2026-03-20 \
  --due-date 2026-04-03 \
  --currency USD \
  --exchange-rate 0.93 \
  --reference SUP-2026-03 \
  --line "description=Materials,quantity=5,unit=unit,unit_price=25.00,vat_rate=20.00,account_id=<expense-account-id>"
go run ./cmd/oa invoices create \
  --type PURCHASE \
  --contact-id <supplier-id> \
  --issue-date 2026-03-15 \
  --due-date 2026-03-29 \
  --line "description=EU service,quantity=1,unit_price=100.00,vat_rate=22.00,vat_treatment=reverse_charge"
go run ./cmd/oa invoices get --id <invoice-id>
go run ./cmd/oa invoices pdf --id <invoice-id> --output ./invoice.pdf
go run ./cmd/oa invoices send --id <invoice-id>
go run ./cmd/oa invoices void --id <invoice-id>
go run ./cmd/oa invoices import --file ./invoices.csv
go run ./cmd/oa invoices import-einvoice --file ./supplier-einvoice.xml --invoice-type PURCHASE
```

Use `--line` repeatedly on `invoices create` for multi-line invoices. Each line is comma-separated `key=value` pairs with `description`, `quantity`, `unit_price`, and `vat_rate`; optional keys include `unit`, `discount_percent`, `vat_treatment`, `reverse_charge`, `account_id`, and `product_id`. Set `vat_treatment=reverse_charge` or `reverse_charge=true` for purchase invoices where VAT is self-assessed: the VAT rate is retained for KMD reporting but VAT is not added to the invoice total. Use `--type PURCHASE` with a supplier contact to enter purchase invoices and supplier bills; `account_id` should point at the expense, asset, or other posting account for that purchase line. Invoice CSV imports use one row per invoice line and group rows by `invoice_number` plus `invoice_type`; optional `id` or `invoice_id` must be a valid UUID, is preserved when supplied, and can be targeted by payment CSV imports through `invoice_id`; line-level `product_id` values must also be valid UUIDs. `invoices import-einvoice` imports local Estonian `E_Invoice` XML files and matches contacts by registry code, VAT number, email, or name; omit `--invoice-type` to default debit e-invoices to `PURCHASE` and credit e-invoices to `CREDIT_NOTE`. Sending or emailing a draft purchase invoice requires at least one approved `receipt`, `supporting_document`, or `tax_support` document attached to the `invoice` entity.

## Quotes

```bash
go run ./cmd/oa quotes list
go run ./cmd/oa quotes list --status DRAFT --contact-id <contact-id> --from 2026-03-01 --to 2026-03-31
go run ./cmd/oa quotes create \
  --contact-id <contact-id> \
  --quote-date 2026-03-15 \
  --valid-until 2026-04-15 \
  --notes "March offer" \
  --line "description=Consulting,quantity=2,unit=hour,unit_price=100.00,vat_rate=22.00"
go run ./cmd/oa quotes get --id <quote-id>
go run ./cmd/oa quotes update \
  --id <quote-id> \
  --contact-id <contact-id> \
  --quote-date 2026-03-16 \
  --line "description=Updated consulting,quantity=3,unit=hour,unit_price=100.00,vat_rate=22.00"
go run ./cmd/oa quotes send --id <quote-id> --require-approved-evidence
go run ./cmd/oa quotes pdf --id <quote-id> --output ./quote.pdf
go run ./cmd/oa quotes accept --id <quote-id>
go run ./cmd/oa quotes reject --id <quote-id>
go run ./cmd/oa quotes convert-to-invoice --id <quote-id> --issue-date 2026-03-20 --due-date 2026-04-03
go run ./cmd/oa quotes delete --id <quote-id>
go run ./cmd/oa quotes import --file ./quotes.csv
go run ./cmd/oa email quote --quote-id <quote-id> --recipient-email billing@example.com --attach-pdf
```

Use `--line` repeatedly on `quotes create` and `quotes update` for multi-line offers. Each line accepts `description`, `quantity`, `unit_price`, and `vat_rate`; optional keys include `unit`, `discount_percent`, and `product_id`. Quote statuses are `DRAFT`, `SENT`, `ACCEPTED`, `REJECTED`, `EXPIRED`, and `CONVERTED`; accepted quotes can be converted into draft sales invoices. `quotes send --require-approved-evidence` blocks sending until an approved `contract` or `supporting_document` is attached to the quote.

Use `--json` on quote read, write, import, status, conversion, email, and delete commands when scripting. Quote IDs and text fields are trimmed before requests, status filters are case-insensitive, and `quotes delete --json` returns `{"status":"deleted"}`. `quotes pdf` writes to `--output` or streams to stdout with `--output -`. `email quote` can attach the generated quote PDF and marks draft quotes as sent after successful delivery. The `--require-approved-evidence` flag is valid on `quotes send` and `email quote`.

Quote imports use one CSV row per quote line and group rows by `quote_number`. Required columns are `quote_number`, `quote_date`, a contact identifier (`contact_id`, `contact_code`, `contact_reg_code`, `contact_email`, or `contact_name`), `line_description`, `quantity`, `unit_price`, and `vat_rate`; optional columns include `id` or `quote_id` for a valid UUID to preserve during cutover, `valid_until`, `status`, `currency`, `exchange_rate`, `notes`, `unit`, `discount_percent`, and `product_id` or `product_code`. Direct `contact_id` and `product_id` values must be valid UUIDs; `sku` and `item_code` are accepted as `product_code` aliases.

## Orders

```bash
go run ./cmd/oa orders list
go run ./cmd/oa orders list --status CONFIRMED --contact-id <contact-id> --from 2026-03-01 --to 2026-03-31
go run ./cmd/oa orders create \
  --contact-id <contact-id> \
  --order-date 2026-03-15 \
  --expected-delivery 2026-03-22 \
  --quote-id <quote-id> \
  --line "description=Consulting,quantity=2,unit=hour,unit_price=100.00,vat_rate=22.00"
go run ./cmd/oa orders get --id <order-id>
go run ./cmd/oa orders stock-check --id <order-id> --warehouse-id <warehouse-id>
go run ./cmd/oa orders stock-reservations --id <order-id>
go run ./cmd/oa orders pick-list --id <order-id> --warehouse-id <warehouse-id>
go run ./cmd/oa orders reserve-stock --id <order-id> --warehouse-id <warehouse-id> --reason "Pick list"
go run ./cmd/oa orders release-stock --id <order-id> --warehouse-id <warehouse-id> --reason "Order canceled"
go run ./cmd/oa orders update \
  --id <order-id> \
  --contact-id <contact-id> \
  --order-date 2026-03-16 \
  --line "description=Updated consulting,quantity=3,unit=hour,unit_price=100.00,vat_rate=22.00"
go run ./cmd/oa orders confirm --id <order-id> --require-approved-evidence
go run ./cmd/oa orders pdf --id <order-id> --output ./order.pdf
go run ./cmd/oa orders process --id <order-id>
go run ./cmd/oa orders ship --id <order-id>
go run ./cmd/oa orders deliver --id <order-id>
go run ./cmd/oa orders convert-to-invoice --id <order-id> --issue-date 2026-03-24 --due-date 2026-04-07
go run ./cmd/oa orders cancel --id <order-id>
go run ./cmd/oa orders delete --id <order-id>
go run ./cmd/oa orders import --file ./orders.csv
go run ./cmd/oa email order --order-id <order-id> --recipient-email billing@example.com --attach-pdf
```

Use `--line` repeatedly on `orders create` and `orders update`. Each line accepts `description`, `quantity`, `unit_price`, and `vat_rate`; optional keys include `unit`, `discount_percent`, and `product_id`. Order statuses are `PENDING`, `CONFIRMED`, `PROCESSING`, `SHIPPED`, `DELIVERED`, and `CANCELED`. Delivered orders can be converted into draft sales invoices. `orders confirm --require-approved-evidence` blocks confirmation until an approved `contract` or `supporting_document` is attached to the order.

Use `--json` on order read, write, stock, import, status, conversion, email, and delete commands when scripting. Mutating commands return the updated order or operation result; `orders convert-to-invoice --json` returns both the updated order and the created draft invoice, while `orders delete --json` returns `{"status":"deleted"}`. `orders pdf` writes to `--output` or streams to stdout with `--output -`. `email order` can attach the generated order PDF and marks pending orders as confirmed after successful delivery. The `--require-approved-evidence` flag is valid on `orders confirm` and `email order`.

`orders stock-check` checks tracked product lines without mutating inventory. It sums all warehouses unless `--warehouse-id` is provided, consumes repeated lines for the same product cumulatively inside the check, and reports per-line statuses: `AVAILABLE`, `SHORTAGE`, `NOT_TRACKED`, and `PRODUCT_NOT_FOUND`.

`orders reserve-stock` and `orders release-stock` mutate the selected warehouse's reserved and available quantities for the order's cumulative tracked goods and update the persisted order-level reservation ledger. `reserve-stock` first requires the selected warehouse to be ready for every tracked line. Use `orders stock-reservations` to inspect the ledger by order, and `orders pick-list` to turn those reservations into a warehouse picking view.

Order imports use one CSV row per order line and group rows by `order_number`. Required columns are `order_number`, `order_date`, a contact identifier (`contact_id`, `contact_code`, `contact_reg_code`, `contact_email`, or `contact_name`), `line_description`, `quantity`, `unit_price`, and `vat_rate`; optional columns include `expected_delivery`, `status`, `currency`, `exchange_rate`, `notes`, `quote_id`, `unit`, `discount_percent`, and `product_id` or `product_code`. Direct `contact_id`, `product_id`, and `quote_id` values must be valid UUIDs; `quote_id` can point at a quote UUID preserved by a quote import `id` or `quote_id` column. `sku` and `item_code` are accepted as `product_code` aliases.

## Recurring invoices

```bash
go run ./cmd/oa recurring-invoices list --active-only
go run ./cmd/oa recurring-invoices create \
  --name "Monthly retainer" \
  --contact-id <contact-id> \
  --frequency MONTHLY \
  --start-date 2026-03-15 \
  --payment-terms-days 21 \
  --line "description=Consulting,quantity=2,unit=hour,unit_price=100.00,vat_rate=22.00"
go run ./cmd/oa recurring-invoices from-invoice \
  --invoice-id <invoice-id> \
  --name "Repeat invoice" \
  --frequency QUARTERLY \
  --start-date 2026-04-01
go run ./cmd/oa recurring-invoices import --file ./recurring-invoices.csv
go run ./cmd/oa recurring-invoices get --id <recurring-id>
go run ./cmd/oa recurring-invoices update --id <recurring-id> --frequency YEARLY --payment-terms-days 30
go run ./cmd/oa recurring-invoices pause --id <recurring-id>
go run ./cmd/oa recurring-invoices resume --id <recurring-id>
go run ./cmd/oa recurring-invoices generate --id <recurring-id>
go run ./cmd/oa recurring-invoices generate-due
go run ./cmd/oa recurring-invoices delete --id <recurring-id>
```

Frequencies are `WEEKLY`, `BIWEEKLY`, `MONTHLY`, `QUARTERLY`, and `YEARLY`. Use `--line` repeatedly on create or update. Recurring invoice email options include `--send-email`, `--recipient-email`, `--attach-pdf`, `--email-subject`, and `--email-message`.

Recurring invoice imports use one CSV row per recurring template line and group rows by `name`. Required columns are `name`, `frequency`, `start_date`, a contact identifier (`contact_id`, `contact_code`, `contact_reg_code`, `contact_email`, or `contact_name`), `line_description`, `quantity`, `unit_price`, and `vat_rate`; optional columns include `invoice_type`, `currency`, `end_date`, `next_generation_date`, `payment_terms_days`, `reference`, `notes`, active/generation/email settings, `unit`, `discount_percent`, `account_id`, and `product_id` or `product_code`. Direct `contact_id`, `product_id`, and `account_id` values must be valid UUIDs; `sku` and `item_code` are accepted as `product_code` aliases. In migration bundle preflight, recurring line `account_id` values can reference preserved account IDs from the same accounts file. Duplicate template names are skipped.

## Fixed assets

```bash
go run ./cmd/oa assets categories list
go run ./cmd/oa assets categories create \
  --name Equipment \
  --depreciation-method STRAIGHT_LINE \
  --useful-life-months 60
go run ./cmd/oa assets categories get --id <category-id>
go run ./cmd/oa assets categories delete --id <category-id>

go run ./cmd/oa assets list --status ACTIVE --category-id <category-id>
go run ./cmd/oa assets create \
  --name Laptop \
  --category-id <category-id> \
  --purchase-date 2026-03-15 \
  --purchase-cost 1200.00 \
  --useful-life-months 36 \
  --residual-value 100.00 \
  --depreciation-start-date 2026-04-01
go run ./cmd/oa assets import --file ./assets.csv
go run ./cmd/oa assets get --id <asset-id>
go run ./cmd/oa assets update --id <asset-id> --name Laptop --useful-life-months 48
go run ./cmd/oa documents upload --entity-type asset --entity-id <asset-id> --file ./asset-invoice.pdf --document-type asset_record
go run ./cmd/oa documents review --id <document-id> --status APPROVED --note "Asset acquisition evidence accepted"
go run ./cmd/oa assets activate --id <asset-id>
go run ./cmd/oa documents upload --entity-type asset --entity-id <asset-id> --file ./asset-sale-approval.pdf --document-type supporting_document
go run ./cmd/oa documents review --id <document-id> --status APPROVED --note "Asset disposal evidence accepted"
go run ./cmd/oa assets dispose \
  --id <asset-id> \
  --disposal-date 2026-05-01 \
  --method SOLD \
  --proceeds 900.00 \
  --proceeds-account-id <cash-account-id> \
  --gain-loss-account-id <asset-disposal-gain-account-id>
go run ./cmd/oa assets depreciate --id <asset-id>
go run ./cmd/oa assets depreciation --id <asset-id>
go run ./cmd/oa assets delete --id <asset-id>
```

Asset statuses are `DRAFT`, `ACTIVE`, `DISPOSED`, and `SOLD`. Asset creation requires `--name`, `--purchase-date`, and positive `--purchase-cost`; updates require `--id` and `--name`. Asset IDs, category IDs, account IDs, supplier IDs, descriptions, serial numbers, locations, and disposal notes are trimmed before requests are sent. Use `--json` on asset read and mutation commands for automation-friendly output. Asset categories provide defaults for depreciation method, useful life, residual percent, and asset/depreciation account IDs when those fields are omitted on `assets create` or when `assets update` changes category without overriding them; omitted category and account values are preserved on ordinary updates. Activating a draft asset requires approved `asset_record`, `receipt`, or `contract` evidence attached to the `asset` entity; pending or missing evidence returns a conflict before the asset can enter depreciation. Disposing or selling an active asset requires approved `supporting_document` or `contract` evidence attached to the same asset, then persists the disposal date, method, proceeds, notes, and disposal journal ID. Depreciation methods are `STRAIGHT_LINE`, `DECLINING_BALANCE`, and `UNITS_OF_PRODUCTION`; disposal methods are `SOLD`, `SCRAPPED`, `DONATED`, and `LOST`. `assets depreciate` requires depreciation expense and accumulated depreciation account IDs, posts a balanced `ASSET_DEPRECIATION` journal entry, and `assets depreciation` shows the linked journal ID. `assets dispose` requires asset and accumulated-depreciation account links, posts a balanced `ASSET_DISPOSAL` journal that removes asset cost, clears accumulated depreciation, records proceeds to `--proceeds-account-id`, and posts any gain or loss to `--gain-loss-account-id`; the gain/loss account must be `REVENUE` for gains and `EXPENSE` for losses. Asset CSV imports require `name`, `purchase_date`, and `purchase_cost`; optional columns include `asset_number`, `category_id`, `category_name`, `status`, depreciation/book-value fields, disposal fields, account IDs, and account-code columns `asset_account_code`, `depreciation_expense_account_code`, and `accumulated_depreciation_account_code`; ID columns must be valid UUIDs.

## Inventory

```bash
go run ./cmd/oa inventory categories list
go run ./cmd/oa inventory categories create --name Parts --description "Spare parts"
go run ./cmd/oa inventory categories import --file ./categories.csv
go run ./cmd/oa inventory categories get --id <category-id>
go run ./cmd/oa inventory categories delete --id <category-id>

go run ./cmd/oa inventory products list --type GOODS --status ACTIVE
go run ./cmd/oa inventory products list --category-id <category-id> --low-stock
go run ./cmd/oa inventory products create \
  --code PRD-001 \
  --name Widget \
  --type GOODS \
  --category-id <category-id> \
  --unit pcs \
  --purchase-price 10.50 \
  --sales-price 15.00 \
  --vat-rate 22.00 \
  --min-stock-level 5 \
  --reorder-point 7
go run ./cmd/oa inventory products import --file ./products.csv --json
go run ./cmd/oa inventory products get --id <product-id>
go run ./cmd/oa inventory products update --id <product-id> --name Widget --sales-price 16.00
go run ./cmd/oa inventory products stock-levels --id <product-id> --json
go run ./cmd/oa inventory products movements --id <product-id> --json
go run ./cmd/oa inventory products delete --id <product-id> --json
go run ./cmd/oa inventory valuation
go run ./cmd/oa inventory valuation --warehouse-id <warehouse-id>
go run ./cmd/oa inventory valuation --method weighted-average
go run ./cmd/oa inventory valuation --method fifo
go run ./cmd/oa inventory lots --product-id <product-id> --warehouse-id <warehouse-id>
go run ./cmd/oa inventory lots --warehouse-id <warehouse-id> --include-empty --json

go run ./cmd/oa inventory warehouses list --active-only
go run ./cmd/oa inventory warehouses create --code MAIN --name "Main warehouse" --address Tallinn --default
go run ./cmd/oa inventory warehouses import --file ./warehouses.csv
go run ./cmd/oa inventory warehouses get --id <warehouse-id>
go run ./cmd/oa inventory warehouses update --id <warehouse-id> --name "Main warehouse" --active true
go run ./cmd/oa inventory warehouses delete --id <warehouse-id>

go run ./cmd/oa inventory adjust --product-id <product-id> --warehouse-id <warehouse-id> --quantity -2 --unit-cost 10.50 --lot-number LOT-2026-01 --serial-number SN-001 --expiry-date 2027-01-31 --reason "Cycle count"
go run ./cmd/oa inventory stock import --file ./stock.csv
go run ./cmd/oa inventory transfer --product-id <product-id> --from-warehouse-id <warehouse-id> --to-warehouse-id <warehouse-id> --quantity 3 --lot-number LOT-2026-01 --serial-number SN-001 --expiry-date 2027-01-31
go run ./cmd/oa inventory reserve --product-id <product-id> --warehouse-id <warehouse-id> --quantity 2 --reason "Sales order allocation"
go run ./cmd/oa inventory release --product-id <product-id> --warehouse-id <warehouse-id> --quantity 1 --reason "Order canceled"
```

Product types are `GOODS` and `SERVICE`; product statuses are `ACTIVE` and `INACTIVE`. Product lists accept `--type`, `--status`, `--category-id`, `--search`, `--low-stock`, and `--json`. Product create/update commands require `--name` and `--sales-price`; price, VAT, stock threshold, reorder, and lead-time flags are validated before requests are sent. Product IDs, category IDs, account IDs, supplier IDs, barcodes, names, descriptions, and units are trimmed before requests are sent. Product category CSV imports require `name`, can preserve optional UUIDs from `id`, `category_id`, or `product_category_id`, and can resolve parents from `parent_id`, `parent_category_id`, or `parent_name`; `migration validate` treats parent/category ID columns as UUID/preserved-ID references and name columns as name references, and also rejects blank category names plus same-file `parent_name` references that point to later rows. Product CSV imports require `name` and `sales_price`; optional columns include `code`, `product_type`, `category_id`, `category_name`, prices, VAT, reorder settings, account IDs, account-code columns `sale_account_code`, `purchase_account_code`, and `inventory_account_code`, inventory tracking, status, barcode, supplier, and lead time. Product imports generate new UUIDs rather than preserving `id` or `product_id` values; supplied product codes are preserved and are the same-bundle lookup key for product lines and stock. Product `category_id` values must be valid UUIDs for existing or already imported product categories; invalid or missing category IDs are returned as row-level import errors. Product import JSON includes row-level errors when rows are skipped. Warehouse CSV imports require `code` and `name`; optional columns include `address`, `is_default`, `status`, and `is_active`. Warehouse imports generate new UUIDs rather than preserving `id` or `warehouse_id` values; supplied warehouse codes are preserved and are the same-bundle lookup key for stock. Use `--json` on inventory read and mutation commands for automation. `inventory valuation` returns stock value for tracked goods with optional warehouse filtering; methods are `standard-cost` (default, using each product purchase price), `weighted-average` (using inbound stock movement costs), and `fifo` (valuing current quantity from newest remaining inbound layers), falling back to purchase price when no costed movements exist. `inventory lots` returns tracked goods grouped by product, warehouse, lot number, serial number, and expiry date; filters are `--product-id` and `--warehouse-id`, and `--include-empty` includes zero or negative lot positions. `inventory adjust` accepts signed quantities; positive quantities add stock and negative quantities remove stock while updating both product total stock and the selected warehouse stock level. Adjustments can also capture optional lot number, serial number, and expiry date metadata on the resulting stock movement. `inventory stock import` accepts `product_id` or `product_code`, `warehouse_id` or `warehouse_code`, signed `quantity`, optional `unit_cost`, optional `lot_number`, `serial_number`, `expiry_date`, and optional `reason`; ID columns are UUIDs, while `product_code` and `warehouse_code` can be checked against same-bundle product and warehouse imports during migration preflight. `lot`, `batch`, `serial`, `expiration_date`, and `description` are accepted CSV aliases. `inventory transfer` requires a positive quantity and sufficient source warehouse availability, then moves stock between warehouse levels without changing product total stock; optional lot number, serial number, and expiry date metadata are copied to both transfer movements. `inventory reserve` moves a positive quantity from available to reserved stock, and `inventory release` moves a positive quantity from reserved back to available stock.

## Cost centers

```bash
go run ./cmd/oa cost-centers list --active-only
go run ./cmd/oa cost-centers create \
  --code CC001 \
  --name Sales \
  --budget-amount 1000.00 \
  --budget-period MONTHLY
go run ./cmd/oa cost-centers import --file ./cost-centers.csv
go run ./cmd/oa cost-centers get --id <cost-center-id>
go run ./cmd/oa cost-centers update \
  --id <cost-center-id> \
  --code CC001 \
  --name Sales \
  --budget-amount 1200.00
go run ./cmd/oa cost-centers allocations create \
  --cost-center-id <cost-center-id> \
  --journal-entry-line-id <journal-entry-line-id> \
  --amount 125.50 \
  --allocation-percentage 50 \
  --allocation-date 2026-03-20 \
  --notes "Shared office expense"
go run ./cmd/oa cost-centers allocations import --file ./cost-allocations.csv
go run ./cmd/oa cost-centers allocations list --cost-center-id <cost-center-id> --start 2026-03-01 --end 2026-03-31
go run ./cmd/oa cost-centers report --start 2026-03-01 --end 2026-03-31 --csv --output cost-centers.csv
go run ./cmd/oa cost-centers delete --id <cost-center-id>
```

Budget periods are `MONTHLY`, `QUARTERLY`, and `ANNUAL`. Cost center CSV imports require `code` and `name`, with optional `parent_id`, `parent_code`, `budget_amount`, `budget_period`, `status`, and `is_active`; `parent_id` must be an existing cost-center UUID, while `parent_code` can reference existing cost centers or earlier import rows. Imports generate new UUIDs and preserve codes for downstream lookup. Cost allocations assign positive journal-entry-line amounts to cost centers and can be filtered by cost center, journal entry line, and allocation date range. Cost allocation CSV imports require a UUID `journal_entry_line_id`, `amount`, `allocation_date`, and either an existing UUID in `cost_center_id` or a resolvable `cost_center_code`; optional columns include `allocation_percentage` and `notes`. Cost center reports support `--csv`, `--xlsx`, `--pdf`, and `--output`. Use `--json` on cost-center read, mutation, and import commands for automation.

## Analytics

```bash
go run ./cmd/oa analytics dashboard
go run ./cmd/oa analytics revenue-expense --months 6
go run ./cmd/oa analytics cash-flow --months 6
go run ./cmd/oa analytics activity --limit 20
```

Analytics commands are read-only and support `--json` for dashboards, chart data, and activity feeds.

## Payments

```bash
go run ./cmd/oa payments list
go run ./cmd/oa payments list --type RECEIVED --method BANK_TRANSFER --from 2026-03-01 --to 2026-03-31
go run ./cmd/oa payments create \
  --type RECEIVED \
  --amount 1220.00 \
  --date 2026-03-15 \
  --contact-id <contact-id> \
  --method BANK_TRANSFER \
  --reference BANK-001 \
  --allocate <invoice-id>:1220.00
go run ./cmd/oa payments import --file ./payments.csv --json
go run ./cmd/oa payments sepa-export \
  --message-id MSG-20260331 \
  --debtor-name "Example OU" \
  --debtor-iban EE382200221020145685 \
  --debtor-bic HABAEE2X \
  --execution-date 2026-04-01 \
  --line "name=Supplier AS,iban=EE471000001020145685,amount=125.50,remittance=Invoice INV-1001" \
  --output ./sepa-payments.xml
go run ./cmd/oa payments sepa-export \
  --debtor-name "Example OU" \
  --debtor-iban EE382200221020145685 \
  --execution-date 2026-04-01 \
  --line "name=Supplier AS,iban=EE471000001020145685,amount=125.50"
go run ./cmd/oa payments get --id <payment-id> --json
go run ./cmd/oa payments allocate --id <payment-id> --invoice-id <invoice-id> --amount 250.00 --json
go run ./cmd/oa payments reverse --id <payment-id> --reason "Duplicate bank import" --date 2026-03-20 --json
go run ./cmd/oa payments unallocated --type RECEIVED --json
```

Payment list filters accept `--type RECEIVED|MADE`, `--method`, `--contact-id`, `--from`, and `--to`; date filters must use `YYYY-MM-DD`. Payment create requires `--type` and a positive `--amount`; `--exchange-rate` and allocation amounts must also be positive. Contact IDs, bank accounts, references, notes, payment IDs, invoice IDs, reversal fields, and SEPA debtor/creditor fields are trimmed before requests are sent, and currencies are normalized to uppercase. Use `--allocate invoice-id:amount` repeatedly on `payments create` to allocate a new payment to multiple invoices. Use `payments reverse` to create an auditable offsetting payment instead of deleting payment history; allocated reversals mirror invoice allocations and reduce invoice paid amounts. Payment CSV imports require `payment_type`, `payment_date`, and `amount`, with optional `payment_number`, `contact_id`, `currency`, `exchange_rate`, `payment_method`, `bank_account`, `reference`, `notes`, `invoice_id`, `invoice_number`, and `allocation_amount`; `contact_id` and direct `invoice_id` values must be valid UUIDs. `customer_id` and `supplier_id` are accepted as `contact_id`, `method` as `payment_method`, `description` as `notes`, and `invoice_no` as `invoice_number`. JSON import output includes row-level errors when rows are skipped. `invoice_id` can target UUIDs preserved by invoice import, while `invoice_number` allocations are resolved through the tenant invoice list before storing the allocation. Payment types are `RECEIVED` and `MADE`; `--json` is available on list, create, import, get, allocate, reverse, and unallocated commands. `payments sepa-export` writes ISO 20022 `pain.001.001.03` XML for manual bank upload; omit `--output` to stream the XML to stdout. Repeat `--line` with comma-separated `key=value` pairs including `name`, `iban`, and `amount`, plus optional `bic`, `end_to_end_id`, `currency`, `remittance`, `invoice_id`, `payment_id`, or `payment_number`.

## Payment reminders

```bash
go run ./cmd/oa reminders overdue
go run ./cmd/oa reminders send --invoice-id <invoice-id> --message "Please pay"
go run ./cmd/oa reminders send-bulk --invoice-id <invoice-id> --invoice-id <invoice-id>
go run ./cmd/oa reminders history --invoice-id <invoice-id>

go run ./cmd/oa reminders rules list
go run ./cmd/oa reminders rules create \
  --name "Seven days overdue" \
  --trigger-type AFTER_DUE \
  --days-offset 7 \
  --template-type OVERDUE_REMINDER \
  --active true
go run ./cmd/oa reminders rules get --id <rule-id>
go run ./cmd/oa reminders rules update --id <rule-id> --name "Updated reminder" --active false
go run ./cmd/oa reminders rules delete --id <rule-id>
go run ./cmd/oa reminders rules trigger
```

Reminder trigger types are `BEFORE_DUE`, `ON_DUE`, and `AFTER_DUE`. Use `--json` on reminder reads and mutations for automation.

## Email

```bash
go run ./cmd/oa email smtp get
go run ./cmd/oa email smtp update \
  --host smtp.example.com \
  --port 587 \
  --username robot \
  --password 'smtp-password' \
  --from-email billing@example.com \
  --from-name Billing \
  --use-tls=true
go run ./cmd/oa email smtp test --recipient-email you@example.com

go run ./cmd/oa email templates list
go run ./cmd/oa email templates update \
  --type OVERDUE_REMINDER \
  --subject "Reminder for {{.InvoiceNumber}}" \
  --body-html-file ./overdue-reminder.html \
  --body-text-file ./overdue-reminder.txt \
  --active true

go run ./cmd/oa email log --limit 25
go run ./cmd/oa email invoice --invoice-id <invoice-id> --recipient-email billing@example.com --attach-pdf
go run ./cmd/oa email quote --quote-id <quote-id> --recipient-email billing@example.com --attach-pdf
go run ./cmd/oa email quote --quote-id <quote-id> --recipient-email billing@example.com --require-approved-evidence
go run ./cmd/oa email order --order-id <order-id> --recipient-email billing@example.com --attach-pdf
go run ./cmd/oa email order --order-id <order-id> --recipient-email billing@example.com --require-approved-evidence
go run ./cmd/oa email payment-receipt --payment-id <payment-id> --recipient-email billing@example.com
go run ./cmd/oa email payment-receipt --payment-id <payment-id> --recipient-email billing@example.com --require-approved-evidence
```

Template types are `INVOICE_SEND`, `QUOTE_SEND`, `ORDER_CONFIRM`, `PAYMENT_RECEIPT`, and `OVERDUE_REMINDER`. `email log` requires a positive `--limit` and supports `--json` for delivery-log automation. `email invoice`, `email quote`, `email order`, and `email payment-receipt` require the entity id plus `--recipient-email`; each can print the send result as JSON, including the email log id. Quote and order emails can attach generated PDFs with `--attach-pdf` and can require approved `contract` or `supporting_document` evidence with `--require-approved-evidence`. Payment receipt emails can require at least one approved `receipt`, `supporting_document`, or `tax_support` document attached to the payment by passing `--require-approved-evidence`. Use `--json` on email reads and mutations for automation.

## Interest

```bash
go run ./cmd/oa interest settings get
go run ./cmd/oa interest settings update --rate 0.0005
go run ./cmd/oa interest settings update --annual-rate 0.1825
go run ./cmd/oa interest overdue
go run ./cmd/oa interest invoice --invoice-id <invoice-id>
go run ./cmd/oa interest history --invoice-id <invoice-id>
```

The interest rate is a daily decimal rate (`0.0005` = 0.05% daily). `--annual-rate` divides the provided annual decimal rate by 365 before sending it to the API. Use `--json` on interest commands for automation.

## Period close

```bash
go run ./cmd/oa close events --limit 20
go run ./cmd/oa close period --period-end 2026-03-31 --note "March close"
go run ./cmd/oa close period --period-end 2025-12-31 --note "Year-end reviewed" --reviewer-sign-off
go run ./cmd/oa close reopen --period-end 2026-03-31 --note "Correcting late supplier invoice"
go run ./cmd/oa close year-end-status --period-end 2025-12-31
go run ./cmd/oa close year-end-pack --period-end 2025-12-31
go run ./cmd/oa close year-end-audit --period-end 2025-12-31
go run ./cmd/oa close year-end-archive --period-end 2025-12-31 --output ./year-end-audit.zip
go run ./cmd/oa close carry-forward --period-end 2025-12-31
go run ./cmd/oa close reverse-carry-forward --period-end 2025-12-31 --reason "Late supplier accrual"
```

Period close and reopen operations require a user role that can manage close workflows. Fiscal-year close requires `--reviewer-sign-off` plus approved `close_pack` evidence attached to the `year_end_close` entity printed by `close year-end-status` or `close year-end-pack`; carry-forward requires that approved close-pack evidence too. Reopening requires a note, and fiscal-year periods cannot be reopened after carry-forward has been posted unless the carry-forward is explicitly reversed first. `close year-end-pack` returns the readiness status plus year-end trial balance, balance sheet, and income statement. `close year-end-audit` adds close-pack evidence-policy status and attached close-pack document metadata for auditor handoff. `close year-end-archive` downloads a ZIP with `manifest.json` and attached close-pack files. Use `--json` for automation where available.

## Banking

```bash
go run ./cmd/oa banking accounts list --active-only
go run ./cmd/oa banking accounts create \
  --name "Main bank" \
  --account-number EE471000001020145685 \
  --bank-name LHV \
  --swift-code LHVBEE22 \
  --currency EUR \
  --gl-account-id <asset-account-id> \
  --default
go run ./cmd/oa banking accounts import --file ./bank-accounts.csv
go run ./cmd/oa banking accounts get --id <bank-account-id>
go run ./cmd/oa banking accounts update --id <bank-account-id> --bank-name SEB --active true
go run ./cmd/oa banking accounts delete --id <bank-account-id>

go run ./cmd/oa banking match-rules list --bank-account-id <bank-account-id> --active-only --include-global
go run ./cmd/oa banking match-rules create \
  --name "Stripe receipts" \
  --bank-account-id <bank-account-id> \
  --priority 10 \
  --field DESCRIPTION \
  --pattern stripe \
  --min-confidence 0.85 \
  --max-date-diff-days 3 \
  --require-exact-amount
go run ./cmd/oa banking match-rules get --id <rule-id>
go run ./cmd/oa banking match-rules update --id <rule-id> --global --active false
go run ./cmd/oa banking match-rules delete --id <rule-id>

go run ./cmd/oa banking transactions list \
  --account-id <bank-account-id> \
  --status UNMATCHED \
  --from 2026-03-01 \
  --to 2026-03-31
go run ./cmd/oa banking transactions import --account-id <bank-account-id> --file ./lhv-bank.csv --format lhv
go run ./cmd/oa banking transactions import --account-id <bank-account-id> --file ./ACCOUNT_STATEMENT_CAMT.053A.xml --format camt053
go run ./cmd/oa banking transactions import-history --account-id <bank-account-id>
go run ./cmd/oa banking transactions get --id <transaction-id>
go run ./cmd/oa banking transactions suggestions --id <transaction-id>
go run ./cmd/oa banking transactions match --id <transaction-id> --payment-id <payment-id>
go run ./cmd/oa banking transactions unmatch --id <transaction-id>
go run ./cmd/oa banking transactions review \
  --id <transaction-id> \
  --follow-up-status EVIDENCE_REQUIRED \
  --review-note "Request receipt"
go run ./cmd/oa banking transactions create-payment --id <transaction-id>
go run ./cmd/oa banking transactions auto-match --account-id <bank-account-id> --min-confidence 0.80

go run ./cmd/oa banking reconciliations list --account-id <bank-account-id>
go run ./cmd/oa banking reconciliations create \
  --account-id <bank-account-id> \
  --statement-date 2026-03-31 \
  --opening-balance 0.00 \
  --closing-balance 100.00
go run ./cmd/oa banking reconciliations get --id <reconciliation-id>
go run ./cmd/oa banking reconciliations complete --id <reconciliation-id>
```

Bank account creation requires `--name` and `--account-number`; optional values are trimmed, currencies are normalized to uppercase, and `--default` marks the account as the tenant default. Bank account updates require `--id`; `--active` and `--default` accept `true` or `false`. Bank account CSV imports default to `--skip-duplicates=true`; pass `--skip-duplicates=false` when duplicate account numbers should fail instead of being skipped. CSV imports require `name` and `account_number`; accepted aliases include `account_name` for `name`, `iban`/`account` for `account_number`, `bank` for `bank_name`, `bic` for `swift_code`, `ledger_account_id` for `gl_account_id`, `gl_account_code`/`ledger_account_code`/`cash_account_code` for ledger account-code resolution, `default` for `is_default`, and `active` for `is_active`. Use `--json` on `banking accounts` read and mutation commands for automation.

Bank transaction statuses are `UNMATCHED`, `MATCHED`, and `RECONCILED`. Follow-up statuses are `NONE`, `EVIDENCE_REQUIRED`, and `READY_TO_MATCH`. Auto-match rule fields are `DESCRIPTION`, `REFERENCE`, `COUNTERPARTY_NAME`, and `COUNTERPARTY_ACCOUNT`; omit `--bank-account-id` or pass `--global` on update for tenant-wide rules. Reconciliation completion blocks matched transactions marked `EVIDENCE_REQUIRED` until they have approved `reconciliation_evidence` documents; use `documents upload`, `documents review`, and `documents evidence-policy` to resolve evidence failures. Bank transaction CSV imports accept comma, semicolon, or tab delimiters. Use `--format lhv` for LHV Internet Bank account statement CSV exports with the documented 2026 columns: `Client account`, `Document number`, `Date`, `Beneficiary's/remitter's account`, `Beneficiary's/remitter's name`, `Debit/Credit (D/C)`, `Amount`, `Reference number`, `Archival ID`, `Details`, `Currency`, personal or registry code, counterparty bank BIC, payment initiator name, `Entry reference`, and `Account service provider's reference`. Use `--format camt053` for ISO 20022 camt.053 account statement XML; `lhv-camt` remains accepted as an LHV compatibility alias. The parser is covered against LHV Connect's current Account Statement `Statement data` sample. `--format auto` detects LHV CSV and camt.053 XML layouts and otherwise uses the generic headers `date`, `amount`, `currency`, `source_account`, `description`, `reference`, `counterparty_name`, `counterparty_account`, `value_date`, and `external_id`. Imports reject rows whose statement account or currency does not match the selected bank account. Use `--json` on banking read and mutation commands for automation.

## Reports

```bash
go run ./cmd/oa reports trial-balance --as-of 2026-03-31
go run ./cmd/oa reports trial-balance --as-of 2026-03-31 --csv --output ./trial-balance.csv
go run ./cmd/oa reports trial-balance --as-of 2026-03-31 --xlsx --output ./trial-balance.xlsx
go run ./cmd/oa reports trial-balance --as-of 2026-03-31 --pdf --output ./trial-balance.pdf
go run ./cmd/oa reports account-balance --account-id <account-id> --as-of 2026-03-31
go run ./cmd/oa reports account-balance --account-id <account-id> --as-of 2026-03-31 --csv --output ./account-balance.csv
go run ./cmd/oa reports account-balance --account-id <account-id> --as-of 2026-03-31 --xlsx --output ./account-balance.xlsx
go run ./cmd/oa reports account-balance --account-id <account-id> --as-of 2026-03-31 --pdf --output ./account-balance.pdf
go run ./cmd/oa reports balance-sheet --as-of 2026-03-31
go run ./cmd/oa reports balance-sheet --as-of 2026-03-31 --csv --output ./balance-sheet.csv
go run ./cmd/oa reports balance-sheet --as-of 2026-03-31 --xlsx --output ./balance-sheet.xlsx
go run ./cmd/oa reports balance-sheet --as-of 2026-03-31 --pdf --output ./balance-sheet.pdf
go run ./cmd/oa reports income-statement --start 2026-01-01 --end 2026-03-31
go run ./cmd/oa reports income-statement --start 2026-01-01 --end 2026-03-31 --csv --output ./income-statement.csv
go run ./cmd/oa reports income-statement --start 2026-01-01 --end 2026-03-31 --xlsx --output ./income-statement.xlsx
go run ./cmd/oa reports income-statement --start 2026-01-01 --end 2026-03-31 --pdf --output ./income-statement.pdf
go run ./cmd/oa reports consolidated --as-of 2026-12-31 --start 2026-01-01 --end 2026-12-31 --tenant-ids tenant-a,tenant-b
go run ./cmd/oa reports annual --period-end 2026-12-31 --cash-flow-method indirect
go run ./cmd/oa reports cash-flow --start 2026-01-01 --end 2026-03-31
go run ./cmd/oa reports cash-flow --start 2026-01-01 --end 2026-03-31 --method indirect
go run ./cmd/oa reports cash-flow --start 2026-01-01 --end 2026-03-31 --investing-accounts CAPEX-1,CAPEX-2
go run ./cmd/oa reports cash-flow-mapping get
go run ./cmd/oa reports cash-flow-mapping update --operating-accounts PREPAY --investing-accounts CAPEX-1,CAPEX-2 --financing-accounts FOUNDERS
go run ./cmd/oa reports cash-flow --start 2026-01-01 --end 2026-03-31 --csv --output ./cash-flow.csv
go run ./cmd/oa reports cash-flow --start 2026-01-01 --end 2026-03-31 --xlsx --output ./cash-flow.xlsx
go run ./cmd/oa reports cash-flow --start 2026-01-01 --end 2026-03-31 --pdf --output ./cash-flow.pdf
go run ./cmd/oa reports aging --type receivables
go run ./cmd/oa reports aging --type receivables --csv --output ./receivables-aging.csv
go run ./cmd/oa reports aging --type payables --xlsx --output ./payables-aging.xlsx
go run ./cmd/oa reports aging --type receivables --pdf --output ./receivables-aging.pdf
go run ./cmd/oa reports aging --type payables --json
go run ./cmd/oa reports balance-confirmations --type RECEIVABLE --as-of 2026-03-31
go run ./cmd/oa reports balance-confirmations --type RECEIVABLE --as-of 2026-03-31 --xlsx --output ./balance-confirmations.xlsx
go run ./cmd/oa reports balance-confirmations --type RECEIVABLE --as-of 2026-03-31 --pdf --output ./balance-confirmations.pdf
go run ./cmd/oa reports balance-confirmation \
  --contact-id <contact-id> \
  --type RECEIVABLE \
  --as-of 2026-03-31
go run ./cmd/oa reports balance-confirmation \
  --contact-id <contact-id> \
  --type RECEIVABLE \
  --as-of 2026-03-31 \
  --csv \
  --output ./balance-confirmation.csv
go run ./cmd/oa reports balance-confirmation \
  --contact-id <contact-id> \
  --type RECEIVABLE \
  --as-of 2026-03-31 \
  --pdf \
  --output ./balance-confirmation.pdf
go run ./cmd/oa reports contact-statement \
  --contact-id <contact-id> \
  --type RECEIVABLE \
  --start 2026-01-01 \
  --end 2026-03-31
go run ./cmd/oa reports contact-statement \
  --contact-id <contact-id> \
  --type PAYABLE \
  --start 2026-01-01 \
  --end 2026-03-31 \
  --csv \
  --output ./vendor-statement.csv
go run ./cmd/oa reports contact-statement \
  --contact-id <contact-id> \
  --type RECEIVABLE \
  --start 2026-01-01 \
  --end 2026-03-31 \
  --pdf \
  --output ./customer-statement.pdf
go run ./cmd/oa reports sales-margin --start 2026-01-01 --end 2026-03-31
go run ./cmd/oa reports sales-margin --start 2026-01-01 --end 2026-03-31 --xlsx --output ./sales-margin.xlsx
go run ./cmd/oa reports sales-margin --start 2026-01-01 --end 2026-03-31 --pdf --output ./sales-margin.pdf
go run ./cmd/oa reports customer-profitability --start 2026-01-01 --end 2026-03-31
go run ./cmd/oa reports customer-profitability --start 2026-01-01 --end 2026-03-31 --xlsx --output ./customer-profitability.xlsx
go run ./cmd/oa reports customer-profitability --start 2026-01-01 --end 2026-03-31 --pdf --output ./customer-profitability.pdf
go run ./cmd/oa reports budget-vs-actual --start 2026-03-01 --end 2026-03-31
go run ./cmd/oa reports budget-vs-actual --start 2026-03-01 --end 2026-03-31 --csv --output ./budget-vs-actual.csv
go run ./cmd/oa reports budget-vs-actual --start 2026-03-01 --end 2026-03-31 --xlsx --output ./budget-vs-actual.xlsx
go run ./cmd/oa reports budget-vs-actual --start 2026-03-01 --end 2026-03-31 --pdf --output ./budget-vs-actual.pdf
```

Every report command supports `--json` for automation. Choose only one output mode per report command: `--json`, `--csv`, `--xlsx`, and `--pdf` cannot be combined, and `--output` is valid only with `--csv`, `--xlsx`, or `--pdf`. `reports consolidated` combines trial balance, balance sheet, and income statement totals across selected tenant IDs the authenticated user can view; tenant-scoped API tokens can only consolidate their own tenant. `reports annual` combines year-end close status, trial balance, balance sheet, income statement, and cash flow for a fiscal year. `reports cash-flow --method` accepts `direct` or `indirect`; indirect operating cash flow starts with net income and adjusts for depreciation/amortization plus receivables, inventory, and payables changes. Cash-flow account mapping can be saved with `reports cash-flow-mapping update` or overridden per request with comma-separated `--operating-accounts`, `--investing-accounts`, and `--financing-accounts` for custom charts. Request-level overrides take precedence over saved mappings. Trial-balance, account-balance, balance-sheet, income-statement, cash-flow, aging, balance-confirmations, balance-confirmation, contact-statement, sales-margin, customer-profitability, and budget-vs-actual commands support backend CSV export with `--csv`, XLSX export with `--xlsx`, and PDF export with `--pdf`; omit `--output` to stream the export bytes to stdout. Contact statements show one customer or supplier's opening balance, period invoices, period payments, and closing balance. Sales margin uses sales invoice line revenue and product purchase prices to estimate line cost and margin. Customer profitability presents those same product-cost-backed margins as customer rollups with supporting invoice-line detail. Budget-vs-actual compares cost-center actual expenses against configured budgets and marks over-budget centers.

## Documents

```bash
go run ./cmd/oa documents list --entity-type payment --entity-id <payment-id>
go run ./cmd/oa documents upload \
  --entity-type bank_transaction \
  --entity-id <transaction-id> \
  --file ./statement-line.pdf \
  --document-type reconciliation_evidence \
  --notes "Matched against March bank statement" \
  --retention-years 7
go run ./cmd/oa documents upload \
  --entity-type year_end_close \
  --entity-id <close-pack-evidence-entity-id> \
  --file ./close-pack.pdf \
  --document-type close_pack \
  --notes "Approved fiscal-year close pack"
go run ./cmd/oa documents upload \
  --entity-type quote \
  --entity-id <quote-id> \
  --file ./signed-offer.pdf \
  --document-type contract
go run ./cmd/oa documents review-queue \
  --entity-type year_end_close \
  --document-type close_pack \
  --status PENDING
go run ./cmd/oa documents review-summary --entity-type payment --entity-id <payment-id> --entity-id <payment-id>
go run ./cmd/oa documents evidence-policy --entity-type payment --entity-id <payment-id> --document-type receipt --require-approved
go run ./cmd/oa documents retention --as-of 2027-03-01 --horizon-days 45 --include-missing
go run ./cmd/oa documents retention-set --id <document-id> --retention-until 2028-03-31
go run ./cmd/oa documents retention-set --id <document-id> --clear
go run ./cmd/oa documents download --id <document-id> --output ./document.pdf
go run ./cmd/oa documents review --id <document-id> --status APPROVED --note "Evidence accepted"
go run ./cmd/oa documents review --id <document-id> --status REJECTED --note "Receipt does not match"
go run ./cmd/oa documents mark-reviewed --id <document-id>
go run ./cmd/oa documents delete --id <document-id>
```

`documents upload` accepts either `--retention-until YYYY-MM-DD` or `--retention-years N` up to `100`; `--retention-years` sets `retention_until` to the upload date plus the selected number of years, and the two retention flags cannot be combined. Supported entity types include invoices, journal entries, payments, bank transactions, fixed assets, expenses, quotes, orders, year-end close packs, and leave records. `documents review-queue` returns a tenant-wide reviewer queue, defaulting to `PENDING` documents; filter by `--entity-type year_end_close --document-type close_pack` for fiscal-year close-pack approvals or `--entity-type leave_record` for leave evidence approvals, and use `--status all` for audit review. `documents evidence-policy` checks required evidence for one or more entity IDs. Repeat `--document-type` or `--required-document-type` to allow several document types in the rule, set `--min-count` for the required count, and use `--require-approved` when pending or reviewed-but-unapproved evidence must fail. `documents retention` returns a tenant-wide queue of documents whose `retention_until` is due by the cutoff, with optional missing-retention records. `documents retention-set` corrects one document's `retention_until` date or clears it with `--clear`. `documents review` supports `REVIEWED`, `APPROVED`, and `REJECTED`; rejected documents require a review note. `documents download` uses the server-provided filename when `--output` is omitted. Use `--output -` to stream the document content to stdout.

## Journal entries

```bash
go run ./cmd/oa journal list --limit 50
go run ./cmd/oa journal create \
  --entry-date 2026-03-31 \
  --description "Manual accrual" \
  --reference ACC-1 \
  --requires-evidence \
  --line "account_id=<expense-account-id>,description=Expense,debit=100.00,currency=USD,exchange_rate=0.92" \
  --line "account_id=<accrual-account-id>,description=Accrual,credit=100.00,currency=USD,exchange_rate=0.92"
go run ./cmd/oa journal get --id <journal-entry-id>
go run ./cmd/oa journal post --id <journal-entry-id>
go run ./cmd/oa journal void --id <journal-entry-id> --reason "Duplicate entry"
go run ./cmd/oa journal import --file ./journal-entries.csv --source-type LEGACY_GL --post
go run ./cmd/oa journal templates list --active-only
go run ./cmd/oa journal templates create \
  --name "Monthly rent accrual" \
  --description "Monthly rent accrual" \
  --reference RENT \
  --requires-evidence \
  --frequency MONTHLY \
  --start-date 2026-04-30 \
  --end-date 2026-12-31 \
  --next-generation-date 2026-05-31 \
  --line "account_id=<expense-account-id>,description=Rent expense,debit=500.00" \
  --line "account_id=<accrual-account-id>,description=Accrued rent,credit=500.00"
go run ./cmd/oa journal templates get --id <template-id>
go run ./cmd/oa journal templates apply \
  --id <template-id> \
  --entry-date 2026-04-30 \
  --description "April rent accrual" \
  --reference RENT-APR \
  --post
go run ./cmd/oa journal templates generate --id <template-id> --entry-date 2026-05-31 --post
go run ./cmd/oa journal templates generate-due --as-of 2026-05-31 --post
```

Use `--line` repeatedly on `journal create`. Each line is comma-separated `key=value` pairs with `account_id` and exactly one of `debit` or `credit`; optional keys include `description`, `currency`, and positive `exchange_rate`. Omitted currency defaults to `EUR` and omitted exchange rate defaults to `1`; journal entries balance on base-currency debit/credit totals. Use `--requires-evidence` for manual adjustments that must have approved `supporting_document`, `receipt`, or `tax_support` evidence attached before posting.
`journal templates create` uses the same `--line` syntax. Add `--frequency` with `--start-date` for recurring templates; optional `--end-date` and `--next-generation-date` bound or override the schedule. Supported frequencies are `WEEKLY`, `BIWEEKLY`, `MONTHLY`, `QUARTERLY`, and `YEARLY`. Use `--requires-evidence` when generated entries must stay in draft until approved evidence is attached. `journal templates apply` creates an on-demand entry without advancing the schedule. `journal templates generate` advances one recurring template and accepts `--entry-date` to override the generated date; `generate-due` advances all due recurring templates. Pass `--post` only when generated entries should be posted immediately.
`journal import` expects grouped CSV rows with `entry_reference`, `entry_date`, `account_code`, `debit`, and `credit`; optional columns include `entry_description`, `line_description`, `currency`, `exchange_rate`, `source_type`, and `source_id`. `source_id` must be a valid UUID when supplied.

## Opening balances

```bash
go run ./cmd/oa journal import-opening-balances \
  --file ./opening-balances.csv \
  --entry-date 2026-01-01 \
  --reference OB-2026
```

## Example CSV shapes

The CSV importers accept comma, semicolon, or tab delimiters.

### Accounts

```csv
code,name,account_type,description,parent_code
1000,Cash,ASSET,Cash on hand,
1100,Bank Account,ASSET,Main bank account,1000
4000,Sales Revenue,REVENUE,Primary revenue,
```

### Contacts

```csv
contact_id,name,type,email,payment_terms_days,country_code,credit_limit
11111111-1111-1111-1111-111111111111,Northwind OU,CUSTOMER,ap@northwind.example,14,EE,1500.00
,Supply Partner,SUPPLIER,purchases@supply.example,30,EE,2500.00
```

### Invoices

```csv
invoice_id,invoice_number,invoice_type,contact_code,issue_date,due_date,status,amount_paid,reference,notes,line_description,quantity,unit,unit_price,discount_percent,vat_rate,vat_treatment,product_code
11111111-1111-1111-1111-111111111111,INV-EXT-001,SALES,CUST-001,2026-02-01,2026-02-15,SENT,0,PO-12345,Imported migration invoice,Implementation work,1,hour,100.00,0,22,standard,SERV-001
11111111-1111-1111-1111-111111111111,INV-EXT-001,SALES,CUST-001,2026-02-01,2026-02-15,SENT,0,PO-12345,Imported migration invoice,Support retainer,1,month,50.00,0,22,standard,RET-001
```

### Employees

```csv
employee_number,first_name,last_name,personal_code,email,start_date,employment_type,apply_basic_exemption,basic_exemption_amount,funded_pension_rate,base_salary,salary_effective_from
EMP-001,Mari,Maasikas,49001010001,mari@example.com,2026-01-15,FULL_TIME,true,700.00,0.02,3200.00,2026-01-15
EMP-002,Juhan,Tamm,49001010002,juhan@example.com,2026-02-01,PART_TIME,true,700.00,0.02,,
```

### Payroll history

```csv
period_year,period_month,status,payment_date,notes,employee_number,gross_salary,income_tax,unemployment_insurance_employee,funded_pension,other_deductions,net_salary,social_tax,unemployment_insurance_employer,total_employer_cost,basic_exemption_applied,payment_status,paid_at
2025,12,PAID,2026-01-05,Imported December payroll,EMP-001,3200.00,550.00,51.20,64.00,0.00,2534.80,1056.00,25.60,4281.60,50.00,PAID,2026-01-05
2025,12,PAID,2026-01-05,Imported December payroll,EMP-002,2800.00,420.00,44.80,56.00,10.00,2269.20,924.00,22.40,3746.40,40.00,PAID,2026-01-05
```

### Leave balances

```csv
year,employee_number,absence_type_code,entitled_days,carryover_days,used_days,pending_days,notes
2025,EMP-001,ANNUAL_LEAVE,28,2,4,0,Imported leave balance
2025,EMP-002,ANNUAL_LEAVE,28,0,10,1,Imported leave balance
```

### Opening balances

```csv
account_code,debit,credit,description
1000,1500.00,0,Cash opening balance
3000,0,1500.00,Owner equity opening balance
```

### Bank transactions

```csv
date,amount,description,reference,counterparty_name,counterparty_account,value_date,external_id
2026-03-15,100.00,Client payment,REF-1,Acme,EE111,2026-03-16,bank-ext-1
2026-03-16,-25.50,Bank fee,FEE-1,LHV,EE222,2026-03-16,bank-ext-2
```

## Automation without stored config

```bash
export OA_BASE_URL=http://localhost:8080
export OA_TENANT_ID=b0000000-0000-0000-0001-000000000001
export OA_API_TOKEN=oa_your_token_here

go run ./cmd/oa accounts list --json
go run ./cmd/oa contacts create --name "Scripted Contact" --type CUSTOMER
go run ./cmd/oa reports trial-balance --as-of 2026-03-31 --json
go run ./cmd/oa tax kmd list --json
go run ./cmd/oa tsd export-csv --year 2026 --month 3 --output ./tsd.csv
go run ./cmd/oa payroll runs list --year 2026 --json
go run ./cmd/oa employees import --file ./employees.csv
go run ./cmd/oa payroll import-history --file ./payroll-history.csv
go run ./cmd/oa payroll import-leave-balances --file ./leave-balances.csv
go run ./cmd/oa tsd import-history --file ./tsd-history.csv
go run ./cmd/oa documents upload --entity-type asset --entity-id <asset-id> --file ./warranty.pdf --document-type asset_record
```

## Notes

- Normal data commands use the stored API token, not the login password.
- API tokens are tenant-scoped. A token created for one tenant cannot be used on another tenant path.
- API tokens belong to the authenticated user that created them, stop working while that user's tenant membership is suspended, and can be revoked later.
- `auth status` verifies the stored token by calling `/api/v1/me`.
- Use `--json` on list/create/import commands if you want machine-readable output.
