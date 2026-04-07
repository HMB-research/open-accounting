package accounting

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

const (
	SourceTypeYearEndCarryForward = "YEAR_END_CARRY_FORWARD"
	yearEndDateLayout             = "2006-01-02"
)

// AccountSummary is a lightweight account reference for workflow responses.
type AccountSummary struct {
	ID   string `json:"id"`
	Code string `json:"code"`
	Name string `json:"name"`
}

// JournalEntrySummary is a lightweight journal-entry reference for workflow responses.
type JournalEntrySummary struct {
	ID          string             `json:"id"`
	EntryNumber string             `json:"entry_number"`
	EntryDate   string             `json:"entry_date"`
	Description string             `json:"description"`
	Reference   string             `json:"reference,omitempty"`
	Status      JournalEntryStatus `json:"status"`
}

// YearEndCloseStatus summarizes readiness for fiscal-year carry-forward.
type YearEndCloseStatus struct {
	PeriodEndDate              string               `json:"period_end_date"`
	FiscalYearLabel            string               `json:"fiscal_year_label"`
	FiscalYearStartDate        string               `json:"fiscal_year_start_date"`
	FiscalYearEndDate          string               `json:"fiscal_year_end_date"`
	CarryForwardDate           string               `json:"carry_forward_date"`
	LockedThroughDate          *string              `json:"locked_through_date,omitempty"`
	IsFiscalYearEnd            bool                 `json:"is_fiscal_year_end"`
	PeriodClosed               bool                 `json:"period_closed"`
	HasProfitAndLossActivity   bool                 `json:"has_profit_and_loss_activity"`
	CarryForwardNeeded         bool                 `json:"carry_forward_needed"`
	CarryForwardReady          bool                 `json:"carry_forward_ready"`
	HasRetainedEarningsAccount bool                 `json:"has_retained_earnings_account"`
	RetainedEarningsAccount    *AccountSummary      `json:"retained_earnings_account,omitempty"`
	NetIncome                  decimal.Decimal      `json:"net_income"`
	ExistingCarryForward       *JournalEntrySummary `json:"existing_carry_forward,omitempty"`
}

// CreateYearEndCarryForwardRequest requests a year-end carry-forward journal.
type CreateYearEndCarryForwardRequest struct {
	PeriodEndDate string `json:"period_end_date"`
	UserID        string `json:"-"`
}

// YearEndCarryForwardResult contains the created journal entry and refreshed status.
type YearEndCarryForwardResult struct {
	JournalEntry *JournalEntry       `json:"journal_entry"`
	Status       *YearEndCloseStatus `json:"status"`
}

// GetYearEndCloseStatus returns the carry-forward readiness state for a fiscal year.
func (s *Service) GetYearEndCloseStatus(ctx context.Context, schemaName, tenantID string, fiscalYearStartMonth int, rawPeriodEndDate string, lockedThroughDate *string) (*YearEndCloseStatus, error) {
	periodEndDate, err := parseYearEndDate(rawPeriodEndDate)
	if err != nil {
		return nil, err
	}

	fiscalYearStartDate, fiscalYearEndDate := fiscalYearBounds(periodEndDate, fiscalYearStartMonth)
	periodClosed, normalizedLockDate, err := periodClosedThrough(lockedThroughDate, periodEndDate)
	if err != nil {
		return nil, err
	}

	periodBalances, err := s.repo.GetPeriodBalances(ctx, schemaName, tenantID, fiscalYearStartDate, fiscalYearEndDate)
	if err != nil {
		return nil, fmt.Errorf("load fiscal-year balances: %w", err)
	}

	incomeStatement, err := s.GetIncomeStatement(ctx, schemaName, tenantID, fiscalYearStartDate, fiscalYearEndDate)
	if err != nil {
		return nil, fmt.Errorf("load fiscal-year income statement: %w", err)
	}

	accounts, err := s.repo.ListAccounts(ctx, schemaName, tenantID, false)
	if err != nil {
		return nil, fmt.Errorf("list accounts: %w", err)
	}
	retainedEarningsAccount := findRetainedEarningsAccount(accounts)

	existingEntry, err := s.repo.GetJournalEntryBySource(ctx, schemaName, tenantID, SourceTypeYearEndCarryForward, yearEndCarryForwardSourceID(tenantID, fiscalYearEndDate))
	if err != nil {
		return nil, fmt.Errorf("check carry-forward journal: %w", err)
	}

	status := &YearEndCloseStatus{
		PeriodEndDate:            periodEndDate.Format(yearEndDateLayout),
		FiscalYearLabel:          fiscalYearLabel(fiscalYearStartDate, fiscalYearEndDate),
		FiscalYearStartDate:      fiscalYearStartDate.Format(yearEndDateLayout),
		FiscalYearEndDate:        fiscalYearEndDate.Format(yearEndDateLayout),
		CarryForwardDate:         fiscalYearEndDate.AddDate(0, 0, 1).Format(yearEndDateLayout),
		LockedThroughDate:        normalizedLockDate,
		IsFiscalYearEnd:          periodEndDate.Equal(fiscalYearEndDate),
		PeriodClosed:             periodClosed,
		HasProfitAndLossActivity: len(periodBalances) > 0,
		CarryForwardNeeded:       len(periodBalances) > 0 && existingEntry == nil,
		NetIncome:                incomeStatement.NetIncome,
	}

	if retainedEarningsAccount != nil {
		status.HasRetainedEarningsAccount = true
		status.RetainedEarningsAccount = &AccountSummary{
			ID:   retainedEarningsAccount.ID,
			Code: retainedEarningsAccount.Code,
			Name: retainedEarningsAccount.Name,
		}
	}

	if existingEntry != nil {
		status.ExistingCarryForward = &JournalEntrySummary{
			ID:          existingEntry.ID,
			EntryNumber: existingEntry.EntryNumber,
			EntryDate:   existingEntry.EntryDate.Format(yearEndDateLayout),
			Description: existingEntry.Description,
			Reference:   existingEntry.Reference,
			Status:      existingEntry.Status,
		}
	}

	needsRetainedEarningsAccount := carryForwardDiff(periodBalances).GreaterThan(decimal.Zero)
	status.CarryForwardReady = status.IsFiscalYearEnd &&
		status.PeriodClosed &&
		status.CarryForwardNeeded &&
		(!needsRetainedEarningsAccount || status.HasRetainedEarningsAccount)

	return status, nil
}

// CreateYearEndCarryForward creates and posts a year-end carry-forward journal entry.
func (s *Service) CreateYearEndCarryForward(ctx context.Context, schemaName, tenantID string, fiscalYearStartMonth int, lockedThroughDate *string, req *CreateYearEndCarryForwardRequest) (*YearEndCarryForwardResult, error) {
	if req == nil {
		return nil, fmt.Errorf("request is required")
	}
	if strings.TrimSpace(req.UserID) == "" {
		return nil, fmt.Errorf("user_id is required")
	}

	status, err := s.GetYearEndCloseStatus(ctx, schemaName, tenantID, fiscalYearStartMonth, req.PeriodEndDate, lockedThroughDate)
	if err != nil {
		return nil, err
	}

	switch {
	case !status.IsFiscalYearEnd:
		return nil, fmt.Errorf("period end date must match the fiscal year end")
	case !status.PeriodClosed:
		return nil, fmt.Errorf("fiscal year must be closed before carry-forward")
	case !status.HasProfitAndLossActivity:
		return nil, fmt.Errorf("no revenue or expense activity found for this fiscal year")
	case status.ExistingCarryForward != nil:
		return nil, fmt.Errorf("carry-forward already exists for fiscal year ending %s", status.FiscalYearEndDate)
	}

	fiscalYearEndDate, err := time.Parse(yearEndDateLayout, status.FiscalYearEndDate)
	if err != nil {
		return nil, fmt.Errorf("parse fiscal year end date: %w", err)
	}
	fiscalYearStartDate, err := time.Parse(yearEndDateLayout, status.FiscalYearStartDate)
	if err != nil {
		return nil, fmt.Errorf("parse fiscal year start date: %w", err)
	}

	periodBalances, err := s.repo.GetPeriodBalances(ctx, schemaName, tenantID, fiscalYearStartDate, fiscalYearEndDate)
	if err != nil {
		return nil, fmt.Errorf("load fiscal-year balances: %w", err)
	}

	lines, err := buildYearEndCarryForwardLines(periodBalances, status.RetainedEarningsAccount)
	if err != nil {
		return nil, err
	}

	sourceID := yearEndCarryForwardSourceID(tenantID, fiscalYearEndDate)
	entryDescription := fmt.Sprintf("Year-end carry-forward for fiscal year ending %s", status.FiscalYearEndDate)
	entryReference := fmt.Sprintf("CF-%s", fiscalYearEndDate.Format("20060102"))
	entryDate := fiscalYearEndDate.AddDate(0, 0, 1)

	entry, err := s.CreateJournalEntry(ctx, schemaName, tenantID, &CreateJournalEntryRequest{
		EntryDate:   entryDate,
		Description: entryDescription,
		Reference:   entryReference,
		SourceType:  SourceTypeYearEndCarryForward,
		SourceID:    &sourceID,
		Lines:       lines,
		UserID:      req.UserID,
	})
	if err != nil {
		return nil, fmt.Errorf("create carry-forward journal entry: %w", err)
	}

	if err := s.PostJournalEntry(ctx, schemaName, tenantID, entry.ID, req.UserID); err != nil {
		return nil, fmt.Errorf("post carry-forward journal entry: %w", err)
	}

	postedEntry, err := s.GetJournalEntry(ctx, schemaName, tenantID, entry.ID)
	if err != nil {
		return nil, fmt.Errorf("load carry-forward journal entry: %w", err)
	}

	updatedStatus, err := s.GetYearEndCloseStatus(ctx, schemaName, tenantID, fiscalYearStartMonth, req.PeriodEndDate, lockedThroughDate)
	if err != nil {
		return nil, err
	}

	return &YearEndCarryForwardResult{
		JournalEntry: postedEntry,
		Status:       updatedStatus,
	}, nil
}

func buildYearEndCarryForwardLines(periodBalances []AccountBalance, retainedEarningsAccount *AccountSummary) ([]CreateJournalEntryLineReq, error) {
	lines := make([]CreateJournalEntryLineReq, 0, len(periodBalances)+1)
	totalDebits := decimal.Zero
	totalCredits := decimal.Zero

	for _, balance := range periodBalances {
		if balance.NetBalance.IsZero() {
			continue
		}

		line := CreateJournalEntryLineReq{
			AccountID:    balance.AccountID,
			Description:  "Year-end carry-forward",
			Currency:     "EUR",
			ExchangeRate: decimal.NewFromInt(1),
		}

		switch balance.AccountType {
		case AccountTypeRevenue:
			if balance.NetBalance.GreaterThan(decimal.Zero) {
				line.DebitAmount = balance.NetBalance
				totalDebits = totalDebits.Add(balance.NetBalance)
			} else {
				line.CreditAmount = balance.NetBalance.Abs()
				totalCredits = totalCredits.Add(balance.NetBalance.Abs())
			}
		case AccountTypeExpense:
			if balance.NetBalance.GreaterThan(decimal.Zero) {
				line.CreditAmount = balance.NetBalance
				totalCredits = totalCredits.Add(balance.NetBalance)
			} else {
				line.DebitAmount = balance.NetBalance.Abs()
				totalDebits = totalDebits.Add(balance.NetBalance.Abs())
			}
		default:
			continue
		}

		lines = append(lines, line)
	}

	if len(lines) == 0 {
		return nil, fmt.Errorf("no revenue or expense activity found for this fiscal year")
	}

	diff := totalDebits.Sub(totalCredits)
	if !diff.IsZero() {
		if retainedEarningsAccount == nil {
			return nil, fmt.Errorf("retained earnings account is required before carry-forward")
		}

		retainedLine := CreateJournalEntryLineReq{
			AccountID:    retainedEarningsAccount.ID,
			Description:  "Retained earnings carry-forward",
			Currency:     "EUR",
			ExchangeRate: decimal.NewFromInt(1),
		}
		if diff.GreaterThan(decimal.Zero) {
			retainedLine.CreditAmount = diff
			totalCredits = totalCredits.Add(diff)
		} else {
			retainedLine.DebitAmount = diff.Abs()
			totalDebits = totalDebits.Add(diff.Abs())
		}
		lines = append(lines, retainedLine)
	}

	if !totalDebits.Equal(totalCredits) {
		return nil, fmt.Errorf("carry-forward journal entry does not balance")
	}

	return lines, nil
}

func carryForwardDiff(periodBalances []AccountBalance) decimal.Decimal {
	totalDebits := decimal.Zero
	totalCredits := decimal.Zero

	for _, balance := range periodBalances {
		if balance.NetBalance.IsZero() {
			continue
		}
		switch balance.AccountType {
		case AccountTypeRevenue:
			if balance.NetBalance.GreaterThan(decimal.Zero) {
				totalDebits = totalDebits.Add(balance.NetBalance)
			} else {
				totalCredits = totalCredits.Add(balance.NetBalance.Abs())
			}
		case AccountTypeExpense:
			if balance.NetBalance.GreaterThan(decimal.Zero) {
				totalCredits = totalCredits.Add(balance.NetBalance)
			} else {
				totalDebits = totalDebits.Add(balance.NetBalance.Abs())
			}
		}
	}

	return totalDebits.Sub(totalCredits).Abs()
}

func findRetainedEarningsAccount(accounts []Account) *Account {
	bestScore := -1
	var best *Account

	for i := range accounts {
		account := &accounts[i]
		if account.AccountType != AccountTypeEquity {
			continue
		}

		score := 0
		normalizedName := strings.ToLower(strings.TrimSpace(account.Name))
		switch {
		case normalizedName == "retained earnings":
			score += 100
		case strings.Contains(normalizedName, "retained earnings"):
			score += 80
		}

		switch strings.TrimSpace(account.Code) {
		case "3200":
			score += 50
		case "3100":
			score += 30
		}

		if score > bestScore {
			bestScore = score
			best = account
		}
	}

	if bestScore <= 0 {
		return nil
	}

	return best
}

func periodClosedThrough(rawLockDate *string, targetDate time.Time) (bool, *string, error) {
	if rawLockDate == nil {
		return false, nil, nil
	}

	value := strings.TrimSpace(*rawLockDate)
	if value == "" {
		return false, nil, nil
	}

	lockDate, err := time.Parse(yearEndDateLayout, value)
	if err != nil {
		return false, nil, fmt.Errorf("invalid tenant period lock date %q: %w", value, err)
	}

	normalizedLockDate := normalizeYearEndDate(lockDate)
	formatted := normalizedLockDate.Format(yearEndDateLayout)
	return !targetDate.After(normalizedLockDate), &formatted, nil
}

func parseYearEndDate(raw string) (time.Time, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return time.Time{}, fmt.Errorf("period end date is required")
	}

	parsed, err := time.Parse(yearEndDateLayout, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("period end date must use YYYY-MM-DD")
	}

	normalized := normalizeYearEndDate(parsed)
	if normalized.Day() != time.Date(normalized.Year(), normalized.Month()+1, 0, 0, 0, 0, 0, time.UTC).Day() {
		return time.Time{}, fmt.Errorf("period end date must be the last day of a month")
	}

	return normalized, nil
}

func fiscalYearBounds(periodEndDate time.Time, fiscalYearStartMonth int) (time.Time, time.Time) {
	if fiscalYearStartMonth <= 0 || fiscalYearStartMonth > 12 {
		fiscalYearStartMonth = 1
	}

	startYear := periodEndDate.Year()
	if int(periodEndDate.Month()) < fiscalYearStartMonth {
		startYear--
	}

	startDate := time.Date(startYear, time.Month(fiscalYearStartMonth), 1, 0, 0, 0, 0, time.UTC)
	endDate := startDate.AddDate(1, 0, -1)
	return startDate, endDate
}

func fiscalYearLabel(startDate, endDate time.Time) string {
	if startDate.Year() == endDate.Year() {
		return fmt.Sprintf("%d", endDate.Year())
	}
	return fmt.Sprintf("%d/%d", startDate.Year(), endDate.Year())
}

func yearEndCarryForwardSourceID(tenantID string, fiscalYearEndDate time.Time) string {
	return uuid.NewSHA1(uuid.NameSpaceOID, []byte(fmt.Sprintf("year-end-carry-forward:%s:%s", tenantID, fiscalYearEndDate.Format(yearEndDateLayout)))).String()
}

// YearEndCarryForwardSourceID returns the deterministic source UUID used for a fiscal year.
func YearEndCarryForwardSourceID(tenantID string, fiscalYearEndDate time.Time) string {
	return yearEndCarryForwardSourceID(tenantID, fiscalYearEndDate)
}

func normalizeYearEndDate(value time.Time) time.Time {
	utcValue := value.UTC()
	return time.Date(utcValue.Year(), utcValue.Month(), utcValue.Day(), 0, 0, 0, 0, time.UTC)
}
