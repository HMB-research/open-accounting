package documents

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDocumentsWave4RepositoryReviewSummaryNilDatabaseEdges(t *testing.T) {
	repo := NewRepository(nil)
	ctx := context.Background()

	summaries, err := repo.ListReviewSummaries(ctx, "tenant_schema", "tenant-1", EntityTypeInvoice, nil)
	require.NoError(t, err)
	assert.Empty(t, summaries)

	summaries, err = repo.ListReviewSummaries(ctx, "tenant_schema", "tenant-1", EntityTypeInvoice, []string{"invoice-1"})
	require.Error(t, err)
	assert.Nil(t, summaries)
	assert.Contains(t, err.Error(), "documents repository database is not configured")
}

func TestDocumentsWave4ReviewQueueAndPurgeNormalizationEdges(t *testing.T) {
	status, err := normalizeReviewQueueStatus(" all ")
	require.NoError(t, err)
	assert.Empty(t, status)
	assert.Equal(t, "ALL", reviewQueueStatusLabel(status))

	status, err = normalizeReviewQueueStatus("approved")
	require.NoError(t, err)
	assert.Equal(t, ReviewStatusApproved, status)

	_, err = normalizeReviewQueueStatus("waiting")
	require.ErrorContains(t, err, "review_status must be")

	limit, err := normalizeReviewQueueLimit(maxReviewQueueLimit + 50)
	require.NoError(t, err)
	assert.Equal(t, maxReviewQueueLimit, limit)

	_, err = normalizeReviewQueueLimit(-1)
	require.ErrorContains(t, err, "limit must be zero or greater")

	limit, err = normalizePurgeLimit(0)
	require.NoError(t, err)
	assert.Equal(t, defaultPurgeLimit, limit)

	_, err = normalizePurgeLimit(maxPurgeLimit + 1)
	require.ErrorContains(t, err, "limit cannot exceed")
}

func TestDocumentsWave4EvidencePolicyAnyTypeAndInactiveDocuments(t *testing.T) {
	result := evaluateEvidencePolicyForDocuments(EntityTypeExpense, "expense-1", []Document{
		{ID: "receipt-1", DocumentType: DocumentTypeReceipt, ReviewStatus: ReviewStatusApproved, LifecycleStatus: LifecycleStatusArchived},
		{ID: "contract-1", DocumentType: DocumentTypeContract, ReviewStatus: ReviewStatusApproved, LifecycleStatus: LifecycleStatusSuperseded},
	}, []EvidencePolicyRule{
		{MinCount: 1, RequireApproved: true},
		{DocumentTypes: []string{DocumentTypeContract}, MinCount: 1, RequireApproved: true},
	})

	assert.False(t, result.Compliant)
	assert.Equal(t, 1, result.TotalCount)
	require.Len(t, result.RuleResults, 2)
	assert.True(t, result.RuleResults[0].Compliant)
	assert.False(t, result.RuleResults[1].Compliant)
	assert.Contains(t, result.RuleResults[1].Message, "requires at least 1 approved documents")
}

func TestDocumentsWave4LocalStoreRejectsEscapingSaveKey(t *testing.T) {
	store, err := NewLocalStore(t.TempDir())
	require.NoError(t, err)

	err = store.Save(context.Background(), "../escape.txt", strings.NewReader("content"))

	require.ErrorContains(t, err, "invalid document storage key")
}
