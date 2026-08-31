package smartaccountssync

import "sort"

// Full-claim dispositions describe the current contract boundary for a fixed
// SmartAccounts source surface. They deliberately are not import, staging,
// reconciliation, or accountant-approval outcomes.
const (
	FullClaimDispositionGLApplyGated        = "GL_APPLY_GATED"
	FullClaimDispositionReferenceApplyGated = "REFERENCE_APPLY_GATED"
	FullClaimDispositionArchiveOnly         = "ARCHIVE_ONLY"
	FullClaimDispositionFilterRequired      = "FILTER_CONTRACT_REQUIRED"
	FullClaimDispositionPageOnlyRequired    = "PAGE_ONLY_CONTRACT_REQUIRED"
	FullClaimDispositionReviewRequired      = "REVIEW_REQUIRED"
	FullClaimDispositionUnconsumed          = "UNCONSUMED"
	FullClaimDispositionMissingAPIEndpoint  = "MISSING_API_ENDPOINT"
	// FullClaimDispositionExportContractRequired is a deliberate source-path
	// obligation for a business domain the documented API does not fully cover.
	// It cannot be cleared by a generic archive or a nearby API resource: the
	// selected vendor export, reviewed browser read contract, or later signed
	// API replacement must be recorded explicitly in a new plan version.
	FullClaimDispositionExportContractRequired = "EXPORT_CONTRACT_REQUIRED"
	FullClaimDispositionResolved               = "RESOLVED"
)

const (
	fullClaimSourceBraveV2           = "brave_ui_v2"
	fullClaimSourceBraveMasterDetail = "brave_master_detail"
	fullClaimSourceBraveCommercial   = "brave_commercial_detail"
	fullClaimSourceAPIv17            = "api_v1_7"
	fullClaimSourceVendorExport      = "vendor_immutable_export"

	fullClaimAPIContractVersion        = "smartaccounts-api-v1.7"
	fullClaimCommercialContractVersion = "smartaccounts-browser-commercial-detail-v1"
	fullClaimVendorExportVersion       = "smartaccounts-vendor-export-coverage-v1"
)

// FullClaimCoverageRow contains fixed contract metadata only. It contains no
// tenant, company, source record, count, amount, credential, or endpoint URL.
// A non-empty BlockReason blocks this route in the legacy inventory assessment;
// in a selected domain plan it records the evidence that must be proven before
// the route can clear.
type FullClaimCoverageRow struct {
	Source          string `json:"source"`
	ResourceID      string `json:"resource_id"`
	ContractVersion string `json:"contract_version"`
	Disposition     string `json:"disposition"`
	BlockReason     string `json:"block_reason,omitempty"`
}

// FullClaimEligibility is a deterministic, data-minimized capability result.
// It must never be displayed as a reconciliation or accountant approval.
type FullClaimEligibility struct {
	FullClaimEligible bool     `json:"full_claim_eligible"`
	BlockingResources []string `json:"blocking_resources"`
}

// FullClaimCoveragePlanVersion is included in each immutable domain choice so
// future live-evidence storage cannot accidentally satisfy a changed source
// selection. Bump it when a reviewed source choice changes.
const FullClaimCoveragePlanVersion = "smartaccounts-full-claim-domain-plan-v2"

// FullClaimDomainPlanEntry chooses exactly one source route for one business
// domain. Alternatives remain in the immutable plan for audit, but they do
// not become duplicate full-claim requirements. Neither the plan nor its
// evidence contains a tenant, company, record, amount, credential, or URL.
type FullClaimDomainPlanEntry struct {
	PlanVersion  string                 `json:"plan_version"`
	DomainID     string                 `json:"domain_id"`
	Selected     FullClaimCoverageRow   `json:"selected"`
	Alternatives []FullClaimCoverageRow `json:"alternatives,omitempty"`
}

// FullClaimDomainEvidence is the future per-domain, per-source evidence
// binding. Every flag is intentionally required: a route cannot become full
// merely because code or a synthetic fixture exists. This pure model is not
// currently persisted or accepted from clients, so the live product continues
// to report NOT_ELIGIBLE until a later reviewed integration supplies durable
// evidence for every selected domain.
type FullClaimDomainEvidence struct {
	PlanVersion             string
	DomainID                string
	Source                  string
	ResourceID              string
	ContractVersion         string
	LiveSourceValidated     bool
	SchemaValidated         bool
	CompletenessValidated   bool
	ReconciliationValidated bool
	TombstonesResolved      bool
	AccountantAttested      bool
}

// CurrentFullClaimCoverageMatrix is the immutable route inventory. It keeps
// every API and Brave pathway distinct for audit; it is deliberately not the
// full-claim selection because several routes represent the same domain.
func CurrentFullClaimCoverageMatrix() []FullClaimCoverageRow {
	rows := make([]FullClaimCoverageRow, 0, len(browserDiscoveryProtocolResources)+55)
	for _, resource := range browserDiscoveryProtocolResources {
		row := FullClaimCoverageRow{
			Source:          fullClaimSourceBraveV2,
			ResourceID:      resource.ID,
			ContractVersion: BrowserDiscoveryManifestVersion,
			Disposition:     FullClaimDispositionFilterRequired,
			BlockReason:     "reviewed_visible_filter_and_schema_contract_required",
		}
		switch resource.ID {
		case BrowserGeneralLedgerResourceID:
			row.ContractVersion = BrowserGeneralLedgerCSVSourceSchema
			row.Disposition = FullClaimDispositionGLApplyGated
			row.BlockReason = "staged_package_apply_reconciliation_and_independent_attestation_required"
		case BrowserJournalEntriesSummaryResourceID:
			row.Disposition = FullClaimDispositionUnconsumed
			row.BlockReason = "summary_evidence_has_no_reviewed_archive_or_posting_adapter"
		default:
			if resource.Coverage == "page_only" {
				row.Disposition = FullClaimDispositionPageOnlyRequired
				row.BlockReason = "reviewed_read_only_page_contract_required"
			}
		}
		rows = append(rows, row)
	}

	rows = append(rows,
		FullClaimCoverageRow{fullClaimSourceBraveMasterDetail, BrowserMasterDetailClientsResource, BrowserMasterDetailClientsSchema, FullClaimDispositionReferenceApplyGated, "live_serial_detail_capture_complete_snapshot_and_iso2_projection_required"},
		FullClaimCoverageRow{fullClaimSourceBraveMasterDetail, BrowserMasterDetailVendorsResource, BrowserMasterDetailVendorsSchema, FullClaimDispositionReferenceApplyGated, "live_serial_detail_capture_complete_snapshot_and_iso2_projection_required"},
		FullClaimCoverageRow{fullClaimSourceBraveMasterDetail, BrowserMasterDetailArticlesResource, BrowserMasterDetailArticlesSchema, FullClaimDispositionReviewRequired, "reviewed_article_vat_mapping_required"},
		FullClaimCoverageRow{fullClaimSourceBraveCommercial, "client_invoices", fullClaimCommercialContractVersion + "/client_invoices_detail_v1", FullClaimDispositionReviewRequired, "visible_list_selector_pager_and_owner_delivery_contract_required"},
		FullClaimCoverageRow{fullClaimSourceBraveCommercial, "bank_payments", fullClaimCommercialContractVersion + "/bank_payments_detail_v1", FullClaimDispositionReviewRequired, "visible_list_selector_pager_and_owner_delivery_contract_required"},
	)

	for _, resourceID := range fullClaimAPIResourceIDs {
		rows = append(rows, currentFullClaimAPIRow(resourceID))
	}
	// These are not undocumented endpoints and not exclusions. They are the
	// explicit, selected source-path obligations for business capabilities that
	// the documented API does not cover completely. The default is an immutable
	// vendor export because no reviewed signed API or authenticated browser
	// export contract exists yet. Replacing one must bump the plan version and
	// bind fresh per-domain evidence; it cannot silently inherit an unrelated
	// API, UI grid, package, or archive result.
	rows = append(rows, currentFullClaimNonAPIObligations()...)
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Source != rows[j].Source {
			return rows[i].Source < rows[j].Source
		}
		return rows[i].ResourceID < rows[j].ResourceID
	})
	return rows
}

// CurrentFullClaimDomainPlan selects one primary route for every known
// business domain. The documented API is preferred when it represents
// the same domain; browser pathways are retained as alternatives and do not
// block a direct-API choice merely by being unreviewed. A source choice is
// still fail-closed: selected routes with missing endpoints, unconsumed
// schemas, missing evidence, incomplete snapshots, unresolved tombstones, or
// missing reconciliation/accountant attestation remain ineligible.
func CurrentFullClaimDomainPlan() []FullClaimDomainPlanEntry {
	grouped := make(map[string][]FullClaimCoverageRow)
	for _, row := range CurrentFullClaimCoverageMatrix() {
		domainID, ok := fullClaimDomainForRoute(row)
		if !ok {
			// The plan validator and its tests fail closed for an unclassified
			// inventory route. Do not silently drop it from coverage.
			domainID = "unclassified_" + row.Source + "_" + row.ResourceID
		}
		grouped[domainID] = append(grouped[domainID], row)
	}

	domainIDs := make([]string, 0, len(grouped))
	for domainID := range grouped {
		domainIDs = append(domainIDs, domainID)
	}
	sort.Strings(domainIDs)

	plan := make([]FullClaimDomainPlanEntry, 0, len(domainIDs))
	for _, domainID := range domainIDs {
		routes := append([]FullClaimCoverageRow(nil), grouped[domainID]...)
		sort.Slice(routes, func(i, j int) bool {
			left, right := fullClaimSourcePriority(routes[i].Source), fullClaimSourcePriority(routes[j].Source)
			if left != right {
				return left < right
			}
			if routes[i].Source != routes[j].Source {
				return routes[i].Source < routes[j].Source
			}
			return routes[i].ResourceID < routes[j].ResourceID
		})
		plan = append(plan, FullClaimDomainPlanEntry{
			PlanVersion:  FullClaimCoveragePlanVersion,
			DomainID:     domainID,
			Selected:     routes[0],
			Alternatives: append([]FullClaimCoverageRow(nil), routes[1:]...),
		})
	}
	return plan
}

func fullClaimSourcePriority(source string) int {
	switch source {
	case fullClaimSourceAPIv17:
		return 1
	case fullClaimSourceBraveMasterDetail:
		return 2
	case fullClaimSourceBraveCommercial:
		return 3
	case fullClaimSourceBraveV2:
		return 4
	case fullClaimSourceVendorExport:
		return 5
	default:
		return 99
	}
}

// fullClaimDomainForRoute maps every inventory route to one business domain.
// A helper/export route is intentionally its own domain when it is needed as
// evidence (for example an invoice PDF); this prevents a successful invoice
// list capture from silently satisfying document-retention coverage.
func fullClaimDomainForRoute(row FullClaimCoverageRow) (string, bool) {
	if row.Source == fullClaimSourceVendorExport {
		switch row.ResourceID {
		case "recurring_invoices_templates_reminders":
			return "recurring_invoices_templates_reminders", true
		case "purchase_order_receipt_lifecycle":
			return "purchase_order_receipt_lifecycle", true
		case "e_invoice_inbox_pending_documents":
			return "e_invoice_inbox_pending_documents", true
		case "bank_import_export_statement_matching":
			return "bank_import_export_statement_matching", true
		case "vat_tsd_declarations_filing_receipts":
			return "vat_tsd_declarations_filing_receipts", true
		case "inventory_count_valuation_multi_warehouse":
			return "inventory_count_valuation_multi_warehouse", true
		case "fixed_assets_depreciation_schedules":
			return "fixed_assets_depreciation_schedules", true
		case "salary_sheets_payroll_calculations":
			return "salary_sheets_payroll_calculations", true
		case "annual_and_other_report_outputs":
			return "annual_and_other_report_outputs", true
		case "company_financial_year_defaults_audit_metadata":
			return "company_financial_year_defaults_audit_metadata", true
		}
	}
	if row.Source == fullClaimSourceBraveMasterDetail {
		switch row.ResourceID {
		case BrowserMasterDetailClientsResource:
			return "clients", true
		case BrowserMasterDetailVendorsResource:
			return "vendors", true
		case BrowserMasterDetailArticlesResource:
			return "articles", true
		}
	}
	if row.Source == fullClaimSourceBraveCommercial {
		switch row.ResourceID {
		case "client_invoices":
			return "client_invoices", true
		case "bank_payments":
			return "payments", true
		}
	}
	if row.Source == fullClaimSourceBraveV2 {
		switch row.ResourceID {
		case "client_offers", "client_orders", "client_invoices", "clients", "vendor_invoices", "vendor_orders", "vendors", "articles", "account_turnover", "balance_sheet", "income_statement", "cash_flow_statement", "vat_returns", "tsd_returns", "warehouse_movements", "fixed_assets", "depreciations", "fixed_asset_depreciation_report", "workers", "salaries", "annual_report", "other_reports", "warehouse_inventory", "warehouse_movements_report", "warehouses", "worker_absences", "wage_reports":
			return row.ResourceID, true
		case "bank_payments", "cash_payments":
			return "payments", true
		case BrowserGeneralLedgerResourceID, BrowserJournalEntriesSummaryResourceID:
			return "general_ledger", true
		}
	}
	if row.Source != fullClaimSourceAPIv17 {
		return "", false
	}
	switch row.ResourceID {
	case "general.files.get":
		return "files", true
	case "general.files.get_details":
		return "file_details", true
	case "general.entries.get":
		return "general_ledger", true
	case "general.entries.get_pdf":
		return "general_ledger_pdf", true
	case "general.account_balances.get":
		return "account_balances", true
	case "settings.accounts.get":
		return "accounts", true
	case "settings.bank_accounts.get":
		return "bank_accounts", true
	case "settings.cash_accounts.get":
		return "cash_accounts", true
	case "settings.countries.get":
		return "countries", true
	case "settings.document_templates.get":
		return "document_templates", true
	case "settings.groups.get":
		return "groups", true
	case "settings.objects.get":
		return "objects", true
	case "settings.payment_methods.get":
		return "payment_methods", true
	case "settings.report_rows.get":
		return "report_rows", true
	case "settings.warehouses.get":
		return "warehouses", true
	case "settings.vat_pcs.get":
		return "vat_percentages", true
	case "purchasesales.articles.get":
		return "articles", true
	case "purchasesales.articles.get_warehouse_quantities":
		return "article_warehouse_quantities", true
	case "purchasesales.clients.get":
		return "clients", true
	case "purchasesales.clients.get_balance":
		return "client_balances", true
	case "purchasesales.client_invoices.get":
		return "client_invoices", true
	case "purchasesales.client_invoices.get_pdf":
		return "client_invoice_pdf", true
	case "purchasesales.client_invoices.get_xml":
		return "client_invoice_xml", true
	case "purchasesales.client_offers.get":
		return "client_offers", true
	case "purchasesales.client_offers.get_pdf":
		return "client_offer_pdf", true
	case "purchasesales.client_offers.get_statuses":
		return "client_offer_statuses", true
	case "purchasesales.client_orders.get":
		return "client_orders", true
	case "purchasesales.client_orders.get_pdf":
		return "client_order_pdf", true
	case "purchasesales.client_orders.get_statuses":
		return "client_order_statuses", true
	case "purchasesales.vendors.get":
		return "vendors", true
	case "purchasesales.vendors.get_balance":
		return "vendor_balances", true
	case "purchasesales.vendor_invoices.get":
		return "vendor_invoices", true
	case "purchasesales.vendor_invoices.get_pdf":
		return "vendor_invoice_pdf", true
	case "purchasesales.payments.get":
		return "payments", true
	case "payroll.settings.payout_types.get":
		return "payout_types", true
	case "payroll.settings.absence_types.get":
		return "absence_types", true
	case "payroll.workers.get":
		return "workers", true
	case "payroll.workers.get_vacation_report":
		return "vacation_report", true
	case "payroll.worker_absences.get":
		return "worker_absences", true
	case "inventory.warehouse_movements.get":
		return "warehouse_movements", true
	default:
		return "", false
	}
}

func currentFullClaimAPIRow(resourceID string) FullClaimCoverageRow {
	row := FullClaimCoverageRow{
		Source:          fullClaimSourceAPIv17,
		ResourceID:      resourceID,
		ContractVersion: fullClaimAPIContractVersion,
		Disposition:     FullClaimDispositionUnconsumed,
		BlockReason:     "live_api_access_signature_schema_and_retention_validation_required",
	}
	switch resourceID {
	case "general.entries.get":
		row.Disposition = FullClaimDispositionGLApplyGated
		row.BlockReason = "live_api_access_signature_schema_and_retention_validation_required"
	case "settings.accounts.get", "purchasesales.clients.get", "purchasesales.vendors.get", "purchasesales.articles.get":
		row.Disposition = FullClaimDispositionReferenceApplyGated
	case "purchasesales.client_invoices.get", "purchasesales.vendor_invoices.get", "purchasesales.client_offers.get", "purchasesales.client_orders.get", "purchasesales.payments.get",
		"settings.bank_accounts.get", "settings.cash_accounts.get", "settings.payment_methods.get", "settings.report_rows.get", "settings.vat_pcs.get",
		"payroll.settings.payout_types.get", "payroll.settings.absence_types.get", "payroll.workers.get", "payroll.workers.get_vacation_report", "payroll.worker_absences.get",
		"inventory.warehouse_movements.get":
		row.Disposition = FullClaimDispositionArchiveOnly
	case "general.account_balances.get", "settings.warehouses.get":
		row.Disposition = FullClaimDispositionMissingAPIEndpoint
		row.BlockReason = "documented_get_method_has_no_vendor_endpoint_url"
	}
	return row
}

// currentFullClaimNonAPIObligations is intentionally small, fixed metadata.
// It records no source route, selector, URL, record, company, credential, or
// payload. A concrete source may be added only by replacing the selected row
// with a vendor-confirmed signed API contract, independently reviewed
// authenticated browser/export contract, or immutable vendor export contract.
func currentFullClaimNonAPIObligations() []FullClaimCoverageRow {
	return []FullClaimCoverageRow{
		{fullClaimSourceVendorExport, "recurring_invoices_templates_reminders", fullClaimVendorExportVersion, FullClaimDispositionExportContractRequired, "immutable_recurring_workflow_export_and_cutover_rule_required"},
		{fullClaimSourceVendorExport, "purchase_order_receipt_lifecycle", fullClaimVendorExportVersion, FullClaimDispositionExportContractRequired, "immutable_purchase_order_receipt_lifecycle_export_and_target_adapter_required"},
		{fullClaimSourceVendorExport, "e_invoice_inbox_pending_documents", fullClaimVendorExportVersion, FullClaimDispositionExportContractRequired, "immutable_einvoice_queue_state_export_and_parent_closure_required"},
		{fullClaimSourceVendorExport, "bank_import_export_statement_matching", fullClaimVendorExportVersion, FullClaimDispositionExportContractRequired, "signed_bank_statement_matching_export_and_reconciliation_mapping_required"},
		{fullClaimSourceVendorExport, "vat_tsd_declarations_filing_receipts", fullClaimVendorExportVersion, FullClaimDispositionExportContractRequired, "immutable_period_versioned_declaration_filing_receipt_export_required"},
		{fullClaimSourceVendorExport, "inventory_count_valuation_multi_warehouse", fullClaimVendorExportVersion, FullClaimDispositionExportContractRequired, "timestamped_inventory_valuation_export_and_target_disposition_required"},
		{fullClaimSourceVendorExport, "fixed_assets_depreciation_schedules", fullClaimVendorExportVersion, FullClaimDispositionExportContractRequired, "immutable_asset_register_depreciation_schedule_export_and_target_adapter_required"},
		{fullClaimSourceVendorExport, "salary_sheets_payroll_calculations", fullClaimVendorExportVersion, FullClaimDispositionExportContractRequired, "redacted_payroll_run_export_and_accountant_cutover_rule_required"},
		{fullClaimSourceVendorExport, "annual_and_other_report_outputs", fullClaimVendorExportVersion, FullClaimDispositionExportContractRequired, "immutable_parameterized_report_export_and_gl_control_linkage_required"},
		{fullClaimSourceVendorExport, "company_financial_year_defaults_audit_metadata", fullClaimVendorExportVersion, FullClaimDispositionExportContractRequired, "allowlisted_nonsecret_company_metadata_export_and_cutover_evidence_required"},
	}
}

// AssessFullClaimEligibility is intentionally pure. It is used to prevent a
// capability catalog or package with unresolved coverage from being labelled a
// full claim. Actual GL replay, current evidence, actor separation, and
// accountant attestation remain separate reconciliation requirements.
func AssessFullClaimEligibility(rows []FullClaimCoverageRow, unresolvedTombstones int) FullClaimEligibility {
	blocking := make([]string, 0)
	for _, row := range rows {
		if !validFullClaimCoverageRow(row) || fullClaimDispositionBlocks(row.Disposition) || row.BlockReason != "" {
			blocking = append(blocking, row.Source+":"+row.ResourceID)
		}
	}
	if unresolvedTombstones != 0 {
		blocking = append(blocking, "package:unresolved_tombstones")
	}
	sort.Strings(blocking)
	return FullClaimEligibility{FullClaimEligible: len(blocking) == 0, BlockingResources: blocking}
}

// AssessFullClaimDomainPlanEligibility evaluates the immutable selected route
// for each domain. It deliberately ignores an alternative route's state: an
// API-primary domain must not be blocked by an unreviewed browser fallback.
// Conversely, an unproven selected route is always blocking. This function is
// pure so the future persistence layer must bind evidence to this exact plan,
// rather than accepting a caller assertion that a route was complete.
func AssessFullClaimDomainPlanEligibility(plan []FullClaimDomainPlanEntry, evidence []FullClaimDomainEvidence, unresolvedTombstones int) FullClaimEligibility {
	blocking := make([]string, 0, len(plan)+3)
	if len(plan) == 0 {
		blocking = append(blocking, "plan:empty")
	}

	evidenceByDomain := make(map[string]FullClaimDomainEvidence, len(evidence))
	invalidEvidence := false
	for _, item := range evidence {
		if !validFullClaimDomainEvidence(item) {
			invalidEvidence = true
			continue
		}
		if _, exists := evidenceByDomain[item.DomainID]; exists {
			invalidEvidence = true
			continue
		}
		evidenceByDomain[item.DomainID] = item
	}

	seenDomains := make(map[string]struct{}, len(plan))
	for _, entry := range plan {
		if !validFullClaimDomainPlanEntry(entry) {
			blocking = append(blocking, "plan:invalid_domain")
			continue
		}
		if _, exists := seenDomains[entry.DomainID]; exists {
			blocking = append(blocking, "plan:duplicate_domain")
			continue
		}
		seenDomains[entry.DomainID] = struct{}{}

		// Filter/page-only/unconsumed/missing-endpoint routes remain hard
		// product gaps even if a caller attempts to supply evidence for them.
		if fullClaimDispositionBlocks(entry.Selected.Disposition) {
			blocking = append(blocking, "domain:"+entry.DomainID)
			continue
		}
		item, found := evidenceByDomain[entry.DomainID]
		if !found || !matchesFullClaimSelection(entry, item) || !fullClaimDomainEvidenceComplete(item) {
			blocking = append(blocking, "domain:"+entry.DomainID)
		}
	}
	for domainID := range evidenceByDomain {
		if _, found := seenDomains[domainID]; !found {
			invalidEvidence = true
		}
	}
	if invalidEvidence {
		blocking = append(blocking, "plan:invalid_evidence")
	}
	if unresolvedTombstones != 0 {
		blocking = append(blocking, "package:unresolved_tombstones")
	}
	sort.Strings(blocking)
	blocking = compactFullClaimBlockers(blocking)
	return FullClaimEligibility{FullClaimEligible: len(blocking) == 0, BlockingResources: blocking}
}

func validFullClaimDomainPlanEntry(entry FullClaimDomainPlanEntry) bool {
	if entry.PlanVersion != FullClaimCoveragePlanVersion || entry.DomainID == "" || !validFullClaimCoverageRow(entry.Selected) {
		return false
	}
	seen := map[string]struct{}{fullClaimRouteKey(entry.Selected): {}}
	for _, alternative := range entry.Alternatives {
		if !validFullClaimCoverageRow(alternative) {
			return false
		}
		key := fullClaimRouteKey(alternative)
		if _, duplicate := seen[key]; duplicate {
			return false
		}
		seen[key] = struct{}{}
	}
	return true
}

func validFullClaimDomainEvidence(item FullClaimDomainEvidence) bool {
	return item.PlanVersion == FullClaimCoveragePlanVersion && item.DomainID != "" && item.Source != "" && item.ResourceID != "" && item.ContractVersion != ""
}

func matchesFullClaimSelection(entry FullClaimDomainPlanEntry, item FullClaimDomainEvidence) bool {
	return item.PlanVersion == entry.PlanVersion && item.DomainID == entry.DomainID && item.Source == entry.Selected.Source && item.ResourceID == entry.Selected.ResourceID && item.ContractVersion == entry.Selected.ContractVersion
}

func fullClaimDomainEvidenceComplete(item FullClaimDomainEvidence) bool {
	return item.LiveSourceValidated && item.SchemaValidated && item.CompletenessValidated && item.ReconciliationValidated && item.TombstonesResolved && item.AccountantAttested
}

func fullClaimRouteKey(row FullClaimCoverageRow) string {
	return row.Source + "\x00" + row.ResourceID + "\x00" + row.ContractVersion
}

func compactFullClaimBlockers(values []string) []string {
	if len(values) < 2 {
		return values
	}
	result := values[:1]
	for _, value := range values[1:] {
		if value != result[len(result)-1] {
			result = append(result, value)
		}
	}
	return result
}

func validFullClaimCoverageRow(row FullClaimCoverageRow) bool {
	return row.Source != "" && row.ResourceID != "" && row.ContractVersion != "" && row.Disposition != ""
}

func fullClaimDispositionBlocks(disposition string) bool {
	switch disposition {
	case FullClaimDispositionGLApplyGated, FullClaimDispositionReferenceApplyGated, FullClaimDispositionArchiveOnly, FullClaimDispositionResolved:
		return false
	case FullClaimDispositionFilterRequired, FullClaimDispositionPageOnlyRequired, FullClaimDispositionReviewRequired, FullClaimDispositionUnconsumed, FullClaimDispositionMissingAPIEndpoint, FullClaimDispositionExportContractRequired:
		return true
	default:
		return true
	}
}

// fullClaimAPIResourceIDs mirrors every documented API v1.7 GET resource. A
// new API surface must enter this matrix explicitly before it can participate
// in a full-claim decision.
var fullClaimAPIResourceIDs = [...]string{
	"general.account_balances.get",
	"general.entries.get",
	"general.entries.get_pdf",
	"general.files.get",
	"general.files.get_details",
	"inventory.warehouse_movements.get",
	"payroll.settings.absence_types.get",
	"payroll.settings.payout_types.get",
	"payroll.worker_absences.get",
	"payroll.workers.get",
	"payroll.workers.get_vacation_report",
	"purchasesales.articles.get",
	"purchasesales.articles.get_warehouse_quantities",
	"purchasesales.client_invoices.get",
	"purchasesales.client_invoices.get_pdf",
	"purchasesales.client_invoices.get_xml",
	"purchasesales.client_offers.get",
	"purchasesales.client_offers.get_pdf",
	"purchasesales.client_offers.get_statuses",
	"purchasesales.client_orders.get",
	"purchasesales.client_orders.get_pdf",
	"purchasesales.client_orders.get_statuses",
	"purchasesales.clients.get",
	"purchasesales.clients.get_balance",
	"purchasesales.payments.get",
	"purchasesales.vendor_invoices.get",
	"purchasesales.vendor_invoices.get_pdf",
	"purchasesales.vendors.get",
	"purchasesales.vendors.get_balance",
	"settings.accounts.get",
	"settings.bank_accounts.get",
	"settings.cash_accounts.get",
	"settings.countries.get",
	"settings.document_templates.get",
	"settings.groups.get",
	"settings.objects.get",
	"settings.payment_methods.get",
	"settings.report_rows.get",
	"settings.vat_pcs.get",
	"settings.warehouses.get",
}
