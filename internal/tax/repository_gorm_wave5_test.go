package tax

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type taxDryRunRowSetWave5 struct {
	columns []string
	values  [][]driver.Value
}

var taxDryRunRowsWave5ID uint64
var taxDryRunRowsWave5DriverOnce sync.Once
var taxDryRunRowsWave5Mu sync.Mutex
var taxDryRunRowsWave5ByDSN = map[string]taxDryRunRowSetWave5{}

func withTaxDryRunScanRowsWave5(rowSets ...taxDryRunRowSetWave5) taxDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		var index int
		err := db.Callback().Row().After("gorm:row").Register(taxDryRunCallbackName("wave5_scan_rows"), func(tx *gorm.DB) {
			if index >= len(rowSets) {
				tx.AddError(fmt.Errorf("missing tax wave5 dry-run row set %d", index))
				return
			}
			rowSet := rowSets[index]
			index++
			tx.Statement.Dest = newTaxDryRunSQLRowsWave5(t, rowSet)
			tx.RowsAffected = int64(len(rowSet.values))
		})
		require.NoError(t, err)
	}
}

func newTaxDryRunSQLRowsWave5(t *testing.T, rowSet taxDryRunRowSetWave5) *sql.Rows {
	t.Helper()

	taxDryRunRowsWave5DriverOnce.Do(func() {
		sql.Register("tax_wave5_dryrun_rows", taxDryRunRowsWave5Driver{})
	})

	dsn := fmt.Sprintf("tax-wave5-dry-run-rows-%d", atomic.AddUint64(&taxDryRunRowsWave5ID, 1))
	taxDryRunRowsWave5Mu.Lock()
	taxDryRunRowsWave5ByDSN[dsn] = rowSet
	taxDryRunRowsWave5Mu.Unlock()

	db, err := sql.Open("tax_wave5_dryrun_rows", dsn)
	require.NoError(t, err)
	rows, err := db.QueryContext(context.Background(), "SELECT 1")
	require.NoError(t, err)

	t.Cleanup(func() {
		_ = rows.Close()
		_ = db.Close()
		taxDryRunRowsWave5Mu.Lock()
		delete(taxDryRunRowsWave5ByDSN, dsn)
		taxDryRunRowsWave5Mu.Unlock()
	})

	return rows
}

type taxDryRunRowsWave5Driver struct{}

func (taxDryRunRowsWave5Driver) Open(name string) (driver.Conn, error) {
	return taxDryRunRowsWave5Conn{dsn: name}, nil
}

type taxDryRunRowsWave5Conn struct {
	dsn string
}

func (taxDryRunRowsWave5Conn) Prepare(string) (driver.Stmt, error) {
	return nil, fmt.Errorf("tax wave5 dry-run rows do not prepare statements")
}

func (taxDryRunRowsWave5Conn) Close() error {
	return nil
}

func (taxDryRunRowsWave5Conn) Begin() (driver.Tx, error) {
	return nil, fmt.Errorf("tax wave5 dry-run rows do not begin transactions")
}

func (c taxDryRunRowsWave5Conn) QueryContext(context.Context, string, []driver.NamedValue) (driver.Rows, error) {
	taxDryRunRowsWave5Mu.Lock()
	rowSet, ok := taxDryRunRowsWave5ByDSN[c.dsn]
	taxDryRunRowsWave5Mu.Unlock()
	if !ok {
		return nil, fmt.Errorf("tax wave5 dry-run row set %q not found", c.dsn)
	}
	return &taxDryRunSQLRowsWave5{
		columns: append([]string(nil), rowSet.columns...),
		values:  append([][]driver.Value(nil), rowSet.values...),
	}, nil
}

type taxDryRunSQLRowsWave5 struct {
	columns []string
	values  [][]driver.Value
	index   int
}

func (r *taxDryRunSQLRowsWave5) Columns() []string {
	return append([]string(nil), r.columns...)
}

func (*taxDryRunSQLRowsWave5) Close() error {
	return nil
}

func (r *taxDryRunSQLRowsWave5) Next(dest []driver.Value) error {
	if r.index >= len(r.values) {
		return io.EOF
	}
	copy(dest, r.values[r.index])
	r.index++
	return nil
}

func TestGORMRepositoryWave5TaxScanQueriesBuildRows(t *testing.T) {
	ctx := context.Background()
	schemaName := "tenant_tax"
	tenantID := "tenant-1"
	startDate := time.Date(2026, time.June, 1, 0, 0, 0, 0, time.UTC)
	endDate := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)
	invoiceDate := time.Date(2026, time.June, 15, 0, 0, 0, 0, time.UTC)
	repo := NewGORMRepository(newTaxDryRunDB(t, withTaxDryRunScanRowsWave5(
		taxDryRunRowSetWave5{
			columns: []string{"vat_rate", "is_output", "tax_base", "tax_amount"},
			values: [][]driver.Value{
				{"22", true, "1000", "220"},
				{"22", true, "50", "11"},
				{"22", false, "400", "88"},
			},
		},
		taxDryRunRowSetWave5{
			columns: []string{"vat_rate", "tax_base", "tax_amount"},
			values:  [][]driver.Value{{"20", "100", "20"}},
		},
		taxDryRunRowSetWave5{
			columns: []string{
				"part",
				"contact_id",
				"contact_name",
				"contact_reg_code",
				"contact_vat_number",
				"invoice_id",
				"invoice_number",
				"invoice_date",
				"invoice_type",
				"taxable_amount",
				"vat_amount",
				"total_amount",
				"partner_period_taxable_amount",
			},
			values: [][]driver.Value{{
				string(KMDINFPartSales),
				"contact-1",
				"Acme OU",
				"12345678",
				"EE123456789",
				"invoice-1",
				"INV-1",
				invoiceDate,
				"SALES",
				"1200",
				"264",
				"1464",
				"1200",
			}},
		},
		taxDryRunRowSetWave5{
			columns: []string{"country_code", "vat_rate", "invoice_count", "line_count", "taxable_amount", "vat_amount", "total_amount"},
			values: [][]driver.Value{{
				"DE",
				"19",
				int64(2),
				int64(3),
				"1000",
				"190",
				"1190",
			}},
		},
	)))

	vatRows, err := repo.QueryVATData(ctx, schemaName, tenantID, startDate, endDate)
	require.NoError(t, err)
	require.Len(t, vatRows, 4)
	assert.True(t, vatRows[0].VATRate.Equal(decimal.NewFromInt(22)))
	assert.True(t, vatRows[0].IsOutput)
	assert.True(t, vatRows[0].TaxBase.Equal(decimal.NewFromInt(1050)))
	assert.True(t, vatRows[0].TaxAmount.Equal(decimal.NewFromInt(231)))
	assert.True(t, vatRows[2].VATRate.Equal(decimal.NewFromInt(20)))
	assert.True(t, vatRows[2].IsOutput)
	assert.True(t, vatRows[3].VATRate.Equal(decimal.NewFromInt(20)))
	assert.False(t, vatRows[3].IsOutput)

	kmdINFRows, err := repo.QueryKMDINFData(ctx, schemaName, tenantID, startDate, endDate, decimal.NewFromInt(1000))
	require.NoError(t, err)
	require.Len(t, kmdINFRows, 1)
	assert.Equal(t, KMDINFPartSales, kmdINFRows[0].Part)
	assert.Equal(t, "Acme OU", kmdINFRows[0].ContactName)
	assert.True(t, kmdINFRows[0].TaxableAmount.Equal(decimal.NewFromInt(1200)))
	assert.True(t, kmdINFRows[0].PartnerPeriodTaxableAmount.Equal(decimal.NewFromInt(1200)))

	ossRows, err := repo.QueryEUVATOSSData(ctx, schemaName, tenantID, startDate, endDate, true)
	require.NoError(t, err)
	require.Len(t, ossRows, 1)
	assert.Equal(t, "DE", ossRows[0].CountryCode)
	assert.True(t, ossRows[0].VATRate.Equal(decimal.NewFromInt(19)))
	assert.Equal(t, 2, ossRows[0].InvoiceCount)
	assert.Equal(t, 3, ossRows[0].LineCount)
	assert.True(t, ossRows[0].TotalAmount.Equal(decimal.NewFromInt(1190)))
}
