package cutover

import (
	"fmt"
	"strings"
)

var providerPresetAliases = map[MigrationProviderPreset]map[FileKind]map[string]string{
	MigrationProviderPresetMerit: {
		KindAccounts: {
			"konto":         "code",
			"konto_kood":    "code",
			"kontokood":     "code",
			"konto_nimi":    "name",
			"kontonimi":     "name",
			"konto_nimetus": "name",
			"konto_tüüp":    "account_type",
			"konto_tyyp":    "account_type",
			"tüüp":          "account_type",
			"tyyp":          "account_type",
			"liik":          "account_type",
		},
		KindContacts: {
			"kood":          "code",
			"kliendi_kood":  "code",
			"kliendikood":   "code",
			"hankija_kood":  "code",
			"hankijakood":   "code",
			"klient":        "name",
			"hankija":       "name",
			"nimi":          "name",
			"registrikood":  "reg_code",
			"registri_kood": "reg_code",
			"reg_nr":        "reg_code",
			"kmkr_nr":       "vat_number",
			"kmkr":          "vat_number",
			"e_post":        "email",
			"epost":         "email",
		},
		KindInvoices: mergeAliases(meritCommercialDocumentAliases("invoice_number", "issue_date"), map[string]string{
			"arve_nr":      "invoice_number",
			"arvenr":       "invoice_number",
			"arve_number":  "invoice_number",
			"arve_kuupäev": "issue_date",
			"arve_kuupaev": "issue_date",
			"arve_tüüp":    "invoice_type",
			"arve_tyyp":    "invoice_type",
			"arve_liik":    "invoice_type",
		}),
		KindQuotes: mergeAliases(meritCommercialDocumentAliases("quote_number", "quote_date"), map[string]string{
			"pakkumise_nr":      "quote_number",
			"pakkumisenr":       "quote_number",
			"pakkumise_kuupäev": "quote_date",
			"pakkumise_kuupaev": "quote_date",
		}),
		KindOrders: mergeAliases(meritCommercialDocumentAliases("order_number", "order_date"), map[string]string{
			"tellimuse_nr":      "order_number",
			"tellimusenr":       "order_number",
			"tellimuse_kuupäev": "order_date",
			"tellimuse_kuupaev": "order_date",
		}),
		KindRecurringInvoices: meritCommercialDocumentAliases("name", "start_date"),
		KindPayments: {
			"makse_nr":    "payment_number", //nolint:misspell // Estonian CSV header alias.
			"maksenr":     "payment_number",
			"makse_tüüp":  "payment_type", //nolint:misspell // Estonian CSV header alias.
			"makse_tyyp":  "payment_type", //nolint:misspell // Estonian CSV header alias.
			"kuupäev":     "payment_date",
			"kuupaev":     "payment_date",
			"summa":       "amount",
			"arve_nr":     "invoice_number",
			"arvenr":      "invoice_number",
			"arve_number": "invoice_number",
		},
		KindBankAccounts: {
			"pangakonto":  "account_number",
			"panga_konto": "account_number",
			"konto_nr":    "account_number",
			"kontonr":     "account_number",
			"pank":        "bank_name",
			"panga_nimi":  "bank_name",
			"konto_nimi":  "name",
		},
		KindBankTransactions: {
			"kuupäev":           "date",
			"kuupaev":           "date",
			"summa":             "amount",
			"selgitus":          "description",
			"kirjeldus":         "description",
			"viitenumber":       "reference",
			"viite_number":      "reference",
			"vastaspool":        "counterparty_name",
			"vastaspoole_konto": "counterparty_account",
			"konto":             "source_account",
			"valuuta":           "currency",
		},
		KindCostCenters: {
			"kulukoht":         "code",
			"kulukoha_kood":    "code",
			"kulukoha_nimi":    "name",
			"kulukoha_nimetus": "name",
			"nimetus":          "name",
		},
		KindProducts: {
			"artikkel":     "code",
			"artikli_kood": "code",
			"toode":        "name",
			"toote_nimi":   "name",
			"nimetus":      "name",
			"müügihind":    "sales_price",
			"myygihind":    "sales_price",
			"ostuhind":     "purchase_price",
			"käibemaks":    "vat_rate",
			"kaibemaks":    "vat_rate",
		},
		KindOpeningBalances: {
			"konto":     "account_code",
			"deebet":    "debit",
			"kreedit":   "credit",
			"kulukoht":  "cost_center_code",
			"projekt":   "project_code",
			"kirjeldus": "description",
			"selgitus":  "description",
		},
		KindJournalEntries: {
			"kanne_nr":      "entry_reference",
			"kandenr":       "entry_reference",
			"dokumendi_nr":  "entry_reference",
			"dokumendinr":   "entry_reference",
			"kuupäev":       "entry_date",
			"kuupaev":       "entry_date",
			"konto":         "account_code",
			"deebet":        "debit",
			"kreedit":       "credit",
			"selgitus":      "line_description",
			"kirjeldus":     "line_description",
			"valuutakurss":  "exchange_rate",
			"valuuta_kurss": "exchange_rate",
		},
		KindEmployees: {
			"importcode":          "employee_number",
			"import_code":         "employee_number",
			"contractcode":        "employee_number",
			"contract_code":       "employee_number",
			"contractno":          "employee_number",
			"contract_no":         "employee_number",
			"personalcode":        "personal_code",
			"personal_code":       "personal_code",
			"firstname":           "first_name",
			"first_name":          "first_name",
			"surname":             "last_name",
			"sur_name":            "last_name",
			"bankaccountno":       "bank_account",
			"bank_account_no":     "bank_account",
			"phoneno":             "phone",
			"phone_no":            "phone",
			"startdate":           "start_date",
			"start_date":          "start_date",
			"enddate":             "end_date",
			"end_date":            "end_date",
			"amount":              "base_salary",
			"salarytypeimpcode":   "salary_type_import_code",
			"salary_type_impcode": "salary_type_import_code",
			"departmentcode":      "department",
			"department_code":     "department",
		},
		KindPayrollHistory: meritPayrollHistoryAliases("gross_salary"),
		KindLeaveBalances: {
			"contractcode":     "employee_number",
			"contract_code":    "employee_number",
			"personalcode":     "personal_code",
			"personal_code":    "personal_code",
			"date":             "balance_date",
			"balancedate":      "balance_date",
			"balance_date":     "balance_date",
			"typecode":         "absence_type_code",
			"type_code":        "absence_type_code",
			"days":             "entitled_days",
			"daysobligations":  "entitled_days",
			"days_obligations": "entitled_days",
			"initbalance":      "carryover_days",
			"init_balance":     "carryover_days",
			"daysunused":       "carryover_days",
			"days_unused":      "carryover_days",
			"daysacquired":     "used_days",
			"days_acquired":    "used_days",
		},
		KindTSDHistory: meritPayrollHistoryAliases("gross_payment"),
	},
	MigrationProviderPresetSmartAccounts: {
		KindAccounts: {
			"account_no":          "code",
			"account_number":      "code",
			"account_title":       "name",
			"account_description": "name",
			"classification":      "account_type",
			"parent_account":      "parent_code",
			"parent_account_no":   "parent_code",
		},
		KindContacts: {
			"client_no":       "code",
			"client_number":   "code",
			"client_name":     "name",
			"customer_no":     "code",
			"customer_number": "code",
			"vendor_no":       "code",
			"vendor_number":   "code",
			"vendor_name":     "name",
			"supplier_no":     "code",
			"supplier_number": "code",
			"registration_no": "reg_code",
			"registry_no":     "reg_code",
			"vat_no":          "vat_number",
			"email_address":   "email",
		},
		KindInvoices:          smartAccountsCommercialDocumentAliases("invoice_number", "issue_date"),
		KindQuotes:            smartAccountsCommercialDocumentAliases("quote_number", "quote_date"),
		KindOrders:            smartAccountsCommercialDocumentAliases("order_number", "order_date"),
		KindRecurringInvoices: smartAccountsCommercialDocumentAliases("name", "start_date"),
		KindPayments: {
			"payment_no":      "payment_number",
			"payment_number":  "payment_number",
			"payment_date":    "payment_date",
			"payment_kind":    "payment_type",
			"paid_amount":     "amount",
			"document_no":     "invoice_number",
			"document_number": "invoice_number",
			"invoice_no":      "invoice_number",
		},
		KindBankAccounts: {
			"bank_account_no":   "account_number",
			"bank_account_name": "name",
			"account_no":        "account_number",
			"bank_name":         "bank_name",
		},
		KindBankTransactions: {
			"transaction_date": "date",
			"booking_date":     "date",
			"transaction_sum":  "amount",
			"transaction_text": "description",
			"account_no":       "source_account",
			"bank_account_no":  "source_account",
		},
		KindProducts: {
			"item_no":        "code",
			"item_number":    "code",
			"item_name":      "name",
			"item_type":      "product_type",
			"sales_price":    "sales_price",
			"purchase_price": "purchase_price",
			"vat_percent":    "vat_rate",
		},
		KindOpeningBalances: {
			"account_no":     "account_code",
			"account_number": "account_code",
			"debit_amount":   "debit",
			"credit_amount":  "credit",
			"line_memo":      "description",
		},
		KindJournalEntries: {
			"entry_no":         "entry_reference",
			"entry_number":     "entry_reference",
			"transaction_no":   "entry_reference",
			"transaction_date": "entry_date",
			"account_no":       "account_code",
			"account_number":   "account_code",
			"debit_amount":     "debit",
			"credit_amount":    "credit",
			"line_memo":        "line_description",
			"transaction_memo": "entry_description",
		},
		KindEmployees: {
			"employee_no":     "employee_number",
			"employee_number": "employee_number",
			"personal_no":     "personal_code",
			"personal_code":   "personal_code",
			"firstname":       "first_name",
			"first_name":      "first_name",
			"surname":         "last_name",
			"last_name":       "last_name",
			"bank_account_no": "bank_account",
			"start_date":      "start_date",
			"end_date":        "end_date",
			"salary_amount":   "base_salary",
		},
		KindPayrollHistory: {
			"employee_no":       "employee_number",
			"employee_number":   "employee_number",
			"personal_no":       "personal_code",
			"personal_code":     "personal_code",
			"employee_name":     "name",
			"period":            "period_code",
			"period_code":       "period_code",
			"accounting_period": "period_code",
			"gross_amount":      "gross_salary",
			"salary_amount":     "gross_salary",
			"net_amount":        "net_salary",
			"employer_cost":     "total_employer_cost",
			"social_tax_amount": "social_tax",
			"income_tax_amount": "income_tax",
		},
		KindLeaveBalances: {
			"employee_no":      "employee_number",
			"employee_number":  "employee_number",
			"personal_no":      "personal_code",
			"personal_code":    "personal_code",
			"balance_date":     "balance_date",
			"leave_type_code":  "absence_type_code",
			"leave_type":       "absence_type",
			"entitlement_days": "entitled_days",
			"unused_days":      "carryover_days",
			"used_days":        "used_days",
		},
		KindTSDHistory: {
			"employee_no":       "employee_number",
			"employee_number":   "employee_number",
			"personal_no":       "personal_code",
			"personal_code":     "personal_code",
			"employee_name":     "name",
			"period":            "period_code",
			"period_code":       "period_code",
			"accounting_period": "period_code",
			"gross_amount":      "gross_payment",
			"salary_amount":     "gross_payment",
			"income_tax_amount": "income_tax",
			"social_tax_amount": "social_tax",
		},
	},
}

func normalizeMigrationProviderPreset(preset MigrationProviderPreset) (MigrationProviderPreset, error) {
	switch normalized := normalizedHeader(string(preset)); normalized {
	case "":
		return MigrationProviderPresetGeneric, nil
	case string(MigrationProviderPresetGeneric):
		return MigrationProviderPresetGeneric, nil
	case string(MigrationProviderPresetMerit):
		return MigrationProviderPresetMerit, nil
	case string(MigrationProviderPresetSmartAccounts), "smart_accounts", "smart_account":
		return MigrationProviderPresetSmartAccounts, nil
	default:
		return "", fmt.Errorf("unsupported provider_preset %q (expected generic, merit, or smartaccounts)", preset)
	}
}

func fileSpecForProviderPreset(kind FileKind, preset MigrationProviderPreset) fileSpec {
	spec := fileSpecs[kind]
	if preset == MigrationProviderPresetGeneric {
		return spec
	}
	aliases := providerPresetAliases[preset][kind]
	if len(aliases) == 0 {
		return spec
	}
	spec.aliases = mergeAliases(spec.aliases, aliases)
	return spec
}

func meritCommercialDocumentAliases(numberField, dateField string) map[string]string {
	return map[string]string{
		"kuupäev":       dateField,
		"kuupaev":       dateField,
		"kliendi_kood":  "contact_code",
		"kliendikood":   "contact_code",
		"hankija_kood":  "contact_code",
		"hankijakood":   "contact_code",
		"klient":        "contact_name",
		"hankija":       "contact_name",
		"registrikood":  "contact_reg_code",
		"kmkr_nr":       "contact_vat_number",
		"e_post":        "contact_email",
		"epost":         "contact_email",
		"rea_kirjeldus": "line_description",
		"kirjeldus":     "line_description",
		"nimetus":       "line_description",
		"kogus":         "quantity",
		"ühiku_hind":    "unit_price",
		"yhiku_hind":    "unit_price",
		"hind":          "unit_price",
		"käibemaks":     "vat_rate",
		"kaibemaks":     "vat_rate",
		"km":            "vat_rate",
		"artikkel":      "product_code",
		"artikli_kood":  "product_code",
		"toode":         "product_code",
	}
}

func smartAccountsCommercialDocumentAliases(numberField, dateField string) map[string]string {
	aliases := map[string]string{
		"document_no":      numberField,
		"document_number":  numberField,
		"document_date":    dateField,
		"client_no":        "contact_code",
		"client_number":    "contact_code",
		"customer_no":      "contact_code",
		"customer_number":  "contact_code",
		"vendor_no":        "contact_code",
		"vendor_number":    "contact_code",
		"supplier_no":      "contact_code",
		"supplier_number":  "contact_code",
		"client":           "contact_name",
		"customer":         "contact_name",
		"vendor":           "contact_name",
		"supplier":         "contact_name",
		"item_description": "line_description",
		"item_name":        "line_description",
		"qty":              "quantity",
		"vat_percent":      "vat_rate",
		"vat_percentage":   "vat_rate",
		"item_no":          "product_code",
		"item_number":      "product_code",
	}
	if strings.HasPrefix(numberField, "invoice") {
		aliases["invoice_no"] = numberField
		aliases["invoice_number"] = numberField
		aliases["invoice_date"] = dateField
		aliases["document_type"] = "invoice_type"
		aliases["invoice_type"] = "invoice_type"
	}
	return aliases
}

func meritPayrollHistoryAliases(grossField string) map[string]string {
	return map[string]string{
		"contractcode":             "employee_number",
		"contract_code":            "employee_number",
		"importcode":               "employee_number",
		"import_code":              "employee_number",
		"personalcode":             "personal_code",
		"personal_code":            "personal_code",
		"employeefullname":         "name",
		"employee_full_name":       "name",
		"month6":                   "period_code",
		"period":                   "period_code",
		"period_code":              "period_code",
		"salarytypename":           "salary_type_name",
		"salary_type_name":         "salary_type_name",
		"sum":                      grossField,
		"totalsum":                 "total_employer_cost",
		"total_sum":                "total_employer_cost",
		"employerunempinsurance":   "unemployment_insurance_employer",
		"employer_unemp_insurance": "unemployment_insurance_employer",
		"socialtax":                "social_tax",
		"social_tax":               "social_tax",
		"vacationreserve":          "vacation_reserve",
		"vacation_reserve":         "vacation_reserve",
		"incometax":                "income_tax",
		"income_tax":               "income_tax",
		"employeeunempinsurance":   "unemployment_insurance_employee",
		"employee_unemp_insurance": "unemployment_insurance_employee",
		"fundedpension":            "funded_pension",
		"funded_pension":           "funded_pension",
	}
}
