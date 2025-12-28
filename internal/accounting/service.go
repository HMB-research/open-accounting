package accounting

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

// Service provides accounting operations
type Service struct {
	db   *pgxpool.Pool
	repo *Repository
}

// NewService creates a new accounting service
func NewService(db *pgxpool.Pool) *Service {
	return &Service{
		db:   db,
		repo: NewRepository(db),
	}
}

// GetAccount retrieves an account by ID
func (s *Service) GetAccount(ctx context.Context, tenantID, accountID string) (*Account, error) {
	return s.repo.GetAccountByID(ctx, tenantID, accountID)
}

// ListAccounts retrieves all accounts for a tenant
func (s *Service) ListAccounts(ctx context.Context, tenantID string, activeOnly bool) ([]Account, error) {
	return s.repo.ListAccounts(ctx, tenantID, activeOnly)
}

// CreateAccount creates a new account
func (s *Service) CreateAccount(ctx context.Context, tenantID string, req *CreateAccountRequest) (*Account, error) {
	account := &Account{
		ID:          uuid.New().String(),
		TenantID:    tenantID,
		Code:        req.Code,
		Name:        req.Name,
		AccountType: req.AccountType,
		ParentID:    req.ParentID,
		IsActive:    true,
		IsSystem:    false,
		Description: req.Description,
		CreatedAt:   time.Now(),
	}

	if err := s.repo.CreateAccount(ctx, account); err != nil {
		return nil, err
	}
	return account, nil
}

// CreateAccountRequest is the request to create an account
type CreateAccountRequest struct {
	Code        string      `json:"code"`
	Name        string      `json:"name"`
	AccountType AccountType `json:"account_type"`
	ParentID    *string     `json:"parent_id,omitempty"`
	Description string      `json:"description,omitempty"`
}

// GetJournalEntry retrieves a journal entry by ID
func (s *Service) GetJournalEntry(ctx context.Context, tenantID, entryID string) (*JournalEntry, error) {
	return s.repo.GetJournalEntryByID(ctx, tenantID, entryID)
}

// CreateJournalEntry creates a new journal entry
func (s *Service) CreateJournalEntry(ctx context.Context, tenantID string, req *CreateJournalEntryRequest) (*JournalEntry, error) {
	entry := &JournalEntry{
		ID:          uuid.New().String(),
		TenantID:    tenantID,
		EntryDate:   req.EntryDate,
		Description: req.Description,
		Reference:   req.Reference,
		SourceType:  req.SourceType,
		SourceID:    req.SourceID,
		Status:      StatusDraft,
		CreatedAt:   time.Now(),
		CreatedBy:   req.UserID,
	}

	// Convert request lines to entry lines
	for _, reqLine := range req.Lines {
		currency := reqLine.Currency
		if currency == "" {
			currency = "EUR"
		}
		exchangeRate := reqLine.ExchangeRate
		if exchangeRate.IsZero() {
			exchangeRate = decimal.NewFromInt(1)
		}

		line := JournalEntryLine{
			ID:           uuid.New().String(),
			AccountID:    reqLine.AccountID,
			Description:  reqLine.Description,
			DebitAmount:  reqLine.DebitAmount,
			CreditAmount: reqLine.CreditAmount,
			Currency:     currency,
			ExchangeRate: exchangeRate,
			BaseDebit:    reqLine.DebitAmount.Mul(exchangeRate),
			BaseCredit:   reqLine.CreditAmount.Mul(exchangeRate),
		}
		entry.Lines = append(entry.Lines, line)
	}

	// Validate the entry balances
	if err := entry.Validate(); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Create in database
	if err := s.repo.CreateJournalEntry(ctx, entry); err != nil {
		return nil, err
	}

	return entry, nil
}

// PostJournalEntry posts a draft journal entry
func (s *Service) PostJournalEntry(ctx context.Context, tenantID, entryID, userID string) error {
	// Get the entry to verify it exists and is in draft status
	entry, err := s.repo.GetJournalEntryByID(ctx, tenantID, entryID)
	if err != nil {
		return err
	}

	if entry.Status != StatusDraft {
		return fmt.Errorf("only draft entries can be posted, current status: %s", entry.Status)
	}

	// Validate the entry still balances
	if err := entry.Validate(); err != nil {
		return fmt.Errorf("entry validation failed: %w", err)
	}

	return s.repo.UpdateJournalEntryStatus(ctx, tenantID, entryID, StatusPosted, userID)
}

// VoidJournalEntry voids a posted journal entry by creating a reversing entry
func (s *Service) VoidJournalEntry(ctx context.Context, tenantID, entryID, userID, reason string) (*JournalEntry, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Get the original entry
	original, err := s.repo.GetJournalEntryByID(ctx, tenantID, entryID)
	if err != nil {
		return nil, err
	}

	if original.Status != StatusPosted {
		return nil, fmt.Errorf("only posted entries can be voided, current status: %s", original.Status)
	}

	// Mark original as voided
	now := time.Now()
	_, err = tx.Exec(ctx, `
		UPDATE journal_entries
		SET status = $1, voided_at = $2, voided_by = $3, void_reason = $4
		WHERE id = $5 AND tenant_id = $6
	`, StatusVoided, now, userID, reason, entryID, tenantID)
	if err != nil {
		return nil, fmt.Errorf("mark entry as voided: %w", err)
	}

	// Create reversing entry
	reversal := &JournalEntry{
		ID:          uuid.New().String(),
		TenantID:    tenantID,
		EntryDate:   time.Now(),
		Description: fmt.Sprintf("Reversal of %s: %s", original.EntryNumber, reason),
		Reference:   original.EntryNumber,
		SourceType:  "VOID",
		SourceID:    &original.ID,
		Status:      StatusPosted,
		PostedAt:    &now,
		PostedBy:    &userID,
		CreatedAt:   now,
		CreatedBy:   userID,
	}

	// Reverse debits and credits
	for _, line := range original.Lines {
		reversal.Lines = append(reversal.Lines, JournalEntryLine{
			ID:           uuid.New().String(),
			AccountID:    line.AccountID,
			Description:  "Reversal",
			DebitAmount:  line.CreditAmount, // Swap
			CreditAmount: line.DebitAmount,  // Swap
			Currency:     line.Currency,
			ExchangeRate: line.ExchangeRate,
			BaseDebit:    line.BaseCredit,
			BaseCredit:   line.BaseDebit,
		})
	}

	// Create the reversal entry
	if err := s.repo.CreateJournalEntryTx(ctx, tx, reversal); err != nil {
		return nil, fmt.Errorf("create reversal entry: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit transaction: %w", err)
	}

	return reversal, nil
}

// GetAccountBalance retrieves the balance of an account as of a date
func (s *Service) GetAccountBalance(ctx context.Context, tenantID, accountID string, asOfDate time.Time) (decimal.Decimal, error) {
	return s.repo.GetAccountBalance(ctx, tenantID, accountID, asOfDate)
}

// GetTrialBalance retrieves all account balances as of a date
func (s *Service) GetTrialBalance(ctx context.Context, tenantID string, asOfDate time.Time) (*TrialBalance, error) {
	balances, err := s.repo.GetTrialBalance(ctx, tenantID, asOfDate)
	if err != nil {
		return nil, err
	}

	tb := &TrialBalance{
		TenantID:    tenantID,
		AsOfDate:    asOfDate,
		GeneratedAt: time.Now(),
		Accounts:    balances,
	}

	for _, b := range balances {
		tb.TotalDebits = tb.TotalDebits.Add(b.DebitBalance)
		tb.TotalCredits = tb.TotalCredits.Add(b.CreditBalance)
	}
	tb.IsBalanced = tb.TotalDebits.Equal(tb.TotalCredits)

	return tb, nil
}

// TrialBalance represents a trial balance report
type TrialBalance struct {
	TenantID     string           `json:"tenant_id"`
	AsOfDate     time.Time        `json:"as_of_date"`
	GeneratedAt  time.Time        `json:"generated_at"`
	Accounts     []AccountBalance `json:"accounts"`
	TotalDebits  decimal.Decimal  `json:"total_debits"`
	TotalCredits decimal.Decimal  `json:"total_credits"`
	IsBalanced   bool             `json:"is_balanced"`
}

// Tx interface for transaction support
type Tx interface {
	Exec(ctx context.Context, sql string, arguments ...interface{}) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row
}
