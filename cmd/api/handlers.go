package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	"github.com/HMB-research/open-accounting/internal/accounting"
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
	// Check if demo mode is enabled
	if os.Getenv("DEMO_MODE") != "true" {
		respondError(w, http.StatusForbidden, "Demo mode is not enabled")
		return
	}

	// Validate secret key
	secret := os.Getenv("DEMO_RESET_SECRET")
	if secret == "" {
		respondError(w, http.StatusForbidden, "Demo reset not configured")
		return
	}

	providedSecret := r.Header.Get("X-Demo-Secret")
	if providedSecret == "" {
		providedSecret = r.URL.Query().Get("secret")
	}

	if providedSecret != secret {
		respondError(w, http.StatusUnauthorized, "Invalid or missing secret key")
		return
	}

	ctx := r.Context()

	// Demo user and tenant IDs (matching demo-seed.sql)
	demoUserID := "a0000000-0000-0000-0000-000000000001"
	demoTenantID := "b0000000-0000-0000-0000-000000000001"

	// Drop tenant schema
	_, err := h.pool.Exec(ctx, "DROP SCHEMA IF EXISTS tenant_acme CASCADE")
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to drop tenant schema: "+err.Error())
		return
	}

	// Delete demo data from public tables
	_, err = h.pool.Exec(ctx, "DELETE FROM tenant_users WHERE tenant_id = $1", demoTenantID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to clean tenant_users: "+err.Error())
		return
	}

	_, err = h.pool.Exec(ctx, "DELETE FROM tenants WHERE id = $1", demoTenantID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to clean tenants: "+err.Error())
		return
	}

	_, err = h.pool.Exec(ctx, "DELETE FROM users WHERE id = $1", demoUserID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to clean users: "+err.Error())
		return
	}

	// Re-seed demo data by executing the seed SQL
	// Note: In production, you might want to read from a file or embedded resource
	seedSQL := getDemoSeedSQL()
	_, err = h.pool.Exec(ctx, seedSQL)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to seed demo data: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"status":  "success",
		"message": "Demo database reset successfully",
	})
}

// getDemoSeedSQL returns the SQL to seed the demo database
// This creates the demo user, tenant, schema, and sample data
func getDemoSeedSQL() string {
	return `
-- Demo User (password: demo123)
INSERT INTO users (id, email, password_hash, name, is_active)
VALUES (
    'a0000000-0000-0000-0000-000000000001'::uuid,
    'demo@example.com',
    '$2a$10$m/V.x2rKPDFQj6xo9voXAeCXJBua.EzF.A3v1bnreBdgbAaKBcsTu',
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
        "pdf_primary_color": "#4f46e5",
        "onboarding_completed": true
    }'::jsonb,
    true
) ON CONFLICT (slug) DO NOTHING;

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

-- Chart of Accounts (Estonian standard)
INSERT INTO tenant_acme.accounts (id, tenant_id, code, name, account_type, is_system) VALUES
('c0000000-0000-0000-0001-000000000001'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, '1000', 'Cash', 'ASSET', true),
('c0000000-0000-0000-0001-000000000002'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, '1100', 'Bank Account - EUR', 'ASSET', true),
('c0000000-0000-0000-0001-000000000003'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, '1200', 'Accounts Receivable', 'ASSET', true),
('c0000000-0000-0000-0002-000000000001'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, '2000', 'Accounts Payable', 'LIABILITY', true),
('c0000000-0000-0000-0002-000000000002'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, '2100', 'VAT Payable', 'LIABILITY', true),
('c0000000-0000-0000-0003-000000000001'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, '3000', 'Share Capital', 'EQUITY', true),
('c0000000-0000-0000-0003-000000000002'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, '3100', 'Retained Earnings', 'EQUITY', true),
('c0000000-0000-0000-0004-000000000001'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, '4000', 'Sales Revenue', 'REVENUE', true),
('c0000000-0000-0000-0004-000000000002'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, '4100', 'Service Revenue', 'REVENUE', true),
('c0000000-0000-0000-0005-000000000001'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, '5000', 'Cost of Goods Sold', 'EXPENSE', true),
('c0000000-0000-0000-0005-000000000002'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, '6000', 'Salaries Expense', 'EXPENSE', true)
ON CONFLICT DO NOTHING;

-- Contacts
INSERT INTO tenant_acme.contacts (id, tenant_id, code, name, contact_type, reg_code, vat_number, email, phone, address_line1, city, postal_code, country_code, payment_terms_days) VALUES
('d0000000-0000-0000-0001-000000000001'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, 'C001', 'TechStart OÜ', 'CUSTOMER', '14567890', 'EE145678901', 'info@techstart.ee', '+372 5234 5678', 'Pärnu mnt 15', 'Tallinn', '10141', 'EE', 14),
('d0000000-0000-0000-0001-000000000002'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, 'C002', 'Nordic Solutions AS', 'CUSTOMER', '98765432', 'EE987654321', 'orders@nordic.ee', '+372 5345 6789', 'Tartu mnt 83', 'Tallinn', '10115', 'EE', 30),
('d0000000-0000-0000-0002-000000000001'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, 'S001', 'Office Supplies Ltd', 'SUPPLIER', '33445566', NULL, 'orders@officesupplies.ee', '+372 5678 9012', 'Peterburi tee 71', 'Tallinn', '11415', 'EE', 30)
ON CONFLICT DO NOTHING;

-- Invoices
INSERT INTO tenant_acme.invoices (id, tenant_id, invoice_number, invoice_type, contact_id, issue_date, due_date, subtotal, vat_amount, total, base_subtotal, base_vat_amount, base_total, amount_paid, status, created_by) VALUES
('e0000000-0000-0000-0001-000000000001'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, 'INV-2024-001', 'SALES', 'd0000000-0000-0000-0001-000000000001'::uuid, '2024-11-01', '2024-11-15', 2500.00, 550.00, 3050.00, 2500.00, 550.00, 3050.00, 3050.00, 'PAID', 'a0000000-0000-0000-0000-000000000001'::uuid),
('e0000000-0000-0000-0001-000000000002'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, 'INV-2024-002', 'SALES', 'd0000000-0000-0000-0001-000000000002'::uuid, '2024-12-01', '2024-12-15', 3200.00, 704.00, 3904.00, 3200.00, 704.00, 3904.00, 0.00, 'SENT', 'a0000000-0000-0000-0000-000000000001'::uuid)
ON CONFLICT DO NOTHING;

-- Invoice Lines
INSERT INTO tenant_acme.invoice_lines (tenant_id, invoice_id, line_number, description, quantity, unit, unit_price, vat_rate, line_subtotal, line_vat, line_total, account_id) VALUES
('b0000000-0000-0000-0000-000000000001'::uuid, 'e0000000-0000-0000-0001-000000000001'::uuid, 1, 'Software Development Services', 50, 'hours', 50.00, 22.00, 2500.00, 550.00, 3050.00, 'c0000000-0000-0000-0004-000000000002'::uuid),
('b0000000-0000-0000-0000-000000000001'::uuid, 'e0000000-0000-0000-0001-000000000002'::uuid, 1, 'Custom Integration Development', 40, 'hours', 80.00, 22.00, 3200.00, 704.00, 3904.00, 'c0000000-0000-0000-0004-000000000002'::uuid)
ON CONFLICT DO NOTHING;

-- Payments
INSERT INTO tenant_acme.payments (id, tenant_id, payment_number, payment_type, contact_id, payment_date, amount, base_amount, payment_method, reference, created_by) VALUES
('f0000000-0000-0000-0001-000000000001'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, 'PAY-2024-001', 'RECEIVED', 'd0000000-0000-0000-0001-000000000001'::uuid, '2024-11-12', 3050.00, 3050.00, 'Bank Transfer', 'INV-2024-001', 'a0000000-0000-0000-0000-000000000001'::uuid)
ON CONFLICT DO NOTHING;

-- Fiscal Years
INSERT INTO tenant_acme.fiscal_years (id, tenant_id, name, start_date, end_date, is_closed) VALUES
('90000000-0000-0000-0001-000000000001'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, 'FY 2024', '2024-01-01', '2024-12-31', false),
('90000000-0000-0000-0001-000000000002'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, 'FY 2025', '2025-01-01', '2025-12-31', false)
ON CONFLICT DO NOTHING;

-- Bank Accounts
INSERT INTO tenant_acme.bank_accounts (id, tenant_id, name, account_number, bank_name, currency, opening_balance, current_balance, is_active) VALUES
('80000000-0000-0000-0001-000000000001'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, 'Main EUR Account', 'EE123456789012345678', 'Swedbank', 'EUR', 50000.00, 53050.00, true)
ON CONFLICT DO NOTHING;
`
}
