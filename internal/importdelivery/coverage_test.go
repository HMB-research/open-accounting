package importdelivery

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCoverageClassifiesCurrentCanonicalDomainsWithoutSourcePayloads(t *testing.T) {
	records := []json.RawMessage{
		coverageRecordJSON(t, "general_ledger_journal", "general_ledger", "smartaccounts-brave-ui-v2/general_ledger_csv_v1", "authoritative_once", "upsert", "", "sa-key-v1-source"),
		coverageRecordJSON(t, "general_ledger_journal", "general.entries.get", "", "authoritative_once", "upsert", "", "sa-key-v1-source"),
		coverageRecordJSON(t, "customer", "clients", "smartaccounts-browser-master-detail-v1/clients_detail_v1", "non_posting_reference", "upsert", "", "sa-key-v1-source"),
		coverageRecordJSON(t, "item", "articles", "smartaccounts-browser-master-detail-v1/articles_detail_v1", "non_posting_reference", "upsert", "", "sa-key-v1-source"),
		coverageRecordJSON(t, "sales_document", "purchasesales.client_invoices.get", "smartaccounts-api-v1.7/client_invoice_v1", "non_posting_reference", "upsert", "", "sa-key-v1-source"),
		coverageRecordJSON(t, "warehouse_movement_reference", "inventory.warehouse_movements.get", "smartaccounts-api-v1.7/warehouse_movement_reference_v1", "non_posting_reference", "upsert", "", "sa-key-v1-source"),
		coverageRecordJSON(t, "worker_sensitive_reference", "payroll.workers.get", "smartaccounts-api-v1.7/worker_sensitive_reference_v1", "non_posting_reference", "upsert", "", "sa-key-v1-source"),
		coverageRecordJSON(t, "payout_type_reference", "payroll.settings.payout_types.get", "smartaccounts-api-v1.7/payout_type_reference_v1", "non_posting_reference", "upsert", "", "sa-key-v1-source"),
		coverageRecordJSON(t, "vat_percentage_reference", "settings.vat_pcs.get", "smartaccounts-api-v1.7/vat_percentage_reference_v1", "non_posting_reference", "upsert", "", "sa-key-v1-source"),
		coverageRecordJSON(t, "browser_capture_evidence", "journal_entries", "", "non_posting_reference", "upsert", "", "sa-key-v1-source"),
		coverageRecordJSON(t, "attachment", "", "", "non_posting_reference", "upsert", "", "sa-key-v1-source"),
		coverageRecordJSON(t, "future_private_contract", "future", "future/v1", "non_posting_reference", "upsert", "", "sa-key-v1-source"),
		coverageRecordJSON(t, "vendor", "purchasesales.vendors.get", "smartaccounts-api-v1.7/vendor_v1", "review_required", "tombstone", "", "sa-key-v1-source"),
		coverageRecordJSON(t, "customer", "purchasesales.clients.get", "smartaccounts-api-v1.7/customer_v1", "non_posting_reference", "upsert", "", "other-source"),
	}
	service, manifest := coverageTestService(t, records)

	report, err := service.Coverage(context.Background(), "tenant_schema", "tenant-a", manifest.PackageID)
	require.NoError(t, err)
	assert.True(t, report.IntegrityOK)
	assert.Equal(t, len(records), report.DeclaredRecordCount)
	assert.Equal(t, len(records), report.ObservedRecordCount)
	assert.Equal(t, 3, report.UnconsumedRecordCount, "unreviewed browser summary/attachment and unknown contracts must not claim archive coverage")
	assert.Equal(t, 3, report.ReviewRequiredRecordCount, "browser article, tombstone, and source mismatch must remain reviewed")
	assert.Equal(t, 2, coverageBucketCount(report, "general_ledger", CoverageGLApplyGated))
	assert.Equal(t, 1, coverageBucketCount(report, "reference_master", CoverageReferenceApplyGated))
	assert.Equal(t, 2, coverageBucketCount(report, "reference_master", CoverageReviewRequired))
	assert.Equal(t, 1, coverageBucketCount(report, "commercial", CoverageArchiveOnly))
	assert.Equal(t, 1, coverageBucketCount(report, "warehouse_inventory", CoverageArchiveOnly))
	assert.Equal(t, 2, coverageBucketCount(report, "payroll_sensitive", CoverageArchiveOnly))
	assert.Equal(t, 1, coverageBucketCount(report, "statutory_reporting", CoverageArchiveOnly))
	assert.Equal(t, 3, coverageBucketCount(report, "unrecognized_canonical_contract", CoverageUnconsumed))
	assert.Equal(t, 1, coverageBucketCount(report, "source_binding", CoverageReviewRequired))

	encoded, err := json.Marshal(report)
	require.NoError(t, err)
	assert.NotContains(t, string(encoded), "source-secret-payload")
	assert.NotContains(t, string(encoded), "other-source")
}

func TestCoverageFailsClosedForUnstagedOrMismatchedArchiveAndFlagsIntegrityMismatch(t *testing.T) {
	service, manifest := coverageTestService(t, []json.RawMessage{
		coverageRecordJSON(t, "account", "settings.accounts.get", "smartaccounts-api-v1.7/account_v1", "non_posting_reference", "upsert", "", "sa-key-v1-source"),
	})
	store := service.store.(*memoryStore)
	status := store.statuses[key("tenant-a", manifest.PackageID)]
	status.Status = StatusReceiving
	store.statuses[key("tenant-a", manifest.PackageID)] = status
	_, err := service.Coverage(context.Background(), "tenant_schema", "tenant-a", manifest.PackageID)
	assert.ErrorIs(t, err, ErrCoverageNotReady)

	status.Status = StatusStagedReview
	status.RecordCount = 2
	store.statuses[key("tenant-a", manifest.PackageID)] = status
	report, err := service.Coverage(context.Background(), "tenant_schema", "tenant-a", manifest.PackageID)
	require.NoError(t, err)
	assert.False(t, report.IntegrityOK)
	assert.Equal(t, 1, coverageBucketCount(report, "archive_record_count", CoverageReviewRequired))

	broken := store.manifests[key("tenant-a", manifest.PackageID)]
	broken.SourceCompanyID = "other-source"
	store.manifests[key("tenant-a", manifest.PackageID)] = broken
	_, err = service.Coverage(context.Background(), "tenant_schema", "tenant-a", manifest.PackageID)
	assert.ErrorIs(t, err, ErrCoverageNotReady)
}

func TestCoverageCatalogCoversEveryCurrentReferenceAndEvidenceContractFailClosed(t *testing.T) {
	for _, record := range []coverageRecord{
		{EntityType: "account", Resource: "settings.accounts.get", SourceSchema: "smartaccounts-api-v1.7/account_v1", GLPostingMode: "non_posting_reference", Operation: "upsert", SourceCompanyID: "source"},
		{EntityType: "customer", Resource: "purchasesales.clients.get", SourceSchema: "smartaccounts-api-v1.7/customer_v1", GLPostingMode: "non_posting_reference", Operation: "upsert", SourceCompanyID: "source"},
		{EntityType: "vendor", Resource: "purchasesales.vendors.get", SourceSchema: "smartaccounts-api-v1.7/vendor_v1", GLPostingMode: "non_posting_reference", Operation: "upsert", SourceCompanyID: "source"},
		{EntityType: "item", Resource: "purchasesales.articles.get", SourceSchema: "smartaccounts-api-v1.7/article_v1", GLPostingMode: "non_posting_reference", Operation: "upsert", SourceCompanyID: "source"},
		{EntityType: "customer", Resource: "clients", SourceSchema: "smartaccounts-browser-master-detail-v1/clients_detail_v1", GLPostingMode: "non_posting_reference", Operation: "upsert", SourceCompanyID: "source"},
		{EntityType: "vendor", Resource: "vendors", SourceSchema: "smartaccounts-browser-master-detail-v1/vendors_detail_v1", GLPostingMode: "non_posting_reference", Operation: "upsert", SourceCompanyID: "source"},
	} {
		_, disposition := classifyCoverageRecord(record, "source")
		assert.Equal(t, CoverageReferenceApplyGated, disposition, "%#v", record)
	}
	article := coverageRecord{EntityType: "item", Resource: "articles", SourceSchema: "smartaccounts-browser-master-detail-v1/articles_detail_v1", GLPostingMode: "non_posting_reference", Operation: "upsert", SourceCompanyID: "source"}
	_, disposition := classifyCoverageRecord(article, "source")
	assert.Equal(t, CoverageReviewRequired, disposition, "browser articles need an explicit VAT mapping")

	for contract := range archiveOnlyContracts {
		_, disposition := classifyCoverageRecord(coverageRecord{EntityType: contract.entityType, Resource: contract.resource, SourceSchema: contract.sourceSchema, GLPostingMode: "non_posting_reference", Operation: "upsert", SourceCompanyID: "source"}, "source")
		assert.Equal(t, CoverageArchiveOnly, disposition, "%#v", contract)
	}
	_, disposition = classifyCoverageRecord(coverageRecord{EntityType: "future_writer", GLPostingMode: "non_posting_reference", Operation: "upsert", SourceCompanyID: "source"}, "source")
	assert.Equal(t, CoverageUnconsumed, disposition)
}

func TestCoverageArchiveOnlyRequiresExactReviewedContract(t *testing.T) {
	known := coverageRecord{EntityType: "sales_document", Resource: "purchasesales.client_invoices.get", SourceSchema: "smartaccounts-api-v1.7/client_invoice_v1", GLPostingMode: "non_posting_reference", Operation: "upsert", SourceCompanyID: "source"}
	_, disposition := classifyCoverageRecord(known, "source")
	assert.Equal(t, CoverageArchiveOnly, disposition)

	for _, changed := range []coverageRecord{
		{EntityType: known.EntityType, Resource: known.Resource, SourceSchema: "smartaccounts-api-v1.8/client_invoice_v1", GLPostingMode: known.GLPostingMode, Operation: known.Operation, SourceCompanyID: known.SourceCompanyID},
		{EntityType: known.EntityType, Resource: "purchasesales.unreviewed_invoices.get", SourceSchema: known.SourceSchema, GLPostingMode: known.GLPostingMode, Operation: known.Operation, SourceCompanyID: known.SourceCompanyID},
		{EntityType: "attachment", Resource: "unreviewed_attachment", SourceSchema: "future/private-v1", GLPostingMode: "non_posting_reference", Operation: "upsert", SourceCompanyID: "source"},
		{EntityType: "report_snapshot_evidence", Resource: "income_statement", SourceSchema: "future-reviewed-evidence-schema", GLPostingMode: "non_posting_reference", Operation: "upsert", SourceCompanyID: "source"},
	} {
		_, got := classifyCoverageRecord(changed, "source")
		assert.Equal(t, CoverageUnconsumed, got, "%#v", changed)
	}

	// The private commercial bridge intentionally gives these an immutable
	// identity-review reason. It is retained in the protected package but must
	// never be reported as completed archive coverage.
	commercial := coverageRecord{EntityType: "browser_capture_evidence", Resource: "client_invoices", SourceSchema: "smartaccounts-browser-commercial-detail-v1/client_invoices_detail_v1", GLPostingMode: "non_posting_reference", Operation: "upsert", ReviewReason: "commercial_detail_identity_review_required", SourceCompanyID: "source"}
	_, disposition = classifyCoverageRecord(commercial, "source")
	assert.Equal(t, CoverageReviewRequired, disposition)
}

func coverageTestService(t *testing.T, records []json.RawMessage) (*Service, Manifest) {
	t.Helper()
	data := []byte(strings.Join(rawStrings(records), "\n") + "\n")
	manifest := testManifest(data, nil)
	manifest.RecordCount = len(records)
	store := &memoryStore{
		manifests: map[string]Manifest{key("tenant-a", manifest.PackageID): manifest},
		statuses: map[string]Status{key("tenant-a", manifest.PackageID): {
			PackageID: manifest.PackageID, TenantID: "tenant-a", SourceCompanyID: manifest.SourceCompanyID,
			Status: StatusStagedReview, ManifestSHA256: manifest.ManifestSHA256, PackageSHA256: manifest.PackageSHA256,
			RecordCount: len(records), ArtifactCount: len(manifest.Artifacts),
		}},
		records:   map[string][]StoredRecordChunk{key("tenant-a", manifest.PackageID): {{Sequence: 0, RecordCount: len(records), SHA256: sha256Hex(data), Data: data}}},
		artifacts: map[string]map[string][]StoredArtifactChunk{},
	}
	return NewService(store, &memoryBinder{}), manifest
}

func coverageRecordJSON(t *testing.T, entityType, resource, sourceSchema, postingMode, operation, reviewReason, sourceID string) json.RawMessage {
	t.Helper()
	record, err := json.Marshal(map[string]any{
		"entity_type":       entityType,
		"resource":          resource,
		"source_schema":     sourceSchema,
		"gl_posting_mode":   postingMode,
		"operation":         operation,
		"review_reason":     reviewReason,
		"source_company_id": sourceID,
		"payload":           map[string]string{"private": "source-secret-payload"},
	})
	require.NoError(t, err)
	return record
}

func rawStrings(values []json.RawMessage) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		result = append(result, string(value))
	}
	return result
}

func coverageBucketCount(report CoverageReport, domain, disposition string) int {
	for _, bucket := range report.Buckets {
		if bucket.Domain == domain && bucket.Disposition == disposition {
			return bucket.RecordCount
		}
	}
	return 0
}
