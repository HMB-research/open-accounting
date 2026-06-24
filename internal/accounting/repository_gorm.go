package accounting

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/HMB-research/open-accounting/internal/database"
	"github.com/HMB-research/open-accounting/internal/models"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// GORMRepository implements RepositoryInterface using GORM
type GORMRepository struct {
	db *gorm.DB
}

// NewGORMRepository creates a new GORM accounting repository
func NewGORMRepository(db *gorm.DB) *GORMRepository {
	return &GORMRepository{db: db}
}

func (r *GORMRepository) tenantTable(ctx context.Context, schemaName, tableName string) (*gorm.DB, error) {
	if r.db == nil {
		return nil, fmt.Errorf("accounting repository database is not configured")
	}
	return database.TenantTable(r.db.WithContext(ctx), schemaName, tableName)
}

func (r *GORMRepository) tenantTableAlias(ctx context.Context, schemaName, tableName, alias string) (*gorm.DB, error) {
	if r.db == nil {
		return nil, fmt.Errorf("accounting repository database is not configured")
	}
	qualifiedTable, err := database.QualifiedTable(schemaName, tableName)
	if err != nil {
		return nil, err
	}
	quotedAlias, err := database.QuoteIdentifier(alias)
	if err != nil {
		return nil, err
	}
	return r.db.WithContext(ctx).Table(qualifiedTable + " AS " + quotedAlias), nil
}

// GetAccountByID retrieves an account by ID
func (r *GORMRepository) GetAccountByID(ctx context.Context, schemaName, tenantID, accountID string) (*Account, error) {
	db, err := r.tenantTable(ctx, schemaName, "accounts")
	if err != nil {
		return nil, err
	}

	var account models.Account
	err = db.Where("id = ? AND tenant_id = ?", accountID, tenantID).First(&account).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("account not found: %s", accountID)
	}
	if err != nil {
		return nil, fmt.Errorf("get account: %w", err)
	}

	return modelToAccount(&account), nil
}

// ListAccounts retrieves all accounts for a tenant
func (r *GORMRepository) ListAccounts(ctx context.Context, schemaName, tenantID string, activeOnly bool) ([]Account, error) {
	db, err := r.tenantTable(ctx, schemaName, "accounts")
	if err != nil {
		return nil, err
	}

	query := db.Where("tenant_id = ?", tenantID)
	if activeOnly {
		query = query.Where("is_active = ?", true)
	}
	query = query.Order("code")

	var modelAccounts []models.Account
	if err := query.Find(&modelAccounts).Error; err != nil {
		return nil, fmt.Errorf("list accounts: %w", err)
	}

	accounts := make([]Account, len(modelAccounts))
	for i, ma := range modelAccounts {
		accounts[i] = *modelToAccount(&ma)
	}
	return accounts, nil
}

// CreateAccount creates a new account
func (r *GORMRepository) CreateAccount(ctx context.Context, schemaName string, a *Account) error {
	db, err := r.tenantTable(ctx, schemaName, "accounts")
	if err != nil {
		return err
	}

	if a.ID == "" {
		a.ID = uuid.New().String()
	}
	if a.CreatedAt.IsZero() {
		a.CreatedAt = time.Now()
	}

	account := accountToModel(a)
	if err := db.Select("*").Create(account).Error; err != nil {
		return fmt.Errorf("create account: %w", err)
	}
	return nil
}

// UpdateAccount updates an existing tenant account.
func (r *GORMRepository) UpdateAccount(ctx context.Context, schemaName string, a *Account) error {
	db, err := r.tenantTable(ctx, schemaName, "accounts")
	if err != nil {
		return err
	}

	updates := map[string]interface{}{
		"code":         a.Code,
		"name":         a.Name,
		"account_type": models.AccountType(a.AccountType),
		"parent_id":    a.ParentID,
		"is_active":    a.IsActive,
		"description":  a.Description,
	}
	result := db.Where("id = ? AND tenant_id = ?", a.ID, a.TenantID).Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("update account: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("account not found: %s", a.ID)
	}
	return nil
}

// GetJournalEntryByID retrieves a journal entry with its lines
func (r *GORMRepository) GetJournalEntryByID(ctx context.Context, schemaName, tenantID, entryID string) (*JournalEntry, error) {
	db, err := r.tenantTable(ctx, schemaName, "journal_entries")
	if err != nil {
		return nil, err
	}

	var entry models.JournalEntry
	err = db.Where("id = ? AND tenant_id = ?", entryID, tenantID).First(&entry).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("journal entry not found: %s", entryID)
	}
	if err != nil {
		return nil, fmt.Errorf("get journal entry: %w", err)
	}

	// Load lines
	linesDB, err := r.tenantTable(ctx, schemaName, "journal_entry_lines")
	if err != nil {
		return nil, err
	}
	var lines []models.JournalEntryLine
	if err := linesDB.Where("journal_entry_id = ? AND tenant_id = ?", entryID, tenantID).
		Order("id").
		Find(&lines).Error; err != nil {
		return nil, fmt.Errorf("get journal entry lines: %w", err)
	}

	je := modelToJournalEntry(&entry)
	je.Lines = make([]JournalEntryLine, len(lines))
	for i, line := range lines {
		je.Lines[i] = *modelToJournalEntryLine(&line)
	}

	return je, nil
}

// GetJournalEntryBySource retrieves the most recent non-voided journal entry for a source pair.
func (r *GORMRepository) GetJournalEntryBySource(ctx context.Context, schemaName, tenantID, sourceType, sourceID string) (*JournalEntry, error) {
	db, err := r.tenantTable(ctx, schemaName, "journal_entries")
	if err != nil {
		return nil, err
	}

	var entry models.JournalEntry
	err = db.Where("tenant_id = ? AND source_type = ? AND source_id = ? AND status <> ?", tenantID, sourceType, sourceID, StatusVoided).
		Order("created_at DESC").
		First(&entry).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get journal entry by source: %w", err)
	}

	return r.GetJournalEntryByID(ctx, schemaName, tenantID, entry.ID)
}

// ListJournalEntries retrieves the most recent journal entries with their lines.
func (r *GORMRepository) ListJournalEntries(ctx context.Context, schemaName, tenantID string, limit int) ([]JournalEntry, error) {
	if limit <= 0 {
		limit = 50
	}

	entriesDB, err := r.tenantTable(ctx, schemaName, "journal_entries")
	if err != nil {
		return nil, err
	}

	var entryModels []models.JournalEntry
	if err := entriesDB.
		Where("tenant_id = ?", tenantID).
		Order("entry_date DESC, created_at DESC").
		Limit(limit).
		Find(&entryModels).Error; err != nil {
		return nil, fmt.Errorf("list journal entries: %w", err)
	}

	entries := make([]JournalEntry, 0, len(entryModels))
	entryIDs := make([]string, 0, len(entryModels))
	entryIndex := make(map[string]int, len(entryModels))
	for _, entryModel := range entryModels {
		entry := modelToJournalEntry(&entryModel)
		entryIndex[entry.ID] = len(entries)
		entryIDs = append(entryIDs, entry.ID)
		entries = append(entries, *entry)
	}
	if len(entryIDs) == 0 {
		return entries, nil
	}

	linesDB, err := r.tenantTable(ctx, schemaName, "journal_entry_lines")
	if err != nil {
		return nil, err
	}
	var lineModels []models.JournalEntryLine
	if err := linesDB.
		Where("tenant_id = ? AND journal_entry_id IN ?", tenantID, entryIDs).
		Order("journal_entry_id ASC, id ASC").
		Find(&lineModels).Error; err != nil {
		return nil, fmt.Errorf("list journal entry lines: %w", err)
	}

	for _, lineModel := range lineModels {
		idx, ok := entryIndex[lineModel.JournalEntryID]
		if !ok {
			continue
		}
		entries[idx].Lines = append(entries[idx].Lines, *modelToJournalEntryLine(&lineModel))
	}
	return entries, nil
}

// CreateJournalEntryTemplate creates a reusable balanced journal entry template.
func (r *GORMRepository) CreateJournalEntryTemplate(ctx context.Context, schemaName string, template *JournalEntryTemplate) error {
	if r.db == nil {
		return fmt.Errorf("accounting repository database is not configured")
	}

	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		templatesDB, err := database.TenantTable(tx, schemaName, "journal_entry_templates")
		if err != nil {
			return err
		}
		linesDB, err := database.TenantTable(tx, schemaName, "journal_entry_template_lines")
		if err != nil {
			return err
		}

		if template.ID == "" {
			template.ID = uuid.New().String()
		}
		if template.CreatedAt.IsZero() {
			template.CreatedAt = time.Now()
		}
		if template.UpdatedAt.IsZero() {
			template.UpdatedAt = template.CreatedAt
		}

		if err := templatesDB.Select("*").Create(journalEntryTemplateToModel(template)).Error; err != nil {
			return fmt.Errorf("insert journal entry template: %w", err)
		}

		for i := range template.Lines {
			line := &template.Lines[i]
			if line.ID == "" {
				line.ID = uuid.New().String()
			}
			line.TemplateID = template.ID
			if line.LineNumber == 0 {
				line.LineNumber = i + 1
			}

			if err := linesDB.Select("*").Create(journalEntryTemplateLineToModel(line)).Error; err != nil {
				return fmt.Errorf("insert journal entry template line: %w", err)
			}
		}
		template.LineCount = len(template.Lines)

		return nil
	})
}

// ListJournalEntryTemplates lists reusable journal entry templates for a tenant.
func (r *GORMRepository) ListJournalEntryTemplates(ctx context.Context, schemaName, tenantID string, activeOnly bool) ([]JournalEntryTemplate, error) {
	templatesDB, err := r.tenantTable(ctx, schemaName, "journal_entry_templates")
	if err != nil {
		return nil, err
	}

	query := templatesDB.Where("tenant_id = ?", tenantID)
	if activeOnly {
		query = query.Where("is_active = ?", true)
	}

	var templateModels []models.JournalEntryTemplate
	if err := query.Order("name").Find(&templateModels).Error; err != nil {
		return nil, fmt.Errorf("list journal entry templates: %w", err)
	}

	templateIDs := make([]string, 0, len(templateModels))
	for _, templateModel := range templateModels {
		templateIDs = append(templateIDs, templateModel.ID)
	}
	lineCounts, err := r.countJournalEntryTemplateLines(ctx, schemaName, templateIDs)
	if err != nil {
		return nil, err
	}

	templates := make([]JournalEntryTemplate, len(templateModels))
	for i, templateModel := range templateModels {
		template := modelToJournalEntryTemplate(&templateModel)
		template.LineCount = lineCounts[template.ID]
		templates[i] = *template
	}
	return templates, nil
}

// GetJournalEntryTemplateByID retrieves a reusable journal entry template with lines.
func (r *GORMRepository) GetJournalEntryTemplateByID(ctx context.Context, schemaName, tenantID, templateID string) (*JournalEntryTemplate, error) {
	templatesDB, err := r.tenantTable(ctx, schemaName, "journal_entry_templates")
	if err != nil {
		return nil, err
	}

	var templateModel models.JournalEntryTemplate
	err = templatesDB.Where("id = ? AND tenant_id = ?", templateID, tenantID).First(&templateModel).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("journal entry template not found: %s", templateID)
	}
	if err != nil {
		return nil, fmt.Errorf("get journal entry template: %w", err)
	}

	linesDB, err := r.tenantTable(ctx, schemaName, "journal_entry_template_lines")
	if err != nil {
		return nil, err
	}
	var lineModels []models.JournalEntryTemplateLine
	if err := linesDB.Where("template_id = ?", templateID).Order("line_number").Find(&lineModels).Error; err != nil {
		return nil, fmt.Errorf("get journal entry template lines: %w", err)
	}

	template := modelToJournalEntryTemplate(&templateModel)
	template.Lines = make([]JournalEntryTemplateLine, len(lineModels))
	for i, lineModel := range lineModels {
		template.Lines[i] = *modelToJournalEntryTemplateLine(&lineModel)
	}
	template.LineCount = len(template.Lines)

	return template, nil
}

// GetDueJournalEntryTemplateIDs returns active recurring templates due by a date.
func (r *GORMRepository) GetDueJournalEntryTemplateIDs(ctx context.Context, schemaName, tenantID string, asOfDate time.Time) ([]string, error) {
	templatesDB, err := r.tenantTable(ctx, schemaName, "journal_entry_templates")
	if err != nil {
		return nil, err
	}

	var ids []string
	if err := templatesDB.
		Where("tenant_id = ? AND is_active = ?", tenantID, true).
		Where("COALESCE(frequency, '') <> ''").
		Where("next_generation_date IS NOT NULL").
		Where("next_generation_date <= ?", asOfDate).
		Where("(end_date IS NULL OR next_generation_date <= end_date)").
		Order("next_generation_date").
		Order("name").
		Pluck("id", &ids).Error; err != nil {
		return nil, fmt.Errorf("list due journal entry templates: %w", err)
	}
	return ids, nil
}

// UpdateJournalEntryTemplateAfterGeneration advances recurring template schedule metadata.
func (r *GORMRepository) UpdateJournalEntryTemplateAfterGeneration(ctx context.Context, schemaName, tenantID, templateID string, nextDate time.Time, generatedAt time.Time) error {
	templatesDB, err := r.tenantTable(ctx, schemaName, "journal_entry_templates")
	if err != nil {
		return err
	}

	result := templatesDB.Where("id = ? AND tenant_id = ?", templateID, tenantID).Updates(map[string]interface{}{
		"next_generation_date": nextDate,
		"last_generated_at":    generatedAt,
		"generated_count":      gorm.Expr("COALESCE(generated_count, 0) + 1"),
		"updated_at":           generatedAt,
	})
	if result.Error != nil {
		return fmt.Errorf("update journal entry template after generation: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("journal entry template not found: %s", templateID)
	}
	return nil
}

func (r *GORMRepository) countJournalEntryTemplateLines(ctx context.Context, schemaName string, templateIDs []string) (map[string]int, error) {
	counts := make(map[string]int, len(templateIDs))
	if len(templateIDs) == 0 {
		return counts, nil
	}

	linesDB, err := r.tenantTable(ctx, schemaName, "journal_entry_template_lines")
	if err != nil {
		return nil, err
	}

	var rows []struct {
		TemplateID string
		LineCount  int
	}
	if err := linesDB.
		Select("template_id, COUNT(*) AS line_count").
		Where("template_id IN ?", templateIDs).
		Group("template_id").
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("count journal entry template lines: %w", err)
	}

	for _, row := range rows {
		counts[row.TemplateID] = row.LineCount
	}
	return counts, nil
}

// CreateJournalEntry creates a new journal entry with lines
func (r *GORMRepository) CreateJournalEntry(ctx context.Context, schemaName string, je *JournalEntry) error {
	db, err := r.tenantTable(ctx, schemaName, "journal_entries")
	if err != nil {
		return err
	}

	return db.Transaction(func(tx *gorm.DB) error {
		return r.createJournalEntryInTx(ctx, tx, schemaName, je)
	})
}

// createJournalEntryInTx is an internal method that creates a journal entry within a GORM transaction
func (r *GORMRepository) createJournalEntryInTx(ctx context.Context, tx *gorm.DB, schemaName string, je *JournalEntry) error {
	entriesDB, err := database.TenantTable(tx, schemaName, "journal_entries")
	if err != nil {
		return err
	}
	linesDB, err := database.TenantTable(tx, schemaName, "journal_entry_lines")
	if err != nil {
		return err
	}

	if je.ID == "" {
		je.ID = uuid.New().String()
	}
	if je.CreatedAt.IsZero() {
		je.CreatedAt = time.Now()
	}

	// Generate the next sequence from any trailing digits in existing entry numbers.
	var seq int
	err = entriesDB.
		Select(`
			COALESCE(MAX(
			CASE
				WHEN entry_number ~ '[0-9]+$' THEN CAST(SUBSTRING(entry_number FROM '([0-9]+)$') AS INTEGER)
				ELSE 0
			END
		), 0) + 1
		`).
		Where("tenant_id = ?", je.TenantID).
		Scan(&seq).Error
	if err != nil {
		return fmt.Errorf("generate entry number: %w", err)
	}
	je.EntryNumber = fmt.Sprintf("JE-%05d", seq)

	// Insert entry
	entry := journalEntryToModel(je)
	if err := entriesDB.Select("*").Create(entry).Error; err != nil {
		return fmt.Errorf("insert journal entry: %w", err)
	}

	// Insert lines
	for i := range je.Lines {
		line := &je.Lines[i]
		if line.ID == "" {
			line.ID = uuid.New().String()
		}
		line.TenantID = je.TenantID
		line.JournalEntryID = je.ID

		modelLine := journalEntryLineToModel(line)
		if err := linesDB.Select("*").Create(modelLine).Error; err != nil {
			return fmt.Errorf("insert journal entry line: %w", err)
		}
	}

	return nil
}

// UpdateJournalEntryStatus updates the status of a journal entry
func (r *GORMRepository) UpdateJournalEntryStatus(ctx context.Context, schemaName, tenantID, entryID string, status JournalEntryStatus, userID string) error {
	db, err := r.tenantTable(ctx, schemaName, "journal_entries")
	if err != nil {
		return err
	}

	now := time.Now()

	switch status {
	case StatusPosted:
		result := db.Where("id = ? AND tenant_id = ? AND status = ?", entryID, tenantID, StatusDraft).
			Updates(map[string]interface{}{
				"status":    status,
				"posted_at": now,
				"posted_by": userID,
			})
		if result.Error != nil {
			return fmt.Errorf("update journal entry status: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return fmt.Errorf("entry not found or invalid status transition")
		}
	case StatusVoided:
		return fmt.Errorf("use VoidJournalEntry method to void entries")
	default:
		return fmt.Errorf("invalid status transition to: %s", status)
	}

	return nil
}

// GetAccountBalance retrieves the balance of an account as of a date
func (r *GORMRepository) GetAccountBalance(ctx context.Context, schemaName, tenantID, accountID string, asOfDate time.Time) (decimal.Decimal, error) {
	account, err := r.GetAccountByID(ctx, schemaName, tenantID, accountID)
	if err != nil {
		return decimal.Zero, err
	}

	linesDB, err := r.tenantTableAlias(ctx, schemaName, "journal_entry_lines", "jel")
	if err != nil {
		return decimal.Zero, err
	}
	entriesTable, err := database.QualifiedTable(schemaName, "journal_entries")
	if err != nil {
		return decimal.Zero, err
	}

	var result struct {
		DebitSum  models.Decimal
		CreditSum models.Decimal
	}

	err = linesDB.
		Select("COALESCE(SUM(jel.debit_amount), 0) AS debit_sum, COALESCE(SUM(jel.credit_amount), 0) AS credit_sum").
		Joins(fmt.Sprintf("JOIN %s AS je ON je.id = jel.journal_entry_id", entriesTable)).
		Where("jel.account_id = ? AND jel.tenant_id = ?", accountID, tenantID).
		Where("je.entry_date <= ? AND je.status = ?", asOfDate, StatusPosted).
		Scan(&result).Error
	if err != nil {
		return decimal.Zero, fmt.Errorf("get account balance: %w", err)
	}

	if account.AccountType.IsDebitNormal() {
		return result.DebitSum.Sub(result.CreditSum.Decimal), nil
	}
	return result.CreditSum.Sub(result.DebitSum.Decimal), nil
}

// GetTrialBalance retrieves all account balances as of a date
func (r *GORMRepository) GetTrialBalance(ctx context.Context, schemaName, tenantID string, asOfDate time.Time) ([]AccountBalance, error) {
	accountsDB, err := r.tenantTableAlias(ctx, schemaName, "accounts", "a")
	if err != nil {
		return nil, err
	}
	linesTable, err := database.QualifiedTable(schemaName, "journal_entry_lines")
	if err != nil {
		return nil, err
	}
	entriesTable, err := database.QualifiedTable(schemaName, "journal_entries")
	if err != nil {
		return nil, err
	}

	var results []struct {
		AccountID    string
		AccountCode  string
		AccountName  string
		AccountType  string
		TotalDebits  models.Decimal
		TotalCredits models.Decimal
		NetBalance   models.Decimal
	}

	err = accountsDB.
		Select(`
			a.id AS account_id,
			a.code AS account_code,
			a.name AS account_name,
			a.account_type,
			COALESCE(SUM(jel.debit_amount), 0) AS total_debits,
			COALESCE(SUM(jel.credit_amount), 0) AS total_credits,
			CASE
				WHEN a.account_type IN ('ASSET', 'EXPENSE') THEN COALESCE(SUM(jel.debit_amount), 0) - COALESCE(SUM(jel.credit_amount), 0)
				ELSE COALESCE(SUM(jel.credit_amount), 0) - COALESCE(SUM(jel.debit_amount), 0)
			END AS net_balance
		`).
		Joins(fmt.Sprintf("LEFT JOIN %s AS jel ON jel.account_id = a.id AND jel.tenant_id = a.tenant_id", linesTable)).
		Joins(fmt.Sprintf("LEFT JOIN %s AS je ON je.id = jel.journal_entry_id", entriesTable)).
		Where("a.tenant_id = ?", tenantID).
		Where("(je.id IS NULL OR (je.entry_date <= ? AND je.status = ?))", asOfDate, StatusPosted).
		Group("a.id, a.code, a.name, a.account_type").
		Having("COALESCE(SUM(jel.debit_amount), 0) != 0 OR COALESCE(SUM(jel.credit_amount), 0) != 0").
		Order("a.code").
		Scan(&results).Error
	if err != nil {
		return nil, fmt.Errorf("get trial balance: %w", err)
	}

	balances := make([]AccountBalance, len(results))
	for i, r := range results {
		balances[i] = AccountBalance{
			AccountID:     r.AccountID,
			AccountCode:   r.AccountCode,
			AccountName:   r.AccountName,
			AccountType:   AccountType(r.AccountType),
			DebitBalance:  r.TotalDebits.Decimal,
			CreditBalance: r.TotalCredits.Decimal,
			NetBalance:    r.NetBalance.Decimal,
		}
	}
	return balances, nil
}

// GetPeriodBalances retrieves account activity for a specific period (not cumulative)
func (r *GORMRepository) GetPeriodBalances(ctx context.Context, schemaName, tenantID string, startDate, endDate time.Time) ([]AccountBalance, error) {
	accountsDB, err := r.tenantTableAlias(ctx, schemaName, "accounts", "a")
	if err != nil {
		return nil, err
	}
	linesTable, err := database.QualifiedTable(schemaName, "journal_entry_lines")
	if err != nil {
		return nil, err
	}
	entriesTable, err := database.QualifiedTable(schemaName, "journal_entries")
	if err != nil {
		return nil, err
	}

	var results []struct {
		AccountID    string
		AccountCode  string
		AccountName  string
		AccountType  string
		TotalDebits  models.Decimal
		TotalCredits models.Decimal
		NetBalance   models.Decimal
	}

	err = accountsDB.
		Select(`
			a.id AS account_id,
			a.code AS account_code,
			a.name AS account_name,
			a.account_type,
			COALESCE(SUM(jel.debit_amount), 0) AS total_debits,
			COALESCE(SUM(jel.credit_amount), 0) AS total_credits,
			CASE
				WHEN a.account_type = 'EXPENSE' THEN COALESCE(SUM(jel.debit_amount), 0) - COALESCE(SUM(jel.credit_amount), 0)
				ELSE COALESCE(SUM(jel.credit_amount), 0) - COALESCE(SUM(jel.debit_amount), 0)
			END AS net_balance
		`).
		Joins(fmt.Sprintf("LEFT JOIN %s AS jel ON jel.account_id = a.id AND jel.tenant_id = a.tenant_id", linesTable)).
		Joins(fmt.Sprintf("LEFT JOIN %s AS je ON je.id = jel.journal_entry_id", entriesTable)).
		Where("a.tenant_id = ?", tenantID).
		Where("(je.id IS NULL OR (je.entry_date >= ? AND je.entry_date <= ? AND je.status = ? AND COALESCE(je.source_type, '') NOT IN ?))",
			startDate, endDate, StatusPosted, []string{SourceTypeYearEndCarryForward, SourceTypeYearEndCarryForwardReversal}).
		Where("a.account_type IN ?", []string{string(AccountTypeRevenue), string(AccountTypeExpense)}).
		Group("a.id, a.code, a.name, a.account_type").
		Having("COALESCE(SUM(jel.debit_amount), 0) != 0 OR COALESCE(SUM(jel.credit_amount), 0) != 0").
		Order("a.account_type DESC, a.code").
		Scan(&results).Error
	if err != nil {
		return nil, fmt.Errorf("get period balances: %w", err)
	}

	balances := make([]AccountBalance, len(results))
	for i, r := range results {
		balances[i] = AccountBalance{
			AccountID:     r.AccountID,
			AccountCode:   r.AccountCode,
			AccountName:   r.AccountName,
			AccountType:   AccountType(r.AccountType),
			DebitBalance:  r.TotalDebits.Decimal,
			CreditBalance: r.TotalCredits.Decimal,
			NetBalance:    r.NetBalance.Decimal,
		}
	}
	return balances, nil
}

// VoidJournalEntry voids a journal entry and creates a reversal entry within a transaction
func (r *GORMRepository) VoidJournalEntry(ctx context.Context, schemaName, tenantID, entryID, userID, reason string, reversal *JournalEntry) error {
	db, err := r.tenantTable(ctx, schemaName, "journal_entries")
	if err != nil {
		return err
	}

	return db.Transaction(func(tx *gorm.DB) error {
		now := time.Now()
		entriesDB, err := database.TenantTable(tx, schemaName, "journal_entries")
		if err != nil {
			return err
		}

		// Mark original as voided
		result := entriesDB.Where("id = ? AND tenant_id = ? AND status = ?", entryID, tenantID, StatusPosted).
			Updates(map[string]interface{}{
				"status":      StatusVoided,
				"voided_at":   now,
				"voided_by":   userID,
				"void_reason": reason,
			})
		if result.Error != nil {
			return fmt.Errorf("mark entry as voided: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return fmt.Errorf("entry not found or not in posted status")
		}

		// Create the reversal entry using the transaction
		return r.createJournalEntryInTx(ctx, tx, schemaName, reversal)
	})
}

// Conversion helpers between domain types and GORM models

func modelToAccount(m *models.Account) *Account {
	return &Account{
		ID:          m.ID,
		TenantID:    m.TenantID,
		Code:        m.Code,
		Name:        m.Name,
		AccountType: AccountType(m.AccountType),
		ParentID:    m.ParentID,
		IsActive:    m.IsActive,
		IsSystem:    m.IsSystem,
		Description: m.Description,
		CreatedAt:   m.CreatedAt,
	}
}

func accountToModel(a *Account) *models.Account {
	return &models.Account{
		ID:          a.ID,
		TenantID:    a.TenantID,
		Code:        a.Code,
		Name:        a.Name,
		AccountType: models.AccountType(a.AccountType),
		ParentID:    a.ParentID,
		IsActive:    a.IsActive,
		IsSystem:    a.IsSystem,
		Description: a.Description,
		CreatedAt:   a.CreatedAt,
	}
}

func modelToJournalEntry(m *models.JournalEntry) *JournalEntry {
	return &JournalEntry{
		ID:               m.ID,
		TenantID:         m.TenantID,
		EntryNumber:      m.EntryNumber,
		EntryDate:        m.EntryDate,
		Description:      m.Description,
		Reference:        m.Reference,
		SourceType:       m.SourceType,
		SourceID:         m.SourceID,
		RequiresEvidence: m.RequiresEvidence,
		Status:           JournalEntryStatus(m.Status),
		PostedAt:         m.PostedAt,
		PostedBy:         m.PostedBy,
		VoidedAt:         m.VoidedAt,
		VoidedBy:         m.VoidedBy,
		VoidReason:       m.VoidReason,
		CreatedAt:        m.CreatedAt,
		CreatedBy:        m.CreatedBy,
	}
}

func journalEntryToModel(je *JournalEntry) *models.JournalEntry {
	return &models.JournalEntry{
		ID:               je.ID,
		TenantID:         je.TenantID,
		EntryNumber:      je.EntryNumber,
		EntryDate:        je.EntryDate,
		Description:      je.Description,
		Reference:        je.Reference,
		SourceType:       je.SourceType,
		SourceID:         je.SourceID,
		RequiresEvidence: je.RequiresEvidence,
		Status:           models.JournalEntryStatus(je.Status),
		PostedAt:         je.PostedAt,
		PostedBy:         je.PostedBy,
		VoidedAt:         je.VoidedAt,
		VoidedBy:         je.VoidedBy,
		VoidReason:       je.VoidReason,
		CreatedAt:        je.CreatedAt,
		CreatedBy:        je.CreatedBy,
	}
}

func modelToJournalEntryLine(m *models.JournalEntryLine) *JournalEntryLine {
	return &JournalEntryLine{
		ID:             m.ID,
		TenantID:       m.TenantID,
		JournalEntryID: m.JournalEntryID,
		AccountID:      m.AccountID,
		Description:    m.Description,
		DebitAmount:    m.DebitAmount.Decimal,
		CreditAmount:   m.CreditAmount.Decimal,
		Currency:       m.Currency,
		ExchangeRate:   m.ExchangeRate.Decimal,
		BaseDebit:      m.BaseDebit.Decimal,
		BaseCredit:     m.BaseCredit.Decimal,
		VATRate:        m.VATRate.Decimal,
		IsVATInclusive: m.IsVATInclusive,
	}
}

func journalEntryLineToModel(l *JournalEntryLine) *models.JournalEntryLine {
	return &models.JournalEntryLine{
		ID:             l.ID,
		TenantID:       l.TenantID,
		JournalEntryID: l.JournalEntryID,
		AccountID:      l.AccountID,
		Description:    l.Description,
		DebitAmount:    models.Decimal{Decimal: l.DebitAmount},
		CreditAmount:   models.Decimal{Decimal: l.CreditAmount},
		Currency:       l.Currency,
		ExchangeRate:   models.Decimal{Decimal: l.ExchangeRate},
		BaseDebit:      models.Decimal{Decimal: l.BaseDebit},
		BaseCredit:     models.Decimal{Decimal: l.BaseCredit},
		VATRate:        models.Decimal{Decimal: l.VATRate},
		IsVATInclusive: l.IsVATInclusive,
	}
}

func modelToJournalEntryTemplate(m *models.JournalEntryTemplate) *JournalEntryTemplate {
	return &JournalEntryTemplate{
		ID:                 m.ID,
		TenantID:           m.TenantID,
		Name:               m.Name,
		Description:        m.Description,
		Reference:          m.Reference,
		RequiresEvidence:   m.RequiresEvidence,
		IsActive:           m.IsActive,
		Frequency:          JournalEntryTemplateFrequency(m.Frequency),
		StartDate:          m.StartDate,
		EndDate:            m.EndDate,
		NextGenerationDate: m.NextGenerationDate,
		LastGeneratedAt:    m.LastGeneratedAt,
		GeneratedCount:     m.GeneratedCount,
		CreatedAt:          m.CreatedAt,
		CreatedBy:          m.CreatedBy,
		UpdatedAt:          m.UpdatedAt,
	}
}

func journalEntryTemplateToModel(t *JournalEntryTemplate) *models.JournalEntryTemplate {
	return &models.JournalEntryTemplate{
		ID:                 t.ID,
		TenantID:           t.TenantID,
		Name:               t.Name,
		Description:        t.Description,
		Reference:          t.Reference,
		RequiresEvidence:   t.RequiresEvidence,
		IsActive:           t.IsActive,
		Frequency:          string(t.Frequency),
		StartDate:          t.StartDate,
		EndDate:            t.EndDate,
		NextGenerationDate: t.NextGenerationDate,
		LastGeneratedAt:    t.LastGeneratedAt,
		GeneratedCount:     t.GeneratedCount,
		CreatedAt:          t.CreatedAt,
		CreatedBy:          t.CreatedBy,
		UpdatedAt:          t.UpdatedAt,
	}
}

func modelToJournalEntryTemplateLine(m *models.JournalEntryTemplateLine) *JournalEntryTemplateLine {
	return &JournalEntryTemplateLine{
		ID:           m.ID,
		TemplateID:   m.TemplateID,
		LineNumber:   m.LineNumber,
		AccountID:    m.AccountID,
		Description:  m.Description,
		DebitAmount:  m.DebitAmount.Decimal,
		CreditAmount: m.CreditAmount.Decimal,
		Currency:     m.Currency,
		ExchangeRate: m.ExchangeRate.Decimal,
	}
}

func journalEntryTemplateLineToModel(l *JournalEntryTemplateLine) *models.JournalEntryTemplateLine {
	return &models.JournalEntryTemplateLine{
		ID:           l.ID,
		TemplateID:   l.TemplateID,
		LineNumber:   l.LineNumber,
		AccountID:    l.AccountID,
		Description:  l.Description,
		DebitAmount:  models.Decimal{Decimal: l.DebitAmount},
		CreditAmount: models.Decimal{Decimal: l.CreditAmount},
		Currency:     l.Currency,
		ExchangeRate: models.Decimal{Decimal: l.ExchangeRate},
	}
}
