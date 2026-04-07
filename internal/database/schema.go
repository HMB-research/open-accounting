package database

import (
	"fmt"
	"regexp"
	"strings"
)

var identifierPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// ValidateIdentifier ensures a schema/table identifier is safe to interpolate.
func ValidateIdentifier(identifier string) error {
	if !identifierPattern.MatchString(identifier) {
		return fmt.Errorf("invalid SQL identifier %q", identifier)
	}
	return nil
}

// QuoteIdentifier safely quotes a validated SQL identifier.
func QuoteIdentifier(identifier string) (string, error) {
	if err := ValidateIdentifier(identifier); err != nil {
		return "", err
	}
	return `"` + strings.ReplaceAll(identifier, `"`, `""`) + `"`, nil
}

// QualifiedTable returns a schema-qualified table reference safe for interpolation.
func QualifiedTable(schemaName, tableName string) (string, error) {
	quotedSchema, err := QuoteIdentifier(schemaName)
	if err != nil {
		return "", err
	}
	quotedTable, err := QuoteIdentifier(tableName)
	if err != nil {
		return "", err
	}
	return quotedSchema + "." + quotedTable, nil
}
