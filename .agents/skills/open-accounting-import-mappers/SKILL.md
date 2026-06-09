---
name: open-accounting-import-mappers
description: Use when adding, refactoring, or testing open-accounting external import formats, especially bank/provider-specific mappers, LHV statement samples, mapper registries, API/CLI format parity, and removal of legacy parser paths.
---

# Open Accounting Import Mappers

Use this with `open-accounting-development` when an import needs provider-specific parsing or when an existing import path is duplicated across API, CLI, service, or frontend code.

## Boundaries

- Put parsing and normalization in `internal/<domain>/mappers/<provider>/` when a provider or bank has its own format.
- Keep shared CSV/header/date helpers in the domain mapper package, not in handlers or CLI commands.
- Keep format selection and auto-detection in a small registry package such as `internal/banking/mappers/registry`.
- Services own validation, duplicate handling, and import orchestration after rows are normalized.
- Repositories own persistence through the ORM/repository layer. Do not add direct SQL to handlers, services, or CLI code.
- Delete replaced legacy parser paths while touching the import. Compatibility shims need an explicit product reason and a removal plan.

## Provider Sample Workflow

1. Fetch the current provider documentation or official sample before changing parser assumptions.
2. Add the smallest useful fixture under the provider mapper `testdata/` directory.
3. Note the source in the test or nearby docs using a short source label, for example `LHV Connect Account Statement "Statement data" sample`.
4. Test the fixture through the provider mapper and the registry, not only through an end-to-end import.
5. Keep fixtures deterministic and free of private customer data.

## Current Banking Mapper Shape

Bank transaction import supports these formats:

- `auto`: detect LHV CSV and LHV camt.053 XML, then fall back to generic CSV.
- `generic`: shared generic transaction CSV headers.
- `lhv`: LHV Internet Bank account statement CSV.
- `lhv-camt`: LHV Connect camt.053 account statement XML.

Key files:

- `internal/banking/mappers/csv.go`
- `internal/banking/mappers/generic/transactions.go`
- `internal/banking/mappers/lhv/transactions.go`
- `internal/banking/mappers/lhv/testdata/account_statement_camt053_official.xml`
- `internal/banking/mappers/registry/transactions.go`
- `internal/banking/transaction_import.go`

## Parity Checklist

When adding or changing an import format, update the whole surface:

- Mapper implementation and provider-specific tests.
- Registry format constants, detection, unsupported-format errors, and registry tests.
- API request docs and handler/service tests.
- CLI flags, parser delegation, command tests, output/docs, and `docs/CLI.md`.
- `docs/API.md`, generated Swagger docs when the route surface changes, and status docs.
- Frontend format options and a demo E2E workflow when the import is user-facing.

## Focused Validation

Run mapper tests before broader gates:

```bash
go test -count=1 ./internal/banking/mappers/...
go test -count=1 ./internal/banking -run 'Test.*Import.*'
make test-cli-coverage
go test -timeout=3m ./docs -count=1
```

For user-facing bank import changes, start the branch API from `open-accounting-demo-e2e` and run:

```bash
cd frontend
BASE_URL=http://localhost:5174 \
PUBLIC_API_URL=http://localhost:18080 \
DEMO_RESET_SECRET=test-demo-secret \
bunx playwright test --config=playwright.demo.config.ts --project=demo-chromium e2e/demo/bank-import.spec.ts --workers=1
```
