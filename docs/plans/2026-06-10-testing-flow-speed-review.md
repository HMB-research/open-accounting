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
