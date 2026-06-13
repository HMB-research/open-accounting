package tax

import (
	"fmt"
	"strings"

	"github.com/HMB-research/open-accounting/internal/workspace"
)

// BuildKMDRemediationActions turns a KMD declaration status into accountant follow-up actions.
func BuildKMDRemediationActions(declaration *KMDDeclaration) []KMDRemediationAction {
	if declaration == nil {
		return nil
	}

	period := declaration.Period()
	uiPath := fmt.Sprintf("/tax/kmd?year=%d&month=%d", declaration.Year, declaration.Month)
	periodFlags := fmt.Sprintf("--year %d --month %d", declaration.Year, declaration.Month)
	base := KMDRemediationAction{
		Scope:      "tax",
		OwnerRole:  "accountant",
		Period:     period,
		EntityType: "kmd_declaration",
		EntityID:   declaration.ID,
		UIPath:     uiPath,
	}

	action := func(code, severity, message, text, command string) KMDRemediationAction {
		item := base
		item.Code = code
		item.Severity = severity
		meta := workspace.RemediationAssignment(
			"kmd_declarations",
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

	status := strings.ToUpper(strings.TrimSpace(declaration.Status))
	switch status {
	case "ACCEPTED":
		return []KMDRemediationAction{
			action(
				"kmd_accepted_archive",
				"INFO",
				fmt.Sprintf("KMD %s is accepted.", period),
				"Archive the accepted declaration XML and supporting VAT evidence with the period close pack.",
				fmt.Sprintf("oa tax kmd export-xml %s --output ./kmd-%s.xml", periodFlags, period),
			),
		}
	case "SUBMITTED":
		actions := []KMDRemediationAction{
			action(
				"kmd_awaiting_authority_acceptance",
				"ACTION",
				fmt.Sprintf("KMD %s has been submitted and is awaiting authority acceptance.", period),
				"Monitor e-MTA acceptance and retain the accepted confirmation with supporting VAT evidence.",
				fmt.Sprintf("oa tax kmd export-xml %s --output ./kmd-%s.xml", periodFlags, period),
			),
		}
		if declaration.SubmittedAt == nil {
			actions = append(actions, action(
				"kmd_submission_date_missing",
				"WARNING",
				fmt.Sprintf("KMD %s is marked submitted without a submitted_at timestamp.", period),
				"Record the submission timestamp or re-import the historical KMD period with submitted_at populated.",
				"oa tax kmd import-history --file ./kmd-history.csv",
			))
		}
		return actions
	}

	actions := make([]KMDRemediationAction, 0, 2)
	if len(declaration.Rows) == 0 && declaration.TotalOutputVAT.IsZero() && declaration.TotalInputVAT.IsZero() {
		actions = append(actions, action(
			"kmd_no_vat_rows",
			"WARNING",
			fmt.Sprintf("KMD %s has no VAT rows or totals.", period),
			"Confirm the period has no VAT activity, or post missing VAT-bearing invoices and regenerate KMD before export.",
			fmt.Sprintf("oa tax kmd generate %s", periodFlags),
		))
	}

	payable := declaration.CalculatePayable()
	switch {
	case payable.IsPositive():
		actions = append(actions, action(
			"kmd_payable_review",
			"ACTION",
			fmt.Sprintf("KMD %s has VAT payable of %s.", period, payable.String()),
			"Review output/input VAT totals, generate KMD INF when needed, export XML, and submit the declaration in e-MTA.",
			fmt.Sprintf("oa tax kmd export-xml %s --output ./kmd-%s.xml", periodFlags, period),
		))
	case payable.IsNegative():
		actions = append(actions, action(
			"kmd_refund_review",
			"ACTION",
			fmt.Sprintf("KMD %s has VAT refundable of %s.", period, payable.Abs().String()),
			"Reconcile input VAT evidence, review purchase invoice support, export XML, and submit the refund declaration in e-MTA.",
			fmt.Sprintf("oa tax kmd inf %s --json", periodFlags),
		))
	default:
		actions = append(actions, action(
			"kmd_zero_payable_review",
			"ACTION",
			fmt.Sprintf("KMD %s has zero net VAT payable.", period),
			"Review zero-payable VAT totals, export XML, and submit the declaration when the period is ready.",
			fmt.Sprintf("oa tax kmd export-xml %s --output ./kmd-%s.xml", periodFlags, period),
		))
	}

	return actions
}
