package apitoken

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/HMB-research/open-accounting/internal/testutil"
)

func TestPostgresRepositoryTokenLifecycle(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	userID := testutil.CreateTestUser(t, pool, "apitoken-lifecycle@example.com")
	testutil.AddUserToTenant(t, pool, tenant.ID, userID, "owner")

	repo := NewPostgresRepository(pool)
	ctx := context.Background()
	rawToken := "oa_lifecycle_raw_token"
	token := &APIToken{
		ID:          uuid.New().String(),
		TenantID:    tenant.ID,
		UserID:      userID,
		Name:        "Lifecycle Token",
		TokenPrefix: rawToken[:14],
		CreatedAt:   time.Now().UTC(),
	}
	tokenHash := hashToken(rawToken)

	if err := repo.CreateToken(ctx, token, tokenHash); err != nil {
		t.Fatalf("CreateToken failed: %v", err)
	}

	tokens, err := repo.ListTokens(ctx, userID, tenant.ID)
	if err != nil {
		t.Fatalf("ListTokens failed: %v", err)
	}
	if len(tokens) != 1 || tokens[0].Name != "Lifecycle Token" {
		t.Fatalf("unexpected listed tokens: %+v", tokens)
	}

	record, err := repo.GetValidationRecord(ctx, tokenHash, time.Now().Add(time.Minute))
	if err != nil {
		t.Fatalf("GetValidationRecord failed: %v", err)
	}
	if record.UserID != userID || record.TenantID != tenant.ID || record.Role != "owner" {
		t.Fatalf("unexpected validation record: %+v", record)
	}

	lastUsedAt := time.Now().UTC().Add(2 * time.Minute)
	if err := repo.TouchToken(ctx, token.ID, lastUsedAt); err != nil {
		t.Fatalf("TouchToken failed: %v", err)
	}

	record, err = repo.GetValidationRecord(ctx, tokenHash, time.Now().Add(3*time.Minute))
	if err != nil {
		t.Fatalf("GetValidationRecord after touch failed: %v", err)
	}
	if record.LastUsedAt == nil || !record.LastUsedAt.After(time.Now()) {
		t.Fatalf("expected last_used_at to be updated, got %+v", record.LastUsedAt)
	}

	if err := repo.RevokeToken(ctx, userID, tenant.ID, token.ID, time.Now().UTC()); err != nil {
		t.Fatalf("RevokeToken failed: %v", err)
	}

	if _, err := repo.GetValidationRecord(ctx, tokenHash, time.Now().Add(4*time.Minute)); err != ErrTokenNotFound {
		t.Fatalf("expected ErrTokenNotFound after revoke, got %v", err)
	}
}

func TestPostgresRepositoryRevokeMissingToken(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	userID := testutil.CreateTestUser(t, pool, "apitoken-missing@example.com")
	testutil.AddUserToTenant(t, pool, tenant.ID, userID, "owner")

	repo := NewPostgresRepository(pool)
	err := repo.RevokeToken(context.Background(), userID, tenant.ID, uuid.New().String(), time.Now().UTC())
	if err != ErrTokenNotFound {
		t.Fatalf("expected ErrTokenNotFound, got %v", err)
	}
}

func TestNewServiceUsesRepository(t *testing.T) {
	svc := NewService(nil)
	if svc == nil || svc.repo == nil {
		t.Fatal("expected api token service with repository")
	}
}
