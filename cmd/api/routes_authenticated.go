package main

import (
	"github.com/go-chi/chi/v5"

	"github.com/HMB-research/open-accounting/internal/auth"
	"github.com/HMB-research/open-accounting/internal/tenant"
)

func registerAuthenticatedRoutes(r chi.Router, h *Handlers, tokenService *auth.TokenService) {
	canCreateEntries := func(perms tenant.RolePermissions) bool { return perms.CanCreateEntries }
	canManageSettings := func(perms tenant.RolePermissions) bool { return perms.CanManageSettings }

	r.Group(func(r chi.Router) {
		r.Use(tokenService.Middleware)

		// User routes
		r.Get("/me", h.GetCurrentUser)
		r.Get("/me/tenants", h.ListMyTenants)
		r.Put("/auth/password", h.ChangePassword)
		r.Get("/auth/sessions", h.ListAuthSessions)
		r.Delete("/auth/sessions", h.RevokeAllAuthSessions)
		r.Delete("/auth/sessions/{sessionID}", h.RevokeAuthSession)
		r.Get("/auth/security-events", h.ListSecurityAuditEvents)

		// Tenant management
		r.Post("/tenants", h.CreateTenant)

		registerAdminRoutes(r, h)

		registerTenantRoutes(r, h, canCreateEntries, canManageSettings)

		// Register exact tenant management routes after the tenant-scoped
		// subrouter so /tenants/{tenantID} is not shadowed by child routes.
		r.Get("/tenants/{tenantID}", h.GetTenant)
		r.Put("/tenants/{tenantID}", h.UpdateTenant)
	})
}
