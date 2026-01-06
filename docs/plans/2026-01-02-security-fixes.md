# Security Fixes Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Fix all security vulnerabilities identified in the security audit report.

**Architecture:** Add security headers middleware to Go API, replace vulnerable xlsx dependency with exceljs, increase bcrypt cost factor, and sanitize error messages. Each fix is isolated and can be deployed independently.

**Tech Stack:** Go 1.24, Chi router, bcrypt, npm/pnpm, exceljs

---

## Task 1: Add Security Headers Middleware

**Files:**
- Create: `internal/middleware/security.go`
- Create: `internal/middleware/security_test.go`
- Modify: `cmd/api/main.go:218-237`

**Step 1: Write the failing test**

Create file `internal/middleware/security_test.go`:

```go
package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSecurityHeaders(t *testing.T) {
	handler := SecurityHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	tests := []struct {
		header   string
		expected string
	}{
		{"X-Frame-Options", "DENY"},
		{"X-Content-Type-Options", "nosniff"},
		{"X-XSS-Protection", "1; mode=block"},
		{"Referrer-Policy", "strict-origin-when-cross-origin"},
		{"Permissions-Policy", "geolocation=(), microphone=(), camera=()"},
	}

	for _, tt := range tests {
		got := rec.Header().Get(tt.header)
		if got != tt.expected {
			t.Errorf("Header %s = %q, want %q", tt.header, got, tt.expected)
		}
	}
}

func TestSecurityHeadersPassesThrough(t *testing.T) {
	called := false
	handler := SecurityHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if !called {
		t.Error("Next handler was not called")
	}
	if rec.Code != http.StatusOK {
		t.Errorf("Status = %d, want %d", rec.Code, http.StatusOK)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/middleware/... -v`
Expected: FAIL with "cannot find package" or "undefined: SecurityHeaders"

**Step 3: Write minimal implementation**

Create file `internal/middleware/security.go`:

```go
package middleware

import "net/http"

// SecurityHeaders adds security-related HTTP headers to all responses
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Prevent clickjacking
		w.Header().Set("X-Frame-Options", "DENY")

		// Prevent MIME type sniffing
		w.Header().Set("X-Content-Type-Options", "nosniff")

		// XSS protection for older browsers
		w.Header().Set("X-XSS-Protection", "1; mode=block")

		// Control referrer information
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

		// Restrict browser features
		w.Header().Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")

		next.ServeHTTP(w, r)
	})
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/middleware/... -v`
Expected: PASS

**Step 5: Integrate middleware into router**

Modify `cmd/api/main.go` - add import and middleware usage.

Find line ~20 with imports and add:
```go
"github.com/HMB-research/open-accounting/internal/middleware"
```

Find line ~223 after `r.Use(middleware.Timeout(30 * time.Second))` and add before CORS:
```go
// Security headers
r.Use(middleware.SecurityHeaders)
```

**Step 6: Run all tests**

Run: `go test ./... -v`
Expected: All tests PASS

**Step 7: Commit**

```bash
git add internal/middleware/security.go internal/middleware/security_test.go cmd/api/main.go
git commit -m "feat(security): add security headers middleware

Adds X-Frame-Options, X-Content-Type-Options, X-XSS-Protection,
Referrer-Policy, and Permissions-Policy headers to all API responses."
```

---

## Task 2: Increase bcrypt Cost Factor

**Files:**
- Modify: `internal/tenant/service.go:210`
- Modify: `internal/tenant/service_test.go` (add test)

**Step 1: Write the failing test**

Add to `internal/tenant/service_test.go`:

```go
func TestCreateUser_BcryptCost(t *testing.T) {
	// This test verifies bcrypt uses cost 12 (stronger than default 10)
	// by checking the hash prefix format: $2a$12$...
	mockRepo := &MockRepository{
		createUserFn: func(ctx context.Context, user *User) error {
			// Check hash starts with $2a$12$ (cost 12)
			if !strings.HasPrefix(user.PasswordHash, "$2a$12$") {
				t.Errorf("Expected bcrypt cost 12 ($2a$12$), got hash prefix: %s", user.PasswordHash[:7])
			}
			return nil
		},
	}
	service := NewServiceWithRepository(mockRepo)

	_, err := service.CreateUser(context.Background(), &CreateUserRequest{
		Email:    "test@example.com",
		Password: "testpassword123",
		Name:     "Test User",
	})

	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/tenant/... -run TestCreateUser_BcryptCost -v`
Expected: FAIL with "Expected bcrypt cost 12 ($2a$12$), got hash prefix: $2a$10$"

**Step 3: Write minimal implementation**

Modify `internal/tenant/service.go` line 210.

Change from:
```go
hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
```

To:
```go
hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), 12)
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/tenant/... -run TestCreateUser_BcryptCost -v`
Expected: PASS

**Step 5: Run all tenant tests**

Run: `go test ./internal/tenant/... -v`
Expected: All tests PASS

**Step 6: Commit**

```bash
git add internal/tenant/service.go internal/tenant/service_test.go
git commit -m "security: increase bcrypt cost from 10 to 12

Improves password hashing strength against brute-force attacks.
New cost factor is 4x slower to compute (2^12 vs 2^10 iterations)."
```

---

## Task 3: Replace xlsx with exceljs

**Files:**
- Modify: `frontend/package.json`
- Modify: Any files importing `xlsx` (need to search)

**Step 1: Find xlsx usage**

Run: `grep -r "from 'xlsx'" frontend/src/ || grep -r 'from "xlsx"' frontend/src/`
Expected: List of files using xlsx

**Step 2: Uninstall xlsx**

Run: `cd frontend && npm uninstall xlsx`
Expected: Package removed from package.json

**Step 3: Install exceljs**

Run: `cd frontend && npm install exceljs`
Expected: Package added to package.json

**Step 4: Update imports in affected files**

For each file found in Step 1, replace xlsx usage with exceljs.

Example transformation for a typical export function:

From (xlsx):
```typescript
import * as XLSX from 'xlsx';

export function exportToExcel(data: any[], filename: string) {
    const ws = XLSX.utils.json_to_sheet(data);
    const wb = XLSX.utils.book_new();
    XLSX.utils.book_append_sheet(wb, ws, 'Sheet1');
    XLSX.writeFile(wb, filename);
}
```

To (exceljs):
```typescript
import ExcelJS from 'exceljs';

export async function exportToExcel(data: any[], filename: string) {
    const workbook = new ExcelJS.Workbook();
    const worksheet = workbook.addWorksheet('Sheet1');

    if (data.length > 0) {
        worksheet.columns = Object.keys(data[0]).map(key => ({
            header: key,
            key: key,
            width: 15
        }));
        worksheet.addRows(data);
    }

    const buffer = await workbook.xlsx.writeBuffer();
    const blob = new Blob([buffer], { type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = filename;
    a.click();
    URL.revokeObjectURL(url);
}
```

**Step 5: Run frontend tests**

Run: `cd frontend && npm test`
Expected: All tests PASS

**Step 6: Build frontend**

Run: `cd frontend && npm run build`
Expected: Build succeeds without errors

**Step 7: Run npm audit**

Run: `cd frontend && npm audit`
Expected: xlsx vulnerabilities no longer present

**Step 8: Commit**

```bash
git add frontend/package.json frontend/package-lock.json frontend/src/
git commit -m "security: replace vulnerable xlsx with exceljs

Fixes CVE prototype pollution and ReDoS vulnerabilities in xlsx package.
exceljs provides equivalent functionality with active maintenance."
```

---

## Task 4: Sanitize Error Messages

**Files:**
- Modify: `cmd/api/handlers.go:69-71`
- Modify: `cmd/api/handlers_business.go` (multiple locations)
- Create: `internal/apierror/errors.go`
- Create: `internal/apierror/errors_test.go`

**Step 1: Write the failing test for error sanitizer**

Create file `internal/apierror/errors_test.go`:

```go
package apierror

import "testing"

func TestSanitize_HidesInternalDetails(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "SQL error",
			input:    "pq: relation \"users\" does not exist",
			expected: "An internal error occurred",
		},
		{
			name:     "file path",
			input:    "open /var/lib/data/secret.json: no such file",
			expected: "An internal error occurred",
		},
		{
			name:     "connection error",
			input:    "dial tcp 192.168.1.100:5432: connection refused",
			expected: "An internal error occurred",
		},
		{
			name:     "safe validation error",
			input:    "name is required",
			expected: "name is required",
		},
		{
			name:     "safe format error",
			input:    "invalid date format",
			expected: "invalid date format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Sanitize(tt.input)
			if got != tt.expected {
				t.Errorf("Sanitize(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/apierror/... -v`
Expected: FAIL with "cannot find package"

**Step 3: Write minimal implementation**

Create file `internal/apierror/errors.go`:

```go
package apierror

import (
	"regexp"
	"strings"
)

// Patterns that indicate internal/sensitive errors
var sensitivePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)pq:|pgx:|sql:|postgres`),
	regexp.MustCompile(`(?i)connection|timeout|refused`),
	regexp.MustCompile(`(?i)/var/|/tmp/|/home/|/app/|\.go:\d+`),
	regexp.MustCompile(`(?i)dial tcp|network|socket`),
	regexp.MustCompile(`(?i)panic|runtime error`),
	regexp.MustCompile(`(?i)internal server|stack trace`),
	regexp.MustCompile(`\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}`), // IP addresses
}

const genericError = "An internal error occurred"

// Sanitize removes sensitive information from error messages
// Safe messages (validation errors, format errors) are passed through
func Sanitize(msg string) string {
	for _, pattern := range sensitivePatterns {
		if pattern.MatchString(msg) {
			return genericError
		}
	}

	// Additional check for file paths
	if strings.Contains(msg, "/") && (strings.Contains(msg, "open") || strings.Contains(msg, "read") || strings.Contains(msg, "write")) {
		return genericError
	}

	return msg
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/apierror/... -v`
Expected: PASS

**Step 5: Update respondError function**

Modify `cmd/api/handlers.go` line 69-71.

First, add import:
```go
"github.com/HMB-research/open-accounting/internal/apierror"
```

Then change:
```go
func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, map[string]string{"error": message})
}
```

To:
```go
func respondError(w http.ResponseWriter, status int, message string) {
	// Sanitize error messages for 5xx errors to prevent information leakage
	if status >= 500 {
		message = apierror.Sanitize(message)
	}
	respondJSON(w, status, map[string]string{"error": message})
}
```

**Step 6: Run all API tests**

Run: `go test ./cmd/api/... -v`
Expected: All tests PASS

**Step 7: Commit**

```bash
git add internal/apierror/errors.go internal/apierror/errors_test.go cmd/api/handlers.go
git commit -m "security: sanitize error messages in API responses

Prevents leaking internal details (SQL errors, file paths, IP addresses)
in 5xx error responses. Validation errors remain user-friendly."
```

---

## Task 5: Create .env.example Template

**Files:**
- Create: `.env.example`

**Step 1: Create the template file**

Create file `.env.example`:

```bash
# Open Accounting API Configuration
# Copy this file to .env and fill in the values

# Database connection string (required)
# Format: postgres://user:password@host:port/database?sslmode=require
DATABASE_URL=postgres://openaccounting:CHANGE_ME@localhost:5432/openaccounting?sslmode=disable

# JWT secret for token signing (required, min 32 characters)
# Generate with: openssl rand -base64 32
JWT_SECRET=CHANGE_ME_TO_A_SECURE_RANDOM_STRING_MIN_32_CHARS

# Server port (optional, default: 8080)
PORT=8080

# CORS allowed origins (required for production)
# Comma-separated list of allowed frontend URLs
ALLOWED_ORIGINS=http://localhost:5173

# Log level (optional, default: info)
# Valid values: trace, debug, info, warn, error, fatal, panic
LOG_LEVEL=info

# Enable verbose CORS logging (optional, default: false)
CORS_DEBUG=false

# Demo mode reset secret (optional, only for demo deployments)
# DEMO_RESET_SECRET=your-secret-key
```

**Step 2: Verify file is gitignored**

Run: `grep "\.env$" .gitignore`
Expected: `.env` is listed (already confirmed in audit)

**Step 3: Commit**

```bash
git add .env.example
git commit -m "docs: add .env.example configuration template

Provides documented environment variable template for developers.
Helps prevent configuration errors and documents all options."
```

---

## Task 6: Update cookie Package Override

**Files:**
- Modify: `frontend/package.json`

**Step 1: Check current override version**

The `frontend/package.json` already has:
```json
"overrides": {
    "cookie": "^1.0.0"
}
```

This is already the fix. Verify it's working:

**Step 2: Verify cookie version**

Run: `cd frontend && npm ls cookie`
Expected: Shows cookie@1.0.x or higher

**Step 3: Run npm audit**

Run: `cd frontend && npm audit`
Expected: No cookie vulnerabilities

**Step 4: No commit needed**

The fix is already in place. Mark as verified.

---

## Task 7: Final Verification

**Step 1: Run all backend tests**

Run: `go test ./... -v`
Expected: All tests PASS

**Step 2: Run all frontend tests**

Run: `cd frontend && npm test`
Expected: All tests PASS

**Step 3: Build and verify**

Run: `go build ./cmd/api && cd frontend && npm run build`
Expected: Both builds succeed

**Step 4: Run security scans**

Run: `cd frontend && npm audit`
Expected: No high/critical vulnerabilities

**Step 5: Create summary commit (if needed)**

If there were any additional fixes during verification:

```bash
git add -A
git commit -m "chore: security fixes verification and cleanup"
```

---

## Summary of Changes

| Task | Severity | Status |
|------|----------|--------|
| Security headers middleware | Medium | TODO |
| Increase bcrypt cost | Medium | TODO |
| Replace xlsx with exceljs | High | TODO |
| Sanitize error messages | Medium | TODO |
| Add .env.example | Low | TODO |
| Update cookie override | Low | Already Fixed |

**Total estimated time:** 45-60 minutes
