package tax

import (
	"testing"
	"time"

	"github.com/HMB-research/open-accounting/internal/models"
	"github.com/shopspring/decimal"
)

func TestMergeVATAggregateRowsCombinesDuplicateKeysAndSkipsZeroTax(t *testing.T) {
	rows := []vatAggregateScanRow{
		{
			VATRate:   models.Decimal{Decimal: decimal.NewFromInt(22)},
			IsOutput:  true,
			TaxBase:   models.Decimal{Decimal: decimal.NewFromInt(1000)},
			TaxAmount: models.Decimal{Decimal: decimal.NewFromInt(220)},
		},
		{
			VATRate:   models.Decimal{Decimal: decimal.NewFromInt(22)},
			IsOutput:  true,
			TaxBase:   models.Decimal{Decimal: decimal.NewFromInt(500)},
			TaxAmount: models.Decimal{Decimal: decimal.NewFromInt(110)},
		},
		{
			VATRate:   models.Decimal{Decimal: decimal.NewFromInt(22)},
			IsOutput:  false,
			TaxBase:   models.Decimal{Decimal: decimal.NewFromInt(250)},
			TaxAmount: models.Decimal{Decimal: decimal.NewFromInt(55)},
		},
		{
			VATRate:   models.Decimal{Decimal: decimal.NewFromInt(9)},
			IsOutput:  true,
			TaxBase:   models.Decimal{Decimal: decimal.NewFromInt(100)},
			TaxAmount: models.Decimal{Decimal: decimal.Zero},
		},
	}

	merged := mergeVATAggregateRows(rows)

	if len(merged) != 2 {
		t.Fatalf("mergeVATAggregateRows() returned %d rows, want 2: %#v", len(merged), merged)
	}

	assertVATRow(t, merged[0], decimal.NewFromInt(22), true, decimal.NewFromInt(1500), decimal.NewFromInt(330))
	assertVATRow(t, merged[1], decimal.NewFromInt(22), false, decimal.NewFromInt(250), decimal.NewFromInt(55))
}

func TestKMDDeclarationModelMappings(t *testing.T) {
	submittedAt := time.Date(2026, 6, 10, 15, 30, 0, 0, time.UTC)
	createdAt := time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC)
	updatedAt := time.Date(2026, 6, 10, 16, 0, 0, 0, time.UTC)

	declaration := &KMDDeclaration{
		ID:             "kmd-1",
		TenantID:       "tenant-1",
		Year:           2026,
		Month:          5,
		Status:         "SUBMITTED",
		TotalOutputVAT: decimal.NewFromInt(330),
		TotalInputVAT:  decimal.NewFromInt(55),
		Rows: []KMDRow{
			{Code: KMDRow1, Description: "Standard rate sales", TaxBase: decimal.NewFromInt(1500), TaxAmount: decimal.NewFromInt(330)},
		},
		SubmittedAt: &submittedAt,
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
	}

	model := kmdDeclarationToModel(declaration)

	if model.ID != declaration.ID ||
		model.TenantID != declaration.TenantID ||
		model.Year != declaration.Year ||
		model.Month != declaration.Month ||
		model.Status != declaration.Status ||
		!model.TotalOutputVAT.Decimal.Equal(declaration.TotalOutputVAT) ||
		!model.TotalInputVAT.Decimal.Equal(declaration.TotalInputVAT) ||
		model.SubmittedAt != declaration.SubmittedAt ||
		!model.CreatedAt.Equal(declaration.CreatedAt) ||
		!model.UpdatedAt.Equal(declaration.UpdatedAt) {
		t.Fatalf("kmdDeclarationToModel() = %#v, want fields from %#v", model, declaration)
	}

	roundTrip := modelToKMDDeclaration(model)

	if roundTrip.ID != declaration.ID ||
		roundTrip.TenantID != declaration.TenantID ||
		roundTrip.Year != declaration.Year ||
		roundTrip.Month != declaration.Month ||
		roundTrip.Status != declaration.Status ||
		!roundTrip.TotalOutputVAT.Equal(declaration.TotalOutputVAT) ||
		!roundTrip.TotalInputVAT.Equal(declaration.TotalInputVAT) ||
		roundTrip.SubmittedAt != declaration.SubmittedAt ||
		!roundTrip.CreatedAt.Equal(declaration.CreatedAt) ||
		!roundTrip.UpdatedAt.Equal(declaration.UpdatedAt) {
		t.Fatalf("modelToKMDDeclaration() = %#v, want fields from %#v", roundTrip, model)
	}

	if len(roundTrip.Rows) != 0 {
		t.Fatalf("modelToKMDDeclaration() copied relation rows, got %#v", roundTrip.Rows)
	}
}

func TestModelToKMDRowMapsAmounts(t *testing.T) {
	model := &models.KMDRow{
		ID:            "row-1",
		DeclarationID: "kmd-1",
		Code:          KMDRow4,
		Description:   "Input VAT on domestic purchases",
		TaxBase:       models.Decimal{Decimal: decimal.NewFromInt(250)},
		TaxAmount:     models.Decimal{Decimal: decimal.NewFromInt(55)},
	}

	row := modelToKMDRow(model)

	if row.Code != model.Code ||
		row.Description != model.Description ||
		!row.TaxBase.Equal(model.TaxBase.Decimal) ||
		!row.TaxAmount.Equal(model.TaxAmount.Decimal) {
		t.Fatalf("modelToKMDRow() = %#v, want fields from %#v", row, model)
	}
}

func assertVATRow(t *testing.T, got VATAggregateRow, wantRate decimal.Decimal, wantOutput bool, wantBase, wantTax decimal.Decimal) {
	t.Helper()

	if !got.VATRate.Equal(wantRate) ||
		got.IsOutput != wantOutput ||
		!got.TaxBase.Equal(wantBase) ||
		!got.TaxAmount.Equal(wantTax) {
		t.Fatalf("VAT aggregate row = %#v, want rate=%s output=%t base=%s tax=%s", got, wantRate, wantOutput, wantBase, wantTax)
	}
}
