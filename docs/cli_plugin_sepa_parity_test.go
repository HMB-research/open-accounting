package docs

import (
	"os"
	"strings"
	"testing"
)

func TestCLIDocumentsOperatorPluginAndSEPAParity(t *testing.T) {
	requireDocSnippets(t, "CLI.md", []string{
		"--no-retention",
		"--min-size-bytes",
		"--allow-missing-checksum",
		"--allow-non-empty",
		"--skip-checksum",
		"--status-dir",
		"--backup-calendar",
		"--offsite-calendar",
		"--health-calendar",
		"--restore-calendar",
		"--query",
		"--body-file",
		"GET, POST, PUT, PATCH, and DELETE",
		"current tenant membership to be `owner` or `admin`",
		"--payment-info-id",
		"--creation-date-time",
		"--batch-booking=false",
		"--charge-bearer SLEV",
	})
}

func TestAPIDocumentsPluginRuntimeAndSEPAAdvancedFields(t *testing.T) {
	requireDocSnippets(t, "API.md", []string{
		`"payment_info_id": "PMT-INF-20260331"`,
		`"creation_date_time": "2026-03-31T12:00:00Z"`,
		`"batch_booking": false`,
		`"charge_bearer": "SLEV"`,
		"`charge_bearer` defaults to `SLEV`",
		"`SLEV` is the only accepted value",
		"Tenant runtime routes support `GET`, `POST`, `PUT`, `PATCH`, and `DELETE`",
		"returns the plugin runtime status code, response headers, and raw response body",
		"current tenant membership is `owner` or `admin`",
	})
}

func TestPluginGuideDocumentsRuntimeInvokeAndAdminMembership(t *testing.T) {
	requireDocSnippets(t, "PLUGINS.md", []string{
		"oa plugins runtime invoke --id <plugin-id> --method GET|POST|PUT|PATCH|DELETE --path <route>",
		"--query",
		"--body-file",
		"raw response body",
		"current tenant membership is `owner` or `admin`",
		"GET    /api/v1/tenants/:id/plugins/:pid/runtime/*",
		"PATCH  /api/v1/tenants/:id/plugins/:pid/runtime/*",
		"DELETE /api/v1/tenants/:id/plugins/:pid/runtime/*",
	})
}

func requireDocSnippets(t *testing.T, path string, snippets []string) {
	t.Helper()

	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	doc := string(payload)
	for _, snippet := range snippets {
		if !strings.Contains(doc, snippet) {
			t.Fatalf("%s missing snippet %q", path, snippet)
		}
	}
}
