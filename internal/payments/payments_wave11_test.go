package payments

import (
	"context"
	"errors"
	"testing"

	"github.com/HMB-research/open-accounting/internal/contacts"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type wave11PaymentContactLister struct{}

func (wave11PaymentContactLister) List(context.Context, string, string, *contacts.ContactFilter) ([]contacts.Contact, error) {
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
	contactsSvc := wave11PaymentContactLister{}
	var called bool

	stubWave11GormDBFromPool(t, func(ctx context.Context, got *pgxpool.Pool) (*gorm.DB, error) {
		require.NotNil(t, ctx)
		require.Same(t, pool, got)
		called = true
		return expectedDB, nil
	})
	originalContacts := newPaymentsContactService
	newPaymentsContactService = func(*pgxpool.Pool) contactLister {
		return contactsSvc
	}
	t.Cleanup(func() {
		newPaymentsContactService = originalContacts
	})

	service := NewService(pool, nil)

	require.True(t, called)
	require.NotNil(t, service)
	repo, ok := service.repo.(*GORMRepository)
	require.True(t, ok)
	require.Same(t, expectedDB, repo.db)
	require.Equal(t, contactsSvc, service.contacts)
}

func TestWave11DefaultContactConstructorAllowsNilPool(t *testing.T) {
	require.NotNil(t, newPaymentsContactService(nil))
}

func TestWave11NewServicePanicsOnInjectedGormError(t *testing.T) {
	stubWave11GormDBFromPool(t, func(context.Context, *pgxpool.Pool) (*gorm.DB, error) {
		return nil, errors.New("pool unavailable")
	})

	require.PanicsWithError(t, "create payments GORM repository: pool unavailable", func() {
		_ = NewService(new(pgxpool.Pool), nil)
	})
}
