package contacts

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type contactDryRunConnPool struct{}

func (contactDryRunConnPool) PrepareContext(context.Context, string) (*sql.Stmt, error) {
	return nil, errors.New("dry-run contacts tests should not prepare statements")
}

func (contactDryRunConnPool) ExecContext(context.Context, string, ...interface{}) (sql.Result, error) {
	return nil, errors.New("dry-run contacts tests should not execute statements")
}

func (contactDryRunConnPool) QueryContext(context.Context, string, ...interface{}) (*sql.Rows, error) {
	return nil, errors.New("dry-run contacts tests should not query rows")
}

func (contactDryRunConnPool) QueryRowContext(context.Context, string, ...interface{}) *sql.Row {
	return nil
}

func (contactDryRunConnPool) BeginTx(context.Context, *sql.TxOptions) (gorm.ConnPool, error) {
	return &contactDryRunTx{}, nil
}

type contactDryRunTx struct {
	contactDryRunConnPool
}

func (*contactDryRunTx) Commit() error {
	return nil
}

func (*contactDryRunTx) Rollback() error {
	return nil
}

type contactDryRunDBOption func(t *testing.T, db *gorm.DB)

type contactDryRunFixtures struct {
	contact  *Contact
	contacts []Contact
}

func newContactDryRunDB(t *testing.T, opts ...contactDryRunDBOption) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(postgres.New(postgres.Config{Conn: contactDryRunConnPool{}}), &gorm.Config{
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

func withContactDryRunFixtures(fixtures contactDryRunFixtures) contactDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		err := db.Callback().Query().After("gorm:query").Register(contactDryRunCallbackName(t, "query_fixtures"), func(tx *gorm.DB) {
			switch dest := tx.Statement.Dest.(type) {
			case *Contact:
				if fixtures.contact != nil {
					*dest = *fixtures.contact
					tx.RowsAffected = 1
				}
			case *[]Contact:
				if fixtures.contacts != nil {
					*dest = append([]Contact(nil), fixtures.contacts...)
					tx.RowsAffected = int64(len(fixtures.contacts))
				}
			}
		})
		require.NoError(t, err)
	}
}

func withContactDryRunCreateError(expectedErr error) contactDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		err := db.Callback().Create().Before("gorm:create").Register(contactDryRunCallbackName(t, "create_error"), func(tx *gorm.DB) {
			tx.AddError(expectedErr)
		})
		require.NoError(t, err)
	}
}

func withContactDryRunQueryError(expectedErr error) contactDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		err := db.Callback().Query().Before("gorm:query").Register(contactDryRunCallbackName(t, "query_error"), func(tx *gorm.DB) {
			tx.AddError(expectedErr)
		})
		require.NoError(t, err)
	}
}

func withContactDryRunUpdateRows(rows int64) contactDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		err := db.Callback().Update().After("gorm:update").Register(contactDryRunCallbackName(t, "update_rows"), func(tx *gorm.DB) {
			tx.RowsAffected = rows
		})
		require.NoError(t, err)
	}
}

func withContactDryRunUpdateError(expectedErr error) contactDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		err := db.Callback().Update().Before("gorm:update").Register(contactDryRunCallbackName(t, "update_error"), func(tx *gorm.DB) {
			tx.AddError(expectedErr)
		})
		require.NoError(t, err)
	}
}

func contactDryRunCallbackName(t *testing.T, suffix string) string {
	replacer := strings.NewReplacer("/", "_", " ", "_", ":", "_")
	return "contacts_test:" + replacer.Replace(t.Name()) + ":" + suffix
}

func TestGORMRepositoryDryRunOperations(t *testing.T) {
	ctx := context.Background()
	schemaName := "tenant_contacts"
	tenantID := "tenant-1"
	contact := contactDryRunContact(tenantID)
	repo := NewGORMRepository(newContactDryRunDB(t,
		withContactDryRunFixtures(contactDryRunFixtures{
			contact:  contact,
			contacts: []Contact{*contact},
		}),
		withContactDryRunUpdateRows(1),
	))

	require.NoError(t, repo.Create(ctx, schemaName, contact))

	got, err := repo.GetByID(ctx, schemaName, tenantID, contact.ID)
	require.NoError(t, err)
	assert.Equal(t, contact.ID, got.ID)

	contacts, err := repo.List(ctx, schemaName, tenantID, &ContactFilter{
		ContactType: ContactTypeCustomer,
		ActiveOnly:  true,
		Search:      "acme",
	})
	require.NoError(t, err)
	require.Len(t, contacts, 1)
	assert.Equal(t, contact.Name, contacts[0].Name)

	contact.Name = "Acme Updated"
	require.NoError(t, repo.Update(ctx, schemaName, contact))
	require.NoError(t, repo.Delete(ctx, schemaName, tenantID, contact.ID))
}

func TestGORMRepositoryDryRunErrors(t *testing.T) {
	ctx := context.Background()
	schemaName := "tenant_contacts"
	tenantID := "tenant-1"
	contact := contactDryRunContact(tenantID)

	t.Run("invalid schema", func(t *testing.T) {
		repo := NewGORMRepository(newContactDryRunDB(t))
		err := repo.Create(ctx, "tenant-contacts", contact)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid")
	})

	t.Run("create error", func(t *testing.T) {
		expectedErr := errors.New("create failed")
		repo := NewGORMRepository(newContactDryRunDB(t, withContactDryRunCreateError(expectedErr)))
		err := repo.Create(ctx, schemaName, contact)
		require.ErrorIs(t, err, expectedErr)
	})

	t.Run("get not found", func(t *testing.T) {
		repo := NewGORMRepository(newContactDryRunDB(t, withContactDryRunQueryError(gorm.ErrRecordNotFound)))
		got, err := repo.GetByID(ctx, schemaName, tenantID, contact.ID)
		require.ErrorIs(t, err, ErrContactNotFound)
		assert.Nil(t, got)
	})

	t.Run("get query error", func(t *testing.T) {
		expectedErr := errors.New("query failed")
		repo := NewGORMRepository(newContactDryRunDB(t, withContactDryRunQueryError(expectedErr)))
		got, err := repo.GetByID(ctx, schemaName, tenantID, contact.ID)
		require.ErrorIs(t, err, expectedErr)
		assert.Nil(t, got)
	})

	t.Run("list query error", func(t *testing.T) {
		expectedErr := errors.New("list failed")
		repo := NewGORMRepository(newContactDryRunDB(t, withContactDryRunQueryError(expectedErr)))
		contacts, err := repo.List(ctx, schemaName, tenantID, nil)
		require.ErrorIs(t, err, expectedErr)
		assert.Nil(t, contacts)
	})

	t.Run("update error", func(t *testing.T) {
		expectedErr := errors.New("update failed")
		repo := NewGORMRepository(newContactDryRunDB(t, withContactDryRunUpdateError(expectedErr)))
		err := repo.Update(ctx, schemaName, contact)
		require.ErrorIs(t, err, expectedErr)
	})

	t.Run("delete error", func(t *testing.T) {
		expectedErr := errors.New("delete failed")
		repo := NewGORMRepository(newContactDryRunDB(t, withContactDryRunUpdateError(expectedErr)))
		err := repo.Delete(ctx, schemaName, tenantID, contact.ID)
		require.ErrorIs(t, err, expectedErr)
	})

	t.Run("delete missing contact", func(t *testing.T) {
		repo := NewGORMRepository(newContactDryRunDB(t, withContactDryRunUpdateRows(0)))
		err := repo.Delete(ctx, schemaName, tenantID, contact.ID)
		require.ErrorIs(t, err, ErrContactNotFound)
	})
}

func contactDryRunContact(tenantID string) *Contact {
	now := time.Date(2026, time.June, 25, 12, 0, 0, 0, time.UTC)
	accountID := "0bc53113-67f7-44a3-ab4f-f49320005580"
	return &Contact{
		ID:               "contact-1",
		TenantID:         tenantID,
		Code:             "C-001",
		Name:             "Acme OU",
		ContactType:      ContactTypeCustomer,
		RegCode:          "12345678",
		VATNumber:        "EE123456789",
		Email:            "billing@example.com",
		Phone:            "+37255555555",
		AddressLine1:     "Main 1",
		City:             "Tallinn",
		PostalCode:       "10111",
		CountryCode:      "EE",
		PaymentTermsDays: 14,
		CreditLimit:      decimal.NewFromInt(1000),
		DefaultAccountID: &accountID,
		IsActive:         true,
		Notes:            "priority",
		CreatedAt:        now,
		UpdatedAt:        now,
	}
}
