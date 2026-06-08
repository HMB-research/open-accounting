package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"
	"github.com/shopspring/decimal"
	"github.com/wneessen/go-mail"

	"github.com/HMB-research/open-accounting/internal/accounting"
	"github.com/HMB-research/open-accounting/internal/analytics"
	"github.com/HMB-research/open-accounting/internal/apierror"
	"github.com/HMB-research/open-accounting/internal/apitoken"
	"github.com/HMB-research/open-accounting/internal/assets"
	"github.com/HMB-research/open-accounting/internal/auth"
	"github.com/HMB-research/open-accounting/internal/banking"
	"github.com/HMB-research/open-accounting/internal/contacts"
	"github.com/HMB-research/open-accounting/internal/demo"
	"github.com/HMB-research/open-accounting/internal/documents"
	"github.com/HMB-research/open-accounting/internal/email"
	"github.com/HMB-research/open-accounting/internal/expenses"
	"github.com/HMB-research/open-accounting/internal/inventory"
	"github.com/HMB-research/open-accounting/internal/invoicing"
	"github.com/HMB-research/open-accounting/internal/orders"
	"github.com/HMB-research/open-accounting/internal/payments"
	"github.com/HMB-research/open-accounting/internal/payroll"
	"github.com/HMB-research/open-accounting/internal/pdf"
	"github.com/HMB-research/open-accounting/internal/plugin"
	"github.com/HMB-research/open-accounting/internal/quotes"
	"github.com/HMB-research/open-accounting/internal/recurring"
	"github.com/HMB-research/open-accounting/internal/reports"
	"github.com/HMB-research/open-accounting/internal/tax"
	"github.com/HMB-research/open-accounting/internal/tenant"
	"github.com/HMB-research/open-accounting/internal/webhooks"
)

// Handlers contains all HTTP handlers
type Handlers struct {
	tokenService             *auth.TokenService
	refreshSessionService    refreshSessionManager
	passwordResetService     passwordResetManager
	passwordResetExposeToken bool
	passwordResetBaseURL     string
	passwordResetSMTPConfig  *email.SMTPConfig
	passwordResetMailer      email.MailSender
	securityAuditService     securityAuditManager
	apiTokenService          *apitoken.Service
	tenantService            *tenant.Service
	accountingService        *accounting.Service
	contactsService          *contacts.Service
	documentsService         *documents.Service
	invoicingService         *invoicing.Service
	paymentsService          *payments.Service
	pdfService               *pdf.Service
	analyticsService         *analytics.Service
	recurringService         *recurring.Service
	emailService             *email.Service
	bankingService           *banking.Service
	taxService               *tax.Service
	payrollService           *payroll.Service
	absenceService           *payroll.AbsenceService
	pluginService            *plugin.Service
	quotesService            *quotes.Service
	ordersService            *orders.Service
	assetsService            *assets.Service
	inventoryService         *inventory.Service
	reportsService           *reports.Service
	reminderService          *invoicing.ReminderService
	automatedReminderService *invoicing.AutomatedReminderService
	costCenterService        *accounting.CostCenterService
	interestService          *invoicing.InterestService
	webhookService           *webhooks.Service
	expensesService          *expenses.Service
	demoResetService         demoResetter
	demoStatusReader         demo.StatusReader
}

type refreshSessionManager interface {
	CreateRefreshSession(ctx context.Context, userID, tokenID, tokenHash string, expiresAt time.Time) error
	RotateRefreshSession(ctx context.Context, userID, oldTokenID, oldTokenHash, newTokenID, newTokenHash string, newExpiresAt time.Time) error
	RevokeRefreshSession(ctx context.Context, userID, tokenID, tokenHash string) error
	ListRefreshSessions(ctx context.Context, userID string, includeInactive bool) ([]auth.RefreshSession, error)
	RevokeRefreshSessionByID(ctx context.Context, userID, tokenID string) error
	RevokeAllRefreshSessions(ctx context.Context, userID string) error
}

type passwordResetManager interface {
	RequestPasswordReset(ctx context.Context, email, requestIP, userAgent string) (*auth.PasswordResetRequestResult, error)
	ResetPassword(ctx context.Context, resetToken, newPassword string) (string, error)
}

type securityAuditManager interface {
	RecordEvent(ctx context.Context, event *auth.SecurityAuditEvent) error
	ListUserEvents(ctx context.Context, userID string, limit int) ([]auth.SecurityAuditEvent, error)
}

type demoResetter interface {
	Reset(ctx context.Context, users []demo.ResetUser, userNums []int) error
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

		if claims.TokenKind == auth.TokenKindAPIToken && claims.TenantID != "" && claims.TenantID != tenantID {
			respondError(w, http.StatusForbidden, "API token is scoped to a different tenant")
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
		membership, err := h.tenantService.GetTenantUser(r.Context(), req.TenantID, user.ID)
		if err != nil {
			respondError(w, http.StatusForbidden, "Access denied to tenant")
			return
		}
		if !membership.IsActive {
			respondError(w, http.StatusForbidden, "Tenant access is suspended")
			return
		}
		tenantID = req.TenantID
		role = membership.Role
	}

	accessToken, err := h.tokenService.GenerateAccessToken(user.ID, user.Email, tenantID, role)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to generate token")
		return
	}

	refreshToken, refreshClaims, err := h.generateRefreshTokenWithClaims(user.ID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to generate refresh token")
		return
	}
	if h.refreshSessionService == nil {
		respondError(w, http.StatusInternalServerError, "Refresh session service unavailable")
		return
	}
	if err := h.refreshSessionService.CreateRefreshSession(r.Context(), user.ID, refreshClaims.ID, auth.HashRefreshToken(refreshToken), refreshClaims.ExpiresAt.Time); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to create refresh session")
		return
	}
	h.recordSecurityAuditEvent(r, &auth.SecurityAuditEvent{
		ActorUserID:  user.ID,
		ActorEmail:   user.Email,
		Action:       auth.SecurityAuditActionLogin,
		TargetUserID: user.ID,
		TargetEmail:  user.Email,
		Metadata: map[string]string{
			"tenant_id": tenantID,
		},
	})

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

	refreshClaims, err := h.tokenService.ValidateRefreshTokenClaims(req.RefreshToken)
	if err != nil {
		respondError(w, http.StatusUnauthorized, "Invalid refresh token")
		return
	}
	if h.refreshSessionService == nil {
		respondError(w, http.StatusInternalServerError, "Refresh session service unavailable")
		return
	}

	user, err := h.tenantService.GetUserByID(r.Context(), refreshClaims.Subject)
	if err != nil {
		respondError(w, http.StatusUnauthorized, "User not found")
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
		membership, err := h.tenantService.GetTenantUser(r.Context(), req.TenantID, user.ID)
		if err != nil {
			respondError(w, http.StatusForbidden, "Access denied to tenant")
			return
		}
		if !membership.IsActive {
			respondError(w, http.StatusForbidden, "Tenant access is suspended")
			return
		}
		tenantID = req.TenantID
		role = membership.Role
	}

	accessToken, err := h.tokenService.GenerateAccessToken(user.ID, user.Email, tenantID, role)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to generate token")
		return
	}

	newRefreshToken, newRefreshClaims, err := h.generateRefreshTokenWithClaims(user.ID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to generate refresh token")
		return
	}
	if err := h.refreshSessionService.RotateRefreshSession(
		r.Context(),
		user.ID,
		refreshClaims.ID,
		auth.HashRefreshToken(req.RefreshToken),
		newRefreshClaims.ID,
		auth.HashRefreshToken(newRefreshToken),
		newRefreshClaims.ExpiresAt.Time,
	); err != nil {
		if errors.Is(err, auth.ErrRefreshSessionInvalid) {
			respondError(w, http.StatusUnauthorized, "Invalid refresh token")
			return
		}
		respondError(w, http.StatusInternalServerError, "Failed to rotate refresh session")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"access_token":  accessToken,
		"refresh_token": newRefreshToken,
		"token_type":    "Bearer",
		"expires_in":    900,
	})
}

// Logout revokes a refresh token session
// @Summary Revoke refresh token
// @Description Revoke a refresh token session so it can no longer be used
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body object{refresh_token=string} true "Refresh token"
// @Success 200 {object} object{status=string}
// @Failure 401 {object} object{error=string}
// @Router /auth/logout [post]
func (h *Handlers) Logout(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	refreshClaims, err := h.tokenService.ValidateRefreshTokenClaims(req.RefreshToken)
	if err != nil {
		respondError(w, http.StatusUnauthorized, "Invalid refresh token")
		return
	}
	if h.refreshSessionService == nil {
		respondError(w, http.StatusInternalServerError, "Refresh session service unavailable")
		return
	}

	err = h.refreshSessionService.RevokeRefreshSession(r.Context(), refreshClaims.Subject, refreshClaims.ID, auth.HashRefreshToken(req.RefreshToken))
	if err != nil {
		if errors.Is(err, auth.ErrRefreshSessionInvalid) {
			respondError(w, http.StatusUnauthorized, "Invalid refresh token")
			return
		}
		respondError(w, http.StatusInternalServerError, "Failed to revoke refresh session")
		return
	}
	targetEmail := h.userEmailForAudit(r.Context(), refreshClaims.Subject)
	h.recordSecurityAuditEvent(r, &auth.SecurityAuditEvent{
		ActorUserID:  refreshClaims.Subject,
		ActorEmail:   targetEmail,
		Action:       auth.SecurityAuditActionLogout,
		TargetUserID: refreshClaims.Subject,
		TargetEmail:  targetEmail,
		Metadata: map[string]string{
			"session_id": refreshClaims.ID,
		},
	})

	respondJSON(w, http.StatusOK, map[string]string{"status": "revoked"})
}

// RequestPasswordReset prepares a one-time password reset token.
// @Summary Request password reset
// @Description Request a one-time password reset token for an account. The response is intentionally generic to avoid account enumeration.
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body object{email=string} true "Password reset request"
// @Success 202 {object} object{status=string,message=string,reset_token=string,expires_at=string}
// @Failure 400 {object} object{error=string}
// @Router /auth/password-reset/request [post]
func (h *Handlers) RequestPasswordReset(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email string `json:"email"`
	}
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if strings.TrimSpace(req.Email) == "" {
		respondError(w, http.StatusBadRequest, "Email is required")
		return
	}
	if h.passwordResetService == nil {
		respondError(w, http.StatusInternalServerError, "Password reset service unavailable")
		return
	}

	result, err := h.passwordResetService.RequestPasswordReset(r.Context(), req.Email, r.RemoteAddr, r.UserAgent())
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to request password reset")
		return
	}
	if result != nil && result.Issued {
		if err := h.deliverPasswordReset(r.Context(), result); err != nil {
			log.Warn().Err(err).Str("email", result.Email).Msg("Failed to deliver password reset email")
		}
		h.recordSecurityAuditEvent(r, &auth.SecurityAuditEvent{
			Action:       auth.SecurityAuditActionPasswordResetRequested,
			TargetUserID: result.UserID,
			TargetEmail:  result.Email,
		})
	}

	response := map[string]interface{}{
		"status":  "accepted",
		"message": "If the email belongs to an active account, reset instructions have been prepared.",
	}
	if h.passwordResetExposeToken && result != nil && result.Issued && result.Token != "" {
		response["reset_token"] = result.Token
		response["expires_at"] = result.ExpiresAt
	}
	respondJSON(w, http.StatusAccepted, response)
}

// ResetPassword resets an account password with a one-time token.
// @Summary Reset password
// @Description Reset an account password using a one-time password reset token. Active refresh-token sessions are revoked after a successful reset.
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body object{token=string,new_password=string} true "Password reset confirmation"
// @Success 200 {object} object{status=string}
// @Failure 400 {object} object{error=string}
// @Failure 401 {object} object{error=string}
// @Router /auth/password-reset/confirm [post]
func (h *Handlers) ResetPassword(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Token       string `json:"token"`
		NewPassword string `json:"new_password"`
	}
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if strings.TrimSpace(req.Token) == "" || req.NewPassword == "" {
		respondError(w, http.StatusBadRequest, "Token and new password are required")
		return
	}
	if h.passwordResetService == nil {
		respondError(w, http.StatusInternalServerError, "Password reset service unavailable")
		return
	}
	if h.refreshSessionService == nil {
		respondError(w, http.StatusInternalServerError, "Refresh session service unavailable")
		return
	}

	userID, err := h.passwordResetService.ResetPassword(r.Context(), req.Token, req.NewPassword)
	if err != nil {
		status := http.StatusBadRequest
		switch {
		case errors.Is(err, auth.ErrPasswordResetTokenInvalid),
			strings.Contains(err.Error(), "account is disabled"):
			status = http.StatusUnauthorized
		}
		respondError(w, status, err.Error())
		return
	}

	if err := h.refreshSessionService.RevokeAllRefreshSessions(r.Context(), userID); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to revoke refresh sessions")
		return
	}
	targetEmail := h.userEmailForAudit(r.Context(), userID)
	h.recordSecurityAuditEvent(r, &auth.SecurityAuditEvent{
		Action:       auth.SecurityAuditActionPasswordResetCompleted,
		TargetUserID: userID,
		TargetEmail:  targetEmail,
	})

	respondJSON(w, http.StatusOK, map[string]string{"status": "password_reset"})
}

func (h *Handlers) deliverPasswordReset(ctx context.Context, result *auth.PasswordResetRequestResult) error {
	if h.passwordResetMailer == nil || h.passwordResetSMTPConfig == nil || !h.passwordResetSMTPConfig.IsConfigured() || strings.TrimSpace(h.passwordResetBaseURL) == "" {
		return nil
	}
	if strings.TrimSpace(result.Token) == "" || strings.TrimSpace(result.Email) == "" {
		return nil
	}

	resetURL, err := buildPasswordResetURL(h.passwordResetBaseURL, result.Token)
	if err != nil {
		return err
	}

	expiresAt := "one hour"
	if result.ExpiresAt != nil {
		expiresAt = result.ExpiresAt.Format(time.RFC3339)
	}

	msg := mail.NewMsg()
	if strings.TrimSpace(h.passwordResetSMTPConfig.FromName) != "" {
		if err := msg.FromFormat(h.passwordResetSMTPConfig.FromName, h.passwordResetSMTPConfig.FromEmail); err != nil {
			return err
		}
	} else if err := msg.From(h.passwordResetSMTPConfig.FromEmail); err != nil {
		return err
	}
	if err := msg.To(result.Email); err != nil {
		return err
	}
	msg.Subject("Open Accounting password reset")
	msg.SetBodyString(mail.TypeTextPlain, fmt.Sprintf(
		"Use this link to reset your Open Accounting password:\n\n%s\n\nThis link expires at %s.\n\nIf you did not request a password reset, you can ignore this email.",
		resetURL,
		expiresAt,
	))

	done := make(chan error, 1)
	go func() {
		done <- h.passwordResetMailer.SendMail(h.passwordResetSMTPConfig, msg)
	}()

	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func buildPasswordResetURL(baseURL, resetToken string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil {
		return "", fmt.Errorf("parse password reset base URL: %w", err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("password reset base URL must include scheme and host")
	}
	if parsed.Path == "" || parsed.Path == "/" {
		parsed.Path = "/reset-password"
	}
	values := parsed.Query()
	values.Set("token", resetToken)
	parsed.RawQuery = values.Encode()
	return parsed.String(), nil
}

func (h *Handlers) recordSecurityAuditEvent(r *http.Request, event *auth.SecurityAuditEvent) {
	if h.securityAuditService == nil || event == nil {
		return
	}
	if event.RequestIP == "" {
		event.RequestIP = r.RemoteAddr
	}
	if event.UserAgent == "" {
		event.UserAgent = r.UserAgent()
	}
	if err := h.securityAuditService.RecordEvent(r.Context(), event); err != nil {
		log.Warn().Err(err).Str("action", event.Action).Msg("Failed to record security audit event")
	}
}

func (h *Handlers) userEmailForAudit(ctx context.Context, userID string) string {
	if h.tenantService == nil || strings.TrimSpace(userID) == "" {
		return ""
	}
	user, err := h.tenantService.GetUserByID(ctx, userID)
	if err != nil {
		return ""
	}
	return user.Email
}

// ChangePassword changes the authenticated user's password.
// @Summary Change password
// @Description Change the authenticated user's password after verifying the current password. Active refresh-token sessions are revoked after a successful change.
// @Tags Auth
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body object{current_password=string,new_password=string} true "Password change details"
// @Success 200 {object} object{status=string}
// @Failure 400 {object} object{error=string}
// @Failure 401 {object} object{error=string}
// @Router /auth/password [put]
func (h *Handlers) ChangePassword(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.GetClaims(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	var req struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if h.refreshSessionService == nil {
		respondError(w, http.StatusInternalServerError, "Refresh session service unavailable")
		return
	}

	if err := h.tenantService.ChangeUserPassword(r.Context(), claims.UserID, req.CurrentPassword, req.NewPassword); err != nil {
		status := http.StatusBadRequest
		switch {
		case strings.Contains(err.Error(), "current password is incorrect"),
			strings.Contains(err.Error(), "account is disabled"),
			strings.Contains(err.Error(), "user not found"):
			status = http.StatusUnauthorized
		}
		respondError(w, status, err.Error())
		return
	}

	if err := h.refreshSessionService.RevokeAllRefreshSessions(r.Context(), claims.UserID); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to revoke refresh sessions")
		return
	}
	h.recordSecurityAuditEvent(r, &auth.SecurityAuditEvent{
		ActorUserID:  claims.UserID,
		ActorEmail:   claims.Email,
		Action:       auth.SecurityAuditActionPasswordChanged,
		TargetUserID: claims.UserID,
		TargetEmail:  claims.Email,
	})

	respondJSON(w, http.StatusOK, map[string]string{"status": "password_changed"})
}

// ListAuthSessions lists refresh sessions for the authenticated user
// @Summary List auth sessions
// @Description List active refresh-token sessions for the authenticated user
// @Tags Auth
// @Produce json
// @Security BearerAuth
// @Param include_inactive query bool false "Include revoked and expired sessions"
// @Success 200 {array} auth.RefreshSession
// @Failure 401 {object} object{error=string}
// @Router /auth/sessions [get]
func (h *Handlers) ListAuthSessions(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.GetClaims(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Authentication required")
		return
	}
	if h.refreshSessionService == nil {
		respondError(w, http.StatusInternalServerError, "Refresh session service unavailable")
		return
	}

	includeInactive := strings.EqualFold(r.URL.Query().Get("include_inactive"), "true")
	sessions, err := h.refreshSessionService.ListRefreshSessions(r.Context(), claims.UserID, includeInactive)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to list refresh sessions")
		return
	}
	respondJSON(w, http.StatusOK, sessions)
}

// RevokeAuthSession revokes one refresh session for the authenticated user
// @Summary Revoke auth session
// @Description Revoke one refresh-token session for the authenticated user
// @Tags Auth
// @Produce json
// @Security BearerAuth
// @Param sessionID path string true "Refresh session id"
// @Success 200 {object} object{status=string}
// @Failure 401 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Router /auth/sessions/{sessionID} [delete]
func (h *Handlers) RevokeAuthSession(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.GetClaims(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Authentication required")
		return
	}
	if h.refreshSessionService == nil {
		respondError(w, http.StatusInternalServerError, "Refresh session service unavailable")
		return
	}

	sessionID := strings.TrimSpace(chi.URLParam(r, "sessionID"))
	if sessionID == "" {
		respondError(w, http.StatusBadRequest, "Session id is required")
		return
	}
	if err := h.refreshSessionService.RevokeRefreshSessionByID(r.Context(), claims.UserID, sessionID); err != nil {
		if errors.Is(err, auth.ErrRefreshSessionInvalid) {
			respondError(w, http.StatusNotFound, "Refresh session not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "Failed to revoke refresh session")
		return
	}
	h.recordSecurityAuditEvent(r, &auth.SecurityAuditEvent{
		ActorUserID:  claims.UserID,
		ActorEmail:   claims.Email,
		Action:       auth.SecurityAuditActionSessionRevoked,
		TargetUserID: claims.UserID,
		TargetEmail:  claims.Email,
		Metadata: map[string]string{
			"session_id": sessionID,
		},
	})
	respondJSON(w, http.StatusOK, map[string]string{"status": "revoked"})
}

// RevokeAllAuthSessions revokes all refresh sessions for the authenticated user.
// @Summary Revoke all auth sessions
// @Description Revoke all active refresh-token sessions for the authenticated user
// @Tags Auth
// @Produce json
// @Security BearerAuth
// @Success 200 {object} object{status=string}
// @Failure 401 {object} object{error=string}
// @Router /auth/sessions [delete]
func (h *Handlers) RevokeAllAuthSessions(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.GetClaims(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Authentication required")
		return
	}
	if h.refreshSessionService == nil {
		respondError(w, http.StatusInternalServerError, "Refresh session service unavailable")
		return
	}

	if err := h.refreshSessionService.RevokeAllRefreshSessions(r.Context(), claims.UserID); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to revoke refresh sessions")
		return
	}
	h.recordSecurityAuditEvent(r, &auth.SecurityAuditEvent{
		ActorUserID:  claims.UserID,
		ActorEmail:   claims.Email,
		Action:       auth.SecurityAuditActionAllSessionsRevoked,
		TargetUserID: claims.UserID,
		TargetEmail:  claims.Email,
	})
	respondJSON(w, http.StatusOK, map[string]string{"status": "revoked"})
}

// ListSecurityAuditEvents lists security audit events for the authenticated user.
// @Summary List security audit events
// @Description List recent auth security events where the authenticated user is actor or target
// @Tags Auth
// @Produce json
// @Security BearerAuth
// @Param limit query int false "Maximum number of events to return"
// @Success 200 {array} auth.SecurityAuditEvent
// @Failure 401 {object} object{error=string}
// @Router /auth/security-events [get]
func (h *Handlers) ListSecurityAuditEvents(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.GetClaims(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Authentication required")
		return
	}
	if h.securityAuditService == nil {
		respondError(w, http.StatusInternalServerError, "Security audit service unavailable")
		return
	}

	limit := 50
	if rawLimit := strings.TrimSpace(r.URL.Query().Get("limit")); rawLimit != "" {
		parsedLimit, err := strconv.Atoi(rawLimit)
		if err != nil || parsedLimit <= 0 {
			respondError(w, http.StatusBadRequest, "Invalid limit")
			return
		}
		limit = parsedLimit
	}

	events, err := h.securityAuditService.ListUserEvents(r.Context(), claims.UserID, limit)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to list security audit events")
		return
	}
	respondJSON(w, http.StatusOK, events)
}

func (h *Handlers) generateRefreshTokenWithClaims(userID string) (string, *auth.RefreshClaims, error) {
	refreshToken, err := h.tokenService.GenerateRefreshToken(userID)
	if err != nil {
		return "", nil, err
	}
	refreshClaims, err := h.tokenService.ValidateRefreshTokenClaims(refreshToken)
	if err != nil {
		return "", nil, err
	}
	return refreshToken, refreshClaims, nil
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

// UpdateTenant updates a tenant's name and/or settings.
// @Summary Update tenant
// @Description Update tenant name or settings. Requires permission to manage tenant settings.
// @Tags Tenants
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param request body tenant.UpdateTenantRequest true "Tenant update"
// @Success 200 {object} tenant.Tenant
// @Failure 400 {object} object{error=string}
// @Failure 401 {object} object{error=string}
// @Failure 403 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID} [put]
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
		switch {
		case strings.Contains(err.Error(), "tenant not found"):
			respondError(w, http.StatusNotFound, "Tenant not found")
		case strings.Contains(err.Error(), "period lock must be managed through close or reopen actions"):
			respondError(w, http.StatusBadRequest, err.Error())
		default:
			respondError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	if !h.recordTenantAuditEvent(w, r, &tenant.TenantAuditEvent{
		TenantID:    tenantID,
		ActorUserID: claims.UserID,
		Action:      tenant.AuditActionTenantUpdated,
		TargetType:  tenant.AuditTargetTenant,
		TargetID:    tenantID,
		Metadata: map[string]string{
			"role": role,
		},
	}) {
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

// GetAccountHierarchy returns a flattened parent-child chart of accounts.
// @Summary Get account hierarchy
// @Description Get chart of accounts rows ordered by parent-child account grouping
// @Tags Accounts
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param active_only query bool false "Filter for active accounts only"
// @Success 200 {array} accounting.AccountHierarchyRow
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/accounts/hierarchy [get]
func (h *Handlers) GetAccountHierarchy(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	activeOnly := r.URL.Query().Get("active_only") == "true"
	rows, err := h.accountingService.GetAccountHierarchy(r.Context(), schemaName, tenantID, activeOnly)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get account hierarchy")
		return
	}

	respondJSON(w, http.StatusOK, rows)
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

// ImportAccounts imports accounts from CSV data.
// @Summary Import accounts
// @Description Import chart of accounts rows from CSV data and skip duplicate or invalid rows
// @Tags Accounts
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param request body accounting.ImportAccountsRequest true "CSV import payload"
// @Success 200 {object} accounting.ImportAccountsResult
// @Failure 400 {object} object{error=string}
// @Router /tenants/{tenantID}/accounts/import [post]
func (h *Handlers) ImportAccounts(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	var req accounting.ImportAccountsRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if strings.TrimSpace(req.CSVContent) == "" {
		respondError(w, http.StatusBadRequest, "csv_content is required")
		return
	}

	if req.FileName == "" {
		req.FileName = "accounts_import.csv"
	}

	result, err := h.accountingService.ImportAccountsCSV(r.Context(), schemaName, tenantID, &req)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, result)
}

// ImportOpeningBalances imports opening balances from CSV and posts them as a journal entry.
// @Summary Import opening balances
// @Description Import opening balances from CSV data, create a journal entry, and post it immediately
// @Tags Journal Entries
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param request body accounting.ImportOpeningBalancesRequest true "Opening-balance CSV payload"
// @Success 200 {object} accounting.ImportOpeningBalancesResult
// @Failure 400 {object} object{error=string}
// @Failure 401 {object} object{error=string}
// @Router /tenants/{tenantID}/journal-entries/import-opening-balances [post]
func (h *Handlers) ImportOpeningBalances(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.GetClaims(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	tenantID := chi.URLParam(r, "tenantID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	var req accounting.ImportOpeningBalancesRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if strings.TrimSpace(req.CSVContent) == "" {
		respondError(w, http.StatusBadRequest, "csv_content is required")
		return
	}
	if strings.TrimSpace(req.EntryDate) == "" {
		respondError(w, http.StatusBadRequest, "entry_date is required")
		return
	}

	entryDate, err := time.Parse("2006-01-02", req.EntryDate)
	if err != nil {
		respondError(w, http.StatusBadRequest, "entry_date must be in YYYY-MM-DD format")
		return
	}
	if h.rejectLockedPeriod(w, r.Context(), tenantID, entryDate) {
		return
	}

	req.UserID = claims.UserID
	if req.FileName == "" {
		req.FileName = "opening_balances.csv"
	}

	result, err := h.accountingService.ImportOpeningBalancesCSV(r.Context(), schemaName, tenantID, &req)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, result)
}

// ImportJournalEntries imports historical journal entries from grouped CSV rows.
// @Summary Import journal entries
// @Description Import historical double-entry journal entries from grouped CSV rows
// @Tags Journal Entries
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param request body accounting.ImportJournalEntriesRequest true "Historical journal CSV payload"
// @Success 200 {object} accounting.ImportJournalEntriesResult
// @Failure 400 {object} object{error=string}
// @Failure 401 {object} object{error=string}
// @Router /tenants/{tenantID}/journal-entries/import [post]
func (h *Handlers) ImportJournalEntries(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.GetClaims(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	tenantID := chi.URLParam(r, "tenantID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	var req accounting.ImportJournalEntriesRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if strings.TrimSpace(req.CSVContent) == "" {
		respondError(w, http.StatusBadRequest, "csv_content is required")
		return
	}

	if tenantRecord, err := h.tenantService.GetTenant(r.Context(), tenantID); err == nil && tenantRecord.Settings.PeriodLockDate != nil {
		lockDate, err := time.Parse("2006-01-02", strings.TrimSpace(*tenantRecord.Settings.PeriodLockDate))
		if err != nil {
			respondError(w, http.StatusBadRequest, "tenant period_lock_date must be in YYYY-MM-DD format")
			return
		}
		req.PeriodLockDate = &lockDate
	}

	req.UserID = claims.UserID
	if req.FileName == "" {
		req.FileName = "journal_entries.csv"
	}

	result, err := h.accountingService.ImportJournalEntriesCSV(r.Context(), schemaName, tenantID, &req)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, result)
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

// ListJournalEntries returns recent journal entries.
// @Summary List journal entries
// @Description List recent journal entries for a tenant
// @Tags Journal Entries
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param limit query int false "Max entries to return" default(50)
// @Success 200 {array} accounting.JournalEntry
// @Failure 400 {object} object{error=string}
// @Router /tenants/{tenantID}/journal-entries [get]
func (h *Handlers) ListJournalEntries(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	limit := 50
	if rawLimit := strings.TrimSpace(r.URL.Query().Get("limit")); rawLimit != "" {
		parsed, err := strconv.Atoi(rawLimit)
		if err != nil || parsed <= 0 || parsed > 200 {
			respondError(w, http.StatusBadRequest, "Limit must be between 1 and 200")
			return
		}
		limit = parsed
	}

	entries, err := h.accountingService.ListJournalEntries(r.Context(), schemaName, tenantID, limit)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, entries)
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

	if h.rejectLockedPeriod(w, r.Context(), tenantID, req.EntryDate) {
		return
	}

	entry, err := h.accountingService.CreateJournalEntry(r.Context(), schemaName, tenantID, &req)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	h.emitWebhookEvent(plugin.EventJournalEntryCreated, tenantID, entry)
	respondJSON(w, http.StatusCreated, entry)
}

// PostJournalEntry posts a draft journal entry
// @Summary Post journal entry
// @Description Post a draft journal entry to finalize it. Entries marked requires_evidence need approved supporting, receipt, or tax evidence first.
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

	entry, err := h.accountingService.GetJournalEntry(r.Context(), schemaName, tenantID, entryID)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	if h.rejectLockedPeriod(w, r.Context(), tenantID, entry.EntryDate) {
		return
	}

	if err := h.requireApprovedJournalEntryEvidence(r.Context(), schemaName, tenantID, entry); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, errApprovedJournalEntryEvidenceRequired) {
			status = http.StatusConflict
		}
		respondError(w, status, err.Error())
		return
	}

	err = h.accountingService.PostJournalEntry(r.Context(), schemaName, tenantID, entryID, claims.UserID)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	h.emitWebhookEvent(plugin.EventJournalEntryPosted, tenantID, map[string]string{"journal_entry_id": entryID})
	respondJSON(w, http.StatusOK, map[string]string{"status": "posted"})
}

func (h *Handlers) requireApprovedJournalEntryEvidence(ctx context.Context, schemaName, tenantID string, entry *accounting.JournalEntry) error {
	if h.documentsService == nil || entry == nil || !entry.RequiresEvidence {
		return nil
	}

	results, err := h.documentsService.EvaluateEvidencePolicy(ctx, schemaName, tenantID, &documents.EvidencePolicyRequest{
		EntityType: documents.EntityTypeJournalEntry,
		EntityIDs:  []string{entry.ID},
		Rules: []documents.EvidencePolicyRule{{
			DocumentTypes: []string{
				documents.DocumentTypeSupportingDocument,
				documents.DocumentTypeReceipt,
				documents.DocumentTypeTaxSupport,
			},
			MinCount:        1,
			RequireApproved: true,
		}},
	})
	if err != nil {
		return fmt.Errorf("evaluate journal-entry evidence: %w", err)
	}
	if len(results) == 0 || !results[0].Compliant {
		return fmt.Errorf("%w before posting journal entry %s", errApprovedJournalEntryEvidenceRequired, entry.ID)
	}

	return nil
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

	entry, err := h.accountingService.GetJournalEntry(r.Context(), schemaName, tenantID, entryID)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	if h.rejectLockedPeriod(w, r.Context(), tenantID, entry.EntryDate) {
		return
	}

	reversal, err := h.accountingService.VoidJournalEntry(r.Context(), schemaName, tenantID, entryID, claims.UserID, req.Reason)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	h.emitWebhookEvent(plugin.EventJournalEntryVoided, tenantID, reversal)
	respondJSON(w, http.StatusOK, reversal)
}

// ListJournalEntryTemplates returns reusable journal entry templates.
// @Summary List journal entry templates
// @Description List reusable balanced journal entry templates for a tenant
// @Tags Journal Entries
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param active_only query bool false "Filter for active templates only"
// @Success 200 {array} accounting.JournalEntryTemplate
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/journal-entry-templates [get]
func (h *Handlers) ListJournalEntryTemplates(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	schemaName := h.getSchemaName(r.Context(), tenantID)
	activeOnly := r.URL.Query().Get("active_only") == "true"

	templates, err := h.accountingService.ListJournalEntryTemplates(r.Context(), schemaName, tenantID, activeOnly)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, templates)
}

// CreateJournalEntryTemplate creates a reusable journal entry template.
// @Summary Create journal entry template
// @Description Create a reusable balanced journal entry template
// @Tags Journal Entries
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param request body accounting.CreateJournalEntryTemplateRequest true "Journal entry template details"
// @Success 201 {object} accounting.JournalEntryTemplate
// @Failure 400 {object} object{error=string}
// @Router /tenants/{tenantID}/journal-entry-templates [post]
func (h *Handlers) CreateJournalEntryTemplate(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.GetClaims(r.Context())
	tenantID := chi.URLParam(r, "tenantID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	var req accounting.CreateJournalEntryTemplateRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	req.UserID = claims.UserID

	template, err := h.accountingService.CreateJournalEntryTemplate(r.Context(), schemaName, tenantID, &req)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	respondJSON(w, http.StatusCreated, template)
}

// GenerateDueJournalEntryTemplates generates all recurring journal entry templates due by a date.
// @Summary Generate due recurring journal entry templates
// @Description Generate entries for active recurring templates due by as_of_date
// @Tags Journal Entries
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param request body accounting.GenerateDueJournalEntryTemplatesRequest true "Due generation details"
// @Success 200 {array} accounting.JournalEntryTemplateGenerationResult
// @Failure 400 {object} object{error=string}
// @Router /tenants/{tenantID}/journal-entry-templates/generate-due [post]
func (h *Handlers) GenerateDueJournalEntryTemplates(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.GetClaims(r.Context())
	tenantID := chi.URLParam(r, "tenantID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	var req accounting.GenerateDueJournalEntryTemplatesRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	req.UserID = claims.UserID
	lockDate, err := h.getTenantPeriodLockDate(r.Context(), tenantID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to validate period lock")
		return
	}
	req.PeriodLockDate = lockDate

	results, err := h.accountingService.GenerateDueJournalEntryTemplates(r.Context(), schemaName, tenantID, &req)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, results)
}

// GetJournalEntryTemplate returns one reusable journal entry template.
// @Summary Get journal entry template
// @Description Get one reusable journal entry template with lines
// @Tags Journal Entries
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param templateID path string true "Template ID"
// @Success 200 {object} accounting.JournalEntryTemplate
// @Failure 404 {object} object{error=string}
// @Router /tenants/{tenantID}/journal-entry-templates/{templateID} [get]
func (h *Handlers) GetJournalEntryTemplate(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	templateID := chi.URLParam(r, "templateID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	template, err := h.accountingService.GetJournalEntryTemplate(r.Context(), schemaName, tenantID, templateID)
	if err != nil {
		respondError(w, http.StatusNotFound, "Journal entry template not found")
		return
	}
	respondJSON(w, http.StatusOK, template)
}

// GenerateJournalEntryTemplate generates and advances one recurring journal entry template.
// @Summary Generate recurring journal entry template
// @Description Generate an entry from one recurring template and advance its next generation date
// @Tags Journal Entries
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param templateID path string true "Template ID"
// @Param request body accounting.GenerateJournalEntryTemplateRequest true "Recurring generation details"
// @Success 201 {object} accounting.JournalEntryTemplateGenerationResult
// @Failure 400 {object} object{error=string}
// @Failure 409 {object} object{error=string}
// @Router /tenants/{tenantID}/journal-entry-templates/{templateID}/generate [post]
func (h *Handlers) GenerateJournalEntryTemplate(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.GetClaims(r.Context())
	tenantID := chi.URLParam(r, "tenantID")
	templateID := chi.URLParam(r, "templateID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	var req accounting.GenerateJournalEntryTemplateRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	req.UserID = claims.UserID
	template, err := h.accountingService.GetJournalEntryTemplate(r.Context(), schemaName, tenantID, templateID)
	if err != nil {
		respondError(w, http.StatusNotFound, "Journal entry template not found")
		return
	}
	entryDate := time.Now()
	if req.EntryDate != nil {
		entryDate = *req.EntryDate
	} else if template.NextGenerationDate != nil {
		entryDate = *template.NextGenerationDate
	}
	if h.rejectLockedPeriod(w, r.Context(), tenantID, entryDate) {
		return
	}
	lockDate, err := h.getTenantPeriodLockDate(r.Context(), tenantID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to validate period lock")
		return
	}
	req.PeriodLockDate = lockDate

	result, err := h.accountingService.GenerateJournalEntryTemplate(r.Context(), schemaName, tenantID, templateID, &req)
	if err != nil {
		if errors.Is(err, accounting.ErrTemplateEvidenceAutoPost) || strings.Contains(err.Error(), "period locked") {
			respondError(w, http.StatusConflict, err.Error())
			return
		}
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	respondJSON(w, http.StatusCreated, result)
}

// ApplyJournalEntryTemplate creates a journal entry from a reusable template.
// @Summary Apply journal entry template
// @Description Create a draft or posted journal entry from a reusable template
// @Tags Journal Entries
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param templateID path string true "Template ID"
// @Param request body accounting.ApplyJournalEntryTemplateRequest true "Template application details"
// @Success 201 {object} accounting.JournalEntry
// @Failure 400 {object} object{error=string}
// @Failure 409 {object} object{error=string}
// @Router /tenants/{tenantID}/journal-entry-templates/{templateID}/apply [post]
func (h *Handlers) ApplyJournalEntryTemplate(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.GetClaims(r.Context())
	tenantID := chi.URLParam(r, "tenantID")
	templateID := chi.URLParam(r, "templateID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	var req accounting.ApplyJournalEntryTemplateRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	req.UserID = claims.UserID
	entryDate := req.EntryDate
	if entryDate.IsZero() {
		entryDate = time.Now()
	}
	if h.rejectLockedPeriod(w, r.Context(), tenantID, entryDate) {
		return
	}

	entry, err := h.accountingService.ApplyJournalEntryTemplate(r.Context(), schemaName, tenantID, templateID, &req)
	if err != nil {
		if errors.Is(err, accounting.ErrTemplateEvidenceAutoPost) {
			respondError(w, http.StatusConflict, err.Error())
			return
		}
		status := http.StatusBadRequest
		if strings.Contains(err.Error(), "not found") {
			status = http.StatusNotFound
		}
		respondError(w, status, err.Error())
		return
	}
	respondJSON(w, http.StatusCreated, entry)
}

// GetTrialBalance returns the trial balance for a tenant
// @Summary Get trial balance
// @Description Get trial balance report as of a specific date
// @Tags Reports
// @Produce json,text/csv,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet,application/pdf
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param as_of_date query string false "As of date (YYYY-MM-DD)"
// @Param format query string false "Response format: json, csv, xlsx, or pdf"
// @Success 200 {object} accounting.TrialBalance
// @Failure 400 {object} object{error=string}
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/reports/trial-balance [get]
func (h *Handlers) GetTrialBalance(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	format, err := reportResponseFormat(r)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

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

	if format == "csv" {
		content, err := trialBalanceCSV(tb)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "Failed to export trial balance CSV")
			return
		}
		respondReportCSV(w, fmt.Sprintf("trial-balance-%s.csv", asOfDate.Format("2006-01-02")), content)
		return
	}
	if format == "xlsx" {
		content, err := trialBalanceXLSX(tb)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "Failed to export trial balance XLSX")
			return
		}
		respondReportXLSX(w, fmt.Sprintf("trial-balance-%s.xlsx", asOfDate.Format("2006-01-02")), content)
		return
	}
	if format == "pdf" {
		content, err := trialBalancePDF(tb, asOfDate)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "Failed to export trial balance PDF")
			return
		}
		respondReportPDF(w, fmt.Sprintf("trial-balance-%s.pdf", asOfDate.Format("2006-01-02")), content)
		return
	}

	respondJSON(w, http.StatusOK, tb)
}

// GetAccountBalance returns the balance of a specific account
// @Summary Get account balance
// @Description Get the balance of a specific account as of a date
// @Tags Reports
// @Produce json,text/csv,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet,application/pdf
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param accountID path string true "Account ID"
// @Param as_of_date query string false "As of date (YYYY-MM-DD)"
// @Param format query string false "Response format: json, csv, xlsx, or pdf"
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

	format, err := reportResponseFormat(r)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	balance, err := h.accountingService.GetAccountBalance(r.Context(), schemaName, tenantID, accountID, asOfDate)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get account balance")
		return
	}
	asOfDateText := asOfDate.Format("2006-01-02")
	balanceText := balance.String()

	if format == "csv" {
		content, err := accountBalanceCSV(accountID, asOfDateText, balanceText)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "Failed to export account balance CSV")
			return
		}
		respondReportCSV(w, fmt.Sprintf("account-balance-%s-%s.csv", accountID, asOfDateText), content)
		return
	}
	if format == "xlsx" {
		content, err := accountBalanceXLSX(accountID, asOfDateText, balanceText)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "Failed to export account balance XLSX")
			return
		}
		respondReportXLSX(w, fmt.Sprintf("account-balance-%s-%s.xlsx", accountID, asOfDateText), content)
		return
	}
	if format == "pdf" {
		content, err := accountBalancePDF(accountID, asOfDateText, balanceText)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "Failed to export account balance PDF")
			return
		}
		respondReportPDF(w, fmt.Sprintf("account-balance-%s-%s.pdf", accountID, asOfDateText), content)
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"account_id": accountID,
		"as_of_date": asOfDateText,
		"balance":    balanceText,
	})
}

// GetBalanceSheet returns the balance sheet for a tenant
// @Summary Get balance sheet
// @Description Get balance sheet report as of a specific date
// @Tags Reports
// @Produce json,text/csv,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet,application/pdf
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param as_of query string false "As of date (YYYY-MM-DD)"
// @Param format query string false "Response format: json, csv, xlsx, or pdf"
// @Success 200 {object} accounting.BalanceSheet
// @Failure 400 {object} object{error=string}
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/reports/balance-sheet [get]
func (h *Handlers) GetBalanceSheet(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	format, err := reportResponseFormat(r)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

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

	if format == "csv" {
		content, err := balanceSheetCSV(bs)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "Failed to export balance sheet CSV")
			return
		}
		respondReportCSV(w, fmt.Sprintf("balance-sheet-%s.csv", asOfDate.Format("2006-01-02")), content)
		return
	}
	if format == "xlsx" {
		content, err := balanceSheetXLSX(bs)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "Failed to export balance sheet XLSX")
			return
		}
		respondReportXLSX(w, fmt.Sprintf("balance-sheet-%s.xlsx", asOfDate.Format("2006-01-02")), content)
		return
	}
	if format == "pdf" {
		content, err := balanceSheetPDF(bs, asOfDate)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "Failed to export balance sheet PDF")
			return
		}
		respondReportPDF(w, fmt.Sprintf("balance-sheet-%s.pdf", asOfDate.Format("2006-01-02")), content)
		return
	}

	respondJSON(w, http.StatusOK, bs)
}

// GetIncomeStatement returns the income statement for a tenant
// @Summary Get income statement
// @Description Get income statement (P&L) report for a specific period
// @Tags Reports
// @Produce json,text/csv,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet,application/pdf
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param start query string true "Start date (YYYY-MM-DD)"
// @Param end query string true "End date (YYYY-MM-DD)"
// @Param format query string false "Response format: json, csv, xlsx, or pdf"
// @Success 200 {object} accounting.IncomeStatement
// @Failure 400 {object} object{error=string}
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/reports/income-statement [get]
func (h *Handlers) GetIncomeStatement(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")

	format, err := reportResponseFormat(r)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

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

	if format == "csv" {
		content, err := incomeStatementCSV(is)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "Failed to export income statement CSV")
			return
		}
		respondReportCSV(w, fmt.Sprintf("income-statement-%s-%s.csv", startDate.Format("2006-01-02"), endDate.Format("2006-01-02")), content)
		return
	}
	if format == "xlsx" {
		content, err := incomeStatementXLSX(is)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "Failed to export income statement XLSX")
			return
		}
		respondReportXLSX(w, fmt.Sprintf("income-statement-%s-%s.xlsx", startDate.Format("2006-01-02"), endDate.Format("2006-01-02")), content)
		return
	}
	if format == "pdf" {
		content, err := incomeStatementPDF(is, startDate, endDate)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "Failed to export income statement PDF")
			return
		}
		respondReportPDF(w, fmt.Sprintf("income-statement-%s-%s.pdf", startDate.Format("2006-01-02"), endDate.Format("2006-01-02")), content)
		return
	}

	respondJSON(w, http.StatusOK, is)
}

// GetConsolidatedReport returns consolidated financial statements across selected tenant memberships.
// @Summary Get consolidated financial report
// @Description Consolidate trial balance, balance sheet, and income statement across selected companies the caller can view
// @Tags Reports
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Anchor tenant ID"
// @Param tenant_ids query string false "Comma-separated tenant IDs to consolidate; defaults to the anchor tenant"
// @Param tenant_id query []string false "Repeatable tenant ID selector"
// @Param as_of query string false "Balance sheet and trial balance date (YYYY-MM-DD)"
// @Param start query string false "Income statement start date (YYYY-MM-DD); defaults to Jan 1 of as_of year"
// @Param end query string false "Income statement end date (YYYY-MM-DD); defaults to as_of"
// @Success 200 {object} reports.ConsolidatedFinancialReport
// @Failure 400 {object} object{error=string}
// @Failure 403 {object} object{error=string}
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/reports/consolidated [get]
func (h *Handlers) GetConsolidatedReport(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.GetClaims(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	anchorTenantID := chi.URLParam(r, "tenantID")
	tenantIDs, err := parseConsolidatedTenantIDs(r, anchorTenantID)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	asOfDate, startDate, endDate, err := parseConsolidatedReportDates(r)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	allowedTenants, err := h.allowedConsolidationTenants(r.Context(), claims, anchorTenantID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to resolve tenant access")
		return
	}

	entities := make([]reports.ConsolidatedTenantReport, 0, len(tenantIDs))
	for _, tenantID := range tenantIDs {
		tenantRecord, ok := allowedTenants[tenantID]
		if !ok {
			respondError(w, http.StatusForbidden, "Access denied to one or more selected tenants")
			return
		}

		trialBalance, err := h.accountingService.GetTrialBalance(r.Context(), tenantRecord.SchemaName, tenantRecord.ID, asOfDate)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "Failed to generate consolidated trial balance")
			return
		}
		balanceSheet, err := h.accountingService.GetBalanceSheet(r.Context(), tenantRecord.SchemaName, tenantRecord.ID, asOfDate)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "Failed to generate consolidated balance sheet")
			return
		}
		incomeStatement, err := h.accountingService.GetIncomeStatement(r.Context(), tenantRecord.SchemaName, tenantRecord.ID, startDate, endDate)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "Failed to generate consolidated income statement")
			return
		}
		entities = append(entities, reports.ConsolidatedTenantReport{
			TenantID:        tenantRecord.ID,
			TenantName:      tenantRecord.Name,
			TenantSlug:      tenantRecord.Slug,
			TrialBalance:    trialBalance,
			BalanceSheet:    balanceSheet,
			IncomeStatement: incomeStatement,
		})
	}

	report := reports.BuildConsolidatedFinancialReport(anchorTenantID, asOfDate, startDate, endDate, entities)
	respondJSON(w, http.StatusOK, report)
}

func parseConsolidatedTenantIDs(r *http.Request, anchorTenantID string) ([]string, error) {
	rawValues := r.URL.Query()["tenant_id"]
	if raw := strings.TrimSpace(r.URL.Query().Get("tenant_ids")); raw != "" {
		rawValues = append(rawValues, strings.Split(raw, ",")...)
	}
	if len(rawValues) == 0 {
		rawValues = []string{anchorTenantID}
	}

	seen := make(map[string]bool, len(rawValues))
	tenantIDs := make([]string, 0, len(rawValues))
	for _, raw := range rawValues {
		tenantID := strings.TrimSpace(raw)
		if tenantID == "" {
			continue
		}
		if !seen[tenantID] {
			tenantIDs = append(tenantIDs, tenantID)
			seen[tenantID] = true
		}
	}
	if len(tenantIDs) == 0 {
		return nil, fmt.Errorf("at least one tenant_id is required")
	}
	if len(tenantIDs) > 20 {
		return nil, fmt.Errorf("at most 20 tenants can be consolidated at once")
	}
	return tenantIDs, nil
}

func parseConsolidatedReportDates(r *http.Request) (time.Time, time.Time, time.Time, error) {
	asOfRaw := strings.TrimSpace(r.URL.Query().Get("as_of"))
	if asOfRaw == "" {
		asOfRaw = strings.TrimSpace(r.URL.Query().Get("as_of_date"))
	}
	asOfDate := time.Now()
	var err error
	if asOfRaw != "" {
		asOfDate, err = time.Parse("2006-01-02", asOfRaw)
		if err != nil {
			return time.Time{}, time.Time{}, time.Time{}, fmt.Errorf("invalid as_of date format. Use YYYY-MM-DD")
		}
	}

	startDate := time.Date(asOfDate.Year(), 1, 1, 0, 0, 0, 0, time.UTC)
	endDate := asOfDate
	if raw := strings.TrimSpace(r.URL.Query().Get("start")); raw != "" {
		startDate, err = time.Parse("2006-01-02", raw)
		if err != nil {
			return time.Time{}, time.Time{}, time.Time{}, fmt.Errorf("invalid start date format. Use YYYY-MM-DD")
		}
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("end")); raw != "" {
		endDate, err = time.Parse("2006-01-02", raw)
		if err != nil {
			return time.Time{}, time.Time{}, time.Time{}, fmt.Errorf("invalid end date format. Use YYYY-MM-DD")
		}
	}
	if endDate.Before(startDate) {
		return time.Time{}, time.Time{}, time.Time{}, fmt.Errorf("end date must be on or after start date")
	}
	return asOfDate, startDate, endDate, nil
}

func (h *Handlers) allowedConsolidationTenants(ctx context.Context, claims *auth.Claims, anchorTenantID string) (map[string]tenant.Tenant, error) {
	if claims.TokenKind == auth.TokenKindAPIToken {
		if claims.TenantID != "" && claims.TenantID != anchorTenantID {
			return map[string]tenant.Tenant{}, nil
		}
		tenantRecord, err := h.tenantService.GetTenant(ctx, anchorTenantID)
		if err != nil {
			return nil, err
		}
		return map[string]tenant.Tenant{tenantRecord.ID: *tenantRecord}, nil
	}

	memberships, err := h.tenantService.ListUserTenants(ctx, claims.UserID)
	if err != nil {
		return nil, err
	}
	allowed := make(map[string]tenant.Tenant, len(memberships))
	for _, membership := range memberships {
		if tenant.GetRolePermissions(membership.Role).CanViewReports {
			allowed[membership.Tenant.ID] = membership.Tenant
		}
	}
	return allowed, nil
}

// GetAnnualReport returns a fiscal-year annual report pack for a tenant.
// @Summary Get annual report
// @Description Get year-end close readiness plus trial balance, balance sheet, income statement, and cash flow for the fiscal year
// @Tags Reports
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param period_end_date query string true "Fiscal year-end date (YYYY-MM-DD)"
// @Param cash_flow_method query string false "Cash flow method: direct or indirect"
// @Success 200 {object} reports.AnnualReport
// @Failure 400 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Failure 409 {object} object{error=string}
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/reports/annual [get]
func (h *Handlers) GetAnnualReport(w http.ResponseWriter, r *http.Request) {
	routeCtx := h.tenantContextFromRequest(r)
	periodEndDate := strings.TrimSpace(r.URL.Query().Get("period_end_date"))
	if periodEndDate == "" {
		respondError(w, http.StatusBadRequest, "period end date is required")
		return
	}
	cashFlowMethod, err := reports.NormalizeCashFlowMethod(r.URL.Query().Get("cash_flow_method"))
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	tenantRecord, err := h.tenantService.GetTenant(r.Context(), routeCtx.tenantID)
	if err != nil {
		respondError(w, http.StatusNotFound, "Tenant not found")
		return
	}

	pack, err := h.accountingService.GetYearEndClosePack(
		r.Context(),
		routeCtx.schemaName,
		routeCtx.tenantID,
		tenantRecord.Settings.FiscalYearStart,
		periodEndDate,
		tenantRecord.Settings.PeriodLockDate,
	)
	if err != nil {
		respondYearEndCloseError(w, err)
		return
	}
	if pack.Status == nil {
		respondError(w, http.StatusInternalServerError, "Failed to generate annual report")
		return
	}
	if err := h.attachYearEndCloseEvidenceStatus(r.Context(), routeCtx.schemaName, routeCtx.tenantID, pack.Status); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to evaluate close-pack evidence")
		return
	}

	cashFlow, err := h.reportsService.GenerateCashFlowStatement(r.Context(), routeCtx.tenantID, routeCtx.schemaName, &reports.CashFlowRequest{
		StartDate: pack.Status.FiscalYearStartDate,
		EndDate:   pack.Status.FiscalYearEndDate,
		Method:    cashFlowMethod,
	})
	if err != nil {
		log.Error().Err(err).Str("tenant", routeCtx.tenantID).Msg("Failed to generate annual report cash flow")
		respondError(w, http.StatusInternalServerError, "Failed to generate annual report")
		return
	}

	respondJSON(w, http.StatusOK, &reports.AnnualReport{
		TenantID:            routeCtx.tenantID,
		PeriodEndDate:       periodEndDate,
		FiscalYearLabel:     pack.Status.FiscalYearLabel,
		FiscalYearStartDate: pack.Status.FiscalYearStartDate,
		FiscalYearEndDate:   pack.Status.FiscalYearEndDate,
		CloseStatus:         pack.Status,
		TrialBalance:        pack.TrialBalance,
		BalanceSheet:        pack.BalanceSheet,
		IncomeStatement:     pack.IncomeStatement,
		CashFlowStatement:   cashFlow,
		GeneratedAt:         time.Now(),
	})
}

// GetCashFlowStatement returns the cash flow statement for a tenant
// @Summary Get cash flow statement
// @Description Get cash flow statement report for a specific period (Estonian standard)
// @Tags Reports
// @Produce json,text/csv,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet,application/pdf
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param start_date query string true "Start date (YYYY-MM-DD)"
// @Param end_date query string true "End date (YYYY-MM-DD)"
// @Param method query string false "Cash flow method: direct or indirect"
// @Param operating_accounts query string false "Comma-separated account codes to force into operating cash flow"
// @Param investing_accounts query string false "Comma-separated account codes to force into investing cash flow"
// @Param financing_accounts query string false "Comma-separated account codes to force into financing cash flow"
// @Param format query string false "Response format: json, csv, xlsx, or pdf"
// @Success 200 {object} reports.CashFlowStatement
// @Failure 400 {object} object{error=string}
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/reports/cash-flow [get]
func (h *Handlers) GetCashFlowStatement(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")

	format, err := reportResponseFormat(r)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	startDateStr := r.URL.Query().Get("start_date")
	endDateStr := r.URL.Query().Get("end_date")
	method, err := reports.NormalizeCashFlowMethod(r.URL.Query().Get("method"))
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	if startDateStr == "" || endDateStr == "" {
		respondError(w, http.StatusBadRequest, "start_date and end_date parameters are required")
		return
	}

	startDate, err := time.Parse("2006-01-02", startDateStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid start_date format. Use YYYY-MM-DD")
		return
	}

	endDate, err := time.Parse("2006-01-02", endDateStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid end_date format. Use YYYY-MM-DD")
		return
	}
	if endDate.Before(startDate) {
		respondError(w, http.StatusBadRequest, "end_date must be on or after start_date")
		return
	}

	schemaName := h.getSchemaName(r.Context(), tenantID)
	req := &reports.CashFlowRequest{
		StartDate: startDateStr,
		EndDate:   endDateStr,
		Method:    method,
		MappingOverrides: reports.CashFlowMappingOverrides{
			OperatingAccountCodes: splitCSVQueryParam(r.URL.Query().Get("operating_accounts")),
			InvestingAccountCodes: splitCSVQueryParam(r.URL.Query().Get("investing_accounts")),
			FinancingAccountCodes: splitCSVQueryParam(r.URL.Query().Get("financing_accounts")),
		},
	}
	req.MappingOverrides, err = reports.NormalizeCashFlowMappingOverrides(req.MappingOverrides)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	result, err := h.reportsService.GenerateCashFlowStatement(r.Context(), tenantID, schemaName, req)
	if err != nil {
		log.Error().Err(err).Str("tenant", tenantID).Msg("Failed to generate cash flow statement")
		respondError(w, http.StatusInternalServerError, "Failed to generate cash flow statement")
		return
	}

	if format == "csv" {
		content, err := cashFlowStatementCSV(result)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "Failed to export cash flow CSV")
			return
		}
		respondReportCSV(w, fmt.Sprintf("cash-flow-%s-%s.csv", startDateStr, endDateStr), content)
		return
	}
	if format == "xlsx" {
		content, err := cashFlowStatementXLSX(result)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "Failed to export cash flow XLSX")
			return
		}
		respondReportXLSX(w, fmt.Sprintf("cash-flow-%s-%s.xlsx", startDateStr, endDateStr), content)
		return
	}
	if format == "pdf" {
		content, err := cashFlowStatementPDF(result)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "Failed to export cash flow PDF")
			return
		}
		respondReportPDF(w, fmt.Sprintf("cash-flow-%s-%s.pdf", startDateStr, endDateStr), content)
		return
	}

	respondJSON(w, http.StatusOK, result)
}

// GetCashFlowMapping returns tenant-level cash-flow account mapping settings
// @Summary Get cash flow mapping
// @Description Get tenant-level cash-flow account-code mapping settings
// @Tags Reports
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Success 200 {object} reports.CashFlowMappingOverrides
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/reports/cash-flow/mapping [get]
func (h *Handlers) GetCashFlowMapping(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")

	mapping, err := h.reportsService.GetCashFlowMapping(r.Context(), tenantID)
	if err != nil {
		log.Error().Err(err).Str("tenant", tenantID).Msg("Failed to get cash flow mapping")
		respondError(w, http.StatusInternalServerError, "Failed to get cash flow mapping")
		return
	}

	respondJSON(w, http.StatusOK, mapping)
}

// UpdateCashFlowMapping updates tenant-level cash-flow account mapping settings
// @Summary Update cash flow mapping
// @Description Replace tenant-level cash-flow account-code mapping settings
// @Tags Reports
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param request body reports.UpdateCashFlowMappingRequest true "Cash-flow mapping settings"
// @Success 200 {object} reports.CashFlowMappingOverrides
// @Failure 400 {object} object{error=string}
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/reports/cash-flow/mapping [put]
func (h *Handlers) UpdateCashFlowMapping(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")

	var req reports.UpdateCashFlowMappingRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	normalized, err := reports.NormalizeCashFlowMappingOverrides(req)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	mapping, err := h.reportsService.UpdateCashFlowMapping(r.Context(), tenantID, normalized)
	if err != nil {
		log.Error().Err(err).Str("tenant", tenantID).Msg("Failed to update cash flow mapping")
		respondError(w, http.StatusInternalServerError, "Failed to update cash flow mapping")
		return
	}

	respondJSON(w, http.StatusOK, mapping)
}

func splitCSVQueryParam(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

// GetBalanceConfirmationSummary returns a summary of all outstanding balances by contact
// @Summary Get balance confirmation summary
// @Description Get a summary of all receivables or payables grouped by contact
// @Tags Reports
// @Produce json,text/csv,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet,application/pdf
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param type query string true "Balance type (RECEIVABLE or PAYABLE)"
// @Param as_of_date query string true "As of date (YYYY-MM-DD)"
// @Param format query string false "Response format: json, csv, xlsx, or pdf"
// @Success 200 {object} reports.BalanceConfirmationSummary
// @Failure 400 {object} object{error=string}
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/reports/balance-confirmations [get]
func (h *Handlers) GetBalanceConfirmationSummary(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")

	balanceType := r.URL.Query().Get("type")
	asOfDate := r.URL.Query().Get("as_of_date")

	format, err := reportResponseFormat(r)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	if balanceType == "" {
		respondError(w, http.StatusBadRequest, "type parameter is required (RECEIVABLE or PAYABLE)")
		return
	}

	if balanceType != "RECEIVABLE" && balanceType != "PAYABLE" {
		respondError(w, http.StatusBadRequest, "type must be RECEIVABLE or PAYABLE")
		return
	}

	if asOfDate == "" {
		respondError(w, http.StatusBadRequest, "as_of_date parameter is required")
		return
	}

	// Validate date format
	_, err = time.Parse("2006-01-02", asOfDate)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid as_of_date format. Use YYYY-MM-DD")
		return
	}

	schemaName := h.getSchemaName(r.Context(), tenantID)
	req := &reports.BalanceConfirmationRequest{
		Type:     balanceType,
		AsOfDate: asOfDate,
	}

	result, err := h.reportsService.GetBalanceConfirmationSummary(r.Context(), tenantID, schemaName, req)
	if err != nil {
		log.Error().Err(err).Str("tenant", tenantID).Msg("Failed to get balance confirmation summary")
		respondError(w, http.StatusInternalServerError, "Failed to get balance confirmation summary")
		return
	}

	if format == "csv" {
		content, err := balanceConfirmationSummaryCSV(result)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "Failed to export balance confirmations CSV")
			return
		}
		respondReportCSV(w, fmt.Sprintf("balance-confirmations-%s-%s.csv", strings.ToLower(balanceType), asOfDate), content)
		return
	}
	if format == "xlsx" {
		content, err := balanceConfirmationSummaryXLSX(result)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "Failed to export balance confirmations XLSX")
			return
		}
		respondReportXLSX(w, fmt.Sprintf("balance-confirmations-%s-%s.xlsx", strings.ToLower(balanceType), asOfDate), content)
		return
	}
	if format == "pdf" {
		content, err := balanceConfirmationSummaryPDF(result)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "Failed to export balance confirmations PDF")
			return
		}
		respondReportPDF(w, fmt.Sprintf("balance-confirmations-%s-%s.pdf", strings.ToLower(balanceType), asOfDate), content)
		return
	}

	respondJSON(w, http.StatusOK, result)
}

// GetBalanceConfirmation returns balance confirmation for a specific contact
// @Summary Get balance confirmation for contact
// @Description Get detailed balance confirmation with invoices for a specific contact
// @Tags Reports
// @Produce json,text/csv,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet,application/pdf
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param contactID path string true "Contact ID"
// @Param type query string true "Balance type (RECEIVABLE or PAYABLE)"
// @Param as_of_date query string true "As of date (YYYY-MM-DD)"
// @Param format query string false "Response format: json, csv, xlsx, or pdf"
// @Success 200 {object} reports.BalanceConfirmation
// @Failure 400 {object} object{error=string}
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/reports/balance-confirmations/{contactID} [get]
func (h *Handlers) GetBalanceConfirmation(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	contactID := chi.URLParam(r, "contactID")

	balanceType := r.URL.Query().Get("type")
	asOfDate := r.URL.Query().Get("as_of_date")

	format, err := reportResponseFormat(r)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	if balanceType == "" {
		respondError(w, http.StatusBadRequest, "type parameter is required (RECEIVABLE or PAYABLE)")
		return
	}

	if balanceType != "RECEIVABLE" && balanceType != "PAYABLE" {
		respondError(w, http.StatusBadRequest, "type must be RECEIVABLE or PAYABLE")
		return
	}

	if asOfDate == "" {
		respondError(w, http.StatusBadRequest, "as_of_date parameter is required")
		return
	}

	// Validate date format
	_, err = time.Parse("2006-01-02", asOfDate)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid as_of_date format. Use YYYY-MM-DD")
		return
	}

	schemaName := h.getSchemaName(r.Context(), tenantID)
	req := &reports.BalanceConfirmationRequest{
		ContactID: contactID,
		Type:      balanceType,
		AsOfDate:  asOfDate,
	}

	result, err := h.reportsService.GetBalanceConfirmation(r.Context(), tenantID, schemaName, req)
	if err != nil {
		log.Error().Err(err).Str("tenant", tenantID).Str("contact", contactID).Msg("Failed to get balance confirmation")
		respondError(w, http.StatusInternalServerError, "Failed to get balance confirmation")
		return
	}

	if format == "csv" {
		content, err := balanceConfirmationCSV(result)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "Failed to export balance confirmation CSV")
			return
		}
		respondReportCSV(w, fmt.Sprintf("balance-confirmation-%s-%s.csv", contactID, asOfDate), content)
		return
	}
	if format == "xlsx" {
		content, err := balanceConfirmationXLSX(result)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "Failed to export balance confirmation XLSX")
			return
		}
		respondReportXLSX(w, fmt.Sprintf("balance-confirmation-%s-%s.xlsx", contactID, asOfDate), content)
		return
	}
	if format == "pdf" {
		content, err := balanceConfirmationPDF(result)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "Failed to export balance confirmation PDF")
			return
		}
		respondReportPDF(w, fmt.Sprintf("balance-confirmation-%s-%s.pdf", contactID, asOfDate), content)
		return
	}

	respondJSON(w, http.StatusOK, result)
}

// GetContactStatement returns periodic statement activity for one customer or supplier
// @Summary Get contact statement
// @Description Get opening balance, invoice/payment activity, and closing balance for one customer or supplier over a period
// @Tags Reports
// @Produce json,text/csv,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet,application/pdf
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param contactID path string true "Contact ID"
// @Param type query string true "Statement type (RECEIVABLE or PAYABLE)"
// @Param start_date query string true "Start date (YYYY-MM-DD)"
// @Param end_date query string true "End date (YYYY-MM-DD)"
// @Param format query string false "Response format: json, csv, xlsx, or pdf"
// @Success 200 {object} reports.ContactStatement
// @Failure 400 {object} object{error=string}
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/reports/contact-statements/{contactID} [get]
func (h *Handlers) GetContactStatement(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	contactID := chi.URLParam(r, "contactID")

	format, err := reportResponseFormat(r)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	req, err := contactStatementRequestFromQuery(contactID, r)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	schemaName := h.getSchemaName(r.Context(), tenantID)
	result, err := h.reportsService.GetContactStatement(r.Context(), tenantID, schemaName, req)
	if err != nil {
		log.Error().Err(err).Str("tenant", tenantID).Str("contact", contactID).Msg("Failed to get contact statement")
		respondError(w, http.StatusInternalServerError, "Failed to get contact statement")
		return
	}

	fileStem := fmt.Sprintf("contact-statement-%s-%s-%s", contactID, req.StartDate, req.EndDate)
	if format == "csv" {
		content, err := contactStatementCSV(result)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "Failed to export contact statement CSV")
			return
		}
		respondReportCSV(w, fileStem+".csv", content)
		return
	}
	if format == "xlsx" {
		content, err := contactStatementXLSX(result)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "Failed to export contact statement XLSX")
			return
		}
		respondReportXLSX(w, fileStem+".xlsx", content)
		return
	}
	if format == "pdf" {
		content, err := contactStatementPDF(result)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "Failed to export contact statement PDF")
			return
		}
		respondReportPDF(w, fileStem+".pdf", content)
		return
	}

	respondJSON(w, http.StatusOK, result)
}

func contactStatementRequestFromQuery(contactID string, r *http.Request) (*reports.ContactStatementRequest, error) {
	balanceType := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("type")))
	if balanceType == "" {
		return nil, fmt.Errorf("type parameter is required (RECEIVABLE or PAYABLE)")
	}
	if balanceType != string(reports.BalanceTypeReceivable) && balanceType != string(reports.BalanceTypePayable) {
		return nil, fmt.Errorf("type must be RECEIVABLE or PAYABLE")
	}

	startDate := strings.TrimSpace(r.URL.Query().Get("start_date"))
	if startDate == "" {
		return nil, fmt.Errorf("start_date parameter is required")
	}
	parsedStart, err := time.Parse("2006-01-02", startDate)
	if err != nil {
		return nil, fmt.Errorf("invalid start_date format. Use YYYY-MM-DD")
	}

	endDate := strings.TrimSpace(r.URL.Query().Get("end_date"))
	if endDate == "" {
		return nil, fmt.Errorf("end_date parameter is required")
	}
	parsedEnd, err := time.Parse("2006-01-02", endDate)
	if err != nil {
		return nil, fmt.Errorf("invalid end_date format. Use YYYY-MM-DD")
	}
	if parsedEnd.Before(parsedStart) {
		return nil, fmt.Errorf("end_date must be on or after start_date")
	}

	if strings.TrimSpace(contactID) == "" {
		return nil, fmt.Errorf("contactID path parameter is required")
	}
	return &reports.ContactStatementRequest{
		ContactID: strings.TrimSpace(contactID),
		Type:      balanceType,
		StartDate: startDate,
		EndDate:   endDate,
	}, nil
}

// GetSalesMarginReport returns sales margin reporting for a period
// @Summary Get sales margin report
// @Description Get sales invoice revenue, estimated product cost, and margin by invoice line
// @Tags Reports
// @Produce json,text/csv,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet,application/pdf
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param start_date query string true "Start date (YYYY-MM-DD)"
// @Param end_date query string true "End date (YYYY-MM-DD)"
// @Param format query string false "Response format: json, csv, xlsx, or pdf"
// @Success 200 {object} reports.SalesMarginReport
// @Failure 400 {object} object{error=string}
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/reports/sales-margin [get]
func (h *Handlers) GetSalesMarginReport(w http.ResponseWriter, r *http.Request) {
	h.getSalesMarginLikeReport(w, r, "sales margin", "sales-margin")
}

// GetCustomerProfitabilityReport returns customer profitability reporting for a period
// @Summary Get customer profitability report
// @Description Get customer-level revenue, estimated product cost, margin, and supporting sales invoice line detail
// @Tags Reports
// @Produce json,text/csv,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet,application/pdf
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param start_date query string true "Start date (YYYY-MM-DD)"
// @Param end_date query string true "End date (YYYY-MM-DD)"
// @Param format query string false "Response format: json, csv, xlsx, or pdf"
// @Success 200 {object} reports.SalesMarginReport
// @Failure 400 {object} object{error=string}
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/reports/customer-profitability [get]
func (h *Handlers) GetCustomerProfitabilityReport(w http.ResponseWriter, r *http.Request) {
	h.getSalesMarginLikeReport(w, r, "customer profitability", "customer-profitability")
}

func (h *Handlers) getSalesMarginLikeReport(w http.ResponseWriter, r *http.Request, reportName, fileStemPrefix string) {
	tenantID := chi.URLParam(r, "tenantID")

	format, err := reportResponseFormat(r)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	req, err := salesMarginRequestFromQuery(r)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	schemaName := h.getSchemaName(r.Context(), tenantID)
	result, err := h.reportsService.GetSalesMarginReport(r.Context(), tenantID, schemaName, req)
	if err != nil {
		log.Error().Err(err).Str("tenant", tenantID).Str("report", reportName).Msg("Failed to get report")
		respondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to get %s report", reportName))
		return
	}

	fileStem := fmt.Sprintf("%s-%s-%s", fileStemPrefix, req.StartDate, req.EndDate)
	if format == "csv" {
		content, err := salesMarginCSV(result)
		if err != nil {
			respondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to export %s CSV", reportName))
			return
		}
		respondReportCSV(w, fileStem+".csv", content)
		return
	}
	if format == "xlsx" {
		content, err := salesMarginXLSX(result)
		if err != nil {
			respondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to export %s XLSX", reportName))
			return
		}
		respondReportXLSX(w, fileStem+".xlsx", content)
		return
	}
	if format == "pdf" {
		content, err := salesMarginPDF(result)
		if err != nil {
			respondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to export %s PDF", reportName))
			return
		}
		respondReportPDF(w, fileStem+".pdf", content)
		return
	}

	respondJSON(w, http.StatusOK, result)
}

func salesMarginRequestFromQuery(r *http.Request) (*reports.SalesMarginRequest, error) {
	startDate := strings.TrimSpace(r.URL.Query().Get("start_date"))
	if startDate == "" {
		return nil, fmt.Errorf("start_date parameter is required")
	}
	parsedStart, err := time.Parse("2006-01-02", startDate)
	if err != nil {
		return nil, fmt.Errorf("invalid start_date format. Use YYYY-MM-DD")
	}

	endDate := strings.TrimSpace(r.URL.Query().Get("end_date"))
	if endDate == "" {
		return nil, fmt.Errorf("end_date parameter is required")
	}
	parsedEnd, err := time.Parse("2006-01-02", endDate)
	if err != nil {
		return nil, fmt.Errorf("invalid end_date format. Use YYYY-MM-DD")
	}
	if parsedEnd.Before(parsedStart) {
		return nil, fmt.Errorf("end_date must be on or after start_date")
	}

	return &reports.SalesMarginRequest{
		StartDate: startDate,
		EndDate:   endDate,
	}, nil
}

// GetOverdueInvoices returns a summary of all overdue invoices for payment reminders
// @Summary Get overdue invoices
// @Description Get a summary of all overdue sales invoices for sending payment reminders
// @Tags Reminders
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Success 200 {object} invoicing.OverdueInvoicesSummary
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/invoices/overdue [get]
func (h *Handlers) GetOverdueInvoices(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	result, err := h.reminderService.GetOverdueInvoicesSummary(r.Context(), tenantID, schemaName)
	if err != nil {
		log.Error().Err(err).Str("tenant", tenantID).Msg("Failed to get overdue invoices")
		respondError(w, http.StatusInternalServerError, "Failed to get overdue invoices")
		return
	}

	respondJSON(w, http.StatusOK, result)
}

// SendPaymentReminder sends a payment reminder for a specific invoice
// @Summary Send payment reminder
// @Description Send a payment reminder email for an overdue invoice
// @Tags Reminders
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param request body invoicing.SendReminderRequest true "Reminder request"
// @Success 200 {object} invoicing.ReminderResult
// @Failure 400 {object} object{error=string}
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/invoices/reminders [post]
func (h *Handlers) SendPaymentReminder(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")

	var req invoicing.SendReminderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.InvoiceID == "" {
		respondError(w, http.StatusBadRequest, "invoice_id is required")
		return
	}

	schemaName := h.getSchemaName(r.Context(), tenantID)

	// Get company name for email template
	t, err := h.tenantService.GetTenant(r.Context(), tenantID)
	companyName := "Open Accounting"
	if err == nil && t.Name != "" {
		companyName = t.Name
	}

	result, err := h.reminderService.SendReminder(r.Context(), tenantID, schemaName, &req, companyName)
	if err != nil {
		log.Error().Err(err).Str("tenant", tenantID).Str("invoice", req.InvoiceID).Msg("Failed to send payment reminder")
		respondError(w, http.StatusInternalServerError, "Failed to send payment reminder")
		return
	}

	respondJSON(w, http.StatusOK, result)
}

// SendBulkPaymentReminders sends payment reminders for multiple invoices
// @Summary Send bulk payment reminders
// @Description Send payment reminder emails for multiple overdue invoices
// @Tags Reminders
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param request body invoicing.SendBulkRemindersRequest true "Bulk reminder request"
// @Success 200 {object} invoicing.BulkReminderResult
// @Failure 400 {object} object{error=string}
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/invoices/reminders/bulk [post]
func (h *Handlers) SendBulkPaymentReminders(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")

	var req invoicing.SendBulkRemindersRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if len(req.InvoiceIDs) == 0 {
		respondError(w, http.StatusBadRequest, "invoice_ids is required and must not be empty")
		return
	}

	schemaName := h.getSchemaName(r.Context(), tenantID)

	// Get company name for email template
	t, err := h.tenantService.GetTenant(r.Context(), tenantID)
	companyName := "Open Accounting"
	if err == nil && t.Name != "" {
		companyName = t.Name
	}

	result, err := h.reminderService.SendBulkReminders(r.Context(), tenantID, schemaName, &req, companyName)
	if err != nil {
		log.Error().Err(err).Str("tenant", tenantID).Msg("Failed to send bulk payment reminders")
		respondError(w, http.StatusInternalServerError, "Failed to send bulk payment reminders")
		return
	}

	respondJSON(w, http.StatusOK, result)
}

// GetInvoiceReminderHistory gets the reminder history for an invoice
// @Summary Get invoice reminder history
// @Description Get the history of payment reminders sent for an invoice
// @Tags Reminders
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param invoiceID path string true "Invoice ID"
// @Success 200 {array} invoicing.PaymentReminder
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/invoices/{invoiceID}/reminders [get]
func (h *Handlers) GetInvoiceReminderHistory(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	invoiceID := chi.URLParam(r, "invoiceID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	reminders, err := h.reminderService.GetReminderHistory(r.Context(), tenantID, schemaName, invoiceID)
	if err != nil {
		log.Error().Err(err).Str("tenant", tenantID).Str("invoice", invoiceID).Msg("Failed to get reminder history")
		respondError(w, http.StatusInternalServerError, "Failed to get reminder history")
		return
	}

	if reminders == nil {
		reminders = []invoicing.PaymentReminder{}
	}

	respondJSON(w, http.StatusOK, reminders)
}

// Custom JSON marshaling for decimal values
func init() {
	// Register decimal type for proper JSON encoding
	decimal.MarshalJSONWithoutQuotes = true
}

// Cost Center Handlers

// ListCostCenters handles GET /tenants/{tenantID}/cost-centers.
// @Summary List cost centers
// @Description List cost centers for a tenant, optionally filtering to active centers
// @Tags Cost Centers
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param active_only query bool false "Only include active cost centers"
// @Success 200 {array} accounting.CostCenter
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/cost-centers [get]
func (h *Handlers) ListCostCenters(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	activeOnlyStr := r.URL.Query().Get("active_only")
	activeOnly := activeOnlyStr == "true"

	costCenters, err := h.costCenterService.ListCostCenters(r.Context(), schemaName, tenantID, activeOnly)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, costCenters)
}

// GetCostCenter handles GET /tenants/{tenantID}/cost-centers/{costCenterID}.
// @Summary Get cost center
// @Description Get one cost center by ID
// @Tags Cost Centers
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param costCenterID path string true "Cost center ID"
// @Success 200 {object} accounting.CostCenter
// @Failure 404 {object} object{error=string}
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/cost-centers/{costCenterID} [get]
func (h *Handlers) GetCostCenter(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	costCenterID := chi.URLParam(r, "costCenterID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	cc, err := h.costCenterService.GetCostCenter(r.Context(), schemaName, tenantID, costCenterID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			respondError(w, http.StatusNotFound, err.Error())
			return
		}
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, cc)
}

// CreateCostCenter handles POST /tenants/{tenantID}/cost-centers.
// @Summary Create cost center
// @Description Create a tenant cost center for expense tracking and budgeting
// @Tags Cost Centers
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param request body accounting.CreateCostCenterRequest true "Cost center"
// @Success 201 {object} accounting.CostCenter
// @Failure 400 {object} object{error=string}
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/cost-centers [post]
func (h *Handlers) CreateCostCenter(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	var req accounting.CreateCostCenterRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	cc, err := h.costCenterService.CreateCostCenter(r.Context(), schemaName, tenantID, &req)
	if err != nil {
		if strings.Contains(err.Error(), "required") {
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, cc)
}

// ImportCostCenters handles POST /tenants/{tenantID}/cost-centers/import
// @Summary Import cost centers
// @Description Import cost center master data from CSV and resolve parent cost centers by code
// @Tags Cost Centers
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param request body accounting.ImportCostCentersRequest true "CSV import payload"
// @Success 200 {object} accounting.ImportCostCentersResult
// @Failure 400 {object} object{error=string}
// @Router /tenants/{tenantID}/cost-centers/import [post]
func (h *Handlers) ImportCostCenters(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	var req accounting.ImportCostCentersRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if strings.TrimSpace(req.CSVContent) == "" {
		respondError(w, http.StatusBadRequest, "csv_content is required")
		return
	}

	result, err := h.costCenterService.ImportCostCentersCSV(r.Context(), schemaName, tenantID, &req)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, result)
}

// ListCostAllocations handles GET /tenants/{tenantID}/cost-centers/allocations.
// @Summary List cost allocations
// @Description List journal-entry-line allocations to cost centers, optionally filtered by cost center, journal line, or allocation date range
// @Tags Cost Centers
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param cost_center_id query string false "Cost center ID"
// @Param journal_entry_line_id query string false "Journal entry line ID"
// @Param start_date query string false "Start date (YYYY-MM-DD)"
// @Param end_date query string false "End date (YYYY-MM-DD)"
// @Success 200 {array} accounting.CostAllocation
// @Failure 400 {object} object{error=string}
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/cost-centers/allocations [get]
func (h *Handlers) ListCostAllocations(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	filters := accounting.CostAllocationFilters{
		CostCenterID:       r.URL.Query().Get("cost_center_id"),
		JournalEntryLineID: r.URL.Query().Get("journal_entry_line_id"),
	}
	if startStr := r.URL.Query().Get("start_date"); startStr != "" {
		start, err := time.Parse("2006-01-02", startStr)
		if err != nil {
			respondError(w, http.StatusBadRequest, "Invalid start_date format (use YYYY-MM-DD)")
			return
		}
		filters.StartDate = &start
	}
	if endStr := r.URL.Query().Get("end_date"); endStr != "" {
		end, err := time.Parse("2006-01-02", endStr)
		if err != nil {
			respondError(w, http.StatusBadRequest, "Invalid end_date format (use YYYY-MM-DD)")
			return
		}
		filters.EndDate = &end
	}

	allocations, err := h.costCenterService.ListCostAllocations(r.Context(), schemaName, tenantID, filters)
	if err != nil {
		if strings.Contains(err.Error(), "end_date") {
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, allocations)
}

// CreateCostAllocation handles POST /tenants/{tenantID}/cost-centers/allocations.
// @Summary Create cost allocation
// @Description Assign a positive journal-entry-line amount to a tenant cost center
// @Tags Cost Centers
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param request body accounting.CreateCostAllocationRequest true "Cost allocation"
// @Success 201 {object} accounting.CostAllocation
// @Failure 400 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/cost-centers/allocations [post]
func (h *Handlers) CreateCostAllocation(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	var req accounting.CreateCostAllocationRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	allocation, err := h.costCenterService.CreateCostAllocation(r.Context(), schemaName, tenantID, &req)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			respondError(w, http.StatusNotFound, err.Error())
			return
		}
		if strings.Contains(err.Error(), "required") || strings.Contains(err.Error(), "greater than zero") || strings.Contains(err.Error(), "between 0 and 100") {
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, allocation)
}

// UpdateCostCenter handles PUT /tenants/{tenantID}/cost-centers/{costCenterID}.
// @Summary Update cost center
// @Description Update cost center details, hierarchy, activity, and budget settings
// @Tags Cost Centers
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param costCenterID path string true "Cost center ID"
// @Param request body accounting.UpdateCostCenterRequest true "Cost center update"
// @Success 200 {object} accounting.CostCenter
// @Failure 400 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/cost-centers/{costCenterID} [put]
func (h *Handlers) UpdateCostCenter(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	costCenterID := chi.URLParam(r, "costCenterID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	var req accounting.UpdateCostCenterRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	cc, err := h.costCenterService.UpdateCostCenter(r.Context(), schemaName, tenantID, costCenterID, &req)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			respondError(w, http.StatusNotFound, err.Error())
			return
		}
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, cc)
}

// DeleteCostCenter handles DELETE /tenants/{tenantID}/cost-centers/{costCenterID}.
// @Summary Delete cost center
// @Description Delete a cost center that has no blocking usage
// @Tags Cost Centers
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param costCenterID path string true "Cost center ID"
// @Success 204 "No Content"
// @Failure 404 {object} object{error=string}
// @Failure 409 {object} object{error=string}
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/cost-centers/{costCenterID} [delete]
func (h *Handlers) DeleteCostCenter(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	costCenterID := chi.URLParam(r, "costCenterID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	err := h.costCenterService.DeleteCostCenter(r.Context(), schemaName, tenantID, costCenterID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			respondError(w, http.StatusNotFound, err.Error())
			return
		}
		if strings.Contains(err.Error(), "cannot delete") {
			respondError(w, http.StatusConflict, err.Error())
			return
		}
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GetCostCenterReport handles GET /tenants/{tenantID}/cost-centers/report
// @Summary Get cost center budget report
// @Description Get cost center budget and expense report, with optional CSV, XLSX, or PDF export
// @Tags Cost Centers
// @Produce json,text/csv,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet,application/pdf
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param start_date query string false "Start date (YYYY-MM-DD)"
// @Param end_date query string false "End date (YYYY-MM-DD)"
// @Param format query string false "Response format: json, csv, xlsx, or pdf"
// @Success 200 {object} accounting.CostCenterReport
// @Failure 400 {object} object{error=string}
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/cost-centers/report [get]
func (h *Handlers) GetCostCenterReport(w http.ResponseWriter, r *http.Request) {
	h.writeCostCenterBudgetReport(w, r, "cost-center-report")
}

// GetBudgetVsActualReport handles GET /tenants/{tenantID}/reports/budget-vs-actual
// @Summary Get budget vs actual report
// @Description Get budget versus actual expenses by cost center, with optional CSV, XLSX, or PDF export
// @Tags Reports
// @Produce json,text/csv,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet,application/pdf
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param start_date query string false "Start date (YYYY-MM-DD)"
// @Param end_date query string false "End date (YYYY-MM-DD)"
// @Param format query string false "Response format: json, csv, xlsx, or pdf"
// @Success 200 {object} accounting.CostCenterReport
// @Failure 400 {object} object{error=string}
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/reports/budget-vs-actual [get]
func (h *Handlers) GetBudgetVsActualReport(w http.ResponseWriter, r *http.Request) {
	h.writeCostCenterBudgetReport(w, r, "budget-vs-actual")
}

func (h *Handlers) writeCostCenterBudgetReport(w http.ResponseWriter, r *http.Request, filenamePrefix string) {
	tenantID := chi.URLParam(r, "tenantID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	format, err := reportResponseFormat(r)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Parse date range from query params
	startStr := r.URL.Query().Get("start_date")
	endStr := r.URL.Query().Get("end_date")

	var start, end time.Time

	if startStr != "" {
		start, err = time.Parse("2006-01-02", startStr)
		if err != nil {
			respondError(w, http.StatusBadRequest, "Invalid start_date format (use YYYY-MM-DD)")
			return
		}
	} else {
		// Default to start of current year
		now := time.Now()
		start = time.Date(now.Year(), 1, 1, 0, 0, 0, 0, time.UTC)
	}

	if endStr != "" {
		end, err = time.Parse("2006-01-02", endStr)
		if err != nil {
			respondError(w, http.StatusBadRequest, "Invalid end_date format (use YYYY-MM-DD)")
			return
		}
	} else {
		// Default to today
		end = time.Now()
	}
	if end.Before(start) {
		respondError(w, http.StatusBadRequest, "end_date must be on or after start_date")
		return
	}

	report, err := h.costCenterService.GetCostCenterReport(r.Context(), schemaName, tenantID, start, end)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if format == "csv" {
		content, err := costCenterReportCSV(report)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "Failed to export cost center report CSV")
			return
		}
		respondReportCSV(w, fmt.Sprintf("%s-%s-%s.csv", filenamePrefix, reportExportDate(report.PeriodStart), reportExportDate(report.PeriodEnd)), content)
		return
	}
	if format == "xlsx" {
		content, err := costCenterReportXLSX(report)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "Failed to export cost center report XLSX")
			return
		}
		respondReportXLSX(w, fmt.Sprintf("%s-%s-%s.xlsx", filenamePrefix, reportExportDate(report.PeriodStart), reportExportDate(report.PeriodEnd)), content)
		return
	}
	if format == "pdf" {
		content, err := costCenterReportPDF(report)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "Failed to export cost center report PDF")
			return
		}
		respondReportPDF(w, fmt.Sprintf("%s-%s-%s.pdf", filenamePrefix, reportExportDate(report.PeriodStart), reportExportDate(report.PeriodEnd)), content)
		return
	}

	respondJSON(w, http.StatusOK, report)
}
