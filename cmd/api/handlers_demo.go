package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/HMB-research/open-accounting/internal/database"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Keep this aligned with the test schema lifecycle lock so demo reset DDL does
// not deadlock against concurrent test processes using the same database.
const demoResetAdvisoryLockKey = 12345678

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

	if h.pool == nil {
		log.Error().Msg("Demo reset failed: database pool is not configured")
		respondError(w, http.StatusInternalServerError, "Failed to acquire database connection")
		return
	}

	conn, err := h.pool.Acquire(ctx)
	if err != nil {
		log.Error().Err(err).Msg("Demo reset failed: acquire connection")
		respondError(w, http.StatusInternalServerError, "Failed to acquire database connection")
		return
	}
	defer conn.Release()

	if _, err := conn.Exec(ctx, "SELECT pg_advisory_lock($1)", demoResetAdvisoryLockKey); err != nil {
		log.Error().Err(err).Msg("Demo reset failed: acquire advisory lock")
		respondError(w, http.StatusInternalServerError, "Failed to acquire demo reset lock")
		return
	}
	defer func() {
		unlockCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = conn.Exec(unlockCtx, "SELECT pg_advisory_unlock($1)", demoResetAdvisoryLockKey)
	}()

	for _, demoUser := range selectedUsers {
		log.Info().Str("schema", demoUser.schema).Msg("Demo reset: dropping tenant schema")
		_, err := conn.Exec(ctx, fmt.Sprintf("DROP SCHEMA IF EXISTS %s CASCADE", demoUser.schema))
		if err != nil {
			log.Error().Err(err).Str("schema", demoUser.schema).Msg("Demo reset failed: drop schema")
			respondError(w, http.StatusInternalServerError, "Failed to drop tenant schema: "+err.Error())
			return
		}
	}

	for _, demoUser := range selectedUsers {
		log.Info().Str("slug", demoUser.slug).Msg("Demo reset: cleaning tenant_users by slug")
		_, err := conn.Exec(ctx, "DELETE FROM tenant_users WHERE tenant_id IN (SELECT id FROM tenants WHERE slug = $1)", demoUser.slug)
		if err != nil {
			log.Error().Err(err).Msg("Demo reset failed: clean tenant_users")
			respondError(w, http.StatusInternalServerError, "Failed to clean tenant_users: "+err.Error())
			return
		}

		log.Info().Str("slug", demoUser.slug).Msg("Demo reset: cleaning tenants by slug")
		_, err = conn.Exec(ctx, "DELETE FROM tenants WHERE slug = $1", demoUser.slug)
		if err != nil {
			log.Error().Err(err).Msg("Demo reset failed: clean tenants")
			respondError(w, http.StatusInternalServerError, "Failed to clean tenants: "+err.Error())
			return
		}

		log.Info().Str("email", demoUser.email).Msg("Demo reset: cleaning users by email")
		_, err = conn.Exec(ctx, "DELETE FROM users WHERE email = $1", demoUser.email)
		if err != nil {
			log.Error().Err(err).Msg("Demo reset failed: clean users")
			respondError(w, http.StatusInternalServerError, "Failed to clean users: "+err.Error())
			return
		}
	}

	log.Info().Ints("users", userNums).Msg("Demo reset: seeding demo data")
	seedSQL := getDemoSeedSQLForUsers(userNums)
	_, err = conn.Exec(ctx, seedSQL)
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

	statusReader, err := h.getDemoStatusReader()
	if err != nil {
		log.Error().Err(err).Msg("Demo status failed: status reader unavailable")
		respondError(w, http.StatusInternalServerError, "Failed to read demo status")
		return
	}

	response, err := statusReader.ReadDemoStatus(ctx, demoUser.schema, userNum)
	if err != nil {
		log.Error().Err(err).Str("schema", demoUser.schema).Msg("Demo status failed")
		respondError(w, http.StatusInternalServerError, "Failed to read demo status")
		return
	}

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

func (h *Handlers) getDemoStatusReader() (demoStatusReader, error) {
	if h.demoStatusReader != nil {
		return h.demoStatusReader, nil
	}
	if h.pool == nil {
		return nil, fmt.Errorf("database pool is not configured")
	}
	statusReader, err := newDemoStatusReader(h.pool)
	if err != nil {
		return nil, err
	}
	h.demoStatusReader = statusReader
	return statusReader, nil
}

type demoStatusReader interface {
	ReadDemoStatus(ctx context.Context, schema string, userNum int) (DemoStatusResponse, error)
}

type gormDemoStatusReader struct {
	db *gorm.DB
}

func newDemoStatusReader(pool *pgxpool.Pool) (*gormDemoStatusReader, error) {
	gormDB, err := database.NewGormDBFromPool(context.Background(), pool)
	if err != nil {
		return nil, fmt.Errorf("create demo status GORM reader: %w", err)
	}
	return &gormDemoStatusReader{db: gormDB}, nil
}

func (r *gormDemoStatusReader) ReadDemoStatus(ctx context.Context, schema string, userNum int) (DemoStatusResponse, error) {
	response := DemoStatusResponse{User: userNum}
	var err error

	if response.Accounts, err = r.entityStatus(ctx, schema, "accounts", "name"); err != nil {
		return response, fmt.Errorf("read accounts status: %w", err)
	}
	if response.Contacts, err = r.entityStatus(ctx, schema, "contacts", "name"); err != nil {
		return response, fmt.Errorf("read contacts status: %w", err)
	}
	if response.Invoices, err = r.entityStatus(ctx, schema, "invoices", "invoice_number"); err != nil {
		return response, fmt.Errorf("read invoices status: %w", err)
	}
	if response.Employees, err = r.employeeStatus(ctx, schema); err != nil {
		return response, fmt.Errorf("read employees status: %w", err)
	}
	if response.Payments, err = r.entityStatus(ctx, schema, "payments", "payment_number"); err != nil {
		return response, fmt.Errorf("read payments status: %w", err)
	}
	if response.JournalEntries, err = r.entityStatus(ctx, schema, "journal_entries", "entry_number"); err != nil {
		return response, fmt.Errorf("read journal entries status: %w", err)
	}
	if response.BankAccounts, err = r.entityStatus(ctx, schema, "bank_accounts", "name"); err != nil {
		return response, fmt.Errorf("read bank accounts status: %w", err)
	}
	if response.RecurringInvoices, err = r.entityStatus(ctx, schema, "recurring_invoices", "name"); err != nil {
		return response, fmt.Errorf("read recurring invoices status: %w", err)
	}
	if response.PayrollRuns, err = r.periodStatus(ctx, schema, "payroll_runs"); err != nil {
		return response, fmt.Errorf("read payroll runs status: %w", err)
	}
	if response.TsdDeclarations, err = r.periodStatus(ctx, schema, "tsd_declarations"); err != nil {
		return response, fmt.Errorf("read TSD declarations status: %w", err)
	}

	return response, nil
}

func (r *gormDemoStatusReader) entityStatus(ctx context.Context, schema, table, keyColumn string) (EntityStatus, error) {
	db, err := r.tenantTable(ctx, schema, table)
	if err != nil {
		return EntityStatus{}, err
	}

	var count int64
	if err := db.Count(&count).Error; err != nil {
		return EntityStatus{}, nil
	}

	var keys []string
	if err := db.Session(&gorm.Session{}).
		Order(clause.OrderByColumn{Column: clause.Column{Name: keyColumn}}).
		Limit(10).
		Pluck(keyColumn, &keys).Error; err != nil {
		return EntityStatus{Count: int(count)}, nil
	}

	return EntityStatus{Count: int(count), Keys: keys}, nil
}

func (r *gormDemoStatusReader) employeeStatus(ctx context.Context, schema string) (EntityStatus, error) {
	db, err := r.tenantTable(ctx, schema, "employees")
	if err != nil {
		return EntityStatus{}, err
	}

	var count int64
	if err := db.Count(&count).Error; err != nil {
		return EntityStatus{}, nil
	}

	var rows []demoEmployeeStatusRow
	if err := db.Session(&gorm.Session{}).
		Order(clause.OrderByColumn{Column: clause.Column{Name: "first_name"}}).
		Limit(10).
		Find(&rows).Error; err != nil {
		return EntityStatus{Count: int(count)}, nil
	}

	keys := make([]string, 0, len(rows))
	for _, row := range rows {
		keys = append(keys, strings.TrimSpace(row.FirstName+" "+row.LastName))
	}

	return EntityStatus{Count: int(count), Keys: keys}, nil
}

func (r *gormDemoStatusReader) periodStatus(ctx context.Context, schema, table string) (EntityStatus, error) {
	db, err := r.tenantTable(ctx, schema, table)
	if err != nil {
		return EntityStatus{}, err
	}

	var count int64
	if err := db.Count(&count).Error; err != nil {
		return EntityStatus{}, nil
	}

	var rows []demoPeriodStatusRow
	if err := db.Session(&gorm.Session{}).
		Order(clause.OrderByColumn{Column: clause.Column{Name: "period_year"}}).
		Order(clause.OrderByColumn{Column: clause.Column{Name: "period_month"}}).
		Limit(10).
		Find(&rows).Error; err != nil {
		return EntityStatus{Count: int(count)}, nil
	}

	keys := make([]string, 0, len(rows))
	for _, row := range rows {
		keys = append(keys, fmt.Sprintf("%d-%02d", row.PeriodYear, row.PeriodMonth))
	}

	return EntityStatus{Count: int(count), Keys: keys}, nil
}

func (r *gormDemoStatusReader) tenantTable(ctx context.Context, schema, table string) (*gorm.DB, error) {
	qualifiedTable, err := database.QualifiedTable(schema, table)
	if err != nil {
		return nil, err
	}
	return r.db.WithContext(ctx).Table(qualifiedTable), nil
}

type demoEmployeeStatusRow struct {
	FirstName string `gorm:"column:first_name"`
	LastName  string `gorm:"column:last_name"`
}

type demoPeriodStatusRow struct {
	PeriodYear  int `gorm:"column:period_year"`
	PeriodMonth int `gorm:"column:period_month"`
}
