package expenses

import (
	"context"
	"testing"
	"time"

	"github.com/HMB-research/open-accounting/internal/contacts"
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
