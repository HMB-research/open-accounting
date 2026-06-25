package database

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func restoreGormConstructorSeams(t *testing.T) {
	t.Helper()
	originalOpenGorm := openGorm
	originalOpenDBFromPool := openDBFromPool
	originalGormDBSQL := gormDBSQL
	originalPingSQLDB := pingSQLDB
	t.Cleanup(func() {
		openGorm = originalOpenGorm
		openDBFromPool = originalOpenDBFromPool
		gormDBSQL = originalGormDBSQL
		pingSQLDB = originalPingSQLDB
	})
}

type gormPingConnector struct {
	pingErr error
}

func (c gormPingConnector) Connect(context.Context) (driver.Conn, error) {
	return gormPingConn{pingErr: c.pingErr}, nil
}

func (gormPingConnector) Driver() driver.Driver {
	return gormPingDriver{}
}

type gormPingDriver struct{}

func (gormPingDriver) Open(string) (driver.Conn, error) {
	return nil, errors.New("gorm ping test driver should use Connector")
}

type gormPingConn struct {
	pingErr error
}

func (c gormPingConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("gorm ping test driver should not prepare statements")
}

func (c gormPingConn) Close() error {
	return nil
}

func (c gormPingConn) Begin() (driver.Tx, error) {
	return nil, errors.New("gorm ping test driver should not begin transactions")
}

func (c gormPingConn) Ping(context.Context) error {
	return c.pingErr
}

func newGormDBFromSQLHandle(t *testing.T, sqlDB *sql.DB) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{
		DisableAutomaticPing:   true,
		Logger:                 logger.Default.LogMode(logger.Silent),
		SkipDefaultTransaction: true,
	})
	if err != nil {
		t.Fatalf("open gorm database: %v", err)
	}
	return db
}

func TestNewGormDBReturnsOpenError(t *testing.T) {
	restoreGormConstructorSeams(t)
	expectedErr := errors.New("open failed")
	openGorm = func(gorm.Dialector, ...gorm.Option) (*gorm.DB, error) {
		return nil, expectedErr
	}

	db, err := NewGormDB(context.Background(), "postgres://ignored")

	if !errors.Is(err, expectedErr) {
		t.Fatalf("NewGormDB() error = %v, want %v", err, expectedErr)
	}
	if db != nil {
		t.Fatalf("NewGormDB() db = %#v, want nil", db)
	}
}

func TestNewGormDBReturnsUnderlyingDBError(t *testing.T) {
	restoreGormConstructorSeams(t)
	expectedErr := errors.New("sql handle unavailable")
	fakeDB := &gorm.DB{}
	openGorm = func(gorm.Dialector, ...gorm.Option) (*gorm.DB, error) {
		return fakeDB, nil
	}
	gormDBSQL = func(db *gorm.DB) (*sql.DB, error) {
		if db != fakeDB {
			t.Fatalf("gormDBSQL got %#v, want %#v", db, fakeDB)
		}
		return nil, expectedErr
	}

	db, err := NewGormDB(context.Background(), "postgres://ignored")

	if !errors.Is(err, expectedErr) {
		t.Fatalf("NewGormDB() error = %v, want %v", err, expectedErr)
	}
	if db != nil {
		t.Fatalf("NewGormDB() db = %#v, want nil", db)
	}
}

func TestNewGormDBUsesInjectedOpenAndPing(t *testing.T) {
	restoreGormConstructorSeams(t)
	fakeDB := &gorm.DB{}
	var pinged bool
	openGorm = func(gorm.Dialector, ...gorm.Option) (*gorm.DB, error) {
		return fakeDB, nil
	}
	gormDBSQL = func(db *gorm.DB) (*sql.DB, error) {
		if db != fakeDB {
			t.Fatalf("gormDBSQL got %#v, want %#v", db, fakeDB)
		}
		return nil, nil
	}
	pingSQLDB = func(context.Context, *sql.DB) error {
		pinged = true
		return nil
	}

	db, err := NewGormDB(context.Background(), "postgres://ignored")

	if err != nil {
		t.Fatalf("NewGormDB() error = %v, want nil", err)
	}
	if db == nil || db.DB != fakeDB {
		t.Fatalf("NewGormDB() db = %#v, want wrapper around %#v", db, fakeDB)
	}
	if !pinged {
		t.Fatal("expected pingSQLDB to be called")
	}
}

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
	restoreGormConstructorSeams(t)
	expectedErr := errors.New("ping failed")
	sqlDB := sql.OpenDB(gormPingConnector{pingErr: expectedErr})
	defer sqlDB.Close()
	fakeDB := newGormDBFromSQLHandle(t, sqlDB)
	openGorm = func(gorm.Dialector, ...gorm.Option) (*gorm.DB, error) {
		return fakeDB, nil
	}

	db, err := NewGormDB(context.Background(), "postgres://ignored")
	if !errors.Is(err, expectedErr) {
		t.Fatalf("NewGormDB() error = %v, want %v", err, expectedErr)
	}
	if db != nil {
		t.Fatalf("NewGormDB() db = %#v, want nil", db)
	}
}

func TestNewGormDBFromPoolReturnsOpenError(t *testing.T) {
	restoreGormConstructorSeams(t)
	expectedErr := errors.New("open from pool failed")
	pool := new(pgxpool.Pool)
	openDBFromPool = func(got *pgxpool.Pool, _ ...stdlib.OptionOpenDB) *sql.DB {
		if got != pool {
			t.Fatalf("openDBFromPool got %#v, want %#v", got, pool)
		}
		return nil
	}
	openGorm = func(gorm.Dialector, ...gorm.Option) (*gorm.DB, error) {
		return nil, expectedErr
	}

	db, err := NewGormDBFromPool(context.Background(), pool)

	if !errors.Is(err, expectedErr) {
		t.Fatalf("NewGormDBFromPool() error = %v, want %v", err, expectedErr)
	}
	if db != nil {
		t.Fatalf("NewGormDBFromPool() db = %#v, want nil", db)
	}
}

func TestNewGormDBFromPoolReturnsPingError(t *testing.T) {
	restoreGormConstructorSeams(t)
	expectedErr := errors.New("ping failed")
	pool := new(pgxpool.Pool)
	fakeDB := &gorm.DB{}
	openDBFromPool = func(got *pgxpool.Pool, _ ...stdlib.OptionOpenDB) *sql.DB {
		if got != pool {
			t.Fatalf("openDBFromPool got %#v, want %#v", got, pool)
		}
		return nil
	}
	openGorm = func(gorm.Dialector, ...gorm.Option) (*gorm.DB, error) {
		return fakeDB, nil
	}
	pingSQLDB = func(context.Context, *sql.DB) error {
		return expectedErr
	}

	db, err := NewGormDBFromPool(context.Background(), pool)

	if !errors.Is(err, expectedErr) {
		t.Fatalf("NewGormDBFromPool() error = %v, want %v", err, expectedErr)
	}
	if db != nil {
		t.Fatalf("NewGormDBFromPool() db = %#v, want nil", db)
	}
}

func TestNewGormDBFromPoolUsesInjectedOpenAndPing(t *testing.T) {
	restoreGormConstructorSeams(t)
	pool := new(pgxpool.Pool)
	fakeDB := &gorm.DB{}
	var pinged bool
	openDBFromPool = func(got *pgxpool.Pool, _ ...stdlib.OptionOpenDB) *sql.DB {
		if got != pool {
			t.Fatalf("openDBFromPool got %#v, want %#v", got, pool)
		}
		return nil
	}
	openGorm = func(gorm.Dialector, ...gorm.Option) (*gorm.DB, error) {
		return fakeDB, nil
	}
	pingSQLDB = func(context.Context, *sql.DB) error {
		pinged = true
		return nil
	}

	db, err := NewGormDBFromPool(context.Background(), pool)

	if err != nil {
		t.Fatalf("NewGormDBFromPool() error = %v, want nil", err)
	}
	if db != fakeDB {
		t.Fatalf("NewGormDBFromPool() db = %#v, want %#v", db, fakeDB)
	}
	if !pinged {
		t.Fatal("expected pingSQLDB to be called")
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

func TestGormDBCloseClosesUnderlyingSQLHandle(t *testing.T) {
	sqlDB := sql.OpenDB(gormPingConnector{})
	db := newGormDBFromSQLHandle(t, sqlDB)

	if err := (&GormDB{DB: db}).Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}
