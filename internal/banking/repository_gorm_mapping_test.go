package banking

import (
	"testing"
	"time"

	"github.com/HMB-research/open-accounting/internal/models"
	"github.com/shopspring/decimal"
)

func TestBankAccountModelMappings(t *testing.T) {
	createdAt := time.Date(2026, 6, 1, 10, 30, 0, 0, time.UTC)
	glAccountID := "gl-account-1"

	model := &models.BankAccount{
		ID:            "bank-account-1",
		TenantID:      "tenant-1",
		Name:          "Operating account",
		AccountNumber: "EE471000001020145685",
		BankName:      "Demo Bank",
		SwiftCode:     "DEMOEE2X",
		Currency:      "EUR",
		GLAccountID:   &glAccountID,
		IsDefault:     true,
		IsActive:      true,
		CreatedAt:     createdAt,
	}

	account := modelToBankAccount(model)

	if account.ID != model.ID ||
		account.TenantID != model.TenantID ||
		account.Name != model.Name ||
		account.AccountNumber != model.AccountNumber ||
		account.BankName != model.BankName ||
		account.SwiftCode != model.SwiftCode ||
		account.Currency != model.Currency ||
		account.GLAccountID != model.GLAccountID ||
		account.IsDefault != model.IsDefault ||
		account.IsActive != model.IsActive ||
		!account.CreatedAt.Equal(model.CreatedAt) {
		t.Fatalf("modelToBankAccount() = %#v, want fields from %#v", account, model)
	}

	roundTrip := bankAccountToModel(account)

	if roundTrip.ID != account.ID ||
		roundTrip.TenantID != account.TenantID ||
		roundTrip.Name != account.Name ||
		roundTrip.AccountNumber != account.AccountNumber ||
		roundTrip.BankName != account.BankName ||
		roundTrip.SwiftCode != account.SwiftCode ||
		roundTrip.Currency != account.Currency ||
		roundTrip.GLAccountID != account.GLAccountID ||
		roundTrip.IsDefault != account.IsDefault ||
		roundTrip.IsActive != account.IsActive ||
		!roundTrip.CreatedAt.Equal(account.CreatedAt) {
		t.Fatalf("bankAccountToModel() = %#v, want fields from %#v", roundTrip, account)
	}
}

func TestBankTransactionModelMappings(t *testing.T) {
	transactionDate := time.Date(2026, 6, 2, 0, 0, 0, 0, time.UTC)
	valueDate := time.Date(2026, 6, 3, 0, 0, 0, 0, time.UTC)
	reviewedAt := time.Date(2026, 6, 4, 9, 0, 0, 0, time.UTC)
	importedAt := time.Date(2026, 6, 4, 12, 0, 0, 0, time.UTC)
	reviewedBy := "user-1"
	paymentID := "payment-1"
	journalEntryID := "journal-entry-1"
	reconciliationID := "reconciliation-1"

	model := &models.BankTransaction{
		ID:                  "transaction-1",
		TenantID:            "tenant-1",
		BankAccountID:       "bank-account-1",
		TransactionDate:     transactionDate,
		ValueDate:           &valueDate,
		Amount:              models.Decimal{Decimal: decimal.NewFromInt(12550)},
		Currency:            "EUR",
		Description:         "Invoice payment",
		Reference:           "RF18539007547034",
		CounterpartyName:    "Acme OU",
		CounterpartyAccount: "EE111111111111111111",
		Status:              models.TransactionStatusMatched,
		FollowUpStatus:      models.TransactionFollowUpEvidenceRequired,
		ReviewNote:          "Receipt requested",
		ReviewedBy:          &reviewedBy,
		ReviewedAt:          &reviewedAt,
		MatchedPaymentID:    &paymentID,
		JournalEntryID:      &journalEntryID,
		ReconciliationID:    &reconciliationID,
		ImportedAt:          importedAt,
		ExternalID:          "external-1",
	}

	transaction := modelToBankTransaction(model)

	if transaction.ID != model.ID ||
		transaction.TenantID != model.TenantID ||
		transaction.BankAccountID != model.BankAccountID ||
		!transaction.TransactionDate.Equal(model.TransactionDate) ||
		transaction.ValueDate != model.ValueDate ||
		!transaction.Amount.Equal(model.Amount.Decimal) ||
		transaction.Currency != model.Currency ||
		transaction.Description != model.Description ||
		transaction.Reference != model.Reference ||
		transaction.CounterpartyName != model.CounterpartyName ||
		transaction.CounterpartyAccount != model.CounterpartyAccount ||
		transaction.Status != StatusMatched ||
		transaction.FollowUpStatus != FollowUpEvidenceRequired ||
		transaction.ReviewNote != model.ReviewNote ||
		transaction.ReviewedBy != model.ReviewedBy ||
		transaction.ReviewedAt != model.ReviewedAt ||
		transaction.MatchedPaymentID != model.MatchedPaymentID ||
		transaction.JournalEntryID != model.JournalEntryID ||
		transaction.ReconciliationID != model.ReconciliationID ||
		!transaction.ImportedAt.Equal(model.ImportedAt) ||
		transaction.ExternalID != model.ExternalID {
		t.Fatalf("modelToBankTransaction() = %#v, want fields from %#v", transaction, model)
	}

	roundTrip := bankTransactionToModel(transaction)

	if roundTrip.ID != transaction.ID ||
		roundTrip.TenantID != transaction.TenantID ||
		roundTrip.BankAccountID != transaction.BankAccountID ||
		!roundTrip.TransactionDate.Equal(transaction.TransactionDate) ||
		roundTrip.ValueDate != transaction.ValueDate ||
		!roundTrip.Amount.Decimal.Equal(transaction.Amount) ||
		roundTrip.Currency != transaction.Currency ||
		roundTrip.Description != transaction.Description ||
		roundTrip.Reference != transaction.Reference ||
		roundTrip.CounterpartyName != transaction.CounterpartyName ||
		roundTrip.CounterpartyAccount != transaction.CounterpartyAccount ||
		roundTrip.Status != models.TransactionStatusMatched ||
		roundTrip.FollowUpStatus != models.TransactionFollowUpEvidenceRequired ||
		roundTrip.ReviewNote != transaction.ReviewNote ||
		roundTrip.ReviewedBy != transaction.ReviewedBy ||
		roundTrip.ReviewedAt != transaction.ReviewedAt ||
		roundTrip.MatchedPaymentID != transaction.MatchedPaymentID ||
		roundTrip.JournalEntryID != transaction.JournalEntryID ||
		roundTrip.ReconciliationID != transaction.ReconciliationID ||
		!roundTrip.ImportedAt.Equal(transaction.ImportedAt) ||
		roundTrip.ExternalID != transaction.ExternalID {
		t.Fatalf("bankTransactionToModel() = %#v, want fields from %#v", roundTrip, transaction)
	}
}

func TestBankTransactionModelMappingsDefaultEmptyFollowUpStatus(t *testing.T) {
	transaction := modelToBankTransaction(&models.BankTransaction{})
	if transaction.FollowUpStatus != FollowUpNone {
		t.Fatalf("modelToBankTransaction() FollowUpStatus = %q, want %q", transaction.FollowUpStatus, FollowUpNone)
	}

	model := bankTransactionToModel(&BankTransaction{})
	if model.FollowUpStatus != models.TransactionFollowUpNone {
		t.Fatalf("bankTransactionToModel() FollowUpStatus = %q, want %q", model.FollowUpStatus, models.TransactionFollowUpNone)
	}
}

func TestBankReconciliationModelMappings(t *testing.T) {
	statementDate := time.Date(2026, 5, 31, 0, 0, 0, 0, time.UTC)
	completedAt := time.Date(2026, 6, 1, 11, 0, 0, 0, time.UTC)
	createdAt := time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC)

	model := &models.BankReconciliation{
		ID:             "reconciliation-1",
		TenantID:       "tenant-1",
		BankAccountID:  "bank-account-1",
		StatementDate:  statementDate,
		OpeningBalance: models.Decimal{Decimal: decimal.NewFromInt(1000)},
		ClosingBalance: models.Decimal{Decimal: decimal.NewFromInt(1250)},
		Status:         models.ReconciliationCompleted,
		CompletedAt:    &completedAt,
		CreatedAt:      createdAt,
		CreatedBy:      "user-1",
	}

	reconciliation := modelToBankReconciliation(model)

	if reconciliation.ID != model.ID ||
		reconciliation.TenantID != model.TenantID ||
		reconciliation.BankAccountID != model.BankAccountID ||
		!reconciliation.StatementDate.Equal(model.StatementDate) ||
		!reconciliation.OpeningBalance.Equal(model.OpeningBalance.Decimal) ||
		!reconciliation.ClosingBalance.Equal(model.ClosingBalance.Decimal) ||
		reconciliation.Status != ReconciliationCompleted ||
		reconciliation.CompletedAt != model.CompletedAt ||
		!reconciliation.CreatedAt.Equal(model.CreatedAt) ||
		reconciliation.CreatedBy != model.CreatedBy {
		t.Fatalf("modelToBankReconciliation() = %#v, want fields from %#v", reconciliation, model)
	}

	roundTrip := bankReconciliationToModel(reconciliation)

	if roundTrip.ID != reconciliation.ID ||
		roundTrip.TenantID != reconciliation.TenantID ||
		roundTrip.BankAccountID != reconciliation.BankAccountID ||
		!roundTrip.StatementDate.Equal(reconciliation.StatementDate) ||
		!roundTrip.OpeningBalance.Decimal.Equal(reconciliation.OpeningBalance) ||
		!roundTrip.ClosingBalance.Decimal.Equal(reconciliation.ClosingBalance) ||
		roundTrip.Status != models.ReconciliationCompleted ||
		roundTrip.CompletedAt != reconciliation.CompletedAt ||
		!roundTrip.CreatedAt.Equal(reconciliation.CreatedAt) ||
		roundTrip.CreatedBy != reconciliation.CreatedBy {
		t.Fatalf("bankReconciliationToModel() = %#v, want fields from %#v", roundTrip, reconciliation)
	}
}

func TestBankStatementImportModelMappings(t *testing.T) {
	createdAt := time.Date(2026, 6, 5, 8, 0, 0, 0, time.UTC)

	model := &models.BankStatementImport{
		ID:                   "import-1",
		TenantID:             "tenant-1",
		BankAccountID:        "bank-account-1",
		FileName:             "statement.csv",
		TransactionsImported: 42,
		TransactionsMatched:  31,
		DuplicatesSkipped:    3,
		CreatedAt:            createdAt,
	}

	statementImport := modelToBankStatementImport(model)

	if statementImport.ID != model.ID ||
		statementImport.TenantID != model.TenantID ||
		statementImport.BankAccountID != model.BankAccountID ||
		statementImport.FileName != model.FileName ||
		statementImport.TransactionsImported != model.TransactionsImported ||
		statementImport.TransactionsMatched != model.TransactionsMatched ||
		statementImport.DuplicatesSkipped != model.DuplicatesSkipped ||
		!statementImport.CreatedAt.Equal(model.CreatedAt) {
		t.Fatalf("modelToBankStatementImport() = %#v, want fields from %#v", statementImport, model)
	}

	roundTrip := bankStatementImportToModel(statementImport)

	if roundTrip.ID != statementImport.ID ||
		roundTrip.TenantID != statementImport.TenantID ||
		roundTrip.BankAccountID != statementImport.BankAccountID ||
		roundTrip.FileName != statementImport.FileName ||
		roundTrip.TransactionsImported != statementImport.TransactionsImported ||
		roundTrip.TransactionsMatched != statementImport.TransactionsMatched ||
		roundTrip.DuplicatesSkipped != statementImport.DuplicatesSkipped ||
		!roundTrip.CreatedAt.Equal(statementImport.CreatedAt) {
		t.Fatalf("bankStatementImportToModel() = %#v, want fields from %#v", roundTrip, statementImport)
	}
}
