package main

import (
	"github.com/go-chi/chi/v5"

	"github.com/HMB-research/open-accounting/internal/tenant"
)

func registerTenantRoutes(
	r chi.Router,
	h *Handlers,
	canCreateEntries func(tenant.RolePermissions) bool,
	canManageSettings func(tenant.RolePermissions) bool,
) {
	r.Route("/tenants/{tenantID}", func(r chi.Router) {
		r.Use(h.TenantContext)
		r.Use(h.RequireTenantWritePermission(canCreateEntries))

		// Onboarding
		r.Post("/complete-onboarding", h.CompleteOnboarding)
		r.Get("/period-close-events", h.ListPeriodCloseEvents)
		r.Post("/period-close", h.ClosePeriod)
		r.Post("/period-reopen", h.ReopenPeriod)
		r.Get("/year-end-close-status", h.GetYearEndCloseStatus)
		r.Get("/year-end-close-pack", h.GetYearEndClosePack)
		r.Get("/year-end-close-audit-evidence", h.GetYearEndCloseAuditEvidence)
		r.Get("/year-end-close-audit-archive", h.DownloadYearEndCloseAuditArchive)
		r.Post("/year-end-carry-forward", h.CreateYearEndCarryForward)
		r.Post("/year-end-carry-forward/reverse", h.ReverseYearEndCarryForward)
		r.Get("/documents", h.ListDocuments)
		r.Post("/documents/review-summary", h.ListDocumentReviewSummaries)
		r.Get("/documents/review-queue", h.GetDocumentReviewQueue)
		r.Post("/documents/evidence-policy", h.EvaluateDocumentEvidencePolicy)
		r.Get("/documents/retention", h.GetDocumentRetentionReview)
		r.Post("/documents/purge", h.PurgeExpiredDocuments)
		r.Post("/documents", h.UploadDocument)
		r.Get("/documents/{documentID}/download", h.DownloadDocument)
		r.Patch("/documents/{documentID}/retention", h.UpdateDocumentRetention)
		r.Patch("/documents/{documentID}/lifecycle", h.UpdateDocumentLifecycle)
		r.Patch("/documents/{documentID}/legal-hold", h.UpdateDocumentLegalHold)
		r.Post("/documents/{documentID}/review", h.ReviewDocument)
		r.Post("/documents/{documentID}/mark-reviewed", h.MarkDocumentReviewed)
		r.Delete("/documents/{documentID}", h.DeleteDocument)
		r.Get("/api-tokens", h.ListAPITokens)
		r.Post("/api-tokens", h.CreateAPIToken)
		r.Delete("/api-tokens/{tokenID}", h.RevokeAPIToken)

		// Migration cutover preflight
		r.Get("/migration/provider-presets", h.ListMigrationProviderPresets)
		r.Post("/migration/validate", h.ValidateMigrationBundle)
		r.Post("/migration/execution-plan", h.PlanMigrationExecution)
		r.With(h.RequireTenantPermission(canCreateEntries)).Post("/migration/execute", h.ExecuteMigration)
		r.Get("/migration/execution-runs", h.ListMigrationExecutionRuns)
		r.Get("/migration/execution-runs/{runID}", h.GetMigrationExecutionRun)
		r.Get("/migration/execution-runs/{runID}/events", h.StreamMigrationExecutionRun)

		// External import sessions are receive-only in v1. They validate and
		// persist package receipts but never create accounting transactions.
		r.Post("/import-sessions/validate", h.ValidateImportSessionPackage)
		r.Post("/import-sessions", h.CreateImportSession)
		r.Get("/import-sessions/{sessionID}", h.GetImportSession)
		r.Post("/import-sessions/{sessionID}/plan", h.PlanImportSession)

		// SmartAccounts bridge control is isolated from the legacy migration
		// executor. The control endpoint proxies transient credentials only to
		// the private bridge and persists its opaque reference; it makes no
		// financial write in Open Accounting.
		r.Get("/smartaccounts-sync/sources", h.DiscoverSmartAccountsSyncSources)
		r.Get("/smartaccounts-sync/status", h.GetSmartAccountsSyncStatus)
		r.With(h.RequireTenantPermission(canManageSettings)).Post("/smartaccounts-sync/browser-pairings", h.IssueSmartAccountsBrowserPairing)
		r.With(h.RequireTenantPermission(canManageSettings)).Get("/smartaccounts-sync/browser-pairings/{pairingID}", h.GetSmartAccountsBrowserPairing)
		r.With(h.RequireTenantPermission(canManageSettings)).Post("/smartaccounts-sync/browser-discoveries", h.IssueSmartAccountsBrowserDiscovery)
		r.With(h.RequireTenantPermission(canManageSettings)).Post("/smartaccounts-sync/browser-discoveries/{discoveryID}/receipt", h.ReceiveSmartAccountsBrowserDiscoveryReceipt)
		r.With(h.RequireTenantPermission(canManageSettings)).Get("/smartaccounts-sync/browser-discoveries/{discoveryID}", h.GetSmartAccountsBrowserDiscoveryReceipt)
		r.With(h.RequireTenantPermission(canManageSettings)).Post("/smartaccounts-sync/browser-discoveries/{discoveryID}/resources/{resourceID}/schemas/{schemaID}/review", h.ReviewSmartAccountsBrowserCSVSchema)
		r.With(h.RequireTenantPermission(canManageSettings)).Get("/smartaccounts-sync/browser-discoveries/{discoveryID}/resources/{resourceID}/schemas/{schemaID}/review", h.GetSmartAccountsBrowserCSVSchemaReview)
		r.With(h.RequireTenantPermission(canManageSettings)).Post("/smartaccounts-sync/browser-captures", h.IssueSmartAccountsBrowserCapture)
		r.With(h.RequireTenantPermission(canManageSettings)).Get("/smartaccounts-sync/browser-captures/{runID}", h.GetSmartAccountsBrowserCaptureOwnerStatus)
		r.With(h.RequireTenantPermission(canManageSettings)).Post("/smartaccounts-sync/browser-captures/{runID}/resume", h.ResumeSmartAccountsBrowserCapture)
		// Master detail is a separate reviewed, current-snapshot relay. It has
		// no financial apply surface and never shares CSV/GL capture routes.
		r.With(h.RequireTenantPermission(canManageSettings)).Post("/smartaccounts-sync/browser-master-details", h.IssueSmartAccountsBrowserMasterDetails)
		r.With(h.RequireTenantPermission(canManageSettings)).Get("/smartaccounts-sync/browser-master-details/{runID}", h.GetSmartAccountsBrowserMasterDetailOwnerStatus)
		r.With(h.RequireTenantPermission(canManageSettings)).Post("/smartaccounts-sync/browser-master-details/{runID}/resume", h.ResumeSmartAccountsBrowserMasterDetail)
		// Commercial detail is review/archive-only. Its relay currently stops at
		// a reviewed visible-selector blocker and has no apply or preview route.
		r.With(h.RequireTenantPermission(canManageSettings)).Post("/smartaccounts-sync/browser-commercial-details", h.IssueSmartAccountsBrowserCommercialDetails)
		r.With(h.RequireTenantPermission(canManageSettings)).Get("/smartaccounts-sync/browser-commercial-details/{runID}", h.GetSmartAccountsBrowserCommercialDetailOwnerStatus)
		r.With(h.RequireTenantPermission(canManageSettings)).Post("/smartaccounts-sync/browser-commercial-details/{runID}/resume", h.ResumeSmartAccountsBrowserCommercialDetail)
		r.With(h.RequireTenantPermission(canManageSettings)).Post("/smartaccounts-sync/browser-capture-workflows", h.StartSmartAccountsBrowserCaptureWorkflow)
		r.With(h.RequireTenantPermission(canManageSettings)).Get("/smartaccounts-sync/browser-capture-workflows/{workflowID}", h.GetSmartAccountsBrowserCaptureWorkflowStatus)
		r.With(h.RequireTenantPermission(canManageSettings)).Post("/smartaccounts-sync/control", h.ConfigureSmartAccountsSync)
		r.With(h.RequireTenantPermission(canManageSettings)).Post("/smartaccounts-sync/dry-run", h.RequestSmartAccountsSyncDryRun)
		r.With(h.RequireTenantPermission(canManageSettings)).Post("/smartaccounts-sync/apply", h.ConfirmSmartAccountsFinancialApply)
		// Financial execution is deliberately separate from bridge capture and
		// requires a stored plan plus an exact explicit confirmation.
		r.With(h.RequireTenantPermission(canManageSettings)).Post("/smartaccounts-sync/packages/{packageID}/preview", h.PreviewSmartAccountsPackage)
		r.With(h.RequireTenantPermission(canManageSettings)).Post("/smartaccounts-sync/packages/apply", h.ApplySmartAccountsPackage)
		// Reference masters use a separate confirmed-only, non-financial path.
		r.With(h.RequireTenantPermission(canManageSettings)).Get("/smartaccounts-sync/packages/{packageID}/archive-coverage", h.GetSmartAccountsPackageArchiveCoverage)
		r.With(h.RequireTenantPermission(canManageSettings)).Post("/smartaccounts-sync/packages/{packageID}/reference-preview", h.PreviewSmartAccountsReferenceMasters)
		r.With(h.RequireTenantPermission(canManageSettings)).Post("/smartaccounts-sync/reference-masters/apply", h.ApplySmartAccountsReferenceMasters)
		// Reconciliation policy and approval paths are deliberately stricter than
		// normal GL confirmation: an active accountant must approve policy and
		// independently attest the resulting digest-bound evidence.
		r.Post("/smartaccounts-sync/sources/{sourceCompanyID}/tolerance-policy-candidates", h.GetSmartAccountsTolerancePolicyCandidate)
		r.Post("/smartaccounts-sync/sources/{sourceCompanyID}/tolerance-policies", h.ApproveSmartAccountsTolerancePolicy)
		r.Post("/smartaccounts-sync/sources/{sourceCompanyID}/tolerance-policy-resolutions", h.ResolveSmartAccountsTolerancePolicy)
		r.Get("/smartaccounts-sync/reconciliation/batches/{batchID}/sources/{sourceCompanyID}", h.GetSmartAccountsTenantReconciliation)
		r.Post("/smartaccounts-sync/reconciliation/evaluations/{evaluationID}/approval", h.ApproveSmartAccountsReconciliation)

		// Accounts
		r.Get("/accounts", h.ListAccounts)
		r.Post("/accounts", h.CreateAccount)
		r.Post("/accounts/import", h.ImportAccounts)
		r.Get("/accounts/hierarchy", h.GetAccountHierarchy)
		r.Get("/accounts/{accountID}", h.GetAccount)
		r.Put("/accounts/{accountID}", h.UpdateAccount)
		r.Delete("/accounts/{accountID}", h.DeleteAccount)

		// Journal entries
		r.Post("/journal-entries/import-opening-balances", h.ImportOpeningBalances)
		r.Post("/journal-entries/import", h.ImportJournalEntries)
		r.Get("/journal-entries", h.ListJournalEntries)
		r.Get("/journal-entries/{entryID}", h.GetJournalEntry)
		r.Post("/journal-entries", h.CreateJournalEntry)
		r.Post("/journal-entries/{entryID}/post", h.PostJournalEntry)
		r.Post("/journal-entries/{entryID}/void", h.VoidJournalEntry)
		r.Get("/journal-entry-templates", h.ListJournalEntryTemplates)
		r.Post("/journal-entry-templates", h.CreateJournalEntryTemplate)
		r.Post("/journal-entry-templates/generate-due", h.GenerateDueJournalEntryTemplates)
		r.Get("/journal-entry-templates/{templateID}", h.GetJournalEntryTemplate)
		r.Post("/journal-entry-templates/{templateID}/generate", h.GenerateJournalEntryTemplate)
		r.Post("/journal-entry-templates/{templateID}/apply", h.ApplyJournalEntryTemplate)

		// Contacts
		r.Get("/contacts", h.ListContacts)
		r.Post("/contacts", h.CreateContact)
		r.Post("/contacts/import", h.ImportContacts)
		r.Get("/contacts/{contactID}", h.GetContact)
		r.Put("/contacts/{contactID}", h.UpdateContact)
		r.Delete("/contacts/{contactID}", h.DeleteContact)

		// Invoices
		r.Get("/invoices", h.ListInvoices)
		r.Post("/invoices", h.CreateInvoice)
		r.Post("/invoices/import", h.ImportInvoices)
		r.Post("/invoices/import-einvoice", h.ImportEInvoice)
		r.Get("/invoices/{invoiceID}", h.GetInvoice)
		r.Get("/invoices/{invoiceID}/pdf", h.GetInvoicePDF)
		r.Post("/invoices/{invoiceID}/send", h.SendInvoice)
		r.Post("/invoices/{invoiceID}/void", h.VoidInvoice)
		r.Get("/invoices/{invoiceID}/reminders", h.GetInvoiceReminderHistory)

		// Payment Reminders
		r.Get("/invoices/overdue", h.GetOverdueInvoices)
		r.Post("/invoices/reminders", h.SendPaymentReminder)
		r.Post("/invoices/reminders/bulk", h.SendBulkPaymentReminders)

		// Quotes
		r.Get("/quotes", h.ListQuotes)
		r.Post("/quotes", h.CreateQuote)
		r.Post("/quotes/import", h.ImportQuotes)
		r.Get("/quotes/{quoteID}", h.GetQuote)
		r.Get("/quotes/{quoteID}/pdf", h.GetQuotePDF)
		r.Put("/quotes/{quoteID}", h.UpdateQuote)
		r.Delete("/quotes/{quoteID}", h.DeleteQuote)
		r.Post("/quotes/{quoteID}/email", h.EmailQuote)
		r.Post("/quotes/{quoteID}/send", h.SendQuote)
		r.Post("/quotes/{quoteID}/accept", h.AcceptQuote)
		r.Post("/quotes/{quoteID}/reject", h.RejectQuote)
		r.Post("/quotes/{quoteID}/convert-to-invoice", h.ConvertQuoteToInvoice)

		// Orders
		r.Get("/orders", h.ListOrders)
		r.Post("/orders", h.CreateOrder)
		r.Post("/orders/import", h.ImportOrders)
		r.Get("/orders/{orderID}", h.GetOrder)
		r.Get("/orders/{orderID}/pdf", h.GetOrderPDF)
		r.Put("/orders/{orderID}", h.UpdateOrder)
		r.Delete("/orders/{orderID}", h.DeleteOrder)
		r.Post("/orders/{orderID}/email", h.EmailOrder)
		r.Get("/orders/{orderID}/stock-check", h.CheckOrderStock)
		r.Get("/orders/{orderID}/stock-reservations", h.ListOrderStockReservations)
		r.Get("/orders/{orderID}/pick-list", h.GetOrderPickList)
		r.Post("/orders/{orderID}/reserve-stock", h.ReserveOrderStock)
		r.Post("/orders/{orderID}/release-stock", h.ReleaseOrderStock)
		r.Post("/orders/{orderID}/confirm", h.ConfirmOrder)
		r.Post("/orders/{orderID}/process", h.ProcessOrder)
		r.Post("/orders/{orderID}/ship", h.ShipOrder)
		r.Post("/orders/{orderID}/deliver", h.DeliverOrder)
		r.Post("/orders/{orderID}/cancel", h.CancelOrder)
		r.Post("/orders/{orderID}/convert-to-invoice", h.ConvertOrderToInvoice)

		// Fixed Assets
		r.Get("/asset-categories", h.ListAssetCategories)
		r.Post("/asset-categories", h.CreateAssetCategory)
		r.Get("/asset-categories/{categoryID}", h.GetAssetCategory)
		r.Delete("/asset-categories/{categoryID}", h.DeleteAssetCategory)
		r.Get("/assets", h.ListAssets)
		r.Post("/assets", h.CreateAsset)
		r.Post("/assets/import", h.ImportAssets)
		r.Get("/assets/{assetID}", h.GetAsset)
		r.Put("/assets/{assetID}", h.UpdateAsset)
		r.Delete("/assets/{assetID}", h.DeleteAsset)
		r.Post("/assets/{assetID}/activate", h.ActivateAsset)
		r.Post("/assets/{assetID}/dispose", h.DisposeAsset)
		r.Post("/assets/{assetID}/depreciation", h.RecordDepreciation)
		r.Get("/assets/{assetID}/depreciation", h.GetDepreciationHistory)

		// Inventory - Product Categories
		r.Get("/product-categories", h.ListProductCategories)
		r.Post("/product-categories", h.CreateProductCategory)
		r.Post("/product-categories/import", h.ImportProductCategories)
		r.Get("/product-categories/{categoryID}", h.GetProductCategory)
		r.Delete("/product-categories/{categoryID}", h.DeleteProductCategory)

		// Inventory - Products
		r.Get("/products", h.ListProducts)
		r.Post("/products", h.CreateProduct)
		r.Post("/products/import", h.ImportProducts)
		r.Get("/products/{productID}", h.GetProduct)
		r.Put("/products/{productID}", h.UpdateProduct)
		r.Delete("/products/{productID}", h.DeleteProduct)
		r.Get("/products/{productID}/stock-levels", h.GetStockLevels)
		r.Get("/products/{productID}/movements", h.GetInventoryMovements)
		r.Get("/inventory/valuation", h.GetInventoryValuation)
		r.Get("/inventory/subledger-reconciliation", h.GetInventorySubledgerReconciliation)
		r.Get("/inventory/lots", h.GetInventoryLotReport)

		// Inventory - Warehouses
		r.Get("/warehouses", h.ListWarehouses)
		r.Post("/warehouses", h.CreateWarehouse)
		r.Post("/warehouses/import", h.ImportWarehouses)
		r.Get("/warehouses/{warehouseID}", h.GetWarehouse)
		r.Put("/warehouses/{warehouseID}", h.UpdateWarehouse)
		r.Delete("/warehouses/{warehouseID}", h.DeleteWarehouse)

		// Inventory - Stock Operations
		r.Post("/inventory/adjust", h.AdjustStock)
		r.Post("/inventory/stock-import", h.ImportStockAdjustments)
		r.Post("/inventory/issue", h.IssueStock)
		r.Post("/inventory/transfer", h.TransferStock)
		r.Post("/inventory/reserve", h.ReserveStock)
		r.Post("/inventory/release", h.ReleaseStock)

		// Payments
		r.Get("/payments", h.ListPayments)
		r.Post("/payments", h.CreatePayment)
		r.Post("/payments/import", h.ImportPayments)
		r.Post("/payments/sepa-export", h.ExportSEPAPayments)
		r.Get("/payments/{paymentID}", h.GetPayment)
		r.Post("/payments/{paymentID}/allocate", h.AllocatePayment)
		r.Post("/payments/{paymentID}/reverse", h.ReversePayment)
		r.Get("/payments/unallocated", h.GetUnallocatedPayments)

		// Reports
		r.Get("/reports/trial-balance", h.GetTrialBalance)
		r.Get("/reports/account-balance/{accountID}", h.GetAccountBalance)
		r.Get("/reports/balance-sheet", h.GetBalanceSheet)
		r.Get("/reports/income-statement", h.GetIncomeStatement)
		r.Get("/reports/consolidated", h.GetConsolidatedReport)
		r.Get("/reports/annual", h.GetAnnualReport)
		r.Get("/reports/cash-flow", h.GetCashFlowStatement)
		r.Get("/reports/cash-flow/mapping", h.GetCashFlowMapping)
		r.Put("/reports/cash-flow/mapping", h.UpdateCashFlowMapping)
		r.Get("/reports/balance-confirmations", h.GetBalanceConfirmationSummary)
		r.Get("/reports/balance-confirmations/{contactID}", h.GetBalanceConfirmation)
		r.Get("/reports/contact-statements/{contactID}", h.GetContactStatement)
		r.Get("/reports/sales-margin", h.GetSalesMarginReport)
		r.Get("/reports/customer-profitability", h.GetCustomerProfitabilityReport)
		r.Get("/reports/budget-vs-actual", h.GetBudgetVsActualReport)

		// Cost Centers
		r.Get("/cost-centers", h.ListCostCenters)
		r.Post("/cost-centers", h.CreateCostCenter)
		r.Post("/cost-centers/import", h.ImportCostCenters)
		r.Get("/cost-centers/report", h.GetCostCenterReport)
		r.Get("/cost-centers/allocations", h.ListCostAllocations)
		r.Post("/cost-centers/allocations", h.CreateCostAllocation)
		r.Post("/cost-centers/allocations/import", h.ImportCostAllocations)
		r.Get("/cost-centers/{costCenterID}", h.GetCostCenter)
		r.Put("/cost-centers/{costCenterID}", h.UpdateCostCenter)
		r.Delete("/cost-centers/{costCenterID}", h.DeleteCostCenter)

		// Analytics
		r.Get("/analytics/dashboard", h.GetDashboardSummary)
		r.Get("/analytics/revenue-expense", h.GetRevenueExpenseChart)
		r.Get("/analytics/cash-flow", h.GetCashFlowChart)
		r.Get("/analytics/activity", h.GetRecentActivity)
		r.Get("/reports/aging/receivables", h.GetReceivablesAging)
		r.Get("/reports/aging/payables", h.GetPayablesAging)

		// Recurring Invoices
		r.Get("/recurring-invoices", h.ListRecurringInvoices)
		r.Post("/recurring-invoices", h.CreateRecurringInvoice)
		r.Post("/recurring-invoices/import", h.ImportRecurringInvoices)
		r.Post("/recurring-invoices/from-invoice/{invoiceID}", h.CreateRecurringInvoiceFromInvoice)
		r.Post("/recurring-invoices/generate-due", h.GenerateDueRecurringInvoices)
		r.Get("/recurring-invoices/{recurringID}", h.GetRecurringInvoice)
		r.Put("/recurring-invoices/{recurringID}", h.UpdateRecurringInvoice)
		r.Delete("/recurring-invoices/{recurringID}", h.DeleteRecurringInvoice)
		r.Post("/recurring-invoices/{recurringID}/pause", h.PauseRecurringInvoice)
		r.Post("/recurring-invoices/{recurringID}/resume", h.ResumeRecurringInvoice)
		r.Post("/recurring-invoices/{recurringID}/generate", h.GenerateRecurringInvoice)

		// Email Settings
		r.With(h.RequireTenantPermission(canManageSettings)).Get("/settings/smtp", h.GetSMTPConfig)
		r.With(h.RequireTenantPermission(canManageSettings)).Put("/settings/smtp", h.UpdateSMTPConfig)
		r.With(h.RequireTenantPermission(canManageSettings)).Post("/settings/smtp/test", h.TestSMTP)
		r.With(h.RequireTenantPermission(canManageSettings)).Get("/email-templates", h.ListEmailTemplates)
		r.With(h.RequireTenantPermission(canManageSettings)).Put("/email-templates/{templateType}", h.UpdateEmailTemplate)
		r.With(h.RequireTenantPermission(canManageSettings)).Get("/email-log", h.GetEmailLog)

		// Reminder Rules (Automated Payment Reminders)
		r.With(h.RequireTenantPermission(canManageSettings)).Get("/reminder-rules", h.ListReminderRules)
		r.With(h.RequireTenantPermission(canManageSettings)).Post("/reminder-rules", h.CreateReminderRule)
		r.With(h.RequireTenantPermission(canManageSettings)).Post("/reminder-rules/trigger", h.TriggerReminders)
		r.With(h.RequireTenantPermission(canManageSettings)).Get("/reminder-rules/{ruleID}", h.GetReminderRule)
		r.With(h.RequireTenantPermission(canManageSettings)).Put("/reminder-rules/{ruleID}", h.UpdateReminderRule)
		r.With(h.RequireTenantPermission(canManageSettings)).Delete("/reminder-rules/{ruleID}", h.DeleteReminderRule)

		// Interest Calculations
		r.With(h.RequireTenantPermission(canManageSettings)).Get("/settings/interest", h.GetInterestSettings)
		r.With(h.RequireTenantPermission(canManageSettings)).Put("/settings/interest", h.UpdateInterestSettings)
		r.Get("/invoices/overdue-with-interest", h.GetOverdueInvoicesWithInterest)
		r.Get("/invoices/{invoiceID}/interest", h.GetInvoiceInterest)
		r.Get("/invoices/{invoiceID}/interest/history", h.GetInvoiceInterestHistory)

		// Email Actions (linked to invoices/payments)
		r.Post("/invoices/{invoiceID}/email", h.EmailInvoice)
		r.Post("/payments/{paymentID}/email-receipt", h.EmailPaymentReceipt)

		// Bank Accounts
		r.Get("/bank-accounts", h.ListBankAccounts)
		r.Post("/bank-accounts", h.CreateBankAccount)
		r.Post("/bank-accounts/import", h.ImportBankAccounts)
		r.Get("/bank-match-rules", h.ListBankMatchRules)
		r.Post("/bank-match-rules", h.CreateBankMatchRule)
		r.Get("/bank-match-rules/{ruleID}", h.GetBankMatchRule)
		r.Put("/bank-match-rules/{ruleID}", h.UpdateBankMatchRule)
		r.Delete("/bank-match-rules/{ruleID}", h.DeleteBankMatchRule)
		r.Get("/bank-accounts/{accountID}", h.GetBankAccount)
		r.Put("/bank-accounts/{accountID}", h.UpdateBankAccount)
		r.Delete("/bank-accounts/{accountID}", h.DeleteBankAccount)

		// Bank Transactions
		r.Get("/bank-accounts/{accountID}/transactions", h.ListBankTransactions)
		r.Post("/bank-accounts/{accountID}/import", h.ImportBankTransactions)
		r.Get("/bank-accounts/{accountID}/import-history", h.GetImportHistory)
		r.Get("/bank-transactions/{transactionID}", h.GetBankTransaction)
		r.Get("/bank-transactions/{transactionID}/suggestions", h.GetMatchSuggestions)
		r.Post("/bank-transactions/{transactionID}/match", h.MatchBankTransaction)
		r.Post("/bank-transactions/{transactionID}/unmatch", h.UnmatchBankTransaction)
		r.Post("/bank-transactions/{transactionID}/review", h.ReviewBankTransaction)
		r.Post("/bank-transactions/{transactionID}/create-payment", h.CreatePaymentFromTransaction)

		// Bank Reconciliation
		r.Get("/bank-accounts/{accountID}/reconciliations", h.ListReconciliations)
		r.Post("/bank-accounts/{accountID}/reconciliation", h.CreateReconciliation)
		r.Get("/reconciliations/{reconciliationID}", h.GetReconciliation)
		r.Post("/reconciliations/{reconciliationID}/complete", h.CompleteReconciliation)
		r.Post("/bank-accounts/{accountID}/auto-match", h.AutoMatchTransactions)

		// Tax (Estonian KMD)
		r.Post("/tax/kmd", h.HandleGenerateKMD)
		r.With(h.RequireTenantPermission(canCreateEntries)).Post("/tax/kmd/import-history", h.HandleImportKMDHistory)
		r.Get("/tax/kmd", h.HandleListKMD)
		r.Get("/tax/kmd/{year}/{month}/xml", h.HandleExportKMD)
		r.Get("/tax/kmd/{year}/{month}/inf", h.HandleGenerateKMDINF)
		r.Post("/tax/kmd/{year}/{month}/submit", h.HandleMarkKMDSubmitted)
		r.Post("/tax/kmd/{year}/{month}/accept", h.HandleMarkKMDAccepted)
		r.Get("/tax/eu-vat/oss", h.HandleGenerateEUVATOSS)

		// Payroll - Employees
		r.Get("/employees", h.ListEmployees)
		r.Post("/employees", h.CreateEmployee)
		r.Post("/employees/import", h.ImportEmployees)
		r.Get("/employees/{employeeID}", h.GetEmployee)
		r.Put("/employees/{employeeID}", h.UpdateEmployee)
		r.Post("/employees/{employeeID}/salary", h.SetBaseSalary)
		r.Get("/employees/{employeeID}/salary-components", h.ListSalaryComponents)
		r.Post("/employees/{employeeID}/salary-components", h.AddSalaryComponent)

		// Payroll - Runs
		r.Get("/payroll-runs", h.ListPayrollRuns)
		r.Post("/payroll-runs", h.CreatePayrollRun)
		r.With(h.RequireTenantPermission(canCreateEntries)).Post("/payroll-runs/import-history", h.ImportPayrollHistory)
		r.Get("/payroll-runs/{runID}", h.GetPayrollRun)
		r.Patch("/payroll-runs/{runID}/payment-date", h.UpdatePayrollRunPaymentDate)
		r.Post("/payroll-runs/{runID}/calculate", h.CalculatePayroll)
		r.Post("/payroll-runs/{runID}/process", h.ProcessPayrollRun)
		r.Post("/payroll-runs/{runID}/approve", h.ApprovePayroll)
		r.Get("/payroll-runs/{runID}/payslips", h.GetPayslips)
		r.Get("/payroll-runs/{runID}/payslips/{payslipID}/pdf", h.GetPayslipPDF)
		r.Post("/payroll-runs/{runID}/tsd", h.GenerateTSD)

		// Payroll - Tax Preview
		r.Post("/payroll/tax-preview", h.CalculateTaxPreview)

		// Leave/Absence Management
		r.Get("/absence-types", h.ListAbsenceTypes)
		r.Get("/absence-types/{typeID}", h.GetAbsenceType)
		r.Get("/employees/{employeeID}/leave-balances", h.ListLeaveBalances)
		r.Get("/employees/{employeeID}/leave-balances/{year}", h.GetLeaveBalancesByYear)
		r.Put("/employees/{employeeID}/leave-balances/{year}/{typeID}", h.UpdateLeaveBalance)
		r.Post("/employees/{employeeID}/leave-balances/{year}/initialize", h.InitializeLeaveBalances)
		r.With(h.RequireTenantPermission(canCreateEntries)).Post("/leave-balances/import", h.ImportLeaveBalances)
		r.Get("/leave-records", h.ListLeaveRecords)
		r.Post("/leave-records", h.CreateLeaveRecord)
		r.Get("/leave-records/{recordID}", h.GetLeaveRecord)
		r.Post("/leave-records/{recordID}/approve", h.ApproveLeaveRecord)
		r.Post("/leave-records/{recordID}/reject", h.RejectLeaveRecord)
		r.Post("/leave-records/{recordID}/cancel", h.CancelLeaveRecord)

		// TSD Declarations
		r.Get("/tsd", h.ListTSD)
		r.Get("/tsd/{year}/{month}", h.GetTSD)
		r.Get("/tsd/{year}/{month}/xml", h.ExportTSDXML)
		r.Get("/tsd/{year}/{month}/csv", h.ExportTSDCSV)
		r.With(h.RequireTenantPermission(canCreateEntries)).Post("/tsd/import-history", h.ImportTSDHistory)
		r.Post("/tsd/{year}/{month}/submit", h.MarkTSDSubmitted)
		r.Post("/tsd/{year}/{month}/accept", h.MarkTSDAccepted)
		r.Post("/tsd/{year}/{month}/reject", h.MarkTSDRejected)

		// User Management
		r.Get("/users", h.ListTenantUsers)
		r.Delete("/users/{userID}", h.RemoveTenantUser)
		r.Put("/users/{userID}/role", h.UpdateTenantUserRole)
		r.Put("/users/{userID}/status", h.UpdateTenantUserStatus)
		r.Get("/users/{userID}/sessions", h.ListTenantUserAuthSessions)
		r.Delete("/users/{userID}/sessions", h.RevokeTenantUserAuthSessions)
		r.Delete("/users/{userID}/sessions/{sessionID}", h.RevokeTenantUserAuthSession)
		r.Get("/users/{userID}/api-tokens", h.ListTenantUserAPITokens)
		r.Delete("/users/{userID}/api-tokens/{tokenID}", h.RevokeTenantUserAPIToken)
		r.Get("/users/{userID}/security-events", h.ListTenantUserSecurityAuditEvents)
		r.Get("/audit-events", h.ListTenantAuditEvents)

		// Webhooks
		r.With(h.RequireTenantPermission(canManageSettings)).Get("/webhooks/events", h.ListWebhookEventTypes)
		r.With(h.RequireTenantPermission(canManageSettings)).Get("/webhooks", h.ListWebhookEndpoints)
		r.With(h.RequireTenantPermission(canManageSettings)).Post("/webhooks", h.CreateWebhookEndpoint)
		r.With(h.RequireTenantPermission(canManageSettings)).Get("/webhooks/{webhookID}", h.GetWebhookEndpoint)
		r.With(h.RequireTenantPermission(canManageSettings)).Put("/webhooks/{webhookID}", h.UpdateWebhookEndpoint)
		r.With(h.RequireTenantPermission(canManageSettings)).Delete("/webhooks/{webhookID}", h.DeleteWebhookEndpoint)
		r.With(h.RequireTenantPermission(canManageSettings)).Get("/webhooks/{webhookID}/deliveries", h.ListWebhookDeliveries)
		r.With(h.RequireTenantPermission(canManageSettings)).Post("/webhooks/{webhookID}/test", h.TestWebhookEndpoint)

		// Expenses
		r.Get("/expenses", h.ListExpenses)
		r.Post("/expenses", h.CreateExpense)
		r.Post("/expenses/import", h.ImportExpenses)
		r.Get("/expenses/{expenseID}", h.GetExpense)
		r.Post("/expenses/{expenseID}/submit", h.SubmitExpense)
		r.Post("/expenses/{expenseID}/approve", h.ApproveExpense)
		r.Post("/expenses/{expenseID}/reject", h.RejectExpense)
		r.Post("/expenses/{expenseID}/post", h.PostExpense)

		// Invitations
		r.Get("/invitations", h.ListInvitations)
		r.Post("/invitations", h.CreateInvitation)
		r.Delete("/invitations/{invitationID}", h.RevokeInvitation)

		// Tenant Plugin Management
		r.Get("/plugins", h.ListTenantPlugins)
		r.With(h.RequireTenantPermission(canManageSettings)).Post("/plugins/{pluginID}/enable", h.EnableTenantPlugin)
		r.With(h.RequireTenantPermission(canManageSettings)).Post("/plugins/{pluginID}/disable", h.DisableTenantPlugin)
		r.With(h.RequireTenantPermission(canManageSettings)).Get("/plugins/{pluginID}/settings", h.GetTenantPluginSettings)
		r.With(h.RequireTenantPermission(canManageSettings)).Put("/plugins/{pluginID}/settings", h.UpdateTenantPluginSettings)
		r.Get("/plugins/{pluginID}/runtime/*", h.InvokeTenantPluginRoute)
		r.Post("/plugins/{pluginID}/runtime/*", h.InvokeTenantPluginRoute)
		r.Put("/plugins/{pluginID}/runtime/*", h.InvokeTenantPluginRoute)
		r.Patch("/plugins/{pluginID}/runtime/*", h.InvokeTenantPluginRoute)
		r.Delete("/plugins/{pluginID}/runtime/*", h.InvokeTenantPluginRoute)
	})
}
