package docs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStatusDocumentationTracksCurrentGates(t *testing.T) {
	read := func(path string) string {
		t.Helper()
		payload, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		return string(payload)
	}

	activeDocs := map[string]string{
		"README.md":                                           read(filepath.Join("..", "README.md")),
		"docs/DEVELOPMENT_STATUS.md":                          read("DEVELOPMENT_STATUS.md"),
		"docs/ARCHITECTURE.md":                                read("ARCHITECTURE.md"),
		"docs/demo-e2e-testing.md":                            read("demo-e2e-testing.md"),
		"docs/FEATURE_MAPPING_MERIT_SMARTACCOUNTS.md":         read("FEATURE_MAPPING_MERIT_SMARTACCOUNTS.md"),
		".agents/skills/open-accounting-development/SKILL.md": read(filepath.Join("..", ".agents", "skills", "open-accounting-development", "SKILL.md")),
	}

	required := map[string][]string{
		"README.md": {
			"Full local baseline last verified on 2026-06-08",
			"On 2026-06-12, local revalidation covered",
			"On 2026-06-13, the payroll-history-import branch was revalidated",
			"`make test-backend-coverage`",
			"`make test-cli-coverage`",
			"`go test -timeout=3m ./docs -count=1`",
			"`go test -tags=integration ./internal/payroll -count=1`",
			"focused quote/order delivery demo E2E with 3 passed",
			"inventory subledger reconciliation stages",
			"PR #62 CI run `27475857115` at commit `a7a0068`",
			"frontend inventory subledger drill-down stage",
			"close remediation actions stage",
			"migration remediation actions stage",
			"migration remediation assignment stage",
			"migration remediation priority fallback stage",
			"cross-workspace remediation assignment metadata stage",
			"accountant workspace assignment queue stage",
			"expense assignment queue stage",
			"expense assignment completion stage",
			"document assignment completion stage",
			"payroll assignment approval stage",
			"payroll TSD assignment generation stage",
			"payroll payment-date assignment stage",
			"close assignment completion stage",
			"KMD assignment execution stage",
			"KMD tax remediation actions stage",
			"payroll run remediation actions stage",
			"document retention and evidence remediation actions stage",
			"TSD declaration remediation actions stage",
			"banking transaction and expense claim remediation action stages",
			"guarded migration execution stage",
			"server-side migration execution stage",
			"migration progress telemetry stage",
			"migration duration telemetry stage",
			"migration accountant-workspace launch handoff stage",
			"migration dashboard live stream stage",
			"migration provider preset catalog stage",
			"provider execution CSV canonicalization stage",
			"migration FK UUID preflight stage",
			"product supplier-code migration stage",
			"fixed-asset supplier-code migration stage",
			"supplier identity migration stage",
			"supplier VAT-number migration reference stage",
			"payment contact VAT-number migration reference stage",
			"payment and expense contact identity migration stage",
			"commercial-document contact identity migration stage",
			"commercial-document VAT contact import stage",
			"payment allocation consistency stage",
			"payment allocation amount decimal validation stage",
			"e-invoice payment allocation consistency stage",
			"payment allocation currency consistency stage",
			"payment bank-account default-currency consistency stage",
			"bank-account currency letter validation stage",
			"bank-transaction source-account omitted-currency consistency stage",
			"bank-transaction description-source preflight stage",
			"bank-transaction format execution-plan stage",
			"invoice paid-amount consistency stage",
			"combined invoice paid/allocation consistency stage",
			"payment allocation direction consistency stage",
			"payment invoice-contact consistency stage",
			"e-invoice payment invoice-contact consistency stage",
			"e-invoice credit-note payment contact selection stage",
			"payment allocation date consistency stage",
			"payment allocation malformed-date guard stage",
			"payment allocation invoice-status consistency stage",
			"payment allocation invoice-ID status consistency stage",
			"fixed-asset source-invoice consistency stage",
			"fixed-asset source-invoice date consistency stage",
			"fixed-asset source-invoice amount consistency stage",
			"fixed-asset source-invoice supplier identity stage",
			"stock-adjustment product stockability stage",
			"cost-allocation journal-line total consistency stage",
			"cost-allocation journal-line percentage consistency stage",
			"expense employee-ID UUID preflight stage",
			"product account-type consistency stage",
			"fixed-asset account-type consistency stage",
			"bank-account GL account-type consistency stage",
			"recurring-invoice account-type consistency stage",
			"`make test-integration-coverage`",
			"32 files and 572 tests",
			"all four seeded demo E2E shards",
		},
		"docs/DEVELOPMENT_STATUS.md": {
			"Full local baseline last completed on 2026-06-08. On 2026-06-12, the current branch was revalidated locally",
			"On 2026-06-13, the payroll-history-import branch was revalidated locally",
			"inventory subledger reconciliation stages",
			"PR #62 CI run `27475857115` at commit `a7a0068`",
			"frontend inventory subledger drill-down stage",
			"close remediation actions stage",
			"migration remediation actions stage",
			"migration remediation assignment stage",
			"cross-workspace remediation assignment metadata stage",
			"accountant workspace assignment queue stage",
			"expense assignment queue stage",
			"expense assignment completion stage",
			"document assignment completion stage",
			"payroll assignment approval stage",
			"payroll TSD assignment generation stage",
			"payroll payment-date assignment stage",
			"close assignment completion stage",
			"KMD assignment execution stage",
			"KMD tax remediation actions stage",
			"payroll run remediation actions stage",
			"TSD declaration remediation actions stage",
			"banking transaction remediation actions stage",
			"expense claim remediation actions stage",
			"guarded migration execution stage",
			"server-side migration execution stage",
			"migration accountant-workspace launch handoff coverage",
			"`make test-backend-coverage` passes without requiring PostgreSQL",
			"`make test-cli-coverage` passes as the focused CLI-only coverage gate",
			"`go test -timeout=3m ./docs -count=1` passes",
			"`go test -tags=integration ./internal/payroll -count=1` passes",
			"`DATABASE_URL=postgres://openaccounting:openaccounting@localhost:5432/openaccounting?sslmode=disable go test -tags=integration ./cmd/migrate -run TestEmailTemplateTypeMigrationAllowsQuoteAndOrderTemplates -count=1` passes",
			"`DATABASE_URL=postgres://openaccounting:openaccounting@localhost:5432/openaccounting?sslmode=disable make test-integration-coverage` passes",
			"`cd frontend && bun run test:prepared` passes with 32 files and 572 tests",
			"`cd frontend && bun run test:prepared -- AccountantReviewPanel.test.ts AccountantPortfolioPanel.test.ts review-workspace.test.ts api.test.ts` passes targeted accountant workspace assignment queue, expense frontend follow-up, expense assignment completion, document assignment approval, payroll assignment approval, payroll payment-date assignment, payroll TSD assignment generation, close assignment completion, and KMD assignment execution coverage",
			"`cd frontend && bun run test:prepared -- api.test.ts` passes",
			"`go test ./internal/accounting -run 'Test(Service_GetYearEndCloseStatusEdgeCases|BuildYearEndCloseRemediationActions)' -count=1` passes",
			"`go test ./cmd/api -run 'TestGetYearEndCloseStatus|TestGetYearEndCloseStatusIncludes' -count=1` passes",
			"`go test ./cmd/oa -run TestPrintCloseOutputs -count=1` passes",
			"`make swagger` regenerates the OpenAPI schema for `remediation_actions`",
			"`cd frontend && bun run check:prepared` passes against the close remediation API type additions",
			"`go test ./internal/cutover -run 'TestValidateBundle(ReportsReadyBundle|BuildsRemediationActions)' -count=1` passes",
			"`go test ./cmd/api -run TestValidateMigrationBundleHandler -count=1` passes",
			"`go test ./cmd/oa -run 'TestPrintOutputs|TestCLIMigrationValidation' -count=1` passes",
			"`make swagger` regenerates the OpenAPI schema for migration `remediation_actions`",
			"`cd frontend && bun run test:prepared -- api.test.ts` passes focused frontend API coverage for migration assignment-ready remediation actions",
			"`cd frontend && bun run check:prepared` passes against the migration assignment API type additions",
			"focused migration remediation priority fallback coverage",
			"`go test ./internal/cutover -count=1` passes focused migration execution-plan coverage",
			"`go test ./cmd/api -run 'Test(ValidateMigrationBundleHandler|PlanMigrationExecutionHandler|Routes)' -count=1` passes focused migration execution-plan API coverage",
			"`go test ./cmd/oa -run 'Test(CLI(MigrationValidationCommand|MigrationExecutionPlanCommand|MigrationValidationBranches)|OutputHelpers|CLIRouteCoverage)' -count=1` passes focused migration execution-plan CLI and docs-route coverage",
			"`go test ./cmd/oa -run 'TestCLIMigrationExecute|TestMigrationExecutionRunHelperBranches|TestPrintMigrationExecutionRunBranches|TestCLIRouteCoverage' -count=1` passes focused guarded migration execution CLI coverage",
			"`go test ./cmd/api -run 'Test(ExecuteMigrationHandler|PlanMigrationExecutionHandler|ValidateMigrationBundleHandler|Routes)' -count=1` passes focused server-side migration execution API coverage",
			"`go test ./internal/cutover ./internal/banking -count=1` passes shared migration execution-run and bank-account parser coverage",
			"`go test ./internal/cutover ./cmd/api ./cmd/oa -count=1` and `cd frontend && bun run test:prepared -- MigrationWorkbench.test.ts api.test.ts` pass focused migration progress and duration telemetry coverage",
			"`go test ./internal/cutover -count=1`, `make swagger`, `go test -timeout=3m ./docs -count=1`, `cd frontend && bun run test:prepared -- AccountantReviewPanel.test.ts MigrationWorkbench.test.ts api.test.ts`, and `cd frontend && bun run check:prepared` pass focused migration accountant-workspace launch handoff coverage",
			"`cd frontend && bun run test:prepared -- src/tests/components/MigrationWorkbench.test.ts src/tests/lib/api.test.ts` and `cd frontend && bun run check:prepared` pass focused migration dashboard live stream coverage",
			"`go test ./internal/cutover ./cmd/api ./cmd/oa -run 'Test(ListMigrationProviderPresets|CLIMigrationProviderPresets|CLIRouteCoverage)' -count=1`, `cd frontend && bun run test:prepared -- src/tests/components/MigrationWorkbench.test.ts src/tests/lib/api.test.ts`, and `cd frontend && bun run check:prepared` pass focused migration provider preset catalog coverage",
			"focused migration FK UUID preflight coverage",
			"focused product supplier-code migration coverage",
			"focused fixed-asset supplier-code migration coverage",
			"focused supplier identity migration coverage",
			"focused supplier VAT-number migration reference coverage",
			"focused payment contact VAT-number migration reference coverage",
			"focused payment and expense contact identity migration coverage",
			"commercial-document contact identity migration stage",
			"Commercial-document imports now resolve quote, order, and recurring-invoice `contact_vat_number`/`vat_number` columns against contact VAT numbers rather than registry codes.",
			"focused payment allocation consistency migration coverage",
			"focused payment allocation amount decimal validation coverage",
			"focused e-invoice payment allocation consistency migration coverage",
			"focused payment allocation currency consistency migration coverage",
			"focused payment bank-account default-currency consistency migration coverage",
			"focused bank-account currency letter validation coverage",
			"focused bank-transaction source-account omitted-currency consistency migration coverage",
			"focused bank-transaction description-source preflight coverage",
			"focused bank-transaction format execution-plan coverage",
			"focused invoice paid-amount consistency migration coverage",
			"focused combined invoice paid/allocation consistency migration coverage",
			"focused payment allocation direction consistency migration coverage",
			"focused payment invoice-contact consistency migration coverage",
			"focused e-invoice payment invoice-contact consistency migration coverage",
			"focused e-invoice credit-note payment contact selection coverage",
			"focused payment allocation date consistency migration coverage",
			"focused payment allocation malformed-date guard coverage",
			"focused payment allocation invoice-status consistency migration coverage",
			"focused payment allocation invoice-ID status consistency migration coverage",
			"focused fixed-asset source-invoice consistency migration coverage",
			"focused fixed-asset source-invoice date consistency migration coverage",
			"focused fixed-asset source-invoice amount consistency migration coverage",
			"focused fixed-asset source-invoice supplier identity coverage",
			"focused stock-adjustment product stockability migration coverage",
			"focused cost-allocation journal-line total consistency migration coverage",
			"focused cost-allocation journal-line percentage consistency migration coverage",
			"focused cost-allocation amount/percentage consistency migration coverage",
			"focused expense account-type consistency migration coverage",
			"focused expense employee-ID UUID preflight coverage",
			"focused product account-type consistency migration coverage",
			"focused fixed-asset account-type consistency migration coverage",
			"focused bank-account GL account-type consistency migration coverage",
			"focused recurring-invoice account-type consistency migration coverage",
			"separate SmartAccounts payroll/TSD year-month and tax amount aliases",
			"provider opening-balance amount alias validation and import coverage",
			"focused provider historical-journal alias import coverage",
			"focused provider cost-center and cost-allocation alias import coverage",
			"focused provider-preset execution CSV canonicalization coverage",
			"opening-balance account-code and debit/credit row values including Merit/Directo/SmartAccounts account and amount aliases in both validation and import execution",
			"Historical-journal provider aliases now execute through the mutating importer",
			"Provider cost-center and cost-allocation aliases now execute through the mutating importers",
			"Provider-preset migration execution now canonicalizes CSV headers with the same file-kind alias specs used by preflight",
			"Fixed-asset source-invoice supplier identity preflight now rejects mismatches for `supplier_id`, `supplier_reg_code`, `supplier_vat_number`, `supplier_email`, and `supplier_name`",
			"Product supplier VAT-number preflight now validates `supplier_vat_number` values against same-bundle contact VAT numbers",
			"Payment contact VAT-number preflight now validates `contact_vat_number` values against same-bundle contact VAT numbers",
			"Migration remediation priority fallback preflight keeps non-blocking action severities at low priority",
			"Payment allocation invoice-ID status preflight now rejects `invoice_id` allocations",
			"E-invoice credit-note payment contact preflight now selects buyer contact fields in customer mode",
			"provider-specific S3/rclone env examples and an executable host install helper",
			"Product migration preflight also rejects",
			"Fixed-asset migration preflight also rejects",
			"Bank-account migration preflight also rejects",
			"Recurring-invoice migration preflight also rejects",
			"payment allocation date consistency migration coverage",
			"`make swagger` regenerates the OpenAPI schema for server-side migration execution",
			"`cd frontend && bun run test:prepared src/tests/lib/api.test.ts` passes focused frontend API coverage for migration execution plans",
			"`make swagger` regenerates the OpenAPI schema for migration execution plans",
			"`go test ./internal/tax -run 'Test(Service_GenerateKMD_Success|Service_GenerateKMD_EmptyVATData|BuildKMDRemediationActions|Service_GetKMD_Success|Service_ListKMD_Success)' -count=1` passes",
			"`go test ./cmd/api -run TestTaxHandlersKMDWorkflow -count=1` passes",
			"`go test ./cmd/oa -run 'TestPrintOutputs|TestCLITaxAndTSDCommands' -count=1` passes",
			"`make swagger` regenerates the OpenAPI schema for KMD tax `remediation_actions`",
			"`go test ./internal/payroll -run 'Test(CreatePayrollRun_Success|UpdatePayrollRunPaymentDate|GetPayrollRun_Success|ListPayrollRuns_Success|ListPayrollRuns_FilterByYear|CalculatePayroll_Success|ProcessPayrollRun_CalculateOnly|ProcessPayrollRun_CalculatesAndApproves|CalculatePayroll_SkipsEmployeesWithoutSalary|BuildPayrollRunRemediationActions)' -count=1` passes",
			"`go test ./cmd/api -run TestPayrollBusinessHandlersRunLifecycleAndTSD -count=1` passes",
			"`go test ./cmd/oa -run 'TestPrintPayrollOutputs|TestCLIPayrollRunCommands' -count=1` passes",
			"`make swagger` regenerates the OpenAPI schema for payroll run `remediation_actions`",
			"`go test ./internal/documents -run 'TestService_UploadOpenListAndDeleteDocument|TestService_EvaluateEvidencePolicyValidation' -count=1` passes",
			"`go test ./cmd/api -run TestUploadListDownloadAndDeleteDocument -count=1` passes",
			"`go test ./cmd/oa -run 'TestPrintTables|TestCLIDocumentCommands' -count=1` passes",
			"`make swagger` regenerates the OpenAPI schema for document `remediation_actions`",
			"`go test ./internal/payroll -run 'Test(ServiceGenerateTSDWithMockRepository|ServiceTSDQuerySummaryAndStatusMarkers|BuildTSDRemediationActions)' -count=1` passes",
			"`go test ./cmd/api -run 'TestPayrollBusinessHandlers(RunLifecycleAndTSD|TSDPeriodActions)' -count=1` passes",
			"`go test ./cmd/oa -run 'TestPrintTaxReports|TestCLITaxAndTSDCommands' -count=1` passes",
			"`make swagger` regenerates the OpenAPI schema for TSD declaration `remediation_actions`",
			"`go test ./internal/expenses -run 'Test(ServiceExpenseRemediationHydration|BuildExpenseRemediationActions|ServiceExpenseLifecycleRequiresReceiptBeforeApproval|ServiceListExpensesNormalizesStatus)' -count=1` passes",
			"`go test ./cmd/api -run TestExpenseHandlers -count=1` passes",
			"`go test ./cmd/oa -run 'Test(PrintExpenseOutputs|CLIExpenseCommands|CLIExpenseBranches)' -count=1` passes",
			"`go test ./internal/expenses ./cmd/api ./cmd/oa -count=1` passes",
			"`make swagger` regenerates the OpenAPI schema for expense `remediation_actions`",
			"`go test ./internal/workspace ./internal/accounting ./internal/banking ./internal/expenses ./internal/documents ./internal/payroll ./internal/tax -run 'Test(RemediationAssignment|AssignmentPriority|NormalizeAssignmentPart|BuildYearEndCloseRemediationActions|BuildBankRemediationActions|BuildExpenseRemediationActions|RetentionReview|EvidencePolicy|BuildPayrollRunRemediationActions|BuildTSDRemediationActions|BuildKMDRemediationActions)' -count=1` passes focused cross-workspace assignment metadata coverage",
			"`go test ./cmd/oa -run 'TestPrint.*Outputs|TestRemediationAssignmentCells' -count=1` passes focused CLI rendering coverage for remediation assignment fields",
			"e2e/demo/inventory-stock.spec.ts --workers=1` passes with 3 inventory demo specs",
			"quotes.spec.ts e2e/demo/orders.spec.ts --workers=1` passes with auth setup plus quote and order delivery workflows",
			"Backend unit tests no longer start PostgreSQL in CI",
			"Backend integration tests are sharded in CI",
			"Full local seeded demo E2E runs across four blocking CI shards",
		},
		"docs/ARCHITECTURE.md": {
			"`go test -race ./...` must pass without PostgreSQL using Go's default package parallelism",
			"`DATABASE_URL=... make test-integration-coverage` must pass",
			"`INTEGRATION_SHARD` and `INTEGRATION_SHARDS`",
			"Blocking smoke E2E plus blocking local seeded demo shards",
		},
		"docs/demo-e2e-testing.md": {
			"The broader `e2e` job runs the full `demo-chromium` project across four shards and is blocking.",
			"The separate `e2e-demo` job targets an externally hosted demo and remains optional/informational",
		},
		"docs/FEATURE_MAPPING_MERIT_SMARTACCOUNTS.md": {
			"including the last full local baseline and current branch revalidation dates",
			"Testing and coverage status changed materially after this comparison was first drafted.",
		},
		".agents/skills/open-accounting-development/SKILL.md": {
			"demo1@example.com",
			"demo12345",
			"frontend/playwright.demo.config.ts",
			"bun run test:e2e:smoke",
		},
	}
	for path, snippets := range required {
		for _, snippet := range snippets {
			if !strings.Contains(activeDocs[path], snippet) {
				t.Fatalf("%s missing current status snippet %q", path, snippet)
			}
		}
	}

	staleSnippets := []string{
		"Full local baseline last verified on 2026-05-28",
		"Full local baseline last completed on 2026-05-28",
		"verified repository baseline as of 2026-04-24",
		"PR #62 CI run `27364700539` at commit `2d55632`",
		"PR #62 CI run `27365326654` at commit `4e608bb`",
		"PR #62 CI run `27385096250` at commit `1737a54`",
		"PR #62 CI run `27456883760` at commit `88157da`",
		"PR #62 CI run `27469291590` at commit `5a4280f`",
		"PR #62 CI run `27470177921` at commit `ef9a860`",
		"PR #62 CI run `27470695729` at commit `77f057c`",
		"PR #62 CI run `27473470206` at commit `991314e`",
		"PR #62 CI run `27473943178` at commit `f6a553f`",
		"PR #62 CI run `27474353861` at commit `c5cd5bc`",
		"PR #62 CI run `27474840778` at commit `4bb6e16`",
		"PR #62 CI run `27475433666` at commit `9888937`",
		"PR #62 CI run `27364049917` at commit `23f91bd`",
		"PR #62 CI run `27363638201`",
		"focused fixed-assets demo E2E with 3 passed",
		"21 files and 493 tests",
		"22 files and 517 tests",
		"22 files and 509 tests",
		"22 files and 510 tests",
		"22 files and 515 tests",
		"30 files and 546 tests",
		"30 files and 555 tests",
		"30 files and 557 tests",
		"31 files and 559 tests",
		"31 files and 569 tests",
		"251 passed",
		"259 passed",
		"`go test -p 1 -count=1 -race ./...`",
		"`go test -p 1 -race ./...`",
		"`go test -p 1 -race -coverprofile=/tmp/open-accounting-unit-coverage.out ./...`",
		"`go test -p 1 -count=1 -race -coverprofile=/tmp/open-accounting-unit-coverage.out ./...`",
		"`go test -p 1 -count=1 -race -tags=integration $(go list ./... | rg -v /testutil)`",
		"`go test -tags=integration -race ...` must pass",
		"Broad demo E2E remains informational",
		"Blocking smoke E2E plus informational demo shards",
		"The broader `e2e` job runs the full `demo-chromium` project in shards and is informational.",
		"The broader `e2e` job runs the full `demo-chromium` project in shards and is blocking.",
		"Full local seeded demo E2E shards are now blocking in CI",
		"dashboard-side live stream wiring",
		"provider presets and other dashboard-side incumbent-system mutating cutover controls are still incomplete",
		"two-shard local demo E2E CI job",
		"--shard=<shard>/2",
		"Shard ${{ matrix.shard }}/2",
		"demo@example.com",
		"`demo123`",
		"open-accounting.up.railway.app",
		"frontend/playwright.config.ts",
		"bun run test:e2e:demo",
	}
	for path, doc := range activeDocs {
		for _, stale := range staleSnippets {
			if strings.Contains(doc, stale) {
				t.Fatalf("%s contains stale status snippet %q", path, stale)
			}
		}
	}
}

func TestCLIGuideDocumentsPayrollMigrationSamples(t *testing.T) {
	payload, err := os.ReadFile("CLI.md")
	if err != nil {
		t.Fatalf("read CLI guide: %v", err)
	}
	guide := string(payload)

	for _, snippet := range []string{
		"### Payroll history",
		"period_year,period_month,status,payment_date,notes,employee_number,gross_salary",
		"### Leave balances",
		"year,employee_number,absence_type_code,entitled_days,carryover_days,used_days,pending_days,notes",
		"### TSD history",
		"period_year,period_month,status,submitted_at,emta_reference,employee_number,payment_type,gross_payment",
		"TSD history accepts aliases such as `tsd_year`, `tsd_month`, `declaration_status`, `submitted_date`, `emta_ref`, `employee_no`, `isikukood`, `gross_salary`, `taxable_income`, `unemployment_insurance_er`, `unemployment_insurance_ee`, and `pension`.",
		"Existing TSD declaration periods are skipped instead of overwritten.",
	} {
		if !strings.Contains(guide, snippet) {
			t.Fatalf("CLI guide missing payroll migration sample snippet %q", snippet)
		}
	}
}

func TestUseCaseCoverageMatrixDocumentsGoalEvidence(t *testing.T) {
	payload, err := os.ReadFile("USE_CASE_COVERAGE.md")
	if err != nil {
		t.Fatalf("read use case coverage matrix: %v", err)
	}
	matrix := string(payload)

	for _, snippet := range []string{
		"# Use Case Coverage Matrix",
		"`make test-cli-coverage` verifies `cmd/oa` at 100.0% statement coverage.",
		"`make test-backend-coverage` enforces the same CLI coverage from the backend coverage gate.",
		"`go test -timeout=3m ./docs -count=1` keeps the documentation status and route coverage checks active.",
		"| Historical migration and cutover | `Partial` |",
		"grouped migration remediation actions for ready bundles, unsupported file kinds, missing columns, missing references, duplicate identifiers, grouped consistency failures, malformed IDs, invalid row values, warning review, workspace queue assignment, stable assignment keys, priorities, and due windows",
		"dependency-aware execution plans for ready bundles with API/CLI import steps, missing-context markers for bank-transaction and opening-balance imports, guarded CLI plus server-side API execution for fully ready plans, provider-aware execution-time CSV header canonicalization for Merit/SmartAccounts/Directo imports, resume snapshots that skip previously succeeded steps when retrying interrupted runs, saved server-side execution run snapshots with list/get APIs, CLI access, status counters, progress percentages, active-step telemetry, per-step timestamps, and duration totals, saved-run event stream API/CLI access, provider preset catalog discovery for generic/Merit/SmartAccounts/Directo mapping metadata, dashboard live stream consumption, resume-by-ID support, accountant-workspace saved-run assignment handoff with deep links",
		"focused migration dashboard live stream tests",
		"SmartAccounts commercial contact alias stage",
		"SmartAccounts payroll/TSD year-month alias stage",
		"Merit/Directo commercial contact alias stage",
		"provider opening-balance amount alias stage",
		"provider historical-journal import alias stage",
		"provider cost-center and cost-allocation import alias stage",
		"provider execution CSV canonicalization stage",
		"migration remediation priority fallback stage",
		"Migration remediation priority fallback coverage now locks non-blocking action severities to low priority",
		"payment allocation invoice-ID status consistency stage",
		"Payment allocation invoice-ID status consistency coverage now locks",
		"pending-document assignment approval, calculated payroll-run payment-date setting and approval, approved payroll-run TSD generation, KMD regeneration/XML export assignment actions, draft expense submission, submitted expense approval, approved-expense ledger posting, fiscal-year close, and carry-forward posting directly from the dashboard",
		"a dashboard assignment queue that aggregates close, banking, document-retention, expense-claim, payroll-run, TSD, KMD, and migration cutover remediation actions with tenant-scoped deep links plus CLI commands",
		"KMD remediation actions for empty VAT periods, payable/refund/zero declarations, submitted declarations awaiting acceptance, missing submission timestamps, and accepted declaration archiving",
		"dashboard regeneration for empty KMD periods and XML export for actionable KMD review/archive assignments",
		"payroll run remediation actions for draft calculation, missing payment dates, zero-payslip review, approval, TSD generation, paid-run declaration follow-up, and declared payroll archive evidence",
		"TSD declaration remediation actions for empty rows/totals, draft export/submission, submitted declarations awaiting acceptance, missing submission timestamps, rejected declaration review, and accepted declaration archiving",
		"document remediation actions for missing retention, due-soon/expired retention, pending/rejected reviews, missing evidence, unapproved evidence, and evidence-policy violations",
		"focused migration FK UUID preflight tests",
		"focused product supplier-code migration tests",
		"focused fixed-asset supplier-code migration tests",
		"focused supplier identity migration tests",
		"supplier VAT-number migration reference stage",
		"Product supplier VAT-number preflight now validates `supplier_vat_number` values",
		"payment contact VAT-number migration reference stage",
		"Payment contact VAT-number preflight now validates `contact_vat_number` values",
		"focused payment and expense contact identity migration tests",
		"focused commercial-document contact identity migration tests",
		"Focused commercial-document VAT contact import tests",
		"focused payment allocation consistency migration tests",
		"payment allocation amount decimal validation stage",
		"Payment allocation amount decimal validation rejects malformed",
		"focused e-invoice payment allocation consistency migration tests",
		"focused payment allocation currency consistency migration tests",
		"focused invoice paid-amount consistency migration tests",
		"focused combined invoice paid/allocation consistency migration tests",
		"payment invoice-contact consistency stage",
		"e-invoice payment invoice-contact consistency stage",
		"e-invoice credit-note payment contact selection stage",
		"focused e-invoice credit-note payment contact selection migration tests",
		"Credit-note e-invoice allocations now select buyer contact references in customer mode",
		"payment allocation date consistency stage",
		"Payment allocation date consistency rejects",
		"focused payment allocation date consistency migration tests",
		"payment allocation malformed-date guard stage",
		"Payment allocation malformed-date guard coverage confirms malformed",
		"payment bank-account default-currency consistency stage",
		"Payment bank-account default-currency consistency rejects",
		"focused payment bank-account default-currency consistency migration tests",
		"bank-account currency letter validation stage",
		"Bank-account currency letter validation rejects",
		"bank-transaction source-account omitted-currency consistency stage",
		"Bank-transaction source-account omitted-currency consistency accepts",
		"focused bank-transaction source-account omitted-currency consistency migration tests",
		"bank-transaction description-source preflight stage",
		"Bank-transaction description-source preflight rejects",
		"focused bank-transaction description-source preflight tests",
		"bank-transaction format execution-plan stage",
		"Bank-transaction execution plans preserve the requested import format",
		"payment allocation invoice-status consistency stage",
		"Payment allocation invoice-status consistency rejects",
		"focused payment allocation invoice-status consistency migration tests",
		"customer-mode sales e-invoices where the payment contact must match the buyer",
		"fixed-asset source-invoice consistency migration tests",
		"fixed-asset source-invoice date consistency stage",
		"Fixed-asset source-invoice date consistency rejects",
		"focused fixed-asset source-invoice date consistency migration tests",
		"fixed-asset source-invoice amount consistency migration tests",
		"fixed-asset source-invoice supplier identity tests",
		"focused stock-adjustment product stockability migration tests",
		"focused cost-allocation journal-line total consistency migration tests",
		"focused cost-allocation journal-line percentage consistency migration tests",
		"focused cost-allocation amount/percentage consistency migration tests",
		"focused expense account-type consistency migration tests",
		"Expense employee-ID preflight rejects",
		"focused product account-type consistency migration tests",
		"focused fixed-asset account-type consistency migration tests",
		"focused bank-account GL account-type consistency migration tests",
		"focused recurring-invoice account-type consistency migration tests",
		"Fixed-asset source-invoice consistency now",
		"Fixed-asset source-invoice supplier identity consistency now covers",
		"`supplier_id`, `supplier_reg_code`, `supplier_vat_number`, `supplier_email`,",
		"purchase_date` is before the imported source invoice issue date",
		"Fixed-asset source-invoice amount consistency also rejects",
		"Stock-adjustment product stockability also rejects",
		"Cost-allocation journal-line total consistency also rejects",
		"Cost-allocation journal-line percentage consistency also rejects",
		"Cost-allocation amount/percentage consistency also rejects",
		"Expense account-type consistency also rejects",
		"expense `employee_id` UUID syntax",
		"Product account-type consistency also rejects",
		"Fixed-asset account-type consistency also rejects",
		"Bank-account GL account-type consistency also rejects",
		"Recurring-invoice account-type consistency also rejects",
		"supplier identity cross-file references by code, registry code, VAT number, email, or name",
		"commercial-document and payment/expense contact identity cross-file references by matching contact field",
		"preflight now checks invoice, quote, order, and recurring-invoice contact",
		"recurring-invoice contacts by VAT number",
		"SmartAccounts commercial-document provider presets now canonicalize",
		"SmartAccounts payroll and TSD-history provider presets now canonicalize separate",
		"Provider opening-balance presets and the opening-balance importer now",
		"Provider historical-journal presets and the historical-journal importer now",
		"Provider cost-center and cost-allocation presets and importers now",
		"Provider-preset migration execution now rewrites CSV headers",
		"focused provider execution CSV canonicalization tests",
		"validation and import execution run",
		"Merit and Directo commercial-document provider presets now canonicalize",
		"backup systemd hardening stage",
		"provider-specific backup env templates for S3/rclone",
		"generated host install helper for timer enablement",
		"payment bank-account default-currency consistency, bank-transaction source-account omitted-currency consistency, bank-transaction description-source preflight, invoice `amount_paid` consistency against imported invoice CSV totals and statuses, combined imported invoice paid amount/payment allocation totals, payment allocation totals against imported invoice CSV and e-invoice XML totals, payment allocation currency consistency against imported invoice CSV and e-invoice XML currencies, payment allocation direction consistency against imported invoice CSV and effective e-invoice XML invoice types, payment allocation date consistency against imported invoice CSV and e-invoice XML issue dates, payment allocation invoice-status consistency for imported invoice CSV draft/voided targets, ambiguous invoice-number reference checks, fixed-asset source-invoice purchase-type, supplier identity field, purchase-date, and amount-total consistency, stock-adjustment product stockability against same-bundle product type and tracking flags, expense/product/fixed-asset/bank-account GL and recurring-invoice account-type consistency against same-bundle chart-of-account rows, provider opening-balance account and amount aliases for Merit, SmartAccounts, and Directo exports, provider historical-journal entry/date/line/account/amount/currency aliases for Merit, SmartAccounts, and Directo exports in import execution",
		"payment bank-account and provider journal-line/cost-allocation cross-reference tests, provider opening-balance amount alias tests, provider historical-journal import alias tests, Merit/SmartAccounts payment, bank-data, expense, cost-allocation, inventory, fixed-asset, and KMD-history alias tests, Directo commercial/bank/journal/payroll/inventory/tax alias tests",
		"Further provider-specific mapping depth, additional cross-file validation, and dashboard-side mutating cutover controls remain open.",
		"| Direct bank feeds, direct SEPA initiation, e-invoice operator exchange, OCR, and automatic authority filing | `Blocked` |",
		"Keep replacing uncovered migration validator branches with focused tests until the use-case coverage evidence is no longer mostly indirect.",
	} {
		if !strings.Contains(matrix, snippet) {
			t.Fatalf("use case coverage matrix missing snippet %q", snippet)
		}
	}

	read := func(path string) string {
		t.Helper()
		payload, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		return string(payload)
	}
	for path, doc := range map[string]string{
		filepath.Join("..", "README.md"): read(filepath.Join("..", "README.md")),
		"DEVELOPMENT_STATUS.md":          read("DEVELOPMENT_STATUS.md"),
	} {
		if !strings.Contains(doc, "USE_CASE_COVERAGE.md") {
			t.Fatalf("%s missing use case coverage matrix link", path)
		}
	}
}

func TestWorkflowDemoE2EGatesMatchDocumentation(t *testing.T) {
	workflowPayload, err := os.ReadFile(filepath.Join("..", ".github", "workflows", "ci.yml"))
	if err != nil {
		t.Fatalf("read workflow: %v", err)
	}
	workflow := string(workflowPayload)

	localE2E := workflowJobBlock(t, workflow, "e2e")
	for _, snippet := range []string{
		"shard: [1, 2, 3, 4]",
		"total-shards: [4]",
		"Run Demo E2E tests (Shard ${{ matrix.shard }}/${{ matrix.total-shards }})",
		"--shard=${{ matrix.shard }}/${{ matrix.total-shards }}",
	} {
		if !strings.Contains(localE2E, snippet) {
			t.Fatalf("local e2e job missing shard snippet %q", snippet)
		}
	}
	if workflowJobHasJobLevelContinueOnError(localE2E) {
		t.Fatal("local seeded e2e job must be a blocking CI gate")
	}

	remoteDemo := workflowJobBlock(t, workflow, "e2e-demo")
	if !strings.Contains(remoteDemo, "continue-on-error: true") {
		t.Fatal("remote hosted demo job should remain optional/informational")
	}
	if !strings.Contains(remoteDemo, "TEST_DEMO: 'true'") && !strings.Contains(remoteDemo, `TEST_DEMO: "true"`) {
		t.Fatal("remote hosted demo job must opt into hosted demo testing")
	}
}

func TestWorkflowIntegrationShardsMatchMakefile(t *testing.T) {
	workflowPayload, err := os.ReadFile(filepath.Join("..", ".github", "workflows", "ci.yml"))
	if err != nil {
		t.Fatalf("read workflow: %v", err)
	}
	workflow := string(workflowPayload)
	integrationJob := workflowJobBlock(t, workflow, "integration-test")
	for _, snippet := range []string{
		"shard: [1, 2, 3, 4]",
		"total-shards: [4]",
		"INTEGRATION_SHARD: ${{ matrix.shard }}",
		"INTEGRATION_SHARDS: ${{ matrix.total-shards }}",
		"make test-integration-coverage",
	} {
		if !strings.Contains(integrationJob, snippet) {
			t.Fatalf("integration workflow missing shard snippet %q", snippet)
		}
	}

	makefilePayload, err := os.ReadFile(filepath.Join("..", "Makefile"))
	if err != nil {
		t.Fatalf("read Makefile: %v", err)
	}
	makefile := string(makefilePayload)
	for _, snippet := range []string{
		"INTEGRATION_PACKAGE_SHARD",
		"scripts/select-integration-packages.sh",
		"test-integration-coverage:",
	} {
		if !strings.Contains(makefile, snippet) {
			t.Fatalf("Makefile missing integration shard snippet %q", snippet)
		}
	}

	selectorPayload, err := os.ReadFile(filepath.Join("..", "scripts", "select-integration-packages.sh"))
	if err != nil {
		t.Fatalf("read integration shard selector: %v", err)
	}
	selector := string(selectorPayload)
	for _, snippet := range []string{
		"INTEGRATION_SHARD and INTEGRATION_SHARDS must be set together",
		"INTEGRATION_PACKAGE_WEIGHTS",
		"scripts/integration-package-weights.tsv",
	} {
		if !strings.Contains(selector, snippet) {
			t.Fatalf("integration shard selector missing snippet %q", snippet)
		}
	}

	weightsPayload, err := os.ReadFile(filepath.Join("..", "scripts", "integration-package-weights.tsv"))
	if err != nil {
		t.Fatalf("read integration package weights: %v", err)
	}
	if !strings.Contains(string(weightsPayload), "./internal/accounting") {
		t.Fatal("integration package weights missing expected package entries")
	}

	architecturePayload, err := os.ReadFile("ARCHITECTURE.md")
	if err != nil {
		t.Fatalf("read architecture docs: %v", err)
	}
	architecture := string(architecturePayload)
	for _, snippet := range []string{
		"Package selection is weight-aware",
		"`scripts/select-integration-packages.sh`",
		"`scripts/integration-package-weights.tsv`",
	} {
		if !strings.Contains(architecture, snippet) {
			t.Fatalf("architecture docs missing integration shard snippet %q", snippet)
		}
	}
}

func workflowJobBlock(t *testing.T, workflow, jobName string) string {
	t.Helper()
	lines := strings.Split(workflow, "\n")
	start := -1
	for index, line := range lines {
		if line == "  "+jobName+":" {
			start = index
			break
		}
	}
	if start == -1 {
		t.Fatalf("workflow job %s not found", jobName)
	}

	end := len(lines)
	for index := start + 1; index < len(lines); index++ {
		line := lines[index]
		if strings.HasPrefix(line, "  ") && !strings.HasPrefix(line, "    ") && strings.TrimSpace(line) != "" {
			end = index
			break
		}
	}
	return strings.Join(lines[start:end], "\n")
}

func workflowJobHasJobLevelContinueOnError(jobBlock string) bool {
	for _, line := range strings.Split(jobBlock, "\n") {
		if strings.HasPrefix(line, "    continue-on-error:") {
			return true
		}
	}
	return false
}
