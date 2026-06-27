package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/HMB-research/open-accounting/internal/accounting"
	"github.com/HMB-research/open-accounting/internal/assets"
	"github.com/HMB-research/open-accounting/internal/contacts"
	"github.com/HMB-research/open-accounting/internal/cutover"
	"github.com/HMB-research/open-accounting/internal/documents"
	"github.com/HMB-research/open-accounting/internal/inventory"
	"github.com/HMB-research/open-accounting/internal/quotes"
	"github.com/HMB-research/open-accounting/internal/tax"
	"github.com/HMB-research/open-accounting/internal/tenant"
)

type wave6DocumentStore struct {
	reader  io.ReadCloser
	openErr error
}

func (s *wave6DocumentStore) Save(context.Context, string, io.Reader) error {
	return nil
}

func (s *wave6DocumentStore) Open(context.Context, string) (io.ReadCloser, error) {
	if s.openErr != nil {
		return nil, s.openErr
	}
	return s.reader, nil
}

func (s *wave6DocumentStore) Delete(context.Context, string) error {
	return nil
}

type wave6ReadCloser struct {
	reader   *strings.Reader
	readErr  error
	closeErr error
}

func (r *wave6ReadCloser) Read(payload []byte) (int, error) {
	if r.readErr != nil {
		return 0, r.readErr
	}
	return r.reader.Read(payload)
}

func (r *wave6ReadCloser) Close() error {
	return r.closeErr
}

type wave6ListDocumentsSequenceRepository struct {
	*mockDocumentRepository
	calls       int
	failOnCall  int
	listCallErr error
}

func (r *wave6ListDocumentsSequenceRepository) ListDocuments(ctx context.Context, schemaName, tenantID, entityType, entityID string) ([]documents.Document, error) {
	r.calls++
	if r.failOnCall > 0 && r.calls == r.failOnCall {
		return nil, r.listCallErr
	}
	return r.mockDocumentRepository.ListDocuments(ctx, schemaName, tenantID, entityType, entityID)
}

type wave6MigrationRunStore struct {
	runs     []*cutover.MigrationExecutionRun
	errs     []error
	getCalls int
}

func (s *wave6MigrationRunStore) SaveExecutionRun(context.Context, string, string, string, *cutover.MigrationExecutionRun) (*cutover.MigrationExecutionRun, error) {
	return nil, nil
}

func (s *wave6MigrationRunStore) ListExecutionRuns(context.Context, string, string, cutover.MigrationExecutionRunFilter) ([]cutover.MigrationExecutionRun, error) {
	return nil, nil
}

func (s *wave6MigrationRunStore) GetExecutionRun(context.Context, string, string, string) (*cutover.MigrationExecutionRun, error) {
	index := s.getCalls
	s.getCalls++
	if index < len(s.errs) && s.errs[index] != nil {
		return nil, s.errs[index]
	}
	if index >= len(s.runs) {
		index = len(s.runs) - 1
	}
	return s.runs[index], nil
}

type wave6TaxRepository struct {
	savedDeclarations []*tax.KMDDeclaration
}

func (r *wave6TaxRepository) QueryVATData(context.Context, string, string, time.Time, time.Time) ([]tax.VATAggregateRow, error) {
	return nil, nil
}

func (r *wave6TaxRepository) QueryKMDINFData(context.Context, string, string, time.Time, time.Time, decimal.Decimal) ([]tax.KMDINFReportRow, error) {
	return nil, nil
}

func (r *wave6TaxRepository) QueryEUVATOSSData(context.Context, string, string, time.Time, time.Time, bool) ([]tax.EUVATOSSReportRow, error) {
	return nil, nil
}

func (r *wave6TaxRepository) SaveDeclaration(_ context.Context, _ string, decl *tax.KMDDeclaration) error {
	r.savedDeclarations = append(r.savedDeclarations, decl)
	return nil
}

func (r *wave6TaxRepository) GetDeclaration(context.Context, string, string, int, int) (*tax.KMDDeclaration, error) {
	return nil, nil
}

func (r *wave6TaxRepository) ListDeclarations(context.Context, string, string) ([]tax.KMDDeclaration, error) {
	return nil, nil
}

func (r *wave6TaxRepository) MarkKMDSubmitted(context.Context, string, string, string, time.Time) error {
	return nil
}

func (r *wave6TaxRepository) UpdateKMDStatus(context.Context, string, string, string, string, time.Time) error {
	return nil
}

func setupWave6YearEndReady(t *testing.T) (*Handlers, *mockTenantRepository, *mockYearEndAccountingRepository, *tenant.Tenant) {
	t.Helper()

	h, repo, accountingRepo := setupTenantAccountingHandlers()
	settings := tenant.DefaultSettings()
	settings.PeriodLockDate = stringPtr("2025-12-31")
	tenantRecord := &tenant.Tenant{
		ID:         "tenant-1",
		Name:       "Tenant",
		Slug:       "tenant",
		SchemaName: "tenant_tenant",
		Settings:   settings,
	}
	repo.tenants["tenant-1"] = tenantRecord
	repo.tenantUsers["tenant-1"] = []tenant.TenantUser{
		{TenantID: "tenant-1", UserID: "user-1", Role: tenant.RoleOwner},
	}
	seedYearEndAccountingReady(accountingRepo)
	return h, repo, accountingRepo, tenantRecord
}

func wave6YearEndRequest(method, path string, body any, tenantID string) *http.Request {
	req := makeAuthenticatedRequest(method, path, body, createTestClaims("user-1", "user@example.com", tenantID, "owner"))
	return withURLParams(req, map[string]string{"tenantID": tenantID})
}

func TestWave6YearEndHandlerErrorBranches(t *testing.T) {
	t.Run("status maps close pack evidence evaluation failure", func(t *testing.T) {
		h, _, _, _ := setupWave6YearEndReady(t)
		docRepo := newMockDocumentRepository()
		docRepo.listDocumentsErr = errors.New("evidence store unavailable")
		h.documentsService = documents.NewService(docRepo, nil)
		req := wave6YearEndRequest(http.MethodGet, "/tenants/tenant-1/year-end-close-status?period_end_date=2025-12-31", nil, "tenant-1")
		rec := httptest.NewRecorder()

		h.GetYearEndCloseStatus(rec, req)

		require.Equal(t, http.StatusInternalServerError, rec.Code, rec.Body.String())
		assert.Contains(t, rec.Body.String(), "Failed to evaluate close-pack evidence")
	})

	t.Run("status maps inventory valuation failure", func(t *testing.T) {
		h, _, _, _ := setupWave6YearEndReady(t)
		attachYearEndInventoryFixture(h, decimal.NewFromInt(5))
		req := wave6YearEndRequest(http.MethodGet, "/tenants/tenant-1/year-end-close-status?period_end_date=2025-12-31&inventory_valuation_method=bad-method", nil, "tenant-1")
		rec := httptest.NewRecorder()

		h.GetYearEndCloseStatus(rec, req)

		require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
		assert.Contains(t, rec.Body.String(), "invalid valuation method")
	})

	t.Run("pack maps tenant and evidence failures", func(t *testing.T) {
		h, repo, _, _ := setupWave6YearEndReady(t)
		delete(repo.tenants, "tenant-1")
		req := wave6YearEndRequest(http.MethodGet, "/tenants/tenant-1/year-end-close-pack?period_end_date=2025-12-31", nil, "tenant-1")
		rec := httptest.NewRecorder()

		h.GetYearEndClosePack(rec, req)

		require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())

		h, _, _, _ = setupWave6YearEndReady(t)
		docRepo := newMockDocumentRepository()
		docRepo.listDocumentsErr = errors.New("evidence store unavailable")
		h.documentsService = documents.NewService(docRepo, nil)
		req = wave6YearEndRequest(http.MethodGet, "/tenants/tenant-1/year-end-close-pack?period_end_date=2025-12-31", nil, "tenant-1")
		rec = httptest.NewRecorder()

		h.GetYearEndClosePack(rec, req)

		require.Equal(t, http.StatusInternalServerError, rec.Code, rec.Body.String())
		assert.Contains(t, rec.Body.String(), "Failed to evaluate close-pack evidence")
	})

	t.Run("audit and archive map lookup and workflow failures", func(t *testing.T) {
		h, repo, _, _ := setupWave6YearEndReady(t)
		delete(repo.tenants, "tenant-1")
		req := wave6YearEndRequest(http.MethodGet, "/tenants/tenant-1/year-end-close-audit-evidence?period_end_date=2025-12-31", nil, "tenant-1")
		rec := httptest.NewRecorder()

		h.GetYearEndCloseAuditEvidence(rec, req)

		require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())

		h, repo, _, _ = setupWave6YearEndReady(t)
		delete(repo.tenants, "tenant-1")
		h.documentsService = documents.NewService(newMockDocumentRepository(), &wave6DocumentStore{reader: io.NopCloser(strings.NewReader("doc"))})
		req = wave6YearEndRequest(http.MethodGet, "/tenants/tenant-1/year-end-close-audit-archive?period_end_date=2025-12-31", nil, "tenant-1")
		rec = httptest.NewRecorder()

		h.DownloadYearEndCloseAuditArchive(rec, req)

		require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())

		h, _, _, _ = setupWave6YearEndReady(t)
		h.documentsService = documents.NewService(newMockDocumentRepository(), &wave6DocumentStore{reader: io.NopCloser(strings.NewReader("doc"))})
		attachYearEndInventoryFixture(h, decimal.NewFromInt(5))
		req = wave6YearEndRequest(http.MethodGet, "/tenants/tenant-1/year-end-close-audit-archive?period_end_date=2025-12-31&inventory_valuation_method=bad-method", nil, "tenant-1")
		rec = httptest.NewRecorder()

		h.DownloadYearEndCloseAuditArchive(rec, req)

		require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
		assert.Contains(t, rec.Body.String(), "invalid valuation method")
	})

	t.Run("carry forward and reversal stop before service work", func(t *testing.T) {
		h, repo, _, _ := setupWave6YearEndReady(t)
		delete(repo.tenants, "tenant-1")
		req := wave6YearEndRequest(http.MethodPost, "/tenants/tenant-1/year-end-carry-forward", map[string]any{
			"period_end_date": "2025-12-31",
		}, "tenant-1")
		rec := httptest.NewRecorder()

		h.CreateYearEndCarryForward(rec, req)

		require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())

		req = httptest.NewRequest(http.MethodPost, "/tenants/tenant-1/year-end-carry-forward/reverse", strings.NewReader(`{"period_end_date":"2025-12-31","reason":"late accrual"}`))
		req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
		rec = httptest.NewRecorder()

		h.ReverseYearEndCarryForward(rec, req)

		require.Equal(t, http.StatusUnauthorized, rec.Code, rec.Body.String())
		assert.Contains(t, rec.Body.String(), "Not authenticated")
	})
}

func TestWave6YearEndHelperBranches(t *testing.T) {
	t.Run("carry forward existence handles nil service and service errors", func(t *testing.T) {
		_, _, _, tenantRecord := setupWave6YearEndReady(t)
		req := httptest.NewRequest(http.MethodGet, "/", nil)

		exists, err := (&Handlers{}).yearEndCarryForwardExists(req, tenantRecord, "2025-12-31")
		require.NoError(t, err)
		assert.False(t, exists)

		h, _, accountingRepo, tenantRecord := setupWave6YearEndReady(t)
		accountingRepo.periodBalanceErr = errors.New("balances unavailable")
		exists, err = h.yearEndCarryForwardExists(req, tenantRecord, "2025-12-31")
		require.Error(t, err)
		assert.False(t, exists)
	})

	t.Run("audit evidence wraps pack evidence and document list failures", func(t *testing.T) {
		h, _, _, tenantRecord := setupWave6YearEndReady(t)
		docRepo := &wave6ListDocumentsSequenceRepository{
			mockDocumentRepository: newMockDocumentRepository(),
			failOnCall:             1,
			listCallErr:            errors.New("policy failed"),
		}
		h.documentsService = documents.NewService(docRepo, nil)

		audit, err := h.buildYearEndCloseAuditEvidence(context.Background(), tenantRecord, "2025-12-31", "")

		require.Error(t, err)
		assert.Nil(t, audit)
		assert.Contains(t, err.Error(), "evaluate close-pack evidence")

		h, _, _, tenantRecord = setupWave6YearEndReady(t)
		entityID, err := accounting.YearEndCloseEvidenceEntityID("tenant-1", "2025-12-31")
		require.NoError(t, err)
		docRepo = &wave6ListDocumentsSequenceRepository{
			mockDocumentRepository: newMockDocumentRepository(),
			failOnCall:             2,
			listCallErr:            errors.New("list failed"),
		}
		docRepo.docs["doc-1"] = &documents.Document{
			ID:           "doc-1",
			TenantID:     "tenant-1",
			EntityType:   documents.EntityTypeYearEndClose,
			EntityID:     entityID,
			DocumentType: documents.DocumentTypeClosePack,
			FileName:     "pack.pdf",
			ReviewStatus: documents.ReviewStatusApproved,
		}
		h.documentsService = documents.NewService(docRepo, nil)

		audit, err = h.buildYearEndCloseAuditEvidence(context.Background(), tenantRecord, "2025-12-31", "")

		require.Error(t, err)
		assert.Nil(t, audit)
		assert.Contains(t, err.Error(), "list failed")
	})

	t.Run("audit evidence returns pack and propagates inventory errors", func(t *testing.T) {
		h, _, accountingRepo, tenantRecord := setupWave6YearEndReady(t)
		accountingRepo.periodBalanceErr = errors.New("pack failed")

		audit, err := h.buildYearEndCloseAuditEvidence(context.Background(), tenantRecord, "2025-12-31", "")

		require.Error(t, err)
		assert.Nil(t, audit)

		h, _, _, tenantRecord = setupWave6YearEndReady(t)
		attachYearEndInventoryFixture(h, decimal.NewFromInt(5))

		audit, err = h.buildYearEndCloseAuditEvidence(context.Background(), tenantRecord, "2025-12-31", "bad-method")

		require.Error(t, err)
		assert.Nil(t, audit)
		assert.Contains(t, err.Error(), "invalid valuation method")
	})

	t.Run("archive reports document read and close failures", func(t *testing.T) {
		_, _, _, tenantRecord := setupWave6YearEndReady(t)
		docRepo := newMockDocumentRepository()
		docRepo.docs["doc-1"] = &documents.Document{ID: "doc-1", TenantID: "tenant-1", FileName: "pack.pdf", StorageKey: "pack.pdf"}
		audit := &accounting.YearEndCloseAuditEvidence{Documents: []documents.Document{{ID: "doc-1"}}}

		h := &Handlers{documentsService: documents.NewService(docRepo, &wave6DocumentStore{
			reader: &wave6ReadCloser{reader: strings.NewReader(""), readErr: errors.New("read failed")},
		})}
		archive, err := h.buildYearEndCloseAuditArchive(context.Background(), tenantRecord, audit)
		require.Error(t, err)
		assert.Nil(t, archive)
		assert.Contains(t, err.Error(), "write archive document entry")

		h.documentsService = documents.NewService(docRepo, &wave6DocumentStore{
			reader: &wave6ReadCloser{reader: strings.NewReader("document payload"), closeErr: errors.New("close failed")},
		})
		archive, err = h.buildYearEndCloseAuditArchive(context.Background(), tenantRecord, audit)
		require.Error(t, err)
		assert.Nil(t, archive)
		assert.Contains(t, err.Error(), "close document reader")
	})

	t.Run("evidence and inventory helpers handle boundaries", func(t *testing.T) {
		h, _, _, tenantRecord := setupWave6YearEndReady(t)
		require.NoError(t, h.requireApprovedYearEndClosePackEvidence(context.Background(), tenantRecord, "2025-11-30"))

		err := h.requireApprovedYearEndClosePackEvidence(context.Background(), tenantRecord, "bad-date")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "period end date")

		docRepo := newMockDocumentRepository()
		docRepo.listDocumentsErr = errors.New("policy failed")
		h.documentsService = documents.NewService(docRepo, nil)
		err = h.requireApprovedYearEndClosePackEvidence(context.Background(), tenantRecord, "2025-12-31")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "policy failed")

		status := &accounting.YearEndCloseStatus{CarryForwardReady: true}
		require.NoError(t, h.attachYearEndCloseEvidenceStatus(context.Background(), "tenant_tenant", "tenant-1", status))
		assert.Nil(t, status.ClosePackEvidence)

		status.ClosePackEvidenceEntityID = "entity-1"
		err = h.attachYearEndCloseEvidenceStatus(context.Background(), "tenant_tenant", "tenant-1", status)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "policy failed")

		review, err := (&Handlers{}).yearEndInventoryCostingReview(context.Background(), "tenant_tenant", "tenant-1", "")
		require.NoError(t, err)
		assert.Nil(t, review)

		attachYearEndInventoryFixture(h, decimal.NewFromInt(5))
		err = h.requireYearEndInventoryCostingReady(context.Background(), "tenant_tenant", "tenant-1", tenantRecord.Settings.FiscalYearStart, "bad-date", "")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "period end date")

		require.NoError(t, h.requireYearEndInventoryCostingReady(context.Background(), "tenant_tenant", "tenant-1", tenantRecord.Settings.FiscalYearStart, "2025-11-30", ""))

		_, err = h.yearEndInventoryCostingReview(context.Background(), "tenant_tenant", "tenant-1", "bad-method")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid valuation method")
	})
}

func TestWave6MigrationExecutionHandlerBranches(t *testing.T) {
	t.Run("confirmed ready initial save failure", func(t *testing.T) {
		h := &Handlers{
			migrationExecutor: &fakeMigrationStepExecutor{},
			migrationRunStore: &fakeMigrationRunStore{saveErr: errors.New("save failed")},
		}
		req := executeMigrationRequest(cutover.ExecuteMigrationRequest{
			Confirm: true,
			Files: []cutover.BundleFile{{
				Kind:       cutover.KindAccounts,
				FileName:   "accounts.csv",
				CSVContent: "code,name,account_type\n1000,Cash,ASSET\n",
			}},
		})
		rec := httptest.NewRecorder()

		h.ExecuteMigration(rec, req)

		require.Equal(t, http.StatusInternalServerError, rec.Code, rec.Body.String())
		assert.Contains(t, rec.Body.String(), "Failed to save migration execution run")
	})

	t.Run("successful step snapshot save failure", func(t *testing.T) {
		store := &failAfterMigrationRunStore{failAfter: 2, err: errors.New("save failed")}
		h := &Handlers{
			migrationExecutor: &fakeMigrationStepExecutor{},
			migrationRunStore: store,
		}
		req := executeMigrationRequest(cutover.ExecuteMigrationRequest{
			Confirm: true,
			Files: []cutover.BundleFile{{
				Kind:       cutover.KindAccounts,
				FileName:   "accounts.csv",
				CSVContent: "code,name,account_type\n1000,Cash,ASSET\n",
			}},
		})
		rec := httptest.NewRecorder()

		h.ExecuteMigration(rec, req)

		require.Equal(t, http.StatusInternalServerError, rec.Code, rec.Body.String())
		assert.Equal(t, 3, store.saves)
		assert.Contains(t, rec.Body.String(), "Failed to save migration execution run")
	})

	t.Run("resolve resume run boundaries", func(t *testing.T) {
		h := &Handlers{}
		run, err := h.resolveMigrationExecutionResumeRun(context.Background(), "tenant_tenant", "tenant-1", nil)
		require.NoError(t, err)
		assert.Nil(t, run)

		run, err = h.resolveMigrationExecutionResumeRun(context.Background(), "tenant_tenant", "tenant-1", &cutover.ExecuteMigrationRequest{ResumeFromRunID: "run-1"})
		require.Error(t, err)
		assert.Nil(t, run)
		assert.Contains(t, err.Error(), "storage is not configured")
	})

	t.Run("stream exits on write error context cancellation and refresh errors", func(t *testing.T) {
		runningRun := &cutover.MigrationExecutionRun{ID: "run-1", Summary: cutover.MigrationExecutionRunSummary{Status: "running", StepCount: 1}}

		h := &Handlers{migrationRunStore: &fakeMigrationRunStore{getRun: runningRun}}
		req := withURLParams(makeAuthenticatedRequest(http.MethodGet, "/tenants/tenant-1/migration/execution-runs/run-1/events", nil, createTestClaims("user-1", "user@example.com", "tenant-1", "admin")), map[string]string{"tenantID": "tenant-1", "runID": "run-1"})
		h.StreamMigrationExecutionRun(&failingSSEWriter{}, req)

		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		req = httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/migration/execution-runs/run-1/events?interval_ms=1&max_events=2", nil).WithContext(contextWithClaims(ctx, createTestClaims("user-1", "user@example.com", "tenant-1", "admin")))
		req = withURLParams(req, map[string]string{"tenantID": "tenant-1", "runID": "run-1"})
		rec := httptest.NewRecorder()
		h.StreamMigrationExecutionRun(rec, req)
		assert.Contains(t, rec.Body.String(), "event: snapshot")

		for _, tt := range []struct {
			name string
			err  error
		}{
			{name: "not found", err: cutover.ErrMigrationExecutionRunNotFound},
			{name: "load failed", err: errors.New("load failed")},
		} {
			t.Run(tt.name, func(t *testing.T) {
				store := &wave6MigrationRunStore{
					runs: []*cutover.MigrationExecutionRun{runningRun},
					errs: []error{nil, tt.err},
				}
				h := &Handlers{migrationRunStore: store}
				req := withURLParams(makeAuthenticatedRequest(http.MethodGet, "/tenants/tenant-1/migration/execution-runs/run-1/events?interval_ms=1&max_events=2", nil, createTestClaims("user-1", "user@example.com", "tenant-1", "admin")), map[string]string{"tenantID": "tenant-1", "runID": "run-1"})
				rec := httptest.NewRecorder()

				h.StreamMigrationExecutionRun(rec, req)

				require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
				assert.Contains(t, rec.Body.String(), "event: error")
				assert.Contains(t, rec.Body.String(), `"sequence":2`)
			})
		}
	})

	t.Run("sse rejects unmarshalable run payload", func(t *testing.T) {
		err := writeMigrationExecutionRunSSE(httptest.NewRecorder(), httptest.NewRecorder(), cutover.MigrationExecutionRunEvent{
			Type:     "snapshot",
			Sequence: 1,
			Run: &cutover.MigrationExecutionRun{Steps: []cutover.MigrationExecutionStepRun{{
				Response: func() {},
			}}},
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported type")
	})
}

func TestWave6MigrationStepExecutorBranches(t *testing.T) {
	t.Run("canonicalization errors are returned before dispatch", func(t *testing.T) {
		executor := &handlerMigrationStepExecutor{h: &Handlers{}}

		result, err := executor.ExecuteMigrationStep(context.Background(), "tenant-1", "tenant_tenant", "user-1",
			cutover.MigrationExecutionStep{Kind: cutover.KindAccounts},
			cutover.BundleFile{Kind: cutover.KindAccounts, FileName: "accounts.csv", CSVContent: "\"unterminated"},
			&cutover.ExecuteMigrationRequest{ProviderPreset: cutover.MigrationProviderPresetDirecto},
		)

		require.Error(t, err)
		assert.Nil(t, result)
	})

	t.Run("period lock validation errors stop expense and payment imports", func(t *testing.T) {
		h, repo := setupTenantTestHandlers()
		tenantRecord := repo.addTestTenant("tenant-1", "Tenant", "tenant")
		tenantRecord.Settings.PeriodLockDate = stringPtr("05/31/2026")
		executor := &handlerMigrationStepExecutor{h: h}

		for _, kind := range []cutover.FileKind{cutover.KindExpenses, cutover.KindPayments} {
			result, err := executor.ExecuteMigrationStep(context.Background(), "tenant-1", "tenant_tenant", "user-1",
				cutover.MigrationExecutionStep{Kind: kind},
				cutover.BundleFile{Kind: kind, FileName: string(kind) + ".csv", CSVContent: "value\n"},
				&cutover.ExecuteMigrationRequest{},
			)

			require.Error(t, err)
			assert.Nil(t, result)
			assert.Contains(t, err.Error(), "validate period lock")
		}
	})

	t.Run("dispatches additional domain import branches", func(t *testing.T) {
		payrollHandlers, _, _ := setupPayrollImportHandlerTest(t)
		assetHandlers, _, _ := setupAssetsTestHandlers()
		h := &Handlers{
			payrollService:    payrollHandlers.payrollService,
			absenceService:    payrollHandlers.absenceService,
			taxService:        tax.NewServiceWithRepository(&wave6TaxRepository{}),
			costCenterService: accounting.NewCostCenterServiceWithRepository(accounting.NewMockCostCenterRepository()),
			inventoryService:  inventory.NewServiceWithRepository(newMockInventoryRepository()),
			assetsService:     assetHandlers.assetsService,
		}
		executor := &handlerMigrationStepExecutor{h: h}

		tests := []struct {
			name string
			kind cutover.FileKind
		}{
			{name: "employees", kind: cutover.KindEmployees},
			{name: "payroll history", kind: cutover.KindPayrollHistory},
			{name: "leave balances", kind: cutover.KindLeaveBalances},
			{name: "tsd history", kind: cutover.KindTSDHistory},
			{name: "kmd history", kind: cutover.KindKMDHistory},
			{name: "cost centers", kind: cutover.KindCostCenters},
			{name: "cost allocations", kind: cutover.KindCostAllocations},
			{name: "stock adjustments", kind: cutover.KindStockAdjustments},
			{name: "fixed assets", kind: cutover.KindFixedAssets},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result, err := executor.ExecuteMigrationStep(context.Background(), "tenant-1", "tenant_tenant", "user-1",
					cutover.MigrationExecutionStep{Kind: tt.kind},
					cutover.BundleFile{Kind: tt.kind, FileName: string(tt.kind) + ".csv", CSVContent: " "},
					&cutover.ExecuteMigrationRequest{},
				)

				require.Error(t, err)
				assert.Nil(t, result)
				assert.Contains(t, err.Error(), "csv_content is required")
			})
		}
	})

	t.Run("migration import references and order quote references propagate list errors", func(t *testing.T) {
		contactsRepo := newMockContactsRepository()
		contactsRepo.listErr = errors.New("contacts unavailable")
		h := &Handlers{contactsService: contacts.NewServiceWithRepository(contactsRepo)}

		contactsList, productsList, err := h.migrationImportReferences(context.Background(), "tenant-1", "tenant_tenant")
		require.Error(t, err)
		assert.Nil(t, contactsList)
		assert.Nil(t, productsList)
		assert.Contains(t, err.Error(), "load contacts")

		contactsRepo.listErr = nil
		inventoryRepo := newMockInventoryRepository()
		inventoryRepo.listProductsErr = errors.New("products unavailable")
		h.inventoryService = inventory.NewServiceWithRepository(inventoryRepo)

		contactsList, productsList, err = h.migrationImportReferences(context.Background(), "tenant-1", "tenant_tenant")
		require.Error(t, err)
		assert.Nil(t, contactsList)
		assert.Nil(t, productsList)
		assert.Contains(t, err.Error(), "load products")

		refs, err := (&Handlers{}).importOrderQuoteReferences(context.Background(), "tenant-1", "tenant_tenant")
		require.NoError(t, err)
		assert.Nil(t, refs)

		quotesRepo := newMockQuotesRepository()
		quotesRepo.listErr = errors.New("quotes unavailable")
		h.quotesService = quotes.NewServiceWithRepository(quotesRepo)
		refs, err = h.importOrderQuoteReferences(context.Background(), "tenant-1", "tenant_tenant")
		require.Error(t, err)
		assert.Nil(t, refs)
		assert.Contains(t, err.Error(), "quotes unavailable")
	})
}

func TestWave6MigrationKMDAndAssetHappyDispatchSmoke(t *testing.T) {
	taxRepo := &wave6TaxRepository{}
	assetHandlers, assetRepo, _ := setupAssetsTestHandlers()
	assetRepo.categories["cat-1"] = &assets.AssetCategory{
		ID:                 "cat-1",
		TenantID:           "tenant-1",
		Name:               "Computers",
		DepreciationMethod: assets.DepreciationStraightLine,
	}
	h := &Handlers{
		taxService:    tax.NewServiceWithRepository(taxRepo),
		assetsService: assetHandlers.assetsService,
	}
	executor := &handlerMigrationStepExecutor{h: h}

	result, err := executor.ExecuteMigrationStep(context.Background(), "tenant-1", "tenant_tenant", "user-1",
		cutover.MigrationExecutionStep{Kind: cutover.KindKMDHistory},
		cutover.BundleFile{
			Kind:     cutover.KindKMDHistory,
			FileName: "kmd.csv",
			CSVContent: `year,month,status,submitted_at,row_code,description,tax_base,tax_amount
2025,12,SUBMITTED,2026-01-20,1,Taxable sales,1000.00,220.00
`,
		},
		&cutover.ExecuteMigrationRequest{},
	)
	require.NoError(t, err)
	assert.NotNil(t, result)
	require.Len(t, taxRepo.savedDeclarations, 1)

	result, err = executor.ExecuteMigrationStep(context.Background(), "tenant-1", "tenant_tenant", "user-1",
		cutover.MigrationExecutionStep{Kind: cutover.KindFixedAssets},
		cutover.BundleFile{
			Kind:       cutover.KindFixedAssets,
			FileName:   "assets.csv",
			CSVContent: "asset_number,name,category_name,status,purchase_date,purchase_cost,asset_account_code,depreciation_expense_account_code,accumulated_depreciation_account_code\nLEG-001,Laptop,Computers,ACTIVE,2025-01-10,1200.00,FA,DEP-EXP,ACC-DEP\n",
		},
		&cutover.ExecuteMigrationRequest{},
	)
	require.NoError(t, err)
	assert.NotNil(t, result)
	foundAssetNumber := false
	for _, asset := range assetRepo.assets {
		if asset.AssetNumber == "LEG-001" {
			foundAssetNumber = true
			break
		}
	}
	assert.True(t, foundAssetNumber)

	var run cutover.MigrationExecutionRun
	payload, err := json.Marshal(cutover.MigrationExecutionRun{ID: "run-json-smoke"})
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(payload, &run))
	assert.Equal(t, "run-json-smoke", run.ID)
}
