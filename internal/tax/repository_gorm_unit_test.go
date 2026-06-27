package tax

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

func TestGORMRepositoryNilDatabaseGuards(t *testing.T) {
	ctx := context.Background()
	schemaName := "tenant_demo"
	tenantID := "tenant-1"
	declarationID := "declaration-1"
	startDate := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	endDate := time.Date(2026, time.February, 1, 0, 0, 0, 0, time.UTC)
	now := time.Date(2026, time.February, 2, 12, 0, 0, 0, time.UTC)
	declaration := &KMDDeclaration{
		ID:             declarationID,
		TenantID:       tenantID,
		Year:           2026,
		Month:          1,
		Status:         KMDStatusDraft,
		TotalOutputVAT: decimal.NewFromInt(220),
		TotalInputVAT:  decimal.NewFromInt(55),
		Rows: []KMDRow{
			{Code: KMDRow1, Description: "Sales", TaxBase: decimal.NewFromInt(1000), TaxAmount: decimal.NewFromInt(220)},
		},
		CreatedAt: now,
		UpdatedAt: now,
	}

	repositories := []struct {
		name string
		repo *GORMRepository
	}{
		{name: "nil receiver"},
		{name: "nil gorm database", repo: NewGORMRepository(nil)},
	}

	tests := []struct {
		name string
		run  func(t *testing.T, repo *GORMRepository) error
	}{
		{
			name: "dbWithContext",
			run: func(t *testing.T, repo *GORMRepository) error {
				db, err := repo.dbWithContext(ctx)
				if db != nil {
					t.Fatalf("dbWithContext() db = %#v, want nil", db)
				}
				return err
			},
		},
		{
			name: "tenantTable",
			run: func(t *testing.T, repo *GORMRepository) error {
				db, err := repo.tenantTable(ctx, schemaName, "kmd_declarations")
				if db != nil {
					t.Fatalf("tenantTable() db = %#v, want nil", db)
				}
				return err
			},
		},
		{
			name: "QueryVATData",
			run: func(t *testing.T, repo *GORMRepository) error {
				rows, err := repo.QueryVATData(ctx, schemaName, tenantID, startDate, endDate)
				if rows != nil {
					t.Fatalf("QueryVATData() rows = %#v, want nil", rows)
				}
				return err
			},
		},
		{
			name: "QueryKMDINFData",
			run: func(t *testing.T, repo *GORMRepository) error {
				rows, err := repo.QueryKMDINFData(ctx, schemaName, tenantID, startDate, endDate, decimal.NewFromInt(1000))
				if rows != nil {
					t.Fatalf("QueryKMDINFData() rows = %#v, want nil", rows)
				}
				return err
			},
		},
		{
			name: "QueryEUVATOSSData",
			run: func(t *testing.T, repo *GORMRepository) error {
				rows, err := repo.QueryEUVATOSSData(ctx, schemaName, tenantID, startDate, endDate, false)
				if rows != nil {
					t.Fatalf("QueryEUVATOSSData() rows = %#v, want nil", rows)
				}
				return err
			},
		},
		{
			name: "SaveDeclaration",
			run: func(t *testing.T, repo *GORMRepository) error {
				return repo.SaveDeclaration(ctx, schemaName, declaration)
			},
		},
		{
			name: "GetDeclaration",
			run: func(t *testing.T, repo *GORMRepository) error {
				got, err := repo.GetDeclaration(ctx, schemaName, tenantID, 2026, 1)
				if got != nil {
					t.Fatalf("GetDeclaration() declaration = %#v, want nil", got)
				}
				return err
			},
		},
		{
			name: "ListDeclarations",
			run: func(t *testing.T, repo *GORMRepository) error {
				rows, err := repo.ListDeclarations(ctx, schemaName, tenantID)
				if rows != nil {
					t.Fatalf("ListDeclarations() rows = %#v, want nil", rows)
				}
				return err
			},
		},
		{
			name: "MarkKMDSubmitted",
			run: func(t *testing.T, repo *GORMRepository) error {
				return repo.MarkKMDSubmitted(ctx, schemaName, tenantID, declarationID, now)
			},
		},
		{
			name: "UpdateKMDStatus",
			run: func(t *testing.T, repo *GORMRepository) error {
				return repo.UpdateKMDStatus(ctx, schemaName, tenantID, declarationID, KMDStatusSubmitted, now)
			},
		},
	}

	for _, repository := range repositories {
		t.Run(repository.name, func(t *testing.T) {
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					err := tt.run(t, repository.repo)
					if err == nil {
						t.Fatal("expected error")
					}
					if !strings.Contains(err.Error(), "tax repository database is not configured") {
						t.Fatalf("error = %q, want tax repository database is not configured", err)
					}
				})
			}
		})
	}
}
