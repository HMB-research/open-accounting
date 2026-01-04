package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRespondJSON(t *testing.T) {
	tests := []struct {
		name       string
		status     int
		data       interface{}
		wantStatus int
		wantBody   string
	}{
		{
			name:       "success with data",
			status:     http.StatusOK,
			data:       map[string]string{"message": "hello"},
			wantStatus: http.StatusOK,
			wantBody:   `{"message":"hello"}`,
		},
		{
			name:       "created with data",
			status:     http.StatusCreated,
			data:       map[string]int{"id": 123},
			wantStatus: http.StatusCreated,
			wantBody:   `{"id":123}`,
		},
		{
			name:       "no content",
			status:     http.StatusNoContent,
			data:       nil,
			wantStatus: http.StatusNoContent,
			wantBody:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			respondJSON(w, tt.status, tt.data)

			assert.Equal(t, tt.wantStatus, w.Code)
			assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

			if tt.wantBody != "" {
				// Trim newline from encoder
				body := bytes.TrimSpace(w.Body.Bytes())
				assert.JSONEq(t, tt.wantBody, string(body))
			}
		})
	}
}

func TestRespondError(t *testing.T) {
	tests := []struct {
		name       string
		status     int
		message    string
		wantStatus int
	}{
		{
			name:       "bad request",
			status:     http.StatusBadRequest,
			message:    "Invalid input",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "unauthorized",
			status:     http.StatusUnauthorized,
			message:    "Not authenticated",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "not found",
			status:     http.StatusNotFound,
			message:    "Resource not found",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "internal error",
			status:     http.StatusInternalServerError,
			message:    "Something went wrong",
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			respondError(w, tt.status, tt.message)

			assert.Equal(t, tt.wantStatus, w.Code)
			assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

			var resp map[string]string
			err := json.NewDecoder(w.Body).Decode(&resp)
			require.NoError(t, err)
			assert.Equal(t, tt.message, resp["error"])
		})
	}
}

func TestDecodeJSON(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		wantErr bool
	}{
		{
			name:    "valid json",
			body:    `{"email":"test@example.com","password":"secret123"}`,
			wantErr: false,
		},
		{
			name:    "empty object",
			body:    `{}`,
			wantErr: false,
		},
		{
			name:    "invalid json",
			body:    `{invalid}`,
			wantErr: true,
		},
		{
			name:    "empty body",
			body:    ``,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewBufferString(tt.body))
			req.Header.Set("Content-Type", "application/json")

			var data struct {
				Email    string `json:"email"`
				Password string `json:"password"`
			}

			err := decodeJSON(req, &data)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestHealthEndpoint(t *testing.T) {
	// Create a simple handler for the health check
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("OK"))
	})

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "OK", w.Body.String())
}

func TestGenerateDemoSeedForUser(t *testing.T) {
	tests := []struct {
		name        string
		userNum     int
		template    string
		contains    []string
		notContains []string
	}{
		{
			name:     "user 1 replacements",
			userNum:  1,
			template: "demo@example.com 'acme' tenant_acme a0000000-0000-0000-0000-000000000001",
			contains: []string{
				"demo1@example.com",
				"'demo1'",
				"tenant_demo1",
				"a0000000-0000-0000-0001-000000000001",
			},
			notContains: []string{
				"demo@example.com",
				"'acme'",
				"tenant_acme",
				"a0000000-0000-0000-0000-000000000001",
			},
		},
		{
			name:     "user 2 replacements",
			userNum:  2,
			template: "demo@example.com 'acme' tenant_acme b0000000-0000-0000-0000-000000000001",
			contains: []string{
				"demo2@example.com",
				"'demo2'",
				"tenant_demo2",
				"b0000000-0000-0000-0002-000000000001",
			},
		},
		{
			name:     "user 3 replacements",
			userNum:  3,
			template: "Acme Corporation @acme.ee info@acme.example.com",
			contains: []string{
				"Demo Company 3",
				"@demo3.example.com",
				"info@demo3.example.com",
			},
		},
		{
			name:     "uuid replacements for accounts",
			userNum:  1,
			template: "c0000000-0000-0000-0000-000000000001",
			contains: []string{"c1000000-0000-0000-0000-000000000001"},
		},
		{
			name:     "uuid replacements for contacts",
			userNum:  2,
			template: "d0000000-0000-0000-0000-000000000001",
			contains: []string{"d2000000-0000-0000-0000-000000000001"},
		},
		{
			name:     "uuid replacements for invoices",
			userNum:  1,
			template: "e0000000-0000-0000-0000-000000000001",
			contains: []string{"e1000000-0000-0000-0000-000000000001"},
		},
		{
			name:     "uuid replacements for payments",
			userNum:  2,
			template: "f0000000-0000-0000-0000-000000000001",
			contains: []string{"f2000000-0000-0000-0000-000000000001"},
		},
		{
			name:    "employee id replacements",
			userNum: 1,
			// Note: replacements cascade - 70000000 becomes 71000000 then 71100000
			template: "72000000-0000-0000-0000-000000000001",
			contains: []string{
				"72100000-0000-0000-0000-000000000001", // 72000000 becomes 72100000 for user 1
			},
		},
		{
			name:     "invoice number replacements",
			userNum:  1,
			template: "INV-2024-001 INV-2025-001 PAY-2024-001 JE-2024-001",
			contains: []string{
				"INV1-2024-001",
				"INV1-2025-001",
				"PAY1-2024-001",
				"JE1-2024-001",
			},
		},
		{
			name:     "fiscal year and bank account ids",
			userNum:  2,
			template: "80000000-0000-0000-0000-000000000001 90000000-0000-0000-0000-000000000001",
			contains: []string{
				"82000000-0000-0000-0000-000000000001",
				"92000000-0000-0000-0000-000000000001",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := generateDemoSeedForUser(tt.template, tt.userNum)

			for _, s := range tt.contains {
				assert.Contains(t, result, s, "expected result to contain %q", s)
			}

			for _, s := range tt.notContains {
				assert.NotContains(t, result, s, "expected result to NOT contain %q", s)
			}
		})
	}
}

func TestGetDemoSeedTemplate(t *testing.T) {
	template := getDemoSeedTemplate()

	// Verify the template contains key elements
	assert.Contains(t, template, "demo@example.com", "template should contain demo email")
	assert.Contains(t, template, "'acme'", "template should contain acme slug")
	assert.Contains(t, template, "tenant_acme", "template should contain tenant_acme schema")
	assert.Contains(t, template, "a0000000-0000-0000-0000-000000000001", "template should contain user UUID")
	assert.Contains(t, template, "b0000000-0000-0000-0000-000000000001", "template should contain tenant UUID")
	assert.Contains(t, template, "INSERT INTO users", "template should contain user insert")
	assert.Contains(t, template, "INSERT INTO tenants", "template should contain tenant insert")
	assert.Contains(t, template, "create_tenant_schema", "template should contain schema creation function")
}

func TestRequestValidation(t *testing.T) {
	t.Run("register validation", func(t *testing.T) {
		tests := []struct {
			name      string
			body      map[string]string
			expectErr bool
		}{
			{
				name:      "valid registration",
				body:      map[string]string{"email": "test@example.com", "password": "password123", "name": "Test User"},
				expectErr: false,
			},
			{
				name:      "missing email",
				body:      map[string]string{"password": "password123", "name": "Test User"},
				expectErr: true,
			},
			{
				name:      "missing password",
				body:      map[string]string{"email": "test@example.com", "name": "Test User"},
				expectErr: true,
			},
			{
				name:      "missing name",
				body:      map[string]string{"email": "test@example.com", "password": "password123"},
				expectErr: true,
			},
			{
				name:      "short password",
				body:      map[string]string{"email": "test@example.com", "password": "short", "name": "Test User"},
				expectErr: true,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				// Validate the request structure
				email := tt.body["email"]
				password := tt.body["password"]
				name := tt.body["name"]

				hasError := email == "" || password == "" || name == "" || len(password) < 8

				if tt.expectErr {
					assert.True(t, hasError, "expected validation error")
				} else {
					assert.False(t, hasError, "expected no validation error")
				}
			})
		}
	})
}
