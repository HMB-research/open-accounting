package docs

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func readDoc(t *testing.T, path string) string {
	t.Helper()
	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(payload)
}

func TestStatusDocumentationTracksCurrentGates(t *testing.T) {
	activeDocs := map[string]string{
		filepath.Join("..", "README.md"):         readDoc(t, filepath.Join("..", "README.md")),
		"DEVELOPMENT_STATUS.md":                  readDoc(t, "DEVELOPMENT_STATUS.md"),
		"CURRENT_PRODUCT_LIMITS.md":              readDoc(t, "CURRENT_PRODUCT_LIMITS.md"),
		"USE_CASE_COVERAGE.md":                   readDoc(t, "USE_CASE_COVERAGE.md"),
		"FEATURE_MAPPING_MERIT_SMARTACCOUNTS.md": readDoc(t, "FEATURE_MAPPING_MERIT_SMARTACCOUNTS.md"),
		"EMTA_INTEGRATION.md":                    readDoc(t, "EMTA_INTEGRATION.md"),
		"PLUGINS.md":                             readDoc(t, "PLUGINS.md"),
		"ARCHITECTURE.md":                        readDoc(t, "ARCHITECTURE.md"),
		"DEPLOYMENT.md":                          readDoc(t, "DEPLOYMENT.md"),
		"README.md":                              readDoc(t, "README.md"),
		"demo-e2e-testing.md":                    readDoc(t, "demo-e2e-testing.md"),
		filepath.Join("..", ".agents", "skills", "open-accounting-development", "SKILL.md"): readDoc(t, filepath.Join("..", ".agents", "skills", "open-accounting-development", "SKILL.md")),
	}

	required := map[string][]string{
		filepath.Join("..", "README.md"): {
			"This project is under active development and not yet production-ready.",
			"actions/workflows/coverage.yml/badge.svg?branch=main",
			"[documentation index](docs/README.md)",
			"[Current Product Limits](docs/CURRENT_PRODUCT_LIMITS.md)",
			"[Development Status](docs/DEVELOPMENT_STATUS.md)",
			"[Use Case Coverage](docs/USE_CASE_COVERAGE.md)",
		},
		"README.md": {
			"[Current Product Limits](./CURRENT_PRODUCT_LIMITS.md)",
			"[Development Status](./DEVELOPMENT_STATUS.md)",
			"[Use Case Coverage](./USE_CASE_COVERAGE.md)",
			"[EMTA Integration](./EMTA_INTEGRATION.md)",
		},
		"DEVELOPMENT_STATUS.md": {
			"> Last updated: 2026-06-27",
			"latest verified local baseline reviewed on 2026-06-27",
			"Backend coverage: 100.0% statements (`45097/45097`, zero missed statements).",
			"Frontend coverage: 100.0% statements, 100.0% functions, 100.0% lines, and 94.27% branches.",
			"Documentation gate: `go test -timeout=3m ./docs -count=1` passing.",
			"Historical pull-request logs belong in PRs and CI, not in this status page.",
			"## Capability Matrix",
			"## Current Verification Gates",
			"make test-backend-coverage",
			"cd frontend && bun run test:prepared",
		},
		"CURRENT_PRODUCT_LIMITS.md": {
			"Last reviewed: 2026-06-25",
			"Current gate evidence is maintained in\n[Development Status](./DEVELOPMENT_STATUS.md).",
			"## Gaps From A Full-Featured Product",
			"Use this file for the concise product cap/gap summary and open work.",
			"Do not move an item out of the gaps table until there is authoritative code, test, and documentation evidence",
		},
		"USE_CASE_COVERAGE.md": {
			"Last reviewed: 2026-06-27",
			"Latest verified baseline reviewed on 2026-06-27",
			"`make test-cli-coverage` verifies `cmd/oa` at 100.0% statement coverage.",
			"`make test-backend-coverage` verifies the backend at 100.0% statement coverage",
			"`go test -timeout=3m ./docs -count=1` keeps the documentation status, route coverage, and link checks active.",
			"Broad workflow proof is summarized by matrix area instead of by per-stage pull-request history.",
			"## Matrix",
			"## Sources Of Truth",
			"Use [Development Status](./DEVELOPMENT_STATUS.md) for the latest gate evidence",
		},
		"FEATURE_MAPPING_MERIT_SMARTACCOUNTS.md": {
			"# Feature Mapping: Merit & SmartAccounts vs Open Accounting",
			"## Blockers Summary",
			"For the verified repository baseline and current branch gate summary",
			"Treat this file as dated vendor research",
			"## Verification Note",
		},
		"EMTA_INTEGRATION.md": {
			"# e-MTA Manual Export And Blocked Automatic Submission",
			"**Status: BLOCKED / EXTERNAL INTEGRATION NOT LIVE**",
			"Automatic e-MTA submission is not implemented",
			"## X-Road Integration Requirements",
			"### X-Road Service Verification",
			"## External Integration Boundary",
			"Existing TSD status endpoints record manual workflow state; they do not submit declarations to e-MTA.",
			"## Boundary Until Blockers Clear",
			"Do not describe automatic submission, status polling, or feedback retrieval as working product capabilities.",
		},
		"DEPLOYMENT.md": {
			"The demo environment supports 4 parallel users for E2E testing:",
			"demo4@example.com",
			"tenant_demo4",
			"resets all 4 demo tenants simultaneously",
		},
		"ARCHITECTURE.md": {
			"Web App",
			"API Client",
			"bun run test:prepared",
			"bun run test:coverage",
		},
		"demo-e2e-testing.md": {
			"The broader `e2e` job runs the full `demo-chromium` project across four shards and is blocking.",
			"The separate `e2e-demo` job targets an externally hosted demo and remains optional/informational",
		},
		filepath.Join("..", ".agents", "skills", "open-accounting-development", "SKILL.md"): {
			"demo1@example.com",
			"demo12345",
			"frontend/playwright.demo.config.ts",
			"bun run test:e2e:smoke",
			"keep design notes in the task, PR description, or the current canonical docs instead of adding legacy plan files",
		},
	}

	for path, snippets := range required {
		doc := activeDocs[path]
		for _, snippet := range snippets {
			if !strings.Contains(doc, snippet) {
				t.Fatalf("%s missing current status snippet %q", path, snippet)
			}
		}
	}

	for path, doc := range activeDocs {
		for _, stale := range []string{
			"PR #62",
			"payroll-history-import",
			"27512752079",
			"97e9d8b",
			"2026-06-15",
			"June 12, 2026 verification pass",
			"## Priority Themes",
			"## Timeline",
			"Proposed Implementation",
			"estimated implementation time",
			"Current branch documentation baseline",
			"`chore/coverage-docs-reorg` branch baseline",
			"Legacy plan docs have been removed",
			"Legacy development plans were removed",
			"## Immediate Priorities",
			"## Open Goal Work Items",
			"## Stage Gates To Keep Current",
			"Candidate Shape After Blockers Clear",
			"Candidate API surface",
			"Candidate persistence",
			"Recommendation: Create",
			"roughly 60-70%",
			"Mobile App",
			"(Future)",
			"codecov",
			"Codecov",
			"3 parallel users",
			"all 3 demo tenants",
			"Legacy manifest metadata",
			"future component-runtime work",
		} {
			if strings.Contains(doc, stale) {
				t.Fatalf("%s contains stale status snippet %q", path, stale)
			}
		}
	}
}

func TestDeploymentDemoUserDocumentationMatchesCode(t *testing.T) {
	demoUsers := readDoc(t, filepath.Join("..", "cmd", "api", "demo_users.go"))
	deployment := readDoc(t, "DEPLOYMENT.md")

	emailMatches := regexp.MustCompile(`email: "demo[0-9]+@example\.com"`).FindAllString(demoUsers, -1)
	schemaMatches := regexp.MustCompile(`schema: "tenant_demo[0-9]+"`).FindAllString(demoUsers, -1)
	requireCount := len(emailMatches)
	if requireCount == 0 || len(schemaMatches) != requireCount {
		t.Fatalf("expected demo user source to expose matching email/schema rows, got emails=%d schemas=%d", requireCount, len(schemaMatches))
	}

	for _, snippet := range []string{
		fmt.Sprintf("supports %d parallel users", requireCount),
		fmt.Sprintf("resets all %d demo tenants", requireCount),
	} {
		if !strings.Contains(deployment, snippet) {
			t.Fatalf("deployment docs missing demo count snippet %q", snippet)
		}
	}
	for i := 1; i <= requireCount; i++ {
		for _, snippet := range []string{
			fmt.Sprintf("demo%d@example.com", i),
			fmt.Sprintf("tenant_demo%d", i),
		} {
			if !strings.Contains(deployment, snippet) {
				t.Fatalf("deployment docs missing demo user snippet %q", snippet)
			}
		}
	}
}

func TestCLIGuideDocumentsPayrollMigrationSamples(t *testing.T) {
	guide := readDoc(t, "CLI.md")

	for _, snippet := range []string{
		"### Payroll history",
		"period_year,period_month,status,payment_date,notes,employee_number,gross_salary",
		"### Leave balances",
		"year,employee_number,absence_type_code,entitled_days,carryover_days,used_days,pending_days,notes",
		"### TSD history",
		"period_year,period_month,status,submitted_at,emta_reference,employee_number,payment_type,gross_payment",
		"TSD history accepts aliases such as `tsd_year`, `tsd_month`, `declaration_status`, `submitted_date`, `emta_ref`, `employee_no`, `isikukood`, `gross_salary`, `taxable_income`, `unemployment_insurance_er`, `unemployment_insurance_ee`, and `pension`.",
		"Existing TSD declaration periods are skipped instead of overwritten.",
		"go run ./cmd/oa migration execute \\\n  --employees ./employees.csv \\\n  --payroll-history ./payroll-history.csv \\\n  --leave-balances ./leave-balances.csv \\\n  --tsd-history ./tsd-history.csv",
		"`tsd mark-submitted` and `tsd mark-accepted` require one approved `tax_support` or `supporting_document` uploaded to `--entity-type tsd_declaration`",
		"--entity-type kmd_declaration",
		"go run ./cmd/oa tax kmd mark-submitted --year 2026 --month 3",
		"go run ./cmd/oa tax kmd mark-accepted --year 2026 --month 3",
		"`tax kmd mark-submitted` and `tax kmd mark-accepted` require one approved `tax_support` or `supporting_document` uploaded to `--entity-type kmd_declaration`",
		"`tax kmd mark-submitted` records the declaration as submitted with the current server timestamp.",
		"go run ./cmd/oa documents review-summary --entity-type payment --entity-id <payment-id> --entity-id <second-payment-id>",
		"Backend hook and route manifest entries are executable when they declare a supported `backend.runtime`: `http` proxies to an operator-managed loopback process with `backend.base_url`, while `package` starts a plugin-local executable and waits for its loopback health endpoint before proxying hooks and tenant runtime routes.",
	} {
		if !strings.Contains(guide, snippet) {
			t.Fatalf("CLI guide missing payroll migration sample snippet %q", snippet)
		}
	}
	if strings.Contains(guide, "Backend hook and route manifest entries are still rejected during plugin enablement until a backend runtime exists.") {
		t.Fatal("CLI guide still describes backend plugin runtime as unavailable")
	}
}

func TestUseCaseCoverageMatrixDocumentsGoalEvidence(t *testing.T) {
	matrix := readDoc(t, "USE_CASE_COVERAGE.md")

	for _, snippet := range []string{
		"# Use Case Coverage Matrix",
		"Latest verified baseline reviewed on 2026-06-27",
		"`make test-cli-coverage` verifies `cmd/oa` at 100.0% statement coverage.",
		"`make test-backend-coverage` verifies the backend at 100.0% statement coverage",
		"`go test -timeout=3m ./docs -count=1` keeps the documentation status, route coverage, and link checks active.",
		"| Historical migration and cutover | `Partial` |",
		"provider-aware execution-time CSV header canonicalization for Merit/SmartAccounts/Directo imports including payroll, leave-balance, and TSD history payloads",
		"legal hold placement/release audit metadata with disposal, replacement, hard-delete, and purge guards",
		"loopback-only out-of-process HTTP backend runtime and supervised package runtime startup/proxy/shutdown for manifest-declared hooks and tenant-scoped plugin routes",
		"| Direct bank feeds, direct SEPA initiation, e-invoice operator exchange, OCR, and automatic authority filing | `Blocked` |",
		"Use [Current Product Limits](./CURRENT_PRODUCT_LIMITS.md)\nfor the concise open work and product gap list.",
	} {
		if !strings.Contains(matrix, snippet) {
			t.Fatalf("use case coverage matrix missing snippet %q", snippet)
		}
	}

	for path, doc := range map[string]string{
		filepath.Join("..", "README.md"): readDoc(t, filepath.Join("..", "README.md")),
		"DEVELOPMENT_STATUS.md":          readDoc(t, "DEVELOPMENT_STATUS.md"),
	} {
		if !strings.Contains(doc, "USE_CASE_COVERAGE.md") {
			t.Fatalf("%s missing use case coverage matrix link", path)
		}
	}
}

func TestWorkflowDemoE2EGatesMatchDocumentation(t *testing.T) {
	workflow := readDoc(t, filepath.Join("..", ".github", "workflows", "ci.yml"))

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
	workflow := readDoc(t, filepath.Join("..", ".github", "workflows", "ci.yml"))
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

	makefile := readDoc(t, filepath.Join("..", "Makefile"))
	for _, snippet := range []string{
		"INTEGRATION_PACKAGE_SHARD",
		"scripts/select-integration-packages.sh",
		"test-integration-coverage:",
	} {
		if !strings.Contains(makefile, snippet) {
			t.Fatalf("Makefile missing integration shard snippet %q", snippet)
		}
	}

	selector := readDoc(t, filepath.Join("..", "scripts", "select-integration-packages.sh"))
	for _, snippet := range []string{
		"INTEGRATION_SHARD and INTEGRATION_SHARDS must be set together",
		"INTEGRATION_PACKAGE_WEIGHTS",
		"scripts/integration-package-weights.tsv",
	} {
		if !strings.Contains(selector, snippet) {
			t.Fatalf("integration shard selector missing snippet %q", snippet)
		}
	}

	weights := readDoc(t, filepath.Join("..", "scripts", "integration-package-weights.tsv"))
	if !strings.Contains(weights, "./internal/accounting") {
		t.Fatal("integration package weights missing expected package entries")
	}

	architecture := readDoc(t, "ARCHITECTURE.md")
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
