package main

import (
	"context"
	"net/http"
	"time"

	"github.com/HMB-research/open-accounting/internal/smartaccountssync"
)

type readinessCheck struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

type readinessResponse struct {
	Status string           `json:"status"`
	Checks []readinessCheck `json:"checks"`
}

type readinessPinger interface {
	Ping(context.Context) error
}

// ReadinessCheck verifies the API-to-private-bridge control plane. It is
// deliberately data-free and unauthenticated so container orchestrators and
// migration operators can distinguish a live API process from a usable sync
// deployment without receiving tenant, source, or credential metadata. The
// bridge check also fails closed when its fixed capability protocol differs
// from the Open Accounting receiver's supported contract.
// @Summary Readiness check
// @Description Return ready only when the API can reach its configured private SmartAccounts bridge. No tenant or source data is read.
// @Tags System
// @Produce json
// @Success 200 {object} readinessResponse
// @Failure 503 {object} readinessResponse
// @Router /ready [get]
func (h *Handlers) ReadinessCheck(w http.ResponseWriter, r *http.Request) {
	response := readinessResponse{Status: "not_ready", Checks: []readinessCheck{
		{Name: "api", Status: "ready"},
		{Name: "database", Status: "not_ready"},
		{Name: "smartaccounts_bridge", Status: "not_ready"},
	}}
	checker, ok := h.smartAccountsBridgeClient.(smartaccountssync.BridgeHealthChecker)
	if !ok || checker == nil || h.readinessDatabase == nil {
		w.Header().Set("Cache-Control", "no-store")
		respondJSON(w, http.StatusServiceUnavailable, response)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	databaseResult := make(chan error, 1)
	bridgeResult := make(chan error, 1)
	go func() { databaseResult <- h.readinessDatabase.Ping(ctx) }()
	go func() { bridgeResult <- checker.Health(ctx) }()
	var databaseErr, bridgeErr error
	select {
	case databaseErr = <-databaseResult:
	case <-ctx.Done():
		databaseErr = ctx.Err()
	}
	select {
	case bridgeErr = <-bridgeResult:
	case <-ctx.Done():
		bridgeErr = ctx.Err()
	}
	if databaseErr != nil || bridgeErr != nil {
		w.Header().Set("Cache-Control", "no-store")
		respondJSON(w, http.StatusServiceUnavailable, response)
		return
	}
	response.Status = "ready"
	response.Checks[1].Status = "ready"
	response.Checks[2].Status = "ready"
	w.Header().Set("Cache-Control", "no-store")
	respondJSON(w, http.StatusOK, response)
}
