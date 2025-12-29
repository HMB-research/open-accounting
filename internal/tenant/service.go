package tenant

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

var slugRegex = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*[a-z0-9]$`)

// Service provides tenant management operations
type Service struct {
	db *pgxpool.Pool
}

// NewService creates a new tenant service
func NewService(db *pgxpool.Pool) *Service {
	return &Service{db: db}
}

// CreateTenant creates a new tenant with its schema
func (s *Service) CreateTenant(ctx context.Context, req *CreateTenantRequest) (*Tenant, error) {
	// Validate slug
	if len(req.Slug) < 3 || len(req.Slug) > 50 {
		return nil, fmt.Errorf("slug must be between 3 and 50 characters")
	}
	if !slugRegex.MatchString(req.Slug) {
		return nil, fmt.Errorf("slug must contain only lowercase letters, numbers, and hyphens")
	}

	schemaName := fmt.Sprintf("tenant_%s", strings.ReplaceAll(req.Slug, "-", "_"))

	settings := DefaultSettings()
	if req.Settings != nil {
		settings = *req.Settings
	}

	settingsJSON, err := json.Marshal(settings)
	if err != nil {
		return nil, fmt.Errorf("marshal settings: %w", err)
	}

	tenant := &Tenant{
		ID:         uuid.New().String(),
		Name:       req.Name,
		Slug:       req.Slug,
		SchemaName: schemaName,
		Settings:   settings,
		IsActive:   true,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Insert tenant record
	_, err = tx.Exec(ctx, `
		INSERT INTO tenants (id, name, slug, schema_name, settings, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, tenant.ID, tenant.Name, tenant.Slug, tenant.SchemaName, settingsJSON, tenant.IsActive, tenant.CreatedAt, tenant.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert tenant: %w", err)
	}

	// Create tenant schema with all tables
	_, err = tx.Exec(ctx, "SELECT create_tenant_schema($1)", schemaName)
	if err != nil {
		return nil, fmt.Errorf("create tenant schema: %w", err)
	}

	// Create default chart of accounts
	_, err = tx.Exec(ctx, "SELECT create_default_chart_of_accounts($1, $2)", schemaName, tenant.ID)
	if err != nil {
		return nil, fmt.Errorf("create default chart of accounts: %w", err)
	}

	// Add owner as tenant user
	if req.OwnerID != "" {
		_, err = tx.Exec(ctx, `
			INSERT INTO tenant_users (tenant_id, user_id, role, is_default)
			VALUES ($1, $2, $3, true)
		`, tenant.ID, req.OwnerID, RoleOwner)
		if err != nil {
			return nil, fmt.Errorf("add owner to tenant: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit transaction: %w", err)
	}

	return tenant, nil
}

// GetTenant retrieves a tenant by ID
func (s *Service) GetTenant(ctx context.Context, tenantID string) (*Tenant, error) {
	var t Tenant
	var settingsJSON []byte
	err := s.db.QueryRow(ctx, `
		SELECT id, name, slug, schema_name, settings, is_active, created_at, updated_at
		FROM tenants
		WHERE id = $1
	`, tenantID).Scan(
		&t.ID, &t.Name, &t.Slug, &t.SchemaName, &settingsJSON,
		&t.IsActive, &t.CreatedAt, &t.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("tenant not found: %s", tenantID)
	}
	if err != nil {
		return nil, fmt.Errorf("get tenant: %w", err)
	}

	if err := json.Unmarshal(settingsJSON, &t.Settings); err != nil {
		t.Settings = DefaultSettings()
	}

	return &t, nil
}

// GetTenantBySlug retrieves a tenant by slug
func (s *Service) GetTenantBySlug(ctx context.Context, slug string) (*Tenant, error) {
	var t Tenant
	var settingsJSON []byte
	err := s.db.QueryRow(ctx, `
		SELECT id, name, slug, schema_name, settings, is_active, created_at, updated_at
		FROM tenants
		WHERE slug = $1
	`, slug).Scan(
		&t.ID, &t.Name, &t.Slug, &t.SchemaName, &settingsJSON,
		&t.IsActive, &t.CreatedAt, &t.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("tenant not found: %s", slug)
	}
	if err != nil {
		return nil, fmt.Errorf("get tenant: %w", err)
	}

	if err := json.Unmarshal(settingsJSON, &t.Settings); err != nil {
		t.Settings = DefaultSettings()
	}

	return &t, nil
}

// ListUserTenants retrieves all tenants a user belongs to
func (s *Service) ListUserTenants(ctx context.Context, userID string) ([]TenantMembership, error) {
	rows, err := s.db.Query(ctx, `
		SELECT t.id, t.name, t.slug, t.schema_name, t.settings, t.is_active, t.created_at, t.updated_at,
		       tu.role, tu.is_default
		FROM tenants t
		JOIN tenant_users tu ON tu.tenant_id = t.id
		WHERE tu.user_id = $1 AND t.is_active = true
		ORDER BY tu.is_default DESC, t.name
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("list user tenants: %w", err)
	}
	defer rows.Close()

	var memberships []TenantMembership
	for rows.Next() {
		var m TenantMembership
		var settingsJSON []byte
		if err := rows.Scan(
			&m.Tenant.ID, &m.Tenant.Name, &m.Tenant.Slug, &m.Tenant.SchemaName, &settingsJSON,
			&m.Tenant.IsActive, &m.Tenant.CreatedAt, &m.Tenant.UpdatedAt,
			&m.Role, &m.IsDefault,
		); err != nil {
			return nil, fmt.Errorf("scan tenant: %w", err)
		}

		if err := json.Unmarshal(settingsJSON, &m.Tenant.Settings); err != nil {
			m.Tenant.Settings = DefaultSettings()
		}

		memberships = append(memberships, m)
	}

	return memberships, nil
}

// AddUserToTenant adds a user to a tenant with a specified role
func (s *Service) AddUserToTenant(ctx context.Context, tenantID, userID, role string) error {
	_, err := s.db.Exec(ctx, `
		INSERT INTO tenant_users (tenant_id, user_id, role, is_default)
		VALUES ($1, $2, $3, false)
		ON CONFLICT (tenant_id, user_id) DO UPDATE SET role = $3
	`, tenantID, userID, role)
	if err != nil {
		return fmt.Errorf("add user to tenant: %w", err)
	}
	return nil
}

// RemoveUserFromTenant removes a user from a tenant
func (s *Service) RemoveUserFromTenant(ctx context.Context, tenantID, userID string) error {
	result, err := s.db.Exec(ctx, `
		DELETE FROM tenant_users WHERE tenant_id = $1 AND user_id = $2
	`, tenantID, userID)
	if err != nil {
		return fmt.Errorf("remove user from tenant: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("user not found in tenant")
	}
	return nil
}

// GetUserRole returns the user's role in a tenant
func (s *Service) GetUserRole(ctx context.Context, tenantID, userID string) (string, error) {
	var role string
	err := s.db.QueryRow(ctx, `
		SELECT role FROM tenant_users WHERE tenant_id = $1 AND user_id = $2
	`, tenantID, userID).Scan(&role)
	if err == pgx.ErrNoRows {
		return "", fmt.Errorf("user not member of tenant")
	}
	if err != nil {
		return "", fmt.Errorf("get user role: %w", err)
	}
	return role, nil
}

// CreateUser creates a new user
func (s *Service) CreateUser(ctx context.Context, req *CreateUserRequest) (*User, error) {
	// Hash password
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	user := &User{
		ID:           uuid.New().String(),
		Email:        strings.ToLower(strings.TrimSpace(req.Email)),
		PasswordHash: string(hash),
		Name:         req.Name,
		IsActive:     true,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	_, err = s.db.Exec(ctx, `
		INSERT INTO users (id, email, password_hash, name, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, user.ID, user.Email, user.PasswordHash, user.Name, user.IsActive, user.CreatedAt, user.UpdatedAt)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") {
			return nil, fmt.Errorf("email already exists")
		}
		return nil, fmt.Errorf("create user: %w", err)
	}

	return user, nil
}

// GetUserByEmail retrieves a user by email
func (s *Service) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	var u User
	err := s.db.QueryRow(ctx, `
		SELECT id, email, password_hash, name, is_active, created_at, updated_at
		FROM users WHERE email = $1
	`, strings.ToLower(strings.TrimSpace(email))).Scan(
		&u.ID, &u.Email, &u.PasswordHash, &u.Name, &u.IsActive, &u.CreatedAt, &u.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("user not found")
	}
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}
	return &u, nil
}

// GetUserByID retrieves a user by ID
func (s *Service) GetUserByID(ctx context.Context, userID string) (*User, error) {
	var u User
	err := s.db.QueryRow(ctx, `
		SELECT id, email, password_hash, name, is_active, created_at, updated_at
		FROM users WHERE id = $1
	`, userID).Scan(
		&u.ID, &u.Email, &u.PasswordHash, &u.Name, &u.IsActive, &u.CreatedAt, &u.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("user not found")
	}
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}
	return &u, nil
}

// ValidatePassword checks if the provided password matches the user's hash
func (s *Service) ValidatePassword(user *User, password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
	return err == nil
}

// DeleteTenant deletes a tenant and its schema (use with caution)
func (s *Service) DeleteTenant(ctx context.Context, tenantID string) error {
	tenant, err := s.GetTenant(ctx, tenantID)
	if err != nil {
		return err
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Remove all tenant users
	_, err = tx.Exec(ctx, "DELETE FROM tenant_users WHERE tenant_id = $1", tenantID)
	if err != nil {
		return fmt.Errorf("delete tenant users: %w", err)
	}

	// Drop tenant schema
	_, err = tx.Exec(ctx, "SELECT drop_tenant_schema($1)", tenant.SchemaName)
	if err != nil {
		return fmt.Errorf("drop tenant schema: %w", err)
	}

	// Delete tenant record
	_, err = tx.Exec(ctx, "DELETE FROM tenants WHERE id = $1", tenantID)
	if err != nil {
		return fmt.Errorf("delete tenant: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

// CreateInvitation creates a new user invitation
func (s *Service) CreateInvitation(ctx context.Context, tenantID, invitedByUserID string, req *CreateInvitationRequest) (*UserInvitation, error) {
	// Validate role
	if !IsValidRole(req.Role) {
		return nil, fmt.Errorf("invalid role: %s", req.Role)
	}

	// Normalize email
	email := strings.ToLower(strings.TrimSpace(req.Email))
	if email == "" {
		return nil, fmt.Errorf("email is required")
	}

	// Check if user is already a member
	var exists bool
	err := s.db.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM tenant_users tu
			JOIN users u ON u.id = tu.user_id
			WHERE tu.tenant_id = $1 AND LOWER(u.email) = $2
		)
	`, tenantID, email).Scan(&exists)
	if err != nil {
		return nil, fmt.Errorf("check existing member: %w", err)
	}
	if exists {
		return nil, fmt.Errorf("user is already a member of this organization")
	}

	// Generate invitation token
	token := uuid.New().String()
	expiresAt := time.Now().Add(7 * 24 * time.Hour) // 7 days

	inv := &UserInvitation{
		ID:        uuid.New().String(),
		TenantID:  tenantID,
		Email:     email,
		Role:      req.Role,
		InvitedBy: invitedByUserID,
		Token:     token,
		ExpiresAt: expiresAt,
		CreatedAt: time.Now(),
	}

	_, err = s.db.Exec(ctx, `
		INSERT INTO user_invitations (id, tenant_id, email, role, invited_by, token, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (tenant_id, email) DO UPDATE SET
			role = EXCLUDED.role,
			invited_by = EXCLUDED.invited_by,
			token = EXCLUDED.token,
			expires_at = EXCLUDED.expires_at,
			accepted_at = NULL
	`, inv.ID, inv.TenantID, inv.Email, inv.Role, inv.InvitedBy, inv.Token, inv.ExpiresAt, inv.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("create invitation: %w", err)
	}

	return inv, nil
}

// GetInvitationByToken retrieves an invitation by its token
func (s *Service) GetInvitationByToken(ctx context.Context, token string) (*UserInvitation, error) {
	var inv UserInvitation
	err := s.db.QueryRow(ctx, `
		SELECT i.id, i.tenant_id, t.name, i.email, i.role, i.invited_by, i.token, i.expires_at, i.accepted_at, i.created_at
		FROM user_invitations i
		JOIN tenants t ON t.id = i.tenant_id
		WHERE i.token = $1
	`, token).Scan(
		&inv.ID, &inv.TenantID, &inv.TenantName, &inv.Email, &inv.Role,
		&inv.InvitedBy, &inv.Token, &inv.ExpiresAt, &inv.AcceptedAt, &inv.CreatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("invitation not found")
	}
	if err != nil {
		return nil, fmt.Errorf("get invitation: %w", err)
	}

	if inv.AcceptedAt != nil {
		return nil, fmt.Errorf("invitation already accepted")
	}
	if time.Now().After(inv.ExpiresAt) {
		return nil, fmt.Errorf("invitation expired")
	}

	return &inv, nil
}

// AcceptInvitation accepts an invitation and adds the user to the tenant
func (s *Service) AcceptInvitation(ctx context.Context, req *AcceptInvitationRequest) (*TenantMembership, error) {
	inv, err := s.GetInvitationByToken(ctx, req.Token)
	if err != nil {
		return nil, err
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Check if user exists
	var userID string
	err = tx.QueryRow(ctx, `SELECT id FROM users WHERE LOWER(email) = $1`, inv.Email).Scan(&userID)
	if err == pgx.ErrNoRows {
		// Create new user
		if req.Password == "" || req.Name == "" {
			return nil, fmt.Errorf("password and name are required for new users")
		}
		hash, hashErr := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if hashErr != nil {
			return nil, fmt.Errorf("hash password: %w", hashErr)
		}
		userID = uuid.New().String()
		_, err = tx.Exec(ctx, `
			INSERT INTO users (id, email, password_hash, name, is_active, created_at, updated_at)
			VALUES ($1, $2, $3, $4, true, NOW(), NOW())
		`, userID, inv.Email, string(hash), req.Name)
		if err != nil {
			return nil, fmt.Errorf("create user: %w", err)
		}
	} else if err != nil {
		return nil, fmt.Errorf("check user: %w", err)
	}

	// Add user to tenant
	_, err = tx.Exec(ctx, `
		INSERT INTO tenant_users (tenant_id, user_id, role, is_default, invited_by, invited_at, created_at)
		VALUES ($1, $2, $3, false, $4, NOW(), NOW())
		ON CONFLICT (tenant_id, user_id) DO UPDATE SET role = EXCLUDED.role
	`, inv.TenantID, userID, inv.Role, inv.InvitedBy)
	if err != nil {
		return nil, fmt.Errorf("add user to tenant: %w", err)
	}

	// Mark invitation as accepted
	_, err = tx.Exec(ctx, `UPDATE user_invitations SET accepted_at = NOW() WHERE id = $1`, inv.ID)
	if err != nil {
		return nil, fmt.Errorf("mark invitation accepted: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	// Get the tenant for the response
	tenant, err := s.GetTenant(ctx, inv.TenantID)
	if err != nil {
		return nil, fmt.Errorf("get tenant: %w", err)
	}

	return &TenantMembership{
		Tenant:    *tenant,
		Role:      inv.Role,
		IsDefault: false,
	}, nil
}

// ListInvitations lists pending invitations for a tenant
func (s *Service) ListInvitations(ctx context.Context, tenantID string) ([]UserInvitation, error) {
	rows, err := s.db.Query(ctx, `
		SELECT id, tenant_id, email, role, invited_by, expires_at, created_at
		FROM user_invitations
		WHERE tenant_id = $1 AND accepted_at IS NULL AND expires_at > NOW()
		ORDER BY created_at DESC
	`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list invitations: %w", err)
	}
	defer rows.Close()

	var invitations []UserInvitation
	for rows.Next() {
		var inv UserInvitation
		if err := rows.Scan(&inv.ID, &inv.TenantID, &inv.Email, &inv.Role, &inv.InvitedBy, &inv.ExpiresAt, &inv.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan invitation: %w", err)
		}
		invitations = append(invitations, inv)
	}

	return invitations, nil
}

// RevokeInvitation revokes a pending invitation
func (s *Service) RevokeInvitation(ctx context.Context, tenantID, invitationID string) error {
	result, err := s.db.Exec(ctx, `
		DELETE FROM user_invitations
		WHERE id = $1 AND tenant_id = $2 AND accepted_at IS NULL
	`, invitationID, tenantID)
	if err != nil {
		return fmt.Errorf("revoke invitation: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("invitation not found or already accepted")
	}
	return nil
}

// RemoveTenantUser removes a user from a tenant
func (s *Service) RemoveTenantUser(ctx context.Context, tenantID, userID string) error {
	// Check if user is owner
	var role string
	err := s.db.QueryRow(ctx, `SELECT role FROM tenant_users WHERE tenant_id = $1 AND user_id = $2`, tenantID, userID).Scan(&role)
	if err == pgx.ErrNoRows {
		return fmt.Errorf("user not found in tenant")
	}
	if err != nil {
		return fmt.Errorf("check user role: %w", err)
	}
	if role == RoleOwner {
		return fmt.Errorf("cannot remove owner from tenant")
	}

	_, err = s.db.Exec(ctx, `DELETE FROM tenant_users WHERE tenant_id = $1 AND user_id = $2`, tenantID, userID)
	if err != nil {
		return fmt.Errorf("remove user: %w", err)
	}
	return nil
}

// UpdateTenantUserRole updates a user's role in a tenant
func (s *Service) UpdateTenantUserRole(ctx context.Context, tenantID, userID, newRole string) error {
	if !IsValidRole(newRole) && newRole != RoleOwner {
		return fmt.Errorf("invalid role: %s", newRole)
	}

	// Check current role
	var currentRole string
	err := s.db.QueryRow(ctx, `SELECT role FROM tenant_users WHERE tenant_id = $1 AND user_id = $2`, tenantID, userID).Scan(&currentRole)
	if err == pgx.ErrNoRows {
		return fmt.Errorf("user not found in tenant")
	}
	if err != nil {
		return fmt.Errorf("check current role: %w", err)
	}
	if currentRole == RoleOwner && newRole != RoleOwner {
		return fmt.Errorf("cannot change owner role")
	}

	_, err = s.db.Exec(ctx, `UPDATE tenant_users SET role = $3 WHERE tenant_id = $1 AND user_id = $2`, tenantID, userID, newRole)
	if err != nil {
		return fmt.Errorf("update role: %w", err)
	}
	return nil
}

// ListTenantUsers lists all users in a tenant
func (s *Service) ListTenantUsers(ctx context.Context, tenantID string) ([]TenantUser, error) {
	rows, err := s.db.Query(ctx, `
		SELECT tu.tenant_id, tu.user_id, tu.role, tu.is_default, tu.created_at
		FROM tenant_users tu
		WHERE tu.tenant_id = $1
		ORDER BY tu.created_at
	`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list tenant users: %w", err)
	}
	defer rows.Close()

	var users []TenantUser
	for rows.Next() {
		var u TenantUser
		if err := rows.Scan(&u.TenantID, &u.UserID, &u.Role, &u.IsDefault, &u.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan tenant user: %w", err)
		}
		users = append(users, u)
	}

	return users, nil
}
