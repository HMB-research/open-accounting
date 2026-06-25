package tenant

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestGORMRepositoryWave5SlugAndUserLookupErrors(t *testing.T) {
	ctx := context.Background()

	repo := NewGORMRepository(newTenantDryRunDB(t, withTenantDryRunQueryError(gorm.ErrRecordNotFound)))
	tenantValue, err := repo.GetTenantBySlug(ctx, "missing")
	assert.Nil(t, tenantValue)
	assert.ErrorIs(t, err, ErrTenantNotFound)

	userByEmail, err := repo.GetUserByEmail(ctx, "missing@example.com")
	assert.Nil(t, userByEmail)
	assert.ErrorIs(t, err, ErrUserNotFound)

	userByID, err := repo.GetUserByID(ctx, "missing-user")
	assert.Nil(t, userByID)
	assert.ErrorIs(t, err, ErrUserNotFound)

	expectedErr := errors.New("lookup failed")
	repo = NewGORMRepository(newTenantDryRunDB(t, withTenantDryRunQueryError(expectedErr)))
	tenantValue, err = repo.GetTenantBySlug(ctx, "broken")
	assert.Nil(t, tenantValue)
	require.ErrorContains(t, err, "get tenant")
	assert.ErrorIs(t, err, expectedErr)

	userByEmail, err = repo.GetUserByEmail(ctx, "broken@example.com")
	assert.Nil(t, userByEmail)
	require.ErrorContains(t, err, "get user")
	assert.ErrorIs(t, err, expectedErr)

	userByID, err = repo.GetUserByID(ctx, "broken-user")
	assert.Nil(t, userByID)
	require.ErrorContains(t, err, "get user")
	assert.ErrorIs(t, err, expectedErr)
}

func TestGORMRepositoryWave5CreateUserErrors(t *testing.T) {
	ctx := context.Background()
	user := &User{
		ID:           "user-1",
		Email:        "user@example.com",
		PasswordHash: "hash",
		Name:         "Test User",
		IsActive:     true,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	repo := NewGORMRepository(newTenantDryRunDB(t, withTenantDryRunCreateError(errors.New("duplicate key value violates unique constraint"))))
	assert.ErrorIs(t, repo.CreateUser(ctx, user), ErrEmailExists)

	expectedErr := errors.New("create failed")
	repo = NewGORMRepository(newTenantDryRunDB(t, withTenantDryRunCreateError(expectedErr)))
	err := repo.CreateUser(ctx, user)
	require.ErrorContains(t, err, "create user")
	assert.ErrorIs(t, err, expectedErr)
}

func TestGORMRepositoryWave5InvitationAndMembershipErrorBranches(t *testing.T) {
	ctx := context.Background()

	repo := NewGORMRepository(newTenantDryRunDB(t, withTenantDryRunDeleteRows(0)))
	err := repo.RevokeInvitation(ctx, "tenant-1", "invitation-1")
	require.ErrorContains(t, err, "invitation not found or already accepted")

	expectedDeleteErr := errors.New("delete invitation failed")
	repo = NewGORMRepository(newTenantDryRunDB(t, withTenantDryRunDeleteError(expectedDeleteErr)))
	err = repo.RevokeInvitation(ctx, "tenant-1", "invitation-1")
	require.ErrorContains(t, err, "revoke invitation")
	assert.ErrorIs(t, err, expectedDeleteErr)

	expectedQueryErr := errors.New("member lookup failed")
	repo = NewGORMRepository(newTenantDryRunDB(t, withTenantDryRunQueryError(expectedQueryErr)))
	member, err := repo.CheckUserIsMember(ctx, "tenant-1", "USER@EXAMPLE.COM")
	assert.False(t, member)
	require.ErrorContains(t, err, "check existing member")
	assert.ErrorIs(t, err, expectedQueryErr)

	repo = NewGORMRepository(newTenantDryRunDB(t, withTenantDryRunFixtures(tenantDryRunFixture{count: 0, countSet: true})))
	member, err = repo.CheckUserIsMember(ctx, "tenant-1", "user@example.com")
	require.NoError(t, err)
	assert.False(t, member)
}
