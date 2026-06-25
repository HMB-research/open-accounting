package database

import (
	"testing"

	"gorm.io/gorm"
)

func TestTenantTableRejectsNilDBDefault(t *testing.T) {
	db, err := TenantTable(nil, "tenant_demo", "contacts")
	if err == nil {
		t.Fatal("expected nil db error")
	}
	if db != nil {
		t.Fatal("expected nil db on error")
	}
}

func TestTenantTableReturnsQualifiedTableDefault(t *testing.T) {
	wrapper := newDryRunGormWrapper(t)

	db, err := TenantTable(wrapper.DB, "tenant_demo", "contacts")
	if err != nil {
		t.Fatalf("TenantTable returned error: %v", err)
	}
	if db == nil {
		t.Fatal("TenantTable returned nil db")
	}
}

func TestTenantTableRejectsInvalidSchemaDefault(t *testing.T) {
	db, err := TenantTable(&gorm.DB{}, "tenant-demo", "contacts")
	if err == nil {
		t.Fatal("expected invalid schema error")
	}
	if db != nil {
		t.Fatal("expected nil db on invalid schema")
	}
}

func TestTenantTableRejectsInvalidTableDefault(t *testing.T) {
	db, err := TenantTable(&gorm.DB{}, "tenant_demo", "bad-table")
	if err == nil {
		t.Fatal("expected invalid table error")
	}
	if db != nil {
		t.Fatal("expected nil db on invalid table")
	}
}
