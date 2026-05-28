package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"github.com/HMB-research/open-accounting/internal/accounting"
	"github.com/HMB-research/open-accounting/internal/analytics"
	"github.com/HMB-research/open-accounting/internal/apitoken"
	"github.com/HMB-research/open-accounting/internal/contacts"
	"github.com/HMB-research/open-accounting/internal/documents"
	"github.com/HMB-research/open-accounting/internal/invoicing"
	"github.com/HMB-research/open-accounting/internal/payments"
	"github.com/HMB-research/open-accounting/internal/payroll"
	"github.com/HMB-research/open-accounting/internal/reports"
	"github.com/HMB-research/open-accounting/internal/tax"
	"github.com/HMB-research/open-accounting/internal/tenant"
)

type apiClient struct {
	baseURL    string
	apiToken   string
	httpClient *http.Client
}

type loginResponse struct {
	AccessToken string `json:"access_token"`
}

type currentUser struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
}

type accountBalanceReport struct {
	AccountID string `json:"account_id"`
	AsOfDate  string `json:"as_of_date"`
	Balance   string `json:"balance"`
}

func newAPIClient(baseURL, apiToken string) *apiClient {
	return &apiClient{
		baseURL:  normalizeBaseURL(baseURL),
		apiToken: apiToken,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *apiClient) login(ctx context.Context, email, password string) (*loginResponse, error) {
	var resp loginResponse
	err := c.request(ctx, http.MethodPost, "/api/v1/auth/login", map[string]string{
		"email":    email,
		"password": password,
	}, "", &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) getCurrentUser(ctx context.Context) (*currentUser, error) {
	var resp currentUser
	if err := c.request(ctx, http.MethodGet, "/api/v1/me", nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) listMyTenants(ctx context.Context, bearerToken string) ([]tenant.TenantMembership, error) {
	var resp []tenant.TenantMembership
	if err := c.request(ctx, http.MethodGet, "/api/v1/me/tenants", nil, bearerToken, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *apiClient) createAPIToken(ctx context.Context, tenantID string, req *apitoken.CreateRequest, bearerToken string) (*apitoken.CreateResult, error) {
	var resp apitoken.CreateResult
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "api-tokens"), req, bearerToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) listAPITokens(ctx context.Context, tenantID string) ([]apitoken.APIToken, error) {
	var resp []apitoken.APIToken
	if err := c.request(ctx, http.MethodGet, path.Join("/api/v1/tenants", tenantID, "api-tokens"), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *apiClient) revokeAPIToken(ctx context.Context, tenantID, tokenID string) error {
	return c.request(ctx, http.MethodDelete, path.Join("/api/v1/tenants", tenantID, "api-tokens", tokenID), nil, c.apiToken, nil)
}

func (c *apiClient) listAccounts(ctx context.Context, tenantID string, activeOnly bool) ([]accounting.Account, error) {
	query := ""
	if activeOnly {
		query = "?active_only=true"
	}
	var resp []accounting.Account
	if err := c.request(ctx, http.MethodGet, path.Join("/api/v1/tenants", tenantID, "accounts")+query, nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *apiClient) createAccount(ctx context.Context, tenantID string, req *accounting.CreateAccountRequest) (*accounting.Account, error) {
	var resp accounting.Account
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "accounts"), req, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) importAccounts(ctx context.Context, tenantID string, req *accounting.ImportAccountsRequest) (*accounting.ImportAccountsResult, error) {
	var resp accounting.ImportAccountsResult
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "accounts", "import"), req, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) listContacts(ctx context.Context, tenantID string, filter contacts.ContactFilter) ([]contacts.Contact, error) {
	values := url.Values{}
	if filter.ContactType != "" {
		values.Set("type", string(filter.ContactType))
	}
	if filter.ActiveOnly {
		values.Set("active_only", "true")
	}
	if filter.Search != "" {
		values.Set("search", filter.Search)
	}

	urlPath := path.Join("/api/v1/tenants", tenantID, "contacts")
	if encoded := values.Encode(); encoded != "" {
		urlPath += "?" + encoded
	}

	var resp []contacts.Contact
	if err := c.request(ctx, http.MethodGet, urlPath, nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *apiClient) createContact(ctx context.Context, tenantID string, req *contacts.CreateContactRequest) (*contacts.Contact, error) {
	var resp contacts.Contact
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "contacts"), req, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) importContacts(ctx context.Context, tenantID string, req *contacts.ImportContactsRequest) (*contacts.ImportContactsResult, error) {
	var resp contacts.ImportContactsResult
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "contacts", "import"), req, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) importInvoices(ctx context.Context, tenantID string, req *invoicing.ImportInvoicesRequest) (*invoicing.ImportInvoicesResult, error) {
	var resp invoicing.ImportInvoicesResult
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "invoices", "import"), req, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) listPayments(ctx context.Context, tenantID string, filter payments.PaymentFilter) ([]payments.Payment, error) {
	values := url.Values{}
	if filter.PaymentType != "" {
		values.Set("type", string(filter.PaymentType))
	}
	if strings.TrimSpace(filter.PaymentMethod) != "" {
		values.Set("method", strings.TrimSpace(filter.PaymentMethod))
	}
	if strings.TrimSpace(filter.ContactID) != "" {
		values.Set("contact_id", strings.TrimSpace(filter.ContactID))
	}
	if filter.FromDate != nil {
		values.Set("from_date", filter.FromDate.Format("2006-01-02"))
	}
	if filter.ToDate != nil {
		values.Set("to_date", filter.ToDate.Format("2006-01-02"))
	}

	var resp []payments.Payment
	if err := c.request(ctx, http.MethodGet, withQuery(path.Join("/api/v1/tenants", tenantID, "payments"), values), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *apiClient) createPayment(ctx context.Context, tenantID string, req *payments.CreatePaymentRequest) (*payments.Payment, error) {
	var resp payments.Payment
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "payments"), req, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) getPayment(ctx context.Context, tenantID, paymentID string) (*payments.Payment, error) {
	var resp payments.Payment
	if err := c.request(ctx, http.MethodGet, path.Join("/api/v1/tenants", tenantID, "payments", paymentID), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) allocatePayment(ctx context.Context, tenantID, paymentID string, req *payments.AllocationRequest) (map[string]string, error) {
	var resp map[string]string
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "payments", paymentID, "allocate"), req, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *apiClient) listUnallocatedPayments(ctx context.Context, tenantID string, paymentType payments.PaymentType) ([]payments.Payment, error) {
	values := url.Values{}
	if paymentType != "" {
		values.Set("type", string(paymentType))
	}

	var resp []payments.Payment
	if err := c.request(ctx, http.MethodGet, withQuery(path.Join("/api/v1/tenants", tenantID, "payments", "unallocated"), values), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *apiClient) importOpeningBalances(ctx context.Context, tenantID string, req *accounting.ImportOpeningBalancesRequest) (*accounting.ImportOpeningBalancesResult, error) {
	var resp accounting.ImportOpeningBalancesResult
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "journal-entries", "import-opening-balances"), req, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) listEmployees(ctx context.Context, tenantID string, activeOnly bool) ([]payroll.Employee, error) {
	urlPath := path.Join("/api/v1/tenants", tenantID, "employees")
	if activeOnly {
		urlPath += "?active_only=true"
	}

	var resp []payroll.Employee
	if err := c.request(ctx, http.MethodGet, urlPath, nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *apiClient) createEmployee(ctx context.Context, tenantID string, req *payroll.CreateEmployeeRequest) (*payroll.Employee, error) {
	var resp payroll.Employee
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "employees"), req, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) importEmployees(ctx context.Context, tenantID string, req *payroll.ImportEmployeesRequest) (*payroll.ImportEmployeesResult, error) {
	var resp payroll.ImportEmployeesResult
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "employees", "import"), req, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) importPayrollHistory(ctx context.Context, tenantID string, req *payroll.ImportPayrollHistoryRequest) (*payroll.ImportPayrollHistoryResult, error) {
	var resp payroll.ImportPayrollHistoryResult
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "payroll-runs", "import-history"), req, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) importLeaveBalances(ctx context.Context, tenantID string, req *payroll.ImportLeaveBalancesRequest) (*payroll.ImportLeaveBalancesResult, error) {
	var resp payroll.ImportLeaveBalancesResult
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "leave-balances", "import"), req, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) listPayrollRuns(ctx context.Context, tenantID string, year int) ([]payroll.PayrollRun, error) {
	values := url.Values{}
	if year > 0 {
		values.Set("year", strconv.Itoa(year))
	}

	var resp []payroll.PayrollRun
	if err := c.request(ctx, http.MethodGet, withQuery(path.Join("/api/v1/tenants", tenantID, "payroll-runs"), values), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *apiClient) createPayrollRun(ctx context.Context, tenantID string, req *payroll.CreatePayrollRunRequest) (*payroll.PayrollRun, error) {
	var resp payroll.PayrollRun
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "payroll-runs"), req, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) getPayrollRun(ctx context.Context, tenantID, runID string) (*payroll.PayrollRun, error) {
	var resp payroll.PayrollRun
	if err := c.request(ctx, http.MethodGet, path.Join("/api/v1/tenants", tenantID, "payroll-runs", runID), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) calculatePayrollRun(ctx context.Context, tenantID, runID string) (*payroll.PayrollRun, error) {
	var resp payroll.PayrollRun
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "payroll-runs", runID, "calculate"), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) approvePayrollRun(ctx context.Context, tenantID, runID string) (map[string]string, error) {
	var resp map[string]string
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "payroll-runs", runID, "approve"), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *apiClient) listPayslips(ctx context.Context, tenantID, runID string) ([]payroll.Payslip, error) {
	var resp []payroll.Payslip
	if err := c.request(ctx, http.MethodGet, path.Join("/api/v1/tenants", tenantID, "payroll-runs", runID, "payslips"), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *apiClient) calculateTaxPreview(ctx context.Context, tenantID string, grossSalary decimal.Decimal, applyBasicExemption bool, fundedPensionRate decimal.Decimal) (*payroll.TaxCalculation, error) {
	var resp payroll.TaxCalculation
	body := map[string]any{
		"gross_salary":          grossSalary,
		"apply_basic_exemption": applyBasicExemption,
		"funded_pension_rate":   fundedPensionRate,
	}
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "payroll", "tax-preview"), body, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) listTSD(ctx context.Context, tenantID string) ([]payroll.TSDDeclaration, error) {
	var resp []payroll.TSDDeclaration
	if err := c.request(ctx, http.MethodGet, path.Join("/api/v1/tenants", tenantID, "tsd"), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *apiClient) getTSD(ctx context.Context, tenantID string, year, month int) (*payroll.TSDDeclaration, error) {
	var resp payroll.TSDDeclaration
	if err := c.request(ctx, http.MethodGet, path.Join("/api/v1/tenants", tenantID, "tsd", strconv.Itoa(year), strconv.Itoa(month)), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) generateTSD(ctx context.Context, tenantID, runID string) (*payroll.TSDDeclaration, error) {
	var resp payroll.TSDDeclaration
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "payroll-runs", runID, "tsd"), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) exportTSDXML(ctx context.Context, tenantID string, year, month int) ([]byte, error) {
	return c.requestRaw(ctx, http.MethodGet, path.Join("/api/v1/tenants", tenantID, "tsd", strconv.Itoa(year), strconv.Itoa(month), "xml"), nil, c.apiToken)
}

func (c *apiClient) exportTSDCSV(ctx context.Context, tenantID string, year, month int) ([]byte, error) {
	return c.requestRaw(ctx, http.MethodGet, path.Join("/api/v1/tenants", tenantID, "tsd", strconv.Itoa(year), strconv.Itoa(month), "csv"), nil, c.apiToken)
}

func (c *apiClient) markTSDSubmitted(ctx context.Context, tenantID string, year, month int, emtaReference string) (map[string]string, error) {
	var resp map[string]string
	body := map[string]string{"emta_reference": emtaReference}
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "tsd", strconv.Itoa(year), strconv.Itoa(month), "submit"), body, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *apiClient) listKMD(ctx context.Context, tenantID string) ([]tax.KMDDeclaration, error) {
	var resp []tax.KMDDeclaration
	if err := c.request(ctx, http.MethodGet, path.Join("/api/v1/tenants", tenantID, "tax", "kmd"), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *apiClient) generateKMD(ctx context.Context, tenantID string, req *tax.CreateKMDRequest) (*tax.KMDDeclaration, error) {
	var resp tax.KMDDeclaration
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "tax", "kmd"), req, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) exportKMDXML(ctx context.Context, tenantID string, year, month int) ([]byte, error) {
	return c.requestRaw(ctx, http.MethodGet, path.Join("/api/v1/tenants", tenantID, "tax", "kmd", strconv.Itoa(year), strconv.Itoa(month), "xml"), nil, c.apiToken)
}

func (c *apiClient) getTrialBalance(ctx context.Context, tenantID, asOfDate string) (*accounting.TrialBalance, error) {
	values := url.Values{}
	if strings.TrimSpace(asOfDate) != "" {
		values.Set("as_of_date", strings.TrimSpace(asOfDate))
	}

	var resp accounting.TrialBalance
	if err := c.request(ctx, http.MethodGet, withQuery(path.Join("/api/v1/tenants", tenantID, "reports", "trial-balance"), values), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) getAccountBalanceReport(ctx context.Context, tenantID, accountID, asOfDate string) (*accountBalanceReport, error) {
	values := url.Values{}
	if strings.TrimSpace(asOfDate) != "" {
		values.Set("as_of_date", strings.TrimSpace(asOfDate))
	}

	var resp accountBalanceReport
	if err := c.request(ctx, http.MethodGet, withQuery(path.Join("/api/v1/tenants", tenantID, "reports", "account-balance", accountID), values), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) getBalanceSheet(ctx context.Context, tenantID, asOfDate string) (*accounting.BalanceSheet, error) {
	values := url.Values{}
	if strings.TrimSpace(asOfDate) != "" {
		values.Set("as_of", strings.TrimSpace(asOfDate))
	}

	var resp accounting.BalanceSheet
	if err := c.request(ctx, http.MethodGet, withQuery(path.Join("/api/v1/tenants", tenantID, "reports", "balance-sheet"), values), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) getIncomeStatement(ctx context.Context, tenantID, startDate, endDate string) (*accounting.IncomeStatement, error) {
	values := url.Values{}
	values.Set("start", strings.TrimSpace(startDate))
	values.Set("end", strings.TrimSpace(endDate))

	var resp accounting.IncomeStatement
	if err := c.request(ctx, http.MethodGet, withQuery(path.Join("/api/v1/tenants", tenantID, "reports", "income-statement"), values), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) getCashFlowStatement(ctx context.Context, tenantID, startDate, endDate string) (*reports.CashFlowStatement, error) {
	values := url.Values{}
	values.Set("start_date", strings.TrimSpace(startDate))
	values.Set("end_date", strings.TrimSpace(endDate))

	var resp reports.CashFlowStatement
	if err := c.request(ctx, http.MethodGet, withQuery(path.Join("/api/v1/tenants", tenantID, "reports", "cash-flow"), values), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) getBalanceConfirmationSummary(ctx context.Context, tenantID, balanceType, asOfDate string) (*reports.BalanceConfirmationSummary, error) {
	values := url.Values{}
	values.Set("type", strings.TrimSpace(balanceType))
	values.Set("as_of_date", strings.TrimSpace(asOfDate))

	var resp reports.BalanceConfirmationSummary
	if err := c.request(ctx, http.MethodGet, withQuery(path.Join("/api/v1/tenants", tenantID, "reports", "balance-confirmations"), values), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) getBalanceConfirmation(ctx context.Context, tenantID, contactID, balanceType, asOfDate string) (*reports.BalanceConfirmation, error) {
	values := url.Values{}
	values.Set("type", strings.TrimSpace(balanceType))
	values.Set("as_of_date", strings.TrimSpace(asOfDate))

	var resp reports.BalanceConfirmation
	if err := c.request(ctx, http.MethodGet, withQuery(path.Join("/api/v1/tenants", tenantID, "reports", "balance-confirmations", contactID), values), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) getAgingReport(ctx context.Context, tenantID, reportType string) (*analytics.AgingReport, error) {
	var resp analytics.AgingReport
	if err := c.request(ctx, http.MethodGet, path.Join("/api/v1/tenants", tenantID, "reports", "aging", reportType), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) listDocuments(ctx context.Context, tenantID, entityType, entityID string) ([]documents.Document, error) {
	values := url.Values{}
	values.Set("entity_type", entityType)
	values.Set("entity_id", entityID)

	var resp []documents.Document
	urlPath := path.Join("/api/v1/tenants", tenantID, "documents") + "?" + values.Encode()
	if err := c.request(ctx, http.MethodGet, urlPath, nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *apiClient) uploadDocument(ctx context.Context, tenantID string, req *documents.UploadDocumentRequest, fileContent []byte) (*documents.Document, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	if err := writer.WriteField("entity_type", strings.TrimSpace(req.EntityType)); err != nil {
		return nil, fmt.Errorf("write entity_type: %w", err)
	}
	if err := writer.WriteField("entity_id", strings.TrimSpace(req.EntityID)); err != nil {
		return nil, fmt.Errorf("write entity_id: %w", err)
	}
	if strings.TrimSpace(req.DocumentType) != "" {
		if err := writer.WriteField("document_type", strings.TrimSpace(req.DocumentType)); err != nil {
			return nil, fmt.Errorf("write document_type: %w", err)
		}
	}
	if strings.TrimSpace(req.Notes) != "" {
		if err := writer.WriteField("notes", strings.TrimSpace(req.Notes)); err != nil {
			return nil, fmt.Errorf("write notes: %w", err)
		}
	}
	if req.RetentionUntil != nil {
		if err := writer.WriteField("retention_until", req.RetentionUntil.Format("2006-01-02")); err != nil {
			return nil, fmt.Errorf("write retention_until: %w", err)
		}
	}

	part, err := writer.CreateFormFile("file", strings.TrimSpace(req.FileName))
	if err != nil {
		return nil, fmt.Errorf("create multipart file: %w", err)
	}
	if _, err := part.Write(fileContent); err != nil {
		return nil, fmt.Errorf("write multipart file: %w", err)
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("close multipart writer: %w", err)
	}

	fullURL := c.baseURL + path.Join("/api/v1/tenants", tenantID, "documents")
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, fullURL, &body)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("Content-Type", writer.FormDataContentType())
	if strings.TrimSpace(c.apiToken) != "" {
		httpReq.Header.Set("Authorization", "Bearer "+strings.TrimSpace(c.apiToken))
	}

	//nolint:gosec // The CLI intentionally talks to a user-configured Open Accounting base URL.
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request POST /api/v1/tenants/%s/documents: %w", tenantID, err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, decodeAPIError(resp)
	}

	var doc documents.Document
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &doc, nil
}

func (c *apiClient) markDocumentReviewed(ctx context.Context, tenantID, documentID string) (*documents.Document, error) {
	var resp documents.Document
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "documents", documentID, "mark-reviewed"), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) deleteDocument(ctx context.Context, tenantID, documentID string) error {
	return c.request(ctx, http.MethodDelete, path.Join("/api/v1/tenants", tenantID, "documents", documentID), nil, c.apiToken, nil)
}

func (c *apiClient) request(ctx context.Context, method, apiPath string, body any, bearerToken string, out any) error {
	fullURL := c.baseURL + apiPath

	var bodyReader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encode request body: %w", err)
		}
		bodyReader = bytes.NewReader(payload)
	}

	req, err := http.NewRequestWithContext(ctx, method, fullURL, bodyReader)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if strings.TrimSpace(bearerToken) != "" {
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(bearerToken))
	}

	//nolint:gosec // The CLI intentionally talks to a user-configured Open Accounting base URL.
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request %s %s: %w", method, apiPath, err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return decodeAPIError(resp)
	}

	if out == nil {
		return nil
	}

	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

func (c *apiClient) requestRaw(ctx context.Context, method, apiPath string, body any, bearerToken string) ([]byte, error) {
	fullURL := c.baseURL + apiPath

	var bodyReader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("encode request body: %w", err)
		}
		bodyReader = bytes.NewReader(payload)
	}

	req, err := http.NewRequestWithContext(ctx, method, fullURL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "*/*")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if strings.TrimSpace(bearerToken) != "" {
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(bearerToken))
	}

	//nolint:gosec // The CLI intentionally talks to a user-configured Open Accounting base URL.
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request %s %s: %w", method, apiPath, err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, decodeAPIError(resp)
	}

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	return content, nil
}

func decodeAPIError(resp *http.Response) error {
	var payload map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&payload); err == nil {
		if message, ok := payload["error"].(string); ok && strings.TrimSpace(message) != "" {
			return fmt.Errorf("%s", message)
		}
	}
	return fmt.Errorf("request failed with status %s", resp.Status)
}

func parseDaysToExpiry(days int) *time.Time {
	if days <= 0 {
		return nil
	}
	expiresAt := time.Now().Add(time.Duration(days) * 24 * time.Hour)
	return &expiresAt
}

func parseOptionalInt(value string) (int, error) {
	if strings.TrimSpace(value) == "" {
		return 0, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("parse integer %q: %w", value, err)
	}
	return parsed, nil
}

func withQuery(apiPath string, values url.Values) string {
	if encoded := values.Encode(); encoded != "" {
		return apiPath + "?" + encoded
	}
	return apiPath
}
