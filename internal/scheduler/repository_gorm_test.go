package scheduler

import "testing"

func TestTenantModelTableName(t *testing.T) {
	var model tenantModel
	if got := model.TableName(); got != "tenants" {
		t.Fatalf("TableName() = %q, want %q", got, "tenants")
	}
}

func TestNewGORMRepository(t *testing.T) {
	repo := NewGORMRepository(nil)
	if repo == nil {
		t.Fatal("NewGORMRepository() returned nil")
	}
	if repo.db != nil {
		t.Fatalf("NewGORMRepository(nil).db = %#v, want nil", repo.db)
	}
}

func TestPeriodLockDateFromSettings(t *testing.T) {
	tests := []struct {
		name     string
		settings []byte
		want     string
	}{
		{
			name:     "empty settings",
			settings: nil,
			want:     "",
		},
		{
			name:     "invalid json",
			settings: []byte(`{"period_lock_date":`),
			want:     "",
		},
		{
			name:     "missing period lock date",
			settings: []byte(`{"locale":"et-EE"}`),
			want:     "",
		},
		{
			name:     "non-string period lock date",
			settings: []byte(`{"period_lock_date":20260531}`),
			want:     "",
		},
		{
			name:     "valid period lock date",
			settings: []byte(`{"period_lock_date":"2026-05-31","locale":"et-EE"}`),
			want:     "2026-05-31",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := periodLockDateFromSettings(tt.settings); got != tt.want {
				t.Fatalf("periodLockDateFromSettings() = %q, want %q", got, tt.want)
			}
		})
	}
}
