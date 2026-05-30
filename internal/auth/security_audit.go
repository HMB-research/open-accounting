package auth

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	SecurityAuditActionLogin                  = "login"
	SecurityAuditActionLogout                 = "logout"
	SecurityAuditActionPasswordChanged        = "password_changed"
	SecurityAuditActionPasswordResetRequested = "password_reset_requested"
	SecurityAuditActionPasswordResetCompleted = "password_reset_completed"
	SecurityAuditActionSessionRevoked         = "session_revoked"
	SecurityAuditActionAllSessionsRevoked     = "all_sessions_revoked"
	SecurityAuditActionAPITokenRevoked        = "api_token_revoked"
	SecurityAuditActionTenantAccessSuspended  = "tenant_access_suspended"
	SecurityAuditActionTenantAccessRestored   = "tenant_access_restored"
)

// SecurityAuditEvent records an auth-sensitive account action.
type SecurityAuditEvent struct {
	ID           string            `json:"id"`
	ActorUserID  string            `json:"actor_user_id,omitempty"`
	ActorEmail   string            `json:"actor_email,omitempty"`
	Action       string            `json:"action"`
	TargetUserID string            `json:"target_user_id,omitempty"`
	TargetEmail  string            `json:"target_email,omitempty"`
	RequestIP    string            `json:"request_ip,omitempty"`
	UserAgent    string            `json:"user_agent,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
	CreatedAt    time.Time         `json:"created_at"`
}

// SecurityAuditService stores and lists auth security audit events.
type SecurityAuditService struct {
	pool *pgxpool.Pool
	now  func() time.Time
}

// NewSecurityAuditService creates a PostgreSQL-backed security audit service.
func NewSecurityAuditService(pool *pgxpool.Pool) *SecurityAuditService {
	return &SecurityAuditService{
		pool: pool,
		now:  time.Now,
	}
}

// RecordEvent records a security audit event.
func (s *SecurityAuditService) RecordEvent(ctx context.Context, event *SecurityAuditEvent) error {
	if event == nil {
		return fmt.Errorf("security audit event is required")
	}
	event.Action = strings.TrimSpace(event.Action)
	if event.Action == "" {
		return fmt.Errorf("action is required")
	}
	if event.ID == "" {
		event.ID = uuid.NewString()
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = s.now().UTC()
	}
	if event.Metadata == nil {
		event.Metadata = map[string]string{}
	}
	metadataJSON, err := json.Marshal(event.Metadata)
	if err != nil {
		return fmt.Errorf("marshal security audit metadata: %w", err)
	}

	_, err = s.pool.Exec(ctx, `
		INSERT INTO security_audit_events (
			id, actor_user_id, actor_email, action, target_user_id, target_email,
			request_ip, user_agent, metadata, created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`, event.ID, nullableString(event.ActorUserID), nullableString(event.ActorEmail), event.Action,
		nullableString(event.TargetUserID), nullableString(event.TargetEmail), nullableString(event.RequestIP),
		nullableString(event.UserAgent), metadataJSON, event.CreatedAt)
	if err != nil {
		return fmt.Errorf("record security audit event: %w", err)
	}
	return nil
}

// ListUserEvents returns recent security audit events where the user is actor or target.
func (s *SecurityAuditService) ListUserEvents(ctx context.Context, userID string, limit int) ([]SecurityAuditEvent, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, fmt.Errorf("user ID is required")
	}
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	rows, err := s.pool.Query(ctx, `
		SELECT id::text, actor_user_id::text, COALESCE(actor_email, ''), action,
			target_user_id::text, COALESCE(target_email, ''), COALESCE(request_ip, ''),
			COALESCE(user_agent, ''), metadata, created_at
		FROM security_audit_events
		WHERE actor_user_id = $1 OR target_user_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("list security audit events: %w", err)
	}
	defer rows.Close()

	events := make([]SecurityAuditEvent, 0, limit)
	for rows.Next() {
		var event SecurityAuditEvent
		var actorUserID sql.NullString
		var targetUserID sql.NullString
		var metadataJSON []byte
		if err := rows.Scan(
			&event.ID,
			&actorUserID,
			&event.ActorEmail,
			&event.Action,
			&targetUserID,
			&event.TargetEmail,
			&event.RequestIP,
			&event.UserAgent,
			&metadataJSON,
			&event.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan security audit event: %w", err)
		}
		event.ActorUserID = nullStringValue(actorUserID)
		event.TargetUserID = nullStringValue(targetUserID)
		if len(metadataJSON) > 0 {
			if err := json.Unmarshal(metadataJSON, &event.Metadata); err != nil {
				return nil, fmt.Errorf("unmarshal security audit metadata: %w", err)
			}
		}
		if event.Metadata == nil {
			event.Metadata = map[string]string{}
		}
		events = append(events, event)
	}
	return events, rows.Err()
}

func nullableString(value string) any {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return value
}

func nullStringValue(value sql.NullString) string {
	if !value.Valid {
		return ""
	}
	return value.String
}
