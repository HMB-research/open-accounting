//go:build gorm

package payments

import (
	"context"
	"errors"
	"fmt"

	"github.com/HMB-research/open-accounting/internal/database"
	"github.com/HMB-research/open-accounting/internal/models"
	"gorm.io/gorm"
)

// GORMRepository implements Repository using GORM
type GORMRepository struct {
	db *gorm.DB
}

// NewGORMRepository creates a new GORM payments repository
func NewGORMRepository(db *gorm.DB) *GORMRepository {
	return &GORMRepository{db: db}
}

// Create inserts a new payment
func (r *GORMRepository) Create(ctx context.Context, schemaName string, payment *Payment) error {
	db := database.TenantDB(r.db, schemaName).WithContext(ctx)

	paymentModel := paymentToModel(payment)
	if err := db.Create(paymentModel).Error; err != nil {
		return fmt.Errorf("create payment: %w", err)
	}
	return nil
}

// GetByID retrieves a payment by ID
func (r *GORMRepository) GetByID(ctx context.Context, schemaName, tenantID, paymentID string) (*Payment, error) {
	db := database.TenantDB(r.db, schemaName).WithContext(ctx)

	var paymentModel models.Payment
	err := db.Where("id = ? AND tenant_id = ?", paymentID, tenantID).First(&paymentModel).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrPaymentNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get payment: %w", err)
	}

	return modelToPayment(&paymentModel), nil
}

// List retrieves payments with optional filtering
func (r *GORMRepository) List(ctx context.Context, schemaName, tenantID string, filter *PaymentFilter) ([]Payment, error) {
	db := database.TenantDB(r.db, schemaName).WithContext(ctx)

	query := db.Where("tenant_id = ?", tenantID)

	if filter != nil {
		if filter.PaymentType != "" {
			query = query.Where("payment_type = ?", filter.PaymentType)
		}
		if filter.ContactID != "" {
			query = query.Where("contact_id = ?", filter.ContactID)
		}
		if filter.FromDate != nil {
			query = query.Where("payment_date >= ?", filter.FromDate)
		}
		if filter.ToDate != nil {
			query = query.Where("payment_date <= ?", filter.ToDate)
		}
	}

	query = query.Order("payment_date DESC, payment_number DESC")

	var paymentModels []models.Payment
	if err := query.Find(&paymentModels).Error; err != nil {
		return nil, fmt.Errorf("list payments: %w", err)
	}

	payments := make([]Payment, len(paymentModels))
	for i, pm := range paymentModels {
		payments[i] = *modelToPayment(&pm)
	}

	return payments, nil
}

// CreateAllocation inserts a payment allocation
func (r *GORMRepository) CreateAllocation(ctx context.Context, schemaName string, allocation *PaymentAllocation) error {
	db := database.TenantDB(r.db, schemaName).WithContext(ctx)

	allocationModel := allocationToModel(allocation)
	if err := db.Create(allocationModel).Error; err != nil {
		return fmt.Errorf("create allocation: %w", err)
	}
	return nil
}

// GetAllocations retrieves allocations for a payment
func (r *GORMRepository) GetAllocations(ctx context.Context, schemaName, tenantID, paymentID string) ([]PaymentAllocation, error) {
	db := database.TenantDB(r.db, schemaName).WithContext(ctx)

	var allocationModels []models.PaymentAllocation
	if err := db.Where("payment_id = ? AND tenant_id = ?", paymentID, tenantID).
		Find(&allocationModels).Error; err != nil {
		return nil, fmt.Errorf("get allocations: %w", err)
	}

	allocations := make([]PaymentAllocation, len(allocationModels))
	for i, am := range allocationModels {
		allocations[i] = *modelToAllocation(&am)
	}

	return allocations, nil
}

// GetNextPaymentNumber returns the next payment number sequence
func (r *GORMRepository) GetNextPaymentNumber(ctx context.Context, schemaName, tenantID string, paymentType PaymentType) (int, error) {
	db := database.TenantDB(r.db, schemaName).WithContext(ctx)

	prefix := "PMT"
	if paymentType == PaymentTypeMade {
		prefix = "OUT"
	}

	var seq int
	err := db.Raw(fmt.Sprintf(`
		SELECT COALESCE(MAX(CAST(SUBSTRING(payment_number FROM '%s-([0-9]+)') AS INTEGER)), 0) + 1
		FROM payments WHERE tenant_id = ? AND payment_type = ?
	`, prefix), tenantID, paymentType).Scan(&seq).Error
	if err != nil {
		return 0, fmt.Errorf("get next payment number: %w", err)
	}

	return seq, nil
}

// GetUnallocatedPayments returns payments with unallocated amounts
func (r *GORMRepository) GetUnallocatedPayments(ctx context.Context, schemaName, tenantID string, paymentType PaymentType) ([]Payment, error) {
	db := database.TenantDB(r.db, schemaName).WithContext(ctx)

	var paymentModels []models.Payment
	err := db.Raw(`
		SELECT p.id, p.tenant_id, p.payment_number, p.payment_type, p.contact_id, p.payment_date,
		       p.amount, p.currency, p.exchange_rate, p.base_amount, p.payment_method, p.bank_account,
		       p.reference, p.notes, p.journal_entry_id, p.created_at, p.created_by
		FROM payments p
		WHERE p.tenant_id = ? AND p.payment_type = ?
		  AND p.amount > COALESCE((
		      SELECT SUM(pa.amount) FROM payment_allocations pa WHERE pa.payment_id = p.id
		  ), 0)
		ORDER BY p.payment_date
	`, tenantID, paymentType).Scan(&paymentModels).Error
	if err != nil {
		return nil, fmt.Errorf("get unallocated payments: %w", err)
	}

	payments := make([]Payment, len(paymentModels))
	for i, pm := range paymentModels {
		payments[i] = *modelToPayment(&pm)
	}

	return payments, nil
}

// Conversion helpers between domain types and GORM models

func modelToPayment(m *models.Payment) *Payment {
	return &Payment{
		ID:             m.ID,
		TenantID:       m.TenantID,
		PaymentNumber:  m.PaymentNumber,
		PaymentType:    PaymentType(m.PaymentType),
		ContactID:      m.ContactID,
		PaymentDate:    m.PaymentDate,
		Amount:         m.Amount.Decimal,
		Currency:       m.Currency,
		ExchangeRate:   m.ExchangeRate.Decimal,
		BaseAmount:     m.BaseAmount.Decimal,
		PaymentMethod:  m.PaymentMethod,
		BankAccount:    m.BankAccount,
		Reference:      m.Reference,
		Notes:          m.Notes,
		JournalEntryID: m.JournalEntryID,
		CreatedAt:      m.CreatedAt,
		CreatedBy:      m.CreatedBy,
	}
}

func paymentToModel(p *Payment) *models.Payment {
	return &models.Payment{
		ID:             p.ID,
		TenantID:       p.TenantID,
		PaymentNumber:  p.PaymentNumber,
		PaymentType:    models.PaymentType(p.PaymentType),
		ContactID:      p.ContactID,
		PaymentDate:    p.PaymentDate,
		Amount:         models.Decimal{Decimal: p.Amount},
		Currency:       p.Currency,
		ExchangeRate:   models.Decimal{Decimal: p.ExchangeRate},
		BaseAmount:     models.Decimal{Decimal: p.BaseAmount},
		PaymentMethod:  p.PaymentMethod,
		BankAccount:    p.BankAccount,
		Reference:      p.Reference,
		Notes:          p.Notes,
		JournalEntryID: p.JournalEntryID,
		CreatedAt:      p.CreatedAt,
		CreatedBy:      p.CreatedBy,
	}
}

func modelToAllocation(m *models.PaymentAllocation) *PaymentAllocation {
	return &PaymentAllocation{
		ID:        m.ID,
		TenantID:  m.TenantID,
		PaymentID: m.PaymentID,
		InvoiceID: m.InvoiceID,
		Amount:    m.Amount.Decimal,
		CreatedAt: m.CreatedAt,
	}
}

func allocationToModel(a *PaymentAllocation) *models.PaymentAllocation {
	return &models.PaymentAllocation{
		ID:        a.ID,
		TenantID:  a.TenantID,
		PaymentID: a.PaymentID,
		InvoiceID: a.InvoiceID,
		Amount:    models.Decimal{Decimal: a.Amount},
		CreatedAt: a.CreatedAt,
	}
}
