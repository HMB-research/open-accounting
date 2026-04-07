package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/HMB-research/open-accounting/internal/tenant"
)

func TestResolveTenantMembership(t *testing.T) {
	memberships := []tenant.TenantMembership{
		{
			Tenant: tenant.Tenant{ID: "tenant-1", Name: "Alpha", Slug: "alpha"},
		},
		{
			Tenant:    tenant.Tenant{ID: "tenant-2", Name: "Beta", Slug: "beta"},
			IsDefault: true,
		},
	}

	match, err := resolveTenantMembership(memberships, "")
	require.NoError(t, err)
	assert.Equal(t, "tenant-2", match.Tenant.ID)

	match, err = resolveTenantMembership(memberships, "alpha")
	require.NoError(t, err)
	assert.Equal(t, "tenant-1", match.Tenant.ID)

	match, err = resolveTenantMembership(memberships, "tenant-2")
	require.NoError(t, err)
	assert.Equal(t, "Beta", match.Tenant.Name)
}

func TestResolveTenantMembershipRequiresSelectorWhenAmbiguous(t *testing.T) {
	memberships := []tenant.TenantMembership{
		{Tenant: tenant.Tenant{ID: "tenant-1", Name: "Alpha", Slug: "alpha"}},
		{Tenant: tenant.Tenant{ID: "tenant-2", Name: "Beta", Slug: "beta"}},
	}

	_, err := resolveTenantMembership(memberships, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "multiple tenants found")
}
