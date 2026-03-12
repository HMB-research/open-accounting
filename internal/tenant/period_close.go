package tenant

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

const periodCloseDateLayout = "2006-01-02"

// ClosePeriod closes the tenant through the provided month-end date.
func (s *Service) ClosePeriod(ctx context.Context, tenantID, performedBy string, req *ClosePeriodRequest) (*Tenant, *PeriodCloseEvent, error) {
	current, closeDate, lockDateBefore, err := s.loadPeriodCloseState(ctx, tenantID, req.PeriodEndDate)
	if err != nil {
		return nil, nil, err
	}

	if lockDateBefore != nil && !closeDate.After(*lockDateBefore) {
		return nil, nil, fmt.Errorf("period already closed through %s", lockDateBefore.Format(periodCloseDateLayout))
	}

	lockDateAfterValue := closeDate.Format(periodCloseDateLayout)
	nextSettings := current.Settings
	nextSettings.PeriodLockDate = &lockDateAfterValue

	event := &PeriodCloseEvent{
		ID:             uuid.New().String(),
		TenantID:       tenantID,
		Action:         PeriodCloseActionClose,
		CloseKind:      closeKindForDate(closeDate, nextSettings.FiscalYearStart),
		PeriodEndDate:  closeDate.Format(periodCloseDateLayout),
		LockDateBefore: formatOptionalDate(lockDateBefore),
		LockDateAfter:  &lockDateAfterValue,
		Note:           strings.TrimSpace(req.Note),
		PerformedBy:    performedBy,
		CreatedAt:      time.Now().UTC(),
	}

	updatedTenant, err := s.persistPeriodCloseState(ctx, current, nextSettings, event)
	if err != nil {
		return nil, nil, err
	}

	return updatedTenant, event, nil
}

// ReopenPeriod reopens the provided month-end period and all later periods.
func (s *Service) ReopenPeriod(ctx context.Context, tenantID, performedBy string, req *ReopenPeriodRequest) (*Tenant, *PeriodCloseEvent, error) {
	current, reopenDate, lockDateBefore, err := s.loadPeriodCloseState(ctx, tenantID, req.PeriodEndDate)
	if err != nil {
		return nil, nil, err
	}

	if lockDateBefore == nil {
		return nil, nil, fmt.Errorf("no closed period to reopen")
	}
	if reopenDate.After(*lockDateBefore) {
		return nil, nil, fmt.Errorf("period %s is not currently closed", reopenDate.Format(periodCloseDateLayout))
	}

	note := strings.TrimSpace(req.Note)
	if note == "" {
		return nil, nil, fmt.Errorf("note is required when reopening a period")
	}

	nextSettings := current.Settings
	latestCloseEvent, err := s.repo.GetLatestCloseEventForPeriod(ctx, tenantID, reopenDate.Format(periodCloseDateLayout))
	if err != nil {
		return nil, nil, err
	}
	if latestCloseEvent == nil {
		return nil, nil, fmt.Errorf("period %s has not been closed yet", reopenDate.Format(periodCloseDateLayout))
	}

	nextSettings.PeriodLockDate = latestCloseEvent.LockDateBefore
	lockDateAfter := formatOptionalDateFromString(latestCloseEvent.LockDateBefore)

	event := &PeriodCloseEvent{
		ID:             uuid.New().String(),
		TenantID:       tenantID,
		Action:         PeriodCloseActionReopen,
		CloseKind:      closeKindForDate(reopenDate, nextSettings.FiscalYearStart),
		PeriodEndDate:  reopenDate.Format(periodCloseDateLayout),
		LockDateBefore: formatOptionalDate(lockDateBefore),
		LockDateAfter:  lockDateAfter,
		Note:           note,
		PerformedBy:    performedBy,
		CreatedAt:      time.Now().UTC(),
	}

	updatedTenant, err := s.persistPeriodCloseState(ctx, current, nextSettings, event)
	if err != nil {
		return nil, nil, err
	}

	return updatedTenant, event, nil
}

// ListPeriodCloseEvents returns the most recent close or reopen events for a tenant.
func (s *Service) ListPeriodCloseEvents(ctx context.Context, tenantID string, limit int) ([]PeriodCloseEvent, error) {
	return s.repo.ListPeriodCloseEvents(ctx, tenantID, limit)
}

func (s *Service) persistPeriodCloseState(ctx context.Context, current *Tenant, nextSettings TenantSettings, event *PeriodCloseEvent) (*Tenant, error) {
	updatedAt := event.CreatedAt
	settingsJSON, err := json.Marshal(nextSettings)
	if err != nil {
		return nil, fmt.Errorf("marshal tenant settings: %w", err)
	}

	if err := s.repo.UpdateTenantWithPeriodCloseEvent(ctx, current.ID, current.Name, settingsJSON, updatedAt, event); err != nil {
		return nil, err
	}

	current.Settings = nextSettings
	current.UpdatedAt = updatedAt
	return current, nil
}

func (s *Service) loadPeriodCloseState(ctx context.Context, tenantID, rawPeriodEndDate string) (*Tenant, time.Time, *time.Time, error) {
	current, err := s.GetTenant(ctx, tenantID)
	if err != nil {
		return nil, time.Time{}, nil, err
	}

	periodEndDate, err := parseMonthEndDate(rawPeriodEndDate)
	if err != nil {
		return nil, time.Time{}, nil, err
	}

	lockDate, err := parseOptionalLockDate(current.Settings.PeriodLockDate)
	if err != nil {
		return nil, time.Time{}, nil, err
	}

	return current, periodEndDate, lockDate, nil
}

func parseMonthEndDate(raw string) (time.Time, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return time.Time{}, fmt.Errorf("period end date is required")
	}

	parsed, err := time.Parse(periodCloseDateLayout, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("period end date must use YYYY-MM-DD")
	}

	normalized := normalizePeriodDate(parsed)
	if normalized.Day() != daysInMonth(normalized) {
		return time.Time{}, fmt.Errorf("period end date must be the last day of a month")
	}

	return normalized, nil
}

func parseOptionalLockDate(raw *string) (*time.Time, error) {
	if raw == nil {
		return nil, nil
	}

	value := strings.TrimSpace(*raw)
	if value == "" {
		return nil, nil
	}

	parsed, err := time.Parse(periodCloseDateLayout, value)
	if err != nil {
		return nil, fmt.Errorf("invalid tenant period lock date %q: %w", value, err)
	}

	normalized := normalizePeriodDate(parsed)
	return &normalized, nil
}

func formatOptionalDate(value *time.Time) *string {
	if value == nil {
		return nil
	}
	formatted := value.Format(periodCloseDateLayout)
	return &formatted
}

func formatOptionalDateFromString(value *string) *string {
	if value == nil {
		return nil
	}
	formatted := strings.TrimSpace(*value)
	if formatted == "" {
		return nil
	}
	return &formatted
}

func normalizePeriodDate(value time.Time) time.Time {
	utcValue := value.UTC()
	return time.Date(utcValue.Year(), utcValue.Month(), utcValue.Day(), 0, 0, 0, 0, time.UTC)
}

func daysInMonth(value time.Time) int {
	return time.Date(value.Year(), value.Month()+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

func closeKindForDate(periodEndDate time.Time, fiscalYearStartMonth int) string {
	if fiscalYearStartMonth <= 0 || fiscalYearStartMonth > 12 {
		fiscalYearStartMonth = 1
	}

	fiscalYearEndMonth := fiscalYearStartMonth - 1
	if fiscalYearEndMonth == 0 {
		fiscalYearEndMonth = 12
	}

	if int(periodEndDate.Month()) == fiscalYearEndMonth && periodEndDate.Day() == daysInMonth(periodEndDate) {
		return PeriodCloseKindYearEnd
	}

	return PeriodCloseKindMonthEnd
}
