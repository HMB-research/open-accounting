package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/HMB-research/open-accounting/internal/email"
	"github.com/HMB-research/open-accounting/internal/inventory"
	"github.com/HMB-research/open-accounting/internal/invoicing"
	"github.com/HMB-research/open-accounting/internal/orders"
	"github.com/HMB-research/open-accounting/internal/tenant"
)

func TestPaymentReminderHandlers_SendSuccessAndFailureBranches(t *testing.T) {
	h, tenantRepo, _, reminderRepo, _, _ := setupMiscHandlers()
	tenantRepo.addTestTenant("tenant-1", "Example OU", "tenant-example")

	emailRepo := &emailHandlerRepository{
		settings:  make(map[string][]byte),
		templates: make(map[string]email.EmailTemplate),
		logs:      []email.EmailLog{},
	}
	emailRepo.settings["tenant-1"] = []byte(`{
		"smtp_host":"smtp.example.com",
		"smtp_port":587,
		"smtp_username":"user@example.com",
		"smtp_password":"secret",
		"smtp_from_email":"billing@example.com",
		"smtp_from_name":"Billing",
		"smtp_use_tls":true
	}`)
	emailRepo.templates[emailTemplateKey("tenant-1", email.TemplateOverdueReminder)] = email.EmailTemplate{
		ID:           "template-overdue",
		TenantID:     "tenant-1",
		TemplateType: email.TemplateOverdueReminder,
		Subject:      "Reminder for {{.InvoiceNumber}}",
		BodyHTML:     "<p>{{.ContactName}} {{.CompanyName}} {{.TotalAmount}}</p>",
		BodyText:     "{{.Message}}",
		IsActive:     true,
	}
	mailer := &emailHandlerMailer{}
	h.emailService = email.NewServiceWithRepository(emailRepo, mailer)
	h.reminderService = invoicing.NewReminderServiceWithRepository(reminderRepo, h.emailService)

	reminderRepo.AddMockOverdueInvoice(
		"inv-1",
		"INV-001",
		"contact-1",
		"Acme OU",
		"billing@example.com",
		"EUR",
		decimal.NewFromInt(125),
		decimal.Zero,
		14,
	)
	reminderRepo.AddMockOverdueInvoice(
		"inv-no-email",
		"INV-002",
		"contact-2",
		"No Email OU",
		"",
		"EUR",
		decimal.NewFromInt(80),
		decimal.Zero,
		21,
	)

	req := makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/invoices/reminders", invoicing.SendReminderRequest{
		InvoiceID: "inv-1",
		Message:   "Please settle this invoice.",
	}, nil)
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
	rr := httptest.NewRecorder()
	h.SendPaymentReminder(rr, req)

	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	var single invoicing.ReminderResult
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&single))
	assert.True(t, single.Success)
	assert.Equal(t, "INV-001", single.InvoiceNumber)
	assert.Equal(t, 1, mailer.sentCount)
	require.Len(t, reminderRepo.Reminders["inv-1"], 1)
	assert.Equal(t, invoicing.ReminderStatusSent, reminderRepo.Reminders["inv-1"][0].Status)
	require.Len(t, emailRepo.logs, 1)
	assert.Equal(t, string(email.TemplateOverdueReminder), emailRepo.logs[0].EmailType)
	assert.Equal(t, "inv-1", emailRepo.logs[0].RelatedID)

	req = makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/invoices/reminders", invoicing.SendReminderRequest{
		InvoiceID: "missing",
	}, nil)
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
	rr = httptest.NewRecorder()
	h.SendPaymentReminder(rr, req)

	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&single))
	assert.False(t, single.Success)
	assert.Contains(t, single.Message, "not found")

	req = makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/invoices/reminders/bulk", invoicing.SendBulkRemindersRequest{
		InvoiceIDs: []string{"inv-1", "inv-no-email", "missing"},
		Message:    "Bulk reminder",
	}, nil)
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
	rr = httptest.NewRecorder()
	h.SendBulkPaymentReminders(rr, req)

	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	var bulk invoicing.BulkReminderResult
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&bulk))
	assert.Equal(t, 3, bulk.TotalRequested)
	assert.Equal(t, 1, bulk.Successful)
	assert.Equal(t, 2, bulk.Failed)

	reminderRepo.GetOverdueErr = errors.New("database unavailable")
	req = makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/invoices/reminders", invoicing.SendReminderRequest{
		InvoiceID: "inv-1",
	}, nil)
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
	rr = httptest.NewRecorder()
	h.SendPaymentReminder(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	assert.Contains(t, rr.Body.String(), "Failed to send payment reminder")
}

func TestPaymentReminderHandlers_RequestValidationAndTemplateFailure(t *testing.T) {
	h, tenantRepo, _, reminderRepo, _, _ := setupMiscHandlers()
	tenantRepo.tenants["tenant-1"] = &tenant.Tenant{ID: "tenant-1", SchemaName: "tenant_test"}

	reminderRepo.AddMockOverdueInvoice(
		"inv-1",
		"INV-001",
		"contact-1",
		"Acme OU",
		"billing@example.com",
		"EUR",
		decimal.NewFromInt(100),
		decimal.Zero,
		5,
	)

	req := httptest.NewRequest(http.MethodPost, "/tenants/tenant-1/invoices/reminders", bytes.NewReader([]byte("{")))
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
	rr := httptest.NewRecorder()
	h.SendPaymentReminder(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)

	req = httptest.NewRequest(http.MethodPost, "/tenants/tenant-1/invoices/reminders/bulk", bytes.NewReader([]byte("{")))
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
	rr = httptest.NewRecorder()
	h.SendBulkPaymentReminders(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)

	req = makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/invoices/reminders", invoicing.SendReminderRequest{
		InvoiceID: "inv-1",
	}, nil)
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
	rr = httptest.NewRecorder()
	h.SendPaymentReminder(rr, req)

	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	var result invoicing.ReminderResult
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&result))
	assert.False(t, result.Success)
	assert.Contains(t, result.Message, "email template")
}

func TestOrderStockReservationHandlers_ErrorBranches(t *testing.T) {
	h, repo, tenantRepo := setupOrdersTestHandlers()
	tenantRepo.tenants["tenant-1"] = &tenant.Tenant{
		ID:         "tenant-1",
		SchemaName: "tenant_test",
	}
	claims := createTestClaims("user-1", "test@example.com", "tenant-1", "owner")

	req := httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/orders/order-1/pick-list?warehouse_id=wh-1", nil)
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1", "orderID": "order-1"})
	req = req.WithContext(contextWithClaims(req.Context(), claims))
	rr := httptest.NewRecorder()
	h.GetOrderPickList(rr, req)
	assert.Equal(t, http.StatusInternalServerError, rr.Code)

	req = makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/orders/order-1/reserve-stock", orders.OrderStockReservationRequest{
		WarehouseID: "wh-1",
	}, nil)
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1", "orderID": "order-1"})
	rr = httptest.NewRecorder()
	h.ReserveOrderStock(rr, req)
	assert.Equal(t, http.StatusInternalServerError, rr.Code)

	inventoryRepo := newMockInventoryRepository()
	h.inventoryService = inventory.NewServiceWithRepository(inventoryRepo)

	req = httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/orders/order-1/pick-list", nil)
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1", "orderID": "order-1"})
	req = req.WithContext(contextWithClaims(req.Context(), claims))
	rr = httptest.NewRecorder()
	h.GetOrderPickList(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "warehouse_id is required")

	req = httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/orders/missing/stock-reservations", nil)
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1", "orderID": "missing"})
	req = req.WithContext(contextWithClaims(req.Context(), claims))
	rr = httptest.NewRecorder()
	h.ListOrderStockReservations(rr, req)
	assert.Equal(t, http.StatusNotFound, rr.Code)

	repo.getErr = errors.New("lookup failed")
	req = httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/orders/order-1/stock-reservations", nil)
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1", "orderID": "order-1"})
	req = req.WithContext(contextWithClaims(req.Context(), claims))
	rr = httptest.NewRecorder()
	h.ListOrderStockReservations(rr, req)
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	repo.getErr = nil

	productID := "11111111-1111-4111-8111-111111111111"
	warehouseID := "22222222-2222-4222-8222-222222222222"
	repo.orders["order-1"] = &orders.Order{
		ID:          "order-1",
		TenantID:    "tenant-1",
		OrderNumber: "ORD-001",
		Status:      orders.OrderStatusConfirmed,
		Lines: []orders.OrderLine{
			{ID: "line-1", LineNumber: 1, Description: "Missing product", Quantity: decimal.NewFromInt(2), ProductID: &productID},
		},
	}
	inventoryRepo.warehouses[warehouseID] = &inventory.Warehouse{ID: warehouseID, TenantID: "tenant-1", Code: "MAIN", Name: "Main", IsActive: true}

	req = httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/orders/order-1/pick-list?warehouse_id="+warehouseID, nil)
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1", "orderID": "order-1"})
	req = req.WithContext(contextWithClaims(req.Context(), claims))
	rr = httptest.NewRecorder()
	h.GetOrderPickList(rr, req)

	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	var pickList orders.OrderPickList
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&pickList))
	require.Len(t, pickList.Lines, 1)
	assert.False(t, pickList.Ready)
	assert.Equal(t, orders.OrderPickListLineStatusProductNotFound, pickList.Lines[0].Status)

	body, _ := json.Marshal(orders.OrderStockReservationRequest{WarehouseID: warehouseID})
	req = httptest.NewRequest(http.MethodPost, "/tenants/tenant-1/orders/order-1/release-stock", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1", "orderID": "order-1"})
	req = req.WithContext(contextWithClaims(req.Context(), claims))
	rr = httptest.NewRecorder()
	h.ReleaseOrderStock(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "missing product references")
}

func TestOrderStockReservationHandlers_ApplyReservationFailureRollsBack(t *testing.T) {
	repo := &failingStockReservationRepository{
		mockOrdersRepository: newMockOrdersRepository(),
		upsertErr:            errors.New("reservation ledger down"),
	}
	tenantRepo := newMockTenantRepository()
	h := &Handlers{
		ordersService: orders.NewServiceWithRepository(repo),
		tenantService: tenant.NewServiceWithRepository(tenantRepo),
	}
	inventoryRepo := newMockInventoryRepository()
	h.inventoryService = inventory.NewServiceWithRepository(inventoryRepo)
	tenantRepo.tenants["tenant-1"] = &tenant.Tenant{
		ID:         "tenant-1",
		SchemaName: "tenant_test",
	}

	productID := "33333333-3333-4333-8333-333333333333"
	warehouseID := "44444444-4444-4444-8444-444444444444"
	repo.orders["order-1"] = &orders.Order{
		ID:          "order-1",
		TenantID:    "tenant-1",
		OrderNumber: "ORD-001",
		Status:      orders.OrderStatusConfirmed,
		Lines: []orders.OrderLine{
			{ID: "line-1", LineNumber: 1, Description: "Tracked goods", Quantity: decimal.NewFromInt(2), ProductID: &productID},
		},
	}
	inventoryRepo.warehouses[warehouseID] = &inventory.Warehouse{ID: warehouseID, TenantID: "tenant-1", Code: "MAIN", Name: "Main", IsActive: true}
	inventoryRepo.products[productID] = &inventory.Product{
		ID:             productID,
		TenantID:       "tenant-1",
		Code:           "PROD-001",
		Name:           "Widget",
		ProductType:    inventory.ProductTypeGoods,
		TrackInventory: true,
		IsActive:       true,
	}
	inventoryRepo.stockLevels[productID+"-"+warehouseID] = &inventory.StockLevel{
		ID:           "sl-1",
		TenantID:     "tenant-1",
		ProductID:    productID,
		WarehouseID:  warehouseID,
		Quantity:     decimal.NewFromInt(5),
		ReservedQty:  decimal.Zero,
		AvailableQty: decimal.NewFromInt(5),
	}

	ctx := contextWithClaims(context.Background(), createTestClaims("user-1", "test@example.com", "tenant-1", "owner"))
	check, err := h.buildOrderStockCheck(ctx, "tenant-1", "tenant_test", "order-1", warehouseID)
	require.NoError(t, err)

	result, err := h.applyOrderStockReservation(ctx, "tenant-1", "tenant_test", orders.OrderStockReservationActionReserve, "", "user-1", check)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "reservation ledger down")
	level := inventoryRepo.stockLevels[productID+"-"+warehouseID]
	assert.True(t, level.ReservedQty.IsZero())
	assert.True(t, level.AvailableQty.Equal(decimal.NewFromInt(5)))
}

type failingStockReservationRepository struct {
	*mockOrdersRepository
	upsertErr error
}

func (m *failingStockReservationRepository) UpsertStockReservation(ctx context.Context, schemaName string, reservation *orders.OrderStockReservation) error {
	if m.upsertErr != nil {
		return m.upsertErr
	}
	return m.mockOrdersRepository.UpsertStockReservation(ctx, schemaName, reservation)
}
