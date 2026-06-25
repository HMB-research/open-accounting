package tenant

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestTenantWave8UpdateTenantRejectsInvalidInventoryPolicy(t *testing.T) {
	repo := NewMockRepository()
	repo.tenants["tenant-1"] = &Tenant{
		ID:       "tenant-1",
		Name:     "Acme",
		Settings: DefaultSettings(),
	}
	service := newTestServiceWithRepository(repo)

	_, err := service.UpdateTenant(context.Background(), "tenant-1", &UpdateTenantRequest{
		Settings: &TenantSettings{InventoryIssueCostingMethod: "unsupported"},
	})
	if err == nil || !strings.Contains(err.Error(), "inventory issue costing method") {
		t.Fatalf("UpdateTenant() error = %v, want invalid inventory policy", err)
	}
}

func TestTenantWave8AuditEventDefaultsMetadata(t *testing.T) {
	repo := NewMockRepository()
	service := newTestServiceWithRepository(repo)
	event := &TenantAuditEvent{
		TenantID:    "tenant-1",
		ActorUserID: "user-1",
		Action:      "settings.updated",
		TargetType:  "tenant",
		TargetID:    "tenant-1",
	}

	if err := service.RecordTenantAuditEvent(context.Background(), event); err != nil {
		t.Fatalf("RecordTenantAuditEvent() error = %v", err)
	}
	if event.ID == "" || event.CreatedAt.IsZero() {
		t.Fatalf("RecordTenantAuditEvent() did not default id/time: %#v", event)
	}
	if event.Metadata == nil {
		t.Fatalf("RecordTenantAuditEvent() metadata = nil, want empty map")
	}
}

func TestTenantWave8HashPasswordDefaultCostAndInvitationHashError(t *testing.T) {
	service := NewServiceWithRepository(NewMockRepository())
	hash, err := service.hashPassword("valid-password")
	if err != nil {
		t.Fatalf("hashPassword() default cost error = %v", err)
	}
	if !service.ValidatePassword(&User{PasswordHash: string(hash)}, "valid-password") {
		t.Fatalf("ValidatePassword() rejected hash produced with default cost")
	}

	repo := NewMockRepository()
	repo.invitations["token-1"] = &UserInvitation{
		ID:        "inv-1",
		TenantID:  "tenant-1",
		Email:     "invitee@example.com",
		Role:      RoleAccountant,
		Token:     "token-1",
		ExpiresAt: time.Now().AddDate(0, 0, 1),
	}
	repo.getUserByEmailErr = ErrUserNotFound
	service = newTestServiceWithRepository(repo)

	_, err = service.AcceptInvitation(context.Background(), &AcceptInvitationRequest{
		Token:    "token-1",
		Name:     "Invitee",
		Password: strings.Repeat("x", 73),
	})
	if err == nil || !strings.Contains(err.Error(), "hash password") {
		t.Fatalf("AcceptInvitation() long password error = %v, want hash error", err)
	}
}
