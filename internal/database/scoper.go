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
			db = db.Session(&gorm.Session{})
			_ = db.AddError(err)
			return db
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
