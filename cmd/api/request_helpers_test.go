package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/HMB-research/open-accounting/internal/auth"
)

func TestTenantContextFromRequest(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/resource", nil)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("tenantID", "tenant-1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))

	got := (&Handlers{}).tenantContextFromRequest(req)

	assert.Equal(t, "tenant-1", got.tenantID)
	assert.Equal(t, "tenant_tenant-1", got.schemaName)
}

func TestUserClaimsFromRequest(t *testing.T) {
	claims := &auth.Claims{UserID: "user-1", Email: "user@example.com"}
	req := httptest.NewRequest(http.MethodGet, "/resource", nil).WithContext(
		context.WithValue(context.Background(), auth.ClaimsContextKey, claims),
	)

	assert.Same(t, claims, userClaimsFromRequest(req))
	assert.Equal(t, "user-1", userIDFromRequest(req))

	missingClaimsRequest := httptest.NewRequest(http.MethodGet, "/resource", nil)
	assert.Nil(t, userClaimsFromRequest(missingClaimsRequest))
	assert.Empty(t, userIDFromRequest(missingClaimsRequest))
}

func TestDecodeJSONRequest(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		wantOK     bool
		wantStatus int
		wantName   string
	}{
		{
			name:       "valid body",
			body:       `{"name":"Example"}`,
			wantOK:     true,
			wantStatus: http.StatusOK,
			wantName:   "Example",
		},
		{
			name:       "invalid body",
			body:       `{invalid}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "empty body",
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var payload struct {
				Name string `json:"name"`
			}
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/resource", bytes.NewBufferString(tt.body))

			gotOK := decodeJSONRequest(w, req, &payload)

			assert.Equal(t, tt.wantOK, gotOK)
			assert.Equal(t, tt.wantStatus, w.Code)
			if tt.wantOK {
				assert.Equal(t, tt.wantName, payload.Name)
				assert.Empty(t, w.Body.String())
				return
			}

			var response map[string]string
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
			assert.Equal(t, "Invalid request body", response["error"])
		})
	}
}
