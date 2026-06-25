package tenant

import (
	"context"
	"math"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTenantWave11SettingsMarshalErrors(t *testing.T) {
	ctx := context.Background()

	t.Run("create tenant rejects non-finite settings", func(t *testing.T) {
		settings := DefaultSettings()
		settings.LatePaymentInterestRate = math.NaN()
		service := newTestServiceWithRepository(NewMockRepository())

		tenant, err := service.CreateTenant(ctx, &CreateTenantRequest{
			Name:     "Acme",
			Slug:     "acme",
			Settings: &settings,
			OwnerID:  "owner-1",
		})

		require.Nil(t, tenant)
		require.ErrorContains(t, err, "marshal settings")
	})

	t.Run("update tenant rejects non-finite stored settings", func(t *testing.T) {
		settings := DefaultSettings()
		settings.LatePaymentInterestRate = math.Inf(1)
		repo := NewMockRepository()
		repo.AddTestTenant(&Tenant{ID: "tenant-1", Name: "Acme", Slug: "acme", Settings: settings})
		service := newTestServiceWithRepository(repo)
		name := "Acme updated"

		tenant, err := service.UpdateTenant(ctx, "tenant-1", &UpdateTenantRequest{Name: &name})

		require.Nil(t, tenant)
		require.ErrorContains(t, err, "marshal settings")
	})

	t.Run("late payment interest rejects non-finite rate", func(t *testing.T) {
		repo := NewMockRepository()
		repo.AddTestTenant(&Tenant{ID: "tenant-1", Name: "Acme", Slug: "acme", Settings: DefaultSettings()})
		service := newTestServiceWithRepository(repo)

		tenant, err := service.UpdateLatePaymentInterestRate(ctx, "tenant-1", math.NaN())

		require.Nil(t, tenant)
		require.ErrorContains(t, err, "marshal settings")
	})

	t.Run("close period rejects non-finite stored settings", func(t *testing.T) {
		settings := DefaultSettings()
		settings.LatePaymentInterestRate = math.NaN()
		repo := NewMockRepository()
		repo.AddTestTenant(&Tenant{ID: "tenant-1", Name: "Acme", Slug: "acme", Settings: settings})
		service := newTestServiceWithRepository(repo)

		tenant, event, err := service.ClosePeriod(ctx, "tenant-1", "user-1", &ClosePeriodRequest{PeriodEndDate: "2026-01-31"})

		require.Nil(t, tenant)
		require.Nil(t, event)
		require.ErrorContains(t, err, "marshal tenant settings")
	})
}
