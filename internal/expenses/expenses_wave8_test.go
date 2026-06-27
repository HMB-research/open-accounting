package expenses

import (
	"context"
	"strings"
	"testing"
)

func TestExpensesWave8LifecycleEarlyErrors(t *testing.T) {
	ctx := context.Background()

	t.Run("get missing expense", func(t *testing.T) {
		service := NewServiceWithRepository(newMemoryRepository(), newFakeAccountingPoster(), nil)
		_, err := service.GetExpense(ctx, "tenant_acme", "tenant-1", "missing")
		if err == nil || !strings.Contains(err.Error(), "expense not found") {
			t.Fatalf("GetExpense() error = %v, want missing expense", err)
		}
	})

	t.Run("submit missing expense", func(t *testing.T) {
		service := NewServiceWithRepository(newMemoryRepository(), newFakeAccountingPoster(), nil)
		_, err := service.SubmitExpense(ctx, "tenant_acme", "tenant-1", "missing", &ExpenseActionRequest{UserID: "user-1"})
		if err == nil || !strings.Contains(err.Error(), "expense not found") {
			t.Fatalf("SubmitExpense() error = %v, want missing expense", err)
		}
	})

	t.Run("approve nil action request", func(t *testing.T) {
		repo := newMemoryRepository()
		service := NewServiceWithRepository(repo, newFakeAccountingPoster(), nil)
		expense := createTestExpense(t, service)
		repo.expenses[expense.ID].Status = StatusSubmitted

		_, err := service.ApproveExpense(ctx, "tenant_acme", "tenant-1", expense.ID, nil)
		if err == nil || !strings.Contains(err.Error(), "action request is required") {
			t.Fatalf("ApproveExpense(nil) error = %v, want action validation", err)
		}
	})

	t.Run("approve invalid status", func(t *testing.T) {
		repo := newMemoryRepository()
		service := NewServiceWithRepository(repo, newFakeAccountingPoster(), nil)
		expense := createTestExpense(t, service)

		_, err := service.ApproveExpense(ctx, "tenant_acme", "tenant-1", expense.ID, &ExpenseActionRequest{UserID: "user-2"})
		if err == nil || !strings.Contains(err.Error(), "only submitted expenses can be approved") {
			t.Fatalf("ApproveExpense(draft) error = %v, want invalid status", err)
		}
	})

	t.Run("post already posted", func(t *testing.T) {
		repo := newMemoryRepository()
		service := NewServiceWithRepository(repo, newFakeAccountingPoster(), nil)
		expense := createTestExpense(t, service)
		repo.expenses[expense.ID].Status = StatusPosted

		_, err := service.PostExpense(ctx, "tenant_acme", "tenant-1", expense.ID, &ExpenseActionRequest{UserID: "user-3"})
		if err == nil || !strings.Contains(err.Error(), "expense is already posted") {
			t.Fatalf("PostExpense(posted) error = %v, want already posted", err)
		}
	})
}
