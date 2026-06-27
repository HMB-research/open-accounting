package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/HMB-research/open-accounting/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"gorm.io/gorm"
)

const (
	SecurityAuditActionLogin                  = "login"
	SecurityAuditActionLoginFailed            = "login_failed"
	SecurityAuditActionLogout                 = "logout"
	SecurityAuditActionPasswordChanged        = "password_changed"
	SecurityAuditActionPasswordResetRequested = "password_reset_requested"
	SecurityAuditActionPasswordResetCompleted = "password_reset_completed"
	SecurityAuditActionSessionRevoked         = "session_revoked"
	SecurityAuditActionAllSessionsRevoked     = "all_sessions_revoked"
	SecurityAuditActionAPITokenCreated        = "api_token_created"
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
	db  *gorm.DB
	now func() time.Time
}

var marshalSecurityAuditMetadata = json.Marshal

// NewSecurityAuditService creates a PostgreSQL-backed security audit service.
func NewSecurityAuditService(pool *pgxpool.Pool) *SecurityAuditService {
	gormDB, err := newGormDBFromPool(context.Background(), pool)
	if err != nil {
		panic(fmt.Errorf("create security audit GORM repository: %w", err))
	}
	return &SecurityAuditService{
		db:  gormDB,
		now: time.Now,
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
	metadataJSON, err := marshalSecurityAuditMetadata(event.Metadata)
	if err != nil {
		return fmt.Errorf("marshal security audit metadata: %w", err)
	}

	if err := s.db.WithContext(ctx).Create(securityAuditEventToModel(event, metadataJSON)).Error; err != nil {
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

	var eventModels []models.SecurityAuditEvent
	err := s.db.WithContext(ctx).
		Where("actor_user_id = ? OR target_user_id = ?", userID, userID).
		Order("created_at DESC").
		Limit(limit).
		Find(&eventModels).Error
	if err != nil {
		return nil, fmt.Errorf("list security audit events: %w", err)
	}

	events := make([]SecurityAuditEvent, 0, len(eventModels))
	for _, eventModel := range eventModels {
		event := modelToSecurityAuditEvent(&eventModel)
		if len(eventModel.Metadata) > 0 {
			if err := json.Unmarshal(eventModel.Metadata, &event.Metadata); err != nil {
				return nil, fmt.Errorf("unmarshal security audit metadata: %w", err)
			}
		}
		if event.Metadata == nil {
			event.Metadata = map[string]string{}
		}
		events = append(events, event)
	}
	return events, nil
}

func securityAuditEventToModel(event *SecurityAuditEvent, metadata json.RawMessage) *models.SecurityAuditEvent {
	return &models.SecurityAuditEvent{
		ID:           event.ID,
		ActorUserID:  auditOptionalString(event.ActorUserID),
		ActorEmail:   auditOptionalString(event.ActorEmail),
		Action:       event.Action,
		TargetUserID: auditOptionalString(event.TargetUserID),
		TargetEmail:  auditOptionalString(event.TargetEmail),
		RequestIP:    auditOptionalString(event.RequestIP),
		UserAgent:    auditOptionalString(event.UserAgent),
		Metadata:     metadata,
		CreatedAt:    event.CreatedAt,
	}
}

func modelToSecurityAuditEvent(event *models.SecurityAuditEvent) SecurityAuditEvent {
	return SecurityAuditEvent{
		ID:           event.ID,
		ActorUserID:  auditStringValue(event.ActorUserID),
		ActorEmail:   auditStringValue(event.ActorEmail),
		Action:       event.Action,
		TargetUserID: auditStringValue(event.TargetUserID),
		TargetEmail:  auditStringValue(event.TargetEmail),
		RequestIP:    auditStringValue(event.RequestIP),
		UserAgent:    auditStringValue(event.UserAgent),
		CreatedAt:    event.CreatedAt,
	}
}

func auditOptionalString(value string) *string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func auditStringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
