package main

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/HMB-research/open-accounting/internal/accounting"
	"github.com/HMB-research/open-accounting/internal/apitoken"
	"github.com/HMB-research/open-accounting/internal/contacts"
	"github.com/HMB-research/open-accounting/internal/documents"
	"github.com/HMB-research/open-accounting/internal/payroll"
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

func printContactsTable(w io.Writer, contactsList []contacts.Contact) {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "ID\tNAME\tTYPE\tEMAIL\tACTIVE")
	for _, contact := range contactsList {
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%t\n", contact.ID, contact.Name, contact.ContactType, contact.Email, contact.IsActive)
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

func formatTimePtr(value *time.Time) string {
	if value == nil {
		return "-"
	}
	return value.Format(time.RFC3339)
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
