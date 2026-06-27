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
	"github.com/HMB-research/open-accounting/internal/documents"
	"github.com/HMB-research/open-accounting/internal/inventory"
	"github.com/HMB-research/open-accounting/internal/invoicing"
	"github.com/HMB-research/open-accounting/internal/orders"
	"github.com/HMB-research/open-accounting/internal/payments"
	"github.com/HMB-research/open-accounting/internal/quotes"
	"github.com/HMB-research/open-accounting/internal/tenant"
)

func wave7BusinessRequest(method, path string, body any, params map[string]string) *http.Request {
	req := makeAuthenticatedRequest(method, path, body, createTestClaims("user-1", "test@example.com", "tenant-1", "owner"))
	return withURLParams(req, params)
}

func wave7BusinessRawRequest(method, path, body string, params map[string]string) *http.Request {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = withURLParams(req, params)
	return req.WithContext(contextWithClaims(req.Context(), createTestClaims("user-1", "test@example.com", "tenant-1", "owner")))
}

func wave7AddTenant(repo *mockTenantRepository) *tenant.Tenant {
	return repo.addTestTenant("tenant-1", "Wave 7 Tenant", "wave-7-tenant")
}

func wave7InvoiceLine() invoicing.CreateInvoiceLineRequest {
	return invoicing.CreateInvoiceLineRequest{
		Description: "Consulting",
		Quantity:    decimal.NewFromInt(1),
		UnitPrice:   decimal.NewFromInt(100),
		VATRate:     decimal.NewFromInt(22),
	}
}

func TestBusinessWave7InvoiceAndPaymentLockImportBranches(t *testing.T) {
	t.Run("create invoice rejects locked issue date", func(t *testing.T) {
		h, tenantRepo, _ := setupInvoiceTestHandlers()
		tenantRecord := wave7AddTenant(tenantRepo)
		lockDate := "2026-02-01"
		tenantRecord.Settings.PeriodLockDate = &lockDate

		rr := httptest.NewRecorder()
		h.CreateInvoice(rr, wave7BusinessRequest(http.MethodPost, "/tenants/tenant-1/invoices", invoicing.CreateInvoiceRequest{
			InvoiceType: invoicing.InvoiceTypeSales,
			ContactID:   "contact-1",
			IssueDate:   time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC),
			DueDate:     time.Date(2026, 2, 15, 0, 0, 0, 0, time.UTC),
			Currency:    "EUR",
			Lines:       []invoicing.CreateInvoiceLineRequest{wave7InvoiceLine()},
		}, map[string]string{"tenantID": "tenant-1"}))

		assert.Equal(t, http.StatusConflict, rr.Code, rr.Body.String())
		assert.Contains(t, rr.Body.String(), "period locked through 2026-02-01")
	})

	t.Run("import invoices handles product dependency and importer errors", func(t *testing.T) {
		h, tenantRepo, _, _ := setupInvoiceImportTestHandlers()
		wave7AddTenant(tenantRepo)
		inventoryRepo := newMockInventoryRepository()
		inventoryRepo.listProductsErr = errors.New("products unavailable")
		h.inventoryService = inventory.NewServiceWithRepository(inventoryRepo)

		rr := httptest.NewRecorder()
		h.ImportInvoices(rr, wave7BusinessRequest(http.MethodPost, "/tenants/tenant-1/invoices/import", invoicing.ImportInvoicesRequest{
			CSVContent: "invoice_number,invoice_type,contact_name,issue_date,due_date,line_description,quantity,unit_price,vat_rate\nINV-1,SALES,Acme,2026-02-02,2026-02-16,Work,1,100,22\n",
		}, map[string]string{"tenantID": "tenant-1"}))

		assert.Equal(t, http.StatusInternalServerError, rr.Code, rr.Body.String())
		assert.Contains(t, rr.Body.String(), "Failed to load products")

		h, tenantRepo, _, _ = setupInvoiceImportTestHandlers()
		wave7AddTenant(tenantRepo)
		rr = httptest.NewRecorder()
		h.ImportInvoices(rr, wave7BusinessRequest(http.MethodPost, "/tenants/tenant-1/invoices/import", invoicing.ImportInvoicesRequest{
			CSVContent: "bad\n",
		}, map[string]string{"tenantID": "tenant-1"}))

		assert.Equal(t, http.StatusBadRequest, rr.Code, rr.Body.String())
		assert.NotEmpty(t, rr.Body.String())
	})

	t.Run("create payment rejects locked payment date", func(t *testing.T) {
		h, _, tenantRepo := setupPaymentTestHandlers()
		tenantRecord := wave7AddTenant(tenantRepo)
		lockDate := "2026-03-31"
		tenantRecord.Settings.PeriodLockDate = &lockDate

		rr := httptest.NewRecorder()
		h.CreatePayment(rr, wave7BusinessRequest(http.MethodPost, "/tenants/tenant-1/payments", payments.CreatePaymentRequest{
			PaymentType:   payments.PaymentTypeReceived,
			PaymentDate:   time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC),
			Amount:        decimal.NewFromInt(50),
			Currency:      "EUR",
			PaymentMethod: "BANK",
		}, map[string]string{"tenantID": "tenant-1"}))

		assert.Equal(t, http.StatusConflict, rr.Code, rr.Body.String())
		assert.Contains(t, rr.Body.String(), "period locked through 2026-03-31")
	})
}

func TestBusinessWave7QuoteCommercialEvidenceAndConversionBranches(t *testing.T) {
	t.Run("update quote request and service errors", func(t *testing.T) {
		h, repo, tenantRepo := setupQuotesTestHandlers()
		wave7AddTenant(tenantRepo)
		repo.quotes["quote-1"] = wave5Quote(quotes.QuoteStatusDraft)

		rr := httptest.NewRecorder()
		h.UpdateQuote(rr, wave7BusinessRawRequest(http.MethodPut, "/tenants/tenant-1/quotes/quote-1", "{", map[string]string{"tenantID": "tenant-1", "quoteID": "quote-1"}))
		assert.Equal(t, http.StatusBadRequest, rr.Code, rr.Body.String())
		assert.Contains(t, rr.Body.String(), "Invalid request body")

		repo.updateErr = errors.New("quote update failed")
		rr = httptest.NewRecorder()
		h.UpdateQuote(rr, wave7BusinessRequest(http.MethodPut, "/tenants/tenant-1/quotes/quote-1", quotes.UpdateQuoteRequest{
			ContactID: "contact-1",
			QuoteDate: time.Date(2026, 4, 5, 0, 0, 0, 0, time.UTC),
			Lines:     []quotes.CreateQuoteLineRequest{wave5QuoteLine()},
		}, map[string]string{"tenantID": "tenant-1", "quoteID": "quote-1"}))
		assert.Equal(t, http.StatusBadRequest, rr.Code, rr.Body.String())
		assert.Contains(t, rr.Body.String(), "quote update failed")
	})

	t.Run("send quote evidence evaluation error and conflict response", func(t *testing.T) {
		h, repo, tenantRepo := setupQuotesTestHandlers()
		wave7AddTenant(tenantRepo)
		repo.quotes["quote-1"] = wave5Quote(quotes.QuoteStatusDraft)
		docRepo := newMockDocumentRepository()
		docRepo.listDocumentsErr = errors.New("document store unavailable")
		h.documentsService = documents.NewService(docRepo, nil)

		rr := httptest.NewRecorder()
		h.SendQuote(rr, wave7BusinessRequest(http.MethodPost, "/tenants/tenant-1/quotes/quote-1/send", map[string]bool{
			"require_approved_evidence": true,
		}, map[string]string{"tenantID": "tenant-1", "quoteID": "quote-1"}))
		assert.Equal(t, http.StatusInternalServerError, rr.Code, rr.Body.String())
		assert.Contains(t, rr.Body.String(), "evaluate quote evidence")

		h, repo, tenantRepo = setupQuotesTestHandlers()
		wave7AddTenant(tenantRepo)
		repo.quotes["quote-1"] = wave5Quote(quotes.QuoteStatusDraft)
		h.documentsService = documents.NewService(newMockDocumentRepository(), nil)
		rr = httptest.NewRecorder()
		h.SendQuote(rr, wave7BusinessRequest(http.MethodPost, "/tenants/tenant-1/quotes/quote-1/send", map[string]bool{
			"require_approved_evidence": true,
		}, map[string]string{"tenantID": "tenant-1", "quoteID": "quote-1"}))

		assert.Equal(t, http.StatusConflict, rr.Code, rr.Body.String())
		var conflict struct {
			Error              string                                `json:"error"`
			RemediationActions []documents.DocumentRemediationAction `json:"remediation_actions"`
		}
		require.NoError(t, json.NewDecoder(rr.Body).Decode(&conflict))
		assert.Contains(t, conflict.Error, "approved quote evidence is required")
		require.NotEmpty(t, conflict.RemediationActions)
	})

	t.Run("convert quote to invoice rejects locked issue date", func(t *testing.T) {
		h, repo, tenantRepo := setupQuotesTestHandlers()
		h.invoicingService = invoicing.NewServiceWithRepository(newMockInvoicingRepository(), nil)
		tenantRecord := wave7AddTenant(tenantRepo)
		lockDate := "2026-04-30"
		tenantRecord.Settings.PeriodLockDate = &lockDate
		repo.quotes["quote-1"] = wave5Quote(quotes.QuoteStatusAccepted)

		rr := httptest.NewRecorder()
		h.ConvertQuoteToInvoice(rr, wave7BusinessRequest(http.MethodPost, "/tenants/tenant-1/quotes/quote-1/convert-to-invoice", quotes.ConvertQuoteToInvoiceRequest{
			IssueDate: time.Date(2026, 4, 30, 0, 0, 0, 0, time.UTC),
			DueDate:   time.Date(2026, 5, 14, 0, 0, 0, 0, time.UTC),
		}, map[string]string{"tenantID": "tenant-1", "quoteID": "quote-1"}))

		assert.Equal(t, http.StatusConflict, rr.Code, rr.Body.String())
		assert.Contains(t, rr.Body.String(), "period locked through 2026-04-30")
	})
}

type wave7StockReservationListErrorRepository struct {
	*mockOrdersRepository
	listStockReservationsErr error
}

func (r *wave7StockReservationListErrorRepository) ListStockReservations(ctx context.Context, schemaName, tenantID, orderID string) ([]orders.OrderStockReservation, error) {
	if r.listStockReservationsErr != nil {
		return nil, r.listStockReservationsErr
	}
	return r.mockOrdersRepository.ListStockReservations(ctx, schemaName, tenantID, orderID)
}

func wave7OrderStockHandlers(repo *mockOrdersRepository, inventoryRepo *mockInventoryRepository) *Handlers {
	tenantRepo := newMockTenantRepository()
	wave7AddTenant(tenantRepo)
	return &Handlers{
		tenantService:    tenant.NewServiceWithRepository(tenantRepo),
		ordersService:    orders.NewServiceWithRepository(repo),
		inventoryService: inventory.NewServiceWithRepository(inventoryRepo),
	}
}

func wave7SeedOrderStockFixture(repo *mockOrdersRepository, inventoryRepo *mockInventoryRepository) (string, string, string) {
	productAvailable := "11111111-1111-4111-8111-111111111111"
	productShort := "22222222-2222-4222-8222-222222222222"
	productService := "33333333-3333-4333-8333-333333333333"
	warehouseID := "44444444-4444-4444-8444-444444444444"

	repo.orders["order-1"] = &orders.Order{
		ID:          "order-1",
		TenantID:    "tenant-1",
		OrderNumber: "ORD-W7",
		Status:      orders.OrderStatusConfirmed,
		Lines: []orders.OrderLine{
			{ID: "line-empty", LineNumber: 1, Description: "Manual item", Quantity: decimal.NewFromInt(1)},
			{ID: "line-service", LineNumber: 2, Description: "Service item", Quantity: decimal.NewFromInt(1), ProductID: &productService},
			{ID: "line-stocked", LineNumber: 3, Description: "Stocked item", Quantity: decimal.NewFromInt(3), ProductID: &productAvailable},
			{ID: "line-short", LineNumber: 4, Description: "Short item", Quantity: decimal.NewFromInt(4), ProductID: &productShort},
		},
	}
	inventoryRepo.warehouses[warehouseID] = &inventory.Warehouse{ID: warehouseID, TenantID: "tenant-1", Code: "MAIN", Name: "Main", IsActive: true}
	inventoryRepo.products[productAvailable] = &inventory.Product{
		ID: productAvailable, TenantID: "tenant-1", Code: "SKU-OK", Name: "Stocked", ProductType: inventory.ProductTypeGoods, TrackInventory: true, IsActive: true,
	}
	inventoryRepo.products[productShort] = &inventory.Product{
		ID: productShort, TenantID: "tenant-1", Code: "SKU-SHORT", Name: "Short", ProductType: inventory.ProductTypeGoods, TrackInventory: true, IsActive: true,
	}
	inventoryRepo.products[productService] = &inventory.Product{
		ID: productService, TenantID: "tenant-1", Code: "SVC", Name: "Service", ProductType: inventory.ProductTypeService, TrackInventory: false, IsActive: true,
	}
	inventoryRepo.stockLevels[productAvailable+"-"+warehouseID] = &inventory.StockLevel{
		ID: "stock-ok", TenantID: "tenant-1", ProductID: productAvailable, WarehouseID: warehouseID,
		Quantity: decimal.NewFromInt(10), ReservedQty: decimal.Zero, AvailableQty: decimal.NewFromInt(10),
	}
	inventoryRepo.stockLevels[productShort+"-"+warehouseID] = &inventory.StockLevel{
		ID: "stock-short", TenantID: "tenant-1", ProductID: productShort, WarehouseID: warehouseID,
		Quantity: decimal.NewFromInt(2), ReservedQty: decimal.Zero, AvailableQty: decimal.NewFromInt(2),
	}
	return productAvailable, productShort, warehouseID
}

func TestBusinessWave7OrderStockEdgeBranches(t *testing.T) {
	t.Run("check stock warehouse lookup and order errors", func(t *testing.T) {
		repo := newMockOrdersRepository()
		inventoryRepo := newMockInventoryRepository()
		h := wave7OrderStockHandlers(repo, inventoryRepo)

		rr := httptest.NewRecorder()
		h.CheckOrderStock(rr, wave7BusinessRequest(http.MethodGet, "/tenants/tenant-1/orders/order-1/stock-check?warehouse_id=missing", nil, map[string]string{"tenantID": "tenant-1", "orderID": "order-1"}))
		assert.Equal(t, http.StatusBadRequest, rr.Code, rr.Body.String())
		assert.Contains(t, rr.Body.String(), "Warehouse not found")

		warehouseID := "55555555-5555-4555-8555-555555555555"
		inventoryRepo.warehouses[warehouseID] = &inventory.Warehouse{ID: warehouseID, TenantID: "tenant-1", Code: "MAIN", Name: "Main", IsActive: true}
		rr = httptest.NewRecorder()
		h.CheckOrderStock(rr, wave7BusinessRequest(http.MethodGet, "/tenants/tenant-1/orders/missing/stock-check?warehouse_id="+warehouseID, nil, map[string]string{"tenantID": "tenant-1", "orderID": "missing"}))
		assert.Equal(t, http.StatusNotFound, rr.Code, rr.Body.String())
		assert.Contains(t, rr.Body.String(), "Order not found")

		repo.getErr = errors.New("order store down")
		rr = httptest.NewRecorder()
		h.CheckOrderStock(rr, wave7BusinessRequest(http.MethodGet, "/tenants/tenant-1/orders/order-1/stock-check?warehouse_id="+warehouseID, nil, map[string]string{"tenantID": "tenant-1", "orderID": "order-1"}))
		assert.Equal(t, http.StatusInternalServerError, rr.Code, rr.Body.String())
		assert.Contains(t, rr.Body.String(), "Failed to check order stock")
	})

	t.Run("builds stock check, rejects reservation shortage, and reports pick-list states", func(t *testing.T) {
		repo := newMockOrdersRepository()
		inventoryRepo := newMockInventoryRepository()
		productAvailable, productShort, warehouseID := wave7SeedOrderStockFixture(repo, inventoryRepo)
		h := wave7OrderStockHandlers(repo, inventoryRepo)

		rr := httptest.NewRecorder()
		h.CheckOrderStock(rr, wave7BusinessRequest(http.MethodGet, "/tenants/tenant-1/orders/order-1/stock-check?warehouse_id="+warehouseID, nil, map[string]string{"tenantID": "tenant-1", "orderID": "order-1"}))

		require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
		var check orders.OrderStockCheck
		require.NoError(t, json.NewDecoder(rr.Body).Decode(&check))
		assert.False(t, check.Ready)
		require.Len(t, check.Lines, 4)
		assert.Equal(t, orders.OrderStockLineStatusNotTracked, check.Lines[0].Status)
		assert.Equal(t, orders.OrderStockLineStatusNotTracked, check.Lines[1].Status)
		assert.Equal(t, orders.OrderStockLineStatusAvailable, check.Lines[2].Status)
		assert.Equal(t, orders.OrderStockLineStatusShortage, check.Lines[3].Status)

		rr = httptest.NewRecorder()
		h.ReserveOrderStock(rr, wave7BusinessRequest(http.MethodPost, "/tenants/tenant-1/orders/order-1/reserve-stock", orders.OrderStockReservationRequest{WarehouseID: warehouseID}, map[string]string{"tenantID": "tenant-1", "orderID": "order-1"}))
		assert.Equal(t, http.StatusBadRequest, rr.Code, rr.Body.String())
		assert.Contains(t, rr.Body.String(), "not ready for reservation")

		repo.stockReservations[orderStockReservationKey("order-1", productAvailable, warehouseID)] = &orders.OrderStockReservation{
			TenantID: "tenant-1", OrderID: "order-1", ProductID: productAvailable, WarehouseID: warehouseID,
			Quantity: decimal.NewFromInt(3), Status: orders.OrderStockReservationStatusReserved,
		}
		repo.stockReservations[orderStockReservationKey("order-1", productShort, warehouseID)] = &orders.OrderStockReservation{
			TenantID: "tenant-1", OrderID: "order-1", ProductID: productShort, WarehouseID: warehouseID,
			Quantity: decimal.NewFromInt(1), Status: orders.OrderStockReservationStatusReserved,
		}
		rr = httptest.NewRecorder()
		h.GetOrderPickList(rr, wave7BusinessRequest(http.MethodGet, "/tenants/tenant-1/orders/order-1/pick-list?warehouse_id="+warehouseID, nil, map[string]string{"tenantID": "tenant-1", "orderID": "order-1"}))

		require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
		var pickList orders.OrderPickList
		require.NoError(t, json.NewDecoder(rr.Body).Decode(&pickList))
		assert.False(t, pickList.Ready)
		require.Len(t, pickList.Lines, 4)
		assert.Equal(t, orders.OrderPickListLineStatusNotTracked, pickList.Lines[0].Status)
		assert.Equal(t, orders.OrderPickListLineStatusNotTracked, pickList.Lines[1].Status)
		assert.Equal(t, orders.OrderPickListLineStatusReady, pickList.Lines[2].Status)
		assert.Equal(t, orders.OrderPickListLineStatusShortage, pickList.Lines[3].Status)
	})

	t.Run("pick-list maps reservation list errors and nil status checks", func(t *testing.T) {
		baseRepo := newMockOrdersRepository()
		inventoryRepo := newMockInventoryRepository()
		_, _, warehouseID := wave7SeedOrderStockFixture(baseRepo, inventoryRepo)
		repo := &wave7StockReservationListErrorRepository{
			mockOrdersRepository:     baseRepo,
			listStockReservationsErr: errors.New("reservations unavailable"),
		}
		h := wave7OrderStockHandlers(baseRepo, inventoryRepo)
		h.ordersService = orders.NewServiceWithRepository(repo)

		rr := httptest.NewRecorder()
		h.GetOrderPickList(rr, wave7BusinessRequest(http.MethodGet, "/tenants/tenant-1/orders/order-1/pick-list?warehouse_id="+warehouseID, nil, map[string]string{"tenantID": "tenant-1", "orderID": "order-1"}))
		assert.Equal(t, http.StatusInternalServerError, rr.Code, rr.Body.String())
		assert.Contains(t, rr.Body.String(), "Failed to build order pick list")

		assert.False(t, orderStockCheckHasStatus(nil, orders.OrderStockLineStatusShortage))
	})
}

func wave7Asset(status assets.AssetStatus) *assets.FixedAsset {
	assetAccountID := "fixed-assets"
	depreciationExpenseID := "depreciation-expense"
	accumulatedDepreciationID := "accumulated-depreciation"
	return &assets.FixedAsset{
		ID:                            "asset-1",
		TenantID:                      "tenant-1",
		AssetNumber:                   "FA-W7",
		Name:                          "Laptop",
		Status:                        status,
		PurchaseDate:                  time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC),
		PurchaseCost:                  decimal.NewFromInt(1200),
		DepreciationMethod:            assets.DepreciationStraightLine,
		UsefulLifeMonths:              12,
		ResidualValue:                 decimal.Zero,
		AccumulatedDepreciation:       decimal.Zero,
		BookValue:                     decimal.NewFromInt(1200),
		AssetAccountID:                &assetAccountID,
		DepreciationExpenseAccountID:  &depreciationExpenseID,
		AccumulatedDepreciationAcctID: &accumulatedDepreciationID,
	}
}

func TestBusinessWave7FixedAssetHandlerBranches(t *testing.T) {
	t.Run("category list create and asset list create import update errors", func(t *testing.T) {
		h, repo, tenantRepo := setupAssetsTestHandlers()
		wave7AddTenant(tenantRepo)

		repo.listCategoriesErr = errors.New("asset category store down")
		rr := httptest.NewRecorder()
		h.ListAssetCategories(rr, wave7BusinessRequest(http.MethodGet, "/tenants/tenant-1/asset-categories", nil, map[string]string{"tenantID": "tenant-1"}))
		assert.Equal(t, http.StatusInternalServerError, rr.Code, rr.Body.String())
		assert.Contains(t, rr.Body.String(), "Failed to list asset categories")
		repo.listCategoriesErr = nil

		rr = httptest.NewRecorder()
		h.CreateAssetCategory(rr, wave7BusinessRawRequest(http.MethodPost, "/tenants/tenant-1/asset-categories", "{", map[string]string{"tenantID": "tenant-1"}))
		assert.Equal(t, http.StatusBadRequest, rr.Code, rr.Body.String())

		rr = httptest.NewRecorder()
		h.CreateAssetCategory(rr, wave7BusinessRequest(http.MethodPost, "/tenants/tenant-1/asset-categories", assets.CreateCategoryRequest{}, map[string]string{"tenantID": "tenant-1"}))
		assert.Equal(t, http.StatusBadRequest, rr.Code, rr.Body.String())
		assert.Contains(t, rr.Body.String(), "Category name is required")

		repo.createCategoryErr = errors.New("category insert failed")
		rr = httptest.NewRecorder()
		h.CreateAssetCategory(rr, wave7BusinessRequest(http.MethodPost, "/tenants/tenant-1/asset-categories", assets.CreateCategoryRequest{Name: "Computers"}, map[string]string{"tenantID": "tenant-1"}))
		assert.Equal(t, http.StatusBadRequest, rr.Code, rr.Body.String())
		assert.Contains(t, rr.Body.String(), "category insert failed")
		repo.createCategoryErr = nil

		repo.listErr = errors.New("asset list failed")
		rr = httptest.NewRecorder()
		h.ListAssets(rr, wave7BusinessRequest(http.MethodGet, "/tenants/tenant-1/assets?status=DRAFT&category_id=cat-1&search=laptop", nil, map[string]string{"tenantID": "tenant-1"}))
		assert.Equal(t, http.StatusInternalServerError, rr.Code, rr.Body.String())
		assert.Contains(t, rr.Body.String(), "Failed to list assets")
		repo.listErr = nil

		repo.createErr = errors.New("asset create failed")
		rr = httptest.NewRecorder()
		h.CreateAsset(rr, wave7BusinessRequest(http.MethodPost, "/tenants/tenant-1/assets", assets.CreateAssetRequest{
			Name:         "Laptop",
			PurchaseDate: time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC),
			PurchaseCost: decimal.NewFromInt(1200),
		}, map[string]string{"tenantID": "tenant-1"}))
		assert.Equal(t, http.StatusBadRequest, rr.Code, rr.Body.String())
		assert.Contains(t, rr.Body.String(), "asset create failed")
		repo.createErr = nil

		rr = httptest.NewRecorder()
		h.ImportAssets(rr, wave7BusinessRawRequest(http.MethodPost, "/tenants/tenant-1/assets/import", "{", map[string]string{"tenantID": "tenant-1"}))
		assert.Equal(t, http.StatusBadRequest, rr.Code, rr.Body.String())

		rr = httptest.NewRecorder()
		h.ImportAssets(rr, wave7BusinessRequest(http.MethodPost, "/tenants/tenant-1/assets/import", assets.ImportAssetsRequest{}, map[string]string{"tenantID": "tenant-1"}))
		assert.Equal(t, http.StatusBadRequest, rr.Code, rr.Body.String())
		assert.Contains(t, rr.Body.String(), "csv_content is required")

		rr = httptest.NewRecorder()
		h.ImportAssets(rr, wave7BusinessRequest(http.MethodPost, "/tenants/tenant-1/assets/import", assets.ImportAssetsRequest{CSVContent: "bad\n"}, map[string]string{"tenantID": "tenant-1"}))
		assert.Equal(t, http.StatusBadRequest, rr.Code, rr.Body.String())

		repo.assets["asset-1"] = wave7Asset(assets.AssetStatusDraft)
		rr = httptest.NewRecorder()
		h.UpdateAsset(rr, wave7BusinessRawRequest(http.MethodPut, "/tenants/tenant-1/assets/asset-1", "{", map[string]string{"tenantID": "tenant-1", "assetID": "asset-1"}))
		assert.Equal(t, http.StatusBadRequest, rr.Code, rr.Body.String())

		repo.updateErr = errors.New("asset update failed")
		rr = httptest.NewRecorder()
		h.UpdateAsset(rr, wave7BusinessRequest(http.MethodPut, "/tenants/tenant-1/assets/asset-1", assets.UpdateAssetRequest{Name: "Updated Laptop"}, map[string]string{"tenantID": "tenant-1", "assetID": "asset-1"}))
		assert.Equal(t, http.StatusBadRequest, rr.Code, rr.Body.String())
		assert.Contains(t, rr.Body.String(), "asset update failed")
	})

	t.Run("activation evidence and service branches", func(t *testing.T) {
		h, repo, tenantRepo := setupAssetsTestHandlers()
		wave7AddTenant(tenantRepo)

		rr := httptest.NewRecorder()
		h.ActivateAsset(rr, wave7BusinessRequest(http.MethodPost, "/tenants/tenant-1/assets/missing/activate", nil, map[string]string{"tenantID": "tenant-1", "assetID": "missing"}))
		assert.Equal(t, http.StatusBadRequest, rr.Code, rr.Body.String())
		assert.Contains(t, rr.Body.String(), "get asset")

		repo.assets["asset-1"] = wave7Asset(assets.AssetStatusDraft)
		rr = httptest.NewRecorder()
		h.ActivateAsset(rr, wave7BusinessRequest(http.MethodPost, "/tenants/tenant-1/assets/asset-1/activate", nil, map[string]string{"tenantID": "tenant-1", "assetID": "asset-1"}))
		assert.Equal(t, http.StatusConflict, rr.Code, rr.Body.String())
		assert.Contains(t, rr.Body.String(), "approved asset activation evidence")

		docRepo := newMockDocumentRepository()
		docRepo.listDocumentsErr = errors.New("documents unavailable")
		h.documentsService = documents.NewService(docRepo, nil)
		rr = httptest.NewRecorder()
		h.ActivateAsset(rr, wave7BusinessRequest(http.MethodPost, "/tenants/tenant-1/assets/asset-1/activate", nil, map[string]string{"tenantID": "tenant-1", "assetID": "asset-1"}))
		assert.Equal(t, http.StatusInternalServerError, rr.Code, rr.Body.String())
		assert.Contains(t, rr.Body.String(), "evaluate asset evidence")

		installApprovedEvidenceDocuments(t, h, documents.Document{
			EntityType:   documents.EntityTypeAsset,
			EntityID:     "asset-1",
			DocumentType: documents.DocumentTypeAssetRecord,
		})
		repo.updateErr = errors.New("activate failed")
		rr = httptest.NewRecorder()
		h.ActivateAsset(rr, wave7BusinessRequest(http.MethodPost, "/tenants/tenant-1/assets/asset-1/activate", nil, map[string]string{"tenantID": "tenant-1", "assetID": "asset-1"}))
		assert.Equal(t, http.StatusBadRequest, rr.Code, rr.Body.String())
		assert.Contains(t, rr.Body.String(), "activate failed")
	})

	t.Run("disposal evidence and service branches", func(t *testing.T) {
		h, repo, tenantRepo := setupAssetsTestHandlers()
		wave7AddTenant(tenantRepo)

		rr := httptest.NewRecorder()
		h.DisposeAsset(rr, wave7BusinessRawRequest(http.MethodPost, "/tenants/tenant-1/assets/asset-1/dispose", "{", map[string]string{"tenantID": "tenant-1", "assetID": "asset-1"}))
		assert.Equal(t, http.StatusBadRequest, rr.Code, rr.Body.String())

		rr = httptest.NewRecorder()
		h.DisposeAsset(rr, wave7BusinessRequest(http.MethodPost, "/tenants/tenant-1/assets/missing/dispose", assets.DisposeAssetRequest{
			DisposalDate:   time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC),
			DisposalMethod: assets.DisposalScrapped,
		}, map[string]string{"tenantID": "tenant-1", "assetID": "missing"}))
		assert.Equal(t, http.StatusBadRequest, rr.Code, rr.Body.String())
		assert.Contains(t, rr.Body.String(), "get asset")

		activeAsset := wave7Asset(assets.AssetStatusActive)
		activeAsset.AccumulatedDepreciation = activeAsset.PurchaseCost
		activeAsset.BookValue = decimal.Zero
		repo.assets["asset-1"] = activeAsset

		rr = httptest.NewRecorder()
		h.DisposeAsset(rr, wave7BusinessRequest(http.MethodPost, "/tenants/tenant-1/assets/asset-1/dispose", assets.DisposeAssetRequest{
			DisposalDate:   time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC),
			DisposalMethod: assets.DisposalScrapped,
		}, map[string]string{"tenantID": "tenant-1", "assetID": "asset-1"}))
		assert.Equal(t, http.StatusConflict, rr.Code, rr.Body.String())
		assert.Contains(t, rr.Body.String(), "approved asset disposal evidence")

		docRepo := newMockDocumentRepository()
		docRepo.listDocumentsErr = errors.New("documents unavailable")
		h.documentsService = documents.NewService(docRepo, nil)
		rr = httptest.NewRecorder()
		h.DisposeAsset(rr, wave7BusinessRequest(http.MethodPost, "/tenants/tenant-1/assets/asset-1/dispose", assets.DisposeAssetRequest{
			DisposalDate:   time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC),
			DisposalMethod: assets.DisposalScrapped,
		}, map[string]string{"tenantID": "tenant-1", "assetID": "asset-1"}))
		assert.Equal(t, http.StatusInternalServerError, rr.Code, rr.Body.String())
		assert.Contains(t, rr.Body.String(), "evaluate asset disposal evidence")

		installApprovedEvidenceDocuments(t, h, documents.Document{
			EntityType:   documents.EntityTypeAsset,
			EntityID:     "asset-1",
			DocumentType: documents.DocumentTypeSupportingDocument,
		})
		repo.updateErr = errors.New("dispose failed")
		rr = httptest.NewRecorder()
		h.DisposeAsset(rr, wave7BusinessRequest(http.MethodPost, "/tenants/tenant-1/assets/asset-1/dispose", assets.DisposeAssetRequest{
			DisposalDate:   time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC),
			DisposalMethod: assets.DisposalScrapped,
		}, map[string]string{"tenantID": "tenant-1", "assetID": "asset-1"}))
		assert.Equal(t, http.StatusBadRequest, rr.Code, rr.Body.String())
		assert.Contains(t, rr.Body.String(), "dispose failed")
	})

	t.Run("depreciation handler errors", func(t *testing.T) {
		h, repo, tenantRepo := setupAssetsTestHandlers()
		wave7AddTenant(tenantRepo)

		rr := httptest.NewRecorder()
		h.RecordDepreciation(rr, wave7BusinessRequest(http.MethodPost, "/tenants/tenant-1/assets/missing/depreciation", nil, map[string]string{"tenantID": "tenant-1", "assetID": "missing"}))
		assert.Equal(t, http.StatusBadRequest, rr.Code, rr.Body.String())
		assert.Contains(t, rr.Body.String(), "get asset")

		repo.depreciationErr = errors.New("depreciation history failed")
		rr = httptest.NewRecorder()
		h.GetDepreciationHistory(rr, wave7BusinessRequest(http.MethodGet, "/tenants/tenant-1/assets/asset-1/depreciation", nil, map[string]string{"tenantID": "tenant-1", "assetID": "asset-1"}))
		assert.Equal(t, http.StatusInternalServerError, rr.Code, rr.Body.String())
		assert.Contains(t, rr.Body.String(), "Failed to get depreciation history")
	})
}

func TestBusinessWave7EvidencePolicyConflictNilHelpers(t *testing.T) {
	var conflict *evidencePolicyConflictError
	assert.Equal(t, "", conflict.Error())
	assert.Nil(t, conflict.Unwrap())
}
