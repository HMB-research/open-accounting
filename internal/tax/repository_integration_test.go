//go:build integration

package tax

import (
	"context"
	"testing"
	"time"

	"github.com/HMB-research/open-accounting/internal/testutil"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

func TestPostgresRepository_SaveDeclaration(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Ensure schema exists
	err := repo.EnsureSchema(ctx, tenant.SchemaName)
	if err != nil {
		t.Fatalf("EnsureSchema failed: %v", err)
	}

	// Create KMD declaration
	now := time.Now()
	decl := &KMDDeclaration{
		ID:             uuid.New().String(),
		TenantID:       tenant.ID,
		Year:           now.Year(),
		Month:          int(now.Month()),
		TotalOutputVAT: decimal.NewFromFloat(1000),
		TotalInputVAT:  decimal.NewFromFloat(300),
		Status:         "DRAFT",
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	err = repo.SaveDeclaration(ctx, tenant.SchemaName, decl)
	if err != nil {
		t.Fatalf("SaveDeclaration failed: %v", err)
	}

	// Verify it was created
	retrieved, err := repo.GetDeclaration(ctx, tenant.SchemaName, tenant.ID, now.Year(), int(now.Month()))
	if err != nil {
		t.Fatalf("GetDeclaration failed: %v", err)
	}

	if retrieved == nil {
		t.Fatal("expected KMD declaration, got nil")
	}

	if !retrieved.TotalOutputVAT.Equal(decl.TotalOutputVAT) {
		t.Errorf("expected TotalOutputVAT %s, got %s", decl.TotalOutputVAT, retrieved.TotalOutputVAT)
	}
	if !retrieved.TotalInputVAT.Equal(decl.TotalInputVAT) {
		t.Errorf("expected TotalInputVAT %s, got %s", decl.TotalInputVAT, retrieved.TotalInputVAT)
	}
}

func TestPostgresRepository_ListDeclarations(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Ensure schema exists
	err := repo.EnsureSchema(ctx, tenant.SchemaName)
	if err != nil {
		t.Fatalf("EnsureSchema failed: %v", err)
	}

	// Create multiple KMD declarations
	now := time.Now()
	for i := 1; i <= 3; i++ {
		decl := &KMDDeclaration{
			ID:             uuid.New().String(),
			TenantID:       tenant.ID,
			Year:           now.Year(),
			Month:          i,
			TotalOutputVAT: decimal.NewFromFloat(float64(i * 100)),
			TotalInputVAT:  decimal.NewFromFloat(float64(i * 30)),
			Status:         "DRAFT",
			CreatedAt:      now,
			UpdatedAt:      now,
		}

		err = repo.SaveDeclaration(ctx, tenant.SchemaName, decl)
		if err != nil {
			t.Fatalf("SaveDeclaration for month %d failed: %v", i, err)
		}
	}

	// List all declarations
	declarations, err := repo.ListDeclarations(ctx, tenant.SchemaName, tenant.ID)
	if err != nil {
		t.Fatalf("ListDeclarations failed: %v", err)
	}

	if len(declarations) != 3 {
		t.Errorf("expected 3 declarations, got %d", len(declarations))
	}
}

func TestPostgresRepository_GetDeclaration_NotFound(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Ensure schema exists
	err := repo.EnsureSchema(ctx, tenant.SchemaName)
	if err != nil {
		t.Fatalf("EnsureSchema failed: %v", err)
	}

	// Try to get non-existent KMD
	decl, err := repo.GetDeclaration(ctx, tenant.SchemaName, tenant.ID, 2099, 12)
	if err != nil {
		t.Fatalf("GetDeclaration failed: %v", err)
	}

	if decl != nil {
		t.Error("expected nil for non-existent KMD, got a declaration")
	}
}
