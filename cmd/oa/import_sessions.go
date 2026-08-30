package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/HMB-research/open-accounting/internal/importsession"
)

const maxImportSessionPackageFileBytes = 2 << 20

// runImportSessions operates the receive-only import-session API. It reads a
// canonical package emitted by a separate source connector; it never contacts
// SmartAccounts itself and has no accounting-import operation.
func (a *cliApp) runImportSessions(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("import-sessions subcommand required")
	}
	cfg, client, err := a.loadAuthenticatedClient()
	if err != nil {
		return err
	}

	switch args[0] {
	case "validate":
		fs := flag.NewFlagSet("import-sessions validate", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		packageFile := fs.String("package", "", "Canonical package JSON file")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if fs.NArg() != 0 {
			return fmt.Errorf("unexpected arguments: %s", strings.Join(fs.Args(), " "))
		}
		pkg, err := readCanonicalImportSessionPackage(*packageFile)
		if err != nil {
			return err
		}
		report, err := client.validateImportSessionPackage(ctx, cfg.TenantID, &importsession.PackageRequest{Package: pkg})
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, report)
		}
		printImportSessionValidation(a.stdout, report)
		return nil

	case "receive":
		fs := flag.NewFlagSet("import-sessions receive", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		packageFile := fs.String("package", "", "Canonical package JSON file")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if fs.NArg() != 0 {
			return fmt.Errorf("unexpected arguments: %s", strings.Join(fs.Args(), " "))
		}
		pkg, err := readCanonicalImportSessionPackage(*packageFile)
		if err != nil {
			return err
		}
		receipt, err := client.receiveImportSession(ctx, cfg.TenantID, &importsession.PackageRequest{Package: pkg})
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, receipt)
		}
		printImportSessionReceipt(a.stdout, receipt)
		return nil

	case "get":
		fs := flag.NewFlagSet("import-sessions get", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		sessionID := fs.String("id", "", "Import session ID")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if fs.NArg() > 1 {
			return errors.New("only one import session id may be provided")
		}
		id := strings.TrimSpace(*sessionID)
		if id != "" && fs.NArg() == 1 {
			return errors.New("use either --id or a positional import session id, not both")
		}
		if id == "" && fs.NArg() == 1 {
			id = strings.TrimSpace(fs.Arg(0))
		}
		if id == "" {
			return errors.New("import session id is required")
		}
		receipt, err := client.getImportSession(ctx, cfg.TenantID, id)
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, receipt)
		}
		printImportSessionReceipt(a.stdout, receipt)
		return nil

	case "plan":
		fs := flag.NewFlagSet("import-sessions plan", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		sessionID := fs.String("id", "", "Import session ID")
		var mappingFlags stringListFlags
		fs.Var(&mappingFlags, "map", "Source account external ID=target Open Accounting account UUID (repeatable)")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if fs.NArg() > 1 {
			return errors.New("only one import session id may be provided")
		}
		id := strings.TrimSpace(*sessionID)
		if id != "" && fs.NArg() == 1 {
			return errors.New("use either --id or a positional import session id, not both")
		}
		if id == "" && fs.NArg() == 1 {
			id = strings.TrimSpace(fs.Arg(0))
		}
		if id == "" {
			return errors.New("import session id is required")
		}
		mappings, err := parseImportSessionAccountMappings(mappingFlags)
		if err != nil {
			return err
		}
		plan, err := client.planImportSession(ctx, cfg.TenantID, id, &importsession.ImportPlanRequest{AccountMappings: mappings})
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, plan)
		}
		printImportSessionPlan(a.stdout, plan)
		return nil

	default:
		return fmt.Errorf("unknown import-sessions subcommand %q", args[0])
	}
}

func parseImportSessionAccountMappings(values []string) ([]importsession.AccountMapping, error) {
	mappings := make([]importsession.AccountMapping, 0, len(values))
	for _, value := range values {
		sourceAccountID, targetAccountID, found := strings.Cut(strings.TrimSpace(value), "=")
		if !found || strings.TrimSpace(sourceAccountID) == "" || strings.TrimSpace(targetAccountID) == "" {
			return nil, fmt.Errorf("invalid --map %q; expected source_account_external_id=target_account_uuid", value)
		}
		mappings = append(mappings, importsession.AccountMapping{
			SourceAccountExternalID: strings.TrimSpace(sourceAccountID),
			TargetAccountID:         strings.TrimSpace(targetAccountID),
		})
	}
	return mappings, nil
}

func readCanonicalImportSessionPackage(path string) (importsession.CanonicalPackage, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return importsession.CanonicalPackage{}, errors.New("package is required")
	}
	info, err := os.Stat(path)
	if err != nil {
		return importsession.CanonicalPackage{}, fmt.Errorf("stat package file: %w", err)
	}
	if info.Size() > maxImportSessionPackageFileBytes {
		return importsession.CanonicalPackage{}, fmt.Errorf("package file exceeds %d byte request limit", maxImportSessionPackageFileBytes)
	}
	payload, err := os.ReadFile(path)
	if err != nil {
		return importsession.CanonicalPackage{}, fmt.Errorf("read package file: %w", err)
	}
	var pkg importsession.CanonicalPackage
	if err := json.Unmarshal(payload, &pkg); err != nil {
		return importsession.CanonicalPackage{}, fmt.Errorf("parse canonical package JSON: %w", err)
	}
	return pkg, nil
}

func printImportSessionValidation(w interface{ Write([]byte) (int, error) }, report *importsession.ValidationReport) {
	if report == nil {
		_, _ = fmt.Fprintln(w, "No import-session validation report")
		return
	}
	status := "ready"
	if !report.Ready {
		status = "blocked"
	} else if report.LedgerVerification != nil && report.LedgerVerification.ReviewRequired {
		status = "review required"
	}
	_, _ = fmt.Fprintf(w, "Import-session validation: %s (%d records, %d issues)\n", status, report.RecordCount, len(report.Issues))
	for _, issue := range report.Issues {
		_, _ = fmt.Fprintf(w, "- %s: %s\n", issue.Code, issue.Message)
	}
}

func printImportSessionReceipt(w interface{ Write([]byte) (int, error) }, receipt *importsession.Receipt) {
	if receipt == nil {
		_, _ = fmt.Fprintln(w, "No import-session receipt")
		return
	}
	state := "already received (idempotent no-op)"
	if receipt.Created {
		state = "received"
	}
	_, _ = fmt.Fprintf(
		w,
		"Import session %s: %s [%s] (%d records, source company %s)\n",
		receipt.ID,
		state,
		receipt.Status,
		receipt.RecordCount,
		receipt.SourceCompanyID,
	)
}

// printImportSessionPlan keeps default terminal output compact and oriented to
// the next review decision. Use --json to inspect the individual planned
// journal lines and reconciliation expectations.
func printImportSessionPlan(w interface{ Write([]byte) (int, error) }, plan *importsession.ImportPlanResult) {
	if plan == nil {
		_, _ = fmt.Fprintln(w, "No import-session plan")
		return
	}
	status := "ready"
	if !plan.Ready {
		status = "blocked"
	}
	_, _ = fmt.Fprintf(
		w,
		"Import-session plan: %s (financial writes planned: %t, journals: %d, accounts: %d)\n",
		status,
		plan.FinancialWritesPlanned,
		len(plan.JournalActions),
		len(plan.AccountReconciliations),
	)
	if plan.PlanSHA256 != "" {
		_, _ = fmt.Fprintf(w, "Plan digest: %s\n", plan.PlanSHA256)
	}
	for _, issue := range plan.Issues {
		_, _ = fmt.Fprintf(w, "- %s: %s\n", issue.Code, issue.Message)
	}
}
