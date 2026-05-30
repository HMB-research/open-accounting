package expenses

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/HMB-research/open-accounting/internal/testutil"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

func TestPostgresRepositoryLifecycle(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	userID := testutil.CreateTestUser(t, pool, "expenses-integration@example.com")
	repo := NewRepository(pool)
	ctx := context.Background()

	expenseAccountID := findAccountID(t, ctx, pool, tenant.SchemaName, "5500")
	paymentAccountID := findAccountID(t, ctx, pool, tenant.SchemaName, "1000")

	nextNumber, err := repo.GenerateNumber(ctx, tenant.SchemaName, tenant.ID)
	if err != nil {
		t.Fatalf("GenerateNumber before create failed: %v", err)
	}
	if nextNumber != "EXP-00001" {
		t.Fatalf("expected first expense number EXP-00001, got %s", nextNumber)
	}

	now := time.Now().UTC().Truncate(time.Microsecond)
	expense := &Expense{
		ID:               uuid.NewString(),
		TenantID:         tenant.ID,
		ExpenseNumber:    nextNumber,
		ExpenseDate:      time.Date(2026, 5, 30, 0, 0, 0, 0, time.UTC),
		Merchant:         "Integration Cafe",
		Description:      "Team lunch",
		ExpenseAccountID: expenseAccountID,
		PaymentAccountID: paymentAccountID,
		Amount:           decimal.NewFromInt(42),
		Currency:         "EUR",
		ExchangeRate:     decimal.NewFromInt(1),
		BaseAmount:       decimal.NewFromInt(42),
		RequiresReceipt:  true,
		Status:           StatusDraft,
		CreatedAt:        now,
		CreatedBy:        userID,
		UpdatedAt:        now,
	}
	if err := repo.Create(ctx, tenant.SchemaName, expense); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	nextNumber, err = repo.GenerateNumber(ctx, tenant.SchemaName, tenant.ID)
	if err != nil {
		t.Fatalf("GenerateNumber after create failed: %v", err)
	}
	if nextNumber != "EXP-00002" {
		t.Fatalf("expected second expense number EXP-00002, got %s", nextNumber)
	}

	retrieved, err := repo.GetByID(ctx, tenant.SchemaName, tenant.ID, expense.ID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}
	if retrieved.Merchant != expense.Merchant {
		t.Fatalf("expected merchant %q, got %q", expense.Merchant, retrieved.Merchant)
	}
	if !retrieved.Amount.Equal(expense.Amount) {
		t.Fatalf("expected amount %s, got %s", expense.Amount, retrieved.Amount)
	}

	drafts, err := repo.List(ctx, tenant.SchemaName, tenant.ID, ListExpensesFilter{Status: StatusDraft, Limit: 10})
	if err != nil {
		t.Fatalf("List draft expenses failed: %v", err)
	}
	if len(drafts) != 1 || drafts[0].ID != expense.ID {
		t.Fatalf("expected draft list to contain created expense, got %#v", drafts)
	}

	submittedAt := now.Add(time.Minute)
	expense.Status = StatusSubmitted
	expense.SubmittedAt = &submittedAt
	expense.SubmittedBy = &userID
	if err := repo.Update(ctx, tenant.SchemaName, expense); err != nil {
		t.Fatalf("Update submitted status failed: %v", err)
	}

	submitted, err := repo.List(ctx, tenant.SchemaName, tenant.ID, ListExpensesFilter{Status: StatusSubmitted, Limit: 10})
	if err != nil {
		t.Fatalf("List submitted expenses failed: %v", err)
	}
	if len(submitted) != 1 || submitted[0].Status != StatusSubmitted {
		t.Fatalf("expected submitted list to contain updated expense, got %#v", submitted)
	}

	updated, err := repo.GetByID(ctx, tenant.SchemaName, tenant.ID, expense.ID)
	if err != nil {
		t.Fatalf("GetByID after update failed: %v", err)
	}
	if updated.SubmittedAt == nil || updated.SubmittedBy == nil {
		t.Fatalf("expected submitted metadata to be persisted, got %#v", updated)
	}
}

func findAccountID(t *testing.T, ctx context.Context, pool *pgxpool.Pool, schemaName, code string) string {
	t.Helper()
	var accountID string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.accounts WHERE code = $1`, schemaName), code).Scan(&accountID); err != nil {
		t.Fatalf("failed to find account %s: %v", code, err)
	}
	return accountID
}
