package cutover

import (
	"fmt"
	"sort"
	"strings"
)

type MigrationExecutionStepStatus string

const (
	MigrationExecutionStepReady        MigrationExecutionStepStatus = "READY"
	MigrationExecutionStepNeedsContext MigrationExecutionStepStatus = "NEEDS_CONTEXT"
	MigrationExecutionStepBlocked      MigrationExecutionStepStatus = "BLOCKED"
)

type PlanMigrationExecutionRequest struct {
	Files                    []BundleFile            `json:"files"`
	EInvoiceContactMode      EInvoiceContactMode     `json:"e_invoice_contact_mode,omitempty"`
	EInvoiceInvoiceType      string                  `json:"e_invoice_invoice_type,omitempty"`
	ProviderPreset           MigrationProviderPreset `json:"provider_preset,omitempty"`
	BankTransactionAccountID string                  `json:"bank_transaction_account_id,omitempty"`
	OpeningBalanceEntryDate  string                  `json:"opening_balance_entry_date,omitempty"`
}

type MigrationExecutionPlan struct {
	Summary            MigrationExecutionPlanSummary `json:"summary"`
	Validation         BundleValidationReport        `json:"validation"`
	Steps              []MigrationExecutionStep      `json:"steps,omitempty"`
	RemediationActions []MigrationRemediationAction  `json:"remediation_actions,omitempty"`
}

type MigrationExecutionPlanSummary struct {
	ValidationReady   bool `json:"validation_ready"`
	Ready             bool `json:"ready"`
	StepCount         int  `json:"step_count"`
	ReadyStepCount    int  `json:"ready_step_count"`
	NeedsContextCount int  `json:"needs_context_count"`
	BlockedStepCount  int  `json:"blocked_step_count"`
}

type MigrationExecutionStep struct {
	StepNumber    int                          `json:"step_number"`
	Kind          FileKind                     `json:"kind"`
	FileName      string                       `json:"file_name"`
	Status        MigrationExecutionStepStatus `json:"status"`
	Message       string                       `json:"message"`
	Action        string                       `json:"action"`
	APIMethod     string                       `json:"api_method,omitempty"`
	APIPath       string                       `json:"api_path,omitempty"`
	CLICommand    string                       `json:"cli_command,omitempty"`
	DependsOn     []FileKind                   `json:"depends_on,omitempty"`
	ContextFields []string                     `json:"context_fields,omitempty"`
}

type migrationExecutionSpec struct {
	apiPath       string
	cliCommand    string
	dependsOn     []FileKind
	contextFields []string
	message       string
}

var migrationExecutionOrder = map[FileKind]int{
	KindAccounts:          10,
	KindContacts:          20,
	KindEmployees:         30,
	KindCostCenters:       40,
	KindProductCategories: 50,
	KindWarehouses:        60,
	KindProducts:          70,
	KindBankAccounts:      80,
	KindInvoices:          90,
	KindEInvoices:         100,
	KindQuotes:            110,
	KindOrders:            120,
	KindRecurringInvoices: 130,
	KindExpenses:          140,
	KindPayments:          150,
	KindPayrollHistory:    160,
	KindLeaveBalances:     170,
	KindTSDHistory:        180,
	KindKMDHistory:        190,
	KindFixedAssets:       200,
	KindStockAdjustments:  210,
	KindBankTransactions:  220,
	KindOpeningBalances:   230,
	KindJournalEntries:    240,
	KindCostAllocations:   250,
}

func BuildMigrationExecutionPlan(req *PlanMigrationExecutionRequest) (*MigrationExecutionPlan, error) {
	if req == nil {
		return nil, fmt.Errorf("migration execution plan request is required")
	}

	report, err := ValidateBundle(&ValidateBundleRequest{
		Files:               req.Files,
		EInvoiceContactMode: req.EInvoiceContactMode,
		EInvoiceInvoiceType: req.EInvoiceInvoiceType,
		ProviderPreset:      req.ProviderPreset,
	})
	if err != nil {
		return nil, err
	}

	plan := &MigrationExecutionPlan{
		Validation:         *report,
		RemediationActions: report.RemediationActions,
	}

	files := sortedExecutionFiles(req.Files)
	plan.Steps = make([]MigrationExecutionStep, 0, len(files))
	for _, file := range files {
		step := buildMigrationExecutionStep(file, req, report.Summary.Ready)
		step.StepNumber = len(plan.Steps) + 1
		plan.Steps = append(plan.Steps, step)
	}
	plan.Summary = summarizeMigrationExecutionPlan(report.Summary.Ready, plan.Steps)
	return plan, nil
}

func sortedExecutionFiles(files []BundleFile) []BundleFile {
	sorted := append([]BundleFile(nil), files...)
	sort.SliceStable(sorted, func(i, j int) bool {
		left := migrationExecutionOrder[sorted[i].Kind]
		right := migrationExecutionOrder[sorted[j].Kind]
		if left == 0 {
			left = 1000
		}
		if right == 0 {
			right = 1000
		}
		if left != right {
			return left < right
		}
		return sorted[i].FileName < sorted[j].FileName
	})
	return sorted
}

func buildMigrationExecutionStep(file BundleFile, req *PlanMigrationExecutionRequest, validationReady bool) MigrationExecutionStep {
	spec := migrationExecutionStepSpec(file.Kind, file.FileName, req)
	step := MigrationExecutionStep{
		Kind:          file.Kind,
		FileName:      file.FileName,
		Status:        MigrationExecutionStepReady,
		Message:       spec.message,
		Action:        "Import this validated cutover file through the listed API or CLI command.",
		APIMethod:     "POST",
		APIPath:       spec.apiPath,
		CLICommand:    spec.cliCommand,
		DependsOn:     spec.dependsOn,
		ContextFields: spec.contextFields,
	}

	if !validationReady {
		step.Status = MigrationExecutionStepBlocked
		step.Message = "Resolve migration preflight blockers before running this import."
		step.Action = "Rerun migration validation after fixing the listed remediation actions."
		step.APIMethod = ""
		step.APIPath = ""
		step.CLICommand = migrationValidationCommand(file.Kind)
		step.ContextFields = nil
		return step
	}

	if len(spec.contextFields) > 0 {
		step.Status = MigrationExecutionStepNeedsContext
		step.Action = "Provide the missing execution context, then run the listed import command."
	}
	return step
}

func migrationExecutionStepSpec(kind FileKind, fileName string, req *PlanMigrationExecutionRequest) migrationExecutionSpec {
	fileRef := shellPlaceholderFile(fileName, kind)
	switch kind {
	case KindAccounts:
		return basicMigrationExecutionSpec("/accounts/import", "oa accounts import --file "+fileRef)
	case KindContacts:
		return basicMigrationExecutionSpec("/contacts/import", "oa contacts import --file "+fileRef)
	case KindEmployees:
		return basicMigrationExecutionSpec("/employees/import", "oa employees import --file "+fileRef)
	case KindExpenses:
		return migrationExecutionSpec{
			apiPath:    tenantAPIPath("/expenses/import"),
			cliCommand: "oa expenses import --file " + fileRef,
			dependsOn:  []FileKind{KindAccounts, KindContacts, KindEmployees},
			message:    "Import expense claim history after account, contact, and employee masters.",
		}
	case KindInvoices:
		return migrationExecutionSpec{
			apiPath:    tenantAPIPath("/invoices/import"),
			cliCommand: "oa invoices import --file " + fileRef,
			dependsOn:  []FileKind{KindContacts, KindProducts},
			message:    "Import grouped invoice history after contacts and products.",
		}
	case KindEInvoices:
		invoiceTypeFlag := ""
		if invoiceType := strings.TrimSpace(req.EInvoiceInvoiceType); invoiceType != "" {
			invoiceTypeFlag = " --invoice-type " + normalizeCutoverInvoiceType(invoiceType)
		}
		return migrationExecutionSpec{
			apiPath:    tenantAPIPath("/invoices/import-einvoice"),
			cliCommand: "oa invoices import-einvoice --file " + fileRef + invoiceTypeFlag,
			dependsOn:  []FileKind{KindContacts},
			message:    "Import Estonian e-invoice XML after contact references are ready.",
		}
	case KindPayments:
		return migrationExecutionSpec{
			apiPath:    tenantAPIPath("/payments/import"),
			cliCommand: "oa payments import --file " + fileRef,
			dependsOn:  []FileKind{KindContacts, KindInvoices, KindBankAccounts},
			message:    "Import historical payments after contacts, invoices, and bank accounts.",
		}
	case KindBankAccounts:
		return migrationExecutionSpec{
			apiPath:    tenantAPIPath("/bank-accounts/import"),
			cliCommand: "oa banking accounts import --file " + fileRef,
			dependsOn:  []FileKind{KindAccounts},
			message:    "Import bank account masters after ledger accounts.",
		}
	case KindBankTransactions:
		accountID := strings.TrimSpace(req.BankTransactionAccountID)
		contextFields := []string(nil)
		if accountID == "" {
			accountID = "<bank-account-id>"
			contextFields = []string{"bank_transaction_account_id"}
		}
		return migrationExecutionSpec{
			apiPath:       tenantAPIPath(fmt.Sprintf("/bank-accounts/%s/import", accountID)),
			cliCommand:    fmt.Sprintf("oa banking transactions import --account-id %s --file %s", accountID, fileRef),
			dependsOn:     []FileKind{KindBankAccounts},
			contextFields: contextFields,
			message:       "Import bank transactions after selecting the target bank account.",
		}
	case KindPayrollHistory:
		return migrationExecutionSpec{
			apiPath:    tenantAPIPath("/payroll-runs/import-history"),
			cliCommand: "oa payroll import-history --file " + fileRef,
			dependsOn:  []FileKind{KindEmployees},
			message:    "Import finalized historical payroll runs after employees.",
		}
	case KindLeaveBalances:
		return migrationExecutionSpec{
			apiPath:    tenantAPIPath("/leave-balances/import"),
			cliCommand: "oa leave balances import --file " + fileRef,
			dependsOn:  []FileKind{KindEmployees},
			message:    "Import leave balances after employees and absence type setup.",
		}
	case KindTSDHistory:
		return migrationExecutionSpec{
			apiPath:    tenantAPIPath("/tsd/import-history"),
			cliCommand: "oa tsd import-history --file " + fileRef,
			dependsOn:  []FileKind{KindEmployees},
			message:    "Import historical TSD declarations after employees.",
		}
	case KindKMDHistory:
		return migrationExecutionSpec{
			apiPath:    tenantAPIPath("/tax/kmd/import-history"),
			cliCommand: "oa tax kmd import-history --file " + fileRef,
			message:    "Import historical KMD declarations after VAT history is validated.",
		}
	case KindQuotes:
		return migrationExecutionSpec{
			apiPath:    tenantAPIPath("/quotes/import"),
			cliCommand: "oa quotes import --file " + fileRef,
			dependsOn:  []FileKind{KindContacts, KindProducts},
			message:    "Import historical quotes after contacts and products.",
		}
	case KindOrders:
		return migrationExecutionSpec{
			apiPath:    tenantAPIPath("/orders/import"),
			cliCommand: "oa orders import --file " + fileRef,
			dependsOn:  []FileKind{KindContacts, KindProducts, KindQuotes},
			message:    "Import historical orders after contacts, products, and linked quotes.",
		}
	case KindRecurringInvoices:
		return migrationExecutionSpec{
			apiPath:    tenantAPIPath("/recurring-invoices/import"),
			cliCommand: "oa recurring-invoices import --file " + fileRef,
			dependsOn:  []FileKind{KindContacts, KindProducts, KindAccounts},
			message:    "Import recurring invoice templates after reusable masters.",
		}
	case KindCostCenters:
		return basicMigrationExecutionSpec("/cost-centers/import", "oa cost-centers import --file "+fileRef)
	case KindCostAllocations:
		return migrationExecutionSpec{
			apiPath:    tenantAPIPath("/cost-centers/allocations/import"),
			cliCommand: "oa cost-centers allocations import --file " + fileRef,
			dependsOn:  []FileKind{KindCostCenters, KindJournalEntries},
			message:    "Import cost allocations after cost centers and historical journal lines.",
		}
	case KindProductCategories:
		return basicMigrationExecutionSpec("/product-categories/import", "oa inventory categories import --file "+fileRef)
	case KindWarehouses:
		return basicMigrationExecutionSpec("/warehouses/import", "oa inventory warehouses import --file "+fileRef)
	case KindProducts:
		return migrationExecutionSpec{
			apiPath:    tenantAPIPath("/products/import"),
			cliCommand: "oa inventory products import --file " + fileRef,
			dependsOn:  []FileKind{KindProductCategories, KindContacts},
			message:    "Import product masters after categories and supplier contacts.",
		}
	case KindStockAdjustments:
		return migrationExecutionSpec{
			apiPath:    tenantAPIPath("/inventory/stock-import"),
			cliCommand: "oa inventory stock import --file " + fileRef,
			dependsOn:  []FileKind{KindProducts, KindWarehouses, KindCostCenters},
			message:    "Import stock balances after products, warehouses, and cost centers.",
		}
	case KindFixedAssets:
		return migrationExecutionSpec{
			apiPath:    tenantAPIPath("/assets/import"),
			cliCommand: "oa assets import --file " + fileRef,
			dependsOn:  []FileKind{KindAccounts, KindContacts, KindInvoices},
			message:    "Import fixed assets after ledger accounts, suppliers, and source invoices.",
		}
	case KindOpeningBalances:
		entryDate := strings.TrimSpace(req.OpeningBalanceEntryDate)
		contextFields := []string(nil)
		if entryDate == "" {
			entryDate = "<YYYY-MM-DD>"
			contextFields = []string{"opening_balance_entry_date"}
		}
		return migrationExecutionSpec{
			apiPath:       tenantAPIPath("/journal-entries/import-opening-balances"),
			cliCommand:    fmt.Sprintf("oa journal import-opening-balances --entry-date %s --file %s", entryDate, fileRef),
			dependsOn:     []FileKind{KindAccounts},
			contextFields: contextFields,
			message:       "Import opening balances after the chart of accounts.",
		}
	case KindJournalEntries:
		return migrationExecutionSpec{
			apiPath:    tenantAPIPath("/journal-entries/import"),
			cliCommand: "oa journal import --file " + fileRef,
			dependsOn:  []FileKind{KindAccounts},
			message:    "Import historical journals after the chart of accounts.",
		}
	default:
		return migrationExecutionSpec{
			cliCommand:    migrationValidationCommand(kind),
			contextFields: []string{"supported_migration_file_kind"},
			message:       "Map this file to a supported migration import kind before execution.",
		}
	}
}

func basicMigrationExecutionSpec(pathSuffix, command string) migrationExecutionSpec {
	return migrationExecutionSpec{
		apiPath:    tenantAPIPath(pathSuffix),
		cliCommand: command,
		message:    "Import this validated migration file.",
	}
}

func tenantAPIPath(pathSuffix string) string {
	return "/api/v1/tenants/{tenantID}" + pathSuffix
}

func shellPlaceholderFile(fileName string, kind FileKind) string {
	fileName = strings.TrimSpace(fileName)
	if fileName == "" {
		fileName = string(kind) + ".csv"
		if kind == KindEInvoices {
			fileName = string(kind) + ".xml"
		}
	}
	return fmt.Sprintf("<%s>", fileName)
}

func summarizeMigrationExecutionPlan(validationReady bool, steps []MigrationExecutionStep) MigrationExecutionPlanSummary {
	summary := MigrationExecutionPlanSummary{
		ValidationReady: validationReady,
		StepCount:       len(steps),
	}
	for _, step := range steps {
		switch step.Status {
		case MigrationExecutionStepReady:
			summary.ReadyStepCount++
		case MigrationExecutionStepNeedsContext:
			summary.NeedsContextCount++
		case MigrationExecutionStepBlocked:
			summary.BlockedStepCount++
		}
	}
	summary.Ready = summary.ValidationReady && summary.StepCount > 0 && summary.NeedsContextCount == 0 && summary.BlockedStepCount == 0
	return summary
}
