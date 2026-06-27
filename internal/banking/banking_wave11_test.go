package banking

import (
	"context"
	"errors"
	"testing"

	"github.com/HMB-research/open-accounting/internal/accounting"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type wave11AccountLister struct{}

func (wave11AccountLister) ListAccounts(context.Context, string, string, bool) ([]accounting.Account, error) {
	return nil, nil
}

func stubWave11GormDBFromPool(t *testing.T, fn func(context.Context, *pgxpool.Pool) (*gorm.DB, error)) {
	t.Helper()
	original := newGormDBFromPool
	newGormDBFromPool = fn
	t.Cleanup(func() {
		newGormDBFromPool = original
	})
}

func TestWave11NewServiceUsesInjectedGormDB(t *testing.T) {
	expectedDB := &gorm.DB{}
	pool := new(pgxpool.Pool)
	accounts := wave11AccountLister{}
	var called bool

	stubWave11GormDBFromPool(t, func(ctx context.Context, got *pgxpool.Pool) (*gorm.DB, error) {
		require.NotNil(t, ctx)
		require.Same(t, pool, got)
		called = true
		return expectedDB, nil
	})
	originalAccounts := newBankingAccountService
	newBankingAccountService = func(*pgxpool.Pool) accountingLister {
		return accounts
	}
	t.Cleanup(func() {
		newBankingAccountService = originalAccounts
	})

	service := NewService(pool)

	require.True(t, called)
	require.NotNil(t, service)
	repo, ok := service.repo.(*GORMRepository)
	require.True(t, ok)
	require.Same(t, expectedDB, repo.db)
	require.Equal(t, accounts, service.accounts)
}

func TestWave11DefaultAccountConstructorAllowsNilPool(t *testing.T) {
	require.NotNil(t, newBankingAccountService(nil))
}

func TestWave11NewServicePanicsOnInjectedGormError(t *testing.T) {
	stubWave11GormDBFromPool(t, func(context.Context, *pgxpool.Pool) (*gorm.DB, error) {
		return nil, errors.New("pool unavailable")
	})

	require.PanicsWithError(t, "create banking GORM repository: pool unavailable", func() {
		_ = NewService(new(pgxpool.Pool))
	})
}

func TestWave11BankRemediationFallbacks(t *testing.T) {
	actions := BuildBankRemediationActions(&BankTransaction{
		FollowUpStatus: FollowUpEvidenceRequired,
		Status:         StatusUnmatched,
		Amount:         decimal.NewFromInt(-25),
	})
	require.NotEmpty(t, actions)
	require.Contains(t, actions[0].CLICommand, "<transaction-id>")
	require.Equal(t, "/banking", actions[0].UIPath)

	actions = BuildBankRemediationActions(&BankTransaction{
		ID:     "tx-1",
		Status: StatusMatched,
		Amount: decimal.NewFromInt(25),
	})
	require.NotEmpty(t, actions)
	require.Contains(t, actions[len(actions)-1].CLICommand, "<bank-account-id>")
}
