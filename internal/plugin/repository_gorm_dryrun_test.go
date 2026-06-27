package plugin

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
	"sync/atomic"
	"testing"
	"time"

	"github.com/HMB-research/open-accounting/internal/models"
	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type pluginDryRunConnPool struct{}

func (pluginDryRunConnPool) PrepareContext(context.Context, string) (*sql.Stmt, error) {
	return nil, errors.New("dry-run plugin tests should not prepare statements")
}

func (pluginDryRunConnPool) ExecContext(context.Context, string, ...interface{}) (sql.Result, error) {
	return nil, errors.New("dry-run plugin tests should not execute statements")
}

func (pluginDryRunConnPool) QueryContext(context.Context, string, ...interface{}) (*sql.Rows, error) {
	return nil, errors.New("dry-run plugin tests should not query rows")
}

func (pluginDryRunConnPool) QueryRowContext(context.Context, string, ...interface{}) *sql.Row {
	return nil
}

func (pluginDryRunConnPool) BeginTx(context.Context, *sql.TxOptions) (gorm.ConnPool, error) {
	return &pluginDryRunTx{}, nil
}

type pluginDryRunTx struct {
	pluginDryRunConnPool
}

func (*pluginDryRunTx) Commit() error {
	return nil
}

func (*pluginDryRunTx) Rollback() error {
	return nil
}

type pluginDryRunDBOption func(t *testing.T, db *gorm.DB)

type pluginDryRunFixtures struct {
	registries        []models.PluginRegistry
	registryIndex     int
	plugins           []models.Plugin
	pluginIndex       int
	tenantPlugins     []models.TenantPlugin
	tenantPluginIndex int
	settingsRows      []json.RawMessage
	settingsIndex     int
	counts            []int64
	countIndex        int
}

type pluginDryRunSQLCapture struct {
	statements []string
}

var pluginDryRunCallbackID uint64

func newPluginDryRunDB(t *testing.T, opts ...pluginDryRunDBOption) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(postgres.New(postgres.Config{Conn: pluginDryRunConnPool{}}), &gorm.Config{
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

func withPluginDryRunFixtures(fixtures pluginDryRunFixtures) pluginDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		err := db.Callback().Query().After("gorm:query").Register(pluginDryRunCallbackName("query_fixtures"), func(tx *gorm.DB) {
			populatePluginDryRunDest(tx, tx.Statement.Dest, &fixtures)
		})
		require.NoError(t, err)
	}
}

func withPluginDryRunSQLCapture(capture *pluginDryRunSQLCapture) pluginDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		err := db.Callback().Create().After("gorm:create").Register(pluginDryRunCallbackName("capture_create"), capture.capture)
		require.NoError(t, err)
		err = db.Callback().Query().After("gorm:query").Register(pluginDryRunCallbackName("capture_query"), capture.capture)
		require.NoError(t, err)
		err = db.Callback().Row().After("gorm:row").Register(pluginDryRunCallbackName("capture_row"), capture.capture)
		require.NoError(t, err)
		err = db.Callback().Update().After("gorm:update").Register(pluginDryRunCallbackName("capture_update"), capture.capture)
		require.NoError(t, err)
		err = db.Callback().Delete().After("gorm:delete").Register(pluginDryRunCallbackName("capture_delete"), capture.capture)
		require.NoError(t, err)
	}
}

func withPluginDryRunScanRows(rows *sql.Rows) pluginDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		err := db.Callback().Row().After("gorm:row").Register(pluginDryRunCallbackName("scan_rows"), func(tx *gorm.DB) {
			if rows == nil || !strings.Contains(tx.Statement.SQL.String(), "plugins AS p") {
				return
			}
			tx.Statement.Dest = rows
			tx.Error = nil
		})
		require.NoError(t, err)
	}
}

func withPluginDryRunQueryErrors(expectedErrs ...error) pluginDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		var index int
		err := db.Callback().Query().Before("gorm:query").Register(pluginDryRunCallbackName("query_error"), func(tx *gorm.DB) {
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

func withPluginDryRunCreateError(expectedErr error) pluginDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		err := db.Callback().Create().Before("gorm:create").Register(pluginDryRunCallbackName("create_error"), func(tx *gorm.DB) {
			tx.AddError(expectedErr)
		})
		require.NoError(t, err)
	}
}

func withPluginDryRunUpdateRows(rows ...int64) pluginDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		var index int
		err := db.Callback().Update().After("gorm:update").Register(pluginDryRunCallbackName("update_rows"), func(tx *gorm.DB) {
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

func withPluginDryRunUpdateError(expectedErr error) pluginDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		err := db.Callback().Update().Before("gorm:update").Register(pluginDryRunCallbackName("update_error"), func(tx *gorm.DB) {
			tx.AddError(expectedErr)
		})
		require.NoError(t, err)
	}
}

func withPluginDryRunDeleteRows(rows ...int64) pluginDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		var index int
		err := db.Callback().Delete().After("gorm:delete").Register(pluginDryRunCallbackName("delete_rows"), func(tx *gorm.DB) {
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

func withPluginDryRunDeleteError(expectedErr error) pluginDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		err := db.Callback().Delete().Before("gorm:delete").Register(pluginDryRunCallbackName("delete_error"), func(tx *gorm.DB) {
			tx.AddError(expectedErr)
		})
		require.NoError(t, err)
	}
}

func pluginDryRunCallbackName(suffix string) string {
	id := atomic.AddUint64(&pluginDryRunCallbackID, 1)
	return fmt.Sprintf("plugin_dryrun:%d:%s", id, suffix)
}

func populatePluginDryRunDest(tx *gorm.DB, dest any, fixtures *pluginDryRunFixtures) {
	switch typed := dest.(type) {
	case *models.PluginRegistry:
		if registry, ok := nextPluginDryRunValue(fixtures.registries, &fixtures.registryIndex); ok {
			*typed = registry
			tx.RowsAffected = 1
		}
	case *[]models.PluginRegistry:
		*typed = append([]models.PluginRegistry(nil), fixtures.registries...)
		tx.RowsAffected = int64(len(fixtures.registries))
	case *models.Plugin:
		if plugin, ok := nextPluginDryRunValue(fixtures.plugins, &fixtures.pluginIndex); ok {
			*typed = plugin
			tx.RowsAffected = 1
		}
	case *[]models.Plugin:
		*typed = append([]models.Plugin(nil), fixtures.plugins...)
		tx.RowsAffected = int64(len(fixtures.plugins))
	case *models.TenantPlugin:
		if tenantPlugin, ok := nextPluginDryRunValue(fixtures.tenantPlugins, &fixtures.tenantPluginIndex); ok {
			*typed = tenantPlugin
			tx.RowsAffected = 1
		}
	case *[]models.TenantPlugin:
		*typed = append([]models.TenantPlugin(nil), fixtures.tenantPlugins...)
		tx.RowsAffected = int64(len(fixtures.tenantPlugins))
	case *int64:
		count, ok := nextPluginDryRunValue(fixtures.counts, &fixtures.countIndex)
		if !ok {
			count = 0
		}
		*typed = count
		tx.RowsAffected = 1
	default:
		populatePluginDryRunSettingsDest(tx, dest, fixtures)
	}
}

func populatePluginDryRunSettingsDest(tx *gorm.DB, dest any, fixtures *pluginDryRunFixtures) {
	row, ok := nextPluginDryRunValue(fixtures.settingsRows, &fixtures.settingsIndex)
	if !ok {
		return
	}

	// The repository uses an anonymous settings projection; match its shape by reflection.
	setAnonymousSettingsRow(tx, dest, row)
}

func setAnonymousSettingsRow(tx *gorm.DB, dest any, settings json.RawMessage) {
	value := reflect.ValueOf(dest)
	if !value.IsValid() || value.Kind() != reflect.Ptr || value.IsNil() {
		return
	}
	value = value.Elem()
	if !value.IsValid() || value.Kind() != reflect.Struct {
		return
	}
	field := value.FieldByName("Settings")
	if !field.IsValid() || !field.CanSet() || field.Type() != reflect.TypeOf(json.RawMessage{}) {
		return
	}
	field.Set(reflect.ValueOf(settings))
	tx.RowsAffected = 1
}

func nextPluginDryRunValue[T any](values []T, index *int) (T, bool) {
	var zero T
	if len(values) == 0 {
		return zero, false
	}
	if *index >= len(values) {
		return values[len(values)-1], true
	}
	value := values[*index]
	*index++
	return value, true
}

func (c *pluginDryRunSQLCapture) capture(tx *gorm.DB) {
	if c == nil {
		return
	}
	if sql := strings.TrimSpace(tx.Statement.SQL.String()); sql != "" {
		c.statements = append(c.statements, sql)
	}
}

func (c *pluginDryRunSQLCapture) assertContains(t *testing.T, expected string) {
	t.Helper()
	for _, statement := range c.statements {
		if strings.Contains(statement, expected) {
			return
		}
	}
	t.Fatalf("expected dry-run SQL to contain %q in %#v", expected, c.statements)
}

type pluginDryRunRowsConnector struct {
	columns []string
	values  [][]driver.Value
}

func (c *pluginDryRunRowsConnector) Connect(context.Context) (driver.Conn, error) {
	return &pluginDryRunRowsConn{
		columns: append([]string(nil), c.columns...),
		values:  clonePluginDryRunDriverValues(c.values),
	}, nil
}

func (*pluginDryRunRowsConnector) Driver() driver.Driver {
	return pluginDryRunRowsDriver{}
}

type pluginDryRunRowsDriver struct{}

func (pluginDryRunRowsDriver) Open(string) (driver.Conn, error) {
	return nil, errors.New("plugin dry-run rows driver requires a connector")
}

type pluginDryRunRowsConn struct {
	columns []string
	values  [][]driver.Value
}

func (*pluginDryRunRowsConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("plugin dry-run rows should not prepare statements")
}

func (*pluginDryRunRowsConn) Close() error {
	return nil
}

func (*pluginDryRunRowsConn) Begin() (driver.Tx, error) {
	return nil, errors.New("plugin dry-run rows should not begin transactions")
}

func (c *pluginDryRunRowsConn) QueryContext(context.Context, string, []driver.NamedValue) (driver.Rows, error) {
	return &pluginDryRunRows{
		columns: append([]string(nil), c.columns...),
		values:  clonePluginDryRunDriverValues(c.values),
	}, nil
}

type pluginDryRunRows struct {
	columns []string
	values  [][]driver.Value
	index   int
}

func (r *pluginDryRunRows) Columns() []string {
	return append([]string(nil), r.columns...)
}

func (*pluginDryRunRows) Close() error {
	return nil
}

func (r *pluginDryRunRows) Next(dest []driver.Value) error {
	if r.index >= len(r.values) {
		return io.EOF
	}
	copy(dest, r.values[r.index])
	r.index++
	return nil
}

func pluginDryRunSQLRows(t *testing.T, columns []string, values [][]driver.Value) *sql.Rows {
	t.Helper()

	db := sql.OpenDB(&pluginDryRunRowsConnector{
		columns: columns,
		values:  values,
	})
	t.Cleanup(func() {
		_ = db.Close()
	})

	rows, err := db.QueryContext(context.Background(), "SELECT plugin_dryrun_rows")
	require.NoError(t, err)
	return rows
}

func clonePluginDryRunDriverValues(values [][]driver.Value) [][]driver.Value {
	clone := make([][]driver.Value, len(values))
	for i := range values {
		clone[i] = append([]driver.Value(nil), values[i]...)
	}
	return clone
}

func TestGORMRepositoryDryRunOperations(t *testing.T) {
	ctx := context.Background()
	tenantID := uuid.New()
	pluginID := uuid.New()
	registryID := uuid.New()
	now := time.Date(2026, time.June, 25, 9, 0, 0, 0, time.UTC)
	settings := json.RawMessage(`{"mode":"dry-run"}`)
	registry := pluginDryRunRegistryModel(registryID, now)
	pluginModel := pluginDryRunPluginModel(pluginID, now)
	tenantPluginModel := pluginDryRunTenantPluginModel(tenantID, pluginID, now, settings, &pluginModel)
	capture := &pluginDryRunSQLCapture{}
	repo := NewGORMRepository(newPluginDryRunDB(t,
		withPluginDryRunFixtures(pluginDryRunFixtures{
			registries:    []models.PluginRegistry{registry},
			plugins:       []models.Plugin{pluginModel},
			tenantPlugins: []models.TenantPlugin{tenantPluginModel},
			settingsRows:  []json.RawMessage{settings},
			counts:        []int64{1, 3},
		}),
		withPluginDryRunUpdateRows(1),
		withPluginDryRunDeleteRows(1),
		withPluginDryRunSQLCapture(capture),
	))

	registries, err := repo.ListRegistries(ctx)
	require.NoError(t, err)
	require.Len(t, registries, 1)
	assert.Equal(t, registryID, registries[0].ID)

	gotRegistry, err := repo.GetRegistry(ctx, registryID)
	require.NoError(t, err)
	require.NotNil(t, gotRegistry)
	assert.Equal(t, "official", gotRegistry.Name)

	createdRegistry, err := repo.CreateRegistry(ctx, "community", "https://plugins.example.com", "Community plugins")
	require.NoError(t, err)
	require.NotNil(t, createdRegistry)
	assert.Equal(t, "community", createdRegistry.Name)

	deletedRegistries, err := repo.DeleteRegistry(ctx, registryID)
	require.NoError(t, err)
	assert.Equal(t, int64(1), deletedRegistries)
	require.NoError(t, repo.UpdateRegistryLastSynced(ctx, registryID))

	plugins, err := repo.ListPlugins(ctx)
	require.NoError(t, err)
	require.Len(t, plugins, 1)
	assert.Equal(t, "payroll-sync", plugins[0].Name)

	gotPlugin, err := repo.GetPlugin(ctx, pluginID)
	require.NoError(t, err)
	require.NotNil(t, gotPlugin)
	assert.Equal(t, pluginID, gotPlugin.ID)

	gotPluginByName, err := repo.GetPluginByName(ctx, "payroll-sync")
	require.NoError(t, err)
	require.NotNil(t, gotPluginByName)
	assert.Equal(t, "Payroll Sync", gotPluginByName.DisplayName)

	domainPlugin := modelToPlugin(&pluginModel)
	require.NoError(t, repo.CreatePlugin(ctx, &domainPlugin))
	require.NoError(t, repo.UpdatePlugin(ctx, &domainPlugin))
	deletedPlugins, err := repo.DeletePlugin(ctx, pluginID)
	require.NoError(t, err)
	assert.Equal(t, int64(1), deletedPlugins)

	tenantPlugins, err := repo.ListTenantPlugins(ctx, tenantID)
	require.NoError(t, err)
	require.Len(t, tenantPlugins, 1)
	assert.Equal(t, tenantID, tenantPlugins[0].TenantID)
	require.NotNil(t, tenantPlugins[0].Plugin)
	assert.Equal(t, "payroll-sync", tenantPlugins[0].Plugin.Name)

	gotTenantPlugin, err := repo.GetTenantPlugin(ctx, tenantID, pluginID)
	require.NoError(t, err)
	require.NotNil(t, gotTenantPlugin)
	assert.Equal(t, pluginID, gotTenantPlugin.PluginID)

	require.NoError(t, repo.CreateTenantPlugin(ctx, tenantID, pluginID, settings))
	require.NoError(t, repo.EnableTenantPlugin(ctx, tenantID, pluginID, settings))
	disabledRows, err := repo.DisableTenantPlugin(ctx, tenantID, pluginID)
	require.NoError(t, err)
	assert.Equal(t, int64(1), disabledRows)

	gotSettings, err := repo.GetTenantPluginSettings(ctx, tenantID, pluginID)
	require.NoError(t, err)
	assert.JSONEq(t, string(settings), string(gotSettings))

	require.NoError(t, repo.UpdateTenantPluginSettings(ctx, tenantID, pluginID, json.RawMessage(`{"mode":"updated"}`)))
	require.NoError(t, repo.DeleteTenantPlugin(ctx, tenantID, pluginID))

	enabled, err := repo.IsPluginEnabledForTenant(ctx, tenantID, pluginID)
	require.NoError(t, err)
	assert.True(t, enabled)

	enabledPlugins, err := repo.ListEnabledPlugins(ctx)
	require.NoError(t, err)
	require.Len(t, enabledPlugins, 1)
	assert.Equal(t, StateEnabled, enabledPlugins[0].State)

	manifestJSON := []byte(`{"name":"payroll-sync","version":"1.2.3"}`)
	insertedPlugin, err := repo.InsertPluginReturning(ctx, &Manifest{
		Name:        "payroll-sync",
		DisplayName: "Payroll Sync",
		Description: "Imports payroll activity",
		Version:     "1.2.3",
		Author:      "Open Accounting",
		License:     "MIT",
		Homepage:    "https://example.com/payroll-sync",
	}, "https://github.com/example/payroll-sync", RepoGitHub, manifestJSON)
	require.NoError(t, err)
	require.NotNil(t, insertedPlugin)
	assert.Equal(t, "payroll-sync", insertedPlugin.Name)
	assert.Equal(t, StateInstalled, insertedPlugin.State)

	tenantCount, err := repo.CountEnabledTenantsForPlugin(ctx, pluginID)
	require.NoError(t, err)
	assert.Equal(t, 3, tenantCount)

	require.NoError(t, repo.UpdatePluginState(ctx, pluginID, StateEnabled, []string{"read:invoices"}))
	require.NoError(t, repo.DisableAllTenantsForPlugin(ctx, pluginID))

	capture.assertContains(t, `ORDER BY is_official DESC, name ASC`)
	capture.assertContains(t, `ORDER BY display_name ASC`)
	capture.assertContains(t, `JOIN plugins ON plugins.id = tenant_plugins.plugin_id`)
	capture.assertContains(t, `ON CONFLICT ("tenant_id","plugin_id") DO UPDATE`)
	capture.assertContains(t, `WHERE tenant_id = $1 AND plugin_id = $2`)
}

func TestGORMRepositoryDryRunGetTenantPluginsWithAll(t *testing.T) {
	ctx := context.Background()
	tenantID := uuid.New()
	pluginID := uuid.New()
	availablePluginID := uuid.New()
	tenantPluginID := uuid.New()
	now := time.Date(2026, time.June, 25, 10, 0, 0, 0, time.UTC)
	capture := &pluginDryRunSQLCapture{}
	rows := pluginDryRunSQLRows(t, pluginTenantPluginsWithAllColumns(), [][]driver.Value{
		pluginTenantPluginsWithAllRow(tenantPluginID, tenantID, pluginID, true, json.RawMessage(`{"provider":"csv"}`), now, pluginDryRunPluginModel(pluginID, now)),
		pluginTenantPluginsWithAllAvailableRow(availablePluginID, now),
	})
	repo := NewGORMRepository(newPluginDryRunDB(t,
		withPluginDryRunSQLCapture(capture),
		withPluginDryRunScanRows(rows),
	))

	tenantPlugins, err := repo.GetTenantPluginsWithAll(ctx, tenantID)
	require.NoError(t, err)
	require.Len(t, tenantPlugins, 2)

	assert.Equal(t, tenantPluginID, tenantPlugins[0].ID)
	assert.Equal(t, tenantID, tenantPlugins[0].TenantID)
	assert.Equal(t, pluginID, tenantPlugins[0].PluginID)
	assert.True(t, tenantPlugins[0].IsEnabled)
	assert.JSONEq(t, `{"provider":"csv"}`, string(tenantPlugins[0].Settings))
	require.NotNil(t, tenantPlugins[0].EnabledAt)
	assert.Equal(t, now, *tenantPlugins[0].EnabledAt)
	require.NotNil(t, tenantPlugins[0].Plugin)
	assert.Equal(t, "payroll-sync", tenantPlugins[0].Plugin.Name)

	assert.Equal(t, uuid.Nil, tenantPlugins[1].ID)
	assert.Equal(t, tenantID, tenantPlugins[1].TenantID)
	assert.Equal(t, availablePluginID, tenantPlugins[1].PluginID)
	assert.False(t, tenantPlugins[1].IsEnabled)
	assert.Nil(t, tenantPlugins[1].Settings)
	assert.Nil(t, tenantPlugins[1].EnabledAt)
	require.NotNil(t, tenantPlugins[1].Plugin)
	assert.Equal(t, "available-plugin", tenantPlugins[1].Plugin.Name)

	capture.assertContains(t, `plugins AS p`)
	capture.assertContains(t, `LEFT JOIN tenant_plugins AS tp ON tp.plugin_id = p.id AND tp.tenant_id = $1`)
	capture.assertContains(t, `WHERE p.state = $2`)
	capture.assertContains(t, `ORDER BY p.display_name ASC`)
}

func TestGORMRepositoryDryRunNotFoundDefaults(t *testing.T) {
	ctx := context.Background()
	tenantID := uuid.New()
	pluginID := uuid.New()
	repo := NewGORMRepository(newPluginDryRunDB(t, withPluginDryRunQueryErrors(
		gorm.ErrRecordNotFound,
		gorm.ErrRecordNotFound,
		gorm.ErrRecordNotFound,
		gorm.ErrRecordNotFound,
		gorm.ErrRecordNotFound,
	)))

	registry, err := repo.GetRegistry(ctx, uuid.New())
	assert.Nil(t, registry)
	require.Error(t, err)
	assert.EqualError(t, err, "registry not found")

	plugin, err := repo.GetPlugin(ctx, pluginID)
	assert.Nil(t, plugin)
	require.Error(t, err)
	assert.EqualError(t, err, "plugin not found")

	pluginByName, err := repo.GetPluginByName(ctx, "missing")
	assert.Nil(t, pluginByName)
	require.Error(t, err)
	assert.EqualError(t, err, "plugin not found")

	tenantPlugin, err := repo.GetTenantPlugin(ctx, tenantID, pluginID)
	assert.Nil(t, tenantPlugin)
	require.Error(t, err)
	assert.EqualError(t, err, "tenant plugin not found")

	settings, err := repo.GetTenantPluginSettings(ctx, tenantID, pluginID)
	require.NoError(t, err)
	assert.JSONEq(t, `{}`, string(settings))

	nilSettingsRepo := NewGORMRepository(newPluginDryRunDB(t, withPluginDryRunFixtures(pluginDryRunFixtures{
		settingsRows: []json.RawMessage{nil},
	})))
	settings, err = nilSettingsRepo.GetTenantPluginSettings(ctx, tenantID, pluginID)
	require.NoError(t, err)
	assert.JSONEq(t, `{}`, string(settings))
}

func TestGORMRepositoryDryRunErrors(t *testing.T) {
	ctx := context.Background()
	tenantID := uuid.New()
	pluginID := uuid.New()
	registryID := uuid.New()
	now := time.Date(2026, time.June, 25, 11, 0, 0, 0, time.UTC)
	domainPlugin := modelToPlugin(ptrPluginDryRunModel(pluginDryRunPluginModel(pluginID, now)))
	manifest := &Manifest{Name: "payroll-sync", DisplayName: "Payroll Sync", Version: "1.2.3"}
	dbErr := errors.New("database failed")

	t.Run("create errors", func(t *testing.T) {
		repo := NewGORMRepository(newPluginDryRunDB(t, withPluginDryRunCreateError(dbErr)))

		createdRegistry, err := repo.CreateRegistry(ctx, "community", "https://plugins.example.com", "Community")
		assert.Nil(t, createdRegistry)
		assertWrappedPluginDryRunError(t, err, "create registry", dbErr)
		assertWrappedPluginDryRunError(t, repo.CreatePlugin(ctx, &domainPlugin), "create plugin", dbErr)
		assertWrappedPluginDryRunError(t, repo.CreateTenantPlugin(ctx, tenantID, pluginID, json.RawMessage(`{}`)), "create tenant plugin", dbErr)
		assertWrappedPluginDryRunError(t, repo.EnableTenantPlugin(ctx, tenantID, pluginID, json.RawMessage(`{}`)), "enable tenant plugin", dbErr)

		insertedPlugin, err := repo.InsertPluginReturning(ctx, manifest, "https://github.com/example/payroll-sync", RepoGitHub, []byte(`{}`))
		assert.Nil(t, insertedPlugin)
		assertWrappedPluginDryRunError(t, err, "insert plugin", dbErr)
	})

	t.Run("query errors", func(t *testing.T) {
		repo := NewGORMRepository(newPluginDryRunDB(t, withPluginDryRunQueryErrors(dbErr)))

		registries, err := repo.ListRegistries(ctx)
		assert.Nil(t, registries)
		assertWrappedPluginDryRunError(t, err, "list registries", dbErr)

		registry, err := repo.GetRegistry(ctx, registryID)
		assert.Nil(t, registry)
		assertWrappedPluginDryRunError(t, err, "get registry", dbErr)

		plugins, err := repo.ListPlugins(ctx)
		assert.Nil(t, plugins)
		assertWrappedPluginDryRunError(t, err, "list plugins", dbErr)

		plugin, err := repo.GetPlugin(ctx, pluginID)
		assert.Nil(t, plugin)
		assertWrappedPluginDryRunError(t, err, "get plugin", dbErr)

		pluginByName, err := repo.GetPluginByName(ctx, "payroll-sync")
		assert.Nil(t, pluginByName)
		assertWrappedPluginDryRunError(t, err, "get plugin by name", dbErr)

		tenantPlugins, err := repo.ListTenantPlugins(ctx, tenantID)
		assert.Nil(t, tenantPlugins)
		assertWrappedPluginDryRunError(t, err, "list tenant plugins", dbErr)

		tenantPlugin, err := repo.GetTenantPlugin(ctx, tenantID, pluginID)
		assert.Nil(t, tenantPlugin)
		assertWrappedPluginDryRunError(t, err, "get tenant plugin", dbErr)

		settings, err := repo.GetTenantPluginSettings(ctx, tenantID, pluginID)
		assert.Nil(t, settings)
		assertWrappedPluginDryRunError(t, err, "get tenant plugin settings", dbErr)

		enabled, err := repo.IsPluginEnabledForTenant(ctx, tenantID, pluginID)
		assert.False(t, enabled)
		assertWrappedPluginDryRunError(t, err, "check tenant plugin", dbErr)

		enabledPlugins, err := repo.ListEnabledPlugins(ctx)
		assert.Nil(t, enabledPlugins)
		assertWrappedPluginDryRunError(t, err, "list enabled plugins", dbErr)

		count, err := repo.CountEnabledTenantsForPlugin(ctx, pluginID)
		assert.Zero(t, count)
		assertWrappedPluginDryRunError(t, err, "count enabled tenants", dbErr)
	})

	t.Run("scan errors", func(t *testing.T) {
		capture := &pluginDryRunSQLCapture{}
		repo := NewGORMRepository(newPluginDryRunDB(t, withPluginDryRunSQLCapture(capture)))

		tenantPlugins, err := repo.GetTenantPluginsWithAll(ctx, tenantID)

		assert.Nil(t, tenantPlugins)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "list tenant plugins")
		assert.ErrorIs(t, err, gorm.ErrDryRunModeUnsupported)
		capture.assertContains(t, `LEFT JOIN tenant_plugins AS tp`)
	})

	t.Run("update errors", func(t *testing.T) {
		repo := NewGORMRepository(newPluginDryRunDB(t, withPluginDryRunUpdateError(dbErr)))

		assertWrappedPluginDryRunError(t, repo.UpdateRegistryLastSynced(ctx, registryID), "update registry last synced", dbErr)
		assertWrappedPluginDryRunError(t, repo.UpdatePlugin(ctx, &domainPlugin), "update plugin", dbErr)

		rows, err := repo.DisableTenantPlugin(ctx, tenantID, pluginID)
		assert.Zero(t, rows)
		assertWrappedPluginDryRunError(t, err, "disable tenant plugin", dbErr)

		assertWrappedPluginDryRunError(t, repo.UpdateTenantPluginSettings(ctx, tenantID, pluginID, json.RawMessage(`{}`)), "update tenant plugin settings", dbErr)
		assertWrappedPluginDryRunError(t, repo.UpdatePluginState(ctx, pluginID, StateEnabled, []string{"read:invoices"}), "update plugin state", dbErr)
		assertWrappedPluginDryRunError(t, repo.DisableAllTenantsForPlugin(ctx, pluginID), "disable plugin for all tenants", dbErr)
	})

	t.Run("update missing rows", func(t *testing.T) {
		repo := NewGORMRepository(newPluginDryRunDB(t, withPluginDryRunUpdateRows(0)))

		err := repo.UpdateTenantPluginSettings(ctx, tenantID, pluginID, json.RawMessage(`{}`))

		require.Error(t, err)
		assert.EqualError(t, err, "tenant plugin not found")
	})

	t.Run("delete errors", func(t *testing.T) {
		repo := NewGORMRepository(newPluginDryRunDB(t, withPluginDryRunDeleteError(dbErr)))

		rows, err := repo.DeleteRegistry(ctx, registryID)
		assert.Zero(t, rows)
		assertWrappedPluginDryRunError(t, err, "delete registry", dbErr)

		rows, err = repo.DeletePlugin(ctx, pluginID)
		assert.Zero(t, rows)
		assertWrappedPluginDryRunError(t, err, "delete plugin", dbErr)

		assertWrappedPluginDryRunError(t, repo.DeleteTenantPlugin(ctx, tenantID, pluginID), "delete tenant plugin", dbErr)
	})
}

func assertWrappedPluginDryRunError(t *testing.T, err error, message string, target error) {
	t.Helper()
	require.Error(t, err)
	assert.Contains(t, err.Error(), message)
	assert.ErrorIs(t, err, target)
}

func pluginDryRunRegistryModel(id uuid.UUID, now time.Time) models.PluginRegistry {
	lastSyncedAt := now.Add(-time.Hour)
	return models.PluginRegistry{
		ID:           id,
		Name:         "official",
		URL:          "https://plugins.example.com",
		Description:  "Official plugins",
		IsOfficial:   true,
		IsActive:     true,
		LastSyncedAt: &lastSyncedAt,
		CreatedAt:    now.Add(-2 * time.Hour),
		UpdatedAt:    now,
	}
}

func pluginDryRunPluginModel(id uuid.UUID, now time.Time) models.Plugin {
	return models.Plugin{
		ID:                 id,
		Name:               "payroll-sync",
		DisplayName:        "Payroll Sync",
		Description:        "Imports payroll activity",
		Version:            "1.2.3",
		RepositoryURL:      "https://github.com/example/payroll-sync",
		RepositoryType:     models.RepoGitHub,
		Author:             "Open Accounting",
		License:            "MIT",
		HomepageURL:        "https://example.com/payroll-sync",
		State:              models.PluginStateEnabled,
		GrantedPermissions: pq.StringArray{"read:invoices", "write:journal"},
		Manifest:           json.RawMessage(`{"name":"payroll-sync"}`),
		InstalledAt:        now.Add(-24 * time.Hour),
		UpdatedAt:          now,
	}
}

func pluginDryRunTenantPluginModel(tenantID, pluginID uuid.UUID, now time.Time, settings json.RawMessage, pluginModel *models.Plugin) models.TenantPlugin {
	enabledAt := now.Add(-time.Hour)
	return models.TenantPlugin{
		ID:        uuid.New(),
		TenantID:  tenantID,
		PluginID:  pluginID,
		IsEnabled: true,
		Settings:  settings,
		EnabledAt: &enabledAt,
		CreatedAt: now.Add(-2 * time.Hour),
		UpdatedAt: now,
		Plugin:    pluginModel,
	}
}

func ptrPluginDryRunModel(model models.Plugin) *models.Plugin {
	return &model
}

func pluginTenantPluginsWithAllColumns() []string {
	return []string{
		"tp_id",
		"tp_tenant_id",
		"tp_plugin_id",
		"tp_is_enabled",
		"tp_settings",
		"tp_enabled_at",
		"tp_created_at",
		"tp_updated_at",
		"id",
		"name",
		"display_name",
		"description",
		"version",
		"repository_url",
		"repository_type",
		"author",
		"license",
		"homepage_url",
		"state",
		"granted_permissions",
		"manifest",
		"installed_at",
		"updated_at",
	}
}

func pluginTenantPluginsWithAllRow(tenantPluginID, tenantID, pluginID uuid.UUID, enabled bool, settings json.RawMessage, now time.Time, pluginModel models.Plugin) []driver.Value {
	return []driver.Value{
		tenantPluginID.String(),
		tenantID.String(),
		pluginID.String(),
		enabled,
		[]byte(settings),
		now,
		now.Add(-2 * time.Hour),
		now,
		pluginModel.ID.String(),
		pluginModel.Name,
		pluginModel.DisplayName,
		pluginModel.Description,
		pluginModel.Version,
		pluginModel.RepositoryURL,
		string(pluginModel.RepositoryType),
		pluginModel.Author,
		pluginModel.License,
		pluginModel.HomepageURL,
		string(pluginModel.State),
		`{"read:invoices","write:journal"}`,
		[]byte(pluginModel.Manifest),
		pluginModel.InstalledAt,
		pluginModel.UpdatedAt,
	}
}

func pluginTenantPluginsWithAllAvailableRow(pluginID uuid.UUID, now time.Time) []driver.Value {
	pluginModel := pluginDryRunPluginModel(pluginID, now)
	pluginModel.Name = "available-plugin"
	pluginModel.DisplayName = "Available Plugin"
	pluginModel.GrantedPermissions = pq.StringArray{}
	return []driver.Value{
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		pluginModel.ID.String(),
		pluginModel.Name,
		pluginModel.DisplayName,
		pluginModel.Description,
		pluginModel.Version,
		pluginModel.RepositoryURL,
		string(pluginModel.RepositoryType),
		pluginModel.Author,
		pluginModel.License,
		pluginModel.HomepageURL,
		string(pluginModel.State),
		`{}`,
		[]byte(pluginModel.Manifest),
		pluginModel.InstalledAt,
		pluginModel.UpdatedAt,
	}
}
