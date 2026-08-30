package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	_ "github.com/HMB-research/open-accounting/docs"
	"github.com/HMB-research/open-accounting/internal/accounting"
	"github.com/HMB-research/open-accounting/internal/analytics"
	"github.com/HMB-research/open-accounting/internal/apitoken"
	"github.com/HMB-research/open-accounting/internal/assets"
	"github.com/HMB-research/open-accounting/internal/auth"
	"github.com/HMB-research/open-accounting/internal/banking"
	"github.com/HMB-research/open-accounting/internal/contacts"
	"github.com/HMB-research/open-accounting/internal/cutover"
	"github.com/HMB-research/open-accounting/internal/demo"
	"github.com/HMB-research/open-accounting/internal/documents"
	"github.com/HMB-research/open-accounting/internal/email"
	"github.com/HMB-research/open-accounting/internal/expenses"
	"github.com/HMB-research/open-accounting/internal/importdelivery"
	"github.com/HMB-research/open-accounting/internal/importsession"
	"github.com/HMB-research/open-accounting/internal/inventory"
	"github.com/HMB-research/open-accounting/internal/invoicing"
	secmiddleware "github.com/HMB-research/open-accounting/internal/middleware"
	"github.com/HMB-research/open-accounting/internal/orders"
	"github.com/HMB-research/open-accounting/internal/payments"
	"github.com/HMB-research/open-accounting/internal/payroll"
	"github.com/HMB-research/open-accounting/internal/pdf"
	"github.com/HMB-research/open-accounting/internal/plugin"
	"github.com/HMB-research/open-accounting/internal/quotes"
	"github.com/HMB-research/open-accounting/internal/recurring"
	"github.com/HMB-research/open-accounting/internal/reports"
	"github.com/HMB-research/open-accounting/internal/scheduler"
	"github.com/HMB-research/open-accounting/internal/smartaccountsexecutor"
	"github.com/HMB-research/open-accounting/internal/smartaccountsreconciliation"
	"github.com/HMB-research/open-accounting/internal/smartaccountsreferences"
	"github.com/HMB-research/open-accounting/internal/smartaccountssync"
	"github.com/HMB-research/open-accounting/internal/tax"
	"github.com/HMB-research/open-accounting/internal/tenant"
	"github.com/HMB-research/open-accounting/internal/webhooks"
)

// Config holds the application configuration
type Config struct {
	Port                              string
	DatabaseURL                       string
	JWTSecret                         string
	AccessExpiry                      time.Duration
	RefreshExpiry                     time.Duration
	AllowedOrigins                    []string
	DocumentsDir                      string
	PasswordReset                     PasswordResetConfig
	SmartAccountsBridge               SmartAccountsBridgeConfig
	SmartAccountsPackageDeliveryToken string
}

// PasswordResetConfig controls optional password reset token email delivery.
type PasswordResetConfig struct {
	BaseURL     string
	ExposeToken bool
	SMTPConfig  *email.SMTPConfig
}

// SmartAccountsBridgeConfig contains only server-side private bridge settings.
// Its token is an HMAC secret used to mint short-lived tenant-scoped bridge
// tokens; it is never sent to a browser or returned by any API endpoint.
type SmartAccountsBridgeConfig struct {
	URL   string
	Token string
}

func (c SmartAccountsBridgeConfig) configured() bool {
	return strings.TrimSpace(c.URL) != "" && strings.TrimSpace(c.Token) != ""
}

func (c SmartAccountsBridgeConfig) hasAnyValue() bool {
	return strings.TrimSpace(c.URL) != "" || strings.TrimSpace(c.Token) != ""
}

var defaultDevelopmentOrigins = []string{"http://localhost:5173", "http://localhost:3000"}

const developmentJWTSecret = "development-only-insecure-jwt-secret" //nolint:gosec // Explicitly development-only fallback rejected in production mode.

type apiPool interface {
	Ping(ctx context.Context) error
	Close()
}

type apiScheduler interface {
	Start() error
	Stop() context.Context
}

type apiHTTPServer interface {
	ListenAndServe() error
	Shutdown(ctx context.Context) error
}

type apiPluginLoader interface {
	LoadEnabledPlugins(ctx context.Context) error
}

type apiApplication struct {
	handlers      *Handlers
	tokenService  *auth.TokenService
	scheduler     apiScheduler
	pluginLoader  apiPluginLoader
	schedulerConf scheduler.Config
}

type apiMainDeps struct {
	getenv           func(string) string
	loadConfig       func() *Config
	newPool          func(context.Context, string) (apiPool, error)
	buildApplication func(context.Context, *Config, apiPool, scheduler.Config) (*apiApplication, error)
	newServer        func(*Config, *apiApplication) apiHTTPServer
	signalNotify     func(chan<- os.Signal, ...os.Signal)
}

var (
	newDemoStatusReader = demo.NewStatusReader
	newDemoResetService = demo.NewResetService
	mainDepsProvider    = defaultAPIMainDeps
	apiMainExit         = os.Exit
	configFatalExit     = os.Exit
)

// healthCheck returns the API health status.
// @Summary Health check
// @Description Return OK when the API process is accepting requests
// @Tags System
// @Produce plain
// @Success 200 {string} string "OK"
// @Router /health [get]
func healthCheck(w http.ResponseWriter, _ *http.Request) {
	_, _ = w.Write([]byte("OK"))
}

func main() {
	if err := runAPI(context.Background(), mainDepsProvider()); err != nil {
		log.WithLevel(zerolog.FatalLevel).Err(err).Msg("API failed")
		apiMainExit(1)
	}
}

func configFatal(err error, message string) {
	log.WithLevel(zerolog.FatalLevel).Err(err).Msg(message)
	configFatalExit(1)
}

func defaultAPIMainDeps() apiMainDeps {
	return apiMainDeps{
		getenv:     os.Getenv,
		loadConfig: loadConfig,
		newPool: func(ctx context.Context, databaseURL string) (apiPool, error) {
			return pgxpool.New(ctx, databaseURL)
		},
		buildApplication: buildProductionAPIApplication,
		newServer: func(cfg *Config, app *apiApplication) apiHTTPServer {
			return &http.Server{
				Addr:         ":" + cfg.Port,
				Handler:      setupRouter(cfg, app.handlers, app.tokenService),
				ReadTimeout:  15 * time.Second,
				WriteTimeout: 15 * time.Second,
				IdleTimeout:  60 * time.Second,
			}
		},
		signalNotify: signal.Notify,
	}
}

func runAPI(ctx context.Context, deps apiMainDeps) error {
	// Configure logging
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339})

	// Set log level from environment (default: info)
	// Valid levels: trace, debug, info, warn, error, fatal, panic
	logLevel := deps.getenv("LOG_LEVEL")
	if logLevel == "" {
		logLevel = "info"
	}
	level, err := zerolog.ParseLevel(logLevel)
	if err != nil {
		log.Warn().Str("level", logLevel).Msg("Invalid LOG_LEVEL, defaulting to info")
		level = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(level)
	log.Info().Str("level", level.String()).Msg("Log level configured")

	// Load configuration
	cfg := deps.loadConfig()

	// Connect to database
	pool, err := deps.newPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}
	log.Info().Msg("Connected to database")

	schedulerConfig := loadSchedulerConfig(deps.getenv)
	app, err := deps.buildApplication(ctx, cfg, pool, schedulerConfig)
	if err != nil {
		return err
	}

	// Load enabled plugins on startup
	if err := app.pluginLoader.LoadEnabledPlugins(ctx); err != nil {
		log.Warn().Err(err).Msg("Failed to load some plugins")
	}

	if err := app.scheduler.Start(); err != nil {
		log.Warn().Err(err).Msg("Failed to start scheduler")
	}

	srv := deps.newServer(cfg, app)
	startShutdownListener(deps.signalNotify, app.scheduler, srv)

	log.Info().Str("port", cfg.Port).Msg("Starting server")
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		return fmt.Errorf("server failed: %w", err)
	}
	return nil
}

func buildProductionAPIApplication(ctx context.Context, cfg *Config, pool apiPool, schedulerConfig scheduler.Config) (*apiApplication, error) {
	pgxPool, ok := pool.(*pgxpool.Pool)
	if !ok {
		return nil, fmt.Errorf("production API requires a pgx pool")
	}

	documentStore, err := documents.NewLocalStore(cfg.DocumentsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize document storage: %w", err)
	}
	demoStatusReader, err := newDemoStatusReader(pgxPool)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize demo status reader: %w", err)
	}
	demoResetService, err := newDemoResetService(ctx, pgxPool, demo.SeedSQLForUsers)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize demo reset service: %w", err)
	}

	tokenService := auth.NewTokenService(cfg.JWTSecret, cfg.AccessExpiry, cfg.RefreshExpiry)
	refreshSessionService := auth.NewRefreshSessionService(pgxPool)
	passwordResetService := auth.NewPasswordResetService(pgxPool)
	securityAuditService := auth.NewSecurityAuditService(pgxPool)
	apiTokenService := apitoken.NewService(pgxPool)
	tokenService.SetAPITokenValidator(apiTokenService)
	tenantService := tenant.NewService(pgxPool)
	accountingService := accounting.NewService(pgxPool)
	contactsService := contacts.NewService(pgxPool)
	documentsService := documents.NewService(documents.NewRepository(pgxPool), documentStore)
	invoicingService := invoicing.NewService(pgxPool, accountingService)
	paymentsService := payments.NewService(pgxPool, invoicingService)
	pdfService := pdf.NewService()
	analyticsService := analytics.NewService(pgxPool)
	emailService := email.NewService(pgxPool)
	recurringService := recurring.NewService(pgxPool, invoicingService, emailService, pdfService, tenantService, contactsService)
	bankingService := banking.NewService(pgxPool)
	taxService := tax.NewService(pgxPool)
	payrollService := payroll.NewService(pgxPool)
	absenceService := payroll.NewAbsenceServiceWithPoolAndEvidence(pgxPool, documentsService)
	pluginService := plugin.NewService(pgxPool, "./plugins")
	quotesService := quotes.NewService(pgxPool)
	ordersService := orders.NewService(pgxPool)
	assetsService := assets.NewService(pgxPool)
	reportsService := reports.NewService(pgxPool)
	inventoryService := inventory.NewService(pgxPool)
	reminderService := invoicing.NewReminderService(pgxPool, emailService)
	automatedReminderService := invoicing.NewAutomatedReminderService(pgxPool, emailService)
	costCenterService := accounting.NewCostCenterService(pgxPool)
	interestService := invoicing.NewInterestService(pgxPool)
	webhookService := webhooks.NewService(pgxPool)
	webhookService.RegisterPluginHooks(pluginService.GetHookRegistry())
	expensesService := expenses.NewService(pgxPool, documentsService)
	migrationRunStore := cutover.NewMigrationExecutionRunRepository(pgxPool)
	importSessionRepository := importsession.NewRepository(pgxPool)
	importSessionService := importsession.NewService(importSessionRepository)
	importSessionService.SetAccountResolver(importsession.AccountResolverFunc(func(ctx context.Context, schemaName, tenantID, accountID string) error {
		_, err := accountingService.GetAccount(ctx, schemaName, tenantID, accountID)
		return err
	}))
	bridgeCatalog := smartaccountssync.BridgeCatalog(smartaccountssync.UnavailableBridgeCatalog{})
	bridgeClient := smartaccountssync.BridgeClient(smartaccountssync.UnavailableBridgeClient{})
	browserCaptureBridge := smartaccountssync.BrowserCaptureBridge(smartaccountssync.UnavailableBridgeClient{})
	browserMasterDetailBridge := smartaccountssync.BrowserMasterDetailBridge(smartaccountssync.UnavailableBridgeClient{})
	browserCommercialDetailBridge := smartaccountssync.BrowserCommercialDetailBridge(smartaccountssync.UnavailableBridgeClient{})
	browserDiscoveryBridge := smartaccountssync.BrowserDiscoveryBridge(smartaccountssync.UnavailableBridgeClient{})
	browserCSVSchemaApprovalBridge := smartaccountssync.BrowserCSVSchemaApprovalBridge(smartaccountssync.UnavailableBridgeClient{})
	if cfg.SmartAccountsBridge.hasAnyValue() {
		if !cfg.SmartAccountsBridge.configured() {
			return nil, errors.New("SmartAccounts bridge configuration is incomplete")
		}
		configuredBridgeClient, err := smartaccountssync.NewHTTPBridgeClient(cfg.SmartAccountsBridge.URL, cfg.SmartAccountsBridge.Token)
		if err != nil {
			return nil, fmt.Errorf("configure SmartAccounts bridge client: %w", err)
		}
		bridgeCatalog = smartaccountssync.ConfiguredBridgeCatalog{}
		bridgeClient = configuredBridgeClient
		browserCaptureBridge = configuredBridgeClient
		browserMasterDetailBridge = configuredBridgeClient
		browserCommercialDetailBridge = configuredBridgeClient
		browserDiscoveryBridge = configuredBridgeClient
		browserCSVSchemaApprovalBridge = configuredBridgeClient
	}
	// This private adapter can only connect and validate source credentials. It
	// never captures source records or applies accounting data.
	smartAccountsSyncRepository := smartaccountssync.NewRepository(pgxPool)
	smartAccountsSyncService := smartaccountssync.NewService(smartAccountsSyncRepository, bridgeCatalog)
	smartAccountsBrowserPairingService := smartaccountssync.NewBrowserPairingService(smartAccountsSyncRepository, smartAccountsSyncService)
	smartAccountsBrowserOnboardingService := smartaccountssync.NewBrowserOnboardingService(smartAccountsSyncRepository, tenantService, smartAccountsBrowserPairingService)
	smartAccountsBrowserOnboardingCatalogService := smartaccountssync.NewBrowserOnboardingCatalogService(smartAccountsSyncRepository)
	smartAccountsBrowserOnboardingBatchService := smartaccountssync.NewBrowserOnboardingBatchService(smartAccountsSyncRepository, smartAccountsBrowserOnboardingCatalogService, smartAccountsBrowserOnboardingService)
	smartAccountsBrowserDiscoveryService := smartaccountssync.NewBrowserDiscoveryService(smartAccountsSyncRepository, smartAccountsSyncRepository, browserDiscoveryBridge)
	smartAccountsBrowserCSVSchemaApprovalService := smartaccountssync.NewBrowserCSVSchemaApprovalService(smartAccountsSyncRepository, smartAccountsSyncRepository, browserCSVSchemaApprovalBridge)
	smartAccountsBrowserCaptureService := smartaccountssync.NewBrowserCaptureService(smartAccountsSyncRepository, smartAccountsSyncRepository, browserCaptureBridge)
	smartAccountsBrowserMasterDetailService := smartaccountssync.NewBrowserMasterDetailService(smartAccountsSyncRepository, smartAccountsSyncRepository, browserMasterDetailBridge)
	smartAccountsBrowserCommercialDetailService := smartaccountssync.NewBrowserCommercialDetailService(smartAccountsSyncRepository, smartAccountsSyncRepository, browserCommercialDetailBridge)
	smartAccountsBrowserCaptureWorkflowService := smartaccountssync.NewBrowserCaptureWorkflowService(smartAccountsSyncRepository, smartAccountsSyncRepository, smartAccountsBrowserCaptureService)
	smartAccountsBrowserBatchWorkflowService := smartaccountssync.NewBrowserBatchWorkflowService(smartAccountsSyncRepository, smartAccountsSyncRepository)
	importDeliveryRepository := importdelivery.NewRepository(pgxPool)
	importDeliveryService := importdelivery.NewService(importDeliveryRepository, importdelivery.NewControlledSourceBinder(smartAccountsSyncRepository, importSessionRepository))
	// A browser master-detail package becomes staging-review-ready only when
	// OA's own tenant archive proves the exact source/package/digest tuple.
	// The private bridge's sealed-package response alone never enables preview.
	smartAccountsBrowserMasterDetailService.SetStagedPackageVerifier(smartaccountssync.BrowserMasterDetailStagedPackageVerifierFunc(func(ctx context.Context, tenantID, sourceCompanyID, packageID, packageSHA256 string) error {
		currentTenant, err := tenantService.GetTenant(ctx, tenantID)
		if err != nil {
			return err
		}
		return importDeliveryService.VerifyStagedPackage(ctx, currentTenant.SchemaName, tenantID, sourceCompanyID, packageID, packageSHA256)
	}))
	smartAccountsExecutorRepository := smartaccountsexecutor.NewRepository(pgxPool)
	smartAccountsCaptureCoverageRepository := smartaccountsexecutor.NewCaptureCoverageRepository(pgxPool)
	smartAccountsExecutorPlanner := smartaccountsexecutor.NewPlanner(
		importDeliveryRepository,
		smartAccountsExecutorRepository,
		smartaccountsexecutor.AccountResolverFunc(func(ctx context.Context, schemaName, tenantID, accountID string) error {
			_, err := accountingService.GetAccount(ctx, schemaName, tenantID, accountID)
			return err
		}),
	).SetCaptureCoverageReader(smartAccountsCaptureCoverageRepository).SetAccountCatalog(smartaccountsexecutor.AccountCatalogFunc(func(ctx context.Context, schemaName, tenantID string) ([]smartaccountsexecutor.ChartAccount, error) {
		accounts, err := accountingService.ListAccounts(ctx, schemaName, tenantID, true)
		if err != nil {
			return nil, err
		}
		result := make([]smartaccountsexecutor.ChartAccount, 0, len(accounts))
		for _, account := range accounts {
			result = append(result, smartaccountsexecutor.ChartAccount{ID: account.ID, Code: account.Code, Name: account.Name})
		}
		return result, nil
	}))
	smartAccountsExecutor := smartaccountsexecutor.NewService(smartAccountsExecutorPlanner, smartAccountsExecutorRepository, accountingService)
	smartAccountsReconciliationRepository := smartaccountsreconciliation.NewRepository(pgxPool)
	smartAccountsExecutor.SetApplyReceiptRecorder(smartAccountsReconciliationRepository)
	smartAccountsExecutor.SetTolerancePolicyVerifier(smartAccountsReconciliationRepository)
	smartAccountsBrowserBatchWorkflowActions := smartaccountssync.NewBrowserBatchWorkflowActionsService(
		smartAccountsBrowserBatchWorkflowService,
		smartAccountsBrowserDiscoveryService,
		smartAccountsBrowserCSVSchemaApprovalService,
		smartAccountsBrowserCaptureService,
		smartAccountsBrowserBatchPreviewAdapter{executor: smartAccountsExecutor, tenants: tenantService},
	)
	// Reference planners read only through the delivery service, which verifies
	// tenant/source/package staging before iterating any retained record.
	smartAccountsReferenceService := smartaccountsreferences.NewService(importDeliveryService, smartaccountsreferences.NewRepository(pgxPool), smartAccountsReferenceWriter{accounts: accountingService, contacts: contactsService, inventory: inventoryService}, smartAccountsReferenceCatalog{accounts: accountingService, contacts: contactsService, inventory: inventoryService})
	// Reconciliation reads the protected archive and exact posted target IDs
	// only inside the API process, compares all amounts in memory, and returns
	// digest-only technical proof. None of these readers is browser-facing.
	smartAccountsReconciliationProofs := smartaccountsreconciliation.NewZeroFileStreamingProofComputer(importDeliveryService, smartAccountsReconciliationRepository, smartAccountsReconciliationRepository, accountingService)
	smartAccountsReconciliationResolver := smartAccountsReconciliationResolver{batches: smartAccountsBrowserOnboardingBatchService, workflows: smartAccountsBrowserBatchWorkflowActions, delivery: importDeliveryService, executor: smartAccountsExecutor, references: smartAccountsReferenceService, receipts: smartAccountsReconciliationRepository, tenants: tenantService, proofs: smartAccountsReconciliationProofs}
	smartAccountsReconciliationService := smartaccountsreconciliation.NewService(smartAccountsReconciliationRepository, smartAccountsReconciliationResolver)
	smartAccountsTolerancePolicyService := smartaccountsreconciliation.NewTolerancePolicyService(smartAccountsReconciliationRepository, smartAccountsReconciliationResolver)
	var importDeliveryAuthenticator importdelivery.Authenticator
	if strings.TrimSpace(cfg.SmartAccountsPackageDeliveryToken) != "" {
		authenticator, err := importdelivery.NewHMACAuthenticator(cfg.SmartAccountsPackageDeliveryToken, importDeliveryRepository)
		if err != nil {
			return nil, fmt.Errorf("configure SmartAccounts package delivery authentication: %w", err)
		}
		importDeliveryAuthenticator = authenticator
	}
	documentRetentionReminderPolicy := loadDocumentRetentionReminderPolicy()
	documentRetentionReminderService := documents.NewRetentionReminderServiceWithPolicy(documentsService, emailService, documentRetentionReminderPolicy)
	appScheduler := scheduler.NewScheduler(pgxPool, recurringService, automatedReminderService, schedulerConfig)
	appScheduler.SetRecurringJournalEntryService(accountingService)
	appScheduler.SetDocumentRetentionReminderService(documentRetentionReminderService)

	// Create handlers
	handlers := &Handlers{
		tokenService:                          tokenService,
		refreshSessionService:                 refreshSessionService,
		passwordResetService:                  passwordResetService,
		passwordResetExposeToken:              cfg.PasswordReset.ExposeToken,
		passwordResetBaseURL:                  cfg.PasswordReset.BaseURL,
		passwordResetSMTPConfig:               cfg.PasswordReset.SMTPConfig,
		passwordResetMailer:                   &email.DefaultMailSender{},
		loginAttemptLimiter:                   auth.DefaultLoginAttemptLimiter(),
		securityAuditService:                  securityAuditService,
		apiTokenService:                       apiTokenService,
		tenantService:                         tenantService,
		accountingService:                     accountingService,
		contactsService:                       contactsService,
		documentsService:                      documentsService,
		invoicingService:                      invoicingService,
		paymentsService:                       paymentsService,
		pdfService:                            pdfService,
		analyticsService:                      analyticsService,
		recurringService:                      recurringService,
		emailService:                          emailService,
		bankingService:                        bankingService,
		taxService:                            taxService,
		payrollService:                        payrollService,
		absenceService:                        absenceService,
		pluginService:                         pluginService,
		quotesService:                         quotesService,
		ordersService:                         ordersService,
		assetsService:                         assetsService,
		inventoryService:                      inventoryService,
		reportsService:                        reportsService,
		reminderService:                       reminderService,
		automatedReminderService:              automatedReminderService,
		costCenterService:                     costCenterService,
		interestService:                       interestService,
		webhookService:                        webhookService,
		expensesService:                       expensesService,
		migrationRunStore:                     migrationRunStore,
		importSessionService:                  importSessionService,
		importDeliveryService:                 importDeliveryService,
		importDeliveryAuthenticator:           importDeliveryAuthenticator,
		smartAccountsSyncService:              smartAccountsSyncService,
		smartAccountsBrowserPairingService:    smartAccountsBrowserPairingService,
		smartAccountsBrowserOnboardingService: smartAccountsBrowserOnboardingService,
		smartAccountsBrowserOnboardingCatalogService: smartAccountsBrowserOnboardingCatalogService,
		smartAccountsBrowserOnboardingBatchService:   smartAccountsBrowserOnboardingBatchService,
		smartAccountsBrowserDiscoveryService:         smartAccountsBrowserDiscoveryService,
		smartAccountsBrowserCSVSchemaApprovalService: smartAccountsBrowserCSVSchemaApprovalService,
		smartAccountsBrowserCaptureService:           smartAccountsBrowserCaptureService,
		smartAccountsBrowserMasterDetailService:      smartAccountsBrowserMasterDetailService,
		smartAccountsBrowserCommercialDetailService:  smartAccountsBrowserCommercialDetailService,
		smartAccountsBrowserCaptureWorkflowService:   smartAccountsBrowserCaptureWorkflowService,
		smartAccountsBrowserBatchWorkflowActions:     smartAccountsBrowserBatchWorkflowActions,
		smartAccountsBridgeClient:                    bridgeClient,
		smartAccountsExecutor:                        smartAccountsExecutor,
		smartAccountsReferenceService:                smartAccountsReferenceService,
		smartAccountsReconciliationService:           smartAccountsReconciliationService,
		smartAccountsTolerancePolicyService:          smartAccountsTolerancePolicyService,
		readinessDatabase:                            pool,
		demoResetService:                             demoResetService,
		demoStatusReader:                             demoStatusReader,
	}

	return &apiApplication{
		handlers:      handlers,
		tokenService:  tokenService,
		scheduler:     appScheduler,
		pluginLoader:  pluginService,
		schedulerConf: schedulerConfig,
	}, nil
}

func loadSchedulerConfig(getenv func(string) string) scheduler.Config {
	schedulerConfig := scheduler.DefaultConfig()
	if schedule := getenv("RECURRING_INVOICE_SCHEDULE"); schedule != "" {
		schedulerConfig.RecurringInvoiceSchedule = schedule
	}
	if schedule := getenv("RECURRING_JOURNAL_ENTRY_SCHEDULE"); schedule != "" {
		schedulerConfig.RecurringJournalEntrySchedule = schedule
	}
	if schedule := getenv("DOCUMENT_RETENTION_REMINDER_SCHEDULE"); schedule != "" {
		schedulerConfig.DocumentRetentionReminderSchedule = schedule
	}
	if horizon := getenv("DOCUMENT_RETENTION_REMINDER_HORIZON_DAYS"); horizon != "" {
		parsed, err := strconv.Atoi(horizon)
		if err != nil || parsed < 0 {
			log.Warn().Str("horizon_days", horizon).Msg("Invalid DOCUMENT_RETENTION_REMINDER_HORIZON_DAYS, using default")
		} else {
			schedulerConfig.DocumentRetentionReminderHorizonDays = parsed
		}
	}
	if includeMissing := getenv("DOCUMENT_RETENTION_REMINDER_INCLUDE_MISSING"); includeMissing != "" {
		parsed, err := strconv.ParseBool(includeMissing)
		if err != nil {
			log.Warn().Str("include_missing", includeMissing).Msg("Invalid DOCUMENT_RETENTION_REMINDER_INCLUDE_MISSING, using default")
		} else {
			schedulerConfig.DocumentRetentionReminderIncludeMissing = parsed
		}
	}
	if getenv("SCHEDULER_ENABLED") == "false" {
		schedulerConfig.Enabled = false
	}
	return schedulerConfig
}

func startShutdownListener(signalNotify func(chan<- os.Signal, ...os.Signal), appScheduler apiScheduler, srv apiHTTPServer) {
	go func() {
		sigChan := make(chan os.Signal, 1)
		signalNotify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan

		log.Info().Msg("Shutting down server...")

		// Stop the scheduler first
		schedulerCtx := appScheduler.Stop()
		<-schedulerCtx.Done()

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := srv.Shutdown(ctx); err != nil {
			log.Error().Err(err).Msg("Server shutdown error")
		}
	}()
}

func loadConfig() *Config {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		configFatal(nil, "DATABASE_URL environment variable required")
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	production := isProductionEnvironment()
	resolvedJWTSecret, err := resolveJWTSecret(jwtSecret, production)
	if err != nil {
		configFatal(err, "Invalid JWT_SECRET configuration")
	}
	if strings.TrimSpace(jwtSecret) == "" {
		log.Warn().Msg("Using development-only JWT_SECRET; set JWT_SECRET for shared or production deployments")
	}

	// ALLOWED_ORIGINS supports comma-separated list of origins
	// Example: "https://app.example.com,https://admin.example.com"
	origins := os.Getenv("ALLOWED_ORIGINS")
	allowedOrigins, err := resolveAllowedOrigins(origins, production)
	if err != nil {
		configFatal(err, "Invalid ALLOWED_ORIGINS configuration")
	}
	log.Info().Strs("allowed_origins", allowedOrigins).Msg("CORS configuration")

	documentsDir := os.Getenv("DOCUMENTS_DIR")
	if documentsDir == "" {
		documentsDir = "./data/documents"
	}
	bridgeToken, err := resolveSmartAccountsBridgeToken(os.Getenv("SMARTACCOUNTS_BRIDGE_TOKEN"), os.Getenv("SMARTACCOUNTS_BRIDGE_TOKEN_FILE"))
	if err != nil {
		configFatal(err, "Invalid SMARTACCOUNTS bridge token configuration")
	}
	packageDeliveryToken, err := resolveSecretFile("SMARTACCOUNTS_PACKAGE_DELIVERY_TOKEN", os.Getenv("SMARTACCOUNTS_PACKAGE_DELIVERY_TOKEN"), os.Getenv("SMARTACCOUNTS_PACKAGE_DELIVERY_TOKEN_FILE"))
	if err != nil {
		configFatal(err, "Invalid SMARTACCOUNTS package delivery token configuration")
	}

	return &Config{
		Port:           port,
		DatabaseURL:    dbURL,
		JWTSecret:      resolvedJWTSecret,
		AccessExpiry:   15 * time.Minute,
		RefreshExpiry:  7 * 24 * time.Hour,
		AllowedOrigins: allowedOrigins,
		DocumentsDir:   documentsDir,
		PasswordReset: PasswordResetConfig{
			BaseURL:     strings.TrimSpace(os.Getenv("PASSWORD_RESET_BASE_URL")),
			ExposeToken: os.Getenv("PASSWORD_RESET_EXPOSE_TOKEN") == "true",
			SMTPConfig:  loadPasswordResetSMTPConfig(),
		},
		SmartAccountsBridge: SmartAccountsBridgeConfig{
			URL:   strings.TrimSpace(os.Getenv("SMARTACCOUNTS_BRIDGE_URL")),
			Token: bridgeToken,
		},
		SmartAccountsPackageDeliveryToken: packageDeliveryToken,
	}
}

// resolveSmartAccountsBridgeToken prefers a Docker-secret file over the
// legacy inline environment value. The value is only held in server memory to
// mint short-lived bridge HMAC tokens and must never be logged or returned.
func resolveSmartAccountsBridgeToken(inlineValue, filePath string) (string, error) {
	return resolveSecretFile("SMARTACCOUNTS_BRIDGE_TOKEN", inlineValue, filePath)
}

// resolveSecretFile prefers an owner-readable Docker secret file over a
// development-only inline value. It never includes secret contents in errors.
func resolveSecretFile(name, inlineValue, filePath string) (string, error) {
	filePath = strings.TrimSpace(filePath)
	if filePath == "" {
		return strings.TrimSpace(inlineValue), nil
	}
	contents, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("read %s_FILE: %w", name, err)
	}
	value := strings.TrimSpace(string(contents))
	if value == "" {
		return "", fmt.Errorf("%s_FILE is empty", name)
	}
	return value, nil
}

func loadPasswordResetSMTPConfig() *email.SMTPConfig {
	host := strings.TrimSpace(os.Getenv("PASSWORD_RESET_SMTP_HOST"))
	fromEmail := strings.TrimSpace(os.Getenv("PASSWORD_RESET_SMTP_FROM_EMAIL"))
	if host == "" || fromEmail == "" {
		return nil
	}

	port := 587
	if rawPort := strings.TrimSpace(os.Getenv("PASSWORD_RESET_SMTP_PORT")); rawPort != "" {
		parsedPort, err := strconv.Atoi(rawPort)
		if err != nil || parsedPort <= 0 || parsedPort > 65535 {
			log.Warn().Str("port", rawPort).Msg("Invalid PASSWORD_RESET_SMTP_PORT, using 587")
		} else {
			port = parsedPort
		}
	}

	return &email.SMTPConfig{
		Host:      host,
		Port:      port,
		Username:  strings.TrimSpace(os.Getenv("PASSWORD_RESET_SMTP_USERNAME")),
		Password:  os.Getenv("PASSWORD_RESET_SMTP_PASSWORD"),
		FromEmail: fromEmail,
		FromName:  strings.TrimSpace(os.Getenv("PASSWORD_RESET_SMTP_FROM_NAME")),
		UseTLS:    strings.EqualFold(strings.TrimSpace(os.Getenv("PASSWORD_RESET_SMTP_USE_TLS")), "true"),
	}
}

func loadDocumentRetentionReminderPolicy() documents.RetentionReminderPolicy {
	policy := documents.RetentionReminderPolicy{}
	if maxAttempts := strings.TrimSpace(os.Getenv("DOCUMENT_RETENTION_REMINDER_MAX_ATTEMPTS")); maxAttempts != "" {
		parsed, err := strconv.Atoi(maxAttempts)
		if err != nil || parsed < 1 {
			log.Warn().Str("max_attempts", maxAttempts).Msg("Invalid DOCUMENT_RETENTION_REMINDER_MAX_ATTEMPTS, using default")
		} else {
			policy.MaxAttempts = parsed
		}
	}
	if escalateAttempts := strings.TrimSpace(os.Getenv("DOCUMENT_RETENTION_REMINDER_ESCALATE_AFTER_ATTEMPTS")); escalateAttempts != "" {
		parsed, err := strconv.Atoi(escalateAttempts)
		if err != nil || parsed < 1 {
			log.Warn().Str("escalate_after_attempts", escalateAttempts).Msg("Invalid DOCUMENT_RETENTION_REMINDER_ESCALATE_AFTER_ATTEMPTS, using default")
		} else {
			policy.EscalateAfterAttempts = parsed
		}
	}
	return policy
}

func isProductionEnvironment() bool {
	for _, key := range []string{"APP_ENV", "ENV", "GO_ENV"} {
		if strings.EqualFold(strings.TrimSpace(os.Getenv(key)), "production") {
			return true
		}
	}
	return strings.EqualFold(strings.TrimSpace(os.Getenv("RAILWAY_ENVIRONMENT")), "production")
}

func resolveJWTSecret(raw string, production bool) (string, error) {
	secret := strings.TrimSpace(raw)
	if secret == "" {
		if production {
			return "", errors.New("JWT_SECRET is required in production")
		}
		return developmentJWTSecret, nil
	}
	if production && len(secret) < 32 {
		return "", errors.New("JWT_SECRET must be at least 32 characters in production")
	}
	return secret, nil
}

func resolveAllowedOrigins(raw string, production bool) ([]string, error) {
	var origins []string
	for _, origin := range strings.Split(raw, ",") {
		origin = strings.TrimSpace(origin)
		if origin != "" {
			origins = append(origins, origin)
		}
	}
	if len(origins) == 0 {
		if production {
			return nil, errors.New("ALLOWED_ORIGINS is required in production")
		}
		return append([]string(nil), defaultDevelopmentOrigins...), nil
	}
	if production {
		return origins, nil
	}

	merged := append([]string(nil), defaultDevelopmentOrigins...)
	merged = append(merged, origins...)
	return merged, nil
}

func setupRouter(cfg *Config, h *Handlers, tokenService *auth.TokenService) *chi.Mux {
	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.RequestID)
	// Use the socket peer by default. Trusting forwarding headers without an
	// explicit trusted-proxy boundary allows callers to spoof their source IP.
	r.Use(middleware.ClientIPFromRemoteAddr)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))

	// Security headers
	r.Use(secmiddleware.SecurityHeaders)

	// CORS - Configure allowed origins via ALLOWED_ORIGINS env var
	// If you see CORS errors, ensure your frontend origin is in ALLOWED_ORIGINS
	// Example: ALLOWED_ORIGINS="https://app.example.com,https://admin.example.com"
	corsDebug := os.Getenv("CORS_DEBUG") == "true"
	r.Use(cors.Handler(cors.Options{
		// Browser relay needs CORS preflight from a locally installed Brave
		// extension. Keep this exception bound to exact no-cookie capability
		// routes; every other endpoint retains configured web origins.
		AllowOriginFunc: func(request *http.Request, origin string) bool {
			return configuredCORSOriginAllowed(origin, cfg.AllowedOrigins) ||
				(isSmartAccountsBrowserExtensionRelayPath(request.URL.Path) && braveExtensionOriginPattern.MatchString(strings.TrimSpace(origin)))
		},
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Tenant-ID", "X-SA-Browser-Resource-SHA256", "X-SA-Browser-Commercial-SHA256"},
		ExposedHeaders:   []string{"Link", "X-RateLimit-Limit", "X-RateLimit-Remaining", "X-RateLimit-Reset", "Retry-After"},
		AllowCredentials: true,
		MaxAge:           300,
		Debug:            corsDebug,
	}))

	// Rate limiting - disabled in demo mode for E2E testing, otherwise 100 requests/minute with burst 10
	if os.Getenv("DEMO_MODE") != "true" {
		rateLimiter := auth.DefaultRateLimiter()
		r.Use(rateLimiter.Middleware)
	}

	registerPublicRoutes(r, h)

	r.Route("/api/v1", func(r chi.Router) {
		registerAPIPublicRoutes(r, h)
		registerInternalBridgeRoutes(r, h)
		registerAuthenticatedRoutes(r, h, tokenService)
	})

	return r
}

func isSmartAccountsBrowserExtensionRelayPath(path string) bool {
	const pairingPrefix = "/api/v1/smartaccounts-browser-pairings/"
	if strings.HasPrefix(path, pairingPrefix) && strings.HasSuffix(path, "/claim") && len(strings.TrimSuffix(strings.TrimPrefix(path, pairingPrefix), "/claim")) == 36 {
		return true
	}
	const catalogPrefix = "/api/v1/smartaccounts-browser-onboarding/catalogs/"
	if strings.HasPrefix(path, catalogPrefix) && strings.HasSuffix(path, "/handoff") && len(strings.TrimSuffix(strings.TrimPrefix(path, catalogPrefix), "/handoff")) == 36 {
		return true
	}
	// These are extension-only bearer relay paths. They never use a cookie or
	// accept an ordinary web origin; handler-level scope checks still bind every
	// tenant/run request to the short-lived capability.
	for _, prefix := range []string{
		"/api/v1/smartaccounts-browser-captures/tenants/",
		"/api/v1/smartaccounts-browser-master-detail-captures/tenants/",
		"/api/v1/smartaccounts-browser-commercial-captures/tenants/",
	} {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

// configuredCORSOriginAllowed preserves the existing exact and single-star
// origin matching when the pairing-only origin callback is installed.
func configuredCORSOriginAllowed(origin string, configured []string) bool {
	origin = strings.TrimSpace(origin)
	for _, candidate := range configured {
		candidate = strings.TrimSpace(candidate)
		if candidate == "*" || candidate == origin {
			return true
		}
		if strings.Count(candidate, "*") != 1 {
			continue
		}
		parts := strings.SplitN(candidate, "*", 2)
		if strings.HasPrefix(origin, parts[0]) && strings.HasSuffix(origin, parts[1]) && len(origin) >= len(parts[0])+len(parts[1]) {
			return true
		}
	}
	return false
}
