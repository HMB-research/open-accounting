package database

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeDBTX struct {
	lastExecQuery    string
	lastExecArgs     []any
	lastQuery        string
	lastQueryArgs    []any
	lastQueryRow     string
	lastQueryRowArgs []any

	rows pgx.Rows
	row  pgx.Row

	execErr  error
	queryErr error
}

func (f *fakeDBTX) Exec(_ context.Context, query string, args ...interface{}) (pgconn.CommandTag, error) {
	f.lastExecQuery = query
	f.lastExecArgs = args
	return pgconn.CommandTag{}, f.execErr
}

func (f *fakeDBTX) Query(_ context.Context, query string, args ...interface{}) (pgx.Rows, error) {
	f.lastQuery = query
	f.lastQueryArgs = args
	if f.queryErr != nil {
		return nil, f.queryErr
	}
	return f.rows, nil
}

func (f *fakeDBTX) QueryRow(_ context.Context, query string, args ...interface{}) pgx.Row {
	f.lastQueryRow = query
	f.lastQueryRowArgs = args
	return f.row
}

type fakeRow struct {
	values []any
	err    error
}

func (r fakeRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	for i := range dest {
		assignScanValue(dest[i], r.values[i])
	}
	return nil
}

type fakeRows struct {
	records [][]any
	idx     int
	err     error
}

func (r *fakeRows) Close() {}

func (r *fakeRows) Err() error {
	return r.err
}

func (r *fakeRows) CommandTag() pgconn.CommandTag {
	return pgconn.CommandTag{}
}

func (r *fakeRows) FieldDescriptions() []pgconn.FieldDescription {
	return nil
}

func (r *fakeRows) Next() bool {
	if r.idx >= len(r.records) {
		return false
	}
	r.idx++
	return true
}

func (r *fakeRows) Scan(dest ...any) error {
	if r.idx == 0 || r.idx > len(r.records) {
		return errors.New("scan called without current row")
	}
	for i := range dest {
		assignScanValue(dest[i], r.records[r.idx-1][i])
	}
	return nil
}

func (r *fakeRows) Values() ([]any, error) {
	if r.idx == 0 || r.idx > len(r.records) {
		return nil, errors.New("values called without current row")
	}
	return r.records[r.idx-1], nil
}

func (r *fakeRows) RawValues() [][]byte {
	return nil
}

func (r *fakeRows) Conn() *pgx.Conn {
	return nil
}

func assignScanValue(dest any, value any) {
	reflect.ValueOf(dest).Elem().Set(reflect.ValueOf(value))
}

func sampleTenantRow(id uuid.UUID) []any {
	ts := pgtype.Timestamptz{Time: time.Date(2026, 3, 12, 10, 0, 0, 0, time.UTC), Valid: true}
	return []any{
		id,
		"Acme",
		"acme",
		"tenant_acme",
		json.RawMessage(`{"currency":"EUR"}`),
		true,
		ts,
		ts,
	}
}

func sampleUserRow(id uuid.UUID) []any {
	ts := pgtype.Timestamptz{Time: time.Date(2026, 3, 12, 10, 0, 0, 0, time.UTC), Valid: true}
	return []any{
		id,
		"user@example.com",
		"hashed",
		"Test User",
		true,
		ts,
		ts,
	}
}

func sampleVATRow(id uuid.UUID) []any {
	date := pgtype.Date{Time: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), Valid: true}
	ts := pgtype.Timestamptz{Time: time.Date(2026, 3, 12, 10, 0, 0, 0, time.UTC), Valid: true}
	return []any{
		id,
		"EE",
		"STANDARD",
		decimal.RequireFromString("22"),
		"Standard VAT",
		date,
		pgtype.Date{},
		ts,
	}
}

func TestQueriesConstructors(t *testing.T) {
	db := &fakeDBTX{}
	q := New(db)
	require.NotNil(t, q)
	assert.Same(t, db, q.db)

	txQueries := q.WithTx(nil)
	require.NotNil(t, txQueries)
	assert.Nil(t, txQueries.db)
}

func TestTenantQueries(t *testing.T) {
	ctx := context.Background()
	tenantID := uuid.New()
	db := &fakeDBTX{}
	q := New(db)

	db.row = fakeRow{values: sampleTenantRow(tenantID)}
	created, err := q.CreateTenant(ctx, &CreateTenantParams{
		Name: "Acme", Slug: "acme", SchemaName: "tenant_acme", Settings: json.RawMessage(`{"currency":"EUR"}`),
	})
	require.NoError(t, err)
	assert.Equal(t, tenantID, created.ID)
	assert.Contains(t, db.lastQueryRow, "INSERT INTO tenants")
	assert.Equal(t, "Acme", db.lastQueryRowArgs[0])

	require.NoError(t, q.DeactivateTenant(ctx, tenantID))
	assert.Contains(t, db.lastExecQuery, "UPDATE tenants")
	assert.Equal(t, tenantID, db.lastExecArgs[0])

	db.row = fakeRow{values: sampleTenantRow(tenantID)}
	got, err := q.GetTenant(ctx, tenantID)
	require.NoError(t, err)
	assert.Equal(t, "acme", got.Slug)

	db.row = fakeRow{values: sampleTenantRow(tenantID)}
	got, err = q.GetTenantBySchemaName(ctx, "tenant_acme")
	require.NoError(t, err)
	assert.Equal(t, "tenant_acme", got.SchemaName)

	db.row = fakeRow{values: sampleTenantRow(tenantID)}
	got, err = q.GetTenantBySlug(ctx, "acme")
	require.NoError(t, err)
	assert.Equal(t, tenantID, got.ID)

	db.rows = &fakeRows{records: [][]any{
		sampleTenantRow(tenantID),
		sampleTenantRow(uuid.New()),
	}}
	list, err := q.ListTenants(ctx)
	require.NoError(t, err)
	assert.Len(t, list, 2)

	db.row = fakeRow{values: []any{true}}
	exists, err := q.SlugExists(ctx, "acme")
	require.NoError(t, err)
	assert.True(t, exists)

	db.row = fakeRow{values: sampleTenantRow(tenantID)}
	updated, err := q.UpdateTenant(ctx, &UpdateTenantParams{
		ID: tenantID, Name: "Acme Updated", Settings: json.RawMessage(`{"currency":"USD"}`),
	})
	require.NoError(t, err)
	assert.Equal(t, tenantID, updated.ID)

	db.row = fakeRow{values: sampleTenantRow(tenantID)}
	updated, err = q.UpdateTenantSettings(ctx, &UpdateTenantSettingsParams{
		ID: tenantID, Settings: json.RawMessage(`{"currency":"GBP"}`),
	})
	require.NoError(t, err)
	assert.Equal(t, "Acme", updated.Name)
}

func TestUserQueries(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	tenantID := uuid.New()
	db := &fakeDBTX{}
	q := New(db)

	require.NoError(t, q.AddUserToTenant(ctx, &AddUserToTenantParams{
		TenantID: tenantID, UserID: userID, Role: "admin", IsDefault: true,
	}))
	assert.Contains(t, db.lastExecQuery, "INSERT INTO tenant_users")
	assert.Equal(t, tenantID, db.lastExecArgs[0])

	db.row = fakeRow{values: sampleUserRow(userID)}
	user, err := q.CreateUser(ctx, &CreateUserParams{
		Email: "user@example.com", PasswordHash: "hashed", Name: "Test User",
	})
	require.NoError(t, err)
	assert.Equal(t, userID, user.ID)

	require.NoError(t, q.DeactivateUser(ctx, userID))
	assert.Contains(t, db.lastExecQuery, "UPDATE users")

	db.row = fakeRow{values: []any{true}}
	emailExists, err := q.EmailExists(ctx, "user@example.com")
	require.NoError(t, err)
	assert.True(t, emailExists)

	db.row = fakeRow{values: sampleUserRow(userID)}
	user, err = q.GetUser(ctx, userID)
	require.NoError(t, err)
	assert.Equal(t, "Test User", user.Name)

	db.row = fakeRow{values: sampleUserRow(userID)}
	user, err = q.GetUserByEmail(ctx, "user@example.com")
	require.NoError(t, err)
	assert.Equal(t, "user@example.com", user.Email)

	db.row = fakeRow{values: []any{"owner"}}
	role, err := q.GetUserRole(ctx, &GetUserRoleParams{TenantID: tenantID, UserID: userID})
	require.NoError(t, err)
	assert.Equal(t, "owner", role)

	ts := pgtype.Timestamptz{Time: time.Date(2026, 3, 12, 10, 0, 0, 0, time.UTC), Valid: true}
	db.rows = &fakeRows{records: [][]any{{
		tenantID, "Acme", "acme", "tenant_acme", json.RawMessage(`{}`), true, ts, ts, "admin", true,
	}}}
	tenants, err := q.GetUserTenants(ctx, userID)
	require.NoError(t, err)
	assert.Len(t, tenants, 1)
	assert.Equal(t, "admin", tenants[0].Role)

	require.NoError(t, q.RemoveUserFromTenant(ctx, &RemoveUserFromTenantParams{
		TenantID: tenantID, UserID: userID,
	}))
	assert.Contains(t, db.lastExecQuery, "DELETE FROM tenant_users")

	require.NoError(t, q.SetDefaultTenant(ctx, &SetDefaultTenantParams{
		UserID: userID, TenantID: tenantID,
	}))
	assert.Contains(t, db.lastExecQuery, "UPDATE tenant_users")

	db.row = fakeRow{values: sampleUserRow(userID)}
	user, err = q.UpdateUser(ctx, &UpdateUserParams{ID: userID, Name: "Updated"})
	require.NoError(t, err)
	assert.Equal(t, userID, user.ID)

	require.NoError(t, q.UpdateUserPassword(ctx, &UpdateUserPasswordParams{
		ID: userID, PasswordHash: "new-hash",
	}))
	assert.Contains(t, db.lastExecQuery, "UPDATE users")

	db.row = fakeRow{values: []any{true}}
	hasAccess, err := q.UserHasTenantAccess(ctx, &UserHasTenantAccessParams{
		TenantID: tenantID, UserID: userID,
	})
	require.NoError(t, err)
	assert.True(t, hasAccess)
}

func TestVATRateQueries(t *testing.T) {
	ctx := context.Background()
	rateID := uuid.New()
	db := &fakeDBTX{}
	q := New(db)

	date := pgtype.Date{Time: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), Valid: true}

	db.row = fakeRow{values: sampleVATRow(rateID)}
	rate, err := q.CreateVATRate(ctx, &CreateVATRateParams{
		CountryCode: "EE",
		RateType:    "STANDARD",
		Rate:        decimal.RequireFromString("22"),
		Name:        "Standard VAT",
		ValidFrom:   date,
	})
	require.NoError(t, err)
	assert.Equal(t, rateID, rate.ID)

	db.row = fakeRow{values: sampleVATRow(rateID)}
	rate, err = q.GetCurrentVATRate(ctx, &GetCurrentVATRateParams{
		CountryCode: "EE",
		RateType:    "STANDARD",
		ValidFrom:   date,
	})
	require.NoError(t, err)
	assert.Equal(t, "STANDARD", rate.RateType)

	db.row = fakeRow{values: sampleVATRow(rateID)}
	rate, err = q.GetVATRate(ctx, rateID)
	require.NoError(t, err)
	assert.Equal(t, "EE", rate.CountryCode)

	db.rows = &fakeRows{records: [][]any{
		sampleVATRow(rateID),
		sampleVATRow(uuid.New()),
	}}
	rates, err := q.ListCurrentVATRates(ctx, "EE")
	require.NoError(t, err)
	assert.Len(t, rates, 2)

	db.rows = &fakeRows{records: [][]any{
		sampleVATRow(rateID),
	}}
	rates, err = q.ListVATRates(ctx, "EE")
	require.NoError(t, err)
	assert.Len(t, rates, 1)

	db.row = fakeRow{values: sampleVATRow(rateID)}
	rate, err = q.UpdateVATRate(ctx, &UpdateVATRateParams{
		ID: rateID, Rate: decimal.RequireFromString("24"), Name: "Updated", ValidTo: date,
	})
	require.NoError(t, err)
	assert.Equal(t, rateID, rate.ID)
}

func TestQueriesPropagateDatabaseErrors(t *testing.T) {
	ctx := context.Background()
	db := &fakeDBTX{
		row:      fakeRow{err: pgx.ErrNoRows},
		execErr:  errors.New("exec failed"),
		queryErr: errors.New("query failed"),
	}
	q := New(db)

	_, err := q.GetTenant(ctx, uuid.New())
	require.ErrorIs(t, err, pgx.ErrNoRows)

	err = q.DeactivateTenant(ctx, uuid.New())
	require.EqualError(t, err, "exec failed")

	_, err = q.ListTenants(ctx)
	require.EqualError(t, err, "query failed")
}
