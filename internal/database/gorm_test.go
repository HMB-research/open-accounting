package database

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
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

func TestNewGormDBReturnsPingError(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	db, err := NewGormDB(ctx, "postgres://invalid")
	if err == nil {
		_ = db.Close()
		t.Fatal("expected connection error")
	}
	if db != nil {
		t.Fatalf("expected nil database on error, got %#v", db)
	}
}

func TestNewGormDBReturnsPingErrorAfterOpening(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	db, err := NewGormDB(ctx, "postgres://open_accounting:open_accounting@127.0.0.1:1/open_accounting?sslmode=disable")
	if err == nil {
		_ = db.Close()
		t.Fatal("expected ping error")
	}
	if db != nil {
		t.Fatalf("expected nil database on ping error, got %#v", db)
	}
}

func TestNewGormDBFromPoolReturnsPingErrorForUnreachablePool(t *testing.T) {
	config, err := pgxpool.ParseConfig("postgres://open_accounting:open_accounting@127.0.0.1:1/open_accounting?sslmode=disable")
	if err != nil {
		t.Fatalf("parse pgxpool config: %v", err)
	}
	config.ConnConfig.ConnectTimeout = 10 * time.Millisecond
	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		t.Fatalf("create pgxpool: %v", err)
	}
	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	db, err := NewGormDBFromPool(ctx, pool)
	if err == nil {
		sqlDB, _ := db.DB()
		if sqlDB != nil {
			_ = sqlDB.Close()
		}
		t.Fatal("expected ping error")
	}
	if db != nil {
		t.Fatalf("expected nil gorm DB on ping error, got %#v", db)
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
