package payments

import (
	"context"
	"testing"
	"time"

	"github.com/HMB-research/open-accounting/internal/invoicing"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type atomicityTestRepository struct {
	*MockRepository
	lockedPayment bool
	inTxReversal  bool
}

func (r *atomicityTestRepository) GetByIDForUpdate(ctx context.Context, schemaName, tenantID, paymentID string) (*Payment, error) {
	r.lockedPayment = true
	return r.MockRepository.GetByID(ctx, schemaName, tenantID, paymentID)
}

func (r *atomicityTestRepository) createReversal(ctx context.Context, schemaName, originalPaymentID string, reversal *Payment, allocations []PaymentAllocation, reversedAt time.Time, reversedBy, reason string) error {
	r.inTxReversal = true
	return r.MockRepository.CreateReversal(ctx, schemaName, originalPaymentID, reversal, allocations, reversedAt, reversedBy, reason)
}

type atomicityTestTransactionRunner struct {
	repo      Repository
	invoicing InvoiceService
}

func (r atomicityTestTransactionRunner) WithTransaction(_ context.Context, fn func(Repository, InvoiceService) error) error {
	return fn(r.repo, r.invoicing)
}

func TestService_TransactionRunnerUsesLockedPaymentAndInTxReversal(t *testing.T) {
	repo := &atomicityTestRepository{MockRepository: NewMockRepository()}
	invoiceService := &MockInvoiceService{}
	original := &Payment{
		ID:            "pay-atomic",
		TenantID:      "tenant-1",
		PaymentNumber: "PMT-ATOMIC",
		PaymentType:   PaymentTypeReceived,
		Amount:        decimal.NewFromInt(100),
		Currency:      "EUR",
		ExchangeRate:  decimal.NewFromInt(1),
		BaseAmount:    decimal.NewFromInt(100),
	}
	repo.payments[original.ID] = original
	repo.allocations[original.ID] = []PaymentAllocation{{
		ID:        "alloc-atomic",
		TenantID:  "tenant-1",
		PaymentID: original.ID,
		InvoiceID: "invoice-atomic",
		Amount:    decimal.NewFromInt(40),
	}}
	service := &Service{
		repo:      repo,
		invoicing: invoiceService,
		transactionRunner: atomicityTestTransactionRunner{
			repo:      repo,
			invoicing: invoiceService,
		},
	}

	result, err := service.Reverse(context.Background(), "tenant-1", "tenant_demo", original.ID, &ReversePaymentRequest{Reason: "test", UserID: "user-1"})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, repo.lockedPayment)
	assert.True(t, repo.inTxReversal)
}

func TestService_CreateSkipsInvoiceCallbackOnlyWhenTransactionProvidesNone(t *testing.T) {
	repo := NewMockRepository()
	service := &Service{
		repo:      repo,
		invoicing: &MockInvoiceService{},
		transactionRunner: atomicityTestTransactionRunner{
			repo:      repo,
			invoicing: nil,
		},
	}

	_, err := service.Create(context.Background(), "tenant-1", "tenant_demo", &CreatePaymentRequest{
		PaymentType: PaymentTypeReceived,
		Amount:      decimal.NewFromInt(10),
		Allocations: []AllocationRequest{{InvoiceID: "invoice-1", Amount: decimal.NewFromInt(4)}},
	})
	require.NoError(t, err)
}

func TestGormPaymentTransactionRunnerBindsBothRepositories(t *testing.T) {
	ctx := context.Background()
	t.Run("without invoice service", func(t *testing.T) {
		runner := &gormPaymentTransactionRunner{db: newPaymentsDryRunDB(t)}
		called := false
		err := runner.WithTransaction(ctx, func(repo Repository, invoiceService InvoiceService) error {
			called = true
			assert.IsType(t, &GORMRepository{}, repo)
			assert.Nil(t, invoiceService)
			return nil
		})
		require.NoError(t, err)
		assert.True(t, called)
	})

	t.Run("with invoice service", func(t *testing.T) {
		base := invoicing.NewServiceWithRepository(nil, nil)
		runner := &gormPaymentTransactionRunner{db: newPaymentsDryRunDB(t), invoicing: base}
		err := runner.WithTransaction(ctx, func(repo Repository, invoiceService InvoiceService) error {
			assert.IsType(t, &GORMRepository{}, repo)
			assert.NotNil(t, invoiceService)
			return nil
		})
		require.NoError(t, err)
	})
}

func TestGORMRepositoryGetByIDForUpdate(t *testing.T) {
	now := time.Date(2026, time.June, 25, 12, 0, 0, 0, time.UTC)
	payment := paymentsDryRunPayment("payment-lock", "tenant-1", now)
	repo := NewGORMRepository(newPaymentsDryRunDB(t, withPaymentsDryRunFixtures(paymentsDryRunFixtures{payment: paymentToModel(payment)}, nil)))

	got, err := repo.GetByIDForUpdate(context.Background(), "tenant_payments", "tenant-1", payment.ID)
	require.NoError(t, err)
	assert.Equal(t, payment.ID, got.ID)
}

func TestService_AllocationRequiresInvoiceService(t *testing.T) {
	service := NewServiceWithRepository(NewMockRepository(), nil)
	err := service.AllocateToInvoice(context.Background(), "tenant-1", "tenant_demo", "payment-1", "invoice-1", decimal.NewFromInt(1))
	assert.ErrorContains(t, err, "invoicing service is required")
}

func TestService_AllocationRejectsInvalidPaymentStates(t *testing.T) {
	ctx := context.Background()
	for name, payment := range map[string]*Payment{
		"reversed original": {
			ID:                  "pay-reversed-original",
			TenantID:            "tenant-1",
			Amount:              decimal.NewFromInt(10),
			ReversedByPaymentID: stringPointer("reversal-1"),
		},
		"reversal payment": {
			ID:                  "pay-reversal",
			TenantID:            "tenant-1",
			Amount:              decimal.NewFromInt(10),
			ReversalOfPaymentID: stringPointer("original-1"),
		},
	} {
		t.Run(name, func(t *testing.T) {
			repo := NewMockRepository()
			repo.payments[payment.ID] = payment
			service := NewServiceWithRepository(repo, &MockInvoiceService{})
			err := service.AllocateToInvoice(ctx, "tenant-1", "tenant_demo", payment.ID, "invoice-1", decimal.NewFromInt(1))
			assert.ErrorContains(t, err, "reversed payments cannot be allocated")
		})
	}

	repo := NewMockRepository()
	payment := &Payment{ID: "pay-negative", TenantID: "tenant-1", Amount: decimal.NewFromInt(10)}
	repo.payments[payment.ID] = payment
	service := NewServiceWithRepository(repo, &MockInvoiceService{})
	err := service.AllocateToInvoice(ctx, "tenant-1", "tenant_demo", payment.ID, "invoice-1", decimal.Zero)
	assert.ErrorContains(t, err, "allocation amount must be positive")
}

func stringPointer(value string) *string {
	return &value
}

var _ Repository = (*atomicityTestRepository)(nil)
var _ paymentLocker = (*atomicityTestRepository)(nil)
var _ transactionReversalCreator = (*atomicityTestRepository)(nil)
