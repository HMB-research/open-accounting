package tenant

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeEvidencePolicyMode(t *testing.T) {
	for _, tt := range []struct {
		input string
		want  string
		valid bool
	}{
		{input: "", want: EvidencePolicyModeWarn, valid: true},
		{input: " WARN ", want: EvidencePolicyModeWarn, valid: true},
		{input: "block_high_risk", want: EvidencePolicyModeBlockHighRisk, valid: true},
		{input: "block_everything", valid: false},
	} {
		t.Run(tt.input, func(t *testing.T) {
			got, err := NormalizeEvidencePolicyMode(tt.input)
			if !tt.valid {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestTenantSettingsBlocksHighRiskEvidence(t *testing.T) {
	assert.False(t, (TenantSettings{}).BlocksHighRiskEvidence())
	assert.False(t, (TenantSettings{EvidencePolicyMode: EvidencePolicyModeWarn}).BlocksHighRiskEvidence())
	assert.True(t, (TenantSettings{EvidencePolicyMode: " BLOCK_HIGH_RISK "}).BlocksHighRiskEvidence())
}

func TestTenantServiceNormalizesAndValidatesEvidencePolicySettings(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	service := newTestServiceWithRepository(repo)

	created, err := service.CreateTenant(ctx, &CreateTenantRequest{
		Name: "Pilot",
		Slug: "pilot",
		Settings: &TenantSettings{
			EvidencePolicyMode: EvidencePolicyModeBlockHighRisk,
		},
	})
	require.NoError(t, err)
	assert.Equal(t, EvidencePolicyModeBlockHighRisk, created.Settings.EvidencePolicyMode)

	updated, err := service.UpdateTenant(ctx, created.ID, &UpdateTenantRequest{Settings: &TenantSettings{EvidencePolicyMode: EvidencePolicyModeWarn}})
	require.NoError(t, err)
	assert.Equal(t, EvidencePolicyModeWarn, updated.Settings.EvidencePolicyMode)

	_, err = service.CreateTenant(ctx, &CreateTenantRequest{
		Name:     "Invalid policy",
		Slug:     "invalid-policy",
		Settings: &TenantSettings{EvidencePolicyMode: "block_everything"},
	})
	require.ErrorContains(t, err, "invalid evidence policy mode")

	_, err = service.UpdateTenant(ctx, created.ID, &UpdateTenantRequest{Settings: &TenantSettings{EvidencePolicyMode: "block_everything"}})
	require.ErrorContains(t, err, "invalid evidence policy mode")

	require.NoError(t, normalizeEvidencePolicySettings(nil))
	err = normalizeEvidencePolicySettings(&TenantSettings{EvidencePolicyMode: "block_everything"})
	require.ErrorContains(t, err, "invalid evidence policy mode")
}
