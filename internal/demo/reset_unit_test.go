package demo

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResetServiceUnitPaths(t *testing.T) {
	ctx := context.Background()
	user := ResetUser{
		Number: 1,
		Email:  "demo@example.com",
		Slug:   "demo",
		Schema: "tenant_demo",
	}

	repository := &unitResetRepository{}
	var receivedNums []int
	service := NewResetServiceWithRepository(repository, func(userNums []int) string {
		receivedNums = append(receivedNums, userNums...)
		return "seed sql"
	})

	require.NoError(t, service.Reset(ctx, []ResetUser{user}, []int{user.Number}))
	assert.Equal(t, []int{user.Number}, receivedNums)
	assert.Equal(t, 1, repository.calls)
	assert.Equal(t, []ResetUser{user}, repository.users)
	assert.Equal(t, "seed sql", repository.seedSQL)

	expectedErr := errors.New("repository failed")
	service = NewResetServiceWithRepository(&unitResetRepository{err: expectedErr}, func(userNums []int) string {
		return "seed sql"
	})
	require.ErrorIs(t, service.Reset(ctx, []ResetUser{user}, []int{user.Number}), expectedErr)

	service = NewResetServiceWithRepository(nil, func(userNums []int) string {
		return "seed sql"
	})
	err := service.Reset(ctx, []ResetUser{user}, []int{user.Number})
	require.ErrorContains(t, err, "demo reset service is not configured")

	var nilService *ResetService
	err = nilService.Reset(ctx, []ResetUser{user}, []int{user.Number})
	require.ErrorContains(t, err, "demo reset service is not configured")

	service = NewResetServiceWithRepository(&unitResetRepository{}, nil)
	err = service.Reset(ctx, []ResetUser{user}, []int{user.Number})
	require.ErrorContains(t, err, "demo seed script provider is not configured")

	service = NewResetServiceWithRepository(&unitResetRepository{}, func(userNums []int) string {
		return ""
	})
	err = service.Reset(ctx, []ResetUser{user}, []int{user.Number})
	require.ErrorContains(t, err, "demo seed script is empty")
}

func TestDemoConstructorsRejectNilPools(t *testing.T) {
	ctx := context.Background()

	service, err := NewResetService(ctx, nil, func(userNums []int) string { return "seed sql" })
	require.ErrorContains(t, err, "database pool is not configured")
	assert.Nil(t, service)

	repository, err := NewResetRepositoryFromPool(ctx, nil)
	require.ErrorContains(t, err, "database pool is not configured")
	assert.Nil(t, repository)

	reader, err := NewStatusReader(nil)
	require.ErrorContains(t, err, "database pool is not configured")
	assert.Nil(t, reader)
}

func TestResetRepositoryUnitConfiguration(t *testing.T) {
	repository := NewResetRepository(nil, nil)
	require.NotNil(t, repository)
	assert.Equal(t, ResetAdvisoryLockKey, repository.advisoryLockKey)

	err := repository.ResetDemoData(context.Background(), []ResetUser{{Number: 1}}, "seed sql")
	require.ErrorContains(t, err, "demo reset repository is not configured")

	var nilRepository *GORMResetRepository
	err = nilRepository.ResetDemoData(context.Background(), []ResetUser{{Number: 1}}, "seed sql")
	require.ErrorContains(t, err, "demo reset repository is not configured")
}

type unitResetRepository struct {
	calls   int
	users   []ResetUser
	seedSQL string
	err     error
}

func (r *unitResetRepository) ResetDemoData(_ context.Context, users []ResetUser, seedSQL string) error {
	r.calls++
	r.users = append([]ResetUser(nil), users...)
	r.seedSQL = seedSQL
	return r.err
}
