package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/HMB-research/open-accounting/internal/cutover"
)

var (
	commandArgs             = func() []string { return os.Args[1:] }
	commandStdout io.Writer = os.Stdout
	commandStderr io.Writer = os.Stderr
	exitProcess             = os.Exit
)

func main() {
	if err := run(commandArgs(), commandStdout, commandStderr); err != nil {
		_, _ = fmt.Fprintln(commandStderr, "Error:", err)
		exitProcess(1)
	}
}

func run(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("smartaccounts-snapshot", flag.ContinueOnError)
	fs.SetOutput(stderr)
	sourceDir := fs.String("source-dir", "", "Directory containing SmartAccounts CSV/XML exports")
	outputDir := fs.String("out-dir", "", "Directory where manifest.json and bundle files are written")
	companyID := fs.String("company-id", "", "Source company registry or SmartAccounts company id")
	companyName := fs.String("company-name", "", "Source company name")
	cutoverDate := fs.String("cutover-date", "", "Accounting cutover date in YYYY-MM-DD")
	asJSON := fs.Bool("json", false, "Output manifest JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*sourceDir) == "" || strings.TrimSpace(*outputDir) == "" {
		return fmt.Errorf("source-dir and out-dir are required")
	}

	report, err := cutover.PrepareSmartAccountsSnapshot(cutover.SmartAccountsSnapshotOptions{
		SourceDir:         strings.TrimSpace(*sourceDir),
		OutputDir:         strings.TrimSpace(*outputDir),
		SourceCompanyID:   strings.TrimSpace(*companyID),
		SourceCompanyName: strings.TrimSpace(*companyName),
		CutoverDate:       strings.TrimSpace(*cutoverDate),
		GeneratedAt:       time.Now().UTC(),
	})
	if err != nil {
		return err
	}

	if *asJSON {
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(report)
	}

	_, _ = fmt.Fprintf(stdout, "SmartAccounts snapshot prepared\n")
	_, _ = fmt.Fprintf(stdout, "Manifest: %s\n", report.ManifestPath)
	_, _ = fmt.Fprintf(stdout, "Snapshot hash: %s\n", report.SnapshotHash)
	_, _ = fmt.Fprintf(stdout, "Prepared files: %d\n", len(report.PreparedFiles))
	if len(report.UnsupportedFiles) > 0 {
		_, _ = fmt.Fprintf(stdout, "Unsupported files: %d\n", len(report.UnsupportedFiles))
	}
	if len(report.Warnings) > 0 {
		_, _ = fmt.Fprintf(stdout, "Warnings: %d\n", len(report.Warnings))
	}
	if report.ValidationCommand != "" {
		_, _ = fmt.Fprintf(stdout, "\nValidate bundle:\n%s\n", report.ValidationCommand)
	}
	return nil
}
