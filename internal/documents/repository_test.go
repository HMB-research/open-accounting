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
