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

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/HMB-research/open-accounting/internal/assets"
	"github.com/HMB-research/open-accounting/internal/auth"
	"github.com/HMB-research/open-accounting/internal/banking"
	"github.com/HMB-research/open-accounting/internal/documents"
	"github.com/HMB-research/open-accounting/internal/inventory"
	"github.com/HMB-research/open-accounting/internal/invoicing"
	"github.com/HMB-research/open-accounting/internal/orders"
	"github.com/HMB-research/open-accounting/internal/payroll"
	"github.com/HMB-research/open-accounting/internal/quotes"
	"github.com/HMB-research/open-accounting/internal/reports"
	"github.com/HMB-research/open-accounting/internal/tenant"
)

type wave9ListUserTenantsErrorRepository struct {
	*mockTenantRepository
	err error
}

func (r *wave9ListUserTenantsErrorRepository) ListUserTenants(ctx context.Context, userID string) ([]tenant.TenantMembership, error) {
	return nil, r.err
}

type wave9ReleaseStockReservationErrorRepository struct {
	*mockOrdersRepository
	err error
}

func (r *wave9ReleaseStockReservationErrorRepository) ReleaseStockReservation(ctx context.Context, schemaName, tenantID, orderID, productID, warehouseID string, quantity decimal.Decimal, reason, releasedBy string) (*orders.OrderStockReservation, error) {
	return nil, r.err
}

type wave9UpsertStockReservationErrorRepository struct {
	*mockOrdersRepository
	err error
}

func (r *wave9UpsertStockReservationErrorRepository) UpsertStockReservation(ctx context.Context, schemaName string, reservation *orders.OrderStockReservation) error {
	return r.err
}

type wave9SecondStockUpsertErrorRepository struct {
	*mockInventoryRepository
	calls int
	err   error
}

func (r *wave9SecondStockUpsertErrorRepository) UpsertStockLevel(ctx context.Context, schemaName string, level *inventory.StockLevel) error {
	r.calls++
	if r.calls >= 2 {
		return r.err
	}
	return r.mockInventoryRepository.UpsertStockLevel(ctx, schemaName, level)
}

func TestCoreBusinessWave9CoreHandlerBranches(t *testing.T) {
	t.Run("api token cannot consolidate another tenant", func(t *testing.T) {
		h, _ := setupAuthTestHandlers()

		got, err := h.allowedConsolidationTenants(context.Background(), &auth.Claims{
			UserID:    "service-user",
			TenantID:  "tenant-other",
			TokenKind: auth.TokenKindAPIToken,
		}, "tenant-1")

		require.NoError(t, err)
		assert.Empty(t, got)
	})

	t.Run("allowed consolidation tenants returns membership lookup error", func(t *testing.T) {
		repo := &wave9ListUserTenantsErrorRepository{
			mockTenantRepository: newMockTenantRepository(),
			err:                  errors.New("membership store unavailable"),
		}
		h := &Handlers{tenantService: tenant.NewServiceWithRepository(repo)}

		got, err := h.allowedConsolidationTenants(context.Background(), &auth.Claims{
			UserID:   "user-1",
			TenantID: "tenant-1",
			Role:     tenant.RoleOwner,
		}, "tenant-1")

		require.Error(t, err)
		assert.Nil(t, got)
		assert.Contains(t, err.Error(), "membership store unavailable")
	})

	t.Run("cash flow rejects unsupported export format", func(t *testing.T) {
		h, _, _, _, _, _ := setupMiscHandlers()
		req := withURLParams(httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/reports/cash-flow?start_date=2026-01-01&end_date=2026-01-31&format=xml", nil), map[string]string{
			"tenantID": "tenant-1",
		})
		rr := httptest.NewRecorder()

		h.GetCashFlowStatement(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "format must be json")
	})

	t.Run("annual report close pack evidence lookup error", func(t *testing.T) {
		h, tenantRepo, accountingRepo := setupTenantAccountingHandlers()
		settings := tenant.DefaultSettings()
		settings.PeriodLockDate = stringPtr("2025-12-31")
		tenantRepo.tenants["tenant-1"] = &tenant.Tenant{
			ID:         "tenant-1",
			Name:       "Tenant",
			SchemaName: "tenant_test",
			Settings:   settings,
		}
		accountingRepo.periodBalances = annualReportBalancedPeriodBalances()
		h.reportsService = reports.NewServiceWithRepository(reports.NewMockRepository())
		documentRepo := newMockDocumentRepository()
		documentRepo.listDocumentsErr = errors.New("document store unavailable")
		h.documentsService = documents.NewService(documentRepo, nil)

		req := makeAuthenticatedRequest(http.MethodGet, "/tenants/tenant-1/reports/annual?period_end_date=2025-12-31", nil, &auth.Claims{UserID: "user-1"})
		req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
		rr := httptest.NewRecorder()

		h.GetAnnualReport(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
		assert.Contains(t, rr.Body.String(), "Failed to evaluate close-pack evidence")
	})

	t.Run("invoice reminder history normalizes nil repository slice", func(t *testing.T) {
		h, _, _, _, _, _ := setupMiscHandlers()
		req := withURLParams(httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/invoices/missing/reminders", nil), map[string]string{
			"tenantID":  "tenant-1",
			"invoiceID": "missing",
		})
		rr := httptest.NewRecorder()

		h.GetInvoiceReminderHistory(rr, req)

		require.Equal(t, http.StatusOK, rr.Code)
		var reminders []invoicing.PaymentReminder
		require.NoError(t, json.NewDecoder(rr.Body).Decode(&reminders))
		assert.Empty(t, reminders)
	})

	t.Run("get cost center maps not found", func(t *testing.T) {
		h, _, _, _, _, _ := setupMiscHandlers()
		req := withURLParams(httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/cost-centers/missing", nil), map[string]string{
			"tenantID":     "tenant-1",
			"costCenterID": "missing",
		})
		rr := httptest.NewRecorder()

		h.GetCostCenter(rr, req)

		assert.Equal(t, http.StatusNotFound, rr.Code)
		assert.Contains(t, rr.Body.String(), "not found")
	})
}

func TestCoreBusinessWave9BusinessValidationAndEvidenceBranches(t *testing.T) {
	t.Run("create invoice rejects missing lines", func(t *testing.T) {
		h, _, _ := setupInvoiceTestHandlers()
		req := makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/invoices", invoicing.CreateInvoiceRequest{
			ContactID: "contact-1",
		}, createTestClaims("user-1", "test@example.com", "tenant-1", tenant.RoleAdmin))
		req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
		rr := httptest.NewRecorder()

		h.CreateInvoice(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "At least one line is required")
	})

	t.Run("send purchase invoice requires evidence when documents service missing", func(t *testing.T) {
		h, _, invoiceRepo := setupInvoiceTestHandlers()
		invoiceRepo.addTestInvoice("bill-1", "tenant-1", "supplier-1", invoicing.InvoiceTypePurchase, invoicing.StatusDraft)
		req := withURLParams(httptest.NewRequest(http.MethodPost, "/tenants/tenant-1/invoices/bill-1/send", nil), map[string]string{
			"tenantID":  "tenant-1",
			"invoiceID": "bill-1",
		})
		rr := httptest.NewRecorder()

		h.SendInvoice(rr, req)

		assert.Equal(t, http.StatusConflict, rr.Code)
		assert.Contains(t, rr.Body.String(), "approved purchase-invoice evidence")
	})

	t.Run("update email template rejects invalid json", func(t *testing.T) {
		h := &Handlers{}
		req := httptest.NewRequest(http.MethodPut, "/tenants/tenant-1/email-templates/INVOICE_SEND", strings.NewReader("{"))
		req.Header.Set("Content-Type", "application/json")
		req = withURLParams(req, map[string]string{
			"tenantID":     "tenant-1",
			"templateType": "INVOICE_SEND",
		})
		rr := httptest.NewRecorder()

		h.UpdateEmailTemplate(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "Invalid request body")
	})

	t.Run("bank import returns mapper parse error", func(t *testing.T) {
		h, _, _ := setupBankingTestHandlers()
		req := makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/bank-accounts/bank-1/import", banking.ImportCSVRequest{
			CSVContent: "date,amount,description\n2026-01-15,12.34,Deposit",
			Format:     "unsupported-bank",
		}, createTestClaims("user-1", "test@example.com", "tenant-1", tenant.RoleAdmin))
		req = withURLParams(req, map[string]string{
			"tenantID":  "tenant-1",
			"accountID": "bank-1",
		})
		rr := httptest.NewRecorder()

		h.ImportBankTransactions(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "unsupported bank transaction import format")
	})

	t.Run("bank import returns service import record error", func(t *testing.T) {
		h, bankingRepo, _ := setupBankingTestHandlers()
		bankingRepo.accounts["bank-1"] = &banking.BankAccount{
			ID:            "bank-1",
			TenantID:      "tenant-1",
			AccountNumber: "EE123",
			Name:          "Main Bank",
			Currency:      "EUR",
			IsActive:      true,
		}
		bankingRepo.importRecordErr = errors.New("import record failed")
		req := makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/bank-accounts/bank-1/import", banking.ImportCSVRequest{
			Transactions: []banking.CSVTransactionRow{{
				Date:        "2026-01-15",
				Amount:      "12.34",
				Currency:    "EUR",
				Description: "Deposit",
			}},
		}, createTestClaims("user-1", "test@example.com", "tenant-1", tenant.RoleAdmin))
		req = withURLParams(req, map[string]string{
			"tenantID":  "tenant-1",
			"accountID": "bank-1",
		})
		rr := httptest.NewRecorder()

		h.ImportBankTransactions(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "record import")
		assert.Contains(t, rr.Body.String(), "import record failed")
	})

	t.Run("review bank transaction rejects invalid json", func(t *testing.T) {
		h, _, _ := setupBankingTestHandlers()
		req := httptest.NewRequest(http.MethodPost, "/tenants/tenant-1/bank-transactions/tx-1/review", strings.NewReader("{"))
		req.Header.Set("Content-Type", "application/json")
		req = withURLParams(req, map[string]string{
			"tenantID":      "tenant-1",
			"transactionID": "tx-1",
		})
		req = req.WithContext(contextWithClaims(req.Context(), createTestClaims("user-1", "test@example.com", "tenant-1", tenant.RoleAdmin)))
		rr := httptest.NewRecorder()

		h.ReviewBankTransaction(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "Invalid request body")
	})

	t.Run("create payment from transaction rejects locked period", func(t *testing.T) {
		h, bankingRepo, tenantRepo := setupBankingTestHandlers()
		settings := tenant.DefaultSettings()
		settings.PeriodLockDate = stringPtr("2026-01-31")
		tenantRepo.tenants["tenant-1"] = &tenant.Tenant{
			ID:         "tenant-1",
			Name:       "Tenant",
			SchemaName: "tenant_test",
			Settings:   settings,
		}
		bankingRepo.transactions["tx-1"] = &banking.BankTransaction{
			ID:              "tx-1",
			TenantID:        "tenant-1",
			BankAccountID:   "bank-1",
			TransactionDate: time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC),
			Amount:          decimal.NewFromInt(25),
			Status:          banking.StatusUnmatched,
		}
		req := makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/bank-transactions/tx-1/create-payment", nil, createTestClaims("user-1", "test@example.com", "tenant-1", tenant.RoleAdmin))
		req = withURLParams(req, map[string]string{
			"tenantID":      "tenant-1",
			"transactionID": "tx-1",
		})
		rr := httptest.NewRecorder()

		h.CreatePaymentFromTransaction(rr, req)

		assert.Equal(t, http.StatusConflict, rr.Code)
		assert.Contains(t, rr.Body.String(), "period locked through")
	})

	t.Run("auto match surfaces repository list error", func(t *testing.T) {
		h, bankingRepo, _ := setupBankingTestHandlers()
		bankingRepo.listTxErr = errors.New("transactions unavailable")
		req := withURLParams(httptest.NewRequest(http.MethodPost, "/tenants/tenant-1/bank-accounts/bank-1/auto-match", nil), map[string]string{
			"tenantID":  "tenant-1",
			"accountID": "bank-1",
		})
		rr := httptest.NewRecorder()

		h.AutoMatchTransactions(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
		assert.Contains(t, rr.Body.String(), "Failed to auto-match transactions")
	})

	t.Run("reconciliation evidence returns document service error", func(t *testing.T) {
		h, bankingRepo, _ := setupBankingTestHandlers()
		reconciliationID := "rec-1"
		bankingRepo.transactions["tx-1"] = &banking.BankTransaction{
			ID:               "tx-1",
			TenantID:         "tenant-1",
			BankAccountID:    "bank-1",
			TransactionDate:  time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC),
			Amount:           decimal.NewFromInt(25),
			Status:           banking.StatusMatched,
			FollowUpStatus:   banking.FollowUpEvidenceRequired,
			ReconciliationID: &reconciliationID,
		}
		documentRepo := newMockDocumentRepository()
		documentRepo.listDocumentsErr = errors.New("document lookup failed")
		h.documentsService = documents.NewService(documentRepo, nil)
		req := withURLParams(httptest.NewRequest(http.MethodPost, "/tenants/tenant-1/reconciliations/rec-1/complete", nil), map[string]string{
			"tenantID":         "tenant-1",
			"reconciliationID": reconciliationID,
		})
		rr := httptest.NewRecorder()

		h.CompleteReconciliation(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
		assert.Contains(t, rr.Body.String(), "document lookup failed")
	})
}

func TestCoreBusinessWave9TenantAdminBranches(t *testing.T) {
	t.Run("tenant user admin handlers return after authorization failure", func(t *testing.T) {
		handlers := []struct {
			name    string
			handler func(*Handlers, http.ResponseWriter, *http.Request)
			method  string
			path    string
			body    any
			params  map[string]string
		}{
			{
				name:    "security events",
				handler: hListTenantUserSecurityAuditEvents,
				method:  http.MethodGet,
				path:    "/tenants/tenant-1/users/user-2/security-events",
				params:  map[string]string{"tenantID": "tenant-1", "userID": "user-2"},
			},
			{
				name:    "status",
				handler: hUpdateTenantUserStatus,
				method:  http.MethodPut,
				path:    "/tenants/tenant-1/users/user-2/status",
				body:    map[string]bool{"is_active": true},
				params:  map[string]string{"tenantID": "tenant-1", "userID": "user-2"},
			},
			{
				name:    "single session revoke",
				handler: hRevokeTenantUserAuthSession,
				method:  http.MethodDelete,
				path:    "/tenants/tenant-1/users/user-2/sessions/session-1",
				params:  map[string]string{"tenantID": "tenant-1", "userID": "user-2", "sessionID": "session-1"},
			},
			{
				name:    "all sessions revoke",
				handler: hRevokeTenantUserAuthSessions,
				method:  http.MethodDelete,
				path:    "/tenants/tenant-1/users/user-2/sessions",
				params:  map[string]string{"tenantID": "tenant-1", "userID": "user-2"},
			},
		}

		for _, tt := range handlers {
			t.Run(tt.name, func(t *testing.T) {
				h, repo := setupWave5TenantHandlers()
				wave5SeedTenantUsers(repo)
				claims := createTestClaims("viewer-1", "viewer@example.com", "tenant-1", tenant.RoleViewer)
				req := makeAuthenticatedRequest(tt.method, tt.path, tt.body, claims)
				req = withURLParams(req, tt.params)
				rr := httptest.NewRecorder()

				tt.handler(h, rr, req)

				assert.Equal(t, http.StatusForbidden, rr.Code)
				assert.Contains(t, rr.Body.String(), "Permission denied")
			})
		}
	})

	t.Run("single session revoke returns tenant audit failure", func(t *testing.T) {
		h, repo := setupWave5TenantHandlers()
		wave5SeedTenantUsers(repo)
		h.refreshSessionService.(*mockRefreshSessionService).sessions["session-1"] = mockRefreshSession{
			userID:    "user-2",
			expiresAt: time.Now().Add(time.Hour),
		}
		repo.createAuditEventErr = errors.New("audit write failed")
		req := makeAuthenticatedRequest(http.MethodDelete, "/tenants/tenant-1/users/user-2/sessions/session-1", nil, wave5AdminClaims())
		req = withURLParams(req, map[string]string{"tenantID": "tenant-1", "userID": "user-2", "sessionID": "session-1"})
		rr := httptest.NewRecorder()

		h.RevokeTenantUserAuthSession(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
		assert.Contains(t, rr.Body.String(), "Failed to record tenant audit event")
	})

	t.Run("role update returns tenant audit failure", func(t *testing.T) {
		h, repo := setupWave5TenantHandlers()
		wave5SeedTenantUsers(repo)
		repo.createAuditEventErr = errors.New("audit write failed")
		req := makeAuthenticatedRequest(http.MethodPut, "/tenants/tenant-1/users/user-2/role", map[string]string{
			"role": tenant.RoleAdmin,
		}, wave5AdminClaims())
		req = withURLParams(req, map[string]string{"tenantID": "tenant-1", "userID": "user-2"})
		rr := httptest.NewRecorder()

		h.UpdateTenantUserRole(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
		assert.Contains(t, rr.Body.String(), "Failed to record tenant audit event")
	})

	t.Run("create invitation returns tenant audit failure", func(t *testing.T) {
		h, repo := setupWave5TenantHandlers()
		wave5SeedTenantUsers(repo)
		repo.createAuditEventErr = errors.New("audit write failed")
		req := makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/invitations", map[string]string{
			"email": "new@example.com",
			"role":  tenant.RoleViewer,
		}, wave5AdminClaims())
		req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
		rr := httptest.NewRecorder()

		h.CreateInvitation(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
		assert.Contains(t, rr.Body.String(), "Failed to record tenant audit event")
	})
}

func hListTenantUserSecurityAuditEvents(h *Handlers, w http.ResponseWriter, r *http.Request) {
	h.ListTenantUserSecurityAuditEvents(w, r)
}

func hUpdateTenantUserStatus(h *Handlers, w http.ResponseWriter, r *http.Request) {
	h.UpdateTenantUserStatus(w, r)
}

func hRevokeTenantUserAuthSession(h *Handlers, w http.ResponseWriter, r *http.Request) {
	h.RevokeTenantUserAuthSession(w, r)
}

func hRevokeTenantUserAuthSessions(h *Handlers, w http.ResponseWriter, r *http.Request) {
	h.RevokeTenantUserAuthSessions(w, r)
}

func TestCoreBusinessWave9OrderStockBranches(t *testing.T) {
	t.Run("check order stock rejects missing inventory service", func(t *testing.T) {
		h, _, _ := setupOrdersTestHandlers()
		req := withURLParams(httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/orders/order-1/stock-check", nil), map[string]string{
			"tenantID": "tenant-1",
			"orderID":  "order-1",
		})
		rr := httptest.NewRecorder()

		h.CheckOrderStock(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
		assert.Contains(t, rr.Body.String(), "Inventory service unavailable")
	})

	t.Run("pick list rejects unknown warehouse", func(t *testing.T) {
		h, _, _ := setupOrdersTestHandlers()
		h.inventoryService = inventory.NewServiceWithRepository(newMockInventoryRepository())
		req := withURLParams(httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/orders/order-1/pick-list?warehouse_id=missing", nil), map[string]string{
			"tenantID": "tenant-1",
			"orderID":  "order-1",
		})
		rr := httptest.NewRecorder()

		h.GetOrderPickList(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "Warehouse not found")
	})

	t.Run("pick list maps missing order after warehouse validation", func(t *testing.T) {
		h, _, _ := setupOrdersTestHandlers()
		inventoryRepo := newMockInventoryRepository()
		inventoryRepo.warehouses["wh-1"] = wave9Warehouse("wh-1")
		h.inventoryService = inventory.NewServiceWithRepository(inventoryRepo)
		req := withURLParams(httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/orders/missing/pick-list?warehouse_id=wh-1", nil), map[string]string{
			"tenantID": "tenant-1",
			"orderID":  "missing",
		})
		rr := httptest.NewRecorder()

		h.GetOrderPickList(rr, req)

		assert.Equal(t, http.StatusNotFound, rr.Code)
		assert.Contains(t, rr.Body.String(), "Order not found")
	})

	t.Run("build pick list returns stock-level load error", func(t *testing.T) {
		h, ordersRepo, _ := setupOrdersTestHandlers()
		order := wave9TrackedOrder()
		ordersRepo.orders[order.ID] = order
		inventoryRepo := newMockInventoryRepository()
		inventoryRepo.products[apiInventoryStockProductID] = wave9TrackedProduct(apiInventoryStockProductID)
		inventoryRepo.warehouses["wh-1"] = wave9Warehouse("wh-1")
		inventoryRepo.getStockErr = errors.New("stock levels unavailable")
		h.inventoryService = inventory.NewServiceWithRepository(inventoryRepo)

		got, err := h.buildOrderPickList(context.Background(), "tenant-1", "tenant_test", "order-1", "wh-1")

		require.Error(t, err)
		assert.Nil(t, got)
		assert.Contains(t, err.Error(), "stock levels unavailable")
	})

	t.Run("build pick list marks unreserved tracked line", func(t *testing.T) {
		h, ordersRepo, _ := setupOrdersTestHandlers()
		order := wave9TrackedOrder()
		ordersRepo.orders[order.ID] = order
		ordersRepo.stockReservations[orderStockReservationKey("order-1", apiInventoryStockProductID, "other-warehouse")] = &orders.OrderStockReservation{
			TenantID:    "tenant-1",
			OrderID:     "order-1",
			ProductID:   apiInventoryStockProductID,
			WarehouseID: "other-warehouse",
			Quantity:    decimal.NewFromInt(2),
			Status:      orders.OrderStockReservationStatusReleased,
		}
		inventoryRepo := newMockInventoryRepository()
		inventoryRepo.products[apiInventoryStockProductID] = wave9TrackedProduct(apiInventoryStockProductID)
		inventoryRepo.warehouses["wh-1"] = wave9Warehouse("wh-1")
		inventoryRepo.stockLevels[apiInventoryStockLevelKey(apiInventoryStockProductID, "wh-1")] = wave9StockLevel(apiInventoryStockProductID, "wh-1", 5, 0, 5)
		h.inventoryService = inventory.NewServiceWithRepository(inventoryRepo)

		pickList, err := h.buildOrderPickList(context.Background(), "tenant-1", "tenant_test", "order-1", "wh-1")

		require.NoError(t, err)
		require.Len(t, pickList.Lines, 1)
		assert.Equal(t, orders.OrderPickListLineStatusUnreserved, pickList.Lines[0].Status)
		assert.True(t, pickList.Lines[0].ShortageQty.Equal(decimal.NewFromInt(2)))
		assert.False(t, pickList.Ready)
	})

	t.Run("reserve stock rejects missing warehouse id", func(t *testing.T) {
		h, _, _ := setupOrdersTestHandlers()
		h.inventoryService = inventory.NewServiceWithRepository(newMockInventoryRepository())
		req := makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/orders/order-1/reserve-stock", orders.OrderStockReservationRequest{}, createTestClaims("user-1", "test@example.com", "tenant-1", tenant.RoleAdmin))
		req = withURLParams(req, map[string]string{
			"tenantID": "tenant-1",
			"orderID":  "order-1",
		})
		rr := httptest.NewRecorder()

		h.ReserveOrderStock(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "warehouse_id is required")
	})

	t.Run("reserve stock maps order stock check error", func(t *testing.T) {
		h, ordersRepo, _ := setupOrdersTestHandlers()
		ordersRepo.getErr = errors.New("order store unavailable")
		inventoryRepo := newMockInventoryRepository()
		inventoryRepo.warehouses["wh-1"] = wave9Warehouse("wh-1")
		h.inventoryService = inventory.NewServiceWithRepository(inventoryRepo)
		req := makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/orders/order-1/reserve-stock", orders.OrderStockReservationRequest{
			WarehouseID: "wh-1",
		}, createTestClaims("user-1", "test@example.com", "tenant-1", tenant.RoleAdmin))
		req = withURLParams(req, map[string]string{
			"tenantID": "tenant-1",
			"orderID":  "order-1",
		})
		rr := httptest.NewRecorder()

		h.ReserveOrderStock(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
		assert.Contains(t, rr.Body.String(), "Failed to check order stock")
	})

	t.Run("apply release returns missing order reservation", func(t *testing.T) {
		h, _, _ := setupOrdersTestHandlers()
		h.inventoryService = inventory.NewServiceWithRepository(newMockInventoryRepository())
		check := wave9OrderStockCheck(orders.OrderStockLineStatusAvailable)

		got, err := h.applyOrderStockReservation(context.Background(), "tenant-1", "tenant_test", orders.OrderStockReservationActionRelease, "", "user-1", check)

		require.Error(t, err)
		assert.Nil(t, got)
		assert.ErrorIs(t, err, orders.ErrOrderStockReservationNotFound)
	})

	t.Run("apply reserve returns inventory reservation error", func(t *testing.T) {
		h, _, _ := setupOrdersTestHandlers()
		inventoryRepo := newMockInventoryRepository()
		inventoryRepo.products[apiInventoryStockProductID] = wave9TrackedProduct(apiInventoryStockProductID)
		inventoryRepo.warehouses[apiInventoryStockWarehouseID] = wave9Warehouse(apiInventoryStockWarehouseID)
		inventoryRepo.stockLevels[apiInventoryStockLevelKey(apiInventoryStockProductID, apiInventoryStockWarehouseID)] = wave9StockLevel(apiInventoryStockProductID, apiInventoryStockWarehouseID, 5, 0, 5)
		inventoryRepo.upsertStockErr = errors.New("stock update failed")
		h.inventoryService = inventory.NewServiceWithRepository(inventoryRepo)
		check := wave9OrderStockCheck(orders.OrderStockLineStatusAvailable)

		got, err := h.applyOrderStockReservation(context.Background(), "tenant-1", "tenant_test", orders.OrderStockReservationActionReserve, "", "user-1", check)

		require.Error(t, err)
		assert.Nil(t, got)
		assert.Contains(t, err.Error(), "stock update failed")
	})

	t.Run("apply release returns order reservation persistence error", func(t *testing.T) {
		baseRepo := newMockOrdersRepository()
		baseRepo.stockReservations[orderStockReservationKey("order-1", apiInventoryStockProductID, apiInventoryStockWarehouseID)] = &orders.OrderStockReservation{
			TenantID:    "tenant-1",
			OrderID:     "order-1",
			ProductID:   apiInventoryStockProductID,
			WarehouseID: apiInventoryStockWarehouseID,
			Quantity:    decimal.NewFromInt(2),
			Status:      orders.OrderStockReservationStatusReserved,
		}
		ordersRepo := &wave9ReleaseStockReservationErrorRepository{
			mockOrdersRepository: baseRepo,
			err:                  errors.New("reservation write failed"),
		}
		h := &Handlers{ordersService: orders.NewServiceWithRepository(ordersRepo)}
		inventoryRepo := newMockInventoryRepository()
		inventoryRepo.products[apiInventoryStockProductID] = wave9TrackedProduct(apiInventoryStockProductID)
		inventoryRepo.warehouses[apiInventoryStockWarehouseID] = wave9Warehouse(apiInventoryStockWarehouseID)
		inventoryRepo.stockLevels[apiInventoryStockLevelKey(apiInventoryStockProductID, apiInventoryStockWarehouseID)] = wave9StockLevel(apiInventoryStockProductID, apiInventoryStockWarehouseID, 5, 2, 3)
		h.inventoryService = inventory.NewServiceWithRepository(inventoryRepo)
		check := wave9OrderStockCheck(orders.OrderStockLineStatusAvailable)

		got, err := h.applyOrderStockReservation(context.Background(), "tenant-1", "tenant_test", orders.OrderStockReservationActionRelease, "", "user-1", check)

		require.Error(t, err)
		assert.Nil(t, got)
		assert.Contains(t, err.Error(), "reservation write failed")
	})

	t.Run("apply reserve reports rollback release failure", func(t *testing.T) {
		baseOrdersRepo := newMockOrdersRepository()
		ordersRepo := &wave9UpsertStockReservationErrorRepository{
			mockOrdersRepository: baseOrdersRepo,
			err:                  errors.New("reservation write failed"),
		}
		h := &Handlers{ordersService: orders.NewServiceWithRepository(ordersRepo)}
		baseInventoryRepo := newMockInventoryRepository()
		baseInventoryRepo.products[apiInventoryStockProductID] = wave9TrackedProduct(apiInventoryStockProductID)
		baseInventoryRepo.warehouses[apiInventoryStockWarehouseID] = wave9Warehouse(apiInventoryStockWarehouseID)
		baseInventoryRepo.stockLevels[apiInventoryStockLevelKey(apiInventoryStockProductID, apiInventoryStockWarehouseID)] = wave9StockLevel(apiInventoryStockProductID, apiInventoryStockWarehouseID, 5, 0, 5)
		inventoryRepo := &wave9SecondStockUpsertErrorRepository{
			mockInventoryRepository: baseInventoryRepo,
			err:                     errors.New("rollback stock update failed"),
		}
		h.inventoryService = inventory.NewServiceWithRepository(inventoryRepo)
		check := wave9OrderStockCheck(orders.OrderStockLineStatusAvailable)

		got, err := h.applyOrderStockReservation(context.Background(), "tenant-1", "tenant_test", orders.OrderStockReservationActionReserve, "", "user-1", check)

		require.Error(t, err)
		assert.Nil(t, got)
		assert.Contains(t, err.Error(), "reservation write failed")
		assert.Contains(t, err.Error(), "rollback release failed")
		assert.Contains(t, err.Error(), "rollback stock update failed")
	})
}

func TestCoreBusinessWave9CommercialEvidenceBranches(t *testing.T) {
	t.Run("send quote maps missing document service to conflict", func(t *testing.T) {
		h, repo, tenantRepo := setupQuotesTestHandlers()
		tenantRepo.tenants["tenant-1"] = &tenant.Tenant{ID: "tenant-1", SchemaName: "tenant_test"}
		repo.quotes["quote-1"] = &quotes.Quote{
			ID:          "quote-1",
			TenantID:    "tenant-1",
			QuoteNumber: "QT-001",
			ContactID:   "contact-1",
			QuoteDate:   time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC),
			Status:      quotes.QuoteStatusDraft,
			Lines: []quotes.QuoteLine{{
				ID:          "line-1",
				Description: "Consulting",
				Quantity:    decimal.NewFromInt(1),
				UnitPrice:   decimal.NewFromInt(100),
			}},
		}
		req := makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/quotes/quote-1/send", map[string]bool{
			"require_approved_evidence": true,
		}, createTestClaims("user-1", "test@example.com", "tenant-1", tenant.RoleAdmin))
		req = withURLParams(req, map[string]string{"tenantID": "tenant-1", "quoteID": "quote-1"})
		rr := httptest.NewRecorder()

		h.SendQuote(rr, req)

		assert.Equal(t, http.StatusConflict, rr.Code)
		assert.Contains(t, rr.Body.String(), "approved quote evidence")
	})

	t.Run("confirm order maps missing document service to conflict", func(t *testing.T) {
		h, repo, tenantRepo := setupOrdersTestHandlers()
		tenantRepo.tenants["tenant-1"] = &tenant.Tenant{ID: "tenant-1", SchemaName: "tenant_test"}
		repo.orders["order-1"] = &orders.Order{
			ID:          "order-1",
			TenantID:    "tenant-1",
			OrderNumber: "ORD-001",
			ContactID:   "contact-1",
			Status:      orders.OrderStatusPending,
			Lines:       []orders.OrderLine{},
		}
		req := makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/orders/order-1/confirm", map[string]bool{
			"require_approved_evidence": true,
		}, createTestClaims("user-1", "test@example.com", "tenant-1", tenant.RoleAdmin))
		req = withURLParams(req, map[string]string{"tenantID": "tenant-1", "orderID": "order-1"})
		rr := httptest.NewRecorder()

		h.ConfirmOrder(rr, req)

		assert.Equal(t, http.StatusConflict, rr.Code)
		assert.Contains(t, rr.Body.String(), "approved order evidence")
	})

	t.Run("convert order to invoice rejects locked issue date", func(t *testing.T) {
		h, _, tenantRepo := setupOrdersTestHandlers()
		settings := tenant.DefaultSettings()
		settings.PeriodLockDate = stringPtr("2026-01-31")
		tenantRepo.tenants["tenant-1"] = &tenant.Tenant{
			ID:         "tenant-1",
			SchemaName: "tenant_test",
			Settings:   settings,
		}
		req := makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/orders/order-1/convert-to-invoice", orders.ConvertOrderToInvoiceRequest{
			IssueDate: time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC),
		}, createTestClaims("user-1", "test@example.com", "tenant-1", tenant.RoleAdmin))
		req = withURLParams(req, map[string]string{"tenantID": "tenant-1", "orderID": "order-1"})
		rr := httptest.NewRecorder()

		h.ConvertOrderToInvoice(rr, req)

		assert.Equal(t, http.StatusConflict, rr.Code)
		assert.Contains(t, rr.Body.String(), "period locked through")
	})
}

func TestCoreBusinessWave9AssetEvidenceBranches(t *testing.T) {
	t.Run("activation evidence is skipped for non-draft asset", func(t *testing.T) {
		h, repo, _ := setupAssetsTestHandlers()
		repo.assets["asset-1"] = wave9Asset("asset-1", assets.AssetStatusActive)

		err := h.requireApprovedAssetActivationEvidence(context.Background(), "tenant_test", "tenant-1", "asset-1")

		require.NoError(t, err)
	})

	t.Run("activation evidence returns document-service error", func(t *testing.T) {
		h, repo, _ := setupAssetsTestHandlers()
		repo.assets["asset-1"] = wave9Asset("asset-1", assets.AssetStatusDraft)
		documentRepo := newMockDocumentRepository()
		documentRepo.listDocumentsErr = errors.New("document search failed")
		h.documentsService = documents.NewService(documentRepo, nil)

		err := h.requireApprovedAssetActivationEvidence(context.Background(), "tenant_test", "tenant-1", "asset-1")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "evaluate asset evidence")
		assert.Contains(t, err.Error(), "document search failed")
	})

	t.Run("disposal evidence is skipped for non-active asset", func(t *testing.T) {
		h, repo, _ := setupAssetsTestHandlers()
		repo.assets["asset-1"] = wave9Asset("asset-1", assets.AssetStatusDraft)

		err := h.requireApprovedAssetDisposalEvidence(context.Background(), "tenant_test", "tenant-1", "asset-1")

		require.NoError(t, err)
	})

	t.Run("disposal evidence returns document-service error", func(t *testing.T) {
		h, repo, _ := setupAssetsTestHandlers()
		repo.assets["asset-1"] = wave9Asset("asset-1", assets.AssetStatusActive)
		documentRepo := newMockDocumentRepository()
		documentRepo.listDocumentsErr = errors.New("document search failed")
		h.documentsService = documents.NewService(documentRepo, nil)

		err := h.requireApprovedAssetDisposalEvidence(context.Background(), "tenant_test", "tenant-1", "asset-1")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "evaluate asset disposal evidence")
		assert.Contains(t, err.Error(), "document search failed")
	})
}

func TestCoreBusinessWave9PayrollPDFTenantError(t *testing.T) {
	h, payrollRepo, _ := setupPayrollImportHandlerTest(t)
	payrollRepo.payrollRuns["run-1"] = &payroll.PayrollRun{
		ID:          "run-1",
		TenantID:    "tenant-1",
		PeriodYear:  2026,
		PeriodMonth: 5,
	}
	payrollRepo.payslips = []payroll.Payslip{{
		ID:           "payslip-1",
		TenantID:     "tenant-1",
		PayrollRunID: "run-1",
		EmployeeID:   "employee-1",
	}}
	tenantRepo := newMockTenantRepository()
	tenantRepo.getTenantErr = errors.New("tenant store unavailable")
	h.tenantService = tenant.NewServiceWithRepository(tenantRepo)
	req := payrollHandlerRequest(http.MethodGet, "/tenants/tenant-1/payroll-runs/run-1/payslips/payslip-1/pdf", nil, map[string]string{
		"tenantID":  "tenant-1",
		"runID":     "run-1",
		"payslipID": "payslip-1",
	})
	rr := httptest.NewRecorder()

	h.GetPayslipPDF(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	assert.Contains(t, rr.Body.String(), "Failed to get tenant")
}

func wave9TrackedOrder() *orders.Order {
	productID := apiInventoryStockProductID
	order := &orders.Order{
		ID:          "order-1",
		TenantID:    "tenant-1",
		OrderNumber: "ORD-001",
		ContactID:   "contact-1",
		Status:      orders.OrderStatusConfirmed,
		Currency:    "EUR",
		Lines: []orders.OrderLine{{
			ID:          "line-1",
			TenantID:    "tenant-1",
			OrderID:     "order-1",
			LineNumber:  1,
			Description: "Tracked item",
			ProductID:   &productID,
			Quantity:    decimal.NewFromInt(2),
			Unit:        "pcs",
			UnitPrice:   decimal.NewFromInt(10),
			VATRate:     decimal.NewFromInt(22),
		}},
	}
	order.Calculate()
	return order
}

func wave9TrackedProduct(productID string) *inventory.Product {
	return &inventory.Product{
		ID:             productID,
		TenantID:       "tenant-1",
		Code:           "SKU-1",
		Name:           "Tracked item",
		ProductType:    inventory.ProductTypeGoods,
		TrackInventory: true,
		IsActive:       true,
	}
}

func wave9Warehouse(warehouseID string) *inventory.Warehouse {
	return &inventory.Warehouse{
		ID:       warehouseID,
		TenantID: "tenant-1",
		Code:     "MAIN",
		Name:     "Main warehouse",
		IsActive: true,
	}
}

func wave9StockLevel(productID, warehouseID string, quantity, reserved, available int64) *inventory.StockLevel {
	return &inventory.StockLevel{
		ID:           "stock-" + productID + "-" + warehouseID,
		TenantID:     "tenant-1",
		ProductID:    productID,
		WarehouseID:  warehouseID,
		Quantity:     decimal.NewFromInt(quantity),
		ReservedQty:  decimal.NewFromInt(reserved),
		AvailableQty: decimal.NewFromInt(available),
	}
}

func wave9OrderStockCheck(status string) *orders.OrderStockCheck {
	return &orders.OrderStockCheck{
		OrderID:     "order-1",
		OrderNumber: "ORD-001",
		WarehouseID: apiInventoryStockWarehouseID,
		Ready:       true,
		Lines: []orders.OrderStockCheckLine{{
			LineID:      "line-1",
			LineNumber:  1,
			ProductID:   apiInventoryStockProductID,
			ProductCode: "SKU-1",
			ProductName: "Tracked item",
			RequiredQty: decimal.NewFromInt(2),
			Status:      status,
		}},
	}
}

func wave9Asset(assetID string, status assets.AssetStatus) *assets.FixedAsset {
	return &assets.FixedAsset{
		ID:           assetID,
		TenantID:     "tenant-1",
		AssetNumber:  "FA-001",
		Name:         "Laptop",
		Status:       status,
		PurchaseDate: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		PurchaseCost: decimal.NewFromInt(1000),
		BookValue:    decimal.NewFromInt(1000),
	}
}
