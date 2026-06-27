package documents

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestDocumentsWave9RetentionFormattingNilValues(t *testing.T) {
	require.Empty(t, formatRetentionReminderDays(nil))
	require.Empty(t, formatRetentionReminderDate(nil))
}

func TestDocumentsWave9RetentionFormattingValues(t *testing.T) {
	days := 12
	retentionDate := time.Date(2026, 7, 8, 16, 30, 0, 0, time.FixedZone("EET", 2*60*60))

	require.Equal(t, "12", formatRetentionReminderDays(&days))
	require.Equal(t, "2026-07-08", formatRetentionReminderDate(&retentionDate))
}

func TestDocumentsWave9NormalizeReviewQueueStatusPendingLowercase(t *testing.T) {
	status, err := normalizeReviewQueueStatus(" pending ")

	require.NoError(t, err)
	require.Equal(t, ReviewStatusPending, status)
}
