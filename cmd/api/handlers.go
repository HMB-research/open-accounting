package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
	"github.com/shopspring/decimal"

	"github.com/HMB-research/open-accounting/internal/accounting"
	"github.com/HMB-research/open-accounting/internal/apierror"
	"github.com/HMB-research/open-accounting/internal/analytics"
	"github.com/HMB-research/open-accounting/internal/auth"
	"github.com/HMB-research/open-accounting/internal/banking"
	"github.com/HMB-research/open-accounting/internal/contacts"
	"github.com/HMB-research/open-accounting/internal/email"
	"github.com/HMB-research/open-accounting/internal/invoicing"
	"github.com/HMB-research/open-accounting/internal/payments"
	"github.com/HMB-research/open-accounting/internal/payroll"
	"github.com/HMB-research/open-accounting/internal/pdf"
	"github.com/HMB-research/open-accounting/internal/plugin"
	"github.com/HMB-research/open-accounting/internal/recurring"
	"github.com/HMB-research/open-accounting/internal/tax"
	"github.com/HMB-research/open-accounting/internal/tenant"
)

// Handlers contains all HTTP handlers
type Handlers struct {
	pool              *pgxpool.Pool
	tokenService      *auth.TokenService
	tenantService     *tenant.Service
	accountingService *accounting.Service
	contactsService   *contacts.Service
	invoicingService  *invoicing.Service
	paymentsService   *payments.Service
	pdfService        *pdf.Service
	analyticsService  *analytics.Service
	recurringService  *recurring.Service
	emailService      *email.Service
	bankingService    *banking.Service
	taxService        *tax.Service
	payrollService    *payroll.Service
	pluginService     *plugin.Service
}

// getSchemaName returns the schema name for a tenant
func (h *Handlers) getSchemaName(ctx context.Context, tenantID string) string {
	t, err := h.tenantService.GetTenant(ctx, tenantID)
	if err != nil {
		return "tenant_" + tenantID
	}
	return t.SchemaName
}

// JSON helper functions
func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if data != nil {
		_ = json.NewEncoder(w).Encode(data)
	}
}

func respondError(w http.ResponseWriter, status int, message string) {
	// Sanitize error messages for 5xx errors to prevent information leakage
	if status >= 500 {
		message = apierror.Sanitize(message)
	}
	respondJSON(w, status, map[string]string{"error": message})
}

func decodeJSON(r *http.Request, v interface{}) error {
	return json.NewDecoder(r.Body).Decode(v)
}

// TenantContext middleware ensures user has access to the tenant
func (h *Handlers) TenantContext(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, ok := auth.GetClaims(r.Context())
		if !ok {
			respondError(w, http.StatusUnauthorized, "Authentication required")
			return
		}

		tenantID := chi.URLParam(r, "tenantID")
		if tenantID == "" {
			respondError(w, http.StatusBadRequest, "Tenant ID required")
			return
		}

		// Verify user has access to this tenant
		role, err := h.tenantService.GetUserRole(r.Context(), tenantID, claims.UserID)
		if err != nil {
			respondError(w, http.StatusForbidden, "Access denied to this tenant")
			return
		}

		// Update claims with tenant context
		claims.TenantID = tenantID
		claims.Role = role

		next.ServeHTTP(w, r)
	})
}

// Register creates a new user account
// @Summary Register new user
// @Description Create a new user account
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body object{email=string,password=string,name=string} true "Registration details"
// @Success 201 {object} object{id=string,email=string,name=string}
// @Failure 400 {object} object{error=string}
// @Router /auth/register [post]
func (h *Handlers) Register(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		Name     string `json:"name"`
	}
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Email == "" || req.Password == "" || req.Name == "" {
		respondError(w, http.StatusBadRequest, "Email, password, and name are required")
		return
	}

	if len(req.Password) < 8 {
		respondError(w, http.StatusBadRequest, "Password must be at least 8 characters")
		return
	}

	user, err := h.tenantService.CreateUser(r.Context(), &tenant.CreateUserRequest{
		Email:    req.Email,
		Password: req.Password,
		Name:     req.Name,
	})
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, map[string]interface{}{
		"id":    user.ID,
		"email": user.Email,
		"name":  user.Name,
	})
}

// Login authenticates a user and returns tokens
// @Summary User login
// @Description Authenticate user and get JWT tokens
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body object{email=string,password=string,tenant_id=string} true "Login credentials"
// @Success 200 {object} object{access_token=string,refresh_token=string,token_type=string,expires_in=int,user=object}
// @Failure 401 {object} object{error=string}
// @Failure 403 {object} object{error=string}
// @Router /auth/login [post]
func (h *Handlers) Login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		TenantID string `json:"tenant_id,omitempty"`
	}
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	user, err := h.tenantService.GetUserByEmail(r.Context(), req.Email)
	if err != nil {
		respondError(w, http.StatusUnauthorized, "Invalid credentials")
		return
	}

	if !h.tenantService.ValidatePassword(user, req.Password) {
		respondError(w, http.StatusUnauthorized, "Invalid credentials")
		return
	}

	if !user.IsActive {
		respondError(w, http.StatusForbidden, "Account is disabled")
		return
	}

	// Get tenant and role if specified
	tenantID := ""
	role := ""
	if req.TenantID != "" {
		r, err := h.tenantService.GetUserRole(r.Context(), req.TenantID, user.ID)
		if err != nil {
			respondError(w, http.StatusForbidden, "Access denied to tenant")
			return
		}
		tenantID = req.TenantID
		role = r
	}

	accessToken, err := h.tokenService.GenerateAccessToken(user.ID, user.Email, tenantID, role)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to generate token")
		return
	}

	refreshToken, err := h.tokenService.GenerateRefreshToken(user.ID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to generate refresh token")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"access_token":  accessToken,
		"refresh_token": refreshToken,
		"token_type":    "Bearer",
		"expires_in":    900, // 15 minutes
		"user": map[string]interface{}{
			"id":    user.ID,
			"email": user.Email,
			"name":  user.Name,
		},
	})
}

// RefreshToken generates a new access token from a refresh token
// @Summary Refresh access token
// @Description Exchange refresh token for new access token
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body object{refresh_token=string,tenant_id=string} true "Refresh token"
// @Success 200 {object} object{access_token=string,token_type=string,expires_in=int}
// @Failure 401 {object} object{error=string}
// @Router /auth/refresh [post]
func (h *Handlers) RefreshToken(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RefreshToken string `json:"refresh_token"`
		TenantID     string `json:"tenant_id,omitempty"`
	}
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	userID, err := h.tokenService.ValidateRefreshToken(req.RefreshToken)
	if err != nil {
		respondError(w, http.StatusUnauthorized, "Invalid refresh token")
		return
	}

	user, err := h.tenantService.GetUserByID(r.Context(), userID)
	if err != nil {
		respondError(w, http.StatusUnauthorized, "User not found")
		return
	}

	// Get tenant and role if specified
	tenantID := ""
	role := ""
	if req.TenantID != "" {
		r, err := h.tenantService.GetUserRole(r.Context(), req.TenantID, user.ID)
		if err != nil {
			respondError(w, http.StatusForbidden, "Access denied to tenant")
			return
		}
		tenantID = req.TenantID
		role = r
	}

	accessToken, err := h.tokenService.GenerateAccessToken(user.ID, user.Email, tenantID, role)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to generate token")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"access_token": accessToken,
		"token_type":   "Bearer",
		"expires_in":   900,
	})
}

// GetCurrentUser returns the current authenticated user
// @Summary Get current user
// @Description Get the currently authenticated user's profile
// @Tags Users
// @Produce json
// @Security BearerAuth
// @Success 200 {object} object{id=string,email=string,name=string,created_at=string}
// @Failure 401 {object} object{error=string}
// @Router /me [get]
func (h *Handlers) GetCurrentUser(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.GetClaims(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	user, err := h.tenantService.GetUserByID(r.Context(), claims.UserID)
	if err != nil {
		respondError(w, http.StatusNotFound, "User not found")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"id":         user.ID,
		"email":      user.Email,
		"name":       user.Name,
		"created_at": user.CreatedAt,
	})
}

// ListMyTenants returns all tenants the current user belongs to
// @Summary List user's tenants
// @Description Get all tenants the current user is a member of
// @Tags Users
// @Produce json
// @Security BearerAuth
// @Success 200 {array} object{tenant_id=string,tenant_name=string,role=string}
// @Failure 401 {object} object{error=string}
// @Router /me/tenants [get]
func (h *Handlers) ListMyTenants(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.GetClaims(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	memberships, err := h.tenantService.ListUserTenants(r.Context(), claims.UserID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to list tenants")
		return
	}

	respondJSON(w, http.StatusOK, memberships)
}

// CreateTenant creates a new tenant
// @Summary Create tenant
// @Description Create a new tenant organization
// @Tags Tenants
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body object{name=string,slug=string,settings=object} true "Tenant details"
// @Success 201 {object} tenant.Tenant
// @Failure 400 {object} object{error=string}
// @Failure 401 {object} object{error=string}
// @Router /tenants [post]
func (h *Handlers) CreateTenant(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.GetClaims(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	var req struct {
		Name     string                 `json:"name"`
		Slug     string                 `json:"slug"`
		Settings *tenant.TenantSettings `json:"settings,omitempty"`
	}
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Name == "" || req.Slug == "" {
		respondError(w, http.StatusBadRequest, "Name and slug are required")
		return
	}

	t, err := h.tenantService.CreateTenant(r.Context(), &tenant.CreateTenantRequest{
		Name:     req.Name,
		Slug:     req.Slug,
		Settings: req.Settings,
		OwnerID:  claims.UserID,
	})
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, t)
}

// GetTenant returns a tenant by ID
// @Summary Get tenant
// @Description Get tenant details by ID
// @Tags Tenants
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Success 200 {object} tenant.Tenant
// @Failure 403 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Router /tenants/{tenantID} [get]
func (h *Handlers) GetTenant(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.GetClaims(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	tenantID := chi.URLParam(r, "tenantID")

	// Verify user has access
	_, err := h.tenantService.GetUserRole(r.Context(), tenantID, claims.UserID)
	if err != nil {
		respondError(w, http.StatusForbidden, "Access denied")
		return
	}

	t, err := h.tenantService.GetTenant(r.Context(), tenantID)
	if err != nil {
		respondError(w, http.StatusNotFound, "Tenant not found")
		return
	}

	respondJSON(w, http.StatusOK, t)
}

// UpdateTenant updates a tenant's name and/or settings
func (h *Handlers) UpdateTenant(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.GetClaims(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	tenantID := chi.URLParam(r, "tenantID")

	// Verify user has admin access
	role, err := h.tenantService.GetUserRole(r.Context(), tenantID, claims.UserID)
	if err != nil {
		respondError(w, http.StatusForbidden, "Access denied")
		return
	}

	// Only owners and admins can update tenant settings
	perms := tenant.GetRolePermissions(role)
	if !perms.CanManageSettings {
		respondError(w, http.StatusForbidden, "Insufficient permissions")
		return
	}

	var req tenant.UpdateTenantRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	t, err := h.tenantService.UpdateTenant(r.Context(), tenantID, &req)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, t)
}

// CompleteOnboarding marks the tenant's onboarding as completed
// @Summary Complete onboarding
// @Description Mark the tenant's onboarding wizard as completed
// @Tags Tenants
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Success 200 {object} object{success=bool}
// @Failure 403 {object} object{error=string}
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/complete-onboarding [post]
func (h *Handlers) CompleteOnboarding(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.GetClaims(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	tenantID := chi.URLParam(r, "tenantID")

	// Verify user has access to this tenant
	_, err := h.tenantService.GetUserRole(r.Context(), tenantID, claims.UserID)
	if err != nil {
		respondError(w, http.StatusForbidden, "Access denied")
		return
	}

	if err := h.tenantService.CompleteOnboarding(r.Context(), tenantID); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]bool{"success": true})
}

// ListAccounts returns all accounts for a tenant
// @Summary List accounts
// @Description Get all accounts (chart of accounts) for a tenant
// @Tags Accounts
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param active_only query bool false "Filter for active accounts only"
// @Success 200 {array} accounting.Account
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/accounts [get]
func (h *Handlers) ListAccounts(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.GetClaims(r.Context())
	tenantID := chi.URLParam(r, "tenantID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	activeOnly := r.URL.Query().Get("active_only") == "true"

	accounts, err := h.accountingService.ListAccounts(r.Context(), schemaName, tenantID, activeOnly)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to list accounts")
		return
	}

	_ = claims // Used for audit logging in production
	respondJSON(w, http.StatusOK, accounts)
}

// CreateAccount creates a new account
// @Summary Create account
// @Description Create a new account in the chart of accounts
// @Tags Accounts
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param request body accounting.CreateAccountRequest true "Account details"
// @Success 201 {object} accounting.Account
// @Failure 400 {object} object{error=string}
// @Router /tenants/{tenantID}/accounts [post]
func (h *Handlers) CreateAccount(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	var req accounting.CreateAccountRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Code == "" || req.Name == "" || req.AccountType == "" {
		respondError(w, http.StatusBadRequest, "Code, name, and account_type are required")
		return
	}

	account, err := h.accountingService.CreateAccount(r.Context(), schemaName, tenantID, &req)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, account)
}

// GetAccount returns an account by ID
// @Summary Get account
// @Description Get account details by ID
// @Tags Accounts
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param accountID path string true "Account ID"
// @Success 200 {object} accounting.Account
// @Failure 404 {object} object{error=string}
// @Router /tenants/{tenantID}/accounts/{accountID} [get]
func (h *Handlers) GetAccount(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	accountID := chi.URLParam(r, "accountID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	account, err := h.accountingService.GetAccount(r.Context(), schemaName, tenantID, accountID)
	if err != nil {
		respondError(w, http.StatusNotFound, "Account not found")
		return
	}

	respondJSON(w, http.StatusOK, account)
}

// GetJournalEntry returns a journal entry by ID
// @Summary Get journal entry
// @Description Get journal entry details by ID
// @Tags Journal Entries
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param entryID path string true "Journal Entry ID"
// @Success 200 {object} accounting.JournalEntry
// @Failure 404 {object} object{error=string}
// @Router /tenants/{tenantID}/journal-entries/{entryID} [get]
func (h *Handlers) GetJournalEntry(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	entryID := chi.URLParam(r, "entryID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	entry, err := h.accountingService.GetJournalEntry(r.Context(), schemaName, tenantID, entryID)
	if err != nil {
		respondError(w, http.StatusNotFound, "Journal entry not found")
		return
	}

	respondJSON(w, http.StatusOK, entry)
}

// CreateJournalEntry creates a new journal entry
// @Summary Create journal entry
// @Description Create a new double-entry journal entry
// @Tags Journal Entries
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param request body accounting.CreateJournalEntryRequest true "Journal entry details"
// @Success 201 {object} accounting.JournalEntry
// @Failure 400 {object} object{error=string}
// @Router /tenants/{tenantID}/journal-entries [post]
func (h *Handlers) CreateJournalEntry(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.GetClaims(r.Context())
	tenantID := chi.URLParam(r, "tenantID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	var req accounting.CreateJournalEntryRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	req.UserID = claims.UserID

	if req.EntryDate.IsZero() {
		req.EntryDate = time.Now()
	}

	if len(req.Lines) < 2 {
		respondError(w, http.StatusBadRequest, "At least 2 lines required")
		return
	}

	entry, err := h.accountingService.CreateJournalEntry(r.Context(), schemaName, tenantID, &req)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, entry)
}

// PostJournalEntry posts a draft journal entry
// @Summary Post journal entry
// @Description Post a draft journal entry to finalize it
// @Tags Journal Entries
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param entryID path string true "Journal Entry ID"
// @Success 200 {object} object{status=string}
// @Failure 400 {object} object{error=string}
// @Router /tenants/{tenantID}/journal-entries/{entryID}/post [post]
func (h *Handlers) PostJournalEntry(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.GetClaims(r.Context())
	tenantID := chi.URLParam(r, "tenantID")
	entryID := chi.URLParam(r, "entryID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	err := h.accountingService.PostJournalEntry(r.Context(), schemaName, tenantID, entryID, claims.UserID)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "posted"})
}

// VoidJournalEntry voids a posted journal entry
// @Summary Void journal entry
// @Description Void a posted journal entry (creates reversal)
// @Tags Journal Entries
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param entryID path string true "Journal Entry ID"
// @Param request body object{reason=string} true "Void reason"
// @Success 200 {object} accounting.JournalEntry
// @Failure 400 {object} object{error=string}
// @Router /tenants/{tenantID}/journal-entries/{entryID}/void [post]
func (h *Handlers) VoidJournalEntry(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.GetClaims(r.Context())
	tenantID := chi.URLParam(r, "tenantID")
	entryID := chi.URLParam(r, "entryID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	var req struct {
		Reason string `json:"reason"`
	}
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Reason == "" {
		respondError(w, http.StatusBadRequest, "Void reason is required")
		return
	}

	reversal, err := h.accountingService.VoidJournalEntry(r.Context(), schemaName, tenantID, entryID, claims.UserID, req.Reason)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, reversal)
}

// GetTrialBalance returns the trial balance for a tenant
// @Summary Get trial balance
// @Description Get trial balance report as of a specific date
// @Tags Reports
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param as_of_date query string false "As of date (YYYY-MM-DD)"
// @Success 200 {object} accounting.TrialBalance
// @Failure 400 {object} object{error=string}
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/reports/trial-balance [get]
func (h *Handlers) GetTrialBalance(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	asOfDateStr := r.URL.Query().Get("as_of_date")
	asOfDate := time.Now()
	if asOfDateStr != "" {
		parsed, err := time.Parse("2006-01-02", asOfDateStr)
		if err != nil {
			respondError(w, http.StatusBadRequest, "Invalid date format. Use YYYY-MM-DD")
			return
		}
		asOfDate = parsed
	}

	tb, err := h.accountingService.GetTrialBalance(r.Context(), schemaName, tenantID, asOfDate)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to generate trial balance")
		return
	}

	respondJSON(w, http.StatusOK, tb)
}

// GetAccountBalance returns the balance of a specific account
// @Summary Get account balance
// @Description Get the balance of a specific account as of a date
// @Tags Reports
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param accountID path string true "Account ID"
// @Param as_of_date query string false "As of date (YYYY-MM-DD)"
// @Success 200 {object} object{account_id=string,as_of_date=string,balance=string}
// @Failure 400 {object} object{error=string}
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/reports/account-balance/{accountID} [get]
func (h *Handlers) GetAccountBalance(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	accountID := chi.URLParam(r, "accountID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	asOfDateStr := r.URL.Query().Get("as_of_date")
	asOfDate := time.Now()
	if asOfDateStr != "" {
		parsed, err := time.Parse("2006-01-02", asOfDateStr)
		if err != nil {
			respondError(w, http.StatusBadRequest, "Invalid date format. Use YYYY-MM-DD")
			return
		}
		asOfDate = parsed
	}

	balance, err := h.accountingService.GetAccountBalance(r.Context(), schemaName, tenantID, accountID, asOfDate)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get account balance")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"account_id": accountID,
		"as_of_date": asOfDate.Format("2006-01-02"),
		"balance":    balance.String(),
	})
}

// GetBalanceSheet returns the balance sheet for a tenant
// @Summary Get balance sheet
// @Description Get balance sheet report as of a specific date
// @Tags Reports
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param as_of query string false "As of date (YYYY-MM-DD)"
// @Success 200 {object} accounting.BalanceSheet
// @Failure 400 {object} object{error=string}
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/reports/balance-sheet [get]
func (h *Handlers) GetBalanceSheet(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	asOfDateStr := r.URL.Query().Get("as_of")
	asOfDate := time.Now()
	if asOfDateStr != "" {
		parsed, err := time.Parse("2006-01-02", asOfDateStr)
		if err != nil {
			respondError(w, http.StatusBadRequest, "Invalid date format. Use YYYY-MM-DD")
			return
		}
		asOfDate = parsed
	}

	bs, err := h.accountingService.GetBalanceSheet(r.Context(), schemaName, tenantID, asOfDate)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to generate balance sheet")
		return
	}

	respondJSON(w, http.StatusOK, bs)
}

// GetIncomeStatement returns the income statement for a tenant
// @Summary Get income statement
// @Description Get income statement (P&L) report for a specific period
// @Tags Reports
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param start query string true "Start date (YYYY-MM-DD)"
// @Param end query string true "End date (YYYY-MM-DD)"
// @Success 200 {object} accounting.IncomeStatement
// @Failure 400 {object} object{error=string}
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/reports/income-statement [get]
func (h *Handlers) GetIncomeStatement(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")

	startDateStr := r.URL.Query().Get("start")
	endDateStr := r.URL.Query().Get("end")

	if startDateStr == "" || endDateStr == "" {
		respondError(w, http.StatusBadRequest, "start and end date parameters are required")
		return
	}

	startDate, err := time.Parse("2006-01-02", startDateStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid start date format. Use YYYY-MM-DD")
		return
	}

	endDate, err := time.Parse("2006-01-02", endDateStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid end date format. Use YYYY-MM-DD")
		return
	}

	if endDate.Before(startDate) {
		respondError(w, http.StatusBadRequest, "End date must be after start date")
		return
	}

	schemaName := h.getSchemaName(r.Context(), tenantID)
	is, err := h.accountingService.GetIncomeStatement(r.Context(), schemaName, tenantID, startDate, endDate)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to generate income statement")
		return
	}

	respondJSON(w, http.StatusOK, is)
}

// Custom JSON marshaling for decimal values
func init() {
	// Register decimal type for proper JSON encoding
	decimal.MarshalJSONWithoutQuotes = true
}

// HandleGenerateKMD generates a KMD declaration for a period
// @Summary Generate KMD declaration
// @Description Generate an Estonian VAT declaration (KMD) for a specific period
// @Tags Tax
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param request body tax.CreateKMDRequest true "Period to generate"
// @Success 200 {object} tax.KMDDeclaration
// @Failure 400 {object} object{error=string}
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/tax/kmd [post]
func (h *Handlers) HandleGenerateKMD(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")

	var req tax.CreateKMDRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	schemaName := h.getSchemaName(r.Context(), tenantID)

	decl, err := h.taxService.GenerateKMD(r.Context(), tenantID, schemaName, &req)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, decl)
}

// HandleListKMD lists all KMD declarations for a tenant
// @Summary List KMD declarations
// @Description Get all KMD declarations for a tenant
// @Tags Tax
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Success 200 {array} tax.KMDDeclaration
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/tax/kmd [get]
func (h *Handlers) HandleListKMD(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	declarations, err := h.taxService.ListKMD(r.Context(), tenantID, schemaName)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, declarations)
}

// HandleExportKMD exports a KMD declaration to XML
// @Summary Export KMD to XML
// @Description Export a KMD declaration to Estonian e-MTA XML format
// @Tags Tax
// @Produce application/xml
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param year path string true "Year"
// @Param month path string true "Month"
// @Success 200 {file} file "XML file"
// @Failure 404 {object} object{error=string}
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/tax/kmd/{year}/{month}/xml [get]
func (h *Handlers) HandleExportKMD(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	year := chi.URLParam(r, "year")
	month := chi.URLParam(r, "month")

	// Get tenant settings for registration number
	t, err := h.tenantService.GetTenant(r.Context(), tenantID)
	if err != nil {
		respondError(w, http.StatusNotFound, "Tenant not found")
		return
	}

	schemaName := h.getSchemaName(r.Context(), tenantID)

	// Get declaration
	decl, err := h.taxService.GetKMD(r.Context(), tenantID, schemaName, year, month)
	if err != nil {
		respondError(w, http.StatusNotFound, "Declaration not found")
		return
	}

	// Get registration number from tenant settings
	regNr := t.Settings.RegCode

	xml, err := tax.ExportKMDToXML(decl, regNr)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/xml")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=KMD_%s_%s.xml", year, month))
	_, _ = w.Write(xml)
}

// DemoReset resets the demo database to initial state
// @Summary Reset demo database
// @Description Reset the demo database to initial state (requires DEMO_RESET_SECRET)
// @Tags Demo
// @Accept json
// @Produce json
// @Param X-Demo-Secret header string true "Demo reset secret key"
// @Success 200 {object} object{status=string,message=string}
// @Failure 401 {object} object{error=string}
// @Failure 403 {object} object{error=string}
// @Failure 500 {object} object{error=string}
// @Router /api/demo/reset [post]
func (h *Handlers) DemoReset(w http.ResponseWriter, r *http.Request) {
	log.Info().Msg("Demo reset requested")

	// Check if demo mode is enabled
	if os.Getenv("DEMO_MODE") != "true" {
		log.Warn().Msg("Demo reset rejected: DEMO_MODE not enabled")
		respondError(w, http.StatusForbidden, "Demo mode is not enabled")
		return
	}

	// Validate secret key
	secret := os.Getenv("DEMO_RESET_SECRET")
	if secret == "" {
		log.Warn().Msg("Demo reset rejected: DEMO_RESET_SECRET not configured")
		respondError(w, http.StatusForbidden, "Demo reset not configured")
		return
	}

	providedSecret := r.Header.Get("X-Demo-Secret")
	if providedSecret == "" {
		providedSecret = r.URL.Query().Get("secret")
	}

	if providedSecret != secret {
		log.Warn().Msg("Demo reset rejected: invalid secret")
		respondError(w, http.StatusUnauthorized, "Invalid or missing secret key")
		return
	}

	ctx := r.Context()

	// Demo identifiers for 4 demo users:
	// - demo1: Reserved for end users (README documentation)
	// - demo2, demo3, demo4: Used by E2E tests (3 parallel workers)
	allDemoUsers := []struct {
		email  string
		slug   string
		schema string
	}{
		{"demo1@example.com", "demo1", "tenant_demo1"},
		{"demo2@example.com", "demo2", "tenant_demo2"},
		{"demo3@example.com", "demo3", "tenant_demo3"},
		{"demo4@example.com", "demo4", "tenant_demo4"},
	}

	// Parse optional user parameter for single-user reset
	var demoUsers []struct {
		email  string
		slug   string
		schema string
	}

	userParam := r.URL.Query().Get("user")
	if userParam != "" {
		userNum, err := strconv.Atoi(userParam)
		if err != nil || userNum < 1 || userNum > 4 {
			log.Warn().Str("user", userParam).Msg("Demo reset rejected: invalid user parameter")
			respondError(w, http.StatusBadRequest, "Invalid user parameter. Must be 1, 2, 3, or 4")
			return
		}
		demoUsers = []struct {
			email  string
			slug   string
			schema string
		}{allDemoUsers[userNum-1]}
		log.Info().Int("user", userNum).Msg("Demo reset: resetting single user")
	} else {
		demoUsers = allDemoUsers
		log.Info().Msg("Demo reset: resetting all users")
	}

	// Drop demo tenant schemas
	for _, demo := range demoUsers {
		log.Info().Str("schema", demo.schema).Msg("Demo reset: dropping tenant schema")
		_, err := h.pool.Exec(ctx, fmt.Sprintf("DROP SCHEMA IF EXISTS %s CASCADE", demo.schema))
		if err != nil {
			log.Error().Err(err).Str("schema", demo.schema).Msg("Demo reset failed: drop schema")
			respondError(w, http.StatusInternalServerError, "Failed to drop tenant schema: "+err.Error())
			return
		}
	}

	// Delete demo data from public tables
	for _, demo := range demoUsers {
		log.Info().Str("slug", demo.slug).Msg("Demo reset: cleaning tenant_users by slug")
		_, err := h.pool.Exec(ctx, "DELETE FROM tenant_users WHERE tenant_id IN (SELECT id FROM tenants WHERE slug = $1)", demo.slug)
		if err != nil {
			log.Error().Err(err).Msg("Demo reset failed: clean tenant_users")
			respondError(w, http.StatusInternalServerError, "Failed to clean tenant_users: "+err.Error())
			return
		}

		log.Info().Str("slug", demo.slug).Msg("Demo reset: cleaning tenants by slug")
		_, err = h.pool.Exec(ctx, "DELETE FROM tenants WHERE slug = $1", demo.slug)
		if err != nil {
			log.Error().Err(err).Msg("Demo reset failed: clean tenants")
			respondError(w, http.StatusInternalServerError, "Failed to clean tenants: "+err.Error())
			return
		}

		log.Info().Str("email", demo.email).Msg("Demo reset: cleaning users by email")
		_, err = h.pool.Exec(ctx, "DELETE FROM users WHERE email = $1", demo.email)
		if err != nil {
			log.Error().Err(err).Msg("Demo reset failed: clean users")
			respondError(w, http.StatusInternalServerError, "Failed to clean users: "+err.Error())
			return
		}
	}

	log.Info().Msg("Demo reset: seeding demo data")
	// Re-seed demo data by executing the seed SQL
	// Note: In production, you might want to read from a file or embedded resource
	seedSQL := getDemoSeedSQL()
	_, err := h.pool.Exec(ctx, seedSQL)
	if err != nil {
		log.Error().Err(err).Str("sql_preview", seedSQL[:500]).Msg("Demo reset failed: seed data")
		respondError(w, http.StatusInternalServerError, "Failed to seed demo data: "+err.Error())
		return
	}

	log.Info().Msg("Demo reset completed successfully")
	respondJSON(w, http.StatusOK, map[string]string{
		"status":  "success",
		"message": "Demo database reset successfully",
	})
}

// DemoStatusResponse represents the demo data status
type DemoStatusResponse struct {
	User              int          `json:"user"`
	Accounts          EntityStatus `json:"accounts"`
	Contacts          EntityStatus `json:"contacts"`
	Invoices          EntityStatus `json:"invoices"`
	Employees         EntityStatus `json:"employees"`
	Payments          EntityStatus `json:"payments"`
	JournalEntries    EntityStatus `json:"journalEntries"`
	BankAccounts      EntityStatus `json:"bankAccounts"`
	RecurringInvoices EntityStatus `json:"recurringInvoices"`
	PayrollRuns       EntityStatus `json:"payrollRuns"`
	TsdDeclarations   EntityStatus `json:"tsdDeclarations"`
}

// EntityStatus represents count and key identifiers for an entity type
type EntityStatus struct {
	Count int      `json:"count"`
	Keys  []string `json:"keys"`
}

// DemoStatus returns counts and key identifiers for demo data verification
// @Summary Get demo data status
// @Description Get counts and key identifiers for demo data verification
// @Tags Demo
// @Produce json
// @Param user query int true "Demo user number (1-3)"
// @Param X-Demo-Secret header string true "Demo secret key"
// @Success 200 {object} DemoStatusResponse
// @Failure 400 {object} object{error=string}
// @Failure 401 {object} object{error=string}
// @Failure 403 {object} object{error=string}
// @Router /api/demo/status [get]
func (h *Handlers) DemoStatus(w http.ResponseWriter, r *http.Request) {
	// Check if demo mode is enabled
	if os.Getenv("DEMO_MODE") != "true" {
		respondError(w, http.StatusForbidden, "Demo mode is not enabled")
		return
	}

	// Validate secret key
	secret := os.Getenv("DEMO_RESET_SECRET")
	if secret == "" {
		respondError(w, http.StatusForbidden, "Demo status not configured")
		return
	}

	providedSecret := r.Header.Get("X-Demo-Secret")
	if providedSecret != secret {
		respondError(w, http.StatusUnauthorized, "Invalid or missing secret key")
		return
	}

	// Parse required user parameter
	userParam := r.URL.Query().Get("user")
	if userParam == "" {
		respondError(w, http.StatusBadRequest, "User parameter is required")
		return
	}

	userNum, err := strconv.Atoi(userParam)
	if err != nil || userNum < 1 || userNum > 4 {
		respondError(w, http.StatusBadRequest, "Invalid user parameter. Must be 1, 2, 3, or 4")
		return
	}

	schema := fmt.Sprintf("tenant_demo%d", userNum)
	ctx := r.Context()

	response := DemoStatusResponse{User: userNum}

	// Query each entity count and keys
	response.Accounts = h.getEntityStatus(ctx, schema, "accounts", "name")
	response.Contacts = h.getEntityStatus(ctx, schema, "contacts", "name")
	response.Invoices = h.getEntityStatus(ctx, schema, "invoices", "invoice_number")
	response.Employees = h.getEntityStatusConcat(ctx, schema, "employees", "first_name", "last_name")
	response.Payments = h.getEntityStatus(ctx, schema, "payments", "payment_number")
	response.JournalEntries = h.getEntityStatus(ctx, schema, "journal_entries", "entry_number")
	response.BankAccounts = h.getEntityStatus(ctx, schema, "bank_accounts", "name")
	response.RecurringInvoices = h.getEntityStatus(ctx, schema, "recurring_invoices", "name")
	response.PayrollRuns = h.getEntityStatusPeriod(ctx, schema, "payroll_runs")
	response.TsdDeclarations = h.getEntityStatusPeriod(ctx, schema, "tsd_declarations")

	respondJSON(w, http.StatusOK, response)
}

func (h *Handlers) getEntityStatus(ctx context.Context, schema, table, keyColumn string) EntityStatus {
	var count int
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s.%s", schema, table)
	_ = h.pool.QueryRow(ctx, query).Scan(&count)

	var keys []string
	keysQuery := fmt.Sprintf("SELECT %s FROM %s.%s ORDER BY %s LIMIT 10", keyColumn, schema, table, keyColumn)
	rows, _ := h.pool.Query(ctx, keysQuery)
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var key string
			if rows.Scan(&key) == nil {
				keys = append(keys, key)
			}
		}
	}

	return EntityStatus{Count: count, Keys: keys}
}

func (h *Handlers) getEntityStatusConcat(ctx context.Context, schema, table, col1, col2 string) EntityStatus {
	var count int
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s.%s", schema, table)
	_ = h.pool.QueryRow(ctx, query).Scan(&count)

	var keys []string
	keysQuery := fmt.Sprintf("SELECT %s || ' ' || %s FROM %s.%s ORDER BY %s LIMIT 10", col1, col2, schema, table, col1)
	rows, _ := h.pool.Query(ctx, keysQuery)
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var key string
			if rows.Scan(&key) == nil {
				keys = append(keys, key)
			}
		}
	}

	return EntityStatus{Count: count, Keys: keys}
}

func (h *Handlers) getEntityStatusPeriod(ctx context.Context, schema, table string) EntityStatus {
	var count int
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s.%s", schema, table)
	_ = h.pool.QueryRow(ctx, query).Scan(&count)

	var keys []string
	keysQuery := fmt.Sprintf("SELECT period_year || '-' || LPAD(period_month::text, 2, '0') FROM %s.%s ORDER BY period_year, period_month LIMIT 10", schema, table)
	rows, _ := h.pool.Query(ctx, keysQuery)
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var key string
			if rows.Scan(&key) == nil {
				keys = append(keys, key)
			}
		}
	}

	return EntityStatus{Count: count, Keys: keys}
}

// getDemoSeedSQL returns the SQL to seed the demo database for all 4 demo users
// This creates demo users, tenants, schemas, and comprehensive sample data
func getDemoSeedSQL() string {
	var sql strings.Builder
	template := getDemoSeedTemplate()

	// Generate seed data for all 4 demo users (demo1-4)
	for userNum := 1; userNum <= 4; userNum++ {
		sql.WriteString(generateDemoSeedForUser(template, userNum))
	}

	return sql.String()
}

// generateDemoSeedForUser adapts the template for a specific user number
func generateDemoSeedForUser(template string, userNum int) string {
	n := fmt.Sprintf("%d", userNum)

	// Replace identifiers
	result := strings.ReplaceAll(template, "demo@example.com", fmt.Sprintf("demo%s@example.com", n))
	result = strings.ReplaceAll(result, "'acme'", fmt.Sprintf("'demo%s'", n))
	result = strings.ReplaceAll(result, "tenant_acme", fmt.Sprintf("tenant_demo%s", n))
	result = strings.ReplaceAll(result, "Acme Corporation", fmt.Sprintf("Demo Company %s", n))
	result = strings.ReplaceAll(result, "@acme.ee", fmt.Sprintf("@demo%s.example.com", n))
	result = strings.ReplaceAll(result, "info@acme.example.com", fmt.Sprintf("info@demo%s.example.com", n))

	// Replace UUID prefixes to ensure uniqueness per user
	// User ID prefix
	result = strings.ReplaceAll(result, "a0000000-0000-0000-0000-", fmt.Sprintf("a0000000-0000-0000-000%s-", n))
	// Tenant ID prefix
	result = strings.ReplaceAll(result, "b0000000-0000-0000-0000-", fmt.Sprintf("b0000000-0000-0000-000%s-", n))
	// Account IDs
	result = strings.ReplaceAll(result, "c0000000-0000-0000-", fmt.Sprintf("c%s000000-0000-0000-", n))
	// Contact IDs
	result = strings.ReplaceAll(result, "d0000000-0000-0000-", fmt.Sprintf("d%s000000-0000-0000-", n))
	// Invoice IDs
	result = strings.ReplaceAll(result, "e0000000-0000-0000-", fmt.Sprintf("e%s000000-0000-0000-", n))
	// Payment IDs
	result = strings.ReplaceAll(result, "f0000000-0000-0000-", fmt.Sprintf("f%s000000-0000-0000-", n))
	// Employee IDs (70-79)
	result = strings.ReplaceAll(result, "70000000-0000-0000-", fmt.Sprintf("7%s000000-0000-0000-", n))
	result = strings.ReplaceAll(result, "71000000-0000-0000-", fmt.Sprintf("71%s00000-0000-0000-", n))
	result = strings.ReplaceAll(result, "72000000-0000-0000-", fmt.Sprintf("72%s00000-0000-0000-", n))
	result = strings.ReplaceAll(result, "73000000-0000-0000-", fmt.Sprintf("73%s00000-0000-0000-", n))
	result = strings.ReplaceAll(result, "74000000-0000-0000-", fmt.Sprintf("74%s00000-0000-0000-", n))
	result = strings.ReplaceAll(result, "75000000-0000-0000-", fmt.Sprintf("75%s00000-0000-0000-", n))
	result = strings.ReplaceAll(result, "76000000-0000-0000-", fmt.Sprintf("76%s00000-0000-0000-", n))
	result = strings.ReplaceAll(result, "77000000-0000-0000-", fmt.Sprintf("77%s00000-0000-0000-", n))
	result = strings.ReplaceAll(result, "78000000-0000-0000-", fmt.Sprintf("78%s00000-0000-0000-", n))
	result = strings.ReplaceAll(result, "79000000-0000-0000-", fmt.Sprintf("79%s00000-0000-0000-", n))
	// Fiscal year and bank account IDs
	result = strings.ReplaceAll(result, "80000000-0000-0000-", fmt.Sprintf("8%s000000-0000-0000-", n))
	result = strings.ReplaceAll(result, "90000000-0000-0000-", fmt.Sprintf("9%s000000-0000-0000-", n))

	// Make invoice numbers unique per user
	result = strings.ReplaceAll(result, "INV-2024-", fmt.Sprintf("INV%s-2024-", n))
	result = strings.ReplaceAll(result, "INV-2025-", fmt.Sprintf("INV%s-2025-", n))
	result = strings.ReplaceAll(result, "PAY-2024-", fmt.Sprintf("PAY%s-2024-", n))
	result = strings.ReplaceAll(result, "JE-2024-", fmt.Sprintf("JE%s-2024-", n))

	return result
}

// getDemoSeedTemplate returns the SQL template for seeding one demo user
func getDemoSeedTemplate() string {
	return `
-- Demo User (password: demo12345)
INSERT INTO users (id, email, password_hash, name, is_active)
VALUES (
    'a0000000-0000-0000-0000-000000000001'::uuid,
    'demo@example.com',
    '$2a$10$NDz5VvAjksvnHzAq1p892.rZedeCGsy08iEiYzMUWcudFe7XH08pi',
    'Demo User',
    true
) ON CONFLICT (email) DO NOTHING;

-- Demo Tenant
INSERT INTO tenants (id, name, slug, schema_name, settings, is_active)
VALUES (
    'b0000000-0000-0000-0000-000000000001'::uuid,
    'Acme Corporation',
    'acme',
    'tenant_acme',
    '{
        "reg_code": "12345678",
        "vat_number": "EE123456789",
        "address": "Viru väljak 2, 10111 Tallinn",
        "email": "info@acme.example.com",
        "phone": "+372 5123 4567",
        "bank_details": "Swedbank EE123456789012345678",
        "invoice_prefix": "INV-",
        "invoice_footer": "Thank you for your business!",
        "default_payment_terms": 14,
        "pdf_primary_color": "#4f46e5"
    }'::jsonb,
    true
) ON CONFLICT (slug) DO NOTHING;

-- Mark onboarding as complete (column added in migration 009, safe to fail if column missing)
DO $$ BEGIN
    UPDATE tenants SET onboarding_completed = true WHERE id = 'b0000000-0000-0000-0000-000000000001'::uuid;
EXCEPTION WHEN undefined_column THEN
    NULL;
END $$;

-- Link demo user to tenant
INSERT INTO tenant_users (tenant_id, user_id, role, is_default)
VALUES (
    'b0000000-0000-0000-0000-000000000001'::uuid,
    'a0000000-0000-0000-0000-000000000001'::uuid,
    'admin',
    true
) ON CONFLICT (tenant_id, user_id) DO NOTHING;

-- Create tenant schema with all tables
SELECT create_tenant_schema('tenant_acme');

-- Add tables from later migrations
SELECT add_recurring_tables_to_schema('tenant_acme');
SELECT fix_recurring_invoices_schema('tenant_acme');
SELECT add_email_tables_to_schema('tenant_acme');
SELECT add_reconciliation_tables_to_schema('tenant_acme');
SELECT add_payroll_tables('tenant_acme');
SELECT add_recurring_email_fields_to_schema('tenant_acme');

-- Chart of Accounts (Estonian standard - 28 accounts)
INSERT INTO tenant_acme.accounts (id, tenant_id, code, name, account_type, is_system) VALUES
-- Assets (1xxx)
('c0000000-0000-0000-0001-000000000001'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, '1000', 'Cash', 'ASSET', true),
('c0000000-0000-0000-0001-000000000002'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, '1100', 'Bank Account - EUR', 'ASSET', true),
('c0000000-0000-0000-0001-000000000003'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, '1200', 'Accounts Receivable', 'ASSET', true),
('c0000000-0000-0000-0001-000000000004'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, '1300', 'Inventory', 'ASSET', false),
('c0000000-0000-0000-0001-000000000005'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, '1500', 'Prepaid Expenses', 'ASSET', false),
('c0000000-0000-0000-0001-000000000006'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, '1600', 'Fixed Assets', 'ASSET', false),
('c0000000-0000-0000-0001-000000000007'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, '1700', 'Accumulated Depreciation', 'ASSET', false),
-- Liabilities (2xxx)
('c0000000-0000-0000-0002-000000000001'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, '2000', 'Accounts Payable', 'LIABILITY', true),
('c0000000-0000-0000-0002-000000000002'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, '2100', 'VAT Payable', 'LIABILITY', true),
('c0000000-0000-0000-0002-000000000003'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, '2200', 'Income Tax Payable', 'LIABILITY', true),
('c0000000-0000-0000-0002-000000000004'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, '2300', 'Social Tax Payable', 'LIABILITY', true),
('c0000000-0000-0000-0002-000000000005'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, '2400', 'Salaries Payable', 'LIABILITY', true),
('c0000000-0000-0000-0002-000000000006'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, '2500', 'Pension Fund Payable', 'LIABILITY', true),
('c0000000-0000-0000-0002-000000000007'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, '2600', 'Unemployment Insurance Payable', 'LIABILITY', true),
('c0000000-0000-0000-0002-000000000008'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, '2900', 'Other Liabilities', 'LIABILITY', false),
-- Equity (3xxx)
('c0000000-0000-0000-0003-000000000001'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, '3000', 'Share Capital', 'EQUITY', true),
('c0000000-0000-0000-0003-000000000002'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, '3100', 'Retained Earnings', 'EQUITY', true),
('c0000000-0000-0000-0003-000000000003'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, '3200', 'Current Year Earnings', 'EQUITY', true),
-- Revenue (4xxx)
('c0000000-0000-0000-0004-000000000001'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, '4000', 'Sales Revenue', 'REVENUE', true),
('c0000000-0000-0000-0004-000000000002'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, '4100', 'Service Revenue', 'REVENUE', true),
('c0000000-0000-0000-0004-000000000003'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, '4900', 'Other Income', 'REVENUE', false),
-- Expenses (5xxx-7xxx)
('c0000000-0000-0000-0005-000000000001'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, '5000', 'Cost of Goods Sold', 'EXPENSE', true),
('c0000000-0000-0000-0005-000000000002'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, '6000', 'Salaries Expense', 'EXPENSE', true),
('c0000000-0000-0000-0005-000000000003'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, '6100', 'Social Tax Expense', 'EXPENSE', true),
('c0000000-0000-0000-0005-000000000004'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, '6200', 'Rent Expense', 'EXPENSE', false),
('c0000000-0000-0000-0005-000000000005'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, '6300', 'Utilities Expense', 'EXPENSE', false),
('c0000000-0000-0000-0005-000000000006'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, '6400', 'Office Supplies', 'EXPENSE', false),
('c0000000-0000-0000-0005-000000000007'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, '6500', 'Marketing Expense', 'EXPENSE', false),
('c0000000-0000-0000-0005-000000000008'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, '6600', 'Travel Expense', 'EXPENSE', false),
('c0000000-0000-0000-0005-000000000009'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, '6700', 'Professional Services', 'EXPENSE', false),
('c0000000-0000-0000-0005-000000000010'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, '6800', 'Depreciation Expense', 'EXPENSE', false),
('c0000000-0000-0000-0005-000000000011'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, '6900', 'Other Expenses', 'EXPENSE', false),
('c0000000-0000-0000-0005-000000000012'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, '7000', 'Bank Fees', 'EXPENSE', false)
ON CONFLICT DO NOTHING;

-- Contacts (7 total: 4 customers, 3 suppliers)
INSERT INTO tenant_acme.contacts (id, tenant_id, code, name, contact_type, reg_code, vat_number, email, phone, address_line1, city, postal_code, country_code, payment_terms_days) VALUES
-- Customers
('d0000000-0000-0000-0001-000000000001'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, 'C001', 'TechStart OÜ', 'CUSTOMER', '14567890', 'EE145678901', 'info@techstart.ee', '+372 5234 5678', 'Pärnu mnt 15', 'Tallinn', '10141', 'EE', 14),
('d0000000-0000-0000-0001-000000000002'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, 'C002', 'Nordic Solutions AS', 'CUSTOMER', '98765432', 'EE987654321', 'orders@nordic.ee', '+372 5345 6789', 'Tartu mnt 83', 'Tallinn', '10115', 'EE', 30),
('d0000000-0000-0000-0001-000000000003'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, 'C003', 'Baltic Commerce', 'CUSTOMER', '11223344', 'EE112233445', 'accounting@baltic.ee', '+372 5456 7890', 'Narva mnt 7', 'Tallinn', '10117', 'EE', 14),
('d0000000-0000-0000-0001-000000000004'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, 'C004', 'GreenTech Industries', 'CUSTOMER', '55667788', 'EE556677889', 'finance@greentech.ee', '+372 5567 8901', 'Lõõtsa 5', 'Tallinn', '11415', 'EE', 21),
-- Suppliers
('d0000000-0000-0000-0002-000000000001'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, 'S001', 'Office Supplies Ltd', 'SUPPLIER', '33445566', NULL, 'orders@officesupplies.ee', '+372 5678 9012', 'Peterburi tee 71', 'Tallinn', '11415', 'EE', 30),
('d0000000-0000-0000-0002-000000000002'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, 'S002', 'CloudHost Services', 'SUPPLIER', '44556677', 'EE445566778', 'billing@cloudhost.ee', '+372 5789 0123', 'Ülemiste tee 5', 'Tallinn', '11415', 'EE', 14),
('d0000000-0000-0000-0002-000000000003'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, 'S003', 'Marketing Agency OÜ', 'SUPPLIER', '77889900', 'EE778899001', 'invoices@marketing.ee', '+372 5890 1234', 'Telliskivi 60a', 'Tallinn', '10412', 'EE', 14)
ON CONFLICT DO NOTHING;

-- Invoices (9 total with various statuses)
INSERT INTO tenant_acme.invoices (id, tenant_id, invoice_number, invoice_type, contact_id, issue_date, due_date, subtotal, vat_amount, total, base_subtotal, base_vat_amount, base_total, amount_paid, status, created_by) VALUES
-- Paid invoices
('e0000000-0000-0000-0001-000000000001'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, 'INV-2024-001', 'SALES', 'd0000000-0000-0000-0001-000000000001'::uuid, '2024-11-01', '2024-11-15', 2500.00, 550.00, 3050.00, 2500.00, 550.00, 3050.00, 3050.00, 'PAID', 'a0000000-0000-0000-0000-000000000001'::uuid),
('e0000000-0000-0000-0001-000000000002'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, 'INV-2024-002', 'SALES', 'd0000000-0000-0000-0001-000000000002'::uuid, '2024-11-05', '2024-12-05', 8750.00, 1925.00, 10675.00, 8750.00, 1925.00, 10675.00, 10675.00, 'PAID', 'a0000000-0000-0000-0000-000000000001'::uuid),
('e0000000-0000-0000-0001-000000000003'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, 'INV-2024-003', 'SALES', 'd0000000-0000-0000-0001-000000000003'::uuid, '2024-11-10', '2024-11-24', 1200.00, 264.00, 1464.00, 1200.00, 264.00, 1464.00, 1464.00, 'PAID', 'a0000000-0000-0000-0000-000000000001'::uuid),
-- Sent/Outstanding invoices
('e0000000-0000-0000-0001-000000000004'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, 'INV-2024-004', 'SALES', 'd0000000-0000-0000-0001-000000000001'::uuid, '2024-12-01', '2024-12-15', 3200.00, 704.00, 3904.00, 3200.00, 704.00, 3904.00, 0.00, 'SENT', 'a0000000-0000-0000-0000-000000000001'::uuid),
('e0000000-0000-0000-0001-000000000005'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, 'INV-2024-005', 'SALES', 'd0000000-0000-0000-0001-000000000004'::uuid, '2024-12-10', '2024-12-31', 5500.00, 1210.00, 6710.00, 5500.00, 1210.00, 6710.00, 0.00, 'SENT', 'a0000000-0000-0000-0000-000000000001'::uuid),
-- Partially paid
('e0000000-0000-0000-0001-000000000006'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, 'INV-2024-006', 'SALES', 'd0000000-0000-0000-0001-000000000002'::uuid, '2024-12-05', '2025-01-04', 12000.00, 2640.00, 14640.00, 12000.00, 2640.00, 14640.00, 7000.00, 'PARTIALLY_PAID', 'a0000000-0000-0000-0000-000000000001'::uuid),
-- Draft invoice
('e0000000-0000-0000-0001-000000000007'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, 'INV-2024-007', 'SALES', 'd0000000-0000-0000-0001-000000000003'::uuid, '2024-12-20', '2025-01-03', 4800.00, 1056.00, 5856.00, 4800.00, 1056.00, 5856.00, 0.00, 'DRAFT', 'a0000000-0000-0000-0000-000000000001'::uuid),
-- Current month invoices
('e0000000-0000-0000-0001-000000000008'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, 'INV-2025-001', 'SALES', 'd0000000-0000-0000-0001-000000000001'::uuid, CURRENT_DATE - INTERVAL '5 days', CURRENT_DATE + INTERVAL '9 days', 1850.00, 407.00, 2257.00, 1850.00, 407.00, 2257.00, 0.00, 'SENT', 'a0000000-0000-0000-0000-000000000001'::uuid),
('e0000000-0000-0000-0001-000000000009'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, 'INV-2025-002', 'SALES', 'd0000000-0000-0000-0001-000000000004'::uuid, CURRENT_DATE - INTERVAL '2 days', CURRENT_DATE + INTERVAL '12 days', 6200.00, 1364.00, 7564.00, 6200.00, 1364.00, 7564.00, 0.00, 'SENT', 'a0000000-0000-0000-0000-000000000001'::uuid)
ON CONFLICT DO NOTHING;

-- Invoice Lines
INSERT INTO tenant_acme.invoice_lines (tenant_id, invoice_id, line_number, description, quantity, unit, unit_price, vat_rate, line_subtotal, line_vat, line_total, account_id) VALUES
('b0000000-0000-0000-0000-000000000001'::uuid, 'e0000000-0000-0000-0001-000000000001'::uuid, 1, 'Software Development Services - November', 50, 'hours', 50.00, 22.00, 2500.00, 550.00, 3050.00, 'c0000000-0000-0000-0004-000000000002'::uuid),
('b0000000-0000-0000-0000-000000000001'::uuid, 'e0000000-0000-0000-0001-000000000002'::uuid, 1, 'ERP Implementation - Phase 1', 1, 'project', 5000.00, 22.00, 5000.00, 1100.00, 6100.00, 'c0000000-0000-0000-0004-000000000002'::uuid),
('b0000000-0000-0000-0000-000000000001'::uuid, 'e0000000-0000-0000-0001-000000000002'::uuid, 2, 'Training & Documentation', 15, 'hours', 250.00, 22.00, 3750.00, 825.00, 4575.00, 'c0000000-0000-0000-0004-000000000002'::uuid),
('b0000000-0000-0000-0000-000000000001'::uuid, 'e0000000-0000-0000-0001-000000000003'::uuid, 1, 'Monthly Support Package', 1, 'month', 1200.00, 22.00, 1200.00, 264.00, 1464.00, 'c0000000-0000-0000-0004-000000000002'::uuid),
('b0000000-0000-0000-0000-000000000001'::uuid, 'e0000000-0000-0000-0001-000000000004'::uuid, 1, 'Custom Integration Development', 40, 'hours', 80.00, 22.00, 3200.00, 704.00, 3904.00, 'c0000000-0000-0000-0004-000000000002'::uuid),
('b0000000-0000-0000-0000-000000000001'::uuid, 'e0000000-0000-0000-0001-000000000005'::uuid, 1, 'Cloud Migration Services', 1, 'project', 4000.00, 22.00, 4000.00, 880.00, 4880.00, 'c0000000-0000-0000-0004-000000000002'::uuid),
('b0000000-0000-0000-0000-000000000001'::uuid, 'e0000000-0000-0000-0001-000000000005'::uuid, 2, 'Infrastructure Setup', 1, 'fixed', 1500.00, 22.00, 1500.00, 330.00, 1830.00, 'c0000000-0000-0000-0004-000000000002'::uuid),
('b0000000-0000-0000-0000-000000000001'::uuid, 'e0000000-0000-0000-0001-000000000006'::uuid, 1, 'Enterprise Software License', 12, 'months', 1000.00, 22.00, 12000.00, 2640.00, 14640.00, 'c0000000-0000-0000-0004-000000000001'::uuid),
('b0000000-0000-0000-0000-000000000001'::uuid, 'e0000000-0000-0000-0001-000000000007'::uuid, 1, 'API Development', 30, 'hours', 120.00, 22.00, 3600.00, 792.00, 4392.00, 'c0000000-0000-0000-0004-000000000002'::uuid),
('b0000000-0000-0000-0000-000000000001'::uuid, 'e0000000-0000-0000-0001-000000000007'::uuid, 2, 'Testing & QA', 10, 'hours', 120.00, 22.00, 1200.00, 264.00, 1464.00, 'c0000000-0000-0000-0004-000000000002'::uuid),
('b0000000-0000-0000-0000-000000000001'::uuid, 'e0000000-0000-0000-0001-000000000008'::uuid, 1, 'Consulting Services', 15, 'hours', 100.00, 22.00, 1500.00, 330.00, 1830.00, 'c0000000-0000-0000-0004-000000000002'::uuid),
('b0000000-0000-0000-0000-000000000001'::uuid, 'e0000000-0000-0000-0001-000000000008'::uuid, 2, 'Support Ticket Resolution', 5, 'tickets', 70.00, 22.00, 350.00, 77.00, 427.00, 'c0000000-0000-0000-0004-000000000002'::uuid),
('b0000000-0000-0000-0000-000000000001'::uuid, 'e0000000-0000-0000-0001-000000000009'::uuid, 1, 'Annual Maintenance Contract', 1, 'year', 6200.00, 22.00, 6200.00, 1364.00, 7564.00, 'c0000000-0000-0000-0004-000000000001'::uuid)
ON CONFLICT DO NOTHING;

-- Payments (4 total)
INSERT INTO tenant_acme.payments (id, tenant_id, payment_number, payment_type, contact_id, payment_date, amount, base_amount, payment_method, reference, created_by) VALUES
('f0000000-0000-0000-0001-000000000001'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, 'PAY-2024-001', 'RECEIVED', 'd0000000-0000-0000-0001-000000000001'::uuid, '2024-11-12', 3050.00, 3050.00, 'Bank Transfer', 'INV-2024-001', 'a0000000-0000-0000-0000-000000000001'::uuid),
('f0000000-0000-0000-0001-000000000002'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, 'PAY-2024-002', 'RECEIVED', 'd0000000-0000-0000-0001-000000000002'::uuid, '2024-11-28', 10675.00, 10675.00, 'Bank Transfer', 'INV-2024-002', 'a0000000-0000-0000-0000-000000000001'::uuid),
('f0000000-0000-0000-0001-000000000003'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, 'PAY-2024-003', 'RECEIVED', 'd0000000-0000-0000-0001-000000000003'::uuid, '2024-11-22', 1464.00, 1464.00, 'Bank Transfer', 'INV-2024-003', 'a0000000-0000-0000-0000-000000000001'::uuid),
('f0000000-0000-0000-0001-000000000004'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, 'PAY-2024-004', 'RECEIVED', 'd0000000-0000-0000-0001-000000000002'::uuid, '2024-12-15', 7000.00, 7000.00, 'Bank Transfer', 'Partial payment INV-2024-006', 'a0000000-0000-0000-0000-000000000001'::uuid)
ON CONFLICT DO NOTHING;

-- Fiscal Years
INSERT INTO tenant_acme.fiscal_years (id, tenant_id, name, start_date, end_date, is_closed) VALUES
('90000000-0000-0000-0001-000000000001'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, 'FY 2024', '2024-01-01', '2024-12-31', false),
('90000000-0000-0000-0001-000000000002'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, 'FY 2025', '2025-01-01', '2025-12-31', false)
ON CONFLICT DO NOTHING;

-- Bank Accounts (2 total)
INSERT INTO tenant_acme.bank_accounts (id, tenant_id, name, account_number, bank_name, currency, is_active) VALUES
('80000000-0000-0000-0001-000000000001'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, 'Main EUR Account', 'EE123456789012345678', 'Swedbank', 'EUR', true),
('80000000-0000-0000-0001-000000000002'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, 'Savings Account', 'EE987654321098765432', 'SEB', 'EUR', true)
ON CONFLICT DO NOTHING;

-- Employees (5 total)
INSERT INTO tenant_acme.employees (id, tenant_id, employee_number, first_name, last_name, personal_code, email, phone, address, bank_account, start_date, end_date, position, department, employment_type, tax_residency, apply_basic_exemption, basic_exemption_amount, funded_pension_rate, is_active) VALUES
('70000000-0000-0000-0001-000000000001'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, 'EMP001', 'Maria', 'Tamm', '49001010001', 'maria.tamm@acme.ee', '+372 5111 2222', 'Liivalaia 33-15, Tallinn', 'EE382200221020145678', '2023-01-15', NULL, 'Software Developer', 'Engineering', 'FULL_TIME', 'EE', true, 700.00, 2.00, true),
('70000000-0000-0000-0001-000000000002'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, 'EMP002', 'Jaan', 'Kask', '38505050002', 'jaan.kask@acme.ee', '+372 5222 3333', 'Pärnu mnt 45-8, Tallinn', 'EE382200221020156789', '2022-06-01', NULL, 'Project Manager', 'Management', 'FULL_TIME', 'EE', true, 700.00, 4.00, true),
('70000000-0000-0000-0001-000000000003'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, 'EMP003', 'Anna', 'Mets', '49503030003', 'anna.mets@acme.ee', '+372 5333 4444', 'Tartu mnt 12-3, Tallinn', 'EE382200221020167890', '2024-03-01', NULL, 'UX Designer', 'Design', 'FULL_TIME', 'EE', true, 700.00, 0.00, true),
('70000000-0000-0000-0001-000000000004'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, 'EMP004', 'Peeter', 'Saar', '37801010004', 'peeter.saar@acme.ee', '+372 5444 5555', 'Mustamäe tee 5-22, Tallinn', 'EE382200221020178901', '2021-09-15', NULL, 'Senior Developer', 'Engineering', 'FULL_TIME', 'EE', true, 700.00, 2.00, true),
('70000000-0000-0000-0001-000000000005'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, 'EMP005', 'Liisa', 'Kivi', '49207070005', 'liisa.kivi@acme.ee', '+372 5555 6666', 'Kadaka tee 88-5, Tallinn', 'EE382200221020189012', '2024-01-02', '2024-08-31', 'Intern', 'Engineering', 'PART_TIME', 'EE', false, 0.00, 0.00, false)
ON CONFLICT DO NOTHING;

-- Salary Components
INSERT INTO tenant_acme.salary_components (id, tenant_id, employee_id, component_type, name, amount, is_taxable, is_recurring, effective_from, effective_to) VALUES
('71000000-0000-0000-0001-000000000001'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, '70000000-0000-0000-0001-000000000001'::uuid, 'BASE_SALARY', 'Monthly Salary', 3500.00, true, true, '2023-01-15', NULL),
('71000000-0000-0000-0001-000000000002'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, '70000000-0000-0000-0001-000000000002'::uuid, 'BASE_SALARY', 'Monthly Salary', 4200.00, true, true, '2022-06-01', NULL),
('71000000-0000-0000-0001-000000000003'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, '70000000-0000-0000-0001-000000000002'::uuid, 'BONUS', 'Management Bonus', 500.00, true, true, '2024-01-01', NULL),
('71000000-0000-0000-0001-000000000004'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, '70000000-0000-0000-0001-000000000003'::uuid, 'BASE_SALARY', 'Monthly Salary', 2800.00, true, true, '2024-03-01', NULL),
('71000000-0000-0000-0001-000000000005'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, '70000000-0000-0000-0001-000000000004'::uuid, 'BASE_SALARY', 'Monthly Salary', 4800.00, true, true, '2021-09-15', NULL)
ON CONFLICT DO NOTHING;

-- Payroll Runs (3 total)
INSERT INTO tenant_acme.payroll_runs (id, tenant_id, period_year, period_month, status, payment_date, total_gross, total_net, total_employer_cost, notes, created_by) VALUES
('72000000-0000-0000-0001-000000000001'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, 2024, 10, 'PAID', '2024-11-05', 15800.00, 11034.40, 21169.68, 'October 2024 payroll', 'a0000000-0000-0000-0000-000000000001'::uuid),
('72000000-0000-0000-0001-000000000002'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, 2024, 11, 'PAID', '2024-12-05', 15800.00, 11034.40, 21169.68, 'November 2024 payroll', 'a0000000-0000-0000-0000-000000000001'::uuid),
('72000000-0000-0000-0001-000000000003'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, 2024, 12, 'APPROVED', '2025-01-05', 15800.00, 11034.40, 21169.68, 'December 2024 payroll', 'a0000000-0000-0000-0000-000000000001'::uuid)
ON CONFLICT DO NOTHING;

-- Payslips (12 total - 4 employees x 3 months)
INSERT INTO tenant_acme.payslips (id, tenant_id, payroll_run_id, employee_id, gross_salary, taxable_income, income_tax, unemployment_insurance_employee, funded_pension, other_deductions, net_salary, social_tax, unemployment_insurance_employer, total_employer_cost, basic_exemption_applied, payment_status) VALUES
-- October 2024
('73000000-0000-0000-0001-000000000001'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, '72000000-0000-0000-0001-000000000001'::uuid, '70000000-0000-0000-0001-000000000001'::uuid, 3500.00, 2800.00, 616.00, 56.00, 70.00, 0.00, 2758.00, 1155.00, 28.00, 4683.00, 700.00, 'PAID'),
('73000000-0000-0000-0001-000000000002'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, '72000000-0000-0000-0001-000000000001'::uuid, '70000000-0000-0000-0001-000000000002'::uuid, 4700.00, 4000.00, 880.00, 75.20, 188.00, 0.00, 3556.80, 1551.00, 37.60, 6288.60, 700.00, 'PAID'),
('73000000-0000-0000-0001-000000000003'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, '72000000-0000-0000-0001-000000000001'::uuid, '70000000-0000-0000-0001-000000000003'::uuid, 2800.00, 2100.00, 462.00, 44.80, 0.00, 0.00, 2293.20, 924.00, 22.40, 3746.40, 700.00, 'PAID'),
('73000000-0000-0000-0001-000000000004'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, '72000000-0000-0000-0001-000000000001'::uuid, '70000000-0000-0000-0001-000000000004'::uuid, 4800.00, 4100.00, 902.00, 76.80, 96.00, 0.00, 3725.20, 1584.00, 38.40, 6422.40, 700.00, 'PAID'),
-- November 2024
('73000000-0000-0000-0001-000000000005'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, '72000000-0000-0000-0001-000000000002'::uuid, '70000000-0000-0000-0001-000000000001'::uuid, 3500.00, 2800.00, 616.00, 56.00, 70.00, 0.00, 2758.00, 1155.00, 28.00, 4683.00, 700.00, 'PAID'),
('73000000-0000-0000-0001-000000000006'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, '72000000-0000-0000-0001-000000000002'::uuid, '70000000-0000-0000-0001-000000000002'::uuid, 4700.00, 4000.00, 880.00, 75.20, 188.00, 0.00, 3556.80, 1551.00, 37.60, 6288.60, 700.00, 'PAID'),
('73000000-0000-0000-0001-000000000007'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, '72000000-0000-0000-0001-000000000002'::uuid, '70000000-0000-0000-0001-000000000003'::uuid, 2800.00, 2100.00, 462.00, 44.80, 0.00, 0.00, 2293.20, 924.00, 22.40, 3746.40, 700.00, 'PAID'),
('73000000-0000-0000-0001-000000000008'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, '72000000-0000-0000-0001-000000000002'::uuid, '70000000-0000-0000-0001-000000000004'::uuid, 4800.00, 4100.00, 902.00, 76.80, 96.00, 0.00, 3725.20, 1584.00, 38.40, 6422.40, 700.00, 'PAID'),
-- December 2024
('73000000-0000-0000-0001-000000000009'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, '72000000-0000-0000-0001-000000000003'::uuid, '70000000-0000-0000-0001-000000000001'::uuid, 3500.00, 2800.00, 616.00, 56.00, 70.00, 0.00, 2758.00, 1155.00, 28.00, 4683.00, 700.00, 'PENDING'),
('73000000-0000-0000-0001-000000000010'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, '72000000-0000-0000-0001-000000000003'::uuid, '70000000-0000-0000-0001-000000000002'::uuid, 4700.00, 4000.00, 880.00, 75.20, 188.00, 0.00, 3556.80, 1551.00, 37.60, 6288.60, 700.00, 'PENDING'),
('73000000-0000-0000-0001-000000000011'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, '72000000-0000-0000-0001-000000000003'::uuid, '70000000-0000-0000-0001-000000000003'::uuid, 2800.00, 2100.00, 462.00, 44.80, 0.00, 0.00, 2293.20, 924.00, 22.40, 3746.40, 700.00, 'PENDING'),
('73000000-0000-0000-0001-000000000012'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, '72000000-0000-0000-0001-000000000003'::uuid, '70000000-0000-0000-0001-000000000004'::uuid, 4800.00, 4100.00, 902.00, 76.80, 96.00, 0.00, 3725.20, 1584.00, 38.40, 6422.40, 700.00, 'PENDING')
ON CONFLICT DO NOTHING;

-- TSD Declarations (3 total)
INSERT INTO tenant_acme.tsd_declarations (id, tenant_id, period_year, period_month, payroll_run_id, total_payments, total_income_tax, total_social_tax, total_unemployment_employer, total_unemployment_employee, total_funded_pension, status) VALUES
('74000000-0000-0000-0001-000000000001'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, 2024, 10, '72000000-0000-0000-0001-000000000001'::uuid, 15800.00, 2860.00, 5214.00, 126.40, 252.80, 354.00, 'SUBMITTED'),
('74000000-0000-0000-0001-000000000002'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, 2024, 11, '72000000-0000-0000-0001-000000000002'::uuid, 15800.00, 2860.00, 5214.00, 126.40, 252.80, 354.00, 'SUBMITTED'),
('74000000-0000-0000-0001-000000000003'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, 2024, 12, '72000000-0000-0000-0001-000000000003'::uuid, 15800.00, 2860.00, 5214.00, 126.40, 252.80, 354.00, 'DRAFT')
ON CONFLICT DO NOTHING;

-- Recurring Invoices (3 total)
INSERT INTO tenant_acme.recurring_invoices (id, tenant_id, name, contact_id, invoice_type, frequency, start_date, end_date, next_generation_date, payment_terms_days, currency, notes, is_active, last_generated_at, generated_count, created_by) VALUES
('75000000-0000-0000-0001-000000000001'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, 'Monthly Support - TechStart', 'd0000000-0000-0000-0001-000000000001'::uuid, 'SALES', 'MONTHLY', '2024-01-01', '2024-12-31', '2025-01-01', 14, 'EUR', 'Monthly IT support package', true, '2024-12-01', 12, 'a0000000-0000-0000-0000-000000000001'::uuid),
('75000000-0000-0000-0001-000000000002'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, 'Quarterly Retainer - Nordic', 'd0000000-0000-0000-0001-000000000002'::uuid, 'SALES', 'QUARTERLY', '2024-01-01', NULL, '2025-01-01', 30, 'EUR', 'Quarterly consulting retainer', true, '2024-10-01', 4, 'a0000000-0000-0000-0000-000000000001'::uuid),
('75000000-0000-0000-0001-000000000003'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, 'Annual License - GreenTech', 'd0000000-0000-0000-0001-000000000004'::uuid, 'SALES', 'YEARLY', '2024-06-01', NULL, '2025-06-01', 30, 'EUR', 'Annual software license', true, '2024-06-01', 1, 'a0000000-0000-0000-0000-000000000001'::uuid)
ON CONFLICT DO NOTHING;

INSERT INTO tenant_acme.recurring_invoice_lines (id, tenant_id, recurring_invoice_id, line_number, description, quantity, unit_price, vat_rate, account_id) VALUES
('76000000-0000-0000-0001-000000000001'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, '75000000-0000-0000-0001-000000000001'::uuid, 1, 'IT Support Package - Standard', 1, 1200.00, 22.00, 'c0000000-0000-0000-0004-000000000002'::uuid),
('76000000-0000-0000-0001-000000000002'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, '75000000-0000-0000-0001-000000000002'::uuid, 1, 'Consulting Retainer - Q4', 1, 7500.00, 22.00, 'c0000000-0000-0000-0004-000000000002'::uuid),
('76000000-0000-0000-0001-000000000003'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, '75000000-0000-0000-0001-000000000003'::uuid, 1, 'Enterprise Software License', 1, 12000.00, 22.00, 'c0000000-0000-0000-0004-000000000001'::uuid)
ON CONFLICT DO NOTHING;

-- Journal Entries (4 total)
INSERT INTO tenant_acme.journal_entries (id, tenant_id, entry_number, entry_date, description, reference, source_type, status, created_by) VALUES
('77000000-0000-0000-0001-000000000001'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, 'JE-2024-001', '2024-01-01', 'Opening balances', 'OB-2024', 'MANUAL', 'POSTED', 'a0000000-0000-0000-0000-000000000001'::uuid),
('77000000-0000-0000-0001-000000000002'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, 'JE-2024-002', '2024-11-30', 'Office rent November', 'RENT-NOV-24', 'MANUAL', 'POSTED', 'a0000000-0000-0000-0000-000000000001'::uuid),
('77000000-0000-0000-0001-000000000003'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, 'JE-2024-003', '2024-12-01', 'Depreciation December', 'DEP-DEC-24', 'MANUAL', 'POSTED', 'a0000000-0000-0000-0000-000000000001'::uuid),
('77000000-0000-0000-0001-000000000004'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, 'JE-2024-004', '2024-12-15', 'Utilities expense', 'UTIL-DEC-24', 'MANUAL', 'DRAFT', 'a0000000-0000-0000-0000-000000000001'::uuid)
ON CONFLICT DO NOTHING;

INSERT INTO tenant_acme.journal_entry_lines (id, tenant_id, journal_entry_id, account_id, description, debit_amount, credit_amount, currency, base_debit, base_credit) VALUES
('78000000-0000-0000-0001-000000000001'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, '77000000-0000-0000-0001-000000000001'::uuid, 'c0000000-0000-0000-0001-000000000002'::uuid, 'Bank opening balance', 50000.00, 0.00, 'EUR', 50000.00, 0.00),
('78000000-0000-0000-0001-000000000002'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, '77000000-0000-0000-0001-000000000001'::uuid, 'c0000000-0000-0000-0003-000000000001'::uuid, 'Share capital', 0.00, 50000.00, 'EUR', 0.00, 50000.00),
('78000000-0000-0000-0001-000000000003'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, '77000000-0000-0000-0001-000000000002'::uuid, 'c0000000-0000-0000-0005-000000000004'::uuid, 'Office rent', 2500.00, 0.00, 'EUR', 2500.00, 0.00),
('78000000-0000-0000-0001-000000000004'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, '77000000-0000-0000-0001-000000000002'::uuid, 'c0000000-0000-0000-0001-000000000002'::uuid, 'Rent payment', 0.00, 2500.00, 'EUR', 0.00, 2500.00),
('78000000-0000-0000-0001-000000000005'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, '77000000-0000-0000-0001-000000000003'::uuid, 'c0000000-0000-0000-0005-000000000010'::uuid, 'Monthly depreciation', 500.00, 0.00, 'EUR', 500.00, 0.00),
('78000000-0000-0000-0001-000000000006'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, '77000000-0000-0000-0001-000000000003'::uuid, 'c0000000-0000-0000-0001-000000000007'::uuid, 'Accumulated depreciation', 0.00, 500.00, 'EUR', 0.00, 500.00),
('78000000-0000-0000-0001-000000000007'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, '77000000-0000-0000-0001-000000000004'::uuid, 'c0000000-0000-0000-0005-000000000005'::uuid, 'Electricity and water', 350.00, 0.00, 'EUR', 350.00, 0.00),
('78000000-0000-0000-0001-000000000008'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, '77000000-0000-0000-0001-000000000004'::uuid, 'c0000000-0000-0000-0002-000000000001'::uuid, 'Utilities payable', 0.00, 350.00, 'EUR', 0.00, 350.00)
ON CONFLICT DO NOTHING;

-- Bank Transactions (8 total)
INSERT INTO tenant_acme.bank_transactions (id, tenant_id, bank_account_id, transaction_date, value_date, amount, currency, description, reference, counterparty_name, counterparty_account, status) VALUES
('79000000-0000-0000-0001-000000000001'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, '80000000-0000-0000-0001-000000000001'::uuid, '2024-11-12', '2024-11-12', 3050.00, 'EUR', 'Invoice payment INV-2024-001', 'INV-2024-001', 'TechStart OÜ', 'EE123456789012345679', 'MATCHED'),
('79000000-0000-0000-0001-000000000002'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, '80000000-0000-0000-0001-000000000001'::uuid, '2024-11-28', '2024-11-28', 10675.00, 'EUR', 'Invoice payment INV-2024-002', 'INV-2024-002', 'Nordic Solutions AS', 'EE987654321098765433', 'MATCHED'),
('79000000-0000-0000-0001-000000000003'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, '80000000-0000-0000-0001-000000000001'::uuid, '2024-11-22', '2024-11-22', 1464.00, 'EUR', 'Invoice payment INV-2024-003', 'INV-2024-003', 'Baltic Commerce', 'EE112233445566778899', 'MATCHED'),
('79000000-0000-0000-0001-000000000004'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, '80000000-0000-0000-0001-000000000001'::uuid, '2024-12-15', '2024-12-15', 7000.00, 'EUR', 'Partial payment INV-2024-006', 'INV-2024-006-P1', 'Nordic Solutions AS', 'EE987654321098765433', 'MATCHED'),
('79000000-0000-0000-0001-000000000005'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, '80000000-0000-0000-0001-000000000001'::uuid, '2024-11-30', '2024-11-30', -2500.00, 'EUR', 'Office rent November', 'RENT-NOV-24', 'Kinnisvara AS', 'EE111222333444555666', 'RECONCILED'),
('79000000-0000-0000-0001-000000000006'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, '80000000-0000-0000-0001-000000000001'::uuid, '2024-12-05', '2024-12-05', -11034.40, 'EUR', 'Salary payments Nov 2024', 'SAL-NOV-24', 'Multiple employees', NULL, 'RECONCILED'),
('79000000-0000-0000-0001-000000000007'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, '80000000-0000-0000-0001-000000000001'::uuid, '2024-12-20', '2024-12-20', 1500.00, 'EUR', 'Unknown deposit', 'REF-123456', 'Unknown sender', 'EE999888777666555444', 'UNMATCHED'),
('79000000-0000-0000-0001-000000000008'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, '80000000-0000-0000-0001-000000000001'::uuid, '2024-12-22', '2024-12-22', -75.50, 'EUR', 'Bank service fee', 'FEE-DEC-24', 'Swedbank', NULL, 'UNMATCHED')
ON CONFLICT DO NOTHING;
`
}
