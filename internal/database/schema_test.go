package database

import "testing"

func TestValidateIdentifier(t *testing.T) {
	tests := []struct {
		name       string
		identifier string
		wantErr    bool
	}{
		{name: "simple", identifier: "tenant_demo1"},
		{name: "underscore", identifier: "inventory_movements"},
		{name: "starts with number", identifier: "1tenant", wantErr: true},
		{name: "contains dash", identifier: "tenant-demo", wantErr: true},
		{name: "contains quote", identifier: `bad"name`, wantErr: true},
		{name: "contains dot", identifier: "tenant.demo", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateIdentifier(tt.identifier)
			if tt.wantErr && err == nil {
				t.Fatalf("expected error for %q", tt.identifier)
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error for %q: %v", tt.identifier, err)
			}
		})
	}
}

func TestQuoteIdentifier(t *testing.T) {
	quoted, err := QuoteIdentifier("tenant_demo1")
	if err != nil {
		t.Fatalf("QuoteIdentifier returned error: %v", err)
	}
	if quoted != `"tenant_demo1"` {
		t.Fatalf("unexpected quoted identifier: %s", quoted)
	}
}

func TestQuoteIdentifierRejectsInvalidIdentifier(t *testing.T) {
	quoted, err := QuoteIdentifier("tenant-demo")
	if err == nil {
		t.Fatal("expected invalid identifier error")
	}
	if quoted != "" {
		t.Fatalf("expected empty quoted identifier on error, got %q", quoted)
	}
}

func TestQualifiedTable(t *testing.T) {
	qualified, err := QualifiedTable("tenant_demo1", "contacts")
	if err != nil {
		t.Fatalf("QualifiedTable returned error: %v", err)
	}
	if qualified != `"tenant_demo1"."contacts"` {
		t.Fatalf("unexpected qualified table: %s", qualified)
	}
}

func TestQualifiedTableRejectsInvalidSchema(t *testing.T) {
	qualified, err := QualifiedTable("tenant-demo", "contacts")
	if err == nil {
		t.Fatal("expected invalid schema error")
	}
	if qualified != "" {
		t.Fatalf("expected empty qualified table on error, got %q", qualified)
	}
}

func TestQualifiedTableRejectsInvalidTable(t *testing.T) {
	qualified, err := QualifiedTable("tenant_demo", "bad-table")
	if err == nil {
		t.Fatal("expected invalid table error")
	}
	if qualified != "" {
		t.Fatalf("expected empty qualified table on error, got %q", qualified)
	}
}
