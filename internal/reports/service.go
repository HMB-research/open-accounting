package reports

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

// Service provides financial report operations
type Service struct {
	repo Repository
}

// NewService creates a new reports service with a PostgreSQL repository
func NewService(db *pgxpool.Pool) *Service {
	return &Service{repo: NewPostgresRepository(db)}
}

// NewServiceWithRepository creates a new reports service with an injected repository
func NewServiceWithRepository(repo Repository) *Service {
	return &Service{repo: repo}
}

// GenerateCashFlowStatement generates a cash flow statement for the given period
func (s *Service) GenerateCashFlowStatement(ctx context.Context, tenantID, schemaName string, req *CashFlowRequest) (*CashFlowStatement, error) {
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

	// Classify and aggregate cash flows
	operating := s.classifyOperatingActivities(entries)
	investing := s.classifyInvestingActivities(entries)
	financing := s.classifyFinancingActivities(entries)

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

func (s *Service) classifyOperatingActivities(entries []JournalEntryWithLines) []CashFlowItem {
	var receipts, payments, wages, taxes, interestPaid decimal.Decimal

	for _, entry := range entries {
		cashMovement := cashMovementForEntry(entry)
		if cashMovement.IsZero() {
			continue
		}

		if hasCashFlowAccount(entry.Lines, isFixedAssetAccount, isLoanAccount, isShareCapitalAccount, isDividendAccount) {
			continue
		}

		if cashMovement.GreaterThan(decimal.Zero) {
			if hasRevenueOrReceivable(entry.Lines) {
				receipts = receipts.Add(cashMovement)
			}
			continue
		}

		outflow := cashMovement.Abs()
		switch {
		case hasCashFlowAccount(entry.Lines, isInterestAccount):
			interestPaid = interestPaid.Add(outflow)
		case hasCashFlowAccount(entry.Lines, isTaxAccount):
			taxes = taxes.Add(outflow)
		case hasCashFlowAccount(entry.Lines, isWageAccount):
			wages = wages.Add(outflow)
		default:
			if hasOperatingPayableOrExpense(entry.Lines) {
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

func (s *Service) classifyInvestingActivities(entries []JournalEntryWithLines) []CashFlowItem {
	// Simplified - look for fixed asset related cash movements
	var fixedAssets decimal.Decimal

	for _, entry := range entries {
		cashMovement := cashMovementForEntry(entry)
		var isFixedAsset bool

		for _, line := range entry.Lines {
			if isFixedAssetAccount(line.AccountCode) {
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

func (s *Service) classifyFinancingActivities(entries []JournalEntryWithLines) []CashFlowItem {
	// Simplified - look for loan and equity related cash movements
	var loans, shares, dividends decimal.Decimal

	for _, entry := range entries {
		cashMovement := cashMovementForEntry(entry)
		var isLoan, isShare, isDividend bool

		for _, line := range entry.Lines {
			if isLoanAccount(line.AccountCode) {
				isLoan = true
			}
			if isShareCapitalAccount(line.AccountCode) {
				isShare = true
			}
			if isDividendAccount(line.AccountCode) {
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
		if isCashAccount(line.AccountCode) {
			cashMovement = cashMovement.Add(line.Debit.Sub(line.Credit))
		}
	}
	return cashMovement
}

func hasCashFlowAccount(lines []JournalLine, classifiers ...func(string) bool) bool {
	for _, line := range lines {
		if isCashAccount(line.AccountCode) {
			continue
		}
		for _, classifier := range classifiers {
			if classifier(line.AccountCode) {
				return true
			}
		}
	}
	return false
}

func hasRevenueOrReceivable(lines []JournalLine) bool {
	for _, line := range lines {
		if isCashAccount(line.AccountCode) {
			continue
		}
		if line.AccountType == "REVENUE" || isReceivableAccount(line.AccountCode) {
			return true
		}
	}
	return false
}

func hasOperatingPayableOrExpense(lines []JournalLine) bool {
	for _, line := range lines {
		if isCashAccount(line.AccountCode) {
			continue
		}
		if line.AccountType == "EXPENSE" || isPayableAccount(line.AccountCode) || line.AccountType == "LIABILITY" {
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
