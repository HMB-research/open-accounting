package main

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/HMB-research/open-accounting/internal/auth"
	"github.com/HMB-research/open-accounting/internal/plugin"
)

// Plugin Registry Handlers (Admin only)

// ListPluginRegistries lists all plugin registries
// @Summary List plugin registries
// @Description Get all plugin registries (marketplaces)
// @Tags Plugins
// @Produce json
// @Security BearerAuth
// @Success 200 {array} plugin.Registry
// @Failure 500 {object} object{error=string}
// @Router /admin/plugin-registries [get]
func (h *Handlers) ListPluginRegistries(w http.ResponseWriter, r *http.Request) {
	registries, err := h.pluginService.ListRegistries(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to list registries")
		return
	}

	respondJSON(w, http.StatusOK, registries)
}

// AddPluginRegistry adds a new plugin registry
// @Summary Add plugin registry
// @Description Add a new plugin registry (marketplace source)
// @Tags Plugins
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body plugin.CreateRegistryRequest true "Registry details"
// @Success 201 {object} plugin.Registry
// @Failure 400 {object} object{error=string}
// @Router /admin/plugin-registries [post]
func (h *Handlers) AddPluginRegistry(w http.ResponseWriter, r *http.Request) {
	var req plugin.CreateRegistryRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Name == "" || req.URL == "" {
		respondError(w, http.StatusBadRequest, "Name and URL are required")
		return
	}

	registry, err := h.pluginService.AddRegistry(r.Context(), req)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, registry)
}

// RemovePluginRegistry removes a plugin registry
// @Summary Remove plugin registry
// @Description Remove a plugin registry (cannot remove official registries)
// @Tags Plugins
// @Produce json
// @Security BearerAuth
// @Param id path string true "Registry ID"
// @Success 204 "No Content"
// @Failure 400 {object} object{error=string}
// @Router /admin/plugin-registries/{id} [delete]
func (h *Handlers) RemovePluginRegistry(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid registry ID")
		return
	}

	if err := h.pluginService.RemoveRegistry(r.Context(), id); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// SyncPluginRegistry syncs a plugin registry
// @Summary Sync plugin registry
// @Description Fetch latest plugin list from a registry
// @Tags Plugins
// @Produce json
// @Security BearerAuth
// @Param id path string true "Registry ID"
// @Success 200 {object} object{status=string}
// @Failure 400 {object} object{error=string}
// @Router /admin/plugin-registries/{id}/sync [post]
func (h *Handlers) SyncPluginRegistry(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid registry ID")
		return
	}

	if err := h.pluginService.SyncRegistry(r.Context(), id); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "synced"})
}

// Plugin Management Handlers (Admin only)

// ListPlugins lists all installed plugins
// @Summary List installed plugins
// @Description Get all plugins installed on the instance
// @Tags Plugins
// @Produce json
// @Security BearerAuth
// @Success 200 {array} plugin.Plugin
// @Failure 500 {object} object{error=string}
// @Router /admin/plugins [get]
func (h *Handlers) ListPlugins(w http.ResponseWriter, r *http.Request) {
	plugins, err := h.pluginService.ListPlugins(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to list plugins")
		return
	}

	respondJSON(w, http.StatusOK, plugins)
}

// SearchPlugins searches for plugins across registries
// @Summary Search plugins
// @Description Search for plugins across all active registries
// @Tags Plugins
// @Produce json
// @Security BearerAuth
// @Param q query string true "Search query"
// @Success 200 {array} plugin.PluginSearchResult
// @Failure 400 {object} object{error=string}
// @Router /admin/plugins/search [get]
func (h *Handlers) SearchPlugins(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		respondError(w, http.StatusBadRequest, "Search query is required")
		return
	}

	results, err := h.pluginService.SearchPlugins(r.Context(), query)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, results)
}

// InstallPlugin installs a plugin from a repository
// @Summary Install plugin
// @Description Install a plugin from a Git repository
// @Tags Plugins
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body plugin.InstallPluginRequest true "Repository URL"
// @Success 201 {object} plugin.Plugin
// @Failure 400 {object} object{error=string}
// @Router /admin/plugins/install [post]
func (h *Handlers) InstallPlugin(w http.ResponseWriter, r *http.Request) {
	var req plugin.InstallPluginRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.RepositoryURL == "" {
		respondError(w, http.StatusBadRequest, "Repository URL is required")
		return
	}

	p, err := h.pluginService.InstallPlugin(r.Context(), req.RepositoryURL)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, p)
}

// UninstallPlugin removes an installed plugin
// @Summary Uninstall plugin
// @Description Remove an installed plugin (must not be enabled for any tenant)
// @Tags Plugins
// @Produce json
// @Security BearerAuth
// @Param id path string true "Plugin ID"
// @Success 204 "No Content"
// @Failure 400 {object} object{error=string}
// @Router /admin/plugins/{id} [delete]
func (h *Handlers) UninstallPlugin(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid plugin ID")
		return
	}

	if err := h.pluginService.UninstallPlugin(r.Context(), id); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// EnablePlugin enables a plugin at the instance level
// @Summary Enable plugin
// @Description Enable a plugin for the instance with specified permissions
// @Tags Plugins
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Plugin ID"
// @Param request body plugin.EnablePluginRequest true "Permissions to grant"
// @Success 200 {object} plugin.Plugin
// @Failure 400 {object} object{error=string}
// @Router /admin/plugins/{id}/enable [post]
func (h *Handlers) EnablePlugin(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid plugin ID")
		return
	}

	var req plugin.EnablePluginRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if err := h.pluginService.EnablePlugin(r.Context(), id, req.GrantedPermissions); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	updated, err := h.pluginService.GetPlugin(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to load plugin")
		return
	}

	respondJSON(w, http.StatusOK, updated)
}

// DisablePlugin disables a plugin at the instance level
// @Summary Disable plugin
// @Description Disable a plugin for the entire instance
// @Tags Plugins
// @Produce json
// @Security BearerAuth
// @Param id path string true "Plugin ID"
// @Success 200 {object} plugin.Plugin
// @Failure 400 {object} object{error=string}
// @Router /admin/plugins/{id}/disable [post]
func (h *Handlers) DisablePlugin(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid plugin ID")
		return
	}

	if err := h.pluginService.DisablePlugin(r.Context(), id); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	updated, err := h.pluginService.GetPlugin(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to load plugin")
		return
	}

	respondJSON(w, http.StatusOK, updated)
}

// GetPlugin returns a plugin by ID
// @Summary Get plugin details
// @Description Get details of an installed plugin
// @Tags Plugins
// @Produce json
// @Security BearerAuth
// @Param id path string true "Plugin ID"
// @Success 200 {object} plugin.Plugin
// @Failure 404 {object} object{error=string}
// @Router /admin/plugins/{id} [get]
func (h *Handlers) GetPlugin(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid plugin ID")
		return
	}

	p, err := h.pluginService.GetPlugin(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusNotFound, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, p)
}

// GetPluginRuntimeStatus returns the backend runtime lifecycle status for a plugin.
// @Summary Get plugin runtime status
// @Description Get operator-visible lifecycle, health, crash, and backoff state for a plugin backend runtime
// @Tags Plugins
// @Produce json
// @Security BearerAuth
// @Param id path string true "Plugin ID"
// @Success 200 {object} plugin.PluginRuntimeStatus
// @Failure 400 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Router /admin/plugins/{id}/runtime [get]
func (h *Handlers) GetPluginRuntimeStatus(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid plugin ID")
		return
	}

	status, err := h.pluginService.GetPluginRuntimeStatus(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusNotFound, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, status)
}

// RestartPluginRuntime manually restarts a supervised package runtime.
// @Summary Restart plugin runtime
// @Description Restart a supervised package plugin runtime and return its updated lifecycle status
// @Tags Plugins
// @Produce json
// @Security BearerAuth
// @Param id path string true "Plugin ID"
// @Success 200 {object} plugin.PluginRuntimeStatus
// @Failure 400 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Failure 502 {object} object{error=string}
// @Router /admin/plugins/{id}/runtime/restart [post]
func (h *Handlers) RestartPluginRuntime(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid plugin ID")
		return
	}

	status, err := h.pluginService.RestartPluginRuntime(r.Context(), id)
	if err != nil {
		switch {
		case errors.Is(err, plugin.ErrPluginRuntimeUnavailable):
			respondError(w, http.StatusBadGateway, err.Error())
		case errors.Is(err, plugin.ErrPluginRuntimeUnsupported), errors.Is(err, plugin.ErrPluginNotEnabled):
			respondError(w, http.StatusBadRequest, err.Error())
		default:
			respondError(w, http.StatusNotFound, err.Error())
		}
		return
	}

	respondJSON(w, http.StatusOK, status)
}

// GetAllPermissions returns all available plugin permissions
// @Summary List available permissions
// @Description Get all available plugin permissions with descriptions
// @Tags Plugins
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string]plugin.Permission
// @Router /admin/plugins/permissions [get]
func (h *Handlers) GetAllPermissions(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, plugin.AllPermissions)
}

// Tenant Plugin Handlers

// ListTenantPlugins lists all plugins available to a tenant
// @Summary List tenant plugins
// @Description Get all plugins available for a tenant with their enablement status
// @Tags Plugins
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Success 200 {array} plugin.TenantPlugin
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/plugins [get]
func (h *Handlers) ListTenantPlugins(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.GetClaims(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	tenantID, err := uuid.Parse(chi.URLParam(r, "tenantID"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid tenant ID")
		return
	}

	// Verify user has access to tenant
	_, err = h.tenantService.GetUserRole(r.Context(), tenantID.String(), claims.UserID)
	if err != nil {
		respondError(w, http.StatusForbidden, "Access denied")
		return
	}

	plugins, err := h.pluginService.GetTenantPlugins(r.Context(), tenantID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to list plugins")
		return
	}

	respondJSON(w, http.StatusOK, plugins)
}

// EnableTenantPlugin enables a plugin for a tenant
// @Summary Enable plugin for tenant
// @Description Enable an installed plugin for a specific tenant
// @Tags Plugins
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param pluginID path string true "Plugin ID"
// @Param request body plugin.TenantPluginSettingsRequest false "Initial settings"
// @Success 200 {object} plugin.TenantPlugin
// @Failure 400 {object} object{error=string}
// @Router /tenants/{tenantID}/plugins/{pluginID}/enable [post]
func (h *Handlers) EnableTenantPlugin(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.GetClaims(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	tenantID, err := uuid.Parse(chi.URLParam(r, "tenantID"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid tenant ID")
		return
	}

	pluginID, err := uuid.Parse(chi.URLParam(r, "pluginID"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid plugin ID")
		return
	}

	// Verify user has admin role in tenant
	role, err := h.tenantService.GetUserRole(r.Context(), tenantID.String(), claims.UserID)
	if err != nil || (role != "owner" && role != "admin") {
		respondError(w, http.StatusForbidden, "Admin access required")
		return
	}

	var req plugin.TenantPluginSettingsRequest
	_ = decodeJSON(r, &req) // Settings are optional

	settings := req.Settings
	if settings == nil {
		settings = json.RawMessage("{}")
	}

	if err := h.pluginService.EnableForTenant(r.Context(), tenantID, pluginID, settings); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	plugins, err := h.pluginService.GetTenantPlugins(r.Context(), tenantID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to load tenant plugin")
		return
	}
	for _, tenantPlugin := range plugins {
		if tenantPlugin.PluginID == pluginID {
			respondJSON(w, http.StatusOK, tenantPlugin)
			return
		}
	}

	respondError(w, http.StatusInternalServerError, "Failed to load tenant plugin")
}

// DisableTenantPlugin disables a plugin for a tenant
// @Summary Disable plugin for tenant
// @Description Disable a plugin for a specific tenant
// @Tags Plugins
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param pluginID path string true "Plugin ID"
// @Success 200 {object} object{status=string}
// @Failure 400 {object} object{error=string}
// @Router /tenants/{tenantID}/plugins/{pluginID}/disable [post]
func (h *Handlers) DisableTenantPlugin(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.GetClaims(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	tenantID, err := uuid.Parse(chi.URLParam(r, "tenantID"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid tenant ID")
		return
	}

	pluginID, err := uuid.Parse(chi.URLParam(r, "pluginID"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid plugin ID")
		return
	}

	// Verify user has admin role in tenant
	role, err := h.tenantService.GetUserRole(r.Context(), tenantID.String(), claims.UserID)
	if err != nil || (role != "owner" && role != "admin") {
		respondError(w, http.StatusForbidden, "Admin access required")
		return
	}

	if err := h.pluginService.DisableForTenant(r.Context(), tenantID, pluginID); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "disabled"})
}

// GetTenantPluginSettings returns plugin settings for a tenant
// @Summary Get tenant plugin settings
// @Description Get the settings for a plugin for a specific tenant
// @Tags Plugins
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param pluginID path string true "Plugin ID"
// @Success 200 {object} object
// @Failure 404 {object} object{error=string}
// @Router /tenants/{tenantID}/plugins/{pluginID}/settings [get]
func (h *Handlers) GetTenantPluginSettings(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.GetClaims(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	tenantID, err := uuid.Parse(chi.URLParam(r, "tenantID"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid tenant ID")
		return
	}

	pluginID, err := uuid.Parse(chi.URLParam(r, "pluginID"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid plugin ID")
		return
	}

	// Verify user has access to tenant
	_, err = h.tenantService.GetUserRole(r.Context(), tenantID.String(), claims.UserID)
	if err != nil {
		respondError(w, http.StatusForbidden, "Access denied")
		return
	}

	settings, err := h.pluginService.GetTenantPluginSettings(r.Context(), tenantID, pluginID)
	if err != nil {
		respondError(w, http.StatusNotFound, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(settings)
}

// UpdateTenantPluginSettings updates plugin settings for a tenant
// @Summary Update tenant plugin settings
// @Description Update the settings for a plugin for a specific tenant
// @Tags Plugins
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param pluginID path string true "Plugin ID"
// @Param request body object true "Settings object"
// @Success 200 {object} object{status=string}
// @Failure 400 {object} object{error=string}
// @Router /tenants/{tenantID}/plugins/{pluginID}/settings [put]
func (h *Handlers) UpdateTenantPluginSettings(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.GetClaims(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	tenantID, err := uuid.Parse(chi.URLParam(r, "tenantID"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid tenant ID")
		return
	}

	pluginID, err := uuid.Parse(chi.URLParam(r, "pluginID"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid plugin ID")
		return
	}

	// Verify user has admin role in tenant
	role, err := h.tenantService.GetUserRole(r.Context(), tenantID.String(), claims.UserID)
	if err != nil || (role != "owner" && role != "admin") {
		respondError(w, http.StatusForbidden, "Admin access required")
		return
	}

	var settings json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&settings); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if err := h.pluginService.UpdateTenantPluginSettings(r.Context(), tenantID, pluginID, settings); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

// InvokeTenantPluginRoute proxies a declared plugin runtime route for a tenant.
// @Summary Invoke tenant plugin route
// @Description Invoke a manifest-declared plugin runtime route through the out-of-process HTTP plugin runtime
// @Tags Plugins
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param pluginID path string true "Plugin ID"
// @Param path path string true "Plugin route path"
// @Success 200 {object} object
// @Failure 400 {object} object{error=string}
// @Failure 403 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Failure 502 {object} object{error=string}
// @Router /tenants/{tenantID}/plugins/{pluginID}/runtime/{path} [get]
// @Router /tenants/{tenantID}/plugins/{pluginID}/runtime/{path} [post]
// @Router /tenants/{tenantID}/plugins/{pluginID}/runtime/{path} [put]
// @Router /tenants/{tenantID}/plugins/{pluginID}/runtime/{path} [patch]
// @Router /tenants/{tenantID}/plugins/{pluginID}/runtime/{path} [delete]
func (h *Handlers) InvokeTenantPluginRoute(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.GetClaims(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	tenantID, err := uuid.Parse(chi.URLParam(r, "tenantID"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid tenant ID")
		return
	}

	pluginID, err := uuid.Parse(chi.URLParam(r, "pluginID"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid plugin ID")
		return
	}

	if _, err := h.tenantService.GetUserRole(r.Context(), tenantID.String(), claims.UserID); err != nil {
		respondError(w, http.StatusForbidden, "Access denied")
		return
	}

	runtimePath := "/" + chi.URLParam(r, "*")
	resp, err := h.pluginService.InvokeTenantPluginRoute(
		r.Context(),
		tenantID,
		pluginID,
		r.Method,
		runtimePath,
		r.URL.RawQuery,
		r.Header,
		r.Body,
	)
	if err != nil {
		switch {
		case errors.Is(err, plugin.ErrPluginNotEnabled):
			respondError(w, http.StatusForbidden, "Plugin is not enabled for this tenant")
		case errors.Is(err, plugin.ErrPluginRouteNotFound):
			respondError(w, http.StatusNotFound, "Plugin route is not registered")
		case errors.Is(err, plugin.ErrPluginRuntimeUnavailable), errors.Is(err, plugin.ErrPluginRuntimeUnsupported):
			respondError(w, http.StatusBadGateway, err.Error())
		default:
			respondError(w, http.StatusBadGateway, "Plugin runtime request failed")
		}
		return
	}

	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	w.WriteHeader(resp.StatusCode)
	_, _ = w.Write(resp.Body)
}
