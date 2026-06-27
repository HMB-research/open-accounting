package expenses

import (
	"context"
	"testing"
	"time"

	"github.com/HMB-research/open-accounting/internal/contactrefs"
	"github.com/HMB-research/open-accounting/internal/contacts"
	"github.com/HMB-research/open-accounting/internal/payroll"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServiceImportExpensesCSV(t *testing.T) {
	repo := newMemoryRepository()
	service := NewServiceWithRepository(repo, newFakeAccountingPoster(), &fakeEvidenceEvaluator{compliant: true})
	service.now = fixedExpenseNow

	result, err := service.ImportExpensesCSV(context.Background(), "tenant_acme", "tenant-1", &ImportExpensesRequest{
		FileName: "expenses.csv",
		UserID:   "user-1",
		CSVContent: "expense_number,expense_date,merchant,description,expense_account_code,payment_account_code,amount,currency,exchange_rate,requires_receipt,status\n" +
			"EXP-LEG-1,2026-05-30,Office Store,Toner,5500,1000,120.50,EUR,1,true,DRAFT\n" +
			"EXP-LEG-2,2026-05-31,Taxi,Ride,5500,1000,25.00,EUR,1,false,APPROVED\n",
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "expenses.csv", result.FileName)
	assert.Equal(t, 2, result.RowsProcessed)
	assert.Equal(t, 2, result.ExpensesCreated)
	assert.Equal(t, 0, result.RowsSkipped)
	assert.Len(t, repo.expenses, 2)
	assert.Equal(t, StatusDraft, repo.expensesByNumber(t, "EXP-LEG-1").Status)
	approved := repo.expensesByNumber(t, "EXP-LEG-2")
	assert.Equal(t, StatusApproved, approved.Status)
	assert.Equal(t, "expense-account", approved.ExpenseAccountID)
	assert.Equal(t, "cash-account", approved.PaymentAccountID)
	assert.False(t, approved.RequiresReceipt)
	require.NotNil(t, approved.ApprovedAt)
}

func TestServiceImportExpensesCSVResolvesContactIdentityFields(t *testing.T) {
	contactID := "55555555-5555-4555-8555-555555555555"
	repo := newMemoryRepository()
	service := NewServiceWithRepository(repo, newFakeAccountingPoster(), &fakeEvidenceEvaluator{compliant: true})
	service.now = fixedExpenseNow
	service.contacts = &fakeExpenseContactLister{contacts: []contacts.Contact{
		{
			ID:        contactID,
			Code:      "SUP-1",
			Name:      "Supplier One",
			RegCode:   "12345678",
			VATNumber: "EE12345678",
			Email:     "billing@supplier.example",
		},
	}}

	result, err := service.ImportExpensesCSV(context.Background(), "tenant_acme", "tenant-1", &ImportExpensesRequest{
		FileName: "expenses.csv",
		UserID:   "user-1",
		CSVContent: "expense_number,expense_date,merchant,expense_account_code,payment_account_code,amount,contact_code,contact_reg_code,contact_vat_number,contact_email,contact_name\n" +
			"EXP-CODE,2026-05-30,Supplier One,5500,1000,10,SUP-1,,,,\n" +
			"EXP-REG,2026-05-31,Supplier One,5500,1000,10,,12345678,,,\n" +
			"EXP-VAT,2026-06-01,Supplier One,5500,1000,10,,,EE12345678,,\n" +
			"EXP-EMAIL,2026-06-02,Supplier One,5500,1000,10,,,,BILLING@SUPPLIER.EXAMPLE,\n" +
			"EXP-NAME,2026-06-03,Supplier One,5500,1000,10,,,,,Supplier One\n",
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 5, result.RowsProcessed)
	assert.Equal(t, 5, result.ExpensesCreated)
	assert.Zero(t, result.RowsSkipped)
	assert.Empty(t, result.Errors)
	for _, expense := range repo.expenses {
		require.NotNil(t, expense.ContactID)
		assert.Equal(t, contactID, *expense.ContactID)
	}
}

func TestServiceImportExpensesCSVReportsAmbiguousContactName(t *testing.T) {
	repo := newMemoryRepository()
	service := NewServiceWithRepository(repo, newFakeAccountingPoster(), &fakeEvidenceEvaluator{compliant: true})
	service.now = fixedExpenseNow
	service.contacts = &fakeExpenseContactLister{contacts: []contacts.Contact{
		{ID: "11111111-1111-4111-8111-111111111111", Name: "Supplier One"},
		{ID: "22222222-2222-4222-8222-222222222222", Name: " supplier one "},
	}}

	result, err := service.ImportExpensesCSV(context.Background(), "tenant_acme", "tenant-1", &ImportExpensesRequest{
		FileName: "expenses.csv",
		UserID:   "user-1",
		CSVContent: "expense_number,expense_date,merchant,expense_account_code,payment_account_code,amount,contact_name\n" +
			"EXP-AMBIGUOUS,2026-05-30,Supplier One,5500,1000,10,Supplier One\n",
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 1, result.RowsProcessed)
	assert.Zero(t, result.ExpensesCreated)
	assert.Equal(t, 1, result.RowsSkipped)
	require.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0].Message, `contact_name "Supplier One" matched multiple contacts`)
	assert.Empty(t, repo.expenses)
}

func TestServiceImportExpensesCSVSkipsInvalidRows(t *testing.T) {
	expenseAccountID := "99999999-9999-4999-8999-999999999999"
	paymentAccountID := "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	repo := newMemoryRepository()
	service := NewServiceWithRepository(repo, newFakeAccountingPoster(), &fakeEvidenceEvaluator{compliant: true})
	service.now = fixedExpenseNow
	lockDate := time.Date(2026, 5, 30, 0, 0, 0, 0, time.UTC)

	result, err := service.ImportExpensesCSV(context.Background(), "tenant_acme", "tenant-1", &ImportExpensesRequest{
		UserID:   "user-1",
		LockDate: &lockDate,
		CSVContent: "expense_number,expense_date,merchant,expense_account_id,payment_account_id,amount,status,rejection_reason\n" +
			"EXP-LOCKED,2026-05-30,Locked," + expenseAccountID + "," + paymentAccountID + ",10,DRAFT,\n" +
			"EXP-POSTED,2026-05-31,Posted," + expenseAccountID + "," + paymentAccountID + ",20,POSTED,\n" +
			"EXP-REJECTED,2026-05-31,Rejected," + expenseAccountID + "," + paymentAccountID + ",30,REJECTED,Missing receipt\n",
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 3, result.RowsProcessed)
	assert.Equal(t, 1, result.ExpensesCreated)
	assert.Equal(t, 2, result.RowsSkipped)
	require.Len(t, result.Errors, 2)
	assert.Contains(t, result.Errors[0].Message, "period locked through 2026-05-30")
	assert.Contains(t, result.Errors[1].Message, "posted expenses must be imported")
	assert.Equal(t, StatusRejected, repo.expensesByNumber(t, "EXP-REJECTED").Status)
}

type fakeExpenseContactLister struct {
	contacts []contacts.Contact
	err      error
}

func (f *fakeExpenseContactLister) List(_ context.Context, _, _ string, _ *contacts.ContactFilter) ([]contacts.Contact, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.contacts, nil
}

type fakeExpenseEmployeeLister struct {
	employees []payroll.Employee
	err       error
}

func (f *fakeExpenseEmployeeLister) ListEmployees(_ context.Context, _, _ string, _ bool) ([]payroll.Employee, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.employees, nil
}

func TestServiceImportExpensesCSVReportsInvalidUUIDReferences(t *testing.T) {
	expenseAccountID := "99999999-9999-4999-8999-999999999999"
	paymentAccountID := "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	repo := newMemoryRepository()
	service := NewServiceWithRepository(repo, newFakeAccountingPoster(), &fakeEvidenceEvaluator{compliant: true})
	service.now = fixedExpenseNow

	result, err := service.ImportExpensesCSV(context.Background(), "tenant_acme", "tenant-1", &ImportExpensesRequest{
		UserID: "user-1",
		CSVContent: "expense_number,expense_date,merchant,employee_id,contact_id,expense_account_id,payment_account_id,amount\n" +
			"EXP-BAD-EMPLOYEE,2026-05-31,Employee,legacy-employee,," + expenseAccountID + "," + paymentAccountID + ",10\n" +
			"EXP-BAD-CONTACT,2026-05-31,Contact,,legacy-contact," + expenseAccountID + "," + paymentAccountID + ",10\n" +
			"EXP-BAD-EXPENSE-ACCOUNT,2026-05-31,Expense Account,,,legacy-expense-account," + paymentAccountID + ",10\n" +
			"EXP-BAD-PAYMENT-ACCOUNT,2026-05-31,Payment Account,,," + expenseAccountID + ",legacy-payment-account,10\n",
	})

	require.NoError(t, err)
	assert.Equal(t, 4, result.RowsProcessed)
	assert.Zero(t, result.ExpensesCreated)
	assert.Equal(t, 4, result.RowsSkipped)
	require.Len(t, result.Errors, 4)
	assert.Contains(t, result.Errors[0].Message, "employee_id must be a valid UUID")
	assert.Contains(t, result.Errors[1].Message, "contact_id must be a valid UUID")
	assert.Contains(t, result.Errors[2].Message, "expense_account_id must be a valid UUID")
	assert.Contains(t, result.Errors[3].Message, "payment_account_id must be a valid UUID")
}

func TestServiceImportExpensesCSVResolvesEmployeeIDs(t *testing.T) {
	employeeID := "11111111-1111-4111-8111-111111111111"
	unknownEmployeeID := "22222222-2222-4222-8222-222222222222"
	expenseAccountID := "99999999-9999-4999-8999-999999999999"
	paymentAccountID := "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	repo := newMemoryRepository()
	service := NewServiceWithRepository(repo, newFakeAccountingPoster(), &fakeEvidenceEvaluator{compliant: true})
	service.now = fixedExpenseNow
	service.employees = &fakeExpenseEmployeeLister{employees: []payroll.Employee{{ID: employeeID}}}

	result, err := service.ImportExpensesCSV(context.Background(), "tenant_acme", "tenant-1", &ImportExpensesRequest{
		UserID: "user-1",
		CSVContent: "expense_number,expense_date,merchant,employee_id,expense_account_id,payment_account_id,amount\n" +
			"EXP-EMPLOYEE,2026-05-31,Employee," + employeeID + "," + expenseAccountID + "," + paymentAccountID + ",10\n" +
			"EXP-UNKNOWN,2026-05-31,Unknown," + unknownEmployeeID + "," + expenseAccountID + "," + paymentAccountID + ",10\n",
	})

	require.NoError(t, err)
	assert.Equal(t, 2, result.RowsProcessed)
	assert.Equal(t, 1, result.ExpensesCreated)
	assert.Equal(t, 1, result.RowsSkipped)
	require.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0].Message, `employee_id "`+unknownEmployeeID+`" was not found in employees`)
	expense := repo.expensesByNumber(t, "EXP-EMPLOYEE")
	require.NotNil(t, expense.EmployeeID)
	assert.Equal(t, employeeID, *expense.EmployeeID)
}

func TestParseExpenseImportRowsAndScalarHelpersEdges(t *testing.T) {
	rows, err := parseExpenseImportRows("\ufeffnumber;supplier;notes;date;amount;legacy column\n EXP-1 ; Vendor ; Memo ; 2026-06-01 ; 12,50 ; ignored \n;;;;;\n")
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, 2, rows[0].rowNumber)
	assert.Equal(t, "EXP-1", rows[0].values["expense_number"])
	assert.Equal(t, "Vendor", rows[0].values["merchant"])
	assert.Equal(t, "Memo", rows[0].values["description"])
	assert.Equal(t, "ignored", rows[0].values["legacy_column"])

	_, err = parseExpenseImportRows("  \ufeff  ")
	assert.ErrorContains(t, err, "csv_content is required")

	_, err = parseExpenseImportRows("\"unterminated\n")
	assert.ErrorContains(t, err, "parse csv header")

	_, err = parseExpenseImportRows("expense_number\n\"unterminated\n")
	assert.ErrorContains(t, err, "parse csv row 2")

	assert.Equal(t, "contact_code", canonicalExpenseImportHeader("Customer Code"))
	assert.Equal(t, "_legacy_field_", canonicalExpenseImportHeader(" Legacy Field "))
	assert.Equal(t, '\t', detectExpenseImportDelimiter("expense_number\tmerchant\tamount\n"))

	status, err := parseExpenseImportStatus("Submitted")
	require.NoError(t, err)
	assert.Equal(t, StatusSubmitted, status)
	status, err = parseExpenseImportStatus("")
	require.NoError(t, err)
	assert.Equal(t, StatusDraft, status)
	_, err = parseExpenseImportStatus("POSTED")
	assert.ErrorContains(t, err, "posted expenses must be imported")
	_, err = parseExpenseImportStatus("void")
	assert.ErrorContains(t, err, `invalid status "void"`)

	date, err := parseExpenseImportDate("2026-06-02T15:04:05+03:00", "expense_date")
	require.NoError(t, err)
	assert.Equal(t, time.Date(2026, 6, 2, 0, 0, 0, 0, time.UTC), date)
	_, err = parseExpenseImportDate("", "expense_date")
	assert.ErrorContains(t, err, "expense_date is required")
	_, err = parseExpenseImportDate("not-a-date", "expense_date")
	assert.ErrorContains(t, err, "invalid expense_date")

	fallback := time.Date(2026, 6, 3, 9, 0, 0, 0, time.UTC)
	timestamp, err := parseOptionalExpenseImportDateTime("", fallback)
	require.NoError(t, err)
	assert.Equal(t, fallback, timestamp)
	timestamp, err = parseOptionalExpenseImportDateTime("2026-06-04", fallback)
	require.NoError(t, err)
	assert.Equal(t, time.Date(2026, 6, 4, 0, 0, 0, 0, time.UTC), timestamp)
	timestamp, err = parseOptionalExpenseImportDateTime("2026-06-04T13:30:00+03:00", fallback)
	require.NoError(t, err)
	assert.Equal(t, time.Date(2026, 6, 4, 10, 30, 0, 0, time.UTC), timestamp)
	_, err = parseOptionalExpenseImportDateTime("tomorrow", fallback)
	assert.ErrorContains(t, err, "invalid timestamp")

	amount, err := parseExpenseImportDecimal("12,50", "amount")
	require.NoError(t, err)
	assert.Equal(t, "12.5", amount.String())
	_, err = parseExpenseImportDecimal("", "amount")
	assert.ErrorContains(t, err, "amount is required")
	_, err = parseExpenseImportDecimal("x", "amount")
	assert.ErrorContains(t, err, "invalid amount")

	for _, value := range []string{"true", "T", "yes", "Y", "1"} {
		parsed, err := parseExpenseImportBool(value, "requires_receipt")
		require.NoError(t, err)
		assert.True(t, parsed)
	}
	for _, value := range []string{"false", "F", "no", "N", "0"} {
		parsed, err := parseExpenseImportBool(value, "requires_receipt")
		require.NoError(t, err)
		assert.False(t, parsed)
	}
	_, err = parseExpenseImportBool("maybe", "requires_receipt")
	assert.ErrorContains(t, err, "invalid requires_receipt")
}

func TestApplyExpenseImportStatusMetadataEdges(t *testing.T) {
	now := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)

	draft := &Expense{Status: StatusDraft}
	require.NoError(t, applyExpenseImportStatusMetadata(draft, expenseImportRow{}, "user-1", now))
	assert.Nil(t, draft.SubmittedAt)

	submitted := &Expense{Status: StatusSubmitted}
	require.NoError(t, applyExpenseImportStatusMetadata(submitted, expenseImportRow{
		values: map[string]string{"submitted_at": "2026-06-04T13:30:00+03:00"},
	}, "user-1", now))
	require.NotNil(t, submitted.SubmittedAt)
	assert.Equal(t, time.Date(2026, 6, 4, 10, 30, 0, 0, time.UTC), *submitted.SubmittedAt)
	require.NotNil(t, submitted.SubmittedBy)
	assert.Equal(t, "user-1", *submitted.SubmittedBy)

	err := applyExpenseImportStatusMetadata(&Expense{Status: StatusSubmitted}, expenseImportRow{
		values: map[string]string{"submitted_at": "bad"},
	}, "user-1", now)
	assert.ErrorContains(t, err, "submitted_at: invalid timestamp")

	approved := &Expense{Status: StatusApproved}
	require.NoError(t, applyExpenseImportStatusMetadata(approved, expenseImportRow{
		values: map[string]string{
			"submitted_at": "2026-06-03",
			"approved_at":  "2026-06-04T13:30:00+03:00",
		},
	}, "user-1", now))
	require.NotNil(t, approved.SubmittedAt)
	require.NotNil(t, approved.ApprovedAt)
	assert.Equal(t, time.Date(2026, 6, 3, 0, 0, 0, 0, time.UTC), *approved.SubmittedAt)
	assert.Equal(t, time.Date(2026, 6, 4, 10, 30, 0, 0, time.UTC), *approved.ApprovedAt)
	require.NotNil(t, approved.ApprovedBy)
	assert.Equal(t, "user-1", *approved.ApprovedBy)

	err = applyExpenseImportStatusMetadata(&Expense{Status: StatusApproved}, expenseImportRow{
		values: map[string]string{"approved_at": "bad"},
	}, "user-1", now)
	assert.ErrorContains(t, err, "approved_at: invalid timestamp")

	err = applyExpenseImportStatusMetadata(&Expense{Status: StatusRejected}, expenseImportRow{
		values: map[string]string{},
	}, "user-1", now)
	assert.ErrorContains(t, err, "rejection_reason is required")

	err = applyExpenseImportStatusMetadata(&Expense{Status: StatusRejected}, expenseImportRow{
		values: map[string]string{
			"rejection_reason": "Missing receipt",
			"rejected_at":      "bad",
		},
	}, "user-1", now)
	assert.ErrorContains(t, err, "rejected_at: invalid timestamp")

	rejected := &Expense{Status: StatusRejected}
	require.NoError(t, applyExpenseImportStatusMetadata(rejected, expenseImportRow{
		values: map[string]string{"rejection_reason": " Missing receipt "},
	}, "user-1", now))
	require.NotNil(t, rejected.SubmittedAt)
	require.NotNil(t, rejected.RejectedAt)
	assert.Equal(t, now, *rejected.SubmittedAt)
	assert.Equal(t, now, *rejected.RejectedAt)
	assert.Equal(t, "Missing receipt", rejected.RejectionReason)
}

func TestExpenseImportLookupHelpersEdges(t *testing.T) {
	ctx := context.Background()
	service := NewServiceWithRepository(newMemoryRepository(), nil, nil)

	lookup, err := service.expenseImportContactLookup(ctx, "tenant_acme", "tenant-1", []expenseImportRow{{
		values: map[string]string{"merchant": "Vendor"},
	}})
	require.NoError(t, err)
	assert.Empty(t, lookup)

	_, err = service.expenseImportContactLookup(ctx, "tenant_acme", "tenant-1", []expenseImportRow{{
		values: map[string]string{"contact_code": "SUP-1"},
	}})
	assert.ErrorContains(t, err, "contact service is required")

	service.contacts = &fakeExpenseContactLister{err: assert.AnError}
	_, err = service.expenseImportContactLookup(ctx, "tenant_acme", "tenant-1", []expenseImportRow{{
		values: map[string]string{"contact_name": "Supplier One"},
	}})
	assert.ErrorContains(t, err, "list contacts for expense import")

	employeeIDs, err := service.expenseImportEmployeeIDs(ctx, "tenant_acme", "tenant-1", []expenseImportRow{{
		values: map[string]string{"merchant": "Vendor"},
	}})
	require.NoError(t, err)
	assert.Nil(t, employeeIDs)

	employeeIDs, err = service.expenseImportEmployeeIDs(ctx, "tenant_acme", "tenant-1", []expenseImportRow{{
		values: map[string]string{"employee_id": "11111111-1111-4111-8111-111111111111"},
	}})
	require.NoError(t, err)
	assert.Nil(t, employeeIDs)

	service.employees = &fakeExpenseEmployeeLister{err: assert.AnError}
	_, err = service.expenseImportEmployeeIDs(ctx, "tenant_acme", "tenant-1", []expenseImportRow{{
		values: map[string]string{"employee_id": "11111111-1111-4111-8111-111111111111"},
	}})
	assert.ErrorContains(t, err, "list employees for expense import")

	service.employees = &fakeExpenseEmployeeLister{employees: []payroll.Employee{
		{ID: "11111111-1111-4111-8111-111111111111"},
		{ID: " not-a-uuid "},
		{ID: " "},
	}}
	employeeIDs, err = service.expenseImportEmployeeIDs(ctx, "tenant_acme", "tenant-1", []expenseImportRow{{
		values: map[string]string{"employee_id": "11111111-1111-4111-8111-111111111111"},
	}})
	require.NoError(t, err)
	assert.True(t, employeeIDs["11111111-1111-4111-8111-111111111111"])
	assert.True(t, employeeIDs["not-a-uuid"])

	accountIDs, err := service.expenseImportAccountIDsByCode(ctx, "tenant_acme", "tenant-1", []expenseImportRow{{
		values: map[string]string{"expense_account_id": "99999999-9999-4999-8999-999999999999"},
	}})
	require.NoError(t, err)
	assert.Nil(t, accountIDs)

	_, err = service.expenseImportAccountIDsByCode(ctx, "tenant_acme", "tenant-1", []expenseImportRow{{
		values: map[string]string{"expense_account_code": "5500"},
	}})
	assert.ErrorContains(t, err, "accounting service is required")

	service.accounting = newFakeAccountingPoster()
	accountIDs, err = service.expenseImportAccountIDsByCode(ctx, "tenant_acme", "tenant-1", []expenseImportRow{{
		values: map[string]string{"payment_account_code": "1000"},
	}})
	require.NoError(t, err)
	assert.Equal(t, "cash-account", accountIDs["1000"])

	id, err := resolveExpenseImportAccountID(expenseImportRow{
		values: map[string]string{"expense_account_id": "99999999-9999-4999-8999-999999999999"},
	}, "expense_account_id", "expense_account_code", nil)
	require.NoError(t, err)
	assert.Equal(t, "99999999-9999-4999-8999-999999999999", id)

	_, err = resolveExpenseImportAccountID(expenseImportRow{
		values: map[string]string{},
	}, "expense_account_id", "expense_account_code", nil)
	assert.ErrorContains(t, err, "expense_account_id or expense_account_code is required")

	_, err = resolveExpenseImportAccountID(expenseImportRow{
		values: map[string]string{"expense_account_code": "9999"},
	}, "expense_account_id", "expense_account_code", map[string]string{"5500": "expense-account"})
	assert.ErrorContains(t, err, `account code "9999" was not found`)
}

func TestBuildExpenseFromImportRowEdges(t *testing.T) {
	ctx := context.Background()
	accountIDsByCode := map[string]string{
		"5500": "99999999-9999-4999-8999-999999999999",
		"1000": "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa",
	}
	baseValues := func() map[string]string {
		return map[string]string{
			"expense_date":         "2026-06-01",
			"merchant":             "Vendor",
			"expense_account_code": "5500",
			"payment_account_code": "1000",
			"amount":               "12.50",
			"currency":             "eur",
			"status":               "SUBMITTED",
		}
	}
	build := func(service *Service, values map[string]string, usedNumbers map[string]bool) (*Expense, error) {
		return service.buildExpenseFromImportRow(ctx, "tenant_acme", "tenant-1", "user-1", expenseImportRow{
			rowNumber: 2,
			values:    values,
		}, nil, usedNumbers, accountIDsByCode, contactrefs.ContactLookup{}, nil)
	}

	repo := newMemoryRepository()
	service := NewServiceWithRepository(repo, newFakeAccountingPoster(), nil)
	service.now = fixedExpenseNow

	expense, err := build(service, baseValues(), map[string]bool{})
	require.NoError(t, err)
	assert.Equal(t, "EXP-00001", expense.ExpenseNumber)
	assert.Equal(t, "EUR", expense.Currency)
	assert.True(t, expense.RequiresReceipt)
	assert.Equal(t, StatusSubmitted, expense.Status)
	require.NotNil(t, expense.SubmittedAt)

	values := baseValues()
	values["expense_number"] = "EXP-DUP"
	_, err = build(service, values, map[string]bool{"exp-dup": true})
	assert.ErrorContains(t, err, `duplicate expense_number "EXP-DUP"`)

	values = baseValues()
	values["merchant"] = " "
	_, err = build(service, values, map[string]bool{})
	assert.ErrorContains(t, err, "merchant is required")

	values = baseValues()
	values["amount"] = "0"
	_, err = build(service, values, map[string]bool{})
	assert.ErrorContains(t, err, "amount must be positive")

	values = baseValues()
	values["currency"] = "EURO"
	_, err = build(service, values, map[string]bool{})
	assert.ErrorContains(t, err, "currency must be a 3-letter ISO code")

	values = baseValues()
	values["exchange_rate"] = "0"
	_, err = build(service, values, map[string]bool{})
	assert.ErrorContains(t, err, "exchange_rate must be positive")

	values = baseValues()
	values["requires_receipt"] = "maybe"
	_, err = build(service, values, map[string]bool{})
	assert.ErrorContains(t, err, "invalid requires_receipt")

	generateErrRepo := newMemoryRepository()
	generateErrRepo.generateErr = assert.AnError
	generateErrService := NewServiceWithRepository(generateErrRepo, newFakeAccountingPoster(), nil)
	_, err = build(generateErrService, baseValues(), map[string]bool{})
	assert.ErrorContains(t, err, "generate expense number")
}

func (r *memoryRepository) expensesByNumber(t *testing.T, expenseNumber string) *Expense {
	t.Helper()
	for _, expense := range r.expenses {
		if expense.ExpenseNumber == expenseNumber {
			return expense
		}
	}
	t.Fatalf("expense %s not found", expenseNumber)
	return nil
}
