package docs

import (
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var markdownLinkPattern = regexp.MustCompile(`!?\[[^\]\n]*\]\(([^)\s]+)(?:\s+"[^"]*")?\)`)

func TestLocalMarkdownLinksResolve(t *testing.T) {
	sources := []string{filepath.Join("..", "README.md")}
	docFiles, err := filepath.Glob("*.md")
	if err != nil {
		t.Fatalf("glob docs markdown files: %v", err)
	}
	sources = append(sources, docFiles...)

	for _, source := range sources {
		payload, err := os.ReadFile(source)
		if err != nil {
			t.Fatalf("read %s: %v", source, err)
		}
		sourceDir := filepath.Dir(source)
		for _, match := range markdownLinkPattern.FindAllStringSubmatch(string(payload), -1) {
			target := strings.TrimSpace(match[1])
			if shouldSkipMarkdownTarget(target) {
				continue
			}
			if hash := strings.Index(target, "#"); hash >= 0 {
				target = target[:hash]
			}
			if target == "" || shouldSkipMarkdownTarget(target) {
				continue
			}
			unescaped, err := url.PathUnescape(target)
			if err != nil {
				t.Fatalf("%s has invalid escaped markdown link %q: %v", source, target, err)
			}
			candidate := filepath.Clean(filepath.Join(sourceDir, unescaped))
			if _, err := os.Stat(candidate); err != nil {
				t.Fatalf("%s has unresolved local markdown link %q resolved to %s: %v", source, match[1], candidate, err)
			}
		}
	}
}

func shouldSkipMarkdownTarget(target string) bool {
	return target == "" ||
		strings.HasPrefix(target, "#") ||
		strings.HasPrefix(target, "/") ||
		strings.Contains(target, "://") ||
		strings.HasPrefix(target, "mailto:")
}
