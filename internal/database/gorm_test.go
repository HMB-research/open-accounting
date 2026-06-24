package database

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestNewGormDBFromPoolRejectsNilPool(t *testing.T) {
	db, err := NewGormDBFromPool(context.Background(), nil)
	if err == nil {
		t.Fatal("expected nil pool error")
	}
	if db != nil {
		t.Fatal("expected nil gorm DB on error")
	}
}

type gormDryRunConnPool struct{}

func (gormDryRunConnPool) PrepareContext(context.Context, string) (*sql.Stmt, error) {
	return nil, errors.New("dry-run tests should not prepare statements")
}

func (gormDryRunConnPool) ExecContext(context.Context, string, ...interface{}) (sql.Result, error) {
	return nil, errors.New("dry-run tests should not execute statements")
}

func (gormDryRunConnPool) QueryContext(context.Context, string, ...interface{}) (*sql.Rows, error) {
	return nil, errors.New("dry-run tests should not query rows")
}

func (gormDryRunConnPool) QueryRowContext(context.Context, string, ...interface{}) *sql.Row {
	return nil
}

func (gormDryRunConnPool) BeginTx(context.Context, *sql.TxOptions) (gorm.ConnPool, error) {
	return &gormDryRunTx{}, nil
}

type gormDryRunTx struct {
	gormDryRunConnPool
}

func (*gormDryRunTx) Commit() error {
	return nil
}

func (*gormDryRunTx) Rollback() error {
	return nil
}

func newDryRunGormWrapper(t *testing.T) *GormDB {
	t.Helper()

	db, err := gorm.Open(postgres.New(postgres.Config{Conn: gormDryRunConnPool{}}), &gorm.Config{
		DisableAutomaticPing:   true,
		DryRun:                 true,
		Logger:                 logger.Default.LogMode(logger.Silent),
		SkipDefaultTransaction: true,
	})
	if err != nil {
		t.Fatalf("open dry-run gorm database: %v", err)
	}
	return &GormDB{DB: db}
}

func TestGormDBWrapperMethodsWithDryRunDB(t *testing.T) {
	ctx := context.Background()
	db := newDryRunGormWrapper(t)

	if got := db.WithContext(ctx); got == nil {
		t.Fatal("WithContext() returned nil")
	}

	if err := db.Transaction(ctx, func(tx *gorm.DB) error {
		if tx == nil {
			t.Fatal("transaction callback received nil tx")
		}
		return nil
	}); err != nil {
		t.Fatalf("Transaction() error = %v, want nil", err)
	}

	expectedErr := errors.New("rollback")
	if err := db.Transaction(ctx, func(tx *gorm.DB) error {
		return expectedErr
	}); !errors.Is(err, expectedErr) {
		t.Fatalf("Transaction() error = %v, want %v", err, expectedErr)
	}

	if err := db.Close(); err == nil {
		t.Fatal("Close() error = nil, want error for non-sql dry-run connection")
	}
}
