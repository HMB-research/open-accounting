package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/HMB-research/open-accounting/internal/importdelivery"
)

// The bridge delivers bounded raw bytes (NDJSON records or artifact media),
// never base64 JSON. This ceiling is intentionally separate from the 2 MiB
// browser-adjacent import-session API.
const maxInternalBridgeDeliveryRequestBytes = 1500 << 10

// PutBridgePackageManifest accepts manifest metadata before archive content.
// It is an internal HMAC-authenticated endpoint, not a browser API.
func (h *Handlers) PutBridgePackageManifest(w http.ResponseWriter, r *http.Request) {
	body, tenantID, ok := h.authenticateBridgeDelivery(w, r)
	if !ok {
		return
	}
	var manifest importdelivery.Manifest
	if !decodeInternalBridgeJSON(w, body, &manifest) {
		return
	}
	packageID := strings.TrimSpace(chi.URLParam(r, "packageID"))
	if manifest.PackageID != packageID {
		respondError(w, http.StatusBadRequest, "Package ID does not match manifest")
		return
	}
	status, err := h.importDeliveryService.AcceptManifest(r.Context(), h.getSchemaName(r.Context(), tenantID), tenantID, manifest)
	if errors.Is(err, importdelivery.ErrSourceBindingConflict) {
		respondError(w, http.StatusConflict, "Source identity is already bound to another tenant")
		return
	}
	if errors.Is(err, importdelivery.ErrSourceNotConfiguredForTenant) {
		respondError(w, http.StatusConflict, "Bridge source is not configured for this tenant")
		return
	}
	if errors.Is(err, importdelivery.ErrManifestInvalid) || errors.Is(err, importdelivery.ErrTenantIsolation) {
		respondError(w, http.StatusUnprocessableEntity, "Invalid bridge package manifest")
		return
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Could not stage bridge package manifest")
		return
	}
	code := http.StatusOK
	if status.Created {
		code = http.StatusCreated
	}
	respondJSON(w, code, status)
}

func (h *Handlers) PutBridgePackageRecords(w http.ResponseWriter, r *http.Request) {
	body, tenantID, ok := h.authenticateBridgeDelivery(w, r)
	if !ok {
		return
	}
	sequence, err := strconv.Atoi(chi.URLParam(r, "sequence"))
	recordCount, countErr := strconv.Atoi(r.Header.Get("X-OA-Bridge-Record-Count"))
	digest := strings.TrimSpace(r.Header.Get("X-OA-Bridge-Chunk-SHA256"))
	if err != nil || countErr != nil || sequence < 0 || recordCount < 1 || r.Header.Get("Content-Type") != "application/x-ndjson" || digest != r.Header.Get("X-OA-Bridge-Content-SHA256") {
		respondError(w, http.StatusBadRequest, "Record chunk sequence does not match path")
		return
	}
	result, err := h.importDeliveryService.AcceptRawRecordChunk(r.Context(), h.getSchemaName(r.Context(), tenantID), tenantID, chi.URLParam(r, "packageID"), sequence, recordCount, digest, body)
	h.handleBridgeChunkResult(w, result, err)
}

func (h *Handlers) PutBridgePackageArtifactChunk(w http.ResponseWriter, r *http.Request) {
	body, tenantID, ok := h.authenticateBridgeDelivery(w, r)
	if !ok {
		return
	}
	sequence, err := strconv.Atoi(chi.URLParam(r, "sequence"))
	chunkCount, countErr := strconv.Atoi(r.Header.Get("X-OA-Bridge-Chunk-Count"))
	digest := strings.TrimSpace(r.Header.Get("X-OA-Bridge-Chunk-SHA256"))
	if err != nil || countErr != nil || sequence < 0 || chunkCount < 1 || sequence >= chunkCount || strings.TrimSpace(r.Header.Get("Content-Type")) == "" || digest != r.Header.Get("X-OA-Bridge-Content-SHA256") {
		respondError(w, http.StatusBadRequest, "Artifact chunk sequence does not match path")
		return
	}
	result, err := h.importDeliveryService.AcceptRawArtifactChunk(r.Context(), h.getSchemaName(r.Context(), tenantID), tenantID, chi.URLParam(r, "packageID"), chi.URLParam(r, "artifactID"), sequence, chunkCount, digest, body)
	h.handleBridgeChunkResult(w, result, err)
}

func (h *Handlers) GetBridgePackageDelivery(w http.ResponseWriter, r *http.Request) {
	_, tenantID, ok := h.authenticateBridgeDelivery(w, r)
	if !ok {
		return
	}
	status, err := h.importDeliveryService.Status(r.Context(), h.getSchemaName(r.Context(), tenantID), tenantID, chi.URLParam(r, "packageID"))
	if errors.Is(err, importdelivery.ErrNotFound) {
		respondError(w, http.StatusNotFound, "Bridge package delivery not found")
		return
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Could not load bridge package delivery")
		return
	}
	respondJSON(w, http.StatusOK, status)
}

func (h *Handlers) FinalizeBridgePackageDelivery(w http.ResponseWriter, r *http.Request) {
	body, tenantID, ok := h.authenticateBridgeDelivery(w, r)
	if !ok {
		return
	}
	var finalize importdelivery.FinalizeRequest
	if !decodeInternalBridgeJSON(w, body, &finalize) {
		return
	}
	status, err := h.importDeliveryService.Finalize(r.Context(), h.getSchemaName(r.Context(), tenantID), tenantID, chi.URLParam(r, "packageID"), finalize)
	if errors.Is(err, importdelivery.ErrNotFound) {
		respondError(w, http.StatusNotFound, "Bridge package delivery not found")
		return
	}
	if errors.Is(err, importdelivery.ErrFinalizeIncomplete) {
		respondError(w, http.StatusConflict, "Bridge package delivery is incomplete")
		return
	}
	if errors.Is(err, importdelivery.ErrAlreadyFinalized) {
		respondError(w, http.StatusConflict, "Bridge package delivery conflicts with an existing staged package")
		return
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Could not finalize bridge package delivery")
		return
	}
	respondJSON(w, http.StatusAccepted, status)
}

func (h *Handlers) handleBridgeChunkResult(w http.ResponseWriter, result importdelivery.ChunkResult, err error) {
	if errors.Is(err, importdelivery.ErrNotFound) {
		respondError(w, http.StatusNotFound, "Bridge package delivery not found")
		return
	}
	if errors.Is(err, importdelivery.ErrChunkOutOfOrder) {
		respondError(w, http.StatusConflict, "Bridge package chunk is out of order")
		return
	}
	if errors.Is(err, importdelivery.ErrChunkConflict) || errors.Is(err, importdelivery.ErrAlreadyFinalized) {
		respondError(w, http.StatusConflict, "Bridge package chunk conflicts with existing staged content")
		return
	}
	if errors.Is(err, importdelivery.ErrChunkInvalid) || errors.Is(err, importdelivery.ErrTenantIsolation) {
		respondError(w, http.StatusUnprocessableEntity, "Invalid bridge package chunk")
		return
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Could not stage bridge package chunk")
		return
	}
	code := http.StatusOK
	if result.Created {
		code = http.StatusCreated
	}
	respondJSON(w, code, result)
}

func (h *Handlers) authenticateBridgeDelivery(w http.ResponseWriter, r *http.Request) ([]byte, string, bool) {
	if h.importDeliveryService == nil || h.importDeliveryAuthenticator == nil {
		respondError(w, http.StatusServiceUnavailable, "Bridge package delivery is not configured")
		return nil, "", false
	}
	if r.URL.RawQuery != "" {
		respondError(w, http.StatusBadRequest, "Bridge package delivery query parameters are not supported")
		return nil, "", false
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxInternalBridgeDeliveryRequestBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		respondError(w, http.StatusRequestEntityTooLarge, "Bridge package request is too large")
		return nil, "", false
	}
	tenantID := strings.TrimSpace(chi.URLParam(r, "tenantID"))
	if tenantID == "" || tenantID != strings.TrimSpace(r.Header.Get("X-OA-Bridge-Tenant")) {
		respondError(w, http.StatusUnauthorized, "Bridge package authentication failed")
		return nil, "", false
	}
	err = h.importDeliveryAuthenticator.Authenticate(r.Context(), importdelivery.SignedRequest{Method: r.Method, Path: r.URL.Path, TenantID: tenantID, Timestamp: r.Header.Get("X-OA-Bridge-Timestamp"), Nonce: r.Header.Get("X-OA-Bridge-Nonce"), ContentSHA256: r.Header.Get("X-OA-Bridge-Content-SHA256"), Signature: r.Header.Get("X-OA-Bridge-Signature"), Body: body})
	if err != nil {
		respondError(w, http.StatusUnauthorized, "Bridge package authentication failed")
		return nil, "", false
	}
	return body, tenantID, true
}

func decodeInternalBridgeJSON(w http.ResponseWriter, body []byte, target interface{}) bool {
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil || decoder.Decode(&struct{}{}) != io.EOF {
		respondError(w, http.StatusBadRequest, "Invalid bridge package request")
		return false
	}
	return true
}
