package payments

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/HMB-research/open-accounting/internal/database"
	"github.com/HMB-research/open-accounting/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// GORMRepository implements Repository using GORM
type GORMRepository struct {
	db *gorm.DB
}

var errRepositoryDatabaseNotConfigured = errors.New("payments repository database is not configured")

// NewGORMRepository creates a new GORM payments repository
func NewGORMRepository(db *gorm.DB) *GORMRepository {
	return &GORMRepository{db: db}
}

func (r *GORMRepository) gormDB(ctx context.Context) (*gorm.DB, error) {
	if r == nil || r.db == nil {
		return nil, errRepositoryDatabaseNotConfigured
	}
	return r.db.WithContext(ctx), nil
}

func (r *GORMRepository) tenantTable(ctx context.Context, schemaName, tableName string) (*gorm.DB, error) {
	db, err := r.gormDB(ctx)
	if err != nil {
		return nil, err
	}
	return database.TenantTable(db, schemaName, tableName)
}

func qualifiedTableAfterSchemaValidated(schemaName, tableName string) string {
	qualifiedTable, _ := database.QualifiedTable(schemaName, tableName)
	return qualifiedTable
}

// Create inserts a new payment
func (r *GORMRepository) Create(ctx context.Context, schemaName string, payment *Payment) error {
	db, err := r.tenantTable(ctx, schemaName, "payments")
	if err != nil {
		return err
	}

	paymentModel := paymentToModel(payment)
	if err := db.Create(paymentModel).Error; err != nil {
		return fmt.Errorf("create payment: %w", err)
	}
	return nil
}

// CreateReversal creates an offsetting payment and marks the original as reversed atomically.
func (r *GORMRepository) CreateReversal(ctx context.Context, schemaName string, originalPaymentID string, reversal *Payment, allocations []PaymentAllocation, reversedAt time.Time, reversedBy string, reason string) error {
	db, err := r.tenantTable(ctx, schemaName, "payments")
	if err != nil {
		return err
	}

	return db.Transaction(func(tx *gorm.DB) error {
		return NewGORMRepository(tx).createReversal(ctx, schemaName, originalPaymentID, reversal, allocations, reversedAt, reversedBy, reason)
	})
}

func (r *GORMRepository) createReversal(ctx context.Context, schemaName string, originalPaymentID string, reversal *Payment, allocations []PaymentAllocation, reversedAt time.Time, reversedBy string, reason string) error {
	paymentsDB, err := r.tenantTable(ctx, schemaName, "payments")
	if err != nil {
		return err
	}
	// The schema and table identifiers were validated by the payments lookup
	// above, so the remaining handles can be derived without repeating the
	// validation or introducing a second failure point in the same transaction.
	allocationsDB := paymentsDB.Session(&gorm.Session{NewDB: true}).Table(
		qualifiedTableAfterSchemaValidated(schemaName, "payment_allocations"),
	)

	if err := paymentsDB.Create(paymentToModel(reversal)).Error; err != nil {
		return fmt.Errorf("create reversal payment: %w", err)
	}

	for i := range allocations {
		if err := allocationsDB.Create(allocationToModel(&allocations[i])).Error; err != nil {
			return fmt.Errorf("create reversal allocation: %w", err)
		}
	}

	result := paymentsDB.Session(&gorm.Session{NewDB: true}).
		Table(qualifiedTableAfterSchemaValidated(schemaName, "payments")).
		Model(&models.Payment{}).
		Where("id = ? AND tenant_id = ? AND reversed_by_payment_id IS NULL", originalPaymentID, reversal.TenantID).
		Updates(map[string]interface{}{
			"reversed_by_payment_id": reversal.ID,
			"reversed_at":            reversedAt,
			"reversed_by":            reversedBy,
			"reversal_reason":        reason,
		})
	if result.Error != nil {
		return fmt.Errorf("mark original payment reversed: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrPaymentAlreadyReversed
	}

	return nil
}

// GetByID retrieves a payment by ID
func (r *GORMRepository) GetByID(ctx context.Context, schemaName, tenantID, paymentID string) (*Payment, error) {
	return r.getByID(ctx, schemaName, tenantID, paymentID, false)
}

func (r *GORMRepository) GetByIDForUpdate(ctx context.Context, schemaName, tenantID, paymentID string) (*Payment, error) {
	return r.getByID(ctx, schemaName, tenantID, paymentID, true)
}

func (r *GORMRepository) getByID(ctx context.Context, schemaName, tenantID, paymentID string, forUpdate bool) (*Payment, error) {
	db, err := r.tenantTable(ctx, schemaName, "payments")
	if err != nil {
		return nil, err
	}

	var paymentModel models.Payment
	query := db.Where("id = ? AND tenant_id = ?", paymentID, tenantID)
	if forUpdate {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	err = query.First(&paymentModel).Error
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
	db, err := r.tenantTable(ctx, schemaName, "payments")
	if err != nil {
		return nil, err
	}

	query := db.Where("tenant_id = ?", tenantID)

	if filter != nil {
		if filter.PaymentType != "" {
			query = query.Where("payment_type = ?", filter.PaymentType)
		}
		if filter.PaymentMethod != "" {
			query = query.Where("payment_method = ?", filter.PaymentMethod)
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
	db, err := r.tenantTable(ctx, schemaName, "payment_allocations")
	if err != nil {
		return err
	}

	allocationModel := allocationToModel(allocation)
	if err := db.Create(allocationModel).Error; err != nil {
		return fmt.Errorf("create allocation: %w", err)
	}
	return nil
}

// GetAllocations retrieves allocations for a payment
func (r *GORMRepository) GetAllocations(ctx context.Context, schemaName, tenantID, paymentID string) ([]PaymentAllocation, error) {
	db, err := r.tenantTable(ctx, schemaName, "payment_allocations")
	if err != nil {
		return nil, err
	}

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
	prefix := PaymentNumberPrefix(paymentType)

	db, err := r.tenantTable(ctx, schemaName, "payments")
	if err != nil {
		return 0, err
	}

	var paymentNumbers []string
	if err := db.
		Where("tenant_id = ? AND payment_type = ?", tenantID, paymentType).
		Where("payment_number LIKE ?", prefix+"-%").
		Pluck("payment_number", &paymentNumbers).Error; err != nil {
		return 0, fmt.Errorf("get next payment number: %w", err)
	}

	return NextPaymentNumberSequence(paymentNumbers, paymentType), nil
}

// GetUnallocatedPayments returns payments with unallocated amounts
func (r *GORMRepository) GetUnallocatedPayments(ctx context.Context, schemaName, tenantID string, paymentType PaymentType) ([]Payment, error) {
	db, err := r.gormDB(ctx)
	if err != nil {
		return nil, err
	}

	paymentsTable, err := database.QualifiedTable(schemaName, "payments")
	if err != nil {
		return nil, err
	}
	allocationsTable := qualifiedTableAfterSchemaValidated(schemaName, "payment_allocations")

	var paymentModels []models.Payment
	allocatedAmount := db.
		Table(allocationsTable + " AS pa").
		Select("SUM(pa.amount)").
		Where("pa.payment_id = p.id")

	err = db.
		Table(paymentsTable+" AS p").
		Select("p.*").
		Where("p.tenant_id = ? AND p.payment_type = ?", tenantID, paymentType).
		Where("p.amount > COALESCE((?), 0)", allocatedAmount).
		Order("p.payment_date").
		Scan(&paymentModels).Error
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
		ID:                  m.ID,
		TenantID:            m.TenantID,
		PaymentNumber:       m.PaymentNumber,
		PaymentType:         PaymentType(m.PaymentType),
		ContactID:           m.ContactID,
		PaymentDate:         m.PaymentDate,
		Amount:              m.Amount.Decimal,
		Currency:            m.Currency,
		ExchangeRate:        m.ExchangeRate.Decimal,
		BaseAmount:          m.BaseAmount.Decimal,
		PaymentMethod:       m.PaymentMethod,
		BankAccount:         m.BankAccount,
		Reference:           m.Reference,
		Notes:               m.Notes,
		JournalEntryID:      m.JournalEntryID,
		ReversalOfPaymentID: m.ReversalOfPaymentID,
		ReversedByPaymentID: m.ReversedByPaymentID,
		ReversedAt:          m.ReversedAt,
		ReversedBy:          m.ReversedBy,
		ReversalReason:      m.ReversalReason,
		CreatedAt:           m.CreatedAt,
		CreatedBy:           m.CreatedBy,
	}
}

func paymentToModel(p *Payment) *models.Payment {
	return &models.Payment{
		ID:                  p.ID,
		TenantID:            p.TenantID,
		PaymentNumber:       p.PaymentNumber,
		PaymentType:         models.PaymentType(p.PaymentType),
		ContactID:           p.ContactID,
		PaymentDate:         p.PaymentDate,
		Amount:              models.Decimal{Decimal: p.Amount},
		Currency:            p.Currency,
		ExchangeRate:        models.Decimal{Decimal: p.ExchangeRate},
		BaseAmount:          models.Decimal{Decimal: p.BaseAmount},
		PaymentMethod:       p.PaymentMethod,
		BankAccount:         p.BankAccount,
		Reference:           p.Reference,
		Notes:               p.Notes,
		JournalEntryID:      p.JournalEntryID,
		ReversalOfPaymentID: p.ReversalOfPaymentID,
		ReversedByPaymentID: p.ReversedByPaymentID,
		ReversedAt:          p.ReversedAt,
		ReversedBy:          p.ReversedBy,
		ReversalReason:      p.ReversalReason,
		CreatedAt:           p.CreatedAt,
		CreatedBy:           p.CreatedBy,
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
