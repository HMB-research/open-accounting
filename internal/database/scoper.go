//go:build gorm

package database

import (
	"fmt"

	"gorm.io/gorm"
)

// SchemaScope is a legacy helper that sets search_path for a tenant schema.
// Deprecated: use TenantTable with explicit qualified tables instead.
func SchemaScope(schemaName string) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		if schemaName == "" || schemaName == "public" {
			return db
		}
		quotedSchema, err := QuoteIdentifier(schemaName)
		if err != nil {
			return db.AddError(err)
		}
		return db.Exec(fmt.Sprintf("SET search_path TO %s, public", quotedSchema))
	}
}

// TenantDB is a legacy helper for tenant-scoped GORM access.
// Deprecated: use TenantTable with explicit qualified table names instead.
func TenantDB(db *gorm.DB, schemaName string) *gorm.DB {
	if db == nil {
		return nil
	}
	if schemaName == "" || schemaName == "public" {
		return db
	}
	return db.Scopes(SchemaScope(schemaName))
}

// TenantTable returns a GORM handle bound to an explicit schema-qualified table.
func TenantTable(db *gorm.DB, schemaName, tableName string) (*gorm.DB, error) {
	if db == nil {
		return nil, fmt.Errorf("nil gorm DB")
	}
	qualifiedTable, err := QualifiedTable(schemaName, tableName)
	if err != nil {
		return nil, err
	}
	return db.Session(&gorm.Session{NewDB: true}).Table(qualifiedTable), nil
}
