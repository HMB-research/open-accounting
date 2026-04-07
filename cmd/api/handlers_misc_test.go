package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"

	"github.com/HMB-research/open-accounting/internal/accounting"
	"github.com/HMB-research/open-accounting/internal/auth"
	"github.com/HMB-research/open-accounting/internal/invoicing"
	"github.com/HMB-research/open-accounting/internal/reports"
	"github.com/HMB-research/open-accounting/internal/tenant"
)

type mockReminderRuleRepository struct {
	rules map[string]*invoicing.ReminderRule
}

func newMockReminderRuleRepository() *mockReminderRuleRepository {
	return &mockReminderRuleRepository{rules: make(map[string]*invoicing.ReminderRule)}
}

func (m *mockReminderRuleRepository) ListRules(ctx context.Context, schemaName, tenantID string) ([]invoicing.ReminderRule, error) {
	result := make([]invoicing.ReminderRule, 0, len(m.rules))
	for _, rule := range m.rules {
		if rule.TenantID == tenantID {
			result = append(result, *rule)
		}
	}
	return result, nil
}

func (m *mockReminderRuleRepository) ListActiveRules(ctx context.Context, schemaName, tenantID string) ([]invoicing.ReminderRule, error) {
	result := make([]invoicing.ReminderRule, 0, len(m.rules))
	for _, rule := range m.rules {
		if rule.TenantID == tenantID && rule.IsActive {
			result = append(result, *rule)
		}
	}
	return result, nil
}

func (m *mockReminderRuleRepository) GetRule(ctx context.Context, schemaName, tenantID, ruleID string) (*invoicing.ReminderRule, error) {
	rule, ok := m.rules[ruleID]
	if !ok || rule.TenantID != tenantID {
		return nil, invoicing.ErrRuleNotFound
	}
	return rule, nil
}

func (m *mockReminderRuleRepository) CreateRule(ctx context.Context, schemaName string, rule *invoicing.ReminderRule) error {
	m.rules[rule.ID] = rule
	return nil
}

func (m *mockReminderRuleRepository) UpdateRule(ctx context.Context, schemaName string, rule *invoicing.ReminderRule) error {
	if _, ok := m.rules[rule.ID]; !ok {
		return invoicing.ErrRuleNotFound
	}
	m.rules[rule.ID] = rule
	return nil
}

func (m *mockReminderRuleRepository) DeleteRule(ctx context.Context, schemaName, tenantID, ruleID string) error {
	if _, ok := m.rules[ruleID]; !ok {
		return invoicing.ErrRuleNotFound
	}
	delete(m.rules, ruleID)
	return nil
}

func (m *mockReminderRuleRepository) GetInvoicesForRule(ctx context.Context, schemaName, tenantID string, rule *invoicing.ReminderRule, asOfDate time.Time) ([]invoicing.InvoiceForReminder, error) {
	return nil, nil
}

func (m *mockReminderRuleRepository) HasReminderBeenSent(ctx context.Context, schemaName, tenantID, invoiceID, ruleID string) (bool, error) {
	return false, nil
}

func (m *mockReminderRuleRepository) RecordReminderSent(ctx context.Context, schemaName string, reminder *invoicing.PaymentReminder) error {
	return nil
}

func setupMiscHandlers() (*Handlers, *mockTenantRepository, *reports.MockRepository, *invoicing.MockReminderRepository, *accounting.MockCostCenterRepository, *mockReminderRuleRepository) {
	tenantRepo := newMockTenantRepository()
	tenantSvc := tenant.NewServiceWithRepository(tenantRepo)

	reportsRepo := reports.NewMockRepository()
	reminderRepo := invoicing.NewMockReminderRepository()
	costCenterRepo := accounting.NewMockCostCenterRepository()
	ruleRepo := newMockReminderRuleRepository()

	return &Handlers{
		tenantService:            tenantSvc,
		reportsService:           reports.NewServiceWithRepository(reportsRepo),
		reminderService:          invoicing.NewReminderServiceWithRepository(reminderRepo, nil),
		costCenterService:        accounting.NewCostCenterServiceWithRepository(costCenterRepo),
		automatedReminderService: invoicing.NewAutomatedReminderServiceWithRepository(ruleRepo, nil),
	}, tenantRepo, reportsRepo, reminderRepo, costCenterRepo, ruleRepo
}

func TestExtendedReportHandlers(t *testing.T) {
	h, tenantRepo, reportsRepo, _, _, _ := setupMiscHandlers()
	tenantRepo.addTestTenant("tenant-1", "Test Tenant", "test-tenant")

	reportsRepo.JournalEntries = []reports.JournalEntryWithLines{
		{
			ID:        "je-1",
			EntryDate: time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC),
			Lines: []reports.JournalLine{
				{AccountCode: "1000", AccountType: "ASSET", Debit: decimal.NewFromInt(500)},
				{AccountCode: "4000", AccountType: "REVENUE", Credit: decimal.NewFromInt(500)},
			},
		},
	}
	reportsRepo.CashBalance = decimal.NewFromInt(1000)
	reportsRepo.ContactBalances = []reports.ContactBalance{{
		ContactID:    "contact-1",
		ContactName:  "Example Customer",
		ContactCode:  "CUST-1",
		ContactEmail: "billing@example.com",
		Balance:      decimal.NewFromInt(250),
		InvoiceCount: 2,
	}}
	reportsRepo.Contact = reports.ContactInfo{ID: "contact-1", Name: "Example Customer", Code: "CUST-1", Email: "billing@example.com"}
	reportsRepo.ContactInvoices = []reports.BalanceInvoice{{
		InvoiceID:         "inv-1",
		InvoiceNumber:     "INV-001",
		InvoiceDate:       "2026-01-01",
		DueDate:           "2026-01-15",
		TotalAmount:       decimal.NewFromInt(250),
		OutstandingAmount: decimal.NewFromInt(250),
		Currency:          "EUR",
		DaysOverdue:       10,
	}}

	req := withURLParams(httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/reports/cash-flow?start_date=2026-01-01&end_date=2026-01-31", nil), map[string]string{"tenantID": "tenant-1"})
	rr := httptest.NewRecorder()
	h.GetCashFlowStatement(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)

	req = withURLParams(httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/reports/cash-flow?start_date=bad&end_date=2026-01-31", nil), map[string]string{"tenantID": "tenant-1"})
	rr = httptest.NewRecorder()
	h.GetCashFlowStatement(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)

	req = withURLParams(httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/reports/balance-confirmations?type=RECEIVABLE&as_of_date=2026-01-31", nil), map[string]string{"tenantID": "tenant-1"})
	rr = httptest.NewRecorder()
	h.GetBalanceConfirmationSummary(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)

	req = withURLParams(httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/reports/balance-confirmations?type=OTHER&as_of_date=2026-01-31", nil), map[string]string{"tenantID": "tenant-1"})
	rr = httptest.NewRecorder()
	h.GetBalanceConfirmationSummary(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)

	req = withURLParams(httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/reports/balance-confirmations/contact-1?type=RECEIVABLE&as_of_date=2026-01-31", nil), map[string]string{
		"tenantID":  "tenant-1",
		"contactID": "contact-1",
	})
	rr = httptest.NewRecorder()
	h.GetBalanceConfirmation(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)

	req = withURLParams(httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/reports/balance-confirmations/contact-1?type=RECEIVABLE&as_of_date=invalid", nil), map[string]string{
		"tenantID":  "tenant-1",
		"contactID": "contact-1",
	})
	rr = httptest.NewRecorder()
	h.GetBalanceConfirmation(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestReminderAndCostCenterHandlers(t *testing.T) {
	h, tenantRepo, _, reminderRepo, costCenterRepo, _ := setupMiscHandlers()
	tenantRepo.addTestTenant("tenant-1", "Test Tenant", "test-tenant")

	reminderRepo.AddMockOverdueInvoice("inv-1", "INV-001", "contact-1", "Example Customer", "billing@example.com", "EUR", decimal.NewFromInt(100), decimal.Zero, 7)
	reminderRepo.Reminders["inv-1"] = []invoicing.PaymentReminder{{ID: "rem-1", InvoiceID: "inv-1", Status: invoicing.ReminderStatusSent}}

	req := withURLParams(httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/invoices/overdue", nil), map[string]string{"tenantID": "tenant-1"})
	rr := httptest.NewRecorder()
	h.GetOverdueInvoices(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)

	req = makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/invoices/reminders", map[string]string{}, nil)
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
	rr = httptest.NewRecorder()
	h.SendPaymentReminder(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)

	req = makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/invoices/reminders/bulk", map[string]interface{}{"invoice_ids": []string{}}, nil)
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
	rr = httptest.NewRecorder()
	h.SendBulkPaymentReminders(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)

	req = withURLParams(httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/invoices/inv-1/reminders", nil), map[string]string{
		"tenantID":  "tenant-1",
		"invoiceID": "inv-1",
	})
	rr = httptest.NewRecorder()
	h.GetInvoiceReminderHistory(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)

	budget := decimal.NewFromInt(1000)
	costCenterRepo.CostCenters["cc-1"] = &accounting.CostCenter{
		ID:           "cc-1",
		TenantID:     "tenant-1",
		Code:         "ADMIN",
		Name:         "Administration",
		IsActive:     true,
		BudgetAmount: &budget,
		BudgetPeriod: accounting.BudgetPeriodAnnual,
	}
	costCenterRepo.Allocations["cc-1"] = []accounting.CostAllocation{{
		ID:             "alloc-1",
		TenantID:       "tenant-1",
		CostCenterID:   "cc-1",
		Amount:         decimal.NewFromInt(300),
		AllocationDate: time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC),
	}}

	req = withURLParams(httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/cost-centers?active_only=true", nil), map[string]string{"tenantID": "tenant-1"})
	rr = httptest.NewRecorder()
	h.ListCostCenters(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)

	req = withURLParams(httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/cost-centers/cc-1", nil), map[string]string{
		"tenantID":     "tenant-1",
		"costCenterID": "cc-1",
	})
	rr = httptest.NewRecorder()
	h.GetCostCenter(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)

	req = makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/cost-centers", map[string]interface{}{
		"code": "OPS",
		"name": "Operations",
	}, nil)
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
	rr = httptest.NewRecorder()
	h.CreateCostCenter(rr, req)
	assert.Equal(t, http.StatusCreated, rr.Code)

	req = makeAuthenticatedRequest(http.MethodPut, "/tenants/tenant-1/cost-centers/cc-1", map[string]interface{}{
		"code": "ADMIN",
		"name": "Admin Updated",
	}, nil)
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1", "costCenterID": "cc-1"})
	rr = httptest.NewRecorder()
	h.UpdateCostCenter(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)

	req = withURLParams(httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/cost-centers/report?start_date=2026-01-01&end_date=2026-01-31", nil), map[string]string{"tenantID": "tenant-1"})
	rr = httptest.NewRecorder()
	h.GetCostCenterReport(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)

	req = withURLParams(httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/cost-centers/report?start_date=bad-date", nil), map[string]string{"tenantID": "tenant-1"})
	rr = httptest.NewRecorder()
	h.GetCostCenterReport(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)

	req = makeAuthenticatedRequest(http.MethodDelete, "/tenants/tenant-1/cost-centers/cc-1", nil, nil)
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1", "costCenterID": "cc-1"})
	rr = httptest.NewRecorder()
	h.DeleteCostCenter(rr, req)
	assert.Equal(t, http.StatusNoContent, rr.Code)

	req = makeAuthenticatedRequest(http.MethodDelete, "/tenants/tenant-1/cost-centers/missing", nil, nil)
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1", "costCenterID": "missing"})
	rr = httptest.NewRecorder()
	h.DeleteCostCenter(rr, req)
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestReminderRuleHandlers(t *testing.T) {
	h, tenantRepo, _, _, _, ruleRepo := setupMiscHandlers()
	tenantRepo.addTestTenant("tenant-1", "Test Tenant", "test-tenant")

	existing := &invoicing.ReminderRule{
		ID:                "rule-1",
		TenantID:          "tenant-1",
		Name:              "Seven days overdue",
		TriggerType:       invoicing.TriggerAfterDue,
		DaysOffset:        7,
		EmailTemplateType: "OVERDUE_REMINDER",
		IsActive:          true,
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}
	ruleRepo.rules[existing.ID] = existing

	req := withURLParams(httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/reminder-rules", nil), map[string]string{"tenantID": "tenant-1"})
	rr := httptest.NewRecorder()
	h.ListReminderRules(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)

	req = withURLParams(httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/reminder-rules/rule-1", nil), map[string]string{"tenantID": "tenant-1", "ruleID": "rule-1"})
	rr = httptest.NewRecorder()
	h.GetReminderRule(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)

	req = makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/reminder-rules", map[string]interface{}{
		"name":         "Before due",
		"trigger_type": invoicing.TriggerBeforeDue,
		"days_offset":  3,
		"is_active":    true,
	}, nil)
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
	rr = httptest.NewRecorder()
	h.CreateReminderRule(rr, req)
	assert.Equal(t, http.StatusCreated, rr.Code)

	req = makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/reminder-rules", map[string]interface{}{}, nil)
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
	rr = httptest.NewRecorder()
	h.CreateReminderRule(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)

	name := "Renamed rule"
	req = makeAuthenticatedRequest(http.MethodPut, "/tenants/tenant-1/reminder-rules/rule-1", map[string]interface{}{
		"name": name,
	}, nil)
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1", "ruleID": "rule-1"})
	rr = httptest.NewRecorder()
	h.UpdateReminderRule(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)

	req = makeAuthenticatedRequest(http.MethodDelete, "/tenants/tenant-1/reminder-rules/rule-1", nil, nil)
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1", "ruleID": "rule-1"})
	rr = httptest.NewRecorder()
	h.DeleteReminderRule(rr, req)
	assert.Equal(t, http.StatusNoContent, rr.Code)

	req = makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/reminder-rules/trigger", nil, nil)
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
	rr = httptest.NewRecorder()
	h.TriggerReminders(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestInterestAndPluginValidationHandlers(t *testing.T) {
	tenantRepo := newMockTenantRepository()
	tenantRecord := tenantRepo.addTestTenant("de305d54-75b4-431b-adb2-eb6b9e546014", "Test Tenant", "test-tenant")
	tenantSvc := tenant.NewServiceWithRepository(tenantRepo)

	h := &Handlers{tenantService: tenantSvc}
	claims := &auth.Claims{UserID: "user-1", TenantID: tenantRecord.ID, Role: tenant.RoleOwner}
	tenantRepo.tenantUsers[tenantRecord.ID] = []tenant.TenantUser{{TenantID: tenantRecord.ID, UserID: "user-1", Role: tenant.RoleOwner}}

	req := withURLParams(httptest.NewRequest(http.MethodGet, "/tenants/"+tenantRecord.ID+"/settings/interest", nil), map[string]string{"tenantID": tenantRecord.ID})
	rr := httptest.NewRecorder()
	h.GetInterestSettings(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)

	req = makeAuthenticatedRequest(http.MethodPut, "/tenants/"+tenantRecord.ID+"/settings/interest", map[string]interface{}{"rate": 0.001}, claims)
	req = withURLParams(req, map[string]string{"tenantID": tenantRecord.ID})
	rr = httptest.NewRecorder()
	h.UpdateInterestSettings(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)

	req = makeAuthenticatedRequest(http.MethodPut, "/tenants/"+tenantRecord.ID+"/settings/interest", map[string]interface{}{"rate": -1}, claims)
	req = withURLParams(req, map[string]string{"tenantID": tenantRecord.ID})
	rr = httptest.NewRecorder()
	h.UpdateInterestSettings(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)

	req = withURLParams(httptest.NewRequest(http.MethodGet, "/tenants/missing/invoices/inv-1/interest", nil), map[string]string{
		"tenantID":  "missing",
		"invoiceID": "inv-1",
	})
	rr = httptest.NewRecorder()
	h.GetInvoiceInterest(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)

	req = withURLParams(httptest.NewRequest(http.MethodGet, "/tenants/missing/invoices/overdue-with-interest", nil), map[string]string{"tenantID": "missing"})
	rr = httptest.NewRecorder()
	h.GetOverdueInvoicesWithInterest(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)

	assert.Equal(t, "Interest calculation disabled", formatInterestDescription(0))
	assert.Equal(t, "1.235", formatFloat(1.23456, 3))

	req = httptest.NewRequest(http.MethodPost, "/admin/plugin-registries", strings.NewReader("{"))
	rr = httptest.NewRecorder()
	h.AddPluginRegistry(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)

	req = makeAuthenticatedRequest(http.MethodPost, "/admin/plugin-registries", map[string]string{}, nil)
	rr = httptest.NewRecorder()
	h.AddPluginRegistry(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)

	for _, handler := range []func(http.ResponseWriter, *http.Request){h.RemovePluginRegistry, h.SyncPluginRegistry, h.UninstallPlugin, h.DisablePlugin, h.GetPlugin} {
		req = withURLParams(httptest.NewRequest(http.MethodDelete, "/admin/plugins/not-a-uuid", nil), map[string]string{"id": "not-a-uuid"})
		rr = httptest.NewRecorder()
		handler(rr, req)
		assert.Equal(t, http.StatusBadRequest, rr.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/admin/plugins/search", nil)
	rr = httptest.NewRecorder()
	h.SearchPlugins(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)

	req = httptest.NewRequest(http.MethodPost, "/admin/plugins/install", strings.NewReader("{"))
	rr = httptest.NewRecorder()
	h.InstallPlugin(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)

	req = makeAuthenticatedRequest(http.MethodPost, "/admin/plugins/install", map[string]string{}, nil)
	rr = httptest.NewRecorder()
	h.InstallPlugin(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)

	req = withURLParams(httptest.NewRequest(http.MethodPost, "/admin/plugins/not-a-uuid/enable", strings.NewReader("{}")), map[string]string{"id": "not-a-uuid"})
	rr = httptest.NewRecorder()
	h.EnablePlugin(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)

	req = httptest.NewRequest(http.MethodGet, "/admin/plugins/permissions", nil)
	rr = httptest.NewRecorder()
	h.GetAllPermissions(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)

	req = httptest.NewRequest(http.MethodGet, "/tenants/"+tenantRecord.ID+"/plugins", nil)
	rr = httptest.NewRecorder()
	h.ListTenantPlugins(rr, req)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)

	req = makeAuthenticatedRequest(http.MethodGet, "/tenants/not-a-uuid/plugins", nil, claims)
	req = withURLParams(req, map[string]string{"tenantID": "not-a-uuid"})
	rr = httptest.NewRecorder()
	h.ListTenantPlugins(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)

	req = makeAuthenticatedRequest(http.MethodPost, "/tenants/"+tenantRecord.ID+"/plugins/not-a-uuid/enable", nil, claims)
	req = withURLParams(req, map[string]string{"tenantID": tenantRecord.ID, "pluginID": "not-a-uuid"})
	rr = httptest.NewRecorder()
	h.EnableTenantPlugin(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)

	req = makeAuthenticatedRequest(http.MethodPost, "/tenants/"+tenantRecord.ID+"/plugins/not-a-uuid/disable", nil, claims)
	req = withURLParams(req, map[string]string{"tenantID": tenantRecord.ID, "pluginID": "not-a-uuid"})
	rr = httptest.NewRecorder()
	h.DisableTenantPlugin(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)

	req = makeAuthenticatedRequest(http.MethodGet, "/tenants/"+tenantRecord.ID+"/plugins/not-a-uuid/settings", nil, claims)
	req = withURLParams(req, map[string]string{"tenantID": tenantRecord.ID, "pluginID": "not-a-uuid"})
	rr = httptest.NewRecorder()
	h.GetTenantPluginSettings(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)

	req = makeAuthenticatedRequest(http.MethodPut, "/tenants/"+tenantRecord.ID+"/plugins/not-a-uuid/settings", map[string]string{"enabled": "true"}, claims)
	req = withURLParams(req, map[string]string{"tenantID": tenantRecord.ID, "pluginID": "not-a-uuid"})
	rr = httptest.NewRecorder()
	h.UpdateTenantPluginSettings(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestPluginTenantAdminValidation(t *testing.T) {
	tenantRepo := newMockTenantRepository()
	tenantRecord := tenantRepo.addTestTenant(uuid.New().String(), "Test Tenant", "test-tenant")
	tenantRepo.tenantUsers[tenantRecord.ID] = []tenant.TenantUser{{TenantID: tenantRecord.ID, UserID: "user-1", Role: tenant.RoleViewer}}
	h := &Handlers{tenantService: tenant.NewServiceWithRepository(tenantRepo)}
	claims := &auth.Claims{UserID: "user-1", TenantID: tenantRecord.ID, Role: tenant.RoleViewer}

	req := makeAuthenticatedRequest(http.MethodPost, "/tenants/"+tenantRecord.ID+"/plugins/"+uuid.New().String()+"/enable", nil, claims)
	req = withURLParams(req, map[string]string{"tenantID": tenantRecord.ID, "pluginID": uuid.New().String()})
	rr := httptest.NewRecorder()
	h.EnableTenantPlugin(rr, req)
	assert.Equal(t, http.StatusForbidden, rr.Code)

	req = makeAuthenticatedRequest(http.MethodPut, "/tenants/"+tenantRecord.ID+"/plugins/"+uuid.New().String()+"/settings", json.RawMessage(`{`), claims)
	req = withURLParams(req, map[string]string{"tenantID": tenantRecord.ID, "pluginID": uuid.New().String()})
	rr = httptest.NewRecorder()
	h.UpdateTenantPluginSettings(rr, req)
	assert.Equal(t, http.StatusForbidden, rr.Code)
}
