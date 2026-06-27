package scheduler

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestTenantModelTableName(t *testing.T) {
	var model tenantModel
	if got := model.TableName(); got != "tenants" {
		t.Fatalf("TableName() = %q, want %q", got, "tenants")
	}
}

func TestNewGORMRepository(t *testing.T) {
	repo := NewGORMRepository(nil)
	if repo == nil {
		t.Fatal("NewGORMRepository() returned nil")
	}
	if repo.db != nil {
		t.Fatalf("NewGORMRepository(nil).db = %#v, want nil", repo.db)
	}
}

func TestGORMRepository_ListActiveTenantsNilDatabase(t *testing.T) {
	repo := NewGORMRepository(nil)

	tenants, err := repo.ListActiveTenants(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if tenants != nil {
		t.Fatalf("ListActiveTenants() tenants = %#v, want nil", tenants)
	}
	if got := err.Error(); got != "scheduler repository database is not configured" {
		t.Fatalf("error = %q, want scheduler repository database is not configured", got)
	}
}

func TestGORMRepository_ListActiveTenantsDryRun(t *testing.T) {
	repo := NewGORMRepository(newSchedulerDryRunDB(t, withSchedulerDryRunTenants([]tenantModel{
		{
			ID:         "tenant-1",
			SchemaName: "tenant_1",
			Name:       "Tenant One",
			Settings:   []byte(`{"email":"ops@example.com","period_lock_date":"2026-05-31"}`),
			IsActive:   true,
		},
	})))

	tenants, err := repo.ListActiveTenants(context.Background())
	if err != nil {
		t.Fatalf("ListActiveTenants() error = %v", err)
	}
	if len(tenants) != 1 {
		t.Fatalf("len(tenants) = %d, want 1", len(tenants))
	}
	if tenants[0].ID != "tenant-1" || tenants[0].SchemaName != "tenant_1" || tenants[0].CompanyName != "Tenant One" {
		t.Fatalf("unexpected tenant: %+v", tenants[0])
	}
	if tenants[0].Email != "ops@example.com" || tenants[0].PeriodLockDate != "2026-05-31" {
		t.Fatalf("unexpected tenant settings projection: %+v", tenants[0])
	}
}

func TestGORMRepository_ListActiveTenantsQueryError(t *testing.T) {
	repo := NewGORMRepository(newSchedulerDryRunDB(t, withSchedulerDryRunQueryError(errors.New("query failed"))))

	tenants, err := repo.ListActiveTenants(context.Background())
	if err == nil {
		t.Fatal("ListActiveTenants() error = nil, want query error")
	}
	if tenants != nil {
		t.Fatalf("tenants = %#v, want nil", tenants)
	}
	if got := err.Error(); got != "list active tenants: query failed" {
		t.Fatalf("error = %q, want wrapped query error", got)
	}
}

func TestStringValueFromSettings(t *testing.T) {
	tests := []struct {
		name     string
		settings []byte
		key      string
		want     string
	}{
		{
			name:     "empty settings",
			settings: nil,
			key:      "period_lock_date",
			want:     "",
		},
		{
			name:     "invalid json",
			settings: []byte(`{"period_lock_date":`),
			key:      "period_lock_date",
			want:     "",
		},
		{
			name:     "missing period lock date",
			settings: []byte(`{"locale":"et-EE"}`),
			key:      "period_lock_date",
			want:     "",
		},
		{
			name:     "non-string period lock date",
			settings: []byte(`{"period_lock_date":20260531}`),
			key:      "period_lock_date",
			want:     "",
		},
		{
			name:     "valid period lock date",
			settings: []byte(`{"period_lock_date":"2026-05-31","locale":"et-EE"}`),
			key:      "period_lock_date",
			want:     "2026-05-31",
		},
		{
			name:     "valid email",
			settings: []byte(`{"email":"accounting@example.com","period_lock_date":"2026-05-31"}`),
			key:      "email",
			want:     "accounting@example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := stringValueFromSettings(tt.settings, tt.key); got != tt.want {
				t.Fatalf("stringValueFromSettings() = %q, want %q", got, tt.want)
			}
		})
	}
}

type schedulerDryRunConnPool struct{}

func (schedulerDryRunConnPool) PrepareContext(context.Context, string) (*sql.Stmt, error) {
	return nil, errors.New("dry-run tests should not prepare statements")
}

func (schedulerDryRunConnPool) ExecContext(context.Context, string, ...interface{}) (sql.Result, error) {
	return nil, errors.New("dry-run tests should not execute statements")
}

func (schedulerDryRunConnPool) QueryContext(context.Context, string, ...interface{}) (*sql.Rows, error) {
	return nil, errors.New("dry-run tests should not query rows")
}

func (schedulerDryRunConnPool) QueryRowContext(context.Context, string, ...interface{}) *sql.Row {
	return nil
}

func (schedulerDryRunConnPool) BeginTx(context.Context, *sql.TxOptions) (gorm.ConnPool, error) {
	return &schedulerDryRunTx{}, nil
}

type schedulerDryRunTx struct {
	schedulerDryRunConnPool
}

func (*schedulerDryRunTx) Commit() error {
	return nil
}

func (*schedulerDryRunTx) Rollback() error {
	return nil
}

type schedulerDryRunDBOption func(*gorm.DB)

func newSchedulerDryRunDB(t *testing.T, opts ...schedulerDryRunDBOption) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(postgres.New(postgres.Config{Conn: schedulerDryRunConnPool{}}), &gorm.Config{
		DisableAutomaticPing:   true,
		DryRun:                 true,
		Logger:                 logger.Default.LogMode(logger.Silent),
		SkipDefaultTransaction: true,
	})
	if err != nil {
		t.Fatalf("open dry-run gorm database: %v", err)
	}
	for _, opt := range opts {
		opt(db)
	}
	return db
}

func withSchedulerDryRunTenants(tenants []tenantModel) schedulerDryRunDBOption {
	return func(db *gorm.DB) {
		if err := db.Callback().Query().After("gorm:query").Register("scheduler_test:tenant_fixtures", func(tx *gorm.DB) {
			if dest, ok := tx.Statement.Dest.(*[]tenantModel); ok {
				*dest = append([]tenantModel(nil), tenants...)
				tx.RowsAffected = int64(len(tenants))
			}
		}); err != nil {
			panic(err)
		}
	}
}

func withSchedulerDryRunQueryError(expectedErr error) schedulerDryRunDBOption {
	return func(db *gorm.DB) {
		if err := db.Callback().Query().Before("gorm:query").Register("scheduler_test:query_error", func(tx *gorm.DB) {
			tx.AddError(expectedErr)
		}); err != nil {
			panic(err)
		}
	}
}
