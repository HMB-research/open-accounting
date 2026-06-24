package contacts

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGORMRepositoryNilDatabase(t *testing.T) {
	repo := NewGORMRepository(nil)
	ctx := context.Background()
	schemaName := "tenant_schema"
	tenantID := "tenant-1"

	require.NotNil(t, repo)
	assert.Nil(t, repo.db)

	tests := []struct {
		name string
		run  func(t *testing.T) error
	}{
		{
			name: "Create",
			run: func(t *testing.T) error {
				return repo.Create(ctx, schemaName, &Contact{TenantID: tenantID})
			},
		},
		{
			name: "GetByID",
			run: func(t *testing.T) error {
				contact, err := repo.GetByID(ctx, schemaName, tenantID, "contact-1")
				assert.Nil(t, contact)
				return err
			},
		},
		{
			name: "List",
			run: func(t *testing.T) error {
				contacts, err := repo.List(ctx, schemaName, tenantID, &ContactFilter{
					ContactType: ContactTypeCustomer,
					ActiveOnly:  true,
					Search:      "acme",
				})
				assert.Nil(t, contacts)
				return err
			},
		},
		{
			name: "Update",
			run: func(t *testing.T) error {
				return repo.Update(ctx, schemaName, &Contact{ID: "contact-1", TenantID: tenantID})
			},
		},
		{
			name: "Delete",
			run: func(t *testing.T) error {
				return repo.Delete(ctx, schemaName, tenantID, "contact-1")
			},
		},
		{
			name: "tenantTable",
			run: func(t *testing.T) error {
				table, err := repo.tenantTable(ctx, schemaName, "contacts")
				assert.Nil(t, table)
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.run(t)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "contacts repository database is not configured")
		})
	}
}
