package main

import (
	"net/http"

	"github.com/HMB-research/open-accounting/internal/cutover"
)

// ValidateMigrationBundle validates a non-mutating CSV/XML migration bundle.
// @Summary Validate migration bundle
// @Description Validate CSV and XML cutover files for required columns and cross-file references before running imports
// @Tags Migration
// @Accept json
// @Produce json
// @Param tenantID path string true "Tenant ID"
// @Param request body cutover.ValidateBundleRequest true "Migration bundle files"
// @Success 200 {object} cutover.BundleValidationReport
// @Failure 400 {object} object{error=string}
// @Failure 401 {object} object{error=string}
// @Failure 403 {object} object{error=string}
// @Router /tenants/{tenantID}/migration/validate [post]
func (h *Handlers) ValidateMigrationBundle(w http.ResponseWriter, r *http.Request) {
	var req cutover.ValidateBundleRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	report, err := cutover.ValidateBundle(&req)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, report)
}

// PlanMigrationExecution validates a bundle and returns ordered cutover import steps.
// @Summary Plan migration execution
// @Description Validate CSV and XML cutover files and return a dependency-aware import execution plan with API and CLI commands
// @Tags Migration
// @Accept json
// @Produce json
// @Param tenantID path string true "Tenant ID"
// @Param request body cutover.PlanMigrationExecutionRequest true "Migration bundle files and execution context"
// @Success 200 {object} cutover.MigrationExecutionPlan
// @Failure 400 {object} object{error=string}
// @Failure 401 {object} object{error=string}
// @Failure 403 {object} object{error=string}
// @Router /tenants/{tenantID}/migration/execution-plan [post]
func (h *Handlers) PlanMigrationExecution(w http.ResponseWriter, r *http.Request) {
	var req cutover.PlanMigrationExecutionRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	plan, err := cutover.BuildMigrationExecutionPlan(&req)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, plan)
}
