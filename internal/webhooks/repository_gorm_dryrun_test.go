package webhooks

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/HMB-research/open-accounting/internal/models"
	"github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type webhookDryRunConnPool struct{}

func (webhookDryRunConnPool) PrepareContext(context.Context, string) (*sql.Stmt, error) {
	return nil, errors.New("dry-run webhook tests should not prepare statements")
}

func (webhookDryRunConnPool) ExecContext(context.Context, string, ...interface{}) (sql.Result, error) {
	return nil, errors.New("dry-run webhook tests should not execute statements")
}

func (webhookDryRunConnPool) QueryContext(context.Context, string, ...interface{}) (*sql.Rows, error) {
	return nil, errors.New("dry-run webhook tests should not query rows")
}

func (webhookDryRunConnPool) QueryRowContext(context.Context, string, ...interface{}) *sql.Row {
	return nil
}

func (webhookDryRunConnPool) BeginTx(context.Context, *sql.TxOptions) (gorm.ConnPool, error) {
	return &webhookDryRunTx{}, nil
}

type webhookDryRunTx struct {
	webhookDryRunConnPool
}

func (*webhookDryRunTx) Commit() error {
	return nil
}

func (*webhookDryRunTx) Rollback() error {
	return nil
}

type webhookDryRunDBOption func(t *testing.T, db *gorm.DB)

type webhookDryRunFixtures struct {
	endpoint   *models.WebhookEndpoint
	endpoints  []models.WebhookEndpoint
	deliveries []models.WebhookDelivery
}

var webhookDryRunCallbackID uint64

func newWebhookDryRunDB(t *testing.T, opts ...webhookDryRunDBOption) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(postgres.New(postgres.Config{Conn: webhookDryRunConnPool{}}), &gorm.Config{
		DisableAutomaticPing:   true,
		DryRun:                 true,
		Logger:                 logger.Default.LogMode(logger.Silent),
		SkipDefaultTransaction: true,
	})
	require.NoError(t, err)

	for _, opt := range opts {
		opt(t, db)
	}
	return db
}

func withWebhookDryRunFixtures(fixtures webhookDryRunFixtures) webhookDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		err := db.Callback().Query().After("gorm:query").Register(webhookDryRunCallbackName("query_fixtures"), func(tx *gorm.DB) {
			switch dest := tx.Statement.Dest.(type) {
			case *models.WebhookEndpoint:
				if fixtures.endpoint != nil {
					*dest = *fixtures.endpoint
					tx.RowsAffected = 1
				}
			case *[]models.WebhookEndpoint:
				*dest = append([]models.WebhookEndpoint(nil), fixtures.endpoints...)
				tx.RowsAffected = int64(len(fixtures.endpoints))
			case *[]models.WebhookDelivery:
				*dest = append([]models.WebhookDelivery(nil), fixtures.deliveries...)
				tx.RowsAffected = int64(len(fixtures.deliveries))
			}
		})
		require.NoError(t, err)
	}
}

func withWebhookDryRunQueryError(expectedErr error) webhookDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		err := db.Callback().Query().Before("gorm:query").Register(webhookDryRunCallbackName("query_error"), func(tx *gorm.DB) {
			tx.AddError(expectedErr)
		})
		require.NoError(t, err)
	}
}

func withWebhookDryRunCreateError(expectedErr error) webhookDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		err := db.Callback().Create().Before("gorm:create").Register(webhookDryRunCallbackName("create_error"), func(tx *gorm.DB) {
			tx.AddError(expectedErr)
		})
		require.NoError(t, err)
	}
}

func withWebhookDryRunUpdateError(expectedErr error) webhookDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		err := db.Callback().Update().Before("gorm:update").Register(webhookDryRunCallbackName("update_error"), func(tx *gorm.DB) {
			tx.AddError(expectedErr)
		})
		require.NoError(t, err)
	}
}

func withWebhookDryRunDeleteRows(rows int64) webhookDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		err := db.Callback().Delete().After("gorm:delete").Register(webhookDryRunCallbackName("delete_rows"), func(tx *gorm.DB) {
			tx.RowsAffected = rows
		})
		require.NoError(t, err)
	}
}

func withWebhookDryRunDeleteError(expectedErr error) webhookDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		err := db.Callback().Delete().Before("gorm:delete").Register(webhookDryRunCallbackName("delete_error"), func(tx *gorm.DB) {
			tx.AddError(expectedErr)
		})
		require.NoError(t, err)
	}
}

func webhookDryRunCallbackName(suffix string) string {
	id := atomic.AddUint64(&webhookDryRunCallbackID, 1)
	return fmt.Sprintf("webhook_dryrun:%d:%s", id, suffix)
}

func TestGORMRepositoryDryRunOperations(t *testing.T) {
	ctx := context.Background()
	tenantID := "tenant-1"
	endpointID := "endpoint-1"
	now := time.Date(2026, time.June, 25, 13, 0, 0, 0, time.UTC)
	statusCode := 202
	endpointModel := models.WebhookEndpoint{
		ID:             endpointID,
		TenantID:       tenantID,
		Name:           "Invoice events",
		URL:            "https://example.com/webhooks",
		Events:         pq.StringArray{"invoice.created", "payment.received"},
		Secret:         "stored-secret",
		IsActive:       true,
		LastDeliveryAt: &now,
		CreatedAt:      now.Add(-time.Hour),
		UpdatedAt:      now,
	}
	deliveryModel := models.WebhookDelivery{
		ID:            "delivery-1",
		TenantID:      tenantID,
		EndpointID:    endpointID,
		EventID:       "event-1",
		EventType:     "invoice.created",
		Status:        DeliveryStatusSucceeded,
		StatusCode:    &statusCode,
		AttemptNumber: 1,
		RequestBody:   []byte(`{"invoice_id":"invoice-1"}`),
		ResponseBody:  "accepted",
		DeliveredAt:   now,
		CreatedAt:     now,
	}
	repo := NewGORMRepository(newWebhookDryRunDB(t,
		withWebhookDryRunFixtures(webhookDryRunFixtures{
			endpoint:   &endpointModel,
			endpoints:  []models.WebhookEndpoint{endpointModel},
			deliveries: []models.WebhookDelivery{deliveryModel},
		}),
		withWebhookDryRunDeleteRows(1),
	))

	activeEndpoints, err := repo.ListEndpoints(ctx, tenantID, true)
	require.NoError(t, err)
	require.Len(t, activeEndpoints, 1)
	assert.Equal(t, endpointID, activeEndpoints[0].ID)
	assert.True(t, activeEndpoints[0].SecretSet)

	allEndpoints, err := repo.ListEndpoints(ctx, tenantID, false)
	require.NoError(t, err)
	require.Len(t, allEndpoints, 1)

	endpoint, err := repo.GetEndpoint(ctx, tenantID, endpointID)
	require.NoError(t, err)
	assert.Equal(t, endpointModel.Name, endpoint.Name)

	require.NoError(t, repo.CreateEndpoint(ctx, endpoint))
	require.NoError(t, repo.UpdateEndpoint(ctx, endpoint))

	deletedRows, err := repo.DeleteEndpoint(ctx, tenantID, endpointID)
	require.NoError(t, err)
	assert.Equal(t, int64(1), deletedRows)

	delivery := modelToDelivery(&deliveryModel)
	require.NoError(t, repo.CreateDelivery(ctx, delivery))

	deliveries, err := repo.ListDeliveries(ctx, tenantID, endpointID, 10)
	require.NoError(t, err)
	require.Len(t, deliveries, 1)
	assert.Equal(t, deliveryModel.ID, deliveries[0].ID)
	assert.Equal(t, statusCode, deliveries[0].StatusCode)

	require.NoError(t, repo.UpdateEndpointLastDelivery(ctx, tenantID, endpointID, now))
}

func TestGORMRepositoryDryRunErrors(t *testing.T) {
	ctx := context.Background()
	tenantID := "tenant-1"
	endpointID := "endpoint-1"
	now := time.Date(2026, time.June, 25, 14, 0, 0, 0, time.UTC)
	dbErr := errors.New("dry-run database error")
	endpoint := &Endpoint{
		ID:        endpointID,
		TenantID:  tenantID,
		Name:      "Invoice events",
		URL:       "https://example.com/webhooks",
		Events:    []string{"invoice.created"},
		Secret:    "stored-secret",
		IsActive:  true,
		CreatedAt: now,
		UpdatedAt: now,
	}
	delivery := &Delivery{
		ID:            "delivery-1",
		TenantID:      tenantID,
		EndpointID:    endpointID,
		EventID:       "event-1",
		EventType:     "invoice.created",
		Status:        DeliveryStatusFailed,
		AttemptNumber: 1,
		Error:         "timeout",
		DeliveredAt:   now,
		CreatedAt:     now,
	}

	t.Run("ListEndpoints wraps query errors", func(t *testing.T) {
		repo := NewGORMRepository(newWebhookDryRunDB(t, withWebhookDryRunQueryError(dbErr)))

		endpoints, err := repo.ListEndpoints(ctx, tenantID, true)

		assert.Nil(t, endpoints)
		require.ErrorContains(t, err, "list webhook endpoints")
		assert.ErrorIs(t, err, dbErr)
	})

	t.Run("GetEndpoint maps not found", func(t *testing.T) {
		repo := NewGORMRepository(newWebhookDryRunDB(t, withWebhookDryRunQueryError(gorm.ErrRecordNotFound)))

		got, err := repo.GetEndpoint(ctx, tenantID, endpointID)

		assert.Nil(t, got)
		require.ErrorContains(t, err, "webhook endpoint not found")
	})

	t.Run("GetEndpoint wraps query errors", func(t *testing.T) {
		repo := NewGORMRepository(newWebhookDryRunDB(t, withWebhookDryRunQueryError(dbErr)))

		got, err := repo.GetEndpoint(ctx, tenantID, endpointID)

		assert.Nil(t, got)
		require.ErrorContains(t, err, "get webhook endpoint")
		assert.ErrorIs(t, err, dbErr)
	})

	t.Run("CreateEndpoint wraps create errors", func(t *testing.T) {
		repo := NewGORMRepository(newWebhookDryRunDB(t, withWebhookDryRunCreateError(dbErr)))

		err := repo.CreateEndpoint(ctx, endpoint)

		require.ErrorContains(t, err, "create webhook endpoint")
		assert.ErrorIs(t, err, dbErr)
	})

	t.Run("UpdateEndpoint wraps update errors", func(t *testing.T) {
		repo := NewGORMRepository(newWebhookDryRunDB(t, withWebhookDryRunUpdateError(dbErr)))

		err := repo.UpdateEndpoint(ctx, endpoint)

		require.ErrorContains(t, err, "update webhook endpoint")
		assert.ErrorIs(t, err, dbErr)
	})

	t.Run("DeleteEndpoint wraps delete errors", func(t *testing.T) {
		repo := NewGORMRepository(newWebhookDryRunDB(t, withWebhookDryRunDeleteError(dbErr)))

		rows, err := repo.DeleteEndpoint(ctx, tenantID, endpointID)

		assert.Zero(t, rows)
		require.ErrorContains(t, err, "delete webhook endpoint")
		assert.ErrorIs(t, err, dbErr)
	})

	t.Run("CreateDelivery wraps create errors", func(t *testing.T) {
		repo := NewGORMRepository(newWebhookDryRunDB(t, withWebhookDryRunCreateError(dbErr)))

		err := repo.CreateDelivery(ctx, delivery)

		require.ErrorContains(t, err, "create webhook delivery")
		assert.ErrorIs(t, err, dbErr)
	})

	t.Run("ListDeliveries wraps query errors", func(t *testing.T) {
		repo := NewGORMRepository(newWebhookDryRunDB(t, withWebhookDryRunQueryError(dbErr)))

		deliveries, err := repo.ListDeliveries(ctx, tenantID, endpointID, 10)

		assert.Nil(t, deliveries)
		require.ErrorContains(t, err, "list webhook deliveries")
		assert.ErrorIs(t, err, dbErr)
	})

	t.Run("UpdateEndpointLastDelivery wraps update errors", func(t *testing.T) {
		repo := NewGORMRepository(newWebhookDryRunDB(t, withWebhookDryRunUpdateError(dbErr)))

		err := repo.UpdateEndpointLastDelivery(ctx, tenantID, endpointID, now)

		require.ErrorContains(t, err, "update webhook endpoint last delivery")
		assert.ErrorIs(t, err, dbErr)
	})
}
