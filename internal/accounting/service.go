package accounting

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

// JournalEntryTemplateRepository is the optional repository surface for reusable entry templates.
type JournalEntryTemplateRepository interface {
	CreateJournalEntryTemplate(ctx context.Context, schemaName string, template *JournalEntryTemplate) error
	ListJournalEntryTemplates(ctx context.Context, schemaName, tenantID string, activeOnly bool) ([]JournalEntryTemplate, error)
	GetJournalEntryTemplateByID(ctx context.Context, schemaName, tenantID, templateID string) (*JournalEntryTemplate, error)
}

var (
	errJournalEntryTemplatesUnsupported = errors.New("journal entry templates are not supported by repository")
	ErrTemplateEvidenceAutoPost         = errors.New("cannot auto-post a template entry that requires evidence")
)

// Service provides accounting operations
type Service struct {
	db   *pgxpool.Pool
	repo RepositoryInterface
}

// NewService creates a new accounting service
func NewService(db *pgxpool.Pool) *Service {
	return &Service{
		db:   db,
		repo: NewRepository(db),
	}
}

// NewServiceWithRepo creates a new accounting service with a custom repository (for testing)
func NewServiceWithRepo(db *pgxpool.Pool, repo RepositoryInterface) *Service {
	return &Service{
		db:   db,
		repo: repo,
	}
}

// GetAccount retrieves an account by ID
func (s *Service) GetAccount(ctx context.Context, schemaName, tenantID, accountID string) (*Account, error) {
	return s.repo.GetAccountByID(ctx, schemaName, tenantID, accountID)
}

// ListAccounts retrieves all accounts for a tenant
func (s *Service) ListAccounts(ctx context.Context, schemaName, tenantID string, activeOnly bool) ([]Account, error) {
	return s.repo.ListAccounts(ctx, schemaName, tenantID, activeOnly)
}

// CreateAccount creates a new account
func (s *Service) CreateAccount(ctx context.Context, schemaName, tenantID string, req *CreateAccountRequest) (*Account, error) {
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

	if err := s.repo.CreateAccount(ctx, schemaName, account); err != nil {
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
func (s *Service) GetJournalEntry(ctx context.Context, schemaName, tenantID, entryID string) (*JournalEntry, error) {
	return s.repo.GetJournalEntryByID(ctx, schemaName, tenantID, entryID)
}

// ListJournalEntries retrieves recent journal entries for a tenant.
func (s *Service) ListJournalEntries(ctx context.Context, schemaName, tenantID string, limit int) ([]JournalEntry, error) {
	return s.repo.ListJournalEntries(ctx, schemaName, tenantID, limit)
}

// CreateJournalEntry creates a new journal entry
func (s *Service) CreateJournalEntry(ctx context.Context, schemaName, tenantID string, req *CreateJournalEntryRequest) (*JournalEntry, error) {
	entry := &JournalEntry{
		ID:               uuid.New().String(),
		TenantID:         tenantID,
		EntryDate:        req.EntryDate,
		Description:      req.Description,
		Reference:        req.Reference,
		SourceType:       req.SourceType,
		SourceID:         req.SourceID,
		RequiresEvidence: req.RequiresEvidence,
		Status:           StatusDraft,
		CreatedAt:        time.Now(),
		CreatedBy:        req.UserID,
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
	if err := s.repo.CreateJournalEntry(ctx, schemaName, entry); err != nil {
		return nil, err
	}

	return entry, nil
}

// CreateJournalEntryTemplate stores a balanced reusable journal entry template.
func (s *Service) CreateJournalEntryTemplate(ctx context.Context, schemaName, tenantID string, req *CreateJournalEntryTemplateRequest) (*JournalEntryTemplate, error) {
	repo, err := s.templateRepository()
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(req.Name) == "" {
		return nil, errors.New("name is required")
	}
	if len(req.Lines) < 2 {
		return nil, errors.New("at least two lines are required")
	}

	now := time.Now()
	template := &JournalEntryTemplate{
		ID:               uuid.New().String(),
		TenantID:         tenantID,
		Name:             strings.TrimSpace(req.Name),
		Description:      strings.TrimSpace(req.Description),
		Reference:        strings.TrimSpace(req.Reference),
		RequiresEvidence: req.RequiresEvidence,
		IsActive:         true,
		CreatedAt:        now,
		CreatedBy:        req.UserID,
		UpdatedAt:        now,
	}
	for i, reqLine := range req.Lines {
		line := newJournalEntryTemplateLine(template.ID, i+1, reqLine)
		template.Lines = append(template.Lines, line)
	}
	template.LineCount = len(template.Lines)

	if err := validateJournalEntryTemplate(template); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}
	if err := repo.CreateJournalEntryTemplate(ctx, schemaName, template); err != nil {
		return nil, fmt.Errorf("create journal entry template: %w", err)
	}
	return template, nil
}

// ListJournalEntryTemplates retrieves reusable journal entry templates.
func (s *Service) ListJournalEntryTemplates(ctx context.Context, schemaName, tenantID string, activeOnly bool) ([]JournalEntryTemplate, error) {
	repo, err := s.templateRepository()
	if err != nil {
		return nil, err
	}
	return repo.ListJournalEntryTemplates(ctx, schemaName, tenantID, activeOnly)
}

// GetJournalEntryTemplate retrieves a reusable journal entry template.
func (s *Service) GetJournalEntryTemplate(ctx context.Context, schemaName, tenantID, templateID string) (*JournalEntryTemplate, error) {
	repo, err := s.templateRepository()
	if err != nil {
		return nil, err
	}
	return repo.GetJournalEntryTemplateByID(ctx, schemaName, tenantID, templateID)
}

// ApplyJournalEntryTemplate creates a journal entry from a reusable template.
func (s *Service) ApplyJournalEntryTemplate(ctx context.Context, schemaName, tenantID, templateID string, req *ApplyJournalEntryTemplateRequest) (*JournalEntry, error) {
	template, err := s.GetJournalEntryTemplate(ctx, schemaName, tenantID, templateID)
	if err != nil {
		return nil, err
	}
	if !template.IsActive {
		return nil, errors.New("journal entry template is inactive")
	}
	if req.Post && template.RequiresEvidence {
		return nil, ErrTemplateEvidenceAutoPost
	}

	entryDate := req.EntryDate
	if entryDate.IsZero() {
		entryDate = time.Now()
	}
	description := strings.TrimSpace(req.Description)
	if description == "" {
		description = template.Description
	}
	reference := strings.TrimSpace(req.Reference)
	if reference == "" {
		reference = template.Reference
	}

	sourceID := template.ID
	lines := make([]CreateJournalEntryLineReq, 0, len(template.Lines))
	for _, line := range template.Lines {
		lines = append(lines, CreateJournalEntryLineReq{
			AccountID:    line.AccountID,
			Description:  line.Description,
			DebitAmount:  line.DebitAmount,
			CreditAmount: line.CreditAmount,
			Currency:     line.Currency,
			ExchangeRate: line.ExchangeRate,
		})
	}

	entry, err := s.CreateJournalEntry(ctx, schemaName, tenantID, &CreateJournalEntryRequest{
		EntryDate:        entryDate,
		Description:      description,
		Reference:        reference,
		SourceType:       SourceTypeJournalTemplate,
		SourceID:         &sourceID,
		RequiresEvidence: template.RequiresEvidence,
		Lines:            lines,
		UserID:           req.UserID,
	})
	if err != nil {
		return nil, err
	}
	if !req.Post {
		return entry, nil
	}
	if err := s.PostJournalEntry(ctx, schemaName, tenantID, entry.ID, req.UserID); err != nil {
		return nil, err
	}
	return s.GetJournalEntry(ctx, schemaName, tenantID, entry.ID)
}

func (s *Service) templateRepository() (JournalEntryTemplateRepository, error) {
	repo, ok := s.repo.(JournalEntryTemplateRepository)
	if !ok {
		return nil, errJournalEntryTemplatesUnsupported
	}
	return repo, nil
}

func newJournalEntryTemplateLine(templateID string, lineNumber int, reqLine CreateJournalEntryLineReq) JournalEntryTemplateLine {
	currency := strings.TrimSpace(reqLine.Currency)
	if currency == "" {
		currency = "EUR"
	}
	exchangeRate := reqLine.ExchangeRate
	if exchangeRate.IsZero() {
		exchangeRate = decimal.NewFromInt(1)
	}
	return JournalEntryTemplateLine{
		ID:           uuid.New().String(),
		TemplateID:   templateID,
		LineNumber:   lineNumber,
		AccountID:    strings.TrimSpace(reqLine.AccountID),
		Description:  strings.TrimSpace(reqLine.Description),
		DebitAmount:  reqLine.DebitAmount,
		CreditAmount: reqLine.CreditAmount,
		Currency:     currency,
		ExchangeRate: exchangeRate,
	}
}

func validateJournalEntryTemplate(template *JournalEntryTemplate) error {
	entry := &JournalEntry{Lines: make([]JournalEntryLine, 0, len(template.Lines))}
	for _, templateLine := range template.Lines {
		if strings.TrimSpace(templateLine.AccountID) == "" {
			return errors.New("line account_id is required")
		}
		entry.Lines = append(entry.Lines, JournalEntryLine{
			AccountID:    templateLine.AccountID,
			Description:  templateLine.Description,
			DebitAmount:  templateLine.DebitAmount,
			CreditAmount: templateLine.CreditAmount,
			Currency:     templateLine.Currency,
			ExchangeRate: templateLine.ExchangeRate,
			BaseDebit:    templateLine.DebitAmount.Mul(templateLine.ExchangeRate),
			BaseCredit:   templateLine.CreditAmount.Mul(templateLine.ExchangeRate),
		})
	}
	return entry.Validate()
}

// PostJournalEntry posts a draft journal entry
func (s *Service) PostJournalEntry(ctx context.Context, schemaName, tenantID, entryID, userID string) error {
	// Get the entry to verify it exists and is in draft status
	entry, err := s.repo.GetJournalEntryByID(ctx, schemaName, tenantID, entryID)
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

	return s.repo.UpdateJournalEntryStatus(ctx, schemaName, tenantID, entryID, StatusPosted, userID)
}

// VoidJournalEntry voids a posted journal entry by creating a reversing entry
func (s *Service) VoidJournalEntry(ctx context.Context, schemaName, tenantID, entryID, userID, reason string) (*JournalEntry, error) {
	// Get the original entry
	original, err := s.repo.GetJournalEntryByID(ctx, schemaName, tenantID, entryID)
	if err != nil {
		return nil, err
	}

	return s.voidPostedJournalEntry(ctx, schemaName, tenantID, original, userID, reason, "VOID", time.Now(), fmt.Sprintf("Reversal of %s: %s", original.EntryNumber, reason))
}

func (s *Service) voidPostedJournalEntry(ctx context.Context, schemaName, tenantID string, original *JournalEntry, userID, reason, sourceType string, entryDate time.Time, description string) (*JournalEntry, error) {
	if original.Status != StatusPosted {
		return nil, fmt.Errorf("only posted entries can be voided, current status: %s", original.Status)
	}

	// Create reversing entry
	now := time.Now()
	sourceID := original.ID
	reversal := &JournalEntry{
		ID:          uuid.New().String(),
		TenantID:    tenantID,
		EntryDate:   entryDate,
		Description: description,
		Reference:   original.EntryNumber,
		SourceType:  sourceType,
		SourceID:    &sourceID,
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

	// Void the entry and create reversal via repository
	if err := s.repo.VoidJournalEntry(ctx, schemaName, tenantID, original.ID, userID, reason, reversal); err != nil {
		return nil, err
	}

	return reversal, nil
}

// GetAccountBalance retrieves the balance of an account as of a date
func (s *Service) GetAccountBalance(ctx context.Context, schemaName, tenantID, accountID string, asOfDate time.Time) (decimal.Decimal, error) {
	return s.repo.GetAccountBalance(ctx, schemaName, tenantID, accountID, asOfDate)
}

// GetTrialBalance retrieves all account balances as of a date
func (s *Service) GetTrialBalance(ctx context.Context, schemaName, tenantID string, asOfDate time.Time) (*TrialBalance, error) {
	balances, err := s.repo.GetTrialBalance(ctx, schemaName, tenantID, asOfDate)
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

// GetBalanceSheet generates a balance sheet as of a specific date
func (s *Service) GetBalanceSheet(ctx context.Context, schemaName, tenantID string, asOfDate time.Time) (*BalanceSheet, error) {
	// Get all account balances as of the date
	balances, err := s.repo.GetTrialBalance(ctx, schemaName, tenantID, asOfDate)
	if err != nil {
		return nil, err
	}

	bs := &BalanceSheet{
		TenantID:    tenantID,
		AsOfDate:    asOfDate,
		GeneratedAt: time.Now(),
	}

	// Categorize accounts by type
	for _, b := range balances {
		switch b.AccountType {
		case AccountTypeAsset:
			bs.Assets = append(bs.Assets, b)
			bs.TotalAssets = bs.TotalAssets.Add(b.NetBalance)
		case AccountTypeLiability:
			bs.Liabilities = append(bs.Liabilities, b)
			bs.TotalLiabilities = bs.TotalLiabilities.Add(b.NetBalance.Abs())
		case AccountTypeEquity:
			bs.Equity = append(bs.Equity, b)
			bs.TotalEquity = bs.TotalEquity.Add(b.NetBalance.Abs())
		case AccountTypeRevenue:
			// Revenue contributes to retained earnings (credit balance = positive)
			bs.RetainedEarnings = bs.RetainedEarnings.Add(b.NetBalance.Abs())
		case AccountTypeExpense:
			// Expenses reduce retained earnings (debit balance = positive, so subtract)
			bs.RetainedEarnings = bs.RetainedEarnings.Sub(b.NetBalance)
		}
	}

	// Add retained earnings to total equity
	bs.TotalEquity = bs.TotalEquity.Add(bs.RetainedEarnings)

	// Check if the balance sheet balances: Assets = Liabilities + Equity
	bs.IsBalanced = bs.TotalAssets.Equal(bs.TotalLiabilities.Add(bs.TotalEquity))

	return bs, nil
}

// GetIncomeStatement generates an income statement for a period
func (s *Service) GetIncomeStatement(ctx context.Context, schemaName, tenantID string, startDate, endDate time.Time) (*IncomeStatement, error) {
	// Get balances for the period (revenue and expenses)
	balances, err := s.repo.GetPeriodBalances(ctx, schemaName, tenantID, startDate, endDate)
	if err != nil {
		return nil, err
	}

	is := &IncomeStatement{
		TenantID:    tenantID,
		StartDate:   startDate,
		EndDate:     endDate,
		GeneratedAt: time.Now(),
	}

	// Categorize accounts by type
	for _, b := range balances {
		switch b.AccountType {
		case AccountTypeRevenue:
			is.Revenue = append(is.Revenue, b)
			// Revenue has credit balance, so NetBalance is negative; use absolute value
			is.TotalRevenue = is.TotalRevenue.Add(b.NetBalance.Abs())
		case AccountTypeExpense:
			is.Expenses = append(is.Expenses, b)
			// Expenses have debit balance, so NetBalance is positive
			is.TotalExpenses = is.TotalExpenses.Add(b.NetBalance)
		}
	}

	// Net Income = Revenue - Expenses
	is.NetIncome = is.TotalRevenue.Sub(is.TotalExpenses)

	return is, nil
}

// Tx interface for transaction support
type Tx interface {
	Exec(ctx context.Context, sql string, arguments ...interface{}) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row
}
