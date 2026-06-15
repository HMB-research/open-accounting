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

func TestDocumentedFrontendScriptsExist(t *testing.T) {
	scripts := readFrontendScripts(t)
	checkedFiles := []string{
		filepath.Join("..", "README.md"),
		filepath.Join("..", "CONTRIBUTING.md"),
		"DEVELOPMENT_STATUS.md",
		"demo-e2e-testing.md",
		"RALPH_HOOK_SETUP.md",
		"ralph-wiggum-loop.md",
		filepath.Join("..", "scripts", "ralph-loop-views.sh"),
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
		t.Fatalf("documented frontend scripts missing from frontend/package.json:\n%s", strings.Join(missing, "\n"))
	}
}

func readFrontendScripts(t *testing.T) map[string]bool {
	t.Helper()

	payload, err := os.ReadFile(filepath.Join("..", "frontend", "package.json"))
	if err != nil {
		t.Fatalf("read frontend package.json: %v", err)
	}

	var packageJSON struct {
		Scripts map[string]string `json:"scripts"`
	}
	if err := json.Unmarshal(payload, &packageJSON); err != nil {
		t.Fatalf("parse frontend package.json: %v", err)
	}

	scripts := make(map[string]bool, len(packageJSON.Scripts))
	for name := range packageJSON.Scripts {
		scripts[name] = true
	}
	return scripts
}
