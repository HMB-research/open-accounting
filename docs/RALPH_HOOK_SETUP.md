# Ralph Wiggum Hook Setup & Usage Guide

This document explains how to use the Ralph Wiggum plugin to achieve automated test coverage goals.

## Overview

The Ralph Wiggum hook enables iterative AI development loops that continue until a defined success criterion is met. This is ideal for:
- Increasing test coverage to a target percentage
- Fixing all linting errors
- Making code mobile-friendly
- Any task with measurable, automated success criteria

## Installation

The Ralph Wiggum plugin has been installed at:
```
.claude/plugins/ralph-wiggum/
├── .claude-plugin          # Plugin manifest
├── commands/
│   ├── ralph-loop.md       # Start loop command
│   ├── cancel-ralph.md     # Cancel loop command
│   └── help.md             # Help documentation
├── hooks/
│   ├── hooks.json          # Hook configuration
│   └── stop-hook.sh        # Exit interception hook
└── scripts/
    └── setup-ralph-loop.sh # Loop initialization
```

## How to Run

### 1. Start a Coverage Loop

To increase test coverage to 95%:

```bash
/ralph-loop "Increase test coverage to 95%.

WORKFLOW:
1. Run 'npm run test:coverage' in frontend/ to see current coverage
2. Run 'go test -coverprofile=coverage.out ./internal/...' for backend coverage
3. Identify files with lowest coverage
4. Write comprehensive unit tests for those files
5. Run tests to verify they pass
6. Repeat until 95% coverage achieved

SUCCESS CRITERIA:
- Frontend statement coverage >= 95%
- Backend package coverage >= 95% (average)

When coverage reaches 95%, output: <promise>COVERAGE_95_ACHIEVED</promise>" --max-iterations 50 --completion-promise "COVERAGE_95_ACHIEVED"
```

### 2. Start a Mobile Responsiveness Loop

```bash
/ralph-loop "Make all pages mobile-friendly.

WORKFLOW:
1. Run E2E mobile tests: cd frontend && npm run test:e2e -- --project='Mobile Chrome'
2. Fix any failing mobile tests
3. Audit each route for mobile responsiveness:
   - Touch targets >= 44px
   - Full-width inputs on mobile
   - Proper navigation menu behavior
   - Tables scroll horizontally
   - Cards stack vertically
4. Update CSS for proper breakpoints

SUCCESS CRITERIA:
- All mobile E2E tests pass
- All pages render correctly on 375px viewport

When all mobile tests pass, output: <promise>MOBILE_READY</promise>" --max-iterations 30 --completion-promise "MOBILE_READY"
```

### 3. Start a CI Fix Loop

```bash
/ralph-loop "Fix all CI/CD pipeline failures.

WORKFLOW:
1. Check current CI status: gh run list --limit 1
2. If failing, get logs: gh run view <run-id> --log-failed
3. Fix the identified issues
4. Commit and push changes
5. Wait for CI to run
6. Repeat until CI passes

SUCCESS CRITERIA:
- All CI jobs pass (changes, test, lint, build, frontend, e2e)

When CI fully passes, output: <promise>CI_GREEN</promise>" --max-iterations 20 --completion-promise "CI_GREEN"
```

### 4. Cancel an Active Loop

```bash
/cancel-ralph
```

### 5. Check Loop Progress

```bash
# View current iteration
grep '^iteration:' .claude/ralph-loop.local.md

# View full loop state
cat .claude/ralph-loop.local.md
```

## Testing Infrastructure

### Running Unit Tests

**Frontend (Vitest):**
```bash
cd frontend
npm run test                  # Run tests once
npm run test:watch           # Watch mode
npm run test:coverage        # With coverage report
```

**Backend (Go):**
```bash
# All packages
go test ./...

# With coverage
go test -coverprofile=coverage.out ./internal/...

# View coverage report
go tool cover -func=coverage.out

# HTML coverage report
go tool cover -html=coverage.out -o coverage.html
```

### Running E2E Tests

```bash
cd frontend

# All browsers
npm run test:e2e

# Specific browser
npm run test:e2e:chromium

# Mobile only
npx playwright test --project='Mobile Chrome'

# With UI
npm run test:e2e:ui

# Debug mode
npm run test:e2e:debug
```

### Coverage Targets

| Component | Current | Target |
|-----------|---------|--------|
| Frontend Statements | 34% | 95% |
| Frontend Branches | 5% | 90% |
| Frontend Functions | 6% | 95% |
| Backend auth | 78% | 95% |
| Backend scheduler | 58% | 95% |
| Backend tax | 35% | 95% |
| Backend plugin | 29% | 95% |
| Backend invoicing | 23% | 95% |
| Backend banking | 15% | 95% |
| Backend recurring | 13% | 95% |
| Backend accounting | 11% | 95% |
| Backend payroll | 9% | 95% |
| Backend pdf | 8% | 95% |
| Backend email | 7% | 95% |
| Backend tenant | 4% | 95% |
| Backend payments | 4% | 95% |
| Backend analytics | 1% | 95% |
| Backend contacts | 0% | 95% |

## Best Practices

### 1. Set Reasonable Iteration Limits
Always use `--max-iterations` as a safety net:
- Small tasks: 10-20 iterations
- Medium tasks: 30-50 iterations
- Large tasks: 50-100 iterations

### 2. Define Clear Success Criteria
The completion promise must be:
- Measurable (can be verified by running a command)
- Objective (no human judgment required)
- Deterministic (same input always gives same result)

### 3. Use Incremental Goals
Instead of one massive loop, use multiple smaller loops:

```bash
# First: Get frontend to 70%
/ralph-loop "Get frontend coverage to 70%..." --max-iterations 20 --completion-promise "FRONTEND_70"

# Then: Get frontend to 90%
/ralph-loop "Get frontend coverage to 90%..." --max-iterations 20 --completion-promise "FRONTEND_90"

# Finally: Get to 95%
/ralph-loop "Get frontend coverage to 95%..." --max-iterations 10 --completion-promise "FRONTEND_95"
```

### 4. Monitor Progress
Keep a terminal open to watch:
```bash
watch -n 10 'cat .claude/ralph-loop.local.md'
```

## Troubleshooting

### Loop Won't Start
Check if there's already an active loop:
```bash
ls -la .claude/ralph-loop.local.md
```

Cancel it first if needed:
```bash
rm .claude/ralph-loop.local.md
```

### Loop Exits Too Early
Ensure the completion promise is specific enough and wrapped in `<promise>` tags.

### Loop Runs Forever
- Check `--max-iterations` is set
- Verify the completion criteria is achievable
- Cancel with `/cancel-ralph`

## CI/CD Verification

Current CI workflow jobs:
1. **changes** - Detects which parts of codebase changed
2. **test** - Runs Go backend tests with PostgreSQL
3. **lint** - Runs golangci-lint
4. **build** - Builds Go binaries
5. **frontend** - Type checks, tests, and builds frontend
6. **e2e** - Runs Playwright E2E tests
7. **docker** - Builds Docker image (main branch only)

To verify CI passes locally before pushing:
```bash
# Backend
go test -v ./...
golangci-lint run --timeout=5m

# Frontend
cd frontend
npm run check
npm test -- --run
npm run build
```

## File Locations

| Purpose | Location |
|---------|----------|
| Ralph plugin | `.claude/plugins/ralph-wiggum/` |
| Loop state | `.claude/ralph-loop.local.md` |
| Frontend tests | `frontend/src/tests/` |
| E2E tests | `frontend/e2e/` |
| Backend tests | `internal/*/..._test.go` |
| Coverage output | `coverage.out` (backend), `frontend/coverage/` (frontend) |
| Playwright report | `frontend/playwright-report/` |
