package expenses

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/HMB-research/open-accounting/internal/accounting"
	"github.com/HMB-research/open-accounting/internal/documents"
)

func TestExpensesWave7CreateExpenseRepositoryErrors(t *testing.T) {
	service := NewServiceWithRepository(&memoryRepository{generateErr: errors.New("number failed"), expenses: map[string]*Expense{}}, newFakeAccountingPoster(), nil)
	_, err := service.CreateExpense(context.Background(), "tenant_demo", "tenant-1", validCreateExpenseRequest())
	if err == nil || !strings.Contains(err.Error(), "number failed") {
		t.Fatalf("CreateExpense() generate error = %v", err)
	}

	service = NewServiceWithRepository(&memoryRepository{createErr: errors.New("create failed"), expenses: map[string]*Expense{}}, newFakeAccountingPoster(), nil)
	_, err = service.CreateExpense(context.Background(), "tenant_demo", "tenant-1", validCreateExpenseRequest())
	if err == nil || !strings.Contains(err.Error(), "create failed") {
		t.Fatalf("CreateExpense() create error = %v", err)
	}
}

func TestExpensesWave7LifecycleLateErrors(t *testing.T) {
	ctx := context.Background()

	t.Run("submit update error", func(t *testing.T) {
		repo := newMemoryRepository()
		service := NewServiceWithRepository(repo, newFakeAccountingPoster(), nil)
		expense := createTestExpense(t, service)
		repo.updateErr = errors.New("update failed")
		_, err := service.SubmitExpense(ctx, "tenant_acme", "tenant-1", expense.ID, &ExpenseActionRequest{UserID: "user-1"})
		if err == nil || !strings.Contains(err.Error(), "update failed") {
			t.Fatalf("SubmitExpense() error = %v", err)
		}
	})

	t.Run("reject validation and update errors", func(t *testing.T) {
		repo := newMemoryRepository()
		service := NewServiceWithRepository(repo, newFakeAccountingPoster(), nil)
		expense := createTestExpense(t, service)
		_, err := service.SubmitExpense(ctx, "tenant_acme", "tenant-1", expense.ID, &ExpenseActionRequest{UserID: "user-1"})
		if err != nil {
			t.Fatalf("SubmitExpense() error = %v", err)
		}
		_, err = service.RejectExpense(ctx, "tenant_acme", "tenant-1", expense.ID, nil)
		if err == nil || !strings.Contains(err.Error(), "reject request is required") {
			t.Fatalf("RejectExpense(nil) error = %v", err)
		}
		_, err = service.RejectExpense(ctx, "tenant_acme", "tenant-1", expense.ID, &RejectExpenseRequest{UserID: " ", Reason: "duplicate"})
		if err == nil || !strings.Contains(err.Error(), "user id is required") {
			t.Fatalf("RejectExpense(blank user) error = %v", err)
		}
		_, err = service.RejectExpense(ctx, "tenant_acme", "tenant-1", expense.ID, &RejectExpenseRequest{UserID: "user-1", Reason: " "})
		if err == nil || !strings.Contains(err.Error(), "reason is required") {
			t.Fatalf("RejectExpense(blank reason) error = %v", err)
		}
		repo.updateErr = errors.New("reject update failed")
		_, err = service.RejectExpense(ctx, "tenant_acme", "tenant-1", expense.ID, &RejectExpenseRequest{UserID: "user-1", Reason: "duplicate"})
		if err == nil || !strings.Contains(err.Error(), "reject update failed") {
			t.Fatalf("RejectExpense(update) error = %v", err)
		}
	})

	t.Run("approval evidence errors", func(t *testing.T) {
		repo := newMemoryRepository()
		evidence := &fakeEvidenceEvaluator{err: errors.New("evidence failed")}
		service := NewServiceWithRepository(repo, newFakeAccountingPoster(), evidence)
		expense := createTestExpense(t, service)
		_, err := service.SubmitExpense(ctx, "tenant_acme", "tenant-1", expense.ID, &ExpenseActionRequest{UserID: "user-1"})
		if err != nil {
			t.Fatalf("SubmitExpense() error = %v", err)
		}
		_, err = service.ApproveExpense(ctx, "tenant_acme", "tenant-1", expense.ID, &ExpenseActionRequest{UserID: "user-2"})
		if err == nil || !strings.Contains(err.Error(), "evaluate expense receipt evidence") {
			t.Fatalf("ApproveExpense() evidence error = %v", err)
		}
	})
}

func TestExpensesWave7PostingValidationAndUpdateErrors(t *testing.T) {
	ctx := context.Background()

	repo := newMemoryRepository()
	service := NewServiceWithRepository(repo, nil, nil)
	expense := createTestExpense(t, service)
	stored := repo.expenses[expense.ID]
	stored.Status = StatusApproved
	stored.RequiresReceipt = false
	_, err := service.PostExpense(ctx, "tenant_acme", "tenant-1", expense.ID, &ExpenseActionRequest{UserID: "user-3"})
	if err == nil || !errors.Is(err, ErrExpenseAccountingInvalid) {
		t.Fatalf("PostExpense() no accounting error = %v", err)
	}

	accountingSvc := newFakeAccountingPoster()
	accountingSvc.accounts["expense-account"] = &accounting.Account{ID: "expense-account", AccountType: accounting.AccountTypeAsset}
	service = NewServiceWithRepository(repo, accountingSvc, nil)
	_, err = service.PostExpense(ctx, "tenant_acme", "tenant-1", expense.ID, &ExpenseActionRequest{UserID: "user-3"})
	if err == nil || !strings.Contains(err.Error(), "expense account must be EXPENSE") {
		t.Fatalf("PostExpense() bad expense account error = %v", err)
	}

	accountingSvc = newFakeAccountingPoster()
	accountingSvc.accounts["cash-account"] = &accounting.Account{ID: "cash-account", AccountType: accounting.AccountTypeRevenue}
	service = NewServiceWithRepository(repo, accountingSvc, nil)
	_, err = service.PostExpense(ctx, "tenant_acme", "tenant-1", expense.ID, &ExpenseActionRequest{UserID: "user-3"})
	if err == nil || !strings.Contains(err.Error(), "payment account must be ASSET or LIABILITY") {
		t.Fatalf("PostExpense() bad payment account error = %v", err)
	}

	accountingSvc = newFakeAccountingPoster()
	accountingSvc.createErr = errors.New("journal create failed")
	service = NewServiceWithRepository(repo, accountingSvc, nil)
	_, err = service.PostExpense(ctx, "tenant_acme", "tenant-1", expense.ID, &ExpenseActionRequest{UserID: "user-3"})
	if err == nil || !strings.Contains(err.Error(), "journal create failed") {
		t.Fatalf("PostExpense() create journal error = %v", err)
	}

	accountingSvc = newFakeAccountingPoster()
	accountingSvc.postErr = errors.New("journal post failed")
	service = NewServiceWithRepository(repo, accountingSvc, nil)
	_, err = service.PostExpense(ctx, "tenant_acme", "tenant-1", expense.ID, &ExpenseActionRequest{UserID: "user-3"})
	if err == nil || !strings.Contains(err.Error(), "journal post failed") {
		t.Fatalf("PostExpense() post journal error = %v", err)
	}

	accountingSvc = newFakeAccountingPoster()
	repo.updateErr = errors.New("posting update failed")
	service = NewServiceWithRepository(repo, accountingSvc, nil)
	_, err = service.PostExpense(ctx, "tenant_acme", "tenant-1", expense.ID, &ExpenseActionRequest{UserID: "user-3"})
	if err == nil || !strings.Contains(err.Error(), "posting update failed") {
		t.Fatalf("PostExpense() update error = %v", err)
	}
}

func TestExpensesWave7SmallHelpers(t *testing.T) {
	hydrateExpenseDerivedFields(nil)
	hydrateExpenseSlice(nil)

	userID, err := actionUserID(&ExpenseActionRequest{UserID: " user-1 "})
	if err != nil || userID != "user-1" {
		t.Fatalf("actionUserID() = %q, %v", userID, err)
	}
	_, err = actionUserID(nil)
	if err == nil || !strings.Contains(err.Error(), "action request is required") {
		t.Fatalf("actionUserID(nil) error = %v", err)
	}
	_, err = actionUserID(&ExpenseActionRequest{UserID: " "})
	if err == nil || !strings.Contains(err.Error(), "user id is required") {
		t.Fatalf("actionUserID(blank) error = %v", err)
	}

	noDescription := expenseJournalDescription(&Expense{ExpenseNumber: "EXP-1", Merchant: "Shop"})
	withDescription := expenseJournalDescription(&Expense{ExpenseNumber: "EXP-1", Merchant: "Shop", Description: "Paper"})
	if noDescription != "Expense EXP-1 - Shop" || withDescription != "Expense EXP-1 - Shop: Paper" {
		t.Fatalf("expenseJournalDescription() = %q / %q", noDescription, withDescription)
	}

	if got := normalizeExpenseDate(time.Time{}, time.Date(2026, 6, 25, 15, 30, 0, 0, time.FixedZone("EET", 2*60*60))); got.Hour() != 0 || got.Location() != time.UTC {
		t.Fatalf("normalizeExpenseDate() = %v, want UTC date only", got)
	}
}

func TestExpensesWave7ReceiptlessApprovalSkipsEvidence(t *testing.T) {
	repo := newMemoryRepository()
	service := NewServiceWithRepository(repo, newFakeAccountingPoster(), &fakeEvidenceEvaluator{err: errors.New("should not be called")})
	requiresReceipt := false
	expense, err := service.CreateExpense(context.Background(), "tenant_acme", "tenant-1", validCreateExpenseRequest(func(req *CreateExpenseRequest) {
		req.RequiresReceipt = &requiresReceipt
	}))
	if err != nil {
		t.Fatalf("CreateExpense() error = %v", err)
	}
	_, err = service.SubmitExpense(context.Background(), "tenant_acme", "tenant-1", expense.ID, &ExpenseActionRequest{UserID: "user-1"})
	if err != nil {
		t.Fatalf("SubmitExpense() error = %v", err)
	}
	_, err = service.ApproveExpense(context.Background(), "tenant_acme", "tenant-1", expense.ID, &ExpenseActionRequest{UserID: "user-2"})
	if err != nil {
		t.Fatalf("ApproveExpense() receiptless error = %v", err)
	}

	results, err := (&fakeEvidenceEvaluator{compliant: true}).EvaluateEvidencePolicy(context.Background(), "tenant_acme", "tenant-1", &documents.EvidencePolicyRequest{
		EntityType: documents.EntityTypeExpense,
		EntityIDs:  []string{expense.ID},
	})
	if err != nil || len(results) != 1 || !results[0].Compliant {
		t.Fatalf("fake evidence sanity check = %#v, %v", results, err)
	}
}
