package reports

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/shopspring/decimal"
)

func TestReportsWave7CashFlowRequestValidation(t *testing.T) {
	service := NewServiceWithRepository(NewMockRepository())
	ctx := context.Background()

	tests := []struct {
		name string
		req  *CashFlowRequest
		want string
	}{
		{name: "bad method", req: &CashFlowRequest{Method: "hybrid"}, want: "cash flow method"},
		{name: "bad start date", req: &CashFlowRequest{StartDate: "bad", EndDate: "2026-01-31"}, want: "invalid start date"},
		{name: "bad end date", req: &CashFlowRequest{StartDate: "2026-01-01", EndDate: "bad"}, want: "invalid end date"},
		{name: "end before start", req: &CashFlowRequest{StartDate: "2026-02-01", EndDate: "2026-01-31"}, want: "end date must be on or after start date"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			statement, err := service.GenerateCashFlowStatement(ctx, "tenant-1", "tenant_demo", tt.req)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("GenerateCashFlowStatement() error = %v, want containing %q", err, tt.want)
			}
			if statement != nil {
				t.Fatalf("GenerateCashFlowStatement() statement = %#v, want nil", statement)
			}
		})
	}
}

func TestReportsWave7CashFlowMappingNormalizationErrors(t *testing.T) {
	service := NewServiceWithRepository(NewMockRepository())

	_, err := service.UpdateCashFlowMapping(context.Background(), "tenant-1", CashFlowMappingOverrides{
		OperatingAccountCodes: []string{"4000"},
		FinancingAccountCodes: []string{"4000"},
	})
	if err == nil || !strings.Contains(err.Error(), "cannot be assigned") {
		t.Fatalf("UpdateCashFlowMapping() error = %v, want mapping conflict", err)
	}

	repo := NewMockRepository()
	repo.CashFlowMapping = CashFlowMappingOverrides{
		OperatingAccountCodes: []string{"1200"},
		InvestingAccountCodes: []string{"1200"},
	}
	_, err = NewServiceWithRepository(repo).GetCashFlowMapping(context.Background(), "tenant-1")
	if err == nil || !strings.Contains(err.Error(), "cannot be assigned") {
		t.Fatalf("GetCashFlowMapping() error = %v, want mapping conflict", err)
	}

	repo = NewMockRepository()
	repo.UpdateCashFlowMappingErr = nil
	repo.CashFlowMapping = CashFlowMappingOverrides{OperatingAccountCodes: []string{"4000"}}
	got, err := NewServiceWithRepository(repo).UpdateCashFlowMapping(context.Background(), "tenant-1", CashFlowMappingOverrides{
		OperatingAccountCodes: []string{" 4000, 4000 ", "4100"},
	})
	if err != nil {
		t.Fatalf("UpdateCashFlowMapping() unexpected error: %v", err)
	}
	if strings.Join(got.OperatingAccountCodes, ",") != "4000,4100" {
		t.Fatalf("UpdateCashFlowMapping() normalized codes = %#v", got.OperatingAccountCodes)
	}
}

func TestReportsWave7CashFlowClassifiersHitSkippedAndNegativeBranches(t *testing.T) {
	service := NewServiceWithRepository(NewMockRepository())
	overrides := newCashFlowMappingOverrides(CashFlowMappingOverrides{
		OperatingAccountCodes: []string{"8899"},
		InvestingAccountCodes: []string{"9999"},
	})

	direct := service.classifyOperatingActivities([]JournalEntryWithLines{
		{Lines: []JournalLine{{AccountCode: "4000", AccountType: "REVENUE", Credit: decimal.NewFromInt(10)}}},
		{Lines: []JournalLine{
			{AccountCode: "1000", AccountType: "ASSET", Debit: decimal.NewFromInt(100)},
			{AccountCode: "4000", AccountType: "REVENUE", Credit: decimal.NewFromInt(100)},
		}},
		{Lines: []JournalLine{
			{AccountCode: "1000", AccountType: "ASSET", Credit: decimal.NewFromInt(5)},
			{AccountCode: "9999", AccountType: "ASSET", Debit: decimal.NewFromInt(5)},
		}},
		{Lines: []JournalLine{
			{AccountCode: "1000", AccountType: "ASSET", Credit: decimal.NewFromInt(40)},
			{AccountCode: "5000", AccountType: "EXPENSE", Debit: decimal.NewFromInt(40)},
		}},
		{Lines: []JournalLine{
			{AccountCode: "1000", AccountType: "ASSET", Credit: decimal.NewFromInt(30)},
			{AccountCode: "5200", AccountType: "EXPENSE", Debit: decimal.NewFromInt(30)},
		}},
		{Lines: []JournalLine{
			{AccountCode: "1000", AccountType: "ASSET", Credit: decimal.NewFromInt(20)},
			{AccountCode: "2200", AccountType: "LIABILITY", Debit: decimal.NewFromInt(20)},
		}},
		{Lines: []JournalLine{
			{AccountCode: "1000", AccountType: "ASSET", Credit: decimal.NewFromInt(10)},
			{AccountCode: "5700", AccountType: "EXPENSE", Debit: decimal.NewFromInt(10)},
		}},
	}, overrides)
	if len(direct) != 5 {
		t.Fatalf("classifyOperatingActivities() len = %d, want 5: %#v", len(direct), direct)
	}

	indirect := service.classifyOperatingActivitiesIndirect([]JournalEntryWithLines{{
		Lines: []JournalLine{
			{AccountCode: "1000", AccountType: "ASSET", Debit: decimal.NewFromInt(1)},
			{AccountCode: "9999", AccountType: "ASSET", Debit: decimal.NewFromInt(1)},
			{AccountCode: "8899", AccountType: "OTHER", Credit: decimal.NewFromInt(5)},
			{AccountCode: "4000", AccountType: "REVENUE", Credit: decimal.NewFromInt(100)},
			{AccountCode: "5600", AccountType: "EXPENSE", Debit: decimal.NewFromInt(15)},
			{AccountCode: "1200", AccountType: "ASSET", Debit: decimal.NewFromInt(7)},
			{AccountCode: "1300", AccountType: "ASSET", Debit: decimal.NewFromInt(6)},
			{AccountCode: "2100", AccountType: "LIABILITY", Credit: decimal.NewFromInt(4)},
		},
	}}, overrides)
	if len(indirect) != 5 {
		t.Fatalf("classifyOperatingActivitiesIndirect() len = %d, want 5: %#v", len(indirect), indirect)
	}

	financing := service.classifyFinancingActivities([]JournalEntryWithLines{
		{Lines: []JournalLine{
			{AccountCode: "1000", AccountType: "ASSET", Credit: decimal.NewFromInt(50)},
			{AccountCode: "2400", AccountType: "LIABILITY", Debit: decimal.NewFromInt(50)},
		}},
		{Lines: []JournalLine{
			{AccountCode: "1000", AccountType: "ASSET", Debit: decimal.NewFromInt(25)},
			{AccountCode: "3100", AccountType: "EQUITY", Credit: decimal.NewFromInt(25)},
		}},
		{Lines: []JournalLine{
			{AccountCode: "1000", AccountType: "ASSET", Credit: decimal.NewFromInt(10)},
			{AccountCode: "3200", AccountType: "EQUITY", Debit: decimal.NewFromInt(10)},
		}},
	}, overrides)
	if len(financing) != 3 || financing[0].Code != CFFinLoansRepaid {
		t.Fatalf("classifyFinancingActivities() = %#v, want loan repayment, shares, dividends", financing)
	}
}

func TestReportsWave7ContactStatementAndSalesMarginErrors(t *testing.T) {
	service := NewServiceWithRepository(NewMockRepository())
	ctx := context.Background()

	statementReq := &ContactStatementRequest{ContactID: "contact-1", Type: string(BalanceTypeReceivable), StartDate: "2026-01-01", EndDate: "2026-01-31"}
	tests := []struct {
		name string
		req  *ContactStatementRequest
		repo *MockRepository
		want string
	}{
		{name: "missing contact", req: &ContactStatementRequest{Type: string(BalanceTypeReceivable), StartDate: "2026-01-01", EndDate: "2026-01-31"}, repo: NewMockRepository(), want: "contact_id is required"},
		{name: "bad start", req: &ContactStatementRequest{ContactID: "contact-1", Type: string(BalanceTypeReceivable), StartDate: "bad", EndDate: "2026-01-31"}, repo: NewMockRepository(), want: "invalid start_date"},
		{name: "bad end", req: &ContactStatementRequest{ContactID: "contact-1", Type: string(BalanceTypeReceivable), StartDate: "2026-01-01", EndDate: "bad"}, repo: NewMockRepository(), want: "invalid end_date"},
		{name: "end before start", req: &ContactStatementRequest{ContactID: "contact-1", Type: string(BalanceTypeReceivable), StartDate: "2026-02-01", EndDate: "2026-01-31"}, repo: NewMockRepository(), want: "end_date must be on or after start_date"},
		{name: "bad type", req: &ContactStatementRequest{ContactID: "contact-1", Type: "OTHER", StartDate: "2026-01-01", EndDate: "2026-01-31"}, repo: NewMockRepository(), want: "type must be RECEIVABLE"},
		{name: "contact error", req: statementReq, repo: &MockRepository{GetContactErr: errors.New("contact failed")}, want: "get contact"},
		{name: "opening error", req: statementReq, repo: &MockRepository{Contact: ContactInfo{ID: "contact-1"}, GetContactStatementOpeningErr: errors.New("opening failed")}, want: "get contact statement opening balance"},
		{name: "entries error", req: statementReq, repo: &MockRepository{Contact: ContactInfo{ID: "contact-1"}, GetContactStatementEntriesErr: errors.New("entries failed")}, want: "get contact statement entries"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewServiceWithRepository(tt.repo).GetContactStatement(ctx, "tenant-1", "tenant_demo", tt.req)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("GetContactStatement() error = %v, want containing %q", err, tt.want)
			}
			if got != nil {
				t.Fatalf("GetContactStatement() = %#v, want nil", got)
			}
		})
	}

	_, err := service.GetSalesMarginReport(ctx, "tenant-1", "tenant_demo", &SalesMarginRequest{StartDate: "bad", EndDate: "2026-01-31"})
	if err == nil || !strings.Contains(err.Error(), "invalid start_date") {
		t.Fatalf("GetSalesMarginReport() bad start error = %v", err)
	}
	_, err = service.GetSalesMarginReport(ctx, "tenant-1", "tenant_demo", &SalesMarginRequest{StartDate: "2026-01-01", EndDate: "bad"})
	if err == nil || !strings.Contains(err.Error(), "invalid end_date") {
		t.Fatalf("GetSalesMarginReport() bad end error = %v", err)
	}
	_, err = service.GetSalesMarginReport(ctx, "tenant-1", "tenant_demo", &SalesMarginRequest{StartDate: "2026-02-01", EndDate: "2026-01-31"})
	if err == nil || !strings.Contains(err.Error(), "end_date must be on or after start_date") {
		t.Fatalf("GetSalesMarginReport() reversed dates error = %v", err)
	}
	repo := NewMockRepository()
	repo.GetSalesMarginLinesErr = errors.New("margin failed")
	_, err = NewServiceWithRepository(repo).GetSalesMarginReport(ctx, "tenant-1", "tenant_demo", &SalesMarginRequest{StartDate: "2026-01-01", EndDate: "2026-01-31"})
	if err == nil || !strings.Contains(err.Error(), "get sales margin lines") {
		t.Fatalf("GetSalesMarginReport() repo error = %v", err)
	}
}
