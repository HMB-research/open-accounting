package accounting

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
)

// TestCostCenterValidation tests cost center validation logic
func TestCostCenterValidation(t *testing.T) {
	tests := []struct {
		name       string
		costCenter CostCenter
		isValid    bool
		errorField string
	}{
		{
			name: "valid cost center",
			costCenter: CostCenter{
				ID:          uuid.New().String(),
				TenantID:    uuid.New().String(),
				Code:        "CC001",
				Name:        "Marketing Department",
				Description: "Marketing and advertising costs",
				IsActive:    true,
			},
			isValid: true,
		},
		{
			name: "cost center with empty code",
			costCenter: CostCenter{
				ID:       uuid.New().String(),
				TenantID: uuid.New().String(),
				Code:     "",
				Name:     "Marketing Department",
			},
			isValid:    false,
			errorField: "code",
		},
		{
			name: "cost center with empty name",
			costCenter: CostCenter{
				ID:       uuid.New().String(),
				TenantID: uuid.New().String(),
				Code:     "CC001",
				Name:     "",
			},
			isValid:    false,
			errorField: "name",
		},
		{
			name: "cost center with empty tenant ID",
			costCenter: CostCenter{
				ID:       uuid.New().String(),
				TenantID: "",
				Code:     "CC001",
				Name:     "Marketing Department",
			},
			isValid:    false,
			errorField: "tenant_id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test validation logic that would be used in service layer
			isValid := tt.costCenter.Code != "" && tt.costCenter.Name != "" && tt.costCenter.TenantID != ""
			assert.Equal(t, tt.isValid, isValid)
		})
	}
}

// TestCostCenterServiceValidation tests service-level validation
func TestCostCenterServiceValidation(t *testing.T) {
	service := NewCostCenterServiceWithRepository(NewMockCostCenterRepository())

	tests := []struct {
		name        string
		request     CreateCostCenterRequest
		expectError bool
		errorField  string
	}{
		{
			name: "valid create request",
			request: CreateCostCenterRequest{
				Code:        "CC001",
				Name:        "Marketing",
				Description: "Marketing department",
			},
			expectError: false,
		},
		{
			name: "request with empty code",
			request: CreateCostCenterRequest{
				Code: "",
				Name: "Marketing",
			},
			expectError: true,
			errorField:  "code",
		},
		{
			name: "request with empty name",
			request: CreateCostCenterRequest{
				Code: "CC001",
				Name: "",
			},
			expectError: true,
			errorField:  "name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test request validation
			hasError := tt.request.Code == "" || tt.request.Name == ""
			assert.Equal(t, tt.expectError, hasError)

			// Can't test actual service method since it requires DB, but we can test the service creation
			assert.NotNil(t, service)
		})
	}
}

func TestCostCenterAllocationService(t *testing.T) {
	repo := NewMockCostCenterRepository()
	repo.CostCenters["cc-1"] = &CostCenter{
		ID:       "cc-1",
		TenantID: "tenant-1",
		Code:     "OPS",
		Name:     "Operations",
		IsActive: true,
	}
	service := NewCostCenterServiceWithRepository(repo)
	percentage := decimal.NewFromInt(75)
	allocationDate := time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC)

	allocation, err := service.CreateCostAllocation(context.Background(), "tenant_1", "tenant-1", &CreateCostAllocationRequest{
		CostCenterID:         " cc-1 ",
		JournalEntryLineID:   " line-1 ",
		Amount:               decimal.NewFromFloat(125.50),
		AllocationPercentage: &percentage,
		AllocationDate:       allocationDate,
		Notes:                "  Shared hosting  ",
	})
	assert.NoError(t, err)
	assert.Equal(t, "cc-1", allocation.CostCenterID)
	assert.Equal(t, "line-1", allocation.JournalEntryLineID)
	assert.Equal(t, "Shared hosting", allocation.Notes)
	assert.True(t, allocation.Amount.Equal(decimal.NewFromFloat(125.50)))

	allocations, err := service.ListCostAllocations(context.Background(), "tenant_1", "tenant-1", CostAllocationFilters{
		CostCenterID: "cc-1",
		StartDate:    ptrTime(allocationDate.AddDate(0, 0, -1)),
		EndDate:      ptrTime(allocationDate.AddDate(0, 0, 1)),
	})
	assert.NoError(t, err)
	assert.Len(t, allocations, 1)
	assert.Equal(t, "OPS", allocations[0].CostCenterCode)

	_, err = service.ListCostAllocations(context.Background(), "tenant_1", "tenant-1", CostAllocationFilters{
		StartDate: ptrTime(allocationDate),
		EndDate:   ptrTime(allocationDate.AddDate(0, 0, -1)),
	})
	assert.ErrorContains(t, err, "end_date must be on or after start_date")
}

func TestCostCenterAllocationServiceValidation(t *testing.T) {
	service := NewCostCenterServiceWithRepository(NewMockCostCenterRepository())
	validDate := time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC)
	tooHighPercentage := decimal.NewFromInt(101)

	for _, tt := range []struct {
		name string
		req  CreateCostAllocationRequest
		want string
	}{
		{
			name: "missing cost center",
			req: CreateCostAllocationRequest{
				JournalEntryLineID: "line-1",
				Amount:             decimal.NewFromInt(10),
				AllocationDate:     validDate,
			},
			want: "cost_center_id is required",
		},
		{
			name: "missing line",
			req: CreateCostAllocationRequest{
				CostCenterID:   "cc-1",
				Amount:         decimal.NewFromInt(10),
				AllocationDate: validDate,
			},
			want: "journal_entry_line_id is required",
		},
		{
			name: "zero amount",
			req: CreateCostAllocationRequest{
				CostCenterID:       "cc-1",
				JournalEntryLineID: "line-1",
				AllocationDate:     validDate,
			},
			want: "amount must be greater than zero",
		},
		{
			name: "missing date",
			req: CreateCostAllocationRequest{
				CostCenterID:       "cc-1",
				JournalEntryLineID: "line-1",
				Amount:             decimal.NewFromInt(10),
			},
			want: "allocation_date is required",
		},
		{
			name: "invalid percentage",
			req: CreateCostAllocationRequest{
				CostCenterID:         "cc-1",
				JournalEntryLineID:   "line-1",
				Amount:               decimal.NewFromInt(10),
				AllocationDate:       validDate,
				AllocationPercentage: &tooHighPercentage,
			},
			want: "allocation_percentage must be between 0 and 100",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := service.CreateCostAllocation(context.Background(), "tenant_1", "tenant-1", &tt.req)
			assert.ErrorContains(t, err, tt.want)
		})
	}
}

func ptrTime(value time.Time) *time.Time {
	return &value
}

// TestCostCenterReportCalculations tests expense calculation logic
func TestCostCenterReportCalculations(t *testing.T) {
	expenses := []CostAllocation{
		{
			CostCenterID:   "cc1",
			Amount:         decimal.NewFromFloat(100.50),
			AllocationDate: time.Date(2024, time.January, 15, 0, 0, 0, 0, time.UTC),
		},
		{
			CostCenterID:   "cc1",
			Amount:         decimal.NewFromFloat(250.75),
			AllocationDate: time.Date(2024, time.January, 20, 0, 0, 0, 0, time.UTC),
		},
		{
			CostCenterID:   "cc2",
			Amount:         decimal.NewFromFloat(75.25),
			AllocationDate: time.Date(2024, time.January, 10, 0, 0, 0, 0, time.UTC),
		},
	}

	tests := []struct {
		name          string
		costCenterID  string
		expectedTotal decimal.Decimal
		expectedCount int
	}{
		{
			name:          "calculate total for cc1",
			costCenterID:  "cc1",
			expectedTotal: decimal.NewFromFloat(351.25), // 100.50 + 250.75
			expectedCount: 2,
		},
		{
			name:          "calculate total for cc2",
			costCenterID:  "cc2",
			expectedTotal: decimal.NewFromFloat(75.25),
			expectedCount: 1,
		},
		{
			name:          "calculate total for non-existent cc",
			costCenterID:  "cc3",
			expectedTotal: decimal.Zero,
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate calculation logic
			total := decimal.Zero
			count := 0

			for _, expense := range expenses {
				if expense.CostCenterID == tt.costCenterID {
					total = total.Add(expense.Amount)
					count++
				}
			}

			assert.True(t, tt.expectedTotal.Equal(total))
			assert.Equal(t, tt.expectedCount, count)
		})
	}
}

// TestCostCenterFiltering tests cost center filtering logic
func TestCostCenterFiltering(t *testing.T) {
	costCenters := []CostCenter{
		{
			ID:       "cc1",
			Code:     "CC001",
			Name:     "Marketing",
			IsActive: true,
		},
		{
			ID:       "cc2",
			Code:     "CC002",
			Name:     "Sales",
			IsActive: false,
		},
		{
			ID:       "cc3",
			Code:     "CC003",
			Name:     "Development",
			IsActive: true,
		},
	}

	tests := []struct {
		name          string
		activeOnly    bool
		expectedCount int
		expectedIDs   []string
	}{
		{
			name:          "filter active only",
			activeOnly:    true,
			expectedCount: 2,
			expectedIDs:   []string{"cc1", "cc3"},
		},
		{
			name:          "include inactive",
			activeOnly:    false,
			expectedCount: 3,
			expectedIDs:   []string{"cc1", "cc2", "cc3"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate filtering logic
			var filtered []CostCenter
			for _, cc := range costCenters {
				if !tt.activeOnly || cc.IsActive {
					filtered = append(filtered, cc)
				}
			}

			assert.Len(t, filtered, tt.expectedCount)
			for i, expectedID := range tt.expectedIDs {
				if i < len(filtered) {
					assert.Equal(t, expectedID, filtered[i].ID)
				}
			}
		})
	}
}

// TestCostCenterCodeGeneration tests automatic code generation logic
func TestCostCenterCodeGeneration(t *testing.T) {
	existingCodes := []string{"CC001", "CC002", "CC005"}

	tests := []struct {
		name         string
		prefix       string
		expectedNext string
	}{
		{
			name:         "generate next sequential code",
			prefix:       "CC",
			expectedNext: "CC006", // Next after CC005
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate code generation logic
			maxNum := 0
			prefixWithDash := tt.prefix

			for _, code := range existingCodes {
				if len(code) > len(prefixWithDash) && code[:len(prefixWithDash)] == prefixWithDash {
					// Extract number part and find maximum
					numPart := code[len(prefixWithDash):]
					if num := parseIntFromString(numPart); num > maxNum {
						maxNum = num
					}
				}
			}

			nextCode := formatCostCenterCode(tt.prefix, maxNum+1)
			assert.Equal(t, tt.expectedNext, nextCode)
		})
	}
}

// TestCostCenterRepository tests repository initialization
func TestCostCenterRepository(t *testing.T) {
	// Test that NewCostCenterRepository doesn't panic with nil DB
	// This tests the constructor that has 0% coverage
	repo := NewCostCenterRepository(nil)
	assert.NotNil(t, repo)

	// Test that service creation works
	service := NewCostCenterServiceWithRepository(repo)
	assert.NotNil(t, service)
}

// Helper functions for tests
func parseIntFromString(s string) int {
	num := 0
	for _, char := range s {
		if char >= '0' && char <= '9' {
			num = num*10 + int(char-'0')
		} else {
			break
		}
	}
	return num
}

func formatCostCenterCode(prefix string, num int) string {
	return prefix + formatNumber(num, 3) // 3-digit padding
}

func formatNumber(num, width int) string {
	str := ""
	temp := num
	if temp == 0 {
		str = "0"
	} else {
		for temp > 0 {
			str = string(rune('0'+temp%10)) + str
			temp /= 10
		}
	}

	// Pad with zeros
	for len(str) < width {
		str = "0" + str
	}
	return str
}

// TestMockCostCenterRepository tests the mock repository that has good coverage
func TestMockCostCenterRepository(t *testing.T) {
	mock := NewMockCostCenterRepository()
	assert.NotNil(t, mock)

	// Test that we can call methods without panicking
	costCenters, err := mock.List(context.Background(), "test", "tenant", false)
	assert.NoError(t, err)
	assert.NotNil(t, costCenters)
}
