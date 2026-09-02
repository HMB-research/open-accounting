package docs

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

func TestDocumentedBunScriptsExist(t *testing.T) {
	scripts := readPackageScripts(t, filepath.Join("..", "package.json"), filepath.Join("..", "frontend", "package.json"))
	checkedFiles := []string{
		filepath.Join("..", "README.md"),
		filepath.Join("..", "CONTRIBUTING.md"),
		"README.md",
		"ARCHITECTURE.md",
		"DEVELOPMENT_STATUS.md",
		"demo-e2e-testing.md",
		filepath.Join("..", "scripts", "test-demo-loop.sh"),
		filepath.Join("..", "scripts", "test-loop.sh"),
	}

	commandPattern := regexp.MustCompile(`\bbun\s+run\s+([A-Za-z0-9:_-]+)`)
	var missing []string
	checkedCommands := 0

	for _, file := range checkedFiles {
		payload, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		for lineNumber, line := range strings.Split(string(payload), "\n") {
			for _, match := range commandPattern.FindAllStringSubmatch(line, -1) {
				checkedCommands++
				command := match[1]
				if !scripts[command] {
					missing = append(missing, fmt.Sprintf("%s:%d bun run %s", file, lineNumber+1, command))
				}
			}
		}
	}

	if checkedCommands < 10 {
		t.Fatalf("expected to validate documented frontend scripts, found only %d commands", checkedCommands)
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Fatalf("documented Bun scripts missing from root or frontend package.json:\n%s", strings.Join(missing, "\n"))
	}
}

func readPackageScripts(t *testing.T, paths ...string) map[string]bool {
	t.Helper()

	scripts := make(map[string]bool)
	for _, path := range paths {
		payload, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}

		var packageJSON struct {
			Scripts map[string]string `json:"scripts"`
		}
		if err := json.Unmarshal(payload, &packageJSON); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}

		for name := range packageJSON.Scripts {
			scripts[name] = true
		}
	}
	return scripts
}
