package contacts

import (
	"context"
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
				";Supply Partner;tarnija;supplier@example.com;30;2500.00\n",
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
