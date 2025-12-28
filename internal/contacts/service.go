package contacts

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Service provides contact management operations
type Service struct {
	db *pgxpool.Pool
}

// NewService creates a new contacts service
func NewService(db *pgxpool.Pool) *Service {
	return &Service{db: db}
}

// Create creates a new contact
func (s *Service) Create(ctx context.Context, tenantID string, schemaName string, req *CreateContactRequest) (*Contact, error) {
	contact := &Contact{
		ID:               uuid.New().String(),
		TenantID:         tenantID,
		Code:             req.Code,
		Name:             req.Name,
		ContactType:      req.ContactType,
		RegCode:          req.RegCode,
		VATNumber:        req.VATNumber,
		Email:            req.Email,
		Phone:            req.Phone,
		AddressLine1:     req.AddressLine1,
		AddressLine2:     req.AddressLine2,
		City:             req.City,
		PostalCode:       req.PostalCode,
		CountryCode:      req.CountryCode,
		PaymentTermsDays: req.PaymentTermsDays,
		CreditLimit:      req.CreditLimit,
		DefaultAccountID: req.DefaultAccountID,
		IsActive:         true,
		Notes:            req.Notes,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	if contact.CountryCode == "" {
		contact.CountryCode = "EE"
	}
	if contact.PaymentTermsDays == 0 {
		contact.PaymentTermsDays = 14
	}

	query := fmt.Sprintf(`
		INSERT INTO %s.contacts (
			id, tenant_id, code, name, contact_type, reg_code, vat_number,
			email, phone, address_line1, address_line2, city, postal_code,
			country_code, payment_terms_days, credit_limit, default_account_id,
			is_active, notes, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21)
	`, schemaName)

	_, err := s.db.Exec(ctx, query,
		contact.ID, contact.TenantID, contact.Code, contact.Name, contact.ContactType,
		contact.RegCode, contact.VATNumber, contact.Email, contact.Phone,
		contact.AddressLine1, contact.AddressLine2, contact.City, contact.PostalCode,
		contact.CountryCode, contact.PaymentTermsDays, contact.CreditLimit,
		contact.DefaultAccountID, contact.IsActive, contact.Notes,
		contact.CreatedAt, contact.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create contact: %w", err)
	}

	return contact, nil
}

// GetByID retrieves a contact by ID
func (s *Service) GetByID(ctx context.Context, tenantID, schemaName, contactID string) (*Contact, error) {
	query := fmt.Sprintf(`
		SELECT id, tenant_id, code, name, contact_type, reg_code, vat_number,
		       email, phone, address_line1, address_line2, city, postal_code,
		       country_code, payment_terms_days, credit_limit, default_account_id,
		       is_active, notes, created_at, updated_at
		FROM %s.contacts
		WHERE id = $1 AND tenant_id = $2
	`, schemaName)

	var c Contact
	err := s.db.QueryRow(ctx, query, contactID, tenantID).Scan(
		&c.ID, &c.TenantID, &c.Code, &c.Name, &c.ContactType,
		&c.RegCode, &c.VATNumber, &c.Email, &c.Phone,
		&c.AddressLine1, &c.AddressLine2, &c.City, &c.PostalCode,
		&c.CountryCode, &c.PaymentTermsDays, &c.CreditLimit,
		&c.DefaultAccountID, &c.IsActive, &c.Notes,
		&c.CreatedAt, &c.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("contact not found: %s", contactID)
	}
	if err != nil {
		return nil, fmt.Errorf("get contact: %w", err)
	}

	return &c, nil
}

// List retrieves contacts with optional filtering
func (s *Service) List(ctx context.Context, tenantID, schemaName string, filter *ContactFilter) ([]Contact, error) {
	query := fmt.Sprintf(`
		SELECT id, tenant_id, code, name, contact_type, reg_code, vat_number,
		       email, phone, address_line1, address_line2, city, postal_code,
		       country_code, payment_terms_days, credit_limit, default_account_id,
		       is_active, notes, created_at, updated_at
		FROM %s.contacts
		WHERE tenant_id = $1
	`, schemaName)

	args := []interface{}{tenantID}
	argNum := 2

	if filter != nil {
		if filter.ContactType != "" {
			query += fmt.Sprintf(" AND contact_type = $%d", argNum)
			args = append(args, filter.ContactType)
			argNum++
		}
		if filter.ActiveOnly {
			query += " AND is_active = true"
		}
		if filter.Search != "" {
			query += fmt.Sprintf(" AND (name ILIKE $%d OR code ILIKE $%d OR email ILIKE $%d)", argNum, argNum, argNum)
			args = append(args, "%"+filter.Search+"%")
		}
	}

	query += " ORDER BY name"

	rows, err := s.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list contacts: %w", err)
	}
	defer rows.Close()

	var contacts []Contact
	for rows.Next() {
		var c Contact
		if err := rows.Scan(
			&c.ID, &c.TenantID, &c.Code, &c.Name, &c.ContactType,
			&c.RegCode, &c.VATNumber, &c.Email, &c.Phone,
			&c.AddressLine1, &c.AddressLine2, &c.City, &c.PostalCode,
			&c.CountryCode, &c.PaymentTermsDays, &c.CreditLimit,
			&c.DefaultAccountID, &c.IsActive, &c.Notes,
			&c.CreatedAt, &c.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan contact: %w", err)
		}
		contacts = append(contacts, c)
	}

	return contacts, nil
}

// Update updates a contact
func (s *Service) Update(ctx context.Context, tenantID, schemaName, contactID string, req *UpdateContactRequest) (*Contact, error) {
	contact, err := s.GetByID(ctx, tenantID, schemaName, contactID)
	if err != nil {
		return nil, err
	}

	if req.Name != nil {
		contact.Name = *req.Name
	}
	if req.RegCode != nil {
		contact.RegCode = *req.RegCode
	}
	if req.VATNumber != nil {
		contact.VATNumber = *req.VATNumber
	}
	if req.Email != nil {
		contact.Email = *req.Email
	}
	if req.Phone != nil {
		contact.Phone = *req.Phone
	}
	if req.AddressLine1 != nil {
		contact.AddressLine1 = *req.AddressLine1
	}
	if req.AddressLine2 != nil {
		contact.AddressLine2 = *req.AddressLine2
	}
	if req.City != nil {
		contact.City = *req.City
	}
	if req.PostalCode != nil {
		contact.PostalCode = *req.PostalCode
	}
	if req.CountryCode != nil {
		contact.CountryCode = *req.CountryCode
	}
	if req.PaymentTermsDays != nil {
		contact.PaymentTermsDays = *req.PaymentTermsDays
	}
	if req.CreditLimit != nil {
		contact.CreditLimit = *req.CreditLimit
	}
	if req.DefaultAccountID != nil {
		contact.DefaultAccountID = req.DefaultAccountID
	}
	if req.Notes != nil {
		contact.Notes = *req.Notes
	}
	if req.IsActive != nil {
		contact.IsActive = *req.IsActive
	}
	contact.UpdatedAt = time.Now()

	query := fmt.Sprintf(`
		UPDATE %s.contacts SET
			name = $1, reg_code = $2, vat_number = $3, email = $4, phone = $5,
			address_line1 = $6, address_line2 = $7, city = $8, postal_code = $9,
			country_code = $10, payment_terms_days = $11, credit_limit = $12,
			default_account_id = $13, is_active = $14, notes = $15, updated_at = $16
		WHERE id = $17 AND tenant_id = $18
	`, schemaName)

	_, err = s.db.Exec(ctx, query,
		contact.Name, contact.RegCode, contact.VATNumber, contact.Email, contact.Phone,
		contact.AddressLine1, contact.AddressLine2, contact.City, contact.PostalCode,
		contact.CountryCode, contact.PaymentTermsDays, contact.CreditLimit,
		contact.DefaultAccountID, contact.IsActive, contact.Notes, contact.UpdatedAt,
		contactID, tenantID,
	)
	if err != nil {
		return nil, fmt.Errorf("update contact: %w", err)
	}

	return contact, nil
}

// Delete deactivates a contact (soft delete)
func (s *Service) Delete(ctx context.Context, tenantID, schemaName, contactID string) error {
	query := fmt.Sprintf(`
		UPDATE %s.contacts SET is_active = false, updated_at = $1
		WHERE id = $2 AND tenant_id = $3
	`, schemaName)

	result, err := s.db.Exec(ctx, query, time.Now(), contactID, tenantID)
	if err != nil {
		return fmt.Errorf("delete contact: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("contact not found: %s", contactID)
	}

	return nil
}
