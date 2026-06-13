package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/HMB-research/open-accounting/internal/accounting"
	"github.com/HMB-research/open-accounting/internal/assets"
	"github.com/HMB-research/open-accounting/internal/auth"
	"github.com/HMB-research/open-accounting/internal/banking"
	"github.com/HMB-research/open-accounting/internal/banking/mappers/registry"
	"github.com/HMB-research/open-accounting/internal/contacts"
	"github.com/HMB-research/open-accounting/internal/cutover"
	"github.com/HMB-research/open-accounting/internal/expenses"
	"github.com/HMB-research/open-accounting/internal/inventory"
	"github.com/HMB-research/open-accounting/internal/invoicing"
	"github.com/HMB-research/open-accounting/internal/orders"
	"github.com/HMB-research/open-accounting/internal/payments"
	"github.com/HMB-research/open-accounting/internal/payroll"
	"github.com/HMB-research/open-accounting/internal/quotes"
	"github.com/HMB-research/open-accounting/internal/recurring"
	"github.com/HMB-research/open-accounting/internal/tax"
)

type migrationStepExecutor interface {
	ExecuteMigrationStep(ctx context.Context, tenantID, schemaName, userID string, step cutover.MigrationExecutionStep, file cutover.BundleFile, req *cutover.ExecuteMigrationRequest) (any, error)
}

type handlerMigrationStepExecutor struct {
	h *Handlers
}

// ExecuteMigration validates, plans, and optionally executes a migration bundle server-side.
// @Summary Execute migration bundle
// @Description Validate and plan CSV/XML cutover files, then execute ready imports server-side when confirm is true
// @Tags Migration
// @Accept json
// @Produce json
// @Param tenantID path string true "Tenant ID"
// @Param request body cutover.ExecuteMigrationRequest true "Migration bundle files and execution context"
// @Success 200 {object} cutover.MigrationExecutionRun
// @Failure 400 {object} cutover.MigrationExecutionRun
// @Failure 409 {object} cutover.MigrationExecutionRun
// @Failure 401 {object} object{error=string}
// @Failure 403 {object} object{error=string}
// @Router /tenants/{tenantID}/migration/execute [post]
func (h *Handlers) ExecuteMigration(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.GetClaims(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	tenantID := strings.TrimSpace(chi.URLParam(r, "tenantID"))
	if tenantID == "" {
		respondError(w, http.StatusBadRequest, "tenantID is required")
		return
	}
	schemaName := h.getSchemaName(r.Context(), tenantID)

	var req cutover.ExecuteMigrationRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	resumeFromRun, err := h.resolveMigrationExecutionResumeRun(r.Context(), schemaName, tenantID, &req)
	if err != nil {
		if errors.Is(err, cutover.ErrMigrationExecutionRunNotFound) {
			respondError(w, http.StatusNotFound, err.Error())
			return
		}
		respondError(w, http.StatusInternalServerError, "Failed to load migration execution resume run")
		return
	}

	plan, err := cutover.BuildMigrationExecutionPlan(req.PlanRequest())
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	run := cutover.NewResumableMigrationExecutionRun(plan, req.Confirm, resumeFromRun)
	if !req.Confirm {
		if err := h.saveMigrationExecutionRun(r.Context(), schemaName, tenantID, claims.UserID, run); err != nil {
			respondError(w, http.StatusInternalServerError, "Failed to save migration execution run")
			return
		}
		respondJSON(w, http.StatusOK, run)
		return
	}
	if !plan.Summary.Ready {
		if err := h.saveMigrationExecutionRun(r.Context(), schemaName, tenantID, claims.UserID, run); err != nil {
			respondError(w, http.StatusInternalServerError, "Failed to save migration execution run")
			return
		}
		respondJSON(w, http.StatusConflict, run)
		return
	}
	if err := h.saveMigrationExecutionRun(r.Context(), schemaName, tenantID, claims.UserID, run); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to save migration execution run")
		return
	}

	filesByKey := migrationFilesByExecutionStepKey(req.Files)
	executor := h.effectiveMigrationExecutor()
	for index, step := range plan.Steps {
		if run.Steps[index].Status == cutover.MigrationExecutionResultSucceeded {
			continue
		}
		if step.Status != cutover.MigrationExecutionStepReady {
			run.Steps[index].Status = cutover.MigrationExecutionResultSkipped
			run.Steps[index].Message = "Step is not ready to execute."
			continue
		}
		file, ok := filesByKey[migrationExecutionStepFileKey(step.Kind, step.FileName)]
		if !ok {
			run.Steps[index].Status = cutover.MigrationExecutionResultFailed
			run.Steps[index].Error = "migration bundle file not found"
			run.Summary.FailedStepCount++
			run.Summary.Status = "failed"
			if err := h.saveMigrationExecutionRun(r.Context(), schemaName, tenantID, claims.UserID, run); err != nil {
				respondError(w, http.StatusInternalServerError, "Failed to save migration execution run")
				return
			}
			respondJSON(w, http.StatusBadRequest, run)
			return
		}
		response, err := executor.ExecuteMigrationStep(r.Context(), tenantID, schemaName, claims.UserID, step, file, &req)
		if err != nil {
			run.Steps[index].Status = cutover.MigrationExecutionResultFailed
			run.Steps[index].Error = err.Error()
			run.Summary.FailedStepCount++
			run.Summary.Status = "failed"
			if err := h.saveMigrationExecutionRun(r.Context(), schemaName, tenantID, claims.UserID, run); err != nil {
				respondError(w, http.StatusInternalServerError, "Failed to save migration execution run")
				return
			}
			respondJSON(w, http.StatusBadRequest, run)
			return
		}
		run.Steps[index].Status = cutover.MigrationExecutionResultSucceeded
		run.Steps[index].Message = "Import completed."
		run.Steps[index].Response = response
		run.Summary.SucceededStepCount++
		if err := h.saveMigrationExecutionRun(r.Context(), schemaName, tenantID, claims.UserID, run); err != nil {
			respondError(w, http.StatusInternalServerError, "Failed to save migration execution run")
			return
		}
	}
	run.Summary.Status = "succeeded"
	if err := h.saveMigrationExecutionRun(r.Context(), schemaName, tenantID, claims.UserID, run); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to save migration execution run")
		return
	}
	respondJSON(w, http.StatusOK, run)
}

// ListMigrationExecutionRuns returns saved migration execution runs for dashboard and CLI resume workflows.
// @Summary List migration execution runs
// @Description List saved tenant-scoped migration execution runs
// @Tags Migration
// @Produce json
// @Param tenantID path string true "Tenant ID"
// @Param status query string false "Filter by run status"
// @Param limit query int false "Maximum runs to return"
// @Success 200 {array} cutover.MigrationExecutionRun
// @Failure 400 {object} object{error=string}
// @Failure 401 {object} object{error=string}
// @Failure 403 {object} object{error=string}
// @Router /tenants/{tenantID}/migration/execution-runs [get]
func (h *Handlers) ListMigrationExecutionRuns(w http.ResponseWriter, r *http.Request) {
	tenantID := strings.TrimSpace(chi.URLParam(r, "tenantID"))
	if tenantID == "" {
		respondError(w, http.StatusBadRequest, "tenantID is required")
		return
	}
	limit := 50
	if rawLimit := strings.TrimSpace(r.URL.Query().Get("limit")); rawLimit != "" {
		parsed, err := strconv.Atoi(rawLimit)
		if err != nil || parsed < 1 || parsed > 200 {
			respondError(w, http.StatusBadRequest, "limit must be between 1 and 200")
			return
		}
		limit = parsed
	}
	store := h.migrationRunStore
	if store == nil {
		respondError(w, http.StatusInternalServerError, "Migration execution run storage is not configured")
		return
	}
	runs, err := store.ListExecutionRuns(r.Context(), h.getSchemaName(r.Context(), tenantID), tenantID, cutover.MigrationExecutionRunFilter{
		Status: strings.TrimSpace(r.URL.Query().Get("status")),
		Limit:  limit,
	})
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to list migration execution runs")
		return
	}
	respondJSON(w, http.StatusOK, runs)
}

// GetMigrationExecutionRun returns one saved migration execution run.
// @Summary Get migration execution run
// @Description Return a saved tenant-scoped migration execution run snapshot
// @Tags Migration
// @Produce json
// @Param tenantID path string true "Tenant ID"
// @Param runID path string true "Migration execution run ID"
// @Success 200 {object} cutover.MigrationExecutionRun
// @Failure 400 {object} object{error=string}
// @Failure 401 {object} object{error=string}
// @Failure 403 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Router /tenants/{tenantID}/migration/execution-runs/{runID} [get]
func (h *Handlers) GetMigrationExecutionRun(w http.ResponseWriter, r *http.Request) {
	tenantID := strings.TrimSpace(chi.URLParam(r, "tenantID"))
	runID := strings.TrimSpace(chi.URLParam(r, "runID"))
	if tenantID == "" {
		respondError(w, http.StatusBadRequest, "tenantID is required")
		return
	}
	if runID == "" {
		respondError(w, http.StatusBadRequest, "runID is required")
		return
	}
	store := h.migrationRunStore
	if store == nil {
		respondError(w, http.StatusInternalServerError, "Migration execution run storage is not configured")
		return
	}
	run, err := store.GetExecutionRun(r.Context(), h.getSchemaName(r.Context(), tenantID), tenantID, runID)
	if errors.Is(err, cutover.ErrMigrationExecutionRunNotFound) {
		respondError(w, http.StatusNotFound, err.Error())
		return
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to load migration execution run")
		return
	}
	respondJSON(w, http.StatusOK, run)
}

func (h *Handlers) resolveMigrationExecutionResumeRun(ctx context.Context, schemaName, tenantID string, req *cutover.ExecuteMigrationRequest) (*cutover.MigrationExecutionRun, error) {
	if req == nil {
		return nil, nil
	}
	if runID := strings.TrimSpace(req.ResumeFromRunID); runID != "" {
		if h.migrationRunStore == nil {
			return nil, fmt.Errorf("migration execution run storage is not configured")
		}
		return h.migrationRunStore.GetExecutionRun(ctx, schemaName, tenantID, runID)
	}
	return req.ResumeFromRun, nil
}

func (h *Handlers) saveMigrationExecutionRun(ctx context.Context, schemaName, tenantID, createdBy string, run *cutover.MigrationExecutionRun) error {
	if h.migrationRunStore == nil {
		return nil
	}
	_, err := h.migrationRunStore.SaveExecutionRun(ctx, schemaName, tenantID, createdBy, run)
	return err
}

func (h *Handlers) effectiveMigrationExecutor() migrationStepExecutor {
	if h.migrationExecutor != nil {
		return h.migrationExecutor
	}
	return &handlerMigrationStepExecutor{h: h}
}

func migrationFilesByExecutionStepKey(files []cutover.BundleFile) map[string]cutover.BundleFile {
	filesByKey := make(map[string]cutover.BundleFile, len(files))
	for _, file := range files {
		filesByKey[migrationExecutionStepFileKey(file.Kind, file.FileName)] = file
	}
	return filesByKey
}

func migrationExecutionStepFileKey(kind cutover.FileKind, fileName string) string {
	return string(kind) + "\x00" + strings.TrimSpace(fileName)
}

func (e *handlerMigrationStepExecutor) ExecuteMigrationStep(ctx context.Context, tenantID, schemaName, userID string, step cutover.MigrationExecutionStep, file cutover.BundleFile, req *cutover.ExecuteMigrationRequest) (any, error) {
	h := e.h
	switch step.Kind {
	case cutover.KindAccounts:
		return h.accountingService.ImportAccountsCSV(ctx, schemaName, tenantID, &accounting.ImportAccountsRequest{CSVContent: file.CSVContent, FileName: file.FileName})
	case cutover.KindContacts:
		return h.contactsService.ImportCSV(ctx, tenantID, schemaName, &contacts.ImportContactsRequest{CSVContent: file.CSVContent, FileName: file.FileName})
	case cutover.KindEmployees:
		return h.payrollService.ImportEmployeesCSV(ctx, schemaName, tenantID, &payroll.ImportEmployeesRequest{CSVContent: file.CSVContent, FileName: file.FileName})
	case cutover.KindExpenses:
		lockDate, err := h.getTenantPeriodLockDate(ctx, tenantID)
		if err != nil {
			return nil, fmt.Errorf("validate period lock: %w", err)
		}
		return h.expensesService.ImportExpensesCSV(ctx, schemaName, tenantID, &expenses.ImportExpensesRequest{CSVContent: file.CSVContent, FileName: file.FileName, UserID: userID, LockDate: lockDate})
	case cutover.KindInvoices:
		contactsList, productsList, err := h.migrationImportReferences(ctx, tenantID, schemaName)
		if err != nil {
			return nil, err
		}
		return h.invoicingService.ImportCSV(ctx, tenantID, schemaName, contactsList, productsList, &invoicing.ImportInvoicesRequest{CSVContent: file.CSVContent, FileName: file.FileName, UserID: userID}, func(issueDate time.Time) error {
			return h.ensurePeriodUnlocked(ctx, tenantID, issueDate)
		})
	case cutover.KindEInvoices:
		invoiceType, err := parseMigrationExecuteInvoiceType(req.EInvoiceInvoiceType)
		if err != nil {
			return nil, err
		}
		contactsList, err := h.contactsService.List(ctx, tenantID, schemaName, nil)
		if err != nil {
			return nil, fmt.Errorf("load contacts: %w", err)
		}
		return h.invoicingService.ImportEInvoiceXML(ctx, tenantID, schemaName, contactsList, &invoicing.ImportEInvoiceRequest{XMLContent: file.XMLContent, FileName: file.FileName, InvoiceType: invoiceType, UserID: userID}, func(issueDate time.Time) error {
			return h.ensurePeriodUnlocked(ctx, tenantID, issueDate)
		})
	case cutover.KindPayments:
		lockDate, err := h.getTenantPeriodLockDate(ctx, tenantID)
		if err != nil {
			return nil, fmt.Errorf("validate period lock: %w", err)
		}
		return h.paymentsService.ImportPaymentsCSV(ctx, tenantID, schemaName, &payments.ImportPaymentsRequest{CSVContent: file.CSVContent, FileName: file.FileName, UserID: userID, LockDate: lockDate})
	case cutover.KindBankAccounts:
		rows, err := banking.ParseBankAccountCSVRows(file.CSVContent)
		if err != nil {
			return nil, err
		}
		return h.bankingService.ImportBankAccounts(ctx, schemaName, tenantID, &banking.ImportBankAccountsRequest{FileName: file.FileName, Rows: rows, SkipDuplicates: true})
	case cutover.KindBankTransactions:
		accountID := strings.TrimSpace(req.BankTransactionAccountID)
		if accountID == "" {
			return nil, errors.New("bank_transaction_account_id is required")
		}
		rows, err := registry.ParseTransactions(file.CSVContent, strings.TrimSpace(req.BankTransactionFormat))
		if err != nil {
			return nil, err
		}
		return h.bankingService.ImportTransactions(ctx, schemaName, tenantID, accountID, &banking.ImportCSVRequest{FileName: file.FileName, Transactions: rows, SkipDuplicates: true})
	case cutover.KindPayrollHistory:
		return h.payrollService.ImportPayrollHistoryCSV(ctx, schemaName, tenantID, userID, &payroll.ImportPayrollHistoryRequest{CSVContent: file.CSVContent, FileName: file.FileName})
	case cutover.KindLeaveBalances:
		return h.absenceService.ImportLeaveBalancesCSV(ctx, schemaName, tenantID, &payroll.ImportLeaveBalancesRequest{CSVContent: file.CSVContent, FileName: file.FileName})
	case cutover.KindTSDHistory:
		return h.payrollService.ImportTSDHistoryCSV(ctx, schemaName, tenantID, &payroll.ImportTSDHistoryRequest{CSVContent: file.CSVContent, FileName: file.FileName})
	case cutover.KindKMDHistory:
		return h.taxService.ImportKMDHistoryCSV(ctx, schemaName, tenantID, &tax.ImportKMDHistoryRequest{CSVContent: file.CSVContent, FileName: file.FileName})
	case cutover.KindQuotes:
		contactsList, productsList, err := h.migrationImportReferences(ctx, tenantID, schemaName)
		if err != nil {
			return nil, err
		}
		return h.quotesService.ImportCSV(ctx, tenantID, schemaName, contactsList, productsList, &quotes.ImportQuotesRequest{CSVContent: file.CSVContent, FileName: file.FileName, UserID: userID})
	case cutover.KindOrders:
		contactsList, productsList, err := h.migrationImportReferences(ctx, tenantID, schemaName)
		if err != nil {
			return nil, err
		}
		return h.ordersService.ImportCSV(ctx, tenantID, schemaName, contactsList, productsList, &orders.ImportOrdersRequest{CSVContent: file.CSVContent, FileName: file.FileName, UserID: userID})
	case cutover.KindRecurringInvoices:
		contactsList, productsList, err := h.migrationImportReferences(ctx, tenantID, schemaName)
		if err != nil {
			return nil, err
		}
		return h.recurringService.ImportCSV(ctx, tenantID, schemaName, contactsList, productsList, &recurring.ImportRecurringInvoicesRequest{CSVContent: file.CSVContent, FileName: file.FileName, UserID: userID})
	case cutover.KindCostCenters:
		return h.costCenterService.ImportCostCentersCSV(ctx, schemaName, tenantID, &accounting.ImportCostCentersRequest{CSVContent: file.CSVContent, FileName: file.FileName})
	case cutover.KindCostAllocations:
		return h.costCenterService.ImportCostAllocationsCSV(ctx, schemaName, tenantID, &accounting.ImportCostAllocationsRequest{CSVContent: file.CSVContent, FileName: file.FileName})
	case cutover.KindProductCategories:
		return h.inventoryService.ImportProductCategoriesCSV(ctx, tenantID, schemaName, &inventory.ImportProductCategoriesRequest{CSVContent: file.CSVContent, FileName: file.FileName})
	case cutover.KindWarehouses:
		return h.inventoryService.ImportWarehousesCSV(ctx, tenantID, schemaName, &inventory.ImportWarehousesRequest{CSVContent: file.CSVContent, FileName: file.FileName})
	case cutover.KindProducts:
		return h.inventoryService.ImportProductsCSV(ctx, tenantID, schemaName, &inventory.ImportProductsRequest{CSVContent: file.CSVContent, FileName: file.FileName})
	case cutover.KindStockAdjustments:
		return h.inventoryService.ImportStockAdjustmentsCSV(ctx, tenantID, schemaName, &inventory.ImportStockAdjustmentsRequest{CSVContent: file.CSVContent, FileName: file.FileName, UserID: userID})
	case cutover.KindFixedAssets:
		return h.assetsService.ImportAssetsCSV(ctx, tenantID, schemaName, &assets.ImportAssetsRequest{CSVContent: file.CSVContent, FileName: file.FileName, UserID: userID})
	case cutover.KindOpeningBalances:
		entryDate := strings.TrimSpace(req.OpeningBalanceEntryDate)
		if entryDate == "" {
			return nil, errors.New("opening_balance_entry_date is required")
		}
		return h.accountingService.ImportOpeningBalancesCSV(ctx, schemaName, tenantID, &accounting.ImportOpeningBalancesRequest{CSVContent: file.CSVContent, FileName: file.FileName, EntryDate: entryDate, Description: "Opening balances", Reference: migrationOpeningBalanceReference(entryDate), UserID: userID})
	case cutover.KindJournalEntries:
		lockDate, err := h.migrationPeriodLockDate(ctx, tenantID)
		if err != nil {
			return nil, err
		}
		return h.accountingService.ImportJournalEntriesCSV(ctx, schemaName, tenantID, &accounting.ImportJournalEntriesRequest{CSVContent: file.CSVContent, FileName: file.FileName, UserID: userID, PeriodLockDate: lockDate})
	default:
		return nil, fmt.Errorf("unsupported migration execution kind %q", step.Kind)
	}
}

func (h *Handlers) migrationImportReferences(ctx context.Context, tenantID, schemaName string) ([]contacts.Contact, []inventory.Product, error) {
	contactsList, err := h.contactsService.List(ctx, tenantID, schemaName, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("load contacts: %w", err)
	}
	productsList, err := h.importProductList(ctx, tenantID, schemaName)
	if err != nil {
		return nil, nil, fmt.Errorf("load products: %w", err)
	}
	return contactsList, productsList, nil
}

func (h *Handlers) migrationPeriodLockDate(ctx context.Context, tenantID string) (*time.Time, error) {
	if h.tenantService == nil {
		return nil, nil
	}
	tenantRecord, err := h.tenantService.GetTenant(ctx, tenantID)
	if err != nil || tenantRecord.Settings.PeriodLockDate == nil {
		return nil, err
	}
	lockDate, err := time.Parse("2006-01-02", strings.TrimSpace(*tenantRecord.Settings.PeriodLockDate))
	if err != nil {
		return nil, errors.New("tenant period_lock_date must be in YYYY-MM-DD format")
	}
	return &lockDate, nil
}

func parseMigrationExecuteInvoiceType(value string) (invoicing.InvoiceType, error) {
	normalized := strings.ToUpper(strings.TrimSpace(value))
	if normalized == "" {
		return "", nil
	}
	switch invoicing.InvoiceType(normalized) {
	case invoicing.InvoiceTypeSales, invoicing.InvoiceTypePurchase, invoicing.InvoiceTypeCreditNote:
		return invoicing.InvoiceType(normalized), nil
	default:
		return "", fmt.Errorf("invalid e_invoice_invoice_type %q", value)
	}
}

func migrationOpeningBalanceReference(entryDate string) string {
	trimmed := strings.TrimSpace(entryDate)
	if len(trimmed) >= 4 {
		return "OB-" + trimmed[:4]
	}
	return "OB"
}
