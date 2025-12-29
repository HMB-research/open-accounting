import { browser } from '$app/environment';
import Decimal from 'decimal.js';

const API_BASE = import.meta.env.PUBLIC_API_URL || 'http://localhost:8080';

interface TokenResponse {
	access_token: string;
	refresh_token: string;
	token_type: string;
	expires_in: number;
	user: {
		id: string;
		email: string;
		name: string;
	};
}

interface ApiError {
	error: string;
}

class ApiClient {
	private accessToken: string | null = null;
	private refreshToken: string | null = null;

	constructor() {
		if (browser) {
			this.accessToken = localStorage.getItem('access_token');
			this.refreshToken = localStorage.getItem('refresh_token');
		}
	}

	setTokens(access: string, refresh: string) {
		this.accessToken = access;
		this.refreshToken = refresh;
		if (browser) {
			localStorage.setItem('access_token', access);
			localStorage.setItem('refresh_token', refresh);
		}
	}

	clearTokens() {
		this.accessToken = null;
		this.refreshToken = null;
		if (browser) {
			localStorage.removeItem('access_token');
			localStorage.removeItem('refresh_token');
		}
	}

	get isAuthenticated(): boolean {
		return !!this.accessToken;
	}

	private async request<T>(
		method: string,
		path: string,
		body?: unknown,
		skipAuth = false
	): Promise<T> {
		const headers: Record<string, string> = {
			'Content-Type': 'application/json'
		};

		if (!skipAuth && this.accessToken) {
			headers['Authorization'] = `Bearer ${this.accessToken}`;
		}

		const response = await fetch(`${API_BASE}${path}`, {
			method,
			headers,
			body: body ? JSON.stringify(body) : undefined
		});

		// Handle token refresh on 401
		if (response.status === 401 && this.refreshToken && !skipAuth) {
			const refreshed = await this.refreshAccessToken();
			if (refreshed) {
				return this.request(method, path, body, false);
			}
			throw new Error('Session expired. Please log in again.');
		}

		const data = await response.json();

		if (!response.ok) {
			throw new Error((data as ApiError).error || 'Request failed');
		}

		return this.parseDecimals(data) as T;
	}

	private parseDecimals(obj: unknown): unknown {
		if (typeof obj === 'string' && /^-?\d+(\.\d+)?$/.test(obj)) {
			return new Decimal(obj);
		}
		if (Array.isArray(obj)) {
			return obj.map((item) => this.parseDecimals(item));
		}
		if (obj !== null && typeof obj === 'object') {
			const result: Record<string, unknown> = {};
			for (const [key, value] of Object.entries(obj)) {
				result[key] = this.parseDecimals(value);
			}
			return result;
		}
		return obj;
	}

	private async refreshAccessToken(): Promise<boolean> {
		try {
			const data = await this.request<{ access_token: string }>(
				'POST',
				'/api/v1/auth/refresh',
				{ refresh_token: this.refreshToken },
				true
			);
			this.accessToken = data.access_token;
			if (browser) {
				localStorage.setItem('access_token', data.access_token);
			}
			return true;
		} catch {
			this.clearTokens();
			return false;
		}
	}

	// Auth endpoints
	async register(email: string, password: string, name: string) {
		return this.request<{ id: string; email: string; name: string }>(
			'POST',
			'/api/v1/auth/register',
			{ email, password, name },
			true
		);
	}

	async login(email: string, password: string, tenantId?: string): Promise<TokenResponse> {
		const data = await this.request<TokenResponse>(
			'POST',
			'/api/v1/auth/login',
			{ email, password, tenant_id: tenantId },
			true
		);
		this.setTokens(data.access_token, data.refresh_token);
		return data;
	}

	logout() {
		this.clearTokens();
	}

	// User endpoints
	async getCurrentUser() {
		return this.request<{ id: string; email: string; name: string; created_at: string }>(
			'GET',
			'/api/v1/me'
		);
	}

	async getMyTenants() {
		return this.request<TenantMembership[]>('GET', '/api/v1/me/tenants');
	}

	// Tenant endpoints
	async createTenant(name: string, slug: string, settings?: TenantSettings) {
		return this.request<Tenant>('POST', '/api/v1/tenants', { name, slug, settings });
	}

	async getTenant(tenantId: string) {
		return this.request<Tenant>('GET', `/api/v1/tenants/${tenantId}`);
	}

	// Account endpoints
	async listAccounts(tenantId: string, activeOnly = false) {
		const query = activeOnly ? '?active_only=true' : '';
		return this.request<Account[]>('GET', `/api/v1/tenants/${tenantId}/accounts${query}`);
	}

	async createAccount(tenantId: string, data: CreateAccountRequest) {
		return this.request<Account>('POST', `/api/v1/tenants/${tenantId}/accounts`, data);
	}

	async getAccount(tenantId: string, accountId: string) {
		return this.request<Account>('GET', `/api/v1/tenants/${tenantId}/accounts/${accountId}`);
	}

	// Journal entry endpoints
	async getJournalEntry(tenantId: string, entryId: string) {
		return this.request<JournalEntry>(
			'GET',
			`/api/v1/tenants/${tenantId}/journal-entries/${entryId}`
		);
	}

	async createJournalEntry(tenantId: string, data: CreateJournalEntryRequest) {
		return this.request<JournalEntry>('POST', `/api/v1/tenants/${tenantId}/journal-entries`, data);
	}

	async postJournalEntry(tenantId: string, entryId: string) {
		return this.request<{ status: string }>(
			'POST',
			`/api/v1/tenants/${tenantId}/journal-entries/${entryId}/post`
		);
	}

	async voidJournalEntry(tenantId: string, entryId: string, reason: string) {
		return this.request<JournalEntry>(
			'POST',
			`/api/v1/tenants/${tenantId}/journal-entries/${entryId}/void`,
			{ reason }
		);
	}

	// Report endpoints
	async getTrialBalance(tenantId: string, asOfDate?: string) {
		const query = asOfDate ? `?as_of_date=${asOfDate}` : '';
		return this.request<TrialBalance>(
			'GET',
			`/api/v1/tenants/${tenantId}/reports/trial-balance${query}`
		);
	}

	async getAccountBalance(tenantId: string, accountId: string, asOfDate?: string) {
		const query = asOfDate ? `?as_of_date=${asOfDate}` : '';
		return this.request<{ account_id: string; as_of_date: string; balance: Decimal }>(
			'GET',
			`/api/v1/tenants/${tenantId}/reports/account-balance/${accountId}${query}`
		);
	}

	async getBalanceSheet(tenantId: string, asOfDate?: string) {
		const query = asOfDate ? `?as_of=${asOfDate}` : '';
		return this.request<BalanceSheet>(
			'GET',
			`/api/v1/tenants/${tenantId}/reports/balance-sheet${query}`
		);
	}

	async getIncomeStatement(tenantId: string, startDate: string, endDate: string) {
		return this.request<IncomeStatement>(
			'GET',
			`/api/v1/tenants/${tenantId}/reports/income-statement?start=${startDate}&end=${endDate}`
		);
	}

	// Contact endpoints
	async listContacts(tenantId: string, filter?: ContactFilter) {
		const params = new URLSearchParams();
		if (filter?.type) params.set('type', filter.type);
		if (filter?.active_only) params.set('active_only', 'true');
		if (filter?.search) params.set('search', filter.search);
		const query = params.toString() ? `?${params.toString()}` : '';
		return this.request<Contact[]>('GET', `/api/v1/tenants/${tenantId}/contacts${query}`);
	}

	async createContact(tenantId: string, data: CreateContactRequest) {
		return this.request<Contact>('POST', `/api/v1/tenants/${tenantId}/contacts`, data);
	}

	async getContact(tenantId: string, contactId: string) {
		return this.request<Contact>('GET', `/api/v1/tenants/${tenantId}/contacts/${contactId}`);
	}

	async updateContact(tenantId: string, contactId: string, data: UpdateContactRequest) {
		return this.request<Contact>('PUT', `/api/v1/tenants/${tenantId}/contacts/${contactId}`, data);
	}

	async deleteContact(tenantId: string, contactId: string) {
		return this.request<{ status: string }>(
			'DELETE',
			`/api/v1/tenants/${tenantId}/contacts/${contactId}`
		);
	}

	// Invoice endpoints
	async listInvoices(tenantId: string, filter?: InvoiceFilter) {
		const params = new URLSearchParams();
		if (filter?.type) params.set('type', filter.type);
		if (filter?.status) params.set('status', filter.status);
		if (filter?.contact_id) params.set('contact_id', filter.contact_id);
		if (filter?.from_date) params.set('from_date', filter.from_date);
		if (filter?.to_date) params.set('to_date', filter.to_date);
		if (filter?.search) params.set('search', filter.search);
		const query = params.toString() ? `?${params.toString()}` : '';
		return this.request<Invoice[]>('GET', `/api/v1/tenants/${tenantId}/invoices${query}`);
	}

	async createInvoice(tenantId: string, data: CreateInvoiceRequest) {
		return this.request<Invoice>('POST', `/api/v1/tenants/${tenantId}/invoices`, data);
	}

	async getInvoice(tenantId: string, invoiceId: string) {
		return this.request<Invoice>('GET', `/api/v1/tenants/${tenantId}/invoices/${invoiceId}`);
	}

	async sendInvoice(tenantId: string, invoiceId: string) {
		return this.request<{ status: string }>(
			'POST',
			`/api/v1/tenants/${tenantId}/invoices/${invoiceId}/send`
		);
	}

	async voidInvoice(tenantId: string, invoiceId: string) {
		return this.request<{ status: string }>(
			'POST',
			`/api/v1/tenants/${tenantId}/invoices/${invoiceId}/void`
		);
	}

	async downloadInvoicePDF(tenantId: string, invoiceId: string, invoiceNumber: string) {
		const headers: Record<string, string> = {};
		if (this.accessToken) {
			headers['Authorization'] = `Bearer ${this.accessToken}`;
		}

		const response = await fetch(
			`${API_BASE}/api/v1/tenants/${tenantId}/invoices/${invoiceId}/pdf`,
			{ headers }
		);

		if (!response.ok) {
			throw new Error('Failed to download PDF');
		}

		const blob = await response.blob();
		const url = window.URL.createObjectURL(blob);
		const a = document.createElement('a');
		a.href = url;
		a.download = `invoice-${invoiceNumber}.pdf`;
		document.body.appendChild(a);
		a.click();
		document.body.removeChild(a);
		window.URL.revokeObjectURL(url);
	}

	// Payment endpoints
	async listPayments(tenantId: string, filter?: PaymentFilter) {
		const params = new URLSearchParams();
		if (filter?.type) params.set('type', filter.type);
		if (filter?.contact_id) params.set('contact_id', filter.contact_id);
		if (filter?.from_date) params.set('from_date', filter.from_date);
		if (filter?.to_date) params.set('to_date', filter.to_date);
		const query = params.toString() ? `?${params.toString()}` : '';
		return this.request<Payment[]>('GET', `/api/v1/tenants/${tenantId}/payments${query}`);
	}

	async createPayment(tenantId: string, data: CreatePaymentRequest) {
		return this.request<Payment>('POST', `/api/v1/tenants/${tenantId}/payments`, data);
	}

	async getPayment(tenantId: string, paymentId: string) {
		return this.request<Payment>('GET', `/api/v1/tenants/${tenantId}/payments/${paymentId}`);
	}

	async allocatePayment(tenantId: string, paymentId: string, invoiceId: string, amount: string) {
		return this.request<{ status: string }>(
			'POST',
			`/api/v1/tenants/${tenantId}/payments/${paymentId}/allocate`,
			{ invoice_id: invoiceId, amount }
		);
	}

	async getUnallocatedPayments(tenantId: string, type: 'RECEIVED' | 'MADE' = 'RECEIVED') {
		return this.request<Payment[]>(
			'GET',
			`/api/v1/tenants/${tenantId}/payments/unallocated?type=${type}`
		);
	}

	// Analytics endpoints
	async getDashboardSummary(tenantId: string) {
		return this.request<DashboardSummary>('GET', `/api/v1/tenants/${tenantId}/analytics/dashboard`);
	}

	async getRevenueExpenseChart(tenantId: string, months = 12) {
		return this.request<RevenueExpenseChart>(
			'GET',
			`/api/v1/tenants/${tenantId}/analytics/revenue-expense?months=${months}`
		);
	}

	async getCashFlowChart(tenantId: string, months = 12) {
		return this.request<CashFlowChart>(
			'GET',
			`/api/v1/tenants/${tenantId}/analytics/cash-flow?months=${months}`
		);
	}

	async getReceivablesAging(tenantId: string) {
		return this.request<AgingReport>('GET', `/api/v1/tenants/${tenantId}/reports/aging/receivables`);
	}

	async getPayablesAging(tenantId: string) {
		return this.request<AgingReport>('GET', `/api/v1/tenants/${tenantId}/reports/aging/payables`);
	}

	// Recurring Invoice endpoints
	async listRecurringInvoices(tenantId: string, activeOnly = false) {
		const query = activeOnly ? '?active_only=true' : '';
		return this.request<RecurringInvoice[]>(
			'GET',
			`/api/v1/tenants/${tenantId}/recurring-invoices${query}`
		);
	}

	async createRecurringInvoice(tenantId: string, data: CreateRecurringInvoiceRequest) {
		return this.request<RecurringInvoice>(
			'POST',
			`/api/v1/tenants/${tenantId}/recurring-invoices`,
			data
		);
	}

	async createRecurringInvoiceFromInvoice(
		tenantId: string,
		invoiceId: string,
		data: CreateFromInvoiceRequest
	) {
		return this.request<RecurringInvoice>(
			'POST',
			`/api/v1/tenants/${tenantId}/recurring-invoices/from-invoice/${invoiceId}`,
			data
		);
	}

	async getRecurringInvoice(tenantId: string, recurringId: string) {
		return this.request<RecurringInvoice>(
			'GET',
			`/api/v1/tenants/${tenantId}/recurring-invoices/${recurringId}`
		);
	}

	async updateRecurringInvoice(
		tenantId: string,
		recurringId: string,
		data: UpdateRecurringInvoiceRequest
	) {
		return this.request<RecurringInvoice>(
			'PUT',
			`/api/v1/tenants/${tenantId}/recurring-invoices/${recurringId}`,
			data
		);
	}

	async deleteRecurringInvoice(tenantId: string, recurringId: string) {
		return this.request<{ status: string }>(
			'DELETE',
			`/api/v1/tenants/${tenantId}/recurring-invoices/${recurringId}`
		);
	}

	async pauseRecurringInvoice(tenantId: string, recurringId: string) {
		return this.request<{ status: string }>(
			'POST',
			`/api/v1/tenants/${tenantId}/recurring-invoices/${recurringId}/pause`
		);
	}

	async resumeRecurringInvoice(tenantId: string, recurringId: string) {
		return this.request<{ status: string }>(
			'POST',
			`/api/v1/tenants/${tenantId}/recurring-invoices/${recurringId}/resume`
		);
	}

	async generateRecurringInvoice(tenantId: string, recurringId: string) {
		return this.request<GenerationResult>(
			'POST',
			`/api/v1/tenants/${tenantId}/recurring-invoices/${recurringId}/generate`
		);
	}

	async generateDueRecurringInvoices(tenantId: string) {
		return this.request<GenerationResult[]>(
			'POST',
			`/api/v1/tenants/${tenantId}/recurring-invoices/generate-due`
		);
	}

	// Email endpoints
	async getSMTPConfig(tenantId: string) {
		return this.request<SMTPConfig>('GET', `/api/v1/tenants/${tenantId}/settings/smtp`);
	}

	async updateSMTPConfig(tenantId: string, data: UpdateSMTPConfigRequest) {
		return this.request<{ status: string }>(
			'PUT',
			`/api/v1/tenants/${tenantId}/settings/smtp`,
			data
		);
	}

	async testSMTP(tenantId: string, recipientEmail: string) {
		return this.request<TestSMTPResponse>(
			'POST',
			`/api/v1/tenants/${tenantId}/settings/smtp/test`,
			{ recipient_email: recipientEmail }
		);
	}

	async listEmailTemplates(tenantId: string) {
		return this.request<EmailTemplate[]>('GET', `/api/v1/tenants/${tenantId}/email-templates`);
	}

	async updateEmailTemplate(tenantId: string, templateType: TemplateType, data: UpdateTemplateRequest) {
		return this.request<EmailTemplate>(
			'PUT',
			`/api/v1/tenants/${tenantId}/email-templates/${templateType}`,
			data
		);
	}

	async getEmailLog(tenantId: string, limit = 50) {
		return this.request<EmailLog[]>(
			'GET',
			`/api/v1/tenants/${tenantId}/email-log?limit=${limit}`
		);
	}

	async emailInvoice(tenantId: string, invoiceId: string, data: SendInvoiceEmailRequest) {
		return this.request<EmailSentResponse>(
			'POST',
			`/api/v1/tenants/${tenantId}/invoices/${invoiceId}/email`,
			data
		);
	}

	async emailPaymentReceipt(tenantId: string, paymentId: string, data: SendPaymentReceiptRequest) {
		return this.request<EmailSentResponse>(
			'POST',
			`/api/v1/tenants/${tenantId}/payments/${paymentId}/email-receipt`,
			data
		);
	}

	// Banking endpoints
	async listBankAccounts(tenantId: string, activeOnly = false) {
		const query = activeOnly ? '?active_only=true' : '';
		return this.request<BankAccount[]>('GET', `/api/v1/tenants/${tenantId}/bank-accounts${query}`);
	}

	async createBankAccount(tenantId: string, data: CreateBankAccountRequest) {
		return this.request<BankAccount>('POST', `/api/v1/tenants/${tenantId}/bank-accounts`, data);
	}

	async getBankAccount(tenantId: string, accountId: string) {
		return this.request<BankAccount>('GET', `/api/v1/tenants/${tenantId}/bank-accounts/${accountId}`);
	}

	async updateBankAccount(tenantId: string, accountId: string, data: UpdateBankAccountRequest) {
		return this.request<BankAccount>(
			'PUT',
			`/api/v1/tenants/${tenantId}/bank-accounts/${accountId}`,
			data
		);
	}

	async deleteBankAccount(tenantId: string, accountId: string) {
		return this.request<void>('DELETE', `/api/v1/tenants/${tenantId}/bank-accounts/${accountId}`);
	}

	async listBankTransactions(
		tenantId: string,
		accountId: string,
		filters?: { status?: TransactionStatus; from_date?: string; to_date?: string }
	) {
		const params = new URLSearchParams();
		if (filters?.status) params.set('status', filters.status);
		if (filters?.from_date) params.set('from_date', filters.from_date);
		if (filters?.to_date) params.set('to_date', filters.to_date);
		const query = params.toString() ? `?${params.toString()}` : '';
		return this.request<BankTransaction[]>(
			'GET',
			`/api/v1/tenants/${tenantId}/bank-accounts/${accountId}/transactions${query}`
		);
	}

	async getBankTransaction(tenantId: string, transactionId: string) {
		return this.request<BankTransaction>(
			'GET',
			`/api/v1/tenants/${tenantId}/bank-transactions/${transactionId}`
		);
	}

	async importBankTransactions(tenantId: string, accountId: string, data: ImportTransactionsRequest) {
		return this.request<ImportResult>(
			'POST',
			`/api/v1/tenants/${tenantId}/bank-accounts/${accountId}/import`,
			data
		);
	}

	async getImportHistory(tenantId: string, accountId: string) {
		return this.request<BankStatementImport[]>(
			'GET',
			`/api/v1/tenants/${tenantId}/bank-accounts/${accountId}/import-history`
		);
	}

	async getMatchSuggestions(tenantId: string, transactionId: string) {
		return this.request<MatchSuggestion[]>(
			'GET',
			`/api/v1/tenants/${tenantId}/bank-transactions/${transactionId}/suggestions`
		);
	}

	async matchBankTransaction(tenantId: string, transactionId: string, paymentId: string) {
		return this.request<{ status: string }>(
			'POST',
			`/api/v1/tenants/${tenantId}/bank-transactions/${transactionId}/match`,
			{ payment_id: paymentId }
		);
	}

	async unmatchBankTransaction(tenantId: string, transactionId: string) {
		return this.request<{ status: string }>(
			'POST',
			`/api/v1/tenants/${tenantId}/bank-transactions/${transactionId}/unmatch`
		);
	}

	async createPaymentFromTransaction(tenantId: string, transactionId: string) {
		return this.request<{ payment_id: string }>(
			'POST',
			`/api/v1/tenants/${tenantId}/bank-transactions/${transactionId}/create-payment`
		);
	}

	async listReconciliations(tenantId: string, accountId: string) {
		return this.request<BankReconciliation[]>(
			'GET',
			`/api/v1/tenants/${tenantId}/bank-accounts/${accountId}/reconciliations`
		);
	}

	async createReconciliation(tenantId: string, accountId: string, data: CreateReconciliationRequest) {
		return this.request<BankReconciliation>(
			'POST',
			`/api/v1/tenants/${tenantId}/bank-accounts/${accountId}/reconciliation`,
			data
		);
	}

	async getReconciliation(tenantId: string, reconciliationId: string) {
		return this.request<BankReconciliation>(
			'GET',
			`/api/v1/tenants/${tenantId}/reconciliations/${reconciliationId}`
		);
	}

	async completeReconciliation(tenantId: string, reconciliationId: string) {
		return this.request<{ status: string }>(
			'POST',
			`/api/v1/tenants/${tenantId}/reconciliations/${reconciliationId}/complete`
		);
	}

	async autoMatchTransactions(tenantId: string, accountId: string, minConfidence = 0.7) {
		return this.request<{ matched: number }>(
			'POST',
			`/api/v1/tenants/${tenantId}/bank-accounts/${accountId}/auto-match?min_confidence=${minConfidence}`
		);
	}

	// Tax (KMD) endpoints
	async generateKMD(tenantId: string, data: CreateKMDRequest) {
		return this.request<KMDDeclaration>('POST', `/api/v1/tenants/${tenantId}/tax/kmd`, data);
	}

	async listKMD(tenantId: string) {
		return this.request<KMDDeclaration[]>('GET', `/api/v1/tenants/${tenantId}/tax/kmd`);
	}

	async downloadKMDXml(tenantId: string, year: number, month: number) {
		const headers: Record<string, string> = {};
		if (this.accessToken) {
			headers['Authorization'] = `Bearer ${this.accessToken}`;
		}

		const response = await fetch(
			`${API_BASE}/api/v1/tenants/${tenantId}/tax/kmd/${year}/${month}/xml`,
			{ headers }
		);

		if (!response.ok) {
			throw new Error('Failed to download XML');
		}

		const blob = await response.blob();
		const url = window.URL.createObjectURL(blob);
		const a = document.createElement('a');
		a.href = url;
		a.download = `KMD_${year}_${String(month).padStart(2, '0')}.xml`;
		document.body.appendChild(a);
		a.click();
		document.body.removeChild(a);
		window.URL.revokeObjectURL(url);
	}

	// Payroll - Employee endpoints
	async listEmployees(tenantId: string, activeOnly = false) {
		const query = activeOnly ? '?active_only=true' : '';
		return this.request<Employee[]>('GET', `/api/v1/tenants/${tenantId}/employees${query}`);
	}

	async createEmployee(tenantId: string, data: CreateEmployeeRequest) {
		return this.request<Employee>('POST', `/api/v1/tenants/${tenantId}/employees`, data);
	}

	async getEmployee(tenantId: string, employeeId: string) {
		return this.request<Employee>('GET', `/api/v1/tenants/${tenantId}/employees/${employeeId}`);
	}

	async updateEmployee(tenantId: string, employeeId: string, data: UpdateEmployeeRequest) {
		return this.request<Employee>(
			'PUT',
			`/api/v1/tenants/${tenantId}/employees/${employeeId}`,
			data
		);
	}

	async setBaseSalary(tenantId: string, employeeId: string, amount: string) {
		return this.request<{ status: string }>(
			'POST',
			`/api/v1/tenants/${tenantId}/employees/${employeeId}/salary`,
			{ amount }
		);
	}

	// Payroll - Payroll Run endpoints
	async listPayrollRuns(tenantId: string, year?: number) {
		const query = year ? `?year=${year}` : '';
		return this.request<PayrollRun[]>('GET', `/api/v1/tenants/${tenantId}/payroll-runs${query}`);
	}

	async createPayrollRun(tenantId: string, data: CreatePayrollRunRequest) {
		return this.request<PayrollRun>('POST', `/api/v1/tenants/${tenantId}/payroll-runs`, data);
	}

	async getPayrollRun(tenantId: string, runId: string) {
		return this.request<PayrollRun>('GET', `/api/v1/tenants/${tenantId}/payroll-runs/${runId}`);
	}

	async calculatePayroll(tenantId: string, runId: string) {
		return this.request<PayrollRun>(
			'POST',
			`/api/v1/tenants/${tenantId}/payroll-runs/${runId}/calculate`
		);
	}

	async approvePayroll(tenantId: string, runId: string) {
		return this.request<PayrollRun>(
			'POST',
			`/api/v1/tenants/${tenantId}/payroll-runs/${runId}/approve`
		);
	}

	async getPayslips(tenantId: string, runId: string) {
		return this.request<Payslip[]>(
			'GET',
			`/api/v1/tenants/${tenantId}/payroll-runs/${runId}/payslips`
		);
	}

	async generateTSD(tenantId: string, runId: string) {
		return this.request<TSDDeclaration>(
			'POST',
			`/api/v1/tenants/${tenantId}/payroll-runs/${runId}/tsd`
		);
	}

	// Payroll - Tax Preview
	async calculateTaxPreview(tenantId: string, grossSalary: string, basicExemption?: string, fundedPensionRate?: string) {
		return this.request<TaxCalculation>(
			'POST',
			`/api/v1/tenants/${tenantId}/payroll/tax-preview`,
			{ gross_salary: grossSalary, basic_exemption: basicExemption, funded_pension_rate: fundedPensionRate }
		);
	}

	// Payroll - TSD endpoints
	async listTSD(tenantId: string, year?: number) {
		const query = year ? `?year=${year}` : '';
		return this.request<TSDDeclaration[]>('GET', `/api/v1/tenants/${tenantId}/tsd${query}`);
	}

	async getTSD(tenantId: string, year: number, month: number) {
		return this.request<TSDDeclaration>(
			'GET',
			`/api/v1/tenants/${tenantId}/tsd/${year}/${month}`
		);
	}

	async downloadTSDXml(tenantId: string, year: number, month: number) {
		const headers: Record<string, string> = {};
		if (this.accessToken) {
			headers['Authorization'] = `Bearer ${this.accessToken}`;
		}

		const response = await fetch(
			`${API_BASE}/api/v1/tenants/${tenantId}/tsd/${year}/${month}/xml`,
			{ headers }
		);

		if (!response.ok) {
			throw new Error('Failed to download TSD XML');
		}

		const blob = await response.blob();
		const url = window.URL.createObjectURL(blob);
		const a = document.createElement('a');
		a.href = url;
		a.download = `TSD_${year}_${String(month).padStart(2, '0')}.xml`;
		document.body.appendChild(a);
		a.click();
		document.body.removeChild(a);
		window.URL.revokeObjectURL(url);
	}

	async downloadTSDCsv(tenantId: string, year: number, month: number) {
		const headers: Record<string, string> = {};
		if (this.accessToken) {
			headers['Authorization'] = `Bearer ${this.accessToken}`;
		}

		const response = await fetch(
			`${API_BASE}/api/v1/tenants/${tenantId}/tsd/${year}/${month}/csv`,
			{ headers }
		);

		if (!response.ok) {
			throw new Error('Failed to download TSD CSV');
		}

		const blob = await response.blob();
		const url = window.URL.createObjectURL(blob);
		const a = document.createElement('a');
		a.href = url;
		a.download = `TSD_${year}_${String(month).padStart(2, '0')}.csv`;
		document.body.appendChild(a);
		a.click();
		document.body.removeChild(a);
		window.URL.revokeObjectURL(url);
	}

	async markTSDSubmitted(tenantId: string, year: number, month: number, emtaReference: string) {
		return this.request<{ status: string }>(
			'POST',
			`/api/v1/tenants/${tenantId}/tsd/${year}/${month}/submit`,
			{ emta_reference: emtaReference }
		);
	}
}

// Types
export interface Tenant {
	id: string;
	name: string;
	slug: string;
	schema_name: string;
	settings: TenantSettings;
	is_active: boolean;
	created_at: string;
	updated_at: string;
}

export interface TenantSettings {
	default_currency: string;
	country_code: string;
	timezone: string;
	date_format: string;
	decimal_sep: string;
	thousands_sep: string;
	fiscal_year_start_month: number;
	vat_number?: string;
	reg_code?: string;
	address?: string;
	email?: string;
	phone?: string;
	logo?: string;
}

export interface TenantMembership {
	tenant: Tenant;
	role: string;
	is_default: boolean;
}

export interface Account {
	id: string;
	tenant_id: string;
	code: string;
	name: string;
	account_type: 'ASSET' | 'LIABILITY' | 'EQUITY' | 'REVENUE' | 'EXPENSE';
	parent_id?: string;
	is_active: boolean;
	is_system: boolean;
	description?: string;
	created_at: string;
}

export interface CreateAccountRequest {
	code: string;
	name: string;
	account_type: Account['account_type'];
	parent_id?: string;
	description?: string;
}

export interface JournalEntry {
	id: string;
	tenant_id: string;
	entry_number: string;
	entry_date: string;
	description: string;
	reference?: string;
	source_type?: string;
	source_id?: string;
	status: 'DRAFT' | 'POSTED' | 'VOIDED';
	lines: JournalEntryLine[];
	posted_at?: string;
	posted_by?: string;
	voided_at?: string;
	voided_by?: string;
	void_reason?: string;
	created_at: string;
	created_by: string;
}

export interface JournalEntryLine {
	id: string;
	account_id: string;
	account?: Account;
	description?: string;
	debit_amount: Decimal;
	credit_amount: Decimal;
	currency: string;
	exchange_rate: Decimal;
	base_debit: Decimal;
	base_credit: Decimal;
}

export interface CreateJournalEntryRequest {
	entry_date: string;
	description: string;
	reference?: string;
	source_type?: string;
	source_id?: string;
	lines: {
		account_id: string;
		description?: string;
		debit_amount: string;
		credit_amount: string;
		currency?: string;
		exchange_rate?: string;
	}[];
}

export interface TrialBalance {
	tenant_id: string;
	as_of_date: string;
	generated_at: string;
	accounts: AccountBalance[];
	total_debits: Decimal;
	total_credits: Decimal;
	is_balanced: boolean;
}

export interface AccountBalance {
	account_id: string;
	account_code: string;
	account_name: string;
	account_type: Account['account_type'];
	debit_balance: Decimal;
	credit_balance: Decimal;
	net_balance: Decimal;
}

export interface BalanceSheet {
	tenant_id: string;
	as_of_date: string;
	generated_at: string;
	assets: AccountBalance[];
	liabilities: AccountBalance[];
	equity: AccountBalance[];
	total_assets: Decimal;
	total_liabilities: Decimal;
	total_equity: Decimal;
	retained_earnings: Decimal;
	is_balanced: boolean;
}

export interface IncomeStatement {
	tenant_id: string;
	start_date: string;
	end_date: string;
	generated_at: string;
	revenue: AccountBalance[];
	expenses: AccountBalance[];
	total_revenue: Decimal;
	total_expenses: Decimal;
	net_income: Decimal;
}

// Contact types
export type ContactType = 'CUSTOMER' | 'SUPPLIER' | 'BOTH';

export interface Contact {
	id: string;
	tenant_id: string;
	code?: string;
	name: string;
	contact_type: ContactType;
	reg_code?: string;
	vat_number?: string;
	email?: string;
	phone?: string;
	address_line1?: string;
	address_line2?: string;
	city?: string;
	postal_code?: string;
	country_code: string;
	payment_terms_days: number;
	credit_limit?: Decimal;
	default_account_id?: string;
	is_active: boolean;
	notes?: string;
	created_at: string;
	updated_at: string;
}

export interface CreateContactRequest {
	code?: string;
	name: string;
	contact_type: ContactType;
	reg_code?: string;
	vat_number?: string;
	email?: string;
	phone?: string;
	address_line1?: string;
	address_line2?: string;
	city?: string;
	postal_code?: string;
	country_code?: string;
	payment_terms_days?: number;
	credit_limit?: string;
	default_account_id?: string;
	notes?: string;
}

export interface UpdateContactRequest {
	name?: string;
	reg_code?: string;
	vat_number?: string;
	email?: string;
	phone?: string;
	address_line1?: string;
	address_line2?: string;
	city?: string;
	postal_code?: string;
	country_code?: string;
	payment_terms_days?: number;
	credit_limit?: string;
	default_account_id?: string;
	notes?: string;
	is_active?: boolean;
}

export interface ContactFilter {
	type?: ContactType;
	active_only?: boolean;
	search?: string;
}

// Invoice types
export type InvoiceType = 'SALES' | 'PURCHASE' | 'CREDIT_NOTE';
export type InvoiceStatus = 'DRAFT' | 'SENT' | 'PARTIALLY_PAID' | 'PAID' | 'OVERDUE' | 'VOIDED';

export interface Invoice {
	id: string;
	tenant_id: string;
	invoice_number: string;
	invoice_type: InvoiceType;
	contact_id: string;
	contact?: Contact;
	issue_date: string;
	due_date: string;
	currency: string;
	exchange_rate: Decimal;
	subtotal: Decimal;
	vat_amount: Decimal;
	total: Decimal;
	base_subtotal: Decimal;
	base_vat_amount: Decimal;
	base_total: Decimal;
	amount_paid: Decimal;
	status: InvoiceStatus;
	reference?: string;
	notes?: string;
	lines: InvoiceLine[];
	journal_entry_id?: string;
	einvoice_sent_at?: string;
	einvoice_id?: string;
	created_at: string;
	created_by: string;
	updated_at: string;
}

export interface InvoiceLine {
	id: string;
	tenant_id: string;
	invoice_id: string;
	line_number: number;
	description: string;
	quantity: Decimal;
	unit?: string;
	unit_price: Decimal;
	discount_percent: Decimal;
	vat_rate: Decimal;
	line_subtotal: Decimal;
	line_vat: Decimal;
	line_total: Decimal;
	account_id?: string;
	product_id?: string;
}

export interface CreateInvoiceRequest {
	invoice_type: InvoiceType;
	contact_id: string;
	issue_date: string;
	due_date: string;
	currency?: string;
	exchange_rate?: string;
	reference?: string;
	notes?: string;
	lines: CreateInvoiceLineRequest[];
}

export interface CreateInvoiceLineRequest {
	description: string;
	quantity: string;
	unit?: string;
	unit_price: string;
	discount_percent?: string;
	vat_rate: string;
	account_id?: string;
	product_id?: string;
}

export interface InvoiceFilter {
	type?: InvoiceType;
	status?: InvoiceStatus;
	contact_id?: string;
	from_date?: string;
	to_date?: string;
	search?: string;
}

// Payment types
export type PaymentType = 'RECEIVED' | 'MADE';

export interface Payment {
	id: string;
	tenant_id: string;
	payment_number: string;
	payment_type: PaymentType;
	contact_id?: string;
	payment_date: string;
	amount: Decimal;
	currency: string;
	exchange_rate: Decimal;
	base_amount: Decimal;
	payment_method?: string;
	bank_account?: string;
	reference?: string;
	notes?: string;
	allocations: PaymentAllocation[];
	journal_entry_id?: string;
	created_at: string;
	created_by: string;
}

export interface PaymentAllocation {
	id: string;
	tenant_id: string;
	payment_id: string;
	invoice_id: string;
	amount: Decimal;
	created_at: string;
}

export interface CreatePaymentRequest {
	payment_type: PaymentType;
	contact_id?: string;
	payment_date: string;
	amount: string;
	currency?: string;
	exchange_rate?: string;
	payment_method?: string;
	bank_account?: string;
	reference?: string;
	notes?: string;
	allocations?: AllocationRequest[];
}

export interface AllocationRequest {
	invoice_id: string;
	amount: string;
}

export interface PaymentFilter {
	type?: PaymentType;
	contact_id?: string;
	from_date?: string;
	to_date?: string;
}

// Analytics types
export interface DashboardSummary {
	total_revenue: Decimal;
	total_expenses: Decimal;
	net_income: Decimal;
	revenue_change: Decimal;
	expenses_change: Decimal;
	total_receivables: Decimal;
	total_payables: Decimal;
	overdue_receivables: Decimal;
	overdue_payables: Decimal;
	draft_invoices: number;
	pending_invoices: number;
	overdue_invoices: number;
	period_start: string;
	period_end: string;
}

export interface RevenueExpenseChart {
	labels: string[];
	revenue: Decimal[];
	expenses: Decimal[];
}

export interface CashFlowChart {
	labels: string[];
	inflows: Decimal[];
	outflows: Decimal[];
	net: Decimal[];
}

export interface AgingBucket {
	label: string;
	amount: Decimal;
	count: number;
}

export interface AgingReport {
	report_type: string;
	as_of_date: string;
	total: Decimal;
	buckets: AgingBucket[];
}

// Recurring Invoice types
export type Frequency = 'WEEKLY' | 'BIWEEKLY' | 'MONTHLY' | 'QUARTERLY' | 'YEARLY';

export interface RecurringInvoice {
	id: string;
	tenant_id: string;
	name: string;
	contact_id: string;
	contact_name?: string;
	invoice_type: string;
	currency: string;
	frequency: Frequency;
	start_date: string;
	end_date?: string;
	next_generation_date: string;
	payment_terms_days: number;
	reference?: string;
	notes?: string;
	is_active: boolean;
	last_generated_at?: string;
	generated_count: number;
	lines: RecurringInvoiceLine[];
	created_at: string;
	created_by: string;
	updated_at: string;
}

export interface RecurringInvoiceLine {
	id: string;
	recurring_invoice_id: string;
	line_number: number;
	description: string;
	quantity: Decimal;
	unit?: string;
	unit_price: Decimal;
	discount_percent: Decimal;
	vat_rate: Decimal;
	account_id?: string;
	product_id?: string;
}

export interface CreateRecurringInvoiceRequest {
	name: string;
	contact_id: string;
	invoice_type?: string;
	currency?: string;
	frequency: Frequency;
	start_date: string;
	end_date?: string;
	payment_terms_days?: number;
	reference?: string;
	notes?: string;
	lines: CreateRecurringInvoiceLineRequest[];
}

export interface CreateRecurringInvoiceLineRequest {
	description: string;
	quantity: string;
	unit?: string;
	unit_price: string;
	discount_percent?: string;
	vat_rate: string;
	account_id?: string;
	product_id?: string;
}

export interface UpdateRecurringInvoiceRequest {
	name?: string;
	contact_id?: string;
	frequency?: Frequency;
	end_date?: string;
	payment_terms_days?: number;
	reference?: string;
	notes?: string;
	lines?: CreateRecurringInvoiceLineRequest[];
}

export interface CreateFromInvoiceRequest {
	name: string;
	frequency: Frequency;
	start_date: string;
	end_date?: string;
	payment_terms_days?: number;
}

export interface GenerationResult {
	recurring_invoice_id: string;
	generated_invoice_id: string;
	generated_invoice_number: string;
}

// Email types
export type TemplateType = 'INVOICE_SEND' | 'PAYMENT_RECEIPT' | 'OVERDUE_REMINDER';
export type EmailStatus = 'PENDING' | 'SENT' | 'FAILED';

export interface SMTPConfig {
	smtp_host: string;
	smtp_port: number;
	smtp_username: string;
	smtp_password?: string;
	smtp_from_email: string;
	smtp_from_name: string;
	smtp_use_tls: boolean;
}

export interface UpdateSMTPConfigRequest {
	smtp_host: string;
	smtp_port: number;
	smtp_username: string;
	smtp_password?: string;
	smtp_from_email: string;
	smtp_from_name: string;
	smtp_use_tls: boolean;
}

export interface TestSMTPResponse {
	success: boolean;
	message: string;
}

export interface EmailTemplate {
	id: string;
	tenant_id: string;
	template_type: TemplateType;
	subject: string;
	body_html: string;
	body_text?: string;
	is_active: boolean;
	created_at: string;
	updated_at: string;
}

export interface UpdateTemplateRequest {
	subject: string;
	body_html: string;
	body_text?: string;
	is_active: boolean;
}

export interface EmailLog {
	id: string;
	tenant_id: string;
	email_type: string;
	recipient_email: string;
	recipient_name?: string;
	subject: string;
	status: EmailStatus;
	sent_at?: string;
	error_message?: string;
	related_id?: string;
	created_at: string;
}

export interface SendInvoiceEmailRequest {
	recipient_email: string;
	recipient_name?: string;
	subject?: string;
	message?: string;
	attach_pdf: boolean;
}

export interface SendPaymentReceiptRequest {
	recipient_email: string;
	recipient_name?: string;
	subject?: string;
	message?: string;
}

export interface EmailSentResponse {
	success: boolean;
	log_id: string;
	message: string;
}

// Banking types
export type TransactionStatus = 'UNMATCHED' | 'MATCHED' | 'RECONCILED';
export type ReconciliationStatus = 'IN_PROGRESS' | 'COMPLETED';

export interface BankAccount {
	id: string;
	tenant_id: string;
	name: string;
	account_number: string;
	bank_name?: string;
	currency: string;
	opening_balance: Decimal;
	current_balance: Decimal;
	gl_account_id?: string;
	is_active: boolean;
	created_at: string;
	updated_at: string;
}

export interface BankTransaction {
	id: string;
	tenant_id: string;
	bank_account_id: string;
	transaction_date: string;
	value_date?: string;
	description: string;
	reference?: string;
	amount: Decimal;
	currency: string;
	counterparty_name?: string;
	counterparty_account?: string;
	status: TransactionStatus;
	matched_payment_id?: string;
	reconciliation_id?: string;
	import_id?: string;
	created_at: string;
}

export interface BankReconciliation {
	id: string;
	tenant_id: string;
	bank_account_id: string;
	statement_date: string;
	opening_balance: Decimal;
	closing_balance: Decimal;
	calculated_balance?: Decimal;
	difference?: Decimal;
	status: ReconciliationStatus;
	completed_at?: string;
	completed_by?: string;
	transactions_matched: number;
	transactions_unmatched: number;
	created_at: string;
	created_by: string;
}

export interface BankStatementImport {
	id: string;
	tenant_id: string;
	bank_account_id: string;
	file_name: string;
	transactions_imported: number;
	transactions_matched: number;
	transactions_duplicates: number;
	created_at: string;
	created_by: string;
}

export interface MatchSuggestion {
	payment_id: string;
	payment_number: string;
	payment_date: string;
	amount: Decimal;
	contact_name?: string;
	reference?: string;
	confidence: number;
	match_reason: string;
}

export interface CreateBankAccountRequest {
	name: string;
	account_number: string;
	bank_name?: string;
	currency?: string;
	opening_balance?: string;
	gl_account_id?: string;
}

export interface UpdateBankAccountRequest {
	name?: string;
	bank_name?: string;
	gl_account_id?: string;
	is_active?: boolean;
}

export interface ImportTransactionsRequest {
	csv_content: string;
	file_name: string;
	mapping: CSVColumnMapping;
	skip_duplicates?: boolean;
}

export interface CSVColumnMapping {
	date_column: number;
	description_column: number;
	amount_column: number;
	reference_column?: number;
	counterparty_column?: number;
	date_format: string;
	decimal_separator: string;
	thousands_separator?: string;
	skip_header: boolean;
}

export interface ImportResult {
	import_id: string;
	transactions_imported: number;
	transactions_duplicates: number;
	errors: string[];
}

export interface CreateReconciliationRequest {
	statement_date: string;
	opening_balance: string;
	closing_balance: string;
}

// Tax (KMD) types
export interface KMDRow {
	code: string;
	description: string;
	tax_base: Decimal;
	tax_amount: Decimal;
}

export interface KMDDeclaration {
	id: string;
	tenant_id: string;
	year: number;
	month: number;
	status: 'DRAFT' | 'SUBMITTED' | 'ACCEPTED';
	total_output_vat: Decimal;
	total_input_vat: Decimal;
	rows: KMDRow[];
	submitted_at?: string;
	created_at: string;
	updated_at: string;
}

export interface CreateKMDRequest {
	year: number;
	month: number;
}

// Payroll types
export type EmploymentType = 'FULL_TIME' | 'PART_TIME' | 'CONTRACT';
export type PayrollStatus = 'DRAFT' | 'CALCULATED' | 'APPROVED' | 'PAID' | 'DECLARED';
export type TSDStatus = 'DRAFT' | 'SUBMITTED' | 'ACCEPTED' | 'REJECTED';

export interface Employee {
	id: string;
	tenant_id: string;
	employee_number?: string;
	first_name: string;
	last_name: string;
	personal_code?: string;
	email?: string;
	phone?: string;
	address?: string;
	bank_account?: string;
	start_date: string;
	end_date?: string;
	position?: string;
	department?: string;
	employment_type: EmploymentType;
	tax_residency: string;
	apply_basic_exemption: boolean;
	basic_exemption_amount: Decimal;
	funded_pension_rate: Decimal;
	is_active: boolean;
	created_at: string;
	updated_at: string;
}

export interface CreateEmployeeRequest {
	employee_number?: string;
	first_name: string;
	last_name: string;
	personal_code?: string;
	email?: string;
	phone?: string;
	address?: string;
	bank_account?: string;
	start_date: string;
	position?: string;
	department?: string;
	employment_type: EmploymentType;
	apply_basic_exemption: boolean;
	basic_exemption_amount?: string;
	funded_pension_rate?: string;
}

export interface UpdateEmployeeRequest {
	employee_number?: string;
	first_name?: string;
	last_name?: string;
	personal_code?: string;
	email?: string;
	phone?: string;
	address?: string;
	bank_account?: string;
	end_date?: string;
	position?: string;
	department?: string;
	employment_type?: EmploymentType;
	apply_basic_exemption?: boolean;
	basic_exemption_amount?: string;
	funded_pension_rate?: string;
	is_active?: boolean;
}

export interface SalaryComponent {
	id: string;
	tenant_id: string;
	employee_id: string;
	component_type: string;
	name: string;
	amount: Decimal;
	is_taxable: boolean;
	is_recurring: boolean;
	effective_from: string;
	effective_to?: string;
	created_at: string;
}

export interface PayrollRun {
	id: string;
	tenant_id: string;
	period_year: number;
	period_month: number;
	status: PayrollStatus;
	payment_date?: string;
	total_gross: Decimal;
	total_net: Decimal;
	total_employer_cost: Decimal;
	notes?: string;
	created_by?: string;
	approved_by?: string;
	approved_at?: string;
	created_at: string;
	updated_at: string;
	payslips?: Payslip[];
}

export interface CreatePayrollRunRequest {
	period_year: number;
	period_month: number;
	payment_date?: string;
	notes?: string;
}

export interface Payslip {
	id: string;
	tenant_id: string;
	payroll_run_id: string;
	employee_id: string;
	gross_salary: Decimal;
	taxable_income: Decimal;
	income_tax: Decimal;
	unemployment_insurance_employee: Decimal;
	funded_pension: Decimal;
	other_deductions: Decimal;
	net_salary: Decimal;
	social_tax: Decimal;
	unemployment_insurance_employer: Decimal;
	total_employer_cost: Decimal;
	basic_exemption_applied: Decimal;
	payment_status: string;
	paid_at?: string;
	created_at: string;
	employee?: Employee;
}

export interface TSDDeclaration {
	id: string;
	tenant_id: string;
	period_year: number;
	period_month: number;
	payroll_run_id?: string;
	total_payments: Decimal;
	total_income_tax: Decimal;
	total_social_tax: Decimal;
	total_unemployment_employer: Decimal;
	total_unemployment_employee: Decimal;
	total_funded_pension: Decimal;
	status: TSDStatus;
	submitted_at?: string;
	emta_reference?: string;
	created_at: string;
	updated_at: string;
	rows?: TSDRow[];
}

export interface TSDRow {
	id: string;
	tenant_id: string;
	declaration_id: string;
	employee_id: string;
	personal_code: string;
	first_name: string;
	last_name: string;
	payment_type: string;
	gross_payment: Decimal;
	basic_exemption: Decimal;
	taxable_amount: Decimal;
	income_tax: Decimal;
	social_tax: Decimal;
	unemployment_insurance_employer: Decimal;
	unemployment_insurance_employee: Decimal;
	funded_pension: Decimal;
	created_at: string;
}

export interface TaxCalculation {
	gross_salary: Decimal;
	basic_exemption: Decimal;
	taxable_income: Decimal;
	income_tax: Decimal;
	unemployment_employee: Decimal;
	funded_pension: Decimal;
	total_deductions: Decimal;
	net_salary: Decimal;
	social_tax: Decimal;
	unemployment_employer: Decimal;
	total_employer_cost: Decimal;
}

export const api = new ApiClient();
