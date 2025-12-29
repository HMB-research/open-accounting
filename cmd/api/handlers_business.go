package main

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/shopspring/decimal"

	"github.com/HMB-research/open-accounting/internal/auth"
	"github.com/HMB-research/open-accounting/internal/banking"
	"github.com/HMB-research/open-accounting/internal/contacts"
	"github.com/HMB-research/open-accounting/internal/email"
	"github.com/HMB-research/open-accounting/internal/invoicing"
	"github.com/HMB-research/open-accounting/internal/payments"
	"github.com/HMB-research/open-accounting/internal/payroll"
	"github.com/HMB-research/open-accounting/internal/recurring"
	"github.com/HMB-research/open-accounting/internal/tenant"
)

// =============================================================================
// ANALYTICS HANDLERS
// =============================================================================

// GetDashboardSummary returns key metrics for the dashboard
// @Summary Get dashboard summary
// @Description Get key metrics including revenue, expenses, receivables, and invoice counts
// @Tags Analytics
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Success 200 {object} analytics.DashboardSummary
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/analytics/dashboard [get]
func (h *Handlers) GetDashboardSummary(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	summary, err := h.analyticsService.GetDashboardSummary(r.Context(), tenantID, schemaName)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get dashboard summary")
		return
	}

	respondJSON(w, http.StatusOK, summary)
}

// GetRevenueExpenseChart returns monthly revenue vs expense data
// @Summary Get revenue/expense chart data
// @Description Get monthly revenue and expense data for charting
// @Tags Analytics
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param months query int false "Number of months (default 12)"
// @Success 200 {object} analytics.RevenueExpenseChart
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/analytics/revenue-expense [get]
func (h *Handlers) GetRevenueExpenseChart(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	months := 12
	if m := r.URL.Query().Get("months"); m != "" {
		if parsed, err := parseIntParam(m); err == nil && parsed > 0 {
			months = parsed
		}
	}

	chart, err := h.analyticsService.GetRevenueExpenseChart(r.Context(), tenantID, schemaName, months)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get chart data")
		return
	}

	respondJSON(w, http.StatusOK, chart)
}

// GetCashFlowChart returns monthly cash flow data
// @Summary Get cash flow chart data
// @Description Get monthly cash inflows and outflows for charting
// @Tags Analytics
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param months query int false "Number of months (default 12)"
// @Success 200 {object} analytics.CashFlowChart
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/analytics/cash-flow [get]
func (h *Handlers) GetCashFlowChart(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	months := 12
	if m := r.URL.Query().Get("months"); m != "" {
		if parsed, err := parseIntParam(m); err == nil && parsed > 0 {
			months = parsed
		}
	}

	chart, err := h.analyticsService.GetCashFlowChart(r.Context(), tenantID, schemaName, months)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get chart data")
		return
	}

	respondJSON(w, http.StatusOK, chart)
}

// GetReceivablesAging returns aging report for receivables
// @Summary Get receivables aging report
// @Description Get aging breakdown for accounts receivable
// @Tags Reports
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Success 200 {object} analytics.AgingReport
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/reports/aging/receivables [get]
func (h *Handlers) GetReceivablesAging(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	report, err := h.analyticsService.GetReceivablesAging(r.Context(), tenantID, schemaName)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get aging report")
		return
	}

	respondJSON(w, http.StatusOK, report)
}

// GetPayablesAging returns aging report for payables
// @Summary Get payables aging report
// @Description Get aging breakdown for accounts payable
// @Tags Reports
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Success 200 {object} analytics.AgingReport
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/reports/aging/payables [get]
func (h *Handlers) GetPayablesAging(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	report, err := h.analyticsService.GetPayablesAging(r.Context(), tenantID, schemaName)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get aging report")
		return
	}

	respondJSON(w, http.StatusOK, report)
}

func parseIntParam(s string) (int, error) {
	var i int
	_, err := fmt.Sscanf(s, "%d", &i)
	return i, err
}

// =============================================================================
// CONTACTS HANDLERS
// =============================================================================

// ListContacts returns all contacts for a tenant
// @Summary List contacts
// @Description Get all contacts (customers/suppliers) for a tenant
// @Tags Contacts
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param active_only query bool false "Filter for active contacts only"
// @Param type query string false "Filter by contact type (CUSTOMER, SUPPLIER, BOTH)"
// @Param search query string false "Search by name or email"
// @Success 200 {array} contacts.Contact
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/contacts [get]
func (h *Handlers) ListContacts(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	filter := &contacts.ContactFilter{
		ActiveOnly: r.URL.Query().Get("active_only") == "true",
		Search:     r.URL.Query().Get("search"),
	}

	if contactType := r.URL.Query().Get("type"); contactType != "" {
		filter.ContactType = contacts.ContactType(contactType)
	}

	contactsList, err := h.contactsService.List(r.Context(), tenantID, schemaName, filter)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to list contacts")
		return
	}

	respondJSON(w, http.StatusOK, contactsList)
}

// CreateContact creates a new contact
// @Summary Create contact
// @Description Create a new contact (customer/supplier)
// @Tags Contacts
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param request body contacts.CreateContactRequest true "Contact details"
// @Success 201 {object} contacts.Contact
// @Failure 400 {object} object{error=string}
// @Router /tenants/{tenantID}/contacts [post]
func (h *Handlers) CreateContact(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	var req contacts.CreateContactRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Name == "" {
		respondError(w, http.StatusBadRequest, "Name is required")
		return
	}

	if req.ContactType == "" {
		req.ContactType = contacts.ContactTypeCustomer
	}

	contact, err := h.contactsService.Create(r.Context(), tenantID, schemaName, &req)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, contact)
}

// GetContact returns a contact by ID
// @Summary Get contact
// @Description Get contact details by ID
// @Tags Contacts
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param contactID path string true "Contact ID"
// @Success 200 {object} contacts.Contact
// @Failure 404 {object} object{error=string}
// @Router /tenants/{tenantID}/contacts/{contactID} [get]
func (h *Handlers) GetContact(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	contactID := chi.URLParam(r, "contactID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	contact, err := h.contactsService.GetByID(r.Context(), tenantID, schemaName, contactID)
	if err != nil {
		respondError(w, http.StatusNotFound, "Contact not found")
		return
	}

	respondJSON(w, http.StatusOK, contact)
}

// UpdateContact updates a contact
// @Summary Update contact
// @Description Update contact details
// @Tags Contacts
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param contactID path string true "Contact ID"
// @Param request body contacts.UpdateContactRequest true "Updated contact details"
// @Success 200 {object} contacts.Contact
// @Failure 400 {object} object{error=string}
// @Router /tenants/{tenantID}/contacts/{contactID} [put]
func (h *Handlers) UpdateContact(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	contactID := chi.URLParam(r, "contactID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	var req contacts.UpdateContactRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	contact, err := h.contactsService.Update(r.Context(), tenantID, schemaName, contactID, &req)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, contact)
}

// DeleteContact deactivates a contact
// @Summary Delete contact
// @Description Deactivate a contact (soft delete)
// @Tags Contacts
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param contactID path string true "Contact ID"
// @Success 200 {object} object{status=string}
// @Failure 400 {object} object{error=string}
// @Router /tenants/{tenantID}/contacts/{contactID} [delete]
func (h *Handlers) DeleteContact(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	contactID := chi.URLParam(r, "contactID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	if err := h.contactsService.Delete(r.Context(), tenantID, schemaName, contactID); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// =============================================================================
// INVOICES HANDLERS
// =============================================================================

// ListInvoices returns all invoices for a tenant
// @Summary List invoices
// @Description Get all invoices for a tenant with optional filtering
// @Tags Invoices
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param type query string false "Filter by invoice type (SALES, PURCHASE)"
// @Param status query string false "Filter by status (DRAFT, SENT, PAID, PARTIALLY_PAID, VOID)"
// @Param contact_id query string false "Filter by contact ID"
// @Param from_date query string false "Filter from date (YYYY-MM-DD)"
// @Param to_date query string false "Filter to date (YYYY-MM-DD)"
// @Param search query string false "Search by invoice number"
// @Success 200 {array} invoicing.Invoice
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/invoices [get]
func (h *Handlers) ListInvoices(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	filter := &invoicing.InvoiceFilter{
		Search: r.URL.Query().Get("search"),
	}

	if invType := r.URL.Query().Get("type"); invType != "" {
		filter.InvoiceType = invoicing.InvoiceType(invType)
	}
	if status := r.URL.Query().Get("status"); status != "" {
		filter.Status = invoicing.InvoiceStatus(status)
	}
	if contactID := r.URL.Query().Get("contact_id"); contactID != "" {
		filter.ContactID = contactID
	}
	if fromDate := r.URL.Query().Get("from_date"); fromDate != "" {
		if parsed, err := time.Parse("2006-01-02", fromDate); err == nil {
			filter.FromDate = &parsed
		}
	}
	if toDate := r.URL.Query().Get("to_date"); toDate != "" {
		if parsed, err := time.Parse("2006-01-02", toDate); err == nil {
			filter.ToDate = &parsed
		}
	}

	invoices, err := h.invoicingService.List(r.Context(), tenantID, schemaName, filter)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to list invoices")
		return
	}

	respondJSON(w, http.StatusOK, invoices)
}

// CreateInvoice creates a new invoice
// @Summary Create invoice
// @Description Create a new sales or purchase invoice
// @Tags Invoices
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param request body invoicing.CreateInvoiceRequest true "Invoice details"
// @Success 201 {object} invoicing.Invoice
// @Failure 400 {object} object{error=string}
// @Router /tenants/{tenantID}/invoices [post]
func (h *Handlers) CreateInvoice(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.GetClaims(r.Context())
	tenantID := chi.URLParam(r, "tenantID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	var req invoicing.CreateInvoiceRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	req.UserID = claims.UserID

	if req.ContactID == "" {
		respondError(w, http.StatusBadRequest, "Contact is required")
		return
	}

	if len(req.Lines) == 0 {
		respondError(w, http.StatusBadRequest, "At least one line is required")
		return
	}

	invoice, err := h.invoicingService.Create(r.Context(), tenantID, schemaName, &req)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, invoice)
}

// GetInvoice returns an invoice by ID
// @Summary Get invoice
// @Description Get invoice details by ID
// @Tags Invoices
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param invoiceID path string true "Invoice ID"
// @Success 200 {object} invoicing.Invoice
// @Failure 404 {object} object{error=string}
// @Router /tenants/{tenantID}/invoices/{invoiceID} [get]
func (h *Handlers) GetInvoice(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	invoiceID := chi.URLParam(r, "invoiceID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	invoice, err := h.invoicingService.GetByID(r.Context(), tenantID, schemaName, invoiceID)
	if err != nil {
		respondError(w, http.StatusNotFound, "Invoice not found")
		return
	}

	respondJSON(w, http.StatusOK, invoice)
}

// SendInvoice marks an invoice as sent
// @Summary Send invoice
// @Description Mark an invoice as sent to the customer
// @Tags Invoices
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param invoiceID path string true "Invoice ID"
// @Success 200 {object} object{status=string}
// @Failure 400 {object} object{error=string}
// @Router /tenants/{tenantID}/invoices/{invoiceID}/send [post]
func (h *Handlers) SendInvoice(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	invoiceID := chi.URLParam(r, "invoiceID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	if err := h.invoicingService.Send(r.Context(), tenantID, schemaName, invoiceID); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "sent"})
}

// VoidInvoice voids an invoice
// @Summary Void invoice
// @Description Void an invoice (cannot be undone)
// @Tags Invoices
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param invoiceID path string true "Invoice ID"
// @Success 200 {object} object{status=string}
// @Failure 400 {object} object{error=string}
// @Router /tenants/{tenantID}/invoices/{invoiceID}/void [post]
func (h *Handlers) VoidInvoice(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	invoiceID := chi.URLParam(r, "invoiceID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	if err := h.invoicingService.Void(r.Context(), tenantID, schemaName, invoiceID); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "voided"})
}

// GetInvoicePDF generates and returns a PDF for an invoice
// @Summary Download invoice PDF
// @Description Generate and download a PDF for an invoice
// @Tags Invoices
// @Produce application/pdf
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param invoiceID path string true "Invoice ID"
// @Success 200 {file} binary
// @Failure 404 {object} object{error=string}
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/invoices/{invoiceID}/pdf [get]
func (h *Handlers) GetInvoicePDF(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	invoiceID := chi.URLParam(r, "invoiceID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	// Get invoice with contact
	invoice, err := h.invoicingService.GetByID(r.Context(), tenantID, schemaName, invoiceID)
	if err != nil {
		respondError(w, http.StatusNotFound, "Invoice not found")
		return
	}

	// Get tenant for company details
	t, err := h.tenantService.GetTenant(r.Context(), tenantID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get tenant")
		return
	}

	// Get PDF settings from tenant
	pdfSettings := h.pdfService.PDFSettingsFromTenant(t)

	// Generate PDF
	pdfBytes, err := h.pdfService.GenerateInvoicePDF(invoice, t, pdfSettings)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to generate PDF")
		return
	}

	// Set response headers for PDF download
	filename := "invoice-" + invoice.InvoiceNumber + ".pdf"
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", "attachment; filename=\""+filename+"\"")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(pdfBytes)))

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(pdfBytes)
}

// =============================================================================
// PAYMENTS HANDLERS
// =============================================================================

// ListPayments returns all payments for a tenant
// @Summary List payments
// @Description Get all payments for a tenant with optional filtering
// @Tags Payments
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param type query string false "Filter by payment type (RECEIVED, MADE)"
// @Param contact_id query string false "Filter by contact ID"
// @Param from_date query string false "Filter from date (YYYY-MM-DD)"
// @Param to_date query string false "Filter to date (YYYY-MM-DD)"
// @Success 200 {array} payments.Payment
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/payments [get]
func (h *Handlers) ListPayments(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	filter := &payments.PaymentFilter{}

	if payType := r.URL.Query().Get("type"); payType != "" {
		filter.PaymentType = payments.PaymentType(payType)
	}
	if contactID := r.URL.Query().Get("contact_id"); contactID != "" {
		filter.ContactID = contactID
	}
	if fromDate := r.URL.Query().Get("from_date"); fromDate != "" {
		if parsed, err := time.Parse("2006-01-02", fromDate); err == nil {
			filter.FromDate = &parsed
		}
	}
	if toDate := r.URL.Query().Get("to_date"); toDate != "" {
		if parsed, err := time.Parse("2006-01-02", toDate); err == nil {
			filter.ToDate = &parsed
		}
	}

	paymentsList, err := h.paymentsService.List(r.Context(), tenantID, schemaName, filter)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to list payments")
		return
	}

	respondJSON(w, http.StatusOK, paymentsList)
}

// CreatePayment creates a new payment
// @Summary Create payment
// @Description Record a new payment received or made
// @Tags Payments
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param request body payments.CreatePaymentRequest true "Payment details"
// @Success 201 {object} payments.Payment
// @Failure 400 {object} object{error=string}
// @Router /tenants/{tenantID}/payments [post]
func (h *Handlers) CreatePayment(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.GetClaims(r.Context())
	tenantID := chi.URLParam(r, "tenantID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	var req payments.CreatePaymentRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	req.UserID = claims.UserID

	if req.Amount.LessThanOrEqual(decimal.Zero) {
		respondError(w, http.StatusBadRequest, "Amount must be positive")
		return
	}

	payment, err := h.paymentsService.Create(r.Context(), tenantID, schemaName, &req)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, payment)
}

// GetPayment returns a payment by ID
// @Summary Get payment
// @Description Get payment details by ID
// @Tags Payments
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param paymentID path string true "Payment ID"
// @Success 200 {object} payments.Payment
// @Failure 404 {object} object{error=string}
// @Router /tenants/{tenantID}/payments/{paymentID} [get]
func (h *Handlers) GetPayment(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	paymentID := chi.URLParam(r, "paymentID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	payment, err := h.paymentsService.GetByID(r.Context(), tenantID, schemaName, paymentID)
	if err != nil {
		respondError(w, http.StatusNotFound, "Payment not found")
		return
	}

	respondJSON(w, http.StatusOK, payment)
}

// AllocatePayment allocates a payment to an invoice
// @Summary Allocate payment
// @Description Allocate a payment amount to an invoice
// @Tags Payments
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param paymentID path string true "Payment ID"
// @Param request body object{invoice_id=string,amount=string} true "Allocation details"
// @Success 200 {object} object{status=string}
// @Failure 400 {object} object{error=string}
// @Router /tenants/{tenantID}/payments/{paymentID}/allocate [post]
func (h *Handlers) AllocatePayment(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	paymentID := chi.URLParam(r, "paymentID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	var req struct {
		InvoiceID string          `json:"invoice_id"`
		Amount    decimal.Decimal `json:"amount"`
	}
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.InvoiceID == "" {
		respondError(w, http.StatusBadRequest, "Invoice ID is required")
		return
	}

	if req.Amount.LessThanOrEqual(decimal.Zero) {
		respondError(w, http.StatusBadRequest, "Amount must be positive")
		return
	}

	if err := h.paymentsService.AllocateToInvoice(r.Context(), tenantID, schemaName, paymentID, req.InvoiceID, req.Amount); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "allocated"})
}

// GetUnallocatedPayments returns payments with unallocated balances
// @Summary Get unallocated payments
// @Description Get payments with remaining unallocated amounts
// @Tags Payments
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param type query string false "Filter by type (RECEIVED, MADE)"
// @Success 200 {array} payments.Payment
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/payments/unallocated [get]
func (h *Handlers) GetUnallocatedPayments(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	paymentType := payments.PaymentTypeReceived
	if r.URL.Query().Get("type") == "MADE" {
		paymentType = payments.PaymentTypeMade
	}

	paymentsList, err := h.paymentsService.GetUnallocatedPayments(r.Context(), tenantID, schemaName, paymentType)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get unallocated payments")
		return
	}

	respondJSON(w, http.StatusOK, paymentsList)
}

// =============================================================================
// RECURRING INVOICES HANDLERS
// =============================================================================

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
	tenantID := chi.URLParam(r, "tenantID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	activeOnly := r.URL.Query().Get("active_only") == "true"

	invoices, err := h.recurringService.List(r.Context(), tenantID, schemaName, activeOnly)
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
	claims, _ := auth.GetClaims(r.Context())
	tenantID := chi.URLParam(r, "tenantID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	var req recurring.CreateRecurringInvoiceRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	req.UserID = claims.UserID

	invoice, err := h.recurringService.Create(r.Context(), tenantID, schemaName, &req)
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
	claims, _ := auth.GetClaims(r.Context())
	tenantID := chi.URLParam(r, "tenantID")
	invoiceID := chi.URLParam(r, "invoiceID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	var req recurring.CreateFromInvoiceRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	req.InvoiceID = invoiceID
	req.UserID = claims.UserID

	invoice, err := h.recurringService.CreateFromInvoice(r.Context(), tenantID, schemaName, &req)
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
	tenantID := chi.URLParam(r, "tenantID")
	recurringID := chi.URLParam(r, "recurringID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	invoice, err := h.recurringService.GetByID(r.Context(), tenantID, schemaName, recurringID)
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
	tenantID := chi.URLParam(r, "tenantID")
	recurringID := chi.URLParam(r, "recurringID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	var req recurring.UpdateRecurringInvoiceRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	invoice, err := h.recurringService.Update(r.Context(), tenantID, schemaName, recurringID, &req)
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
	tenantID := chi.URLParam(r, "tenantID")
	recurringID := chi.URLParam(r, "recurringID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	if err := h.recurringService.Delete(r.Context(), tenantID, schemaName, recurringID); err != nil {
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
	tenantID := chi.URLParam(r, "tenantID")
	recurringID := chi.URLParam(r, "recurringID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	if err := h.recurringService.Pause(r.Context(), tenantID, schemaName, recurringID); err != nil {
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
	tenantID := chi.URLParam(r, "tenantID")
	recurringID := chi.URLParam(r, "recurringID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	if err := h.recurringService.Resume(r.Context(), tenantID, schemaName, recurringID); err != nil {
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
	claims, _ := auth.GetClaims(r.Context())
	tenantID := chi.URLParam(r, "tenantID")
	recurringID := chi.URLParam(r, "recurringID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	result, err := h.recurringService.GenerateInvoice(r.Context(), tenantID, schemaName, recurringID, claims.UserID)
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
	claims, _ := auth.GetClaims(r.Context())
	tenantID := chi.URLParam(r, "tenantID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	results, err := h.recurringService.GenerateDueInvoices(r.Context(), tenantID, schemaName, claims.UserID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to generate invoices")
		return
	}

	respondJSON(w, http.StatusOK, results)
}

// =============================================================================
// EMAIL HANDLERS
// =============================================================================

// GetSMTPConfig returns the SMTP configuration for a tenant
// @Summary Get SMTP configuration
// @Description Get the SMTP email settings for a tenant
// @Tags Email
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Success 200 {object} email.SMTPConfig
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/settings/smtp [get]
func (h *Handlers) GetSMTPConfig(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")

	config, err := h.emailService.GetSMTPConfig(r.Context(), tenantID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get SMTP config")
		return
	}

	// Don't expose password
	config.Password = ""

	respondJSON(w, http.StatusOK, config)
}

// UpdateSMTPConfig updates the SMTP configuration for a tenant
// @Summary Update SMTP configuration
// @Description Update the SMTP email settings for a tenant
// @Tags Email
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param request body email.UpdateSMTPConfigRequest true "SMTP settings"
// @Success 200 {object} object{status=string}
// @Failure 400 {object} object{error=string}
// @Router /tenants/{tenantID}/settings/smtp [put]
func (h *Handlers) UpdateSMTPConfig(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")

	var req email.UpdateSMTPConfigRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if err := h.emailService.UpdateSMTPConfig(r.Context(), tenantID, &req); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

// TestSMTP tests the SMTP configuration
// @Summary Test SMTP configuration
// @Description Send a test email to verify SMTP settings
// @Tags Email
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param request body email.TestSMTPRequest true "Test email recipient"
// @Success 200 {object} email.TestSMTPResponse
// @Failure 400 {object} object{error=string}
// @Router /tenants/{tenantID}/settings/smtp/test [post]
func (h *Handlers) TestSMTP(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")

	var req email.TestSMTPRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.RecipientEmail == "" {
		respondError(w, http.StatusBadRequest, "Recipient email is required")
		return
	}

	result, err := h.emailService.TestSMTP(r.Context(), tenantID, req.RecipientEmail)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, result)
}

// ListEmailTemplates returns all email templates for a tenant
// @Summary List email templates
// @Description Get all email templates for a tenant
// @Tags Email
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Success 200 {array} email.EmailTemplate
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/email-templates [get]
func (h *Handlers) ListEmailTemplates(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	templates, err := h.emailService.ListTemplates(r.Context(), schemaName, tenantID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to list templates")
		return
	}

	respondJSON(w, http.StatusOK, templates)
}

// UpdateEmailTemplate updates an email template
// @Summary Update email template
// @Description Update an email template for a tenant
// @Tags Email
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param templateType path string true "Template type (INVOICE_SEND, PAYMENT_RECEIPT, OVERDUE_REMINDER)"
// @Param request body email.UpdateTemplateRequest true "Template content"
// @Success 200 {object} email.EmailTemplate
// @Failure 400 {object} object{error=string}
// @Router /tenants/{tenantID}/email-templates/{templateType} [put]
func (h *Handlers) UpdateEmailTemplate(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	templateType := chi.URLParam(r, "templateType")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	var req email.UpdateTemplateRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	template, err := h.emailService.UpdateTemplate(r.Context(), schemaName, tenantID, email.TemplateType(templateType), &req)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, template)
}

// GetEmailLog returns the email log for a tenant
// @Summary Get email log
// @Description Get the email sending history for a tenant
// @Tags Email
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param limit query int false "Number of entries to return (default 50)"
// @Success 200 {array} email.EmailLog
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/email-log [get]
func (h *Handlers) GetEmailLog(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := parseIntParam(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	logs, err := h.emailService.GetEmailLog(r.Context(), schemaName, tenantID, limit)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get email log")
		return
	}

	respondJSON(w, http.StatusOK, logs)
}

// EmailInvoice sends an invoice via email
// @Summary Email invoice
// @Description Send an invoice to a recipient via email
// @Tags Email
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param invoiceID path string true "Invoice ID"
// @Param request body email.SendInvoiceRequest true "Email details"
// @Success 200 {object} email.EmailSentResponse
// @Failure 400 {object} object{error=string}
// @Router /tenants/{tenantID}/invoices/{invoiceID}/email [post]
func (h *Handlers) EmailInvoice(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	invoiceID := chi.URLParam(r, "invoiceID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	var req email.SendInvoiceRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if err := req.Validate(); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Get invoice
	invoice, err := h.invoicingService.GetByID(r.Context(), tenantID, schemaName, invoiceID)
	if err != nil {
		respondError(w, http.StatusNotFound, "Invoice not found")
		return
	}

	// Get tenant for company name
	t, err := h.tenantService.GetTenant(r.Context(), tenantID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get tenant")
		return
	}

	// Get template
	template, err := h.emailService.GetTemplate(r.Context(), schemaName, tenantID, email.TemplateInvoiceSend)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get email template")
		return
	}

	// Prepare template data
	data := &email.TemplateData{
		CompanyName:   t.Name,
		ContactName:   req.RecipientName,
		InvoiceNumber: invoice.InvoiceNumber,
		TotalAmount:   invoice.Total.StringFixed(2),
		Currency:      invoice.Currency,
		DueDate:       invoice.DueDate.Format("2006-01-02"),
		IssueDate:     invoice.IssueDate.Format("2006-01-02"),
		Message:       req.Message,
	}

	// Render template
	subject, bodyHTML, bodyText, err := h.emailService.RenderTemplate(template, data)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to render email template")
		return
	}

	// Override subject if provided
	if req.Subject != "" {
		subject = req.Subject
	}

	// Prepare attachments
	var attachments []email.Attachment
	if req.AttachPDF {
		pdfSettings := h.pdfService.PDFSettingsFromTenant(t)
		pdfBytes, err := h.pdfService.GenerateInvoicePDF(invoice, t, pdfSettings)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "Failed to generate PDF")
			return
		}
		attachments = append(attachments, email.Attachment{
			Filename:    "invoice-" + invoice.InvoiceNumber + ".pdf",
			Content:     pdfBytes,
			ContentType: "application/pdf",
		})
	}

	// Send email
	result, err := h.emailService.SendEmail(r.Context(), schemaName, tenantID, string(email.TemplateInvoiceSend), req.RecipientEmail, req.RecipientName, subject, bodyHTML, bodyText, attachments, invoiceID)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Mark invoice as sent if it's a draft
	if invoice.Status == invoicing.StatusDraft {
		_ = h.invoicingService.Send(r.Context(), tenantID, schemaName, invoiceID)
	}

	respondJSON(w, http.StatusOK, result)
}

// EmailPaymentReceipt sends a payment receipt via email
// @Summary Email payment receipt
// @Description Send a payment receipt to a recipient via email
// @Tags Email
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param paymentID path string true "Payment ID"
// @Param request body email.SendPaymentReceiptRequest true "Email details"
// @Success 200 {object} email.EmailSentResponse
// @Failure 400 {object} object{error=string}
// @Router /tenants/{tenantID}/payments/{paymentID}/email-receipt [post]
func (h *Handlers) EmailPaymentReceipt(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	paymentID := chi.URLParam(r, "paymentID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	var req email.SendPaymentReceiptRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if err := req.Validate(); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Get payment
	payment, err := h.paymentsService.GetByID(r.Context(), tenantID, schemaName, paymentID)
	if err != nil {
		respondError(w, http.StatusNotFound, "Payment not found")
		return
	}

	// Get tenant for company name
	t, err := h.tenantService.GetTenant(r.Context(), tenantID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get tenant")
		return
	}

	// Get template
	template, err := h.emailService.GetTemplate(r.Context(), schemaName, tenantID, email.TemplatePaymentReceipt)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get email template")
		return
	}

	// Prepare template data
	data := &email.TemplateData{
		CompanyName: t.Name,
		ContactName: req.RecipientName,
		Amount:      payment.Amount.StringFixed(2),
		Currency:    payment.Currency,
		PaymentDate: payment.PaymentDate.Format("2006-01-02"),
		Reference:   payment.Reference,
		Message:     req.Message,
	}

	// Render template
	subject, bodyHTML, bodyText, err := h.emailService.RenderTemplate(template, data)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to render email template")
		return
	}

	// Override subject if provided
	if req.Subject != "" {
		subject = req.Subject
	}

	// Send email
	result, err := h.emailService.SendEmail(r.Context(), schemaName, tenantID, string(email.TemplatePaymentReceipt), req.RecipientEmail, req.RecipientName, subject, bodyHTML, bodyText, nil, paymentID)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, result)
}

// =============================================================================
// BANKING HANDLERS
// =============================================================================

// ListBankAccounts lists all bank accounts for a tenant
// @Summary List bank accounts
// @Description Get all bank accounts for a tenant
// @Tags Banking
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Success 200 {array} banking.BankAccount
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/bank-accounts [get]
func (h *Handlers) ListBankAccounts(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	activeOnly := r.URL.Query().Get("active_only") == "true"
	var filter *banking.BankAccountFilter
	if activeOnly {
		active := true
		filter = &banking.BankAccountFilter{IsActive: &active}
	}

	accounts, err := h.bankingService.ListBankAccounts(r.Context(), schemaName, tenantID, filter)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to list bank accounts")
		return
	}

	respondJSON(w, http.StatusOK, accounts)
}

// CreateBankAccount creates a new bank account
// @Summary Create bank account
// @Description Create a new bank account
// @Tags Banking
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param request body banking.CreateBankAccountRequest true "Bank account details"
// @Success 201 {object} banking.BankAccount
// @Failure 400 {object} object{error=string}
// @Router /tenants/{tenantID}/bank-accounts [post]
func (h *Handlers) CreateBankAccount(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	var req banking.CreateBankAccountRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Name == "" || req.AccountNumber == "" {
		respondError(w, http.StatusBadRequest, "Name and account number are required")
		return
	}

	account, err := h.bankingService.CreateBankAccount(r.Context(), schemaName, tenantID, &req)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, account)
}

// GetBankAccount retrieves a bank account by ID
// @Summary Get bank account
// @Description Get bank account details by ID
// @Tags Banking
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param accountID path string true "Bank Account ID"
// @Success 200 {object} banking.BankAccount
// @Failure 404 {object} object{error=string}
// @Router /tenants/{tenantID}/bank-accounts/{accountID} [get]
func (h *Handlers) GetBankAccount(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	accountID := chi.URLParam(r, "accountID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	account, err := h.bankingService.GetBankAccount(r.Context(), schemaName, tenantID, accountID)
	if err != nil {
		respondError(w, http.StatusNotFound, "Bank account not found")
		return
	}

	respondJSON(w, http.StatusOK, account)
}

// UpdateBankAccount updates a bank account
// @Summary Update bank account
// @Description Update bank account details
// @Tags Banking
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param accountID path string true "Bank Account ID"
// @Param request body banking.UpdateBankAccountRequest true "Bank account updates"
// @Success 200 {object} banking.BankAccount
// @Failure 400 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Router /tenants/{tenantID}/bank-accounts/{accountID} [put]
func (h *Handlers) UpdateBankAccount(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	accountID := chi.URLParam(r, "accountID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	var req banking.UpdateBankAccountRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	account, err := h.bankingService.UpdateBankAccount(r.Context(), schemaName, tenantID, accountID, &req)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, account)
}

// DeleteBankAccount deletes a bank account
// @Summary Delete bank account
// @Description Delete a bank account (only if no transactions)
// @Tags Banking
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param accountID path string true "Bank Account ID"
// @Success 204
// @Failure 400 {object} object{error=string}
// @Router /tenants/{tenantID}/bank-accounts/{accountID} [delete]
func (h *Handlers) DeleteBankAccount(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	accountID := chi.URLParam(r, "accountID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	if err := h.bankingService.DeleteBankAccount(r.Context(), schemaName, tenantID, accountID); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ListBankTransactions lists bank transactions for an account
// @Summary List bank transactions
// @Description Get bank transactions for a bank account with filters
// @Tags Banking
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param accountID path string true "Bank Account ID"
// @Param status query string false "Filter by status (UNMATCHED, MATCHED, RECONCILED)"
// @Param from_date query string false "Filter from date (YYYY-MM-DD)"
// @Param to_date query string false "Filter to date (YYYY-MM-DD)"
// @Success 200 {array} banking.BankTransaction
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/bank-accounts/{accountID}/transactions [get]
func (h *Handlers) ListBankTransactions(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	accountID := chi.URLParam(r, "accountID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	filter := &banking.TransactionFilter{
		BankAccountID: accountID,
	}

	if status := r.URL.Query().Get("status"); status != "" {
		filter.Status = banking.TransactionStatus(status)
	}

	if fromDate := r.URL.Query().Get("from_date"); fromDate != "" {
		t, err := time.Parse("2006-01-02", fromDate)
		if err == nil {
			filter.FromDate = &t
		}
	}

	if toDate := r.URL.Query().Get("to_date"); toDate != "" {
		t, err := time.Parse("2006-01-02", toDate)
		if err == nil {
			filter.ToDate = &t
		}
	}

	transactions, err := h.bankingService.ListTransactions(r.Context(), schemaName, tenantID, filter)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to list transactions")
		return
	}

	respondJSON(w, http.StatusOK, transactions)
}

// GetBankTransaction retrieves a single bank transaction
// @Summary Get bank transaction
// @Description Get bank transaction details by ID
// @Tags Banking
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param transactionID path string true "Transaction ID"
// @Success 200 {object} banking.BankTransaction
// @Failure 404 {object} object{error=string}
// @Router /tenants/{tenantID}/bank-transactions/{transactionID} [get]
func (h *Handlers) GetBankTransaction(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	transactionID := chi.URLParam(r, "transactionID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	transaction, err := h.bankingService.GetTransaction(r.Context(), schemaName, tenantID, transactionID)
	if err != nil {
		respondError(w, http.StatusNotFound, "Transaction not found")
		return
	}

	respondJSON(w, http.StatusOK, transaction)
}

// ImportBankTransactions imports transactions from JSON data
// @Summary Import bank transactions
// @Description Import bank transactions from CSV data
// @Tags Banking
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param accountID path string true "Bank Account ID"
// @Param request body banking.ImportCSVRequest true "Import data"
// @Success 200 {object} banking.ImportResult
// @Failure 400 {object} object{error=string}
// @Router /tenants/{tenantID}/bank-accounts/{accountID}/import [post]
func (h *Handlers) ImportBankTransactions(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	accountID := chi.URLParam(r, "accountID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	var req banking.ImportCSVRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if len(req.Transactions) == 0 {
		respondError(w, http.StatusBadRequest, "No transactions to import")
		return
	}

	if req.FileName == "" {
		req.FileName = "manual_import.csv"
	}

	result, err := h.bankingService.ImportTransactions(r.Context(), schemaName, tenantID, accountID, &req)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, result)
}

// GetImportHistory retrieves import history for a bank account
// @Summary Get import history
// @Description Get bank statement import history for an account
// @Tags Banking
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param accountID path string true "Bank Account ID"
// @Success 200 {array} banking.BankStatementImport
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/bank-accounts/{accountID}/import-history [get]
func (h *Handlers) GetImportHistory(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	accountID := chi.URLParam(r, "accountID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	imports, err := h.bankingService.GetImportHistory(r.Context(), schemaName, tenantID, accountID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get import history")
		return
	}

	respondJSON(w, http.StatusOK, imports)
}

// GetMatchSuggestions returns match suggestions for a transaction
// @Summary Get match suggestions
// @Description Get payment match suggestions for a bank transaction
// @Tags Banking
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param transactionID path string true "Transaction ID"
// @Success 200 {array} banking.MatchSuggestion
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/bank-transactions/{transactionID}/suggestions [get]
func (h *Handlers) GetMatchSuggestions(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	transactionID := chi.URLParam(r, "transactionID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	suggestions, err := h.bankingService.GetMatchSuggestions(r.Context(), schemaName, tenantID, transactionID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get match suggestions")
		return
	}

	respondJSON(w, http.StatusOK, suggestions)
}

// MatchBankTransaction matches a transaction to a payment
// @Summary Match bank transaction
// @Description Match a bank transaction to a payment
// @Tags Banking
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param transactionID path string true "Transaction ID"
// @Param request body banking.MatchTransactionRequest true "Match details"
// @Success 200 {object} object{status=string}
// @Failure 400 {object} object{error=string}
// @Router /tenants/{tenantID}/bank-transactions/{transactionID}/match [post]
func (h *Handlers) MatchBankTransaction(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	transactionID := chi.URLParam(r, "transactionID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	var req banking.MatchTransactionRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.PaymentID == "" {
		respondError(w, http.StatusBadRequest, "Payment ID is required")
		return
	}

	if err := h.bankingService.MatchTransaction(r.Context(), schemaName, tenantID, transactionID, req.PaymentID); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "matched"})
}

// UnmatchBankTransaction removes match from a transaction
// @Summary Unmatch bank transaction
// @Description Remove the payment match from a bank transaction
// @Tags Banking
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param transactionID path string true "Transaction ID"
// @Success 200 {object} object{status=string}
// @Failure 400 {object} object{error=string}
// @Router /tenants/{tenantID}/bank-transactions/{transactionID}/unmatch [post]
func (h *Handlers) UnmatchBankTransaction(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	transactionID := chi.URLParam(r, "transactionID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	if err := h.bankingService.UnmatchTransaction(r.Context(), schemaName, tenantID, transactionID); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "unmatched"})
}

// CreatePaymentFromTransaction creates a payment from a bank transaction
// @Summary Create payment from transaction
// @Description Create a new payment from a bank transaction and link them
// @Tags Banking
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param transactionID path string true "Transaction ID"
// @Success 200 {object} object{payment_id=string}
// @Failure 400 {object} object{error=string}
// @Router /tenants/{tenantID}/bank-transactions/{transactionID}/create-payment [post]
func (h *Handlers) CreatePaymentFromTransaction(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.GetClaims(r.Context())
	tenantID := chi.URLParam(r, "tenantID")
	transactionID := chi.URLParam(r, "transactionID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	paymentID, err := h.bankingService.CreatePaymentFromTransaction(r.Context(), schemaName, tenantID, claims.UserID, transactionID)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"payment_id": paymentID})
}

// ListReconciliations lists reconciliations for a bank account
// @Summary List reconciliations
// @Description Get reconciliation history for a bank account
// @Tags Banking
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param accountID path string true "Bank Account ID"
// @Success 200 {array} banking.BankReconciliation
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/bank-accounts/{accountID}/reconciliations [get]
func (h *Handlers) ListReconciliations(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	accountID := chi.URLParam(r, "accountID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	reconciliations, err := h.bankingService.ListReconciliations(r.Context(), schemaName, tenantID, accountID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to list reconciliations")
		return
	}

	respondJSON(w, http.StatusOK, reconciliations)
}

// CreateReconciliation starts a new reconciliation session
// @Summary Create reconciliation
// @Description Start a new bank reconciliation session
// @Tags Banking
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param accountID path string true "Bank Account ID"
// @Param request body banking.CreateReconciliationRequest true "Reconciliation details"
// @Success 201 {object} banking.BankReconciliation
// @Failure 400 {object} object{error=string}
// @Router /tenants/{tenantID}/bank-accounts/{accountID}/reconciliation [post]
func (h *Handlers) CreateReconciliation(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.GetClaims(r.Context())
	tenantID := chi.URLParam(r, "tenantID")
	accountID := chi.URLParam(r, "accountID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	var req banking.CreateReconciliationRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	reconciliation, err := h.bankingService.CreateReconciliation(r.Context(), schemaName, tenantID, accountID, claims.UserID, &req)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, reconciliation)
}

// GetReconciliation retrieves a reconciliation by ID
// @Summary Get reconciliation
// @Description Get reconciliation details by ID
// @Tags Banking
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param reconciliationID path string true "Reconciliation ID"
// @Success 200 {object} banking.BankReconciliation
// @Failure 404 {object} object{error=string}
// @Router /tenants/{tenantID}/reconciliations/{reconciliationID} [get]
func (h *Handlers) GetReconciliation(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	reconciliationID := chi.URLParam(r, "reconciliationID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	reconciliation, err := h.bankingService.GetReconciliation(r.Context(), schemaName, tenantID, reconciliationID)
	if err != nil {
		respondError(w, http.StatusNotFound, "Reconciliation not found")
		return
	}

	respondJSON(w, http.StatusOK, reconciliation)
}

// CompleteReconciliation marks a reconciliation as complete
// @Summary Complete reconciliation
// @Description Mark a reconciliation session as complete
// @Tags Banking
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param reconciliationID path string true "Reconciliation ID"
// @Success 200 {object} object{status=string}
// @Failure 400 {object} object{error=string}
// @Router /tenants/{tenantID}/reconciliations/{reconciliationID}/complete [post]
func (h *Handlers) CompleteReconciliation(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	reconciliationID := chi.URLParam(r, "reconciliationID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	if err := h.bankingService.CompleteReconciliation(r.Context(), schemaName, tenantID, reconciliationID); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "completed"})
}

// AutoMatchTransactions attempts to auto-match unmatched transactions
// @Summary Auto-match transactions
// @Description Automatically match unmatched bank transactions to payments
// @Tags Banking
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param accountID path string true "Bank Account ID"
// @Param min_confidence query number false "Minimum confidence threshold (0-1, default 0.7)"
// @Success 200 {object} object{matched=int}
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/bank-accounts/{accountID}/auto-match [post]
func (h *Handlers) AutoMatchTransactions(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	accountID := chi.URLParam(r, "accountID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	minConfidence := 0.7
	if conf := r.URL.Query().Get("min_confidence"); conf != "" {
		if parsed, err := strconv.ParseFloat(conf, 64); err == nil && parsed >= 0 && parsed <= 1 {
			minConfidence = parsed
		}
	}

	matched, err := h.bankingService.AutoMatchTransactions(r.Context(), schemaName, tenantID, accountID, minConfidence)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to auto-match transactions")
		return
	}

	respondJSON(w, http.StatusOK, map[string]int{"matched": matched})
}

// =============================================================================
// USER & INVITATION HANDLERS
// =============================================================================

// ListTenantUsers returns all users for a tenant
// @Summary List tenant users
// @Description Get all users who are members of a tenant
// @Tags Users
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Success 200 {array} tenant.TenantUser
// @Failure 403 {object} object{error=string}
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/users [get]
func (h *Handlers) ListTenantUsers(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.GetClaims(r.Context())
	tenantID := chi.URLParam(r, "tenantID")

	// Only admin/owner can list users
	if !auth.CanManageUsers(claims.Role) {
		respondError(w, http.StatusForbidden, "Permission denied")
		return
	}

	users, err := h.tenantService.ListTenantUsers(r.Context(), tenantID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to list users")
		return
	}

	respondJSON(w, http.StatusOK, users)
}

// RemoveTenantUser removes a user from a tenant
// @Summary Remove user from tenant
// @Description Remove a user from the tenant organization
// @Tags Users
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param userID path string true "User ID"
// @Success 200 {object} object{status=string}
// @Failure 400 {object} object{error=string}
// @Failure 403 {object} object{error=string}
// @Router /tenants/{tenantID}/users/{userID} [delete]
func (h *Handlers) RemoveTenantUser(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.GetClaims(r.Context())
	tenantID := chi.URLParam(r, "tenantID")
	userID := chi.URLParam(r, "userID")

	// Only admin/owner can remove users
	if !auth.CanManageUsers(claims.Role) {
		respondError(w, http.StatusForbidden, "Permission denied")
		return
	}

	// Prevent self-removal
	if userID == claims.UserID {
		respondError(w, http.StatusBadRequest, "Cannot remove yourself from the organization")
		return
	}

	if err := h.tenantService.RemoveTenantUser(r.Context(), tenantID, userID); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "removed"})
}

// UpdateTenantUserRole updates a user's role in the tenant
// @Summary Update user role
// @Description Update a user's role in the tenant organization
// @Tags Users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param userID path string true "User ID"
// @Param request body object{role=string} true "New role"
// @Success 200 {object} object{status=string}
// @Failure 400 {object} object{error=string}
// @Failure 403 {object} object{error=string}
// @Router /tenants/{tenantID}/users/{userID}/role [put]
func (h *Handlers) UpdateTenantUserRole(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.GetClaims(r.Context())
	tenantID := chi.URLParam(r, "tenantID")
	userID := chi.URLParam(r, "userID")

	// Only admin/owner can update roles
	if !auth.CanManageUsers(claims.Role) {
		respondError(w, http.StatusForbidden, "Permission denied")
		return
	}

	var req struct {
		Role string `json:"role"`
	}
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Role == "" {
		respondError(w, http.StatusBadRequest, "Role is required")
		return
	}

	if err := h.tenantService.UpdateTenantUserRole(r.Context(), tenantID, userID, req.Role); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

// CreateInvitation creates a new invitation to join a tenant
// @Summary Create invitation
// @Description Invite a user to join the tenant organization
// @Tags Invitations
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param request body tenant.CreateInvitationRequest true "Invitation details"
// @Success 201 {object} tenant.UserInvitation
// @Failure 400 {object} object{error=string}
// @Failure 403 {object} object{error=string}
// @Router /tenants/{tenantID}/invitations [post]
func (h *Handlers) CreateInvitation(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.GetClaims(r.Context())
	tenantID := chi.URLParam(r, "tenantID")

	// Only admin/owner can invite users
	if !auth.CanManageUsers(claims.Role) {
		respondError(w, http.StatusForbidden, "Permission denied")
		return
	}

	var req struct {
		Email string `json:"email"`
		Role  string `json:"role"`
	}
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Email == "" || req.Role == "" {
		respondError(w, http.StatusBadRequest, "Email and role are required")
		return
	}

	invitation, err := h.tenantService.CreateInvitation(r.Context(), tenantID, claims.UserID, &tenant.CreateInvitationRequest{
		Email: req.Email,
		Role:  req.Role,
	})
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, invitation)
}

// ListInvitations returns pending invitations for a tenant
// @Summary List invitations
// @Description Get all pending invitations for a tenant
// @Tags Invitations
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Success 200 {array} tenant.UserInvitation
// @Failure 403 {object} object{error=string}
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/invitations [get]
func (h *Handlers) ListInvitations(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.GetClaims(r.Context())
	tenantID := chi.URLParam(r, "tenantID")

	// Only admin/owner can view invitations
	if !auth.CanManageUsers(claims.Role) {
		respondError(w, http.StatusForbidden, "Permission denied")
		return
	}

	invitations, err := h.tenantService.ListInvitations(r.Context(), tenantID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to list invitations")
		return
	}

	respondJSON(w, http.StatusOK, invitations)
}

// RevokeInvitation revokes a pending invitation
// @Summary Revoke invitation
// @Description Revoke a pending invitation
// @Tags Invitations
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param invitationID path string true "Invitation ID"
// @Success 200 {object} object{status=string}
// @Failure 400 {object} object{error=string}
// @Failure 403 {object} object{error=string}
// @Router /tenants/{tenantID}/invitations/{invitationID} [delete]
func (h *Handlers) RevokeInvitation(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.GetClaims(r.Context())
	tenantID := chi.URLParam(r, "tenantID")
	invitationID := chi.URLParam(r, "invitationID")

	// Only admin/owner can revoke invitations
	if !auth.CanManageUsers(claims.Role) {
		respondError(w, http.StatusForbidden, "Permission denied")
		return
	}

	if err := h.tenantService.RevokeInvitation(r.Context(), tenantID, invitationID); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "revoked"})
}

// GetInvitationByToken retrieves invitation details by token
// @Summary Get invitation by token
// @Description Get invitation details for the invitation acceptance page
// @Tags Invitations
// @Produce json
// @Param token path string true "Invitation token"
// @Success 200 {object} tenant.UserInvitation
// @Failure 400 {object} object{error=string}
// @Router /invitations/{token} [get]
func (h *Handlers) GetInvitationByToken(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")

	invitation, err := h.tenantService.GetInvitationByToken(r.Context(), token)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, invitation)
}

// AcceptInvitation accepts an invitation and joins the tenant
// @Summary Accept invitation
// @Description Accept an invitation to join a tenant organization
// @Tags Invitations
// @Accept json
// @Produce json
// @Param request body tenant.AcceptInvitationRequest true "Acceptance details"
// @Success 200 {object} tenant.TenantMembership
// @Failure 400 {object} object{error=string}
// @Router /invitations/accept [post]
func (h *Handlers) AcceptInvitation(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Token    string `json:"token"`
		Password string `json:"password,omitempty"`
		Name     string `json:"name,omitempty"`
	}
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Token == "" {
		respondError(w, http.StatusBadRequest, "Token is required")
		return
	}

	membership, err := h.tenantService.AcceptInvitation(r.Context(), &tenant.AcceptInvitationRequest{
		Token:    req.Token,
		Password: req.Password,
		Name:     req.Name,
	})
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, membership)
}

// =============================================================================
// PAYROLL HANDLERS
// =============================================================================

// ListEmployees returns all employees for a tenant
// @Summary List employees
// @Description Get all employees for the payroll system
// @Tags Payroll
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param active_only query bool false "Filter for active employees only"
// @Success 200 {array} payroll.Employee
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/employees [get]
func (h *Handlers) ListEmployees(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	activeOnly := r.URL.Query().Get("active_only") == "true"

	employees, err := h.payrollService.ListEmployees(r.Context(), schemaName, tenantID, activeOnly)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to list employees")
		return
	}

	respondJSON(w, http.StatusOK, employees)
}

// CreateEmployee creates a new employee
// @Summary Create employee
// @Description Create a new employee in the payroll system
// @Tags Payroll
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param request body payroll.CreateEmployeeRequest true "Employee details"
// @Success 201 {object} payroll.Employee
// @Failure 400 {object} object{error=string}
// @Router /tenants/{tenantID}/employees [post]
func (h *Handlers) CreateEmployee(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	var req payroll.CreateEmployeeRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	employee, err := h.payrollService.CreateEmployee(r.Context(), schemaName, tenantID, &req)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, employee)
}

// GetEmployee returns an employee by ID
// @Summary Get employee
// @Description Get employee details by ID
// @Tags Payroll
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param employeeID path string true "Employee ID"
// @Success 200 {object} payroll.Employee
// @Failure 404 {object} object{error=string}
// @Router /tenants/{tenantID}/employees/{employeeID} [get]
func (h *Handlers) GetEmployee(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	employeeID := chi.URLParam(r, "employeeID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	employee, err := h.payrollService.GetEmployee(r.Context(), schemaName, tenantID, employeeID)
	if err != nil {
		respondError(w, http.StatusNotFound, "Employee not found")
		return
	}

	respondJSON(w, http.StatusOK, employee)
}

// UpdateEmployee updates an employee
// @Summary Update employee
// @Description Update an existing employee's details
// @Tags Payroll
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param employeeID path string true "Employee ID"
// @Param request body payroll.UpdateEmployeeRequest true "Updated employee details"
// @Success 200 {object} payroll.Employee
// @Failure 400 {object} object{error=string}
// @Router /tenants/{tenantID}/employees/{employeeID} [put]
func (h *Handlers) UpdateEmployee(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	employeeID := chi.URLParam(r, "employeeID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	var req payroll.UpdateEmployeeRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	employee, err := h.payrollService.UpdateEmployee(r.Context(), schemaName, tenantID, employeeID, &req)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, employee)
}

// SetBaseSalary sets the base salary for an employee
// @Summary Set base salary
// @Description Set or update the base salary for an employee
// @Tags Payroll
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param employeeID path string true "Employee ID"
// @Param request body object{amount=number,effective_from=string} true "Salary details"
// @Success 200 {object} object{status=string}
// @Failure 400 {object} object{error=string}
// @Router /tenants/{tenantID}/employees/{employeeID}/salary [post]
func (h *Handlers) SetBaseSalary(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	employeeID := chi.URLParam(r, "employeeID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	var req struct {
		Amount        decimal.Decimal `json:"amount"`
		EffectiveFrom time.Time       `json:"effective_from"`
	}
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Amount.IsZero() {
		respondError(w, http.StatusBadRequest, "Amount is required")
		return
	}

	if req.EffectiveFrom.IsZero() {
		req.EffectiveFrom = time.Now()
	}

	err := h.payrollService.SetBaseSalary(r.Context(), schemaName, tenantID, employeeID, req.Amount, req.EffectiveFrom)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "salary updated"})
}

// ListPayrollRuns returns all payroll runs for a tenant
// @Summary List payroll runs
// @Description Get all payroll runs for a tenant
// @Tags Payroll
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param year query int false "Filter by year"
// @Success 200 {array} payroll.PayrollRun
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/payroll-runs [get]
func (h *Handlers) ListPayrollRuns(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	year := 0
	if y := r.URL.Query().Get("year"); y != "" {
		if parsed, err := strconv.Atoi(y); err == nil {
			year = parsed
		}
	}

	runs, err := h.payrollService.ListPayrollRuns(r.Context(), schemaName, tenantID, year)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to list payroll runs")
		return
	}

	respondJSON(w, http.StatusOK, runs)
}

// CreatePayrollRun creates a new payroll run
// @Summary Create payroll run
// @Description Create a new monthly payroll run
// @Tags Payroll
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param request body payroll.CreatePayrollRunRequest true "Payroll run details"
// @Success 201 {object} payroll.PayrollRun
// @Failure 400 {object} object{error=string}
// @Router /tenants/{tenantID}/payroll-runs [post]
func (h *Handlers) CreatePayrollRun(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.GetClaims(r.Context())
	tenantID := chi.URLParam(r, "tenantID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	var req payroll.CreatePayrollRunRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	run, err := h.payrollService.CreatePayrollRun(r.Context(), schemaName, tenantID, claims.UserID, &req)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, run)
}

// GetPayrollRun returns a payroll run by ID
// @Summary Get payroll run
// @Description Get payroll run details by ID
// @Tags Payroll
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param runID path string true "Payroll Run ID"
// @Success 200 {object} payroll.PayrollRun
// @Failure 404 {object} object{error=string}
// @Router /tenants/{tenantID}/payroll-runs/{runID} [get]
func (h *Handlers) GetPayrollRun(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	runID := chi.URLParam(r, "runID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	run, err := h.payrollService.GetPayrollRun(r.Context(), schemaName, tenantID, runID)
	if err != nil {
		respondError(w, http.StatusNotFound, "Payroll run not found")
		return
	}

	respondJSON(w, http.StatusOK, run)
}

// CalculatePayroll calculates all payslips for a payroll run
// @Summary Calculate payroll
// @Description Calculate payslips for all active employees in a payroll run
// @Tags Payroll
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param runID path string true "Payroll Run ID"
// @Success 200 {object} payroll.PayrollRun
// @Failure 400 {object} object{error=string}
// @Router /tenants/{tenantID}/payroll-runs/{runID}/calculate [post]
func (h *Handlers) CalculatePayroll(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	runID := chi.URLParam(r, "runID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	run, err := h.payrollService.CalculatePayroll(r.Context(), schemaName, tenantID, runID)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, run)
}

// ApprovePayroll approves a calculated payroll run
// @Summary Approve payroll
// @Description Approve a calculated payroll run for payment
// @Tags Payroll
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param runID path string true "Payroll Run ID"
// @Success 200 {object} object{status=string}
// @Failure 400 {object} object{error=string}
// @Router /tenants/{tenantID}/payroll-runs/{runID}/approve [post]
func (h *Handlers) ApprovePayroll(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.GetClaims(r.Context())
	tenantID := chi.URLParam(r, "tenantID")
	runID := chi.URLParam(r, "runID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	if err := h.payrollService.ApprovePayrollRun(r.Context(), schemaName, tenantID, runID, claims.UserID); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "approved"})
}

// GetPayslips returns all payslips for a payroll run
// @Summary Get payslips
// @Description Get all payslips for a specific payroll run
// @Tags Payroll
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param runID path string true "Payroll Run ID"
// @Success 200 {array} payroll.Payslip
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/payroll-runs/{runID}/payslips [get]
func (h *Handlers) GetPayslips(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	runID := chi.URLParam(r, "runID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	payslips, err := h.payrollService.GetPayslipsWithEmployees(r.Context(), schemaName, tenantID, runID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get payslips")
		return
	}

	respondJSON(w, http.StatusOK, payslips)
}

// CalculateTaxPreview returns a tax preview for a salary
// @Summary Calculate tax preview
// @Description Preview Estonian tax calculations for a given gross salary
// @Tags Payroll
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param request body object{gross_salary=number,apply_basic_exemption=bool,funded_pension_rate=number} true "Calculation parameters"
// @Success 200 {object} payroll.TaxCalculation
// @Failure 400 {object} object{error=string}
// @Router /tenants/{tenantID}/payroll/tax-preview [post]
func (h *Handlers) CalculateTaxPreview(w http.ResponseWriter, r *http.Request) {
	var req struct {
		GrossSalary          decimal.Decimal `json:"gross_salary"`
		ApplyBasicExemption  bool            `json:"apply_basic_exemption"`
		FundedPensionRate    decimal.Decimal `json:"funded_pension_rate"`
	}
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.GrossSalary.IsZero() || req.GrossSalary.IsNegative() {
		respondError(w, http.StatusBadRequest, "Gross salary must be positive")
		return
	}

	calc := payroll.CalculateTaxPreview(req.GrossSalary, req.ApplyBasicExemption, req.FundedPensionRate)
	respondJSON(w, http.StatusOK, calc)
}

// =============================================================================
// TSD (TAX DECLARATION) HANDLERS
// =============================================================================

// GenerateTSD generates a TSD declaration from a payroll run
// @Summary Generate TSD declaration
// @Description Generate an Estonian TSD tax declaration from a payroll run
// @Tags Payroll
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param runID path string true "Payroll Run ID"
// @Success 200 {object} payroll.TSDDeclaration
// @Failure 400 {object} object{error=string}
// @Router /tenants/{tenantID}/payroll-runs/{runID}/tsd [post]
func (h *Handlers) GenerateTSD(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	runID := chi.URLParam(r, "runID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	tsd, err := h.payrollService.GenerateTSD(r.Context(), schemaName, tenantID, runID)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, tsd)
}

// GetTSD returns a TSD declaration by period
// @Summary Get TSD declaration
// @Description Get a TSD declaration for a specific period
// @Tags Payroll
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param year path int true "Year"
// @Param month path int true "Month"
// @Success 200 {object} payroll.TSDDeclaration
// @Failure 404 {object} object{error=string}
// @Router /tenants/{tenantID}/tsd/{year}/{month} [get]
func (h *Handlers) GetTSD(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	year, err := strconv.Atoi(chi.URLParam(r, "year"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid year")
		return
	}

	month, err := strconv.Atoi(chi.URLParam(r, "month"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid month")
		return
	}

	tsd, err := h.payrollService.GetTSD(r.Context(), schemaName, tenantID, year, month)
	if err != nil {
		respondError(w, http.StatusNotFound, "TSD declaration not found")
		return
	}

	respondJSON(w, http.StatusOK, tsd)
}

// ListTSD returns all TSD declarations for a tenant
// @Summary List TSD declarations
// @Description Get all TSD declarations for a tenant
// @Tags Payroll
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Success 200 {array} payroll.TSDDeclaration
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/tsd [get]
func (h *Handlers) ListTSD(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	declarations, err := h.payrollService.ListTSD(r.Context(), schemaName, tenantID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to list TSD declarations")
		return
	}

	respondJSON(w, http.StatusOK, declarations)
}

// ExportTSDXML exports a TSD declaration to e-MTA XML format
// @Summary Export TSD to XML
// @Description Export a TSD declaration to Estonian e-MTA XML format
// @Tags Payroll
// @Produce application/xml
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param year path int true "Year"
// @Param month path int true "Month"
// @Success 200 {file} file "XML file"
// @Failure 404 {object} object{error=string}
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/tsd/{year}/{month}/xml [get]
func (h *Handlers) ExportTSDXML(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	year, err := strconv.Atoi(chi.URLParam(r, "year"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid year")
		return
	}

	month, err := strconv.Atoi(chi.URLParam(r, "month"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid month")
		return
	}

	// Get tenant for company info
	t, err := h.tenantService.GetTenant(r.Context(), tenantID)
	if err != nil {
		respondError(w, http.StatusNotFound, "Tenant not found")
		return
	}

	company := payroll.TSDCompanyInfo{
		RegistryCode: t.Settings.RegCode,
		Name:         t.Name,
	}

	xmlData, err := h.payrollService.ExportTSDToXML(r.Context(), schemaName, tenantID, year, month, company)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	filename := payroll.GenerateTSDFilename(company.RegistryCode, year, month, "xml")
	w.Header().Set("Content-Type", "application/xml")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	_, _ = w.Write(xmlData)
}

// ExportTSDCSV exports a TSD declaration to CSV format
// @Summary Export TSD to CSV
// @Description Export a TSD declaration to CSV format
// @Tags Payroll
// @Produce text/csv
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param year path int true "Year"
// @Param month path int true "Month"
// @Success 200 {file} file "CSV file"
// @Failure 404 {object} object{error=string}
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/tsd/{year}/{month}/csv [get]
func (h *Handlers) ExportTSDCSV(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	year, err := strconv.Atoi(chi.URLParam(r, "year"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid year")
		return
	}

	month, err := strconv.Atoi(chi.URLParam(r, "month"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid month")
		return
	}

	// Get tenant for company info
	t, err := h.tenantService.GetTenant(r.Context(), tenantID)
	if err != nil {
		respondError(w, http.StatusNotFound, "Tenant not found")
		return
	}

	csvData, err := h.payrollService.ExportTSDToCSV(r.Context(), schemaName, tenantID, year, month)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	filename := payroll.GenerateTSDFilename(t.Settings.RegCode, year, month, "csv")
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	_, _ = w.Write(csvData)
}

// MarkTSDSubmitted marks a TSD declaration as submitted
// @Summary Mark TSD as submitted
// @Description Mark a TSD declaration as submitted to e-MTA
// @Tags Payroll
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param year path int true "Year"
// @Param month path int true "Month"
// @Param request body object{emta_reference=string} true "EMTA reference"
// @Success 200 {object} object{status=string}
// @Failure 400 {object} object{error=string}
// @Router /tenants/{tenantID}/tsd/{year}/{month}/submit [post]
func (h *Handlers) MarkTSDSubmitted(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	year, err := strconv.Atoi(chi.URLParam(r, "year"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid year")
		return
	}

	month, err := strconv.Atoi(chi.URLParam(r, "month"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid month")
		return
	}

	var req struct {
		EMTAReference string `json:"emta_reference"`
	}
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Get the TSD declaration to get its ID
	tsd, err := h.payrollService.GetTSD(r.Context(), schemaName, tenantID, year, month)
	if err != nil {
		respondError(w, http.StatusNotFound, "TSD declaration not found")
		return
	}

	if err := h.payrollService.MarkTSDSubmitted(r.Context(), schemaName, tenantID, tsd.ID, req.EMTAReference); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "submitted"})
}
