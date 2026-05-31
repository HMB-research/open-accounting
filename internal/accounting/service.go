package accounting

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

// JournalEntryTemplateRepository is the optional repository surface for reusable entry templates.
type JournalEntryTemplateRepository interface {
	CreateJournalEntryTemplate(ctx context.Context, schemaName string, template *JournalEntryTemplate) error
	ListJournalEntryTemplates(ctx context.Context, schemaName, tenantID string, activeOnly bool) ([]JournalEntryTemplate, error)
	GetJournalEntryTemplateByID(ctx context.Context, schemaName, tenantID, templateID string) (*JournalEntryTemplate, error)
	GetDueJournalEntryTemplateIDs(ctx context.Context, schemaName, tenantID string, asOfDate time.Time) ([]string, error)
	UpdateJournalEntryTemplateAfterGeneration(ctx context.Context, schemaName, tenantID, templateID string, nextDate time.Time, generatedAt time.Time) error
}

var (
	errJournalEntryTemplatesUnsupported = errors.New("journal entry templates are not supported by repository")
	ErrTemplateEvidenceAutoPost         = errors.New("cannot auto-post a template entry that requires evidence")
)

// Service provides accounting operations
type Service struct {
	repo RepositoryInterface
}

// NewService creates a new accounting service
func NewService(db *pgxpool.Pool) *Service {
	return &Service{
		repo: NewRepository(db),
	}
}

// NewServiceWithRepository creates a new accounting service with a custom repository.
func NewServiceWithRepository(repo RepositoryInterface) *Service {
	return &Service{
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

// GetAccountHierarchy returns accounts in parent-child chart order.
func (s *Service) GetAccountHierarchy(ctx context.Context, schemaName, tenantID string, activeOnly bool) ([]AccountHierarchyRow, error) {
	accounts, err := s.repo.ListAccounts(ctx, schemaName, tenantID, activeOnly)
	if err != nil {
		return nil, err
	}
	return BuildAccountHierarchy(accounts), nil
}

// BuildAccountHierarchy flattens accounts into deterministic parent-child order.
func BuildAccountHierarchy(accounts []Account) []AccountHierarchyRow {
	accountsByID := make(map[string]Account, len(accounts))
	for _, account := range accounts {
		accountsByID[account.ID] = account
	}

	childrenByParent := make(map[string][]Account)
	var roots []Account
	for _, account := range accounts {
		if account.ParentID != nil {
			if _, ok := accountsByID[*account.ParentID]; ok {
				childrenByParent[*account.ParentID] = append(childrenByParent[*account.ParentID], account)
				continue
			}
		}
		roots = append(roots, account)
	}

	sortAccounts := func(items []Account) {
		sort.SliceStable(items, func(i, j int) bool {
			if items[i].Code != items[j].Code {
				return items[i].Code < items[j].Code
			}
			if items[i].Name != items[j].Name {
				return items[i].Name < items[j].Name
			}
			return items[i].ID < items[j].ID
		})
	}
	sortAccounts(roots)
	for parentID := range childrenByParent {
		sortAccounts(childrenByParent[parentID])
	}

	rows := make([]AccountHierarchyRow, 0, len(accounts))
	visited := make(map[string]bool, len(accounts))
	var walk func(account Account, depth int, parentCode, parentName, parentPath string)
	walk = func(account Account, depth int, parentCode, parentName, parentPath string) {
		if visited[account.ID] {
			return
		}
		visited[account.ID] = true

		path := account.Code
		if strings.TrimSpace(parentPath) != "" {
			path = parentPath + "/" + account.Code
		}
		children := childrenByParent[account.ID]
		rows = append(rows, AccountHierarchyRow{
			Account:     account,
			ParentCode:  parentCode,
			ParentName:  parentName,
			Depth:       depth,
			Path:        path,
			HasChildren: len(children) > 0,
		})
		for _, child := range children {
			walk(child, depth+1, account.Code, account.Name, path)
		}
	}

	for _, root := range roots {
		walk(root, 0, "", "", "")
	}

	sortedAccounts := make([]Account, 0, len(accounts))
	sortedAccounts = append(sortedAccounts, accounts...)
	sortAccounts(sortedAccounts)
	for _, account := range sortedAccounts {
		if !visited[account.ID] {
			walk(account, 0, "", "", "")
		}
	}

	return rows
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
	for i, reqLine := range req.Lines {
		currency := normalizeJournalLineCurrency(reqLine.Currency)
		exchangeRate, err := normalizeJournalLineExchangeRate(reqLine.ExchangeRate)
		if err != nil {
			return nil, fmt.Errorf("validation failed: line %d: %w", i+1, err)
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
		ID:                 uuid.New().String(),
		TenantID:           tenantID,
		Name:               strings.TrimSpace(req.Name),
		Description:        strings.TrimSpace(req.Description),
		Reference:          strings.TrimSpace(req.Reference),
		RequiresEvidence:   req.RequiresEvidence,
		IsActive:           true,
		Frequency:          req.Frequency,
		StartDate:          cloneTemplateDate(req.StartDate),
		EndDate:            cloneTemplateDate(req.EndDate),
		NextGenerationDate: cloneTemplateDate(req.NextGenerationDate),
		CreatedAt:          now,
		CreatedBy:          req.UserID,
		UpdatedAt:          now,
	}
	normalizeJournalEntryTemplateSchedule(template)
	for i, reqLine := range req.Lines {
		line, err := newJournalEntryTemplateLine(template.ID, i+1, reqLine)
		if err != nil {
			return nil, fmt.Errorf("validation failed: %w", err)
		}
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

// GenerateJournalEntryTemplate generates one due recurring journal template and advances its schedule.
func (s *Service) GenerateJournalEntryTemplate(ctx context.Context, schemaName, tenantID, templateID string, req *GenerateJournalEntryTemplateRequest) (*JournalEntryTemplateGenerationResult, error) {
	repo, err := s.templateRepository()
	if err != nil {
		return nil, err
	}

	template, err := s.GetJournalEntryTemplate(ctx, schemaName, tenantID, templateID)
	if err != nil {
		return nil, err
	}
	entryDate, err := recurringTemplateEntryDate(template, req.EntryDate)
	if err != nil {
		return nil, err
	}
	if req.PeriodLockDate != nil && !entryDate.After(*req.PeriodLockDate) {
		return nil, fmt.Errorf("period locked through %s; recurring template date %s must be later", req.PeriodLockDate.Format("2006-01-02"), entryDate.Format("2006-01-02"))
	}

	entry, err := s.ApplyJournalEntryTemplate(ctx, schemaName, tenantID, templateID, &ApplyJournalEntryTemplateRequest{
		EntryDate: entryDate,
		Post:      req.Post,
		UserID:    req.UserID,
	})
	if err != nil {
		return nil, err
	}

	nextDate := normalizeJournalTemplateDate(template.CalculateNextDate(entryDate))
	generatedAt := time.Now()
	if err := repo.UpdateJournalEntryTemplateAfterGeneration(ctx, schemaName, tenantID, templateID, nextDate, generatedAt); err != nil {
		return nil, fmt.Errorf("update recurring journal template: %w", err)
	}

	return &JournalEntryTemplateGenerationResult{
		TemplateID:           template.ID,
		TemplateName:         template.Name,
		GeneratedEntryID:     entry.ID,
		GeneratedEntryNumber: entry.EntryNumber,
		EntryDate:            &entryDate,
		NextGenerationDate:   &nextDate,
		Status:               "generated",
	}, nil
}

// GenerateDueJournalEntryTemplates generates all recurring journal templates due by a date.
func (s *Service) GenerateDueJournalEntryTemplates(ctx context.Context, schemaName, tenantID string, req *GenerateDueJournalEntryTemplatesRequest) ([]JournalEntryTemplateGenerationResult, error) {
	repo, err := s.templateRepository()
	if err != nil {
		return nil, err
	}
	asOfDate := time.Now()
	if req.AsOfDate != nil {
		asOfDate = *req.AsOfDate
	}
	asOfDate = normalizeJournalTemplateDate(asOfDate)

	ids, err := repo.GetDueJournalEntryTemplateIDs(ctx, schemaName, tenantID, asOfDate)
	if err != nil {
		return nil, fmt.Errorf("list due recurring journal templates: %w", err)
	}

	results := make([]JournalEntryTemplateGenerationResult, 0, len(ids))
	for _, id := range ids {
		result, err := s.GenerateJournalEntryTemplate(ctx, schemaName, tenantID, id, &GenerateJournalEntryTemplateRequest{
			Post:           req.Post,
			UserID:         req.UserID,
			PeriodLockDate: req.PeriodLockDate,
		})
		if err != nil {
			results = append(results, JournalEntryTemplateGenerationResult{
				TemplateID: id,
				Status:     "error",
				Error:      err.Error(),
			})
			continue
		}
		results = append(results, *result)
	}
	return results, nil
}

func (s *Service) templateRepository() (JournalEntryTemplateRepository, error) {
	repo, ok := s.repo.(JournalEntryTemplateRepository)
	if !ok {
		return nil, errJournalEntryTemplatesUnsupported
	}
	return repo, nil
}

func newJournalEntryTemplateLine(templateID string, lineNumber int, reqLine CreateJournalEntryLineReq) (JournalEntryTemplateLine, error) {
	exchangeRate, err := normalizeJournalLineExchangeRate(reqLine.ExchangeRate)
	if err != nil {
		return JournalEntryTemplateLine{}, fmt.Errorf("line %d: %w", lineNumber, err)
	}
	return JournalEntryTemplateLine{
		ID:           uuid.New().String(),
		TemplateID:   templateID,
		LineNumber:   lineNumber,
		AccountID:    strings.TrimSpace(reqLine.AccountID),
		Description:  strings.TrimSpace(reqLine.Description),
		DebitAmount:  reqLine.DebitAmount,
		CreditAmount: reqLine.CreditAmount,
		Currency:     normalizeJournalLineCurrency(reqLine.Currency),
		ExchangeRate: exchangeRate,
	}, nil
}

func normalizeJournalLineCurrency(value string) string {
	currency := strings.ToUpper(strings.TrimSpace(value))
	if currency == "" {
		return "EUR"
	}
	return currency
}

func normalizeJournalLineExchangeRate(value decimal.Decimal) (decimal.Decimal, error) {
	if value.IsZero() {
		return decimal.NewFromInt(1), nil
	}
	if value.LessThanOrEqual(decimal.Zero) {
		return decimal.Zero, errors.New("exchange_rate must be positive")
	}
	return value, nil
}

func validateJournalEntryTemplate(template *JournalEntryTemplate) error {
	if err := validateJournalEntryTemplateSchedule(template); err != nil {
		return err
	}
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

func validateJournalEntryTemplateSchedule(template *JournalEntryTemplate) error {
	if template.Frequency == "" {
		return nil
	}
	if !isValidJournalEntryTemplateFrequency(template.Frequency) {
		return errors.New("invalid journal entry template frequency")
	}
	if template.StartDate == nil {
		return errors.New("start_date is required for recurring journal entry templates")
	}
	if template.EndDate != nil && template.EndDate.Before(*template.StartDate) {
		return errors.New("end_date cannot be before start_date")
	}
	if template.NextGenerationDate == nil {
		return errors.New("next_generation_date is required for recurring journal entry templates")
	}
	if template.NextGenerationDate.Before(*template.StartDate) {
		return errors.New("next_generation_date cannot be before start_date")
	}
	return nil
}

func normalizeJournalEntryTemplateSchedule(template *JournalEntryTemplate) {
	template.StartDate = cloneTemplateDate(template.StartDate)
	template.EndDate = cloneTemplateDate(template.EndDate)
	template.NextGenerationDate = cloneTemplateDate(template.NextGenerationDate)
	if template.Frequency != "" && template.NextGenerationDate == nil && template.StartDate != nil {
		next := *template.StartDate
		template.NextGenerationDate = &next
	}
}

func recurringTemplateEntryDate(template *JournalEntryTemplate, requested *time.Time) (time.Time, error) {
	if !template.IsRecurring() {
		return time.Time{}, errors.New("journal entry template is not recurring")
	}
	var entryDate time.Time
	if requested != nil {
		entryDate = *requested
	} else if template.NextGenerationDate != nil {
		entryDate = *template.NextGenerationDate
	} else if template.StartDate != nil {
		entryDate = *template.StartDate
	} else {
		return time.Time{}, errors.New("recurring journal entry template has no next generation date")
	}
	entryDate = normalizeJournalTemplateDate(entryDate)
	if template.EndDate != nil && entryDate.After(*template.EndDate) {
		return time.Time{}, fmt.Errorf("recurring journal entry template ended on %s", template.EndDate.Format("2006-01-02"))
	}
	return entryDate, nil
}

func cloneTemplateDate(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	normalized := normalizeJournalTemplateDate(*value)
	return &normalized
}

func normalizeJournalTemplateDate(value time.Time) time.Time {
	utc := value.UTC()
	return time.Date(utc.Year(), utc.Month(), utc.Day(), 0, 0, 0, 0, time.UTC)
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
