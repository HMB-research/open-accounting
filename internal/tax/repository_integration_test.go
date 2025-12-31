//go:build integration

package tax

import (
	"context"
	"testing"
	"time"

	"github.com/HMB-research/open-accounting/internal/testutil"
	"github.com/shopspring/decimal"
)

func TestPostgresRepository_GenerateKMD(t *testing.T) {
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
		TenantID:          tenant.ID,
		Year:              now.Year(),
		Month:             int(now.Month()),
		TotalSalesVAT:     decimal.NewFromFloat(1000),
		TotalPurchasesVAT: decimal.NewFromFloat(300),
		NetVATPayable:     decimal.NewFromFloat(700),
		Status:            "draft",
		GeneratedAt:       now,
	}

	err = repo.GenerateKMD(ctx, tenant.ID, tenant.SchemaName, now.Year(), int(now.Month()), decl)
	if err != nil {
		t.Fatalf("GenerateKMD failed: %v", err)
	}

	// Verify it was created
	retrieved, err := repo.GetKMD(ctx, tenant.ID, tenant.SchemaName, now.Year(), int(now.Month()))
	if err != nil {
		t.Fatalf("GetKMD failed: %v", err)
	}

	if retrieved == nil {
		t.Fatal("expected KMD declaration, got nil")
	}

	if !retrieved.TotalSalesVAT.Equal(decl.TotalSalesVAT) {
		t.Errorf("expected TotalSalesVAT %s, got %s", decl.TotalSalesVAT, retrieved.TotalSalesVAT)
	}
	if !retrieved.NetVATPayable.Equal(decl.NetVATPayable) {
		t.Errorf("expected NetVATPayable %s, got %s", decl.NetVATPayable, retrieved.NetVATPayable)
	}
}

func TestPostgresRepository_ListKMD(t *testing.T) {
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
			TenantID:          tenant.ID,
			Year:              now.Year(),
			Month:             i,
			TotalSalesVAT:     decimal.NewFromFloat(float64(i * 100)),
			TotalPurchasesVAT: decimal.NewFromFloat(float64(i * 30)),
			NetVATPayable:     decimal.NewFromFloat(float64(i * 70)),
			Status:            "draft",
			GeneratedAt:       now,
		}

		err = repo.GenerateKMD(ctx, tenant.ID, tenant.SchemaName, now.Year(), i, decl)
		if err != nil {
			t.Fatalf("GenerateKMD for month %d failed: %v", i, err)
		}
	}

	// List all declarations
	declarations, err := repo.ListKMD(ctx, tenant.ID, tenant.SchemaName)
	if err != nil {
		t.Fatalf("ListKMD failed: %v", err)
	}

	if len(declarations) != 3 {
		t.Errorf("expected 3 declarations, got %d", len(declarations))
	}
}

func TestPostgresRepository_GetKMD_NotFound(t *testing.T) {
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
	decl, err := repo.GetKMD(ctx, tenant.ID, tenant.SchemaName, 2099, 12)
	if err != nil {
		t.Fatalf("GetKMD failed: %v", err)
	}

	if decl != nil {
		t.Error("expected nil for non-existent KMD, got a declaration")
	}
}
