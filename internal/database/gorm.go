//go:build gorm

package database

import (
	"context"
	"fmt"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// GormDB wraps gorm.DB and provides multi-tenant support
type GormDB struct {
	*gorm.DB
}

// NewGormDB creates a new GORM database connection from a connection string
func NewGormDB(ctx context.Context, connString string) (*GormDB, error) {
	db, err := gorm.Open(postgres.Open(connString), &gorm.Config{
		Logger:                 logger.Default.LogMode(logger.Warn),
		SkipDefaultTransaction: true, // We manage transactions explicitly
	})
	if err != nil {
		return nil, fmt.Errorf("failed to open gorm connection: %w", err)
	}

	// Get underlying sql.DB to configure connection pool
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}

	// Verify connection
	if err := sqlDB.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &GormDB{DB: db}, nil
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
