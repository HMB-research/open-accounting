package tenant

import (
	"context"
	"database/sql/driver"
	"errors"
	"testing"

	"gorm.io/gorm"
)

func TestGORMRepositoryWave4GetUserRoleBranches(t *testing.T) {
	ctx := context.Background()

	t.Run("returns active role", func(t *testing.T) {
		repo := NewGORMRepository(newTenantDryRunDB(t, withTenantDryRunScanRows(tenantDryRunRowSet{
			columns: []string{"role"},
			values:  [][]driver.Value{{RoleAdmin}},
		})))

		role, err := repo.GetUserRole(ctx, "tenant-1", "user-1")
		if err != nil {
			t.Fatalf("GetUserRole() error = %v", err)
		}
		if role != RoleAdmin {
			t.Fatalf("GetUserRole() role = %q, want %q", role, RoleAdmin)
		}
	})

	t.Run("empty role maps to not in tenant", func(t *testing.T) {
		repo := NewGORMRepository(newTenantDryRunDB(t, withTenantDryRunScanRows(tenantDryRunRowSet{
			columns: []string{"role"},
			values:  nil,
		})))

		role, err := repo.GetUserRole(ctx, "tenant-1", "user-1")
		if !errors.Is(err, ErrUserNotInTenant) {
			t.Fatalf("GetUserRole() error = %v, want ErrUserNotInTenant", err)
		}
		if role != "" {
			t.Fatalf("GetUserRole() role = %q, want empty", role)
		}
	})

	t.Run("wraps query errors", func(t *testing.T) {
		expectedErr := errors.New("role query failed")
		repo := NewGORMRepository(newTenantDryRunDB(t, withTenantDryRunRowErrorWave4(expectedErr)))

		role, err := repo.GetUserRole(ctx, "tenant-1", "user-1")
		if !errors.Is(err, expectedErr) {
			t.Fatalf("GetUserRole() error = %v, want %v", err, expectedErr)
		}
		if role != "" {
			t.Fatalf("GetUserRole() role = %q, want empty", role)
		}
	})
}

func withTenantDryRunRowErrorWave4(expectedErr error) tenantDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		err := db.Callback().Row().Before("gorm:row").Register(tenantDryRunCallbackName("wave4_row_error"), func(tx *gorm.DB) {
			tx.AddError(expectedErr)
		})
		if err != nil {
			t.Fatalf("register row error callback: %v", err)
		}
	}
}
