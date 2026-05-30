package expenses

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/HMB-research/open-accounting/internal/accounting"
	"github.com/HMB-research/open-accounting/internal/documents"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

type accountingPoster interface {
	GetAccount(ctx context.Context, schemaName, tenantID, accountID string) (*accounting.Account, error)
	ListAccounts(ctx context.Context, schemaName, tenantID string, activeOnly bool) ([]accounting.Account, error)
	CreateJournalEntry(ctx context.Context, schemaName, tenantID string, req *accounting.CreateJournalEntryRequest) (*accounting.JournalEntry, error)
	PostJournalEntry(ctx context.Context, schemaName, tenantID, entryID, userID string) error
}

type evidenceEvaluator interface {
	EvaluateEvidencePolicy(ctx context.Context, schemaName, tenantID string, req *documents.EvidencePolicyRequest) ([]documents.EvidencePolicyResult, error)
}

type Service struct {
	repo       Repository
	accounting accountingPoster
	evidence   evidenceEvaluator
	now        func() time.Time
}

func NewService(db *pgxpool.Pool, evidence evidenceEvaluator) *Service {
	return NewServiceWithRepository(NewRepository(db), accounting.NewService(db), evidence)
}

func NewServiceWithRepository(repo Repository, accounting accountingPoster, evidence evidenceEvaluator) *Service {
	return &Service{
		repo:       repo,
		accounting: accounting,
		evidence:   evidence,
		now:        func() time.Time { return time.Now().UTC() },
	}
}

func (s *Service) CreateExpense(ctx context.Context, schemaName, tenantID string, req *CreateExpenseRequest) (*Expense, error) {
	if req == nil {
		return nil, fmt.Errorf("expense request is required")
	}
	if strings.TrimSpace(req.UserID) == "" {
		return nil, fmt.Errorf("user id is required")
	}
	if strings.TrimSpace(req.Merchant) == "" {
		return nil, fmt.Errorf("merchant is required")
	}
	if strings.TrimSpace(req.ExpenseAccountID) == "" {
		return nil, fmt.Errorf("expense_account_id is required")
	}
	if strings.TrimSpace(req.PaymentAccountID) == "" {
		return nil, fmt.Errorf("payment_account_id is required")
	}
	if req.Amount.LessThanOrEqual(decimal.Zero) {
		return nil, fmt.Errorf("amount must be positive")
	}
	currency, err := normalizeCurrency(req.Currency)
	if err != nil {
		return nil, err
	}
	exchangeRate, err := normalizeExchangeRate(req.ExchangeRate)
	if err != nil {
		return nil, err
	}

	expenseDate := normalizeExpenseDate(req.ExpenseDate, s.now())
	number, err := s.repo.GenerateNumber(ctx, schemaName, tenantID)
	if err != nil {
		return nil, err
	}

	requiresReceipt := true
	if req.RequiresReceipt != nil {
		requiresReceipt = *req.RequiresReceipt
	}
	now := s.now()
	expense := &Expense{
		ID:               uuid.New().String(),
		TenantID:         tenantID,
		ExpenseNumber:    number,
		ExpenseDate:      expenseDate,
		Merchant:         strings.TrimSpace(req.Merchant),
		Description:      strings.TrimSpace(req.Description),
		EmployeeID:       trimStringPtr(req.EmployeeID),
		ContactID:        trimStringPtr(req.ContactID),
		ExpenseAccountID: strings.TrimSpace(req.ExpenseAccountID),
		PaymentAccountID: strings.TrimSpace(req.PaymentAccountID),
		Amount:           req.Amount,
		Currency:         currency,
		ExchangeRate:     exchangeRate,
		BaseAmount:       req.Amount.Mul(exchangeRate).Round(2),
		RequiresReceipt:  requiresReceipt,
		Status:           StatusDraft,
		CreatedAt:        now,
		CreatedBy:        strings.TrimSpace(req.UserID),
		UpdatedAt:        now,
	}

	if err := s.repo.Create(ctx, schemaName, expense); err != nil {
		return nil, err
	}
	return expense, nil
}

func (s *Service) ListExpenses(ctx context.Context, schemaName, tenantID string, filter ListExpensesFilter) ([]Expense, error) {
	status, err := normalizeOptionalStatus(filter.Status)
	if err != nil {
		return nil, err
	}
	filter.Status = status
	return s.repo.List(ctx, schemaName, tenantID, filter)
}

func (s *Service) GetExpense(ctx context.Context, schemaName, tenantID, expenseID string) (*Expense, error) {
	if strings.TrimSpace(expenseID) == "" {
		return nil, fmt.Errorf("expense id is required")
	}
	return s.repo.GetByID(ctx, schemaName, tenantID, strings.TrimSpace(expenseID))
}

func (s *Service) SubmitExpense(ctx context.Context, schemaName, tenantID, expenseID string, req *ExpenseActionRequest) (*Expense, error) {
	expense, err := s.GetExpense(ctx, schemaName, tenantID, expenseID)
	if err != nil {
		return nil, err
	}
	userID, err := actionUserID(req)
	if err != nil {
		return nil, err
	}
	if expense.Status != StatusDraft && expense.Status != StatusRejected {
		return nil, fmt.Errorf("%w: only draft or rejected expenses can be submitted", ErrInvalidStatusTransition)
	}

	now := s.now()
	expense.Status = StatusSubmitted
	expense.SubmittedAt = &now
	expense.SubmittedBy = &userID
	expense.RejectedAt = nil
	expense.RejectedBy = nil
	expense.RejectionReason = ""
	if err := s.repo.Update(ctx, schemaName, expense); err != nil {
		return nil, err
	}
	return expense, nil
}

func (s *Service) ApproveExpense(ctx context.Context, schemaName, tenantID, expenseID string, req *ExpenseActionRequest) (*Expense, error) {
	expense, err := s.GetExpense(ctx, schemaName, tenantID, expenseID)
	if err != nil {
		return nil, err
	}
	userID, err := actionUserID(req)
	if err != nil {
		return nil, err
	}
	if expense.Status != StatusSubmitted {
		return nil, fmt.Errorf("%w: only submitted expenses can be approved", ErrInvalidStatusTransition)
	}
	if err := s.requireApprovedReceipt(ctx, schemaName, tenantID, expense); err != nil {
		return nil, err
	}

	now := s.now()
	expense.Status = StatusApproved
	expense.ApprovedAt = &now
	expense.ApprovedBy = &userID
	expense.RejectedAt = nil
	expense.RejectedBy = nil
	expense.RejectionReason = ""
	if err := s.repo.Update(ctx, schemaName, expense); err != nil {
		return nil, err
	}
	return expense, nil
}

func (s *Service) RejectExpense(ctx context.Context, schemaName, tenantID, expenseID string, req *RejectExpenseRequest) (*Expense, error) {
	expense, err := s.GetExpense(ctx, schemaName, tenantID, expenseID)
	if err != nil {
		return nil, err
	}
	if req == nil {
		return nil, fmt.Errorf("reject request is required")
	}
	userID := strings.TrimSpace(req.UserID)
	if userID == "" {
		return nil, fmt.Errorf("user id is required")
	}
	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		return nil, fmt.Errorf("reason is required")
	}
	if expense.Status != StatusSubmitted {
		return nil, fmt.Errorf("%w: only submitted expenses can be rejected", ErrInvalidStatusTransition)
	}

	now := s.now()
	expense.Status = StatusRejected
	expense.RejectedAt = &now
	expense.RejectedBy = &userID
	expense.RejectionReason = reason
	if err := s.repo.Update(ctx, schemaName, expense); err != nil {
		return nil, err
	}
	return expense, nil
}

func (s *Service) PostExpense(ctx context.Context, schemaName, tenantID, expenseID string, req *ExpenseActionRequest) (*Expense, error) {
	expense, err := s.GetExpense(ctx, schemaName, tenantID, expenseID)
	if err != nil {
		return nil, err
	}
	userID, err := actionUserID(req)
	if err != nil {
		return nil, err
	}
	if expense.Status == StatusPosted {
		return nil, ErrExpenseAlreadyPosted
	}
	if expense.Status != StatusApproved {
		return nil, fmt.Errorf("%w: only approved expenses can be posted", ErrInvalidStatusTransition)
	}
	if err := s.requireApprovedReceipt(ctx, schemaName, tenantID, expense); err != nil {
		return nil, err
	}
	if err := s.validatePostingAccounts(ctx, schemaName, tenantID, expense); err != nil {
		return nil, err
	}

	sourceID := expense.ID
	entry, err := s.accounting.CreateJournalEntry(ctx, schemaName, tenantID, &accounting.CreateJournalEntryRequest{
		EntryDate:   expense.ExpenseDate,
		Description: expenseJournalDescription(expense),
		Reference:   expense.ExpenseNumber,
		SourceType:  SourceTypeExpense,
		SourceID:    &sourceID,
		UserID:      userID,
		Lines: []accounting.CreateJournalEntryLineReq{
			{
				AccountID:    expense.ExpenseAccountID,
				Description:  expenseJournalDescription(expense),
				DebitAmount:  expense.Amount,
				CreditAmount: decimal.Zero,
				Currency:     expense.Currency,
				ExchangeRate: expense.ExchangeRate,
			},
			{
				AccountID:    expense.PaymentAccountID,
				Description:  expenseJournalDescription(expense),
				DebitAmount:  decimal.Zero,
				CreditAmount: expense.Amount,
				Currency:     expense.Currency,
				ExchangeRate: expense.ExchangeRate,
			},
		},
	})
	if err != nil {
		return nil, err
	}
	if err := s.accounting.PostJournalEntry(ctx, schemaName, tenantID, entry.ID, userID); err != nil {
		return nil, err
	}

	now := s.now()
	expense.Status = StatusPosted
	expense.JournalEntryID = &entry.ID
	expense.PostedAt = &now
	expense.PostedBy = &userID
	if err := s.repo.Update(ctx, schemaName, expense); err != nil {
		return nil, err
	}
	return expense, nil
}

func (s *Service) requireApprovedReceipt(ctx context.Context, schemaName, tenantID string, expense *Expense) error {
	if expense == nil || !expense.RequiresReceipt {
		return nil
	}
	if s.evidence == nil {
		return fmt.Errorf("%w: document evidence service is unavailable", ErrApprovedReceiptRequired)
	}
	results, err := s.evidence.EvaluateEvidencePolicy(ctx, schemaName, tenantID, &documents.EvidencePolicyRequest{
		EntityType: documents.EntityTypeExpense,
		EntityIDs:  []string{expense.ID},
		Rules: []documents.EvidencePolicyRule{{
			DocumentTypes:   []string{documents.DocumentTypeReceipt},
			MinCount:        1,
			RequireApproved: true,
		}},
	})
	if err != nil {
		return fmt.Errorf("evaluate expense receipt evidence: %w", err)
	}
	if len(results) == 0 || !results[0].Compliant {
		return fmt.Errorf("%w before approving expense %s", ErrApprovedReceiptRequired, expense.ID)
	}
	return nil
}

func (s *Service) validatePostingAccounts(ctx context.Context, schemaName, tenantID string, expense *Expense) error {
	if s.accounting == nil {
		return fmt.Errorf("%w: accounting service is unavailable", ErrExpenseAccountingInvalid)
	}
	expenseAccount, err := s.accounting.GetAccount(ctx, schemaName, tenantID, expense.ExpenseAccountID)
	if err != nil {
		return fmt.Errorf("%w: load expense account: %v", ErrExpenseAccountingInvalid, err)
	}
	if expenseAccount.AccountType != accounting.AccountTypeExpense {
		return fmt.Errorf("%w: expense account must be EXPENSE", ErrExpenseAccountingInvalid)
	}
	paymentAccount, err := s.accounting.GetAccount(ctx, schemaName, tenantID, expense.PaymentAccountID)
	if err != nil {
		return fmt.Errorf("%w: load payment account: %v", ErrExpenseAccountingInvalid, err)
	}
	if paymentAccount.AccountType != accounting.AccountTypeAsset && paymentAccount.AccountType != accounting.AccountTypeLiability {
		return fmt.Errorf("%w: payment account must be ASSET or LIABILITY", ErrExpenseAccountingInvalid)
	}
	return nil
}

func actionUserID(req *ExpenseActionRequest) (string, error) {
	if req == nil {
		return "", fmt.Errorf("action request is required")
	}
	userID := strings.TrimSpace(req.UserID)
	if userID == "" {
		return "", fmt.Errorf("user id is required")
	}
	return userID, nil
}

func expenseJournalDescription(expense *Expense) string {
	description := strings.TrimSpace(expense.Description)
	if description == "" {
		return fmt.Sprintf("Expense %s - %s", expense.ExpenseNumber, expense.Merchant)
	}
	return fmt.Sprintf("Expense %s - %s: %s", expense.ExpenseNumber, expense.Merchant, description)
}

func normalizeOptionalStatus(status ExpenseStatus) (ExpenseStatus, error) {
	trimmed := ExpenseStatus(strings.ToUpper(strings.TrimSpace(string(status))))
	if trimmed == "" {
		return "", nil
	}
	if !isValidStatus(trimmed) {
		return "", fmt.Errorf("unsupported expense status %q", status)
	}
	return trimmed, nil
}

func isValidStatus(status ExpenseStatus) bool {
	switch status {
	case StatusDraft, StatusSubmitted, StatusApproved, StatusRejected, StatusPosted:
		return true
	default:
		return false
	}
}

func normalizeCurrency(value string) (string, error) {
	trimmed := strings.ToUpper(strings.TrimSpace(value))
	if trimmed == "" {
		return "EUR", nil
	}
	if len(trimmed) != 3 {
		return "", fmt.Errorf("currency must be a 3-letter ISO code")
	}
	return trimmed, nil
}

func normalizeExchangeRate(value decimal.Decimal) (decimal.Decimal, error) {
	if value.IsZero() {
		return decimal.NewFromInt(1), nil
	}
	if value.LessThanOrEqual(decimal.Zero) {
		return decimal.Zero, fmt.Errorf("exchange_rate must be positive")
	}
	return value, nil
}

func normalizeExpenseDate(value, fallback time.Time) time.Time {
	if value.IsZero() {
		value = fallback
	}
	utc := value.UTC()
	return time.Date(utc.Year(), utc.Month(), utc.Day(), 0, 0, 0, 0, time.UTC)
}

func trimStringPtr(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}
