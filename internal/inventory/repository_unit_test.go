package inventory

import (
	"context"
	"testing"
)

func TestQualifiedInventoryTableBuildsQualifiedTableReference(t *testing.T) {
	table, err := qualifiedInventoryTable("tenant_demo", "products")
	if err != nil {
		t.Fatalf("qualifiedInventoryTable returned error: %v", err)
	}

	expected := `"tenant_demo"."products"`
	if table != expected {
		t.Fatalf("expected %q, got %q", expected, table)
	}
}

func TestGenerateCodeRejectsInvalidSchemaName(t *testing.T) {
	repo := &GORMRepository{}

	_, err := repo.GenerateCode(context.Background(), "tenant-demo", "tenant-1")
	if err == nil {
		t.Fatal("expected invalid schema error")
	}
}

func TestTenantTableRejectsInvalidSchemaName(t *testing.T) {
	repo := &GORMRepository{}
	ctx := context.Background()

	if _, err := repo.tenantTable(ctx, "tenant-demo", "products"); err == nil {
		t.Fatal("expected tenantTable to reject invalid schema")
	}
}
