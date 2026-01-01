package tenant

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

var slugRegex = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*[a-z0-9]$`)

// Service provides tenant management operations
type Service struct {
	db   *pgxpool.Pool
	repo Repository
}

// NewService creates a new tenant service
func NewService(db *pgxpool.Pool) *Service {
	return &Service{
		db:   db,
		repo: NewPostgresRepository(db),
	}
}

// NewServiceWithRepository creates a new tenant service with a custom repository (for testing)
func NewServiceWithRepository(repo Repository) *Service {
	return &Service{
		repo: repo,
	}
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

	if err := s.repo.CreateTenant(ctx, tenant, settingsJSON, req.OwnerID); err != nil {
		return nil, err
	}

	return tenant, nil
}

// GetTenant retrieves a tenant by ID
func (s *Service) GetTenant(ctx context.Context, tenantID string) (*Tenant, error) {
	tenant, err := s.repo.GetTenant(ctx, tenantID)
	if err == ErrTenantNotFound {
		return nil, fmt.Errorf("tenant not found: %s", tenantID)
	}
	return tenant, err
}

// GetTenantBySlug retrieves a tenant by slug
func (s *Service) GetTenantBySlug(ctx context.Context, slug string) (*Tenant, error) {
	tenant, err := s.repo.GetTenantBySlug(ctx, slug)
	if err == ErrTenantNotFound {
		return nil, fmt.Errorf("tenant not found: %s", slug)
	}
	return tenant, err
}

// UpdateTenant updates a tenant's name and/or settings
func (s *Service) UpdateTenant(ctx context.Context, tenantID string, req *UpdateTenantRequest) (*Tenant, error) {
	// Get current tenant
	current, err := s.GetTenant(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	// Update name if provided
	if req.Name != nil && *req.Name != "" {
		current.Name = *req.Name
	}

	// Update settings if provided
	if req.Settings != nil {
		// Merge settings - keep existing values for fields not provided
		if req.Settings.VATNumber != "" {
			current.Settings.VATNumber = req.Settings.VATNumber
		}
		if req.Settings.RegCode != "" {
			current.Settings.RegCode = req.Settings.RegCode
		}
		if req.Settings.Address != "" {
			current.Settings.Address = req.Settings.Address
		}
		if req.Settings.Email != "" {
			current.Settings.Email = req.Settings.Email
		}
		if req.Settings.Phone != "" {
			current.Settings.Phone = req.Settings.Phone
		}
		if req.Settings.Logo != "" {
			current.Settings.Logo = req.Settings.Logo
		}
		if req.Settings.PDFPrimaryColor != "" {
			current.Settings.PDFPrimaryColor = req.Settings.PDFPrimaryColor
		}
		if req.Settings.PDFFooterText != "" {
			current.Settings.PDFFooterText = req.Settings.PDFFooterText
		}
		if req.Settings.BankDetails != "" {
			current.Settings.BankDetails = req.Settings.BankDetails
		}
		if req.Settings.InvoiceTerms != "" {
			current.Settings.InvoiceTerms = req.Settings.InvoiceTerms
		}
		if req.Settings.Timezone != "" {
			current.Settings.Timezone = req.Settings.Timezone
		}
		if req.Settings.DateFormat != "" {
			current.Settings.DateFormat = req.Settings.DateFormat
		}
		if req.Settings.DecimalSep != "" {
			current.Settings.DecimalSep = req.Settings.DecimalSep
		}
		if req.Settings.ThousandsSep != "" {
			current.Settings.ThousandsSep = req.Settings.ThousandsSep
		}
		if req.Settings.FiscalYearStart != 0 {
			current.Settings.FiscalYearStart = req.Settings.FiscalYearStart
		}
	}

	current.UpdatedAt = time.Now()

	settingsJSON, err := json.Marshal(current.Settings)
	if err != nil {
		return nil, fmt.Errorf("marshal settings: %w", err)
	}

	if err := s.repo.UpdateTenant(ctx, tenantID, current.Name, settingsJSON, current.UpdatedAt); err != nil {
		return nil, err
	}

	return current, nil
}

// ListUserTenants retrieves all tenants a user belongs to
func (s *Service) ListUserTenants(ctx context.Context, userID string) ([]TenantMembership, error) {
	return s.repo.ListUserTenants(ctx, userID)
}

// CompleteOnboarding marks the tenant's onboarding as completed
func (s *Service) CompleteOnboarding(ctx context.Context, tenantID string) error {
	return s.repo.CompleteOnboarding(ctx, tenantID)
}

// AddUserToTenant adds a user to a tenant with a specified role
func (s *Service) AddUserToTenant(ctx context.Context, tenantID, userID, role string) error {
	return s.repo.AddUserToTenant(ctx, tenantID, userID, role)
}

// RemoveUserFromTenant removes a user from a tenant
func (s *Service) RemoveUserFromTenant(ctx context.Context, tenantID, userID string) error {
	err := s.repo.RemoveUserFromTenant(ctx, tenantID, userID)
	if err == ErrUserNotInTenant {
		return fmt.Errorf("user not found in tenant")
	}
	return err
}

// GetUserRole returns the user's role in a tenant
func (s *Service) GetUserRole(ctx context.Context, tenantID, userID string) (string, error) {
	role, err := s.repo.GetUserRole(ctx, tenantID, userID)
	if err == ErrUserNotInTenant {
		return "", fmt.Errorf("user not member of tenant")
	}
	return role, err
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

	if err := s.repo.CreateUser(ctx, user); err != nil {
		if err == ErrEmailExists {
			return nil, fmt.Errorf("email already exists")
		}
		return nil, err
	}

	return user, nil
}

// GetUserByEmail retrieves a user by email
func (s *Service) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	user, err := s.repo.GetUserByEmail(ctx, email)
	if err == ErrUserNotFound {
		return nil, fmt.Errorf("user not found")
	}
	return user, err
}

// GetUserByID retrieves a user by ID
func (s *Service) GetUserByID(ctx context.Context, userID string) (*User, error) {
	user, err := s.repo.GetUserByID(ctx, userID)
	if err == ErrUserNotFound {
		return nil, fmt.Errorf("user not found")
	}
	return user, err
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

	return s.repo.DeleteTenant(ctx, tenantID, tenant.SchemaName)
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
	exists, err := s.repo.CheckUserIsMember(ctx, tenantID, email)
	if err != nil {
		return nil, err
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

	if err := s.repo.CreateInvitation(ctx, inv); err != nil {
		return nil, err
	}

	return inv, nil
}

// GetInvitationByToken retrieves an invitation by its token
func (s *Service) GetInvitationByToken(ctx context.Context, token string) (*UserInvitation, error) {
	inv, err := s.repo.GetInvitationByToken(ctx, token)
	if err == ErrInvitationNotFound {
		return nil, fmt.Errorf("invitation not found")
	}
	if err != nil {
		return nil, err
	}

	if inv.AcceptedAt != nil {
		return nil, fmt.Errorf("invitation already accepted")
	}
	if time.Now().After(inv.ExpiresAt) {
		return nil, fmt.Errorf("invitation expired")
	}

	return inv, nil
}

// AcceptInvitation accepts an invitation and adds the user to the tenant
func (s *Service) AcceptInvitation(ctx context.Context, req *AcceptInvitationRequest) (*TenantMembership, error) {
	inv, err := s.GetInvitationByToken(ctx, req.Token)
	if err != nil {
		return nil, err
	}

	// Check if user exists
	existingUser, err := s.repo.GetUserByEmail(ctx, inv.Email)
	var userID string
	createUser := false

	if err == ErrUserNotFound {
		// New user - need password and name
		if req.Password == "" || req.Name == "" {
			return nil, fmt.Errorf("password and name are required for new users")
		}
		userID = uuid.New().String()
		createUser = true
	} else if err != nil {
		return nil, fmt.Errorf("check user: %w", err)
	} else {
		userID = existingUser.ID
	}

	// Hash password if creating user
	var passwordHash string
	if createUser {
		hash, hashErr := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if hashErr != nil {
			return nil, fmt.Errorf("hash password: %w", hashErr)
		}
		passwordHash = string(hash)
	}

	if err := s.repo.AcceptInvitation(ctx, inv, userID, passwordHash, req.Name, createUser); err != nil {
		return nil, err
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
	return s.repo.ListInvitations(ctx, tenantID)
}

// RevokeInvitation revokes a pending invitation
func (s *Service) RevokeInvitation(ctx context.Context, tenantID, invitationID string) error {
	return s.repo.RevokeInvitation(ctx, tenantID, invitationID)
}

// RemoveTenantUser removes a user from a tenant
func (s *Service) RemoveTenantUser(ctx context.Context, tenantID, userID string) error {
	// Check if user is owner
	role, err := s.repo.GetUserRole(ctx, tenantID, userID)
	if err == ErrUserNotInTenant {
		return fmt.Errorf("user not found in tenant")
	}
	if err != nil {
		return fmt.Errorf("check user role: %w", err)
	}
	if role == RoleOwner {
		return fmt.Errorf("cannot remove owner from tenant")
	}

	return s.repo.RemoveTenantUser(ctx, tenantID, userID)
}

// UpdateTenantUserRole updates a user's role in a tenant
func (s *Service) UpdateTenantUserRole(ctx context.Context, tenantID, userID, newRole string) error {
	if !IsValidRole(newRole) && newRole != RoleOwner {
		return fmt.Errorf("invalid role: %s", newRole)
	}

	// Check current role
	currentRole, err := s.repo.GetUserRole(ctx, tenantID, userID)
	if err == ErrUserNotInTenant {
		return fmt.Errorf("user not found in tenant")
	}
	if err != nil {
		return fmt.Errorf("check current role: %w", err)
	}
	if currentRole == RoleOwner && newRole != RoleOwner {
		return fmt.Errorf("cannot change owner role")
	}

	return s.repo.UpdateTenantUserRole(ctx, tenantID, userID, newRole)
}

// ListTenantUsers lists all users in a tenant
func (s *Service) ListTenantUsers(ctx context.Context, tenantID string) ([]TenantUser, error) {
	return s.repo.ListTenantUsers(ctx, tenantID)
}
