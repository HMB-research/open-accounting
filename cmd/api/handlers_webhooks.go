package main

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"

	"github.com/HMB-research/open-accounting/internal/webhooks"
)

// ListWebhookEventTypes returns supported webhook event names.
// @Summary List webhook events
// @Description List event types that outbound webhook endpoints can subscribe to
// @Tags Webhooks
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Success 200 {array} string
// @Router /tenants/{tenantID}/webhooks/events [get]
func (h *Handlers) ListWebhookEventTypes(w http.ResponseWriter, _ *http.Request) {
	respondJSON(w, http.StatusOK, webhooks.ListEventTypes())
}

// ListWebhookEndpoints returns tenant outbound webhook endpoints.
// @Summary List webhook endpoints
// @Description List outbound webhook endpoints configured for a tenant
// @Tags Webhooks
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param active_only query bool false "Filter to active endpoints"
// @Success 200 {array} webhooks.Endpoint
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/webhooks [get]
func (h *Handlers) ListWebhookEndpoints(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	endpoints, err := h.webhookService.ListEndpoints(r.Context(), tenantID, r.URL.Query().Get("active_only") == "true")
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to list webhook endpoints")
		return
	}
	respondJSON(w, http.StatusOK, endpoints)
}

// CreateWebhookEndpoint creates a tenant outbound webhook endpoint.
// @Summary Create webhook endpoint
// @Description Create an outbound webhook endpoint with event subscriptions and optional HMAC secret
// @Tags Webhooks
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param request body webhooks.CreateEndpointRequest true "Webhook endpoint details"
// @Success 201 {object} webhooks.Endpoint
// @Failure 400 {object} object{error=string}
// @Router /tenants/{tenantID}/webhooks [post]
func (h *Handlers) CreateWebhookEndpoint(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	var req webhooks.CreateEndpointRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	endpoint, err := h.webhookService.CreateEndpoint(r.Context(), tenantID, &req)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	respondJSON(w, http.StatusCreated, endpoint)
}

// GetWebhookEndpoint returns one tenant outbound webhook endpoint.
// @Summary Get webhook endpoint
// @Description Get an outbound webhook endpoint by ID
// @Tags Webhooks
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param webhookID path string true "Webhook endpoint ID"
// @Success 200 {object} webhooks.Endpoint
// @Failure 404 {object} object{error=string}
// @Router /tenants/{tenantID}/webhooks/{webhookID} [get]
func (h *Handlers) GetWebhookEndpoint(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	webhookID := chi.URLParam(r, "webhookID")
	endpoint, err := h.webhookService.GetEndpoint(r.Context(), tenantID, webhookID)
	if err != nil {
		respondError(w, http.StatusNotFound, "Webhook endpoint not found")
		return
	}
	respondJSON(w, http.StatusOK, endpoint)
}

// UpdateWebhookEndpoint updates one tenant outbound webhook endpoint.
// @Summary Update webhook endpoint
// @Description Update outbound webhook endpoint settings
// @Tags Webhooks
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param webhookID path string true "Webhook endpoint ID"
// @Param request body webhooks.UpdateEndpointRequest true "Webhook endpoint updates"
// @Success 200 {object} webhooks.Endpoint
// @Failure 400 {object} object{error=string}
// @Router /tenants/{tenantID}/webhooks/{webhookID} [put]
func (h *Handlers) UpdateWebhookEndpoint(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	webhookID := chi.URLParam(r, "webhookID")
	var req webhooks.UpdateEndpointRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	endpoint, err := h.webhookService.UpdateEndpoint(r.Context(), tenantID, webhookID, &req)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, endpoint)
}

// DeleteWebhookEndpoint deletes one tenant outbound webhook endpoint.
// @Summary Delete webhook endpoint
// @Description Delete an outbound webhook endpoint and its delivery history
// @Tags Webhooks
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param webhookID path string true "Webhook endpoint ID"
// @Success 204 "No Content"
// @Failure 400 {object} object{error=string}
// @Router /tenants/{tenantID}/webhooks/{webhookID} [delete]
func (h *Handlers) DeleteWebhookEndpoint(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	webhookID := chi.URLParam(r, "webhookID")
	if err := h.webhookService.DeleteEndpoint(r.Context(), tenantID, webhookID); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ListWebhookDeliveries returns delivery attempts for one endpoint.
// @Summary List webhook deliveries
// @Description List recent outbound webhook delivery attempts for one endpoint
// @Tags Webhooks
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param webhookID path string true "Webhook endpoint ID"
// @Param limit query int false "Maximum deliveries to return" default(50)
// @Success 200 {array} webhooks.Delivery
// @Failure 400 {object} object{error=string}
// @Router /tenants/{tenantID}/webhooks/{webhookID}/deliveries [get]
func (h *Handlers) ListWebhookDeliveries(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	webhookID := chi.URLParam(r, "webhookID")
	limit := 50
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 || parsed > 200 {
			respondError(w, http.StatusBadRequest, "Limit must be between 1 and 200")
			return
		}
		limit = parsed
	}
	deliveries, err := h.webhookService.ListDeliveries(r.Context(), tenantID, webhookID, limit)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, deliveries)
}

// TestWebhookEndpoint sends a test event to one endpoint.
// @Summary Test webhook endpoint
// @Description Send a deterministic test event to one endpoint and record the delivery result
// @Tags Webhooks
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param webhookID path string true "Webhook endpoint ID"
// @Param request body webhooks.TestDeliveryRequest false "Optional test payload"
// @Success 200 {object} webhooks.DeliveryResult
// @Failure 400 {object} object{error=string}
// @Router /tenants/{tenantID}/webhooks/{webhookID}/test [post]
func (h *Handlers) TestWebhookEndpoint(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	webhookID := chi.URLParam(r, "webhookID")
	var req webhooks.TestDeliveryRequest
	if r.Body != nil && r.ContentLength != 0 {
		if err := decodeJSON(r, &req); err != nil {
			respondError(w, http.StatusBadRequest, "Invalid request body")
			return
		}
	}
	result, err := h.webhookService.DispatchTest(r.Context(), tenantID, webhookID, &req)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, result)
}

func (h *Handlers) emitWebhookEvent(eventType, tenantID string, data any) {
	if h == nil || h.webhookService == nil {
		return
	}
	payload, err := json.Marshal(data)
	if err != nil {
		log.Warn().Err(err).Str("event", eventType).Msg("Failed to marshal webhook event payload")
		return
	}
	h.webhookService.DispatchAsync(webhooks.Event{
		Type:     eventType,
		TenantID: tenantID,
		Data:     payload,
	})
}
