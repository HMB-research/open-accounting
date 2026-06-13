package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/HMB-research/open-accounting/internal/accounting"
	"github.com/HMB-research/open-accounting/internal/assets"
	"github.com/HMB-research/open-accounting/internal/banking"
	"github.com/HMB-research/open-accounting/internal/contacts"
	"github.com/HMB-research/open-accounting/internal/cutover"
	"github.com/HMB-research/open-accounting/internal/expenses"
	"github.com/HMB-research/open-accounting/internal/inventory"
	"github.com/HMB-research/open-accounting/internal/invoicing"
	"github.com/HMB-research/open-accounting/internal/orders"
	"github.com/HMB-research/open-accounting/internal/payments"
	"github.com/HMB-research/open-accounting/internal/payroll"
	"github.com/HMB-research/open-accounting/internal/quotes"
	"github.com/HMB-research/open-accounting/internal/recurring"
	"github.com/HMB-research/open-accounting/internal/tax"
)

const (
	migrationExecutionResultPlanned   = cutover.MigrationExecutionResultPlanned
	migrationExecutionResultSkipped   = cutover.MigrationExecutionResultSkipped
	migrationExecutionResultSucceeded = cutover.MigrationExecutionResultSucceeded
	migrationExecutionResultFailed    = cutover.MigrationExecutionResultFailed
)

type migrationExecuteOptions struct {
	Confirm                  bool
	BankTransactionAccountID string
	BankTransactionFormat    string
	EInvoiceInvoiceType      invoicing.InvoiceType
	OpeningBalanceEntryDate  string
	ResumeFromRun            *migrationExecutionRun
}

type migrationExecutionRun = cutover.MigrationExecutionRun
type migrationExecutionRunSummary = cutover.MigrationExecutionRunSummary
type migrationExecutionStepRun = cutover.MigrationExecutionStepRun

func (a *cliApp) runMigrationExecute(ctx context.Context, cfg *cliConfig, client *apiClient, args []string) error {
	fs := flag.NewFlagSet("migration execute", flag.ContinueOnError)
	fs.SetOutput(a.stderr)
	accountsFile := fs.String("accounts", "", "Accounts CSV file")
	contactsFile := fs.String("contacts", "", "Contacts CSV file")
	employeesFile := fs.String("employees", "", "Employees CSV file")
	expensesFile := fs.String("expenses", "", "Expenses CSV file")
	invoicesFile := fs.String("invoices", "", "Invoices CSV file")
	eInvoicesFile := fs.String("e-invoices", "", "Estonian e-invoice XML file")
	eInvoiceContactMode := fs.String("e-invoice-contact-mode", string(cutover.EInvoiceContactModeSupplier), "E-invoice contact validation mode: supplier, customer, or both")
	eInvoiceInvoiceType := fs.String("e-invoice-invoice-type", "", "Override executed e-invoice type: SALES, PURCHASE, or CREDIT_NOTE")
	providerPreset := fs.String("provider-preset", string(cutover.MigrationProviderPresetGeneric), "Migration CSV provider preset: generic, merit, or smartaccounts")
	paymentsFile := fs.String("payments", "", "Payments CSV file")
	bankAccountsFile := fs.String("bank-accounts", "", "Bank accounts CSV file")
	bankTransactionsFile := fs.String("bank-transactions", "", "Bank transactions CSV file")
	bankTransactionAccountID := fs.String("bank-transaction-account-id", "", "Bank account ID for bank transaction import execution")
	bankTransactionFormat := fs.String("bank-transaction-format", "auto", "Bank statement format: auto, generic, lhv, camt053, or lhv-camt")
	payrollHistoryFile := fs.String("payroll-history", "", "Historical payroll CSV file")
	leaveBalancesFile := fs.String("leave-balances", "", "Leave balances CSV file")
	tsdHistoryFile := fs.String("tsd-history", "", "TSD history CSV file")
	kmdHistoryFile := fs.String("kmd-history", "", "KMD history CSV file")
	quotesFile := fs.String("quotes", "", "Quotes CSV file")
	ordersFile := fs.String("orders", "", "Orders CSV file")
	recurringInvoicesFile := fs.String("recurring-invoices", "", "Recurring invoice templates CSV file")
	costCentersFile := fs.String("cost-centers", "", "Cost centers CSV file")
	costAllocationsFile := fs.String("cost-allocations", "", "Cost allocations CSV file")
	productCategoriesFile := fs.String("product-categories", "", "Product categories CSV file")
	warehousesFile := fs.String("warehouses", "", "Warehouses CSV file")
	productsFile := fs.String("products", "", "Products CSV file")
	stockFile := fs.String("stock", "", "Stock adjustments CSV file")
	fixedAssetsFile := fs.String("fixed-assets", "", "Fixed assets CSV file")
	openingBalancesFile := fs.String("opening-balances", "", "Opening balances CSV file")
	openingBalanceEntryDate := fs.String("opening-balance-entry-date", "", "Opening balance journal entry date in YYYY-MM-DD")
	journalFile := fs.String("journal", "", "Historical journal CSV file")
	resumeRunFile := fs.String("resume-run", "", "Previous migration execution run JSON file to resume from")
	confirm := fs.Bool("confirm", false, "Execute the planned imports")
	asJSON := fs.Bool("json", false, "Output JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}

	invoiceType, err := parseOptionalInvoiceType(*eInvoiceInvoiceType)
	if err != nil {
		return err
	}
	files, err := buildMigrationBundleFiles([]migrationFileInput{
		{kind: cutover.KindAccounts, path: *accountsFile},
		{kind: cutover.KindContacts, path: *contactsFile},
		{kind: cutover.KindEmployees, path: *employeesFile},
		{kind: cutover.KindExpenses, path: *expensesFile},
		{kind: cutover.KindInvoices, path: *invoicesFile},
		{kind: cutover.KindEInvoices, path: *eInvoicesFile},
		{kind: cutover.KindPayments, path: *paymentsFile},
		{kind: cutover.KindBankAccounts, path: *bankAccountsFile},
		{kind: cutover.KindBankTransactions, path: *bankTransactionsFile},
		{kind: cutover.KindPayrollHistory, path: *payrollHistoryFile},
		{kind: cutover.KindLeaveBalances, path: *leaveBalancesFile},
		{kind: cutover.KindTSDHistory, path: *tsdHistoryFile},
		{kind: cutover.KindKMDHistory, path: *kmdHistoryFile},
		{kind: cutover.KindQuotes, path: *quotesFile},
		{kind: cutover.KindOrders, path: *ordersFile},
		{kind: cutover.KindRecurringInvoices, path: *recurringInvoicesFile},
		{kind: cutover.KindCostCenters, path: *costCentersFile},
		{kind: cutover.KindCostAllocations, path: *costAllocationsFile},
		{kind: cutover.KindProductCategories, path: *productCategoriesFile},
		{kind: cutover.KindWarehouses, path: *warehousesFile},
		{kind: cutover.KindProducts, path: *productsFile},
		{kind: cutover.KindStockAdjustments, path: *stockFile},
		{kind: cutover.KindFixedAssets, path: *fixedAssetsFile},
		{kind: cutover.KindOpeningBalances, path: *openingBalancesFile},
		{kind: cutover.KindJournalEntries, path: *journalFile},
	})
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return errors.New("at least one migration CSV or XML file is required")
	}
	resumeFromRun, err := readMigrationExecutionRunFile(*resumeRunFile)
	if err != nil {
		return err
	}

	plan, err := client.planMigrationExecution(ctx, cfg.TenantID, &cutover.PlanMigrationExecutionRequest{
		Files:                    files,
		EInvoiceContactMode:      cutover.EInvoiceContactMode(strings.TrimSpace(*eInvoiceContactMode)),
		ProviderPreset:           cutover.MigrationProviderPreset(strings.TrimSpace(*providerPreset)),
		BankTransactionAccountID: strings.TrimSpace(*bankTransactionAccountID),
		OpeningBalanceEntryDate:  strings.TrimSpace(*openingBalanceEntryDate),
	})
	if err != nil {
		return err
	}

	run, err := executeMigrationRun(ctx, client, cfg.TenantID, files, plan, migrationExecuteOptions{
		Confirm:                  *confirm,
		BankTransactionAccountID: strings.TrimSpace(*bankTransactionAccountID),
		BankTransactionFormat:    strings.TrimSpace(*bankTransactionFormat),
		EInvoiceInvoiceType:      invoiceType,
		OpeningBalanceEntryDate:  strings.TrimSpace(*openingBalanceEntryDate),
		ResumeFromRun:            resumeFromRun,
	})
	if *asJSON {
		_ = printJSON(a.stdout, run)
	} else {
		printMigrationExecutionRun(a.stdout, run)
	}
	return err
}

func executeMigrationRun(ctx context.Context, client *apiClient, tenantID string, files []cutover.BundleFile, plan *cutover.MigrationExecutionPlan, opts migrationExecuteOptions) (*migrationExecutionRun, error) {
	run := newMigrationExecutionRun(plan, opts.Confirm, opts.ResumeFromRun)
	if plan == nil {
		return run, errors.New("migration execution plan is required")
	}
	if !plan.Summary.Ready {
		return run, errors.New("migration execution plan is not ready")
	}
	if !opts.Confirm {
		return run, errors.New("migration execute requires --confirm")
	}

	filesByKey := migrationFilesByStepKey(files)
	for i, step := range plan.Steps {
		if run.Steps[i].Status == migrationExecutionResultSucceeded {
			continue
		}
		if step.Status != cutover.MigrationExecutionStepReady {
			run.Steps[i].Status = migrationExecutionResultSkipped
			run.Steps[i].Message = "Step is not ready to execute."
			continue
		}
		file, ok := filesByKey[migrationStepFileKey(step.Kind, step.FileName)]
		if !ok {
			run.Steps[i].Status = migrationExecutionResultFailed
			run.Steps[i].Error = "migration bundle file not found"
			run.Summary.FailedStepCount++
			run.Summary.Status = "failed"
			return run, fmt.Errorf("migration bundle file not found for step %d (%s)", step.StepNumber, step.Kind)
		}
		response, err := executeMigrationImportStep(ctx, client, tenantID, step, file, opts)
		if err != nil {
			run.Steps[i].Status = migrationExecutionResultFailed
			run.Steps[i].Error = err.Error()
			run.Summary.FailedStepCount++
			run.Summary.Status = "failed"
			return run, fmt.Errorf("execute migration step %d (%s): %w", step.StepNumber, step.Kind, err)
		}
		run.Steps[i].Status = migrationExecutionResultSucceeded
		run.Steps[i].Message = "Import completed."
		run.Steps[i].Response = response
		run.Summary.SucceededStepCount++
	}
	run.Summary.Status = "succeeded"
	return run, nil
}

func newMigrationExecutionRun(plan *cutover.MigrationExecutionPlan, confirmed bool, resumeFrom *migrationExecutionRun) *migrationExecutionRun {
	run := cutover.NewResumableMigrationExecutionRun(plan, confirmed, resumeFrom)
	if run != nil && !confirmed {
		for index := range run.Steps {
			if run.Steps[index].Status == cutover.MigrationExecutionResultPlanned {
				run.Steps[index].Message = "Pass --confirm to run this import."
			}
		}
	}
	return run
}

func readMigrationExecutionRunFile(path string) (*migrationExecutionRun, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, nil
	}
	payload, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read migration resume run: %w", err)
	}
	var run migrationExecutionRun
	if err := json.Unmarshal(payload, &run); err != nil {
		return nil, fmt.Errorf("parse migration resume run: %w", err)
	}
	return &run, nil
}

func migrationFilesByStepKey(files []cutover.BundleFile) map[string]cutover.BundleFile {
	filesByKey := make(map[string]cutover.BundleFile, len(files))
	for _, file := range files {
		filesByKey[migrationStepFileKey(file.Kind, file.FileName)] = file
	}
	return filesByKey
}

func migrationStepFileKey(kind cutover.FileKind, fileName string) string {
	return string(kind) + "\x00" + strings.TrimSpace(fileName)
}

func executeMigrationImportStep(ctx context.Context, client *apiClient, tenantID string, step cutover.MigrationExecutionStep, file cutover.BundleFile, opts migrationExecuteOptions) (json.RawMessage, error) {
	switch step.Kind {
	case cutover.KindAccounts:
		return migrationStepResponse(client.importAccounts(ctx, tenantID, &accounting.ImportAccountsRequest{CSVContent: file.CSVContent, FileName: file.FileName}))
	case cutover.KindContacts:
		return migrationStepResponse(client.importContacts(ctx, tenantID, &contacts.ImportContactsRequest{CSVContent: file.CSVContent, FileName: file.FileName}))
	case cutover.KindEmployees:
		return migrationStepResponse(client.importEmployees(ctx, tenantID, &payroll.ImportEmployeesRequest{CSVContent: file.CSVContent, FileName: file.FileName}))
	case cutover.KindExpenses:
		return migrationStepResponse(client.importExpenses(ctx, tenantID, &expenses.ImportExpensesRequest{CSVContent: file.CSVContent, FileName: file.FileName}))
	case cutover.KindInvoices:
		return migrationStepResponse(client.importInvoices(ctx, tenantID, &invoicing.ImportInvoicesRequest{CSVContent: file.CSVContent, FileName: file.FileName}))
	case cutover.KindEInvoices:
		return migrationStepResponse(client.importEInvoice(ctx, tenantID, &invoicing.ImportEInvoiceRequest{XMLContent: file.XMLContent, FileName: file.FileName, InvoiceType: opts.EInvoiceInvoiceType}))
	case cutover.KindPayments:
		return migrationStepResponse(client.importPayments(ctx, tenantID, &payments.ImportPaymentsRequest{CSVContent: file.CSVContent, FileName: file.FileName}))
	case cutover.KindBankAccounts:
		rows, err := parseBankAccountCSVRows(file.CSVContent)
		if err != nil {
			return nil, err
		}
		return migrationStepResponse(client.importBankAccounts(ctx, tenantID, &banking.ImportBankAccountsRequest{FileName: file.FileName, Rows: rows, SkipDuplicates: true}))
	case cutover.KindBankTransactions:
		if strings.TrimSpace(opts.BankTransactionAccountID) == "" {
			return nil, errors.New("bank-transaction-account-id is required")
		}
		rows, err := parseBankTransactionCSVRowsWithFormat(file.CSVContent, opts.BankTransactionFormat)
		if err != nil {
			return nil, err
		}
		return migrationStepResponse(client.importBankTransactions(ctx, tenantID, opts.BankTransactionAccountID, &banking.ImportCSVRequest{FileName: file.FileName, Transactions: rows, SkipDuplicates: true}))
	case cutover.KindPayrollHistory:
		return migrationStepResponse(client.importPayrollHistory(ctx, tenantID, &payroll.ImportPayrollHistoryRequest{CSVContent: file.CSVContent, FileName: file.FileName}))
	case cutover.KindLeaveBalances:
		return migrationStepResponse(client.importLeaveBalances(ctx, tenantID, &payroll.ImportLeaveBalancesRequest{CSVContent: file.CSVContent, FileName: file.FileName}))
	case cutover.KindTSDHistory:
		return migrationStepResponse(client.importTSDHistory(ctx, tenantID, &payroll.ImportTSDHistoryRequest{CSVContent: file.CSVContent, FileName: file.FileName}))
	case cutover.KindKMDHistory:
		return migrationStepResponse(client.importKMDHistory(ctx, tenantID, &tax.ImportKMDHistoryRequest{CSVContent: file.CSVContent, FileName: file.FileName}))
	case cutover.KindQuotes:
		return migrationStepResponse(client.importQuotes(ctx, tenantID, &quotes.ImportQuotesRequest{CSVContent: file.CSVContent, FileName: file.FileName}))
	case cutover.KindOrders:
		return migrationStepResponse(client.importOrders(ctx, tenantID, &orders.ImportOrdersRequest{CSVContent: file.CSVContent, FileName: file.FileName}))
	case cutover.KindRecurringInvoices:
		return migrationStepResponse(client.importRecurringInvoices(ctx, tenantID, &recurring.ImportRecurringInvoicesRequest{CSVContent: file.CSVContent, FileName: file.FileName}))
	case cutover.KindCostCenters:
		return migrationStepResponse(client.importCostCenters(ctx, tenantID, &accounting.ImportCostCentersRequest{CSVContent: file.CSVContent, FileName: file.FileName}))
	case cutover.KindCostAllocations:
		return migrationStepResponse(client.importCostAllocations(ctx, tenantID, &accounting.ImportCostAllocationsRequest{CSVContent: file.CSVContent, FileName: file.FileName}))
	case cutover.KindProductCategories:
		return migrationStepResponse(client.importProductCategories(ctx, tenantID, &inventory.ImportProductCategoriesRequest{CSVContent: file.CSVContent, FileName: file.FileName}))
	case cutover.KindWarehouses:
		return migrationStepResponse(client.importWarehouses(ctx, tenantID, &inventory.ImportWarehousesRequest{CSVContent: file.CSVContent, FileName: file.FileName}))
	case cutover.KindProducts:
		return migrationStepResponse(client.importProducts(ctx, tenantID, &inventory.ImportProductsRequest{CSVContent: file.CSVContent, FileName: file.FileName}))
	case cutover.KindStockAdjustments:
		return migrationStepResponse(client.importStockAdjustments(ctx, tenantID, &inventory.ImportStockAdjustmentsRequest{CSVContent: file.CSVContent, FileName: file.FileName}))
	case cutover.KindFixedAssets:
		return migrationStepResponse(client.importAssets(ctx, tenantID, &assets.ImportAssetsRequest{CSVContent: file.CSVContent, FileName: file.FileName}))
	case cutover.KindOpeningBalances:
		return migrationStepResponse(client.importOpeningBalances(ctx, tenantID, &accounting.ImportOpeningBalancesRequest{CSVContent: file.CSVContent, FileName: file.FileName, EntryDate: opts.OpeningBalanceEntryDate, Description: "Opening balances", Reference: openingBalanceExecutionReference(opts.OpeningBalanceEntryDate)}))
	case cutover.KindJournalEntries:
		return migrationStepResponse(client.importJournalEntries(ctx, tenantID, &accounting.ImportJournalEntriesRequest{CSVContent: file.CSVContent, FileName: file.FileName}))
	default:
		return nil, fmt.Errorf("unsupported migration execution kind %q", step.Kind)
	}
}

func migrationStepResponse(value any, err error) (json.RawMessage, error) {
	if err != nil {
		return nil, err
	}
	payload, _ := json.Marshal(value)
	return payload, nil
}

func openingBalanceExecutionReference(entryDate string) string {
	trimmed := strings.TrimSpace(entryDate)
	if len(trimmed) >= 4 {
		return "OB-" + trimmed[:4]
	}
	return "OB"
}
