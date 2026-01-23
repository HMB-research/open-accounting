package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/shopspring/decimal"

	"github.com/HMB-research/open-accounting/internal/analytics"
	"github.com/HMB-research/open-accounting/internal/auth"
	"github.com/HMB-research/open-accounting/internal/contacts"
	"github.com/HMB-research/open-accounting/internal/invoicing"
	"github.com/HMB-research/open-accounting/internal/payments"
	"github.com/HMB-research/open-accounting/internal/tenant"
)

// =============================================================================
// Test Helpers
// =============================================================================

// createTestClaims creates JWT claims for testing with the given parameters
func createTestClaims(userID, email, tenantID, role string) *auth.Claims {
	return &auth.Claims{
		UserID:   userID,
		Email:    email,
		TenantID: tenantID,
		Role:     role,
	}
}

// contextWithClaims adds JWT claims to a context
func contextWithClaims(ctx context.Context, claims *auth.Claims) context.Context {
	return context.WithValue(ctx, auth.ClaimsContextKey, claims)
}

// makeAuthenticatedRequest creates an HTTP request with authentication context
func makeAuthenticatedRequest(method, path string, body interface{}, claims *auth.Claims) *http.Request {
	var bodyReader *bytes.Buffer
	if body != nil {
		jsonBody, _ := json.Marshal(body)
		bodyReader = bytes.NewBuffer(jsonBody)
	} else {
		bodyReader = bytes.NewBuffer(nil)
	}

	req := httptest.NewRequest(method, path, bodyReader)
	req.Header.Set("Content-Type", "application/json")

	if claims != nil {
		ctx := contextWithClaims(req.Context(), claims)
		req = req.WithContext(ctx)
	}

	return req
}

// withURLParams adds chi URL parameters to a request context
func withURLParams(r *http.Request, params map[string]string) *http.Request {
	rctx := chi.NewRouteContext()
	for key, value := range params {
		rctx.URLParams.Add(key, value)
	}
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

// assertJSONResponse checks common JSON response patterns
func assertJSONResponse(t interface{ Helper(); Errorf(string, ...interface{}) }, w *httptest.ResponseRecorder, expectedStatus int) map[string]interface{} {
	t.Helper()
	if w.Code != expectedStatus {
		t.Errorf("expected status %d, got %d: %s", expectedStatus, w.Code, w.Body.String())
	}

	if w.Body.Len() == 0 {
		return nil
	}

	var result map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		// Try to decode as array or other type
		return nil
	}
	return result
}

// createTestTokenService creates a token service for testing
func createTestTokenService() *auth.TokenService {
	return auth.NewTokenService("test-secret-key-for-testing-only", 15*time.Minute, 7*24*time.Hour)
}

// =============================================================================
// Test Data Helpers
// =============================================================================

// createTestTenant creates a tenant struct for testing
func createTestTenant(id, name, slug string) *tenant.Tenant {
	return &tenant.Tenant{
		ID:         id,
		Name:       name,
		Slug:       slug,
		SchemaName: "tenant_" + slug,
		IsActive:   true,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
}

// createTestUser creates a user struct for testing
func createTestUser(id, email, name string) *tenant.User {
	return &tenant.User{
		ID:        id,
		Email:     email,
		Name:      name,
		IsActive:  true,
		CreatedAt: time.Now(),
	}
}

// createTestContact creates a contact struct for testing
func createTestContact(id, tenantID, name string, contactType contacts.ContactType) *contacts.Contact {
	return &contacts.Contact{
		ID:               id,
		TenantID:         tenantID,
		Name:             name,
		ContactType:      contactType,
		CountryCode:      "EE",
		PaymentTermsDays: 14,
		IsActive:         true,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}
}

// createTestInvoice creates an invoice struct for testing
func createTestInvoice(id, tenantID, contactID string, invType invoicing.InvoiceType, status invoicing.InvoiceStatus) *invoicing.Invoice {
	return &invoicing.Invoice{
		ID:            id,
		TenantID:      tenantID,
		InvoiceNumber: "INV-001",
		InvoiceType:   invType,
		ContactID:     contactID,
		IssueDate:     time.Now(),
		DueDate:       time.Now().AddDate(0, 0, 14),
		Currency:      "EUR",
		ExchangeRate:  decimal.NewFromInt(1),
		Status:        status,
		Subtotal:      decimal.NewFromInt(100),
		VATAmount:     decimal.NewFromInt(20),
		Total:         decimal.NewFromInt(120),
		AmountPaid:    decimal.Zero,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
}

// createTestPayment creates a payment struct for testing
func createTestPayment(id, tenantID string, paymentType payments.PaymentType, amount decimal.Decimal) *payments.Payment {
	return &payments.Payment{
		ID:            id,
		TenantID:      tenantID,
		PaymentNumber: "PAY-001",
		PaymentType:   paymentType,
		Amount:        amount,
		Currency:      "EUR",
		ExchangeRate:  decimal.NewFromInt(1),
		BaseAmount:    amount,
		PaymentDate:   time.Now(),
		CreatedAt:     time.Now(),
	}
}

// createTestActivity creates activity items for testing
func createTestActivity() []analytics.ActivityItem {
	return []analytics.ActivityItem{
		{Type: "invoice", Description: "Invoice INV-001 created", CreatedAt: time.Now().Add(-1 * time.Hour)},
		{Type: "payment", Description: "Payment received", CreatedAt: time.Now().Add(-2 * time.Hour)},
	}
}

// createTestDashboardSummary creates a dashboard summary for testing
func createTestDashboardSummary() *analytics.DashboardSummary {
	return &analytics.DashboardSummary{
		TotalRevenue:     decimal.NewFromInt(50000),
		TotalExpenses:    decimal.NewFromInt(30000),
		NetIncome:        decimal.NewFromInt(20000),
		TotalReceivables: decimal.NewFromInt(15000),
		TotalPayables:    decimal.NewFromInt(10000),
		PeriodStart:      time.Now().AddDate(0, -1, 0),
		PeriodEnd:        time.Now(),
	}
}
