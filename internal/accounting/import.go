package accounting

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"strings"
)

type accountImportRow struct {
	rowNumber int
	values    map[string]string
}

type accountImportPending struct {
	row        accountImportRow
	req        *CreateAccountRequest
	parentCode string
}

var accountImportHeaderAliases = map[string]string{
	"code":           "code",
	"account_code":   "code",
	"number":         "code",
	"name":           "name",
	"account_name":   "name",
	"description":    "description",
	"account_type":   "account_type",
	"type":           "account_type",
	"category":       "account_type",
	"parent_code":    "parent_code",
	"parent":         "parent_code",
	"parent_account": "parent_code",
}

var accountImportTypeAliases = map[string]AccountType{
	"asset":       AccountTypeAsset,
	"assets":      AccountTypeAsset,
	"vara":        AccountTypeAsset,
	"liability":   AccountTypeLiability,
	"liabilities": AccountTypeLiability,
	"kohustus":    AccountTypeLiability,
	"equity":      AccountTypeEquity,
	"omakapital":  AccountTypeEquity,
	"revenue":     AccountTypeRevenue,
	"income":      AccountTypeRevenue,
	"tulu":        AccountTypeRevenue,
	"expense":     AccountTypeExpense,
	"expenses":    AccountTypeExpense,
	"kulu":        AccountTypeExpense,
}

// ImportAccountsCSV imports chart-of-account rows from CSV content.
func (s *Service) ImportAccountsCSV(ctx context.Context, schemaName, tenantID string, req *ImportAccountsRequest) (*ImportAccountsResult, error) {
	if strings.TrimSpace(req.CSVContent) == "" {
		return nil, fmt.Errorf("csv_content is required")
	}

	rows, err := parseAccountImportRows(req.CSVContent)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("no accounts found in CSV")
	}

	existingAccounts, err := s.repo.ListAccounts(ctx, schemaName, tenantID, false)
	if err != nil {
		return nil, fmt.Errorf("list existing accounts: %w", err)
	}

	codeToID := make(map[string]string, len(existingAccounts))
	usedCodes := make(map[string]string, len(existingAccounts))
	for _, account := range existingAccounts {
		key := normalizedAccountImportKey(account.Code)
		if key == "" {
			continue
		}
		codeToID[key] = account.ID
		usedCodes[key] = account.Name
	}

	result := &ImportAccountsResult{
		FileName: req.FileName,
		Errors:   []ImportAccountsRowError{},
	}

	pending := make([]accountImportPending, 0, len(rows))
	for _, row := range rows {
		result.RowsProcessed++

		createReq, parentCode, err := buildCreateAccountRequestFromImportRow(row)
		if err != nil {
			result.RowsSkipped++
			result.Errors = append(result.Errors, ImportAccountsRowError{
				Row:     row.rowNumber,
				Code:    strings.TrimSpace(row.values["code"]),
				Name:    strings.TrimSpace(row.values["name"]),
				Message: err.Error(),
			})
			continue
		}

		codeKey := normalizedAccountImportKey(createReq.Code)
		if existingName, exists := usedCodes[codeKey]; exists {
			result.RowsSkipped++
			result.Errors = append(result.Errors, ImportAccountsRowError{
				Row:     row.rowNumber,
				Code:    createReq.Code,
				Name:    createReq.Name,
				Message: fmt.Sprintf("duplicate code %q matches existing account %q", createReq.Code, existingName),
			})
			continue
		}

		usedCodes[codeKey] = createReq.Name
		pending = append(pending, accountImportPending{
			row:        row,
			req:        createReq,
			parentCode: parentCode,
		})
	}

	for len(pending) > 0 {
		progressed := false
		remaining := make([]accountImportPending, 0, len(pending))

		for _, item := range pending {
			if item.parentCode != "" {
				parentID, ok := codeToID[normalizedAccountImportKey(item.parentCode)]
				if !ok {
					remaining = append(remaining, item)
					continue
				}
				item.req.ParentID = &parentID
			}

			account, err := s.CreateAccount(ctx, schemaName, tenantID, item.req)
			if err != nil {
				result.RowsSkipped++
				result.Errors = append(result.Errors, ImportAccountsRowError{
					Row:     item.row.rowNumber,
					Code:    item.req.Code,
					Name:    item.req.Name,
					Message: err.Error(),
				})
				continue
			}

			codeToID[normalizedAccountImportKey(account.Code)] = account.ID
			result.AccountsCreated++
			progressed = true
		}

		if progressed {
			pending = remaining
			continue
		}

		for _, item := range remaining {
			result.RowsSkipped++
			result.Errors = append(result.Errors, ImportAccountsRowError{
				Row:     item.row.rowNumber,
				Code:    item.req.Code,
				Name:    item.req.Name,
				Message: fmt.Sprintf("parent_code %q was not found", item.parentCode),
			})
		}
		break
	}

	if len(result.Errors) == 0 {
		result.Errors = nil
	}

	return result, nil
}

func parseAccountImportRows(content string) ([]accountImportRow, error) {
	trimmed := strings.TrimPrefix(strings.TrimSpace(content), "\ufeff")
	if trimmed == "" {
		return nil, fmt.Errorf("csv_content is required")
	}

	reader := csv.NewReader(strings.NewReader(trimmed))
	reader.Comma = detectAccountImportDelimiter(trimmed)
	reader.FieldsPerRecord = -1
	reader.TrimLeadingSpace = true

	headers, err := reader.Read()
	if err != nil {
		if err == io.EOF {
			return nil, fmt.Errorf("csv file is empty")
		}
		return nil, fmt.Errorf("parse csv header: %w", err)
	}

	canonicalHeaders := make([]string, len(headers))
	hasCodeColumn := false
	hasNameColumn := false
	hasTypeColumn := false
	for i, header := range headers {
		canonicalHeaders[i] = canonicalAccountImportHeader(header)
		switch canonicalHeaders[i] {
		case "code":
			hasCodeColumn = true
		case "name":
			hasNameColumn = true
		case "account_type":
			hasTypeColumn = true
		}
	}

	if !hasCodeColumn || !hasNameColumn || !hasTypeColumn {
		return nil, fmt.Errorf("missing required columns: code, name, account_type")
	}

	rows := make([]accountImportRow, 0)
	rowNumber := 1
	for {
		record, err := reader.Read()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("parse csv row %d: %w", rowNumber+1, err)
		}

		rowNumber++
		values := make(map[string]string, len(canonicalHeaders))
		isBlank := true
		for i, header := range canonicalHeaders {
			if header == "" {
				continue
			}

			value := ""
			if i < len(record) {
				value = strings.TrimSpace(record[i])
			}
			if value != "" {
				isBlank = false
			}
			values[header] = value
		}

		if isBlank {
			continue
		}

		rows = append(rows, accountImportRow{
			rowNumber: rowNumber,
			values:    values,
		})
	}

	return rows, nil
}

func buildCreateAccountRequestFromImportRow(row accountImportRow) (*CreateAccountRequest, string, error) {
	code := strings.TrimSpace(row.values["code"])
	if code == "" {
		return nil, "", fmt.Errorf("code is required")
	}

	name := strings.TrimSpace(row.values["name"])
	if name == "" {
		return nil, "", fmt.Errorf("name is required")
	}

	accountType, err := parseAccountImportType(row.values["account_type"])
	if err != nil {
		return nil, "", err
	}

	parentCode := strings.TrimSpace(row.values["parent_code"])
	if parentCode != "" && normalizedAccountImportKey(parentCode) == normalizedAccountImportKey(code) {
		return nil, "", fmt.Errorf("parent_code cannot match code")
	}

	return &CreateAccountRequest{
		Code:        code,
		Name:        name,
		AccountType: accountType,
		Description: strings.TrimSpace(row.values["description"]),
	}, parentCode, nil
}

func parseAccountImportType(value string) (AccountType, error) {
	normalized := normalizedAccountImportKey(value)
	if normalized == "" {
		return "", fmt.Errorf("account_type is required")
	}

	if accountType, ok := accountImportTypeAliases[normalized]; ok {
		return accountType, nil
	}

	switch AccountType(strings.ToUpper(strings.TrimSpace(value))) {
	case AccountTypeAsset, AccountTypeLiability, AccountTypeEquity, AccountTypeRevenue, AccountTypeExpense:
		return AccountType(strings.ToUpper(strings.TrimSpace(value))), nil
	default:
		return "", fmt.Errorf("invalid account_type %q", value)
	}
}

func detectAccountImportDelimiter(content string) rune {
	firstLine := content
	if idx := strings.IndexByte(content, '\n'); idx >= 0 {
		firstLine = content[:idx]
	}

	bestDelimiter := ','
	bestCount := strings.Count(firstLine, string(bestDelimiter))
	for _, delimiter := range []rune{';', '\t'} {
		count := strings.Count(firstLine, string(delimiter))
		if count > bestCount {
			bestDelimiter = delimiter
			bestCount = count
		}
	}
	return bestDelimiter
}

func canonicalAccountImportHeader(header string) string {
	normalized := normalizedAccountImportHeader(header)
	if canonical, ok := accountImportHeaderAliases[normalized]; ok {
		return canonical
	}
	return normalized
}

func normalizedAccountImportHeader(header string) string {
	normalized := strings.TrimSpace(strings.ToLower(header))
	replacer := strings.NewReplacer(" ", "_", "-", "_", "/", "_", ".", "_")
	return replacer.Replace(normalized)
}

func normalizedAccountImportKey(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}
