package tenant

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGORMRepositoryWave6TenantMutationErrors(t *testing.T) {
	ctx := context.Background()
	tenantID := "tenant-1"
	userID := "user-1"
	now := time.Date(2026, time.June, 25, 12, 0, 0, 0, time.UTC)
	expectedErr := errors.New("write failed")

	err := NewGORMRepository(newTenantDryRunDB(t, withTenantDryRunUpdateError(expectedErr))).UpdateTenant(ctx, tenantID, "Updated OU", []byte(`{"currency":"EUR"}`), now)
	require.ErrorContains(t, err, "update tenant")
	assert.ErrorIs(t, err, expectedErr)

	err = NewGORMRepository(newTenantDryRunDB(t, withTenantDryRunUpdateError(expectedErr))).CompleteOnboarding(ctx, tenantID)
	require.ErrorContains(t, err, "complete onboarding")
	assert.ErrorIs(t, err, expectedErr)

	err = NewGORMRepository(newTenantDryRunDB(t, withTenantDryRunCreateError(expectedErr))).CreateTenantAuditEvent(ctx, &TenantAuditEvent{
		ID:          "audit-1",
		TenantID:    tenantID,
		ActorUserID: userID,
		Action:      AuditActionInvitationCreated,
		TargetType:  AuditTargetInvitation,
		TargetID:    "invitation-1",
		Metadata:    map[string]string{"role": RoleAccountant},
		CreatedAt:   now,
	})
	require.ErrorContains(t, err, "create tenant audit event")
	assert.ErrorIs(t, err, expectedErr)

	err = NewGORMRepository(newTenantDryRunDB(t, withTenantDryRunDeleteError(expectedErr))).RemoveUserFromTenant(ctx, tenantID, userID)
	require.ErrorContains(t, err, "remove user from tenant")
	assert.ErrorIs(t, err, expectedErr)

	err = NewGORMRepository(newTenantDryRunDB(t, withTenantDryRunDeleteRows(0))).RemoveUserFromTenant(ctx, tenantID, userID)
	assert.ErrorIs(t, err, ErrUserNotInTenant)

	err = NewGORMRepository(newTenantDryRunDB(t, withTenantDryRunUpdateError(expectedErr))).UpdateTenantUserRole(ctx, tenantID, userID, RoleAdmin)
	require.ErrorContains(t, err, "update role")
	assert.ErrorIs(t, err, expectedErr)

	err = NewGORMRepository(newTenantDryRunDB(t, withTenantDryRunDeleteError(expectedErr))).RemoveTenantUser(ctx, tenantID, userID)
	require.ErrorContains(t, err, "remove user")
	assert.ErrorIs(t, err, expectedErr)

	err = NewGORMRepository(newTenantDryRunDB(t, withTenantDryRunUpdateError(expectedErr))).UpdateUserPassword(ctx, userID, "hash", now)
	require.ErrorContains(t, err, "update user password")
	assert.ErrorIs(t, err, expectedErr)

	err = NewGORMRepository(newTenantDryRunDB(t, withTenantDryRunCreateError(expectedErr))).CreateInvitation(ctx, &UserInvitation{
		ID:        "invitation-1",
		TenantID:  tenantID,
		Email:     "invitee@example.com",
		Role:      RoleAccountant,
		InvitedBy: userID,
		Token:     "token-1",
		ExpiresAt: now.Add(24 * time.Hour),
		CreatedAt: now,
	})
	require.ErrorContains(t, err, "create invitation")
	assert.ErrorIs(t, err, expectedErr)
}

func TestGORMRepositoryWave6TenantQueryErrors(t *testing.T) {
	ctx := context.Background()
	tenantID := "tenant-1"
	userID := "user-1"
	expectedErr := errors.New("query failed")
	repo := NewGORMRepository(newTenantDryRunDB(t, withTenantDryRunQueryError(expectedErr)))

	tenantUser, err := repo.GetTenantUser(ctx, tenantID, userID)
	assert.Nil(t, tenantUser)
	require.ErrorContains(t, err, "get tenant user")
	assert.ErrorIs(t, err, expectedErr)

	memberships, err := repo.ListUserTenants(ctx, userID)
	assert.Nil(t, memberships)
	require.ErrorContains(t, err, "list user tenants")

	users, err := repo.ListTenantUsers(ctx, tenantID)
	assert.Nil(t, users)
	require.ErrorContains(t, err, "list tenant users")
	assert.ErrorIs(t, err, expectedErr)

	invitations, err := repo.ListInvitations(ctx, tenantID)
	assert.Nil(t, invitations)
	require.ErrorContains(t, err, "list invitations")
	assert.ErrorIs(t, err, expectedErr)
}
