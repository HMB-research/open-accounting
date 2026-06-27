package payments

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type paymentsWave7RowSet struct {
	columns []string
	values  [][]driver.Value
}

var paymentsWave7RowsDSNID uint64
var paymentsWave7RowsDriverOnce sync.Once
var paymentsWave7RowsMu sync.Mutex
var paymentsWave7RowsByDSN = map[string]paymentsWave7RowSet{}

func withPaymentsWave7ScanRows(rowSets ...paymentsWave7RowSet) paymentsDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		var index int
		err := db.Callback().Row().After("gorm:row").Register(paymentsDryRunCallbackName("scan_rows_wave7"), func(tx *gorm.DB) {
			if index >= len(rowSets) {
				tx.AddError(fmt.Errorf("missing payments wave7 row set %d", index))
				return
			}
			rowSet := rowSets[index]
			index++
			tx.Statement.Dest = newPaymentsWave7SQLRows(t, rowSet)
			tx.RowsAffected = int64(len(rowSet.values))
		})
		require.NoError(t, err)
	}
}

func newPaymentsWave7SQLRows(t *testing.T, rowSet paymentsWave7RowSet) *sql.Rows {
	t.Helper()

	paymentsWave7RowsDriverOnce.Do(func() {
		sql.Register("payments_wave7_rows", paymentsWave7RowsDriver{})
	})

	dsn := fmt.Sprintf("payments-wave7-rows-%d", atomic.AddUint64(&paymentsWave7RowsDSNID, 1))
	paymentsWave7RowsMu.Lock()
	paymentsWave7RowsByDSN[dsn] = rowSet
	paymentsWave7RowsMu.Unlock()

	db, err := sql.Open("payments_wave7_rows", dsn)
	require.NoError(t, err)
	rows, err := db.QueryContext(context.Background(), "SELECT 1")
	require.NoError(t, err)

	t.Cleanup(func() {
		_ = rows.Close()
		_ = db.Close()
		paymentsWave7RowsMu.Lock()
		delete(paymentsWave7RowsByDSN, dsn)
		paymentsWave7RowsMu.Unlock()
	})

	return rows
}

type paymentsWave7RowsDriver struct{}

func (paymentsWave7RowsDriver) Open(name string) (driver.Conn, error) {
	return paymentsWave7RowsConn{dsn: name}, nil
}

type paymentsWave7RowsConn struct {
	dsn string
}

func (paymentsWave7RowsConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("payments wave7 rows do not prepare statements")
}

func (paymentsWave7RowsConn) Close() error {
	return nil
}

func (paymentsWave7RowsConn) Begin() (driver.Tx, error) {
	return nil, errors.New("payments wave7 rows do not begin transactions")
}

func (c paymentsWave7RowsConn) QueryContext(context.Context, string, []driver.NamedValue) (driver.Rows, error) {
	paymentsWave7RowsMu.Lock()
	rowSet, ok := paymentsWave7RowsByDSN[c.dsn]
	paymentsWave7RowsMu.Unlock()
	if !ok {
		return nil, fmt.Errorf("payments wave7 row set %q not found", c.dsn)
	}
	return &paymentsWave7SQLRows{
		columns: append([]string(nil), rowSet.columns...),
		values:  append([][]driver.Value(nil), rowSet.values...),
	}, nil
}

type paymentsWave7SQLRows struct {
	columns []string
	values  [][]driver.Value
	index   int
}

func (r *paymentsWave7SQLRows) Columns() []string {
	return append([]string(nil), r.columns...)
}

func (*paymentsWave7SQLRows) Close() error {
	return nil
}

func (r *paymentsWave7SQLRows) Next(dest []driver.Value) error {
	if r.index >= len(r.values) {
		return io.EOF
	}
	copy(dest, r.values[r.index])
	r.index++
	return nil
}

func TestGORMRepositoryWave7GetUnallocatedPaymentsMapsRows(t *testing.T) {
	ctx := context.Background()
	schemaName := "tenant_payments"
	tenantID := "tenant-1"
	paymentDate := time.Date(2026, time.June, 25, 12, 0, 0, 0, time.UTC)
	repo := NewGORMRepository(newPaymentsDryRunDB(t, withPaymentsWave7ScanRows(paymentsWave7RowSet{
		columns: []string{"id", "tenant_id", "payment_number", "payment_type", "payment_date", "amount", "currency", "exchange_rate", "base_amount", "created_at"},
		values: [][]driver.Value{{
			"payment-1",
			tenantID,
			"PMT-00001",
			string(PaymentTypeReceived),
			paymentDate,
			"122.00",
			"EUR",
			"1.00",
			"122.00",
			paymentDate,
		}},
	})))

	payments, err := repo.GetUnallocatedPayments(ctx, schemaName, tenantID, PaymentTypeReceived)

	require.NoError(t, err)
	require.Len(t, payments, 1)
	assert.Equal(t, "payment-1", payments[0].ID)
	assert.Equal(t, "PMT-00001", payments[0].PaymentNumber)
}
