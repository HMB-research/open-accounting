package demo

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
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

func TestDemoConstructorsUsePoolConversion(t *testing.T) {
	original := newGormDBFromPool
	t.Cleanup(func() { newGormDBFromPool = original })

	ctx := context.Background()
	pool := &pgxpool.Pool{}
	db := newDryRunGormDB(t)
	newGormDBFromPool = func(_ context.Context, receivedPool *pgxpool.Pool) (*gorm.DB, error) {
		assert.Same(t, pool, receivedPool)
		return db, nil
	}

	repository, err := NewResetRepositoryFromPool(ctx, pool)
	require.NoError(t, err)
	require.NotNil(t, repository)
	assert.Same(t, db, repository.db)
	assert.Equal(t, ResetAdvisoryLockKey, repository.advisoryLockKey)

	service, err := NewResetService(ctx, pool, func(userNums []int) string { return "seed sql" })
	require.NoError(t, err)
	require.NotNil(t, service)
	serviceRepository, ok := service.repository.(*GORMResetRepository)
	require.True(t, ok)
	assert.Same(t, db, serviceRepository.db)

	reader, err := NewStatusReader(pool)
	require.NoError(t, err)
	statusReader, ok := reader.(*gormStatusReader)
	require.True(t, ok)
	assert.Same(t, db, statusReader.db)
}

func TestDemoConstructorsWrapPoolConversionErrors(t *testing.T) {
	original := newGormDBFromPool
	t.Cleanup(func() { newGormDBFromPool = original })

	expectedErr := errors.New("gorm conversion failed")
	newGormDBFromPool = func(context.Context, *pgxpool.Pool) (*gorm.DB, error) {
		return nil, expectedErr
	}

	repository, err := NewResetRepositoryFromPool(context.Background(), &pgxpool.Pool{})
	require.ErrorIs(t, err, expectedErr)
	assert.Nil(t, repository)
	assert.Contains(t, err.Error(), "create demo reset ORM repository")

	service, err := NewResetService(context.Background(), &pgxpool.Pool{}, func(userNums []int) string { return "seed sql" })
	require.ErrorIs(t, err, expectedErr)
	assert.Nil(t, service)
	assert.Contains(t, err.Error(), "create demo reset ORM repository")

	reader, err := NewStatusReader(&pgxpool.Pool{})
	require.ErrorIs(t, err, expectedErr)
	assert.Nil(t, reader)
	assert.Contains(t, err.Error(), "create demo status GORM reader")
}

func TestResetRepositoryUnitConfiguration(t *testing.T) {
	pool := &pgxpool.Pool{}
	db := newDryRunGormDB(t)

	repository := NewResetRepository(pool, db)
	require.NotNil(t, repository)
	assert.Same(t, pool, repository.pool)
	assert.Same(t, db, repository.db)
	assert.Equal(t, ResetAdvisoryLockKey, repository.advisoryLockKey)
	assert.NotNil(t, repository.acquireResetConn)

	tests := []struct {
		name       string
		repository *GORMResetRepository
	}{
		{name: "nil receiver"},
		{name: "missing pool", repository: NewResetRepository(nil, db)},
		{name: "missing db", repository: NewResetRepository(pool, nil)},
		{name: "missing both", repository: NewResetRepository(nil, nil)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.repository.ResetDemoData(context.Background(), []ResetUser{{Number: 1}}, "seed sql")
			require.ErrorContains(t, err, "demo reset repository is not configured")
		})
	}
}

func TestResetRepositoryResetDemoDataOrchestratesLockCleanupAndSeed(t *testing.T) {
	ctx := context.Background()
	conn := &fakeResetConn{}
	repository := newResetRepositoryWithFakeConn(t, conn, nil)
	repository.advisoryLockKey = 2468

	err := repository.ResetDemoData(ctx, []ResetUser{
		{Number: 1, Email: "demo1@example.com", Slug: "demo-one", Schema: "tenant_demo1"},
		{Number: 2, Email: "demo2@example.com", Slug: "demo-two", Schema: "tenant_demo2"},
	}, "SELECT seed_demo_data()")

	require.NoError(t, err)
	require.Equal(t, 1, conn.releaseCount)
	require.Len(t, conn.execs, 5)
	assert.Equal(t, "SELECT pg_advisory_lock($1)", conn.execs[0].sql)
	assert.Equal(t, []any{int64(2468)}, conn.execs[0].args)
	assert.Equal(t, `DROP SCHEMA IF EXISTS "tenant_demo1" CASCADE`, conn.execs[1].sql)
	assert.Equal(t, `DROP SCHEMA IF EXISTS "tenant_demo2" CASCADE`, conn.execs[2].sql)
	assert.Equal(t, "SELECT seed_demo_data()", conn.execs[3].sql)
	assert.Equal(t, "SELECT pg_advisory_unlock($1)", conn.execs[4].sql)
	assert.Equal(t, []any{int64(2468)}, conn.execs[4].args)
}

func TestResetRepositoryResetDemoDataPropagatesExternalErrors(t *testing.T) {
	ctx := context.Background()
	expectedErr := errors.New("external failure")

	t.Run("acquire connection", func(t *testing.T) {
		repository := newResetRepositoryWithFakeConn(t, nil, expectedErr)

		err := repository.ResetDemoData(ctx, []ResetUser{{Schema: "tenant_demo"}}, "SELECT 1")

		require.ErrorIs(t, err, expectedErr)
		assert.Contains(t, err.Error(), "acquire database connection")
	})

	t.Run("advisory lock", func(t *testing.T) {
		conn := &fakeResetConn{errorsBySQLToken: map[string]error{"pg_advisory_lock": expectedErr}}
		repository := newResetRepositoryWithFakeConn(t, conn, nil)

		err := repository.ResetDemoData(ctx, []ResetUser{{Schema: "tenant_demo"}}, "SELECT 1")

		require.ErrorIs(t, err, expectedErr)
		assert.Contains(t, err.Error(), "acquire demo reset lock")
		assert.Equal(t, 1, conn.releaseCount)
		assert.False(t, conn.executedSQLContaining("pg_advisory_unlock"))
	})

	t.Run("drop schema", func(t *testing.T) {
		conn := &fakeResetConn{errorsBySQLToken: map[string]error{"DROP SCHEMA": expectedErr}}
		repository := newResetRepositoryWithFakeConn(t, conn, nil)

		err := repository.ResetDemoData(ctx, []ResetUser{{Schema: "tenant_demo"}}, "SELECT 1")

		require.ErrorIs(t, err, expectedErr)
		assert.Contains(t, err.Error(), "drop tenant schema tenant_demo")
		assert.Equal(t, 1, conn.releaseCount)
		assert.True(t, conn.executedSQLContaining("pg_advisory_unlock"))
	})

	t.Run("public cleanup", func(t *testing.T) {
		db := newDryRunGormDB(t)
		registerDryRunDeleteError(t, db, expectedErr)
		conn := &fakeResetConn{}
		repository := newResetRepositoryWithFakeConnAndDB(t, conn, db, nil)

		err := repository.ResetDemoData(ctx, []ResetUser{{Email: "demo@example.com", Slug: "demo", Schema: "tenant_demo"}}, "SELECT 1")

		require.ErrorIs(t, err, expectedErr)
		assert.Contains(t, err.Error(), "clean demo plugin fixture tenant links")
		assert.False(t, conn.executedSQLContaining("SELECT 1"))
		assert.True(t, conn.executedSQLContaining("pg_advisory_unlock"))
	})

	t.Run("public demo rows cleanup", func(t *testing.T) {
		db := newDryRunGormDB(t)
		registerDryRunDeleteErrorForTable(t, db, "tenant_users", expectedErr)
		conn := &fakeResetConn{}
		repository := newResetRepositoryWithFakeConnAndDB(t, conn, db, nil)

		err := repository.ResetDemoData(ctx, []ResetUser{{Email: "demo@example.com", Slug: "demo", Schema: "tenant_demo"}}, "SELECT 1")

		require.ErrorIs(t, err, expectedErr)
		assert.Contains(t, err.Error(), "clean tenant users for demo")
		assert.False(t, conn.executedSQLContaining("SELECT 1"))
		assert.True(t, conn.executedSQLContaining("pg_advisory_unlock"))
	})

	t.Run("seed script", func(t *testing.T) {
		conn := &fakeResetConn{errorsBySQLToken: map[string]error{"SELECT fail_seed()": expectedErr}}
		repository := newResetRepositoryWithFakeConn(t, conn, nil)

		err := repository.ResetDemoData(ctx, []ResetUser{{Email: "demo@example.com", Slug: "demo", Schema: "tenant_demo"}}, "SELECT fail_seed()")

		require.ErrorIs(t, err, expectedErr)
		assert.Contains(t, err.Error(), "seed demo data")
		assert.True(t, conn.executedSQLContaining("pg_advisory_unlock"))
	})
}

func TestResetRepositoryDryRunCleanupQueries(t *testing.T) {
	ctx := context.Background()
	repository := NewResetRepository(nil, newDryRunGormDB(t))
	user := ResetUser{
		Email: "demo@example.com",
		Slug:  "demo-company",
	}

	require.NoError(t, repository.cleanPublicDemoPluginFixtures(ctx))
	require.NoError(t, repository.cleanPublicDemoRows(ctx, user))
}

func TestResetRepositoryDryRunCleanupErrors(t *testing.T) {
	ctx := context.Background()
	expectedErr := errors.New("delete failed")

	t.Run("plugin fixtures", func(t *testing.T) {
		db := newDryRunGormDB(t)
		registerDryRunDeleteError(t, db, expectedErr)
		repository := NewResetRepository(nil, db)

		err := repository.cleanPublicDemoPluginFixtures(ctx)

		require.ErrorIs(t, err, expectedErr)
		require.ErrorContains(t, err, "clean demo plugin fixture tenant links")
	})

	t.Run("demo rows", func(t *testing.T) {
		db := newDryRunGormDB(t)
		registerDryRunDeleteError(t, db, expectedErr)
		repository := NewResetRepository(nil, db)

		err := repository.cleanPublicDemoRows(ctx, ResetUser{
			Email: "demo@example.com",
			Slug:  "demo-company",
		})

		require.ErrorIs(t, err, expectedErr)
		require.ErrorContains(t, err, "clean tenant users for demo-company")
	})
}

func TestResetRepositoryDryRunCleanupLaterDeleteErrors(t *testing.T) {
	ctx := context.Background()
	expectedErr := errors.New("delete failed")
	user := ResetUser{
		Email: "demo@example.com",
		Slug:  "demo-company",
	}

	t.Run("plugin row delete", func(t *testing.T) {
		db := newDryRunGormDB(t)
		registerDryRunDeleteErrorForTable(t, db, "plugins", expectedErr)
		repository := NewResetRepository(nil, db)

		err := repository.cleanPublicDemoPluginFixtures(ctx)

		require.ErrorIs(t, err, expectedErr)
		require.ErrorContains(t, err, "clean demo plugin fixture")
	})

	t.Run("tenant delete", func(t *testing.T) {
		db := newDryRunGormDB(t)
		registerDryRunDeleteErrorForTable(t, db, "tenants", expectedErr)
		repository := NewResetRepository(nil, db)

		err := repository.cleanPublicDemoRows(ctx, user)

		require.ErrorIs(t, err, expectedErr)
		require.ErrorContains(t, err, "clean tenant demo-company")
	})

	t.Run("user delete", func(t *testing.T) {
		db := newDryRunGormDB(t)
		registerDryRunDeleteErrorForTable(t, db, "users", expectedErr)
		repository := NewResetRepository(nil, db)

		err := repository.cleanPublicDemoRows(ctx, user)

		require.ErrorIs(t, err, expectedErr)
		require.ErrorContains(t, err, "clean user demo@example.com")
	})
}

func TestResetRepositoryDropTenantSchemaExecutesQuotedDrop(t *testing.T) {
	ctx := context.Background()
	repository := NewResetRepository(nil, nil)
	conn := &fakeResetConn{}

	err := repository.dropTenantSchema(ctx, conn, "tenant_demo")

	require.NoError(t, err)
	require.Len(t, conn.execs, 1)
	assert.Equal(t, `DROP SCHEMA IF EXISTS "tenant_demo" CASCADE`, conn.execs[0].sql)
}

func TestResetRepositoryDropTenantSchemaWrapsExecError(t *testing.T) {
	ctx := context.Background()
	expectedErr := errors.New("drop failed")
	repository := NewResetRepository(nil, nil)
	conn := &fakeResetConn{errorsBySQLToken: map[string]error{"DROP SCHEMA": expectedErr}}

	err := repository.dropTenantSchema(ctx, conn, "tenant_demo")

	require.ErrorIs(t, err, expectedErr)
	assert.Contains(t, err.Error(), "drop tenant schema tenant_demo")
}

func TestResetRepositoryDropTenantSchemaRejectsInvalidSchema(t *testing.T) {
	repository := NewResetRepository(nil, nil)

	err := repository.dropTenantSchema(context.Background(), nil, "tenant-demo")

	require.ErrorContains(t, err, "quote tenant schema")
}

func registerDryRunDeleteError(t *testing.T, db *gorm.DB, err error) {
	t.Helper()

	callbackName := fmt.Sprintf("demo_reset_unit:delete_error_%d", atomic.AddUint64(&dryRunCallbackID, 1))
	registerErr := db.Callback().Delete().Before("gorm:delete").Register(callbackName, func(tx *gorm.DB) {
		tx.AddError(err)
	})
	require.NoError(t, registerErr)
}

func registerDryRunDeleteErrorForTable(t *testing.T, db *gorm.DB, table string, err error) {
	t.Helper()

	callbackName := fmt.Sprintf("demo_reset_unit:delete_error_for_%s_%d", table, atomic.AddUint64(&dryRunCallbackID, 1))
	registerErr := db.Callback().Delete().Before("gorm:delete").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement.Table == table {
			tx.AddError(err)
		}
	})
	require.NoError(t, registerErr)
}

func newResetRepositoryWithFakeConn(t *testing.T, conn *fakeResetConn, acquireErr error) *GORMResetRepository {
	t.Helper()

	return newResetRepositoryWithFakeConnAndDB(t, conn, newDryRunGormDB(t), acquireErr)
}

func newResetRepositoryWithFakeConnAndDB(t *testing.T, conn *fakeResetConn, db *gorm.DB, acquireErr error) *GORMResetRepository {
	t.Helper()

	return &GORMResetRepository{
		pool:            &pgxpool.Pool{},
		db:              db,
		advisoryLockKey: ResetAdvisoryLockKey,
		acquireResetConn: func(context.Context) (resetConn, error) {
			if acquireErr != nil {
				return nil, acquireErr
			}
			return conn, nil
		},
	}
}

type fakeResetConn struct {
	execs            []fakeResetExec
	releaseCount     int
	errorsBySQLToken map[string]error
}

type fakeResetExec struct {
	sql  string
	args []any
}

func (c *fakeResetConn) Exec(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	c.execs = append(c.execs, fakeResetExec{
		sql:  sql,
		args: append([]any(nil), args...),
	})
	for token, err := range c.errorsBySQLToken {
		if strings.Contains(sql, token) {
			return pgconn.CommandTag{}, err
		}
	}
	return pgconn.CommandTag{}, nil
}

func (c *fakeResetConn) Release() {
	c.releaseCount++
}

func (c *fakeResetConn) executedSQLContaining(token string) bool {
	for _, exec := range c.execs {
		if strings.Contains(exec.sql, token) {
			return true
		}
	}
	return false
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
