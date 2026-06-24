package banking

import (
	"context"
	"testing"
	"time"

	"github.com/HMB-research/open-accounting/internal/models"
	"github.com/HMB-research/open-accounting/internal/payments"
	"github.com/shopspring/decimal"
)

func TestGORMRepositoryNilDatabase(t *testing.T) {
	repo := NewGORMRepository(nil)
	ctx := context.Background()
	schemaName := "tenant_schema"
	tenantID := "tenant-1"
	now := time.Date(2026, 6, 24, 12, 0, 0, 0, time.UTC)
	followUp := FollowUpEvidenceRequired
	reviewNote := "Needs evidence"

	if repo == nil {
		t.Fatal("NewGORMRepository(nil) returned nil")
	}
	if repo.db != nil {
		t.Fatalf("NewGORMRepository(nil).db = %#v, want nil", repo.db)
	}

	tests := []struct {
		name string
		run  func(t *testing.T) error
	}{
		{
			name: "tenantTable",
			run: func(t *testing.T) error {
				table, err := repo.tenantTable(ctx, schemaName, "bank_accounts")
				if table != nil {
					t.Fatalf("tenantTable() table = %#v, want nil", table)
				}
				return err
			},
		},
		{
			name: "CreateBankAccount",
			run: func(t *testing.T) error {
				return repo.CreateBankAccount(ctx, schemaName, &BankAccount{TenantID: tenantID})
			},
		},
		{
			name: "GetBankAccount",
			run: func(t *testing.T) error {
				account, err := repo.GetBankAccount(ctx, schemaName, tenantID, "account-1")
				if account != nil {
					t.Fatalf("GetBankAccount() account = %#v, want nil", account)
				}
				return err
			},
		},
		{
			name: "ListBankAccounts",
			run: func(t *testing.T) error {
				active := true
				accounts, err := repo.ListBankAccounts(ctx, schemaName, tenantID, &BankAccountFilter{
					IsActive: &active,
					Currency: "EUR",
				})
				if accounts != nil {
					t.Fatalf("ListBankAccounts() accounts = %#v, want nil", accounts)
				}
				return err
			},
		},
		{
			name: "UpdateBankAccount",
			run: func(t *testing.T) error {
				return repo.UpdateBankAccount(ctx, schemaName, &BankAccount{ID: "account-1", TenantID: tenantID})
			},
		},
		{
			name: "DeleteBankAccount",
			run: func(t *testing.T) error {
				return repo.DeleteBankAccount(ctx, schemaName, tenantID, "account-1")
			},
		},
		{
			name: "UnsetDefaultAccounts",
			run: func(t *testing.T) error {
				return repo.UnsetDefaultAccounts(ctx, schemaName, tenantID)
			},
		},
		{
			name: "CountTransactionsForAccount",
			run: func(t *testing.T) error {
				count, err := repo.CountTransactionsForAccount(ctx, schemaName, "account-1")
				if count != 0 {
					t.Fatalf("CountTransactionsForAccount() count = %d, want 0", count)
				}
				return err
			},
		},
		{
			name: "CalculateAccountBalance",
			run: func(t *testing.T) error {
				balance, err := repo.CalculateAccountBalance(ctx, schemaName, "account-1")
				if !balance.IsZero() {
					t.Fatalf("CalculateAccountBalance() balance = %s, want zero", balance)
				}
				return err
			},
		},
		{
			name: "CreateBankMatchRule",
			run: func(t *testing.T) error {
				return repo.CreateBankMatchRule(ctx, schemaName, &BankMatchRule{TenantID: tenantID})
			},
		},
		{
			name: "GetBankMatchRule",
			run: func(t *testing.T) error {
				rule, err := repo.GetBankMatchRule(ctx, schemaName, tenantID, "rule-1")
				if rule != nil {
					t.Fatalf("GetBankMatchRule() rule = %#v, want nil", rule)
				}
				return err
			},
		},
		{
			name: "ListBankMatchRules",
			run: func(t *testing.T) error {
				rules, err := repo.ListBankMatchRules(ctx, schemaName, tenantID, &BankMatchRuleFilter{
					ActiveOnly:    true,
					BankAccountID: "account-1",
					IncludeGlobal: true,
				})
				if rules != nil {
					t.Fatalf("ListBankMatchRules() rules = %#v, want nil", rules)
				}
				return err
			},
		},
		{
			name: "UpdateBankMatchRule",
			run: func(t *testing.T) error {
				return repo.UpdateBankMatchRule(ctx, schemaName, &BankMatchRule{ID: "rule-1", TenantID: tenantID})
			},
		},
		{
			name: "DeleteBankMatchRule",
			run: func(t *testing.T) error {
				return repo.DeleteBankMatchRule(ctx, schemaName, tenantID, "rule-1")
			},
		},
		{
			name: "ListTransactions",
			run: func(t *testing.T) error {
				minAmount := decimal.NewFromInt(10)
				maxAmount := decimal.NewFromInt(100)
				transactions, err := repo.ListTransactions(ctx, schemaName, tenantID, &TransactionFilter{
					BankAccountID: "account-1",
					Status:        StatusUnmatched,
					FromDate:      &now,
					ToDate:        &now,
					MinAmount:     &minAmount,
					MaxAmount:     &maxAmount,
				})
				if transactions != nil {
					t.Fatalf("ListTransactions() transactions = %#v, want nil", transactions)
				}
				return err
			},
		},
		{
			name: "GetTransaction",
			run: func(t *testing.T) error {
				transaction, err := repo.GetTransaction(ctx, schemaName, tenantID, "transaction-1")
				if transaction != nil {
					t.Fatalf("GetTransaction() transaction = %#v, want nil", transaction)
				}
				return err
			},
		},
		{
			name: "ListPaymentMatchCandidates",
			run: func(t *testing.T) error {
				candidates, err := repo.ListPaymentMatchCandidates(ctx, schemaName, tenantID, payments.PaymentTypeReceived, decimal.NewFromInt(25), 5)
				if candidates != nil {
					t.Fatalf("ListPaymentMatchCandidates() candidates = %#v, want nil", candidates)
				}
				return err
			},
		},
		{
			name: "MatchTransaction",
			run: func(t *testing.T) error {
				return repo.MatchTransaction(ctx, schemaName, tenantID, "transaction-1", "payment-1")
			},
		},
		{
			name: "UnmatchTransaction",
			run: func(t *testing.T) error {
				return repo.UnmatchTransaction(ctx, schemaName, tenantID, "transaction-1")
			},
		},
		{
			name: "UpdateTransactionReview",
			run: func(t *testing.T) error {
				transaction, err := repo.UpdateTransactionReview(ctx, schemaName, tenantID, "transaction-1", TransactionReviewUpdate{
					FollowUpStatus: &followUp,
					ReviewNote:     &reviewNote,
					ReviewedBy:     "user-1",
					ReviewedAt:     now,
				})
				if transaction != nil {
					t.Fatalf("UpdateTransactionReview() transaction = %#v, want nil", transaction)
				}
				return err
			},
		},
		{
			name: "CreateTransaction",
			run: func(t *testing.T) error {
				return repo.CreateTransaction(ctx, schemaName, &BankTransaction{TenantID: tenantID})
			},
		},
		{
			name: "CreatePaymentFromTransaction",
			run: func(t *testing.T) error {
				paymentID, err := repo.CreatePaymentFromTransaction(ctx, schemaName, tenantID, "user-1", &BankTransaction{
					ID:              "transaction-1",
					TenantID:        tenantID,
					Status:          StatusUnmatched,
					Amount:          decimal.NewFromInt(25),
					Currency:        "EUR",
					TransactionDate: now,
				})
				if paymentID != "" {
					t.Fatalf("CreatePaymentFromTransaction() paymentID = %q, want empty", paymentID)
				}
				return err
			},
		},
		{
			name: "IsTransactionDuplicate",
			run: func(t *testing.T) error {
				duplicate, err := repo.IsTransactionDuplicate(ctx, schemaName, tenantID, "account-1", now, decimal.NewFromInt(25), "external-1")
				if duplicate {
					t.Fatal("IsTransactionDuplicate() duplicate = true, want false")
				}
				return err
			},
		},
		{
			name: "CreateReconciliation",
			run: func(t *testing.T) error {
				return repo.CreateReconciliation(ctx, schemaName, &BankReconciliation{TenantID: tenantID})
			},
		},
		{
			name: "GetReconciliation",
			run: func(t *testing.T) error {
				reconciliation, err := repo.GetReconciliation(ctx, schemaName, tenantID, "reconciliation-1")
				if reconciliation != nil {
					t.Fatalf("GetReconciliation() reconciliation = %#v, want nil", reconciliation)
				}
				return err
			},
		},
		{
			name: "ListReconciliations",
			run: func(t *testing.T) error {
				reconciliations, err := repo.ListReconciliations(ctx, schemaName, tenantID, "account-1")
				if reconciliations != nil {
					t.Fatalf("ListReconciliations() reconciliations = %#v, want nil", reconciliations)
				}
				return err
			},
		},
		{
			name: "CompleteReconciliation",
			run: func(t *testing.T) error {
				return repo.CompleteReconciliation(ctx, schemaName, tenantID, "reconciliation-1")
			},
		},
		{
			name: "AddTransactionToReconciliation",
			run: func(t *testing.T) error {
				return repo.AddTransactionToReconciliation(ctx, schemaName, tenantID, "transaction-1", "reconciliation-1")
			},
		},
		{
			name: "CreateImportRecord",
			run: func(t *testing.T) error {
				return repo.CreateImportRecord(ctx, schemaName, &BankStatementImport{TenantID: tenantID})
			},
		},
		{
			name: "IncrementLatestImportMatchedCount",
			run: func(t *testing.T) error {
				return repo.IncrementLatestImportMatchedCount(ctx, schemaName, tenantID, "account-1", 1)
			},
		},
		{
			name: "GetImportHistory",
			run: func(t *testing.T) error {
				imports, err := repo.GetImportHistory(ctx, schemaName, tenantID, "account-1")
				if imports != nil {
					t.Fatalf("GetImportHistory() imports = %#v, want nil", imports)
				}
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.run(t)
			if err == nil {
				t.Fatal("expected error")
			}
			if got := err.Error(); got != "banking repository database is not configured" {
				t.Fatalf("error = %q, want banking repository database is not configured", got)
			}
		})
	}

	if err := repo.IncrementLatestImportMatchedCount(ctx, schemaName, tenantID, "account-1", 0); err != nil {
		t.Fatalf("IncrementLatestImportMatchedCount zero matched count error = %v, want nil", err)
	}
}

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
