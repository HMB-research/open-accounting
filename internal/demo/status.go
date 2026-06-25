package demo

import (
	"context"
	"fmt"
	"strings"

	"github.com/HMB-research/open-accounting/internal/database"
	"github.com/jackc/pgx/v5/pgxpool"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// StatusResponse represents the demo data status.
type StatusResponse struct {
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

// EntityStatus represents count and key identifiers for an entity type.
type EntityStatus struct {
	Count int      `json:"count"`
	Keys  []string `json:"keys"`
}

// StatusReader reads demo tenant status behind a reusable service boundary.
type StatusReader interface {
	ReadDemoStatus(ctx context.Context, schema string, userNum int) (StatusResponse, error)
}

type gormStatusReader struct {
	db *gorm.DB
}

// NewStatusReader creates a GORM-backed demo status reader.
func NewStatusReader(pool *pgxpool.Pool) (StatusReader, error) {
	if pool == nil {
		return nil, fmt.Errorf("database pool is not configured")
	}
	gormDB, err := newGormDBFromPool(context.Background(), pool)
	if err != nil {
		return nil, fmt.Errorf("create demo status GORM reader: %w", err)
	}
	return &gormStatusReader{db: gormDB}, nil
}

func (r *gormStatusReader) ReadDemoStatus(ctx context.Context, schema string, userNum int) (StatusResponse, error) {
	response := StatusResponse{User: userNum}
	if r == nil || r.db == nil {
		return response, fmt.Errorf("demo status reader is not configured")
	}

	steps := []struct {
		label  string
		read   func() (EntityStatus, error)
		assign func(EntityStatus)
	}{
		{
			label:  "accounts",
			read:   func() (EntityStatus, error) { return r.entityStatus(ctx, schema, "accounts", "name") },
			assign: func(status EntityStatus) { response.Accounts = status },
		},
		{
			label:  "contacts",
			read:   func() (EntityStatus, error) { return r.entityStatus(ctx, schema, "contacts", "name") },
			assign: func(status EntityStatus) { response.Contacts = status },
		},
		{
			label:  "invoices",
			read:   func() (EntityStatus, error) { return r.entityStatus(ctx, schema, "invoices", "invoice_number") },
			assign: func(status EntityStatus) { response.Invoices = status },
		},
		{
			label:  "employees",
			read:   func() (EntityStatus, error) { return r.employeeStatus(ctx, schema) },
			assign: func(status EntityStatus) { response.Employees = status },
		},
		{
			label:  "payments",
			read:   func() (EntityStatus, error) { return r.entityStatus(ctx, schema, "payments", "payment_number") },
			assign: func(status EntityStatus) { response.Payments = status },
		},
		{
			label:  "journal entries",
			read:   func() (EntityStatus, error) { return r.entityStatus(ctx, schema, "journal_entries", "entry_number") },
			assign: func(status EntityStatus) { response.JournalEntries = status },
		},
		{
			label:  "bank accounts",
			read:   func() (EntityStatus, error) { return r.entityStatus(ctx, schema, "bank_accounts", "name") },
			assign: func(status EntityStatus) { response.BankAccounts = status },
		},
		{
			label:  "recurring invoices",
			read:   func() (EntityStatus, error) { return r.entityStatus(ctx, schema, "recurring_invoices", "name") },
			assign: func(status EntityStatus) { response.RecurringInvoices = status },
		},
		{
			label:  "payroll runs",
			read:   func() (EntityStatus, error) { return r.periodStatus(ctx, schema, "payroll_runs") },
			assign: func(status EntityStatus) { response.PayrollRuns = status },
		},
		{
			label:  "TSD declarations",
			read:   func() (EntityStatus, error) { return r.periodStatus(ctx, schema, "tsd_declarations") },
			assign: func(status EntityStatus) { response.TsdDeclarations = status },
		},
	}

	for _, step := range steps {
		status, err := step.read()
		if err != nil {
			return response, fmt.Errorf("read %s status: %w", step.label, err)
		}
		step.assign(status)
	}

	return response, nil
}

func (r *gormStatusReader) entityStatus(ctx context.Context, schema, table, keyColumn string) (EntityStatus, error) {
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

func (r *gormStatusReader) employeeStatus(ctx context.Context, schema string) (EntityStatus, error) {
	db, err := r.tenantTable(ctx, schema, "employees")
	if err != nil {
		return EntityStatus{}, err
	}

	var count int64
	if err := db.Count(&count).Error; err != nil {
		return EntityStatus{}, nil
	}

	var rows []employeeStatusRow
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

func (r *gormStatusReader) periodStatus(ctx context.Context, schema, table string) (EntityStatus, error) {
	db, err := r.tenantTable(ctx, schema, table)
	if err != nil {
		return EntityStatus{}, err
	}

	var count int64
	if err := db.Count(&count).Error; err != nil {
		return EntityStatus{}, nil
	}

	var rows []periodStatusRow
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

func (r *gormStatusReader) tenantTable(ctx context.Context, schema, table string) (*gorm.DB, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("demo status reader is not configured")
	}
	qualifiedTable, err := database.QualifiedTable(schema, table)
	if err != nil {
		return nil, err
	}
	return r.db.WithContext(ctx).Table(qualifiedTable), nil
}

type employeeStatusRow struct {
	FirstName string `gorm:"column:first_name"`
	LastName  string `gorm:"column:last_name"`
}

type periodStatusRow struct {
	PeriodYear  int `gorm:"column:period_year"`
	PeriodMonth int `gorm:"column:period_month"`
}
