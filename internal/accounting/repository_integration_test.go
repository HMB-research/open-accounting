//go:build integration

package accounting

import (
	"context"
	"testing"
	"time"

	"github.com/HMB-research/open-accounting/internal/testutil"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

func TestPostgresRepository_VoidJournalEntry(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	userID := testutil.CreateTestUser(t, pool, "test@example.com")
	testutil.AddUserToTenant(t, pool, tenant.ID, userID, "admin")
	repo := NewRepository(pool)
	ctx := context.Background()

	// Get account IDs from default chart of accounts
	var cashAccountID, revenueAccountID string
	err := pool.QueryRow(ctx, `
		SELECT id FROM `+tenant.SchemaName+`.accounts WHERE code = '1000' LIMIT 1
	`).Scan(&cashAccountID)
	if err != nil {
		t.Fatalf("failed to get cash account: %v", err)
	}

	err = pool.QueryRow(ctx, `
		SELECT id FROM `+tenant.SchemaName+`.accounts WHERE code = '4000' LIMIT 1
	`).Scan(&revenueAccountID)
	if err != nil {
		t.Fatalf("failed to get revenue account: %v", err)
	}

	// Create a journal entry
	entryID := uuid.New().String()
	now := time.Now()
	amount := decimal.NewFromFloat(100)

	_, err = pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.journal_entries
		(id, tenant_id, entry_number, entry_date, description, status, created_by, created_at, updated_at)
		VALUES ($1, $2, 'JE-001', $3, 'Test entry', 'posted', $4, NOW(), NOW())
	`, entryID, tenant.ID, now, userID)
	if err != nil {
		t.Fatalf("failed to create journal entry: %v", err)
	}

	// Create journal lines
	_, err = pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.journal_lines (id, tenant_id, entry_id, account_id, debit, credit, created_at)
		VALUES ($1, $2, $3, $4, $5, 0, NOW())
	`, uuid.New().String(), tenant.ID, entryID, cashAccountID, amount)
	if err != nil {
		t.Fatalf("failed to create debit line: %v", err)
	}

	_, err = pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.journal_lines (id, tenant_id, entry_id, account_id, debit, credit, created_at)
		VALUES ($1, $2, $3, $4, 0, $5, NOW())
	`, uuid.New().String(), tenant.ID, entryID, revenueAccountID, amount)
	if err != nil {
		t.Fatalf("failed to create credit line: %v", err)
	}

	// Create reversal entry
	reversalID := uuid.New().String()
	reversal := &JournalEntry{
		ID:          reversalID,
		TenantID:    tenant.ID,
		EntryNumber: "JE-002",
		EntryDate:   now,
		Description: "Reversal of JE-001",
		Status:      StatusPosted,
		CreatedBy:   userID,
		Lines: []JournalEntryLine{
			{
				ID:        uuid.New().String(),
				TenantID:  tenant.ID,
				JournalEntryID:   reversalID,
				AccountID: revenueAccountID,
				DebitAmount:     amount,
				CreditAmount:    decimal.Zero,
			},
			{
				ID:        uuid.New().String(),
				TenantID:  tenant.ID,
				JournalEntryID:   reversalID,
				AccountID: cashAccountID,
				DebitAmount:     decimal.Zero,
				CreditAmount:    amount,
			},
		},
	}

	// Void the entry
	err = repo.VoidJournalEntry(ctx, tenant.ID, entryID, userID, "Test void reason", reversal)
	if err != nil {
		t.Fatalf("VoidJournalEntry failed: %v", err)
	}

	// Verify original entry is voided
	var status string
	var voidReason *string
	err = pool.QueryRow(ctx, `
		SELECT status, void_reason FROM `+tenant.SchemaName+`.journal_entries WHERE id = $1
	`, entryID).Scan(&status, &voidReason)
	if err != nil {
		t.Fatalf("failed to query voided entry: %v", err)
	}

	if status != "voided" {
		t.Errorf("expected status 'voided', got '%s'", status)
	}
	if voidReason == nil || *voidReason != "Test void reason" {
		t.Errorf("expected void reason 'Test void reason', got %v", voidReason)
	}

	// Verify reversal entry was created
	var reversalStatus string
	err = pool.QueryRow(ctx, `
		SELECT status FROM `+tenant.SchemaName+`.journal_entries WHERE id = $1
	`, reversalID).Scan(&reversalStatus)
	if err != nil {
		t.Fatalf("failed to query reversal entry: %v", err)
	}

	if reversalStatus != "posted" {
		t.Errorf("expected reversal status 'posted', got '%s'", reversalStatus)
	}
}

func TestPostgresRepository_CreateJournalEntry(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	userID := testutil.CreateTestUser(t, pool, "test@example.com")
	testutil.AddUserToTenant(t, pool, tenant.ID, userID, "admin")
	repo := NewRepository(pool)
	ctx := context.Background()

	// Get account IDs
	var cashAccountID, revenueAccountID string
	err := pool.QueryRow(ctx, `
		SELECT id FROM `+tenant.SchemaName+`.accounts WHERE code = '1000' LIMIT 1
	`).Scan(&cashAccountID)
	if err != nil {
		t.Fatalf("failed to get cash account: %v", err)
	}

	err = pool.QueryRow(ctx, `
		SELECT id FROM `+tenant.SchemaName+`.accounts WHERE code = '4000' LIMIT 1
	`).Scan(&revenueAccountID)
	if err != nil {
		t.Fatalf("failed to get revenue account: %v", err)
	}

	entryID := uuid.New().String()
	amount := decimal.NewFromFloat(500)

	entry := &JournalEntry{
		ID:          entryID,
		TenantID:    tenant.ID,
		EntryNumber: "JE-TEST-001",
		EntryDate:   time.Now(),
		Description: "Test journal entry",
		Status:      StatusDraft,
		CreatedBy:   userID,
		Lines: []JournalEntryLine{
			{
				ID:        uuid.New().String(),
				TenantID:  tenant.ID,
				JournalEntryID:   entryID,
				AccountID: cashAccountID,
				DebitAmount:     amount,
				CreditAmount:    decimal.Zero,
			},
			{
				ID:        uuid.New().String(),
				TenantID:  tenant.ID,
				JournalEntryID:   entryID,
				AccountID: revenueAccountID,
				DebitAmount:     decimal.Zero,
				CreditAmount:    amount,
			},
		},
	}

	err = repo.CreateJournalEntry(ctx, entry)
	if err != nil {
		t.Fatalf("CreateJournalEntry failed: %v", err)
	}

	// Verify entry was created
	retrieved, err := repo.GetJournalEntryByID(ctx, tenant.ID, entryID)
	if err != nil {
		t.Fatalf("GetJournalEntryByID failed: %v", err)
	}

	if retrieved.EntryNumber != entry.EntryNumber {
		t.Errorf("expected entry number %s, got %s", entry.EntryNumber, retrieved.EntryNumber)
	}
	if len(retrieved.Lines) != 2 {
		t.Errorf("expected 2 lines, got %d", len(retrieved.Lines))
	}
}
