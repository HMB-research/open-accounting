package payroll

import (
	"fmt"
	"strings"

	"github.com/HMB-research/open-accounting/internal/workspace"
)

// BuildPayrollRunRemediationActions turns payroll run state into accountant follow-up actions.
func BuildPayrollRunRemediationActions(run *PayrollRun) []PayrollRunRemediationAction {
	if run == nil {
		return nil
	}

	period := fmt.Sprintf("%04d-%02d", run.PeriodYear, run.PeriodMonth)
	uiPath := fmt.Sprintf("/payroll?run_id=%s", run.ID)
	base := PayrollRunRemediationAction{
		Scope:      "payroll",
		OwnerRole:  "accountant",
		Period:     period,
		EntityType: "payroll_run",
		EntityID:   run.ID,
		UIPath:     uiPath,
	}

	action := func(code, severity, message, text, command string) PayrollRunRemediationAction {
		item := base
		item.Code = code
		item.Severity = severity
		meta := workspace.RemediationAssignment(
			"payroll_runs",
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

	actions := make([]PayrollRunRemediationAction, 0, 3)
	status := PayrollStatus(strings.ToUpper(strings.TrimSpace(string(run.Status))))

	if run.PaymentDate == nil && status != PayrollDeclared {
		actions = append(actions, action(
			"payroll_payment_date_missing",
			"WARNING",
			fmt.Sprintf("Payroll run %s has no payment date.", period),
			"Confirm the intended salary payment date before approving payroll or filing TSD.",
			fmt.Sprintf("oa payroll runs get --id %s", run.ID),
		))
	}

	switch status {
	case PayrollDraft:
		actions = append(actions, action(
			"payroll_run_calculate",
			"ACTION",
			fmt.Sprintf("Payroll run %s is still in draft.", period),
			"Verify active employees and salary components, then calculate payslips for the period.",
			fmt.Sprintf("oa payroll runs calculate --id %s", run.ID),
		))
	case PayrollCalculated:
		if run.TotalGross.IsZero() && len(run.Payslips) == 0 {
			actions = append(actions, action(
				"payroll_no_payslips",
				"WARNING",
				fmt.Sprintf("Payroll run %s calculated with no gross pay or payslips.", period),
				"Confirm this is an intentional zero-payroll period, or add salary setup and recalculate before approval.",
				fmt.Sprintf("oa payroll runs calculate --id %s", run.ID),
			))
		}
		actions = append(actions, action(
			"payroll_run_approve",
			"ACTION",
			fmt.Sprintf("Payroll run %s is calculated and awaiting approval.", period),
			"Review payroll totals and payslips, then approve the run for salary payment and TSD generation.",
			fmt.Sprintf("oa payroll runs approve --id %s", run.ID),
		))
	case PayrollApproved:
		actions = append(actions, action(
			"payroll_generate_tsd",
			"ACTION",
			fmt.Sprintf("Payroll run %s is approved and ready for TSD generation.", period),
			"Generate the TSD declaration, export it, and file it through e-MTA.",
			fmt.Sprintf("oa tsd generate --run-id %s", run.ID),
		))
	case PayrollPaid:
		actions = append(actions, action(
			"payroll_paid_tsd_followup",
			"ACTION",
			fmt.Sprintf("Payroll run %s is paid and still needs declaration follow-up.", period),
			"Reconcile salary payments, generate the TSD declaration, and retain payment evidence.",
			fmt.Sprintf("oa tsd generate --run-id %s", run.ID),
		))
	case PayrollDeclared:
		actions = append(actions, action(
			"payroll_declared_archive",
			"INFO",
			fmt.Sprintf("Payroll run %s is declared.", period),
			"Archive the accepted TSD export, payslips, and salary payment evidence with the monthly close support.",
			fmt.Sprintf("oa tsd export-xml --year %d --month %d --output ./tsd-%s.xml", run.PeriodYear, run.PeriodMonth, period),
		))
	default:
		actions = append(actions, action(
			"payroll_status_review",
			"WARNING",
			fmt.Sprintf("Payroll run %s has unrecognized status %q.", period, run.Status),
			"Review the payroll run status before continuing payroll, payment, or TSD workflows.",
			fmt.Sprintf("oa payroll runs get --id %s", run.ID),
		))
	}

	return actions
}
