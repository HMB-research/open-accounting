package orders

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/HMB-research/open-accounting/internal/contacts"
	"github.com/HMB-research/open-accounting/internal/importrefs"
)

func TestOrderImportServiceEdges(t *testing.T) {
	t.Run("rejects missing content before repository access", func(t *testing.T) {
		svc := NewServiceWithRepository(NewMockRepository())

		_, err := svc.ImportCSV(context.Background(), "tenant-1", "test_schema", nil, nil, nil)

		require.ErrorContains(t, err, "csv_content is required")
	})

	t.Run("rejects files with only headers", func(t *testing.T) {
		svc := NewServiceWithRepository(NewMockRepository())

		_, err := svc.ImportCSV(context.Background(), "tenant-1", "test_schema", nil, nil, &ImportOrdersRequest{
			CSVContent: "order_number,contact_code,order_date,line_description,quantity,unit_price,vat_rate\n",
		})

		require.ErrorContains(t, err, "no orders found in CSV")
	})

	t.Run("wraps repository list errors", func(t *testing.T) {
		repo := NewMockRepository()
		repo.ListErr = errors.New("database unavailable")
		svc := NewServiceWithRepository(repo)

		_, err := svc.ImportCSV(context.Background(), "tenant-1", "test_schema", nil, nil, &ImportOrdersRequest{
			CSVContent: "order_number,contact_code,order_date,line_description,quantity,unit_price,vat_rate\n" +
				"ORD-1,CUST-1,2026-03-15,Consulting,1,100,22\n",
		})

		require.ErrorContains(t, err, "list existing orders")
	})

	t.Run("skips groups with merged header conflicts", func(t *testing.T) {
		repo := NewMockRepository()
		svc := NewServiceWithRepository(repo)

		result, err := svc.ImportCSV(context.Background(), "tenant-1", "test_schema", []contacts.Contact{{
			ID:   "contact-1",
			Code: "CUST-1",
		}}, nil, &ImportOrdersRequest{
			CSVContent: "order_number,contact_code,order_date,currency,line_description,quantity,unit_price,vat_rate\n" +
				"ORD-1,CUST-1,2026-03-15,EUR,Consulting,1,100,22\n" +
				"ORD-1,CUST-1,2026-03-15,USD,Support,1,50,22\n",
		})

		require.NoError(t, err)
		assert.Zero(t, result.OrdersCreated)
		assert.Equal(t, 2, result.RowsSkipped)
		require.Len(t, result.Errors, 1)
		assert.Contains(t, result.Errors[0].Message, "currency must be consistent")
		assert.Empty(t, repo.Orders)
	})

	t.Run("records repository create errors as skipped rows", func(t *testing.T) {
		repo := NewMockRepository()
		repo.CreateErr = errors.New("write failed")
		svc := NewServiceWithRepository(repo)

		result, err := svc.ImportCSV(context.Background(), "tenant-1", "test_schema", []contacts.Contact{{
			ID:   "contact-1",
			Code: "CUST-1",
		}}, nil, &ImportOrdersRequest{
			CSVContent: "order_number,contact_code,order_date,line_description,quantity,unit_price,vat_rate\n" +
				"ORD-1,CUST-1,2026-03-15,Consulting,1,100,22\n",
		})

		require.NoError(t, err)
		assert.Zero(t, result.OrdersCreated)
		assert.Equal(t, 1, result.RowsSkipped)
		require.Len(t, result.Errors, 1)
		assert.Contains(t, result.Errors[0].Message, "write failed")
	})
}

func TestParseOrderImportRowsEdges(t *testing.T) {
	_, err := parseOrderImportRows(`"unterminated`)
	require.ErrorContains(t, err, "parse csv header")

	_, err = parseOrderImportRows("order_number,contact_code,order_date,line_description,unit_price,vat_rate\n")
	require.ErrorContains(t, err, "missing required quantity column")

	_, err = parseOrderImportRows("order_number,order_date,line_description,quantity,unit_price,vat_rate\n")
	require.ErrorContains(t, err, "missing contact identifier column")

	rows, err := parseOrderImportRows("order_number;customer_code;date;description;qty;price;vat\n" +
		"\n" +
		"ORD-1;CUST-1;2026-03-15;Consulting;1;100;22\n")
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, 2, rows[0].rowNumber)
	assert.Equal(t, "ORD-1", rows[0].values["order_number"])
	assert.Equal(t, "CUST-1", rows[0].values["contact_code"])
}

func TestParseOrderImportDataRowEdges(t *testing.T) {
	validValues := map[string]string{
		"order_number":     "ORD-1",
		"contact_code":     "CUST-1",
		"order_date":       "2026-03-15",
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
			name:        "order number required",
			mutate:      func(values map[string]string) { values["order_number"] = "" },
			wantMessage: "order_number is required",
		},
		{
			name:        "contact identifier required",
			mutate:      func(values map[string]string) { values["contact_code"] = "" },
			wantMessage: "a contact identifier is required",
		},
		{
			name:        "invalid expected delivery",
			mutate:      func(values map[string]string) { values["expected_delivery"] = "2026/03/20" },
			wantMessage: "expected_delivery must use YYYY-MM-DD",
		},
		{
			name:        "invalid exchange rate",
			mutate:      func(values map[string]string) { values["exchange_rate"] = "bad" },
			wantMessage: "invalid exchange_rate",
		},
		{
			name:        "zero exchange rate",
			mutate:      func(values map[string]string) { values["exchange_rate"] = "0" },
			wantMessage: "exchange_rate must be greater than zero",
		},
		{
			name:        "invalid status",
			mutate:      func(values map[string]string) { values["status"] = "waiting" },
			wantMessage: "invalid status",
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

			_, err := parseOrderImportDataRow(orderImportRow{rowNumber: 2, values: values}, importrefs.NewProductLookup(nil))

			require.ErrorContains(t, err, tt.wantMessage)
		})
	}
}

func TestOrderImportParserHelpers(t *testing.T) {
	status, err := parseOrderImportStatus("")
	require.NoError(t, err)
	assert.Empty(t, status)

	status, err = parseOrderImportStatus("open")
	require.NoError(t, err)
	assert.Equal(t, OrderStatusPending, status)

	status, err = parseOrderImportStatus("PROCESSING")
	require.NoError(t, err)
	assert.Equal(t, OrderStatusProcessing, status)

	_, err = parseOrderImportStatus("waiting")
	require.ErrorContains(t, err, "invalid status")

	_, err = parseOrderImportDate("2026/03/01", "order_date")
	require.ErrorContains(t, err, "order_date must use YYYY-MM-DD")

	_, err = parseOrderImportDecimal("not-a-number", "quantity")
	require.ErrorContains(t, err, "invalid quantity")

	assert.Equal(t, "order_number", canonicalOrderImportHeader(" order_no. "))
	assert.Empty(t, canonicalOrderImportHeader("legacy_only"))
	assert.Equal(t, '\t', detectOrderImportDelimiter("a\tb\n1\t2"))
	assert.Equal(t, ';', detectOrderImportDelimiter("a;b\n1;2"))
	assert.Equal(t, ',', detectOrderImportDelimiter("a,b\n1,2"))
}

func TestMergeOrderImportGroupEdges(t *testing.T) {
	orderDate := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	deliveryDate := time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC)
	quoteID := "11111111-1111-4111-8111-111111111111"
	baseHeader := orderImportHeader{
		orderNumber:  "ORD-1",
		contactRef:   orderImportContactRef{code: "CUST-1"},
		orderDate:    orderDate,
		currency:     "EUR",
		exchangeRate: decimal.NewFromInt(1),
	}

	t.Run("fills optional header values", func(t *testing.T) {
		group := &orderImportGroup{header: baseHeader}
		next := baseHeader
		next.expectedDelivery = &deliveryDate
		next.notes = "Imported note"
		next.quoteID = &quoteID
		next.quoteNumber = "QT-1"
		next.explicitStatus = OrderStatusConfirmed

		conflict := mergeOrderImportGroup(group, next, 7)

		assert.Empty(t, conflict)
		require.NotNil(t, group.header.expectedDelivery)
		assert.True(t, group.header.expectedDelivery.Equal(deliveryDate))
		assert.Equal(t, "Imported note", group.header.notes)
		require.NotNil(t, group.header.quoteID)
		assert.Equal(t, quoteID, *group.header.quoteID)
		assert.Equal(t, "QT-1", group.header.quoteNumber)
		assert.Equal(t, OrderStatusConfirmed, group.header.explicitStatus)
	})

	tests := []struct {
		name         string
		mutateGroup  func(*orderImportHeader)
		mutateNext   func(*orderImportHeader)
		wantConflict string
	}{
		{
			name:         "order date mismatch",
			mutateNext:   func(header *orderImportHeader) { header.orderDate = orderDate.AddDate(0, 0, 1) },
			wantConflict: "order_date must be consistent",
		},
		{
			name:        "expected delivery mismatch",
			mutateGroup: func(header *orderImportHeader) { header.expectedDelivery = &deliveryDate },
			mutateNext: func(header *orderImportHeader) {
				nextDate := deliveryDate.AddDate(0, 0, 1)
				header.expectedDelivery = &nextDate
			},
			wantConflict: "expected_delivery must be consistent",
		},
		{
			name:         "currency mismatch",
			mutateNext:   func(header *orderImportHeader) { header.currency = "USD" },
			wantConflict: "currency must be consistent",
		},
		{
			name:         "exchange rate mismatch",
			mutateNext:   func(header *orderImportHeader) { header.exchangeRate = decimal.RequireFromString("1.1") },
			wantConflict: "exchange_rate must be consistent",
		},
		{
			name:         "contact code mismatch",
			mutateNext:   func(header *orderImportHeader) { header.contactRef.code = "CUST-2" },
			wantConflict: "contact_code must be consistent",
		},
		{
			name:         "notes mismatch",
			mutateGroup:  func(header *orderImportHeader) { header.notes = "note 1" },
			mutateNext:   func(header *orderImportHeader) { header.notes = "note 2" },
			wantConflict: "notes must be consistent",
		},
		{
			name:        "quote id mismatch",
			mutateGroup: func(header *orderImportHeader) { header.quoteID = &quoteID },
			mutateNext: func(header *orderImportHeader) {
				nextQuoteID := "22222222-2222-4222-8222-222222222222"
				header.quoteID = &nextQuoteID
			},
			wantConflict: "quote_id must be consistent",
		},
		{
			name:         "quote number mismatch",
			mutateGroup:  func(header *orderImportHeader) { header.quoteNumber = "QT-1" },
			mutateNext:   func(header *orderImportHeader) { header.quoteNumber = "QT-2" },
			wantConflict: "quote_number must be consistent",
		},
		{
			name:         "status mismatch",
			mutateGroup:  func(header *orderImportHeader) { header.explicitStatus = OrderStatusConfirmed },
			mutateNext:   func(header *orderImportHeader) { header.explicitStatus = OrderStatusShipped },
			wantConflict: "status must be consistent for each order_number (row 7)",
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
			group := &orderImportGroup{header: groupHeader}

			conflict := mergeOrderImportGroup(group, next, 7)

			assert.Contains(t, conflict, tt.wantConflict)
		})
	}
}

func TestMergeOrderImportContactRefConflicts(t *testing.T) {
	tests := []struct {
		name         string
		target       orderImportContactRef
		next         orderImportContactRef
		wantConflict string
	}{
		{
			name:         "contact id",
			target:       orderImportContactRef{id: "contact-1"},
			next:         orderImportContactRef{id: "contact-2"},
			wantConflict: "contact_id must be consistent",
		},
		{
			name:         "registry code",
			target:       orderImportContactRef{regCode: "100"},
			next:         orderImportContactRef{regCode: "200"},
			wantConflict: "contact_reg_code must be consistent",
		},
		{
			name:         "VAT number",
			target:       orderImportContactRef{vatNumber: "EE100"},
			next:         orderImportContactRef{vatNumber: "EE200"},
			wantConflict: "contact_vat_number must be consistent",
		},
		{
			name:         "email",
			target:       orderImportContactRef{email: "a@example.com"},
			next:         orderImportContactRef{email: "b@example.com"},
			wantConflict: "contact_email must be consistent",
		},
		{
			name:         "name",
			target:       orderImportContactRef{name: "Alpha"},
			next:         orderImportContactRef{name: "Beta"},
			wantConflict: "contact_name must be consistent",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conflict := mergeOrderImportContactRef(&tt.target, tt.next)

			assert.Contains(t, conflict, tt.wantConflict)
		})
	}
}

func TestOrderImportContactLookupEdges(t *testing.T) {
	lookup := buildOrderImportContactLookup([]contacts.Contact{
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

	contact, err := lookup.find(orderImportContactRef{id: " by-code "})
	require.NoError(t, err)
	assert.Equal(t, "by-code", contact.ID)

	contact, err = lookup.find(orderImportContactRef{email: " OPS@example.com "})
	require.NoError(t, err)
	assert.Equal(t, "by-email", contact.ID)

	contact, err = lookup.find(orderImportContactRef{name: " acme ou "})
	require.NoError(t, err)
	assert.Equal(t, "by-name", contact.ID)

	_, err = lookup.find(orderImportContactRef{code: "missing"})
	require.ErrorContains(t, err, `contact_code "missing" was not found`)

	_, err = lookup.find(orderImportContactRef{regCode: "404"})
	require.ErrorContains(t, err, `contact_reg_code "404" was not found`)

	_, err = lookup.find(orderImportContactRef{vatNumber: "EE404"})
	require.ErrorContains(t, err, `contact_vat_number "EE404" was not found`)

	_, err = lookup.find(orderImportContactRef{email: "missing@example.com"})
	require.ErrorContains(t, err, `contact_email "missing@example.com" was not found`)

	_, err = lookup.find(orderImportContactRef{name: "Missing OU"})
	require.ErrorContains(t, err, `contact_name "Missing OU" was not found`)

	_, err = lookup.find(orderImportContactRef{})
	require.ErrorContains(t, err, "a contact identifier is required")
}

func TestOrderImportQuoteLookupEdges(t *testing.T) {
	lookup := buildOrderImportQuoteLookup([]ImportQuoteReference{
		{ID: "", QuoteNumber: "QT-SKIPPED"},
		{ID: "quote-1", QuoteNumber: "QT-1"},
		{ID: "quote-2", QuoteNumber: "QT-DUP"},
		{ID: "quote-3", QuoteNumber: "qt-dup"},
	})

	header := orderImportHeader{quoteNumber: " qt-1 "}
	require.NoError(t, lookup.resolve(&header))
	require.NotNil(t, header.quoteID)
	assert.Equal(t, "quote-1", *header.quoteID)

	header = orderImportHeader{quoteNumber: "QT-DUP"}
	err := lookup.resolve(&header)
	require.ErrorContains(t, err, `quote_number "QT-DUP" is ambiguous`)

	header = orderImportHeader{quoteNumber: "QT-SKIPPED"}
	err = lookup.resolve(&header)
	require.ErrorContains(t, err, `quote_number "QT-SKIPPED" was not found`)

	explicitID := "explicit-quote"
	header = orderImportHeader{quoteID: &explicitID, quoteNumber: "QT-DUP"}
	require.NoError(t, lookup.resolve(&header))
	assert.Equal(t, explicitID, *header.quoteID)
}

func TestBuildImportedOrderValidationError(t *testing.T) {
	group := &orderImportGroup{
		header: orderImportHeader{
			orderNumber:  "ORD-NO-LINES",
			orderDate:    time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
			currency:     "EUR",
			exchangeRate: decimal.NewFromInt(1),
		},
	}

	_, err := buildImportedOrder("tenant-1", "user-1", "contact-1", group)

	require.ErrorContains(t, err, "validation failed")
}

func TestOrderImportServiceAdditionalBranches(t *testing.T) {
	t.Run("skips duplicate existing order number", func(t *testing.T) {
		repo := NewMockRepository()
		repo.Orders["existing"] = &Order{
			ID:          "existing",
			TenantID:    "tenant-1",
			OrderNumber: "ORD-1",
		}
		svc := NewServiceWithRepository(repo)

		result, err := svc.ImportCSV(context.Background(), "tenant-1", "test_schema", []contacts.Contact{{
			ID:   "contact-1",
			Code: "CUST-1",
		}}, nil, &ImportOrdersRequest{
			CSVContent: "order_number,contact_code,order_date,line_description,quantity,unit_price,vat_rate\n" +
				"ORD-1,CUST-1,2026-03-15,Consulting,1,100,22\n",
		})

		require.NoError(t, err)
		assert.Zero(t, result.OrdersCreated)
		assert.Equal(t, 1, result.RowsSkipped)
		require.Len(t, result.Errors, 1)
		assert.Contains(t, result.Errors[0].Message, `order_number "ORD-1" already exists`)
	})

	t.Run("skips orders when quote number cannot be resolved", func(t *testing.T) {
		repo := NewMockRepository()
		svc := NewServiceWithRepository(repo)

		result, err := svc.ImportCSVWithQuoteReferences(context.Background(), "tenant-1", "test_schema", []contacts.Contact{{
			ID:   "contact-1",
			Code: "CUST-1",
		}}, nil, nil, &ImportOrdersRequest{
			CSVContent: "order_number,contact_code,order_date,quote_number,line_description,quantity,unit_price,vat_rate\n" +
				"ORD-1,CUST-1,2026-03-15,QT-MISSING,Consulting,1,100,22\n",
		})

		require.NoError(t, err)
		assert.Zero(t, result.OrdersCreated)
		assert.Equal(t, 1, result.RowsSkipped)
		require.Len(t, result.Errors, 1)
		assert.Contains(t, result.Errors[0].Message, `quote_number "QT-MISSING" was not found`)
	})

	t.Run("skips orders when quote number is ambiguous", func(t *testing.T) {
		repo := NewMockRepository()
		svc := NewServiceWithRepository(repo)

		result, err := svc.ImportCSVWithQuoteReferences(context.Background(), "tenant-1", "test_schema", []contacts.Contact{{
			ID:   "contact-1",
			Code: "CUST-1",
		}}, nil, []ImportQuoteReference{
			{ID: "quote-1", QuoteNumber: "QT-1"},
			{ID: "quote-2", QuoteNumber: "qt-1"},
		}, &ImportOrdersRequest{
			CSVContent: "order_number,contact_code,order_date,quote_number,line_description,quantity,unit_price,vat_rate\n" +
				"ORD-1,CUST-1,2026-03-15,QT-1,Consulting,1,100,22\n",
		})

		require.NoError(t, err)
		assert.Zero(t, result.OrdersCreated)
		assert.Equal(t, 1, result.RowsSkipped)
		require.Len(t, result.Errors, 1)
		assert.Contains(t, result.Errors[0].Message, `quote_number "QT-1" is ambiguous`)
	})

	t.Run("skips order when resolved contact has no id", func(t *testing.T) {
		repo := NewMockRepository()
		svc := NewServiceWithRepository(repo)

		result, err := svc.ImportCSV(context.Background(), "tenant-1", "test_schema", []contacts.Contact{{
			Code: "CUST-1",
			Name: "Nameless Customer",
		}}, nil, &ImportOrdersRequest{
			CSVContent: "order_number,contact_code,order_date,line_description,quantity,unit_price,vat_rate\n" +
				"ORD-1,CUST-1,2026-03-15,Consulting,1,100,22\n",
		})

		require.NoError(t, err)
		assert.Zero(t, result.OrdersCreated)
		assert.Equal(t, 1, result.RowsSkipped)
		require.Len(t, result.Errors, 1)
		assert.Contains(t, result.Errors[0].Message, "validation failed")
	})
}

func TestOrderImportParserAdditionalBranches(t *testing.T) {
	_, err := parseOrderImportRows(`"unterminated`)
	require.ErrorContains(t, err, "parse csv header")

	_, err = parseOrderImportRows("order_number,contact_code,order_date,line_description,quantity,unit_price,vat_rate\nORD-1,CUST-1,2026-03-15,\"Consulting,1,100,22\n")
	require.ErrorContains(t, err, "parse csv row 2")

	validValues := map[string]string{
		"order_number":     "ORD-1",
		"contact_code":     "CUST-1",
		"order_date":       "2026-03-15",
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
			name:        "invalid contact id",
			mutate:      func(values map[string]string) { values["contact_id"] = "not-a-uuid"; values["contact_code"] = "" },
			wantMessage: "contact_id must be a valid UUID",
		},
		{
			name:        "invalid quote id",
			mutate:      func(values map[string]string) { values["quote_id"] = "not-a-uuid" },
			wantMessage: "quote_id must be a valid UUID",
		},
		{
			name:        "zero quantity",
			mutate:      func(values map[string]string) { values["quantity"] = "0" },
			wantMessage: "quantity must be greater than zero",
		},
		{
			name:        "missing product code",
			mutate:      func(values map[string]string) { values["product_code"] = "MISSING" },
			wantMessage: "product_code",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			values := make(map[string]string, len(validValues)+1)
			for key, value := range validValues {
				values[key] = value
			}
			tt.mutate(values)

			_, err := parseOrderImportDataRow(orderImportRow{rowNumber: 2, values: values}, importrefs.NewProductLookup(nil))

			require.ErrorContains(t, err, tt.wantMessage)
		})
	}

	status, err := parseOrderImportStatus("CANCELED")
	require.NoError(t, err)
	assert.Equal(t, OrderStatusCanceled, status)
}
