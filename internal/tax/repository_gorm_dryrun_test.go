package tax

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/HMB-research/open-accounting/internal/models"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type taxDryRunConnPool struct{}

func (taxDryRunConnPool) PrepareContext(context.Context, string) (*sql.Stmt, error) {
	return nil, errors.New("dry-run tax tests should not prepare statements")
}

func (taxDryRunConnPool) ExecContext(context.Context, string, ...interface{}) (sql.Result, error) {
	return nil, errors.New("dry-run tax tests should not execute statements")
}

func (taxDryRunConnPool) QueryContext(context.Context, string, ...interface{}) (*sql.Rows, error) {
	return nil, errors.New("dry-run tax tests should not query rows")
}

func (taxDryRunConnPool) QueryRowContext(context.Context, string, ...interface{}) *sql.Row {
	return nil
}

func (taxDryRunConnPool) BeginTx(context.Context, *sql.TxOptions) (gorm.ConnPool, error) {
	return &taxDryRunTx{}, nil
}

type taxDryRunTx struct {
	taxDryRunConnPool
}

func (*taxDryRunTx) Commit() error {
	return nil
}

func (*taxDryRunTx) Rollback() error {
	return nil
}

type taxDryRunDBOption func(t *testing.T, db *gorm.DB)

type taxDryRunFixtures struct {
	declaration  *models.KMDDeclaration
	declarations []models.KMDDeclaration
	rows         []models.KMDRow
}

type taxDryRunRecorder struct {
	queries             []string
	rows                []string
	creates             []string
	deletes             []string
	updates             []string
	createdDeclarations []models.KMDDeclaration
	createdRows         []models.KMDRow
}

var taxDryRunCallbackID uint64

func newTaxDryRunDB(t *testing.T, opts ...taxDryRunDBOption) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(postgres.New(postgres.Config{Conn: taxDryRunConnPool{}}), &gorm.Config{
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

func withTaxDryRunFixtures(fixtures taxDryRunFixtures, recorder *taxDryRunRecorder) taxDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		err := db.Callback().Query().After("gorm:query").Register(taxDryRunCallbackName("query_fixtures"), func(tx *gorm.DB) {
			if recorder != nil {
				recorder.queries = append(recorder.queries, tx.Statement.SQL.String())
			}
			populateTaxDryRunQueryDest(tx, tx.Statement.Dest, &fixtures)
		})
		require.NoError(t, err)
	}
}

func withTaxDryRunQueryErrors(queryErrors ...error) taxDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		var index int
		err := db.Callback().Query().Before("gorm:query").Register(taxDryRunCallbackName("query_error"), func(tx *gorm.DB) {
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

func withTaxDryRunRowRecorder(recorder *taxDryRunRecorder) taxDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		err := db.Callback().Row().After("gorm:row").Register(taxDryRunCallbackName("row_recorder"), func(tx *gorm.DB) {
			if recorder != nil {
				recorder.rows = append(recorder.rows, tx.Statement.SQL.String())
			}
		})
		require.NoError(t, err)
	}
}

func withTaxDryRunRowErrors(rowErrors ...error) taxDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		var index int
		err := db.Callback().Row().After("gorm:row").Register(taxDryRunCallbackName("row_error"), func(tx *gorm.DB) {
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

func withTaxDryRunCreateCapture(recorder *taxDryRunRecorder) taxDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		err := db.Callback().Create().After("gorm:create").Register(taxDryRunCallbackName("create_capture"), func(tx *gorm.DB) {
			if recorder != nil {
				recorder.creates = append(recorder.creates, tx.Statement.SQL.String())
				switch dest := tx.Statement.Dest.(type) {
				case *models.KMDDeclaration:
					recorder.createdDeclarations = append(recorder.createdDeclarations, *dest)
				case *models.KMDRow:
					recorder.createdRows = append(recorder.createdRows, *dest)
				}
			}
			if tx.RowsAffected == 0 {
				tx.RowsAffected = 1
			}
		})
		require.NoError(t, err)
	}
}

func withTaxDryRunCreateErrors(createErrors ...error) taxDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		var index int
		err := db.Callback().Create().Before("gorm:create").Register(taxDryRunCallbackName("create_error"), func(tx *gorm.DB) {
			if len(createErrors) == 0 {
				return
			}
			errIndex := index
			if errIndex >= len(createErrors) {
				errIndex = len(createErrors) - 1
			}
			index++
			if createErrors[errIndex] != nil {
				tx.AddError(createErrors[errIndex])
			}
		})
		require.NoError(t, err)
	}
}

func withTaxDryRunDeleteRows(recorder *taxDryRunRecorder, rows ...int64) taxDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		var index int
		err := db.Callback().Delete().After("gorm:delete").Register(taxDryRunCallbackName("delete_rows"), func(tx *gorm.DB) {
			if recorder != nil {
				recorder.deletes = append(recorder.deletes, tx.Statement.SQL.String())
			}
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

func withTaxDryRunDeleteError(expectedErr error) taxDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		err := db.Callback().Delete().Before("gorm:delete").Register(taxDryRunCallbackName("delete_error"), func(tx *gorm.DB) {
			tx.AddError(expectedErr)
		})
		require.NoError(t, err)
	}
}

func withTaxDryRunUpdateRows(recorder *taxDryRunRecorder, rows ...int64) taxDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		var index int
		err := db.Callback().Update().After("gorm:update").Register(taxDryRunCallbackName("update_rows"), func(tx *gorm.DB) {
			if recorder != nil {
				recorder.updates = append(recorder.updates, tx.Statement.SQL.String())
			}
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

func withTaxDryRunUpdateError(expectedErr error) taxDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		err := db.Callback().Update().Before("gorm:update").Register(taxDryRunCallbackName("update_error"), func(tx *gorm.DB) {
			tx.AddError(expectedErr)
		})
		require.NoError(t, err)
	}
}

func taxDryRunCallbackName(suffix string) string {
	id := atomic.AddUint64(&taxDryRunCallbackID, 1)
	return fmt.Sprintf("tax_dryrun:%d:%s", id, suffix)
}

func populateTaxDryRunQueryDest(tx *gorm.DB, dest any, fixtures *taxDryRunFixtures) {
	switch typed := dest.(type) {
	case *models.KMDDeclaration:
		if fixtures.declaration != nil {
			*typed = *fixtures.declaration
			tx.RowsAffected = 1
		}
	case *[]models.KMDDeclaration:
		*typed = append([]models.KMDDeclaration(nil), fixtures.declarations...)
		tx.RowsAffected = int64(len(fixtures.declarations))
	case *[]models.KMDRow:
		*typed = append([]models.KMDRow(nil), fixtures.rows...)
		tx.RowsAffected = int64(len(fixtures.rows))
	}
}

func TestGORMRepositoryDryRunSaveDeclaration(t *testing.T) {
	ctx := context.Background()
	schemaName := "tenant_tax"
	declaration := taxDryRunDeclaration()

	t.Run("upserts declaration replaces rows and inserts current rows", func(t *testing.T) {
		recorder := &taxDryRunRecorder{}
		repo := NewGORMRepository(newTaxDryRunDB(t,
			withTaxDryRunCreateCapture(recorder),
			withTaxDryRunDeleteRows(recorder, 1),
		))

		err := repo.SaveDeclaration(ctx, schemaName, declaration)

		require.NoError(t, err)
		assert.Equal(t, "decl-1", declaration.ID)
		require.Len(t, recorder.createdDeclarations, 1)
		assert.Equal(t, declaration.ID, recorder.createdDeclarations[0].ID)
		require.Len(t, recorder.createdRows, 2)
		assert.Equal(t, declaration.ID, recorder.createdRows[0].DeclarationID)
		assert.Equal(t, KMDRow1, recorder.createdRows[0].Code)
		assert.True(t, recorder.createdRows[1].TaxAmount.Decimal.Equal(decimal.NewFromInt(55)))
		assertTaxRecordedSQLContains(t, recorder.creates,
			`INSERT INTO "tenant_tax"."kmd_declarations"`,
			`ON CONFLICT ("tenant_id","year","month") DO UPDATE`,
		)
		assertTaxRecordedSQLContains(t, recorder.deletes,
			`DELETE FROM "tenant_tax"."kmd_rows"`,
			`declaration_id = $1`,
		)
	})

	t.Run("wraps declaration insert errors", func(t *testing.T) {
		expectedErr := errors.New("declaration insert failed")
		repo := NewGORMRepository(newTaxDryRunDB(t, withTaxDryRunCreateErrors(expectedErr)))

		err := repo.SaveDeclaration(ctx, schemaName, taxDryRunDeclaration())

		require.ErrorContains(t, err, "insert declaration")
		assert.ErrorIs(t, err, expectedErr)
	})

	t.Run("wraps stale row delete errors", func(t *testing.T) {
		expectedErr := errors.New("delete rows failed")
		repo := NewGORMRepository(newTaxDryRunDB(t,
			withTaxDryRunCreateCapture(nil),
			withTaxDryRunDeleteError(expectedErr),
		))

		err := repo.SaveDeclaration(ctx, schemaName, taxDryRunDeclaration())

		require.ErrorContains(t, err, "delete old rows")
		assert.ErrorIs(t, err, expectedErr)
	})

	t.Run("wraps row insert errors", func(t *testing.T) {
		expectedErr := errors.New("row insert failed")
		repo := NewGORMRepository(newTaxDryRunDB(t,
			withTaxDryRunCreateCapture(nil),
			withTaxDryRunCreateErrors(nil, expectedErr),
			withTaxDryRunDeleteRows(nil, 1),
		))

		err := repo.SaveDeclaration(ctx, schemaName, taxDryRunDeclaration())

		require.ErrorContains(t, err, "insert row")
		assert.ErrorIs(t, err, expectedErr)
	})
}

func TestGORMRepositoryDryRunDeclarationReadQueries(t *testing.T) {
	ctx := context.Background()
	schemaName := "tenant_tax"
	tenantID := "tenant-1"
	model := taxDryRunDeclarationModel()
	rowModels := []models.KMDRow{
		{ID: "row-1", DeclarationID: model.ID, Code: KMDRow1, Description: "Sales", TaxBase: models.Decimal{Decimal: decimal.NewFromInt(1000)}, TaxAmount: models.Decimal{Decimal: decimal.NewFromInt(220)}},
		{ID: "row-2", DeclarationID: model.ID, Code: KMDRow4, Description: "Purchases", TaxBase: models.Decimal{Decimal: decimal.NewFromInt(250)}, TaxAmount: models.Decimal{Decimal: decimal.NewFromInt(55)}},
	}
	recorder := &taxDryRunRecorder{}
	repo := NewGORMRepository(newTaxDryRunDB(t, withTaxDryRunFixtures(taxDryRunFixtures{
		declaration:  &model,
		declarations: []models.KMDDeclaration{model},
		rows:         rowModels,
	}, recorder)))

	declaration, err := repo.GetDeclaration(ctx, schemaName, tenantID, 2026, 6)
	require.NoError(t, err)
	require.NotNil(t, declaration)
	assert.Equal(t, model.ID, declaration.ID)
	require.Len(t, declaration.Rows, 2)
	assert.Equal(t, KMDRow1, declaration.Rows[0].Code)
	assert.True(t, declaration.Rows[1].TaxAmount.Equal(decimal.NewFromInt(55)))

	declarations, err := repo.ListDeclarations(ctx, schemaName, tenantID)
	require.NoError(t, err)
	require.Len(t, declarations, 1)
	assert.Equal(t, model.ID, declarations[0].ID)

	assertTaxRecordedSQLContains(t, recorder.queries,
		`FROM "tenant_tax"."kmd_declarations"`,
		`tenant_id = $1 AND year = $2 AND month = $3`,
		`FROM "tenant_tax"."kmd_rows"`,
		`declaration_id = $1`,
		`ORDER BY code`,
		`ORDER BY year DESC, month DESC`,
	)
}

func TestGORMRepositoryDryRunDeclarationReadErrors(t *testing.T) {
	ctx := context.Background()
	schemaName := "tenant_tax"
	tenantID := "tenant-1"
	expectedErr := errors.New("dry-run query failed")

	t.Run("get declaration not found returns nil", func(t *testing.T) {
		repo := NewGORMRepository(newTaxDryRunDB(t, withTaxDryRunQueryErrors(gorm.ErrRecordNotFound)))

		declaration, err := repo.GetDeclaration(ctx, schemaName, tenantID, 2026, 6)

		require.NoError(t, err)
		assert.Nil(t, declaration)
	})

	t.Run("get declaration query error", func(t *testing.T) {
		repo := NewGORMRepository(newTaxDryRunDB(t, withTaxDryRunQueryErrors(expectedErr)))

		declaration, err := repo.GetDeclaration(ctx, schemaName, tenantID, 2026, 6)

		require.Nil(t, declaration)
		require.ErrorContains(t, err, "get declaration")
		assert.ErrorIs(t, err, expectedErr)
	})

	t.Run("get declaration row query error", func(t *testing.T) {
		model := taxDryRunDeclarationModel()
		repo := NewGORMRepository(newTaxDryRunDB(t,
			withTaxDryRunFixtures(taxDryRunFixtures{declaration: &model}, nil),
			withTaxDryRunQueryErrors(nil, expectedErr),
		))

		declaration, err := repo.GetDeclaration(ctx, schemaName, tenantID, 2026, 6)

		require.Nil(t, declaration)
		require.ErrorContains(t, err, "get rows")
		assert.ErrorIs(t, err, expectedErr)
	})

	t.Run("list declarations query error", func(t *testing.T) {
		repo := NewGORMRepository(newTaxDryRunDB(t, withTaxDryRunQueryErrors(expectedErr)))

		declarations, err := repo.ListDeclarations(ctx, schemaName, tenantID)

		require.Nil(t, declarations)
		require.ErrorContains(t, err, "list declarations")
		assert.ErrorIs(t, err, expectedErr)
	})
}

func TestGORMRepositoryDryRunKMDStatusUpdates(t *testing.T) {
	ctx := context.Background()
	schemaName := "tenant_tax"
	tenantID := "tenant-1"
	declarationID := "decl-1"
	now := time.Date(2026, time.June, 25, 16, 0, 0, 0, time.UTC)

	t.Run("updates submitted and accepted status", func(t *testing.T) {
		recorder := &taxDryRunRecorder{}
		repo := NewGORMRepository(newTaxDryRunDB(t, withTaxDryRunUpdateRows(recorder, 1, 1)))

		require.NoError(t, repo.MarkKMDSubmitted(ctx, schemaName, tenantID, declarationID, now))
		require.NoError(t, repo.UpdateKMDStatus(ctx, schemaName, tenantID, declarationID, KMDStatusAccepted, now.Add(time.Hour)))

		assertTaxRecordedSQLContains(t, recorder.updates,
			`UPDATE "tenant_tax"."kmd_declarations"`,
			`status`,
			`submitted_at`,
			`updated_at`,
			`tenant_id = $`,
			`id = $`,
		)
	})

	t.Run("returns not found when no rows are updated", func(t *testing.T) {
		repo := NewGORMRepository(newTaxDryRunDB(t, withTaxDryRunUpdateRows(nil, 0, 0)))

		assert.ErrorIs(t, repo.MarkKMDSubmitted(ctx, schemaName, tenantID, declarationID, now), ErrKMDDeclarationNotFound)
		assert.ErrorIs(t, repo.UpdateKMDStatus(ctx, schemaName, tenantID, declarationID, KMDStatusAccepted, now), ErrKMDDeclarationNotFound)
	})

	t.Run("wraps update errors", func(t *testing.T) {
		expectedErr := errors.New("update failed")
		repo := NewGORMRepository(newTaxDryRunDB(t, withTaxDryRunUpdateError(expectedErr)))

		err := repo.MarkKMDSubmitted(ctx, schemaName, tenantID, declarationID, now)
		require.ErrorContains(t, err, "mark KMD submitted")
		assert.ErrorIs(t, err, expectedErr)

		err = repo.UpdateKMDStatus(ctx, schemaName, tenantID, declarationID, KMDStatusAccepted, now)
		require.ErrorContains(t, err, "update KMD status")
		assert.ErrorIs(t, err, expectedErr)
	})
}

func TestGORMRepositoryDryRunTaxScanQueryErrorsAndSQL(t *testing.T) {
	ctx := context.Background()
	schemaName := "tenant_tax"
	tenantID := "tenant-1"
	startDate := time.Date(2026, time.June, 1, 0, 0, 0, 0, time.UTC)
	endDate := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)
	rowErr := errors.New("dry-run scan failed")
	recorder := &taxDryRunRecorder{}
	repo := NewGORMRepository(newTaxDryRunDB(t,
		withTaxDryRunRowRecorder(recorder),
		withTaxDryRunRowErrors(rowErr),
	))

	tests := []struct {
		name string
		run  func() error
		want string
	}{
		{
			name: "VAT data",
			run: func() error {
				_, err := repo.QueryVATData(ctx, schemaName, tenantID, startDate, endDate)
				return err
			},
			want: "query VAT data",
		},
		{
			name: "KMD INF data",
			run: func() error {
				_, err := repo.QueryKMDINFData(ctx, schemaName, tenantID, startDate, endDate, decimal.NewFromInt(1000))
				return err
			},
			want: "query KMD INF data",
		},
		{
			name: "EU VAT OSS data",
			run: func() error {
				_, err := repo.QueryEUVATOSSData(ctx, schemaName, tenantID, startDate, endDate, false)
				return err
			},
			want: "query EU VAT OSS data",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.run()

			require.ErrorContains(t, err, tt.want)
			assert.ErrorIs(t, err, rowErr)
		})
	}

	assertTaxRecordedSQLContains(t, recorder.rows,
		`FROM "tenant_tax"."journal_entries" AS je`,
		`JOIN "tenant_tax"."journal_entry_lines" AS jl ON je.id = jl.journal_entry_id`,
		`JOIN "tenant_tax"."accounts" AS a ON jl.account_id = a.id`,
		`FROM "tenant_tax"."invoices" AS i`,
		`JOIN "tenant_tax"."contacts" AS c ON c.id = i.contact_id AND c.tenant_id = i.tenant_id`,
		`JOIN "tenant_tax"."invoice_lines" AS il ON il.invoice_id = i.id AND il.tenant_id = i.tenant_id`,
	)
}

func TestGORMRepositoryDryRunRejectsInvalidTenantSchema(t *testing.T) {
	ctx := context.Background()
	repo := NewGORMRepository(newTaxDryRunDB(t))
	declaration := taxDryRunDeclaration()
	now := time.Date(2026, time.June, 25, 17, 0, 0, 0, time.UTC)

	tests := []struct {
		name string
		run  func() error
	}{
		{
			name: "QueryVATData",
			run: func() error {
				_, err := repo.QueryVATData(ctx, "tenant-tax", declaration.TenantID, now, now)
				return err
			},
		},
		{
			name: "QueryKMDINFData",
			run: func() error {
				_, err := repo.QueryKMDINFData(ctx, "tenant-tax", declaration.TenantID, now, now, decimal.NewFromInt(1000))
				return err
			},
		},
		{
			name: "QueryEUVATOSSData",
			run: func() error {
				_, err := repo.QueryEUVATOSSData(ctx, "tenant-tax", declaration.TenantID, now, now, false)
				return err
			},
		},
		{
			name: "SaveDeclaration",
			run: func() error {
				return repo.SaveDeclaration(ctx, "tenant-tax", declaration)
			},
		},
		{
			name: "GetDeclaration",
			run: func() error {
				_, err := repo.GetDeclaration(ctx, "tenant-tax", declaration.TenantID, declaration.Year, declaration.Month)
				return err
			},
		},
		{
			name: "ListDeclarations",
			run: func() error {
				_, err := repo.ListDeclarations(ctx, "tenant-tax", declaration.TenantID)
				return err
			},
		},
		{
			name: "MarkKMDSubmitted",
			run: func() error {
				return repo.MarkKMDSubmitted(ctx, "tenant-tax", declaration.TenantID, declaration.ID, now)
			},
		},
		{
			name: "UpdateKMDStatus",
			run: func() error {
				return repo.UpdateKMDStatus(ctx, "tenant-tax", declaration.TenantID, declaration.ID, KMDStatusAccepted, now)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.run()

			require.Error(t, err)
			assert.Contains(t, err.Error(), "invalid SQL identifier")
		})
	}
}

func taxDryRunDeclaration() *KMDDeclaration {
	now := time.Date(2026, time.June, 25, 15, 0, 0, 0, time.UTC)
	return &KMDDeclaration{
		ID:             "decl-1",
		TenantID:       "tenant-1",
		Year:           2026,
		Month:          6,
		Status:         KMDStatusDraft,
		TotalOutputVAT: decimal.NewFromInt(220),
		TotalInputVAT:  decimal.NewFromInt(55),
		Rows: []KMDRow{
			{Code: KMDRow1, Description: "Sales", TaxBase: decimal.NewFromInt(1000), TaxAmount: decimal.NewFromInt(220)},
			{Code: KMDRow4, Description: "Purchases", TaxBase: decimal.NewFromInt(250), TaxAmount: decimal.NewFromInt(55)},
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func taxDryRunDeclarationModel() models.KMDDeclaration {
	decl := taxDryRunDeclaration()
	return models.KMDDeclaration{
		ID:             decl.ID,
		TenantID:       decl.TenantID,
		Year:           decl.Year,
		Month:          decl.Month,
		Status:         decl.Status,
		TotalOutputVAT: models.Decimal{Decimal: decl.TotalOutputVAT},
		TotalInputVAT:  models.Decimal{Decimal: decl.TotalInputVAT},
		SubmittedAt:    decl.SubmittedAt,
		CreatedAt:      decl.CreatedAt,
		UpdatedAt:      decl.UpdatedAt,
	}
}

func assertTaxRecordedSQLContains(t *testing.T, sqlStatements []string, fragments ...string) {
	t.Helper()

	allSQL := strings.Join(sqlStatements, "\n")
	require.NotEmpty(t, allSQL)
	for _, fragment := range fragments {
		assert.Contains(t, allSQL, fragment)
	}
}
