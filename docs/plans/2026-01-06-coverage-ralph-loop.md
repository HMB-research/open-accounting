# Ralph Loop: Increase Test Coverage to Meet Threshold

> **For Claude:** This is an autonomous Ralph Loop. Execute until the completion promise is satisfied or max iterations reached.

**Goal:** Increase Go test coverage from 54.25% to at least 67% (within 2% of main branch 68.83%)

**Completion Promise:** `go test ./... -cover` shows overall coverage >= 67% AND all new tests pass

**Max Iterations:** 30

---

## Command to Run This Loop

```bash
/ralph-wiggum:ralph-loop "Systematically add unit tests to increase Go test coverage to >=67%.

EACH ITERATION:
1. Check coverage: go test ./... -cover 2>&1 | grep coverage
2. Find lowest coverage package
3. Analyze gaps: go test -coverprofile=cov.out ./internal/PACKAGE/... && go tool cover -func=cov.out | grep -v 100.0
4. Write table-driven tests with mocks
5. Verify: go test ./internal/PACKAGE/... -v
6. Commit: git add . && git commit -m 'test(PACKAGE): add unit tests'
7. If coverage >= 67%, say COVERAGE_TARGET_REACHED

Focus packages: orders, quotes, invoicing, reports, banking, inventory" --completion-promise "COVERAGE_TARGET_REACHED" --max-iterations 30
```

---

## Current State (Updated 2026-01-06)

| Package | Coverage | Status |
|---------|----------|--------|
| `internal/database` | 34.2% | Needs integration tests |
| `internal/banking` | 35.0% | Needs integration tests |
| `internal/inventory` | 37.4% | Needs integration tests |
| `internal/accounting` | 44.5% | Service layer tested |
| `internal/invoicing` | 44.0% | ✅ Improved from 30.8% |
| `internal/assets` | 45.6% | Service layer tested |
| `internal/payments` | 47.6% | Needs integration tests |
| `internal/payroll` | 49.5% | Service layer tested |
| `internal/tenant` | 49.9% | Service layer tested |
| `internal/quotes` | 51.4% | ✅ Improved from 0% |
| `internal/orders` | 53.0% | ✅ Improved from 0% |
| `internal/reports` | 63.0% | ✅ Improved from 42.3% |

**Overall: 55.63% (up from 52.81%)**

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
| 1 | internal/orders | 0.0% | 13% | types_test.go (types, validation, calculations) |
| 2 | internal/quotes | 0.0% | 15% | types_test.go (types, validation, calculations) |
| 3 | internal/invoicing | 30.8% | 44% | reminder_test.go (mock repo, service tests) |
| 4 | internal/reports | 42.3% | 63% | service_test.go (mock repo, balance confirmations) |
| 5 | internal/orders | 13% | 53% | service_test.go (mock repo, CRUD, status transitions) |
| 6 | internal/quotes | 15% | 51% | service_test.go (mock repo, CRUD, status transitions) |

### Current Overall Coverage: 55.63%

### Blocker Identified

Reaching 67% coverage requires **database integration tests** for the PostgresRepository implementations. The remaining untested code (~12%) is primarily:

1. PostgreSQL repository methods (all packages)
2. sqlc-generated database queries (internal/database)
3. Complex service methods requiring database transactions

**Recommendation:** To reach 67%, add integration tests that use a test database. Consider:
- Using testcontainers-go to spin up PostgreSQL in tests
- Creating a test fixtures framework
- Adding integration test files (*_integration_test.go) with build tags

