package smartaccountsexecutor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/HMB-research/open-accounting/internal/database"
	"github.com/HMB-research/open-accounting/internal/models"
	"github.com/HMB-research/open-accounting/internal/smartaccountssync"
	"github.com/jackc/pgx/v5/pgxpool"
	"gorm.io/gorm"
)

// CaptureCoverage is an immutable decision derived only from final staged
// run metadata. A false result blocks GL preview; it never discards the source
// archive or alters any accounting record.
type CaptureCoverage struct {
	Complete bool
	Gaps     []CaptureCoverageGap
}

type CaptureCoverageGap struct {
	ResourceID string
	Code       string
	Message    string
}

// CaptureCoverageRepository is the production bridge from the safe public
// capture-history table to the executor. It does not have access to archive
// chunks, source credentials, or financial writers.
type CaptureCoverageRepository struct{ db *gorm.DB }

var newCoverageGormDBFromPool = database.NewGormDBFromPool

func NewCaptureCoverageRepository(pool *pgxpool.Pool) *CaptureCoverageRepository {
	if pool == nil {
		return &CaptureCoverageRepository{}
	}
	db, err := newCoverageGormDBFromPool(context.Background(), pool)
	if err != nil {
		panic(fmt.Errorf("create SmartAccounts capture coverage repository: %w", err))
	}
	return &CaptureCoverageRepository{db: db}
}

func NewGORMCaptureCoverageRepository(db *gorm.DB) *CaptureCoverageRepository {
	return &CaptureCoverageRepository{db: db}
}

func (r *CaptureCoverageRepository) AssessCaptureCoverage(ctx context.Context, tenantID, sourceCompanyID, packageID string) (CaptureCoverage, error) {
	if r == nil || r.db == nil {
		return CaptureCoverage{}, errors.New("SmartAccounts capture coverage storage is not configured")
	}
	var rows []models.SmartAccountsSyncCaptureRunRecord
	if err := r.db.WithContext(ctx).Table("public.smartaccounts_sync_capture_run_history").Where("tenant_id = ? AND source_company_id = ?", strings.TrimSpace(tenantID), strings.TrimSpace(sourceCompanyID)).Order("updated_at ASC, run_id ASC").Find(&rows).Error; err != nil {
		return CaptureCoverage{}, fmt.Errorf("load SmartAccounts capture coverage: %w", err)
	}
	progresses := make([]smartaccountssync.CaptureProgress, 0, len(rows))
	for _, row := range rows {
		var progress smartaccountssync.CaptureProgress
		if err := json.Unmarshal(row.Progress, &progress); err != nil || strings.TrimSpace(progress.RunID) != row.RunID {
			return CaptureCoverage{}, errors.New("stored SmartAccounts capture coverage is invalid")
		}
		progresses = append(progresses, progress)
	}
	return AssessCaptureCoverage(progresses, packageID), nil
}

// AssessCaptureCoverage is pure so tests can prove the same rule applied by
// production persistence. A full-history package establishes the baseline;
// later finalized staged runs may satisfy only its initially incomplete
// resource entries (for example a required explicit date window).
func AssessCaptureCoverage(progresses []smartaccountssync.CaptureProgress, packageID string) CaptureCoverage {
	var baseline *smartaccountssync.CaptureProgress
	completedWindows := map[string][]smartaccountssync.CaptureProgress{}
	for index := range progresses {
		progress := &progresses[index]
		if !isFinalStagedProgress(*progress) {
			continue
		}
		if progress.Staging.PackageID == packageID {
			baseline = progress
		}
		if progress.ScopeMode != "window" {
			continue
		}
		for _, resource := range progress.Resources {
			if resource.Status == "completed" {
				completedWindows[resource.ResourceID] = append(completedWindows[resource.ResourceID], *progress)
			}
		}
	}
	if baseline == nil {
		return CaptureCoverage{Gaps: []CaptureCoverageGap{{Code: "full_capture_coverage_required", Message: "the staged package has no durable capture-history evidence"}}}
	}
	if baseline.ScopeMode != "full_history" {
		return CaptureCoverage{Gaps: []CaptureCoverageGap{{Code: "full_history_capture_required", Message: "GL preview requires a finalized full-history capture package"}}}
	}
	if len(baseline.Resources) == 0 {
		return CaptureCoverage{Gaps: []CaptureCoverageGap{{Code: "full_capture_coverage_required", Message: "the full-history capture did not report resource coverage"}}}
	}
	cutoff, cutoffErr := time.Parse(time.DateOnly, strings.TrimSpace(baseline.SourceAsOfDate))
	if cutoffErr != nil {
		return CaptureCoverage{Gaps: []CaptureCoverageGap{{Code: "source_as_of_required", Message: "the full-history capture has no valid source-as-of date"}}}
	}
	gaps := make([]CaptureCoverageGap, 0)
	seen := map[string]bool{}
	for _, resource := range baseline.Resources {
		resourceID := strings.TrimSpace(resource.ResourceID)
		if resourceID == "" || seen[resourceID] {
			gaps = append(gaps, CaptureCoverageGap{Code: "full_capture_coverage_invalid", Message: "the full-history capture has invalid resource coverage metadata"})
			continue
		}
		seen[resourceID] = true
		if resource.Status == "completed" {
			continue
		}
		if windowsCoverCutoff(completedWindows[resourceID], cutoff) {
			continue
		}
		gaps = append(gaps, captureCoverageGap(resource))
	}
	sort.Slice(gaps, func(left, right int) bool {
		if gaps[left].ResourceID == gaps[right].ResourceID {
			return gaps[left].Code < gaps[right].Code
		}
		return gaps[left].ResourceID < gaps[right].ResourceID
	})
	return CaptureCoverage{Complete: len(gaps) == 0, Gaps: gaps}
}

// windowsCoverCutoff accepts a follow-up only when it is a final staged
// date-window capture that reaches the source-as-of date of the full capture.
// A successful window ending earlier cannot prove a full historical snapshot.
// The user/accountant still selects the historical start because the vendor
// API does not expose an authoritative company-history lower bound.
func windowsCoverCutoff(windows []smartaccountssync.CaptureProgress, cutoff time.Time) bool {
	for _, window := range windows {
		from, fromErr := time.Parse(time.DateOnly, strings.TrimSpace(window.DateFrom))
		to, toErr := time.Parse(time.DateOnly, strings.TrimSpace(window.DateTo))
		if fromErr == nil && toErr == nil && !to.Before(from) && !to.Before(cutoff) {
			return true
		}
	}
	return false
}

func isFinalStagedProgress(progress smartaccountssync.CaptureProgress) bool {
	return progress.Staging != nil && progress.Staging.Finalized && strings.EqualFold(strings.TrimSpace(progress.Staging.Status), "staged_review_required") && strings.TrimSpace(progress.Staging.PackageID) != ""
}

func captureCoverageGap(resource smartaccountssync.CaptureResourceStatus) CaptureCoverageGap {
	resourceID := strings.TrimSpace(resource.ResourceID)
	switch {
	case resource.Status == "brave_discovery_required":
		return CaptureCoverageGap{ResourceID: resourceID, Code: "brave_discovery_required", Message: "resource requires verified read-only Brave or vendor endpoint discovery before GL preview"}
	case resource.Status == "review_required" && resource.ReasonCode == "source_date_window_required":
		return CaptureCoverageGap{ResourceID: resourceID, Code: "date_window_capture_required", Message: "resource requires a finalized explicit date-window capture before GL preview"}
	default:
		return CaptureCoverageGap{ResourceID: resourceID, Code: "resource_coverage_required", Message: "resource is not durably captured in a finalized staged package"}
	}
}
