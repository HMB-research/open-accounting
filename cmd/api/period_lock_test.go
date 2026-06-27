package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnsurePeriodUnlockedBranches(t *testing.T) {
	ctx := context.Background()
	tenantRepo := newMockTenantRepository()
	tenantRecord := tenantRepo.addTestTenant("tenant-1", "Tenant", "tenant")
	h := &Handlers{tenantService: newTestTenantService(tenantRepo)}

	err := h.ensurePeriodUnlocked(ctx, "tenant-1", time.Date(2026, 2, 1, 15, 30, 0, 0, time.UTC))
	require.NoError(t, err)

	blank := " "
	tenantRecord.Settings.PeriodLockDate = &blank
	err = h.ensurePeriodUnlocked(ctx, "tenant-1", time.Date(2026, 2, 1, 15, 30, 0, 0, time.UTC))
	require.NoError(t, err)

	lockRaw := "2026-01-31"
	tenantRecord.Settings.PeriodLockDate = &lockRaw
	err = h.ensurePeriodUnlocked(ctx, "tenant-1", time.Date(2026, 2, 1, 15, 30, 0, 0, time.UTC))
	require.NoError(t, err)

	err = h.ensurePeriodUnlocked(ctx, "tenant-1", time.Date(2026, 1, 31, 23, 59, 0, 0, time.UTC))
	var lockedErr *periodLockedError
	require.ErrorAs(t, err, &lockedErr)
	assert.Contains(t, lockedErr.Error(), "period locked through 2026-01-31")

	invalidLockRaw := "31-01-2026"
	tenantRecord.Settings.PeriodLockDate = &invalidLockRaw
	err = h.ensurePeriodUnlocked(ctx, "tenant-1", time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC))
	require.ErrorContains(t, err, "invalid tenant period lock date")

	tenantRepo.getTenantErr = assert.AnError
	err = h.ensurePeriodUnlocked(ctx, "tenant-1", time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC))
	require.ErrorContains(t, err, "load tenant settings")
}

func TestRejectLockedPeriodStatusCodes(t *testing.T) {
	ctx := context.Background()

	t.Run("unlocked", func(t *testing.T) {
		tenantRepo := newMockTenantRepository()
		tenantRecord := tenantRepo.addTestTenant("tenant-1", "Tenant", "tenant")
		lockRaw := "2026-01-31"
		tenantRecord.Settings.PeriodLockDate = &lockRaw
		h := &Handlers{tenantService: newTestTenantService(tenantRepo)}
		rr := httptest.NewRecorder()

		rejected := h.rejectLockedPeriod(rr, ctx, "tenant-1", time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC))

		assert.False(t, rejected)
		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("locked", func(t *testing.T) {
		tenantRepo := newMockTenantRepository()
		tenantRecord := tenantRepo.addTestTenant("tenant-1", "Tenant", "tenant")
		lockRaw := "2026-01-31"
		tenantRecord.Settings.PeriodLockDate = &lockRaw
		h := &Handlers{tenantService: newTestTenantService(tenantRepo)}
		rr := httptest.NewRecorder()

		rejected := h.rejectLockedPeriod(rr, ctx, "tenant-1", time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC))

		assert.True(t, rejected)
		assert.Equal(t, http.StatusConflict, rr.Code)
		assert.Contains(t, rr.Body.String(), "period locked through 2026-01-31")
	})

	t.Run("validation error", func(t *testing.T) {
		tenantRepo := newMockTenantRepository()
		tenantRepo.getTenantErr = assert.AnError
		h := &Handlers{tenantService: newTestTenantService(tenantRepo)}
		rr := httptest.NewRecorder()

		rejected := h.rejectLockedPeriod(rr, ctx, "tenant-1", time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC))

		assert.True(t, rejected)
		assert.Equal(t, http.StatusInternalServerError, rr.Code)
		assert.Contains(t, rr.Body.String(), "Failed to validate period lock")
	})
}
