package main

import "github.com/go-chi/chi/v5"

// registerInternalBridgeRoutes intentionally lives outside browser-authenticated
// tenant routes. Every route is protected by the dedicated short-lived HMAC
// protocol in the handler and must be reachable only on the private Docker
// network. It never exposes raw package data to a browser.
func registerInternalBridgeRoutes(r chi.Router, h *Handlers) {
	r.Get("/internal/bridge/tenants/{tenantID}/packages/{packageID}", h.GetBridgePackageDelivery)
	r.Route("/internal/bridge/tenants/{tenantID}/packages/{packageID}", func(r chi.Router) {
		r.Put("/manifest", h.PutBridgePackageManifest)
		r.Put("/records/{sequence}", h.PutBridgePackageRecords)
		r.Put("/artifacts/{artifactID}/chunks/{sequence}", h.PutBridgePackageArtifactChunk)
		r.Post("/finalize", h.FinalizeBridgePackageDelivery)
	})
}
