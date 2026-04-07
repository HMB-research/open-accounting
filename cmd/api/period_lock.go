package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const periodLockLayout = "2006-01-02"

type periodLockedError struct {
	lockDate      string
	operationDate string
}

func (e *periodLockedError) Error() string {
	return fmt.Sprintf("period locked through %s; transaction date %s must be later", e.lockDate, e.operationDate)
}

func (h *Handlers) ensurePeriodUnlocked(ctx context.Context, tenantID string, operationDate time.Time) error {
	currentTenant, err := h.tenantService.GetTenant(ctx, tenantID)
	if err != nil {
		return fmt.Errorf("load tenant settings: %w", err)
	}

	if currentTenant.Settings.PeriodLockDate == nil {
		return nil
	}

	lockDateRaw := strings.TrimSpace(*currentTenant.Settings.PeriodLockDate)
	if lockDateRaw == "" {
		return nil
	}

	lockDate, err := time.Parse(periodLockLayout, lockDateRaw)
	if err != nil {
		return fmt.Errorf("invalid tenant period lock date %q: %w", lockDateRaw, err)
	}

	normalizedOperationDate := normalizePeriodDate(operationDate)
	if !normalizedOperationDate.After(lockDate) {
		return &periodLockedError{
			lockDate:      lockDate.Format(periodLockLayout),
			operationDate: normalizedOperationDate.Format(periodLockLayout),
		}
	}

	return nil
}

func (h *Handlers) rejectLockedPeriod(w http.ResponseWriter, ctx context.Context, tenantID string, operationDate time.Time) bool {
	err := h.ensurePeriodUnlocked(ctx, tenantID, operationDate)
	if err == nil {
		return false
	}

	var lockedErr *periodLockedError
	if errors.As(err, &lockedErr) {
		respondError(w, http.StatusConflict, err.Error())
		return true
	}

	respondError(w, http.StatusInternalServerError, "Failed to validate period lock")
	return true
}

func normalizePeriodDate(value time.Time) time.Time {
	utcValue := value.UTC()
	return time.Date(utcValue.Year(), utcValue.Month(), utcValue.Day(), 0, 0, 0, 0, time.UTC)
}
