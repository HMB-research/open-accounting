package smartaccountsexecutor

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/HMB-research/open-accounting/internal/importdelivery"
	"github.com/HMB-research/open-accounting/internal/smartaccountssync"
)

func stagedProgress(packageID, scope string, resources ...smartaccountssync.CaptureResourceStatus) smartaccountssync.CaptureProgress {
	return smartaccountssync.CaptureProgress{
		RunID:     packageID + "-run",
		ScopeMode: scope,
		Resources: resources,
		Staging: &smartaccountssync.CaptureStaging{
			PackageID: packageID,
			Status:    "staged_review_required",
			Finalized: true,
		},
	}
}

func fullHistoryProgress(packageID string, resources ...smartaccountssync.CaptureResourceStatus) smartaccountssync.CaptureProgress {
	progress := stagedProgress(packageID, "full_history", resources...)
	progress.SourceAsOfDate = "2026-08-27"
	return progress
}

func TestAssessCaptureCoverageRequiresFullHistoryAndEveryResource(t *testing.T) {
	full := fullHistoryProgress("full-package",
		smartaccountssync.CaptureResourceStatus{ResourceID: "general.entries.get", Status: "completed"},
		smartaccountssync.CaptureResourceStatus{ResourceID: "inventory.warehouse_movements.get", Status: "review_required", ReasonCode: "source_date_window_required"},
		smartaccountssync.CaptureResourceStatus{ResourceID: "general.account_balances.get", Status: "brave_discovery_required"},
	)
	coverage := AssessCaptureCoverage([]smartaccountssync.CaptureProgress{full}, "full-package")
	if coverage.Complete || len(coverage.Gaps) != 2 || coverage.Gaps[0].Code != "brave_discovery_required" || coverage.Gaps[1].Code != "date_window_capture_required" {
		t.Fatalf("unexpected coverage: %#v", coverage)
	}

	window := stagedProgress("window-package", "window",
		smartaccountssync.CaptureResourceStatus{ResourceID: "inventory.warehouse_movements.get", Status: "completed"},
	)
	window.DateFrom, window.DateTo = "2020-01-01", "2026-08-27"
	coverage = AssessCaptureCoverage([]smartaccountssync.CaptureProgress{full, window}, "full-package")
	if coverage.Complete || len(coverage.Gaps) != 1 || coverage.Gaps[0].ResourceID != "general.account_balances.get" {
		t.Fatalf("window capture must close only its own gap: %#v", coverage)
	}
}

func TestAssessCaptureCoverageRejectsDateWindowBeforeFullCaptureCutoff(t *testing.T) {
	full := fullHistoryProgress("full-package", smartaccountssync.CaptureResourceStatus{ResourceID: "inventory.warehouse_movements.get", Status: "review_required", ReasonCode: "source_date_window_required"})
	earlyWindow := stagedProgress("window-package", "window", smartaccountssync.CaptureResourceStatus{ResourceID: "inventory.warehouse_movements.get", Status: "completed"})
	earlyWindow.DateFrom, earlyWindow.DateTo = "2020-01-01", "2026-08-26"
	coverage := AssessCaptureCoverage([]smartaccountssync.CaptureProgress{full, earlyWindow}, "full-package")
	if coverage.Complete || len(coverage.Gaps) != 1 || coverage.Gaps[0].Code != "date_window_capture_required" {
		t.Fatalf("window ending before source cutoff must not satisfy coverage: %#v", coverage)
	}

	matchingWindow := earlyWindow
	matchingWindow.DateTo = "2026-08-27"
	coverage = AssessCaptureCoverage([]smartaccountssync.CaptureProgress{full, matchingWindow}, "full-package")
	if !coverage.Complete {
		t.Fatalf("window through source cutoff must satisfy its resource coverage: %#v", coverage)
	}
}

func TestAssessCaptureCoverageRejectsWindowPackageAndMissingHistory(t *testing.T) {
	window := stagedProgress("window-package", "window", smartaccountssync.CaptureResourceStatus{ResourceID: "general.entries.get", Status: "completed"})
	coverage := AssessCaptureCoverage([]smartaccountssync.CaptureProgress{window}, "window-package")
	if coverage.Complete || len(coverage.Gaps) != 1 || coverage.Gaps[0].Code != "full_history_capture_required" {
		t.Fatalf("window package must not become GL baseline: %#v", coverage)
	}
	coverage = AssessCaptureCoverage(nil, "missing-package")
	if coverage.Complete || len(coverage.Gaps) != 1 || coverage.Gaps[0].Code != "full_capture_coverage_required" {
		t.Fatalf("missing history must be blocked: %#v", coverage)
	}
}

func TestPreviewRequiresCoverageReader(t *testing.T) {
	source := "sa-key-v1-test"
	planner := NewPlanner(fakeArchive{
		status:   importdelivery.Status{Status: importdelivery.StatusStagedReview, SourceCompanyID: source},
		manifest: importdelivery.Manifest{Provider: Provider, SourceCompanyID: source, Authority: importdelivery.Authority{GeneralLedgerAuthority: Provider, SmartAccountsGLAuthoritative: true}},
		records:  []json.RawMessage{authoritativeRecord(t, source)},
	}, nil, fakeAccounts{"oa-1000": true, "oa-3000": true})
	preview, err := planner.Preview(context.Background(), "tenant", "tenant-id", "package", PreviewRequest{AccountMappings: []AccountMapping{{SourceAccountExternalID: "1000", TargetAccountID: "oa-1000"}, {SourceAccountExternalID: "3000", TargetAccountID: "oa-3000"}}})
	if err != ErrPreviewReviewRequired || preview.FinancialWritesPlanned || len(preview.Issues) != 1 || preview.Issues[0].Code != "full_capture_coverage_unavailable" {
		t.Fatalf("missing coverage reader must not plan financial writes: %#v / %v", preview, err)
	}
}
