package database

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

// Decimal wraps shopspring/decimal for GORM compatibility
// It implements the necessary interfaces for database scanning and value conversion
type Decimal struct {
	decimal.Decimal
}

// NewDecimal creates a new Decimal from a shopspring decimal
func NewDecimal(d decimal.Decimal) Decimal {
	return Decimal{Decimal: d}
}

// NewDecimalFromFloat creates a new Decimal from a float64
func NewDecimalFromFloat(f float64) Decimal {
	return Decimal{Decimal: decimal.NewFromFloat(f)}
}

// NewDecimalFromString creates a new Decimal from a string
func NewDecimalFromString(s string) (Decimal, error) {
	d, err := decimal.NewFromString(s)
	if err != nil {
		return Decimal{}, err
	}
	return Decimal{Decimal: d}, nil
}

// Zero returns a zero-value Decimal
func DecimalZero() Decimal {
	return Decimal{Decimal: decimal.Zero}
}

// Scan implements sql.Scanner interface
func (d *Decimal) Scan(value interface{}) error {
	if value == nil {
		d.Decimal = decimal.Zero
		return nil
	}

	switch v := value.(type) {
	case []byte:
		dec, err := decimal.NewFromString(string(v))
		if err != nil {
			return fmt.Errorf("failed to scan decimal from bytes: %w", err)
		}
		d.Decimal = dec
	case string:
		dec, err := decimal.NewFromString(v)
		if err != nil {
			return fmt.Errorf("failed to scan decimal from string: %w", err)
		}
		d.Decimal = dec
	case float64:
		d.Decimal = decimal.NewFromFloat(v)
	case int64:
		d.Decimal = decimal.NewFromInt(v)
	default:
		return fmt.Errorf("unsupported type for decimal: %T", value)
	}
	return nil
}

// Value implements driver.Valuer interface
func (d Decimal) Value() (driver.Value, error) {
	return d.String(), nil
}

// GormDataType returns the GORM data type for this field
func (Decimal) GormDataType() string {
	return "NUMERIC(28,8)"
}

// GormDBDataType returns the database data type based on dialect
func (Decimal) GormDBDataType(db *gorm.DB, field *schema.Field) string {
	switch db.Name() {
	case "postgres":
		return "NUMERIC(28,8)"
	case "mysql":
		return "DECIMAL(28,8)"
	case "sqlite":
		return "REAL"
	default:
		return "NUMERIC(28,8)"
	}
}

// MarshalJSON implements json.Marshaler
func (d Decimal) MarshalJSON() ([]byte, error) {
	return d.Decimal.MarshalJSON()
}

// UnmarshalJSON implements json.Unmarshaler
func (d *Decimal) UnmarshalJSON(data []byte) error {
	return d.Decimal.UnmarshalJSON(data)
}

// JSONB represents a PostgreSQL JSONB column
type JSONB map[string]interface{}

// Scan implements sql.Scanner interface
func (j *JSONB) Scan(value interface{}) error {
	if value == nil {
		*j = nil
		return nil
	}

	var bytes []byte
	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		return fmt.Errorf("unsupported type for JSONB: %T", value)
	}

	if len(bytes) == 0 {
		*j = nil
		return nil
	}

	result := make(map[string]interface{})
	if err := json.Unmarshal(bytes, &result); err != nil {
		return fmt.Errorf("failed to unmarshal JSONB: %w", err)
	}
	*j = result
	return nil
}

// Value implements driver.Valuer interface
func (j JSONB) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}
	return json.Marshal(j)
}

// GormDataType returns the GORM data type for this field
func (JSONB) GormDataType() string {
	return "JSONB"
}

// GormDBDataType returns the database data type based on dialect
func (JSONB) GormDBDataType(db *gorm.DB, field *schema.Field) string {
	switch db.Name() {
	case "postgres":
		return "JSONB"
	case "mysql":
		return "JSON"
	case "sqlite":
		return "TEXT"
	default:
		return "JSONB"
	}
}

// JSONBRaw represents a raw JSONB column that preserves the original JSON
type JSONBRaw json.RawMessage

// Scan implements sql.Scanner interface
func (j *JSONBRaw) Scan(value interface{}) error {
	if value == nil {
		*j = nil
		return nil
	}

	switch v := value.(type) {
	case []byte:
		*j = make([]byte, len(v))
		copy(*j, v)
	case string:
		*j = []byte(v)
	default:
		return fmt.Errorf("unsupported type for JSONBRaw: %T", value)
	}
	return nil
}

// Value implements driver.Valuer interface
func (j JSONBRaw) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}
	return []byte(j), nil
}

// GormDataType returns the GORM data type for this field
func (JSONBRaw) GormDataType() string {
	return "JSONB"
}

// GormDBDataType returns the database data type based on dialect
func (JSONBRaw) GormDBDataType(db *gorm.DB, field *schema.Field) string {
	switch db.Name() {
	case "postgres":
		return "JSONB"
	case "mysql":
		return "JSON"
	case "sqlite":
		return "TEXT"
	default:
		return "JSONB"
	}
}

// MarshalJSON implements json.Marshaler
func (j JSONBRaw) MarshalJSON() ([]byte, error) {
	if j == nil {
		return []byte("null"), nil
	}
	return j, nil
}

// UnmarshalJSON implements json.Unmarshaler
func (j *JSONBRaw) UnmarshalJSON(data []byte) error {
	*j = make([]byte, len(data))
	copy(*j, data)
	return nil
}
