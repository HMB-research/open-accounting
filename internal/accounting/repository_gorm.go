//go:build gorm

package accounting

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/HMB-research/open-accounting/internal/database"
	"github.com/HMB-research/open-accounting/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
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
	return database.TenantTable(r.db.WithContext(ctx), schemaName, tableName)
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
	if err := db.Create(account).Error; err != nil {
		return fmt.Errorf("create account: %w", err)
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

// CreateJournalEntryTx creates a journal entry within a pgx transaction
// NOTE: This method is not supported in GORM implementation as it requires pgx.Tx.
// Use CreateJournalEntry instead which handles transactions internally.
func (r *GORMRepository) CreateJournalEntryTx(ctx context.Context, schemaName string, tx pgx.Tx, je *JournalEntry) error {
	return fmt.Errorf("CreateJournalEntryTx is not supported in GORM implementation; use CreateJournalEntry instead")
}

// createJournalEntryInTx is an internal method that creates a journal entry within a GORM transaction
func (r *GORMRepository) createJournalEntryInTx(ctx context.Context, tx *gorm.DB, schemaName string, je *JournalEntry) error {
	entriesTable, err := database.QualifiedTable(schemaName, "journal_entries")
	if err != nil {
		return err
	}
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
	err = tx.Raw(fmt.Sprintf(`
		SELECT COALESCE(MAX(
			CASE
				WHEN entry_number ~ '[0-9]+$' THEN CAST(SUBSTRING(entry_number FROM '([0-9]+)$') AS INTEGER)
				ELSE 0
			END
		), 0) + 1
		FROM %s WHERE tenant_id = ?
	`, entriesTable), je.TenantID).Scan(&seq).Error
	if err != nil {
		return fmt.Errorf("generate entry number: %w", err)
	}
	je.EntryNumber = fmt.Sprintf("JE-%05d", seq)

	// Insert entry
	entry := journalEntryToModel(je)
	if err := entriesDB.Create(entry).Error; err != nil {
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
		if err := linesDB.Create(modelLine).Error; err != nil {
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
	linesTable, err := database.QualifiedTable(schemaName, "journal_entry_lines")
	if err != nil {
		return decimal.Zero, err
	}
	entriesTable, err := database.QualifiedTable(schemaName, "journal_entries")
	if err != nil {
		return decimal.Zero, err
	}
	accountsTable, err := database.QualifiedTable(schemaName, "accounts")
	if err != nil {
		return decimal.Zero, err
	}

	var result struct {
		DebitSum  models.Decimal
		CreditSum models.Decimal
	}

	err = r.db.WithContext(ctx).Raw(fmt.Sprintf(`
		SELECT COALESCE(SUM(jel.debit_amount), 0) as debit_sum, COALESCE(SUM(jel.credit_amount), 0) as credit_sum
		FROM %s jel
		JOIN %s je ON je.id = jel.journal_entry_id
		WHERE jel.account_id = ? AND jel.tenant_id = ?
		  AND je.entry_date <= ? AND je.status = 'POSTED'
	`, linesTable, entriesTable), accountID, tenantID, asOfDate).Scan(&result).Error
	if err != nil {
		return decimal.Zero, fmt.Errorf("get account balance: %w", err)
	}

	// Get account type to determine normal balance
	var accountType string
	err = r.db.WithContext(ctx).Raw(fmt.Sprintf(`SELECT account_type FROM %s WHERE id = ? AND tenant_id = ?`, accountsTable), accountID, tenantID).Scan(&accountType).Error
	if err != nil {
		return decimal.Zero, fmt.Errorf("get account type: %w", err)
	}

	at := AccountType(accountType)
	if at.IsDebitNormal() {
		return result.DebitSum.Decimal.Sub(result.CreditSum.Decimal), nil
	}
	return result.CreditSum.Decimal.Sub(result.DebitSum.Decimal), nil
}

// GetTrialBalance retrieves all account balances as of a date
func (r *GORMRepository) GetTrialBalance(ctx context.Context, schemaName, tenantID string, asOfDate time.Time) ([]AccountBalance, error) {
	accountsTable, err := database.QualifiedTable(schemaName, "accounts")
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

	err = r.db.WithContext(ctx).Raw(fmt.Sprintf(`
		WITH account_totals AS (
			SELECT
				a.id AS account_id,
				a.code AS account_code,
				a.name AS account_name,
				a.account_type,
				COALESCE(SUM(jel.debit_amount), 0) AS total_debits,
				COALESCE(SUM(jel.credit_amount), 0) AS total_credits
			FROM %s a
			LEFT JOIN %s jel ON jel.account_id = a.id AND jel.tenant_id = a.tenant_id
			LEFT JOIN %s je ON je.id = jel.journal_entry_id
			WHERE a.tenant_id = ?
			  AND (je.id IS NULL OR (je.entry_date <= ? AND je.status = 'POSTED'))
			GROUP BY a.id, a.code, a.name, a.account_type
		)
		SELECT
			account_id,
			account_code,
			account_name,
			account_type,
			total_debits,
			total_credits,
			CASE
				WHEN account_type IN ('ASSET', 'EXPENSE') THEN total_debits - total_credits
				ELSE total_credits - total_debits
			END AS net_balance
		FROM account_totals
		WHERE total_debits != 0 OR total_credits != 0
		ORDER BY account_code
	`, accountsTable, linesTable, entriesTable), tenantID, asOfDate).Scan(&results).Error
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
	accountsTable, err := database.QualifiedTable(schemaName, "accounts")
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

	err = r.db.WithContext(ctx).Raw(fmt.Sprintf(`
		WITH period_totals AS (
			SELECT
				a.id AS account_id,
				a.code AS account_code,
				a.name AS account_name,
				a.account_type,
				COALESCE(SUM(jel.debit_amount), 0) AS total_debits,
				COALESCE(SUM(jel.credit_amount), 0) AS total_credits
			FROM %s a
			LEFT JOIN %s jel ON jel.account_id = a.id AND jel.tenant_id = a.tenant_id
			LEFT JOIN %s je ON je.id = jel.journal_entry_id
			WHERE a.tenant_id = ?
			  AND (je.id IS NULL OR (je.entry_date >= ? AND je.entry_date <= ? AND je.status = 'POSTED' AND COALESCE(je.source_type, '') != ?))
			  AND a.account_type IN ('REVENUE', 'EXPENSE')
			GROUP BY a.id, a.code, a.name, a.account_type
		)
		SELECT
			account_id,
			account_code,
			account_name,
			account_type,
			total_debits,
			total_credits,
			CASE
				WHEN account_type = 'EXPENSE' THEN total_debits - total_credits
				ELSE total_credits - total_debits
			END AS net_balance
		FROM period_totals
		WHERE total_debits != 0 OR total_credits != 0
		ORDER BY account_type DESC, account_code
	`, accountsTable, linesTable, entriesTable), tenantID, startDate, endDate, SourceTypeYearEndCarryForward).Scan(&results).Error
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
		ID:          m.ID,
		TenantID:    m.TenantID,
		EntryNumber: m.EntryNumber,
		EntryDate:   m.EntryDate,
		Description: m.Description,
		Reference:   m.Reference,
		SourceType:  m.SourceType,
		SourceID:    m.SourceID,
		Status:      JournalEntryStatus(m.Status),
		PostedAt:    m.PostedAt,
		PostedBy:    m.PostedBy,
		VoidedAt:    m.VoidedAt,
		VoidedBy:    m.VoidedBy,
		VoidReason:  m.VoidReason,
		CreatedAt:   m.CreatedAt,
		CreatedBy:   m.CreatedBy,
	}
}

func journalEntryToModel(je *JournalEntry) *models.JournalEntry {
	return &models.JournalEntry{
		ID:          je.ID,
		TenantID:    je.TenantID,
		EntryNumber: je.EntryNumber,
		EntryDate:   je.EntryDate,
		Description: je.Description,
		Reference:   je.Reference,
		SourceType:  je.SourceType,
		SourceID:    je.SourceID,
		Status:      models.JournalEntryStatus(je.Status),
		PostedAt:    je.PostedAt,
		PostedBy:    je.PostedBy,
		VoidedAt:    je.VoidedAt,
		VoidedBy:    je.VoidedBy,
		VoidReason:  je.VoidReason,
		CreatedAt:   je.CreatedAt,
		CreatedBy:   je.CreatedBy,
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
	}
}
