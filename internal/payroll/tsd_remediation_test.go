package payroll

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

func TestBuildTSDRemediationActions(t *testing.T) {
	submittedAt := time.Date(2026, 3, 20, 10, 0, 0, 0, time.UTC)

	tests := []struct {
		name        string
		declaration *TSDDeclaration
		wantCodes   []string
	}{
		{
			name: "draft with rows",
			declaration: &TSDDeclaration{
				ID:            "tsd-draft",
				PeriodYear:    2026,
				PeriodMonth:   3,
				PayrollRunID:  "run-1",
				Status:        TSDDraft,
				TotalPayments: decimal.NewFromInt(3200),
				Rows:          []TSDRow{{ID: "row-1"}},
			},
			wantCodes: []string{"tsd_export_and_submit"},
		},
		{
			name: "draft without rows or totals",
			declaration: &TSDDeclaration{
				ID:          "tsd-empty",
				PeriodYear:  2026,
				PeriodMonth: 4,
				Status:      TSDDraft,
			},
			wantCodes: []string{"tsd_no_declaration_rows", "tsd_export_and_submit"},
		},
		{
			name: "submitted with timestamp",
			declaration: &TSDDeclaration{
				ID:          "tsd-submitted",
				PeriodYear:  2026,
				PeriodMonth: 5,
				Status:      TSDSubmitted,
				SubmittedAt: &submittedAt,
			},
			wantCodes: []string{"tsd_awaiting_authority_acceptance"},
		},
		{
			name: "submitted missing timestamp",
			declaration: &TSDDeclaration{
				ID:          "tsd-submitted-missing-date",
				PeriodYear:  2026,
				PeriodMonth: 6,
				Status:      TSDSubmitted,
			},
			wantCodes: []string{"tsd_awaiting_authority_acceptance", "tsd_submission_date_missing"},
		},
		{
			name: "accepted",
			declaration: &TSDDeclaration{
				ID:          "tsd-accepted",
				PeriodYear:  2026,
				PeriodMonth: 7,
				Status:      TSDAccepted,
			},
			wantCodes: []string{"tsd_accepted_archive"},
		},
		{
			name: "rejected",
			declaration: &TSDDeclaration{
				ID:          "tsd-rejected",
				PeriodYear:  2026,
				PeriodMonth: 8,
				Status:      TSDRejected,
			},
			wantCodes: []string{"tsd_rejected_review"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actions := BuildTSDRemediationActions(tt.declaration)
			if got := tsdRemediationCodes(actions); !equalStringSlices(got, tt.wantCodes) {
				t.Fatalf("unexpected action codes: got %v want %v", got, tt.wantCodes)
			}
			for _, action := range actions {
				if action.Scope != "tax" {
					t.Fatalf("expected tax scope, got %q", action.Scope)
				}
				if action.OwnerRole != "accountant" {
					t.Fatalf("expected accountant owner, got %q", action.OwnerRole)
				}
				if action.Period == "" || action.Action == "" || action.CLICommand == "" {
					t.Fatalf("expected action metadata to be populated: %+v", action)
				}
			}
		})
	}

	if BuildTSDRemediationActions(nil) != nil {
		t.Fatal("expected nil actions for nil declaration")
	}
}

func tsdRemediationCodes(actions []TSDRemediationAction) []string {
	codes := make([]string, 0, len(actions))
	for _, action := range actions {
		codes = append(codes, action.Code)
	}
	return codes
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
