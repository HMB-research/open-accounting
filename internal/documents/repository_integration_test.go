package documents

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/HMB-research/open-accounting/internal/database"
	"github.com/HMB-research/open-accounting/internal/testutil"
	"github.com/google/uuid"
)

func TestDocumentEntityConstraintAllowsWorkflowRecords(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)

	table, err := database.QualifiedTable(tenant.SchemaName, "documents")
	if err != nil {
		t.Fatalf("QualifiedTable failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	for _, entityType := range []string{EntityTypeExpense, EntityTypeQuote, EntityTypeOrder, EntityTypeLeaveRecord} {
		// #nosec G201 -- table is schema-qualified by database.QualifiedTable in test setup.
		_, err := pool.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s (
				id, tenant_id, entity_type, entity_id, document_type, file_name,
				content_type, file_size, storage_key, uploaded_by
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		`, table),
			uuid.New().String(),
			tenant.ID,
			entityType,
			uuid.New().String(),
			DocumentTypeContract,
			entityType+"-document.pdf",
			"application/pdf",
			int64(12),
			"test/"+entityType+"/"+uuid.New().String(),
			uuid.New().String(),
		)
		if err != nil {
			t.Fatalf("expected %s documents to satisfy entity type check: %v", entityType, err)
		}
	}
}
