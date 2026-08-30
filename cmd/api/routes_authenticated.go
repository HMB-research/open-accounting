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
		r.Post("/smartaccounts-sync/browser-onboarding", h.StartSmartAccountsBrowserOnboarding)
		r.Get("/smartaccounts-sync/browser-onboarding/{sourceCompanyID}", h.GetSmartAccountsBrowserOnboarding)
		r.Post("/smartaccounts-sync/browser-onboarding/catalogs", h.IssueSmartAccountsBrowserOnboardingCatalog)
		r.Get("/smartaccounts-sync/browser-onboarding/catalogs/{catalogID}", h.GetSmartAccountsBrowserOnboardingCatalog)
		r.Post("/smartaccounts-sync/browser-onboarding/batches", h.StartSmartAccountsBrowserOnboardingBatch)
		r.Get("/smartaccounts-sync/browser-onboarding/batches/{batchID}", h.GetSmartAccountsBrowserOnboardingBatch)
		r.Post("/smartaccounts-sync/browser-onboarding/batches/{batchID}/resume", h.ResumeSmartAccountsBrowserOnboardingBatch)
		r.Post("/smartaccounts-sync/browser-onboarding/batches/{batchID}/workflow", h.PrepareSmartAccountsBrowserBatchWorkflow)
		r.Get("/smartaccounts-sync/browser-onboarding/batches/{batchID}/workflow", h.GetSmartAccountsBrowserBatchWorkflow)
		r.Post("/smartaccounts-sync/browser-onboarding/batches/{batchID}/workflow/resume", h.ResumeSmartAccountsBrowserBatchWorkflow)
		r.Post("/smartaccounts-sync/browser-onboarding/batches/{batchID}/workflow/advance-safe", h.AdvanceSmartAccountsBrowserBatchWorkflowSafe)
		r.Post("/smartaccounts-sync/browser-onboarding/batches/{batchID}/workflow/discovery/acquire", h.AcquireSmartAccountsBrowserBatchDiscovery)
		r.Post("/smartaccounts-sync/browser-onboarding/batches/{batchID}/workflow/sources/{sourceCompanyID}/discovery/reissue", h.ReissueSmartAccountsBrowserBatchDiscovery)
		r.Post("/smartaccounts-sync/browser-onboarding/batches/{batchID}/workflow/sources/{sourceCompanyID}/discovery/complete", h.CompleteSmartAccountsBrowserBatchDiscovery)
		r.Post("/smartaccounts-sync/browser-onboarding/batches/{batchID}/workflow/sources/{sourceCompanyID}/schema/require", h.RequireSmartAccountsBrowserBatchSchemaReview)
		r.Post("/smartaccounts-sync/browser-onboarding/batches/{batchID}/workflow/sources/{sourceCompanyID}/schema/refresh", h.RefreshSmartAccountsBrowserBatchSchemaReadiness)
		r.Post("/smartaccounts-sync/browser-onboarding/batches/{batchID}/workflow/sources/{sourceCompanyID}/schema/confirm", h.ConfirmSmartAccountsBrowserBatchSchema)
		r.Post("/smartaccounts-sync/browser-onboarding/batches/{batchID}/workflow/transfer/open", h.OpenSmartAccountsBrowserBatchTransferConfirmation)
		r.Post("/smartaccounts-sync/browser-onboarding/batches/{batchID}/workflow/transfer/confirm", h.ConfirmSmartAccountsBrowserBatchTransfer)
		r.Post("/smartaccounts-sync/browser-onboarding/batches/{batchID}/workflow/capture/acquire", h.AcquireSmartAccountsBrowserBatchCapture)
		r.Post("/smartaccounts-sync/browser-onboarding/batches/{batchID}/workflow/sources/{sourceCompanyID}/capture/complete", h.CompleteSmartAccountsBrowserBatchCapture)
		r.Post("/smartaccounts-sync/browser-onboarding/batches/{batchID}/workflow/sources/{sourceCompanyID}/preview", h.PreviewSmartAccountsBrowserBatchPackage)
		r.Post("/smartaccounts-sync/browser-onboarding/batches/{batchID}/sources/{sourceCompanyID}/reconciliation", h.EvaluateSmartAccountsReconciliation)
		r.Get("/smartaccounts-sync/browser-onboarding/batches/{batchID}/sources/{sourceCompanyID}/reconciliation", h.GetSmartAccountsReconciliation)
		r.Get("/smartaccounts-sync/browser-onboarding/batches/{batchID}/reconciliation", h.GetSmartAccountsReconciliationRollup)
		r.Get("/smartaccounts-sync/browser-onboarding/batches/{batchID}/full-claim-eligibility", h.GetSmartAccountsFullClaimEligibility)

		registerAdminRoutes(r, h)

		registerTenantRoutes(r, h, canCreateEntries, canManageSettings)

		// Register exact tenant management routes after the tenant-scoped
		// subrouter so /tenants/{tenantID} is not shadowed by child routes.
		r.Get("/tenants/{tenantID}", h.GetTenant)
		r.Put("/tenants/{tenantID}", h.UpdateTenant)
	})
}
