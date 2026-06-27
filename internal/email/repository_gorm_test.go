package email

import (
	"context"
	"testing"
	"time"

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

	sentAt := time.Date(2026, time.January, 5, 14, 30, 0, 0, time.UTC)

	tests := []struct {
		name string
		run  func(t *testing.T) error
	}{
		{
			name: "GetTenantSettings",
			run: func(t *testing.T) error {
				settings, err := repo.GetTenantSettings(ctx, tenantID)
				assert.Nil(t, settings)
				return err
			},
		},
		{
			name: "UpdateTenantSettings",
			run: func(t *testing.T) error {
				return repo.UpdateTenantSettings(ctx, tenantID, []byte(`{"smtp_host":"smtp.example.com"}`))
			},
		},
		{
			name: "GetTemplate",
			run: func(t *testing.T) error {
				template, err := repo.GetTemplate(ctx, schemaName, tenantID, TemplateInvoiceSend)
				assert.Nil(t, template)
				return err
			},
		},
		{
			name: "ListTemplates",
			run: func(t *testing.T) error {
				templates, err := repo.ListTemplates(ctx, schemaName, tenantID)
				assert.Nil(t, templates)
				return err
			},
		},
		{
			name: "UpsertTemplate",
			run: func(t *testing.T) error {
				return repo.UpsertTemplate(ctx, schemaName, &EmailTemplate{
					TenantID:     tenantID,
					TemplateType: TemplateInvoiceSend,
				})
			},
		},
		{
			name: "CreateEmailLog",
			run: func(t *testing.T) error {
				return repo.CreateEmailLog(ctx, schemaName, &EmailLog{
					TenantID: tenantID,
					Status:   StatusPending,
				})
			},
		},
		{
			name: "UpdateEmailLogStatus",
			run: func(t *testing.T) error {
				return repo.UpdateEmailLogStatus(ctx, schemaName, "log-1", StatusSent, &sentAt, "")
			},
		},
		{
			name: "GetEmailLog",
			run: func(t *testing.T) error {
				logs, err := repo.GetEmailLog(ctx, schemaName, tenantID, 0)
				assert.Nil(t, logs)
				return err
			},
		},
		{
			name: "tenantTable",
			run: func(t *testing.T) error {
				table, err := repo.tenantTable(ctx, schemaName, "email_log")
				assert.Nil(t, table)
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.run(t)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "email repository database is not configured")
		})
	}
}

func TestTenantSettingsTableName(t *testing.T) {
	assert.Equal(t, "tenants", tenantSettings{}.TableName())
}

func TestEmailLogRecordMappingRoundTrip(t *testing.T) {
	sentAt := time.Date(2026, time.February, 6, 11, 20, 0, 0, time.UTC)
	createdAt := time.Date(2026, time.February, 6, 11, 0, 0, 0, time.UTC)
	log := &EmailLog{
		ID:             "log-id",
		TenantID:       "tenant-id",
		EmailType:      "invoice",
		RecipientEmail: "customer@example.com",
		RecipientName:  "Customer",
		Subject:        "Invoice INV-001",
		Status:         StatusSent,
		SentAt:         &sentAt,
		ErrorMessage:   "",
		RelatedID:      "invoice-id",
		CreatedAt:      createdAt,
	}

	record := emailLogToRecord(log)
	require.NotNil(t, record.RelatedID)
	assert.Equal(t, log.RelatedID, *record.RelatedID)

	roundTrip := record.toEmailLog()
	assert.Equal(t, *log, roundTrip)
}

func TestEmailLogRecordMappingOmitsBlankRelatedID(t *testing.T) {
	createdAt := time.Date(2026, time.March, 7, 9, 15, 0, 0, time.UTC)
	log := &EmailLog{
		ID:             "log-id",
		TenantID:       "tenant-id",
		EmailType:      "payment",
		RecipientEmail: "payer@example.com",
		Subject:        "Payment receipt",
		Status:         StatusPending,
		RelatedID:      "   ",
		CreatedAt:      createdAt,
	}

	record := emailLogToRecord(log)
	assert.Nil(t, record.RelatedID)

	roundTrip := record.toEmailLog()
	log.RelatedID = ""
	assert.Equal(t, *log, roundTrip)
}
