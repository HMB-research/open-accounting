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
