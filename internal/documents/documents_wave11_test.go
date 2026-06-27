package documents

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type closeFailingTempFile struct {
	bytes.Buffer
}

func (closeFailingTempFile) Close() error {
	return errors.New("close failed")
}

func stubWave11GormDBFromPool(t *testing.T, fn func(context.Context, *pgxpool.Pool) (*gorm.DB, error)) {
	t.Helper()
	original := newGormDBFromPool
	newGormDBFromPool = fn
	t.Cleanup(func() {
		newGormDBFromPool = original
	})
}

func TestWave11NewRepositoryUsesInjectedGormDB(t *testing.T) {
	expectedDB := &gorm.DB{}
	pool := new(pgxpool.Pool)
	var called bool
	stubWave11GormDBFromPool(t, func(ctx context.Context, got *pgxpool.Pool) (*gorm.DB, error) {
		require.NotNil(t, ctx)
		require.Same(t, pool, got)
		called = true
		return expectedDB, nil
	})

	repo := NewRepository(pool)

	require.True(t, called)
	require.NotNil(t, repo)
	require.Same(t, expectedDB, repo.db)
}

func TestWave11NewRepositoryPanicsOnInjectedGormError(t *testing.T) {
	stubWave11GormDBFromPool(t, func(context.Context, *pgxpool.Pool) (*gorm.DB, error) {
		return nil, errors.New("pool unavailable")
	})

	require.PanicsWithError(t, "create documents GORM repository: pool unavailable", func() {
		_ = NewRepository(new(pgxpool.Pool))
	})
}

func TestWave11LocalStoreSaveTempFileFailures(t *testing.T) {
	store, err := NewLocalStore(t.TempDir())
	require.NoError(t, err)

	tempPath := filepath.Join(store.rootDir, "tenant", "directory-target.tmp")
	require.NoError(t, os.MkdirAll(tempPath, 0o750))

	err = store.Save(context.Background(), "tenant/directory-target", strings.NewReader("payload"))
	require.Error(t, err)
	require.Contains(t, err.Error(), "create temp document")

	original := createLocalStoreTempFile
	createLocalStoreTempFile = func(string) (localStoreTempFile, error) {
		return &closeFailingTempFile{}, nil
	}
	t.Cleanup(func() {
		createLocalStoreTempFile = original
	})

	err = store.Save(context.Background(), "tenant/close-error.txt", strings.NewReader("payload"))
	require.Error(t, err)
	require.Contains(t, err.Error(), "close document")
}
