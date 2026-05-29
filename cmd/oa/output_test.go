package main

import (
	"bytes"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/HMB-research/open-accounting/internal/accounting"
	"github.com/HMB-research/open-accounting/internal/analytics"
	"github.com/HMB-research/open-accounting/internal/apitoken"
	"github.com/HMB-research/open-accounting/internal/assets"
	"github.com/HMB-research/open-accounting/internal/banking"
	"github.com/HMB-research/open-accounting/internal/contacts"
	"github.com/HMB-research/open-accounting/internal/documents"
	"github.com/HMB-research/open-accounting/internal/email"
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
)

func TestPrintJSON(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	err := printJSON(&buf, map[string]string{"status": "ok"})
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "\"status\": \"ok\"")

	buf.Reset()
	err = printRawJSON(&buf, []byte(`{"status":"ok"}`))
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "\"status\": \"ok\"")

	buf.Reset()
	printLoginResponse(&buf, &loginResponse{
		AccessToken:  "jwt-123",
		RefreshToken: "refresh-123",
		TokenType:    "Bearer",
		ExpiresIn:    900,
		User:         &currentUser{ID: "user-1", Name: "CLI User", Email: "cli@example.com"},
	})
	assert.Contains(t, buf.String(), "Access token: jwt-123")
	assert.Contains(t, buf.String(), "Refresh token: refresh-123")
	assert.Contains(t, buf.String(), "Token type: Bearer")
	assert.Contains(t, buf.String(), "Expires in: 900 seconds")
	assert.Contains(t, buf.String(), "User: CLI User <cli@example.com> (user-1)")
}

func TestPrintTables(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 3, 12, 10, 0, 0, 0, time.UTC)

	var tokenBuf bytes.Buffer
	printAPITokensTable(&tokenBuf, []apitoken.APIToken{{
		ID:          "token-1",
		Name:        "CLI",
		TokenPrefix: "oa_tok",
		CreatedAt:   now,
	}})
	assert.Contains(t, tokenBuf.String(), "ID")
	assert.Contains(t, tokenBuf.String(), "CLI")

	var tenantBuf bytes.Buffer
	printTenant(&tenantBuf, &tenant.Tenant{
		ID:                  "tenant-1",
		Name:                "Alpha",
		Slug:                "alpha",
		IsActive:            true,
		OnboardingCompleted: true,
		Settings: tenant.TenantSettings{
			DefaultCurrency: "EUR",
			CountryCode:     "EE",
			Timezone:        "Europe/Tallinn",
			Email:           "finance@example.com",
		},
	})
	assert.Contains(t, tenantBuf.String(), "Tenant Alpha")
	assert.Contains(t, tenantBuf.String(), "finance@example.com")

	var tenantUsersBuf bytes.Buffer
	printTenantUsersTable(&tenantUsersBuf, []tenant.TenantUser{{
		TenantID:  "tenant-1",
		UserID:    "user-1",
		Role:      tenant.RoleAdmin,
		IsDefault: true,
		CreatedAt: now,
	}})
	assert.Contains(t, tenantUsersBuf.String(), "user-1")
	assert.Contains(t, tenantUsersBuf.String(), "admin")

	var invitationsBuf bytes.Buffer
	printInvitationsTable(&invitationsBuf, []tenant.UserInvitation{{
		ID:         "inv-1",
		TenantID:   "tenant-1",
		TenantName: "Alpha",
		Email:      "new@example.com",
		Role:       tenant.RoleAccountant,
		ExpiresAt:  now,
		CreatedAt:  now,
	}})
	assert.Contains(t, invitationsBuf.String(), "new@example.com")
	assert.Contains(t, invitationsBuf.String(), "Alpha")

	var auditEventsBuf bytes.Buffer
	printTenantAuditEventsTable(&auditEventsBuf, []tenant.TenantAuditEvent{{
		ID:          "audit-1",
		TenantID:    "tenant-1",
		ActorUserID: "user-1",
		Action:      tenant.AuditActionUserRoleUpdated,
		TargetType:  tenant.AuditTargetUser,
		TargetID:    "user-2",
		TargetEmail: "target@example.com",
		Metadata:    map[string]string{"new_role": tenant.RoleAccountant, "previous_role": tenant.RoleViewer},
		CreatedAt:   now,
	}})
	assert.Contains(t, auditEventsBuf.String(), tenant.AuditActionUserRoleUpdated)
	assert.Contains(t, auditEventsBuf.String(), "user:user-2")
	assert.Contains(t, auditEventsBuf.String(), "new_role=accountant")

	var membershipBuf bytes.Buffer
	printTenantMembership(&membershipBuf, &tenant.TenantMembership{
		Tenant: tenant.Tenant{ID: "tenant-1", Name: "Alpha", Slug: "alpha"},
		Role:   tenant.RoleViewer,
	})
	assert.Contains(t, membershipBuf.String(), "Joined tenant Alpha")

	var membershipsBuf bytes.Buffer
	printTenantMembershipsTable(&membershipsBuf, []tenant.TenantMembership{{
		Tenant:    tenant.Tenant{ID: "tenant-1", Name: "Alpha", Slug: "alpha"},
		Role:      tenant.RoleAdmin,
		IsDefault: true,
	}})
	assert.Contains(t, membershipsBuf.String(), "Alpha")
	assert.Contains(t, membershipsBuf.String(), "admin")

	var registriesBuf bytes.Buffer
	printPluginRegistriesTable(&registriesBuf, []plugin.Registry{{
		Name:       "Official",
		URL:        "https://plugins.example.com",
		IsOfficial: true,
		IsActive:   true,
	}})
	assert.Contains(t, registriesBuf.String(), "Official")

	var pluginsBuf bytes.Buffer
	printPluginsTable(&pluginsBuf, []plugin.Plugin{{
		Name:          "vat-tools",
		DisplayName:   "VAT Tools",
		Version:       "1.0.0",
		RepositoryURL: "https://github.com/example/vat-tools",
		State:         plugin.StateEnabled,
	}})
	assert.Contains(t, pluginsBuf.String(), "VAT Tools")

	var searchBuf bytes.Buffer
	printPluginSearchResultsTable(&searchBuf, []plugin.PluginSearchResult{{
		Plugin:   plugin.PluginInfo{Name: "vat-tools", DisplayName: "VAT Tools", Version: "1.0.0", Repository: "https://github.com/example/vat-tools"},
		Registry: "Official",
	}})
	assert.Contains(t, searchBuf.String(), "Official")

	var permissionsBuf bytes.Buffer
	printPluginPermissionsTable(&permissionsBuf, map[string]plugin.Permission{
		"contacts:read": {Name: "contacts:read", Category: plugin.CategoryDataAccess, Risk: plugin.RiskLow, Description: "Read contacts"},
	})
	assert.Contains(t, permissionsBuf.String(), "contacts:read")

	var tenantPluginsBuf bytes.Buffer
	printTenantPluginsTable(&tenantPluginsBuf, []plugin.TenantPlugin{{
		IsEnabled: true,
		UpdatedAt: now,
		Plugin:    &plugin.Plugin{DisplayName: "VAT Tools"},
	}})
	assert.Contains(t, tenantPluginsBuf.String(), "VAT Tools")

	var accountBuf bytes.Buffer
	account := accounting.Account{
		ID:          "account-1",
		Code:        "1000",
		Name:        "Cash",
		AccountType: accounting.AccountTypeAsset,
		IsActive:    true,
		Description: "Cash on hand",
	}
	printAccountsTable(&accountBuf, []accounting.Account{account})
	assert.Contains(t, accountBuf.String(), "CODE")
	assert.Contains(t, accountBuf.String(), "1000")

	var accountDetailBuf bytes.Buffer
	printAccount(&accountDetailBuf, &account)
	assert.Contains(t, accountDetailBuf.String(), "Account 1000 Cash")
	assert.Contains(t, accountDetailBuf.String(), "Cash on hand")

	var contactBuf bytes.Buffer
	contact := contacts.Contact{
		ID:          "contact-1",
		Name:        "Acme Corp",
		ContactType: contacts.ContactTypeCustomer,
		Email:       "hello@example.com",
		Phone:       "+372 555 1234",
		CountryCode: "EE",
		IsActive:    true,
	}
	printContactsTable(&contactBuf, []contacts.Contact{contact})
	assert.Contains(t, contactBuf.String(), "NAME")
	assert.Contains(t, contactBuf.String(), "Acme Corp")

	var contactDetailBuf bytes.Buffer
	printContact(&contactDetailBuf, &contact)
	assert.Contains(t, contactDetailBuf.String(), "Contact Acme Corp")
	assert.Contains(t, contactDetailBuf.String(), "hello@example.com")

	var employeeBuf bytes.Buffer
	employee := payroll.Employee{
		ID:                "employee-1",
		EmployeeNumber:    "EMP-001",
		FirstName:         "Mari",
		LastName:          "Maasikas",
		EmploymentType:    payroll.EmploymentFullTime,
		Email:             "mari@example.com",
		StartDate:         now,
		Position:          "Accountant",
		Department:        "Finance",
		FundedPensionRate: decimal.NewFromFloat(0.02),
		IsActive:          true,
	}
	printEmployeesTable(&employeeBuf, []payroll.Employee{employee})
	assert.Contains(t, employeeBuf.String(), "NUMBER")
	assert.Contains(t, employeeBuf.String(), "Mari Maasikas")

	var employeeDetailBuf bytes.Buffer
	printEmployee(&employeeDetailBuf, &employee)
	assert.Contains(t, employeeDetailBuf.String(), "Employee Mari Maasikas")
	assert.Contains(t, employeeDetailBuf.String(), "Position: Accountant")

	var documentBuf bytes.Buffer
	printDocumentsTable(&documentBuf, []documents.Document{{
		ID:           "doc-1",
		EntityType:   documents.EntityTypeBankTxn,
		EntityID:     "txn-1",
		DocumentType: documents.DocumentTypeReconciliation,
		FileName:     "statement.pdf",
		ReviewStatus: documents.ReviewStatusPending,
		CreatedAt:    now,
	}})
	assert.Contains(t, documentBuf.String(), "ENTITY")
	assert.Contains(t, documentBuf.String(), "statement.pdf")

	var summaryBuf bytes.Buffer
	printDocumentReviewSummariesTable(&summaryBuf, []documents.ReviewSummary{{
		EntityType:         documents.EntityTypePayment,
		EntityID:           "pay-1",
		TotalCount:         2,
		PendingReviewCount: 1,
		ReviewedCount:      1,
		MissingEvidence:    false,
		HasPendingReview:   true,
	}})
	assert.Contains(t, summaryBuf.String(), "pay-1")
	assert.Contains(t, summaryBuf.String(), "true")

	var queueBuf bytes.Buffer
	printDocumentReviewQueue(&queueBuf, &documents.ReviewQueue{
		EntityType:         documents.EntityTypeYearEndClose,
		DocumentType:       documents.DocumentTypeClosePack,
		ReviewStatus:       documents.ReviewStatusPending,
		Limit:              50,
		TotalCount:         1,
		PendingReviewCount: 1,
		Documents: []documents.Document{{
			ID:           "doc-close-pack",
			EntityType:   documents.EntityTypeYearEndClose,
			EntityID:     "year-end-close-2025",
			DocumentType: documents.DocumentTypeClosePack,
			FileName:     "close-pack.pdf",
			ReviewStatus: documents.ReviewStatusPending,
			CreatedAt:    now,
		}},
	})
	assert.Contains(t, queueBuf.String(), "Document review queue: status PENDING")
	assert.Contains(t, queueBuf.String(), "close-pack.pdf")

	var policyBuf bytes.Buffer
	printDocumentEvidencePolicy(&policyBuf, []documents.EvidencePolicyResult{{
		EntityType:         documents.EntityTypePayment,
		EntityID:           "pay-1",
		Compliant:          false,
		MissingEvidence:    true,
		DocumentTypeCounts: map[string]int{},
		RuleResults: []documents.EvidencePolicyRuleResult{{
			RuleIndex:       1,
			DocumentTypes:   []string{documents.DocumentTypeReceipt},
			RequiredCount:   1,
			RequireApproved: true,
			Compliant:       false,
			Message:         "requires at least 1 approved documents for receipt; found 0",
		}},
		Violations: []documents.EvidencePolicyRuleResult{{
			RuleIndex:       1,
			DocumentTypes:   []string{documents.DocumentTypeReceipt},
			RequiredCount:   1,
			RequireApproved: true,
			Compliant:       false,
			Message:         "requires at least 1 approved documents for receipt; found 0",
		}},
	}})
	assert.Contains(t, policyBuf.String(), "COMPLIANT")
	assert.Contains(t, policyBuf.String(), "pay-1")
	assert.Contains(t, policyBuf.String(), "requires at least 1 approved documents for receipt")

	var retentionBuf bytes.Buffer
	retentionUntil := now.AddDate(1, 0, 0)
	printDocumentRetentionReview(&retentionBuf, &documents.RetentionReview{
		AsOfDate:              "2026-03-15",
		CutoffDate:            "2026-04-14",
		TotalCount:            1,
		DueSoonCount:          1,
		MissingRetentionCount: 0,
		Documents: []documents.Document{{
			ID:             "doc-1",
			EntityType:     documents.EntityTypeBankTxn,
			EntityID:       "txn-1",
			DocumentType:   documents.DocumentTypeReconciliation,
			FileName:       "statement.pdf",
			ReviewStatus:   documents.ReviewStatusPending,
			RetentionUntil: &retentionUntil,
			CreatedAt:      now,
		}},
	})
	assert.Contains(t, retentionBuf.String(), "Document retention review")
	assert.Contains(t, retentionBuf.String(), "statement.pdf")
}

func TestPrintPaymentOutputs(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC)
	contactID := "contact-1"
	payment := payments.Payment{
		ID:             "pay-1",
		TenantID:       "tenant-1",
		PaymentNumber:  "PMT-00001",
		PaymentType:    payments.PaymentTypeReceived,
		ContactID:      &contactID,
		PaymentDate:    now,
		Amount:         decimal.NewFromInt(100),
		Currency:       "EUR",
		ExchangeRate:   decimal.NewFromInt(1),
		BaseAmount:     decimal.NewFromInt(100),
		PaymentMethod:  "BANK_TRANSFER",
		BankAccount:    "EE471000001020145685",
		Reference:      "REF-1",
		Notes:          "March receipt",
		JournalEntryID: nil,
		CreatedAt:      now,
		CreatedBy:      "user-1",
		Allocations: []payments.PaymentAllocation{{
			ID:        "alloc-1",
			TenantID:  "tenant-1",
			PaymentID: "pay-1",
			InvoiceID: "inv-1",
			Amount:    decimal.NewFromInt(60),
			CreatedAt: now,
		}},
	}

	var paymentsBuf bytes.Buffer
	printPaymentsTable(&paymentsBuf, []payments.Payment{payment})
	assert.Contains(t, paymentsBuf.String(), "PMT-00001")
	assert.Contains(t, paymentsBuf.String(), "40")

	var paymentBuf bytes.Buffer
	printPayment(&paymentBuf, &payment)
	assert.Contains(t, paymentBuf.String(), "Payment PMT-00001")
	assert.Contains(t, paymentBuf.String(), "Unallocated: 40")
	assert.Contains(t, paymentBuf.String(), "inv-1")
}

func TestPrintReminderOutputs(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC)
	ruleID := "rule-1"
	summary := invoicing.OverdueInvoicesSummary{
		TotalOverdue:       decimal.NewFromInt(500),
		InvoiceCount:       1,
		ContactCount:       1,
		AverageDaysOverdue: 12,
		GeneratedAt:        now,
		Invoices: []invoicing.OverdueInvoice{{
			ID:                "inv-1",
			InvoiceNumber:     "INV-00001",
			ContactName:       "Acme",
			DueDate:           "2026-03-01",
			OutstandingAmount: decimal.NewFromInt(500),
			Currency:          "EUR",
			DaysOverdue:       14,
			ReminderCount:     1,
		}},
	}
	reminder := invoicing.PaymentReminder{
		ID:             "rem-1",
		InvoiceID:      "inv-1",
		InvoiceNumber:  "INV-00001",
		ContactName:    "Acme",
		RuleID:         &ruleID,
		ReminderNumber: 2,
		Status:         invoicing.ReminderStatusSent,
		SentAt:         &now,
	}
	rule := invoicing.ReminderRule{
		ID:                "rule-1",
		Name:              "Seven days overdue",
		TriggerType:       invoicing.TriggerAfterDue,
		DaysOffset:        7,
		EmailTemplateType: "OVERDUE_REMINDER",
		IsActive:          true,
	}
	triggerResult := invoicing.AutomatedReminderResult{
		RuleName:      "Seven days overdue",
		InvoicesFound: 2,
		RemindersSent: 1,
		Skipped:       1,
		RunAt:         now,
	}

	var overdueBuf bytes.Buffer
	printOverdueInvoicesSummary(&overdueBuf, &summary)
	assert.Contains(t, overdueBuf.String(), "Total overdue: 500")
	assert.Contains(t, overdueBuf.String(), "INV-00001")

	var remindersBuf bytes.Buffer
	printPaymentRemindersTable(&remindersBuf, []invoicing.PaymentReminder{reminder})
	assert.Contains(t, remindersBuf.String(), "INV-00001")
	assert.Contains(t, remindersBuf.String(), "SENT")

	var resultBuf bytes.Buffer
	printReminderResult(&resultBuf, &invoicing.ReminderResult{InvoiceID: "inv-1", InvoiceNumber: "INV-00001", Success: true, ReminderID: "rem-1", Message: "sent"})
	assert.Contains(t, resultBuf.String(), "Reminder ID: rem-1")

	var bulkBuf bytes.Buffer
	printBulkReminderResult(&bulkBuf, &invoicing.BulkReminderResult{TotalRequested: 2, Successful: 1, Failed: 1, Results: []invoicing.ReminderResult{{InvoiceID: "inv-1", InvoiceNumber: "INV-00001", Success: true, Message: "sent"}}})
	assert.Contains(t, bulkBuf.String(), "Requested: 2")
	assert.Contains(t, bulkBuf.String(), "INV-00001")

	var rulesBuf bytes.Buffer
	printReminderRulesTable(&rulesBuf, []invoicing.ReminderRule{rule})
	assert.Contains(t, rulesBuf.String(), "Seven days overdue")
	assert.Contains(t, rulesBuf.String(), "AFTER_DUE")

	var ruleBuf bytes.Buffer
	printReminderRule(&ruleBuf, &rule)
	assert.Contains(t, ruleBuf.String(), "Reminder rule Seven days overdue")
	assert.Contains(t, ruleBuf.String(), "Active: true")

	var triggerBuf bytes.Buffer
	printAutomatedReminderResultsTable(&triggerBuf, []invoicing.AutomatedReminderResult{triggerResult})
	assert.Contains(t, triggerBuf.String(), "Seven days overdue")
	assert.Contains(t, triggerBuf.String(), "1")
}

func TestPrintEmailOutputs(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC)
	smtpConfig := email.SMTPConfig{
		Host:      "smtp.example.com",
		Port:      587,
		Username:  "robot",
		FromEmail: "billing@example.com",
		FromName:  "Billing",
		UseTLS:    true,
	}
	template := email.EmailTemplate{
		ID:           "tmpl-1",
		TemplateType: email.TemplateOverdueReminder,
		Subject:      "Reminder",
		BodyHTML:     "<p>Reminder</p>",
		BodyText:     "Reminder",
		IsActive:     true,
		UpdatedAt:    now,
	}
	log := email.EmailLog{
		ID:             "email-1",
		EmailType:      string(email.TemplateInvoiceSend),
		RecipientEmail: "billing@example.com",
		Subject:        "Invoice",
		Status:         email.StatusSent,
		SentAt:         &now,
		CreatedAt:      now,
	}

	var smtpBuf bytes.Buffer
	printSMTPConfig(&smtpBuf, &smtpConfig)
	assert.Contains(t, smtpBuf.String(), "smtp.example.com")
	assert.Contains(t, smtpBuf.String(), "Configured: true")

	var testBuf bytes.Buffer
	printSMTPTestResponse(&testBuf, &email.TestSMTPResponse{Success: true, Message: "Test email sent successfully"})
	assert.Contains(t, testBuf.String(), "Success: true")

	var templatesBuf bytes.Buffer
	printEmailTemplatesTable(&templatesBuf, []email.EmailTemplate{template})
	assert.Contains(t, templatesBuf.String(), "OVERDUE_REMINDER")
	assert.Contains(t, templatesBuf.String(), "Reminder")

	var templateBuf bytes.Buffer
	printEmailTemplate(&templateBuf, &template)
	assert.Contains(t, templateBuf.String(), "Email template OVERDUE_REMINDER")
	assert.Contains(t, templateBuf.String(), "Body HTML bytes")

	var logsBuf bytes.Buffer
	printEmailLogsTable(&logsBuf, []email.EmailLog{log})
	assert.Contains(t, logsBuf.String(), "email-1")
	assert.Contains(t, logsBuf.String(), "billing@example.com")

	var sentBuf bytes.Buffer
	printEmailSentResponse(&sentBuf, &email.EmailSentResponse{Success: true, LogID: "email-1", Message: "sent"})
	assert.Contains(t, sentBuf.String(), "Email sent")
	assert.Contains(t, sentBuf.String(), "Log ID: email-1")
}

func TestPrintInterestOutputs(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC)
	result := invoicing.InterestCalculationResult{
		InvoiceID:         "inv-1",
		InvoiceNumber:     "INV-00001",
		DueDate:           time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
		DaysOverdue:       14,
		OutstandingAmount: decimal.NewFromInt(500),
		InterestRate:      decimal.RequireFromString("0.0005"),
		DailyInterest:     decimal.RequireFromString("0.25"),
		TotalInterest:     decimal.RequireFromString("3.50"),
		TotalWithInterest: decimal.RequireFromString("503.50"),
		CalculatedAt:      now,
		Currency:          "EUR",
	}
	history := invoicing.InvoiceInterest{
		ID:                "interest-1",
		InvoiceID:         "inv-1",
		CalculatedAt:      now,
		DaysOverdue:       14,
		PrincipalAmount:   decimal.NewFromInt(500),
		InterestRate:      decimal.RequireFromString("0.0005"),
		InterestAmount:    decimal.RequireFromString("3.50"),
		TotalWithInterest: decimal.RequireFromString("503.50"),
		CreatedAt:         now,
	}

	var settingsBuf bytes.Buffer
	printInterestSettings(&settingsBuf, &invoicing.InterestSettings{Rate: 0.0005, AnnualRate: 0.1825, Description: "0.050% daily", IsEnabled: true})
	assert.Contains(t, settingsBuf.String(), "Enabled: true")
	assert.Contains(t, settingsBuf.String(), "Daily rate: 0.000500")

	var tableBuf bytes.Buffer
	printInterestCalculationsTable(&tableBuf, []invoicing.InterestCalculationResult{result})
	assert.Contains(t, tableBuf.String(), "INV-00001")
	assert.Contains(t, tableBuf.String(), "503.5")

	var resultBuf bytes.Buffer
	printInterestCalculation(&resultBuf, &result)
	assert.Contains(t, resultBuf.String(), "Interest for invoice INV-00001")
	assert.Contains(t, resultBuf.String(), "Total interest: 3.5")

	var historyBuf bytes.Buffer
	printInvoiceInterestHistoryTable(&historyBuf, []invoicing.InvoiceInterest{history})
	assert.Contains(t, historyBuf.String(), "interest-1")
	assert.Contains(t, historyBuf.String(), "503.5")
}

func TestPrintCloseOutputs(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC)
	lockBefore := "2026-02-28"
	lockAfter := "2026-03-31"
	event := tenant.PeriodCloseEvent{
		ID:              "close-1",
		Action:          tenant.PeriodCloseActionClose,
		CloseKind:       tenant.PeriodCloseKindMonthEnd,
		PeriodEndDate:   "2026-03-31",
		LockDateBefore:  &lockBefore,
		LockDateAfter:   &lockAfter,
		Note:            "March close",
		ReviewerSignOff: true,
		CreatedAt:       now,
	}
	status := accounting.YearEndCloseStatus{
		PeriodEndDate:              "2025-12-31",
		FiscalYearLabel:            "2025",
		FiscalYearEndDate:          "2025-12-31",
		CarryForwardDate:           "2026-01-01",
		IsFiscalYearEnd:            true,
		PeriodClosed:               true,
		CarryForwardNeeded:         true,
		CarryForwardReady:          true,
		HasRetainedEarningsAccount: true,
		RetainedEarningsAccount:    &accounting.AccountSummary{ID: "acc-retained", Code: "2999", Name: "Retained earnings"},
		NetIncome:                  decimal.NewFromInt(1200),
		ClosePackEvidenceEntityID:  "11111111-1111-5111-8111-111111111111",
		ClosePackEvidence: &documents.EvidencePolicyResult{
			EntityType: documents.EntityTypeYearEndClose,
			EntityID:   "11111111-1111-5111-8111-111111111111",
			Compliant:  true,
		},
	}
	result := accounting.YearEndCarryForwardResult{
		JournalEntry: &accounting.JournalEntry{ID: "je-1", EntryNumber: "JE-2026-001", Status: accounting.StatusPosted},
		Status:       &status,
	}
	reversalResult := accounting.YearEndCarryForwardReversalResult{
		ReversalJournalEntry: &accounting.JournalEntry{ID: "je-2", EntryNumber: "JE-2026-002", EntryDate: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), Status: accounting.StatusPosted},
		Status:               &status,
	}
	closePack := accounting.YearEndClosePack{
		Status: &status,
		TrialBalance: &accounting.TrialBalance{
			TotalDebits:  decimal.NewFromInt(1000),
			TotalCredits: decimal.NewFromInt(1000),
			IsBalanced:   true,
		},
		BalanceSheet: &accounting.BalanceSheet{
			TotalAssets:      decimal.NewFromInt(1500),
			TotalLiabilities: decimal.NewFromInt(300),
			TotalEquity:      decimal.NewFromInt(1200),
			IsBalanced:       true,
		},
		IncomeStatement: &accounting.IncomeStatement{
			TotalRevenue:  decimal.NewFromInt(2000),
			TotalExpenses: decimal.NewFromInt(800),
			NetIncome:     decimal.NewFromInt(1200),
		},
	}

	var eventsBuf bytes.Buffer
	printPeriodCloseEventsTable(&eventsBuf, []tenant.PeriodCloseEvent{event})
	assert.Contains(t, eventsBuf.String(), "close-1")
	assert.Contains(t, eventsBuf.String(), "true")
	assert.Contains(t, eventsBuf.String(), "March close")

	var mutationBuf bytes.Buffer
	printPeriodCloseMutationResponse(&mutationBuf, "Closed period", &periodCloseMutationResponse{
		Tenant: &tenant.Tenant{Settings: tenant.TenantSettings{PeriodLockDate: &lockAfter}},
		Event:  &event,
	})
	assert.Contains(t, mutationBuf.String(), "Closed period")
	assert.Contains(t, mutationBuf.String(), "Reviewer sign-off: true")
	assert.Contains(t, mutationBuf.String(), "Tenant lock date: 2026-03-31")

	var statusBuf bytes.Buffer
	printYearEndCloseStatus(&statusBuf, &status)
	assert.Contains(t, statusBuf.String(), "Year-end close status 2025")
	assert.Contains(t, statusBuf.String(), "Carry-forward ready: true")
	assert.Contains(t, statusBuf.String(), "Close-pack evidence compliant: true")

	var packBuf bytes.Buffer
	printYearEndClosePack(&packBuf, &closePack)
	assert.Contains(t, packBuf.String(), "Trial balance: debits 1000")
	assert.Contains(t, packBuf.String(), "Income statement: revenue 2000")

	var annualBuf bytes.Buffer
	printAnnualReport(&annualBuf, &reports.AnnualReport{
		TenantID:            "tenant-1",
		PeriodEndDate:       "2025-12-31",
		FiscalYearLabel:     "2025",
		FiscalYearStartDate: "2025-01-01",
		FiscalYearEndDate:   "2025-12-31",
		CloseStatus:         &status,
		TrialBalance:        closePack.TrialBalance,
		BalanceSheet:        closePack.BalanceSheet,
		IncomeStatement:     closePack.IncomeStatement,
		CashFlowStatement: &reports.CashFlowStatement{
			Method:        reports.CashFlowMethodIndirect,
			NetCashChange: decimal.NewFromInt(700),
			ClosingCash:   decimal.NewFromInt(1700),
		},
	})
	assert.Contains(t, annualBuf.String(), "Annual report 2025")
	assert.Contains(t, annualBuf.String(), "Fiscal year: 2025-01-01 to 2025-12-31")
	assert.Contains(t, annualBuf.String(), "Cash flow: method indirect")

	var auditBuf bytes.Buffer
	printYearEndCloseAuditEvidence(&auditBuf, &accounting.YearEndCloseAuditEvidence{
		Pack:           &closePack,
		EvidencePolicy: status.ClosePackEvidence,
		Documents: []documents.Document{{
			ID:           "doc-close-pack",
			EntityType:   documents.EntityTypeYearEndClose,
			EntityID:     "11111111-1111-5111-8111-111111111111",
			DocumentType: documents.DocumentTypeClosePack,
			FileName:     "close-pack.pdf",
			ReviewStatus: documents.ReviewStatusApproved,
			CreatedAt:    now,
		}},
		GeneratedAt: now,
	})
	assert.Contains(t, auditBuf.String(), "Close-pack audit evidence generated")
	assert.Contains(t, auditBuf.String(), "Evidence policy compliant: true")
	assert.Contains(t, auditBuf.String(), "close-pack.pdf")

	var resultBuf bytes.Buffer
	printYearEndCarryForwardResult(&resultBuf, &result)
	assert.Contains(t, resultBuf.String(), "Created year-end carry-forward JE-2026-001")
	assert.Contains(t, resultBuf.String(), "Status: POSTED")

	var reversalBuf bytes.Buffer
	printYearEndCarryForwardReversalResult(&reversalBuf, &reversalResult)
	assert.Contains(t, reversalBuf.String(), "Reversed year-end carry-forward JE-2026-002")
	assert.Contains(t, reversalBuf.String(), "Reversal date: 2026-01-01")
}

func TestPrintBankingOutputs(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC)
	completedAt := now.Add(2 * time.Hour)
	glAccountID := "acc-bank"
	paymentID := "pay-1"
	reconciliationID := "rec-1"
	account := banking.BankAccount{
		ID:            "bank-1",
		TenantID:      "tenant-1",
		Name:          "Main bank",
		AccountNumber: "EE471000001020145685",
		BankName:      "LHV",
		SwiftCode:     "LHVBEE22",
		Currency:      "EUR",
		GLAccountID:   &glAccountID,
		IsDefault:     true,
		IsActive:      true,
		CreatedAt:     now,
		Balance:       decimal.NewFromInt(100),
	}
	matchRule := banking.BankMatchRule{
		ID:                 "rule-1",
		TenantID:           "tenant-1",
		BankAccountID:      &account.ID,
		Name:               "Stripe receipts",
		Priority:           10,
		MatchField:         banking.BankMatchFieldDescription,
		Pattern:            "stripe",
		MinConfidence:      0.85,
		MaxDateDiffDays:    3,
		RequireExactAmount: true,
		IsActive:           true,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	transaction := banking.BankTransaction{
		ID:                  "tx-1",
		TenantID:            "tenant-1",
		BankAccountID:       "bank-1",
		TransactionDate:     now,
		ValueDate:           &now,
		Amount:              decimal.NewFromInt(100),
		Currency:            "EUR",
		Description:         "Client payment",
		Reference:           "REF-1",
		CounterpartyName:    "Acme",
		CounterpartyAccount: "EE111",
		Status:              banking.StatusMatched,
		FollowUpStatus:      banking.FollowUpReadyToMatch,
		ReviewNote:          "Ready",
		MatchedPaymentID:    &paymentID,
		ReconciliationID:    &reconciliationID,
		ImportedAt:          now,
	}
	result := banking.ImportResult{
		ImportID:             "import-1",
		TransactionsImported: 1,
		TransactionsMatched:  0,
		DuplicatesSkipped:    0,
	}
	statementImport := banking.BankStatementImport{
		ID:                   "import-1",
		TenantID:             "tenant-1",
		BankAccountID:        "bank-1",
		FileName:             "bank.csv",
		TransactionsImported: 1,
		TransactionsMatched:  0,
		DuplicatesSkipped:    0,
		CreatedAt:            now,
	}
	suggestion := banking.MatchSuggestion{
		PaymentID:     "pay-1",
		PaymentNumber: "PMT-00001",
		PaymentDate:   now,
		Amount:        decimal.NewFromInt(100),
		ContactName:   "Acme",
		Confidence:    0.95,
		MatchReason:   "Amount and reference match",
	}
	reconciliation := banking.BankReconciliation{
		ID:             "rec-1",
		TenantID:       "tenant-1",
		BankAccountID:  "bank-1",
		StatementDate:  now,
		OpeningBalance: decimal.Zero,
		ClosingBalance: decimal.NewFromInt(100),
		Status:         banking.ReconciliationInProgress,
		CompletedAt:    &completedAt,
		CreatedAt:      now,
		CreatedBy:      "user-1",
	}

	var accountsBuf bytes.Buffer
	printBankAccountsTable(&accountsBuf, []banking.BankAccount{account})
	assert.Contains(t, accountsBuf.String(), "Main bank")
	assert.Contains(t, accountsBuf.String(), "EE471000001020145685")

	var accountBuf bytes.Buffer
	printBankAccount(&accountBuf, &account)
	assert.Contains(t, accountBuf.String(), "Bank account Main bank")
	assert.Contains(t, accountBuf.String(), "GL account: acc-bank")

	var matchRulesBuf bytes.Buffer
	printBankMatchRulesTable(&matchRulesBuf, []banking.BankMatchRule{matchRule})
	assert.Contains(t, matchRulesBuf.String(), "Stripe receipts")
	assert.Contains(t, matchRulesBuf.String(), "0.85")

	var matchRuleBuf bytes.Buffer
	printBankMatchRule(&matchRuleBuf, &matchRule)
	assert.Contains(t, matchRuleBuf.String(), "Bank match rule Stripe receipts")
	assert.Contains(t, matchRuleBuf.String(), "Require exact amount: true")

	var transactionsBuf bytes.Buffer
	printBankTransactionsTable(&transactionsBuf, []banking.BankTransaction{transaction})
	assert.Contains(t, transactionsBuf.String(), "Client payment")
	assert.Contains(t, transactionsBuf.String(), "READY_TO_MATCH")

	var transactionBuf bytes.Buffer
	printBankTransaction(&transactionBuf, &transaction)
	assert.Contains(t, transactionBuf.String(), "Bank transaction tx-1")
	assert.Contains(t, transactionBuf.String(), "Matched payment: pay-1")
	assert.Contains(t, transactionBuf.String(), "Review note: Ready")

	var resultBuf bytes.Buffer
	printBankImportResult(&resultBuf, &result)
	assert.Contains(t, resultBuf.String(), "Import import-1")
	assert.Contains(t, resultBuf.String(), "Imported: 1")

	var importsBuf bytes.Buffer
	printBankImportsTable(&importsBuf, []banking.BankStatementImport{statementImport})
	assert.Contains(t, importsBuf.String(), "bank.csv")
	assert.Contains(t, importsBuf.String(), "import-1")

	var suggestionsBuf bytes.Buffer
	printMatchSuggestionsTable(&suggestionsBuf, []banking.MatchSuggestion{suggestion})
	assert.Contains(t, suggestionsBuf.String(), "PMT-00001")
	assert.Contains(t, suggestionsBuf.String(), "0.95")

	var reconciliationsBuf bytes.Buffer
	printBankReconciliationsTable(&reconciliationsBuf, []banking.BankReconciliation{reconciliation})
	assert.Contains(t, reconciliationsBuf.String(), "rec-1")
	assert.Contains(t, reconciliationsBuf.String(), "IN_PROGRESS")

	var reconciliationBuf bytes.Buffer
	printBankReconciliation(&reconciliationBuf, &reconciliation)
	assert.Contains(t, reconciliationBuf.String(), "Bank reconciliation rec-1")
	assert.Contains(t, reconciliationBuf.String(), "Closing balance: 100")
}

func TestPrintInvoiceOutputs(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC)
	invoice := invoicing.Invoice{
		ID:            "inv-1",
		TenantID:      "tenant-1",
		InvoiceNumber: "INV-00001",
		InvoiceType:   invoicing.InvoiceTypeSales,
		ContactID:     "contact-1",
		Contact:       &contacts.Contact{Name: "Acme"},
		IssueDate:     now,
		DueDate:       now.AddDate(0, 0, 14),
		Currency:      "EUR",
		ExchangeRate:  decimal.NewFromInt(1),
		Subtotal:      decimal.NewFromInt(180),
		VATAmount:     decimal.NewFromFloat(39.6),
		Total:         decimal.NewFromFloat(219.6),
		BaseSubtotal:  decimal.NewFromInt(180),
		BaseVATAmount: decimal.NewFromFloat(39.6),
		BaseTotal:     decimal.NewFromFloat(219.6),
		AmountPaid:    decimal.NewFromInt(20),
		Status:        invoicing.StatusDraft,
		Reference:     "REF-1",
		Notes:         "March services",
		CreatedAt:     now,
		CreatedBy:     "user-1",
		UpdatedAt:     now,
		Lines: []invoicing.InvoiceLine{{
			LineNumber:   1,
			Description:  "Consulting",
			Quantity:     decimal.NewFromInt(2),
			Unit:         "hour",
			UnitPrice:    decimal.NewFromInt(100),
			VATRate:      decimal.NewFromInt(22),
			LineSubtotal: decimal.NewFromInt(180),
			LineVAT:      decimal.NewFromFloat(39.6),
			LineTotal:    decimal.NewFromFloat(219.6),
		}},
	}

	var invoicesBuf bytes.Buffer
	printInvoicesTable(&invoicesBuf, []invoicing.Invoice{invoice})
	assert.Contains(t, invoicesBuf.String(), "INV-00001")
	assert.Contains(t, invoicesBuf.String(), "199.6")

	var invoiceBuf bytes.Buffer
	printInvoice(&invoiceBuf, &invoice)
	assert.Contains(t, invoiceBuf.String(), "Invoice INV-00001")
	assert.Contains(t, invoiceBuf.String(), "Due amount: 199.6")
	assert.Contains(t, invoiceBuf.String(), "Consulting")
}

func TestPrintQuoteOutputs(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC)
	validUntil := now.AddDate(0, 0, 30)
	quote := quotes.Quote{
		ID:           "quote-1",
		TenantID:     "tenant-1",
		QuoteNumber:  "QUO-00001",
		ContactID:    "contact-1",
		Contact:      &contacts.Contact{Name: "Acme"},
		QuoteDate:    now,
		ValidUntil:   &validUntil,
		Status:       quotes.QuoteStatusDraft,
		Currency:     "EUR",
		ExchangeRate: decimal.NewFromInt(1),
		Subtotal:     decimal.NewFromInt(180),
		VATAmount:    decimal.NewFromFloat(39.6),
		Total:        decimal.NewFromFloat(219.6),
		Notes:        "March offer",
		CreatedAt:    now,
		CreatedBy:    "user-1",
		UpdatedAt:    now,
		Lines: []quotes.QuoteLine{{
			LineNumber:   1,
			Description:  "Consulting",
			Quantity:     decimal.NewFromInt(2),
			Unit:         "hour",
			UnitPrice:    decimal.NewFromInt(100),
			VATRate:      decimal.NewFromInt(22),
			LineSubtotal: decimal.NewFromInt(180),
			LineVAT:      decimal.NewFromFloat(39.6),
			LineTotal:    decimal.NewFromFloat(219.6),
		}},
	}

	var quotesBuf bytes.Buffer
	printQuotesTable(&quotesBuf, []quotes.Quote{quote})
	assert.Contains(t, quotesBuf.String(), "QUO-00001")
	assert.Contains(t, quotesBuf.String(), "Acme")

	var quoteBuf bytes.Buffer
	printQuote(&quoteBuf, &quote)
	assert.Contains(t, quoteBuf.String(), "Quote QUO-00001")
	assert.Contains(t, quoteBuf.String(), "Valid until: 2026-04-14")
	assert.Contains(t, quoteBuf.String(), "Consulting")
}

func TestPrintOrderOutputs(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC)
	expectedDelivery := now.AddDate(0, 0, 7)
	quoteID := "quote-1"
	invoiceID := "inv-1"
	order := orders.Order{
		ID:                   "order-1",
		TenantID:             "tenant-1",
		OrderNumber:          "ORD-00001",
		ContactID:            "contact-1",
		Contact:              &contacts.Contact{Name: "Acme"},
		OrderDate:            now,
		ExpectedDelivery:     &expectedDelivery,
		Status:               orders.OrderStatusConfirmed,
		Currency:             "EUR",
		ExchangeRate:         decimal.NewFromInt(1),
		Subtotal:             decimal.NewFromInt(180),
		VATAmount:            decimal.NewFromFloat(39.6),
		Total:                decimal.NewFromFloat(219.6),
		Notes:                "March order",
		QuoteID:              &quoteID,
		ConvertedToInvoiceID: &invoiceID,
		CreatedAt:            now,
		CreatedBy:            "user-1",
		UpdatedAt:            now,
		Lines: []orders.OrderLine{{
			LineNumber:   1,
			Description:  "Consulting",
			Quantity:     decimal.NewFromInt(2),
			Unit:         "hour",
			UnitPrice:    decimal.NewFromInt(100),
			VATRate:      decimal.NewFromInt(22),
			LineSubtotal: decimal.NewFromInt(180),
			LineVAT:      decimal.NewFromFloat(39.6),
			LineTotal:    decimal.NewFromFloat(219.6),
		}},
	}

	var ordersBuf bytes.Buffer
	printOrdersTable(&ordersBuf, []orders.Order{order})
	assert.Contains(t, ordersBuf.String(), "ORD-00001")
	assert.Contains(t, ordersBuf.String(), "Acme")

	var orderBuf bytes.Buffer
	printOrder(&orderBuf, &order)
	assert.Contains(t, orderBuf.String(), "Order ORD-00001")
	assert.Contains(t, orderBuf.String(), "Expected delivery: 2026-03-22")
	assert.Contains(t, orderBuf.String(), "Converted invoice: inv-1")
	assert.Contains(t, orderBuf.String(), "Consulting")
}

func TestPrintRecurringInvoiceOutputs(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC)
	endDate := now.AddDate(0, 9, 16)
	recurringInvoice := recurring.RecurringInvoice{
		ID:                     "rec-1",
		TenantID:               "tenant-1",
		Name:                   "Monthly retainer",
		ContactID:              "contact-1",
		ContactName:            "Acme",
		InvoiceType:            "SALES",
		Currency:               "EUR",
		Frequency:              recurring.FrequencyMonthly,
		StartDate:              now,
		EndDate:                &endDate,
		NextGenerationDate:     now.AddDate(0, 1, 0),
		PaymentTermsDays:       21,
		Reference:              "RET-1",
		Notes:                  "Monthly services",
		IsActive:               true,
		GeneratedCount:         2,
		CreatedAt:              now,
		CreatedBy:              "user-1",
		UpdatedAt:              now,
		SendEmailOnGeneration:  true,
		EmailTemplateType:      "INVOICE_SEND",
		RecipientEmailOverride: "billing@example.com",
		AttachPDFToEmail:       true,
		EmailSubjectOverride:   "Monthly invoice",
		EmailMessage:           "Please see attached invoice.",
		Lines: []recurring.RecurringInvoiceLine{{
			LineNumber:      1,
			Description:     "Consulting",
			Quantity:        decimal.NewFromInt(2),
			Unit:            "hour",
			UnitPrice:       decimal.NewFromInt(100),
			DiscountPercent: decimal.NewFromInt(10),
			VATRate:         decimal.NewFromInt(22),
		}},
	}
	result := recurring.GenerationResult{
		RecurringInvoiceID:     "rec-1",
		GeneratedInvoiceID:     "inv-1",
		GeneratedInvoiceNumber: "INV-00001",
		EmailSent:              true,
		EmailStatus:            "SENT",
	}

	var tableBuf bytes.Buffer
	printRecurringInvoicesTable(&tableBuf, []recurring.RecurringInvoice{recurringInvoice})
	assert.Contains(t, tableBuf.String(), "Monthly retainer")
	assert.Contains(t, tableBuf.String(), "Acme")

	var detailBuf bytes.Buffer
	printRecurringInvoice(&detailBuf, &recurringInvoice)
	assert.Contains(t, detailBuf.String(), "Recurring invoice Monthly retainer")
	assert.Contains(t, detailBuf.String(), "Email recipient: billing@example.com")
	assert.Contains(t, detailBuf.String(), "Consulting")

	var resultsBuf bytes.Buffer
	printRecurringGenerationResultsTable(&resultsBuf, []recurring.GenerationResult{result})
	assert.Contains(t, resultsBuf.String(), "INV-00001")
	assert.Contains(t, resultsBuf.String(), "SENT")
}

func TestPrintAssetOutputs(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC)
	category := assets.AssetCategory{
		ID:                          "cat-1",
		TenantID:                    "tenant-1",
		Name:                        "Equipment",
		Description:                 "Office equipment",
		DepreciationMethod:          assets.DepreciationStraightLine,
		DefaultUsefulLifeMonths:     60,
		DefaultResidualValuePercent: decimal.NewFromInt(10),
		CreatedAt:                   now,
		UpdatedAt:                   now,
	}
	categoryID := "cat-1"
	supplierID := "supplier-1"
	depStart := now
	lastDep := now.AddDate(0, 1, 0)
	asset := assets.FixedAsset{
		ID:                      "asset-1",
		TenantID:                "tenant-1",
		AssetNumber:             "FA-00001",
		Name:                    "Laptop",
		Description:             "Developer laptop",
		CategoryID:              &categoryID,
		Status:                  assets.AssetStatusActive,
		PurchaseDate:            now,
		PurchaseCost:            decimal.NewFromInt(1200),
		SupplierID:              &supplierID,
		SerialNumber:            "SN-1",
		Location:                "Tallinn",
		DepreciationMethod:      assets.DepreciationStraightLine,
		UsefulLifeMonths:        36,
		ResidualValue:           decimal.NewFromInt(100),
		DepreciationStartDate:   &depStart,
		AccumulatedDepreciation: decimal.NewFromInt(50),
		BookValue:               decimal.NewFromInt(1150),
		LastDepreciationDate:    &lastDep,
		CreatedAt:               now,
		CreatedBy:               "user-1",
		UpdatedAt:               now,
	}
	entry := assets.DepreciationEntry{
		ID:                 "dep-1",
		TenantID:           "tenant-1",
		AssetID:            "asset-1",
		DepreciationDate:   now,
		PeriodStart:        now,
		PeriodEnd:          now.AddDate(0, 1, -1),
		DepreciationAmount: decimal.NewFromInt(25),
		AccumulatedTotal:   decimal.NewFromInt(75),
		BookValueAfter:     decimal.NewFromInt(1125),
		CreatedAt:          now,
		CreatedBy:          "user-1",
	}

	var categoriesBuf bytes.Buffer
	printAssetCategoriesTable(&categoriesBuf, []assets.AssetCategory{category})
	assert.Contains(t, categoriesBuf.String(), "Equipment")

	var categoryBuf bytes.Buffer
	printAssetCategory(&categoryBuf, &category)
	assert.Contains(t, categoryBuf.String(), "Office equipment")

	var assetsBuf bytes.Buffer
	printAssetsTable(&assetsBuf, []assets.FixedAsset{asset})
	assert.Contains(t, assetsBuf.String(), "FA-00001")
	assert.Contains(t, assetsBuf.String(), "1150")

	var assetBuf bytes.Buffer
	printAsset(&assetBuf, &asset)
	assert.Contains(t, assetBuf.String(), "Asset FA-00001 Laptop")
	assert.Contains(t, assetBuf.String(), "Serial number: SN-1")

	var depreciationBuf bytes.Buffer
	printDepreciationEntriesTable(&depreciationBuf, []assets.DepreciationEntry{entry})
	assert.Contains(t, depreciationBuf.String(), "dep-1")
	assert.Contains(t, depreciationBuf.String(), "25")
}

func TestPrintInventoryOutputs(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC)
	category := inventory.ProductCategory{
		ID:          "cat-1",
		TenantID:    "tenant-1",
		Name:        "Parts",
		Description: "Spare parts",
		ParentID:    "parent-1",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	product := inventory.Product{
		ID:                 "prod-1",
		TenantID:           "tenant-1",
		Code:               "PRD-001",
		Name:               "Widget",
		Description:        "Inventory item",
		ProductType:        inventory.ProductTypeGoods,
		CategoryID:         "cat-1",
		Unit:               "pcs",
		PurchasePrice:      decimal.NewFromFloat(10.5),
		SalesPrice:         decimal.NewFromInt(15),
		VATRate:            decimal.NewFromInt(22),
		MinStockLevel:      decimal.NewFromInt(5),
		CurrentStock:       decimal.NewFromInt(12),
		ReorderPoint:       decimal.NewFromInt(7),
		SaleAccountID:      "acc-sale",
		PurchaseAccountID:  "acc-purchase",
		InventoryAccountID: "acc-inventory",
		TrackInventory:     true,
		IsActive:           true,
		Barcode:            "123456",
		SupplierID:         "supplier-1",
		LeadTimeDays:       4,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	warehouse := inventory.Warehouse{
		ID:        "wh-1",
		TenantID:  "tenant-1",
		Code:      "MAIN",
		Name:      "Main warehouse",
		Address:   "Tallinn",
		IsDefault: true,
		IsActive:  true,
		CreatedAt: now,
		UpdatedAt: now,
	}
	level := inventory.StockLevel{
		ID:           "stock-1",
		TenantID:     "tenant-1",
		ProductID:    "prod-1",
		WarehouseID:  "wh-1",
		Quantity:     decimal.NewFromInt(12),
		ReservedQty:  decimal.NewFromInt(2),
		AvailableQty: decimal.NewFromInt(10),
		LastUpdated:  now,
	}
	movement := inventory.InventoryMovement{
		ID:           "mov-1",
		TenantID:     "tenant-1",
		ProductID:    "prod-1",
		WarehouseID:  "wh-1",
		MovementType: inventory.MovementTypeAdjustment,
		Quantity:     decimal.NewFromInt(2),
		UnitCost:     decimal.NewFromFloat(10.5),
		TotalCost:    decimal.NewFromInt(21),
		Reference:    "ADJ-1",
		Notes:        "Cycle count",
		MovementDate: now,
		CreatedAt:    now,
		CreatedBy:    "user-1",
	}
	valuation := inventory.InventoryValuationReport{
		TenantID:        "tenant-1",
		WarehouseID:     "wh-1",
		ValuationMethod: inventory.InventoryValuationMethodStandardCost,
		Lines: []inventory.InventoryValuationLine{
			{
				ProductID:      "prod-1",
				ProductCode:    "PRD-001",
				ProductName:    "Widget",
				WarehouseID:    "wh-1",
				WarehouseCode:  "MAIN",
				WarehouseName:  "Main warehouse",
				Quantity:       decimal.NewFromInt(12),
				ReservedQty:    decimal.NewFromInt(2),
				AvailableQty:   decimal.NewFromInt(10),
				UnitCost:       decimal.NewFromFloat(10.5),
				InventoryValue: decimal.NewFromInt(126),
			},
		},
		TotalQuantity:  decimal.NewFromInt(12),
		TotalReserved:  decimal.NewFromInt(2),
		TotalAvailable: decimal.NewFromInt(10),
		TotalValue:     decimal.NewFromInt(126),
		GeneratedAt:    now,
	}

	var categoriesBuf bytes.Buffer
	printProductCategoriesTable(&categoriesBuf, []inventory.ProductCategory{category})
	assert.Contains(t, categoriesBuf.String(), "Parts")
	assert.Contains(t, categoriesBuf.String(), "parent-1")

	var categoryBuf bytes.Buffer
	printProductCategory(&categoryBuf, &category)
	assert.Contains(t, categoryBuf.String(), "Product category Parts")
	assert.Contains(t, categoryBuf.String(), "Spare parts")

	var productsBuf bytes.Buffer
	printProductsTable(&productsBuf, []inventory.Product{product})
	assert.Contains(t, productsBuf.String(), "PRD-001")
	assert.Contains(t, productsBuf.String(), "12")

	var productBuf bytes.Buffer
	printProduct(&productBuf, &product)
	assert.Contains(t, productBuf.String(), "Product PRD-001 Widget")
	assert.Contains(t, productBuf.String(), "Track inventory: true")

	var warehousesBuf bytes.Buffer
	printWarehousesTable(&warehousesBuf, []inventory.Warehouse{warehouse})
	assert.Contains(t, warehousesBuf.String(), "MAIN")
	assert.Contains(t, warehousesBuf.String(), "Tallinn")

	var warehouseBuf bytes.Buffer
	printWarehouse(&warehouseBuf, &warehouse)
	assert.Contains(t, warehouseBuf.String(), "Warehouse MAIN Main warehouse")
	assert.Contains(t, warehouseBuf.String(), "Default: true")

	var stockBuf bytes.Buffer
	printStockLevelsTable(&stockBuf, []inventory.StockLevel{level})
	assert.Contains(t, stockBuf.String(), "AVAILABLE")
	assert.Contains(t, stockBuf.String(), "10")

	var movementsBuf bytes.Buffer
	printInventoryMovementsTable(&movementsBuf, []inventory.InventoryMovement{movement})
	assert.Contains(t, movementsBuf.String(), "ADJUSTMENT")
	assert.Contains(t, movementsBuf.String(), "Cycle count")

	var valuationBuf bytes.Buffer
	printInventoryValuation(&valuationBuf, &valuation)
	assert.Contains(t, valuationBuf.String(), "Inventory valuation (STANDARD_COST)")
	assert.Contains(t, valuationBuf.String(), "PRD-001 Widget")
	assert.Contains(t, valuationBuf.String(), "MAIN Main warehouse")
	assert.Contains(t, valuationBuf.String(), "126")
}

func TestPrintCostCenterOutputs(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC)
	budget := decimal.NewFromInt(1000)
	spent := decimal.NewFromInt(250)
	used := decimal.NewFromInt(25)
	costCenter := accounting.CostCenter{
		ID:           "cc-1",
		TenantID:     "tenant-1",
		Code:         "CC001",
		Name:         "Sales",
		Description:  "Sales team",
		IsActive:     true,
		BudgetAmount: &budget,
		BudgetPeriod: accounting.BudgetPeriodMonthly,
		CreatedAt:    now,
		UpdatedAt:    now,
		TotalSpent:   &spent,
		BudgetUsed:   &used,
	}
	report := accounting.CostCenterReport{
		TenantID:      "tenant-1",
		PeriodStart:   now,
		PeriodEnd:     now.AddDate(0, 1, -1),
		GeneratedAt:   now,
		TotalExpenses: spent,
		TotalBudget:   budget,
		CostCenters: []accounting.CostCenterSummary{{
			CostCenter:    costCenter,
			TotalExpenses: spent,
			BudgetAmount:  budget,
			BudgetUsed:    used,
			IsOverBudget:  false,
			PeriodStart:   now,
			PeriodEnd:     now.AddDate(0, 1, -1),
		}},
	}

	var tableBuf bytes.Buffer
	printCostCentersTable(&tableBuf, []accounting.CostCenter{costCenter})
	assert.Contains(t, tableBuf.String(), "CC001")
	assert.Contains(t, tableBuf.String(), "1000")

	var detailBuf bytes.Buffer
	printCostCenter(&detailBuf, &costCenter)
	assert.Contains(t, detailBuf.String(), "Cost center CC001 Sales")
	assert.Contains(t, detailBuf.String(), "Budget used: 25%")

	var reportBuf bytes.Buffer
	printCostCenterReport(&reportBuf, &report)
	assert.Contains(t, reportBuf.String(), "Total expenses: 250")
	assert.Contains(t, reportBuf.String(), "Sales")
}

func TestPrintPayrollOutputs(t *testing.T) {
	t.Parallel()

	paymentDate := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)
	now := time.Date(2026, 3, 20, 12, 0, 0, 0, time.UTC)
	payslip := payroll.Payslip{
		ID:                "payslip-1",
		EmployeeID:        "emp-1",
		GrossSalary:       decimal.NewFromInt(3200),
		NetSalary:         decimal.NewFromFloat(2534.8),
		IncomeTax:         decimal.NewFromInt(550),
		SocialTax:         decimal.NewFromInt(1056),
		TotalEmployerCost: decimal.NewFromFloat(4281.6),
		PaymentStatus:     "PENDING",
		CreatedAt:         now,
		Employee:          &payroll.Employee{FirstName: "Mari", LastName: "Maasikas"},
	}
	run := payroll.PayrollRun{
		ID:                "run-1",
		PeriodYear:        2026,
		PeriodMonth:       3,
		Status:            payroll.PayrollCalculated,
		PaymentDate:       &paymentDate,
		TotalGross:        decimal.NewFromInt(3200),
		TotalNet:          decimal.NewFromFloat(2534.8),
		TotalEmployerCost: decimal.NewFromFloat(4281.6),
		Notes:             "March payroll",
		CreatedAt:         now,
		UpdatedAt:         now,
		Payslips:          []payroll.Payslip{payslip},
	}

	var runsBuf bytes.Buffer
	printPayrollRunsTable(&runsBuf, []payroll.PayrollRun{run})
	assert.Contains(t, runsBuf.String(), "2026-03")
	assert.Contains(t, runsBuf.String(), "CALCULATED")

	var runBuf bytes.Buffer
	printPayrollRun(&runBuf, &run)
	assert.Contains(t, runBuf.String(), "Payroll run 2026-03")
	assert.Contains(t, runBuf.String(), "Mari Maasikas")

	var payslipsBuf bytes.Buffer
	printPayslipsTable(&payslipsBuf, []payroll.Payslip{payslip})
	assert.Contains(t, payslipsBuf.String(), "Mari Maasikas")
	assert.Contains(t, payslipsBuf.String(), "2534.8")

	var componentsBuf bytes.Buffer
	printSalaryComponentsTable(&componentsBuf, []payroll.SalaryComponent{
		{
			ID:            "comp-1",
			ComponentType: payroll.SalaryComponentSecondaryEmployment,
			Name:          "Evening contract",
			Amount:        decimal.NewFromInt(600),
			IsTaxable:     true,
			IsRecurring:   true,
			EffectiveFrom: paymentDate,
		},
	})
	assert.Contains(t, componentsBuf.String(), "SECONDARY_EMPLOYMENT")
	assert.Contains(t, componentsBuf.String(), "Evening contract")

	var taxBuf bytes.Buffer
	printTaxCalculation(&taxBuf, &payroll.TaxCalculation{
		GrossSalary:       decimal.NewFromInt(3200),
		BasicExemption:    decimal.NewFromInt(700),
		TaxableIncome:     decimal.NewFromInt(2500),
		IncomeTax:         decimal.NewFromInt(550),
		UnemploymentEE:    decimal.NewFromFloat(51.2),
		FundedPension:     decimal.NewFromInt(64),
		TotalDeductions:   decimal.NewFromFloat(665.2),
		NetSalary:         decimal.NewFromFloat(2534.8),
		SocialTax:         decimal.NewFromInt(1056),
		UnemploymentER:    decimal.NewFromFloat(25.6),
		TotalEmployerCost: decimal.NewFromFloat(4281.6),
	})
	assert.Contains(t, taxBuf.String(), "Net salary: 2534.8")
	assert.Contains(t, taxBuf.String(), "Total employer cost: 4281.6")

	assert.Equal(t, "emp-2", payslipEmployeeName(payroll.Payslip{EmployeeID: "emp-2"}))
	assert.Equal(t, "2026-03-31", formatDatePtr(&paymentDate))
	assert.Equal(t, "-", formatDatePtr(nil))
}

func TestPrintLeaveOutputs(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC)
	absenceType := payroll.AbsenceType{
		ID:                 "type-1",
		TenantID:           "tenant-1",
		Code:               "ANNUAL_LEAVE",
		Name:               "Annual leave",
		NameET:             "Pohipuhkus",
		Description:        "Paid annual leave",
		IsPaid:             true,
		AffectsSalary:      false,
		RequiresDocument:   false,
		DefaultDaysPerYear: decimal.NewFromInt(28),
		MaxCarryoverDays:   decimal.NewFromInt(5),
		IsActive:           true,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	employee := payroll.Employee{
		ID:        "emp-1",
		FirstName: "Mari",
		LastName:  "Maasikas",
	}
	balance := payroll.LeaveBalance{
		ID:            "balance-1",
		TenantID:      "tenant-1",
		EmployeeID:    "emp-1",
		AbsenceTypeID: "type-1",
		Year:          2026,
		EntitledDays:  decimal.NewFromInt(28),
		CarryoverDays: decimal.NewFromInt(2),
		UsedDays:      decimal.NewFromInt(5),
		PendingDays:   decimal.NewFromInt(1),
		RemainingDays: decimal.NewFromInt(24),
		AbsenceType:   &absenceType,
	}
	record := payroll.LeaveRecord{
		ID:             "leave-1",
		TenantID:       "tenant-1",
		EmployeeID:     "emp-1",
		AbsenceTypeID:  "type-1",
		StartDate:      now,
		EndDate:        now.AddDate(0, 0, 4),
		TotalDays:      decimal.NewFromInt(5),
		WorkingDays:    decimal.NewFromInt(3),
		Status:         payroll.LeavePending,
		DocumentNumber: "DOC-1",
		DocumentDate:   &now,
		Notes:          "Spring break",
		AbsenceType:    &absenceType,
		Employee:       &employee,
	}

	var typesBuf bytes.Buffer
	printAbsenceTypesTable(&typesBuf, []payroll.AbsenceType{absenceType})
	assert.Contains(t, typesBuf.String(), "ANNUAL_LEAVE")
	assert.Contains(t, typesBuf.String(), "28")

	var typeBuf bytes.Buffer
	printAbsenceType(&typeBuf, &absenceType)
	assert.Contains(t, typeBuf.String(), "Absence type ANNUAL_LEAVE Annual leave")
	assert.Contains(t, typeBuf.String(), "Paid annual leave")

	var balancesBuf bytes.Buffer
	printLeaveBalancesTable(&balancesBuf, []payroll.LeaveBalance{balance})
	assert.Contains(t, balancesBuf.String(), "ANNUAL_LEAVE")
	assert.Contains(t, balancesBuf.String(), "24")

	var recordsBuf bytes.Buffer
	printLeaveRecordsTable(&recordsBuf, []payroll.LeaveRecord{record})
	assert.Contains(t, recordsBuf.String(), "Mari Maasikas")
	assert.Contains(t, recordsBuf.String(), "PENDING")

	var recordBuf bytes.Buffer
	printLeaveRecord(&recordBuf, &record)
	assert.Contains(t, recordBuf.String(), "Leave record leave-1")
	assert.Contains(t, recordBuf.String(), "Document number: DOC-1")
	assert.Contains(t, recordBuf.String(), "Spring break")
}

func TestPrintReports(t *testing.T) {
	t.Parallel()

	asOf := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)
	balances := []accounting.AccountBalance{{
		AccountID:     "account-1",
		AccountCode:   "1000",
		AccountName:   "Cash",
		AccountType:   accounting.AccountTypeAsset,
		DebitBalance:  decimal.NewFromInt(500),
		CreditBalance: decimal.Zero,
		NetBalance:    decimal.NewFromInt(500),
	}}

	var trialBuf bytes.Buffer
	printTrialBalance(&trialBuf, &accounting.TrialBalance{
		AsOfDate:     asOf,
		Accounts:     balances,
		TotalDebits:  decimal.NewFromInt(500),
		TotalCredits: decimal.NewFromInt(500),
		IsBalanced:   true,
	})
	assert.Contains(t, trialBuf.String(), "Trial balance as of 2026-03-31")
	assert.Contains(t, trialBuf.String(), "1000")

	var accountBalanceBuf bytes.Buffer
	printAccountBalance(&accountBalanceBuf, &accountBalanceReport{
		AccountID: "account-1",
		AsOfDate:  "2026-03-31",
		Balance:   "500.00",
	})
	assert.Contains(t, accountBalanceBuf.String(), "ACCOUNT ID")
	assert.Contains(t, accountBalanceBuf.String(), "500.00")

	var balanceSheetBuf bytes.Buffer
	printBalanceSheet(&balanceSheetBuf, &accounting.BalanceSheet{
		AsOfDate:         asOf,
		Assets:           balances,
		TotalAssets:      decimal.NewFromInt(500),
		TotalLiabilities: decimal.NewFromInt(200),
		TotalEquity:      decimal.NewFromInt(300),
		IsBalanced:       true,
	})
	assert.Contains(t, balanceSheetBuf.String(), "Balance sheet as of 2026-03-31")
	assert.Contains(t, balanceSheetBuf.String(), "Total assets: 500")

	var incomeBuf bytes.Buffer
	printIncomeStatement(&incomeBuf, &accounting.IncomeStatement{
		StartDate:     asOf,
		EndDate:       asOf,
		Revenue:       balances,
		TotalRevenue:  decimal.NewFromInt(1200),
		TotalExpenses: decimal.NewFromInt(700),
		NetIncome:     decimal.NewFromInt(500),
	})
	assert.Contains(t, incomeBuf.String(), "Income statement")
	assert.Contains(t, incomeBuf.String(), "Net income: 500")

	var cashFlowBuf bytes.Buffer
	printCashFlowStatement(&cashFlowBuf, &reports.CashFlowStatement{
		StartDate: "2026-01-01",
		EndDate:   "2026-03-31",
		OperatingActivities: []reports.CashFlowItem{{
			Code:        reports.CFOperTotal,
			Description: "Operating total",
			Amount:      decimal.NewFromInt(500),
		}},
		ClosingCash: decimal.NewFromInt(500),
	})
	assert.Contains(t, cashFlowBuf.String(), "Cash flow 2026-01-01 to 2026-03-31")
	assert.Contains(t, cashFlowBuf.String(), "Closing cash: 500")

	var cashFlowMappingBuf bytes.Buffer
	printCashFlowMapping(&cashFlowMappingBuf, &reports.CashFlowMappingOverrides{
		OperatingAccountCodes: []string{"PREPAY"},
		InvestingAccountCodes: []string{"CAPEX-1", "CAPEX-2"},
	})
	assert.Contains(t, cashFlowMappingBuf.String(), "Cash flow mapping")
	assert.Contains(t, cashFlowMappingBuf.String(), "Operating accounts: PREPAY")
	assert.Contains(t, cashFlowMappingBuf.String(), "Investing accounts: CAPEX-1, CAPEX-2")
	assert.Contains(t, cashFlowMappingBuf.String(), "Financing accounts: -")

	var agingBuf bytes.Buffer
	printAgingReport(&agingBuf, &analytics.AgingReport{
		ReportType: "receivables",
		AsOfDate:   asOf,
		Total:      decimal.NewFromInt(900),
		Buckets: []analytics.AgingBucket{{
			Label:  "Current",
			Amount: decimal.NewFromInt(900),
			Count:  2,
		}},
		ByContact: []analytics.ContactAging{{
			ContactName: "Acme",
			Total:       decimal.NewFromInt(900),
		}},
	})
	assert.Contains(t, agingBuf.String(), "Receivables aging")
	assert.Contains(t, agingBuf.String(), "Acme")

	var dashboardBuf bytes.Buffer
	printDashboardSummary(&dashboardBuf, &analytics.DashboardSummary{
		TotalRevenue:       decimal.NewFromInt(1200),
		TotalExpenses:      decimal.NewFromInt(700),
		NetIncome:          decimal.NewFromInt(500),
		RevenueChange:      decimal.NewFromInt(10),
		ExpensesChange:     decimal.NewFromInt(5),
		TotalReceivables:   decimal.NewFromInt(900),
		OverdueReceivables: decimal.NewFromInt(100),
		TotalPayables:      decimal.NewFromInt(300),
		OverduePayables:    decimal.NewFromInt(50),
		DraftInvoices:      1,
		PendingInvoices:    2,
		OverdueInvoices:    3,
		PeriodStart:        asOf,
		PeriodEnd:          asOf,
	})
	assert.Contains(t, dashboardBuf.String(), "Dashboard")
	assert.Contains(t, dashboardBuf.String(), "Net income: 500")

	var revenueChartBuf bytes.Buffer
	printRevenueExpenseChart(&revenueChartBuf, &analytics.RevenueExpenseChart{
		Labels:   []string{"2026-03"},
		Revenue:  []decimal.Decimal{decimal.NewFromInt(1200)},
		Expenses: []decimal.Decimal{decimal.NewFromInt(700)},
		Profit:   []decimal.Decimal{decimal.NewFromInt(500)},
	})
	assert.Contains(t, revenueChartBuf.String(), "2026-03")
	assert.Contains(t, revenueChartBuf.String(), "500")

	var cashFlowChartBuf bytes.Buffer
	printCashFlowChart(&cashFlowChartBuf, &analytics.CashFlowChart{
		Labels:   []string{"2026-03"},
		Inflows:  []decimal.Decimal{decimal.NewFromInt(1500)},
		Outflows: []decimal.Decimal{decimal.NewFromInt(800)},
		Net:      []decimal.Decimal{decimal.NewFromInt(700)},
	})
	assert.Contains(t, cashFlowChartBuf.String(), "INFLOWS")
	assert.Contains(t, cashFlowChartBuf.String(), "700")

	amount := decimal.NewFromInt(219)
	var activityBuf bytes.Buffer
	printActivityItems(&activityBuf, []analytics.ActivityItem{{
		ID:          "act-1",
		Type:        "INVOICE",
		Action:      "created",
		Description: "Invoice INV-1",
		CreatedAt:   asOf,
		Amount:      &amount,
	}})
	assert.Contains(t, activityBuf.String(), "Invoice INV-1")
	assert.Contains(t, activityBuf.String(), "219")

	var confirmationSummaryBuf bytes.Buffer
	printBalanceConfirmationSummary(&confirmationSummaryBuf, &reports.BalanceConfirmationSummary{
		Type:         reports.BalanceTypeReceivable,
		AsOfDate:     "2026-03-31",
		TotalBalance: decimal.NewFromInt(900),
		ContactCount: 1,
		InvoiceCount: 2,
		Contacts: []reports.ContactBalance{{
			ContactName:  "Acme",
			ContactCode:  "CUST-1",
			ContactEmail: "billing@example.com",
			Balance:      decimal.NewFromInt(900),
			InvoiceCount: 2,
		}},
	})
	assert.Contains(t, confirmationSummaryBuf.String(), "RECEIVABLE balance confirmations")
	assert.Contains(t, confirmationSummaryBuf.String(), "Total balance: 900")

	var confirmationBuf bytes.Buffer
	printBalanceConfirmation(&confirmationBuf, &reports.BalanceConfirmation{
		Type:         reports.BalanceTypeReceivable,
		ContactName:  "Acme",
		AsOfDate:     "2026-03-31",
		TotalBalance: decimal.NewFromInt(900),
		Invoices: []reports.BalanceInvoice{{
			InvoiceNumber:     "INV-1",
			InvoiceDate:       "2026-03-01",
			DueDate:           "2026-03-15",
			TotalAmount:       decimal.NewFromInt(1000),
			AmountPaid:        decimal.NewFromInt(100),
			OutstandingAmount: decimal.NewFromInt(900),
			DaysOverdue:       16,
		}},
	})
	assert.Contains(t, confirmationBuf.String(), "INV-1")
	assert.Contains(t, confirmationBuf.String(), "Total balance: 900")
}

func TestPrintJournalEntries(t *testing.T) {
	t.Parallel()

	entryDate := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)
	entry := accounting.JournalEntry{
		ID:               "je-1",
		EntryNumber:      "JE-2026-001",
		EntryDate:        entryDate,
		Description:      "Manual accrual",
		Reference:        "ACC-1",
		RequiresEvidence: true,
		Status:           accounting.StatusDraft,
		Lines: []accounting.JournalEntryLine{
			{
				AccountID:   "acc-1",
				Description: "Expense",
				DebitAmount: decimal.NewFromInt(100),
				Currency:    "EUR",
				BaseDebit:   decimal.NewFromInt(100),
				Account:     &accounting.Account{Code: "6000", Name: "Expenses"},
			},
			{
				AccountID:    "acc-2",
				Description:  "Accrual",
				CreditAmount: decimal.NewFromInt(100),
				Currency:     "EUR",
				BaseCredit:   decimal.NewFromInt(100),
			},
		},
	}

	var entriesBuf bytes.Buffer
	printJournalEntriesTable(&entriesBuf, []accounting.JournalEntry{entry})
	assert.Contains(t, entriesBuf.String(), "JE-2026-001")
	assert.Contains(t, entriesBuf.String(), "true")
	assert.Contains(t, entriesBuf.String(), "Manual accrual")

	var entryBuf bytes.Buffer
	printJournalEntry(&entryBuf, &entry)
	assert.Contains(t, entryBuf.String(), "Requires evidence: true")
	assert.Contains(t, entryBuf.String(), "Balanced: true")
	assert.Contains(t, entryBuf.String(), "6000 Expenses")
}

func TestPrintTaxReports(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC)

	tsd := payroll.TSDDeclaration{
		ID:                  "tsd-1",
		PeriodYear:          2026,
		PeriodMonth:         3,
		TotalPayments:       decimal.NewFromInt(3200),
		TotalIncomeTax:      decimal.NewFromInt(500),
		TotalSocialTax:      decimal.NewFromInt(1056),
		TotalUnemploymentER: decimal.NewFromFloat(25.6),
		TotalUnemploymentEE: decimal.NewFromFloat(51.2),
		TotalFundedPension:  decimal.NewFromInt(64),
		Status:              payroll.TSDDraft,
		CreatedAt:           now,
		UpdatedAt:           now,
		Rows: []payroll.TSDRow{{
			FirstName:     "Mari",
			LastName:      "Maasikas",
			PaymentType:   "10",
			GrossPayment:  decimal.NewFromInt(3200),
			TaxableAmount: decimal.NewFromInt(2500),
			IncomeTax:     decimal.NewFromInt(500),
			SocialTax:     decimal.NewFromInt(1056),
		}},
	}

	var tsdListBuf bytes.Buffer
	printTSDDeclarationsTable(&tsdListBuf, []payroll.TSDDeclaration{tsd})
	assert.Contains(t, tsdListBuf.String(), "2026-03")
	assert.Contains(t, tsdListBuf.String(), "3200")

	var tsdBuf bytes.Buffer
	printTSDDeclaration(&tsdBuf, &tsd)
	assert.Contains(t, tsdBuf.String(), "TSD 2026-03")
	assert.Contains(t, tsdBuf.String(), "Mari Maasikas")

	kmd := tax.KMDDeclaration{
		ID:             "kmd-1",
		Year:           2026,
		Month:          3,
		Status:         "DRAFT",
		TotalOutputVAT: decimal.NewFromInt(220),
		TotalInputVAT:  decimal.NewFromInt(80),
		Rows: []tax.KMDRow{{
			Code:        tax.KMDRow1,
			Description: "Taxable sales",
			TaxBase:     decimal.NewFromInt(1000),
			TaxAmount:   decimal.NewFromInt(220),
		}},
		CreatedAt: now,
		UpdatedAt: now,
	}

	var kmdListBuf bytes.Buffer
	printKMDDeclarationsTable(&kmdListBuf, []tax.KMDDeclaration{kmd})
	assert.Contains(t, kmdListBuf.String(), "2026-03")
	assert.Contains(t, kmdListBuf.String(), "140")

	var kmdBuf bytes.Buffer
	printKMDDeclaration(&kmdBuf, &kmd)
	assert.Contains(t, kmdBuf.String(), "KMD 2026-03")
	assert.Contains(t, kmdBuf.String(), "Taxable sales")

	var infBuf bytes.Buffer
	printKMDINFReport(&infBuf, &tax.KMDINFReport{
		TenantID:    "tenant-1",
		Year:        2026,
		Month:       3,
		Threshold:   decimal.NewFromInt(1000),
		GeneratedAt: now,
		Summary: []tax.KMDINFPartSummary{{
			Part:          tax.KMDINFPartSales,
			PartnerCount:  1,
			InvoiceCount:  1,
			TaxableAmount: decimal.NewFromInt(1200),
			VATAmount:     decimal.NewFromInt(264),
			TotalAmount:   decimal.NewFromInt(1464),
		}},
		Rows: []tax.KMDINFReportRow{{
			Part:                       tax.KMDINFPartSales,
			ContactName:                "Alpha OU",
			ContactRegCode:             "12345678",
			InvoiceNumber:              "INV-1",
			InvoiceDate:                now,
			TaxableAmount:              decimal.NewFromInt(1200),
			VATAmount:                  decimal.NewFromInt(264),
			TotalAmount:                decimal.NewFromInt(1464),
			PartnerPeriodTaxableAmount: decimal.NewFromInt(1200),
		}},
	})
	assert.Contains(t, infBuf.String(), "KMD INF 2026-03")
	assert.Contains(t, infBuf.String(), "A sales")
	assert.Contains(t, infBuf.String(), "Alpha OU")
}

func TestFormatHelpers(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "-", formatTimePtr(nil))

	now := time.Date(2026, 3, 12, 10, 0, 0, 0, time.UTC)
	assert.Equal(t, now.Format(time.RFC3339), formatTimePtr(&now))
	assert.Equal(t, "2026-03-12", formatDate(now))
	assert.Equal(t, "-", formatDate(time.Time{}))
	assert.Equal(t, "Receivables", titleLabel("receivables"))

	assert.Equal(t, "oa_token_12345...", tokenPreview("oa_token_1234567890"))
	assert.Equal(t, "short-token", tokenPreview("short-token"))
	assert.Equal(t, "tenant-slug", normalizeSelector("  Tenant-Slug "))
}
