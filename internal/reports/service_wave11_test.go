package reports

import (
	"context"
	"strings"
	"testing"
)

type malformedCashFlowUpdateRepository struct {
	*MockRepository
}

func (r malformedCashFlowUpdateRepository) UpdateCashFlowMappingOverrides(context.Context, string, CashFlowMappingOverrides) (CashFlowMappingOverrides, error) {
	return CashFlowMappingOverrides{
		OperatingAccountCodes: []string{"4000"},
		InvestingAccountCodes: []string{"4000"},
	}, nil
}

func TestReportsWave11CashFlowMappingErrorBranches(t *testing.T) {
	ctx := context.Background()

	_, err := NewServiceWithRepository(malformedCashFlowUpdateRepository{MockRepository: NewMockRepository()}).
		UpdateCashFlowMapping(ctx, "tenant-1", CashFlowMappingOverrides{OperatingAccountCodes: []string{"4000"}})
	if err == nil || !strings.Contains(err.Error(), "cannot be assigned") {
		t.Fatalf("UpdateCashFlowMapping() error = %v, want normalized update conflict", err)
	}

	_, err = NewServiceWithRepository(NewMockRepository()).GenerateCashFlowStatement(ctx, "tenant-1", "tenant_demo", &CashFlowRequest{
		StartDate: "2026-01-01",
		EndDate:   "2026-01-31",
		MappingOverrides: CashFlowMappingOverrides{
			OperatingAccountCodes: []string{"4000"},
			InvestingAccountCodes: []string{"4000"},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "cannot be assigned") {
		t.Fatalf("GenerateCashFlowStatement() error = %v, want request mapping conflict", err)
	}
}

func TestReportsWave11CashFlowPredicateNegativeBranches(t *testing.T) {
	overrides := newCashFlowMappingOverrides(CashFlowMappingOverrides{})
	lines := []JournalLine{
		{AccountCode: "1000", AccountType: "ASSET", AccountName: "Cash"},
		{AccountCode: "1600", AccountType: "ASSET", AccountName: "Prepaid expenses"},
	}

	if hasRevenueOrReceivable(lines, overrides) {
		t.Fatalf("hasRevenueOrReceivable() = true, want false")
	}
	if hasOperatingPayableOrExpense(lines, overrides) {
		t.Fatalf("hasOperatingPayableOrExpense() = true, want false")
	}
}
