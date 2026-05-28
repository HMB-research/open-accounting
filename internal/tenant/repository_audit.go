package tenant

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
)

func (r *PostgresRepository) CreateTenantAuditEvent(ctx context.Context, event *TenantAuditEvent) error {
	metadataJSON, err := json.Marshal(event.Metadata)
	if err != nil {
		return fmt.Errorf("marshal audit metadata: %w", err)
	}

	var actorUserID any
	if event.ActorUserID != "" {
		actorUserID = event.ActorUserID
	}
	var targetEmail any
	if event.TargetEmail != "" {
		targetEmail = event.TargetEmail
	}

	_, err = r.db.Exec(ctx, `
		INSERT INTO tenant_audit_events (
			id,
			tenant_id,
			actor_user_id,
			action,
			target_type,
			target_id,
			target_email,
			metadata,
			created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8::jsonb, $9)
	`, event.ID, event.TenantID, actorUserID, event.Action, event.TargetType, event.TargetID, targetEmail, string(metadataJSON), event.CreatedAt)
	if err != nil {
		return fmt.Errorf("create tenant audit event: %w", err)
	}
	return nil
}

func (r *PostgresRepository) ListTenantAuditEvents(ctx context.Context, tenantID string, limit int) ([]TenantAuditEvent, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	rows, err := r.db.Query(ctx, `
		SELECT id, tenant_id, actor_user_id, action, target_type, target_id, target_email, metadata, created_at
		FROM tenant_audit_events
		WHERE tenant_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`, tenantID, limit)
	if err != nil {
		return nil, fmt.Errorf("list tenant audit events: %w", err)
	}
	defer rows.Close()

	events := make([]TenantAuditEvent, 0, limit)
	for rows.Next() {
		var event TenantAuditEvent
		var actorUserID sql.NullString
		var targetEmail sql.NullString
		var metadataJSON []byte
		if err := rows.Scan(
			&event.ID,
			&event.TenantID,
			&actorUserID,
			&event.Action,
			&event.TargetType,
			&event.TargetID,
			&targetEmail,
			&metadataJSON,
			&event.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan tenant audit event: %w", err)
		}
		if actorUserID.Valid {
			event.ActorUserID = actorUserID.String
		}
		if targetEmail.Valid {
			event.TargetEmail = targetEmail.String
		}
		if len(metadataJSON) > 0 {
			if err := json.Unmarshal(metadataJSON, &event.Metadata); err != nil {
				return nil, fmt.Errorf("parse audit metadata: %w", err)
			}
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate tenant audit events: %w", err)
	}

	return events, nil
}
