package tenant

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"gorm.io/gorm"
)

func stubNewGormDBFromPool(t *testing.T, fn func(context.Context, *pgxpool.Pool) (*gorm.DB, error)) {
	t.Helper()
	original := newGormDBFromPool
	newGormDBFromPool = fn
	t.Cleanup(func() {
		newGormDBFromPool = original
	})
}

func TestNewRepositoryUsesInjectedGormDB(t *testing.T) {
	expectedDB := &gorm.DB{}
	pool := new(pgxpool.Pool)
	var called bool
	stubNewGormDBFromPool(t, func(ctx context.Context, got *pgxpool.Pool) (*gorm.DB, error) {
		if ctx == nil {
			t.Fatal("expected non-nil context")
		}
		if got != pool {
			t.Fatalf("newGormDBFromPool got %#v, want %#v", got, pool)
		}
		called = true
		return expectedDB, nil
	})

	repo := NewRepository(pool)

	if !called {
		t.Fatal("expected newGormDBFromPool to be called")
	}
	if repo == nil {
		t.Fatal("NewRepository() = nil, want repository")
	}
	if repo.db != expectedDB {
		t.Fatalf("NewRepository().db = %#v, want %#v", repo.db, expectedDB)
	}
}

func TestNewRepositoryPanicsOnGormPoolError(t *testing.T) {
	expectedErr := errors.New("pool unavailable")
	stubNewGormDBFromPool(t, func(context.Context, *pgxpool.Pool) (*gorm.DB, error) {
		return nil, expectedErr
	})

	defer func() {
		panicValue := recover()
		if panicValue == nil {
			t.Fatal("expected panic")
		}
		if got := panicValue.(error).Error(); got != "create tenant GORM repository: pool unavailable" {
			t.Fatalf("panic error = %q, want %q", got, "create tenant GORM repository: pool unavailable")
		}
	}()

	_ = NewRepository(new(pgxpool.Pool))
}
