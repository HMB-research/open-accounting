# Test Coverage Improvement Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Increase Go test coverage from 52.1% to 67%+ through targeted unit tests for service layer business logic.

**Architecture:** Focus on service-layer unit tests using mock repositories (already established pattern). Avoid repository tests (require database). Prioritize packages with low coverage that have testable business logic: reports (42.3%), invoicing (43.4%), accounting (44.5%), banking (35.0%). Use dependency injection pattern already in place (`NewServiceWithRepository`).

**Tech Stack:** Go 1.21+, testify/assert, testify/require, shopspring/decimal

---

### Task 1: Add Balance Confirmation Tests to Reports Package

**Files:**
- Modify: `internal/reports/service_test.go`
- Reference: `internal/reports/service.go:297-379`

**Step 1: Add MockRepository methods for balance confirmation**

Add to `internal/reports/service_test.go` after existing mock struct:

```go
// Add to MockRepository struct fields:
OutstandingInvoicesByContact []ContactBalance
ContactInvoices              []BalanceInvoice
Contact                      ContactInfo
GetOutstandingErr            error
GetContactInvoicesErr        error
GetContactErr                error

// Add method implementations:
func (m *MockRepository) GetOutstandingInvoicesByContact(ctx context.Context, schemaName, tenantID string, invoiceType string, asOfDate time.Time) ([]ContactBalance, error) {
	if m.GetOutstandingErr != nil {
		return nil, m.GetOutstandingErr
	}
	return m.OutstandingInvoicesByContact, nil
}

func (m *MockRepository) GetContactInvoices(ctx context.Context, schemaName, tenantID, contactID string, invoiceType string, asOfDate time.Time) ([]BalanceInvoice, error) {
	if m.GetContactInvoicesErr != nil {
		return nil, m.GetContactInvoicesErr
	}
	return m.ContactInvoices, nil
}

func (m *MockRepository) GetContact(ctx context.Context, schemaName, tenantID, contactID string) (ContactInfo, error) {
	if m.GetContactErr != nil {
		return ContactInfo{}, m.GetContactErr
	}
	return m.Contact, nil
}
```

**Step 2: Write test for GetBalanceConfirmationSummary**

Add to `internal/reports/service_test.go`:

```go
func TestGetBalanceConfirmationSummary(t *testing.T) {
	t.Run("returns receivable summary", func(t *testing.T) {
		ctx := context.Background()
		mockRepo := NewMockRepository()
		mockRepo.OutstandingInvoicesByContact = []ContactBalance{
			{ContactID: "c1", ContactName: "Customer A", Balance: decimal.NewFromFloat(1000), InvoiceCount: 2},
			{ContactID: "c2", ContactName: "Customer B", Balance: decimal.NewFromFloat(500), InvoiceCount: 1},
		}
		svc := NewServiceWithRepository(mockRepo)

		req := &BalanceConfirmationRequest{
			Type:     "RECEIVABLE",
			AsOfDate: "2024-01-31",
		}

		result, err := svc.GetBalanceConfirmationSummary(ctx, "tenant-1", "test_schema", req)

		require.NoError(t, err)
		assert.Equal(t, BalanceTypeReceivable, result.Type)
		assert.Equal(t, "2024-01-31", result.AsOfDate)
		assert.Equal(t, 2, result.ContactCount)
		assert.Equal(t, 3, result.InvoiceCount)
		assert.True(t, result.TotalBalance.Equal(decimal.NewFromFloat(1500)))
	})

	t.Run("returns payable summary", func(t *testing.T) {
		ctx := context.Background()
		mockRepo := NewMockRepository()
		mockRepo.OutstandingInvoicesByContact = []ContactBalance{
			{ContactID: "s1", ContactName: "Supplier A", Balance: decimal.NewFromFloat(2000), InvoiceCount: 1},
		}
		svc := NewServiceWithRepository(mockRepo)

		req := &BalanceConfirmationRequest{
			Type:     "PAYABLE",
			AsOfDate: "2024-01-31",
		}

		result, err := svc.GetBalanceConfirmationSummary(ctx, "tenant-1", "test_schema", req)

		require.NoError(t, err)
		assert.Equal(t, BalanceTypePayable, result.Type)
	})

	t.Run("returns error on invalid date", func(t *testing.T) {
		ctx := context.Background()
		mockRepo := NewMockRepository()
		svc := NewServiceWithRepository(mockRepo)

		req := &BalanceConfirmationRequest{
			Type:     "RECEIVABLE",
			AsOfDate: "invalid-date",
		}

		_, err := svc.GetBalanceConfirmationSummary(ctx, "tenant-1", "test_schema", req)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid as_of_date")
	})
}
```

**Step 3: Write test for GetBalanceConfirmation**

Add to `internal/reports/service_test.go`:

```go
func TestGetBalanceConfirmation(t *testing.T) {
	t.Run("returns individual balance confirmation", func(t *testing.T) {
		ctx := context.Background()
		mockRepo := NewMockRepository()
		mockRepo.Contact = ContactInfo{
			ID:    "contact-1",
			Name:  "Test Customer",
			Code:  "CUST001",
			Email: "test@example.com",
		}
		mockRepo.ContactInvoices = []BalanceInvoice{
			{InvoiceNumber: "INV-001", InvoiceDate: "2024-01-15", DueDate: "2024-02-15", TotalAmount: decimal.NewFromFloat(1000), OutstandingAmount: decimal.NewFromFloat(500)},
		}
		svc := NewServiceWithRepository(mockRepo)

		req := &BalanceConfirmationRequest{
			Type:      "RECEIVABLE",
			AsOfDate:  "2024-01-31",
			ContactID: "contact-1",
		}

		result, err := svc.GetBalanceConfirmation(ctx, "tenant-1", "test_schema", req)

		require.NoError(t, err)
		assert.Equal(t, "contact-1", result.ContactID)
		assert.Equal(t, "Test Customer", result.ContactName)
		assert.Equal(t, "CUST001", result.ContactCode)
		assert.True(t, result.TotalBalance.Equal(decimal.NewFromFloat(500)))
		assert.Len(t, result.Invoices, 1)
	})

	t.Run("returns error when contact_id missing", func(t *testing.T) {
		ctx := context.Background()
		mockRepo := NewMockRepository()
		svc := NewServiceWithRepository(mockRepo)

		req := &BalanceConfirmationRequest{
			Type:     "RECEIVABLE",
			AsOfDate: "2024-01-31",
		}

		_, err := svc.GetBalanceConfirmation(ctx, "tenant-1", "test_schema", req)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "contact_id is required")
	})
}
```

**Step 4: Run tests to verify**

Run: `go test ./internal/reports/... -v`
Expected: All tests PASS

**Step 5: Commit**

```bash
git add internal/reports/service_test.go
git commit -m "test(reports): add balance confirmation tests

- Add GetBalanceConfirmationSummary tests
- Add GetBalanceConfirmation tests
- Add mock methods for contact/invoice queries"
```

---

### Task 2: Add Banking Import Function Tests

**Files:**
- Create: `internal/banking/import_service_test.go`
- Reference: `internal/banking/import.go:83-260`

**Step 1: Create test file with mock setup**

Create `internal/banking/import_service_test.go`:

```go
package banking

import (
	"context"
	"strings"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestImportCSV(t *testing.T) {
	t.Run("imports simple CSV successfully", func(t *testing.T) {
		repo := NewMockRepository()
		// Pre-create bank account
		repo.BankAccounts["acc-1"] = &BankAccount{
			ID:       "acc-1",
			TenantID: "tenant-1",
			Currency: "EUR",
		}
		svc := NewServiceWithRepository(repo)

		csvData := `date,amount,description
2024-01-15,100.00,Payment received
2024-01-16,-50.00,Supplier payment`

		reader := strings.NewReader(csvData)
		mapping := DefaultGenericMapping()

		result, err := svc.ImportCSV(
			context.Background(),
			"test_schema",
			"tenant-1",
			"acc-1",
			reader,
			"test.csv",
			mapping,
			true,
		)

		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, 2, result.TransactionsImported)
	})

	t.Run("returns error for invalid bank account", func(t *testing.T) {
		repo := NewMockRepository()
		repo.GetAccountErr = ErrBankAccountNotFound
		svc := NewServiceWithRepository(repo)

		csvData := `date,amount,description
2024-01-15,100.00,Test`

		reader := strings.NewReader(csvData)
		mapping := DefaultGenericMapping()

		_, err := svc.ImportCSV(
			context.Background(),
			"test_schema",
			"tenant-1",
			"invalid-id",
			reader,
			"test.csv",
			mapping,
			true,
		)

		require.Error(t, err)
	})

	t.Run("skips duplicate transactions when flag set", func(t *testing.T) {
		repo := NewMockRepository()
		repo.BankAccounts["acc-1"] = &BankAccount{ID: "acc-1", Currency: "EUR"}
		repo.DuplicateCheck = true // Mark transactions as duplicates
		svc := NewServiceWithRepository(repo)

		csvData := `date,amount,description
2024-01-15,100.00,Duplicate payment`

		reader := strings.NewReader(csvData)
		mapping := DefaultGenericMapping()

		result, err := svc.ImportCSV(
			context.Background(),
			"test_schema",
			"tenant-1",
			"acc-1",
			reader,
			"test.csv",
			mapping,
			true, // skipDuplicates = true
		)

		require.NoError(t, err)
		assert.Equal(t, 0, result.TransactionsImported)
		assert.Equal(t, 1, result.DuplicatesSkipped)
	})
}

func TestImportTransactions(t *testing.T) {
	t.Run("creates transactions from parsed rows", func(t *testing.T) {
		repo := NewMockRepository()
		repo.BankAccounts["acc-1"] = &BankAccount{ID: "acc-1", Currency: "EUR"}
		svc := NewServiceWithRepository(repo)

		transactions := []ParsedTransaction{
			{
				Date:        "2024-01-15",
				Amount:      decimal.NewFromFloat(100),
				Description: "Test payment",
			},
		}

		result, err := svc.ImportTransactions(
			context.Background(),
			"test_schema",
			"tenant-1",
			"acc-1",
			transactions,
			"test.csv",
			true,
		)

		require.NoError(t, err)
		assert.Equal(t, 1, result.TransactionsImported)
	})
}
```

**Step 2: Run tests to verify**

Run: `go test ./internal/banking/... -v -run TestImport`
Expected: Tests may fail initially due to missing types/methods - note what needs to be added to MockRepository

**Step 3: Add missing MockRepository fields if needed**

If tests fail due to missing mock methods, add to existing MockRepository in `internal/banking/service_test.go`:

```go
// Add to MockRepository struct:
DuplicateCheck bool
ImportRecords  []ImportRecord

// Add methods if not present:
func (m *MockRepository) IsTransactionDuplicate(ctx context.Context, schemaName, tenantID string, tx *BankTransaction) (bool, error) {
	return m.DuplicateCheck, nil
}

func (m *MockRepository) CreateImportRecord(ctx context.Context, schemaName string, record *ImportRecord) error {
	m.ImportRecords = append(m.ImportRecords, *record)
	return nil
}
```

**Step 4: Run tests again**

Run: `go test ./internal/banking/... -v`
Expected: All tests PASS

**Step 5: Commit**

```bash
git add internal/banking/import_service_test.go internal/banking/service_test.go
git commit -m "test(banking): add CSV import tests

- Add ImportCSV tests for successful import, errors, duplicates
- Add ImportTransactions tests
- Extend MockRepository with import-related methods"
```

---

### Task 3: Add Banking Matcher Tests

**Files:**
- Create: `internal/banking/matcher_service_test.go`
- Reference: `internal/banking/matcher.go:54-145`

**Step 1: Create matcher service tests**

Create `internal/banking/matcher_service_test.go`:

```go
package banking

import (
	"context"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetMatchSuggestions(t *testing.T) {
	t.Run("returns matching payments for transaction", func(t *testing.T) {
		repo := NewMockRepository()
		repo.Transactions["tx-1"] = &BankTransaction{
			ID:                "tx-1",
			Amount:            decimal.NewFromFloat(100),
			CounterpartyName:  "Test Customer",
			Reference:         "INV-001",
			TransactionDate:   time.Now(),
		}
		repo.UnallocatedPayments = []UnallocatedPayment{
			{
				ID:            "pay-1",
				ContactName:   "Test Customer",
				Reference:     "INV-001",
				Amount:        decimal.NewFromFloat(100),
				InvoiceNumber: "INV-001",
			},
		}
		svc := NewServiceWithRepository(repo)

		suggestions, err := svc.GetMatchSuggestions(
			context.Background(),
			"test_schema",
			"tenant-1",
			"tx-1",
			DefaultMatcherConfig(),
		)

		require.NoError(t, err)
		assert.NotEmpty(t, suggestions)
		assert.True(t, suggestions[0].Confidence > 0.5)
	})

	t.Run("returns empty for no matches", func(t *testing.T) {
		repo := NewMockRepository()
		repo.Transactions["tx-1"] = &BankTransaction{
			ID:               "tx-1",
			Amount:           decimal.NewFromFloat(999),
			CounterpartyName: "Unknown",
		}
		repo.UnallocatedPayments = []UnallocatedPayment{
			{
				ID:          "pay-1",
				ContactName: "Different Customer",
				Amount:      decimal.NewFromFloat(100),
			},
		}
		svc := NewServiceWithRepository(repo)

		suggestions, err := svc.GetMatchSuggestions(
			context.Background(),
			"test_schema",
			"tenant-1",
			"tx-1",
			DefaultMatcherConfig(),
		)

		require.NoError(t, err)
		assert.Empty(t, suggestions)
	})
}

func TestAutoMatchTransactions(t *testing.T) {
	t.Run("matches transactions above threshold", func(t *testing.T) {
		repo := NewMockRepository()
		repo.UnmatchedTransactions = []BankTransaction{
			{
				ID:               "tx-1",
				Amount:           decimal.NewFromFloat(100),
				CounterpartyName: "Exact Match Customer",
				Reference:        "INV-001",
			},
		}
		repo.UnallocatedPayments = []UnallocatedPayment{
			{
				ID:            "pay-1",
				ContactName:   "Exact Match Customer",
				Reference:     "INV-001",
				Amount:        decimal.NewFromFloat(100),
				InvoiceNumber: "INV-001",
			},
		}
		svc := NewServiceWithRepository(repo)

		config := DefaultMatcherConfig()
		config.AutoMatchThreshold = 0.8

		result, err := svc.AutoMatchTransactions(
			context.Background(),
			"test_schema",
			"tenant-1",
			"acc-1",
			config,
		)

		require.NoError(t, err)
		assert.Equal(t, 1, result.Matched)
	})
}

func TestCreatePaymentFromTransaction(t *testing.T) {
	t.Run("creates payment with correct fields", func(t *testing.T) {
		repo := NewMockRepository()
		repo.Transactions["tx-1"] = &BankTransaction{
			ID:              "tx-1",
			Amount:          decimal.NewFromFloat(100),
			TransactionDate: time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
			Description:     "Payment for invoice",
		}
		svc := NewServiceWithRepository(repo)

		payment, err := svc.CreatePaymentFromTransaction(
			context.Background(),
			"test_schema",
			"tenant-1",
			"tx-1",
			"invoice-1",
			"user-1",
		)

		require.NoError(t, err)
		assert.NotNil(t, payment)
		assert.True(t, payment.Amount.Equal(decimal.NewFromFloat(100)))
	})
}
```

**Step 2: Add MockRepository fields for matching**

Add to MockRepository in `internal/banking/service_test.go`:

```go
// Add fields:
UnmatchedTransactions []BankTransaction
UnallocatedPayments   []UnallocatedPayment

// Add methods:
func (m *MockRepository) GetUnmatchedTransactions(ctx context.Context, schemaName, tenantID, accountID string) ([]BankTransaction, error) {
	return m.UnmatchedTransactions, nil
}

func (m *MockRepository) GetUnallocatedPayments(ctx context.Context, schemaName, tenantID string) ([]UnallocatedPayment, error) {
	return m.UnallocatedPayments, nil
}
```

**Step 3: Run tests**

Run: `go test ./internal/banking/... -v -run TestMatch -run TestCreate`
Expected: Tests PASS

**Step 4: Commit**

```bash
git add internal/banking/matcher_service_test.go internal/banking/service_test.go
git commit -m "test(banking): add transaction matching tests

- Add GetMatchSuggestions tests
- Add AutoMatchTransactions tests
- Add CreatePaymentFromTransaction tests
- Extend MockRepository with matching methods"
```

---

### Task 4: Add Accounting UpdateCostCenter Test

**Files:**
- Modify: `internal/accounting/cost_centers_test.go`
- Reference: `internal/accounting/cost_centers.go:410-418`

**Step 1: Add test for UpdateCostCenter service method**

Add to `internal/accounting/cost_centers_test.go`:

```go
func TestService_UpdateCostCenter(t *testing.T) {
	t.Run("updates existing cost center", func(t *testing.T) {
		repo := NewMockCostCenterRepository()
		repo.CostCenters["cc-1"] = &CostCenter{
			ID:       "cc-1",
			TenantID: "tenant-1",
			Code:     "CC001",
			Name:     "Original Name",
			IsActive: true,
		}
		svc := NewCostCenterService(repo)

		req := &UpdateCostCenterRequest{
			Name:        "Updated Name",
			Description: "Updated description",
			ParentID:    "",
			IsActive:    true,
		}

		result, err := svc.UpdateCostCenter(context.Background(), "tenant-1", "test_schema", "cc-1", req)

		require.NoError(t, err)
		assert.Equal(t, "Updated Name", result.Name)
		assert.Equal(t, "Updated description", result.Description)
	})

	t.Run("returns error for non-existent cost center", func(t *testing.T) {
		repo := NewMockCostCenterRepository()
		svc := NewCostCenterService(repo)

		req := &UpdateCostCenterRequest{
			Name: "Test",
		}

		_, err := svc.UpdateCostCenter(context.Background(), "tenant-1", "test_schema", "not-found", req)

		require.Error(t, err)
	})
}
```

**Step 2: Run test**

Run: `go test ./internal/accounting/... -v -run TestService_UpdateCostCenter`
Expected: PASS

**Step 3: Commit**

```bash
git add internal/accounting/cost_centers_test.go
git commit -m "test(accounting): add UpdateCostCenter tests"
```

---

### Task 5: Verify Coverage and Create PR

**Step 1: Run full test suite with coverage**

Run: `go test ./internal/... -coverprofile=/tmp/cov.out && go tool cover -func=/tmp/cov.out | tail -1`
Expected: Coverage > 55%

**Step 2: Check improved package coverage**

Run: `go test ./internal/reports/... ./internal/banking/... ./internal/accounting/... -cover`
Expected: reports > 50%, banking > 40%, accounting > 48%

**Step 3: Push changes**

```bash
git push origin feat/testcontainers-integration
```

**Step 4: Update PR description**

Update PR #22 description with new coverage numbers.

---

## Coverage Impact Summary

| Package | Before | Target | Method |
|---------|--------|--------|--------|
| reports | 42.3% | 55%+ | Balance confirmation tests |
| banking | 35.0% | 45%+ | Import/matcher tests |
| accounting | 44.5% | 50%+ | UpdateCostCenter tests |

**Total Target:** 52.1% → 60%+
