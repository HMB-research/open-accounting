package main

import (
	"github.com/go-chi/chi/v5"
	httpSwagger "github.com/swaggo/http-swagger/v2"
)

func registerPublicRoutes(r chi.Router, h *Handlers) {
	// Health check
	r.Get("/health", healthCheck)
	r.Get("/ready", h.ReadinessCheck)

	// Demo endpoints (protected by secret key)
	r.Post("/api/demo/reset", h.DemoReset)
	r.Get("/api/demo/status", h.DemoStatus)

	// Swagger documentation
	r.Get("/swagger/*", httpSwagger.Handler(
		httpSwagger.URL("/swagger/doc.json"),
	))
}

func registerAPIPublicRoutes(r chi.Router, h *Handlers) {
	// Public routes
	r.Post("/auth/register", h.Register)
	r.Post("/auth/login", h.Login)
	r.Post("/auth/refresh", h.RefreshToken)
	r.Post("/auth/logout", h.Logout)
	r.Post("/auth/password-reset/request", h.RequestPasswordReset)
	r.Post("/auth/password-reset/confirm", h.ResetPassword)

	// A short-lived pairing token is the sole authorization for this route.
	// It is restricted to the Brave extension origin in the handler and cannot
	// capture source data or write accounting transactions.
	r.Options("/smartaccounts-browser-pairings/{pairingID}/claim", h.OptionsSmartAccountsBrowserPairingClaim)
	r.Post("/smartaccounts-browser-pairings/{pairingID}/claim", h.ClaimSmartAccountsBrowserPairing)
	r.Options("/smartaccounts-browser-onboarding/catalogs/{catalogID}/handoff", h.OptionsSmartAccountsBrowserOnboardingCatalogHandoff)
	r.Post("/smartaccounts-browser-onboarding/catalogs/{catalogID}/handoff", h.HandoffSmartAccountsBrowserOnboardingCatalog)

	// A capture token is the only authorization for these extension-origin
	// relay routes. Tenant and run are part of each path and must match the
	// immutable token scope; these routes never accept cookies or source APIs.
	r.Options("/smartaccounts-browser-captures/tenants/{tenantID}/runs/{runID}", h.OptionsSmartAccountsBrowserCapture)
	r.Get("/smartaccounts-browser-captures/tenants/{tenantID}/runs/{runID}", h.GetSmartAccountsBrowserCapture)
	r.Put("/smartaccounts-browser-captures/tenants/{tenantID}/runs/{runID}/resources/{resourceID}", h.UploadSmartAccountsBrowserCaptureResource)
	r.Post("/smartaccounts-browser-captures/tenants/{tenantID}/runs/{runID}/finalize", h.FinalizeSmartAccountsBrowserCapture)

	// These extension-only routes relay one owner-authorized, protected master
	// detail artifact. They are never called by OA CLI clients or ordinary web
	// origins, and they cannot apply reference records or finance.
	r.Options("/smartaccounts-browser-master-detail-captures/tenants/{tenantID}/runs/{runID}", h.OptionsSmartAccountsBrowserMasterDetail)
	r.Get("/smartaccounts-browser-master-detail-captures/tenants/{tenantID}/runs/{runID}", h.GetSmartAccountsBrowserMasterDetail)
	r.Put("/smartaccounts-browser-master-detail-captures/tenants/{tenantID}/runs/{runID}/resource", h.UploadSmartAccountsBrowserMasterDetail)
	r.Post("/smartaccounts-browser-master-detail-captures/tenants/{tenantID}/runs/{runID}/finalize", h.FinalizeSmartAccountsBrowserMasterDetail)

	// Commercial extension relays use only an in-memory, run-scoped bearer.
	// The reviewed contract presently rejects source upload/finalize at the
	// list-selector gate, so these routes cannot collect or apply source data.
	r.Options("/smartaccounts-browser-commercial-captures/tenants/{tenantID}/runs/{runID}", h.OptionsSmartAccountsBrowserCommercialDetail)
	r.Get("/smartaccounts-browser-commercial-captures/tenants/{tenantID}/runs/{runID}", h.GetSmartAccountsBrowserCommercialDetail)
	r.Post("/smartaccounts-browser-commercial-captures/tenants/{tenantID}/runs/{runID}", h.StartSmartAccountsBrowserCommercialDetail)
	r.Put("/smartaccounts-browser-commercial-captures/tenants/{tenantID}/runs/{runID}/resource", h.UploadSmartAccountsBrowserCommercialDetail)
	r.Post("/smartaccounts-browser-commercial-captures/tenants/{tenantID}/runs/{runID}/finalize", h.FinalizeSmartAccountsBrowserCommercialDetail)

	// Public invitation endpoints (no auth required)
	r.Get("/invitations/{token}", h.GetInvitationByToken)
	r.Post("/invitations/accept", h.AcceptInvitation)
}
