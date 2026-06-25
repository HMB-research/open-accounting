package invoicing

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"github.com/HMB-research/open-accounting/internal/contacts"
	einvoicemapper "github.com/HMB-research/open-accounting/internal/invoicing/mappers/einvoice"
)

var buildEInvoiceImportedInvoice = buildImportedInvoice

// ImportEInvoiceXML imports invoices from Estonian e-invoice XML.
func (s *Service) ImportEInvoiceXML(
	ctx context.Context,
	tenantID, schemaName string,
	existingContacts []contacts.Contact,
	req *ImportEInvoiceRequest,
	validateDate func(time.Time) error,
) (*ImportInvoicesResult, error) {
	if strings.TrimSpace(req.XMLContent) == "" {
		return nil, fmt.Errorf("xml_content is required")
	}

	mappedInvoices, err := einvoicemapper.Parse(req.XMLContent)
	if err != nil {
		return nil, err
	}

	existingInvoices, err := s.repo.List(ctx, schemaName, tenantID, nil)
	if err != nil {
		return nil, fmt.Errorf("list existing invoices: %w", err)
	}

	existingKeys := make(map[string]struct{}, len(existingInvoices))
	for _, invoice := range existingInvoices {
		existingKeys[normalizedInvoiceImportGroupKey(invoice.InvoiceNumber, invoice.InvoiceType)] = struct{}{}
	}

	result := &ImportInvoicesResult{
		FileName: req.FileName,
		Errors:   []ImportInvoicesRowError{},
	}
	contactLookup := buildInvoiceImportContactLookup(existingContacts)
	now := normalizeInvoiceImportDate(time.Now())

	for index, mapped := range mappedInvoices {
		result.RowsProcessed++
		rowNumber := index + 1

		group, err := mappedEInvoiceToImportGroup(mapped, req.InvoiceType)
		if err != nil {
			result.RowsSkipped++
			result.Errors = append(result.Errors, ImportInvoicesRowError{
				Row:           rowNumber,
				InvoiceNumber: mapped.Number,
				Message:       err.Error(),
			})
			continue
		}

		key := normalizedInvoiceImportGroupKey(group.header.invoiceNumber, group.header.invoiceType)
		if _, exists := existingKeys[key]; exists {
			result.RowsSkipped++
			result.Errors = append(result.Errors, ImportInvoicesRowError{
				Row:           rowNumber,
				InvoiceNumber: group.header.invoiceNumber,
				Message:       fmt.Sprintf("invoice_number %q already exists for invoice_type %s", group.header.invoiceNumber, group.header.invoiceType),
			})
			continue
		}

		contact, err := contactLookup.find(group.header.contactRef)
		if err != nil {
			result.RowsSkipped++
			result.Errors = append(result.Errors, ImportInvoicesRowError{
				Row:           rowNumber,
				InvoiceNumber: group.header.invoiceNumber,
				Message:       err.Error(),
			})
			continue
		}

		if validateDate != nil {
			if err := validateDate(group.header.issueDate); err != nil {
				result.RowsSkipped++
				result.Errors = append(result.Errors, ImportInvoicesRowError{
					Row:           rowNumber,
					InvoiceNumber: group.header.invoiceNumber,
					Message:       err.Error(),
				})
				continue
			}
		}

		invoice, err := buildEInvoiceImportedInvoice(tenantID, req.UserID, contact.ID, group, now)
		if err != nil {
			result.RowsSkipped++
			result.Errors = append(result.Errors, ImportInvoicesRowError{
				Row:           rowNumber,
				InvoiceNumber: group.header.invoiceNumber,
				Message:       err.Error(),
			})
			continue
		}

		if err := s.repo.Create(ctx, schemaName, invoice); err != nil {
			result.RowsSkipped++
			result.Errors = append(result.Errors, ImportInvoicesRowError{
				Row:           rowNumber,
				InvoiceNumber: group.header.invoiceNumber,
				Message:       err.Error(),
			})
			continue
		}

		existingKeys[key] = struct{}{}
		result.InvoicesCreated++
		result.LinesImported += len(invoice.Lines)
	}

	if len(result.Errors) == 0 {
		result.Errors = nil
	}

	return result, nil
}

func mappedEInvoiceToImportGroup(mapped einvoicemapper.Invoice, requestedType InvoiceType) (*invoiceImportGroup, error) {
	invoiceType, err := resolveEInvoiceImportType(mapped.Type, requestedType)
	if err != nil {
		return nil, err
	}
	party := mapped.Seller
	if invoiceType == InvoiceTypeSales {
		party = mapped.Buyer
	}

	group := &invoiceImportGroup{
		header: invoiceImportHeader{
			invoiceNumber:  mapped.Number,
			invoiceType:    invoiceType,
			contactRef:     eInvoicePartyContactRef(party),
			issueDate:      mapped.IssueDate,
			dueDate:        mapped.DueDate,
			currency:       strings.ToUpper(strings.TrimSpace(mapped.Currency)),
			exchangeRate:   decimal.NewFromInt(1),
			reference:      mapped.Reference,
			notes:          eInvoiceNotes(mapped),
			explicitStatus: StatusSent,
		},
		rowCount: len(mapped.Lines),
		firstRow: 1,
	}
	if group.header.currency == "" {
		group.header.currency = "EUR"
	}

	for _, line := range mapped.Lines {
		group.lines = append(group.lines, invoiceImportLine{
			description:     line.Description,
			quantity:        line.Quantity,
			unit:            line.Unit,
			unitPrice:       line.UnitPrice,
			discountPercent: line.DiscountPercent,
			vatRate:         line.VATRate,
			vatTreatment:    VATTreatmentStandard,
		})
	}
	return group, nil
}

func resolveEInvoiceImportType(rawType string, requestedType InvoiceType) (InvoiceType, error) {
	if requestedType != "" {
		switch requestedType {
		case InvoiceTypeSales, InvoiceTypePurchase, InvoiceTypeCreditNote:
			return requestedType, nil
		default:
			return "", fmt.Errorf("invalid invoice_type %q", requestedType)
		}
	}

	switch normalizedInvoiceImportKey(rawType) {
	case "cre", "credit", "credit_note", "creditnote":
		return InvoiceTypeCreditNote, nil
	default:
		return InvoiceTypePurchase, nil
	}
}

func eInvoicePartyContactRef(party einvoicemapper.Party) invoiceImportContactRef {
	return invoiceImportContactRef{
		regCode: firstNonEmptyImportValue(party.RegNumber, party.VATRegNumber),
		email:   party.Email,
		name:    party.Name,
	}
}

func eInvoiceNotes(mapped einvoicemapper.Invoice) string {
	notes := strings.TrimSpace(mapped.Notes)
	if mapped.SourceInvoice != "" {
		sourceNote := "Source invoice: " + mapped.SourceInvoice
		if notes == "" {
			return sourceNote
		}
		return notes + "\n" + sourceNote
	}
	return notes
}

func firstNonEmptyImportValue(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
