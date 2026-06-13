package main

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/shopspring/decimal"

	"github.com/HMB-research/open-accounting/internal/accounting"
	"github.com/HMB-research/open-accounting/internal/analytics"
	"github.com/HMB-research/open-accounting/internal/apitoken"
	"github.com/HMB-research/open-accounting/internal/assets"
	"github.com/HMB-research/open-accounting/internal/auth"
	"github.com/HMB-research/open-accounting/internal/banking"
	"github.com/HMB-research/open-accounting/internal/contacts"
	"github.com/HMB-research/open-accounting/internal/cutover"
	"github.com/HMB-research/open-accounting/internal/documents"
	"github.com/HMB-research/open-accounting/internal/email"
	"github.com/HMB-research/open-accounting/internal/expenses"
	"github.com/HMB-research/open-accounting/internal/inventory"
	"github.com/HMB-research/open-accounting/internal/invoicing"
	"github.com/HMB-research/open-accounting/internal/orders"
	"github.com/HMB-research/open-accounting/internal/payments"
	"github.com/HMB-research/open-accounting/internal/payroll"
	"github.com/HMB-research/open-accounting/internal/plugin"
	"github.com/HMB-research/open-accounting/internal/quotes"
	"github.com/HMB-research/open-accounting/internal/recurring"
	"github.com/HMB-research/open-accounting/internal/reports"
	"github.com/HMB-research/open-accounting/internal/tax"
	"github.com/HMB-research/open-accounting/internal/tenant"
	"github.com/HMB-research/open-accounting/internal/webhooks"
)

func printJSON(w io.Writer, value any) error {
	payload, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("encode json output: %w", err)
	}
	_, err = fmt.Fprintln(w, string(payload))
	return err
}

func printRawJSON(w io.Writer, payload json.RawMessage) error {
	var value any
	if err := json.Unmarshal(payload, &value); err != nil {
		_, writeErr := fmt.Fprintln(w, string(payload))
		return writeErr
	}
	return printJSON(w, value)
}

func printMigrationValidationReport(w io.Writer, report *cutover.BundleValidationReport) {
	if report == nil {
		_, _ = fmt.Fprintln(w, "No migration validation report")
		return
	}
	status := "ready"
	if !report.Summary.Ready {
		status = "blocked"
	}
	_, _ = fmt.Fprintf(
		w,
		"Migration validation: %s (%d files, %d rows, %d errors, %d warnings)\n",
		status,
		report.Summary.FilesValidated,
		report.Summary.RowsValidated,
		report.Summary.ErrorCount,
		report.Summary.WarningCount,
	)

	if len(report.Files) > 0 {
		tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
		_, _ = fmt.Fprintln(tw, "KIND\tFILE\tROWS\tMISSING COLUMNS")
		for _, file := range report.Files {
			missing := "-"
			if len(file.MissingColumns) > 0 {
				missing = strings.Join(file.MissingColumns, ",")
			}
			_, _ = fmt.Fprintf(tw, "%s\t%s\t%d\t%s\n", file.Kind, file.FileName, file.Rows, missing)
		}
		_ = tw.Flush()
	}

	printMigrationRemediationActions(w, report.RemediationActions)

	if len(report.Issues) == 0 {
		return
	}
	_, _ = fmt.Fprintln(w, "Issues:")
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "SEVERITY\tFILE\tROW\tFIELD\tMESSAGE")
	for _, issue := range report.Issues {
		row := "-"
		if issue.Row > 0 {
			row = strconv.Itoa(issue.Row)
		}
		field := issue.Field
		if field == "" {
			field = "-"
		}
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", issue.Severity, issue.FileName, row, field, issue.Message)
	}
	_ = tw.Flush()
}

func printMigrationRemediationActions(w io.Writer, actions []cutover.MigrationRemediationAction) {
	if len(actions) == 0 {
		return
	}
	_, _ = fmt.Fprintln(w, "Migration remediation actions")
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "SEVERITY\tKIND\tFILE\tFIELD\tCODE\tISSUES\tACTION\tCOMMAND")
	for _, action := range actions {
		kind := string(action.Kind)
		if kind == "" {
			kind = "-"
		}
		field := action.Field
		if field == "" {
			field = "-"
		}
		fileName := action.FileName
		if fileName == "" {
			fileName = "-"
		}
		_, _ = fmt.Fprintf(
			tw,
			"%s\t%s\t%s\t%s\t%s\t%d\t%s\t%s\n",
			action.Severity,
			kind,
			fileName,
			field,
			action.Code,
			action.IssueCount,
			action.Action,
			action.CLICommand,
		)
	}
	_ = tw.Flush()
}

func printLoginResponse(w io.Writer, resp *loginResponse) {
	_, _ = fmt.Fprintf(w, "Access token: %s\n", resp.AccessToken)
	if strings.TrimSpace(resp.RefreshToken) != "" {
		_, _ = fmt.Fprintf(w, "Refresh token: %s\n", resp.RefreshToken)
	}
	if strings.TrimSpace(resp.TokenType) != "" {
		_, _ = fmt.Fprintf(w, "Token type: %s\n", resp.TokenType)
	}
	if resp.ExpiresIn > 0 {
		_, _ = fmt.Fprintf(w, "Expires in: %d seconds\n", resp.ExpiresIn)
	}
	if resp.User != nil {
		_, _ = fmt.Fprintf(w, "User: %s <%s> (%s)\n", resp.User.Name, resp.User.Email, resp.User.ID)
	}
}

func printRefreshSessions(w io.Writer, sessions []refreshSession) {
	if len(sessions) == 0 {
		_, _ = fmt.Fprintln(w, "No refresh sessions found")
		return
	}
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "ID\tCREATED\tLAST USED\tEXPIRES\tSTATUS")
	for _, session := range sessions {
		status := "active"
		if session.RevokedAt != nil {
			status = "revoked"
		} else if time.Now().After(session.ExpiresAt) {
			status = "expired"
		}
		_, _ = fmt.Fprintf(
			tw,
			"%s\t%s\t%s\t%s\t%s\n",
			session.ID,
			session.CreatedAt.Format(time.RFC3339),
			formatTimePtr(session.LastUsedAt),
			session.ExpiresAt.Format(time.RFC3339),
			status,
		)
	}
	_ = tw.Flush()
}

func printSecurityAuditEvents(w io.Writer, events []auth.SecurityAuditEvent) {
	if len(events) == 0 {
		_, _ = fmt.Fprintln(w, "No security audit events found")
		return
	}
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "CREATED\tACTION\tACTOR\tTARGET\tIP\tMETADATA")
	for _, event := range events {
		_, _ = fmt.Fprintf(
			tw,
			"%s\t%s\t%s\t%s\t%s\t%s\n",
			event.CreatedAt.Format(time.RFC3339),
			event.Action,
			firstNonEmpty(event.ActorEmail, event.ActorUserID, "-"),
			firstNonEmpty(event.TargetEmail, event.TargetUserID, "-"),
			firstNonEmpty(event.RequestIP, "-"),
			formatStringMap(event.Metadata),
		)
	}
	_ = tw.Flush()
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

func printTenant(w io.Writer, tenantRecord *tenant.Tenant) {
	_, _ = fmt.Fprintf(w, "Tenant %s (%s)\n", tenantRecord.Name, tenantRecord.ID)
	if strings.TrimSpace(tenantRecord.Slug) != "" {
		_, _ = fmt.Fprintf(w, "Slug: %s\n", tenantRecord.Slug)
	}
	if strings.TrimSpace(tenantRecord.SchemaName) != "" {
		_, _ = fmt.Fprintf(w, "Schema: %s\n", tenantRecord.SchemaName)
	}
	_, _ = fmt.Fprintf(w, "Active: %t\n", tenantRecord.IsActive)
	_, _ = fmt.Fprintf(w, "Onboarding complete: %t\n", tenantRecord.OnboardingCompleted)
	_, _ = fmt.Fprintf(w, "Currency: %s\n", tenantRecord.Settings.DefaultCurrency)
	_, _ = fmt.Fprintf(w, "Country: %s\n", tenantRecord.Settings.CountryCode)
	_, _ = fmt.Fprintf(w, "Timezone: %s\n", tenantRecord.Settings.Timezone)
	_, _ = fmt.Fprintf(w, "Inventory issue costing: %s\n", tenant.EffectiveInventoryIssueCostingMethod(tenantRecord.Settings.InventoryIssueCostingMethod))
	_, _ = fmt.Fprintf(w, "Inventory valuation: %s\n", tenant.EffectiveInventoryValuationMethod(tenantRecord.Settings.InventoryValuationMethod))
	if strings.TrimSpace(tenantRecord.Settings.Email) != "" {
		_, _ = fmt.Fprintf(w, "Email: %s\n", tenantRecord.Settings.Email)
	}
	if strings.TrimSpace(tenantRecord.Settings.VATNumber) != "" {
		_, _ = fmt.Fprintf(w, "VAT number: %s\n", tenantRecord.Settings.VATNumber)
	}
	if strings.TrimSpace(tenantRecord.Settings.RegCode) != "" {
		_, _ = fmt.Fprintf(w, "Registration code: %s\n", tenantRecord.Settings.RegCode)
	}
	if tenantRecord.Settings.PeriodLockDate != nil && strings.TrimSpace(*tenantRecord.Settings.PeriodLockDate) != "" {
		_, _ = fmt.Fprintf(w, "Period lock date: %s\n", *tenantRecord.Settings.PeriodLockDate)
	}
}

func printTenantUsersTable(w io.Writer, users []tenant.TenantUser) {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "USER\tROLE\tDEFAULT\tACTIVE\tCREATED")
	for _, user := range users {
		_, _ = fmt.Fprintf(
			tw,
			"%s\t%s\t%t\t%t\t%s\n",
			user.UserID,
			user.Role,
			user.IsDefault,
			user.IsActive,
			user.CreatedAt.Format(time.RFC3339),
		)
	}
	_ = tw.Flush()
}

func printInvitationsTable(w io.Writer, invitations []tenant.UserInvitation) {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "ID\tEMAIL\tROLE\tTENANT\tEXPIRES\tCREATED")
	for _, invitation := range invitations {
		_, _ = fmt.Fprintf(
			tw,
			"%s\t%s\t%s\t%s\t%s\t%s\n",
			invitation.ID,
			invitation.Email,
			invitation.Role,
			firstNonEmpty(invitation.TenantName, invitation.TenantID),
			invitation.ExpiresAt.Format(time.RFC3339),
			invitation.CreatedAt.Format(time.RFC3339),
		)
	}
	_ = tw.Flush()
}

func printTenantAuditEventsTable(w io.Writer, events []tenant.TenantAuditEvent) {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "CREATED\tACTION\tACTOR\tTARGET\tEMAIL\tMETADATA")
	for _, event := range events {
		_, _ = fmt.Fprintf(
			tw,
			"%s\t%s\t%s\t%s\t%s\t%s\n",
			event.CreatedAt.Format(time.RFC3339),
			event.Action,
			event.ActorUserID,
			event.TargetType+":"+event.TargetID,
			event.TargetEmail,
			formatStringMap(event.Metadata),
		)
	}
	_ = tw.Flush()
}

func formatStringMap(values map[string]string) string {
	if len(values) == 0 {
		return ""
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+"="+values[key])
	}
	return strings.Join(parts, ", ")
}

func printTenantMembership(w io.Writer, membership *tenant.TenantMembership) {
	_, _ = fmt.Fprintf(w, "Joined tenant %s (%s) as %s\n", membership.Tenant.Name, membership.Tenant.ID, membership.Role)
	if strings.TrimSpace(membership.Tenant.Slug) != "" {
		_, _ = fmt.Fprintf(w, "Slug: %s\n", membership.Tenant.Slug)
	}
	_, _ = fmt.Fprintf(w, "Default: %t\n", membership.IsDefault)
}

func printTenantMembershipsTable(w io.Writer, memberships []tenant.TenantMembership) {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "ID\tNAME\tSLUG\tROLE\tDEFAULT")
	for _, membership := range memberships {
		_, _ = fmt.Fprintf(
			tw,
			"%s\t%s\t%s\t%s\t%t\n",
			membership.Tenant.ID,
			membership.Tenant.Name,
			membership.Tenant.Slug,
			membership.Role,
			membership.IsDefault,
		)
	}
	_ = tw.Flush()
}

func printPluginRegistriesTable(w io.Writer, registries []plugin.Registry) {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "ID\tNAME\tURL\tOFFICIAL\tACTIVE\tLAST SYNC")
	for _, registry := range registries {
		_, _ = fmt.Fprintf(
			tw,
			"%s\t%s\t%s\t%t\t%t\t%s\n",
			registry.ID,
			registry.Name,
			registry.URL,
			registry.IsOfficial,
			registry.IsActive,
			formatTimePtr(registry.LastSyncedAt),
		)
	}
	_ = tw.Flush()
}

func printPluginsTable(w io.Writer, plugins []plugin.Plugin) {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "ID\tNAME\tDISPLAY\tVERSION\tSTATE\tREPOSITORY")
	for _, item := range plugins {
		_, _ = fmt.Fprintf(
			tw,
			"%s\t%s\t%s\t%s\t%s\t%s\n",
			item.ID,
			item.Name,
			item.DisplayName,
			item.Version,
			item.State,
			item.RepositoryURL,
		)
	}
	_ = tw.Flush()
}

func printPluginSearchResultsTable(w io.Writer, results []plugin.PluginSearchResult) {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "NAME\tDISPLAY\tVERSION\tREGISTRY\tREPOSITORY")
	for _, result := range results {
		_, _ = fmt.Fprintf(
			tw,
			"%s\t%s\t%s\t%s\t%s\n",
			result.Plugin.Name,
			result.Plugin.DisplayName,
			result.Plugin.Version,
			result.Registry,
			result.Plugin.Repository,
		)
	}
	_ = tw.Flush()
}

func printPluginPermissionsTable(w io.Writer, permissions map[string]plugin.Permission) {
	keys := make([]string, 0, len(permissions))
	for key := range permissions {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "NAME\tCATEGORY\tRISK\tDESCRIPTION")
	for _, key := range keys {
		permission := permissions[key]
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", permission.Name, permission.Category, permission.Risk, permission.Description)
	}
	_ = tw.Flush()
}

func printTenantPluginsTable(w io.Writer, plugins []plugin.TenantPlugin) {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "PLUGIN\tNAME\tENABLED\tTENANT\tUPDATED")
	for _, tenantPlugin := range plugins {
		name := tenantPlugin.PluginID.String()
		if tenantPlugin.Plugin != nil && strings.TrimSpace(tenantPlugin.Plugin.DisplayName) != "" {
			name = tenantPlugin.Plugin.DisplayName
		}
		_, _ = fmt.Fprintf(
			tw,
			"%s\t%s\t%t\t%s\t%s\n",
			tenantPlugin.PluginID,
			name,
			tenantPlugin.IsEnabled,
			tenantPlugin.TenantID,
			tenantPlugin.UpdatedAt.Format(time.RFC3339),
		)
	}
	_ = tw.Flush()
}

func printWebhookEndpointsTable(w io.Writer, endpoints []webhooks.Endpoint) {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "ID\tNAME\tURL\tEVENTS\tACTIVE\tSECRET\tLAST DELIVERY")
	for _, endpoint := range endpoints {
		_, _ = fmt.Fprintf(
			tw,
			"%s\t%s\t%s\t%s\t%t\t%t\t%s\n",
			endpoint.ID,
			endpoint.Name,
			endpoint.URL,
			strings.Join(endpoint.Events, ","),
			endpoint.IsActive,
			endpoint.SecretSet,
			formatTimePtr(endpoint.LastDeliveryAt),
		)
	}
	_ = tw.Flush()
}

func printWebhookEndpoint(w io.Writer, endpoint *webhooks.Endpoint) {
	if endpoint == nil {
		return
	}
	_, _ = fmt.Fprintf(w, "Webhook endpoint %s (%s)\n", endpoint.Name, endpoint.ID)
	_, _ = fmt.Fprintf(w, "URL: %s\n", endpoint.URL)
	_, _ = fmt.Fprintf(w, "Events: %s\n", strings.Join(endpoint.Events, ", "))
	_, _ = fmt.Fprintf(w, "Active: %t\n", endpoint.IsActive)
	_, _ = fmt.Fprintf(w, "Secret set: %t\n", endpoint.SecretSet)
	if endpoint.LastDeliveryAt != nil {
		_, _ = fmt.Fprintf(w, "Last delivery: %s\n", endpoint.LastDeliveryAt.Format(time.RFC3339))
	}
}

func printWebhookDeliveriesTable(w io.Writer, deliveries []webhooks.Delivery) {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "DELIVERED\tEVENT\tSTATUS\tHTTP\tERROR")
	for _, delivery := range deliveries {
		_, _ = fmt.Fprintf(
			tw,
			"%s\t%s\t%s\t%d\t%s\n",
			delivery.DeliveredAt.Format(time.RFC3339),
			delivery.EventType,
			delivery.Status,
			delivery.StatusCode,
			delivery.Error,
		)
	}
	_ = tw.Flush()
}

func printWebhookDeliveryResult(w io.Writer, result *webhooks.DeliveryResult) {
	if result == nil {
		return
	}
	_, _ = fmt.Fprintf(w, "Webhook event %s (%s)\n", result.Event.Type, result.Event.ID)
	printWebhookDeliveriesTable(w, result.Deliveries)
}

func printAccountsTable(w io.Writer, accounts []accounting.Account) {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "ID\tCODE\tNAME\tTYPE\tACTIVE")
	for _, account := range accounts {
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%t\n", account.ID, account.Code, account.Name, account.AccountType, account.IsActive)
	}
	_ = tw.Flush()
}

func printAccountHierarchyTable(w io.Writer, rows []accounting.AccountHierarchyRow) {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "CODE\tNAME\tTYPE\tPARENT\tPATH\tACTIVE")
	for _, row := range rows {
		code := strings.Repeat("  ", row.Depth) + row.Code
		parent := row.ParentCode
		if strings.TrimSpace(parent) == "" {
			parent = "-"
		}
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%t\n", code, row.Name, row.AccountType, parent, row.Path, row.IsActive)
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
	_, _ = fmt.Fprintln(tw, "NO\tDESCRIPTION\tQTY\tUNIT\tUNIT PRICE\tVAT\tTREATMENT\tTOTAL")
	for _, line := range lines {
		_, _ = fmt.Fprintf(
			tw,
			"%d\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			line.LineNumber,
			line.Description,
			line.Quantity.String(),
			line.Unit,
			line.UnitPrice.String(),
			line.VATRate.String(),
			invoiceLineVATTreatmentLabel(line.VATTreatment),
			line.LineTotal.String(),
		)
	}
	_ = tw.Flush()
}

func invoiceLineVATTreatmentLabel(treatment invoicing.VATTreatment) string {
	if treatment == "" {
		return string(invoicing.VATTreatmentStandard)
	}
	return string(treatment)
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

func printOrderStockCheck(w io.Writer, check *orders.OrderStockCheck) {
	if check == nil {
		return
	}

	_, _ = fmt.Fprintf(w, "Order stock check %s\n", check.OrderNumber)
	if strings.TrimSpace(check.WarehouseID) != "" {
		_, _ = fmt.Fprintf(w, "Warehouse: %s\n", check.WarehouseID)
	}
	_, _ = fmt.Fprintf(w, "Ready: %t\n\n", check.Ready)

	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "LINE\tPRODUCT\tREQUIRED\tAVAILABLE\tSHORTAGE\tSTATUS")
	for _, line := range check.Lines {
		_, _ = fmt.Fprintf(
			tw,
			"%d\t%s\t%s\t%s\t%s\t%s\n",
			line.LineNumber,
			orderStockProductLabel(line),
			line.RequiredQty.String(),
			line.AvailableQty.String(),
			line.ShortageQty.String(),
			line.Status,
		)
	}
	_ = tw.Flush()
}

func orderStockProductLabel(line orders.OrderStockCheckLine) string {
	if strings.TrimSpace(line.ProductCode) != "" || strings.TrimSpace(line.ProductName) != "" {
		return strings.TrimSpace(strings.TrimSpace(line.ProductCode) + " " + strings.TrimSpace(line.ProductName))
	}
	if strings.TrimSpace(line.ProductID) != "" {
		return line.ProductID
	}
	return line.Description
}

func printOrderPickList(w io.Writer, pickList *orders.OrderPickList) {
	if pickList == nil {
		return
	}

	_, _ = fmt.Fprintf(w, "Order pick list %s\n", pickList.OrderNumber)
	_, _ = fmt.Fprintf(w, "Warehouse: %s\n", pickList.WarehouseID)
	_, _ = fmt.Fprintf(w, "Ready: %t\n\n", pickList.Ready)

	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "LINE\tPRODUCT\tREQUIRED\tRESERVED\tPICK\tAVAILABLE\tSHORTAGE\tSTATUS")
	for _, line := range pickList.Lines {
		_, _ = fmt.Fprintf(
			tw,
			"%d\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			line.LineNumber,
			orderPickListProductLabel(line),
			line.RequiredQty.String(),
			line.ReservedQty.String(),
			line.PickQty.String(),
			line.AvailableQty.String(),
			line.ShortageQty.String(),
			line.Status,
		)
	}
	_ = tw.Flush()
}

func orderPickListProductLabel(line orders.OrderPickListLine) string {
	if strings.TrimSpace(line.ProductCode) != "" || strings.TrimSpace(line.ProductName) != "" {
		return strings.TrimSpace(strings.TrimSpace(line.ProductCode) + " " + strings.TrimSpace(line.ProductName))
	}
	if strings.TrimSpace(line.ProductID) != "" {
		return line.ProductID
	}
	return line.Description
}

func printOrderStockReservation(w io.Writer, result *orders.OrderStockReservationResult) {
	if result == nil {
		return
	}

	action := strings.ToLower(result.Action)
	switch result.Action {
	case orders.OrderStockReservationActionReserve:
		action = "reserved"
	case orders.OrderStockReservationActionRelease:
		action = "released"
	}
	_, _ = fmt.Fprintf(w, "Order stock %s %s\n", action, result.OrderNumber)
	_, _ = fmt.Fprintf(w, "Warehouse: %s\n\n", result.WarehouseID)

	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "PRODUCT\tQUANTITY\tRESERVED\tAVAILABLE\tSTATUS")
	for _, line := range result.Lines {
		_, _ = fmt.Fprintf(
			tw,
			"%s\t%s\t%s\t%s\t%s\n",
			orderStockReservationProductLabel(line),
			line.Quantity.String(),
			line.ReservedQty.String(),
			line.AvailableQty.String(),
			line.Status,
		)
	}
	_ = tw.Flush()
}

func orderStockReservationProductLabel(line orders.OrderStockReservationLine) string {
	if strings.TrimSpace(line.ProductCode) != "" || strings.TrimSpace(line.ProductName) != "" {
		return strings.TrimSpace(strings.TrimSpace(line.ProductCode) + " " + strings.TrimSpace(line.ProductName))
	}
	return line.ProductID
}

func printOrderStockReservations(w io.Writer, reservations []orders.OrderStockReservation) {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "ID\tPRODUCT\tWAREHOUSE\tQUANTITY\tSTATUS\tUPDATED")
	for _, reservation := range reservations {
		_, _ = fmt.Fprintf(
			tw,
			"%s\t%s\t%s\t%s\t%s\t%s\n",
			reservation.ID,
			reservation.ProductID,
			reservation.WarehouseID,
			reservation.Quantity.String(),
			reservation.Status,
			formatTime(reservation.UpdatedAt),
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

func printExpensesTable(w io.Writer, expenseList []expenses.Expense) {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "ID\tNUMBER\tDATE\tMERCHANT\tSTATUS\tAMOUNT\tCURRENCY\tRECEIPT\tJOURNAL")
	for _, expense := range expenseList {
		journalID := ""
		if expense.JournalEntryID != nil {
			journalID = *expense.JournalEntryID
		}
		receipt := "no"
		if expense.RequiresReceipt {
			receipt = "yes"
		}
		_, _ = fmt.Fprintf(
			tw,
			"%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			expense.ID,
			expense.ExpenseNumber,
			formatDate(expense.ExpenseDate),
			expense.Merchant,
			expense.Status,
			expense.Amount.String(),
			expense.Currency,
			receipt,
			journalID,
		)
	}
	_ = tw.Flush()
}

func printExpense(w io.Writer, expense *expenses.Expense) {
	_, _ = fmt.Fprintf(w, "Expense %s %s (%s)\n", expense.ExpenseNumber, expense.Merchant, expense.Status)
	_, _ = fmt.Fprintf(w, "ID: %s\n", expense.ID)
	_, _ = fmt.Fprintf(w, "Date: %s\n", formatDate(expense.ExpenseDate))
	if strings.TrimSpace(expense.Description) != "" {
		_, _ = fmt.Fprintf(w, "Description: %s\n", expense.Description)
	}
	if expense.EmployeeID != nil && strings.TrimSpace(*expense.EmployeeID) != "" {
		_, _ = fmt.Fprintf(w, "Employee: %s\n", *expense.EmployeeID)
	}
	if expense.ContactID != nil && strings.TrimSpace(*expense.ContactID) != "" {
		_, _ = fmt.Fprintf(w, "Contact: %s\n", *expense.ContactID)
	}
	_, _ = fmt.Fprintf(w, "Expense account: %s\n", expense.ExpenseAccountID)
	_, _ = fmt.Fprintf(w, "Payment account: %s\n", expense.PaymentAccountID)
	_, _ = fmt.Fprintf(w, "Amount: %s %s\n", expense.Amount.String(), expense.Currency)
	_, _ = fmt.Fprintf(w, "Exchange rate: %s\n", expense.ExchangeRate.String())
	_, _ = fmt.Fprintf(w, "Base amount: %s\n", expense.BaseAmount.String())
	_, _ = fmt.Fprintf(w, "Requires receipt: %t\n", expense.RequiresReceipt)
	if expense.JournalEntryID != nil && strings.TrimSpace(*expense.JournalEntryID) != "" {
		_, _ = fmt.Fprintf(w, "Journal entry: %s\n", *expense.JournalEntryID)
	}
	if strings.TrimSpace(expense.RejectionReason) != "" {
		_, _ = fmt.Fprintf(w, "Rejection reason: %s\n", expense.RejectionReason)
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
	if !asset.DisposalProceeds.IsZero() {
		_, _ = fmt.Fprintf(w, "Disposal proceeds: %s\n", asset.DisposalProceeds.String())
	}
	if strings.TrimSpace(asset.DisposalNotes) != "" {
		_, _ = fmt.Fprintf(w, "Disposal notes: %s\n", asset.DisposalNotes)
	}
	if asset.DisposalJournalEntryID != nil && strings.TrimSpace(*asset.DisposalJournalEntryID) != "" {
		_, _ = fmt.Fprintf(w, "Disposal journal: %s\n", *asset.DisposalJournalEntryID)
	}
}

func printDepreciationEntriesTable(w io.Writer, entries []assets.DepreciationEntry) {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "ID\tDATE\tPERIOD\tAMOUNT\tACCUMULATED\tBOOK VALUE\tJOURNAL")
	for _, entry := range entries {
		journalID := ""
		if entry.JournalEntryID != nil {
			journalID = strings.TrimSpace(*entry.JournalEntryID)
		}
		_, _ = fmt.Fprintf(
			tw,
			"%s\t%s\t%s..%s\t%s\t%s\t%s\t%s\n",
			entry.ID,
			formatDate(entry.DepreciationDate),
			formatDate(entry.PeriodStart),
			formatDate(entry.PeriodEnd),
			entry.DepreciationAmount.String(),
			entry.AccumulatedTotal.String(),
			entry.BookValueAfter.String(),
			journalID,
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
	_, _ = fmt.Fprintln(tw, "ID\tDATE\tTYPE\tPRODUCT\tWAREHOUSE\tQTY\tUNIT COST\tLOT\tSERIAL\tEXPIRY\tREFERENCE\tNOTES")
	for _, movement := range movements {
		_, _ = fmt.Fprintf(
			tw,
			"%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			movement.ID,
			formatDate(movement.MovementDate),
			movement.MovementType,
			movement.ProductID,
			movement.WarehouseID,
			movement.Quantity.String(),
			movement.UnitCost.String(),
			formatOptionalString(movement.LotNumber),
			formatOptionalString(movement.SerialNumber),
			formatOptionalString(movement.ExpiryDate),
			movement.Reference,
			movement.Notes,
		)
	}
	_ = tw.Flush()
}

func printInventoryValuation(w io.Writer, report *inventory.InventoryValuationReport) {
	if report == nil {
		return
	}

	_, _ = fmt.Fprintf(w, "Inventory valuation (%s)\n", report.ValuationMethod)
	if strings.TrimSpace(report.WarehouseID) != "" {
		_, _ = fmt.Fprintf(w, "Warehouse: %s\n", report.WarehouseID)
	}
	_, _ = fmt.Fprintf(w, "Generated: %s\n\n", formatTime(report.GeneratedAt))

	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "PRODUCT\tWAREHOUSE\tQUANTITY\tRESERVED\tAVAILABLE\tUNIT COST\tVALUE")
	for _, line := range report.Lines {
		_, _ = fmt.Fprintf(
			tw,
			"%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			inventoryValuationProductLabel(line),
			inventoryValuationWarehouseLabel(line),
			line.Quantity.String(),
			line.ReservedQty.String(),
			line.AvailableQty.String(),
			line.UnitCost.String(),
			line.InventoryValue.String(),
		)
	}
	_, _ = fmt.Fprintf(
		tw,
		"TOTAL\t\t%s\t%s\t%s\t\t%s\n",
		report.TotalQuantity.String(),
		report.TotalReserved.String(),
		report.TotalAvailable.String(),
		report.TotalValue.String(),
	)
	_ = tw.Flush()
}

func inventoryValuationProductLabel(line inventory.InventoryValuationLine) string {
	if strings.TrimSpace(line.ProductCode) != "" && strings.TrimSpace(line.ProductName) != "" {
		return line.ProductCode + " " + line.ProductName
	}
	if strings.TrimSpace(line.ProductCode) != "" {
		return line.ProductCode
	}
	if strings.TrimSpace(line.ProductName) != "" {
		return line.ProductName
	}
	return line.ProductID
}

func inventoryValuationWarehouseLabel(line inventory.InventoryValuationLine) string {
	if strings.TrimSpace(line.WarehouseCode) != "" && strings.TrimSpace(line.WarehouseName) != "" {
		return line.WarehouseCode + " " + line.WarehouseName
	}
	if strings.TrimSpace(line.WarehouseCode) != "" {
		return line.WarehouseCode
	}
	if strings.TrimSpace(line.WarehouseName) != "" {
		return line.WarehouseName
	}
	return line.WarehouseID
}

func printInventorySubledgerReconciliation(w io.Writer, report *inventory.InventorySubledgerReconciliationReport) {
	if report == nil {
		return
	}

	_, _ = fmt.Fprintf(w, "Inventory subledger reconciliation (%s)\n", report.ValuationMethod)
	if strings.TrimSpace(report.WarehouseID) != "" {
		_, _ = fmt.Fprintf(w, "Warehouse: %s\n", report.WarehouseID)
	}
	_, _ = fmt.Fprintf(w, "As of: %s\n", formatDate(report.AsOfDate))
	_, _ = fmt.Fprintf(w, "Ready: %t\n", report.Ready)
	_, _ = fmt.Fprintf(w, "Generated: %s\n\n", formatTime(report.GeneratedAt))

	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "ACCOUNT\tSUBLEDGER\tGL BALANCE\tDIFFERENCE\tBALANCED\tLINES")
	for _, line := range report.AccountLines {
		_, _ = fmt.Fprintf(
			tw,
			"%s\t%s\t%s\t%s\t%t\t%d\n",
			inventorySubledgerAccountLabel(line),
			line.SubledgerValue.String(),
			line.GeneralLedgerBalance.String(),
			line.Difference.String(),
			line.Balanced,
			line.ProductLineCount,
		)
	}
	_, _ = fmt.Fprintf(
		tw,
		"TOTAL\t%s\t%s\t%s\t%t\t\n",
		report.TotalSubledgerValue.String(),
		report.TotalGeneralLedgerBalance.String(),
		report.TotalDifference.String(),
		report.Ready,
	)
	_ = tw.Flush()

	if report.BlockingExceptionLineCount == 0 {
		return
	}

	_, _ = fmt.Fprintln(w, "\nInventory subledger exceptions")
	tw = tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "PRODUCT\tWAREHOUSE\tSTATUS\tQTY\tVALUE\tACCOUNT")
	for _, line := range report.Lines {
		if line.Status == "MAPPED" {
			continue
		}
		_, _ = fmt.Fprintf(
			tw,
			"%s\t%s\t%s\t%s\t%s\t%s\n",
			inventorySubledgerProductLabel(line),
			inventorySubledgerWarehouseLabel(line),
			line.Status,
			line.Quantity.String(),
			line.InventoryValue.String(),
			inventorySubledgerLineAccountLabel(line),
		)
	}
	_ = tw.Flush()
}

func inventorySubledgerAccountLabel(line inventory.InventorySubledgerReconciliationAccountLine) string {
	if strings.TrimSpace(line.AccountCode) != "" && strings.TrimSpace(line.AccountName) != "" {
		return line.AccountCode + " " + line.AccountName
	}
	if strings.TrimSpace(line.AccountCode) != "" {
		return line.AccountCode
	}
	if strings.TrimSpace(line.AccountName) != "" {
		return line.AccountName
	}
	return line.AccountID
}

func inventorySubledgerProductLabel(line inventory.InventorySubledgerReconciliationLine) string {
	if strings.TrimSpace(line.ProductCode) != "" && strings.TrimSpace(line.ProductName) != "" {
		return line.ProductCode + " " + line.ProductName
	}
	if strings.TrimSpace(line.ProductCode) != "" {
		return line.ProductCode
	}
	if strings.TrimSpace(line.ProductName) != "" {
		return line.ProductName
	}
	return line.ProductID
}

func inventorySubledgerWarehouseLabel(line inventory.InventorySubledgerReconciliationLine) string {
	if strings.TrimSpace(line.WarehouseCode) != "" && strings.TrimSpace(line.WarehouseName) != "" {
		return line.WarehouseCode + " " + line.WarehouseName
	}
	if strings.TrimSpace(line.WarehouseCode) != "" {
		return line.WarehouseCode
	}
	if strings.TrimSpace(line.WarehouseName) != "" {
		return line.WarehouseName
	}
	if strings.TrimSpace(line.WarehouseID) != "" {
		return line.WarehouseID
	}
	return "All warehouses"
}

func inventorySubledgerLineAccountLabel(line inventory.InventorySubledgerReconciliationLine) string {
	if strings.TrimSpace(line.AccountCode) != "" && strings.TrimSpace(line.AccountName) != "" {
		return line.AccountCode + " " + line.AccountName
	}
	if strings.TrimSpace(line.InventoryAccountID) != "" {
		return line.InventoryAccountID
	}
	return "-"
}

func printInventoryLotReport(w io.Writer, report *inventory.InventoryLotReport) {
	if report == nil {
		return
	}

	_, _ = fmt.Fprintln(w, "Inventory lots")
	if strings.TrimSpace(report.ProductID) != "" {
		_, _ = fmt.Fprintf(w, "Product: %s\n", report.ProductID)
	}
	if strings.TrimSpace(report.WarehouseID) != "" {
		_, _ = fmt.Fprintf(w, "Warehouse: %s\n", report.WarehouseID)
	}
	if report.IncludeEmpty {
		_, _ = fmt.Fprintln(w, "Including empty positions")
	}
	_, _ = fmt.Fprintf(w, "Generated: %s\n\n", formatTime(report.GeneratedAt))

	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "PRODUCT\tWAREHOUSE\tLOT\tSERIAL\tEXPIRY\tQUANTITY\tUNIT COST\tVALUE\tLAST MOVEMENT")
	for _, line := range report.Lines {
		_, _ = fmt.Fprintf(
			tw,
			"%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			inventoryLotProductLabel(line),
			inventoryLotWarehouseLabel(line),
			formatOptionalString(line.LotNumber),
			formatOptionalString(line.SerialNumber),
			formatOptionalString(line.ExpiryDate),
			line.Quantity.String(),
			line.UnitCost.String(),
			line.InventoryValue.String(),
			formatTime(line.LastMovementDate),
		)
	}
	_, _ = fmt.Fprintf(
		tw,
		"TOTAL\t\t\t\t\t%s\t\t%s\t\n",
		report.TotalQuantity.String(),
		report.TotalValue.String(),
	)
	_ = tw.Flush()
}

func inventoryLotProductLabel(line inventory.InventoryLotLine) string {
	if strings.TrimSpace(line.ProductCode) != "" && strings.TrimSpace(line.ProductName) != "" {
		return line.ProductCode + " " + line.ProductName
	}
	if strings.TrimSpace(line.ProductCode) != "" {
		return line.ProductCode
	}
	if strings.TrimSpace(line.ProductName) != "" {
		return line.ProductName
	}
	return line.ProductID
}

func inventoryLotWarehouseLabel(line inventory.InventoryLotLine) string {
	if strings.TrimSpace(line.WarehouseCode) != "" && strings.TrimSpace(line.WarehouseName) != "" {
		return line.WarehouseCode + " " + line.WarehouseName
	}
	if strings.TrimSpace(line.WarehouseCode) != "" {
		return line.WarehouseCode
	}
	if strings.TrimSpace(line.WarehouseName) != "" {
		return line.WarehouseName
	}
	return line.WarehouseID
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
	printCostCenterBudgetReport(w, report, "Cost center report")
}

func printBudgetVsActualReport(w io.Writer, report *accounting.CostCenterReport) {
	printCostCenterBudgetReport(w, report, "Budget vs actual report")
}

func printCostCenterBudgetReport(w io.Writer, report *accounting.CostCenterReport, title string) {
	_, _ = fmt.Fprintf(w, "%s %s..%s\n", title, formatDate(report.PeriodStart), formatDate(report.PeriodEnd))
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

func printCostAllocationsTable(w io.Writer, allocations []accounting.CostAllocation) {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "ID\tDATE\tCOST CENTER\tJOURNAL LINE\tAMOUNT\tPERCENT\tNOTES")
	for _, allocation := range allocations {
		costCenterLabel := allocation.CostCenterID
		if allocation.CostCenterCode != "" || allocation.CostCenterName != "" {
			costCenterLabel = strings.TrimSpace(allocation.CostCenterCode + " " + allocation.CostCenterName)
		}
		_, _ = fmt.Fprintf(
			tw,
			"%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			allocation.ID,
			formatDate(allocation.AllocationDate),
			costCenterLabel,
			allocation.JournalEntryLineID,
			allocation.Amount.String(),
			formatDecimalPtr(allocation.AllocationPercentage),
			allocation.Notes,
		)
	}
	_ = tw.Flush()
}

func printJournalEntriesTable(w io.Writer, entries []accounting.JournalEntry) {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "ID\tNUMBER\tDATE\tSTATUS\tEVIDENCE\tDEBIT\tCREDIT\tREFERENCE\tDESCRIPTION")
	for _, entry := range entries {
		_, _ = fmt.Fprintf(
			tw,
			"%s\t%s\t%s\t%s\t%t\t%s\t%s\t%s\t%s\n",
			entry.ID,
			entry.EntryNumber,
			formatDate(entry.EntryDate),
			entry.Status,
			entry.RequiresEvidence,
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
	_, _ = fmt.Fprintf(w, "Requires evidence: %t\n", entry.RequiresEvidence)
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

func printJournalEntryTemplatesTable(w io.Writer, templates []accounting.JournalEntryTemplate) {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "ID\tNAME\tACTIVE\tEVIDENCE\tLINES\tREFERENCE\tDESCRIPTION")
	for _, template := range templates {
		_, _ = fmt.Fprintf(
			tw,
			"%s\t%s\t%t\t%t\t%d\t%s\t%s\n",
			template.ID,
			template.Name,
			template.IsActive,
			template.RequiresEvidence,
			template.LineCount,
			template.Reference,
			template.Description,
		)
	}
	_ = tw.Flush()
}

func printJournalEntryTemplate(w io.Writer, template *accounting.JournalEntryTemplate) {
	_, _ = fmt.Fprintf(w, "Journal entry template %s\n", template.Name)
	_, _ = fmt.Fprintf(w, "ID: %s\n", template.ID)
	_, _ = fmt.Fprintf(w, "Description: %s\n", template.Description)
	if strings.TrimSpace(template.Reference) != "" {
		_, _ = fmt.Fprintf(w, "Reference: %s\n", template.Reference)
	}
	_, _ = fmt.Fprintf(w, "Active: %t\n", template.IsActive)
	_, _ = fmt.Fprintf(w, "Requires evidence: %t\n", template.RequiresEvidence)
	_, _ = fmt.Fprintf(w, "Lines: %d\n", template.LineCount)
	_, _ = fmt.Fprintf(w, "Total debits: %s\n", journalTemplateTotalDebit(template.Lines).String())
	_, _ = fmt.Fprintf(w, "Total credits: %s\n", journalTemplateTotalCredit(template.Lines).String())
	if len(template.Lines) > 0 {
		printJournalEntryTemplateLinesTable(w, template.Lines)
	}
}

func printJournalEntryTemplateLinesTable(w io.Writer, lines []accounting.JournalEntryTemplateLine) {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "NO\tACCOUNT\tDESCRIPTION\tDEBIT\tCREDIT\tCURRENCY")
	for _, line := range lines {
		_, _ = fmt.Fprintf(
			tw,
			"%d\t%s\t%s\t%s\t%s\t%s\n",
			line.LineNumber,
			line.AccountID,
			line.Description,
			line.DebitAmount.Mul(line.ExchangeRate).String(),
			line.CreditAmount.Mul(line.ExchangeRate).String(),
			line.Currency,
		)
	}
	_ = tw.Flush()
}

func journalTemplateTotalDebit(lines []accounting.JournalEntryTemplateLine) decimal.Decimal {
	total := decimal.Zero
	for _, line := range lines {
		total = total.Add(line.DebitAmount.Mul(line.ExchangeRate))
	}
	return total
}

func journalTemplateTotalCredit(lines []accounting.JournalEntryTemplateLine) decimal.Decimal {
	total := decimal.Zero
	for _, line := range lines {
		total = total.Add(line.CreditAmount.Mul(line.ExchangeRate))
	}
	return total
}

func printJournalEntryTemplateGenerationResults(w io.Writer, results []accounting.JournalEntryTemplateGenerationResult) {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "TEMPLATE\tENTRY\tNUMBER\tENTRY DATE\tNEXT DATE\tSTATUS\tERROR")
	for _, result := range results {
		_, _ = fmt.Fprintf(
			tw,
			"%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			result.TemplateID,
			result.GeneratedEntryID,
			result.GeneratedEntryNumber,
			formatDatePtr(result.EntryDate),
			formatDatePtr(result.NextGenerationDate),
			result.Status,
			result.Error,
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

func printSalaryComponentsTable(w io.Writer, components []payroll.SalaryComponent) {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "ID\tTYPE\tNAME\tAMOUNT\tTAXABLE\tRECURRING\tFROM\tTO")
	for _, component := range components {
		_, _ = fmt.Fprintf(
			tw,
			"%s\t%s\t%s\t%s\t%t\t%t\t%s\t%s\n",
			component.ID,
			component.ComponentType,
			component.Name,
			component.Amount.String(),
			component.IsTaxable,
			component.IsRecurring,
			formatDate(component.EffectiveFrom),
			formatDatePtr(component.EffectiveTo),
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

func printDocumentReviewSummariesTable(w io.Writer, summaries []documents.ReviewSummary) {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "ENTITY\tID\tTOTAL\tPENDING\tREVIEWED\tAPPROVED\tREJECTED\tMISSING\tHAS PENDING\tHAS REJECTED")
	for _, summary := range summaries {
		_, _ = fmt.Fprintf(
			tw,
			"%s\t%s\t%d\t%d\t%d\t%d\t%d\t%t\t%t\t%t\n",
			summary.EntityType,
			summary.EntityID,
			summary.TotalCount,
			summary.PendingReviewCount,
			summary.ReviewedCount,
			summary.ApprovedCount,
			summary.RejectedCount,
			summary.MissingEvidence,
			summary.HasPendingReview,
			summary.HasRejected,
		)
	}
	_ = tw.Flush()
}

func printDocumentReviewQueue(w io.Writer, queue *documents.ReviewQueue) {
	if queue == nil {
		return
	}
	entityType := queue.EntityType
	if strings.TrimSpace(entityType) == "" {
		entityType = "all"
	}
	documentType := queue.DocumentType
	if strings.TrimSpace(documentType) == "" {
		documentType = "all"
	}
	_, _ = fmt.Fprintf(w, "Document review queue: status %s, entity %s, document type %s, limit %d\n",
		queue.ReviewStatus,
		entityType,
		documentType,
		queue.Limit,
	)
	_, _ = fmt.Fprintf(w, "Total: %d, pending: %d, reviewed: %d, approved: %d, rejected: %d\n",
		queue.TotalCount,
		queue.PendingReviewCount,
		queue.ReviewedCount,
		queue.ApprovedCount,
		queue.RejectedCount,
	)
	if len(queue.Documents) == 0 {
		return
	}
	printDocumentsTable(w, queue.Documents)
}

func printDocumentEvidencePolicy(w io.Writer, results []documents.EvidencePolicyResult) {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "ENTITY\tID\tCOMPLIANT\tTOTAL\tPENDING\tAPPROVED\tREJECTED\tVIOLATIONS")
	for _, result := range results {
		_, _ = fmt.Fprintf(
			tw,
			"%s\t%s\t%t\t%d\t%d\t%d\t%d\t%d\n",
			result.EntityType,
			result.EntityID,
			result.Compliant,
			result.TotalCount,
			result.PendingReviewCount,
			result.ApprovedCount,
			result.RejectedCount,
			len(result.Violations),
		)
	}
	_ = tw.Flush()

	for _, result := range results {
		for _, violation := range result.Violations {
			_, _ = fmt.Fprintf(w, "%s:%s rule %d: %s\n", result.EntityType, result.EntityID, violation.RuleIndex, violation.Message)
		}
	}
}

func printDocumentRetentionReview(w io.Writer, review *documents.RetentionReview) {
	if review == nil {
		return
	}
	_, _ = fmt.Fprintf(w, "Document retention review as of %s, cutoff %s\n", review.AsOfDate, review.CutoffDate)
	_, _ = fmt.Fprintf(w, "Total: %d, expired: %d, due soon: %d, missing retention: %d, pending review: %d, rejected: %d\n",
		review.TotalCount,
		review.ExpiredCount,
		review.DueSoonCount,
		review.MissingRetentionCount,
		review.PendingReviewCount,
		review.RejectedCount,
	)
	if len(review.Documents) == 0 {
		return
	}

	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "ID\tENTITY\tTYPE\tFILE\tREVIEW\tRETENTION\tCREATED")
	for _, doc := range review.Documents {
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

	if len(review.ReminderActions) == 0 {
		return
	}

	_, _ = fmt.Fprintln(w, "Reminder actions")
	actionWriter := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(actionWriter, "ACTION\tDOCUMENT\tRETENTION\tDAYS\tMESSAGE")
	for _, action := range review.ReminderActions {
		_, _ = fmt.Fprintf(
			actionWriter,
			"%s\t%s\t%s\t%s\t%s\n",
			action.Action,
			action.DocumentID,
			formatDatePtr(action.RetentionUntil),
			formatIntPtr(action.DaysUntilRetention),
			action.Message,
		)
	}
	_ = actionWriter.Flush()
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

func printOverdueInvoicesSummary(w io.Writer, summary *invoicing.OverdueInvoicesSummary) {
	_, _ = fmt.Fprintf(w, "Overdue invoices as of %s\n", formatTime(summary.GeneratedAt))
	_, _ = fmt.Fprintf(w, "Total overdue: %s\n", summary.TotalOverdue.String())
	_, _ = fmt.Fprintf(w, "Invoices: %d\n", summary.InvoiceCount)
	_, _ = fmt.Fprintf(w, "Contacts: %d\n", summary.ContactCount)
	_, _ = fmt.Fprintf(w, "Average days overdue: %d\n", summary.AverageDaysOverdue)
	if len(summary.Invoices) > 0 {
		printOverdueInvoicesTable(w, summary.Invoices)
	}
}

func printOverdueInvoicesTable(w io.Writer, invoices []invoicing.OverdueInvoice) {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "ID\tNUMBER\tCONTACT\tDUE\tOUTSTANDING\tCURRENCY\tDAYS\tREMINDERS")
	for _, invoice := range invoices {
		_, _ = fmt.Fprintf(
			tw,
			"%s\t%s\t%s\t%s\t%s\t%s\t%d\t%d\n",
			invoice.ID,
			invoice.InvoiceNumber,
			invoice.ContactName,
			invoice.DueDate,
			invoice.OutstandingAmount.String(),
			invoice.Currency,
			invoice.DaysOverdue,
			invoice.ReminderCount,
		)
	}
	_ = tw.Flush()
}

func printPaymentRemindersTable(w io.Writer, reminders []invoicing.PaymentReminder) {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "ID\tINVOICE\tCONTACT\tNUMBER\tSTATUS\tSENT")
	for _, reminder := range reminders {
		_, _ = fmt.Fprintf(
			tw,
			"%s\t%s\t%s\t%d\t%s\t%s\n",
			reminder.ID,
			reminder.InvoiceNumber,
			reminder.ContactName,
			reminder.ReminderNumber,
			reminder.Status,
			formatTimePtr(reminder.SentAt),
		)
	}
	_ = tw.Flush()
}

func printReminderResult(w io.Writer, result *invoicing.ReminderResult) {
	_, _ = fmt.Fprintf(w, "Reminder for invoice %s (%s)\n", result.InvoiceID, result.InvoiceNumber)
	_, _ = fmt.Fprintf(w, "Success: %t\n", result.Success)
	if strings.TrimSpace(result.ReminderID) != "" {
		_, _ = fmt.Fprintf(w, "Reminder ID: %s\n", result.ReminderID)
	}
	if strings.TrimSpace(result.Message) != "" {
		_, _ = fmt.Fprintf(w, "Message: %s\n", result.Message)
	}
}

func printBulkReminderResult(w io.Writer, result *invoicing.BulkReminderResult) {
	_, _ = fmt.Fprintf(w, "Bulk reminder result\n")
	_, _ = fmt.Fprintf(w, "Requested: %d\n", result.TotalRequested)
	_, _ = fmt.Fprintf(w, "Successful: %d\n", result.Successful)
	_, _ = fmt.Fprintf(w, "Failed: %d\n", result.Failed)
	for _, item := range result.Results {
		_, _ = fmt.Fprintf(w, "- %s %s success=%t message=%s\n", item.InvoiceID, item.InvoiceNumber, item.Success, item.Message)
	}
}

func printReminderRulesTable(w io.Writer, rules []invoicing.ReminderRule) {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "ID\tNAME\tTRIGGER\tOFFSET\tTEMPLATE\tACTIVE")
	for _, rule := range rules {
		_, _ = fmt.Fprintf(
			tw,
			"%s\t%s\t%s\t%d\t%s\t%t\n",
			rule.ID,
			rule.Name,
			rule.TriggerType,
			rule.DaysOffset,
			rule.EmailTemplateType,
			rule.IsActive,
		)
	}
	_ = tw.Flush()
}

func printReminderRule(w io.Writer, rule *invoicing.ReminderRule) {
	_, _ = fmt.Fprintf(w, "Reminder rule %s (%s)\n", rule.Name, rule.ID)
	_, _ = fmt.Fprintf(w, "Trigger: %s\n", rule.TriggerType)
	_, _ = fmt.Fprintf(w, "Days offset: %d\n", rule.DaysOffset)
	_, _ = fmt.Fprintf(w, "Template: %s\n", rule.EmailTemplateType)
	_, _ = fmt.Fprintf(w, "Active: %t\n", rule.IsActive)
}

func printAutomatedReminderResultsTable(w io.Writer, results []invoicing.AutomatedReminderResult) {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "RULE\tFOUND\tSENT\tSKIPPED\tFAILED\tRUN AT")
	for _, result := range results {
		_, _ = fmt.Fprintf(
			tw,
			"%s\t%d\t%d\t%d\t%d\t%s\n",
			result.RuleName,
			result.InvoicesFound,
			result.RemindersSent,
			result.Skipped,
			result.Failed,
			formatTime(result.RunAt),
		)
	}
	_ = tw.Flush()
}

func printSMTPConfig(w io.Writer, config *email.SMTPConfig) {
	_, _ = fmt.Fprintf(w, "SMTP configuration\n")
	_, _ = fmt.Fprintf(w, "Host: %s\n", config.Host)
	_, _ = fmt.Fprintf(w, "Port: %d\n", config.Port)
	_, _ = fmt.Fprintf(w, "Username: %s\n", config.Username)
	_, _ = fmt.Fprintf(w, "From: %s <%s>\n", config.FromName, config.FromEmail)
	_, _ = fmt.Fprintf(w, "Use TLS: %t\n", config.UseTLS)
	_, _ = fmt.Fprintf(w, "Configured: %t\n", config.IsConfigured())
}

func printSMTPTestResponse(w io.Writer, result *email.TestSMTPResponse) {
	_, _ = fmt.Fprintf(w, "SMTP test result\n")
	_, _ = fmt.Fprintf(w, "Success: %t\n", result.Success)
	if strings.TrimSpace(result.Message) != "" {
		_, _ = fmt.Fprintf(w, "Message: %s\n", result.Message)
	}
}

func printEmailTemplatesTable(w io.Writer, templates []email.EmailTemplate) {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "TYPE\tSUBJECT\tACTIVE\tUPDATED")
	for _, template := range templates {
		_, _ = fmt.Fprintf(
			tw,
			"%s\t%s\t%t\t%s\n",
			template.TemplateType,
			template.Subject,
			template.IsActive,
			formatTime(template.UpdatedAt),
		)
	}
	_ = tw.Flush()
}

func printEmailTemplate(w io.Writer, template *email.EmailTemplate) {
	_, _ = fmt.Fprintf(w, "Email template %s\n", template.TemplateType)
	_, _ = fmt.Fprintf(w, "Subject: %s\n", template.Subject)
	_, _ = fmt.Fprintf(w, "Active: %t\n", template.IsActive)
	_, _ = fmt.Fprintf(w, "Body HTML bytes: %d\n", len(template.BodyHTML))
	if strings.TrimSpace(template.BodyText) != "" {
		_, _ = fmt.Fprintf(w, "Body text bytes: %d\n", len(template.BodyText))
	}
}

func printEmailLogsTable(w io.Writer, logs []email.EmailLog) {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "ID\tTYPE\tRECIPIENT\tSUBJECT\tSTATUS\tSENT\tERROR")
	for _, log := range logs {
		_, _ = fmt.Fprintf(
			tw,
			"%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			log.ID,
			log.EmailType,
			log.RecipientEmail,
			log.Subject,
			log.Status,
			formatTimePtr(log.SentAt),
			log.ErrorMessage,
		)
	}
	_ = tw.Flush()
}

func printEmailSentResponse(w io.Writer, result *email.EmailSentResponse) {
	_, _ = fmt.Fprintf(w, "Email sent\n")
	_, _ = fmt.Fprintf(w, "Success: %t\n", result.Success)
	if strings.TrimSpace(result.LogID) != "" {
		_, _ = fmt.Fprintf(w, "Log ID: %s\n", result.LogID)
	}
	if strings.TrimSpace(result.Message) != "" {
		_, _ = fmt.Fprintf(w, "Message: %s\n", result.Message)
	}
}

func printInterestSettings(w io.Writer, settings *invoicing.InterestSettings) {
	_, _ = fmt.Fprintf(w, "Interest settings\n")
	_, _ = fmt.Fprintf(w, "Enabled: %t\n", settings.IsEnabled)
	_, _ = fmt.Fprintf(w, "Daily rate: %.6f\n", settings.Rate)
	_, _ = fmt.Fprintf(w, "Annual rate: %.6f\n", settings.AnnualRate)
	if strings.TrimSpace(settings.Description) != "" {
		_, _ = fmt.Fprintf(w, "Description: %s\n", settings.Description)
	}
}

func printInterestCalculationsTable(w io.Writer, results []invoicing.InterestCalculationResult) {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "INVOICE\tDUE\tDAYS\tOUTSTANDING\tDAILY\tINTEREST\tTOTAL\tCURRENCY")
	for _, result := range results {
		_, _ = fmt.Fprintf(
			tw,
			"%s\t%s\t%d\t%s\t%s\t%s\t%s\t%s\n",
			result.InvoiceNumber,
			formatDate(result.DueDate),
			result.DaysOverdue,
			result.OutstandingAmount.String(),
			result.DailyInterest.String(),
			result.TotalInterest.String(),
			result.TotalWithInterest.String(),
			result.Currency,
		)
	}
	_ = tw.Flush()
}

func printInterestCalculation(w io.Writer, result *invoicing.InterestCalculationResult) {
	_, _ = fmt.Fprintf(w, "Interest for invoice %s (%s)\n", result.InvoiceNumber, result.InvoiceID)
	_, _ = fmt.Fprintf(w, "Due date: %s\n", formatDate(result.DueDate))
	_, _ = fmt.Fprintf(w, "Days overdue: %d\n", result.DaysOverdue)
	_, _ = fmt.Fprintf(w, "Outstanding: %s %s\n", result.OutstandingAmount.String(), result.Currency)
	_, _ = fmt.Fprintf(w, "Interest rate: %s\n", result.InterestRate.String())
	_, _ = fmt.Fprintf(w, "Daily interest: %s\n", result.DailyInterest.String())
	_, _ = fmt.Fprintf(w, "Total interest: %s\n", result.TotalInterest.String())
	_, _ = fmt.Fprintf(w, "Total with interest: %s\n", result.TotalWithInterest.String())
}

func printInvoiceInterestHistoryTable(w io.Writer, history []invoicing.InvoiceInterest) {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "ID\tCALCULATED\tDAYS\tPRINCIPAL\tRATE\tINTEREST\tTOTAL")
	for _, item := range history {
		_, _ = fmt.Fprintf(
			tw,
			"%s\t%s\t%d\t%s\t%s\t%s\t%s\n",
			item.ID,
			formatTime(item.CalculatedAt),
			item.DaysOverdue,
			item.PrincipalAmount.String(),
			item.InterestRate.String(),
			item.InterestAmount.String(),
			item.TotalWithInterest.String(),
		)
	}
	_ = tw.Flush()
}

func printPeriodCloseEventsTable(w io.Writer, events []tenant.PeriodCloseEvent) {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "ID\tACTION\tKIND\tPERIOD END\tLOCK BEFORE\tLOCK AFTER\tSIGNOFF\tNOTE\tCREATED")
	for _, event := range events {
		_, _ = fmt.Fprintf(
			tw,
			"%s\t%s\t%s\t%s\t%s\t%s\t%t\t%s\t%s\n",
			event.ID,
			event.Action,
			event.CloseKind,
			event.PeriodEndDate,
			stringValue(event.LockDateBefore),
			stringValue(event.LockDateAfter),
			event.ReviewerSignOff,
			event.Note,
			formatTime(event.CreatedAt),
		)
	}
	_ = tw.Flush()
}

func printPeriodCloseMutationResponse(w io.Writer, title string, resp *periodCloseMutationResponse) {
	_, _ = fmt.Fprintf(w, "%s\n", title)
	if resp.Event != nil {
		_, _ = fmt.Fprintf(w, "Period end: %s\n", resp.Event.PeriodEndDate)
		_, _ = fmt.Fprintf(w, "Action: %s\n", resp.Event.Action)
		_, _ = fmt.Fprintf(w, "Kind: %s\n", resp.Event.CloseKind)
		_, _ = fmt.Fprintf(w, "Lock before: %s\n", stringValue(resp.Event.LockDateBefore))
		_, _ = fmt.Fprintf(w, "Lock after: %s\n", stringValue(resp.Event.LockDateAfter))
		if resp.Event.ReviewerSignOff {
			_, _ = fmt.Fprintln(w, "Reviewer sign-off: true")
		}
		if strings.TrimSpace(resp.Event.Note) != "" {
			_, _ = fmt.Fprintf(w, "Note: %s\n", resp.Event.Note)
		}
	}
	if resp.Tenant != nil && resp.Tenant.Settings.PeriodLockDate != nil {
		_, _ = fmt.Fprintf(w, "Tenant lock date: %s\n", *resp.Tenant.Settings.PeriodLockDate)
	}
}

func printYearEndCloseStatus(w io.Writer, status *accounting.YearEndCloseStatus) {
	_, _ = fmt.Fprintf(w, "Year-end close status %s\n", status.FiscalYearLabel)
	_, _ = fmt.Fprintf(w, "Period end: %s\n", status.PeriodEndDate)
	_, _ = fmt.Fprintf(w, "Fiscal year end: %s\n", status.FiscalYearEndDate)
	_, _ = fmt.Fprintf(w, "Carry-forward date: %s\n", status.CarryForwardDate)
	_, _ = fmt.Fprintf(w, "Fiscal year end period: %t\n", status.IsFiscalYearEnd)
	_, _ = fmt.Fprintf(w, "Period closed: %t\n", status.PeriodClosed)
	_, _ = fmt.Fprintf(w, "Carry-forward needed: %t\n", status.CarryForwardNeeded)
	_, _ = fmt.Fprintf(w, "Carry-forward ready: %t\n", status.CarryForwardReady)
	_, _ = fmt.Fprintf(w, "Retained earnings account: %t\n", status.HasRetainedEarningsAccount)
	_, _ = fmt.Fprintf(w, "Net income: %s\n", status.NetIncome.String())
	if status.ClosePackEvidenceEntityID != "" {
		_, _ = fmt.Fprintf(w, "Close-pack evidence entity: %s\n", status.ClosePackEvidenceEntityID)
	}
	if status.ClosePackEvidence != nil {
		_, _ = fmt.Fprintf(w, "Close-pack evidence compliant: %t\n", status.ClosePackEvidence.Compliant)
	}
	if status.InventoryCostingReview != nil {
		printYearEndInventoryCostingReview(w, status.InventoryCostingReview)
	}
	printYearEndCloseRemediationActions(w, status.RemediationActions)
	if status.RetainedEarningsAccount != nil {
		_, _ = fmt.Fprintf(w, "Retained earnings: %s %s\n", status.RetainedEarningsAccount.Code, status.RetainedEarningsAccount.Name)
	}
	if status.ExistingCarryForward != nil {
		_, _ = fmt.Fprintf(w, "Existing carry-forward: %s (%s)\n", status.ExistingCarryForward.EntryNumber, status.ExistingCarryForward.ID)
	}
}

func printYearEndInventoryCostingReview(w io.Writer, review *accounting.YearEndInventoryCostingReview) {
	if review == nil {
		return
	}
	_, _ = fmt.Fprintf(
		w,
		"Inventory costing review: method %s, ready %t, lines %d, total value %s\n",
		review.ValuationMethod,
		review.Ready,
		review.LineCount,
		review.TotalValue.String(),
	)
	if review.BlockingExceptionLineCount > 0 {
		_, _ = fmt.Fprintf(
			w,
			"Inventory costing exceptions: blocking lines %d, negative qty %d, negative available %d, negative value %d, missing cost %d\n",
			review.BlockingExceptionLineCount,
			review.NegativeQuantityLineCount,
			review.NegativeAvailableLineCount,
			review.NegativeValueLineCount,
			review.MissingCostLineCount,
		)
	}
}

func printYearEndCloseRemediationActions(w io.Writer, actions []accounting.YearEndCloseRemediationAction) {
	if len(actions) == 0 {
		return
	}
	_, _ = fmt.Fprintln(w, "Close remediation actions")
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "SEVERITY\tSCOPE\tOWNER\tCODE\tACTION\tCOMMAND")
	for _, action := range actions {
		_, _ = fmt.Fprintf(
			tw,
			"%s\t%s\t%s\t%s\t%s\t%s\n",
			action.Severity,
			action.Scope,
			action.OwnerRole,
			action.Code,
			action.Action,
			action.CLICommand,
		)
	}
	_ = tw.Flush()
}

func printYearEndClosePack(w io.Writer, pack *accounting.YearEndClosePack) {
	if pack.Status != nil {
		printYearEndCloseStatus(w, pack.Status)
	}
	if pack.TrialBalance != nil {
		_, _ = fmt.Fprintf(w, "Trial balance: debits %s, credits %s, balanced %t\n", pack.TrialBalance.TotalDebits.String(), pack.TrialBalance.TotalCredits.String(), pack.TrialBalance.IsBalanced)
	}
	if pack.BalanceSheet != nil {
		_, _ = fmt.Fprintf(w, "Balance sheet: assets %s, liabilities %s, equity %s, balanced %t\n", pack.BalanceSheet.TotalAssets.String(), pack.BalanceSheet.TotalLiabilities.String(), pack.BalanceSheet.TotalEquity.String(), pack.BalanceSheet.IsBalanced)
	}
	if pack.IncomeStatement != nil {
		_, _ = fmt.Fprintf(w, "Income statement: revenue %s, expenses %s, net income %s\n", pack.IncomeStatement.TotalRevenue.String(), pack.IncomeStatement.TotalExpenses.String(), pack.IncomeStatement.NetIncome.String())
	}
}

func printAnnualReport(w io.Writer, report *reports.AnnualReport) {
	if report == nil {
		return
	}
	_, _ = fmt.Fprintf(w, "Annual report %s\n", report.FiscalYearLabel)
	_, _ = fmt.Fprintf(w, "Fiscal year: %s to %s\n", report.FiscalYearStartDate, report.FiscalYearEndDate)
	if report.CloseStatus != nil {
		_, _ = fmt.Fprintf(w, "Period closed: %t\n", report.CloseStatus.PeriodClosed)
		_, _ = fmt.Fprintf(w, "Carry-forward ready: %t\n", report.CloseStatus.CarryForwardReady)
		_, _ = fmt.Fprintf(w, "Net income: %s\n", report.CloseStatus.NetIncome.String())
	}
	if report.TrialBalance != nil {
		_, _ = fmt.Fprintf(w, "Trial balance: debits %s, credits %s, balanced %t\n", report.TrialBalance.TotalDebits.String(), report.TrialBalance.TotalCredits.String(), report.TrialBalance.IsBalanced)
	}
	if report.BalanceSheet != nil {
		_, _ = fmt.Fprintf(w, "Balance sheet: assets %s, liabilities %s, equity %s, balanced %t\n", report.BalanceSheet.TotalAssets.String(), report.BalanceSheet.TotalLiabilities.String(), report.BalanceSheet.TotalEquity.String(), report.BalanceSheet.IsBalanced)
	}
	if report.IncomeStatement != nil {
		_, _ = fmt.Fprintf(w, "Income statement: revenue %s, expenses %s, net income %s\n", report.IncomeStatement.TotalRevenue.String(), report.IncomeStatement.TotalExpenses.String(), report.IncomeStatement.NetIncome.String())
	}
	if report.CashFlowStatement != nil {
		_, _ = fmt.Fprintf(w, "Cash flow: method %s, net change %s, closing cash %s\n", report.CashFlowStatement.Method, report.CashFlowStatement.NetCashChange.String(), report.CashFlowStatement.ClosingCash.String())
	}
}

func printYearEndCloseAuditEvidence(w io.Writer, audit *accounting.YearEndCloseAuditEvidence) {
	if audit == nil {
		return
	}
	if audit.Pack != nil {
		printYearEndClosePack(w, audit.Pack)
	}
	_, _ = fmt.Fprintf(w, "Close-pack audit evidence generated: %s\n", audit.GeneratedAt.Format(time.RFC3339))
	if audit.EvidencePolicy != nil {
		_, _ = fmt.Fprintf(w, "Evidence policy compliant: %t, total documents: %d, pending: %d, approved: %d, rejected: %d\n",
			audit.EvidencePolicy.Compliant,
			audit.EvidencePolicy.TotalCount,
			audit.EvidencePolicy.PendingReviewCount,
			audit.EvidencePolicy.ApprovedCount,
			audit.EvidencePolicy.RejectedCount,
		)
	}
	if len(audit.Documents) == 0 {
		_, _ = fmt.Fprintln(w, "Attached close-pack documents: 0")
		return
	}
	_, _ = fmt.Fprintf(w, "Attached close-pack documents: %d\n", len(audit.Documents))
	printDocumentsTable(w, audit.Documents)
}

func printYearEndCarryForwardResult(w io.Writer, result *accounting.YearEndCarryForwardResult) {
	if result.JournalEntry != nil {
		_, _ = fmt.Fprintf(w, "Created year-end carry-forward %s (%s)\n", result.JournalEntry.EntryNumber, result.JournalEntry.ID)
		_, _ = fmt.Fprintf(w, "Status: %s\n", result.JournalEntry.Status)
	}
	if result.Status != nil {
		_, _ = fmt.Fprintf(w, "Carry-forward ready: %t\n", result.Status.CarryForwardReady)
		if result.Status.ExistingCarryForward != nil {
			_, _ = fmt.Fprintf(w, "Existing carry-forward: %s (%s)\n", result.Status.ExistingCarryForward.EntryNumber, result.Status.ExistingCarryForward.ID)
		}
	}
}

func printYearEndCarryForwardReversalResult(w io.Writer, result *accounting.YearEndCarryForwardReversalResult) {
	if result.ReversalJournalEntry != nil {
		_, _ = fmt.Fprintf(w, "Reversed year-end carry-forward %s (%s)\n", result.ReversalJournalEntry.EntryNumber, result.ReversalJournalEntry.ID)
		_, _ = fmt.Fprintf(w, "Status: %s\n", result.ReversalJournalEntry.Status)
		_, _ = fmt.Fprintf(w, "Reversal date: %s\n", result.ReversalJournalEntry.EntryDate.Format("2006-01-02"))
	}
	if result.Status != nil {
		_, _ = fmt.Fprintf(w, "Carry-forward ready: %t\n", result.Status.CarryForwardReady)
		_, _ = fmt.Fprintf(w, "Carry-forward needed: %t\n", result.Status.CarryForwardNeeded)
	}
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func printBankAccountsTable(w io.Writer, accounts []banking.BankAccount) {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "ID\tNAME\tACCOUNT\tBANK\tCURRENCY\tDEFAULT\tACTIVE\tBALANCE")
	for _, account := range accounts {
		_, _ = fmt.Fprintf(
			tw,
			"%s\t%s\t%s\t%s\t%s\t%t\t%t\t%s\n",
			account.ID,
			account.Name,
			account.AccountNumber,
			account.BankName,
			account.Currency,
			account.IsDefault,
			account.IsActive,
			account.Balance.String(),
		)
	}
	_ = tw.Flush()
}

func printBankAccount(w io.Writer, account *banking.BankAccount) {
	_, _ = fmt.Fprintf(w, "Bank account %s (%s)\n", account.Name, account.AccountNumber)
	_, _ = fmt.Fprintf(w, "ID: %s\n", account.ID)
	if strings.TrimSpace(account.BankName) != "" {
		_, _ = fmt.Fprintf(w, "Bank: %s\n", account.BankName)
	}
	if strings.TrimSpace(account.SwiftCode) != "" {
		_, _ = fmt.Fprintf(w, "SWIFT: %s\n", account.SwiftCode)
	}
	_, _ = fmt.Fprintf(w, "Currency: %s\n", account.Currency)
	_, _ = fmt.Fprintf(w, "Default: %t\n", account.IsDefault)
	_, _ = fmt.Fprintf(w, "Active: %t\n", account.IsActive)
	_, _ = fmt.Fprintf(w, "Balance: %s\n", account.Balance.String())
	if account.GLAccountID != nil && strings.TrimSpace(*account.GLAccountID) != "" {
		_, _ = fmt.Fprintf(w, "GL account: %s\n", *account.GLAccountID)
	}
}

func printBankMatchRulesTable(w io.Writer, rules []banking.BankMatchRule) {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "ID\tNAME\tACCOUNT\tPRIORITY\tFIELD\tPATTERN\tCONFIDENCE\tDATE DIFF\tEXACT\tACTIVE")
	for _, rule := range rules {
		_, _ = fmt.Fprintf(
			tw,
			"%s\t%s\t%s\t%d\t%s\t%s\t%.2f\t%d\t%t\t%t\n",
			rule.ID,
			rule.Name,
			stringValue(rule.BankAccountID),
			rule.Priority,
			rule.MatchField,
			rule.Pattern,
			rule.MinConfidence,
			rule.MaxDateDiffDays,
			rule.RequireExactAmount,
			rule.IsActive,
		)
	}
	_ = tw.Flush()
}

func printBankMatchRule(w io.Writer, rule *banking.BankMatchRule) {
	_, _ = fmt.Fprintf(w, "Bank match rule %s (%s)\n", rule.Name, rule.ID)
	_, _ = fmt.Fprintf(w, "Bank account: %s\n", stringValue(rule.BankAccountID))
	_, _ = fmt.Fprintf(w, "Priority: %d\n", rule.Priority)
	_, _ = fmt.Fprintf(w, "Field: %s\n", rule.MatchField)
	_, _ = fmt.Fprintf(w, "Pattern: %s\n", rule.Pattern)
	_, _ = fmt.Fprintf(w, "Min confidence: %.2f\n", rule.MinConfidence)
	_, _ = fmt.Fprintf(w, "Max date diff days: %d\n", rule.MaxDateDiffDays)
	_, _ = fmt.Fprintf(w, "Require exact amount: %t\n", rule.RequireExactAmount)
	_, _ = fmt.Fprintf(w, "Active: %t\n", rule.IsActive)
}

func printBankTransactionsTable(w io.Writer, transactions []banking.BankTransaction) {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "ID\tDATE\tAMOUNT\tCURRENCY\tSTATUS\tFOLLOW-UP\tCOUNTERPARTY\tREFERENCE\tDESCRIPTION")
	for _, transaction := range transactions {
		_, _ = fmt.Fprintf(
			tw,
			"%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			transaction.ID,
			formatDate(transaction.TransactionDate),
			transaction.Amount.String(),
			transaction.Currency,
			transaction.Status,
			transaction.FollowUpStatus,
			transaction.CounterpartyName,
			transaction.Reference,
			transaction.Description,
		)
	}
	_ = tw.Flush()
}

func printBankTransaction(w io.Writer, transaction *banking.BankTransaction) {
	_, _ = fmt.Fprintf(w, "Bank transaction %s (%s)\n", transaction.ID, transaction.Status)
	_, _ = fmt.Fprintf(w, "Bank account: %s\n", transaction.BankAccountID)
	_, _ = fmt.Fprintf(w, "Transaction date: %s\n", formatDate(transaction.TransactionDate))
	_, _ = fmt.Fprintf(w, "Value date: %s\n", formatDatePtr(transaction.ValueDate))
	_, _ = fmt.Fprintf(w, "Amount: %s %s\n", transaction.Amount.String(), transaction.Currency)
	if strings.TrimSpace(transaction.Description) != "" {
		_, _ = fmt.Fprintf(w, "Description: %s\n", transaction.Description)
	}
	if strings.TrimSpace(transaction.Reference) != "" {
		_, _ = fmt.Fprintf(w, "Reference: %s\n", transaction.Reference)
	}
	if strings.TrimSpace(transaction.CounterpartyName) != "" {
		_, _ = fmt.Fprintf(w, "Counterparty: %s\n", transaction.CounterpartyName)
	}
	if strings.TrimSpace(transaction.CounterpartyAccount) != "" {
		_, _ = fmt.Fprintf(w, "Counterparty account: %s\n", transaction.CounterpartyAccount)
	}
	_, _ = fmt.Fprintf(w, "Follow-up: %s\n", transaction.FollowUpStatus)
	if strings.TrimSpace(transaction.ReviewNote) != "" {
		_, _ = fmt.Fprintf(w, "Review note: %s\n", transaction.ReviewNote)
	}
	if transaction.MatchedPaymentID != nil && strings.TrimSpace(*transaction.MatchedPaymentID) != "" {
		_, _ = fmt.Fprintf(w, "Matched payment: %s\n", *transaction.MatchedPaymentID)
	}
	if transaction.ReconciliationID != nil && strings.TrimSpace(*transaction.ReconciliationID) != "" {
		_, _ = fmt.Fprintf(w, "Reconciliation: %s\n", *transaction.ReconciliationID)
	}
}

func printBankImportResult(w io.Writer, result *banking.ImportResult) {
	_, _ = fmt.Fprintf(w, "Import %s\n", result.ImportID)
	_, _ = fmt.Fprintf(w, "Imported: %d\n", result.TransactionsImported)
	_, _ = fmt.Fprintf(w, "Matched: %d\n", result.TransactionsMatched)
	_, _ = fmt.Fprintf(w, "Duplicates skipped: %d\n", result.DuplicatesSkipped)
	for _, rowErr := range result.Errors {
		_, _ = fmt.Fprintf(w, "Error: %s\n", rowErr)
	}
}

func printBankAccountImportResult(w io.Writer, result *banking.ImportBankAccountsResult) {
	_, _ = fmt.Fprintf(w, "File: %s\n", result.FileName)
	_, _ = fmt.Fprintf(w, "Processed: %d\n", result.RowsProcessed)
	_, _ = fmt.Fprintf(w, "Imported: %d\n", result.AccountsImported)
	_, _ = fmt.Fprintf(w, "Skipped: %d\n", result.RowsSkipped)
	for _, rowErr := range result.Errors {
		_, _ = fmt.Fprintf(w, "Error: %s\n", rowErr)
	}
}

func printBankImportsTable(w io.Writer, imports []banking.BankStatementImport) {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "ID\tFILE\tIMPORTED\tMATCHED\tDUPLICATES\tCREATED")
	for _, entry := range imports {
		_, _ = fmt.Fprintf(
			tw,
			"%s\t%s\t%d\t%d\t%d\t%s\n",
			entry.ID,
			entry.FileName,
			entry.TransactionsImported,
			entry.TransactionsMatched,
			entry.DuplicatesSkipped,
			entry.CreatedAt.Format(time.RFC3339),
		)
	}
	_ = tw.Flush()
}

func printMatchSuggestionsTable(w io.Writer, suggestions []banking.MatchSuggestion) {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "PAYMENT\tNUMBER\tDATE\tAMOUNT\tCONFIDENCE\tCONTACT\tREASON")
	for _, suggestion := range suggestions {
		_, _ = fmt.Fprintf(
			tw,
			"%s\t%s\t%s\t%s\t%.2f\t%s\t%s\n",
			suggestion.PaymentID,
			suggestion.PaymentNumber,
			formatDate(suggestion.PaymentDate),
			suggestion.Amount.String(),
			suggestion.Confidence,
			suggestion.ContactName,
			suggestion.MatchReason,
		)
	}
	_ = tw.Flush()
}

func printBankReconciliationsTable(w io.Writer, reconciliations []banking.BankReconciliation) {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "ID\tSTATEMENT\tSTATUS\tOPENING\tCLOSING\tCOMPLETED")
	for _, reconciliation := range reconciliations {
		_, _ = fmt.Fprintf(
			tw,
			"%s\t%s\t%s\t%s\t%s\t%s\n",
			reconciliation.ID,
			formatDate(reconciliation.StatementDate),
			reconciliation.Status,
			reconciliation.OpeningBalance.String(),
			reconciliation.ClosingBalance.String(),
			formatTimePtr(reconciliation.CompletedAt),
		)
	}
	_ = tw.Flush()
}

func printBankReconciliation(w io.Writer, reconciliation *banking.BankReconciliation) {
	_, _ = fmt.Fprintf(w, "Bank reconciliation %s (%s)\n", reconciliation.ID, reconciliation.Status)
	_, _ = fmt.Fprintf(w, "Bank account: %s\n", reconciliation.BankAccountID)
	_, _ = fmt.Fprintf(w, "Statement date: %s\n", formatDate(reconciliation.StatementDate))
	_, _ = fmt.Fprintf(w, "Opening balance: %s\n", reconciliation.OpeningBalance.String())
	_, _ = fmt.Fprintf(w, "Closing balance: %s\n", reconciliation.ClosingBalance.String())
	_, _ = fmt.Fprintf(w, "Completed: %s\n", formatTimePtr(reconciliation.CompletedAt))
}

func printAbsenceTypesTable(w io.Writer, types []payroll.AbsenceType) {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "ID\tCODE\tNAME\tPAID\tACTIVE\tDEFAULT DAYS")
	for _, absenceType := range types {
		_, _ = fmt.Fprintf(
			tw,
			"%s\t%s\t%s\t%t\t%t\t%s\n",
			absenceType.ID,
			absenceType.Code,
			absenceType.Name,
			absenceType.IsPaid,
			absenceType.IsActive,
			absenceType.DefaultDaysPerYear.String(),
		)
	}
	_ = tw.Flush()
}

func printAbsenceType(w io.Writer, absenceType *payroll.AbsenceType) {
	_, _ = fmt.Fprintf(w, "Absence type %s %s\n", absenceType.Code, absenceType.Name)
	_, _ = fmt.Fprintf(w, "ID: %s\n", absenceType.ID)
	if strings.TrimSpace(absenceType.NameET) != "" {
		_, _ = fmt.Fprintf(w, "Name ET: %s\n", absenceType.NameET)
	}
	if strings.TrimSpace(absenceType.Description) != "" {
		_, _ = fmt.Fprintf(w, "Description: %s\n", absenceType.Description)
	}
	_, _ = fmt.Fprintf(w, "Paid: %t\n", absenceType.IsPaid)
	_, _ = fmt.Fprintf(w, "Affects salary: %t\n", absenceType.AffectsSalary)
	_, _ = fmt.Fprintf(w, "Requires document: %t\n", absenceType.RequiresDocument)
	if strings.TrimSpace(absenceType.DocumentType) != "" {
		_, _ = fmt.Fprintf(w, "Document type: %s\n", absenceType.DocumentType)
	}
	_, _ = fmt.Fprintf(w, "Default days per year: %s\n", absenceType.DefaultDaysPerYear.String())
	_, _ = fmt.Fprintf(w, "Max carryover days: %s\n", absenceType.MaxCarryoverDays.String())
	_, _ = fmt.Fprintf(w, "Active: %t\n", absenceType.IsActive)
}

func printLeaveBalancesTable(w io.Writer, balances []payroll.LeaveBalance) {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "EMPLOYEE\tYEAR\tTYPE\tENTITLED\tCARRYOVER\tUSED\tPENDING\tREMAINING")
	for _, balance := range balances {
		_, _ = fmt.Fprintf(
			tw,
			"%s\t%d\t%s\t%s\t%s\t%s\t%s\t%s\n",
			balance.EmployeeID,
			balance.Year,
			leaveAbsenceTypeLabel(balance.AbsenceTypeID, balance.AbsenceType),
			balance.EntitledDays.String(),
			balance.CarryoverDays.String(),
			balance.UsedDays.String(),
			balance.PendingDays.String(),
			balance.RemainingDays.String(),
		)
	}
	_ = tw.Flush()
}

func printLeaveRecordsTable(w io.Writer, records []payroll.LeaveRecord) {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "ID\tEMPLOYEE\tTYPE\tSTART\tEND\tWORKING DAYS\tSTATUS")
	for _, record := range records {
		_, _ = fmt.Fprintf(
			tw,
			"%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			record.ID,
			leaveEmployeeLabel(record.EmployeeID, record.Employee),
			leaveAbsenceTypeLabel(record.AbsenceTypeID, record.AbsenceType),
			formatDate(record.StartDate),
			formatDate(record.EndDate),
			record.WorkingDays.String(),
			record.Status,
		)
	}
	_ = tw.Flush()
}

func printLeaveRecord(w io.Writer, record *payroll.LeaveRecord) {
	_, _ = fmt.Fprintf(w, "Leave record %s (%s)\n", record.ID, record.Status)
	_, _ = fmt.Fprintf(w, "Employee: %s\n", leaveEmployeeLabel(record.EmployeeID, record.Employee))
	_, _ = fmt.Fprintf(w, "Absence type: %s\n", leaveAbsenceTypeLabel(record.AbsenceTypeID, record.AbsenceType))
	_, _ = fmt.Fprintf(w, "Start: %s\n", formatDate(record.StartDate))
	_, _ = fmt.Fprintf(w, "End: %s\n", formatDate(record.EndDate))
	_, _ = fmt.Fprintf(w, "Total days: %s\n", record.TotalDays.String())
	_, _ = fmt.Fprintf(w, "Working days: %s\n", record.WorkingDays.String())
	if strings.TrimSpace(record.DocumentNumber) != "" {
		_, _ = fmt.Fprintf(w, "Document number: %s\n", record.DocumentNumber)
	}
	if record.DocumentDate != nil {
		_, _ = fmt.Fprintf(w, "Document date: %s\n", formatDatePtr(record.DocumentDate))
	}
	if strings.TrimSpace(record.RejectionReason) != "" {
		_, _ = fmt.Fprintf(w, "Rejection reason: %s\n", record.RejectionReason)
	}
	if strings.TrimSpace(record.Notes) != "" {
		_, _ = fmt.Fprintf(w, "Notes: %s\n", record.Notes)
	}
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
	printKMDRemediationActions(w, declaration.RemediationActions)
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

func printKMDRemediationActions(w io.Writer, actions []tax.KMDRemediationAction) {
	if len(actions) == 0 {
		return
	}
	_, _ = fmt.Fprintln(w, "KMD remediation actions")
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "SEVERITY\tSCOPE\tOWNER\tCODE\tACTION\tCOMMAND")
	for _, action := range actions {
		_, _ = fmt.Fprintf(
			tw,
			"%s\t%s\t%s\t%s\t%s\t%s\n",
			action.Severity,
			action.Scope,
			action.OwnerRole,
			action.Code,
			action.Action,
			action.CLICommand,
		)
	}
	_ = tw.Flush()
}

func printKMDINFReport(w io.Writer, report *tax.KMDINFReport) {
	_, _ = fmt.Fprintf(w, "KMD INF %04d-%02d\n", report.Year, report.Month)
	_, _ = fmt.Fprintf(w, "Threshold: %s\n", report.Threshold.String())
	_, _ = fmt.Fprintf(w, "Generated: %s\n", formatTime(report.GeneratedAt))

	if len(report.Summary) > 0 {
		tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
		_, _ = fmt.Fprintln(tw, "PART\tPARTNERS\tINVOICES\tTAXABLE\tVAT\tTOTAL")
		for _, summary := range report.Summary {
			_, _ = fmt.Fprintf(
				tw,
				"%s\t%d\t%d\t%s\t%s\t%s\n",
				kmdINFPartLabel(summary.Part),
				summary.PartnerCount,
				summary.InvoiceCount,
				summary.TaxableAmount.String(),
				summary.VATAmount.String(),
				summary.TotalAmount.String(),
			)
		}
		_ = tw.Flush()
	}
	if len(report.Rows) == 0 {
		return
	}

	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "PART\tCONTACT\tREG/VAT\tINVOICE\tDATE\tTAXABLE\tVAT\tTOTAL\tPARTNER PERIOD")
	for _, row := range report.Rows {
		_, _ = fmt.Fprintf(
			tw,
			"%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			kmdINFPartLabel(row.Part),
			row.ContactName,
			firstNonEmpty(row.ContactRegCode, row.ContactVATNumber, "-"),
			row.InvoiceNumber,
			formatDate(row.InvoiceDate),
			row.TaxableAmount.String(),
			row.VATAmount.String(),
			row.TotalAmount.String(),
			row.PartnerPeriodTaxableAmount.String(),
		)
	}
	_ = tw.Flush()
}

func printEUVATOSSReport(w io.Writer, report *tax.EUVATOSSReport) {
	_, _ = fmt.Fprintf(w, "EU VAT OSS %04d-Q%d (%s)\n", report.Year, report.Quarter, report.Scheme)
	_, _ = fmt.Fprintf(w, "Period: %s to %s\n", formatDate(report.PeriodStart), formatDate(report.PeriodEnd))
	_, _ = fmt.Fprintf(w, "Currency: %s\n", report.Currency)
	_, _ = fmt.Fprintf(w, "Taxable: %s\n", report.TaxableAmount.String())
	_, _ = fmt.Fprintf(w, "VAT: %s\n", report.VATAmount.String())
	_, _ = fmt.Fprintf(w, "Total: %s\n", report.TotalAmount.String())

	if len(report.Summary) > 0 {
		tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
		_, _ = fmt.Fprintln(tw, "COUNTRY\tINVOICES\tLINES\tTAXABLE\tVAT\tTOTAL")
		for _, summary := range report.Summary {
			_, _ = fmt.Fprintf(
				tw,
				"%s %s\t%d\t%d\t%s\t%s\t%s\n",
				summary.CountryCode,
				summary.CountryName,
				summary.InvoiceCount,
				summary.LineCount,
				summary.TaxableAmount.String(),
				summary.VATAmount.String(),
				summary.TotalAmount.String(),
			)
		}
		_ = tw.Flush()
	}
	if len(report.Rows) == 0 {
		return
	}

	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "COUNTRY\tVAT RATE\tINVOICES\tLINES\tTAXABLE\tVAT\tTOTAL")
	for _, row := range report.Rows {
		_, _ = fmt.Fprintf(
			tw,
			"%s %s\t%s\t%d\t%d\t%s\t%s\t%s\n",
			row.CountryCode,
			row.CountryName,
			row.VATRate.String(),
			row.InvoiceCount,
			row.LineCount,
			row.TaxableAmount.String(),
			row.VATAmount.String(),
			row.TotalAmount.String(),
		)
	}
	_ = tw.Flush()
}

func kmdINFPartLabel(part tax.KMDINFPart) string {
	switch part {
	case tax.KMDINFPartSales:
		return "A sales"
	case tax.KMDINFPartPurchases:
		return "B purchases"
	default:
		return string(part)
	}
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

func printConsolidatedFinancialReport(w io.Writer, report *reports.ConsolidatedFinancialReport) {
	_, _ = fmt.Fprintf(w, "Consolidated report (%d tenants)\n", report.TenantCount)
	_, _ = fmt.Fprintf(w, "As of: %s\n", formatDate(report.AsOfDate))
	_, _ = fmt.Fprintf(w, "Income period: %s to %s\n", formatDate(report.StartDate), formatDate(report.EndDate))
	if len(report.Entities) > 0 {
		tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
		_, _ = fmt.Fprintln(tw, "TENANT\tASSETS\tLIABILITIES\tEQUITY\tREVENUE\tEXPENSES\tNET INCOME")
		for _, entity := range report.Entities {
			balanceSheet := entity.BalanceSheet
			incomeStatement := entity.IncomeStatement
			var assets, liabilities, equity, revenue, expenses, netIncome string
			if balanceSheet != nil {
				assets = balanceSheet.TotalAssets.String()
				liabilities = balanceSheet.TotalLiabilities.String()
				equity = balanceSheet.TotalEquity.String()
			}
			if incomeStatement != nil {
				revenue = incomeStatement.TotalRevenue.String()
				expenses = incomeStatement.TotalExpenses.String()
				netIncome = incomeStatement.NetIncome.String()
			}
			_, _ = fmt.Fprintf(
				tw,
				"%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
				firstNonEmpty(entity.TenantName, entity.TenantID),
				firstNonEmpty(assets, "0"),
				firstNonEmpty(liabilities, "0"),
				firstNonEmpty(equity, "0"),
				firstNonEmpty(revenue, "0"),
				firstNonEmpty(expenses, "0"),
				firstNonEmpty(netIncome, "0"),
			)
		}
		_ = tw.Flush()
	}
	if report.BalanceSheet != nil {
		_, _ = fmt.Fprintf(w, "Total assets: %s\n", report.BalanceSheet.TotalAssets.String())
		_, _ = fmt.Fprintf(w, "Total liabilities: %s\n", report.BalanceSheet.TotalLiabilities.String())
		_, _ = fmt.Fprintf(w, "Total equity: %s\n", report.BalanceSheet.TotalEquity.String())
	}
	if report.IncomeStatement != nil {
		_, _ = fmt.Fprintf(w, "Total revenue: %s\n", report.IncomeStatement.TotalRevenue.String())
		_, _ = fmt.Fprintf(w, "Total expenses: %s\n", report.IncomeStatement.TotalExpenses.String())
		_, _ = fmt.Fprintf(w, "Net income: %s\n", report.IncomeStatement.NetIncome.String())
	}
}

func printCashFlowStatement(w io.Writer, report *reports.CashFlowStatement) {
	_, _ = fmt.Fprintf(w, "Cash flow %s to %s\n", report.StartDate, report.EndDate)
	if strings.TrimSpace(report.Method) != "" {
		_, _ = fmt.Fprintf(w, "Method: %s\n", report.Method)
	}
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

func printCashFlowMapping(w io.Writer, mapping *reports.CashFlowMappingOverrides) {
	_, _ = fmt.Fprintln(w, "Cash flow mapping")
	_, _ = fmt.Fprintf(w, "Operating accounts: %s\n", formatAccountCodeList(mapping.OperatingAccountCodes))
	_, _ = fmt.Fprintf(w, "Investing accounts: %s\n", formatAccountCodeList(mapping.InvestingAccountCodes))
	_, _ = fmt.Fprintf(w, "Financing accounts: %s\n", formatAccountCodeList(mapping.FinancingAccountCodes))
}

func formatAccountCodeList(codes []string) string {
	if len(codes) == 0 {
		return "-"
	}
	return strings.Join(codes, ", ")
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

func printContactStatement(w io.Writer, report *reports.ContactStatement) {
	_, _ = fmt.Fprintf(w, "%s contact statement for %s from %s to %s\n", report.Type, report.ContactName, report.StartDate, report.EndDate)
	_, _ = fmt.Fprintf(w, "Opening balance: %s\n", report.OpeningBalance.String())
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "DATE\tTYPE\tDOCUMENT\tDUE\tINCREASE\tDECREASE\tBALANCE")
	for _, entry := range report.Entries {
		_, _ = fmt.Fprintf(
			tw,
			"%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			entry.Date,
			entry.DocumentType,
			entry.DocumentNumber,
			entry.DueDate,
			entry.IncreaseAmount.String(),
			entry.DecreaseAmount.String(),
			entry.Balance.String(),
		)
	}
	_ = tw.Flush()
	_, _ = fmt.Fprintf(w, "Total invoiced: %s\n", report.TotalInvoiced.String())
	_, _ = fmt.Fprintf(w, "Total paid: %s\n", report.TotalPaid.String())
	_, _ = fmt.Fprintf(w, "Closing balance: %s\n", report.ClosingBalance.String())
}

func printSalesMarginReport(w io.Writer, report *reports.SalesMarginReport) {
	_, _ = fmt.Fprintf(w, "Sales margin from %s to %s\n", report.StartDate, report.EndDate)
	if len(report.ByContact) > 0 {
		_, _ = fmt.Fprintln(w, "By customer:")
		contactWriter := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
		_, _ = fmt.Fprintln(contactWriter, "CONTACT\tREVENUE\tCOST\tMARGIN\tMARGIN %\tLINES")
		for _, contact := range report.ByContact {
			_, _ = fmt.Fprintf(
				contactWriter,
				"%s\t%s\t%s\t%s\t%s\t%d\n",
				contact.ContactName,
				contact.Revenue.String(),
				contact.Cost.String(),
				contact.Margin.String(),
				contact.MarginPercent.String(),
				contact.LineCount,
			)
		}
		_ = contactWriter.Flush()
	}
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "DATE\tINVOICE\tCONTACT\tPRODUCT\tREVENUE\tCOST\tMARGIN\tMARGIN %")
	for _, line := range report.Lines {
		product := line.ProductName
		if product == "" {
			product = line.Description
		}
		_, _ = fmt.Fprintf(
			tw,
			"%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			line.InvoiceDate,
			line.InvoiceNumber,
			line.ContactName,
			product,
			line.Revenue.String(),
			line.Cost.String(),
			line.Margin.String(),
			line.MarginPercent.String(),
		)
	}
	_ = tw.Flush()
	_, _ = fmt.Fprintf(w, "Lines: %d\n", report.LineCount)
	_, _ = fmt.Fprintf(w, "Total revenue: %s\n", report.TotalRevenue.String())
	_, _ = fmt.Fprintf(w, "Total cost: %s\n", report.TotalCost.String())
	_, _ = fmt.Fprintf(w, "Total margin: %s\n", report.TotalMargin.String())
	_, _ = fmt.Fprintf(w, "Margin percent: %s\n", report.MarginPercent.String())
}

func printCustomerProfitabilityReport(w io.Writer, report *reports.SalesMarginReport) {
	_, _ = fmt.Fprintf(w, "Customer profitability from %s to %s\n", report.StartDate, report.EndDate)
	if len(report.ByContact) > 0 {
		_, _ = fmt.Fprintln(w, "By customer:")
		contactWriter := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
		_, _ = fmt.Fprintln(contactWriter, "CUSTOMER\tREVENUE\tEST. COST\tPROFIT\tPROFIT %\tLINES")
		for _, contact := range report.ByContact {
			_, _ = fmt.Fprintf(
				contactWriter,
				"%s\t%s\t%s\t%s\t%s\t%d\n",
				contact.ContactName,
				contact.Revenue.String(),
				contact.Cost.String(),
				contact.Margin.String(),
				contact.MarginPercent.String(),
				contact.LineCount,
			)
		}
		_ = contactWriter.Flush()
	}
	if len(report.Lines) > 0 {
		_, _ = fmt.Fprintln(w, "Supporting invoice lines:")
		tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
		_, _ = fmt.Fprintln(tw, "DATE\tINVOICE\tCUSTOMER\tPRODUCT\tREVENUE\tEST. COST\tPROFIT\tPROFIT %")
		for _, line := range report.Lines {
			product := line.ProductName
			if product == "" {
				product = line.Description
			}
			_, _ = fmt.Fprintf(
				tw,
				"%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
				line.InvoiceDate,
				line.InvoiceNumber,
				line.ContactName,
				product,
				line.Revenue.String(),
				line.Cost.String(),
				line.Margin.String(),
				line.MarginPercent.String(),
			)
		}
		_ = tw.Flush()
	}
	_, _ = fmt.Fprintf(w, "Lines: %d\n", report.LineCount)
	_, _ = fmt.Fprintf(w, "Total revenue: %s\n", report.TotalRevenue.String())
	_, _ = fmt.Fprintf(w, "Total estimated cost: %s\n", report.TotalCost.String())
	_, _ = fmt.Fprintf(w, "Total profit: %s\n", report.TotalMargin.String())
	_, _ = fmt.Fprintf(w, "Profit percent: %s\n", report.MarginPercent.String())
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

func formatIntPtr(value *int) string {
	if value == nil {
		return "-"
	}
	return strconv.Itoa(*value)
}

func formatOptionalString(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "-"
	}
	return trimmed
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

func leaveEmployeeLabel(employeeID string, employee *payroll.Employee) string {
	if employee == nil {
		return employeeID
	}
	name := strings.TrimSpace(employee.FullName())
	if name == "" {
		return employeeID
	}
	return name
}

func leaveAbsenceTypeLabel(absenceTypeID string, absenceType *payroll.AbsenceType) string {
	if absenceType == nil {
		return absenceTypeID
	}
	if strings.TrimSpace(absenceType.Code) != "" {
		return strings.TrimSpace(absenceType.Code)
	}
	if strings.TrimSpace(absenceType.Name) != "" {
		return strings.TrimSpace(absenceType.Name)
	}
	return absenceTypeID
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
