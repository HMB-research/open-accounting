package auth

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/HMB-research/open-accounting/internal/testutil"
)

func TestSecurityAuditServiceRecordAndListUserEvents(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	userID := testutil.CreateTestUser(t, pool, "security-audit@example.com")
	otherUserID := testutil.CreateTestUser(t, pool, "security-audit-other@example.com")

	now := time.Now().UTC().Truncate(time.Second)
	service := NewSecurityAuditService(pool)
	service.now = func() time.Time { return now }

	ctx := context.Background()
	err := service.RecordEvent(ctx, &SecurityAuditEvent{
		ActorUserID:  userID,
		ActorEmail:   "security-audit@example.com",
		Action:       SecurityAuditActionPasswordChanged,
		TargetUserID: userID,
		TargetEmail:  "security-audit@example.com",
		RequestIP:    "192.0.2.10:1234",
		UserAgent:    "audit-test",
		Metadata: map[string]string{
			"source": "unit",
		},
	})
	require.NoError(t, err)

	err = service.RecordEvent(ctx, &SecurityAuditEvent{
		ActorUserID:  otherUserID,
		Action:       SecurityAuditActionLogin,
		TargetUserID: otherUserID,
	})
	require.NoError(t, err)

	events, err := service.ListUserEvents(ctx, userID, 10)
	require.NoError(t, err)
	require.Len(t, events, 1)
	assert.Equal(t, SecurityAuditActionPasswordChanged, events[0].Action)
	assert.Equal(t, userID, events[0].ActorUserID)
	assert.Equal(t, userID, events[0].TargetUserID)
	assert.Equal(t, "unit", events[0].Metadata["source"])
	assert.True(t, events[0].CreatedAt.Equal(now))
}

func TestSecurityAuditServiceValidatesRequiredFields(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	service := NewSecurityAuditService(pool)

	err := service.RecordEvent(context.Background(), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "event is required")

	err = service.RecordEvent(context.Background(), &SecurityAuditEvent{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "action is required")

	_, err = service.ListUserEvents(context.Background(), "", 10)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "user ID is required")
}
