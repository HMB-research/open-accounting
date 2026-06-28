package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/HMB-research/open-accounting/internal/accounting"
	"github.com/HMB-research/open-accounting/internal/contacts"
	"github.com/HMB-research/open-accounting/internal/cutover"
	"github.com/HMB-research/open-accounting/internal/documents"
	"github.com/HMB-research/open-accounting/internal/email"
	"github.com/HMB-research/open-accounting/internal/expenses"
	"github.com/HMB-research/open-accounting/internal/inventory"
	"github.com/HMB-research/open-accounting/internal/invoicing"
	"github.com/HMB-research/open-accounting/internal/payments"
	"github.com/HMB-research/open-accounting/internal/plugin"
	"github.com/HMB-research/open-accounting/internal/recurring"
	"github.com/HMB-research/open-accounting/internal/tax"
	"github.com/HMB-research/open-accounting/internal/tenant"
)

type wave11TenantRepository struct {
	*mockTenantRepository
	getTenantCalls     int
	failGetTenantOn    int
	getTenantErr       error
	getTenantUserCalls int
	failGetUserOn      int
	getTenantUserErr   error
}

func (r *wave11TenantRepository) GetTenant(ctx context.Context, tenantID string) (*tenant.Tenant, error) {
	r.getTenantCalls++
	if r.failGetTenantOn > 0 && r.getTenantCalls == r.failGetTenantOn {
		return nil, r.getTenantErr
	}
	return r.mockTenantRepository.GetTenant(ctx, tenantID)
}

func (r *wave11TenantRepository) GetTenantUser(ctx context.Context, tenantID, userID string) (*tenant.TenantUser, error) {
	r.getTenantUserCalls++
	if r.failGetUserOn > 0 && r.getTenantUserCalls == r.failGetUserOn {
		return nil, r.getTenantUserErr
	}
	return r.mockTenantRepository.GetTenantUser(ctx, tenantID, userID)
}

type wave11InventoryRepository struct {
	*mockInventoryRepository
	listProductsCalls int
	failOnCall        int
	listErr           error
}

func (r *wave11InventoryRepository) ListProducts(ctx context.Context, schemaName, tenantID string, filter *inventory.ProductFilter) ([]inventory.Product, error) {
	r.listProductsCalls++
	if r.failOnCall > 0 && r.listProductsCalls == r.failOnCall {
		return nil, r.listErr
	}
	return r.mockInventoryRepository.ListProducts(ctx, schemaName, tenantID, filter)
}

type wave11ExpenseAccounting struct {
	*expenseHandlerAccounting
	postErr error
}

func (a *wave11ExpenseAccounting) PostJournalEntry(context.Context, string, string, string, string, string) error {
	return a.postErr
}

func seedWave11InventoryReady(repo *mockInventoryRepository, unitCost decimal.Decimal) {
	repo.products[apiInventoryStockProductID] = &inventory.Product{
		ID:             apiInventoryStockProductID,
		TenantID:       "tenant-1",
		Code:           "SKU-001",
		Name:           "Widget",
		ProductType:    inventory.ProductTypeGoods,
		PurchasePrice:  unitCost,
		CurrentStock:   decimal.NewFromInt(4),
		TrackInventory: true,
		IsActive:       true,
	}
	repo.warehouses[apiInventoryStockWarehouseID] = &inventory.Warehouse{
		ID:       apiInventoryStockWarehouseID,
		TenantID: "tenant-1",
		Code:     "MAIN",
		Name:     "Main warehouse",
		IsActive: true,
	}
	repo.stockLevels[apiInventoryStockLevelKey(apiInventoryStockProductID, apiInventoryStockWarehouseID)] = &inventory.StockLevel{
		ID:           "stock-1",
		TenantID:     "tenant-1",
		ProductID:    apiInventoryStockProductID,
		WarehouseID:  apiInventoryStockWarehouseID,
		Quantity:     decimal.NewFromInt(4),
		ReservedQty:  decimal.Zero,
		AvailableQty: decimal.NewFromInt(4),
	}
	repo.movements[apiInventoryStockProductID] = []inventory.InventoryMovement{{
		ID:           "movement-1",
		TenantID:     "tenant-1",
		ProductID:    apiInventoryStockProductID,
		WarehouseID:  apiInventoryStockWarehouseID,
		MovementType: inventory.MovementTypeIn,
		Quantity:     decimal.NewFromInt(4),
		UnitCost:     unitCost,
		TotalCost:    unitCost.Mul(decimal.NewFromInt(4)),
		MovementDate: time.Date(2025, 12, 15, 0, 0, 0, 0, time.UTC),
	}}
}

func TestWave11RecurringHandlerErrorBranches(t *testing.T) {
	claims := createTestClaims("user-1", "user@example.com", "tenant-1", tenant.RoleOwner)
	validCSV := "name,contact_code,frequency,start_date,line_description,quantity,unit_price,vat_rate\nMonthly,CUST-1,MONTHLY,2026-06-01,Work,1,100,22\n"

	t.Run("import maps product loading failure", func(t *testing.T) {
		h, tenantRepo, _, contactsRepo := setupRecurringImportTestHandlers()
		tenantRepo.addTestTenant("tenant-1", "Recurring Tenant", "recurring-tenant")
		contact := contactsRepo.addTestContact("contact-1", "tenant-1", "Acme", contacts.ContactTypeCustomer, true)
		contact.Code = "CUST-1"
		inventoryRepo := newMockInventoryRepository()
		inventoryRepo.listProductsErr = errors.New("products unavailable")
		h.inventoryService = inventory.NewServiceWithRepository(inventoryRepo)

		req := withURLParams(makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/recurring-invoices/import", recurring.ImportRecurringInvoicesRequest{
			FileName:   "recurring.csv",
			CSVContent: validCSV,
		}, claims), map[string]string{"tenantID": "tenant-1"})
		rec := httptest.NewRecorder()

		h.ImportRecurringInvoices(rec, req)

		require.Equal(t, http.StatusInternalServerError, rec.Code, rec.Body.String())
		assert.Contains(t, rec.Body.String(), "Failed to load products")
	})

	t.Run("import maps service validation failure", func(t *testing.T) {
		h, tenantRepo, recurringRepo, contactsRepo := setupRecurringImportTestHandlers()
		tenantRepo.addTestTenant("tenant-1", "Recurring Tenant", "recurring-tenant")
		contact := contactsRepo.addTestContact("contact-1", "tenant-1", "Acme", contacts.ContactTypeCustomer, true)
		contact.Code = "CUST-1"
		recurringRepo.listErr = errors.New("recurring storage failed")

		req := withURLParams(makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/recurring-invoices/import", recurring.ImportRecurringInvoicesRequest{
			FileName:   "recurring.csv",
			CSVContent: validCSV,
		}, claims), map[string]string{"tenantID": "tenant-1"})
		rec := httptest.NewRecorder()

		h.ImportRecurringInvoices(rec, req)

		require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
		assert.Contains(t, rec.Body.String(), "list existing recurring invoices")
	})

	t.Run("create from invoice maps service failure", func(t *testing.T) {
		h, tenantRepo, _, invoicingSvc := setupRecurringTestHandlers()
		tenantRepo.addTestTenant("tenant-1", "Recurring Tenant", "recurring-tenant")
		invoicingSvc.getByIDErr = errors.New("invoice service offline")

		req := withURLParams(makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/recurring-invoices/from-invoice/inv-1", map[string]any{
			"name":       "From invoice",
			"frequency":  "MONTHLY",
			"start_date": time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		}, claims), map[string]string{"tenantID": "tenant-1", "invoiceID": "inv-1"})
		rec := httptest.NewRecorder()

		h.CreateRecurringInvoiceFromInvoice(rec, req)

		require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
		assert.Contains(t, rec.Body.String(), "invoice service offline")
	})

	t.Run("update maps repository failure", func(t *testing.T) {
		h, tenantRepo, recurringRepo, _ := setupRecurringTestHandlers()
		tenantRepo.addTestTenant("tenant-1", "Recurring Tenant", "recurring-tenant")
		seedRecurringInvoice(recurringRepo, "tenant-1", "rec-1")
		recurringRepo.updateErr = errors.New("update failed")

		req := withURLParams(makeAuthenticatedRequest(http.MethodPut, "/tenants/tenant-1/recurring-invoices/rec-1", map[string]any{
			"name": "Updated",
		}, claims), map[string]string{"tenantID": "tenant-1", "recurringID": "rec-1"})
		rec := httptest.NewRecorder()

		h.UpdateRecurringInvoice(rec, req)

		require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
		assert.Contains(t, rec.Body.String(), "update failed")
	})
}

func TestWave11YearEndHandlerBranches(t *testing.T) {
	t.Run("download archive maps archive build failure", func(t *testing.T) {
		h, _, _, _ := setupWave6YearEndReady(t)
		docRepo := newMockDocumentRepository()
		entityID, err := accounting.YearEndCloseEvidenceEntityID("tenant-1", "2025-12-31")
		require.NoError(t, err)
		docRepo.docs["doc-close-pack"] = &documents.Document{
			ID:           "doc-close-pack",
			TenantID:     "tenant-1",
			EntityType:   documents.EntityTypeYearEndClose,
			EntityID:     entityID,
			DocumentType: documents.DocumentTypeClosePack,
			FileName:     "close-pack.pdf",
			StorageKey:   "missing.pdf",
			ReviewStatus: documents.ReviewStatusApproved,
		}
		h.documentsService = documents.NewService(docRepo, &wave6DocumentStore{openErr: errors.New("object store offline")})

		req := wave6YearEndRequest(http.MethodGet, "/tenants/tenant-1/year-end-close-audit-archive?period_end_date=2025-12-31", nil, "tenant-1")
		rec := httptest.NewRecorder()

		h.DownloadYearEndCloseAuditArchive(rec, req)

		require.Equal(t, http.StatusInternalServerError, rec.Code, rec.Body.String())
		assert.Contains(t, rec.Body.String(), "object store offline")
	})

	t.Run("carry forward maps post-service evidence attachment failure", func(t *testing.T) {
		h, _, _, _ := setupWave6YearEndReady(t)
		docRepo := &wave6ListDocumentsSequenceRepository{
			mockDocumentRepository: newMockDocumentRepository(),
			failOnCall:             2,
			listCallErr:            errors.New("evidence policy store failed"),
		}
		entityID, err := accounting.YearEndCloseEvidenceEntityID("tenant-1", "2025-12-31")
		require.NoError(t, err)
		docRepo.docs["doc-close-pack"] = &documents.Document{
			ID:           "doc-close-pack",
			TenantID:     "tenant-1",
			EntityType:   documents.EntityTypeYearEndClose,
			EntityID:     entityID,
			DocumentType: documents.DocumentTypeClosePack,
			ReviewStatus: documents.ReviewStatusApproved,
		}
		h.documentsService = documents.NewService(docRepo, nil)

		req := wave6YearEndRequest(http.MethodPost, "/tenants/tenant-1/year-end-carry-forward", map[string]any{
			"period_end_date": "2025-12-31",
		}, "tenant-1")
		rec := httptest.NewRecorder()

		h.CreateYearEndCarryForward(rec, req)

		require.Equal(t, http.StatusInternalServerError, rec.Code, rec.Body.String())
		assert.Contains(t, rec.Body.String(), "Failed to evaluate close-pack evidence")
	})

	t.Run("carry forward maps post-service inventory attachment failure", func(t *testing.T) {
		h, _, _, _ := setupWave6YearEndReady(t)
		installApprovedClosePackEvidence(t, h, "tenant-1", "2025-12-31")
		inventoryRepo := &wave11InventoryRepository{
			mockInventoryRepository: newMockInventoryRepository(),
			failOnCall:              2,
			listErr:                 errors.New("inventory list failed"),
		}
		seedWave11InventoryReady(inventoryRepo.mockInventoryRepository, decimal.NewFromInt(6))
		h.inventoryService = inventory.NewServiceWithRepository(inventoryRepo)

		req := wave6YearEndRequest(http.MethodPost, "/tenants/tenant-1/year-end-carry-forward", map[string]any{
			"period_end_date":            "2025-12-31",
			"inventory_valuation_method": "standard-cost",
		}, "tenant-1")
		rec := httptest.NewRecorder()

		h.CreateYearEndCarryForward(rec, req)

		require.Equal(t, http.StatusInternalServerError, rec.Code, rec.Body.String())
		assert.Contains(t, rec.Body.String(), "Failed to process year-end close workflow")
	})

	t.Run("inventory readiness helper maps review error and success", func(t *testing.T) {
		h, _, _, tenantRecord := setupWave6YearEndReady(t)
		inventoryRepo := newMockInventoryRepository()
		inventoryRepo.listProductsErr = errors.New("inventory unavailable")
		h.inventoryService = inventory.NewServiceWithRepository(inventoryRepo)

		err := h.requireYearEndInventoryCostingReady(context.Background(), "tenant_tenant", "tenant-1", tenantRecord.Settings.FiscalYearStart, "2025-12-31", "standard-cost")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "inventory unavailable")

		h, _, _, tenantRecord = setupWave6YearEndReady(t)
		attachYearEndInventoryFixture(h, decimal.NewFromInt(6))
		err = h.requireYearEndInventoryCostingReady(context.Background(), "tenant_tenant", "tenant-1", tenantRecord.Settings.FiscalYearStart, "2025-12-31", "standard-cost")
		require.NoError(t, err)
	})
}

func TestWave11PeriodCloseBranches(t *testing.T) {
	t.Run("authorization maps role lookup failure", func(t *testing.T) {
		h, _ := setupTenantTestHandlers()
		req := withURLParams(makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/period-close", tenant.ClosePeriodRequest{
			PeriodEndDate: "2025-12-31",
			Note:          "Close",
		}, createTestClaims("user-1", "user@example.com", "tenant-1", tenant.RoleOwner)), map[string]string{"tenantID": "tenant-1"})
		rec := httptest.NewRecorder()

		h.ClosePeriod(rec, req)

		require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
		assert.Contains(t, rec.Body.String(), "Access denied")
	})

	t.Run("reopen maps tenant lookup failure", func(t *testing.T) {
		h, repo := setupTenantTestHandlers()
		repo.tenantUsers["tenant-1"] = []tenant.TenantUser{{TenantID: "tenant-1", UserID: "user-1", Role: tenant.RoleOwner}}
		req := withURLParams(makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/period-reopen", tenant.ReopenPeriodRequest{
			PeriodEndDate: "2025-12-31",
			Note:          "Correction",
		}, createTestClaims("user-1", "user@example.com", "tenant-1", tenant.RoleOwner)), map[string]string{"tenantID": "tenant-1"})
		rec := httptest.NewRecorder()

		h.ReopenPeriod(rec, req)

		require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
		assert.Contains(t, rec.Body.String(), "Tenant not found")
	})

	t.Run("reopen maps year-end carry forward status failure", func(t *testing.T) {
		h, repo, accountingRepo := setupTenantAccountingHandlers()
		settings := tenant.DefaultSettings()
		settings.PeriodLockDate = stringPtr("2025-12-31")
		repo.tenants["tenant-1"] = &tenant.Tenant{ID: "tenant-1", Name: "Tenant", Slug: "tenant", SchemaName: "tenant_tenant", Settings: settings}
		repo.tenantUsers["tenant-1"] = []tenant.TenantUser{{TenantID: "tenant-1", UserID: "user-1", Role: tenant.RoleOwner}}
		accountingRepo.periodBalanceErr = errors.New("balances unavailable")

		req := withURLParams(makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/period-reopen", tenant.ReopenPeriodRequest{
			PeriodEndDate: "2025-12-31",
			Note:          "Correction",
		}, createTestClaims("user-1", "user@example.com", "tenant-1", tenant.RoleOwner)), map[string]string{"tenantID": "tenant-1"})
		rec := httptest.NewRecorder()

		h.ReopenPeriod(rec, req)

		require.Equal(t, http.StatusInternalServerError, rec.Code, rec.Body.String())
		assert.Contains(t, rec.Body.String(), "Failed to process year-end close workflow")
	})

	t.Run("respond period close maps no closed period conflict", func(t *testing.T) {
		rec := httptest.NewRecorder()

		respondPeriodCloseError(rec, errors.New("no closed period to reopen"))

		require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
		assert.Contains(t, rec.Body.String(), "no closed period")
	})

	t.Run("respond period close maps period not currently closed conflict", func(t *testing.T) {
		rec := httptest.NewRecorder()

		respondPeriodCloseError(rec, errors.New("period 2025-12-31 is not currently closed"))

		require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
		assert.Contains(t, rec.Body.String(), "not currently closed")
	})
}

func TestWave11TaxHandlerBranches(t *testing.T) {
	t.Run("submitted maps declaration disappearing during status update", func(t *testing.T) {
		h, tenantRepo, taxRepo := setupTaxHandlerTest(t)
		tenantRecord := tenantRepo.addTestTenant("tenant-1", "Tax Tenant", "tax-tenant")
		taxRepo.getDecl = &tax.KMDDeclaration{ID: "decl-1", TenantID: tenantRecord.ID, Year: 2026, Month: 3}
		installApprovedEvidenceDocuments(t, h, documents.Document{
			ID:           "doc-submission",
			EntityType:   documents.EntityTypeKMD,
			EntityID:     "decl-1",
			DocumentType: documents.DocumentTypeTaxSupport,
		})
		taxRepo.statusErr = tax.ErrKMDDeclarationNotFound

		body := invokeTaxHandlerJSON[map[string]string](t, http.StatusNotFound, h.HandleMarkKMDSubmitted, taxHandlerRequest(
			http.MethodPost,
			"/tenants/tenant-1/tax/kmd/2026/3/submit",
			nil,
			map[string]string{"tenantID": "tenant-1", "year": "2026", "month": "3"},
		))

		assert.Equal(t, "Declaration not found", body["error"])
	})

	t.Run("accepted maps generic status update error", func(t *testing.T) {
		h, tenantRepo, taxRepo := setupTaxHandlerTest(t)
		tenantRecord := tenantRepo.addTestTenant("tenant-1", "Tax Tenant", "tax-tenant")
		taxRepo.getDecl = &tax.KMDDeclaration{ID: "decl-1", TenantID: tenantRecord.ID, Year: 2026, Month: 3, Status: tax.KMDStatusSubmitted}
		installApprovedEvidenceDocuments(t, h, documents.Document{
			ID:           "doc-acceptance",
			EntityType:   documents.EntityTypeKMD,
			EntityID:     "decl-1",
			DocumentType: documents.DocumentTypeSupportingDocument,
		})
		taxRepo.statusErr = errors.New("status update failed")

		body := invokeTaxHandlerJSON[map[string]string](t, http.StatusInternalServerError, h.HandleMarkKMDAccepted, taxHandlerRequest(
			http.MethodPost,
			"/tenants/tenant-1/tax/kmd/2026/3/accept",
			nil,
			map[string]string{"tenantID": "tenant-1", "year": "2026", "month": "3"},
		))

		assert.Contains(t, body["error"], "status update failed")
	})

	t.Run("export maps XML generation failure", func(t *testing.T) {
		h, tenantRepo, taxRepo := setupTaxHandlerTest(t)
		tenantRecord := tenantRepo.addTestTenant("tenant-1", "Tax Tenant", "tax-tenant")
		tenantRecord.Settings.RegCode = "12345678"
		taxRepo.getDecl = &tax.KMDDeclaration{ID: "decl-1", TenantID: tenantRecord.ID, Year: 2026, Month: 3}
		original := exportKMDToXML
		exportKMDToXML = func(*tax.KMDDeclaration, string) ([]byte, error) {
			return nil, errors.New("xml encoder failed")
		}
		defer func() { exportKMDToXML = original }()

		body := invokeTaxHandlerJSON[map[string]string](t, http.StatusInternalServerError, h.HandleExportKMD, taxHandlerRequest(
			http.MethodGet,
			"/tenants/tenant-1/tax/kmd/2026/3/xml",
			nil,
			map[string]string{"tenantID": "tenant-1", "year": "2026", "month": "3"},
		))

		assert.Contains(t, body["error"], "xml encoder failed")
	})
}

func TestWave11BusinessAndCoreHandlerBranches(t *testing.T) {
	t.Run("test smtp maps service failure", func(t *testing.T) {
		original := testSMTPWithService
		testSMTPWithService = func(context.Context, *email.Service, string, string) (*email.TestSMTPResponse, error) {
			return nil, errors.New("smtp transport failed")
		}
		defer func() { testSMTPWithService = original }()
		h := &Handlers{}
		req := withURLParams(makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/settings/smtp/test", email.TestSMTPRequest{
			RecipientEmail: "ops@example.com",
		}, createTestClaims("user-1", "user@example.com", "tenant-1", tenant.RoleOwner)), map[string]string{"tenantID": "tenant-1"})
		rec := httptest.NewRecorder()

		h.TestSMTP(rec, req)

		require.Equal(t, http.StatusInternalServerError, rec.Code, rec.Body.String())
		assert.Contains(t, rec.Body.String(), "smtp transport failed")
	})

	t.Run("tenant user status maps second membership lookup failure", func(t *testing.T) {
		repo := &wave11TenantRepository{
			mockTenantRepository: newMockTenantRepository(),
			failGetUserOn:        2,
			getTenantUserErr:     tenant.ErrUserNotInTenant,
		}
		repo.addTestTenant("tenant-1", "Tenant", "tenant")
		repo.tenantUsers["tenant-1"] = []tenant.TenantUser{
			{TenantID: "tenant-1", UserID: "target-1", Role: tenant.RoleViewer, IsActive: true},
		}
		h := &Handlers{tenantService: tenant.NewServiceWithRepository(repo), refreshSessionService: newMockRefreshSessionService()}

		req := withURLParams(makeAuthenticatedRequest(http.MethodPut, "/tenants/tenant-1/users/target-1/status", map[string]any{
			"is_active": true,
		}, createTestClaims("admin-1", "admin@example.com", "tenant-1", tenant.RoleAdmin)), map[string]string{"tenantID": "tenant-1", "userID": "target-1"})
		rec := httptest.NewRecorder()

		h.UpdateTenantUserStatus(rec, req)

		require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
		assert.Contains(t, rec.Body.String(), "User not found in tenant")
	})

	t.Run("tenant user api token revoke maps nil token service", func(t *testing.T) {
		h, tenantRepo, _ := setupTenantUserAPITokenHandlers()
		h.apiTokenService = nil
		tenantRepo.tenantUsers["tenant-1"] = []tenant.TenantUser{
			{TenantID: "tenant-1", UserID: "target-1", Role: tenant.RoleViewer, IsActive: true},
		}

		req := withURLParams(makeAuthenticatedRequest(http.MethodDelete, "/tenants/tenant-1/users/target-1/api-tokens/token-1", nil, createTestClaims("admin-1", "admin@example.com", "tenant-1", tenant.RoleAdmin)), map[string]string{"tenantID": "tenant-1", "userID": "target-1", "tokenID": "token-1"})
		rec := httptest.NewRecorder()

		h.RevokeTenantUserAPIToken(rec, req)

		require.Equal(t, http.StatusInternalServerError, rec.Code, rec.Body.String())
		assert.Contains(t, rec.Body.String(), "API token service unavailable")
	})

	t.Run("tenant user api token revoke rejects non admin role", func(t *testing.T) {
		h, _, _ := setupTenantUserAPITokenHandlers()

		req := withURLParams(makeAuthenticatedRequest(http.MethodDelete, "/tenants/tenant-1/users/target-1/api-tokens/token-1", nil, createTestClaims("viewer-1", "viewer@example.com", "tenant-1", tenant.RoleViewer)), map[string]string{"tenantID": "tenant-1", "userID": "target-1", "tokenID": "token-1"})
		rec := httptest.NewRecorder()

		h.RevokeTenantUserAPIToken(rec, req)

		require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
		assert.Contains(t, rec.Body.String(), "Permission denied")
	})

	t.Run("bulk reminders maps service error", func(t *testing.T) {
		h, tenantRepo, _, reminderRepo, _, _ := setupMiscHandlers()
		tenantRepo.addTestTenant("tenant-1", "Tenant", "tenant")
		_ = reminderRepo
		original := sendBulkRemindersWithService
		sendBulkRemindersWithService = func(context.Context, *invoicing.ReminderService, string, string, *invoicing.SendBulkRemindersRequest, string) (*invoicing.BulkReminderResult, error) {
			return nil, errors.New("bulk reminder transport failed")
		}
		defer func() { sendBulkRemindersWithService = original }()

		req := withURLParams(makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/invoices/reminders/bulk", invoicing.SendBulkRemindersRequest{
			InvoiceIDs: []string{"inv-1"},
		}, nil), map[string]string{"tenantID": "tenant-1"})
		rec := httptest.NewRecorder()

		h.SendBulkPaymentReminders(rec, req)

		require.Equal(t, http.StatusInternalServerError, rec.Code, rec.Body.String())
		assert.Contains(t, rec.Body.String(), "Failed to send bulk payment reminders")
	})

	t.Run("generate journal template maps lock-date validation failure", func(t *testing.T) {
		repo := &wave11TenantRepository{
			mockTenantRepository: newMockTenantRepository(),
			failGetTenantOn:      3,
			getTenantErr:         errors.New("tenant settings unavailable"),
		}
		repo.addTestTenant("tenant-1", "Tenant", "tenant")
		accountingRepo := newMockAccountingRepository()
		accountingRepo.templates["template-1"] = &accounting.JournalEntryTemplate{
			ID:       "template-1",
			TenantID: "tenant-1",
			Name:     "Monthly accrual",
			IsActive: true,
		}
		h := &Handlers{
			tenantService:     tenant.NewServiceWithRepository(repo),
			accountingService: accounting.NewServiceWithRepository(accountingRepo),
		}

		req := withURLParams(makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/journal-entry-templates/template-1/generate", accounting.GenerateJournalEntryTemplateRequest{
			EntryDate: timePtr(time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)),
		}, createTestClaims("user-1", "user@example.com", "tenant-1", tenant.RoleOwner)), map[string]string{"tenantID": "tenant-1", "templateID": "template-1"})
		rec := httptest.NewRecorder()

		h.GenerateJournalEntryTemplate(rec, req)

		require.Equal(t, http.StatusInternalServerError, rec.Code, rec.Body.String())
		assert.Contains(t, rec.Body.String(), "Failed to validate period lock")
	})
}

func TestWave11ExpenseAndMigrationBranches(t *testing.T) {
	t.Run("post expense maps service failure after lock check", func(t *testing.T) {
		expenseRepo := &expenseHandlerRepository{expenses: make(map[string]*expenses.Expense)}
		accountingSvc := &wave11ExpenseAccounting{
			expenseHandlerAccounting: &expenseHandlerAccounting{accounts: map[string]*accounting.Account{
				"expense-account": {ID: "expense-account", Code: "5500", AccountType: accounting.AccountTypeExpense},
				"cash-account":    {ID: "cash-account", Code: "1000", AccountType: accounting.AccountTypeAsset},
			}},
			postErr: errors.New("ledger post failed"),
		}
		tenantRepo := newMockTenantRepository()
		tenantRepo.tenants["tenant-1"] = &tenant.Tenant{ID: "tenant-1", SchemaName: "tenant_test"}
		h := &Handlers{
			tenantService:   tenant.NewServiceWithRepository(tenantRepo),
			expensesService: expenses.NewServiceWithRepository(expenseRepo, accountingSvc, &expenseHandlerEvidence{compliant: true}),
		}
		approvedAt := time.Now().UTC()
		approvedBy := "approver-1"
		expenseRepo.expenses["expense-1"] = &expenses.Expense{
			ID:               "expense-1",
			TenantID:         "tenant-1",
			ExpenseNumber:    "EXP-1",
			ExpenseDate:      time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
			Merchant:         "Office",
			ExpenseAccountID: "expense-account",
			PaymentAccountID: "cash-account",
			Amount:           decimal.NewFromInt(25),
			Currency:         "EUR",
			ExchangeRate:     decimal.NewFromInt(1),
			BaseAmount:       decimal.NewFromInt(25),
			Status:           expenses.StatusApproved,
			ApprovedAt:       &approvedAt,
			ApprovedBy:       &approvedBy,
			CreatedAt:        approvedAt,
			UpdatedAt:        approvedAt,
		}

		req := withURLParams(makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/expenses/expense-1/post", nil, createTestClaims("user-1", "user@example.com", "tenant-1", tenant.RoleOwner)), map[string]string{"tenantID": "tenant-1", "expenseID": "expense-1"})
		rec := httptest.NewRecorder()

		h.PostExpense(rec, req)

		require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
		assert.Contains(t, rec.Body.String(), "ledger post failed")
	})

	t.Run("migration executor dispatches payments import", func(t *testing.T) {
		h, _, tenantRepo := setupPaymentTestHandlers()
		tenantRepo.addTestTenant("tenant-1", "Tenant", "tenant")
		executor := &handlerMigrationStepExecutor{h: h}

		result, err := executor.ExecuteMigrationStep(context.Background(), "tenant-1", "tenant_tenant", "user-1",
			cutover.MigrationExecutionStep{Kind: cutover.KindPayments},
			cutover.BundleFile{
				Kind:       cutover.KindPayments,
				FileName:   "payments.csv",
				CSVContent: "payment_type,payment_date,amount,reference\nRECEIVED,2026-06-01,25,Receipt\n",
			},
			&cutover.ExecuteMigrationRequest{},
		)

		require.NoError(t, err)
		importResult, ok := result.(*payments.ImportPaymentsResult)
		require.True(t, ok)
		assert.Equal(t, 1, importResult.PaymentsCreated)
	})

	t.Run("migration executor dispatches e-invoice import", func(t *testing.T) {
		h, tenantRepo, _, contactsRepo := setupInvoiceImportTestHandlers()
		tenantRepo.addTestTenant("tenant-1", "Tenant", "tenant")
		contact := contactsRepo.addTestContact("supplier-1", "tenant-1", "Supplier OÜ", contacts.ContactTypeSupplier, true)
		contact.RegCode = "12345678"
		contact.VATNumber = "EE12345678"
		executor := &handlerMigrationStepExecutor{h: h}

		result, err := executor.ExecuteMigrationStep(context.Background(), "tenant-1", "tenant_tenant", "user-1",
			cutover.MigrationExecutionStep{Kind: cutover.KindEInvoices},
			cutover.BundleFile{Kind: cutover.KindEInvoices, FileName: "supplier.xml", XMLContent: handlerEInvoiceXML()},
			&cutover.ExecuteMigrationRequest{EInvoiceInvoiceType: "purchase"},
		)

		require.NoError(t, err)
		importResult, ok := result.(*invoicing.ImportInvoicesResult)
		require.True(t, ok)
		assert.Equal(t, 1, importResult.InvoicesCreated)
		assert.Equal(t, 1, importResult.RowsProcessed)
	})
}

func TestWave11PluginInstallSuccess(t *testing.T) {
	h, _ := setupPluginTestHandlers(t)
	t.Setenv("DEMO_MODE", "true")

	req := httptest.NewRequest(http.MethodPost, "/admin/plugins/install", mustJSONReader(t, plugin.InstallPluginRequest{
		RepositoryURL: plugin.DemoInstallFixtureRepositoryURL,
	}))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.InstallPlugin(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var installed plugin.Plugin
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&installed))
	assert.Equal(t, plugin.DemoInstallFixtureRepositoryURL, installed.RepositoryURL)
}

func TestWave11PluginRegistryAndRuntimeBranches(t *testing.T) {
	t.Run("sync registry maps success", func(t *testing.T) {
		h, _ := setupPluginTestHandlers(t)
		registryID := uuid.New()
		original := syncPluginRegistryWithService
		syncPluginRegistryWithService = func(context.Context, *plugin.Service, uuid.UUID) error {
			return nil
		}
		defer func() { syncPluginRegistryWithService = original }()

		req := withURLParams(httptest.NewRequest(http.MethodPost, "/admin/plugin-registries/"+registryID.String()+"/sync", nil), map[string]string{"id": registryID.String()})
		rec := httptest.NewRecorder()

		h.SyncPluginRegistry(rec, req)

		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		assert.Contains(t, rec.Body.String(), `"status":"synced"`)
	})

	t.Run("restart runtime maps unavailable", func(t *testing.T) {
		h, _ := setupPluginTestHandlers(t)
		pluginID := uuid.New()
		original := restartPluginRuntimeWithService
		restartPluginRuntimeWithService = func(context.Context, *plugin.Service, uuid.UUID) (*plugin.PluginRuntimeStatus, error) {
			return nil, plugin.ErrPluginRuntimeUnavailable
		}
		defer func() { restartPluginRuntimeWithService = original }()

		req := withURLParams(httptest.NewRequest(http.MethodPost, "/admin/plugins/"+pluginID.String()+"/runtime/restart", nil), map[string]string{"id": pluginID.String()})
		rec := httptest.NewRecorder()

		h.RestartPluginRuntime(rec, req)

		require.Equal(t, http.StatusBadGateway, rec.Code, rec.Body.String())
		assert.Contains(t, rec.Body.String(), plugin.ErrPluginRuntimeUnavailable.Error())
	})

	t.Run("restart runtime maps success", func(t *testing.T) {
		h, _ := setupPluginTestHandlers(t)
		pluginID := uuid.New()
		original := restartPluginRuntimeWithService
		restartPluginRuntimeWithService = func(context.Context, *plugin.Service, uuid.UUID) (*plugin.PluginRuntimeStatus, error) {
			return &plugin.PluginRuntimeStatus{PluginID: pluginID, State: plugin.RuntimeStateRunning}, nil
		}
		defer func() { restartPluginRuntimeWithService = original }()

		req := withURLParams(httptest.NewRequest(http.MethodPost, "/admin/plugins/"+pluginID.String()+"/runtime/restart", nil), map[string]string{"id": pluginID.String()})
		rec := httptest.NewRecorder()

		h.RestartPluginRuntime(rec, req)

		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		assert.Contains(t, rec.Body.String(), `"state":"running"`)
	})
}

func mustJSONReader(t *testing.T, value any) *strings.Reader {
	t.Helper()
	payload, err := json.Marshal(value)
	require.NoError(t, err)
	return strings.NewReader(string(payload))
}

func timePtr(value time.Time) *time.Time {
	return &value
}
