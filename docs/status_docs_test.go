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
			"cross-workspace remediation assignment metadata stage",
			"accountant workspace assignment queue stage",
			"expense assignment queue stage",
			"document assignment completion stage",
			"KMD tax remediation actions stage",
			"payroll run remediation actions stage",
			"document retention and evidence remediation actions stage",
			"TSD declaration remediation actions stage",
			"banking transaction and expense claim remediation action stages",
			"`make test-integration-coverage`",
			"31 files and 560 tests",
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
			"document assignment completion stage",
			"KMD tax remediation actions stage",
			"payroll run remediation actions stage",
			"TSD declaration remediation actions stage",
			"banking transaction remediation actions stage",
			"expense claim remediation actions stage",
			"`make test-backend-coverage` passes without requiring PostgreSQL",
			"`make test-cli-coverage` passes as the focused CLI-only coverage gate",
			"`go test -timeout=3m ./docs -count=1` passes",
			"`go test -tags=integration ./internal/payroll -count=1` passes",
			"`DATABASE_URL=postgres://openaccounting:openaccounting@localhost:5432/openaccounting?sslmode=disable go test -tags=integration ./cmd/migrate -run TestEmailTemplateTypeMigrationAllowsQuoteAndOrderTemplates -count=1` passes",
			"`DATABASE_URL=postgres://openaccounting:openaccounting@localhost:5432/openaccounting?sslmode=disable make test-integration-coverage` passes",
			"`cd frontend && bun run test:prepared` passes with 31 files and 560 tests",
			"`cd frontend && bun run test:prepared -- AccountantReviewPanel.test.ts AccountantPortfolioPanel.test.ts review-workspace.test.ts api.test.ts` passes targeted accountant workspace assignment queue, expense frontend follow-up, and document assignment approval coverage",
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
			"`go test ./internal/tax -run 'Test(Service_GenerateKMD_Success|Service_GenerateKMD_EmptyVATData|BuildKMDRemediationActions|Service_GetKMD_Success|Service_ListKMD_Success)' -count=1` passes",
			"`go test ./cmd/api -run TestTaxHandlersKMDWorkflow -count=1` passes",
			"`go test ./cmd/oa -run 'TestPrintOutputs|TestCLITaxAndTSDCommands' -count=1` passes",
			"`make swagger` regenerates the OpenAPI schema for KMD tax `remediation_actions`",
			"`go test ./internal/payroll -run 'Test(CreatePayrollRun_Success|GetPayrollRun_Success|ListPayrollRuns_Success|ListPayrollRuns_FilterByYear|CalculatePayroll_Success|ProcessPayrollRun_CalculateOnly|ProcessPayrollRun_CalculatesAndApproves|CalculatePayroll_SkipsEmployeesWithoutSalary|BuildPayrollRunRemediationActions)' -count=1` passes",
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
		"pending-document assignment approval directly from the dashboard",
		"a dashboard assignment queue that aggregates close, banking, document-retention, expense-claim, payroll-run, TSD, and KMD remediation actions with tenant-scoped deep links plus CLI commands",
		"KMD remediation actions for empty VAT periods, payable/refund/zero declarations, submitted declarations awaiting acceptance, missing submission timestamps, and accepted declaration archiving",
		"payroll run remediation actions for draft calculation, missing payment dates, zero-payslip review, approval, TSD generation, paid-run declaration follow-up, and declared payroll archive evidence",
		"TSD declaration remediation actions for empty rows/totals, draft export/submission, submitted declarations awaiting acceptance, missing submission timestamps, rejected declaration review, and accepted declaration archiving",
		"document remediation actions for missing retention, due-soon/expired retention, pending/rejected reviews, missing evidence, unapproved evidence, and evidence-policy violations",
		"payment bank-account and provider journal-line/cost-allocation cross-reference tests, Merit/SmartAccounts payment, bank-data, expense, cost-allocation, inventory, fixed-asset, and KMD-history alias tests",
		"Full incumbent-system cutover paths, broader vendor mapping presets, deeper cross-file validation, and workspace execution of cutover tasks remain open.",
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
