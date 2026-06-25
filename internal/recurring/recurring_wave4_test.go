package recurring

import (
	"context"
	"errors"
	"testing"

	"github.com/HMB-research/open-accounting/internal/contacts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRecurringWave4NewServiceNilRepositoryGuards(t *testing.T) {
	service := NewService(nil, nil, nil, nil, nil, nil)

	if service == nil {
		t.Fatal("NewService(nil) returned nil")
	}
	if service.repo != nil {
		t.Fatalf("repo = %#v, want nil for nil pool", service.repo)
	}

	_, err := service.List(context.Background(), "tenant-1", "tenant_schema", true)
	require.ErrorContains(t, err, "repository not available")
}

func TestRecurringWave4ListAndDeleteRepositoryErrors(t *testing.T) {
	ctx := context.Background()

	repo := NewMockRepository()
	repo.listErr = errors.New("list failed")
	service := NewServiceWithDependencies(repo, nil, nil, nil, nil, nil)
	_, err := service.List(ctx, "tenant-1", "tenant_schema", false)
	require.ErrorContains(t, err, "list recurring invoices")

	repo = NewMockRepository()
	repo.deleteErr = errors.New("delete failed")
	service = NewServiceWithDependencies(repo, nil, nil, nil, nil, nil)
	err = service.Delete(ctx, "tenant-1", "tenant_schema", "recurring-1")
	require.ErrorContains(t, err, "delete recurring invoice")
}

func TestRecurringWave4ImportExistingTemplateWithContactID(t *testing.T) {
	contactID := "11111111-1111-4111-8111-111111111111"
	repo := NewMockRepository()
	repo.recurring["tenant-1:existing"] = &RecurringInvoice{
		ID:       "existing",
		TenantID: "tenant-1",
		Name:     "Monthly Retainer",
		IsActive: true,
	}
	service := NewServiceWithDependencies(repo, nil, nil, nil, nil, nil)

	result, err := service.ImportCSV(context.Background(), "tenant-1", "tenant_schema", []contacts.Contact{
		{ID: contactID, Name: "Acme OU"},
	}, nil, &ImportRecurringInvoicesRequest{
		CSVContent: "name,contact_id,frequency,start_date,line_description,quantity,unit_price,vat_rate\n" +
			"Monthly Retainer," + contactID + ",MONTHLY,2026-03-01,Consulting,1,100,22\n",
	})

	require.NoError(t, err)
	assert.Zero(t, result.TemplatesCreated)
	assert.Equal(t, 1, result.RowsSkipped)
	require.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0].Message, "already exists")
}
