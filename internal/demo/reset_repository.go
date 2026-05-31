package demo

import (
	"context"
	"fmt"
	"time"

	"github.com/HMB-research/open-accounting/internal/database"
	"github.com/jackc/pgx/v5/pgxpool"
	"gorm.io/gorm"
)

// GORMResetRepository applies demo reset persistence with ORM-backed public-row cleanup.
type GORMResetRepository struct {
	pool            *pgxpool.Pool
	db              *gorm.DB
	advisoryLockKey int64
}

// NewResetRepository creates an ORM-backed demo reset repository.
func NewResetRepository(pool *pgxpool.Pool, db *gorm.DB) *GORMResetRepository {
	return &GORMResetRepository{
		pool:            pool,
		db:              db,
		advisoryLockKey: ResetAdvisoryLockKey,
	}
}

// NewResetRepositoryFromPool creates an ORM-backed demo reset repository from a pgx pool.
func NewResetRepositoryFromPool(ctx context.Context, pool *pgxpool.Pool) (*GORMResetRepository, error) {
	if pool == nil {
		return nil, fmt.Errorf("database pool is not configured")
	}
	db, err := database.NewGormDBFromPool(ctx, pool)
	if err != nil {
		return nil, fmt.Errorf("create demo reset ORM repository: %w", err)
	}
	return NewResetRepository(pool, db), nil
}

// ResetDemoData drops selected demo schemas, removes public rows, and runs the seed script.
func (r *GORMResetRepository) ResetDemoData(ctx context.Context, users []ResetUser, seedSQL string) error {
	if r == nil || r.pool == nil || r.db == nil {
		return fmt.Errorf("demo reset repository is not configured")
	}

	conn, err := r.pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire database connection: %w", err)
	}
	defer conn.Release()

	if _, err := conn.Exec(ctx, "SELECT pg_advisory_lock($1)", r.advisoryLockKey); err != nil {
		return fmt.Errorf("acquire demo reset lock: %w", err)
	}
	defer func() {
		unlockCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = conn.Exec(unlockCtx, "SELECT pg_advisory_unlock($1)", r.advisoryLockKey)
	}()

	for _, user := range users {
		if err := r.dropTenantSchema(ctx, user.Schema); err != nil {
			return err
		}
	}
	for _, user := range users {
		if err := r.cleanPublicDemoRows(ctx, user); err != nil {
			return err
		}
	}

	if _, err := conn.Exec(ctx, seedSQL); err != nil {
		return fmt.Errorf("seed demo data: %w", err)
	}
	return nil
}

func (r *GORMResetRepository) dropTenantSchema(ctx context.Context, schemaName string) error {
	quotedSchema, err := database.QuoteIdentifier(schemaName)
	if err != nil {
		return fmt.Errorf("quote tenant schema: %w", err)
	}
	if err := r.db.WithContext(ctx).Exec("DROP SCHEMA IF EXISTS " + quotedSchema + " CASCADE").Error; err != nil {
		return fmt.Errorf("drop tenant schema %s: %w", schemaName, err)
	}
	return nil
}

func (r *GORMResetRepository) cleanPublicDemoRows(ctx context.Context, user ResetUser) error {
	db := r.db.WithContext(ctx)
	if err := db.Table("tenant_users").
		Where("tenant_id IN (?)", db.Table("tenants").Select("id").Where("slug = ?", user.Slug)).
		Delete(&publicTenantUserRow{}).Error; err != nil {
		return fmt.Errorf("clean tenant users for %s: %w", user.Slug, err)
	}
	if err := db.Table("tenants").Where("slug = ?", user.Slug).Delete(&publicTenantRow{}).Error; err != nil {
		return fmt.Errorf("clean tenant %s: %w", user.Slug, err)
	}
	if err := db.Table("users").Where("email = ?", user.Email).Delete(&publicUserRow{}).Error; err != nil {
		return fmt.Errorf("clean user %s: %w", user.Email, err)
	}
	return nil
}

type publicTenantUserRow struct{}

type publicTenantRow struct{}

type publicUserRow struct{}
