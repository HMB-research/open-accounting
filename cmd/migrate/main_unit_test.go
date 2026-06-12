package main

import (
	"context"
	"errors"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestMigrationFileDiscoveryUnit(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{
		"002_second.up.sql",
		"001_first.up.sql",
		"003_third.down.sql",
		"ignored.txt",
	} {
		writeUnitMigration(t, dir, name, "-- noop")
	}

	upFiles, err := getMigrationFiles(dir, ".up")
	if err != nil {
		t.Fatalf("getMigrationFiles failed: %v", err)
	}
	if len(upFiles) != 2 {
		t.Fatalf("expected 2 up migrations, got %d", len(upFiles))
	}
	if !strings.HasSuffix(upFiles[0], "001_first.up.sql") || !strings.HasSuffix(upFiles[1], "002_second.up.sql") {
		t.Fatalf("expected sorted up migrations, got %v", upFiles)
	}

	if version := extractVersion(filepath.Join(dir, "001_first.up.sql")); version != "001_first" {
		t.Fatalf("unexpected up version: %s", version)
	}
	if version := extractVersion(filepath.Join(dir, "003_third.down.sql")); version != "003_third" {
		t.Fatalf("unexpected down version: %s", version)
	}
}

func TestEnsureAndGetAppliedMigrationsUnit(t *testing.T) {
	ctx := context.Background()
	pool := newFakeMigrationPool(map[string]bool{
		"001_initial": true,
		"002_next":    true,
	})

	if err := ensureMigrationsTable(ctx, pool); err != nil {
		t.Fatalf("ensureMigrationsTable failed: %v", err)
	}
	if len(pool.execs) != 1 || !strings.Contains(pool.execs[0], "CREATE TABLE IF NOT EXISTS schema_migrations") {
		t.Fatalf("expected migrations table DDL, got %v", pool.execs)
	}

	applied, err := getAppliedMigrations(ctx, pool)
	if err != nil {
		t.Fatalf("getAppliedMigrations failed: %v", err)
	}
	if !applied["001_initial"] || !applied["002_next"] {
		t.Fatalf("expected applied migrations to be returned, got %v", applied)
	}

	pool.rowsErr = errors.New("row iteration failed")
	if _, err := getAppliedMigrations(ctx, pool); err == nil || !strings.Contains(err.Error(), "row iteration failed") {
		t.Fatalf("expected rows.Err to be returned, got %v", err)
	}
}

func TestMigrateUpHonorsStepsSkipsAndNoopsUnit(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	writeUnitMigration(t, dir, "001_initial.up.sql", "SELECT 1;")
	writeUnitMigration(t, dir, "002_second.up.sql", "SELECT 2;")
	writeUnitMigration(t, dir, "003_third.up.sql", "SELECT 3;")

	pool := newFakeMigrationPool(map[string]bool{"001_initial": true})
	if err := migrateUp(ctx, pool, dir, 1); err != nil {
		t.Fatalf("migrateUp step-limited failed: %v", err)
	}
	if !pool.applied["002_second"] {
		t.Fatalf("expected second migration to be applied, got %v", pool.applied)
	}
	if pool.applied["003_third"] {
		t.Fatalf("expected third migration to remain pending with steps=1")
	}
	if got := len(pool.txs); got != 1 {
		t.Fatalf("expected 1 transaction, got %d", got)
	}
	if !pool.txs[0].committed || pool.txs[0].rolledBack {
		t.Fatalf("expected first transaction to commit cleanly: %+v", pool.txs[0])
	}

	if err := migrateUp(ctx, pool, dir, 0); err != nil {
		t.Fatalf("migrateUp remaining migrations failed: %v", err)
	}
	if !pool.applied["003_third"] {
		t.Fatalf("expected third migration to be applied, got %v", pool.applied)
	}

	txCount := len(pool.txs)
	if err := migrateUp(ctx, pool, dir, 0); err != nil {
		t.Fatalf("migrateUp no-op failed: %v", err)
	}
	if len(pool.txs) != txCount {
		t.Fatalf("expected no transaction for already-applied migrations")
	}
}

func TestMigrateUpRollsBackFailedMigrationUnit(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	writeUnitMigration(t, dir, "001_bad.up.sql", "SELECT fail_up;")

	pool := newFakeMigrationPool(nil)
	pool.txErrContains = "fail_up"
	pool.txErr = errors.New("boom")

	err := migrateUp(ctx, pool, dir, 0)
	if err == nil || !strings.Contains(err.Error(), "execute migration 001_bad") {
		t.Fatalf("expected execute migration error, got %v", err)
	}
	if pool.applied["001_bad"] {
		t.Fatalf("failed migration should not be recorded as applied")
	}
	if got := len(pool.txs); got != 1 {
		t.Fatalf("expected 1 transaction, got %d", got)
	}
	if !pool.txs[0].rolledBack || pool.txs[0].committed {
		t.Fatalf("expected failed migration to roll back: %+v", pool.txs[0])
	}
}

func TestMigrateDownDefaultsSkipsAndNoopsUnit(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	writeUnitMigration(t, dir, "001_initial.down.sql", "SELECT down_one;")
	writeUnitMigration(t, dir, "002_second.down.sql", "SELECT down_two;")
	writeUnitMigration(t, dir, "003_unapplied.down.sql", "SELECT ignored;")

	pool := newFakeMigrationPool(map[string]bool{
		"001_initial": true,
		"002_second":  true,
	})
	if err := migrateDown(ctx, pool, dir, 0); err != nil {
		t.Fatalf("migrateDown default step failed: %v", err)
	}
	if pool.applied["002_second"] {
		t.Fatalf("expected newest applied migration to be rolled back first")
	}
	if !pool.applied["001_initial"] {
		t.Fatalf("expected default rollback to leave older migration applied")
	}

	if err := migrateDown(ctx, pool, dir, 5); err != nil {
		t.Fatalf("migrateDown remaining migrations failed: %v", err)
	}
	if pool.applied["001_initial"] {
		t.Fatalf("expected remaining applied migration to be rolled back")
	}

	txCount := len(pool.txs)
	if err := migrateDown(ctx, pool, dir, 5); err != nil {
		t.Fatalf("migrateDown no-op failed: %v", err)
	}
	if len(pool.txs) != txCount {
		t.Fatalf("expected no transaction for unapplied down migrations")
	}
}

func TestMigrateDownRollsBackFailedRollbackUnit(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	writeUnitMigration(t, dir, "001_bad.down.sql", "SELECT fail_down;")

	pool := newFakeMigrationPool(map[string]bool{"001_bad": true})
	pool.txErrContains = "fail_down"
	pool.txErr = errors.New("boom")

	err := migrateDown(ctx, pool, dir, 1)
	if err == nil || !strings.Contains(err.Error(), "execute rollback 001_bad") {
		t.Fatalf("expected execute rollback error, got %v", err)
	}
	if !pool.applied["001_bad"] {
		t.Fatalf("failed rollback should leave migration recorded as applied")
	}
	if got := len(pool.txs); got != 1 {
		t.Fatalf("expected 1 transaction, got %d", got)
	}
	if !pool.txs[0].rolledBack || pool.txs[0].committed {
		t.Fatalf("expected failed rollback to roll back: %+v", pool.txs[0])
	}
}

func TestMainRequiresDatabaseURLUnit(t *testing.T) {
	cmd := exec.Command(os.Args[0], "-test.run=TestMainUnitHelperProcess", "--")
	cmd.Env = []string{"GO_WANT_MIGRATE_UNIT_HELPER=1"}
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected helper to fail without db url, output: %s", string(out))
	}
	if !strings.Contains(string(out), "Database URL required") {
		t.Fatalf("expected missing database error, got: %s", string(out))
	}
}

func TestMainUnitHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_MIGRATE_UNIT_HELPER") != "1" {
		return
	}

	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	for i, arg := range os.Args {
		if arg == "--" {
			os.Args = append([]string{os.Args[0]}, os.Args[i+1:]...)
			break
		}
	}
	main()
	os.Exit(0)
}

func writeUnitMigration(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write migration %s: %v", name, err)
	}
}

type fakeMigrationPool struct {
	applied       map[string]bool
	execs         []string
	txs           []*fakeMigrationTx
	execErr       error
	queryErr      error
	rowsErr       error
	beginErr      error
	txErrContains string
	txErr         error
	commitErr     error
}

func newFakeMigrationPool(applied map[string]bool) *fakeMigrationPool {
	copied := make(map[string]bool)
	for version, ok := range applied {
		copied[version] = ok
	}
	return &fakeMigrationPool{applied: copied}
}

func (p *fakeMigrationPool) Exec(_ context.Context, sql string, _ ...any) (pgconn.CommandTag, error) {
	p.execs = append(p.execs, sql)
	if p.execErr != nil {
		return pgconn.CommandTag{}, p.execErr
	}
	return pgconn.CommandTag{}, nil
}

func (p *fakeMigrationPool) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	if p.queryErr != nil {
		return nil, p.queryErr
	}

	versions := make([]string, 0, len(p.applied))
	for version, applied := range p.applied {
		if applied {
			versions = append(versions, version)
		}
	}
	sort.Strings(versions)

	return &fakeMigrationRows{versions: versions, err: p.rowsErr}, nil
}

func (p *fakeMigrationPool) Begin(context.Context) (pgx.Tx, error) {
	if p.beginErr != nil {
		return nil, p.beginErr
	}

	tx := &fakeMigrationTx{pool: p}
	p.txs = append(p.txs, tx)
	return tx, nil
}

type fakeMigrationRows struct {
	versions []string
	index    int
	closed   bool
	err      error
}

func (r *fakeMigrationRows) Close() {
	r.closed = true
}

func (r *fakeMigrationRows) Err() error {
	return r.err
}

func (r *fakeMigrationRows) CommandTag() pgconn.CommandTag {
	return pgconn.CommandTag{}
}

func (r *fakeMigrationRows) FieldDescriptions() []pgconn.FieldDescription {
	return nil
}

func (r *fakeMigrationRows) Next() bool {
	if r.index >= len(r.versions) {
		r.closed = true
		return false
	}
	r.index++
	return true
}

func (r *fakeMigrationRows) Scan(dest ...any) error {
	if len(dest) != 1 {
		return errors.New("expected one scan destination")
	}

	version, ok := dest[0].(*string)
	if !ok {
		return errors.New("expected string scan destination")
	}
	*version = r.versions[r.index-1]
	return nil
}

func (r *fakeMigrationRows) Values() ([]any, error) {
	return []any{r.versions[r.index-1]}, nil
}

func (r *fakeMigrationRows) RawValues() [][]byte {
	return [][]byte{[]byte(r.versions[r.index-1])}
}

func (r *fakeMigrationRows) Conn() *pgx.Conn {
	return nil
}

type fakeMigrationTx struct {
	pool       *fakeMigrationPool
	execs      []string
	committed  bool
	rolledBack bool
}

func (tx *fakeMigrationTx) Begin(context.Context) (pgx.Tx, error) {
	return nil, errors.New("nested transactions are not implemented")
}

func (tx *fakeMigrationTx) Commit(context.Context) error {
	tx.committed = true
	return tx.pool.commitErr
}

func (tx *fakeMigrationTx) Rollback(context.Context) error {
	tx.rolledBack = true
	return nil
}

func (tx *fakeMigrationTx) CopyFrom(context.Context, pgx.Identifier, []string, pgx.CopyFromSource) (int64, error) {
	return 0, errors.New("copy is not implemented")
}

func (tx *fakeMigrationTx) SendBatch(context.Context, *pgx.Batch) pgx.BatchResults {
	return nil
}

func (tx *fakeMigrationTx) LargeObjects() pgx.LargeObjects {
	return pgx.LargeObjects{}
}

func (tx *fakeMigrationTx) Prepare(context.Context, string, string) (*pgconn.StatementDescription, error) {
	return nil, errors.New("prepare is not implemented")
}

func (tx *fakeMigrationTx) Exec(_ context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
	tx.execs = append(tx.execs, sql)
	if tx.pool.txErrContains != "" && strings.Contains(sql, tx.pool.txErrContains) {
		return pgconn.CommandTag{}, tx.pool.txErr
	}

	switch strings.TrimSpace(sql) {
	case "INSERT INTO schema_migrations (version) VALUES ($1)":
		tx.pool.applied[arguments[0].(string)] = true
	case "DELETE FROM schema_migrations WHERE version = $1":
		tx.pool.applied[arguments[0].(string)] = false
	}

	return pgconn.CommandTag{}, nil
}

func (tx *fakeMigrationTx) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, errors.New("query is not implemented")
}

func (tx *fakeMigrationTx) QueryRow(context.Context, string, ...any) pgx.Row {
	return fakeMigrationRow{}
}

func (tx *fakeMigrationTx) Conn() *pgx.Conn {
	return nil
}

type fakeMigrationRow struct{}

func (fakeMigrationRow) Scan(...any) error {
	return errors.New("query row is not implemented")
}
