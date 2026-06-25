package reports

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/shopspring/decimal"
)

func TestServiceWave4GenerateCashFlowRepositoryErrors(t *testing.T) {
	ctx := context.Background()
	req := &CashFlowRequest{StartDate: "2026-01-01", EndDate: "2026-01-31"}

	tests := []struct {
		name    string
		setup   func(*MockRepository)
		wantErr string
	}{
		{
			name: "journal entries",
			setup: func(repo *MockRepository) {
				repo.GetEntriesErr = errors.New("entries unavailable")
			},
			wantErr: "get journal entries",
		},
		{
			name: "opening cash",
			setup: func(repo *MockRepository) {
				repo.GetCashBalanceErr = errors.New("cash unavailable")
			},
			wantErr: "get opening cash",
		},
		{
			name: "cash flow mapping",
			setup: func(repo *MockRepository) {
				repo.GetCashFlowMappingErr = errors.New("mapping unavailable")
			},
			wantErr: "get cash flow mapping",
		},
		{
			name: "persistent mapping conflict",
			setup: func(repo *MockRepository) {
				repo.CashFlowMapping = CashFlowMappingOverrides{
					OperatingAccountCodes: []string{"1400"},
					InvestingAccountCodes: []string{"1400"},
				}
			},
			wantErr: "cannot be assigned",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := NewMockRepository()
			tt.setup(repo)
			service := NewServiceWithRepository(repo)

			statement, err := service.GenerateCashFlowStatement(ctx, "tenant-1", "schema_tenant1", req)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("GenerateCashFlowStatement() error = %v, want containing %q", err, tt.wantErr)
			}
			if statement != nil {
				t.Fatalf("GenerateCashFlowStatement() statement = %#v, want nil", statement)
			}
		})
	}
}

func TestServiceWave4CashFlowMappingRepositoryErrors(t *testing.T) {
	ctx := context.Background()

	t.Run("get wraps repository error", func(t *testing.T) {
		repo := NewMockRepository()
		repo.GetCashFlowMappingErr = errors.New("read failed")
		service := NewServiceWithRepository(repo)

		mapping, err := service.GetCashFlowMapping(ctx, "tenant-1")
		if err == nil || !strings.Contains(err.Error(), "get cash flow mapping") {
			t.Fatalf("GetCashFlowMapping() error = %v, want get cash flow mapping", err)
		}
		if mapping != nil {
			t.Fatalf("GetCashFlowMapping() mapping = %#v, want nil", mapping)
		}
	})

	t.Run("update wraps repository error", func(t *testing.T) {
		repo := NewMockRepository()
		repo.UpdateCashFlowMappingErr = errors.New("write failed")
		service := NewServiceWithRepository(repo)

		mapping, err := service.UpdateCashFlowMapping(ctx, "tenant-1", CashFlowMappingOverrides{
			OperatingAccountCodes: []string{"4000"},
		})
		if err == nil || !strings.Contains(err.Error(), "update cash flow mapping") {
			t.Fatalf("UpdateCashFlowMapping() error = %v, want update cash flow mapping", err)
		}
		if mapping != nil {
			t.Fatalf("UpdateCashFlowMapping() mapping = %#v, want nil", mapping)
		}
	})
}

func TestServiceWave4SalesMarginZeroRevenueAndFallbackContactKey(t *testing.T) {
	lines := []SalesMarginLine{
		{ContactName: "No ID", Revenue: decimal.Zero, Cost: decimal.NewFromInt(5)},
		{ContactID: "contact-2", ContactName: "Beta", Revenue: decimal.NewFromInt(100), Cost: decimal.NewFromInt(80)},
		{ContactID: "contact-1", ContactName: "Acme", Revenue: decimal.NewFromInt(100), Cost: decimal.NewFromInt(80)},
	}

	contacts := aggregateSalesMarginByContact(lines)

	if got := calculateMarginPercent(decimal.NewFromInt(10), decimal.Zero); !got.Equal(decimal.Zero) {
		t.Fatalf("calculateMarginPercent(zero revenue) = %s, want 0", got)
	}
	if len(contacts) != 3 {
		t.Fatalf("aggregateSalesMarginByContact() len = %d, want 3", len(contacts))
	}
	if contacts[0].ContactName != "Acme" || contacts[1].ContactName != "Beta" {
		t.Fatalf("contacts = %#v, want equal margins sorted by contact name", contacts[:2])
	}
	if contacts[2].ContactName != "No ID" || !contacts[2].Margin.Equal(decimal.NewFromInt(-5)) {
		t.Fatalf("fallback contact aggregate = %#v, want No ID with -5 margin", contacts[2])
	}
}
