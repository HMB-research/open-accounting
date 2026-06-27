package expenses

import (
	"context"
	"errors"
	"testing"

	"github.com/HMB-research/open-accounting/internal/payroll"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type wave11EmployeeLister struct{}

func (wave11EmployeeLister) ListEmployees(context.Context, string, string, bool) ([]payroll.Employee, error) {
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

	require.PanicsWithError(t, "create expenses GORM repository: pool unavailable", func() {
		_ = NewRepository(new(pgxpool.Pool))
	})
}

func TestWave11NewServiceWithPoolWiresEmployeeLister(t *testing.T) {
	expectedDB := &gorm.DB{}
	pool := new(pgxpool.Pool)
	employees := wave11EmployeeLister{}

	stubWave11GormDBFromPool(t, func(context.Context, *pgxpool.Pool) (*gorm.DB, error) {
		return expectedDB, nil
	})
	originalAccounting := newExpenseAccountingService
	originalContacts := newExpenseContactService
	originalPayroll := newExpensePayrollService
	newExpenseAccountingService = func(*pgxpool.Pool) accountingPoster { return nil }
	newExpenseContactService = func(*pgxpool.Pool) contactLister { return nil }
	newExpensePayrollService = func(*pgxpool.Pool) employeeLister { return employees }
	t.Cleanup(func() {
		newExpenseAccountingService = originalAccounting
		newExpenseContactService = originalContacts
		newExpensePayrollService = originalPayroll
	})

	service := NewService(pool, nil)

	require.NotNil(t, service)
	repo, ok := service.repo.(*GORMRepository)
	require.True(t, ok)
	require.Same(t, expectedDB, repo.db)
	require.Equal(t, employees, service.employees)
}

func TestWave11DefaultDependencyConstructorsAllowNilPool(t *testing.T) {
	require.NotNil(t, newExpenseAccountingService(nil))
	require.NotNil(t, newExpenseContactService(nil))
	require.NotNil(t, newExpensePayrollService(nil))
}
