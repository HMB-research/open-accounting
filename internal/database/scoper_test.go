//go:build gorm

package database

import "testing"

func TestTenantTableRejectsInvalidIdentifiers(t *testing.T) {
	db, err := TenantTable(nil, "tenant-demo", "contacts")
	if err == nil {
		t.Fatal("expected invalid schema error")
	}
	if db != nil {
		t.Fatal("expected nil db when identifier validation fails")
	}
}

func TestTenantTableRejectsNilDB(t *testing.T) {
	db, err := TenantTable(nil, "tenant_demo", "contacts")
	if err == nil {
		t.Fatal("expected nil db error")
	}
	if db != nil {
		t.Fatal("expected nil db on error")
	}
}
