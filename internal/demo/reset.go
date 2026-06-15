package demo

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
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

// ResetRepository applies demo reset persistence operations.
type ResetRepository interface {
	ResetDemoData(ctx context.Context, users []ResetUser, seedSQL string) error
}

// ResetService resets demo tenants behind a reusable service boundary.
type ResetService struct {
	repository ResetRepository
	seedScript SeedScriptFunc
}

// NewResetService creates a demo reset service.
func NewResetService(ctx context.Context, pool *pgxpool.Pool, seedScript SeedScriptFunc) (*ResetService, error) {
	repository, err := NewResetRepositoryFromPool(ctx, pool)
	if err != nil {
		return nil, err
	}
	return NewResetServiceWithRepository(repository, seedScript), nil
}

// NewResetServiceWithRepository creates a demo reset service with an explicit repository.
func NewResetServiceWithRepository(repository ResetRepository, seedScript SeedScriptFunc) *ResetService {
	return &ResetService{
		repository: repository,
		seedScript: seedScript,
	}
}

// Reset drops selected demo schemas, removes their public rows, and seeds fresh data.
func (s *ResetService) Reset(ctx context.Context, users []ResetUser, userNums []int) error {
	if s == nil || s.repository == nil {
		return fmt.Errorf("demo reset service is not configured")
	}
	if s.seedScript == nil {
		return fmt.Errorf("demo seed script provider is not configured")
	}

	seedSQL := s.seedScript(userNums)
	if seedSQL == "" {
		return fmt.Errorf("demo seed script is empty")
	}
	return s.repository.ResetDemoData(ctx, users, seedSQL)
}
