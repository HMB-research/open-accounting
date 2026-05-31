package demo

import (
	"context"
	"fmt"
	"time"

	"github.com/HMB-research/open-accounting/internal/database"
	"github.com/jackc/pgx/v5/pgxpool"
	"gorm.io/gorm"
)

// ResetAdvisoryLockKey serializes demo reset schema cleanup and seed writes.
const ResetAdvisoryLockKey int64 = 12345678

// ResetUser identifies a demo tenant/user pair that can be reset.
type ResetUser struct {
	Number int
	Email  string
	Slug   string
	Schema string
}

// SeedScriptFunc returns the seed script for the selected demo user numbers.
type SeedScriptFunc func(userNums []int) string

// ResetService resets demo tenants behind a reusable service boundary.
type ResetService struct {
	pool            *pgxpool.Pool
	db              *gorm.DB
	seedScript      SeedScriptFunc
	advisoryLockKey int64
}

// NewResetService creates a demo reset service.
func NewResetService(ctx context.Context, pool *pgxpool.Pool, seedScript SeedScriptFunc) (*ResetService, error) {
	if pool == nil {
		return nil, fmt.Errorf("database pool is not configured")
	}
	db, err := database.NewGormDBFromPool(ctx, pool)
	if err != nil {
		return nil, fmt.Errorf("create demo reset ORM repository: %w", err)
	}
	return &ResetService{
		pool:            pool,
		db:              db,
		seedScript:      seedScript,
		advisoryLockKey: ResetAdvisoryLockKey,
	}, nil
}

// Reset drops selected demo schemas, removes their public rows, and seeds fresh data.
func (s *ResetService) Reset(ctx context.Context, users []ResetUser, userNums []int) error {
	if s == nil || s.pool == nil || s.db == nil {
		return fmt.Errorf("demo reset service is not configured")
	}
	if s.seedScript == nil {
		return fmt.Errorf("demo seed script provider is not configured")
	}

	conn, err := s.pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire database connection: %w", err)
	}
	defer conn.Release()

	if _, err := conn.Exec(ctx, "SELECT pg_advisory_lock($1)", s.advisoryLockKey); err != nil {
		return fmt.Errorf("acquire demo reset lock: %w", err)
	}
	defer func() {
		unlockCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = conn.Exec(unlockCtx, "SELECT pg_advisory_unlock($1)", s.advisoryLockKey)
	}()

	for _, user := range users {
		if err := s.dropTenantSchema(ctx, user.Schema); err != nil {
			return err
		}
	}
	for _, user := range users {
		if err := s.cleanPublicDemoRows(ctx, user); err != nil {
			return err
		}
	}

	seedSQL := s.seedScript(userNums)
	if seedSQL == "" {
		return fmt.Errorf("demo seed script is empty")
	}
	if _, err := conn.Exec(ctx, seedSQL); err != nil {
		return fmt.Errorf("seed demo data: %w", err)
	}
	return nil
}

func (s *ResetService) dropTenantSchema(ctx context.Context, schemaName string) error {
	quotedSchema, err := database.QuoteIdentifier(schemaName)
	if err != nil {
		return fmt.Errorf("quote tenant schema: %w", err)
	}
	if err := s.db.WithContext(ctx).Exec("DROP SCHEMA IF EXISTS " + quotedSchema + " CASCADE").Error; err != nil {
		return fmt.Errorf("drop tenant schema %s: %w", schemaName, err)
	}
	return nil
}

func (s *ResetService) cleanPublicDemoRows(ctx context.Context, user ResetUser) error {
	db := s.db.WithContext(ctx)
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
