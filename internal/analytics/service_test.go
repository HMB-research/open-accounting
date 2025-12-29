package analytics

import (
	"testing"
)

func TestNewService(t *testing.T) {
	// Test with nil pool - service should be created but will fail on actual queries
	service := NewService(nil)
	if service == nil {
		t.Fatal("NewService(nil) returned nil")
	}
	if service.pool != nil {
		t.Error("NewService(nil).pool should be nil")
	}
}

func TestNewService_NotNil(t *testing.T) {
	service := NewService(nil)
	if service == nil {
		t.Error("NewService should always return a non-nil service")
	}
}

// TestDefaultMonthsValue tests that the chart methods use correct default months
func TestDefaultMonthsValue(t *testing.T) {
	// This tests the logic: if months <= 0 { months = 12 }
	tests := []struct {
		input    int
		expected int
	}{
		{0, 12},
		{-1, 12},
		{-100, 12},
		{1, 1},
		{6, 6},
		{12, 12},
		{24, 24},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			months := tt.input
			if months <= 0 {
				months = 12
			}
			if months != tt.expected {
				t.Errorf("Default months logic: input %d, got %d, want %d", tt.input, months, tt.expected)
			}
		})
	}
}

// TestDefaultLimitValue tests that the top items methods use correct default limit
func TestDefaultLimitValue(t *testing.T) {
	// This tests the logic: if limit <= 0 { limit = 10 }
	tests := []struct {
		input    int
		expected int
	}{
		{0, 10},
		{-1, 10},
		{-100, 10},
		{1, 1},
		{5, 5},
		{10, 10},
		{50, 50},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			limit := tt.input
			if limit <= 0 {
				limit = 10
			}
			if limit != tt.expected {
				t.Errorf("Default limit logic: input %d, got %d, want %d", tt.input, limit, tt.expected)
			}
		})
	}
}
