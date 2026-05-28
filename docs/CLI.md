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

## Operational checks

```bash
go run ./cmd/oa health --base-url http://localhost:8080
go run ./cmd/oa demo status --base-url http://localhost:8080 --secret <demo-secret> --user 1
go run ./cmd/oa demo reset --base-url http://localhost:8080 --secret <demo-secret> --user 1
```

`demo reset` accepts `--user 1` through `--user 4` for a single seeded demo user. Omit `--user` to reset all demo users.

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
```

`OA_API_TOKEN` is useful for CI or automation where you do not want to persist local config.

## Inspect auth state

```bash
go run ./cmd/oa auth status
go run ./cmd/oa auth tenants
go run ./cmd/oa auth login --email you@example.com --password 'your-password' --tenant-id <tenant-id>
go run ./cmd/oa auth refresh --refresh-token <refresh-token> --tenant-id <tenant-id>
go run ./cmd/oa auth sessions
go run ./cmd/oa auth sessions --include-inactive
go run ./cmd/oa auth revoke-session --id <session-id>
go run ./cmd/oa auth logout --refresh-token <refresh-token>
go run ./cmd/oa auth logout
```

`auth login` and `auth refresh` print short-lived JWT tokens. `auth refresh` returns a replacement refresh token and revokes the presented refresh session. `auth sessions` lists refresh sessions for the current user, and `auth revoke-session` revokes a session by id. `auth logout --refresh-token` revokes that refresh session on the server before removing local CLI config. The normal automation flow still uses `auth init`, which stores a tenant-scoped API token.

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
```

Use `--id <tenant-id>` on `tenant get`, `tenant update`, and `tenant complete-onboarding` to target a tenant other than the configured one. Use `--settings-file ./tenant-settings.json` instead of `--settings-json` for larger settings payloads.

## Tenant users and invitations

```bash
go run ./cmd/oa users list
go run ./cmd/oa users update-role --id <user-id> --role accountant
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

Use `--password-stdin` on `invitations accept` to avoid placing a new-user password in shell history.

## Plugins

```bash
go run ./cmd/oa plugins list
go run ./cmd/oa plugins enable --id <plugin-id> --settings-json '{"enabled":true}'
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

Use `--permission` repeatedly when enabling an instance-level plugin with multiple permissions.

## Manage API tokens

```bash
go run ./cmd/oa tokens list
go run ./cmd/oa tokens create --name "CI automation" --expires-in-days 90
go run ./cmd/oa tokens revoke --id <token-id>
```

`tokens create` returns the raw token once. Store it immediately if you need to use it outside the CLI config flow.

## Accounts

```bash
go run ./cmd/oa accounts list
go run ./cmd/oa accounts list --active-only
go run ./cmd/oa accounts create --code 1100 --name Cash --type ASSET
go run ./cmd/oa accounts get --id <account-id>
go run ./cmd/oa accounts import --file ./accounts.csv
```

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
go run ./cmd/oa employees import --file ./employees.csv
```

## Payroll runs

```bash
go run ./cmd/oa payroll runs list
go run ./cmd/oa payroll runs list --year 2026
go run ./cmd/oa payroll runs create --year 2026 --month 3 --payment-date 2026-03-31
go run ./cmd/oa payroll runs get --id <payroll-run-id>
go run ./cmd/oa payroll runs calculate --id <payroll-run-id>
go run ./cmd/oa payroll runs approve --id <payroll-run-id>
go run ./cmd/oa payroll runs payslips --id <payroll-run-id>
go run ./cmd/oa payroll tax-preview --gross-salary 3200.00
```

Use `payroll runs calculate` after employee salary setup, then `payroll runs approve` before TSD generation. Use `--json` on read and mutation commands when scripting.

## Payroll migration imports

```bash
go run ./cmd/oa payroll import-history --file ./payroll-history.csv
go run ./cmd/oa payroll import-history --file ./payroll-history.csv --json
go run ./cmd/oa payroll import-leave-balances --file ./leave-balances.csv
```

Import employees first so payroll history rows can match existing employees by `employee_number`, `personal_code`, `email`, or `first_name` + `last_name`. Existing payroll periods are skipped rather than overwritten; use `--json` when you need row-level import errors for automation.

Leave balance imports create or update balances by employee + absence type + year. Match absence types with `absence_type_code`, `absence_type`, or `absence_type_id`.

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
go run ./cmd/oa leave records approve --id <leave-record-id>
go run ./cmd/oa leave records reject --id <leave-record-id> --reason "Staffing shortage"
go run ./cmd/oa leave records cancel --id <leave-record-id>
```

Use `--json` on leave-management reads and mutations for automation. Leave record statuses are `PENDING`, `APPROVED`, `REJECTED`, and `CANCELLED`.

## TSD declarations

```bash
go run ./cmd/oa tsd list
go run ./cmd/oa tsd get --year 2026 --month 3
go run ./cmd/oa tsd generate --run-id <payroll-run-id>
go run ./cmd/oa tsd export-xml --year 2026 --month 3 --output ./tsd-2026-03.xml
go run ./cmd/oa tsd export-csv --year 2026 --month 3 --output ./tsd-2026-03.csv
go run ./cmd/oa tsd mark-submitted --year 2026 --month 3 --emta-reference EMTA-123
```

Omit `--output` on export commands to write the raw XML or CSV to stdout. Use `--json` on list/get/generate/mark-submitted for automation.

## KMD declarations

```bash
go run ./cmd/oa tax kmd list
go run ./cmd/oa tax kmd generate --year 2026 --month 3
go run ./cmd/oa tax kmd import-history --file ./kmd-history.csv
go run ./cmd/oa tax kmd import-history --file ./kmd-history.csv --json
go run ./cmd/oa tax kmd export-xml --year 2026 --month 3 --output ./kmd-2026-03.xml
```

Historical KMD import expects `year`, `month`, and `row_code` columns, plus optional `tax_base`, `tax_amount`, `status`, `submitted_at`, `description`, `total_output_vat`, and `total_input_vat` columns. Existing declaration periods are skipped instead of overwritten.

KMD export writes e-MTA XML. Omit `--output` to stream the XML to stdout.

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
go run ./cmd/oa invoices get --id <invoice-id>
go run ./cmd/oa invoices pdf --id <invoice-id> --output ./invoice.pdf
go run ./cmd/oa invoices send --id <invoice-id>
go run ./cmd/oa invoices void --id <invoice-id>
go run ./cmd/oa invoices import --file ./invoices.csv
```

Use `--line` repeatedly on `invoices create` for multi-line invoices. Each line is comma-separated `key=value` pairs with `description`, `quantity`, `unit_price`, and `vat_rate`; optional keys include `unit`, `discount_percent`, `account_id`, and `product_id`.

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
go run ./cmd/oa quotes send --id <quote-id>
go run ./cmd/oa quotes accept --id <quote-id>
go run ./cmd/oa quotes reject --id <quote-id>
go run ./cmd/oa quotes delete --id <quote-id>
```

Use `--line` repeatedly on `quotes create` and `quotes update` for multi-line offers. Each line accepts `description`, `quantity`, `unit_price`, and `vat_rate`; optional keys include `unit`, `discount_percent`, and `product_id`. Quote statuses are `DRAFT`, `SENT`, `ACCEPTED`, `REJECTED`, `EXPIRED`, and `CONVERTED`.

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
go run ./cmd/oa orders update \
  --id <order-id> \
  --contact-id <contact-id> \
  --order-date 2026-03-16 \
  --line "description=Updated consulting,quantity=3,unit=hour,unit_price=100.00,vat_rate=22.00"
go run ./cmd/oa orders confirm --id <order-id>
go run ./cmd/oa orders process --id <order-id>
go run ./cmd/oa orders ship --id <order-id>
go run ./cmd/oa orders deliver --id <order-id>
go run ./cmd/oa orders cancel --id <order-id>
go run ./cmd/oa orders delete --id <order-id>
```

Use `--line` repeatedly on `orders create` and `orders update`. Each line accepts `description`, `quantity`, `unit_price`, and `vat_rate`; optional keys include `unit`, `discount_percent`, and `product_id`. Order statuses are `PENDING`, `CONFIRMED`, `PROCESSING`, `SHIPPED`, `DELIVERED`, and `CANCELED`.

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
go run ./cmd/oa recurring-invoices get --id <recurring-id>
go run ./cmd/oa recurring-invoices update --id <recurring-id> --frequency YEARLY --payment-terms-days 30
go run ./cmd/oa recurring-invoices pause --id <recurring-id>
go run ./cmd/oa recurring-invoices resume --id <recurring-id>
go run ./cmd/oa recurring-invoices generate --id <recurring-id>
go run ./cmd/oa recurring-invoices generate-due
go run ./cmd/oa recurring-invoices delete --id <recurring-id>
```

Frequencies are `WEEKLY`, `BIWEEKLY`, `MONTHLY`, `QUARTERLY`, and `YEARLY`. Use `--line` repeatedly on create or update. Recurring invoice email options include `--send-email`, `--recipient-email`, `--attach-pdf`, `--email-subject`, and `--email-message`.

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
go run ./cmd/oa assets activate --id <asset-id>
go run ./cmd/oa assets dispose --id <asset-id> --disposal-date 2026-05-01 --method SOLD --proceeds 900.00
go run ./cmd/oa assets depreciate --id <asset-id>
go run ./cmd/oa assets depreciation --id <asset-id>
go run ./cmd/oa assets delete --id <asset-id>
```

Asset statuses are `DRAFT`, `ACTIVE`, `DISPOSED`, and `SOLD`. Depreciation methods are `STRAIGHT_LINE`, `DECLINING_BALANCE`, and `UNITS_OF_PRODUCTION`; disposal methods are `SOLD`, `SCRAPPED`, `DONATED`, and `LOST`. Asset CSV imports require `name`, `purchase_date`, and `purchase_cost`; optional columns include `asset_number`, `category_id`, `category_name`, `status`, depreciation/book-value fields, disposal fields, and account IDs.

## Inventory

```bash
go run ./cmd/oa inventory categories list
go run ./cmd/oa inventory categories create --name Parts --description "Spare parts"
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
go run ./cmd/oa inventory products import --file ./products.csv
go run ./cmd/oa inventory products get --id <product-id>
go run ./cmd/oa inventory products update --id <product-id> --name Widget --sales-price 16.00
go run ./cmd/oa inventory products stock-levels --id <product-id>
go run ./cmd/oa inventory products movements --id <product-id>
go run ./cmd/oa inventory products delete --id <product-id>

go run ./cmd/oa inventory warehouses list --active-only
go run ./cmd/oa inventory warehouses create --code MAIN --name "Main warehouse" --address Tallinn --default
go run ./cmd/oa inventory warehouses get --id <warehouse-id>
go run ./cmd/oa inventory warehouses update --id <warehouse-id> --name "Main warehouse" --active true
go run ./cmd/oa inventory warehouses delete --id <warehouse-id>

go run ./cmd/oa inventory adjust --product-id <product-id> --warehouse-id <warehouse-id> --quantity -2 --unit-cost 10.50 --reason "Cycle count"
go run ./cmd/oa inventory stock import --file ./stock.csv
go run ./cmd/oa inventory transfer --product-id <product-id> --from-warehouse-id <warehouse-id> --to-warehouse-id <warehouse-id> --quantity 3
```

Product types are `GOODS` and `SERVICE`; product statuses are `ACTIVE` and `INACTIVE`. Product CSV imports require `name` and `sales_price`; optional columns include `code`, `product_type`, `category_id`, `category_name`, prices, VAT, reorder settings, account IDs, inventory tracking, status, barcode, supplier, and lead time. Use `--json` on inventory read and mutation commands for automation. `inventory adjust` accepts signed quantities; positive quantities add stock and negative quantities remove stock. `inventory stock import` accepts `product_id` or `product_code`, `warehouse_id` or `warehouse_code`, signed `quantity`, optional `unit_cost`, and optional `reason`.

## Cost centers

```bash
go run ./cmd/oa cost-centers list --active-only
go run ./cmd/oa cost-centers create \
  --code CC001 \
  --name Sales \
  --budget-amount 1000.00 \
  --budget-period MONTHLY
go run ./cmd/oa cost-centers get --id <cost-center-id>
go run ./cmd/oa cost-centers update \
  --id <cost-center-id> \
  --code CC001 \
  --name Sales \
  --budget-amount 1200.00
go run ./cmd/oa cost-centers report --start 2026-03-01 --end 2026-03-31
go run ./cmd/oa cost-centers delete --id <cost-center-id>
```

Budget periods are `MONTHLY`, `QUARTERLY`, and `ANNUAL`. Use `--json` on cost-center read and mutation commands for automation.

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
go run ./cmd/oa payments get --id <payment-id>
go run ./cmd/oa payments allocate --id <payment-id> --invoice-id <invoice-id> --amount 250.00
go run ./cmd/oa payments unallocated --type RECEIVED
```

Use `--allocate invoice-id:amount` repeatedly on `payments create` to allocate a new payment to multiple invoices. Payment types are `RECEIVED` and `MADE`; `--json` is available on list, create, get, allocate, and unallocated commands.

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
go run ./cmd/oa email payment-receipt --payment-id <payment-id> --recipient-email billing@example.com
```

Template types are `INVOICE_SEND`, `PAYMENT_RECEIPT`, and `OVERDUE_REMINDER`. Use `--json` on email reads and mutations for automation.

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
go run ./cmd/oa close reopen --period-end 2026-03-31 --note "Correcting late supplier invoice"
go run ./cmd/oa close year-end-status --period-end 2025-12-31
go run ./cmd/oa close carry-forward --period-end 2025-12-31
go run ./cmd/oa close reverse-carry-forward --period-end 2025-12-31 --reason "Late supplier accrual"
```

Period close and reopen operations require a user role that can manage close workflows. Reopening requires a note, and fiscal-year periods cannot be reopened after carry-forward has been posted unless the carry-forward is explicitly reversed first. Use `--json` for automation.

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
go run ./cmd/oa banking accounts get --id <bank-account-id>
go run ./cmd/oa banking accounts update --id <bank-account-id> --bank-name SEB --active true
go run ./cmd/oa banking accounts delete --id <bank-account-id>

go run ./cmd/oa banking transactions list \
  --account-id <bank-account-id> \
  --status UNMATCHED \
  --from 2026-03-01 \
  --to 2026-03-31
go run ./cmd/oa banking transactions import --account-id <bank-account-id> --file ./bank.csv
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

Bank transaction statuses are `UNMATCHED`, `MATCHED`, and `RECONCILED`. Follow-up statuses are `NONE`, `EVIDENCE_REQUIRED`, and `READY_TO_MATCH`. Reconciliation completion blocks matched transactions marked `EVIDENCE_REQUIRED` until they have approved `reconciliation_evidence` documents; use `documents upload`, `documents review`, and `documents evidence-policy` to resolve evidence failures. Bank CSV imports accept comma, semicolon, or tab delimiters with headers such as `date`, `amount`, `description`, `reference`, `counterparty_name`, `counterparty_account`, `value_date`, and `external_id`. Use `--json` on banking read and mutation commands for automation.

## Reports

```bash
go run ./cmd/oa reports trial-balance --as-of 2026-03-31
go run ./cmd/oa reports trial-balance --as-of 2026-03-31 --csv --output ./trial-balance.csv
go run ./cmd/oa reports trial-balance --as-of 2026-03-31 --xlsx --output ./trial-balance.xlsx
go run ./cmd/oa reports trial-balance --as-of 2026-03-31 --pdf --output ./trial-balance.pdf
go run ./cmd/oa reports account-balance --account-id <account-id> --as-of 2026-03-31
go run ./cmd/oa reports balance-sheet --as-of 2026-03-31
go run ./cmd/oa reports balance-sheet --as-of 2026-03-31 --csv --output ./balance-sheet.csv
go run ./cmd/oa reports balance-sheet --as-of 2026-03-31 --xlsx --output ./balance-sheet.xlsx
go run ./cmd/oa reports balance-sheet --as-of 2026-03-31 --pdf --output ./balance-sheet.pdf
go run ./cmd/oa reports income-statement --start 2026-01-01 --end 2026-03-31
go run ./cmd/oa reports income-statement --start 2026-01-01 --end 2026-03-31 --csv --output ./income-statement.csv
go run ./cmd/oa reports income-statement --start 2026-01-01 --end 2026-03-31 --xlsx --output ./income-statement.xlsx
go run ./cmd/oa reports income-statement --start 2026-01-01 --end 2026-03-31 --pdf --output ./income-statement.pdf
go run ./cmd/oa reports cash-flow --start 2026-01-01 --end 2026-03-31
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
```

Every report command supports `--json` for automation. Core statement, aging, and balance-confirmation commands also support backend CSV export with `--csv`, XLSX export with `--xlsx`, and PDF export with `--pdf`; omit `--output` to stream the export bytes to stdout.

## Documents

```bash
go run ./cmd/oa documents list --entity-type payment --entity-id <payment-id>
go run ./cmd/oa documents upload \
  --entity-type bank_transaction \
  --entity-id <transaction-id> \
  --file ./statement-line.pdf \
  --document-type reconciliation_evidence \
  --notes "Matched against March bank statement" \
  --retention-until 2027-03-31
go run ./cmd/oa documents review-summary --entity-type payment --entity-id <payment-id> --entity-id <payment-id>
go run ./cmd/oa documents evidence-policy --entity-type payment --entity-id <payment-id> --document-type receipt --require-approved
go run ./cmd/oa documents retention --as-of 2027-03-01 --horizon-days 45 --include-missing
go run ./cmd/oa documents download --id <document-id> --output ./document.pdf
go run ./cmd/oa documents review --id <document-id> --status APPROVED --note "Evidence accepted"
go run ./cmd/oa documents review --id <document-id> --status REJECTED --note "Receipt does not match"
go run ./cmd/oa documents mark-reviewed --id <document-id>
go run ./cmd/oa documents delete --id <document-id>
```

`documents evidence-policy` checks required evidence for one or more entity IDs. Repeat `--document-type` or `--required-document-type` to allow several document types in the rule, set `--min-count` for the required count, and use `--require-approved` when pending or reviewed-but-unapproved evidence must fail. `documents retention` returns a tenant-wide queue of documents whose `retention_until` is due by the cutoff, with optional missing-retention records. `documents review` supports `REVIEWED`, `APPROVED`, and `REJECTED`; rejected documents require a review note. `documents download` uses the server-provided filename when `--output` is omitted. Use `--output -` to stream the document content to stdout.

## Journal entries

```bash
go run ./cmd/oa journal list --limit 50
go run ./cmd/oa journal create \
  --entry-date 2026-03-31 \
  --description "Manual accrual" \
  --reference ACC-1 \
  --line "account_id=<expense-account-id>,description=Expense,debit=100.00" \
  --line "account_id=<accrual-account-id>,description=Accrual,credit=100.00"
go run ./cmd/oa journal get --id <journal-entry-id>
go run ./cmd/oa journal post --id <journal-entry-id>
go run ./cmd/oa journal void --id <journal-entry-id> --reason "Duplicate entry"
```

Use `--line` repeatedly on `journal create`. Each line is comma-separated `key=value` pairs with `account_id` and exactly one of `debit` or `credit`; optional keys include `description`, `currency`, and `exchange_rate`.

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
name,type,email,payment_terms_days,country_code,credit_limit
Northwind OU,CUSTOMER,ap@northwind.example,14,EE,1500.00
Supply Partner,SUPPLIER,purchases@supply.example,30,EE,2500.00
```

### Invoices

```csv
invoice_number,invoice_type,contact_code,issue_date,due_date,status,amount_paid,reference,notes,line_description,quantity,unit,unit_price,discount_percent,vat_rate
INV-EXT-001,SALES,CUST-001,2026-02-01,2026-02-15,SENT,0,PO-12345,Imported migration invoice,Implementation work,1,hour,100.00,0,22
INV-EXT-001,SALES,CUST-001,2026-02-01,2026-02-15,SENT,0,PO-12345,Imported migration invoice,Support retainer,1,month,50.00,0,22
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
go run ./cmd/oa documents upload --entity-type asset --entity-id <asset-id> --file ./warranty.pdf --document-type asset_record
```

## Notes

- Normal data commands use the stored API token, not the login password.
- API tokens are tenant-scoped. A token created for one tenant cannot be used on another tenant path.
- API tokens belong to the authenticated user that created them and can be revoked later.
- `auth status` verifies the stored token by calling `/api/v1/me`.
- Use `--json` on list/create/import commands if you want machine-readable output.
