# Ralph Loop: Increase Test Coverage to Meet Threshold

> **For Claude:** This is an autonomous Ralph Loop. Execute until the completion promise is satisfied or max iterations reached.

**Goal:** Increase Go test coverage from 54.25% to at least 67% (within 2% of main branch 68.83%)

**Completion Promise:** `go test ./... -cover` shows overall coverage >= 67% AND all new tests pass

**Max Iterations:** 30

---

## Command to Run This Loop

```bash
claude "Execute the Ralph Loop defined in docs/plans/2026-01-06-coverage-ralph-loop.md.

Your task: Systematically add unit tests to increase Go test coverage from ~54% to >=67%.

LOOP INSTRUCTIONS:
1. Check current coverage: go test ./... -cover 2>&1 | grep coverage
2. Find lowest coverage package that isn't 100%
3. Analyze what functions need tests: go test -coverprofile=cov.out ./internal/PACKAGE/... && go tool cover -func=cov.out | grep -v 100.0
4. Write comprehensive table-driven tests following existing patterns
5. Verify tests pass: go test ./internal/PACKAGE/... -v
6. Commit progress: git add . && git commit -m 'test(PACKAGE): add unit tests - coverage X%'
7. Check overall coverage - if >= 67%, push and exit. Otherwise, continue to next package.

COMPLETION CRITERIA:
- Overall coverage >= 67%
- All tests pass
- Changes committed and pushed

Focus on these packages in order:
1. internal/orders (0%)
2. internal/quotes (0%)
3. internal/invoicing (30.8%)
4. internal/reports (42.3%)
5. internal/banking (35%)
6. internal/inventory (37.4%)

DO NOT stop until coverage >= 67% or you've exhausted reasonable test additions."
```

---

## Current State

| Package | Coverage | Priority |
|---------|----------|----------|
| `internal/orders` | 0.0% | HIGH |
| `internal/quotes` | 0.0% | HIGH |
| `internal/invoicing` | 30.8% | HIGH |
| `internal/database` | 34.2% | MEDIUM |
| `internal/banking` | 35.0% | MEDIUM |
| `internal/inventory` | 37.4% | MEDIUM |
| `internal/reports` | 42.3% | MEDIUM |
| `internal/accounting` | 44.5% | MEDIUM |
| `internal/assets` | 45.6% | MEDIUM |
| `internal/payments` | 47.6% | LOW |
| `internal/payroll` | 49.5% | LOW |
| `internal/tenant` | 49.9% | LOW |

---

## Loop Instructions

### Each Iteration

1. **Check Current Coverage**
   ```bash
   go test ./... -cover 2>&1 | grep -E "coverage:" | awk '{sum+=$5; count++} END {print "Average:", sum/count "%"}'
   ```

2. **Identify Lowest Coverage Package**
   ```bash
   go test ./... -cover 2>&1 | grep -E "coverage:" | sort -t':' -k2 -n | head -5
   ```

3. **Analyze Missing Coverage**
   ```bash
   go test -coverprofile=coverage.out ./internal/PACKAGE/...
   go tool cover -func=coverage.out | grep -v "100.0%"
   ```

4. **Write Tests** for uncovered functions following patterns:
   - Use table-driven tests
   - Use mock repositories (already exist in most packages)
   - Test happy path + error cases
   - Follow existing test patterns in the codebase

5. **Verify Tests Pass**
   ```bash
   go test ./internal/PACKAGE/... -v
   ```

6. **Check New Coverage**
   ```bash
   go test ./internal/PACKAGE/... -cover
   ```

7. **Commit Progress**
   ```bash
   git add internal/PACKAGE/*_test.go
   git commit -m "test(PACKAGE): add unit tests for FUNCTION - coverage X%"
   ```

---

## Package-Specific Guidance

### internal/orders (0% -> target 60%)
- Create `orders_test.go` with mock repository tests
- Test Order CRUD operations
- Test order status transitions

### internal/quotes (0% -> target 60%)
- Create `quotes_test.go` with mock repository tests
- Test Quote CRUD operations
- Test quote-to-order conversion

### internal/invoicing (30.8% -> target 60%)
- Add tests for `recurring.go` functions
- Add tests for `reminders.go` functions
- Test email reminder generation

### internal/banking (35% -> target 55%)
- Add tests for reconciliation service
- Add tests for transaction matching edge cases

### internal/reports (42.3% -> target 60%)
- Add tests for report generation
- Test aging report calculations
- Test balance confirmation generation

### internal/accounting (44.5% -> target 60%)
- Add more cost center service tests
- Test budget calculations
- Test expense allocation

---

## Test Template

```go
func TestFunctionName(t *testing.T) {
    tests := []struct {
        name    string
        input   InputType
        want    OutputType
        wantErr bool
    }{
        {
            name:  "happy path",
            input: validInput,
            want:  expectedOutput,
        },
        {
            name:    "error case",
            input:   invalidInput,
            wantErr: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := FunctionUnderTest(tt.input)
            if (err != nil) != tt.wantErr {
                t.Errorf("error = %v, wantErr %v", err, tt.wantErr)
                return
            }
            if !reflect.DeepEqual(got, tt.want) {
                t.Errorf("got = %v, want %v", got, tt.want)
            }
        })
    }
}
```

---

## Exit Conditions

**Success (exit loop):**
- Overall coverage >= 67%
- All tests pass
- No linting errors

**Failure (continue loop):**
- Coverage < 67%
- Tests failing
- More packages need coverage

---

## Progress Tracking

After each iteration, update this section:

### Iteration Log

| # | Package | Before | After | Tests Added |
|---|---------|--------|-------|-------------|
| 1 | | | | |

