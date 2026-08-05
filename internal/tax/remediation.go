package tax

import (
	"fmt"
	"strings"

	"github.com/HMB-research/open-accounting/internal/workspace"
)

const taxReportWorkspaceQueue = "tax_reports"

// BuildKMDRemediationActions turns a KMD declaration status into accountant follow-up actions.
func BuildKMDRemediationActions(declaration *KMDDeclaration) []KMDRemediationAction {
	if declaration == nil {
		return nil
	}

	period := declaration.Period()
	uiPath := "/vat-returns"
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
	case KMDStatusAccepted:
		return []KMDRemediationAction{
			action(
				"kmd_accepted_archive",
				"INFO",
				fmt.Sprintf("KMD %s is accepted.", period),
				"Archive the accepted declaration XML and supporting VAT evidence with the period close pack.",
				fmt.Sprintf("oa tax kmd export-xml %s --output ./kmd-%s.xml", periodFlags, period),
			),
		}
	case KMDStatusSubmitted:
		actions := []KMDRemediationAction{
			action(
				"kmd_awaiting_authority_acceptance",
				"ACTION",
				fmt.Sprintf("KMD %s has been submitted and is awaiting authority acceptance.", period),
				"Monitor e-MTA acceptance and retain the accepted confirmation with supporting VAT evidence.",
				fmt.Sprintf("oa tax kmd mark-accepted %s", periodFlags),
			),
		}
		if declaration.SubmittedAt == nil {
			actions = append(actions, action(
				"kmd_submission_date_missing",
				"WARNING",
				fmt.Sprintf("KMD %s is marked submitted without a submitted_at timestamp.", period),
				"Record the submission timestamp or re-import the historical KMD period with submitted_at populated.",
				fmt.Sprintf("oa tax kmd mark-submitted %s", periodFlags),
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

// BuildKMDINFRemediationActions turns a KMD INF report into accountant follow-up actions.
func BuildKMDINFRemediationActions(report *KMDINFReport) []TaxReportRemediationAction {
	if report == nil {
		return nil
	}

	period := fmt.Sprintf("%04d-%02d", report.Year, report.Month)
	threshold := report.Threshold
	if threshold.IsZero() {
		threshold = KMDINFDefaultThreshold
	}
	periodFlags := fmt.Sprintf("--year %d --month %d --threshold %s", report.Year, report.Month, threshold.String())
	base := TaxReportRemediationAction{
		Scope:      "tax",
		OwnerRole:  "accountant",
		Period:     period,
		EntityType: "kmd_inf_report",
		EntityID:   period,
		UIPath:     "/reports",
	}

	if len(report.Rows) == 0 {
		return []TaxReportRemediationAction{
			taxReportAction(
				base,
				"kmd_inf_no_threshold_rows",
				"WARNING",
				fmt.Sprintf("KMD INF %s has no partner rows at the %s threshold.", period, threshold.String()),
				"Confirm that no domestic partner exceeded the KMD INF threshold, then retain the empty report with the VAT period evidence.",
				fmt.Sprintf("oa tax kmd inf %s --json", periodFlags),
			),
		}
	}

	return []TaxReportRemediationAction{
		taxReportAction(
			base,
			"kmd_inf_review_required",
			"ACTION",
			fmt.Sprintf("KMD INF %s has %d threshold invoice row(s).", period, len(report.Rows)),
			"Review partner-period threshold totals, attach supporting VAT evidence, and archive the KMD INF report with the KMD declaration.",
			fmt.Sprintf("oa tax kmd inf %s --json", periodFlags),
		),
	}
}

// BuildEUVATOSSRemediationActions turns an EU VAT OSS report into accountant follow-up actions.
func BuildEUVATOSSRemediationActions(report *EUVATOSSReport) []TaxReportRemediationAction {
	if report == nil {
		return nil
	}

	period := fmt.Sprintf("%04d-Q%d", report.Year, report.Quarter)
	includeB2BFlag := ""
	if report.IncludeB2B {
		includeB2BFlag = " --include-b2b"
	}
	command := fmt.Sprintf("oa tax oss report --year %d --quarter %d%s --json", report.Year, report.Quarter, includeB2BFlag)
	base := TaxReportRemediationAction{
		Scope:      "tax",
		OwnerRole:  "accountant",
		Period:     period,
		EntityType: "eu_vat_oss_report",
		EntityID:   period,
		UIPath:     "/reports",
	}

	if len(report.Rows) == 0 {
		return []TaxReportRemediationAction{
			taxReportAction(
				base,
				"eu_vat_oss_no_rows",
				"WARNING",
				fmt.Sprintf("EU VAT OSS %s has no qualifying destination-country rows.", period),
				"Confirm the quarter has no qualifying non-Estonian EU sales or adjust filters before retaining the empty OSS report.",
				command,
			),
		}
	}

	return []TaxReportRemediationAction{
		taxReportAction(
			base,
			"eu_vat_oss_review_required",
			"ACTION",
			fmt.Sprintf("EU VAT OSS %s has VAT due of %s across %d country/rate row(s).", period, report.VATAmount.String(), len(report.Rows)),
			"Review destination-country VAT totals, file the quarterly OSS return manually, and retain the filing confirmation with the report evidence.",
			command,
		),
	}
}

func taxReportAction(base TaxReportRemediationAction, code, severity, message, text, command string) TaxReportRemediationAction {
	action := base
	action.Code = code
	action.Severity = severity
	meta := workspace.RemediationAssignment(
		taxReportWorkspaceQueue,
		code,
		severity,
		action.EntityType,
		action.EntityID,
		action.Period,
	)
	action.WorkspaceQueue = meta.WorkspaceQueue
	action.AssignmentKey = meta.AssignmentKey
	action.Priority = meta.Priority
	action.DueInDays = meta.DueInDays
	action.Message = message
	action.Action = text
	action.CLICommand = command
	return action
}
