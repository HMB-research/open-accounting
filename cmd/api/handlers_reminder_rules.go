package main

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"

	"github.com/HMB-research/open-accounting/internal/invoicing"
)

// ListReminderRules lists all reminder rules for a tenant
// @Summary List reminder rules
// @Tags Reminder Rules
// @Param tenantID path string true "Tenant ID"
// @Success 200 {array} invoicing.ReminderRule
// @Router /api/v1/tenants/{tenantID}/reminder-rules [get]
func (h *Handlers) ListReminderRules(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID := chi.URLParam(r, "tenantID")
	schemaName := h.getSchemaName(ctx, tenantID)

	rules, err := h.automatedReminderService.ListRules(ctx, tenantID, schemaName)
	if err != nil {
		log.Error().Err(err).Msg("Failed to list reminder rules")
		respondError(w, http.StatusInternalServerError, "Failed to list rules")
		return
	}

	respondJSON(w, http.StatusOK, rules)
}

// GetReminderRule gets a single reminder rule
// @Summary Get reminder rule
// @Tags Reminder Rules
// @Param tenantID path string true "Tenant ID"
// @Param ruleID path string true "Rule ID"
// @Success 200 {object} invoicing.ReminderRule
// @Router /api/v1/tenants/{tenantID}/reminder-rules/{ruleID} [get]
func (h *Handlers) GetReminderRule(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID := chi.URLParam(r, "tenantID")
	ruleID := chi.URLParam(r, "ruleID")
	schemaName := h.getSchemaName(ctx, tenantID)

	rule, err := h.automatedReminderService.GetRule(ctx, tenantID, schemaName, ruleID)
	if err != nil {
		if _, ok := err.(*invoicing.NotFoundError); ok {
			respondError(w, http.StatusNotFound, "Rule not found")
			return
		}
		log.Error().Err(err).Msg("Failed to get reminder rule")
		respondError(w, http.StatusInternalServerError, "Failed to get rule")
		return
	}

	respondJSON(w, http.StatusOK, rule)
}

// CreateReminderRule creates a new reminder rule
// @Summary Create reminder rule
// @Tags Reminder Rules
// @Param tenantID path string true "Tenant ID"
// @Param body body invoicing.CreateReminderRuleRequest true "Rule data"
// @Success 201 {object} invoicing.ReminderRule
// @Router /api/v1/tenants/{tenantID}/reminder-rules [post]
func (h *Handlers) CreateReminderRule(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID := chi.URLParam(r, "tenantID")
	schemaName := h.getSchemaName(ctx, tenantID)

	var req invoicing.CreateReminderRuleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	rule, err := h.automatedReminderService.CreateRule(ctx, tenantID, schemaName, &req)
	if err != nil {
		if _, ok := err.(*invoicing.ValidationError); ok {
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}
		log.Error().Err(err).Msg("Failed to create reminder rule")
		respondError(w, http.StatusInternalServerError, "Failed to create rule")
		return
	}

	respondJSON(w, http.StatusCreated, rule)
}

// UpdateReminderRule updates a reminder rule
// @Summary Update reminder rule
// @Tags Reminder Rules
// @Param tenantID path string true "Tenant ID"
// @Param ruleID path string true "Rule ID"
// @Param body body invoicing.UpdateReminderRuleRequest true "Rule data"
// @Success 200 {object} invoicing.ReminderRule
// @Router /api/v1/tenants/{tenantID}/reminder-rules/{ruleID} [put]
func (h *Handlers) UpdateReminderRule(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID := chi.URLParam(r, "tenantID")
	ruleID := chi.URLParam(r, "ruleID")
	schemaName := h.getSchemaName(ctx, tenantID)

	var req invoicing.UpdateReminderRuleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	rule, err := h.automatedReminderService.UpdateRule(ctx, tenantID, schemaName, ruleID, &req)
	if err != nil {
		if _, ok := err.(*invoicing.NotFoundError); ok {
			respondError(w, http.StatusNotFound, "Rule not found")
			return
		}
		log.Error().Err(err).Msg("Failed to update reminder rule")
		respondError(w, http.StatusInternalServerError, "Failed to update rule")
		return
	}

	respondJSON(w, http.StatusOK, rule)
}

// DeleteReminderRule deletes a reminder rule
// @Summary Delete reminder rule
// @Tags Reminder Rules
// @Param tenantID path string true "Tenant ID"
// @Param ruleID path string true "Rule ID"
// @Success 204 "No Content"
// @Router /api/v1/tenants/{tenantID}/reminder-rules/{ruleID} [delete]
func (h *Handlers) DeleteReminderRule(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID := chi.URLParam(r, "tenantID")
	ruleID := chi.URLParam(r, "ruleID")
	schemaName := h.getSchemaName(ctx, tenantID)

	err := h.automatedReminderService.DeleteRule(ctx, tenantID, schemaName, ruleID)
	if err != nil {
		if _, ok := err.(*invoicing.NotFoundError); ok {
			respondError(w, http.StatusNotFound, "Rule not found")
			return
		}
		log.Error().Err(err).Msg("Failed to delete reminder rule")
		respondError(w, http.StatusInternalServerError, "Failed to delete rule")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// TriggerReminders manually triggers reminder processing for a tenant
// @Summary Trigger reminders
// @Tags Reminder Rules
// @Param tenantID path string true "Tenant ID"
// @Success 200 {array} invoicing.AutomatedReminderResult
// @Router /api/v1/tenants/{tenantID}/reminder-rules/trigger [post]
func (h *Handlers) TriggerReminders(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID := chi.URLParam(r, "tenantID")
	schemaName := h.getSchemaName(ctx, tenantID)

	tenant, err := h.tenantService.GetTenant(ctx, tenantID)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Tenant not found")
		return
	}

	results, err := h.automatedReminderService.ProcessRemindersForTenant(ctx, tenantID, schemaName, tenant.Name)
	if err != nil {
		log.Error().Err(err).Msg("Failed to trigger reminders")
		respondError(w, http.StatusInternalServerError, "Failed to process reminders")
		return
	}

	respondJSON(w, http.StatusOK, results)
}
