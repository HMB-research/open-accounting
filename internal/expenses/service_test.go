package expenses

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/HMB-research/open-accounting/internal/accounting"
	"github.com/HMB-research/open-accounting/internal/documents"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServiceExpenseLifecycleRequiresReceiptBeforeApproval(t *testing.T) {
	repo := newMemoryRepository()
	accountingSvc := newFakeAccountingPoster()
	evidence := &fakeEvidenceEvaluator{compliant: false}
	service := NewServiceWithRepository(repo, accountingSvc, evidence)
	service.now = fixedExpenseNow

	expense := createTestExpense(t, service)
	submitted, err := service.SubmitExpense(context.Background(), "tenant_acme", "tenant-1", expense.ID, &ExpenseActionRequest{UserID: "user-1"})
	require.NoError(t, err)
	assert.Equal(t, StatusSubmitted, submitted.Status)

	_, err = service.ApproveExpense(context.Background(), "tenant_acme", "tenant-1", expense.ID, &ExpenseActionRequest{UserID: "user-2"})
	require.ErrorIs(t, err, ErrApprovedReceiptRequired)
	assert.Equal(t, StatusSubmitted, repo.expenses[expense.ID].Status)

	evidence.compliant = true
	approved, err := service.ApproveExpense(context.Background(), "tenant_acme", "tenant-1", expense.ID, &ExpenseActionRequest{UserID: "user-2"})
	require.NoError(t, err)
	assert.Equal(t, StatusApproved, approved.Status)
	assert.NotNil(t, approved.ApprovedAt)
	assert.Equal(t, "user-2", *approved.ApprovedBy)
}

func TestNewService(t *testing.T) {
	service := NewService(nil, nil)

	assert.NotNil(t, service)
	assert.NotNil(t, service.repo)
	assert.NotNil(t, service.accounting)
}

func TestServiceCreateExpenseDefaultsAndValidation(t *testing.T) {
	t.Run("defaults and normalizes fields", func(t *testing.T) {
		repo := newMemoryRepository()
		service := NewServiceWithRepository(repo, newFakeAccountingPoster(), nil)
		service.now = fixedExpenseNow
		employeeID := " employee-1 "
		emptyContactID := " "
		requiresReceipt := false

		expense, err := service.CreateExpense(context.Background(), "tenant_acme", "tenant-1", &CreateExpenseRequest{
			Merchant:         "  Travel Shop  ",
			Description:      "  Train ticket  ",
			EmployeeID:       &employeeID,
			ContactID:        &emptyContactID,
			ExpenseAccountID: " expense-account ",
			PaymentAccountID: " cash-account ",
			Amount:           decimal.RequireFromString("10.25"),
			Currency:         " usd ",
			ExchangeRate:     decimal.RequireFromString("1.2"),
			RequiresReceipt:  &requiresReceipt,
			UserID:           " user-1 ",
		})

		require.NoError(t, err)
		assert.Equal(t, "EXP-00001", expense.ExpenseNumber)
		assert.Equal(t, "Travel Shop", expense.Merchant)
		assert.Equal(t, "Train ticket", expense.Description)
		require.NotNil(t, expense.EmployeeID)
		assert.Equal(t, "employee-1", *expense.EmployeeID)
		assert.Nil(t, expense.ContactID)
		assert.Equal(t, "USD", expense.Currency)
		assert.True(t, expense.ExchangeRate.Equal(decimal.RequireFromString("1.2")))
		assert.True(t, expense.BaseAmount.Equal(decimal.RequireFromString("12.30")))
		assert.False(t, expense.RequiresReceipt)
		assert.Equal(t, StatusDraft, expense.Status)
		assert.Equal(t, fixedExpenseNow().Format("2006-01-02"), expense.ExpenseDate.Format("2006-01-02"))
		assert.Equal(t, "user-1", expense.CreatedBy)
	})

	t.Run("validation errors", func(t *testing.T) {
		tests := []struct {
			name string
			req  *CreateExpenseRequest
			want string
		}{
			{name: "nil request", req: nil, want: "expense request is required"},
			{name: "missing user", req: validCreateExpenseRequest(func(req *CreateExpenseRequest) { req.UserID = " " }), want: "user id is required"},
			{name: "missing merchant", req: validCreateExpenseRequest(func(req *CreateExpenseRequest) { req.Merchant = " " }), want: "merchant is required"},
			{name: "missing expense account", req: validCreateExpenseRequest(func(req *CreateExpenseRequest) { req.ExpenseAccountID = " " }), want: "expense_account_id is required"},
			{name: "missing payment account", req: validCreateExpenseRequest(func(req *CreateExpenseRequest) { req.PaymentAccountID = " " }), want: "payment_account_id is required"},
			{name: "nonpositive amount", req: validCreateExpenseRequest(func(req *CreateExpenseRequest) { req.Amount = decimal.Zero }), want: "amount must be positive"},
			{name: "invalid currency", req: validCreateExpenseRequest(func(req *CreateExpenseRequest) { req.Currency = "EURO" }), want: "currency must be a 3-letter ISO code"},
			{name: "invalid exchange rate", req: validCreateExpenseRequest(func(req *CreateExpenseRequest) { req.ExchangeRate = decimal.NewFromInt(-1) }), want: "exchange_rate must be positive"},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				service := NewServiceWithRepository(newMemoryRepository(), newFakeAccountingPoster(), nil)

				_, err := service.CreateExpense(context.Background(), "tenant_acme", "tenant-1", tt.req)

				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.want)
			})
		}
	})
}

func TestServiceListExpensesNormalizesStatus(t *testing.T) {
	repo := newMemoryRepository()
	service := NewServiceWithRepository(repo, newFakeAccountingPoster(), nil)
	service.now = fixedExpenseNow
	expense := createTestExpense(t, service)
	_, err := service.SubmitExpense(context.Background(), "tenant_acme", "tenant-1", expense.ID, &ExpenseActionRequest{UserID: "user-1"})
	require.NoError(t, err)

	listed, err := service.ListExpenses(context.Background(), "tenant_acme", "tenant-1", ListExpensesFilter{Status: " submitted ", Limit: 10})

	require.NoError(t, err)
	require.Len(t, listed, 1)
	assert.Equal(t, StatusSubmitted, listed[0].Status)
	assert.Equal(t, StatusSubmitted, repo.lastListFilter.Status)
	assert.Equal(t, 10, repo.lastListFilter.Limit)

	_, err = service.ListExpenses(context.Background(), "tenant_acme", "tenant-1", ListExpensesFilter{Status: "archived"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported expense status")

	repo.listErr = errors.New("list failed")
	_, err = service.ListExpenses(context.Background(), "tenant_acme", "tenant-1", ListExpensesFilter{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "list failed")
}

func TestServiceGetExpenseValidatesID(t *testing.T) {
	service := NewServiceWithRepository(newMemoryRepository(), newFakeAccountingPoster(), nil)

	_, err := service.GetExpense(context.Background(), "tenant_acme", "tenant-1", " ")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "expense id is required")
}

func TestServicePostExpenseCreatesAndPostsBalancedJournalEntry(t *testing.T) {
	repo := newMemoryRepository()
	accountingSvc := newFakeAccountingPoster()
	evidence := &fakeEvidenceEvaluator{compliant: true}
	service := NewServiceWithRepository(repo, accountingSvc, evidence)
	service.now = fixedExpenseNow

	expense := createTestExpense(t, service)
	_, err := service.SubmitExpense(context.Background(), "tenant_acme", "tenant-1", expense.ID, &ExpenseActionRequest{UserID: "user-1"})
	require.NoError(t, err)
	_, err = service.ApproveExpense(context.Background(), "tenant_acme", "tenant-1", expense.ID, &ExpenseActionRequest{UserID: "user-2"})
	require.NoError(t, err)

	posted, err := service.PostExpense(context.Background(), "tenant_acme", "tenant-1", expense.ID, &ExpenseActionRequest{UserID: "user-3"})
	require.NoError(t, err)

	assert.Equal(t, StatusPosted, posted.Status)
	require.NotNil(t, posted.JournalEntryID)
	assert.Equal(t, "je-1", *posted.JournalEntryID)
	assert.Equal(t, []string{"je-1"}, accountingSvc.postedIDs)
	require.NotNil(t, accountingSvc.createdRequest)
	assert.Equal(t, SourceTypeExpense, accountingSvc.createdRequest.SourceType)
	require.Len(t, accountingSvc.createdRequest.Lines, 2)
	assert.True(t, accountingSvc.createdRequest.Lines[0].DebitAmount.Equal(decimal.RequireFromString("120.50")))
	assert.True(t, accountingSvc.createdRequest.Lines[1].CreditAmount.Equal(decimal.RequireFromString("120.50")))
}

func TestServicePostExpenseErrors(t *testing.T) {
	t.Run("already posted", func(t *testing.T) {
		repo := newMemoryRepository()
		service := NewServiceWithRepository(repo, newFakeAccountingPoster(), &fakeEvidenceEvaluator{compliant: true})
		service.now = fixedExpenseNow
		expense := createTestExpense(t, service)
		expense.Status = StatusPosted
		require.NoError(t, repo.Update(context.Background(), "tenant_acme", expense))

		_, err := service.PostExpense(context.Background(), "tenant_acme", "tenant-1", expense.ID, &ExpenseActionRequest{UserID: "user-1"})

		require.ErrorIs(t, err, ErrExpenseAlreadyPosted)
	})

	t.Run("requires approved status", func(t *testing.T) {
		service := NewServiceWithRepository(newMemoryRepository(), newFakeAccountingPoster(), &fakeEvidenceEvaluator{compliant: true})
		service.now = fixedExpenseNow
		expense := createTestExpense(t, service)

		_, err := service.PostExpense(context.Background(), "tenant_acme", "tenant-1", expense.ID, &ExpenseActionRequest{UserID: "user-1"})

		require.ErrorIs(t, err, ErrInvalidStatusTransition)
	})

	t.Run("journal creation failure", func(t *testing.T) {
		repo := newMemoryRepository()
		accountingSvc := newFakeAccountingPoster()
		accountingSvc.createErr = errors.New("create journal failed")
		service := NewServiceWithRepository(repo, accountingSvc, &fakeEvidenceEvaluator{compliant: true})
		service.now = fixedExpenseNow
		expense := approvedTestExpense(t, service)

		_, err := service.PostExpense(context.Background(), "tenant_acme", "tenant-1", expense.ID, &ExpenseActionRequest{UserID: "user-3"})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "create journal failed")
	})

	t.Run("journal post failure", func(t *testing.T) {
		repo := newMemoryRepository()
		accountingSvc := newFakeAccountingPoster()
		accountingSvc.postErr = errors.New("post journal failed")
		service := NewServiceWithRepository(repo, accountingSvc, &fakeEvidenceEvaluator{compliant: true})
		service.now = fixedExpenseNow
		expense := approvedTestExpense(t, service)

		_, err := service.PostExpense(context.Background(), "tenant_acme", "tenant-1", expense.ID, &ExpenseActionRequest{UserID: "user-3"})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "post journal failed")
	})
}

func TestServiceRejectExpense(t *testing.T) {
	service := NewServiceWithRepository(newMemoryRepository(), newFakeAccountingPoster(), &fakeEvidenceEvaluator{compliant: true})
	service.now = fixedExpenseNow
	expense := createTestExpense(t, service)
	_, err := service.SubmitExpense(context.Background(), "tenant_acme", "tenant-1", expense.ID, &ExpenseActionRequest{UserID: "user-1"})
	require.NoError(t, err)

	rejected, err := service.RejectExpense(context.Background(), "tenant_acme", "tenant-1", expense.ID, &RejectExpenseRequest{
		Reason: "Missing tax details",
		UserID: "user-2",
	})
	require.NoError(t, err)

	assert.Equal(t, StatusRejected, rejected.Status)
	assert.Equal(t, "Missing tax details", rejected.RejectionReason)
	assert.NotNil(t, rejected.RejectedAt)
}

func TestServiceRejectExpenseValidation(t *testing.T) {
	service := NewServiceWithRepository(newMemoryRepository(), newFakeAccountingPoster(), &fakeEvidenceEvaluator{compliant: true})
	service.now = fixedExpenseNow
	expense := createTestExpense(t, service)

	_, err := service.RejectExpense(context.Background(), "tenant_acme", "tenant-1", expense.ID, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "reject request is required")

	_, err = service.RejectExpense(context.Background(), "tenant_acme", "tenant-1", expense.ID, &RejectExpenseRequest{UserID: " "})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "user id is required")

	_, err = service.RejectExpense(context.Background(), "tenant_acme", "tenant-1", expense.ID, &RejectExpenseRequest{UserID: "user-1", Reason: " "})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "reason is required")

	_, err = service.RejectExpense(context.Background(), "tenant_acme", "tenant-1", expense.ID, &RejectExpenseRequest{UserID: "user-1", Reason: "not submitted"})
	require.ErrorIs(t, err, ErrInvalidStatusTransition)
}

func TestServicePostExpenseValidatesAccountTypes(t *testing.T) {
	repo := newMemoryRepository()
	accountingSvc := newFakeAccountingPoster()
	accountingSvc.accounts["expense-account"] = &accounting.Account{ID: "expense-account", AccountType: accounting.AccountTypeAsset}
	service := NewServiceWithRepository(repo, accountingSvc, &fakeEvidenceEvaluator{compliant: true})
	service.now = fixedExpenseNow
	expense := createTestExpense(t, service)
	_, err := service.SubmitExpense(context.Background(), "tenant_acme", "tenant-1", expense.ID, &ExpenseActionRequest{UserID: "user-1"})
	require.NoError(t, err)
	_, err = service.ApproveExpense(context.Background(), "tenant_acme", "tenant-1", expense.ID, &ExpenseActionRequest{UserID: "user-2"})
	require.NoError(t, err)

	_, err = service.PostExpense(context.Background(), "tenant_acme", "tenant-1", expense.ID, &ExpenseActionRequest{UserID: "user-3"})
	require.ErrorIs(t, err, ErrExpenseAccountingInvalid)
}

func TestServiceReceiptPolicyErrors(t *testing.T) {
	t.Run("nil evidence service", func(t *testing.T) {
		service := NewServiceWithRepository(newMemoryRepository(), newFakeAccountingPoster(), nil)
		service.now = fixedExpenseNow
		expense := createTestExpense(t, service)
		_, err := service.SubmitExpense(context.Background(), "tenant_acme", "tenant-1", expense.ID, &ExpenseActionRequest{UserID: "user-1"})
		require.NoError(t, err)

		_, err = service.ApproveExpense(context.Background(), "tenant_acme", "tenant-1", expense.ID, &ExpenseActionRequest{UserID: "user-2"})

		require.ErrorIs(t, err, ErrApprovedReceiptRequired)
		assert.Contains(t, err.Error(), "document evidence service is unavailable")
	})

	t.Run("evidence evaluator error", func(t *testing.T) {
		service := NewServiceWithRepository(newMemoryRepository(), newFakeAccountingPoster(), &fakeEvidenceEvaluator{err: errors.New("policy failed")})
		service.now = fixedExpenseNow
		expense := createTestExpense(t, service)
		_, err := service.SubmitExpense(context.Background(), "tenant_acme", "tenant-1", expense.ID, &ExpenseActionRequest{UserID: "user-1"})
		require.NoError(t, err)

		_, err = service.ApproveExpense(context.Background(), "tenant_acme", "tenant-1", expense.ID, &ExpenseActionRequest{UserID: "user-2"})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "evaluate expense receipt evidence")
	})
}

func TestServicePostExpenseValidatesAccountingAvailability(t *testing.T) {
	service := NewServiceWithRepository(newMemoryRepository(), nil, &fakeEvidenceEvaluator{compliant: true})
	service.now = fixedExpenseNow
	expense := approvedTestExpense(t, service)

	_, err := service.PostExpense(context.Background(), "tenant_acme", "tenant-1", expense.ID, &ExpenseActionRequest{UserID: "user-3"})

	require.ErrorIs(t, err, ErrExpenseAccountingInvalid)
	assert.Contains(t, err.Error(), "accounting service is unavailable")
}

func createTestExpense(t *testing.T, service *Service) *Expense {
	t.Helper()
	expenseDate := time.Date(2026, 5, 30, 0, 0, 0, 0, time.UTC)
	expense, err := service.CreateExpense(context.Background(), "tenant_acme", "tenant-1", &CreateExpenseRequest{
		ExpenseDate:      expenseDate,
		Merchant:         "Office Store",
		Description:      "Printer toner",
		ExpenseAccountID: "expense-account",
		PaymentAccountID: "cash-account",
		Amount:           decimal.RequireFromString("120.50"),
		UserID:           "user-1",
	})
	require.NoError(t, err)
	return expense
}

func approvedTestExpense(t *testing.T, service *Service) *Expense {
	t.Helper()
	expense := createTestExpense(t, service)
	_, err := service.SubmitExpense(context.Background(), "tenant_acme", "tenant-1", expense.ID, &ExpenseActionRequest{UserID: "user-1"})
	require.NoError(t, err)
	approved, err := service.ApproveExpense(context.Background(), "tenant_acme", "tenant-1", expense.ID, &ExpenseActionRequest{UserID: "user-2"})
	require.NoError(t, err)
	return approved
}

func validCreateExpenseRequest(mutators ...func(*CreateExpenseRequest)) *CreateExpenseRequest {
	req := &CreateExpenseRequest{
		ExpenseDate:      time.Date(2026, 5, 30, 0, 0, 0, 0, time.UTC),
		Merchant:         "Office Store",
		ExpenseAccountID: "expense-account",
		PaymentAccountID: "cash-account",
		Amount:           decimal.NewFromInt(10),
		UserID:           "user-1",
	}
	for _, mutate := range mutators {
		mutate(req)
	}
	return req
}

func fixedExpenseNow() time.Time {
	return time.Date(2026, 5, 30, 12, 0, 0, 0, time.UTC)
}

type memoryRepository struct {
	expenses       map[string]*Expense
	counter        int
	listErr        error
	generateErr    error
	createErr      error
	updateErr      error
	lastListFilter ListExpensesFilter
}

func newMemoryRepository() *memoryRepository {
	return &memoryRepository{expenses: make(map[string]*Expense)}
}

func (r *memoryRepository) Create(_ context.Context, _ string, expense *Expense) error {
	if r.createErr != nil {
		return r.createErr
	}
	copyExpense := *expense
	r.expenses[expense.ID] = &copyExpense
	return nil
}

func (r *memoryRepository) GetByID(_ context.Context, _, tenantID, expenseID string) (*Expense, error) {
	expense, ok := r.expenses[expenseID]
	if !ok || expense.TenantID != tenantID {
		return nil, ErrExpenseNotFound
	}
	copyExpense := *expense
	return &copyExpense, nil
}

func (r *memoryRepository) List(_ context.Context, _, tenantID string, filter ListExpensesFilter) ([]Expense, error) {
	if r.listErr != nil {
		return nil, r.listErr
	}
	r.lastListFilter = filter
	var result []Expense
	for _, expense := range r.expenses {
		if expense.TenantID != tenantID {
			continue
		}
		if filter.Status != "" && expense.Status != filter.Status {
			continue
		}
		result = append(result, *expense)
	}
	return result, nil
}

func (r *memoryRepository) Update(_ context.Context, _ string, expense *Expense) error {
	if r.updateErr != nil {
		return r.updateErr
	}
	if _, ok := r.expenses[expense.ID]; !ok {
		return ErrExpenseNotFound
	}
	copyExpense := *expense
	r.expenses[expense.ID] = &copyExpense
	return nil
}

func (r *memoryRepository) GenerateNumber(_ context.Context, _, _ string) (string, error) {
	if r.generateErr != nil {
		return "", r.generateErr
	}
	r.counter++
	return fmt.Sprintf("EXP-%05d", r.counter), nil
}

type fakeAccountingPoster struct {
	accounts       map[string]*accounting.Account
	createdRequest *accounting.CreateJournalEntryRequest
	postedIDs      []string
	createErr      error
	postErr        error
}

func newFakeAccountingPoster() *fakeAccountingPoster {
	return &fakeAccountingPoster{
		accounts: map[string]*accounting.Account{
			"expense-account": {ID: "expense-account", Code: "5500", AccountType: accounting.AccountTypeExpense},
			"cash-account":    {ID: "cash-account", Code: "1000", AccountType: accounting.AccountTypeAsset},
		},
	}
}

func (f *fakeAccountingPoster) GetAccount(_ context.Context, _, _, accountID string) (*accounting.Account, error) {
	account, ok := f.accounts[accountID]
	if !ok {
		return nil, errors.New("account not found")
	}
	return account, nil
}

func (f *fakeAccountingPoster) ListAccounts(_ context.Context, _, _ string, _ bool) ([]accounting.Account, error) {
	accounts := make([]accounting.Account, 0, len(f.accounts))
	for _, account := range f.accounts {
		accounts = append(accounts, *account)
	}
	return accounts, nil
}

func (f *fakeAccountingPoster) CreateJournalEntry(_ context.Context, _, tenantID string, req *accounting.CreateJournalEntryRequest) (*accounting.JournalEntry, error) {
	if f.createErr != nil {
		return nil, f.createErr
	}
	f.createdRequest = req
	return &accounting.JournalEntry{ID: "je-1", TenantID: tenantID, Status: accounting.StatusDraft}, nil
}

func (f *fakeAccountingPoster) PostJournalEntry(_ context.Context, _, _, entryID, _ string) error {
	if f.postErr != nil {
		return f.postErr
	}
	f.postedIDs = append(f.postedIDs, entryID)
	return nil
}

type fakeEvidenceEvaluator struct {
	compliant bool
	err       error
}

func (f *fakeEvidenceEvaluator) EvaluateEvidencePolicy(_ context.Context, _, _ string, req *documents.EvidencePolicyRequest) ([]documents.EvidencePolicyResult, error) {
	if f.err != nil {
		return nil, f.err
	}
	return []documents.EvidencePolicyResult{{
		EntityType: req.EntityType,
		EntityID:   req.EntityIDs[0],
		Compliant:  f.compliant,
	}}, nil
}
