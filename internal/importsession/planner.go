package importsession

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// Plan creates a deterministic, receipt-derived dry run. It only reads an
// import receipt, sibling receipt metadata, and mapped target accounts. It
// intentionally has no accounting repository write surface.
func (s *Service) Plan(ctx context.Context, schemaName, tenantID, sessionID string, req ImportPlanRequest) (*ImportPlanResult, error) {
	if s == nil || s.store == nil {
		return nil, errors.New("import session storage is not configured")
	}
	receipt, err := s.Get(ctx, schemaName, tenantID, sessionID)
	if err != nil {
		return nil, err
	}
	result := &ImportPlanResult{
		FinancialWritesPlanned: false,
		ImportSessionID:        receipt.ID,
		PackageSHA256:          receipt.PackageSHA256,
		Scope:                  receiptScope(receipt.LedgerVerification),
	}
	appendIssue := func(code, field, message string) {
		result.Issues = append(result.Issues, ImportPlanIssue{Code: code, Field: field, Message: message})
	}

	if receipt.Status == SessionStatusReceivedReviewRequired || receipt.LedgerVerification.ReviewRequired || receipt.LedgerVerification.VerificationStatus == LedgerVerificationStatusReviewRequired {
		appendIssue("review_required", "ledger_verification", "source variance or staleness requires accountant review before planning")
		return result, ErrImportPlanReviewRequired
	}
	if receipt.Status != SessionStatusReceivedValidated || !receipt.LedgerVerification.JournalStagingAllowed || receipt.LedgerVerification.VerificationStatus != LedgerVerificationStatusVerified {
		appendIssue("ledger_authority_not_verified", "ledger_verification", "only a verified SmartAccounts-authoritative ledger receipt can be planned")
		return result, ErrImportPlanReviewRequired
	}
	if receipt.LedgerVerification.JournalGroupCount > 0 && len(receipt.LedgerPlanInput) == 0 {
		appendIssue("staged_ledger_metadata_unavailable", "import_session", "this receipt predates ledger dry-run metadata and must be restaged")
		return result, ErrLedgerPlanInputUnavailable
	}

	partialScope, scopeStart, scopeEnd := validateReceiptScope(result.Scope, appendIssue)
	staged := cloneStagedJournals(receipt.LedgerPlanInput)
	sortStagedJournals(staged)
	requiredAccounts := make(map[string]struct{})
	seenSourceRevisions := make(map[string]string)
	seenJournalGroups := make(map[string]struct{})
	for _, journal := range staged {
		validateStagedJournal(journal, partialScope, scopeStart, scopeEnd, appendIssue)
		if priorRevision, exists := seenSourceRevisions[journal.SourceJournalExternalID]; exists {
			if priorRevision == journal.SourceRevision {
				appendIssue("duplicate_source_revision", "staged_ledger_journals", "a source journal revision appears more than once in this receipt")
			} else {
				appendIssue("source_revision_conflict", "staged_ledger_journals", "one source journal has multiple revisions in this receipt")
			}
		} else {
			seenSourceRevisions[journal.SourceJournalExternalID] = journal.SourceRevision
		}
		if _, exists := seenJournalGroups[journal.JournalGroupID]; exists {
			appendIssue("duplicate_journal_group", "staged_ledger_journals", "a journal group appears more than once in this receipt")
		} else {
			seenJournalGroups[journal.JournalGroupID] = struct{}{}
		}
		for _, line := range journal.Lines {
			requiredAccounts[line.AccountExternalID] = struct{}{}
		}
	}

	mappings := validateAccountMappings(req.AccountMappings, requiredAccounts, appendIssue)
	if err := s.blockConflictingSourceRevisions(ctx, schemaName, tenantID, receipt, staged, appendIssue); err != nil {
		return nil, err
	}
	if s.accountResolver == nil && len(requiredAccounts) > 0 {
		return nil, errors.New("import plan account resolver is not configured")
	}
	for _, targetAccountID := range sortedMappingTargets(mappings) {
		if err := s.accountResolver.ResolveAccount(ctx, schemaName, tenantID, targetAccountID); err != nil {
			appendIssue("account_mapping_target_unavailable", "account_mappings", "a mapped target account is unavailable in the selected tenant")
		}
	}
	if len(result.Issues) > 0 {
		return result, nil
	}

	result.JournalActions, result.JournalReconciliations, result.AccountReconciliations = buildPlannedActions(staged, mappings)
	result.PlanSHA256 = importPlanDigest(*result)
	result.Ready = true
	return result, nil
}

func (s *Service) blockConflictingSourceRevisions(ctx context.Context, schemaName, tenantID string, receipt *Receipt, staged []StagedLedgerJournal, appendIssue func(code, field, message string)) error {
	if len(staged) == 0 {
		return nil
	}
	otherJournals, err := s.store.ListLedgerPlanInputs(ctx, schemaName, tenantID, receipt.Provider, receipt.SourceCompanyID, receipt.ID)
	if err != nil {
		return fmt.Errorf("list staged ledger metadata: %w", err)
	}
	otherRevisions := make(map[string]map[string]struct{})
	for _, journal := range otherJournals {
		externalID := strings.TrimSpace(journal.SourceJournalExternalID)
		revision := strings.TrimSpace(journal.SourceRevision)
		if externalID == "" || revision == "" {
			continue
		}
		if otherRevisions[externalID] == nil {
			otherRevisions[externalID] = make(map[string]struct{})
		}
		otherRevisions[externalID][revision] = struct{}{}
	}
	for _, journal := range staged {
		revisions := otherRevisions[journal.SourceJournalExternalID]
		if len(revisions) == 0 {
			continue
		}
		if _, exists := revisions[journal.SourceRevision]; exists {
			appendIssue("duplicate_source_revision", "staged_ledger_journals", "this source journal revision was already staged in another receipt")
			continue
		}
		appendIssue("source_revision_conflict", "staged_ledger_journals", "this source journal has a different revision in another receipt")
	}
	return nil
}

func receiptScope(verification LedgerVerification) ImportScope {
	return ImportScope{
		Mode:          verification.ScopeMode,
		ResourceTypes: append([]string(nil), verification.ResourceTypes...),
		PeriodStart:   verification.PeriodStart,
		PeriodEnd:     verification.PeriodEnd,
	}
}

func validateReceiptScope(scope ImportScope, appendIssue func(code, field, message string)) (bool, string, string) {
	mode := strings.TrimSpace(strings.ToLower(scope.Mode))
	resources := normalizedResourceTypes(scope.ResourceTypes)
	periodStart := strings.TrimSpace(scope.PeriodStart)
	periodEnd := strings.TrimSpace(scope.PeriodEnd)
	switch mode {
	case ScopeModeFull:
		if !sameStrings(resources, []string{ScopeResourceAll}) || periodStart != "" || periodEnd != "" {
			appendIssue("invalid_staged_scope", "scope", "full-scope receipts must retain resource_types [all] without period bounds")
		}
		return false, "", ""
	case ScopeModePartial:
		start, startOK := parseScopeDate(periodStart)
		end, endOK := parseScopeDate(periodEnd)
		if !sameStrings(resources, []string{ScopeResourceJournalEntry}) || !startOK || !endOK || start.After(end) {
			appendIssue("invalid_staged_scope", "scope", "partial-scope receipts must retain journal_entry-only inclusive period bounds")
		}
		return true, periodStart, periodEnd
	default:
		appendIssue("invalid_staged_scope", "scope", "staged receipt scope must be full or partial")
		return false, "", ""
	}
}

func validateStagedJournal(journal StagedLedgerJournal, partialScope bool, scopeStart, scopeEnd string, appendIssue func(code, field, message string)) {
	if strings.TrimSpace(journal.SourceJournalExternalID) == "" || strings.TrimSpace(journal.SourceRevision) == "" || strings.TrimSpace(journal.JournalGroupID) == "" {
		appendIssue("invalid_staged_journal", "staged_ledger_journals", "staged journal identity and source revision are required")
	}
	if journal.SourceJournalExternalID != journal.JournalGroupID {
		appendIssue("journal_group_external_id_mismatch", "staged_ledger_journals", "staged journal group must match its source external ID")
	}
	start, startOK := parseScopeDate(journal.PeriodStart)
	end, endOK := parseScopeDate(journal.PeriodEnd)
	if !startOK || !endOK || start.After(end) {
		appendIssue("invalid_staged_journal_period", "staged_ledger_journals", "staged journal period is invalid")
	}
	if partialScope && startOK && endOK && (journal.PeriodStart < scopeStart || journal.PeriodEnd > scopeEnd) {
		appendIssue("journal_group_outside_scope", "staged_ledger_journals", "staged journal falls outside the receipt partial scope")
	}
	if !currencyCodePattern.MatchString(strings.TrimSpace(journal.Currency)) {
		appendIssue("invalid_staged_journal_currency", "staged_ledger_journals", "staged journal currency is invalid")
	}
	if len(journal.Lines) < 2 {
		appendIssue("invalid_staged_journal", "staged_ledger_journals", "staged journal must contain at least two lines")
	}
	debitTotal, creditTotal := decimal.Zero, decimal.Zero
	for _, line := range journal.Lines {
		debit, debitOK := parseLedgerAmount(line.Debit)
		credit, creditOK := parseLedgerAmount(line.Credit)
		if strings.TrimSpace(line.AccountExternalID) == "" || !debitOK || !creditOK || (debitOK && creditOK && debit.IsPositive() == credit.IsPositive()) {
			appendIssue("invalid_staged_journal_line", "staged_ledger_journals", "staged journal lines require one positive debit or credit and a source account")
			continue
		}
		debitTotal = debitTotal.Add(debit)
		creditTotal = creditTotal.Add(credit)
	}
	if !debitTotal.Equal(creditTotal) {
		appendIssue("unbalanced_staged_journal", "staged_ledger_journals", "staged journal totals must balance")
	}
}

func validateAccountMappings(requested []AccountMapping, required map[string]struct{}, appendIssue func(code, field, message string)) map[string]string {
	mappings := make(map[string]string, len(requested))
	for _, mapping := range requested {
		sourceAccountID := strings.TrimSpace(mapping.SourceAccountExternalID)
		targetAccountID := strings.TrimSpace(mapping.TargetAccountID)
		if sourceAccountID == "" || targetAccountID == "" {
			appendIssue("invalid_account_mapping", "account_mappings", "source_account_external_id and target_account_id are required")
			continue
		}
		parsedTargetAccountID, err := uuid.Parse(targetAccountID)
		if err != nil {
			appendIssue("invalid_account_mapping", "account_mappings.target_account_id", "target_account_id must be a UUID")
			continue
		}
		if _, exists := mappings[sourceAccountID]; exists {
			appendIssue("duplicate_account_mapping", "account_mappings", "each source account may be mapped once")
			continue
		}
		mappings[sourceAccountID] = parsedTargetAccountID.String()
	}
	for sourceAccountID := range required {
		if _, exists := mappings[sourceAccountID]; !exists {
			appendIssue("account_mapping_missing", "account_mappings", "every staged source account requires a target account mapping")
		}
	}
	for sourceAccountID := range mappings {
		if _, exists := required[sourceAccountID]; !exists {
			appendIssue("unused_account_mapping", "account_mappings", "mappings must only target accounts used by the staged ledger receipt")
		}
	}
	return mappings
}

func sortedMappingTargets(mappings map[string]string) []string {
	unique := make(map[string]struct{}, len(mappings))
	for _, target := range mappings {
		unique[target] = struct{}{}
	}
	targets := make([]string, 0, len(unique))
	for target := range unique {
		targets = append(targets, target)
	}
	sort.Strings(targets)
	return targets
}

func buildPlannedActions(staged []StagedLedgerJournal, mappings map[string]string) ([]PlannedJournalAction, []JournalReconciliationExpectation, []AccountReconciliationExpectation) {
	actions := make([]PlannedJournalAction, 0, len(staged))
	journalExpectations := make([]JournalReconciliationExpectation, 0, len(staged))
	type accountExpectationKey struct{ source, target, currency string }
	accountTotals := make(map[accountExpectationKey]struct{ debit, credit decimal.Decimal })
	for _, journal := range staged {
		lines := make([]PlannedJournalLine, 0, len(journal.Lines))
		debitTotal, creditTotal := decimal.Zero, decimal.Zero
		for _, line := range journal.Lines {
			debit, _ := parseLedgerAmount(line.Debit)
			credit, _ := parseLedgerAmount(line.Credit)
			targetAccountID := mappings[line.AccountExternalID]
			lines = append(lines, PlannedJournalLine{
				SourceAccountExternalID: line.AccountExternalID,
				TargetAccountID:         targetAccountID,
				Debit:                   debit.String(),
				Credit:                  credit.String(),
			})
			debitTotal = debitTotal.Add(debit)
			creditTotal = creditTotal.Add(credit)
			key := accountExpectationKey{source: line.AccountExternalID, target: targetAccountID, currency: journal.Currency}
			totals := accountTotals[key]
			totals.debit = totals.debit.Add(debit)
			totals.credit = totals.credit.Add(credit)
			accountTotals[key] = totals
		}
		actions = append(actions, PlannedJournalAction{
			SourceJournalExternalID: journal.SourceJournalExternalID,
			SourceRevision:          journal.SourceRevision,
			JournalGroupID:          journal.JournalGroupID,
			PeriodStart:             journal.PeriodStart,
			PeriodEnd:               journal.PeriodEnd,
			Currency:                journal.Currency,
			Lines:                   lines,
			DebitTotal:              debitTotal.String(),
			CreditTotal:             creditTotal.String(),
		})
		journalExpectations = append(journalExpectations, JournalReconciliationExpectation{
			SourceJournalExternalID: journal.SourceJournalExternalID,
			SourceRevision:          journal.SourceRevision,
			JournalGroupID:          journal.JournalGroupID,
			Currency:                journal.Currency,
			DebitTotal:              debitTotal.String(),
			CreditTotal:             creditTotal.String(),
			Balanced:                debitTotal.Equal(creditTotal),
		})
	}
	accountExpectations := make([]AccountReconciliationExpectation, 0, len(accountTotals))
	for key, totals := range accountTotals {
		accountExpectations = append(accountExpectations, AccountReconciliationExpectation{
			SourceAccountExternalID: key.source,
			TargetAccountID:         key.target,
			Currency:                key.currency,
			DebitTotal:              totals.debit.String(),
			CreditTotal:             totals.credit.String(),
		})
	}
	sort.Slice(accountExpectations, func(i, j int) bool {
		left := accountExpectations[i].Currency + "\x00" + accountExpectations[i].SourceAccountExternalID + "\x00" + accountExpectations[i].TargetAccountID
		right := accountExpectations[j].Currency + "\x00" + accountExpectations[j].SourceAccountExternalID + "\x00" + accountExpectations[j].TargetAccountID
		return left < right
	})
	return actions, journalExpectations, accountExpectations
}

func cloneStagedJournals(source []StagedLedgerJournal) []StagedLedgerJournal {
	cloned := make([]StagedLedgerJournal, len(source))
	for index, journal := range source {
		cloned[index] = journal
		cloned[index].Lines = append([]StagedLedgerLine(nil), journal.Lines...)
	}
	return cloned
}

func sortStagedJournals(journals []StagedLedgerJournal) {
	for index := range journals {
		sort.Slice(journals[index].Lines, func(left, right int) bool {
			leftKey := journals[index].Lines[left].AccountExternalID + "\x00" + journals[index].Lines[left].Debit + "\x00" + journals[index].Lines[left].Credit
			rightKey := journals[index].Lines[right].AccountExternalID + "\x00" + journals[index].Lines[right].Debit + "\x00" + journals[index].Lines[right].Credit
			return leftKey < rightKey
		})
	}
	sort.Slice(journals, func(left, right int) bool {
		leftKey := journals[left].PeriodStart + "\x00" + journals[left].PeriodEnd + "\x00" + journals[left].Currency + "\x00" + journals[left].JournalGroupID + "\x00" + journals[left].SourceRevision
		rightKey := journals[right].PeriodStart + "\x00" + journals[right].PeriodEnd + "\x00" + journals[right].Currency + "\x00" + journals[right].JournalGroupID + "\x00" + journals[right].SourceRevision
		return leftKey < rightKey
	})
}

func importPlanDigest(result ImportPlanResult) string {
	type digestInput struct {
		ImportSessionID        string                             `json:"import_session_id"`
		PackageSHA256          string                             `json:"package_sha256"`
		Scope                  ImportScope                        `json:"scope"`
		JournalActions         []PlannedJournalAction             `json:"journal_actions"`
		AccountReconciliations []AccountReconciliationExpectation `json:"account_reconciliations"`
	}
	payload, _ := json.Marshal(digestInput{
		ImportSessionID:        result.ImportSessionID,
		PackageSHA256:          result.PackageSHA256,
		Scope:                  result.Scope,
		JournalActions:         result.JournalActions,
		AccountReconciliations: result.AccountReconciliations,
	})
	digest := sha256.Sum256(payload)
	return hex.EncodeToString(digest[:])
}
