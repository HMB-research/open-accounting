package banking

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func TestBankingWave9ConstructorPanicsForUnreachablePool(t *testing.T) {
	pool := bankingWave9UnreachablePool(t)
	defer pool.Close()

	require.Panics(t, func() {
		_ = NewService(pool)
	})
}

func TestBankingWave9MatchRuleUpdateEdges(t *testing.T) {
	ctx := context.Background()

	t.Run("nil request", func(t *testing.T) {
		_, err := NewServiceWithRepository(NewMockRepository()).UpdateBankMatchRule(ctx, testSchemaName, testTenantID, "rule-1", nil)
		require.ErrorContains(t, err, "bank match rule request is required")
	})

	t.Run("get error", func(t *testing.T) {
		repo := NewMockRepository()
		repo.GetBankMatchRuleFn = func(context.Context, string, string, string) (*BankMatchRule, error) {
			return nil, errors.New("lookup failed")
		}
		_, err := NewServiceWithRepository(repo).UpdateBankMatchRule(ctx, testSchemaName, testTenantID, "rule-1", &UpdateBankMatchRuleRequest{})
		require.ErrorContains(t, err, "lookup failed")
	})

	t.Run("update error", func(t *testing.T) {
		repo := NewMockRepository()
		repo.matchRules["rule-1"] = &BankMatchRule{
			ID:              "rule-1",
			TenantID:        testTenantID,
			Name:            "Rule",
			MatchField:      BankMatchFieldDescription,
			Pattern:         "INV",
			MinConfidence:   0.8,
			MaxDateDiffDays: 2,
			IsActive:        true,
		}
		repo.UpdateBankMatchRuleFn = func(context.Context, string, *BankMatchRule) error {
			return errors.New("update failed")
		}
		name := "Updated"
		_, err := NewServiceWithRepository(repo).UpdateBankMatchRule(ctx, testSchemaName, testTenantID, "rule-1", &UpdateBankMatchRuleRequest{Name: &name})
		require.ErrorContains(t, err, "update failed")
	})

	t.Run("blank bank account id clears optional account", func(t *testing.T) {
		blank := " "
		accountID, err := NewServiceWithRepository(NewMockRepository()).normalizeBankMatchRuleAccount(ctx, testSchemaName, testTenantID, &blank)
		require.NoError(t, err)
		require.Nil(t, accountID)
	})
}

func bankingWave9UnreachablePool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	config, err := pgxpool.ParseConfig("postgres://open_accounting:open_accounting@127.0.0.1:1/open_accounting?sslmode=disable")
	require.NoError(t, err)
	config.ConnConfig.ConnectTimeout = 10 * time.Millisecond
	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	require.NoError(t, err)
	return pool
}
