package docs

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

var directPersistenceCallPattern = regexp.MustCompile(`\b(?:pool|db|conn|tx|gormDB)\s*\.\s*(?:Query|QueryRow|Exec|Raw)\s*\(`)

func TestRuntimePersistenceStaysBehindRepositories(t *testing.T) {
	var violations []string
	for _, root := range []string{"cmd", "internal"} {
		rootPath := filepath.Join("..", root)
		if err := filepath.WalkDir(rootPath, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				if shouldSkipPersistenceBoundaryDir(path) {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") || shouldSkipPersistenceBoundaryFile(path) {
				return nil
			}
			matches, err := directPersistenceCalls(path)
			if err != nil {
				return err
			}
			violations = append(violations, matches...)
			return nil
		}); err != nil {
			t.Fatalf("scan %s persistence boundaries: %v", root, err)
		}
	}

	sort.Strings(violations)
	if len(violations) > 0 {
		t.Fatalf("direct persistence calls must stay behind repository/database/migration boundaries:\n%s", strings.Join(violations, "\n"))
	}
}

func TestRepositoryRawSQLExceptionsAreExplicit(t *testing.T) {
	var violations []string
	rootPath := filepath.Join("..", "internal")
	if err := filepath.WalkDir(rootPath, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if shouldSkipPersistenceBoundaryDir(path) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") || !shouldSkipPersistenceBoundaryFile(path) {
			return nil
		}
		matches, err := directPersistenceCalls(path)
		if err != nil {
			return err
		}
		for _, match := range matches {
			if !allowedRawPersistenceException(match) {
				violations = append(violations, match)
			}
		}
		return nil
	}); err != nil {
		t.Fatalf("scan repository raw SQL exceptions: %v", err)
	}

	sort.Strings(violations)
	if len(violations) > 0 {
		t.Fatalf("repository raw SQL must be an explicit schema lifecycle or demo reset exception:\n%s", strings.Join(violations, "\n"))
	}
}

func shouldSkipPersistenceBoundaryDir(path string) bool {
	normalized := filepath.ToSlash(path)
	return strings.Contains(normalized, "/internal/database") ||
		strings.Contains(normalized, "/internal/models") ||
		strings.Contains(normalized, "/internal/testutil") ||
		strings.Contains(normalized, "/cmd/migrate")
}

func shouldSkipPersistenceBoundaryFile(path string) bool {
	base := filepath.Base(path)
	return strings.Contains(base, "repository")
}

func allowedRawPersistenceException(match string) bool {
	normalized := filepath.ToSlash(match)
	allowedFragments := []string{
		"internal/demo/reset_repository.go:",
		"internal/tenant/repository_gorm.go:",
	}
	if !strings.Contains(normalized, allowedFragments[0]) && !strings.Contains(normalized, allowedFragments[1]) {
		return false
	}
	allowedStatements := []string{
		"SELECT pg_advisory_lock",
		"SELECT pg_advisory_unlock",
		"seedSQL",
		"DROP SCHEMA IF EXISTS",
		"SELECT create_tenant_schema",
		"SELECT create_default_chart_of_accounts",
		"SELECT drop_tenant_schema",
	}
	for _, allowed := range allowedStatements {
		if strings.Contains(normalized, allowed) {
			return true
		}
	}
	return false
}

func directPersistenceCalls(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = file.Close()
	}()

	var matches []string
	scanner := bufio.NewScanner(file)
	for lineNumber := 1; scanner.Scan(); lineNumber++ {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}
		if directPersistenceCallPattern.MatchString(line) {
			matches = append(matches, filepath.ToSlash(path)+":"+strconv.Itoa(lineNumber)+": "+line)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return matches, nil
}
