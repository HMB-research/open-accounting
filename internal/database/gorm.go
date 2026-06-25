package database

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var (
	openGorm       = gorm.Open
	openDBFromPool = stdlib.OpenDBFromPool
	gormDBSQL      = func(db *gorm.DB) (*sql.DB, error) { return db.DB() }
	pingSQLDB      = func(ctx context.Context, db *sql.DB) error { return db.PingContext(ctx) }
)

// GormDB wraps gorm.DB and provides multi-tenant support
type GormDB struct {
	*gorm.DB
}

// NewGormDB creates a new GORM database connection from a connection string
func NewGormDB(ctx context.Context, connString string) (*GormDB, error) {
	db, err := openGorm(postgres.Open(connString), &gorm.Config{
		Logger:                 logger.Default.LogMode(logger.Warn),
		SkipDefaultTransaction: true, // We manage transactions explicitly
	})
	if err != nil {
		return nil, fmt.Errorf("failed to open gorm connection: %w", err)
	}

	// Get underlying sql.DB to configure connection pool
	sqlDB, err := gormDBSQL(db)
	if err != nil {
		return nil, fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}

	// Verify connection
	if err := pingSQLDB(ctx, sqlDB); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &GormDB{DB: db}, nil
}

// NewGormDBFromPool creates a GORM handle from the existing pgx pool configuration.
func NewGormDBFromPool(ctx context.Context, pool *pgxpool.Pool) (*gorm.DB, error) {
	if pool == nil {
		return nil, fmt.Errorf("database connection not available")
	}

	sqlDB := openDBFromPool(pool)
	db, err := openGorm(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{
		Logger:                 logger.Default.LogMode(logger.Warn),
		SkipDefaultTransaction: true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to open gorm connection: %w", err)
	}

	if err := pingSQLDB(ctx, sqlDB); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return db, nil
}

// Close closes the database connection
func (g *GormDB) Close() error {
	sqlDB, err := g.DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// WithContext returns a new DB with context
func (g *GormDB) WithContext(ctx context.Context) *gorm.DB {
	return g.DB.WithContext(ctx)
}

// Transaction executes a function within a database transaction
func (g *GormDB) Transaction(ctx context.Context, fn func(tx *gorm.DB) error) error {
	return g.DB.WithContext(ctx).Transaction(fn)
}
