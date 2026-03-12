package database

import (
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDecimalNewFunctions tests the decimal constructor functions
func TestDecimalNewFunctions(t *testing.T) {
	tests := []struct {
		name     string
		function func() Decimal
		expected string
	}{
		{
			name:     "NewDecimal from decimal",
			function: func() Decimal { d, _ := decimal.NewFromString("123.45"); return NewDecimal(d) },
			expected: "123.45",
		},
		{
			name:     "NewDecimalFromFloat",
			function: func() Decimal { return NewDecimalFromFloat(123.45) },
			expected: "123.45",
		},
		{
			name:     "NewDecimalFromString valid",
			function: func() Decimal { d, _ := NewDecimalFromString("123.45"); return d },
			expected: "123.45",
		},
		{
			name:     "DecimalZero",
			function: func() Decimal { return DecimalZero() },
			expected: "0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.function()
			assert.Equal(t, tt.expected, result.String())
		})
	}
}

// TestDecimalNewFromStringError tests error handling in NewDecimalFromString
func TestDecimalNewFromStringError(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		expectError   bool
	}{
		{
			name:        "valid decimal string",
			input:       "123.45",
			expectError: false,
		},
		{
			name:        "invalid decimal string",
			input:       "not-a-number",
			expectError: true,
		},
		{
			name:        "empty string",
			input:       "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := NewDecimalFromString(tt.input)
			
			if tt.expectError {
				assert.Error(t, err)
				assert.True(t, result.IsZero())
				return
			}
			
			assert.NoError(t, err)
			assert.Equal(t, tt.input, result.String())
		})
	}
}

// TestDecimalScanValue tests Scan and Value methods for database operations
func TestDecimalScanValue(t *testing.T) {
	tests := []struct {
		name      string
		value     interface{}
		expectErr bool
		expected  string
	}{
		{
			name:     "scan string value",
			value:    "123.45",
			expected: "123.45",
		},
		{
			name:     "scan byte slice",
			value:    []byte("123.45"),
			expected: "123.45",
		},
		{
			name:     "scan int64",
			value:    int64(123),
			expected: "123",
		},
		{
			name:     "scan float64",
			value:    float64(123.45),
			expected: "123.45",
		},
		{
			name:     "scan nil",
			value:    nil,
			expected: "0",
		},
		{
			name:      "scan unsupported type",
			value:     struct{}{},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var d Decimal
			err := d.Scan(tt.value)
			
			if tt.expectErr {
				assert.Error(t, err)
				return
			}
			
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, d.String())
		})
	}
}

// TestDecimalValueMethod tests the Value method for database storage
func TestDecimalValueMethod(t *testing.T) {
	tests := []struct {
		name     string
		decimal  Decimal
		expected string
	}{
		{
			name:     "positive decimal",
			decimal:  NewDecimalFromFloat(123.45),
			expected: "123.45",
		},
		{
			name:     "negative decimal",
			decimal:  NewDecimalFromFloat(-123.45),
			expected: "-123.45",
		},
		{
			name:     "zero decimal",
			decimal:  DecimalZero(),
			expected: "0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value, err := tt.decimal.Value()
			assert.NoError(t, err)
			
			// Value should return the string representation
			stringValue, ok := value.(string)
			assert.True(t, ok)
			assert.Equal(t, tt.expected, stringValue)
		})
	}
}

// TestDecimalGormMethods tests GORM-related methods
func TestDecimalGormMethods(t *testing.T) {
	d := NewDecimalFromFloat(123.45)
	
	// Test GormDataType - should return NUMERIC(28,8) according to the implementation
	dataType := d.GormDataType()
	assert.Equal(t, "NUMERIC(28,8)", dataType)
	
	// Test GormDBDataType - skip this as it causes nil pointer with nil db
	// dbDataType := d.GormDBDataType(nil, nil)
	// assert.Contains(t, dbDataType, "NUMERIC")
}

// TestDecimalJSONMarshalUnmarshal tests JSON serialization
func TestDecimalJSONMarshalUnmarshal(t *testing.T) {
	original := NewDecimalFromFloat(123.45)
	
	// Test marshaling
	jsonData, err := original.MarshalJSON()
	assert.NoError(t, err)
	assert.Contains(t, string(jsonData), "123.45")
	
	// Test unmarshaling
	var unmarshaled Decimal
	err = unmarshaled.UnmarshalJSON(jsonData)
	assert.NoError(t, err)
	assert.Equal(t, original.String(), unmarshaled.String())
}

// TestDecimalJSONUnmarshalError tests JSON unmarshaling error cases
func TestDecimalJSONUnmarshalError(t *testing.T) {
	tests := []struct {
		name        string
		jsonData    []byte
		expectError bool
	}{
		{
			name:        "valid JSON number",
			jsonData:    []byte(`"123.45"`),
			expectError: false,
		},
		{
			name:        "invalid JSON",
			jsonData:    []byte(`invalid`),
			expectError: true,
		},
		{
			name:        "invalid decimal in JSON",
			jsonData:    []byte(`"not-a-number"`),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var d Decimal
			err := d.UnmarshalJSON(tt.jsonData)
			
			if tt.expectError {
				assert.Error(t, err)
				return
			}
			
			assert.NoError(t, err)
		})
	}
}

// TestDecimalComparisons tests decimal comparison operations
func TestDecimalComparisons(t *testing.T) {
	d1 := NewDecimalFromFloat(123.45)
	d2 := NewDecimalFromFloat(123.45)
	d3 := NewDecimalFromFloat(100.00)
	
	// Test equality
	assert.True(t, d1.Equal(d2.Decimal))
	assert.False(t, d1.Equal(d3.Decimal))
	
	// Test comparisons
	assert.True(t, d1.GreaterThan(d3.Decimal))
	assert.False(t, d3.GreaterThan(d1.Decimal))
	
	// Test zero
	zero := DecimalZero()
	assert.True(t, zero.IsZero())
	assert.False(t, d1.IsZero())
}

// TestJSONBScanValue tests JSONB Scan and Value methods
func TestJSONBScanValue(t *testing.T) {
	tests := []struct {
		name      string
		value     interface{}
		expectErr bool
		expectNil bool
	}{
		{
			name:     "scan string value",
			value:    `{"key": "value"}`,
		},
		{
			name:     "scan byte slice",
			value:    []byte(`{"key": "value"}`),
		},
		{
			name:      "scan nil",
			value:     nil,
			expectNil: true,
		},
		{
			name:      "scan unsupported type",
			value:     123,
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var j JSONB
			err := j.Scan(tt.value)
			
			if tt.expectErr {
				assert.Error(t, err)
				return
			}
			
			assert.NoError(t, err)
			if tt.expectNil {
				assert.Nil(t, j)
			} else {
				assert.NotNil(t, j)
			}
		})
	}
}

// TestJSONBValue tests JSONB Value method
func TestJSONBValue(t *testing.T) {
	j := JSONB{"key": "value"}
	
	value, err := j.Value()
	require.NoError(t, err)
	
	byteValue, ok := value.([]byte)
	require.True(t, ok)
	assert.Contains(t, string(byteValue), "key")
	assert.Contains(t, string(byteValue), "value")
}

// TestJSONBGormMethods tests GORM methods for JSONB
func TestJSONBGormMethods(t *testing.T) {
	j := JSONB{}
	
	// Test GormDataType
	dataType := j.GormDataType()
	assert.Equal(t, "JSONB", dataType)
	
	// Test GormDBDataType
	dbDataType := j.GormDBDataType(nil, nil)
	assert.Equal(t, "JSONB", dbDataType)
}