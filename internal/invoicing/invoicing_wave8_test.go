package invoicing

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

type countErrReminderRepository struct {
	*MockReminderRepository
	err error
}

func (r countErrReminderRepository) GetReminderCount(context.Context, string, string, string) (int, *time.Time, error) {
	return 0, nil, r.err
}

func TestInvoicingWave8CreateRejectsInvalidVATTreatment(t *testing.T) {
	service := NewServiceWithRepository(NewMockRepository(), nil)

	_, err := service.Create(context.Background(), "tenant-1", "tenant_demo", &CreateInvoiceRequest{
		InvoiceType: InvoiceTypeSales,
		ContactID:   "contact-1",
		UserID:      "user-1",
		Lines: []CreateInvoiceLineRequest{{
			Description:  "Consulting",
			Quantity:     decimal.NewFromInt(1),
			UnitPrice:    decimal.NewFromInt(100),
			VATRate:      decimal.NewFromInt(20),
			VATTreatment: VATTreatment("unsupported"),
		}},
	})
	if err == nil || !strings.Contains(err.Error(), "invalid VAT treatment") {
		t.Fatalf("Create() error = %v, want VAT treatment validation", err)
	}
}

func TestInvoicingWave8ResolveInvoiceIDByNumberBranches(t *testing.T) {
	ctx := context.Background()

	t.Run("list error", func(t *testing.T) {
		repo := NewMockRepository()
		repo.ListFn = func(context.Context, string, string, *InvoiceFilter) ([]Invoice, error) {
			return nil, errors.New("list failed")
		}
		_, err := NewServiceWithRepository(repo, nil).ResolveInvoiceIDByNumber(ctx, "tenant-1", "tenant_demo", "INV-1")
		if err == nil || !strings.Contains(err.Error(), "list invoices") {
			t.Fatalf("ResolveInvoiceIDByNumber() error = %v, want list failure", err)
		}
	})

	t.Run("not found", func(t *testing.T) {
		repo := NewMockRepository()
		repo.invoices["inv-1"] = &Invoice{ID: "inv-1", TenantID: "tenant-1", InvoiceNumber: "INV-OTHER"}
		_, err := NewServiceWithRepository(repo, nil).ResolveInvoiceIDByNumber(ctx, "tenant-1", "tenant_demo", "INV-1")
		if !errors.Is(err, ErrInvoiceNotFound) {
			t.Fatalf("ResolveInvoiceIDByNumber() error = %v, want ErrInvoiceNotFound", err)
		}
	})

	t.Run("multiple matches", func(t *testing.T) {
		repo := NewMockRepository()
		repo.ListFn = func(context.Context, string, string, *InvoiceFilter) ([]Invoice, error) {
			return []Invoice{
				{ID: "inv-1", TenantID: "tenant-1", InvoiceNumber: "INV-1"},
				{ID: "inv-2", TenantID: "tenant-1", InvoiceNumber: " inv-1 "},
			}, nil
		}
		_, err := NewServiceWithRepository(repo, nil).ResolveInvoiceIDByNumber(ctx, "tenant-1", "tenant_demo", "INV-1")
		if err == nil || !strings.Contains(err.Error(), "matched multiple invoices") {
			t.Fatalf("ResolveInvoiceIDByNumber() error = %v, want duplicate match", err)
		}
	})
}

func TestInvoicingWave8ReminderSummaryCountError(t *testing.T) {
	base := NewMockReminderRepository()
	base.AddMockOverdueInvoice("inv-1", "INV-1", "contact-1", "Acme", "billing@example.com", "EUR", decimal.NewFromInt(100), decimal.Zero, 10)
	service := NewReminderServiceWithRepository(countErrReminderRepository{
		MockReminderRepository: base,
		err:                    errors.New("count failed"),
	}, nil)

	_, err := service.GetOverdueInvoicesSummary(context.Background(), "tenant-1", "tenant_demo")
	if err == nil || !strings.Contains(err.Error(), "get reminder count for inv-1") {
		t.Fatalf("GetOverdueInvoicesSummary() error = %v, want count failure", err)
	}
}

func TestInvoicingWave8InterestRepositoryNilDatabaseErrors(t *testing.T) {
	repo := NewInterestRepository(nil)
	ctx := context.Background()

	if _, err := repo.GetInvoiceForInterest(ctx, "tenant_demo", "tenant-1", "inv-1"); err == nil || !strings.Contains(err.Error(), "database is not configured") {
		t.Fatalf("GetInvoiceForInterest() error = %v, want nil database error", err)
	}
	if err := repo.CreateInterest(ctx, "tenant_demo", &InvoiceInterest{InvoiceID: "inv-1"}); err == nil || !strings.Contains(err.Error(), "database is not configured") {
		t.Fatalf("CreateInterest() error = %v, want nil database error", err)
	}
	if _, err := repo.ListOverdueInvoices(ctx, "tenant_demo", "tenant-1", time.Now()); err == nil || !strings.Contains(err.Error(), "database is not configured") {
		t.Fatalf("ListOverdueInvoices() error = %v, want nil database error", err)
	}
}
