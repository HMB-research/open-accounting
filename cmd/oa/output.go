package main

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/shopspring/decimal"

	"github.com/HMB-research/open-accounting/internal/accounting"
	"github.com/HMB-research/open-accounting/internal/analytics"
	"github.com/HMB-research/open-accounting/internal/apitoken"
	"github.com/HMB-research/open-accounting/internal/assets"
	"github.com/HMB-research/open-accounting/internal/contacts"
	"github.com/HMB-research/open-accounting/internal/documents"
	"github.com/HMB-research/open-accounting/internal/inventory"
	"github.com/HMB-research/open-accounting/internal/invoicing"
	"github.com/HMB-research/open-accounting/internal/orders"
	"github.com/HMB-research/open-accounting/internal/payments"
	"github.com/HMB-research/open-accounting/internal/payroll"
	"github.com/HMB-research/open-accounting/internal/quotes"
	"github.com/HMB-research/open-accounting/internal/recurring"
	"github.com/HMB-research/open-accounting/internal/reports"
	"github.com/HMB-research/open-accounting/internal/tax"
)

func printJSON(w io.Writer, value any) error {
	payload, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("encode json output: %w", err)
	}
	_, err = fmt.Fprintln(w, string(payload))
	return err
}

func printAPITokensTable(w io.Writer, tokens []apitoken.APIToken) {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "ID\tNAME\tPREFIX\tEXPIRES\tLAST USED\tCREATED")
	for _, token := range tokens {
		_, _ = fmt.Fprintf(
			tw,
			"%s\t%s\t%s\t%s\t%s\t%s\n",
			token.ID,
			token.Name,
			token.TokenPrefix,
			formatTimePtr(token.ExpiresAt),
			formatTimePtr(token.LastUsedAt),
			token.CreatedAt.Format(time.RFC3339),
		)
	}
	_ = tw.Flush()
}

func printAccountsTable(w io.Writer, accounts []accounting.Account) {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "ID\tCODE\tNAME\tTYPE\tACTIVE")
	for _, account := range accounts {
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%t\n", account.ID, account.Code, account.Name, account.AccountType, account.IsActive)
	}
	_ = tw.Flush()
}

func printAccount(w io.Writer, account *accounting.Account) {
	_, _ = fmt.Fprintf(w, "Account %s %s (%s)\n", account.Code, account.Name, account.AccountType)
	_, _ = fmt.Fprintf(w, "ID: %s\n", account.ID)
	if account.ParentID != nil && strings.TrimSpace(*account.ParentID) != "" {
		_, _ = fmt.Fprintf(w, "Parent: %s\n", *account.ParentID)
	}
	_, _ = fmt.Fprintf(w, "Active: %t\n", account.IsActive)
	_, _ = fmt.Fprintf(w, "System: %t\n", account.IsSystem)
	if strings.TrimSpace(account.Description) != "" {
		_, _ = fmt.Fprintf(w, "Description: %s\n", account.Description)
	}
}

func printContactsTable(w io.Writer, contactsList []contacts.Contact) {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "ID\tNAME\tTYPE\tEMAIL\tACTIVE")
	for _, contact := range contactsList {
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%t\n", contact.ID, contact.Name, contact.ContactType, contact.Email, contact.IsActive)
	}
	_ = tw.Flush()
}

func printContact(w io.Writer, contact *contacts.Contact) {
	_, _ = fmt.Fprintf(w, "Contact %s (%s)\n", contact.Name, contact.ContactType)
	_, _ = fmt.Fprintf(w, "ID: %s\n", contact.ID)
	if strings.TrimSpace(contact.Code) != "" {
		_, _ = fmt.Fprintf(w, "Code: %s\n", contact.Code)
	}
	if strings.TrimSpace(contact.Email) != "" {
		_, _ = fmt.Fprintf(w, "Email: %s\n", contact.Email)
	}
	if strings.TrimSpace(contact.Phone) != "" {
		_, _ = fmt.Fprintf(w, "Phone: %s\n", contact.Phone)
	}
	if strings.TrimSpace(contact.RegCode) != "" {
		_, _ = fmt.Fprintf(w, "Registration code: %s\n", contact.RegCode)
	}
	if strings.TrimSpace(contact.VATNumber) != "" {
		_, _ = fmt.Fprintf(w, "VAT number: %s\n", contact.VATNumber)
	}
	if strings.TrimSpace(contact.AddressLine1) != "" {
		_, _ = fmt.Fprintf(w, "Address: %s\n", contact.AddressLine1)
	}
	if strings.TrimSpace(contact.City) != "" || strings.TrimSpace(contact.PostalCode) != "" {
		_, _ = fmt.Fprintf(w, "City: %s %s\n", contact.PostalCode, contact.City)
	}
	if strings.TrimSpace(contact.CountryCode) != "" {
		_, _ = fmt.Fprintf(w, "Country: %s\n", contact.CountryCode)
	}
	_, _ = fmt.Fprintf(w, "Payment terms: %d days\n", contact.PaymentTermsDays)
	_, _ = fmt.Fprintf(w, "Credit limit: %s\n", contact.CreditLimit.String())
	_, _ = fmt.Fprintf(w, "Active: %t\n", contact.IsActive)
	if strings.TrimSpace(contact.Notes) != "" {
		_, _ = fmt.Fprintf(w, "Notes: %s\n", contact.Notes)
	}
}

func printInvoicesTable(w io.Writer, invoices []invoicing.Invoice) {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "ID\tNUMBER\tTYPE\tSTATUS\tISSUE\tDUE\tTOTAL\tPAID\tDUE AMOUNT\tCONTACT")
	for _, invoice := range invoices {
		_, _ = fmt.Fprintf(
			tw,
			"%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			invoice.ID,
			invoice.InvoiceNumber,
			invoice.InvoiceType,
			invoice.Status,
			formatDate(invoice.IssueDate),
			formatDate(invoice.DueDate),
			invoice.Total.String(),
			invoice.AmountPaid.String(),
			invoice.AmountDue().String(),
			invoiceContactLabel(invoice),
		)
	}
	_ = tw.Flush()
}

func printInvoice(w io.Writer, invoice *invoicing.Invoice) {
	_, _ = fmt.Fprintf(w, "Invoice %s (%s)\n", invoice.InvoiceNumber, invoice.Status)
	_, _ = fmt.Fprintf(w, "ID: %s\n", invoice.ID)
	_, _ = fmt.Fprintf(w, "Type: %s\n", invoice.InvoiceType)
	_, _ = fmt.Fprintf(w, "Contact: %s\n", invoiceContactLabel(*invoice))
	_, _ = fmt.Fprintf(w, "Issue date: %s\n", formatDate(invoice.IssueDate))
	_, _ = fmt.Fprintf(w, "Due date: %s\n", formatDate(invoice.DueDate))
	_, _ = fmt.Fprintf(w, "Subtotal: %s %s\n", invoice.Subtotal.String(), invoice.Currency)
	_, _ = fmt.Fprintf(w, "VAT: %s\n", invoice.VATAmount.String())
	_, _ = fmt.Fprintf(w, "Total: %s\n", invoice.Total.String())
	_, _ = fmt.Fprintf(w, "Paid: %s\n", invoice.AmountPaid.String())
	_, _ = fmt.Fprintf(w, "Due amount: %s\n", invoice.AmountDue().String())
	if strings.TrimSpace(invoice.Reference) != "" {
		_, _ = fmt.Fprintf(w, "Reference: %s\n", invoice.Reference)
	}
	if strings.TrimSpace(invoice.Notes) != "" {
		_, _ = fmt.Fprintf(w, "Notes: %s\n", invoice.Notes)
	}
	if len(invoice.Lines) > 0 {
		printInvoiceLinesTable(w, invoice.Lines)
	}
}

func printInvoiceLinesTable(w io.Writer, lines []invoicing.InvoiceLine) {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "NO\tDESCRIPTION\tQTY\tUNIT\tUNIT PRICE\tVAT\tTOTAL")
	for _, line := range lines {
		_, _ = fmt.Fprintf(
			tw,
			"%d\t%s\t%s\t%s\t%s\t%s\t%s\n",
			line.LineNumber,
			line.Description,
			line.Quantity.String(),
			line.Unit,
			line.UnitPrice.String(),
			line.VATRate.String(),
			line.LineTotal.String(),
		)
	}
	_ = tw.Flush()
}

func printQuotesTable(w io.Writer, quotesList []quotes.Quote) {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "ID\tNUMBER\tSTATUS\tDATE\tVALID UNTIL\tTOTAL\tCONTACT")
	for _, quote := range quotesList {
		_, _ = fmt.Fprintf(
			tw,
			"%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			quote.ID,
			quote.QuoteNumber,
			quote.Status,
			formatDate(quote.QuoteDate),
			formatDatePtr(quote.ValidUntil),
			quote.Total.String(),
			quoteContactLabel(quote),
		)
	}
	_ = tw.Flush()
}

func printQuote(w io.Writer, quote *quotes.Quote) {
	_, _ = fmt.Fprintf(w, "Quote %s (%s)\n", quote.QuoteNumber, quote.Status)
	_, _ = fmt.Fprintf(w, "ID: %s\n", quote.ID)
	_, _ = fmt.Fprintf(w, "Contact: %s\n", quoteContactLabel(*quote))
	_, _ = fmt.Fprintf(w, "Quote date: %s\n", formatDate(quote.QuoteDate))
	_, _ = fmt.Fprintf(w, "Valid until: %s\n", formatDatePtr(quote.ValidUntil))
	_, _ = fmt.Fprintf(w, "Subtotal: %s %s\n", quote.Subtotal.String(), quote.Currency)
	_, _ = fmt.Fprintf(w, "VAT: %s\n", quote.VATAmount.String())
	_, _ = fmt.Fprintf(w, "Total: %s\n", quote.Total.String())
	if strings.TrimSpace(quote.Notes) != "" {
		_, _ = fmt.Fprintf(w, "Notes: %s\n", quote.Notes)
	}
	if quote.ConvertedToOrderID != nil && strings.TrimSpace(*quote.ConvertedToOrderID) != "" {
		_, _ = fmt.Fprintf(w, "Converted order: %s\n", *quote.ConvertedToOrderID)
	}
	if quote.ConvertedToInvoiceID != nil && strings.TrimSpace(*quote.ConvertedToInvoiceID) != "" {
		_, _ = fmt.Fprintf(w, "Converted invoice: %s\n", *quote.ConvertedToInvoiceID)
	}
	if len(quote.Lines) > 0 {
		printQuoteLinesTable(w, quote.Lines)
	}
}

func printQuoteLinesTable(w io.Writer, lines []quotes.QuoteLine) {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "NO\tDESCRIPTION\tQTY\tUNIT\tUNIT PRICE\tVAT\tTOTAL")
	for _, line := range lines {
		_, _ = fmt.Fprintf(
			tw,
			"%d\t%s\t%s\t%s\t%s\t%s\t%s\n",
			line.LineNumber,
			line.Description,
			line.Quantity.String(),
			line.Unit,
			line.UnitPrice.String(),
			line.VATRate.String(),
			line.LineTotal.String(),
		)
	}
	_ = tw.Flush()
}

func printOrdersTable(w io.Writer, ordersList []orders.Order) {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "ID\tNUMBER\tSTATUS\tDATE\tEXPECTED\tTOTAL\tCONTACT")
	for _, order := range ordersList {
		_, _ = fmt.Fprintf(
			tw,
			"%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			order.ID,
			order.OrderNumber,
			order.Status,
			formatDate(order.OrderDate),
			formatDatePtr(order.ExpectedDelivery),
			order.Total.String(),
			orderContactLabel(order),
		)
	}
	_ = tw.Flush()
}

func printOrder(w io.Writer, order *orders.Order) {
	_, _ = fmt.Fprintf(w, "Order %s (%s)\n", order.OrderNumber, order.Status)
	_, _ = fmt.Fprintf(w, "ID: %s\n", order.ID)
	_, _ = fmt.Fprintf(w, "Contact: %s\n", orderContactLabel(*order))
	_, _ = fmt.Fprintf(w, "Order date: %s\n", formatDate(order.OrderDate))
	_, _ = fmt.Fprintf(w, "Expected delivery: %s\n", formatDatePtr(order.ExpectedDelivery))
	_, _ = fmt.Fprintf(w, "Subtotal: %s %s\n", order.Subtotal.String(), order.Currency)
	_, _ = fmt.Fprintf(w, "VAT: %s\n", order.VATAmount.String())
	_, _ = fmt.Fprintf(w, "Total: %s\n", order.Total.String())
	if order.QuoteID != nil && strings.TrimSpace(*order.QuoteID) != "" {
		_, _ = fmt.Fprintf(w, "Quote: %s\n", *order.QuoteID)
	}
	if strings.TrimSpace(order.Notes) != "" {
		_, _ = fmt.Fprintf(w, "Notes: %s\n", order.Notes)
	}
	if order.ConvertedToInvoiceID != nil && strings.TrimSpace(*order.ConvertedToInvoiceID) != "" {
		_, _ = fmt.Fprintf(w, "Converted invoice: %s\n", *order.ConvertedToInvoiceID)
	}
	if len(order.Lines) > 0 {
		printOrderLinesTable(w, order.Lines)
	}
}

func printOrderLinesTable(w io.Writer, lines []orders.OrderLine) {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "NO\tDESCRIPTION\tQTY\tUNIT\tUNIT PRICE\tVAT\tTOTAL")
	for _, line := range lines {
		_, _ = fmt.Fprintf(
			tw,
			"%d\t%s\t%s\t%s\t%s\t%s\t%s\n",
			line.LineNumber,
			line.Description,
			line.Quantity.String(),
			line.Unit,
			line.UnitPrice.String(),
			line.VATRate.String(),
			line.LineTotal.String(),
		)
	}
	_ = tw.Flush()
}

func printRecurringInvoicesTable(w io.Writer, invoices []recurring.RecurringInvoice) {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "ID\tNAME\tCONTACT\tFREQUENCY\tNEXT\tACTIVE\tGENERATED")
	for _, invoice := range invoices {
		_, _ = fmt.Fprintf(
			tw,
			"%s\t%s\t%s\t%s\t%s\t%t\t%d\n",
			invoice.ID,
			invoice.Name,
			recurringContactLabel(invoice),
			invoice.Frequency,
			formatDate(invoice.NextGenerationDate),
			invoice.IsActive,
			invoice.GeneratedCount,
		)
	}
	_ = tw.Flush()
}

func printRecurringInvoice(w io.Writer, invoice *recurring.RecurringInvoice) {
	_, _ = fmt.Fprintf(w, "Recurring invoice %s (%s)\n", invoice.Name, invoice.Frequency)
	_, _ = fmt.Fprintf(w, "ID: %s\n", invoice.ID)
	_, _ = fmt.Fprintf(w, "Contact: %s\n", recurringContactLabel(*invoice))
	_, _ = fmt.Fprintf(w, "Type: %s\n", invoice.InvoiceType)
	_, _ = fmt.Fprintf(w, "Currency: %s\n", invoice.Currency)
	_, _ = fmt.Fprintf(w, "Start date: %s\n", formatDate(invoice.StartDate))
	_, _ = fmt.Fprintf(w, "End date: %s\n", formatDatePtr(invoice.EndDate))
	_, _ = fmt.Fprintf(w, "Next generation: %s\n", formatDate(invoice.NextGenerationDate))
	_, _ = fmt.Fprintf(w, "Payment terms: %d days\n", invoice.PaymentTermsDays)
	_, _ = fmt.Fprintf(w, "Active: %t\n", invoice.IsActive)
	_, _ = fmt.Fprintf(w, "Generated count: %d\n", invoice.GeneratedCount)
	if strings.TrimSpace(invoice.Reference) != "" {
		_, _ = fmt.Fprintf(w, "Reference: %s\n", invoice.Reference)
	}
	if strings.TrimSpace(invoice.Notes) != "" {
		_, _ = fmt.Fprintf(w, "Notes: %s\n", invoice.Notes)
	}
	_, _ = fmt.Fprintf(w, "Send email: %t\n", invoice.SendEmailOnGeneration)
	if strings.TrimSpace(invoice.EmailTemplateType) != "" {
		_, _ = fmt.Fprintf(w, "Email template: %s\n", invoice.EmailTemplateType)
	}
	if strings.TrimSpace(invoice.RecipientEmailOverride) != "" {
		_, _ = fmt.Fprintf(w, "Email recipient: %s\n", invoice.RecipientEmailOverride)
	}
	_, _ = fmt.Fprintf(w, "Attach PDF: %t\n", invoice.AttachPDFToEmail)
	if len(invoice.Lines) > 0 {
		printRecurringInvoiceLinesTable(w, invoice.Lines)
	}
}

func printRecurringInvoiceLinesTable(w io.Writer, lines []recurring.RecurringInvoiceLine) {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "NO\tDESCRIPTION\tQTY\tUNIT\tUNIT PRICE\tVAT")
	for _, line := range lines {
		_, _ = fmt.Fprintf(
			tw,
			"%d\t%s\t%s\t%s\t%s\t%s\n",
			line.LineNumber,
			line.Description,
			line.Quantity.String(),
			line.Unit,
			line.UnitPrice.String(),
			line.VATRate.String(),
		)
	}
	_ = tw.Flush()
}

func printRecurringGenerationResultsTable(w io.Writer, results []recurring.GenerationResult) {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "RECURRING\tINVOICE\tNUMBER\tEMAIL")
	for _, result := range results {
		_, _ = fmt.Fprintf(
			tw,
			"%s\t%s\t%s\t%s\n",
			result.RecurringInvoiceID,
			result.GeneratedInvoiceID,
			result.GeneratedInvoiceNumber,
			result.EmailStatus,
		)
	}
	_ = tw.Flush()
}

func printAssetCategoriesTable(w io.Writer, categories []assets.AssetCategory) {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "ID\tNAME\tMETHOD\tLIFE MONTHS\tRESIDUAL %")
	for _, category := range categories {
		_, _ = fmt.Fprintf(
			tw,
			"%s\t%s\t%s\t%d\t%s\n",
			category.ID,
			category.Name,
			category.DepreciationMethod,
			category.DefaultUsefulLifeMonths,
			category.DefaultResidualValuePercent.String(),
		)
	}
	_ = tw.Flush()
}

func printAssetCategory(w io.Writer, category *assets.AssetCategory) {
	_, _ = fmt.Fprintf(w, "Asset category %s\n", category.Name)
	_, _ = fmt.Fprintf(w, "ID: %s\n", category.ID)
	if strings.TrimSpace(category.Description) != "" {
		_, _ = fmt.Fprintf(w, "Description: %s\n", category.Description)
	}
	_, _ = fmt.Fprintf(w, "Depreciation method: %s\n", category.DepreciationMethod)
	_, _ = fmt.Fprintf(w, "Useful life months: %d\n", category.DefaultUsefulLifeMonths)
	_, _ = fmt.Fprintf(w, "Residual percent: %s\n", category.DefaultResidualValuePercent.String())
	if category.AssetAccountID != nil && strings.TrimSpace(*category.AssetAccountID) != "" {
		_, _ = fmt.Fprintf(w, "Asset account: %s\n", *category.AssetAccountID)
	}
	if category.DepreciationExpenseAccountID != nil && strings.TrimSpace(*category.DepreciationExpenseAccountID) != "" {
		_, _ = fmt.Fprintf(w, "Depreciation expense account: %s\n", *category.DepreciationExpenseAccountID)
	}
	if category.AccumulatedDepreciationAcctID != nil && strings.TrimSpace(*category.AccumulatedDepreciationAcctID) != "" {
		_, _ = fmt.Fprintf(w, "Accumulated depreciation account: %s\n", *category.AccumulatedDepreciationAcctID)
	}
}

func printAssetsTable(w io.Writer, assetList []assets.FixedAsset) {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "ID\tNUMBER\tNAME\tSTATUS\tPURCHASE DATE\tCOST\tBOOK VALUE\tLOCATION")
	for _, asset := range assetList {
		_, _ = fmt.Fprintf(
			tw,
			"%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			asset.ID,
			asset.AssetNumber,
			asset.Name,
			asset.Status,
			formatDate(asset.PurchaseDate),
			asset.PurchaseCost.String(),
			asset.BookValue.String(),
			asset.Location,
		)
	}
	_ = tw.Flush()
}

func printAsset(w io.Writer, asset *assets.FixedAsset) {
	_, _ = fmt.Fprintf(w, "Asset %s %s (%s)\n", asset.AssetNumber, asset.Name, asset.Status)
	_, _ = fmt.Fprintf(w, "ID: %s\n", asset.ID)
	if strings.TrimSpace(asset.Description) != "" {
		_, _ = fmt.Fprintf(w, "Description: %s\n", asset.Description)
	}
	if asset.CategoryID != nil && strings.TrimSpace(*asset.CategoryID) != "" {
		_, _ = fmt.Fprintf(w, "Category: %s\n", *asset.CategoryID)
	}
	_, _ = fmt.Fprintf(w, "Purchase date: %s\n", formatDate(asset.PurchaseDate))
	_, _ = fmt.Fprintf(w, "Purchase cost: %s\n", asset.PurchaseCost.String())
	_, _ = fmt.Fprintf(w, "Book value: %s\n", asset.BookValue.String())
	_, _ = fmt.Fprintf(w, "Accumulated depreciation: %s\n", asset.AccumulatedDepreciation.String())
	_, _ = fmt.Fprintf(w, "Depreciation method: %s\n", asset.DepreciationMethod)
	_, _ = fmt.Fprintf(w, "Useful life months: %d\n", asset.UsefulLifeMonths)
	_, _ = fmt.Fprintf(w, "Residual value: %s\n", asset.ResidualValue.String())
	_, _ = fmt.Fprintf(w, "Depreciation start: %s\n", formatDatePtr(asset.DepreciationStartDate))
	_, _ = fmt.Fprintf(w, "Last depreciation: %s\n", formatDatePtr(asset.LastDepreciationDate))
	if asset.SupplierID != nil && strings.TrimSpace(*asset.SupplierID) != "" {
		_, _ = fmt.Fprintf(w, "Supplier: %s\n", *asset.SupplierID)
	}
	if strings.TrimSpace(asset.SerialNumber) != "" {
		_, _ = fmt.Fprintf(w, "Serial number: %s\n", asset.SerialNumber)
	}
	if strings.TrimSpace(asset.Location) != "" {
		_, _ = fmt.Fprintf(w, "Location: %s\n", asset.Location)
	}
	if asset.DisposalDate != nil {
		_, _ = fmt.Fprintf(w, "Disposal date: %s\n", formatDatePtr(asset.DisposalDate))
	}
	if asset.DisposalMethod != nil {
		_, _ = fmt.Fprintf(w, "Disposal method: %s\n", *asset.DisposalMethod)
	}
	if strings.TrimSpace(asset.DisposalNotes) != "" {
		_, _ = fmt.Fprintf(w, "Disposal notes: %s\n", asset.DisposalNotes)
	}
}

func printDepreciationEntriesTable(w io.Writer, entries []assets.DepreciationEntry) {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "ID\tDATE\tPERIOD\tAMOUNT\tACCUMULATED\tBOOK VALUE")
	for _, entry := range entries {
		_, _ = fmt.Fprintf(
			tw,
			"%s\t%s\t%s..%s\t%s\t%s\t%s\n",
			entry.ID,
			formatDate(entry.DepreciationDate),
			formatDate(entry.PeriodStart),
			formatDate(entry.PeriodEnd),
			entry.DepreciationAmount.String(),
			entry.AccumulatedTotal.String(),
			entry.BookValueAfter.String(),
		)
	}
	_ = tw.Flush()
}

func printProductCategoriesTable(w io.Writer, categories []inventory.ProductCategory) {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "ID\tNAME\tPARENT\tDESCRIPTION")
	for _, category := range categories {
		_, _ = fmt.Fprintf(
			tw,
			"%s\t%s\t%s\t%s\n",
			category.ID,
			category.Name,
			category.ParentID,
			category.Description,
		)
	}
	_ = tw.Flush()
}

func printProductCategory(w io.Writer, category *inventory.ProductCategory) {
	_, _ = fmt.Fprintf(w, "Product category %s\n", category.Name)
	_, _ = fmt.Fprintf(w, "ID: %s\n", category.ID)
	if strings.TrimSpace(category.ParentID) != "" {
		_, _ = fmt.Fprintf(w, "Parent: %s\n", category.ParentID)
	}
	if strings.TrimSpace(category.Description) != "" {
		_, _ = fmt.Fprintf(w, "Description: %s\n", category.Description)
	}
}

func printProductsTable(w io.Writer, products []inventory.Product) {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "ID\tCODE\tNAME\tTYPE\tACTIVE\tPRICE\tSTOCK\tUNIT")
	for _, product := range products {
		_, _ = fmt.Fprintf(
			tw,
			"%s\t%s\t%s\t%s\t%t\t%s\t%s\t%s\n",
			product.ID,
			product.Code,
			product.Name,
			product.ProductType,
			product.IsActive,
			product.SalesPrice.String(),
			product.CurrentStock.String(),
			product.Unit,
		)
	}
	_ = tw.Flush()
}

func printProduct(w io.Writer, product *inventory.Product) {
	_, _ = fmt.Fprintf(w, "Product %s %s (%s)\n", product.Code, product.Name, product.ProductType)
	_, _ = fmt.Fprintf(w, "ID: %s\n", product.ID)
	if strings.TrimSpace(product.Description) != "" {
		_, _ = fmt.Fprintf(w, "Description: %s\n", product.Description)
	}
	if strings.TrimSpace(product.CategoryID) != "" {
		_, _ = fmt.Fprintf(w, "Category: %s\n", product.CategoryID)
	}
	_, _ = fmt.Fprintf(w, "Unit: %s\n", product.Unit)
	_, _ = fmt.Fprintf(w, "Sales price: %s\n", product.SalesPrice.String())
	_, _ = fmt.Fprintf(w, "Purchase price: %s\n", product.PurchasePrice.String())
	_, _ = fmt.Fprintf(w, "VAT rate: %s\n", product.VATRate.String())
	_, _ = fmt.Fprintf(w, "Current stock: %s\n", product.CurrentStock.String())
	_, _ = fmt.Fprintf(w, "Minimum stock: %s\n", product.MinStockLevel.String())
	_, _ = fmt.Fprintf(w, "Reorder point: %s\n", product.ReorderPoint.String())
	_, _ = fmt.Fprintf(w, "Track inventory: %t\n", product.TrackInventory)
	_, _ = fmt.Fprintf(w, "Active: %t\n", product.IsActive)
	if strings.TrimSpace(product.Barcode) != "" {
		_, _ = fmt.Fprintf(w, "Barcode: %s\n", product.Barcode)
	}
	if strings.TrimSpace(product.SupplierID) != "" {
		_, _ = fmt.Fprintf(w, "Supplier: %s\n", product.SupplierID)
	}
	if product.LeadTimeDays > 0 {
		_, _ = fmt.Fprintf(w, "Lead time days: %d\n", product.LeadTimeDays)
	}
	if strings.TrimSpace(product.SaleAccountID) != "" {
		_, _ = fmt.Fprintf(w, "Sale account: %s\n", product.SaleAccountID)
	}
	if strings.TrimSpace(product.PurchaseAccountID) != "" {
		_, _ = fmt.Fprintf(w, "Purchase account: %s\n", product.PurchaseAccountID)
	}
	if strings.TrimSpace(product.InventoryAccountID) != "" {
		_, _ = fmt.Fprintf(w, "Inventory account: %s\n", product.InventoryAccountID)
	}
}

func printWarehousesTable(w io.Writer, warehouses []inventory.Warehouse) {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "ID\tCODE\tNAME\tDEFAULT\tACTIVE\tADDRESS")
	for _, warehouse := range warehouses {
		_, _ = fmt.Fprintf(
			tw,
			"%s\t%s\t%s\t%t\t%t\t%s\n",
			warehouse.ID,
			warehouse.Code,
			warehouse.Name,
			warehouse.IsDefault,
			warehouse.IsActive,
			warehouse.Address,
		)
	}
	_ = tw.Flush()
}

func printWarehouse(w io.Writer, warehouse *inventory.Warehouse) {
	_, _ = fmt.Fprintf(w, "Warehouse %s %s\n", warehouse.Code, warehouse.Name)
	_, _ = fmt.Fprintf(w, "ID: %s\n", warehouse.ID)
	if strings.TrimSpace(warehouse.Address) != "" {
		_, _ = fmt.Fprintf(w, "Address: %s\n", warehouse.Address)
	}
	_, _ = fmt.Fprintf(w, "Default: %t\n", warehouse.IsDefault)
	_, _ = fmt.Fprintf(w, "Active: %t\n", warehouse.IsActive)
}

func printStockLevelsTable(w io.Writer, levels []inventory.StockLevel) {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "ID\tPRODUCT\tWAREHOUSE\tQUANTITY\tRESERVED\tAVAILABLE\tUPDATED")
	for _, level := range levels {
		_, _ = fmt.Fprintf(
			tw,
			"%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			level.ID,
			level.ProductID,
			level.WarehouseID,
			level.Quantity.String(),
			level.ReservedQty.String(),
			level.AvailableQty.String(),
			formatTime(level.LastUpdated),
		)
	}
	_ = tw.Flush()
}

func printInventoryMovementsTable(w io.Writer, movements []inventory.InventoryMovement) {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "ID\tDATE\tTYPE\tPRODUCT\tWAREHOUSE\tQTY\tUNIT COST\tREFERENCE\tNOTES")
	for _, movement := range movements {
		_, _ = fmt.Fprintf(
			tw,
			"%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			movement.ID,
			formatDate(movement.MovementDate),
			movement.MovementType,
			movement.ProductID,
			movement.WarehouseID,
			movement.Quantity.String(),
			movement.UnitCost.String(),
			movement.Reference,
			movement.Notes,
		)
	}
	_ = tw.Flush()
}

func printCostCentersTable(w io.Writer, costCenters []accounting.CostCenter) {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "ID\tCODE\tNAME\tACTIVE\tBUDGET\tPERIOD")
	for _, costCenter := range costCenters {
		_, _ = fmt.Fprintf(
			tw,
			"%s\t%s\t%s\t%t\t%s\t%s\n",
			costCenter.ID,
			costCenter.Code,
			costCenter.Name,
			costCenter.IsActive,
			formatDecimalPtr(costCenter.BudgetAmount),
			costCenter.BudgetPeriod,
		)
	}
	_ = tw.Flush()
}

func printCostCenter(w io.Writer, costCenter *accounting.CostCenter) {
	_, _ = fmt.Fprintf(w, "Cost center %s %s\n", costCenter.Code, costCenter.Name)
	_, _ = fmt.Fprintf(w, "ID: %s\n", costCenter.ID)
	if strings.TrimSpace(costCenter.Description) != "" {
		_, _ = fmt.Fprintf(w, "Description: %s\n", costCenter.Description)
	}
	if costCenter.ParentID != nil && strings.TrimSpace(*costCenter.ParentID) != "" {
		_, _ = fmt.Fprintf(w, "Parent: %s\n", *costCenter.ParentID)
	}
	_, _ = fmt.Fprintf(w, "Active: %t\n", costCenter.IsActive)
	_, _ = fmt.Fprintf(w, "Budget: %s\n", formatDecimalPtr(costCenter.BudgetAmount))
	if costCenter.BudgetPeriod != "" {
		_, _ = fmt.Fprintf(w, "Budget period: %s\n", costCenter.BudgetPeriod)
	}
	if costCenter.TotalSpent != nil {
		_, _ = fmt.Fprintf(w, "Total spent: %s\n", costCenter.TotalSpent.String())
	}
	if costCenter.BudgetUsed != nil {
		_, _ = fmt.Fprintf(w, "Budget used: %s%%\n", costCenter.BudgetUsed.String())
	}
}

func printCostCenterReport(w io.Writer, report *accounting.CostCenterReport) {
	_, _ = fmt.Fprintf(w, "Cost center report %s..%s\n", formatDate(report.PeriodStart), formatDate(report.PeriodEnd))
	_, _ = fmt.Fprintf(w, "Total expenses: %s\n", report.TotalExpenses.String())
	_, _ = fmt.Fprintf(w, "Total budget: %s\n", report.TotalBudget.String())
	if len(report.CostCenters) == 0 {
		return
	}

	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "CODE\tNAME\tEXPENSES\tBUDGET\tUSED %\tOVER")
	for _, summary := range report.CostCenters {
		_, _ = fmt.Fprintf(
			tw,
			"%s\t%s\t%s\t%s\t%s\t%t\n",
			summary.CostCenter.Code,
			summary.CostCenter.Name,
			summary.TotalExpenses.String(),
			summary.BudgetAmount.String(),
			summary.BudgetUsed.String(),
			summary.IsOverBudget,
		)
	}
	_ = tw.Flush()
}

func printJournalEntriesTable(w io.Writer, entries []accounting.JournalEntry) {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "ID\tNUMBER\tDATE\tSTATUS\tDEBIT\tCREDIT\tREFERENCE\tDESCRIPTION")
	for _, entry := range entries {
		_, _ = fmt.Fprintf(
			tw,
			"%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			entry.ID,
			entry.EntryNumber,
			formatDate(entry.EntryDate),
			entry.Status,
			entry.TotalDebits().String(),
			entry.TotalCredits().String(),
			entry.Reference,
			entry.Description,
		)
	}
	_ = tw.Flush()
}

func printJournalEntry(w io.Writer, entry *accounting.JournalEntry) {
	_, _ = fmt.Fprintf(w, "Journal entry %s (%s)\n", entry.EntryNumber, entry.Status)
	_, _ = fmt.Fprintf(w, "ID: %s\n", entry.ID)
	_, _ = fmt.Fprintf(w, "Date: %s\n", formatDate(entry.EntryDate))
	_, _ = fmt.Fprintf(w, "Description: %s\n", entry.Description)
	if strings.TrimSpace(entry.Reference) != "" {
		_, _ = fmt.Fprintf(w, "Reference: %s\n", entry.Reference)
	}
	if strings.TrimSpace(entry.SourceType) != "" {
		_, _ = fmt.Fprintf(w, "Source: %s\n", entry.SourceType)
	}
	_, _ = fmt.Fprintf(w, "Total debits: %s\n", entry.TotalDebits().String())
	_, _ = fmt.Fprintf(w, "Total credits: %s\n", entry.TotalCredits().String())
	_, _ = fmt.Fprintf(w, "Balanced: %t\n", entry.IsBalanced())
	if strings.TrimSpace(entry.VoidReason) != "" {
		_, _ = fmt.Fprintf(w, "Void reason: %s\n", entry.VoidReason)
	}
	if len(entry.Lines) > 0 {
		printJournalEntryLinesTable(w, entry.Lines)
	}
}

func printJournalEntryLinesTable(w io.Writer, lines []accounting.JournalEntryLine) {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "ACCOUNT\tDESCRIPTION\tDEBIT\tCREDIT\tCURRENCY")
	for _, line := range lines {
		_, _ = fmt.Fprintf(
			tw,
			"%s\t%s\t%s\t%s\t%s\n",
			journalLineAccountLabel(line),
			line.Description,
			line.BaseDebit.String(),
			line.BaseCredit.String(),
			line.Currency,
		)
	}
	_ = tw.Flush()
}

func printEmployeesTable(w io.Writer, employees []payroll.Employee) {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "ID\tNUMBER\tNAME\tTYPE\tEMAIL\tACTIVE")
	for _, employee := range employees {
		_, _ = fmt.Fprintf(
			tw,
			"%s\t%s\t%s\t%s\t%s\t%t\n",
			employee.ID,
			employee.EmployeeNumber,
			employee.FullName(),
			employee.EmploymentType,
			employee.Email,
			employee.IsActive,
		)
	}
	_ = tw.Flush()
}

func printEmployee(w io.Writer, employee *payroll.Employee) {
	_, _ = fmt.Fprintf(w, "Employee %s (%s)\n", employee.FullName(), employee.EmploymentType)
	_, _ = fmt.Fprintf(w, "ID: %s\n", employee.ID)
	if strings.TrimSpace(employee.EmployeeNumber) != "" {
		_, _ = fmt.Fprintf(w, "Number: %s\n", employee.EmployeeNumber)
	}
	if strings.TrimSpace(employee.PersonalCode) != "" {
		_, _ = fmt.Fprintf(w, "Personal code: %s\n", employee.PersonalCode)
	}
	if strings.TrimSpace(employee.Email) != "" {
		_, _ = fmt.Fprintf(w, "Email: %s\n", employee.Email)
	}
	if strings.TrimSpace(employee.Phone) != "" {
		_, _ = fmt.Fprintf(w, "Phone: %s\n", employee.Phone)
	}
	if strings.TrimSpace(employee.Position) != "" {
		_, _ = fmt.Fprintf(w, "Position: %s\n", employee.Position)
	}
	if strings.TrimSpace(employee.Department) != "" {
		_, _ = fmt.Fprintf(w, "Department: %s\n", employee.Department)
	}
	_, _ = fmt.Fprintf(w, "Start date: %s\n", formatDate(employee.StartDate))
	_, _ = fmt.Fprintf(w, "End date: %s\n", formatDatePtr(employee.EndDate))
	_, _ = fmt.Fprintf(w, "Basic exemption: %t (%s)\n", employee.ApplyBasicExemption, employee.BasicExemptionAmount.String())
	_, _ = fmt.Fprintf(w, "Funded pension rate: %s\n", employee.FundedPensionRate.String())
	_, _ = fmt.Fprintf(w, "Active: %t\n", employee.IsActive)
}

func printDocumentsTable(w io.Writer, docs []documents.Document) {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "ID\tENTITY\tTYPE\tFILE\tREVIEW\tRETENTION\tCREATED")
	for _, doc := range docs {
		_, _ = fmt.Fprintf(
			tw,
			"%s\t%s:%s\t%s\t%s\t%s\t%s\t%s\n",
			doc.ID,
			doc.EntityType,
			doc.EntityID,
			doc.DocumentType,
			doc.FileName,
			doc.ReviewStatus,
			formatTimePtr(doc.RetentionUntil),
			doc.CreatedAt.Format(time.RFC3339),
		)
	}
	_ = tw.Flush()
}

func printPaymentsTable(w io.Writer, paymentsList []payments.Payment) {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "ID\tNUMBER\tTYPE\tDATE\tAMOUNT\tALLOCATED\tUNALLOCATED\tMETHOD\tREFERENCE")
	for _, payment := range paymentsList {
		_, _ = fmt.Fprintf(
			tw,
			"%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			payment.ID,
			payment.PaymentNumber,
			payment.PaymentType,
			formatDate(payment.PaymentDate),
			payment.Amount.String(),
			payment.TotalAllocated().String(),
			payment.UnallocatedAmount().String(),
			payment.PaymentMethod,
			payment.Reference,
		)
	}
	_ = tw.Flush()
}

func printPayment(w io.Writer, payment *payments.Payment) {
	_, _ = fmt.Fprintf(w, "Payment %s (%s)\n", payment.PaymentNumber, payment.PaymentType)
	_, _ = fmt.Fprintf(w, "ID: %s\n", payment.ID)
	_, _ = fmt.Fprintf(w, "Date: %s\n", formatDate(payment.PaymentDate))
	_, _ = fmt.Fprintf(w, "Amount: %s %s\n", payment.Amount.String(), payment.Currency)
	_, _ = fmt.Fprintf(w, "Base amount: %s\n", payment.BaseAmount.String())
	_, _ = fmt.Fprintf(w, "Allocated: %s\n", payment.TotalAllocated().String())
	_, _ = fmt.Fprintf(w, "Unallocated: %s\n", payment.UnallocatedAmount().String())
	if payment.ContactID != nil && strings.TrimSpace(*payment.ContactID) != "" {
		_, _ = fmt.Fprintf(w, "Contact: %s\n", *payment.ContactID)
	}
	if strings.TrimSpace(payment.PaymentMethod) != "" {
		_, _ = fmt.Fprintf(w, "Method: %s\n", payment.PaymentMethod)
	}
	if strings.TrimSpace(payment.BankAccount) != "" {
		_, _ = fmt.Fprintf(w, "Bank account: %s\n", payment.BankAccount)
	}
	if strings.TrimSpace(payment.Reference) != "" {
		_, _ = fmt.Fprintf(w, "Reference: %s\n", payment.Reference)
	}
	if strings.TrimSpace(payment.Notes) != "" {
		_, _ = fmt.Fprintf(w, "Notes: %s\n", payment.Notes)
	}
	if len(payment.Allocations) > 0 {
		printPaymentAllocationsTable(w, payment.Allocations)
	}
}

func printPaymentAllocationsTable(w io.Writer, allocations []payments.PaymentAllocation) {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "ID\tINVOICE\tAMOUNT\tCREATED")
	for _, allocation := range allocations {
		_, _ = fmt.Fprintf(
			tw,
			"%s\t%s\t%s\t%s\n",
			allocation.ID,
			allocation.InvoiceID,
			allocation.Amount.String(),
			allocation.CreatedAt.Format(time.RFC3339),
		)
	}
	_ = tw.Flush()
}

func printPayrollRunsTable(w io.Writer, runs []payroll.PayrollRun) {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "ID\tPERIOD\tSTATUS\tPAYMENT DATE\tGROSS\tNET\tEMPLOYER COST")
	for _, run := range runs {
		_, _ = fmt.Fprintf(
			tw,
			"%s\t%04d-%02d\t%s\t%s\t%s\t%s\t%s\n",
			run.ID,
			run.PeriodYear,
			run.PeriodMonth,
			run.Status,
			formatDatePtr(run.PaymentDate),
			run.TotalGross.String(),
			run.TotalNet.String(),
			run.TotalEmployerCost.String(),
		)
	}
	_ = tw.Flush()
}

func printPayrollRun(w io.Writer, run *payroll.PayrollRun) {
	_, _ = fmt.Fprintf(w, "Payroll run %04d-%02d (%s)\n", run.PeriodYear, run.PeriodMonth, run.Status)
	_, _ = fmt.Fprintf(w, "ID: %s\n", run.ID)
	_, _ = fmt.Fprintf(w, "Payment date: %s\n", formatDatePtr(run.PaymentDate))
	_, _ = fmt.Fprintf(w, "Total gross: %s\n", run.TotalGross.String())
	_, _ = fmt.Fprintf(w, "Total net: %s\n", run.TotalNet.String())
	_, _ = fmt.Fprintf(w, "Total employer cost: %s\n", run.TotalEmployerCost.String())
	if strings.TrimSpace(run.Notes) != "" {
		_, _ = fmt.Fprintf(w, "Notes: %s\n", run.Notes)
	}
	if len(run.Payslips) > 0 {
		printPayslipsTable(w, run.Payslips)
	}
}

func printPayslipsTable(w io.Writer, payslips []payroll.Payslip) {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "ID\tEMPLOYEE\tGROSS\tNET\tINCOME TAX\tSOCIAL TAX\tSTATUS")
	for _, payslip := range payslips {
		_, _ = fmt.Fprintf(
			tw,
			"%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			payslip.ID,
			payslipEmployeeName(payslip),
			payslip.GrossSalary.String(),
			payslip.NetSalary.String(),
			payslip.IncomeTax.String(),
			payslip.SocialTax.String(),
			payslip.PaymentStatus,
		)
	}
	_ = tw.Flush()
}

func printTaxCalculation(w io.Writer, calc *payroll.TaxCalculation) {
	_, _ = fmt.Fprintf(w, "Gross salary: %s\n", calc.GrossSalary.String())
	_, _ = fmt.Fprintf(w, "Basic exemption: %s\n", calc.BasicExemption.String())
	_, _ = fmt.Fprintf(w, "Taxable income: %s\n", calc.TaxableIncome.String())
	_, _ = fmt.Fprintf(w, "Income tax: %s\n", calc.IncomeTax.String())
	_, _ = fmt.Fprintf(w, "Unemployment employee: %s\n", calc.UnemploymentEE.String())
	_, _ = fmt.Fprintf(w, "Funded pension: %s\n", calc.FundedPension.String())
	_, _ = fmt.Fprintf(w, "Total deductions: %s\n", calc.TotalDeductions.String())
	_, _ = fmt.Fprintf(w, "Net salary: %s\n", calc.NetSalary.String())
	_, _ = fmt.Fprintf(w, "Social tax: %s\n", calc.SocialTax.String())
	_, _ = fmt.Fprintf(w, "Unemployment employer: %s\n", calc.UnemploymentER.String())
	_, _ = fmt.Fprintf(w, "Total employer cost: %s\n", calc.TotalEmployerCost.String())
}

func printTSDDeclarationsTable(w io.Writer, declarations []payroll.TSDDeclaration) {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "ID\tPERIOD\tSTATUS\tPAYMENTS\tINCOME TAX\tSOCIAL TAX\tEMTA REF")
	for _, declaration := range declarations {
		_, _ = fmt.Fprintf(
			tw,
			"%s\t%04d-%02d\t%s\t%s\t%s\t%s\t%s\n",
			declaration.ID,
			declaration.PeriodYear,
			declaration.PeriodMonth,
			declaration.Status,
			declaration.TotalPayments.String(),
			declaration.TotalIncomeTax.String(),
			declaration.TotalSocialTax.String(),
			declaration.EMTAReference,
		)
	}
	_ = tw.Flush()
}

func printTSDDeclaration(w io.Writer, declaration *payroll.TSDDeclaration) {
	_, _ = fmt.Fprintf(w, "TSD %04d-%02d (%s)\n", declaration.PeriodYear, declaration.PeriodMonth, declaration.Status)
	_, _ = fmt.Fprintf(w, "Total payments: %s\n", declaration.TotalPayments.String())
	_, _ = fmt.Fprintf(w, "Income tax: %s\n", declaration.TotalIncomeTax.String())
	_, _ = fmt.Fprintf(w, "Social tax: %s\n", declaration.TotalSocialTax.String())
	_, _ = fmt.Fprintf(w, "Unemployment employer: %s\n", declaration.TotalUnemploymentER.String())
	_, _ = fmt.Fprintf(w, "Unemployment employee: %s\n", declaration.TotalUnemploymentEE.String())
	_, _ = fmt.Fprintf(w, "Funded pension: %s\n", declaration.TotalFundedPension.String())
	if declaration.EMTAReference != "" {
		_, _ = fmt.Fprintf(w, "e-MTA reference: %s\n", declaration.EMTAReference)
	}
	if len(declaration.Rows) == 0 {
		return
	}

	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "EMPLOYEE\tPAYMENT TYPE\tGROSS\tTAXABLE\tINCOME TAX\tSOCIAL TAX")
	for _, row := range declaration.Rows {
		_, _ = fmt.Fprintf(
			tw,
			"%s %s\t%s\t%s\t%s\t%s\t%s\n",
			row.FirstName,
			row.LastName,
			row.PaymentType,
			row.GrossPayment.String(),
			row.TaxableAmount.String(),
			row.IncomeTax.String(),
			row.SocialTax.String(),
		)
	}
	_ = tw.Flush()
}

func printKMDDeclarationsTable(w io.Writer, declarations []tax.KMDDeclaration) {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "ID\tPERIOD\tSTATUS\tOUTPUT VAT\tINPUT VAT\tPAYABLE")
	for _, declaration := range declarations {
		_, _ = fmt.Fprintf(
			tw,
			"%s\t%s\t%s\t%s\t%s\t%s\n",
			declaration.ID,
			declaration.Period(),
			declaration.Status,
			declaration.TotalOutputVAT.String(),
			declaration.TotalInputVAT.String(),
			declaration.CalculatePayable().String(),
		)
	}
	_ = tw.Flush()
}

func printKMDDeclaration(w io.Writer, declaration *tax.KMDDeclaration) {
	_, _ = fmt.Fprintf(w, "KMD %s (%s)\n", declaration.Period(), declaration.Status)
	_, _ = fmt.Fprintf(w, "Output VAT: %s\n", declaration.TotalOutputVAT.String())
	_, _ = fmt.Fprintf(w, "Input VAT: %s\n", declaration.TotalInputVAT.String())
	_, _ = fmt.Fprintf(w, "Payable: %s\n", declaration.CalculatePayable().String())
	if len(declaration.Rows) == 0 {
		return
	}

	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "ROW\tDESCRIPTION\tTAX BASE\tTAX AMOUNT")
	for _, row := range declaration.Rows {
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", row.Code, row.Description, row.TaxBase.String(), row.TaxAmount.String())
	}
	_ = tw.Flush()
}

func printTrialBalance(w io.Writer, report *accounting.TrialBalance) {
	_, _ = fmt.Fprintf(w, "Trial balance as of %s\n", formatDate(report.AsOfDate))
	printAccountBalances(w, report.Accounts)
	_, _ = fmt.Fprintf(w, "Total debits: %s\n", report.TotalDebits.String())
	_, _ = fmt.Fprintf(w, "Total credits: %s\n", report.TotalCredits.String())
	_, _ = fmt.Fprintf(w, "Balanced: %t\n", report.IsBalanced)
}

func printAccountBalance(w io.Writer, report *accountBalanceReport) {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "ACCOUNT ID\tAS OF\tBALANCE")
	_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\n", report.AccountID, report.AsOfDate, report.Balance)
	_ = tw.Flush()
}

func printBalanceSheet(w io.Writer, report *accounting.BalanceSheet) {
	_, _ = fmt.Fprintf(w, "Balance sheet as of %s\n", formatDate(report.AsOfDate))
	printReportSection(w, "Assets", report.Assets)
	printReportSection(w, "Liabilities", report.Liabilities)
	printReportSection(w, "Equity", report.Equity)
	_, _ = fmt.Fprintf(w, "Retained earnings: %s\n", report.RetainedEarnings.String())
	_, _ = fmt.Fprintf(w, "Total assets: %s\n", report.TotalAssets.String())
	_, _ = fmt.Fprintf(w, "Total liabilities: %s\n", report.TotalLiabilities.String())
	_, _ = fmt.Fprintf(w, "Total equity: %s\n", report.TotalEquity.String())
	_, _ = fmt.Fprintf(w, "Balanced: %t\n", report.IsBalanced)
}

func printIncomeStatement(w io.Writer, report *accounting.IncomeStatement) {
	_, _ = fmt.Fprintf(w, "Income statement %s to %s\n", formatDate(report.StartDate), formatDate(report.EndDate))
	printReportSection(w, "Revenue", report.Revenue)
	printReportSection(w, "Expenses", report.Expenses)
	_, _ = fmt.Fprintf(w, "Total revenue: %s\n", report.TotalRevenue.String())
	_, _ = fmt.Fprintf(w, "Total expenses: %s\n", report.TotalExpenses.String())
	_, _ = fmt.Fprintf(w, "Net income: %s\n", report.NetIncome.String())
}

func printCashFlowStatement(w io.Writer, report *reports.CashFlowStatement) {
	_, _ = fmt.Fprintf(w, "Cash flow %s to %s\n", report.StartDate, report.EndDate)
	printCashFlowItems(w, "Operating activities", report.OperatingActivities)
	printCashFlowItems(w, "Investing activities", report.InvestingActivities)
	printCashFlowItems(w, "Financing activities", report.FinancingActivities)
	_, _ = fmt.Fprintf(w, "Total operating: %s\n", report.TotalOperating.String())
	_, _ = fmt.Fprintf(w, "Total investing: %s\n", report.TotalInvesting.String())
	_, _ = fmt.Fprintf(w, "Total financing: %s\n", report.TotalFinancing.String())
	_, _ = fmt.Fprintf(w, "Net cash change: %s\n", report.NetCashChange.String())
	_, _ = fmt.Fprintf(w, "Opening cash: %s\n", report.OpeningCash.String())
	_, _ = fmt.Fprintf(w, "Closing cash: %s\n", report.ClosingCash.String())
}

func printAgingReport(w io.Writer, report *analytics.AgingReport) {
	_, _ = fmt.Fprintf(w, "%s aging as of %s\n", titleLabel(report.ReportType), formatDate(report.AsOfDate))
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "BUCKET\tAMOUNT\tCOUNT")
	for _, bucket := range report.Buckets {
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%d\n", bucket.Label, bucket.Amount.String(), bucket.Count)
	}
	_ = tw.Flush()
	_, _ = fmt.Fprintf(w, "Total: %s\n", report.Total.String())

	if len(report.ByContact) == 0 {
		return
	}
	_, _ = fmt.Fprintln(w, "By contact:")
	tw = tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "CONTACT\tCURRENT\t1-30\t31-60\t61-90\t90+\tTOTAL")
	for _, contact := range report.ByContact {
		_, _ = fmt.Fprintf(
			tw,
			"%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			contact.ContactName,
			contact.Current.String(),
			contact.Days1to30.String(),
			contact.Days31to60.String(),
			contact.Days61to90.String(),
			contact.Days90Plus.String(),
			contact.Total.String(),
		)
	}
	_ = tw.Flush()
}

func printDashboardSummary(w io.Writer, summary *analytics.DashboardSummary) {
	_, _ = fmt.Fprintf(w, "Dashboard %s..%s\n", formatDate(summary.PeriodStart), formatDate(summary.PeriodEnd))
	_, _ = fmt.Fprintf(w, "Revenue: %s (%s%%)\n", summary.TotalRevenue.String(), summary.RevenueChange.String())
	_, _ = fmt.Fprintf(w, "Expenses: %s (%s%%)\n", summary.TotalExpenses.String(), summary.ExpensesChange.String())
	_, _ = fmt.Fprintf(w, "Net income: %s\n", summary.NetIncome.String())
	_, _ = fmt.Fprintf(w, "Receivables: %s (overdue %s)\n", summary.TotalReceivables.String(), summary.OverdueReceivables.String())
	_, _ = fmt.Fprintf(w, "Payables: %s (overdue %s)\n", summary.TotalPayables.String(), summary.OverduePayables.String())
	_, _ = fmt.Fprintf(w, "Invoices: %d draft, %d pending, %d overdue\n", summary.DraftInvoices, summary.PendingInvoices, summary.OverdueInvoices)
}

func printRevenueExpenseChart(w io.Writer, chart *analytics.RevenueExpenseChart) {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "PERIOD\tREVENUE\tEXPENSES\tPROFIT")
	for i, label := range chart.Labels {
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", label, decimalAt(chart.Revenue, i), decimalAt(chart.Expenses, i), decimalAt(chart.Profit, i))
	}
	_ = tw.Flush()
}

func printCashFlowChart(w io.Writer, chart *analytics.CashFlowChart) {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "PERIOD\tINFLOWS\tOUTFLOWS\tNET")
	for i, label := range chart.Labels {
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", label, decimalAt(chart.Inflows, i), decimalAt(chart.Outflows, i), decimalAt(chart.Net, i))
	}
	_ = tw.Flush()
}

func printActivityItems(w io.Writer, activity []analytics.ActivityItem) {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "ID\tTYPE\tACTION\tAMOUNT\tCREATED\tDESCRIPTION")
	for _, item := range activity {
		_, _ = fmt.Fprintf(
			tw,
			"%s\t%s\t%s\t%s\t%s\t%s\n",
			item.ID,
			item.Type,
			item.Action,
			formatDecimalPtr(item.Amount),
			item.CreatedAt.Format(time.RFC3339),
			item.Description,
		)
	}
	_ = tw.Flush()
}

func printBalanceConfirmationSummary(w io.Writer, report *reports.BalanceConfirmationSummary) {
	_, _ = fmt.Fprintf(w, "%s balance confirmations as of %s\n", report.Type, report.AsOfDate)
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "CONTACT\tCODE\tEMAIL\tBALANCE\tINVOICES\tOLDEST")
	for _, contact := range report.Contacts {
		_, _ = fmt.Fprintf(
			tw,
			"%s\t%s\t%s\t%s\t%d\t%s\n",
			contact.ContactName,
			contact.ContactCode,
			contact.ContactEmail,
			contact.Balance.String(),
			contact.InvoiceCount,
			contact.OldestInvoice,
		)
	}
	_ = tw.Flush()
	_, _ = fmt.Fprintf(w, "Contacts: %d\n", report.ContactCount)
	_, _ = fmt.Fprintf(w, "Invoices: %d\n", report.InvoiceCount)
	_, _ = fmt.Fprintf(w, "Total balance: %s\n", report.TotalBalance.String())
}

func printBalanceConfirmation(w io.Writer, report *reports.BalanceConfirmation) {
	_, _ = fmt.Fprintf(w, "%s balance confirmation for %s as of %s\n", report.Type, report.ContactName, report.AsOfDate)
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "INVOICE\tDATE\tDUE\tTOTAL\tPAID\tOUTSTANDING\tOVERDUE")
	for _, invoice := range report.Invoices {
		_, _ = fmt.Fprintf(
			tw,
			"%s\t%s\t%s\t%s\t%s\t%s\t%d\n",
			invoice.InvoiceNumber,
			invoice.InvoiceDate,
			invoice.DueDate,
			invoice.TotalAmount.String(),
			invoice.AmountPaid.String(),
			invoice.OutstandingAmount.String(),
			invoice.DaysOverdue,
		)
	}
	_ = tw.Flush()
	_, _ = fmt.Fprintf(w, "Total balance: %s\n", report.TotalBalance.String())
}

func printAccountBalances(w io.Writer, balances []accounting.AccountBalance) {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "CODE\tNAME\tTYPE\tDEBIT\tCREDIT\tNET")
	for _, balance := range balances {
		_, _ = fmt.Fprintf(
			tw,
			"%s\t%s\t%s\t%s\t%s\t%s\n",
			balance.AccountCode,
			balance.AccountName,
			balance.AccountType,
			balance.DebitBalance.String(),
			balance.CreditBalance.String(),
			balance.NetBalance.String(),
		)
	}
	_ = tw.Flush()
}

func printReportSection(w io.Writer, title string, balances []accounting.AccountBalance) {
	_, _ = fmt.Fprintf(w, "%s:\n", title)
	if len(balances) == 0 {
		_, _ = fmt.Fprintln(w, "  -")
		return
	}
	printAccountBalances(w, balances)
}

func printCashFlowItems(w io.Writer, title string, items []reports.CashFlowItem) {
	_, _ = fmt.Fprintf(w, "%s:\n", title)
	if len(items) == 0 {
		_, _ = fmt.Fprintln(w, "  -")
		return
	}
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "CODE\tDESCRIPTION\tAMOUNT")
	for _, item := range items {
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\n", item.Code, item.Description, item.Amount.String())
	}
	_ = tw.Flush()
}

func formatTimePtr(value *time.Time) string {
	if value == nil {
		return "-"
	}
	return value.Format(time.RFC3339)
}

func formatTime(value time.Time) string {
	if value.IsZero() {
		return "-"
	}
	return value.Format(time.RFC3339)
}

func formatDate(value time.Time) string {
	if value.IsZero() {
		return "-"
	}
	return value.Format("2006-01-02")
}

func formatDatePtr(value *time.Time) string {
	if value == nil {
		return "-"
	}
	return formatDate(*value)
}

func formatDecimalPtr(value *decimal.Decimal) string {
	if value == nil {
		return "-"
	}
	return value.String()
}

func decimalAt(values []decimal.Decimal, index int) string {
	if index < 0 || index >= len(values) {
		return "-"
	}
	return values[index].String()
}

func payslipEmployeeName(payslip payroll.Payslip) string {
	if payslip.Employee == nil {
		return payslip.EmployeeID
	}
	return strings.TrimSpace(payslip.Employee.FullName())
}

func invoiceContactLabel(invoice invoicing.Invoice) string {
	if invoice.Contact != nil && strings.TrimSpace(invoice.Contact.Name) != "" {
		return strings.TrimSpace(invoice.Contact.Name)
	}
	return invoice.ContactID
}

func quoteContactLabel(quote quotes.Quote) string {
	if quote.Contact != nil && strings.TrimSpace(quote.Contact.Name) != "" {
		return strings.TrimSpace(quote.Contact.Name)
	}
	return quote.ContactID
}

func orderContactLabel(order orders.Order) string {
	if order.Contact != nil && strings.TrimSpace(order.Contact.Name) != "" {
		return strings.TrimSpace(order.Contact.Name)
	}
	return order.ContactID
}

func recurringContactLabel(invoice recurring.RecurringInvoice) string {
	if strings.TrimSpace(invoice.ContactName) != "" {
		return strings.TrimSpace(invoice.ContactName)
	}
	return invoice.ContactID
}

func journalLineAccountLabel(line accounting.JournalEntryLine) string {
	if line.Account != nil {
		label := strings.TrimSpace(strings.TrimSpace(line.Account.Code) + " " + strings.TrimSpace(line.Account.Name))
		if label != "" {
			return label
		}
	}
	return line.AccountID
}

func titleLabel(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	return strings.ToUpper(trimmed[:1]) + strings.ToLower(trimmed[1:])
}

func tokenPreview(rawToken string) string {
	if len(rawToken) <= 14 {
		return rawToken
	}
	return rawToken[:14] + "..."
}

func normalizeSelector(selector string) string {
	return strings.ToLower(strings.TrimSpace(selector))
}
