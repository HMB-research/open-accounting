package cutover

type FileKind string

const (
	KindAccounts          FileKind = "accounts"
	KindContacts          FileKind = "contacts"
	KindEmployees         FileKind = "employees"
	KindExpenses          FileKind = "expenses"
	KindInvoices          FileKind = "invoices"
	KindEInvoices         FileKind = "e_invoices"
	KindPayments          FileKind = "payments"
	KindBankAccounts      FileKind = "bank_accounts"
	KindBankTransactions  FileKind = "bank_transactions"
	KindPayrollHistory    FileKind = "payroll_history"
	KindLeaveBalances     FileKind = "leave_balances"
	KindTSDHistory        FileKind = "tsd_history"
	KindKMDHistory        FileKind = "kmd_history"
	KindQuotes            FileKind = "quotes"
	KindOrders            FileKind = "orders"
	KindRecurringInvoices FileKind = "recurring_invoices"
	KindCostCenters       FileKind = "cost_centers"
	KindCostAllocations   FileKind = "cost_allocations"
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

type EInvoiceContactMode string

const (
	EInvoiceContactModeSupplier EInvoiceContactMode = "supplier"
	EInvoiceContactModeCustomer EInvoiceContactMode = "customer"
	EInvoiceContactModeBoth     EInvoiceContactMode = "both"
)

type MigrationProviderPreset string

const (
	MigrationProviderPresetGeneric       MigrationProviderPreset = "generic"
	MigrationProviderPresetMerit         MigrationProviderPreset = "merit"
	MigrationProviderPresetSmartAccounts MigrationProviderPreset = "smartaccounts"
	MigrationProviderPresetDirecto       MigrationProviderPreset = "directo"
)

type MigrationProviderPresetInfo struct {
	Preset           MigrationProviderPreset           `json:"preset"`
	Label            string                            `json:"label"`
	Description      string                            `json:"description"`
	FileKindCount    int                               `json:"file_kind_count"`
	PresetAliasCount int                               `json:"preset_alias_count"`
	FileKinds        []MigrationProviderPresetKindInfo `json:"file_kinds,omitempty"`
}

type MigrationProviderPresetKindInfo struct {
	Kind                 FileKind                       `json:"kind"`
	RequiredColumnGroups [][]string                     `json:"required_column_groups,omitempty"`
	PresetAliasCount     int                            `json:"preset_alias_count"`
	SampleAliases        []MigrationProviderPresetAlias `json:"sample_aliases,omitempty"`
}

type MigrationProviderPresetAlias struct {
	SourceHeader    string `json:"source_header"`
	CanonicalHeader string `json:"canonical_header"`
}

type BundleFile struct {
	Kind       FileKind `json:"kind"`
	FileName   string   `json:"file_name"`
	CSVContent string   `json:"csv_content,omitempty"`
	XMLContent string   `json:"xml_content,omitempty"`
}

type ValidateBundleRequest struct {
	Files               []BundleFile            `json:"files"`
	EInvoiceContactMode EInvoiceContactMode     `json:"e_invoice_contact_mode,omitempty"`
	ProviderPreset      MigrationProviderPreset `json:"provider_preset,omitempty"`
}

type BundleValidationReport struct {
	Summary            BundleValidationSummary      `json:"summary"`
	Files              []FileValidation             `json:"files"`
	Issues             []ValidationIssue            `json:"issues,omitempty"`
	RemediationActions []MigrationRemediationAction `json:"remediation_actions,omitempty"`
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

type MigrationRemediationAction struct {
	Code           string   `json:"code"`
	Severity       string   `json:"severity"`
	Scope          string   `json:"scope"`
	OwnerRole      string   `json:"owner_role"`
	WorkspaceQueue string   `json:"workspace_queue,omitempty"`
	AssignmentKey  string   `json:"assignment_key,omitempty"`
	Priority       string   `json:"priority,omitempty"`
	DueInDays      int      `json:"due_in_days,omitempty"`
	Message        string   `json:"message"`
	Action         string   `json:"action"`
	Kind           FileKind `json:"kind,omitempty"`
	FileName       string   `json:"file_name,omitempty"`
	Field          string   `json:"field,omitempty"`
	TargetKind     FileKind `json:"target_kind,omitempty"`
	IssueCount     int      `json:"issue_count"`
	UIPath         string   `json:"ui_path,omitempty"`
	CLICommand     string   `json:"cli_command,omitempty"`
}
