package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

type apiRoute struct {
	Method string
	Path   string
}

func (r apiRoute) String() string {
	return r.Method + " " + r.Path
}

func TestCLIRouteCoverageAgainstAPISource(t *testing.T) {
	routes := collectAPIRoutesFromSource(t)
	require.Greater(t, len(routes), 100, "route source parser should see the API route table")
	knownCommands := collectKnownCLICommandPaths(t)
	documentedCommands := collectDocumentedCLICommandPaths(t, knownCommands, readCLIReference(t))

	var missing []string
	var undocumented []string
	seen := make(map[apiRoute]bool, len(routes))
	for _, route := range routes {
		require.Falsef(t, seen[route], "duplicate API route discovered: %s", route.String())
		seen[route] = true

		command, ok := cliCommandForRoute(route)
		if !ok {
			missing = append(missing, route.String())
			continue
		}
		if command == "" {
			continue
		}
		if !documentedCommands[command] {
			undocumented = append(undocumented, fmt.Sprintf("%s -> %s", route.String(), command))
		}
	}

	sort.Strings(missing)
	sort.Strings(undocumented)
	require.Empty(t, missing, "API routes without CLI coverage")
	require.Empty(t, undocumented, "CLI-covered API routes missing from docs/CLI.md")
}

func TestCLIReferenceExamplesUseKnownCommandPaths(t *testing.T) {
	knownCommands := collectKnownCLICommandPaths(t)
	documentedCommands := collectDocumentedCLICommandPaths(t, knownCommands, readCLIReference(t))
	require.Greater(t, len(documentedCommands), 100, "CLI guide should include broad command examples")
}

func TestKnownCLICommandPathsAreDocumented(t *testing.T) {
	knownCommands := collectKnownCLICommandPaths(t)
	documentedCommands := collectDocumentedCLICommandPaths(t, knownCommands, readCLIReference(t))

	var undocumented []string
	for command := range knownCommands {
		if !documentedCommands[command] {
			undocumented = append(undocumented, command)
		}
	}
	sort.Strings(undocumented)
	require.Empty(t, undocumented, "known CLI commands missing from docs/CLI.md")
}

func TestCLIUsageListsKnownCommandPaths(t *testing.T) {
	knownCommands := collectKnownCLICommandPaths(t)
	usageCommands := collectCLIUsageCommandPaths(t, knownCommands)

	var missing []string
	for command := range knownCommands {
		if !usageCommands[command] {
			missing = append(missing, command)
		}
	}
	sort.Strings(missing)
	require.Empty(t, missing, "known CLI commands missing from oa help")
}

func TestImplementedCLIFlagSetsAreCovered(t *testing.T) {
	knownCommands := collectKnownCLICommandPaths(t)
	documentedCommands := collectDocumentedCLICommandPaths(t, knownCommands, readCLIReference(t))
	usageCommands := collectCLIUsageCommandPaths(t, knownCommands)

	var unknown []string
	var undocumented []string
	var missingUsage []string
	for command := range collectImplementedCLIFlagSetCommands(t) {
		if !knownCommands[command] {
			unknown = append(unknown, command)
			continue
		}
		if !documentedCommands[command] {
			undocumented = append(undocumented, command)
		}
		if !usageCommands[command] {
			missingUsage = append(missingUsage, command)
		}
	}

	sort.Strings(unknown)
	sort.Strings(undocumented)
	sort.Strings(missingUsage)
	require.Empty(t, unknown, "implemented CLI flag sets missing from known command coverage")
	require.Empty(t, undocumented, "implemented CLI flag sets missing from docs/CLI.md")
	require.Empty(t, missingUsage, "implemented CLI flag sets missing from oa help")
}

func TestKnownCLICommandPathsHaveFunctionalTests(t *testing.T) {
	knownCommands := collectKnownCLICommandPaths(t)
	testedCommands := collectFunctionallyTestedCLICommandPaths(t, knownCommands)

	var untested []string
	for command := range knownCommands {
		if !testedCommands[command] {
			untested = append(untested, command)
		}
	}
	sort.Strings(untested)
	require.Empty(t, untested, "known CLI commands missing app.run coverage in cli_commands_test.go")
}

func collectAPIRoutesFromSource(t *testing.T) []apiRoute {
	t.Helper()

	sourcePath := filepath.Join("..", "api", "main.go")
	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, sourcePath, nil, 0)
	require.NoError(t, err)

	var routes []apiRoute
	var walkBlock func(block *ast.BlockStmt, prefixes []string)
	walkBlock = func(block *ast.BlockStmt, prefixes []string) {
		if block == nil {
			return
		}
		for _, stmt := range block.List {
			exprStmt, ok := stmt.(*ast.ExprStmt)
			if !ok {
				continue
			}
			call, ok := exprStmt.X.(*ast.CallExpr)
			if !ok {
				continue
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				continue
			}

			switch selector.Sel.Name {
			case "Route":
				require.GreaterOrEqual(t, len(call.Args), 2)
				prefix := mustStringArg(t, call.Args[0])
				funcLit, ok := call.Args[1].(*ast.FuncLit)
				require.True(t, ok, "expected func literal in Route call")
				walkBlock(funcLit.Body, append(prefixes, prefix))
			case "Group":
				require.GreaterOrEqual(t, len(call.Args), 1)
				funcLit, ok := call.Args[0].(*ast.FuncLit)
				require.True(t, ok, "expected func literal in Group call")
				walkBlock(funcLit.Body, prefixes)
			case "Get", "Post", "Put", "Delete", "Patch":
				require.GreaterOrEqual(t, len(call.Args), 1)
				routes = append(routes, apiRoute{
					Method: strings.ToUpper(selector.Sel.Name),
					Path:   joinRoutePath(prefixes, mustStringArg(t, call.Args[0])),
				})
			}
		}
	}

	for _, decl := range file.Decls {
		funcDecl, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		walkBlock(funcDecl.Body, nil)
	}

	sort.Slice(routes, func(i, j int) bool {
		return routes[i].String() < routes[j].String()
	})
	return routes
}

func collectImplementedCLIFlagSetCommands(t *testing.T) map[string]bool {
	t.Helper()

	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, "main.go", nil, 0)
	require.NoError(t, err)

	commands := map[string]bool{}
	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok || len(call.Args) == 0 {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || selector.Sel.Name != "NewFlagSet" {
			return true
		}
		lit, ok := call.Args[0].(*ast.BasicLit)
		if !ok {
			return true
		}
		command, err := strconv.Unquote(lit.Value)
		require.NoError(t, err)
		command = strings.TrimSpace(command)
		if command != "" {
			commands[command] = true
		}
		return true
	})
	require.Greater(t, len(commands), 100, "implementation parser should see broad CLI flag set coverage")
	return commands
}

func collectFunctionallyTestedCLICommandPaths(t *testing.T, knownCommands map[string]bool) map[string]bool {
	t.Helper()

	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, "cli_commands_test.go", nil, 0)
	require.NoError(t, err)

	tested := map[string]bool{}
	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok || len(call.Args) < 2 {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || selector.Sel.Name != "run" {
			return true
		}
		composite, ok := call.Args[1].(*ast.CompositeLit)
		if !ok {
			return true
		}

		tokens := make([]string, 0, len(composite.Elts))
		for _, elt := range composite.Elts {
			lit, ok := elt.(*ast.BasicLit)
			if !ok {
				continue
			}
			token, err := strconv.Unquote(lit.Value)
			require.NoError(t, err)
			tokens = append(tokens, token)
		}
		command, ok := resolveKnownCLICommandPath(knownCommands, strings.Join(tokens, " "))
		if ok {
			tested[command] = true
		}
		return true
	})
	require.Greater(t, len(tested), 100, "functional test parser should see broad CLI app.run coverage")
	return tested
}

func mustStringArg(t *testing.T, expr ast.Expr) string {
	t.Helper()

	lit, ok := expr.(*ast.BasicLit)
	require.True(t, ok, "expected string literal route path")
	value, err := strconv.Unquote(lit.Value)
	require.NoError(t, err)
	return value
}

func joinRoutePath(prefixes []string, path string) string {
	parts := make([]string, 0, len(prefixes)+1)
	parts = append(parts, prefixes...)
	parts = append(parts, path)

	var joined strings.Builder
	for _, part := range parts {
		clean := strings.Trim(part, "/")
		if clean == "" {
			continue
		}
		joined.WriteString("/")
		joined.WriteString(clean)
	}
	if joined.Len() == 0 {
		return "/"
	}
	return joined.String()
}

func readCLIReference(t *testing.T) string {
	t.Helper()

	payload, err := os.ReadFile(filepath.Join("..", "..", "docs", "CLI.md"))
	require.NoError(t, err)
	return string(payload)
}

func collectKnownCLICommandPaths(t *testing.T) map[string]bool {
	t.Helper()

	commands := map[string]bool{
		"help":                          true,
		"auth init":                     true,
		"auth logout":                   true,
		"ops backup create":             true,
		"ops backup health":             true,
		"ops backup offsite-sync":       true,
		"ops backup restore-drill":      true,
		"ops backup schedule-systemd":   true,
		"migration execute":             true,
		"payroll import-leave-balances": true,
	}
	for _, route := range collectAPIRoutesFromSource(t) {
		command, ok := cliCommandForRoute(route)
		if !ok || command == "" {
			continue
		}
		commands[command] = true
	}
	return commands
}

func collectCLIReferenceExampleCommands(reference string) []string {
	const marker = "go run ./cmd/oa "

	var commands []string
	for _, line := range strings.Split(reference, "\n") {
		_, rawCommand, ok := strings.Cut(line, marker)
		if !ok {
			continue
		}
		rawCommand = strings.TrimSpace(rawCommand)
		if rawCommand == "" {
			continue
		}
		commands = append(commands, rawCommand)
	}
	return commands
}

func collectDocumentedCLICommandPaths(t *testing.T, knownCommands map[string]bool, reference string) map[string]bool {
	t.Helper()

	examples := collectCLIReferenceExampleCommands(reference)
	var unknown []string
	documented := make(map[string]bool, len(examples))
	for _, example := range examples {
		command, ok := resolveKnownCLICommandPath(knownCommands, example)
		if !ok {
			unknown = append(unknown, example)
			continue
		}
		documented[command] = true
	}
	sort.Strings(unknown)
	require.Empty(t, unknown, "docs/CLI.md examples with unknown command paths")
	return documented
}

func collectCLIUsageCommandPaths(t *testing.T, knownCommands map[string]bool) map[string]bool {
	t.Helper()

	app, stdout, _ := newTestCLIApp()
	app.printUsage()

	knownList := make([]string, 0, len(knownCommands))
	for command := range knownCommands {
		knownList = append(knownList, command)
	}
	sort.Slice(knownList, func(i, j int) bool {
		if len(knownList[i]) == len(knownList[j]) {
			return knownList[i] < knownList[j]
		}
		return len(knownList[i]) > len(knownList[j])
	})

	usageCommands := map[string]bool{}
	var unknown []string
	inCommands := false
	for _, line := range strings.Split(stdout.String(), "\n") {
		switch line {
		case "Commands:":
			inCommands = true
			continue
		case "":
			inCommands = false
		}
		if !inCommands || !strings.HasPrefix(line, "  ") {
			continue
		}
		raw := strings.TrimSpace(line)
		command, ok := matchKnownCLIUsageCommand(raw, knownList)
		if !ok {
			unknown = append(unknown, raw)
			continue
		}
		usageCommands[command] = true
	}

	sort.Strings(unknown)
	require.Empty(t, unknown, "oa help entries that do not match known CLI commands")
	return usageCommands
}

func matchKnownCLIUsageCommand(line string, knownCommands []string) (string, bool) {
	for _, command := range knownCommands {
		if line == command {
			return command, true
		}
		if strings.HasPrefix(line, command) {
			remainder := strings.TrimPrefix(line, command)
			if len(remainder) > 0 && (remainder[0] == ' ' || remainder[0] == '\t') {
				return command, true
			}
		}
	}
	return "", false
}

func resolveKnownCLICommandPath(knownCommands map[string]bool, rawCommand string) (string, bool) {
	tokens := strings.Fields(rawCommand)
	cleanTokens := make([]string, 0, len(tokens))
	for _, token := range tokens {
		token = strings.TrimRight(token, `\`)
		token = strings.TrimSpace(token)
		if token == "" {
			continue
		}
		cleanTokens = append(cleanTokens, token)
	}

	for length := len(cleanTokens); length > 0; length-- {
		candidate := strings.Join(cleanTokens[:length], " ")
		if knownCommands[candidate] {
			return candidate, true
		}
	}
	return "", false
}

func cliCommandForRoute(route apiRoute) (string, bool) {
	switch route {
	case apiRoute{Method: "GET", Path: "/health"}:
		return "health", true
	case apiRoute{Method: "GET", Path: "/swagger/*"}:
		return "", true
	case apiRoute{Method: "GET", Path: "/api/demo/status"}:
		return "demo status", true
	case apiRoute{Method: "POST", Path: "/api/demo/reset"}:
		return "demo reset", true
	}

	apiPath, ok := strings.CutPrefix(route.Path, "/api/v1")
	if !ok {
		return "", false
	}

	switch {
	case strings.HasPrefix(apiPath, "/admin/"):
		return adminCLICommand(route.Method, strings.TrimPrefix(apiPath, "/admin"))
	case strings.HasPrefix(apiPath, "/tenants/{tenantID}/"):
		return tenantCLICommand(route.Method, strings.TrimPrefix(apiPath, "/tenants/{tenantID}"))
	}

	switch apiPath {
	case "/auth/register":
		return commandForMethod(route.Method, map[string]string{"POST": "auth register"})
	case "/auth/login":
		return commandForMethod(route.Method, map[string]string{"POST": "auth login"})
	case "/auth/refresh":
		return commandForMethod(route.Method, map[string]string{"POST": "auth refresh"})
	case "/auth/logout":
		return commandForMethod(route.Method, map[string]string{"POST": "auth logout"})
	case "/auth/password":
		return commandForMethod(route.Method, map[string]string{"PUT": "auth change-password"})
	case "/auth/password-reset/request":
		return commandForMethod(route.Method, map[string]string{"POST": "auth request-password-reset"})
	case "/auth/password-reset/confirm":
		return commandForMethod(route.Method, map[string]string{"POST": "auth reset-password"})
	case "/auth/sessions":
		return commandForMethod(route.Method, map[string]string{
			"GET":    "auth sessions",
			"DELETE": "auth revoke-all-sessions",
		})
	case "/auth/sessions/{sessionID}":
		return commandForMethod(route.Method, map[string]string{"DELETE": "auth revoke-session"})
	case "/auth/security-events":
		return commandForMethod(route.Method, map[string]string{"GET": "auth security-events"})
	case "/invitations/{token}":
		return commandForMethod(route.Method, map[string]string{"GET": "invitations get"})
	case "/invitations/accept":
		return commandForMethod(route.Method, map[string]string{"POST": "invitations accept"})
	case "/me":
		return commandForMethod(route.Method, map[string]string{"GET": "auth status"})
	case "/me/tenants":
		return commandForMethod(route.Method, map[string]string{"GET": "auth tenants"})
	case "/tenants":
		return commandForMethod(route.Method, map[string]string{"POST": "tenant create"})
	case "/tenants/{tenantID}":
		return commandForMethod(route.Method, map[string]string{
			"GET": "tenant get",
			"PUT": "tenant update",
		})
	default:
		return "", false
	}
}

func adminCLICommand(method, path string) (string, bool) {
	switch path {
	case "/plugin-registries":
		return commandForMethod(method, map[string]string{
			"GET":  "admin registries list",
			"POST": "admin registries create",
		})
	case "/plugin-registries/{id}":
		return commandForMethod(method, map[string]string{"DELETE": "admin registries delete"})
	case "/plugin-registries/{id}/sync":
		return commandForMethod(method, map[string]string{"POST": "admin registries sync"})
	case "/plugins":
		return commandForMethod(method, map[string]string{"GET": "admin plugins list"})
	case "/plugins/search":
		return commandForMethod(method, map[string]string{"GET": "admin plugins search"})
	case "/plugins/permissions":
		return commandForMethod(method, map[string]string{"GET": "admin plugins permissions"})
	case "/plugins/install":
		return commandForMethod(method, map[string]string{"POST": "admin plugins install"})
	case "/plugins/{id}":
		return commandForMethod(method, map[string]string{
			"GET":    "admin plugins get",
			"DELETE": "admin plugins uninstall",
		})
	case "/plugins/{id}/enable":
		return commandForMethod(method, map[string]string{"POST": "admin plugins enable"})
	case "/plugins/{id}/disable":
		return commandForMethod(method, map[string]string{"POST": "admin plugins disable"})
	default:
		return "", false
	}
}

func tenantCLICommand(method, path string) (string, bool) {
	switch path {
	case "/complete-onboarding":
		return commandForMethod(method, map[string]string{"POST": "tenant complete-onboarding"})
	case "/audit-events":
		return commandForMethod(method, map[string]string{"GET": "tenant audit-events"})
	case "/webhooks/events":
		return commandForMethod(method, map[string]string{"GET": "webhooks events"})
	case "/webhooks":
		return commandForMethod(method, map[string]string{
			"GET":  "webhooks list",
			"POST": "webhooks create",
		})
	case "/webhooks/{webhookID}":
		return commandForMethod(method, map[string]string{
			"GET":    "webhooks get",
			"PUT":    "webhooks update",
			"DELETE": "webhooks delete",
		})
	case "/webhooks/{webhookID}/deliveries":
		return commandForMethod(method, map[string]string{"GET": "webhooks deliveries"})
	case "/webhooks/{webhookID}/test":
		return commandForMethod(method, map[string]string{"POST": "webhooks test"})
	case "/expenses":
		return commandForMethod(method, map[string]string{
			"GET":  "expenses list",
			"POST": "expenses create",
		})
	case "/expenses/import":
		return commandForMethod(method, map[string]string{"POST": "expenses import"})
	case "/expenses/{expenseID}":
		return commandForMethod(method, map[string]string{"GET": "expenses get"})
	case "/expenses/{expenseID}/submit":
		return commandForMethod(method, map[string]string{"POST": "expenses submit"})
	case "/expenses/{expenseID}/approve":
		return commandForMethod(method, map[string]string{"POST": "expenses approve"})
	case "/expenses/{expenseID}/reject":
		return commandForMethod(method, map[string]string{"POST": "expenses reject"})
	case "/expenses/{expenseID}/post":
		return commandForMethod(method, map[string]string{"POST": "expenses post"})
	case "/period-close-events":
		return commandForMethod(method, map[string]string{"GET": "close events"})
	case "/period-close":
		return commandForMethod(method, map[string]string{"POST": "close period"})
	case "/period-reopen":
		return commandForMethod(method, map[string]string{"POST": "close reopen"})
	case "/year-end-close-status":
		return commandForMethod(method, map[string]string{"GET": "close year-end-status"})
	case "/year-end-close-pack":
		return commandForMethod(method, map[string]string{"GET": "close year-end-pack"})
	case "/year-end-close-audit-evidence":
		return commandForMethod(method, map[string]string{"GET": "close year-end-audit"})
	case "/year-end-close-audit-archive":
		return commandForMethod(method, map[string]string{"GET": "close year-end-archive"})
	case "/year-end-carry-forward":
		return commandForMethod(method, map[string]string{"POST": "close carry-forward"})
	case "/year-end-carry-forward/reverse":
		return commandForMethod(method, map[string]string{"POST": "close reverse-carry-forward"})
	case "/documents":
		return commandForMethod(method, map[string]string{
			"GET":  "documents list",
			"POST": "documents upload",
		})
	case "/documents/review-summary":
		return commandForMethod(method, map[string]string{"POST": "documents review-summary"})
	case "/documents/review-queue":
		return commandForMethod(method, map[string]string{"GET": "documents review-queue"})
	case "/documents/evidence-policy":
		return commandForMethod(method, map[string]string{"POST": "documents evidence-policy"})
	case "/documents/retention":
		return commandForMethod(method, map[string]string{"GET": "documents retention"})
	case "/documents/{documentID}/download":
		return commandForMethod(method, map[string]string{"GET": "documents download"})
	case "/documents/{documentID}/retention":
		return commandForMethod(method, map[string]string{"PATCH": "documents retention-set"})
	case "/documents/{documentID}/review":
		return commandForMethod(method, map[string]string{"POST": "documents review"})
	case "/documents/{documentID}/mark-reviewed":
		return commandForMethod(method, map[string]string{"POST": "documents mark-reviewed"})
	case "/documents/{documentID}":
		return commandForMethod(method, map[string]string{"DELETE": "documents delete"})
	case "/api-tokens":
		return commandForMethod(method, map[string]string{
			"GET":  "tokens list",
			"POST": "tokens create",
		})
	case "/api-tokens/{tokenID}":
		return commandForMethod(method, map[string]string{"DELETE": "tokens revoke"})
	case "/migration/provider-presets":
		return commandForMethod(method, map[string]string{"GET": "migration presets"})
	case "/migration/validate":
		return commandForMethod(method, map[string]string{"POST": "migration validate"})
	case "/migration/execution-plan":
		return commandForMethod(method, map[string]string{"POST": "migration plan"})
	case "/migration/execute":
		return commandForMethod(method, map[string]string{"POST": "migration execute"})
	case "/migration/execution-runs":
		return commandForMethod(method, map[string]string{"GET": "migration runs list"})
	case "/migration/execution-runs/{runID}":
		return commandForMethod(method, map[string]string{"GET": "migration runs get"})
	case "/migration/execution-runs/{runID}/events":
		return commandForMethod(method, map[string]string{"GET": "migration runs watch"})
	case "/accounts":
		return commandForMethod(method, map[string]string{
			"GET":  "accounts list",
			"POST": "accounts create",
		})
	case "/accounts/import":
		return commandForMethod(method, map[string]string{"POST": "accounts import"})
	case "/accounts/hierarchy":
		return commandForMethod(method, map[string]string{"GET": "accounts hierarchy"})
	case "/accounts/{accountID}":
		return commandForMethod(method, map[string]string{
			"GET":    "accounts get",
			"PUT":    "accounts update",
			"DELETE": "accounts delete",
		})
	case "/journal-entries":
		return commandForMethod(method, map[string]string{
			"GET":  "journal list",
			"POST": "journal create",
		})
	case "/journal-entries/import-opening-balances":
		return commandForMethod(method, map[string]string{"POST": "journal import-opening-balances"})
	case "/journal-entries/import":
		return commandForMethod(method, map[string]string{"POST": "journal import"})
	case "/journal-entries/{entryID}":
		return commandForMethod(method, map[string]string{"GET": "journal get"})
	case "/journal-entries/{entryID}/post":
		return commandForMethod(method, map[string]string{"POST": "journal post"})
	case "/journal-entries/{entryID}/void":
		return commandForMethod(method, map[string]string{"POST": "journal void"})
	case "/journal-entry-templates":
		return commandForMethod(method, map[string]string{
			"GET":  "journal templates list",
			"POST": "journal templates create",
		})
	case "/journal-entry-templates/generate-due":
		return commandForMethod(method, map[string]string{"POST": "journal templates generate-due"})
	case "/journal-entry-templates/{templateID}":
		return commandForMethod(method, map[string]string{"GET": "journal templates get"})
	case "/journal-entry-templates/{templateID}/generate":
		return commandForMethod(method, map[string]string{"POST": "journal templates generate"})
	case "/journal-entry-templates/{templateID}/apply":
		return commandForMethod(method, map[string]string{"POST": "journal templates apply"})
	case "/contacts":
		return commandForMethod(method, map[string]string{
			"GET":  "contacts list",
			"POST": "contacts create",
		})
	case "/contacts/import":
		return commandForMethod(method, map[string]string{"POST": "contacts import"})
	case "/contacts/{contactID}":
		return commandForMethod(method, map[string]string{
			"GET":    "contacts get",
			"PUT":    "contacts update",
			"DELETE": "contacts delete",
		})
	case "/invoices":
		return commandForMethod(method, map[string]string{
			"GET":  "invoices list",
			"POST": "invoices create",
		})
	case "/invoices/import":
		return commandForMethod(method, map[string]string{"POST": "invoices import"})
	case "/invoices/import-einvoice":
		return commandForMethod(method, map[string]string{"POST": "invoices import-einvoice"})
	case "/invoices/overdue":
		return commandForMethod(method, map[string]string{"GET": "reminders overdue"})
	case "/invoices/reminders":
		return commandForMethod(method, map[string]string{"POST": "reminders send"})
	case "/invoices/reminders/bulk":
		return commandForMethod(method, map[string]string{"POST": "reminders send-bulk"})
	case "/invoices/overdue-with-interest":
		return commandForMethod(method, map[string]string{"GET": "interest overdue"})
	case "/invoices/{invoiceID}":
		return commandForMethod(method, map[string]string{"GET": "invoices get"})
	case "/invoices/{invoiceID}/pdf":
		return commandForMethod(method, map[string]string{"GET": "invoices pdf"})
	case "/invoices/{invoiceID}/send":
		return commandForMethod(method, map[string]string{"POST": "invoices send"})
	case "/invoices/{invoiceID}/void":
		return commandForMethod(method, map[string]string{"POST": "invoices void"})
	case "/invoices/{invoiceID}/reminders":
		return commandForMethod(method, map[string]string{"GET": "reminders history"})
	case "/invoices/{invoiceID}/interest":
		return commandForMethod(method, map[string]string{"GET": "interest invoice"})
	case "/invoices/{invoiceID}/interest/history":
		return commandForMethod(method, map[string]string{"GET": "interest history"})
	case "/invoices/{invoiceID}/email":
		return commandForMethod(method, map[string]string{"POST": "email invoice"})
	case "/quotes":
		return commandForMethod(method, map[string]string{
			"GET":  "quotes list",
			"POST": "quotes create",
		})
	case "/quotes/import":
		return commandForMethod(method, map[string]string{"POST": "quotes import"})
	case "/quotes/{quoteID}":
		return commandForMethod(method, map[string]string{
			"GET":    "quotes get",
			"PUT":    "quotes update",
			"DELETE": "quotes delete",
		})
	case "/quotes/{quoteID}/pdf":
		return commandForMethod(method, map[string]string{"GET": "quotes pdf"})
	case "/quotes/{quoteID}/email":
		return commandForMethod(method, map[string]string{"POST": "email quote"})
	case "/quotes/{quoteID}/send":
		return commandForMethod(method, map[string]string{"POST": "quotes send"})
	case "/quotes/{quoteID}/accept":
		return commandForMethod(method, map[string]string{"POST": "quotes accept"})
	case "/quotes/{quoteID}/reject":
		return commandForMethod(method, map[string]string{"POST": "quotes reject"})
	case "/quotes/{quoteID}/convert-to-invoice":
		return commandForMethod(method, map[string]string{"POST": "quotes convert-to-invoice"})
	case "/orders":
		return commandForMethod(method, map[string]string{
			"GET":  "orders list",
			"POST": "orders create",
		})
	case "/orders/import":
		return commandForMethod(method, map[string]string{"POST": "orders import"})
	case "/orders/{orderID}":
		return commandForMethod(method, map[string]string{
			"GET":    "orders get",
			"PUT":    "orders update",
			"DELETE": "orders delete",
		})
	case "/orders/{orderID}/pdf":
		return commandForMethod(method, map[string]string{"GET": "orders pdf"})
	case "/orders/{orderID}/email":
		return commandForMethod(method, map[string]string{"POST": "email order"})
	case "/orders/{orderID}/stock-check":
		return commandForMethod(method, map[string]string{"GET": "orders stock-check"})
	case "/orders/{orderID}/stock-reservations":
		return commandForMethod(method, map[string]string{"GET": "orders stock-reservations"})
	case "/orders/{orderID}/pick-list":
		return commandForMethod(method, map[string]string{"GET": "orders pick-list"})
	case "/orders/{orderID}/reserve-stock":
		return commandForMethod(method, map[string]string{"POST": "orders reserve-stock"})
	case "/orders/{orderID}/release-stock":
		return commandForMethod(method, map[string]string{"POST": "orders release-stock"})
	case "/orders/{orderID}/confirm":
		return commandForMethod(method, map[string]string{"POST": "orders confirm"})
	case "/orders/{orderID}/process":
		return commandForMethod(method, map[string]string{"POST": "orders process"})
	case "/orders/{orderID}/ship":
		return commandForMethod(method, map[string]string{"POST": "orders ship"})
	case "/orders/{orderID}/deliver":
		return commandForMethod(method, map[string]string{"POST": "orders deliver"})
	case "/orders/{orderID}/cancel":
		return commandForMethod(method, map[string]string{"POST": "orders cancel"})
	case "/orders/{orderID}/convert-to-invoice":
		return commandForMethod(method, map[string]string{"POST": "orders convert-to-invoice"})
	case "/asset-categories":
		return commandForMethod(method, map[string]string{
			"GET":  "assets categories list",
			"POST": "assets categories create",
		})
	case "/asset-categories/{categoryID}":
		return commandForMethod(method, map[string]string{
			"GET":    "assets categories get",
			"DELETE": "assets categories delete",
		})
	case "/assets":
		return commandForMethod(method, map[string]string{
			"GET":  "assets list",
			"POST": "assets create",
		})
	case "/assets/import":
		return commandForMethod(method, map[string]string{"POST": "assets import"})
	case "/assets/{assetID}":
		return commandForMethod(method, map[string]string{
			"GET":    "assets get",
			"PUT":    "assets update",
			"DELETE": "assets delete",
		})
	case "/assets/{assetID}/activate":
		return commandForMethod(method, map[string]string{"POST": "assets activate"})
	case "/assets/{assetID}/dispose":
		return commandForMethod(method, map[string]string{"POST": "assets dispose"})
	case "/assets/{assetID}/depreciation":
		return commandForMethod(method, map[string]string{
			"GET":  "assets depreciation",
			"POST": "assets depreciate",
		})
	case "/product-categories":
		return commandForMethod(method, map[string]string{
			"GET":  "inventory categories list",
			"POST": "inventory categories create",
		})
	case "/product-categories/import":
		return commandForMethod(method, map[string]string{"POST": "inventory categories import"})
	case "/product-categories/{categoryID}":
		return commandForMethod(method, map[string]string{
			"GET":    "inventory categories get",
			"DELETE": "inventory categories delete",
		})
	case "/products":
		return commandForMethod(method, map[string]string{
			"GET":  "inventory products list",
			"POST": "inventory products create",
		})
	case "/products/import":
		return commandForMethod(method, map[string]string{"POST": "inventory products import"})
	case "/products/{productID}":
		return commandForMethod(method, map[string]string{
			"GET":    "inventory products get",
			"PUT":    "inventory products update",
			"DELETE": "inventory products delete",
		})
	case "/products/{productID}/stock-levels":
		return commandForMethod(method, map[string]string{"GET": "inventory products stock-levels"})
	case "/products/{productID}/movements":
		return commandForMethod(method, map[string]string{"GET": "inventory products movements"})
	case "/inventory/valuation":
		return commandForMethod(method, map[string]string{"GET": "inventory valuation"})
	case "/inventory/subledger-reconciliation":
		return commandForMethod(method, map[string]string{"GET": "inventory subledger-reconciliation"})
	case "/inventory/lots":
		return commandForMethod(method, map[string]string{"GET": "inventory lots"})
	case "/warehouses":
		return commandForMethod(method, map[string]string{
			"GET":  "inventory warehouses list",
			"POST": "inventory warehouses create",
		})
	case "/warehouses/import":
		return commandForMethod(method, map[string]string{"POST": "inventory warehouses import"})
	case "/warehouses/{warehouseID}":
		return commandForMethod(method, map[string]string{
			"GET":    "inventory warehouses get",
			"PUT":    "inventory warehouses update",
			"DELETE": "inventory warehouses delete",
		})
	case "/inventory/adjust":
		return commandForMethod(method, map[string]string{"POST": "inventory adjust"})
	case "/inventory/stock-import":
		return commandForMethod(method, map[string]string{"POST": "inventory stock import"})
	case "/inventory/issue":
		return commandForMethod(method, map[string]string{"POST": "inventory issue"})
	case "/inventory/transfer":
		return commandForMethod(method, map[string]string{"POST": "inventory transfer"})
	case "/inventory/reserve":
		return commandForMethod(method, map[string]string{"POST": "inventory reserve"})
	case "/inventory/release":
		return commandForMethod(method, map[string]string{"POST": "inventory release"})
	case "/payments":
		return commandForMethod(method, map[string]string{
			"GET":  "payments list",
			"POST": "payments create",
		})
	case "/payments/import":
		return commandForMethod(method, map[string]string{"POST": "payments import"})
	case "/payments/sepa-export":
		return commandForMethod(method, map[string]string{"POST": "payments sepa-export"})
	case "/payments/unallocated":
		return commandForMethod(method, map[string]string{"GET": "payments unallocated"})
	case "/payments/{paymentID}":
		return commandForMethod(method, map[string]string{"GET": "payments get"})
	case "/payments/{paymentID}/allocate":
		return commandForMethod(method, map[string]string{"POST": "payments allocate"})
	case "/payments/{paymentID}/reverse":
		return commandForMethod(method, map[string]string{"POST": "payments reverse"})
	case "/payments/{paymentID}/email-receipt":
		return commandForMethod(method, map[string]string{"POST": "email payment-receipt"})
	case "/reports/trial-balance":
		return commandForMethod(method, map[string]string{"GET": "reports trial-balance"})
	case "/reports/account-balance/{accountID}":
		return commandForMethod(method, map[string]string{"GET": "reports account-balance"})
	case "/reports/balance-sheet":
		return commandForMethod(method, map[string]string{"GET": "reports balance-sheet"})
	case "/reports/income-statement":
		return commandForMethod(method, map[string]string{"GET": "reports income-statement"})
	case "/reports/consolidated":
		return commandForMethod(method, map[string]string{"GET": "reports consolidated"})
	case "/reports/annual":
		return commandForMethod(method, map[string]string{"GET": "reports annual"})
	case "/reports/cash-flow":
		return commandForMethod(method, map[string]string{"GET": "reports cash-flow"})
	case "/reports/cash-flow/mapping":
		return commandForMethod(method, map[string]string{
			"GET": "reports cash-flow-mapping get",
			"PUT": "reports cash-flow-mapping update",
		})
	case "/reports/balance-confirmations":
		return commandForMethod(method, map[string]string{"GET": "reports balance-confirmations"})
	case "/reports/balance-confirmations/{contactID}":
		return commandForMethod(method, map[string]string{"GET": "reports balance-confirmation"})
	case "/reports/contact-statements/{contactID}":
		return commandForMethod(method, map[string]string{"GET": "reports contact-statement"})
	case "/reports/sales-margin":
		return commandForMethod(method, map[string]string{"GET": "reports sales-margin"})
	case "/reports/customer-profitability":
		return commandForMethod(method, map[string]string{"GET": "reports customer-profitability"})
	case "/reports/budget-vs-actual":
		return commandForMethod(method, map[string]string{"GET": "reports budget-vs-actual"})
	case "/reports/aging/receivables":
		return commandForMethod(method, map[string]string{"GET": "reports aging"})
	case "/reports/aging/payables":
		return commandForMethod(method, map[string]string{"GET": "reports aging"})
	case "/cost-centers":
		return commandForMethod(method, map[string]string{
			"GET":  "cost-centers list",
			"POST": "cost-centers create",
		})
	case "/cost-centers/import":
		return commandForMethod(method, map[string]string{"POST": "cost-centers import"})
	case "/cost-centers/report":
		return commandForMethod(method, map[string]string{"GET": "cost-centers report"})
	case "/cost-centers/allocations":
		return commandForMethod(method, map[string]string{
			"GET":  "cost-centers allocations list",
			"POST": "cost-centers allocations create",
		})
	case "/cost-centers/allocations/import":
		return commandForMethod(method, map[string]string{"POST": "cost-centers allocations import"})
	case "/cost-centers/{costCenterID}":
		return commandForMethod(method, map[string]string{
			"GET":    "cost-centers get",
			"PUT":    "cost-centers update",
			"DELETE": "cost-centers delete",
		})
	case "/analytics/dashboard":
		return commandForMethod(method, map[string]string{"GET": "analytics dashboard"})
	case "/analytics/revenue-expense":
		return commandForMethod(method, map[string]string{"GET": "analytics revenue-expense"})
	case "/analytics/cash-flow":
		return commandForMethod(method, map[string]string{"GET": "analytics cash-flow"})
	case "/analytics/activity":
		return commandForMethod(method, map[string]string{"GET": "analytics activity"})
	case "/recurring-invoices":
		return commandForMethod(method, map[string]string{
			"GET":  "recurring-invoices list",
			"POST": "recurring-invoices create",
		})
	case "/recurring-invoices/import":
		return commandForMethod(method, map[string]string{"POST": "recurring-invoices import"})
	case "/recurring-invoices/from-invoice/{invoiceID}":
		return commandForMethod(method, map[string]string{"POST": "recurring-invoices from-invoice"})
	case "/recurring-invoices/generate-due":
		return commandForMethod(method, map[string]string{"POST": "recurring-invoices generate-due"})
	case "/recurring-invoices/{recurringID}":
		return commandForMethod(method, map[string]string{
			"GET":    "recurring-invoices get",
			"PUT":    "recurring-invoices update",
			"DELETE": "recurring-invoices delete",
		})
	case "/recurring-invoices/{recurringID}/pause":
		return commandForMethod(method, map[string]string{"POST": "recurring-invoices pause"})
	case "/recurring-invoices/{recurringID}/resume":
		return commandForMethod(method, map[string]string{"POST": "recurring-invoices resume"})
	case "/recurring-invoices/{recurringID}/generate":
		return commandForMethod(method, map[string]string{"POST": "recurring-invoices generate"})
	case "/settings/smtp":
		return commandForMethod(method, map[string]string{
			"GET": "email smtp get",
			"PUT": "email smtp update",
		})
	case "/settings/smtp/test":
		return commandForMethod(method, map[string]string{"POST": "email smtp test"})
	case "/email-templates":
		return commandForMethod(method, map[string]string{"GET": "email templates list"})
	case "/email-templates/{templateType}":
		return commandForMethod(method, map[string]string{"PUT": "email templates update"})
	case "/email-log":
		return commandForMethod(method, map[string]string{"GET": "email log"})
	case "/reminder-rules":
		return commandForMethod(method, map[string]string{
			"GET":  "reminders rules list",
			"POST": "reminders rules create",
		})
	case "/reminder-rules/trigger":
		return commandForMethod(method, map[string]string{"POST": "reminders rules trigger"})
	case "/reminder-rules/{ruleID}":
		return commandForMethod(method, map[string]string{
			"GET":    "reminders rules get",
			"PUT":    "reminders rules update",
			"DELETE": "reminders rules delete",
		})
	case "/settings/interest":
		return commandForMethod(method, map[string]string{
			"GET": "interest settings get",
			"PUT": "interest settings update",
		})
	case "/bank-accounts":
		return commandForMethod(method, map[string]string{
			"GET":  "banking accounts list",
			"POST": "banking accounts create",
		})
	case "/bank-accounts/import":
		return commandForMethod(method, map[string]string{"POST": "banking accounts import"})
	case "/bank-accounts/{accountID}":
		return commandForMethod(method, map[string]string{
			"GET":    "banking accounts get",
			"PUT":    "banking accounts update",
			"DELETE": "banking accounts delete",
		})
	case "/bank-match-rules":
		return commandForMethod(method, map[string]string{
			"GET":  "banking match-rules list",
			"POST": "banking match-rules create",
		})
	case "/bank-match-rules/{ruleID}":
		return commandForMethod(method, map[string]string{
			"GET":    "banking match-rules get",
			"PUT":    "banking match-rules update",
			"DELETE": "banking match-rules delete",
		})
	case "/bank-accounts/{accountID}/transactions":
		return commandForMethod(method, map[string]string{"GET": "banking transactions list"})
	case "/bank-accounts/{accountID}/import":
		return commandForMethod(method, map[string]string{"POST": "banking transactions import"})
	case "/bank-accounts/{accountID}/import-history":
		return commandForMethod(method, map[string]string{"GET": "banking transactions import-history"})
	case "/bank-transactions/{transactionID}":
		return commandForMethod(method, map[string]string{"GET": "banking transactions get"})
	case "/bank-transactions/{transactionID}/suggestions":
		return commandForMethod(method, map[string]string{"GET": "banking transactions suggestions"})
	case "/bank-transactions/{transactionID}/match":
		return commandForMethod(method, map[string]string{"POST": "banking transactions match"})
	case "/bank-transactions/{transactionID}/unmatch":
		return commandForMethod(method, map[string]string{"POST": "banking transactions unmatch"})
	case "/bank-transactions/{transactionID}/review":
		return commandForMethod(method, map[string]string{"POST": "banking transactions review"})
	case "/bank-transactions/{transactionID}/create-payment":
		return commandForMethod(method, map[string]string{"POST": "banking transactions create-payment"})
	case "/bank-accounts/{accountID}/reconciliations":
		return commandForMethod(method, map[string]string{"GET": "banking reconciliations list"})
	case "/bank-accounts/{accountID}/reconciliation":
		return commandForMethod(method, map[string]string{"POST": "banking reconciliations create"})
	case "/reconciliations/{reconciliationID}":
		return commandForMethod(method, map[string]string{"GET": "banking reconciliations get"})
	case "/reconciliations/{reconciliationID}/complete":
		return commandForMethod(method, map[string]string{"POST": "banking reconciliations complete"})
	case "/bank-accounts/{accountID}/auto-match":
		return commandForMethod(method, map[string]string{"POST": "banking transactions auto-match"})
	case "/tax/kmd":
		return commandForMethod(method, map[string]string{
			"GET":  "tax kmd list",
			"POST": "tax kmd generate",
		})
	case "/tax/kmd/import-history":
		return commandForMethod(method, map[string]string{"POST": "tax kmd import-history"})
	case "/tax/kmd/{year}/{month}/inf":
		return commandForMethod(method, map[string]string{"GET": "tax kmd inf"})
	case "/tax/kmd/{year}/{month}/xml":
		return commandForMethod(method, map[string]string{"GET": "tax kmd export-xml"})
	case "/tax/eu-vat/oss":
		return commandForMethod(method, map[string]string{"GET": "tax oss report"})
	case "/employees":
		return commandForMethod(method, map[string]string{
			"GET":  "employees list",
			"POST": "employees create",
		})
	case "/employees/import":
		return commandForMethod(method, map[string]string{"POST": "employees import"})
	case "/employees/{employeeID}":
		return commandForMethod(method, map[string]string{
			"GET": "employees get",
			"PUT": "employees update",
		})
	case "/employees/{employeeID}/salary":
		return commandForMethod(method, map[string]string{"POST": "employees set-salary"})
	case "/employees/{employeeID}/salary-components":
		return commandForMethod(method, map[string]string{
			"GET":  "employees salary-components",
			"POST": "employees add-salary-component",
		})
	case "/payroll-runs":
		return commandForMethod(method, map[string]string{
			"GET":  "payroll runs list",
			"POST": "payroll runs create",
		})
	case "/payroll-runs/import-history":
		return commandForMethod(method, map[string]string{"POST": "payroll import-history"})
	case "/payroll-runs/{runID}":
		return commandForMethod(method, map[string]string{"GET": "payroll runs get"})
	case "/payroll-runs/{runID}/payment-date":
		return commandForMethod(method, map[string]string{"PATCH": "payroll runs set-payment-date"})
	case "/payroll-runs/{runID}/calculate":
		return commandForMethod(method, map[string]string{"POST": "payroll runs calculate"})
	case "/payroll-runs/{runID}/process":
		return commandForMethod(method, map[string]string{"POST": "payroll runs process"})
	case "/payroll-runs/{runID}/approve":
		return commandForMethod(method, map[string]string{"POST": "payroll runs approve"})
	case "/payroll-runs/{runID}/payslips":
		return commandForMethod(method, map[string]string{"GET": "payroll runs payslips"})
	case "/payroll-runs/{runID}/payslips/{payslipID}/pdf":
		return commandForMethod(method, map[string]string{"GET": "payroll runs payslip-pdf"})
	case "/payroll-runs/{runID}/tsd":
		return commandForMethod(method, map[string]string{"POST": "tsd generate"})
	case "/payroll/tax-preview":
		return commandForMethod(method, map[string]string{"POST": "payroll tax-preview"})
	case "/absence-types":
		return commandForMethod(method, map[string]string{"GET": "leave absence-types list"})
	case "/absence-types/{typeID}":
		return commandForMethod(method, map[string]string{"GET": "leave absence-types get"})
	case "/employees/{employeeID}/leave-balances":
		return commandForMethod(method, map[string]string{"GET": "leave balances list"})
	case "/employees/{employeeID}/leave-balances/{year}":
		return commandForMethod(method, map[string]string{"GET": "leave balances by-year"})
	case "/employees/{employeeID}/leave-balances/{year}/{typeID}":
		return commandForMethod(method, map[string]string{"PUT": "leave balances update"})
	case "/employees/{employeeID}/leave-balances/{year}/initialize":
		return commandForMethod(method, map[string]string{"POST": "leave balances initialize"})
	case "/leave-balances/import":
		return commandForMethod(method, map[string]string{"POST": "leave balances import"})
	case "/leave-records":
		return commandForMethod(method, map[string]string{
			"GET":  "leave records list",
			"POST": "leave records create",
		})
	case "/leave-records/{recordID}":
		return commandForMethod(method, map[string]string{"GET": "leave records get"})
	case "/leave-records/{recordID}/approve":
		return commandForMethod(method, map[string]string{"POST": "leave records approve"})
	case "/leave-records/{recordID}/reject":
		return commandForMethod(method, map[string]string{"POST": "leave records reject"})
	case "/leave-records/{recordID}/cancel":
		return commandForMethod(method, map[string]string{"POST": "leave records cancel"})
	case "/tsd":
		return commandForMethod(method, map[string]string{"GET": "tsd list"})
	case "/tsd/{year}/{month}":
		return commandForMethod(method, map[string]string{"GET": "tsd get"})
	case "/tsd/{year}/{month}/xml":
		return commandForMethod(method, map[string]string{"GET": "tsd export-xml"})
	case "/tsd/{year}/{month}/csv":
		return commandForMethod(method, map[string]string{"GET": "tsd export-csv"})
	case "/tsd/import-history":
		return commandForMethod(method, map[string]string{"POST": "tsd import-history"})
	case "/tsd/{year}/{month}/submit":
		return commandForMethod(method, map[string]string{"POST": "tsd mark-submitted"})
	case "/tsd/{year}/{month}/accept":
		return commandForMethod(method, map[string]string{"POST": "tsd mark-accepted"})
	case "/tsd/{year}/{month}/reject":
		return commandForMethod(method, map[string]string{"POST": "tsd mark-rejected"})
	case "/users":
		return commandForMethod(method, map[string]string{"GET": "users list"})
	case "/users/{userID}":
		return commandForMethod(method, map[string]string{"DELETE": "users remove"})
	case "/users/{userID}/role":
		return commandForMethod(method, map[string]string{"PUT": "users update-role"})
	case "/users/{userID}/status":
		return commandForMethod(method, map[string]string{"PUT": "users set-status"})
	case "/users/{userID}/sessions":
		return commandForMethod(method, map[string]string{
			"GET":    "users sessions",
			"DELETE": "users revoke-all-sessions",
		})
	case "/users/{userID}/sessions/{sessionID}":
		return commandForMethod(method, map[string]string{"DELETE": "users revoke-session"})
	case "/users/{userID}/api-tokens":
		return commandForMethod(method, map[string]string{"GET": "users api-tokens"})
	case "/users/{userID}/api-tokens/{tokenID}":
		return commandForMethod(method, map[string]string{"DELETE": "users revoke-api-token"})
	case "/users/{userID}/security-events":
		return commandForMethod(method, map[string]string{"GET": "users security-events"})
	case "/invitations":
		return commandForMethod(method, map[string]string{
			"GET":  "invitations list",
			"POST": "invitations create",
		})
	case "/invitations/{invitationID}":
		return commandForMethod(method, map[string]string{"DELETE": "invitations revoke"})
	case "/plugins":
		return commandForMethod(method, map[string]string{"GET": "plugins list"})
	case "/plugins/{pluginID}/enable":
		return commandForMethod(method, map[string]string{"POST": "plugins enable"})
	case "/plugins/{pluginID}/disable":
		return commandForMethod(method, map[string]string{"POST": "plugins disable"})
	case "/plugins/{pluginID}/settings":
		return commandForMethod(method, map[string]string{
			"GET": "plugins settings get",
			"PUT": "plugins settings update",
		})
	default:
		return "", false
	}
}

func commandForMethod(method string, commands map[string]string) (string, bool) {
	command, ok := commands[method]
	return command, ok
}
