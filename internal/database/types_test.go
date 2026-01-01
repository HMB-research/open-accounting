package database

import (
	"encoding/json"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/schema"
)

func TestNewDecimal(t *testing.T) {
	d := decimal.NewFromFloat(123.45)
	result := NewDecimal(d)
	assert.True(t, result.Equal(d))
}

func TestNewDecimalFromFloat(t *testing.T) {
	result := NewDecimalFromFloat(123.45)
	expected := decimal.NewFromFloat(123.45)
	assert.True(t, result.Equal(expected))
}

func TestNewDecimalFromString(t *testing.T) {
	t.Run("valid string", func(t *testing.T) {
		result, err := NewDecimalFromString("123.45")
		require.NoError(t, err)
		expected := decimal.NewFromFloat(123.45)
		assert.True(t, result.Equal(expected))
	})

	t.Run("invalid string", func(t *testing.T) {
		_, err := NewDecimalFromString("not-a-number")
		assert.Error(t, err)
	})
}

func TestDecimalZero(t *testing.T) {
	result := DecimalZero()
	assert.True(t, result.IsZero())
}

func TestDecimal_Scan(t *testing.T) {
	tests := []struct {
		name        string
		input       interface{}
		expected    decimal.Decimal
		expectError bool
	}{
		{
			name:     "nil value",
			input:    nil,
			expected: decimal.Zero,
		},
		{
			name:     "bytes value",
			input:    []byte("123.45"),
			expected: decimal.NewFromFloat(123.45),
		},
		{
			name:     "string value",
			input:    "678.90",
			expected: decimal.NewFromFloat(678.90),
		},
		{
			name:     "float64 value",
			input:    float64(111.22),
			expected: decimal.NewFromFloat(111.22),
		},
		{
			name:     "int64 value",
			input:    int64(500),
			expected: decimal.NewFromInt(500),
		},
		{
			name:        "invalid bytes",
			input:       []byte("not-a-number"),
			expectError: true,
		},
		{
			name:        "invalid string",
			input:       "invalid",
			expectError: true,
		},
		{
			name:        "unsupported type",
			input:       true,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var d Decimal
			err := d.Scan(tt.input)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.True(t, d.Equal(tt.expected), "got %s, want %s", d.String(), tt.expected.String())
			}
		})
	}
}

func TestDecimal_Value(t *testing.T) {
	d := NewDecimalFromFloat(123.45)
	val, err := d.Value()
	require.NoError(t, err)
	assert.Equal(t, "123.45", val)
}

func TestDecimal_GormDataType(t *testing.T) {
	var d Decimal
	assert.Equal(t, "NUMERIC(28,8)", d.GormDataType())
}

func TestDecimal_MarshalJSON(t *testing.T) {
	d := NewDecimalFromFloat(123.45)
	data, err := d.MarshalJSON()
	require.NoError(t, err)
	assert.Equal(t, `"123.45"`, string(data))
}

func TestDecimal_UnmarshalJSON(t *testing.T) {
	var d Decimal
	err := d.UnmarshalJSON([]byte(`"678.90"`))
	require.NoError(t, err)
	expected := decimal.NewFromFloat(678.90)
	assert.True(t, d.Equal(expected))
}

func TestJSONB_Scan(t *testing.T) {
	tests := []struct {
		name        string
		input       interface{}
		expected    JSONB
		expectError bool
	}{
		{
			name:     "nil value",
			input:    nil,
			expected: nil,
		},
		{
			name:     "bytes value",
			input:    []byte(`{"key": "value"}`),
			expected: JSONB{"key": "value"},
		},
		{
			name:     "string value",
			input:    `{"foo": "bar"}`,
			expected: JSONB{"foo": "bar"},
		},
		{
			name:     "empty bytes",
			input:    []byte{},
			expected: nil,
		},
		{
			name:        "invalid json bytes",
			input:       []byte(`{invalid}`),
			expectError: true,
		},
		{
			name:        "unsupported type",
			input:       123,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var j JSONB
			err := j.Scan(tt.input)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				if tt.expected == nil {
					assert.Nil(t, j)
				} else {
					assert.Equal(t, tt.expected, j)
				}
			}
		})
	}
}

func TestJSONB_Value(t *testing.T) {
	t.Run("nil value", func(t *testing.T) {
		var j JSONB
		val, err := j.Value()
		require.NoError(t, err)
		assert.Nil(t, val)
	})

	t.Run("with data", func(t *testing.T) {
		j := JSONB{"key": "value"}
		val, err := j.Value()
		require.NoError(t, err)
		// Value returns JSON bytes
		var result map[string]interface{}
		err = json.Unmarshal(val.([]byte), &result)
		require.NoError(t, err)
		assert.Equal(t, "value", result["key"])
	})
}

func TestJSONB_GormDataType(t *testing.T) {
	var j JSONB
	assert.Equal(t, "JSONB", j.GormDataType())
}

func TestJSONBRaw_Scan(t *testing.T) {
	tests := []struct {
		name        string
		input       interface{}
		expected    JSONBRaw
		expectError bool
	}{
		{
			name:     "nil value",
			input:    nil,
			expected: nil,
		},
		{
			name:     "bytes value",
			input:    []byte(`{"raw": true}`),
			expected: JSONBRaw(`{"raw": true}`),
		},
		{
			name:     "string value",
			input:    `{"str": "val"}`,
			expected: JSONBRaw(`{"str": "val"}`),
		},
		{
			name:        "unsupported type",
			input:       123,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var j JSONBRaw
			err := j.Scan(tt.input)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, j)
			}
		})
	}
}

func TestJSONBRaw_Value(t *testing.T) {
	t.Run("nil value", func(t *testing.T) {
		var j JSONBRaw
		val, err := j.Value()
		require.NoError(t, err)
		assert.Nil(t, val)
	})

	t.Run("with data", func(t *testing.T) {
		j := JSONBRaw(`{"test": 123}`)
		val, err := j.Value()
		require.NoError(t, err)
		assert.Equal(t, []byte(`{"test": 123}`), val)
	})
}

func TestJSONBRaw_GormDataType(t *testing.T) {
	var j JSONBRaw
	assert.Equal(t, "JSONB", j.GormDataType())
}

func TestJSONBRaw_MarshalJSON(t *testing.T) {
	t.Run("nil value", func(t *testing.T) {
		var j JSONBRaw
		data, err := j.MarshalJSON()
		require.NoError(t, err)
		assert.Equal(t, []byte("null"), data)
	})

	t.Run("with data", func(t *testing.T) {
		j := JSONBRaw(`{"test": true}`)
		data, err := j.MarshalJSON()
		require.NoError(t, err)
		assert.Equal(t, []byte(`{"test": true}`), data)
	})
}

func TestJSONBRaw_UnmarshalJSON(t *testing.T) {
	var j JSONBRaw
	err := j.UnmarshalJSON([]byte(`{"unmarshal": "test"}`))
	require.NoError(t, err)
	assert.Equal(t, JSONBRaw(`{"unmarshal": "test"}`), j)
}

// Mock dialector for testing GormDBDataType
type mockDialector struct {
	name string
}

func (m mockDialector) Name() string {
	return m.name
}

func (m mockDialector) Initialize(*gorm.DB) error {
	return nil
}

func (m mockDialector) Migrator(*gorm.DB) gorm.Migrator {
	return nil
}

func (m mockDialector) DataTypeOf(*schema.Field) string {
	return ""
}

func (m mockDialector) DefaultValueOf(*schema.Field) clause.Expression {
	return nil
}

func (m mockDialector) BindVarTo(writer clause.Writer, stmt *gorm.Statement, v interface{}) {}

func (m mockDialector) QuoteTo(writer clause.Writer, str string) {}

func (m mockDialector) Explain(sql string, vars ...interface{}) string {
	return sql
}

func TestDecimal_GormDBDataType(t *testing.T) {
	tests := []struct {
		dialect  string
		expected string
	}{
		{"postgres", "NUMERIC(28,8)"},
		{"mysql", "DECIMAL(28,8)"},
		{"sqlite", "REAL"},
		{"unknown", "NUMERIC(28,8)"},
	}

	for _, tt := range tests {
		t.Run(tt.dialect, func(t *testing.T) {
			db := &gorm.DB{Config: &gorm.Config{}}
			db.Config.Dialector = mockDialector{name: tt.dialect}

			var d Decimal
			result := d.GormDBDataType(db, nil)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestJSONB_GormDBDataType(t *testing.T) {
	tests := []struct {
		dialect  string
		expected string
	}{
		{"postgres", "JSONB"},
		{"mysql", "JSON"},
		{"sqlite", "TEXT"},
		{"unknown", "JSONB"},
	}

	for _, tt := range tests {
		t.Run(tt.dialect, func(t *testing.T) {
			db := &gorm.DB{Config: &gorm.Config{}}
			db.Config.Dialector = mockDialector{name: tt.dialect}

			var j JSONB
			result := j.GormDBDataType(db, nil)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestJSONBRaw_GormDBDataType(t *testing.T) {
	tests := []struct {
		dialect  string
		expected string
	}{
		{"postgres", "JSONB"},
		{"mysql", "JSON"},
		{"sqlite", "TEXT"},
		{"unknown", "JSONB"},
	}

	for _, tt := range tests {
		t.Run(tt.dialect, func(t *testing.T) {
			db := &gorm.DB{Config: &gorm.Config{}}
			db.Config.Dialector = mockDialector{name: tt.dialect}

			var j JSONBRaw
			result := j.GormDBDataType(db, nil)
			assert.Equal(t, tt.expected, result)
		})
	}
}
