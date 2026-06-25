package tenant

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTenantWave9PeriodClosePersistenceErrors(t *testing.T) {
	ctx := context.Background()

	t.Run("close period propagates persistence errors", func(t *testing.T) {
		repo := NewMockRepository()
		repo.AddTestTenant(&Tenant{ID: "tenant-1", Name: "Acme", Settings: DefaultSettings()})
		repo.updateTenantWithEventErr = errors.New("persist failed")
		service := newTestServiceWithRepository(repo)

		_, _, err := service.ClosePeriod(ctx, "tenant-1", "user-1", &ClosePeriodRequest{PeriodEndDate: "2026-01-31"})

		require.ErrorContains(t, err, "persist failed")
	})

	t.Run("reopen period propagates persistence errors", func(t *testing.T) {
		repo := NewMockRepository()
		settings := DefaultSettings()
		settings.PeriodLockDate = strPtr("2026-01-31")
		repo.AddTestTenant(&Tenant{ID: "tenant-1", Name: "Acme", Settings: settings})
		repo.periodCloseEvents["tenant-1"] = []PeriodCloseEvent{{
			ID:            "close-1",
			TenantID:      "tenant-1",
			Action:        PeriodCloseActionClose,
			PeriodEndDate: "2026-01-31",
			LockDateAfter: strPtr("2026-01-31"),
		}}
		repo.updateTenantWithEventErr = errors.New("persist reopen failed")
		service := newTestServiceWithRepository(repo)

		_, _, err := service.ReopenPeriod(ctx, "tenant-1", "user-1", &ReopenPeriodRequest{PeriodEndDate: "2026-01-31", Note: "undo close"})

		require.ErrorContains(t, err, "persist reopen failed")
	})

	t.Run("reopen period propagates load errors", func(t *testing.T) {
		service := newTestServiceWithRepository(NewMockRepository())

		_, _, err := service.ReopenPeriod(ctx, "missing-tenant", "user-1", &ReopenPeriodRequest{PeriodEndDate: "2026-01-31", Note: "undo close"})

		require.ErrorContains(t, err, "tenant not found")
	})
}

func TestTenantWave9DateAndPasswordGuardBranches(t *testing.T) {
	date := mustParsePeriodDateWave9(t, "2026-12-31")
	assert.Equal(t, PeriodCloseKindYearEnd, closeKindForDate(date, 0))
	assert.Equal(t, PeriodCloseKindYearEnd, closeKindForDate(date, 13))

	repo := NewMockRepository()
	service := newTestServiceWithRepository(repo)
	user, err := service.CreateUser(context.Background(), &CreateUserRequest{
		Email:    "long-password@example.com",
		Password: "oldpassword123",
		Name:     "Long Password",
	})
	require.NoError(t, err)

	err = service.ChangeUserPassword(context.Background(), user.ID, "oldpassword123", strings.Repeat("x", 73))
	require.ErrorContains(t, err, "hash password")

	defaultCostService := NewServiceWithRepository(NewMockRepository())
	defaultCostService.passwordHashCost = 0
	hash, err := defaultCostService.hashPassword("new-password")
	require.NoError(t, err)
	assert.NotEmpty(t, hash)
}

func mustParsePeriodDateWave9(t *testing.T, value string) time.Time {
	t.Helper()
	parsed, err := time.Parse(periodCloseDateLayout, value)
	require.NoError(t, err)
	return parsed
}
