package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"

	"github.com/go-chi/chi/v5"

	"github.com/HMB-research/open-accounting/internal/auth"
)

// =============================================================================
// Test Helpers
// =============================================================================

// createTestClaims creates JWT claims for testing with the given parameters
func createTestClaims(userID, email, tenantID, role string) *auth.Claims {
	return &auth.Claims{
		UserID:   userID,
		Email:    email,
		TenantID: tenantID,
		Role:     role,
	}
}

// contextWithClaims adds JWT claims to a context
func contextWithClaims(ctx context.Context, claims *auth.Claims) context.Context {
	return context.WithValue(ctx, auth.ClaimsContextKey, claims)
}

// makeAuthenticatedRequest creates an HTTP request with authentication context
func makeAuthenticatedRequest(method, path string, body interface{}, claims *auth.Claims) *http.Request {
	var bodyReader *bytes.Buffer
	if body != nil {
		jsonBody, _ := json.Marshal(body)
		bodyReader = bytes.NewBuffer(jsonBody)
	} else {
		bodyReader = bytes.NewBuffer(nil)
	}

	req := httptest.NewRequest(method, path, bodyReader)
	req.Header.Set("Content-Type", "application/json")

	if claims != nil {
		ctx := contextWithClaims(req.Context(), claims)
		req = req.WithContext(ctx)
	}

	return req
}

// withURLParams adds chi URL parameters to a request context
func withURLParams(r *http.Request, params map[string]string) *http.Request {
	rctx := chi.NewRouteContext()
	for key, value := range params {
		rctx.URLParams.Add(key, value)
	}
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}
