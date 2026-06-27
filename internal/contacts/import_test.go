package contacts

import (
	"context"
	"errors"
	"testing"

	"github.com/shopspring/decimal"
)

func TestService_ImportCSV(t *testing.T) {
	ctx := context.Background()

	t.Run("creates contacts with defaults and aliases", func(t *testing.T) {
		repo := NewMockRepository()
		service := NewServiceWithRepository(repo)
		legacyContactID := "11111111-1111-1111-1111-111111111111"

		req := &ImportContactsRequest{
			FileName: "contacts.csv",
			CSVContent: "contact_id;company_name;type;email;payment_days;credit_limit\n" +
				legacyContactID + ";Northwind OU;customer;northwind@example.com;;1500,50\n" +
				";Supply Partner;tarnija;supplier@example.com;30;1,500.50\n",
		}

		result, err := service.ImportCSV(ctx, "tenant-1", "tenant_tenant_1", req)
		if err != nil {
			t.Fatalf("ImportCSV returned error: %v", err)
		}

		if result.FileName != "contacts.csv" {
			t.Fatalf("FileName = %q, want %q", result.FileName, "contacts.csv")
		}
		if result.RowsProcessed != 2 {
			t.Fatalf("RowsProcessed = %d, want %d", result.RowsProcessed, 2)
		}
		if result.ContactsCreated != 2 {
			t.Fatalf("ContactsCreated = %d, want %d", result.ContactsCreated, 2)
		}
		if result.RowsSkipped != 0 {
			t.Fatalf("RowsSkipped = %d, want %d", result.RowsSkipped, 0)
		}
		if len(result.Errors) != 0 {
			t.Fatalf("Errors = %v, want none", result.Errors)
		}

		var importedCustomer *Contact
		var importedSupplier *Contact
		for _, contact := range repo.contacts {
			switch contact.Name {
			case "Northwind OU":
				importedCustomer = contact
			case "Supply Partner":
				importedSupplier = contact
			}
		}

		if importedCustomer == nil {
			t.Fatal("Northwind OU contact was not created")
		}
		if importedCustomer.ID != legacyContactID {
			t.Fatalf("customer ID = %s, want %s", importedCustomer.ID, legacyContactID)
		}
		if _, ok := repo.contacts[legacyContactID]; !ok {
			t.Fatalf("contact map does not contain preserved ID %s", legacyContactID)
		}
		if importedCustomer.ContactType != ContactTypeCustomer {
			t.Fatalf("customer ContactType = %s, want %s", importedCustomer.ContactType, ContactTypeCustomer)
		}
		if importedCustomer.CountryCode != "EE" {
			t.Fatalf("customer CountryCode = %s, want EE", importedCustomer.CountryCode)
		}
		if importedCustomer.PaymentTermsDays != 14 {
			t.Fatalf("customer PaymentTermsDays = %d, want 14", importedCustomer.PaymentTermsDays)
		}
		if !importedCustomer.CreditLimit.Equal(decimal.RequireFromString("1500.50")) {
			t.Fatalf("customer CreditLimit = %s, want 1500.50", importedCustomer.CreditLimit)
		}

		if importedSupplier == nil {
			t.Fatal("Supply Partner contact was not created")
		}
		if importedSupplier.ContactType != ContactTypeSupplier {
			t.Fatalf("supplier ContactType = %s, want %s", importedSupplier.ContactType, ContactTypeSupplier)
		}
		if importedSupplier.PaymentTermsDays != 30 {
			t.Fatalf("supplier PaymentTermsDays = %d, want 30", importedSupplier.PaymentTermsDays)
		}
		if !importedSupplier.CreditLimit.Equal(decimal.RequireFromString("1500.50")) {
			t.Fatalf("supplier CreditLimit = %s, want 1500.50", importedSupplier.CreditLimit)
		}
	})

	t.Run("rejects empty csv content", func(t *testing.T) {
		repo := NewMockRepository()
		service := NewServiceWithRepository(repo)

		_, err := service.ImportCSV(ctx, "tenant-1", "tenant_tenant_1", &ImportContactsRequest{CSVContent: " "})
		if err == nil {
			t.Fatal("ImportCSV error = nil, want csv_content error")
		}
		if !contains(err.Error(), "csv_content is required") {
			t.Fatalf("ImportCSV error = %q, want csv_content is required", err.Error())
		}
	})

	t.Run("rejects header only csv", func(t *testing.T) {
		repo := NewMockRepository()
		service := NewServiceWithRepository(repo)

		_, err := service.ImportCSV(ctx, "tenant-1", "tenant_tenant_1", &ImportContactsRequest{CSVContent: "name,email\n"})
		if err == nil {
			t.Fatal("ImportCSV error = nil, want no contacts error")
		}
		if !contains(err.Error(), "no contacts found in CSV") {
			t.Fatalf("ImportCSV error = %q, want no contacts found", err.Error())
		}
	})

	t.Run("returns repository list error", func(t *testing.T) {
		boom := errors.New("boom")
		repo := NewMockRepository()
		repo.ListFn = func(ctx context.Context, schemaName, tenantID string, filter *ContactFilter) ([]Contact, error) {
			return nil, boom
		}
		service := NewServiceWithRepository(repo)

		_, err := service.ImportCSV(ctx, "tenant-1", "tenant_tenant_1", &ImportContactsRequest{
			CSVContent: "name,email\nFresh Customer,fresh@example.com\n",
		})
		if !errors.Is(err, boom) {
			t.Fatalf("ImportCSV error = %v, want boom", err)
		}
		if !contains(err.Error(), "list existing contacts") {
			t.Fatalf("ImportCSV error = %q, want list existing contacts", err.Error())
		}
	})

	t.Run("skips repository create error", func(t *testing.T) {
		boom := errors.New("boom")
		repo := NewMockRepository()
		repo.CreateFn = func(ctx context.Context, schemaName string, contact *Contact) error {
			return boom
		}
		service := NewServiceWithRepository(repo)

		result, err := service.ImportCSV(ctx, "tenant-1", "tenant_tenant_1", &ImportContactsRequest{
			CSVContent: "name,email\nFresh Customer,fresh@example.com\n",
		})
		if err != nil {
			t.Fatalf("ImportCSV returned error: %v", err)
		}
		if result.RowsProcessed != 1 || result.ContactsCreated != 0 || result.RowsSkipped != 1 {
			t.Fatalf("unexpected import result: %+v", result)
		}
		if len(result.Errors) != 1 || !contains(result.Errors[0].Message, boom.Error()) {
			t.Fatalf("unexpected row errors: %+v", result.Errors)
		}
	})

	t.Run("skips invalid imported contact id", func(t *testing.T) {
		repo := NewMockRepository()
		service := NewServiceWithRepository(repo)

		result, err := service.ImportCSV(ctx, "tenant-1", "tenant_tenant_1", &ImportContactsRequest{
			CSVContent: "id,name,type,email\nlegacy-id,Bad ID,customer,bad@example.com\n",
		})
		if err != nil {
			t.Fatalf("ImportCSV returned error: %v", err)
		}

		if result.RowsProcessed != 1 {
			t.Fatalf("RowsProcessed = %d, want 1", result.RowsProcessed)
		}
		if result.ContactsCreated != 0 {
			t.Fatalf("ContactsCreated = %d, want 0", result.ContactsCreated)
		}
		if result.RowsSkipped != 1 {
			t.Fatalf("RowsSkipped = %d, want 1", result.RowsSkipped)
		}
		if len(result.Errors) != 1 {
			t.Fatalf("len(Errors) = %d, want 1", len(result.Errors))
		}
		if result.Errors[0].Row != 2 {
			t.Fatalf("Errors[0].Row = %d, want 2", result.Errors[0].Row)
		}
		if !contains(result.Errors[0].Message, "invalid id") {
			t.Fatalf("Errors[0].Message = %q, want invalid id", result.Errors[0].Message)
		}
	})

	t.Run("skips duplicate imported contact id", func(t *testing.T) {
		repo := NewMockRepository()
		existingContactID := "22222222-2222-2222-2222-222222222222"
		repo.contacts[existingContactID] = &Contact{
			ID:               existingContactID,
			TenantID:         "tenant-1",
			Name:             "Existing Customer",
			ContactType:      ContactTypeCustomer,
			CountryCode:      "EE",
			PaymentTermsDays: 14,
			IsActive:         true,
		}
		service := NewServiceWithRepository(repo)

		result, err := service.ImportCSV(ctx, "tenant-1", "tenant_tenant_1", &ImportContactsRequest{
			CSVContent: "contact_id,name,type,email\n" + existingContactID + ",Replacement Customer,customer,new@example.com\n",
		})
		if err != nil {
			t.Fatalf("ImportCSV returned error: %v", err)
		}

		if result.RowsProcessed != 1 {
			t.Fatalf("RowsProcessed = %d, want 1", result.RowsProcessed)
		}
		if result.ContactsCreated != 0 {
			t.Fatalf("ContactsCreated = %d, want 0", result.ContactsCreated)
		}
		if result.RowsSkipped != 1 {
			t.Fatalf("RowsSkipped = %d, want 1", result.RowsSkipped)
		}
		if len(result.Errors) != 1 {
			t.Fatalf("len(Errors) = %d, want 1", len(result.Errors))
		}
		if !contains(result.Errors[0].Message, "duplicate id") {
			t.Fatalf("Errors[0].Message = %q, want duplicate id", result.Errors[0].Message)
		}
	})

	t.Run("skips duplicate registration code and email", func(t *testing.T) {
		repo := NewMockRepository()
		repo.contacts["existing-contact"] = &Contact{
			ID:          "existing-contact",
			TenantID:    "tenant-1",
			Name:        "Existing Customer",
			RegCode:     "12345678",
			Email:       "existing@example.com",
			ContactType: ContactTypeCustomer,
		}
		service := NewServiceWithRepository(repo)

		result, err := service.ImportCSV(ctx, "tenant-1", "tenant_tenant_1", &ImportContactsRequest{
			CSVContent: "name,reg_code,email\n" +
				"Duplicate Reg,12345678,reg@example.com\n" +
				"Duplicate Email,87654321,existing@example.com\n",
		})
		if err != nil {
			t.Fatalf("ImportCSV returned error: %v", err)
		}
		if result.RowsProcessed != 2 || result.ContactsCreated != 0 || result.RowsSkipped != 2 {
			t.Fatalf("unexpected import result: %+v", result)
		}
		if len(result.Errors) != 2 {
			t.Fatalf("len(Errors) = %d, want 2", len(result.Errors))
		}
		if !contains(result.Errors[0].Message, "duplicate reg_code") {
			t.Fatalf("Errors[0].Message = %q, want duplicate reg_code", result.Errors[0].Message)
		}
		if !contains(result.Errors[1].Message, "duplicate email") {
			t.Fatalf("Errors[1].Message = %q, want duplicate email", result.Errors[1].Message)
		}
	})

	t.Run("skips invalid and duplicate rows", func(t *testing.T) {
		repo := NewMockRepository()
		repo.contacts["existing-contact"] = &Contact{
			ID:               "existing-contact",
			TenantID:         "tenant-1",
			Name:             "Existing Customer",
			Code:             "CUST-001",
			Email:            "existing@example.com",
			ContactType:      ContactTypeCustomer,
			CountryCode:      "EE",
			PaymentTermsDays: 14,
			IsActive:         true,
		}
		service := NewServiceWithRepository(repo)

		req := &ImportContactsRequest{
			CSVContent: "name,code,email,payment_terms_days\n" +
				"Existing Customer,CUST-001,existing@example.com,14\n" +
				",CUST-002,missing-name@example.com,14\n" +
				"Fresh Customer,CUST-003,fresh@example.com,14\n" +
				"Fresh Customer,CUST-004,fresh-duplicate@example.com,14\n" +
				"Bad Terms,CUST-005,bad-terms@example.com,net30\n",
		}

		result, err := service.ImportCSV(ctx, "tenant-1", "tenant_tenant_1", req)
		if err != nil {
			t.Fatalf("ImportCSV returned error: %v", err)
		}

		if result.RowsProcessed != 5 {
			t.Fatalf("RowsProcessed = %d, want %d", result.RowsProcessed, 5)
		}
		if result.ContactsCreated != 1 {
			t.Fatalf("ContactsCreated = %d, want %d", result.ContactsCreated, 1)
		}
		if result.RowsSkipped != 4 {
			t.Fatalf("RowsSkipped = %d, want %d", result.RowsSkipped, 4)
		}
		if len(result.Errors) != 4 {
			t.Fatalf("len(Errors) = %d, want %d", len(result.Errors), 4)
		}

		wantMessages := []string{
			"duplicate code",
			"name is required",
			"duplicate name",
			"invalid payment_terms_days",
		}
		for idx, want := range wantMessages {
			if !contains(result.Errors[idx].Message, want) {
				t.Fatalf("Errors[%d] = %q, want to contain %q", idx, result.Errors[idx].Message, want)
			}
		}

		if result.Errors[0].Row != 2 || result.Errors[1].Row != 3 || result.Errors[2].Row != 5 || result.Errors[3].Row != 6 {
			t.Fatalf("unexpected row numbers: %+v", result.Errors)
		}
	})

	t.Run("rejects csv without name column", func(t *testing.T) {
		repo := NewMockRepository()
		service := NewServiceWithRepository(repo)

		_, err := service.ImportCSV(ctx, "tenant-1", "tenant_tenant_1", &ImportContactsRequest{
			CSVContent: "email,code\nhello@example.com,CUST-001\n",
		})
		if err == nil {
			t.Fatal("ImportCSV error = nil, want error")
		}
		if !contains(err.Error(), "missing required name column") {
			t.Fatalf("ImportCSV error = %q, want missing required name column", err.Error())
		}
	})
}

func TestContactImportParsingEdgeCases(t *testing.T) {
	if _, err := parseImportRows(" "); err == nil || !contains(err.Error(), "csv_content is required") {
		t.Fatalf("blank parse error = %v, want csv_content is required", err)
	}
	if _, err := parseImportRows("\"unterminated"); err == nil || !contains(err.Error(), "parse csv header") {
		t.Fatalf("header parse error = %v, want parse csv header", err)
	}
	if _, err := parseImportRows("name\n\"unterminated"); err == nil || !contains(err.Error(), "parse csv row 2") {
		t.Fatalf("row parse error = %v, want parse csv row 2", err)
	}

	rows, err := parseImportRows("name,email,notes\nOnly Name\n")
	if err != nil {
		t.Fatalf("parseImportRows returned error: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("len(rows) = %d, want 1", len(rows))
	}
	if rows[0].values["email"] != "" || rows[0].values["notes"] != "" {
		t.Fatalf("short record values = %+v, want blank missing fields", rows[0].values)
	}

	rows, err = parseImportRows("name,,email\nBlank Header,,blank-header@example.com\n,,\n")
	if err != nil {
		t.Fatalf("parseImportRows returned error: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("len(rows) = %d, want 1", len(rows))
	}
	if _, ok := rows[0].values[""]; ok {
		t.Fatalf("blank canonical header was not skipped: %+v", rows[0].values)
	}
	if rows[0].values["email"] != "blank-header@example.com" {
		t.Fatalf("email = %q, want blank-header@example.com", rows[0].values["email"])
	}

	if got := canonicalImportHeader("Custom/Header.Name"); got != "custom_header_name" {
		t.Fatalf("canonicalImportHeader fallback = %q, want custom_header_name", got)
	}
}

func TestBuildCreateRequestFromImportRowValidationBranches(t *testing.T) {
	tests := []struct {
		name    string
		values  map[string]string
		wantErr string
	}{
		{name: "invalid contact type", values: map[string]string{"name": "Bad Type", "contact_type": "partner"}, wantErr: "invalid contact_type"},
		{name: "negative payment terms", values: map[string]string{"name": "Bad Terms", "payment_terms_days": "-1"}, wantErr: "payment_terms_days must be zero or greater"},
		{name: "invalid country code", values: map[string]string{"name": "Bad Country", "country_code": "EST"}, wantErr: "country_code must be a 2-letter code"},
		{name: "invalid credit limit", values: map[string]string{"name": "Bad Credit", "credit_limit": "many"}, wantErr: "invalid credit_limit"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := buildCreateRequestFromImportRow(importRow{rowNumber: 2, values: tt.values})
			if err == nil {
				t.Fatal("buildCreateRequestFromImportRow error = nil, want error")
			}
			if !contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %q, want %q", err.Error(), tt.wantErr)
			}
		})
	}
}
