package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"github.com/HMB-research/open-accounting/internal/accounting"
	"github.com/HMB-research/open-accounting/internal/apitoken"
	"github.com/HMB-research/open-accounting/internal/contacts"
	"github.com/HMB-research/open-accounting/internal/documents"
	"github.com/HMB-research/open-accounting/internal/invoicing"
	"github.com/HMB-research/open-accounting/internal/payroll"
	"github.com/HMB-research/open-accounting/internal/tenant"
)

type cliApp struct {
	stdout io.Writer
	stderr io.Writer
}

func main() {
	app := &cliApp{
		stdout: os.Stdout,
		stderr: os.Stderr,
	}

	if err := app.run(context.Background(), os.Args[1:]); err != nil {
		_, _ = fmt.Fprintf(app.stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func (a *cliApp) run(ctx context.Context, args []string) error {
	if len(args) == 0 {
		a.printUsage()
		return nil
	}

	switch args[0] {
	case "auth":
		return a.runAuth(ctx, args[1:])
	case "tokens":
		return a.runTokens(ctx, args[1:])
	case "accounts":
		return a.runAccounts(ctx, args[1:])
	case "contacts":
		return a.runContacts(ctx, args[1:])
	case "employees":
		return a.runEmployees(ctx, args[1:])
	case "invoices":
		return a.runInvoices(ctx, args[1:])
	case "documents":
		return a.runDocuments(ctx, args[1:])
	case "journal":
		return a.runJournal(ctx, args[1:])
	case "help", "--help", "-h":
		a.printUsage()
		return nil
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func (a *cliApp) printUsage() {
	_, _ = fmt.Fprintln(a.stdout, "Open Accounting CLI")
	_, _ = fmt.Fprintln(a.stdout, "")
	_, _ = fmt.Fprintln(a.stdout, "Commands:")
	_, _ = fmt.Fprintln(a.stdout, "  auth init                 Bootstrap and store a tenant-scoped API token")
	_, _ = fmt.Fprintln(a.stdout, "  auth status               Show current CLI auth status")
	_, _ = fmt.Fprintln(a.stdout, "  auth logout               Remove local CLI config")
	_, _ = fmt.Fprintln(a.stdout, "  tokens list               List API tokens for the configured tenant")
	_, _ = fmt.Fprintln(a.stdout, "  tokens create             Create another API token")
	_, _ = fmt.Fprintln(a.stdout, "  tokens revoke             Revoke an API token by id")
	_, _ = fmt.Fprintln(a.stdout, "  accounts list             List accounts")
	_, _ = fmt.Fprintln(a.stdout, "  accounts create           Create an account")
	_, _ = fmt.Fprintln(a.stdout, "  accounts import           Import accounts from CSV")
	_, _ = fmt.Fprintln(a.stdout, "  contacts list             List contacts")
	_, _ = fmt.Fprintln(a.stdout, "  contacts create           Create a contact")
	_, _ = fmt.Fprintln(a.stdout, "  contacts import           Import contacts from CSV")
	_, _ = fmt.Fprintln(a.stdout, "  employees list            List employees")
	_, _ = fmt.Fprintln(a.stdout, "  employees create          Create an employee")
	_, _ = fmt.Fprintln(a.stdout, "  employees import          Import employees from CSV")
	_, _ = fmt.Fprintln(a.stdout, "  invoices import           Import invoices from CSV")
	_, _ = fmt.Fprintln(a.stdout, "  documents list            List documents for a record")
	_, _ = fmt.Fprintln(a.stdout, "  documents upload          Upload a document to a record")
	_, _ = fmt.Fprintln(a.stdout, "  documents mark-reviewed   Mark a document as reviewed")
	_, _ = fmt.Fprintln(a.stdout, "  documents delete          Delete a document")
	_, _ = fmt.Fprintln(a.stdout, "  journal import-opening-balances  Import opening balances from CSV")
	_, _ = fmt.Fprintln(a.stdout, "")
	_, _ = fmt.Fprintln(a.stdout, "Environment overrides:")
	_, _ = fmt.Fprintln(a.stdout, "  OA_BASE_URL, OA_API_TOKEN, OA_TENANT_ID")
}

func (a *cliApp) runAuth(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("auth subcommand required")
	}

	switch args[0] {
	case "init":
		fs := flag.NewFlagSet("auth init", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		baseURL := fs.String("base-url", defaultBaseURL(), "API base URL")
		email := fs.String("email", "", "User email")
		password := fs.String("password", "", "User password")
		passwordStdin := fs.Bool("password-stdin", false, "Read password from stdin")
		tenantSelector := fs.String("tenant", "", "Tenant id, slug, or name")
		tokenName := fs.String("token-name", "Open Accounting CLI", "API token display name")
		expiresInDays := fs.Int("expires-in-days", 365, "Token lifetime in days (0 for no expiry)")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}

		if strings.TrimSpace(*email) == "" {
			return errors.New("email is required")
		}

		passwordValue, err := resolvePassword(*password, *passwordStdin)
		if err != nil {
			return err
		}

		client := newAPIClient(*baseURL, "")
		loginResp, err := client.login(ctx, *email, passwordValue)
		if err != nil {
			return err
		}

		memberships, err := client.listMyTenants(ctx, loginResp.AccessToken)
		if err != nil {
			return err
		}
		membership, err := resolveTenantMembership(memberships, *tenantSelector)
		if err != nil {
			return err
		}

		createResp, err := client.createAPIToken(ctx, membership.Tenant.ID, &apitoken.CreateRequest{
			Name:      *tokenName,
			ExpiresAt: parseDaysToExpiry(*expiresInDays),
		}, loginResp.AccessToken)
		if err != nil {
			return err
		}

		cfg := &cliConfig{
			BaseURL:    normalizeBaseURL(*baseURL),
			TenantID:   membership.Tenant.ID,
			TenantName: membership.Tenant.Name,
			TenantSlug: membership.Tenant.Slug,
			APIToken:   createResp.Token,
		}
		if err := saveConfig(cfg); err != nil {
			return err
		}

		_, _ = fmt.Fprintf(a.stdout, "Stored API token for tenant %s (%s)\n", membership.Tenant.Name, membership.Tenant.ID)
		_, _ = fmt.Fprintf(a.stdout, "Token id: %s\n", createResp.APIToken.ID)
		_, _ = fmt.Fprintf(a.stdout, "Token preview: %s\n", tokenPreview(createResp.Token))
		return nil

	case "status":
		cfg, err := loadRuntimeConfig()
		if err != nil {
			return err
		}
		if strings.TrimSpace(cfg.APIToken) == "" {
			return errors.New("no API token configured")
		}

		client := newAPIClient(cfg.BaseURL, cfg.APIToken)
		user, err := client.getCurrentUser(ctx)
		if err != nil {
			return err
		}

		_, _ = fmt.Fprintf(a.stdout, "Base URL: %s\n", cfg.BaseURL)
		_, _ = fmt.Fprintf(a.stdout, "Tenant: %s (%s)\n", cfg.TenantName, cfg.TenantID)
		_, _ = fmt.Fprintf(a.stdout, "User: %s <%s>\n", user.Name, user.Email)
		_, _ = fmt.Fprintf(a.stdout, "Token: %s\n", tokenPreview(cfg.APIToken))
		return nil

	case "logout":
		if err := deleteConfig(); err != nil {
			return err
		}
		_, _ = fmt.Fprintln(a.stdout, "Removed local CLI config")
		return nil

	default:
		return fmt.Errorf("unknown auth subcommand %q", args[0])
	}
}

func (a *cliApp) runTokens(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("tokens subcommand required")
	}

	cfg, client, err := a.loadAuthenticatedClient()
	if err != nil {
		return err
	}

	switch args[0] {
	case "list":
		fs := flag.NewFlagSet("tokens list", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}

		tokens, err := client.listAPITokens(ctx, cfg.TenantID)
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, tokens)
		}
		printAPITokensTable(a.stdout, tokens)
		return nil

	case "create":
		fs := flag.NewFlagSet("tokens create", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		name := fs.String("name", "", "API token display name")
		expiresInDays := fs.Int("expires-in-days", 365, "Token lifetime in days (0 for no expiry)")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*name) == "" {
			return errors.New("name is required")
		}

		result, err := client.createAPIToken(ctx, cfg.TenantID, &apitoken.CreateRequest{
			Name:      *name,
			ExpiresAt: parseDaysToExpiry(*expiresInDays),
		}, cfg.APIToken)
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, result)
		}
		_, _ = fmt.Fprintf(a.stdout, "Created token %s (%s)\n", result.APIToken.Name, result.APIToken.ID)
		_, _ = fmt.Fprintf(a.stdout, "Token: %s\n", result.Token)
		return nil

	case "revoke":
		fs := flag.NewFlagSet("tokens revoke", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		tokenID := fs.String("id", "", "API token id")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*tokenID) == "" {
			return errors.New("id is required")
		}

		if err := client.revokeAPIToken(ctx, cfg.TenantID, *tokenID); err != nil {
			return err
		}
		_, _ = fmt.Fprintf(a.stdout, "Revoked token %s\n", *tokenID)
		return nil

	default:
		return fmt.Errorf("unknown tokens subcommand %q", args[0])
	}
}

func (a *cliApp) runAccounts(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("accounts subcommand required")
	}
	cfg, client, err := a.loadAuthenticatedClient()
	if err != nil {
		return err
	}

	switch args[0] {
	case "list":
		fs := flag.NewFlagSet("accounts list", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		activeOnly := fs.Bool("active-only", false, "List only active accounts")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		accountsList, err := client.listAccounts(ctx, cfg.TenantID, *activeOnly)
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, accountsList)
		}
		printAccountsTable(a.stdout, accountsList)
		return nil

	case "create":
		fs := flag.NewFlagSet("accounts create", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		code := fs.String("code", "", "Account code")
		name := fs.String("name", "", "Account name")
		accountType := fs.String("type", "", "Account type: ASSET, LIABILITY, EQUITY, REVENUE, EXPENSE")
		description := fs.String("description", "", "Description")
		parentID := fs.String("parent-id", "", "Parent account id")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*code) == "" || strings.TrimSpace(*name) == "" || strings.TrimSpace(*accountType) == "" {
			return errors.New("code, name, and type are required")
		}
		normalizedType := accounting.AccountType(strings.ToUpper(strings.TrimSpace(*accountType)))
		if !isValidAccountType(normalizedType) {
			return fmt.Errorf("invalid account type %q", *accountType)
		}
		var parentRef *string
		if trimmed := strings.TrimSpace(*parentID); trimmed != "" {
			parentRef = &trimmed
		}

		account, err := client.createAccount(ctx, cfg.TenantID, &accounting.CreateAccountRequest{
			Code:        strings.TrimSpace(*code),
			Name:        strings.TrimSpace(*name),
			AccountType: normalizedType,
			ParentID:    parentRef,
			Description: strings.TrimSpace(*description),
		})
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, account)
		}
		_, _ = fmt.Fprintf(a.stdout, "Created account %s (%s)\n", account.Code, account.ID)
		return nil

	case "import":
		fs := flag.NewFlagSet("accounts import", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		filePath := fs.String("file", "", "CSV file path")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*filePath) == "" {
			return errors.New("file is required")
		}

		content, fileName, err := readCSVInput(*filePath)
		if err != nil {
			return err
		}
		result, err := client.importAccounts(ctx, cfg.TenantID, &accounting.ImportAccountsRequest{
			FileName:   fileName,
			CSVContent: content,
		})
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, result)
		}
		_, _ = fmt.Fprintf(a.stdout, "Processed %d rows, created %d accounts, skipped %d rows\n", result.RowsProcessed, result.AccountsCreated, result.RowsSkipped)
		return nil

	default:
		return fmt.Errorf("unknown accounts subcommand %q", args[0])
	}
}

func (a *cliApp) runContacts(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("contacts subcommand required")
	}
	cfg, client, err := a.loadAuthenticatedClient()
	if err != nil {
		return err
	}

	switch args[0] {
	case "list":
		fs := flag.NewFlagSet("contacts list", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		contactType := fs.String("type", "", "Contact type: CUSTOMER, SUPPLIER, BOTH")
		search := fs.String("search", "", "Search term")
		activeOnly := fs.Bool("active-only", false, "List only active contacts")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		filter := contacts.ContactFilter{
			ActiveOnly: *activeOnly,
			Search:     strings.TrimSpace(*search),
		}
		if trimmed := strings.TrimSpace(*contactType); trimmed != "" {
			filter.ContactType = contacts.ContactType(strings.ToUpper(trimmed))
		}

		contactsList, err := client.listContacts(ctx, cfg.TenantID, filter)
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, contactsList)
		}
		printContactsTable(a.stdout, contactsList)
		return nil

	case "create":
		fs := flag.NewFlagSet("contacts create", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		name := fs.String("name", "", "Contact name")
		contactType := fs.String("type", "CUSTOMER", "Contact type: CUSTOMER, SUPPLIER, BOTH")
		code := fs.String("code", "", "Contact code")
		email := fs.String("email", "", "Email")
		phone := fs.String("phone", "", "Phone")
		regCode := fs.String("reg-code", "", "Registration code")
		vatNumber := fs.String("vat-number", "", "VAT number")
		countryCode := fs.String("country-code", "EE", "Country code")
		paymentTermsDays := fs.Int("payment-terms-days", 14, "Payment terms in days")
		creditLimit := fs.String("credit-limit", "", "Credit limit")
		notes := fs.String("notes", "", "Notes")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*name) == "" {
			return errors.New("name is required")
		}

		creditLimitValue := decimal.Zero
		if trimmed := strings.TrimSpace(*creditLimit); trimmed != "" {
			parsed, err := decimal.NewFromString(trimmed)
			if err != nil {
				return fmt.Errorf("parse credit limit: %w", err)
			}
			creditLimitValue = parsed
		}

		contact, err := client.createContact(ctx, cfg.TenantID, &contacts.CreateContactRequest{
			Code:             strings.TrimSpace(*code),
			Name:             strings.TrimSpace(*name),
			ContactType:      contacts.ContactType(strings.ToUpper(strings.TrimSpace(*contactType))),
			RegCode:          strings.TrimSpace(*regCode),
			VATNumber:        strings.TrimSpace(*vatNumber),
			Email:            strings.TrimSpace(*email),
			Phone:            strings.TrimSpace(*phone),
			CountryCode:      strings.ToUpper(strings.TrimSpace(*countryCode)),
			PaymentTermsDays: *paymentTermsDays,
			CreditLimit:      creditLimitValue,
			Notes:            strings.TrimSpace(*notes),
		})
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, contact)
		}
		_, _ = fmt.Fprintf(a.stdout, "Created contact %s (%s)\n", contact.Name, contact.ID)
		return nil

	case "import":
		fs := flag.NewFlagSet("contacts import", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		filePath := fs.String("file", "", "CSV file path")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*filePath) == "" {
			return errors.New("file is required")
		}

		content, fileName, err := readCSVInput(*filePath)
		if err != nil {
			return err
		}
		result, err := client.importContacts(ctx, cfg.TenantID, &contacts.ImportContactsRequest{
			FileName:   fileName,
			CSVContent: content,
		})
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, result)
		}
		_, _ = fmt.Fprintf(a.stdout, "Processed %d rows, created %d contacts, skipped %d rows\n", result.RowsProcessed, result.ContactsCreated, result.RowsSkipped)
		return nil

	default:
		return fmt.Errorf("unknown contacts subcommand %q", args[0])
	}
}

func (a *cliApp) runInvoices(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("invoices subcommand required")
	}
	cfg, client, err := a.loadAuthenticatedClient()
	if err != nil {
		return err
	}

	switch args[0] {
	case "import":
		fs := flag.NewFlagSet("invoices import", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		filePath := fs.String("file", "", "CSV file path")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*filePath) == "" {
			return errors.New("file is required")
		}

		content, fileName, err := readCSVInput(*filePath)
		if err != nil {
			return err
		}
		result, err := client.importInvoices(ctx, cfg.TenantID, &invoicing.ImportInvoicesRequest{
			FileName:   fileName,
			CSVContent: content,
		})
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, result)
		}
		_, _ = fmt.Fprintf(a.stdout, "Processed %d rows, created %d invoices, imported %d lines, skipped %d rows\n", result.RowsProcessed, result.InvoicesCreated, result.LinesImported, result.RowsSkipped)
		return nil

	default:
		return fmt.Errorf("unknown invoices subcommand %q", args[0])
	}
}

func (a *cliApp) runEmployees(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("employees subcommand required")
	}
	cfg, client, err := a.loadAuthenticatedClient()
	if err != nil {
		return err
	}

	switch args[0] {
	case "list":
		fs := flag.NewFlagSet("employees list", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		activeOnly := fs.Bool("active-only", false, "List only active employees")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}

		employees, err := client.listEmployees(ctx, cfg.TenantID, *activeOnly)
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, employees)
		}
		printEmployeesTable(a.stdout, employees)
		return nil

	case "create":
		fs := flag.NewFlagSet("employees create", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		employeeNumber := fs.String("employee-number", "", "Employee number")
		firstName := fs.String("first-name", "", "First name")
		lastName := fs.String("last-name", "", "Last name")
		personalCode := fs.String("personal-code", "", "Personal code")
		email := fs.String("email", "", "Email")
		phone := fs.String("phone", "", "Phone")
		address := fs.String("address", "", "Address")
		bankAccount := fs.String("bank-account", "", "IBAN")
		startDate := fs.String("start-date", "", "Employment start date in YYYY-MM-DD")
		position := fs.String("position", "", "Position")
		department := fs.String("department", "", "Department")
		employmentType := fs.String("employment-type", "FULL_TIME", "Employment type: FULL_TIME, PART_TIME, CONTRACT")
		applyBasicExemption := fs.Bool("apply-basic-exemption", true, "Apply basic exemption")
		basicExemptionAmount := fs.String("basic-exemption-amount", "700.00", "Basic exemption amount")
		fundedPensionRate := fs.String("funded-pension-rate", "0.02", "Funded pension rate")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}

		if strings.TrimSpace(*firstName) == "" || strings.TrimSpace(*lastName) == "" || strings.TrimSpace(*startDate) == "" {
			return errors.New("first-name, last-name, and start-date are required")
		}

		parsedStartDate, err := time.Parse("2006-01-02", strings.TrimSpace(*startDate))
		if err != nil {
			return fmt.Errorf("parse start-date: %w", err)
		}

		basicExemptionValue := decimal.Zero
		if *applyBasicExemption {
			basicExemptionValue, err = decimal.NewFromString(strings.TrimSpace(*basicExemptionAmount))
			if err != nil {
				return fmt.Errorf("parse basic-exemption-amount: %w", err)
			}
		}

		fundedPensionValue := decimal.Zero
		if trimmed := strings.TrimSpace(*fundedPensionRate); trimmed != "" {
			fundedPensionValue, err = decimal.NewFromString(trimmed)
			if err != nil {
				return fmt.Errorf("parse funded-pension-rate: %w", err)
			}
		}

		employee, err := client.createEmployee(ctx, cfg.TenantID, &payroll.CreateEmployeeRequest{
			EmployeeNumber:       strings.TrimSpace(*employeeNumber),
			FirstName:            strings.TrimSpace(*firstName),
			LastName:             strings.TrimSpace(*lastName),
			PersonalCode:         strings.TrimSpace(*personalCode),
			Email:                strings.TrimSpace(*email),
			Phone:                strings.TrimSpace(*phone),
			Address:              strings.TrimSpace(*address),
			BankAccount:          strings.TrimSpace(*bankAccount),
			StartDate:            parsedStartDate,
			Position:             strings.TrimSpace(*position),
			Department:           strings.TrimSpace(*department),
			EmploymentType:       payroll.EmploymentType(strings.ToUpper(strings.TrimSpace(*employmentType))),
			ApplyBasicExemption:  *applyBasicExemption,
			BasicExemptionAmount: basicExemptionValue,
			FundedPensionRate:    fundedPensionValue,
		})
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, employee)
		}
		_, _ = fmt.Fprintf(a.stdout, "Created employee %s (%s)\n", employee.FullName(), employee.ID)
		return nil

	case "import":
		fs := flag.NewFlagSet("employees import", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		filePath := fs.String("file", "", "CSV file path")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*filePath) == "" {
			return errors.New("file is required")
		}

		content, fileName, err := readCSVInput(*filePath)
		if err != nil {
			return err
		}
		result, err := client.importEmployees(ctx, cfg.TenantID, &payroll.ImportEmployeesRequest{
			FileName:   fileName,
			CSVContent: content,
		})
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, result)
		}
		_, _ = fmt.Fprintf(
			a.stdout,
			"Processed %d rows, created %d employees, set %d salaries, skipped %d rows\n",
			result.RowsProcessed,
			result.EmployeesCreated,
			result.SalariesCreated,
			result.RowsSkipped,
		)
		return nil

	default:
		return fmt.Errorf("unknown employees subcommand %q", args[0])
	}
}

func (a *cliApp) runJournal(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("journal subcommand required")
	}
	cfg, client, err := a.loadAuthenticatedClient()
	if err != nil {
		return err
	}

	switch args[0] {
	case "import-opening-balances":
		fs := flag.NewFlagSet("journal import-opening-balances", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		filePath := fs.String("file", "", "CSV file path")
		entryDate := fs.String("entry-date", "", "Entry date in YYYY-MM-DD")
		description := fs.String("description", "Opening balances", "Journal entry description")
		reference := fs.String("reference", fmt.Sprintf("OB-%d", time.Now().Year()), "Reference")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*filePath) == "" {
			return errors.New("file is required")
		}
		if strings.TrimSpace(*entryDate) == "" {
			return errors.New("entry-date is required")
		}

		content, fileName, err := readCSVInput(*filePath)
		if err != nil {
			return err
		}
		result, err := client.importOpeningBalances(ctx, cfg.TenantID, &accounting.ImportOpeningBalancesRequest{
			FileName:    fileName,
			EntryDate:   strings.TrimSpace(*entryDate),
			Description: strings.TrimSpace(*description),
			Reference:   strings.TrimSpace(*reference),
			CSVContent:  content,
		})
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, result)
		}
		_, _ = fmt.Fprintf(
			a.stdout,
			"Created posted journal entry %s with %d lines, debit %s, credit %s\n",
			result.JournalEntry.EntryNumber,
			result.LinesImported,
			result.TotalDebit.String(),
			result.TotalCredit.String(),
		)
		return nil

	default:
		return fmt.Errorf("unknown journal subcommand %q", args[0])
	}
}

func (a *cliApp) runDocuments(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("documents subcommand required")
	}
	cfg, client, err := a.loadAuthenticatedClient()
	if err != nil {
		return err
	}

	switch args[0] {
	case "list":
		fs := flag.NewFlagSet("documents list", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		entityType := fs.String("entity-type", "", "Entity type: invoice, journal_entry, payment, bank_transaction, asset")
		entityID := fs.String("entity-id", "", "Entity id")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*entityType) == "" || strings.TrimSpace(*entityID) == "" {
			return errors.New("entity-type and entity-id are required")
		}

		docs, err := client.listDocuments(ctx, cfg.TenantID, strings.TrimSpace(*entityType), strings.TrimSpace(*entityID))
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, docs)
		}
		printDocumentsTable(a.stdout, docs)
		return nil

	case "upload":
		fs := flag.NewFlagSet("documents upload", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		entityType := fs.String("entity-type", "", "Entity type: invoice, journal_entry, payment, bank_transaction, asset")
		entityID := fs.String("entity-id", "", "Entity id")
		filePath := fs.String("file", "", "File path ('-' for stdin)")
		documentType := fs.String("document-type", documents.DocumentTypeSupportingDocument, "Document type")
		notes := fs.String("notes", "", "Optional notes")
		retentionUntil := fs.String("retention-until", "", "Optional retention date in YYYY-MM-DD")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*entityType) == "" || strings.TrimSpace(*entityID) == "" || strings.TrimSpace(*filePath) == "" {
			return errors.New("entity-type, entity-id, and file are required")
		}

		content, fileName, err := readFileInput(*filePath, "stdin.bin")
		if err != nil {
			return err
		}
		var retentionDate *time.Time
		if strings.TrimSpace(*retentionUntil) != "" {
			parsed, err := time.Parse("2006-01-02", strings.TrimSpace(*retentionUntil))
			if err != nil {
				return fmt.Errorf("parse retention-until: %w", err)
			}
			normalized := parsed.UTC()
			retentionDate = &normalized
		}

		doc, err := client.uploadDocument(ctx, cfg.TenantID, &documents.UploadDocumentRequest{
			EntityType:     strings.TrimSpace(*entityType),
			EntityID:       strings.TrimSpace(*entityID),
			DocumentType:   strings.TrimSpace(*documentType),
			FileName:       fileName,
			Notes:          strings.TrimSpace(*notes),
			RetentionUntil: retentionDate,
		}, content)
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, doc)
		}
		_, _ = fmt.Fprintf(a.stdout, "Uploaded %s (%s) to %s %s\n", doc.FileName, doc.ID, doc.EntityType, doc.EntityID)
		return nil

	case "mark-reviewed":
		fs := flag.NewFlagSet("documents mark-reviewed", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		documentID := fs.String("id", "", "Document id")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*documentID) == "" {
			return errors.New("id is required")
		}

		doc, err := client.markDocumentReviewed(ctx, cfg.TenantID, strings.TrimSpace(*documentID))
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, doc)
		}
		_, _ = fmt.Fprintf(a.stdout, "Marked document %s as reviewed\n", doc.ID)
		return nil

	case "delete":
		fs := flag.NewFlagSet("documents delete", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		documentID := fs.String("id", "", "Document id")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*documentID) == "" {
			return errors.New("id is required")
		}

		if err := client.deleteDocument(ctx, cfg.TenantID, strings.TrimSpace(*documentID)); err != nil {
			return err
		}
		_, _ = fmt.Fprintf(a.stdout, "Deleted document %s\n", strings.TrimSpace(*documentID))
		return nil

	default:
		return fmt.Errorf("unknown documents subcommand %q", args[0])
	}
}

func (a *cliApp) loadAuthenticatedClient() (*cliConfig, *apiClient, error) {
	cfg, err := loadRuntimeConfig()
	if err != nil {
		return nil, nil, err
	}
	if strings.TrimSpace(cfg.APIToken) == "" {
		return nil, nil, errors.New("no API token configured, run `oa auth init` first")
	}
	if strings.TrimSpace(cfg.TenantID) == "" {
		return nil, nil, errors.New("no tenant configured, run `oa auth init` first")
	}
	return cfg, newAPIClient(cfg.BaseURL, cfg.APIToken), nil
}

func resolvePassword(password string, passwordStdin bool) (string, error) {
	if strings.TrimSpace(password) != "" {
		return password, nil
	}
	if !passwordStdin {
		return "", errors.New("password is required")
	}

	reader := bufio.NewReader(os.Stdin)
	value, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", fmt.Errorf("read password from stdin: %w", err)
	}
	value = strings.TrimRight(value, "\r\n")
	if value == "" {
		return "", errors.New("password from stdin is empty")
	}
	return value, nil
}

func resolveTenantMembership(memberships []tenant.TenantMembership, selector string) (*tenant.TenantMembership, error) {
	if len(memberships) == 0 {
		return nil, errors.New("no tenant memberships found for this user")
	}

	normalizedSelector := normalizeSelector(selector)
	if normalizedSelector != "" {
		for _, membership := range memberships {
			if normalizedSelector == normalizeSelector(membership.Tenant.ID) ||
				normalizedSelector == normalizeSelector(membership.Tenant.Slug) ||
				normalizedSelector == normalizeSelector(membership.Tenant.Name) {
				match := membership
				return &match, nil
			}
		}
		return nil, fmt.Errorf("tenant %q not found in your memberships", selector)
	}

	for _, membership := range memberships {
		if membership.IsDefault {
			match := membership
			return &match, nil
		}
	}

	if len(memberships) == 1 {
		match := memberships[0]
		return &match, nil
	}

	var options []string
	for _, membership := range memberships {
		options = append(options, fmt.Sprintf("%s (%s)", membership.Tenant.Name, membership.Tenant.Slug))
	}
	return nil, fmt.Errorf("multiple tenants found; specify --tenant. Available: %s", strings.Join(options, ", "))
}

func readCSVInput(filePath string) (content string, fileName string, err error) {
	data, fileName, err := readFileInput(filePath, "stdin.csv")
	if err != nil {
		return "", "", err
	}
	return string(data), fileName, nil
}

func readFileInput(filePath string, stdinFileName string) (content []byte, fileName string, err error) {
	if filePath == "-" {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return nil, "", fmt.Errorf("read stdin: %w", err)
		}
		return data, stdinFileName, nil
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, "", fmt.Errorf("read file %s: %w", filePath, err)
	}

	return data, filepath.Base(filePath), nil
}

func isValidAccountType(value accounting.AccountType) bool {
	switch value {
	case accounting.AccountTypeAsset, accounting.AccountTypeLiability, accounting.AccountTypeEquity, accounting.AccountTypeRevenue, accounting.AccountTypeExpense:
		return true
	default:
		return false
	}
}
