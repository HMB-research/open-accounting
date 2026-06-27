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

	"github.com/HMB-research/open-accounting/internal/auth"
	"github.com/HMB-research/open-accounting/internal/contacts"
	"github.com/HMB-research/open-accounting/internal/email"
	"github.com/HMB-research/open-accounting/internal/inventory"
	"github.com/HMB-research/open-accounting/internal/invoicing"
	"github.com/HMB-research/open-accounting/internal/orders"
	"github.com/HMB-research/open-accounting/internal/payments"
	"github.com/HMB-research/open-accounting/internal/pdf"
	"github.com/HMB-research/open-accounting/internal/quotes"
	"github.com/HMB-research/open-accounting/internal/tenant"
)

func wave5Claims() *auth.Claims {
	return createTestClaims("user-1", "test@example.com", "tenant-1", "owner")
}

func wave5Request(method, path string, body any, params map[string]string) *http.Request {
	req := makeAuthenticatedRequest(method, path, body, wave5Claims())
	return withURLParams(req, params)
}

func wave5CommercialRawRequest(method, path, body string, params map[string]string) *http.Request {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = withURLParams(req, params)
	return req.WithContext(contextWithClaims(req.Context(), wave5Claims()))
}

func wave5AddTenant(repo *mockTenantRepository) {
	repo.tenants["tenant-1"] = &tenant.Tenant{
		ID:         "tenant-1",
		Name:       "Test Tenant",
		SchemaName: "tenant_test",
	}
}

func wave5Quote(status quotes.QuoteStatus) *quotes.Quote {
	q := &quotes.Quote{
		ID:           "quote-1",
		TenantID:     "tenant-1",
		QuoteNumber:  "QT-001",
		ContactID:    "contact-1",
		Contact:      &contacts.Contact{Name: "Acme OU", Email: "billing@example.com"},
		QuoteDate:    time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC),
		Status:       status,
		Currency:     "EUR",
		ExchangeRate: decimal.NewFromInt(1),
		Notes:        "Quote notes",
		Lines: []quotes.QuoteLine{{
			ID:          "line-1",
			TenantID:    "tenant-1",
			QuoteID:     "quote-1",
			LineNumber:  1,
			Description: "Consulting",
			Quantity:    decimal.NewFromInt(2),
			Unit:        "hour",
			UnitPrice:   decimal.NewFromInt(100),
			VATRate:     decimal.NewFromInt(22),
		}},
	}
	q.Calculate()
	return q
}

func wave5Order(status orders.OrderStatus) *orders.Order {
	o := &orders.Order{
		ID:           "order-1",
		TenantID:     "tenant-1",
		OrderNumber:  "ORD-001",
		ContactID:    "contact-1",
		Contact:      &contacts.Contact{Name: "Acme OU", Email: "billing@example.com"},
		OrderDate:    time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC),
		Status:       status,
		Currency:     "EUR",
		ExchangeRate: decimal.NewFromInt(1),
		Notes:        "Order notes",
		Lines: []orders.OrderLine{{
			ID:          "line-1",
			TenantID:    "tenant-1",
			OrderID:     "order-1",
			LineNumber:  1,
			Description: "Implementation",
			Quantity:    decimal.NewFromInt(3),
			Unit:        "hour",
			UnitPrice:   decimal.NewFromInt(120),
			VATRate:     decimal.NewFromInt(22),
		}},
	}
	o.Calculate()
	return o
}

func wave5QuoteLine() quotes.CreateQuoteLineRequest {
	return quotes.CreateQuoteLineRequest{
		Description: "Consulting",
		Quantity:    decimal.NewFromInt(1),
		UnitPrice:   decimal.NewFromInt(100),
		VATRate:     decimal.NewFromInt(22),
	}
}

func wave5OrderLine() orders.CreateOrderLineRequest {
	return orders.CreateOrderLineRequest{
		Description: "Implementation",
		Quantity:    decimal.NewFromInt(1),
		UnitPrice:   decimal.NewFromInt(100),
		VATRate:     decimal.NewFromInt(22),
	}
}

func TestBusinessCommercialWave5InvoicePDFAndSEPAPaymentBranches(t *testing.T) {
	t.Run("invoice pdf missing invoice", func(t *testing.T) {
		h, tenantRepo, _ := setupInvoiceTestHandlers()
		h.pdfService = pdf.NewService()
		wave5AddTenant(tenantRepo)

		rr := httptest.NewRecorder()
		h.GetInvoicePDF(rr, wave5Request(http.MethodGet, "/tenants/tenant-1/invoices/missing/pdf", nil, map[string]string{
			"tenantID":  "tenant-1",
			"invoiceID": "missing",
		}))

		assert.Equal(t, http.StatusNotFound, rr.Code)
		assert.Contains(t, rr.Body.String(), "Invoice not found")
	})

	t.Run("invoice pdf tenant lookup failure", func(t *testing.T) {
		h, _, invoiceRepo := setupInvoiceTestHandlers()
		h.pdfService = pdf.NewService()
		invoiceRepo.addTestInvoice("inv-1", "tenant-1", "contact-1", invoicing.InvoiceTypeSales, invoicing.StatusDraft)

		rr := httptest.NewRecorder()
		h.GetInvoicePDF(rr, wave5Request(http.MethodGet, "/tenants/tenant-1/invoices/inv-1/pdf", nil, map[string]string{
			"tenantID":  "tenant-1",
			"invoiceID": "inv-1",
		}))

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
		assert.Contains(t, rr.Body.String(), "Failed to get tenant")
	})

	t.Run("sepa export rejects invalid json", func(t *testing.T) {
		h := &Handlers{}
		rr := httptest.NewRecorder()

		h.ExportSEPAPayments(rr, wave5CommercialRawRequest(http.MethodPost, "/tenants/tenant-1/payments/sepa-export", "{", map[string]string{"tenantID": "tenant-1"}))

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "Invalid request body")
	})

	t.Run("sepa export returns xml", func(t *testing.T) {
		h := &Handlers{}
		rr := httptest.NewRecorder()

		h.ExportSEPAPayments(rr, wave5Request(http.MethodPost, "/tenants/tenant-1/payments/sepa-export", payments.SEPAExportRequest{
			MessageID:     "MSG-1",
			PaymentInfoID: "PMT-1",
			DebtorName:    "Example OU",
			DebtorIBAN:    "DE89370400440532013000",
			ExecutionDate: "2026-04-01",
			Lines: []payments.SEPACreditTransferLine{{
				CreditorName: "Vendor OU",
				CreditorIBAN: "DE89370400440532013000",
				Amount:       decimal.NewFromInt(125),
			}},
		}, map[string]string{"tenantID": "tenant-1"}))

		require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
		assert.Contains(t, rr.Header().Get("Content-Type"), "application/xml")
		assert.Contains(t, rr.Body.String(), "<MsgId>MSG-1</MsgId>")
	})
}

func TestBusinessCommercialWave5ReversePaymentBranches(t *testing.T) {
	t.Run("invalid json", func(t *testing.T) {
		h, _, tenantRepo := setupPaymentTestHandlers()
		wave5AddTenant(tenantRepo)

		rr := httptest.NewRecorder()
		h.ReversePayment(rr, wave5CommercialRawRequest(http.MethodPost, "/tenants/tenant-1/payments/payment-1/reverse", "{", map[string]string{
			"tenantID":  "tenant-1",
			"paymentID": "payment-1",
		}))

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "Invalid request body")
	})

	t.Run("defaults reversal date and maps not found", func(t *testing.T) {
		h, repo, tenantRepo := setupPaymentTestHandlers()
		wave5AddTenant(tenantRepo)
		repo.payments["payment-1"] = &payments.Payment{
			ID:            "payment-1",
			TenantID:      "tenant-1",
			PaymentNumber: "PMT-00001",
			PaymentType:   payments.PaymentTypeReceived,
			PaymentDate:   time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC),
			Amount:        decimal.NewFromInt(50),
			Currency:      "EUR",
			ExchangeRate:  decimal.NewFromInt(1),
			BaseAmount:    decimal.NewFromInt(50),
		}

		rr := httptest.NewRecorder()
		h.ReversePayment(rr, wave5Request(http.MethodPost, "/tenants/tenant-1/payments/payment-1/reverse", payments.ReversePaymentRequest{
			Reason: "Duplicate",
		}, map[string]string{"tenantID": "tenant-1", "paymentID": "payment-1"}))

		require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())
		var result payments.PaymentReversalResult
		require.NoError(t, json.NewDecoder(rr.Body).Decode(&result))
		assert.False(t, result.ReversalPayment.PaymentDate.IsZero())

		rr = httptest.NewRecorder()
		h.ReversePayment(rr, wave5Request(http.MethodPost, "/tenants/tenant-1/payments/missing/reverse", payments.ReversePaymentRequest{
			Reason: "Duplicate",
		}, map[string]string{"tenantID": "tenant-1", "paymentID": "missing"}))

		assert.Equal(t, http.StatusNotFound, rr.Code)
		assert.Contains(t, rr.Body.String(), "Payment not found")
	})

	t.Run("maps reversal payment conflict", func(t *testing.T) {
		h, repo, tenantRepo := setupPaymentTestHandlers()
		wave5AddTenant(tenantRepo)
		originalID := "original-payment"
		repo.payments["payment-1"] = &payments.Payment{
			ID:                  "payment-1",
			TenantID:            "tenant-1",
			PaymentNumber:       "PMT-00002",
			PaymentType:         payments.PaymentTypeMade,
			Amount:              decimal.NewFromInt(75),
			Currency:            "EUR",
			ExchangeRate:        decimal.NewFromInt(1),
			BaseAmount:          decimal.NewFromInt(75),
			ReversalOfPaymentID: &originalID,
		}

		rr := httptest.NewRecorder()
		h.ReversePayment(rr, wave5Request(http.MethodPost, "/tenants/tenant-1/payments/payment-1/reverse", payments.ReversePaymentRequest{
			Reason: "Duplicate",
		}, map[string]string{"tenantID": "tenant-1", "paymentID": "payment-1"}))

		assert.Equal(t, http.StatusConflict, rr.Code)
		assert.Contains(t, rr.Body.String(), "reversal payments cannot be reversed")
	})
}

func TestBusinessCommercialWave5SMTPEmailConfigTemplateLogBranches(t *testing.T) {
	t.Run("nil email service errors", func(t *testing.T) {
		h, _, _ := setupEmailHandlers()
		h.emailService = nil

		cases := []struct {
			name   string
			invoke func(http.ResponseWriter, *http.Request)
			req    *http.Request
			status int
		}{
			{
				name:   "get smtp",
				invoke: h.GetSMTPConfig,
				req:    wave5Request(http.MethodGet, "/tenants/tenant-1/settings/smtp", nil, map[string]string{"tenantID": "tenant-1"}),
				status: http.StatusInternalServerError,
			},
			{
				name:   "update smtp",
				invoke: h.UpdateSMTPConfig,
				req: wave5Request(http.MethodPut, "/tenants/tenant-1/settings/smtp", email.UpdateSMTPConfigRequest{
					Host: "smtp.example.com", Port: 587, FromEmail: "billing@example.com",
				}, map[string]string{"tenantID": "tenant-1"}),
				status: http.StatusBadRequest,
			},
			{
				name:   "list templates",
				invoke: h.ListEmailTemplates,
				req:    wave5Request(http.MethodGet, "/tenants/tenant-1/email-templates", nil, map[string]string{"tenantID": "tenant-1"}),
				status: http.StatusInternalServerError,
			},
			{
				name:   "update template",
				invoke: h.UpdateEmailTemplate,
				req: wave5Request(http.MethodPut, "/tenants/tenant-1/email-templates/QUOTE_SEND", email.UpdateTemplateRequest{
					Subject: "Quote", BodyHTML: "<p>Quote</p>", BodyText: "Quote", IsActive: true,
				}, map[string]string{"tenantID": "tenant-1", "templateType": string(email.TemplateQuoteSend)}),
				status: http.StatusBadRequest,
			},
			{
				name:   "email log",
				invoke: h.GetEmailLog,
				req:    wave5Request(http.MethodGet, "/tenants/tenant-1/email-log?limit=bad", nil, map[string]string{"tenantID": "tenant-1"}),
				status: http.StatusInternalServerError,
			},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				rr := httptest.NewRecorder()
				tc.invoke(rr, tc.req)
				assert.Equal(t, tc.status, rr.Code, rr.Body.String())
			})
		}
	})

	t.Run("invalid json and smtp test recipient validation", func(t *testing.T) {
		h, _, _ := setupEmailHandlers()

		rr := httptest.NewRecorder()
		h.UpdateSMTPConfig(rr, wave5CommercialRawRequest(http.MethodPut, "/tenants/tenant-1/settings/smtp", "{", map[string]string{"tenantID": "tenant-1"}))
		assert.Equal(t, http.StatusBadRequest, rr.Code)

		rr = httptest.NewRecorder()
		h.TestSMTP(rr, wave5CommercialRawRequest(http.MethodPost, "/tenants/tenant-1/settings/smtp/test", "{", map[string]string{"tenantID": "tenant-1"}))
		assert.Equal(t, http.StatusBadRequest, rr.Code)

		rr = httptest.NewRecorder()
		h.TestSMTP(rr, wave5Request(http.MethodPost, "/tenants/tenant-1/settings/smtp/test", email.TestSMTPRequest{}, map[string]string{"tenantID": "tenant-1"}))
		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "Recipient email is required")
	})

	t.Run("repository errors", func(t *testing.T) {
		repo := newWave5EmailRepository()
		repo.getSettingsErr = errors.New("settings down")
		h := &Handlers{emailService: email.NewServiceWithRepository(repo, &emailHandlerMailer{})}

		rr := httptest.NewRecorder()
		h.GetSMTPConfig(rr, wave5Request(http.MethodGet, "/tenants/tenant-1/settings/smtp", nil, map[string]string{"tenantID": "tenant-1"}))
		assert.Equal(t, http.StatusInternalServerError, rr.Code)

		repo = newWave5EmailRepository()
		repo.updateSettingsErr = errors.New("update down")
		h.emailService = email.NewServiceWithRepository(repo, &emailHandlerMailer{})
		rr = httptest.NewRecorder()
		h.UpdateSMTPConfig(rr, wave5Request(http.MethodPut, "/tenants/tenant-1/settings/smtp", email.UpdateSMTPConfigRequest{
			Host: "smtp.example.com", Port: 587, FromEmail: "billing@example.com",
		}, map[string]string{"tenantID": "tenant-1"}))
		assert.Equal(t, http.StatusBadRequest, rr.Code)

		repo = newWave5EmailRepository()
		repo.listTemplatesErr = errors.New("templates down")
		h.emailService = email.NewServiceWithRepository(repo, &emailHandlerMailer{})
		rr = httptest.NewRecorder()
		h.ListEmailTemplates(rr, wave5Request(http.MethodGet, "/tenants/tenant-1/email-templates", nil, map[string]string{"tenantID": "tenant-1"}))
		assert.Equal(t, http.StatusInternalServerError, rr.Code)

		repo = newWave5EmailRepository()
		repo.upsertTemplateErr = errors.New("upsert down")
		h.emailService = email.NewServiceWithRepository(repo, &emailHandlerMailer{})
		rr = httptest.NewRecorder()
		h.UpdateEmailTemplate(rr, wave5Request(http.MethodPut, "/tenants/tenant-1/email-templates/QUOTE_SEND", email.UpdateTemplateRequest{
			Subject: "Quote", BodyHTML: "<p>Quote</p>", IsActive: true,
		}, map[string]string{"tenantID": "tenant-1", "templateType": string(email.TemplateQuoteSend)}))
		assert.Equal(t, http.StatusBadRequest, rr.Code)

		repo = newWave5EmailRepository()
		repo.getEmailLogErr = errors.New("log down")
		h.emailService = email.NewServiceWithRepository(repo, &emailHandlerMailer{})
		rr = httptest.NewRecorder()
		h.GetEmailLog(rr, wave5Request(http.MethodGet, "/tenants/tenant-1/email-log?limit=2", nil, map[string]string{"tenantID": "tenant-1"}))
		assert.Equal(t, http.StatusInternalServerError, rr.Code)
	})
}

func TestBusinessCommercialWave5EmailQuoteBranches(t *testing.T) {
	t.Run("request validation and lookup", func(t *testing.T) {
		h, repo, tenantRepo := setupQuotesTestHandlers()
		wave5AddTenant(tenantRepo)
		repo.quotes["quote-1"] = wave5Quote(quotes.QuoteStatusDraft)

		cases := []struct {
			name   string
			req    *http.Request
			status int
			body   string
		}{
			{
				name:   "invalid json",
				req:    wave5CommercialRawRequest(http.MethodPost, "/tenants/tenant-1/quotes/quote-1/email", "{", map[string]string{"tenantID": "tenant-1", "quoteID": "quote-1"}),
				status: http.StatusBadRequest,
				body:   "Invalid request body",
			},
			{
				name:   "recipient required",
				req:    wave5Request(http.MethodPost, "/tenants/tenant-1/quotes/quote-1/email", email.SendQuoteRequest{}, map[string]string{"tenantID": "tenant-1", "quoteID": "quote-1"}),
				status: http.StatusBadRequest,
				body:   "recipient email is required",
			},
			{
				name:   "quote missing",
				req:    wave5Request(http.MethodPost, "/tenants/tenant-1/quotes/missing/email", email.SendQuoteRequest{RecipientEmail: "billing@example.com"}, map[string]string{"tenantID": "tenant-1", "quoteID": "missing"}),
				status: http.StatusNotFound,
				body:   "Quote not found",
			},
			{
				name:   "evidence service missing",
				req:    wave5Request(http.MethodPost, "/tenants/tenant-1/quotes/quote-1/email", email.SendQuoteRequest{RecipientEmail: "billing@example.com", RequireApprovedEvidence: true}, map[string]string{"tenantID": "tenant-1", "quoteID": "quote-1"}),
				status: http.StatusConflict,
				body:   "approved quote evidence is required",
			},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				rr := httptest.NewRecorder()
				h.EmailQuote(rr, tc.req)
				assert.Equal(t, tc.status, rr.Code, rr.Body.String())
				assert.Contains(t, rr.Body.String(), tc.body)
			})
		}
	})

	t.Run("tenant template render and send failures", func(t *testing.T) {
		cases := []struct {
			name     string
			setup    func(*Handlers, *mockQuotesRepository, *mockTenantRepository, *wave5EmailRepository)
			wantCode int
			wantBody string
		}{
			{
				name: "tenant missing",
				setup: func(h *Handlers, repo *mockQuotesRepository, _ *mockTenantRepository, emailRepo *wave5EmailRepository) {
					repo.quotes["quote-1"] = wave5Quote(quotes.QuoteStatusSent)
					emailRepo.templates[emailTemplateKey("tenant-1", email.TemplateQuoteSend)] = email.EmailTemplate{
						TenantID: "tenant-1", TemplateType: email.TemplateQuoteSend, Subject: "Quote", BodyHTML: "<p>Quote</p>", IsActive: true,
					}
				},
				wantCode: http.StatusInternalServerError,
				wantBody: "Failed to get tenant",
			},
			{
				name: "template error",
				setup: func(_ *Handlers, repo *mockQuotesRepository, tenantRepo *mockTenantRepository, emailRepo *wave5EmailRepository) {
					wave5AddTenant(tenantRepo)
					repo.quotes["quote-1"] = wave5Quote(quotes.QuoteStatusSent)
					emailRepo.getTemplateErr = errors.New("template down")
				},
				wantCode: http.StatusInternalServerError,
				wantBody: "Failed to get email template",
			},
			{
				name: "render error",
				setup: func(_ *Handlers, repo *mockQuotesRepository, tenantRepo *mockTenantRepository, emailRepo *wave5EmailRepository) {
					wave5AddTenant(tenantRepo)
					repo.quotes["quote-1"] = wave5Quote(quotes.QuoteStatusSent)
					emailRepo.templates[emailTemplateKey("tenant-1", email.TemplateQuoteSend)] = email.EmailTemplate{
						TenantID: "tenant-1", TemplateType: email.TemplateQuoteSend, Subject: "{{", BodyHTML: "<p>Quote</p>", IsActive: true,
					}
				},
				wantCode: http.StatusInternalServerError,
				wantBody: "Failed to render email template",
			},
			{
				name: "send error",
				setup: func(_ *Handlers, repo *mockQuotesRepository, tenantRepo *mockTenantRepository, emailRepo *wave5EmailRepository) {
					wave5AddTenant(tenantRepo)
					repo.quotes["quote-1"] = wave5Quote(quotes.QuoteStatusSent)
					delete(emailRepo.settings, "tenant-1")
					emailRepo.templates[emailTemplateKey("tenant-1", email.TemplateQuoteSend)] = email.EmailTemplate{
						TenantID: "tenant-1", TemplateType: email.TemplateQuoteSend, Subject: "Quote", BodyHTML: "<p>Quote</p>", IsActive: true,
					}
				},
				wantCode: http.StatusBadRequest,
				wantBody: "SMTP is not configured",
			},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				h, repo, tenantRepo := setupQuotesTestHandlers()
				h.pdfService = pdf.NewService()
				emailRepo := newWave5EmailRepository()
				h.emailService = email.NewServiceWithRepository(emailRepo, &emailHandlerMailer{})
				tc.setup(h, repo, tenantRepo, emailRepo)

				rr := httptest.NewRecorder()
				h.EmailQuote(rr, wave5Request(http.MethodPost, "/tenants/tenant-1/quotes/quote-1/email", email.SendQuoteRequest{
					RecipientEmail: "billing@example.com",
					RecipientName:  "Acme",
				}, map[string]string{"tenantID": "tenant-1", "quoteID": "quote-1"}))

				assert.Equal(t, tc.wantCode, rr.Code, rr.Body.String())
				assert.Contains(t, rr.Body.String(), tc.wantBody)
			})
		}
	})
}

func TestBusinessCommercialWave5EmailOrderBranches(t *testing.T) {
	t.Run("request validation and lookup", func(t *testing.T) {
		h, repo, tenantRepo := setupOrdersTestHandlers()
		wave5AddTenant(tenantRepo)
		repo.orders["order-1"] = wave5Order(orders.OrderStatusPending)

		cases := []struct {
			name   string
			req    *http.Request
			status int
			body   string
		}{
			{
				name:   "invalid json",
				req:    wave5CommercialRawRequest(http.MethodPost, "/tenants/tenant-1/orders/order-1/email", "{", map[string]string{"tenantID": "tenant-1", "orderID": "order-1"}),
				status: http.StatusBadRequest,
				body:   "Invalid request body",
			},
			{
				name:   "recipient required",
				req:    wave5Request(http.MethodPost, "/tenants/tenant-1/orders/order-1/email", email.SendOrderRequest{}, map[string]string{"tenantID": "tenant-1", "orderID": "order-1"}),
				status: http.StatusBadRequest,
				body:   "recipient email is required",
			},
			{
				name:   "order missing",
				req:    wave5Request(http.MethodPost, "/tenants/tenant-1/orders/missing/email", email.SendOrderRequest{RecipientEmail: "billing@example.com"}, map[string]string{"tenantID": "tenant-1", "orderID": "missing"}),
				status: http.StatusNotFound,
				body:   "Order not found",
			},
			{
				name:   "evidence service missing",
				req:    wave5Request(http.MethodPost, "/tenants/tenant-1/orders/order-1/email", email.SendOrderRequest{RecipientEmail: "billing@example.com", RequireApprovedEvidence: true}, map[string]string{"tenantID": "tenant-1", "orderID": "order-1"}),
				status: http.StatusConflict,
				body:   "approved order evidence is required",
			},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				rr := httptest.NewRecorder()
				h.EmailOrder(rr, tc.req)
				assert.Equal(t, tc.status, rr.Code, rr.Body.String())
				assert.Contains(t, rr.Body.String(), tc.body)
			})
		}
	})

	t.Run("tenant template render and send failures", func(t *testing.T) {
		cases := []struct {
			name     string
			setup    func(*mockOrdersRepository, *mockTenantRepository, *wave5EmailRepository)
			wantCode int
			wantBody string
		}{
			{
				name: "tenant missing",
				setup: func(repo *mockOrdersRepository, _ *mockTenantRepository, emailRepo *wave5EmailRepository) {
					repo.orders["order-1"] = wave5Order(orders.OrderStatusConfirmed)
					emailRepo.templates[emailTemplateKey("tenant-1", email.TemplateOrderConfirm)] = email.EmailTemplate{
						TenantID: "tenant-1", TemplateType: email.TemplateOrderConfirm, Subject: "Order", BodyHTML: "<p>Order</p>", IsActive: true,
					}
				},
				wantCode: http.StatusInternalServerError,
				wantBody: "Failed to get tenant",
			},
			{
				name: "template error",
				setup: func(repo *mockOrdersRepository, tenantRepo *mockTenantRepository, emailRepo *wave5EmailRepository) {
					wave5AddTenant(tenantRepo)
					repo.orders["order-1"] = wave5Order(orders.OrderStatusConfirmed)
					emailRepo.getTemplateErr = errors.New("template down")
				},
				wantCode: http.StatusInternalServerError,
				wantBody: "Failed to get email template",
			},
			{
				name: "render error",
				setup: func(repo *mockOrdersRepository, tenantRepo *mockTenantRepository, emailRepo *wave5EmailRepository) {
					wave5AddTenant(tenantRepo)
					repo.orders["order-1"] = wave5Order(orders.OrderStatusConfirmed)
					emailRepo.templates[emailTemplateKey("tenant-1", email.TemplateOrderConfirm)] = email.EmailTemplate{
						TenantID: "tenant-1", TemplateType: email.TemplateOrderConfirm, Subject: "{{", BodyHTML: "<p>Order</p>", IsActive: true,
					}
				},
				wantCode: http.StatusInternalServerError,
				wantBody: "Failed to render email template",
			},
			{
				name: "send error",
				setup: func(repo *mockOrdersRepository, tenantRepo *mockTenantRepository, emailRepo *wave5EmailRepository) {
					wave5AddTenant(tenantRepo)
					repo.orders["order-1"] = wave5Order(orders.OrderStatusConfirmed)
					delete(emailRepo.settings, "tenant-1")
					emailRepo.templates[emailTemplateKey("tenant-1", email.TemplateOrderConfirm)] = email.EmailTemplate{
						TenantID: "tenant-1", TemplateType: email.TemplateOrderConfirm, Subject: "Order", BodyHTML: "<p>Order</p>", IsActive: true,
					}
				},
				wantCode: http.StatusBadRequest,
				wantBody: "SMTP is not configured",
			},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				h, repo, tenantRepo := setupOrdersTestHandlers()
				h.pdfService = pdf.NewService()
				emailRepo := newWave5EmailRepository()
				h.emailService = email.NewServiceWithRepository(emailRepo, &emailHandlerMailer{})
				tc.setup(repo, tenantRepo, emailRepo)

				rr := httptest.NewRecorder()
				h.EmailOrder(rr, wave5Request(http.MethodPost, "/tenants/tenant-1/orders/order-1/email", email.SendOrderRequest{
					RecipientEmail: "billing@example.com",
					RecipientName:  "Acme",
				}, map[string]string{"tenantID": "tenant-1", "orderID": "order-1"}))

				assert.Equal(t, tc.wantCode, rr.Code, rr.Body.String())
				assert.Contains(t, rr.Body.String(), tc.wantBody)
			})
		}
	})
}

func TestBusinessCommercialWave5QuoteHandlerBranches(t *testing.T) {
	t.Run("list filters and service error", func(t *testing.T) {
		h, repo, tenantRepo := setupQuotesTestHandlers()
		wave5AddTenant(tenantRepo)
		repo.quotes["quote-1"] = wave5Quote(quotes.QuoteStatusAccepted)

		rr := httptest.NewRecorder()
		h.ListQuotes(rr, wave5Request(http.MethodGet, "/tenants/tenant-1/quotes?status=ACCEPTED&contact_id=contact-1&from_date=2026-03-01&to_date=2026-03-31&search=QT", nil, map[string]string{"tenantID": "tenant-1"}))
		require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
		var listed []quotes.Quote
		require.NoError(t, json.NewDecoder(rr.Body).Decode(&listed))
		assert.Len(t, listed, 1)

		repo.listErr = errors.New("list down")
		rr = httptest.NewRecorder()
		h.ListQuotes(rr, wave5Request(http.MethodGet, "/tenants/tenant-1/quotes", nil, map[string]string{"tenantID": "tenant-1"}))
		assert.Equal(t, http.StatusInternalServerError, rr.Code)
		assert.Contains(t, rr.Body.String(), "Failed to list quotes")
	})

	t.Run("create validation and service error", func(t *testing.T) {
		h, repo, tenantRepo := setupQuotesTestHandlers()
		wave5AddTenant(tenantRepo)

		cases := []struct {
			name string
			req  *http.Request
			body string
		}{
			{"invalid json", wave5CommercialRawRequest(http.MethodPost, "/tenants/tenant-1/quotes", "{", map[string]string{"tenantID": "tenant-1"}), "Invalid request body"},
			{"contact required", wave5Request(http.MethodPost, "/tenants/tenant-1/quotes", quotes.CreateQuoteRequest{Lines: []quotes.CreateQuoteLineRequest{wave5QuoteLine()}}, map[string]string{"tenantID": "tenant-1"}), "Contact is required"},
			{"line required", wave5Request(http.MethodPost, "/tenants/tenant-1/quotes", quotes.CreateQuoteRequest{ContactID: "contact-1"}, map[string]string{"tenantID": "tenant-1"}), "At least one line is required"},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				rr := httptest.NewRecorder()
				h.CreateQuote(rr, tc.req)
				assert.Equal(t, http.StatusBadRequest, rr.Code)
				assert.Contains(t, rr.Body.String(), tc.body)
			})
		}

		repo.createErr = errors.New("create down")
		rr := httptest.NewRecorder()
		h.CreateQuote(rr, wave5Request(http.MethodPost, "/tenants/tenant-1/quotes", quotes.CreateQuoteRequest{
			ContactID: "contact-1",
			Lines:     []quotes.CreateQuoteLineRequest{wave5QuoteLine()},
		}, map[string]string{"tenantID": "tenant-1"}))
		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "create down")
	})

	t.Run("import validation dependencies and default filename", func(t *testing.T) {
		h, repo, tenantRepo, contactsRepo := setupQuotesImportTestHandlers()
		wave5AddTenant(tenantRepo)

		rr := httptest.NewRecorder()
		h.ImportQuotes(rr, wave5CommercialRawRequest(http.MethodPost, "/tenants/tenant-1/quotes/import", "{", map[string]string{"tenantID": "tenant-1"}))
		assert.Equal(t, http.StatusBadRequest, rr.Code)

		rr = httptest.NewRecorder()
		h.ImportQuotes(rr, wave5Request(http.MethodPost, "/tenants/tenant-1/quotes/import", quotes.ImportQuotesRequest{}, map[string]string{"tenantID": "tenant-1"}))
		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "csv_content is required")

		contactsRepo.listErr = errors.New("contacts down")
		rr = httptest.NewRecorder()
		h.ImportQuotes(rr, wave5Request(http.MethodPost, "/tenants/tenant-1/quotes/import", quotes.ImportQuotesRequest{CSVContent: "quote_number,contact_id,quote_date,line_description,quantity,unit_price,vat_rate\nQT-1,contact-1,2026-03-01,Work,1,100,22"}, map[string]string{"tenantID": "tenant-1"}))
		assert.Equal(t, http.StatusInternalServerError, rr.Code)
		assert.Contains(t, rr.Body.String(), "Failed to load contacts")
		contactsRepo.listErr = nil

		inventoryRepo := newMockInventoryRepository()
		inventoryRepo.listProductsErr = errors.New("products down")
		h.inventoryService = inventory.NewServiceWithRepository(inventoryRepo)
		rr = httptest.NewRecorder()
		h.ImportQuotes(rr, wave5Request(http.MethodPost, "/tenants/tenant-1/quotes/import", quotes.ImportQuotesRequest{CSVContent: "quote_number,contact_id,quote_date,line_description,quantity,unit_price,vat_rate\nQT-1,contact-1,2026-03-01,Work,1,100,22"}, map[string]string{"tenantID": "tenant-1"}))
		assert.Equal(t, http.StatusInternalServerError, rr.Code)
		assert.Contains(t, rr.Body.String(), "Failed to load products")
		h.inventoryService = nil

		repo.listErr = errors.New("quotes down")
		rr = httptest.NewRecorder()
		h.ImportQuotes(rr, wave5Request(http.MethodPost, "/tenants/tenant-1/quotes/import", quotes.ImportQuotesRequest{CSVContent: "quote_number,contact_id,quote_date,line_description,quantity,unit_price,vat_rate\nQT-1,contact-1,2026-03-01,Work,1,100,22"}, map[string]string{"tenantID": "tenant-1"}))
		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "quotes down")
		repo.listErr = nil

		contactsRepo.addTestContact("contact-1", "tenant-1", "Acme OU", contacts.ContactTypeCustomer, true)
		rr = httptest.NewRecorder()
		h.ImportQuotes(rr, wave5Request(http.MethodPost, "/tenants/tenant-1/quotes/import", quotes.ImportQuotesRequest{CSVContent: "quote_number,contact_id,quote_date,line_description,quantity,unit_price,vat_rate\nQT-1,contact-1,2026-03-01,Work,1,100,22"}, map[string]string{"tenantID": "tenant-1"}))
		require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
		var result quotes.ImportQuotesResult
		require.NoError(t, json.NewDecoder(rr.Body).Decode(&result))
		assert.Equal(t, "quotes_import.csv", result.FileName)
	})

	t.Run("pdf and status errors", func(t *testing.T) {
		h, repo, tenantRepo := setupQuotesTestHandlers()
		h.pdfService = pdf.NewService()
		repo.quotes["quote-1"] = wave5Quote(quotes.QuoteStatusDraft)

		rr := httptest.NewRecorder()
		h.GetQuotePDF(rr, wave5Request(http.MethodGet, "/tenants/tenant-1/quotes/missing/pdf", nil, map[string]string{"tenantID": "tenant-1", "quoteID": "missing"}))
		assert.Equal(t, http.StatusNotFound, rr.Code)

		rr = httptest.NewRecorder()
		h.GetQuotePDF(rr, wave5Request(http.MethodGet, "/tenants/tenant-1/quotes/quote-1/pdf", nil, map[string]string{"tenantID": "tenant-1", "quoteID": "quote-1"}))
		assert.Equal(t, http.StatusInternalServerError, rr.Code)
		assert.Contains(t, rr.Body.String(), "Failed to get tenant")

		wave5AddTenant(tenantRepo)
		repo.updateStatusErr = errors.New("status down")
		for _, tc := range []struct {
			name   string
			invoke func(http.ResponseWriter, *http.Request)
		}{
			{"send", h.SendQuote},
			{"accept", h.AcceptQuote},
			{"reject", h.RejectQuote},
		} {
			t.Run(tc.name, func(t *testing.T) {
				rr := httptest.NewRecorder()
				tc.invoke(rr, wave5Request(http.MethodPost, "/tenants/tenant-1/quotes/quote-1/"+tc.name, nil, map[string]string{"tenantID": "tenant-1", "quoteID": "quote-1"}))
				assert.Equal(t, http.StatusBadRequest, rr.Code)
				assert.Contains(t, rr.Body.String(), "status down")
			})
		}

		rr = httptest.NewRecorder()
		h.SendQuote(rr, wave5CommercialRawRequest(http.MethodPost, "/tenants/tenant-1/quotes/quote-1/send", "{", map[string]string{"tenantID": "tenant-1", "quoteID": "quote-1"}))
		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})
}

func TestBusinessCommercialWave5QuoteConvertBranches(t *testing.T) {
	t.Run("defaults dates and quote notes", func(t *testing.T) {
		h, repo, tenantRepo := setupQuotesTestHandlers()
		invoiceRepo := newMockInvoicingRepository()
		h.invoicingService = invoicing.NewServiceWithRepository(invoiceRepo, nil)
		wave5AddTenant(tenantRepo)
		repo.quotes["quote-1"] = wave5Quote(quotes.QuoteStatusAccepted)

		rr := httptest.NewRecorder()
		h.ConvertQuoteToInvoice(rr, wave5Request(http.MethodPost, "/tenants/tenant-1/quotes/quote-1/convert-to-invoice", quotes.ConvertQuoteToInvoiceRequest{}, map[string]string{"tenantID": "tenant-1", "quoteID": "quote-1"}))

		require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())
		var result quotes.QuoteInvoiceConversionResult
		require.NoError(t, json.NewDecoder(rr.Body).Decode(&result))
		assert.Equal(t, "Quote notes", result.Invoice.Notes)
		assert.Equal(t, result.Invoice.IssueDate.AddDate(0, 0, 14).Format("2006-01-02"), result.Invoice.DueDate.Format("2006-01-02"))
	})

	t.Run("validation and service errors", func(t *testing.T) {
		tests := []struct {
			name     string
			body     string
			setup    func(*Handlers, *mockQuotesRepository, *mockTenantRepository)
			wantCode int
			wantBody string
		}{
			{"invalid json", "{", nil, http.StatusBadRequest, "Invalid request body"},
			{"due date before issue", `{"issue_date":"2026-03-10T00:00:00Z","due_date":"2026-03-09T00:00:00Z"}`, nil, http.StatusBadRequest, "due date cannot be before issue date"},
			{"quote missing", `{}`, nil, http.StatusNotFound, "Quote not found"},
			{"not accepted", `{}`, func(_ *Handlers, repo *mockQuotesRepository, _ *mockTenantRepository) {
				repo.quotes["quote-1"] = wave5Quote(quotes.QuoteStatusSent)
			}, http.StatusConflict, "quote must be accepted"},
			{"already converted", `{}`, func(_ *Handlers, repo *mockQuotesRepository, _ *mockTenantRepository) {
				invoiceID := "invoice-1"
				q := wave5Quote(quotes.QuoteStatusAccepted)
				q.ConvertedToInvoiceID = &invoiceID
				repo.quotes["quote-1"] = q
			}, http.StatusConflict, "already been converted"},
			{"invoice create error", `{}`, func(h *Handlers, repo *mockQuotesRepository, _ *mockTenantRepository) {
				repo.quotes["quote-1"] = wave5Quote(quotes.QuoteStatusAccepted)
				invoiceRepo := newMockInvoicingRepository()
				invoiceRepo.createErr = errors.New("invoice down")
				h.invoicingService = invoicing.NewServiceWithRepository(invoiceRepo, nil)
			}, http.StatusBadRequest, "invoice down"},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				h, repo, tenantRepo := setupQuotesTestHandlers()
				h.invoicingService = invoicing.NewServiceWithRepository(newMockInvoicingRepository(), nil)
				wave5AddTenant(tenantRepo)
				if tt.setup != nil {
					tt.setup(h, repo, tenantRepo)
				}
				rr := httptest.NewRecorder()
				h.ConvertQuoteToInvoice(rr, wave5CommercialRawRequest(http.MethodPost, "/tenants/tenant-1/quotes/quote-1/convert-to-invoice", tt.body, map[string]string{"tenantID": "tenant-1", "quoteID": "quote-1"}))
				assert.Equal(t, tt.wantCode, rr.Code, rr.Body.String())
				assert.Contains(t, rr.Body.String(), tt.wantBody)
			})
		}
	})

	t.Run("mark converted error", func(t *testing.T) {
		repo := &wave5QuoteConvertFailRepository{mockQuotesRepository: newMockQuotesRepository(), convertErr: errors.New("convert marker down")}
		h := &Handlers{
			quotesService:    quotes.NewServiceWithRepository(repo),
			invoicingService: invoicing.NewServiceWithRepository(newMockInvoicingRepository(), nil),
		}
		tenantRepo := newMockTenantRepository()
		h.tenantService = tenant.NewServiceWithRepository(tenantRepo)
		wave5AddTenant(tenantRepo)
		repo.quotes["quote-1"] = wave5Quote(quotes.QuoteStatusAccepted)

		rr := httptest.NewRecorder()
		h.ConvertQuoteToInvoice(rr, wave5Request(http.MethodPost, "/tenants/tenant-1/quotes/quote-1/convert-to-invoice", quotes.ConvertQuoteToInvoiceRequest{}, map[string]string{"tenantID": "tenant-1", "quoteID": "quote-1"}))

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
		assert.Contains(t, rr.Body.String(), "Failed to mark quote converted")
	})
}

func TestBusinessCommercialWave5OrderHandlerBranches(t *testing.T) {
	t.Run("list filters and service error", func(t *testing.T) {
		h, repo, tenantRepo := setupOrdersTestHandlers()
		wave5AddTenant(tenantRepo)
		repo.orders["order-1"] = wave5Order(orders.OrderStatusConfirmed)

		rr := httptest.NewRecorder()
		h.ListOrders(rr, wave5Request(http.MethodGet, "/tenants/tenant-1/orders?status=CONFIRMED&contact_id=contact-1&from_date=2026-03-01&to_date=2026-03-31&search=ORD", nil, map[string]string{"tenantID": "tenant-1"}))
		require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
		var listed []orders.Order
		require.NoError(t, json.NewDecoder(rr.Body).Decode(&listed))
		assert.Len(t, listed, 1)

		repo.listErr = errors.New("list down")
		rr = httptest.NewRecorder()
		h.ListOrders(rr, wave5Request(http.MethodGet, "/tenants/tenant-1/orders", nil, map[string]string{"tenantID": "tenant-1"}))
		assert.Equal(t, http.StatusInternalServerError, rr.Code)
		assert.Contains(t, rr.Body.String(), "Failed to list orders")
	})

	t.Run("create validation and service error", func(t *testing.T) {
		h, repo, tenantRepo := setupOrdersTestHandlers()
		wave5AddTenant(tenantRepo)

		cases := []struct {
			name string
			req  *http.Request
			body string
		}{
			{"invalid json", wave5CommercialRawRequest(http.MethodPost, "/tenants/tenant-1/orders", "{", map[string]string{"tenantID": "tenant-1"}), "Invalid request body"},
			{"contact required", wave5Request(http.MethodPost, "/tenants/tenant-1/orders", orders.CreateOrderRequest{Lines: []orders.CreateOrderLineRequest{wave5OrderLine()}}, map[string]string{"tenantID": "tenant-1"}), "Contact is required"},
			{"line required", wave5Request(http.MethodPost, "/tenants/tenant-1/orders", orders.CreateOrderRequest{ContactID: "contact-1"}, map[string]string{"tenantID": "tenant-1"}), "At least one line is required"},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				rr := httptest.NewRecorder()
				h.CreateOrder(rr, tc.req)
				assert.Equal(t, http.StatusBadRequest, rr.Code)
				assert.Contains(t, rr.Body.String(), tc.body)
			})
		}

		repo.createErr = errors.New("create down")
		rr := httptest.NewRecorder()
		h.CreateOrder(rr, wave5Request(http.MethodPost, "/tenants/tenant-1/orders", orders.CreateOrderRequest{
			ContactID: "contact-1",
			Lines:     []orders.CreateOrderLineRequest{wave5OrderLine()},
		}, map[string]string{"tenantID": "tenant-1"}))
		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "create down")
	})

	t.Run("import validation dependencies and default filename", func(t *testing.T) {
		h, repo, tenantRepo, contactsRepo, quotesRepo := setupOrdersImportTestHandlers()
		wave5AddTenant(tenantRepo)

		rr := httptest.NewRecorder()
		h.ImportOrders(rr, wave5CommercialRawRequest(http.MethodPost, "/tenants/tenant-1/orders/import", "{", map[string]string{"tenantID": "tenant-1"}))
		assert.Equal(t, http.StatusBadRequest, rr.Code)

		rr = httptest.NewRecorder()
		h.ImportOrders(rr, wave5Request(http.MethodPost, "/tenants/tenant-1/orders/import", orders.ImportOrdersRequest{}, map[string]string{"tenantID": "tenant-1"}))
		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "csv_content is required")

		contactsRepo.listErr = errors.New("contacts down")
		rr = httptest.NewRecorder()
		h.ImportOrders(rr, wave5Request(http.MethodPost, "/tenants/tenant-1/orders/import", orders.ImportOrdersRequest{CSVContent: "order_number,contact_id,order_date,line_description,quantity,unit_price,vat_rate\nORD-1,contact-1,2026-03-01,Work,1,100,22"}, map[string]string{"tenantID": "tenant-1"}))
		assert.Equal(t, http.StatusInternalServerError, rr.Code)
		assert.Contains(t, rr.Body.String(), "Failed to load contacts")
		contactsRepo.listErr = nil

		inventoryRepo := newMockInventoryRepository()
		inventoryRepo.listProductsErr = errors.New("products down")
		h.inventoryService = inventory.NewServiceWithRepository(inventoryRepo)
		rr = httptest.NewRecorder()
		h.ImportOrders(rr, wave5Request(http.MethodPost, "/tenants/tenant-1/orders/import", orders.ImportOrdersRequest{CSVContent: "order_number,contact_id,order_date,line_description,quantity,unit_price,vat_rate\nORD-1,contact-1,2026-03-01,Work,1,100,22"}, map[string]string{"tenantID": "tenant-1"}))
		assert.Equal(t, http.StatusInternalServerError, rr.Code)
		assert.Contains(t, rr.Body.String(), "Failed to load products")
		h.inventoryService = nil

		quotesRepo.listErr = errors.New("quotes down")
		rr = httptest.NewRecorder()
		h.ImportOrders(rr, wave5Request(http.MethodPost, "/tenants/tenant-1/orders/import", orders.ImportOrdersRequest{CSVContent: "order_number,contact_id,order_date,line_description,quantity,unit_price,vat_rate\nORD-1,contact-1,2026-03-01,Work,1,100,22"}, map[string]string{"tenantID": "tenant-1"}))
		assert.Equal(t, http.StatusInternalServerError, rr.Code)
		assert.Contains(t, rr.Body.String(), "Failed to load quotes")
		quotesRepo.listErr = nil

		repo.listErr = errors.New("orders down")
		rr = httptest.NewRecorder()
		h.ImportOrders(rr, wave5Request(http.MethodPost, "/tenants/tenant-1/orders/import", orders.ImportOrdersRequest{CSVContent: "order_number,contact_id,order_date,line_description,quantity,unit_price,vat_rate\nORD-1,contact-1,2026-03-01,Work,1,100,22"}, map[string]string{"tenantID": "tenant-1"}))
		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "orders down")
		repo.listErr = nil

		contactsRepo.addTestContact("contact-1", "tenant-1", "Acme OU", contacts.ContactTypeCustomer, true)
		rr = httptest.NewRecorder()
		h.ImportOrders(rr, wave5Request(http.MethodPost, "/tenants/tenant-1/orders/import", orders.ImportOrdersRequest{CSVContent: "order_number,contact_id,order_date,line_description,quantity,unit_price,vat_rate\nORD-1,contact-1,2026-03-01,Work,1,100,22"}, map[string]string{"tenantID": "tenant-1"}))
		require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
		var result orders.ImportOrdersResult
		require.NoError(t, json.NewDecoder(rr.Body).Decode(&result))
		assert.Equal(t, "orders_import.csv", result.FileName)
	})

	t.Run("pdf update and status errors", func(t *testing.T) {
		h, repo, tenantRepo := setupOrdersTestHandlers()
		h.pdfService = pdf.NewService()
		repo.orders["order-1"] = wave5Order(orders.OrderStatusPending)

		rr := httptest.NewRecorder()
		h.GetOrderPDF(rr, wave5Request(http.MethodGet, "/tenants/tenant-1/orders/missing/pdf", nil, map[string]string{"tenantID": "tenant-1", "orderID": "missing"}))
		assert.Equal(t, http.StatusNotFound, rr.Code)

		rr = httptest.NewRecorder()
		h.GetOrderPDF(rr, wave5Request(http.MethodGet, "/tenants/tenant-1/orders/order-1/pdf", nil, map[string]string{"tenantID": "tenant-1", "orderID": "order-1"}))
		assert.Equal(t, http.StatusInternalServerError, rr.Code)
		assert.Contains(t, rr.Body.String(), "Failed to get tenant")

		wave5AddTenant(tenantRepo)
		rr = httptest.NewRecorder()
		h.UpdateOrder(rr, wave5CommercialRawRequest(http.MethodPut, "/tenants/tenant-1/orders/order-1", "{", map[string]string{"tenantID": "tenant-1", "orderID": "order-1"}))
		assert.Equal(t, http.StatusBadRequest, rr.Code)

		repo.updateErr = errors.New("update down")
		rr = httptest.NewRecorder()
		h.UpdateOrder(rr, wave5Request(http.MethodPut, "/tenants/tenant-1/orders/order-1", orders.UpdateOrderRequest{
			ContactID: "contact-1",
			OrderDate: time.Date(2026, 3, 20, 0, 0, 0, 0, time.UTC),
			Lines:     []orders.CreateOrderLineRequest{wave5OrderLine()},
		}, map[string]string{"tenantID": "tenant-1", "orderID": "order-1"}))
		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "update down")
		repo.updateErr = nil

		repo.statusErr = errors.New("status down")
		for _, tc := range []struct {
			name   string
			status orders.OrderStatus
			invoke func(http.ResponseWriter, *http.Request)
		}{
			{"confirm", orders.OrderStatusPending, h.ConfirmOrder},
			{"process", orders.OrderStatusConfirmed, h.ProcessOrder},
			{"ship", orders.OrderStatusConfirmed, h.ShipOrder},
			{"deliver", orders.OrderStatusShipped, h.DeliverOrder},
			{"cancel", orders.OrderStatusConfirmed, h.CancelOrder},
		} {
			t.Run(tc.name, func(t *testing.T) {
				repo.orders["order-1"].Status = tc.status
				rr := httptest.NewRecorder()
				tc.invoke(rr, wave5Request(http.MethodPost, "/tenants/tenant-1/orders/order-1/"+tc.name, nil, map[string]string{"tenantID": "tenant-1", "orderID": "order-1"}))
				assert.Equal(t, http.StatusBadRequest, rr.Code)
				assert.Contains(t, rr.Body.String(), "status down")
			})
		}

		rr = httptest.NewRecorder()
		h.ConfirmOrder(rr, wave5CommercialRawRequest(http.MethodPost, "/tenants/tenant-1/orders/order-1/confirm", "{", map[string]string{"tenantID": "tenant-1", "orderID": "order-1"}))
		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})
}

func TestBusinessCommercialWave5OrderStockBranches(t *testing.T) {
	t.Run("request auth and lookup failures", func(t *testing.T) {
		h, repo, tenantRepo := setupOrdersTestHandlers()
		inventoryRepo := newMockInventoryRepository()
		h.inventoryService = inventory.NewServiceWithRepository(inventoryRepo)
		wave5AddTenant(tenantRepo)
		warehouseID := "22222222-2222-4222-8222-222222222222"
		inventoryRepo.warehouses[warehouseID] = &inventory.Warehouse{ID: warehouseID, TenantID: "tenant-1", Code: "MAIN", Name: "Main", IsActive: true}

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/tenants/tenant-1/orders/order-1/reserve-stock", strings.NewReader(`{"warehouse_id":"`+warehouseID+`"}`))
		req.Header.Set("Content-Type", "application/json")
		req = withURLParams(req, map[string]string{"tenantID": "tenant-1", "orderID": "order-1"})
		h.ReserveOrderStock(rr, req)
		assert.Equal(t, http.StatusUnauthorized, rr.Code)

		rr = httptest.NewRecorder()
		h.ReserveOrderStock(rr, wave5CommercialRawRequest(http.MethodPost, "/tenants/tenant-1/orders/order-1/reserve-stock", "{", map[string]string{"tenantID": "tenant-1", "orderID": "order-1"}))
		assert.Equal(t, http.StatusBadRequest, rr.Code)

		rr = httptest.NewRecorder()
		h.ReserveOrderStock(rr, wave5Request(http.MethodPost, "/tenants/tenant-1/orders/order-1/reserve-stock", orders.OrderStockReservationRequest{WarehouseID: "missing"}, map[string]string{"tenantID": "tenant-1", "orderID": "order-1"}))
		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "Warehouse not found")

		rr = httptest.NewRecorder()
		h.ReserveOrderStock(rr, wave5Request(http.MethodPost, "/tenants/tenant-1/orders/order-1/reserve-stock", orders.OrderStockReservationRequest{WarehouseID: warehouseID}, map[string]string{"tenantID": "tenant-1", "orderID": "order-1"}))
		assert.Equal(t, http.StatusNotFound, rr.Code)
		assert.Contains(t, rr.Body.String(), "Order not found")

		repo.getErr = errors.New("orders down")
		rr = httptest.NewRecorder()
		h.CheckOrderStock(rr, wave5Request(http.MethodGet, "/tenants/tenant-1/orders/order-1/stock-check", nil, map[string]string{"tenantID": "tenant-1", "orderID": "order-1"}))
		assert.Equal(t, http.StatusInternalServerError, rr.Code)
		repo.getErr = nil
	})

	t.Run("stock levels and reservation list failures", func(t *testing.T) {
		h, repo, tenantRepo := setupOrdersTestHandlers()
		inventoryRepo := newMockInventoryRepository()
		h.inventoryService = inventory.NewServiceWithRepository(inventoryRepo)
		wave5AddTenant(tenantRepo)
		productID := "11111111-1111-4111-8111-111111111111"
		warehouseID := "22222222-2222-4222-8222-222222222222"
		repo.orders["order-1"] = wave5Order(orders.OrderStatusConfirmed)
		repo.orders["order-1"].Lines[0].ProductID = &productID
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
		inventoryRepo.getStockErr = errors.New("stock down")

		rr := httptest.NewRecorder()
		h.CheckOrderStock(rr, wave5Request(http.MethodGet, "/tenants/tenant-1/orders/order-1/stock-check", nil, map[string]string{"tenantID": "tenant-1", "orderID": "order-1"}))
		assert.Equal(t, http.StatusInternalServerError, rr.Code)

		failingRepo := &wave5OrderStockListFailRepository{mockOrdersRepository: repo, listErr: errors.New("reservations down")}
		h.ordersService = orders.NewServiceWithRepository(failingRepo)
		inventoryRepo.getStockErr = nil
		rr = httptest.NewRecorder()
		h.ListOrderStockReservations(rr, wave5Request(http.MethodGet, "/tenants/tenant-1/orders/order-1/stock-reservations", nil, map[string]string{"tenantID": "tenant-1", "orderID": "order-1"}))
		assert.Equal(t, http.StatusInternalServerError, rr.Code)

		rr = httptest.NewRecorder()
		h.GetOrderPickList(rr, wave5Request(http.MethodGet, "/tenants/tenant-1/orders/order-1/pick-list?warehouse_id="+warehouseID, nil, map[string]string{"tenantID": "tenant-1", "orderID": "order-1"}))
		assert.Equal(t, http.StatusInternalServerError, rr.Code)
	})

	t.Run("pick list shortage and release over reservation", func(t *testing.T) {
		h, repo, tenantRepo := setupOrdersTestHandlers()
		inventoryRepo := newMockInventoryRepository()
		h.inventoryService = inventory.NewServiceWithRepository(inventoryRepo)
		wave5AddTenant(tenantRepo)
		productID := "11111111-1111-4111-8111-111111111111"
		warehouseID := "22222222-2222-4222-8222-222222222222"
		repo.orders["order-1"] = wave5Order(orders.OrderStatusConfirmed)
		repo.orders["order-1"].Lines[0].ProductID = &productID
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
			Quantity:     decimal.NewFromInt(10),
			ReservedQty:  decimal.NewFromInt(1),
			AvailableQty: decimal.NewFromInt(9),
		}
		repo.stockReservations[orderStockReservationKey("order-1", productID, warehouseID)] = &orders.OrderStockReservation{
			ID:          "reservation-1",
			TenantID:    "tenant-1",
			OrderID:     "order-1",
			ProductID:   productID,
			WarehouseID: warehouseID,
			Quantity:    decimal.NewFromInt(1),
			Status:      orders.OrderStockReservationStatusReserved,
		}

		rr := httptest.NewRecorder()
		h.GetOrderPickList(rr, wave5Request(http.MethodGet, "/tenants/tenant-1/orders/order-1/pick-list?warehouse_id="+warehouseID, nil, map[string]string{"tenantID": "tenant-1", "orderID": "order-1"}))
		require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
		var pickList orders.OrderPickList
		require.NoError(t, json.NewDecoder(rr.Body).Decode(&pickList))
		assert.False(t, pickList.Ready)
		require.Len(t, pickList.Lines, 1)
		assert.Equal(t, orders.OrderPickListLineStatusShortage, pickList.Lines[0].Status)

		rr = httptest.NewRecorder()
		h.ReleaseOrderStock(rr, wave5Request(http.MethodPost, "/tenants/tenant-1/orders/order-1/release-stock", orders.OrderStockReservationRequest{WarehouseID: warehouseID}, map[string]string{"tenantID": "tenant-1", "orderID": "order-1"}))
		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "cannot release more than order reserved stock")
	})
}

func TestBusinessCommercialWave5OrderConvertBranches(t *testing.T) {
	t.Run("defaults dates and order notes", func(t *testing.T) {
		h, repo, tenantRepo := setupOrdersTestHandlers()
		h.invoicingService = invoicing.NewServiceWithRepository(newMockInvoicingRepository(), nil)
		wave5AddTenant(tenantRepo)
		repo.orders["order-1"] = wave5Order(orders.OrderStatusDelivered)

		rr := httptest.NewRecorder()
		h.ConvertOrderToInvoice(rr, wave5Request(http.MethodPost, "/tenants/tenant-1/orders/order-1/convert-to-invoice", orders.ConvertOrderToInvoiceRequest{}, map[string]string{"tenantID": "tenant-1", "orderID": "order-1"}))

		require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())
		var result orders.OrderInvoiceConversionResult
		require.NoError(t, json.NewDecoder(rr.Body).Decode(&result))
		assert.Equal(t, "Order notes", result.Invoice.Notes)
		assert.Equal(t, result.Invoice.IssueDate.AddDate(0, 0, 14).Format("2006-01-02"), result.Invoice.DueDate.Format("2006-01-02"))
	})

	t.Run("validation and service errors", func(t *testing.T) {
		tests := []struct {
			name     string
			body     string
			setup    func(*Handlers, *mockOrdersRepository)
			wantCode int
			wantBody string
		}{
			{"invalid json", "{", nil, http.StatusBadRequest, "Invalid request body"},
			{"due date before issue", `{"issue_date":"2026-03-10T00:00:00Z","due_date":"2026-03-09T00:00:00Z"}`, nil, http.StatusBadRequest, "due date cannot be before issue date"},
			{"order missing", `{}`, nil, http.StatusNotFound, "Order not found"},
			{"invoice create error", `{}`, func(h *Handlers, repo *mockOrdersRepository) {
				repo.orders["order-1"] = wave5Order(orders.OrderStatusDelivered)
				invoiceRepo := newMockInvoicingRepository()
				invoiceRepo.createErr = errors.New("invoice down")
				h.invoicingService = invoicing.NewServiceWithRepository(invoiceRepo, nil)
			}, http.StatusBadRequest, "invoice down"},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				h, repo, tenantRepo := setupOrdersTestHandlers()
				h.invoicingService = invoicing.NewServiceWithRepository(newMockInvoicingRepository(), nil)
				wave5AddTenant(tenantRepo)
				if tt.setup != nil {
					tt.setup(h, repo)
				}
				rr := httptest.NewRecorder()
				h.ConvertOrderToInvoice(rr, wave5CommercialRawRequest(http.MethodPost, "/tenants/tenant-1/orders/order-1/convert-to-invoice", tt.body, map[string]string{"tenantID": "tenant-1", "orderID": "order-1"}))
				assert.Equal(t, tt.wantCode, rr.Code, rr.Body.String())
				assert.Contains(t, rr.Body.String(), tt.wantBody)
			})
		}
	})

	t.Run("mark converted error", func(t *testing.T) {
		repo := &wave5OrderConvertFailRepository{mockOrdersRepository: newMockOrdersRepository(), convertErr: errors.New("convert marker down")}
		h := &Handlers{
			ordersService:    orders.NewServiceWithRepository(repo),
			invoicingService: invoicing.NewServiceWithRepository(newMockInvoicingRepository(), nil),
		}
		tenantRepo := newMockTenantRepository()
		h.tenantService = tenant.NewServiceWithRepository(tenantRepo)
		wave5AddTenant(tenantRepo)
		repo.orders["order-1"] = wave5Order(orders.OrderStatusDelivered)

		rr := httptest.NewRecorder()
		h.ConvertOrderToInvoice(rr, wave5Request(http.MethodPost, "/tenants/tenant-1/orders/order-1/convert-to-invoice", orders.ConvertOrderToInvoiceRequest{}, map[string]string{"tenantID": "tenant-1", "orderID": "order-1"}))

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
		assert.Contains(t, rr.Body.String(), "Failed to mark order converted")
	})
}

type wave5EmailRepository struct {
	*emailHandlerRepository

	getSettingsErr    error
	updateSettingsErr error
	getTemplateErr    error
	listTemplatesErr  error
	upsertTemplateErr error
	getEmailLogErr    error
}

func newWave5EmailRepository() *wave5EmailRepository {
	return &wave5EmailRepository{
		emailHandlerRepository: &emailHandlerRepository{
			settings:  make(map[string][]byte),
			templates: make(map[string]email.EmailTemplate),
			logs:      []email.EmailLog{},
		},
	}
}

func (r *wave5EmailRepository) GetTenantSettings(ctx context.Context, tenantID string) ([]byte, error) {
	if r.getSettingsErr != nil {
		return nil, r.getSettingsErr
	}
	return r.emailHandlerRepository.GetTenantSettings(ctx, tenantID)
}

func (r *wave5EmailRepository) UpdateTenantSettings(ctx context.Context, tenantID string, settingsJSON []byte) error {
	if r.updateSettingsErr != nil {
		return r.updateSettingsErr
	}
	return r.emailHandlerRepository.UpdateTenantSettings(ctx, tenantID, settingsJSON)
}

func (r *wave5EmailRepository) GetTemplate(ctx context.Context, schemaName, tenantID string, templateType email.TemplateType) (*email.EmailTemplate, error) {
	if r.getTemplateErr != nil {
		return nil, r.getTemplateErr
	}
	return r.emailHandlerRepository.GetTemplate(ctx, schemaName, tenantID, templateType)
}

func (r *wave5EmailRepository) ListTemplates(ctx context.Context, schemaName, tenantID string) ([]email.EmailTemplate, error) {
	if r.listTemplatesErr != nil {
		return nil, r.listTemplatesErr
	}
	return r.emailHandlerRepository.ListTemplates(ctx, schemaName, tenantID)
}

func (r *wave5EmailRepository) UpsertTemplate(ctx context.Context, schemaName string, template *email.EmailTemplate) error {
	if r.upsertTemplateErr != nil {
		return r.upsertTemplateErr
	}
	return r.emailHandlerRepository.UpsertTemplate(ctx, schemaName, template)
}

func (r *wave5EmailRepository) GetEmailLog(ctx context.Context, schemaName, tenantID string, limit int) ([]email.EmailLog, error) {
	if r.getEmailLogErr != nil {
		return nil, r.getEmailLogErr
	}
	return r.emailHandlerRepository.GetEmailLog(ctx, schemaName, tenantID, limit)
}

type wave5QuoteConvertFailRepository struct {
	*mockQuotesRepository
	convertErr error
}

func (m *wave5QuoteConvertFailRepository) SetConvertedToInvoice(ctx context.Context, schemaName, tenantID, quoteID, invoiceID string) error {
	if m.convertErr != nil {
		return m.convertErr
	}
	return m.mockQuotesRepository.SetConvertedToInvoice(ctx, schemaName, tenantID, quoteID, invoiceID)
}

type wave5OrderConvertFailRepository struct {
	*mockOrdersRepository
	convertErr error
}

func (m *wave5OrderConvertFailRepository) SetConvertedToInvoice(ctx context.Context, schemaName, tenantID, orderID, invoiceID string) error {
	if m.convertErr != nil {
		return m.convertErr
	}
	return m.mockOrdersRepository.SetConvertedToInvoice(ctx, schemaName, tenantID, orderID, invoiceID)
}

type wave5OrderStockListFailRepository struct {
	*mockOrdersRepository
	listErr error
}

func (m *wave5OrderStockListFailRepository) ListStockReservations(ctx context.Context, schemaName, tenantID, orderID string) ([]orders.OrderStockReservation, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	return m.mockOrdersRepository.ListStockReservations(ctx, schemaName, tenantID, orderID)
}
