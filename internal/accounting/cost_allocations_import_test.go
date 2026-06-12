package accounting

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	costAllocationTestCostCenterID        = "11111111-1111-4111-8111-111111111111"
	costAllocationTestJournalLineID       = "22222222-2222-4222-8222-222222222222"
	costAllocationTestOtherJournalLineID  = "33333333-3333-4333-8333-333333333333"
	costAllocationTestMissingCostCenterID = "44444444-4444-4444-8444-444444444444"
)

type costAllocationImportListErrorRepository struct {
	CostCenterRepository
}

func (costAllocationImportListErrorRepository) List(_ context.Context, _, _ string, _ bool) ([]CostCenter, error) {
	return nil, errors.New("list unavailable")
}

func newCostAllocationImportMockRepository() *MockCostCenterRepository {
	repo := NewMockCostCenterRepository()
	repo.CostCenters[costAllocationTestCostCenterID] = &CostCenter{
		ID:       costAllocationTestCostCenterID,
		TenantID: "tenant-1",
		Code:     "OPS",
		Name:     "Operations",
		IsActive: true,
	}
	return repo
}

func TestService_ImportCostAllocationsCSV(t *testing.T) {
	t.Run("imports rows by resolving cost center code", func(t *testing.T) {
		repo := newCostAllocationImportMockRepository()
		svc := NewCostCenterServiceWithRepository(repo)

		result, err := svc.ImportCostAllocationsCSV(context.Background(), "tenant_1", "tenant-1", &ImportCostAllocationsRequest{
			FileName: "cost-allocations.csv",
			CSVContent: "cost_center,journal_line_id,allocation_amount,percentage,date,memo\n" +
				"OPS," + costAllocationTestJournalLineID + ",125.50,75,2026-03-31,Shared hosting\n",
		})

		require.NoError(t, err)
		assert.Equal(t, "cost-allocations.csv", result.FileName)
		assert.Equal(t, 1, result.RowsProcessed)
		assert.Equal(t, 1, result.AllocationsImported)
		assert.Zero(t, result.RowsSkipped)
		assert.Nil(t, result.Errors)

		require.Len(t, repo.Allocations[costAllocationTestCostCenterID], 1)
		allocation := repo.Allocations[costAllocationTestCostCenterID][0]
		assert.Equal(t, "tenant-1", allocation.TenantID)
		assert.Equal(t, costAllocationTestCostCenterID, allocation.CostCenterID)
		assert.Equal(t, costAllocationTestJournalLineID, allocation.JournalEntryLineID)
		assert.True(t, decimal.RequireFromString("125.50").Equal(allocation.Amount))
		require.NotNil(t, allocation.AllocationPercentage)
		assert.True(t, decimal.NewFromInt(75).Equal(*allocation.AllocationPercentage))
		assert.Equal(t, time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC), allocation.AllocationDate)
		assert.Equal(t, "Shared hosting", allocation.Notes)
	})

	t.Run("requires csv content", func(t *testing.T) {
		svc := NewCostCenterServiceWithRepository(NewMockCostCenterRepository())

		result, err := svc.ImportCostAllocationsCSV(context.Background(), "tenant_1", "tenant-1", nil)

		require.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "csv_content is required")
	})

	t.Run("returns parser errors", func(t *testing.T) {
		svc := NewCostCenterServiceWithRepository(NewMockCostCenterRepository())

		result, err := svc.ImportCostAllocationsCSV(context.Background(), "tenant_1", "tenant-1", &ImportCostAllocationsRequest{
			CSVContent: "\"cost_center_code,journal_entry_line_id,amount,allocation_date\n",
		})

		require.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "parse csv header")
	})

	t.Run("rejects header-only csv", func(t *testing.T) {
		svc := NewCostCenterServiceWithRepository(NewMockCostCenterRepository())

		result, err := svc.ImportCostAllocationsCSV(context.Background(), "tenant_1", "tenant-1", &ImportCostAllocationsRequest{
			CSVContent: "cost_center_code,journal_entry_line_id,amount,allocation_date\n",
		})

		require.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "no cost allocations found in CSV")
	})

	t.Run("wraps cost center list errors", func(t *testing.T) {
		svc := NewCostCenterServiceWithRepository(costAllocationImportListErrorRepository{})

		result, err := svc.ImportCostAllocationsCSV(context.Background(), "tenant_1", "tenant-1", &ImportCostAllocationsRequest{
			CSVContent: "cost_center_code,journal_entry_line_id,amount,allocation_date\nOPS," +
				costAllocationTestJournalLineID + ",10.00,2026-03-31\n",
		})

		require.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "list cost centers: list unavailable")
	})

	t.Run("records build errors and continues", func(t *testing.T) {
		repo := newCostAllocationImportMockRepository()
		svc := NewCostCenterServiceWithRepository(repo)

		result, err := svc.ImportCostAllocationsCSV(context.Background(), "tenant_1", "tenant-1", &ImportCostAllocationsRequest{
			CSVContent: "cost_center_code,journal_entry_line_id,amount,allocation_date\n" +
				"OPS,,10.00,2026-03-31\n" +
				"OPS," + costAllocationTestJournalLineID + ",10.00,2026-03-31\n",
		})

		require.NoError(t, err)
		assert.Equal(t, 2, result.RowsProcessed)
		assert.Equal(t, 1, result.AllocationsImported)
		assert.Equal(t, 1, result.RowsSkipped)
		require.Len(t, result.Errors, 1)
		assert.Equal(t, 2, result.Errors[0].Row)
		assert.Equal(t, "OPS", result.Errors[0].CostCenterCode)
		assert.Contains(t, result.Errors[0].Message, "journal_entry_line_id is required")
	})

	t.Run("records create errors and continues", func(t *testing.T) {
		repo := newCostAllocationImportMockRepository()
		svc := NewCostCenterServiceWithRepository(repo)

		result, err := svc.ImportCostAllocationsCSV(context.Background(), "tenant_1", "tenant-1", &ImportCostAllocationsRequest{
			CSVContent: "cost_center_id,cost_center_code,journal_entry_line_id,amount,allocation_date\n" +
				costAllocationTestMissingCostCenterID + ",," + costAllocationTestJournalLineID + ",10.00,2026-03-31\n" +
				",OPS," + costAllocationTestOtherJournalLineID + ",20.00,2026-04-01\n",
		})

		require.NoError(t, err)
		assert.Equal(t, 2, result.RowsProcessed)
		assert.Equal(t, 1, result.AllocationsImported)
		assert.Equal(t, 1, result.RowsSkipped)
		require.Len(t, result.Errors, 1)
		assert.Equal(t, 2, result.Errors[0].Row)
		assert.Equal(t, costAllocationTestMissingCostCenterID, result.Errors[0].CostCenterID)
		assert.Contains(t, result.Errors[0].Message, "cost center not found")
	})
}

func TestParseCostAllocationImportRows(t *testing.T) {
	t.Run("parses aliased semicolon headers and skips blank rows", func(t *testing.T) {
		rows, err := parseCostAllocationImportRows("\ufeff Cost Center ; Journal Line ID ; Allocation Amount ; percentage ; Date ; Memo ; custom field\n" +
			" OPS ; " + costAllocationTestJournalLineID + " ; 1,250.50 ; 25 ; 2026-03-31 ; Shared hosting ; ignored\n" +
			" ; ; ; ; ; ; \n" +
			costAllocationTestCostCenterID + " ; " + costAllocationTestOtherJournalLineID + " ; 75.00 ; ; 2026-04-01 ; Direct ; ignored\n")

		require.NoError(t, err)
		require.Len(t, rows, 2)

		assert.Equal(t, 2, rows[0].rowNumber)
		assert.Equal(t, "OPS", rows[0].values["cost_center_code"])
		assert.Equal(t, costAllocationTestJournalLineID, rows[0].values["journal_entry_line_id"])
		assert.Equal(t, "1,250.50", rows[0].values["amount"])
		assert.Equal(t, "25", rows[0].values["allocation_percentage"])
		assert.Equal(t, "2026-03-31", rows[0].values["allocation_date"])
		assert.Equal(t, "Shared hosting", rows[0].values["notes"])
		assert.Equal(t, "ignored", rows[0].values["custom_field"])

		assert.Equal(t, 4, rows[1].rowNumber)
		assert.Equal(t, costAllocationTestCostCenterID, rows[1].values["cost_center_code"])
		assert.Equal(t, costAllocationTestOtherJournalLineID, rows[1].values["journal_entry_line_id"])
		assert.Equal(t, "75.00", rows[1].values["amount"])
		assert.Empty(t, rows[1].values["allocation_percentage"])
		assert.Equal(t, "2026-04-01", rows[1].values["allocation_date"])
		assert.Equal(t, "Direct", rows[1].values["notes"])
	})

	t.Run("allows header-only csv", func(t *testing.T) {
		rows, err := parseCostAllocationImportRows("cost_center_code,journal_entry_line_id,amount,allocation_date\n")

		require.NoError(t, err)
		assert.Empty(t, rows)
	})

	t.Run("ignores blank header columns", func(t *testing.T) {
		rows, err := parseCostAllocationImportRows("cost_center_code,journal_entry_line_id,amount,allocation_date,\n" +
			"OPS," + costAllocationTestJournalLineID + ",10.00,2026-03-31,ignored\n")

		require.NoError(t, err)
		require.Len(t, rows, 1)
		assert.Equal(t, "OPS", rows[0].values["cost_center_code"])
		assert.Equal(t, costAllocationTestJournalLineID, rows[0].values["journal_entry_line_id"])
		assert.Equal(t, "10.00", rows[0].values["amount"])
		assert.Equal(t, "2026-03-31", rows[0].values["allocation_date"])
		_, ok := rows[0].values[""]
		assert.False(t, ok)
	})

	t.Run("requires content", func(t *testing.T) {
		rows, err := parseCostAllocationImportRows(" \t\n ")

		require.Error(t, err)
		assert.Nil(t, rows)
		assert.Contains(t, err.Error(), "csv_content is required")
	})

	t.Run("requires cost center id or code column", func(t *testing.T) {
		rows, err := parseCostAllocationImportRows("journal_entry_line_id,amount,allocation_date\n" +
			costAllocationTestJournalLineID + ",10.00,2026-03-31\n")

		require.Error(t, err)
		assert.Nil(t, rows)
		assert.Contains(t, err.Error(), "missing required columns: cost_center_id or cost_center_code")
	})

	t.Run("requires journal line amount and date columns", func(t *testing.T) {
		rows, err := parseCostAllocationImportRows("cost_center_code,journal_entry_line_id,amount\nOPS," +
			costAllocationTestJournalLineID + ",10.00\n")

		require.Error(t, err)
		assert.Nil(t, rows)
		assert.Contains(t, err.Error(), "missing required columns: journal_entry_line_id, amount, allocation_date")
	})

	t.Run("reports malformed csv headers", func(t *testing.T) {
		rows, err := parseCostAllocationImportRows("\"cost_center_code,journal_entry_line_id,amount,allocation_date\n")

		require.Error(t, err)
		assert.Nil(t, rows)
		assert.Contains(t, err.Error(), "parse csv header")
	})

	t.Run("reports malformed csv rows", func(t *testing.T) {
		rows, err := parseCostAllocationImportRows("cost_center_code,journal_entry_line_id,amount,allocation_date\nOPS,\"" +
			costAllocationTestJournalLineID + ",10.00,2026-03-31\n")

		require.Error(t, err)
		assert.Nil(t, rows)
		assert.Contains(t, err.Error(), "parse csv row 2")
	})
}

func TestBuildCreateCostAllocationRequestFromImportRow(t *testing.T) {
	t.Run("resolves cost center code and normalizes row values", func(t *testing.T) {
		req, err := buildCreateCostAllocationRequestFromImportRow(costAllocationImportRow{
			values: map[string]string{
				"cost_center_code":      " ops ",
				"journal_entry_line_id": " " + costAllocationTestJournalLineID + " ",
				"amount":                " 12.50 ",
				"allocation_percentage": " 33.3 ",
				"allocation_date":       " 2026-03-31 ",
				"notes":                 "  Shared hosting  ",
			},
		}, map[string]string{"ops": costAllocationTestCostCenterID})

		require.NoError(t, err)
		assert.Equal(t, costAllocationTestCostCenterID, req.CostCenterID)
		assert.Equal(t, costAllocationTestJournalLineID, req.JournalEntryLineID)
		assert.True(t, decimal.RequireFromString("12.50").Equal(req.Amount))
		require.NotNil(t, req.AllocationPercentage)
		assert.True(t, decimal.RequireFromString("33.3").Equal(*req.AllocationPercentage))
		assert.Equal(t, time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC), req.AllocationDate)
		assert.Equal(t, "Shared hosting", req.Notes)
	})

	t.Run("uses provided cost center id before code lookup", func(t *testing.T) {
		req, err := buildCreateCostAllocationRequestFromImportRow(costAllocationImportRow{
			values: map[string]string{
				"cost_center_id":        " " + costAllocationTestCostCenterID + " ",
				"cost_center_code":      "MISSING",
				"journal_entry_line_id": costAllocationTestJournalLineID,
				"amount":                "10.00",
				"allocation_percentage": "",
				"allocation_date":       "2026-03-31",
				"notes":                 "",
			},
		}, map[string]string{})

		require.NoError(t, err)
		assert.Equal(t, costAllocationTestCostCenterID, req.CostCenterID)
		assert.Nil(t, req.AllocationPercentage)
	})

	t.Run("rejects invalid cost center id", func(t *testing.T) {
		req, err := buildCreateCostAllocationRequestFromImportRow(costAllocationImportRow{
			values: map[string]string{"cost_center_id": "legacy-cc"},
		}, nil)

		require.Error(t, err)
		assert.Nil(t, req)
		assert.Contains(t, err.Error(), "cost_center_id must be a valid UUID")
	})

	t.Run("requires cost center id or code", func(t *testing.T) {
		req, err := buildCreateCostAllocationRequestFromImportRow(costAllocationImportRow{
			values: map[string]string{"cost_center_code": " \t "},
		}, nil)

		require.Error(t, err)
		assert.Nil(t, req)
		assert.Contains(t, err.Error(), "cost_center_id or cost_center_code is required")
	})

	t.Run("rejects unknown cost center code", func(t *testing.T) {
		req, err := buildCreateCostAllocationRequestFromImportRow(costAllocationImportRow{
			values: map[string]string{"cost_center_code": "MISSING"},
		}, map[string]string{})

		require.Error(t, err)
		assert.Nil(t, req)
		assert.Contains(t, err.Error(), "cost_center_code \"MISSING\" was not found")
	})

	t.Run("requires journal entry line id", func(t *testing.T) {
		req, err := buildCreateCostAllocationRequestFromImportRow(costAllocationImportRow{
			values: map[string]string{
				"cost_center_id":        costAllocationTestCostCenterID,
				"journal_entry_line_id": " \t ",
			},
		}, nil)

		require.Error(t, err)
		assert.Nil(t, req)
		assert.Contains(t, err.Error(), "journal_entry_line_id is required")
	})

	t.Run("rejects invalid journal entry line id", func(t *testing.T) {
		req, err := buildCreateCostAllocationRequestFromImportRow(costAllocationImportRow{
			values: map[string]string{
				"cost_center_id":        costAllocationTestCostCenterID,
				"journal_entry_line_id": "legacy-line",
			},
		}, nil)

		require.Error(t, err)
		assert.Nil(t, req)
		assert.Contains(t, err.Error(), "journal_entry_line_id must be a valid UUID")
	})

	t.Run("rejects invalid amount", func(t *testing.T) {
		req, err := buildCreateCostAllocationRequestFromImportRow(costAllocationImportRow{
			values: map[string]string{
				"cost_center_id":        costAllocationTestCostCenterID,
				"journal_entry_line_id": costAllocationTestJournalLineID,
				"amount":                "not-a-number",
			},
		}, nil)

		require.Error(t, err)
		assert.Nil(t, req)
		assert.Contains(t, err.Error(), "amount must be a decimal")
	})

	t.Run("rejects invalid allocation percentage", func(t *testing.T) {
		req, err := buildCreateCostAllocationRequestFromImportRow(costAllocationImportRow{
			values: map[string]string{
				"cost_center_id":        costAllocationTestCostCenterID,
				"journal_entry_line_id": costAllocationTestJournalLineID,
				"amount":                "10.00",
				"allocation_percentage": "bad",
			},
		}, nil)

		require.Error(t, err)
		assert.Nil(t, req)
		assert.Contains(t, err.Error(), "allocation_percentage must be a decimal")
	})

	t.Run("rejects invalid allocation date", func(t *testing.T) {
		req, err := buildCreateCostAllocationRequestFromImportRow(costAllocationImportRow{
			values: map[string]string{
				"cost_center_id":        costAllocationTestCostCenterID,
				"journal_entry_line_id": costAllocationTestJournalLineID,
				"amount":                "10.00",
				"allocation_date":       "31/03/2026",
			},
		}, nil)

		require.Error(t, err)
		assert.Nil(t, req)
		assert.Contains(t, err.Error(), "allocation_date must use YYYY-MM-DD")
	})
}

func TestParseCostAllocationImportPositiveDecimal(t *testing.T) {
	t.Run("accepts positive decimal", func(t *testing.T) {
		amount, err := parseCostAllocationImportPositiveDecimal("amount", " 125.50 ")

		require.NoError(t, err)
		assert.True(t, decimal.RequireFromString("125.50").Equal(amount))
	})

	t.Run("requires value", func(t *testing.T) {
		amount, err := parseCostAllocationImportPositiveDecimal("amount", " \t ")

		require.Error(t, err)
		assert.True(t, amount.IsZero())
		assert.Contains(t, err.Error(), "amount is required")
	})

	t.Run("rejects invalid decimal", func(t *testing.T) {
		amount, err := parseCostAllocationImportPositiveDecimal("amount", "not-a-number")

		require.Error(t, err)
		assert.True(t, amount.IsZero())
		assert.Contains(t, err.Error(), "amount must be a decimal")
	})

	t.Run("rejects zero decimal", func(t *testing.T) {
		amount, err := parseCostAllocationImportPositiveDecimal("amount", "0")

		require.Error(t, err)
		assert.True(t, amount.IsZero())
		assert.Contains(t, err.Error(), "amount must be greater than zero")
	})
}

func TestParseCostAllocationImportPercentage(t *testing.T) {
	t.Run("allows blank percentage", func(t *testing.T) {
		percentage, err := parseCostAllocationImportPercentage(" \t ")

		require.NoError(t, err)
		assert.Nil(t, percentage)
	})

	t.Run("accepts decimal percentage", func(t *testing.T) {
		percentage, err := parseCostAllocationImportPercentage(" 99.5 ")

		require.NoError(t, err)
		require.NotNil(t, percentage)
		assert.True(t, decimal.RequireFromString("99.5").Equal(*percentage))
	})

	t.Run("rejects invalid percentage", func(t *testing.T) {
		percentage, err := parseCostAllocationImportPercentage("bad")

		require.Error(t, err)
		assert.Nil(t, percentage)
		assert.Contains(t, err.Error(), "allocation_percentage must be a decimal")
	})

	t.Run("rejects negative percentage", func(t *testing.T) {
		percentage, err := parseCostAllocationImportPercentage("-0.01")

		require.Error(t, err)
		assert.Nil(t, percentage)
		assert.Contains(t, err.Error(), "allocation_percentage must be between 0 and 100")
	})

	t.Run("rejects percentage above 100", func(t *testing.T) {
		percentage, err := parseCostAllocationImportPercentage("100.01")

		require.Error(t, err)
		assert.Nil(t, percentage)
		assert.Contains(t, err.Error(), "allocation_percentage must be between 0 and 100")
	})
}

func TestParseCostAllocationImportDate(t *testing.T) {
	t.Run("accepts trimmed ISO date", func(t *testing.T) {
		parsed, err := parseCostAllocationImportDate(" 2026-03-31 ")

		require.NoError(t, err)
		assert.Equal(t, time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC), parsed)
	})

	t.Run("requires allocation date", func(t *testing.T) {
		parsed, err := parseCostAllocationImportDate(" \t ")

		require.Error(t, err)
		assert.True(t, parsed.IsZero())
		assert.Contains(t, err.Error(), "allocation_date is required")
	})

	t.Run("rejects non ISO date", func(t *testing.T) {
		parsed, err := parseCostAllocationImportDate("31/03/2026")

		require.Error(t, err)
		assert.True(t, parsed.IsZero())
		assert.Contains(t, err.Error(), "allocation_date must use YYYY-MM-DD")
	})
}

func TestCostAllocationImportHeader(t *testing.T) {
	assert.Equal(t, "cost_center_code", canonicalCostAllocationImportHeader("Cost Center"))
	assert.Equal(t, "cost_center_code", canonicalCostAllocationImportHeader("cc_code"))
	assert.Equal(t, "journal_entry_line_id", canonicalCostAllocationImportHeader("journal_line_id"))
	assert.Equal(t, "amount", canonicalCostAllocationImportHeader("Allocation Amount"))
	assert.Equal(t, "allocation_percentage", canonicalCostAllocationImportHeader("allocation_percent"))
	assert.Equal(t, "allocation_date", canonicalCostAllocationImportHeader("Date"))
	assert.Equal(t, "notes", canonicalCostAllocationImportHeader("Memo"))
	assert.Equal(t, "custom_field", canonicalCostAllocationImportHeader("Custom Field"))
}
