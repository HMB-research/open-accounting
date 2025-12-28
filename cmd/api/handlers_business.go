package main

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/shopspring/decimal"

	"github.com/openaccounting/openaccounting/internal/auth"
	"github.com/openaccounting/openaccounting/internal/contacts"
	"github.com/openaccounting/openaccounting/internal/invoicing"
	"github.com/openaccounting/openaccounting/internal/payments"
)

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
