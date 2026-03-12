package main

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/rs/zerolog/log"
)

// DemoStatusResponse represents the demo data status
type DemoStatusResponse struct {
	User              int          `json:"user"`
	Accounts          EntityStatus `json:"accounts"`
	Contacts          EntityStatus `json:"contacts"`
	Invoices          EntityStatus `json:"invoices"`
	Employees         EntityStatus `json:"employees"`
	Payments          EntityStatus `json:"payments"`
	JournalEntries    EntityStatus `json:"journalEntries"`
	BankAccounts      EntityStatus `json:"bankAccounts"`
	RecurringInvoices EntityStatus `json:"recurringInvoices"`
	PayrollRuns       EntityStatus `json:"payrollRuns"`
	TsdDeclarations   EntityStatus `json:"tsdDeclarations"`
}

// EntityStatus represents count and key identifiers for an entity type
type EntityStatus struct {
	Count int      `json:"count"`
	Keys  []string `json:"keys"`
}

// DemoReset resets the demo database to initial state
// @Summary Reset demo database
// @Description Reset the demo database to initial state (requires DEMO_RESET_SECRET)
// @Tags Demo
// @Accept json
// @Produce json
// @Param X-Demo-Secret header string true "Demo reset secret key"
// @Success 200 {object} object{status=string,message=string}
// @Failure 401 {object} object{error=string}
// @Failure 403 {object} object{error=string}
// @Failure 500 {object} object{error=string}
// @Router /api/demo/reset [post]
func (h *Handlers) DemoReset(w http.ResponseWriter, r *http.Request) {
	log.Info().Msg("Demo reset requested")

	secret, ok := validateDemoRequest(w, r, true)
	if !ok {
		return
	}

	_ = secret
	ctx := r.Context()
	selectedUsers, userNums, err := demoUsersForSelection(r.URL.Query().Get("user"))
	if err != nil {
		log.Warn().Str("user", r.URL.Query().Get("user")).Msg("Demo reset rejected: invalid user parameter")
		respondError(w, http.StatusBadRequest, "Invalid user parameter. Must be 1, 2, 3, or 4")
		return
	}

	if len(userNums) == len(demoUsers) {
		log.Info().Msg("Demo reset: resetting all users")
	} else {
		log.Info().Int("user", userNums[0]).Msg("Demo reset: resetting single user")
	}

	for _, demoUser := range selectedUsers {
		log.Info().Str("schema", demoUser.schema).Msg("Demo reset: dropping tenant schema")
		_, err := h.pool.Exec(ctx, fmt.Sprintf("DROP SCHEMA IF EXISTS %s CASCADE", demoUser.schema))
		if err != nil {
			log.Error().Err(err).Str("schema", demoUser.schema).Msg("Demo reset failed: drop schema")
			respondError(w, http.StatusInternalServerError, "Failed to drop tenant schema: "+err.Error())
			return
		}
	}

	for _, demoUser := range selectedUsers {
		log.Info().Str("slug", demoUser.slug).Msg("Demo reset: cleaning tenant_users by slug")
		_, err := h.pool.Exec(ctx, "DELETE FROM tenant_users WHERE tenant_id IN (SELECT id FROM tenants WHERE slug = $1)", demoUser.slug)
		if err != nil {
			log.Error().Err(err).Msg("Demo reset failed: clean tenant_users")
			respondError(w, http.StatusInternalServerError, "Failed to clean tenant_users: "+err.Error())
			return
		}

		log.Info().Str("slug", demoUser.slug).Msg("Demo reset: cleaning tenants by slug")
		_, err = h.pool.Exec(ctx, "DELETE FROM tenants WHERE slug = $1", demoUser.slug)
		if err != nil {
			log.Error().Err(err).Msg("Demo reset failed: clean tenants")
			respondError(w, http.StatusInternalServerError, "Failed to clean tenants: "+err.Error())
			return
		}

		log.Info().Str("email", demoUser.email).Msg("Demo reset: cleaning users by email")
		_, err = h.pool.Exec(ctx, "DELETE FROM users WHERE email = $1", demoUser.email)
		if err != nil {
			log.Error().Err(err).Msg("Demo reset failed: clean users")
			respondError(w, http.StatusInternalServerError, "Failed to clean users: "+err.Error())
			return
		}
	}

	log.Info().Ints("users", userNums).Msg("Demo reset: seeding demo data")
	seedSQL := getDemoSeedSQLForUsers(userNums)
	_, err = h.pool.Exec(ctx, seedSQL)
	if err != nil {
		log.Error().Err(err).Str("sql_preview", seedSQL[:500]).Msg("Demo reset failed: seed data")
		respondError(w, http.StatusInternalServerError, "Failed to seed demo data: "+err.Error())
		return
	}

	log.Info().Msg("Demo reset completed successfully")
	respondJSON(w, http.StatusOK, map[string]string{
		"status":  "success",
		"message": "Demo database reset successfully",
	})
}

// DemoStatus returns counts and key identifiers for demo data verification
// @Summary Get demo data status
// @Description Get counts and key identifiers for demo data verification
// @Tags Demo
// @Produce json
// @Param user query int true "Demo user number (1-3)"
// @Param X-Demo-Secret header string true "Demo secret key"
// @Success 200 {object} DemoStatusResponse
// @Failure 400 {object} object{error=string}
// @Failure 401 {object} object{error=string}
// @Failure 403 {object} object{error=string}
// @Router /api/demo/status [get]
func (h *Handlers) DemoStatus(w http.ResponseWriter, r *http.Request) {
	_, ok := validateDemoRequest(w, r, false)
	if !ok {
		return
	}

	userParam := r.URL.Query().Get("user")
	if userParam == "" {
		respondError(w, http.StatusBadRequest, "User parameter is required")
		return
	}

	userNum, err := parseDemoUserNumber(userParam)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid user parameter. Must be 1, 2, 3, or 4")
		return
	}

	demoUser, _ := demoUserByNumber(userNum)
	ctx := r.Context()
	response := DemoStatusResponse{User: userNum}

	response.Accounts = h.getEntityStatus(ctx, demoUser.schema, "accounts", "name")
	response.Contacts = h.getEntityStatus(ctx, demoUser.schema, "contacts", "name")
	response.Invoices = h.getEntityStatus(ctx, demoUser.schema, "invoices", "invoice_number")
	response.Employees = h.getEntityStatusConcat(ctx, demoUser.schema, "employees", "first_name", "last_name")
	response.Payments = h.getEntityStatus(ctx, demoUser.schema, "payments", "payment_number")
	response.JournalEntries = h.getEntityStatus(ctx, demoUser.schema, "journal_entries", "entry_number")
	response.BankAccounts = h.getEntityStatus(ctx, demoUser.schema, "bank_accounts", "name")
	response.RecurringInvoices = h.getEntityStatus(ctx, demoUser.schema, "recurring_invoices", "name")
	response.PayrollRuns = h.getEntityStatusPeriod(ctx, demoUser.schema, "payroll_runs")
	response.TsdDeclarations = h.getEntityStatusPeriod(ctx, demoUser.schema, "tsd_declarations")

	respondJSON(w, http.StatusOK, response)
}

func validateDemoRequest(w http.ResponseWriter, r *http.Request, allowQuerySecret bool) (string, bool) {
	if os.Getenv("DEMO_MODE") != "true" {
		respondError(w, http.StatusForbidden, "Demo mode is not enabled")
		return "", false
	}

	secret := os.Getenv("DEMO_RESET_SECRET")
	if secret == "" {
		message := "Demo status not configured"
		if allowQuerySecret {
			message = "Demo reset not configured"
		}
		respondError(w, http.StatusForbidden, message)
		return "", false
	}

	providedSecret := r.Header.Get("X-Demo-Secret")
	if allowQuerySecret && providedSecret == "" {
		providedSecret = r.URL.Query().Get("secret")
	}
	if providedSecret != secret {
		respondError(w, http.StatusUnauthorized, "Invalid or missing secret key")
		return "", false
	}

	return secret, true
}

func (h *Handlers) getEntityStatus(ctx context.Context, schema, table, keyColumn string) EntityStatus {
	var count int
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s.%s", schema, table)
	_ = h.pool.QueryRow(ctx, query).Scan(&count)

	var keys []string
	keysQuery := fmt.Sprintf("SELECT %s FROM %s.%s ORDER BY %s LIMIT 10", keyColumn, schema, table, keyColumn)
	rows, _ := h.pool.Query(ctx, keysQuery)
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var key string
			if rows.Scan(&key) == nil {
				keys = append(keys, key)
			}
		}
	}

	return EntityStatus{Count: count, Keys: keys}
}

func (h *Handlers) getEntityStatusConcat(ctx context.Context, schema, table, col1, col2 string) EntityStatus {
	var count int
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s.%s", schema, table)
	_ = h.pool.QueryRow(ctx, query).Scan(&count)

	var keys []string
	keysQuery := fmt.Sprintf("SELECT %s || ' ' || %s FROM %s.%s ORDER BY %s LIMIT 10", col1, col2, schema, table, col1)
	rows, _ := h.pool.Query(ctx, keysQuery)
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var key string
			if rows.Scan(&key) == nil {
				keys = append(keys, key)
			}
		}
	}

	return EntityStatus{Count: count, Keys: keys}
}

func (h *Handlers) getEntityStatusPeriod(ctx context.Context, schema, table string) EntityStatus {
	var count int
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s.%s", schema, table)
	_ = h.pool.QueryRow(ctx, query).Scan(&count)

	var keys []string
	keysQuery := fmt.Sprintf("SELECT period_year || '-' || LPAD(period_month::text, 2, '0') FROM %s.%s ORDER BY period_year, period_month LIMIT 10", schema, table)
	rows, _ := h.pool.Query(ctx, keysQuery)
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var key string
			if rows.Scan(&key) == nil {
				keys = append(keys, key)
			}
		}
	}

	return EntityStatus{Count: count, Keys: keys}
}
