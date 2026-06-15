package banking

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/HMB-research/open-accounting/internal/workspace"
)

// BuildBankRemediationActions turns bank transaction state into accountant follow-up actions.
func BuildBankRemediationActions(transaction *BankTransaction) []BankRemediationAction {
	if transaction == nil {
		return nil
	}

	followUp := normalizeBankRemediationFollowUp(transaction.FollowUpStatus)
	status := TransactionStatus(strings.ToUpper(strings.TrimSpace(string(transaction.Status))))
	base := BankRemediationAction{
		Scope:             "banking",
		OwnerRole:         "accountant",
		EntityType:        "bank_transaction",
		EntityID:          strings.TrimSpace(transaction.ID),
		BankAccountID:     strings.TrimSpace(transaction.BankAccountID),
		TransactionStatus: string(status),
		FollowUpStatus:    string(followUp),
		UIPath:            bankRemediationUIPath(transaction),
	}

	action := func(code, severity, message, text, command string) BankRemediationAction {
		item := base
		item.Code = code
		item.Severity = severity
		meta := workspace.RemediationAssignment(
			"banking_followup",
			code,
			severity,
			item.EntityType,
			item.EntityID,
			item.TransactionStatus,
			item.FollowUpStatus,
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

	commandSuffix := strings.TrimSpace(transaction.ID)
	if commandSuffix == "" {
		commandSuffix = "<transaction-id>"
	}
	getCommand := fmt.Sprintf("oa banking transactions get --id %s", commandSuffix)

	actions := make([]BankRemediationAction, 0, 3)
	switch followUp {
	case FollowUpEvidenceRequired:
		actions = append(actions, action(
			"bank_evidence_required",
			"ACTION",
			fmt.Sprintf("Bank transaction %s requires approved reconciliation evidence.", commandSuffix),
			"Upload and approve reconciliation evidence before completing the reconciliation.",
			fmt.Sprintf("oa documents upload --entity-type bank_transaction --entity-id %s --document-type reconciliation_evidence --file <file>", commandSuffix),
		))
	case FollowUpReadyToMatch:
		actions = append(actions, action(
			"bank_ready_to_match",
			"ACTION",
			fmt.Sprintf("Bank transaction %s is marked ready to match.", commandSuffix),
			"Review payment suggestions and match the transaction to the correct payment.",
			fmt.Sprintf("oa banking transactions suggestions --id %s", commandSuffix),
		))
	case FollowUpNone:
	default:
		actions = append(actions, action(
			"bank_follow_up_status_review",
			"WARNING",
			fmt.Sprintf("Bank transaction %s has unsupported follow-up status %q.", commandSuffix, transaction.FollowUpStatus),
			"Review the transaction follow-up status and update it to NONE, EVIDENCE_REQUIRED, or READY_TO_MATCH.",
			getCommand,
		))
	}

	switch status {
	case StatusUnmatched:
		if followUp != FollowUpReadyToMatch {
			actions = append(actions, action(
				"bank_transaction_unmatched",
				"ACTION",
				fmt.Sprintf("Bank transaction %s is still unmatched.", commandSuffix),
				"Match it to an existing payment, create a payment from the transaction, or mark the needed follow-up.",
				fmt.Sprintf("oa banking transactions suggestions --id %s", commandSuffix),
			))
		}
	case StatusMatched:
		if transaction.ReconciliationID == nil || strings.TrimSpace(*transaction.ReconciliationID) == "" {
			actions = append(actions, action(
				"bank_transaction_reconciliation_pending",
				"ACTION",
				fmt.Sprintf("Bank transaction %s is matched but not assigned to a reconciliation.", commandSuffix),
				"Include the transaction in the relevant bank reconciliation before period close.",
				fmt.Sprintf("oa banking reconciliations list --account-id %s", bankRemediationAccountID(transaction)),
			))
		}
	case StatusReconciled:
		actions = append(actions, action(
			"bank_transaction_reconciled_archive",
			"INFO",
			fmt.Sprintf("Bank transaction %s is reconciled.", commandSuffix),
			"Confirm the bank statement and any required reconciliation evidence are retained with the period records.",
			getCommand,
		))
	default:
		actions = append(actions, action(
			"bank_transaction_status_review",
			"WARNING",
			fmt.Sprintf("Bank transaction %s has unsupported status %q.", commandSuffix, transaction.Status),
			"Review the bank transaction state before matching, reconciliation, or close.",
			getCommand,
		))
	}

	return actions
}

func normalizeBankRemediationFollowUp(value FollowUpStatus) FollowUpStatus {
	if strings.TrimSpace(string(value)) == "" {
		return FollowUpNone
	}
	normalized, err := NormalizeFollowUpStatus(string(value))
	if err != nil {
		return FollowUpStatus(strings.ToUpper(strings.TrimSpace(string(value))))
	}
	return normalized
}

func bankRemediationUIPath(transaction *BankTransaction) string {
	values := url.Values{}
	if strings.TrimSpace(transaction.BankAccountID) != "" {
		values.Set("account_id", strings.TrimSpace(transaction.BankAccountID))
	}
	if strings.TrimSpace(transaction.ID) != "" {
		values.Set("transaction_id", strings.TrimSpace(transaction.ID))
	}
	if encoded := values.Encode(); encoded != "" {
		return "/banking?" + encoded
	}
	return "/banking"
}

func bankRemediationAccountID(transaction *BankTransaction) string {
	if trimmed := strings.TrimSpace(transaction.BankAccountID); trimmed != "" {
		return trimmed
	}
	return "<bank-account-id>"
}
