package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/shopspring/decimal"

	"github.com/HMB-research/open-accounting/internal/payroll"
)

func TestCalculateTaxPreviewHandlerUsesBasicExemptionAmount(t *testing.T) {
	h := &Handlers{}

	req := makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/payroll/tax-preview", map[string]any{
		"gross_salary":           "3000.00",
		"apply_basic_exemption":  true,
		"basic_exemption_amount": "500.00",
		"funded_pension_rate":    "0.02",
	}, nil)
	w := httptest.NewRecorder()

	h.CalculateTaxPreview(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
	var result payroll.TaxCalculation
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !result.BasicExemption.Equal(decimal.RequireFromString("500.00")) {
		t.Fatalf("basic exemption = %s, want 500.00", result.BasicExemption)
	}
	if !result.TaxableIncome.Equal(decimal.RequireFromString("2500.00")) {
		t.Fatalf("taxable income = %s, want 2500.00", result.TaxableIncome)
	}
}

func TestCalculateTaxPreviewHandlerRejectsNegativeBasicExemptionAmount(t *testing.T) {
	h := &Handlers{}

	req := makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/payroll/tax-preview", map[string]any{
		"gross_salary":           "3000.00",
		"apply_basic_exemption":  true,
		"basic_exemption_amount": "-1.00",
		"funded_pension_rate":    "0.02",
	}, nil)
	w := httptest.NewRecorder()

	h.CalculateTaxPreview(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", w.Code)
	}
}
