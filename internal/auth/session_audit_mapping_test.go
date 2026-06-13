package auth

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/HMB-research/open-accounting/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSecurityAuditEventModelMapping(t *testing.T) {
	createdAt := time.Date(2026, 6, 12, 12, 30, 0, 0, time.UTC)
	metadata := json.RawMessage(`{"reason":"manual review"}`)

	event := &SecurityAuditEvent{
		ID:           "event-1",
		ActorUserID:  " actor-1 ",
		ActorEmail:   " actor@example.com ",
		Action:       SecurityAuditActionSessionRevoked,
		TargetUserID: "\ttarget-1\n",
		TargetEmail:  "",
		RequestIP:    " 192.0.2.10 ",
		UserAgent:    " ",
		CreatedAt:    createdAt,
	}

	model := securityAuditEventToModel(event, metadata)

	assert.Equal(t, "event-1", model.ID)
	require.NotNil(t, model.ActorUserID)
	assert.Equal(t, "actor-1", *model.ActorUserID)
	require.NotNil(t, model.ActorEmail)
	assert.Equal(t, "actor@example.com", *model.ActorEmail)
	require.NotNil(t, model.TargetUserID)
	assert.Equal(t, "target-1", *model.TargetUserID)
	assert.Nil(t, model.TargetEmail)
	require.NotNil(t, model.RequestIP)
	assert.Equal(t, "192.0.2.10", *model.RequestIP)
	assert.Nil(t, model.UserAgent)
	assert.Equal(t, SecurityAuditActionSessionRevoked, model.Action)
	assert.JSONEq(t, `{"reason":"manual review"}`, string(model.Metadata))
	assert.Equal(t, createdAt, model.CreatedAt)

	roundTrip := modelToSecurityAuditEvent(model)

	assert.Equal(t, event.ID, roundTrip.ID)
	assert.Equal(t, "actor-1", roundTrip.ActorUserID)
	assert.Equal(t, "actor@example.com", roundTrip.ActorEmail)
	assert.Equal(t, SecurityAuditActionSessionRevoked, roundTrip.Action)
	assert.Equal(t, "target-1", roundTrip.TargetUserID)
	assert.Empty(t, roundTrip.TargetEmail)
	assert.Equal(t, "192.0.2.10", roundTrip.RequestIP)
	assert.Empty(t, roundTrip.UserAgent)
	assert.Equal(t, createdAt, roundTrip.CreatedAt)
}

func TestModelToRefreshSessionPreservesOptionalTimes(t *testing.T) {
	createdAt := time.Date(2026, 6, 12, 9, 0, 0, 0, time.UTC)
	lastUsedAt := createdAt.Add(15 * time.Minute)
	expiresAt := createdAt.Add(24 * time.Hour)
	revokedAt := createdAt.Add(2 * time.Hour)

	session := modelToRefreshSession(&models.RefreshSession{
		ID:         "session-1",
		UserID:     "user-1",
		TokenHash:  "secret-token-hash",
		CreatedAt:  createdAt,
		LastUsedAt: &lastUsedAt,
		ExpiresAt:  expiresAt,
		RevokedAt:  &revokedAt,
	})

	assert.Equal(t, "session-1", session.ID)
	assert.Equal(t, "user-1", session.UserID)
	assert.Equal(t, createdAt, session.CreatedAt)
	require.NotNil(t, session.LastUsedAt)
	assert.Equal(t, lastUsedAt, *session.LastUsedAt)
	assert.Equal(t, expiresAt, session.ExpiresAt)
	require.NotNil(t, session.RevokedAt)
	assert.Equal(t, revokedAt, *session.RevokedAt)

	activeSession := modelToRefreshSession(&models.RefreshSession{
		ID:        "session-2",
		UserID:    "user-1",
		CreatedAt: createdAt,
		ExpiresAt: expiresAt,
	})

	assert.Nil(t, activeSession.LastUsedAt)
	assert.Nil(t, activeSession.RevokedAt)
}
