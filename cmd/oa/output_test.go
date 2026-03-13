package main

import (
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/HMB-research/open-accounting/internal/accounting"
	"github.com/HMB-research/open-accounting/internal/apitoken"
	"github.com/HMB-research/open-accounting/internal/contacts"
	"github.com/HMB-research/open-accounting/internal/documents"
	"github.com/HMB-research/open-accounting/internal/payroll"
)

func TestPrintJSON(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	err := printJSON(&buf, map[string]string{"status": "ok"})
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "\"status\": \"ok\"")
}

func TestPrintTables(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 3, 12, 10, 0, 0, 0, time.UTC)

	var tokenBuf bytes.Buffer
	printAPITokensTable(&tokenBuf, []apitoken.APIToken{{
		ID:          "token-1",
		Name:        "CLI",
		TokenPrefix: "oa_tok",
		CreatedAt:   now,
	}})
	assert.Contains(t, tokenBuf.String(), "ID")
	assert.Contains(t, tokenBuf.String(), "CLI")

	var accountBuf bytes.Buffer
	printAccountsTable(&accountBuf, []accounting.Account{{
		ID:          "account-1",
		Code:        "1000",
		Name:        "Cash",
		AccountType: accounting.AccountTypeAsset,
		IsActive:    true,
	}})
	assert.Contains(t, accountBuf.String(), "CODE")
	assert.Contains(t, accountBuf.String(), "1000")

	var contactBuf bytes.Buffer
	printContactsTable(&contactBuf, []contacts.Contact{{
		ID:          "contact-1",
		Name:        "Acme Corp",
		ContactType: contacts.ContactTypeCustomer,
		Email:       "hello@example.com",
		IsActive:    true,
	}})
	assert.Contains(t, contactBuf.String(), "NAME")
	assert.Contains(t, contactBuf.String(), "Acme Corp")

	var employeeBuf bytes.Buffer
	printEmployeesTable(&employeeBuf, []payroll.Employee{{
		ID:             "employee-1",
		EmployeeNumber: "EMP-001",
		FirstName:      "Mari",
		LastName:       "Maasikas",
		EmploymentType: payroll.EmploymentFullTime,
		Email:          "mari@example.com",
		IsActive:       true,
	}})
	assert.Contains(t, employeeBuf.String(), "NUMBER")
	assert.Contains(t, employeeBuf.String(), "Mari Maasikas")

	var documentBuf bytes.Buffer
	printDocumentsTable(&documentBuf, []documents.Document{{
		ID:           "doc-1",
		EntityType:   documents.EntityTypeBankTxn,
		EntityID:     "txn-1",
		DocumentType: documents.DocumentTypeReconciliation,
		FileName:     "statement.pdf",
		ReviewStatus: documents.ReviewStatusPending,
		CreatedAt:    now,
	}})
	assert.Contains(t, documentBuf.String(), "ENTITY")
	assert.Contains(t, documentBuf.String(), "statement.pdf")
}

func TestFormatHelpers(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "-", formatTimePtr(nil))

	now := time.Date(2026, 3, 12, 10, 0, 0, 0, time.UTC)
	assert.Equal(t, now.Format(time.RFC3339), formatTimePtr(&now))

	assert.Equal(t, "oa_token_12345...", tokenPreview("oa_token_1234567890"))
	assert.Equal(t, "short-token", tokenPreview("short-token"))
	assert.Equal(t, "tenant-slug", normalizeSelector("  Tenant-Slug "))
}
