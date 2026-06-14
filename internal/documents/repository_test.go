package documents

import "testing"

func TestEntityTableName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		entityType string
		wantTable  string
		wantErr    bool
	}{
		{name: "invoice", entityType: EntityTypeInvoice, wantTable: "invoices"},
		{name: "journal entry", entityType: EntityTypeJournalEntry, wantTable: "journal_entries"},
		{name: "payment", entityType: EntityTypePayment, wantTable: "payments"},
		{name: "bank transaction", entityType: EntityTypeBankTxn, wantTable: "bank_transactions"},
		{name: "asset", entityType: EntityTypeAsset, wantTable: "fixed_assets"},
		{name: "expense", entityType: EntityTypeExpense, wantTable: "expenses"},
		{name: "quote", entityType: EntityTypeQuote, wantTable: "quotes"},
		{name: "order", entityType: EntityTypeOrder, wantTable: "orders"},
		{name: "leave record", entityType: EntityTypeLeaveRecord, wantTable: "leave_records"},
		{name: "TSD declaration", entityType: EntityTypeTSD, wantTable: "tsd_declarations"},
		{name: "KMD declaration", entityType: EntityTypeKMD, wantTable: "kmd_declarations"},
		{name: "unsupported", entityType: "contact", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := entityTableName(tt.entityType)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q", tt.entityType)
				}
				return
			}
			if err != nil {
				t.Fatalf("entityTableName returned error: %v", err)
			}
			if got != tt.wantTable {
				t.Fatalf("expected table %q, got %q", tt.wantTable, got)
			}
		})
	}
}
