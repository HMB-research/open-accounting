# Testing Flow Speed Review

## Source Of Truth

Reviewed PR #62 on branch `feat/payroll-history-import` using CI run `27272928801` for commit `52e23a46f43fc9d99ddf5cc41f8ec61f5ef9fc2d`.

The run succeeded. It started at `2026-06-10T11:26:04Z` and finished at `2026-06-10T11:32:56Z`, so the full PR feedback loop was about 6m52s.

Local focused gates checked during the review:

| Gate | Result |
|------|--------|
| `make test-cli-coverage` | 100.0% `cmd/oa` coverage |
| `go test -timeout=3m ./docs -count=1` | pass |

## Current CI Timing Shape

| Area | Current observation | Speed impact |
|------|---------------------|--------------|
| Backend unit gate | `test` job took about 2m03s, with the backend test step about 1m32s | Not the main bottleneck |
| Frontend unit/type/build gate | `frontend` job took about 1m41s | Healthy; keep generated-file steps serial locally |
| Integration gate | Slowest shard job was `integration-test (3, 4)` at about 5m41s | Main critical-path bottleneck before `build` |
| Demo E2E gate | Shards were about 4m27s to 4m59s job time, with Playwright result time about 181s to 209s | Balanced enough after recent file placement change |
| E2E smoke gate | About 2m06s | Useful early signal, not the critical path on this run |
| Build | About 55s after backend gates | Starts after integration gate completes |

## Integration Findings

The integration Make target shards packages by package list position, not by measured runtime. That created a large imbalance in the latest successful run:

| Shard | Sum of package test times |
|-------|---------------------------|
| 1 | 109.8s |
| 2 | 65.0s |
| 3 | 226.1s |
| 4 | 88.4s |

The slowest packages were:

| Package | Time | Current shard |
|---------|------|---------------|
| `internal/accounting` | 118.612s | 3 |
| `internal/payroll` | 55.136s | 3 |
| `internal/inventory` | 42.157s | 3 |
| `internal/banking` | 37.820s | 4 |
| `internal/contacts` | 31.764s | 1 |
| `internal/analytics` | 22.629s | 4 |
| `internal/email` | 21.481s | 1 |

Recommended improvement:

1. Replace count-based integration package sharding with weight-aware buckets based on recent CI package durations.
2. Keep real ORM/Postgres coverage, but reduce repeated tenant schema lifecycle inside the heaviest packages by grouping related repository cases behind shared fixtures where isolation permits.
3. Add a small timing parser script so future reviews can regenerate the package-duration table from `gh run view --job <id> --log` output instead of using ad hoc shell snippets.

Do not add more integration runners before rebalancing. The latest data shows enough idle capacity on shards 1, 2, and 4 to reduce wall time without increasing runner count.

## Demo E2E Findings

Latest Playwright JSON artifacts showed the sharded demo suite is reasonably balanced:

| Shard | Playwright result time | Tests |
|-------|------------------------|-------|
| 1 | 209.0s | 69 |
| 2 | 201.9s | 68 |
| 3 | 202.4s | 68 |
| 4 | 181.0s | 56 |

The largest spec files by accumulated result time were:

| Spec | Accumulated time |
|------|------------------|
| `demo/env.spec.ts` | 289.5s |
| `demo/all-views.spec.ts` | 276.9s |
| `demo/mobile.spec.ts` | 147.6s |
| `demo/payment-reminders.spec.ts` | 136.4s |
| `demo/data-verification.spec.ts` | 127.4s |
| `demo/absences.spec.ts` | 125.1s |
| `demo/balance-confirmations.spec.ts` | 119.6s |
| `demo/invoices.spec.ts` | 113.1s |

Recommended improvement:

1. Keep four Playwright shards for now. The shard spread is small enough that adding runners would mostly duplicate setup cost.
2. Continue keeping broad specs under `frontend/e2e/demo/`; root-level broad specs sort late and can create a slow tail shard.
3. Optimize `env.spec.ts`, `all-views.spec.ts`, and `mobile.spec.ts` first. Favor route-owned readiness selectors, fewer repeated full-page navigations, and API-backed state checks where a UI assertion is not required.
4. Keep `e2e-smoke` as the quick core-flow signal unless runner cost becomes a concern. It is parallel to the full E2E gate and was not the critical path on this run.

## Not Recommended Now

| Change | Reason |
|--------|--------|
| Increase Playwright shard count | Current shards are already close; setup would be repeated more often |
| Remove the smoke E2E gate | It catches core demo breakage early and is not slowing the latest critical path |
| Parallelize local frontend `check`, `test`, and `build` | Generated Paraglide/SvelteKit files can race |
| Drop race/coverage from backend closeout | It would weaken the accounting-sensitive gate more than it helps speed |

## Next Stage Candidate

The integration sharding candidate below has been implemented. The current next candidate is demo Playwright optimization, starting with measured broad specs rather than changing shard counts.

## Implementation Update

Implemented weight-aware integration package selection in `scripts/select-integration-packages.sh`, backed by `scripts/integration-package-weights.tsv`.

The weights were refreshed from CI run `27273825703`, where count-based sharding produced package-time totals of about 106.3s, 66.0s, 90.5s, and 91.7s. The greedy weight-aware assignment projects package-time totals of about 88.4s, 88.8s, 89.0s, and 88.3s across the same four shards.

The Make targets remain unchanged for callers: local `make test-integration-coverage` still runs the full integration package set, while CI sets `INTEGRATION_SHARD` and `INTEGRATION_SHARDS` to select one balanced shard.

Follow-up CI run `27274687441` on commit `9743dfd` completed successfully with integration shard job times of 4m48s, 4m14s, 4m01s, and 3m36s, replacing the earlier 5m41s slow-tail shard.

Added `scripts/parse-go-test-package-times.sh` so future refreshes can be regenerated from GitHub Actions logs:

```bash
for job in 80552847747 80552847725 80552847854 80552847833; do
  gh run view 27274687441 --job "$job" --log
done | scripts/parse-go-test-package-times.sh
```

The checked-in weights were refreshed from that run. With the refreshed weights, the greedy assignment projects package-time totals of about 105.2s, 104.5s, 105.1s, and 104.2s across the four integration shards.

Follow-up CI run `27301644898` on commit `0b1625d` completed successfully. Integration shard job times tightened further to 4m00s, 3m50s, 3m33s, and 3m43s. Full demo E2E is now the slowest required PR gate, with shard job times of 5m11s, 5m17s, 4m59s, and 4m51s.

Added `scripts/parse-playwright-spec-times.mjs` so demo E2E optimization can be based on uploaded Playwright JSON artifacts instead of ad hoc artifact inspection:

```bash
gh run download 27301644898 --pattern 'playwright-results-shard-*' --dir /tmp/playwright-results
scripts/parse-playwright-spec-times.mjs /tmp/playwright-results/*/demo-test-results.json
```

The latest artifact aggregation shows these highest-duration spec files:

| Spec | Executed time | Tests |
|------|---------------|-------|
| `demo/all-views.spec.ts` | 308.924s | 24 |
| `demo/env.spec.ts` | 302.028s | 28 |
| `demo/mobile.spec.ts` | 159.907s | 15 |
| `demo/payment-reminders.spec.ts` | 145.037s | 14 |
| `demo/data-verification.spec.ts` | 138.093s | 13 |
| `demo/absences.spec.ts` | 133.309s | 12 |
| `demo/balance-confirmations.spec.ts` | 127.176s | 12 |
| `demo/invoices.spec.ts` | 118.562s | 10 |

Next demo E2E speed work should start with `demo/all-views.spec.ts` and the split `demo/env-*.spec.ts` files: reduce repeated full-page navigation, add route-owned readiness selectors where broad waits are hiding route load time, and replace duplicated UI setup with API-backed state checks only where the UI is not the behavior under test.

## Follow-up: Environment Spec Split

CI run `27304974475` on commit `50e7bb1` completed successfully after the latest integration weight refresh. The integration shard job times were 3m58s, 3m51s, 3m55s, and 3m27s, while full demo E2E remained the slowest required PR gate with shard job times of 4m16s, 5m08s, 4m58s, and 4m17s.

The Playwright artifact aggregation from that run still showed the environment smoke file as the largest single spec:

| Spec | Executed time | Tests |
|------|---------------|-------|
| `demo/env.spec.ts` | 300.820s | 28 |
| `demo/mobile.spec.ts` | 161.262s | 15 |
| `demo/data-verification.spec.ts` | 141.807s | 13 |
| `demo/payment-reminders.spec.ts` | 133.090s | 14 |
| `demo/absences.spec.ts` | 132.083s | 12 |

Split `demo/env.spec.ts` into workflow-sized files:

| New spec | Coverage moved |
|----------|----------------|
| `demo/env-health-auth.spec.ts` | Health checks and login/logout behavior |
| `demo/env-dashboard-routes.spec.ts` | Dashboard, invoices, contacts, reports, settings route smoke checks |
| `demo/env-responsive-errors.spec.ts` | Mobile/tablet viewport checks and unauthenticated/error-route behavior |
| `demo/env-onboarding.spec.ts` | Onboarding wizard visibility, form, step, and completion checks |
| `demo/env-performance.spec.ts` | Login and dashboard reload timing checks |

The split preserves the same 28 demo tests. `bunx playwright test --config=playwright.demo.config.ts --project=demo-chromium --list e2e/demo/env-*.spec.ts` reports those 28 tests plus the shared `auth-setup` dependency. The next CI Playwright artifact should be used to confirm whether the former `env.spec.ts` slow tail is now distributed across shards.

## Follow-up: Mobile Spec Split

CI run `27305782206` on commit `ab1d583` confirmed the former `demo/env.spec.ts` slow tail was distributed, and shifted the highest single-spec cost to `demo/mobile.spec.ts` at 155.472s across 15 tests. The same artifact showed shard 3 carrying `demo/mobile.spec.ts`, `demo/invoices.spec.ts`, and most `demo/payment-reminders.spec.ts`, so mobile remained one of the next measured E2E distribution targets.

Split `demo/mobile.spec.ts` into workflow-sized files and moved repeated mobile route helpers into `demo/mobile-utils.ts`:

| New spec | Coverage moved |
|----------|----------------|
| `demo/mobile-navigation.spec.ts` | Mobile drawer visibility, grouped navigation, link close behavior, nested sales navigation |
| `demo/mobile-layout.spec.ts` | Mobile table usability, dashboard layout, horizontal overflow checks, tablet layout |
| `demo/mobile-forms.spec.ts` | Mobile contact form access and touch target sizing |

The split preserves the same 15 mobile demo tests. Route setup now uses route-owned readiness selectors with `waitForNetworkIdle: false` for these mobile checks, avoiding broad network-idle waits once the route shell is visible.

Local focused baseline against the branch API showed `demo/mobile.spec.ts` at 28.169s of Playwright result time. After the split, the same coverage reported:

| Spec | Executed time | Tests |
|------|---------------|-------|
| `demo/mobile-navigation.spec.ts` | 6.736s | 4 |
| `demo/mobile-forms.spec.ts` | 6.655s | 2 |
| `demo/mobile-layout.spec.ts` | 3.649s | 9 |

The focused command still reports 16 tests including the shared `auth-setup` dependency. The next CI Playwright artifact should verify whether the prior mobile shard tail is reduced in the required PR gate.

During the local frontend gate, `src/tests/utils/dates.test.ts` exposed a timezone-sensitive assertion: `getTodayISO()` returns the local calendar date, but the test compared it to `new Date().toISOString().slice(0, 10)`, which is UTC. Around midnight in `Europe/Tallinn`, that produced `2026-06-11` vs `2026-06-10`. The test now computes the expected value through the same local `toISODate(new Date())` helper so the gate is stable across local timezone boundaries.

CI run `27306627274` exposed that the first mobile split was too aggressive about skipping `networkidle` for mobile navigation tests: the route heading was visible while the dashboard still showed `Loading...`, so the mobile nav shell had not rendered yet. The mobile drawer helper now waits for plain loading text to disappear before asserting the `toggle menu` button. The focused local `mobile-navigation.spec.ts` and full `mobile-*.spec.ts` suites pass after that adjustment.

CI run `27307133267` on commit `8db6847` completed successfully after the mobile nav-shell fix. The highest-duration demo specs in its uploaded Playwright artifacts were:

| Spec | Executed time | Tests |
|------|---------------|-------|
| `demo/env-dashboard-routes.spec.ts` | 168.709s | 11 |
| `demo/payment-reminders.spec.ts` | 142.869s | 14 |
| `demo/data-verification.spec.ts` | 141.321s | 13 |
| `demo/absences.spec.ts` | 123.533s | 12 |
| `demo/invoices.spec.ts` | 121.062s | 10 |

## Follow-up: Environment Route Split

Split `demo/env-dashboard-routes.spec.ts` into route-sized files and moved route setup through reusable helpers in `demo/env-utils.ts`:

| New spec | Coverage moved |
|----------|----------------|
| `demo/env-dashboard.spec.ts` | Dashboard selector, summary cards, and navigation shell checks |
| `demo/env-invoice-routes.spec.ts` | Invoices navigation, list, and create-form access checks |
| `demo/env-contact-routes.spec.ts` | Contacts navigation and list checks |
| `demo/env-report-settings-routes.spec.ts` | Reports navigation/load checks and settings navigation |

The split preserves the same 11 route tests. Direct route-load checks now authenticate the worker session and go straight to the target route with `waitForNetworkIdle: false`; only tests that explicitly validate sidebar navigation start from the dashboard first.

Local focused baseline against the branch API showed `demo/env-dashboard-routes.spec.ts` at 19.979s of Playwright result time. After the split, the same coverage reported:

| Spec | Executed time | Tests |
|------|---------------|-------|
| `demo/env-invoice-routes.spec.ts` | 6.422s | 3 |
| `demo/env-dashboard.spec.ts` | 4.993s | 3 |
| `demo/env-contact-routes.spec.ts` | 3.751s | 2 |
| `demo/env-report-settings-routes.spec.ts` | 2.427s | 3 |

The next CI Playwright artifact should confirm whether the prior environment-route shard tail is distributed. If it is, the next measured demo E2E candidates are `payment-reminders`, `data-verification`, `absences`, and `invoices`.

CI run `27307960847` on commit `b2c5be6` confirmed the former `demo/env-dashboard-routes.spec.ts` entry was distributed, and shifted the highest single-spec cost to `demo/payment-reminders.spec.ts` at 138.290s across 14 tests.

## Follow-up: Payment Reminders Spec Split

Split `demo/payment-reminders.spec.ts` into workflow-sized files and moved repeated route/data helpers into `demo/payment-reminders-utils.ts`:

| New spec | Coverage moved |
|----------|----------------|
| `demo/payment-reminders-page.spec.ts` | Heading, refresh/back controls, summary and empty/list state checks |
| `demo/payment-reminders-selection.spec.ts` | Invoice checkbox selection and send-button availability checks |
| `demo/payment-reminders-modal.spec.ts` | Send reminder modal open, close, and custom-message checks |
| `demo/payment-reminders-table.spec.ts` | Table headers and overdue badge checks |

The split preserves the same 14 payment-reminder demo tests. Route setup now opens `/invoices/reminders` with `waitForNetworkIdle: false` and waits for a route-owned heading or terminal reminder data state; data-dependent tests still wait for `.summary-card`, `.empty-state`, or `.alert-error` before asserting tables, badges, or modal controls.

Local focused baseline against the branch API showed `demo/payment-reminders.spec.ts` at 23.536s of Playwright result time. After the split, the same coverage reported:

| Spec | Executed time | Tests |
|------|---------------|-------|
| `demo/payment-reminders-modal.spec.ts` | 5.954s | 3 |
| `demo/payment-reminders-selection.spec.ts` | 4.688s | 4 |
| `demo/payment-reminders-page.spec.ts` | 2.679s | 5 |
| `demo/payment-reminders-table.spec.ts` | 1.818s | 2 |

The next CI Playwright artifact should confirm whether the prior payment-reminder shard tail is distributed. The next measured demo E2E candidates are `balance-confirmations`, `invoices`, `data-verification`, and `absences`.

CI run `27308618173` on commit `e54697f` confirmed the prior payment-reminder tail was distributed. The highest single-spec cost shifted to `demo/absences.spec.ts` at 137.300s across 12 micro-tests.

CI run `27325104193` on commit `8c48fa1` confirmed the legacy broad view and inventory specs had been removed or distributed. The highest single-spec costs shifted to smaller workflow files:

| Spec | Executed time | Tests |
|------|---------------|-------|
| `demo/tax-overview.spec.ts` | 75.973s | 5 |
| `auth.spec.ts` | 71.557s | 8 |
| `demo/payments.spec.ts` | 70.494s | 6 |
| `demo/cash-payments.spec.ts` | 68.829s | 6 |
| `demo/plugins-settings.spec.ts` | 68.396s | 6 |

## Follow-up: Tax Overview Workflow Consolidation

Consolidated `demo/tax-overview.spec.ts` from five page-structure micro-tests into one route-owned workflow test. The test now opens `/tax` with a KMD API readiness wait, verifies the VAT controls, generates a declaration for `2026-06`, asserts the POST response shape, and verifies the rendered declaration and VAT amount labels.

The stronger workflow assertion exposed a real demo reset bug: fresh tenant schemas created through `create_tenant_schema` did not include `journal_entry_lines.vat_rate` or `journal_entry_lines.is_vat_inclusive`, so KMD generation returned a 500 from the tax repository. Migration `052_journal_line_vat_tenant_bootstrap` makes the existing VAT helper tenant-bootstrap owned and normalizes existing schemas; the `JournalEntryLine` GORM model now includes the VAT fields.

Local focused baseline against the branch API showed `demo/tax-overview.spec.ts` at 12.483s across 5 tests. After the consolidation and schema fix, the same coverage reports:

| Spec | Executed time | Tests |
|------|---------------|-------|
| `demo/tax-overview.spec.ts` | 1.239s | 1 |

The next CI Playwright artifact should confirm whether the former tax overview entry is no longer a shard tail. The next measured demo E2E candidates are `auth.spec.ts`, `payments`, `cash-payments`, and `plugins-settings`.

## Follow-up: Absences Workflow Consolidation

Replaced `demo/absences.spec.ts` with workflow-sized files and moved repeated route setup into `demo/absences-utils.ts`:

| New spec | Coverage moved |
|----------|----------------|
| `demo/absences-page.spec.ts` | Leave-management shell, request button, year and employee filters, tabs, and records content state |
| `demo/absences-filters-tabs.spec.ts` | Records/balances tab switching behavior |
| `demo/absences-modal.spec.ts` | Request-leave modal open, required fields, and close behavior |
| `demo/absences-selection.spec.ts` | Employee option availability and balances empty-state behavior without a selected employee |

The old file repeated full auth, tenant setup, route navigation, and generic loading waits for 12 separate micro-tests. The new files keep the same user-visible assertions but fold them into 4 workflow tests, so each workflow pays the route setup cost once. Route setup now opens `/employees/absences` with `waitForNetworkIdle: false`, waits for route-owned controls, and then waits for the route's terminal content state (`table.table`, `.empty-state`, or `.alert-error`).

Local focused baseline against the branch API showed `demo/absences.spec.ts` at 21.857s of Playwright result time and `13 passed (12.8s)` including auth setup. A mechanical split that preserved all 12 micro-tests still reported 21.575s, confirming repeated navigation was the cost. After workflow consolidation, the same visible behaviors reported:

| Spec | Executed time | Tests |
|------|---------------|-------|
| `demo/absences-modal.spec.ts` | 2.109s | 1 |
| `demo/absences-filters-tabs.spec.ts` | 2.104s | 1 |
| `demo/absences-selection.spec.ts` | 2.090s | 1 |
| `demo/absences-page.spec.ts` | 2.073s | 1 |

The focused command now reports `5 passed (9.4s)` including the shared `auth-setup` dependency. The next CI Playwright artifact should confirm whether the prior absences shard tail is reduced. The next measured demo E2E candidates are `balance-confirmations`, `data-verification`, and `invoices`.

CI run `27312564983` on commit `c563114` confirmed the prior absences tail was reduced. The four replacement absences specs totaled about 43.095s, down from the prior 137.300s single-spec cost. The highest remaining single-spec cost shifted to `demo/data-verification.spec.ts` at 134.689s across 13 tests.

## Follow-up: Data Verification Workflow Split

Replaced `demo/data-verification.spec.ts` with route-group workflow files and moved repeated route verification into `demo/data-verification-utils.ts`:

| New spec | Coverage moved |
|----------|----------------|
| `demo/data-verification-core.spec.ts` | Dashboard, accounts, and journal page-load/error checks |
| `demo/data-verification-receivables.spec.ts` | Contacts, invoices, and payments page-load/error checks |
| `demo/data-verification-payroll-tax.spec.ts` | Employees, payroll, and TSD page-load/error checks |
| `demo/data-verification-operations.spec.ts` | Recurring invoices, banking, and reports page-load/error checks |
| `demo/data-verification-api.spec.ts` | Demo API seeded-data count checks |

The old file repeated auth, tenant setup, default `networkidle`, and generic loading checks for 12 separate page tests. The replacement keeps all 12 route visits and the API count check, but each route group pays session setup once and uses route-owned readiness selectors with `waitForNetworkIdle: false`. The helper also checks that no visible `.alert-error` is present on every verified route, extending the previous dashboard-only error-alert assertion.

Local focused baseline against the branch API showed `demo/data-verification.spec.ts` at 24.623s of Playwright result time and `14 passed (13.2s)` including auth setup. After the split, the same route/API coverage reported:

| Spec | Executed time | Tests |
|------|---------------|-------|
| `demo/data-verification-receivables.spec.ts` | 3.332s | 1 |
| `demo/data-verification-core.spec.ts` | 2.883s | 1 |
| `demo/data-verification-operations.spec.ts` | 2.076s | 1 |
| `demo/data-verification-payroll-tax.spec.ts` | 0.974s | 1 |
| `demo/data-verification-api.spec.ts` | 0.028s | 1 |

The focused command now reports `6 passed (11.2s)` including the shared `auth-setup` dependency. The next CI Playwright artifact should confirm whether the prior data-verification shard tail is distributed. The next measured demo E2E candidates are `balance-confirmations`, `tax-overview`, and `invoices`.

CI run `27315694773` on commit `352a98d` confirmed the prior data-verification tail was distributed. The five replacement data-verification specs totaled about 90.822s, down from the prior 134.689s single-spec cost. The highest remaining single-spec cost shifted to `demo/balance-confirmations.spec.ts` at 120.974s across 12 micro-tests.

## Follow-up: Balance Confirmations Workflow Consolidation

Replaced `demo/balance-confirmations.spec.ts` with workflow-sized files and moved repeated route/data helpers into `demo/balance-confirmations-utils.ts`:

| New spec | Coverage moved |
|----------|----------------|
| `demo/balance-confirmations-page.spec.ts` | Heading, balance-type selector, as-of date input, generate button, and terminal result state |
| `demo/balance-confirmations-summary.spec.ts` | Default receivables summary heading, statistics, table headers, and total row when data exists |
| `demo/balance-confirmations-filters.spec.ts` | Payables switch and as-of date regeneration behavior |
| `demo/balance-confirmations-modal.spec.ts` | Contact detail modal open, invoice content, and close behavior |

The old file repeated auth, tenant setup, route navigation, and summary loading for 12 separate micro-tests. The replacement keeps the same user-visible behaviors but folds them into 4 workflow tests. Route setup now opens `/reports/balance-confirmations` with `waitForNetworkIdle: false`, waits for route-owned controls, and then waits for the route's terminal content state (`.summary-card`, `.empty-state`, or `.alert-error`).

Local focused baseline against the branch API showed `demo/balance-confirmations.spec.ts` at 20.587s of Playwright result time and `13 passed (13.1s)` including auth setup. After workflow consolidation, the same visible behaviors reported:

| Spec | Executed time | Tests |
|------|---------------|-------|
| `demo/balance-confirmations-filters.spec.ts` | 2.552s | 1 |
| `demo/balance-confirmations-summary.spec.ts` | 2.335s | 1 |
| `demo/balance-confirmations-modal.spec.ts` | 1.408s | 1 |
| `demo/balance-confirmations-page.spec.ts` | 1.338s | 1 |

The focused command now reports `5 passed (10.2s)` including the shared `auth-setup` dependency. The next CI Playwright artifact should confirm whether the prior balance-confirmations shard tail is reduced. The next measured demo E2E candidates are `tax-overview`, `invoices`, and `cash-payments`.

CI run `27319462883` on commit `93dd33d` confirmed the prior balance-confirmations tail was reduced. The four replacement balance-confirmations specs totaled about 40.871s, down from the prior 120.974s single-spec cost. The highest remaining single-spec cost shifted to `demo/invoices.spec.ts` at 114.643s across 10 micro-tests.

## Follow-up: Invoices Workflow Consolidation

Replaced `demo/invoices.spec.ts` with workflow-sized files and moved repeated route/modal helpers into `demo/invoices-utils.ts`:

| New spec | Coverage moved |
|----------|----------------|
| `demo/invoices-page.spec.ts` | Heading, new-invoice control, terminal list/empty state, and table header presence |
| `demo/invoices-modal.spec.ts` | Create-invoice modal open, required fields, inline-contact button presence, and close behavior |
| `demo/invoices-inline-contact.spec.ts` | Inline contact modal open, contact creation API response, and new contact selection in the invoice form |

The old file repeated auth, tenant setup, route navigation, invoice loading, and modal opening for 10 separate micro-tests. The replacement keeps the same visible workflows while each workflow pays route setup once. Route setup now opens `/invoices` with `waitForNetworkIdle: false`, waits for route-owned controls, and then waits for the route's terminal content state (`table tbody tr`, `.empty-state`, or `.alert-error`).

Local focused baseline against the branch API showed `demo/invoices.spec.ts` at 19.342s of Playwright result time and `11 passed (13.4s)` including auth setup. After workflow consolidation, the same visible behaviors reported:

| Spec | Executed time | Tests |
|------|---------------|-------|
| `demo/invoices-inline-contact.spec.ts` | 1.740s | 1 |
| `demo/invoices-modal.spec.ts` | 1.663s | 1 |
| `demo/invoices-page.spec.ts` | 1.616s | 1 |

The focused command now reports `4 passed (9.0s)` including the shared `auth-setup` dependency. The next CI Playwright artifact should confirm whether the prior invoices shard tail is reduced. The next measured demo E2E candidates are `tax-overview`, `cost-centers`, and `inventory`.

CI run `27321518687` on commit `0aa4b4b` confirmed the prior invoices tail was reduced. The three replacement invoice specs totaled about 44.783s, down from the prior 114.643s single-spec cost. The highest remaining single-spec cost shifted to `demo/cost-centers.spec.ts` at 86.009s across 7 micro-tests.

## Follow-up: Cost Centers Workflow Consolidation

Replaced `demo/cost-centers.spec.ts` with workflow-sized files and moved repeated route/modal helpers into `demo/cost-centers-utils.ts`:

| New spec | Coverage moved |
|----------|----------------|
| `demo/cost-centers-page.spec.ts` | Heading, add button, terminal list/empty state, allocation section, journal-line selector, cost-center selector, amount input, and create-allocation button |
| `demo/cost-centers-modal.spec.ts` | Create modal open, form fields, active checkbox, and close behavior |

The old file repeated auth, tenant setup, route navigation, loading waits, and modal opening for 7 separate micro-tests. The replacement keeps the same visible workflows while each workflow pays route setup once. Route setup now opens `/settings/cost-centers` with `waitForNetworkIdle: false`, waits for route-owned controls, hides the spinner, and then waits for terminal route state plus the allocation form.

Local focused baseline against the branch API showed `demo/cost-centers.spec.ts` at 11.653s of Playwright result time and `8 passed (11.1s)` including auth setup. After workflow consolidation, the same visible behaviors reported:

| Spec | Executed time | Tests |
|------|---------------|-------|
| `demo/cost-centers-modal.spec.ts` | 1.296s | 1 |
| `demo/cost-centers-page.spec.ts` | 1.213s | 1 |

The focused command now reports `3 passed (10.9s)` including the shared `auth-setup` dependency. The next CI Playwright artifact should confirm whether the prior cost-centers shard tail is reduced. The next measured demo E2E candidates are `tax-overview`, `fixed-assets`, and `cash-payments`.

CI run `27324099955` on commit `9e592f4` confirmed the prior cost-centers tail was reduced. The two replacement cost-center specs totaled about 20.097s, down from the prior 86.009s single-spec cost. The highest remaining single-spec cost shifted to `demo/inventory.spec.ts` at 99.776s across 6 tests and 9 attempts, with retries in filter and transfer workflows.

## Follow-up: Inventory Workflow Consolidation

Replaced `demo/inventory.spec.ts` with workflow-sized files and moved repeated route/API helpers into `demo/inventory-utils.ts`:

| New spec | Coverage moved |
|----------|----------------|
| `demo/inventory-page.spec.ts` | Inventory heading, product filters, seeded product controls, warehouses tab, and product categories tab |
| `demo/inventory-filters.spec.ts` | Search, type, and category product filters with query-specific response waits |
| `demo/inventory-product.spec.ts` | Product create modal, field payload, rendered row, and delete behavior |
| `demo/inventory-stock.spec.ts` | Test-owned product stock adjustment with lot/serial/expiry metadata, movements modal, and warehouse transfer |

The old file repeated auth, tenant setup, route navigation, product loading, and tab/modal opening for 6 separate tests. It also mutated the seeded `PROD-001` product in the stock metadata and transfer tests, which made CI retries slower and risked retry-state coupling. The replacement keeps the same visible workflows but creates a test-owned product for stock mutation and combines stock metadata plus transfer into one user workflow.

Local focused baseline against the branch API showed `demo/inventory.spec.ts` at 12.083s of Playwright result time and `7 passed (10.6s)` including auth setup. After workflow consolidation, the same visible behaviors reported:

| Spec | Executed time | Tests |
|------|---------------|-------|
| `demo/inventory-stock.spec.ts` | 3.488s | 1 |
| `demo/inventory-product.spec.ts` | 2.782s | 1 |
| `demo/inventory-filters.spec.ts` | 2.623s | 1 |
| `demo/inventory-page.spec.ts` | 2.602s | 1 |

The focused command now reports `5 passed (10.3s)` including the shared `auth-setup` dependency. The next CI Playwright artifact should confirm whether inventory retries are gone and whether the next measured demo E2E candidates remain `fixed-assets`, `all-views`, `tax-overview`, and `cash-payments`.

CI run `27324630474` on commit `3bef124` confirmed the inventory retries were removed. The four replacement inventory specs totaled about 60.194s in CI, all with one attempt each, down from the prior `demo/inventory.spec.ts` at 99.776s across 6 tests and 9 attempts. The highest remaining demo E2E cost shifted to the legacy `demo/all-views.spec.ts` at 85.483s across 24 route-smoke micro-tests, followed by `demo/cash-payments.spec.ts` at 82.270s and `demo/fixed-assets.spec.ts` at 78.980s.

## Follow-up: Legacy All Views Removal

Removed `demo/all-views.spec.ts` instead of splitting it again. It was a legacy broad smoke file whose 24 tests duplicated newer route-owned coverage:

| Former `all-views` area | Current owner |
|-------------------------|---------------|
| Landing and login render/auth checks | `demo/env-health-auth.spec.ts` and root `auth.spec.ts` |
| Dashboard, accounts, journal, invoices, payments, recurring, contacts, reports, employees, payroll, TSD, and banking route-load checks | `demo/data-verification-*.spec.ts` plus route-specific workflow specs |
| Banking import, tax, settings, company settings, email settings, and plugin settings | `demo/bank-import.spec.ts`, `demo/tax-overview.spec.ts`, `demo/settings.spec.ts`, `demo/email-settings.spec.ts`, and `demo/plugins-settings.spec.ts` |
| Desktop and mobile navigation checks | `demo/dashboard.spec.ts`, `demo/mobile-navigation.spec.ts`, and `demo/mobile-layout.spec.ts` |

The only route smoke check that was unique to the legacy file was `/admin/plugins`, so that moved into `demo/plugins-settings.spec.ts` as the route-specific owner. The expected CI outcome is removal of the 85.483s `all-views` timing bucket without reducing user-facing route coverage.

CI run `27325776245` on commit `d8523bd` confirmed the prior tax overview tail was reduced. `demo/tax-overview.spec.ts` reported 12.894s across one test, down from the prior 75.973s across five tests. The highest remaining single-spec costs shifted to:

| Spec | Executed time | Tests |
|------|---------------|-------|
| `demo/cash-payments.spec.ts` | 71.273s | 6 |
| `auth.spec.ts` | 69.866s | 8 |
| `demo/plugins-settings.spec.ts` | 69.060s | 6 |
| `demo/fixed-assets.spec.ts` | 65.213s | 6 |
| `demo/payments.spec.ts` | 65.062s | 6 |

## Follow-up: Cash Payments Workflow Consolidation

Consolidated `demo/cash-payments.spec.ts` from six repeated route tests into two user workflows:

| Workflow | Coverage |
|----------|----------|
| Cash ledger and type filters | Heading, summary cards, new-payment control, type filter options, terminal table/empty state, cash-in create, cash-out create, and received/made/all filter behavior |
| Cash reversal | Cash receipt create, reversal modal payload, reversal API response links, original reversed state, offsetting cash payment row, and made-filter behavior |

The old file repeated auth, tenant setup, route navigation, payment loading, and contact loading for four structure checks plus two mutation workflows. The replacement keeps the same user-visible assertions while each workflow pays route setup once. Route setup now opens `/payments/cash` with `waitForNetworkIdle: false` and waits for the route-owned payments and contacts API responses.

Local focused baseline against the branch API showed `demo/cash-payments.spec.ts` at 13.417s across 6 tests. After consolidation, the same visible behaviors reported:

| Spec | Executed time | Tests |
|------|---------------|-------|
| `demo/cash-payments.spec.ts` | 2.416s | 2 |

The focused command now reports `3 passed (10.3s)` including the shared `auth-setup` dependency. The next CI Playwright artifact should confirm whether the prior cash-payments shard tail is reduced. The next measured demo E2E candidates are `auth.spec.ts`, `plugins-settings`, `fixed-assets`, and `payments`.

CI run `27326205809` on commit `48325c6` confirmed the prior cash-payments tail was reduced. `demo/cash-payments.spec.ts` reported 25.681s across two tests, down from the prior 71.273s across six tests. The highest remaining single-spec costs shifted to:

| Spec | Executed time | Tests |
|------|---------------|-------|
| `demo/dashboard.spec.ts` | 71.924s | 6 |
| `demo/plugins-settings.spec.ts` | 71.152s | 6 |
| `auth.spec.ts` | 69.637s | 8 |
| `demo/payments.spec.ts` | 66.740s | 6 |
| `demo/fixed-assets.spec.ts` | 64.368s | 6 |
| `demo/bank-import.spec.ts` | 63.922s | 5 |

## Follow-up: Dashboard Workflow Consolidation

Consolidated `demo/dashboard.spec.ts` from six repeated page-structure checks into one route-owned dashboard shell workflow. The replacement keeps the same visible coverage: tenant selector, dashboard heading, new-organization control, navigation links, four summary cards, cash-flow card, recent activity, revenue-vs-expenses chart, and quick actions.

The old file repeated auth, tenant setup, dashboard navigation, and broad network-idle waits for every section check. The replacement waits for the dashboard-owned API responses (`/me/tenants`, dashboard analytics, revenue-expense, and cash-flow), opens `/dashboard` with `waitForNetworkIdle: false`, and then asserts the loaded shell once.

Local focused baseline against the branch API showed `demo/dashboard.spec.ts` at 12.436s across 6 tests. After workflow consolidation, the same visible behaviors reported:

| Spec | Executed time | Tests |
|------|---------------|-------|
| `demo/dashboard.spec.ts` | 0.774s | 1 |

The focused command now reports `2 passed (7.8s)` including the shared `auth-setup` dependency. The next CI Playwright artifact should confirm whether the prior dashboard shard tail is reduced. The next measured demo E2E candidates are `plugins-settings`, `auth.spec.ts`, `payments`, `fixed-assets`, and `bank-import`.

CI run `27326646716` on commit `dddc1ba` confirmed the prior dashboard tail was reduced. `demo/dashboard.spec.ts` reported 12.689s across one test, down from the prior 71.924s across six tests. The highest remaining single-spec costs shifted to:

| Spec | Executed time | Tests |
|------|---------------|-------|
| `auth.spec.ts` | 69.242s | 8 |
| `demo/bank-import.spec.ts` | 66.039s | 5 |
| `demo/plugins-settings.spec.ts` | 65.206s | 6 |
| `demo/payments.spec.ts` | 62.842s | 6 |
| `demo/fixed-assets.spec.ts` | 62.058s | 6 |

## Follow-up: Auth Spec Consolidation

Consolidated root `auth.spec.ts` from eight unauthenticated login micro-tests into three workflow checks:

| Workflow | Coverage |
|----------|----------|
| Login and register form controls | Login heading, email/password inputs, submit button, input values, login password validation, register mode toggle, name field, and register password minlength |
| Invalid credentials | Invalid login POST response status, visible error state, and staying on the login route |
| Demo password validation | Demo login password length is accepted by browser validation in login mode without submitting a duplicate full login |

The removed `test@example.com` login attempt was a stale legacy assertion: no seeded user backs it in the demo run, and the test only asserted that the URL was either dashboard or login. Full demo login success remains covered by the shared `auth.setup.ts` dependency and `demo/env-health-auth.spec.ts`, so the root auth spec no longer duplicates that full login path.

Local focused baseline against the branch API showed `auth.spec.ts` at 12.119s across 8 tests. After consolidation, the same login/register coverage reported:

| Spec | Executed time | Tests |
|------|---------------|-------|
| `auth.spec.ts` | 4.435s | 3 |

The focused command now reports `4 passed (8.4s)` including the shared `auth-setup` dependency. The login page still keeps its local `networkidle` readiness wait because clicking before Svelte hydration can turn the submit into a native page reload instead of an API-backed login attempt. The next CI Playwright artifact should confirm whether the prior auth shard tail is reduced. The next measured demo E2E candidates are `bank-import`, `plugins-settings`, `payments`, and `fixed-assets`.

CI run `27327274971` on commit `f8b4413` confirmed the prior auth tail was reduced. `auth.spec.ts` reported 33.333s across three tests, down from the prior 69.242s across eight tests. The highest remaining single-spec costs shifted to:

| Spec | Executed time | Tests |
|------|---------------|-------|
| `demo/payments.spec.ts` | 61.792s | 6 |
| `demo/cash-flow.spec.ts` | 59.235s | 6 |
| `demo/bank-import.spec.ts` | 59.154s | 5 |
| `demo/fixed-assets.spec.ts` | 58.870s | 6 |
| `demo/plugins-settings.spec.ts` | 58.502s | 6 |

The next measured demo E2E candidates are `payments`, `cash-flow`, `bank-import`, `fixed-assets`, and `plugins-settings`.

CI run `27327520607` on commit `bb0340e` showed `demo/payments.spec.ts` as one
of the highest remaining repeated-workflow costs before the next stage:

| Spec | Executed time | Tests | Attempts |
|------|---------------|-------|----------|
| `demo/payments.spec.ts` | 70.776s | 6 | 6 |
| `demo/bank-import.spec.ts` | 76.390s | 5 | 6 |
| `demo/plugins-settings.spec.ts` | 68.196s | 6 | 6 |
| `demo/fixed-assets.spec.ts` | 62.168s | 6 | 6 |
| `demo/recurring.spec.ts` | 59.866s | 5 | 5 |
| `demo/cash-flow.spec.ts` | 59.185s | 6 | 6 |

## Follow-up: Payments Workflow Consolidation

Consolidated `demo/payments.spec.ts` from six repeated route tests into three
workflow checks:

| Workflow | Coverage |
|----------|----------|
| Payment ledger shell | Heading, new-payment control, type filter options, and loaded table/empty terminal state |
| Received payment allocation | Payment create modal, invoice allocation payload, payment API response, rendered row, unallocated amount, and received/made filter behavior |
| Payment reversal | Payment create modal, reversal modal payload, reversal API response links, original reversed state, offsetting payment row, and made-filter behavior |

The old file repeated auth, tenant setup, payments navigation, payment loading,
and sent-invoice loading for four structure checks plus two mutation workflows.
The replacement keeps the same user-visible assertions while each workflow pays
route setup once. Route setup now opens `/payments` with `waitForNetworkIdle:
false` and waits for route-owned payment and sent-invoice API responses. Payment
creation and type filtering now use shared helpers, matching the newer cash
payments pattern.

After consolidation, focused local E2E against the branch API reported:

| Spec | Executed time | Tests |
|------|---------------|-------|
| `demo/payments.spec.ts` | 4.397s | 3 |

The focused command reported `4 passed (8.7s)` including the shared
`auth-setup` dependency. The next CI Playwright artifact should confirm whether
the prior payments shard tail is reduced. The next measured demo E2E candidates
from run `27327520607` are `bank-import`, `plugins-settings`, `fixed-assets`,
`recurring`, and `cash-flow`.

CI run `27328010552` on commit `b950ab8` confirmed the prior payments tail was
reduced. `demo/payments.spec.ts` reported 35.977s across three tests, down from
the prior 70.776s across six tests. The highest remaining single-spec costs
shifted to:

| Spec | Executed time | Tests |
|------|---------------|-------|
| `demo/fixed-assets.spec.ts` | 64.188s | 6 |
| `demo/recurring.spec.ts` | 60.827s | 5 |
| `demo/plugins-settings.spec.ts` | 57.616s | 6 |
| `demo/env-onboarding.spec.ts` | 57.542s | 5 |
| `demo/employees.spec.ts` | 55.121s | 4 |

The next measured demo E2E candidates are `fixed-assets`, `recurring`,
`plugins-settings`, `env-onboarding`, and `employees`.

## Follow-up: Plugin Settings Spec Consolidation

CI run `27328276032` on commit `241a4dc` completed successfully and showed the broad demo E2E shard tail had been reduced enough that smaller repeated-navigation specs were now worth addressing. Uploaded Playwright artifacts showed the highest current demo spec costs as:

| Spec | Executed time | Tests |
|------|---------------|-------|
| `demo/plugins-settings.spec.ts` | 64.293s | 6 |
| `demo/cash-flow.spec.ts` | 59.681s | 6 |
| `demo/fixed-assets.spec.ts` | 59.185s | 6 |
| `demo/bank-import.spec.ts` | 57.338s | 5 |
| `demo/journal.spec.ts` | 56.229s | 4 |
| `demo/env-onboarding.spec.ts` | 55.234s | 5 |
| `demo/recurring.spec.ts` | 54.348s | 5 |

Locally, `demo/plugins-settings.spec.ts` first measured at 29.215s of Playwright result time across six route tests, with the focused command reporting seven total tests including `auth-setup` in 18.1s wall time.

The plugin settings coverage was consolidated from five tenant-route micro-tests into one route-owned workflow assertion, while keeping the admin plugin route check separate. The helpers now wait for the owning API responses and terminal route states instead of treating `.loading` as a successful loaded state.

That stricter readiness check exposed a product/API bug: `GET /api/v1/tenants/{tenantID}/plugins` returned JSON `null` for tenants with no plugin rows. The UI then stayed on `Loading plugins...`. The plugin repository and service now normalize empty tenant plugin lists to `[]`, with service coverage asserting a non-nil empty slice.

After the change, the same focused local command passed with three total tests in 8.4s wall time, and the reporter showed:

| Spec | Executed time | Tests |
|------|---------------|-------|
| `demo/plugins-settings.spec.ts` | 2.496s | 2 |
| `demo/auth.setup.ts` | 2.879s | 1 |

The next measured demo E2E candidates from run `27328276032` are `cash-flow`, `fixed-assets`, `bank-import`, `journal`, `env-onboarding`, and `recurring`.

CI run `27329103415` on commit `2e13d24` confirmed the plugin settings tail was reduced in the required PR gate. `demo/plugins-settings.spec.ts` reported 18.981s across two tests, down from 64.293s across six tests in run `27328276032`.

The highest remaining single-spec costs from run `27329103415` are:

| Spec | Executed time | Tests |
|------|---------------|-------|
| `demo/fixed-assets.spec.ts` | 68.327s | 6 |
| `demo/journal.spec.ts` | 61.108s | 4 |
| `demo/env-onboarding.spec.ts` | 60.164s | 5 |
| `demo/recurring.spec.ts` | 57.860s | 5 |
| `demo/cash-flow.spec.ts` | 56.657s | 6 |
| `demo/bank-import.spec.ts` | 55.604s | 5 |

The next measured demo E2E candidates are `fixed-assets`, `journal`, `env-onboarding`, `recurring`, `cash-flow`, and `bank-import`.

CI run `27329365124` on commit `4616329` completed successfully and showed
`demo/bank-import.spec.ts` as the highest remaining measured demo spec. It also
showed one retry in the import mutation workflow:

| Spec | Executed time | Tests | Attempts |
|------|---------------|-------|----------|
| `demo/bank-import.spec.ts` | 84.114s | 5 | 6 |
| `demo/fixed-assets.spec.ts` | 64.119s | 6 | 6 |
| `demo/journal.spec.ts` | 61.428s | 4 | 4 |
| `demo/env-onboarding.spec.ts` | 57.149s | 5 | 5 |
| `demo/recurring.spec.ts` | 54.303s | 5 | 5 |
| `demo/tsd.spec.ts` | 53.466s | 5 | 5 |
| `demo/payroll.spec.ts` | 52.251s | 5 | 5 |
| `demo/quotes.spec.ts` | 52.020s | 4 | 4 |

The flaky attempt failed while waiting for the imported transaction row after
the POST succeeded and the browser had navigated back to banking. The retry
passed, so the weak point was route readiness: the test waited for URL state,
then asserted rendered transaction data before the banking ledger transaction
request was owned by the test.

## Follow-up: Bank Import Spec Consolidation

Consolidated `demo/bank-import.spec.ts` from five repeated route tests into two
workflow checks:

| Workflow | Coverage |
|----------|----------|
| Import settings and preview | Import page shell, bank-account selector, preset options, duplicate toggle, disabled import button, LHV CSV upload, preview columns, and enabled import button |
| LHV statement import | Import POST response, imported/skipped counts, success dialog, banking ledger reload, rendered imported row, contact name, amount, and unmatched status |

The route helpers now open `/banking/import` and `/banking` with
`waitForNetworkIdle: false`, then wait for route-owned bank account,
transaction, and import API responses. The import test no longer relies on a
URL-only wait before looking for the transaction row.

Local focused baseline against the branch API showed:

| Spec | Executed time | Tests |
|------|---------------|-------|
| `demo/bank-import.spec.ts` | 10.771s | 5 |

The baseline focused command reported `6 passed (11.4s)` including the shared
`auth-setup` dependency. After consolidation, the same coverage reported:

| Spec | Executed time | Tests |
|------|---------------|-------|
| `demo/bank-import.spec.ts` | 2.543s | 2 |

The focused command reported `3 passed (8.8s)` including the shared
`auth-setup` dependency. The next CI Playwright artifact should confirm whether
the prior bank-import shard tail and retry are gone. The next measured demo E2E
candidates from run `27329365124` are `fixed-assets`, `journal`,
`env-onboarding`, `recurring`, `tsd`, `payroll`, and `quotes`.

CI run `27330165993` on commit `5fec7fe` confirmed the bank import change in
the required PR gate:

| Spec | Before | After |
|------|--------|-------|
| `demo/bank-import.spec.ts` | 84.114s, 5 tests, 6 attempts | 29.611s, 2 tests, 2 attempts |

The import retry is gone in this run. The highest remaining measured demo E2E
candidates are now:

| Spec | Executed time | Tests | Attempts |
|------|---------------|-------|----------|
| `demo/recurring.spec.ts` | 61.758s | 5 | 5 |
| `demo/fixed-assets.spec.ts` | 61.669s | 6 | 6 |
| `demo/cash-flow.spec.ts` | 61.066s | 6 | 6 |
| `demo/tsd.spec.ts` | 59.630s | 5 | 5 |
| `demo/quotes.spec.ts` | 59.496s | 4 | 4 |
| `demo/journal.spec.ts` | 58.062s | 4 | 4 |
| `demo/payroll.spec.ts` | 55.227s | 5 | 5 |

CI run `27330447206` on commit `b28d354` confirmed the follow-up docs-only
state and kept the required PR gate green. The highest remaining measured demo
E2E candidates were now smaller repeated-navigation specs:

| Spec | Executed time | Tests | Attempts |
|------|---------------|-------|----------|
| `demo/cash-flow.spec.ts` | 63.060s | 6 | 6 |
| `demo/recurring.spec.ts` | 59.651s | 5 | 5 |
| `demo/quotes.spec.ts` | 57.714s | 4 | 4 |
| `demo/tsd.spec.ts` | 56.873s | 5 | 5 |
| `demo/fixed-assets.spec.ts` | 56.597s | 6 | 6 |
| `demo/salary-calculator.spec.ts` | 53.278s | 5 | 5 |
| `demo/vat-returns.spec.ts` | 51.931s | 6 | 6 |
| `demo/env-onboarding.spec.ts` | 51.701s | 5 | 5 |
| `demo/payroll.spec.ts` | 51.608s | 5 | 5 |

## Follow-up: Cash Flow Spec Consolidation

Consolidated `demo/cash-flow.spec.ts` from six repeated page-structure tests
into two workflow checks:

| Workflow | Coverage |
|----------|----------|
| Report controls and loaded statement | Heading, start/end date inputs, generate button, back link, report container, operating/investing/financing sections, and cash summary labels |
| Regenerate changed period | Date range edits, cash-flow API response parameters, response body dates, and rendered report period |

The route helper now opens `/reports/cash-flow` with `waitForNetworkIdle: false`,
then waits for the route-owned cash-flow statement API response and
terminal report UI. This keeps coverage tied to the actual report load instead
of repeatedly navigating to the same page and polling for generic loading text.

Local focused baseline against the branch API showed:

| Spec | Executed time | Tests |
|------|---------------|-------|
| `demo/cash-flow.spec.ts` | 10.245s | 6 |

The baseline focused command reported `7 passed (11.5s)` including the shared
`auth-setup` dependency. After consolidation, the same coverage reported:

| Spec | Executed time | Tests |
|------|---------------|-------|
| `demo/cash-flow.spec.ts` | 1.710s | 2 |

The focused command reported `3 passed (9.3s)` including the shared
`auth-setup` dependency. The next CI Playwright artifact should confirm whether
the prior cash-flow shard tail is reduced. The next measured demo E2E
candidates from run `27330447206` are `recurring`, `quotes`, `tsd`,
`fixed-assets`, `salary-calculator`, `vat-returns`, `env-onboarding`, and
`payroll`.
