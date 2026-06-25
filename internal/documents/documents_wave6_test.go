package documents

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestDocumentsWave6ListReviewSummariesMapsScanRows(t *testing.T) {
	entityID := "invoice-1"
	db := newDocumentsWave6GORMDB(t, documentsWave6Query{
		contains: []string{`FROM "tenant_schema"."documents"`, "COUNT(*) FILTER", "GROUP BY"},
		columns:  []string{"entity_id", "total_count", "pending_review_count", "reviewed_count", "approved_count", "rejected_count"},
		rows: [][]driver.Value{
			{entityID, int64(4), int64(1), int64(3), int64(2), int64(1)},
		},
	})
	repo := NewGORMRepository(db)

	summaries, err := repo.ListReviewSummaries(context.Background(), "tenant_schema", "tenant-1", EntityTypeInvoice, []string{entityID})

	if err != nil {
		t.Fatalf("ListReviewSummaries returned error: %v", err)
	}
	summary, ok := summaries[entityID]
	if !ok {
		t.Fatalf("expected summary for %q, got %#v", entityID, summaries)
	}
	if summary.EntityType != EntityTypeInvoice || summary.EntityID != entityID {
		t.Fatalf("unexpected summary identity: %#v", summary)
	}
	if summary.TotalCount != 4 || summary.PendingReviewCount != 1 || summary.ReviewedCount != 3 || summary.ApprovedCount != 2 || summary.RejectedCount != 1 {
		t.Fatalf("unexpected summary counts: %#v", summary)
	}
	if summary.MissingEvidence {
		t.Fatal("expected evidence to be present")
	}
	if !summary.HasPendingReview {
		t.Fatal("expected pending review flag")
	}
	if !summary.HasRejected {
		t.Fatal("expected rejected flag")
	}
}

type documentsWave6Query struct {
	contains []string
	columns  []string
	rows     [][]driver.Value
	err      error
}

type documentsWave6StubDB struct {
	mu      sync.Mutex
	queries []documentsWave6Query
}

func newDocumentsWave6GORMDB(t *testing.T, queries ...documentsWave6Query) *gorm.DB {
	t.Helper()

	stub := &documentsWave6StubDB{queries: append([]documentsWave6Query(nil), queries...)}
	sqlDB := sql.OpenDB(documentsWave6Connector{stub: stub})
	t.Cleanup(func() { _ = sqlDB.Close() })

	db, err := gorm.Open(postgres.New(postgres.Config{
		Conn:                 sqlDB,
		PreferSimpleProtocol: true,
	}), &gorm.Config{
		DisableAutomaticPing:   true,
		Logger:                 logger.Default.LogMode(logger.Silent),
		SkipDefaultTransaction: true,
	})
	if err != nil {
		t.Fatalf("open documents wave6 gorm db: %v", err)
	}
	return db
}

func (s *documentsWave6StubDB) query(query string) (driver.Rows, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.queries) == 0 {
		return nil, fmt.Errorf("unexpected documents query: %s", query)
	}
	expected := s.queries[0]
	s.queries = s.queries[1:]
	for _, token := range expected.contains {
		if !strings.Contains(query, token) {
			return nil, fmt.Errorf("query missing %q in %s", token, query)
		}
	}
	if expected.err != nil {
		return nil, expected.err
	}
	return &documentsWave6Rows{
		columns: append([]string(nil), expected.columns...),
		rows:    cloneDocumentsWave6Rows(expected.rows),
	}, nil
}

type documentsWave6Connector struct {
	stub *documentsWave6StubDB
}

func (c documentsWave6Connector) Connect(context.Context) (driver.Conn, error) {
	return documentsWave6Conn{stub: c.stub}, nil
}

func (documentsWave6Connector) Driver() driver.Driver {
	return documentsWave6Driver{}
}

type documentsWave6Driver struct{}

func (documentsWave6Driver) Open(string) (driver.Conn, error) {
	return nil, errors.New("documents wave6 driver requires connector")
}

type documentsWave6Conn struct {
	stub *documentsWave6StubDB
}

func (documentsWave6Conn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("documents wave6 rows do not prepare statements")
}

func (documentsWave6Conn) Close() error {
	return nil
}

func (documentsWave6Conn) Begin() (driver.Tx, error) {
	return nil, errors.New("documents wave6 rows do not begin transactions")
}

func (c documentsWave6Conn) QueryContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	return c.stub.query(query)
}

type documentsWave6Rows struct {
	columns []string
	rows    [][]driver.Value
	index   int
}

func (r *documentsWave6Rows) Columns() []string {
	return append([]string(nil), r.columns...)
}

func (*documentsWave6Rows) Close() error {
	return nil
}

func (r *documentsWave6Rows) Next(dest []driver.Value) error {
	if r.index >= len(r.rows) {
		return io.EOF
	}
	copy(dest, r.rows[r.index])
	r.index++
	return nil
}

func cloneDocumentsWave6Rows(rows [][]driver.Value) [][]driver.Value {
	clone := make([][]driver.Value, len(rows))
	for i := range rows {
		clone[i] = append([]driver.Value(nil), rows[i]...)
	}
	return clone
}
