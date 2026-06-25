package email

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type emailDryRunConnPool struct{}

func (emailDryRunConnPool) PrepareContext(context.Context, string) (*sql.Stmt, error) {
	return nil, errors.New("dry-run email tests should not prepare statements")
}

func (emailDryRunConnPool) ExecContext(context.Context, string, ...interface{}) (sql.Result, error) {
	return nil, errors.New("dry-run email tests should not execute statements")
}

func (emailDryRunConnPool) QueryContext(context.Context, string, ...interface{}) (*sql.Rows, error) {
	return nil, errors.New("dry-run email tests should not query rows")
}

func (emailDryRunConnPool) QueryRowContext(context.Context, string, ...interface{}) *sql.Row {
	return nil
}

func (emailDryRunConnPool) BeginTx(context.Context, *sql.TxOptions) (gorm.ConnPool, error) {
	return &emailDryRunTx{}, nil
}

type emailDryRunTx struct {
	emailDryRunConnPool
}

func (*emailDryRunTx) Commit() error {
	return nil
}

func (*emailDryRunTx) Rollback() error {
	return nil
}

type emailDryRunDBOption func(t *testing.T, db *gorm.DB)

type emailDryRunFixtures struct {
	tenant    *tenantSettings
	template  *EmailTemplate
	templates []EmailTemplate
	logs      []emailLogRecord
}

type emailDryRunRecorder struct {
	queries          []string
	creates          []string
	updates          []string
	createdTemplates []EmailTemplate
	createdLogs      []emailLogRecord
}

func newEmailDryRunDB(t *testing.T, opts ...emailDryRunDBOption) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(postgres.New(postgres.Config{Conn: emailDryRunConnPool{}}), &gorm.Config{
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

func withEmailDryRunFixtures(fixtures emailDryRunFixtures, recorder *emailDryRunRecorder) emailDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		err := db.Callback().Query().After("gorm:query").Register(emailDryRunCallbackName(t, "query_fixtures"), func(tx *gorm.DB) {
			if recorder != nil {
				recorder.queries = append(recorder.queries, tx.Statement.SQL.String())
			}
			switch dest := tx.Statement.Dest.(type) {
			case *tenantSettings:
				if fixtures.tenant != nil {
					*dest = *fixtures.tenant
					tx.RowsAffected = 1
				}
			case *EmailTemplate:
				if fixtures.template != nil {
					*dest = *fixtures.template
					tx.RowsAffected = 1
				}
			case *[]EmailTemplate:
				if fixtures.templates != nil {
					*dest = append([]EmailTemplate(nil), fixtures.templates...)
					tx.RowsAffected = int64(len(fixtures.templates))
				}
			case *[]emailLogRecord:
				if fixtures.logs != nil {
					*dest = append([]emailLogRecord(nil), fixtures.logs...)
					tx.RowsAffected = int64(len(fixtures.logs))
				}
			}
		})
		require.NoError(t, err)
	}
}

func withEmailDryRunQueryErrors(queryErrors ...error) emailDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		var index int
		err := db.Callback().Query().Before("gorm:query").Register(emailDryRunCallbackName(t, "query_error"), func(tx *gorm.DB) {
			if len(queryErrors) == 0 {
				return
			}
			errIndex := index
			if errIndex >= len(queryErrors) {
				errIndex = len(queryErrors) - 1
			}
			index++
			if queryErrors[errIndex] != nil {
				tx.AddError(queryErrors[errIndex])
			}
		})
		require.NoError(t, err)
	}
}

func withEmailDryRunCreateCapture(recorder *emailDryRunRecorder) emailDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		err := db.Callback().Create().After("gorm:create").Register(emailDryRunCallbackName(t, "create_capture"), func(tx *gorm.DB) {
			if recorder != nil {
				recorder.creates = append(recorder.creates, tx.Statement.SQL.String())
				switch dest := tx.Statement.Dest.(type) {
				case *EmailTemplate:
					recorder.createdTemplates = append(recorder.createdTemplates, *dest)
				case *emailLogRecord:
					recorder.createdLogs = append(recorder.createdLogs, *dest)
				}
			}
			if tx.RowsAffected == 0 {
				tx.RowsAffected = 1
			}
		})
		require.NoError(t, err)
	}
}

func withEmailDryRunCreateErrors(createErrors ...error) emailDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		var index int
		err := db.Callback().Create().Before("gorm:create").Register(emailDryRunCallbackName(t, "create_error"), func(tx *gorm.DB) {
			if len(createErrors) == 0 {
				return
			}
			errIndex := index
			if errIndex >= len(createErrors) {
				errIndex = len(createErrors) - 1
			}
			index++
			if createErrors[errIndex] != nil {
				tx.AddError(createErrors[errIndex])
			}
		})
		require.NoError(t, err)
	}
}

func withEmailDryRunUpdateRows(recorder *emailDryRunRecorder, rows ...int64) emailDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		var index int
		err := db.Callback().Update().After("gorm:update").Register(emailDryRunCallbackName(t, "update_rows"), func(tx *gorm.DB) {
			if recorder != nil {
				recorder.updates = append(recorder.updates, tx.Statement.SQL.String())
			}
			rowCount := int64(0)
			if len(rows) > 0 {
				rowCount = rows[len(rows)-1]
				if index < len(rows) {
					rowCount = rows[index]
				}
				index++
			}
			tx.RowsAffected = rowCount
		})
		require.NoError(t, err)
	}
}

func withEmailDryRunUpdateErrors(updateErrors ...error) emailDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		var index int
		err := db.Callback().Update().Before("gorm:update").Register(emailDryRunCallbackName(t, "update_error"), func(tx *gorm.DB) {
			if len(updateErrors) == 0 {
				return
			}
			errIndex := index
			if errIndex >= len(updateErrors) {
				errIndex = len(updateErrors) - 1
			}
			index++
			if updateErrors[errIndex] != nil {
				tx.AddError(updateErrors[errIndex])
			}
		})
		require.NoError(t, err)
	}
}

func emailDryRunCallbackName(t *testing.T, suffix string) string {
	replacer := strings.NewReplacer("/", "_", " ", "_", ":", "_")
	return "email_test:" + replacer.Replace(t.Name()) + ":" + suffix
}

func assertEmailRecordedSQLContains(t *testing.T, statements []string, fragments ...string) {
	t.Helper()

	joined := strings.Join(statements, "\n")
	for _, fragment := range fragments {
		assert.Contains(t, joined, fragment)
	}
}

func TestNewServiceWithNilDatabaseUsesDefaultMailer(t *testing.T) {
	service := NewService(nil)

	require.NotNil(t, service)
	assert.Nil(t, service.repo)
	assert.IsType(t, &DefaultMailSender{}, service.mailer)
}

func TestGORMRepositoryDryRunTenantSettings(t *testing.T) {
	ctx := context.Background()
	tenantID := "tenant-1"
	settings := []byte(`{"smtp_host":"smtp.example.com"}`)

	t.Run("reads and updates settings", func(t *testing.T) {
		recorder := &emailDryRunRecorder{}
		repo := NewGORMRepository(newEmailDryRunDB(t,
			withEmailDryRunFixtures(emailDryRunFixtures{
				tenant: &tenantSettings{ID: tenantID, Settings: settings},
			}, recorder),
			withEmailDryRunUpdateRows(recorder, 1),
		))

		got, err := repo.GetTenantSettings(ctx, tenantID)
		require.NoError(t, err)
		assert.Equal(t, settings, got)
		require.NoError(t, repo.UpdateTenantSettings(ctx, tenantID, settings))

		assertEmailRecordedSQLContains(t, recorder.queries,
			`FROM "public"."tenants"`,
			`id = $1`,
		)
		assertEmailRecordedSQLContains(t, recorder.updates,
			`UPDATE "public"."tenants"`,
			`"settings"`,
			`"updated_at"`,
		)
	})

	t.Run("maps missing settings", func(t *testing.T) {
		repo := NewGORMRepository(newEmailDryRunDB(t, withEmailDryRunQueryErrors(gorm.ErrRecordNotFound)))

		got, err := repo.GetTenantSettings(ctx, tenantID)

		assert.Nil(t, got)
		assert.ErrorIs(t, err, ErrSettingsNotFound)
	})

	t.Run("returns tenant query errors", func(t *testing.T) {
		expectedErr := errors.New("tenant query failed")
		repo := NewGORMRepository(newEmailDryRunDB(t, withEmailDryRunQueryErrors(expectedErr)))

		got, err := repo.GetTenantSettings(ctx, tenantID)

		assert.Nil(t, got)
		assert.ErrorIs(t, err, expectedErr)
	})

	t.Run("returns update errors", func(t *testing.T) {
		expectedErr := errors.New("tenant update failed")
		repo := NewGORMRepository(newEmailDryRunDB(t, withEmailDryRunUpdateErrors(expectedErr)))

		err := repo.UpdateTenantSettings(ctx, tenantID, settings)

		assert.ErrorIs(t, err, expectedErr)
	})
}

func TestGORMRepositoryDryRunTemplates(t *testing.T) {
	ctx := context.Background()
	schemaName := "tenant_email"
	tenantID := "tenant-1"
	now := time.Date(2026, time.June, 25, 9, 30, 0, 0, time.UTC)
	persisted := EmailTemplate{
		ID:           "template-1",
		TenantID:     tenantID,
		TemplateType: TemplateInvoiceSend,
		Subject:      "Invoice {{.InvoiceNumber}}",
		BodyHTML:     "<p>Hello</p>",
		BodyText:     "Hello",
		IsActive:     true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	t.Run("reads lists and upserts templates", func(t *testing.T) {
		recorder := &emailDryRunRecorder{}
		repo := NewGORMRepository(newEmailDryRunDB(t,
			withEmailDryRunFixtures(emailDryRunFixtures{
				template:  &persisted,
				templates: []EmailTemplate{persisted},
			}, recorder),
			withEmailDryRunCreateCapture(recorder),
		))

		got, err := repo.GetTemplate(ctx, schemaName, tenantID, TemplateInvoiceSend)
		require.NoError(t, err)
		assert.Equal(t, persisted.ID, got.ID)

		templates, err := repo.ListTemplates(ctx, schemaName, tenantID)
		require.NoError(t, err)
		require.Len(t, templates, 1)
		assert.Equal(t, TemplateInvoiceSend, templates[0].TemplateType)

		template := persisted
		template.ID = ""
		require.NoError(t, repo.UpsertTemplate(ctx, schemaName, &template))
		assert.Equal(t, persisted.ID, template.ID)

		assertEmailRecordedSQLContains(t, recorder.queries,
			`FROM "tenant_email"."email_templates"`,
			`tenant_id = $1 AND template_type = $2`,
			`ORDER BY template_type`,
		)
		assertEmailRecordedSQLContains(t, recorder.creates,
			`INSERT INTO "tenant_email"."email_templates"`,
			`ON CONFLICT ("tenant_id","template_type") DO UPDATE`,
		)
	})

	t.Run("maps missing template", func(t *testing.T) {
		repo := NewGORMRepository(newEmailDryRunDB(t, withEmailDryRunQueryErrors(gorm.ErrRecordNotFound)))

		got, err := repo.GetTemplate(ctx, schemaName, tenantID, TemplateInvoiceSend)

		assert.Nil(t, got)
		assert.ErrorIs(t, err, ErrTemplateNotFound)
	})

	t.Run("returns get template query errors", func(t *testing.T) {
		expectedErr := errors.New("template query failed")
		repo := NewGORMRepository(newEmailDryRunDB(t, withEmailDryRunQueryErrors(expectedErr)))

		got, err := repo.GetTemplate(ctx, schemaName, tenantID, TemplateInvoiceSend)

		assert.Nil(t, got)
		assert.ErrorIs(t, err, expectedErr)
	})

	t.Run("returns list template query errors", func(t *testing.T) {
		expectedErr := errors.New("template list failed")
		repo := NewGORMRepository(newEmailDryRunDB(t, withEmailDryRunQueryErrors(expectedErr)))

		got, err := repo.ListTemplates(ctx, schemaName, tenantID)

		assert.Nil(t, got)
		assert.ErrorIs(t, err, expectedErr)
	})

	t.Run("returns upsert create errors", func(t *testing.T) {
		expectedErr := errors.New("template create failed")
		repo := NewGORMRepository(newEmailDryRunDB(t, withEmailDryRunCreateErrors(expectedErr)))
		template := persisted

		err := repo.UpsertTemplate(ctx, schemaName, &template)

		assert.ErrorIs(t, err, expectedErr)
	})

	t.Run("returns upsert refresh query errors", func(t *testing.T) {
		expectedErr := errors.New("template refresh failed")
		repo := NewGORMRepository(newEmailDryRunDB(t,
			withEmailDryRunCreateCapture(nil),
			withEmailDryRunQueryErrors(expectedErr),
		))
		template := persisted

		err := repo.UpsertTemplate(ctx, schemaName, &template)

		assert.ErrorIs(t, err, expectedErr)
	})
}

func TestGORMRepositoryDryRunEmailLog(t *testing.T) {
	ctx := context.Background()
	schemaName := "tenant_email"
	tenantID := "tenant-1"
	relatedID := "invoice-1"
	sentAt := time.Date(2026, time.June, 25, 10, 45, 0, 0, time.UTC)
	createdAt := time.Date(2026, time.June, 25, 10, 40, 0, 0, time.UTC)
	log := &EmailLog{
		ID:             "log-1",
		TenantID:       tenantID,
		EmailType:      "invoice",
		RecipientEmail: "customer@example.com",
		RecipientName:  "Customer",
		Subject:        "Invoice INV-001",
		Status:         StatusPending,
		RelatedID:      relatedID,
		CreatedAt:      createdAt,
	}
	record := *emailLogToRecord(log)
	record.Status = StatusSent
	record.SentAt = &sentAt

	t.Run("creates updates and lists logs", func(t *testing.T) {
		recorder := &emailDryRunRecorder{}
		repo := NewGORMRepository(newEmailDryRunDB(t,
			withEmailDryRunFixtures(emailDryRunFixtures{logs: []emailLogRecord{record}}, recorder),
			withEmailDryRunCreateCapture(recorder),
			withEmailDryRunUpdateRows(recorder, 1),
		))

		require.NoError(t, repo.CreateEmailLog(ctx, schemaName, log))
		require.NoError(t, repo.UpdateEmailLogStatus(ctx, schemaName, log.ID, StatusSent, &sentAt, ""))
		logs, err := repo.GetEmailLog(ctx, schemaName, tenantID, 0)
		require.NoError(t, err)
		require.Len(t, logs, 1)
		assert.Equal(t, relatedID, logs[0].RelatedID)
		assert.Equal(t, StatusSent, logs[0].Status)

		require.Len(t, recorder.createdLogs, 1)
		assert.Equal(t, log.ID, recorder.createdLogs[0].ID)
		assertEmailRecordedSQLContains(t, recorder.creates, `INSERT INTO "tenant_email"."email_log"`)
		assertEmailRecordedSQLContains(t, recorder.updates,
			`UPDATE "tenant_email"."email_log"`,
			`"status"`,
			`"sent_at"`,
			`"error_message"`,
		)
		assertEmailRecordedSQLContains(t, recorder.queries,
			`FROM "tenant_email"."email_log"`,
			`ORDER BY created_at DESC`,
			`LIMIT`,
		)
	})

	t.Run("returns create log errors", func(t *testing.T) {
		expectedErr := errors.New("log create failed")
		repo := NewGORMRepository(newEmailDryRunDB(t, withEmailDryRunCreateErrors(expectedErr)))

		err := repo.CreateEmailLog(ctx, schemaName, log)

		assert.ErrorIs(t, err, expectedErr)
	})

	t.Run("returns update log errors", func(t *testing.T) {
		expectedErr := errors.New("log update failed")
		repo := NewGORMRepository(newEmailDryRunDB(t, withEmailDryRunUpdateErrors(expectedErr)))

		err := repo.UpdateEmailLogStatus(ctx, schemaName, log.ID, StatusFailed, nil, "smtp failed")

		assert.ErrorIs(t, err, expectedErr)
	})

	t.Run("returns list log errors", func(t *testing.T) {
		expectedErr := errors.New("log query failed")
		repo := NewGORMRepository(newEmailDryRunDB(t, withEmailDryRunQueryErrors(expectedErr)))

		logs, err := repo.GetEmailLog(ctx, schemaName, tenantID, 10)

		assert.Nil(t, logs)
		assert.ErrorIs(t, err, expectedErr)
	})
}
