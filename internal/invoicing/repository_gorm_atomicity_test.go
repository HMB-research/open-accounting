package invoicing

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/HMB-research/open-accounting/internal/accounting"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestGORMRepositoryApplyPaymentLocksAndUpdatesInvoice(t *testing.T) {
	now := time.Date(2026, time.June, 25, 12, 0, 0, 0, time.UTC)
	invoice := invoicingDryRunInvoice("tenant-1", now)
	capture := &invoicingDryRunSQLCapture{}
	repo := NewGORMRepository(newInvoicingDryRunDB(t,
		withInvoicingDryRunFixtures(invoicingDryRunFixtures{invoice: invoiceToModel(invoice)}),
		withInvoicingDryRunUpdateRows(1),
		withInvoicingDryRunSQLCapture(capture),
	))

	require.NoError(t, repo.ApplyPayment(context.Background(), "tenant_invoicing", "tenant-1", invoice.ID, decimal.RequireFromString("40")))
	assert.Contains(t, strings.Join(capture.statements, "\n"), "FOR UPDATE")
}

func TestGORMRepositoryApplyPaymentReturnsUpdateError(t *testing.T) {
	expectedErr := errors.New("invoice update failed")
	invoice := invoicingDryRunInvoice("tenant-1", time.Date(2026, time.June, 25, 12, 0, 0, 0, time.UTC))
	repo := NewGORMRepository(newInvoicingDryRunDB(t,
		withInvoicingDryRunFixtures(invoicingDryRunFixtures{invoice: invoiceToModel(invoice)}),
		withInvoicingDryRunUpdateError(expectedErr),
	))

	err := repo.ApplyPayment(context.Background(), "tenant_invoicing", "tenant-1", invoice.ID, decimal.RequireFromString("40"))
	require.Error(t, err)
	assert.ErrorIs(t, err, expectedErr)
	assert.Contains(t, err.Error(), "update payment")
}

func TestService_RecordPaymentUsesAtomicRepositoryCapability(t *testing.T) {
	now := time.Date(2026, time.June, 25, 12, 0, 0, 0, time.UTC)
	invoice := invoicingDryRunInvoice("tenant-1", now)
	service := NewServiceWithRepository(NewGORMRepository(newInvoicingDryRunDB(t,
		withInvoicingDryRunFixtures(invoicingDryRunFixtures{invoice: invoiceToModel(invoice)}),
		withInvoicingDryRunUpdateRows(1),
	)), nil)

	require.NoError(t, service.RecordPayment(context.Background(), "tenant-1", "tenant_invoicing", invoice.ID, decimal.RequireFromString("40")))

	voided := invoice
	voided.Status = StatusVoided
	service = NewServiceWithRepository(NewGORMRepository(newInvoicingDryRunDB(t,
		withInvoicingDryRunFixtures(invoicingDryRunFixtures{invoice: invoiceToModel(voided)}),
	)), nil)
	err := service.RecordPayment(context.Background(), "tenant-1", "tenant_invoicing", invoice.ID, decimal.RequireFromString("40"))
	assert.ErrorContains(t, err, "record payment: cannot record payment on voided invoice")
}

type publicPaymentApplicatorRepository struct {
	*MockRepository
	err error
}

func (r *publicPaymentApplicatorRepository) ApplyPayment(context.Context, string, string, string, decimal.Decimal) error {
	return r.err
}

func TestService_RecordPaymentUsesPublicPaymentApplicator(t *testing.T) {
	service := NewServiceWithRepository(&publicPaymentApplicatorRepository{
		MockRepository: NewMockRepository(),
	}, nil)
	require.NoError(t, service.RecordPayment(context.Background(), "tenant-1", "tenant_invoicing", "invoice-1", decimal.NewFromInt(1)))

	expectedErr := errors.New("public applicator failed")
	service = NewServiceWithRepository(&publicPaymentApplicatorRepository{
		MockRepository: NewMockRepository(),
		err:            expectedErr,
	}, nil)

	err := service.RecordPayment(context.Background(), "tenant-1", "tenant_invoicing", "invoice-1", decimal.NewFromInt(1))
	assert.ErrorIs(t, err, expectedErr)
	assert.Contains(t, err.Error(), "record payment")
}

func TestServiceWithRepositoryPreservesAccountingDependency(t *testing.T) {
	accountingService := &accounting.Service{}
	base := NewServiceWithRepository(nil, accountingService)
	bound := base.WithRepository(NewMockRepository())

	assert.Same(t, accountingService, bound.accounting)
}

func TestGORMRepositoryVoidInvoiceEdges(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, time.June, 25, 12, 0, 0, 0, time.UTC)
	invoice := invoicingDryRunInvoice("tenant-1", now)

	t.Run("invalid schema", func(t *testing.T) {
		repo := NewGORMRepository(newInvoicingDryRunDB(t))
		err := repo.VoidInvoice(ctx, "tenant-invalid", "tenant-1", invoice.ID)
		assert.Error(t, err)
	})

	t.Run("update error", func(t *testing.T) {
		expectedErr := errors.New("void update failed")
		repo := NewGORMRepository(newInvoicingDryRunDB(t,
			withInvoicingDryRunUpdateError(expectedErr),
		))
		err := repo.VoidInvoice(ctx, "tenant_invoicing", "tenant-1", invoice.ID)
		assert.ErrorIs(t, err, expectedErr)
		assert.Contains(t, err.Error(), "void invoice")
	})

	t.Run("lookup error after conditional update", func(t *testing.T) {
		expectedErr := errors.New("void lookup failed")
		repo := NewGORMRepository(newInvoicingDryRunDB(t,
			withInvoicingDryRunUpdateRows(0),
			withInvoicingDryRunQueryError(expectedErr),
		))
		err := repo.VoidInvoice(ctx, "tenant_invoicing", "tenant-1", invoice.ID)
		assert.ErrorIs(t, err, expectedErr)
	})

	t.Run("not found after conditional update", func(t *testing.T) {
		repo := NewGORMRepository(newInvoicingDryRunDB(t,
			withInvoicingDryRunFixtures(invoicingDryRunFixtures{invoice: invoiceToModel(invoice)}),
			withInvoicingDryRunUpdateRows(0),
		))
		err := repo.VoidInvoice(ctx, "tenant_invoicing", "tenant-1", invoice.ID)
		assert.ErrorIs(t, err, ErrInvoiceNotFound)
	})
}

func TestGORMRepositoryApplyPaymentRejectsInvalidSchema(t *testing.T) {
	repo := NewGORMRepository(newInvoicingDryRunDB(t))
	err := repo.ApplyPayment(context.Background(), "tenant-invalid", "tenant-1", "invoice-1", decimal.NewFromInt(1))
	assert.Error(t, err)

	err = repo.applyPayment(context.Background(), "tenant-invalid", "tenant-1", "invoice-1", decimal.NewFromInt(1))
	assert.Error(t, err)
}

func TestGORMRepositoryVoidInvoiceRequiresConfiguredDatabase(t *testing.T) {
	err := (*GORMRepository)(nil).VoidInvoice(context.Background(), "tenant_invoicing", "tenant-1", "invoice-1")
	assert.Error(t, err)
}

func TestService_VoidUsesAtomicRepositoryCapability(t *testing.T) {
	now := time.Date(2026, time.June, 25, 12, 0, 0, 0, time.UTC)
	invoice := invoicingDryRunInvoice("tenant-1", now)
	repo := NewGORMRepository(newInvoicingDryRunDB(t, withInvoicingDryRunUpdateRows(1)))
	service := NewServiceWithRepository(repo, nil)

	require.NoError(t, service.Void(context.Background(), "tenant-1", "tenant_invoicing", invoice.ID))
}

func TestGORMRepositoryApplyPaymentRejectsInvalidInvoiceState(t *testing.T) {
	now := time.Date(2026, time.June, 25, 12, 0, 0, 0, time.UTC)
	ctx := context.Background()

	t.Run("not found", func(t *testing.T) {
		repo := NewGORMRepository(newInvoicingDryRunDB(t, withInvoicingDryRunQueryError(gorm.ErrRecordNotFound)))
		err := repo.ApplyPayment(ctx, "tenant_invoicing", "tenant-1", "missing", decimal.NewFromInt(1))
		assert.ErrorIs(t, err, ErrInvoiceNotFound)
	})

	t.Run("query error", func(t *testing.T) {
		expectedErr := errors.New("invoice query failed")
		repo := NewGORMRepository(newInvoicingDryRunDB(t, withInvoicingDryRunQueryError(expectedErr)))
		err := repo.ApplyPayment(ctx, "tenant_invoicing", "tenant-1", "invoice-1", decimal.NewFromInt(1))
		assert.ErrorIs(t, err, expectedErr)
		assert.Contains(t, err.Error(), "get invoice")
	})

	t.Run("voided", func(t *testing.T) {
		invoice := invoicingDryRunInvoice("tenant-1", now)
		invoice.Status = StatusVoided
		repo := NewGORMRepository(newInvoicingDryRunDB(t, withInvoicingDryRunFixtures(invoicingDryRunFixtures{invoice: invoiceToModel(invoice)})))
		err := repo.ApplyPayment(ctx, "tenant_invoicing", "tenant-1", invoice.ID, decimal.NewFromInt(1))
		assert.ErrorContains(t, err, "cannot record payment on voided invoice")
	})

	t.Run("update affects no rows", func(t *testing.T) {
		invoice := invoicingDryRunInvoice("tenant-1", now)
		repo := NewGORMRepository(newInvoicingDryRunDB(t,
			withInvoicingDryRunFixtures(invoicingDryRunFixtures{invoice: invoiceToModel(invoice)}),
			withInvoicingDryRunUpdateRows(0),
		))
		err := repo.ApplyPayment(ctx, "tenant_invoicing", "tenant-1", invoice.ID, decimal.NewFromInt(1))
		assert.ErrorIs(t, err, ErrInvoiceNotFound)
	})
}

func TestGORMRepositoryVoidInvoiceErrors(t *testing.T) {
	now := time.Date(2026, time.June, 25, 12, 0, 0, 0, time.UTC)
	ctx := context.Background()
	for name, tt := range map[string]struct {
		status InvoiceStatus
		paid   decimal.Decimal
		want   string
	}{
		"already voided": {status: StatusVoided, paid: decimal.Zero, want: "invoice already voided"},
		"has payment":    {status: StatusSent, paid: decimal.NewFromInt(1), want: "cannot void invoice with payments"},
	} {
		t.Run(name, func(t *testing.T) {
			invoice := invoicingDryRunInvoice("tenant-1", now)
			invoice.Status = tt.status
			invoice.AmountPaid = tt.paid
			repo := NewGORMRepository(newInvoicingDryRunDB(t,
				withInvoicingDryRunFixtures(invoicingDryRunFixtures{invoice: invoiceToModel(invoice)}),
				withInvoicingDryRunUpdateRows(0),
			))
			err := repo.VoidInvoice(ctx, "tenant_invoicing", "tenant-1", invoice.ID)
			assert.ErrorContains(t, err, tt.want)
		})
	}
}
