package main

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/HMB-research/open-accounting/internal/auth"
)

type tenantRouteContext struct {
	tenantID   string
	schemaName string
}

func (h *Handlers) tenantContextFromRequest(r *http.Request) tenantRouteContext {
	tenantID := chi.URLParam(r, "tenantID")
	return tenantRouteContext{
		tenantID:   tenantID,
		schemaName: h.getSchemaName(r.Context(), tenantID),
	}
}

func userClaimsFromRequest(r *http.Request) *auth.Claims {
	claims, _ := auth.GetClaims(r.Context())
	return claims
}

func userIDFromRequest(r *http.Request) string {
	claims := userClaimsFromRequest(r)
	if claims == nil {
		return ""
	}
	return claims.UserID
}

func decodeJSONRequest(w http.ResponseWriter, r *http.Request, target interface{}) bool {
	if err := decodeJSON(r, target); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return false
	}
	return true
}
