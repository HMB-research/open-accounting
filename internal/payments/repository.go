package payments

import (
	"context"
	"time"
)

// Repository defines the interface for payment data access
type Repository interface {
	Create(ctx context.Context, schemaName string, payment *Payment) error
	CreateReversal(ctx context.Context, schemaName string, originalPaymentID string, reversal *Payment, allocations []PaymentAllocation, reversedAt time.Time, reversedBy string, reason string) error
	GetByID(ctx context.Context, schemaName, tenantID, paymentID string) (*Payment, error)
	List(ctx context.Context, schemaName, tenantID string, filter *PaymentFilter) ([]Payment, error)
	CreateAllocation(ctx context.Context, schemaName string, allocation *PaymentAllocation) error
	GetAllocations(ctx context.Context, schemaName, tenantID, paymentID string) ([]PaymentAllocation, error)
	GetNextPaymentNumber(ctx context.Context, schemaName, tenantID string, paymentType PaymentType) (int, error)
	GetUnallocatedPayments(ctx context.Context, schemaName, tenantID string, paymentType PaymentType) ([]Payment, error)
}
