package main

import (
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/HMB-research/open-accounting/internal/apitoken"
	"github.com/HMB-research/open-accounting/internal/auth"
	"github.com/HMB-research/open-accounting/internal/tenant"
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

// ListTenantUserAPITokens returns API tokens for one tenant user.
// @Summary List tenant user API tokens
// @Description List active API tokens for a user who belongs to the tenant. Requires owner or admin role.
// @Tags Users
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param userID path string true "User ID"
// @Success 200 {array} apitoken.APIToken
// @Failure 400 {object} object{error=string}
// @Failure 403 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/users/{userID}/api-tokens [get]
func (h *Handlers) ListTenantUserAPITokens(w http.ResponseWriter, r *http.Request) {
	_, _, ok := h.authorizeTenantUserAdmin(w, r)
	if !ok {
		return
	}
	if h.apiTokenService == nil {
		respondError(w, http.StatusInternalServerError, "API token service unavailable")
		return
	}

	tenantID := chi.URLParam(r, "tenantID")
	userID := chi.URLParam(r, "userID")
	tokens, err := h.apiTokenService.ListTokens(r.Context(), userID, tenantID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to list API tokens")
		return
	}

	respondJSON(w, http.StatusOK, tokens)
}

// RevokeTenantUserAPIToken revokes one API token for a tenant user.
// @Summary Revoke tenant user API token
// @Description Revoke one active API token for a user who belongs to the tenant. Requires owner or admin role.
// @Tags Users
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param userID path string true "User ID"
// @Param tokenID path string true "API token ID"
// @Success 200 {object} object{status=string}
// @Failure 400 {object} object{error=string}
// @Failure 403 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/users/{userID}/api-tokens/{tokenID} [delete]
func (h *Handlers) RevokeTenantUserAPIToken(w http.ResponseWriter, r *http.Request) {
	claims, targetRole, ok := h.authorizeTenantUserAdmin(w, r)
	if !ok {
		return
	}
	if h.apiTokenService == nil {
		respondError(w, http.StatusInternalServerError, "API token service unavailable")
		return
	}

	tenantID := chi.URLParam(r, "tenantID")
	userID := chi.URLParam(r, "userID")
	tokenID := strings.TrimSpace(chi.URLParam(r, "tokenID"))
	if tokenID == "" {
		respondError(w, http.StatusBadRequest, "Token id is required")
		return
	}

	if err := h.apiTokenService.RevokeToken(r.Context(), userID, tenantID, tokenID); err != nil {
		if strings.Contains(err.Error(), "api token not found") {
			respondError(w, http.StatusNotFound, "API token not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "Failed to revoke API token")
		return
	}

	targetEmail := h.userEmailForAudit(r.Context(), userID)
	h.recordSecurityAuditEvent(r, &auth.SecurityAuditEvent{
		ActorUserID:  claims.UserID,
		ActorEmail:   claims.Email,
		Action:       auth.SecurityAuditActionAPITokenRevoked,
		TargetUserID: userID,
		TargetEmail:  targetEmail,
		Metadata: map[string]string{
			"tenant_id": tenantID,
			"token_id":  tokenID,
		},
	})
	if !h.recordTenantAuditEvent(w, r, &tenant.TenantAuditEvent{
		TenantID:    tenantID,
		ActorUserID: claims.UserID,
		Action:      tenant.AuditActionUserAPITokenRevoked,
		TargetType:  tenant.AuditTargetUser,
		TargetID:    userID,
		TargetEmail: targetEmail,
		Metadata: map[string]string{
			"role":     targetRole,
			"token_id": tokenID,
		},
	}) {
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "revoked"})
}
