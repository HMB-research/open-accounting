package main

import (
	"bufio"
	"context"
	"encoding/csv"
	"encoding/json"
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
	"github.com/HMB-research/open-accounting/internal/banking"
	"github.com/HMB-research/open-accounting/internal/contacts"
	"github.com/HMB-research/open-accounting/internal/documents"
	"github.com/HMB-research/open-accounting/internal/email"
	"github.com/HMB-research/open-accounting/internal/inventory"
	"github.com/HMB-research/open-accounting/internal/invoicing"
	"github.com/HMB-research/open-accounting/internal/orders"
	"github.com/HMB-research/open-accounting/internal/payments"
	"github.com/HMB-research/open-accounting/internal/payroll"
	"github.com/HMB-research/open-accounting/internal/plugin"
	"github.com/HMB-research/open-accounting/internal/quotes"
	"github.com/HMB-research/open-accounting/internal/recurring"
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
	case "health":
		return a.runHealth(ctx, args[1:])
	case "demo":
		return a.runDemo(ctx, args[1:])
	case "auth":
		return a.runAuth(ctx, args[1:])
	case "tenant", "tenants":
		return a.runTenant(ctx, args[1:])
	case "users":
		return a.runUsers(ctx, args[1:])
	case "invitations":
		return a.runInvitations(ctx, args[1:])
	case "plugins":
		return a.runPlugins(ctx, args[1:])
	case "admin":
		return a.runAdmin(ctx, args[1:])
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
	case "leave":
		return a.runLeave(ctx, args[1:])
	case "tsd":
		return a.runTSD(ctx, args[1:])
	case "tax":
		return a.runTax(ctx, args[1:])
	case "invoices":
		return a.runInvoices(ctx, args[1:])
	case "payments":
		return a.runPayments(ctx, args[1:])
	case "reminders":
		return a.runReminders(ctx, args[1:])
	case "email":
		return a.runEmail(ctx, args[1:])
	case "interest":
		return a.runInterest(ctx, args[1:])
	case "close":
		return a.runClose(ctx, args[1:])
	case "banking":
		return a.runBanking(ctx, args[1:])
	case "quotes":
		return a.runQuotes(ctx, args[1:])
	case "orders":
		return a.runOrders(ctx, args[1:])
	case "recurring-invoices":
		return a.runRecurringInvoices(ctx, args[1:])
	case "assets":
		return a.runAssets(ctx, args[1:])
	case "inventory":
		return a.runInventory(ctx, args[1:])
	case "cost-centers":
		return a.runCostCenters(ctx, args[1:])
	case "analytics":
		return a.runAnalytics(ctx, args[1:])
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
	_, _ = fmt.Fprintln(a.stdout, "  help                      Show CLI usage")
	_, _ = fmt.Fprintln(a.stdout, "  health                    Check API health")
	_, _ = fmt.Fprintln(a.stdout, "  demo status               Show demo data status")
	_, _ = fmt.Fprintln(a.stdout, "  demo reset                Reset demo data")
	_, _ = fmt.Fprintln(a.stdout, "  auth register             Register a user")
	_, _ = fmt.Fprintln(a.stdout, "  auth login                Log in and print JWT tokens")
	_, _ = fmt.Fprintln(a.stdout, "  auth init                 Bootstrap and store a tenant-scoped API token")
	_, _ = fmt.Fprintln(a.stdout, "  auth refresh              Exchange a refresh token for an access token")
	_, _ = fmt.Fprintln(a.stdout, "  auth sessions             List refresh token sessions")
	_, _ = fmt.Fprintln(a.stdout, "  auth revoke-session       Revoke a refresh token session by id")
	_, _ = fmt.Fprintln(a.stdout, "  auth tenants              List tenants for the current token")
	_, _ = fmt.Fprintln(a.stdout, "  auth status               Show current CLI auth status")
	_, _ = fmt.Fprintln(a.stdout, "  auth logout               Revoke a refresh token and remove local CLI config")
	_, _ = fmt.Fprintln(a.stdout, "  tenant get                Show tenant details")
	_, _ = fmt.Fprintln(a.stdout, "  tenant create             Create a tenant")
	_, _ = fmt.Fprintln(a.stdout, "  tenant update             Update tenant settings")
	_, _ = fmt.Fprintln(a.stdout, "  tenant complete-onboarding  Mark onboarding complete")
	_, _ = fmt.Fprintln(a.stdout, "  users list                List tenant users")
	_, _ = fmt.Fprintln(a.stdout, "  users update-role         Update a tenant user role")
	_, _ = fmt.Fprintln(a.stdout, "  users remove              Remove a tenant user")
	_, _ = fmt.Fprintln(a.stdout, "  invitations list          List pending tenant invitations")
	_, _ = fmt.Fprintln(a.stdout, "  invitations create        Invite a user")
	_, _ = fmt.Fprintln(a.stdout, "  invitations revoke        Revoke a pending invitation")
	_, _ = fmt.Fprintln(a.stdout, "  invitations get           Show public invitation details")
	_, _ = fmt.Fprintln(a.stdout, "  invitations accept        Accept an invitation token")
	_, _ = fmt.Fprintln(a.stdout, "  plugins list              List tenant plugins")
	_, _ = fmt.Fprintln(a.stdout, "  plugins enable            Enable a tenant plugin")
	_, _ = fmt.Fprintln(a.stdout, "  plugins disable           Disable a tenant plugin")
	_, _ = fmt.Fprintln(a.stdout, "  plugins settings get      Show tenant plugin settings")
	_, _ = fmt.Fprintln(a.stdout, "  plugins settings update   Update tenant plugin settings")
	_, _ = fmt.Fprintln(a.stdout, "  admin plugins list        List installed plugins")
	_, _ = fmt.Fprintln(a.stdout, "  admin plugins search      Search plugin repositories")
	_, _ = fmt.Fprintln(a.stdout, "  admin plugins get         Show an installed plugin")
	_, _ = fmt.Fprintln(a.stdout, "  admin plugins install     Install a plugin from a repository")
	_, _ = fmt.Fprintln(a.stdout, "  admin plugins permissions List plugin permission names")
	_, _ = fmt.Fprintln(a.stdout, "  admin plugins enable      Enable an installed plugin")
	_, _ = fmt.Fprintln(a.stdout, "  admin plugins disable     Disable an installed plugin")
	_, _ = fmt.Fprintln(a.stdout, "  admin plugins uninstall   Uninstall a plugin")
	_, _ = fmt.Fprintln(a.stdout, "  admin registries list     List plugin registries")
	_, _ = fmt.Fprintln(a.stdout, "  admin registries create   Add a plugin registry")
	_, _ = fmt.Fprintln(a.stdout, "  admin registries delete   Remove a plugin registry")
	_, _ = fmt.Fprintln(a.stdout, "  admin registries sync     Sync a plugin registry")
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
	_, _ = fmt.Fprintln(a.stdout, "  payroll runs get          Show one payroll run")
	_, _ = fmt.Fprintln(a.stdout, "  payroll runs calculate    Calculate payslips for a payroll run")
	_, _ = fmt.Fprintln(a.stdout, "  payroll runs approve      Approve a payroll run")
	_, _ = fmt.Fprintln(a.stdout, "  payroll runs payslips     List payslips for a payroll run")
	_, _ = fmt.Fprintln(a.stdout, "  payroll tax-preview       Preview Estonian payroll taxes")
	_, _ = fmt.Fprintln(a.stdout, "  payroll import-history    Import historical payroll runs from CSV")
	_, _ = fmt.Fprintln(a.stdout, "  payroll import-leave-balances  Import leave balances from CSV")
	_, _ = fmt.Fprintln(a.stdout, "  leave absence-types list  List absence types")
	_, _ = fmt.Fprintln(a.stdout, "  leave absence-types get   Show one absence type")
	_, _ = fmt.Fprintln(a.stdout, "  leave balances list       List employee leave balances")
	_, _ = fmt.Fprintln(a.stdout, "  leave balances by-year    Show one employee leave balance year")
	_, _ = fmt.Fprintln(a.stdout, "  leave balances update     Update an employee leave balance")
	_, _ = fmt.Fprintln(a.stdout, "  leave balances initialize Initialize employee leave balances")
	_, _ = fmt.Fprintln(a.stdout, "  leave balances import     Import leave balances from CSV")
	_, _ = fmt.Fprintln(a.stdout, "  leave records list        List leave records")
	_, _ = fmt.Fprintln(a.stdout, "  leave records create      Create a leave record")
	_, _ = fmt.Fprintln(a.stdout, "  leave records get         Show one leave record")
	_, _ = fmt.Fprintln(a.stdout, "  leave records approve     Approve a leave record")
	_, _ = fmt.Fprintln(a.stdout, "  leave records reject      Reject a leave record")
	_, _ = fmt.Fprintln(a.stdout, "  leave records cancel      Cancel a leave record")
	_, _ = fmt.Fprintln(a.stdout, "  tsd list                  List TSD declarations")
	_, _ = fmt.Fprintln(a.stdout, "  tsd get                   Show one TSD declaration")
	_, _ = fmt.Fprintln(a.stdout, "  tsd generate              Generate TSD from a payroll run")
	_, _ = fmt.Fprintln(a.stdout, "  tsd export-xml            Export TSD XML")
	_, _ = fmt.Fprintln(a.stdout, "  tsd export-csv            Export TSD CSV")
	_, _ = fmt.Fprintln(a.stdout, "  tsd mark-submitted        Mark a TSD declaration submitted")
	_, _ = fmt.Fprintln(a.stdout, "  tax kmd list              List KMD declarations")
	_, _ = fmt.Fprintln(a.stdout, "  tax kmd generate          Generate KMD declaration")
	_, _ = fmt.Fprintln(a.stdout, "  tax kmd import-history    Import historical KMD declarations from CSV")
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
	_, _ = fmt.Fprintln(a.stdout, "  reminders overdue         List overdue invoices for reminders")
	_, _ = fmt.Fprintln(a.stdout, "  reminders send            Send a reminder for one invoice")
	_, _ = fmt.Fprintln(a.stdout, "  reminders send-bulk       Send reminders for multiple invoices")
	_, _ = fmt.Fprintln(a.stdout, "  reminders history         List invoice reminder history")
	_, _ = fmt.Fprintln(a.stdout, "  reminders rules list      List automated reminder rules")
	_, _ = fmt.Fprintln(a.stdout, "  reminders rules create    Create an automated reminder rule")
	_, _ = fmt.Fprintln(a.stdout, "  reminders rules get       Show one automated reminder rule")
	_, _ = fmt.Fprintln(a.stdout, "  reminders rules update    Update an automated reminder rule")
	_, _ = fmt.Fprintln(a.stdout, "  reminders rules delete    Delete an automated reminder rule")
	_, _ = fmt.Fprintln(a.stdout, "  reminders rules trigger   Trigger automated reminders")
	_, _ = fmt.Fprintln(a.stdout, "  email smtp get            Show SMTP email settings")
	_, _ = fmt.Fprintln(a.stdout, "  email smtp update         Update SMTP email settings")
	_, _ = fmt.Fprintln(a.stdout, "  email smtp test           Send a test email")
	_, _ = fmt.Fprintln(a.stdout, "  email templates list      List email templates")
	_, _ = fmt.Fprintln(a.stdout, "  email templates update    Update an email template")
	_, _ = fmt.Fprintln(a.stdout, "  email log                 List email delivery log entries")
	_, _ = fmt.Fprintln(a.stdout, "  email invoice             Send an invoice email")
	_, _ = fmt.Fprintln(a.stdout, "  email payment-receipt     Send a payment receipt email")
	_, _ = fmt.Fprintln(a.stdout, "  interest settings get     Show late-payment interest settings")
	_, _ = fmt.Fprintln(a.stdout, "  interest settings update  Update late-payment interest settings")
	_, _ = fmt.Fprintln(a.stdout, "  interest overdue          List overdue invoices with interest")
	_, _ = fmt.Fprintln(a.stdout, "  interest invoice          Show interest for one invoice")
	_, _ = fmt.Fprintln(a.stdout, "  interest history          Show invoice interest history")
	_, _ = fmt.Fprintln(a.stdout, "  close events              List period close events")
	_, _ = fmt.Fprintln(a.stdout, "  close period              Close an accounting period")
	_, _ = fmt.Fprintln(a.stdout, "  close reopen              Reopen an accounting period")
	_, _ = fmt.Fprintln(a.stdout, "  close year-end-status     Show year-end close readiness")
	_, _ = fmt.Fprintln(a.stdout, "  close carry-forward       Create year-end carry-forward entries")
	_, _ = fmt.Fprintln(a.stdout, "  close reverse-carry-forward Reverse year-end carry-forward entries")
	_, _ = fmt.Fprintln(a.stdout, "  banking accounts list     List bank accounts")
	_, _ = fmt.Fprintln(a.stdout, "  banking accounts create   Create a bank account")
	_, _ = fmt.Fprintln(a.stdout, "  banking accounts get      Show one bank account")
	_, _ = fmt.Fprintln(a.stdout, "  banking accounts update   Update a bank account")
	_, _ = fmt.Fprintln(a.stdout, "  banking accounts delete   Delete a bank account")
	_, _ = fmt.Fprintln(a.stdout, "  banking transactions list List bank transactions")
	_, _ = fmt.Fprintln(a.stdout, "  banking transactions import  Import bank transactions from CSV")
	_, _ = fmt.Fprintln(a.stdout, "  banking transactions import-history  Import historical bank transactions")
	_, _ = fmt.Fprintln(a.stdout, "  banking transactions get  Show one bank transaction")
	_, _ = fmt.Fprintln(a.stdout, "  banking transactions suggestions  List match suggestions")
	_, _ = fmt.Fprintln(a.stdout, "  banking transactions match  Match a bank transaction")
	_, _ = fmt.Fprintln(a.stdout, "  banking transactions unmatch  Remove a transaction match")
	_, _ = fmt.Fprintln(a.stdout, "  banking transactions review  Mark a bank transaction reviewed")
	_, _ = fmt.Fprintln(a.stdout, "  banking transactions create-payment  Create payment from transaction")
	_, _ = fmt.Fprintln(a.stdout, "  banking transactions auto-match  Auto-match bank transactions")
	_, _ = fmt.Fprintln(a.stdout, "  banking reconciliations list  List bank reconciliations")
	_, _ = fmt.Fprintln(a.stdout, "  banking reconciliations create  Create a bank reconciliation")
	_, _ = fmt.Fprintln(a.stdout, "  banking reconciliations get  Show one bank reconciliation")
	_, _ = fmt.Fprintln(a.stdout, "  banking reconciliations complete  Complete a bank reconciliation")
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
	_, _ = fmt.Fprintln(a.stdout, "  recurring-invoices list   List recurring invoice templates")
	_, _ = fmt.Fprintln(a.stdout, "  recurring-invoices create Create a recurring invoice template")
	_, _ = fmt.Fprintln(a.stdout, "  recurring-invoices from-invoice  Create template from an invoice")
	_, _ = fmt.Fprintln(a.stdout, "  recurring-invoices get    Show one recurring invoice template")
	_, _ = fmt.Fprintln(a.stdout, "  recurring-invoices update Update a recurring invoice template")
	_, _ = fmt.Fprintln(a.stdout, "  recurring-invoices delete Delete a recurring invoice template")
	_, _ = fmt.Fprintln(a.stdout, "  recurring-invoices pause  Pause a recurring invoice template")
	_, _ = fmt.Fprintln(a.stdout, "  recurring-invoices resume Resume a recurring invoice template")
	_, _ = fmt.Fprintln(a.stdout, "  recurring-invoices generate  Generate one recurring invoice")
	_, _ = fmt.Fprintln(a.stdout, "  recurring-invoices generate-due  Generate all due recurring invoices")
	_, _ = fmt.Fprintln(a.stdout, "  assets categories list    List fixed asset categories")
	_, _ = fmt.Fprintln(a.stdout, "  assets categories create  Create a fixed asset category")
	_, _ = fmt.Fprintln(a.stdout, "  assets categories get     Show one fixed asset category")
	_, _ = fmt.Fprintln(a.stdout, "  assets categories delete  Delete a fixed asset category")
	_, _ = fmt.Fprintln(a.stdout, "  assets list               List fixed assets")
	_, _ = fmt.Fprintln(a.stdout, "  assets create             Create a fixed asset")
	_, _ = fmt.Fprintln(a.stdout, "  assets import             Import fixed assets from CSV")
	_, _ = fmt.Fprintln(a.stdout, "  assets get                Show one fixed asset")
	_, _ = fmt.Fprintln(a.stdout, "  assets update             Update a fixed asset")
	_, _ = fmt.Fprintln(a.stdout, "  assets delete             Delete a draft fixed asset")
	_, _ = fmt.Fprintln(a.stdout, "  assets activate           Activate a fixed asset")
	_, _ = fmt.Fprintln(a.stdout, "  assets dispose            Dispose or sell a fixed asset")
	_, _ = fmt.Fprintln(a.stdout, "  assets depreciate         Record monthly depreciation")
	_, _ = fmt.Fprintln(a.stdout, "  assets depreciation       List depreciation history")
	_, _ = fmt.Fprintln(a.stdout, "  inventory categories list List product categories")
	_, _ = fmt.Fprintln(a.stdout, "  inventory categories create  Create a product category")
	_, _ = fmt.Fprintln(a.stdout, "  inventory categories get  Show one product category")
	_, _ = fmt.Fprintln(a.stdout, "  inventory categories delete  Delete a product category")
	_, _ = fmt.Fprintln(a.stdout, "  inventory products list   List products and services")
	_, _ = fmt.Fprintln(a.stdout, "  inventory products create Create a product or service")
	_, _ = fmt.Fprintln(a.stdout, "  inventory products get    Show one product or service")
	_, _ = fmt.Fprintln(a.stdout, "  inventory products update Update a product or service")
	_, _ = fmt.Fprintln(a.stdout, "  inventory products delete Delete a product or service")
	_, _ = fmt.Fprintln(a.stdout, "  inventory products stock-levels  List product stock levels")
	_, _ = fmt.Fprintln(a.stdout, "  inventory products movements  List product stock movements")
	_, _ = fmt.Fprintln(a.stdout, "  inventory warehouses list List warehouses")
	_, _ = fmt.Fprintln(a.stdout, "  inventory warehouses create  Create a warehouse")
	_, _ = fmt.Fprintln(a.stdout, "  inventory warehouses get  Show one warehouse")
	_, _ = fmt.Fprintln(a.stdout, "  inventory warehouses update  Update a warehouse")
	_, _ = fmt.Fprintln(a.stdout, "  inventory warehouses delete  Delete a warehouse")
	_, _ = fmt.Fprintln(a.stdout, "  inventory adjust          Adjust product stock")
	_, _ = fmt.Fprintln(a.stdout, "  inventory transfer        Transfer stock between warehouses")
	_, _ = fmt.Fprintln(a.stdout, "  cost-centers list         List cost centers")
	_, _ = fmt.Fprintln(a.stdout, "  cost-centers create       Create a cost center")
	_, _ = fmt.Fprintln(a.stdout, "  cost-centers get          Show one cost center")
	_, _ = fmt.Fprintln(a.stdout, "  cost-centers update       Update a cost center")
	_, _ = fmt.Fprintln(a.stdout, "  cost-centers delete       Delete a cost center")
	_, _ = fmt.Fprintln(a.stdout, "  cost-centers report       Show cost center budget report")
	_, _ = fmt.Fprintln(a.stdout, "  analytics dashboard       Show dashboard summary")
	_, _ = fmt.Fprintln(a.stdout, "  analytics revenue-expense Show revenue and expense chart data")
	_, _ = fmt.Fprintln(a.stdout, "  analytics cash-flow       Show cash-flow chart data")
	_, _ = fmt.Fprintln(a.stdout, "  analytics activity        Show recent activity")
	_, _ = fmt.Fprintln(a.stdout, "  reports trial-balance     Show trial balance")
	_, _ = fmt.Fprintln(a.stdout, "  reports account-balance   Show one account balance")
	_, _ = fmt.Fprintln(a.stdout, "  reports balance-sheet     Show balance sheet")
	_, _ = fmt.Fprintln(a.stdout, "  reports income-statement  Show income statement")
	_, _ = fmt.Fprintln(a.stdout, "  reports cash-flow         Show cash flow statement")
	_, _ = fmt.Fprintln(a.stdout, "  reports aging             Show receivables or payables aging")
	_, _ = fmt.Fprintln(a.stdout, "  reports balance-confirmations  Show balance confirmations")
	_, _ = fmt.Fprintln(a.stdout, "  reports balance-confirmation  Show one balance confirmation")
	_, _ = fmt.Fprintln(a.stdout, "  documents list            List documents for a record")
	_, _ = fmt.Fprintln(a.stdout, "  documents review-summary  Summarize document review state")
	_, _ = fmt.Fprintln(a.stdout, "  documents evidence-policy Evaluate required evidence policy")
	_, _ = fmt.Fprintln(a.stdout, "  documents retention       List retention-due documents")
	_, _ = fmt.Fprintln(a.stdout, "  documents upload          Upload a document to a record")
	_, _ = fmt.Fprintln(a.stdout, "  documents download        Download a document")
	_, _ = fmt.Fprintln(a.stdout, "  documents review          Approve, reject, or mark a document reviewed")
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

func (a *cliApp) runHealth(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("health", flag.ContinueOnError)
	fs.SetOutput(a.stderr)
	baseURL := fs.String("base-url", defaultBaseURL(), "API base URL")
	if err := fs.Parse(args); err != nil {
		return err
	}
	client := newAPIClient(*baseURL, "")
	status, err := client.health(ctx)
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintln(a.stdout, strings.TrimSpace(status))
	return nil
}

func (a *cliApp) runDemo(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("demo subcommand required")
	}

	switch args[0] {
	case "status":
		fs := flag.NewFlagSet("demo status", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		baseURL := fs.String("base-url", defaultBaseURL(), "API base URL")
		secret := fs.String("secret", "", "Demo reset/status secret")
		user := fs.Int("user", 0, "Demo user number")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *user <= 0 {
			return errors.New("user is required")
		}
		client := newAPIClient(*baseURL, "")
		payload, err := client.demoStatus(ctx, *user, *secret)
		if err != nil {
			return err
		}
		return printRawJSON(a.stdout, payload)

	case "reset":
		fs := flag.NewFlagSet("demo reset", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		baseURL := fs.String("base-url", defaultBaseURL(), "API base URL")
		secret := fs.String("secret", "", "Demo reset/status secret")
		user := fs.Int("user", 0, "Optional demo user number; omit for all users")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		client := newAPIClient(*baseURL, "")
		payload, err := client.demoReset(ctx, *user, *secret)
		if err != nil {
			return err
		}
		return printRawJSON(a.stdout, payload)

	default:
		return fmt.Errorf("unknown demo subcommand %q", args[0])
	}
}

func (a *cliApp) runAuth(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("auth subcommand required")
	}

	switch args[0] {
	case "register":
		fs := flag.NewFlagSet("auth register", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		baseURL := fs.String("base-url", defaultBaseURL(), "API base URL")
		email := fs.String("email", "", "User email")
		password := fs.String("password", "", "User password")
		passwordStdin := fs.Bool("password-stdin", false, "Read password from stdin")
		name := fs.String("name", "", "User display name")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*email) == "" || strings.TrimSpace(*name) == "" {
			return errors.New("email and name are required")
		}
		passwordValue, err := resolvePassword(*password, *passwordStdin)
		if err != nil {
			return err
		}
		client := newAPIClient(*baseURL, "")
		user, err := client.register(ctx, strings.TrimSpace(*email), passwordValue, strings.TrimSpace(*name))
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, user)
		}
		_, _ = fmt.Fprintf(a.stdout, "Registered %s <%s> (%s)\n", user.Name, user.Email, user.ID)
		return nil

	case "login":
		fs := flag.NewFlagSet("auth login", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		baseURL := fs.String("base-url", defaultBaseURL(), "API base URL")
		email := fs.String("email", "", "User email")
		password := fs.String("password", "", "User password")
		passwordStdin := fs.Bool("password-stdin", false, "Read password from stdin")
		tenantID := fs.String("tenant-id", "", "Optional tenant id for access-token context")
		asJSON := fs.Bool("json", false, "Output JSON")
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
		resp, err := client.login(ctx, strings.TrimSpace(*email), passwordValue, strings.TrimSpace(*tenantID))
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, resp)
		}
		printLoginResponse(a.stdout, resp)
		return nil

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
		loginResp, err := client.login(ctx, *email, passwordValue, "")
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

	case "refresh":
		fs := flag.NewFlagSet("auth refresh", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		baseURL := fs.String("base-url", defaultBaseURL(), "API base URL")
		refreshToken := fs.String("refresh-token", "", "Refresh token")
		tenantID := fs.String("tenant-id", "", "Optional tenant id for access-token context")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*refreshToken) == "" {
			return errors.New("refresh-token is required")
		}
		client := newAPIClient(*baseURL, "")
		resp, err := client.refreshAccessToken(ctx, strings.TrimSpace(*refreshToken), strings.TrimSpace(*tenantID))
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, resp)
		}
		printLoginResponse(a.stdout, resp)
		return nil

	case "tenants":
		cfg, client, err := a.loadTokenClient()
		if err != nil {
			return err
		}

		fs := flag.NewFlagSet("auth tenants", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		memberships, err := client.listMyTenants(ctx, cfg.APIToken)
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, memberships)
		}
		printTenantMembershipsTable(a.stdout, memberships)
		return nil

	case "sessions":
		cfg, client, err := a.loadTokenClient()
		if err != nil {
			return err
		}

		fs := flag.NewFlagSet("auth sessions", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		includeInactive := fs.Bool("include-inactive", false, "Include revoked and expired sessions")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		sessions, err := client.listAuthSessions(ctx, *includeInactive, cfg.APIToken)
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, sessions)
		}
		printRefreshSessions(a.stdout, sessions)
		return nil

	case "revoke-session":
		cfg, client, err := a.loadTokenClient()
		if err != nil {
			return err
		}

		fs := flag.NewFlagSet("auth revoke-session", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		sessionID := fs.String("id", "", "Refresh session id")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*sessionID) == "" {
			return errors.New("id is required")
		}
		if err := client.revokeAuthSession(ctx, strings.TrimSpace(*sessionID), cfg.APIToken); err != nil {
			return err
		}
		_, _ = fmt.Fprintln(a.stdout, "Revoked refresh session")
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
		fs := flag.NewFlagSet("auth logout", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		baseURL := fs.String("base-url", defaultBaseURL(), "API base URL")
		refreshToken := fs.String("refresh-token", "", "Refresh token to revoke on the server")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*refreshToken) != "" {
			client := newAPIClient(*baseURL, "")
			if err := client.logout(ctx, strings.TrimSpace(*refreshToken)); err != nil {
				return err
			}
			_, _ = fmt.Fprintln(a.stdout, "Revoked refresh session")
		}
		if err := deleteConfig(); err != nil {
			return err
		}
		_, _ = fmt.Fprintln(a.stdout, "Removed local CLI config")
		return nil

	default:
		return fmt.Errorf("unknown auth subcommand %q", args[0])
	}
}

func (a *cliApp) runTenant(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("tenant subcommand required")
	}

	switch args[0] {
	case "create":
		_, client, err := a.loadTokenClient()
		if err != nil {
			return err
		}

		fs := flag.NewFlagSet("tenant create", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		name := fs.String("name", "", "Tenant name")
		slug := fs.String("slug", "", "Tenant slug")
		settingsJSON := fs.String("settings-json", "", "Tenant settings JSON object")
		settingsFile := fs.String("settings-file", "", "Path to tenant settings JSON file")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*name) == "" || strings.TrimSpace(*slug) == "" {
			return errors.New("name and slug are required")
		}
		settings, err := parseTenantSettingsInput(*settingsJSON, *settingsFile)
		if err != nil {
			return err
		}

		created, err := client.createTenant(ctx, &tenant.CreateTenantRequest{
			Name:     strings.TrimSpace(*name),
			Slug:     strings.TrimSpace(*slug),
			Settings: settings,
		})
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, created)
		}
		printTenant(a.stdout, created)
		return nil

	case "get":
		cfg, client, err := a.loadAuthenticatedClient()
		if err != nil {
			return err
		}

		fs := flag.NewFlagSet("tenant get", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		tenantID := fs.String("id", "", "Tenant id; defaults to configured tenant")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		targetTenantID := strings.TrimSpace(*tenantID)
		if targetTenantID == "" {
			targetTenantID = cfg.TenantID
		}

		record, err := client.getTenant(ctx, targetTenantID)
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, record)
		}
		printTenant(a.stdout, record)
		return nil

	case "update":
		cfg, client, err := a.loadAuthenticatedClient()
		if err != nil {
			return err
		}

		fs := flag.NewFlagSet("tenant update", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		tenantID := fs.String("id", "", "Tenant id; defaults to configured tenant")
		name := fs.String("name", "", "New tenant name")
		settingsJSON := fs.String("settings-json", "", "Tenant settings JSON object")
		settingsFile := fs.String("settings-file", "", "Path to tenant settings JSON file")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		targetTenantID := strings.TrimSpace(*tenantID)
		if targetTenantID == "" {
			targetTenantID = cfg.TenantID
		}
		settings, err := parseTenantSettingsInput(*settingsJSON, *settingsFile)
		if err != nil {
			return err
		}
		nameValue := strings.TrimSpace(*name)
		if nameValue == "" && settings == nil {
			return errors.New("name, settings-json, or settings-file is required")
		}
		req := &tenant.UpdateTenantRequest{Settings: settings}
		if nameValue != "" {
			req.Name = &nameValue
		}

		updated, err := client.updateTenant(ctx, targetTenantID, req)
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, updated)
		}
		printTenant(a.stdout, updated)
		return nil

	case "complete-onboarding":
		cfg, client, err := a.loadAuthenticatedClient()
		if err != nil {
			return err
		}

		fs := flag.NewFlagSet("tenant complete-onboarding", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		tenantID := fs.String("id", "", "Tenant id; defaults to configured tenant")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		targetTenantID := strings.TrimSpace(*tenantID)
		if targetTenantID == "" {
			targetTenantID = cfg.TenantID
		}

		if err := client.completeTenantOnboarding(ctx, targetTenantID); err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, map[string]any{"success": true, "tenant_id": targetTenantID})
		}
		_, _ = fmt.Fprintf(a.stdout, "Marked tenant %s onboarding complete\n", targetTenantID)
		return nil

	default:
		return fmt.Errorf("unknown tenant subcommand %q", args[0])
	}
}

func (a *cliApp) runUsers(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("users subcommand required")
	}
	cfg, client, err := a.loadAuthenticatedClient()
	if err != nil {
		return err
	}

	switch args[0] {
	case "list":
		fs := flag.NewFlagSet("users list", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}

		users, err := client.listTenantUsers(ctx, cfg.TenantID)
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, users)
		}
		printTenantUsersTable(a.stdout, users)
		return nil

	case "update-role":
		fs := flag.NewFlagSet("users update-role", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		userID := fs.String("id", "", "User id")
		role := fs.String("role", "", "New role: admin, accountant, viewer")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*userID) == "" || strings.TrimSpace(*role) == "" {
			return errors.New("id and role are required")
		}

		if err := client.updateTenantUserRole(ctx, cfg.TenantID, strings.TrimSpace(*userID), strings.TrimSpace(*role)); err != nil {
			return err
		}
		_, _ = fmt.Fprintf(a.stdout, "Updated user %s role to %s\n", strings.TrimSpace(*userID), strings.TrimSpace(*role))
		return nil

	case "remove":
		fs := flag.NewFlagSet("users remove", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		userID := fs.String("id", "", "User id")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*userID) == "" {
			return errors.New("id is required")
		}

		if err := client.removeTenantUser(ctx, cfg.TenantID, strings.TrimSpace(*userID)); err != nil {
			return err
		}
		_, _ = fmt.Fprintf(a.stdout, "Removed user %s\n", strings.TrimSpace(*userID))
		return nil

	default:
		return fmt.Errorf("unknown users subcommand %q", args[0])
	}
}

func (a *cliApp) runInvitations(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("invitations subcommand required")
	}

	switch args[0] {
	case "list":
		cfg, client, err := a.loadAuthenticatedClient()
		if err != nil {
			return err
		}

		fs := flag.NewFlagSet("invitations list", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}

		invitations, err := client.listInvitations(ctx, cfg.TenantID)
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, invitations)
		}
		printInvitationsTable(a.stdout, invitations)
		return nil

	case "create":
		cfg, client, err := a.loadAuthenticatedClient()
		if err != nil {
			return err
		}

		fs := flag.NewFlagSet("invitations create", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		email := fs.String("email", "", "Invitee email")
		role := fs.String("role", tenant.RoleViewer, "Role: admin, accountant, viewer")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*email) == "" || strings.TrimSpace(*role) == "" {
			return errors.New("email and role are required")
		}

		invitation, err := client.createInvitation(ctx, cfg.TenantID, &tenant.CreateInvitationRequest{
			Email: strings.TrimSpace(*email),
			Role:  strings.TrimSpace(*role),
		})
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, invitation)
		}
		_, _ = fmt.Fprintf(a.stdout, "Invited %s as %s (%s)\n", invitation.Email, invitation.Role, invitation.ID)
		return nil

	case "revoke":
		cfg, client, err := a.loadAuthenticatedClient()
		if err != nil {
			return err
		}

		fs := flag.NewFlagSet("invitations revoke", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		invitationID := fs.String("id", "", "Invitation id")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*invitationID) == "" {
			return errors.New("id is required")
		}

		if err := client.revokeInvitation(ctx, cfg.TenantID, strings.TrimSpace(*invitationID)); err != nil {
			return err
		}
		_, _ = fmt.Fprintf(a.stdout, "Revoked invitation %s\n", strings.TrimSpace(*invitationID))
		return nil

	case "get":
		fs := flag.NewFlagSet("invitations get", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		token := fs.String("token", "", "Invitation token")
		baseURL := fs.String("base-url", "", "API base URL; defaults to config or OA_BASE_URL")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*token) == "" {
			return errors.New("token is required")
		}
		client, err := a.loadPublicClient(*baseURL)
		if err != nil {
			return err
		}

		invitation, err := client.getInvitationByToken(ctx, strings.TrimSpace(*token))
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, invitation)
		}
		printInvitationsTable(a.stdout, []tenant.UserInvitation{*invitation})
		return nil

	case "accept":
		fs := flag.NewFlagSet("invitations accept", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		token := fs.String("token", "", "Invitation token")
		password := fs.String("password", "", "Password for a new user")
		passwordStdin := fs.Bool("password-stdin", false, "Read password from stdin")
		name := fs.String("name", "", "Name for a new user")
		baseURL := fs.String("base-url", "", "API base URL; defaults to config or OA_BASE_URL")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*token) == "" {
			return errors.New("token is required")
		}
		passwordValue := ""
		if strings.TrimSpace(*password) != "" || *passwordStdin {
			var err error
			passwordValue, err = resolvePassword(*password, *passwordStdin)
			if err != nil {
				return err
			}
		}
		client, err := a.loadPublicClient(*baseURL)
		if err != nil {
			return err
		}

		membership, err := client.acceptInvitation(ctx, &tenant.AcceptInvitationRequest{
			Token:    strings.TrimSpace(*token),
			Password: passwordValue,
			Name:     strings.TrimSpace(*name),
		})
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, membership)
		}
		printTenantMembership(a.stdout, membership)
		return nil

	default:
		return fmt.Errorf("unknown invitations subcommand %q", args[0])
	}
}

func (a *cliApp) runPlugins(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("plugins subcommand required")
	}
	cfg, client, err := a.loadAuthenticatedClient()
	if err != nil {
		return err
	}

	switch args[0] {
	case "list":
		fs := flag.NewFlagSet("plugins list", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}

		plugins, err := client.listTenantPlugins(ctx, cfg.TenantID)
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, plugins)
		}
		printTenantPluginsTable(a.stdout, plugins)
		return nil

	case "enable":
		fs := flag.NewFlagSet("plugins enable", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		pluginID := fs.String("id", "", "Plugin id")
		settingsJSON := fs.String("settings-json", "", "Tenant plugin settings JSON object")
		settingsFile := fs.String("settings-file", "", "Path to tenant plugin settings JSON file")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*pluginID) == "" {
			return errors.New("id is required")
		}
		settings, err := parseRawJSONInput(*settingsJSON, *settingsFile, "{}")
		if err != nil {
			return err
		}

		if err := client.enableTenantPlugin(ctx, cfg.TenantID, strings.TrimSpace(*pluginID), settings); err != nil {
			return err
		}
		_, _ = fmt.Fprintf(a.stdout, "Enabled tenant plugin %s\n", strings.TrimSpace(*pluginID))
		return nil

	case "disable":
		fs := flag.NewFlagSet("plugins disable", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		pluginID := fs.String("id", "", "Plugin id")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*pluginID) == "" {
			return errors.New("id is required")
		}

		if err := client.disableTenantPlugin(ctx, cfg.TenantID, strings.TrimSpace(*pluginID)); err != nil {
			return err
		}
		_, _ = fmt.Fprintf(a.stdout, "Disabled tenant plugin %s\n", strings.TrimSpace(*pluginID))
		return nil

	case "settings":
		return a.runPluginSettings(ctx, cfg, client, args[1:])

	default:
		return fmt.Errorf("unknown plugins subcommand %q", args[0])
	}
}

func (a *cliApp) runPluginSettings(ctx context.Context, cfg *cliConfig, client *apiClient, args []string) error {
	if len(args) == 0 {
		return errors.New("plugins settings subcommand required")
	}

	switch args[0] {
	case "get":
		fs := flag.NewFlagSet("plugins settings get", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		pluginID := fs.String("id", "", "Plugin id")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*pluginID) == "" {
			return errors.New("id is required")
		}

		settings, err := client.getTenantPluginSettings(ctx, cfg.TenantID, strings.TrimSpace(*pluginID))
		if err != nil {
			return err
		}
		var value any
		if err := json.Unmarshal(settings, &value); err == nil {
			return printJSON(a.stdout, value)
		}
		_, err = fmt.Fprintln(a.stdout, string(settings))
		return err

	case "update":
		fs := flag.NewFlagSet("plugins settings update", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		pluginID := fs.String("id", "", "Plugin id")
		settingsJSON := fs.String("settings-json", "", "Tenant plugin settings JSON object")
		settingsFile := fs.String("settings-file", "", "Path to tenant plugin settings JSON file")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*pluginID) == "" {
			return errors.New("id is required")
		}
		settings, err := parseRawJSONInput(*settingsJSON, *settingsFile, "")
		if err != nil {
			return err
		}
		if len(settings) == 0 {
			return errors.New("settings-json or settings-file is required")
		}

		if err := client.updateTenantPluginSettings(ctx, cfg.TenantID, strings.TrimSpace(*pluginID), settings); err != nil {
			return err
		}
		_, _ = fmt.Fprintf(a.stdout, "Updated tenant plugin %s settings\n", strings.TrimSpace(*pluginID))
		return nil

	default:
		return fmt.Errorf("unknown plugins settings subcommand %q", args[0])
	}
}

func (a *cliApp) runAdmin(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("admin subcommand required")
	}
	_, client, err := a.loadTokenClient()
	if err != nil {
		return err
	}

	switch args[0] {
	case "plugins":
		return a.runAdminPlugins(ctx, client, args[1:])
	case "registries", "plugin-registries":
		return a.runAdminPluginRegistries(ctx, client, args[1:])
	default:
		return fmt.Errorf("unknown admin subcommand %q", args[0])
	}
}

func (a *cliApp) runAdminPluginRegistries(ctx context.Context, client *apiClient, args []string) error {
	if len(args) == 0 {
		return errors.New("admin registries subcommand required")
	}

	switch args[0] {
	case "list":
		fs := flag.NewFlagSet("admin registries list", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		registries, err := client.listPluginRegistries(ctx)
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, registries)
		}
		printPluginRegistriesTable(a.stdout, registries)
		return nil

	case "create":
		fs := flag.NewFlagSet("admin registries create", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		name := fs.String("name", "", "Registry name")
		registryURL := fs.String("url", "", "Registry URL")
		description := fs.String("description", "", "Registry description")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*name) == "" || strings.TrimSpace(*registryURL) == "" {
			return errors.New("name and url are required")
		}
		registry, err := client.addPluginRegistry(ctx, &plugin.CreateRegistryRequest{
			Name:        strings.TrimSpace(*name),
			URL:         strings.TrimSpace(*registryURL),
			Description: strings.TrimSpace(*description),
		})
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, registry)
		}
		printPluginRegistriesTable(a.stdout, []plugin.Registry{*registry})
		return nil

	case "delete", "remove":
		fs := flag.NewFlagSet("admin registries delete", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		registryID := fs.String("id", "", "Registry id")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*registryID) == "" {
			return errors.New("id is required")
		}
		if err := client.removePluginRegistry(ctx, strings.TrimSpace(*registryID)); err != nil {
			return err
		}
		_, _ = fmt.Fprintf(a.stdout, "Removed plugin registry %s\n", strings.TrimSpace(*registryID))
		return nil

	case "sync":
		fs := flag.NewFlagSet("admin registries sync", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		registryID := fs.String("id", "", "Registry id")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*registryID) == "" {
			return errors.New("id is required")
		}
		if err := client.syncPluginRegistry(ctx, strings.TrimSpace(*registryID)); err != nil {
			return err
		}
		_, _ = fmt.Fprintf(a.stdout, "Synced plugin registry %s\n", strings.TrimSpace(*registryID))
		return nil

	default:
		return fmt.Errorf("unknown admin registries subcommand %q", args[0])
	}
}

func (a *cliApp) runAdminPlugins(ctx context.Context, client *apiClient, args []string) error {
	if len(args) == 0 {
		return errors.New("admin plugins subcommand required")
	}

	switch args[0] {
	case "list":
		fs := flag.NewFlagSet("admin plugins list", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		plugins, err := client.listAdminPlugins(ctx)
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, plugins)
		}
		printPluginsTable(a.stdout, plugins)
		return nil

	case "search":
		fs := flag.NewFlagSet("admin plugins search", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		query := fs.String("q", "", "Search query")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*query) == "" {
			return errors.New("q is required")
		}
		results, err := client.searchAdminPlugins(ctx, strings.TrimSpace(*query))
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, results)
		}
		printPluginSearchResultsTable(a.stdout, results)
		return nil

	case "get":
		fs := flag.NewFlagSet("admin plugins get", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		pluginID := fs.String("id", "", "Plugin id")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*pluginID) == "" {
			return errors.New("id is required")
		}
		item, err := client.getAdminPlugin(ctx, strings.TrimSpace(*pluginID))
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, item)
		}
		printPluginsTable(a.stdout, []plugin.Plugin{*item})
		return nil

	case "install":
		fs := flag.NewFlagSet("admin plugins install", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		repositoryURL := fs.String("repository-url", "", "Plugin repository URL")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*repositoryURL) == "" {
			return errors.New("repository-url is required")
		}
		item, err := client.installAdminPlugin(ctx, &plugin.InstallPluginRequest{RepositoryURL: strings.TrimSpace(*repositoryURL)})
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, item)
		}
		printPluginsTable(a.stdout, []plugin.Plugin{*item})
		return nil

	case "permissions":
		fs := flag.NewFlagSet("admin plugins permissions", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		permissions, err := client.listPluginPermissions(ctx)
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, permissions)
		}
		printPluginPermissionsTable(a.stdout, permissions)
		return nil

	case "enable":
		fs := flag.NewFlagSet("admin plugins enable", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		pluginID := fs.String("id", "", "Plugin id")
		permissions := stringListFlags{}
		fs.Var(&permissions, "permission", "Permission to grant; repeatable")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*pluginID) == "" {
			return errors.New("id is required")
		}
		if err := client.enableAdminPlugin(ctx, strings.TrimSpace(*pluginID), []string(permissions)); err != nil {
			return err
		}
		_, _ = fmt.Fprintf(a.stdout, "Enabled plugin %s\n", strings.TrimSpace(*pluginID))
		return nil

	case "disable":
		fs := flag.NewFlagSet("admin plugins disable", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		pluginID := fs.String("id", "", "Plugin id")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*pluginID) == "" {
			return errors.New("id is required")
		}
		if err := client.disableAdminPlugin(ctx, strings.TrimSpace(*pluginID)); err != nil {
			return err
		}
		_, _ = fmt.Fprintf(a.stdout, "Disabled plugin %s\n", strings.TrimSpace(*pluginID))
		return nil

	case "uninstall":
		fs := flag.NewFlagSet("admin plugins uninstall", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		pluginID := fs.String("id", "", "Plugin id")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*pluginID) == "" {
			return errors.New("id is required")
		}
		if err := client.uninstallAdminPlugin(ctx, strings.TrimSpace(*pluginID)); err != nil {
			return err
		}
		_, _ = fmt.Fprintf(a.stdout, "Uninstalled plugin %s\n", strings.TrimSpace(*pluginID))
		return nil

	default:
		return fmt.Errorf("unknown admin plugins subcommand %q", args[0])
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

func (a *cliApp) runReminders(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("reminders subcommand required")
	}
	cfg, client, err := a.loadAuthenticatedClient()
	if err != nil {
		return err
	}

	switch args[0] {
	case "overdue":
		fs := flag.NewFlagSet("reminders overdue", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}

		summary, err := client.getOverdueInvoices(ctx, cfg.TenantID)
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, summary)
		}
		printOverdueInvoicesSummary(a.stdout, summary)
		return nil

	case "send":
		fs := flag.NewFlagSet("reminders send", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		invoiceID := fs.String("invoice-id", "", "Invoice id")
		message := fs.String("message", "", "Optional reminder message")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*invoiceID) == "" {
			return errors.New("invoice-id is required")
		}

		result, err := client.sendPaymentReminder(ctx, cfg.TenantID, &invoicing.SendReminderRequest{
			InvoiceID: strings.TrimSpace(*invoiceID),
			Message:   strings.TrimSpace(*message),
		})
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, result)
		}
		printReminderResult(a.stdout, result)
		return nil

	case "send-bulk":
		fs := flag.NewFlagSet("reminders send-bulk", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		invoiceIDs := stringListFlags{}
		fs.Var(&invoiceIDs, "invoice-id", "Invoice id; repeatable")
		message := fs.String("message", "", "Optional reminder message")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if len(invoiceIDs) == 0 {
			return errors.New("at least one invoice-id is required")
		}

		result, err := client.sendBulkPaymentReminders(ctx, cfg.TenantID, &invoicing.SendBulkRemindersRequest{
			InvoiceIDs: invoiceIDs,
			Message:    strings.TrimSpace(*message),
		})
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, result)
		}
		printBulkReminderResult(a.stdout, result)
		return nil

	case "history":
		fs := flag.NewFlagSet("reminders history", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		invoiceID := fs.String("invoice-id", "", "Invoice id")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*invoiceID) == "" {
			return errors.New("invoice-id is required")
		}

		reminders, err := client.listInvoiceReminderHistory(ctx, cfg.TenantID, strings.TrimSpace(*invoiceID))
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, reminders)
		}
		printPaymentRemindersTable(a.stdout, reminders)
		return nil

	case "rules":
		return a.runReminderRules(ctx, cfg, client, args[1:])

	default:
		return fmt.Errorf("unknown reminders subcommand %q", args[0])
	}
}

func (a *cliApp) runReminderRules(ctx context.Context, cfg *cliConfig, client *apiClient, args []string) error {
	if len(args) == 0 {
		return errors.New("reminders rules subcommand required")
	}

	switch args[0] {
	case "list":
		fs := flag.NewFlagSet("reminders rules list", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}

		rules, err := client.listReminderRules(ctx, cfg.TenantID)
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, rules)
		}
		printReminderRulesTable(a.stdout, rules)
		return nil

	case "create":
		fs := flag.NewFlagSet("reminders rules create", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		name := fs.String("name", "", "Rule name")
		triggerTypeFlag := fs.String("trigger-type", "", "Trigger type: BEFORE_DUE, ON_DUE, AFTER_DUE")
		daysOffsetFlag := fs.String("days-offset", "", "Days from due date")
		templateType := fs.String("template-type", "OVERDUE_REMINDER", "Email template type")
		activeFlag := fs.String("active", "true", "Set active state: true or false")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*name) == "" {
			return errors.New("name is required")
		}
		triggerType, err := parseRequiredReminderTriggerType(*triggerTypeFlag)
		if err != nil {
			return err
		}
		daysOffset, err := parseRequiredNonNegativeInt("days-offset", *daysOffsetFlag)
		if err != nil {
			return err
		}
		active, err := strconv.ParseBool(strings.TrimSpace(*activeFlag))
		if err != nil {
			return fmt.Errorf("parse active: %w", err)
		}

		rule, err := client.createReminderRule(ctx, cfg.TenantID, &invoicing.CreateReminderRuleRequest{
			Name:              strings.TrimSpace(*name),
			TriggerType:       triggerType,
			DaysOffset:        daysOffset,
			EmailTemplateType: strings.TrimSpace(*templateType),
			IsActive:          active,
		})
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, rule)
		}
		_, _ = fmt.Fprintf(a.stdout, "Created reminder rule %s (%s)\n", rule.Name, rule.ID)
		return nil

	case "get":
		fs := flag.NewFlagSet("reminders rules get", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		ruleID := fs.String("id", "", "Reminder rule id")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*ruleID) == "" {
			return errors.New("id is required")
		}

		rule, err := client.getReminderRule(ctx, cfg.TenantID, strings.TrimSpace(*ruleID))
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, rule)
		}
		printReminderRule(a.stdout, rule)
		return nil

	case "update":
		fs := flag.NewFlagSet("reminders rules update", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		ruleID := fs.String("id", "", "Reminder rule id")
		name := fs.String("name", "", "Rule name")
		templateType := fs.String("template-type", "", "Email template type")
		activeFlag := fs.String("active", "", "Set active state: true or false")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*ruleID) == "" {
			return errors.New("id is required")
		}
		active, err := parseOptionalBoolPtr("active", *activeFlag)
		if err != nil {
			return err
		}
		req := &invoicing.UpdateReminderRuleRequest{
			Name:              optionalStringPtr(*name),
			EmailTemplateType: optionalStringPtr(*templateType),
			IsActive:          active,
		}
		if req.Name == nil && req.EmailTemplateType == nil && req.IsActive == nil {
			return errors.New("name, template-type, or active is required")
		}

		rule, err := client.updateReminderRule(ctx, cfg.TenantID, strings.TrimSpace(*ruleID), req)
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, rule)
		}
		printReminderRule(a.stdout, rule)
		return nil

	case "delete":
		fs := flag.NewFlagSet("reminders rules delete", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		ruleID := fs.String("id", "", "Reminder rule id")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*ruleID) == "" {
			return errors.New("id is required")
		}

		if err := client.deleteReminderRule(ctx, cfg.TenantID, strings.TrimSpace(*ruleID)); err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, map[string]string{"status": "deleted"})
		}
		_, _ = fmt.Fprintf(a.stdout, "Deleted reminder rule %s\n", strings.TrimSpace(*ruleID))
		return nil

	case "trigger":
		fs := flag.NewFlagSet("reminders rules trigger", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}

		results, err := client.triggerReminderRules(ctx, cfg.TenantID)
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, results)
		}
		printAutomatedReminderResultsTable(a.stdout, results)
		return nil

	default:
		return fmt.Errorf("unknown reminders rules subcommand %q", args[0])
	}
}

func (a *cliApp) runEmail(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("email subcommand required")
	}
	cfg, client, err := a.loadAuthenticatedClient()
	if err != nil {
		return err
	}

	switch args[0] {
	case "smtp":
		return a.runEmailSMTP(ctx, cfg, client, args[1:])
	case "templates":
		return a.runEmailTemplates(ctx, cfg, client, args[1:])
	case "log":
		fs := flag.NewFlagSet("email log", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		limitFlag := fs.String("limit", "50", "Number of email log entries")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		limit, err := parseRequiredPositiveInt("limit", *limitFlag)
		if err != nil {
			return err
		}

		logs, err := client.listEmailLog(ctx, cfg.TenantID, limit)
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, logs)
		}
		printEmailLogsTable(a.stdout, logs)
		return nil
	case "invoice":
		fs := flag.NewFlagSet("email invoice", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		invoiceID := fs.String("invoice-id", "", "Invoice id")
		recipientEmail := fs.String("recipient-email", "", "Recipient email")
		recipientName := fs.String("recipient-name", "", "Recipient name")
		subject := fs.String("subject", "", "Email subject override")
		message := fs.String("message", "", "Email message")
		attachPDF := fs.Bool("attach-pdf", false, "Attach invoice PDF")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*invoiceID) == "" {
			return errors.New("invoice-id is required")
		}
		if strings.TrimSpace(*recipientEmail) == "" {
			return errors.New("recipient-email is required")
		}

		result, err := client.emailInvoice(ctx, cfg.TenantID, strings.TrimSpace(*invoiceID), &email.SendInvoiceRequest{
			RecipientEmail: strings.TrimSpace(*recipientEmail),
			RecipientName:  strings.TrimSpace(*recipientName),
			Subject:        strings.TrimSpace(*subject),
			Message:        strings.TrimSpace(*message),
			AttachPDF:      *attachPDF,
		})
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, result)
		}
		printEmailSentResponse(a.stdout, result)
		return nil
	case "payment-receipt":
		fs := flag.NewFlagSet("email payment-receipt", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		paymentID := fs.String("payment-id", "", "Payment id")
		recipientEmail := fs.String("recipient-email", "", "Recipient email")
		recipientName := fs.String("recipient-name", "", "Recipient name")
		subject := fs.String("subject", "", "Email subject override")
		message := fs.String("message", "", "Email message")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*paymentID) == "" {
			return errors.New("payment-id is required")
		}
		if strings.TrimSpace(*recipientEmail) == "" {
			return errors.New("recipient-email is required")
		}

		result, err := client.emailPaymentReceipt(ctx, cfg.TenantID, strings.TrimSpace(*paymentID), &email.SendPaymentReceiptRequest{
			RecipientEmail: strings.TrimSpace(*recipientEmail),
			RecipientName:  strings.TrimSpace(*recipientName),
			Subject:        strings.TrimSpace(*subject),
			Message:        strings.TrimSpace(*message),
		})
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, result)
		}
		printEmailSentResponse(a.stdout, result)
		return nil
	default:
		return fmt.Errorf("unknown email subcommand %q", args[0])
	}
}

func (a *cliApp) runEmailSMTP(ctx context.Context, cfg *cliConfig, client *apiClient, args []string) error {
	if len(args) == 0 {
		return errors.New("email smtp subcommand required")
	}

	switch args[0] {
	case "get":
		fs := flag.NewFlagSet("email smtp get", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}

		config, err := client.getSMTPConfig(ctx, cfg.TenantID)
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, config)
		}
		printSMTPConfig(a.stdout, config)
		return nil
	case "update":
		fs := flag.NewFlagSet("email smtp update", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		host := fs.String("host", "", "SMTP host")
		portFlag := fs.String("port", "587", "SMTP port")
		username := fs.String("username", "", "SMTP username")
		password := fs.String("password", "", "SMTP password")
		fromEmail := fs.String("from-email", "", "From email address")
		fromName := fs.String("from-name", "", "From display name")
		useTLS := fs.Bool("use-tls", true, "Use TLS")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*host) == "" {
			return errors.New("host is required")
		}
		if strings.TrimSpace(*fromEmail) == "" {
			return errors.New("from-email is required")
		}
		portValue, err := parseRequiredPositiveInt("port", *portFlag)
		if err != nil {
			return err
		}

		req := &email.UpdateSMTPConfigRequest{
			Host:      strings.TrimSpace(*host),
			Port:      portValue,
			Username:  strings.TrimSpace(*username),
			Password:  *password,
			FromEmail: strings.TrimSpace(*fromEmail),
			FromName:  strings.TrimSpace(*fromName),
			UseTLS:    *useTLS,
		}
		if err := client.updateSMTPConfig(ctx, cfg.TenantID, req); err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, map[string]string{"status": "updated"})
		}
		_, _ = fmt.Fprintln(a.stdout, "Updated SMTP configuration")
		return nil
	case "test":
		fs := flag.NewFlagSet("email smtp test", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		recipientEmail := fs.String("recipient-email", "", "Recipient email")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*recipientEmail) == "" {
			return errors.New("recipient-email is required")
		}

		result, err := client.testSMTP(ctx, cfg.TenantID, &email.TestSMTPRequest{RecipientEmail: strings.TrimSpace(*recipientEmail)})
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, result)
		}
		printSMTPTestResponse(a.stdout, result)
		return nil
	default:
		return fmt.Errorf("unknown email smtp subcommand %q", args[0])
	}
}

func (a *cliApp) runEmailTemplates(ctx context.Context, cfg *cliConfig, client *apiClient, args []string) error {
	if len(args) == 0 {
		return errors.New("email templates subcommand required")
	}

	switch args[0] {
	case "list":
		fs := flag.NewFlagSet("email templates list", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}

		templates, err := client.listEmailTemplates(ctx, cfg.TenantID)
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, templates)
		}
		printEmailTemplatesTable(a.stdout, templates)
		return nil
	case "update":
		fs := flag.NewFlagSet("email templates update", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		templateTypeFlag := fs.String("type", "", "Template type: INVOICE_SEND, PAYMENT_RECEIPT, OVERDUE_REMINDER")
		subject := fs.String("subject", "", "Email subject template")
		bodyHTML := fs.String("body-html", "", "HTML body template")
		bodyHTMLFile := fs.String("body-html-file", "", "Read HTML body template from file or '-'")
		bodyText := fs.String("body-text", "", "Plain text body template")
		bodyTextFile := fs.String("body-text-file", "", "Read plain text body template from file or '-'")
		activeFlag := fs.String("active", "true", "Set active state: true or false")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		templateType, err := parseRequiredEmailTemplateType(*templateTypeFlag)
		if err != nil {
			return err
		}
		if strings.TrimSpace(*subject) == "" {
			return errors.New("subject is required")
		}
		resolvedBodyHTML, err := resolveTextFlag("body-html", *bodyHTML, *bodyHTMLFile)
		if err != nil {
			return err
		}
		if strings.TrimSpace(resolvedBodyHTML) == "" {
			return errors.New("body-html is required")
		}
		resolvedBodyText, err := resolveTextFlag("body-text", *bodyText, *bodyTextFile)
		if err != nil {
			return err
		}
		active, err := strconv.ParseBool(strings.TrimSpace(*activeFlag))
		if err != nil {
			return fmt.Errorf("parse active: %w", err)
		}

		template, err := client.updateEmailTemplate(ctx, cfg.TenantID, templateType, &email.UpdateTemplateRequest{
			Subject:  strings.TrimSpace(*subject),
			BodyHTML: resolvedBodyHTML,
			BodyText: resolvedBodyText,
			IsActive: active,
		})
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, template)
		}
		printEmailTemplate(a.stdout, template)
		return nil
	default:
		return fmt.Errorf("unknown email templates subcommand %q", args[0])
	}
}

func (a *cliApp) runInterest(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("interest subcommand required")
	}
	cfg, client, err := a.loadAuthenticatedClient()
	if err != nil {
		return err
	}

	switch args[0] {
	case "settings":
		return a.runInterestSettings(ctx, cfg, client, args[1:])
	case "overdue":
		fs := flag.NewFlagSet("interest overdue", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}

		results, err := client.listOverdueInvoicesWithInterest(ctx, cfg.TenantID)
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, results)
		}
		printInterestCalculationsTable(a.stdout, results)
		return nil
	case "invoice":
		fs := flag.NewFlagSet("interest invoice", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		invoiceID := fs.String("invoice-id", "", "Invoice id")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*invoiceID) == "" {
			return errors.New("invoice-id is required")
		}

		result, err := client.getInvoiceInterest(ctx, cfg.TenantID, strings.TrimSpace(*invoiceID))
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, result)
		}
		printInterestCalculation(a.stdout, result)
		return nil
	case "history":
		fs := flag.NewFlagSet("interest history", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		invoiceID := fs.String("invoice-id", "", "Invoice id")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*invoiceID) == "" {
			return errors.New("invoice-id is required")
		}

		history, err := client.listInvoiceInterestHistory(ctx, cfg.TenantID, strings.TrimSpace(*invoiceID))
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, history)
		}
		printInvoiceInterestHistoryTable(a.stdout, history)
		return nil
	default:
		return fmt.Errorf("unknown interest subcommand %q", args[0])
	}
}

func (a *cliApp) runInterestSettings(ctx context.Context, cfg *cliConfig, client *apiClient, args []string) error {
	if len(args) == 0 {
		return errors.New("interest settings subcommand required")
	}

	switch args[0] {
	case "get":
		fs := flag.NewFlagSet("interest settings get", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}

		settings, err := client.getInterestSettings(ctx, cfg.TenantID)
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, settings)
		}
		printInterestSettings(a.stdout, settings)
		return nil
	case "update":
		fs := flag.NewFlagSet("interest settings update", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		rate := fs.String("rate", "", "Daily interest rate, e.g. 0.0005")
		annualRate := fs.String("annual-rate", "", "Annual interest rate, e.g. 0.1825")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		parsedRate, err := parseInterestRateFlags(*rate, *annualRate)
		if err != nil {
			return err
		}

		settings, err := client.updateInterestSettings(ctx, cfg.TenantID, &invoicing.UpdateInterestSettingsRequest{Rate: parsedRate})
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, settings)
		}
		printInterestSettings(a.stdout, settings)
		return nil
	default:
		return fmt.Errorf("unknown interest settings subcommand %q", args[0])
	}
}

func (a *cliApp) runClose(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("close subcommand required")
	}
	cfg, client, err := a.loadAuthenticatedClient()
	if err != nil {
		return err
	}

	switch args[0] {
	case "events":
		fs := flag.NewFlagSet("close events", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		limitFlag := fs.String("limit", "20", "Number of period close events")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		limit, err := parseRequiredBoundedInt("limit", *limitFlag, 1, 100)
		if err != nil {
			return err
		}

		events, err := client.listPeriodCloseEvents(ctx, cfg.TenantID, limit)
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, events)
		}
		printPeriodCloseEventsTable(a.stdout, events)
		return nil
	case "period":
		fs := flag.NewFlagSet("close period", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		periodEnd := fs.String("period-end", "", "Period end date, YYYY-MM-DD")
		note := fs.String("note", "", "Close note")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*periodEnd) == "" {
			return errors.New("period-end is required")
		}

		resp, err := client.closePeriod(ctx, cfg.TenantID, &tenant.ClosePeriodRequest{
			PeriodEndDate: strings.TrimSpace(*periodEnd),
			Note:          strings.TrimSpace(*note),
		})
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, resp)
		}
		printPeriodCloseMutationResponse(a.stdout, "Closed period", resp)
		return nil
	case "reopen":
		fs := flag.NewFlagSet("close reopen", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		periodEnd := fs.String("period-end", "", "Period end date, YYYY-MM-DD")
		note := fs.String("note", "", "Reopen note")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*periodEnd) == "" {
			return errors.New("period-end is required")
		}
		if strings.TrimSpace(*note) == "" {
			return errors.New("note is required")
		}

		resp, err := client.reopenPeriod(ctx, cfg.TenantID, &tenant.ReopenPeriodRequest{
			PeriodEndDate: strings.TrimSpace(*periodEnd),
			Note:          strings.TrimSpace(*note),
		})
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, resp)
		}
		printPeriodCloseMutationResponse(a.stdout, "Reopened period", resp)
		return nil
	case "year-end-status":
		fs := flag.NewFlagSet("close year-end-status", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		periodEnd := fs.String("period-end", "", "Period end date, YYYY-MM-DD")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*periodEnd) == "" {
			return errors.New("period-end is required")
		}

		status, err := client.getYearEndCloseStatus(ctx, cfg.TenantID, strings.TrimSpace(*periodEnd))
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, status)
		}
		printYearEndCloseStatus(a.stdout, status)
		return nil
	case "carry-forward":
		fs := flag.NewFlagSet("close carry-forward", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		periodEnd := fs.String("period-end", "", "Period end date, YYYY-MM-DD")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*periodEnd) == "" {
			return errors.New("period-end is required")
		}

		result, err := client.createYearEndCarryForward(ctx, cfg.TenantID, &accounting.CreateYearEndCarryForwardRequest{
			PeriodEndDate: strings.TrimSpace(*periodEnd),
		})
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, result)
		}
		printYearEndCarryForwardResult(a.stdout, result)
		return nil
	case "reverse-carry-forward":
		fs := flag.NewFlagSet("close reverse-carry-forward", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		periodEnd := fs.String("period-end", "", "Period end date, YYYY-MM-DD")
		reason := fs.String("reason", "", "Reversal reason")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*periodEnd) == "" {
			return errors.New("period-end is required")
		}
		if strings.TrimSpace(*reason) == "" {
			return errors.New("reason is required")
		}

		result, err := client.reverseYearEndCarryForward(ctx, cfg.TenantID, &accounting.ReverseYearEndCarryForwardRequest{
			PeriodEndDate: strings.TrimSpace(*periodEnd),
			Reason:        strings.TrimSpace(*reason),
		})
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, result)
		}
		printYearEndCarryForwardReversalResult(a.stdout, result)
		return nil
	default:
		return fmt.Errorf("unknown close subcommand %q", args[0])
	}
}

func (a *cliApp) runBanking(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("banking subcommand required")
	}
	cfg, client, err := a.loadAuthenticatedClient()
	if err != nil {
		return err
	}

	switch args[0] {
	case "accounts":
		return a.runBankAccounts(ctx, cfg, client, args[1:])
	case "transactions":
		return a.runBankTransactions(ctx, cfg, client, args[1:])
	case "reconciliations":
		return a.runBankReconciliations(ctx, cfg, client, args[1:])
	default:
		return fmt.Errorf("unknown banking subcommand %q", args[0])
	}
}

func (a *cliApp) runBankAccounts(ctx context.Context, cfg *cliConfig, client *apiClient, args []string) error {
	if len(args) == 0 {
		return errors.New("banking accounts subcommand required")
	}

	switch args[0] {
	case "list":
		fs := flag.NewFlagSet("banking accounts list", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		activeOnly := fs.Bool("active-only", false, "List only active bank accounts")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}

		accounts, err := client.listBankAccounts(ctx, cfg.TenantID, *activeOnly)
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, accounts)
		}
		printBankAccountsTable(a.stdout, accounts)
		return nil

	case "create":
		fs := flag.NewFlagSet("banking accounts create", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		name := fs.String("name", "", "Bank account name")
		accountNumber := fs.String("account-number", "", "Account number or IBAN")
		bankName := fs.String("bank-name", "", "Bank name")
		swiftCode := fs.String("swift-code", "", "SWIFT/BIC code")
		currency := fs.String("currency", "EUR", "Currency code")
		glAccountID := fs.String("gl-account-id", "", "Linked GL account id")
		isDefault := fs.Bool("default", false, "Set as default bank account")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*name) == "" {
			return errors.New("name is required")
		}
		if strings.TrimSpace(*accountNumber) == "" {
			return errors.New("account-number is required")
		}

		account, err := client.createBankAccount(ctx, cfg.TenantID, &banking.CreateBankAccountRequest{
			Name:          strings.TrimSpace(*name),
			AccountNumber: strings.TrimSpace(*accountNumber),
			BankName:      strings.TrimSpace(*bankName),
			SwiftCode:     strings.TrimSpace(*swiftCode),
			Currency:      strings.ToUpper(strings.TrimSpace(*currency)),
			GLAccountID:   optionalStringPtr(*glAccountID),
			IsDefault:     *isDefault,
		})
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, account)
		}
		_, _ = fmt.Fprintf(a.stdout, "Created bank account %s (%s)\n", account.Name, account.ID)
		return nil

	case "get":
		fs := flag.NewFlagSet("banking accounts get", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		accountID := fs.String("id", "", "Bank account id")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*accountID) == "" {
			return errors.New("id is required")
		}

		account, err := client.getBankAccount(ctx, cfg.TenantID, strings.TrimSpace(*accountID))
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, account)
		}
		printBankAccount(a.stdout, account)
		return nil

	case "update":
		fs := flag.NewFlagSet("banking accounts update", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		accountID := fs.String("id", "", "Bank account id")
		name := fs.String("name", "", "Bank account name")
		bankName := fs.String("bank-name", "", "Bank name")
		swiftCode := fs.String("swift-code", "", "SWIFT/BIC code")
		glAccountID := fs.String("gl-account-id", "", "Linked GL account id")
		activeFlag := fs.String("active", "", "Set active state: true or false")
		defaultFlag := fs.String("default", "", "Set default state: true or false")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*accountID) == "" {
			return errors.New("id is required")
		}
		active, err := parseOptionalBoolPtr("active", *activeFlag)
		if err != nil {
			return err
		}
		isDefault, err := parseOptionalBoolPtr("default", *defaultFlag)
		if err != nil {
			return err
		}

		account, err := client.updateBankAccount(ctx, cfg.TenantID, strings.TrimSpace(*accountID), &banking.UpdateBankAccountRequest{
			Name:        strings.TrimSpace(*name),
			BankName:    strings.TrimSpace(*bankName),
			SwiftCode:   strings.TrimSpace(*swiftCode),
			GLAccountID: optionalStringPtr(*glAccountID),
			IsActive:    active,
			IsDefault:   isDefault,
		})
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, account)
		}
		printBankAccount(a.stdout, account)
		return nil

	case "delete":
		fs := flag.NewFlagSet("banking accounts delete", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		accountID := fs.String("id", "", "Bank account id")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*accountID) == "" {
			return errors.New("id is required")
		}

		if err := client.deleteBankAccount(ctx, cfg.TenantID, strings.TrimSpace(*accountID)); err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, map[string]string{"status": "deleted"})
		}
		_, _ = fmt.Fprintf(a.stdout, "Deleted bank account %s\n", strings.TrimSpace(*accountID))
		return nil

	default:
		return fmt.Errorf("unknown banking accounts subcommand %q", args[0])
	}
}

func (a *cliApp) runBankTransactions(ctx context.Context, cfg *cliConfig, client *apiClient, args []string) error {
	if len(args) == 0 {
		return errors.New("banking transactions subcommand required")
	}

	switch args[0] {
	case "list":
		fs := flag.NewFlagSet("banking transactions list", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		accountID := fs.String("account-id", "", "Bank account id")
		statusFlag := fs.String("status", "", "Transaction status")
		fromDate := fs.String("from", "", "From transaction date in YYYY-MM-DD")
		toDate := fs.String("to", "", "To transaction date in YYYY-MM-DD")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*accountID) == "" {
			return errors.New("account-id is required")
		}
		status, err := parseOptionalBankTransactionStatus(*statusFlag)
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

		transactions, err := client.listBankTransactions(ctx, cfg.TenantID, strings.TrimSpace(*accountID), banking.TransactionFilter{
			Status:   status,
			FromDate: fromDateValue,
			ToDate:   toDateValue,
		})
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, transactions)
		}
		printBankTransactionsTable(a.stdout, transactions)
		return nil

	case "import":
		fs := flag.NewFlagSet("banking transactions import", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		accountID := fs.String("account-id", "", "Bank account id")
		filePath := fs.String("file", "", "CSV file path, or - for stdin")
		skipDuplicates := fs.Bool("skip-duplicates", true, "Skip duplicate transactions")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*accountID) == "" {
			return errors.New("account-id is required")
		}
		content, fileName, err := readCSVInput(*filePath)
		if err != nil {
			return err
		}
		rows, err := parseBankTransactionCSVRows(content)
		if err != nil {
			return err
		}

		result, err := client.importBankTransactions(ctx, cfg.TenantID, strings.TrimSpace(*accountID), &banking.ImportCSVRequest{
			FileName:       fileName,
			Transactions:   rows,
			SkipDuplicates: *skipDuplicates,
		})
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, result)
		}
		printBankImportResult(a.stdout, result)
		return nil

	case "import-history":
		fs := flag.NewFlagSet("banking transactions import-history", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		accountID := fs.String("account-id", "", "Bank account id")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*accountID) == "" {
			return errors.New("account-id is required")
		}

		imports, err := client.listBankImportHistory(ctx, cfg.TenantID, strings.TrimSpace(*accountID))
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, imports)
		}
		printBankImportsTable(a.stdout, imports)
		return nil

	case "get":
		fs := flag.NewFlagSet("banking transactions get", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		transactionID := fs.String("id", "", "Transaction id")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*transactionID) == "" {
			return errors.New("id is required")
		}

		transaction, err := client.getBankTransaction(ctx, cfg.TenantID, strings.TrimSpace(*transactionID))
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, transaction)
		}
		printBankTransaction(a.stdout, transaction)
		return nil

	case "suggestions":
		fs := flag.NewFlagSet("banking transactions suggestions", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		transactionID := fs.String("id", "", "Transaction id")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*transactionID) == "" {
			return errors.New("id is required")
		}

		suggestions, err := client.listBankMatchSuggestions(ctx, cfg.TenantID, strings.TrimSpace(*transactionID))
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, suggestions)
		}
		printMatchSuggestionsTable(a.stdout, suggestions)
		return nil

	case "match":
		fs := flag.NewFlagSet("banking transactions match", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		transactionID := fs.String("id", "", "Transaction id")
		paymentID := fs.String("payment-id", "", "Payment id")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*transactionID) == "" {
			return errors.New("id is required")
		}
		if strings.TrimSpace(*paymentID) == "" {
			return errors.New("payment-id is required")
		}

		result, err := client.matchBankTransaction(ctx, cfg.TenantID, strings.TrimSpace(*transactionID), &banking.MatchTransactionRequest{PaymentID: strings.TrimSpace(*paymentID)})
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, result)
		}
		_, _ = fmt.Fprintf(a.stdout, "Matched bank transaction %s to payment %s\n", strings.TrimSpace(*transactionID), strings.TrimSpace(*paymentID))
		return nil

	case "unmatch":
		fs := flag.NewFlagSet("banking transactions unmatch", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		transactionID := fs.String("id", "", "Transaction id")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*transactionID) == "" {
			return errors.New("id is required")
		}

		result, err := client.unmatchBankTransaction(ctx, cfg.TenantID, strings.TrimSpace(*transactionID))
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, result)
		}
		_, _ = fmt.Fprintf(a.stdout, "Unmatched bank transaction %s\n", strings.TrimSpace(*transactionID))
		return nil

	case "review":
		fs := flag.NewFlagSet("banking transactions review", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		transactionID := fs.String("id", "", "Transaction id")
		followUpStatus := fs.String("follow-up-status", "", "Follow-up status")
		reviewNote := fs.String("review-note", "", "Review note")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*transactionID) == "" {
			return errors.New("id is required")
		}
		req := &banking.UpdateTransactionReviewRequest{}
		if strings.TrimSpace(*followUpStatus) != "" {
			status, err := parseRequiredBankFollowUpStatus(*followUpStatus)
			if err != nil {
				return err
			}
			req.FollowUpStatus = &status
		}
		if trimmed := strings.TrimSpace(*reviewNote); trimmed != "" {
			req.ReviewNote = &trimmed
		}
		if req.FollowUpStatus == nil && req.ReviewNote == nil {
			return errors.New("follow-up-status or review-note is required")
		}

		transaction, err := client.reviewBankTransaction(ctx, cfg.TenantID, strings.TrimSpace(*transactionID), req)
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, transaction)
		}
		printBankTransaction(a.stdout, transaction)
		return nil

	case "create-payment":
		fs := flag.NewFlagSet("banking transactions create-payment", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		transactionID := fs.String("id", "", "Transaction id")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*transactionID) == "" {
			return errors.New("id is required")
		}

		result, err := client.createPaymentFromBankTransaction(ctx, cfg.TenantID, strings.TrimSpace(*transactionID))
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, result)
		}
		_, _ = fmt.Fprintf(a.stdout, "Created payment %s from bank transaction %s\n", result["payment_id"], strings.TrimSpace(*transactionID))
		return nil

	case "auto-match":
		fs := flag.NewFlagSet("banking transactions auto-match", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		accountID := fs.String("account-id", "", "Bank account id")
		minConfidenceFlag := fs.String("min-confidence", "0.7", "Minimum match confidence")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*accountID) == "" {
			return errors.New("account-id is required")
		}
		minConfidence, err := strconv.ParseFloat(strings.TrimSpace(*minConfidenceFlag), 64)
		if err != nil {
			return fmt.Errorf("parse min-confidence: %w", err)
		}
		if minConfidence < 0 || minConfidence > 1 {
			return errors.New("min-confidence must be between 0 and 1")
		}

		result, err := client.autoMatchBankTransactions(ctx, cfg.TenantID, strings.TrimSpace(*accountID), minConfidence)
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, result)
		}
		_, _ = fmt.Fprintf(a.stdout, "Matched %d bank transactions\n", result["matched"])
		return nil

	default:
		return fmt.Errorf("unknown banking transactions subcommand %q", args[0])
	}
}

func (a *cliApp) runBankReconciliations(ctx context.Context, cfg *cliConfig, client *apiClient, args []string) error {
	if len(args) == 0 {
		return errors.New("banking reconciliations subcommand required")
	}

	switch args[0] {
	case "list":
		fs := flag.NewFlagSet("banking reconciliations list", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		accountID := fs.String("account-id", "", "Bank account id")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*accountID) == "" {
			return errors.New("account-id is required")
		}

		reconciliations, err := client.listBankReconciliations(ctx, cfg.TenantID, strings.TrimSpace(*accountID))
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, reconciliations)
		}
		printBankReconciliationsTable(a.stdout, reconciliations)
		return nil

	case "create":
		fs := flag.NewFlagSet("banking reconciliations create", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		accountID := fs.String("account-id", "", "Bank account id")
		statementDate := fs.String("statement-date", "", "Statement date in YYYY-MM-DD")
		openingBalanceFlag := fs.String("opening-balance", "", "Opening balance")
		closingBalanceFlag := fs.String("closing-balance", "", "Closing balance")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*accountID) == "" {
			return errors.New("account-id is required")
		}
		statementDateValue, err := parseRequiredDate("statement-date", *statementDate)
		if err != nil {
			return err
		}
		openingBalance, err := parseRequiredDecimal("opening-balance", *openingBalanceFlag)
		if err != nil {
			return err
		}
		closingBalance, err := parseRequiredDecimal("closing-balance", *closingBalanceFlag)
		if err != nil {
			return err
		}

		reconciliation, err := client.createBankReconciliation(ctx, cfg.TenantID, strings.TrimSpace(*accountID), &banking.CreateReconciliationRequest{
			StatementDate:  statementDateValue.Format("2006-01-02"),
			OpeningBalance: openingBalance,
			ClosingBalance: closingBalance,
		})
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, reconciliation)
		}
		_, _ = fmt.Fprintf(a.stdout, "Created bank reconciliation %s\n", reconciliation.ID)
		return nil

	case "get":
		fs := flag.NewFlagSet("banking reconciliations get", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		reconciliationID := fs.String("id", "", "Reconciliation id")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*reconciliationID) == "" {
			return errors.New("id is required")
		}

		reconciliation, err := client.getBankReconciliation(ctx, cfg.TenantID, strings.TrimSpace(*reconciliationID))
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, reconciliation)
		}
		printBankReconciliation(a.stdout, reconciliation)
		return nil

	case "complete":
		fs := flag.NewFlagSet("banking reconciliations complete", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		reconciliationID := fs.String("id", "", "Reconciliation id")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*reconciliationID) == "" {
			return errors.New("id is required")
		}

		result, err := client.completeBankReconciliation(ctx, cfg.TenantID, strings.TrimSpace(*reconciliationID))
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, result)
		}
		_, _ = fmt.Fprintf(a.stdout, "Completed bank reconciliation %s\n", strings.TrimSpace(*reconciliationID))
		return nil

	default:
		return fmt.Errorf("unknown banking reconciliations subcommand %q", args[0])
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

func (a *cliApp) runRecurringInvoices(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("recurring-invoices subcommand required")
	}
	cfg, client, err := a.loadAuthenticatedClient()
	if err != nil {
		return err
	}

	switch args[0] {
	case "list":
		fs := flag.NewFlagSet("recurring-invoices list", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		activeOnly := fs.Bool("active-only", false, "List only active recurring invoices")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}

		invoices, err := client.listRecurringInvoices(ctx, cfg.TenantID, *activeOnly)
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, invoices)
		}
		printRecurringInvoicesTable(a.stdout, invoices)
		return nil

	case "create":
		fs := flag.NewFlagSet("recurring-invoices create", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		name := fs.String("name", "", "Recurring invoice name")
		contactID := fs.String("contact-id", "", "Contact id")
		invoiceTypeFlag := fs.String("type", "SALES", "Invoice type")
		currency := fs.String("currency", "EUR", "Currency code")
		frequencyFlag := fs.String("frequency", "", "Frequency")
		startDate := fs.String("start-date", "", "Start date in YYYY-MM-DD")
		endDate := fs.String("end-date", "", "End date in YYYY-MM-DD")
		paymentTermsDaysFlag := fs.String("payment-terms-days", "14", "Payment terms in days")
		reference := fs.String("reference", "", "Reference")
		notes := fs.String("notes", "", "Notes")
		sendEmail := fs.Bool("send-email", false, "Send generated invoices by email")
		emailTemplateType := fs.String("email-template-type", "", "Email template type")
		recipientEmail := fs.String("recipient-email", "", "Recipient email override")
		attachPDF := fs.Bool("attach-pdf", true, "Attach generated invoice PDF to email")
		emailSubject := fs.String("email-subject", "", "Email subject override")
		emailMessage := fs.String("email-message", "", "Email message override")
		lines := recurringLineFlags{}
		fs.Var(&lines, "line", "Line as comma-separated key=value pairs; repeatable")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*name) == "" {
			return errors.New("name is required")
		}
		if strings.TrimSpace(*contactID) == "" {
			return errors.New("contact-id is required")
		}
		invoiceType, err := parseRequiredInvoiceType(*invoiceTypeFlag)
		if err != nil {
			return err
		}
		frequency, err := parseRequiredRecurringFrequency(*frequencyFlag)
		if err != nil {
			return err
		}
		startDateValue, err := parseRequiredDate("start-date", *startDate)
		if err != nil {
			return err
		}
		endDateValue, err := parseOptionalDate("end-date", *endDate)
		if err != nil {
			return err
		}
		paymentTermsDays, err := parseRequiredNonNegativeInt("payment-terms-days", *paymentTermsDaysFlag)
		if err != nil {
			return err
		}
		if len(lines) == 0 {
			return errors.New("at least one line is required")
		}

		attachPDFValue := *attachPDF
		invoice, err := client.createRecurringInvoice(ctx, cfg.TenantID, &recurring.CreateRecurringInvoiceRequest{
			Name:                   strings.TrimSpace(*name),
			ContactID:              strings.TrimSpace(*contactID),
			InvoiceType:            string(invoiceType),
			Currency:               strings.ToUpper(strings.TrimSpace(*currency)),
			Frequency:              frequency,
			StartDate:              startDateValue,
			EndDate:                endDateValue,
			PaymentTermsDays:       paymentTermsDays,
			Reference:              strings.TrimSpace(*reference),
			Notes:                  strings.TrimSpace(*notes),
			Lines:                  []recurring.CreateRecurringInvoiceLineRequest(lines),
			SendEmailOnGeneration:  *sendEmail,
			EmailTemplateType:      strings.TrimSpace(*emailTemplateType),
			RecipientEmailOverride: strings.TrimSpace(*recipientEmail),
			AttachPDFToEmail:       &attachPDFValue,
			EmailSubjectOverride:   strings.TrimSpace(*emailSubject),
			EmailMessage:           strings.TrimSpace(*emailMessage),
		})
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, invoice)
		}
		_, _ = fmt.Fprintf(a.stdout, "Created recurring invoice %s (%s)\n", invoice.Name, invoice.ID)
		return nil

	case "from-invoice":
		fs := flag.NewFlagSet("recurring-invoices from-invoice", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		invoiceID := fs.String("invoice-id", "", "Source invoice id")
		name := fs.String("name", "", "Recurring invoice name")
		frequencyFlag := fs.String("frequency", "", "Frequency")
		startDate := fs.String("start-date", "", "Start date in YYYY-MM-DD")
		endDate := fs.String("end-date", "", "End date in YYYY-MM-DD")
		paymentTermsDaysFlag := fs.String("payment-terms-days", "14", "Payment terms in days")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*invoiceID) == "" {
			return errors.New("invoice-id is required")
		}
		if strings.TrimSpace(*name) == "" {
			return errors.New("name is required")
		}
		frequency, err := parseRequiredRecurringFrequency(*frequencyFlag)
		if err != nil {
			return err
		}
		startDateValue, err := parseRequiredDate("start-date", *startDate)
		if err != nil {
			return err
		}
		endDateValue, err := parseOptionalDate("end-date", *endDate)
		if err != nil {
			return err
		}
		paymentTermsDays, err := parseRequiredNonNegativeInt("payment-terms-days", *paymentTermsDaysFlag)
		if err != nil {
			return err
		}

		invoice, err := client.createRecurringInvoiceFromInvoice(ctx, cfg.TenantID, strings.TrimSpace(*invoiceID), &recurring.CreateFromInvoiceRequest{
			Name:             strings.TrimSpace(*name),
			Frequency:        frequency,
			StartDate:        startDateValue,
			EndDate:          endDateValue,
			PaymentTermsDays: paymentTermsDays,
		})
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, invoice)
		}
		_, _ = fmt.Fprintf(a.stdout, "Created recurring invoice %s (%s) from invoice %s\n", invoice.Name, invoice.ID, strings.TrimSpace(*invoiceID))
		return nil

	case "get":
		fs := flag.NewFlagSet("recurring-invoices get", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		recurringID := fs.String("id", "", "Recurring invoice id")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*recurringID) == "" {
			return errors.New("id is required")
		}

		invoice, err := client.getRecurringInvoice(ctx, cfg.TenantID, strings.TrimSpace(*recurringID))
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, invoice)
		}
		printRecurringInvoice(a.stdout, invoice)
		return nil

	case "update":
		fs := flag.NewFlagSet("recurring-invoices update", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		recurringID := fs.String("id", "", "Recurring invoice id")
		name := fs.String("name", "", "Recurring invoice name")
		contactID := fs.String("contact-id", "", "Contact id")
		frequencyFlag := fs.String("frequency", "", "Frequency")
		endDate := fs.String("end-date", "", "End date in YYYY-MM-DD")
		paymentTermsDaysFlag := fs.String("payment-terms-days", "", "Payment terms in days")
		reference := fs.String("reference", "", "Reference")
		notes := fs.String("notes", "", "Notes")
		sendEmailFlag := fs.String("send-email", "", "Send generated invoices by email: true or false")
		emailTemplateType := fs.String("email-template-type", "", "Email template type")
		recipientEmail := fs.String("recipient-email", "", "Recipient email override")
		attachPDFFlag := fs.String("attach-pdf", "", "Attach generated invoice PDF to email: true or false")
		emailSubject := fs.String("email-subject", "", "Email subject override")
		emailMessage := fs.String("email-message", "", "Email message override")
		lines := recurringLineFlags{}
		fs.Var(&lines, "line", "Line as comma-separated key=value pairs; repeatable")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*recurringID) == "" {
			return errors.New("id is required")
		}

		req := &recurring.UpdateRecurringInvoiceRequest{}
		if trimmed := strings.TrimSpace(*name); trimmed != "" {
			req.Name = &trimmed
		}
		if trimmed := strings.TrimSpace(*contactID); trimmed != "" {
			req.ContactID = &trimmed
		}
		if strings.TrimSpace(*frequencyFlag) != "" {
			frequency, err := parseRequiredRecurringFrequency(*frequencyFlag)
			if err != nil {
				return err
			}
			req.Frequency = &frequency
		}
		if strings.TrimSpace(*endDate) != "" {
			endDateValue, err := parseRequiredDate("end-date", *endDate)
			if err != nil {
				return err
			}
			req.EndDate = &endDateValue
		}
		if strings.TrimSpace(*paymentTermsDaysFlag) != "" {
			paymentTermsDays, err := parseRequiredNonNegativeInt("payment-terms-days", *paymentTermsDaysFlag)
			if err != nil {
				return err
			}
			req.PaymentTermsDays = &paymentTermsDays
		}
		if trimmed := strings.TrimSpace(*reference); trimmed != "" {
			req.Reference = &trimmed
		}
		if trimmed := strings.TrimSpace(*notes); trimmed != "" {
			req.Notes = &trimmed
		}
		sendEmailValue, err := parseOptionalBoolPtr("send-email", *sendEmailFlag)
		if err != nil {
			return err
		}
		req.SendEmailOnGeneration = sendEmailValue
		if trimmed := strings.TrimSpace(*emailTemplateType); trimmed != "" {
			req.EmailTemplateType = &trimmed
		}
		if trimmed := strings.TrimSpace(*recipientEmail); trimmed != "" {
			req.RecipientEmailOverride = &trimmed
		}
		attachPDFValue, err := parseOptionalBoolPtr("attach-pdf", *attachPDFFlag)
		if err != nil {
			return err
		}
		req.AttachPDFToEmail = attachPDFValue
		if trimmed := strings.TrimSpace(*emailSubject); trimmed != "" {
			req.EmailSubjectOverride = &trimmed
		}
		if trimmed := strings.TrimSpace(*emailMessage); trimmed != "" {
			req.EmailMessage = &trimmed
		}
		if len(lines) > 0 {
			req.Lines = []recurring.CreateRecurringInvoiceLineRequest(lines)
		}

		invoice, err := client.updateRecurringInvoice(ctx, cfg.TenantID, strings.TrimSpace(*recurringID), req)
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, invoice)
		}
		printRecurringInvoice(a.stdout, invoice)
		return nil

	case "delete":
		fs := flag.NewFlagSet("recurring-invoices delete", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		recurringID := fs.String("id", "", "Recurring invoice id")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*recurringID) == "" {
			return errors.New("id is required")
		}

		result, err := client.deleteRecurringInvoice(ctx, cfg.TenantID, strings.TrimSpace(*recurringID))
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, result)
		}
		_, _ = fmt.Fprintf(a.stdout, "Deleted recurring invoice %s\n", strings.TrimSpace(*recurringID))
		return nil

	case "pause", "resume":
		fs := flag.NewFlagSet("recurring-invoices "+args[0], flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		recurringID := fs.String("id", "", "Recurring invoice id")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*recurringID) == "" {
			return errors.New("id is required")
		}

		result, err := client.updateRecurringInvoiceStatus(ctx, cfg.TenantID, strings.TrimSpace(*recurringID), args[0])
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, result)
		}
		_, _ = fmt.Fprintf(a.stdout, "%s recurring invoice %s\n", titleLabel(args[0])+"d", strings.TrimSpace(*recurringID))
		return nil

	case "generate":
		fs := flag.NewFlagSet("recurring-invoices generate", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		recurringID := fs.String("id", "", "Recurring invoice id")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*recurringID) == "" {
			return errors.New("id is required")
		}

		result, err := client.generateRecurringInvoice(ctx, cfg.TenantID, strings.TrimSpace(*recurringID))
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, result)
		}
		_, _ = fmt.Fprintf(a.stdout, "Generated invoice %s (%s) from recurring invoice %s\n", result.GeneratedInvoiceNumber, result.GeneratedInvoiceID, result.RecurringInvoiceID)
		return nil

	case "generate-due":
		fs := flag.NewFlagSet("recurring-invoices generate-due", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}

		results, err := client.generateDueRecurringInvoices(ctx, cfg.TenantID)
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, results)
		}
		printRecurringGenerationResultsTable(a.stdout, results)
		return nil

	default:
		return fmt.Errorf("unknown recurring-invoices subcommand %q", args[0])
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

	case "import":
		fs := flag.NewFlagSet("assets import", flag.ContinueOnError)
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
		result, err := client.importAssets(ctx, cfg.TenantID, &assets.ImportAssetsRequest{
			FileName:   fileName,
			CSVContent: content,
		})
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, result)
		}
		_, _ = fmt.Fprintf(a.stdout, "Processed %d rows, created %d assets, skipped %d rows\n", result.RowsProcessed, result.AssetsCreated, result.RowsSkipped)
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

func (a *cliApp) runInventory(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("inventory subcommand required")
	}
	cfg, client, err := a.loadAuthenticatedClient()
	if err != nil {
		return err
	}

	switch args[0] {
	case "categories":
		return a.runInventoryCategories(ctx, cfg, client, args[1:])
	case "products":
		return a.runInventoryProducts(ctx, cfg, client, args[1:])
	case "warehouses":
		return a.runInventoryWarehouses(ctx, cfg, client, args[1:])
	case "adjust":
		fs := flag.NewFlagSet("inventory adjust", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		productID := fs.String("product-id", "", "Product id")
		warehouseID := fs.String("warehouse-id", "", "Warehouse id")
		quantityFlag := fs.String("quantity", "", "Signed quantity adjustment")
		unitCostFlag := fs.String("unit-cost", "0", "Unit cost")
		reason := fs.String("reason", "", "Adjustment reason")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*productID) == "" {
			return errors.New("product-id is required")
		}
		if strings.TrimSpace(*warehouseID) == "" {
			return errors.New("warehouse-id is required")
		}
		quantity, err := parseRequiredDecimal("quantity", *quantityFlag)
		if err != nil {
			return err
		}
		if quantity.IsZero() {
			return errors.New("quantity must not be zero")
		}
		unitCost, err := parseRequiredNonNegativeDecimal("unit-cost", *unitCostFlag)
		if err != nil {
			return err
		}

		movement, err := client.adjustStock(ctx, cfg.TenantID, &inventory.AdjustStockRequest{
			ProductID:   strings.TrimSpace(*productID),
			WarehouseID: strings.TrimSpace(*warehouseID),
			Quantity:    quantity.String(),
			UnitCost:    unitCost.String(),
			Reason:      strings.TrimSpace(*reason),
		})
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, movement)
		}
		_, _ = fmt.Fprintf(a.stdout, "Adjusted stock for product %s by %s in warehouse %s\n", strings.TrimSpace(*productID), quantity.String(), strings.TrimSpace(*warehouseID))
		return nil

	case "transfer":
		fs := flag.NewFlagSet("inventory transfer", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		productID := fs.String("product-id", "", "Product id")
		fromWarehouseID := fs.String("from-warehouse-id", "", "Source warehouse id")
		toWarehouseID := fs.String("to-warehouse-id", "", "Destination warehouse id")
		quantityFlag := fs.String("quantity", "", "Quantity to transfer")
		notes := fs.String("notes", "", "Transfer notes")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*productID) == "" {
			return errors.New("product-id is required")
		}
		if strings.TrimSpace(*fromWarehouseID) == "" {
			return errors.New("from-warehouse-id is required")
		}
		if strings.TrimSpace(*toWarehouseID) == "" {
			return errors.New("to-warehouse-id is required")
		}
		quantity, err := parseRequiredPositiveDecimal("quantity", *quantityFlag)
		if err != nil {
			return err
		}

		result, err := client.transferStock(ctx, cfg.TenantID, &inventory.TransferStockRequest{
			ProductID:       strings.TrimSpace(*productID),
			FromWarehouseID: strings.TrimSpace(*fromWarehouseID),
			ToWarehouseID:   strings.TrimSpace(*toWarehouseID),
			Quantity:        quantity.String(),
			Notes:           strings.TrimSpace(*notes),
		})
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, result)
		}
		_, _ = fmt.Fprintf(a.stdout, "Transferred %s of product %s from %s to %s\n", quantity.String(), strings.TrimSpace(*productID), strings.TrimSpace(*fromWarehouseID), strings.TrimSpace(*toWarehouseID))
		return nil

	default:
		return fmt.Errorf("unknown inventory subcommand %q", args[0])
	}
}

func (a *cliApp) runInventoryCategories(ctx context.Context, cfg *cliConfig, client *apiClient, args []string) error {
	if len(args) == 0 {
		return errors.New("inventory categories subcommand required")
	}

	switch args[0] {
	case "list":
		fs := flag.NewFlagSet("inventory categories list", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}

		categories, err := client.listProductCategories(ctx, cfg.TenantID)
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, categories)
		}
		printProductCategoriesTable(a.stdout, categories)
		return nil

	case "create":
		fs := flag.NewFlagSet("inventory categories create", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		name := fs.String("name", "", "Category name")
		description := fs.String("description", "", "Description")
		parentID := fs.String("parent-id", "", "Parent category id")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*name) == "" {
			return errors.New("name is required")
		}

		category, err := client.createProductCategory(ctx, cfg.TenantID, &inventory.CreateCategoryRequest{
			Name:        strings.TrimSpace(*name),
			Description: strings.TrimSpace(*description),
			ParentID:    strings.TrimSpace(*parentID),
		})
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, category)
		}
		_, _ = fmt.Fprintf(a.stdout, "Created product category %s (%s)\n", category.Name, category.ID)
		return nil

	case "get":
		fs := flag.NewFlagSet("inventory categories get", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		categoryID := fs.String("id", "", "Category id")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*categoryID) == "" {
			return errors.New("id is required")
		}

		category, err := client.getProductCategory(ctx, cfg.TenantID, strings.TrimSpace(*categoryID))
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, category)
		}
		printProductCategory(a.stdout, category)
		return nil

	case "delete":
		fs := flag.NewFlagSet("inventory categories delete", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		categoryID := fs.String("id", "", "Category id")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*categoryID) == "" {
			return errors.New("id is required")
		}

		result, err := client.deleteProductCategory(ctx, cfg.TenantID, strings.TrimSpace(*categoryID))
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, result)
		}
		_, _ = fmt.Fprintf(a.stdout, "Deleted product category %s\n", strings.TrimSpace(*categoryID))
		return nil

	default:
		return fmt.Errorf("unknown inventory categories subcommand %q", args[0])
	}
}

func (a *cliApp) runInventoryProducts(ctx context.Context, cfg *cliConfig, client *apiClient, args []string) error {
	if len(args) == 0 {
		return errors.New("inventory products subcommand required")
	}

	switch args[0] {
	case "list":
		fs := flag.NewFlagSet("inventory products list", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		typeFlag := fs.String("type", "", "Product type")
		statusFlag := fs.String("status", "", "Product status")
		categoryID := fs.String("category-id", "", "Category id")
		search := fs.String("search", "", "Search term")
		lowStock := fs.Bool("low-stock", false, "List products below reorder threshold")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		productType, err := parseOptionalProductType(*typeFlag)
		if err != nil {
			return err
		}
		status, err := parseOptionalProductStatus(*statusFlag)
		if err != nil {
			return err
		}

		products, err := client.listProducts(ctx, cfg.TenantID, inventory.ProductFilter{
			ProductType: productType,
			Status:      status,
			CategoryID:  strings.TrimSpace(*categoryID),
			Search:      strings.TrimSpace(*search),
			LowStock:    *lowStock,
		})
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, products)
		}
		printProductsTable(a.stdout, products)
		return nil

	case "create":
		fs := flag.NewFlagSet("inventory products create", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		code := fs.String("code", "", "Product code")
		name := fs.String("name", "", "Product name")
		description := fs.String("description", "", "Description")
		typeFlag := fs.String("type", string(inventory.ProductTypeGoods), "Product type")
		categoryID := fs.String("category-id", "", "Category id")
		unit := fs.String("unit", "pcs", "Unit of measure")
		purchasePriceFlag := fs.String("purchase-price", "0", "Purchase price")
		salesPriceFlag := fs.String("sales-price", "", "Sales price")
		vatRateFlag := fs.String("vat-rate", "22", "VAT rate")
		minStockLevelFlag := fs.String("min-stock-level", "0", "Minimum stock level")
		reorderPointFlag := fs.String("reorder-point", "0", "Reorder point")
		saleAccountID := fs.String("sale-account-id", "", "Sale account id")
		purchaseAccountID := fs.String("purchase-account-id", "", "Purchase account id")
		inventoryAccountID := fs.String("inventory-account-id", "", "Inventory account id")
		trackInventory := fs.Bool("track-inventory", true, "Track inventory for this product")
		barcode := fs.String("barcode", "", "Barcode")
		supplierID := fs.String("supplier-id", "", "Supplier id")
		leadTimeDaysFlag := fs.String("lead-time-days", "0", "Lead time in days")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*name) == "" {
			return errors.New("name is required")
		}
		productType, err := parseRequiredProductType(*typeFlag)
		if err != nil {
			return err
		}
		purchasePrice, err := parseRequiredNonNegativeDecimal("purchase-price", *purchasePriceFlag)
		if err != nil {
			return err
		}
		salesPrice, err := parseRequiredNonNegativeDecimal("sales-price", *salesPriceFlag)
		if err != nil {
			return err
		}
		vatRate, err := parseRequiredNonNegativeDecimal("vat-rate", *vatRateFlag)
		if err != nil {
			return err
		}
		minStockLevel, err := parseRequiredNonNegativeDecimal("min-stock-level", *minStockLevelFlag)
		if err != nil {
			return err
		}
		reorderPoint, err := parseRequiredNonNegativeDecimal("reorder-point", *reorderPointFlag)
		if err != nil {
			return err
		}
		leadTimeDays, err := parseRequiredNonNegativeInt("lead-time-days", *leadTimeDaysFlag)
		if err != nil {
			return err
		}

		product, err := client.createProduct(ctx, cfg.TenantID, &inventory.CreateProductRequest{
			Code:               strings.TrimSpace(*code),
			Name:               strings.TrimSpace(*name),
			Description:        strings.TrimSpace(*description),
			ProductType:        string(productType),
			CategoryID:         strings.TrimSpace(*categoryID),
			Unit:               strings.TrimSpace(*unit),
			PurchasePrice:      purchasePrice.String(),
			SalesPrice:         salesPrice.String(),
			VATRate:            vatRate.String(),
			MinStockLevel:      minStockLevel.String(),
			ReorderPoint:       reorderPoint.String(),
			SaleAccountID:      strings.TrimSpace(*saleAccountID),
			PurchaseAccountID:  strings.TrimSpace(*purchaseAccountID),
			InventoryAccountID: strings.TrimSpace(*inventoryAccountID),
			TrackInventory:     *trackInventory,
			Barcode:            strings.TrimSpace(*barcode),
			SupplierID:         strings.TrimSpace(*supplierID),
			LeadTimeDays:       leadTimeDays,
		})
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, product)
		}
		_, _ = fmt.Fprintf(a.stdout, "Created product %s %s (%s)\n", product.Code, product.Name, product.ID)
		return nil

	case "get":
		fs := flag.NewFlagSet("inventory products get", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		productID := fs.String("id", "", "Product id")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*productID) == "" {
			return errors.New("id is required")
		}

		product, err := client.getProduct(ctx, cfg.TenantID, strings.TrimSpace(*productID))
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, product)
		}
		printProduct(a.stdout, product)
		return nil

	case "update":
		fs := flag.NewFlagSet("inventory products update", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		productID := fs.String("id", "", "Product id")
		name := fs.String("name", "", "Product name")
		description := fs.String("description", "", "Description")
		categoryID := fs.String("category-id", "", "Category id")
		unit := fs.String("unit", "pcs", "Unit of measure")
		purchasePriceFlag := fs.String("purchase-price", "0", "Purchase price")
		salesPriceFlag := fs.String("sales-price", "", "Sales price")
		vatRateFlag := fs.String("vat-rate", "22", "VAT rate")
		minStockLevelFlag := fs.String("min-stock-level", "0", "Minimum stock level")
		reorderPointFlag := fs.String("reorder-point", "0", "Reorder point")
		saleAccountID := fs.String("sale-account-id", "", "Sale account id")
		purchaseAccountID := fs.String("purchase-account-id", "", "Purchase account id")
		inventoryAccountID := fs.String("inventory-account-id", "", "Inventory account id")
		trackInventory := fs.Bool("track-inventory", true, "Track inventory for this product")
		active := fs.Bool("active", true, "Set active")
		barcode := fs.String("barcode", "", "Barcode")
		supplierID := fs.String("supplier-id", "", "Supplier id")
		leadTimeDaysFlag := fs.String("lead-time-days", "0", "Lead time in days")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*productID) == "" {
			return errors.New("id is required")
		}
		if strings.TrimSpace(*name) == "" {
			return errors.New("name is required")
		}
		purchasePrice, err := parseRequiredNonNegativeDecimal("purchase-price", *purchasePriceFlag)
		if err != nil {
			return err
		}
		salesPrice, err := parseRequiredNonNegativeDecimal("sales-price", *salesPriceFlag)
		if err != nil {
			return err
		}
		vatRate, err := parseRequiredNonNegativeDecimal("vat-rate", *vatRateFlag)
		if err != nil {
			return err
		}
		minStockLevel, err := parseRequiredNonNegativeDecimal("min-stock-level", *minStockLevelFlag)
		if err != nil {
			return err
		}
		reorderPoint, err := parseRequiredNonNegativeDecimal("reorder-point", *reorderPointFlag)
		if err != nil {
			return err
		}
		leadTimeDays, err := parseRequiredNonNegativeInt("lead-time-days", *leadTimeDaysFlag)
		if err != nil {
			return err
		}

		product, err := client.updateProduct(ctx, cfg.TenantID, strings.TrimSpace(*productID), &inventory.UpdateProductRequest{
			Name:               strings.TrimSpace(*name),
			Description:        strings.TrimSpace(*description),
			CategoryID:         strings.TrimSpace(*categoryID),
			Unit:               strings.TrimSpace(*unit),
			PurchasePrice:      purchasePrice.String(),
			SalesPrice:         salesPrice.String(),
			VATRate:            vatRate.String(),
			MinStockLevel:      minStockLevel.String(),
			ReorderPoint:       reorderPoint.String(),
			SaleAccountID:      strings.TrimSpace(*saleAccountID),
			PurchaseAccountID:  strings.TrimSpace(*purchaseAccountID),
			InventoryAccountID: strings.TrimSpace(*inventoryAccountID),
			TrackInventory:     *trackInventory,
			IsActive:           *active,
			Barcode:            strings.TrimSpace(*barcode),
			SupplierID:         strings.TrimSpace(*supplierID),
			LeadTimeDays:       leadTimeDays,
		})
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, product)
		}
		printProduct(a.stdout, product)
		return nil

	case "delete":
		fs := flag.NewFlagSet("inventory products delete", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		productID := fs.String("id", "", "Product id")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*productID) == "" {
			return errors.New("id is required")
		}

		result, err := client.deleteProduct(ctx, cfg.TenantID, strings.TrimSpace(*productID))
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, result)
		}
		_, _ = fmt.Fprintf(a.stdout, "Deleted product %s\n", strings.TrimSpace(*productID))
		return nil

	case "stock-levels":
		fs := flag.NewFlagSet("inventory products stock-levels", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		productID := fs.String("id", "", "Product id")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*productID) == "" {
			return errors.New("id is required")
		}

		levels, err := client.listStockLevels(ctx, cfg.TenantID, strings.TrimSpace(*productID))
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, levels)
		}
		printStockLevelsTable(a.stdout, levels)
		return nil

	case "movements":
		fs := flag.NewFlagSet("inventory products movements", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		productID := fs.String("id", "", "Product id")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*productID) == "" {
			return errors.New("id is required")
		}

		movements, err := client.listInventoryMovements(ctx, cfg.TenantID, strings.TrimSpace(*productID))
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, movements)
		}
		printInventoryMovementsTable(a.stdout, movements)
		return nil

	default:
		return fmt.Errorf("unknown inventory products subcommand %q", args[0])
	}
}

func (a *cliApp) runInventoryWarehouses(ctx context.Context, cfg *cliConfig, client *apiClient, args []string) error {
	if len(args) == 0 {
		return errors.New("inventory warehouses subcommand required")
	}

	switch args[0] {
	case "list":
		fs := flag.NewFlagSet("inventory warehouses list", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		activeOnly := fs.Bool("active-only", false, "List only active warehouses")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}

		warehouses, err := client.listWarehouses(ctx, cfg.TenantID, *activeOnly)
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, warehouses)
		}
		printWarehousesTable(a.stdout, warehouses)
		return nil

	case "create":
		fs := flag.NewFlagSet("inventory warehouses create", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		code := fs.String("code", "", "Warehouse code")
		name := fs.String("name", "", "Warehouse name")
		address := fs.String("address", "", "Address")
		isDefault := fs.Bool("default", false, "Set as default warehouse")
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

		warehouse, err := client.createWarehouse(ctx, cfg.TenantID, &inventory.CreateWarehouseRequest{
			Code:      strings.TrimSpace(*code),
			Name:      strings.TrimSpace(*name),
			Address:   strings.TrimSpace(*address),
			IsDefault: *isDefault,
		})
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, warehouse)
		}
		_, _ = fmt.Fprintf(a.stdout, "Created warehouse %s %s (%s)\n", warehouse.Code, warehouse.Name, warehouse.ID)
		return nil

	case "get":
		fs := flag.NewFlagSet("inventory warehouses get", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		warehouseID := fs.String("id", "", "Warehouse id")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*warehouseID) == "" {
			return errors.New("id is required")
		}

		warehouse, err := client.getWarehouse(ctx, cfg.TenantID, strings.TrimSpace(*warehouseID))
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, warehouse)
		}
		printWarehouse(a.stdout, warehouse)
		return nil

	case "update":
		fs := flag.NewFlagSet("inventory warehouses update", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		warehouseID := fs.String("id", "", "Warehouse id")
		name := fs.String("name", "", "Warehouse name")
		address := fs.String("address", "", "Address")
		isDefault := fs.Bool("default", false, "Set as default warehouse")
		active := fs.Bool("active", true, "Set active")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*warehouseID) == "" {
			return errors.New("id is required")
		}
		if strings.TrimSpace(*name) == "" {
			return errors.New("name is required")
		}

		warehouse, err := client.updateWarehouse(ctx, cfg.TenantID, strings.TrimSpace(*warehouseID), &inventory.UpdateWarehouseRequest{
			Name:      strings.TrimSpace(*name),
			Address:   strings.TrimSpace(*address),
			IsDefault: *isDefault,
			IsActive:  *active,
		})
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, warehouse)
		}
		printWarehouse(a.stdout, warehouse)
		return nil

	case "delete":
		fs := flag.NewFlagSet("inventory warehouses delete", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		warehouseID := fs.String("id", "", "Warehouse id")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*warehouseID) == "" {
			return errors.New("id is required")
		}

		result, err := client.deleteWarehouse(ctx, cfg.TenantID, strings.TrimSpace(*warehouseID))
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, result)
		}
		_, _ = fmt.Fprintf(a.stdout, "Deleted warehouse %s\n", strings.TrimSpace(*warehouseID))
		return nil

	default:
		return fmt.Errorf("unknown inventory warehouses subcommand %q", args[0])
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

func (a *cliApp) runAnalytics(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("analytics subcommand required")
	}
	cfg, client, err := a.loadAuthenticatedClient()
	if err != nil {
		return err
	}

	switch args[0] {
	case "dashboard":
		fs := flag.NewFlagSet("analytics dashboard", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}

		summary, err := client.getDashboardSummary(ctx, cfg.TenantID)
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, summary)
		}
		printDashboardSummary(a.stdout, summary)
		return nil

	case "revenue-expense":
		fs := flag.NewFlagSet("analytics revenue-expense", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		monthsFlag := fs.String("months", "12", "Number of months")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		months, err := parseRequiredPositiveInt("months", *monthsFlag)
		if err != nil {
			return err
		}

		chart, err := client.getRevenueExpenseChart(ctx, cfg.TenantID, months)
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, chart)
		}
		printRevenueExpenseChart(a.stdout, chart)
		return nil

	case "cash-flow":
		fs := flag.NewFlagSet("analytics cash-flow", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		monthsFlag := fs.String("months", "12", "Number of months")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		months, err := parseRequiredPositiveInt("months", *monthsFlag)
		if err != nil {
			return err
		}

		chart, err := client.getCashFlowChart(ctx, cfg.TenantID, months)
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, chart)
		}
		printCashFlowChart(a.stdout, chart)
		return nil

	case "activity":
		fs := flag.NewFlagSet("analytics activity", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		limitFlag := fs.String("limit", "10", "Number of activity items")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		limit, err := parseRequiredPositiveInt("limit", *limitFlag)
		if err != nil {
			return err
		}

		activity, err := client.getRecentActivity(ctx, cfg.TenantID, limit)
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, activity)
		}
		printActivityItems(a.stdout, activity)
		return nil

	default:
		return fmt.Errorf("unknown analytics subcommand %q", args[0])
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

func (a *cliApp) runLeave(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("leave subcommand required")
	}
	cfg, client, err := a.loadAuthenticatedClient()
	if err != nil {
		return err
	}

	switch args[0] {
	case "absence-types":
		return a.runLeaveAbsenceTypes(ctx, cfg, client, args[1:])
	case "balances":
		return a.runLeaveBalances(ctx, cfg, client, args[1:])
	case "records":
		return a.runLeaveRecords(ctx, cfg, client, args[1:])
	default:
		return fmt.Errorf("unknown leave subcommand %q", args[0])
	}
}

func (a *cliApp) runLeaveAbsenceTypes(ctx context.Context, cfg *cliConfig, client *apiClient, args []string) error {
	if len(args) == 0 {
		return errors.New("leave absence-types subcommand required")
	}

	switch args[0] {
	case "list":
		fs := flag.NewFlagSet("leave absence-types list", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		activeOnly := fs.Bool("active-only", false, "List only active absence types")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}

		types, err := client.listAbsenceTypes(ctx, cfg.TenantID, *activeOnly)
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, types)
		}
		printAbsenceTypesTable(a.stdout, types)
		return nil

	case "get":
		fs := flag.NewFlagSet("leave absence-types get", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		typeID := fs.String("id", "", "Absence type id")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*typeID) == "" {
			return errors.New("id is required")
		}

		absenceType, err := client.getAbsenceType(ctx, cfg.TenantID, strings.TrimSpace(*typeID))
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, absenceType)
		}
		printAbsenceType(a.stdout, absenceType)
		return nil

	default:
		return fmt.Errorf("unknown leave absence-types subcommand %q", args[0])
	}
}

func (a *cliApp) runLeaveBalances(ctx context.Context, cfg *cliConfig, client *apiClient, args []string) error {
	if len(args) == 0 {
		return errors.New("leave balances subcommand required")
	}

	switch args[0] {
	case "list":
		fs := flag.NewFlagSet("leave balances list", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		employeeID := fs.String("employee-id", "", "Employee id")
		yearFlag := fs.String("year", "", "Balance year")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*employeeID) == "" {
			return errors.New("employee-id is required")
		}
		year, err := parseOptionalInt(*yearFlag)
		if err != nil {
			return err
		}

		balances, err := client.listLeaveBalances(ctx, cfg.TenantID, strings.TrimSpace(*employeeID), year)
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, balances)
		}
		printLeaveBalancesTable(a.stdout, balances)
		return nil

	case "by-year":
		fs := flag.NewFlagSet("leave balances by-year", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		employeeID := fs.String("employee-id", "", "Employee id")
		yearFlag := fs.String("year", "", "Balance year")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*employeeID) == "" {
			return errors.New("employee-id is required")
		}
		year, err := parseRequiredPositiveInt("year", *yearFlag)
		if err != nil {
			return err
		}

		balances, err := client.getLeaveBalancesByYear(ctx, cfg.TenantID, strings.TrimSpace(*employeeID), year)
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, balances)
		}
		printLeaveBalancesTable(a.stdout, balances)
		return nil

	case "update":
		fs := flag.NewFlagSet("leave balances update", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		employeeID := fs.String("employee-id", "", "Employee id")
		absenceTypeID := fs.String("absence-type-id", "", "Absence type id")
		yearFlag := fs.String("year", "", "Balance year")
		entitledDaysFlag := fs.String("entitled-days", "", "Entitled days")
		carryoverDaysFlag := fs.String("carryover-days", "", "Carryover days")
		notes := fs.String("notes", "", "Notes")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*employeeID) == "" {
			return errors.New("employee-id is required")
		}
		if strings.TrimSpace(*absenceTypeID) == "" {
			return errors.New("absence-type-id is required")
		}
		year, err := parseRequiredPositiveInt("year", *yearFlag)
		if err != nil {
			return err
		}
		entitledDays, err := parseOptionalNonNegativeDecimalPtr("entitled-days", *entitledDaysFlag)
		if err != nil {
			return err
		}
		carryoverDays, err := parseOptionalNonNegativeDecimalPtr("carryover-days", *carryoverDaysFlag)
		if err != nil {
			return err
		}
		if entitledDays == nil && carryoverDays == nil && strings.TrimSpace(*notes) == "" {
			return errors.New("entitled-days, carryover-days, or notes is required")
		}

		balance, err := client.updateLeaveBalance(ctx, cfg.TenantID, strings.TrimSpace(*employeeID), year, strings.TrimSpace(*absenceTypeID), &payroll.UpdateLeaveBalanceRequest{
			EntitledDays:  entitledDays,
			CarryoverDays: carryoverDays,
			Notes:         strings.TrimSpace(*notes),
		})
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, balance)
		}
		printLeaveBalancesTable(a.stdout, []payroll.LeaveBalance{*balance})
		return nil

	case "initialize":
		fs := flag.NewFlagSet("leave balances initialize", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		employeeID := fs.String("employee-id", "", "Employee id")
		yearFlag := fs.String("year", "", "Balance year")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*employeeID) == "" {
			return errors.New("employee-id is required")
		}
		year, err := parseRequiredPositiveInt("year", *yearFlag)
		if err != nil {
			return err
		}

		balances, err := client.initializeLeaveBalances(ctx, cfg.TenantID, strings.TrimSpace(*employeeID), year)
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, balances)
		}
		printLeaveBalancesTable(a.stdout, balances)
		return nil

	case "import":
		fs := flag.NewFlagSet("leave balances import", flag.ContinueOnError)
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
		return fmt.Errorf("unknown leave balances subcommand %q", args[0])
	}
}

func (a *cliApp) runLeaveRecords(ctx context.Context, cfg *cliConfig, client *apiClient, args []string) error {
	if len(args) == 0 {
		return errors.New("leave records subcommand required")
	}

	switch args[0] {
	case "list":
		fs := flag.NewFlagSet("leave records list", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		employeeID := fs.String("employee-id", "", "Employee id")
		yearFlag := fs.String("year", "", "Record year")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		year, err := parseOptionalInt(*yearFlag)
		if err != nil {
			return err
		}

		records, err := client.listLeaveRecords(ctx, cfg.TenantID, strings.TrimSpace(*employeeID), year)
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, records)
		}
		printLeaveRecordsTable(a.stdout, records)
		return nil

	case "create":
		fs := flag.NewFlagSet("leave records create", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		employeeID := fs.String("employee-id", "", "Employee id")
		absenceTypeID := fs.String("absence-type-id", "", "Absence type id")
		startDate := fs.String("start-date", "", "Start date in YYYY-MM-DD")
		endDate := fs.String("end-date", "", "End date in YYYY-MM-DD")
		totalDaysFlag := fs.String("total-days", "", "Total calendar days")
		workingDaysFlag := fs.String("working-days", "", "Working days")
		documentNumber := fs.String("document-number", "", "Document number")
		documentDate := fs.String("document-date", "", "Document date in YYYY-MM-DD")
		notes := fs.String("notes", "", "Notes")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*employeeID) == "" {
			return errors.New("employee-id is required")
		}
		if strings.TrimSpace(*absenceTypeID) == "" {
			return errors.New("absence-type-id is required")
		}
		start, err := parseRequiredDate("start-date", *startDate)
		if err != nil {
			return err
		}
		end, err := parseRequiredDate("end-date", *endDate)
		if err != nil {
			return err
		}
		totalDays, err := parseRequiredPositiveDecimal("total-days", *totalDaysFlag)
		if err != nil {
			return err
		}
		workingDays, err := parseRequiredPositiveDecimal("working-days", *workingDaysFlag)
		if err != nil {
			return err
		}
		docDate, err := parseOptionalDate("document-date", *documentDate)
		if err != nil {
			return err
		}

		record, err := client.createLeaveRecord(ctx, cfg.TenantID, &payroll.CreateLeaveRecordRequest{
			EmployeeID:     strings.TrimSpace(*employeeID),
			AbsenceTypeID:  strings.TrimSpace(*absenceTypeID),
			StartDate:      start,
			EndDate:        end,
			TotalDays:      totalDays,
			WorkingDays:    workingDays,
			DocumentNumber: strings.TrimSpace(*documentNumber),
			DocumentDate:   docDate,
			Notes:          strings.TrimSpace(*notes),
		})
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, record)
		}
		_, _ = fmt.Fprintf(a.stdout, "Created leave record %s\n", record.ID)
		return nil

	case "get":
		fs := flag.NewFlagSet("leave records get", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		recordID := fs.String("id", "", "Leave record id")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*recordID) == "" {
			return errors.New("id is required")
		}

		record, err := client.getLeaveRecord(ctx, cfg.TenantID, strings.TrimSpace(*recordID))
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, record)
		}
		printLeaveRecord(a.stdout, record)
		return nil

	case "approve":
		fs := flag.NewFlagSet("leave records approve", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		recordID := fs.String("id", "", "Leave record id")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*recordID) == "" {
			return errors.New("id is required")
		}

		record, err := client.approveLeaveRecord(ctx, cfg.TenantID, strings.TrimSpace(*recordID))
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, record)
		}
		printLeaveRecord(a.stdout, record)
		return nil

	case "reject":
		fs := flag.NewFlagSet("leave records reject", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		recordID := fs.String("id", "", "Leave record id")
		reason := fs.String("reason", "", "Rejection reason")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*recordID) == "" {
			return errors.New("id is required")
		}
		if strings.TrimSpace(*reason) == "" {
			return errors.New("reason is required")
		}

		record, err := client.rejectLeaveRecord(ctx, cfg.TenantID, strings.TrimSpace(*recordID), &payroll.RejectLeaveRequest{Reason: strings.TrimSpace(*reason)})
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, record)
		}
		printLeaveRecord(a.stdout, record)
		return nil

	case "cancel":
		fs := flag.NewFlagSet("leave records cancel", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		recordID := fs.String("id", "", "Leave record id")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*recordID) == "" {
			return errors.New("id is required")
		}

		record, err := client.cancelLeaveRecord(ctx, cfg.TenantID, strings.TrimSpace(*recordID))
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, record)
		}
		printLeaveRecord(a.stdout, record)
		return nil

	default:
		return fmt.Errorf("unknown leave records subcommand %q", args[0])
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

	case "import-history":
		fs := flag.NewFlagSet("tax kmd import-history", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		filePath := fs.String("file", "", "CSV file path")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[2:]); err != nil {
			return err
		}
		if strings.TrimSpace(*filePath) == "" {
			return errors.New("file is required")
		}

		content, fileName, err := readCSVInput(*filePath)
		if err != nil {
			return err
		}
		result, err := client.importKMDHistory(ctx, cfg.TenantID, &tax.ImportKMDHistoryRequest{
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
			"Processed %d rows, created %d KMD declarations, imported %d rows, skipped %d rows\n",
			result.RowsProcessed,
			result.DeclarationsCreated,
			result.RowsImported,
			result.RowsSkipped,
		)
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
		asCSV := fs.Bool("csv", false, "Output CSV")
		asXLSX := fs.Bool("xlsx", false, "Output XLSX")
		asPDF := fs.Bool("pdf", false, "Output PDF")
		outputPath := fs.String("output", "", "Optional CSV/XLSX/PDF output file path")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if err := validateReportOutputFlags(*asJSON, *asCSV, *asXLSX, *asPDF, *outputPath); err != nil {
			return err
		}
		if *asCSV {
			content, err := client.exportTrialBalanceCSV(ctx, cfg.TenantID, strings.TrimSpace(*asOf))
			if err != nil {
				return err
			}
			return writeExportOutput(a.stdout, strings.TrimSpace(*outputPath), content, "trial balance CSV")
		}
		if *asXLSX {
			content, err := client.exportTrialBalanceXLSX(ctx, cfg.TenantID, strings.TrimSpace(*asOf))
			if err != nil {
				return err
			}
			return writeExportOutput(a.stdout, strings.TrimSpace(*outputPath), content, "trial balance XLSX")
		}
		if *asPDF {
			content, err := client.exportTrialBalancePDF(ctx, cfg.TenantID, strings.TrimSpace(*asOf))
			if err != nil {
				return err
			}
			return writeExportOutput(a.stdout, strings.TrimSpace(*outputPath), content, "trial balance PDF")
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
		asCSV := fs.Bool("csv", false, "Output CSV")
		asXLSX := fs.Bool("xlsx", false, "Output XLSX")
		asPDF := fs.Bool("pdf", false, "Output PDF")
		outputPath := fs.String("output", "", "Optional CSV/XLSX/PDF output file path")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if err := validateReportOutputFlags(*asJSON, *asCSV, *asXLSX, *asPDF, *outputPath); err != nil {
			return err
		}
		if *asCSV {
			content, err := client.exportBalanceSheetCSV(ctx, cfg.TenantID, strings.TrimSpace(*asOf))
			if err != nil {
				return err
			}
			return writeExportOutput(a.stdout, strings.TrimSpace(*outputPath), content, "balance sheet CSV")
		}
		if *asXLSX {
			content, err := client.exportBalanceSheetXLSX(ctx, cfg.TenantID, strings.TrimSpace(*asOf))
			if err != nil {
				return err
			}
			return writeExportOutput(a.stdout, strings.TrimSpace(*outputPath), content, "balance sheet XLSX")
		}
		if *asPDF {
			content, err := client.exportBalanceSheetPDF(ctx, cfg.TenantID, strings.TrimSpace(*asOf))
			if err != nil {
				return err
			}
			return writeExportOutput(a.stdout, strings.TrimSpace(*outputPath), content, "balance sheet PDF")
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
		asCSV := fs.Bool("csv", false, "Output CSV")
		asXLSX := fs.Bool("xlsx", false, "Output XLSX")
		asPDF := fs.Bool("pdf", false, "Output PDF")
		outputPath := fs.String("output", "", "Optional CSV/XLSX/PDF output file path")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if err := validateReportOutputFlags(*asJSON, *asCSV, *asXLSX, *asPDF, *outputPath); err != nil {
			return err
		}
		if strings.TrimSpace(*startDate) == "" || strings.TrimSpace(*endDate) == "" {
			return errors.New("start and end are required")
		}
		if *asCSV {
			content, err := client.exportIncomeStatementCSV(ctx, cfg.TenantID, strings.TrimSpace(*startDate), strings.TrimSpace(*endDate))
			if err != nil {
				return err
			}
			return writeExportOutput(a.stdout, strings.TrimSpace(*outputPath), content, "income statement CSV")
		}
		if *asXLSX {
			content, err := client.exportIncomeStatementXLSX(ctx, cfg.TenantID, strings.TrimSpace(*startDate), strings.TrimSpace(*endDate))
			if err != nil {
				return err
			}
			return writeExportOutput(a.stdout, strings.TrimSpace(*outputPath), content, "income statement XLSX")
		}
		if *asPDF {
			content, err := client.exportIncomeStatementPDF(ctx, cfg.TenantID, strings.TrimSpace(*startDate), strings.TrimSpace(*endDate))
			if err != nil {
				return err
			}
			return writeExportOutput(a.stdout, strings.TrimSpace(*outputPath), content, "income statement PDF")
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
		asCSV := fs.Bool("csv", false, "Output CSV")
		asXLSX := fs.Bool("xlsx", false, "Output XLSX")
		asPDF := fs.Bool("pdf", false, "Output PDF")
		outputPath := fs.String("output", "", "Optional CSV/XLSX/PDF output file path")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if err := validateReportOutputFlags(*asJSON, *asCSV, *asXLSX, *asPDF, *outputPath); err != nil {
			return err
		}
		if strings.TrimSpace(*startDate) == "" || strings.TrimSpace(*endDate) == "" {
			return errors.New("start and end are required")
		}
		if *asCSV {
			content, err := client.exportCashFlowStatementCSV(ctx, cfg.TenantID, strings.TrimSpace(*startDate), strings.TrimSpace(*endDate))
			if err != nil {
				return err
			}
			return writeExportOutput(a.stdout, strings.TrimSpace(*outputPath), content, "cash flow CSV")
		}
		if *asXLSX {
			content, err := client.exportCashFlowStatementXLSX(ctx, cfg.TenantID, strings.TrimSpace(*startDate), strings.TrimSpace(*endDate))
			if err != nil {
				return err
			}
			return writeExportOutput(a.stdout, strings.TrimSpace(*outputPath), content, "cash flow XLSX")
		}
		if *asPDF {
			content, err := client.exportCashFlowStatementPDF(ctx, cfg.TenantID, strings.TrimSpace(*startDate), strings.TrimSpace(*endDate))
			if err != nil {
				return err
			}
			return writeExportOutput(a.stdout, strings.TrimSpace(*outputPath), content, "cash flow PDF")
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
		asCSV := fs.Bool("csv", false, "Output CSV")
		asXLSX := fs.Bool("xlsx", false, "Output XLSX")
		asPDF := fs.Bool("pdf", false, "Output PDF")
		outputPath := fs.String("output", "", "Optional CSV/XLSX/PDF output file path")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if err := validateReportOutputFlags(*asJSON, *asCSV, *asXLSX, *asPDF, *outputPath); err != nil {
			return err
		}
		normalizedType := strings.ToLower(strings.TrimSpace(*reportType))
		if normalizedType != "receivables" && normalizedType != "payables" {
			return errors.New("type must be receivables or payables")
		}

		if *asCSV {
			content, err := client.exportAgingReport(ctx, cfg.TenantID, normalizedType, "csv")
			if err != nil {
				return err
			}
			return writeExportOutput(a.stdout, strings.TrimSpace(*outputPath), content, normalizedType+" aging CSV")
		}
		if *asXLSX {
			content, err := client.exportAgingReport(ctx, cfg.TenantID, normalizedType, "xlsx")
			if err != nil {
				return err
			}
			return writeExportOutput(a.stdout, strings.TrimSpace(*outputPath), content, normalizedType+" aging XLSX")
		}
		if *asPDF {
			content, err := client.exportAgingReport(ctx, cfg.TenantID, normalizedType, "pdf")
			if err != nil {
				return err
			}
			return writeExportOutput(a.stdout, strings.TrimSpace(*outputPath), content, normalizedType+" aging PDF")
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
		asCSV := fs.Bool("csv", false, "Output CSV")
		asXLSX := fs.Bool("xlsx", false, "Output XLSX")
		asPDF := fs.Bool("pdf", false, "Output PDF")
		outputPath := fs.String("output", "", "Optional CSV/XLSX/PDF output file path")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if err := validateReportOutputFlags(*asJSON, *asCSV, *asXLSX, *asPDF, *outputPath); err != nil {
			return err
		}
		normalizedType := strings.ToUpper(strings.TrimSpace(*balanceType))
		if normalizedType != "RECEIVABLE" && normalizedType != "PAYABLE" {
			return errors.New("type must be RECEIVABLE or PAYABLE")
		}
		if strings.TrimSpace(*asOf) == "" {
			return errors.New("as-of is required")
		}

		if *asCSV {
			content, err := client.exportBalanceConfirmationSummary(ctx, cfg.TenantID, normalizedType, strings.TrimSpace(*asOf), "csv")
			if err != nil {
				return err
			}
			return writeExportOutput(a.stdout, strings.TrimSpace(*outputPath), content, "balance confirmations CSV")
		}
		if *asXLSX {
			content, err := client.exportBalanceConfirmationSummary(ctx, cfg.TenantID, normalizedType, strings.TrimSpace(*asOf), "xlsx")
			if err != nil {
				return err
			}
			return writeExportOutput(a.stdout, strings.TrimSpace(*outputPath), content, "balance confirmations XLSX")
		}
		if *asPDF {
			content, err := client.exportBalanceConfirmationSummary(ctx, cfg.TenantID, normalizedType, strings.TrimSpace(*asOf), "pdf")
			if err != nil {
				return err
			}
			return writeExportOutput(a.stdout, strings.TrimSpace(*outputPath), content, "balance confirmations PDF")
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
		asCSV := fs.Bool("csv", false, "Output CSV")
		asXLSX := fs.Bool("xlsx", false, "Output XLSX")
		asPDF := fs.Bool("pdf", false, "Output PDF")
		outputPath := fs.String("output", "", "Optional CSV/XLSX/PDF output file path")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if err := validateReportOutputFlags(*asJSON, *asCSV, *asXLSX, *asPDF, *outputPath); err != nil {
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

		if *asCSV {
			content, err := client.exportBalanceConfirmation(ctx, cfg.TenantID, strings.TrimSpace(*contactID), normalizedType, strings.TrimSpace(*asOf), "csv")
			if err != nil {
				return err
			}
			return writeExportOutput(a.stdout, strings.TrimSpace(*outputPath), content, "balance confirmation CSV")
		}
		if *asXLSX {
			content, err := client.exportBalanceConfirmation(ctx, cfg.TenantID, strings.TrimSpace(*contactID), normalizedType, strings.TrimSpace(*asOf), "xlsx")
			if err != nil {
				return err
			}
			return writeExportOutput(a.stdout, strings.TrimSpace(*outputPath), content, "balance confirmation XLSX")
		}
		if *asPDF {
			content, err := client.exportBalanceConfirmation(ctx, cfg.TenantID, strings.TrimSpace(*contactID), normalizedType, strings.TrimSpace(*asOf), "pdf")
			if err != nil {
				return err
			}
			return writeExportOutput(a.stdout, strings.TrimSpace(*outputPath), content, "balance confirmation PDF")
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

func validateReportOutputFlags(asJSON, asCSV, asXLSX, asPDF bool, outputPath string) error {
	selected := 0
	for _, value := range []bool{asJSON, asCSV, asXLSX, asPDF} {
		if value {
			selected++
		}
	}
	if selected > 1 {
		return errors.New("json, csv, xlsx, and pdf cannot be combined")
	}
	if strings.TrimSpace(outputPath) != "" && !asCSV && !asXLSX && !asPDF {
		return errors.New("output requires csv, xlsx, or pdf")
	}
	return nil
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

	case "review-summary":
		fs := flag.NewFlagSet("documents review-summary", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		entityType := fs.String("entity-type", "", "Entity type: invoice, journal_entry, payment, bank_transaction, asset")
		entityIDs := stringListFlags{}
		fs.Var(&entityIDs, "entity-id", "Entity id; repeatable")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*entityType) == "" {
			return errors.New("entity-type is required")
		}
		if len(entityIDs) == 0 {
			return errors.New("at least one entity-id is required")
		}

		summaries, err := client.listDocumentReviewSummaries(ctx, cfg.TenantID, &documentReviewSummaryRequest{
			EntityType: strings.TrimSpace(*entityType),
			EntityIDs:  []string(entityIDs),
		})
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, summaries)
		}
		printDocumentReviewSummariesTable(a.stdout, summaries)
		return nil

	case "evidence-policy":
		fs := flag.NewFlagSet("documents evidence-policy", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		entityType := fs.String("entity-type", "", "Entity type: invoice, journal_entry, payment, bank_transaction, asset")
		entityIDs := stringListFlags{}
		documentTypes := stringListFlags{}
		fs.Var(&entityIDs, "entity-id", "Entity id; repeatable")
		fs.Var(&documentTypes, "document-type", "Document type that satisfies the policy; repeatable")
		fs.Var(&documentTypes, "required-document-type", "Document type that satisfies the policy; repeatable")
		minCount := fs.Int("min-count", 1, "Minimum number of matching documents")
		requireApproved := fs.Bool("require-approved", false, "Only count approved matching documents")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*entityType) == "" {
			return errors.New("entity-type is required")
		}
		if len(entityIDs) == 0 {
			return errors.New("at least one entity-id is required")
		}
		if *minCount <= 0 {
			return errors.New("min-count must be one or greater")
		}

		results, err := client.evaluateDocumentEvidencePolicy(ctx, cfg.TenantID, &documents.EvidencePolicyRequest{
			EntityType: strings.TrimSpace(*entityType),
			EntityIDs:  []string(entityIDs),
			Rules: []documents.EvidencePolicyRule{{
				DocumentTypes:   []string(documentTypes),
				MinCount:        *minCount,
				RequireApproved: *requireApproved,
			}},
		})
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, results)
		}
		printDocumentEvidencePolicy(a.stdout, results)
		return nil

	case "retention":
		fs := flag.NewFlagSet("documents retention", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		asOf := fs.String("as-of", "", "As-of date in YYYY-MM-DD; defaults to today")
		horizonDays := fs.Int("horizon-days", 30, "Include documents due within this many days")
		includeMissing := fs.Bool("include-missing", false, "Include documents without retention_until")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *horizonDays < 0 {
			return errors.New("horizon-days must be zero or greater")
		}

		review, err := client.getDocumentRetentionReview(ctx, cfg.TenantID, strings.TrimSpace(*asOf), *horizonDays, *includeMissing)
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, review)
		}
		printDocumentRetentionReview(a.stdout, review)
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

	case "download":
		fs := flag.NewFlagSet("documents download", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		documentID := fs.String("id", "", "Document id")
		outputPath := fs.String("output", "", "Output path; defaults to downloaded file name, '-' for stdout")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*documentID) == "" {
			return errors.New("id is required")
		}

		download, err := client.downloadDocument(ctx, cfg.TenantID, strings.TrimSpace(*documentID))
		if err != nil {
			return err
		}
		targetPath := strings.TrimSpace(*outputPath)
		if targetPath == "" {
			targetPath = download.FileName
		}
		if targetPath == "-" {
			_, err := a.stdout.Write(download.Content)
			return err
		}
		if strings.TrimSpace(targetPath) == "" {
			targetPath = strings.TrimSpace(*documentID)
		}
		if err := os.WriteFile(targetPath, download.Content, 0o600); err != nil {
			return fmt.Errorf("write document: %w", err)
		}
		_, _ = fmt.Fprintf(a.stdout, "Downloaded %s to %s (%d bytes)\n", strings.TrimSpace(*documentID), targetPath, len(download.Content))
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

	case "review":
		fs := flag.NewFlagSet("documents review", flag.ContinueOnError)
		fs.SetOutput(a.stderr)
		documentID := fs.String("id", "", "Document id")
		reviewStatus := fs.String("status", "", "Review status: REVIEWED, APPROVED, or REJECTED")
		reviewNote := fs.String("note", "", "Optional review note; required for rejected documents")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*documentID) == "" || strings.TrimSpace(*reviewStatus) == "" {
			return errors.New("id and status are required")
		}

		doc, err := client.reviewDocument(ctx, cfg.TenantID, strings.TrimSpace(*documentID), &documents.ReviewDocumentRequest{
			ReviewStatus: strings.ToUpper(strings.TrimSpace(*reviewStatus)),
			ReviewNote:   strings.TrimSpace(*reviewNote),
		})
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a.stdout, doc)
		}
		_, _ = fmt.Fprintf(a.stdout, "Reviewed document %s as %s\n", doc.ID, doc.ReviewStatus)
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
	cfg, client, err := a.loadTokenClient()
	if err != nil {
		return nil, nil, err
	}
	if strings.TrimSpace(cfg.TenantID) == "" {
		return nil, nil, errors.New("no tenant configured, run `oa auth init` first")
	}
	return cfg, client, nil
}

func (a *cliApp) loadTokenClient() (*cliConfig, *apiClient, error) {
	cfg, err := loadRuntimeConfig()
	if err != nil {
		return nil, nil, err
	}
	if strings.TrimSpace(cfg.APIToken) == "" {
		return nil, nil, errors.New("no API token configured, run `oa auth init` first")
	}
	return cfg, newAPIClient(cfg.BaseURL, cfg.APIToken), nil
}

func (a *cliApp) loadPublicClient(baseURL string) (*apiClient, error) {
	if strings.TrimSpace(baseURL) != "" {
		return newAPIClient(baseURL, ""), nil
	}
	cfg, err := loadRuntimeConfig()
	if err != nil {
		return nil, err
	}
	return newAPIClient(cfg.BaseURL, ""), nil
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

func parseTenantSettingsInput(settingsJSON, settingsFile string) (*tenant.TenantSettings, error) {
	settingsJSON = strings.TrimSpace(settingsJSON)
	settingsFile = strings.TrimSpace(settingsFile)
	if settingsJSON == "" && settingsFile == "" {
		return nil, nil
	}
	if settingsJSON != "" && settingsFile != "" {
		return nil, errors.New("use either settings-json or settings-file, not both")
	}

	payload := []byte(settingsJSON)
	if settingsFile != "" {
		content, err := os.ReadFile(settingsFile)
		if err != nil {
			return nil, fmt.Errorf("read settings file: %w", err)
		}
		payload = content
	}

	var settings tenant.TenantSettings
	if err := json.Unmarshal(payload, &settings); err != nil {
		return nil, fmt.Errorf("parse tenant settings JSON: %w", err)
	}
	return &settings, nil
}

func parseRawJSONInput(inlineJSON, filePath, defaultJSON string) (json.RawMessage, error) {
	inlineJSON = strings.TrimSpace(inlineJSON)
	filePath = strings.TrimSpace(filePath)
	if inlineJSON == "" && filePath == "" {
		if strings.TrimSpace(defaultJSON) == "" {
			return nil, nil
		}
		return json.RawMessage(defaultJSON), nil
	}
	if inlineJSON != "" && filePath != "" {
		return nil, errors.New("use either settings-json or settings-file, not both")
	}

	payload := []byte(inlineJSON)
	if filePath != "" {
		content, err := os.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("read settings file: %w", err)
		}
		payload = content
	}

	var value any
	if err := json.Unmarshal(payload, &value); err != nil {
		return nil, fmt.Errorf("parse JSON: %w", err)
	}
	return json.RawMessage(payload), nil
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

func parseRequiredBoundedInt(name, value string, min, max int) (int, error) {
	parsed, err := parseRequiredPositiveInt(name, value)
	if err != nil {
		return 0, err
	}
	if parsed < min || parsed > max {
		return 0, fmt.Errorf("%s must be between %d and %d", name, min, max)
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

func parseOptionalBoolPtr(name, value string) (*bool, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}
	parsed, err := strconv.ParseBool(strings.TrimSpace(value))
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", name, err)
	}
	return &parsed, nil
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

func parseRequiredDecimal(name, value string) (decimal.Decimal, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return decimal.Zero, fmt.Errorf("%s is required", name)
	}
	parsed, err := decimal.NewFromString(trimmed)
	if err != nil {
		return decimal.Zero, fmt.Errorf("parse %s: %w", name, err)
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

func parseOptionalBankTransactionStatus(value string) (banking.TransactionStatus, error) {
	if strings.TrimSpace(value) == "" {
		return "", nil
	}
	normalized := strings.ToUpper(strings.TrimSpace(value))
	switch banking.TransactionStatus(normalized) {
	case banking.StatusUnmatched, banking.StatusMatched, banking.StatusReconciled:
		return banking.TransactionStatus(normalized), nil
	default:
		return "", fmt.Errorf("invalid bank transaction status %q", value)
	}
}

func parseRequiredBankFollowUpStatus(value string) (banking.FollowUpStatus, error) {
	status, err := banking.NormalizeFollowUpStatus(value)
	if err != nil {
		if strings.TrimSpace(value) == "" {
			return "", errors.New("follow-up-status is required")
		}
		return "", fmt.Errorf("invalid bank follow-up status %q", value)
	}
	return status, nil
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

func parseOptionalProductType(value string) (inventory.ProductType, error) {
	if strings.TrimSpace(value) == "" {
		return "", nil
	}
	return parseRequiredProductType(value)
}

func parseRequiredProductType(value string) (inventory.ProductType, error) {
	normalized := strings.ToUpper(strings.TrimSpace(value))
	switch inventory.ProductType(normalized) {
	case inventory.ProductTypeGoods, inventory.ProductTypeService:
		return inventory.ProductType(normalized), nil
	default:
		if normalized == "" {
			return "", errors.New("type is required")
		}
		return "", fmt.Errorf("invalid product type %q", value)
	}
}

func parseOptionalProductStatus(value string) (inventory.ProductStatus, error) {
	if strings.TrimSpace(value) == "" {
		return "", nil
	}
	normalized := strings.ToUpper(strings.TrimSpace(value))
	switch inventory.ProductStatus(normalized) {
	case inventory.ProductStatusActive, inventory.ProductStatusInactive:
		return inventory.ProductStatus(normalized), nil
	default:
		return "", fmt.Errorf("invalid product status %q", value)
	}
}

func parseRequiredRecurringFrequency(value string) (recurring.Frequency, error) {
	normalized := strings.ToUpper(strings.TrimSpace(value))
	switch recurring.Frequency(normalized) {
	case recurring.FrequencyWeekly, recurring.FrequencyBiweekly, recurring.FrequencyMonthly, recurring.FrequencyQuarterly, recurring.FrequencyYearly:
		return recurring.Frequency(normalized), nil
	default:
		if normalized == "" {
			return "", errors.New("frequency is required")
		}
		return "", fmt.Errorf("invalid recurring invoice frequency %q", value)
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

func parseRequiredReminderTriggerType(value string) (invoicing.TriggerType, error) {
	normalized := strings.ToUpper(strings.TrimSpace(value))
	switch invoicing.TriggerType(normalized) {
	case invoicing.TriggerBeforeDue, invoicing.TriggerOnDue, invoicing.TriggerAfterDue:
		return invoicing.TriggerType(normalized), nil
	default:
		if normalized == "" {
			return "", errors.New("trigger-type is required")
		}
		return "", fmt.Errorf("invalid reminder trigger type %q", value)
	}
}

func parseRequiredEmailTemplateType(value string) (email.TemplateType, error) {
	normalized := strings.ToUpper(strings.TrimSpace(value))
	switch email.TemplateType(normalized) {
	case email.TemplateInvoiceSend, email.TemplatePaymentReceipt, email.TemplateOverdueReminder:
		return email.TemplateType(normalized), nil
	default:
		if normalized == "" {
			return "", errors.New("type is required")
		}
		return "", fmt.Errorf("invalid email template type %q", value)
	}
}

func parseInterestRateFlags(rateValue, annualRateValue string) (float64, error) {
	hasRate := strings.TrimSpace(rateValue) != ""
	hasAnnualRate := strings.TrimSpace(annualRateValue) != ""
	if hasRate && hasAnnualRate {
		return 0, errors.New("rate and annual-rate cannot both be set")
	}
	if !hasRate && !hasAnnualRate {
		return 0, errors.New("rate or annual-rate is required")
	}

	sourceName := "rate"
	sourceValue := rateValue
	divisor := 1.0
	if hasAnnualRate {
		sourceName = "annual-rate"
		sourceValue = annualRateValue
		divisor = 365
	}

	parsed, err := strconv.ParseFloat(strings.TrimSpace(sourceValue), 64)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", sourceName, err)
	}
	rate := parsed / divisor
	req := &invoicing.UpdateInterestSettingsRequest{Rate: rate}
	if err := req.Validate(); err != nil {
		return 0, err
	}
	return rate, nil
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

type recurringLineFlags []recurring.CreateRecurringInvoiceLineRequest

func (l *recurringLineFlags) Set(value string) error {
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

	*l = append(*l, recurring.CreateRecurringInvoiceLineRequest{
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

func (l *recurringLineFlags) String() string {
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

type stringListFlags []string

func (f *stringListFlags) Set(value string) error {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return errors.New("value cannot be empty")
	}
	*f = append(*f, trimmed)
	return nil
}

func (f *stringListFlags) String() string {
	if f == nil {
		return ""
	}
	return strings.Join(*f, ",")
}

func resolveTextFlag(name, inlineValue, filePath string) (string, error) {
	if strings.TrimSpace(inlineValue) != "" && strings.TrimSpace(filePath) != "" {
		return "", fmt.Errorf("%s and %s-file cannot both be set", name, name)
	}
	if strings.TrimSpace(filePath) == "" {
		return inlineValue, nil
	}
	data, _, err := readFileInput(strings.TrimSpace(filePath), name+".txt")
	if err != nil {
		return "", err
	}
	return string(data), nil
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

func parseBankTransactionCSVRows(content string) ([]banking.CSVTransactionRow, error) {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return nil, errors.New("bank transaction CSV is empty")
	}

	reader := csv.NewReader(strings.NewReader(trimmed))
	reader.Comma = detectCLICSVDelimiter(trimmed)
	reader.TrimLeadingSpace = true
	reader.FieldsPerRecord = -1

	headers, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("read bank transaction CSV header: %w", err)
	}

	index := make(map[string]int, len(headers))
	for i, header := range headers {
		key := strings.ReplaceAll(strings.ToLower(strings.TrimSpace(header)), "-", "_")
		key = strings.ReplaceAll(key, " ", "_")
		index[key] = i
	}

	get := func(record []string, names ...string) string {
		for _, name := range names {
			if i, ok := index[name]; ok && i < len(record) {
				return strings.TrimSpace(record[i])
			}
		}
		return ""
	}

	var rows []banking.CSVTransactionRow
	for rowNum := 2; ; rowNum++ {
		record, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read bank transaction CSV row %d: %w", rowNum, err)
		}
		empty := true
		for _, field := range record {
			if strings.TrimSpace(field) != "" {
				empty = false
				break
			}
		}
		if empty {
			continue
		}

		row := banking.CSVTransactionRow{
			Date:                get(record, "date", "transaction_date"),
			ValueDate:           get(record, "value_date"),
			Amount:              get(record, "amount", "sum"),
			Description:         get(record, "description", "details", "selgitus"),
			Reference:           get(record, "reference", "payment_reference"),
			CounterpartyName:    get(record, "counterparty_name", "counterparty", "name"),
			CounterpartyAccount: get(record, "counterparty_account", "counterparty_iban", "iban"),
			ExternalID:          get(record, "external_id", "id"),
		}
		if row.Date == "" || row.Amount == "" || row.Description == "" {
			return nil, fmt.Errorf("bank transaction CSV row %d requires date, amount, and description", rowNum)
		}
		rows = append(rows, row)
	}

	if len(rows) == 0 {
		return nil, errors.New("bank transaction CSV contains no transactions")
	}
	return rows, nil
}

func detectCLICSVDelimiter(content string) rune {
	firstLine := content
	if idx := strings.IndexAny(content, "\r\n"); idx >= 0 {
		firstLine = content[:idx]
	}
	delimiters := []rune{',', ';', '\t'}
	bestDelimiter := ','
	bestCount := -1
	for _, delimiter := range delimiters {
		count := strings.Count(firstLine, string(delimiter))
		if count > bestCount {
			bestCount = count
			bestDelimiter = delimiter
		}
	}
	return bestDelimiter
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
