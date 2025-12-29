package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
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
	"github.com/HMB-research/open-accounting/internal/auth"
	"github.com/HMB-research/open-accounting/internal/banking"
	"github.com/HMB-research/open-accounting/internal/contacts"
	"github.com/HMB-research/open-accounting/internal/email"
	"github.com/HMB-research/open-accounting/internal/invoicing"
	"github.com/HMB-research/open-accounting/internal/payments"
	"github.com/HMB-research/open-accounting/internal/payroll"
	"github.com/HMB-research/open-accounting/internal/pdf"
	"github.com/HMB-research/open-accounting/internal/plugin"
	"github.com/HMB-research/open-accounting/internal/recurring"
	"github.com/HMB-research/open-accounting/internal/tax"
	"github.com/HMB-research/open-accounting/internal/tenant"
)

// Config holds the application configuration
type Config struct {
	Port           string
	DatabaseURL    string
	JWTSecret      string
	AccessExpiry   time.Duration
	RefreshExpiry  time.Duration
	AllowedOrigins []string
}

func main() {
	// Configure logging
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339})

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
	tenantService := tenant.NewService(pool)
	accountingService := accounting.NewService(pool)
	contactsService := contacts.NewService(pool)
	invoicingService := invoicing.NewService(pool, accountingService)
	paymentsService := payments.NewService(pool, invoicingService)
	pdfService := pdf.NewService()
	analyticsService := analytics.NewService(pool)
	recurringService := recurring.NewService(pool, invoicingService)
	emailService := email.NewService(pool)
	bankingService := banking.NewService(pool)
	taxService := tax.NewService(pool)
	payrollService := payroll.NewService(pool)
	pluginService := plugin.NewService(pool, "./plugins")

	// Load enabled plugins on startup
	if err := pluginService.LoadEnabledPlugins(ctx); err != nil {
		log.Warn().Err(err).Msg("Failed to load some plugins")
	}

	// Create handlers
	handlers := &Handlers{
		tokenService:      tokenService,
		tenantService:     tenantService,
		accountingService: accountingService,
		contactsService:   contactsService,
		invoicingService:  invoicingService,
		paymentsService:   paymentsService,
		pdfService:        pdfService,
		analyticsService:  analyticsService,
		recurringService:  recurringService,
		emailService:      emailService,
		bankingService:    bankingService,
		taxService:        taxService,
		payrollService:    payrollService,
		pluginService:     pluginService,
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
	if jwtSecret == "" {
		jwtSecret = "change-me-in-production"
		log.Warn().Msg("Using default JWT_SECRET - change this in production!")
	}

	origins := os.Getenv("ALLOWED_ORIGINS")
	allowedOrigins := []string{"http://localhost:5173", "http://localhost:3000"}
	if origins != "" {
		allowedOrigins = append(allowedOrigins, origins)
	}

	return &Config{
		Port:           port,
		DatabaseURL:    dbURL,
		JWTSecret:      jwtSecret,
		AccessExpiry:   15 * time.Minute,
		RefreshExpiry:  7 * 24 * time.Hour,
		AllowedOrigins: allowedOrigins,
	}
}

func setupRouter(cfg *Config, h *Handlers, tokenService *auth.TokenService) *chi.Mux {
	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))

	// CORS
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   cfg.AllowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Tenant-ID"},
		ExposedHeaders:   []string{"Link", "X-RateLimit-Limit", "X-RateLimit-Remaining", "X-RateLimit-Reset", "Retry-After"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Rate limiting (100 requests/minute, burst 10)
	rateLimiter := auth.DefaultRateLimiter()
	r.Use(rateLimiter.Middleware)

	// Health check
	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("OK"))
	})

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

		// Public invitation endpoints (no auth required)
		r.Get("/invitations/{token}", h.GetInvitationByToken)
		r.Post("/invitations/accept", h.AcceptInvitation)

		// Authenticated routes
		r.Group(func(r chi.Router) {
			r.Use(tokenService.Middleware)

			// User routes
			r.Get("/me", h.GetCurrentUser)
			r.Get("/me/tenants", h.ListMyTenants)

			// Tenant management
			r.Post("/tenants", h.CreateTenant)
			r.Get("/tenants/{tenantID}", h.GetTenant)
			r.Put("/tenants/{tenantID}", h.UpdateTenant)

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
			})

			// Tenant-scoped routes
			r.Route("/tenants/{tenantID}", func(r chi.Router) {
				r.Use(h.TenantContext)

				// Onboarding
				r.Post("/complete-onboarding", h.CompleteOnboarding)

				// Accounts
				r.Get("/accounts", h.ListAccounts)
				r.Post("/accounts", h.CreateAccount)
				r.Get("/accounts/{accountID}", h.GetAccount)

				// Journal entries
				r.Get("/journal-entries/{entryID}", h.GetJournalEntry)
				r.Post("/journal-entries", h.CreateJournalEntry)
				r.Post("/journal-entries/{entryID}/post", h.PostJournalEntry)
				r.Post("/journal-entries/{entryID}/void", h.VoidJournalEntry)

				// Contacts
				r.Get("/contacts", h.ListContacts)
				r.Post("/contacts", h.CreateContact)
				r.Get("/contacts/{contactID}", h.GetContact)
				r.Put("/contacts/{contactID}", h.UpdateContact)
				r.Delete("/contacts/{contactID}", h.DeleteContact)

				// Invoices
				r.Get("/invoices", h.ListInvoices)
				r.Post("/invoices", h.CreateInvoice)
				r.Get("/invoices/{invoiceID}", h.GetInvoice)
				r.Get("/invoices/{invoiceID}/pdf", h.GetInvoicePDF)
				r.Post("/invoices/{invoiceID}/send", h.SendInvoice)
				r.Post("/invoices/{invoiceID}/void", h.VoidInvoice)

				// Payments
				r.Get("/payments", h.ListPayments)
				r.Post("/payments", h.CreatePayment)
				r.Get("/payments/{paymentID}", h.GetPayment)
				r.Post("/payments/{paymentID}/allocate", h.AllocatePayment)
				r.Get("/payments/unallocated", h.GetUnallocatedPayments)

				// Reports
				r.Get("/reports/trial-balance", h.GetTrialBalance)
				r.Get("/reports/account-balance/{accountID}", h.GetAccountBalance)
				r.Get("/reports/balance-sheet", h.GetBalanceSheet)
				r.Get("/reports/income-statement", h.GetIncomeStatement)

				// Analytics
				r.Get("/analytics/dashboard", h.GetDashboardSummary)
				r.Get("/analytics/revenue-expense", h.GetRevenueExpenseChart)
				r.Get("/analytics/cash-flow", h.GetCashFlowChart)
				r.Get("/reports/aging/receivables", h.GetReceivablesAging)
				r.Get("/reports/aging/payables", h.GetPayablesAging)

				// Recurring Invoices
				r.Get("/recurring-invoices", h.ListRecurringInvoices)
				r.Post("/recurring-invoices", h.CreateRecurringInvoice)
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

				// Email Actions (linked to invoices/payments)
				r.Post("/invoices/{invoiceID}/email", h.EmailInvoice)
				r.Post("/payments/{paymentID}/email-receipt", h.EmailPaymentReceipt)

				// Bank Accounts
				r.Get("/bank-accounts", h.ListBankAccounts)
				r.Post("/bank-accounts", h.CreateBankAccount)
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
				r.Post("/bank-transactions/{transactionID}/create-payment", h.CreatePaymentFromTransaction)

				// Bank Reconciliation
				r.Get("/bank-accounts/{accountID}/reconciliations", h.ListReconciliations)
				r.Post("/bank-accounts/{accountID}/reconciliation", h.CreateReconciliation)
				r.Get("/reconciliations/{reconciliationID}", h.GetReconciliation)
				r.Post("/reconciliations/{reconciliationID}/complete", h.CompleteReconciliation)
				r.Post("/bank-accounts/{accountID}/auto-match", h.AutoMatchTransactions)

				// Tax (Estonian KMD)
				r.Post("/tax/kmd", h.HandleGenerateKMD)
				r.Get("/tax/kmd", h.HandleListKMD)
				r.Get("/tax/kmd/{year}/{month}/xml", h.HandleExportKMD)

				// Payroll - Employees
				r.Get("/employees", h.ListEmployees)
				r.Post("/employees", h.CreateEmployee)
				r.Get("/employees/{employeeID}", h.GetEmployee)
				r.Put("/employees/{employeeID}", h.UpdateEmployee)
				r.Post("/employees/{employeeID}/salary", h.SetBaseSalary)

				// Payroll - Runs
				r.Get("/payroll-runs", h.ListPayrollRuns)
				r.Post("/payroll-runs", h.CreatePayrollRun)
				r.Get("/payroll-runs/{runID}", h.GetPayrollRun)
				r.Post("/payroll-runs/{runID}/calculate", h.CalculatePayroll)
				r.Post("/payroll-runs/{runID}/approve", h.ApprovePayroll)
				r.Get("/payroll-runs/{runID}/payslips", h.GetPayslips)
				r.Post("/payroll-runs/{runID}/tsd", h.GenerateTSD)

				// Payroll - Tax Preview
				r.Post("/payroll/tax-preview", h.CalculateTaxPreview)

				// TSD Declarations
				r.Get("/tsd", h.ListTSD)
				r.Get("/tsd/{year}/{month}", h.GetTSD)
				r.Get("/tsd/{year}/{month}/xml", h.ExportTSDXML)
				r.Get("/tsd/{year}/{month}/csv", h.ExportTSDCSV)
				r.Post("/tsd/{year}/{month}/submit", h.MarkTSDSubmitted)

				// User Management
				r.Get("/users", h.ListTenantUsers)
				r.Delete("/users/{userID}", h.RemoveTenantUser)
				r.Put("/users/{userID}/role", h.UpdateTenantUserRole)

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
			})
		})
	})

	return r
}
