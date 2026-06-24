package recurring

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/HMB-research/open-accounting/internal/contacts"
	"github.com/HMB-research/open-accounting/internal/importrefs"
)

func TestRecurringImportServiceEdges(t *testing.T) {
	t.Run("rejects missing content before repository access", func(t *testing.T) {
		service := NewServiceWithDependencies(NewMockRepository(), nil, nil, nil, nil, nil)

		_, err := service.ImportCSV(context.Background(), "tenant-1", "test_schema", nil, nil, nil)

		require.ErrorContains(t, err, "csv_content is required")
	})

	t.Run("rejects files with only headers", func(t *testing.T) {
		service := NewServiceWithDependencies(NewMockRepository(), nil, nil, nil, nil, nil)

		_, err := service.ImportCSV(context.Background(), "tenant-1", "test_schema", nil, nil, &ImportRecurringInvoicesRequest{
			CSVContent: "name,contact_code,frequency,start_date,line_description,quantity,unit_price,vat_rate\n",
		})

		require.ErrorContains(t, err, "no recurring invoices found in CSV")
	})

	t.Run("wraps repository list errors", func(t *testing.T) {
		repo := NewMockRepository()
		repo.listErr = errors.New("database unavailable")
		service := NewServiceWithDependencies(repo, nil, nil, nil, nil, nil)

		_, err := service.ImportCSV(context.Background(), "tenant-1", "test_schema", nil, nil, &ImportRecurringInvoicesRequest{
			CSVContent: "name,contact_code,frequency,start_date,line_description,quantity,unit_price,vat_rate\n" +
				"Monthly Retainer,CUST-1,MONTHLY,2026-03-01,Consulting,1,100,22\n",
		})

		require.ErrorContains(t, err, "list existing recurring invoices")
	})

	t.Run("skips groups with merged header conflicts", func(t *testing.T) {
		repo := NewMockRepository()
		service := NewServiceWithDependencies(repo, nil, nil, nil, nil, nil)

		result, err := service.ImportCSV(context.Background(), "tenant-1", "test_schema", []contacts.Contact{{
			ID:   "contact-1",
			Code: "CUST-1",
		}}, nil, &ImportRecurringInvoicesRequest{
			CSVContent: "name,contact_code,frequency,start_date,currency,line_description,quantity,unit_price,vat_rate\n" +
				"Monthly Retainer,CUST-1,MONTHLY,2026-03-01,EUR,Consulting,1,100,22\n" +
				"Monthly Retainer,CUST-1,MONTHLY,2026-03-01,USD,Support,1,50,22\n",
		})

		require.NoError(t, err)
		assert.Zero(t, result.TemplatesCreated)
		assert.Equal(t, 2, result.RowsSkipped)
		require.Len(t, result.Errors, 1)
		assert.Contains(t, result.Errors[0].Message, "currency must be consistent")
		assert.Empty(t, repo.recurring)
	})

	t.Run("records repository create errors as skipped rows", func(t *testing.T) {
		repo := NewMockRepository()
		repo.createErr = errors.New("write failed")
		service := NewServiceWithDependencies(repo, nil, nil, nil, nil, nil)

		result, err := service.ImportCSV(context.Background(), "tenant-1", "test_schema", []contacts.Contact{{
			ID:   "contact-1",
			Code: "CUST-1",
		}}, nil, &ImportRecurringInvoicesRequest{
			CSVContent: "name,contact_code,frequency,start_date,line_description,quantity,unit_price,vat_rate\n" +
				"Monthly Retainer,CUST-1,MONTHLY,2026-03-01,Consulting,1,100,22\n",
		})

		require.NoError(t, err)
		assert.Zero(t, result.TemplatesCreated)
		assert.Equal(t, 1, result.RowsSkipped)
		require.Len(t, result.Errors, 1)
		assert.Contains(t, result.Errors[0].Message, "write failed")
	})

	t.Run("records line create errors as skipped rows", func(t *testing.T) {
		repo := NewMockRepository()
		repo.createLineErr = errors.New("line write failed")
		service := NewServiceWithDependencies(repo, nil, nil, nil, nil, nil)

		result, err := service.ImportCSV(context.Background(), "tenant-1", "test_schema", []contacts.Contact{{
			ID:   "contact-1",
			Code: "CUST-1",
		}}, nil, &ImportRecurringInvoicesRequest{
			CSVContent: "name,contact_code,frequency,start_date,line_description,quantity,unit_price,vat_rate\n" +
				"Monthly Retainer,CUST-1,MONTHLY,2026-03-01,Consulting,1,100,22\n",
		})

		require.NoError(t, err)
		assert.Zero(t, result.TemplatesCreated)
		assert.Equal(t, 1, result.RowsSkipped)
		require.Len(t, result.Errors, 1)
		assert.Contains(t, result.Errors[0].Message, "create recurring invoice line")
	})
}

func TestParseRecurringImportRowsEdges(t *testing.T) {
	_, err := parseRecurringImportRows(`"unterminated`)
	require.ErrorContains(t, err, "parse csv header")

	_, err = parseRecurringImportRows("name,contact_code,frequency,start_date,line_description,unit_price,vat_rate\n")
	require.ErrorContains(t, err, "missing required quantity column")

	_, err = parseRecurringImportRows("name,frequency,start_date,line_description,quantity,unit_price,vat_rate\n")
	require.ErrorContains(t, err, "missing contact identifier column")

	rows, err := parseRecurringImportRows("template;customer_code;frequency;start_date;description;qty;price;vat\n" +
		"\n" +
		"Monthly Retainer;CUST-1;MONTHLY;2026-03-01;Consulting;1;100;22\n")
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, 2, rows[0].rowNumber)
	assert.Equal(t, "Monthly Retainer", rows[0].values["name"])
	assert.Equal(t, "CUST-1", rows[0].values["contact_code"])
}

func TestParseRecurringImportDataRowEdges(t *testing.T) {
	validValues := map[string]string{
		"name":             "Monthly Retainer",
		"contact_code":     "CUST-1",
		"frequency":        "MONTHLY",
		"start_date":       "2026-03-01",
		"line_description": "Consulting",
		"quantity":         "1",
		"unit_price":       "100",
		"vat_rate":         "22",
	}

	tests := []struct {
		name        string
		mutate      func(map[string]string)
		wantMessage string
	}{
		{
			name:        "name required",
			mutate:      func(values map[string]string) { values["name"] = "" },
			wantMessage: "name is required",
		},
		{
			name:        "contact identifier required",
			mutate:      func(values map[string]string) { values["contact_code"] = "" },
			wantMessage: "a contact identifier is required",
		},
		{
			name:        "invalid frequency",
			mutate:      func(values map[string]string) { values["frequency"] = "EVERY_SO_OFTEN" },
			wantMessage: "invalid frequency",
		},
		{
			name:        "invalid start date",
			mutate:      func(values map[string]string) { values["start_date"] = "2026/03/01" },
			wantMessage: "start_date must use YYYY-MM-DD",
		},
		{
			name:        "invalid end date",
			mutate:      func(values map[string]string) { values["end_date"] = "2026/12/31" },
			wantMessage: "end_date must use YYYY-MM-DD",
		},
		{
			name:        "invalid next generation date",
			mutate:      func(values map[string]string) { values["next_generation_date"] = "2026/04/01" },
			wantMessage: "next_generation_date must use YYYY-MM-DD",
		},
		{
			name:        "invalid last generated date",
			mutate:      func(values map[string]string) { values["last_generated_at"] = "2026/02/01" },
			wantMessage: "last_generated_at must use YYYY-MM-DD",
		},
		{
			name:        "invalid payment terms",
			mutate:      func(values map[string]string) { values["payment_terms_days"] = "-1" },
			wantMessage: "payment_terms_days must be a non-negative integer",
		},
		{
			name:        "invalid generated count",
			mutate:      func(values map[string]string) { values["generated_count"] = "many" },
			wantMessage: "generated_count must be a non-negative integer",
		},
		{
			name:        "invalid active flag",
			mutate:      func(values map[string]string) { values["is_active"] = "maybe" },
			wantMessage: "is_active must be true or false",
		},
		{
			name:        "invalid send email flag",
			mutate:      func(values map[string]string) { values["send_email_on_generation"] = "maybe" },
			wantMessage: "send_email_on_generation must be true or false",
		},
		{
			name:        "invalid attach PDF flag",
			mutate:      func(values map[string]string) { values["attach_pdf_to_email"] = "maybe" },
			wantMessage: "attach_pdf_to_email must be true or false",
		},
		{
			name:        "line description required",
			mutate:      func(values map[string]string) { values["line_description"] = "" },
			wantMessage: "line_description is required",
		},
		{
			name:        "negative unit price",
			mutate:      func(values map[string]string) { values["unit_price"] = "-1" },
			wantMessage: "unit_price cannot be negative",
		},
		{
			name:        "invalid discount",
			mutate:      func(values map[string]string) { values["discount_percent"] = "bad" },
			wantMessage: "invalid discount_percent",
		},
		{
			name:        "discount over limit",
			mutate:      func(values map[string]string) { values["discount_percent"] = "101" },
			wantMessage: "discount_percent must be between 0 and 100",
		},
		{
			name:        "negative VAT rate",
			mutate:      func(values map[string]string) { values["vat_rate"] = "-1" },
			wantMessage: "vat_rate cannot be negative",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			values := make(map[string]string, len(validValues))
			for key, value := range validValues {
				values[key] = value
			}
			tt.mutate(values)

			_, err := parseRecurringImportDataRow(recurringImportRow{rowNumber: 2, values: values}, importrefs.NewProductLookup(nil))

			require.ErrorContains(t, err, tt.wantMessage)
		})
	}
}

func TestRecurringImportParserHelpers(t *testing.T) {
	frequency, err := parseRecurringImportFrequency("weekly")
	require.NoError(t, err)
	assert.Equal(t, FrequencyWeekly, frequency)

	_, err = parseRecurringImportFrequency("every month")
	require.ErrorContains(t, err, "invalid frequency")

	_, err = parseRecurringImportDate("2026/03/01", "start_date")
	require.ErrorContains(t, err, "start_date must use YYYY-MM-DD")

	_, err = parseRecurringImportDecimal("not-a-number", "quantity")
	require.ErrorContains(t, err, "invalid quantity")

	count, err := parseRecurringImportOptionalNonNegativeInt("generated_count", "", 3)
	require.NoError(t, err)
	assert.Equal(t, 3, count)

	_, err = parseRecurringImportOptionalNonNegativeInt("generated_count", "-1", 0)
	require.ErrorContains(t, err, "generated_count must be a non-negative integer")

	flag, err := parseRecurringImportOptionalBool("is_active", "", true)
	require.NoError(t, err)
	assert.True(t, flag)

	flag, err = parseRecurringImportOptionalBool("is_active", "no", true)
	require.NoError(t, err)
	assert.False(t, flag)

	_, err = parseRecurringImportOptionalBool("is_active", "maybe", true)
	require.ErrorContains(t, err, "is_active must be true or false")

	assert.Equal(t, "next_generation_date", canonicalRecurringImportHeader(" next_date "))
	assert.Empty(t, canonicalRecurringImportHeader("legacy_only"))
	assert.Equal(t, '\t', detectRecurringImportDelimiter("a\tb\n1\t2"))
	assert.Equal(t, ';', detectRecurringImportDelimiter("a;b\n1;2"))
	assert.Equal(t, ',', detectRecurringImportDelimiter("a,b\n1,2"))
}

func TestMergeRecurringImportGroupEdges(t *testing.T) {
	startDate := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	nextDate := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	endDate := time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC)
	lastGenerated := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	baseHeader := recurringImportHeader{
		name:                   "Monthly Retainer",
		contactRef:             recurringImportContactRef{code: "CUST-1"},
		invoiceType:            "SALES",
		currency:               "EUR",
		frequency:              FrequencyMonthly,
		startDate:              startDate,
		nextGenerationDate:     nextDate,
		paymentTermsDays:       14,
		isActive:               true,
		sendEmailOnGeneration:  false,
		emailTemplateType:      "INVOICE_SEND",
		attachPDFToEmail:       true,
		recipientEmailOverride: "",
	}

	t.Run("fills optional header values", func(t *testing.T) {
		group := &recurringImportGroup{header: baseHeader}
		next := baseHeader
		next.endDate = &endDate
		next.lastGeneratedAt = &lastGenerated
		next.reference = "PO-1"
		next.notes = "Imported note"
		next.recipientEmailOverride = "billing@example.com"
		next.emailSubjectOverride = "Invoice"
		next.emailMessage = "Please pay"

		conflict := mergeRecurringImportGroup(group, next)

		assert.Empty(t, conflict)
		require.NotNil(t, group.header.endDate)
		assert.True(t, group.header.endDate.Equal(endDate))
		require.NotNil(t, group.header.lastGeneratedAt)
		assert.True(t, group.header.lastGeneratedAt.Equal(lastGenerated))
		assert.Equal(t, "PO-1", group.header.reference)
		assert.Equal(t, "Imported note", group.header.notes)
		assert.Equal(t, "billing@example.com", group.header.recipientEmailOverride)
		assert.Equal(t, "Invoice", group.header.emailSubjectOverride)
		assert.Equal(t, "Please pay", group.header.emailMessage)
	})

	tests := []struct {
		name         string
		mutateGroup  func(*recurringImportHeader)
		mutateNext   func(*recurringImportHeader)
		wantConflict string
	}{
		{
			name:         "contact code mismatch",
			mutateNext:   func(header *recurringImportHeader) { header.contactRef.code = "CUST-2" },
			wantConflict: "contact_code must be consistent",
		},
		{
			name:         "invoice type mismatch",
			mutateNext:   func(header *recurringImportHeader) { header.invoiceType = "PURCHASE" },
			wantConflict: "invoice_type must be consistent",
		},
		{
			name:         "currency mismatch",
			mutateNext:   func(header *recurringImportHeader) { header.currency = "USD" },
			wantConflict: "currency must be consistent",
		},
		{
			name:         "frequency mismatch",
			mutateNext:   func(header *recurringImportHeader) { header.frequency = FrequencyWeekly },
			wantConflict: "frequency must be consistent",
		},
		{
			name:         "start date mismatch",
			mutateNext:   func(header *recurringImportHeader) { header.startDate = startDate.AddDate(0, 0, 1) },
			wantConflict: "start_date must be consistent",
		},
		{
			name:         "end date mismatch",
			mutateGroup:  func(header *recurringImportHeader) { header.endDate = &endDate },
			mutateNext:   func(header *recurringImportHeader) { other := endDate.AddDate(0, 0, 1); header.endDate = &other },
			wantConflict: "end_date must be consistent",
		},
		{
			name:         "next generation date mismatch",
			mutateNext:   func(header *recurringImportHeader) { header.nextGenerationDate = nextDate.AddDate(0, 0, 1) },
			wantConflict: "next_generation_date must be consistent",
		},
		{
			name:        "last generated date mismatch",
			mutateGroup: func(header *recurringImportHeader) { header.lastGeneratedAt = &lastGenerated },
			mutateNext: func(header *recurringImportHeader) {
				other := lastGenerated.AddDate(0, 0, 1)
				header.lastGeneratedAt = &other
			},
			wantConflict: "last_generated_at must be consistent",
		},
		{
			name:         "reference mismatch",
			mutateGroup:  func(header *recurringImportHeader) { header.reference = "PO-1" },
			mutateNext:   func(header *recurringImportHeader) { header.reference = "PO-2" },
			wantConflict: "reference must be consistent",
		},
		{
			name:         "payment terms mismatch",
			mutateNext:   func(header *recurringImportHeader) { header.paymentTermsDays = 30 },
			wantConflict: "payment_terms_days must be consistent",
		},
		{
			name:         "active flag mismatch",
			mutateNext:   func(header *recurringImportHeader) { header.isActive = false },
			wantConflict: "is_active must be consistent",
		},
		{
			name:         "generated count mismatch",
			mutateNext:   func(header *recurringImportHeader) { header.generatedCount = 1 },
			wantConflict: "generated_count must be consistent",
		},
		{
			name:         "send email mismatch",
			mutateNext:   func(header *recurringImportHeader) { header.sendEmailOnGeneration = true },
			wantConflict: "send_email_on_generation must be consistent",
		},
		{
			name:         "attach PDF mismatch",
			mutateNext:   func(header *recurringImportHeader) { header.attachPDFToEmail = false },
			wantConflict: "attach_pdf_to_email must be consistent",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			groupHeader := baseHeader
			next := baseHeader
			if tt.mutateGroup != nil {
				tt.mutateGroup(&groupHeader)
			}
			if tt.mutateNext != nil {
				tt.mutateNext(&next)
			}
			group := &recurringImportGroup{header: groupHeader}

			conflict := mergeRecurringImportGroup(group, next)

			assert.Contains(t, conflict, tt.wantConflict)
		})
	}
}

func TestMergeRecurringImportContactRefConflicts(t *testing.T) {
	tests := []struct {
		name         string
		target       recurringImportContactRef
		next         recurringImportContactRef
		wantConflict string
	}{
		{
			name:         "contact id",
			target:       recurringImportContactRef{id: "contact-1"},
			next:         recurringImportContactRef{id: "contact-2"},
			wantConflict: "contact_id must be consistent",
		},
		{
			name:         "registry code",
			target:       recurringImportContactRef{regCode: "100"},
			next:         recurringImportContactRef{regCode: "200"},
			wantConflict: "contact_reg_code must be consistent",
		},
		{
			name:         "VAT number",
			target:       recurringImportContactRef{vatNumber: "EE100"},
			next:         recurringImportContactRef{vatNumber: "EE200"},
			wantConflict: "contact_vat_number must be consistent",
		},
		{
			name:         "email",
			target:       recurringImportContactRef{email: "a@example.com"},
			next:         recurringImportContactRef{email: "b@example.com"},
			wantConflict: "contact_email must be consistent",
		},
		{
			name:         "name",
			target:       recurringImportContactRef{name: "Alpha"},
			next:         recurringImportContactRef{name: "Beta"},
			wantConflict: "contact_name must be consistent",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conflict := mergeRecurringImportContactRef(&tt.target, tt.next)

			assert.Contains(t, conflict, tt.wantConflict)
		})
	}
}

func TestRecurringImportContactLookupEdges(t *testing.T) {
	lookup := buildRecurringImportContactLookup([]contacts.Contact{
		{
			ID:        "by-code",
			Code:      "CUST-1",
			Name:      "Code Customer",
			RegCode:   "100",
			VATNumber: "EE100",
			Email:     "code@example.com",
		},
		{
			ID:    "by-email",
			Email: "ops@example.com",
		},
		{
			ID:   "by-name",
			Name: "Acme OU",
		},
	})

	contact, err := lookup.find(recurringImportContactRef{id: " by-code "})
	require.NoError(t, err)
	assert.Equal(t, "by-code", contact.ID)

	contact, err = lookup.find(recurringImportContactRef{email: " OPS@example.com "})
	require.NoError(t, err)
	assert.Equal(t, "by-email", contact.ID)

	contact, err = lookup.find(recurringImportContactRef{name: " acme ou "})
	require.NoError(t, err)
	assert.Equal(t, "by-name", contact.ID)

	_, err = lookup.find(recurringImportContactRef{code: "missing"})
	require.ErrorContains(t, err, `contact_code "missing" was not found`)

	_, err = lookup.find(recurringImportContactRef{regCode: "404"})
	require.ErrorContains(t, err, `contact_reg_code "404" was not found`)

	_, err = lookup.find(recurringImportContactRef{vatNumber: "EE404"})
	require.ErrorContains(t, err, `contact_vat_number "EE404" was not found`)

	_, err = lookup.find(recurringImportContactRef{email: "missing@example.com"})
	require.ErrorContains(t, err, `contact_email "missing@example.com" was not found`)

	_, err = lookup.find(recurringImportContactRef{name: "Missing OU"})
	require.ErrorContains(t, err, `contact_name "Missing OU" was not found`)

	_, err = lookup.find(recurringImportContactRef{})
	require.ErrorContains(t, err, "a contact identifier is required")
}

func TestBuildImportedRecurringInvoiceValidationError(t *testing.T) {
	group := &recurringImportGroup{
		header: recurringImportHeader{
			name:               "No Lines",
			invoiceType:        "SALES",
			currency:           "EUR",
			frequency:          FrequencyMonthly,
			startDate:          time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
			nextGenerationDate: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
			paymentTermsDays:   14,
			isActive:           true,
			attachPDFToEmail:   true,
		},
	}

	_, err := buildImportedRecurringInvoice("tenant-1", "user-1", "contact-1", group)

	require.ErrorContains(t, err, "validation failed")
}
