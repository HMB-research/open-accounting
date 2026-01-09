package contacts

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository defines the interface for contact data access
type Repository interface {
	Create(ctx context.Context, schemaName string, contact *Contact) error
	GetByID(ctx context.Context, schemaName, tenantID, contactID string) (*Contact, error)
	List(ctx context.Context, schemaName, tenantID string, filter *ContactFilter) ([]Contact, error)
	Update(ctx context.Context, schemaName string, contact *Contact) error
	Delete(ctx context.Context, schemaName, tenantID, contactID string) error
}

// PostgresRepository implements Repository using PostgreSQL
type PostgresRepository struct {
	db *pgxpool.Pool
}

// NewPostgresRepository creates a new PostgreSQL repository
func NewPostgresRepository(db *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{db: db}
}

// Create inserts a new contact
func (r *PostgresRepository) Create(ctx context.Context, schemaName string, contact *Contact) error {
	query := fmt.Sprintf(`
		INSERT INTO %s.contacts (
			id, tenant_id, code, name, contact_type, reg_code, vat_number,
			email, phone, address_line1, address_line2, city, postal_code,
			country_code, payment_terms_days, credit_limit, default_account_id,
			is_active, notes, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21)
	`, schemaName)

	_, err := r.db.Exec(ctx, query,
		contact.ID, contact.TenantID, contact.Code, contact.Name, contact.ContactType,
		contact.RegCode, contact.VATNumber, contact.Email, contact.Phone,
		contact.AddressLine1, contact.AddressLine2, contact.City, contact.PostalCode,
		contact.CountryCode, contact.PaymentTermsDays, contact.CreditLimit,
		contact.DefaultAccountID, contact.IsActive, contact.Notes,
		contact.CreatedAt, contact.UpdatedAt,
	)
	return err
}

// GetByID retrieves a contact by ID
func (r *PostgresRepository) GetByID(ctx context.Context, schemaName, tenantID, contactID string) (*Contact, error) {
	query := fmt.Sprintf(`
		SELECT id, tenant_id, COALESCE(code, ''), name, contact_type,
		       COALESCE(reg_code, ''), COALESCE(vat_number, ''),
		       COALESCE(email, ''), COALESCE(phone, ''),
		       COALESCE(address_line1, ''), COALESCE(address_line2, ''),
		       COALESCE(city, ''), COALESCE(postal_code, ''),
		       country_code, payment_terms_days, COALESCE(credit_limit, 0), default_account_id,
		       is_active, COALESCE(notes, ''), created_at, updated_at
		FROM %s.contacts
		WHERE id = $1 AND tenant_id = $2
	`, schemaName)

	var c Contact
	err := r.db.QueryRow(ctx, query, contactID, tenantID).Scan(
		&c.ID, &c.TenantID, &c.Code, &c.Name, &c.ContactType,
		&c.RegCode, &c.VATNumber, &c.Email, &c.Phone,
		&c.AddressLine1, &c.AddressLine2, &c.City, &c.PostalCode,
		&c.CountryCode, &c.PaymentTermsDays, &c.CreditLimit,
		&c.DefaultAccountID, &c.IsActive, &c.Notes,
		&c.CreatedAt, &c.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, ErrContactNotFound
	}
	if err != nil {
		return nil, err
	}

	return &c, nil
}

// List retrieves contacts with optional filtering
func (r *PostgresRepository) List(ctx context.Context, schemaName, tenantID string, filter *ContactFilter) ([]Contact, error) {
	query := fmt.Sprintf(`
		SELECT id, tenant_id, COALESCE(code, ''), name, contact_type,
		       COALESCE(reg_code, ''), COALESCE(vat_number, ''),
		       COALESCE(email, ''), COALESCE(phone, ''),
		       COALESCE(address_line1, ''), COALESCE(address_line2, ''),
		       COALESCE(city, ''), COALESCE(postal_code, ''),
		       country_code, payment_terms_days, COALESCE(credit_limit, 0), default_account_id,
		       is_active, COALESCE(notes, ''), created_at, updated_at
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

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	contacts := []Contact{}
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
			return nil, err
		}
		contacts = append(contacts, c)
	}

	return contacts, nil
}

// Update updates a contact
func (r *PostgresRepository) Update(ctx context.Context, schemaName string, contact *Contact) error {
	query := fmt.Sprintf(`
		UPDATE %s.contacts SET
			name = $1, reg_code = $2, vat_number = $3, email = $4, phone = $5,
			address_line1 = $6, address_line2 = $7, city = $8, postal_code = $9,
			country_code = $10, payment_terms_days = $11, credit_limit = $12,
			default_account_id = $13, is_active = $14, notes = $15, updated_at = $16
		WHERE id = $17 AND tenant_id = $18
	`, schemaName)

	_, err := r.db.Exec(ctx, query,
		contact.Name, contact.RegCode, contact.VATNumber, contact.Email, contact.Phone,
		contact.AddressLine1, contact.AddressLine2, contact.City, contact.PostalCode,
		contact.CountryCode, contact.PaymentTermsDays, contact.CreditLimit,
		contact.DefaultAccountID, contact.IsActive, contact.Notes, contact.UpdatedAt,
		contact.ID, contact.TenantID,
	)
	return err
}

// Delete soft-deletes a contact
func (r *PostgresRepository) Delete(ctx context.Context, schemaName, tenantID, contactID string) error {
	query := fmt.Sprintf(`
		UPDATE %s.contacts SET is_active = false, updated_at = $1
		WHERE id = $2 AND tenant_id = $3
	`, schemaName)

	result, err := r.db.Exec(ctx, query, time.Now(), contactID, tenantID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrContactNotFound
	}

	return nil
}
