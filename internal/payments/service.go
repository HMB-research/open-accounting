package payments

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/HMB-research/open-accounting/internal/contacts"
	"github.com/HMB-research/open-accounting/internal/database"
	"github.com/HMB-research/open-accounting/internal/invoicing"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

// InvoiceService defines the interface for invoice operations needed by payments
type InvoiceService interface {
	RecordPayment(ctx context.Context, tenantID, schemaName, invoiceID string, amount decimal.Decimal) error
	ResolveInvoiceIDByNumber(ctx context.Context, tenantID, schemaName, invoiceNumber string) (string, error)
}

type contactLister interface {
	List(ctx context.Context, tenantID, schemaName string, filter *contacts.ContactFilter) ([]contacts.Contact, error)
}

// Service provides payment operations
type Service struct {
	repo      Repository
	invoicing InvoiceService
	contacts  contactLister
}

var (
	newGormDBFromPool         = database.NewGormDBFromPool
	newPaymentsContactService = func(db *pgxpool.Pool) contactLister {
		return contacts.NewService(db)
	}
)

// NewService creates a new payments service with an ORM-backed repository.
func NewService(db *pgxpool.Pool, invoicingService *invoicing.Service) *Service {
	if db == nil {
		return &Service{invoicing: invoicingService}
	}
	gormDB, err := newGormDBFromPool(context.Background(), db)
	if err != nil {
		panic(fmt.Errorf("create payments GORM repository: %w", err))
	}
	return &Service{
		repo:      NewGORMRepository(gormDB),
		invoicing: invoicingService,
		contacts:  newPaymentsContactService(db),
	}
}

// NewServiceWithRepository creates a new payments service with a custom repository
func NewServiceWithRepository(repo Repository, invoicingService InvoiceService) *Service {
	return &Service{
		repo:      repo,
		invoicing: invoicingService,
	}
}

// Create creates a new payment
func (s *Service) Create(ctx context.Context, tenantID, schemaName string, req *CreatePaymentRequest) (*Payment, error) {
	if req.Amount.LessThanOrEqual(decimal.Zero) {
		return nil, fmt.Errorf("payment amount must be positive")
	}

	payment := &Payment{
		ID:            uuid.New().String(),
		TenantID:      tenantID,
		PaymentType:   req.PaymentType,
		ContactID:     req.ContactID,
		PaymentDate:   req.PaymentDate,
		Amount:        req.Amount,
		Currency:      req.Currency,
		ExchangeRate:  req.ExchangeRate,
		PaymentMethod: req.PaymentMethod,
		BankAccount:   req.BankAccount,
		Reference:     req.Reference,
		Notes:         req.Notes,
		CreatedAt:     time.Now(),
		CreatedBy:     req.UserID,
	}

	if payment.Currency == "" {
		payment.Currency = "EUR"
	}
	if payment.ExchangeRate.IsZero() {
		payment.ExchangeRate = decimal.NewFromInt(1)
	}
	if payment.PaymentDate.IsZero() {
		payment.PaymentDate = time.Now()
	}

	payment.BaseAmount = payment.Amount.Mul(payment.ExchangeRate).Round(2)

	// Validate allocations don't exceed payment amount
	totalAllocated := decimal.Zero
	for _, alloc := range req.Allocations {
		if alloc.Amount.LessThanOrEqual(decimal.Zero) {
			return nil, fmt.Errorf("allocation amount must be positive")
		}
		totalAllocated = totalAllocated.Add(alloc.Amount)
	}
	if totalAllocated.GreaterThan(payment.Amount) {
		return nil, fmt.Errorf("total allocations exceed payment amount")
	}

	// Generate payment number
	seq, err := s.repo.GetNextPaymentNumber(ctx, schemaName, tenantID, payment.PaymentType)
	if err != nil {
		return nil, fmt.Errorf("generate payment number: %w", err)
	}

	payment.PaymentNumber = FormatPaymentNumber(payment.PaymentType, seq)

	// Insert payment
	if err := s.repo.Create(ctx, schemaName, payment); err != nil {
		return nil, fmt.Errorf("insert payment: %w", err)
	}

	// Create allocations
	for _, allocReq := range req.Allocations {
		allocation := PaymentAllocation{
			ID:        uuid.New().String(),
			TenantID:  tenantID,
			PaymentID: payment.ID,
			InvoiceID: allocReq.InvoiceID,
			Amount:    allocReq.Amount,
			CreatedAt: time.Now(),
		}

		if err := s.repo.CreateAllocation(ctx, schemaName, &allocation); err != nil {
			return nil, fmt.Errorf("insert allocation: %w", err)
		}

		payment.Allocations = append(payment.Allocations, allocation)
	}

	// Update invoice payment amounts
	for _, alloc := range payment.Allocations {
		if s.invoicing != nil {
			if err := s.invoicing.RecordPayment(ctx, tenantID, schemaName, alloc.InvoiceID, alloc.Amount); err != nil {
				// Log error but don't fail - payment is recorded
				fmt.Printf("warning: failed to update invoice %s payment amount: %v\n", alloc.InvoiceID, err)
			}
		}
	}

	return payment, nil
}

// Reverse creates an auditable offsetting payment for an existing payment.
func (s *Service) Reverse(ctx context.Context, tenantID, schemaName, paymentID string, req *ReversePaymentRequest) (*PaymentReversalResult, error) {
	if req == nil {
		req = &ReversePaymentRequest{}
	}
	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		return nil, fmt.Errorf("reversal reason is required")
	}

	original, err := s.GetByID(ctx, tenantID, schemaName, paymentID)
	if err != nil {
		return nil, err
	}
	if original.ReversalOfPaymentID != nil {
		return nil, fmt.Errorf("%w: reversal payments cannot be reversed", ErrPaymentReversalNotAllowed)
	}
	if original.ReversedByPaymentID != nil {
		return nil, ErrPaymentAlreadyReversed
	}

	reversalDate := req.PaymentDate
	if reversalDate.IsZero() {
		reversalDate = time.Now()
	}

	reversalPaymentID := uuid.New().String()
	reversalType := reversePaymentType(original.PaymentType)
	seq, err := s.repo.GetNextPaymentNumber(ctx, schemaName, tenantID, reversalType)
	if err != nil {
		return nil, fmt.Errorf("generate reversal payment number: %w", err)
	}

	reference := strings.TrimSpace(req.Reference)
	if reference == "" {
		reference = fmt.Sprintf("REVERSAL-%s", original.PaymentNumber)
	}
	notes := strings.TrimSpace(req.Notes)
	if notes == "" {
		notes = fmt.Sprintf("Reversal of %s: %s", original.PaymentNumber, reason)
	}
	reversalOfPaymentID := original.ID
	now := time.Now()
	reversal := &Payment{
		ID:                  reversalPaymentID,
		TenantID:            tenantID,
		PaymentNumber:       FormatPaymentNumber(reversalType, seq),
		PaymentType:         reversalType,
		ContactID:           original.ContactID,
		PaymentDate:         reversalDate,
		Amount:              original.Amount,
		Currency:            original.Currency,
		ExchangeRate:        original.ExchangeRate,
		BaseAmount:          original.BaseAmount,
		PaymentMethod:       original.PaymentMethod,
		BankAccount:         original.BankAccount,
		Reference:           reference,
		Notes:               notes,
		ReversalOfPaymentID: &reversalOfPaymentID,
		ReversalReason:      reason,
		CreatedAt:           now,
		CreatedBy:           req.UserID,
	}

	reversalAllocations := make([]PaymentAllocation, 0, len(original.Allocations))
	for _, originalAllocation := range original.Allocations {
		reversalAllocations = append(reversalAllocations, PaymentAllocation{
			ID:        uuid.New().String(),
			TenantID:  tenantID,
			PaymentID: reversal.ID,
			InvoiceID: originalAllocation.InvoiceID,
			Amount:    originalAllocation.Amount,
			CreatedAt: now,
		})
	}

	if err := s.repo.CreateReversal(ctx, schemaName, original.ID, reversal, reversalAllocations, now, req.UserID, reason); err != nil {
		return nil, fmt.Errorf("create payment reversal: %w", err)
	}

	for _, allocation := range original.Allocations {
		if s.invoicing != nil {
			if err := s.invoicing.RecordPayment(ctx, tenantID, schemaName, allocation.InvoiceID, allocation.Amount.Neg()); err != nil {
				return nil, fmt.Errorf("reverse invoice allocation %s: %w", allocation.InvoiceID, err)
			}
		}
	}

	original.ReversedByPaymentID = &reversal.ID
	original.ReversedAt = &now
	original.ReversedBy = &req.UserID
	original.ReversalReason = reason
	reversal.Allocations = reversalAllocations

	return &PaymentReversalResult{
		OriginalPayment: original,
		ReversalPayment: reversal,
	}, nil
}

// GetByID retrieves a payment by ID
func (s *Service) GetByID(ctx context.Context, tenantID, schemaName, paymentID string) (*Payment, error) {
	payment, err := s.repo.GetByID(ctx, schemaName, tenantID, paymentID)
	if err != nil {
		return nil, fmt.Errorf("get payment: %w", err)
	}

	// Load allocations
	allocations, err := s.repo.GetAllocations(ctx, schemaName, tenantID, paymentID)
	if err != nil {
		return nil, fmt.Errorf("get allocations: %w", err)
	}
	payment.Allocations = allocations

	return payment, nil
}

// List retrieves payments with optional filtering
func (s *Service) List(ctx context.Context, tenantID, schemaName string, filter *PaymentFilter) ([]Payment, error) {
	payments, err := s.repo.List(ctx, schemaName, tenantID, filter)
	if err != nil {
		return nil, fmt.Errorf("list payments: %w", err)
	}
	return payments, nil
}

// AllocateToInvoice allocates part of an existing payment to an invoice
func (s *Service) AllocateToInvoice(ctx context.Context, tenantID, schemaName, paymentID, invoiceID string, amount decimal.Decimal) error {
	payment, err := s.GetByID(ctx, tenantID, schemaName, paymentID)
	if err != nil {
		return err
	}

	unallocated := payment.UnallocatedAmount()
	if amount.GreaterThan(unallocated) {
		return fmt.Errorf("amount exceeds unallocated balance of %s", unallocated.String())
	}

	allocation := &PaymentAllocation{
		ID:        uuid.New().String(),
		TenantID:  tenantID,
		PaymentID: paymentID,
		InvoiceID: invoiceID,
		Amount:    amount,
		CreatedAt: time.Now(),
	}

	if err := s.repo.CreateAllocation(ctx, schemaName, allocation); err != nil {
		return fmt.Errorf("insert allocation: %w", err)
	}

	// Update invoice
	if s.invoicing != nil {
		if err := s.invoicing.RecordPayment(ctx, tenantID, schemaName, invoiceID, amount); err != nil {
			return fmt.Errorf("update invoice payment: %w", err)
		}
	}

	return nil
}

// GetUnallocatedPayments returns payments with unallocated amounts
func (s *Service) GetUnallocatedPayments(ctx context.Context, tenantID, schemaName string, paymentType PaymentType) ([]Payment, error) {
	return s.repo.GetUnallocatedPayments(ctx, schemaName, tenantID, paymentType)
}

func reversePaymentType(paymentType PaymentType) PaymentType {
	if paymentType == PaymentTypeMade {
		return PaymentTypeReceived
	}
	return PaymentTypeMade
}
