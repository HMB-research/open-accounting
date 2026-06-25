package recurring

import (
	"context"
	"strings"
	"testing"
)

func TestRecurringWave8NilRepositoryGuards(t *testing.T) {
	service := NewService(nil, nil, nil, nil, nil, nil)
	ctx := context.Background()

	tests := []struct {
		name string
		call func() error
	}{
		{
			name: "create",
			call: func() error {
				_, err := service.Create(ctx, "tenant-1", "tenant_demo", nil)
				return err
			},
		},
		{
			name: "get",
			call: func() error {
				_, err := service.GetByID(ctx, "tenant-1", "tenant_demo", "rec-1")
				return err
			},
		},
		{
			name: "delete",
			call: func() error {
				return service.Delete(ctx, "tenant-1", "tenant_demo", "rec-1")
			},
		},
		{
			name: "generate due",
			call: func() error {
				_, err := service.GenerateDueInvoices(ctx, "tenant-1", "tenant_demo", "user-1")
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.call()
			if err == nil || !strings.Contains(err.Error(), "repository not available") {
				t.Fatalf("%s error = %v, want repository guard", tt.name, err)
			}
		})
	}
}
