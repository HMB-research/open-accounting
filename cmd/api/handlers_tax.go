package main

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/HMB-research/open-accounting/internal/tax"
)

// HandleGenerateKMD generates a KMD declaration for a period
// @Summary Generate KMD declaration
// @Description Generate an Estonian VAT declaration (KMD) for a specific period
// @Tags Tax
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param request body tax.CreateKMDRequest true "Period to generate"
// @Success 200 {object} tax.KMDDeclaration
// @Failure 400 {object} object{error=string}
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/tax/kmd [post]
func (h *Handlers) HandleGenerateKMD(w http.ResponseWriter, r *http.Request) {
	tenantCtx := h.tenantContextFromRequest(r)

	var req tax.CreateKMDRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	decl, err := h.taxService.GenerateKMD(r.Context(), tenantCtx.tenantID, tenantCtx.schemaName, &req)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, decl)
}

// HandleListKMD lists all KMD declarations for a tenant
// @Summary List KMD declarations
// @Description Get all KMD declarations for a tenant
// @Tags Tax
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Success 200 {array} tax.KMDDeclaration
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/tax/kmd [get]
func (h *Handlers) HandleListKMD(w http.ResponseWriter, r *http.Request) {
	tenantCtx := h.tenantContextFromRequest(r)

	declarations, err := h.taxService.ListKMD(r.Context(), tenantCtx.tenantID, tenantCtx.schemaName)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, declarations)
}

// HandleExportKMD exports a KMD declaration to XML
// @Summary Export KMD to XML
// @Description Export a KMD declaration to Estonian e-MTA XML format
// @Tags Tax
// @Produce application/xml
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param year path string true "Year"
// @Param month path string true "Month"
// @Success 200 {file} file "XML file"
// @Failure 404 {object} object{error=string}
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/tax/kmd/{year}/{month}/xml [get]
func (h *Handlers) HandleExportKMD(w http.ResponseWriter, r *http.Request) {
	tenantCtx := h.tenantContextFromRequest(r)
	year := chi.URLParam(r, "year")
	month := chi.URLParam(r, "month")

	currentTenant, err := h.tenantService.GetTenant(r.Context(), tenantCtx.tenantID)
	if err != nil {
		respondError(w, http.StatusNotFound, "Tenant not found")
		return
	}

	decl, err := h.taxService.GetKMD(r.Context(), tenantCtx.tenantID, tenantCtx.schemaName, year, month)
	if err != nil {
		respondError(w, http.StatusNotFound, "Declaration not found")
		return
	}

	xmlBytes, err := tax.ExportKMDToXML(decl, currentTenant.Settings.RegCode)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/xml")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=KMD_%s_%s.xml", year, month))
	_, _ = w.Write(xmlBytes)
}
