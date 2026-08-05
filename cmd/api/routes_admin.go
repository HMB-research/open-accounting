package main

import "github.com/go-chi/chi/v5"

func registerAdminRoutes(r chi.Router, h *Handlers) {
	r.Route("/admin", func(r chi.Router) {
		r.Use(h.RequireInstanceAdmin)

		// Plugin Registries
		r.Get("/plugin-registries", h.ListPluginRegistries)
		r.Post("/plugin-registries", h.AddPluginRegistry)
		r.Delete("/plugin-registries/{id}", h.RemovePluginRegistry)
		r.Post("/plugin-registries/{id}/sync", h.SyncPluginRegistry)

		// Plugin Management
		r.Get("/plugins", h.ListPlugins)
		r.Get("/plugins/search", h.SearchPlugins)
		r.Get("/plugins/permissions", h.GetAllPermissions)
		r.Post("/plugins/install", h.InstallPlugin)
		r.Get("/plugins/{id}", h.GetPlugin)
		r.Delete("/plugins/{id}", h.UninstallPlugin)
		r.Post("/plugins/{id}/enable", h.EnablePlugin)
		r.Post("/plugins/{id}/disable", h.DisablePlugin)
		r.Get("/plugins/{id}/runtime", h.GetPluginRuntimeStatus)
		r.Post("/plugins/{id}/runtime/restart", h.RestartPluginRuntime)
	})
}
