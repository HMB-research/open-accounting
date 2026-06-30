package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/HMB-research/open-accounting/internal/cutover"
)

const smartAccountsSyncReportName = "smartaccounts-sync-report.json"

type smartAccountsSyncPrivateReport struct {
	Provider       cutover.MigrationProviderPreset         `json:"provider"`
	GeneratedAt    string                                  `json:"generated_at"`
	TenantID       string                                  `json:"tenant_id"`
	Confirmed      bool                                    `json:"confirmed"`
	Snapshot       *cutover.SmartAccountsSnapshotReport    `json:"snapshot,omitempty"`
	Validation     *cutover.BundleValidationReport         `json:"validation,omitempty"`
	Plan           *cutover.MigrationExecutionPlan         `json:"plan,omitempty"`
	ExecutionRun   *cutover.MigrationExecutionRun          `json:"execution_run,omitempty"`
	ExecutionError string                                  `json:"execution_error,omitempty"`
	Context        smartAccountsSyncExecutionContext       `json:"context"`
	Reconciliation []smartAccountsSyncReconciliationTarget `json:"reconciliation"`
}

type smartAccountsSyncExecutionContext struct {
	BankTransactionAccountID string `json:"bank_transaction_account_id,omitempty"`
	BankTransactionFormat    string `json:"bank_transaction_format,omitempty"`
	EInvoiceContactMode      string `json:"e_invoice_contact_mode,omitempty"`
	EInvoiceInvoiceType      string `json:"e_invoice_invoice_type,omitempty"`
	OpeningBalanceEntryDate  string `json:"opening_balance_entry_date,omitempty"`
	PostJournalEntries       bool   `json:"post_journal_entries,omitempty"`
}

type smartAccountsSyncReconciliationTarget struct {
	Area                   string `json:"area"`
	SmartAccountsEvidence  string `json:"smartaccounts_evidence"`
	OpenAccountingEvidence string `json:"open_accounting_evidence"`
	DiscrepancyRisk        string `json:"discrepancy_risk"`
}

type smartAccountsSyncPublicSummary struct {
	Provider             cutover.MigrationProviderPreset `json:"provider"`
	TenantID             string                          `json:"tenant_id"`
	ManifestPath         string                          `json:"manifest_path,omitempty"`
	OperatorReportPath   string                          `json:"operator_report_path,omitempty"`
	SnapshotHash         string                          `json:"snapshot_hash,omitempty"`
	PreparedFileCount    int                             `json:"prepared_file_count"`
	UnsupportedFileCount int                             `json:"unsupported_file_count"`
	SnapshotWarningCount int                             `json:"snapshot_warning_count"`
	Validation           smartAccountsSyncValidationView `json:"validation"`
	Plan                 smartAccountsSyncPlanView       `json:"plan"`
	Execution            smartAccountsSyncExecutionView  `json:"execution"`
	ReconciliationChecks int                             `json:"reconciliation_checks"`
	NextAction           string                          `json:"next_action"`
}

type smartAccountsSyncValidationView struct {
	Ready        bool `json:"ready"`
	Files        int  `json:"files"`
	Rows         int  `json:"rows"`
	ErrorCount   int  `json:"error_count"`
	WarningCount int  `json:"warning_count"`
}

type smartAccountsSyncPlanView struct {
	Ready             bool `json:"ready"`
	ValidationReady   bool `json:"validation_ready"`
	StepCount         int  `json:"step_count"`
	ReadyStepCount    int  `json:"ready_step_count"`
	NeedsContextCount int  `json:"needs_context_count"`
	BlockedStepCount  int  `json:"blocked_step_count"`
}

type smartAccountsSyncExecutionView struct {
	RunID           string `json:"run_id,omitempty"`
	Status          string `json:"status,omitempty"`
	Confirmed       bool   `json:"confirmed"`
	ProgressPercent int    `json:"progress_percent"`
	SucceededSteps  int    `json:"succeeded_steps"`
	FailedSteps     int    `json:"failed_steps"`
	PlannedSteps    int    `json:"planned_steps"`
	Error           string `json:"error,omitempty"`
}

func (a *cliApp) runMigrationSmartAccountsSync(ctx context.Context, cfg *cliConfig, client *apiClient, args []string) error {
	fs := flag.NewFlagSet("migration smartaccounts-sync", flag.ContinueOnError)
	fs.SetOutput(a.stderr)
	sourceDir := fs.String("source-dir", "", "Private directory containing SmartAccounts CSV/XML exports")
	outputDir := fs.String("out-dir", "", "Private directory where prepared bundle and operator report are written")
	companyID := fs.String("company-id", "", "Source company registry or SmartAccounts company id")
	companyName := fs.String("company-name", "", "Source company name")
	cutoverDate := fs.String("cutover-date", "", "Accounting cutover date in YYYY-MM-DD")
	bankTransactionAccountID := fs.String("bank-transaction-account-id", "", "Open Accounting bank account ID for bank transaction import execution")
	bankTransactionFormat := fs.String("bank-transaction-format", "auto", "Bank statement format: auto, generic, lhv, camt053, or lhv-camt")
	eInvoiceContactMode := fs.String("e-invoice-contact-mode", string(cutover.EInvoiceContactModeSupplier), "E-invoice contact validation mode: supplier, customer, or both")
	eInvoiceInvoiceType := fs.String("e-invoice-invoice-type", "", "Override e-invoice invoice type: SALES, PURCHASE, or CREDIT_NOTE")
	openingBalanceEntryDate := fs.String("opening-balance-entry-date", "", "Opening balance journal entry date in YYYY-MM-DD; defaults to --cutover-date when omitted")
	postJournalEntries := fs.Bool("post-journal-entries", false, "Post imported historical journal entries during confirmed execution")
	confirm := fs.Bool("confirm", false, "Execute imports after preparation, validation, and planning; default saves a dry run only")
	asJSON := fs.Bool("json", false, "Output operator JSON summary; treat as private because it includes paths, hashes, and tenant context")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*sourceDir) == "" || strings.TrimSpace(*outputDir) == "" {
		return fmt.Errorf("source-dir and out-dir are required")
	}
	invoiceType, err := parseOptionalInvoiceType(*eInvoiceInvoiceType)
	if err != nil {
		return err
	}
	openingDate := strings.TrimSpace(*openingBalanceEntryDate)
	if openingDate == "" {
		openingDate = strings.TrimSpace(*cutoverDate)
	}

	generatedAt := time.Now().UTC()
	report := &smartAccountsSyncPrivateReport{
		Provider:    cutover.MigrationProviderPresetSmartAccounts,
		GeneratedAt: generatedAt.Format(time.RFC3339),
		TenantID:    cfg.TenantID,
		Confirmed:   *confirm,
		Context: smartAccountsSyncExecutionContext{
			BankTransactionAccountID: strings.TrimSpace(*bankTransactionAccountID),
			BankTransactionFormat:    strings.TrimSpace(*bankTransactionFormat),
			EInvoiceContactMode:      strings.TrimSpace(*eInvoiceContactMode),
			EInvoiceInvoiceType:      string(invoiceType),
			OpeningBalanceEntryDate:  openingDate,
			PostJournalEntries:       *postJournalEntries,
		},
		Reconciliation: smartAccountsSyncReconciliationTargets(),
	}

	snapshot, err := cutover.PrepareSmartAccountsSnapshot(cutover.SmartAccountsSnapshotOptions{
		SourceDir:         strings.TrimSpace(*sourceDir),
		OutputDir:         strings.TrimSpace(*outputDir),
		SourceCompanyID:   strings.TrimSpace(*companyID),
		SourceCompanyName: strings.TrimSpace(*companyName),
		CutoverDate:       strings.TrimSpace(*cutoverDate),
		GeneratedAt:       generatedAt,
	})
	if err != nil {
		return err
	}
	report.Snapshot = snapshot
	reportPath := filepath.Join(snapshot.OutputDir, smartAccountsSyncReportName)

	files := snapshot.BundleFiles()
	validation, err := client.validateMigrationBundle(ctx, cfg.TenantID, &cutover.ValidateBundleRequest{
		Files:               files,
		EInvoiceContactMode: cutover.EInvoiceContactMode(report.Context.EInvoiceContactMode),
		EInvoiceInvoiceType: report.Context.EInvoiceInvoiceType,
		ProviderPreset:      cutover.MigrationProviderPresetSmartAccounts,
	})
	if err != nil {
		report.ExecutionError = err.Error()
		_ = writeSmartAccountsSyncReport(reportPath, report)
		return err
	}
	report.Validation = validation

	plan, err := client.planMigrationExecution(ctx, cfg.TenantID, &cutover.PlanMigrationExecutionRequest{
		Files:                    files,
		EInvoiceContactMode:      cutover.EInvoiceContactMode(report.Context.EInvoiceContactMode),
		EInvoiceInvoiceType:      report.Context.EInvoiceInvoiceType,
		ProviderPreset:           cutover.MigrationProviderPresetSmartAccounts,
		BankTransactionAccountID: report.Context.BankTransactionAccountID,
		BankTransactionFormat:    report.Context.BankTransactionFormat,
		OpeningBalanceEntryDate:  report.Context.OpeningBalanceEntryDate,
		PostJournalEntries:       report.Context.PostJournalEntries,
	})
	if err != nil {
		report.ExecutionError = err.Error()
		_ = writeSmartAccountsSyncReport(reportPath, report)
		return err
	}
	report.Plan = plan

	run, err := client.executeMigration(ctx, cfg.TenantID, &cutover.ExecuteMigrationRequest{
		Files:                    files,
		Confirm:                  *confirm,
		EInvoiceContactMode:      cutover.EInvoiceContactMode(report.Context.EInvoiceContactMode),
		EInvoiceInvoiceType:      report.Context.EInvoiceInvoiceType,
		ProviderPreset:           cutover.MigrationProviderPresetSmartAccounts,
		BankTransactionAccountID: report.Context.BankTransactionAccountID,
		BankTransactionFormat:    report.Context.BankTransactionFormat,
		OpeningBalanceEntryDate:  report.Context.OpeningBalanceEntryDate,
		PostJournalEntries:       report.Context.PostJournalEntries,
	})
	if err != nil {
		report.ExecutionError = err.Error()
	}
	report.ExecutionRun = run
	if writeErr := writeSmartAccountsSyncReport(reportPath, report); writeErr != nil {
		if err != nil {
			return fmt.Errorf("%w; additionally failed to write SmartAccounts sync report: %v", err, writeErr)
		}
		return writeErr
	}

	summary := buildSmartAccountsSyncPublicSummary(report, reportPath)
	if *asJSON {
		_ = printJSON(a.stdout, summary)
	} else {
		printSmartAccountsSyncSummary(a.stdout, summary)
	}
	return err
}

func writeSmartAccountsSyncReport(path string, report *smartAccountsSyncPrivateReport) error {
	payload, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("encode SmartAccounts sync report: %w", err)
	}
	if err := os.WriteFile(path, append(payload, '\n'), 0o600); err != nil {
		return fmt.Errorf("write SmartAccounts sync report: %w", err)
	}
	return nil
}

func buildSmartAccountsSyncPublicSummary(report *smartAccountsSyncPrivateReport, reportPath string) smartAccountsSyncPublicSummary {
	summary := smartAccountsSyncPublicSummary{
		Provider:             cutover.MigrationProviderPresetSmartAccounts,
		TenantID:             report.TenantID,
		OperatorReportPath:   reportPath,
		ReconciliationChecks: len(report.Reconciliation),
	}
	if report.Snapshot != nil {
		summary.ManifestPath = report.Snapshot.ManifestPath
		summary.SnapshotHash = report.Snapshot.SnapshotHash
		summary.PreparedFileCount = len(report.Snapshot.PreparedFiles)
		summary.UnsupportedFileCount = len(report.Snapshot.UnsupportedFiles)
		summary.SnapshotWarningCount = len(report.Snapshot.Warnings)
	}
	if report.Validation != nil {
		summary.Validation = smartAccountsSyncValidationView{
			Ready:        report.Validation.Summary.Ready,
			Files:        report.Validation.Summary.FilesValidated,
			Rows:         report.Validation.Summary.RowsValidated,
			ErrorCount:   report.Validation.Summary.ErrorCount,
			WarningCount: report.Validation.Summary.WarningCount,
		}
	}
	if report.Plan != nil {
		summary.Plan = smartAccountsSyncPlanView{
			Ready:             report.Plan.Summary.Ready,
			ValidationReady:   report.Plan.Summary.ValidationReady,
			StepCount:         report.Plan.Summary.StepCount,
			ReadyStepCount:    report.Plan.Summary.ReadyStepCount,
			NeedsContextCount: report.Plan.Summary.NeedsContextCount,
			BlockedStepCount:  report.Plan.Summary.BlockedStepCount,
		}
	}
	if report.ExecutionRun != nil {
		summary.Execution = smartAccountsSyncExecutionView{
			RunID:           report.ExecutionRun.ID,
			Status:          report.ExecutionRun.Summary.Status,
			Confirmed:       report.ExecutionRun.Summary.Confirmed,
			ProgressPercent: report.ExecutionRun.Summary.ProgressPercent,
			SucceededSteps:  report.ExecutionRun.Summary.SucceededStepCount,
			FailedSteps:     report.ExecutionRun.Summary.FailedStepCount,
			PlannedSteps:    report.ExecutionRun.Summary.PlannedStepCount,
		}
	}
	summary.Execution.Error = report.ExecutionError
	summary.NextAction = smartAccountsSyncNextAction(summary)
	return summary
}

func smartAccountsSyncReconciliationTargets() []smartAccountsSyncReconciliationTarget {
	return []smartAccountsSyncReconciliationTarget{
		{
			Area:                   "trial_balance",
			SmartAccountsEvidence:  "Trial balance at cutover date.",
			OpenAccountingEvidence: "Trial balance, balance sheet, and account-balance reports for the same date.",
			DiscrepancyRisk:        "Opening balances can be missing or double-counted when full historical GL overlaps the cutover baseline.",
		},
		{
			Area:                   "receivables_payables",
			SmartAccountsEvidence:  "Aged receivables and aged payables at the same as-of date.",
			OpenAccountingEvidence: "Aging reports and balance confirmations, then GL A/R and A/P account balances separately.",
			DiscrepancyRisk:        "Dashboard A/R and A/P are invoice-subledger totals, not GL balances.",
		},
		{
			Area:                   "revenue_expenses",
			SmartAccountsEvidence:  "Income statement or revenue/expense report for the selected period.",
			OpenAccountingEvidence: "Income statement and revenue/expense account balances for the same period.",
			DiscrepancyRisk:        "Imported invoices can populate subledgers without posted GL revenue or expense journals.",
		},
		{
			Area:                   "bank",
			SmartAccountsEvidence:  "Bank balances, bank statements, and unreconciled item reports.",
			OpenAccountingEvidence: "Bank transaction balances, reconciliation closing balances, and GL bank/cash account balances.",
			DiscrepancyRisk:        "Bank transaction balances and GL bank balances are separate accounting bases.",
		},
		{
			Area:                   "vat_tax",
			SmartAccountsEvidence:  "VAT/KMD period summaries and e-MTA evidence.",
			OpenAccountingEvidence: "KMD/INF/OSS reports plus VAT/tax GL account balances for the same period.",
			DiscrepancyRisk:        "VAT reports can mix invoice-line support and posted GL tax account balances.",
		},
		{
			Area:                   "payroll_tsd",
			SmartAccountsEvidence:  "Payroll and TSD period summaries.",
			OpenAccountingEvidence: "Payroll history, TSD history, and payroll/tax GL accounts for the same periods.",
			DiscrepancyRisk:        "Historical payroll imports need finalized source periods and employee identity matching.",
		},
		{
			Area:                   "inventory_fixed_assets",
			SmartAccountsEvidence:  "Inventory valuation and fixed-asset register totals.",
			OpenAccountingEvidence: "Inventory valuation/subledger reconciliation and fixed-asset register totals.",
			DiscrepancyRisk:        "Draft stock or fixed-asset states can differ from posted GL asset balances.",
		},
	}
}

func smartAccountsSyncNextAction(summary smartAccountsSyncPublicSummary) string {
	if summary.UnsupportedFileCount > 0 {
		return "Review unsupported files in the private operator report and add missing exports or mapper support."
	}
	if !summary.Validation.Ready {
		return "Fix validation blockers from the private operator report, then rerun the sync command."
	}
	if !summary.Plan.Ready {
		return "Supply missing context such as bank account id or opening balance entry date, then rerun the sync command."
	}
	if summary.Execution.Confirmed {
		if summary.Execution.FailedSteps > 0 || summary.Execution.Error != "" {
			return "Review the failed confirmed run and resume after correcting the blocker."
		}
		return "Run private reconciliation reports against SmartAccounts totals before closing the cutover."
	}
	return "Review the saved dry run and private operator report; rerun with --confirm only after accountant signoff."
}

func printSmartAccountsSyncSummary(w io.Writer, summary smartAccountsSyncPublicSummary) {
	status := "dry run saved"
	if summary.Execution.Confirmed {
		status = "confirmed"
	}
	if summary.Execution.Error != "" {
		status = "needs attention"
	}
	_, _ = fmt.Fprintf(w, "SmartAccounts sync: %s\n", status)
	if summary.ManifestPath != "" {
		_, _ = fmt.Fprintf(w, "Manifest: %s\n", summary.ManifestPath)
	}
	if summary.OperatorReportPath != "" {
		_, _ = fmt.Fprintf(w, "Operator report: %s\n", summary.OperatorReportPath)
	}
	if summary.SnapshotHash != "" {
		_, _ = fmt.Fprintf(w, "Snapshot hash: %s\n", summary.SnapshotHash)
	}
	_, _ = fmt.Fprintf(
		w,
		"Snapshot: %d prepared, %d unsupported, %d warnings\n",
		summary.PreparedFileCount,
		summary.UnsupportedFileCount,
		summary.SnapshotWarningCount,
	)
	_, _ = fmt.Fprintf(
		w,
		"Validation: ready=%t, files=%d, rows=%d, errors=%d, warnings=%d\n",
		summary.Validation.Ready,
		summary.Validation.Files,
		summary.Validation.Rows,
		summary.Validation.ErrorCount,
		summary.Validation.WarningCount,
	)
	_, _ = fmt.Fprintf(
		w,
		"Plan: ready=%t, steps=%d, ready=%d, needs_context=%d, blocked=%d\n",
		summary.Plan.Ready,
		summary.Plan.StepCount,
		summary.Plan.ReadyStepCount,
		summary.Plan.NeedsContextCount,
		summary.Plan.BlockedStepCount,
	)
	if summary.Execution.RunID != "" {
		_, _ = fmt.Fprintf(
			w,
			"Execution run: %s (%s, %d%% complete, %d succeeded, %d failed, %d planned)\n",
			summary.Execution.RunID,
			summary.Execution.Status,
			summary.Execution.ProgressPercent,
			summary.Execution.SucceededSteps,
			summary.Execution.FailedSteps,
			summary.Execution.PlannedSteps,
		)
	}
	if summary.Execution.Error != "" {
		_, _ = fmt.Fprintf(w, "Execution error: %s\n", summary.Execution.Error)
	}
	if summary.ReconciliationChecks > 0 {
		_, _ = fmt.Fprintf(w, "Reconciliation checks: %d private proof targets in operator report\n", summary.ReconciliationChecks)
	}
	if summary.NextAction != "" {
		_, _ = fmt.Fprintf(w, "Next: %s\n", summary.NextAction)
	}
}
