package importsession

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

var currencyCodePattern = regexp.MustCompile(`^[A-Z]{3}$`)

type canonicalJournalGroup struct {
	JournalGroupID string                 `json:"journal_group_id"`
	PeriodStart    string                 `json:"period_start"`
	PeriodEnd      string                 `json:"period_end"`
	Currency       string                 `json:"currency"`
	Lines          []canonicalJournalLine `json:"lines"`
}

type canonicalJournalLine struct {
	AccountExternalID string `json:"account_external_id"`
	Debit             string `json:"debit"`
	Credit            string `json:"credit"`
}

type validationIssueAppender func(code string, recordIndex int, field, message string)

type ledgerAuthorityContext struct {
	authoritative bool
	asOf          time.Time
	asOfValid     bool
}

var duplicateFinancialPostingEntities = map[string]struct{}{
	"payment":          {},
	"purchase_invoice": {},
	"sales_invoice":    {},
}

// validateLedgerContract verifies only the ledger-ready package envelope and
// canonical journal payloads. It is deliberately independent of accounting
// write services, account lookups, and financial persistence.
func validateLedgerContract(pkg CanonicalPackage, appendIssue validationIssueAppender) *LedgerVerification {
	verification := &LedgerVerification{}
	authority := validateLedgerAuthority(pkg.LedgerAuthority, verification, appendIssue)
	partialScope, scopeStart, scopeEnd := validateImportScope(pkg.Scope, verification, appendIssue)
	seenJournalGroups := make(map[string]struct{})
	for index, record := range pkg.Records {
		entityType := strings.TrimSpace(strings.ToLower(record.EntityType))
		if _, prohibited := duplicateFinancialPostingEntities[entityType]; prohibited && authority.authoritative {
			appendIssue("duplicate_financial_posting_plan", index+1, "entity_type", "SmartAccounts-authoritative ledger packages cannot stage invoice or payment financial posting plans")
		}
		if entityType != ScopeResourceJournalEntry {
			if partialScope {
				appendIssue("scope_resource_outside_subset", index+1, "entity_type", "partial scope supports journal_entry records only")
			}
			continue
		}
		validateJournalRecord(record, index+1, partialScope, scopeStart, scopeEnd, authority, seenJournalGroups, verification, appendIssue)
	}
	if verification.VarianceCount > 0 || verification.Stale {
		verification.ReviewRequired = true
		verification.VerificationStatus = LedgerVerificationStatusReviewRequired
	} else if authority.authoritative && authority.asOfValid {
		verification.VerificationStatus = LedgerVerificationStatusVerified
	}
	return verification
}

func validateLedgerAuthority(authority *LedgerAuthorityDeclaration, verification *LedgerVerification, appendIssue validationIssueAppender) ledgerAuthorityContext {
	if authority == nil {
		appendIssue("required", 0, "ledger_authority", "ledger_authority is required")
		return ledgerAuthorityContext{}
	}
	ledgerAuthority := strings.TrimSpace(strings.ToLower(authority.GeneralLedgerAuthority))
	verification.GeneralLedgerAuthority = ledgerAuthority
	verification.SourceAsOfDate = strings.TrimSpace(authority.SourceAsOfDate)
	verification.VarianceCount = authority.VarianceCount
	verification.Stale = authority.Stale
	if ledgerAuthority == "" {
		appendIssue("required", 0, "ledger_authority.general_ledger_authority", "general_ledger_authority is required")
	} else if ledgerAuthority != ProviderSmartAccounts {
		appendIssue("unsupported_ledger_authority", 0, "ledger_authority.general_ledger_authority", "general_ledger_authority must be smartaccounts")
	}
	if authority.SmartAccountsGLAuthoritative == nil {
		appendIssue("required", 0, "ledger_authority.smartaccounts_gl_authoritative", "smartaccounts_gl_authoritative must be explicitly true")
	} else if !*authority.SmartAccountsGLAuthoritative {
		appendIssue("ledger_authority_not_confirmed", 0, "ledger_authority.smartaccounts_gl_authoritative", "smartaccounts_gl_authoritative must be true")
	}
	if authority.VarianceCount < 0 {
		appendIssue("invalid_variance_count", 0, "ledger_authority.variance_count", "variance_count must be zero or greater")
	}
	asOf, asOfValid := parseScopeDate(verification.SourceAsOfDate)
	if !asOfValid {
		appendIssue("invalid_source_as_of_date", 0, "ledger_authority.source_as_of_date", "source_as_of_date must be YYYY-MM-DD")
	}
	authoritative := ledgerAuthority == ProviderSmartAccounts && authority.SmartAccountsGLAuthoritative != nil && *authority.SmartAccountsGLAuthoritative
	verification.JournalStagingAllowed = authoritative
	verification.FinancialPostingPlanAllowed = false
	return ledgerAuthorityContext{authoritative: authoritative, asOf: asOf, asOfValid: asOfValid}
}

func validateImportScope(scope *ImportScope, verification *LedgerVerification, appendIssue validationIssueAppender) (bool, time.Time, time.Time) {
	if scope == nil {
		appendIssue("required", 0, "scope", "scope is required")
		return false, time.Time{}, time.Time{}
	}
	mode := strings.TrimSpace(strings.ToLower(scope.Mode))
	verification.ScopeMode = mode
	resources := normalizedResourceTypes(scope.ResourceTypes)
	verification.ResourceTypes = resources
	verification.PeriodStart = strings.TrimSpace(scope.PeriodStart)
	verification.PeriodEnd = strings.TrimSpace(scope.PeriodEnd)

	switch mode {
	case ScopeModeFull:
		if !sameStrings(resources, []string{ScopeResourceAll}) {
			appendIssue("unsupported_resource_subset", 0, "scope.resource_types", "full scope must declare resource_types as [all]")
		}
		if verification.PeriodStart != "" || verification.PeriodEnd != "" {
			appendIssue("scope_period_not_allowed", 0, "scope", "full scope cannot declare period_start or period_end")
		}
		return false, time.Time{}, time.Time{}
	case ScopeModePartial:
		if !sameStrings(resources, []string{ScopeResourceJournalEntry}) {
			appendIssue("unsupported_resource_subset", 0, "scope.resource_types", "partial scope supports resource_types [journal_entry] only")
		}
		start, startOK := parseScopeDate(verification.PeriodStart)
		if !startOK {
			appendIssue("invalid_scope_period", 0, "scope.period_start", "partial scope period_start must be YYYY-MM-DD")
		}
		end, endOK := parseScopeDate(verification.PeriodEnd)
		if !endOK {
			appendIssue("invalid_scope_period", 0, "scope.period_end", "partial scope period_end must be YYYY-MM-DD")
		}
		if startOK && endOK && start.After(end) {
			appendIssue("invalid_scope_period", 0, "scope", "partial scope period_start must not be after period_end")
		}
		return true, start, end
	default:
		appendIssue("unsupported_scope_mode", 0, "scope.mode", "scope.mode must be full or partial")
		return false, time.Time{}, time.Time{}
	}
}

func validateJournalRecord(
	record CanonicalRecord,
	recordIndex int,
	partialScope bool,
	scopeStart time.Time,
	scopeEnd time.Time,
	authority ledgerAuthorityContext,
	seenJournalGroups map[string]struct{},
	verification *LedgerVerification,
	appendIssue validationIssueAppender,
) {
	operation := strings.TrimSpace(strings.ToLower(record.Operation))
	if operation != "upsert" {
		appendIssue("unsupported_ledger_operation", recordIndex, "operation", "journal_entry records must use upsert for ledger verification")
		return
	}
	var group canonicalJournalGroup
	if err := json.Unmarshal(record.Payload, &group); err != nil {
		appendIssue("invalid_journal_group", recordIndex, "payload", "journal_entry payload must be a canonical journal group")
		return
	}
	verification.JournalGroupCount++
	valid := true
	groupID := strings.TrimSpace(group.JournalGroupID)
	if groupID == "" {
		appendIssue("required", recordIndex, "payload.journal_group_id", "journal_group_id is required")
		valid = false
	} else if groupID != strings.TrimSpace(record.ExternalID) {
		appendIssue("journal_group_external_id_mismatch", recordIndex, "payload.journal_group_id", "journal_group_id must match the journal_entry external_id")
		valid = false
	} else if _, exists := seenJournalGroups[groupID]; exists {
		appendIssue("duplicate_journal_group", recordIndex, "payload.journal_group_id", "journal_group_id must be unique within a package")
		valid = false
	} else {
		seenJournalGroups[groupID] = struct{}{}
	}

	periodStart, startOK := parseScopeDate(strings.TrimSpace(group.PeriodStart))
	if !startOK {
		appendIssue("invalid_journal_period", recordIndex, "payload.period_start", "journal group period_start must be YYYY-MM-DD")
		valid = false
	}
	periodEnd, endOK := parseScopeDate(strings.TrimSpace(group.PeriodEnd))
	if !endOK {
		appendIssue("invalid_journal_period", recordIndex, "payload.period_end", "journal group period_end must be YYYY-MM-DD")
		valid = false
	}
	if startOK && endOK && periodStart.After(periodEnd) {
		appendIssue("invalid_journal_period", recordIndex, "payload", "journal group period_start must not be after period_end")
		valid = false
	}
	if partialScope && startOK && endOK && (!periodStart.IsZero() && !periodEnd.IsZero()) && (periodStart.Before(scopeStart) || periodEnd.After(scopeEnd)) {
		appendIssue("journal_group_outside_scope", recordIndex, "payload", "journal group must be wholly inside the partial scope period")
		valid = false
	}
	if authority.asOfValid && endOK && periodEnd.After(authority.asOf) {
		appendIssue("journal_group_after_source_as_of", recordIndex, "payload.period_end", "journal group period_end must not be after source_as_of_date")
		valid = false
	}

	currency := strings.TrimSpace(group.Currency)
	if !currencyCodePattern.MatchString(currency) {
		appendIssue("invalid_journal_currency", recordIndex, "payload.currency", "journal group currency must be a three-letter uppercase code")
		valid = false
	}
	if len(group.Lines) < 2 {
		appendIssue("invalid_journal_group", recordIndex, "payload.lines", "journal group must contain at least two lines")
		valid = false
	}

	debitTotal, creditTotal := decimal.Zero, decimal.Zero
	for _, line := range group.Lines {
		lineValid := true
		if strings.TrimSpace(line.AccountExternalID) == "" {
			appendIssue("required", recordIndex, "payload.lines.account_external_id", "journal line account_external_id is required")
			lineValid = false
		}
		debit, debitOK := parseLedgerAmount(line.Debit)
		if !debitOK {
			appendIssue("invalid_journal_amount", recordIndex, "payload.lines.debit", "journal line debit must be a non-negative decimal string")
			lineValid = false
		}
		credit, creditOK := parseLedgerAmount(line.Credit)
		if !creditOK {
			appendIssue("invalid_journal_amount", recordIndex, "payload.lines.credit", "journal line credit must be a non-negative decimal string")
			lineValid = false
		}
		if debitOK && creditOK && (debit.IsPositive() == credit.IsPositive()) {
			appendIssue("invalid_journal_line", recordIndex, "payload.lines", "journal line must have exactly one positive debit or credit amount")
			lineValid = false
		}
		if !lineValid {
			valid = false
			continue
		}
		debitTotal = debitTotal.Add(debit)
		creditTotal = creditTotal.Add(credit)
	}
	if !debitTotal.Equal(creditTotal) {
		appendIssue("unbalanced_journal_group", recordIndex, "payload.lines", "journal group debit and credit totals must balance")
		valid = false
	}
	if valid {
		verification.BalancedJournalGroupCount++
	}
}

// stageLedgerJournals reduces already-validated journal-entry payloads to the
// exact data needed by a deterministic dry-run. It deliberately excludes all
// non-ledger source payload fields and sorts every collection before storage.
func stageLedgerJournals(pkg CanonicalPackage) ([]StagedLedgerJournal, error) {
	staged := make([]StagedLedgerJournal, 0)
	for _, record := range pkg.Records {
		if strings.TrimSpace(strings.ToLower(record.EntityType)) != ScopeResourceJournalEntry {
			continue
		}
		var group canonicalJournalGroup
		if err := json.Unmarshal(record.Payload, &group); err != nil {
			return nil, fmt.Errorf("decode staged journal %q: %w", record.ExternalID, err)
		}
		lines := make([]StagedLedgerLine, 0, len(group.Lines))
		for _, line := range group.Lines {
			debit, debitOK := parseLedgerAmount(line.Debit)
			credit, creditOK := parseLedgerAmount(line.Credit)
			if !debitOK || !creditOK {
				return nil, fmt.Errorf("decode staged journal amount for %q", record.ExternalID)
			}
			lines = append(lines, StagedLedgerLine{
				AccountExternalID: strings.TrimSpace(line.AccountExternalID),
				Debit:             debit.String(),
				Credit:            credit.String(),
			})
		}
		sort.Slice(lines, func(i, j int) bool {
			left := lines[i].AccountExternalID + "\x00" + lines[i].Debit + "\x00" + lines[i].Credit
			right := lines[j].AccountExternalID + "\x00" + lines[j].Debit + "\x00" + lines[j].Credit
			return left < right
		})
		staged = append(staged, StagedLedgerJournal{
			SourceJournalExternalID: strings.TrimSpace(record.ExternalID),
			SourceRevision:          strings.TrimSpace(record.Revision),
			JournalGroupID:          strings.TrimSpace(group.JournalGroupID),
			PeriodStart:             strings.TrimSpace(group.PeriodStart),
			PeriodEnd:               strings.TrimSpace(group.PeriodEnd),
			Currency:                strings.TrimSpace(group.Currency),
			Lines:                   lines,
		})
	}
	sort.Slice(staged, func(i, j int) bool {
		left := staged[i].PeriodStart + "\x00" + staged[i].PeriodEnd + "\x00" + staged[i].Currency + "\x00" + staged[i].JournalGroupID + "\x00" + staged[i].SourceRevision
		right := staged[j].PeriodStart + "\x00" + staged[j].PeriodEnd + "\x00" + staged[j].Currency + "\x00" + staged[j].JournalGroupID + "\x00" + staged[j].SourceRevision
		return left < right
	})
	return staged, nil
}

func parseScopeDate(value string) (time.Time, bool) {
	parsed, err := time.Parse(time.DateOnly, value)
	return parsed, err == nil
}

func parseLedgerAmount(value string) (decimal.Decimal, bool) {
	amount, err := decimal.NewFromString(strings.TrimSpace(value))
	return amount, err == nil && !amount.IsNegative()
}

func normalizedResourceTypes(values []string) []string {
	resources := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(strings.ToLower(value))
		if trimmed != "" {
			resources = append(resources, trimmed)
		}
	}
	sort.Strings(resources)
	return resources
}

func sameStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
