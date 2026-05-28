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

## Bootstrap a token

```bash
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
go run ./cmd/oa auth logout
```

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
go run ./cmd/oa tax kmd export-xml --year 2026 --month 3 --output ./kmd-2026-03.xml
```

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
go run ./cmd/oa assets get --id <asset-id>
go run ./cmd/oa assets update --id <asset-id> --name Laptop --useful-life-months 48
go run ./cmd/oa assets activate --id <asset-id>
go run ./cmd/oa assets dispose --id <asset-id> --disposal-date 2026-05-01 --method SOLD --proceeds 900.00
go run ./cmd/oa assets depreciate --id <asset-id>
go run ./cmd/oa assets depreciation --id <asset-id>
go run ./cmd/oa assets delete --id <asset-id>
```

Asset statuses are `DRAFT`, `ACTIVE`, `DISPOSED`, and `SOLD`. Depreciation methods are `STRAIGHT_LINE`, `DECLINING_BALANCE`, and `UNITS_OF_PRODUCTION`; disposal methods are `SOLD`, `SCRAPPED`, `DONATED`, and `LOST`.

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

## Reports

```bash
go run ./cmd/oa reports trial-balance --as-of 2026-03-31
go run ./cmd/oa reports account-balance --account-id <account-id> --as-of 2026-03-31
go run ./cmd/oa reports balance-sheet --as-of 2026-03-31
go run ./cmd/oa reports income-statement --start 2026-01-01 --end 2026-03-31
go run ./cmd/oa reports cash-flow --start 2026-01-01 --end 2026-03-31
go run ./cmd/oa reports aging --type receivables
go run ./cmd/oa reports aging --type payables --json
go run ./cmd/oa reports balance-confirmations --type RECEIVABLE --as-of 2026-03-31
go run ./cmd/oa reports balance-confirmation \
  --contact-id <contact-id> \
  --type RECEIVABLE \
  --as-of 2026-03-31
```

Every report command supports `--json` for automation. The table output is intended for quick terminal review, while JSON preserves the API response shape for scripts.

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
go run ./cmd/oa documents mark-reviewed --id <document-id>
go run ./cmd/oa documents delete --id <document-id>
```

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
