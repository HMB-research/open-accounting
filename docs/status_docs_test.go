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
			"`make test-cli-coverage`",
			"`DATABASE_URL=postgres://openaccounting:openaccounting@localhost:5432/openaccounting?sslmode=disable make test-integration-coverage`",
			"full demo E2E shards are CI gates",
		},
		"docs/DEVELOPMENT_STATUS.md": {
			"Full local baseline last completed on 2026-06-08",
			"`go test -p 1 -count=1 -race -coverprofile=/tmp/open-accounting-unit-coverage.out ./...` passes without requiring PostgreSQL",
			"`make test-cli-coverage` passes",
			"`DATABASE_URL=postgres://openaccounting:openaccounting@localhost:5432/openaccounting?sslmode=disable make test-integration-coverage` passes",
			"`cd frontend && bun run test` passes with 22 files and 515 tests",
			"Backend unit tests no longer start PostgreSQL in CI",
			"Backend integration tests are sharded in CI",
			"Full local seeded demo E2E runs across four blocking CI shards",
		},
		"docs/ARCHITECTURE.md": {
			"`go test -p 1 -race ./...` must pass without PostgreSQL",
			"`DATABASE_URL=... make test-integration-coverage` must pass",
			"`INTEGRATION_SHARD` and `INTEGRATION_SHARDS`",
			"Blocking smoke E2E plus blocking local seeded demo shards",
		},
		"docs/demo-e2e-testing.md": {
			"The broader `e2e` job runs the full `demo-chromium` project across four shards and is blocking.",
			"The separate `e2e-demo` job targets an externally hosted demo and remains optional/informational",
		},
		"docs/FEATURE_MAPPING_MERIT_SMARTACCOUNTS.md": {
			"verified repository baseline as of 2026-06-08",
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
		"21 files and 493 tests",
		"22 files and 509 tests",
		"22 files and 510 tests",
		"251 passed",
		"259 passed",
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
	if strings.Contains(localE2E, "continue-on-error") {
		t.Fatal("local seeded e2e job must be a blocking CI gate")
	}

	remoteDemo := workflowJobBlock(t, workflow, "e2e-demo")
	if !strings.Contains(remoteDemo, "continue-on-error: true") {
		t.Fatal("remote hosted demo job should remain optional/informational")
	}
	if !strings.Contains(remoteDemo, "TEST_DEMO: 'true'") {
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
		"INTEGRATION_SHARD and INTEGRATION_SHARDS must be set together",
		"test-integration-coverage:",
	} {
		if !strings.Contains(makefile, snippet) {
			t.Fatalf("Makefile missing integration shard snippet %q", snippet)
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
