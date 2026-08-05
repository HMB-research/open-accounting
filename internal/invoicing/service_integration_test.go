//go:build integration

package invoicing

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/HMB-research/open-accounting/internal/testutil"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestService_RecordPaymentSerializesConcurrentUpdates(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	userID := testutil.CreateTestUser(t, pool, "invoicing-payment-"+uuid.NewString()+"@example.com")
	contactID := createConcurrentPaymentContact(t, pool, tenant.ID, tenant.SchemaName)
	service := NewService(pool, nil)
	ctx := context.Background()

	invoice, err := service.Create(ctx, tenant.ID, tenant.SchemaName, &CreateInvoiceRequest{
		InvoiceType: InvoiceTypeSales,
		ContactID:   contactID,
		IssueDate:   time.Now().UTC(),
		DueDate:     time.Now().UTC().AddDate(0, 0, 30),
		UserID:      userID,
		Lines: []CreateInvoiceLineRequest{{
			Description: "Concurrent payment test",
			Quantity:    decimal.NewFromInt(1),
			UnitPrice:   decimal.NewFromInt(100),
			VATRate:     decimal.NewFromInt(22),
		}},
	})
	require.NoError(t, err)
	require.NoError(t, service.Send(ctx, tenant.ID, tenant.SchemaName, invoice.ID))

	amounts := []decimal.Decimal{decimal.NewFromInt(40), decimal.NewFromInt(30)}
	errs := make(chan error, len(amounts))
	var waitGroup sync.WaitGroup
	for _, amount := range amounts {
		waitGroup.Add(1)
		go func(amount decimal.Decimal) {
			defer waitGroup.Done()
			errs <- service.RecordPayment(ctx, tenant.ID, tenant.SchemaName, invoice.ID, amount)
		}(amount)
	}
	waitGroup.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}

	updated, err := service.GetByID(ctx, tenant.ID, tenant.SchemaName, invoice.ID)
	require.NoError(t, err)
	require.True(t, updated.AmountPaid.Equal(decimal.NewFromInt(70)), "AmountPaid = %s", updated.AmountPaid)
	require.Equal(t, StatusPartiallyPaid, updated.Status)
}

func createConcurrentPaymentContact(t *testing.T, pool *pgxpool.Pool, tenantID, schemaName string) string {
	t.Helper()
	contactID := uuid.NewString()
	_, err := pool.Exec(context.Background(), `
		INSERT INTO `+schemaName+`.contacts (id, tenant_id, contact_type, name, created_at, updated_at)
		VALUES ($1, $2, 'CUSTOMER', $3, NOW(), NOW())
	`, contactID, tenantID, "Concurrent payment contact "+contactID)
	require.NoError(t, err)
	return contactID
}
