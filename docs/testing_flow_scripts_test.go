package docs

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseGoTestPackageTimesScript(t *testing.T) {
	input := strings.Join([]string{
		"integration-test (1, 4)\tRun integration tests\t2026-06-10T12:03:26Z ok  \tgithub.com/HMB-research/open-accounting/internal/accounting\t76.415s\tcoverage: 78.2% of statements",
		"ok  \tgithub.com/HMB-research/open-accounting/cmd/api\t14.751s\tcoverage: 71.0% of statements",
		"?   \tgithub.com/HMB-research/open-accounting/internal/unused\t[no test files]",
		"ok  \tgithub.com/example/other/internal/nope\t1.000s\tcoverage: 99.0% of statements",
		"ok  \t./internal/local\t2.500s\tcoverage: 88.0% of statements",
	}, "\n")

	cmd := exec.Command("../scripts/parse-go-test-package-times.sh")
	cmd.Stdin = strings.NewReader(input)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("parse package times: %v\n%s", err, output)
	}

	expected := strings.Join([]string{
		"./cmd/api\t14.751",
		"./internal/accounting\t76.415",
		"./internal/local\t2.500",
		"",
	}, "\n")
	if string(output) != expected {
		t.Fatalf("unexpected package times:\nwant:\n%s\ngot:\n%s", expected, output)
	}
}

func TestParsePlaywrightSpecTimesScript(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node not available")
	}

	fixture := `{
  "suites": [
    {
      "title": "demo/env.spec.ts",
      "file": "demo/env.spec.ts",
      "specs": [
        {
          "title": "loads environment page",
          "file": "demo/env.spec.ts",
          "tests": [
            {
              "results": [
                {"status": "passed", "duration": 1500}
              ]
            }
          ]
        },
        {
          "title": "retries a slow check",
          "file": "demo/env.spec.ts",
          "tests": [
            {
              "results": [
                {"status": "failed", "duration": 1000},
                {"status": "passed", "duration": 2000}
              ]
            }
          ]
        }
      ]
    },
    {
      "title": "demo/mobile.spec.ts",
      "file": "demo/mobile.spec.ts",
      "specs": [],
      "suites": [
        {
          "title": "Mobile Navigation",
          "file": "demo/mobile.spec.ts",
          "specs": [
            {
              "title": "uses inherited suite file",
              "tests": [
                {
                  "results": [
                    {"status": "passed", "duration": 3000}
                  ]
                }
              ]
            }
          ]
        }
      ]
    }
  ]
}`

	path := filepath.Join(t.TempDir(), "demo-test-results.json")
	if err := os.WriteFile(path, []byte(fixture), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	cmd := exec.Command("../scripts/parse-playwright-spec-times.mjs", path)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("parse Playwright spec times: %v\n%s", err, output)
	}

	expected := strings.Join([]string{
		"demo/env.spec.ts\t4.500\t2\t3",
		"demo/mobile.spec.ts\t3.000\t1\t1",
		"",
	}, "\n")
	if string(output) != expected {
		t.Fatalf("unexpected Playwright spec times:\nwant:\n%s\ngot:\n%s", expected, output)
	}
}

func TestRunAffectedTestsScriptSelectsBackendDependants(t *testing.T) {
	cmd := exec.Command("../scripts/run-affected-tests.sh", "--list", "--changed-file", "internal/cutover/validate.go")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("list affected backend tests: %v\n%s", err, output)
	}

	text := string(output)
	for _, snippet := range []string{
		"go test -count=1 -race",
		"./internal/cutover",
	} {
		if !strings.Contains(text, snippet) {
			t.Fatalf("affected backend output missing %q:\n%s", snippet, text)
		}
	}
}

func TestRunAffectedTestsScriptSelectsDocsForScriptChanges(t *testing.T) {
	cmd := exec.Command("../scripts/run-affected-tests.sh", "--list", "--changed-file", "scripts/run-affected-tests.sh")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("list affected script tests: %v\n%s", err, output)
	}

	expected := "go test -timeout=3m ./docs -count=1"
	if strings.TrimSpace(string(output)) != expected {
		t.Fatalf("unexpected script affected tests:\nwant: %s\ngot:\n%s", expected, output)
	}
}

func TestRunAffectedTestsScriptSelectsMigrationTests(t *testing.T) {
	cmd := exec.Command("../scripts/run-affected-tests.sh", "--list", "--changed-file", "migrations/053_quote_order_email_template_types.up.sql")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("list affected migration tests: %v\n%s", err, output)
	}

	expected := "go test -timeout=5m -tags=integration ./cmd/migrate -count=1"
	if strings.TrimSpace(string(output)) != expected {
		t.Fatalf("unexpected migration affected tests:\nwant: %s\ngot:\n%s", expected, output)
	}
}

func TestRunAffectedTestsScriptSelectsFrontendChangedTests(t *testing.T) {
	cmd := exec.Command("../scripts/run-affected-tests.sh", "--list", "--base", "HEAD", "--changed-file", "frontend/src/lib/api.ts")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("list affected frontend tests: %v\n%s", err, output)
	}

	expected := "cd frontend && bun run paraglide && bun run test:prepared -- --changed HEAD"
	if strings.TrimSpace(string(output)) != expected {
		t.Fatalf("unexpected frontend affected tests:\nwant: %s\ngot:\n%s", expected, output)
	}
}
