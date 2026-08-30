package smartaccountssync

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBrowserOnboardingBatchRepositoryRecordRoundTripRejectsTamperedManifest(t *testing.T) {
	now := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
	batch, err := newBrowserOnboardingBatch("3cecf7fe-922a-4699-9c87-2d031e7011f7", "owner-1", BrowserOnboardingBatchRequest{CatalogReceiptID: batchCatalogID, Mode: BrowserOnboardingBatchModeSelected, SelectedSourceIDs: []string{batchSourceOne}, OwnerConfirmed: true}, testBrowserOnboardingCatalogReceipt("owner-1", batchCatalogID, []BrowserOnboardingSource{{SourceCompanyID: batchSourceTwo, SourceCompanyName: "Second"}, {SourceCompanyID: batchSourceOne, SourceCompanyName: "First"}}), now)
	require.NoError(t, err)
	record, err := browserOnboardingBatchToRecord(batch)
	require.NoError(t, err)
	restored, err := browserOnboardingBatchFromRecord(record)
	require.NoError(t, err)
	assert.True(t, sameBrowserOnboardingBatchManifest(batch, *restored))

	record.ManifestSHA256 = strings.Repeat("0", 64)
	_, err = browserOnboardingBatchFromRecord(record)
	assert.ErrorIs(t, err, ErrBrowserOnboardingBatchUnavailable)
}

func TestBrowserOnboardingBatchMigrationContainsOnlyControlMetadata(t *testing.T) {
	upgrade, err := os.ReadFile(filepath.Join("..", "..", "migrations", "081_smartaccounts_browser_onboarding_batches.up.sql"))
	require.NoError(t, err)
	text := string(upgrade)
	for _, expected := range []string{
		"CREATE TABLE IF NOT EXISTS public.smartaccounts_browser_onboarding_batches",
		"CREATE TABLE IF NOT EXISTS public.smartaccounts_browser_onboarding_batch_outcomes",
		"mode IN ('selected', 'all')",
		"status IN ('PENDING', 'REVIEW_REQUIRED', 'READY', 'COMPLETE')",
		"uq_smartaccounts_browser_onboarding_batch_owner_receipt",
		"CREATE TABLE IF NOT EXISTS public.smartaccounts_browser_onboarding_catalog_receipts",
		fmt.Sprintf("BETWEEN 1 AND %d", BrowserOnboardingMaxSources),
	} {
		assert.Contains(t, text, expected)
	}
	for _, forbidden := range []string{"api_key", "api_secret", "secret_reference", "catalog_token", "capture_token", "journal_entries"} {
		assert.NotContains(t, strings.ToLower(text), forbidden)
	}

	rollback, err := os.ReadFile(filepath.Join("..", "..", "migrations", "081_smartaccounts_browser_onboarding_batches.down.sql"))
	require.NoError(t, err)
	assert.Contains(t, string(rollback), "DROP TABLE IF EXISTS public.smartaccounts_browser_onboarding_batch_outcomes")
	assert.Contains(t, string(rollback), "DROP TABLE IF EXISTS public.smartaccounts_browser_onboarding_batches")
}
