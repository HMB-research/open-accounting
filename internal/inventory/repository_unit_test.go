package inventory

import (
	"context"
	"testing"
)

func TestQualifyQueryBuildsQualifiedTableReference(t *testing.T) {
	repo := &PostgresRepository{}

	query, err := repo.qualifyQuery("tenant_demo", "products", `SELECT * FROM %s`)
	if err != nil {
		t.Fatalf("qualifyQuery returned error: %v", err)
	}

	expected := `SELECT * FROM "tenant_demo"."products"`
	if query != expected {
		t.Fatalf("expected %q, got %q", expected, query)
	}
}

func TestGenerateCodeRejectsInvalidSchemaName(t *testing.T) {
	repo := &PostgresRepository{}

	_, err := repo.GenerateCode(context.Background(), "tenant-demo", "tenant-1")
	if err == nil {
		t.Fatal("expected invalid schema error")
	}
}

func TestQueryHelpersRejectInvalidSchemaName(t *testing.T) {
	repo := &PostgresRepository{}
	ctx := context.Background()

	if err := repo.execInTable(ctx, "tenant-demo", "products", `SELECT * FROM %s`); err == nil {
		t.Fatal("expected execInTable to reject invalid schema")
	}

	if _, err := repo.queryInTable(ctx, "tenant-demo", "products", `SELECT * FROM %s`); err == nil {
		t.Fatal("expected queryInTable to reject invalid schema")
	}

	if _, err := repo.queryRowInTable(ctx, "tenant-demo", "products", `SELECT * FROM %s`); err == nil {
		t.Fatal("expected queryRowInTable to reject invalid schema")
	}
}
