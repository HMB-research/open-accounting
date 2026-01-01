package tenant

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository defines the contract for tenant data access
type Repository interface {
	// Tenant operations
	CreateTenant(ctx context.Context, tenant *Tenant, settingsJSON []byte, ownerID string) error
	GetTenant(ctx context.Context, tenantID string) (*Tenant, error)
	GetTenantBySlug(ctx context.Context, slug string) (*Tenant, error)
	UpdateTenant(ctx context.Context, tenantID, name string, settingsJSON []byte, updatedAt time.Time) error
	DeleteTenant(ctx context.Context, tenantID, schemaName string) error
	CompleteOnboarding(ctx context.Context, tenantID string) error

	// Tenant User operations
	AddUserToTenant(ctx context.Context, tenantID, userID, role string) error
	RemoveUserFromTenant(ctx context.Context, tenantID, userID string) error
	GetUserRole(ctx context.Context, tenantID, userID string) (string, error)
	ListUserTenants(ctx context.Context, userID string) ([]TenantMembership, error)
	ListTenantUsers(ctx context.Context, tenantID string) ([]TenantUser, error)
	UpdateTenantUserRole(ctx context.Context, tenantID, userID, newRole string) error
	RemoveTenantUser(ctx context.Context, tenantID, userID string) error

	// User operations
	CreateUser(ctx context.Context, user *User) error
	GetUserByEmail(ctx context.Context, email string) (*User, error)
	GetUserByID(ctx context.Context, userID string) (*User, error)

	// Invitation operations
	CreateInvitation(ctx context.Context, inv *UserInvitation) error
	GetInvitationByToken(ctx context.Context, token string) (*UserInvitation, error)
	AcceptInvitation(ctx context.Context, inv *UserInvitation, userID string, password string, name string, createUser bool) error
	ListInvitations(ctx context.Context, tenantID string) ([]UserInvitation, error)
	RevokeInvitation(ctx context.Context, tenantID, invitationID string) error
	CheckUserIsMember(ctx context.Context, tenantID, email string) (bool, error)
}

// Common errors
var (
	ErrTenantNotFound     = fmt.Errorf("tenant not found")
	ErrUserNotFound       = fmt.Errorf("user not found")
	ErrUserNotInTenant    = fmt.Errorf("user not member of tenant")
	ErrInvitationNotFound = fmt.Errorf("invitation not found")
	ErrEmailExists        = fmt.Errorf("email already exists")
)

// PostgresRepository implements Repository using PostgreSQL
type PostgresRepository struct {
	db *pgxpool.Pool
}

// NewPostgresRepository creates a new PostgreSQL repository
func NewPostgresRepository(db *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{db: db}
}

// CreateTenant creates a new tenant with its schema
func (r *PostgresRepository) CreateTenant(ctx context.Context, tenant *Tenant, settingsJSON []byte, ownerID string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Insert tenant record
	_, err = tx.Exec(ctx, `
		INSERT INTO tenants (id, name, slug, schema_name, settings, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, tenant.ID, tenant.Name, tenant.Slug, tenant.SchemaName, settingsJSON, tenant.IsActive, tenant.CreatedAt, tenant.UpdatedAt)
	if err != nil {
		return fmt.Errorf("insert tenant: %w", err)
	}

	// Create tenant schema with all tables
	_, err = tx.Exec(ctx, "SELECT create_tenant_schema($1)", tenant.SchemaName)
	if err != nil {
		return fmt.Errorf("create tenant schema: %w", err)
	}

	// Create default chart of accounts
	_, err = tx.Exec(ctx, "SELECT create_default_chart_of_accounts($1, $2)", tenant.SchemaName, tenant.ID)
	if err != nil {
		return fmt.Errorf("create default chart of accounts: %w", err)
	}

	// Add owner as tenant user
	if ownerID != "" {
		_, err = tx.Exec(ctx, `
			INSERT INTO tenant_users (tenant_id, user_id, role, is_default)
			VALUES ($1, $2, $3, true)
		`, tenant.ID, ownerID, RoleOwner)
		if err != nil {
			return fmt.Errorf("add owner to tenant: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

// GetTenant retrieves a tenant by ID
func (r *PostgresRepository) GetTenant(ctx context.Context, tenantID string) (*Tenant, error) {
	var t Tenant
	var settingsJSON []byte
	err := r.db.QueryRow(ctx, `
		SELECT id, name, slug, schema_name, settings, is_active, onboarding_completed, created_at, updated_at
		FROM tenants
		WHERE id = $1
	`, tenantID).Scan(
		&t.ID, &t.Name, &t.Slug, &t.SchemaName, &settingsJSON,
		&t.IsActive, &t.OnboardingCompleted, &t.CreatedAt, &t.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, ErrTenantNotFound
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
func (r *PostgresRepository) GetTenantBySlug(ctx context.Context, slug string) (*Tenant, error) {
	var t Tenant
	var settingsJSON []byte
	err := r.db.QueryRow(ctx, `
		SELECT id, name, slug, schema_name, settings, is_active, onboarding_completed, created_at, updated_at
		FROM tenants
		WHERE slug = $1
	`, slug).Scan(
		&t.ID, &t.Name, &t.Slug, &t.SchemaName, &settingsJSON,
		&t.IsActive, &t.OnboardingCompleted, &t.CreatedAt, &t.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, ErrTenantNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get tenant: %w", err)
	}

	if err := json.Unmarshal(settingsJSON, &t.Settings); err != nil {
		t.Settings = DefaultSettings()
	}

	return &t, nil
}

// UpdateTenant updates a tenant's name and/or settings
func (r *PostgresRepository) UpdateTenant(ctx context.Context, tenantID, name string, settingsJSON []byte, updatedAt time.Time) error {
	_, err := r.db.Exec(ctx, `
		UPDATE tenants
		SET name = $1, settings = $2, updated_at = $3
		WHERE id = $4
	`, name, settingsJSON, updatedAt, tenantID)
	if err != nil {
		return fmt.Errorf("update tenant: %w", err)
	}
	return nil
}

// DeleteTenant deletes a tenant and its schema
func (r *PostgresRepository) DeleteTenant(ctx context.Context, tenantID, schemaName string) error {
	tx, err := r.db.Begin(ctx)
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
	_, err = tx.Exec(ctx, "SELECT drop_tenant_schema($1)", schemaName)
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

// CompleteOnboarding marks the tenant's onboarding as completed
func (r *PostgresRepository) CompleteOnboarding(ctx context.Context, tenantID string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE tenants SET onboarding_completed = true, updated_at = NOW()
		WHERE id = $1
	`, tenantID)
	if err != nil {
		return fmt.Errorf("complete onboarding: %w", err)
	}
	return nil
}

// AddUserToTenant adds a user to a tenant with a specified role
func (r *PostgresRepository) AddUserToTenant(ctx context.Context, tenantID, userID, role string) error {
	_, err := r.db.Exec(ctx, `
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
func (r *PostgresRepository) RemoveUserFromTenant(ctx context.Context, tenantID, userID string) error {
	result, err := r.db.Exec(ctx, `
		DELETE FROM tenant_users WHERE tenant_id = $1 AND user_id = $2
	`, tenantID, userID)
	if err != nil {
		return fmt.Errorf("remove user from tenant: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrUserNotInTenant
	}
	return nil
}

// GetUserRole returns the user's role in a tenant
func (r *PostgresRepository) GetUserRole(ctx context.Context, tenantID, userID string) (string, error) {
	var role string
	err := r.db.QueryRow(ctx, `
		SELECT role FROM tenant_users WHERE tenant_id = $1 AND user_id = $2
	`, tenantID, userID).Scan(&role)
	if err == pgx.ErrNoRows {
		return "", ErrUserNotInTenant
	}
	if err != nil {
		return "", fmt.Errorf("get user role: %w", err)
	}
	return role, nil
}

// ListUserTenants retrieves all tenants a user belongs to
func (r *PostgresRepository) ListUserTenants(ctx context.Context, userID string) ([]TenantMembership, error) {
	rows, err := r.db.Query(ctx, `
		SELECT t.id, t.name, t.slug, t.schema_name, t.settings, t.is_active, t.onboarding_completed, t.created_at, t.updated_at,
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
			&m.Tenant.IsActive, &m.Tenant.OnboardingCompleted, &m.Tenant.CreatedAt, &m.Tenant.UpdatedAt,
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

// ListTenantUsers lists all users in a tenant
func (r *PostgresRepository) ListTenantUsers(ctx context.Context, tenantID string) ([]TenantUser, error) {
	rows, err := r.db.Query(ctx, `
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

// UpdateTenantUserRole updates a user's role in a tenant
func (r *PostgresRepository) UpdateTenantUserRole(ctx context.Context, tenantID, userID, newRole string) error {
	_, err := r.db.Exec(ctx, `UPDATE tenant_users SET role = $3 WHERE tenant_id = $1 AND user_id = $2`, tenantID, userID, newRole)
	if err != nil {
		return fmt.Errorf("update role: %w", err)
	}
	return nil
}

// RemoveTenantUser removes a user from a tenant
func (r *PostgresRepository) RemoveTenantUser(ctx context.Context, tenantID, userID string) error {
	_, err := r.db.Exec(ctx, `DELETE FROM tenant_users WHERE tenant_id = $1 AND user_id = $2`, tenantID, userID)
	if err != nil {
		return fmt.Errorf("remove user: %w", err)
	}
	return nil
}

// CreateUser creates a new user
func (r *PostgresRepository) CreateUser(ctx context.Context, user *User) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO users (id, email, password_hash, name, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, user.ID, user.Email, user.PasswordHash, user.Name, user.IsActive, user.CreatedAt, user.UpdatedAt)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") {
			return ErrEmailExists
		}
		return fmt.Errorf("create user: %w", err)
	}
	return nil
}

// GetUserByEmail retrieves a user by email
func (r *PostgresRepository) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	var u User
	err := r.db.QueryRow(ctx, `
		SELECT id, email, password_hash, name, is_active, created_at, updated_at
		FROM users WHERE email = $1
	`, strings.ToLower(strings.TrimSpace(email))).Scan(
		&u.ID, &u.Email, &u.PasswordHash, &u.Name, &u.IsActive, &u.CreatedAt, &u.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}
	return &u, nil
}

// GetUserByID retrieves a user by ID
func (r *PostgresRepository) GetUserByID(ctx context.Context, userID string) (*User, error) {
	var u User
	err := r.db.QueryRow(ctx, `
		SELECT id, email, password_hash, name, is_active, created_at, updated_at
		FROM users WHERE id = $1
	`, userID).Scan(
		&u.ID, &u.Email, &u.PasswordHash, &u.Name, &u.IsActive, &u.CreatedAt, &u.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}
	return &u, nil
}

// CreateInvitation creates a new user invitation
func (r *PostgresRepository) CreateInvitation(ctx context.Context, inv *UserInvitation) error {
	_, err := r.db.Exec(ctx, `
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
		return fmt.Errorf("create invitation: %w", err)
	}
	return nil
}

// GetInvitationByToken retrieves an invitation by its token
func (r *PostgresRepository) GetInvitationByToken(ctx context.Context, token string) (*UserInvitation, error) {
	var inv UserInvitation
	err := r.db.QueryRow(ctx, `
		SELECT i.id, i.tenant_id, t.name, i.email, i.role, i.invited_by, i.token, i.expires_at, i.accepted_at, i.created_at
		FROM user_invitations i
		JOIN tenants t ON t.id = i.tenant_id
		WHERE i.token = $1
	`, token).Scan(
		&inv.ID, &inv.TenantID, &inv.TenantName, &inv.Email, &inv.Role,
		&inv.InvitedBy, &inv.Token, &inv.ExpiresAt, &inv.AcceptedAt, &inv.CreatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, ErrInvitationNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get invitation: %w", err)
	}
	return &inv, nil
}

// AcceptInvitation accepts an invitation and adds the user to the tenant
func (r *PostgresRepository) AcceptInvitation(ctx context.Context, inv *UserInvitation, userID string, password string, name string, createUser bool) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if createUser {
		_, err = tx.Exec(ctx, `
			INSERT INTO users (id, email, password_hash, name, is_active, created_at, updated_at)
			VALUES ($1, $2, $3, $4, true, NOW(), NOW())
		`, userID, inv.Email, password, name)
		if err != nil {
			return fmt.Errorf("create user: %w", err)
		}
	}

	// Add user to tenant
	_, err = tx.Exec(ctx, `
		INSERT INTO tenant_users (tenant_id, user_id, role, is_default, invited_by, invited_at, created_at)
		VALUES ($1, $2, $3, false, $4, NOW(), NOW())
		ON CONFLICT (tenant_id, user_id) DO UPDATE SET role = EXCLUDED.role
	`, inv.TenantID, userID, inv.Role, inv.InvitedBy)
	if err != nil {
		return fmt.Errorf("add user to tenant: %w", err)
	}

	// Mark invitation as accepted
	_, err = tx.Exec(ctx, `UPDATE user_invitations SET accepted_at = NOW() WHERE id = $1`, inv.ID)
	if err != nil {
		return fmt.Errorf("mark invitation accepted: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	return nil
}

// ListInvitations lists pending invitations for a tenant
func (r *PostgresRepository) ListInvitations(ctx context.Context, tenantID string) ([]UserInvitation, error) {
	rows, err := r.db.Query(ctx, `
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
func (r *PostgresRepository) RevokeInvitation(ctx context.Context, tenantID, invitationID string) error {
	result, err := r.db.Exec(ctx, `
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

// CheckUserIsMember checks if a user is already a member of a tenant
func (r *PostgresRepository) CheckUserIsMember(ctx context.Context, tenantID, email string) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM tenant_users tu
			JOIN users u ON u.id = tu.user_id
			WHERE tu.tenant_id = $1 AND LOWER(u.email) = $2
		)
	`, tenantID, email).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check existing member: %w", err)
	}
	return exists, nil
}
