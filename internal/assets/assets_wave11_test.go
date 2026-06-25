package assets

import (
	"context"
	"errors"
	"testing"

	"github.com/HMB-research/open-accounting/internal/accounting"
	"github.com/HMB-research/open-accounting/internal/contacts"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type wave11AssetContactLister struct{}

func (wave11AssetContactLister) List(context.Context, string, string, *contacts.ContactFilter) ([]contacts.Contact, error) {
	return nil, nil
}

type wave11AssetInvoiceResolver struct{}

func (wave11AssetInvoiceResolver) ResolveInvoiceIDByNumber(context.Context, string, string, string) (string, error) {
	return "", nil
}

func stubWave11GormDBFromPool(t *testing.T, fn func(context.Context, *pgxpool.Pool) (*gorm.DB, error)) {
	t.Helper()
	original := newGormDBFromPool
	newGormDBFromPool = fn
	t.Cleanup(func() {
		newGormDBFromPool = original
	})
}

func TestWave11NewRepositoryUsesInjectedGormDB(t *testing.T) {
	expectedDB := &gorm.DB{}
	pool := new(pgxpool.Pool)
	var called bool
	stubWave11GormDBFromPool(t, func(ctx context.Context, got *pgxpool.Pool) (*gorm.DB, error) {
		require.NotNil(t, ctx)
		require.Same(t, pool, got)
		called = true
		return expectedDB, nil
	})

	repo := NewRepository(pool)

	require.True(t, called)
	require.NotNil(t, repo)
	require.Same(t, expectedDB, repo.db)
}

func TestWave11NewRepositoryPanicsOnInjectedGormError(t *testing.T) {
	stubWave11GormDBFromPool(t, func(context.Context, *pgxpool.Pool) (*gorm.DB, error) {
		return nil, errors.New("pool unavailable")
	})

	require.PanicsWithError(t, "create assets GORM repository: pool unavailable", func() {
		_ = NewRepository(new(pgxpool.Pool))
	})
}

func TestWave11NewServiceWithPoolWiresInvoicing(t *testing.T) {
	expectedDB := &gorm.DB{}
	pool := new(pgxpool.Pool)
	ledger := accounting.NewServiceWithRepository(nil)
	contactsSvc := wave11AssetContactLister{}
	invoicingSvc := wave11AssetInvoiceResolver{}

	stubWave11GormDBFromPool(t, func(context.Context, *pgxpool.Pool) (*gorm.DB, error) {
		return expectedDB, nil
	})
	originalAccounting := newAssetsAccountingService
	originalContacts := newAssetsContactsService
	originalInvoicing := newAssetsInvoicingService
	newAssetsAccountingService = func(*pgxpool.Pool) *accounting.Service { return ledger }
	newAssetsContactsService = func(*pgxpool.Pool) contactLister { return contactsSvc }
	newAssetsInvoicingService = func(gotPool *pgxpool.Pool, gotLedger *accounting.Service) assetInvoiceResolver {
		require.Same(t, pool, gotPool)
		require.Same(t, ledger, gotLedger)
		return invoicingSvc
	}
	t.Cleanup(func() {
		newAssetsAccountingService = originalAccounting
		newAssetsContactsService = originalContacts
		newAssetsInvoicingService = originalInvoicing
	})

	service := NewService(pool)

	require.NotNil(t, service)
	repo, ok := service.repo.(*GORMRepository)
	require.True(t, ok)
	require.Same(t, expectedDB, repo.db)
	require.Same(t, ledger, service.ledger)
	require.Equal(t, contactsSvc, service.contacts)
	require.Equal(t, invoicingSvc, service.invoicing)
}

func TestWave11DefaultDependencyConstructorsAllowNilPool(t *testing.T) {
	ledger := newAssetsAccountingService(nil)
	require.NotNil(t, ledger)
	require.NotNil(t, newAssetsContactsService(nil))
	require.NotNil(t, newAssetsInvoicingService(nil, ledger))
}

func TestWave11GORMRepositoryGenerateNumberSuccess(t *testing.T) {
	original := scanAssetNumberSequence
	scanAssetNumberSequence = func(_ *gorm.DB, tenantID string, seq *int) error {
		require.Equal(t, "tenant-1", tenantID)
		*seq = 7
		return nil
	}
	t.Cleanup(func() {
		scanAssetNumberSequence = original
	})

	got, err := NewGORMRepository(newAssetDryRunDB(t)).GenerateNumber(context.Background(), "tenant_assets", "tenant-1")

	require.NoError(t, err)
	require.Equal(t, "FA-00007", got)
}
