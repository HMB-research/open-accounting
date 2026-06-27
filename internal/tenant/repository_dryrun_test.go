package tenant

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/HMB-research/open-accounting/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type tenantDryRunConnPool struct{}

func (tenantDryRunConnPool) PrepareContext(context.Context, string) (*sql.Stmt, error) {
	return nil, errors.New("dry-run tenant tests should not prepare statements")
}

func (tenantDryRunConnPool) ExecContext(context.Context, string, ...interface{}) (sql.Result, error) {
	return nil, errors.New("dry-run tenant tests should not execute statements")
}

func (tenantDryRunConnPool) QueryContext(context.Context, string, ...interface{}) (*sql.Rows, error) {
	return nil, errors.New("dry-run tenant tests should not query rows")
}

func (tenantDryRunConnPool) QueryRowContext(context.Context, string, ...interface{}) *sql.Row {
	return nil
}

func (tenantDryRunConnPool) BeginTx(context.Context, *sql.TxOptions) (gorm.ConnPool, error) {
	return &tenantDryRunTx{}, nil
}

type tenantDryRunTx struct {
	tenantDryRunConnPool
}

func (*tenantDryRunTx) Commit() error {
	return nil
}

func (*tenantDryRunTx) Rollback() error {
	return nil
}

type tenantDryRunDBOption func(t *testing.T, db *gorm.DB)

type tenantDryRunFixture struct {
	tenants          []models.Tenant
	tenantIndex      int
	users            []models.User
	userIndex        int
	tenantUsers      []models.TenantUserModel
	tenantUserIndex  int
	invitations      []models.UserInvitation
	invitationIndex  int
	auditEvents      []models.TenantAuditEvent
	periodCloses     []periodCloseEventModel
	periodCloseIndex int
	userMemberships  []tenantDryRunMembership
	role             string
	count            int64
	countSet         bool
}

type tenantDryRunMembership struct {
	tenant    models.Tenant
	role      string
	isDefault bool
}

type tenantDryRunRowSet struct {
	columns []string
	values  [][]driver.Value
}

var tenantDryRunCallbackID uint64
var tenantDryRunRowsDSNID uint64
var tenantDryRunRowsDriverOnce sync.Once
var tenantDryRunRowsMu sync.Mutex
var tenantDryRunRowsByDSN = map[string]tenantDryRunRowSet{}

func newTenantDryRunDB(t *testing.T, opts ...tenantDryRunDBOption) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(postgres.New(postgres.Config{Conn: tenantDryRunConnPool{}}), &gorm.Config{
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

func withTenantDryRunFixtures(fixture tenantDryRunFixture) tenantDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		err := db.Callback().Query().After("gorm:query").Register(tenantDryRunCallbackName("query_fixtures"), func(tx *gorm.DB) {
			if populateTenantDryRunDest(tx, tx.Statement.Dest, &fixture) && tx.RowsAffected == 0 {
				tx.RowsAffected = 1
			}
		})
		require.NoError(t, err)
	}
}

func withTenantDryRunCreateError(expectedErr error) tenantDryRunDBOption {
	return withTenantDryRunCreateErrors(expectedErr)
}

func withTenantDryRunCreateErrors(expectedErrs ...error) tenantDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		var index int
		err := db.Callback().Create().Before("gorm:create").Register(tenantDryRunCallbackName("create_error"), func(tx *gorm.DB) {
			if len(expectedErrs) == 0 {
				return
			}
			expectedErr := expectedErrs[len(expectedErrs)-1]
			if index < len(expectedErrs) {
				expectedErr = expectedErrs[index]
			}
			index++
			if expectedErr != nil {
				tx.AddError(expectedErr)
			}
		})
		require.NoError(t, err)
	}
}

func withTenantDryRunQueryError(expectedErr error) tenantDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		err := db.Callback().Query().Before("gorm:query").Register(tenantDryRunCallbackName("query_error"), func(tx *gorm.DB) {
			tx.AddError(expectedErr)
		})
		require.NoError(t, err)
	}
}

func withTenantDryRunScanRows(rowSets ...tenantDryRunRowSet) tenantDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		var index int
		err := db.Callback().Row().After("gorm:row").Register(tenantDryRunCallbackName("scan_rows"), func(tx *gorm.DB) {
			if index >= len(rowSets) {
				tx.AddError(fmt.Errorf("missing tenant dry-run row set %d", index))
				return
			}
			rowSet := rowSets[index]
			index++
			tx.Statement.Dest = newTenantDryRunSQLRows(t, rowSet)
			tx.RowsAffected = int64(len(rowSet.values))
		})
		require.NoError(t, err)
	}
}

func withTenantDryRunRawError(expectedErr error) tenantDryRunDBOption {
	return withTenantDryRunRawErrors(expectedErr)
}

func withTenantDryRunRawErrors(expectedErrs ...error) tenantDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		var index int
		err := db.Callback().Raw().Before("gorm:raw").Register(tenantDryRunCallbackName("raw_error"), func(tx *gorm.DB) {
			var expectedErr error
			if len(expectedErrs) > 0 {
				expectedErr = expectedErrs[len(expectedErrs)-1]
				if index < len(expectedErrs) {
					expectedErr = expectedErrs[index]
				}
				index++
			}
			if expectedErr != nil {
				tx.AddError(expectedErr)
			}
		})
		require.NoError(t, err)
	}
}

func withTenantDryRunUpdateRows(rows int64) tenantDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		err := db.Callback().Update().After("gorm:update").Register(tenantDryRunCallbackName("update_rows"), func(tx *gorm.DB) {
			tx.RowsAffected = rows
		})
		require.NoError(t, err)
	}
}

func withTenantDryRunUpdateError(expectedErr error) tenantDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		err := db.Callback().Update().Before("gorm:update").Register(tenantDryRunCallbackName("update_error"), func(tx *gorm.DB) {
			tx.AddError(expectedErr)
		})
		require.NoError(t, err)
	}
}

func withTenantDryRunDeleteRows(rows int64) tenantDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		err := db.Callback().Delete().After("gorm:delete").Register(tenantDryRunCallbackName("delete_rows"), func(tx *gorm.DB) {
			tx.RowsAffected = rows
		})
		require.NoError(t, err)
	}
}

func withTenantDryRunDeleteError(expectedErr error) tenantDryRunDBOption {
	return withTenantDryRunDeleteErrors(expectedErr)
}

func withTenantDryRunDeleteErrors(expectedErrs ...error) tenantDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		var index int
		err := db.Callback().Delete().Before("gorm:delete").Register(tenantDryRunCallbackName("delete_error"), func(tx *gorm.DB) {
			var expectedErr error
			if len(expectedErrs) > 0 {
				expectedErr = expectedErrs[len(expectedErrs)-1]
				if index < len(expectedErrs) {
					expectedErr = expectedErrs[index]
				}
				index++
			}
			if expectedErr != nil {
				tx.AddError(expectedErr)
			}
		})
		require.NoError(t, err)
	}
}

func tenantDryRunCallbackName(suffix string) string {
	id := atomic.AddUint64(&tenantDryRunCallbackID, 1)
	return fmt.Sprintf("tenant_dryrun:%d:%s", id, suffix)
}

func newTenantDryRunSQLRows(t *testing.T, rowSet tenantDryRunRowSet) *sql.Rows {
	t.Helper()

	tenantDryRunRowsDriverOnce.Do(func() {
		sql.Register("tenant_dryrun_rows", tenantDryRunRowsDriver{})
	})

	dsn := fmt.Sprintf("tenant-dry-run-rows-%d", atomic.AddUint64(&tenantDryRunRowsDSNID, 1))
	tenantDryRunRowsMu.Lock()
	tenantDryRunRowsByDSN[dsn] = rowSet
	tenantDryRunRowsMu.Unlock()

	db, err := sql.Open("tenant_dryrun_rows", dsn)
	require.NoError(t, err)
	rows, err := db.QueryContext(context.Background(), "SELECT 1")
	require.NoError(t, err)

	t.Cleanup(func() {
		_ = rows.Close()
		_ = db.Close()
		tenantDryRunRowsMu.Lock()
		delete(tenantDryRunRowsByDSN, dsn)
		tenantDryRunRowsMu.Unlock()
	})

	return rows
}

type tenantDryRunRowsDriver struct{}

func (tenantDryRunRowsDriver) Open(name string) (driver.Conn, error) {
	return tenantDryRunRowsConn{dsn: name}, nil
}

type tenantDryRunRowsConn struct {
	dsn string
}

func (tenantDryRunRowsConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("tenant dry-run rows do not prepare statements")
}

func (tenantDryRunRowsConn) Close() error {
	return nil
}

func (tenantDryRunRowsConn) Begin() (driver.Tx, error) {
	return nil, errors.New("tenant dry-run rows do not begin transactions")
}

func (c tenantDryRunRowsConn) QueryContext(context.Context, string, []driver.NamedValue) (driver.Rows, error) {
	tenantDryRunRowsMu.Lock()
	rowSet, ok := tenantDryRunRowsByDSN[c.dsn]
	tenantDryRunRowsMu.Unlock()
	if !ok {
		return nil, fmt.Errorf("tenant dry-run row set %q not found", c.dsn)
	}
	return &tenantDryRunSQLRows{
		columns: append([]string(nil), rowSet.columns...),
		values:  append([][]driver.Value(nil), rowSet.values...),
	}, nil
}

type tenantDryRunSQLRows struct {
	columns []string
	values  [][]driver.Value
	index   int
}

func (r *tenantDryRunSQLRows) Columns() []string {
	return append([]string(nil), r.columns...)
}

func (*tenantDryRunSQLRows) Close() error {
	return nil
}

func (r *tenantDryRunSQLRows) Next(dest []driver.Value) error {
	if r.index >= len(r.values) {
		return io.EOF
	}
	copy(dest, r.values[r.index])
	r.index++
	return nil
}

func populateTenantDryRunDest(tx *gorm.DB, dest any, fixture *tenantDryRunFixture) bool {
	switch typed := dest.(type) {
	case *models.Tenant:
		if len(fixture.tenants) == 0 {
			return false
		}
		*typed = fixture.tenants[fixture.nextTenantIndex()]
		return true
	case *[]models.TenantAuditEvent:
		*typed = append((*typed)[:0], fixture.auditEvents...)
		tx.RowsAffected = int64(len(fixture.auditEvents))
		return true
	case *periodCloseEventModel:
		if len(fixture.periodCloses) == 0 {
			return false
		}
		*typed = fixture.periodCloses[fixture.nextPeriodCloseIndex()]
		return true
	case *[]periodCloseEventModel:
		*typed = append((*typed)[:0], fixture.periodCloses...)
		tx.RowsAffected = int64(len(fixture.periodCloses))
		return true
	case *models.TenantUserModel:
		if len(fixture.tenantUsers) == 0 {
			return false
		}
		*typed = fixture.tenantUsers[fixture.nextTenantUserIndex()]
		return true
	case *[]models.TenantUserModel:
		*typed = append((*typed)[:0], fixture.tenantUsers...)
		tx.RowsAffected = int64(len(fixture.tenantUsers))
		return true
	case *models.User:
		if len(fixture.users) == 0 {
			return false
		}
		*typed = fixture.users[fixture.nextUserIndex()]
		return true
	case *models.UserInvitation:
		if len(fixture.invitations) == 0 {
			return false
		}
		*typed = fixture.invitations[fixture.nextInvitationIndex()]
		return true
	case *[]models.UserInvitation:
		*typed = append((*typed)[:0], fixture.invitations...)
		tx.RowsAffected = int64(len(fixture.invitations))
		return true
	case *string:
		*typed = fixture.role
		return true
	case *int64:
		if !fixture.countSet {
			return false
		}
		*typed = fixture.count
		return true
	default:
		return populateTenantDryRunReflectDest(tx, dest, fixture)
	}
}

func populateTenantDryRunReflectDest(tx *gorm.DB, dest any, fixture *tenantDryRunFixture) bool {
	value := reflect.ValueOf(dest)
	if value.Kind() != reflect.Pointer || value.IsNil() {
		return false
	}

	target := value.Elem()
	if target.Kind() != reflect.Slice {
		return false
	}
	elemType := target.Type().Elem()
	if _, ok := elemType.FieldByName("Tenant"); !ok {
		return false
	}
	if _, ok := elemType.FieldByName("Role"); !ok {
		return false
	}
	if _, ok := elemType.FieldByName("IsDefault"); !ok {
		return false
	}

	rows := reflect.MakeSlice(target.Type(), len(fixture.userMemberships), len(fixture.userMemberships))
	for i, membership := range fixture.userMemberships {
		row := rows.Index(i)
		tenantField := row.FieldByName("Tenant")
		if tenantField.IsValid() && tenantField.CanSet() && tenantField.Type() == reflect.TypeOf(models.Tenant{}) {
			tenantField.Set(reflect.ValueOf(membership.tenant))
		}
		tenantDryRunSetStringField(row, "Role", membership.role)
		tenantDryRunSetBoolField(row, "IsDefault", membership.isDefault)
	}
	target.Set(rows)
	tx.RowsAffected = int64(len(fixture.userMemberships))
	return true
}

func (f *tenantDryRunFixture) nextTenantIndex() int {
	index := f.tenantIndex
	if index >= len(f.tenants) {
		index = len(f.tenants) - 1
	}
	f.tenantIndex++
	return index
}

func (f *tenantDryRunFixture) nextUserIndex() int {
	index := f.userIndex
	if index >= len(f.users) {
		index = len(f.users) - 1
	}
	f.userIndex++
	return index
}

func (f *tenantDryRunFixture) nextTenantUserIndex() int {
	index := f.tenantUserIndex
	if index >= len(f.tenantUsers) {
		index = len(f.tenantUsers) - 1
	}
	f.tenantUserIndex++
	return index
}

func (f *tenantDryRunFixture) nextInvitationIndex() int {
	index := f.invitationIndex
	if index >= len(f.invitations) {
		index = len(f.invitations) - 1
	}
	f.invitationIndex++
	return index
}

func (f *tenantDryRunFixture) nextPeriodCloseIndex() int {
	index := f.periodCloseIndex
	if index >= len(f.periodCloses) {
		index = len(f.periodCloses) - 1
	}
	f.periodCloseIndex++
	return index
}

func tenantDryRunSetStringField(target reflect.Value, name string, value string) {
	field := target.FieldByName(name)
	if field.IsValid() && field.CanSet() && field.Kind() == reflect.String {
		field.SetString(value)
	}
}

func tenantDryRunSetBoolField(target reflect.Value, name string, value bool) {
	field := target.FieldByName(name)
	if field.IsValid() && field.CanSet() && field.Kind() == reflect.Bool {
		field.SetBool(value)
	}
}

func TestGORMRepositoryDryRunTenantOperations(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, time.June, 24, 9, 30, 0, 0, time.UTC)
	settings := DefaultSettings()
	settingsJSON, err := json.Marshal(settings)
	require.NoError(t, err)

	tenantModel := models.Tenant{
		ID:                  "tenant-1",
		Name:                "Demo OU",
		Slug:                "demo-ou",
		SchemaName:          "tenant_demo_ou",
		Settings:            settingsJSON,
		IsActive:            true,
		OnboardingCompleted: true,
		CreatedAt:           now.Add(-2 * time.Hour),
		UpdatedAt:           now.Add(-time.Hour),
	}
	userModel := models.User{
		ID:           "user-1",
		Email:        "user@example.com",
		PasswordHash: "hashed-password",
		Name:         "Test User",
		IsActive:     true,
		CreatedAt:    now.Add(-3 * time.Hour),
		UpdatedAt:    now.Add(-2 * time.Hour),
	}
	tenantUserModel := models.TenantUserModel{
		TenantID:  tenantModel.ID,
		UserID:    userModel.ID,
		Role:      RoleOwner,
		IsDefault: true,
		IsActive:  true,
		CreatedAt: now.Add(-time.Hour),
	}
	invitationModel := models.UserInvitation{
		ID:        "invitation-1",
		TenantID:  tenantModel.ID,
		Email:     "invitee@example.com",
		Role:      RoleAccountant,
		InvitedBy: userModel.ID,
		Token:     "token-1",
		ExpiresAt: now.Add(24 * time.Hour),
		CreatedAt: now.Add(-time.Hour),
		Tenant:    &tenantModel,
	}
	actorUserID := userModel.ID
	targetEmail := "invitee@example.com"
	auditEventModel := models.TenantAuditEvent{
		ID:          "audit-1",
		TenantID:    tenantModel.ID,
		ActorUserID: &actorUserID,
		Action:      AuditActionInvitationCreated,
		TargetType:  AuditTargetInvitation,
		TargetID:    invitationModel.ID,
		TargetEmail: &targetEmail,
		Metadata:    json.RawMessage(`{"role":"accountant"}`),
		CreatedAt:   now,
	}
	lockBefore := time.Date(2026, time.May, 31, 0, 0, 0, 0, time.UTC)
	lockAfter := time.Date(2026, time.June, 1, 0, 0, 0, 0, time.UTC)
	periodCloseModel := periodCloseEventModel{
		ID:              "period-close-1",
		TenantID:        tenantModel.ID,
		Action:          PeriodCloseActionClose,
		CloseKind:       PeriodCloseKindMonthEnd,
		PeriodEndDate:   lockBefore,
		LockDateBefore:  &lockBefore,
		LockDateAfter:   &lockAfter,
		Note:            "Month-end close",
		ReviewerSignOff: true,
		PerformedBy:     userModel.ID,
		CreatedAt:       now,
	}
	repo := NewGORMRepository(newTenantDryRunDB(t,
		withTenantDryRunFixtures(tenantDryRunFixture{
			tenants:      []models.Tenant{tenantModel},
			users:        []models.User{userModel},
			tenantUsers:  []models.TenantUserModel{tenantUserModel},
			invitations:  []models.UserInvitation{invitationModel},
			auditEvents:  []models.TenantAuditEvent{auditEventModel},
			periodCloses: []periodCloseEventModel{periodCloseModel},
			count:        1,
			countSet:     true,
		}),
		withTenantDryRunScanRows(tenantDryRunRowSet{
			columns: []string{"id", "name", "slug", "schema_name", "settings", "is_active", "onboarding_completed", "created_at", "updated_at", "role", "is_default"},
			values: [][]driver.Value{{
				tenantModel.ID,
				tenantModel.Name,
				tenantModel.Slug,
				tenantModel.SchemaName,
				settingsJSON,
				true,
				true,
				tenantModel.CreatedAt,
				tenantModel.UpdatedAt,
				RoleOwner,
				true,
			}},
		}),
		withTenantDryRunUpdateRows(1),
		withTenantDryRunDeleteRows(1),
	))
	tenantValue := &Tenant{
		ID:         tenantModel.ID,
		Name:       tenantModel.Name,
		Slug:       tenantModel.Slug,
		SchemaName: tenantModel.SchemaName,
		Settings:   settings,
		IsActive:   true,
		CreatedAt:  tenantModel.CreatedAt,
		UpdatedAt:  tenantModel.UpdatedAt,
	}

	require.NoError(t, repo.CreateTenant(ctx, tenantValue, settingsJSON, userModel.ID))

	gotTenant, err := repo.GetTenant(ctx, tenantModel.ID)
	require.NoError(t, err)
	assert.Equal(t, tenantModel.ID, gotTenant.ID)

	gotTenant, err = repo.GetTenantBySlug(ctx, tenantModel.Slug)
	require.NoError(t, err)
	assert.Equal(t, tenantModel.Slug, gotTenant.Slug)

	require.NoError(t, repo.UpdateTenant(ctx, tenantModel.ID, "Updated OU", settingsJSON, now))
	closeBeforeValue := lockBefore.Format(periodCloseDateLayout)
	closeAfterValue := lockAfter.Format(periodCloseDateLayout)
	require.NoError(t, repo.UpdateTenantWithPeriodCloseEvent(ctx, tenantModel.ID, "Updated OU", settingsJSON, now, &PeriodCloseEvent{
		ID:              periodCloseModel.ID,
		TenantID:        tenantModel.ID,
		Action:          PeriodCloseActionClose,
		CloseKind:       PeriodCloseKindMonthEnd,
		PeriodEndDate:   lockBefore.Format(periodCloseDateLayout),
		LockDateBefore:  &closeBeforeValue,
		LockDateAfter:   &closeAfterValue,
		Note:            periodCloseModel.Note,
		ReviewerSignOff: true,
		PerformedBy:     userModel.ID,
		CreatedAt:       now,
	}))

	periodCloseEvents, err := repo.ListPeriodCloseEvents(ctx, tenantModel.ID, 0)
	require.NoError(t, err)
	require.Len(t, periodCloseEvents, 1)
	assert.Equal(t, PeriodCloseActionClose, periodCloseEvents[0].Action)
	assert.Equal(t, closeBeforeValue, *periodCloseEvents[0].LockDateBefore)

	latestCloseEvent, err := repo.GetLatestCloseEventForPeriod(ctx, tenantModel.ID, lockBefore.Format(periodCloseDateLayout))
	require.NoError(t, err)
	require.NotNil(t, latestCloseEvent)
	assert.Equal(t, periodCloseModel.ID, latestCloseEvent.ID)

	require.NoError(t, repo.CompleteOnboarding(ctx, tenantModel.ID))

	auditEvent := &TenantAuditEvent{
		ID:          auditEventModel.ID,
		TenantID:    tenantModel.ID,
		ActorUserID: userModel.ID,
		Action:      AuditActionInvitationCreated,
		TargetType:  AuditTargetInvitation,
		TargetID:    invitationModel.ID,
		TargetEmail: invitationModel.Email,
		Metadata:    map[string]string{"role": RoleAccountant},
		CreatedAt:   now,
	}
	require.NoError(t, repo.CreateTenantAuditEvent(ctx, auditEvent))

	auditEvents, err := repo.ListTenantAuditEvents(ctx, tenantModel.ID, 0)
	require.NoError(t, err)
	require.Len(t, auditEvents, 1)
	assert.Equal(t, "accountant", auditEvents[0].Metadata["role"])

	auditEvents, err = repo.ListTenantAuditEvents(ctx, tenantModel.ID, 250)
	require.NoError(t, err)
	require.Len(t, auditEvents, 1)

	require.NoError(t, repo.AddUserToTenant(ctx, tenantModel.ID, "user-2", RoleAccountant))
	require.NoError(t, repo.RemoveUserFromTenant(ctx, tenantModel.ID, "user-2"))

	tenantUser, err := repo.GetTenantUser(ctx, tenantModel.ID, userModel.ID)
	require.NoError(t, err)
	assert.Equal(t, RoleOwner, tenantUser.Role)

	memberships, err := repo.ListUserTenants(ctx, userModel.ID)
	require.NoError(t, err)
	require.Len(t, memberships, 1)
	assert.Equal(t, tenantModel.ID, memberships[0].Tenant.ID)
	assert.Equal(t, RoleOwner, memberships[0].Role)
	assert.True(t, memberships[0].IsDefault)

	tenantUsers, err := repo.ListTenantUsers(ctx, tenantModel.ID)
	require.NoError(t, err)
	require.Len(t, tenantUsers, 1)
	assert.Equal(t, userModel.ID, tenantUsers[0].UserID)

	require.NoError(t, repo.UpdateTenantUserRole(ctx, tenantModel.ID, userModel.ID, RoleAdmin))
	require.NoError(t, repo.SetTenantUserActive(ctx, tenantModel.ID, userModel.ID, false))
	require.NoError(t, repo.RemoveTenantUser(ctx, tenantModel.ID, userModel.ID))

	user := &User{
		ID:           userModel.ID,
		Email:        userModel.Email,
		PasswordHash: userModel.PasswordHash,
		Name:         userModel.Name,
		IsActive:     true,
		CreatedAt:    userModel.CreatedAt,
		UpdatedAt:    userModel.UpdatedAt,
	}
	require.NoError(t, repo.CreateUser(ctx, user))

	gotUser, err := repo.GetUserByEmail(ctx, " USER@Example.COM ")
	require.NoError(t, err)
	assert.Equal(t, userModel.ID, gotUser.ID)

	gotUser, err = repo.GetUserByID(ctx, userModel.ID)
	require.NoError(t, err)
	assert.Equal(t, userModel.Email, gotUser.Email)

	require.NoError(t, repo.UpdateUserPassword(ctx, userModel.ID, "new-hash", now))

	invitation := &UserInvitation{
		ID:        invitationModel.ID,
		TenantID:  tenantModel.ID,
		Email:     invitationModel.Email,
		Role:      invitationModel.Role,
		InvitedBy: invitationModel.InvitedBy,
		Token:     invitationModel.Token,
		ExpiresAt: invitationModel.ExpiresAt,
		CreatedAt: invitationModel.CreatedAt,
	}
	require.NoError(t, repo.CreateInvitation(ctx, invitation))

	gotInvitation, err := repo.GetInvitationByToken(ctx, invitationModel.Token)
	require.NoError(t, err)
	assert.Equal(t, tenantModel.Name, gotInvitation.TenantName)

	require.NoError(t, repo.AcceptInvitation(ctx, invitation, "new-user", "hashed-password", "Invitee", true))
	require.NoError(t, repo.AcceptInvitation(ctx, invitation, userModel.ID, "", "", false))

	invitations, err := repo.ListInvitations(ctx, tenantModel.ID)
	require.NoError(t, err)
	require.Len(t, invitations, 1)
	assert.Equal(t, invitationModel.ID, invitations[0].ID)

	require.NoError(t, repo.RevokeInvitation(ctx, tenantModel.ID, invitationModel.ID))

	isMember, err := repo.CheckUserIsMember(ctx, tenantModel.ID, strings.ToUpper(userModel.Email))
	require.NoError(t, err)
	assert.True(t, isMember)

	require.NoError(t, repo.DeleteTenant(ctx, tenantModel.ID, tenantModel.SchemaName))
}

func TestGORMRepositoryDryRunTenantErrors(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, time.June, 24, 12, 0, 0, 0, time.UTC)
	dbErr := errors.New("dry-run database error")
	settings := DefaultSettings()
	settingsJSON, err := json.Marshal(settings)
	require.NoError(t, err)
	tenantValue := &Tenant{
		ID:         "tenant-1",
		Name:       "Demo OU",
		Slug:       "demo-ou",
		SchemaName: "tenant_demo_ou",
		Settings:   settings,
		IsActive:   true,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	invitation := &UserInvitation{
		ID:        "invitation-1",
		TenantID:  tenantValue.ID,
		Email:     "invitee@example.com",
		Role:      RoleAccountant,
		InvitedBy: "owner-1",
		Token:     "token-1",
		ExpiresAt: now.Add(24 * time.Hour),
		CreatedAt: now,
	}
	periodCloseEvent := &PeriodCloseEvent{
		ID:              "period-close-1",
		TenantID:        tenantValue.ID,
		Action:          PeriodCloseActionClose,
		CloseKind:       PeriodCloseKindMonthEnd,
		PeriodEndDate:   "2026-05-31",
		Note:            "Month-end close",
		ReviewerSignOff: true,
		PerformedBy:     "owner-1",
		CreatedAt:       now,
	}

	t.Run("CreateTenant wraps insert errors", func(t *testing.T) {
		repo := NewGORMRepository(newTenantDryRunDB(t, withTenantDryRunCreateError(dbErr)))

		err := repo.CreateTenant(ctx, tenantValue, settingsJSON, "owner-1")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "insert tenant")
	})

	t.Run("CreateTenant wraps schema creation errors", func(t *testing.T) {
		repo := NewGORMRepository(newTenantDryRunDB(t, withTenantDryRunRawError(dbErr)))

		err := repo.CreateTenant(ctx, tenantValue, settingsJSON, "owner-1")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "create tenant schema")
	})

	t.Run("CreateTenant wraps default chart creation errors", func(t *testing.T) {
		repo := NewGORMRepository(newTenantDryRunDB(t, withTenantDryRunRawErrors(nil, dbErr)))

		err := repo.CreateTenant(ctx, tenantValue, settingsJSON, "owner-1")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "create default chart of accounts")
		assert.ErrorIs(t, err, dbErr)
	})

	t.Run("CreateTenant wraps owner membership errors", func(t *testing.T) {
		repo := NewGORMRepository(newTenantDryRunDB(t, withTenantDryRunCreateErrors(nil, dbErr)))

		err := repo.CreateTenant(ctx, tenantValue, settingsJSON, "owner-1")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "add owner to tenant")
	})

	t.Run("DeleteTenant wraps schema drop errors", func(t *testing.T) {
		repo := NewGORMRepository(newTenantDryRunDB(t, withTenantDryRunRawError(dbErr)))

		err := repo.DeleteTenant(ctx, tenantValue.ID, tenantValue.SchemaName)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "drop tenant schema")
	})

	t.Run("DeleteTenant wraps delete errors", func(t *testing.T) {
		repo := NewGORMRepository(newTenantDryRunDB(t, withTenantDryRunDeleteError(dbErr)))

		err := repo.DeleteTenant(ctx, tenantValue.ID, tenantValue.SchemaName)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "delete tenant users")
	})

	t.Run("DeleteTenant wraps tenant record delete errors", func(t *testing.T) {
		repo := NewGORMRepository(newTenantDryRunDB(t, withTenantDryRunDeleteErrors(nil, dbErr)))

		err := repo.DeleteTenant(ctx, tenantValue.ID, tenantValue.SchemaName)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "delete tenant")
		assert.ErrorIs(t, err, dbErr)
	})

	t.Run("UpdateTenantWithPeriodCloseEvent rejects invalid event date", func(t *testing.T) {
		repo := NewGORMRepository(newTenantDryRunDB(t))
		invalidEvent := *periodCloseEvent
		invalidEvent.PeriodEndDate = "2026-99-99"

		err := repo.UpdateTenantWithPeriodCloseEvent(ctx, tenantValue.ID, tenantValue.Name, settingsJSON, now, &invalidEvent)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "parse period end date")
	})

	t.Run("UpdateTenantWithPeriodCloseEvent wraps update errors", func(t *testing.T) {
		repo := NewGORMRepository(newTenantDryRunDB(t, withTenantDryRunUpdateError(dbErr)))

		err := repo.UpdateTenantWithPeriodCloseEvent(ctx, tenantValue.ID, tenantValue.Name, settingsJSON, now, periodCloseEvent)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "update tenant")
		assert.ErrorIs(t, err, dbErr)
	})

	t.Run("UpdateTenantWithPeriodCloseEvent wraps insert errors", func(t *testing.T) {
		repo := NewGORMRepository(newTenantDryRunDB(t, withTenantDryRunCreateError(dbErr)))

		err := repo.UpdateTenantWithPeriodCloseEvent(ctx, tenantValue.ID, tenantValue.Name, settingsJSON, now, periodCloseEvent)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "insert period close event")
		assert.ErrorIs(t, err, dbErr)
	})

	t.Run("ListPeriodCloseEvents wraps query errors", func(t *testing.T) {
		repo := NewGORMRepository(newTenantDryRunDB(t, withTenantDryRunQueryError(dbErr)))

		events, err := repo.ListPeriodCloseEvents(ctx, tenantValue.ID, 0)

		require.Error(t, err)
		assert.Nil(t, events)
		assert.Contains(t, err.Error(), "list period close events")
		assert.ErrorIs(t, err, dbErr)
	})

	t.Run("GetLatestCloseEventForPeriod rejects invalid date", func(t *testing.T) {
		repo := NewGORMRepository(newTenantDryRunDB(t))

		event, err := repo.GetLatestCloseEventForPeriod(ctx, tenantValue.ID, "2026-99-99")

		assert.Nil(t, event)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "parse period end date")
	})

	t.Run("GetLatestCloseEventForPeriod maps not found", func(t *testing.T) {
		repo := NewGORMRepository(newTenantDryRunDB(t, withTenantDryRunQueryError(gorm.ErrRecordNotFound)))

		event, err := repo.GetLatestCloseEventForPeriod(ctx, tenantValue.ID, periodCloseEvent.PeriodEndDate)

		require.NoError(t, err)
		assert.Nil(t, event)
	})

	t.Run("GetLatestCloseEventForPeriod wraps query errors", func(t *testing.T) {
		repo := NewGORMRepository(newTenantDryRunDB(t, withTenantDryRunQueryError(dbErr)))

		event, err := repo.GetLatestCloseEventForPeriod(ctx, tenantValue.ID, periodCloseEvent.PeriodEndDate)

		assert.Nil(t, event)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "get latest period close event")
		assert.ErrorIs(t, err, dbErr)
	})

	t.Run("ListTenantAuditEvents wraps query errors", func(t *testing.T) {
		repo := NewGORMRepository(newTenantDryRunDB(t, withTenantDryRunQueryError(dbErr)))

		events, err := repo.ListTenantAuditEvents(ctx, tenantValue.ID, 10)

		require.Error(t, err)
		assert.Nil(t, events)
		assert.Contains(t, err.Error(), "list tenant audit events")
	})

	t.Run("ListTenantAuditEvents rejects invalid metadata", func(t *testing.T) {
		repo := NewGORMRepository(newTenantDryRunDB(t, withTenantDryRunFixtures(tenantDryRunFixture{
			auditEvents: []models.TenantAuditEvent{{
				ID:         "audit-1",
				TenantID:   tenantValue.ID,
				Action:     AuditActionInvitationCreated,
				TargetType: AuditTargetInvitation,
				TargetID:   invitation.ID,
				Metadata:   json.RawMessage(`{"role"`),
				CreatedAt:  now,
			}},
		})))

		events, err := repo.ListTenantAuditEvents(ctx, tenantValue.ID, 10)

		require.Error(t, err)
		assert.Nil(t, events)
		assert.Contains(t, err.Error(), "parse audit metadata")
	})

	t.Run("AddUserToTenant wraps upsert errors", func(t *testing.T) {
		repo := NewGORMRepository(newTenantDryRunDB(t, withTenantDryRunCreateError(dbErr)))

		err := repo.AddUserToTenant(ctx, tenantValue.ID, "user-1", RoleAccountant)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "add user to tenant")
	})

	t.Run("AcceptInvitation wraps new user errors", func(t *testing.T) {
		repo := NewGORMRepository(newTenantDryRunDB(t, withTenantDryRunCreateError(dbErr)))

		err := repo.AcceptInvitation(ctx, invitation, "user-1", "hashed-password", "Invitee", true)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "create user")
	})

	t.Run("AcceptInvitation wraps membership errors", func(t *testing.T) {
		repo := NewGORMRepository(newTenantDryRunDB(t, withTenantDryRunCreateError(dbErr)))

		err := repo.AcceptInvitation(ctx, invitation, "user-1", "", "", false)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "add user to tenant")
	})

	t.Run("AcceptInvitation wraps accepted update errors", func(t *testing.T) {
		repo := NewGORMRepository(newTenantDryRunDB(t, withTenantDryRunUpdateError(dbErr)))

		err := repo.AcceptInvitation(ctx, invitation, "user-1", "", "", false)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "mark invitation accepted")
	})

	t.Run("GetTenant maps not found and wraps query errors", func(t *testing.T) {
		repo := NewGORMRepository(newTenantDryRunDB(t, withTenantDryRunQueryError(gorm.ErrRecordNotFound)))

		gotTenant, err := repo.GetTenant(ctx, tenantValue.ID)
		assert.Nil(t, gotTenant)
		assert.ErrorIs(t, err, ErrTenantNotFound)

		repo = NewGORMRepository(newTenantDryRunDB(t, withTenantDryRunQueryError(dbErr)))
		gotTenant, err = repo.GetTenant(ctx, tenantValue.ID)
		assert.Nil(t, gotTenant)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "get tenant")
	})

	t.Run("GetInvitationByToken maps not found and wraps query errors", func(t *testing.T) {
		repo := NewGORMRepository(newTenantDryRunDB(t, withTenantDryRunQueryError(gorm.ErrRecordNotFound)))

		gotInvitation, err := repo.GetInvitationByToken(ctx, invitation.Token)
		assert.Nil(t, gotInvitation)
		assert.ErrorIs(t, err, ErrInvitationNotFound)

		repo = NewGORMRepository(newTenantDryRunDB(t, withTenantDryRunQueryError(dbErr)))
		gotInvitation, err = repo.GetInvitationByToken(ctx, invitation.Token)
		assert.Nil(t, gotInvitation)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "get invitation")
	})

	t.Run("GetTenantUser maps not found", func(t *testing.T) {
		repo := NewGORMRepository(newTenantDryRunDB(t, withTenantDryRunQueryError(gorm.ErrRecordNotFound)))

		user, err := repo.GetTenantUser(ctx, tenantValue.ID, "user-1")

		assert.Nil(t, user)
		assert.ErrorIs(t, err, ErrUserNotInTenant)
	})

	t.Run("UpdateUserPassword returns not found without rows", func(t *testing.T) {
		repo := NewGORMRepository(newTenantDryRunDB(t, withTenantDryRunUpdateRows(0)))

		err := repo.UpdateUserPassword(ctx, "missing-user", "hash", now)

		assert.ErrorIs(t, err, ErrUserNotFound)
	})

	t.Run("SetTenantUserActive returns not-in-tenant without rows", func(t *testing.T) {
		repo := NewGORMRepository(newTenantDryRunDB(t, withTenantDryRunUpdateRows(0)))

		err := repo.SetTenantUserActive(ctx, tenantValue.ID, "missing-user", false)

		assert.ErrorIs(t, err, ErrUserNotInTenant)
	})

	t.Run("SetTenantUserActive wraps update errors", func(t *testing.T) {
		repo := NewGORMRepository(newTenantDryRunDB(t, withTenantDryRunUpdateError(dbErr)))

		err := repo.SetTenantUserActive(ctx, tenantValue.ID, "user-1", false)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "update tenant user status")
		assert.ErrorIs(t, err, dbErr)
	})

	t.Run("RevokeInvitation returns not found without rows", func(t *testing.T) {
		repo := NewGORMRepository(newTenantDryRunDB(t, withTenantDryRunDeleteRows(0)))

		err := repo.RevokeInvitation(ctx, tenantValue.ID, invitation.ID)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "invitation not found or already accepted")
	})
}
