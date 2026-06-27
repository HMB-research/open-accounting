package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/HMB-research/open-accounting/internal/cutover"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	fs := flag.NewFlagSet("smartaccounts-snapshot", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
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
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(report)
	}

	fmt.Printf("SmartAccounts snapshot prepared\n")
	fmt.Printf("Manifest: %s\n", report.ManifestPath)
	fmt.Printf("Snapshot hash: %s\n", report.SnapshotHash)
	fmt.Printf("Prepared files: %d\n", len(report.PreparedFiles))
	if len(report.UnsupportedFiles) > 0 {
		fmt.Printf("Unsupported files: %d\n", len(report.UnsupportedFiles))
	}
	if len(report.Warnings) > 0 {
		fmt.Printf("Warnings: %d\n", len(report.Warnings))
	}
	if report.ValidationCommand != "" {
		fmt.Printf("\nValidate bundle:\n%s\n", report.ValidationCommand)
	}
	return nil
}
