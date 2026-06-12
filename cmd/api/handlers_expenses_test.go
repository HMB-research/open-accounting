package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/HMB-research/open-accounting/internal/accounting"
	"github.com/HMB-research/open-accounting/internal/documents"
	"github.com/HMB-research/open-accounting/internal/expenses"
	"github.com/HMB-research/open-accounting/internal/tenant"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupExpenseHandlers() (*Handlers, *expenseHandlerRepository, *expenseHandlerEvidence) {
	expenseRepo := &expenseHandlerRepository{expenses: make(map[string]*expenses.Expense)}
	accountingSvc := &expenseHandlerAccounting{
		accounts: map[string]*accounting.Account{
			"expense-account": {ID: "expense-account", Code: "5500", AccountType: accounting.AccountTypeExpense},
			"cash-account":    {ID: "cash-account", Code: "1000", AccountType: accounting.AccountTypeAsset},
		},
	}
	evidence := &expenseHandlerEvidence{compliant: false}
	expenseService := expenses.NewServiceWithRepository(expenseRepo, accountingSvc, evidence)

	tenantRepo := newMockTenantRepository()
	tenantRepo.tenants["tenant-1"] = &tenant.Tenant{ID: "tenant-1", SchemaName: "tenant_test"}

	h := &Handlers{
		tenantService:   tenant.NewServiceWithRepository(tenantRepo),
		expensesService: expenseService,
	}
	return h, expenseRepo, evidence
}

func TestExpenseHandlersLifecycle(t *testing.T) {
	h, _, evidence := setupExpenseHandlers()
	claims := createTestClaims("user-1", "user@example.com", "tenant-1", "admin")

	createReq := expenses.CreateExpenseRequest{
		ExpenseDate:      time.Date(2026, 5, 30, 0, 0, 0, 0, time.UTC),
		Merchant:         "Office Store",
		Description:      "Printer toner",
		ExpenseAccountID: "expense-account",
		PaymentAccountID: "cash-account",
		Amount:           decimal.RequireFromString("120.50"),
	}
	req := withURLParams(makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/expenses", createReq, claims), map[string]string{"tenantID": "tenant-1"})
	w := httptest.NewRecorder()
	h.CreateExpense(w, req)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	var created expenses.Expense
	require.NoError(t, json.NewDecoder(w.Body).Decode(&created))
	assert.Equal(t, expenses.StatusDraft, created.Status)
	assert.True(t, created.RequiresReceipt)

	req = withURLParams(makeAuthenticatedRequest(http.MethodGet, "/tenants/tenant-1/expenses/"+created.ID, nil, claims), map[string]string{
		"tenantID":  "tenant-1",
		"expenseID": created.ID,
	})
	w = httptest.NewRecorder()
	h.GetExpense(w, req)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var fetched expenses.Expense
	require.NoError(t, json.NewDecoder(w.Body).Decode(&fetched))
	assert.Equal(t, created.ID, fetched.ID)
	assert.Equal(t, "Office Store", fetched.Merchant)

	req = withURLParams(makeAuthenticatedRequest(http.MethodGet, "/tenants/tenant-1/expenses/missing", nil, claims), map[string]string{
		"tenantID":  "tenant-1",
		"expenseID": "missing",
	})
	w = httptest.NewRecorder()
	h.GetExpense(w, req)
	require.Equal(t, http.StatusNotFound, w.Code, w.Body.String())

	req = withURLParams(makeAuthenticatedRequest(http.MethodGet, "/tenants/tenant-1/expenses?status=DRAFT", nil, claims), map[string]string{"tenantID": "tenant-1"})
	w = httptest.NewRecorder()
	h.ListExpenses(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	var listed []expenses.Expense
	require.NoError(t, json.NewDecoder(w.Body).Decode(&listed))
	require.Len(t, listed, 1)

	req = withURLParams(makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/expenses/"+created.ID+"/submit", nil, claims), map[string]string{
		"tenantID":  "tenant-1",
		"expenseID": created.ID,
	})
	w = httptest.NewRecorder()
	h.SubmitExpense(w, req)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	req = withURLParams(makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/expenses/"+created.ID+"/approve", nil, claims), map[string]string{
		"tenantID":  "tenant-1",
		"expenseID": created.ID,
	})
	w = httptest.NewRecorder()
	h.ApproveExpense(w, req)
	require.Equal(t, http.StatusConflict, w.Code, w.Body.String())

	evidence.compliant = true
	req = withURLParams(makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/expenses/"+created.ID+"/approve", nil, claims), map[string]string{
		"tenantID":  "tenant-1",
		"expenseID": created.ID,
	})
	w = httptest.NewRecorder()
	h.ApproveExpense(w, req)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var approved expenses.Expense
	require.NoError(t, json.NewDecoder(w.Body).Decode(&approved))
	assert.Equal(t, expenses.StatusApproved, approved.Status)

	req = withURLParams(makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/expenses/"+created.ID+"/post", nil, claims), map[string]string{
		"tenantID":  "tenant-1",
		"expenseID": created.ID,
	})
	w = httptest.NewRecorder()
	h.PostExpense(w, req)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var posted expenses.Expense
	require.NoError(t, json.NewDecoder(w.Body).Decode(&posted))
	assert.Equal(t, expenses.StatusPosted, posted.Status)
	require.NotNil(t, posted.JournalEntryID)
}

func TestExpenseHandlersReject(t *testing.T) {
	h, _, _ := setupExpenseHandlers()
	claims := createTestClaims("user-1", "user@example.com", "tenant-1", "admin")

	createReq := expenses.CreateExpenseRequest{
		ExpenseDate:      time.Date(2026, 5, 30, 0, 0, 0, 0, time.UTC),
		Merchant:         "Taxi",
		ExpenseAccountID: "expense-account",
		PaymentAccountID: "cash-account",
		Amount:           decimal.NewFromInt(25),
		RequiresReceipt:  boolPtr(false),
	}
	req := withURLParams(makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/expenses", createReq, claims), map[string]string{"tenantID": "tenant-1"})
	w := httptest.NewRecorder()
	h.CreateExpense(w, req)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	var created expenses.Expense
	require.NoError(t, json.NewDecoder(w.Body).Decode(&created))

	req = withURLParams(makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/expenses/"+created.ID+"/submit", nil, claims), map[string]string{
		"tenantID":  "tenant-1",
		"expenseID": created.ID,
	})
	w = httptest.NewRecorder()
	h.SubmitExpense(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	req = withURLParams(makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/expenses/"+created.ID+"/reject", expenses.RejectExpenseRequest{Reason: "Need project code"}, claims), map[string]string{
		"tenantID":  "tenant-1",
		"expenseID": created.ID,
	})
	w = httptest.NewRecorder()
	h.RejectExpense(w, req)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var rejected expenses.Expense
	require.NoError(t, json.NewDecoder(w.Body).Decode(&rejected))
	assert.Equal(t, expenses.StatusRejected, rejected.Status)
	assert.Equal(t, "Need project code", rejected.RejectionReason)
}

func TestExpenseHandlersImport(t *testing.T) {
	h, repo, _ := setupExpenseHandlers()
	claims := createTestClaims("user-1", "user@example.com", "tenant-1", "admin")
	req := withURLParams(makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/expenses/import", expenses.ImportExpensesRequest{
		FileName: "expenses.csv",
		CSVContent: "expense_number,expense_date,merchant,expense_account_id,payment_account_id,amount,status\n" +
			"EXP-IMP-1,2026-05-30,Office Store,99999999-9999-4999-8999-999999999999,aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa,120.50,DRAFT\n",
	}, claims), map[string]string{"tenantID": "tenant-1"})

	w := httptest.NewRecorder()
	h.ImportExpenses(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var result expenses.ImportExpensesResult
	require.NoError(t, json.NewDecoder(w.Body).Decode(&result))
	assert.Equal(t, 1, result.RowsProcessed)
	assert.Equal(t, 1, result.ExpensesCreated)
	require.Len(t, repo.expenses, 1)
}

type expenseHandlerRepository struct {
	expenses map[string]*expenses.Expense
	counter  int
}

func (r *expenseHandlerRepository) Create(_ context.Context, _ string, expense *expenses.Expense) error {
	copyExpense := *expense
	r.expenses[expense.ID] = &copyExpense
	return nil
}

func (r *expenseHandlerRepository) GetByID(_ context.Context, _, tenantID, expenseID string) (*expenses.Expense, error) {
	expense, ok := r.expenses[expenseID]
	if !ok || expense.TenantID != tenantID {
		return nil, expenses.ErrExpenseNotFound
	}
	copyExpense := *expense
	return &copyExpense, nil
}

func (r *expenseHandlerRepository) List(_ context.Context, _, tenantID string, filter expenses.ListExpensesFilter) ([]expenses.Expense, error) {
	var result []expenses.Expense
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

func (r *expenseHandlerRepository) Update(_ context.Context, _ string, expense *expenses.Expense) error {
	if _, ok := r.expenses[expense.ID]; !ok {
		return expenses.ErrExpenseNotFound
	}
	copyExpense := *expense
	r.expenses[expense.ID] = &copyExpense
	return nil
}

func (r *expenseHandlerRepository) GenerateNumber(_ context.Context, _, _ string) (string, error) {
	r.counter++
	return "EXP-00001", nil
}

type expenseHandlerAccounting struct {
	accounts map[string]*accounting.Account
}

func (a *expenseHandlerAccounting) GetAccount(_ context.Context, _, _, accountID string) (*accounting.Account, error) {
	account, ok := a.accounts[accountID]
	if !ok {
		return nil, errors.New("account not found")
	}
	return account, nil
}

func (a *expenseHandlerAccounting) ListAccounts(_ context.Context, _, _ string, _ bool) ([]accounting.Account, error) {
	accounts := make([]accounting.Account, 0, len(a.accounts))
	for _, account := range a.accounts {
		accounts = append(accounts, *account)
	}
	return accounts, nil
}

func (a *expenseHandlerAccounting) CreateJournalEntry(_ context.Context, _, tenantID string, req *accounting.CreateJournalEntryRequest) (*accounting.JournalEntry, error) {
	return &accounting.JournalEntry{ID: "je-expense", TenantID: tenantID, Status: accounting.StatusDraft, Lines: []accounting.JournalEntryLine{
		{AccountID: req.Lines[0].AccountID, DebitAmount: req.Lines[0].DebitAmount},
		{AccountID: req.Lines[1].AccountID, CreditAmount: req.Lines[1].CreditAmount},
	}}, nil
}

func (a *expenseHandlerAccounting) PostJournalEntry(_ context.Context, _, _, _, _ string) error {
	return nil
}

type expenseHandlerEvidence struct {
	compliant bool
}

func (e *expenseHandlerEvidence) EvaluateEvidencePolicy(_ context.Context, _, _ string, req *documents.EvidencePolicyRequest) ([]documents.EvidencePolicyResult, error) {
	return []documents.EvidencePolicyResult{{EntityType: req.EntityType, EntityID: req.EntityIDs[0], Compliant: e.compliant}}, nil
}

func boolPtr(value bool) *bool {
	return &value
}
