package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/HMB-research/open-accounting/internal/auth"
	"github.com/HMB-research/open-accounting/internal/demo"
	"github.com/HMB-research/open-accounting/internal/scheduler"
	"github.com/HMB-research/open-accounting/internal/tenant"
)

type fakeAPIPool struct {
	pingErr error
	closed  bool
}

func (p *fakeAPIPool) Ping(context.Context) error {
	return p.pingErr
}

func (p *fakeAPIPool) Close() {
	p.closed = true
}

type fakeAPIScheduler struct {
	startErr error
	started  bool
	stopped  bool
}

func (s *fakeAPIScheduler) Start() error {
	s.started = true
	return s.startErr
}

func (s *fakeAPIScheduler) Stop() context.Context {
	s.stopped = true
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	return ctx
}

type fakeAPIPluginLoader struct {
	err   error
	calls int
}

func (l *fakeAPIPluginLoader) LoadEnabledPlugins(context.Context) error {
	l.calls++
	return l.err
}

type fakeAPIServer struct {
	listenErr       error
	shutdownErr     error
	waitForShutdown bool
	listenCalled    bool
	shutdownCalled  bool
	shutdownCh      chan struct{}
	shutdownOnce    sync.Once
}

func (s *fakeAPIServer) ListenAndServe() error {
	s.listenCalled = true
	if s.waitForShutdown {
		<-s.shutdownCh
		return http.ErrServerClosed
	}
	return s.listenErr
}

func (s *fakeAPIServer) Shutdown(context.Context) error {
	s.shutdownCalled = true
	if s.shutdownCh != nil {
		s.shutdownOnce.Do(func() {
			close(s.shutdownCh)
		})
	}
	return s.shutdownErr
}

type fakeDemoStatusReader struct{}

func (fakeDemoStatusReader) ReadDemoStatus(context.Context, string, int) (demo.StatusResponse, error) {
	return demo.StatusResponse{}, nil
}

type fakeDemoResetRepository struct{}

func (fakeDemoResetRepository) ResetDemoData(context.Context, []demo.ResetUser, string) error {
	return nil
}

func newRunAPITestDeps(
	env map[string]string,
	cfg *Config,
	pool apiPool,
	app *apiApplication,
	server apiHTTPServer,
) apiMainDeps {
	return apiMainDeps{
		getenv: func(key string) string {
			return env[key]
		},
		loadConfig: func() *Config {
			return cfg
		},
		newPool: func(context.Context, string) (apiPool, error) {
			return pool, nil
		},
		buildApplication: func(context.Context, *Config, apiPool, scheduler.Config) (*apiApplication, error) {
			return app, nil
		},
		newServer: func(*Config, *apiApplication) apiHTTPServer {
			return server
		},
		signalNotify: func(chan<- os.Signal, ...os.Signal) {},
	}
}

func TestLoadConfigUsesDefaultsAndEnvOverrides(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://db")
	t.Setenv("PORT", "")
	t.Setenv("JWT_SECRET", "")
	t.Setenv("ALLOWED_ORIGINS", "https://app.example.com, https://admin.example.com")
	t.Setenv("APP_ENV", "")
	t.Setenv("ENV", "")
	t.Setenv("GO_ENV", "")
	t.Setenv("RAILWAY_ENVIRONMENT", "")
	t.Setenv("PASSWORD_RESET_BASE_URL", "")
	t.Setenv("PASSWORD_RESET_EXPOSE_TOKEN", "")
	t.Setenv("PASSWORD_RESET_SMTP_HOST", "")
	t.Setenv("PASSWORD_RESET_SMTP_FROM_EMAIL", "")

	cfg := loadConfig()

	assert.Equal(t, "8080", cfg.Port)
	assert.Equal(t, "postgres://db", cfg.DatabaseURL)
	assert.Equal(t, developmentJWTSecret, cfg.JWTSecret)
	assert.Equal(t, 15*time.Minute, cfg.AccessExpiry)
	assert.Equal(t, 7*24*time.Hour, cfg.RefreshExpiry)
	assert.Contains(t, cfg.AllowedOrigins, "http://localhost:5173")
	assert.Contains(t, cfg.AllowedOrigins, "https://app.example.com")
	assert.Contains(t, cfg.AllowedOrigins, "https://admin.example.com")
	assert.False(t, cfg.PasswordReset.ExposeToken)
	assert.Nil(t, cfg.PasswordReset.SMTPConfig)

	t.Setenv("PORT", "9090")
	t.Setenv("JWT_SECRET", "secret")
	t.Setenv("ALLOWED_ORIGINS", "")
	t.Setenv("PASSWORD_RESET_BASE_URL", "https://app.example.com/reset-password")
	t.Setenv("PASSWORD_RESET_EXPOSE_TOKEN", "true")
	t.Setenv("PASSWORD_RESET_SMTP_HOST", "smtp.example.com")
	t.Setenv("PASSWORD_RESET_SMTP_PORT", "2525")
	t.Setenv("PASSWORD_RESET_SMTP_USERNAME", "mailer")
	t.Setenv("PASSWORD_RESET_SMTP_PASSWORD", "smtp-secret")
	t.Setenv("PASSWORD_RESET_SMTP_FROM_EMAIL", "no-reply@example.com")
	t.Setenv("PASSWORD_RESET_SMTP_FROM_NAME", "Open Accounting")
	t.Setenv("PASSWORD_RESET_SMTP_USE_TLS", "true")

	cfg = loadConfig()
	assert.Equal(t, "9090", cfg.Port)
	assert.Equal(t, "secret", cfg.JWTSecret)
	assert.Equal(t, []string{"http://localhost:5173", "http://localhost:3000"}, cfg.AllowedOrigins)
	assert.Equal(t, "https://app.example.com/reset-password", cfg.PasswordReset.BaseURL)
	assert.True(t, cfg.PasswordReset.ExposeToken)
	require.NotNil(t, cfg.PasswordReset.SMTPConfig)
	assert.Equal(t, "smtp.example.com", cfg.PasswordReset.SMTPConfig.Host)
	assert.Equal(t, 2525, cfg.PasswordReset.SMTPConfig.Port)
	assert.Equal(t, "mailer", cfg.PasswordReset.SMTPConfig.Username)
	assert.Equal(t, "smtp-secret", cfg.PasswordReset.SMTPConfig.Password)
	assert.Equal(t, "no-reply@example.com", cfg.PasswordReset.SMTPConfig.FromEmail)
	assert.Equal(t, "Open Accounting", cfg.PasswordReset.SMTPConfig.FromName)
	assert.True(t, cfg.PasswordReset.SMTPConfig.UseTLS)
}

func TestRunAPIWithInjectedDependencies(t *testing.T) {
	env := map[string]string{
		"LOG_LEVEL":                                   "debug",
		"RECURRING_INVOICE_SCHEDULE":                  "0 1 * * *",
		"RECURRING_JOURNAL_ENTRY_SCHEDULE":            "0 2 * * *",
		"DOCUMENT_RETENTION_REMINDER_SCHEDULE":        "0 3 * * *",
		"DOCUMENT_RETENTION_REMINDER_HORIZON_DAYS":    "45",
		"DOCUMENT_RETENTION_REMINDER_INCLUDE_MISSING": "true",
		"SCHEDULER_ENABLED":                           "false",
	}
	cfg := &Config{Port: "8080", DatabaseURL: "postgres://unit", JWTSecret: "secret"}
	pool := &fakeAPIPool{}
	fakeScheduler := &fakeAPIScheduler{}
	pluginLoader := &fakeAPIPluginLoader{}
	server := &fakeAPIServer{listenErr: http.ErrServerClosed}
	var gotDatabaseURL string
	var gotSchedulerConfig scheduler.Config

	deps := newRunAPITestDeps(env, cfg, pool, &apiApplication{
		scheduler:    fakeScheduler,
		pluginLoader: pluginLoader,
	}, server)
	deps.newPool = func(_ context.Context, databaseURL string) (apiPool, error) {
		gotDatabaseURL = databaseURL
		return pool, nil
	}
	deps.buildApplication = func(_ context.Context, gotCfg *Config, gotPool apiPool, schedulerConfig scheduler.Config) (*apiApplication, error) {
		require.Same(t, cfg, gotCfg)
		require.Same(t, pool, gotPool)
		gotSchedulerConfig = schedulerConfig
		return &apiApplication{scheduler: fakeScheduler, pluginLoader: pluginLoader}, nil
	}

	err := runAPI(context.Background(), deps)

	require.NoError(t, err)
	assert.Equal(t, "postgres://unit", gotDatabaseURL)
	assert.True(t, pool.closed)
	assert.True(t, fakeScheduler.started)
	assert.False(t, fakeScheduler.stopped)
	assert.Equal(t, 1, pluginLoader.calls)
	assert.True(t, server.listenCalled)
	assert.Equal(t, "0 1 * * *", gotSchedulerConfig.RecurringInvoiceSchedule)
	assert.Equal(t, "0 2 * * *", gotSchedulerConfig.RecurringJournalEntrySchedule)
	assert.Equal(t, "0 3 * * *", gotSchedulerConfig.DocumentRetentionReminderSchedule)
	assert.Equal(t, 45, gotSchedulerConfig.DocumentRetentionReminderHorizonDays)
	assert.True(t, gotSchedulerConfig.DocumentRetentionReminderIncludeMissing)
	assert.False(t, gotSchedulerConfig.Enabled)
}

func TestRunAPIErrorBranches(t *testing.T) {
	baseConfig := &Config{Port: "8080", DatabaseURL: "postgres://unit", JWTSecret: "secret"}

	t.Run("connect error", func(t *testing.T) {
		deps := newRunAPITestDeps(nil, baseConfig, nil, nil, nil)
		deps.newPool = func(context.Context, string) (apiPool, error) {
			return nil, errors.New("connect failed")
		}

		err := runAPI(context.Background(), deps)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to connect to database")
	})

	t.Run("ping error closes pool", func(t *testing.T) {
		pool := &fakeAPIPool{pingErr: errors.New("ping failed")}
		deps := newRunAPITestDeps(nil, baseConfig, pool, nil, nil)

		err := runAPI(context.Background(), deps)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to ping database")
		assert.True(t, pool.closed)
	})

	t.Run("build application error closes pool", func(t *testing.T) {
		pool := &fakeAPIPool{}
		deps := newRunAPITestDeps(nil, baseConfig, pool, nil, nil)
		deps.buildApplication = func(context.Context, *Config, apiPool, scheduler.Config) (*apiApplication, error) {
			return nil, errors.New("build failed")
		}

		err := runAPI(context.Background(), deps)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "build failed")
		assert.True(t, pool.closed)
	})

	t.Run("plugin and scheduler warnings do not stop serving", func(t *testing.T) {
		pool := &fakeAPIPool{}
		fakeScheduler := &fakeAPIScheduler{startErr: errors.New("scheduler failed")}
		pluginLoader := &fakeAPIPluginLoader{err: errors.New("plugins failed")}
		server := &fakeAPIServer{listenErr: http.ErrServerClosed}
		deps := newRunAPITestDeps(
			map[string]string{"LOG_LEVEL": "not-a-level"},
			baseConfig,
			pool,
			&apiApplication{scheduler: fakeScheduler, pluginLoader: pluginLoader},
			server,
		)

		err := runAPI(context.Background(), deps)

		require.NoError(t, err)
		assert.True(t, fakeScheduler.started)
		assert.Equal(t, 1, pluginLoader.calls)
		assert.True(t, server.listenCalled)
		assert.True(t, pool.closed)
	})

	t.Run("listen error is returned", func(t *testing.T) {
		pool := &fakeAPIPool{}
		server := &fakeAPIServer{listenErr: errors.New("listen failed")}
		deps := newRunAPITestDeps(
			nil,
			baseConfig,
			pool,
			&apiApplication{scheduler: &fakeAPIScheduler{}, pluginLoader: &fakeAPIPluginLoader{}},
			server,
		)

		err := runAPI(context.Background(), deps)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "server failed")
	})

	t.Run("signal shuts down scheduler and server", func(t *testing.T) {
		pool := &fakeAPIPool{}
		fakeScheduler := &fakeAPIScheduler{}
		server := &fakeAPIServer{waitForShutdown: true, shutdownCh: make(chan struct{})}
		deps := newRunAPITestDeps(
			nil,
			baseConfig,
			pool,
			&apiApplication{scheduler: fakeScheduler, pluginLoader: &fakeAPIPluginLoader{}},
			server,
		)
		deps.signalNotify = func(ch chan<- os.Signal, _ ...os.Signal) {
			ch <- syscall.SIGTERM
		}

		err := runAPI(context.Background(), deps)

		require.NoError(t, err)
		assert.True(t, fakeScheduler.stopped)
		assert.True(t, server.shutdownCalled)
	})

	t.Run("shutdown error is logged and server close still succeeds", func(t *testing.T) {
		pool := &fakeAPIPool{}
		fakeScheduler := &fakeAPIScheduler{}
		server := &fakeAPIServer{
			waitForShutdown: true,
			shutdownCh:      make(chan struct{}),
			shutdownErr:     errors.New("shutdown failed"),
		}
		deps := newRunAPITestDeps(
			nil,
			baseConfig,
			pool,
			&apiApplication{scheduler: fakeScheduler, pluginLoader: &fakeAPIPluginLoader{}},
			server,
		)
		deps.signalNotify = func(ch chan<- os.Signal, _ ...os.Signal) {
			ch <- syscall.SIGTERM
		}

		err := runAPI(context.Background(), deps)

		require.NoError(t, err)
		assert.True(t, fakeScheduler.stopped)
		assert.True(t, server.shutdownCalled)
	})
}

func TestBuildProductionAPIApplicationBranches(t *testing.T) {
	cfg := &Config{
		Port:          "8080",
		DatabaseURL:   "postgres://unit",
		JWTSecret:     "secret",
		AccessExpiry:  time.Minute,
		RefreshExpiry: time.Hour,
		DocumentsDir:  t.TempDir(),
	}

	t.Run("rejects non pgx pool", func(t *testing.T) {
		app, err := buildProductionAPIApplication(context.Background(), cfg, &fakeAPIPool{}, scheduler.DefaultConfig())
		require.Error(t, err)
		assert.Nil(t, app)
		assert.Contains(t, err.Error(), "pgx pool")
	})

	t.Run("document storage error", func(t *testing.T) {
		badConfig := *cfg
		badConfig.DocumentsDir = ""

		app, err := buildProductionAPIApplication(context.Background(), &badConfig, (*pgxpool.Pool)(nil), scheduler.DefaultConfig())

		require.Error(t, err)
		assert.Nil(t, app)
		assert.Contains(t, err.Error(), "document storage")
	})

	t.Run("demo status reader error", func(t *testing.T) {
		app, err := buildProductionAPIApplication(context.Background(), cfg, (*pgxpool.Pool)(nil), scheduler.DefaultConfig())

		require.Error(t, err)
		assert.Nil(t, app)
		assert.Contains(t, err.Error(), "demo status reader")
	})

	t.Run("demo reset service error", func(t *testing.T) {
		oldStatusReader := newDemoStatusReader
		oldResetService := newDemoResetService
		defer func() {
			newDemoStatusReader = oldStatusReader
			newDemoResetService = oldResetService
		}()
		newDemoStatusReader = func(*pgxpool.Pool) (demo.StatusReader, error) {
			return fakeDemoStatusReader{}, nil
		}
		newDemoResetService = func(context.Context, *pgxpool.Pool, demo.SeedScriptFunc) (*demo.ResetService, error) {
			return nil, errors.New("reset failed")
		}

		app, err := buildProductionAPIApplication(context.Background(), cfg, (*pgxpool.Pool)(nil), scheduler.DefaultConfig())

		require.Error(t, err)
		assert.Nil(t, app)
		assert.Contains(t, err.Error(), "demo reset service")
	})

	t.Run("db backed services still require usable pool", func(t *testing.T) {
		oldStatusReader := newDemoStatusReader
		oldResetService := newDemoResetService
		defer func() {
			newDemoStatusReader = oldStatusReader
			newDemoResetService = oldResetService
		}()
		newDemoStatusReader = func(*pgxpool.Pool) (demo.StatusReader, error) {
			return fakeDemoStatusReader{}, nil
		}
		newDemoResetService = func(_ context.Context, _ *pgxpool.Pool, seed demo.SeedScriptFunc) (*demo.ResetService, error) {
			return demo.NewResetServiceWithRepository(fakeDemoResetRepository{}, seed), nil
		}

		schedulerConfig := scheduler.DefaultConfig()
		schedulerConfig.Enabled = false
		assert.Panics(t, func() {
			_, _ = buildProductionAPIApplication(context.Background(), cfg, (*pgxpool.Pool)(nil), schedulerConfig)
		})
	})
}

func TestLoadSchedulerConfigBranches(t *testing.T) {
	cfg := loadSchedulerConfig(func(key string) string {
		values := map[string]string{
			"RECURRING_INVOICE_SCHEDULE":                  "0 4 * * *",
			"RECURRING_JOURNAL_ENTRY_SCHEDULE":            "0 5 * * *",
			"DOCUMENT_RETENTION_REMINDER_SCHEDULE":        "0 6 * * *",
			"DOCUMENT_RETENTION_REMINDER_HORIZON_DAYS":    "60",
			"DOCUMENT_RETENTION_REMINDER_INCLUDE_MISSING": "true",
			"SCHEDULER_ENABLED":                           "false",
		}
		return values[key]
	})
	assert.Equal(t, "0 4 * * *", cfg.RecurringInvoiceSchedule)
	assert.Equal(t, "0 5 * * *", cfg.RecurringJournalEntrySchedule)
	assert.Equal(t, "0 6 * * *", cfg.DocumentRetentionReminderSchedule)
	assert.Equal(t, 60, cfg.DocumentRetentionReminderHorizonDays)
	assert.True(t, cfg.DocumentRetentionReminderIncludeMissing)
	assert.False(t, cfg.Enabled)

	defaults := scheduler.DefaultConfig()
	cfg = loadSchedulerConfig(func(key string) string {
		values := map[string]string{
			"DOCUMENT_RETENTION_REMINDER_HORIZON_DAYS":    "-1",
			"DOCUMENT_RETENTION_REMINDER_INCLUDE_MISSING": "not-bool",
		}
		return values[key]
	})
	assert.Equal(t, defaults.DocumentRetentionReminderHorizonDays, cfg.DocumentRetentionReminderHorizonDays)
	assert.Equal(t, defaults.DocumentRetentionReminderIncludeMissing, cfg.DocumentRetentionReminderIncludeMissing)
	assert.Equal(t, defaults.Enabled, cfg.Enabled)
}

func TestLoadDocumentRetentionReminderPolicy(t *testing.T) {
	t.Setenv("DOCUMENT_RETENTION_REMINDER_MAX_ATTEMPTS", "5")
	t.Setenv("DOCUMENT_RETENTION_REMINDER_ESCALATE_AFTER_ATTEMPTS", "4")

	policy := loadDocumentRetentionReminderPolicy()
	assert.Equal(t, 5, policy.MaxAttempts)
	assert.Equal(t, 4, policy.EscalateAfterAttempts)

	t.Setenv("DOCUMENT_RETENTION_REMINDER_MAX_ATTEMPTS", "0")
	t.Setenv("DOCUMENT_RETENTION_REMINDER_ESCALATE_AFTER_ATTEMPTS", "bad")

	policy = loadDocumentRetentionReminderPolicy()
	assert.Zero(t, policy.MaxAttempts)
	assert.Zero(t, policy.EscalateAfterAttempts)
}

func TestProductionConfigValidation(t *testing.T) {
	secret, err := resolveJWTSecret("", true)
	require.Error(t, err)
	assert.Empty(t, secret)

	secret, err = resolveJWTSecret("short", true)
	require.Error(t, err)
	assert.Empty(t, secret)

	secret, err = resolveJWTSecret("01234567890123456789012345678901", true)
	require.NoError(t, err)
	assert.Equal(t, "01234567890123456789012345678901", secret)

	origins, err := resolveAllowedOrigins("", true)
	require.Error(t, err)
	assert.Empty(t, origins)

	origins, err = resolveAllowedOrigins("https://app.example.com, https://admin.example.com", true)
	require.NoError(t, err)
	assert.Equal(t, []string{"https://app.example.com", "https://admin.example.com"}, origins)
	assert.NotContains(t, origins, "http://localhost:5173")
}

func TestSetupRouterRegistersCoreRoutes(t *testing.T) {
	cfg := &Config{
		AllowedOrigins: []string{"http://localhost:5173"},
	}
	tokenService := auth.NewTokenService("secret", time.Minute, time.Hour)

	t.Setenv("CORS_DEBUG", "true")
	t.Setenv("DEMO_MODE", "false")

	router := setupRouter(cfg, &Handlers{}, tokenService)
	require.NotNil(t, router)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "OK", rr.Body.String())

	routes := make(map[string]string)
	err := chi.Walk(router, func(method string, route string, handler http.Handler, middlewares ...func(http.Handler) http.Handler) error {
		routes[method+" "+route] = route
		return nil
	})
	require.NoError(t, err)

	assert.Contains(t, routes, "GET /health")
	assert.Contains(t, routes, "POST /api/v1/auth/login")
	assert.Contains(t, routes, "POST /api/v1/auth/logout")
	assert.Contains(t, routes, "POST /api/v1/auth/password-reset/request")
	assert.Contains(t, routes, "POST /api/v1/auth/password-reset/confirm")
	assert.Contains(t, routes, "GET /api/v1/auth/sessions")
	assert.Contains(t, routes, "DELETE /api/v1/auth/sessions")
	assert.Contains(t, routes, "DELETE /api/v1/auth/sessions/{sessionID}")
	assert.Contains(t, routes, "GET /api/v1/auth/security-events")
	assert.Contains(t, routes, "GET /api/v1/me")
	assert.Contains(t, routes, "POST /api/v1/tenants/{tenantID}/complete-onboarding")
	assert.Contains(t, routes, "GET /api/v1/tenants/{tenantID}/users/{userID}/sessions")
	assert.Contains(t, routes, "DELETE /api/v1/tenants/{tenantID}/users/{userID}/sessions")
	assert.Contains(t, routes, "DELETE /api/v1/tenants/{tenantID}/users/{userID}/sessions/{sessionID}")
	assert.Contains(t, routes, "GET /api/v1/tenants/{tenantID}/users/{userID}/api-tokens")
	assert.Contains(t, routes, "DELETE /api/v1/tenants/{tenantID}/users/{userID}/api-tokens/{tokenID}")
	assert.Contains(t, routes, "GET /api/v1/tenants/{tenantID}/users/{userID}/security-events")
	assert.Contains(t, routes, "PUT /api/v1/tenants/{tenantID}/users/{userID}/status")
	assert.Contains(t, routes, "GET /api/v1/tenants/{tenantID}/period-close-events")
	assert.Contains(t, routes, "POST /api/v1/tenants/{tenantID}/period-close")
	assert.Contains(t, routes, "POST /api/v1/tenants/{tenantID}/period-reopen")
	assert.Contains(t, routes, "GET /api/v1/tenants/{tenantID}/year-end-close-status")
	assert.Contains(t, routes, "GET /api/v1/tenants/{tenantID}/year-end-close-pack")
	assert.Contains(t, routes, "POST /api/v1/tenants/{tenantID}/year-end-carry-forward")
	assert.Contains(t, routes, "POST /api/v1/tenants/{tenantID}/year-end-carry-forward/reverse")
	assert.Contains(t, routes, "GET /api/v1/tenants/{tenantID}/journal-entries")
	assert.Contains(t, routes, "POST /api/v1/tenants/{tenantID}/quotes/import")
	assert.Contains(t, routes, "POST /api/v1/tenants/{tenantID}/orders/import")
	assert.Contains(t, routes, "GET /api/v1/tenants/{tenantID}/orders/{orderID}/stock-check")
	assert.Contains(t, routes, "GET /api/v1/tenants/{tenantID}/orders/{orderID}/stock-reservations")
	assert.Contains(t, routes, "GET /api/v1/tenants/{tenantID}/orders/{orderID}/pick-list")
	assert.Contains(t, routes, "POST /api/v1/tenants/{tenantID}/orders/{orderID}/reserve-stock")
	assert.Contains(t, routes, "POST /api/v1/tenants/{tenantID}/orders/{orderID}/release-stock")
	assert.Contains(t, routes, "POST /api/v1/tenants/{tenantID}/orders/{orderID}/convert-to-invoice")
	assert.Contains(t, routes, "POST /api/v1/tenants/{tenantID}/recurring-invoices/import")
	assert.Contains(t, routes, "GET /api/v1/tenants/{tenantID}/documents")
	assert.Contains(t, routes, "POST /api/v1/tenants/{tenantID}/documents/review-summary")
	assert.Contains(t, routes, "POST /api/v1/tenants/{tenantID}/documents/evidence-policy")
	assert.Contains(t, routes, "GET /api/v1/tenants/{tenantID}/documents/retention")
	assert.Contains(t, routes, "POST /api/v1/tenants/{tenantID}/documents/purge")
	assert.Contains(t, routes, "POST /api/v1/tenants/{tenantID}/documents")
	assert.Contains(t, routes, "GET /api/v1/tenants/{tenantID}/documents/{documentID}/download")
	assert.Contains(t, routes, "PATCH /api/v1/tenants/{tenantID}/documents/{documentID}/retention")
	assert.Contains(t, routes, "PATCH /api/v1/tenants/{tenantID}/documents/{documentID}/lifecycle")
	assert.Contains(t, routes, "POST /api/v1/tenants/{tenantID}/documents/{documentID}/review")
	assert.Contains(t, routes, "POST /api/v1/tenants/{tenantID}/documents/{documentID}/mark-reviewed")
	assert.Contains(t, routes, "DELETE /api/v1/tenants/{tenantID}/documents/{documentID}")
	assert.Contains(t, routes, "GET /api/v1/tenants/{tenantID}/migration/provider-presets")
	assert.Contains(t, routes, "POST /api/v1/tenants/{tenantID}/migration/validate")
	assert.Contains(t, routes, "POST /api/v1/tenants/{tenantID}/migration/execution-plan")
	assert.Contains(t, routes, "POST /api/v1/tenants/{tenantID}/migration/execute")
	assert.Contains(t, routes, "GET /api/v1/tenants/{tenantID}/migration/execution-runs")
	assert.Contains(t, routes, "GET /api/v1/tenants/{tenantID}/migration/execution-runs/{runID}")
	assert.Contains(t, routes, "GET /api/v1/tenants/{tenantID}/migration/execution-runs/{runID}/events")
	assert.Contains(t, routes, "POST /api/v1/tenants/{tenantID}/bank-transactions/{transactionID}/review")
	assert.Contains(t, routes, "GET /api/v1/tenants/{tenantID}/webhooks/events")
	assert.Contains(t, routes, "POST /api/v1/tenants/{tenantID}/webhooks/{webhookID}/test")
	assert.Contains(t, routes, "GET /api/v1/tenants/{tenantID}/expenses")
	assert.Contains(t, routes, "POST /api/v1/tenants/{tenantID}/expenses/import")
	assert.Contains(t, routes, "POST /api/v1/tenants/{tenantID}/expenses/{expenseID}/post")
	assert.Contains(t, routes, "POST /api/v1/tenants/{tenantID}/plugins/{pluginID}/runtime/*")
	assert.Contains(t, routes, "GET /api/v1/admin/plugins")
	assert.Contains(t, routes, "GET /api/v1/admin/plugins/{id}/runtime")
	assert.Contains(t, routes, "POST /api/v1/admin/plugins/{id}/runtime/restart")
	assert.Contains(t, routes, "GET /swagger/*")
}

func TestTenantDetailRoutesAreNotShadowedByTenantScopedRoutes(t *testing.T) {
	cfg := &Config{AllowedOrigins: []string{"http://localhost:5173"}}
	tokenService := auth.NewTokenService("secret", time.Minute, time.Hour)

	t.Setenv("CORS_DEBUG", "true")
	t.Setenv("DEMO_MODE", "false")

	router := setupRouter(cfg, &Handlers{}, tokenService)
	require.NotNil(t, router)

	for _, tt := range []struct {
		method string
		path   string
	}{
		{method: http.MethodGet, path: "/api/v1/tenants/tenant-1"},
		{method: http.MethodPut, path: "/api/v1/tenants/tenant-1"},
	} {
		req := httptest.NewRequest(tt.method, tt.path, nil)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusUnauthorized, rr.Code, "%s %s should be protected, not missing", tt.method, tt.path)
	}
}

func TestSetupRouterDisablesRateLimitInDemoMode(t *testing.T) {
	cfg := &Config{AllowedOrigins: []string{"http://localhost:5173"}}
	tokenService := auth.NewTokenService("secret", time.Minute, time.Hour)

	t.Setenv("DEMO_MODE", "true")
	t.Setenv("CORS_DEBUG", "")

	router := setupRouter(cfg, &Handlers{}, tokenService)
	require.NotNil(t, router)

	req := httptest.NewRequest(http.MethodOptions, "/health", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	req.Header.Set("Access-Control-Request-Method", http.MethodGet)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.NotEqual(t, http.StatusNotFound, rr.Code)
	assert.Equal(t, "http://localhost:5173", rr.Header().Get("Access-Control-Allow-Origin"))
}

func TestAdminRoutesRequireTenantOwnerOrAdmin(t *testing.T) {
	cfg := &Config{AllowedOrigins: []string{"http://localhost:5173"}}
	tokenService := auth.NewTokenService("secret", time.Minute, time.Hour)
	handlers, _ := setupPluginTestHandlers(t)
	tenantRepo := newMockTenantRepository()
	tenantRepo.addTestTenant("tenant-1", "Tenant One", "tenant-one")
	now := time.Now()
	tenantRepo.tenantUsers["tenant-1"] = []tenant.TenantUser{
		{TenantID: "tenant-1", UserID: "owner-1", Role: tenant.RoleOwner, IsActive: true, CreatedAt: now},
		{TenantID: "tenant-1", UserID: "admin-1", Role: tenant.RoleAdmin, IsActive: true, CreatedAt: now},
		{TenantID: "tenant-1", UserID: "accountant-1", Role: tenant.RoleAccountant, IsActive: true, CreatedAt: now},
		{TenantID: "tenant-1", UserID: "viewer-1", Role: tenant.RoleViewer, IsActive: true, CreatedAt: now},
		{TenantID: "tenant-1", UserID: "inactive-owner", Role: tenant.RoleOwner, IsActive: false, CreatedAt: now},
	}
	handlers.tenantService = newTestTenantService(tenantRepo)

	t.Setenv("DEMO_MODE", "true")
	t.Setenv("CORS_DEBUG", "")
	router := setupRouter(cfg, handlers, tokenService)

	tests := []struct {
		name         string
		userID       string
		tenantID     string
		claimRole    string
		headerTenant string
		bearerToken  bool
		wantStatus   int
		wantError    string
	}{
		{
			name:       "missing bearer token",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:        "authenticated user without tenant context",
			userID:      "admin-1",
			claimRole:   tenant.RoleAdmin,
			bearerToken: true,
			wantStatus:  http.StatusForbidden,
			wantError:   "Tenant admin context required",
		},
		{
			name:         "explicit tenant header accepts current admin membership",
			userID:       "admin-1",
			claimRole:    tenant.RoleViewer,
			headerTenant: "tenant-1",
			bearerToken:  true,
			wantStatus:   http.StatusOK,
		},
		{
			name:         "explicit tenant header rejects current viewer membership",
			userID:       "viewer-1",
			claimRole:    tenant.RoleAdmin,
			headerTenant: "tenant-1",
			bearerToken:  true,
			wantStatus:   http.StatusForbidden,
			wantError:    "Insufficient permissions",
		},
		{
			name:        "stale admin claim with current viewer membership is rejected",
			userID:      "viewer-1",
			tenantID:    "tenant-1",
			claimRole:   tenant.RoleAdmin,
			bearerToken: true,
			wantStatus:  http.StatusForbidden,
			wantError:   "Insufficient permissions",
		},
		{
			name:        "accountant membership is rejected",
			userID:      "accountant-1",
			tenantID:    "tenant-1",
			claimRole:   tenant.RoleAccountant,
			bearerToken: true,
			wantStatus:  http.StatusForbidden,
			wantError:   "Insufficient permissions",
		},
		{
			name:        "inactive owner membership is rejected",
			userID:      "inactive-owner",
			tenantID:    "tenant-1",
			claimRole:   tenant.RoleOwner,
			bearerToken: true,
			wantStatus:  http.StatusForbidden,
			wantError:   "Access denied",
		},
		{
			name:        "current owner membership is accepted",
			userID:      "owner-1",
			tenantID:    "tenant-1",
			claimRole:   tenant.RoleViewer,
			bearerToken: true,
			wantStatus:  http.StatusOK,
		},
		{
			name:        "current admin membership is accepted",
			userID:      "admin-1",
			tenantID:    "tenant-1",
			claimRole:   tenant.RoleViewer,
			bearerToken: true,
			wantStatus:  http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/plugins", nil)
			if tt.bearerToken {
				token, err := tokenService.GenerateAccessToken(tt.userID, tt.userID+"@example.com", tt.tenantID, tt.claimRole)
				require.NoError(t, err)
				req.Header.Set("Authorization", "Bearer "+token)
			}
			if tt.headerTenant != "" {
				req.Header.Set("X-Tenant-ID", tt.headerTenant)
			}

			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)
			assert.Equal(t, tt.wantStatus, rr.Code, "response body: %s", rr.Body.String())
			if tt.wantError != "" {
				assert.Contains(t, rr.Body.String(), tt.wantError)
			}
		})
	}
}

func TestRequireTenantPermissionUsesCurrentTenantMembership(t *testing.T) {
	tenantRepo := newMockTenantRepository()
	tenantRepo.addTestTenant("tenant-1", "Tenant One", "tenant-one")
	now := time.Now()
	tenantRepo.tenantUsers["tenant-1"] = []tenant.TenantUser{
		{TenantID: "tenant-1", UserID: "accountant-1", Role: tenant.RoleAccountant, IsActive: true, CreatedAt: now},
		{TenantID: "tenant-1", UserID: "viewer-1", Role: tenant.RoleViewer, IsActive: true, CreatedAt: now},
		{TenantID: "tenant-1", UserID: "inactive-admin", Role: tenant.RoleAdmin, IsActive: false, CreatedAt: now},
	}
	handlers := &Handlers{tenantService: newTestTenantService(tenantRepo)}
	requireAccountingWrite := handlers.RequireTenantPermission(func(perms tenant.RolePermissions) bool {
		return perms.CanCreateEntries
	})

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, ok := auth.GetClaims(r.Context())
		require.True(t, ok)
		assert.Equal(t, tenant.RoleAccountant, claims.Role)
		w.WriteHeader(http.StatusNoContent)
	})
	handler := requireAccountingWrite(next)

	tests := []struct {
		name       string
		tenantID   string
		claims     *auth.Claims
		wantStatus int
		wantError  string
	}{
		{
			name:       "missing auth claims",
			tenantID:   "tenant-1",
			wantStatus: http.StatusUnauthorized,
			wantError:  "Authentication required",
		},
		{
			name:       "stale admin claim cannot bypass current viewer role",
			tenantID:   "tenant-1",
			claims:     createTestClaims("viewer-1", "viewer@example.com", "tenant-1", tenant.RoleAdmin),
			wantStatus: http.StatusForbidden,
			wantError:  "Insufficient permissions",
		},
		{
			name:       "inactive admin membership is rejected",
			tenantID:   "tenant-1",
			claims:     createTestClaims("inactive-admin", "inactive@example.com", "tenant-1", tenant.RoleAdmin),
			wantStatus: http.StatusForbidden,
			wantError:  "Access denied",
		},
		{
			name:       "current accountant membership is accepted despite stale viewer claim",
			tenantID:   "tenant-1",
			claims:     createTestClaims("accountant-1", "accountant@example.com", "tenant-1", tenant.RoleViewer),
			wantStatus: http.StatusNoContent,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := makeAuthenticatedRequest(http.MethodPost, "/tenants/"+tt.tenantID+"/protected", nil, tt.claims)
			req = withURLParams(req, map[string]string{"tenantID": tt.tenantID})
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			assert.Equal(t, tt.wantStatus, rr.Code, "response body: %s", rr.Body.String())
			if tt.wantError != "" {
				assert.Contains(t, rr.Body.String(), tt.wantError)
			}
		})
	}
}

func TestRequireTenantWritePermissionAllowsReadOnlyMethods(t *testing.T) {
	handlers := &Handlers{}
	handler := handlers.RequireTenantWritePermission(func(tenant.RolePermissions) bool {
		return false
	})(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	for _, method := range []string{http.MethodGet, http.MethodHead, http.MethodOptions} {
		t.Run(method, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, httptest.NewRequest(method, "/api/v1/tenants/tenant-1/accounts", nil))
			assert.Equal(t, http.StatusNoContent, recorder.Code)
		})
	}
}

func TestTenantSettingsWritesRejectAccountantMembership(t *testing.T) {
	cfg := &Config{AllowedOrigins: []string{"http://localhost:5173"}}
	tokenService := auth.NewTokenService("secret", time.Minute, time.Hour)
	tenantRepo := newMockTenantRepository()
	tenantRepo.addTestTenant("tenant-1", "Tenant One", "tenant-one")
	tenantRepo.tenantUsers["tenant-1"] = []tenant.TenantUser{
		{TenantID: "tenant-1", UserID: "accountant-1", Role: tenant.RoleAccountant, IsActive: true, CreatedAt: time.Now()},
	}
	handlers := &Handlers{tenantService: newTestTenantService(tenantRepo)}

	t.Setenv("DEMO_MODE", "true")
	t.Setenv("CORS_DEBUG", "")
	router := setupRouter(cfg, handlers, tokenService)
	token, err := tokenService.GenerateAccessToken("accountant-1", "accountant@example.com", "tenant-1", tenant.RoleAdmin)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/tenants/tenant-1/settings/smtp", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	assert.Equal(t, http.StatusForbidden, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "Insufficient permissions")
}

func TestSensitiveTenantRoutesRejectViewerMembership(t *testing.T) {
	cfg := &Config{AllowedOrigins: []string{"http://localhost:5173"}}
	tokenService := auth.NewTokenService("secret", time.Minute, time.Hour)
	tenantRepo := newMockTenantRepository()
	tenantRepo.addTestTenant("tenant-1", "Tenant One", "tenant-one")
	tenantRepo.tenantUsers["tenant-1"] = []tenant.TenantUser{
		{TenantID: "tenant-1", UserID: "viewer-1", Role: tenant.RoleViewer, IsActive: true, CreatedAt: time.Now()},
	}
	handlers := &Handlers{tenantService: newTestTenantService(tenantRepo)}

	t.Setenv("DEMO_MODE", "true")
	t.Setenv("CORS_DEBUG", "")
	router := setupRouter(cfg, handlers, tokenService)
	token, err := tokenService.GenerateAccessToken("viewer-1", "viewer@example.com", "tenant-1", tenant.RoleAdmin)
	require.NoError(t, err)

	for _, tt := range []struct {
		name   string
		method string
		path   string
	}{
		{name: "smtp settings", method: http.MethodPut, path: "/api/v1/tenants/tenant-1/settings/smtp"},
		{name: "webhook creation", method: http.MethodPost, path: "/api/v1/tenants/tenant-1/webhooks"},
		{name: "tenant plugin enablement", method: http.MethodPost, path: "/api/v1/tenants/tenant-1/plugins/11111111-1111-1111-1111-111111111111/enable"},
		{name: "migration execution", method: http.MethodPost, path: "/api/v1/tenants/tenant-1/migration/execute"},
		{name: "payroll history import", method: http.MethodPost, path: "/api/v1/tenants/tenant-1/payroll-runs/import-history"},
		{name: "leave balance import", method: http.MethodPost, path: "/api/v1/tenants/tenant-1/leave-balances/import"},
		{name: "TSD history import", method: http.MethodPost, path: "/api/v1/tenants/tenant-1/tsd/import-history"},
		{name: "KMD history import", method: http.MethodPost, path: "/api/v1/tenants/tenant-1/tax/kmd/import-history"},
		{name: "account creation", method: http.MethodPost, path: "/api/v1/tenants/tenant-1/accounts"},
		{name: "account deletion", method: http.MethodDelete, path: "/api/v1/tenants/tenant-1/accounts/account-1"},
		{name: "journal entry creation", method: http.MethodPost, path: "/api/v1/tenants/tenant-1/journal-entries"},
		{name: "invoice creation", method: http.MethodPost, path: "/api/v1/tenants/tenant-1/invoices"},
		{name: "payment creation", method: http.MethodPost, path: "/api/v1/tenants/tenant-1/payments"},
		{name: "bank transaction matching", method: http.MethodPost, path: "/api/v1/tenants/tenant-1/bank-transactions/transaction-1/match"},
		{name: "expense posting", method: http.MethodPost, path: "/api/v1/tenants/tenant-1/expenses/expense-1/post"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			req.Header.Set("Authorization", "Bearer "+token)
			rr := httptest.NewRecorder()

			router.ServeHTTP(rr, req)

			assert.Equal(t, http.StatusForbidden, rr.Code, "response body: %s", rr.Body.String())
			assert.Contains(t, rr.Body.String(), "Insufficient permissions")
		})
	}
}
