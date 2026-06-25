package invoicing

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReminderRuleRepositoryWave9ConstructorPanicsOnGormPoolError(t *testing.T) {
	pool := stubNewGormDBFromPoolError(t, errors.New("pool unavailable"))

	require.PanicsWithError(t, "create reminder rule GORM repository: pool unavailable", func() {
		_ = NewReminderRuleRepository(pool)
	})
}

func TestReminderRuleRepositoryWave9InvalidSchemaForInvoiceRuleQuery(t *testing.T) {
	repo := NewReminderRuleGORMRepository(newInvoicingDryRunDB(t))
	asOfDate := time.Date(2026, time.June, 25, 0, 0, 0, 0, time.UTC)
	rule := &ReminderRule{
		ID:                "rule-1",
		TenantID:          "tenant-1",
		Name:              "After due",
		TriggerType:       TriggerAfterDue,
		DaysOffset:        5,
		EmailTemplateType: "OVERDUE",
		IsActive:          true,
		CreatedAt:         asOfDate,
		UpdatedAt:         asOfDate,
	}

	invoices, err := repo.GetInvoicesForRule(context.Background(), "tenant-invoicing", "tenant-1", rule, asOfDate)

	assert.Nil(t, invoices)
	require.ErrorContains(t, err, "qualify invoices table")
	assert.ErrorContains(t, err, "invalid SQL identifier")
}

func TestReminderRuleRepositoryWave9GetRuleQueryError(t *testing.T) {
	expectedErr := errors.New("rule lookup failed")
	repo := NewReminderRuleGORMRepository(newInvoicingDryRunDB(t, withInvoicingDryRunQueryError(expectedErr)))

	rule, err := repo.GetRule(context.Background(), "tenant_invoicing", "tenant-1", "rule-1")

	assert.Nil(t, rule)
	require.ErrorContains(t, err, "query rule")
	assert.ErrorIs(t, err, expectedErr)
}
