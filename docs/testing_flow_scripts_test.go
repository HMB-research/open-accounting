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
