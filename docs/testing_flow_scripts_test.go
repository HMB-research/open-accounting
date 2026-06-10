package docs

import (
	"os/exec"
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
