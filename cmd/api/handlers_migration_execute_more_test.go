package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/HMB-research/open-accounting/internal/accounting"
	"github.com/HMB-research/open-accounting/internal/auth"
	"github.com/HMB-research/open-accounting/internal/banking"
	"github.com/HMB-research/open-accounting/internal/contacts"
	"github.com/HMB-research/open-accounting/internal/cutover"
	"github.com/HMB-research/open-accounting/internal/inventory"
	"github.com/HMB-research/open-accounting/internal/invoicing"
	"github.com/HMB-research/open-accounting/internal/orders"
	"github.com/HMB-research/open-accounting/internal/quotes"
	"github.com/HMB-research/open-accounting/internal/recurring"
)

type nonFlushingResponseRecorder struct {
	header http.Header
	code   int
	body   strings.Builder
}

func (r *nonFlushingResponseRecorder) Header() http.Header {
	if r.header == nil {
		r.header = make(http.Header)
	}
	return r.header
}

func (r *nonFlushingResponseRecorder) Write(payload []byte) (int, error) {
	return r.body.Write(payload)
}

func (r *nonFlushingResponseRecorder) WriteHeader(statusCode int) {
	r.code = statusCode
}

type failAfterMigrationRunStore struct {
	failAfter int
	saves     int
	err       error
}

func (s *failAfterMigrationRunStore) SaveExecutionRun(_ context.Context, _, tenantID, createdBy string, run *cutover.MigrationExecutionRun) (*cutover.MigrationExecutionRun, error) {
	s.saves++
	if s.err != nil && s.saves > s.failAfter {
		return nil, s.err
	}
	if run.ID == "" {
		run.ID = "run-sequenced"
	}
	run.TenantID = tenantID
	run.CreatedBy = createdBy
	return run, nil
}

func (s *failAfterMigrationRunStore) ListExecutionRuns(context.Context, string, string, cutover.MigrationExecutionRunFilter) ([]cutover.MigrationExecutionRun, error) {
	return nil, nil
}

func (s *failAfterMigrationRunStore) GetExecutionRun(context.Context, string, string, string) (*cutover.MigrationExecutionRun, error) {
	return nil, nil
}

type failingSSEWriter struct {
	header http.Header
}

func (w *failingSSEWriter) Header() http.Header {
	if w.header == nil {
		w.header = make(http.Header)
	}
	return w.header
}

func (w *failingSSEWriter) Write([]byte) (int, error) {
	return 0, errors.New("write failed")
}

func (w *failingSSEWriter) WriteHeader(int) {}

func (w *failingSSEWriter) Flush() {}

func TestExecuteMigrationHandlerAdditionalValidationBranches(t *testing.T) {
	t.Run("rejects unauthenticated request", func(t *testing.T) {
		h := &Handlers{migrationExecutor: &fakeMigrationStepExecutor{}}
		req := withURLParams(makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/migration/execute", cutover.ExecuteMigrationRequest{
			Files: []cutover.BundleFile{{Kind: cutover.KindAccounts, FileName: "accounts.csv", CSVContent: "code,name,account_type\n1000,Cash,ASSET\n"}},
		}, nil), map[string]string{"tenantID": "tenant-1"})
		w := httptest.NewRecorder()

		h.ExecuteMigration(w, req)

		require.Equal(t, http.StatusUnauthorized, w.Code, w.Body.String())
		assert.Contains(t, w.Body.String(), "Not authenticated")
	})

	t.Run("rejects missing tenant id", func(t *testing.T) {
		h := &Handlers{migrationExecutor: &fakeMigrationStepExecutor{}}
		req := makeAuthenticatedRequest(http.MethodPost, "/tenants//migration/execute", cutover.ExecuteMigrationRequest{
			Files: []cutover.BundleFile{{Kind: cutover.KindAccounts, FileName: "accounts.csv", CSVContent: "code,name,account_type\n1000,Cash,ASSET\n"}},
		}, createTestClaims("user-1", "user@example.com", "tenant-1", "admin"))
		w := httptest.NewRecorder()

		h.ExecuteMigration(w, req)

		require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
		assert.Contains(t, w.Body.String(), "tenantID is required")
	})

	t.Run("rejects invalid json", func(t *testing.T) {
		h := &Handlers{migrationExecutor: &fakeMigrationStepExecutor{}}
		req := httptest.NewRequest(http.MethodPost, "/tenants/tenant-1/migration/execute", strings.NewReader("{"))
		req = req.WithContext(contextWithClaims(req.Context(), createTestClaims("user-1", "user@example.com", "tenant-1", "admin")))
		req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
		w := httptest.NewRecorder()

		h.ExecuteMigration(w, req)

		require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
		assert.Contains(t, w.Body.String(), "Invalid request body")
	})

	t.Run("maps missing saved resume run", func(t *testing.T) {
		h := &Handlers{
			migrationExecutor: &fakeMigrationStepExecutor{},
			migrationRunStore: &fakeMigrationRunStore{getErr: cutover.ErrMigrationExecutionRunNotFound},
		}
		req := executeMigrationRequest(cutover.ExecuteMigrationRequest{ResumeFromRunID: "missing-run"})
		w := httptest.NewRecorder()

		h.ExecuteMigration(w, req)

		require.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
		assert.Contains(t, w.Body.String(), "migration execution run not found")
	})

	t.Run("maps saved resume run load failure", func(t *testing.T) {
		h := &Handlers{
			migrationExecutor: &fakeMigrationStepExecutor{},
			migrationRunStore: &fakeMigrationRunStore{getErr: errors.New("store offline")},
		}
		req := executeMigrationRequest(cutover.ExecuteMigrationRequest{ResumeFromRunID: "run-1"})
		w := httptest.NewRecorder()

		h.ExecuteMigration(w, req)

		require.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())
		assert.Contains(t, w.Body.String(), "Failed to load migration execution resume run")
	})
}

func TestExecuteMigrationHandlerSaveErrorBranches(t *testing.T) {
	t.Run("plan save failure", func(t *testing.T) {
		h := &Handlers{
			migrationExecutor: &fakeMigrationStepExecutor{},
			migrationRunStore: &fakeMigrationRunStore{saveErr: errors.New("save failed")},
		}
		req := executeMigrationRequest(cutover.ExecuteMigrationRequest{
			Files: []cutover.BundleFile{{Kind: cutover.KindAccounts, FileName: "accounts.csv", CSVContent: "code,name,account_type\n1000,Cash,ASSET\n"}},
		})
		w := httptest.NewRecorder()

		h.ExecuteMigration(w, req)

		require.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())
		assert.Contains(t, w.Body.String(), "Failed to save migration execution run")
	})

	t.Run("blocked plan save failure", func(t *testing.T) {
		h := &Handlers{
			migrationExecutor: &fakeMigrationStepExecutor{},
			migrationRunStore: &fakeMigrationRunStore{saveErr: errors.New("save failed")},
		}
		req := executeMigrationRequest(cutover.ExecuteMigrationRequest{
			Confirm: true,
			Files: []cutover.BundleFile{{
				Kind:       cutover.KindBankTransactions,
				FileName:   "bank.csv",
				CSVContent: "date,amount,description\n2026-01-02,42.50,Customer receipt\n",
			}},
		})
		w := httptest.NewRecorder()

		h.ExecuteMigration(w, req)

		require.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())
		assert.Contains(t, w.Body.String(), "Failed to save migration execution run")
	})

	t.Run("running snapshot save failure", func(t *testing.T) {
		store := &failAfterMigrationRunStore{failAfter: 1, err: errors.New("save failed")}
		h := &Handlers{
			migrationExecutor: &fakeMigrationStepExecutor{},
			migrationRunStore: store,
		}
		req := executeMigrationRequest(cutover.ExecuteMigrationRequest{
			Confirm: true,
			Files:   []cutover.BundleFile{{Kind: cutover.KindAccounts, FileName: "accounts.csv", CSVContent: "code,name,account_type\n1000,Cash,ASSET\n"}},
		})
		w := httptest.NewRecorder()

		h.ExecuteMigration(w, req)

		require.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())
		assert.Equal(t, 2, store.saves)
		assert.Contains(t, w.Body.String(), "Failed to save migration execution run")
	})

	t.Run("failure snapshot save failure", func(t *testing.T) {
		store := &failAfterMigrationRunStore{failAfter: 2, err: errors.New("save failed")}
		h := &Handlers{
			migrationExecutor: &fakeMigrationStepExecutor{err: errors.New("import failed")},
			migrationRunStore: store,
		}
		req := executeMigrationRequest(cutover.ExecuteMigrationRequest{
			Confirm: true,
			Files:   []cutover.BundleFile{{Kind: cutover.KindAccounts, FileName: "accounts.csv", CSVContent: "code,name,account_type\n1000,Cash,ASSET\n"}},
		})
		w := httptest.NewRecorder()

		h.ExecuteMigration(w, req)

		require.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())
		assert.Equal(t, 3, store.saves)
		assert.Contains(t, w.Body.String(), "Failed to save migration execution run")
	})

	t.Run("final save failure", func(t *testing.T) {
		store := &failAfterMigrationRunStore{failAfter: 3, err: errors.New("save failed")}
		h := &Handlers{
			migrationExecutor: &fakeMigrationStepExecutor{},
			migrationRunStore: store,
		}
		req := executeMigrationRequest(cutover.ExecuteMigrationRequest{
			Confirm: true,
			Files:   []cutover.BundleFile{{Kind: cutover.KindAccounts, FileName: "accounts.csv", CSVContent: "code,name,account_type\n1000,Cash,ASSET\n"}},
		})
		w := httptest.NewRecorder()

		h.ExecuteMigration(w, req)

		require.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())
		assert.Equal(t, 4, store.saves)
		assert.Contains(t, w.Body.String(), "Failed to save migration execution run")
	})
}

func TestMigrationExecutionRunHandlersAdditionalErrorBranches(t *testing.T) {
	t.Run("list validation and storage errors", func(t *testing.T) {
		h := &Handlers{}
		req := makeAuthenticatedRequest(http.MethodGet, "/tenants//migration/execution-runs", nil, createTestClaims("user-1", "user@example.com", "tenant-1", "admin"))
		w := httptest.NewRecorder()
		h.ListMigrationExecutionRuns(w, req)
		require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())

		req = withURLParams(makeAuthenticatedRequest(http.MethodGet, "/tenants/tenant-1/migration/execution-runs?limit=0", nil, createTestClaims("user-1", "user@example.com", "tenant-1", "admin")), map[string]string{"tenantID": "tenant-1"})
		w = httptest.NewRecorder()
		h.ListMigrationExecutionRuns(w, req)
		require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())

		req = withURLParams(makeAuthenticatedRequest(http.MethodGet, "/tenants/tenant-1/migration/execution-runs", nil, createTestClaims("user-1", "user@example.com", "tenant-1", "admin")), map[string]string{"tenantID": "tenant-1"})
		w = httptest.NewRecorder()
		h.ListMigrationExecutionRuns(w, req)
		require.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())

		h.migrationRunStore = &fakeMigrationRunStore{listErr: errors.New("list failed")}
		w = httptest.NewRecorder()
		h.ListMigrationExecutionRuns(w, req)
		require.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())
	})

	t.Run("get validation and storage errors", func(t *testing.T) {
		h := &Handlers{}
		req := makeAuthenticatedRequest(http.MethodGet, "/tenants//migration/execution-runs/run-1", nil, createTestClaims("user-1", "user@example.com", "tenant-1", "admin"))
		w := httptest.NewRecorder()
		h.GetMigrationExecutionRun(w, req)
		require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())

		req = withURLParams(makeAuthenticatedRequest(http.MethodGet, "/tenants/tenant-1/migration/execution-runs/", nil, createTestClaims("user-1", "user@example.com", "tenant-1", "admin")), map[string]string{"tenantID": "tenant-1"})
		w = httptest.NewRecorder()
		h.GetMigrationExecutionRun(w, req)
		require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())

		req = withURLParams(makeAuthenticatedRequest(http.MethodGet, "/tenants/tenant-1/migration/execution-runs/run-1", nil, createTestClaims("user-1", "user@example.com", "tenant-1", "admin")), map[string]string{"tenantID": "tenant-1", "runID": "run-1"})
		w = httptest.NewRecorder()
		h.GetMigrationExecutionRun(w, req)
		require.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())

		h.migrationRunStore = &fakeMigrationRunStore{getErr: errors.New("load failed")}
		w = httptest.NewRecorder()
		h.GetMigrationExecutionRun(w, req)
		require.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())
	})
}

func TestStreamMigrationExecutionRunHandlerAdditionalErrorBranches(t *testing.T) {
	baseRun := &cutover.MigrationExecutionRun{
		ID:      "run-1",
		Summary: cutover.MigrationExecutionRunSummary{Status: "running", StepCount: 1},
	}

	tests := []struct {
		name       string
		handler    *Handlers
		params     map[string]string
		query      string
		writer     http.ResponseWriter
		wantStatus int
		wantBody   string
	}{
		{
			name:       "missing tenant id",
			handler:    &Handlers{},
			params:     map[string]string{"runID": "run-1"},
			wantStatus: http.StatusBadRequest,
			wantBody:   "tenantID is required",
		},
		{
			name:       "missing run id",
			handler:    &Handlers{},
			params:     map[string]string{"tenantID": "tenant-1"},
			wantStatus: http.StatusBadRequest,
			wantBody:   "runID is required",
		},
		{
			name:       "missing store",
			handler:    &Handlers{},
			params:     map[string]string{"tenantID": "tenant-1", "runID": "run-1"},
			wantStatus: http.StatusInternalServerError,
			wantBody:   "Migration execution run storage is not configured",
		},
		{
			name:       "invalid interval",
			handler:    &Handlers{migrationRunStore: &fakeMigrationRunStore{}},
			params:     map[string]string{"tenantID": "tenant-1", "runID": "run-1"},
			query:      "interval_ms=0",
			wantStatus: http.StatusBadRequest,
			wantBody:   "interval_ms must be between 1 and 30000",
		},
		{
			name:       "invalid max events",
			handler:    &Handlers{migrationRunStore: &fakeMigrationRunStore{}},
			params:     map[string]string{"tenantID": "tenant-1", "runID": "run-1"},
			query:      "max_events=1001",
			wantStatus: http.StatusBadRequest,
			wantBody:   "max_events must be between 1 and 1000",
		},
		{
			name:       "not found",
			handler:    &Handlers{migrationRunStore: &fakeMigrationRunStore{getErr: cutover.ErrMigrationExecutionRunNotFound}},
			params:     map[string]string{"tenantID": "tenant-1", "runID": "run-1"},
			wantStatus: http.StatusNotFound,
			wantBody:   "migration execution run not found",
		},
		{
			name:       "load failure",
			handler:    &Handlers{migrationRunStore: &fakeMigrationRunStore{getErr: errors.New("load failed")}},
			params:     map[string]string{"tenantID": "tenant-1", "runID": "run-1"},
			wantStatus: http.StatusInternalServerError,
			wantBody:   "Failed to load migration execution run",
		},
		{
			name:       "writer without flusher",
			handler:    &Handlers{migrationRunStore: &fakeMigrationRunStore{getRun: baseRun}},
			params:     map[string]string{"tenantID": "tenant-1", "runID": "run-1"},
			writer:     &nonFlushingResponseRecorder{},
			wantStatus: http.StatusInternalServerError,
			wantBody:   "Streaming is not supported",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := "/tenants/tenant-1/migration/execution-runs/run-1/events"
			if tt.query != "" {
				path += "?" + tt.query
			}
			req := withURLParams(makeAuthenticatedRequest(http.MethodGet, path, nil, createTestClaims("user-1", "user@example.com", "tenant-1", "admin")), tt.params)
			writer := tt.writer
			if writer == nil {
				writer = httptest.NewRecorder()
			}

			tt.handler.StreamMigrationExecutionRun(writer, req)

			switch typed := writer.(type) {
			case *httptest.ResponseRecorder:
				require.Equal(t, tt.wantStatus, typed.Code, typed.Body.String())
				assert.Contains(t, typed.Body.String(), tt.wantBody)
			case *nonFlushingResponseRecorder:
				require.Equal(t, tt.wantStatus, typed.code, typed.body.String())
				assert.Contains(t, typed.body.String(), tt.wantBody)
			default:
				t.Fatalf("unexpected writer type %T", writer)
			}
		})
	}
}

func TestHandlerMigrationStepExecutorAdditionalBranches(t *testing.T) {
	executor := &handlerMigrationStepExecutor{h: &Handlers{}}

	tests := []struct {
		name string
		step cutover.MigrationExecutionStep
		file cutover.BundleFile
		req  *cutover.ExecuteMigrationRequest
		want string
	}{
		{
			name: "invalid e-invoice type",
			step: cutover.MigrationExecutionStep{Kind: cutover.KindEInvoices},
			file: cutover.BundleFile{Kind: cutover.KindEInvoices, FileName: "einvoice.xml", XMLContent: "<Invoice/>"},
			req:  &cutover.ExecuteMigrationRequest{EInvoiceInvoiceType: "receipt"},
			want: "invalid e_invoice_invoice_type",
		},
		{
			name: "bank transactions require account id",
			step: cutover.MigrationExecutionStep{Kind: cutover.KindBankTransactions},
			file: cutover.BundleFile{Kind: cutover.KindBankTransactions, FileName: "bank.csv", CSVContent: "date,amount,description\n2026-01-02,42.50,Customer receipt\n"},
			req:  &cutover.ExecuteMigrationRequest{},
			want: "bank_transaction_account_id is required",
		},
		{
			name: "opening balances require entry date",
			step: cutover.MigrationExecutionStep{Kind: cutover.KindOpeningBalances},
			file: cutover.BundleFile{Kind: cutover.KindOpeningBalances, FileName: "opening.csv", CSVContent: "account_code,debit,credit\n1000,100,0\n"},
			req:  &cutover.ExecuteMigrationRequest{},
			want: "opening_balance_entry_date is required",
		},
		{
			name: "unsupported kind",
			step: cutover.MigrationExecutionStep{Kind: cutover.FileKind("custom")},
			file: cutover.BundleFile{Kind: cutover.FileKind("custom"), FileName: "custom.csv", CSVContent: "value\n1\n"},
			req:  &cutover.ExecuteMigrationRequest{},
			want: "unsupported migration execution kind",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := executor.ExecuteMigrationStep(context.Background(), "tenant-1", "tenant_test", "user-1", tt.step, tt.file, tt.req)

			require.Error(t, err)
			assert.Nil(t, result)
			assert.Contains(t, err.Error(), tt.want)
		})
	}
}

func TestHandlerMigrationStepExecutorRepositoryBackedImportBranches(t *testing.T) {
	accountingRepo := newMockAccountingRepository()
	accountingRepo.accounts["acc-cash"] = &accounting.Account{ID: "acc-cash", TenantID: "tenant-1", Code: "1000", Name: "Cash", AccountType: accounting.AccountTypeAsset, IsActive: true}
	accountingRepo.accounts["acc-equity"] = &accounting.Account{ID: "acc-equity", TenantID: "tenant-1", Code: "3000", Name: "Equity", AccountType: accounting.AccountTypeEquity, IsActive: true}

	contactsRepo := newMockContactsRepository()
	customer := contactsRepo.addTestContact("contact-1", "tenant-1", "Customer One", contacts.ContactTypeCustomer, true)
	customer.Code = "CUST-1"
	customer.Email = "customer@example.com"

	bankingRepo := newMockBankingRepository()
	bankingRepo.accounts["bank-1"] = &banking.BankAccount{
		ID:            "bank-1",
		TenantID:      "tenant-1",
		Name:          "Main bank",
		AccountNumber: "EE471000001020145685",
		Currency:      "EUR",
		IsActive:      true,
	}

	expenseHandlers, _, _ := setupExpenseHandlers()
	recurringRepo := newMockRecurringRepository()

	h := &Handlers{
		tenantService:     expenseHandlers.tenantService,
		accountingService: accounting.NewServiceWithRepository(accountingRepo),
		contactsService:   contacts.NewServiceWithRepository(contactsRepo),
		invoicingService:  invoicing.NewServiceWithRepository(newMockInvoicingRepository(), nil),
		expensesService:   expenseHandlers.expensesService,
		bankingService: banking.NewServiceWithRepositoryAndAccounting(bankingRepo, fakeBankingAccountLister{accounts: []accounting.Account{
			{ID: "acc-cash", Code: "1000", AccountType: accounting.AccountTypeAsset},
		}}),
		inventoryService: inventory.NewServiceWithRepository(newMockInventoryRepository()),
		quotesService:    quotes.NewServiceWithRepository(newMockQuotesRepository()),
		ordersService:    orders.NewServiceWithRepository(newMockOrdersRepository()),
		recurringService: recurring.NewServiceWithDependencies(recurringRepo, &mockRecurringInvoicingService{}, nil, nil, expenseHandlers.tenantService, nil),
	}
	executor := &handlerMigrationStepExecutor{h: h}

	tests := []struct {
		name string
		step cutover.MigrationExecutionStep
		file cutover.BundleFile
		req  *cutover.ExecuteMigrationRequest
	}{
		{
			name: "accounts",
			step: cutover.MigrationExecutionStep{Kind: cutover.KindAccounts},
			file: cutover.BundleFile{Kind: cutover.KindAccounts, FileName: "accounts.csv", CSVContent: "code,name,account_type\n1100,Bank,ASSET\n"},
			req:  &cutover.ExecuteMigrationRequest{},
		},
		{
			name: "contacts",
			step: cutover.MigrationExecutionStep{Kind: cutover.KindContacts},
			file: cutover.BundleFile{Kind: cutover.KindContacts, FileName: "contacts.csv", CSVContent: "code,name,type,email\nCUST-2,Customer Two,CUSTOMER,two@example.com\n"},
			req:  &cutover.ExecuteMigrationRequest{},
		},
		{
			name: "expenses",
			step: cutover.MigrationExecutionStep{Kind: cutover.KindExpenses},
			file: cutover.BundleFile{Kind: cutover.KindExpenses, FileName: "expenses.csv", CSVContent: "expense_number,expense_date,merchant,expense_account_id,payment_account_id,amount,status\nEXP-1,2026-05-30,Office,expense-account,cash-account,10,DRAFT\n"},
			req:  &cutover.ExecuteMigrationRequest{},
		},
		{
			name: "invoices",
			step: cutover.MigrationExecutionStep{Kind: cutover.KindInvoices},
			file: cutover.BundleFile{Kind: cutover.KindInvoices, FileName: "invoices.csv", CSVContent: "invoice_number,invoice_type,contact_code,issue_date,due_date,line_description,quantity,unit_price,vat_rate\nINV-1,SALES,CUST-1,2026-05-30,2026-06-14,Work,1,100,22\n"},
			req:  &cutover.ExecuteMigrationRequest{},
		},
		{
			name: "bank accounts",
			step: cutover.MigrationExecutionStep{Kind: cutover.KindBankAccounts},
			file: cutover.BundleFile{Kind: cutover.KindBankAccounts, FileName: "bank-accounts.csv", CSVContent: "name,account_number,bank_name,currency\nReserve,EE871600161234567892,LHV,EUR\n"},
			req:  &cutover.ExecuteMigrationRequest{},
		},
		{
			name: "bank transactions",
			step: cutover.MigrationExecutionStep{Kind: cutover.KindBankTransactions},
			file: cutover.BundleFile{Kind: cutover.KindBankTransactions, FileName: "bank.csv", CSVContent: "date,amount,description\n2026-01-02,42.50,Customer receipt\n"},
			req:  &cutover.ExecuteMigrationRequest{BankTransactionAccountID: "bank-1"},
		},
		{
			name: "quotes",
			step: cutover.MigrationExecutionStep{Kind: cutover.KindQuotes},
			file: cutover.BundleFile{Kind: cutover.KindQuotes, FileName: "quotes.csv", CSVContent: "quote_number,contact_code,quote_date,line_description,quantity,unit_price,vat_rate\nQ-1,CUST-1,2026-05-30,Work,1,100,22\n"},
			req:  &cutover.ExecuteMigrationRequest{},
		},
		{
			name: "orders",
			step: cutover.MigrationExecutionStep{Kind: cutover.KindOrders},
			file: cutover.BundleFile{Kind: cutover.KindOrders, FileName: "orders.csv", CSVContent: "order_number,order_date,contact_code,line_description,quantity,unit_price,vat_rate\nSO-1,2026-05-31,CUST-1,Work,1,100,22\n"},
			req:  &cutover.ExecuteMigrationRequest{},
		},
		{
			name: "recurring invoices",
			step: cutover.MigrationExecutionStep{Kind: cutover.KindRecurringInvoices},
			file: cutover.BundleFile{Kind: cutover.KindRecurringInvoices, FileName: "recurring.csv", CSVContent: "name,contact_code,frequency,start_date,line_description,quantity,unit_price,vat_rate\nMonthly service,CUST-1,monthly,2026-06-01,Work,1,100,22\n"},
			req:  &cutover.ExecuteMigrationRequest{},
		},
		{
			name: "product categories",
			step: cutover.MigrationExecutionStep{Kind: cutover.KindProductCategories},
			file: cutover.BundleFile{Kind: cutover.KindProductCategories, FileName: "categories.csv", CSVContent: "name,description\nParts,Spare parts\n"},
			req:  &cutover.ExecuteMigrationRequest{},
		},
		{
			name: "warehouses",
			step: cutover.MigrationExecutionStep{Kind: cutover.KindWarehouses},
			file: cutover.BundleFile{Kind: cutover.KindWarehouses, FileName: "warehouses.csv", CSVContent: "code,name,address,is_default,status\nMAIN,Main Warehouse,Tallinn,true,ACTIVE\n"},
			req:  &cutover.ExecuteMigrationRequest{},
		},
		{
			name: "products",
			step: cutover.MigrationExecutionStep{Kind: cutover.KindProducts},
			file: cutover.BundleFile{Kind: cutover.KindProducts, FileName: "products.csv", CSVContent: "code,name,product_type,sales_price,purchase_price,track_inventory,status\nSKU-001,Widget,GOODS,15.00,10.50,true,ACTIVE\n"},
			req:  &cutover.ExecuteMigrationRequest{},
		},
		{
			name: "opening balances",
			step: cutover.MigrationExecutionStep{Kind: cutover.KindOpeningBalances},
			file: cutover.BundleFile{Kind: cutover.KindOpeningBalances, FileName: "opening.csv", CSVContent: "account_code,debit,credit,description\n1000,100,0,Cash\n3000,0,100,Equity\n"},
			req:  &cutover.ExecuteMigrationRequest{OpeningBalanceEntryDate: "2026-01-01"},
		},
		{
			name: "journal entries",
			step: cutover.MigrationExecutionStep{Kind: cutover.KindJournalEntries},
			file: cutover.BundleFile{Kind: cutover.KindJournalEntries, FileName: "journal.csv", CSVContent: "entry_reference,entry_date,entry_description,account_code,line_description,debit,credit\nJE-1,2026-03-31,Adjustment,1000,Cash,10,0\nJE-1,2026-03-31,Adjustment,3000,Equity,0,10\n"},
			req:  &cutover.ExecuteMigrationRequest{PostJournalEntries: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := executor.ExecuteMigrationStep(context.Background(), "tenant-1", "tenant_test", "user-1", tt.step, tt.file, tt.req)

			require.NoError(t, err)
			assert.NotNil(t, result)
			if tt.step.Kind == cutover.KindJournalEntries {
				importResult, ok := result.(*accounting.ImportJournalEntriesResult)
				require.True(t, ok)
				require.Len(t, importResult.JournalEntries, 1)
				assert.Equal(t, accounting.StatusPosted, importResult.JournalEntries[0].Status)
			}
		})
	}
}

func TestWriteMigrationExecutionRunSSEWriteError(t *testing.T) {
	err := writeMigrationExecutionRunSSE(&failingSSEWriter{}, &failingSSEWriter{}, cutover.MigrationExecutionRunEvent{
		Type:     "snapshot",
		Sequence: 1,
		Run:      &cutover.MigrationExecutionRun{ID: "run-1"},
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "write failed")
}

func TestMigrationExecutionRunStreamHelpersBoundaries(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/events", nil)
	interval, err := migrationExecutionRunStreamInterval(req)
	require.NoError(t, err)
	assert.Equal(t, "1s", interval.String())

	maxEvents, err := migrationExecutionRunStreamMaxEvents(req)
	require.NoError(t, err)
	assert.Equal(t, 100, maxEvents)

	for _, status := range []string{"succeeded", "failed", "blocked", "needs_confirmation"} {
		assert.True(t, migrationExecutionRunStreamTerminal(&cutover.MigrationExecutionRun{
			Summary: cutover.MigrationExecutionRunSummary{Status: status},
		}))
	}
	assert.True(t, migrationExecutionRunStreamTerminal(nil))
	assert.False(t, migrationExecutionRunStreamTerminal(&cutover.MigrationExecutionRun{
		Summary: cutover.MigrationExecutionRunSummary{Status: "running"},
	}))
}

func TestSavedMigrationExecutionRequestAndEffectiveExecutor(t *testing.T) {
	assert.Nil(t, savedMigrationExecutionRequest(nil))

	req := &cutover.ExecuteMigrationRequest{Confirm: true}
	run := &cutover.MigrationExecutionRun{ExecutionRequest: cutover.NewStoredMigrationExecutionRequest(req)}
	require.NotNil(t, savedMigrationExecutionRequest(run))
	assert.False(t, savedMigrationExecutionRequest(run).Confirm)

	custom := &fakeMigrationStepExecutor{}
	h := &Handlers{migrationExecutor: custom}
	assert.Same(t, custom, h.effectiveMigrationExecutor())

	h.migrationExecutor = nil
	assert.IsType(t, &handlerMigrationStepExecutor{}, h.effectiveMigrationExecutor())
}

func TestListMigrationExecutionRunsDefaultLimit(t *testing.T) {
	store := &fakeMigrationRunStore{}
	h := &Handlers{migrationRunStore: store}
	req := withURLParams(makeAuthenticatedRequest(http.MethodGet, "/tenants/tenant-1/migration/execution-runs", nil, &auth.Claims{
		UserID:   "user-1",
		Email:    "user@example.com",
		TenantID: "tenant-1",
		Role:     "admin",
	}), map[string]string{"tenantID": "tenant-1"})
	w := httptest.NewRecorder()

	h.ListMigrationExecutionRuns(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, 50, store.lastFilter.Limit)
	var runs []cutover.MigrationExecutionRun
	require.NoError(t, json.NewDecoder(w.Body).Decode(&runs))
	assert.Empty(t, runs)
}
