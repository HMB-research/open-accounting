package tenant

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/HMB-research/open-accounting/internal/models"
)

func TestTenantModelMappingUsesStoredSettings(t *testing.T) {
	periodLockDate := "2026-05-31"
	createdAt := time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC)
	updatedAt := time.Date(2026, 6, 2, 11, 0, 0, 0, time.UTC)

	model := &models.Tenant{
		ID:         "tenant-1",
		Name:       "Demo OU",
		Slug:       "demo-ou",
		SchemaName: "tenant_demo_ou",
		Settings: json.RawMessage(`{
			"default_currency":"EUR",
			"country_code":"EE",
			"timezone":"Europe/Tallinn",
			"date_format":"YYYY-MM-DD",
			"decimal_sep":".",
			"thousands_sep":",",
			"fiscal_year_start_month":7,
			"period_lock_date":"2026-05-31",
			"vat_number":"EE123456789",
			"cash_flow_mapping":{"operating_account_codes":["1000"],"investing_account_codes":["1200"],"financing_account_codes":["2000"]}
		}`),
		IsActive:            true,
		OnboardingCompleted: true,
		CreatedAt:           createdAt,
		UpdatedAt:           updatedAt,
	}

	tenant := modelToTenant(model)

	if tenant.ID != model.ID ||
		tenant.Name != model.Name ||
		tenant.Slug != model.Slug ||
		tenant.SchemaName != model.SchemaName ||
		tenant.IsActive != model.IsActive ||
		tenant.OnboardingCompleted != model.OnboardingCompleted ||
		!tenant.CreatedAt.Equal(model.CreatedAt) ||
		!tenant.UpdatedAt.Equal(model.UpdatedAt) {
		t.Fatalf("modelToTenant() = %#v, want fields from %#v", tenant, model)
	}
	if tenant.Settings.DefaultCurrency != "EUR" ||
		tenant.Settings.CountryCode != "EE" ||
		tenant.Settings.Timezone != "Europe/Tallinn" ||
		tenant.Settings.FiscalYearStart != 7 ||
		tenant.Settings.PeriodLockDate == nil ||
		*tenant.Settings.PeriodLockDate != periodLockDate ||
		tenant.Settings.VATNumber != "EE123456789" ||
		tenant.Settings.CashFlowMapping == nil ||
		len(tenant.Settings.CashFlowMapping.OperatingAccountCodes) != 1 ||
		tenant.Settings.CashFlowMapping.OperatingAccountCodes[0] != "1000" {
		t.Fatalf("modelToTenant() settings = %#v, want stored settings", tenant.Settings)
	}
}

func TestTenantModelMappingFallsBackToDefaultSettings(t *testing.T) {
	tenant := modelToTenant(&models.Tenant{Settings: json.RawMessage(`{`)})
	defaults := DefaultSettings()

	if tenant.Settings.DefaultCurrency != defaults.DefaultCurrency ||
		tenant.Settings.CountryCode != defaults.CountryCode ||
		tenant.Settings.Timezone != defaults.Timezone ||
		tenant.Settings.FiscalYearStart != defaults.FiscalYearStart {
		t.Fatalf("modelToTenant() settings = %#v, want defaults %#v", tenant.Settings, defaults)
	}
}

func TestUserModelMapping(t *testing.T) {
	createdAt := time.Date(2026, 6, 3, 8, 0, 0, 0, time.UTC)
	updatedAt := time.Date(2026, 6, 4, 9, 0, 0, 0, time.UTC)
	model := &models.User{
		ID:           "user-1",
		Email:        "admin@example.com",
		PasswordHash: "hashed-password",
		Name:         "Admin User",
		IsActive:     true,
		CreatedAt:    createdAt,
		UpdatedAt:    updatedAt,
	}

	user := modelToUser(model)

	if user.ID != model.ID ||
		user.Email != model.Email ||
		user.PasswordHash != model.PasswordHash ||
		user.Name != model.Name ||
		user.IsActive != model.IsActive ||
		!user.CreatedAt.Equal(model.CreatedAt) ||
		!user.UpdatedAt.Equal(model.UpdatedAt) {
		t.Fatalf("modelToUser() = %#v, want fields from %#v", user, model)
	}
}

func TestUserInvitationModelMappings(t *testing.T) {
	acceptedAt := time.Date(2026, 6, 5, 14, 30, 0, 0, time.UTC)
	expiresAt := time.Date(2026, 6, 12, 14, 30, 0, 0, time.UTC)
	createdAt := time.Date(2026, 6, 5, 13, 0, 0, 0, time.UTC)
	model := &models.UserInvitation{
		ID:         "invitation-1",
		TenantID:   "tenant-1",
		Email:      "accountant@example.com",
		Role:       RoleAccountant,
		InvitedBy:  "owner-1",
		Token:      "token-1",
		ExpiresAt:  expiresAt,
		AcceptedAt: &acceptedAt,
		CreatedAt:  createdAt,
	}

	invitation := modelToUserInvitation(model)

	if invitation.ID != model.ID ||
		invitation.TenantID != model.TenantID ||
		invitation.Email != model.Email ||
		invitation.Role != model.Role ||
		invitation.InvitedBy != model.InvitedBy ||
		invitation.Token != model.Token ||
		!invitation.ExpiresAt.Equal(model.ExpiresAt) ||
		invitation.AcceptedAt != model.AcceptedAt ||
		!invitation.CreatedAt.Equal(model.CreatedAt) {
		t.Fatalf("modelToUserInvitation() = %#v, want fields from %#v", invitation, model)
	}

	roundTrip := userInvitationToModel(invitation)

	if roundTrip.ID != invitation.ID ||
		roundTrip.TenantID != invitation.TenantID ||
		roundTrip.Email != invitation.Email ||
		roundTrip.Role != invitation.Role ||
		roundTrip.InvitedBy != invitation.InvitedBy ||
		roundTrip.Token != invitation.Token ||
		!roundTrip.ExpiresAt.Equal(invitation.ExpiresAt) ||
		roundTrip.AcceptedAt != invitation.AcceptedAt ||
		!roundTrip.CreatedAt.Equal(invitation.CreatedAt) {
		t.Fatalf("userInvitationToModel() = %#v, want fields from %#v", roundTrip, invitation)
	}
}

func TestTenantAuditEventModelMappings(t *testing.T) {
	createdAt := time.Date(2026, 6, 6, 15, 0, 0, 0, time.UTC)
	event := &TenantAuditEvent{
		ID:          "audit-1",
		TenantID:    "tenant-1",
		ActorUserID: "user-1",
		Action:      AuditActionUserRoleUpdated,
		TargetType:  AuditTargetUser,
		TargetID:    "target-user-1",
		TargetEmail: "target@example.com",
		CreatedAt:   createdAt,
	}
	metadata := json.RawMessage(`{"role":"admin"}`)

	model := tenantAuditEventToModel(event, metadata)

	if model.ID != event.ID ||
		model.TenantID != event.TenantID ||
		model.ActorUserID == nil ||
		*model.ActorUserID != event.ActorUserID ||
		model.Action != event.Action ||
		model.TargetType != event.TargetType ||
		model.TargetID != event.TargetID ||
		model.TargetEmail == nil ||
		*model.TargetEmail != event.TargetEmail ||
		string(model.Metadata) != string(metadata) ||
		!model.CreatedAt.Equal(event.CreatedAt) {
		t.Fatalf("tenantAuditEventToModel() = %#v, want fields from %#v", model, event)
	}

	roundTrip := modelToTenantAuditEvent(model)

	if roundTrip.ID != event.ID ||
		roundTrip.TenantID != event.TenantID ||
		roundTrip.ActorUserID != event.ActorUserID ||
		roundTrip.Action != event.Action ||
		roundTrip.TargetType != event.TargetType ||
		roundTrip.TargetID != event.TargetID ||
		roundTrip.TargetEmail != event.TargetEmail ||
		!roundTrip.CreatedAt.Equal(event.CreatedAt) {
		t.Fatalf("modelToTenantAuditEvent() = %#v, want fields from %#v", roundTrip, model)
	}
}

func TestTenantAuditEventToModelDefaultsOptionalFieldsAndMetadata(t *testing.T) {
	event := &TenantAuditEvent{
		ID:          "audit-1",
		TenantID:    "tenant-1",
		ActorUserID: "   ",
		Action:      AuditActionInvitationRevoked,
		TargetType:  AuditTargetInvitation,
		TargetID:    "invitation-1",
	}

	model := tenantAuditEventToModel(event, nil)
	if model.ActorUserID != nil ||
		model.TargetEmail != nil ||
		string(model.Metadata) != `{}` {
		t.Fatalf("tenantAuditEventToModel() = %#v, want nil optional fields and empty object metadata", model)
	}

	model = tenantAuditEventToModel(event, json.RawMessage(`null`))
	if string(model.Metadata) != `{}` {
		t.Fatalf("tenantAuditEventToModel(null metadata) Metadata = %s, want {}", model.Metadata)
	}
}

func TestPeriodCloseEventModelMappings(t *testing.T) {
	lockBefore := "2026-04-30"
	lockAfter := "2026-05-31"
	createdAt := time.Date(2026, 6, 7, 16, 0, 0, 0, time.UTC)
	event := &PeriodCloseEvent{
		ID:              "period-close-1",
		TenantID:        "tenant-1",
		Action:          PeriodCloseActionClose,
		CloseKind:       PeriodCloseKindMonthEnd,
		PeriodEndDate:   "2026-05-31",
		LockDateBefore:  &lockBefore,
		LockDateAfter:   &lockAfter,
		Note:            "May closed",
		ReviewerSignOff: true,
		PerformedBy:     "user-1",
		CreatedAt:       createdAt,
	}

	model, err := periodCloseEventToModel(event)
	if err != nil {
		t.Fatalf("periodCloseEventToModel() error = %v", err)
	}

	if model.ID != event.ID ||
		model.TenantID != event.TenantID ||
		model.Action != event.Action ||
		model.CloseKind != event.CloseKind ||
		model.PeriodEndDate.Format(periodCloseDateLayout) != event.PeriodEndDate ||
		model.LockDateBefore == nil ||
		model.LockDateBefore.Format(periodCloseDateLayout) != lockBefore ||
		model.LockDateAfter == nil ||
		model.LockDateAfter.Format(periodCloseDateLayout) != lockAfter ||
		model.Note != event.Note ||
		model.ReviewerSignOff != event.ReviewerSignOff ||
		model.PerformedBy != event.PerformedBy ||
		!model.CreatedAt.Equal(event.CreatedAt) {
		t.Fatalf("periodCloseEventToModel() = %#v, want fields from %#v", model, event)
	}

	roundTrip := periodCloseEventFromModel(model)

	if roundTrip.ID != event.ID ||
		roundTrip.TenantID != event.TenantID ||
		roundTrip.Action != event.Action ||
		roundTrip.CloseKind != event.CloseKind ||
		roundTrip.PeriodEndDate != event.PeriodEndDate ||
		roundTrip.LockDateBefore == nil ||
		*roundTrip.LockDateBefore != lockBefore ||
		roundTrip.LockDateAfter == nil ||
		*roundTrip.LockDateAfter != lockAfter ||
		roundTrip.Note != event.Note ||
		roundTrip.ReviewerSignOff != event.ReviewerSignOff ||
		roundTrip.PerformedBy != event.PerformedBy ||
		!roundTrip.CreatedAt.Equal(event.CreatedAt) {
		t.Fatalf("periodCloseEventFromModel() = %#v, want fields from %#v", roundTrip, model)
	}
}

func TestPeriodCloseEventToModelRejectsInvalidDates(t *testing.T) {
	validPeriodEnd := "2026-05-31"
	invalidDate := "2026-99-99"
	tests := []struct {
		name  string
		event PeriodCloseEvent
	}{
		{
			name:  "period end date",
			event: PeriodCloseEvent{PeriodEndDate: invalidDate},
		},
		{
			name: "lock date before",
			event: PeriodCloseEvent{
				PeriodEndDate:  validPeriodEnd,
				LockDateBefore: &invalidDate,
			},
		},
		{
			name: "lock date after",
			event: PeriodCloseEvent{
				PeriodEndDate: validPeriodEnd,
				LockDateAfter: &invalidDate,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := periodCloseEventToModel(&tt.event); err == nil {
				t.Fatal("periodCloseEventToModel() error = nil, want invalid date error")
			}
		})
	}
}

func TestPeriodCloseEventModelTableName(t *testing.T) {
	var model periodCloseEventModel
	if got := model.TableName(); got != "tenant_period_closes" {
		t.Fatalf("TableName() = %q, want %q", got, "tenant_period_closes")
	}
}
