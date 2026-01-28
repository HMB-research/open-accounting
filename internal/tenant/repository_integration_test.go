//go:build integration

package tenant

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/HMB-research/open-accounting/internal/testutil"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Advisory lock key for test cleanup serialization (matches testutil)
const testCleanupAdvisoryLockKey = 12345678

// cleanupMutex serializes test tenant cleanup to prevent deadlocks
var cleanupMutex sync.Mutex

// createTestOwner creates a test user and registers cleanup.
// This ensures proper isolation between parallel tests.
func createTestOwner(t *testing.T, pool *pgxpool.Pool, emailPrefix string) string {
	t.Helper()
	ctx := context.Background()

	ownerID := uuid.New().String()
	ownerEmail := emailPrefix + "-" + uuid.New().String()[:8] + "@example.com"

	_, err := pool.Exec(ctx, `
		INSERT INTO public.users (id, email, password_hash, name, is_active, created_at, updated_at)
		VALUES ($1, $2, 'hash', 'Test Owner', true, NOW(), NOW())
	`, ownerID, ownerEmail)
	if err != nil {
		t.Fatalf("failed to create test owner: %v", err)
	}

	t.Cleanup(func() {
		cleanupTestUser(t, pool, ownerID)
	})

	return ownerID
}

// cleanupTestUser removes a test user and their tenant associations
func cleanupTestUser(t *testing.T, pool *pgxpool.Pool, userID string) {
	t.Helper()

	cleanupMutex.Lock()
	defer cleanupMutex.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	conn, err := pool.Acquire(ctx)
	if err != nil {
		t.Logf("warning: failed to acquire connection for user cleanup: %v", err)
		return
	}
	defer conn.Release()

	// Acquire advisory lock
	_, err = conn.Exec(ctx, "SELECT pg_advisory_lock($1)", testCleanupAdvisoryLockKey)
	if err != nil {
		t.Logf("warning: failed to acquire advisory lock: %v", err)
	}
	defer func() {
		_, _ = conn.Exec(ctx, "SELECT pg_advisory_unlock($1)", testCleanupAdvisoryLockKey)
	}()

	// Remove from tenant_users first (foreign key)
	_, _ = conn.Exec(ctx, "DELETE FROM public.tenant_users WHERE user_id = $1", userID)

	// Delete user
	_, err = conn.Exec(ctx, "DELETE FROM public.users WHERE id = $1", userID)
	if err != nil {
		t.Logf("warning: failed to delete test user %s: %v", userID, err)
	}
}

// cleanupTestTenantAndSchema removes a test tenant and its schema
func cleanupTestTenantAndSchema(t *testing.T, pool *pgxpool.Pool, tenantID, schemaName string) {
	t.Helper()

	cleanupMutex.Lock()
	defer cleanupMutex.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	conn, err := pool.Acquire(ctx)
	if err != nil {
		t.Logf("warning: failed to acquire connection for tenant cleanup: %v", err)
		return
	}
	defer conn.Release()

	// Acquire advisory lock
	_, err = conn.Exec(ctx, "SELECT pg_advisory_lock($1)", testCleanupAdvisoryLockKey)
	if err != nil {
		t.Logf("warning: failed to acquire advisory lock: %v", err)
	}
	defer func() {
		_, _ = conn.Exec(ctx, "SELECT pg_advisory_unlock($1)", testCleanupAdvisoryLockKey)
	}()

	// Drop schema first
	if schemaName != "" {
		_, _ = conn.Exec(ctx, "DROP SCHEMA IF EXISTS "+schemaName+" CASCADE")
	}

	// Delete tenant_users
	_, _ = conn.Exec(ctx, "DELETE FROM public.tenant_users WHERE tenant_id = $1", tenantID)

	// Delete invitations
	_, _ = conn.Exec(ctx, "DELETE FROM public.user_invitations WHERE tenant_id = $1", tenantID)

	// Delete tenant
	_, err = conn.Exec(ctx, "DELETE FROM public.tenants WHERE id = $1", tenantID)
	if err != nil {
		t.Logf("warning: failed to delete test tenant %s: %v", tenantID, err)
	}
}

func TestPostgresRepository_CreateAndGetTenant(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	ownerID := createTestOwner(t, pool, "owner")

	settings := DefaultSettings()
	settingsJSON, _ := json.Marshal(settings)

	tenantID := uuid.New().String()
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

	t.Cleanup(func() {
		cleanupTestTenantAndSchema(t, pool, tenantID, schemaName)
	})

	err := repo.CreateTenant(ctx, tenant, settingsJSON, ownerID)
	if err != nil {
		t.Fatalf("CreateTenant failed: %v", err)
	}

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

	ownerID := createTestOwner(t, pool, "slug-owner")

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

	t.Cleanup(func() {
		cleanupTestTenantAndSchema(t, pool, tenantID, schemaName)
	})

	err := repo.CreateTenant(ctx, tenant, settingsJSON, ownerID)
	if err != nil {
		t.Fatalf("CreateTenant failed: %v", err)
	}

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

	ownerID := createTestOwner(t, pool, "update-owner")

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

	t.Cleanup(func() {
		cleanupTestTenantAndSchema(t, pool, tenantID, schemaName)
	})

	err := repo.CreateTenant(ctx, tenant, settingsJSON, ownerID)
	if err != nil {
		t.Fatalf("CreateTenant failed: %v", err)
	}

	newName := "Updated Name"
	newSettings := DefaultSettings()
	newSettings.Email = "updated@company.com"
	newSettingsJSON, _ := json.Marshal(newSettings)

	err = repo.UpdateTenant(ctx, tenant.ID, newName, newSettingsJSON, time.Now())
	if err != nil {
		t.Fatalf("UpdateTenant failed: %v", err)
	}

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

	ownerID := createTestOwner(t, pool, "ops-owner")

	// Create second user
	userID := uuid.New().String()
	userEmail := "ops-user-" + uuid.New().String()[:8] + "@example.com"
	_, err := pool.Exec(ctx, `
		INSERT INTO public.users (id, email, password_hash, name, is_active, created_at, updated_at)
		VALUES ($1, $2, 'hash', 'User', true, NOW(), NOW())
	`, userID, userEmail)
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}
	t.Cleanup(func() {
		cleanupTestUser(t, pool, userID)
	})

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

	t.Cleanup(func() {
		cleanupTestTenantAndSchema(t, pool, tenantID, schemaName)
	})

	err = repo.CreateTenant(ctx, tenant, settingsJSON, ownerID)
	if err != nil {
		t.Fatalf("CreateTenant failed: %v", err)
	}

	err = repo.AddUserToTenant(ctx, tenant.ID, userID, RoleAccountant)
	if err != nil {
		t.Fatalf("AddUserToTenant failed: %v", err)
	}

	role, err := repo.GetUserRole(ctx, tenant.ID, userID)
	if err != nil {
		t.Fatalf("GetUserRole failed: %v", err)
	}
	if role != RoleAccountant {
		t.Errorf("expected role %s, got %s", RoleAccountant, role)
	}

	err = repo.UpdateTenantUserRole(ctx, tenant.ID, userID, RoleAdmin)
	if err != nil {
		t.Fatalf("UpdateTenantUserRole failed: %v", err)
	}

	role, err = repo.GetUserRole(ctx, tenant.ID, userID)
	if err != nil {
		t.Fatalf("GetUserRole failed: %v", err)
	}
	if role != RoleAdmin {
		t.Errorf("expected role %s, got %s", RoleAdmin, role)
	}

	users, err := repo.ListTenantUsers(ctx, tenant.ID)
	if err != nil {
		t.Fatalf("ListTenantUsers failed: %v", err)
	}
	if len(users) < 2 {
		t.Errorf("expected at least 2 users, got %d", len(users))
	}

	tenants, err := repo.ListUserTenants(ctx, userID)
	if err != nil {
		t.Fatalf("ListUserTenants failed: %v", err)
	}
	if len(tenants) < 1 {
		t.Errorf("expected at least 1 tenant, got %d", len(tenants))
	}

	err = repo.RemoveTenantUser(ctx, tenant.ID, userID)
	if err != nil {
		t.Fatalf("RemoveTenantUser failed: %v", err)
	}

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

	t.Cleanup(func() {
		cleanupTestUser(t, pool, user.ID)
	})

	err := repo.CreateUser(ctx, user)
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	byEmail, err := repo.GetUserByEmail(ctx, user.Email)
	if err != nil {
		t.Fatalf("GetUserByEmail failed: %v", err)
	}
	if byEmail.ID != user.ID {
		t.Errorf("expected ID %s, got %s", user.ID, byEmail.ID)
	}

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

	ownerID := createTestOwner(t, pool, "onboard-owner")

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

	t.Cleanup(func() {
		cleanupTestTenantAndSchema(t, pool, tenantID, schemaName)
	})

	err := repo.CreateTenant(ctx, tenant, settingsJSON, ownerID)
	if err != nil {
		t.Fatalf("CreateTenant failed: %v", err)
	}

	err = repo.CompleteOnboarding(ctx, tenant.ID)
	if err != nil {
		t.Fatalf("CompleteOnboarding failed: %v", err)
	}

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

	ownerID := createTestOwner(t, pool, "invite-owner")

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

	t.Cleanup(func() {
		cleanupTestTenantAndSchema(t, pool, tenantID, schemaName)
	})

	err := repo.CreateTenant(ctx, tenant, settingsJSON, ownerID)
	if err != nil {
		t.Fatalf("CreateTenant failed: %v", err)
	}

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

	retrieved, err := repo.GetInvitationByToken(ctx, token)
	if err != nil {
		t.Fatalf("GetInvitationByToken failed: %v", err)
	}
	if retrieved.Email != invitation.Email {
		t.Errorf("expected email %s, got %s", invitation.Email, retrieved.Email)
	}

	invitations, err := repo.ListInvitations(ctx, tenant.ID)
	if err != nil {
		t.Fatalf("ListInvitations failed: %v", err)
	}
	if len(invitations) < 1 {
		t.Errorf("expected at least 1 invitation, got %d", len(invitations))
	}

	err = repo.RevokeInvitation(ctx, tenant.ID, invitation.ID)
	if err != nil {
		t.Fatalf("RevokeInvitation failed: %v", err)
	}
}

func TestPostgresRepository_CheckUserIsMember(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	ownerEmail := "member-check-" + uuid.New().String()[:8] + "@example.com"
	ownerID := uuid.New().String()
	_, err := pool.Exec(ctx, `
		INSERT INTO public.users (id, email, password_hash, name, is_active, created_at, updated_at)
		VALUES ($1, $2, 'hash', 'Owner', true, NOW(), NOW())
	`, ownerID, ownerEmail)
	if err != nil {
		t.Fatalf("failed to create owner: %v", err)
	}
	t.Cleanup(func() {
		cleanupTestUser(t, pool, ownerID)
	})

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

	t.Cleanup(func() {
		cleanupTestTenantAndSchema(t, pool, tenantID, schemaName)
	})

	err = repo.CreateTenant(ctx, tenant, settingsJSON, ownerID)
	if err != nil {
		t.Fatalf("CreateTenant failed: %v", err)
	}

	isMember, err := repo.CheckUserIsMember(ctx, tenant.ID, ownerEmail)
	if err != nil {
		t.Fatalf("CheckUserIsMember failed: %v", err)
	}
	if !isMember {
		t.Error("expected owner to be a member")
	}

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

	ownerID := createTestOwner(t, pool, "delete-owner")

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

	err := repo.CreateTenant(ctx, tenant, settingsJSON, ownerID)
	if err != nil {
		t.Fatalf("CreateTenant failed: %v", err)
	}

	// Acquire advisory lock to prevent deadlocks with parallel test cleanup
	conn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("failed to acquire connection: %v", err)
	}
	defer conn.Release()

	_, err = conn.Exec(ctx, "SELECT pg_advisory_lock($1)", testCleanupAdvisoryLockKey)
	if err != nil {
		t.Fatalf("failed to acquire advisory lock: %v", err)
	}
	defer func() { _, _ = conn.Exec(ctx, "SELECT pg_advisory_unlock($1)", testCleanupAdvisoryLockKey) }()

	err = repo.DeleteTenant(ctx, tenant.ID, tenant.SchemaName)
	if err != nil {
		t.Fatalf("DeleteTenant failed: %v", err)
	}

	_, err = repo.GetTenant(ctx, tenant.ID)
	if err == nil {
		t.Error("expected error when getting deleted tenant")
	}
}

func TestPostgresRepository_RemoveUserFromTenant(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	ownerID := createTestOwner(t, pool, "remove-owner")

	// Create user to remove
	userID := uuid.New().String()
	userEmail := "remove-user-" + uuid.New().String()[:8] + "@example.com"
	_, err := pool.Exec(ctx, `
		INSERT INTO public.users (id, email, password_hash, name, is_active, created_at, updated_at)
		VALUES ($1, $2, 'hash', 'User', true, NOW(), NOW())
	`, userID, userEmail)
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}
	t.Cleanup(func() {
		cleanupTestUser(t, pool, userID)
	})

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

	t.Cleanup(func() {
		cleanupTestTenantAndSchema(t, pool, tenantID, schemaName)
	})

	err = repo.CreateTenant(ctx, tenant, settingsJSON, ownerID)
	if err != nil {
		t.Fatalf("CreateTenant failed: %v", err)
	}

	err = repo.AddUserToTenant(ctx, tenant.ID, userID, RoleAccountant)
	if err != nil {
		t.Fatalf("AddUserToTenant failed: %v", err)
	}

	err = repo.RemoveUserFromTenant(ctx, tenant.ID, userID)
	if err != nil {
		t.Fatalf("RemoveUserFromTenant failed: %v", err)
	}

	_, err = repo.GetUserRole(ctx, tenant.ID, userID)
	if err == nil {
		t.Error("expected error when getting role for removed user")
	}
}

func TestPostgresRepository_AcceptInvitation(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	ownerID := createTestOwner(t, pool, "accept-owner")

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

	t.Cleanup(func() {
		cleanupTestTenantAndSchema(t, pool, tenantID, schemaName)
	})

	err := repo.CreateTenant(ctx, tenant, settingsJSON, ownerID)
	if err != nil {
		t.Fatalf("CreateTenant failed: %v", err)
	}

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

	newUserID := uuid.New().String()
	t.Cleanup(func() {
		cleanupTestUser(t, pool, newUserID)
	})

	err = repo.AcceptInvitation(ctx, invitation, newUserID, "hashedpassword123", "New User", true)
	if err != nil {
		t.Fatalf("AcceptInvitation failed: %v", err)
	}

	role, err := repo.GetUserRole(ctx, tenant.ID, newUserID)
	if err != nil {
		t.Fatalf("GetUserRole failed: %v", err)
	}
	if role != RoleAccountant {
		t.Errorf("expected role %s, got %s", RoleAccountant, role)
	}

	retrieved, err := repo.GetInvitationByToken(ctx, token)
	if err != nil {
		t.Fatalf("GetInvitationByToken failed: %v", err)
	}
	if retrieved.AcceptedAt == nil {
		t.Error("expected AcceptedAt to be set after accepting invitation")
	}

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

	ownerID := createTestOwner(t, pool, "accept-existing-owner")

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

	t.Cleanup(func() {
		cleanupTestTenantAndSchema(t, pool, tenantID, schemaName)
	})

	err := repo.CreateTenant(ctx, tenant, settingsJSON, ownerID)
	if err != nil {
		t.Fatalf("CreateTenant failed: %v", err)
	}

	// Create existing user who will accept invitation
	existingUserID := uuid.New().String()
	existingEmail := "existing-user-" + uuid.New().String()[:8] + "@example.com"
	_, err = pool.Exec(ctx, `
		INSERT INTO public.users (id, email, password_hash, name, is_active, created_at, updated_at)
		VALUES ($1, $2, 'hash', 'Existing User', true, NOW(), NOW())
	`, existingUserID, existingEmail)
	if err != nil {
		t.Fatalf("failed to create existing user: %v", err)
	}
	t.Cleanup(func() {
		cleanupTestUser(t, pool, existingUserID)
	})

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

	err = repo.AcceptInvitation(ctx, invitation, existingUserID, "", "", false)
	if err != nil {
		t.Fatalf("AcceptInvitation failed: %v", err)
	}

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

	user1 := &User{
		ID:           uuid.New().String(),
		Email:        email,
		PasswordHash: "hashed_password",
		Name:         "First User",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	t.Cleanup(func() {
		cleanupTestUser(t, pool, user1.ID)
	})

	err := repo.CreateUser(ctx, user1)
	if err != nil {
		t.Fatalf("CreateUser (first) failed: %v", err)
	}

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

	t.Cleanup(func() {
		cleanupTestUser(t, pool, user.ID)
	})

	err := repo.CreateUser(ctx, user)
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

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

	err := repo.RemoveUserFromTenant(ctx, uuid.New().String(), uuid.New().String())
	if err != ErrUserNotInTenant {
		t.Errorf("expected ErrUserNotInTenant, got: %v", err)
	}
}

func TestPostgresRepository_RevokeInvitation_NotFound(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	err := repo.RevokeInvitation(ctx, uuid.New().String(), uuid.New().String())
	if err == nil {
		t.Error("expected error for non-existent invitation")
	}
	if err != nil && err.Error() != "invitation not found or already accepted" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestPostgresRepository_ListTenantUsers_Empty(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	ownerID := createTestOwner(t, pool, "owner-empty")

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

	t.Cleanup(func() {
		cleanupTestTenantAndSchema(t, pool, tenantID, schemaName)
	})

	err := repo.CreateTenant(ctx, tenant, settingsJSON, ownerID)
	if err != nil {
		t.Fatalf("CreateTenant failed: %v", err)
	}

	// Remove the owner from tenant_users to test empty list
	_, err = pool.Exec(ctx, "DELETE FROM public.tenant_users WHERE tenant_id = $1", tenantID)
	if err != nil {
		t.Fatalf("failed to remove users: %v", err)
	}

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

	_, err := repo.GetUserRole(ctx, uuid.New().String(), uuid.New().String())
	if err != ErrUserNotInTenant {
		t.Errorf("expected ErrUserNotInTenant, got: %v", err)
	}
}

func TestPostgresRepository_CheckUserIsMember_False(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	repo := NewPostgresRepository(pool)
	ctx := context.Background()

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

	ownerID := createTestOwner(t, pool, "role-owner")

	// Create user to update role
	userID := uuid.New().String()
	userEmail := "role-user-" + uuid.New().String()[:8] + "@example.com"
	_, err := pool.Exec(ctx, `
		INSERT INTO public.users (id, email, password_hash, name, is_active, created_at, updated_at)
		VALUES ($1, $2, 'hash', 'User', true, NOW(), NOW())
	`, userID, userEmail)
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}
	t.Cleanup(func() {
		cleanupTestUser(t, pool, userID)
	})

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

	t.Cleanup(func() {
		cleanupTestTenantAndSchema(t, pool, tenantID, schemaName)
	})

	err = repo.CreateTenant(ctx, tenant, settingsJSON, ownerID)
	if err != nil {
		t.Fatalf("CreateTenant failed: %v", err)
	}

	err = repo.AddUserToTenant(ctx, tenant.ID, userID, RoleAccountant)
	if err != nil {
		t.Fatalf("AddUserToTenant failed: %v", err)
	}

	err = repo.UpdateTenantUserRole(ctx, tenant.ID, userID, RoleAdmin)
	if err != nil {
		t.Fatalf("UpdateTenantUserRole failed: %v", err)
	}

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

	ownerID := createTestOwner(t, pool, "remove2-owner")

	// Create user to remove
	userID := uuid.New().String()
	userEmail := "remove2-user-" + uuid.New().String()[:8] + "@example.com"
	_, err := pool.Exec(ctx, `
		INSERT INTO public.users (id, email, password_hash, name, is_active, created_at, updated_at)
		VALUES ($1, $2, 'hash', 'User', true, NOW(), NOW())
	`, userID, userEmail)
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}
	t.Cleanup(func() {
		cleanupTestUser(t, pool, userID)
	})

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

	t.Cleanup(func() {
		cleanupTestTenantAndSchema(t, pool, tenantID, schemaName)
	})

	err = repo.CreateTenant(ctx, tenant, settingsJSON, ownerID)
	if err != nil {
		t.Fatalf("CreateTenant failed: %v", err)
	}

	err = repo.AddUserToTenant(ctx, tenant.ID, userID, RoleAccountant)
	if err != nil {
		t.Fatalf("AddUserToTenant failed: %v", err)
	}

	err = repo.RemoveTenantUser(ctx, tenant.ID, userID)
	if err != nil {
		t.Fatalf("RemoveTenantUser failed: %v", err)
	}

	isMember, err := repo.CheckUserIsMember(ctx, tenant.ID, userID)
	if err != nil {
		t.Fatalf("CheckUserIsMember failed: %v", err)
	}
	if isMember {
		t.Error("expected user to not be a member after removal")
	}
}
