package main

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/HMB-research/open-accounting/internal/recurring"
)

// ListRecurringInvoices returns all recurring invoices for a tenant
// @Summary List recurring invoices
// @Description Get all recurring invoice templates for a tenant
// @Tags Recurring
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param active_only query bool false "Filter for active recurring invoices only"
// @Success 200 {array} recurring.RecurringInvoice
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/recurring-invoices [get]
func (h *Handlers) ListRecurringInvoices(w http.ResponseWriter, r *http.Request) {
	tenantCtx := h.tenantContextFromRequest(r)
	activeOnly := r.URL.Query().Get("active_only") == "true"

	invoices, err := h.recurringService.List(r.Context(), tenantCtx.tenantID, tenantCtx.schemaName, activeOnly)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to list recurring invoices")
		return
	}

	respondJSON(w, http.StatusOK, invoices)
}

// CreateRecurringInvoice creates a new recurring invoice
// @Summary Create recurring invoice
// @Description Create a new recurring invoice template
// @Tags Recurring
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param request body recurring.CreateRecurringInvoiceRequest true "Recurring invoice details"
// @Success 201 {object} recurring.RecurringInvoice
// @Failure 400 {object} object{error=string}
// @Router /tenants/{tenantID}/recurring-invoices [post]
func (h *Handlers) CreateRecurringInvoice(w http.ResponseWriter, r *http.Request) {
	tenantCtx := h.tenantContextFromRequest(r)

	var req recurring.CreateRecurringInvoiceRequest
	if !decodeJSONRequest(w, r, &req) {
		return
	}

	req.UserID = userIDFromRequest(r)

	invoice, err := h.recurringService.Create(r.Context(), tenantCtx.tenantID, tenantCtx.schemaName, &req)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, invoice)
}

// CreateRecurringInvoiceFromInvoice creates a recurring invoice from an existing invoice
// @Summary Create recurring invoice from existing invoice
// @Description Create a new recurring invoice template based on an existing invoice
// @Tags Recurring
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param invoiceID path string true "Invoice ID to use as template"
// @Param request body recurring.CreateFromInvoiceRequest true "Recurring invoice settings"
// @Success 201 {object} recurring.RecurringInvoice
// @Failure 400 {object} object{error=string}
// @Router /tenants/{tenantID}/recurring-invoices/from-invoice/{invoiceID} [post]
func (h *Handlers) CreateRecurringInvoiceFromInvoice(w http.ResponseWriter, r *http.Request) {
	tenantCtx := h.tenantContextFromRequest(r)
	invoiceID := chi.URLParam(r, "invoiceID")

	var req recurring.CreateFromInvoiceRequest
	if !decodeJSONRequest(w, r, &req) {
		return
	}

	req.InvoiceID = invoiceID
	req.UserID = userIDFromRequest(r)

	invoice, err := h.recurringService.CreateFromInvoice(r.Context(), tenantCtx.tenantID, tenantCtx.schemaName, &req)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, invoice)
}

// GetRecurringInvoice returns a recurring invoice by ID
// @Summary Get recurring invoice
// @Description Get recurring invoice details by ID
// @Tags Recurring
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param recurringID path string true "Recurring Invoice ID"
// @Success 200 {object} recurring.RecurringInvoice
// @Failure 404 {object} object{error=string}
// @Router /tenants/{tenantID}/recurring-invoices/{recurringID} [get]
func (h *Handlers) GetRecurringInvoice(w http.ResponseWriter, r *http.Request) {
	tenantCtx := h.tenantContextFromRequest(r)
	recurringID := chi.URLParam(r, "recurringID")

	invoice, err := h.recurringService.GetByID(r.Context(), tenantCtx.tenantID, tenantCtx.schemaName, recurringID)
	if err != nil {
		respondError(w, http.StatusNotFound, "Recurring invoice not found")
		return
	}

	respondJSON(w, http.StatusOK, invoice)
}

// UpdateRecurringInvoice updates a recurring invoice
// @Summary Update recurring invoice
// @Description Update recurring invoice details
// @Tags Recurring
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param recurringID path string true "Recurring Invoice ID"
// @Param request body recurring.UpdateRecurringInvoiceRequest true "Updated details"
// @Success 200 {object} recurring.RecurringInvoice
// @Failure 400 {object} object{error=string}
// @Router /tenants/{tenantID}/recurring-invoices/{recurringID} [put]
func (h *Handlers) UpdateRecurringInvoice(w http.ResponseWriter, r *http.Request) {
	tenantCtx := h.tenantContextFromRequest(r)
	recurringID := chi.URLParam(r, "recurringID")

	var req recurring.UpdateRecurringInvoiceRequest
	if !decodeJSONRequest(w, r, &req) {
		return
	}

	invoice, err := h.recurringService.Update(r.Context(), tenantCtx.tenantID, tenantCtx.schemaName, recurringID, &req)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, invoice)
}

// DeleteRecurringInvoice deletes a recurring invoice
// @Summary Delete recurring invoice
// @Description Delete a recurring invoice template
// @Tags Recurring
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param recurringID path string true "Recurring Invoice ID"
// @Success 200 {object} object{status=string}
// @Failure 400 {object} object{error=string}
// @Router /tenants/{tenantID}/recurring-invoices/{recurringID} [delete]
func (h *Handlers) DeleteRecurringInvoice(w http.ResponseWriter, r *http.Request) {
	tenantCtx := h.tenantContextFromRequest(r)
	recurringID := chi.URLParam(r, "recurringID")

	if err := h.recurringService.Delete(r.Context(), tenantCtx.tenantID, tenantCtx.schemaName, recurringID); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// PauseRecurringInvoice pauses a recurring invoice
// @Summary Pause recurring invoice
// @Description Pause automatic generation of a recurring invoice
// @Tags Recurring
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param recurringID path string true "Recurring Invoice ID"
// @Success 200 {object} object{status=string}
// @Failure 400 {object} object{error=string}
// @Router /tenants/{tenantID}/recurring-invoices/{recurringID}/pause [post]
func (h *Handlers) PauseRecurringInvoice(w http.ResponseWriter, r *http.Request) {
	tenantCtx := h.tenantContextFromRequest(r)
	recurringID := chi.URLParam(r, "recurringID")

	if err := h.recurringService.Pause(r.Context(), tenantCtx.tenantID, tenantCtx.schemaName, recurringID); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "paused"})
}

// ResumeRecurringInvoice resumes a paused recurring invoice
// @Summary Resume recurring invoice
// @Description Resume automatic generation of a paused recurring invoice
// @Tags Recurring
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param recurringID path string true "Recurring Invoice ID"
// @Success 200 {object} object{status=string}
// @Failure 400 {object} object{error=string}
// @Router /tenants/{tenantID}/recurring-invoices/{recurringID}/resume [post]
func (h *Handlers) ResumeRecurringInvoice(w http.ResponseWriter, r *http.Request) {
	tenantCtx := h.tenantContextFromRequest(r)
	recurringID := chi.URLParam(r, "recurringID")

	if err := h.recurringService.Resume(r.Context(), tenantCtx.tenantID, tenantCtx.schemaName, recurringID); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "resumed"})
}

// GenerateRecurringInvoice manually generates an invoice from a recurring invoice
// @Summary Generate invoice from recurring template
// @Description Manually trigger generation of an invoice from a recurring invoice
// @Tags Recurring
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param recurringID path string true "Recurring Invoice ID"
// @Success 200 {object} recurring.GenerationResult
// @Failure 400 {object} object{error=string}
// @Router /tenants/{tenantID}/recurring-invoices/{recurringID}/generate [post]
func (h *Handlers) GenerateRecurringInvoice(w http.ResponseWriter, r *http.Request) {
	tenantCtx := h.tenantContextFromRequest(r)
	recurringID := chi.URLParam(r, "recurringID")

	result, err := h.recurringService.GenerateInvoice(r.Context(), tenantCtx.tenantID, tenantCtx.schemaName, recurringID, userIDFromRequest(r))
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, result)
}

// GenerateDueRecurringInvoices generates all due recurring invoices
// @Summary Generate all due invoices
// @Description Trigger generation of all recurring invoices that are due
// @Tags Recurring
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Success 200 {array} recurring.GenerationResult
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/recurring-invoices/generate-due [post]
func (h *Handlers) GenerateDueRecurringInvoices(w http.ResponseWriter, r *http.Request) {
	tenantCtx := h.tenantContextFromRequest(r)

	results, err := h.recurringService.GenerateDueInvoices(r.Context(), tenantCtx.tenantID, tenantCtx.schemaName, userIDFromRequest(r))
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to generate invoices")
		return
	}

	respondJSON(w, http.StatusOK, results)
}
