package main

import (
	"github.com/go-chi/chi/v5"
	httpSwagger "github.com/swaggo/http-swagger/v2"
)

func registerPublicRoutes(r chi.Router, h *Handlers) {
	// Health check
	r.Get("/health", healthCheck)

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

	// Public invitation endpoints (no auth required)
	r.Get("/invitations/{token}", h.GetInvitationByToken)
	r.Post("/invitations/accept", h.AcceptInvitation)
}
