---
active: true
iteration: 1
max_iterations: 25
completion_promise: "DEMO_E2E_COMPLETE"
started_at: "2026-01-02T14:45:00Z"
---

COMMAND:
```bash
/ralph-loop "Complete E2E test coverage for ALL demo environment views. Run: cd frontend && npx playwright test --config=playwright.demo.config.ts --project=demo-chromium. Fix any failures. Push and verify CI." --max-iterations 25 --completion-promise "DEMO_E2E_COMPLETE"
```

Complete E2E test coverage for ALL demo environment views.

WORKFLOW:
1. Run demo E2E tests: cd frontend && npx playwright test --config=playwright.demo.config.ts --project=demo-chromium
2. Check coverage against all 22 routes in the application
3. Verify seed data exists for all views that need data
4. Fix any failing tests by making them more resilient
5. Push changes and verify CI e2e-demo job results

ROUTES TO COVER (22 total):
- Landing & Auth: /, /login
- Dashboard: /dashboard
- Accounting: /accounts, /journal, /invoices, /payments, /recurring, /contacts, /reports
- Payroll: /employees, /payroll, /tsd
- Banking: /banking, /banking/import
- Tax: /tax
- Settings: /settings, /settings/company, /settings/email, /settings/plugins
- Admin: /admin/plugins

TEST FILES:
- frontend/e2e/demo-env.spec.ts (27 tests) - Health, Auth, Dashboard, Invoices, Contacts, Reports, Settings, Responsive, Error handling, Onboarding, Performance
- frontend/e2e/demo-all-views.spec.ts (24 tests) - All 22 routes + Navigation + Responsive

SEED DATA (scripts/demo-seed.sql):
- Users & Tenants: demo@example.com / demo123, Acme Corporation
- Chart of Accounts: 31 Estonian-standard accounts
- Contacts: 7 contacts (4 customers, 3 suppliers)
- Invoices: 9 invoices with 16 line items
- Payments: 4 bank transfers
- Employees: 5 employees with salary components
- Payroll Runs: 3 months (Oct, Nov, Dec 2024) with 12 payslips
- TSD Declarations: 3 periods
- Recurring Invoices: 3 active recurring invoices
- Journal Entries: 4 entries with 8 lines
- Bank Transactions: 8 transactions
- Bank Accounts: 2 accounts (Swedbank, SEB)

SUCCESS CRITERIA:
- All 22 routes have E2E tests
- All demo-all-views.spec.ts tests pass locally
- All demo-env.spec.ts tests pass locally
- CI e2e-demo job runs (may have flaky failures due to network)
- Seed data covers all views that display data

When all criteria met, output: <promise>DEMO_E2E_COMPLETE</promise>
