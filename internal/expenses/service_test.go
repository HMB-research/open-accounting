package expenses

import (
	"context"
	"errors"
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

func fixedExpenseNow() time.Time {
	return time.Date(2026, 5, 30, 12, 0, 0, 0, time.UTC)
}

type memoryRepository struct {
	expenses map[string]*Expense
	counter  int
}

func newMemoryRepository() *memoryRepository {
	return &memoryRepository{expenses: make(map[string]*Expense)}
}

func (r *memoryRepository) Create(_ context.Context, _ string, expense *Expense) error {
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
	if _, ok := r.expenses[expense.ID]; !ok {
		return ErrExpenseNotFound
	}
	copyExpense := *expense
	r.expenses[expense.ID] = &copyExpense
	return nil
}

func (r *memoryRepository) GenerateNumber(_ context.Context, _, _ string) (string, error) {
	r.counter++
	return "EXP-0000" + string(rune('0'+r.counter)), nil
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
}

func (f *fakeEvidenceEvaluator) EvaluateEvidencePolicy(_ context.Context, _, _ string, req *documents.EvidencePolicyRequest) ([]documents.EvidencePolicyResult, error) {
	return []documents.EvidencePolicyResult{{
		EntityType: req.EntityType,
		EntityID:   req.EntityIDs[0],
		Compliant:  f.compliant,
	}}, nil
}
