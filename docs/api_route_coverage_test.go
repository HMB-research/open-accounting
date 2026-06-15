package docs

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

type documentedRoute struct {
	Method string
	Path   string
}

func (r documentedRoute) String() string {
	return r.Method + " " + r.Path
}

func TestAPIReferenceDocumentsAllSourceRoutes(t *testing.T) {
	routes := collectSourceRoutes(t)
	if len(routes) < 100 {
		t.Fatalf("route source parser should see the API route table, got %d routes", len(routes))
	}

	apiReferenceRoutes := collectAPIReferenceRoutes(t)
	var missing []string
	seen := make(map[documentedRoute]bool, len(routes))
	for _, route := range routes {
		if seen[route] {
			t.Fatalf("duplicate API route discovered: %s", route.String())
		}
		seen[route] = true

		expected, ok := apiReferenceRoute(route)
		if !ok {
			continue
		}
		if !apiReferenceRoutes[expected] {
			missing = append(missing, expected.String())
		}
	}

	sort.Strings(missing)
	if len(missing) > 0 {
		t.Fatalf("docs/API.md is missing API route references:\n%s", strings.Join(missing, "\n"))
	}
}

func TestSwaggerDocumentsAllSourceRoutes(t *testing.T) {
	routes := collectSourceRoutes(t)
	if len(routes) < 100 {
		t.Fatalf("route source parser should see the API route table, got %d routes", len(routes))
	}

	swaggerRoutes := collectSwaggerRoutes(t)
	var missing []string
	for _, route := range routes {
		expected, ok := apiReferenceRoute(route)
		if !ok {
			continue
		}
		if !swaggerRoutes[expected] {
			missing = append(missing, expected.String())
		}
	}

	sort.Strings(missing)
	if len(missing) > 0 {
		t.Fatalf("generated Swagger is missing API route references:\n%s", strings.Join(missing, "\n"))
	}
}

func collectSourceRoutes(t *testing.T) []documentedRoute {
	t.Helper()

	sourcePath := filepath.Join("..", "cmd", "api", "main.go")
	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, sourcePath, nil, 0)
	if err != nil {
		t.Fatalf("parse API source: %v", err)
	}

	var routes []documentedRoute
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
				if len(call.Args) < 2 {
					t.Fatalf("route call should include prefix and handler")
				}
				funcLit, ok := call.Args[1].(*ast.FuncLit)
				if !ok {
					t.Fatalf("expected func literal in Route call")
				}
				walkBlock(funcLit.Body, append(prefixes, mustRouteString(t, call.Args[0])))
			case "Group":
				if len(call.Args) < 1 {
					t.Fatalf("group call should include handler")
				}
				funcLit, ok := call.Args[0].(*ast.FuncLit)
				if !ok {
					t.Fatalf("expected func literal in Group call")
				}
				walkBlock(funcLit.Body, prefixes)
			case "Get", "Post", "Put", "Delete", "Patch":
				if len(call.Args) < 1 {
					t.Fatalf("%s route call should include path", selector.Sel.Name)
				}
				routes = append(routes, documentedRoute{
					Method: strings.ToUpper(selector.Sel.Name),
					Path:   joinSourceRoutePath(prefixes, mustRouteString(t, call.Args[0])),
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

func collectSwaggerRoutes(t *testing.T) map[documentedRoute]bool {
	t.Helper()

	var spec struct {
		Paths map[string]map[string]any `json:"paths"`
	}
	if err := json.Unmarshal([]byte(SwaggerInfo.ReadDoc()), &spec); err != nil {
		t.Fatalf("decode generated swagger doc: %v", err)
	}

	routes := make(map[documentedRoute]bool)
	for path, methods := range spec.Paths {
		if strings.TrimSpace(path) == "" {
			continue
		}
		normalizedPath := path
		if trimmed, ok := strings.CutPrefix(normalizedPath, "/api/v1"); ok {
			normalizedPath = trimmed
		}
		normalizedPath = normalizeRoutePlaceholders(normalizedPath)
		for method := range methods {
			method = strings.ToUpper(method)
			if !isHTTPMethod(method) {
				continue
			}
			routes[documentedRoute{
				Method: method,
				Path:   normalizedPath,
			}] = true
		}
	}
	return routes
}

func mustRouteString(t *testing.T, expr ast.Expr) string {
	t.Helper()

	lit, ok := expr.(*ast.BasicLit)
	if !ok {
		t.Fatalf("expected string literal route path")
	}
	value, err := strconv.Unquote(lit.Value)
	if err != nil {
		t.Fatalf("unquote route path: %v", err)
	}
	return value
}

func joinSourceRoutePath(prefixes []string, path string) string {
	var joined strings.Builder
	for _, part := range append(prefixes, path) {
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

func collectAPIReferenceRoutes(t *testing.T) map[documentedRoute]bool {
	t.Helper()

	payload, err := os.ReadFile("API.md")
	if err != nil {
		t.Fatalf("read API reference: %v", err)
	}

	routes := make(map[documentedRoute]bool)
	for _, line := range strings.Split(string(payload), "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) < 2 {
			continue
		}
		method := fields[0]
		if !isHTTPMethod(method) {
			continue
		}
		routePath := strings.Trim(fields[1], "`")
		routePath, _, _ = strings.Cut(routePath, "?")
		routes[documentedRoute{
			Method: method,
			Path:   normalizeRoutePlaceholders(routePath),
		}] = true
	}
	return routes
}

func apiReferenceRoute(route documentedRoute) (documentedRoute, bool) {
	if route.Path == "/swagger/*" {
		return documentedRoute{}, false
	}

	path := route.Path
	if trimmed, ok := strings.CutPrefix(path, "/api/v1"); ok {
		path = trimmed
	}
	return documentedRoute{
		Method: route.Method,
		Path:   normalizeRoutePlaceholders(path),
	}, true
}

func normalizeRoutePlaceholders(value string) string {
	replacer := strings.NewReplacer(
		"ID}", "Id}",
		"/runtime/{path}", "/runtime/*",
	)
	return replacer.Replace(value)
}

func isHTTPMethod(value string) bool {
	switch value {
	case "GET", "POST", "PUT", "DELETE", "PATCH":
		return true
	default:
		return false
	}
}
