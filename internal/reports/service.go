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
	var receipts, payments decimal.Decimal

	for _, entry := range entries {
		// Look for cash account movements
		var cashMovement decimal.Decimal
		var counterpartyType string

		for _, line := range entry.Lines {
			if isCashAccount(line.AccountCode) {
				cashMovement = line.Debit.Sub(line.Credit)
			} else {
				counterpartyType = line.AccountType
			}
		}

		if cashMovement.IsZero() {
			continue
		}

		// Skip if counterparty is fixed asset (investing) or loan (financing)
		for _, line := range entry.Lines {
			if isFixedAssetAccount(line.AccountCode) || isLoanAccount(line.AccountCode) || isDividendAccount(line.AccountCode) {
				// This is investing or financing, not operating
				cashMovement = decimal.Zero
				break
			}
		}

		if cashMovement.IsZero() {
			continue
		}

		// Classify based on counterparty account
		switch counterpartyType {
		case "REVENUE", "ASSET": // Receivables
			if cashMovement.GreaterThan(decimal.Zero) {
				receipts = receipts.Add(cashMovement)
			}
		case "EXPENSE":
			if cashMovement.LessThan(decimal.Zero) {
				payments = payments.Add(cashMovement.Abs())
			}
		case "LIABILITY":
			if cashMovement.LessThan(decimal.Zero) {
				payments = payments.Add(cashMovement.Abs())
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

	return items
}

func (s *Service) classifyInvestingActivities(entries []JournalEntryWithLines) []CashFlowItem {
	// Simplified - look for fixed asset related cash movements
	var fixedAssets decimal.Decimal

	for _, entry := range entries {
		var cashMovement decimal.Decimal
		var isFixedAsset bool

		for _, line := range entry.Lines {
			if isCashAccount(line.AccountCode) {
				cashMovement = line.Debit.Sub(line.Credit)
			}
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
	var loans, dividends decimal.Decimal

	for _, entry := range entries {
		var cashMovement decimal.Decimal
		var isLoan, isDividend bool

		for _, line := range entry.Lines {
			if isCashAccount(line.AccountCode) {
				cashMovement = line.Debit.Sub(line.Credit)
			}
			if isLoanAccount(line.AccountCode) {
				isLoan = true
			}
			if isDividendAccount(line.AccountCode) {
				isDividend = true
			}
		}

		if isLoan && !cashMovement.IsZero() {
			loans = loans.Add(cashMovement)
		}
		if isDividend && !cashMovement.IsZero() {
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

func isCashAccount(code string) bool {
	// Estonian chart of accounts: 1000-1099 are typically cash accounts
	return len(code) >= 4 && code[:2] == "10"
}

func isFixedAssetAccount(code string) bool {
	// Estonian chart of accounts: 1500-1599 are fixed assets
	return len(code) >= 4 && code[:2] == "15"
}

func isLoanAccount(code string) bool {
	// Estonian chart of accounts: 2000-2099 are short-term loans, 2500+ long-term
	return len(code) >= 4 && (code[:2] == "20" || code[:2] == "25")
}

func isDividendAccount(code string) bool {
	// Estonian chart of accounts: 3xxx are equity, dividends declared would be here
	return len(code) >= 4 && code[:1] == "3"
}
