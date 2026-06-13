package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"strings"
)

func (a *cliApp) runMigrationRuns(ctx context.Context, cfg *cliConfig, client *apiClient, args []string) error {
	if len(args) == 0 {
		return errors.New("migration runs subcommand required")
	}
	switch args[0] {
	case "list":
		fs := flag.NewFlagSet("migration runs list", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		status := fs.String("status", "", "Filter by run status")
		limit := fs.Int("limit", 50, "Maximum runs to return")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		runs, err := client.listMigrationExecutionRuns(ctx, cfg.TenantID, strings.TrimSpace(*status), *limit)
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, runs)
		}
		printMigrationExecutionRunsTable(a.stdout, runs)
		return nil
	case "get":
		fs := flag.NewFlagSet("migration runs get", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		runID := fs.String("id", "", "Migration execution run ID")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		id := strings.TrimSpace(*runID)
		if id == "" && fs.NArg() > 0 {
			id = strings.TrimSpace(fs.Arg(0))
		}
		if id == "" {
			return errors.New("migration run id is required")
		}
		run, err := client.getMigrationExecutionRun(ctx, cfg.TenantID, id)
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, run)
		}
		printMigrationExecutionRun(a.stdout, run)
		return nil
	default:
		return fmt.Errorf("unknown migration runs subcommand %q", args[0])
	}
}
