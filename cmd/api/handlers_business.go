package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/shopspring/decimal"

	"github.com/HMB-research/open-accounting/internal/assets"
	"github.com/HMB-research/open-accounting/internal/auth"
	"github.com/HMB-research/open-accounting/internal/banking"
	"github.com/HMB-research/open-accounting/internal/banking/mappers/registry"
	"github.com/HMB-research/open-accounting/internal/contacts"
	"github.com/HMB-research/open-accounting/internal/documents"
	"github.com/HMB-research/open-accounting/internal/email"
	"github.com/HMB-research/open-accounting/internal/inventory"
	"github.com/HMB-research/open-accounting/internal/invoicing"
	"github.com/HMB-research/open-accounting/internal/orders"
	"github.com/HMB-research/open-accounting/internal/payments"
	"github.com/HMB-research/open-accounting/internal/payroll"
	internalpdf "github.com/HMB-research/open-accounting/internal/pdf"
	"github.com/HMB-research/open-accounting/internal/plugin"
	"github.com/HMB-research/open-accounting/internal/quotes"
	"github.com/HMB-research/open-accounting/internal/tenant"

	// Blank imports for swagger annotations
	_ "github.com/HMB-research/open-accounting/internal/accounting"
	_ "github.com/HMB-research/open-accounting/internal/analytics"
	_ "github.com/HMB-research/open-accounting/internal/tax"
)

var (
	errApprovedReconciliationEvidenceRequired  = errors.New("approved reconciliation evidence is required")
	errApprovedAssetActivationEvidenceRequired = errors.New("approved asset activation evidence is required")
	errApprovedAssetDisposalEvidenceRequired   = errors.New("approved asset disposal evidence is required")
	errApprovedJournalEntryEvidenceRequired    = errors.New("approved journal-entry evidence is required")
	errApprovedPaymentReceiptEvidenceRequired  = errors.New("approved payment receipt evidence is required")
	errApprovedPurchaseInvoiceEvidenceRequired = errors.New("approved purchase-invoice evidence is required")
	errApprovedQuoteEvidenceRequired           = errors.New("approved quote evidence is required")
	errApprovedOrderEvidenceRequired           = errors.New("approved order evidence is required")
	errApprovedTSDSubmissionEvidenceRequired   = errors.New("approved TSD submission evidence is required")
	errApprovedTSDAcceptanceEvidenceRequired   = errors.New("approved TSD acceptance evidence is required")
)

var (
	generateInvoicePDF = func(pdfService *internalpdf.Service, invoice *invoicing.Invoice, tenantRecord *tenant.Tenant, pdfSettings internalpdf.PDFSettings) ([]byte, error) {
		return pdfService.GenerateInvoicePDF(invoice, tenantRecord, pdfSettings)
	}
	generateQuotePDF = func(pdfService *internalpdf.Service, quote *quotes.Quote, tenantRecord *tenant.Tenant, pdfSettings internalpdf.PDFSettings) ([]byte, error) {
		return pdfService.GenerateQuotePDF(quote, tenantRecord, pdfSettings)
	}
	generateOrderPDF = func(pdfService *internalpdf.Service, order *orders.Order, tenantRecord *tenant.Tenant, pdfSettings internalpdf.PDFSettings) ([]byte, error) {
		return pdfService.GenerateOrderPDF(order, tenantRecord, pdfSettings)
	}
	generatePayslipPDF = func(pdfService *internalpdf.Service, payslip *payroll.Payslip, run *payroll.PayrollRun, tenantRecord *tenant.Tenant) ([]byte, error) {
		return pdfService.GeneratePayslipPDF(payslip, run, tenantRecord)
	}
	testSMTPWithService = func(ctx context.Context, service *email.Service, tenantID, recipientEmail string) (*email.TestSMTPResponse, error) {
		return service.TestSMTP(ctx, tenantID, recipientEmail)
	}
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
// @Produce json,text/csv,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet,application/pdf
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param format query string false "Response format: json, csv, xlsx, or pdf"
// @Success 200 {object} analytics.AgingReport
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/reports/aging/receivables [get]
func (h *Handlers) GetReceivablesAging(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	format, err := reportResponseFormat(r)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	report, err := h.analyticsService.GetReceivablesAging(r.Context(), tenantID, schemaName)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get aging report")
		return
	}

	if format == "csv" {
		content, err := exportAgingReportCSV(report)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "Failed to export aging CSV")
			return
		}
		respondReportCSV(w, fmt.Sprintf("receivables-aging-%s.csv", reportExportDate(report.AsOfDate)), content)
		return
	}
	if format == "xlsx" {
		content, err := exportAgingReportXLSX(report)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "Failed to export aging XLSX")
			return
		}
		respondReportXLSX(w, fmt.Sprintf("receivables-aging-%s.xlsx", reportExportDate(report.AsOfDate)), content)
		return
	}
	if format == "pdf" {
		content, err := exportAgingReportPDF(report)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "Failed to export aging PDF")
			return
		}
		respondReportPDF(w, fmt.Sprintf("receivables-aging-%s.pdf", reportExportDate(report.AsOfDate)), content)
		return
	}

	respondJSON(w, http.StatusOK, report)
}

// GetPayablesAging returns aging report for payables
// @Summary Get payables aging report
// @Description Get aging breakdown for accounts payable
// @Tags Reports
// @Produce json,text/csv,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet,application/pdf
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param format query string false "Response format: json, csv, xlsx, or pdf"
// @Success 200 {object} analytics.AgingReport
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/reports/aging/payables [get]
func (h *Handlers) GetPayablesAging(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	format, err := reportResponseFormat(r)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	report, err := h.analyticsService.GetPayablesAging(r.Context(), tenantID, schemaName)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get aging report")
		return
	}

	if format == "csv" {
		content, err := exportAgingReportCSV(report)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "Failed to export aging CSV")
			return
		}
		respondReportCSV(w, fmt.Sprintf("payables-aging-%s.csv", reportExportDate(report.AsOfDate)), content)
		return
	}
	if format == "xlsx" {
		content, err := exportAgingReportXLSX(report)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "Failed to export aging XLSX")
			return
		}
		respondReportXLSX(w, fmt.Sprintf("payables-aging-%s.xlsx", reportExportDate(report.AsOfDate)), content)
		return
	}
	if format == "pdf" {
		content, err := exportAgingReportPDF(report)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "Failed to export aging PDF")
			return
		}
		respondReportPDF(w, fmt.Sprintf("payables-aging-%s.pdf", reportExportDate(report.AsOfDate)), content)
		return
	}

	respondJSON(w, http.StatusOK, report)
}

// GetRecentActivity returns recent activity feed
// @Summary Get recent activity
// @Description Get recent activity from invoices, payments, journal entries, and contacts
// @Tags Analytics
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param limit query int false "Number of items (default 10)"
// @Success 200 {array} analytics.ActivityItem
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/analytics/activity [get]
func (h *Handlers) GetRecentActivity(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	limit := 10
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := parseIntParam(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	activity, err := h.analyticsService.GetRecentActivity(r.Context(), tenantID, schemaName, limit)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get recent activity")
		return
	}

	respondJSON(w, http.StatusOK, activity)
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
		log.Printf("Failed to list contacts for tenant %s (schema: %s): %v", tenantID, schemaName, err)
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

	h.emitWebhookEvent(plugin.EventContactCreated, tenantID, contact)
	respondJSON(w, http.StatusCreated, contact)
}

// ImportContacts imports contacts from CSV data.
// @Summary Import contacts
// @Description Import contacts from CSV data and skip duplicate or invalid rows
// @Tags Contacts
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param request body contacts.ImportContactsRequest true "CSV import payload"
// @Success 200 {object} contacts.ImportContactsResult
// @Failure 400 {object} object{error=string}
// @Router /tenants/{tenantID}/contacts/import [post]
func (h *Handlers) ImportContacts(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	var req contacts.ImportContactsRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if strings.TrimSpace(req.CSVContent) == "" {
		respondError(w, http.StatusBadRequest, "csv_content is required")
		return
	}

	if req.FileName == "" {
		req.FileName = "contacts_import.csv"
	}

	result, err := h.contactsService.ImportCSV(r.Context(), tenantID, schemaName, &req)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, result)
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

	h.emitWebhookEvent(plugin.EventContactUpdated, tenantID, contact)
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

	h.emitWebhookEvent(plugin.EventContactDeleted, tenantID, map[string]string{"contact_id": contactID})
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

	if req.IssueDate.IsZero() {
		req.IssueDate = time.Now()
	}

	if req.ContactID == "" {
		respondError(w, http.StatusBadRequest, "Contact is required")
		return
	}

	if len(req.Lines) == 0 {
		respondError(w, http.StatusBadRequest, "At least one line is required")
		return
	}

	if h.rejectLockedPeriod(w, r.Context(), tenantID, req.IssueDate) {
		return
	}

	invoice, err := h.invoicingService.Create(r.Context(), tenantID, schemaName, &req)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	h.emitWebhookEvent(plugin.EventInvoiceCreated, tenantID, invoice)
	respondJSON(w, http.StatusCreated, invoice)
}

// ImportInvoices imports invoices from CSV data.
// @Summary Import invoices
// @Description Import invoices from grouped CSV data and skip duplicate, invalid, or locked rows
// @Tags Invoices
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param request body invoicing.ImportInvoicesRequest true "CSV import payload"
// @Success 200 {object} invoicing.ImportInvoicesResult
// @Failure 400 {object} object{error=string}
// @Router /tenants/{tenantID}/invoices/import [post]
func (h *Handlers) ImportInvoices(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.GetClaims(r.Context())
	tenantID := chi.URLParam(r, "tenantID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	var req invoicing.ImportInvoicesRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if strings.TrimSpace(req.CSVContent) == "" {
		respondError(w, http.StatusBadRequest, "csv_content is required")
		return
	}

	if req.FileName == "" {
		req.FileName = "invoices_import.csv"
	}
	req.UserID = claims.UserID

	contactsList, err := h.contactsService.List(r.Context(), tenantID, schemaName, nil)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to load contacts")
		return
	}

	productsList, err := h.importProductList(r.Context(), tenantID, schemaName)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to load products")
		return
	}

	result, err := h.invoicingService.ImportCSV(r.Context(), tenantID, schemaName, contactsList, productsList, &req, func(issueDate time.Time) error {
		return h.ensurePeriodUnlocked(r.Context(), tenantID, issueDate)
	})
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, result)
}

func (h *Handlers) importProductList(ctx context.Context, tenantID, schemaName string) ([]inventory.Product, error) {
	if h.inventoryService == nil {
		return nil, nil
	}
	return h.inventoryService.ListProducts(ctx, tenantID, schemaName, nil)
}

// ImportEInvoice imports invoices from Estonian e-invoice XML data.
// @Summary Import Estonian e-invoice XML
// @Description Import invoices from manual Estonian e-invoice XML upload and skip duplicate, invalid, or locked invoices
// @Tags Invoices
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param request body invoicing.ImportEInvoiceRequest true "Estonian e-invoice XML import payload"
// @Success 200 {object} invoicing.ImportInvoicesResult
// @Failure 400 {object} object{error=string}
// @Router /tenants/{tenantID}/invoices/import-einvoice [post]
func (h *Handlers) ImportEInvoice(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.GetClaims(r.Context())
	tenantID := chi.URLParam(r, "tenantID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	var req invoicing.ImportEInvoiceRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if strings.TrimSpace(req.XMLContent) == "" {
		respondError(w, http.StatusBadRequest, "xml_content is required")
		return
	}

	if req.FileName == "" {
		req.FileName = "einvoice_import.xml"
	}
	req.UserID = claims.UserID

	contactsList, err := h.contactsService.List(r.Context(), tenantID, schemaName, nil)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to load contacts")
		return
	}

	result, err := h.invoicingService.ImportEInvoiceXML(r.Context(), tenantID, schemaName, contactsList, &req, func(issueDate time.Time) error {
		return h.ensurePeriodUnlocked(r.Context(), tenantID, issueDate)
	})
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, result)
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
// @Description Mark an invoice as sent to the customer. Draft purchase invoices require approved invoice evidence before sending.
// @Tags Invoices
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param invoiceID path string true "Invoice ID"
// @Success 200 {object} object{status=string}
// @Failure 400 {object} object{error=string}
// @Failure 409 {object} object{error=string,evidence_policy_results=[]documents.EvidencePolicyResult,remediation_actions=[]documents.DocumentRemediationAction}
// @Router /tenants/{tenantID}/invoices/{invoiceID}/send [post]
func (h *Handlers) SendInvoice(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	invoiceID := chi.URLParam(r, "invoiceID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	if err := h.requireApprovedPurchaseInvoiceEvidence(r.Context(), schemaName, tenantID, invoiceID); err != nil {
		var conflict *evidencePolicyConflictError
		if errors.As(err, &conflict) {
			respondEvidencePolicyConflict(w, conflict.Error(), conflict.Results)
			return
		}
		status := http.StatusInternalServerError
		switch {
		case errors.Is(err, errApprovedPurchaseInvoiceEvidenceRequired):
			status = http.StatusConflict
		case strings.Contains(err.Error(), "get invoice"):
			status = http.StatusBadRequest
		}
		respondError(w, status, err.Error())
		return
	}

	if err := h.invoicingService.Send(r.Context(), tenantID, schemaName, invoiceID); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	h.emitWebhookEvent(plugin.EventInvoiceSent, tenantID, map[string]string{"invoice_id": invoiceID})
	respondJSON(w, http.StatusOK, map[string]string{"status": "sent"})
}

func (h *Handlers) requireApprovedPurchaseInvoiceEvidence(ctx context.Context, schemaName, tenantID, invoiceID string) error {
	invoice, err := h.invoicingService.GetByID(ctx, tenantID, schemaName, invoiceID)
	if err != nil {
		return fmt.Errorf("get invoice: %w", err)
	}
	if invoice.InvoiceType != invoicing.InvoiceTypePurchase || invoice.Status != invoicing.StatusDraft {
		return nil
	}
	if h.documentsService == nil {
		return fmt.Errorf("%w before sending purchase invoice %s", errApprovedPurchaseInvoiceEvidenceRequired, invoiceID)
	}

	results, err := evaluateDocumentsEvidencePolicy(h.documentsService, ctx, schemaName, tenantID, &documents.EvidencePolicyRequest{
		EntityType: documents.EntityTypeInvoice,
		EntityIDs:  []string{invoiceID},
		Rules: []documents.EvidencePolicyRule{{
			DocumentTypes: []string{
				documents.DocumentTypeReceipt,
				documents.DocumentTypeSupportingDocument,
				documents.DocumentTypeTaxSupport,
			},
			MinCount:        1,
			RequireApproved: true,
		}},
	})
	if err != nil {
		return fmt.Errorf("evaluate purchase invoice evidence: %w", err)
	}
	if len(results) == 0 {
		return fmt.Errorf("%w before sending purchase invoice %s", errApprovedPurchaseInvoiceEvidenceRequired, invoiceID)
	}
	for _, result := range results {
		if !result.Compliant {
			return &evidencePolicyConflictError{
				Err:     fmt.Errorf("%w before sending purchase invoice %s", errApprovedPurchaseInvoiceEvidenceRequired, invoiceID),
				Results: results,
			}
		}
	}

	return nil
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

	invoice, err := h.invoicingService.GetByID(r.Context(), tenantID, schemaName, invoiceID)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	if h.rejectLockedPeriod(w, r.Context(), tenantID, invoice.IssueDate) {
		return
	}

	if err := h.invoicingService.Void(r.Context(), tenantID, schemaName, invoiceID); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	h.emitWebhookEvent(plugin.EventInvoiceVoided, tenantID, map[string]string{"invoice_id": invoiceID})
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
	pdfBytes, err := generateInvoicePDF(h.pdfService, invoice, t, pdfSettings)
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
	if method := r.URL.Query().Get("method"); method != "" {
		filter.PaymentMethod = method
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

	if req.PaymentDate.IsZero() {
		req.PaymentDate = time.Now()
	}

	if req.Amount.LessThanOrEqual(decimal.Zero) {
		respondError(w, http.StatusBadRequest, "Amount must be positive")
		return
	}

	if h.rejectLockedPeriod(w, r.Context(), tenantID, req.PaymentDate) {
		return
	}

	payment, err := h.paymentsService.Create(r.Context(), tenantID, schemaName, &req)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	h.emitWebhookEvent(plugin.EventPaymentReceived, tenantID, payment)
	respondJSON(w, http.StatusCreated, payment)
}

// ImportPayments imports payment history from CSV
// @Summary Import payments
// @Description Import historical payments from CSV, preserving supplied payment numbers and optional invoice allocations
// @Tags Payments
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param request body payments.ImportPaymentsRequest true "CSV import payload"
// @Success 200 {object} payments.ImportPaymentsResult
// @Failure 400 {object} object{error=string}
// @Router /tenants/{tenantID}/payments/import [post]
func (h *Handlers) ImportPayments(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.GetClaims(r.Context())
	tenantID := chi.URLParam(r, "tenantID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	var req payments.ImportPaymentsRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if strings.TrimSpace(req.CSVContent) == "" {
		respondError(w, http.StatusBadRequest, "csv_content is required")
		return
	}
	req.UserID = claims.UserID

	lockDate, err := h.getTenantPeriodLockDate(r.Context(), tenantID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to validate period lock")
		return
	}
	req.LockDate = lockDate

	result, err := h.paymentsService.ImportPaymentsCSV(r.Context(), tenantID, schemaName, &req)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, result)
}

// ExportSEPAPayments exports SEPA credit-transfer XML for bank upload
// @Summary Export SEPA payment file
// @Description Generate an ISO 20022 pain.001.001.03 SEPA credit-transfer XML file for manual bank upload
// @Tags Payments
// @Accept json
// @Produce application/xml
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param request body payments.SEPAExportRequest true "SEPA export details"
// @Success 200 {string} string "SEPA pain.001 XML"
// @Failure 400 {object} object{error=string}
// @Router /tenants/{tenantID}/payments/sepa-export [post]
func (h *Handlers) ExportSEPAPayments(w http.ResponseWriter, r *http.Request) {
	var req payments.SEPAExportRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	result, err := payments.BuildSEPAExport(&req)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondReportXML(w, result.FileName, []byte(result.XML))
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

	h.emitWebhookEvent(plugin.EventPaymentAllocated, tenantID, map[string]string{
		"payment_id": paymentID,
		"invoice_id": req.InvoiceID,
		"amount":     req.Amount.String(),
	})
	respondJSON(w, http.StatusOK, map[string]string{"status": "allocated"})
}

// ReversePayment creates an auditable offsetting payment
// @Summary Reverse payment
// @Description Create an offsetting payment, mark the original as reversed, and mirror invoice allocation reversals
// @Tags Payments
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param paymentID path string true "Payment ID"
// @Param request body payments.ReversePaymentRequest true "Reversal details"
// @Success 201 {object} payments.PaymentReversalResult
// @Failure 400 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Failure 409 {object} object{error=string}
// @Router /tenants/{tenantID}/payments/{paymentID}/reverse [post]
func (h *Handlers) ReversePayment(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.GetClaims(r.Context())
	tenantID := chi.URLParam(r, "tenantID")
	paymentID := chi.URLParam(r, "paymentID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	var req payments.ReversePaymentRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	req.UserID = claims.UserID
	if req.PaymentDate.IsZero() {
		req.PaymentDate = time.Now()
	}
	if h.rejectLockedPeriod(w, r.Context(), tenantID, req.PaymentDate) {
		return
	}

	result, err := h.paymentsService.Reverse(r.Context(), tenantID, schemaName, paymentID, &req)
	if err != nil {
		switch {
		case errors.Is(err, payments.ErrPaymentNotFound):
			respondError(w, http.StatusNotFound, "Payment not found")
		case errors.Is(err, payments.ErrPaymentAlreadyReversed):
			respondError(w, http.StatusConflict, "Payment already reversed")
		case errors.Is(err, payments.ErrPaymentReversalNotAllowed):
			respondError(w, http.StatusConflict, err.Error())
		default:
			respondError(w, http.StatusBadRequest, err.Error())
		}
		return
	}

	respondJSON(w, http.StatusCreated, result)
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

	result, err := testSMTPWithService(r.Context(), h.emailService, tenantID, req.RecipientEmail)
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
// @Param templateType path string true "Template type (INVOICE_SEND, QUOTE_SEND, ORDER_CONFIRM, PAYMENT_RECEIPT, OVERDUE_REMINDER, DOCUMENT_RETENTION_REMINDER)"
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
// @Description Send an invoice to a recipient via email. Draft purchase invoices require approved invoice evidence before emailing.
// @Tags Email
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param invoiceID path string true "Invoice ID"
// @Param request body email.SendInvoiceRequest true "Email details"
// @Success 200 {object} email.EmailSentResponse
// @Failure 400 {object} object{error=string}
// @Failure 409 {object} object{error=string,evidence_policy_results=[]documents.EvidencePolicyResult,remediation_actions=[]documents.DocumentRemediationAction}
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

	if err := h.requireApprovedPurchaseInvoiceEvidence(r.Context(), schemaName, tenantID, invoiceID); err != nil {
		var conflict *evidencePolicyConflictError
		if errors.As(err, &conflict) {
			respondEvidencePolicyConflict(w, conflict.Error(), conflict.Results)
			return
		}
		status := http.StatusInternalServerError
		switch {
		case errors.Is(err, errApprovedPurchaseInvoiceEvidenceRequired):
			status = http.StatusConflict
		case strings.Contains(err.Error(), "get invoice"):
			status = http.StatusBadRequest
		}
		respondError(w, status, err.Error())
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
		pdfBytes, err := generateInvoicePDF(h.pdfService, invoice, t, pdfSettings)
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

// EmailQuote sends a quote via email
// @Summary Email quote
// @Description Send a quote to a recipient via email, optionally requiring approved quote evidence first
// @Tags Email
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param quoteID path string true "Quote ID"
// @Param request body email.SendQuoteRequest true "Email details"
// @Success 200 {object} email.EmailSentResponse
// @Failure 400 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Failure 409 {object} object{error=string,evidence_policy_results=[]documents.EvidencePolicyResult,remediation_actions=[]documents.DocumentRemediationAction}
// @Router /tenants/{tenantID}/quotes/{quoteID}/email [post]
func (h *Handlers) EmailQuote(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	quoteID := chi.URLParam(r, "quoteID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	var req email.SendQuoteRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if err := req.Validate(); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	quote, err := h.quotesService.GetByID(r.Context(), tenantID, schemaName, quoteID)
	if err != nil {
		respondError(w, http.StatusNotFound, "Quote not found")
		return
	}

	if err := h.requireApprovedCommercialEvidence(r.Context(), schemaName, tenantID, documents.EntityTypeQuote, quoteID, req.RequireApprovedEvidence, errApprovedQuoteEvidenceRequired, "emailing quote"); err != nil {
		var conflict *evidencePolicyConflictError
		if errors.As(err, &conflict) {
			respondEvidencePolicyConflict(w, conflict.Error(), conflict.Results)
			return
		}
		status := http.StatusInternalServerError
		if errors.Is(err, errApprovedQuoteEvidenceRequired) {
			status = http.StatusConflict
		}
		respondError(w, status, err.Error())
		return
	}

	t, err := h.tenantService.GetTenant(r.Context(), tenantID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get tenant")
		return
	}

	template, err := h.emailService.GetTemplate(r.Context(), schemaName, tenantID, email.TemplateQuoteSend)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get email template")
		return
	}

	data := &email.TemplateData{
		CompanyName: t.Name,
		ContactName: req.RecipientName,
		QuoteNumber: quote.QuoteNumber,
		TotalAmount: quote.Total.StringFixed(2),
		Currency:    quote.Currency,
		QuoteDate:   quote.QuoteDate.Format("2006-01-02"),
		Message:     req.Message,
	}
	if quote.ValidUntil != nil {
		data.ValidUntil = quote.ValidUntil.Format("2006-01-02")
	}

	subject, bodyHTML, bodyText, err := h.emailService.RenderTemplate(template, data)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to render email template")
		return
	}
	if req.Subject != "" {
		subject = req.Subject
	}

	var attachments []email.Attachment
	if req.AttachPDF {
		pdfSettings := h.pdfService.PDFSettingsFromTenant(t)
		pdfBytes, err := generateQuotePDF(h.pdfService, quote, t, pdfSettings)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "Failed to generate PDF")
			return
		}
		attachments = append(attachments, email.Attachment{
			Filename:    "quote-" + quote.QuoteNumber + ".pdf",
			Content:     pdfBytes,
			ContentType: "application/pdf",
		})
	}

	result, err := h.emailService.SendEmail(r.Context(), schemaName, tenantID, string(email.TemplateQuoteSend), req.RecipientEmail, req.RecipientName, subject, bodyHTML, bodyText, attachments, quoteID)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	if quote.Status == quotes.QuoteStatusDraft {
		_ = h.quotesService.Send(r.Context(), tenantID, schemaName, quoteID)
	}

	respondJSON(w, http.StatusOK, result)
}

// EmailOrder sends an order confirmation via email
// @Summary Email order
// @Description Send an order confirmation to a recipient via email, optionally requiring approved order evidence first
// @Tags Email
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param orderID path string true "Order ID"
// @Param request body email.SendOrderRequest true "Email details"
// @Success 200 {object} email.EmailSentResponse
// @Failure 400 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Failure 409 {object} object{error=string,evidence_policy_results=[]documents.EvidencePolicyResult,remediation_actions=[]documents.DocumentRemediationAction}
// @Router /tenants/{tenantID}/orders/{orderID}/email [post]
func (h *Handlers) EmailOrder(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	orderID := chi.URLParam(r, "orderID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	var req email.SendOrderRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if err := req.Validate(); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	order, err := h.ordersService.GetByID(r.Context(), tenantID, schemaName, orderID)
	if err != nil {
		respondError(w, http.StatusNotFound, "Order not found")
		return
	}

	if err := h.requireApprovedCommercialEvidence(r.Context(), schemaName, tenantID, documents.EntityTypeOrder, orderID, req.RequireApprovedEvidence, errApprovedOrderEvidenceRequired, "emailing order"); err != nil {
		var conflict *evidencePolicyConflictError
		if errors.As(err, &conflict) {
			respondEvidencePolicyConflict(w, conflict.Error(), conflict.Results)
			return
		}
		status := http.StatusInternalServerError
		if errors.Is(err, errApprovedOrderEvidenceRequired) {
			status = http.StatusConflict
		}
		respondError(w, status, err.Error())
		return
	}

	t, err := h.tenantService.GetTenant(r.Context(), tenantID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get tenant")
		return
	}

	template, err := h.emailService.GetTemplate(r.Context(), schemaName, tenantID, email.TemplateOrderConfirm)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get email template")
		return
	}

	data := &email.TemplateData{
		CompanyName: t.Name,
		ContactName: req.RecipientName,
		OrderNumber: order.OrderNumber,
		TotalAmount: order.Total.StringFixed(2),
		Currency:    order.Currency,
		OrderDate:   order.OrderDate.Format("2006-01-02"),
		Message:     req.Message,
	}
	if order.ExpectedDelivery != nil {
		data.ExpectedDelivery = order.ExpectedDelivery.Format("2006-01-02")
	}

	subject, bodyHTML, bodyText, err := h.emailService.RenderTemplate(template, data)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to render email template")
		return
	}
	if req.Subject != "" {
		subject = req.Subject
	}

	var attachments []email.Attachment
	if req.AttachPDF {
		pdfSettings := h.pdfService.PDFSettingsFromTenant(t)
		pdfBytes, err := generateOrderPDF(h.pdfService, order, t, pdfSettings)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "Failed to generate PDF")
			return
		}
		attachments = append(attachments, email.Attachment{
			Filename:    "order-" + order.OrderNumber + ".pdf",
			Content:     pdfBytes,
			ContentType: "application/pdf",
		})
	}

	result, err := h.emailService.SendEmail(r.Context(), schemaName, tenantID, string(email.TemplateOrderConfirm), req.RecipientEmail, req.RecipientName, subject, bodyHTML, bodyText, attachments, orderID)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	if order.Status == orders.OrderStatusPending {
		_ = h.ordersService.Confirm(r.Context(), tenantID, schemaName, orderID)
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
// @Failure 409 {object} object{error=string,evidence_policy_results=[]documents.EvidencePolicyResult,remediation_actions=[]documents.DocumentRemediationAction}
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

	if err := h.requireApprovedPaymentReceiptEvidence(r.Context(), schemaName, tenantID, paymentID, req.RequireApprovedEvidence); err != nil {
		var conflict *evidencePolicyConflictError
		if errors.As(err, &conflict) {
			respondEvidencePolicyConflict(w, conflict.Error(), conflict.Results)
			return
		}
		if errors.Is(err, errApprovedPaymentReceiptEvidenceRequired) {
			respondError(w, http.StatusConflict, err.Error())
			return
		}
		respondError(w, http.StatusInternalServerError, "Failed to verify payment receipt evidence")
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

	h.emitWebhookEvent(plugin.EventBankTransactionImported, tenantID, result)
	respondJSON(w, http.StatusOK, result)
}

func (h *Handlers) requireApprovedPaymentReceiptEvidence(ctx context.Context, schemaName, tenantID, paymentID string, requireApproved bool) error {
	if !requireApproved {
		return nil
	}
	if h.documentsService == nil {
		return fmt.Errorf("%w before sending payment receipt %s", errApprovedPaymentReceiptEvidenceRequired, paymentID)
	}

	results, err := h.documentsService.EvaluateEvidencePolicy(ctx, schemaName, tenantID, &documents.EvidencePolicyRequest{
		EntityType: documents.EntityTypePayment,
		EntityIDs:  []string{paymentID},
		Rules: []documents.EvidencePolicyRule{{
			DocumentTypes: []string{
				documents.DocumentTypeReceipt,
				documents.DocumentTypeSupportingDocument,
				documents.DocumentTypeTaxSupport,
			},
			MinCount:        1,
			RequireApproved: true,
		}},
	})
	if err != nil {
		return fmt.Errorf("evaluate payment receipt evidence: %w", err)
	}
	for _, result := range results {
		if !result.Compliant {
			return &evidencePolicyConflictError{
				Err:     fmt.Errorf("%w before sending payment receipt %s", errApprovedPaymentReceiptEvidenceRequired, paymentID),
				Results: results,
			}
		}
	}
	return nil
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

// ImportBankAccounts imports bank account master data.
// @Summary Import bank accounts
// @Description Import bank account master data from CSV rows for incumbent-system cutover.
// @Tags Banking
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param request body banking.ImportBankAccountsRequest true "Bank account import data"
// @Success 200 {object} banking.ImportBankAccountsResult
// @Failure 400 {object} object{error=string}
// @Router /tenants/{tenantID}/bank-accounts/import [post]
func (h *Handlers) ImportBankAccounts(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	var req banking.ImportBankAccountsRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if len(req.Rows) == 0 {
		respondError(w, http.StatusBadRequest, "No bank accounts to import")
		return
	}
	if strings.TrimSpace(req.FileName) == "" {
		req.FileName = "bank_accounts_import.csv"
	}

	result, err := h.bankingService.ImportBankAccounts(r.Context(), schemaName, tenantID, &req)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, result)
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

// ListBankMatchRules lists bank auto-match rules for a tenant
// @Summary List bank auto-match rules
// @Description Get bank auto-match rules, optionally scoped to a bank account
// @Tags Banking
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param bank_account_id query string false "Filter to rules for a bank account"
// @Param active_only query bool false "Only active rules"
// @Param include_global query bool false "Include tenant-wide rules when filtering by bank account"
// @Success 200 {array} banking.BankMatchRule
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/bank-match-rules [get]
func (h *Handlers) ListBankMatchRules(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	filter := &banking.BankMatchRuleFilter{
		BankAccountID: strings.TrimSpace(r.URL.Query().Get("bank_account_id")),
		ActiveOnly:    r.URL.Query().Get("active_only") == "true",
		IncludeGlobal: r.URL.Query().Get("include_global") == "true",
	}
	rules, err := h.bankingService.ListBankMatchRules(r.Context(), schemaName, tenantID, filter)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to list bank match rules")
		return
	}
	respondJSON(w, http.StatusOK, rules)
}

// CreateBankMatchRule creates a bank auto-match rule
// @Summary Create bank auto-match rule
// @Description Create a transaction-pattern rule that tunes automatic payment matching
// @Tags Banking
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param request body banking.CreateBankMatchRuleRequest true "Bank match rule details"
// @Success 201 {object} banking.BankMatchRule
// @Failure 400 {object} object{error=string}
// @Router /tenants/{tenantID}/bank-match-rules [post]
func (h *Handlers) CreateBankMatchRule(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	var req banking.CreateBankMatchRuleRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	rule, err := h.bankingService.CreateBankMatchRule(r.Context(), schemaName, tenantID, &req)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	respondJSON(w, http.StatusCreated, rule)
}

// GetBankMatchRule gets a bank auto-match rule by ID
// @Summary Get bank auto-match rule
// @Description Get one bank auto-match rule
// @Tags Banking
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param ruleID path string true "Rule ID"
// @Success 200 {object} banking.BankMatchRule
// @Failure 404 {object} object{error=string}
// @Router /tenants/{tenantID}/bank-match-rules/{ruleID} [get]
func (h *Handlers) GetBankMatchRule(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	ruleID := chi.URLParam(r, "ruleID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	rule, err := h.bankingService.GetBankMatchRule(r.Context(), schemaName, tenantID, ruleID)
	if err != nil {
		respondError(w, http.StatusNotFound, "Bank match rule not found")
		return
	}
	respondJSON(w, http.StatusOK, rule)
}

// UpdateBankMatchRule updates a bank auto-match rule
// @Summary Update bank auto-match rule
// @Description Update one bank auto-match rule
// @Tags Banking
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param ruleID path string true "Rule ID"
// @Param request body banking.UpdateBankMatchRuleRequest true "Bank match rule updates"
// @Success 200 {object} banking.BankMatchRule
// @Failure 400 {object} object{error=string}
// @Router /tenants/{tenantID}/bank-match-rules/{ruleID} [put]
func (h *Handlers) UpdateBankMatchRule(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	ruleID := chi.URLParam(r, "ruleID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	var req banking.UpdateBankMatchRuleRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	rule, err := h.bankingService.UpdateBankMatchRule(r.Context(), schemaName, tenantID, ruleID, &req)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, rule)
}

// DeleteBankMatchRule deletes a bank auto-match rule
// @Summary Delete bank auto-match rule
// @Description Delete one bank auto-match rule
// @Tags Banking
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param ruleID path string true "Rule ID"
// @Success 204
// @Failure 404 {object} object{error=string}
// @Router /tenants/{tenantID}/bank-match-rules/{ruleID} [delete]
func (h *Handlers) DeleteBankMatchRule(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	ruleID := chi.URLParam(r, "ruleID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	if err := h.bankingService.DeleteBankMatchRule(r.Context(), schemaName, tenantID, ruleID); err != nil {
		respondError(w, http.StatusNotFound, err.Error())
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
// @Description Import bank transactions from normalized rows or raw statement data. Raw format supports auto, generic, lhv, camt053, and lhv-camt.
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

	if req.FileName == "" {
		req.FileName = "manual_import.csv"
	}
	if len(req.Transactions) == 0 && strings.TrimSpace(req.CSVContent) != "" {
		rows, err := registry.ParseTransactions(req.CSVContent, req.Format)
		if err != nil {
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}
		req.Transactions = rows
	}
	if len(req.Transactions) == 0 {
		respondError(w, http.StatusBadRequest, "No transactions to import")
		return
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

	h.emitWebhookEvent(plugin.EventBankTransactionMatched, tenantID, map[string]string{
		"transaction_id": transactionID,
		"payment_id":     req.PaymentID,
	})
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

// ReviewBankTransaction updates accountant follow-up guidance on a bank transaction.
// @Summary Review bank transaction
// @Description Update follow-up status and review note for a bank transaction
// @Tags Banking
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param transactionID path string true "Transaction ID"
// @Param request body banking.UpdateTransactionReviewRequest true "Review update"
// @Success 200 {object} banking.BankTransaction
// @Failure 400 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Router /tenants/{tenantID}/bank-transactions/{transactionID}/review [post]
func (h *Handlers) ReviewBankTransaction(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.GetClaims(r.Context())
	tenantID := chi.URLParam(r, "tenantID")
	transactionID := chi.URLParam(r, "transactionID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	var req banking.UpdateTransactionReviewRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	transaction, err := h.bankingService.UpdateTransactionReview(r.Context(), schemaName, tenantID, transactionID, claims.UserID, &req)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, banking.ErrTransactionNotFound) {
			status = http.StatusNotFound
		}
		respondError(w, status, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, transaction)
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

	transaction, err := h.bankingService.GetTransaction(r.Context(), schemaName, tenantID, transactionID)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	if h.rejectLockedPeriod(w, r.Context(), tenantID, transaction.TransactionDate) {
		return
	}

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
// @Description Mark a reconciliation session as complete. Matched transactions marked EVIDENCE_REQUIRED must have approved reconciliation evidence before completion.
// @Tags Banking
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param reconciliationID path string true "Reconciliation ID"
// @Success 200 {object} object{status=string}
// @Failure 400 {object} object{error=string}
// @Failure 409 {object} object{error=string,evidence_policy_results=[]documents.EvidencePolicyResult,remediation_actions=[]documents.DocumentRemediationAction}
// @Router /tenants/{tenantID}/reconciliations/{reconciliationID}/complete [post]
func (h *Handlers) CompleteReconciliation(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	reconciliationID := chi.URLParam(r, "reconciliationID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	if err := h.requireApprovedReconciliationEvidence(r.Context(), schemaName, tenantID, reconciliationID); err != nil {
		var conflict *evidencePolicyConflictError
		if errors.As(err, &conflict) {
			respondEvidencePolicyConflict(w, conflict.Error(), conflict.Results)
			return
		}
		status := http.StatusInternalServerError
		if errors.Is(err, errApprovedReconciliationEvidenceRequired) {
			status = http.StatusConflict
		}
		respondError(w, status, err.Error())
		return
	}

	if err := h.bankingService.CompleteReconciliation(r.Context(), schemaName, tenantID, reconciliationID); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	h.emitWebhookEvent(plugin.EventReconciliationCompleted, tenantID, map[string]string{"reconciliation_id": reconciliationID})
	respondJSON(w, http.StatusOK, map[string]string{"status": "completed"})
}

func (h *Handlers) requireApprovedReconciliationEvidence(ctx context.Context, schemaName, tenantID, reconciliationID string) error {
	transactions, err := h.bankingService.ListTransactions(ctx, schemaName, tenantID, &banking.TransactionFilter{
		ReconciliationID: reconciliationID,
		Status:           banking.StatusMatched,
	})
	if err != nil {
		return fmt.Errorf("load reconciliation transactions: %w", err)
	}

	transactionIDs := make([]string, 0, len(transactions))
	for _, transaction := range transactions {
		if transaction.FollowUpStatus == banking.FollowUpEvidenceRequired {
			transactionIDs = append(transactionIDs, transaction.ID)
		}
	}
	if len(transactionIDs) == 0 {
		return nil
	}
	if h.documentsService == nil {
		return fmt.Errorf("%w before completing reconciliation for bank transactions: %s", errApprovedReconciliationEvidenceRequired, strings.Join(transactionIDs, ", "))
	}

	results, err := h.documentsService.EvaluateEvidencePolicy(ctx, schemaName, tenantID, &documents.EvidencePolicyRequest{
		EntityType: documents.EntityTypeBankTxn,
		EntityIDs:  transactionIDs,
		Rules: []documents.EvidencePolicyRule{{
			DocumentTypes:   []string{documents.DocumentTypeReconciliation},
			MinCount:        1,
			RequireApproved: true,
		}},
	})
	if err != nil {
		return err
	}

	failingIDs := make([]string, 0, len(results))
	for _, result := range results {
		if !result.Compliant {
			failingIDs = append(failingIDs, result.EntityID)
		}
	}
	if len(failingIDs) > 0 {
		return &evidencePolicyConflictError{
			Err:     fmt.Errorf("%w before completing reconciliation for bank transactions: %s", errApprovedReconciliationEvidenceRequired, strings.Join(failingIDs, ", ")),
			Results: results,
		}
	}

	return nil
}

type evidencePolicyConflictError struct {
	Err     error
	Results []documents.EvidencePolicyResult
}

func (e *evidencePolicyConflictError) Error() string {
	if e == nil || e.Err == nil {
		return ""
	}
	return e.Err.Error()
}

func (e *evidencePolicyConflictError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func respondEvidencePolicyConflict(w http.ResponseWriter, message string, results []documents.EvidencePolicyResult) {
	respondJSON(w, http.StatusConflict, struct {
		Error                 string                                `json:"error"`
		EvidencePolicyResults []documents.EvidencePolicyResult      `json:"evidence_policy_results,omitempty"`
		RemediationActions    []documents.DocumentRemediationAction `json:"remediation_actions,omitempty"`
	}{
		Error:                 message,
		EvidencePolicyResults: results,
		RemediationActions:    flattenEvidencePolicyRemediationActions(results),
	})
}

func flattenEvidencePolicyRemediationActions(results []documents.EvidencePolicyResult) []documents.DocumentRemediationAction {
	actions := make([]documents.DocumentRemediationAction, 0)
	for _, result := range results {
		actions = append(actions, result.RemediationActions...)
	}
	return actions
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

// ListTenantUserAuthSessions returns refresh-token sessions for one tenant user.
// @Summary List tenant user auth sessions
// @Description List refresh-token sessions for a user who belongs to the tenant. Requires owner or admin role.
// @Tags Users
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param userID path string true "User ID"
// @Param include_inactive query bool false "Include revoked and expired sessions"
// @Success 200 {array} auth.RefreshSession
// @Failure 400 {object} object{error=string}
// @Failure 403 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/users/{userID}/sessions [get]
func (h *Handlers) ListTenantUserAuthSessions(w http.ResponseWriter, r *http.Request) {
	_, _, ok := h.authorizeTenantUserAdmin(w, r)
	if !ok {
		return
	}
	if h.refreshSessionService == nil {
		respondError(w, http.StatusInternalServerError, "Refresh session service unavailable")
		return
	}

	userID := chi.URLParam(r, "userID")
	includeInactive := strings.EqualFold(r.URL.Query().Get("include_inactive"), "true")
	sessions, err := h.refreshSessionService.ListRefreshSessions(r.Context(), userID, includeInactive)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to list refresh sessions")
		return
	}

	respondJSON(w, http.StatusOK, sessions)
}

// ListTenantUserSecurityAuditEvents returns auth security audit events for one tenant user.
// @Summary List tenant user security audit events
// @Description List recent auth security events where the tenant user is actor or target. Requires owner or admin role.
// @Tags Users
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param userID path string true "User ID"
// @Param limit query int false "Maximum events to return" default(50)
// @Success 200 {array} auth.SecurityAuditEvent
// @Failure 400 {object} object{error=string}
// @Failure 403 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/users/{userID}/security-events [get]
func (h *Handlers) ListTenantUserSecurityAuditEvents(w http.ResponseWriter, r *http.Request) {
	_, _, ok := h.authorizeTenantUserAdmin(w, r)
	if !ok {
		return
	}
	if h.securityAuditService == nil {
		respondError(w, http.StatusInternalServerError, "Security audit service unavailable")
		return
	}

	limit := 50
	if rawLimit := strings.TrimSpace(r.URL.Query().Get("limit")); rawLimit != "" {
		parsed, err := strconv.Atoi(rawLimit)
		if err != nil || parsed <= 0 || parsed > 200 {
			respondError(w, http.StatusBadRequest, "Limit must be between 1 and 200")
			return
		}
		limit = parsed
	}

	events, err := h.securityAuditService.ListUserEvents(r.Context(), chi.URLParam(r, "userID"), limit)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to list security audit events")
		return
	}

	respondJSON(w, http.StatusOK, events)
}

// UpdateTenantUserStatus suspends or restores one tenant user's access.
// @Summary Update tenant user status
// @Description Suspend or restore a tenant user's membership. Suspended users cannot log in or refresh tokens for the tenant. Suspending revokes active refresh sessions.
// @Tags Users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param userID path string true "User ID"
// @Param request body object{is_active=bool} true "Membership status"
// @Success 200 {object} object{status=string,is_active=bool}
// @Failure 400 {object} object{error=string}
// @Failure 403 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/users/{userID}/status [put]
func (h *Handlers) UpdateTenantUserStatus(w http.ResponseWriter, r *http.Request) {
	claims, targetRole, ok := h.authorizeTenantUserAdmin(w, r)
	if !ok {
		return
	}

	tenantID := chi.URLParam(r, "tenantID")
	userID := chi.URLParam(r, "userID")
	if userID == claims.UserID {
		respondError(w, http.StatusBadRequest, "Cannot update your own tenant access status")
		return
	}

	var req struct {
		IsActive *bool `json:"is_active"`
	}
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if req.IsActive == nil {
		respondError(w, http.StatusBadRequest, "is_active is required")
		return
	}

	current, err := h.tenantService.GetTenantUser(r.Context(), tenantID, userID)
	if err != nil {
		respondError(w, http.StatusNotFound, "User not found in tenant")
		return
	}
	if !*req.IsActive && h.refreshSessionService == nil {
		respondError(w, http.StatusInternalServerError, "Refresh session service unavailable")
		return
	}

	if err := h.tenantService.SetTenantUserActive(r.Context(), tenantID, userID, *req.IsActive); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	if !*req.IsActive {
		if err := h.refreshSessionService.RevokeAllRefreshSessions(r.Context(), userID); err != nil {
			respondError(w, http.StatusInternalServerError, "Failed to revoke refresh sessions")
			return
		}
	}

	targetEmail := h.userEmailForAudit(r.Context(), userID)
	securityAction := auth.SecurityAuditActionTenantAccessSuspended
	if *req.IsActive {
		securityAction = auth.SecurityAuditActionTenantAccessRestored
	}
	h.recordSecurityAuditEvent(r, &auth.SecurityAuditEvent{
		ActorUserID:  claims.UserID,
		ActorEmail:   claims.Email,
		Action:       securityAction,
		TargetUserID: userID,
		TargetEmail:  targetEmail,
		Metadata: map[string]string{
			"tenant_id":       tenantID,
			"previous_active": strconv.FormatBool(current.IsActive),
			"new_active":      strconv.FormatBool(*req.IsActive),
		},
	})
	if !h.recordTenantAuditEvent(w, r, &tenant.TenantAuditEvent{
		TenantID:    tenantID,
		ActorUserID: claims.UserID,
		Action:      tenant.AuditActionUserStatusUpdated,
		TargetType:  tenant.AuditTargetUser,
		TargetID:    userID,
		TargetEmail: targetEmail,
		Metadata: map[string]string{
			"role":            targetRole,
			"previous_active": strconv.FormatBool(current.IsActive),
			"new_active":      strconv.FormatBool(*req.IsActive),
		},
	}) {
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{"status": "updated", "is_active": *req.IsActive})
}

// RevokeTenantUserAuthSession revokes one refresh-token session for a tenant user.
// @Summary Revoke tenant user auth session
// @Description Revoke one active refresh-token session for a user who belongs to the tenant. Requires owner or admin role.
// @Tags Users
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param userID path string true "User ID"
// @Param sessionID path string true "Refresh session ID"
// @Success 200 {object} object{status=string}
// @Failure 400 {object} object{error=string}
// @Failure 403 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/users/{userID}/sessions/{sessionID} [delete]
func (h *Handlers) RevokeTenantUserAuthSession(w http.ResponseWriter, r *http.Request) {
	claims, targetRole, ok := h.authorizeTenantUserAdmin(w, r)
	if !ok {
		return
	}
	if h.refreshSessionService == nil {
		respondError(w, http.StatusInternalServerError, "Refresh session service unavailable")
		return
	}

	tenantID := chi.URLParam(r, "tenantID")
	userID := chi.URLParam(r, "userID")
	sessionID := strings.TrimSpace(chi.URLParam(r, "sessionID"))
	if sessionID == "" {
		respondError(w, http.StatusBadRequest, "Session id is required")
		return
	}

	if err := h.refreshSessionService.RevokeRefreshSessionByID(r.Context(), userID, sessionID); err != nil {
		if errors.Is(err, auth.ErrRefreshSessionInvalid) {
			respondError(w, http.StatusNotFound, "Refresh session not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "Failed to revoke refresh session")
		return
	}
	targetEmail := h.userEmailForAudit(r.Context(), userID)
	h.recordSecurityAuditEvent(r, &auth.SecurityAuditEvent{
		ActorUserID:  claims.UserID,
		ActorEmail:   claims.Email,
		Action:       auth.SecurityAuditActionSessionRevoked,
		TargetUserID: userID,
		TargetEmail:  targetEmail,
		Metadata: map[string]string{
			"tenant_id":  tenantID,
			"session_id": sessionID,
		},
	})
	if !h.recordTenantAuditEvent(w, r, &tenant.TenantAuditEvent{
		TenantID:    tenantID,
		ActorUserID: claims.UserID,
		Action:      tenant.AuditActionUserSessionRevoked,
		TargetType:  tenant.AuditTargetUser,
		TargetID:    userID,
		TargetEmail: targetEmail,
		Metadata: map[string]string{
			"role":       targetRole,
			"session_id": sessionID,
		},
	}) {
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "revoked"})
}

// RevokeTenantUserAuthSessions revokes all refresh-token sessions for a tenant user.
// @Summary Revoke all tenant user auth sessions
// @Description Revoke every active refresh-token session for a user who belongs to the tenant. Requires owner or admin role.
// @Tags Users
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param userID path string true "User ID"
// @Success 200 {object} object{status=string}
// @Failure 400 {object} object{error=string}
// @Failure 403 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/users/{userID}/sessions [delete]
func (h *Handlers) RevokeTenantUserAuthSessions(w http.ResponseWriter, r *http.Request) {
	claims, targetRole, ok := h.authorizeTenantUserAdmin(w, r)
	if !ok {
		return
	}
	if h.refreshSessionService == nil {
		respondError(w, http.StatusInternalServerError, "Refresh session service unavailable")
		return
	}

	tenantID := chi.URLParam(r, "tenantID")
	userID := chi.URLParam(r, "userID")
	if err := h.refreshSessionService.RevokeAllRefreshSessions(r.Context(), userID); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to revoke refresh sessions")
		return
	}
	targetEmail := h.userEmailForAudit(r.Context(), userID)
	h.recordSecurityAuditEvent(r, &auth.SecurityAuditEvent{
		ActorUserID:  claims.UserID,
		ActorEmail:   claims.Email,
		Action:       auth.SecurityAuditActionAllSessionsRevoked,
		TargetUserID: userID,
		TargetEmail:  targetEmail,
		Metadata: map[string]string{
			"tenant_id": tenantID,
		},
	})
	if !h.recordTenantAuditEvent(w, r, &tenant.TenantAuditEvent{
		TenantID:    tenantID,
		ActorUserID: claims.UserID,
		Action:      tenant.AuditActionUserSessionsRevoked,
		TargetType:  tenant.AuditTargetUser,
		TargetID:    userID,
		TargetEmail: targetEmail,
		Metadata: map[string]string{
			"role": targetRole,
		},
	}) {
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "revoked"})
}

func (h *Handlers) authorizeTenantUserAdmin(w http.ResponseWriter, r *http.Request) (*auth.Claims, string, bool) {
	claims, _ := auth.GetClaims(r.Context())
	if !auth.CanManageUsers(claims.Role) {
		respondError(w, http.StatusForbidden, "Permission denied")
		return nil, "", false
	}

	tenantID := chi.URLParam(r, "tenantID")
	userID := strings.TrimSpace(chi.URLParam(r, "userID"))
	if userID == "" {
		respondError(w, http.StatusBadRequest, "User id is required")
		return nil, "", false
	}

	membership, err := h.tenantService.GetTenantUser(r.Context(), tenantID, userID)
	if err != nil {
		respondError(w, http.StatusNotFound, "User not found in tenant")
		return nil, "", false
	}

	return claims, membership.Role, true
}

// ListTenantAuditEvents returns tenant administration audit events
// @Summary List tenant audit events
// @Description Get recent tenant administration audit events
// @Tags Users
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param limit query int false "Maximum events to return" default(50)
// @Success 200 {array} tenant.TenantAuditEvent
// @Failure 400 {object} object{error=string}
// @Failure 403 {object} object{error=string}
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/audit-events [get]
func (h *Handlers) ListTenantAuditEvents(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.GetClaims(r.Context())
	tenantID := chi.URLParam(r, "tenantID")

	if !auth.CanManageUsers(claims.Role) {
		respondError(w, http.StatusForbidden, "Permission denied")
		return
	}

	limit := 50
	if rawLimit := strings.TrimSpace(r.URL.Query().Get("limit")); rawLimit != "" {
		parsed, err := strconv.Atoi(rawLimit)
		if err != nil || parsed <= 0 || parsed > 200 {
			respondError(w, http.StatusBadRequest, "Limit must be between 1 and 200")
			return
		}
		limit = parsed
	}

	events, err := h.tenantService.ListTenantAuditEvents(r.Context(), tenantID, limit)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to list audit events")
		return
	}

	respondJSON(w, http.StatusOK, events)
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

	previousRole, _ := h.tenantService.GetUserRole(r.Context(), tenantID, userID)
	if err := h.tenantService.RemoveTenantUser(r.Context(), tenantID, userID); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	if !h.recordTenantAuditEvent(w, r, &tenant.TenantAuditEvent{
		TenantID:    tenantID,
		ActorUserID: claims.UserID,
		Action:      tenant.AuditActionUserRemoved,
		TargetType:  tenant.AuditTargetUser,
		TargetID:    userID,
		Metadata: map[string]string{
			"previous_role": previousRole,
		},
	}) {
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

	if userID == claims.UserID {
		respondError(w, http.StatusBadRequest, "Cannot update your own role")
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

	previousRole, _ := h.tenantService.GetUserRole(r.Context(), tenantID, userID)
	if err := h.tenantService.UpdateTenantUserRole(r.Context(), tenantID, userID, req.Role); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	if !h.recordTenantAuditEvent(w, r, &tenant.TenantAuditEvent{
		TenantID:    tenantID,
		ActorUserID: claims.UserID,
		Action:      tenant.AuditActionUserRoleUpdated,
		TargetType:  tenant.AuditTargetUser,
		TargetID:    userID,
		Metadata: map[string]string{
			"previous_role": previousRole,
			"new_role":      req.Role,
		},
	}) {
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
	if !h.recordTenantAuditEvent(w, r, &tenant.TenantAuditEvent{
		TenantID:    tenantID,
		ActorUserID: claims.UserID,
		Action:      tenant.AuditActionInvitationCreated,
		TargetType:  tenant.AuditTargetInvitation,
		TargetID:    invitation.ID,
		TargetEmail: invitation.Email,
		Metadata: map[string]string{
			"role": invitation.Role,
		},
	}) {
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
	if !h.recordTenantAuditEvent(w, r, &tenant.TenantAuditEvent{
		TenantID:    tenantID,
		ActorUserID: claims.UserID,
		Action:      tenant.AuditActionInvitationRevoked,
		TargetType:  tenant.AuditTargetInvitation,
		TargetID:    invitationID,
	}) {
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "revoked"})
}

func (h *Handlers) recordTenantAuditEvent(w http.ResponseWriter, r *http.Request, event *tenant.TenantAuditEvent) bool {
	if err := h.tenantService.RecordTenantAuditEvent(r.Context(), event); err != nil {
		log.Printf("Failed to record tenant audit event for tenant %s action %s: %v", event.TenantID, event.Action, err)
		respondError(w, http.StatusInternalServerError, "Failed to record tenant audit event")
		return false
	}
	return true
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

	h.emitWebhookEvent(plugin.EventEmployeeCreated, tenantID, employee)
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

// ListSalaryComponents returns salary components for an employee
// @Summary List salary components
// @Description List salary components for an employee, optionally filtered to components active on a date
// @Tags Payroll
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param employeeID path string true "Employee ID"
// @Param active_on query string false "Filter components active on date (YYYY-MM-DD)"
// @Success 200 {array} payroll.SalaryComponent
// @Failure 400 {object} object{error=string}
// @Router /tenants/{tenantID}/employees/{employeeID}/salary-components [get]
func (h *Handlers) ListSalaryComponents(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	employeeID := chi.URLParam(r, "employeeID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	var activeOn *time.Time
	if rawActiveOn := strings.TrimSpace(r.URL.Query().Get("active_on")); rawActiveOn != "" {
		parsed, err := time.Parse("2006-01-02", rawActiveOn)
		if err != nil {
			respondError(w, http.StatusBadRequest, "Invalid active_on date")
			return
		}
		activeOn = &parsed
	}

	components, err := h.payrollService.ListSalaryComponents(r.Context(), schemaName, tenantID, employeeID, activeOn)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, components)
}

// AddSalaryComponent creates a salary component for an employee
// @Summary Add salary component
// @Description Add a recurring or one-off salary component, such as secondary employment income, bonus, commission, or taxable benefit
// @Tags Payroll
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param employeeID path string true "Employee ID"
// @Param request body payroll.CreateSalaryComponentRequest true "Salary component details"
// @Success 201 {object} payroll.SalaryComponent
// @Failure 400 {object} object{error=string}
// @Router /tenants/{tenantID}/employees/{employeeID}/salary-components [post]
func (h *Handlers) AddSalaryComponent(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	employeeID := chi.URLParam(r, "employeeID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	var req payroll.CreateSalaryComponentRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	component, err := h.payrollService.AddSalaryComponent(r.Context(), schemaName, tenantID, employeeID, &req)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, component)
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

// UpdatePayrollRunPaymentDate updates the intended salary payment date.
// @Summary Update payroll payment date
// @Description Set or correct the intended salary payment date for a payroll run until the run is declared
// @Tags Payroll
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param runID path string true "Payroll Run ID"
// @Param request body payroll.UpdatePayrollRunPaymentDateRequest true "Payment date update"
// @Success 200 {object} payroll.PayrollRun
// @Failure 400 {object} object{error=string}
// @Router /tenants/{tenantID}/payroll-runs/{runID}/payment-date [patch]
func (h *Handlers) UpdatePayrollRunPaymentDate(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	runID := chi.URLParam(r, "runID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	var req payroll.UpdatePayrollRunPaymentDateRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	run, err := h.payrollService.UpdatePayrollRunPaymentDate(r.Context(), schemaName, tenantID, runID, &req)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
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

	h.emitWebhookEvent(plugin.EventPayrollCalculated, tenantID, run)
	respondJSON(w, http.StatusOK, run)
}

// ProcessPayrollRun bulk-calculates a payroll run and optionally approves it.
// @Summary Process payroll run
// @Description Bulk-calculate payslips for all active employees in a payroll run and optionally approve it
// @Tags Payroll
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param runID path string true "Payroll Run ID"
// @Param request body payroll.ProcessPayrollRunRequest false "Bulk payroll processing options"
// @Success 200 {object} payroll.PayrollRunProcessResult
// @Failure 400 {object} object{error=string}
// @Router /tenants/{tenantID}/payroll-runs/{runID}/process [post]
func (h *Handlers) ProcessPayrollRun(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.GetClaims(r.Context())
	tenantID := chi.URLParam(r, "tenantID")
	runID := chi.URLParam(r, "runID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	var req payroll.ProcessPayrollRunRequest
	if r.Body != nil && r.ContentLength != 0 {
		if err := decodeJSON(r, &req); err != nil {
			respondError(w, http.StatusBadRequest, "Invalid request body")
			return
		}
	}

	result, err := h.payrollService.ProcessPayrollRun(r.Context(), schemaName, tenantID, runID, claims.UserID, &req)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, result)
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

	h.emitWebhookEvent(plugin.EventPayrollApproved, tenantID, map[string]string{"payroll_run_id": runID})
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

// GetPayslipPDF generates and returns a PDF for one payroll run payslip.
// @Summary Download payslip PDF
// @Description Generate and download a PDF for an employee payslip in a payroll run
// @Tags Payroll
// @Produce application/pdf
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param runID path string true "Payroll Run ID"
// @Param payslipID path string true "Payslip ID"
// @Success 200 {file} binary
// @Failure 404 {object} object{error=string}
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/payroll-runs/{runID}/payslips/{payslipID}/pdf [get]
func (h *Handlers) GetPayslipPDF(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	runID := chi.URLParam(r, "runID")
	payslipID := chi.URLParam(r, "payslipID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	run, err := h.payrollService.GetPayrollRun(r.Context(), schemaName, tenantID, runID)
	if err != nil {
		respondError(w, http.StatusNotFound, "Payroll run not found")
		return
	}

	payslips, err := h.payrollService.GetPayslipsWithEmployees(r.Context(), schemaName, tenantID, runID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get payslips")
		return
	}
	var selected *payroll.Payslip
	for i := range payslips {
		if payslips[i].ID == payslipID {
			selected = &payslips[i]
			break
		}
	}
	if selected == nil {
		respondError(w, http.StatusNotFound, "Payslip not found")
		return
	}

	tenantRecord, err := h.tenantService.GetTenant(r.Context(), tenantID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get tenant")
		return
	}

	pdfBytes, err := generatePayslipPDF(h.pdfService, selected, run, tenantRecord)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to generate PDF")
		return
	}

	filename := fmt.Sprintf("payslip-%04d-%02d-%s.pdf", run.PeriodYear, run.PeriodMonth, safeArchiveFileName(payslipID))
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", "attachment; filename=\""+filename+"\"")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(pdfBytes)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(pdfBytes)
}

// CalculateTaxPreview returns a tax preview for a salary
// @Summary Calculate tax preview
// @Description Preview Estonian tax calculations for a given gross salary
// @Tags Payroll
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param request body object{gross_salary=number,apply_basic_exemption=bool,basic_exemption_amount=number,funded_pension_rate=number} true "Calculation parameters"
// @Success 200 {object} payroll.TaxCalculation
// @Failure 400 {object} object{error=string}
// @Router /tenants/{tenantID}/payroll/tax-preview [post]
func (h *Handlers) CalculateTaxPreview(w http.ResponseWriter, r *http.Request) {
	var req struct {
		GrossSalary          decimal.Decimal  `json:"gross_salary"`
		ApplyBasicExemption  *bool            `json:"apply_basic_exemption"`
		BasicExemption       *decimal.Decimal `json:"basic_exemption"`
		BasicExemptionAmount *decimal.Decimal `json:"basic_exemption_amount"`
		FundedPensionRate    decimal.Decimal  `json:"funded_pension_rate"`
	}
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.GrossSalary.IsZero() || req.GrossSalary.IsNegative() {
		respondError(w, http.StatusBadRequest, "Gross salary must be positive")
		return
	}

	applyBasicExemption := req.ApplyBasicExemption != nil && *req.ApplyBasicExemption
	basicExemption := decimal.Zero
	if req.ApplyBasicExemption == nil || applyBasicExemption {
		switch {
		case req.BasicExemptionAmount != nil:
			basicExemption = *req.BasicExemptionAmount
		case req.BasicExemption != nil:
			basicExemption = *req.BasicExemption
		case applyBasicExemption:
			basicExemption = payroll.DefaultBasicExemption
		}
	}
	if basicExemption.IsNegative() {
		respondError(w, http.StatusBadRequest, "Basic exemption must be zero or greater")
		return
	}

	calc := payroll.CalculateEstonianTaxes(req.GrossSalary, basicExemption, req.FundedPensionRate)
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

// ListTSD returns TSD declarations for a tenant
// @Summary List TSD declarations
// @Description Get TSD declarations for a tenant, optionally filtered by period
// @Tags Payroll
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param year query int false "Filter by declaration year"
// @Param month query int false "Filter by declaration month"
// @Success 200 {array} payroll.TSDDeclaration
// @Failure 400 {object} object{error=string}
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/tsd [get]
func (h *Handlers) ListTSD(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	filter, err := parseTSDListFilter(r)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	declarations, err := h.payrollService.ListTSD(r.Context(), schemaName, tenantID, filter)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to list TSD declarations")
		return
	}

	respondJSON(w, http.StatusOK, declarations)
}

func parseTSDListFilter(r *http.Request) (payroll.TSDListFilter, error) {
	var filter payroll.TSDListFilter

	if rawYear := strings.TrimSpace(r.URL.Query().Get("year")); rawYear != "" {
		year, err := strconv.Atoi(rawYear)
		if err != nil || year <= 0 {
			return filter, errors.New("invalid year")
		}
		filter.Year = year
	}

	if rawMonth := strings.TrimSpace(r.URL.Query().Get("month")); rawMonth != "" {
		month, err := strconv.Atoi(rawMonth)
		if err != nil || month < 1 || month > 12 {
			return filter, errors.New("invalid month")
		}
		filter.Month = month
	}

	return filter, nil
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
// @Description Mark a TSD declaration as submitted to e-MTA after approved tax/support evidence is attached.
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
// @Failure 409 {object} object{error=string}
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

	if err := h.requireApprovedTSDSubmissionEvidence(r.Context(), schemaName, tenantID, tsd.ID); err != nil {
		if errors.Is(err, errApprovedTSDSubmissionEvidenceRequired) {
			respondError(w, http.StatusConflict, err.Error())
			return
		}
		respondError(w, http.StatusInternalServerError, "Failed to verify TSD submission evidence")
		return
	}

	if err := h.payrollService.MarkTSDSubmitted(r.Context(), schemaName, tenantID, tsd.ID, req.EMTAReference); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "submitted"})
}

func (h *Handlers) requireApprovedTSDSubmissionEvidence(ctx context.Context, schemaName, tenantID, declarationID string) error {
	return h.requireApprovedTSDEvidence(ctx, schemaName, tenantID, declarationID, "submission", "submitted", errApprovedTSDSubmissionEvidenceRequired)
}

func (h *Handlers) requireApprovedTSDAcceptanceEvidence(ctx context.Context, schemaName, tenantID, declarationID string) error {
	return h.requireApprovedTSDEvidence(ctx, schemaName, tenantID, declarationID, "acceptance", "accepted", errApprovedTSDAcceptanceEvidenceRequired)
}

func (h *Handlers) requireApprovedTSDEvidence(ctx context.Context, schemaName, tenantID, declarationID, evidenceStage, status string, requiredErr error) error {
	if h.documentsService == nil {
		return fmt.Errorf("%w before marking TSD declaration %s %s", requiredErr, declarationID, status)
	}

	results, err := h.documentsService.EvaluateEvidencePolicy(ctx, schemaName, tenantID, &documents.EvidencePolicyRequest{
		EntityType: documents.EntityTypeTSD,
		EntityIDs:  []string{declarationID},
		Rules: []documents.EvidencePolicyRule{{
			DocumentTypes: []string{
				documents.DocumentTypeTaxSupport,
				documents.DocumentTypeSupportingDocument,
			},
			MinCount:        1,
			RequireApproved: true,
		}},
	})
	if err != nil {
		return fmt.Errorf("evaluate TSD %s evidence: %w", evidenceStage, err)
	}
	if len(results) == 0 || !results[0].Compliant {
		return fmt.Errorf("%w before marking TSD declaration %s %s", requiredErr, declarationID, status)
	}
	return nil
}

// MarkTSDAccepted marks a TSD declaration as accepted
// @Summary Mark TSD as accepted
// @Description Mark a TSD declaration as accepted by e-MTA after approved tax/support evidence is attached.
// @Tags Payroll
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param year path int true "Year"
// @Param month path int true "Month"
// @Success 200 {object} object{status=string}
// @Failure 400 {object} object{error=string}
// @Failure 409 {object} object{error=string}
// @Router /tenants/{tenantID}/tsd/{year}/{month}/accept [post]
func (h *Handlers) MarkTSDAccepted(w http.ResponseWriter, r *http.Request) {
	h.markTSDStatusByPeriod(w, r, payroll.TSDAccepted, "accepted")
}

// MarkTSDRejected marks a TSD declaration as rejected
// @Summary Mark TSD as rejected
// @Description Mark a TSD declaration as rejected by e-MTA
// @Tags Payroll
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param year path int true "Year"
// @Param month path int true "Month"
// @Success 200 {object} object{status=string}
// @Failure 400 {object} object{error=string}
// @Router /tenants/{tenantID}/tsd/{year}/{month}/reject [post]
func (h *Handlers) MarkTSDRejected(w http.ResponseWriter, r *http.Request) {
	h.markTSDStatusByPeriod(w, r, payroll.TSDRejected, "rejected")
}

func (h *Handlers) markTSDStatusByPeriod(w http.ResponseWriter, r *http.Request, status payroll.TSDStatus, responseStatus string) {
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

	switch status {
	case payroll.TSDAccepted:
		if err := h.requireApprovedTSDAcceptanceEvidence(r.Context(), schemaName, tenantID, tsd.ID); err != nil {
			if errors.Is(err, errApprovedTSDAcceptanceEvidenceRequired) {
				respondError(w, http.StatusConflict, err.Error())
				return
			}
			respondError(w, http.StatusInternalServerError, "Failed to verify TSD acceptance evidence")
			return
		}
		err = h.payrollService.MarkTSDAccepted(r.Context(), schemaName, tenantID, tsd.ID)
	case payroll.TSDRejected:
		err = h.payrollService.MarkTSDRejected(r.Context(), schemaName, tenantID, tsd.ID)
	default:
		respondError(w, http.StatusBadRequest, "Unsupported TSD status")
		return
	}
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": responseStatus})
}

// =============================================================================
// QUOTES HANDLERS
// =============================================================================

// ListQuotes returns all quotes for a tenant
// @Summary List quotes
// @Description Get all quotes for a tenant with optional filtering
// @Tags Quotes
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param status query string false "Filter by status (DRAFT, SENT, ACCEPTED, REJECTED, EXPIRED, CONVERTED)"
// @Param contact_id query string false "Filter by contact ID"
// @Param from_date query string false "Filter from date (YYYY-MM-DD)"
// @Param to_date query string false "Filter to date (YYYY-MM-DD)"
// @Param search query string false "Search in quote number"
// @Success 200 {array} quotes.Quote
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/quotes [get]
func (h *Handlers) ListQuotes(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	filter := &quotes.QuoteFilter{
		Search: r.URL.Query().Get("search"),
	}

	if status := r.URL.Query().Get("status"); status != "" {
		filter.Status = quotes.QuoteStatus(status)
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

	quoteList, err := h.quotesService.List(r.Context(), tenantID, schemaName, filter)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to list quotes")
		return
	}

	respondJSON(w, http.StatusOK, quoteList)
}

// CreateQuote creates a new quote
// @Summary Create quote
// @Description Create a new sales quote
// @Tags Quotes
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param request body quotes.CreateQuoteRequest true "Quote details"
// @Success 201 {object} quotes.Quote
// @Failure 400 {object} object{error=string}
// @Router /tenants/{tenantID}/quotes [post]
func (h *Handlers) CreateQuote(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.GetClaims(r.Context())
	tenantID := chi.URLParam(r, "tenantID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	var req quotes.CreateQuoteRequest
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

	quote, err := h.quotesService.Create(r.Context(), tenantID, schemaName, &req)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, quote)
}

// ImportQuotes imports quotes from CSV data.
// @Summary Import quotes
// @Description Import historical quotes from grouped CSV data and skip duplicate or invalid rows
// @Tags Quotes
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param request body quotes.ImportQuotesRequest true "CSV import payload"
// @Success 200 {object} quotes.ImportQuotesResult
// @Failure 400 {object} object{error=string}
// @Router /tenants/{tenantID}/quotes/import [post]
func (h *Handlers) ImportQuotes(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.GetClaims(r.Context())
	tenantID := chi.URLParam(r, "tenantID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	var req quotes.ImportQuotesRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if strings.TrimSpace(req.CSVContent) == "" {
		respondError(w, http.StatusBadRequest, "csv_content is required")
		return
	}
	if req.FileName == "" {
		req.FileName = "quotes_import.csv"
	}
	req.UserID = claims.UserID

	contactsList, err := h.contactsService.List(r.Context(), tenantID, schemaName, nil)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to load contacts")
		return
	}

	productsList, err := h.importProductList(r.Context(), tenantID, schemaName)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to load products")
		return
	}

	result, err := h.quotesService.ImportCSV(r.Context(), tenantID, schemaName, contactsList, productsList, &req)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, result)
}

// GetQuote returns a quote by ID
// @Summary Get quote
// @Description Get quote details by ID
// @Tags Quotes
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param quoteID path string true "Quote ID"
// @Success 200 {object} quotes.Quote
// @Failure 404 {object} object{error=string}
// @Router /tenants/{tenantID}/quotes/{quoteID} [get]
func (h *Handlers) GetQuote(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	quoteID := chi.URLParam(r, "quoteID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	quote, err := h.quotesService.GetByID(r.Context(), tenantID, schemaName, quoteID)
	if err != nil {
		respondError(w, http.StatusNotFound, "Quote not found")
		return
	}

	respondJSON(w, http.StatusOK, quote)
}

// GetQuotePDF generates and returns a PDF for a quote
// @Summary Download quote PDF
// @Description Generate and download a PDF for a quote
// @Tags Quotes
// @Produce application/pdf
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param quoteID path string true "Quote ID"
// @Success 200 {file} binary
// @Failure 404 {object} object{error=string}
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/quotes/{quoteID}/pdf [get]
func (h *Handlers) GetQuotePDF(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	quoteID := chi.URLParam(r, "quoteID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	quote, err := h.quotesService.GetByID(r.Context(), tenantID, schemaName, quoteID)
	if err != nil {
		respondError(w, http.StatusNotFound, "Quote not found")
		return
	}

	t, err := h.tenantService.GetTenant(r.Context(), tenantID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get tenant")
		return
	}

	pdfSettings := h.pdfService.PDFSettingsFromTenant(t)
	pdfBytes, err := generateQuotePDF(h.pdfService, quote, t, pdfSettings)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to generate PDF")
		return
	}

	filename := "quote-" + quote.QuoteNumber + ".pdf"
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", "attachment; filename=\""+filename+"\"")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(pdfBytes)))

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(pdfBytes)
}

// UpdateQuote updates a quote
// @Summary Update quote
// @Description Update a draft quote
// @Tags Quotes
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param quoteID path string true "Quote ID"
// @Param request body quotes.UpdateQuoteRequest true "Quote details"
// @Success 200 {object} quotes.Quote
// @Failure 400 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Router /tenants/{tenantID}/quotes/{quoteID} [put]
func (h *Handlers) UpdateQuote(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	quoteID := chi.URLParam(r, "quoteID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	var req quotes.UpdateQuoteRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	quote, err := h.quotesService.Update(r.Context(), tenantID, schemaName, quoteID, &req)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, quote)
}

// DeleteQuote deletes a draft quote
// @Summary Delete quote
// @Description Delete a draft quote
// @Tags Quotes
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param quoteID path string true "Quote ID"
// @Success 204 "No Content"
// @Failure 400 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Router /tenants/{tenantID}/quotes/{quoteID} [delete]
func (h *Handlers) DeleteQuote(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	quoteID := chi.URLParam(r, "quoteID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	if err := h.quotesService.Delete(r.Context(), tenantID, schemaName, quoteID); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// SendQuote marks a quote as sent
// @Summary Send quote
// @Description Mark a quote as sent to the customer, optionally requiring approved quote evidence first
// @Tags Quotes
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param quoteID path string true "Quote ID"
// @Param request body object{require_approved_evidence=bool} false "Evidence requirement options"
// @Success 200 {object} object{status=string}
// @Failure 400 {object} object{error=string}
// @Failure 409 {object} object{error=string,evidence_policy_results=[]documents.EvidencePolicyResult,remediation_actions=[]documents.DocumentRemediationAction}
// @Router /tenants/{tenantID}/quotes/{quoteID}/send [post]
func (h *Handlers) SendQuote(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	quoteID := chi.URLParam(r, "quoteID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	var req struct {
		RequireApprovedEvidence bool `json:"require_approved_evidence"`
	}
	if r.Body != nil && r.ContentLength != 0 {
		if err := decodeJSON(r, &req); err != nil {
			respondError(w, http.StatusBadRequest, "Invalid request body")
			return
		}
	}

	if err := h.requireApprovedCommercialEvidence(r.Context(), schemaName, tenantID, documents.EntityTypeQuote, quoteID, req.RequireApprovedEvidence, errApprovedQuoteEvidenceRequired, "sending quote"); err != nil {
		var conflict *evidencePolicyConflictError
		if errors.As(err, &conflict) {
			respondEvidencePolicyConflict(w, conflict.Error(), conflict.Results)
			return
		}
		status := http.StatusInternalServerError
		if errors.Is(err, errApprovedQuoteEvidenceRequired) {
			status = http.StatusConflict
		}
		respondError(w, status, err.Error())
		return
	}

	if err := h.quotesService.Send(r.Context(), tenantID, schemaName, quoteID); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "sent"})
}

func (h *Handlers) requireApprovedCommercialEvidence(ctx context.Context, schemaName, tenantID, entityType, entityID string, requireApproved bool, requiredErr error, action string) error {
	if !requireApproved {
		return nil
	}
	if h.documentsService == nil {
		return fmt.Errorf("%w before %s %s", requiredErr, action, entityID)
	}

	results, err := evaluateDocumentsEvidencePolicy(h.documentsService, ctx, schemaName, tenantID, &documents.EvidencePolicyRequest{
		EntityType: entityType,
		EntityIDs:  []string{entityID},
		Rules: []documents.EvidencePolicyRule{{
			DocumentTypes: []string{
				documents.DocumentTypeContract,
				documents.DocumentTypeSupportingDocument,
			},
			MinCount:        1,
			RequireApproved: true,
		}},
	})
	if err != nil {
		return fmt.Errorf("evaluate %s evidence: %w", entityType, err)
	}
	if len(results) == 0 {
		return fmt.Errorf("%w before %s %s", requiredErr, action, entityID)
	}
	for _, result := range results {
		if !result.Compliant {
			return &evidencePolicyConflictError{
				Err:     fmt.Errorf("%w before %s %s", requiredErr, action, entityID),
				Results: results,
			}
		}
	}

	return nil
}

// AcceptQuote marks a quote as accepted
// @Summary Accept quote
// @Description Mark a quote as accepted by the customer
// @Tags Quotes
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param quoteID path string true "Quote ID"
// @Success 200 {object} object{status=string}
// @Failure 400 {object} object{error=string}
// @Router /tenants/{tenantID}/quotes/{quoteID}/accept [post]
func (h *Handlers) AcceptQuote(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	quoteID := chi.URLParam(r, "quoteID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	if err := h.quotesService.Accept(r.Context(), tenantID, schemaName, quoteID); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "accepted"})
}

// RejectQuote marks a quote as rejected
// @Summary Reject quote
// @Description Mark a quote as rejected by the customer
// @Tags Quotes
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param quoteID path string true "Quote ID"
// @Success 200 {object} object{status=string}
// @Failure 400 {object} object{error=string}
// @Router /tenants/{tenantID}/quotes/{quoteID}/reject [post]
func (h *Handlers) RejectQuote(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	quoteID := chi.URLParam(r, "quoteID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	if err := h.quotesService.Reject(r.Context(), tenantID, schemaName, quoteID); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "rejected"})
}

// ConvertQuoteToInvoice creates a draft sales invoice from an accepted quote.
// @Summary Convert quote to invoice
// @Description Create a draft sales invoice from an accepted quote and mark the quote converted
// @Tags Quotes
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param quoteID path string true "Quote ID"
// @Param request body quotes.ConvertQuoteToInvoiceRequest true "Invoice conversion options"
// @Success 201 {object} quotes.QuoteInvoiceConversionResult
// @Failure 400 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Failure 409 {object} object{error=string}
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/quotes/{quoteID}/convert-to-invoice [post]
func (h *Handlers) ConvertQuoteToInvoice(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.GetClaims(r.Context())
	tenantID := chi.URLParam(r, "tenantID")
	quoteID := chi.URLParam(r, "quoteID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	var req quotes.ConvertQuoteToInvoiceRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	req.UserID = claims.UserID
	if req.IssueDate.IsZero() {
		req.IssueDate = time.Now()
	}
	if req.DueDate.IsZero() {
		req.DueDate = req.IssueDate.AddDate(0, 0, 14)
	}
	if req.DueDate.Before(req.IssueDate) {
		respondError(w, http.StatusBadRequest, "due date cannot be before issue date")
		return
	}
	if h.rejectLockedPeriod(w, r.Context(), tenantID, req.IssueDate) {
		return
	}

	quote, err := h.quotesService.GetByID(r.Context(), tenantID, schemaName, quoteID)
	if err != nil {
		respondError(w, http.StatusNotFound, "Quote not found")
		return
	}
	if quote.Status != quotes.QuoteStatusAccepted {
		respondError(w, http.StatusConflict, "quote must be accepted before conversion")
		return
	}
	if quote.ConvertedToInvoiceID != nil {
		respondError(w, http.StatusConflict, "quote has already been converted to an invoice")
		return
	}

	notes := strings.TrimSpace(req.Notes)
	if notes == "" {
		notes = quote.Notes
	}
	invoiceReq := invoicing.CreateInvoiceRequest{
		InvoiceType:  invoicing.InvoiceTypeSales,
		ContactID:    quote.ContactID,
		IssueDate:    req.IssueDate,
		DueDate:      req.DueDate,
		Currency:     quote.Currency,
		ExchangeRate: quote.ExchangeRate,
		Reference:    quote.QuoteNumber,
		Notes:        notes,
		UserID:       req.UserID,
	}
	for _, line := range quote.Lines {
		invoiceReq.Lines = append(invoiceReq.Lines, invoicing.CreateInvoiceLineRequest{
			Description:     line.Description,
			Quantity:        line.Quantity,
			Unit:            line.Unit,
			UnitPrice:       line.UnitPrice,
			DiscountPercent: line.DiscountPercent,
			VATRate:         line.VATRate,
			ProductID:       line.ProductID,
		})
	}

	invoice, err := h.invoicingService.Create(r.Context(), tenantID, schemaName, &invoiceReq)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.quotesService.ConvertToInvoice(r.Context(), tenantID, schemaName, quoteID, invoice.ID); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to mark quote converted")
		return
	}
	quote.Status = quotes.QuoteStatusConverted
	quote.ConvertedToInvoiceID = &invoice.ID

	respondJSON(w, http.StatusCreated, &quotes.QuoteInvoiceConversionResult{
		Quote:   quote,
		Invoice: invoice,
	})
}

// =============================================================================
// ORDERS HANDLERS
// =============================================================================

// ListOrders returns all orders for a tenant
// @Summary List orders
// @Description Get all orders for a tenant with optional filtering
// @Tags Orders
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param status query string false "Filter by status (PENDING, CONFIRMED, PROCESSING, SHIPPED, DELIVERED, CANCELED)"
// @Param contact_id query string false "Filter by contact ID"
// @Param from_date query string false "Filter from date (YYYY-MM-DD)"
// @Param to_date query string false "Filter to date (YYYY-MM-DD)"
// @Param search query string false "Search in order number"
// @Success 200 {array} orders.Order
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/orders [get]
func (h *Handlers) ListOrders(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	filter := &orders.OrderFilter{
		Search: r.URL.Query().Get("search"),
	}

	if status := r.URL.Query().Get("status"); status != "" {
		filter.Status = orders.OrderStatus(status)
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

	orderList, err := h.ordersService.List(r.Context(), tenantID, schemaName, filter)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to list orders")
		return
	}

	respondJSON(w, http.StatusOK, orderList)
}

// CreateOrder creates a new order
// @Summary Create order
// @Description Create a new sales order
// @Tags Orders
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param request body orders.CreateOrderRequest true "Order details"
// @Success 201 {object} orders.Order
// @Failure 400 {object} object{error=string}
// @Router /tenants/{tenantID}/orders [post]
func (h *Handlers) CreateOrder(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.GetClaims(r.Context())
	tenantID := chi.URLParam(r, "tenantID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	var req orders.CreateOrderRequest
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

	order, err := h.ordersService.Create(r.Context(), tenantID, schemaName, &req)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, order)
}

// ImportOrders imports orders from CSV data.
// @Summary Import orders
// @Description Import historical orders from grouped CSV data and skip duplicate or invalid rows
// @Tags Orders
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param request body orders.ImportOrdersRequest true "CSV import payload"
// @Success 200 {object} orders.ImportOrdersResult
// @Failure 400 {object} object{error=string}
// @Router /tenants/{tenantID}/orders/import [post]
func (h *Handlers) ImportOrders(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.GetClaims(r.Context())
	tenantID := chi.URLParam(r, "tenantID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	var req orders.ImportOrdersRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if strings.TrimSpace(req.CSVContent) == "" {
		respondError(w, http.StatusBadRequest, "csv_content is required")
		return
	}
	if req.FileName == "" {
		req.FileName = "orders_import.csv"
	}
	req.UserID = claims.UserID

	contactsList, err := h.contactsService.List(r.Context(), tenantID, schemaName, nil)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to load contacts")
		return
	}

	productsList, err := h.importProductList(r.Context(), tenantID, schemaName)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to load products")
		return
	}

	quoteReferences, err := h.importOrderQuoteReferences(r.Context(), tenantID, schemaName)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to load quotes")
		return
	}

	result, err := h.ordersService.ImportCSVWithQuoteReferences(r.Context(), tenantID, schemaName, contactsList, productsList, quoteReferences, &req)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, result)
}

// GetOrder returns an order by ID
// @Summary Get order
// @Description Get order details by ID
// @Tags Orders
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param orderID path string true "Order ID"
// @Success 200 {object} orders.Order
// @Failure 404 {object} object{error=string}
// @Router /tenants/{tenantID}/orders/{orderID} [get]
func (h *Handlers) GetOrder(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	orderID := chi.URLParam(r, "orderID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	order, err := h.ordersService.GetByID(r.Context(), tenantID, schemaName, orderID)
	if err != nil {
		respondError(w, http.StatusNotFound, "Order not found")
		return
	}

	respondJSON(w, http.StatusOK, order)
}

// GetOrderPDF generates and returns a PDF for an order
// @Summary Download order PDF
// @Description Generate and download a PDF for an order
// @Tags Orders
// @Produce application/pdf
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param orderID path string true "Order ID"
// @Success 200 {file} binary
// @Failure 404 {object} object{error=string}
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/orders/{orderID}/pdf [get]
func (h *Handlers) GetOrderPDF(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	orderID := chi.URLParam(r, "orderID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	order, err := h.ordersService.GetByID(r.Context(), tenantID, schemaName, orderID)
	if err != nil {
		respondError(w, http.StatusNotFound, "Order not found")
		return
	}

	t, err := h.tenantService.GetTenant(r.Context(), tenantID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get tenant")
		return
	}

	pdfSettings := h.pdfService.PDFSettingsFromTenant(t)
	pdfBytes, err := generateOrderPDF(h.pdfService, order, t, pdfSettings)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to generate PDF")
		return
	}

	filename := "order-" + order.OrderNumber + ".pdf"
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", "attachment; filename=\""+filename+"\"")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(pdfBytes)))

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(pdfBytes)
}

// CheckOrderStock reports whether tracked order lines have enough available stock.
// @Summary Check order stock availability
// @Description Check whether an order can be fulfilled from all warehouses or one warehouse without mutating stock
// @Tags Orders
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param orderID path string true "Order ID"
// @Param warehouse_id query string false "Warehouse ID to check; omit to sum all warehouses"
// @Success 200 {object} orders.OrderStockCheck
// @Failure 400 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Router /tenants/{tenantID}/orders/{orderID}/stock-check [get]
func (h *Handlers) CheckOrderStock(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	orderID := chi.URLParam(r, "orderID")
	schemaName := h.getSchemaName(r.Context(), tenantID)
	warehouseID := strings.TrimSpace(r.URL.Query().Get("warehouse_id"))

	if h.inventoryService == nil {
		respondError(w, http.StatusInternalServerError, "Inventory service unavailable")
		return
	}
	if warehouseID != "" {
		if _, err := h.inventoryService.GetWarehouseByID(r.Context(), tenantID, schemaName, warehouseID); err != nil {
			respondError(w, http.StatusBadRequest, "Warehouse not found")
			return
		}
	}

	check, err := h.buildOrderStockCheck(r.Context(), tenantID, schemaName, orderID, warehouseID)
	if err != nil {
		if errors.Is(err, orders.ErrOrderNotFound) {
			respondError(w, http.StatusNotFound, "Order not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "Failed to check order stock")
		return
	}

	respondJSON(w, http.StatusOK, check)
}

func (h *Handlers) buildOrderStockCheck(ctx context.Context, tenantID, schemaName, orderID, warehouseID string) (*orders.OrderStockCheck, error) {
	order, err := h.ordersService.GetByID(ctx, tenantID, schemaName, orderID)
	if err != nil {
		return nil, err
	}

	check := &orders.OrderStockCheck{
		OrderID:     order.ID,
		OrderNumber: order.OrderNumber,
		WarehouseID: warehouseID,
		Ready:       true,
		Lines:       make([]orders.OrderStockCheckLine, 0, len(order.Lines)),
	}
	remainingAvailableByProduct := map[string]decimal.Decimal{}
	loadedAvailabilityByProduct := map[string]bool{}

	for _, line := range order.Lines {
		checkLine := orders.OrderStockCheckLine{
			LineID:      line.ID,
			LineNumber:  line.LineNumber,
			Description: line.Description,
			RequiredQty: line.Quantity,
			Status:      orders.OrderStockLineStatusNotTracked,
		}
		if line.ProductID == nil || strings.TrimSpace(*line.ProductID) == "" {
			check.Lines = append(check.Lines, checkLine)
			continue
		}

		productID := strings.TrimSpace(*line.ProductID)
		checkLine.ProductID = productID
		product, err := h.inventoryService.GetProductByID(ctx, tenantID, schemaName, productID)
		if err != nil {
			checkLine.Status = orders.OrderStockLineStatusProductNotFound
			checkLine.ShortageQty = line.Quantity
			check.Ready = false
			check.Lines = append(check.Lines, checkLine)
			continue
		}
		checkLine.ProductCode = product.Code
		checkLine.ProductName = product.Name
		if product.ProductType != inventory.ProductTypeGoods || !product.TrackInventory {
			check.Lines = append(check.Lines, checkLine)
			continue
		}

		if !loadedAvailabilityByProduct[product.ID] {
			levels, err := h.inventoryService.GetStockLevels(ctx, tenantID, schemaName, product.ID)
			if err != nil {
				return nil, err
			}
			for _, level := range levels {
				if warehouseID != "" && level.WarehouseID != warehouseID {
					continue
				}
				remainingAvailableByProduct[product.ID] = remainingAvailableByProduct[product.ID].Add(level.AvailableQty)
			}
			loadedAvailabilityByProduct[product.ID] = true
		}

		checkLine.AvailableQty = remainingAvailableByProduct[product.ID]
		if checkLine.AvailableQty.LessThan(line.Quantity) {
			checkLine.Status = orders.OrderStockLineStatusShortage
			checkLine.ShortageQty = line.Quantity.Sub(checkLine.AvailableQty)
			remainingAvailableByProduct[product.ID] = decimal.Zero
			check.Ready = false
		} else {
			checkLine.Status = orders.OrderStockLineStatusAvailable
			remainingAvailableByProduct[product.ID] = checkLine.AvailableQty.Sub(line.Quantity)
		}
		check.Lines = append(check.Lines, checkLine)
	}

	return check, nil
}

// ListOrderStockReservations lists persisted stock reservations for an order.
// @Summary List order stock reservations
// @Description List current product and warehouse stock reservations recorded for an order
// @Tags Orders
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param orderID path string true "Order ID"
// @Success 200 {array} orders.OrderStockReservation
// @Failure 404 {object} object{error=string}
// @Router /tenants/{tenantID}/orders/{orderID}/stock-reservations [get]
func (h *Handlers) ListOrderStockReservations(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	orderID := chi.URLParam(r, "orderID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	if _, err := h.ordersService.GetByID(r.Context(), tenantID, schemaName, orderID); err != nil {
		if errors.Is(err, orders.ErrOrderNotFound) {
			respondError(w, http.StatusNotFound, "Order not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "Failed to get order")
		return
	}

	reservations, err := h.ordersService.ListStockReservations(r.Context(), tenantID, schemaName, orderID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to list order stock reservations")
		return
	}
	respondJSON(w, http.StatusOK, reservations)
}

// GetOrderPickList returns warehouse picking readiness for an order.
// @Summary Get order pick list
// @Description Build a warehouse pick list from persisted order stock reservations
// @Tags Orders
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param orderID path string true "Order ID"
// @Param warehouse_id query string true "Warehouse ID"
// @Success 200 {object} orders.OrderPickList
// @Failure 400 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Router /tenants/{tenantID}/orders/{orderID}/pick-list [get]
func (h *Handlers) GetOrderPickList(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	orderID := chi.URLParam(r, "orderID")
	schemaName := h.getSchemaName(r.Context(), tenantID)
	warehouseID := strings.TrimSpace(r.URL.Query().Get("warehouse_id"))

	if h.inventoryService == nil {
		respondError(w, http.StatusInternalServerError, "Inventory service unavailable")
		return
	}
	if warehouseID == "" {
		respondError(w, http.StatusBadRequest, "warehouse_id is required")
		return
	}
	if _, err := h.inventoryService.GetWarehouseByID(r.Context(), tenantID, schemaName, warehouseID); err != nil {
		respondError(w, http.StatusBadRequest, "Warehouse not found")
		return
	}

	pickList, err := h.buildOrderPickList(r.Context(), tenantID, schemaName, orderID, warehouseID)
	if err != nil {
		if errors.Is(err, orders.ErrOrderNotFound) {
			respondError(w, http.StatusNotFound, "Order not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "Failed to build order pick list")
		return
	}

	respondJSON(w, http.StatusOK, pickList)
}

func (h *Handlers) buildOrderPickList(ctx context.Context, tenantID, schemaName, orderID, warehouseID string) (*orders.OrderPickList, error) {
	order, err := h.ordersService.GetByID(ctx, tenantID, schemaName, orderID)
	if err != nil {
		return nil, err
	}
	reservations, err := h.ordersService.ListStockReservations(ctx, tenantID, schemaName, orderID)
	if err != nil {
		return nil, err
	}

	remainingReservedByProduct := map[string]decimal.Decimal{}
	for _, reservation := range reservations {
		if reservation.WarehouseID != warehouseID || reservation.Status != orders.OrderStockReservationStatusReserved {
			continue
		}
		remainingReservedByProduct[reservation.ProductID] = remainingReservedByProduct[reservation.ProductID].Add(reservation.Quantity)
	}

	pickList := &orders.OrderPickList{
		OrderID:     order.ID,
		OrderNumber: order.OrderNumber,
		WarehouseID: warehouseID,
		Ready:       true,
		Lines:       make([]orders.OrderPickListLine, 0, len(order.Lines)),
	}
	availableByProduct := map[string]decimal.Decimal{}
	loadedAvailabilityByProduct := map[string]bool{}

	for _, line := range order.Lines {
		pickLine := orders.OrderPickListLine{
			LineID:      line.ID,
			LineNumber:  line.LineNumber,
			Description: line.Description,
			RequiredQty: line.Quantity,
			Status:      orders.OrderPickListLineStatusNotTracked,
		}
		if line.ProductID == nil || strings.TrimSpace(*line.ProductID) == "" {
			pickList.Lines = append(pickList.Lines, pickLine)
			continue
		}

		productID := strings.TrimSpace(*line.ProductID)
		pickLine.ProductID = productID
		product, err := h.inventoryService.GetProductByID(ctx, tenantID, schemaName, productID)
		if err != nil {
			pickLine.Status = orders.OrderPickListLineStatusProductNotFound
			pickLine.ShortageQty = line.Quantity
			pickList.Ready = false
			pickList.Lines = append(pickList.Lines, pickLine)
			continue
		}
		pickLine.ProductCode = product.Code
		pickLine.ProductName = product.Name
		if product.ProductType != inventory.ProductTypeGoods || !product.TrackInventory {
			pickList.Lines = append(pickList.Lines, pickLine)
			continue
		}

		if !loadedAvailabilityByProduct[product.ID] {
			levels, err := h.inventoryService.GetStockLevels(ctx, tenantID, schemaName, product.ID)
			if err != nil {
				return nil, err
			}
			for _, level := range levels {
				if level.WarehouseID == warehouseID {
					availableByProduct[product.ID] = availableByProduct[product.ID].Add(level.AvailableQty)
				}
			}
			loadedAvailabilityByProduct[product.ID] = true
		}

		pickLine.AvailableQty = availableByProduct[product.ID]
		pickLine.ReservedQty = remainingReservedByProduct[product.ID]
		if pickLine.ReservedQty.GreaterThanOrEqual(line.Quantity) {
			pickLine.Status = orders.OrderPickListLineStatusReady
			pickLine.PickQty = line.Quantity
			remainingReservedByProduct[product.ID] = pickLine.ReservedQty.Sub(line.Quantity)
		} else {
			pickLine.PickQty = pickLine.ReservedQty
			pickLine.ShortageQty = line.Quantity.Sub(pickLine.ReservedQty)
			remainingReservedByProduct[product.ID] = decimal.Zero
			pickList.Ready = false
			if pickLine.ReservedQty.IsZero() {
				pickLine.Status = orders.OrderPickListLineStatusUnreserved
			} else {
				pickLine.Status = orders.OrderPickListLineStatusShortage
			}
		}
		pickList.Lines = append(pickList.Lines, pickLine)
	}

	return pickList, nil
}

// ReserveOrderStock reserves the tracked product quantities required by an order.
// @Summary Reserve order stock
// @Description Reserve tracked goods for an order from one warehouse without shipping stock
// @Tags Orders
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param orderID path string true "Order ID"
// @Param request body orders.OrderStockReservationRequest true "Warehouse reservation request"
// @Success 200 {object} orders.OrderStockReservationResult
// @Failure 400 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Router /tenants/{tenantID}/orders/{orderID}/reserve-stock [post]
func (h *Handlers) ReserveOrderStock(w http.ResponseWriter, r *http.Request) {
	h.handleOrderStockReservation(w, r, orders.OrderStockReservationActionReserve)
}

// ReleaseOrderStock releases the tracked product quantities required by an order.
// @Summary Release order stock
// @Description Release tracked goods for an order from one warehouse back to available stock
// @Tags Orders
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param orderID path string true "Order ID"
// @Param request body orders.OrderStockReservationRequest true "Warehouse release request"
// @Success 200 {object} orders.OrderStockReservationResult
// @Failure 400 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Router /tenants/{tenantID}/orders/{orderID}/release-stock [post]
func (h *Handlers) ReleaseOrderStock(w http.ResponseWriter, r *http.Request) {
	h.handleOrderStockReservation(w, r, orders.OrderStockReservationActionRelease)
}

func (h *Handlers) handleOrderStockReservation(w http.ResponseWriter, r *http.Request, action string) {
	tenantID := chi.URLParam(r, "tenantID")
	orderID := chi.URLParam(r, "orderID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	if h.inventoryService == nil {
		respondError(w, http.StatusInternalServerError, "Inventory service unavailable")
		return
	}

	claims, ok := auth.GetClaims(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Invalid or missing authentication")
		return
	}

	var req orders.OrderStockReservationRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	req.WarehouseID = strings.TrimSpace(req.WarehouseID)
	req.Reason = strings.TrimSpace(req.Reason)
	if req.WarehouseID == "" {
		respondError(w, http.StatusBadRequest, "warehouse_id is required")
		return
	}
	if _, err := h.inventoryService.GetWarehouseByID(r.Context(), tenantID, schemaName, req.WarehouseID); err != nil {
		respondError(w, http.StatusBadRequest, "Warehouse not found")
		return
	}

	check, err := h.buildOrderStockCheck(r.Context(), tenantID, schemaName, orderID, req.WarehouseID)
	if err != nil {
		if errors.Is(err, orders.ErrOrderNotFound) {
			respondError(w, http.StatusNotFound, "Order not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "Failed to check order stock")
		return
	}
	if action == orders.OrderStockReservationActionReserve && !check.Ready {
		respondJSON(w, http.StatusBadRequest, map[string]interface{}{
			"error":       "Order stock is not ready for reservation",
			"stock_check": check,
		})
		return
	}
	if action == orders.OrderStockReservationActionRelease && orderStockCheckHasStatus(check, orders.OrderStockLineStatusProductNotFound) {
		respondJSON(w, http.StatusBadRequest, map[string]interface{}{
			"error":       "Order has missing product references",
			"stock_check": check,
		})
		return
	}

	result, err := h.applyOrderStockReservation(r.Context(), tenantID, schemaName, action, req.Reason, claims.UserID, check)
	if err != nil {
		respondError(w, http.StatusBadRequest, fmt.Sprintf("Failed to %s order stock: %v", strings.ToLower(action), err))
		return
	}

	respondJSON(w, http.StatusOK, result)
}

func orderStockCheckHasStatus(check *orders.OrderStockCheck, status string) bool {
	if check == nil {
		return false
	}
	for _, line := range check.Lines {
		if line.Status == status {
			return true
		}
	}
	return false
}

func (h *Handlers) applyOrderStockReservation(
	ctx context.Context,
	tenantID string,
	schemaName string,
	action string,
	reason string,
	userID string,
	check *orders.OrderStockCheck,
) (*orders.OrderStockReservationResult, error) {
	result := &orders.OrderStockReservationResult{
		OrderID:     check.OrderID,
		OrderNumber: check.OrderNumber,
		WarehouseID: check.WarehouseID,
		Action:      action,
	}

	aggregates := map[string]*orders.OrderStockReservationLine{}
	productOrder := []string{}
	for _, line := range check.Lines {
		if line.ProductID == "" {
			continue
		}
		if line.Status != orders.OrderStockLineStatusAvailable && line.Status != orders.OrderStockLineStatusShortage {
			continue
		}
		aggregate, ok := aggregates[line.ProductID]
		if !ok {
			aggregate = &orders.OrderStockReservationLine{
				ProductID:   line.ProductID,
				ProductCode: line.ProductCode,
				ProductName: line.ProductName,
			}
			aggregates[line.ProductID] = aggregate
			productOrder = append(productOrder, line.ProductID)
		}
		aggregate.Quantity = aggregate.Quantity.Add(line.RequiredQty)
	}

	status := orders.OrderStockReservationStatusReserved
	if action == orders.OrderStockReservationActionRelease {
		status = orders.OrderStockReservationStatusReleased
	}
	if reason == "" {
		reason = fmt.Sprintf("Order %s stock %s", check.OrderNumber, strings.ToLower(action))
	}

	for _, productID := range productOrder {
		line := aggregates[productID]
		if action == orders.OrderStockReservationActionRelease {
			reservation, err := h.ordersService.GetStockReservation(ctx, tenantID, schemaName, check.OrderID, productID, check.WarehouseID)
			if err != nil {
				return nil, err
			}
			if reservation.Quantity.LessThan(line.Quantity) {
				return nil, fmt.Errorf("cannot release more than order reserved stock for product %s", productID)
			}
		}

		req := &inventory.StockReservationRequest{
			ProductID:   productID,
			WarehouseID: check.WarehouseID,
			Quantity:    line.Quantity.String(),
			Reason:      reason,
			UserID:      userID,
		}
		var (
			level *inventory.StockLevel
			err   error
		)
		if action == orders.OrderStockReservationActionRelease {
			level, err = h.inventoryService.ReleaseStock(ctx, tenantID, schemaName, req)
		} else {
			level, err = h.inventoryService.ReserveStock(ctx, tenantID, schemaName, req)
		}
		if err != nil {
			return nil, err
		}
		if action == orders.OrderStockReservationActionRelease {
			if _, err := h.ordersService.ReleaseStockReservation(ctx, tenantID, schemaName, check.OrderID, productID, check.WarehouseID, line.Quantity, reason, userID); err != nil {
				return nil, err
			}
		} else {
			if err := h.ordersService.UpsertStockReservation(ctx, tenantID, schemaName, &orders.OrderStockReservation{
				TenantID:    tenantID,
				OrderID:     check.OrderID,
				ProductID:   productID,
				WarehouseID: check.WarehouseID,
				Quantity:    line.Quantity,
				Status:      orders.OrderStockReservationStatusReserved,
				Reason:      reason,
				CreatedBy:   userID,
			}); err != nil {
				_, rollbackErr := h.inventoryService.ReleaseStock(ctx, tenantID, schemaName, req)
				if rollbackErr != nil {
					return nil, fmt.Errorf("%w; rollback release failed: %v", err, rollbackErr)
				}
				return nil, err
			}
		}
		line.ReservedQty = level.ReservedQty
		line.AvailableQty = level.AvailableQty
		line.Status = status
		result.Lines = append(result.Lines, *line)
	}

	return result, nil
}

// UpdateOrder updates an order
// @Summary Update order
// @Description Update a pending or confirmed order
// @Tags Orders
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param orderID path string true "Order ID"
// @Param request body orders.UpdateOrderRequest true "Order details"
// @Success 200 {object} orders.Order
// @Failure 400 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Router /tenants/{tenantID}/orders/{orderID} [put]
func (h *Handlers) UpdateOrder(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	orderID := chi.URLParam(r, "orderID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	var req orders.UpdateOrderRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	order, err := h.ordersService.Update(r.Context(), tenantID, schemaName, orderID, &req)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, order)
}

// DeleteOrder deletes a pending order
// @Summary Delete order
// @Description Delete a pending order
// @Tags Orders
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param orderID path string true "Order ID"
// @Success 204 "No Content"
// @Failure 400 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Router /tenants/{tenantID}/orders/{orderID} [delete]
func (h *Handlers) DeleteOrder(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	orderID := chi.URLParam(r, "orderID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	if err := h.ordersService.Delete(r.Context(), tenantID, schemaName, orderID); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ConfirmOrder marks an order as confirmed
// @Summary Confirm order
// @Description Mark a pending order as confirmed, optionally requiring approved order evidence first
// @Tags Orders
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param orderID path string true "Order ID"
// @Param request body object{require_approved_evidence=bool} false "Evidence requirement options"
// @Success 200 {object} object{status=string}
// @Failure 400 {object} object{error=string}
// @Failure 409 {object} object{error=string,evidence_policy_results=[]documents.EvidencePolicyResult,remediation_actions=[]documents.DocumentRemediationAction}
// @Router /tenants/{tenantID}/orders/{orderID}/confirm [post]
func (h *Handlers) ConfirmOrder(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	orderID := chi.URLParam(r, "orderID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	var req struct {
		RequireApprovedEvidence bool `json:"require_approved_evidence"`
	}
	if r.Body != nil && r.ContentLength != 0 {
		if err := decodeJSON(r, &req); err != nil {
			respondError(w, http.StatusBadRequest, "Invalid request body")
			return
		}
	}

	if err := h.requireApprovedCommercialEvidence(r.Context(), schemaName, tenantID, documents.EntityTypeOrder, orderID, req.RequireApprovedEvidence, errApprovedOrderEvidenceRequired, "confirming order"); err != nil {
		var conflict *evidencePolicyConflictError
		if errors.As(err, &conflict) {
			respondEvidencePolicyConflict(w, conflict.Error(), conflict.Results)
			return
		}
		status := http.StatusInternalServerError
		if errors.Is(err, errApprovedOrderEvidenceRequired) {
			status = http.StatusConflict
		}
		respondError(w, status, err.Error())
		return
	}

	if err := h.ordersService.Confirm(r.Context(), tenantID, schemaName, orderID); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "confirmed"})
}

// ProcessOrder marks an order as processing
// @Summary Process order
// @Description Mark a confirmed order as processing
// @Tags Orders
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param orderID path string true "Order ID"
// @Success 200 {object} object{status=string}
// @Failure 400 {object} object{error=string}
// @Router /tenants/{tenantID}/orders/{orderID}/process [post]
func (h *Handlers) ProcessOrder(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	orderID := chi.URLParam(r, "orderID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	if err := h.ordersService.Process(r.Context(), tenantID, schemaName, orderID); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "processing"})
}

// ShipOrder marks an order as shipped
// @Summary Ship order
// @Description Mark an order as shipped
// @Tags Orders
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param orderID path string true "Order ID"
// @Success 200 {object} object{status=string}
// @Failure 400 {object} object{error=string}
// @Router /tenants/{tenantID}/orders/{orderID}/ship [post]
func (h *Handlers) ShipOrder(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	orderID := chi.URLParam(r, "orderID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	if err := h.ordersService.Ship(r.Context(), tenantID, schemaName, orderID); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "shipped"})
}

// DeliverOrder marks an order as delivered
// @Summary Deliver order
// @Description Mark a shipped order as delivered
// @Tags Orders
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param orderID path string true "Order ID"
// @Success 200 {object} object{status=string}
// @Failure 400 {object} object{error=string}
// @Router /tenants/{tenantID}/orders/{orderID}/deliver [post]
func (h *Handlers) DeliverOrder(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	orderID := chi.URLParam(r, "orderID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	if err := h.ordersService.Deliver(r.Context(), tenantID, schemaName, orderID); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "delivered"})
}

// CancelOrder cancels an order
// @Summary Cancel order
// @Description Cancel an order (not allowed if already delivered)
// @Tags Orders
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param orderID path string true "Order ID"
// @Success 200 {object} object{status=string}
// @Failure 400 {object} object{error=string}
// @Router /tenants/{tenantID}/orders/{orderID}/cancel [post]
func (h *Handlers) CancelOrder(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	orderID := chi.URLParam(r, "orderID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	if err := h.ordersService.Cancel(r.Context(), tenantID, schemaName, orderID); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "canceled"})
}

// ConvertOrderToInvoice creates a draft sales invoice from a delivered order.
// @Summary Convert order to invoice
// @Description Create a draft sales invoice from a delivered order and mark the order converted
// @Tags Orders
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param orderID path string true "Order ID"
// @Param request body orders.ConvertOrderToInvoiceRequest true "Invoice conversion options"
// @Success 201 {object} orders.OrderInvoiceConversionResult
// @Failure 400 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Failure 409 {object} object{error=string}
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/orders/{orderID}/convert-to-invoice [post]
func (h *Handlers) ConvertOrderToInvoice(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.GetClaims(r.Context())
	tenantID := chi.URLParam(r, "tenantID")
	orderID := chi.URLParam(r, "orderID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	var req orders.ConvertOrderToInvoiceRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	req.UserID = claims.UserID
	if req.IssueDate.IsZero() {
		req.IssueDate = time.Now()
	}
	if req.DueDate.IsZero() {
		req.DueDate = req.IssueDate.AddDate(0, 0, 14)
	}
	if req.DueDate.Before(req.IssueDate) {
		respondError(w, http.StatusBadRequest, "due date cannot be before issue date")
		return
	}
	if h.rejectLockedPeriod(w, r.Context(), tenantID, req.IssueDate) {
		return
	}

	order, err := h.ordersService.GetByID(r.Context(), tenantID, schemaName, orderID)
	if err != nil {
		respondError(w, http.StatusNotFound, "Order not found")
		return
	}
	if order.Status != orders.OrderStatusDelivered {
		respondError(w, http.StatusConflict, "order must be delivered before conversion")
		return
	}
	if order.ConvertedToInvoiceID != nil {
		respondError(w, http.StatusConflict, "order has already been converted to an invoice")
		return
	}

	notes := strings.TrimSpace(req.Notes)
	if notes == "" {
		notes = order.Notes
	}
	invoiceReq := invoicing.CreateInvoiceRequest{
		InvoiceType:  invoicing.InvoiceTypeSales,
		ContactID:    order.ContactID,
		IssueDate:    req.IssueDate,
		DueDate:      req.DueDate,
		Currency:     order.Currency,
		ExchangeRate: order.ExchangeRate,
		Reference:    order.OrderNumber,
		Notes:        notes,
		UserID:       req.UserID,
	}
	for _, line := range order.Lines {
		invoiceReq.Lines = append(invoiceReq.Lines, invoicing.CreateInvoiceLineRequest{
			Description:     line.Description,
			Quantity:        line.Quantity,
			Unit:            line.Unit,
			UnitPrice:       line.UnitPrice,
			DiscountPercent: line.DiscountPercent,
			VATRate:         line.VATRate,
			ProductID:       line.ProductID,
		})
	}

	invoice, err := h.invoicingService.Create(r.Context(), tenantID, schemaName, &invoiceReq)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.ordersService.ConvertToInvoice(r.Context(), tenantID, schemaName, orderID, invoice.ID); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to mark order converted")
		return
	}
	order.ConvertedToInvoiceID = &invoice.ID

	respondJSON(w, http.StatusCreated, &orders.OrderInvoiceConversionResult{
		Order:   order,
		Invoice: invoice,
	})
}

// =============================================================================
// FIXED ASSETS HANDLERS
// =============================================================================

// ListAssetCategories returns all asset categories for a tenant
// @Summary List asset categories
// @Description Get all asset categories for a tenant
// @Tags Fixed Assets
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Success 200 {array} assets.AssetCategory
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/asset-categories [get]
func (h *Handlers) ListAssetCategories(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	categories, err := h.assetsService.ListCategories(r.Context(), tenantID, schemaName)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to list asset categories")
		return
	}

	respondJSON(w, http.StatusOK, categories)
}

// CreateAssetCategory creates a new asset category
// @Summary Create asset category
// @Description Create a new asset category
// @Tags Fixed Assets
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param request body assets.CreateCategoryRequest true "Category details"
// @Success 201 {object} assets.AssetCategory
// @Failure 400 {object} object{error=string}
// @Router /tenants/{tenantID}/asset-categories [post]
func (h *Handlers) CreateAssetCategory(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	var req assets.CreateCategoryRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Name == "" {
		respondError(w, http.StatusBadRequest, "Category name is required")
		return
	}

	category, err := h.assetsService.CreateCategory(r.Context(), tenantID, schemaName, &req)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, category)
}

// GetAssetCategory returns an asset category by ID
// @Summary Get asset category
// @Description Get asset category details by ID
// @Tags Fixed Assets
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param categoryID path string true "Category ID"
// @Success 200 {object} assets.AssetCategory
// @Failure 404 {object} object{error=string}
// @Router /tenants/{tenantID}/asset-categories/{categoryID} [get]
func (h *Handlers) GetAssetCategory(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	categoryID := chi.URLParam(r, "categoryID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	category, err := h.assetsService.GetCategoryByID(r.Context(), tenantID, schemaName, categoryID)
	if err != nil {
		respondError(w, http.StatusNotFound, "Category not found")
		return
	}

	respondJSON(w, http.StatusOK, category)
}

// DeleteAssetCategory deletes an asset category
// @Summary Delete asset category
// @Description Delete an asset category
// @Tags Fixed Assets
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param categoryID path string true "Category ID"
// @Success 204 "No Content"
// @Failure 400 {object} object{error=string}
// @Router /tenants/{tenantID}/asset-categories/{categoryID} [delete]
func (h *Handlers) DeleteAssetCategory(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	categoryID := chi.URLParam(r, "categoryID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	if err := h.assetsService.DeleteCategory(r.Context(), tenantID, schemaName, categoryID); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ListAssets returns all fixed assets for a tenant
// @Summary List fixed assets
// @Description Get all fixed assets for a tenant with optional filtering
// @Tags Fixed Assets
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param status query string false "Filter by status (DRAFT, ACTIVE, DISPOSED, SOLD)"
// @Param category_id query string false "Filter by category ID"
// @Param search query string false "Search in name or asset number"
// @Success 200 {array} assets.FixedAsset
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/assets [get]
func (h *Handlers) ListAssets(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	filter := &assets.AssetFilter{
		Search: r.URL.Query().Get("search"),
	}

	if status := r.URL.Query().Get("status"); status != "" {
		filter.Status = assets.AssetStatus(status)
	}
	if categoryID := r.URL.Query().Get("category_id"); categoryID != "" {
		filter.CategoryID = categoryID
	}

	assetList, err := h.assetsService.List(r.Context(), tenantID, schemaName, filter)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to list assets")
		return
	}

	respondJSON(w, http.StatusOK, assetList)
}

// CreateAsset creates a new fixed asset
// @Summary Create fixed asset
// @Description Create a new fixed asset
// @Tags Fixed Assets
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param request body assets.CreateAssetRequest true "Asset details"
// @Success 201 {object} assets.FixedAsset
// @Failure 400 {object} object{error=string}
// @Router /tenants/{tenantID}/assets [post]
func (h *Handlers) CreateAsset(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.GetClaims(r.Context())
	tenantID := chi.URLParam(r, "tenantID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	var req assets.CreateAssetRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	req.UserID = claims.UserID

	asset, err := h.assetsService.Create(r.Context(), tenantID, schemaName, &req)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, asset)
}

// ImportAssets imports fixed assets from CSV
// @Summary Import fixed assets
// @Description Import fixed assets from CSV, preserving optional legacy asset numbers and cutover depreciation values
// @Tags Fixed Assets
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param request body assets.ImportAssetsRequest true "CSV import payload"
// @Success 200 {object} assets.ImportAssetsResult
// @Failure 400 {object} object{error=string}
// @Router /tenants/{tenantID}/assets/import [post]
func (h *Handlers) ImportAssets(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.GetClaims(r.Context())
	tenantID := chi.URLParam(r, "tenantID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	var req assets.ImportAssetsRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if strings.TrimSpace(req.CSVContent) == "" {
		respondError(w, http.StatusBadRequest, "csv_content is required")
		return
	}

	req.UserID = claims.UserID
	result, err := h.assetsService.ImportAssetsCSV(r.Context(), tenantID, schemaName, &req)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, result)
}

// GetAsset returns a fixed asset by ID
// @Summary Get fixed asset
// @Description Get fixed asset details by ID
// @Tags Fixed Assets
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param assetID path string true "Asset ID"
// @Success 200 {object} assets.FixedAsset
// @Failure 404 {object} object{error=string}
// @Router /tenants/{tenantID}/assets/{assetID} [get]
func (h *Handlers) GetAsset(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	assetID := chi.URLParam(r, "assetID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	asset, err := h.assetsService.GetByID(r.Context(), tenantID, schemaName, assetID)
	if err != nil {
		respondError(w, http.StatusNotFound, "Asset not found")
		return
	}

	respondJSON(w, http.StatusOK, asset)
}

// UpdateAsset updates a fixed asset
// @Summary Update fixed asset
// @Description Update a draft or active fixed asset
// @Tags Fixed Assets
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param assetID path string true "Asset ID"
// @Param request body assets.UpdateAssetRequest true "Asset details"
// @Success 200 {object} assets.FixedAsset
// @Failure 400 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Router /tenants/{tenantID}/assets/{assetID} [put]
func (h *Handlers) UpdateAsset(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	assetID := chi.URLParam(r, "assetID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	var req assets.UpdateAssetRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	asset, err := h.assetsService.Update(r.Context(), tenantID, schemaName, assetID, &req)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, asset)
}

// DeleteAsset deletes a draft fixed asset
// @Summary Delete fixed asset
// @Description Delete a draft fixed asset
// @Tags Fixed Assets
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param assetID path string true "Asset ID"
// @Success 204 "No Content"
// @Failure 400 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Router /tenants/{tenantID}/assets/{assetID} [delete]
func (h *Handlers) DeleteAsset(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	assetID := chi.URLParam(r, "assetID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	if err := h.assetsService.Delete(r.Context(), tenantID, schemaName, assetID); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ActivateAsset marks an asset as active
// @Summary Activate fixed asset
// @Description Mark a draft fixed asset as active after approved asset evidence is attached
// @Tags Fixed Assets
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param assetID path string true "Asset ID"
// @Success 200 {object} object{status=string}
// @Failure 400 {object} object{error=string}
// @Failure 409 {object} object{error=string,evidence_policy_results=[]documents.EvidencePolicyResult,remediation_actions=[]documents.DocumentRemediationAction}
// @Router /tenants/{tenantID}/assets/{assetID}/activate [post]
func (h *Handlers) ActivateAsset(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	assetID := chi.URLParam(r, "assetID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	if err := h.requireApprovedAssetActivationEvidence(r.Context(), schemaName, tenantID, assetID); err != nil {
		var conflict *evidencePolicyConflictError
		if errors.As(err, &conflict) {
			respondEvidencePolicyConflict(w, conflict.Error(), conflict.Results)
			return
		}
		status := http.StatusInternalServerError
		switch {
		case errors.Is(err, errApprovedAssetActivationEvidenceRequired):
			status = http.StatusConflict
		case strings.Contains(err.Error(), "get asset"):
			status = http.StatusBadRequest
		}
		respondError(w, status, err.Error())
		return
	}

	if err := h.assetsService.Activate(r.Context(), tenantID, schemaName, assetID); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "active"})
}

func (h *Handlers) requireApprovedAssetActivationEvidence(ctx context.Context, schemaName, tenantID, assetID string) error {
	asset, err := h.assetsService.GetByID(ctx, tenantID, schemaName, assetID)
	if err != nil {
		return fmt.Errorf("get asset: %w", err)
	}
	if asset.Status != assets.AssetStatusDraft {
		return nil
	}
	if h.documentsService == nil {
		return fmt.Errorf("%w before activating fixed asset %s", errApprovedAssetActivationEvidenceRequired, assetID)
	}

	results, err := evaluateDocumentsEvidencePolicy(h.documentsService, ctx, schemaName, tenantID, &documents.EvidencePolicyRequest{
		EntityType: documents.EntityTypeAsset,
		EntityIDs:  []string{assetID},
		Rules: []documents.EvidencePolicyRule{{
			DocumentTypes: []string{
				documents.DocumentTypeAssetRecord,
				documents.DocumentTypeReceipt,
				documents.DocumentTypeContract,
			},
			MinCount:        1,
			RequireApproved: true,
		}},
	})
	if err != nil {
		return fmt.Errorf("evaluate asset evidence: %w", err)
	}
	if len(results) == 0 {
		return fmt.Errorf("%w before activating fixed asset %s", errApprovedAssetActivationEvidenceRequired, assetID)
	}
	for _, result := range results {
		if !result.Compliant {
			return &evidencePolicyConflictError{
				Err:     fmt.Errorf("%w before activating fixed asset %s", errApprovedAssetActivationEvidenceRequired, assetID),
				Results: results,
			}
		}
	}

	return nil
}

// DisposeAsset marks an asset as disposed
// @Summary Dispose fixed asset
// @Description Mark an active fixed asset as disposed or sold after approved disposal evidence is attached
// @Tags Fixed Assets
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param assetID path string true "Asset ID"
// @Param request body assets.DisposeAssetRequest true "Disposal details"
// @Success 200 {object} object{status=string}
// @Failure 400 {object} object{error=string}
// @Failure 409 {object} object{error=string,evidence_policy_results=[]documents.EvidencePolicyResult,remediation_actions=[]documents.DocumentRemediationAction}
// @Router /tenants/{tenantID}/assets/{assetID}/dispose [post]
func (h *Handlers) DisposeAsset(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.GetClaims(r.Context())
	tenantID := chi.URLParam(r, "tenantID")
	assetID := chi.URLParam(r, "assetID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	var req assets.DisposeAssetRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	req.UserID = claims.UserID

	if err := h.requireApprovedAssetDisposalEvidence(r.Context(), schemaName, tenantID, assetID); err != nil {
		var conflict *evidencePolicyConflictError
		if errors.As(err, &conflict) {
			respondEvidencePolicyConflict(w, conflict.Error(), conflict.Results)
			return
		}
		status := http.StatusInternalServerError
		switch {
		case errors.Is(err, errApprovedAssetDisposalEvidenceRequired):
			status = http.StatusConflict
		case strings.Contains(err.Error(), "get asset"):
			status = http.StatusBadRequest
		}
		respondError(w, status, err.Error())
		return
	}

	if err := h.assetsService.Dispose(r.Context(), tenantID, schemaName, assetID, &req); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "disposed"})
}

func (h *Handlers) requireApprovedAssetDisposalEvidence(ctx context.Context, schemaName, tenantID, assetID string) error {
	asset, err := h.assetsService.GetByID(ctx, tenantID, schemaName, assetID)
	if err != nil {
		return fmt.Errorf("get asset: %w", err)
	}
	if asset.Status != assets.AssetStatusActive {
		return nil
	}
	if h.documentsService == nil {
		return fmt.Errorf("%w before disposing fixed asset %s", errApprovedAssetDisposalEvidenceRequired, assetID)
	}

	results, err := evaluateDocumentsEvidencePolicy(h.documentsService, ctx, schemaName, tenantID, &documents.EvidencePolicyRequest{
		EntityType: documents.EntityTypeAsset,
		EntityIDs:  []string{assetID},
		Rules: []documents.EvidencePolicyRule{{
			DocumentTypes: []string{
				documents.DocumentTypeSupportingDocument,
				documents.DocumentTypeContract,
			},
			MinCount:        1,
			RequireApproved: true,
		}},
	})
	if err != nil {
		return fmt.Errorf("evaluate asset disposal evidence: %w", err)
	}
	if len(results) == 0 {
		return fmt.Errorf("%w before disposing fixed asset %s", errApprovedAssetDisposalEvidenceRequired, assetID)
	}
	for _, result := range results {
		if !result.Compliant {
			return &evidencePolicyConflictError{
				Err:     fmt.Errorf("%w before disposing fixed asset %s", errApprovedAssetDisposalEvidenceRequired, assetID),
				Results: results,
			}
		}
	}

	return nil
}

// RecordDepreciation records depreciation for an asset
// @Summary Record depreciation
// @Description Record monthly depreciation for an active fixed asset and post a depreciation journal entry when depreciation accounts are configured
// @Tags Fixed Assets
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param assetID path string true "Asset ID"
// @Success 201 {object} assets.DepreciationEntry
// @Failure 400 {object} object{error=string}
// @Router /tenants/{tenantID}/assets/{assetID}/depreciation [post]
func (h *Handlers) RecordDepreciation(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.GetClaims(r.Context())
	tenantID := chi.URLParam(r, "tenantID")
	assetID := chi.URLParam(r, "assetID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	// Default to current month
	now := time.Now()
	periodStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	periodEnd := periodStart.AddDate(0, 1, -1)

	entry, err := h.assetsService.RecordDepreciation(r.Context(), tenantID, schemaName, assetID, claims.UserID, periodStart, periodEnd)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, entry)
}

// GetDepreciationHistory returns depreciation history for an asset
// @Summary Get depreciation history
// @Description Get all depreciation entries for a fixed asset
// @Tags Fixed Assets
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param assetID path string true "Asset ID"
// @Success 200 {array} assets.DepreciationEntry
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/assets/{assetID}/depreciation [get]
func (h *Handlers) GetDepreciationHistory(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	assetID := chi.URLParam(r, "assetID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	entries, err := h.assetsService.GetDepreciationHistory(r.Context(), tenantID, schemaName, assetID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get depreciation history")
		return
	}

	respondJSON(w, http.StatusOK, entries)
}

// ============================================================================
// Inventory Handlers
// ============================================================================

// ListProductCategories lists all product categories.
// @Summary List product categories
// @Description List product categories for a tenant
// @Tags Inventory
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Success 200 {array} inventory.ProductCategory
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/product-categories [get]
func (h *Handlers) ListProductCategories(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	categories, err := h.inventoryService.ListCategories(r.Context(), tenantID, schemaName)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to list categories")
		return
	}

	respondJSON(w, http.StatusOK, categories)
}

// CreateProductCategory creates a new product category.
// @Summary Create product category
// @Description Create a product category, optionally under a parent category
// @Tags Inventory
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param request body inventory.CreateCategoryRequest true "Product category"
// @Success 201 {object} inventory.ProductCategory
// @Failure 400 {object} object{error=string}
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/product-categories [post]
func (h *Handlers) CreateProductCategory(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	var req inventory.CreateCategoryRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	category, err := h.inventoryService.CreateCategory(r.Context(), tenantID, schemaName, &req)
	if err != nil {
		if strings.Contains(err.Error(), "must be a valid UUID") {
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}
		respondError(w, http.StatusInternalServerError, "Failed to create category")
		return
	}

	respondJSON(w, http.StatusCreated, category)
}

// ImportProductCategories imports product category master data from CSV
// @Summary Import product categories
// @Description Import product category master data from CSV and resolve parent categories by id or name
// @Tags Inventory
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param request body inventory.ImportProductCategoriesRequest true "CSV import payload"
// @Success 200 {object} inventory.ImportProductCategoriesResult
// @Failure 400 {object} object{error=string}
// @Router /tenants/{tenantID}/product-categories/import [post]
func (h *Handlers) ImportProductCategories(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	var req inventory.ImportProductCategoriesRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if strings.TrimSpace(req.CSVContent) == "" {
		respondError(w, http.StatusBadRequest, "csv_content is required")
		return
	}

	result, err := h.inventoryService.ImportProductCategoriesCSV(r.Context(), tenantID, schemaName, &req)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, result)
}

// GetProductCategory gets a product category by ID.
// @Summary Get product category
// @Description Get one product category by ID
// @Tags Inventory
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param categoryID path string true "Product category ID"
// @Success 200 {object} inventory.ProductCategory
// @Failure 404 {object} object{error=string}
// @Router /tenants/{tenantID}/product-categories/{categoryID} [get]
func (h *Handlers) GetProductCategory(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	categoryID := chi.URLParam(r, "categoryID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	category, err := h.inventoryService.GetCategoryByID(r.Context(), tenantID, schemaName, categoryID)
	if err != nil {
		respondError(w, http.StatusNotFound, "Category not found")
		return
	}

	respondJSON(w, http.StatusOK, category)
}

// DeleteProductCategory deletes a product category.
// @Summary Delete product category
// @Description Delete a product category
// @Tags Inventory
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param categoryID path string true "Product category ID"
// @Success 200 {object} object{status=string}
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/product-categories/{categoryID} [delete]
func (h *Handlers) DeleteProductCategory(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	categoryID := chi.URLParam(r, "categoryID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	if err := h.inventoryService.DeleteCategory(r.Context(), tenantID, schemaName, categoryID); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to delete category")
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// ListProducts lists all products.
// @Summary List products
// @Description List products and services with optional type, status, category, search, and low stock filters
// @Tags Inventory
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param product_type query string false "Product type: GOODS or SERVICE"
// @Param status query string false "Product status: ACTIVE or INACTIVE"
// @Param category_id query string false "Category ID"
// @Param search query string false "Search text"
// @Param low_stock query bool false "Only include low stock products"
// @Success 200 {array} inventory.Product
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/products [get]
func (h *Handlers) ListProducts(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	filter := &inventory.ProductFilter{
		ProductType: inventory.ProductType(r.URL.Query().Get("product_type")),
		Status:      inventory.ProductStatus(r.URL.Query().Get("status")),
		CategoryID:  r.URL.Query().Get("category_id"),
		Search:      r.URL.Query().Get("search"),
		LowStock:    r.URL.Query().Get("low_stock") == "true",
	}

	products, err := h.inventoryService.ListProducts(r.Context(), tenantID, schemaName, filter)
	if err != nil {
		if strings.Contains(err.Error(), "valid UUID") {
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}
		respondError(w, http.StatusInternalServerError, "Failed to list products")
		return
	}

	respondJSON(w, http.StatusOK, products)
}

// CreateProduct creates a new product.
// @Summary Create product
// @Description Create a product or service with pricing, account, and inventory settings
// @Tags Inventory
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param request body inventory.CreateProductRequest true "Product"
// @Success 201 {object} inventory.Product
// @Failure 400 {object} object{error=string}
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/products [post]
func (h *Handlers) CreateProduct(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	var req inventory.CreateProductRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	product, err := h.inventoryService.CreateProduct(r.Context(), tenantID, schemaName, &req)
	if err != nil {
		if strings.Contains(err.Error(), "valid UUID") {
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}
		respondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to create product: %v", err))
		return
	}

	respondJSON(w, http.StatusCreated, product)
}

// ImportProducts imports product master data from CSV
// @Summary Import products
// @Description Import product master data from CSV, preserving optional product codes and resolving category names
// @Tags Inventory
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param request body inventory.ImportProductsRequest true "CSV import payload"
// @Success 200 {object} inventory.ImportProductsResult
// @Failure 400 {object} object{error=string}
// @Router /tenants/{tenantID}/products/import [post]
func (h *Handlers) ImportProducts(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	var req inventory.ImportProductsRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if strings.TrimSpace(req.CSVContent) == "" {
		respondError(w, http.StatusBadRequest, "csv_content is required")
		return
	}

	result, err := h.inventoryService.ImportProductsCSV(r.Context(), tenantID, schemaName, &req)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, result)
}

// GetProduct gets a product by ID.
// @Summary Get product
// @Description Get one product or service by ID
// @Tags Inventory
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param productID path string true "Product ID"
// @Success 200 {object} inventory.Product
// @Failure 404 {object} object{error=string}
// @Router /tenants/{tenantID}/products/{productID} [get]
func (h *Handlers) GetProduct(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	productID := chi.URLParam(r, "productID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	product, err := h.inventoryService.GetProductByID(r.Context(), tenantID, schemaName, productID)
	if err != nil {
		respondError(w, http.StatusNotFound, "Product not found")
		return
	}

	respondJSON(w, http.StatusOK, product)
}

// UpdateProduct updates a product.
// @Summary Update product
// @Description Update product or service details, pricing, accounts, and inventory settings
// @Tags Inventory
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param productID path string true "Product ID"
// @Param request body inventory.UpdateProductRequest true "Product update"
// @Success 200 {object} inventory.Product
// @Failure 400 {object} object{error=string}
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/products/{productID} [put]
func (h *Handlers) UpdateProduct(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	productID := chi.URLParam(r, "productID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	var req inventory.UpdateProductRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	product, err := h.inventoryService.UpdateProduct(r.Context(), tenantID, schemaName, productID, &req)
	if err != nil {
		if strings.Contains(err.Error(), "valid UUID") {
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}
		respondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to update product: %v", err))
		return
	}

	respondJSON(w, http.StatusOK, product)
}

// DeleteProduct deletes a product.
// @Summary Delete product
// @Description Delete a product or service
// @Tags Inventory
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param productID path string true "Product ID"
// @Success 200 {object} object{status=string}
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/products/{productID} [delete]
func (h *Handlers) DeleteProduct(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	productID := chi.URLParam(r, "productID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	if err := h.inventoryService.DeleteProduct(r.Context(), tenantID, schemaName, productID); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to delete product")
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// GetStockLevels gets stock levels for a product
// @Summary List product stock levels
// @Description List per-warehouse stock levels for one product
// @Tags Inventory
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param productID path string true "Product ID"
// @Success 200 {array} inventory.StockLevel
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/products/{productID}/stock-levels [get]
func (h *Handlers) GetStockLevels(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	productID := chi.URLParam(r, "productID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	levels, err := h.inventoryService.GetStockLevels(r.Context(), tenantID, schemaName, productID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get stock levels")
		return
	}

	respondJSON(w, http.StatusOK, levels)
}

// GetInventoryMovements gets inventory movements for a product
// @Summary List product inventory movements
// @Description List stock movements for one product, including optional lot, serial, expiry, and source metadata
// @Tags Inventory
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param productID path string true "Product ID"
// @Success 200 {array} inventory.InventoryMovement
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/products/{productID}/movements [get]
func (h *Handlers) GetInventoryMovements(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	productID := chi.URLParam(r, "productID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	movements, err := h.inventoryService.GetMovements(r.Context(), tenantID, schemaName, productID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get movements")
		return
	}

	respondJSON(w, http.StatusOK, movements)
}

// GetInventoryValuation returns inventory valuation by warehouse.
// @Summary Get inventory valuation
// @Description Return valued on-hand stock for tracked goods using the explicit valuation method or the tenant inventory valuation policy
// @Tags Inventory
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param warehouse_id query string false "Warehouse ID"
// @Param method query string false "Valuation method override: standard-cost, weighted-average, or fifo"
// @Success 200 {object} inventory.InventoryValuationReport
// @Failure 400 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/inventory/valuation [get]
func (h *Handlers) GetInventoryValuation(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	tenantRecord, err := h.tenantService.GetTenant(r.Context(), tenantID)
	if err != nil {
		respondError(w, http.StatusNotFound, "Tenant not found")
		return
	}
	schemaName := tenantRecord.SchemaName
	warehouseID := strings.TrimSpace(r.URL.Query().Get("warehouse_id"))
	method := tenantInventoryValuationMethod(tenantRecord, r.URL.Query().Get("method"))

	report, err := h.inventoryService.GetInventoryValuation(r.Context(), tenantID, schemaName, warehouseID, method)
	if err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "invalid valuation method") || (warehouseID != "" && strings.Contains(err.Error(), "warehouse")) {
			status = http.StatusBadRequest
		}
		respondError(w, status, fmt.Sprintf("Failed to get inventory valuation: %v", err))
		return
	}

	respondJSON(w, http.StatusOK, report)
}

// GetInventorySubledgerReconciliation returns inventory subledger reconciliation by inventory asset account.
// @Summary Get inventory subledger reconciliation
// @Description Compare valued inventory stock against posted general-ledger balances by configured product inventory asset account
// @Tags Inventory
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param warehouse_id query string false "Warehouse ID"
// @Param method query string false "Valuation method override: standard-cost, weighted-average, or fifo"
// @Param as_of_date query string false "GL balance date in YYYY-MM-DD format"
// @Success 200 {object} inventory.InventorySubledgerReconciliationReport
// @Failure 400 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/inventory/subledger-reconciliation [get]
func (h *Handlers) GetInventorySubledgerReconciliation(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	tenantRecord, err := h.tenantService.GetTenant(r.Context(), tenantID)
	if err != nil {
		respondError(w, http.StatusNotFound, "Tenant not found")
		return
	}
	schemaName := tenantRecord.SchemaName
	warehouseID := strings.TrimSpace(r.URL.Query().Get("warehouse_id"))
	method := tenantInventoryValuationMethod(tenantRecord, r.URL.Query().Get("method"))
	asOfDate, err := inventorySubledgerAsOfDate(r)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	report, err := h.inventoryService.GetInventorySubledgerReconciliation(r.Context(), tenantID, schemaName, warehouseID, method, asOfDate)
	if err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "invalid valuation method") || (warehouseID != "" && strings.Contains(err.Error(), "warehouse")) {
			status = http.StatusBadRequest
		}
		respondError(w, status, fmt.Sprintf("Failed to get inventory subledger reconciliation: %v", err))
		return
	}

	respondJSON(w, http.StatusOK, report)
}

func inventorySubledgerAsOfDate(r *http.Request) (time.Time, error) {
	raw := strings.TrimSpace(r.URL.Query().Get("as_of_date"))
	if raw == "" {
		return time.Time{}, nil
	}
	parsed, err := time.Parse("2006-01-02", raw)
	if err != nil {
		return time.Time{}, fmt.Errorf("as_of_date must be in YYYY-MM-DD format")
	}
	return parsed, nil
}

// GetInventoryLotReport returns stock grouped by lot, serial, and expiry metadata.
// @Summary Get inventory lot report
// @Description Return on-hand tracked goods stock grouped by lot number, serial number, expiry date, and warehouse
// @Tags Inventory
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param product_id query string false "Product ID"
// @Param warehouse_id query string false "Warehouse ID"
// @Param include_empty query bool false "Include zero or negative lot positions"
// @Success 200 {object} inventory.InventoryLotReport
// @Failure 400 {object} object{error=string}
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/inventory/lots [get]
func (h *Handlers) GetInventoryLotReport(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	schemaName := h.getSchemaName(r.Context(), tenantID)
	productID := strings.TrimSpace(r.URL.Query().Get("product_id"))
	warehouseID := strings.TrimSpace(r.URL.Query().Get("warehouse_id"))
	includeEmpty := false
	if value := strings.TrimSpace(r.URL.Query().Get("include_empty")); value != "" {
		parsed, err := strconv.ParseBool(value)
		if err != nil {
			respondError(w, http.StatusBadRequest, "include_empty must be a boolean")
			return
		}
		includeEmpty = parsed
	}

	report, err := h.inventoryService.GetInventoryLotReport(r.Context(), tenantID, schemaName, productID, warehouseID, includeEmpty)
	if err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "get product") || strings.Contains(err.Error(), "get warehouse") {
			status = http.StatusBadRequest
		}
		respondError(w, status, fmt.Sprintf("Failed to get inventory lot report: %v", err))
		return
	}

	respondJSON(w, http.StatusOK, report)
}

// ListWarehouses lists all warehouses.
// @Summary List warehouses
// @Description List tenant warehouses, optionally filtering to active warehouses
// @Tags Inventory
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param active_only query bool false "Only include active warehouses"
// @Success 200 {array} inventory.Warehouse
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/warehouses [get]
func (h *Handlers) ListWarehouses(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	activeOnly := r.URL.Query().Get("active_only") == "true"

	warehouses, err := h.inventoryService.ListWarehouses(r.Context(), tenantID, schemaName, activeOnly)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to list warehouses")
		return
	}

	respondJSON(w, http.StatusOK, warehouses)
}

// CreateWarehouse creates a new warehouse.
// @Summary Create warehouse
// @Description Create a warehouse or storage location
// @Tags Inventory
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param request body inventory.CreateWarehouseRequest true "Warehouse"
// @Success 201 {object} inventory.Warehouse
// @Failure 400 {object} object{error=string}
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/warehouses [post]
func (h *Handlers) CreateWarehouse(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	var req inventory.CreateWarehouseRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	warehouse, err := h.inventoryService.CreateWarehouse(r.Context(), tenantID, schemaName, &req)
	if err != nil {
		respondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to create warehouse: %v", err))
		return
	}

	respondJSON(w, http.StatusCreated, warehouse)
}

// ImportWarehouses imports warehouse master data from CSV
// @Summary Import warehouses
// @Description Import warehouse master data from CSV, preserving warehouse codes and active/default flags
// @Tags Inventory
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param request body inventory.ImportWarehousesRequest true "CSV import payload"
// @Success 200 {object} inventory.ImportWarehousesResult
// @Failure 400 {object} object{error=string}
// @Router /tenants/{tenantID}/warehouses/import [post]
func (h *Handlers) ImportWarehouses(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	var req inventory.ImportWarehousesRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if strings.TrimSpace(req.CSVContent) == "" {
		respondError(w, http.StatusBadRequest, "csv_content is required")
		return
	}

	result, err := h.inventoryService.ImportWarehousesCSV(r.Context(), tenantID, schemaName, &req)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, result)
}

// GetWarehouse gets a warehouse by ID.
// @Summary Get warehouse
// @Description Get one warehouse by ID
// @Tags Inventory
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param warehouseID path string true "Warehouse ID"
// @Success 200 {object} inventory.Warehouse
// @Failure 404 {object} object{error=string}
// @Router /tenants/{tenantID}/warehouses/{warehouseID} [get]
func (h *Handlers) GetWarehouse(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	warehouseID := chi.URLParam(r, "warehouseID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	warehouse, err := h.inventoryService.GetWarehouseByID(r.Context(), tenantID, schemaName, warehouseID)
	if err != nil {
		respondError(w, http.StatusNotFound, "Warehouse not found")
		return
	}

	respondJSON(w, http.StatusOK, warehouse)
}

// UpdateWarehouse updates a warehouse.
// @Summary Update warehouse
// @Description Update warehouse details, default status, and active status
// @Tags Inventory
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param warehouseID path string true "Warehouse ID"
// @Param request body inventory.UpdateWarehouseRequest true "Warehouse update"
// @Success 200 {object} inventory.Warehouse
// @Failure 400 {object} object{error=string}
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/warehouses/{warehouseID} [put]
func (h *Handlers) UpdateWarehouse(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	warehouseID := chi.URLParam(r, "warehouseID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	var req inventory.UpdateWarehouseRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	warehouse, err := h.inventoryService.UpdateWarehouse(r.Context(), tenantID, schemaName, warehouseID, &req)
	if err != nil {
		respondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to update warehouse: %v", err))
		return
	}

	respondJSON(w, http.StatusOK, warehouse)
}

// DeleteWarehouse deletes a warehouse.
// @Summary Delete warehouse
// @Description Delete a warehouse or storage location
// @Tags Inventory
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param warehouseID path string true "Warehouse ID"
// @Success 200 {object} object{status=string}
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/warehouses/{warehouseID} [delete]
func (h *Handlers) DeleteWarehouse(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	warehouseID := chi.URLParam(r, "warehouseID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	if err := h.inventoryService.DeleteWarehouse(r.Context(), tenantID, schemaName, warehouseID); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to delete warehouse")
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// ImportStockAdjustments imports stock adjustments from CSV
// @Summary Import stock adjustments
// @Description Import signed stock adjustment rows from CSV using product and warehouse IDs or codes
// @Tags Inventory
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param request body inventory.ImportStockAdjustmentsRequest true "CSV import payload"
// @Success 200 {object} inventory.ImportStockAdjustmentsResult
// @Failure 400 {object} object{error=string}
// @Router /tenants/{tenantID}/inventory/stock-import [post]
func (h *Handlers) ImportStockAdjustments(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	claims, ok := auth.GetClaims(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Invalid or missing authentication")
		return
	}

	var req inventory.ImportStockAdjustmentsRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if strings.TrimSpace(req.CSVContent) == "" {
		respondError(w, http.StatusBadRequest, "csv_content is required")
		return
	}
	req.UserID = claims.UserID

	result, err := h.inventoryService.ImportStockAdjustmentsCSV(r.Context(), tenantID, schemaName, &req)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, result)
}

// AdjustStock adjusts stock for a product
// @Summary Adjust product stock
// @Description Apply a signed stock adjustment in one warehouse, optionally recording lot number, serial number, and expiry date metadata
// @Tags Inventory
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param request body inventory.AdjustStockRequest true "Stock adjustment"
// @Success 200 {object} inventory.InventoryMovement
// @Failure 400 {object} object{error=string}
// @Failure 401 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/inventory/adjust [post]
func (h *Handlers) AdjustStock(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	claims, ok := auth.GetClaims(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Invalid or missing authentication")
		return
	}

	var req inventory.AdjustStockRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	req.UserID = claims.UserID

	movement, err := h.inventoryService.AdjustStock(r.Context(), tenantID, schemaName, &req)
	if err != nil {
		respondError(w, http.StatusBadRequest, fmt.Sprintf("Failed to adjust stock: %v", err))
		return
	}

	respondJSON(w, http.StatusOK, movement)
}

// IssueStock consumes available stock from a warehouse.
// @Summary Issue warehouse stock
// @Description Consume positive available stock from one warehouse with optional lot/serial/expiry allocation, tenant/default issue costing policy, and accounting-ready COGS lines
// @Tags Inventory
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param request body inventory.IssueStockRequest true "Stock issue"
// @Success 200 {object} inventory.IssueStockResult
// @Failure 400 {object} object{error=string}
// @Failure 401 {object} object{error=string}
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/inventory/issue [post]
func (h *Handlers) IssueStock(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	tenantRecord, err := h.tenantService.GetTenant(r.Context(), tenantID)
	if err != nil {
		respondError(w, http.StatusNotFound, "Tenant not found")
		return
	}
	schemaName := tenantRecord.SchemaName

	claims, ok := auth.GetClaims(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Invalid or missing authentication")
		return
	}

	var req inventory.IssueStockRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	req.UserID = claims.UserID
	req.CostingMethod = tenantInventoryIssueCostingMethod(tenantRecord, req.CostingMethod)

	result, err := h.inventoryService.IssueStock(r.Context(), tenantID, schemaName, &req)
	if err != nil {
		respondError(w, http.StatusBadRequest, fmt.Sprintf("Failed to issue stock: %v", err))
		return
	}

	respondJSON(w, http.StatusOK, result)
}

// TransferStock transfers stock between warehouses
// @Summary Transfer product stock
// @Description Move positive available stock between warehouses without changing total product stock
// @Tags Inventory
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param request body inventory.TransferStockRequest true "Stock transfer"
// @Success 200 {object} object{status=string}
// @Failure 400 {object} object{error=string}
// @Failure 401 {object} object{error=string}
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/inventory/transfer [post]
func (h *Handlers) TransferStock(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	claims, ok := auth.GetClaims(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Invalid or missing authentication")
		return
	}

	var req inventory.TransferStockRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	req.UserID = claims.UserID

	if err := h.inventoryService.TransferStock(r.Context(), tenantID, schemaName, &req); err != nil {
		respondError(w, http.StatusBadRequest, fmt.Sprintf("Failed to transfer stock: %v", err))
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "transferred"})
}

// ReserveStock reserves available stock in a warehouse.
// @Summary Reserve warehouse stock
// @Description Move available stock into reserved quantity for a product in one warehouse
// @Tags Inventory
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param request body inventory.StockReservationRequest true "Stock reservation"
// @Success 200 {object} inventory.StockLevel
// @Failure 400 {object} object{error=string}
// @Failure 401 {object} object{error=string}
// @Router /tenants/{tenantID}/inventory/reserve [post]
func (h *Handlers) ReserveStock(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	claims, ok := auth.GetClaims(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Invalid or missing authentication")
		return
	}

	var req inventory.StockReservationRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	req.UserID = claims.UserID

	level, err := h.inventoryService.ReserveStock(r.Context(), tenantID, schemaName, &req)
	if err != nil {
		respondError(w, http.StatusBadRequest, fmt.Sprintf("Failed to reserve stock: %v", err))
		return
	}

	respondJSON(w, http.StatusOK, level)
}

// ReleaseStock releases reserved stock back to available quantity.
// @Summary Release reserved warehouse stock
// @Description Release reserved stock back to available quantity for a product in one warehouse
// @Tags Inventory
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param request body inventory.StockReservationRequest true "Stock release"
// @Success 200 {object} inventory.StockLevel
// @Failure 400 {object} object{error=string}
// @Failure 401 {object} object{error=string}
// @Router /tenants/{tenantID}/inventory/release [post]
func (h *Handlers) ReleaseStock(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	claims, ok := auth.GetClaims(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Invalid or missing authentication")
		return
	}

	var req inventory.StockReservationRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	req.UserID = claims.UserID

	level, err := h.inventoryService.ReleaseStock(r.Context(), tenantID, schemaName, &req)
	if err != nil {
		respondError(w, http.StatusBadRequest, fmt.Sprintf("Failed to release stock: %v", err))
		return
	}

	respondJSON(w, http.StatusOK, level)
}

// =============================================================================
// ABSENCE / LEAVE MANAGEMENT HANDLERS
// =============================================================================

// ListAbsenceTypes returns all absence types for a tenant
// @Summary List absence types
// @Description Get all available absence/leave types
// @Tags Leave Management
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Success 200 {array} payroll.AbsenceType
// @Router /tenants/{tenantID}/absence-types [get]
func (h *Handlers) ListAbsenceTypes(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	activeOnly := r.URL.Query().Get("active_only") == "true"

	types, err := h.absenceService.ListAbsenceTypes(r.Context(), schemaName, tenantID, activeOnly)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to list absence types")
		return
	}

	respondJSON(w, http.StatusOK, types)
}

// GetAbsenceType returns a specific absence type
// @Summary Get absence type
// @Description Get a specific absence type by ID
// @Tags Leave Management
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param typeID path string true "Absence Type ID"
// @Success 200 {object} payroll.AbsenceType
// @Router /tenants/{tenantID}/absence-types/{typeID} [get]
func (h *Handlers) GetAbsenceType(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	typeID := chi.URLParam(r, "typeID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	absenceType, err := h.absenceService.GetAbsenceType(r.Context(), schemaName, tenantID, typeID)
	if err != nil {
		respondError(w, http.StatusNotFound, "Absence type not found")
		return
	}

	respondJSON(w, http.StatusOK, absenceType)
}

// ListLeaveBalances returns leave balances for an employee
// @Summary List leave balances
// @Description Get all leave balances for an employee
// @Tags Leave Management
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param employeeID path string true "Employee ID"
// @Success 200 {array} payroll.LeaveBalance
// @Router /tenants/{tenantID}/employees/{employeeID}/leave-balances [get]
func (h *Handlers) ListLeaveBalances(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	employeeID := chi.URLParam(r, "employeeID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	// Default to current year
	year := time.Now().Year()
	if yearParam := r.URL.Query().Get("year"); yearParam != "" {
		if y, err := strconv.Atoi(yearParam); err == nil {
			year = y
		}
	}

	balances, err := h.absenceService.GetLeaveBalances(r.Context(), schemaName, tenantID, employeeID, year)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to list leave balances")
		return
	}

	respondJSON(w, http.StatusOK, balances)
}

// GetLeaveBalancesByYear returns leave balances for an employee for a specific year
// @Summary Get leave balances by year
// @Description Get leave balances for an employee for a specific year
// @Tags Leave Management
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param employeeID path string true "Employee ID"
// @Param year path int true "Year"
// @Success 200 {array} payroll.LeaveBalance
// @Router /tenants/{tenantID}/employees/{employeeID}/leave-balances/{year} [get]
func (h *Handlers) GetLeaveBalancesByYear(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	employeeID := chi.URLParam(r, "employeeID")
	yearStr := chi.URLParam(r, "year")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	year, err := strconv.Atoi(yearStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid year")
		return
	}

	balances, err := h.absenceService.GetLeaveBalances(r.Context(), schemaName, tenantID, employeeID, year)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to list leave balances")
		return
	}

	respondJSON(w, http.StatusOK, balances)
}

// UpdateLeaveBalance updates a leave balance
// @Summary Update leave balance
// @Description Update an employee's leave balance
// @Tags Leave Management
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param employeeID path string true "Employee ID"
// @Param year path int true "Year"
// @Param typeID path string true "Absence Type ID"
// @Param request body payroll.UpdateLeaveBalanceRequest true "Balance update"
// @Success 200 {object} payroll.LeaveBalance
// @Router /tenants/{tenantID}/employees/{employeeID}/leave-balances/{year}/{typeID} [put]
func (h *Handlers) UpdateLeaveBalance(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	employeeID := chi.URLParam(r, "employeeID")
	yearStr := chi.URLParam(r, "year")
	typeID := chi.URLParam(r, "typeID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	year, err := strconv.Atoi(yearStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid year")
		return
	}

	var req payroll.UpdateLeaveBalanceRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	balance, err := h.absenceService.UpdateLeaveBalance(r.Context(), schemaName, tenantID, employeeID, typeID, year, &req)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, balance)
}

// InitializeLeaveBalances initializes leave balances for an employee
// @Summary Initialize leave balances
// @Description Initialize leave balances for an employee for a specific year
// @Tags Leave Management
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param employeeID path string true "Employee ID"
// @Param year path int true "Year"
// @Success 200 {array} payroll.LeaveBalance
// @Router /tenants/{tenantID}/employees/{employeeID}/leave-balances/{year}/initialize [post]
func (h *Handlers) InitializeLeaveBalances(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	employeeID := chi.URLParam(r, "employeeID")
	yearStr := chi.URLParam(r, "year")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	year, err := strconv.Atoi(yearStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid year")
		return
	}

	balances, err := h.absenceService.InitializeEmployeeLeaveBalances(r.Context(), schemaName, tenantID, employeeID, year)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to initialize leave balances")
		return
	}

	respondJSON(w, http.StatusOK, balances)
}

// ListLeaveRecords returns leave records
// @Summary List leave records
// @Description Get leave records for a tenant or employee
// @Tags Leave Management
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param employee_id query string false "Filter by employee ID"
// @Param year query int false "Filter by year"
// @Success 200 {array} payroll.LeaveRecord
// @Router /tenants/{tenantID}/leave-records [get]
func (h *Handlers) ListLeaveRecords(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	employeeID := r.URL.Query().Get("employee_id")
	year := 0
	if yearParam := r.URL.Query().Get("year"); yearParam != "" {
		if y, err := strconv.Atoi(yearParam); err == nil {
			year = y
		}
	}

	records, err := h.absenceService.ListLeaveRecords(r.Context(), schemaName, tenantID, employeeID, year)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to list leave records")
		return
	}

	respondJSON(w, http.StatusOK, records)
}

// CreateLeaveRecord creates a new leave record
// @Summary Create leave record
// @Description Create a new leave/absence request
// @Tags Leave Management
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param request body payroll.CreateLeaveRecordRequest true "Leave request details"
// @Success 201 {object} payroll.LeaveRecord
// @Router /tenants/{tenantID}/leave-records [post]
func (h *Handlers) CreateLeaveRecord(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	claims, _ := auth.GetClaims(r.Context())

	var req payroll.CreateLeaveRecordRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	record, err := h.absenceService.CreateLeaveRecord(r.Context(), schemaName, tenantID, claims.UserID, &req)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, record)
}

// GetLeaveRecord returns a specific leave record
// @Summary Get leave record
// @Description Get a specific leave record by ID
// @Tags Leave Management
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param recordID path string true "Leave Record ID"
// @Success 200 {object} payroll.LeaveRecord
// @Router /tenants/{tenantID}/leave-records/{recordID} [get]
func (h *Handlers) GetLeaveRecord(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	recordID := chi.URLParam(r, "recordID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	record, err := h.absenceService.GetLeaveRecord(r.Context(), schemaName, tenantID, recordID)
	if err != nil {
		respondError(w, http.StatusNotFound, "Leave record not found")
		return
	}

	respondJSON(w, http.StatusOK, record)
}

// ApproveLeaveRecord approves a leave request
// @Summary Approve leave record
// @Description Approve a pending leave request. Absence types marked requires_document require approved leave-record supporting evidence before approval.
// @Tags Leave Management
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param recordID path string true "Leave Record ID"
// @Success 200 {object} payroll.LeaveRecord
// @Failure 409 {object} object{error=string,evidence_policy_results=[]documents.EvidencePolicyResult,remediation_actions=[]documents.DocumentRemediationAction}
// @Router /tenants/{tenantID}/leave-records/{recordID}/approve [post]
func (h *Handlers) ApproveLeaveRecord(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	recordID := chi.URLParam(r, "recordID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	claims, _ := auth.GetClaims(r.Context())

	record, err := h.absenceService.ApproveLeaveRecord(r.Context(), schemaName, tenantID, recordID, claims.UserID)
	if err != nil {
		var leaveEvidenceConflict *payroll.LeaveEvidencePolicyConflictError
		if errors.As(err, &leaveEvidenceConflict) {
			respondEvidencePolicyConflict(w, leaveEvidenceConflict.Error(), leaveEvidenceConflict.Results)
			return
		}
		status := http.StatusBadRequest
		if errors.Is(err, payroll.ErrApprovedLeaveDocumentRequired) {
			status = http.StatusConflict
		}
		respondError(w, status, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, record)
}

// RejectLeaveRecord rejects a leave request
// @Summary Reject leave record
// @Description Reject a pending leave request
// @Tags Leave Management
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param recordID path string true "Leave Record ID"
// @Param request body payroll.RejectLeaveRequest true "Rejection details"
// @Success 200 {object} payroll.LeaveRecord
// @Router /tenants/{tenantID}/leave-records/{recordID}/reject [post]
func (h *Handlers) RejectLeaveRecord(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	recordID := chi.URLParam(r, "recordID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	claims, _ := auth.GetClaims(r.Context())

	var req payroll.RejectLeaveRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	record, err := h.absenceService.RejectLeaveRecord(r.Context(), schemaName, tenantID, recordID, claims.UserID, req.Reason)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, record)
}

// CancelLeaveRecord cancels a leave request
// @Summary Cancel leave record
// @Description Cancel a pending or approved leave request
// @Tags Leave Management
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param recordID path string true "Leave Record ID"
// @Success 200 {object} payroll.LeaveRecord
// @Router /tenants/{tenantID}/leave-records/{recordID}/cancel [post]
func (h *Handlers) CancelLeaveRecord(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	recordID := chi.URLParam(r, "recordID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	claims, _ := auth.GetClaims(r.Context())

	record, err := h.absenceService.CancelLeaveRecord(r.Context(), schemaName, tenantID, recordID, claims.UserID)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, record)
}
