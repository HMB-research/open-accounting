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

Implement weight-aware integration package sharding first. It targets the current critical path directly and should be measurable with one CI run by comparing the slowest integration shard against the current `integration-test (3, 4)` baseline.
