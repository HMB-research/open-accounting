package documents

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/HMB-research/open-accounting/internal/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type documentsDryRunConnPool struct{}

func (documentsDryRunConnPool) PrepareContext(context.Context, string) (*sql.Stmt, error) {
	return nil, errors.New("dry-run tests should not prepare statements")
}

func (documentsDryRunConnPool) ExecContext(context.Context, string, ...interface{}) (sql.Result, error) {
	return nil, errors.New("dry-run tests should not execute statements")
}

func (documentsDryRunConnPool) QueryContext(context.Context, string, ...interface{}) (*sql.Rows, error) {
	return nil, errors.New("dry-run tests should not query rows")
}

func (documentsDryRunConnPool) QueryRowContext(context.Context, string, ...interface{}) *sql.Row {
	return nil
}

func (documentsDryRunConnPool) BeginTx(context.Context, *sql.TxOptions) (gorm.ConnPool, error) {
	return &documentsDryRunTx{}, nil
}

type documentsDryRunTx struct {
	documentsDryRunConnPool
}

func (*documentsDryRunTx) Commit() error {
	return nil
}

func (*documentsDryRunTx) Rollback() error {
	return nil
}

type documentsDryRunDBOption func(t *testing.T, db *gorm.DB)

type documentsDryRunQueryFixtures struct {
	documents []models.Document
	document  *models.Document
	count     *int64
}

func newDocumentsDryRunDB(t *testing.T, opts ...documentsDryRunDBOption) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(postgres.New(postgres.Config{Conn: documentsDryRunConnPool{}}), &gorm.Config{
		DisableAutomaticPing:   true,
		DryRun:                 true,
		Logger:                 logger.Default.LogMode(logger.Silent),
		SkipDefaultTransaction: true,
	})
	if err != nil {
		t.Fatalf("open dry-run gorm db: %v", err)
	}

	for _, opt := range opts {
		opt(t, db)
	}
	return db
}

func withDocumentsDryRunQueryFixtures(fixtures documentsDryRunQueryFixtures) documentsDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		err := db.Callback().Query().After("gorm:query").Register(documentsDryRunCallbackName(t, "query_fixtures"), func(tx *gorm.DB) {
			switch dest := tx.Statement.Dest.(type) {
			case *int64:
				if fixtures.count != nil {
					*dest = *fixtures.count
					tx.RowsAffected = 1
				}
			case *[]models.Document:
				*dest = append([]models.Document(nil), fixtures.documents...)
				tx.RowsAffected = int64(len(fixtures.documents))
			case *models.Document:
				if fixtures.document != nil {
					*dest = *fixtures.document
					tx.RowsAffected = 1
				}
			}
		})
		if err != nil {
			t.Fatalf("register query fixture callback: %v", err)
		}
	}
}

func withDocumentsDryRunCreateError(expectedErr error) documentsDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		err := db.Callback().Create().Before("gorm:create").Register(documentsDryRunCallbackName(t, "create_error"), func(tx *gorm.DB) {
			tx.AddError(expectedErr)
		})
		if err != nil {
			t.Fatalf("register create error callback: %v", err)
		}
	}
}

func withDocumentsDryRunQueryError(expectedErr error) documentsDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		err := db.Callback().Query().Before("gorm:query").Register(documentsDryRunCallbackName(t, "query_error"), func(tx *gorm.DB) {
			tx.AddError(expectedErr)
		})
		if err != nil {
			t.Fatalf("register query error callback: %v", err)
		}
	}
}

func withDocumentsDryRunRowError(expectedErr error) documentsDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		err := db.Callback().Row().Before("gorm:row").Register(documentsDryRunCallbackName(t, "row_error"), func(tx *gorm.DB) {
			tx.AddError(expectedErr)
		})
		if err != nil {
			t.Fatalf("register row error callback: %v", err)
		}
	}
}

func withDocumentsDryRunUpdateRows(rows int64) documentsDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		err := db.Callback().Update().After("gorm:update").Register(documentsDryRunCallbackName(t, "update_rows"), func(tx *gorm.DB) {
			tx.RowsAffected = rows
		})
		if err != nil {
			t.Fatalf("register update rows callback: %v", err)
		}
	}
}

func withDocumentsDryRunUpdateError(expectedErr error) documentsDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		err := db.Callback().Update().Before("gorm:update").Register(documentsDryRunCallbackName(t, "update_error"), func(tx *gorm.DB) {
			tx.AddError(expectedErr)
		})
		if err != nil {
			t.Fatalf("register update error callback: %v", err)
		}
	}
}

func withDocumentsDryRunDeleteRows(rows int64) documentsDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		err := db.Callback().Delete().After("gorm:delete").Register(documentsDryRunCallbackName(t, "delete_rows"), func(tx *gorm.DB) {
			tx.RowsAffected = rows
		})
		if err != nil {
			t.Fatalf("register delete rows callback: %v", err)
		}
	}
}

func withDocumentsDryRunDeleteError(expectedErr error) documentsDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		err := db.Callback().Delete().Before("gorm:delete").Register(documentsDryRunCallbackName(t, "delete_error"), func(tx *gorm.DB) {
			tx.AddError(expectedErr)
		})
		if err != nil {
			t.Fatalf("register delete error callback: %v", err)
		}
	}
}

func documentsDryRunCallbackName(t *testing.T, suffix string) string {
	replacer := strings.NewReplacer("/", "_", " ", "_", ":", "_")
	return "documents_test:" + replacer.Replace(t.Name()) + ":" + suffix
}

func TestGORMRepositoryDryRunOperations(t *testing.T) {
	ctx := context.Background()
	schemaName := "tenant_schema"
	tenantID := "11111111-1111-1111-1111-111111111111"
	entityID := "22222222-2222-2222-2222-222222222222"
	documentID := "33333333-3333-3333-3333-333333333333"
	replacementID := "44444444-4444-4444-4444-444444444444"
	userID := "55555555-5555-5555-5555-555555555555"
	now := time.Date(2026, time.June, 25, 9, 30, 0, 0, time.UTC)
	retentionUntil := now.AddDate(1, 0, 0)
	count := int64(1)
	docModel := models.Document{
		ID:              documentID,
		TenantID:        tenantID,
		EntityType:      EntityTypeInvoice,
		EntityID:        entityID,
		DocumentType:    DocumentTypeSupportingDocument,
		FileName:        "invoice-evidence.pdf",
		ContentType:     "application/pdf",
		FileSize:        2048,
		StorageKey:      "documents/invoice-evidence.pdf",
		Notes:           "signed invoice",
		RetentionUntil:  &retentionUntil,
		ReviewStatus:    ReviewStatusPending,
		LifecycleStatus: LifecycleStatusActive,
		UploadedBy:      userID,
		CreatedAt:       now,
	}
	repo := NewGORMRepository(newDocumentsDryRunDB(t,
		withDocumentsDryRunQueryFixtures(documentsDryRunQueryFixtures{
			documents: []models.Document{docModel},
			document:  &docModel,
			count:     &count,
		}),
		withDocumentsDryRunUpdateRows(1),
		withDocumentsDryRunDeleteRows(1),
	))

	exists, err := repo.EntityExists(ctx, schemaName, tenantID, EntityTypeInvoice, entityID)
	if err != nil {
		t.Fatalf("EntityExists returned error: %v", err)
	}
	if !exists {
		t.Fatal("expected entity count fixture to report an existing entity")
	}

	if err := repo.CreateDocument(ctx, schemaName, modelToDocument(&docModel)); err != nil {
		t.Fatalf("CreateDocument returned error: %v", err)
	}

	docs, err := repo.ListDocuments(ctx, schemaName, tenantID, EntityTypeInvoice, entityID)
	if err != nil {
		t.Fatalf("ListDocuments returned error: %v", err)
	}
	if len(docs) != 1 || docs[0].ID != documentID {
		t.Fatalf("expected listed document fixture, got %#v", docs)
	}

	queue, err := repo.ListReviewQueueDocuments(ctx, schemaName, tenantID, ReviewQueueFilter{
		EntityType:   EntityTypeInvoice,
		DocumentType: DocumentTypeSupportingDocument,
		ReviewStatus: ReviewStatusPending,
		Limit:        25,
	})
	if err != nil {
		t.Fatalf("ListReviewQueueDocuments returned error: %v", err)
	}
	if len(queue) != 1 || queue[0].DocumentType != DocumentTypeSupportingDocument {
		t.Fatalf("expected review queue fixture, got %#v", queue)
	}

	retentionDocs, err := repo.ListRetentionReviewDocuments(ctx, schemaName, tenantID, now, true)
	if err != nil {
		t.Fatalf("ListRetentionReviewDocuments(includeMissing=true) returned error: %v", err)
	}
	if len(retentionDocs) != 1 || retentionDocs[0].RetentionUntil == nil {
		t.Fatalf("expected retention review fixture, got %#v", retentionDocs)
	}

	expiredDocs, err := repo.ListRetentionReviewDocuments(ctx, schemaName, tenantID, now, false)
	if err != nil {
		t.Fatalf("ListRetentionReviewDocuments(includeMissing=false) returned error: %v", err)
	}
	if len(expiredDocs) != 1 || expiredDocs[0].ID != documentID {
		t.Fatalf("expected expired review fixture, got %#v", expiredDocs)
	}

	doc, err := repo.GetDocumentByID(ctx, schemaName, tenantID, documentID)
	if err != nil {
		t.Fatalf("GetDocumentByID returned error: %v", err)
	}
	if doc.ID != documentID || doc.FileName != "invoice-evidence.pdf" {
		t.Fatalf("expected document fixture, got %#v", doc)
	}

	hasDependents, err := repo.DocumentHasSupersededDependents(ctx, schemaName, tenantID, replacementID)
	if err != nil {
		t.Fatalf("DocumentHasSupersededDependents returned error: %v", err)
	}
	if !hasDependents {
		t.Fatal("expected superseded dependent count fixture")
	}

	if err := repo.UpdateDocumentRetention(ctx, schemaName, tenantID, documentID, &retentionUntil); err != nil {
		t.Fatalf("UpdateDocumentRetention returned error: %v", err)
	}
	if err := repo.UpdateDocumentLifecycle(ctx, schemaName, tenantID, documentID, LifecycleStatusSuperseded, "replaced", userID, now, &replacementID); err != nil {
		t.Fatalf("UpdateDocumentLifecycle returned error: %v", err)
	}
	if err := repo.UpdateDocumentLegalHold(ctx, schemaName, tenantID, documentID, true, "audit hold", userID, now); err != nil {
		t.Fatalf("UpdateDocumentLegalHold returned error: %v", err)
	}
	if err := repo.ReviewDocument(ctx, schemaName, tenantID, documentID, ReviewStatusApproved, "ready", userID, now); err != nil {
		t.Fatalf("ReviewDocument returned error: %v", err)
	}
	if err := repo.DeleteDocument(ctx, schemaName, tenantID, documentID); err != nil {
		t.Fatalf("DeleteDocument returned error: %v", err)
	}
}

func TestGORMRepositoryDryRunErrors(t *testing.T) {
	ctx := context.Background()
	schemaName := "tenant_schema"
	tenantID := "11111111-1111-1111-1111-111111111111"
	entityID := "22222222-2222-2222-2222-222222222222"
	documentID := "33333333-3333-3333-3333-333333333333"
	userID := "55555555-5555-5555-5555-555555555555"
	now := time.Date(2026, time.June, 25, 10, 0, 0, 0, time.UTC)
	expectedErr := errors.New("dry-run failure")

	t.Run("EntityExists wraps query errors", func(t *testing.T) {
		repo := NewGORMRepository(newDocumentsDryRunDB(t, withDocumentsDryRunQueryError(expectedErr)))
		exists, err := repo.EntityExists(ctx, schemaName, tenantID, EntityTypeInvoice, entityID)
		assertWrappedError(t, err, expectedErr, "check entity exists")
		if exists {
			t.Fatal("expected failed entity lookup to report false")
		}
	})

	t.Run("CreateDocument wraps create errors", func(t *testing.T) {
		repo := NewGORMRepository(newDocumentsDryRunDB(t, withDocumentsDryRunCreateError(expectedErr)))
		err := repo.CreateDocument(ctx, schemaName, &Document{TenantID: tenantID})
		assertWrappedError(t, err, expectedErr, "create document")
	})

	t.Run("ListDocuments wraps query errors", func(t *testing.T) {
		repo := NewGORMRepository(newDocumentsDryRunDB(t, withDocumentsDryRunQueryError(expectedErr)))
		docs, err := repo.ListDocuments(ctx, schemaName, tenantID, EntityTypeInvoice, entityID)
		assertWrappedError(t, err, expectedErr, "list documents")
		if docs != nil {
			t.Fatalf("expected nil documents on error, got %#v", docs)
		}
	})

	t.Run("ListReviewQueueDocuments wraps query errors", func(t *testing.T) {
		repo := NewGORMRepository(newDocumentsDryRunDB(t, withDocumentsDryRunQueryError(expectedErr)))
		docs, err := repo.ListReviewQueueDocuments(ctx, schemaName, tenantID, ReviewQueueFilter{Limit: 10})
		assertWrappedError(t, err, expectedErr, "list document review queue")
		if docs != nil {
			t.Fatalf("expected nil documents on error, got %#v", docs)
		}
	})

	t.Run("ListRetentionReviewDocuments wraps query errors", func(t *testing.T) {
		repo := NewGORMRepository(newDocumentsDryRunDB(t, withDocumentsDryRunQueryError(expectedErr)))
		docs, err := repo.ListRetentionReviewDocuments(ctx, schemaName, tenantID, now, false)
		assertWrappedError(t, err, expectedErr, "list retention review documents")
		if docs != nil {
			t.Fatalf("expected nil documents on error, got %#v", docs)
		}
	})

	t.Run("ListReviewSummaries wraps scan errors", func(t *testing.T) {
		repo := NewGORMRepository(newDocumentsDryRunDB(t, withDocumentsDryRunRowError(expectedErr)))
		summaries, err := repo.ListReviewSummaries(ctx, schemaName, tenantID, EntityTypeInvoice, []string{entityID})
		assertWrappedError(t, err, expectedErr, "list review summaries")
		if summaries != nil {
			t.Fatalf("expected nil summaries on error, got %#v", summaries)
		}
	})

	t.Run("GetDocumentByID maps missing documents", func(t *testing.T) {
		repo := NewGORMRepository(newDocumentsDryRunDB(t, withDocumentsDryRunQueryError(gorm.ErrRecordNotFound)))
		doc, err := repo.GetDocumentByID(ctx, schemaName, tenantID, documentID)
		if err == nil || err.Error() != "document not found" {
			t.Fatalf("expected document not found error, got %v", err)
		}
		if doc != nil {
			t.Fatalf("expected nil document on not found, got %#v", doc)
		}
	})

	t.Run("GetDocumentByID wraps query errors", func(t *testing.T) {
		repo := NewGORMRepository(newDocumentsDryRunDB(t, withDocumentsDryRunQueryError(expectedErr)))
		doc, err := repo.GetDocumentByID(ctx, schemaName, tenantID, documentID)
		assertWrappedError(t, err, expectedErr, "get document")
		if doc != nil {
			t.Fatalf("expected nil document on error, got %#v", doc)
		}
	})

	t.Run("DocumentHasSupersededDependents wraps query errors", func(t *testing.T) {
		repo := NewGORMRepository(newDocumentsDryRunDB(t, withDocumentsDryRunQueryError(expectedErr)))
		hasDependents, err := repo.DocumentHasSupersededDependents(ctx, schemaName, tenantID, documentID)
		assertWrappedError(t, err, expectedErr, "check document supersession dependents")
		if hasDependents {
			t.Fatal("expected failed dependent lookup to report false")
		}
	})

	updateCases := []struct {
		name string
		run  func(repo *GORMRepository) error
		want string
	}{
		{
			name: "UpdateDocumentRetention",
			run: func(repo *GORMRepository) error {
				return repo.UpdateDocumentRetention(ctx, schemaName, tenantID, documentID, &now)
			},
			want: "update document retention",
		},
		{
			name: "UpdateDocumentLifecycle",
			run: func(repo *GORMRepository) error {
				return repo.UpdateDocumentLifecycle(ctx, schemaName, tenantID, documentID, LifecycleStatusArchived, "archived", userID, now, nil)
			},
			want: "update document lifecycle",
		},
		{
			name: "UpdateDocumentLegalHold",
			run: func(repo *GORMRepository) error {
				return repo.UpdateDocumentLegalHold(ctx, schemaName, tenantID, documentID, false, "", userID, now)
			},
			want: "update document legal hold",
		},
		{
			name: "ReviewDocument",
			run: func(repo *GORMRepository) error {
				return repo.ReviewDocument(ctx, schemaName, tenantID, documentID, ReviewStatusRejected, "missing signature", userID, now)
			},
			want: "review document",
		},
	}

	for _, tt := range updateCases {
		t.Run(tt.name+" wraps update errors", func(t *testing.T) {
			repo := NewGORMRepository(newDocumentsDryRunDB(t, withDocumentsDryRunUpdateError(expectedErr)))
			assertWrappedError(t, tt.run(repo), expectedErr, tt.want)
		})

		t.Run(tt.name+" returns not found when no rows update", func(t *testing.T) {
			repo := NewGORMRepository(newDocumentsDryRunDB(t, withDocumentsDryRunUpdateRows(0)))
			err := tt.run(repo)
			if err == nil || err.Error() != "document not found" {
				t.Fatalf("expected document not found error, got %v", err)
			}
		})
	}

	t.Run("DeleteDocument wraps delete errors", func(t *testing.T) {
		repo := NewGORMRepository(newDocumentsDryRunDB(t, withDocumentsDryRunDeleteError(expectedErr)))
		assertWrappedError(t, repo.DeleteDocument(ctx, schemaName, tenantID, documentID), expectedErr, "delete document")
	})

	t.Run("DeleteDocument returns not found when no rows delete", func(t *testing.T) {
		repo := NewGORMRepository(newDocumentsDryRunDB(t, withDocumentsDryRunDeleteRows(0)))
		err := repo.DeleteDocument(ctx, schemaName, tenantID, documentID)
		if err == nil || err.Error() != "document not found" {
			t.Fatalf("expected document not found error, got %v", err)
		}
	})
}

func assertWrappedError(t *testing.T, err, target error, message string) {
	t.Helper()

	if err == nil {
		t.Fatalf("expected error containing %q", message)
	}
	if !errors.Is(err, target) {
		t.Fatalf("expected error to wrap %v, got %v", target, err)
	}
	if !strings.Contains(err.Error(), message) {
		t.Fatalf("expected error to contain %q, got %q", message, err.Error())
	}
}
