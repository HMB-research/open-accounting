package generic

import (
	"fmt"

	"github.com/HMB-research/open-accounting/internal/banking"
	"github.com/HMB-research/open-accounting/internal/banking/mappers"
)

// ParseTransactions parses the existing simple bank transaction CSV layout.
func ParseTransactions(content string) ([]banking.CSVTransactionRow, error) {
	parsed, err := mappers.ParseCSV(content, "bank transaction")
	if err != nil {
		return nil, err
	}

	var rows []banking.CSVTransactionRow
	for i, record := range parsed.Rows {
		rowNum := i + 2
		row := banking.CSVTransactionRow{
			Date:                mappers.Field(record, parsed.Index, "date", "transaction_date"),
			ValueDate:           mappers.Field(record, parsed.Index, "value_date"),
			Amount:              mappers.Field(record, parsed.Index, "amount", "sum"),
			Currency:            mappers.Field(record, parsed.Index, "currency"),
			SourceAccount:       mappers.Field(record, parsed.Index, "source_account", "client_account", "account_number", "bank_account"),
			Description:         mappers.Field(record, parsed.Index, "description", "details", "selgitus"),
			Reference:           mappers.Field(record, parsed.Index, "reference", "payment_reference"),
			CounterpartyName:    mappers.Field(record, parsed.Index, "counterparty_name", "counterparty", "name"),
			CounterpartyAccount: mappers.Field(record, parsed.Index, "counterparty_account", "counterparty_iban", "iban"),
			ExternalID:          mappers.Field(record, parsed.Index, "external_id", "id"),
		}
		if row.Date == "" || row.Amount == "" || row.Description == "" {
			return nil, fmt.Errorf("bank transaction CSV row %d requires date, amount, and description", rowNum)
		}
		rows = append(rows, row)
	}

	if len(rows) == 0 {
		return nil, fmt.Errorf("bank transaction CSV contains no transactions")
	}
	return rows, nil
}
