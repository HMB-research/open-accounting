package main

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/HMB-research/open-accounting/internal/auth"
	"github.com/HMB-research/open-accounting/internal/expenses"
	"github.com/HMB-research/open-accounting/internal/plugin"
)

// ListExpenses returns tracked tenant expenses.
// @Summary List expenses
// @Description List expense claims with optional status filtering
// @Tags Expenses
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param status query string false "Expense status"
// @Param limit query int false "Maximum expenses to return"
// @Success 200 {array} expenses.Expense
// @Failure 400 {object} object{error=string}
// @Router /tenants/{tenantID}/expenses [get]
func (h *Handlers) ListExpenses(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	limit := 0
	if rawLimit := strings.TrimSpace(r.URL.Query().Get("limit")); rawLimit != "" {
		parsed, err := strconv.Atoi(rawLimit)
		if err != nil || parsed < 0 {
			respondError(w, http.StatusBadRequest, "limit must be zero or greater")
			return
		}
		limit = parsed
	}

	result, err := h.expensesService.ListExpenses(r.Context(), schemaName, tenantID, expenses.ListExpensesFilter{
		Status: expenses.ExpenseStatus(strings.TrimSpace(r.URL.Query().Get("status"))),
		Limit:  limit,
	})
	if err != nil {
		respondExpenseError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, result)
}

// CreateExpense creates a tenant expense claim.
// @Summary Create expense
// @Description Create a draft expense claim. Receipt-backed expenses can be linked to uploaded receipt documents with entity_type=expense.
// @Tags Expenses
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param request body expenses.CreateExpenseRequest true "Expense details"
// @Success 201 {object} expenses.Expense
// @Failure 400 {object} object{error=string}
// @Failure 409 {object} object{error=string}
// @Router /tenants/{tenantID}/expenses [post]
func (h *Handlers) CreateExpense(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.GetClaims(r.Context())
	tenantID := chi.URLParam(r, "tenantID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	var req expenses.CreateExpenseRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	req.UserID = claims.UserID

	if h.rejectLockedPeriod(w, r.Context(), tenantID, expenseOperationDate(req.ExpenseDate)) {
		return
	}

	expense, err := h.expensesService.CreateExpense(r.Context(), schemaName, tenantID, &req)
	if err != nil {
		respondExpenseError(w, err)
		return
	}

	h.emitWebhookEvent(plugin.EventExpenseCreated, tenantID, expense)
	respondJSON(w, http.StatusCreated, expense)
}

// GetExpense returns one expense.
// @Summary Get expense
// @Description Get expense claim details
// @Tags Expenses
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param expenseID path string true "Expense ID"
// @Success 200 {object} expenses.Expense
// @Failure 404 {object} object{error=string}
// @Router /tenants/{tenantID}/expenses/{expenseID} [get]
func (h *Handlers) GetExpense(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	expenseID := chi.URLParam(r, "expenseID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	expense, err := h.expensesService.GetExpense(r.Context(), schemaName, tenantID, expenseID)
	if err != nil {
		respondExpenseError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, expense)
}

// SubmitExpense submits a draft expense for approval.
// @Summary Submit expense
// @Description Move a draft or rejected expense into submitted status
// @Tags Expenses
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param expenseID path string true "Expense ID"
// @Success 200 {object} expenses.Expense
// @Failure 400 {object} object{error=string}
// @Router /tenants/{tenantID}/expenses/{expenseID}/submit [post]
func (h *Handlers) SubmitExpense(w http.ResponseWriter, r *http.Request) {
	h.applyExpenseAction(w, r, plugin.EventExpenseSubmitted, func(schemaName, tenantID, expenseID, userID string) (*expenses.Expense, error) {
		return h.expensesService.SubmitExpense(r.Context(), schemaName, tenantID, expenseID, &expenses.ExpenseActionRequest{UserID: userID})
	})
}

// ApproveExpense approves a submitted expense after required receipt evidence passes.
// @Summary Approve expense
// @Description Approve a submitted expense. If the expense requires receipts, at least one approved receipt document must be linked.
// @Tags Expenses
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param expenseID path string true "Expense ID"
// @Success 200 {object} expenses.Expense
// @Failure 400 {object} object{error=string}
// @Failure 409 {object} object{error=string}
// @Router /tenants/{tenantID}/expenses/{expenseID}/approve [post]
func (h *Handlers) ApproveExpense(w http.ResponseWriter, r *http.Request) {
	h.applyExpenseAction(w, r, plugin.EventExpenseApproved, func(schemaName, tenantID, expenseID, userID string) (*expenses.Expense, error) {
		return h.expensesService.ApproveExpense(r.Context(), schemaName, tenantID, expenseID, &expenses.ExpenseActionRequest{UserID: userID})
	})
}

// RejectExpense rejects a submitted expense.
// @Summary Reject expense
// @Description Reject a submitted expense with a reason
// @Tags Expenses
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param expenseID path string true "Expense ID"
// @Param request body expenses.RejectExpenseRequest true "Rejection details"
// @Success 200 {object} expenses.Expense
// @Failure 400 {object} object{error=string}
// @Router /tenants/{tenantID}/expenses/{expenseID}/reject [post]
func (h *Handlers) RejectExpense(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.GetClaims(r.Context())
	tenantID := chi.URLParam(r, "tenantID")
	expenseID := chi.URLParam(r, "expenseID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	var req expenses.RejectExpenseRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	req.UserID = claims.UserID

	expense, err := h.expensesService.RejectExpense(r.Context(), schemaName, tenantID, expenseID, &req)
	if err != nil {
		respondExpenseError(w, err)
		return
	}

	h.emitWebhookEvent(plugin.EventExpenseRejected, tenantID, expense)
	respondJSON(w, http.StatusOK, expense)
}

// PostExpense posts an approved expense to the general ledger.
// @Summary Post expense
// @Description Create and post a balanced journal entry for an approved expense
// @Tags Expenses
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param expenseID path string true "Expense ID"
// @Success 200 {object} expenses.Expense
// @Failure 400 {object} object{error=string}
// @Failure 409 {object} object{error=string}
// @Router /tenants/{tenantID}/expenses/{expenseID}/post [post]
func (h *Handlers) PostExpense(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.GetClaims(r.Context())
	tenantID := chi.URLParam(r, "tenantID")
	expenseID := chi.URLParam(r, "expenseID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	expense, err := h.expensesService.GetExpense(r.Context(), schemaName, tenantID, expenseID)
	if err != nil {
		respondExpenseError(w, err)
		return
	}
	if h.rejectLockedPeriod(w, r.Context(), tenantID, expense.ExpenseDate) {
		return
	}

	expense, err = h.expensesService.PostExpense(r.Context(), schemaName, tenantID, expenseID, &expenses.ExpenseActionRequest{UserID: claims.UserID})
	if err != nil {
		respondExpenseError(w, err)
		return
	}

	h.emitWebhookEvent(plugin.EventExpensePosted, tenantID, expense)
	respondJSON(w, http.StatusOK, expense)
}

func (h *Handlers) applyExpenseAction(w http.ResponseWriter, r *http.Request, eventType string, action func(schemaName, tenantID, expenseID, userID string) (*expenses.Expense, error)) {
	claims, _ := auth.GetClaims(r.Context())
	tenantID := chi.URLParam(r, "tenantID")
	expenseID := chi.URLParam(r, "expenseID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	expense, err := action(schemaName, tenantID, expenseID, claims.UserID)
	if err != nil {
		respondExpenseError(w, err)
		return
	}

	h.emitWebhookEvent(eventType, tenantID, expense)
	respondJSON(w, http.StatusOK, expense)
}

func expenseOperationDate(value time.Time) time.Time {
	if value.IsZero() {
		return time.Now().UTC()
	}
	return value
}

func respondExpenseError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, expenses.ErrExpenseNotFound):
		respondError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, expenses.ErrApprovedReceiptRequired),
		errors.Is(err, expenses.ErrExpenseAlreadyPosted):
		respondError(w, http.StatusConflict, err.Error())
	case errors.Is(err, expenses.ErrInvalidStatusTransition),
		errors.Is(err, expenses.ErrExpenseAccountingInvalid):
		respondError(w, http.StatusBadRequest, err.Error())
	default:
		respondError(w, http.StatusBadRequest, err.Error())
	}
}
