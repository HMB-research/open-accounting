package main

import (
	"context"
	"errors"
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
	httpSwagger "github.com/swaggo/http-swagger/v2"

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
	"github.com/HMB-research/open-accounting/internal/tax"
	"github.com/HMB-research/open-accounting/internal/tenant"
	"github.com/HMB-research/open-accounting/internal/webhooks"
)

// Config holds the application configuration
type Config struct {
	Port           string
	DatabaseURL    string
	JWTSecret      string
	AccessExpiry   time.Duration
	RefreshExpiry  time.Duration
	AllowedOrigins []string
	DocumentsDir   string
	PasswordReset  PasswordResetConfig
}

// PasswordResetConfig controls optional password reset token email delivery.
type PasswordResetConfig struct {
	BaseURL     string
	ExposeToken bool
	SMTPConfig  *email.SMTPConfig
}

var defaultDevelopmentOrigins = []string{"http://localhost:5173", "http://localhost:3000"}

const developmentJWTSecret = "development-only-insecure-jwt-secret" //nolint:gosec // Explicitly development-only fallback rejected in production mode.

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
	// Configure logging
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339})

	// Set log level from environment (default: info)
	// Valid levels: trace, debug, info, warn, error, fatal, panic
	logLevel := os.Getenv("LOG_LEVEL")
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
	cfg := loadConfig()

	// Connect to database
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to database")
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		log.Fatal().Err(err).Msg("Failed to ping database")
	}
	log.Info().Msg("Connected to database")

	// Initialize services
	tokenService := auth.NewTokenService(cfg.JWTSecret, cfg.AccessExpiry, cfg.RefreshExpiry)
	refreshSessionService := auth.NewRefreshSessionService(pool)
	passwordResetService := auth.NewPasswordResetService(pool)
	securityAuditService := auth.NewSecurityAuditService(pool)
	apiTokenService := apitoken.NewService(pool)
	tokenService.SetAPITokenValidator(apiTokenService)
	tenantService := tenant.NewService(pool)
	accountingService := accounting.NewService(pool)
	contactsService := contacts.NewService(pool)
	documentStore, err := documents.NewLocalStore(cfg.DocumentsDir)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize document storage")
	}
	documentsService := documents.NewService(documents.NewRepository(pool), documentStore)
	invoicingService := invoicing.NewService(pool, accountingService)
	paymentsService := payments.NewService(pool, invoicingService)
	pdfService := pdf.NewService()
	analyticsService := analytics.NewService(pool)
	emailService := email.NewService(pool)
	recurringService := recurring.NewService(pool, invoicingService, emailService, pdfService, tenantService, contactsService)
	bankingService := banking.NewService(pool)
	taxService := tax.NewService(pool)
	payrollService := payroll.NewService(pool)
	absenceService := payroll.NewAbsenceServiceWithPoolAndEvidence(pool, documentsService)
	pluginService := plugin.NewService(pool, "./plugins")
	quotesService := quotes.NewService(pool)
	ordersService := orders.NewService(pool)
	assetsService := assets.NewService(pool)
	reportsService := reports.NewService(pool)
	inventoryService := inventory.NewService(pool)
	reminderService := invoicing.NewReminderService(pool, emailService)
	automatedReminderService := invoicing.NewAutomatedReminderService(pool, emailService)
	costCenterService := accounting.NewCostCenterService(pool)
	interestService := invoicing.NewInterestService(pool)
	webhookService := webhooks.NewService(pool)
	webhookService.RegisterPluginHooks(pluginService.GetHookRegistry())
	expensesService := expenses.NewService(pool, documentsService)
	migrationRunStore := cutover.NewMigrationExecutionRunRepository(pool)
	demoStatusReader, err := demo.NewStatusReader(pool)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize demo status reader")
	}
	demoResetService, err := demo.NewResetService(ctx, pool, demo.SeedSQLForUsers)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize demo reset service")
	}

	// Load enabled plugins on startup
	if err := pluginService.LoadEnabledPlugins(ctx); err != nil {
		log.Warn().Err(err).Msg("Failed to load some plugins")
	}

	// Initialize and start scheduler for recurring work
	schedulerConfig := scheduler.DefaultConfig()
	if schedule := os.Getenv("RECURRING_INVOICE_SCHEDULE"); schedule != "" {
		schedulerConfig.RecurringInvoiceSchedule = schedule
	}
	if schedule := os.Getenv("RECURRING_JOURNAL_ENTRY_SCHEDULE"); schedule != "" {
		schedulerConfig.RecurringJournalEntrySchedule = schedule
	}
	if schedule := os.Getenv("DOCUMENT_RETENTION_REMINDER_SCHEDULE"); schedule != "" {
		schedulerConfig.DocumentRetentionReminderSchedule = schedule
	}
	if horizon := os.Getenv("DOCUMENT_RETENTION_REMINDER_HORIZON_DAYS"); horizon != "" {
		parsed, err := strconv.Atoi(horizon)
		if err != nil || parsed < 0 {
			log.Warn().Str("horizon_days", horizon).Msg("Invalid DOCUMENT_RETENTION_REMINDER_HORIZON_DAYS, using default")
		} else {
			schedulerConfig.DocumentRetentionReminderHorizonDays = parsed
		}
	}
	if includeMissing := os.Getenv("DOCUMENT_RETENTION_REMINDER_INCLUDE_MISSING"); includeMissing != "" {
		parsed, err := strconv.ParseBool(includeMissing)
		if err != nil {
			log.Warn().Str("include_missing", includeMissing).Msg("Invalid DOCUMENT_RETENTION_REMINDER_INCLUDE_MISSING, using default")
		} else {
			schedulerConfig.DocumentRetentionReminderIncludeMissing = parsed
		}
	}
	if os.Getenv("SCHEDULER_ENABLED") == "false" {
		schedulerConfig.Enabled = false
	}
	documentRetentionReminderPolicy := loadDocumentRetentionReminderPolicy()
	documentRetentionReminderService := documents.NewRetentionReminderServiceWithPolicy(documentsService, emailService, documentRetentionReminderPolicy)
	appScheduler := scheduler.NewScheduler(pool, recurringService, automatedReminderService, schedulerConfig)
	appScheduler.SetRecurringJournalEntryService(accountingService)
	appScheduler.SetDocumentRetentionReminderService(documentRetentionReminderService)
	if err := appScheduler.Start(); err != nil {
		log.Warn().Err(err).Msg("Failed to start scheduler")
	}

	// Create handlers
	handlers := &Handlers{
		tokenService:             tokenService,
		refreshSessionService:    refreshSessionService,
		passwordResetService:     passwordResetService,
		passwordResetExposeToken: cfg.PasswordReset.ExposeToken,
		passwordResetBaseURL:     cfg.PasswordReset.BaseURL,
		passwordResetSMTPConfig:  cfg.PasswordReset.SMTPConfig,
		passwordResetMailer:      &email.DefaultMailSender{},
		loginAttemptLimiter:      auth.DefaultLoginAttemptLimiter(),
		securityAuditService:     securityAuditService,
		apiTokenService:          apiTokenService,
		tenantService:            tenantService,
		accountingService:        accountingService,
		contactsService:          contactsService,
		documentsService:         documentsService,
		invoicingService:         invoicingService,
		paymentsService:          paymentsService,
		pdfService:               pdfService,
		analyticsService:         analyticsService,
		recurringService:         recurringService,
		emailService:             emailService,
		bankingService:           bankingService,
		taxService:               taxService,
		payrollService:           payrollService,
		absenceService:           absenceService,
		pluginService:            pluginService,
		quotesService:            quotesService,
		ordersService:            ordersService,
		assetsService:            assetsService,
		inventoryService:         inventoryService,
		reportsService:           reportsService,
		reminderService:          reminderService,
		automatedReminderService: automatedReminderService,
		costCenterService:        costCenterService,
		interestService:          interestService,
		webhookService:           webhookService,
		expensesService:          expensesService,
		migrationRunStore:        migrationRunStore,
		demoResetService:         demoResetService,
		demoStatusReader:         demoStatusReader,
	}

	// Setup router
	r := setupRouter(cfg, handlers, tokenService)

	// Start server
	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
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

	log.Info().Str("port", cfg.Port).Msg("Starting server")
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatal().Err(err).Msg("Server failed")
	}
}

func loadConfig() *Config {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal().Msg("DATABASE_URL environment variable required")
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	production := isProductionEnvironment()
	resolvedJWTSecret, err := resolveJWTSecret(jwtSecret, production)
	if err != nil {
		log.Fatal().Err(err).Msg("Invalid JWT_SECRET configuration")
	}
	if strings.TrimSpace(jwtSecret) == "" {
		log.Warn().Msg("Using development-only JWT_SECRET; set JWT_SECRET for shared or production deployments")
	}

	// ALLOWED_ORIGINS supports comma-separated list of origins
	// Example: "https://app.example.com,https://admin.example.com"
	origins := os.Getenv("ALLOWED_ORIGINS")
	allowedOrigins, err := resolveAllowedOrigins(origins, production)
	if err != nil {
		log.Fatal().Err(err).Msg("Invalid ALLOWED_ORIGINS configuration")
	}
	log.Info().Strs("allowed_origins", allowedOrigins).Msg("CORS configuration")

	documentsDir := os.Getenv("DOCUMENTS_DIR")
	if documentsDir == "" {
		documentsDir = "./data/documents"
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
	}
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
	r.Use(middleware.RealIP)
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
		AllowedOrigins:   cfg.AllowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Tenant-ID"},
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

	// Health check
	r.Get("/health", healthCheck)

	// Demo endpoints (protected by secret key)
	r.Post("/api/demo/reset", h.DemoReset)
	r.Get("/api/demo/status", h.DemoStatus)

	// Swagger documentation
	r.Get("/swagger/*", httpSwagger.Handler(
		httpSwagger.URL("/swagger/doc.json"),
	))

	// API routes
	r.Route("/api/v1", func(r chi.Router) {
		// Public routes
		r.Post("/auth/register", h.Register)
		r.Post("/auth/login", h.Login)
		r.Post("/auth/refresh", h.RefreshToken)
		r.Post("/auth/logout", h.Logout)
		r.Post("/auth/password-reset/request", h.RequestPasswordReset)
		r.Post("/auth/password-reset/confirm", h.ResetPassword)

		// Public invitation endpoints (no auth required)
		r.Get("/invitations/{token}", h.GetInvitationByToken)
		r.Post("/invitations/accept", h.AcceptInvitation)

		// Authenticated routes
		r.Group(func(r chi.Router) {
			r.Use(tokenService.Middleware)

			// User routes
			r.Get("/me", h.GetCurrentUser)
			r.Get("/me/tenants", h.ListMyTenants)
			r.Put("/auth/password", h.ChangePassword)
			r.Get("/auth/sessions", h.ListAuthSessions)
			r.Delete("/auth/sessions", h.RevokeAllAuthSessions)
			r.Delete("/auth/sessions/{sessionID}", h.RevokeAuthSession)
			r.Get("/auth/security-events", h.ListSecurityAuditEvents)

			// Tenant management
			r.Post("/tenants", h.CreateTenant)

			// Admin routes (instance-level plugin management)
			r.Route("/admin", func(r chi.Router) {
				// Plugin Registries
				r.Get("/plugin-registries", h.ListPluginRegistries)
				r.Post("/plugin-registries", h.AddPluginRegistry)
				r.Delete("/plugin-registries/{id}", h.RemovePluginRegistry)
				r.Post("/plugin-registries/{id}/sync", h.SyncPluginRegistry)

				// Plugin Management
				r.Get("/plugins", h.ListPlugins)
				r.Get("/plugins/search", h.SearchPlugins)
				r.Get("/plugins/permissions", h.GetAllPermissions)
				r.Post("/plugins/install", h.InstallPlugin)
				r.Get("/plugins/{id}", h.GetPlugin)
				r.Delete("/plugins/{id}", h.UninstallPlugin)
				r.Post("/plugins/{id}/enable", h.EnablePlugin)
				r.Post("/plugins/{id}/disable", h.DisablePlugin)
				r.Get("/plugins/{id}/runtime", h.GetPluginRuntimeStatus)
				r.Post("/plugins/{id}/runtime/restart", h.RestartPluginRuntime)
			})

			// Tenant-scoped routes
			r.Route("/tenants/{tenantID}", func(r chi.Router) {
				r.Use(h.TenantContext)

				// Onboarding
				r.Post("/complete-onboarding", h.CompleteOnboarding)
				r.Get("/period-close-events", h.ListPeriodCloseEvents)
				r.Post("/period-close", h.ClosePeriod)
				r.Post("/period-reopen", h.ReopenPeriod)
				r.Get("/year-end-close-status", h.GetYearEndCloseStatus)
				r.Get("/year-end-close-pack", h.GetYearEndClosePack)
				r.Get("/year-end-close-audit-evidence", h.GetYearEndCloseAuditEvidence)
				r.Get("/year-end-close-audit-archive", h.DownloadYearEndCloseAuditArchive)
				r.Post("/year-end-carry-forward", h.CreateYearEndCarryForward)
				r.Post("/year-end-carry-forward/reverse", h.ReverseYearEndCarryForward)
				r.Get("/documents", h.ListDocuments)
				r.Post("/documents/review-summary", h.ListDocumentReviewSummaries)
				r.Get("/documents/review-queue", h.GetDocumentReviewQueue)
				r.Post("/documents/evidence-policy", h.EvaluateDocumentEvidencePolicy)
				r.Get("/documents/retention", h.GetDocumentRetentionReview)
				r.Post("/documents/purge", h.PurgeExpiredDocuments)
				r.Post("/documents", h.UploadDocument)
				r.Get("/documents/{documentID}/download", h.DownloadDocument)
				r.Patch("/documents/{documentID}/retention", h.UpdateDocumentRetention)
				r.Patch("/documents/{documentID}/lifecycle", h.UpdateDocumentLifecycle)
				r.Patch("/documents/{documentID}/legal-hold", h.UpdateDocumentLegalHold)
				r.Post("/documents/{documentID}/review", h.ReviewDocument)
				r.Post("/documents/{documentID}/mark-reviewed", h.MarkDocumentReviewed)
				r.Delete("/documents/{documentID}", h.DeleteDocument)
				r.Get("/api-tokens", h.ListAPITokens)
				r.Post("/api-tokens", h.CreateAPIToken)
				r.Delete("/api-tokens/{tokenID}", h.RevokeAPIToken)

				// Migration cutover preflight
				r.Get("/migration/provider-presets", h.ListMigrationProviderPresets)
				r.Post("/migration/validate", h.ValidateMigrationBundle)
				r.Post("/migration/execution-plan", h.PlanMigrationExecution)
				r.Post("/migration/execute", h.ExecuteMigration)
				r.Get("/migration/execution-runs", h.ListMigrationExecutionRuns)
				r.Get("/migration/execution-runs/{runID}", h.GetMigrationExecutionRun)
				r.Get("/migration/execution-runs/{runID}/events", h.StreamMigrationExecutionRun)

				// Accounts
				r.Get("/accounts", h.ListAccounts)
				r.Post("/accounts", h.CreateAccount)
				r.Post("/accounts/import", h.ImportAccounts)
				r.Get("/accounts/hierarchy", h.GetAccountHierarchy)
				r.Get("/accounts/{accountID}", h.GetAccount)
				r.Put("/accounts/{accountID}", h.UpdateAccount)
				r.Delete("/accounts/{accountID}", h.DeleteAccount)

				// Journal entries
				r.Post("/journal-entries/import-opening-balances", h.ImportOpeningBalances)
				r.Post("/journal-entries/import", h.ImportJournalEntries)
				r.Get("/journal-entries", h.ListJournalEntries)
				r.Get("/journal-entries/{entryID}", h.GetJournalEntry)
				r.Post("/journal-entries", h.CreateJournalEntry)
				r.Post("/journal-entries/{entryID}/post", h.PostJournalEntry)
				r.Post("/journal-entries/{entryID}/void", h.VoidJournalEntry)
				r.Get("/journal-entry-templates", h.ListJournalEntryTemplates)
				r.Post("/journal-entry-templates", h.CreateJournalEntryTemplate)
				r.Post("/journal-entry-templates/generate-due", h.GenerateDueJournalEntryTemplates)
				r.Get("/journal-entry-templates/{templateID}", h.GetJournalEntryTemplate)
				r.Post("/journal-entry-templates/{templateID}/generate", h.GenerateJournalEntryTemplate)
				r.Post("/journal-entry-templates/{templateID}/apply", h.ApplyJournalEntryTemplate)

				// Contacts
				r.Get("/contacts", h.ListContacts)
				r.Post("/contacts", h.CreateContact)
				r.Post("/contacts/import", h.ImportContacts)
				r.Get("/contacts/{contactID}", h.GetContact)
				r.Put("/contacts/{contactID}", h.UpdateContact)
				r.Delete("/contacts/{contactID}", h.DeleteContact)

				// Invoices
				r.Get("/invoices", h.ListInvoices)
				r.Post("/invoices", h.CreateInvoice)
				r.Post("/invoices/import", h.ImportInvoices)
				r.Post("/invoices/import-einvoice", h.ImportEInvoice)
				r.Get("/invoices/{invoiceID}", h.GetInvoice)
				r.Get("/invoices/{invoiceID}/pdf", h.GetInvoicePDF)
				r.Post("/invoices/{invoiceID}/send", h.SendInvoice)
				r.Post("/invoices/{invoiceID}/void", h.VoidInvoice)
				r.Get("/invoices/{invoiceID}/reminders", h.GetInvoiceReminderHistory)

				// Payment Reminders
				r.Get("/invoices/overdue", h.GetOverdueInvoices)
				r.Post("/invoices/reminders", h.SendPaymentReminder)
				r.Post("/invoices/reminders/bulk", h.SendBulkPaymentReminders)

				// Quotes
				r.Get("/quotes", h.ListQuotes)
				r.Post("/quotes", h.CreateQuote)
				r.Post("/quotes/import", h.ImportQuotes)
				r.Get("/quotes/{quoteID}", h.GetQuote)
				r.Get("/quotes/{quoteID}/pdf", h.GetQuotePDF)
				r.Put("/quotes/{quoteID}", h.UpdateQuote)
				r.Delete("/quotes/{quoteID}", h.DeleteQuote)
				r.Post("/quotes/{quoteID}/email", h.EmailQuote)
				r.Post("/quotes/{quoteID}/send", h.SendQuote)
				r.Post("/quotes/{quoteID}/accept", h.AcceptQuote)
				r.Post("/quotes/{quoteID}/reject", h.RejectQuote)
				r.Post("/quotes/{quoteID}/convert-to-invoice", h.ConvertQuoteToInvoice)

				// Orders
				r.Get("/orders", h.ListOrders)
				r.Post("/orders", h.CreateOrder)
				r.Post("/orders/import", h.ImportOrders)
				r.Get("/orders/{orderID}", h.GetOrder)
				r.Get("/orders/{orderID}/pdf", h.GetOrderPDF)
				r.Put("/orders/{orderID}", h.UpdateOrder)
				r.Delete("/orders/{orderID}", h.DeleteOrder)
				r.Post("/orders/{orderID}/email", h.EmailOrder)
				r.Get("/orders/{orderID}/stock-check", h.CheckOrderStock)
				r.Get("/orders/{orderID}/stock-reservations", h.ListOrderStockReservations)
				r.Get("/orders/{orderID}/pick-list", h.GetOrderPickList)
				r.Post("/orders/{orderID}/reserve-stock", h.ReserveOrderStock)
				r.Post("/orders/{orderID}/release-stock", h.ReleaseOrderStock)
				r.Post("/orders/{orderID}/confirm", h.ConfirmOrder)
				r.Post("/orders/{orderID}/process", h.ProcessOrder)
				r.Post("/orders/{orderID}/ship", h.ShipOrder)
				r.Post("/orders/{orderID}/deliver", h.DeliverOrder)
				r.Post("/orders/{orderID}/cancel", h.CancelOrder)
				r.Post("/orders/{orderID}/convert-to-invoice", h.ConvertOrderToInvoice)

				// Fixed Assets
				r.Get("/asset-categories", h.ListAssetCategories)
				r.Post("/asset-categories", h.CreateAssetCategory)
				r.Get("/asset-categories/{categoryID}", h.GetAssetCategory)
				r.Delete("/asset-categories/{categoryID}", h.DeleteAssetCategory)
				r.Get("/assets", h.ListAssets)
				r.Post("/assets", h.CreateAsset)
				r.Post("/assets/import", h.ImportAssets)
				r.Get("/assets/{assetID}", h.GetAsset)
				r.Put("/assets/{assetID}", h.UpdateAsset)
				r.Delete("/assets/{assetID}", h.DeleteAsset)
				r.Post("/assets/{assetID}/activate", h.ActivateAsset)
				r.Post("/assets/{assetID}/dispose", h.DisposeAsset)
				r.Post("/assets/{assetID}/depreciation", h.RecordDepreciation)
				r.Get("/assets/{assetID}/depreciation", h.GetDepreciationHistory)

				// Inventory - Product Categories
				r.Get("/product-categories", h.ListProductCategories)
				r.Post("/product-categories", h.CreateProductCategory)
				r.Post("/product-categories/import", h.ImportProductCategories)
				r.Get("/product-categories/{categoryID}", h.GetProductCategory)
				r.Delete("/product-categories/{categoryID}", h.DeleteProductCategory)

				// Inventory - Products
				r.Get("/products", h.ListProducts)
				r.Post("/products", h.CreateProduct)
				r.Post("/products/import", h.ImportProducts)
				r.Get("/products/{productID}", h.GetProduct)
				r.Put("/products/{productID}", h.UpdateProduct)
				r.Delete("/products/{productID}", h.DeleteProduct)
				r.Get("/products/{productID}/stock-levels", h.GetStockLevels)
				r.Get("/products/{productID}/movements", h.GetInventoryMovements)
				r.Get("/inventory/valuation", h.GetInventoryValuation)
				r.Get("/inventory/subledger-reconciliation", h.GetInventorySubledgerReconciliation)
				r.Get("/inventory/lots", h.GetInventoryLotReport)

				// Inventory - Warehouses
				r.Get("/warehouses", h.ListWarehouses)
				r.Post("/warehouses", h.CreateWarehouse)
				r.Post("/warehouses/import", h.ImportWarehouses)
				r.Get("/warehouses/{warehouseID}", h.GetWarehouse)
				r.Put("/warehouses/{warehouseID}", h.UpdateWarehouse)
				r.Delete("/warehouses/{warehouseID}", h.DeleteWarehouse)

				// Inventory - Stock Operations
				r.Post("/inventory/adjust", h.AdjustStock)
				r.Post("/inventory/stock-import", h.ImportStockAdjustments)
				r.Post("/inventory/issue", h.IssueStock)
				r.Post("/inventory/transfer", h.TransferStock)
				r.Post("/inventory/reserve", h.ReserveStock)
				r.Post("/inventory/release", h.ReleaseStock)

				// Payments
				r.Get("/payments", h.ListPayments)
				r.Post("/payments", h.CreatePayment)
				r.Post("/payments/import", h.ImportPayments)
				r.Post("/payments/sepa-export", h.ExportSEPAPayments)
				r.Get("/payments/{paymentID}", h.GetPayment)
				r.Post("/payments/{paymentID}/allocate", h.AllocatePayment)
				r.Post("/payments/{paymentID}/reverse", h.ReversePayment)
				r.Get("/payments/unallocated", h.GetUnallocatedPayments)

				// Reports
				r.Get("/reports/trial-balance", h.GetTrialBalance)
				r.Get("/reports/account-balance/{accountID}", h.GetAccountBalance)
				r.Get("/reports/balance-sheet", h.GetBalanceSheet)
				r.Get("/reports/income-statement", h.GetIncomeStatement)
				r.Get("/reports/consolidated", h.GetConsolidatedReport)
				r.Get("/reports/annual", h.GetAnnualReport)
				r.Get("/reports/cash-flow", h.GetCashFlowStatement)
				r.Get("/reports/cash-flow/mapping", h.GetCashFlowMapping)
				r.Put("/reports/cash-flow/mapping", h.UpdateCashFlowMapping)
				r.Get("/reports/balance-confirmations", h.GetBalanceConfirmationSummary)
				r.Get("/reports/balance-confirmations/{contactID}", h.GetBalanceConfirmation)
				r.Get("/reports/contact-statements/{contactID}", h.GetContactStatement)
				r.Get("/reports/sales-margin", h.GetSalesMarginReport)
				r.Get("/reports/customer-profitability", h.GetCustomerProfitabilityReport)
				r.Get("/reports/budget-vs-actual", h.GetBudgetVsActualReport)

				// Cost Centers
				r.Get("/cost-centers", h.ListCostCenters)
				r.Post("/cost-centers", h.CreateCostCenter)
				r.Post("/cost-centers/import", h.ImportCostCenters)
				r.Get("/cost-centers/report", h.GetCostCenterReport)
				r.Get("/cost-centers/allocations", h.ListCostAllocations)
				r.Post("/cost-centers/allocations", h.CreateCostAllocation)
				r.Post("/cost-centers/allocations/import", h.ImportCostAllocations)
				r.Get("/cost-centers/{costCenterID}", h.GetCostCenter)
				r.Put("/cost-centers/{costCenterID}", h.UpdateCostCenter)
				r.Delete("/cost-centers/{costCenterID}", h.DeleteCostCenter)

				// Analytics
				r.Get("/analytics/dashboard", h.GetDashboardSummary)
				r.Get("/analytics/revenue-expense", h.GetRevenueExpenseChart)
				r.Get("/analytics/cash-flow", h.GetCashFlowChart)
				r.Get("/analytics/activity", h.GetRecentActivity)
				r.Get("/reports/aging/receivables", h.GetReceivablesAging)
				r.Get("/reports/aging/payables", h.GetPayablesAging)

				// Recurring Invoices
				r.Get("/recurring-invoices", h.ListRecurringInvoices)
				r.Post("/recurring-invoices", h.CreateRecurringInvoice)
				r.Post("/recurring-invoices/import", h.ImportRecurringInvoices)
				r.Post("/recurring-invoices/from-invoice/{invoiceID}", h.CreateRecurringInvoiceFromInvoice)
				r.Post("/recurring-invoices/generate-due", h.GenerateDueRecurringInvoices)
				r.Get("/recurring-invoices/{recurringID}", h.GetRecurringInvoice)
				r.Put("/recurring-invoices/{recurringID}", h.UpdateRecurringInvoice)
				r.Delete("/recurring-invoices/{recurringID}", h.DeleteRecurringInvoice)
				r.Post("/recurring-invoices/{recurringID}/pause", h.PauseRecurringInvoice)
				r.Post("/recurring-invoices/{recurringID}/resume", h.ResumeRecurringInvoice)
				r.Post("/recurring-invoices/{recurringID}/generate", h.GenerateRecurringInvoice)

				// Email Settings
				r.Get("/settings/smtp", h.GetSMTPConfig)
				r.Put("/settings/smtp", h.UpdateSMTPConfig)
				r.Post("/settings/smtp/test", h.TestSMTP)
				r.Get("/email-templates", h.ListEmailTemplates)
				r.Put("/email-templates/{templateType}", h.UpdateEmailTemplate)
				r.Get("/email-log", h.GetEmailLog)

				// Reminder Rules (Automated Payment Reminders)
				r.Get("/reminder-rules", h.ListReminderRules)
				r.Post("/reminder-rules", h.CreateReminderRule)
				r.Post("/reminder-rules/trigger", h.TriggerReminders)
				r.Get("/reminder-rules/{ruleID}", h.GetReminderRule)
				r.Put("/reminder-rules/{ruleID}", h.UpdateReminderRule)
				r.Delete("/reminder-rules/{ruleID}", h.DeleteReminderRule)

				// Interest Calculations
				r.Get("/settings/interest", h.GetInterestSettings)
				r.Put("/settings/interest", h.UpdateInterestSettings)
				r.Get("/invoices/overdue-with-interest", h.GetOverdueInvoicesWithInterest)
				r.Get("/invoices/{invoiceID}/interest", h.GetInvoiceInterest)
				r.Get("/invoices/{invoiceID}/interest/history", h.GetInvoiceInterestHistory)

				// Email Actions (linked to invoices/payments)
				r.Post("/invoices/{invoiceID}/email", h.EmailInvoice)
				r.Post("/payments/{paymentID}/email-receipt", h.EmailPaymentReceipt)

				// Bank Accounts
				r.Get("/bank-accounts", h.ListBankAccounts)
				r.Post("/bank-accounts", h.CreateBankAccount)
				r.Post("/bank-accounts/import", h.ImportBankAccounts)
				r.Get("/bank-match-rules", h.ListBankMatchRules)
				r.Post("/bank-match-rules", h.CreateBankMatchRule)
				r.Get("/bank-match-rules/{ruleID}", h.GetBankMatchRule)
				r.Put("/bank-match-rules/{ruleID}", h.UpdateBankMatchRule)
				r.Delete("/bank-match-rules/{ruleID}", h.DeleteBankMatchRule)
				r.Get("/bank-accounts/{accountID}", h.GetBankAccount)
				r.Put("/bank-accounts/{accountID}", h.UpdateBankAccount)
				r.Delete("/bank-accounts/{accountID}", h.DeleteBankAccount)

				// Bank Transactions
				r.Get("/bank-accounts/{accountID}/transactions", h.ListBankTransactions)
				r.Post("/bank-accounts/{accountID}/import", h.ImportBankTransactions)
				r.Get("/bank-accounts/{accountID}/import-history", h.GetImportHistory)
				r.Get("/bank-transactions/{transactionID}", h.GetBankTransaction)
				r.Get("/bank-transactions/{transactionID}/suggestions", h.GetMatchSuggestions)
				r.Post("/bank-transactions/{transactionID}/match", h.MatchBankTransaction)
				r.Post("/bank-transactions/{transactionID}/unmatch", h.UnmatchBankTransaction)
				r.Post("/bank-transactions/{transactionID}/review", h.ReviewBankTransaction)
				r.Post("/bank-transactions/{transactionID}/create-payment", h.CreatePaymentFromTransaction)

				// Bank Reconciliation
				r.Get("/bank-accounts/{accountID}/reconciliations", h.ListReconciliations)
				r.Post("/bank-accounts/{accountID}/reconciliation", h.CreateReconciliation)
				r.Get("/reconciliations/{reconciliationID}", h.GetReconciliation)
				r.Post("/reconciliations/{reconciliationID}/complete", h.CompleteReconciliation)
				r.Post("/bank-accounts/{accountID}/auto-match", h.AutoMatchTransactions)

				// Tax (Estonian KMD)
				r.Post("/tax/kmd", h.HandleGenerateKMD)
				r.Post("/tax/kmd/import-history", h.HandleImportKMDHistory)
				r.Get("/tax/kmd", h.HandleListKMD)
				r.Get("/tax/kmd/{year}/{month}/xml", h.HandleExportKMD)
				r.Get("/tax/kmd/{year}/{month}/inf", h.HandleGenerateKMDINF)
				r.Post("/tax/kmd/{year}/{month}/submit", h.HandleMarkKMDSubmitted)
				r.Post("/tax/kmd/{year}/{month}/accept", h.HandleMarkKMDAccepted)
				r.Get("/tax/eu-vat/oss", h.HandleGenerateEUVATOSS)

				// Payroll - Employees
				r.Get("/employees", h.ListEmployees)
				r.Post("/employees", h.CreateEmployee)
				r.Post("/employees/import", h.ImportEmployees)
				r.Get("/employees/{employeeID}", h.GetEmployee)
				r.Put("/employees/{employeeID}", h.UpdateEmployee)
				r.Post("/employees/{employeeID}/salary", h.SetBaseSalary)
				r.Get("/employees/{employeeID}/salary-components", h.ListSalaryComponents)
				r.Post("/employees/{employeeID}/salary-components", h.AddSalaryComponent)

				// Payroll - Runs
				r.Get("/payroll-runs", h.ListPayrollRuns)
				r.Post("/payroll-runs", h.CreatePayrollRun)
				r.Post("/payroll-runs/import-history", h.ImportPayrollHistory)
				r.Get("/payroll-runs/{runID}", h.GetPayrollRun)
				r.Patch("/payroll-runs/{runID}/payment-date", h.UpdatePayrollRunPaymentDate)
				r.Post("/payroll-runs/{runID}/calculate", h.CalculatePayroll)
				r.Post("/payroll-runs/{runID}/process", h.ProcessPayrollRun)
				r.Post("/payroll-runs/{runID}/approve", h.ApprovePayroll)
				r.Get("/payroll-runs/{runID}/payslips", h.GetPayslips)
				r.Get("/payroll-runs/{runID}/payslips/{payslipID}/pdf", h.GetPayslipPDF)
				r.Post("/payroll-runs/{runID}/tsd", h.GenerateTSD)

				// Payroll - Tax Preview
				r.Post("/payroll/tax-preview", h.CalculateTaxPreview)

				// Leave/Absence Management
				r.Get("/absence-types", h.ListAbsenceTypes)
				r.Get("/absence-types/{typeID}", h.GetAbsenceType)
				r.Get("/employees/{employeeID}/leave-balances", h.ListLeaveBalances)
				r.Get("/employees/{employeeID}/leave-balances/{year}", h.GetLeaveBalancesByYear)
				r.Put("/employees/{employeeID}/leave-balances/{year}/{typeID}", h.UpdateLeaveBalance)
				r.Post("/employees/{employeeID}/leave-balances/{year}/initialize", h.InitializeLeaveBalances)
				r.Post("/leave-balances/import", h.ImportLeaveBalances)
				r.Get("/leave-records", h.ListLeaveRecords)
				r.Post("/leave-records", h.CreateLeaveRecord)
				r.Get("/leave-records/{recordID}", h.GetLeaveRecord)
				r.Post("/leave-records/{recordID}/approve", h.ApproveLeaveRecord)
				r.Post("/leave-records/{recordID}/reject", h.RejectLeaveRecord)
				r.Post("/leave-records/{recordID}/cancel", h.CancelLeaveRecord)

				// TSD Declarations
				r.Get("/tsd", h.ListTSD)
				r.Get("/tsd/{year}/{month}", h.GetTSD)
				r.Get("/tsd/{year}/{month}/xml", h.ExportTSDXML)
				r.Get("/tsd/{year}/{month}/csv", h.ExportTSDCSV)
				r.Post("/tsd/import-history", h.ImportTSDHistory)
				r.Post("/tsd/{year}/{month}/submit", h.MarkTSDSubmitted)
				r.Post("/tsd/{year}/{month}/accept", h.MarkTSDAccepted)
				r.Post("/tsd/{year}/{month}/reject", h.MarkTSDRejected)

				// User Management
				r.Get("/users", h.ListTenantUsers)
				r.Delete("/users/{userID}", h.RemoveTenantUser)
				r.Put("/users/{userID}/role", h.UpdateTenantUserRole)
				r.Put("/users/{userID}/status", h.UpdateTenantUserStatus)
				r.Get("/users/{userID}/sessions", h.ListTenantUserAuthSessions)
				r.Delete("/users/{userID}/sessions", h.RevokeTenantUserAuthSessions)
				r.Delete("/users/{userID}/sessions/{sessionID}", h.RevokeTenantUserAuthSession)
				r.Get("/users/{userID}/api-tokens", h.ListTenantUserAPITokens)
				r.Delete("/users/{userID}/api-tokens/{tokenID}", h.RevokeTenantUserAPIToken)
				r.Get("/users/{userID}/security-events", h.ListTenantUserSecurityAuditEvents)
				r.Get("/audit-events", h.ListTenantAuditEvents)

				// Webhooks
				r.Get("/webhooks/events", h.ListWebhookEventTypes)
				r.Get("/webhooks", h.ListWebhookEndpoints)
				r.Post("/webhooks", h.CreateWebhookEndpoint)
				r.Get("/webhooks/{webhookID}", h.GetWebhookEndpoint)
				r.Put("/webhooks/{webhookID}", h.UpdateWebhookEndpoint)
				r.Delete("/webhooks/{webhookID}", h.DeleteWebhookEndpoint)
				r.Get("/webhooks/{webhookID}/deliveries", h.ListWebhookDeliveries)
				r.Post("/webhooks/{webhookID}/test", h.TestWebhookEndpoint)

				// Expenses
				r.Get("/expenses", h.ListExpenses)
				r.Post("/expenses", h.CreateExpense)
				r.Post("/expenses/import", h.ImportExpenses)
				r.Get("/expenses/{expenseID}", h.GetExpense)
				r.Post("/expenses/{expenseID}/submit", h.SubmitExpense)
				r.Post("/expenses/{expenseID}/approve", h.ApproveExpense)
				r.Post("/expenses/{expenseID}/reject", h.RejectExpense)
				r.Post("/expenses/{expenseID}/post", h.PostExpense)

				// Invitations
				r.Get("/invitations", h.ListInvitations)
				r.Post("/invitations", h.CreateInvitation)
				r.Delete("/invitations/{invitationID}", h.RevokeInvitation)

				// Tenant Plugin Management
				r.Get("/plugins", h.ListTenantPlugins)
				r.Post("/plugins/{pluginID}/enable", h.EnableTenantPlugin)
				r.Post("/plugins/{pluginID}/disable", h.DisableTenantPlugin)
				r.Get("/plugins/{pluginID}/settings", h.GetTenantPluginSettings)
				r.Put("/plugins/{pluginID}/settings", h.UpdateTenantPluginSettings)
				r.Get("/plugins/{pluginID}/runtime/*", h.InvokeTenantPluginRoute)
				r.Post("/plugins/{pluginID}/runtime/*", h.InvokeTenantPluginRoute)
				r.Put("/plugins/{pluginID}/runtime/*", h.InvokeTenantPluginRoute)
				r.Patch("/plugins/{pluginID}/runtime/*", h.InvokeTenantPluginRoute)
				r.Delete("/plugins/{pluginID}/runtime/*", h.InvokeTenantPluginRoute)
			})

			// Register exact tenant management routes after the tenant-scoped
			// subrouter so /tenants/{tenantID} is not shadowed by child routes.
			r.Get("/tenants/{tenantID}", h.GetTenant)
			r.Put("/tenants/{tenantID}", h.UpdateTenant)
		})
	})

	return r
}
