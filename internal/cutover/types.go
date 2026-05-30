package cutover

type FileKind string

const (
	KindAccounts          FileKind = "accounts"
	KindContacts          FileKind = "contacts"
	KindEmployees         FileKind = "employees"
	KindExpenses          FileKind = "expenses"
	KindInvoices          FileKind = "invoices"
	KindPayments          FileKind = "payments"
	KindBankAccounts      FileKind = "bank_accounts"
	KindBankTransactions  FileKind = "bank_transactions"
	KindPayrollHistory    FileKind = "payroll_history"
	KindLeaveBalances     FileKind = "leave_balances"
	KindKMDHistory        FileKind = "kmd_history"
	KindQuotes            FileKind = "quotes"
	KindOrders            FileKind = "orders"
	KindRecurringInvoices FileKind = "recurring_invoices"
	KindCostCenters       FileKind = "cost_centers"
	KindProductCategories FileKind = "product_categories"
	KindWarehouses        FileKind = "warehouses"
	KindProducts          FileKind = "products"
	KindStockAdjustments  FileKind = "stock_adjustments"
	KindFixedAssets       FileKind = "fixed_assets"
	KindOpeningBalances   FileKind = "opening_balances"
	KindJournalEntries    FileKind = "journal_entries"
)

type IssueSeverity string

const (
	SeverityError   IssueSeverity = "ERROR"
	SeverityWarning IssueSeverity = "WARNING"
)

type BundleFile struct {
	Kind       FileKind `json:"kind"`
	FileName   string   `json:"file_name"`
	CSVContent string   `json:"csv_content"`
}

type ValidateBundleRequest struct {
	Files []BundleFile `json:"files"`
}

type BundleValidationReport struct {
	Summary BundleValidationSummary `json:"summary"`
	Files   []FileValidation        `json:"files"`
	Issues  []ValidationIssue       `json:"issues,omitempty"`
}

type BundleValidationSummary struct {
	FilesValidated int  `json:"files_validated"`
	RowsValidated  int  `json:"rows_validated"`
	ErrorCount     int  `json:"error_count"`
	WarningCount   int  `json:"warning_count"`
	Ready          bool `json:"ready"`
}

type FileValidation struct {
	Kind           FileKind `json:"kind"`
	FileName       string   `json:"file_name"`
	Rows           int      `json:"rows"`
	Headers        []string `json:"headers,omitempty"`
	MissingColumns []string `json:"missing_columns,omitempty"`
}

type ValidationIssue struct {
	Severity   IssueSeverity `json:"severity"`
	Kind       FileKind      `json:"kind"`
	FileName   string        `json:"file_name"`
	Row        int           `json:"row,omitempty"`
	Field      string        `json:"field,omitempty"`
	Value      string        `json:"value,omitempty"`
	TargetKind FileKind      `json:"target_kind,omitempty"`
	Message    string        `json:"message"`
}
