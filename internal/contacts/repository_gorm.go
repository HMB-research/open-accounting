//go:build gorm

package contacts

import (
	"context"
	"errors"
	"time"

	"github.com/HMB-research/open-accounting/internal/database"
	"gorm.io/gorm"
)

// GORMRepository implements Repository using GORM
type GORMRepository struct {
	db *gorm.DB
}

// NewGORMRepository creates a new GORM repository
func NewGORMRepository(db *gorm.DB) *GORMRepository {
	return &GORMRepository{db: db}
}

func (r *GORMRepository) tenantTable(ctx context.Context, schemaName, tableName string) (*gorm.DB, error) {
	db, err := database.TenantTable(r.db.WithContext(ctx), schemaName, tableName)
	if err != nil {
		return nil, err
	}
	return db, nil
}

// Create inserts a new contact
func (r *GORMRepository) Create(ctx context.Context, schemaName string, contact *Contact) error {
	db, err := r.tenantTable(ctx, schemaName, "contacts")
	if err != nil {
		return err
	}
	return db.Create(contact).Error
}

// GetByID retrieves a contact by ID
func (r *GORMRepository) GetByID(ctx context.Context, schemaName, tenantID, contactID string) (*Contact, error) {
	db, err := r.tenantTable(ctx, schemaName, "contacts")
	if err != nil {
		return nil, err
	}

	var contact Contact
	err = db.Where("id = ? AND tenant_id = ?", contactID, tenantID).First(&contact).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrContactNotFound
	}
	if err != nil {
		return nil, err
	}

	return &contact, nil
}

// List retrieves contacts with optional filtering
func (r *GORMRepository) List(ctx context.Context, schemaName, tenantID string, filter *ContactFilter) ([]Contact, error) {
	db, err := r.tenantTable(ctx, schemaName, "contacts")
	if err != nil {
		return nil, err
	}

	query := db.Where("tenant_id = ?", tenantID)

	if filter != nil {
		if filter.ContactType != "" {
			query = query.Where("contact_type = ?", filter.ContactType)
		}
		if filter.ActiveOnly {
			query = query.Where("is_active = ?", true)
		}
		if filter.Search != "" {
			searchPattern := "%" + filter.Search + "%"
			query = query.Where("name ILIKE ? OR code ILIKE ? OR email ILIKE ?",
				searchPattern, searchPattern, searchPattern)
		}
	}

	var contacts []Contact
	err = query.Order("name").Find(&contacts).Error
	if err != nil {
		return nil, err
	}

	return contacts, nil
}

// Update updates a contact
func (r *GORMRepository) Update(ctx context.Context, schemaName string, contact *Contact) error {
	db, err := r.tenantTable(ctx, schemaName, "contacts")
	if err != nil {
		return err
	}

	contact.UpdatedAt = time.Now()

	result := db.Where("id = ? AND tenant_id = ?", contact.ID, contact.TenantID).
		Updates(map[string]interface{}{
			"name":               contact.Name,
			"reg_code":           contact.RegCode,
			"vat_number":         contact.VATNumber,
			"email":              contact.Email,
			"phone":              contact.Phone,
			"address_line1":      contact.AddressLine1,
			"address_line2":      contact.AddressLine2,
			"city":               contact.City,
			"postal_code":        contact.PostalCode,
			"country_code":       contact.CountryCode,
			"payment_terms_days": contact.PaymentTermsDays,
			"credit_limit":       contact.CreditLimit,
			"default_account_id": contact.DefaultAccountID,
			"is_active":          contact.IsActive,
			"notes":              contact.Notes,
			"updated_at":         contact.UpdatedAt,
		})

	return result.Error
}

// Delete soft-deletes a contact
func (r *GORMRepository) Delete(ctx context.Context, schemaName, tenantID, contactID string) error {
	db, err := r.tenantTable(ctx, schemaName, "contacts")
	if err != nil {
		return err
	}

	result := db.Where("id = ? AND tenant_id = ?", contactID, tenantID).
		Updates(map[string]interface{}{
			"is_active":  false,
			"updated_at": time.Now(),
		})

	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrContactNotFound
	}

	return nil
}
