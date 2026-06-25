package contacts

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

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
	var called bool
	stubWave11GormDBFromPool(t, func(ctx context.Context, got *pgxpool.Pool) (*gorm.DB, error) {
		require.NotNil(t, ctx)
		require.Same(t, pool, got)
		called = true
		return expectedDB, nil
	})

	service := NewService(pool)

	require.True(t, called)
	require.NotNil(t, service)
	repo, ok := service.repo.(*GORMRepository)
	require.True(t, ok)
	require.Same(t, expectedDB, repo.db)
}

func TestWave11NewServicePanicsOnInjectedGormError(t *testing.T) {
	stubWave11GormDBFromPool(t, func(context.Context, *pgxpool.Pool) (*gorm.DB, error) {
		return nil, errors.New("pool unavailable")
	})

	require.PanicsWithError(t, "create contacts GORM repository: pool unavailable", func() {
		_ = NewService(new(pgxpool.Pool))
	})
}

func TestWave11CreateRejectsInvalidDefaultAccountID(t *testing.T) {
	defaultAccountID := "not-a-uuid"
	contact, err := NewServiceWithRepository(NewMockRepository()).Create(context.Background(), "tenant-1", "tenant_contacts", &CreateContactRequest{
		Name:             "Supplier",
		ContactType:      ContactTypeSupplier,
		DefaultAccountID: &defaultAccountID,
	})

	require.Nil(t, contact)
	require.Error(t, err)
	require.Contains(t, err.Error(), "default_account_id")
}
