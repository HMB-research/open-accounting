package expenses

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExpensesWave9LifecycleStatusAndUpdateErrors(t *testing.T) {
	ctx := context.Background()

	t.Run("submit rejects non draft or rejected status", func(t *testing.T) {
		repo := newMemoryRepository()
		service := NewServiceWithRepository(repo, newFakeAccountingPoster(), nil)
		expense := createTestExpense(t, service)
		repo.expenses[expense.ID].Status = StatusSubmitted

		_, err := service.SubmitExpense(ctx, "tenant_acme", "tenant-1", expense.ID, &ExpenseActionRequest{UserID: "user-1"})

		require.ErrorIs(t, err, ErrInvalidStatusTransition)
		require.ErrorContains(t, err, "only draft or rejected expenses can be submitted")
	})

	t.Run("approve propagates get and update errors", func(t *testing.T) {
		service := NewServiceWithRepository(newMemoryRepository(), newFakeAccountingPoster(), nil)
		_, err := service.ApproveExpense(ctx, "tenant_acme", "tenant-1", "missing", &ExpenseActionRequest{UserID: "user-1"})
		require.ErrorIs(t, err, ErrExpenseNotFound)

		repo := newMemoryRepository()
		service = NewServiceWithRepository(repo, newFakeAccountingPoster(), &fakeEvidenceEvaluator{compliant: true})
		expense := createTestExpense(t, service)
		repo.expenses[expense.ID].Status = StatusSubmitted
		repo.updateErr = errors.New("update failed")

		_, err = service.ApproveExpense(ctx, "tenant_acme", "tenant-1", expense.ID, &ExpenseActionRequest{UserID: "user-1"})
		require.ErrorContains(t, err, "update failed")
	})

	t.Run("reject propagates get and update errors", func(t *testing.T) {
		service := NewServiceWithRepository(newMemoryRepository(), newFakeAccountingPoster(), nil)
		_, err := service.RejectExpense(ctx, "tenant_acme", "tenant-1", "missing", &RejectExpenseRequest{UserID: "user-1", Reason: "bad receipt"})
		require.ErrorIs(t, err, ErrExpenseNotFound)

		repo := newMemoryRepository()
		service = NewServiceWithRepository(repo, newFakeAccountingPoster(), nil)
		expense := createTestExpense(t, service)
		repo.expenses[expense.ID].Status = StatusSubmitted
		repo.updateErr = errors.New("reject update failed")

		_, err = service.RejectExpense(ctx, "tenant_acme", "tenant-1", expense.ID, &RejectExpenseRequest{UserID: "user-1", Reason: "bad receipt"})
		require.ErrorContains(t, err, "reject update failed")
	})

	t.Run("post propagates early errors", func(t *testing.T) {
		service := NewServiceWithRepository(newMemoryRepository(), newFakeAccountingPoster(), nil)
		_, err := service.PostExpense(ctx, "tenant_acme", "tenant-1", "missing", &ExpenseActionRequest{UserID: "user-1"})
		require.ErrorIs(t, err, ErrExpenseNotFound)

		repo := newMemoryRepository()
		service = NewServiceWithRepository(repo, newFakeAccountingPoster(), &fakeEvidenceEvaluator{compliant: true})
		expense := approvedTestExpense(t, service)

		_, err = service.PostExpense(ctx, "tenant_acme", "tenant-1", expense.ID, nil)
		require.ErrorContains(t, err, "action request is required")
	})

	t.Run("post requires approved receipt evidence", func(t *testing.T) {
		repo := newMemoryRepository()
		accountingSvc := newFakeAccountingPoster()
		evidence := &fakeEvidenceEvaluator{compliant: true}
		service := NewServiceWithRepository(repo, accountingSvc, evidence)
		expense := approvedTestExpense(t, service)
		evidence.compliant = false
		_, err := service.PostExpense(ctx, "tenant_acme", "tenant-1", expense.ID, &ExpenseActionRequest{UserID: "user-1"})
		require.ErrorIs(t, err, ErrApprovedReceiptRequired)
	})

	t.Run("post propagates update error", func(t *testing.T) {
		repo := newMemoryRepository()
		accountingSvc := newFakeAccountingPoster()
		service := NewServiceWithRepository(repo, accountingSvc, &fakeEvidenceEvaluator{compliant: true})
		expense := approvedTestExpense(t, service)
		repo.updateErr = errors.New("post update failed")

		_, err := service.PostExpense(ctx, "tenant_acme", "tenant-1", expense.ID, &ExpenseActionRequest{UserID: "user-1"})
		require.ErrorContains(t, err, "post update failed")
	})
}
