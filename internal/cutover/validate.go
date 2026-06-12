package cutover

import (
	"encoding/csv"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/HMB-research/open-accounting/internal/invoicing/mappers/einvoice"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type fileSpec struct {
	aliases        map[string]string
	requiredGroups [][]string
}

type parsedFile struct {
	kind     FileKind
	fileName string
	headers  []string
	rows     []parsedRow
}

type parsedRow struct {
	number int
	values map[string]string
}

type bundleIndexes struct {
	files             map[FileKind]bool
	accounts          map[string]bool
	bankAccounts      map[string]string
	contacts          map[string]bool
	employees         map[string]bool
	invoices          map[string]bool
	quotes            map[string]bool
	costCenters       map[string]bool
	productCategories map[string]bool
	products          map[string]bool
	warehouses        map[string]bool
}

type duplicateIdentifierSpec struct {
	field     string
	normalize func(string) string
}

type duplicateIdentifierValue struct {
	row int
}

type groupedDocumentSpec struct {
	keyLabel string
	key      []groupedFieldSpec
	fields   []groupedFieldSpec
}

type groupedFieldSpec struct {
	field        string
	optional     bool
	defaultValue string
	defaultFrom  string
	normalize    func(string) string
}

type groupedComparableValue struct {
	normalized string
	display    string
}

type groupedSeenValue struct {
	normalized string
}

var fileSpecs = map[FileKind]fileSpec{
	KindAccounts: {
		aliases: mergeAliases(commonAliases(), map[string]string{
			"account_code": "code",
			"account_name": "name",
			"account_type": "account_type",
			"type":         "account_type",
			"parent_code":  "parent_code",
		}),
		requiredGroups: [][]string{{"code"}, {"name"}, {"account_type"}},
	},
	KindContacts: {
		aliases: mergeAliases(commonAliases(), map[string]string{
			"contact_id":         "id",
			"contact_name":       "name",
			"company":            "name",
			"company_name":       "name",
			"contact_code":       "code",
			"customer_code":      "code",
			"supplier_code":      "code",
			"contact_type":       "contact_type",
			"type":               "contact_type",
			"reg_code":           "reg_code",
			"registration_code":  "reg_code",
			"registry_code":      "reg_code",
			"vat_number":         "vat_number",
			"vat_reg_number":     "vat_number",
			"contact_vat_number": "vat_number",
			"contact_email":      "email",
			"e_mail":             "email",
		}),
		requiredGroups: [][]string{{"name"}},
	},
	KindEmployees: {
		aliases: mergeAliases(commonAliases(), map[string]string{
			"employee_number":       "employee_number",
			"employee_no":           "employee_number",
			"employee_code":         "employee_number",
			"employee_id":           "employee_number",
			"personal_code":         "personal_code",
			"isikukood":             "personal_code",
			"e_mail":                "email",
			"base_salary":           "base_salary",
			"salary_effective_from": "salary_effective_from",
		}),
		requiredGroups: [][]string{{"first_name", "name"}, {"last_name", "name"}},
	},
	KindExpenses: {
		aliases: mergeAliases(commonAliases(), map[string]string{
			"expense_number":       "expense_number",
			"expense_no":           "expense_number",
			"number":               "expense_number",
			"date":                 "expense_date",
			"expense_date":         "expense_date",
			"supplier":             "merchant",
			"vendor":               "merchant",
			"merchant":             "merchant",
			"notes":                "description",
			"employee_id":          "employee_id",
			"contact_id":           "contact_id",
			"expense_account_id":   "expense_account_id",
			"expense_account":      "expense_account_id",
			"expense_account_code": "expense_account_code",
			"payment_account_id":   "payment_account_id",
			"payment_account":      "payment_account_id",
			"payment_account_code": "payment_account_code",
			"requires_receipt":     "requires_receipt",
			"receipt_required":     "requires_receipt",
		}),
		requiredGroups: [][]string{
			{"expense_date"},
			{"merchant"},
			{"expense_account_id", "expense_account_code"},
			{"payment_account_id", "payment_account_code"},
			{"amount"},
		},
	},
	KindInvoices: {
		aliases: mergeAliases(commonAliases(), map[string]string{
			"invoice_id":         "id",
			"invoice_number":     "invoice_number",
			"number":             "invoice_number",
			"invoice_no":         "invoice_number",
			"invoice_type":       "invoice_type",
			"type":               "invoice_type",
			"contact_code":       "contact_code",
			"customer_code":      "contact_code",
			"supplier_code":      "contact_code",
			"contact_reg_code":   "contact_reg_code",
			"contact_vat_number": "contact_vat_number",
			"vat_number":         "contact_vat_number",
			"contact_email":      "contact_email",
			"email":              "contact_email",
			"contact_name":       "contact_name",
			"customer_name":      "contact_name",
			"supplier_name":      "contact_name",
			"issue_date":         "issue_date",
			"invoice_date":       "issue_date",
			"line_description":   "line_description",
			"description":        "line_description",
			"qty":                "quantity",
			"price":              "unit_price",
			"vat":                "vat_rate",
			"product":            "product_code",
			"product_code":       "product_code",
			"sku":                "product_code",
			"item_code":          "product_code",
			"product_id":         "product_id",
		}),
		requiredGroups: [][]string{
			{"invoice_number"},
			{"invoice_type"},
			{"issue_date"},
			{"contact_code", "contact_reg_code", "contact_vat_number", "contact_email", "contact_name"},
			{"line_description"},
			{"quantity"},
			{"unit_price"},
			{"vat_rate"},
		},
	},
	KindPayments: {
		aliases: mergeAliases(commonAliases(), map[string]string{
			"payment_number":    "payment_number",
			"payment_no":        "payment_number",
			"number":            "payment_number",
			"type":              "payment_type",
			"payment_type":      "payment_type",
			"date":              "payment_date",
			"payment_date":      "payment_date",
			"invoice_id":        "invoice_id",
			"invoice_number":    "invoice_number",
			"allocation_amount": "allocation_amount",
			"allocated_amount":  "allocation_amount",
		}),
		requiredGroups: [][]string{{"payment_type"}, {"payment_date"}, {"amount"}},
	},
	KindBankAccounts: {
		aliases: mergeAliases(commonAliases(), map[string]string{
			"account_name":        "name",
			"bank_account_name":   "name",
			"account_number":      "account_number",
			"iban":                "account_number",
			"bank_account":        "account_number",
			"account_no":          "account_number",
			"bank":                "bank_name",
			"bank_name":           "bank_name",
			"bic":                 "swift_code",
			"swift":               "swift_code",
			"swift_code":          "swift_code",
			"gl_account_id":       "gl_account_id",
			"ledger_account_id":   "gl_account_id",
			"gl_account_code":     "gl_account_code",
			"ledger_account_code": "gl_account_code",
			"cash_account_code":   "gl_account_code",
			"default":             "is_default",
			"is_default":          "is_default",
			"active":              "is_active",
			"is_active":           "is_active",
		}),
		requiredGroups: [][]string{{"name"}, {"account_number"}},
	},
	KindBankTransactions: {
		aliases: mergeAliases(commonAliases(), map[string]string{
			"date":                 "date",
			"transaction_date":     "date",
			"booking_date":         "date",
			"value_date":           "value_date",
			"amount":               "amount",
			"sum":                  "amount",
			"source_account":       "source_account",
			"client_account":       "source_account",
			"account_number":       "source_account",
			"bank_account":         "source_account",
			"description":          "description",
			"details":              "description",
			"reference":            "reference",
			"payment_reference":    "reference",
			"counterparty_name":    "counterparty_name",
			"counterparty":         "counterparty_name",
			"counterparty_account": "counterparty_account",
			"counterparty_iban":    "counterparty_account",
			"external_id":          "external_id",
			"entry_reference":      "external_id",
		}),
		requiredGroups: [][]string{{"date"}, {"amount"}},
	},
	KindPayrollHistory: {
		aliases: mergeAliases(employeeReferenceAliases(), map[string]string{
			"period_year":   "period_year",
			"payroll_year":  "period_year",
			"year":          "period_year",
			"period_month":  "period_month",
			"payroll_month": "period_month",
			"month":         "period_month",
			"gross":         "gross_salary",
			"gross_salary":  "gross_salary",
		}),
		requiredGroups: payrollHistoryRequiredGroups(),
	},
	KindLeaveBalances: {
		aliases: mergeAliases(employeeReferenceAliases(), map[string]string{
			"year":              "year",
			"absence_type":      "absence_type",
			"absence_type_code": "absence_type_code",
			"type_code":         "absence_type_code",
			"entitled":          "entitled_days",
			"entitled_days":     "entitled_days",
		}),
		requiredGroups: leaveBalanceRequiredGroups(),
	},
	KindTSDHistory: {
		aliases: mergeAliases(employeeReferenceAliases(), map[string]string{
			"period_year":                     "period_year",
			"declaration_year":                "period_year",
			"tsd_year":                        "period_year",
			"year":                            "period_year",
			"period_month":                    "period_month",
			"declaration_month":               "period_month",
			"tsd_month":                       "period_month",
			"month":                           "period_month",
			"declaration_status":              "status",
			"submitted_at":                    "submitted_at",
			"submitted_date":                  "submitted_at",
			"submission_date":                 "submitted_at",
			"emta_reference":                  "emta_reference",
			"submission_reference":            "emta_reference",
			"payment_type":                    "payment_type",
			"payment_code":                    "payment_type",
			"gross":                           "gross_payment",
			"gross_salary":                    "gross_payment",
			"gross_payment":                   "gross_payment",
			"basic_exemption_applied":         "basic_exemption",
			"taxable_income":                  "taxable_amount",
			"unemployment_insurance_employee": "unemployment_insurance_employee",
			"unemployment_employee":           "unemployment_insurance_employee",
			"unemployment_insurance_ee":       "unemployment_insurance_employee",
			"unemployment_insurance_employer": "unemployment_insurance_employer",
			"unemployment_employer":           "unemployment_insurance_employer",
			"unemployment_insurance_er":       "unemployment_insurance_employer",
			"pension":                         "funded_pension",
		}),
		requiredGroups: tsdHistoryRequiredGroups(),
	},
	KindKMDHistory: {
		aliases: mergeAliases(commonAliases(), map[string]string{
			"period_year":        "year",
			"declaration_year":   "year",
			"kmd_year":           "year",
			"period_month":       "month",
			"declaration_month":  "month",
			"kmd_month":          "month",
			"declaration_status": "status",
			"submitted_at":       "submitted_at",
			"submitted_date":     "submitted_at",
			"submission_date":    "submitted_at",
			"row_code":           "row_code",
			"code":               "row_code",
			"kmd_row":            "row_code",
			"kmd_code":           "row_code",
			"row_description":    "description",
			"tax_base":           "tax_base",
			"base":               "tax_base",
			"taxable_amount":     "tax_base",
			"tax_amount":         "tax_amount",
			"vat_amount":         "tax_amount",
			"amount":             "tax_amount",
			"total_output_vat":   "total_output_vat",
			"output_vat":         "total_output_vat",
			"total_input_vat":    "total_input_vat",
			"input_vat":          "total_input_vat",
		}),
		requiredGroups: [][]string{{"year"}, {"month"}, {"row_code"}},
	},
	KindQuotes: {
		aliases: mergeAliases(commercialDocumentAliases(), map[string]string{
			"quote_id":         "id",
			"quote_number":     "quote_number",
			"quotation_number": "quote_number",
			"offer_number":     "quote_number",
			"number":           "quote_number",
			"quote_no":         "quote_number",
			"quote_date":       "quote_date",
			"date":             "quote_date",
			"valid_until":      "valid_until",
			"valid_to":         "valid_until",
			"expiry_date":      "valid_until",
			"expires_at":       "valid_until",
		}),
		requiredGroups: commercialDocumentRequiredGroups("quote_number", "quote_date"),
	},
	KindOrders: {
		aliases: mergeAliases(commercialDocumentAliases(), map[string]string{
			"order_number":      "order_number",
			"sales_order":       "order_number",
			"sales_order_no":    "order_number",
			"number":            "order_number",
			"order_no":          "order_number",
			"order_date":        "order_date",
			"date":              "order_date",
			"expected_delivery": "expected_delivery",
			"delivery_date":     "expected_delivery",
			"quote_id":          "quote_id",
		}),
		requiredGroups: commercialDocumentRequiredGroups("order_number", "order_date"),
	},
	KindRecurringInvoices: {
		aliases: mergeAliases(commercialDocumentAliases(), map[string]string{
			"template":                 "name",
			"template_name":            "name",
			"recurring_name":           "name",
			"frequency":                "frequency",
			"start_date":               "start_date",
			"end_date":                 "end_date",
			"next_generation_date":     "next_generation_date",
			"next_date":                "next_generation_date",
			"payment_terms_days":       "payment_terms_days",
			"payment_terms":            "payment_terms_days",
			"send_email_on_generation": "send_email_on_generation",
			"send_email":               "send_email_on_generation",
			"email_template_type":      "email_template_type",
			"recipient_email_override": "recipient_email_override",
			"recipient_email":          "recipient_email_override",
			"attach_pdf_to_email":      "attach_pdf_to_email",
			"attach_pdf":               "attach_pdf_to_email",
			"email_subject_override":   "email_subject_override",
			"email_subject":            "email_subject_override",
			"email_message":            "email_message",
			"account_id":               "account_id",
		}),
		requiredGroups: append(commercialDocumentRequiredGroups("name", "start_date"), []string{"frequency"}),
	},
	KindCostCenters: {
		aliases: mergeAliases(commonAliases(), map[string]string{
			"cost_center_code":        "code",
			"cc_code":                 "code",
			"cost_center_name":        "name",
			"cc_name":                 "name",
			"parent_id":               "parent_id",
			"parent_code":             "parent_code",
			"parent":                  "parent_code",
			"parent_cost_center_code": "parent_code",
			"budget_amount":           "budget_amount",
			"budget":                  "budget_amount",
			"budget_period":           "budget_period",
			"is_active":               "is_active",
			"active":                  "is_active",
		}),
		requiredGroups: [][]string{{"code"}, {"name"}},
	},
	KindCostAllocations: {
		aliases: mergeAliases(commonAliases(), map[string]string{
			"cost_center_id":        "cost_center_id",
			"cost_center":           "cost_center_code",
			"cost_center_code":      "cost_center_code",
			"cc_code":               "cost_center_code",
			"journal_entry_line_id": "journal_entry_line_id",
			"journal_line_id":       "journal_entry_line_id",
			"line_id":               "journal_entry_line_id",
			"allocation_amount":     "amount",
			"allocation_percentage": "allocation_percentage",
			"percentage":            "allocation_percentage",
			"allocation_percent":    "allocation_percentage",
			"allocation_date":       "allocation_date",
			"date":                  "allocation_date",
			"notes":                 "notes",
			"note":                  "notes",
			"memo":                  "notes",
		}),
		requiredGroups: [][]string{{"cost_center_id", "cost_center_code"}, {"journal_entry_line_id"}, {"amount"}, {"allocation_date"}},
	},
	KindProductCategories: {
		aliases: mergeAliases(commonAliases(), map[string]string{
			"category":         "name",
			"category_name":    "name",
			"product_category": "name",
			"parent":           "parent_name",
			"parent_name":      "parent_name",
			"parent_category":  "parent_name",
		}),
		requiredGroups: [][]string{{"name"}},
	},
	KindWarehouses: {
		aliases: mergeAliases(commonAliases(), map[string]string{
			"warehouse_code": "code",
			"location_code":  "code",
			"storage_code":   "code",
			"warehouse_name": "name",
			"location_name":  "name",
			"storage_name":   "name",
			"address":        "address",
			"is_default":     "is_default",
			"default":        "is_default",
			"is_active":      "is_active",
			"active":         "is_active",
		}),
		requiredGroups: [][]string{{"code"}, {"name"}},
	},
	KindProducts: {
		aliases: mergeAliases(commonAliases(), map[string]string{
			"product_code":           "code",
			"sku":                    "code",
			"item_code":              "code",
			"product_name":           "name",
			"item_name":              "name",
			"product_type":           "product_type",
			"type":                   "product_type",
			"category_id":            "category_id",
			"category":               "category_name",
			"category_name":          "category_name",
			"unit":                   "unit",
			"unit_of_measure":        "unit",
			"purchase_price":         "purchase_price",
			"cost_price":             "purchase_price",
			"cost":                   "purchase_price",
			"sales_price":            "sales_price",
			"sale_price":             "sales_price",
			"selling_price":          "sales_price",
			"price":                  "sales_price",
			"vat_rate":               "vat_rate",
			"vat":                    "vat_rate",
			"min_stock_level":        "min_stock_level",
			"minimum_stock":          "min_stock_level",
			"reorder_point":          "reorder_point",
			"sale_account_id":        "sale_account_id",
			"sales_account_id":       "sale_account_id",
			"sale_account_code":      "sale_account_code",
			"sales_account_code":     "sale_account_code",
			"purchase_account_id":    "purchase_account_id",
			"purchase_account_code":  "purchase_account_code",
			"inventory_account_id":   "inventory_account_id",
			"inventory_account_code": "inventory_account_code",
			"track_inventory":        "track_inventory",
			"track_stock":            "track_inventory",
			"is_active":              "is_active",
			"active":                 "is_active",
			"barcode":                "barcode",
			"supplier_id":            "supplier_id",
			"lead_time_days":         "lead_time_days",
		}),
		requiredGroups: [][]string{{"name"}, {"sales_price"}},
	},
	KindStockAdjustments: {
		aliases: mergeAliases(commonAliases(), map[string]string{
			"product":         "product_code",
			"product_code":    "product_code",
			"sku":             "product_code",
			"item_code":       "product_code",
			"product_id":      "product_id",
			"warehouse":       "warehouse_code",
			"warehouse_code":  "warehouse_code",
			"location_code":   "warehouse_code",
			"warehouse_id":    "warehouse_id",
			"quantity":        "quantity",
			"qty":             "quantity",
			"opening_qty":     "quantity",
			"opening_stock":   "quantity",
			"unit_cost":       "unit_cost",
			"cost":            "unit_cost",
			"lot":             "lot_number",
			"lot_number":      "lot_number",
			"batch":           "lot_number",
			"batch_number":    "lot_number",
			"serial":          "serial_number",
			"serial_number":   "serial_number",
			"expiry":          "expiry_date",
			"expiry_date":     "expiry_date",
			"expiration":      "expiry_date",
			"expiration_date": "expiry_date",
			"expires_at":      "expiry_date",
			"reason":          "reason",
			"notes":           "reason",
		}),
		requiredGroups: [][]string{{"product_id", "product_code"}, {"warehouse_id", "warehouse_code"}, {"quantity"}},
	},
	KindFixedAssets: {
		aliases: mergeAliases(commonAliases(), map[string]string{
			"asset_number":                          "asset_number",
			"asset_no":                              "asset_number",
			"asset_code":                            "asset_number",
			"code":                                  "asset_number",
			"number":                                "asset_number",
			"fixed_asset_number":                    "asset_number",
			"asset_name":                            "name",
			"category_id":                           "category_id",
			"category":                              "category_name",
			"category_name":                         "category_name",
			"purchase_date":                         "purchase_date",
			"acquisition_date":                      "purchase_date",
			"date":                                  "purchase_date",
			"purchase_cost":                         "purchase_cost",
			"acquisition_cost":                      "purchase_cost",
			"cost":                                  "purchase_cost",
			"price":                                 "purchase_cost",
			"supplier_id":                           "supplier_id",
			"invoice_id":                            "invoice_id",
			"serial_number":                         "serial_number",
			"serial_no":                             "serial_number",
			"depreciation_method":                   "depreciation_method",
			"useful_life_months":                    "useful_life_months",
			"life_months":                           "useful_life_months",
			"residual_value":                        "residual_value",
			"depreciation_start_date":               "depreciation_start_date",
			"accumulated_depreciation":              "accumulated_depreciation",
			"book_value":                            "book_value",
			"carrying_value":                        "book_value",
			"last_depreciation_date":                "last_depreciation_date",
			"disposal_date":                         "disposal_date",
			"disposal_method":                       "disposal_method",
			"disposal_proceeds":                     "disposal_proceeds",
			"disposal_notes":                        "disposal_notes",
			"asset_account_id":                      "asset_account_id",
			"asset_account_code":                    "asset_account_code",
			"depreciation_expense_account_id":       "depreciation_expense_account_id",
			"depreciation_expense_account_code":     "depreciation_expense_account_code",
			"accumulated_depreciation_account_id":   "accumulated_depreciation_account_id",
			"accumulated_depreciation_account_code": "accumulated_depreciation_account_code",
		}),
		requiredGroups: [][]string{{"name"}, {"purchase_date"}, {"purchase_cost"}},
	},
	KindOpeningBalances: {
		aliases: mergeAliases(commonAliases(), map[string]string{
			"account_code":     "account_code",
			"code":             "account_code",
			"account":          "account_code",
			"description":      "description",
			"line_description": "description",
			"debit_amount":     "debit",
			"credit_amount":    "credit",
		}),
		requiredGroups: [][]string{{"account_code"}, {"debit"}, {"credit"}},
	},
	KindJournalEntries: {
		aliases: mergeAliases(commonAliases(), map[string]string{
			"entry_reference":     "entry_reference",
			"reference":           "entry_reference",
			"document_number":     "entry_reference",
			"voucher_number":      "entry_reference",
			"journal_number":      "entry_reference",
			"entry_date":          "entry_date",
			"date":                "entry_date",
			"posting_date":        "entry_date",
			"account_code":        "account_code",
			"code":                "account_code",
			"account":             "account_code",
			"entry_description":   "entry_description",
			"journal_description": "entry_description",
			"entry_memo":          "entry_description",
			"line_description":    "line_description",
			"description":         "line_description",
			"memo":                "line_description",
			"debit_amount":        "debit",
			"credit_amount":       "credit",
			"exchange_rate":       "exchange_rate",
			"source_type":         "source_type",
			"source_id":           "source_id",
		}),
		requiredGroups: [][]string{{"entry_reference"}, {"entry_date"}, {"account_code"}, {"debit"}, {"credit"}},
	},
}

var duplicateIdentifierPreflightSpecs = map[FileKind][]duplicateIdentifierSpec{
	KindAccounts: {
		{field: "id"},
		{field: "code"},
	},
	KindContacts: {
		{field: "id"},
		{field: "code"},
		{field: "reg_code"},
		{field: "vat_number"},
		{field: "email"},
	},
	KindEmployees: {
		{field: "employee_number"},
		{field: "personal_code"},
		{field: "email"},
	},
	KindExpenses: {
		{field: "expense_number"},
	},
	KindEInvoices: {
		{field: "invoice_id"},
		{field: "invoice_number"},
	},
	KindPayments: {
		{field: "payment_number"},
	},
	KindBankAccounts: {
		{field: "account_number", normalize: bankAccountIndexKey},
	},
	KindBankTransactions: {
		{field: "external_id"},
	},
	KindCostCenters: {
		{field: "id"},
		{field: "code"},
	},
	KindProductCategories: {
		{field: "id"},
		{field: "name"},
	},
	KindWarehouses: {
		{field: "id"},
		{field: "code"},
	},
	KindProducts: {
		{field: "id"},
		{field: "code"},
	},
	KindFixedAssets: {
		{field: "id"},
		{field: "asset_number"},
	},
}

var groupedDocumentPreflightSpecs = map[FileKind]groupedDocumentSpec{
	KindInvoices: {
		keyLabel: "invoice_number/invoice_type",
		key: []groupedFieldSpec{
			{field: "invoice_number"},
			{field: "invoice_type", normalize: normalizeCutoverInvoiceType},
		},
		fields: []groupedFieldSpec{
			{field: "id", optional: true},
			{field: "issue_date", normalize: normalizeCutoverDate},
			{field: "due_date", optional: true, normalize: normalizeCutoverDate},
			{field: "currency", defaultValue: "EUR", normalize: normalizeCutoverUpper},
			{field: "exchange_rate", defaultValue: "1", normalize: normalizeCutoverDecimalComparable},
			{field: "contact_code", optional: true},
			{field: "contact_reg_code", optional: true},
			{field: "contact_vat_number", optional: true},
			{field: "contact_email", optional: true},
			{field: "contact_name", optional: true},
			{field: "reference", optional: true},
			{field: "notes", optional: true},
			{field: "status", optional: true, normalize: normalizeCutoverInvoiceStatus},
			{field: "amount_paid", optional: true, normalize: normalizeCutoverDecimalComparable},
		},
	},
	KindQuotes: {
		keyLabel: "quote_number",
		key:      []groupedFieldSpec{{field: "quote_number"}},
		fields: []groupedFieldSpec{
			{field: "id", optional: true},
			{field: "quote_date", normalize: normalizeCutoverDate},
			{field: "valid_until", optional: true, normalize: normalizeCutoverDate},
			{field: "currency", defaultValue: "EUR", normalize: normalizeCutoverUpper},
			{field: "exchange_rate", defaultValue: "1", normalize: normalizeCutoverDecimalComparable},
			{field: "contact_id", optional: true},
			{field: "contact_code", optional: true},
			{field: "contact_reg_code", optional: true},
			{field: "contact_vat_number", optional: true},
			{field: "contact_email", optional: true},
			{field: "contact_name", optional: true},
			{field: "notes", optional: true},
			{field: "status", optional: true, normalize: normalizeCutoverQuoteStatus},
		},
	},
	KindOrders: {
		keyLabel: "order_number",
		key:      []groupedFieldSpec{{field: "order_number"}},
		fields: []groupedFieldSpec{
			{field: "order_date", normalize: normalizeCutoverDate},
			{field: "expected_delivery", optional: true, normalize: normalizeCutoverDate},
			{field: "currency", defaultValue: "EUR", normalize: normalizeCutoverUpper},
			{field: "exchange_rate", defaultValue: "1", normalize: normalizeCutoverDecimalComparable},
			{field: "contact_id", optional: true},
			{field: "contact_code", optional: true},
			{field: "contact_reg_code", optional: true},
			{field: "contact_vat_number", optional: true},
			{field: "contact_email", optional: true},
			{field: "contact_name", optional: true},
			{field: "notes", optional: true},
			{field: "quote_id", optional: true},
			{field: "status", optional: true, normalize: normalizeCutoverOrderStatus},
		},
	},
	KindRecurringInvoices: {
		keyLabel: "template",
		key:      []groupedFieldSpec{{field: "name"}},
		fields: []groupedFieldSpec{
			{field: "contact_id", optional: true},
			{field: "contact_code", optional: true},
			{field: "contact_reg_code", optional: true},
			{field: "contact_vat_number", optional: true},
			{field: "contact_email", optional: true},
			{field: "contact_name", optional: true},
			{field: "invoice_type", defaultValue: "SALES", normalize: normalizeCutoverUpper},
			{field: "currency", defaultValue: "EUR", normalize: normalizeCutoverUpper},
			{field: "frequency", normalize: normalizeCutoverUpper},
			{field: "start_date", normalize: normalizeCutoverDate},
			{field: "end_date", optional: true, normalize: normalizeCutoverDate},
			{field: "next_generation_date", defaultFrom: "start_date", normalize: normalizeCutoverDate},
			{field: "payment_terms_days", defaultValue: "14", normalize: normalizeCutoverIntComparable},
			{field: "reference", optional: true},
			{field: "notes", optional: true},
			{field: "is_active", defaultValue: "true", normalize: normalizeCutoverBoolComparable},
			{field: "last_generated_at", optional: true, normalize: normalizeCutoverDate},
			{field: "generated_count", defaultValue: "0", normalize: normalizeCutoverIntComparable},
			{field: "send_email_on_generation", defaultValue: "false", normalize: normalizeCutoverBoolComparable},
			{field: "email_template_type", defaultValue: "INVOICE_SEND", normalize: normalizeCutoverUpper},
			{field: "recipient_email_override", optional: true},
			{field: "attach_pdf_to_email", defaultValue: "true", normalize: normalizeCutoverBoolComparable},
			{field: "email_subject_override", optional: true},
			{field: "email_message", optional: true},
		},
	},
}

var cutoverInvoiceTypeAliases = map[string]string{
	"sales":            "SALES",
	"sale":             "SALES",
	"salesinvoice":     "SALES",
	"sales_invoice":    "SALES",
	"sales invoice":    "SALES",
	"myygiarve":        "SALES",
	"purchase":         "PURCHASE",
	"purchaseinvoice":  "PURCHASE",
	"purchase_invoice": "PURCHASE",
	"purchase invoice": "PURCHASE",
	"bill":             "PURCHASE",
	"ostuarve":         "PURCHASE",
	"credit_note":      "CREDIT_NOTE",
	"creditnote":       "CREDIT_NOTE",
	"credit note":      "CREDIT_NOTE",
	"kreeditarve":      "CREDIT_NOTE",
}

var cutoverInvoiceStatusAliases = map[string]string{
	"draft":            "DRAFT",
	"mustand":          "DRAFT",
	"sent":             "SENT",
	"issued":           "SENT",
	"open":             "SENT",
	"saadetud":         "SENT",
	"partially_paid":   "PARTIALLY_PAID",
	"partial":          "PARTIALLY_PAID",
	"osaline":          "PARTIALLY_PAID",
	"paid":             "PAID",
	"makstud":          "PAID",
	"overdue":          "OVERDUE",
	"tahtaja_uletanud": "OVERDUE",
	"voided":           "VOIDED",
	"void":             "VOIDED",
	"tuhistatud":       "VOIDED",
}

var cutoverQuoteStatusAliases = map[string]string{
	"draft":       "DRAFT",
	"mustand":     "DRAFT",
	"sent":        "SENT",
	"issued":      "SENT",
	"saadetud":    "SENT",
	"accepted":    "ACCEPTED",
	"approved":    "ACCEPTED",
	"rejected":    "REJECTED",
	"declined":    "REJECTED",
	"expired":     "EXPIRED",
	"converted":   "CONVERTED",
	"convertedto": "CONVERTED",
}

var cutoverOrderStatusAliases = map[string]string{
	"pending":    "PENDING",
	"open":       "PENDING",
	"confirmed":  "CONFIRMED",
	"processing": "PROCESSING",
	"shipped":    "SHIPPED",
	"delivered":  "DELIVERED",
	"canceled":   "CANCELED",
}

func ValidateBundle(req *ValidateBundleRequest) (*BundleValidationReport, error) {
	if req == nil || len(req.Files) == 0 {
		return nil, fmt.Errorf("at least one migration file is required")
	}
	eInvoiceContactMode, err := normalizeEInvoiceContactMode(req.EInvoiceContactMode)
	if err != nil {
		return nil, err
	}
	providerPreset, err := normalizeMigrationProviderPreset(req.ProviderPreset)
	if err != nil {
		return nil, err
	}

	report := &BundleValidationReport{}
	parsed := make([]parsedFile, 0, len(req.Files))
	for _, file := range req.Files {
		if !isSupportedBundleKind(file.Kind) {
			report.addIssue(ValidationIssue{
				Severity: SeverityError,
				Kind:     file.Kind,
				FileName: displayFileName(file),
				Message:  fmt.Sprintf("unsupported migration file kind %q", file.Kind),
			})
			continue
		}

		parsedFile, validation, err := parseBundleFileByKind(file, providerPreset)
		report.Files = append(report.Files, validation)
		if err != nil {
			report.addIssue(ValidationIssue{
				Severity: SeverityError,
				Kind:     file.Kind,
				FileName: validation.FileName,
				Message:  err.Error(),
			})
			continue
		}

		for _, missing := range validation.MissingColumns {
			report.addIssue(ValidationIssue{
				Severity: SeverityError,
				Kind:     file.Kind,
				FileName: validation.FileName,
				Message:  "missing required column group: " + missing,
			})
		}
		report.Summary.RowsValidated += len(parsedFile.rows)
		parsed = append(parsed, parsedFile)
	}

	report.Summary.FilesValidated = len(report.Files)
	indexes := buildIndexes(parsed)
	for _, file := range parsed {
		validateReferences(report, indexes, file, eInvoiceContactMode)
		validateDuplicateIdentifierPreflight(report, file)
		validateGroupedDocumentPreflight(report, file)
		validateAccountingPreflight(report, file)
	}

	sort.SliceStable(report.Issues, func(i, j int) bool {
		if report.Issues[i].Severity != report.Issues[j].Severity {
			return report.Issues[i].Severity < report.Issues[j].Severity
		}
		if report.Issues[i].FileName != report.Issues[j].FileName {
			return report.Issues[i].FileName < report.Issues[j].FileName
		}
		return report.Issues[i].Row < report.Issues[j].Row
	})

	report.Summary.Ready = report.Summary.ErrorCount == 0
	return report, nil
}

func normalizeEInvoiceContactMode(mode EInvoiceContactMode) (EInvoiceContactMode, error) {
	switch normalized := EInvoiceContactMode(strings.ToLower(strings.TrimSpace(string(mode)))); normalized {
	case "":
		return EInvoiceContactModeSupplier, nil
	case EInvoiceContactModeSupplier, EInvoiceContactModeCustomer, EInvoiceContactModeBoth:
		return normalized, nil
	default:
		return "", fmt.Errorf("unsupported e_invoice_contact_mode %q (expected supplier, customer, or both)", mode)
	}
}

func parseBundleFileByKind(file BundleFile, providerPreset MigrationProviderPreset) (parsedFile, FileValidation, error) {
	if file.Kind == KindEInvoices {
		return parseEInvoiceBundleFile(file)
	}

	return parseBundleFile(file, fileSpecForProviderPreset(file.Kind, providerPreset))
}

func isSupportedBundleKind(kind FileKind) bool {
	if kind == KindEInvoices {
		return true
	}
	_, ok := fileSpecs[kind]
	return ok
}

func parseBundleFile(file BundleFile, spec fileSpec) (parsedFile, FileValidation, error) {
	fileName := displayFileName(file)
	validation := FileValidation{
		Kind:     file.Kind,
		FileName: fileName,
	}

	trimmed := strings.TrimPrefix(strings.TrimSpace(file.CSVContent), "\ufeff")
	if trimmed == "" {
		return parsedFile{}, validation, fmt.Errorf("csv_content is required")
	}

	reader := csv.NewReader(strings.NewReader(trimmed))
	reader.Comma = detectDelimiter(trimmed)
	reader.FieldsPerRecord = -1
	reader.TrimLeadingSpace = true

	headers, err := reader.Read()
	if err != nil {
		if err == io.EOF {
			return parsedFile{}, validation, fmt.Errorf("csv file is empty")
		}
		return parsedFile{}, validation, fmt.Errorf("parse csv header: %w", err)
	}

	canonicalHeaders := make([]string, len(headers))
	headerSet := map[string]bool{}
	for i, header := range headers {
		canonical := canonicalHeader(spec.aliases, header)
		canonicalHeaders[i] = canonical
		if canonical != "" {
			headerSet[canonical] = true
			validation.Headers = append(validation.Headers, canonical)
		}
	}
	validation.MissingColumns = missingRequiredGroups(spec.requiredGroups, headerSet)

	rows := []parsedRow{}
	rowNumber := 1
	for {
		record, err := reader.Read()
		if err != nil {
			if err == io.EOF {
				break
			}
			return parsedFile{}, validation, fmt.Errorf("parse csv row %d: %w", rowNumber+1, err)
		}
		rowNumber++

		values := make(map[string]string, len(canonicalHeaders))
		blank := true
		for i, header := range canonicalHeaders {
			if header == "" {
				continue
			}
			value := ""
			if i < len(record) {
				value = strings.TrimSpace(record[i])
			}
			if value != "" {
				blank = false
			}
			values[header] = value
		}
		if blank {
			continue
		}
		rows = append(rows, parsedRow{number: rowNumber, values: values})
	}

	validation.Rows = len(rows)
	return parsedFile{kind: file.Kind, fileName: fileName, headers: validation.Headers, rows: rows}, validation, nil
}

func parseEInvoiceBundleFile(file BundleFile) (parsedFile, FileValidation, error) {
	fileName := displayFileName(file)
	validation := FileValidation{
		Kind:     file.Kind,
		FileName: fileName,
		Headers:  eInvoiceValidationHeaders(),
	}

	invoices, err := einvoice.Parse(file.XMLContent)
	if err != nil {
		return parsedFile{}, validation, err
	}

	rows := make([]parsedRow, 0, len(invoices))
	for index, invoice := range invoices {
		rows = append(rows, parsedRow{
			number: index + 1,
			values: map[string]string{
				"invoice_id":          invoice.ID,
				"invoice_number":      invoice.Number,
				"contact_reg_code":    invoice.Seller.RegNumber,
				"contact_vat_number":  invoice.Seller.VATRegNumber,
				"contact_email":       invoice.Seller.Email,
				"contact_name":        invoice.Seller.Name,
				"buyer_reg_code":      invoice.Buyer.RegNumber,
				"buyer_vat_number":    invoice.Buyer.VATRegNumber,
				"buyer_contact_email": invoice.Buyer.Email,
				"buyer_contact_name":  invoice.Buyer.Name,
			},
		})
	}

	validation.Rows = len(rows)
	return parsedFile{kind: file.Kind, fileName: fileName, headers: validation.Headers, rows: rows}, validation, nil
}

func eInvoiceValidationHeaders() []string {
	return []string{
		"invoice_id",
		"invoice_number",
		"contact_reg_code",
		"contact_vat_number",
		"contact_email",
		"contact_name",
		"buyer_reg_code",
		"buyer_vat_number",
		"buyer_contact_email",
		"buyer_contact_name",
	}
}

func buildIndexes(files []parsedFile) bundleIndexes {
	indexes := bundleIndexes{
		files:             map[FileKind]bool{},
		accounts:          map[string]bool{},
		bankAccounts:      map[string]string{},
		contacts:          map[string]bool{},
		employees:         map[string]bool{},
		invoices:          map[string]bool{},
		quotes:            map[string]bool{},
		costCenters:       map[string]bool{},
		productCategories: map[string]bool{},
		products:          map[string]bool{},
		warehouses:        map[string]bool{},
	}
	for _, file := range files {
		indexes.files[file.kind] = true
		for _, row := range file.rows {
			switch file.kind {
			case KindAccounts:
				addIndexValue(indexes.accounts, row.values["code"])
				addIndexValue(indexes.accounts, row.values["id"])
			case KindBankAccounts:
				addBankAccountIndexValue(indexes.bankAccounts, row.values["account_number"], row.values["currency"])
			case KindContacts:
				addIndexValue(indexes.contacts, row.values["id"])
				addIndexValue(indexes.contacts, row.values["code"])
				addIndexValue(indexes.contacts, row.values["reg_code"])
				addIndexValue(indexes.contacts, row.values["vat_number"])
				addIndexValue(indexes.contacts, row.values["email"])
				addIndexValue(indexes.contacts, row.values["name"])
			case KindEmployees:
				addEmployeeIndexValues(indexes.employees, row.values)
			case KindInvoices, KindEInvoices:
				addIndexValue(indexes.invoices, row.values["invoice_number"])
				addIndexValue(indexes.invoices, row.values["invoice_id"])
				addIndexValue(indexes.invoices, row.values["id"])
			case KindQuotes:
				addIndexValue(indexes.quotes, row.values["id"])
			case KindCostCenters:
				addIndexValue(indexes.costCenters, row.values["code"])
				addIndexValue(indexes.costCenters, row.values["id"])
			case KindProductCategories:
				addIndexValue(indexes.productCategories, row.values["name"])
				addIndexValue(indexes.productCategories, row.values["id"])
			case KindProducts:
				addIndexValue(indexes.products, row.values["code"])
				addIndexValue(indexes.products, row.values["id"])
				addIndexValue(indexes.products, row.values["name"])
			case KindWarehouses:
				addIndexValue(indexes.warehouses, row.values["code"])
				addIndexValue(indexes.warehouses, row.values["id"])
				addIndexValue(indexes.warehouses, row.values["name"])
			}
		}
	}
	return indexes
}

func validateReferences(report *BundleValidationReport, indexes bundleIndexes, file parsedFile, eInvoiceContactMode EInvoiceContactMode) {
	for _, row := range file.rows {
		switch file.kind {
		case KindAccounts:
			checkSelfReference(report, file, row, "parent_code", "code")
			checkTargetReference(report, indexes.files[KindAccounts], indexes.accounts, file, row, KindAccounts,
				[]string{"parent_code"})
		case KindContacts:
			checkOptionalUUID(report, file, row, "id")
		case KindExpenses:
			checkTargetReference(report, indexes.files[KindAccounts], indexes.accounts, file, row, KindAccounts,
				[]string{"expense_account_code"})
			checkTargetReference(report, indexes.files[KindAccounts], indexes.accounts, file, row, KindAccounts,
				[]string{"payment_account_code"})
			checkTargetReference(report, indexes.files[KindContacts], indexes.contacts, file, row, KindContacts,
				[]string{"contact_id"})
		case KindInvoices:
			checkOptionalUUID(report, file, row, "id")
			checkTargetReference(report, indexes.files[KindContacts], indexes.contacts, file, row, KindContacts,
				[]string{"contact_code", "contact_reg_code", "contact_vat_number", "contact_email", "contact_name"})
			checkTargetReference(report, indexes.files[KindProducts], indexes.products, file, row, KindProducts,
				[]string{"product_id", "product_code"})
		case KindEInvoices:
			checkEInvoiceContactReferences(report, indexes, file, row, eInvoiceContactMode)
		case KindQuotes, KindRecurringInvoices:
			checkTargetReference(report, indexes.files[KindContacts], indexes.contacts, file, row, KindContacts,
				commercialDocumentContactReferenceFields())
			checkTargetReference(report, indexes.files[KindProducts], indexes.products, file, row, KindProducts,
				[]string{"product_id", "product_code"})
		case KindOrders:
			checkTargetReference(report, indexes.files[KindContacts], indexes.contacts, file, row, KindContacts,
				commercialDocumentContactReferenceFields())
			checkTargetReference(report, indexes.files[KindProducts], indexes.products, file, row, KindProducts,
				[]string{"product_id", "product_code"})
			checkTargetReference(report, indexes.files[KindQuotes], indexes.quotes, file, row, KindQuotes,
				[]string{"quote_id"})
		case KindPayments:
			checkTargetReference(report, indexes.files[KindContacts], indexes.contacts, file, row, KindContacts,
				[]string{"contact_id"})
			checkTargetReference(report, indexes.files[KindInvoices] || indexes.files[KindEInvoices], indexes.invoices, file, row, KindInvoices,
				[]string{"invoice_id", "invoice_number"})
		case KindBankAccounts:
			checkTargetReference(report, indexes.files[KindAccounts], indexes.accounts, file, row, KindAccounts,
				[]string{"gl_account_id", "gl_account_code"})
		case KindBankTransactions:
			checkBankTransactionSourceAccount(report, indexes, file, row)
		case KindPayrollHistory, KindLeaveBalances, KindTSDHistory:
			checkEmployeeReference(report, indexes, file, row)
		case KindOpeningBalances, KindJournalEntries:
			checkTargetReference(report, indexes.files[KindAccounts], indexes.accounts, file, row, KindAccounts,
				[]string{"account_code"})
		case KindCostCenters:
			checkSelfReference(report, file, row, "parent_code", "code")
			checkTargetReference(report, indexes.files[KindCostCenters], indexes.costCenters, file, row, KindCostCenters,
				[]string{"parent_code"})
		case KindCostAllocations:
			checkTargetReference(report, indexes.files[KindCostCenters], indexes.costCenters, file, row, KindCostCenters,
				[]string{"cost_center_id", "cost_center_code"})
		case KindProductCategories:
			checkSelfReference(report, file, row, "parent_name", "name")
			checkTargetReference(report, indexes.files[KindProductCategories], indexes.productCategories, file, row, KindProductCategories,
				[]string{"parent_name"})
		case KindProducts:
			checkTargetReference(report, indexes.files[KindProductCategories], indexes.productCategories, file, row, KindProductCategories,
				[]string{"category_id", "category_name"})
			checkTargetReference(report, indexes.files[KindAccounts], indexes.accounts, file, row, KindAccounts,
				[]string{"sale_account_id", "sale_account_code"})
			checkTargetReference(report, indexes.files[KindAccounts], indexes.accounts, file, row, KindAccounts,
				[]string{"purchase_account_id", "purchase_account_code"})
			checkTargetReference(report, indexes.files[KindAccounts], indexes.accounts, file, row, KindAccounts,
				[]string{"inventory_account_id", "inventory_account_code"})
			checkTargetReference(report, indexes.files[KindContacts], indexes.contacts, file, row, KindContacts,
				[]string{"supplier_id"})
		case KindStockAdjustments:
			checkTargetReference(report, indexes.files[KindProducts], indexes.products, file, row, KindProducts,
				[]string{"product_id", "product_code"})
			checkTargetReference(report, indexes.files[KindWarehouses], indexes.warehouses, file, row, KindWarehouses,
				[]string{"warehouse_id", "warehouse_code"})
		case KindFixedAssets:
			checkTargetReference(report, indexes.files[KindAccounts], indexes.accounts, file, row, KindAccounts,
				[]string{"asset_account_id", "asset_account_code"})
			checkTargetReference(report, indexes.files[KindAccounts], indexes.accounts, file, row, KindAccounts,
				[]string{"depreciation_expense_account_id", "depreciation_expense_account_code"})
			checkTargetReference(report, indexes.files[KindAccounts], indexes.accounts, file, row, KindAccounts,
				[]string{"accumulated_depreciation_account_id", "accumulated_depreciation_account_code"})
			checkTargetReference(report, indexes.files[KindContacts], indexes.contacts, file, row, KindContacts,
				[]string{"supplier_id"})
			checkTargetReference(report, indexes.files[KindInvoices] || indexes.files[KindEInvoices], indexes.invoices, file, row, KindInvoices,
				[]string{"invoice_id"})
		}
	}
}

type cutoverAmountIssue struct {
	field   string
	value   string
	message string
}

type journalValidationGroup struct {
	firstRow  int
	reference string
	rows      []parsedRow
}

func validateDuplicateIdentifierPreflight(report *BundleValidationReport, file parsedFile) {
	specs := duplicateIdentifierPreflightSpecs[file.kind]
	if len(specs) == 0 {
		return
	}

	for _, spec := range specs {
		seen := map[string]duplicateIdentifierValue{}
		for _, row := range file.rows {
			value := strings.TrimSpace(row.values[spec.field])
			if value == "" {
				continue
			}
			key := normalizedDuplicateIdentifierValue(spec, value)
			if key == "" {
				continue
			}
			first, ok := seen[key]
			if !ok {
				seen[key] = duplicateIdentifierValue{row: row.number}
				continue
			}
			report.addIssue(ValidationIssue{
				Severity: SeverityError,
				Kind:     file.kind,
				FileName: file.fileName,
				Row:      row.number,
				Field:    spec.field,
				Value:    value,
				Message:  fmt.Sprintf("%s %q duplicates row %d in %s file", spec.field, value, first.row, file.kind),
			})
		}
	}
}

func normalizedDuplicateIdentifierValue(spec duplicateIdentifierSpec, value string) string {
	if spec.normalize != nil {
		return spec.normalize(value)
	}
	return normalizedValue(value)
}

func validateGroupedDocumentPreflight(report *BundleValidationReport, file parsedFile) {
	spec, ok := groupedDocumentPreflightSpecs[file.kind]
	if !ok {
		return
	}

	groups := map[string]map[string]groupedSeenValue{}
	groupDisplays := map[string]string{}
	for _, row := range file.rows {
		key, displayKey, ok := groupedDocumentKey(row, spec)
		if !ok {
			continue
		}
		fieldValues, ok := groups[key]
		if !ok {
			fieldValues = map[string]groupedSeenValue{}
			groups[key] = fieldValues
			groupDisplays[key] = displayKey
		}
		for _, field := range spec.fields {
			value, ok := groupedComparableFieldValue(row, field)
			if !ok {
				continue
			}
			seen, exists := fieldValues[field.field]
			if !exists {
				fieldValues[field.field] = groupedSeenValue{
					normalized: value.normalized,
				}
				continue
			}
			if seen.normalized == value.normalized {
				continue
			}
			report.addIssue(ValidationIssue{
				Severity: SeverityError,
				Kind:     file.kind,
				FileName: file.fileName,
				Row:      row.number,
				Field:    field.field,
				Value:    value.display,
				Message:  fmt.Sprintf("%s must be consistent for each %s %q", field.field, spec.keyLabel, groupDisplays[key]),
			})
		}
	}
}

func groupedDocumentKey(row parsedRow, spec groupedDocumentSpec) (string, string, bool) {
	keyParts := make([]string, 0, len(spec.key))
	displayParts := make([]string, 0, len(spec.key))
	for _, field := range spec.key {
		value, ok := groupedComparableFieldValue(row, field)
		if !ok || value.normalized == "" {
			return "", "", false
		}
		keyParts = append(keyParts, normalizedValue(value.normalized))
		displayParts = append(displayParts, value.display)
	}
	return strings.Join(keyParts, "\x00"), strings.Join(displayParts, "/"), true
}

func groupedComparableFieldValue(row parsedRow, field groupedFieldSpec) (groupedComparableValue, bool) {
	display := strings.TrimSpace(row.values[field.field])
	value := display
	if value == "" && field.defaultFrom != "" {
		value = strings.TrimSpace(row.values[field.defaultFrom])
		display = value
	}
	if value == "" && field.defaultValue != "" {
		value = field.defaultValue
		display = field.defaultValue
	}
	if value == "" && field.optional {
		return groupedComparableValue{}, false
	}

	normalize := field.normalize
	if normalize == nil {
		normalize = strings.TrimSpace
	}
	return groupedComparableValue{
		normalized: normalize(value),
		display:    display,
	}, true
}

func normalizeCutoverInvoiceType(value string) string {
	normalized := normalizedValue(value)
	if canonical, ok := cutoverInvoiceTypeAliases[normalized]; ok {
		return canonical
	}
	return normalizeCutoverUpper(value)
}

func normalizeCutoverInvoiceStatus(value string) string {
	normalized := normalizedValue(value)
	if canonical, ok := cutoverInvoiceStatusAliases[normalized]; ok {
		return canonical
	}
	return normalizeCutoverUpper(value)
}

func normalizeCutoverQuoteStatus(value string) string {
	normalized := normalizedValue(value)
	if canonical, ok := cutoverQuoteStatusAliases[normalized]; ok {
		return canonical
	}
	return normalizeCutoverUpper(value)
}

func normalizeCutoverOrderStatus(value string) string {
	normalized := normalizedValue(value)
	if canonical, ok := cutoverOrderStatusAliases[normalized]; ok {
		return canonical
	}
	return normalizeCutoverUpper(value)
}

func normalizeCutoverUpper(value string) string {
	return strings.ToUpper(strings.TrimSpace(value))
}

func normalizeCutoverDate(value string) string {
	trimmed := strings.TrimSpace(value)
	parsed, err := time.Parse("2006-01-02", trimmed)
	if err != nil {
		return trimmed
	}
	return parsed.Format("2006-01-02")
}

func normalizeCutoverDecimalComparable(value string) string {
	trimmed := strings.TrimSpace(value)
	parsed, err := parseCutoverDecimal(trimmed, "amount")
	if err != nil {
		return trimmed
	}
	return parsed.String()
}

func normalizeCutoverIntComparable(value string) string {
	trimmed := strings.TrimSpace(value)
	parsed, err := strconv.Atoi(trimmed)
	if err != nil {
		return trimmed
	}
	return strconv.Itoa(parsed)
}

func normalizeCutoverBoolComparable(value string) string {
	switch normalizedValue(value) {
	case "true", "t", "yes", "y", "1":
		return "true"
	case "false", "f", "no", "n", "0":
		return "false"
	default:
		return strings.TrimSpace(value)
	}
}

func validateAccountingPreflight(report *BundleValidationReport, file parsedFile) {
	switch file.kind {
	case KindExpenses:
		checkExpenseRows(report, file)
	case KindPayments:
		checkPaymentRows(report, file)
	case KindBankTransactions:
		checkBankTransactionRows(report, file)
	case KindOpeningBalances:
		checkOpeningBalanceTotals(report, file)
	case KindJournalEntries:
		checkJournalEntryGroups(report, file)
	}
}

func checkExpenseRows(report *BundleValidationReport, file parsedFile) {
	hasExpenseDate := fileHasHeaders(file, "expense_date")
	hasMerchant := fileHasHeaders(file, "merchant")
	hasAmount := fileHasHeaders(file, "amount")
	for _, row := range file.rows {
		if hasExpenseDate {
			checkExpenseDate(report, file, row)
		}
		if hasMerchant {
			checkRequiredCutoverField(report, file, row, "merchant")
		}
		checkRequiredCutoverFieldGroup(report, file, row, "expense_account_id", "expense_account_code")
		checkRequiredCutoverFieldGroup(report, file, row, "payment_account_id", "payment_account_code")
		if hasAmount {
			checkExpenseAmount(report, file, row)
		}
		checkExpenseExchangeRate(report, file, row)
		checkExpenseRequiresReceipt(report, file, row)
		status, statusOK := checkExpenseStatus(report, file, row)
		if statusOK {
			checkExpenseStatusMetadata(report, file, row, status)
		}
	}
}

func checkExpenseDate(report *BundleValidationReport, file parsedFile, row parsedRow) {
	value := strings.TrimSpace(row.values["expense_date"])
	if value == "" {
		report.addIssue(ValidationIssue{
			Severity: SeverityError,
			Kind:     file.kind,
			FileName: file.fileName,
			Row:      row.number,
			Field:    "expense_date",
			Message:  "expense_date is required",
		})
		return
	}
	if isCutoverDateOrRFC3339(value) {
		return
	}
	report.addIssue(ValidationIssue{
		Severity: SeverityError,
		Kind:     file.kind,
		FileName: file.fileName,
		Row:      row.number,
		Field:    "expense_date",
		Value:    value,
		Message:  "expense_date must be YYYY-MM-DD or RFC3339",
	})
}

func checkExpenseAmount(report *BundleValidationReport, file parsedFile, row parsedRow) {
	if _, issue := parseCutoverPositiveNormalizedDecimal(row.values["amount"], "amount"); issue != nil {
		report.addIssue(cutoverAmountValidationIssue(file, row, *issue))
	}
}

func checkExpenseExchangeRate(report *BundleValidationReport, file parsedFile, row parsedRow) {
	if strings.TrimSpace(row.values["exchange_rate"]) == "" {
		return
	}
	if _, issue := parseCutoverPositiveNormalizedDecimal(row.values["exchange_rate"], "exchange_rate"); issue != nil {
		report.addIssue(cutoverAmountValidationIssue(file, row, *issue))
	}
}

func checkExpenseRequiresReceipt(report *BundleValidationReport, file parsedFile, row parsedRow) {
	value := strings.TrimSpace(row.values["requires_receipt"])
	if value == "" {
		return
	}
	switch normalizeCutoverBoolComparable(value) {
	case "true", "false":
		return
	default:
		report.addIssue(ValidationIssue{
			Severity: SeverityError,
			Kind:     file.kind,
			FileName: file.fileName,
			Row:      row.number,
			Field:    "requires_receipt",
			Value:    value,
			Message:  "invalid requires_receipt",
		})
	}
}

func checkExpenseStatus(report *BundleValidationReport, file parsedFile, row parsedRow) (string, bool) {
	value := strings.TrimSpace(row.values["status"])
	switch normalizedCutoverStatus(value) {
	case "", "draft":
		return "draft", true
	case "submitted":
		return "submitted", true
	case "approved":
		return "approved", true
	case "rejected":
		return "rejected", true
	case "posted":
		report.addIssue(ValidationIssue{
			Severity: SeverityError,
			Kind:     file.kind,
			FileName: file.fileName,
			Row:      row.number,
			Field:    "status",
			Value:    value,
			Message:  "posted expenses must be imported as approved and posted through the expense workflow",
		})
		return "", false
	default:
		report.addIssue(ValidationIssue{
			Severity: SeverityError,
			Kind:     file.kind,
			FileName: file.fileName,
			Row:      row.number,
			Field:    "status",
			Value:    value,
			Message:  fmt.Sprintf("invalid status %q", value),
		})
		return "", false
	}
}

func checkExpenseStatusMetadata(report *BundleValidationReport, file parsedFile, row parsedRow, status string) {
	switch status {
	case "submitted":
		checkExpenseOptionalTimestamp(report, file, row, "submitted_at")
	case "approved":
		checkExpenseOptionalTimestamp(report, file, row, "submitted_at")
		checkExpenseOptionalTimestamp(report, file, row, "approved_at")
	case "rejected":
		checkRequiredCutoverField(report, file, row, "rejection_reason")
		checkExpenseOptionalTimestamp(report, file, row, "submitted_at")
		checkExpenseOptionalTimestamp(report, file, row, "rejected_at")
	}
}

func checkExpenseOptionalTimestamp(report *BundleValidationReport, file parsedFile, row parsedRow, field string) {
	value := strings.TrimSpace(row.values[field])
	if value == "" {
		return
	}
	if isCutoverDateOrRFC3339(value) {
		return
	}
	report.addIssue(ValidationIssue{
		Severity: SeverityError,
		Kind:     file.kind,
		FileName: file.fileName,
		Row:      row.number,
		Field:    field,
		Value:    value,
		Message:  fmt.Sprintf("%s must be YYYY-MM-DD or RFC3339", field),
	})
}

func checkRequiredCutoverField(report *BundleValidationReport, file parsedFile, row parsedRow, field string) {
	if strings.TrimSpace(row.values[field]) != "" {
		return
	}
	report.addIssue(ValidationIssue{
		Severity: SeverityError,
		Kind:     file.kind,
		FileName: file.fileName,
		Row:      row.number,
		Field:    field,
		Message:  fmt.Sprintf("%s is required", field),
	})
}

func checkRequiredCutoverFieldGroup(report *BundleValidationReport, file parsedFile, row parsedRow, fields ...string) {
	hasHeader := false
	for _, field := range fields {
		if fileHasHeaders(file, field) {
			hasHeader = true
		}
		if strings.TrimSpace(row.values[field]) != "" {
			return
		}
	}
	if !hasHeader {
		return
	}
	report.addIssue(ValidationIssue{
		Severity: SeverityError,
		Kind:     file.kind,
		FileName: file.fileName,
		Row:      row.number,
		Field:    strings.Join(fields, "/"),
		Message:  strings.Join(fields, " or ") + " is required",
	})
}

func checkPaymentRows(report *BundleValidationReport, file parsedFile) {
	hasPaymentType := fileHasHeaders(file, "payment_type")
	hasPaymentDate := fileHasHeaders(file, "payment_date")
	hasAmount := fileHasHeaders(file, "amount")
	for _, row := range file.rows {
		if hasPaymentType {
			checkPaymentType(report, file, row)
		}
		if hasPaymentDate {
			checkPaymentDate(report, file, row)
		}
		amount, amountOK := decimal.Zero, false
		if hasAmount {
			parsedAmount, amountIssue := parseCutoverPositiveDecimal(row.values["amount"], "amount")
			if amountIssue != nil {
				report.addIssue(cutoverAmountValidationIssue(file, row, *amountIssue))
			} else {
				amount = parsedAmount
				amountOK = true
			}
		}
		checkPaymentExchangeRate(report, file, row)
		checkPaymentAllocation(report, file, row, amount, amountOK)
	}
}

func checkPaymentType(report *BundleValidationReport, file parsedFile, row parsedRow) {
	value := strings.TrimSpace(row.values["payment_type"])
	switch strings.ToUpper(value) {
	case "RECEIVED", "MADE":
		return
	default:
		report.addIssue(ValidationIssue{
			Severity: SeverityError,
			Kind:     file.kind,
			FileName: file.fileName,
			Row:      row.number,
			Field:    "payment_type",
			Value:    value,
			Message:  fmt.Sprintf("invalid payment_type %q", value),
		})
	}
}

func checkPaymentDate(report *BundleValidationReport, file parsedFile, row parsedRow) {
	value := strings.TrimSpace(row.values["payment_date"])
	if value == "" {
		report.addIssue(ValidationIssue{
			Severity: SeverityError,
			Kind:     file.kind,
			FileName: file.fileName,
			Row:      row.number,
			Field:    "payment_date",
			Message:  "payment_date is required",
		})
		return
	}
	if _, err := time.Parse("2006-01-02", value); err == nil {
		return
	}
	if _, err := time.Parse(time.RFC3339, value); err == nil {
		return
	}
	report.addIssue(ValidationIssue{
		Severity: SeverityError,
		Kind:     file.kind,
		FileName: file.fileName,
		Row:      row.number,
		Field:    "payment_date",
		Value:    value,
		Message:  "payment_date must be YYYY-MM-DD or RFC3339",
	})
}

func checkPaymentExchangeRate(report *BundleValidationReport, file parsedFile, row parsedRow) {
	if !fileHasHeaders(file, "exchange_rate") || strings.TrimSpace(row.values["exchange_rate"]) == "" {
		return
	}
	if _, issue := parseCutoverPositiveDecimal(row.values["exchange_rate"], "exchange_rate"); issue != nil {
		report.addIssue(cutoverAmountValidationIssue(file, row, *issue))
	}
}

func checkPaymentAllocation(report *BundleValidationReport, file parsedFile, row parsedRow, paymentAmount decimal.Decimal, amountOK bool) {
	allocationValue := strings.TrimSpace(row.values["allocation_amount"])
	if allocationValue == "" {
		return
	}
	if strings.TrimSpace(row.values["invoice_id"]) == "" && strings.TrimSpace(row.values["invoice_number"]) == "" {
		report.addIssue(ValidationIssue{
			Severity: SeverityError,
			Kind:     file.kind,
			FileName: file.fileName,
			Row:      row.number,
			Field:    "allocation_amount",
			Value:    allocationValue,
			Message:  "invoice_id or invoice_number is required when allocation_amount is provided",
		})
	}
	allocationAmount, issue := parseCutoverPositiveDecimal(allocationValue, "allocation_amount")
	if issue != nil {
		report.addIssue(cutoverAmountValidationIssue(file, row, *issue))
		return
	}
	if amountOK && allocationAmount.GreaterThan(paymentAmount) {
		report.addIssue(ValidationIssue{
			Severity: SeverityError,
			Kind:     file.kind,
			FileName: file.fileName,
			Row:      row.number,
			Field:    "allocation_amount",
			Value:    allocationValue,
			Message:  "allocation_amount exceeds payment amount",
		})
	}
}

func checkBankTransactionRows(report *BundleValidationReport, file parsedFile) {
	hasDate := fileHasHeaders(file, "date")
	hasAmount := fileHasHeaders(file, "amount")
	for _, row := range file.rows {
		if hasDate {
			checkBankTransactionDate(report, file, row)
		}
		if hasAmount {
			checkBankTransactionAmount(report, file, row)
		}
	}
}

func checkBankTransactionDate(report *BundleValidationReport, file parsedFile, row parsedRow) {
	value := strings.TrimSpace(row.values["date"])
	if value == "" {
		report.addIssue(ValidationIssue{
			Severity: SeverityError,
			Kind:     file.kind,
			FileName: file.fileName,
			Row:      row.number,
			Field:    "date",
			Message:  "date is required",
		})
		return
	}
	if _, err := time.Parse("2006-01-02", value); err == nil {
		return
	}
	report.addIssue(ValidationIssue{
		Severity: SeverityError,
		Kind:     file.kind,
		FileName: file.fileName,
		Row:      row.number,
		Field:    "date",
		Value:    value,
		Message:  "date must be YYYY-MM-DD",
	})
}

func checkBankTransactionAmount(report *BundleValidationReport, file parsedFile, row parsedRow) {
	if _, issue := parseCutoverRequiredDecimal(row.values["amount"], "amount"); issue != nil {
		report.addIssue(cutoverAmountValidationIssue(file, row, *issue))
	}
}

func checkOpeningBalanceTotals(report *BundleValidationReport, file parsedFile) {
	if !fileHasHeaders(file, "debit", "credit") {
		return
	}

	totalDebit := decimal.Zero
	totalCredit := decimal.Zero
	for _, row := range file.rows {
		debit, credit, amountIssue := parseCutoverDebitCredit(row)
		if amountIssue != nil {
			report.addIssue(cutoverAmountValidationIssue(file, row, *amountIssue))
			return
		}
		totalDebit = totalDebit.Add(debit)
		totalCredit = totalCredit.Add(credit)
	}

	if len(file.rows) == 0 {
		return
	}
	if totalDebit.IsZero() || totalCredit.IsZero() {
		report.addIssue(ValidationIssue{
			Severity: SeverityError,
			Kind:     file.kind,
			FileName: file.fileName,
			Field:    "debit/credit",
			Value:    debitCreditTotalsValue(totalDebit, totalCredit),
			Message:  "opening balances must include both debit and credit totals",
		})
		return
	}
	if !totalDebit.Equal(totalCredit) {
		report.addIssue(ValidationIssue{
			Severity: SeverityError,
			Kind:     file.kind,
			FileName: file.fileName,
			Field:    "debit/credit",
			Value:    debitCreditTotalsValue(totalDebit, totalCredit),
			Message:  fmt.Sprintf("opening balances do not balance: debits=%s credits=%s", totalDebit.String(), totalCredit.String()),
		})
	}
}

func checkJournalEntryGroups(report *BundleValidationReport, file parsedFile) {
	if !fileHasHeaders(file, "entry_reference", "entry_date", "debit", "credit") {
		return
	}

	for _, group := range groupJournalRows(file.rows) {
		checkJournalEntryGroup(report, file, group)
	}
}

func groupJournalRows(rows []parsedRow) []*journalValidationGroup {
	groupByReference := make(map[string]*journalValidationGroup)
	groups := make([]*journalValidationGroup, 0)
	for _, row := range rows {
		reference := strings.TrimSpace(row.values["entry_reference"])
		key := normalizedValue(reference)
		if key == "" {
			key = fmt.Sprintf("row-%d", row.number)
		}
		group, ok := groupByReference[key]
		if !ok {
			group = &journalValidationGroup{
				firstRow:  row.number,
				reference: reference,
			}
			groupByReference[key] = group
			groups = append(groups, group)
		}
		group.rows = append(group.rows, row)
	}
	return groups
}

func checkJournalEntryGroup(report *BundleValidationReport, file parsedFile, group *journalValidationGroup) {
	if strings.TrimSpace(group.reference) == "" {
		report.addIssue(ValidationIssue{
			Severity: SeverityError,
			Kind:     file.kind,
			FileName: file.fileName,
			Row:      group.firstRow,
			Field:    "entry_reference",
			Message:  "entry_reference is required",
		})
		return
	}

	groupDate := ""
	hasIssue := false
	totalDebit := decimal.Zero
	totalCredit := decimal.Zero
	for _, row := range group.rows {
		rowDate, dateIssue := parseCutoverEntryDate(row.values["entry_date"])
		if dateIssue != nil {
			report.addIssue(ValidationIssue{
				Severity: SeverityError,
				Kind:     file.kind,
				FileName: file.fileName,
				Row:      row.number,
				Field:    "entry_date",
				Value:    strings.TrimSpace(row.values["entry_date"]),
				Message:  dateIssue.message,
			})
			hasIssue = true
			continue
		}
		if groupDate == "" {
			groupDate = rowDate
		} else if rowDate != groupDate {
			report.addIssue(ValidationIssue{
				Severity: SeverityError,
				Kind:     file.kind,
				FileName: file.fileName,
				Row:      row.number,
				Field:    "entry_date",
				Value:    rowDate,
				Message:  fmt.Sprintf("entry_date must match the group date %s", groupDate),
			})
			hasIssue = true
		}

		debit, credit, amountIssue := parseCutoverDebitCredit(row)
		if amountIssue != nil {
			report.addIssue(cutoverAmountValidationIssue(file, row, *amountIssue))
			hasIssue = true
			continue
		}
		exchangeRate, exchangeIssue := parseCutoverExchangeRate(row.values["exchange_rate"])
		if exchangeIssue != nil {
			report.addIssue(cutoverAmountValidationIssue(file, row, *exchangeIssue))
			hasIssue = true
			continue
		}
		totalDebit = totalDebit.Add(debit.Mul(exchangeRate))
		totalCredit = totalCredit.Add(credit.Mul(exchangeRate))
	}
	if hasIssue {
		return
	}

	if len(group.rows) < 2 {
		report.addIssue(ValidationIssue{
			Severity: SeverityError,
			Kind:     file.kind,
			FileName: file.fileName,
			Row:      group.firstRow,
			Field:    "entry_reference",
			Value:    group.reference,
			Message:  fmt.Sprintf("journal entry %q must have at least two lines", group.reference),
		})
		return
	}
	if totalDebit.IsZero() {
		report.addIssue(ValidationIssue{
			Severity: SeverityError,
			Kind:     file.kind,
			FileName: file.fileName,
			Row:      group.firstRow,
			Field:    "entry_reference/debit/credit",
			Value:    group.reference,
			Message:  fmt.Sprintf("journal entry %q cannot have zero amounts", group.reference),
		})
		return
	}
	if !totalDebit.Equal(totalCredit) {
		report.addIssue(ValidationIssue{
			Severity: SeverityError,
			Kind:     file.kind,
			FileName: file.fileName,
			Row:      group.firstRow,
			Field:    "entry_reference/debit/credit",
			Value:    group.reference,
			Message:  fmt.Sprintf("journal entry %q does not balance: debits=%s credits=%s", group.reference, totalDebit.String(), totalCredit.String()),
		})
	}
}

func parseCutoverDebitCredit(row parsedRow) (decimal.Decimal, decimal.Decimal, *cutoverAmountIssue) {
	debit, err := parseCutoverDecimal(row.values["debit"], "debit")
	if err != nil {
		return decimal.Zero, decimal.Zero, &cutoverAmountIssue{field: "debit", value: strings.TrimSpace(row.values["debit"]), message: err.Error()}
	}
	credit, err := parseCutoverDecimal(row.values["credit"], "credit")
	if err != nil {
		return decimal.Zero, decimal.Zero, &cutoverAmountIssue{field: "credit", value: strings.TrimSpace(row.values["credit"]), message: err.Error()}
	}
	if debit.LessThan(decimal.Zero) || credit.LessThan(decimal.Zero) {
		return decimal.Zero, decimal.Zero, &cutoverAmountIssue{field: "debit/credit", value: debitCreditRowValue(row), message: "amounts cannot be negative"}
	}
	if debit.IsZero() && credit.IsZero() {
		return decimal.Zero, decimal.Zero, &cutoverAmountIssue{field: "debit/credit", value: debitCreditRowValue(row), message: "either debit or credit is required"}
	}
	if debit.GreaterThan(decimal.Zero) && credit.GreaterThan(decimal.Zero) {
		return decimal.Zero, decimal.Zero, &cutoverAmountIssue{field: "debit/credit", value: debitCreditRowValue(row), message: "row cannot contain both debit and credit amounts"}
	}
	return debit, credit, nil
}

func parseCutoverExchangeRate(value string) (decimal.Decimal, *cutoverAmountIssue) {
	exchangeRate, err := parseCutoverDecimal(value, "exchange_rate")
	if err != nil {
		return decimal.Zero, &cutoverAmountIssue{field: "exchange_rate", value: strings.TrimSpace(value), message: err.Error()}
	}
	if exchangeRate.IsZero() {
		return decimal.NewFromInt(1), nil
	}
	if exchangeRate.LessThan(decimal.Zero) {
		return decimal.Zero, &cutoverAmountIssue{field: "exchange_rate", value: strings.TrimSpace(value), message: "exchange_rate cannot be negative"}
	}
	return exchangeRate, nil
}

func parseCutoverDecimal(value, fieldName string) (decimal.Decimal, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return decimal.Zero, nil
	}
	parsed, err := decimal.NewFromString(normalizeCutoverDecimal(trimmed))
	if err != nil {
		return decimal.Zero, fmt.Errorf("invalid %s", fieldName)
	}
	return parsed, nil
}

func parseCutoverPositiveDecimal(value, fieldName string) (decimal.Decimal, *cutoverAmountIssue) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return decimal.Zero, &cutoverAmountIssue{field: fieldName, message: fmt.Sprintf("%s is required", fieldName)}
	}
	parsed, err := decimal.NewFromString(trimmed)
	if err != nil {
		return decimal.Zero, &cutoverAmountIssue{field: fieldName, value: trimmed, message: fmt.Sprintf("%s must be a decimal", fieldName)}
	}
	if parsed.LessThanOrEqual(decimal.Zero) {
		return decimal.Zero, &cutoverAmountIssue{field: fieldName, value: trimmed, message: fmt.Sprintf("%s must be positive", fieldName)}
	}
	return parsed, nil
}

func parseCutoverRequiredDecimal(value, fieldName string) (decimal.Decimal, *cutoverAmountIssue) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return decimal.Zero, &cutoverAmountIssue{field: fieldName, message: fmt.Sprintf("%s is required", fieldName)}
	}
	parsed, err := decimal.NewFromString(trimmed)
	if err != nil {
		return decimal.Zero, &cutoverAmountIssue{field: fieldName, value: trimmed, message: fmt.Sprintf("%s must be a decimal", fieldName)}
	}
	return parsed, nil
}

func parseCutoverPositiveNormalizedDecimal(value, fieldName string) (decimal.Decimal, *cutoverAmountIssue) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return decimal.Zero, &cutoverAmountIssue{field: fieldName, message: fmt.Sprintf("%s is required", fieldName)}
	}
	parsed, err := decimal.NewFromString(strings.ReplaceAll(trimmed, ",", "."))
	if err != nil {
		return decimal.Zero, &cutoverAmountIssue{field: fieldName, value: trimmed, message: fmt.Sprintf("%s must be a decimal", fieldName)}
	}
	if parsed.LessThanOrEqual(decimal.Zero) {
		return decimal.Zero, &cutoverAmountIssue{field: fieldName, value: trimmed, message: fmt.Sprintf("%s must be positive", fieldName)}
	}
	return parsed, nil
}

func parseCutoverEntryDate(value string) (string, *cutoverAmountIssue) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", &cutoverAmountIssue{field: "entry_date", message: "entry_date is required"}
	}
	parsed, err := time.Parse("2006-01-02", trimmed)
	if err != nil {
		return "", &cutoverAmountIssue{field: "entry_date", value: trimmed, message: "entry_date must be in YYYY-MM-DD format"}
	}
	return parsed.Format("2006-01-02"), nil
}

func cutoverAmountValidationIssue(file parsedFile, row parsedRow, amountIssue cutoverAmountIssue) ValidationIssue {
	return ValidationIssue{
		Severity: SeverityError,
		Kind:     file.kind,
		FileName: file.fileName,
		Row:      row.number,
		Field:    amountIssue.field,
		Value:    amountIssue.value,
		Message:  amountIssue.message,
	}
}

func fileHasHeaders(file parsedFile, headers ...string) bool {
	present := make(map[string]bool, len(file.headers))
	for _, header := range file.headers {
		present[header] = true
	}
	for _, header := range headers {
		if !present[header] {
			return false
		}
	}
	return true
}

func normalizeCutoverDecimal(value string) string {
	if strings.Contains(value, ",") && !strings.Contains(value, ".") {
		return strings.ReplaceAll(value, ",", ".")
	}
	return strings.ReplaceAll(value, ",", "")
}

func isCutoverDateOrRFC3339(value string) bool {
	if _, err := time.Parse("2006-01-02", value); err == nil {
		return true
	}
	if _, err := time.Parse(time.RFC3339, value); err == nil {
		return true
	}
	return false
}

func normalizedCutoverStatus(value string) string {
	return strings.ReplaceAll(normalizedValue(value), " ", "_")
}

func debitCreditRowValue(row parsedRow) string {
	return fmt.Sprintf("debit=%s credit=%s", strings.TrimSpace(row.values["debit"]), strings.TrimSpace(row.values["credit"]))
}

func debitCreditTotalsValue(totalDebit, totalCredit decimal.Decimal) string {
	return fmt.Sprintf("debits=%s credits=%s", totalDebit.String(), totalCredit.String())
}

func checkEInvoiceContactReferences(report *BundleValidationReport, indexes bundleIndexes, file parsedFile, row parsedRow, mode EInvoiceContactMode) {
	switch mode {
	case EInvoiceContactModeCustomer:
		checkTargetReference(report, indexes.files[KindContacts], indexes.contacts, file, row, KindContacts, eInvoiceBuyerContactReferenceFields())
	case EInvoiceContactModeBoth:
		checkTargetReference(report, indexes.files[KindContacts], indexes.contacts, file, row, KindContacts, eInvoiceSellerContactReferenceFields())
		checkTargetReference(report, indexes.files[KindContacts], indexes.contacts, file, row, KindContacts, eInvoiceBuyerContactReferenceFields())
	default:
		checkTargetReference(report, indexes.files[KindContacts], indexes.contacts, file, row, KindContacts, eInvoiceSellerContactReferenceFields())
	}
}

func eInvoiceSellerContactReferenceFields() []string {
	return []string{"contact_code", "contact_reg_code", "contact_vat_number", "contact_email", "contact_name"}
}

func eInvoiceBuyerContactReferenceFields() []string {
	return []string{"buyer_reg_code", "buyer_vat_number", "buyer_contact_email", "buyer_contact_name"}
}

func checkBankTransactionSourceAccount(report *BundleValidationReport, indexes bundleIndexes, file parsedFile, row parsedRow) {
	if !indexes.files[KindBankAccounts] {
		return
	}

	sourceAccount := strings.TrimSpace(row.values["source_account"])
	if sourceAccount == "" {
		return
	}

	accountCurrency, ok := indexes.bankAccounts[bankAccountIndexKey(sourceAccount)]
	if !ok {
		report.addIssue(ValidationIssue{
			Severity:   SeverityError,
			Kind:       file.kind,
			FileName:   file.fileName,
			Row:        row.number,
			Field:      "source_account",
			Value:      sourceAccount,
			TargetKind: KindBankAccounts,
			Message:    fmt.Sprintf("source_account reference %q was not found in %s file", sourceAccount, KindBankAccounts),
		})
		return
	}

	rowCurrency := strings.ToUpper(strings.TrimSpace(row.values["currency"]))
	if rowCurrency == "" || accountCurrency == "" || rowCurrency == accountCurrency {
		return
	}

	report.addIssue(ValidationIssue{
		Severity:   SeverityError,
		Kind:       file.kind,
		FileName:   file.fileName,
		Row:        row.number,
		Field:      "source_account/currency",
		Value:      sourceAccount + "/" + rowCurrency,
		TargetKind: KindBankAccounts,
		Message:    fmt.Sprintf("source_account %q uses currency %q but %s file has currency %q", sourceAccount, rowCurrency, KindBankAccounts, accountCurrency),
	})
}

func checkEmployeeReference(report *BundleValidationReport, indexes bundleIndexes, file parsedFile, row parsedRow) {
	if !indexes.files[KindEmployees] {
		return
	}
	values := []string{
		row.values["employee_number"],
		row.values["personal_code"],
		row.values["email"],
		employeeName(row.values),
	}
	checkReferenceValues(report, indexes.employees, file, row, KindEmployees, "employee", values)
}

func checkSelfReference(report *BundleValidationReport, file parsedFile, row parsedRow, field, identityField string) {
	value := strings.TrimSpace(row.values[field])
	identity := strings.TrimSpace(row.values[identityField])
	if value == "" || identity == "" || normalizedValue(value) != normalizedValue(identity) {
		return
	}

	report.addIssue(ValidationIssue{
		Severity:   SeverityError,
		Kind:       file.kind,
		FileName:   file.fileName,
		Row:        row.number,
		Field:      field,
		Value:      value,
		TargetKind: file.kind,
		Message:    fmt.Sprintf("%s cannot reference the same row's %s", field, identityField),
	})
}

func checkOptionalUUID(report *BundleValidationReport, file parsedFile, row parsedRow, field string) {
	value := strings.TrimSpace(row.values[field])
	if value == "" {
		return
	}
	if _, err := uuid.Parse(value); err == nil {
		return
	}

	report.addIssue(ValidationIssue{
		Severity: SeverityError,
		Kind:     file.kind,
		FileName: file.fileName,
		Row:      row.number,
		Field:    field,
		Value:    value,
		Message:  fmt.Sprintf("%s must be a valid UUID", field),
	})
}

func checkTargetReference(
	report *BundleValidationReport,
	targetPresent bool,
	targetIndex map[string]bool,
	file parsedFile,
	row parsedRow,
	targetKind FileKind,
	fields []string,
) {
	if !targetPresent {
		return
	}
	values := make([]string, 0, len(fields))
	for _, field := range fields {
		values = append(values, row.values[field])
	}
	checkReferenceValues(report, targetIndex, file, row, targetKind, strings.Join(fields, "/"), values)
}

func checkReferenceValues(
	report *BundleValidationReport,
	targetIndex map[string]bool,
	file parsedFile,
	row parsedRow,
	targetKind FileKind,
	field string,
	values []string,
) {
	var firstValue string
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if firstValue == "" {
			firstValue = trimmed
		}
		if targetIndex[normalizedValue(trimmed)] {
			return
		}
	}
	if firstValue == "" {
		return
	}
	report.addIssue(ValidationIssue{
		Severity:   SeverityError,
		Kind:       file.kind,
		FileName:   file.fileName,
		Row:        row.number,
		Field:      field,
		Value:      firstValue,
		TargetKind: targetKind,
		Message:    fmt.Sprintf("%s reference %q was not found in %s file", field, firstValue, targetKind),
	})
}

func (r *BundleValidationReport) addIssue(issue ValidationIssue) {
	r.Issues = append(r.Issues, issue)
	switch issue.Severity {
	case SeverityWarning:
		r.Summary.WarningCount++
	default:
		r.Summary.ErrorCount++
	}
}

func missingRequiredGroups(groups [][]string, headerSet map[string]bool) []string {
	var missing []string
	for _, group := range groups {
		found := false
		for _, column := range group {
			if headerSet[column] {
				found = true
				break
			}
		}
		if !found {
			missing = append(missing, strings.Join(group, "|"))
		}
	}
	return missing
}

func addEmployeeIndexValues(index map[string]bool, values map[string]string) {
	addIndexValue(index, values["employee_number"])
	addIndexValue(index, values["personal_code"])
	addIndexValue(index, values["email"])
	addIndexValue(index, employeeName(values))
}

func addBankAccountIndexValue(index map[string]string, accountNumber, currency string) {
	key := bankAccountIndexKey(accountNumber)
	if key == "" {
		return
	}
	normalizedCurrency := strings.ToUpper(strings.TrimSpace(currency))
	if normalizedCurrency == "" {
		normalizedCurrency = "EUR"
	}
	index[key] = normalizedCurrency
}

func addIndexValue(index map[string]bool, value string) {
	key := normalizedValue(value)
	if key != "" {
		index[key] = true
	}
}

func employeeName(values map[string]string) string {
	if name := strings.TrimSpace(values["name"]); name != "" {
		return name
	}
	return strings.TrimSpace(strings.Join([]string{values["first_name"], values["last_name"]}, " "))
}

func displayFileName(file BundleFile) string {
	if strings.TrimSpace(file.FileName) != "" {
		return strings.TrimSpace(file.FileName)
	}
	if file.Kind == KindEInvoices {
		return string(file.Kind) + ".xml"
	}
	return string(file.Kind) + ".csv"
}

func canonicalHeader(aliases map[string]string, value string) string {
	normalized := normalizedHeader(value)
	if canonical, ok := aliases[normalized]; ok {
		return canonical
	}
	return normalized
}

func normalizedHeader(value string) string {
	value = strings.TrimPrefix(strings.TrimSpace(strings.ToLower(value)), "\ufeff")
	var builder strings.Builder
	lastUnderscore := false
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			builder.WriteRune(r)
			lastUnderscore = false
			continue
		}
		if !lastUnderscore {
			builder.WriteByte('_')
			lastUnderscore = true
		}
	}
	return strings.Trim(builder.String(), "_")
}

func normalizedValue(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func bankAccountIndexKey(value string) string {
	return strings.ReplaceAll(normalizedValue(value), " ", "")
}

func detectDelimiter(content string) rune {
	firstLine := content
	if idx := strings.IndexAny(content, "\r\n"); idx >= 0 {
		firstLine = content[:idx]
	}
	candidates := []rune{',', ';', '\t'}
	best := ','
	bestCount := -1
	for _, candidate := range candidates {
		count := strings.Count(firstLine, string(candidate))
		if count > bestCount {
			best = candidate
			bestCount = count
		}
	}
	return best
}

func commonAliases() map[string]string {
	return map[string]string{
		"id":          "id",
		"code":        "code",
		"name":        "name",
		"email":       "email",
		"amount":      "amount",
		"debit":       "debit",
		"credit":      "credit",
		"currency":    "currency",
		"status":      "status",
		"description": "description",
	}
}

func commercialDocumentAliases() map[string]string {
	return mergeAliases(commonAliases(), map[string]string{
		"number":             "number",
		"contact_id":         "contact_id",
		"customer_id":        "contact_id",
		"contact_code":       "contact_code",
		"customer_code":      "contact_code",
		"contact_reg_code":   "contact_reg_code",
		"contact_vat_number": "contact_vat_number",
		"vat_number":         "contact_vat_number",
		"contact_email":      "contact_email",
		"email":              "contact_email",
		"contact_name":       "contact_name",
		"customer_name":      "contact_name",
		"currency":           "currency",
		"exchange_rate":      "exchange_rate",
		"notes":              "notes",
		"line_description":   "line_description",
		"description":        "line_description",
		"quantity":           "quantity",
		"qty":                "quantity",
		"unit":               "unit",
		"unit_price":         "unit_price",
		"price":              "unit_price",
		"discount_percent":   "discount_percent",
		"discount":           "discount_percent",
		"vat_rate":           "vat_rate",
		"vat":                "vat_rate",
		"product":            "product_code",
		"product_code":       "product_code",
		"sku":                "product_code",
		"item_code":          "product_code",
		"product_id":         "product_id",
		"invoice_type":       "invoice_type",
		"type":               "invoice_type",
		"is_active":          "is_active",
		"active":             "is_active",
	})
}

func commercialDocumentRequiredGroups(numberColumn, dateColumn string) [][]string {
	return [][]string{
		{numberColumn},
		{dateColumn},
		{"contact_id", "contact_code", "contact_reg_code", "contact_vat_number", "contact_email", "contact_name"},
		{"line_description"},
		{"quantity"},
		{"unit_price"},
		{"vat_rate"},
	}
}

func commercialDocumentContactReferenceFields() []string {
	return []string{"contact_id", "contact_code", "contact_reg_code", "contact_vat_number", "contact_email", "contact_name"}
}

func employeeReferenceRequiredGroups() [][]string {
	return [][]string{
		{"employee_number", "personal_code", "email", "name", "first_name"},
		{"employee_number", "personal_code", "email", "name", "last_name"},
	}
}

func payrollHistoryRequiredGroups() [][]string {
	groups := [][]string{{"period_year"}, {"period_month"}}
	groups = append(groups, employeeReferenceRequiredGroups()...)
	return append(groups, []string{"gross_salary"})
}

func leaveBalanceRequiredGroups() [][]string {
	groups := [][]string{{"year"}}
	groups = append(groups, employeeReferenceRequiredGroups()...)
	return append(groups, []string{"absence_type_code", "absence_type", "absence_type_id"})
}

func tsdHistoryRequiredGroups() [][]string {
	groups := [][]string{{"period_year"}, {"period_month"}}
	groups = append(groups, employeeReferenceRequiredGroups()...)
	return append(groups, []string{"gross_payment"})
}

func employeeReferenceAliases() map[string]string {
	return mergeAliases(commonAliases(), map[string]string{
		"employee_number": "employee_number",
		"employee_no":     "employee_number",
		"employee_code":   "employee_number",
		"employee_id":     "employee_number",
		"personal_code":   "personal_code",
		"isikukood":       "personal_code",
		"e_mail":          "email",
		"first_name":      "first_name",
		"last_name":       "last_name",
	})
}

func mergeAliases(base map[string]string, extra map[string]string) map[string]string {
	merged := make(map[string]string, len(base)+len(extra))
	for key, value := range base {
		merged[normalizedHeader(key)] = value
	}
	for key, value := range extra {
		merged[normalizedHeader(key)] = value
	}
	return merged
}
