package payroll

import (
	"fmt"
	"strings"

	"github.com/HMB-research/open-accounting/internal/workspace"
)

// BuildTSDRemediationActions turns a TSD declaration status into accountant follow-up actions.
func BuildTSDRemediationActions(declaration *TSDDeclaration) []TSDRemediationAction {
	if declaration == nil {
		return nil
	}

	period := fmt.Sprintf("%04d-%02d", declaration.PeriodYear, declaration.PeriodMonth)
	periodFlags := fmt.Sprintf("--year %d --month %d", declaration.PeriodYear, declaration.PeriodMonth)
	uiPath := fmt.Sprintf("/tsd?year=%d&month=%d", declaration.PeriodYear, declaration.PeriodMonth)
	base := TSDRemediationAction{
		Scope:      "tax",
		OwnerRole:  "accountant",
		Period:     period,
		EntityType: "tsd_declaration",
		EntityID:   declaration.ID,
		UIPath:     uiPath,
	}

	action := func(code, severity, message, text, command string) TSDRemediationAction {
		item := base
		item.Code = code
		item.Severity = severity
		meta := workspace.RemediationAssignment(
			"tsd_declarations",
			code,
			severity,
			item.EntityType,
			item.EntityID,
			item.Period,
		)
		item.WorkspaceQueue = meta.WorkspaceQueue
		item.AssignmentKey = meta.AssignmentKey
		item.Priority = meta.Priority
		item.DueInDays = meta.DueInDays
		item.Message = message
		item.Action = text
		item.CLICommand = command
		return item
	}

	generateCommand := fmt.Sprintf("oa tsd get %s", periodFlags)
	if strings.TrimSpace(declaration.PayrollRunID) != "" {
		generateCommand = fmt.Sprintf("oa tsd generate --run-id %s", declaration.PayrollRunID)
	}

	status := TSDStatus(strings.ToUpper(strings.TrimSpace(string(declaration.Status))))
	switch status {
	case TSDAccepted:
		return []TSDRemediationAction{
			action(
				"tsd_accepted_archive",
				"INFO",
				fmt.Sprintf("TSD %s is accepted.", period),
				"Archive the accepted declaration XML or CSV and supporting payroll evidence with the period close pack.",
				fmt.Sprintf("oa tsd export-xml %s --output ./tsd-%s.xml", periodFlags, period),
			),
		}
	case TSDSubmitted:
		actions := []TSDRemediationAction{
			action(
				"tsd_awaiting_authority_acceptance",
				"ACTION",
				fmt.Sprintf("TSD %s has been submitted and is awaiting authority acceptance.", period),
				"Monitor e-MTA acceptance, mark the declaration accepted or rejected, and retain the accepted confirmation.",
				fmt.Sprintf("oa tsd mark-accepted %s", periodFlags),
			),
		}
		if declaration.SubmittedAt == nil {
			actions = append(actions, action(
				"tsd_submission_date_missing",
				"WARNING",
				fmt.Sprintf("TSD %s is marked submitted without a submitted_at timestamp.", period),
				"Record the submission timestamp or re-import the historical TSD period with submitted_at populated.",
				"oa tsd import-history --file ./tsd-history.csv",
			))
		}
		return actions
	case TSDRejected:
		return []TSDRemediationAction{
			action(
				"tsd_rejected_review",
				"ACTION",
				fmt.Sprintf("TSD %s was rejected by e-MTA or marked rejected.", period),
				"Review the rejection, correct payroll or historical declaration data, regenerate/export the declaration, and resubmit it.",
				generateCommand,
			),
		}
	}

	actions := make([]TSDRemediationAction, 0, 2)
	if len(declaration.Rows) == 0 && declaration.TotalPayments.IsZero() {
		actions = append(actions, action(
			"tsd_no_declaration_rows",
			"WARNING",
			fmt.Sprintf("TSD %s has no Annex 1 rows or payment totals.", period),
			"Confirm the period has no salary payments, or calculate payroll and regenerate TSD before export.",
			generateCommand,
		))
	}

	if status != TSDDraft && status != "" {
		return append(actions, action(
			"tsd_status_review",
			"WARNING",
			fmt.Sprintf("TSD %s has unsupported status %q.", period, declaration.Status),
			"Review the declaration status and reconcile it with e-MTA before filing or closing the period.",
			fmt.Sprintf("oa tsd get %s", periodFlags),
		))
	}

	return append(actions, action(
		"tsd_export_and_submit",
		"ACTION",
		fmt.Sprintf("TSD %s is ready for export and submission review.", period),
		"Review declaration totals, export XML or CSV, submit through e-MTA, and mark the declaration submitted with the e-MTA reference.",
		fmt.Sprintf("oa tsd export-xml %s --output ./tsd-%s.xml", periodFlags, period),
	))
}
