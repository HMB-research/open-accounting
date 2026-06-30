package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/HMB-research/open-accounting/internal/cutover"
	"github.com/HMB-research/open-accounting/internal/reports"
)

const (
	smartAccountsProofPlanName   = "smartaccounts-proof-plan.json"
	smartAccountsProofScriptName = "open-accounting-proof-commands.sh"
)

type smartAccountsProofPlan struct {
	Provider           cutover.MigrationProviderPreset `json:"provider"`
	GeneratedAt        string                          `json:"generated_at"`
	TenantID           string                          `json:"tenant_id"`
	SyncReportPath     string                          `json:"sync_report_path"`
	OutputDir          string                          `json:"output_dir"`
	PlanPath           string                          `json:"plan_path"`
	ScriptPath         string                          `json:"script_path"`
	AsOfDate           string                          `json:"as_of_date"`
	StartDate          string                          `json:"start_date"`
	EndDate            string                          `json:"end_date"`
	CashFlowMethod     string                          `json:"cash_flow_method"`
	BankAccountID      string                          `json:"bank_account_id,omitempty"`
	KMDYear            int                             `json:"kmd_year"`
	KMDMonth           int                             `json:"kmd_month"`
	TSDYear            int                             `json:"tsd_year"`
	TSDMonth           int                             `json:"tsd_month"`
	InventoryMethod    string                          `json:"inventory_method,omitempty"`
	WarehouseID        string                          `json:"warehouse_id,omitempty"`
	Items              []smartAccountsProofPlanItem    `json:"items"`
	MissingContext     []string                        `json:"missing_context,omitempty"`
	RequiredCommands   int                             `json:"required_commands"`
	OptionalCommands   int                             `json:"optional_commands"`
	ReadyForPrivateRun bool                            `json:"ready_for_private_run"`
	NextAction         string                          `json:"next_action"`
}

type smartAccountsProofPlanItem struct {
	Area                   string                          `json:"area"`
	Status                 string                          `json:"status"`
	SmartAccountsEvidence  string                          `json:"smartaccounts_evidence"`
	OpenAccountingEvidence string                          `json:"open_accounting_evidence"`
	DiscrepancyRisk        string                          `json:"discrepancy_risk"`
	Commands               []smartAccountsProofPlanCommand `json:"commands"`
	MissingContext         []string                        `json:"missing_context,omitempty"`
	Notes                  []string                        `json:"notes,omitempty"`
	NextAction             string                          `json:"next_action"`
}

type smartAccountsProofPlanCommand struct {
	Label          string `json:"label"`
	Command        string `json:"command"`
	OutputPath     string `json:"output_path"`
	Required       bool   `json:"required"`
	CapturesStdout bool   `json:"captures_stdout"`
}

func (a *cliApp) runMigrationSmartAccountsProofPlan(cfg *cliConfig, args []string) error {
	fs := flag.NewFlagSet("migration smartaccounts-proof-plan", flag.ContinueOnError)
	fs.SetOutput(a.stderr)
	reportPath := fs.String("report", "", "Private smartaccounts-sync-report.json path")
	outputDir := fs.String("out-dir", "", "Private directory for proof manifest and Open Accounting report artifacts")
	asOf := fs.String("as-of", "", "As-of date for balance-sheet, trial-balance, AR/AP, and subledger proof in YYYY-MM-DD")
	startDate := fs.String("start", "", "Period start date for income statement and cash flow proof in YYYY-MM-DD")
	endDate := fs.String("end", "", "Period end date for income statement, cash flow, KMD, and TSD proof in YYYY-MM-DD")
	cashFlowMethod := fs.String("cash-flow-method", reports.CashFlowMethodIndirect, "Cash flow method: direct or indirect")
	bankAccountID := fs.String("bank-account-id", "", "Optional Open Accounting bank account id for bank transactions and reconciliations")
	kmdYearFlag := fs.String("kmd-year", "", "Optional KMD proof year; defaults to --end year")
	kmdMonthFlag := fs.String("kmd-month", "", "Optional KMD proof month; defaults to --end month")
	tsdYearFlag := fs.String("tsd-year", "", "Optional TSD proof year; defaults to --end year")
	tsdMonthFlag := fs.String("tsd-month", "", "Optional TSD proof month; defaults to --end month")
	inventoryMethod := fs.String("inventory-method", "weighted-average", "Inventory valuation method: standard-cost, weighted-average, or fifo")
	warehouseID := fs.String("warehouse-id", "", "Optional warehouse id for inventory proof")
	asJSON := fs.Bool("json", false, "Output proof plan JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*reportPath) == "" || strings.TrimSpace(*outputDir) == "" {
		return errors.New("report and out-dir are required")
	}
	asOfDate, err := parseRequiredDate("as-of", *asOf)
	if err != nil {
		return err
	}
	periodStart, err := parseRequiredDate("start", *startDate)
	if err != nil {
		return err
	}
	periodEnd, err := parseRequiredDate("end", *endDate)
	if err != nil {
		return err
	}
	if periodEnd.Before(periodStart) {
		return errors.New("end must be on or after start")
	}
	method, err := reports.NormalizeCashFlowMethod(*cashFlowMethod)
	if err != nil {
		return err
	}
	kmdYear, kmdMonth, err := smartAccountsProofPeriodFromFlags("kmd", *kmdYearFlag, *kmdMonthFlag, periodEnd)
	if err != nil {
		return err
	}
	tsdYear, tsdMonth, err := smartAccountsProofPeriodFromFlags("tsd", *tsdYearFlag, *tsdMonthFlag, periodEnd)
	if err != nil {
		return err
	}
	reportAbs, _ := filepath.Abs(strings.TrimSpace(*reportPath))
	outAbs, _ := filepath.Abs(strings.TrimSpace(*outputDir))
	if err := rejectSmartAccountsProofPublicWorktreePath(outAbs); err != nil {
		return err
	}

	reportPayload, err := os.ReadFile(reportAbs)
	if err != nil {
		return fmt.Errorf("read SmartAccounts sync report: %w", err)
	}
	var syncReport smartAccountsSyncPrivateReport
	if err := json.Unmarshal(reportPayload, &syncReport); err != nil {
		return fmt.Errorf("decode SmartAccounts sync report: %w", err)
	}
	if syncReport.Provider != "" && syncReport.Provider != cutover.MigrationProviderPresetSmartAccounts {
		return fmt.Errorf("sync report provider must be %q", cutover.MigrationProviderPresetSmartAccounts)
	}
	if len(syncReport.ParityChecklist) == 0 {
		syncReport.ParityChecklist = smartAccountsSyncParityChecklist(&syncReport)
	}

	plan := smartAccountsProofPlan{
		Provider:        cutover.MigrationProviderPresetSmartAccounts,
		GeneratedAt:     time.Now().UTC().Format(time.RFC3339),
		TenantID:        smartAccountsProofFirstNonEmpty(syncReport.TenantID, cfg.TenantID),
		SyncReportPath:  reportAbs,
		OutputDir:       outAbs,
		PlanPath:        filepath.Join(outAbs, smartAccountsProofPlanName),
		ScriptPath:      filepath.Join(outAbs, smartAccountsProofScriptName),
		AsOfDate:        asOfDate.Format("2006-01-02"),
		StartDate:       periodStart.Format("2006-01-02"),
		EndDate:         periodEnd.Format("2006-01-02"),
		CashFlowMethod:  method,
		BankAccountID:   strings.TrimSpace(*bankAccountID),
		KMDYear:         kmdYear,
		KMDMonth:        kmdMonth,
		TSDYear:         tsdYear,
		TSDMonth:        tsdMonth,
		InventoryMethod: strings.TrimSpace(*inventoryMethod),
		WarehouseID:     strings.TrimSpace(*warehouseID),
	}
	plan.Items = smartAccountsProofPlanItems(&plan, syncReport.ParityChecklist)
	for _, item := range plan.Items {
		plan.MissingContext = append(plan.MissingContext, item.MissingContext...)
		for _, command := range item.Commands {
			if command.Required {
				plan.RequiredCommands++
			} else {
				plan.OptionalCommands++
			}
		}
	}
	plan.MissingContext = uniqueNonEmptyStrings(plan.MissingContext)
	plan.ReadyForPrivateRun = len(plan.MissingContext) == 0
	plan.NextAction = "Run the generated shell script in a private operator shell, then compare each Open Accounting artifact with the matching SmartAccounts proof report before marking parity passed."
	if !plan.ReadyForPrivateRun {
		plan.NextAction = "Supply missing context, rerun the proof-plan command, then run the generated shell script in a private operator shell."
	}

	if err := os.MkdirAll(outAbs, 0o700); err != nil {
		return fmt.Errorf("create proof output dir: %w", err)
	}
	planPayload, _ := json.MarshalIndent(plan, "", "  ")
	planPayload = append(planPayload, '\n')
	if err := os.WriteFile(plan.PlanPath, planPayload, 0o600); err != nil {
		return fmt.Errorf("write SmartAccounts proof plan: %w", err)
	}
	if err := writeSmartAccountsProofScript(plan.ScriptPath, &plan); err != nil {
		return err
	}

	if *asJSON {
		return printJSON(a.stdout, &plan)
	}
	printSmartAccountsProofPlanSummary(a.stdout, &plan)
	return nil
}

func smartAccountsProofPeriodFromFlags(name, yearValue, monthValue string, fallback time.Time) (int, int, error) {
	yearTrimmed := strings.TrimSpace(yearValue)
	monthTrimmed := strings.TrimSpace(monthValue)
	if yearTrimmed == "" && monthTrimmed == "" {
		return fallback.Year(), int(fallback.Month()), nil
	}
	if yearTrimmed == "" || monthTrimmed == "" {
		return 0, 0, fmt.Errorf("%s-year and %s-month must be supplied together", name, name)
	}
	year, month, err := parseYearMonthFlags(yearTrimmed, monthTrimmed)
	if err != nil {
		return 0, 0, err
	}
	return year, month, nil
}

func smartAccountsProofPlanItems(plan *smartAccountsProofPlan, checklist []smartAccountsSyncParityChecklistItem) []smartAccountsProofPlanItem {
	items := make([]smartAccountsProofPlanItem, 0, len(checklist))
	for _, checklistItem := range checklist {
		item := smartAccountsProofPlanItem{
			Area:                   checklistItem.Area,
			Status:                 checklistItem.Status,
			SmartAccountsEvidence:  checklistItem.SmartAccountsEvidence,
			OpenAccountingEvidence: checklistItem.OpenAccountingEvidence,
			DiscrepancyRisk:        checklistItem.DiscrepancyRisk,
			NextAction:             "Compare generated Open Accounting artifacts against private SmartAccounts evidence for this area.",
		}
		switch checklistItem.Area {
		case "trial_balance":
			item.Commands = append(item.Commands,
				smartAccountsProofJSONCommand(plan, item.Area, "trial balance JSON", "trial-balance.json", "reports", "trial-balance", "--as-of", plan.AsOfDate, "--json"),
				smartAccountsProofFileCommand(plan, item.Area, "trial balance CSV", "trial-balance.csv", false, "reports", "trial-balance", "--as-of", plan.AsOfDate, "--csv"),
				smartAccountsProofJSONCommand(plan, item.Area, "balance sheet JSON", "balance-sheet.json", "reports", "balance-sheet", "--as-of", plan.AsOfDate, "--json"),
				smartAccountsProofFileCommand(plan, item.Area, "balance sheet CSV", "balance-sheet.csv", false, "reports", "balance-sheet", "--as-of", plan.AsOfDate, "--csv"),
			)
			item.Notes = append(item.Notes, "Add targeted reports account-balance commands manually for material cash, receivable, payable, VAT, inventory, asset, payroll, revenue, and expense GL account ids.")
		case "receivables_payables":
			item.Commands = append(item.Commands,
				smartAccountsProofJSONCommand(plan, item.Area, "receivables aging JSON", "receivables-aging.json", "reports", "aging", "--type", "receivables", "--json"),
				smartAccountsProofFileCommand(plan, item.Area, "receivables aging CSV", "receivables-aging.csv", false, "reports", "aging", "--type", "receivables", "--csv"),
				smartAccountsProofJSONCommand(plan, item.Area, "payables aging JSON", "payables-aging.json", "reports", "aging", "--type", "payables", "--json"),
				smartAccountsProofFileCommand(plan, item.Area, "payables aging CSV", "payables-aging.csv", false, "reports", "aging", "--type", "payables", "--csv"),
				smartAccountsProofJSONCommand(plan, item.Area, "receivable balance confirmations JSON", "receivable-balance-confirmations.json", "reports", "balance-confirmations", "--type", "RECEIVABLE", "--as-of", plan.AsOfDate, "--json"),
				smartAccountsProofFileCommand(plan, item.Area, "receivable balance confirmations CSV", "receivable-balance-confirmations.csv", false, "reports", "balance-confirmations", "--type", "RECEIVABLE", "--as-of", plan.AsOfDate, "--csv"),
				smartAccountsProofJSONCommand(plan, item.Area, "payable balance confirmations JSON", "payable-balance-confirmations.json", "reports", "balance-confirmations", "--type", "PAYABLE", "--as-of", plan.AsOfDate, "--json"),
				smartAccountsProofFileCommand(plan, item.Area, "payable balance confirmations CSV", "payable-balance-confirmations.csv", false, "reports", "balance-confirmations", "--type", "PAYABLE", "--as-of", plan.AsOfDate, "--csv"),
			)
		case "revenue_expenses":
			item.Commands = append(item.Commands,
				smartAccountsProofJSONCommand(plan, item.Area, "income statement JSON", "income-statement.json", "reports", "income-statement", "--start", plan.StartDate, "--end", plan.EndDate, "--json"),
				smartAccountsProofFileCommand(plan, item.Area, "income statement CSV", "income-statement.csv", false, "reports", "income-statement", "--start", plan.StartDate, "--end", plan.EndDate, "--csv"),
			)
			item.Notes = append(item.Notes, "Use account-balance proof manually for material revenue and expense GL account ids if the SmartAccounts comparison report is account-level.")
		case "bank":
			item.Commands = append(item.Commands,
				smartAccountsProofJSONCommand(plan, item.Area, "bank accounts JSON", "bank-accounts.json", "banking", "accounts", "list", "--json"),
				smartAccountsProofJSONCommand(plan, item.Area, "cash flow JSON", "cash-flow.json", "reports", "cash-flow", "--start", plan.StartDate, "--end", plan.EndDate, "--method", plan.CashFlowMethod, "--json"),
				smartAccountsProofFileCommand(plan, item.Area, "cash flow CSV", "cash-flow.csv", false, "reports", "cash-flow", "--start", plan.StartDate, "--end", plan.EndDate, "--method", plan.CashFlowMethod, "--csv"),
			)
			if plan.BankAccountID == "" {
				item.MissingContext = append(item.MissingContext, "bank-account-id is required for bank transaction and reconciliation proof commands")
			} else {
				item.Commands = append(item.Commands,
					smartAccountsProofJSONCommand(plan, item.Area, "bank transactions JSON", "bank-transactions.json", "banking", "transactions", "list", "--account-id", plan.BankAccountID, "--from", plan.StartDate, "--to", plan.EndDate, "--json"),
					smartAccountsProofJSONCommand(plan, item.Area, "bank reconciliations JSON", "bank-reconciliations.json", "banking", "reconciliations", "list", "--account-id", plan.BankAccountID, "--json"),
				)
			}
		case "vat_tax":
			item.Commands = append(item.Commands,
				smartAccountsProofJSONCommand(plan, item.Area, "KMD declarations JSON", "kmd-declarations.json", "tax", "kmd", "list", "--json"),
				smartAccountsProofJSONCommand(plan, item.Area, "KMD INF JSON", "kmd-inf.json", "tax", "kmd", "inf", "--year", fmt.Sprint(plan.KMDYear), "--month", fmt.Sprint(plan.KMDMonth), "--json"),
				smartAccountsProofFileCommand(plan, item.Area, "KMD XML", "kmd.xml", false, "tax", "kmd", "export-xml", "--year", fmt.Sprint(plan.KMDYear), "--month", fmt.Sprint(plan.KMDMonth)),
			)
			item.Notes = append(item.Notes, "The proof script reads existing KMD evidence; generate or import reviewed KMD declarations before running exports if none exist.")
		case "payroll_tsd":
			item.Commands = append(item.Commands,
				smartAccountsProofJSONCommand(plan, item.Area, "payroll runs JSON", "payroll-runs.json", "payroll", "runs", "list", "--year", fmt.Sprint(plan.TSDYear), "--json"),
				smartAccountsProofJSONCommand(plan, item.Area, "TSD declaration JSON", "tsd-declaration.json", "tsd", "get", "--year", fmt.Sprint(plan.TSDYear), "--month", fmt.Sprint(plan.TSDMonth), "--json"),
				smartAccountsProofFileCommand(plan, item.Area, "TSD CSV", "tsd.csv", false, "tsd", "export-csv", "--year", fmt.Sprint(plan.TSDYear), "--month", fmt.Sprint(plan.TSDMonth)),
				smartAccountsProofFileCommand(plan, item.Area, "TSD XML", "tsd.xml", false, "tsd", "export-xml", "--year", fmt.Sprint(plan.TSDYear), "--month", fmt.Sprint(plan.TSDMonth)),
			)
		case "inventory_fixed_assets":
			item.Commands = append(item.Commands,
				smartAccountsProofJSONCommand(plan, item.Area, "inventory valuation JSON", "inventory-valuation.json", "inventory", "valuation", "--method", plan.InventoryMethod, "--json"),
				smartAccountsProofJSONCommand(plan, item.Area, "inventory subledger reconciliation JSON", "inventory-subledger-reconciliation.json", "inventory", "subledger-reconciliation", "--method", plan.InventoryMethod, "--as-of", plan.AsOfDate, "--json"),
				smartAccountsProofJSONCommand(plan, item.Area, "fixed assets JSON", "fixed-assets.json", "assets", "list", "--json"),
			)
			if plan.WarehouseID != "" {
				item.Commands = append(item.Commands,
					smartAccountsProofJSONCommand(plan, item.Area, "warehouse inventory valuation JSON", "warehouse-inventory-valuation.json", "inventory", "valuation", "--warehouse-id", plan.WarehouseID, "--method", plan.InventoryMethod, "--json"),
					smartAccountsProofJSONCommand(plan, item.Area, "warehouse inventory subledger reconciliation JSON", "warehouse-inventory-subledger-reconciliation.json", "inventory", "subledger-reconciliation", "--warehouse-id", plan.WarehouseID, "--method", plan.InventoryMethod, "--as-of", plan.AsOfDate, "--json"),
				)
			}
			item.Notes = append(item.Notes, "Fixed-asset register proof is currently JSON asset list output; a dedicated fixed-asset register export remains a report surface gap.")
		default:
			item.MissingContext = append(item.MissingContext, "no proof command mapping exists for parity area "+checklistItem.Area)
			item.NextAction = "Add proof command coverage for this parity area before relying on this plan."
		}
		items = append(items, item)
	}
	return items
}

func smartAccountsProofJSONCommand(plan *smartAccountsProofPlan, area, label, fileName string, parts ...string) smartAccountsProofPlanCommand {
	return smartAccountsProofPlanCommand{
		Label:          label,
		Command:        smartAccountsProofCommand(parts...),
		OutputPath:     filepath.Join(plan.OutputDir, area, fileName),
		Required:       true,
		CapturesStdout: true,
	}
}

func smartAccountsProofFileCommand(plan *smartAccountsProofPlan, area, label, fileName string, required bool, parts ...string) smartAccountsProofPlanCommand {
	outputPath := filepath.Join(plan.OutputDir, area, fileName)
	withOutput := append(append([]string{}, parts...), "--output", outputPath)
	return smartAccountsProofPlanCommand{
		Label:      label,
		Command:    smartAccountsProofCommand(withOutput...),
		OutputPath: outputPath,
		Required:   required,
	}
}

func smartAccountsProofCommand(parts ...string) string {
	quoted := make([]string, 0, len(parts)+1)
	quoted = append(quoted, "oa")
	for _, part := range parts {
		quoted = append(quoted, smartAccountsProofShellQuote(part))
	}
	return strings.Join(quoted, " ")
}

func writeSmartAccountsProofScript(path string, plan *smartAccountsProofPlan) error {
	var builder strings.Builder
	builder.WriteString("#!/bin/sh\n")
	builder.WriteString("set -eu\n\n")
	builder.WriteString("# Generated from a private SmartAccounts sync report. Keep outputs private.\n")
	builder.WriteString("command -v oa >/dev/null 2>&1 || { echo 'oa CLI not found on PATH' >&2; exit 127; }\n\n")

	dirs := []string{plan.OutputDir}
	for _, item := range plan.Items {
		dirs = append(dirs, filepath.Join(plan.OutputDir, item.Area))
	}
	dirs = uniqueNonEmptyStrings(dirs)
	builder.WriteString("mkdir -p")
	for _, dir := range dirs {
		builder.WriteByte(' ')
		builder.WriteString(smartAccountsProofShellQuote(dir))
	}
	builder.WriteString("\n\n")

	for _, item := range plan.Items {
		builder.WriteString("echo ")
		builder.WriteString(smartAccountsProofShellQuote("== " + item.Area + " =="))
		builder.WriteByte('\n')
		for _, command := range item.Commands {
			builder.WriteString("echo ")
			builder.WriteString(smartAccountsProofShellQuote(command.Label))
			builder.WriteByte('\n')
			if command.CapturesStdout {
				builder.WriteString(command.Command)
				builder.WriteString(" > ")
				builder.WriteString(smartAccountsProofShellQuote(command.OutputPath))
				builder.WriteByte('\n')
			} else {
				builder.WriteString(command.Command)
				builder.WriteByte('\n')
			}
		}
		builder.WriteByte('\n')
	}
	builder.WriteString("echo ")
	builder.WriteString(smartAccountsProofShellQuote("Proof artifacts written to " + plan.OutputDir))
	builder.WriteByte('\n')

	if err := os.WriteFile(path, []byte(builder.String()), 0o700); err != nil {
		return fmt.Errorf("write SmartAccounts proof script: %w", err)
	}
	return nil
}

func rejectSmartAccountsProofPublicWorktreePath(path string) error {
	absPath, _ := filepath.Abs(path)
	root, ok := smartAccountsProofOpenAccountingRootForPath(absPath)
	if ok && pathWithin(absPath, root) {
		return fmt.Errorf("SmartAccounts proof output must not be inside public Open Accounting Git worktree %s; use a private directory or separate private repository", root)
	}
	return nil
}

func smartAccountsProofOpenAccountingRootForPath(path string) (string, bool) {
	dir := filepath.Clean(path)
	if info, err := os.Stat(dir); err == nil && !info.IsDir() {
		dir = filepath.Dir(dir)
	}
	for {
		modulePath := filepath.Join(dir, "go.mod")
		moduleFile, err := os.ReadFile(modulePath)
		if err == nil && strings.Contains(string(moduleFile), "module github.com/HMB-research/open-accounting") {
			return dir, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}

func pathWithin(path, root string) bool {
	rel, _ := filepath.Rel(root, path)
	return rel == "." || (rel != "" && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

func smartAccountsProofShellQuote(value string) string {
	if value == "" {
		return "''"
	}
	if strings.IndexFunc(value, func(r rune) bool {
		return r != '/' && r != '.' && r != '_' && r != '-' && r != ':' && (r < '0' || r > '9') && (r < 'A' || r > 'Z') && (r < 'a' || r > 'z')
	}) == -1 {
		return value
	}
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

func uniqueNonEmptyStrings(values []string) []string {
	seen := map[string]bool{}
	unique := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" || seen[trimmed] {
			continue
		}
		seen[trimmed] = true
		unique = append(unique, trimmed)
	}
	return unique
}

func smartAccountsProofFirstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func printSmartAccountsProofPlanSummary(w io.Writer, plan *smartAccountsProofPlan) {
	_, _ = fmt.Fprintln(w, "SmartAccounts proof plan written")
	_, _ = fmt.Fprintf(w, "Plan: %s\n", plan.PlanPath)
	_, _ = fmt.Fprintf(w, "Script: %s\n", plan.ScriptPath)
	_, _ = fmt.Fprintf(w, "Items: %d\n", len(plan.Items))
	_, _ = fmt.Fprintf(w, "Commands: required=%d optional=%d\n", plan.RequiredCommands, plan.OptionalCommands)
	if len(plan.MissingContext) > 0 {
		_, _ = fmt.Fprintf(w, "Missing context: %d\n", len(plan.MissingContext))
		for _, missing := range plan.MissingContext {
			_, _ = fmt.Fprintf(w, "- %s\n", missing)
		}
	}
	_, _ = fmt.Fprintf(w, "Next: %s\n", plan.NextAction)
}
