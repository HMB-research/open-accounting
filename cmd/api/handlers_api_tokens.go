package main

import (
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/HMB-research/open-accounting/internal/apitoken"
	"github.com/HMB-research/open-accounting/internal/auth"
)

// ListAPITokens returns API tokens for the current user in a tenant.
func (h *Handlers) ListAPITokens(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.GetClaims(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	tenantID := chi.URLParam(r, "tenantID")
	tokens, err := h.apiTokenService.ListTokens(r.Context(), claims.UserID, tenantID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to list API tokens")
		return
	}

	respondJSON(w, http.StatusOK, tokens)
}

// CreateAPIToken creates a new API token for the current user in a tenant.
func (h *Handlers) CreateAPIToken(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.GetClaims(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	var req apitoken.CreateRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		respondError(w, http.StatusBadRequest, "name is required")
		return
	}
	if req.ExpiresAt != nil && !req.ExpiresAt.After(time.Now()) {
		respondError(w, http.StatusBadRequest, "expires_at must be in the future")
		return
	}

	tenantID := chi.URLParam(r, "tenantID")
	result, err := h.apiTokenService.CreateToken(r.Context(), claims.UserID, tenantID, &req)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, result)
}

// RevokeAPIToken revokes an API token owned by the current user in a tenant.
func (h *Handlers) RevokeAPIToken(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.GetClaims(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	tenantID := chi.URLParam(r, "tenantID")
	tokenID := chi.URLParam(r, "tokenID")
	if err := h.apiTokenService.RevokeToken(r.Context(), claims.UserID, tenantID, tokenID); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "revoked"})
}
