package main

import (
	"bufio"
	"context"
	"encoding/csv"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"github.com/HMB-research/open-accounting/internal/accounting"
	"github.com/HMB-research/open-accounting/internal/apitoken"
	"github.com/HMB-research/open-accounting/internal/assets"
	"github.com/HMB-research/open-accounting/internal/contacts"
	"github.com/HMB-research/open-accounting/internal/documents"
	"github.com/HMB-research/open-accounting/internal/invoicing"
	"github.com/HMB-research/open-accounting/internal/orders"
	"github.com/HMB-research/open-accounting/internal/payments"
	"github.com/HMB-research/open-accounting/internal/payroll"
	"github.com/HMB-research/open-accounting/internal/quotes"
	"github.com/HMB-research/open-accounting/internal/tax"
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
	case "payroll":
		return a.runPayroll(ctx, args[1:])
	case "tsd":
		return a.runTSD(ctx, args[1:])
	case "tax":
		return a.runTax(ctx, args[1:])
	case "invoices":
		return a.runInvoices(ctx, args[1:])
	case "payments":
		return a.runPayments(ctx, args[1:])
	case "quotes":
		return a.runQuotes(ctx, args[1:])
	case "orders":
		return a.runOrders(ctx, args[1:])
	case "assets":
		return a.runAssets(ctx, args[1:])
	case "cost-centers":
		return a.runCostCenters(ctx, args[1:])
	case "reports":
		return a.runReports(ctx, args[1:])
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
	_, _ = fmt.Fprintln(a.stdout, "  accounts get              Show one account")
	_, _ = fmt.Fprintln(a.stdout, "  accounts import           Import accounts from CSV")
	_, _ = fmt.Fprintln(a.stdout, "  contacts list             List contacts")
	_, _ = fmt.Fprintln(a.stdout, "  contacts create           Create a contact")
	_, _ = fmt.Fprintln(a.stdout, "  contacts get              Show one contact")
	_, _ = fmt.Fprintln(a.stdout, "  contacts update           Update a contact")
	_, _ = fmt.Fprintln(a.stdout, "  contacts delete           Delete a contact")
	_, _ = fmt.Fprintln(a.stdout, "  contacts import           Import contacts from CSV")
	_, _ = fmt.Fprintln(a.stdout, "  employees list            List employees")
	_, _ = fmt.Fprintln(a.stdout, "  employees create          Create an employee")
	_, _ = fmt.Fprintln(a.stdout, "  employees get             Show one employee")
	_, _ = fmt.Fprintln(a.stdout, "  employees update          Update an employee")
	_, _ = fmt.Fprintln(a.stdout, "  employees set-salary      Set an employee base salary")
	_, _ = fmt.Fprintln(a.stdout, "  employees import          Import employees from CSV")
	_, _ = fmt.Fprintln(a.stdout, "  payroll runs list         List payroll runs")
	_, _ = fmt.Fprintln(a.stdout, "  payroll runs create       Create a payroll run")
	_, _ = fmt.Fprintln(a.stdout, "  payroll runs calculate    Calculate payslips for a payroll run")
	_, _ = fmt.Fprintln(a.stdout, "  payroll runs approve      Approve a payroll run")
	_, _ = fmt.Fprintln(a.stdout, "  payroll runs payslips     List payslips for a payroll run")
	_, _ = fmt.Fprintln(a.stdout, "  payroll tax-preview       Preview Estonian payroll taxes")
	_, _ = fmt.Fprintln(a.stdout, "  payroll import-history    Import historical payroll runs from CSV")
	_, _ = fmt.Fprintln(a.stdout, "  payroll import-leave-balances  Import leave balances from CSV")
	_, _ = fmt.Fprintln(a.stdout, "  tsd list                  List TSD declarations")
	_, _ = fmt.Fprintln(a.stdout, "  tsd get                   Show one TSD declaration")
	_, _ = fmt.Fprintln(a.stdout, "  tsd generate              Generate TSD from a payroll run")
	_, _ = fmt.Fprintln(a.stdout, "  tsd export-xml            Export TSD XML")
	_, _ = fmt.Fprintln(a.stdout, "  tsd export-csv            Export TSD CSV")
	_, _ = fmt.Fprintln(a.stdout, "  tax kmd list              List KMD declarations")
	_, _ = fmt.Fprintln(a.stdout, "  tax kmd generate          Generate KMD declaration")
	_, _ = fmt.Fprintln(a.stdout, "  tax kmd export-xml        Export KMD XML")
	_, _ = fmt.Fprintln(a.stdout, "  invoices list             List invoices")
	_, _ = fmt.Fprintln(a.stdout, "  invoices create           Create an invoice")
	_, _ = fmt.Fprintln(a.stdout, "  invoices get              Show one invoice")
	_, _ = fmt.Fprintln(a.stdout, "  invoices pdf              Download an invoice PDF")
	_, _ = fmt.Fprintln(a.stdout, "  invoices send             Mark an invoice sent")
	_, _ = fmt.Fprintln(a.stdout, "  invoices void             Void an invoice")
	_, _ = fmt.Fprintln(a.stdout, "  invoices import           Import invoices from CSV")
	_, _ = fmt.Fprintln(a.stdout, "  payments list             List payments")
	_, _ = fmt.Fprintln(a.stdout, "  payments create           Create a payment")
	_, _ = fmt.Fprintln(a.stdout, "  payments get              Show one payment")
	_, _ = fmt.Fprintln(a.stdout, "  payments allocate         Allocate a payment to an invoice")
	_, _ = fmt.Fprintln(a.stdout, "  payments unallocated      List unallocated payments")
	_, _ = fmt.Fprintln(a.stdout, "  quotes list               List quotes")
	_, _ = fmt.Fprintln(a.stdout, "  quotes create             Create a quote")
	_, _ = fmt.Fprintln(a.stdout, "  quotes get                Show one quote")
	_, _ = fmt.Fprintln(a.stdout, "  quotes update             Update a draft quote")
	_, _ = fmt.Fprintln(a.stdout, "  quotes delete             Delete a draft quote")
	_, _ = fmt.Fprintln(a.stdout, "  quotes send               Mark a quote sent")
	_, _ = fmt.Fprintln(a.stdout, "  quotes accept             Mark a quote accepted")
	_, _ = fmt.Fprintln(a.stdout, "  quotes reject             Mark a quote rejected")
	_, _ = fmt.Fprintln(a.stdout, "  orders list               List orders")
	_, _ = fmt.Fprintln(a.stdout, "  orders create             Create an order")
	_, _ = fmt.Fprintln(a.stdout, "  orders get                Show one order")
	_, _ = fmt.Fprintln(a.stdout, "  orders update             Update an order")
	_, _ = fmt.Fprintln(a.stdout, "  orders delete             Delete a pending order")
	_, _ = fmt.Fprintln(a.stdout, "  orders confirm            Mark an order confirmed")
	_, _ = fmt.Fprintln(a.stdout, "  orders process            Mark an order processing")
	_, _ = fmt.Fprintln(a.stdout, "  orders ship               Mark an order shipped")
	_, _ = fmt.Fprintln(a.stdout, "  orders deliver            Mark an order delivered")
	_, _ = fmt.Fprintln(a.stdout, "  orders cancel             Cancel an order")
	_, _ = fmt.Fprintln(a.stdout, "  assets categories list    List fixed asset categories")
	_, _ = fmt.Fprintln(a.stdout, "  assets categories create  Create a fixed asset category")
	_, _ = fmt.Fprintln(a.stdout, "  assets categories get     Show one fixed asset category")
	_, _ = fmt.Fprintln(a.stdout, "  assets categories delete  Delete a fixed asset category")
	_, _ = fmt.Fprintln(a.stdout, "  assets list               List fixed assets")
	_, _ = fmt.Fprintln(a.stdout, "  assets create             Create a fixed asset")
	_, _ = fmt.Fprintln(a.stdout, "  assets get                Show one fixed asset")
	_, _ = fmt.Fprintln(a.stdout, "  assets update             Update a fixed asset")
	_, _ = fmt.Fprintln(a.stdout, "  assets delete             Delete a draft fixed asset")
	_, _ = fmt.Fprintln(a.stdout, "  assets activate           Activate a fixed asset")
	_, _ = fmt.Fprintln(a.stdout, "  assets dispose            Dispose or sell a fixed asset")
	_, _ = fmt.Fprintln(a.stdout, "  assets depreciate         Record monthly depreciation")
	_, _ = fmt.Fprintln(a.stdout, "  assets depreciation       List depreciation history")
	_, _ = fmt.Fprintln(a.stdout, "  cost-centers list         List cost centers")
	_, _ = fmt.Fprintln(a.stdout, "  cost-centers create       Create a cost center")
	_, _ = fmt.Fprintln(a.stdout, "  cost-centers get          Show one cost center")
	_, _ = fmt.Fprintln(a.stdout, "  cost-centers update       Update a cost center")
	_, _ = fmt.Fprintln(a.stdout, "  cost-centers delete       Delete a cost center")
	_, _ = fmt.Fprintln(a.stdout, "  cost-centers report       Show cost center budget report")
	_, _ = fmt.Fprintln(a.stdout, "  reports trial-balance     Show trial balance")
	_, _ = fmt.Fprintln(a.stdout, "  reports account-balance   Show one account balance")
	_, _ = fmt.Fprintln(a.stdout, "  reports balance-sheet     Show balance sheet")
	_, _ = fmt.Fprintln(a.stdout, "  reports income-statement  Show income statement")
	_, _ = fmt.Fprintln(a.stdout, "  reports cash-flow         Show cash flow statement")
	_, _ = fmt.Fprintln(a.stdout, "  reports aging             Show receivables or payables aging")
	_, _ = fmt.Fprintln(a.stdout, "  reports balance-confirmations  Show balance confirmations")
	_, _ = fmt.Fprintln(a.stdout, "  documents list            List documents for a record")
	_, _ = fmt.Fprintln(a.stdout, "  documents upload          Upload a document to a record")
	_, _ = fmt.Fprintln(a.stdout, "  documents mark-reviewed   Mark a document as reviewed")
	_, _ = fmt.Fprintln(a.stdout, "  documents delete          Delete a document")
	_, _ = fmt.Fprintln(a.stdout, "  journal list              List journal entries")
	_, _ = fmt.Fprintln(a.stdout, "  journal create            Create a journal entry")
	_, _ = fmt.Fprintln(a.stdout, "  journal get               Show one journal entry")
	_, _ = fmt.Fprintln(a.stdout, "  journal post              Post a journal entry")
	_, _ = fmt.Fprintln(a.stdout, "  journal void              Void a journal entry")
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

	case "get":
		fs := flag.NewFlagSet("accounts get", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		accountID := fs.String("id", "", "Account id")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*accountID) == "" {
			return errors.New("id is required")
		}

		account, err := client.getAccount(ctx, cfg.TenantID, strings.TrimSpace(*accountID))
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, account)
		}
		printAccount(a.stdout, account)
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
		addressLine1 := fs.String("address-line1", "", "Address line 1")
		addressLine2 := fs.String("address-line2", "", "Address line 2")
		city := fs.String("city", "", "City")
		postalCode := fs.String("postal-code", "", "Postal code")
		countryCode := fs.String("country-code", "EE", "Country code")
		paymentTermsDays := fs.Int("payment-terms-days", 14, "Payment terms in days")
		creditLimit := fs.String("credit-limit", "", "Credit limit")
		defaultAccountID := fs.String("default-account-id", "", "Default account id")
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
			AddressLine1:     strings.TrimSpace(*addressLine1),
			AddressLine2:     strings.TrimSpace(*addressLine2),
			City:             strings.TrimSpace(*city),
			PostalCode:       strings.TrimSpace(*postalCode),
			CountryCode:      strings.ToUpper(strings.TrimSpace(*countryCode)),
			PaymentTermsDays: *paymentTermsDays,
			CreditLimit:      creditLimitValue,
			DefaultAccountID: optionalStringPtr(*defaultAccountID),
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

	case "get":
		fs := flag.NewFlagSet("contacts get", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		contactID := fs.String("id", "", "Contact id")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*contactID) == "" {
			return errors.New("id is required")
		}

		contact, err := client.getContact(ctx, cfg.TenantID, strings.TrimSpace(*contactID))
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, contact)
		}
		printContact(a.stdout, contact)
		return nil

	case "update":
		fs := flag.NewFlagSet("contacts update", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		contactID := fs.String("id", "", "Contact id")
		name := fs.String("name", "", "Contact name")
		email := fs.String("email", "", "Email")
		phone := fs.String("phone", "", "Phone")
		regCode := fs.String("reg-code", "", "Registration code")
		vatNumber := fs.String("vat-number", "", "VAT number")
		addressLine1 := fs.String("address-line1", "", "Address line 1")
		addressLine2 := fs.String("address-line2", "", "Address line 2")
		city := fs.String("city", "", "City")
		postalCode := fs.String("postal-code", "", "Postal code")
		countryCode := fs.String("country-code", "", "Country code")
		paymentTermsDays := fs.String("payment-terms-days", "", "Payment terms in days")
		creditLimit := fs.String("credit-limit", "", "Credit limit")
		defaultAccountID := fs.String("default-account-id", "", "Default account id")
		notes := fs.String("notes", "", "Notes")
		active := fs.String("active", "", "Set active state: true or false")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*contactID) == "" {
			return errors.New("id is required")
		}

		req := &contacts.UpdateContactRequest{
			Name:             optionalStringPtr(*name),
			RegCode:          optionalStringPtr(*regCode),
			VATNumber:        optionalStringPtr(*vatNumber),
			Email:            optionalStringPtr(*email),
			Phone:            optionalStringPtr(*phone),
			AddressLine1:     optionalStringPtr(*addressLine1),
			AddressLine2:     optionalStringPtr(*addressLine2),
			City:             optionalStringPtr(*city),
			PostalCode:       optionalStringPtr(*postalCode),
			CountryCode:      optionalUpperStringPtr(*countryCode),
			DefaultAccountID: optionalStringPtr(*defaultAccountID),
			Notes:            optionalStringPtr(*notes),
		}
		if strings.TrimSpace(*paymentTermsDays) != "" {
			parsed, err := parseRequiredNonNegativeInt("payment-terms-days", *paymentTermsDays)
			if err != nil {
				return err
			}
			req.PaymentTermsDays = &parsed
		}
		if strings.TrimSpace(*creditLimit) != "" {
			parsed, err := decimal.NewFromString(strings.TrimSpace(*creditLimit))
			if err != nil {
				return fmt.Errorf("parse credit limit: %w", err)
			}
			req.CreditLimit = &parsed
		}
		if strings.TrimSpace(*active) != "" {
			parsed, err := strconv.ParseBool(strings.TrimSpace(*active))
			if err != nil {
				return fmt.Errorf("parse active: %w", err)
			}
			req.IsActive = &parsed
		}

		contact, err := client.updateContact(ctx, cfg.TenantID, strings.TrimSpace(*contactID), req)
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, contact)
		}
		printContact(a.stdout, contact)
		return nil

	case "delete":
		fs := flag.NewFlagSet("contacts delete", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		contactID := fs.String("id", "", "Contact id")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*contactID) == "" {
			return errors.New("id is required")
		}

		result, err := client.deleteContact(ctx, cfg.TenantID, strings.TrimSpace(*contactID))
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, result)
		}
		_, _ = fmt.Fprintf(a.stdout, "Deleted contact %s\n", strings.TrimSpace(*contactID))
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
	case "list":
		fs := flag.NewFlagSet("invoices list", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		invoiceTypeFlag := fs.String("type", "", "Invoice type: SALES, PURCHASE, or CREDIT_NOTE")
		statusFlag := fs.String("status", "", "Invoice status")
		contactID := fs.String("contact-id", "", "Contact id")
		fromDate := fs.String("from", "", "From issue date in YYYY-MM-DD")
		toDate := fs.String("to", "", "To issue date in YYYY-MM-DD")
		search := fs.String("search", "", "Search term")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}

		invoiceType, err := parseOptionalInvoiceType(*invoiceTypeFlag)
		if err != nil {
			return err
		}
		status, err := parseOptionalInvoiceStatus(*statusFlag)
		if err != nil {
			return err
		}
		fromDateValue, err := parseOptionalDate("from", *fromDate)
		if err != nil {
			return err
		}
		toDateValue, err := parseOptionalDate("to", *toDate)
		if err != nil {
			return err
		}

		invoices, err := client.listInvoices(ctx, cfg.TenantID, invoicing.InvoiceFilter{
			InvoiceType: invoiceType,
			Status:      status,
			ContactID:   strings.TrimSpace(*contactID),
			FromDate:    fromDateValue,
			ToDate:      toDateValue,
			Search:      strings.TrimSpace(*search),
		})
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, invoices)
		}
		printInvoicesTable(a.stdout, invoices)
		return nil

	case "create":
		fs := flag.NewFlagSet("invoices create", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		invoiceTypeFlag := fs.String("type", "", "Invoice type: SALES, PURCHASE, or CREDIT_NOTE")
		contactID := fs.String("contact-id", "", "Contact id")
		issueDate := fs.String("issue-date", "", "Issue date in YYYY-MM-DD")
		dueDate := fs.String("due-date", "", "Due date in YYYY-MM-DD")
		currency := fs.String("currency", "EUR", "Currency code")
		exchangeRateFlag := fs.String("exchange-rate", "1", "Exchange rate to base currency")
		reference := fs.String("reference", "", "Reference")
		notes := fs.String("notes", "", "Notes")
		lines := invoiceLineFlags{}
		fs.Var(&lines, "line", "Line as comma-separated key=value pairs; repeatable")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}

		invoiceType, err := parseRequiredInvoiceType(*invoiceTypeFlag)
		if err != nil {
			return err
		}
		if strings.TrimSpace(*contactID) == "" {
			return errors.New("contact-id is required")
		}
		issueDateValue, err := parseRequiredDate("issue-date", *issueDate)
		if err != nil {
			return err
		}
		dueDateValue, err := parseRequiredDate("due-date", *dueDate)
		if err != nil {
			return err
		}
		if len(lines) == 0 {
			return errors.New("at least one line is required")
		}
		exchangeRate, err := parseRequiredPositiveDecimal("exchange-rate", *exchangeRateFlag)
		if err != nil {
			return err
		}

		invoice, err := client.createInvoice(ctx, cfg.TenantID, &invoicing.CreateInvoiceRequest{
			InvoiceType:  invoiceType,
			ContactID:    strings.TrimSpace(*contactID),
			IssueDate:    issueDateValue,
			DueDate:      dueDateValue,
			Currency:     strings.ToUpper(strings.TrimSpace(*currency)),
			ExchangeRate: exchangeRate,
			Reference:    strings.TrimSpace(*reference),
			Notes:        strings.TrimSpace(*notes),
			Lines:        []invoicing.CreateInvoiceLineRequest(lines),
		})
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, invoice)
		}
		_, _ = fmt.Fprintf(a.stdout, "Created invoice %s (%s)\n", invoice.InvoiceNumber, invoice.ID)
		return nil

	case "get":
		fs := flag.NewFlagSet("invoices get", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		invoiceID := fs.String("id", "", "Invoice id")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*invoiceID) == "" {
			return errors.New("id is required")
		}

		invoice, err := client.getInvoice(ctx, cfg.TenantID, strings.TrimSpace(*invoiceID))
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, invoice)
		}
		printInvoice(a.stdout, invoice)
		return nil

	case "pdf":
		fs := flag.NewFlagSet("invoices pdf", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		invoiceID := fs.String("id", "", "Invoice id")
		outputPath := fs.String("output", "", "Optional output file path")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*invoiceID) == "" {
			return errors.New("id is required")
		}

		content, err := client.downloadInvoicePDF(ctx, cfg.TenantID, strings.TrimSpace(*invoiceID))
		if err != nil {
			return err
		}
		return writeExportOutput(a.stdout, strings.TrimSpace(*outputPath), content, "Invoice PDF")

	case "send":
		fs := flag.NewFlagSet("invoices send", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		invoiceID := fs.String("id", "", "Invoice id")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*invoiceID) == "" {
			return errors.New("id is required")
		}

		result, err := client.sendInvoice(ctx, cfg.TenantID, strings.TrimSpace(*invoiceID))
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, result)
		}
		_, _ = fmt.Fprintf(a.stdout, "Sent invoice %s\n", strings.TrimSpace(*invoiceID))
		return nil

	case "void":
		fs := flag.NewFlagSet("invoices void", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		invoiceID := fs.String("id", "", "Invoice id")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*invoiceID) == "" {
			return errors.New("id is required")
		}

		result, err := client.voidInvoice(ctx, cfg.TenantID, strings.TrimSpace(*invoiceID))
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, result)
		}
		_, _ = fmt.Fprintf(a.stdout, "Voided invoice %s\n", strings.TrimSpace(*invoiceID))
		return nil

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

func (a *cliApp) runPayments(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("payments subcommand required")
	}
	cfg, client, err := a.loadAuthenticatedClient()
	if err != nil {
		return err
	}

	switch args[0] {
	case "list":
		fs := flag.NewFlagSet("payments list", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		paymentTypeFlag := fs.String("type", "", "Payment type: RECEIVED or MADE")
		method := fs.String("method", "", "Payment method")
		contactID := fs.String("contact-id", "", "Contact id")
		fromDate := fs.String("from", "", "From date in YYYY-MM-DD")
		toDate := fs.String("to", "", "To date in YYYY-MM-DD")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}

		paymentType, err := parseOptionalPaymentType(*paymentTypeFlag)
		if err != nil {
			return err
		}
		fromDateValue, err := parseOptionalDate("from", *fromDate)
		if err != nil {
			return err
		}
		toDateValue, err := parseOptionalDate("to", *toDate)
		if err != nil {
			return err
		}

		paymentsList, err := client.listPayments(ctx, cfg.TenantID, payments.PaymentFilter{
			PaymentType:   paymentType,
			PaymentMethod: strings.TrimSpace(*method),
			ContactID:     strings.TrimSpace(*contactID),
			FromDate:      fromDateValue,
			ToDate:        toDateValue,
		})
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, paymentsList)
		}
		printPaymentsTable(a.stdout, paymentsList)
		return nil

	case "create":
		fs := flag.NewFlagSet("payments create", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		paymentTypeFlag := fs.String("type", "", "Payment type: RECEIVED or MADE")
		contactID := fs.String("contact-id", "", "Contact id")
		paymentDate := fs.String("date", "", "Payment date in YYYY-MM-DD")
		amountFlag := fs.String("amount", "", "Payment amount")
		currency := fs.String("currency", "EUR", "Currency code")
		exchangeRateFlag := fs.String("exchange-rate", "1", "Exchange rate to base currency")
		method := fs.String("method", "", "Payment method")
		bankAccount := fs.String("bank-account", "", "Bank account")
		reference := fs.String("reference", "", "Reference")
		notes := fs.String("notes", "", "Notes")
		allocations := allocationFlags{}
		fs.Var(&allocations, "allocate", "Allocation in invoice-id:amount form; repeatable")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}

		paymentType, err := parseRequiredPaymentType(*paymentTypeFlag)
		if err != nil {
			return err
		}
		amount, err := parseRequiredPositiveDecimal("amount", *amountFlag)
		if err != nil {
			return err
		}
		paymentDateValue := time.Time{}
		if strings.TrimSpace(*paymentDate) != "" {
			parsed, err := time.Parse("2006-01-02", strings.TrimSpace(*paymentDate))
			if err != nil {
				return fmt.Errorf("parse date: %w", err)
			}
			paymentDateValue = parsed
		}
		exchangeRate, err := parseRequiredPositiveDecimal("exchange-rate", *exchangeRateFlag)
		if err != nil {
			return err
		}

		payment, err := client.createPayment(ctx, cfg.TenantID, &payments.CreatePaymentRequest{
			PaymentType:   paymentType,
			ContactID:     optionalStringPtr(*contactID),
			PaymentDate:   paymentDateValue,
			Amount:        amount,
			Currency:      strings.ToUpper(strings.TrimSpace(*currency)),
			ExchangeRate:  exchangeRate,
			PaymentMethod: strings.TrimSpace(*method),
			BankAccount:   strings.TrimSpace(*bankAccount),
			Reference:     strings.TrimSpace(*reference),
			Notes:         strings.TrimSpace(*notes),
			Allocations:   []payments.AllocationRequest(allocations),
		})
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, payment)
		}
		_, _ = fmt.Fprintf(a.stdout, "Created payment %s (%s)\n", payment.PaymentNumber, payment.ID)
		return nil

	case "get":
		fs := flag.NewFlagSet("payments get", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		paymentID := fs.String("id", "", "Payment id")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*paymentID) == "" {
			return errors.New("id is required")
		}

		payment, err := client.getPayment(ctx, cfg.TenantID, strings.TrimSpace(*paymentID))
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, payment)
		}
		printPayment(a.stdout, payment)
		return nil

	case "allocate":
		fs := flag.NewFlagSet("payments allocate", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		paymentID := fs.String("id", "", "Payment id")
		invoiceID := fs.String("invoice-id", "", "Invoice id")
		amountFlag := fs.String("amount", "", "Allocation amount")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*paymentID) == "" {
			return errors.New("id is required")
		}
		if strings.TrimSpace(*invoiceID) == "" {
			return errors.New("invoice-id is required")
		}
		amount, err := parseRequiredPositiveDecimal("amount", *amountFlag)
		if err != nil {
			return err
		}

		result, err := client.allocatePayment(ctx, cfg.TenantID, strings.TrimSpace(*paymentID), &payments.AllocationRequest{
			InvoiceID: strings.TrimSpace(*invoiceID),
			Amount:    amount,
		})
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, result)
		}
		_, _ = fmt.Fprintf(a.stdout, "Allocated %s to invoice %s for payment %s\n", amount.String(), strings.TrimSpace(*invoiceID), strings.TrimSpace(*paymentID))
		return nil

	case "unallocated":
		fs := flag.NewFlagSet("payments unallocated", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		paymentTypeFlag := fs.String("type", "RECEIVED", "Payment type: RECEIVED or MADE")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		paymentType, err := parseRequiredPaymentType(*paymentTypeFlag)
		if err != nil {
			return err
		}

		paymentsList, err := client.listUnallocatedPayments(ctx, cfg.TenantID, paymentType)
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, paymentsList)
		}
		printPaymentsTable(a.stdout, paymentsList)
		return nil

	default:
		return fmt.Errorf("unknown payments subcommand %q", args[0])
	}
}

func (a *cliApp) runQuotes(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("quotes subcommand required")
	}
	cfg, client, err := a.loadAuthenticatedClient()
	if err != nil {
		return err
	}

	switch args[0] {
	case "list":
		fs := flag.NewFlagSet("quotes list", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		statusFlag := fs.String("status", "", "Quote status")
		contactID := fs.String("contact-id", "", "Contact id")
		fromDate := fs.String("from", "", "From quote date in YYYY-MM-DD")
		toDate := fs.String("to", "", "To quote date in YYYY-MM-DD")
		search := fs.String("search", "", "Search term")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}

		status, err := parseOptionalQuoteStatus(*statusFlag)
		if err != nil {
			return err
		}
		fromDateValue, err := parseOptionalDate("from", *fromDate)
		if err != nil {
			return err
		}
		toDateValue, err := parseOptionalDate("to", *toDate)
		if err != nil {
			return err
		}

		quotesList, err := client.listQuotes(ctx, cfg.TenantID, quotes.QuoteFilter{
			Status:    status,
			ContactID: strings.TrimSpace(*contactID),
			FromDate:  fromDateValue,
			ToDate:    toDateValue,
			Search:    strings.TrimSpace(*search),
		})
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, quotesList)
		}
		printQuotesTable(a.stdout, quotesList)
		return nil

	case "create":
		fs := flag.NewFlagSet("quotes create", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		contactID := fs.String("contact-id", "", "Contact id")
		quoteDate := fs.String("quote-date", "", "Quote date in YYYY-MM-DD")
		validUntil := fs.String("valid-until", "", "Valid until date in YYYY-MM-DD")
		currency := fs.String("currency", "EUR", "Currency code")
		exchangeRateFlag := fs.String("exchange-rate", "1", "Exchange rate to base currency")
		notes := fs.String("notes", "", "Notes")
		lines := quoteLineFlags{}
		fs.Var(&lines, "line", "Line as comma-separated key=value pairs; repeatable")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*contactID) == "" {
			return errors.New("contact-id is required")
		}
		quoteDateValue, err := parseRequiredDate("quote-date", *quoteDate)
		if err != nil {
			return err
		}
		validUntilValue, err := parseOptionalDate("valid-until", *validUntil)
		if err != nil {
			return err
		}
		if len(lines) == 0 {
			return errors.New("at least one line is required")
		}
		exchangeRate, err := parseRequiredPositiveDecimal("exchange-rate", *exchangeRateFlag)
		if err != nil {
			return err
		}

		quote, err := client.createQuote(ctx, cfg.TenantID, &quotes.CreateQuoteRequest{
			ContactID:    strings.TrimSpace(*contactID),
			QuoteDate:    quoteDateValue,
			ValidUntil:   validUntilValue,
			Currency:     strings.ToUpper(strings.TrimSpace(*currency)),
			ExchangeRate: exchangeRate,
			Notes:        strings.TrimSpace(*notes),
			Lines:        []quotes.CreateQuoteLineRequest(lines),
		})
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, quote)
		}
		_, _ = fmt.Fprintf(a.stdout, "Created quote %s (%s)\n", quote.QuoteNumber, quote.ID)
		return nil

	case "get":
		fs := flag.NewFlagSet("quotes get", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		quoteID := fs.String("id", "", "Quote id")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*quoteID) == "" {
			return errors.New("id is required")
		}

		quote, err := client.getQuote(ctx, cfg.TenantID, strings.TrimSpace(*quoteID))
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, quote)
		}
		printQuote(a.stdout, quote)
		return nil

	case "update":
		fs := flag.NewFlagSet("quotes update", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		quoteID := fs.String("id", "", "Quote id")
		contactID := fs.String("contact-id", "", "Contact id")
		quoteDate := fs.String("quote-date", "", "Quote date in YYYY-MM-DD")
		validUntil := fs.String("valid-until", "", "Valid until date in YYYY-MM-DD")
		currency := fs.String("currency", "EUR", "Currency code")
		exchangeRateFlag := fs.String("exchange-rate", "1", "Exchange rate to base currency")
		notes := fs.String("notes", "", "Notes")
		lines := quoteLineFlags{}
		fs.Var(&lines, "line", "Line as comma-separated key=value pairs; repeatable")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*quoteID) == "" {
			return errors.New("id is required")
		}
		if strings.TrimSpace(*contactID) == "" {
			return errors.New("contact-id is required")
		}
		quoteDateValue, err := parseRequiredDate("quote-date", *quoteDate)
		if err != nil {
			return err
		}
		validUntilValue, err := parseOptionalDate("valid-until", *validUntil)
		if err != nil {
			return err
		}
		if len(lines) == 0 {
			return errors.New("at least one line is required")
		}
		exchangeRate, err := parseRequiredPositiveDecimal("exchange-rate", *exchangeRateFlag)
		if err != nil {
			return err
		}

		quote, err := client.updateQuote(ctx, cfg.TenantID, strings.TrimSpace(*quoteID), &quotes.UpdateQuoteRequest{
			ContactID:    strings.TrimSpace(*contactID),
			QuoteDate:    quoteDateValue,
			ValidUntil:   validUntilValue,
			Currency:     strings.ToUpper(strings.TrimSpace(*currency)),
			ExchangeRate: exchangeRate,
			Notes:        strings.TrimSpace(*notes),
			Lines:        []quotes.CreateQuoteLineRequest(lines),
		})
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, quote)
		}
		printQuote(a.stdout, quote)
		return nil

	case "delete":
		fs := flag.NewFlagSet("quotes delete", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		quoteID := fs.String("id", "", "Quote id")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*quoteID) == "" {
			return errors.New("id is required")
		}

		if err := client.deleteQuote(ctx, cfg.TenantID, strings.TrimSpace(*quoteID)); err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, map[string]string{"status": "deleted"})
		}
		_, _ = fmt.Fprintf(a.stdout, "Deleted quote %s\n", strings.TrimSpace(*quoteID))
		return nil

	case "send", "accept", "reject":
		fs := flag.NewFlagSet("quotes "+args[0], flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		quoteID := fs.String("id", "", "Quote id")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*quoteID) == "" {
			return errors.New("id is required")
		}

		result, err := client.updateQuoteStatus(ctx, cfg.TenantID, strings.TrimSpace(*quoteID), args[0])
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, result)
		}
		_, _ = fmt.Fprintf(a.stdout, "%s quote %s\n", quoteActionPastTense(args[0]), strings.TrimSpace(*quoteID))
		return nil

	default:
		return fmt.Errorf("unknown quotes subcommand %q", args[0])
	}
}

func (a *cliApp) runOrders(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("orders subcommand required")
	}
	cfg, client, err := a.loadAuthenticatedClient()
	if err != nil {
		return err
	}

	switch args[0] {
	case "list":
		fs := flag.NewFlagSet("orders list", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		statusFlag := fs.String("status", "", "Order status")
		contactID := fs.String("contact-id", "", "Contact id")
		fromDate := fs.String("from", "", "From order date in YYYY-MM-DD")
		toDate := fs.String("to", "", "To order date in YYYY-MM-DD")
		search := fs.String("search", "", "Search term")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}

		status, err := parseOptionalOrderStatus(*statusFlag)
		if err != nil {
			return err
		}
		fromDateValue, err := parseOptionalDate("from", *fromDate)
		if err != nil {
			return err
		}
		toDateValue, err := parseOptionalDate("to", *toDate)
		if err != nil {
			return err
		}

		ordersList, err := client.listOrders(ctx, cfg.TenantID, orders.OrderFilter{
			Status:    status,
			ContactID: strings.TrimSpace(*contactID),
			FromDate:  fromDateValue,
			ToDate:    toDateValue,
			Search:    strings.TrimSpace(*search),
		})
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, ordersList)
		}
		printOrdersTable(a.stdout, ordersList)
		return nil

	case "create":
		fs := flag.NewFlagSet("orders create", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		contactID := fs.String("contact-id", "", "Contact id")
		orderDate := fs.String("order-date", "", "Order date in YYYY-MM-DD")
		expectedDelivery := fs.String("expected-delivery", "", "Expected delivery date in YYYY-MM-DD")
		currency := fs.String("currency", "EUR", "Currency code")
		exchangeRateFlag := fs.String("exchange-rate", "1", "Exchange rate to base currency")
		notes := fs.String("notes", "", "Notes")
		quoteID := fs.String("quote-id", "", "Source quote id")
		lines := orderLineFlags{}
		fs.Var(&lines, "line", "Line as comma-separated key=value pairs; repeatable")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*contactID) == "" {
			return errors.New("contact-id is required")
		}
		orderDateValue, err := parseRequiredDate("order-date", *orderDate)
		if err != nil {
			return err
		}
		expectedDeliveryValue, err := parseOptionalDate("expected-delivery", *expectedDelivery)
		if err != nil {
			return err
		}
		if len(lines) == 0 {
			return errors.New("at least one line is required")
		}
		exchangeRate, err := parseRequiredPositiveDecimal("exchange-rate", *exchangeRateFlag)
		if err != nil {
			return err
		}

		order, err := client.createOrder(ctx, cfg.TenantID, &orders.CreateOrderRequest{
			ContactID:        strings.TrimSpace(*contactID),
			OrderDate:        orderDateValue,
			ExpectedDelivery: expectedDeliveryValue,
			Currency:         strings.ToUpper(strings.TrimSpace(*currency)),
			ExchangeRate:     exchangeRate,
			Notes:            strings.TrimSpace(*notes),
			QuoteID:          optionalStringPtr(*quoteID),
			Lines:            []orders.CreateOrderLineRequest(lines),
		})
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, order)
		}
		_, _ = fmt.Fprintf(a.stdout, "Created order %s (%s)\n", order.OrderNumber, order.ID)
		return nil

	case "get":
		fs := flag.NewFlagSet("orders get", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		orderID := fs.String("id", "", "Order id")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*orderID) == "" {
			return errors.New("id is required")
		}

		order, err := client.getOrder(ctx, cfg.TenantID, strings.TrimSpace(*orderID))
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, order)
		}
		printOrder(a.stdout, order)
		return nil

	case "update":
		fs := flag.NewFlagSet("orders update", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		orderID := fs.String("id", "", "Order id")
		contactID := fs.String("contact-id", "", "Contact id")
		orderDate := fs.String("order-date", "", "Order date in YYYY-MM-DD")
		expectedDelivery := fs.String("expected-delivery", "", "Expected delivery date in YYYY-MM-DD")
		currency := fs.String("currency", "EUR", "Currency code")
		exchangeRateFlag := fs.String("exchange-rate", "1", "Exchange rate to base currency")
		notes := fs.String("notes", "", "Notes")
		lines := orderLineFlags{}
		fs.Var(&lines, "line", "Line as comma-separated key=value pairs; repeatable")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*orderID) == "" {
			return errors.New("id is required")
		}
		if strings.TrimSpace(*contactID) == "" {
			return errors.New("contact-id is required")
		}
		orderDateValue, err := parseRequiredDate("order-date", *orderDate)
		if err != nil {
			return err
		}
		expectedDeliveryValue, err := parseOptionalDate("expected-delivery", *expectedDelivery)
		if err != nil {
			return err
		}
		if len(lines) == 0 {
			return errors.New("at least one line is required")
		}
		exchangeRate, err := parseRequiredPositiveDecimal("exchange-rate", *exchangeRateFlag)
		if err != nil {
			return err
		}

		order, err := client.updateOrder(ctx, cfg.TenantID, strings.TrimSpace(*orderID), &orders.UpdateOrderRequest{
			ContactID:        strings.TrimSpace(*contactID),
			OrderDate:        orderDateValue,
			ExpectedDelivery: expectedDeliveryValue,
			Currency:         strings.ToUpper(strings.TrimSpace(*currency)),
			ExchangeRate:     exchangeRate,
			Notes:            strings.TrimSpace(*notes),
			Lines:            []orders.CreateOrderLineRequest(lines),
		})
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, order)
		}
		printOrder(a.stdout, order)
		return nil

	case "delete":
		fs := flag.NewFlagSet("orders delete", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		orderID := fs.String("id", "", "Order id")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*orderID) == "" {
			return errors.New("id is required")
		}

		if err := client.deleteOrder(ctx, cfg.TenantID, strings.TrimSpace(*orderID)); err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, map[string]string{"status": "deleted"})
		}
		_, _ = fmt.Fprintf(a.stdout, "Deleted order %s\n", strings.TrimSpace(*orderID))
		return nil

	case "confirm", "process", "ship", "deliver", "cancel":
		fs := flag.NewFlagSet("orders "+args[0], flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		orderID := fs.String("id", "", "Order id")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*orderID) == "" {
			return errors.New("id is required")
		}

		result, err := client.updateOrderStatus(ctx, cfg.TenantID, strings.TrimSpace(*orderID), args[0])
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, result)
		}
		_, _ = fmt.Fprintf(a.stdout, "%s order %s\n", orderActionPastTense(args[0]), strings.TrimSpace(*orderID))
		return nil

	default:
		return fmt.Errorf("unknown orders subcommand %q", args[0])
	}
}

func (a *cliApp) runAssets(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("assets subcommand required")
	}
	cfg, client, err := a.loadAuthenticatedClient()
	if err != nil {
		return err
	}
	if args[0] == "categories" {
		return a.runAssetCategories(ctx, cfg, client, args[1:])
	}

	switch args[0] {
	case "list":
		fs := flag.NewFlagSet("assets list", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		statusFlag := fs.String("status", "", "Asset status")
		categoryID := fs.String("category-id", "", "Category id")
		search := fs.String("search", "", "Search term")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		status, err := parseOptionalAssetStatus(*statusFlag)
		if err != nil {
			return err
		}

		assetList, err := client.listAssets(ctx, cfg.TenantID, assets.AssetFilter{
			Status:     status,
			CategoryID: strings.TrimSpace(*categoryID),
			Search:     strings.TrimSpace(*search),
		})
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, assetList)
		}
		printAssetsTable(a.stdout, assetList)
		return nil

	case "create":
		fs := flag.NewFlagSet("assets create", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		name := fs.String("name", "", "Asset name")
		description := fs.String("description", "", "Description")
		categoryID := fs.String("category-id", "", "Category id")
		purchaseDate := fs.String("purchase-date", "", "Purchase date in YYYY-MM-DD")
		purchaseCostFlag := fs.String("purchase-cost", "", "Purchase cost")
		supplierID := fs.String("supplier-id", "", "Supplier id")
		serialNumber := fs.String("serial-number", "", "Serial number")
		location := fs.String("location", "", "Location")
		depreciationMethodFlag := fs.String("depreciation-method", "STRAIGHT_LINE", "Depreciation method")
		usefulLifeMonthsFlag := fs.String("useful-life-months", "60", "Useful life in months")
		residualValueFlag := fs.String("residual-value", "0", "Residual value")
		depreciationStartDate := fs.String("depreciation-start-date", "", "Depreciation start date in YYYY-MM-DD")
		assetAccountID := fs.String("asset-account-id", "", "Asset account id")
		depreciationExpenseAccountID := fs.String("depreciation-expense-account-id", "", "Depreciation expense account id")
		accumulatedDepreciationAccountID := fs.String("accumulated-depreciation-account-id", "", "Accumulated depreciation account id")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*name) == "" {
			return errors.New("name is required")
		}
		purchaseDateValue, err := parseRequiredDate("purchase-date", *purchaseDate)
		if err != nil {
			return err
		}
		purchaseCost, err := parseRequiredPositiveDecimal("purchase-cost", *purchaseCostFlag)
		if err != nil {
			return err
		}
		depreciationMethod, err := parseOptionalDepreciationMethod(*depreciationMethodFlag)
		if err != nil {
			return err
		}
		usefulLifeMonths, err := parseRequiredPositiveInt("useful-life-months", *usefulLifeMonthsFlag)
		if err != nil {
			return err
		}
		residualValue, err := parseRequiredNonNegativeDecimal("residual-value", *residualValueFlag)
		if err != nil {
			return err
		}
		depreciationStartDateValue, err := parseOptionalDate("depreciation-start-date", *depreciationStartDate)
		if err != nil {
			return err
		}

		asset, err := client.createAsset(ctx, cfg.TenantID, &assets.CreateAssetRequest{
			Name:                          strings.TrimSpace(*name),
			Description:                   strings.TrimSpace(*description),
			CategoryID:                    optionalStringPtr(*categoryID),
			PurchaseDate:                  purchaseDateValue,
			PurchaseCost:                  purchaseCost,
			SupplierID:                    optionalStringPtr(*supplierID),
			SerialNumber:                  strings.TrimSpace(*serialNumber),
			Location:                      strings.TrimSpace(*location),
			DepreciationMethod:            depreciationMethod,
			UsefulLifeMonths:              usefulLifeMonths,
			ResidualValue:                 residualValue,
			DepreciationStartDate:         depreciationStartDateValue,
			AssetAccountID:                optionalStringPtr(*assetAccountID),
			DepreciationExpenseAccountID:  optionalStringPtr(*depreciationExpenseAccountID),
			AccumulatedDepreciationAcctID: optionalStringPtr(*accumulatedDepreciationAccountID),
		})
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, asset)
		}
		_, _ = fmt.Fprintf(a.stdout, "Created asset %s (%s)\n", asset.AssetNumber, asset.ID)
		return nil

	case "get":
		fs := flag.NewFlagSet("assets get", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		assetID := fs.String("id", "", "Asset id")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*assetID) == "" {
			return errors.New("id is required")
		}

		asset, err := client.getAsset(ctx, cfg.TenantID, strings.TrimSpace(*assetID))
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, asset)
		}
		printAsset(a.stdout, asset)
		return nil

	case "update":
		fs := flag.NewFlagSet("assets update", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		assetID := fs.String("id", "", "Asset id")
		name := fs.String("name", "", "Asset name")
		description := fs.String("description", "", "Description")
		categoryID := fs.String("category-id", "", "Category id")
		serialNumber := fs.String("serial-number", "", "Serial number")
		location := fs.String("location", "", "Location")
		depreciationMethodFlag := fs.String("depreciation-method", "STRAIGHT_LINE", "Depreciation method")
		usefulLifeMonthsFlag := fs.String("useful-life-months", "60", "Useful life in months")
		residualValueFlag := fs.String("residual-value", "0", "Residual value")
		assetAccountID := fs.String("asset-account-id", "", "Asset account id")
		depreciationExpenseAccountID := fs.String("depreciation-expense-account-id", "", "Depreciation expense account id")
		accumulatedDepreciationAccountID := fs.String("accumulated-depreciation-account-id", "", "Accumulated depreciation account id")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*assetID) == "" {
			return errors.New("id is required")
		}
		if strings.TrimSpace(*name) == "" {
			return errors.New("name is required")
		}
		depreciationMethod, err := parseOptionalDepreciationMethod(*depreciationMethodFlag)
		if err != nil {
			return err
		}
		usefulLifeMonths, err := parseRequiredPositiveInt("useful-life-months", *usefulLifeMonthsFlag)
		if err != nil {
			return err
		}
		residualValue, err := parseRequiredNonNegativeDecimal("residual-value", *residualValueFlag)
		if err != nil {
			return err
		}

		asset, err := client.updateAsset(ctx, cfg.TenantID, strings.TrimSpace(*assetID), &assets.UpdateAssetRequest{
			Name:                          strings.TrimSpace(*name),
			Description:                   strings.TrimSpace(*description),
			CategoryID:                    optionalStringPtr(*categoryID),
			SerialNumber:                  strings.TrimSpace(*serialNumber),
			Location:                      strings.TrimSpace(*location),
			DepreciationMethod:            depreciationMethod,
			UsefulLifeMonths:              usefulLifeMonths,
			ResidualValue:                 residualValue,
			AssetAccountID:                optionalStringPtr(*assetAccountID),
			DepreciationExpenseAccountID:  optionalStringPtr(*depreciationExpenseAccountID),
			AccumulatedDepreciationAcctID: optionalStringPtr(*accumulatedDepreciationAccountID),
		})
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, asset)
		}
		printAsset(a.stdout, asset)
		return nil

	case "delete":
		fs := flag.NewFlagSet("assets delete", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		assetID := fs.String("id", "", "Asset id")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*assetID) == "" {
			return errors.New("id is required")
		}

		if err := client.deleteAsset(ctx, cfg.TenantID, strings.TrimSpace(*assetID)); err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, map[string]string{"status": "deleted"})
		}
		_, _ = fmt.Fprintf(a.stdout, "Deleted asset %s\n", strings.TrimSpace(*assetID))
		return nil

	case "activate":
		fs := flag.NewFlagSet("assets activate", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		assetID := fs.String("id", "", "Asset id")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*assetID) == "" {
			return errors.New("id is required")
		}

		result, err := client.activateAsset(ctx, cfg.TenantID, strings.TrimSpace(*assetID))
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, result)
		}
		_, _ = fmt.Fprintf(a.stdout, "Activated asset %s\n", strings.TrimSpace(*assetID))
		return nil

	case "dispose":
		fs := flag.NewFlagSet("assets dispose", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		assetID := fs.String("id", "", "Asset id")
		disposalDate := fs.String("disposal-date", "", "Disposal date in YYYY-MM-DD")
		methodFlag := fs.String("method", "", "Disposal method")
		proceedsFlag := fs.String("proceeds", "0", "Disposal proceeds")
		notes := fs.String("notes", "", "Disposal notes")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*assetID) == "" {
			return errors.New("id is required")
		}
		disposalDateValue, err := parseRequiredDate("disposal-date", *disposalDate)
		if err != nil {
			return err
		}
		method, err := parseRequiredDisposalMethod(*methodFlag)
		if err != nil {
			return err
		}
		proceeds, err := parseRequiredNonNegativeDecimal("proceeds", *proceedsFlag)
		if err != nil {
			return err
		}

		result, err := client.disposeAsset(ctx, cfg.TenantID, strings.TrimSpace(*assetID), &assets.DisposeAssetRequest{
			DisposalDate:     disposalDateValue,
			DisposalMethod:   method,
			DisposalProceeds: proceeds,
			DisposalNotes:    strings.TrimSpace(*notes),
		})
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, result)
		}
		_, _ = fmt.Fprintf(a.stdout, "Disposed asset %s\n", strings.TrimSpace(*assetID))
		return nil

	case "depreciate":
		fs := flag.NewFlagSet("assets depreciate", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		assetID := fs.String("id", "", "Asset id")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*assetID) == "" {
			return errors.New("id is required")
		}

		entry, err := client.recordAssetDepreciation(ctx, cfg.TenantID, strings.TrimSpace(*assetID))
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, entry)
		}
		_, _ = fmt.Fprintf(a.stdout, "Recorded depreciation %s for asset %s\n", entry.DepreciationAmount.String(), strings.TrimSpace(*assetID))
		return nil

	case "depreciation":
		fs := flag.NewFlagSet("assets depreciation", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		assetID := fs.String("id", "", "Asset id")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*assetID) == "" {
			return errors.New("id is required")
		}

		entries, err := client.listAssetDepreciation(ctx, cfg.TenantID, strings.TrimSpace(*assetID))
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, entries)
		}
		printDepreciationEntriesTable(a.stdout, entries)
		return nil

	default:
		return fmt.Errorf("unknown assets subcommand %q", args[0])
	}
}

func (a *cliApp) runAssetCategories(ctx context.Context, cfg *cliConfig, client *apiClient, args []string) error {
	if len(args) == 0 {
		return errors.New("assets categories subcommand required")
	}

	switch args[0] {
	case "list":
		fs := flag.NewFlagSet("assets categories list", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}

		categories, err := client.listAssetCategories(ctx, cfg.TenantID)
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, categories)
		}
		printAssetCategoriesTable(a.stdout, categories)
		return nil

	case "create":
		fs := flag.NewFlagSet("assets categories create", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		name := fs.String("name", "", "Category name")
		description := fs.String("description", "", "Description")
		depreciationMethodFlag := fs.String("depreciation-method", "STRAIGHT_LINE", "Depreciation method")
		usefulLifeMonthsFlag := fs.String("useful-life-months", "60", "Default useful life in months")
		residualPercentFlag := fs.String("residual-percent", "0", "Default residual value percentage")
		assetAccountID := fs.String("asset-account-id", "", "Asset account id")
		depreciationExpenseAccountID := fs.String("depreciation-expense-account-id", "", "Depreciation expense account id")
		accumulatedDepreciationAccountID := fs.String("accumulated-depreciation-account-id", "", "Accumulated depreciation account id")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*name) == "" {
			return errors.New("name is required")
		}
		depreciationMethod, err := parseOptionalDepreciationMethod(*depreciationMethodFlag)
		if err != nil {
			return err
		}
		usefulLifeMonths, err := parseRequiredPositiveInt("useful-life-months", *usefulLifeMonthsFlag)
		if err != nil {
			return err
		}
		residualPercent, err := parseRequiredNonNegativeDecimal("residual-percent", *residualPercentFlag)
		if err != nil {
			return err
		}

		category, err := client.createAssetCategory(ctx, cfg.TenantID, &assets.CreateCategoryRequest{
			Name:                          strings.TrimSpace(*name),
			Description:                   strings.TrimSpace(*description),
			DepreciationMethod:            depreciationMethod,
			DefaultUsefulLifeMonths:       usefulLifeMonths,
			DefaultResidualValuePercent:   residualPercent,
			AssetAccountID:                optionalStringPtr(*assetAccountID),
			DepreciationExpenseAccountID:  optionalStringPtr(*depreciationExpenseAccountID),
			AccumulatedDepreciationAcctID: optionalStringPtr(*accumulatedDepreciationAccountID),
		})
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, category)
		}
		_, _ = fmt.Fprintf(a.stdout, "Created asset category %s (%s)\n", category.Name, category.ID)
		return nil

	case "get":
		fs := flag.NewFlagSet("assets categories get", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		categoryID := fs.String("id", "", "Category id")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*categoryID) == "" {
			return errors.New("id is required")
		}

		category, err := client.getAssetCategory(ctx, cfg.TenantID, strings.TrimSpace(*categoryID))
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, category)
		}
		printAssetCategory(a.stdout, category)
		return nil

	case "delete":
		fs := flag.NewFlagSet("assets categories delete", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		categoryID := fs.String("id", "", "Category id")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*categoryID) == "" {
			return errors.New("id is required")
		}

		if err := client.deleteAssetCategory(ctx, cfg.TenantID, strings.TrimSpace(*categoryID)); err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, map[string]string{"status": "deleted"})
		}
		_, _ = fmt.Fprintf(a.stdout, "Deleted asset category %s\n", strings.TrimSpace(*categoryID))
		return nil

	default:
		return fmt.Errorf("unknown assets categories subcommand %q", args[0])
	}
}

func (a *cliApp) runCostCenters(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("cost-centers subcommand required")
	}
	cfg, client, err := a.loadAuthenticatedClient()
	if err != nil {
		return err
	}

	switch args[0] {
	case "list":
		fs := flag.NewFlagSet("cost-centers list", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		activeOnly := fs.Bool("active-only", false, "List only active cost centers")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}

		costCenters, err := client.listCostCenters(ctx, cfg.TenantID, *activeOnly)
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, costCenters)
		}
		printCostCentersTable(a.stdout, costCenters)
		return nil

	case "create":
		fs := flag.NewFlagSet("cost-centers create", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		code := fs.String("code", "", "Cost center code")
		name := fs.String("name", "", "Cost center name")
		description := fs.String("description", "", "Description")
		parentID := fs.String("parent-id", "", "Parent cost center id")
		active := fs.Bool("active", true, "Create as active")
		budgetAmountFlag := fs.String("budget-amount", "", "Budget amount")
		budgetPeriodFlag := fs.String("budget-period", "ANNUAL", "Budget period")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*code) == "" {
			return errors.New("code is required")
		}
		if strings.TrimSpace(*name) == "" {
			return errors.New("name is required")
		}
		budgetAmount, err := parseOptionalNonNegativeDecimalPtr("budget-amount", *budgetAmountFlag)
		if err != nil {
			return err
		}
		budgetPeriod, err := parseOptionalBudgetPeriod(*budgetPeriodFlag)
		if err != nil {
			return err
		}

		costCenter, err := client.createCostCenter(ctx, cfg.TenantID, &accounting.CreateCostCenterRequest{
			Code:         strings.TrimSpace(*code),
			Name:         strings.TrimSpace(*name),
			Description:  strings.TrimSpace(*description),
			ParentID:     optionalStringPtr(*parentID),
			IsActive:     *active,
			BudgetAmount: budgetAmount,
			BudgetPeriod: budgetPeriod,
		})
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, costCenter)
		}
		_, _ = fmt.Fprintf(a.stdout, "Created cost center %s %s (%s)\n", costCenter.Code, costCenter.Name, costCenter.ID)
		return nil

	case "get":
		fs := flag.NewFlagSet("cost-centers get", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		costCenterID := fs.String("id", "", "Cost center id")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*costCenterID) == "" {
			return errors.New("id is required")
		}

		costCenter, err := client.getCostCenter(ctx, cfg.TenantID, strings.TrimSpace(*costCenterID))
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, costCenter)
		}
		printCostCenter(a.stdout, costCenter)
		return nil

	case "update":
		fs := flag.NewFlagSet("cost-centers update", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		costCenterID := fs.String("id", "", "Cost center id")
		code := fs.String("code", "", "Cost center code")
		name := fs.String("name", "", "Cost center name")
		description := fs.String("description", "", "Description")
		parentID := fs.String("parent-id", "", "Parent cost center id")
		active := fs.Bool("active", true, "Set active")
		budgetAmountFlag := fs.String("budget-amount", "", "Budget amount")
		budgetPeriodFlag := fs.String("budget-period", "ANNUAL", "Budget period")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*costCenterID) == "" {
			return errors.New("id is required")
		}
		if strings.TrimSpace(*code) == "" {
			return errors.New("code is required")
		}
		if strings.TrimSpace(*name) == "" {
			return errors.New("name is required")
		}
		budgetAmount, err := parseOptionalNonNegativeDecimalPtr("budget-amount", *budgetAmountFlag)
		if err != nil {
			return err
		}
		budgetPeriod, err := parseOptionalBudgetPeriod(*budgetPeriodFlag)
		if err != nil {
			return err
		}

		costCenter, err := client.updateCostCenter(ctx, cfg.TenantID, strings.TrimSpace(*costCenterID), &accounting.UpdateCostCenterRequest{
			Code:         strings.TrimSpace(*code),
			Name:         strings.TrimSpace(*name),
			Description:  strings.TrimSpace(*description),
			ParentID:     optionalStringPtr(*parentID),
			IsActive:     *active,
			BudgetAmount: budgetAmount,
			BudgetPeriod: budgetPeriod,
		})
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, costCenter)
		}
		printCostCenter(a.stdout, costCenter)
		return nil

	case "delete":
		fs := flag.NewFlagSet("cost-centers delete", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		costCenterID := fs.String("id", "", "Cost center id")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*costCenterID) == "" {
			return errors.New("id is required")
		}

		if err := client.deleteCostCenter(ctx, cfg.TenantID, strings.TrimSpace(*costCenterID)); err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, map[string]string{"status": "deleted"})
		}
		_, _ = fmt.Fprintf(a.stdout, "Deleted cost center %s\n", strings.TrimSpace(*costCenterID))
		return nil

	case "report":
		fs := flag.NewFlagSet("cost-centers report", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		startDate := fs.String("start", "", "Start date in YYYY-MM-DD")
		endDate := fs.String("end", "", "End date in YYYY-MM-DD")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		startDateValue, err := parseOptionalDate("start", *startDate)
		if err != nil {
			return err
		}
		endDateValue, err := parseOptionalDate("end", *endDate)
		if err != nil {
			return err
		}

		report, err := client.getCostCenterReport(ctx, cfg.TenantID, startDateValue, endDateValue)
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, report)
		}
		printCostCenterReport(a.stdout, report)
		return nil

	default:
		return fmt.Errorf("unknown cost-centers subcommand %q", args[0])
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

	case "get":
		fs := flag.NewFlagSet("employees get", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		employeeID := fs.String("id", "", "Employee id")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*employeeID) == "" {
			return errors.New("id is required")
		}

		employee, err := client.getEmployee(ctx, cfg.TenantID, strings.TrimSpace(*employeeID))
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, employee)
		}
		printEmployee(a.stdout, employee)
		return nil

	case "update":
		fs := flag.NewFlagSet("employees update", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		employeeID := fs.String("id", "", "Employee id")
		employeeNumber := fs.String("employee-number", "", "Employee number")
		firstName := fs.String("first-name", "", "First name")
		lastName := fs.String("last-name", "", "Last name")
		personalCode := fs.String("personal-code", "", "Personal code")
		email := fs.String("email", "", "Email")
		phone := fs.String("phone", "", "Phone")
		address := fs.String("address", "", "Address")
		bankAccount := fs.String("bank-account", "", "IBAN")
		endDate := fs.String("end-date", "", "Employment end date in YYYY-MM-DD")
		position := fs.String("position", "", "Position")
		department := fs.String("department", "", "Department")
		employmentType := fs.String("employment-type", "", "Employment type: FULL_TIME, PART_TIME, CONTRACT")
		applyBasicExemption := fs.String("apply-basic-exemption", "", "Apply basic exemption: true or false")
		basicExemptionAmount := fs.String("basic-exemption-amount", "", "Basic exemption amount")
		fundedPensionRate := fs.String("funded-pension-rate", "", "Funded pension rate")
		active := fs.String("active", "", "Set active state: true or false")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*employeeID) == "" {
			return errors.New("id is required")
		}

		req := &payroll.UpdateEmployeeRequest{
			EmployeeNumber: strings.TrimSpace(*employeeNumber),
			FirstName:      strings.TrimSpace(*firstName),
			LastName:       strings.TrimSpace(*lastName),
			PersonalCode:   strings.TrimSpace(*personalCode),
			Email:          strings.TrimSpace(*email),
			Phone:          strings.TrimSpace(*phone),
			Address:        strings.TrimSpace(*address),
			BankAccount:    strings.TrimSpace(*bankAccount),
			Position:       strings.TrimSpace(*position),
			Department:     strings.TrimSpace(*department),
		}
		if strings.TrimSpace(*employmentType) != "" {
			req.EmploymentType = payroll.EmploymentType(strings.ToUpper(strings.TrimSpace(*employmentType)))
		}
		if strings.TrimSpace(*endDate) != "" {
			parsed, err := parseRequiredDate("end-date", *endDate)
			if err != nil {
				return err
			}
			req.EndDate = &parsed
		}
		if strings.TrimSpace(*applyBasicExemption) != "" {
			parsed, err := strconv.ParseBool(strings.TrimSpace(*applyBasicExemption))
			if err != nil {
				return fmt.Errorf("parse apply-basic-exemption: %w", err)
			}
			req.ApplyBasicExemption = &parsed
		}
		if strings.TrimSpace(*basicExemptionAmount) != "" {
			parsed, err := decimal.NewFromString(strings.TrimSpace(*basicExemptionAmount))
			if err != nil {
				return fmt.Errorf("parse basic-exemption-amount: %w", err)
			}
			req.BasicExemptionAmount = &parsed
		}
		if strings.TrimSpace(*fundedPensionRate) != "" {
			parsed, err := decimal.NewFromString(strings.TrimSpace(*fundedPensionRate))
			if err != nil {
				return fmt.Errorf("parse funded-pension-rate: %w", err)
			}
			req.FundedPensionRate = &parsed
		}
		if strings.TrimSpace(*active) != "" {
			parsed, err := strconv.ParseBool(strings.TrimSpace(*active))
			if err != nil {
				return fmt.Errorf("parse active: %w", err)
			}
			req.IsActive = &parsed
		}

		employee, err := client.updateEmployee(ctx, cfg.TenantID, strings.TrimSpace(*employeeID), req)
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, employee)
		}
		printEmployee(a.stdout, employee)
		return nil

	case "set-salary":
		fs := flag.NewFlagSet("employees set-salary", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		employeeID := fs.String("id", "", "Employee id")
		amountFlag := fs.String("amount", "", "Base salary amount")
		effectiveFrom := fs.String("effective-from", "", "Effective date in YYYY-MM-DD")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*employeeID) == "" {
			return errors.New("id is required")
		}
		amount, err := parseRequiredPositiveDecimal("amount", *amountFlag)
		if err != nil {
			return err
		}
		effectiveFromValue, err := parseRequiredDate("effective-from", *effectiveFrom)
		if err != nil {
			return err
		}

		result, err := client.setBaseSalary(ctx, cfg.TenantID, strings.TrimSpace(*employeeID), amount, effectiveFromValue)
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, result)
		}
		_, _ = fmt.Fprintf(a.stdout, "Set base salary for employee %s to %s\n", strings.TrimSpace(*employeeID), amount.String())
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
	case "list":
		fs := flag.NewFlagSet("journal list", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		limitFlag := fs.String("limit", "50", "Maximum entries to return")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		limit, err := parseRequiredPositiveInt("limit", *limitFlag)
		if err != nil {
			return err
		}
		if limit > 200 {
			return errors.New("limit must be between 1 and 200")
		}

		entries, err := client.listJournalEntries(ctx, cfg.TenantID, limit)
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, entries)
		}
		printJournalEntriesTable(a.stdout, entries)
		return nil

	case "get":
		fs := flag.NewFlagSet("journal get", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		entryID := fs.String("id", "", "Journal entry id")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*entryID) == "" {
			return errors.New("id is required")
		}

		entry, err := client.getJournalEntry(ctx, cfg.TenantID, strings.TrimSpace(*entryID))
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, entry)
		}
		printJournalEntry(a.stdout, entry)
		return nil

	case "create":
		fs := flag.NewFlagSet("journal create", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		entryDate := fs.String("entry-date", "", "Entry date in YYYY-MM-DD")
		description := fs.String("description", "", "Journal entry description")
		reference := fs.String("reference", "", "Reference")
		sourceType := fs.String("source-type", "", "Source type")
		sourceID := fs.String("source-id", "", "Source id")
		lines := journalLineFlags{}
		fs.Var(&lines, "line", "Line as comma-separated key=value pairs; repeatable")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		entryDateValue, err := parseRequiredDate("entry-date", *entryDate)
		if err != nil {
			return err
		}
		if strings.TrimSpace(*description) == "" {
			return errors.New("description is required")
		}
		if len(lines) < 2 {
			return errors.New("at least two lines are required")
		}

		entry, err := client.createJournalEntry(ctx, cfg.TenantID, &accounting.CreateJournalEntryRequest{
			EntryDate:   entryDateValue,
			Description: strings.TrimSpace(*description),
			Reference:   strings.TrimSpace(*reference),
			SourceType:  strings.TrimSpace(*sourceType),
			SourceID:    optionalStringPtr(*sourceID),
			Lines:       []accounting.CreateJournalEntryLineReq(lines),
		})
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, entry)
		}
		_, _ = fmt.Fprintf(a.stdout, "Created journal entry %s (%s)\n", entry.EntryNumber, entry.ID)
		return nil

	case "post":
		fs := flag.NewFlagSet("journal post", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		entryID := fs.String("id", "", "Journal entry id")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*entryID) == "" {
			return errors.New("id is required")
		}

		result, err := client.postJournalEntry(ctx, cfg.TenantID, strings.TrimSpace(*entryID))
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, result)
		}
		_, _ = fmt.Fprintf(a.stdout, "Posted journal entry %s\n", strings.TrimSpace(*entryID))
		return nil

	case "void":
		fs := flag.NewFlagSet("journal void", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		entryID := fs.String("id", "", "Journal entry id")
		reason := fs.String("reason", "", "Void reason")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*entryID) == "" {
			return errors.New("id is required")
		}
		if strings.TrimSpace(*reason) == "" {
			return errors.New("reason is required")
		}

		reversal, err := client.voidJournalEntry(ctx, cfg.TenantID, strings.TrimSpace(*entryID), strings.TrimSpace(*reason))
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, reversal)
		}
		_, _ = fmt.Fprintf(a.stdout, "Voided journal entry %s with reversal %s\n", strings.TrimSpace(*entryID), reversal.EntryNumber)
		return nil

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

func (a *cliApp) runPayroll(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("payroll subcommand required")
	}

	cfg, client, err := a.loadAuthenticatedClient()
	if err != nil {
		return err
	}

	switch args[0] {
	case "runs":
		return a.runPayrollRuns(ctx, cfg, client, args[1:])

	case "tax-preview":
		fs := flag.NewFlagSet("payroll tax-preview", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		grossSalary := fs.String("gross-salary", "", "Gross salary")
		applyBasicExemption := fs.Bool("apply-basic-exemption", true, "Apply basic exemption")
		fundedPensionRate := fs.String("funded-pension-rate", "0.02", "Funded pension rate")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		grossSalaryValue, err := decimal.NewFromString(strings.TrimSpace(*grossSalary))
		if err != nil || grossSalaryValue.LessThanOrEqual(decimal.Zero) {
			return errors.New("gross-salary must be a positive decimal")
		}
		fundedPensionValue, err := decimal.NewFromString(strings.TrimSpace(*fundedPensionRate))
		if err != nil {
			return fmt.Errorf("parse funded-pension-rate: %w", err)
		}

		calculation, err := client.calculateTaxPreview(ctx, cfg.TenantID, grossSalaryValue, *applyBasicExemption, fundedPensionValue)
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, calculation)
		}
		printTaxCalculation(a.stdout, calculation)
		return nil

	case "import-history":
		fs := flag.NewFlagSet("payroll import-history", flag.ContinueOnError)
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
		result, err := client.importPayrollHistory(ctx, cfg.TenantID, &payroll.ImportPayrollHistoryRequest{
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
			"Processed %d rows, created %d payroll runs, created %d payslips, skipped %d rows\n",
			result.RowsProcessed,
			result.PayrollRunsCreated,
			result.PayslipsCreated,
			result.RowsSkipped,
		)
		return nil
	case "import-leave-balances":
		fs := flag.NewFlagSet("payroll import-leave-balances", flag.ContinueOnError)
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
		result, err := client.importLeaveBalances(ctx, cfg.TenantID, &payroll.ImportLeaveBalancesRequest{
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
			"Processed %d rows, created %d leave balances, updated %d leave balances, skipped %d rows\n",
			result.RowsProcessed,
			result.LeaveBalancesCreated,
			result.LeaveBalancesUpdated,
			result.RowsSkipped,
		)
		return nil
	default:
		return fmt.Errorf("unknown payroll subcommand %q", args[0])
	}
}

func (a *cliApp) runPayrollRuns(ctx context.Context, cfg *cliConfig, client *apiClient, args []string) error {
	if len(args) == 0 {
		return errors.New("payroll runs subcommand required")
	}

	switch args[0] {
	case "list":
		fs := flag.NewFlagSet("payroll runs list", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		yearFlag := fs.String("year", "", "Optional period year")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		year := 0
		if strings.TrimSpace(*yearFlag) != "" {
			parsed, err := parseRequiredPositiveInt("year", *yearFlag)
			if err != nil {
				return err
			}
			year = parsed
		}

		runs, err := client.listPayrollRuns(ctx, cfg.TenantID, year)
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, runs)
		}
		printPayrollRunsTable(a.stdout, runs)
		return nil

	case "create":
		fs := flag.NewFlagSet("payroll runs create", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		yearFlag := fs.String("year", "", "Period year")
		monthFlag := fs.String("month", "", "Period month")
		paymentDate := fs.String("payment-date", "", "Optional payment date in YYYY-MM-DD")
		notes := fs.String("notes", "", "Optional notes")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		year, month, err := parseYearMonthFlags(*yearFlag, *monthFlag)
		if err != nil {
			return err
		}
		var paymentDateValue *time.Time
		if strings.TrimSpace(*paymentDate) != "" {
			parsed, err := time.Parse("2006-01-02", strings.TrimSpace(*paymentDate))
			if err != nil {
				return fmt.Errorf("parse payment-date: %w", err)
			}
			paymentDateValue = &parsed
		}

		run, err := client.createPayrollRun(ctx, cfg.TenantID, &payroll.CreatePayrollRunRequest{
			PeriodYear:  year,
			PeriodMonth: month,
			PaymentDate: paymentDateValue,
			Notes:       strings.TrimSpace(*notes),
		})
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, run)
		}
		_, _ = fmt.Fprintf(a.stdout, "Created payroll run %04d-%02d (%s)\n", run.PeriodYear, run.PeriodMonth, run.ID)
		return nil

	case "get":
		fs := flag.NewFlagSet("payroll runs get", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		runID := fs.String("id", "", "Payroll run id")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*runID) == "" {
			return errors.New("id is required")
		}

		run, err := client.getPayrollRun(ctx, cfg.TenantID, strings.TrimSpace(*runID))
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, run)
		}
		printPayrollRun(a.stdout, run)
		return nil

	case "calculate":
		fs := flag.NewFlagSet("payroll runs calculate", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		runID := fs.String("id", "", "Payroll run id")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*runID) == "" {
			return errors.New("id is required")
		}

		run, err := client.calculatePayrollRun(ctx, cfg.TenantID, strings.TrimSpace(*runID))
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, run)
		}
		printPayrollRun(a.stdout, run)
		return nil

	case "approve":
		fs := flag.NewFlagSet("payroll runs approve", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		runID := fs.String("id", "", "Payroll run id")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*runID) == "" {
			return errors.New("id is required")
		}

		result, err := client.approvePayrollRun(ctx, cfg.TenantID, strings.TrimSpace(*runID))
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, result)
		}
		_, _ = fmt.Fprintf(a.stdout, "Approved payroll run %s\n", strings.TrimSpace(*runID))
		return nil

	case "payslips":
		fs := flag.NewFlagSet("payroll runs payslips", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		runID := fs.String("id", "", "Payroll run id")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*runID) == "" {
			return errors.New("id is required")
		}

		payslips, err := client.listPayslips(ctx, cfg.TenantID, strings.TrimSpace(*runID))
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, payslips)
		}
		printPayslipsTable(a.stdout, payslips)
		return nil

	default:
		return fmt.Errorf("unknown payroll runs subcommand %q", args[0])
	}
}

func (a *cliApp) runTSD(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("tsd subcommand required")
	}

	cfg, client, err := a.loadAuthenticatedClient()
	if err != nil {
		return err
	}

	switch args[0] {
	case "list":
		fs := flag.NewFlagSet("tsd list", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}

		declarations, err := client.listTSD(ctx, cfg.TenantID)
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, declarations)
		}
		printTSDDeclarationsTable(a.stdout, declarations)
		return nil

	case "get":
		fs := flag.NewFlagSet("tsd get", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		yearFlag := fs.String("year", "", "Declaration year")
		monthFlag := fs.String("month", "", "Declaration month")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		year, month, err := parseYearMonthFlags(*yearFlag, *monthFlag)
		if err != nil {
			return err
		}

		declaration, err := client.getTSD(ctx, cfg.TenantID, year, month)
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, declaration)
		}
		printTSDDeclaration(a.stdout, declaration)
		return nil

	case "generate":
		fs := flag.NewFlagSet("tsd generate", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		runID := fs.String("run-id", "", "Payroll run id")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*runID) == "" {
			return errors.New("run-id is required")
		}

		declaration, err := client.generateTSD(ctx, cfg.TenantID, strings.TrimSpace(*runID))
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, declaration)
		}
		printTSDDeclaration(a.stdout, declaration)
		return nil

	case "export-xml":
		fs := flag.NewFlagSet("tsd export-xml", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		yearFlag := fs.String("year", "", "Declaration year")
		monthFlag := fs.String("month", "", "Declaration month")
		outputPath := fs.String("output", "", "Optional output file path")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		year, month, err := parseYearMonthFlags(*yearFlag, *monthFlag)
		if err != nil {
			return err
		}

		content, err := client.exportTSDXML(ctx, cfg.TenantID, year, month)
		if err != nil {
			return err
		}
		return writeExportOutput(a.stdout, strings.TrimSpace(*outputPath), content, "TSD XML")

	case "export-csv":
		fs := flag.NewFlagSet("tsd export-csv", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		yearFlag := fs.String("year", "", "Declaration year")
		monthFlag := fs.String("month", "", "Declaration month")
		outputPath := fs.String("output", "", "Optional output file path")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		year, month, err := parseYearMonthFlags(*yearFlag, *monthFlag)
		if err != nil {
			return err
		}

		content, err := client.exportTSDCSV(ctx, cfg.TenantID, year, month)
		if err != nil {
			return err
		}
		return writeExportOutput(a.stdout, strings.TrimSpace(*outputPath), content, "TSD CSV")

	case "mark-submitted":
		fs := flag.NewFlagSet("tsd mark-submitted", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		yearFlag := fs.String("year", "", "Declaration year")
		monthFlag := fs.String("month", "", "Declaration month")
		emtaReference := fs.String("emta-reference", "", "e-MTA submission reference")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		year, month, err := parseYearMonthFlags(*yearFlag, *monthFlag)
		if err != nil {
			return err
		}

		result, err := client.markTSDSubmitted(ctx, cfg.TenantID, year, month, strings.TrimSpace(*emtaReference))
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, result)
		}
		_, _ = fmt.Fprintf(a.stdout, "Marked TSD %04d-%02d as submitted\n", year, month)
		return nil

	default:
		return fmt.Errorf("unknown tsd subcommand %q", args[0])
	}
}

func (a *cliApp) runTax(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("tax subcommand required")
	}
	if args[0] != "kmd" {
		return fmt.Errorf("unknown tax subcommand %q", args[0])
	}
	if len(args) == 1 {
		return errors.New("tax kmd subcommand required")
	}

	cfg, client, err := a.loadAuthenticatedClient()
	if err != nil {
		return err
	}

	switch args[1] {
	case "list":
		fs := flag.NewFlagSet("tax kmd list", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[2:]); err != nil {
			return err
		}

		declarations, err := client.listKMD(ctx, cfg.TenantID)
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, declarations)
		}
		printKMDDeclarationsTable(a.stdout, declarations)
		return nil

	case "generate":
		fs := flag.NewFlagSet("tax kmd generate", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		yearFlag := fs.String("year", "", "Declaration year")
		monthFlag := fs.String("month", "", "Declaration month")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[2:]); err != nil {
			return err
		}
		year, month, err := parseYearMonthFlags(*yearFlag, *monthFlag)
		if err != nil {
			return err
		}

		declaration, err := client.generateKMD(ctx, cfg.TenantID, &tax.CreateKMDRequest{
			Year:  year,
			Month: month,
		})
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, declaration)
		}
		printKMDDeclaration(a.stdout, declaration)
		return nil

	case "export-xml":
		fs := flag.NewFlagSet("tax kmd export-xml", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		yearFlag := fs.String("year", "", "Declaration year")
		monthFlag := fs.String("month", "", "Declaration month")
		outputPath := fs.String("output", "", "Optional output file path")
		if err := fs.Parse(args[2:]); err != nil {
			return err
		}
		year, month, err := parseYearMonthFlags(*yearFlag, *monthFlag)
		if err != nil {
			return err
		}

		content, err := client.exportKMDXML(ctx, cfg.TenantID, year, month)
		if err != nil {
			return err
		}
		return writeExportOutput(a.stdout, strings.TrimSpace(*outputPath), content, "KMD XML")

	default:
		return fmt.Errorf("unknown tax kmd subcommand %q", args[1])
	}
}

func (a *cliApp) runReports(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("reports subcommand required")
	}

	cfg, client, err := a.loadAuthenticatedClient()
	if err != nil {
		return err
	}

	switch args[0] {
	case "trial-balance":
		fs := flag.NewFlagSet("reports trial-balance", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		asOf := fs.String("as-of", "", "As-of date in YYYY-MM-DD")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}

		report, err := client.getTrialBalance(ctx, cfg.TenantID, strings.TrimSpace(*asOf))
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, report)
		}
		printTrialBalance(a.stdout, report)
		return nil

	case "account-balance":
		fs := flag.NewFlagSet("reports account-balance", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		accountID := fs.String("account-id", "", "Account id")
		asOf := fs.String("as-of", "", "As-of date in YYYY-MM-DD")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*accountID) == "" {
			return errors.New("account-id is required")
		}

		report, err := client.getAccountBalanceReport(ctx, cfg.TenantID, strings.TrimSpace(*accountID), strings.TrimSpace(*asOf))
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, report)
		}
		printAccountBalance(a.stdout, report)
		return nil

	case "balance-sheet":
		fs := flag.NewFlagSet("reports balance-sheet", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		asOf := fs.String("as-of", "", "As-of date in YYYY-MM-DD")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}

		report, err := client.getBalanceSheet(ctx, cfg.TenantID, strings.TrimSpace(*asOf))
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, report)
		}
		printBalanceSheet(a.stdout, report)
		return nil

	case "income-statement":
		fs := flag.NewFlagSet("reports income-statement", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		startDate := fs.String("start", "", "Start date in YYYY-MM-DD")
		endDate := fs.String("end", "", "End date in YYYY-MM-DD")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*startDate) == "" || strings.TrimSpace(*endDate) == "" {
			return errors.New("start and end are required")
		}

		report, err := client.getIncomeStatement(ctx, cfg.TenantID, strings.TrimSpace(*startDate), strings.TrimSpace(*endDate))
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, report)
		}
		printIncomeStatement(a.stdout, report)
		return nil

	case "cash-flow":
		fs := flag.NewFlagSet("reports cash-flow", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		startDate := fs.String("start", "", "Start date in YYYY-MM-DD")
		endDate := fs.String("end", "", "End date in YYYY-MM-DD")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*startDate) == "" || strings.TrimSpace(*endDate) == "" {
			return errors.New("start and end are required")
		}

		report, err := client.getCashFlowStatement(ctx, cfg.TenantID, strings.TrimSpace(*startDate), strings.TrimSpace(*endDate))
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, report)
		}
		printCashFlowStatement(a.stdout, report)
		return nil

	case "aging":
		fs := flag.NewFlagSet("reports aging", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		reportType := fs.String("type", "receivables", "Aging report type: receivables or payables")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		normalizedType := strings.ToLower(strings.TrimSpace(*reportType))
		if normalizedType != "receivables" && normalizedType != "payables" {
			return errors.New("type must be receivables or payables")
		}

		report, err := client.getAgingReport(ctx, cfg.TenantID, normalizedType)
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, report)
		}
		printAgingReport(a.stdout, report)
		return nil

	case "balance-confirmations":
		fs := flag.NewFlagSet("reports balance-confirmations", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		balanceType := fs.String("type", "", "Balance type: RECEIVABLE or PAYABLE")
		asOf := fs.String("as-of", "", "As-of date in YYYY-MM-DD")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		normalizedType := strings.ToUpper(strings.TrimSpace(*balanceType))
		if normalizedType != "RECEIVABLE" && normalizedType != "PAYABLE" {
			return errors.New("type must be RECEIVABLE or PAYABLE")
		}
		if strings.TrimSpace(*asOf) == "" {
			return errors.New("as-of is required")
		}

		report, err := client.getBalanceConfirmationSummary(ctx, cfg.TenantID, normalizedType, strings.TrimSpace(*asOf))
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, report)
		}
		printBalanceConfirmationSummary(a.stdout, report)
		return nil

	case "balance-confirmation":
		fs := flag.NewFlagSet("reports balance-confirmation", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		contactID := fs.String("contact-id", "", "Contact id")
		balanceType := fs.String("type", "", "Balance type: RECEIVABLE or PAYABLE")
		asOf := fs.String("as-of", "", "As-of date in YYYY-MM-DD")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*contactID) == "" {
			return errors.New("contact-id is required")
		}
		normalizedType := strings.ToUpper(strings.TrimSpace(*balanceType))
		if normalizedType != "RECEIVABLE" && normalizedType != "PAYABLE" {
			return errors.New("type must be RECEIVABLE or PAYABLE")
		}
		if strings.TrimSpace(*asOf) == "" {
			return errors.New("as-of is required")
		}

		report, err := client.getBalanceConfirmation(ctx, cfg.TenantID, strings.TrimSpace(*contactID), normalizedType, strings.TrimSpace(*asOf))
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, report)
		}
		printBalanceConfirmation(a.stdout, report)
		return nil

	default:
		return fmt.Errorf("unknown reports subcommand %q", args[0])
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

func parseYearMonthFlags(yearValue, monthValue string) (int, int, error) {
	year, err := parseRequiredPositiveInt("year", yearValue)
	if err != nil {
		return 0, 0, err
	}
	month, err := parseRequiredPositiveInt("month", monthValue)
	if err != nil {
		return 0, 0, err
	}
	if month < 1 || month > 12 {
		return 0, 0, errors.New("month must be between 1 and 12")
	}
	return year, month, nil
}

func parseOptionalDate(name, value string) (*time.Time, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}
	parsed, err := time.Parse("2006-01-02", strings.TrimSpace(value))
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", name, err)
	}
	return &parsed, nil
}

func parseRequiredDate(name, value string) (time.Time, error) {
	parsed, err := parseOptionalDate(name, value)
	if err != nil {
		return time.Time{}, err
	}
	if parsed == nil {
		return time.Time{}, fmt.Errorf("%s is required", name)
	}
	return *parsed, nil
}

func parseRequiredPositiveInt(name, value string) (int, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return 0, fmt.Errorf("%s is required", name)
	}
	parsed, err := strconv.Atoi(trimmed)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", name, err)
	}
	if parsed <= 0 {
		return 0, fmt.Errorf("%s must be positive", name)
	}
	return parsed, nil
}

func parseRequiredNonNegativeInt(name, value string) (int, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return 0, fmt.Errorf("%s is required", name)
	}
	parsed, err := strconv.Atoi(trimmed)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", name, err)
	}
	if parsed < 0 {
		return 0, fmt.Errorf("%s must be non-negative", name)
	}
	return parsed, nil
}

func parseRequiredPositiveDecimal(name, value string) (decimal.Decimal, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return decimal.Zero, fmt.Errorf("%s is required", name)
	}
	parsed, err := decimal.NewFromString(trimmed)
	if err != nil {
		return decimal.Zero, fmt.Errorf("parse %s: %w", name, err)
	}
	if parsed.LessThanOrEqual(decimal.Zero) {
		return decimal.Zero, fmt.Errorf("%s must be positive", name)
	}
	return parsed, nil
}

func parseRequiredNonNegativeDecimal(name, value string) (decimal.Decimal, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return decimal.Zero, fmt.Errorf("%s is required", name)
	}
	parsed, err := decimal.NewFromString(trimmed)
	if err != nil {
		return decimal.Zero, fmt.Errorf("parse %s: %w", name, err)
	}
	if parsed.LessThan(decimal.Zero) {
		return decimal.Zero, fmt.Errorf("%s must be non-negative", name)
	}
	return parsed, nil
}

func parseOptionalNonNegativeDecimalPtr(name, value string) (*decimal.Decimal, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}
	parsed, err := parseRequiredNonNegativeDecimal(name, value)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

func parseOptionalInvoiceType(value string) (invoicing.InvoiceType, error) {
	if strings.TrimSpace(value) == "" {
		return "", nil
	}
	return parseRequiredInvoiceType(value)
}

func parseRequiredInvoiceType(value string) (invoicing.InvoiceType, error) {
	normalized := strings.ToUpper(strings.TrimSpace(value))
	switch invoicing.InvoiceType(normalized) {
	case invoicing.InvoiceTypeSales, invoicing.InvoiceTypePurchase, invoicing.InvoiceTypeCreditNote:
		return invoicing.InvoiceType(normalized), nil
	default:
		if normalized == "" {
			return "", errors.New("type is required")
		}
		return "", fmt.Errorf("invalid invoice type %q", value)
	}
}

func parseOptionalInvoiceStatus(value string) (invoicing.InvoiceStatus, error) {
	if strings.TrimSpace(value) == "" {
		return "", nil
	}
	normalized := strings.ToUpper(strings.TrimSpace(value))
	switch invoicing.InvoiceStatus(normalized) {
	case invoicing.StatusDraft, invoicing.StatusSent, invoicing.StatusPartiallyPaid, invoicing.StatusPaid, invoicing.StatusOverdue, invoicing.StatusVoided:
		return invoicing.InvoiceStatus(normalized), nil
	default:
		return "", fmt.Errorf("invalid invoice status %q", value)
	}
}

func parseOptionalQuoteStatus(value string) (quotes.QuoteStatus, error) {
	if strings.TrimSpace(value) == "" {
		return "", nil
	}
	normalized := strings.ToUpper(strings.TrimSpace(value))
	switch quotes.QuoteStatus(normalized) {
	case quotes.QuoteStatusDraft, quotes.QuoteStatusSent, quotes.QuoteStatusAccepted, quotes.QuoteStatusRejected, quotes.QuoteStatusExpired, quotes.QuoteStatusConverted:
		return quotes.QuoteStatus(normalized), nil
	default:
		return "", fmt.Errorf("invalid quote status %q", value)
	}
}

func parseOptionalOrderStatus(value string) (orders.OrderStatus, error) {
	if strings.TrimSpace(value) == "" {
		return "", nil
	}
	normalized := strings.ToUpper(strings.TrimSpace(value))
	switch orders.OrderStatus(normalized) {
	case orders.OrderStatusPending, orders.OrderStatusConfirmed, orders.OrderStatusProcessing, orders.OrderStatusShipped, orders.OrderStatusDelivered, orders.OrderStatusCanceled:
		return orders.OrderStatus(normalized), nil
	default:
		return "", fmt.Errorf("invalid order status %q", value)
	}
}

func parseOptionalAssetStatus(value string) (assets.AssetStatus, error) {
	if strings.TrimSpace(value) == "" {
		return "", nil
	}
	normalized := strings.ToUpper(strings.TrimSpace(value))
	switch assets.AssetStatus(normalized) {
	case assets.AssetStatusDraft, assets.AssetStatusActive, assets.AssetStatusDisposed, assets.AssetStatusSold:
		return assets.AssetStatus(normalized), nil
	default:
		return "", fmt.Errorf("invalid asset status %q", value)
	}
}

func parseOptionalDepreciationMethod(value string) (assets.DepreciationMethod, error) {
	if strings.TrimSpace(value) == "" {
		return "", nil
	}
	normalized := strings.ToUpper(strings.TrimSpace(value))
	switch assets.DepreciationMethod(normalized) {
	case assets.DepreciationStraightLine, assets.DepreciationDecliningBalance, assets.DepreciationUnitsOfProd:
		return assets.DepreciationMethod(normalized), nil
	default:
		return "", fmt.Errorf("invalid depreciation method %q", value)
	}
}

func parseRequiredDisposalMethod(value string) (assets.DisposalMethod, error) {
	normalized := strings.ToUpper(strings.TrimSpace(value))
	switch assets.DisposalMethod(normalized) {
	case assets.DisposalSold, assets.DisposalScrapped, assets.DisposalDonated, assets.DisposalLost:
		return assets.DisposalMethod(normalized), nil
	default:
		if normalized == "" {
			return "", errors.New("method is required")
		}
		return "", fmt.Errorf("invalid disposal method %q", value)
	}
}

func parseOptionalBudgetPeriod(value string) (accounting.BudgetPeriod, error) {
	if strings.TrimSpace(value) == "" {
		return "", nil
	}
	normalized := strings.ToUpper(strings.TrimSpace(value))
	switch accounting.BudgetPeriod(normalized) {
	case accounting.BudgetPeriodMonthly, accounting.BudgetPeriodQuarterly, accounting.BudgetPeriodAnnual:
		return accounting.BudgetPeriod(normalized), nil
	default:
		return "", fmt.Errorf("invalid budget period %q", value)
	}
}

func parseOptionalPaymentType(value string) (payments.PaymentType, error) {
	if strings.TrimSpace(value) == "" {
		return "", nil
	}
	return parseRequiredPaymentType(value)
}

func parseRequiredPaymentType(value string) (payments.PaymentType, error) {
	normalized := strings.ToUpper(strings.TrimSpace(value))
	switch payments.PaymentType(normalized) {
	case payments.PaymentTypeReceived, payments.PaymentTypeMade:
		return payments.PaymentType(normalized), nil
	default:
		if normalized == "" {
			return "", errors.New("type is required")
		}
		return "", fmt.Errorf("invalid payment type %q", value)
	}
}

func optionalStringPtr(value string) *string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func optionalUpperStringPtr(value string) *string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	upper := strings.ToUpper(trimmed)
	return &upper
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func quoteActionPastTense(action string) string {
	switch action {
	case "send":
		return "Sent"
	case "accept":
		return "Accepted"
	case "reject":
		return "Rejected"
	default:
		return titleLabel(action)
	}
}

func orderActionPastTense(action string) string {
	switch action {
	case "confirm":
		return "Confirmed"
	case "process":
		return "Processed"
	case "ship":
		return "Shipped"
	case "deliver":
		return "Delivered"
	case "cancel":
		return "Canceled"
	default:
		return titleLabel(action)
	}
}

type invoiceLineFlags []invoicing.CreateInvoiceLineRequest

func (l *invoiceLineFlags) Set(value string) error {
	reader := csv.NewReader(strings.NewReader(value))
	reader.TrimLeadingSpace = true
	reader.FieldsPerRecord = -1
	fields, err := reader.Read()
	if err != nil {
		return fmt.Errorf("parse line: %w", err)
	}

	values := make(map[string]string)
	for _, field := range fields {
		key, val, ok := strings.Cut(field, "=")
		if !ok {
			return fmt.Errorf("line field %q must be key=value", field)
		}
		normalizedKey := strings.ReplaceAll(strings.ToLower(strings.TrimSpace(key)), "-", "_")
		values[normalizedKey] = strings.TrimSpace(val)
	}

	description := strings.TrimSpace(values["description"])
	if description == "" {
		return errors.New("line description is required")
	}
	quantity, err := parseRequiredPositiveDecimal("line quantity", firstNonEmpty(values["quantity"], values["qty"]))
	if err != nil {
		return err
	}
	unitPrice, err := parseRequiredNonNegativeDecimal("line unit_price", firstNonEmpty(values["unit_price"], values["price"]))
	if err != nil {
		return err
	}
	vatRate, err := parseRequiredNonNegativeDecimal("line vat_rate", firstNonEmpty(values["vat_rate"], values["vat"]))
	if err != nil {
		return err
	}
	discountPercent := decimal.Zero
	if rawDiscount := firstNonEmpty(values["discount_percent"], values["discount"]); rawDiscount != "" {
		discountPercent, err = parseRequiredNonNegativeDecimal("line discount_percent", rawDiscount)
		if err != nil {
			return err
		}
	}

	*l = append(*l, invoicing.CreateInvoiceLineRequest{
		Description:     description,
		Quantity:        quantity,
		Unit:            strings.TrimSpace(values["unit"]),
		UnitPrice:       unitPrice,
		DiscountPercent: discountPercent,
		VATRate:         vatRate,
		AccountID:       optionalStringPtr(firstNonEmpty(values["account_id"], values["account"])),
		ProductID:       optionalStringPtr(firstNonEmpty(values["product_id"], values["product"])),
	})
	return nil
}

func (l *invoiceLineFlags) String() string {
	if l == nil {
		return ""
	}
	descriptions := make([]string, 0, len(*l))
	for _, line := range *l {
		descriptions = append(descriptions, line.Description)
	}
	return strings.Join(descriptions, ",")
}

type orderLineFlags []orders.CreateOrderLineRequest

func (l *orderLineFlags) Set(value string) error {
	reader := csv.NewReader(strings.NewReader(value))
	reader.TrimLeadingSpace = true
	reader.FieldsPerRecord = -1
	fields, err := reader.Read()
	if err != nil {
		return fmt.Errorf("parse line: %w", err)
	}

	values := make(map[string]string)
	for _, field := range fields {
		key, val, ok := strings.Cut(field, "=")
		if !ok {
			return fmt.Errorf("line field %q must be key=value", field)
		}
		normalizedKey := strings.ReplaceAll(strings.ToLower(strings.TrimSpace(key)), "-", "_")
		values[normalizedKey] = strings.TrimSpace(val)
	}

	description := strings.TrimSpace(values["description"])
	if description == "" {
		return errors.New("line description is required")
	}
	quantity, err := parseRequiredPositiveDecimal("line quantity", firstNonEmpty(values["quantity"], values["qty"]))
	if err != nil {
		return err
	}
	unitPrice, err := parseRequiredNonNegativeDecimal("line unit_price", firstNonEmpty(values["unit_price"], values["price"]))
	if err != nil {
		return err
	}
	vatRate, err := parseRequiredNonNegativeDecimal("line vat_rate", firstNonEmpty(values["vat_rate"], values["vat"]))
	if err != nil {
		return err
	}
	discountPercent := decimal.Zero
	if rawDiscount := firstNonEmpty(values["discount_percent"], values["discount"]); rawDiscount != "" {
		discountPercent, err = parseRequiredNonNegativeDecimal("line discount_percent", rawDiscount)
		if err != nil {
			return err
		}
	}

	*l = append(*l, orders.CreateOrderLineRequest{
		Description:     description,
		Quantity:        quantity,
		Unit:            strings.TrimSpace(values["unit"]),
		UnitPrice:       unitPrice,
		DiscountPercent: discountPercent,
		VATRate:         vatRate,
		ProductID:       optionalStringPtr(firstNonEmpty(values["product_id"], values["product"])),
	})
	return nil
}

func (l *orderLineFlags) String() string {
	if l == nil {
		return ""
	}
	descriptions := make([]string, 0, len(*l))
	for _, line := range *l {
		descriptions = append(descriptions, line.Description)
	}
	return strings.Join(descriptions, ",")
}

type quoteLineFlags []quotes.CreateQuoteLineRequest

func (l *quoteLineFlags) Set(value string) error {
	reader := csv.NewReader(strings.NewReader(value))
	reader.TrimLeadingSpace = true
	reader.FieldsPerRecord = -1
	fields, err := reader.Read()
	if err != nil {
		return fmt.Errorf("parse line: %w", err)
	}

	values := make(map[string]string)
	for _, field := range fields {
		key, val, ok := strings.Cut(field, "=")
		if !ok {
			return fmt.Errorf("line field %q must be key=value", field)
		}
		normalizedKey := strings.ReplaceAll(strings.ToLower(strings.TrimSpace(key)), "-", "_")
		values[normalizedKey] = strings.TrimSpace(val)
	}

	description := strings.TrimSpace(values["description"])
	if description == "" {
		return errors.New("line description is required")
	}
	quantity, err := parseRequiredPositiveDecimal("line quantity", firstNonEmpty(values["quantity"], values["qty"]))
	if err != nil {
		return err
	}
	unitPrice, err := parseRequiredNonNegativeDecimal("line unit_price", firstNonEmpty(values["unit_price"], values["price"]))
	if err != nil {
		return err
	}
	vatRate, err := parseRequiredNonNegativeDecimal("line vat_rate", firstNonEmpty(values["vat_rate"], values["vat"]))
	if err != nil {
		return err
	}
	discountPercent := decimal.Zero
	if rawDiscount := firstNonEmpty(values["discount_percent"], values["discount"]); rawDiscount != "" {
		discountPercent, err = parseRequiredNonNegativeDecimal("line discount_percent", rawDiscount)
		if err != nil {
			return err
		}
	}

	*l = append(*l, quotes.CreateQuoteLineRequest{
		Description:     description,
		Quantity:        quantity,
		Unit:            strings.TrimSpace(values["unit"]),
		UnitPrice:       unitPrice,
		DiscountPercent: discountPercent,
		VATRate:         vatRate,
		ProductID:       optionalStringPtr(firstNonEmpty(values["product_id"], values["product"])),
	})
	return nil
}

func (l *quoteLineFlags) String() string {
	if l == nil {
		return ""
	}
	descriptions := make([]string, 0, len(*l))
	for _, line := range *l {
		descriptions = append(descriptions, line.Description)
	}
	return strings.Join(descriptions, ",")
}

type journalLineFlags []accounting.CreateJournalEntryLineReq

func (l *journalLineFlags) Set(value string) error {
	reader := csv.NewReader(strings.NewReader(value))
	reader.TrimLeadingSpace = true
	reader.FieldsPerRecord = -1
	fields, err := reader.Read()
	if err != nil {
		return fmt.Errorf("parse line: %w", err)
	}

	values := make(map[string]string)
	for _, field := range fields {
		key, val, ok := strings.Cut(field, "=")
		if !ok {
			return fmt.Errorf("line field %q must be key=value", field)
		}
		normalizedKey := strings.ReplaceAll(strings.ToLower(strings.TrimSpace(key)), "-", "_")
		values[normalizedKey] = strings.TrimSpace(val)
	}

	accountID := strings.TrimSpace(values["account_id"])
	if accountID == "" {
		accountID = strings.TrimSpace(values["account"])
	}
	if accountID == "" {
		return errors.New("line account_id is required")
	}

	debitAmount := decimal.Zero
	if rawDebit := firstNonEmpty(values["debit_amount"], values["debit"]); rawDebit != "" {
		debitAmount, err = parseRequiredNonNegativeDecimal("line debit_amount", rawDebit)
		if err != nil {
			return err
		}
	}
	creditAmount := decimal.Zero
	if rawCredit := firstNonEmpty(values["credit_amount"], values["credit"]); rawCredit != "" {
		creditAmount, err = parseRequiredNonNegativeDecimal("line credit_amount", rawCredit)
		if err != nil {
			return err
		}
	}
	if debitAmount.IsZero() == creditAmount.IsZero() {
		return errors.New("line must have exactly one of debit or credit")
	}

	exchangeRate := decimal.Zero
	if rawExchangeRate := firstNonEmpty(values["exchange_rate"], values["rate"]); rawExchangeRate != "" {
		exchangeRate, err = parseRequiredPositiveDecimal("line exchange_rate", rawExchangeRate)
		if err != nil {
			return err
		}
	}

	*l = append(*l, accounting.CreateJournalEntryLineReq{
		AccountID:    accountID,
		Description:  strings.TrimSpace(values["description"]),
		DebitAmount:  debitAmount,
		CreditAmount: creditAmount,
		Currency:     strings.ToUpper(firstNonEmpty(values["currency"], "EUR")),
		ExchangeRate: exchangeRate,
	})
	return nil
}

func (l *journalLineFlags) String() string {
	if l == nil {
		return ""
	}
	accounts := make([]string, 0, len(*l))
	for _, line := range *l {
		accounts = append(accounts, line.AccountID)
	}
	return strings.Join(accounts, ",")
}

type allocationFlags []payments.AllocationRequest

func (a *allocationFlags) Set(value string) error {
	parts := strings.SplitN(strings.TrimSpace(value), ":", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		return errors.New("allocate must be in invoice-id:amount form")
	}
	amount, err := parseRequiredPositiveDecimal("allocation amount", parts[1])
	if err != nil {
		return err
	}
	*a = append(*a, payments.AllocationRequest{
		InvoiceID: strings.TrimSpace(parts[0]),
		Amount:    amount,
	})
	return nil
}

func (a *allocationFlags) String() string {
	if a == nil {
		return ""
	}
	values := make([]string, 0, len(*a))
	for _, allocation := range *a {
		values = append(values, allocation.InvoiceID+":"+allocation.Amount.String())
	}
	return strings.Join(values, ",")
}

func writeExportOutput(w io.Writer, outputPath string, content []byte, description string) error {
	if strings.TrimSpace(outputPath) == "" {
		_, err := w.Write(content)
		return err
	}
	if err := os.WriteFile(outputPath, content, 0o600); err != nil {
		return fmt.Errorf("write %s: %w", outputPath, err)
	}
	_, _ = fmt.Fprintf(w, "Wrote %s to %s\n", description, outputPath)
	return nil
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
