package invoicing

import (
	"context"
	"fmt"

	"github.com/shopspring/decimal"
)

// Repository defines the contract for invoice data access
type Repository interface {
	Create(ctx context.Context, schemaName string, invoice *Invoice) error
	GetByID(ctx context.Context, schemaName, tenantID, invoiceID string) (*Invoice, error)
	List(ctx context.Context, schemaName, tenantID string, filter *InvoiceFilter) ([]Invoice, error)
	UpdateStatus(ctx context.Context, schemaName, tenantID, invoiceID string, status InvoiceStatus) error
	UpdatePayment(ctx context.Context, schemaName, tenantID, invoiceID string, amountPaid decimal.Decimal, status InvoiceStatus) error
	GenerateNumber(ctx context.Context, schemaName, tenantID string, invoiceType InvoiceType) (string, error)
	UpdateOverdueStatus(ctx context.Context, schemaName, tenantID string) (int, error)
}

// ErrInvoiceNotFound is returned when an invoice is not found
var ErrInvoiceNotFound = fmt.Errorf("invoice not found")
