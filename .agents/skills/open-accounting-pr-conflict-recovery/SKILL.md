---
name: open-accounting-pr-conflict-recovery
description: Use when an open-accounting PR has merge conflicts, conflicted frontend tests, stale branch state, or a pushed stage needs conflict resolution followed by local tests and PR check recovery.
---

# Open Accounting PR Conflict Recovery

Use this with `open-accounting-stage-gate` when GitHub says the branch has conflicts or a PR stage needs recovery after main moved.

## Conflict Workflow

1. Inspect state first:
   ```bash
   git status --short --branch
   gh pr view --json number,url,headRefName,headRefOid,mergeable,isDraft,statusCheckRollup
   git fetch origin main
   ```
2. Bring the branch up to date with the least surprising repo workflow. Prefer merging `origin/main` into the feature branch when preserving every stage commit matters. Use rebase only if the user asks for it or the branch has not been pushed.
3. For conflicted files, read both sides before editing:
   ```bash
   git diff --name-only --diff-filter=U
   git diff --ours -- <file>
   git diff --theirs -- <file>
   ```
4. Resolve toward current product behavior and current tests. Do not keep duplicate legacy branches, stale compatibility helpers, or both versions of a parser/test just to avoid choosing.
5. If the conflict is in a Svelte component or frontend test, load `open-accounting-frontend-workflow`, run the Svelte MCP autofixer when available, and run the focused component or E2E test that owns the conflicted behavior before broader gates.
6. If the conflict is in a test file, preserve the newest product expectation and make the test prove the intended workflow rather than merely matching either side of the conflict.

## Post-Resolution Gates

Run focused checks for the conflicted surface, then enough broad checks for confidence:

```bash
git diff --check
cd frontend && bun run check
cd frontend && bun run test -- <focused-test-file>
```

For backend/import conflicts:

```bash
go test -count=1 ./internal/banking/mappers/...
make test-cli-coverage
go test -timeout=3m ./docs -count=1
```

For demo workflow conflicts, use `open-accounting-demo-e2e` and run the affected Playwright spec against the branch API.

## Push And PR Recovery

- Commit the conflict resolution separately when it is substantial enough to review.
- Push immediately after focused and relevant broad gates pass.
- Re-check the PR mergeable state and remote checks:
  ```bash
  gh pr view --json number,url,headRefOid,mergeable,isDraft,statusCheckRollup
  gh pr checks --watch --interval 15
  ```
- If CI fails after the conflict resolution, inspect the failed logs and fix forward on the same branch. Do not revert unrelated user changes.
