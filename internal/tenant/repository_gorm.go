package tenant

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/HMB-research/open-accounting/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// GORMRepository implements Repository using GORM
type GORMRepository struct {
	db *gorm.DB
}

// NewGORMRepository creates a new GORM tenant repository
func NewGORMRepository(db *gorm.DB) *GORMRepository {
	return &GORMRepository{db: db}
}

// CreateTenant creates a new tenant with its schema
func (r *GORMRepository) CreateTenant(ctx context.Context, tenant *Tenant, settingsJSON []byte, ownerID string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Insert tenant record
		tenantModel := &models.Tenant{
			ID:                  tenant.ID,
			Name:                tenant.Name,
			Slug:                tenant.Slug,
			SchemaName:          tenant.SchemaName,
			Settings:            settingsJSON,
			IsActive:            tenant.IsActive,
			OnboardingCompleted: false,
			CreatedAt:           tenant.CreatedAt,
			UpdatedAt:           tenant.UpdatedAt,
		}
		if err := tx.Create(tenantModel).Error; err != nil {
			return fmt.Errorf("insert tenant: %w", err)
		}

		// Create tenant schema with all tables (PostgreSQL function)
		if err := tx.Exec("SELECT create_tenant_schema(?)", tenant.SchemaName).Error; err != nil {
			return fmt.Errorf("create tenant schema: %w", err)
		}

		// Create default chart of accounts (PostgreSQL function)
		if err := tx.Exec("SELECT create_default_chart_of_accounts(?, ?)", tenant.SchemaName, tenant.ID).Error; err != nil {
			return fmt.Errorf("create default chart of accounts: %w", err)
		}

		// Add owner as tenant user
		if ownerID != "" {
			tuModel := &models.TenantUserModel{
				TenantID:  tenant.ID,
				UserID:    ownerID,
				Role:      RoleOwner,
				IsDefault: true,
				CreatedAt: time.Now(),
			}
			if err := tx.Create(tuModel).Error; err != nil {
				return fmt.Errorf("add owner to tenant: %w", err)
			}
		}

		return nil
	})
}

// GetTenant retrieves a tenant by ID
func (r *GORMRepository) GetTenant(ctx context.Context, tenantID string) (*Tenant, error) {
	var tenantModel models.Tenant
	if err := r.db.WithContext(ctx).Where("id = ?", tenantID).First(&tenantModel).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrTenantNotFound
		}
		return nil, fmt.Errorf("get tenant: %w", err)
	}

	return modelToTenant(&tenantModel), nil
}

// GetTenantBySlug retrieves a tenant by slug
func (r *GORMRepository) GetTenantBySlug(ctx context.Context, slug string) (*Tenant, error) {
	var tenantModel models.Tenant
	if err := r.db.WithContext(ctx).Where("slug = ?", slug).First(&tenantModel).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrTenantNotFound
		}
		return nil, fmt.Errorf("get tenant: %w", err)
	}

	return modelToTenant(&tenantModel), nil
}

// UpdateTenant updates a tenant's name and/or settings
func (r *GORMRepository) UpdateTenant(ctx context.Context, tenantID, name string, settingsJSON []byte, updatedAt time.Time) error {
	if err := r.db.WithContext(ctx).Model(&models.Tenant{}).
		Where("id = ?", tenantID).
		Updates(map[string]interface{}{
			"name":       name,
			"settings":   settingsJSON,
			"updated_at": updatedAt,
		}).Error; err != nil {
		return fmt.Errorf("update tenant: %w", err)
	}
	return nil
}

// DeleteTenant deletes a tenant and its schema
func (r *GORMRepository) DeleteTenant(ctx context.Context, tenantID, schemaName string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Remove all tenant users
		if err := tx.Where("tenant_id = ?", tenantID).Delete(&models.TenantUserModel{}).Error; err != nil {
			return fmt.Errorf("delete tenant users: %w", err)
		}

		// Drop tenant schema (PostgreSQL function)
		if err := tx.Exec("SELECT drop_tenant_schema(?)", schemaName).Error; err != nil {
			return fmt.Errorf("drop tenant schema: %w", err)
		}

		// Delete tenant record
		if err := tx.Where("id = ?", tenantID).Delete(&models.Tenant{}).Error; err != nil {
			return fmt.Errorf("delete tenant: %w", err)
		}

		return nil
	})
}

// CompleteOnboarding marks the tenant's onboarding as completed
func (r *GORMRepository) CompleteOnboarding(ctx context.Context, tenantID string) error {
	if err := r.db.WithContext(ctx).Model(&models.Tenant{}).
		Where("id = ?", tenantID).
		Updates(map[string]interface{}{
			"onboarding_completed": true,
			"updated_at":           time.Now(),
		}).Error; err != nil {
		return fmt.Errorf("complete onboarding: %w", err)
	}
	return nil
}

// CreateTenantAuditEvent records a tenant administration audit event.
func (r *GORMRepository) CreateTenantAuditEvent(ctx context.Context, event *TenantAuditEvent) error {
	metadataJSON, err := json.Marshal(event.Metadata)
	if err != nil {
		return fmt.Errorf("marshal audit metadata: %w", err)
	}

	eventModel := tenantAuditEventToModel(event, metadataJSON)
	if err := r.db.WithContext(ctx).Create(eventModel).Error; err != nil {
		return fmt.Errorf("create tenant audit event: %w", err)
	}
	return nil
}

// ListTenantAuditEvents returns recent tenant administration audit events.
func (r *GORMRepository) ListTenantAuditEvents(ctx context.Context, tenantID string, limit int) ([]TenantAuditEvent, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	var eventModels []models.TenantAuditEvent
	if err := r.db.WithContext(ctx).
		Where("tenant_id = ?", tenantID).
		Order("created_at DESC").
		Limit(limit).
		Find(&eventModels).Error; err != nil {
		return nil, fmt.Errorf("list tenant audit events: %w", err)
	}

	events := make([]TenantAuditEvent, 0, len(eventModels))
	for _, eventModel := range eventModels {
		event := modelToTenantAuditEvent(&eventModel)
		if len(eventModel.Metadata) > 0 {
			if err := json.Unmarshal(eventModel.Metadata, &event.Metadata); err != nil {
				return nil, fmt.Errorf("parse audit metadata: %w", err)
			}
		}
		events = append(events, event)
	}
	return events, nil
}

// AddUserToTenant adds a user to a tenant with a specified role
func (r *GORMRepository) AddUserToTenant(ctx context.Context, tenantID, userID, role string) error {
	membership := &models.TenantUserModel{
		TenantID:  tenantID,
		UserID:    userID,
		Role:      role,
		IsDefault: false,
		IsActive:  true,
		CreatedAt: time.Now(),
	}
	if err := r.upsertTenantUser(ctx, r.db, membership); err != nil {
		return fmt.Errorf("add user to tenant: %w", err)
	}
	return nil
}

// RemoveUserFromTenant removes a user from a tenant
func (r *GORMRepository) RemoveUserFromTenant(ctx context.Context, tenantID, userID string) error {
	result := r.db.WithContext(ctx).
		Where("tenant_id = ? AND user_id = ?", tenantID, userID).
		Delete(&models.TenantUserModel{})
	if result.Error != nil {
		return fmt.Errorf("remove user from tenant: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrUserNotInTenant
	}
	return nil
}

// GetUserRole returns the user's role in a tenant
func (r *GORMRepository) GetUserRole(ctx context.Context, tenantID, userID string) (string, error) {
	var role string
	err := r.db.WithContext(ctx).Model(&models.TenantUserModel{}).
		Select("role").
		Where("tenant_id = ? AND user_id = ? AND COALESCE(is_active, true) = true", tenantID, userID).
		Scan(&role).Error
	if err != nil {
		return "", fmt.Errorf("get user role: %w", err)
	}
	if role == "" {
		return "", ErrUserNotInTenant
	}
	return role, nil
}

// GetTenantUser returns one tenant membership, including inactive memberships.
func (r *GORMRepository) GetTenantUser(ctx context.Context, tenantID, userID string) (*TenantUser, error) {
	var tu models.TenantUserModel
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND user_id = ?", tenantID, userID).
		First(&tu).Error
	if err == gorm.ErrRecordNotFound {
		return nil, ErrUserNotInTenant
	}
	if err != nil {
		return nil, fmt.Errorf("get tenant user: %w", err)
	}
	return &TenantUser{
		TenantID:  tu.TenantID,
		UserID:    tu.UserID,
		Role:      tu.Role,
		IsDefault: tu.IsDefault,
		IsActive:  tu.IsActive,
		CreatedAt: tu.CreatedAt,
	}, nil
}

// ListUserTenants retrieves all tenants a user belongs to
func (r *GORMRepository) ListUserTenants(ctx context.Context, userID string) ([]TenantMembership, error) {
	var results []struct {
		models.Tenant
		Role      string
		IsDefault bool
	}

	err := r.db.WithContext(ctx).
		Model(&models.Tenant{}).
		Select("tenants.*, tu.role, tu.is_default").
		Joins("JOIN tenant_users tu ON tu.tenant_id = tenants.id").
		Where("tu.user_id = ? AND tenants.is_active = ? AND tu.is_active = ?", userID, true, true).
		Order("tu.is_default DESC, tenants.name").
		Scan(&results).Error
	if err != nil {
		return nil, fmt.Errorf("list user tenants: %w", err)
	}

	memberships := make([]TenantMembership, len(results))
	for i, res := range results {
		memberships[i] = TenantMembership{
			Tenant:    *modelToTenant(&res.Tenant),
			Role:      res.Role,
			IsDefault: res.IsDefault,
		}
	}

	return memberships, nil
}

// ListTenantUsers lists all users in a tenant
func (r *GORMRepository) ListTenantUsers(ctx context.Context, tenantID string) ([]TenantUser, error) {
	var tuModels []models.TenantUserModel
	if err := r.db.WithContext(ctx).
		Where("tenant_id = ?", tenantID).
		Order("created_at").
		Find(&tuModels).Error; err != nil {
		return nil, fmt.Errorf("list tenant users: %w", err)
	}

	users := make([]TenantUser, len(tuModels))
	for i, tu := range tuModels {
		users[i] = TenantUser{
			TenantID:  tu.TenantID,
			UserID:    tu.UserID,
			Role:      tu.Role,
			IsDefault: tu.IsDefault,
			IsActive:  tu.IsActive,
			CreatedAt: tu.CreatedAt,
		}
	}

	return users, nil
}

// UpdateTenantUserRole updates a user's role in a tenant
func (r *GORMRepository) UpdateTenantUserRole(ctx context.Context, tenantID, userID, newRole string) error {
	if err := r.db.WithContext(ctx).Model(&models.TenantUserModel{}).
		Where("tenant_id = ? AND user_id = ?", tenantID, userID).
		Update("role", newRole).Error; err != nil {
		return fmt.Errorf("update role: %w", err)
	}
	return nil
}

// SetTenantUserActive updates a tenant membership active flag.
func (r *GORMRepository) SetTenantUserActive(ctx context.Context, tenantID, userID string, active bool) error {
	result := r.db.WithContext(ctx).Model(&models.TenantUserModel{}).
		Where("tenant_id = ? AND user_id = ?", tenantID, userID).
		Update("is_active", active)
	if result.Error != nil {
		return fmt.Errorf("update tenant user status: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrUserNotInTenant
	}
	return nil
}

// RemoveTenantUser removes a user from a tenant
func (r *GORMRepository) RemoveTenantUser(ctx context.Context, tenantID, userID string) error {
	if err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND user_id = ?", tenantID, userID).
		Delete(&models.TenantUserModel{}).Error; err != nil {
		return fmt.Errorf("remove user: %w", err)
	}
	return nil
}

// CreateUser creates a new user
func (r *GORMRepository) CreateUser(ctx context.Context, user *User) error {
	userModel := &models.User{
		ID:           user.ID,
		Email:        user.Email,
		PasswordHash: user.PasswordHash,
		Name:         user.Name,
		IsActive:     user.IsActive,
		CreatedAt:    user.CreatedAt,
		UpdatedAt:    user.UpdatedAt,
	}

	if err := r.db.WithContext(ctx).Create(userModel).Error; err != nil {
		if strings.Contains(err.Error(), "duplicate key") {
			return ErrEmailExists
		}
		return fmt.Errorf("create user: %w", err)
	}
	return nil
}

// GetUserByEmail retrieves a user by email
func (r *GORMRepository) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	var userModel models.User
	if err := r.db.WithContext(ctx).
		Where("email = ?", strings.ToLower(strings.TrimSpace(email))).
		First(&userModel).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("get user: %w", err)
	}

	return modelToUser(&userModel), nil
}

// GetUserByID retrieves a user by ID
func (r *GORMRepository) GetUserByID(ctx context.Context, userID string) (*User, error) {
	var userModel models.User
	if err := r.db.WithContext(ctx).Where("id = ?", userID).First(&userModel).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("get user: %w", err)
	}

	return modelToUser(&userModel), nil
}

// UpdateUserPassword updates a user's password hash.
func (r *GORMRepository) UpdateUserPassword(ctx context.Context, userID, passwordHash string, updatedAt time.Time) error {
	result := r.db.WithContext(ctx).
		Model(&models.User{}).
		Where("id = ?", userID).
		Updates(map[string]interface{}{
			"password_hash": passwordHash,
			"updated_at":    updatedAt,
		})
	if result.Error != nil {
		return fmt.Errorf("update user password: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrUserNotFound
	}
	return nil
}

// CreateInvitation creates a new user invitation
func (r *GORMRepository) CreateInvitation(ctx context.Context, inv *UserInvitation) error {
	invModel := userInvitationToModel(inv)
	if err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "tenant_id"}, {Name: "email"}},
			DoUpdates: clause.Assignments(map[string]interface{}{
				"role":        inv.Role,
				"invited_by":  inv.InvitedBy,
				"token":       inv.Token,
				"expires_at":  inv.ExpiresAt,
				"accepted_at": nil,
			}),
		}).
		Create(invModel).Error; err != nil {
		return fmt.Errorf("create invitation: %w", err)
	}
	return nil
}

// GetInvitationByToken retrieves an invitation by its token
func (r *GORMRepository) GetInvitationByToken(ctx context.Context, token string) (*UserInvitation, error) {
	var invModel models.UserInvitation
	err := r.db.WithContext(ctx).
		Preload("Tenant").
		Where("token = ?", token).
		First(&invModel).Error
	if err == gorm.ErrRecordNotFound {
		return nil, ErrInvitationNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get invitation: %w", err)
	}

	inv := modelToUserInvitation(&invModel)
	if invModel.Tenant != nil {
		inv.TenantName = invModel.Tenant.Name
	}
	return inv, nil
}

// AcceptInvitation accepts an invitation and adds the user to the tenant
func (r *GORMRepository) AcceptInvitation(ctx context.Context, inv *UserInvitation, userID string, password string, name string, createUser bool) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if createUser {
			userModel := &models.User{
				ID:           userID,
				Email:        inv.Email,
				PasswordHash: password,
				Name:         name,
				IsActive:     true,
				CreatedAt:    time.Now(),
				UpdatedAt:    time.Now(),
			}
			if err := tx.Create(userModel).Error; err != nil {
				return fmt.Errorf("create user: %w", err)
			}
		}

		now := time.Now()
		membership := &models.TenantUserModel{
			TenantID:  inv.TenantID,
			UserID:    userID,
			Role:      inv.Role,
			IsDefault: false,
			IsActive:  true,
			InvitedBy: &inv.InvitedBy,
			InvitedAt: &now,
			CreatedAt: now,
		}
		if err := r.upsertTenantUser(ctx, tx, membership); err != nil {
			return fmt.Errorf("add user to tenant: %w", err)
		}

		// Mark invitation as accepted
		if err := tx.Model(&models.UserInvitation{}).
			Where("id = ?", inv.ID).
			Update("accepted_at", time.Now()).Error; err != nil {
			return fmt.Errorf("mark invitation accepted: %w", err)
		}

		return nil
	})
}

// ListInvitations lists pending invitations for a tenant
func (r *GORMRepository) ListInvitations(ctx context.Context, tenantID string) ([]UserInvitation, error) {
	var invModels []models.UserInvitation
	if err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND accepted_at IS NULL AND expires_at > ?", tenantID, time.Now()).
		Order("created_at DESC").
		Find(&invModels).Error; err != nil {
		return nil, fmt.Errorf("list invitations: %w", err)
	}

	invitations := make([]UserInvitation, len(invModels))
	for i, im := range invModels {
		invitations[i] = *modelToUserInvitation(&im)
	}

	return invitations, nil
}

// RevokeInvitation revokes a pending invitation
func (r *GORMRepository) RevokeInvitation(ctx context.Context, tenantID, invitationID string) error {
	result := r.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ? AND accepted_at IS NULL", invitationID, tenantID).
		Delete(&models.UserInvitation{})
	if result.Error != nil {
		return fmt.Errorf("revoke invitation: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("invitation not found or already accepted")
	}
	return nil
}

// CheckUserIsMember checks if a user is already a member of a tenant
func (r *GORMRepository) CheckUserIsMember(ctx context.Context, tenantID, email string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&models.TenantUserModel{}).
		Joins("JOIN users u ON u.id = tenant_users.user_id").
		Where("tenant_users.tenant_id = ? AND LOWER(u.email) = ?", tenantID, strings.ToLower(email)).
		Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("check existing member: %w", err)
	}
	return count > 0, nil
}

func (r *GORMRepository) upsertTenantUser(ctx context.Context, db *gorm.DB, membership *models.TenantUserModel) error {
	assignments := map[string]interface{}{
		"role":      membership.Role,
		"is_active": true,
	}
	if membership.InvitedBy != nil {
		assignments["invited_by"] = membership.InvitedBy
	}
	if membership.InvitedAt != nil {
		assignments["invited_at"] = membership.InvitedAt
	}

	return db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "tenant_id"}, {Name: "user_id"}},
			DoUpdates: clause.Assignments(assignments),
		}).
		Create(membership).Error
}

// Conversion helpers

func modelToTenant(m *models.Tenant) *Tenant {
	t := &Tenant{
		ID:                  m.ID,
		Name:                m.Name,
		Slug:                m.Slug,
		SchemaName:          m.SchemaName,
		IsActive:            m.IsActive,
		OnboardingCompleted: m.OnboardingCompleted,
		CreatedAt:           m.CreatedAt,
		UpdatedAt:           m.UpdatedAt,
	}

	if err := json.Unmarshal(m.Settings, &t.Settings); err != nil {
		t.Settings = DefaultSettings()
	}

	return t
}

func modelToUser(m *models.User) *User {
	return &User{
		ID:           m.ID,
		Email:        m.Email,
		PasswordHash: m.PasswordHash,
		Name:         m.Name,
		IsActive:     m.IsActive,
		CreatedAt:    m.CreatedAt,
		UpdatedAt:    m.UpdatedAt,
	}
}

func modelToUserInvitation(m *models.UserInvitation) *UserInvitation {
	return &UserInvitation{
		ID:         m.ID,
		TenantID:   m.TenantID,
		Email:      m.Email,
		Role:       m.Role,
		InvitedBy:  m.InvitedBy,
		Token:      m.Token,
		ExpiresAt:  m.ExpiresAt,
		AcceptedAt: m.AcceptedAt,
		CreatedAt:  m.CreatedAt,
	}
}

func userInvitationToModel(inv *UserInvitation) *models.UserInvitation {
	return &models.UserInvitation{
		ID:         inv.ID,
		TenantID:   inv.TenantID,
		Email:      inv.Email,
		Role:       inv.Role,
		InvitedBy:  inv.InvitedBy,
		Token:      inv.Token,
		ExpiresAt:  inv.ExpiresAt,
		AcceptedAt: inv.AcceptedAt,
		CreatedAt:  inv.CreatedAt,
	}
}

func tenantAuditEventToModel(event *TenantAuditEvent, metadata json.RawMessage) *models.TenantAuditEvent {
	if len(metadata) == 0 || string(metadata) == "null" {
		metadata = json.RawMessage(`{}`)
	}
	return &models.TenantAuditEvent{
		ID:          event.ID,
		TenantID:    event.TenantID,
		ActorUserID: optionalString(event.ActorUserID),
		Action:      event.Action,
		TargetType:  event.TargetType,
		TargetID:    event.TargetID,
		TargetEmail: optionalString(event.TargetEmail),
		Metadata:    metadata,
		CreatedAt:   event.CreatedAt,
	}
}

func modelToTenantAuditEvent(m *models.TenantAuditEvent) TenantAuditEvent {
	event := TenantAuditEvent{
		ID:         m.ID,
		TenantID:   m.TenantID,
		Action:     m.Action,
		TargetType: m.TargetType,
		TargetID:   m.TargetID,
		CreatedAt:  m.CreatedAt,
	}
	if m.ActorUserID != nil {
		event.ActorUserID = *m.ActorUserID
	}
	if m.TargetEmail != nil {
		event.TargetEmail = *m.TargetEmail
	}
	return event
}

func optionalString(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return &value
}

// Ensure GORMRepository implements Repository interface
var _ Repository = (*GORMRepository)(nil)
