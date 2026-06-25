package payroll

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/HMB-research/open-accounting/internal/models"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type payrollDryRunConnPool struct{}

func (payrollDryRunConnPool) PrepareContext(context.Context, string) (*sql.Stmt, error) {
	return nil, errors.New("dry-run payroll tests should not prepare statements")
}

func (payrollDryRunConnPool) ExecContext(context.Context, string, ...interface{}) (sql.Result, error) {
	return nil, errors.New("dry-run payroll tests should not execute statements")
}

func (payrollDryRunConnPool) QueryContext(context.Context, string, ...interface{}) (*sql.Rows, error) {
	return nil, errors.New("dry-run payroll tests should not query rows")
}

func (payrollDryRunConnPool) QueryRowContext(context.Context, string, ...interface{}) *sql.Row {
	return nil
}

func (payrollDryRunConnPool) BeginTx(context.Context, *sql.TxOptions) (gorm.ConnPool, error) {
	return &payrollDryRunTx{}, nil
}

type payrollDryRunTx struct {
	payrollDryRunConnPool
}

func (*payrollDryRunTx) Commit() error {
	return nil
}

func (*payrollDryRunTx) Rollback() error {
	return nil
}

type payrollDryRunDBOption func(t *testing.T, db *gorm.DB)

type payrollDryRunFixtures struct {
	employees           []models.Employee
	employeeIndex       int
	salaryComponents    []models.SalaryComponent
	payrollRuns         []models.PayrollRun
	payrollRunIndex     int
	tsdDeclarations     []models.TSDDeclaration
	tsdDeclarationIndex int
	tsdRows             []models.TSDRow
	absenceTypes        []models.AbsenceType
	absenceTypeIndex    int
	leaveBalances       []models.LeaveBalance
	leaveBalanceIndex   int
	leaveRecords        []models.LeaveRecord
	leaveRecordIndex    int
}

var payrollDryRunCallbackID uint64

func newPayrollDryRunDB(t *testing.T, opts ...payrollDryRunDBOption) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(postgres.New(postgres.Config{Conn: payrollDryRunConnPool{}}), &gorm.Config{
		DisableAutomaticPing:   true,
		DryRun:                 true,
		Logger:                 logger.Default.LogMode(logger.Silent),
		SkipDefaultTransaction: true,
	})
	require.NoError(t, err)

	for _, opt := range opts {
		opt(t, db)
	}
	return db
}

func withPayrollDryRunFixtures(fixtures payrollDryRunFixtures) payrollDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		err := db.Callback().Query().After("gorm:query").Register(payrollDryRunCallbackName("query_fixtures"), func(tx *gorm.DB) {
			populatePayrollDryRunQueryDest(tx, tx.Statement.Dest, &fixtures)
		})
		require.NoError(t, err)
	}
}

func withPayrollDryRunQueryErrors(queryErrors ...error) payrollDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		var index int
		err := db.Callback().Query().Before("gorm:query").Register(payrollDryRunCallbackName("query_error"), func(tx *gorm.DB) {
			if len(queryErrors) == 0 {
				return
			}
			errIndex := index
			if errIndex >= len(queryErrors) {
				errIndex = len(queryErrors) - 1
			}
			index++
			if queryErrors[errIndex] != nil {
				tx.AddError(queryErrors[errIndex])
			}
		})
		require.NoError(t, err)
	}
}

func withPayrollDryRunRowErrors(rowErrors ...error) payrollDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		var index int
		err := db.Callback().Row().After("gorm:row").Register(payrollDryRunCallbackName("row_error"), func(tx *gorm.DB) {
			if len(rowErrors) == 0 {
				return
			}
			errIndex := index
			if errIndex >= len(rowErrors) {
				errIndex = len(rowErrors) - 1
			}
			index++
			if rowErrors[errIndex] != nil {
				tx.AddError(rowErrors[errIndex])
			}
		})
		require.NoError(t, err)
	}
}

func withPayrollDryRunScanRows(match string, rows *sql.Rows) payrollDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		err := db.Callback().Row().After("gorm:row").Register(payrollDryRunCallbackName("scan_rows"), func(tx *gorm.DB) {
			if rows == nil || !strings.Contains(tx.Statement.SQL.String(), match) {
				return
			}
			tx.Statement.Dest = rows
			tx.Error = nil
		})
		require.NoError(t, err)
	}
}

func withPayrollDryRunCreateError(expectedErr error) payrollDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		err := db.Callback().Create().Before("gorm:create").Register(payrollDryRunCallbackName("create_error"), func(tx *gorm.DB) {
			tx.AddError(expectedErr)
		})
		require.NoError(t, err)
	}
}

func withPayrollDryRunUpdateRows(rows ...int64) payrollDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		var index int
		err := db.Callback().Update().After("gorm:update").Register(payrollDryRunCallbackName("update_rows"), func(tx *gorm.DB) {
			rowCount := int64(0)
			if len(rows) > 0 {
				rowCount = rows[len(rows)-1]
				if index < len(rows) {
					rowCount = rows[index]
				}
				index++
			}
			tx.RowsAffected = rowCount
		})
		require.NoError(t, err)
	}
}

func withPayrollDryRunUpdateError(expectedErr error) payrollDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		err := db.Callback().Update().Before("gorm:update").Register(payrollDryRunCallbackName("update_error"), func(tx *gorm.DB) {
			tx.AddError(expectedErr)
		})
		require.NoError(t, err)
	}
}

func withPayrollDryRunDeleteError(expectedErr error) payrollDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		err := db.Callback().Delete().Before("gorm:delete").Register(payrollDryRunCallbackName("delete_error"), func(tx *gorm.DB) {
			tx.AddError(expectedErr)
		})
		require.NoError(t, err)
	}
}

func payrollDryRunCallbackName(suffix string) string {
	id := atomic.AddUint64(&payrollDryRunCallbackID, 1)
	return fmt.Sprintf("payroll_dryrun:%d:%s", id, suffix)
}

func populatePayrollDryRunQueryDest(tx *gorm.DB, dest any, fixtures *payrollDryRunFixtures) {
	switch typed := dest.(type) {
	case *models.Employee:
		if employee, ok := nextPayrollDryRunValue(fixtures.employees, &fixtures.employeeIndex); ok {
			*typed = employee
			tx.RowsAffected = 1
		}
	case *[]models.Employee:
		*typed = append([]models.Employee(nil), fixtures.employees...)
		tx.RowsAffected = int64(len(fixtures.employees))
	case *[]models.SalaryComponent:
		*typed = append([]models.SalaryComponent(nil), fixtures.salaryComponents...)
		tx.RowsAffected = int64(len(fixtures.salaryComponents))
	case *models.PayrollRun:
		if run, ok := nextPayrollDryRunValue(fixtures.payrollRuns, &fixtures.payrollRunIndex); ok {
			*typed = run
			tx.RowsAffected = 1
		}
	case *[]models.PayrollRun:
		*typed = append([]models.PayrollRun(nil), fixtures.payrollRuns...)
		tx.RowsAffected = int64(len(fixtures.payrollRuns))
	case *models.TSDDeclaration:
		if declaration, ok := nextPayrollDryRunValue(fixtures.tsdDeclarations, &fixtures.tsdDeclarationIndex); ok {
			*typed = declaration
			tx.RowsAffected = 1
		}
	case *[]models.TSDDeclaration:
		*typed = append([]models.TSDDeclaration(nil), fixtures.tsdDeclarations...)
		tx.RowsAffected = int64(len(fixtures.tsdDeclarations))
	case *[]models.TSDRow:
		*typed = append([]models.TSDRow(nil), fixtures.tsdRows...)
		tx.RowsAffected = int64(len(fixtures.tsdRows))
	case *models.AbsenceType:
		if absenceType, ok := nextPayrollDryRunValue(fixtures.absenceTypes, &fixtures.absenceTypeIndex); ok {
			*typed = absenceType
			tx.RowsAffected = 1
		}
	case *[]models.AbsenceType:
		*typed = append([]models.AbsenceType(nil), fixtures.absenceTypes...)
		tx.RowsAffected = int64(len(fixtures.absenceTypes))
	case *models.LeaveBalance:
		if balance, ok := nextPayrollDryRunValue(fixtures.leaveBalances, &fixtures.leaveBalanceIndex); ok {
			*typed = balance
			tx.RowsAffected = 1
		}
	case *models.LeaveRecord:
		if record, ok := nextPayrollDryRunValue(fixtures.leaveRecords, &fixtures.leaveRecordIndex); ok {
			*typed = record
			tx.RowsAffected = 1
		}
	}
}

func nextPayrollDryRunValue[T any](values []T, index *int) (T, bool) {
	var zero T
	if len(values) == 0 {
		return zero, false
	}
	if *index >= len(values) {
		return values[len(values)-1], true
	}
	value := values[*index]
	*index = *index + 1
	return value, true
}

type payrollDryRunRowsConnector struct {
	columns []string
	values  [][]driver.Value
}

func (c *payrollDryRunRowsConnector) Connect(context.Context) (driver.Conn, error) {
	return &payrollDryRunRowsConn{
		columns: append([]string(nil), c.columns...),
		values:  clonePayrollDryRunDriverValues(c.values),
	}, nil
}

func (*payrollDryRunRowsConnector) Driver() driver.Driver {
	return payrollDryRunRowsDriver{}
}

type payrollDryRunRowsDriver struct{}

func (payrollDryRunRowsDriver) Open(string) (driver.Conn, error) {
	return nil, errors.New("payroll dry-run rows driver requires a connector")
}

type payrollDryRunRowsConn struct {
	columns []string
	values  [][]driver.Value
}

func (*payrollDryRunRowsConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("payroll dry-run rows should not prepare statements")
}

func (*payrollDryRunRowsConn) Close() error {
	return nil
}

func (*payrollDryRunRowsConn) Begin() (driver.Tx, error) {
	return nil, errors.New("payroll dry-run rows should not begin transactions")
}

func (c *payrollDryRunRowsConn) QueryContext(context.Context, string, []driver.NamedValue) (driver.Rows, error) {
	return &payrollDryRunRows{
		columns: append([]string(nil), c.columns...),
		values:  clonePayrollDryRunDriverValues(c.values),
	}, nil
}

type payrollDryRunRows struct {
	columns []string
	values  [][]driver.Value
	index   int
}

func (r *payrollDryRunRows) Columns() []string {
	return append([]string(nil), r.columns...)
}

func (*payrollDryRunRows) Close() error {
	return nil
}

func (r *payrollDryRunRows) Next(dest []driver.Value) error {
	if r.index >= len(r.values) {
		return io.EOF
	}
	copy(dest, r.values[r.index])
	r.index++
	return nil
}

func payrollDryRunSQLRows(t *testing.T, columns []string, values [][]driver.Value) *sql.Rows {
	t.Helper()

	db := sql.OpenDB(&payrollDryRunRowsConnector{
		columns: columns,
		values:  values,
	})
	t.Cleanup(func() {
		_ = db.Close()
	})

	rows, err := db.QueryContext(context.Background(), "SELECT payroll_dryrun_rows")
	require.NoError(t, err)
	return rows
}

func clonePayrollDryRunDriverValues(values [][]driver.Value) [][]driver.Value {
	clone := make([][]driver.Value, len(values))
	for i := range values {
		clone[i] = append([]driver.Value(nil), values[i]...)
	}
	return clone
}

func TestMockAbsenceRepositoryGetAbsenceTypeByCode(t *testing.T) {
	ctx := context.Background()
	repo := NewMockAbsenceRepository()
	repo.AbsenceTypes["annual"] = &AbsenceType{
		ID:       "annual",
		TenantID: "tenant-1",
		Code:     "ANNUAL",
		Name:     "Annual leave",
		IsActive: true,
	}

	got, err := repo.GetAbsenceTypeByCode(ctx, "tenant_payroll", "tenant-1", "ANNUAL")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "annual", got.ID)

	missing, err := repo.GetAbsenceTypeByCode(ctx, "tenant_payroll", "tenant-2", "ANNUAL")
	assert.Nil(t, missing)
	assert.ErrorIs(t, err, ErrAbsenceTypeNotFound)

	expectedErr := errors.New("lookup failed")
	repo.GetAbsenceTypeByCodeErr = expectedErr
	missing, err = repo.GetAbsenceTypeByCode(ctx, "tenant_payroll", "tenant-1", "ANNUAL")
	assert.Nil(t, missing)
	assert.ErrorIs(t, err, expectedErr)
}

func TestPayrollGORMRepositoryDryRunCoreOperations(t *testing.T) {
	ctx := context.Background()
	schemaName := "tenant_payroll"
	tenantID := "tenant-1"
	now := time.Date(2026, time.June, 25, 10, 0, 0, 0, time.UTC)
	employee := payrollDryRunEmployeeModel(tenantID, now)
	component := payrollDryRunSalaryComponentModel(tenantID, employee.ID, now)
	run := payrollDryRunPayrollRunModel(tenantID, now)
	payslip := payrollDryRunPayslipModel(tenantID, run.ID, employee.ID, now)

	repo := NewGORMRepository(newPayrollDryRunDB(t,
		withPayrollDryRunFixtures(payrollDryRunFixtures{
			employees:        []models.Employee{employee},
			salaryComponents: []models.SalaryComponent{component},
			payrollRuns:      []models.PayrollRun{run},
		}),
		withPayrollDryRunUpdateRows(1, 1, 1),
	))

	called := false
	require.NoError(t, repo.WithTransaction(ctx, func(txRepo Repository) error {
		called = true
		return txRepo.CreateEmployee(ctx, schemaName, modelToEmployee(&employee))
	}))
	assert.True(t, called)

	require.NoError(t, repo.CreateEmployee(ctx, schemaName, modelToEmployee(&employee)))
	gotEmployee, err := repo.GetEmployee(ctx, schemaName, tenantID, employee.ID)
	require.NoError(t, err)
	assert.Equal(t, employee.ID, gotEmployee.ID)

	employees, err := repo.ListEmployees(ctx, schemaName, tenantID, true)
	require.NoError(t, err)
	require.Len(t, employees, 1)
	assert.Equal(t, "Mets", employees[0].LastName)

	employees, err = repo.ListEmployees(ctx, schemaName, tenantID, false)
	require.NoError(t, err)
	require.Len(t, employees, 1)

	employeeDomain := modelToEmployee(&employee)
	employeeDomain.LastName = "Kask"
	require.NoError(t, repo.UpdateEmployee(ctx, schemaName, employeeDomain))
	require.NoError(t, repo.EndCurrentBaseSalary(ctx, schemaName, tenantID, employee.ID, now))

	componentDomain := modelToSalaryComponent(&component)
	require.NoError(t, repo.CreateSalaryComponent(ctx, schemaName, componentDomain))
	components, err := repo.ListSalaryComponents(ctx, schemaName, tenantID, employee.ID, &now)
	require.NoError(t, err)
	require.Len(t, components, 1)
	assert.Equal(t, SalaryComponentBaseSalary, components[0].ComponentType)

	components, err = repo.ListSalaryComponents(ctx, schemaName, tenantID, employee.ID, nil)
	require.NoError(t, err)
	require.Len(t, components, 1)

	salaryRepo := NewGORMRepository(newPayrollDryRunDB(t,
		withPayrollDryRunScanRows("salary_components", payrollDryRunSQLRows(t, []string{"total"}, [][]driver.Value{{"2500"}})),
	))
	salary, err := salaryRepo.GetCurrentSalary(ctx, schemaName, tenantID, employee.ID)
	require.NoError(t, err)
	assert.True(t, salary.Equal(decimal.NewFromInt(2500)))

	require.NoError(t, repo.CreatePayrollRun(ctx, schemaName, modelToPayrollRun(&run)))
	gotRun, err := repo.GetPayrollRun(ctx, schemaName, tenantID, run.ID)
	require.NoError(t, err)
	assert.Equal(t, PayrollCalculated, gotRun.Status)

	runs, err := repo.ListPayrollRuns(ctx, schemaName, tenantID, 2026)
	require.NoError(t, err)
	require.Len(t, runs, 1)
	runs, err = repo.ListPayrollRuns(ctx, schemaName, tenantID, 0)
	require.NoError(t, err)
	require.Len(t, runs, 1)

	runDomain := modelToPayrollRun(&run)
	runDomain.TotalGross = decimal.NewFromInt(2600)
	require.NoError(t, repo.UpdatePayrollRun(ctx, schemaName, runDomain))
	require.NoError(t, repo.ApprovePayrollRun(ctx, schemaName, tenantID, run.ID, "approver-1"))
	require.NoError(t, repo.DeletePayslipsByRunID(ctx, schemaName, run.ID))
	require.NoError(t, repo.CreatePayslip(ctx, schemaName, modelToPayslip(&payslip)))
}

func TestPayrollGORMRepositoryDryRunTSDOperations(t *testing.T) {
	ctx := context.Background()
	schemaName := "tenant_payroll"
	tenantID := "tenant-1"
	now := time.Date(2026, time.June, 25, 11, 0, 0, 0, time.UTC)
	employee := payrollDryRunEmployeeModel(tenantID, now)
	run := payrollDryRunPayrollRunModel(tenantID, now)
	payslip := payrollDryRunPayslipModel(tenantID, run.ID, employee.ID, now)
	declaration := payrollDryRunTSDDeclarationModel(tenantID, run.ID, now)
	row := payrollDryRunTSDRowModel(tenantID, declaration.ID, employee.ID, now)

	payslipRepo := NewGORMRepository(newPayrollDryRunDB(t,
		withPayrollDryRunScanRows("payslips", payrollDryRunSQLRows(t, payrollDryRunPayslipWithEmployeeColumns(), [][]driver.Value{
			payrollDryRunPayslipWithEmployeeRow(payslip, "Mari", "Mets", "49001010010", "mari@example.com"),
		})),
	))
	payslips, err := payslipRepo.GetPayslipsWithEmployees(ctx, schemaName, tenantID, run.ID)
	require.NoError(t, err)
	require.Len(t, payslips, 1)
	require.NotNil(t, payslips[0].Employee)
	assert.Equal(t, "Mari", payslips[0].Employee.FirstName)
	assert.True(t, payslips[0].GrossSalary.Equal(decimal.NewFromInt(2500)))

	repo := NewGORMRepository(newPayrollDryRunDB(t,
		withPayrollDryRunFixtures(payrollDryRunFixtures{
			tsdDeclarations: []models.TSDDeclaration{declaration},
			tsdRows:         []models.TSDRow{row},
		}),
		withPayrollDryRunUpdateRows(1, 1),
	))

	require.NoError(t, repo.DeleteTSDByPeriod(ctx, schemaName, tenantID, 2026, 6))
	require.NoError(t, repo.CreateTSDDeclaration(ctx, schemaName, modelToTSDDeclaration(&declaration)))
	require.NoError(t, repo.CreateTSDRows(ctx, schemaName, []TSDRow{*modelToTSDRow(&row)}))
	require.NoError(t, repo.CreateTSDRows(ctx, schemaName, nil))

	gotDeclaration, err := repo.GetTSD(ctx, schemaName, tenantID, 2026, 6)
	require.NoError(t, err)
	require.NotNil(t, gotDeclaration)
	require.Len(t, gotDeclaration.Rows, 1)
	assert.Equal(t, row.ID, gotDeclaration.Rows[0].ID)

	rows, err := repo.GetTSDRows(ctx, schemaName, tenantID, declaration.ID)
	require.NoError(t, err)
	require.Len(t, rows, 1)

	declarations, err := repo.ListTSD(ctx, schemaName, tenantID, TSDListFilter{Year: 2026, Month: 6})
	require.NoError(t, err)
	require.Len(t, declarations, 1)
	declarations, err = repo.ListTSD(ctx, schemaName, tenantID, TSDListFilter{})
	require.NoError(t, err)
	require.Len(t, declarations, 1)

	require.NoError(t, repo.MarkTSDSubmitted(ctx, schemaName, tenantID, declaration.ID, "EMTA-REF-1", now))
	require.NoError(t, repo.UpdateTSDStatus(ctx, schemaName, tenantID, declaration.ID, TSDAccepted, now.Add(time.Hour)))
}

func TestPayrollGORMRepositoryDryRunErrors(t *testing.T) {
	ctx := context.Background()
	schemaName := "tenant_payroll"
	tenantID := "tenant-1"
	now := time.Date(2026, time.June, 25, 12, 0, 0, 0, time.UTC)
	employee := payrollDryRunEmployeeModel(tenantID, now)
	run := payrollDryRunPayrollRunModel(tenantID, now)
	declaration := payrollDryRunTSDDeclarationModel(tenantID, run.ID, now)
	expectedErr := errors.New("dry-run failed")

	t.Run("invalid schema", func(t *testing.T) {
		repo := NewGORMRepository(newPayrollDryRunDB(t))
		assert.ErrorContains(t, repo.CreateEmployee(ctx, "tenant-payroll", modelToEmployee(&employee)), "invalid")
		_, err := repo.GetPayslipsWithEmployees(ctx, "tenant-payroll", tenantID, run.ID)
		assert.ErrorContains(t, err, "invalid")
	})

	t.Run("create errors", func(t *testing.T) {
		repo := NewGORMRepository(newPayrollDryRunDB(t, withPayrollDryRunCreateError(expectedErr)))
		component := payrollDryRunSalaryComponentModel(tenantID, employee.ID, now)
		payslip := payrollDryRunPayslipModel(tenantID, run.ID, employee.ID, now)
		row := payrollDryRunTSDRowModel(tenantID, declaration.ID, employee.ID, now)
		assert.ErrorIs(t, repo.CreateEmployee(ctx, schemaName, modelToEmployee(&employee)), expectedErr)
		assert.ErrorIs(t, repo.CreateSalaryComponent(ctx, schemaName, modelToSalaryComponent(&component)), expectedErr)
		assert.ErrorIs(t, repo.CreatePayrollRun(ctx, schemaName, modelToPayrollRun(&run)), expectedErr)
		assert.ErrorIs(t, repo.CreatePayslip(ctx, schemaName, modelToPayslip(&payslip)), expectedErr)
		assert.ErrorIs(t, repo.CreateTSDDeclaration(ctx, schemaName, modelToTSDDeclaration(&declaration)), expectedErr)
		assert.ErrorIs(t, repo.CreateTSDRows(ctx, schemaName, []TSDRow{*modelToTSDRow(&row)}), expectedErr)
	})

	t.Run("get not found and query errors", func(t *testing.T) {
		repo := NewGORMRepository(newPayrollDryRunDB(t, withPayrollDryRunQueryErrors(gorm.ErrRecordNotFound)))
		gotEmployee, err := repo.GetEmployee(ctx, schemaName, tenantID, employee.ID)
		assert.Nil(t, gotEmployee)
		assert.ErrorIs(t, err, ErrEmployeeNotFound)
		gotRun, err := repo.GetPayrollRun(ctx, schemaName, tenantID, run.ID)
		assert.Nil(t, gotRun)
		assert.ErrorIs(t, err, ErrPayrollRunNotFound)
		gotTSD, err := repo.GetTSD(ctx, schemaName, tenantID, 2026, 6)
		assert.Nil(t, gotTSD)
		assert.ErrorIs(t, err, ErrTSDDeclarationNotFound)

		repo = NewGORMRepository(newPayrollDryRunDB(t, withPayrollDryRunQueryErrors(expectedErr)))
		gotEmployee, err = repo.GetEmployee(ctx, schemaName, tenantID, employee.ID)
		assert.Nil(t, gotEmployee)
		assert.ErrorIs(t, err, expectedErr)
		employees, err := repo.ListEmployees(ctx, schemaName, tenantID, false)
		assert.Nil(t, employees)
		assert.ErrorIs(t, err, expectedErr)
		components, err := repo.ListSalaryComponents(ctx, schemaName, tenantID, employee.ID, nil)
		assert.Nil(t, components)
		assert.ErrorIs(t, err, expectedErr)
		gotRun, err = repo.GetPayrollRun(ctx, schemaName, tenantID, run.ID)
		assert.Nil(t, gotRun)
		assert.ErrorIs(t, err, expectedErr)
		runs, err := repo.ListPayrollRuns(ctx, schemaName, tenantID, 2026)
		assert.Nil(t, runs)
		assert.ErrorIs(t, err, expectedErr)
		gotTSD, err = repo.GetTSD(ctx, schemaName, tenantID, 2026, 6)
		assert.Nil(t, gotTSD)
		assert.ErrorIs(t, err, expectedErr)
		rows, err := repo.GetTSDRows(ctx, schemaName, tenantID, declaration.ID)
		assert.Nil(t, rows)
		assert.ErrorIs(t, err, expectedErr)
		declarations, err := repo.ListTSD(ctx, schemaName, tenantID, TSDListFilter{})
		assert.Nil(t, declarations)
		assert.ErrorIs(t, err, expectedErr)
	})

	t.Run("get TSD rows error", func(t *testing.T) {
		repo := NewGORMRepository(newPayrollDryRunDB(t,
			withPayrollDryRunFixtures(payrollDryRunFixtures{tsdDeclarations: []models.TSDDeclaration{declaration}}),
			withPayrollDryRunQueryErrors(nil, expectedErr),
		))
		gotTSD, err := repo.GetTSD(ctx, schemaName, tenantID, 2026, 6)
		assert.Nil(t, gotTSD)
		assert.ErrorIs(t, err, expectedErr)
	})

	t.Run("scan errors", func(t *testing.T) {
		repo := NewGORMRepository(newPayrollDryRunDB(t, withPayrollDryRunRowErrors(expectedErr)))
		salary, err := repo.GetCurrentSalary(ctx, schemaName, tenantID, employee.ID)
		assert.True(t, salary.IsZero())
		assert.ErrorIs(t, err, expectedErr)
		payslips, err := repo.GetPayslipsWithEmployees(ctx, schemaName, tenantID, run.ID)
		assert.Nil(t, payslips)
		assert.ErrorIs(t, err, expectedErr)
	})

	t.Run("delete update errors and missing rows", func(t *testing.T) {
		repo := NewGORMRepository(newPayrollDryRunDB(t, withPayrollDryRunDeleteError(expectedErr)))
		assert.ErrorIs(t, repo.DeletePayslipsByRunID(ctx, schemaName, run.ID), expectedErr)
		assert.ErrorIs(t, repo.DeleteTSDByPeriod(ctx, schemaName, tenantID, 2026, 6), expectedErr)

		repo = NewGORMRepository(newPayrollDryRunDB(t, withPayrollDryRunUpdateError(expectedErr)))
		assert.ErrorIs(t, repo.UpdateEmployee(ctx, schemaName, modelToEmployee(&employee)), expectedErr)
		assert.ErrorIs(t, repo.UpdatePayrollRun(ctx, schemaName, modelToPayrollRun(&run)), expectedErr)
		assert.ErrorIs(t, repo.ApprovePayrollRun(ctx, schemaName, tenantID, run.ID, "approver-1"), expectedErr)
		assert.ErrorIs(t, repo.MarkTSDSubmitted(ctx, schemaName, tenantID, declaration.ID, "EMTA-REF-1", now), expectedErr)
		assert.ErrorIs(t, repo.UpdateTSDStatus(ctx, schemaName, tenantID, declaration.ID, TSDAccepted, now), expectedErr)

		repo = NewGORMRepository(newPayrollDryRunDB(t, withPayrollDryRunUpdateRows(0, 0, 0)))
		assert.ErrorIs(t, repo.ApprovePayrollRun(ctx, schemaName, tenantID, run.ID, "approver-1"), ErrPayrollRunNotFound)
		assert.ErrorIs(t, repo.MarkTSDSubmitted(ctx, schemaName, tenantID, declaration.ID, "EMTA-REF-1", now), ErrTSDDeclarationNotFound)
		assert.ErrorIs(t, repo.UpdateTSDStatus(ctx, schemaName, tenantID, declaration.ID, TSDAccepted, now), ErrTSDDeclarationNotFound)
	})
}

func TestAbsenceGORMRepositoryDryRunOperations(t *testing.T) {
	ctx := context.Background()
	schemaName := "tenant_payroll"
	tenantID := "tenant-1"
	now := time.Date(2026, time.June, 25, 13, 0, 0, 0, time.UTC)
	employee := payrollDryRunEmployeeModel(tenantID, now)
	absenceType := payrollDryRunAbsenceTypeModel(tenantID, now)
	balance := payrollDryRunLeaveBalanceModel(tenantID, employee.ID, absenceType.ID, now)
	record := payrollDryRunLeaveRecordModel(tenantID, employee.ID, absenceType.ID, now)

	repo := NewAbsenceGORMRepository(newPayrollDryRunDB(t,
		withPayrollDryRunFixtures(payrollDryRunFixtures{
			employees:     []models.Employee{employee},
			absenceTypes:  []models.AbsenceType{absenceType},
			leaveBalances: []models.LeaveBalance{balance},
			leaveRecords:  []models.LeaveRecord{record},
		}),
		withPayrollDryRunUpdateRows(1, 1),
	))

	employees, err := repo.ListEmployees(ctx, schemaName, tenantID, true)
	require.NoError(t, err)
	require.Len(t, employees, 1)
	assert.Equal(t, employee.ID, employees[0].ID)

	absenceTypes, err := repo.ListAbsenceTypes(ctx, schemaName, tenantID, true)
	require.NoError(t, err)
	require.Len(t, absenceTypes, 1)
	assert.Equal(t, "ANNUAL", absenceTypes[0].Code)
	absenceTypes, err = repo.ListAbsenceTypes(ctx, schemaName, tenantID, false)
	require.NoError(t, err)
	require.Len(t, absenceTypes, 1)

	gotType, err := repo.GetAbsenceType(ctx, schemaName, tenantID, absenceType.ID)
	require.NoError(t, err)
	assert.Equal(t, absenceType.ID, gotType.ID)
	gotType, err = repo.GetAbsenceTypeByCode(ctx, schemaName, tenantID, absenceType.Code)
	require.NoError(t, err)
	assert.Equal(t, absenceType.ID, gotType.ID)

	gotBalance, err := repo.GetLeaveBalance(ctx, schemaName, tenantID, employee.ID, absenceType.ID, 2026)
	require.NoError(t, err)
	assert.Equal(t, balance.ID, gotBalance.ID)

	require.NoError(t, repo.CreateLeaveBalance(ctx, schemaName, modelToLeaveBalance(&balance)))
	require.NoError(t, repo.UpdateLeaveBalance(ctx, schemaName, modelToLeaveBalance(&balance)))
	require.NoError(t, repo.CreateLeaveRecord(ctx, schemaName, modelToLeaveRecord(&record)))
	gotRecord, err := repo.GetLeaveRecord(ctx, schemaName, tenantID, record.ID)
	require.NoError(t, err)
	assert.Equal(t, record.ID, gotRecord.ID)
	require.NoError(t, repo.UpdateLeaveRecord(ctx, schemaName, modelToLeaveRecord(&record)))

	balanceRepo := NewAbsenceGORMRepository(newPayrollDryRunDB(t,
		withPayrollDryRunScanRows("leave_balances", payrollDryRunSQLRows(t, payrollDryRunLeaveBalanceColumns(), [][]driver.Value{
			payrollDryRunLeaveBalanceRow(balance, absenceType),
		})),
	))
	balances, err := balanceRepo.ListLeaveBalances(ctx, schemaName, tenantID, employee.ID, 2026)
	require.NoError(t, err)
	require.Len(t, balances, 1)
	require.NotNil(t, balances[0].AbsenceType)
	assert.Equal(t, "ANNUAL", balances[0].AbsenceType.Code)

	recordRepo := NewAbsenceGORMRepository(newPayrollDryRunDB(t,
		withPayrollDryRunScanRows("leave_records", payrollDryRunSQLRows(t, payrollDryRunLeaveRecordColumns(), [][]driver.Value{
			payrollDryRunLeaveRecordRow(record, absenceType),
		})),
	))
	records, err := recordRepo.ListLeaveRecords(ctx, schemaName, tenantID, employee.ID, 2026)
	require.NoError(t, err)
	require.Len(t, records, 1)
	require.NotNil(t, records[0].AbsenceType)
	assert.Equal(t, "Annual leave", records[0].AbsenceType.Name)
	records, err = recordRepo.ListLeaveRecords(ctx, schemaName, tenantID, "", 0)
	require.NoError(t, err)
	require.Len(t, records, 0)
}

func TestAbsenceGORMRepositoryDryRunErrors(t *testing.T) {
	ctx := context.Background()
	schemaName := "tenant_payroll"
	tenantID := "tenant-1"
	now := time.Date(2026, time.June, 25, 14, 0, 0, 0, time.UTC)
	employee := payrollDryRunEmployeeModel(tenantID, now)
	absenceType := payrollDryRunAbsenceTypeModel(tenantID, now)
	balance := payrollDryRunLeaveBalanceModel(tenantID, employee.ID, absenceType.ID, now)
	record := payrollDryRunLeaveRecordModel(tenantID, employee.ID, absenceType.ID, now)
	expectedErr := errors.New("dry-run failed")

	t.Run("invalid schema", func(t *testing.T) {
		repo := NewAbsenceGORMRepository(newPayrollDryRunDB(t))
		assert.ErrorContains(t, repo.CreateLeaveBalance(ctx, "tenant-payroll", modelToLeaveBalance(&balance)), "invalid")
		_, err := repo.ListLeaveBalances(ctx, "tenant-payroll", tenantID, employee.ID, 2026)
		assert.ErrorContains(t, err, "invalid")
		_, err = repo.ListLeaveRecords(ctx, "tenant-payroll", tenantID, employee.ID, 2026)
		assert.ErrorContains(t, err, "invalid")
	})

	t.Run("create errors", func(t *testing.T) {
		repo := NewAbsenceGORMRepository(newPayrollDryRunDB(t, withPayrollDryRunCreateError(expectedErr)))
		assert.ErrorIs(t, repo.CreateLeaveBalance(ctx, schemaName, modelToLeaveBalance(&balance)), expectedErr)
		assert.ErrorIs(t, repo.CreateLeaveRecord(ctx, schemaName, modelToLeaveRecord(&record)), expectedErr)
	})

	t.Run("get not found and query errors", func(t *testing.T) {
		repo := NewAbsenceGORMRepository(newPayrollDryRunDB(t, withPayrollDryRunQueryErrors(gorm.ErrRecordNotFound)))
		gotType, err := repo.GetAbsenceType(ctx, schemaName, tenantID, absenceType.ID)
		assert.Nil(t, gotType)
		assert.ErrorIs(t, err, ErrAbsenceTypeNotFound)
		gotType, err = repo.GetAbsenceTypeByCode(ctx, schemaName, tenantID, absenceType.Code)
		assert.Nil(t, gotType)
		assert.ErrorIs(t, err, ErrAbsenceTypeNotFound)
		gotBalance, err := repo.GetLeaveBalance(ctx, schemaName, tenantID, employee.ID, absenceType.ID, 2026)
		assert.Nil(t, gotBalance)
		assert.ErrorIs(t, err, ErrLeaveBalanceNotFound)
		gotRecord, err := repo.GetLeaveRecord(ctx, schemaName, tenantID, record.ID)
		assert.Nil(t, gotRecord)
		assert.ErrorIs(t, err, ErrLeaveRecordNotFound)

		repo = NewAbsenceGORMRepository(newPayrollDryRunDB(t, withPayrollDryRunQueryErrors(expectedErr)))
		gotType, err = repo.GetAbsenceType(ctx, schemaName, tenantID, absenceType.ID)
		assert.Nil(t, gotType)
		assert.ErrorIs(t, err, expectedErr)
		gotType, err = repo.GetAbsenceTypeByCode(ctx, schemaName, tenantID, absenceType.Code)
		assert.Nil(t, gotType)
		assert.ErrorIs(t, err, expectedErr)
		absenceTypes, err := repo.ListAbsenceTypes(ctx, schemaName, tenantID, false)
		assert.Nil(t, absenceTypes)
		assert.ErrorIs(t, err, expectedErr)
		gotBalance, err = repo.GetLeaveBalance(ctx, schemaName, tenantID, employee.ID, absenceType.ID, 2026)
		assert.Nil(t, gotBalance)
		assert.ErrorIs(t, err, expectedErr)
		gotRecord, err = repo.GetLeaveRecord(ctx, schemaName, tenantID, record.ID)
		assert.Nil(t, gotRecord)
		assert.ErrorIs(t, err, expectedErr)
	})

	t.Run("scan errors", func(t *testing.T) {
		repo := NewAbsenceGORMRepository(newPayrollDryRunDB(t, withPayrollDryRunRowErrors(expectedErr)))
		balances, err := repo.ListLeaveBalances(ctx, schemaName, tenantID, employee.ID, 2026)
		assert.Nil(t, balances)
		assert.ErrorIs(t, err, expectedErr)
		records, err := repo.ListLeaveRecords(ctx, schemaName, tenantID, employee.ID, 2026)
		assert.Nil(t, records)
		assert.ErrorIs(t, err, expectedErr)
	})

	t.Run("update errors and missing rows", func(t *testing.T) {
		repo := NewAbsenceGORMRepository(newPayrollDryRunDB(t, withPayrollDryRunUpdateError(expectedErr)))
		assert.ErrorIs(t, repo.UpdateLeaveBalance(ctx, schemaName, modelToLeaveBalance(&balance)), expectedErr)
		assert.ErrorIs(t, repo.UpdateLeaveRecord(ctx, schemaName, modelToLeaveRecord(&record)), expectedErr)

		repo = NewAbsenceGORMRepository(newPayrollDryRunDB(t, withPayrollDryRunUpdateRows(0, 0)))
		assert.ErrorIs(t, repo.UpdateLeaveBalance(ctx, schemaName, modelToLeaveBalance(&balance)), ErrLeaveBalanceNotFound)
		assert.ErrorIs(t, repo.UpdateLeaveRecord(ctx, schemaName, modelToLeaveRecord(&record)), ErrLeaveRecordNotFound)
	})
}

func TestLeaveEvidencePolicyConflictErrorNilReceivers(t *testing.T) {
	var conflict *LeaveEvidencePolicyConflictError

	assert.Empty(t, conflict.Error())
	assert.Nil(t, conflict.Unwrap())

	conflict = &LeaveEvidencePolicyConflictError{}
	assert.Empty(t, conflict.Error())
	assert.Nil(t, conflict.Unwrap())
}

func TestUpdatePayrollRunPaymentDateRepositoryErrors(t *testing.T) {
	ctx := context.Background()
	paymentDate := time.Date(2026, time.June, 30, 0, 0, 0, 0, time.UTC)

	t.Run("get run error", func(t *testing.T) {
		repo := NewMockRepository()
		repo.GetPayrollRunErr = errors.New("get failed")
		service := NewServiceWithRepository(repo, &MockUUIDGenerator{prefix: "run"})

		run, err := service.UpdatePayrollRunPaymentDate(ctx, "tenant_payroll", "tenant-1", "run-1", &UpdatePayrollRunPaymentDateRequest{
			PaymentDate: paymentDate,
		})

		assert.Nil(t, run)
		require.ErrorContains(t, err, "get payroll run")
	})

	t.Run("update run error", func(t *testing.T) {
		repo := NewMockRepository()
		repo.PayrollRuns["run-1"] = &PayrollRun{
			ID:          "run-1",
			TenantID:    "tenant-1",
			PeriodYear:  2026,
			PeriodMonth: 6,
			Status:      PayrollCalculated,
		}
		repo.UpdatePayrollRunErr = errors.New("update failed")
		service := NewServiceWithRepository(repo, &MockUUIDGenerator{prefix: "run"})

		run, err := service.UpdatePayrollRunPaymentDate(ctx, "tenant_payroll", "tenant-1", "run-1", &UpdatePayrollRunPaymentDateRequest{
			PaymentDate: paymentDate,
		})

		assert.Nil(t, run)
		require.ErrorContains(t, err, "update payroll payment date")
	})
}

func TestPayrollServiceConstructorsWithUnavailablePoolPanic(t *testing.T) {
	assert.Panics(t, func() {
		NewService(newUnavailablePayrollPool(t))
	})
	assert.Panics(t, func() {
		NewAbsenceServiceWithPoolAndEvidence(newUnavailablePayrollPool(t), nil)
	})
}

func newUnavailablePayrollPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	config, err := pgxpool.ParseConfig("postgres://postgres:postgres@127.0.0.1:1/open_accounting?connect_timeout=1")
	require.NoError(t, err)
	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	return pool
}

func payrollDryRunEmployeeModel(tenantID string, now time.Time) models.Employee {
	return models.Employee{
		ID:                   "employee-1",
		TenantID:             tenantID,
		EmployeeNumber:       "EMP-001",
		FirstName:            "Mari",
		LastName:             "Mets",
		PersonalCode:         "49001010010",
		Email:                "mari@example.com",
		Phone:                "+3725550100",
		Address:              "Payroll 1",
		BankAccount:          "EE383800853212345678",
		StartDate:            now.AddDate(-1, 0, 0),
		Position:             "Accountant",
		Department:           "Finance",
		EmploymentType:       models.EmploymentFullTime,
		TaxResidency:         "EE",
		ApplyBasicExemption:  true,
		BasicExemptionAmount: models.Decimal{Decimal: decimal.NewFromInt(700)},
		FundedPensionRate:    models.Decimal{Decimal: decimal.NewFromFloat(0.02)},
		IsActive:             true,
		CreatedAt:            now,
		UpdatedAt:            now,
	}
}

func payrollDryRunSalaryComponentModel(tenantID, employeeID string, now time.Time) models.SalaryComponent {
	return models.SalaryComponent{
		ID:            "component-1",
		TenantID:      tenantID,
		EmployeeID:    employeeID,
		ComponentType: SalaryComponentBaseSalary,
		Name:          "Base salary",
		Amount:        models.Decimal{Decimal: decimal.NewFromInt(2500)},
		IsTaxable:     true,
		IsRecurring:   true,
		EffectiveFrom: now.AddDate(0, -1, 0),
		CreatedAt:     now,
	}
}

func payrollDryRunPayrollRunModel(tenantID string, now time.Time) models.PayrollRun {
	createdBy := "user-1"
	return models.PayrollRun{
		ID:                "run-1",
		TenantID:          tenantID,
		PeriodYear:        2026,
		PeriodMonth:       6,
		Status:            models.PayrollStatus(PayrollCalculated),
		TotalGross:        models.Decimal{Decimal: decimal.NewFromInt(2500)},
		TotalNet:          models.Decimal{Decimal: decimal.NewFromInt(1950)},
		TotalEmployerCost: models.Decimal{Decimal: decimal.NewFromInt(3350)},
		Notes:             "June payroll",
		CreatedBy:         &createdBy,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
}

func payrollDryRunPayslipModel(tenantID, runID, employeeID string, now time.Time) models.Payslip {
	return models.Payslip{
		ID:                      "payslip-1",
		TenantID:                tenantID,
		PayrollRunID:            runID,
		EmployeeID:              employeeID,
		GrossSalary:             models.Decimal{Decimal: decimal.NewFromInt(2500)},
		TaxableIncome:           models.Decimal{Decimal: decimal.NewFromInt(1800)},
		IncomeTax:               models.Decimal{Decimal: decimal.NewFromInt(396)},
		UnemploymentInsuranceEE: models.Decimal{Decimal: decimal.NewFromInt(40)},
		FundedPension:           models.Decimal{Decimal: decimal.NewFromInt(50)},
		OtherDeductions:         models.Decimal{Decimal: decimal.Zero},
		NetSalary:               models.Decimal{Decimal: decimal.NewFromInt(2014)},
		SocialTax:               models.Decimal{Decimal: decimal.NewFromInt(825)},
		UnemploymentInsuranceER: models.Decimal{Decimal: decimal.NewFromInt(20)},
		TotalEmployerCost:       models.Decimal{Decimal: decimal.NewFromInt(3345)},
		BasicExemptionApplied:   models.Decimal{Decimal: decimal.NewFromInt(700)},
		PaymentStatus:           "PENDING",
		CreatedAt:               now,
	}
}

func payrollDryRunTSDDeclarationModel(tenantID, runID string, now time.Time) models.TSDDeclaration {
	return models.TSDDeclaration{
		ID:                  "tsd-1",
		TenantID:            tenantID,
		PeriodYear:          2026,
		PeriodMonth:         6,
		PayrollRunID:        &runID,
		TotalPayments:       models.Decimal{Decimal: decimal.NewFromInt(2500)},
		TotalIncomeTax:      models.Decimal{Decimal: decimal.NewFromInt(396)},
		TotalSocialTax:      models.Decimal{Decimal: decimal.NewFromInt(825)},
		TotalUnemploymentER: models.Decimal{Decimal: decimal.NewFromInt(20)},
		TotalUnemploymentEE: models.Decimal{Decimal: decimal.NewFromInt(40)},
		TotalFundedPension:  models.Decimal{Decimal: decimal.NewFromInt(50)},
		Status:              string(TSDDraft),
		CreatedAt:           now,
		UpdatedAt:           now,
	}
}

func payrollDryRunTSDRowModel(tenantID, declarationID, employeeID string, now time.Time) models.TSDRow {
	return models.TSDRow{
		ID:             "tsd-row-1",
		TenantID:       tenantID,
		DeclarationID:  declarationID,
		EmployeeID:     employeeID,
		PersonalCode:   "49001010010",
		FirstName:      "Mari",
		LastName:       "Mets",
		PaymentType:    PaymentTypeSalary,
		GrossPayment:   models.Decimal{Decimal: decimal.NewFromInt(2500)},
		BasicExemption: models.Decimal{Decimal: decimal.NewFromInt(700)},
		TaxableAmount:  models.Decimal{Decimal: decimal.NewFromInt(1800)},
		IncomeTax:      models.Decimal{Decimal: decimal.NewFromInt(396)},
		SocialTax:      models.Decimal{Decimal: decimal.NewFromInt(825)},
		UnemploymentER: models.Decimal{Decimal: decimal.NewFromInt(20)},
		UnemploymentEE: models.Decimal{Decimal: decimal.NewFromInt(40)},
		FundedPension:  models.Decimal{Decimal: decimal.NewFromInt(50)},
		CreatedAt:      now,
	}
}

func payrollDryRunAbsenceTypeModel(tenantID string, now time.Time) models.AbsenceType {
	return models.AbsenceType{
		ID:                 "absence-type-1",
		TenantID:           tenantID,
		Code:               "ANNUAL",
		Name:               "Annual leave",
		NameET:             "Puhkus",
		IsPaid:             true,
		AffectsSalary:      false,
		RequiresDocument:   false,
		DefaultDaysPerYear: models.Decimal{Decimal: decimal.NewFromInt(28)},
		MaxCarryoverDays:   models.Decimal{Decimal: decimal.NewFromInt(7)},
		IsSystem:           true,
		IsActive:           true,
		SortOrder:          10,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
}

func payrollDryRunLeaveBalanceModel(tenantID, employeeID, absenceTypeID string, now time.Time) models.LeaveBalance {
	notes := "Imported balance"
	return models.LeaveBalance{
		ID:            "leave-balance-1",
		TenantID:      tenantID,
		EmployeeID:    employeeID,
		AbsenceTypeID: absenceTypeID,
		Year:          2026,
		EntitledDays:  models.Decimal{Decimal: decimal.NewFromInt(28)},
		CarryoverDays: models.Decimal{Decimal: decimal.NewFromInt(2)},
		UsedDays:      models.Decimal{Decimal: decimal.NewFromInt(5)},
		PendingDays:   models.Decimal{Decimal: decimal.NewFromInt(1)},
		RemainingDays: models.Decimal{Decimal: decimal.NewFromInt(24)},
		Notes:         &notes,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
}

func payrollDryRunLeaveRecordModel(tenantID, employeeID, absenceTypeID string, now time.Time) models.LeaveRecord {
	requestedBy := "user-1"
	return models.LeaveRecord{
		ID:            "leave-record-1",
		TenantID:      tenantID,
		EmployeeID:    employeeID,
		AbsenceTypeID: absenceTypeID,
		StartDate:     now.AddDate(0, 0, 7),
		EndDate:       now.AddDate(0, 0, 11),
		TotalDays:     models.Decimal{Decimal: decimal.NewFromInt(5)},
		WorkingDays:   models.Decimal{Decimal: decimal.NewFromInt(5)},
		Status:        string(LeavePending),
		RequestedAt:   now,
		RequestedBy:   &requestedBy,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
}

func payrollDryRunPayslipWithEmployeeColumns() []string {
	return []string{
		"id", "tenant_id", "payroll_run_id", "employee_id",
		"gross_salary", "taxable_income", "income_tax", "unemployment_insurance_employee",
		"funded_pension", "other_deductions", "net_salary", "social_tax",
		"unemployment_insurance_employer", "total_employer_cost", "basic_exemption_applied",
		"payment_status", "paid_at", "created_at",
		"employee_first_name", "employee_last_name", "employee_personal_code", "employee_email",
	}
}

func payrollDryRunPayslipWithEmployeeRow(p models.Payslip, firstName, lastName, personalCode, email string) []driver.Value {
	return []driver.Value{
		p.ID, p.TenantID, p.PayrollRunID, p.EmployeeID,
		p.GrossSalary.String(), p.TaxableIncome.String(), p.IncomeTax.String(), p.UnemploymentInsuranceEE.String(),
		p.FundedPension.String(), p.OtherDeductions.String(), p.NetSalary.String(), p.SocialTax.String(),
		p.UnemploymentInsuranceER.String(), p.TotalEmployerCost.String(), p.BasicExemptionApplied.String(),
		p.PaymentStatus, p.PaidAt, p.CreatedAt,
		firstName, lastName, personalCode, email,
	}
}

func payrollDryRunLeaveBalanceColumns() []string {
	return []string{
		"id", "tenant_id", "employee_id", "absence_type_id", "year",
		"entitled_days", "carryover_days", "used_days", "pending_days", "remaining_days",
		"notes", "created_at", "updated_at",
		"absence_type_code", "absence_type_name", "absence_type_name_et",
	}
}

func payrollDryRunLeaveBalanceRow(b models.LeaveBalance, absenceType models.AbsenceType) []driver.Value {
	var notes driver.Value
	if b.Notes != nil {
		notes = *b.Notes
	}
	return []driver.Value{
		b.ID, b.TenantID, b.EmployeeID, b.AbsenceTypeID, int64(b.Year),
		b.EntitledDays.String(), b.CarryoverDays.String(), b.UsedDays.String(), b.PendingDays.String(), b.RemainingDays.String(),
		notes, b.CreatedAt, b.UpdatedAt,
		absenceType.Code, absenceType.Name, absenceType.NameET,
	}
}

func payrollDryRunLeaveRecordColumns() []string {
	return []string{
		"id", "tenant_id", "employee_id", "absence_type_id",
		"start_date", "end_date", "total_days", "working_days", "status",
		"document_number", "document_date", "document_url",
		"requested_at", "requested_by", "approved_at", "approved_by", "rejected_at", "rejected_by",
		"rejection_reason", "payroll_run_id", "notes", "created_at", "updated_at",
		"absence_type_code", "absence_type_name", "absence_type_name_et",
	}
}

func payrollDryRunLeaveRecordRow(r models.LeaveRecord, absenceType models.AbsenceType) []driver.Value {
	return []driver.Value{
		r.ID, r.TenantID, r.EmployeeID, r.AbsenceTypeID,
		r.StartDate, r.EndDate, r.TotalDays.String(), r.WorkingDays.String(), r.Status,
		valueStringPtr(r.DocumentNumber), r.DocumentDate, valueStringPtr(r.DocumentURL),
		r.RequestedAt, valueStringPtr(r.RequestedBy), r.ApprovedAt, valueStringPtr(r.ApprovedBy), r.RejectedAt, valueStringPtr(r.RejectedBy),
		valueStringPtr(r.RejectionReason), valueStringPtr(r.PayrollRunID), valueStringPtr(r.Notes), r.CreatedAt, r.UpdatedAt,
		absenceType.Code, absenceType.Name, absenceType.NameET,
	}
}

func valueStringPtr(value *string) driver.Value {
	if value == nil {
		return nil
	}
	return *value
}
