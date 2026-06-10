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
