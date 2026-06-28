package inventory

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/HMB-research/open-accounting/internal/accounting"
	"github.com/shopspring/decimal"
)

type inventoryWave7Accounts struct {
	accounts []accounting.Account
	err      error
}

func (f inventoryWave7Accounts) ListAccounts(_ context.Context, _, _ string, _ bool) ([]accounting.Account, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.accounts, nil
}

type inventoryWave7Ledger struct {
	inventoryWave7Accounts
	createErr error
	postErr   error
}

func (f *inventoryWave7Ledger) CreateJournalEntry(_ context.Context, _, tenantID string, _ *accounting.CreateJournalEntryRequest) (*accounting.JournalEntry, error) {
	if f.createErr != nil {
		return nil, f.createErr
	}
	return &accounting.JournalEntry{ID: "journal-1", TenantID: tenantID, EntryNumber: "JE-1"}, nil
}

func (f *inventoryWave7Ledger) PostJournalEntry(_ context.Context, _, _, _, _, _ string) error {
	return f.postErr
}

func TestInventoryWave7IssueAccountValidationErrors(t *testing.T) {
	ctx := context.Background()
	cogsID := "11111111-1111-4111-8111-111111111111"
	inventoryID := "22222222-2222-4222-8222-222222222222"
	service := NewServiceWithRepository(NewMockRepository())

	tests := []struct {
		name     string
		accounts inventoryWave7Accounts
		want     string
	}{
		{name: "list error", accounts: inventoryWave7Accounts{err: errors.New("ledger failed")}, want: "list accounts for issue accounting"},
		{name: "missing cogs", accounts: inventoryWave7Accounts{accounts: []accounting.Account{{ID: inventoryID, AccountType: accounting.AccountTypeAsset}}}, want: "cost_of_goods_sold_account_id was not found"},
		{name: "cogs wrong type", accounts: inventoryWave7Accounts{accounts: []accounting.Account{{ID: cogsID, AccountType: accounting.AccountTypeAsset}, {ID: inventoryID, AccountType: accounting.AccountTypeAsset}}}, want: "EXPENSE"},
		{name: "missing inventory", accounts: inventoryWave7Accounts{accounts: []accounting.Account{{ID: cogsID, AccountType: accounting.AccountTypeExpense}}}, want: "inventory_account_id was not found"},
		{name: "inventory wrong type", accounts: inventoryWave7Accounts{accounts: []accounting.Account{{ID: cogsID, AccountType: accounting.AccountTypeExpense}, {ID: inventoryID, AccountType: accounting.AccountTypeExpense}}}, want: "ASSET"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service.accounts = tt.accounts
			err := service.validateInventoryIssueAccounts(ctx, "tenant_demo", "tenant-1", cogsID, inventoryID)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("validateInventoryIssueAccounts() error = %v, want containing %q", err, tt.want)
			}
		})
	}
}

func TestInventoryWave7PostIssueAccountingErrors(t *testing.T) {
	ctx := context.Background()
	service := NewServiceWithRepository(NewMockRepository())
	req := &IssueStockRequest{PostToLedger: true, UserID: "user-1"}
	accountingLines := &InventoryIssueAccounting{
		SourceType:  inventoryIssueSourceTypeDefault,
		SourceID:    "issue-1",
		Reference:   "Issue",
		Description: "Issue stock",
		Lines: []InventoryIssueAccountingLine{{
			AccountID:   "11111111-1111-4111-8111-111111111111",
			DebitAmount: decimal.NewFromInt(5),
			Currency:    inventoryIssueAccountingCurrencyEUR,
		}},
	}

	if err := service.postInventoryIssueAccounting(ctx, "tenant_demo", "tenant-1", &IssueStockRequest{}, nil, time.Now()); err != nil {
		t.Fatalf("postInventoryIssueAccounting() non-posting error = %v", err)
	}
	if err := service.postInventoryIssueAccounting(ctx, "tenant_demo", "tenant-1", req, nil, time.Now()); err == nil || !strings.Contains(err.Error(), "issue accounting lines are required") {
		t.Fatalf("postInventoryIssueAccounting() nil lines error = %v", err)
	}
	if err := service.postInventoryIssueAccounting(ctx, "tenant_demo", "tenant-1", &IssueStockRequest{PostToLedger: true}, accountingLines, time.Now()); err == nil || !strings.Contains(err.Error(), "user id is required") {
		t.Fatalf("postInventoryIssueAccounting() missing user error = %v", err)
	}

	service.ledger = &inventoryWave7Ledger{createErr: errors.New("create failed")}
	if err := service.postInventoryIssueAccounting(ctx, "tenant_demo", "tenant-1", req, accountingLines, time.Now()); err == nil || !strings.Contains(err.Error(), "create inventory issue journal entry") {
		t.Fatalf("postInventoryIssueAccounting() create error = %v", err)
	}

	service.ledger = &inventoryWave7Ledger{postErr: errors.New("post failed")}
	if err := service.postInventoryIssueAccounting(ctx, "tenant_demo", "tenant-1", req, accountingLines, time.Now()); err == nil || !strings.Contains(err.Error(), "post inventory issue journal entry") {
		t.Fatalf("postInventoryIssueAccounting() post error = %v", err)
	}

	accountingLines.Posted = false
	service.ledger = &inventoryWave7Ledger{}
	if err := service.postInventoryIssueAccounting(ctx, "tenant_demo", "tenant-1", req, accountingLines, time.Now()); err != nil {
		t.Fatalf("postInventoryIssueAccounting() success error = %v", err)
	}
	if !accountingLines.Posted || accountingLines.JournalID != "journal-1" || accountingLines.JournalNo != "JE-1" {
		t.Fatalf("postInventoryIssueAccounting() did not mark accounting posted: %#v", accountingLines)
	}
}

func TestInventoryWave7ValuationCostBranches(t *testing.T) {
	product := Product{ID: "product-1", Name: "Widget", PurchasePrice: decimal.NewFromInt(10)}
	movements := []InventoryMovement{
		{TenantID: "other", MovementType: MovementTypeIn, Quantity: decimal.NewFromInt(5), UnitCost: decimal.NewFromInt(1)},
		{TenantID: "tenant-1", MovementType: MovementTypeOut, Quantity: decimal.NewFromInt(5), UnitCost: decimal.NewFromInt(99)},
		{TenantID: "tenant-1", MovementType: MovementTypeIn, Quantity: decimal.NewFromInt(3), TotalCost: decimal.NewFromInt(12)},
	}
	if got := weightedAverageInventoryUnitCost(product, movements, "tenant-1"); !got.Equal(decimal.NewFromInt(4)) {
		t.Fatalf("weightedAverageInventoryUnitCost() = %s, want 4", got)
	}
	if got := fifoInventoryUnitCost(product, movements, "tenant-1", decimal.Zero); !got.Equal(product.PurchasePrice) {
		t.Fatalf("fifoInventoryUnitCost(zero quantity) = %s, want purchase price", got)
	}

	createdEarly := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	createdLate := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
	fifoCost := fifoInventoryUnitCost(product, []InventoryMovement{
		{TenantID: "tenant-1", MovementType: MovementTypeIn, Quantity: decimal.NewFromInt(1), UnitCost: decimal.NewFromInt(3), CreatedAt: createdEarly},
		{TenantID: "tenant-1", MovementType: MovementTypeAdjustment, Quantity: decimal.NewFromInt(1), UnitCost: decimal.NewFromInt(6), CreatedAt: createdLate},
	}, "tenant-1", decimal.NewFromInt(3))
	if !fifoCost.GreaterThan(decimal.NewFromInt(6)) || !fifoCost.LessThan(decimal.NewFromInt(7)) {
		t.Fatalf("fifoInventoryUnitCost() = %s, want blended late, early, and fallback cost", fifoCost)
	}

	line := inventoryValuationLine(product, StockLevel{ProductID: product.ID, Quantity: decimal.NewFromInt(2)}, Warehouse{}, decimal.NewFromInt(4))
	if line.WarehouseCode != "UNASSIGNED" || line.WarehouseName != "Unassigned" || !line.InventoryValue.Equal(decimal.NewFromInt(8)) {
		t.Fatalf("inventoryValuationLine() = %#v", line)
	}
}
