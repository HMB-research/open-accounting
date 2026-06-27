package tenant

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestGORMRepositoryNilDatabaseGuards(t *testing.T) {
	repo := NewGORMRepository(nil)
	ctx := context.Background()
	now := time.Date(2026, 6, 24, 12, 0, 0, 0, time.UTC)
	tenantValue := &Tenant{
		ID:         "tenant-1",
		Name:       "Demo OU",
		Slug:       "demo-ou",
		SchemaName: "tenant_demo_ou",
		Settings:   DefaultSettings(),
		IsActive:   true,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	userValue := &User{
		ID:           "user-1",
		Email:        "user@example.com",
		PasswordHash: "hashed-password",
		Name:         "Test User",
		IsActive:     true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	invitationValue := &UserInvitation{
		ID:        "invitation-1",
		TenantID:  "tenant-1",
		Email:     "invitee@example.com",
		Role:      RoleAccountant,
		InvitedBy: "owner-1",
		Token:     "token-1",
		ExpiresAt: now.Add(24 * time.Hour),
		CreatedAt: now,
	}
	auditEvent := &TenantAuditEvent{
		ID:         "audit-1",
		TenantID:   "tenant-1",
		Action:     AuditActionInvitationCreated,
		TargetType: AuditTargetInvitation,
		TargetID:   "invitation-1",
		Metadata:   map[string]string{},
		CreatedAt:  now,
	}
	periodCloseEvent := &PeriodCloseEvent{
		ID:            "period-close-1",
		TenantID:      "tenant-1",
		Action:        PeriodCloseActionClose,
		CloseKind:     PeriodCloseKindMonthEnd,
		PeriodEndDate: "2026-05-31",
		PerformedBy:   "user-1",
		CreatedAt:     now,
	}

	tests := []struct {
		name string
		run  func(t *testing.T) error
	}{
		{
			name: "CreateTenant",
			run: func(t *testing.T) error {
				return repo.CreateTenant(ctx, tenantValue, []byte(`{}`), "owner-1")
			},
		},
		{
			name: "GetTenant",
			run: func(t *testing.T) error {
				tenant, err := repo.GetTenant(ctx, "tenant-1")
				if tenant != nil {
					t.Fatalf("GetTenant() tenant = %#v, want nil", tenant)
				}
				return err
			},
		},
		{
			name: "GetTenantBySlug",
			run: func(t *testing.T) error {
				tenant, err := repo.GetTenantBySlug(ctx, "demo-ou")
				if tenant != nil {
					t.Fatalf("GetTenantBySlug() tenant = %#v, want nil", tenant)
				}
				return err
			},
		},
		{
			name: "UpdateTenant",
			run: func(t *testing.T) error {
				return repo.UpdateTenant(ctx, "tenant-1", "Demo OU", []byte(`{}`), now)
			},
		},
		{
			name: "UpdateTenantWithPeriodCloseEvent",
			run: func(t *testing.T) error {
				return repo.UpdateTenantWithPeriodCloseEvent(ctx, "tenant-1", "Demo OU", []byte(`{}`), now, periodCloseEvent)
			},
		},
		{
			name: "DeleteTenant",
			run: func(t *testing.T) error {
				return repo.DeleteTenant(ctx, "tenant-1", "tenant_demo_ou")
			},
		},
		{
			name: "CompleteOnboarding",
			run: func(t *testing.T) error {
				return repo.CompleteOnboarding(ctx, "tenant-1")
			},
		},
		{
			name: "ListPeriodCloseEvents",
			run: func(t *testing.T) error {
				events, err := repo.ListPeriodCloseEvents(ctx, "tenant-1", 20)
				if events != nil {
					t.Fatalf("ListPeriodCloseEvents() events = %#v, want nil", events)
				}
				return err
			},
		},
		{
			name: "GetLatestCloseEventForPeriod",
			run: func(t *testing.T) error {
				event, err := repo.GetLatestCloseEventForPeriod(ctx, "tenant-1", "2026-05-31")
				if event != nil {
					t.Fatalf("GetLatestCloseEventForPeriod() event = %#v, want nil", event)
				}
				return err
			},
		},
		{
			name: "CreateTenantAuditEvent",
			run: func(t *testing.T) error {
				return repo.CreateTenantAuditEvent(ctx, auditEvent)
			},
		},
		{
			name: "ListTenantAuditEvents",
			run: func(t *testing.T) error {
				events, err := repo.ListTenantAuditEvents(ctx, "tenant-1", 50)
				if events != nil {
					t.Fatalf("ListTenantAuditEvents() events = %#v, want nil", events)
				}
				return err
			},
		},
		{
			name: "AddUserToTenant",
			run: func(t *testing.T) error {
				return repo.AddUserToTenant(ctx, "tenant-1", "user-1", RoleAccountant)
			},
		},
		{
			name: "RemoveUserFromTenant",
			run: func(t *testing.T) error {
				return repo.RemoveUserFromTenant(ctx, "tenant-1", "user-1")
			},
		},
		{
			name: "GetUserRole",
			run: func(t *testing.T) error {
				role, err := repo.GetUserRole(ctx, "tenant-1", "user-1")
				if role != "" {
					t.Fatalf("GetUserRole() role = %q, want empty", role)
				}
				return err
			},
		},
		{
			name: "GetTenantUser",
			run: func(t *testing.T) error {
				user, err := repo.GetTenantUser(ctx, "tenant-1", "user-1")
				if user != nil {
					t.Fatalf("GetTenantUser() user = %#v, want nil", user)
				}
				return err
			},
		},
		{
			name: "ListUserTenants",
			run: func(t *testing.T) error {
				memberships, err := repo.ListUserTenants(ctx, "user-1")
				if memberships != nil {
					t.Fatalf("ListUserTenants() memberships = %#v, want nil", memberships)
				}
				return err
			},
		},
		{
			name: "ListTenantUsers",
			run: func(t *testing.T) error {
				users, err := repo.ListTenantUsers(ctx, "tenant-1")
				if users != nil {
					t.Fatalf("ListTenantUsers() users = %#v, want nil", users)
				}
				return err
			},
		},
		{
			name: "UpdateTenantUserRole",
			run: func(t *testing.T) error {
				return repo.UpdateTenantUserRole(ctx, "tenant-1", "user-1", RoleAdmin)
			},
		},
		{
			name: "SetTenantUserActive",
			run: func(t *testing.T) error {
				return repo.SetTenantUserActive(ctx, "tenant-1", "user-1", false)
			},
		},
		{
			name: "RemoveTenantUser",
			run: func(t *testing.T) error {
				return repo.RemoveTenantUser(ctx, "tenant-1", "user-1")
			},
		},
		{
			name: "CreateUser",
			run: func(t *testing.T) error {
				return repo.CreateUser(ctx, userValue)
			},
		},
		{
			name: "GetUserByEmail",
			run: func(t *testing.T) error {
				user, err := repo.GetUserByEmail(ctx, "user@example.com")
				if user != nil {
					t.Fatalf("GetUserByEmail() user = %#v, want nil", user)
				}
				return err
			},
		},
		{
			name: "GetUserByID",
			run: func(t *testing.T) error {
				user, err := repo.GetUserByID(ctx, "user-1")
				if user != nil {
					t.Fatalf("GetUserByID() user = %#v, want nil", user)
				}
				return err
			},
		},
		{
			name: "UpdateUserPassword",
			run: func(t *testing.T) error {
				return repo.UpdateUserPassword(ctx, "user-1", "new-hash", now)
			},
		},
		{
			name: "CreateInvitation",
			run: func(t *testing.T) error {
				return repo.CreateInvitation(ctx, invitationValue)
			},
		},
		{
			name: "GetInvitationByToken",
			run: func(t *testing.T) error {
				invitation, err := repo.GetInvitationByToken(ctx, "token-1")
				if invitation != nil {
					t.Fatalf("GetInvitationByToken() invitation = %#v, want nil", invitation)
				}
				return err
			},
		},
		{
			name: "AcceptInvitation",
			run: func(t *testing.T) error {
				return repo.AcceptInvitation(ctx, invitationValue, "user-1", "hashed-password", "Test User", true)
			},
		},
		{
			name: "ListInvitations",
			run: func(t *testing.T) error {
				invitations, err := repo.ListInvitations(ctx, "tenant-1")
				if invitations != nil {
					t.Fatalf("ListInvitations() invitations = %#v, want nil", invitations)
				}
				return err
			},
		},
		{
			name: "RevokeInvitation",
			run: func(t *testing.T) error {
				return repo.RevokeInvitation(ctx, "tenant-1", "invitation-1")
			},
		},
		{
			name: "CheckUserIsMember",
			run: func(t *testing.T) error {
				exists, err := repo.CheckUserIsMember(ctx, "tenant-1", "user@example.com")
				if exists {
					t.Fatal("CheckUserIsMember() exists = true, want false")
				}
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.run(t)
			requireTenantRepositoryNotConfiguredError(t, err)
		})
	}
}

func TestNewRepositoryNilPoolReturnsGuardedRepository(t *testing.T) {
	repo := NewRepository(nil)
	if repo == nil {
		t.Fatal("NewRepository(nil) = nil, want repository")
	}
	if repo.db != nil {
		t.Fatalf("NewRepository(nil).db = %#v, want nil", repo.db)
	}

	_, err := repo.GetTenant(context.Background(), "tenant-1")
	requireTenantRepositoryNotConfiguredError(t, err)
}

func requireTenantRepositoryNotConfiguredError(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "tenant repository database is not configured") {
		t.Fatalf("error = %q, want tenant repository database is not configured", err)
	}
}
