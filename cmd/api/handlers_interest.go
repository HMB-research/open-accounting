package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"

	"github.com/HMB-research/open-accounting/internal/invoicing"
	"github.com/HMB-research/open-accounting/internal/tenant"
)

// GetInvoiceInterest calculates and returns current interest for an invoice
// @Summary Get invoice interest
// @Tags Interest
// @Param tenantID path string true "Tenant ID"
// @Param invoiceID path string true "Invoice ID"
// @Success 200 {object} invoicing.InterestCalculationResult
// @Router /api/v1/tenants/{tenantID}/invoices/{invoiceID}/interest [get]
func (h *Handlers) GetInvoiceInterest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID := chi.URLParam(r, "tenantID")
	invoiceID := chi.URLParam(r, "invoiceID")
	schemaName := h.getSchemaName(ctx, tenantID)

	// Get tenant settings for interest rate
	tenant, err := h.tenantService.GetTenant(ctx, tenantID)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Tenant not found")
		return
	}

	interestRate := tenant.Settings.LatePaymentInterestRate
	if interestRate == 0 {
		interestRate = 0.0005 // Default 0.05% daily
	}

	result, err := h.interestService.CalculateInterest(ctx, schemaName, tenantID, invoiceID, interestRate, time.Now())
	if err != nil {
		if _, ok := err.(*invoicing.NotFoundError); ok {
			respondError(w, http.StatusNotFound, "Invoice not found")
			return
		}
		log.Error().Err(err).Msg("Failed to calculate interest")
		respondError(w, http.StatusInternalServerError, "Failed to calculate interest")
		return
	}

	respondJSON(w, http.StatusOK, result)
}

// GetInvoiceInterestHistory returns the interest calculation history for an invoice
// @Summary Get invoice interest history
// @Tags Interest
// @Param tenantID path string true "Tenant ID"
// @Param invoiceID path string true "Invoice ID"
// @Success 200 {array} invoicing.InvoiceInterest
// @Router /api/v1/tenants/{tenantID}/invoices/{invoiceID}/interest/history [get]
func (h *Handlers) GetInvoiceInterestHistory(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID := chi.URLParam(r, "tenantID")
	invoiceID := chi.URLParam(r, "invoiceID")
	schemaName := h.getSchemaName(ctx, tenantID)

	_ = tenantID // Used for schema lookup

	history, err := h.interestService.ListInterestHistory(ctx, schemaName, invoiceID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get interest history")
		respondError(w, http.StatusInternalServerError, "Failed to get interest history")
		return
	}

	if history == nil {
		history = []invoicing.InvoiceInterest{}
	}

	respondJSON(w, http.StatusOK, history)
}

// GetInterestSettings returns the interest settings for a tenant
// @Summary Get interest settings
// @Tags Interest
// @Param tenantID path string true "Tenant ID"
// @Success 200 {object} invoicing.InterestSettings
// @Router /api/v1/tenants/{tenantID}/settings/interest [get]
func (h *Handlers) GetInterestSettings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID := chi.URLParam(r, "tenantID")

	tenant, err := h.tenantService.GetTenant(ctx, tenantID)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Tenant not found")
		return
	}

	rate := tenant.Settings.LatePaymentInterestRate
	settings := invoicing.InterestSettings{
		Rate:        rate,
		AnnualRate:  rate * 365,
		IsEnabled:   rate > 0,
		Description: formatInterestDescription(rate),
	}

	respondJSON(w, http.StatusOK, settings)
}

// UpdateInterestSettings updates the interest settings for a tenant
// @Summary Update interest settings
// @Tags Interest
// @Param tenantID path string true "Tenant ID"
// @Param body body invoicing.UpdateInterestSettingsRequest true "Interest settings"
// @Success 200 {object} invoicing.InterestSettings
// @Router /api/v1/tenants/{tenantID}/settings/interest [put]
func (h *Handlers) UpdateInterestSettings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID := chi.URLParam(r, "tenantID")

	var req invoicing.UpdateInterestSettingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if err := req.Validate(); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Get current tenant
	currentTenant, err := h.tenantService.GetTenant(ctx, tenantID)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Tenant not found")
		return
	}

	// Update settings
	currentTenant.Settings.LatePaymentInterestRate = req.Rate

	updateReq := &tenant.UpdateTenantRequest{
		Name:     &currentTenant.Name,
		Settings: &currentTenant.Settings,
	}

	if _, err := h.tenantService.UpdateTenant(ctx, tenantID, updateReq); err != nil {
		log.Error().Err(err).Msg("Failed to update interest settings")
		respondError(w, http.StatusInternalServerError, "Failed to update settings")
		return
	}

	settings := invoicing.InterestSettings{
		Rate:        req.Rate,
		AnnualRate:  req.Rate * 365,
		IsEnabled:   req.Rate > 0,
		Description: formatInterestDescription(req.Rate),
	}

	respondJSON(w, http.StatusOK, settings)
}

// GetOverdueInvoicesWithInterest returns all overdue invoices with calculated interest
// @Summary Get overdue invoices with interest
// @Tags Interest
// @Param tenantID path string true "Tenant ID"
// @Success 200 {array} invoicing.InterestCalculationResult
// @Router /api/v1/tenants/{tenantID}/invoices/overdue-with-interest [get]
func (h *Handlers) GetOverdueInvoicesWithInterest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID := chi.URLParam(r, "tenantID")
	schemaName := h.getSchemaName(ctx, tenantID)

	// Get tenant settings for interest rate
	tenant, err := h.tenantService.GetTenant(ctx, tenantID)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Tenant not found")
		return
	}

	interestRate := tenant.Settings.LatePaymentInterestRate
	if interestRate == 0 {
		interestRate = 0.0005 // Default 0.05% daily
	}

	results, err := h.interestService.CalculateInterestForOverdueInvoices(ctx, schemaName, tenantID, interestRate)
	if err != nil {
		log.Error().Err(err).Msg("Failed to calculate interest for overdue invoices")
		respondError(w, http.StatusInternalServerError, "Failed to calculate interest")
		return
	}

	if results == nil {
		results = []invoicing.InterestCalculationResult{}
	}

	respondJSON(w, http.StatusOK, results)
}

func formatInterestDescription(rate float64) string {
	if rate == 0 {
		return "Interest calculation disabled"
	}
	dailyPercent := rate * 100
	annualPercent := rate * 365 * 100
	return formatFloat(dailyPercent, 3) + "% daily (" + formatFloat(annualPercent, 1) + "% annually)"
}

func formatFloat(f float64, precision int) string {
	format := fmt.Sprintf("%%.%df", precision)
	return fmt.Sprintf(format, f)
}
