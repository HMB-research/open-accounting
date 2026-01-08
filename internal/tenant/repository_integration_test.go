//go:build integration

package tenant

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/HMB-research/open-accounting/internal/testutil"
	"github.com/google/uuid"
)

func TestPostgresRepository_CreateAndGetTenant(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Create a user first (owner)
	ownerID := uuid.New().String()
	ownerEmail := "owner-" + uuid.New().String()[:8] + "@example.com"
	_, err := pool.Exec(ctx, `
		INSERT INTO users (id, email, password_hash, name, created_at, updated_at)
		VALUES ($1, $2, 'hash', 'Owner', NOW(), NOW())
	`, ownerID, ownerEmail)
	if err != nil {
		t.Fatalf("failed to create owner: %v", err)
	}

	settings := DefaultSettings()
	settingsJSON, _ := json.Marshal(settings)

	tenantID := uuid.New().String()
	// Schema name must start with letter to avoid SQL identifier issues
	schemaName := "test_create_" + tenantID[:8]
	tenant := &Tenant{
		ID:         tenantID,
		Name:       "Test Tenant",
		Slug:       "test-tenant-" + uuid.New().String()[:8],
		SchemaName: schemaName,
		IsActive:   true,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	err = repo.CreateTenant(ctx, tenant, settingsJSON, ownerID)
	if err != nil {
		t.Fatalf("CreateTenant failed: %v", err)
	}

	// Get the tenant
	retrieved, err := repo.GetTenant(ctx, tenant.ID)
	if err != nil {
		t.Fatalf("GetTenant failed: %v", err)
	}

	if retrieved.Name != tenant.Name {
		t.Errorf("expected name %s, got %s", tenant.Name, retrieved.Name)
	}
	if retrieved.Slug != tenant.Slug {
		t.Errorf("expected slug %s, got %s", tenant.Slug, retrieved.Slug)
	}
}

func TestPostgresRepository_GetTenantBySlug(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Create a user first
	ownerID := uuid.New().String()
	ownerEmail := "slug-owner-" + uuid.New().String()[:8] + "@example.com"
	_, err := pool.Exec(ctx, `
		INSERT INTO users (id, email, password_hash, name, created_at, updated_at)
		VALUES ($1, $2, 'hash', 'Owner', NOW(), NOW())
	`, ownerID, ownerEmail)
	if err != nil {
		t.Fatalf("failed to create owner: %v", err)
	}

	settings := DefaultSettings()
	settingsJSON, _ := json.Marshal(settings)

	tenantID := uuid.New().String()
	schemaName := "test_slug_" + tenantID[:8]
	slug := "slug-test-" + uuid.New().String()[:8]
	tenant := &Tenant{
		ID:         tenantID,
		Name:       "Slug Test Tenant",
		Slug:       slug,
		SchemaName: schemaName,
		IsActive:   true,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	err = repo.CreateTenant(ctx, tenant, settingsJSON, ownerID)
	if err != nil {
		t.Fatalf("CreateTenant failed: %v", err)
	}

	// Get by slug
	retrieved, err := repo.GetTenantBySlug(ctx, slug)
	if err != nil {
		t.Fatalf("GetTenantBySlug failed: %v", err)
	}

	if retrieved.ID != tenant.ID {
		t.Errorf("expected ID %s, got %s", tenant.ID, retrieved.ID)
	}
}

func TestPostgresRepository_UpdateTenant(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Create a user first
	ownerID := uuid.New().String()
	ownerEmail := "update-owner-" + uuid.New().String()[:8] + "@example.com"
	_, err := pool.Exec(ctx, `
		INSERT INTO users (id, email, password_hash, name, created_at, updated_at)
		VALUES ($1, $2, 'hash', 'Owner', NOW(), NOW())
	`, ownerID, ownerEmail)
	if err != nil {
		t.Fatalf("failed to create owner: %v", err)
	}

	settings := DefaultSettings()
	settingsJSON, _ := json.Marshal(settings)

	tenantID := uuid.New().String()
	schemaName := "test_update_" + tenantID[:8]
	tenant := &Tenant{
		ID:         tenantID,
		Name:       "Original Name",
		Slug:       "update-test-" + uuid.New().String()[:8],
		SchemaName: schemaName,
		IsActive:   true,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	err = repo.CreateTenant(ctx, tenant, settingsJSON, ownerID)
	if err != nil {
		t.Fatalf("CreateTenant failed: %v", err)
	}

	// Update the tenant
	newName := "Updated Name"
	newSettings := DefaultSettings()
	newSettings.Email = "updated@company.com"
	newSettingsJSON, _ := json.Marshal(newSettings)

	err = repo.UpdateTenant(ctx, tenant.ID, newName, newSettingsJSON, time.Now())
	if err != nil {
		t.Fatalf("UpdateTenant failed: %v", err)
	}

	// Verify update
	retrieved, err := repo.GetTenant(ctx, tenant.ID)
	if err != nil {
		t.Fatalf("GetTenant failed: %v", err)
	}

	if retrieved.Name != newName {
		t.Errorf("expected name %s, got %s", newName, retrieved.Name)
	}
}

func TestPostgresRepository_UserTenantOperations(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Create owner user
	ownerID := uuid.New().String()
	ownerEmail := "ops-owner-" + uuid.New().String()[:8] + "@example.com"
	_, err := pool.Exec(ctx, `
		INSERT INTO users (id, email, password_hash, name, created_at, updated_at)
		VALUES ($1, $2, 'hash', 'Owner', NOW(), NOW())
	`, ownerID, ownerEmail)
	if err != nil {
		t.Fatalf("failed to create owner: %v", err)
	}

	// Create second user
	userID := uuid.New().String()
	userEmail := "ops-user-" + uuid.New().String()[:8] + "@example.com"
	_, err = pool.Exec(ctx, `
		INSERT INTO users (id, email, password_hash, name, created_at, updated_at)
		VALUES ($1, $2, 'hash', 'User', NOW(), NOW())
	`, userID, userEmail)
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	settings := DefaultSettings()
	settingsJSON, _ := json.Marshal(settings)

	tenantID := uuid.New().String()
	schemaName := "test_userops_" + tenantID[:8]
	tenant := &Tenant{
		ID:         tenantID,
		Name:       "User Ops Tenant",
		Slug:       "user-ops-" + uuid.New().String()[:8],
		SchemaName: schemaName,
		IsActive:   true,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	err = repo.CreateTenant(ctx, tenant, settingsJSON, ownerID)
	if err != nil {
		t.Fatalf("CreateTenant failed: %v", err)
	}

	// Add user to tenant
	err = repo.AddUserToTenant(ctx, tenant.ID, userID, RoleAccountant)
	if err != nil {
		t.Fatalf("AddUserToTenant failed: %v", err)
	}

	// Get user role
	role, err := repo.GetUserRole(ctx, tenant.ID, userID)
	if err != nil {
		t.Fatalf("GetUserRole failed: %v", err)
	}
	if role != RoleAccountant {
		t.Errorf("expected role %s, got %s", RoleAccountant, role)
	}

	// Update role
	err = repo.UpdateTenantUserRole(ctx, tenant.ID, userID, RoleAdmin)
	if err != nil {
		t.Fatalf("UpdateTenantUserRole failed: %v", err)
	}

	// Verify role update
	role, err = repo.GetUserRole(ctx, tenant.ID, userID)
	if err != nil {
		t.Fatalf("GetUserRole failed: %v", err)
	}
	if role != RoleAdmin {
		t.Errorf("expected role %s, got %s", RoleAdmin, role)
	}

	// List tenant users
	users, err := repo.ListTenantUsers(ctx, tenant.ID)
	if err != nil {
		t.Fatalf("ListTenantUsers failed: %v", err)
	}
	if len(users) < 2 { // owner + user
		t.Errorf("expected at least 2 users, got %d", len(users))
	}

	// List user tenants
	tenants, err := repo.ListUserTenants(ctx, userID)
	if err != nil {
		t.Fatalf("ListUserTenants failed: %v", err)
	}
	if len(tenants) < 1 {
		t.Errorf("expected at least 1 tenant, got %d", len(tenants))
	}

	// Remove user from tenant
	err = repo.RemoveTenantUser(ctx, tenant.ID, userID)
	if err != nil {
		t.Fatalf("RemoveTenantUser failed: %v", err)
	}

	// Verify removal
	_, err = repo.GetUserRole(ctx, tenant.ID, userID)
	if err == nil {
		t.Error("expected error after removing user")
	}
}

func TestPostgresRepository_CreateAndGetUser(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	user := &User{
		ID:           uuid.New().String(),
		Email:        "new-user-" + uuid.New().String()[:8] + "@example.com",
		PasswordHash: "hashed_password",
		Name:         "New User",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	err := repo.CreateUser(ctx, user)
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	// Get by email
	byEmail, err := repo.GetUserByEmail(ctx, user.Email)
	if err != nil {
		t.Fatalf("GetUserByEmail failed: %v", err)
	}
	if byEmail.ID != user.ID {
		t.Errorf("expected ID %s, got %s", user.ID, byEmail.ID)
	}

	// Get by ID
	byID, err := repo.GetUserByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetUserByID failed: %v", err)
	}
	if byID.Email != user.Email {
		t.Errorf("expected email %s, got %s", user.Email, byID.Email)
	}
}

func TestPostgresRepository_CompleteOnboarding(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Create owner
	ownerID := uuid.New().String()
	ownerEmail := "onboard-owner-" + uuid.New().String()[:8] + "@example.com"
	_, err := pool.Exec(ctx, `
		INSERT INTO users (id, email, password_hash, name, created_at, updated_at)
		VALUES ($1, $2, 'hash', 'Owner', NOW(), NOW())
	`, ownerID, ownerEmail)
	if err != nil {
		t.Fatalf("failed to create owner: %v", err)
	}

	settings := DefaultSettings()
	settingsJSON, _ := json.Marshal(settings)

	tenantID := uuid.New().String()
	schemaName := "test_onboard_" + tenantID[:8]
	tenant := &Tenant{
		ID:         tenantID,
		Name:       "Onboarding Tenant",
		Slug:       "onboard-" + uuid.New().String()[:8],
		SchemaName: schemaName,
		IsActive:   true,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	err = repo.CreateTenant(ctx, tenant, settingsJSON, ownerID)
	if err != nil {
		t.Fatalf("CreateTenant failed: %v", err)
	}

	// Complete onboarding
	err = repo.CompleteOnboarding(ctx, tenant.ID)
	if err != nil {
		t.Fatalf("CompleteOnboarding failed: %v", err)
	}

	// Verify onboarding completed
	retrieved, err := repo.GetTenant(ctx, tenant.ID)
	if err != nil {
		t.Fatalf("GetTenant failed: %v", err)
	}

	if !retrieved.OnboardingCompleted {
		t.Error("expected onboarding to be complete")
	}
}

func TestPostgresRepository_InvitationFlow(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Create owner
	ownerID := uuid.New().String()
	ownerEmail := "invite-owner-" + uuid.New().String()[:8] + "@example.com"
	_, err := pool.Exec(ctx, `
		INSERT INTO users (id, email, password_hash, name, created_at, updated_at)
		VALUES ($1, $2, 'hash', 'Owner', NOW(), NOW())
	`, ownerID, ownerEmail)
	if err != nil {
		t.Fatalf("failed to create owner: %v", err)
	}

	settings := DefaultSettings()
	settingsJSON, _ := json.Marshal(settings)

	tenantID := uuid.New().String()
	schemaName := "test_invite_" + tenantID[:8]
	tenant := &Tenant{
		ID:         tenantID,
		Name:       "Invitation Tenant",
		Slug:       "invite-" + uuid.New().String()[:8],
		SchemaName: schemaName,
		IsActive:   true,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	err = repo.CreateTenant(ctx, tenant, settingsJSON, ownerID)
	if err != nil {
		t.Fatalf("CreateTenant failed: %v", err)
	}

	// Create invitation
	token := uuid.New().String()
	invitation := &UserInvitation{
		ID:        uuid.New().String(),
		TenantID:  tenant.ID,
		Email:     "invited-" + uuid.New().String()[:8] + "@example.com",
		Role:      RoleAccountant,
		Token:     token,
		ExpiresAt: time.Now().Add(24 * time.Hour),
		InvitedBy: ownerID,
		CreatedAt: time.Now(),
	}

	err = repo.CreateInvitation(ctx, invitation)
	if err != nil {
		t.Fatalf("CreateInvitation failed: %v", err)
	}

	// Get invitation by token
	retrieved, err := repo.GetInvitationByToken(ctx, token)
	if err != nil {
		t.Fatalf("GetInvitationByToken failed: %v", err)
	}
	if retrieved.Email != invitation.Email {
		t.Errorf("expected email %s, got %s", invitation.Email, retrieved.Email)
	}

	// List invitations
	invitations, err := repo.ListInvitations(ctx, tenant.ID)
	if err != nil {
		t.Fatalf("ListInvitations failed: %v", err)
	}
	if len(invitations) < 1 {
		t.Errorf("expected at least 1 invitation, got %d", len(invitations))
	}

	// Revoke invitation
	err = repo.RevokeInvitation(ctx, tenant.ID, invitation.ID)
	if err != nil {
		t.Fatalf("RevokeInvitation failed: %v", err)
	}
}

func TestPostgresRepository_CheckUserIsMember(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Create owner
	ownerEmail := "member-check-" + uuid.New().String()[:8] + "@example.com"
	ownerID := uuid.New().String()
	_, err := pool.Exec(ctx, `
		INSERT INTO users (id, email, password_hash, name, created_at, updated_at)
		VALUES ($1, $2, 'hash', 'Owner', NOW(), NOW())
	`, ownerID, ownerEmail)
	if err != nil {
		t.Fatalf("failed to create owner: %v", err)
	}

	settings := DefaultSettings()
	settingsJSON, _ := json.Marshal(settings)

	tenantID := uuid.New().String()
	schemaName := "test_member_" + tenantID[:8]
	tenant := &Tenant{
		ID:         tenantID,
		Name:       "Member Check Tenant",
		Slug:       "member-check-" + uuid.New().String()[:8],
		SchemaName: schemaName,
		IsActive:   true,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	err = repo.CreateTenant(ctx, tenant, settingsJSON, ownerID)
	if err != nil {
		t.Fatalf("CreateTenant failed: %v", err)
	}

	// Check if owner is member
	isMember, err := repo.CheckUserIsMember(ctx, tenant.ID, ownerEmail)
	if err != nil {
		t.Fatalf("CheckUserIsMember failed: %v", err)
	}
	if !isMember {
		t.Error("expected owner to be a member")
	}

	// Check non-member
	isMember, err = repo.CheckUserIsMember(ctx, tenant.ID, "nonexistent@example.com")
	if err != nil {
		t.Fatalf("CheckUserIsMember failed: %v", err)
	}
	if isMember {
		t.Error("expected non-existent user to not be a member")
	}
}

func TestPostgresRepository_DeleteTenant(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Create owner
	ownerID := uuid.New().String()
	ownerEmail := "delete-owner-" + uuid.New().String()[:8] + "@example.com"
	_, err := pool.Exec(ctx, `
		INSERT INTO users (id, email, password_hash, name, created_at, updated_at)
		VALUES ($1, $2, 'hash', 'Owner', NOW(), NOW())
	`, ownerID, ownerEmail)
	if err != nil {
		t.Fatalf("failed to create owner: %v", err)
	}

	settings := DefaultSettings()
	settingsJSON, _ := json.Marshal(settings)

	tenantID := uuid.New().String()
	schemaName := "test_delete_" + tenantID[:8]
	tenant := &Tenant{
		ID:         tenantID,
		Name:       "Delete Test Tenant",
		Slug:       "delete-test-" + uuid.New().String()[:8],
		SchemaName: schemaName,
		IsActive:   true,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	err = repo.CreateTenant(ctx, tenant, settingsJSON, ownerID)
	if err != nil {
		t.Fatalf("CreateTenant failed: %v", err)
	}

	// Delete the tenant (pass both tenantID and schemaName)
	err = repo.DeleteTenant(ctx, tenant.ID, tenant.SchemaName)
	if err != nil {
		t.Fatalf("DeleteTenant failed: %v", err)
	}

	// Verify tenant is deleted
	_, err = repo.GetTenant(ctx, tenant.ID)
	if err == nil {
		t.Error("expected error when getting deleted tenant")
	}
}

func TestPostgresRepository_RemoveUserFromTenant(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Create owner
	ownerID := uuid.New().String()
	ownerEmail := "remove-owner-" + uuid.New().String()[:8] + "@example.com"
	_, err := pool.Exec(ctx, `
		INSERT INTO users (id, email, password_hash, name, created_at, updated_at)
		VALUES ($1, $2, 'hash', 'Owner', NOW(), NOW())
	`, ownerID, ownerEmail)
	if err != nil {
		t.Fatalf("failed to create owner: %v", err)
	}

	// Create user to remove
	userID := uuid.New().String()
	userEmail := "remove-user-" + uuid.New().String()[:8] + "@example.com"
	_, err = pool.Exec(ctx, `
		INSERT INTO users (id, email, password_hash, name, created_at, updated_at)
		VALUES ($1, $2, 'hash', 'User', NOW(), NOW())
	`, userID, userEmail)
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	settings := DefaultSettings()
	settingsJSON, _ := json.Marshal(settings)

	tenantID := uuid.New().String()
	schemaName := "test_remove_" + tenantID[:8]
	tenant := &Tenant{
		ID:         tenantID,
		Name:       "Remove User Tenant",
		Slug:       "remove-user-" + uuid.New().String()[:8],
		SchemaName: schemaName,
		IsActive:   true,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	err = repo.CreateTenant(ctx, tenant, settingsJSON, ownerID)
	if err != nil {
		t.Fatalf("CreateTenant failed: %v", err)
	}

	// Add user to tenant
	err = repo.AddUserToTenant(ctx, tenant.ID, userID, RoleAccountant)
	if err != nil {
		t.Fatalf("AddUserToTenant failed: %v", err)
	}

	// Remove user from tenant
	err = repo.RemoveUserFromTenant(ctx, tenant.ID, userID)
	if err != nil {
		t.Fatalf("RemoveUserFromTenant failed: %v", err)
	}

	// Verify user is removed
	_, err = repo.GetUserRole(ctx, tenant.ID, userID)
	if err == nil {
		t.Error("expected error when getting role for removed user")
	}
}

func TestPostgresRepository_AcceptInvitation(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Create owner
	ownerID := uuid.New().String()
	ownerEmail := "accept-owner-" + uuid.New().String()[:8] + "@example.com"
	_, err := pool.Exec(ctx, `
		INSERT INTO users (id, email, password_hash, name, created_at, updated_at)
		VALUES ($1, $2, 'hash', 'Owner', NOW(), NOW())
	`, ownerID, ownerEmail)
	if err != nil {
		t.Fatalf("failed to create owner: %v", err)
	}

	settings := DefaultSettings()
	settingsJSON, _ := json.Marshal(settings)

	tenantID := uuid.New().String()
	schemaName := "test_accept_" + tenantID[:8]
	tenant := &Tenant{
		ID:         tenantID,
		Name:       "Accept Invitation Tenant",
		Slug:       "accept-invite-" + uuid.New().String()[:8],
		SchemaName: schemaName,
		IsActive:   true,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	err = repo.CreateTenant(ctx, tenant, settingsJSON, ownerID)
	if err != nil {
		t.Fatalf("CreateTenant failed: %v", err)
	}

	// Create invitation
	token := uuid.New().String()
	invitedEmail := "invited-accept-" + uuid.New().String()[:8] + "@example.com"
	invitation := &UserInvitation{
		ID:        uuid.New().String(),
		TenantID:  tenant.ID,
		Email:     invitedEmail,
		Role:      RoleAccountant,
		Token:     token,
		ExpiresAt: time.Now().Add(24 * time.Hour),
		InvitedBy: ownerID,
		CreatedAt: time.Now(),
	}

	err = repo.CreateInvitation(ctx, invitation)
	if err != nil {
		t.Fatalf("CreateInvitation failed: %v", err)
	}

	// Accept the invitation with createUser=true (new user flow)
	newUserID := uuid.New().String()
	err = repo.AcceptInvitation(ctx, invitation, newUserID, "hashedpassword123", "New User", true)
	if err != nil {
		t.Fatalf("AcceptInvitation failed: %v", err)
	}

	// Verify user is now a member
	role, err := repo.GetUserRole(ctx, tenant.ID, newUserID)
	if err != nil {
		t.Fatalf("GetUserRole failed: %v", err)
	}
	if role != RoleAccountant {
		t.Errorf("expected role %s, got %s", RoleAccountant, role)
	}

	// Verify invitation is marked as accepted
	retrieved, err := repo.GetInvitationByToken(ctx, token)
	if err != nil {
		t.Fatalf("GetInvitationByToken failed: %v", err)
	}
	if retrieved.AcceptedAt == nil {
		t.Error("expected AcceptedAt to be set after accepting invitation")
	}

	// Verify user was created
	user, err := repo.GetUserByEmail(ctx, invitedEmail)
	if err != nil {
		t.Fatalf("GetUserByEmail failed: %v", err)
	}
	if user.Name != "New User" {
		t.Errorf("expected user name 'New User', got '%s'", user.Name)
	}
}

func TestPostgresRepository_AcceptInvitation_ExistingUser(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Create owner
	ownerID := uuid.New().String()
	ownerEmail := "accept-existing-owner-" + uuid.New().String()[:8] + "@example.com"
	_, err := pool.Exec(ctx, `
		INSERT INTO users (id, email, password_hash, name, created_at, updated_at)
		VALUES ($1, $2, 'hash', 'Owner', NOW(), NOW())
	`, ownerID, ownerEmail)
	if err != nil {
		t.Fatalf("failed to create owner: %v", err)
	}

	settings := DefaultSettings()
	settingsJSON, _ := json.Marshal(settings)

	tenantID := uuid.New().String()
	schemaName := "test_accept_ex_" + tenantID[:8]
	tenant := &Tenant{
		ID:         tenantID,
		Name:       "Accept Existing User Tenant",
		Slug:       "accept-exist-" + uuid.New().String()[:8],
		SchemaName: schemaName,
		IsActive:   true,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	err = repo.CreateTenant(ctx, tenant, settingsJSON, ownerID)
	if err != nil {
		t.Fatalf("CreateTenant failed: %v", err)
	}

	// Create existing user who will accept invitation
	existingUserID := uuid.New().String()
	existingEmail := "existing-user-" + uuid.New().String()[:8] + "@example.com"
	_, err = pool.Exec(ctx, `
		INSERT INTO users (id, email, password_hash, name, created_at, updated_at)
		VALUES ($1, $2, 'hash', 'Existing User', NOW(), NOW())
	`, existingUserID, existingEmail)
	if err != nil {
		t.Fatalf("failed to create existing user: %v", err)
	}

	// Create invitation
	token := uuid.New().String()
	invitation := &UserInvitation{
		ID:        uuid.New().String(),
		TenantID:  tenant.ID,
		Email:     existingEmail,
		Role:      RoleAdmin,
		Token:     token,
		ExpiresAt: time.Now().Add(24 * time.Hour),
		InvitedBy: ownerID,
		CreatedAt: time.Now(),
	}

	err = repo.CreateInvitation(ctx, invitation)
	if err != nil {
		t.Fatalf("CreateInvitation failed: %v", err)
	}

	// Accept the invitation with createUser=false (existing user flow)
	err = repo.AcceptInvitation(ctx, invitation, existingUserID, "", "", false)
	if err != nil {
		t.Fatalf("AcceptInvitation failed: %v", err)
	}

	// Verify user is now a member with correct role
	role, err := repo.GetUserRole(ctx, tenant.ID, existingUserID)
	if err != nil {
		t.Fatalf("GetUserRole failed: %v", err)
	}
	if role != RoleAdmin {
		t.Errorf("expected role %s, got %s", RoleAdmin, role)
	}
}

func TestPostgresRepository_GetTenant_NotFound(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	_, err := repo.GetTenant(ctx, uuid.New().String())
	if err == nil {
		t.Error("expected error when getting non-existent tenant")
	}
}

func TestPostgresRepository_GetTenantBySlug_NotFound(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	_, err := repo.GetTenantBySlug(ctx, "non-existent-slug-"+uuid.New().String()[:8])
	if err == nil {
		t.Error("expected error when getting tenant by non-existent slug")
	}
}

func TestPostgresRepository_GetUserByEmail_NotFound(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	_, err := repo.GetUserByEmail(ctx, "nonexistent-"+uuid.New().String()[:8]+"@example.com")
	if err == nil {
		t.Error("expected error when getting non-existent user by email")
	}
}

func TestPostgresRepository_GetUserByID_NotFound(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	_, err := repo.GetUserByID(ctx, uuid.New().String())
	if err == nil {
		t.Error("expected error when getting non-existent user by ID")
	}
}

func TestPostgresRepository_GetInvitationByToken_NotFound(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	_, err := repo.GetInvitationByToken(ctx, uuid.New().String())
	if err == nil {
		t.Error("expected error when getting invitation by non-existent token")
	}
}

func TestPostgresRepository_CreateUser_DuplicateEmail(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	email := "duplicate-" + uuid.New().String()[:8] + "@example.com"

	// Create first user
	user1 := &User{
		ID:           uuid.New().String(),
		Email:        email,
		PasswordHash: "hashed_password",
		Name:         "First User",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	err := repo.CreateUser(ctx, user1)
	if err != nil {
		t.Fatalf("CreateUser (first) failed: %v", err)
	}

	// Clean up the user after test
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM public.users WHERE id = $1", user1.ID)
	})

	// Try to create second user with same email
	user2 := &User{
		ID:           uuid.New().String(),
		Email:        email,
		PasswordHash: "hashed_password2",
		Name:         "Second User",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	err = repo.CreateUser(ctx, user2)
	if err != ErrEmailExists {
		t.Errorf("expected ErrEmailExists, got %v", err)
	}
}

func TestPostgresRepository_CreateUser_WithIsActive(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	user := &User{
		ID:           uuid.New().String(),
		Email:        "active-" + uuid.New().String()[:8] + "@example.com",
		PasswordHash: "hashed_password",
		Name:         "Active User",
		IsActive:     true,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	err := repo.CreateUser(ctx, user)
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	// Verify user is active
	retrieved, err := repo.GetUserByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetUserByID failed: %v", err)
	}
	if !retrieved.IsActive {
		t.Error("expected user to be active")
	}
}

func TestPostgresRepository_RemoveUserFromTenant_NotFound(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Try to remove a user that doesn't exist in the tenant
	err := repo.RemoveUserFromTenant(ctx, uuid.New().String(), uuid.New().String())
	if err != ErrUserNotInTenant {
		t.Errorf("expected ErrUserNotInTenant, got: %v", err)
	}
}

func TestPostgresRepository_RevokeInvitation_NotFound(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Try to revoke a non-existent invitation
	err := repo.RevokeInvitation(ctx, uuid.New().String(), uuid.New().String())
	if err == nil {
		t.Error("expected error for non-existent invitation")
	}
	// Error message should indicate invitation not found
	if err != nil && err.Error() != "invitation not found or already accepted" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestPostgresRepository_ListTenantUsers_Empty(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Create owner
	ownerID := uuid.New().String()
	ownerEmail := "owner-empty-" + uuid.New().String()[:8] + "@example.com"
	_, err := pool.Exec(ctx, `
		INSERT INTO users (id, email, password_hash, name, created_at, updated_at)
		VALUES ($1, $2, 'hash', 'Owner', NOW(), NOW())
	`, ownerID, ownerEmail)
	if err != nil {
		t.Fatalf("failed to create owner: %v", err)
	}

	settings := DefaultSettings()
	settingsJSON, _ := json.Marshal(settings)

	tenantID := uuid.New().String()
	schemaName := "test_empty_" + tenantID[:8]
	tenant := &Tenant{
		ID:         tenantID,
		Name:       "Empty Users Tenant",
		Slug:       "empty-users-" + uuid.New().String()[:8],
		SchemaName: schemaName,
		IsActive:   true,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	err = repo.CreateTenant(ctx, tenant, settingsJSON, ownerID)
	if err != nil {
		t.Fatalf("CreateTenant failed: %v", err)
	}

	// Remove the owner from tenant_users to test empty list
	_, err = pool.Exec(ctx, "DELETE FROM tenant_users WHERE tenant_id = $1", tenantID)
	if err != nil {
		t.Fatalf("failed to remove users: %v", err)
	}

	// List users - should return empty slice
	users, err := repo.ListTenantUsers(ctx, tenantID)
	if err != nil {
		t.Fatalf("ListTenantUsers failed: %v", err)
	}
	if len(users) != 0 {
		t.Errorf("expected 0 users, got %d", len(users))
	}
}

func TestPostgresRepository_ListInvitations_Empty(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// List invitations for non-existent tenant - should return empty slice
	invitations, err := repo.ListInvitations(ctx, uuid.New().String())
	if err != nil {
		t.Fatalf("ListInvitations failed: %v", err)
	}
	if len(invitations) != 0 {
		t.Errorf("expected 0 invitations, got %d", len(invitations))
	}
}

func TestPostgresRepository_GetUserRole_NotInTenant(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Get role for user not in tenant
	_, err := repo.GetUserRole(ctx, uuid.New().String(), uuid.New().String())
	if err != ErrUserNotInTenant {
		t.Errorf("expected ErrUserNotInTenant, got: %v", err)
	}
}

func TestPostgresRepository_CheckUserIsMember_False(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Check membership for non-existent relationship
	isMember, err := repo.CheckUserIsMember(ctx, uuid.New().String(), uuid.New().String())
	if err != nil {
		t.Fatalf("CheckUserIsMember failed: %v", err)
	}
	if isMember {
		t.Error("expected false for non-member")
	}
}

func TestPostgresRepository_UpdateTenantUserRole(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Create owner
	ownerID := uuid.New().String()
	ownerEmail := "role-owner-" + uuid.New().String()[:8] + "@example.com"
	_, err := pool.Exec(ctx, `
		INSERT INTO users (id, email, password_hash, name, created_at, updated_at)
		VALUES ($1, $2, 'hash', 'Owner', NOW(), NOW())
	`, ownerID, ownerEmail)
	if err != nil {
		t.Fatalf("failed to create owner: %v", err)
	}

	// Create user to update role
	userID := uuid.New().String()
	userEmail := "role-user-" + uuid.New().String()[:8] + "@example.com"
	_, err = pool.Exec(ctx, `
		INSERT INTO users (id, email, password_hash, name, created_at, updated_at)
		VALUES ($1, $2, 'hash', 'User', NOW(), NOW())
	`, userID, userEmail)
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	settings := DefaultSettings()
	settingsJSON, _ := json.Marshal(settings)

	tenantID := uuid.New().String()
	schemaName := "test_role_" + tenantID[:8]
	tenant := &Tenant{
		ID:         tenantID,
		Name:       "Role Update Tenant",
		Slug:       "role-update-" + uuid.New().String()[:8],
		SchemaName: schemaName,
		IsActive:   true,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	err = repo.CreateTenant(ctx, tenant, settingsJSON, ownerID)
	if err != nil {
		t.Fatalf("CreateTenant failed: %v", err)
	}

	// Add user as accountant
	err = repo.AddUserToTenant(ctx, tenant.ID, userID, RoleAccountant)
	if err != nil {
		t.Fatalf("AddUserToTenant failed: %v", err)
	}

	// Update role to admin
	err = repo.UpdateTenantUserRole(ctx, tenant.ID, userID, RoleAdmin)
	if err != nil {
		t.Fatalf("UpdateTenantUserRole failed: %v", err)
	}

	// Verify role was updated
	role, err := repo.GetUserRole(ctx, tenant.ID, userID)
	if err != nil {
		t.Fatalf("GetUserRole failed: %v", err)
	}
	if role != RoleAdmin {
		t.Errorf("expected role %s, got %s", RoleAdmin, role)
	}
}

func TestPostgresRepository_RemoveTenantUser(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Create owner
	ownerID := uuid.New().String()
	ownerEmail := "remove2-owner-" + uuid.New().String()[:8] + "@example.com"
	_, err := pool.Exec(ctx, `
		INSERT INTO users (id, email, password_hash, name, created_at, updated_at)
		VALUES ($1, $2, 'hash', 'Owner', NOW(), NOW())
	`, ownerID, ownerEmail)
	if err != nil {
		t.Fatalf("failed to create owner: %v", err)
	}

	// Create user to remove
	userID := uuid.New().String()
	userEmail := "remove2-user-" + uuid.New().String()[:8] + "@example.com"
	_, err = pool.Exec(ctx, `
		INSERT INTO users (id, email, password_hash, name, created_at, updated_at)
		VALUES ($1, $2, 'hash', 'User', NOW(), NOW())
	`, userID, userEmail)
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	settings := DefaultSettings()
	settingsJSON, _ := json.Marshal(settings)

	tenantID := uuid.New().String()
	schemaName := "test_remove2_" + tenantID[:8]
	tenant := &Tenant{
		ID:         tenantID,
		Name:       "Remove User 2 Tenant",
		Slug:       "remove-user2-" + uuid.New().String()[:8],
		SchemaName: schemaName,
		IsActive:   true,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	err = repo.CreateTenant(ctx, tenant, settingsJSON, ownerID)
	if err != nil {
		t.Fatalf("CreateTenant failed: %v", err)
	}

	// Add user to tenant
	err = repo.AddUserToTenant(ctx, tenant.ID, userID, RoleAccountant)
	if err != nil {
		t.Fatalf("AddUserToTenant failed: %v", err)
	}

	// Remove using RemoveTenantUser (different from RemoveUserFromTenant)
	err = repo.RemoveTenantUser(ctx, tenant.ID, userID)
	if err != nil {
		t.Fatalf("RemoveTenantUser failed: %v", err)
	}

	// Verify user was removed
	isMember, err := repo.CheckUserIsMember(ctx, tenant.ID, userID)
	if err != nil {
		t.Fatalf("CheckUserIsMember failed: %v", err)
	}
	if isMember {
		t.Error("expected user to not be a member after removal")
	}
}
