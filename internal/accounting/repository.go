package accounting

import (
	"context"
	"fmt"
	"time"

	"github.com/HMB-research/open-accounting/internal/database"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

// RepositoryInterface defines the contract for accounting data access
type RepositoryInterface interface {
	GetAccountByID(ctx context.Context, schemaName, tenantID, accountID string) (*Account, error)
	ListAccounts(ctx context.Context, schemaName, tenantID string, activeOnly bool) ([]Account, error)
	CreateAccount(ctx context.Context, schemaName string, a *Account) error
	UpdateAccount(ctx context.Context, schemaName string, a *Account) error
	ListJournalEntries(ctx context.Context, schemaName, tenantID string, limit int) ([]JournalEntry, error)
	GetJournalEntryByID(ctx context.Context, schemaName, tenantID, entryID string) (*JournalEntry, error)
	GetJournalEntryBySource(ctx context.Context, schemaName, tenantID, sourceType, sourceID string) (*JournalEntry, error)
	CreateJournalEntry(ctx context.Context, schemaName string, je *JournalEntry) error
	UpdateJournalEntryStatus(ctx context.Context, schemaName, tenantID, entryID string, status JournalEntryStatus, userID string) error
	GetAccountBalance(ctx context.Context, schemaName, tenantID, accountID string, asOfDate time.Time) (decimal.Decimal, error)
	GetTrialBalance(ctx context.Context, schemaName, tenantID string, asOfDate time.Time) ([]AccountBalance, error)
	GetPeriodBalances(ctx context.Context, schemaName, tenantID string, startDate, endDate time.Time) ([]AccountBalance, error)
	VoidJournalEntry(ctx context.Context, schemaName, tenantID, entryID, userID, reason string, reversal *JournalEntry) error
}

// NewRepository creates an ORM-backed accounting repository.
func NewRepository(db *pgxpool.Pool) *GORMRepository {
	if db == nil {
		return &GORMRepository{}
	}
	gormDB, err := database.NewGormDBFromPool(context.Background(), db)
	if err != nil {
		panic(fmt.Errorf("create accounting GORM repository: %w", err))
	}
	return NewGORMRepository(gormDB)
}
