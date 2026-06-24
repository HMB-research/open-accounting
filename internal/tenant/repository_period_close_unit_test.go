package tenant

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestGORMRepositoryPeriodCloseNilDatabase(t *testing.T) {
	repo := NewGORMRepository(nil)
	ctx := context.Background()
	now := time.Date(2026, 6, 24, 12, 0, 0, 0, time.UTC)
	event := &PeriodCloseEvent{
		ID:            "period-close-1",
		TenantID:      "tenant-1",
		Action:        PeriodCloseActionClose,
		CloseKind:     PeriodCloseKindMonthEnd,
		PeriodEndDate: "2026-05-31",
		PerformedBy:   "user-1",
		CreatedAt:     now,
	}

	if repo.db != nil {
		t.Fatalf("NewGORMRepository(nil).db = %#v, want nil", repo.db)
	}

	tests := []struct {
		name string
		run  func(t *testing.T) error
	}{
		{
			name: "dbWithContext",
			run: func(t *testing.T) error {
				db, err := repo.dbWithContext(ctx)
				if db != nil {
					t.Fatalf("dbWithContext() db = %#v, want nil", db)
				}
				return err
			},
		},
		{
			name: "UpdateTenantWithPeriodCloseEvent",
			run: func(t *testing.T) error {
				return repo.UpdateTenantWithPeriodCloseEvent(ctx, "tenant-1", "Demo", []byte(`{}`), now, event)
			},
		},
		{
			name: "ListPeriodCloseEvents",
			run: func(t *testing.T) error {
				events, err := repo.ListPeriodCloseEvents(ctx, "tenant-1", 0)
				if events != nil {
					t.Fatalf("ListPeriodCloseEvents() events = %#v, want nil", events)
				}
				return err
			},
		},
		{
			name: "GetLatestCloseEventForPeriod",
			run: func(t *testing.T) error {
				event, err := repo.GetLatestCloseEventForPeriod(ctx, "tenant-1", "2026-05-31")
				if event != nil {
					t.Fatalf("GetLatestCloseEventForPeriod() event = %#v, want nil", event)
				}
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.run(t)
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), "tenant repository database is not configured") {
				t.Fatalf("error = %q, want tenant repository database is not configured", err)
			}
		})
	}
}

func TestGORMRepositoryNilReceiverDatabaseGuard(t *testing.T) {
	var repo *GORMRepository

	db, err := repo.dbWithContext(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if db != nil {
		t.Fatalf("dbWithContext() db = %#v, want nil", db)
	}
	if !strings.Contains(err.Error(), "tenant repository database is not configured") {
		t.Fatalf("error = %q, want tenant repository database is not configured", err)
	}
}
