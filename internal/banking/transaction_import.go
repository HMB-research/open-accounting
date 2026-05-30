package banking

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// ImportTransactions imports pre-parsed bank statement transactions.
func (s *Service) ImportTransactions(ctx context.Context, schemaName, tenantID, bankAccountID string, req *ImportCSVRequest) (*ImportResult, error) {
	account, err := s.GetBankAccount(ctx, schemaName, tenantID, bankAccountID)
	if err != nil {
		return nil, fmt.Errorf("get bank account: %w", err)
	}

	result := &ImportResult{
		ImportID: uuid.New().String(),
	}

	for i, row := range req.Transactions {
		transactionDate, err := time.Parse("2006-01-02", row.Date)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("Row %d: invalid date '%s'", i+1, row.Date))
			continue
		}

		var valueDate *time.Time
		if row.ValueDate != "" {
			vd, err := time.Parse("2006-01-02", row.ValueDate)
			if err == nil {
				valueDate = &vd
			}
		}

		amount, err := decimal.NewFromString(row.Amount)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("Row %d: invalid amount '%s'", i+1, row.Amount))
			continue
		}

		if req.SkipDuplicates {
			isDuplicate, err := s.repo.IsTransactionDuplicate(ctx, schemaName, tenantID, bankAccountID, transactionDate, amount, row.ExternalID)
			if err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("Row %d: duplicate check failed: %v", i+1, err))
				continue
			}
			if isDuplicate {
				result.DuplicatesSkipped++
				continue
			}
		}

		transactionID := uuid.New().String()
		if err := s.repo.CreateTransaction(ctx, schemaName, &BankTransaction{
			ID:                  transactionID,
			TenantID:            tenantID,
			BankAccountID:       bankAccountID,
			TransactionDate:     transactionDate,
			ValueDate:           valueDate,
			Amount:              amount,
			Currency:            account.Currency,
			Description:         row.Description,
			Reference:           row.Reference,
			CounterpartyName:    row.CounterpartyName,
			CounterpartyAccount: row.CounterpartyAccount,
			Status:              StatusUnmatched,
			FollowUpStatus:      FollowUpNone,
			ImportedAt:          time.Now(),
			ExternalID:          row.ExternalID,
		}); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("Row %d: insert failed: %v", i+1, err))
			continue
		}

		result.TransactionsImported++
	}

	if err := s.repo.CreateImportRecord(ctx, schemaName, &BankStatementImport{
		ID:                   result.ImportID,
		TenantID:             tenantID,
		BankAccountID:        bankAccountID,
		FileName:             req.FileName,
		TransactionsImported: result.TransactionsImported,
		DuplicatesSkipped:    result.DuplicatesSkipped,
		CreatedAt:            time.Now(),
	}); err != nil {
		return nil, fmt.Errorf("record import: %w", err)
	}

	return result, nil
}
