package recurring

import (
	"context"
	"fmt"
	"time"
)

// Repository defines the contract for recurring invoice data access
type Repository interface {
	// Recurring Invoice CRUD
	Create(ctx context.Context, schemaName string, ri *RecurringInvoice) error
	CreateLine(ctx context.Context, schemaName string, line *RecurringInvoiceLine) error
	GetByID(ctx context.Context, schemaName, tenantID, id string) (*RecurringInvoice, error)
	GetLines(ctx context.Context, schemaName, recurringInvoiceID string) ([]RecurringInvoiceLine, error)
	List(ctx context.Context, schemaName, tenantID string, activeOnly bool) ([]RecurringInvoice, error)
	Update(ctx context.Context, schemaName string, ri *RecurringInvoice) error
	DeleteLines(ctx context.Context, schemaName, recurringInvoiceID string) error
	Delete(ctx context.Context, schemaName, tenantID, id string) error

	// Status operations
	SetActive(ctx context.Context, schemaName, tenantID, id string, active bool) error

	// Generation tracking
	GetDueRecurringInvoiceIDs(ctx context.Context, schemaName, tenantID string, asOfDate time.Time) ([]string, error)
	UpdateAfterGeneration(ctx context.Context, schemaName, tenantID, id string, nextDate time.Time, generatedAt time.Time) error
	UpdateInvoiceEmailStatus(ctx context.Context, schemaName, invoiceID string, sentAt *time.Time, status, logID string) error
}

// Common errors
var (
	ErrRecurringInvoiceNotFound = fmt.Errorf("recurring invoice not found")
)
