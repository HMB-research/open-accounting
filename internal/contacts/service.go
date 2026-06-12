package contacts

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/HMB-research/open-accounting/internal/database"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Service provides contact management operations
type Service struct {
	repo Repository
}

// NewService creates a new contacts service with an ORM-backed repository.
func NewService(db *pgxpool.Pool) *Service {
	if db == nil {
		return &Service{}
	}
	gormDB, err := database.NewGormDBFromPool(context.Background(), db)
	if err != nil {
		panic(fmt.Errorf("create contacts GORM repository: %w", err))
	}
	return &Service{repo: NewGORMRepository(gormDB)}
}

// NewServiceWithRepository creates a new contacts service with a custom repository
func NewServiceWithRepository(repo Repository) *Service {
	return &Service{repo: repo}
}

// Create creates a new contact
func (s *Service) Create(ctx context.Context, tenantID string, schemaName string, req *CreateContactRequest) (*Contact, error) {
	if err := validateCreateRequest(req); err != nil {
		return nil, err
	}

	contactID := req.ID
	if contactID == "" {
		contactID = uuid.New().String()
	}
	defaultAccountID, err := normalizeOptionalContactUUIDPtr(req.DefaultAccountID, "default_account_id")
	if err != nil {
		return nil, err
	}

	contact := &Contact{
		ID:               contactID,
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
		DefaultAccountID: defaultAccountID,
		IsActive:         true,
		Notes:            req.Notes,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	// Apply defaults
	if contact.CountryCode == "" {
		contact.CountryCode = "EE"
	}
	if contact.PaymentTermsDays == 0 {
		contact.PaymentTermsDays = 14
	}

	if err := s.repo.Create(ctx, schemaName, contact); err != nil {
		return nil, fmt.Errorf("create contact: %w", err)
	}

	return contact, nil
}

// validateCreateRequest validates the create contact request
func validateCreateRequest(req *CreateContactRequest) error {
	if req.ID != "" {
		if _, err := uuid.Parse(req.ID); err != nil {
			return fmt.Errorf("id must be a valid UUID")
		}
	}
	if req.Name == "" {
		return fmt.Errorf("name is required")
	}
	if req.ContactType == "" {
		return fmt.Errorf("contact type is required")
	}
	if !isValidContactType(req.ContactType) {
		return fmt.Errorf("invalid contact type: %s", req.ContactType)
	}
	if _, err := normalizeOptionalContactUUIDPtr(req.DefaultAccountID, "default_account_id"); err != nil {
		return err
	}
	return nil
}

func normalizeOptionalContactUUIDPtr(value *string, field string) (*string, error) {
	if value == nil {
		return nil, nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil, nil
	}
	parsedID, err := uuid.Parse(trimmed)
	if err != nil {
		return nil, fmt.Errorf("%s must be a valid UUID", field)
	}
	id := parsedID.String()
	return &id, nil
}

// isValidContactType checks if the contact type is valid
func isValidContactType(ct ContactType) bool {
	switch ct {
	case ContactTypeCustomer, ContactTypeSupplier, ContactTypeBoth:
		return true
	}
	return false
}

// GetByID retrieves a contact by ID
func (s *Service) GetByID(ctx context.Context, tenantID, schemaName, contactID string) (*Contact, error) {
	contact, err := s.repo.GetByID(ctx, schemaName, tenantID, contactID)
	if err != nil {
		return nil, fmt.Errorf("get contact: %w", err)
	}
	return contact, nil
}

// List retrieves contacts with optional filtering
func (s *Service) List(ctx context.Context, tenantID, schemaName string, filter *ContactFilter) ([]Contact, error) {
	contacts, err := s.repo.List(ctx, schemaName, tenantID, filter)
	if err != nil {
		return nil, fmt.Errorf("list contacts: %w", err)
	}
	return contacts, nil
}

// Update updates a contact
func (s *Service) Update(ctx context.Context, tenantID, schemaName, contactID string, req *UpdateContactRequest) (*Contact, error) {
	contact, err := s.repo.GetByID(ctx, schemaName, tenantID, contactID)
	if err != nil {
		return nil, err
	}

	// Apply updates
	if err := applyUpdates(contact, req); err != nil {
		return nil, err
	}
	contact.UpdatedAt = time.Now()

	if err := s.repo.Update(ctx, schemaName, contact); err != nil {
		return nil, fmt.Errorf("update contact: %w", err)
	}

	return contact, nil
}

// applyUpdates applies update request fields to a contact
func applyUpdates(contact *Contact, req *UpdateContactRequest) error {
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
		defaultAccountID, err := normalizeOptionalContactUUIDPtr(req.DefaultAccountID, "default_account_id")
		if err != nil {
			return err
		}
		contact.DefaultAccountID = defaultAccountID
	}
	if req.Notes != nil {
		contact.Notes = *req.Notes
	}
	if req.IsActive != nil {
		contact.IsActive = *req.IsActive
	}
	return nil
}

// Delete deactivates a contact (soft delete)
func (s *Service) Delete(ctx context.Context, tenantID, schemaName, contactID string) error {
	if err := s.repo.Delete(ctx, schemaName, tenantID, contactID); err != nil {
		return fmt.Errorf("delete contact: %w", err)
	}
	return nil
}
