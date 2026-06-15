package database

import (
	"fmt"

	"gorm.io/gorm"
)

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
