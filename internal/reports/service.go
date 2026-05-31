package reports

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

// Service provides financial report operations
type Service struct {
	repo Repository
}

// NewService creates a new reports service with an ORM-backed repository.
func NewService(db *pgxpool.Pool) *Service {
	return &Service{repo: NewGORMRepository(db)}
}

// NewServiceWithRepository creates a new reports service with an injected repository
func NewServiceWithRepository(repo Repository) *Service {
	return &Service{repo: repo}
}

// GenerateCashFlowStatement generates a cash flow statement for the given period
func (s *Service) GenerateCashFlowStatement(ctx context.Context, tenantID, schemaName string, req *CashFlowRequest) (*CashFlowStatement, error) {
	method, err := NormalizeCashFlowMethod(req.Method)
	if err != nil {
		return nil, err
	}

	startDate, err := time.Parse("2006-01-02", req.StartDate)
	if err != nil {
		return nil, fmt.Errorf("invalid start date: %w", err)
	}
	endDate, err := time.Parse("2006-01-02", req.EndDate)
	if err != nil {
		return nil, fmt.Errorf("invalid end date: %w", err)
	}
	if endDate.Before(startDate) {
		return nil, fmt.Errorf("end date must be on or after start date")
	}

	// Get journal entries for the period
	entries, err := s.repo.GetJournalEntriesForPeriod(ctx, schemaName, tenantID, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("get journal entries: %w", err)
	}

	// Get opening cash balance
	openingCash, err := s.repo.GetCashAccountBalance(ctx, schemaName, tenantID, startDate.AddDate(0, 0, -1))
	if err != nil {
		return nil, fmt.Errorf("get opening cash: %w", err)
	}
	persistentMapping, err := s.repo.GetCashFlowMappingOverrides(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("get cash flow mapping: %w", err)
	}
	mappingOverrides, err := newEffectiveCashFlowMappingOverrides(persistentMapping, req.MappingOverrides)
	if err != nil {
		return nil, err
	}

	// Classify and aggregate cash flows
	var operating []CashFlowItem
	if method == CashFlowMethodIndirect {
		operating = s.classifyOperatingActivitiesIndirect(entries, mappingOverrides)
	} else {
		operating = s.classifyOperatingActivities(entries, mappingOverrides)
	}
	investing := s.classifyInvestingActivities(entries, mappingOverrides)
	financing := s.classifyFinancingActivities(entries, mappingOverrides)

	totalOperating := sumCashFlowItems(operating)
	totalInvesting := sumCashFlowItems(investing)
	totalFinancing := sumCashFlowItems(financing)
	netChange := totalOperating.Add(totalInvesting).Add(totalFinancing)

	// Add subtotals
	operating = append(operating, CashFlowItem{
		Code:          CFOperTotal,
		Description:   "Net cash from operating activities",
		DescriptionET: "Rahavood äritegevusest kokku",
		Amount:        totalOperating,
		IsSubtotal:    true,
	})

	investing = append(investing, CashFlowItem{
		Code:          CFInvTotal,
		Description:   "Net cash from investing activities",
		DescriptionET: "Rahavood investeerimistegevusest kokku",
		Amount:        totalInvesting,
		IsSubtotal:    true,
	})

	financing = append(financing, CashFlowItem{
		Code:          CFFinTotal,
		Description:   "Net cash from financing activities",
		DescriptionET: "Rahavood finantseerimistegevusest kokku",
		Amount:        totalFinancing,
		IsSubtotal:    true,
	})

	return &CashFlowStatement{
		TenantID:            tenantID,
		StartDate:           req.StartDate,
		EndDate:             req.EndDate,
		Method:              method,
		MappingOverrides:    mappingOverrides.response(),
		OperatingActivities: operating,
		InvestingActivities: investing,
		FinancingActivities: financing,
		TotalOperating:      totalOperating,
		TotalInvesting:      totalInvesting,
		TotalFinancing:      totalFinancing,
		NetCashChange:       netChange,
		OpeningCash:         openingCash,
		ClosingCash:         openingCash.Add(netChange),
		GeneratedAt:         time.Now(),
	}, nil
}

const (
	CashFlowMethodDirect   = "direct"
	CashFlowMethodIndirect = "indirect"
)

func NormalizeCashFlowMethod(method string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(method)) {
	case "", CashFlowMethodDirect:
		return CashFlowMethodDirect, nil
	case CashFlowMethodIndirect:
		return CashFlowMethodIndirect, nil
	default:
		return "", fmt.Errorf("cash flow method must be direct or indirect")
	}
}

// GetCashFlowMapping returns tenant-level cash-flow account-code mappings.
func (s *Service) GetCashFlowMapping(ctx context.Context, tenantID string) (*CashFlowMappingOverrides, error) {
	mapping, err := s.repo.GetCashFlowMappingOverrides(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("get cash flow mapping: %w", err)
	}
	normalized, err := NormalizeCashFlowMappingOverrides(mapping)
	if err != nil {
		return nil, err
	}
	return &normalized, nil
}

// UpdateCashFlowMapping replaces tenant-level cash-flow account-code mappings.
func (s *Service) UpdateCashFlowMapping(ctx context.Context, tenantID string, mapping CashFlowMappingOverrides) (*CashFlowMappingOverrides, error) {
	normalized, err := NormalizeCashFlowMappingOverrides(mapping)
	if err != nil {
		return nil, err
	}
	updated, err := s.repo.UpdateCashFlowMappingOverrides(ctx, tenantID, normalized)
	if err != nil {
		return nil, fmt.Errorf("update cash flow mapping: %w", err)
	}
	normalizedUpdated, err := NormalizeCashFlowMappingOverrides(updated)
	if err != nil {
		return nil, err
	}
	return &normalizedUpdated, nil
}

// NormalizeCashFlowMappingOverrides trims, uppercases, sorts, deduplicates, and validates cash-flow mappings.
func NormalizeCashFlowMappingOverrides(overrides CashFlowMappingOverrides) (CashFlowMappingOverrides, error) {
	mapping := newCashFlowMappingOverrides(overrides)
	if err := mapping.validateNoConflicts(); err != nil {
		return CashFlowMappingOverrides{}, err
	}
	return mapping.value(), nil
}

type cashFlowMappingOverrides struct {
	operating map[string]struct{}
	investing map[string]struct{}
	financing map[string]struct{}
}

func newCashFlowMappingOverrides(overrides CashFlowMappingOverrides) cashFlowMappingOverrides {
	return cashFlowMappingOverrides{
		operating: accountCodeSet(overrides.OperatingAccountCodes),
		investing: accountCodeSet(overrides.InvestingAccountCodes),
		financing: accountCodeSet(overrides.FinancingAccountCodes),
	}
}

func accountCodeSet(codes []string) map[string]struct{} {
	result := make(map[string]struct{})
	for _, code := range codes {
		for _, part := range strings.Split(code, ",") {
			normalized := normalizeAccountCode(part)
			if normalized != "" {
				result[normalized] = struct{}{}
			}
		}
	}
	return result
}

func normalizeAccountCode(code string) string {
	return strings.ToUpper(strings.TrimSpace(code))
}

func newEffectiveCashFlowMappingOverrides(persistent, request CashFlowMappingOverrides) (cashFlowMappingOverrides, error) {
	normalizedPersistent, err := NormalizeCashFlowMappingOverrides(persistent)
	if err != nil {
		return cashFlowMappingOverrides{}, err
	}
	normalizedRequest, err := NormalizeCashFlowMappingOverrides(request)
	if err != nil {
		return cashFlowMappingOverrides{}, err
	}

	result := newCashFlowMappingOverrides(normalizedPersistent)
	requestOverrides := newCashFlowMappingOverrides(normalizedRequest)
	result.applyRequestOverrides(requestOverrides)
	return result, nil
}

func (o cashFlowMappingOverrides) value() CashFlowMappingOverrides {
	return CashFlowMappingOverrides{
		OperatingAccountCodes: sortedAccountCodes(o.operating),
		InvestingAccountCodes: sortedAccountCodes(o.investing),
		FinancingAccountCodes: sortedAccountCodes(o.financing),
	}
}

func (o cashFlowMappingOverrides) response() *CashFlowMappingOverrides {
	if len(o.operating) == 0 && len(o.investing) == 0 && len(o.financing) == 0 {
		return nil
	}
	value := o.value()
	return &value
}

func sortedAccountCodes(codeSet map[string]struct{}) []string {
	if len(codeSet) == 0 {
		return nil
	}
	codes := make([]string, 0, len(codeSet))
	for code := range codeSet {
		codes = append(codes, code)
	}
	sort.Strings(codes)
	return codes
}

func (o cashFlowMappingOverrides) hasAccount(codeSet map[string]struct{}, line JournalLine) bool {
	_, ok := codeSet[normalizeAccountCode(line.AccountCode)]
	return ok
}

func (o cashFlowMappingOverrides) validateNoConflicts() error {
	seen := make(map[string]string, len(o.operating)+len(o.investing)+len(o.financing))
	for section, codeSet := range map[string]map[string]struct{}{
		"operating": o.operating,
		"investing": o.investing,
		"financing": o.financing,
	} {
		for code := range codeSet {
			if previous, ok := seen[code]; ok {
				return fmt.Errorf("cash flow mapping account code %s cannot be assigned to both %s and %s", code, previous, section)
			}
			seen[code] = section
		}
	}
	return nil
}

func (o *cashFlowMappingOverrides) applyRequestOverrides(request cashFlowMappingOverrides) {
	for code := range request.operating {
		o.moveAccount(code, o.operating)
	}
	for code := range request.investing {
		o.moveAccount(code, o.investing)
	}
	for code := range request.financing {
		o.moveAccount(code, o.financing)
	}
}

func (o *cashFlowMappingOverrides) moveAccount(code string, target map[string]struct{}) {
	delete(o.operating, code)
	delete(o.investing, code)
	delete(o.financing, code)
	target[code] = struct{}{}
}

func (o cashFlowMappingOverrides) isOperatingOverride(line JournalLine) bool {
	return o.hasAccount(o.operating, line)
}

func (o cashFlowMappingOverrides) isInvestingOverride(line JournalLine) bool {
	return o.hasAccount(o.investing, line)
}

func (o cashFlowMappingOverrides) isFinancingOverride(line JournalLine) bool {
	return o.hasAccount(o.financing, line)
}

func (o cashFlowMappingOverrides) isInvestingLine(line JournalLine) bool {
	return o.isInvestingOverride(line) || isFixedAssetLine(line)
}

func (o cashFlowMappingOverrides) isFinancingLine(line JournalLine) bool {
	return o.isFinancingOverride(line) || isLoanLine(line) || isShareCapitalLine(line) || isDividendLine(line)
}

func (s *Service) classifyOperatingActivities(entries []JournalEntryWithLines, overrides cashFlowMappingOverrides) []CashFlowItem {
	var receipts, payments, wages, taxes, interestPaid decimal.Decimal

	for _, entry := range entries {
		cashMovement := cashMovementForEntry(entry)
		if cashMovement.IsZero() {
			continue
		}

		if hasCashFlowAccount(entry.Lines, overrides.isInvestingLine, overrides.isFinancingLine) {
			continue
		}

		if cashMovement.GreaterThan(decimal.Zero) {
			if hasRevenueOrReceivable(entry.Lines, overrides) {
				receipts = receipts.Add(cashMovement)
			}
			continue
		}

		outflow := cashMovement.Abs()
		switch {
		case hasCashFlowAccount(entry.Lines, isInterestLine):
			interestPaid = interestPaid.Add(outflow)
		case hasCashFlowAccount(entry.Lines, isTaxLine):
			taxes = taxes.Add(outflow)
		case hasCashFlowAccount(entry.Lines, isWageLine):
			wages = wages.Add(outflow)
		default:
			if hasOperatingPayableOrExpense(entry.Lines, overrides) {
				payments = payments.Add(outflow)
			}
		}
	}

	items := []CashFlowItem{}
	if !receipts.IsZero() {
		items = append(items, CashFlowItem{
			Code:          CFOperReceipts,
			Description:   "Cash received from customers",
			DescriptionET: "Kaupade või teenuste müügist laekunud raha",
			Amount:        receipts,
		})
	}
	if !payments.IsZero() {
		items = append(items, CashFlowItem{
			Code:          CFOperPayments,
			Description:   "Cash paid to suppliers",
			DescriptionET: "Kaupade, materjalide ja teenuste eest makstud raha",
			Amount:        payments.Neg(),
		})
	}
	if !wages.IsZero() {
		items = append(items, CashFlowItem{
			Code:          CFOperWages,
			Description:   "Wages and payroll taxes paid",
			DescriptionET: "Töötajatele ja nende eest makstud raha",
			Amount:        wages.Neg(),
		})
	}
	if !taxes.IsZero() {
		items = append(items, CashFlowItem{
			Code:          CFOperTaxes,
			Description:   "Taxes paid",
			DescriptionET: "Maksude tasumine",
			Amount:        taxes.Neg(),
		})
	}
	if !interestPaid.IsZero() {
		items = append(items, CashFlowItem{
			Code:          CFOperInterestPd,
			Description:   "Interest paid",
			DescriptionET: "Makstud intressid",
			Amount:        interestPaid.Neg(),
		})
	}

	return items
}

func (s *Service) classifyOperatingActivitiesIndirect(entries []JournalEntryWithLines, overrides cashFlowMappingOverrides) []CashFlowItem {
	var netIncome, depreciation, receivablesDelta, inventoryDelta, payablesDelta decimal.Decimal

	for _, entry := range entries {
		for _, line := range entry.Lines {
			if isCashLine(line) {
				continue
			}
			if overrides.isInvestingOverride(line) || overrides.isFinancingOverride(line) {
				continue
			}

			switch {
			case overrides.isOperatingOverride(line):
				netIncome = netIncome.Add(line.Credit.Sub(line.Debit))
			case line.AccountType == "REVENUE":
				netIncome = netIncome.Add(line.Credit.Sub(line.Debit))
			case line.AccountType == "EXPENSE":
				netIncome = netIncome.Add(line.Credit.Sub(line.Debit))
				if isDepreciationLine(line) {
					depreciation = depreciation.Add(line.Debit.Sub(line.Credit))
				}
			case isReceivableLine(line):
				receivablesDelta = receivablesDelta.Add(line.Debit.Sub(line.Credit))
			case isInventoryLine(line):
				inventoryDelta = inventoryDelta.Add(line.Debit.Sub(line.Credit))
			case isPayableLine(line):
				payablesDelta = payablesDelta.Add(line.Credit.Sub(line.Debit))
			}
		}
	}

	items := []CashFlowItem{{
		Code:          CFOperNetIncome,
		Description:   "Net income",
		DescriptionET: "Aruandeperioodi kasum või kahjum",
		Amount:        netIncome,
	}}
	if !depreciation.IsZero() {
		items = append(items, CashFlowItem{
			Code:          CFOperDepreciation,
			Description:   "Depreciation and amortization",
			DescriptionET: "Kulum ja amortisatsioon",
			Amount:        depreciation,
		})
	}
	if !receivablesDelta.IsZero() {
		items = append(items, CashFlowItem{
			Code:          CFOperReceivables,
			Description:   "Change in receivables",
			DescriptionET: "Nõuete muutus",
			Amount:        receivablesDelta.Neg(),
		})
	}
	if !inventoryDelta.IsZero() {
		items = append(items, CashFlowItem{
			Code:          CFOperInventory,
			Description:   "Change in inventory",
			DescriptionET: "Varude muutus",
			Amount:        inventoryDelta.Neg(),
		})
	}
	if !payablesDelta.IsZero() {
		items = append(items, CashFlowItem{
			Code:          CFOperPayables,
			Description:   "Change in payables",
			DescriptionET: "Võlgnevuste muutus",
			Amount:        payablesDelta,
		})
	}

	return items
}

func (s *Service) classifyInvestingActivities(entries []JournalEntryWithLines, overrides cashFlowMappingOverrides) []CashFlowItem {
	var fixedAssets decimal.Decimal

	for _, entry := range entries {
		cashMovement := cashMovementForEntry(entry)
		var isFixedAsset bool

		for _, line := range entry.Lines {
			if overrides.isInvestingLine(line) {
				isFixedAsset = true
			}
		}

		if isFixedAsset && !cashMovement.IsZero() {
			fixedAssets = fixedAssets.Add(cashMovement)
		}
	}

	items := []CashFlowItem{}
	if !fixedAssets.IsZero() {
		items = append(items, CashFlowItem{
			Code:          CFInvFixedAssets,
			Description:   "Purchase/sale of fixed assets",
			DescriptionET: "Materiaalse ja immateriaalse põhivara ost ja müük",
			Amount:        fixedAssets,
		})
	}

	return items
}

func (s *Service) classifyFinancingActivities(entries []JournalEntryWithLines, overrides cashFlowMappingOverrides) []CashFlowItem {
	var loans, shares, dividends decimal.Decimal

	for _, entry := range entries {
		cashMovement := cashMovementForEntry(entry)
		var isLoan, isShare, isDividend bool

		for _, line := range entry.Lines {
			if overrides.isFinancingOverride(line) || isLoanLine(line) {
				isLoan = true
			}
			if isShareCapitalLine(line) {
				isShare = true
			}
			if isDividendLine(line) {
				isDividend = true
			}
		}

		if isLoan && !cashMovement.IsZero() {
			loans = loans.Add(cashMovement)
		}
		if isShare && cashMovement.GreaterThan(decimal.Zero) {
			shares = shares.Add(cashMovement)
		}
		if isDividend && cashMovement.LessThan(decimal.Zero) {
			dividends = dividends.Add(cashMovement)
		}
	}

	items := []CashFlowItem{}
	if !loans.IsZero() {
		if loans.GreaterThan(decimal.Zero) {
			items = append(items, CashFlowItem{
				Code:          CFFinLoansRcvd,
				Description:   "Proceeds from loans",
				DescriptionET: "Laenude saamine",
				Amount:        loans,
			})
		} else {
			items = append(items, CashFlowItem{
				Code:          CFFinLoansRepaid,
				Description:   "Repayment of loans",
				DescriptionET: "Saadud laenude tagasimaksmine",
				Amount:        loans,
			})
		}
	}
	if !shares.IsZero() {
		items = append(items, CashFlowItem{
			Code:          CFFinShares,
			Description:   "Share capital contributions",
			DescriptionET: "Aktsiate või osade emiteerimine",
			Amount:        shares,
		})
	}
	if !dividends.IsZero() {
		items = append(items, CashFlowItem{
			Code:          CFFinDividendsPd,
			Description:   "Dividends paid",
			DescriptionET: "Dividendide maksmine",
			Amount:        dividends,
		})
	}

	return items
}

func sumCashFlowItems(items []CashFlowItem) decimal.Decimal {
	sum := decimal.Zero
	for _, item := range items {
		if !item.IsSubtotal {
			sum = sum.Add(item.Amount)
		}
	}
	return sum
}

func cashMovementForEntry(entry JournalEntryWithLines) decimal.Decimal {
	cashMovement := decimal.Zero
	for _, line := range entry.Lines {
		if isCashLine(line) {
			cashMovement = cashMovement.Add(line.Debit.Sub(line.Credit))
		}
	}
	return cashMovement
}

func hasCashFlowAccount(lines []JournalLine, classifiers ...func(JournalLine) bool) bool {
	for _, line := range lines {
		if isCashLine(line) {
			continue
		}
		for _, classifier := range classifiers {
			if classifier(line) {
				return true
			}
		}
	}
	return false
}

func hasRevenueOrReceivable(lines []JournalLine, overrides cashFlowMappingOverrides) bool {
	for _, line := range lines {
		if isCashLine(line) {
			continue
		}
		if overrides.isOperatingOverride(line) || line.AccountType == "REVENUE" || isReceivableLine(line) {
			return true
		}
	}
	return false
}

func hasOperatingPayableOrExpense(lines []JournalLine, overrides cashFlowMappingOverrides) bool {
	for _, line := range lines {
		if isCashLine(line) {
			continue
		}
		if overrides.isOperatingOverride(line) {
			return true
		}
		if line.AccountType == "EXPENSE" || isPayableLine(line) || line.AccountType == "LIABILITY" {
			return true
		}
		if isInventoryLine(line) {
			return true
		}
	}
	return false
}

func isCashLine(line JournalLine) bool {
	return isCashAccount(line.AccountCode) ||
		(line.AccountType == "ASSET" && accountNameContains(line, "cash", "bank", "checking", "kassa", "pank", "arveldus"))
}

func isFixedAssetLine(line JournalLine) bool {
	return isFixedAssetAccount(line.AccountCode) ||
		(line.AccountType == "ASSET" && accountNameContains(line, "fixed asset", "equipment", "tangible asset", "intangible asset", "põhivara", "materiaalse", "immateriaalse"))
}

func isLoanLine(line JournalLine) bool {
	return isLoanAccount(line.AccountCode) ||
		(line.AccountType == "LIABILITY" && accountNameContains(line, "loan", "borrowing", "laen"))
}

func isShareCapitalLine(line JournalLine) bool {
	return isShareCapitalAccount(line.AccountCode) ||
		(line.AccountType == "EQUITY" && accountNameContains(line, "share capital", "osakapital", "aktsiakapital"))
}

func isDividendLine(line JournalLine) bool {
	return isDividendAccount(line.AccountCode) ||
		((line.AccountType == "EQUITY" || line.AccountType == "LIABILITY") && accountNameContains(line, "dividend", "dividendid"))
}

func isReceivableLine(line JournalLine) bool {
	return isReceivableAccount(line.AccountCode) ||
		(line.AccountType == "ASSET" && accountNameContains(line, "receivable", "customer balance", "nõuded", "ostja"))
}

func isPayableLine(line JournalLine) bool {
	return isPayableAccount(line.AccountCode) ||
		(line.AccountType == "LIABILITY" && accountNameContains(line, "payable", "supplier", "vendor", "võlad", "hankija"))
}

func isWageLine(line JournalLine) bool {
	return isWageAccount(line.AccountCode) ||
		accountNameContains(line, "wage", "salary", "payroll", "palk", "töötasu")
}

func isTaxLine(line JournalLine) bool {
	return isTaxAccount(line.AccountCode) ||
		accountNameContains(line, "tax", "vat", "käibemaks", "maks")
}

func isInterestLine(line JournalLine) bool {
	return isInterestAccount(line.AccountCode) ||
		accountNameContains(line, "interest", "intress")
}

func isDepreciationLine(line JournalLine) bool {
	return isDepreciationAccount(line.AccountCode) ||
		accountNameContains(line, "depreciation", "amortization", "kulum", "amortisatsioon")
}

func isInventoryLine(line JournalLine) bool {
	return isInventoryAccount(line.AccountCode) ||
		(line.AccountType == "ASSET" && accountNameContains(line, "inventory", "stock", "varud", "kaubad"))
}

func accountNameContains(line JournalLine, terms ...string) bool {
	name := strings.ToLower(line.AccountName)
	for _, term := range terms {
		if strings.Contains(name, term) {
			return true
		}
	}
	return false
}

func isCashAccount(code string) bool {
	// Default chart: 1000 is the asset header and 1100 is cash/bank.
	return len(code) >= 4 && (code[:2] == "10" || code[:2] == "11")
}

func isFixedAssetAccount(code string) bool {
	// Estonian chart of accounts: 1500-1599 are fixed assets
	return len(code) >= 4 && code[:2] == "15"
}

func isLoanAccount(code string) bool {
	return len(code) >= 4 && (code[:2] == "24" || code[:2] == "25")
}

func isShareCapitalAccount(code string) bool {
	return len(code) >= 4 && code[:2] == "31"
}

func isDividendAccount(code string) bool {
	return len(code) >= 4 && (code[:2] == "32" || code[:2] == "33")
}

func isReceivableAccount(code string) bool {
	return len(code) >= 4 && code[:2] == "12"
}

func isPayableAccount(code string) bool {
	return len(code) >= 4 && code[:2] == "21"
}

func isWageAccount(code string) bool {
	return len(code) >= 4 && (code[:2] == "23" || code[:2] == "52")
}

func isTaxAccount(code string) bool {
	return len(code) >= 4 && (code[:2] == "22" || code[:2] == "58")
}

func isInterestAccount(code string) bool {
	return len(code) >= 4 && code[:2] == "57"
}

func isDepreciationAccount(code string) bool {
	return len(code) >= 4 && code[:2] == "56"
}

func isInventoryAccount(code string) bool {
	return len(code) >= 4 && code[:2] == "13"
}

// GetBalanceConfirmationSummary generates a summary of all balances for receivables or payables
func (s *Service) GetBalanceConfirmationSummary(ctx context.Context, tenantID, schemaName string, req *BalanceConfirmationRequest) (*BalanceConfirmationSummary, error) {
	asOfDate, err := time.Parse("2006-01-02", req.AsOfDate)
	if err != nil {
		return nil, fmt.Errorf("invalid as_of_date: %w", err)
	}

	invoiceType := "SALES"
	balanceType := BalanceTypeReceivable
	if req.Type == "PAYABLE" {
		invoiceType = "PURCHASE"
		balanceType = BalanceTypePayable
	}

	contacts, err := s.repo.GetOutstandingInvoicesByContact(ctx, schemaName, tenantID, invoiceType, asOfDate)
	if err != nil {
		return nil, fmt.Errorf("get contact balances: %w", err)
	}

	var totalBalance decimal.Decimal
	var totalInvoices int
	for _, c := range contacts {
		totalBalance = totalBalance.Add(c.Balance)
		totalInvoices += c.InvoiceCount
	}

	return &BalanceConfirmationSummary{
		Type:         balanceType,
		AsOfDate:     req.AsOfDate,
		TotalBalance: totalBalance,
		ContactCount: len(contacts),
		InvoiceCount: totalInvoices,
		Contacts:     contacts,
		GeneratedAt:  time.Now(),
	}, nil
}

// GetBalanceConfirmation generates a balance confirmation for a specific contact
func (s *Service) GetBalanceConfirmation(ctx context.Context, tenantID, schemaName string, req *BalanceConfirmationRequest) (*BalanceConfirmation, error) {
	if req.ContactID == "" {
		return nil, fmt.Errorf("contact_id is required for individual balance confirmation")
	}

	asOfDate, err := time.Parse("2006-01-02", req.AsOfDate)
	if err != nil {
		return nil, fmt.Errorf("invalid as_of_date: %w", err)
	}

	contact, err := s.repo.GetContact(ctx, schemaName, tenantID, req.ContactID)
	if err != nil {
		return nil, fmt.Errorf("get contact: %w", err)
	}

	invoiceType := "SALES"
	balanceType := BalanceTypeReceivable
	if req.Type == "PAYABLE" {
		invoiceType = "PURCHASE"
		balanceType = BalanceTypePayable
	}

	invoices, err := s.repo.GetContactInvoices(ctx, schemaName, tenantID, req.ContactID, invoiceType, asOfDate)
	if err != nil {
		return nil, fmt.Errorf("get contact invoices: %w", err)
	}

	var totalBalance decimal.Decimal
	for _, inv := range invoices {
		totalBalance = totalBalance.Add(inv.OutstandingAmount)
	}

	return &BalanceConfirmation{
		ID:           fmt.Sprintf("%s-%s-%s", req.ContactID, req.Type, req.AsOfDate),
		TenantID:     tenantID,
		ContactID:    contact.ID,
		ContactName:  contact.Name,
		ContactCode:  contact.Code,
		ContactEmail: contact.Email,
		Type:         balanceType,
		AsOfDate:     req.AsOfDate,
		TotalBalance: totalBalance,
		Invoices:     invoices,
		GeneratedAt:  time.Now(),
	}, nil
}

// GetContactStatement generates a contact activity statement for a period.
func (s *Service) GetContactStatement(ctx context.Context, tenantID, schemaName string, req *ContactStatementRequest) (*ContactStatement, error) {
	if strings.TrimSpace(req.ContactID) == "" {
		return nil, fmt.Errorf("contact_id is required")
	}

	startDate, err := time.Parse("2006-01-02", req.StartDate)
	if err != nil {
		return nil, fmt.Errorf("invalid start_date: %w", err)
	}
	endDate, err := time.Parse("2006-01-02", req.EndDate)
	if err != nil {
		return nil, fmt.Errorf("invalid end_date: %w", err)
	}
	if endDate.Before(startDate) {
		return nil, fmt.Errorf("end_date must be on or after start_date")
	}

	balanceType, invoiceType, paymentType, err := contactStatementKinds(req.Type)
	if err != nil {
		return nil, err
	}

	contact, err := s.repo.GetContact(ctx, schemaName, tenantID, strings.TrimSpace(req.ContactID))
	if err != nil {
		return nil, fmt.Errorf("get contact: %w", err)
	}

	openingBalance, err := s.repo.GetContactStatementOpeningBalance(ctx, schemaName, tenantID, contact.ID, invoiceType, paymentType, startDate)
	if err != nil {
		return nil, fmt.Errorf("get contact statement opening balance: %w", err)
	}

	entries, err := s.repo.GetContactStatementEntries(ctx, schemaName, tenantID, contact.ID, invoiceType, paymentType, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("get contact statement entries: %w", err)
	}

	runningBalance := openingBalance
	var totalInvoiced decimal.Decimal
	var totalPaid decimal.Decimal
	for i := range entries {
		amount := entries[i].StatementAmount
		if amount.GreaterThanOrEqual(decimal.Zero) {
			entries[i].IncreaseAmount = amount
			totalInvoiced = totalInvoiced.Add(amount)
		} else {
			decrease := amount.Abs()
			entries[i].DecreaseAmount = decrease
			totalPaid = totalPaid.Add(decrease)
		}
		runningBalance = runningBalance.Add(amount)
		entries[i].Balance = runningBalance
	}

	return &ContactStatement{
		ID:             fmt.Sprintf("%s-%s-%s-%s", contact.ID, balanceType, req.StartDate, req.EndDate),
		TenantID:       tenantID,
		ContactID:      contact.ID,
		ContactName:    contact.Name,
		ContactCode:    contact.Code,
		ContactEmail:   contact.Email,
		Type:           balanceType,
		StartDate:      req.StartDate,
		EndDate:        req.EndDate,
		OpeningBalance: openingBalance,
		ClosingBalance: runningBalance,
		TotalInvoiced:  totalInvoiced,
		TotalPaid:      totalPaid,
		Entries:        entries,
		GeneratedAt:    time.Now(),
	}, nil
}

func contactStatementKinds(rawType string) (BalanceConfirmationType, string, string, error) {
	switch strings.ToUpper(strings.TrimSpace(rawType)) {
	case string(BalanceTypeReceivable):
		return BalanceTypeReceivable, "SALES", "RECEIVED", nil
	case string(BalanceTypePayable):
		return BalanceTypePayable, "PURCHASE", "MADE", nil
	default:
		return "", "", "", fmt.Errorf("type must be RECEIVABLE or PAYABLE")
	}
}

// GetSalesMarginReport generates sales margin reporting for a period.
func (s *Service) GetSalesMarginReport(ctx context.Context, tenantID, schemaName string, req *SalesMarginRequest) (*SalesMarginReport, error) {
	startDate, err := time.Parse("2006-01-02", req.StartDate)
	if err != nil {
		return nil, fmt.Errorf("invalid start_date: %w", err)
	}
	endDate, err := time.Parse("2006-01-02", req.EndDate)
	if err != nil {
		return nil, fmt.Errorf("invalid end_date: %w", err)
	}
	if endDate.Before(startDate) {
		return nil, fmt.Errorf("end_date must be on or after start_date")
	}

	lines, err := s.repo.GetSalesMarginLines(ctx, schemaName, tenantID, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("get sales margin lines: %w", err)
	}

	var totalRevenue decimal.Decimal
	var totalCost decimal.Decimal
	for i := range lines {
		lines[i].Margin = lines[i].Revenue.Sub(lines[i].Cost)
		lines[i].MarginPercent = calculateMarginPercent(lines[i].Margin, lines[i].Revenue)
		totalRevenue = totalRevenue.Add(lines[i].Revenue)
		totalCost = totalCost.Add(lines[i].Cost)
	}
	totalMargin := totalRevenue.Sub(totalCost)
	byContact := aggregateSalesMarginByContact(lines)

	return &SalesMarginReport{
		TenantID:      tenantID,
		StartDate:     req.StartDate,
		EndDate:       req.EndDate,
		TotalRevenue:  totalRevenue,
		TotalCost:     totalCost,
		TotalMargin:   totalMargin,
		MarginPercent: calculateMarginPercent(totalMargin, totalRevenue),
		LineCount:     len(lines),
		ByContact:     byContact,
		Lines:         lines,
		GeneratedAt:   time.Now(),
	}, nil
}

func aggregateSalesMarginByContact(lines []SalesMarginLine) []SalesMarginContact {
	contactsByID := make(map[string]*SalesMarginContact)
	for _, line := range lines {
		key := line.ContactID
		if key == "" {
			key = line.ContactName
		}
		contact, ok := contactsByID[key]
		if !ok {
			contact = &SalesMarginContact{
				ContactID:   line.ContactID,
				ContactName: line.ContactName,
			}
			contactsByID[key] = contact
		}
		contact.Revenue = contact.Revenue.Add(line.Revenue)
		contact.Cost = contact.Cost.Add(line.Cost)
		contact.LineCount++
	}

	contacts := make([]SalesMarginContact, 0, len(contactsByID))
	for _, contact := range contactsByID {
		contact.Margin = contact.Revenue.Sub(contact.Cost)
		contact.MarginPercent = calculateMarginPercent(contact.Margin, contact.Revenue)
		contacts = append(contacts, *contact)
	}
	sort.Slice(contacts, func(i, j int) bool {
		if contacts[i].Margin.Equal(contacts[j].Margin) {
			return contacts[i].ContactName < contacts[j].ContactName
		}
		return contacts[i].Margin.GreaterThan(contacts[j].Margin)
	})
	return contacts
}

func calculateMarginPercent(margin, revenue decimal.Decimal) decimal.Decimal {
	if revenue.IsZero() {
		return decimal.Zero
	}
	return margin.Div(revenue).Mul(decimal.NewFromInt(100)).Round(2)
}
