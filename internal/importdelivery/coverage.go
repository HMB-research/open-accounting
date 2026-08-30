package importdelivery

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strings"
)

// Coverage dispositions describe what the OA target can safely do with a
// staged canonical record. They are intentionally not import outcomes: a
// gated record still needs its own reviewed preview and explicit confirmation.
const (
	CoverageGLApplyGated        = "GL_APPLY_GATED"
	CoverageReferenceApplyGated = "REFERENCE_APPLY_GATED"
	CoverageArchiveOnly         = "ARCHIVE_ONLY"
	CoverageReviewRequired      = "REVIEW_REQUIRED"
	CoverageUnconsumed          = "UNCONSUMED"
)

var ErrCoverageNotReady = errors.New("bridge package coverage requires a staged SmartAccounts package")

// CoverageBucket is a count-only disposition for one fixed target domain. It
// deliberately includes no source record, identifier, name, row, amount, or
// attachment data.
type CoverageBucket struct {
	Domain      string `json:"domain"`
	Disposition string `json:"disposition"`
	RecordCount int    `json:"record_count"`
}

// CoverageReport is a tenant-scoped reconciliation seam over an immutable
// staged package. Archive-only means the protected package is retained, not
// that an OA financial or operational object was created.
type CoverageReport struct {
	PackageID                 string           `json:"package_id"`
	PackageSHA256             string           `json:"package_sha256"`
	ManifestSHA256            string           `json:"manifest_sha256"`
	ScopeMode                 string           `json:"scope_mode"`
	DeclaredRecordCount       int              `json:"declared_record_count"`
	ObservedRecordCount       int              `json:"observed_record_count"`
	ArtifactCount             int              `json:"artifact_count"`
	IntegrityOK               bool             `json:"integrity_ok"`
	UnconsumedRecordCount     int              `json:"unconsumed_record_count"`
	ReviewRequiredRecordCount int              `json:"review_required_record_count"`
	Buckets                   []CoverageBucket `json:"buckets"`
}

// Coverage classifies current, documented SmartAccounts canonical contracts
// without reading payload fields into OA domain models. It is strictly
// read-only and has no dependency on financial, invoice, payment, payroll,
// inventory, document, or attachment writers.
func (s *Service) Coverage(ctx context.Context, schemaName, tenantID, packageID string) (CoverageReport, error) {
	if s == nil || s.store == nil || strings.TrimSpace(schemaName) == "" || strings.TrimSpace(tenantID) == "" || !safeID(packageID) {
		return CoverageReport{}, ErrCoverageNotReady
	}
	status, err := s.Status(ctx, schemaName, tenantID, packageID)
	if err != nil || status.Status != StatusStagedReview || status.TenantID != strings.TrimSpace(tenantID) {
		return CoverageReport{}, ErrCoverageNotReady
	}
	manifest, err := s.GetManifest(ctx, schemaName, tenantID, packageID)
	if err != nil || manifest.Provider != ProviderSmartAccounts || manifest.PackageID != status.PackageID || manifest.SourceCompanyID != status.SourceCompanyID || manifest.ManifestSHA256 != status.ManifestSHA256 || manifest.PackageSHA256 != status.PackageSHA256 {
		return CoverageReport{}, ErrCoverageNotReady
	}
	report := CoverageReport{
		PackageID:           status.PackageID,
		PackageSHA256:       status.PackageSHA256,
		ManifestSHA256:      status.ManifestSHA256,
		ScopeMode:           manifest.Scope.Mode,
		DeclaredRecordCount: status.RecordCount,
		ArtifactCount:       status.ArtifactCount,
		IntegrityOK:         true,
	}
	buckets := map[string]*CoverageBucket{}
	add := func(domain, disposition string) {
		key := domain + "\x00" + disposition
		bucket := buckets[key]
		if bucket == nil {
			bucket = &CoverageBucket{Domain: domain, Disposition: disposition}
			buckets[key] = bucket
		}
		bucket.RecordCount++
		if disposition == CoverageUnconsumed {
			report.UnconsumedRecordCount++
		}
		if disposition == CoverageReviewRequired {
			report.ReviewRequiredRecordCount++
		}
	}
	err = s.IterateRecords(ctx, schemaName, tenantID, packageID, func(raw json.RawMessage) error {
		report.ObservedRecordCount++
		var record coverageRecord
		if err := json.Unmarshal(raw, &record); err != nil {
			add("malformed_canonical_record", CoverageUnconsumed)
			return nil
		}
		domain, disposition := classifyCoverageRecord(record, manifest.SourceCompanyID)
		add(domain, disposition)
		return nil
	})
	if err != nil {
		return CoverageReport{}, err
	}
	if report.ObservedRecordCount != report.DeclaredRecordCount {
		report.IntegrityOK = false
		add("archive_record_count", CoverageReviewRequired)
	}
	report.Buckets = make([]CoverageBucket, 0, len(buckets))
	for _, bucket := range buckets {
		report.Buckets = append(report.Buckets, *bucket)
	}
	sort.Slice(report.Buckets, func(i, j int) bool {
		if report.Buckets[i].Domain != report.Buckets[j].Domain {
			return report.Buckets[i].Domain < report.Buckets[j].Domain
		}
		return report.Buckets[i].Disposition < report.Buckets[j].Disposition
	})
	return report, nil
}

type coverageRecord struct {
	EntityType      string `json:"entity_type"`
	SourceSchema    string `json:"source_schema"`
	SourceCompanyID string `json:"source_company_id"`
	Resource        string `json:"resource"`
	Operation       string `json:"operation"`
	GLPostingMode   string `json:"gl_posting_mode"`
	ReviewReason    string `json:"review_reason"`
}

func classifyCoverageRecord(record coverageRecord, expectedSource string) (string, string) {
	if strings.TrimSpace(record.SourceCompanyID) != expectedSource {
		return "source_binding", CoverageReviewRequired
	}
	if record.Operation == "tombstone" {
		return coverageDomain(record), CoverageReviewRequired
	}
	if record.Operation != "upsert" || record.GLPostingMode == "review_required" || strings.TrimSpace(record.ReviewReason) != "" {
		return coverageDomain(record), CoverageReviewRequired
	}
	if record.EntityType == "general_ledger_journal" {
		if (record.Resource == "general_ledger" && record.SourceSchema == "smartaccounts-brave-ui-v2/general_ledger_csv_v1" ||
			record.Resource == "general.entries.get" && record.SourceSchema == "") && record.GLPostingMode == "authoritative_once" {
			return "general_ledger", CoverageGLApplyGated
		}
		return "general_ledger", CoverageReviewRequired
	}
	if isReferenceMasterRecord(record) {
		if record.EntityType == "item" && record.SourceSchema == "smartaccounts-browser-master-detail-v1/articles_detail_v1" {
			// Browser article data has no reviewed VAT mapping. Do not let its
			// otherwise valid-looking item shape claim a target apply path.
			return "reference_master", CoverageReviewRequired
		}
		return "reference_master", CoverageReferenceApplyGated
	}
	if isArchiveOnlyRecord(record) {
		return coverageDomain(record), CoverageArchiveOnly
	}
	return "unrecognized_canonical_contract", CoverageUnconsumed
}

func isReferenceMasterRecord(record coverageRecord) bool {
	switch record.EntityType {
	case "account":
		return record.Resource == "settings.accounts.get" && record.SourceSchema == "smartaccounts-api-v1.7/account_v1" && record.GLPostingMode == "non_posting_reference"
	case "customer":
		return (record.Resource == "purchasesales.clients.get" && record.SourceSchema == "smartaccounts-api-v1.7/customer_v1" ||
			record.Resource == "clients" && record.SourceSchema == "smartaccounts-browser-master-detail-v1/clients_detail_v1") && record.GLPostingMode == "non_posting_reference"
	case "vendor":
		return (record.Resource == "purchasesales.vendors.get" && record.SourceSchema == "smartaccounts-api-v1.7/vendor_v1" ||
			record.Resource == "vendors" && record.SourceSchema == "smartaccounts-browser-master-detail-v1/vendors_detail_v1") && record.GLPostingMode == "non_posting_reference"
	case "item":
		return (record.Resource == "purchasesales.articles.get" && record.SourceSchema == "smartaccounts-api-v1.7/article_v1" ||
			record.Resource == "articles" && record.SourceSchema == "smartaccounts-browser-master-detail-v1/articles_detail_v1") && record.GLPostingMode == "non_posting_reference"
	}
	return false
}

func isArchiveOnlyRecord(record coverageRecord) bool {
	if record.GLPostingMode != "non_posting_reference" {
		return false
	}
	// Archive retention is also a reviewed consumer boundary.  In particular,
	// a familiar entity name (for example an attachment, report snapshot, or
	// payroll evidence) cannot turn a new source schema into completed archive
	// coverage.  Declared manifest artifacts are retained independently of this
	// record classifier; a canonical record must match its exact reviewed
	// entity/resource/source-schema contract below.
	return archiveOnlyContracts[coverageContract{record.EntityType, record.Resource, record.SourceSchema}]
}

type coverageContract struct{ entityType, resource, sourceSchema string }

var archiveOnlyContracts = func() map[coverageContract]bool {
	contracts := map[coverageContract]bool{}
	add := func(entityType, resource, sourceSchema string) {
		contracts[coverageContract{entityType, resource, sourceSchema}] = true
	}
	// API commercial documents and payments are archived evidence only. They
	// must never create a second journal, invoice, or payment posting path.
	add("sales_document", "purchasesales.client_invoices.get", "smartaccounts-api-v1.7/client_invoice_v1")
	add("purchase_document", "purchasesales.vendor_invoices.get", "smartaccounts-api-v1.7/vendor_invoice_v1")
	add("sales_offer", "purchasesales.client_offers.get", "smartaccounts-api-v1.7/client_offer_v1")
	add("sales_order", "purchasesales.client_orders.get", "smartaccounts-api-v1.7/client_order_v1")
	add("payment_reference", "purchasesales.payments.get", "smartaccounts-api-v1.7/payment_reference_v1")
	// API wave two: operational configuration, payroll, statutory, and
	// warehouse records are retained for evidence/reconciliation only.
	add("bank_account_reference", "settings.bank_accounts.get", "smartaccounts-api-v1.7/bank_account_reference_v1")
	add("cash_account_reference", "settings.cash_accounts.get", "smartaccounts-api-v1.7/cash_account_reference_v1")
	add("payment_method_reference", "settings.payment_methods.get", "smartaccounts-api-v1.7/payment_method_reference_v1")
	add("payout_type_reference", "payroll.settings.payout_types.get", "smartaccounts-api-v1.7/payout_type_reference_v1")
	add("absence_type_reference", "payroll.settings.absence_types.get", "smartaccounts-api-v1.7/absence_type_reference_v1")
	add("worker_sensitive_reference", "payroll.workers.get", "smartaccounts-api-v1.7/worker_sensitive_reference_v1")
	add("vacation_balance_evidence", "payroll.workers.get_vacation_report", "smartaccounts-api-v1.7/vacation_balance_evidence_v1")
	add("worker_absence_reference", "payroll.worker_absences.get", "smartaccounts-api-v1.7/worker_absence_reference_v1")
	add("warehouse_movement_reference", "inventory.warehouse_movements.get", "smartaccounts-api-v1.7/warehouse_movement_reference_v1")
	add("vat_percentage_reference", "settings.vat_pcs.get", "smartaccounts-api-v1.7/vat_percentage_reference_v1")
	add("report_row_definition", "settings.report_rows.get", "smartaccounts-api-v1.7/report_row_definition_v1")
	// Browser commercial detail has an immutable review reason in normal
	// operation and is caught above. Keep the exact reviewed contracts here as
	// archive-only if a future bridge removes that reason only after a new OA
	// consumer has been deliberately added.
	add("browser_capture_evidence", "client_invoices", "smartaccounts-browser-commercial-detail-v1/client_invoices_detail_v1")
	add("browser_capture_evidence", "bank_payments", "smartaccounts-browser-commercial-detail-v1/bank_payments_detail_v1")
	return contracts
}()

func coverageDomain(record coverageRecord) string {
	switch record.EntityType {
	case "general_ledger_journal":
		return "general_ledger"
	case "account", "customer", "vendor", "item":
		return "reference_master"
	case "sales_document", "purchase_document", "sales_offer", "sales_order", "browser_capture_evidence":
		return "commercial"
	case "payment_reference", "bank_account_reference", "cash_account_reference", "payment_method_reference":
		return "payments_and_cash"
	case "payout_type_reference", "absence_type_reference", "worker_sensitive_reference", "vacation_balance_evidence", "worker_absence_reference", "payroll_evidence", "payroll_report_evidence":
		return "payroll_sensitive"
	case "warehouse_movement_reference", "warehouse_reference", "inventory_snapshot_evidence", "warehouse_movement_evidence", "inventory_report_evidence":
		return "warehouse_inventory"
	case "vat_percentage_reference", "report_row_definition", "statutory_return_evidence", "report_snapshot_evidence", "annual_report_evidence":
		return "statutory_reporting"
	case "file_evidence", "comment_evidence", "attachment":
		return "files_and_comments"
	case "fixed_asset_evidence", "depreciation_evidence", "depreciation_report_evidence":
		return "fixed_assets"
	case "journal_summary_evidence":
		return "journal_summary"
	default:
		return "unrecognized_canonical_contract"
	}
}
