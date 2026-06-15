package expenses

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/HMB-research/open-accounting/internal/workspace"
)

// BuildExpenseRemediationActions turns expense claim state into accountant follow-up actions.
func BuildExpenseRemediationActions(expense *Expense) []ExpenseRemediationAction {
	if expense == nil {
		return nil
	}

	status := ExpenseStatus(strings.ToUpper(strings.TrimSpace(string(expense.Status))))
	base := ExpenseRemediationAction{
		Scope:         "expenses",
		OwnerRole:     "accountant",
		EntityType:    "expense",
		EntityID:      strings.TrimSpace(expense.ID),
		ExpenseNumber: strings.TrimSpace(expense.ExpenseNumber),
		Status:        string(status),
		UIPath:        expenseRemediationUIPath(expense),
	}

	action := func(code, severity, message, text, command string) ExpenseRemediationAction {
		item := base
		item.Code = code
		item.Severity = severity
		meta := workspace.RemediationAssignment(
			"expense_claims",
			code,
			severity,
			item.EntityType,
			item.EntityID,
			item.ExpenseNumber,
			item.Status,
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

	commandID := expenseRemediationID(expense)
	label := expenseRemediationLabel(expense)
	getCommand := fmt.Sprintf("oa expenses get --id %s", commandID)
	actions := make([]ExpenseRemediationAction, 0, 3)

	switch status {
	case StatusDraft:
		if expense.RequiresReceipt {
			actions = append(actions, action(
				"expense_receipt_required",
				"ACTION",
				fmt.Sprintf("Expense %s requires an approved receipt before approval or posting.", label),
				"Upload the receipt and route it through document review before approving the expense.",
				fmt.Sprintf("oa documents upload --entity-type expense --entity-id %s --document-type receipt --file <file>", commandID),
			))
		}
		actions = append(actions, action(
			"expense_submit_for_approval",
			"ACTION",
			fmt.Sprintf("Expense %s is still in draft.", label),
			"Review the merchant, accounts, amount, and evidence requirements, then submit it for approval.",
			fmt.Sprintf("oa expenses submit --id %s", commandID),
		))
	case StatusSubmitted:
		if expense.RequiresReceipt {
			actions = append(actions, action(
				"expense_receipt_approval_required",
				"ACTION",
				fmt.Sprintf("Expense %s is submitted and receipt-backed.", label),
				"Confirm a linked receipt exists and is approved before approving the expense.",
				"oa documents review-queue --entity-type expense --document-type receipt --status PENDING",
			))
		}
		actions = append(actions, action(
			"expense_approve_or_reject",
			"ACTION",
			fmt.Sprintf("Expense %s is awaiting approval.", label),
			"Approve the expense when policy evidence is complete, or reject it with a reason for correction.",
			fmt.Sprintf("oa expenses approve --id %s", commandID),
		))
	case StatusApproved:
		actions = append(actions, action(
			"expense_post_to_ledger",
			"ACTION",
			fmt.Sprintf("Expense %s is approved but not posted.", label),
			"Post the approved expense to create the balanced ledger entry before closing the period.",
			fmt.Sprintf("oa expenses post --id %s", commandID),
		))
	case StatusRejected:
		actions = append(actions, action(
			"expense_rejection_review",
			"ACTION",
			fmt.Sprintf("Expense %s is rejected.", label),
			"Correct the rejection reason, attach missing support when needed, then resubmit the expense.",
			fmt.Sprintf("oa expenses submit --id %s", commandID),
		))
	case StatusPosted:
		actions = append(actions, action(
			"expense_posted_archive",
			"INFO",
			fmt.Sprintf("Expense %s is posted.", label),
			"Confirm the journal entry and any required receipt evidence are retained with the period records.",
			getCommand,
		))
	default:
		actions = append(actions, action(
			"expense_status_review",
			"WARNING",
			fmt.Sprintf("Expense %s has unsupported status %q.", label, expense.Status),
			"Review the expense status before approval, posting, reporting, or period close.",
			getCommand,
		))
	}

	return actions
}

func expenseRemediationUIPath(expense *Expense) string {
	values := url.Values{}
	if strings.TrimSpace(expense.ID) != "" {
		values.Set("expense_id", strings.TrimSpace(expense.ID))
	}
	if encoded := values.Encode(); encoded != "" {
		return "/expenses?" + encoded
	}
	return "/expenses"
}

func expenseRemediationID(expense *Expense) string {
	if trimmed := strings.TrimSpace(expense.ID); trimmed != "" {
		return trimmed
	}
	return "<expense-id>"
}

func expenseRemediationLabel(expense *Expense) string {
	if trimmed := strings.TrimSpace(expense.ExpenseNumber); trimmed != "" {
		return trimmed
	}
	return expenseRemediationID(expense)
}
