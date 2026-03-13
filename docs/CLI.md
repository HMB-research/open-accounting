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
go run ./cmd/oa accounts import --file ./accounts.csv
```

## Contacts

```bash
go run ./cmd/oa contacts list
go run ./cmd/oa contacts list --type CUSTOMER --search Nordic
go run ./cmd/oa contacts create --name "New Customer" --type CUSTOMER --email customer@example.com
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
go run ./cmd/oa employees import --file ./employees.csv
```

## Invoices

```bash
go run ./cmd/oa invoices import --file ./invoices.csv
```

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
go run ./cmd/oa employees import --file ./employees.csv
go run ./cmd/oa documents upload --entity-type asset --entity-id <asset-id> --file ./warranty.pdf --document-type asset_record
```

## Notes

- Normal data commands use the stored API token, not the login password.
- API tokens are tenant-scoped. A token created for one tenant cannot be used on another tenant path.
- API tokens belong to the authenticated user that created them and can be revoked later.
- `auth status` verifies the stored token by calling `/api/v1/me`.
- Use `--json` on list/create/import commands if you want machine-readable output.
