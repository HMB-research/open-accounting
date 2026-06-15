package contacts

import "context"

// Repository defines the interface for contact data access
type Repository interface {
	Create(ctx context.Context, schemaName string, contact *Contact) error
	GetByID(ctx context.Context, schemaName, tenantID, contactID string) (*Contact, error)
	List(ctx context.Context, schemaName, tenantID string, filter *ContactFilter) ([]Contact, error)
	Update(ctx context.Context, schemaName string, contact *Contact) error
	Delete(ctx context.Context, schemaName, tenantID, contactID string) error
}
