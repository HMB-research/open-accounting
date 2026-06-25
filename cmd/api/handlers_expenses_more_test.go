package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/HMB-research/open-accounting/internal/accounting"
	"github.com/HMB-research/open-accounting/internal/expenses"
	"github.com/HMB-research/open-accounting/internal/tenant"
)

type erroringExpenseRepository struct {
	expenseHandlerRepository
	createErr error
	getErr    error
	listErr   error
	updateErr error
}

func newErroringExpenseRepository() *erroringExpenseRepository {
	return &erroringExpenseRepository{
		expenseHandlerRepository: expenseHandlerRepository{expenses: make(map[string]*expenses.Expense)},
	}
}

func (r *erroringExpenseRepository) Create(ctx context.Context, schemaName string, expense *expenses.Expense) error {
	if r.createErr != nil {
		return r.createErr
	}
	return r.expenseHandlerRepository.Create(ctx, schemaName, expense)
}

func (r *erroringExpenseRepository) GetByID(ctx context.Context, schemaName, tenantID, expenseID string) (*expenses.Expense, error) {
	if r.getErr != nil {
		return nil, r.getErr
	}
	return r.expenseHandlerRepository.GetByID(ctx, schemaName, tenantID, expenseID)
}

func (r *erroringExpenseRepository) List(ctx context.Context, schemaName, tenantID string, filter expenses.ListExpensesFilter) ([]expenses.Expense, error) {
	if r.listErr != nil {
		return nil, r.listErr
	}
	return r.expenseHandlerRepository.List(ctx, schemaName, tenantID, filter)
}

func (r *erroringExpenseRepository) Update(ctx context.Context, schemaName string, expense *expenses.Expense) error {
	if r.updateErr != nil {
		return r.updateErr
	}
	return r.expenseHandlerRepository.Update(ctx, schemaName, expense)
}

func TestExpenseHandlersAdditionalValidationBranches(t *testing.T) {
	h, _, _ := setupExpenseHandlers()
	claims := createTestClaims("user-1", "user@example.com", "tenant-1", "admin")

	req := withURLParams(makeAuthenticatedRequest(http.MethodGet, "/tenants/tenant-1/expenses?limit=-1", nil, claims), map[string]string{"tenantID": "tenant-1"})
	w := httptest.NewRecorder()
	h.ListExpenses(w, req)
	require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "limit must be zero or greater")

	req = httptest.NewRequest(http.MethodPost, "/tenants/tenant-1/expenses", strings.NewReader("{"))
	req = req.WithContext(contextWithClaims(req.Context(), claims))
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
	w = httptest.NewRecorder()
	h.CreateExpense(w, req)
	require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "Invalid request body")

	req = httptest.NewRequest(http.MethodPost, "/tenants/tenant-1/expenses/import", strings.NewReader("{"))
	req = req.WithContext(contextWithClaims(req.Context(), claims))
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
	w = httptest.NewRecorder()
	h.ImportExpenses(w, req)
	require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "Invalid request body")

	req = withURLParams(makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/expenses/import", expenses.ImportExpensesRequest{}, claims), map[string]string{"tenantID": "tenant-1"})
	w = httptest.NewRecorder()
	h.ImportExpenses(w, req)
	require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "csv_content is required")

	req = httptest.NewRequest(http.MethodPost, "/tenants/tenant-1/expenses/expense-1/reject", strings.NewReader("{"))
	req = req.WithContext(contextWithClaims(req.Context(), claims))
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1", "expenseID": "expense-1"})
	w = httptest.NewRecorder()
	h.RejectExpense(w, req)
	require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "Invalid request body")
}

func TestExpenseHandlersAdditionalServiceErrorBranches(t *testing.T) {
	repo := newErroringExpenseRepository()
	h, _, evidence := setupExpenseHandlers()
	h.expensesService = expenses.NewServiceWithRepository(repo, &expenseHandlerAccounting{
		accounts: map[string]*accounting.Account{
			"expense-account": {ID: "expense-account", Code: "5500", AccountType: accounting.AccountTypeExpense},
			"cash-account":    {ID: "cash-account", Code: "1000", AccountType: accounting.AccountTypeAsset},
		},
	}, evidence)
	claims := createTestClaims("user-1", "user@example.com", "tenant-1", "admin")

	repo.listErr = errors.New("list failed")
	req := withURLParams(makeAuthenticatedRequest(http.MethodGet, "/tenants/tenant-1/expenses", nil, claims), map[string]string{"tenantID": "tenant-1"})
	w := httptest.NewRecorder()
	h.ListExpenses(w, req)
	require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "list failed")
	repo.listErr = nil

	repo.createErr = errors.New("create failed")
	req = withURLParams(makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/expenses", expenses.CreateExpenseRequest{
		ExpenseDate:      time.Date(2026, 5, 30, 0, 0, 0, 0, time.UTC),
		Merchant:         "Office Store",
		ExpenseAccountID: "expense-account",
		PaymentAccountID: "cash-account",
		Amount:           decimal.NewFromInt(10),
	}, claims), map[string]string{"tenantID": "tenant-1"})
	w = httptest.NewRecorder()
	h.CreateExpense(w, req)
	require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "create failed")
	repo.createErr = nil

	repo.getErr = expenses.ErrExpenseNotFound
	req = withURLParams(makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/expenses/missing/post", nil, claims), map[string]string{"tenantID": "tenant-1", "expenseID": "missing"})
	w = httptest.NewRecorder()
	h.PostExpense(w, req)
	require.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), expenses.ErrExpenseNotFound.Error())
	repo.getErr = nil

	repo.updateErr = expenses.ErrInvalidStatusTransition
	created := &expenses.Expense{
		ID:               "expense-1",
		TenantID:         "tenant-1",
		ExpenseDate:      time.Date(2026, 5, 30, 0, 0, 0, 0, time.UTC),
		Merchant:         "Office Store",
		ExpenseAccountID: "expense-account",
		PaymentAccountID: "cash-account",
		Amount:           decimal.NewFromInt(10),
		Status:           expenses.StatusDraft,
	}
	require.NoError(t, repo.expenseHandlerRepository.Create(context.Background(), "tenant_test", created))
	req = withURLParams(makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/expenses/expense-1/submit", nil, claims), map[string]string{"tenantID": "tenant-1", "expenseID": "expense-1"})
	w = httptest.NewRecorder()
	h.SubmitExpense(w, req)
	require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), expenses.ErrInvalidStatusTransition.Error())
}

func TestImportExpensesPeriodLockLoadFailure(t *testing.T) {
	h, _, _ := setupExpenseHandlers()
	tenantRepo := newMockTenantRepository()
	tenantRepo.tenants["tenant-1"] = &tenant.Tenant{ID: "tenant-1", SchemaName: "tenant_test"}
	tenantRepo.getTenantErr = errors.New("tenant load failed")
	h.tenantService = tenant.NewServiceWithRepository(tenantRepo)

	req := withURLParams(makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/expenses/import", expenses.ImportExpensesRequest{
		FileName:   "expenses.csv",
		CSVContent: "expense_number,expense_date,merchant,expense_account_id,payment_account_id,amount,status\nEXP-1,2026-05-30,Office,expense-account,cash-account,10,DRAFT\n",
	}, createTestClaims("user-1", "user@example.com", "tenant-1", "admin")), map[string]string{"tenantID": "tenant-1"})
	w := httptest.NewRecorder()

	h.ImportExpenses(w, req)

	require.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "Failed to validate period lock")
}

func TestExpenseHelperBoundaries(t *testing.T) {
	nowish := expenseOperationDate(time.Time{})
	assert.WithinDuration(t, time.Now().UTC(), nowish, 2*time.Second)

	explicit := time.Date(2026, 6, 25, 0, 0, 0, 0, time.UTC)
	assert.Equal(t, explicit, expenseOperationDate(explicit))

	for _, tt := range []struct {
		err  error
		code int
	}{
		{err: expenses.ErrApprovedReceiptRequired, code: http.StatusConflict},
		{err: expenses.ErrExpenseAlreadyPosted, code: http.StatusConflict},
		{err: expenses.ErrExpenseAccountingInvalid, code: http.StatusBadRequest},
		{err: errors.New("plain validation"), code: http.StatusBadRequest},
	} {
		w := httptest.NewRecorder()
		respondExpenseError(w, tt.err)
		assert.Equal(t, tt.code, w.Code)
		assert.Contains(t, w.Body.String(), tt.err.Error())
	}
}
