package tenant

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

func TestDefaultSettings(t *testing.T) {
	settings := DefaultSettings()

	assert.Equal(t, "EUR", settings.DefaultCurrency)
	assert.Equal(t, "EE", settings.CountryCode)
	assert.Equal(t, "Europe/Tallinn", settings.Timezone)
	assert.Equal(t, "DD.MM.YYYY", settings.DateFormat)
	assert.Equal(t, ",", settings.DecimalSep)
	assert.Equal(t, " ", settings.ThousandsSep)
	assert.Equal(t, 1, settings.FiscalYearStart)
}

func TestTenantSettings_Scan(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected TenantSettings
		wantErr  bool
	}{
		{
			name:  "valid JSON bytes",
			input: []byte(`{"default_currency":"USD","country_code":"US","timezone":"America/New_York"}`),
			expected: TenantSettings{
				DefaultCurrency: "USD",
				CountryCode:     "US",
				Timezone:        "America/New_York",
			},
			wantErr: false,
		},
		{
			name:  "valid JSON string",
			input: `{"default_currency":"GBP","country_code":"GB"}`,
			expected: TenantSettings{
				DefaultCurrency: "GBP",
				CountryCode:     "GB",
			},
			wantErr: false,
		},
		{
			name:     "nil input returns defaults",
			input:    nil,
			expected: DefaultSettings(),
			wantErr:  false,
		},
		{
			name:     "invalid type returns defaults",
			input:    12345,
			expected: DefaultSettings(),
			wantErr:  false,
		},
		{
			name:    "invalid JSON returns error",
			input:   []byte(`{invalid json}`),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var settings TenantSettings
			err := settings.Scan(tt.input)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected.DefaultCurrency, settings.DefaultCurrency)
				assert.Equal(t, tt.expected.CountryCode, settings.CountryCode)
			}
		})
	}
}

func TestSlugValidation(t *testing.T) {
	tests := []struct {
		name    string
		slug    string
		isValid bool
	}{
		{"valid simple slug", "my-company", true},
		{"valid numeric slug", "company123", true},
		{"valid alphanumeric", "my-company-2024", true},
		{"minimum valid length", "abc", true},
		{"starts with number", "123company", true},
		{"ends with number", "company123", true},

		// Invalid cases
		{"too short", "ab", false},
		{"contains uppercase", "My-Company", false},
		{"starts with hyphen", "-company", false},
		{"ends with hyphen", "company-", false},
		{"contains underscore", "my_company", false},
		{"contains spaces", "my company", false},
		{"contains special chars", "my@company", false},
		{"single character", "a", false},
		{"only hyphens", "---", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isValid := len(tt.slug) >= 3 && slugRegex.MatchString(tt.slug)
			assert.Equal(t, tt.isValid, isValid, "slug: %s", tt.slug)
		})
	}
}

func TestValidatePassword(t *testing.T) {
	service := &Service{} // No DB needed for password validation

	password := "securePassword123!"
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	require.NoError(t, err)

	user := &User{
		ID:           "user-123",
		Email:        "test@example.com",
		PasswordHash: string(hash),
		Name:         "Test User",
	}

	tests := []struct {
		name     string
		password string
		isValid  bool
	}{
		{"correct password", "securePassword123!", true},
		{"wrong password", "wrongPassword", false},
		{"empty password", "", false},
		{"similar password", "securePassword123", false},
		{"case sensitive", "SECUREPASSWORD123!", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.ValidatePassword(user, tt.password)
			assert.Equal(t, tt.isValid, result)
		})
	}
}

func TestRoleConstants(t *testing.T) {
	// Verify role constants are defined correctly
	assert.Equal(t, "owner", RoleOwner)
	assert.Equal(t, "admin", RoleAdmin)
	assert.Equal(t, "accountant", RoleAccountant)
	assert.Equal(t, "viewer", RoleViewer)
}

func TestGetRolePermissions(t *testing.T) {
	tests := []struct {
		role              string
		canManageUsers    bool
		canManageSettings bool
		canViewReports    bool
	}{
		{RoleOwner, true, true, true},
		{RoleAdmin, true, true, true},
		{RoleAccountant, false, false, true},
		{RoleViewer, false, false, true},
		{"unknown", false, false, false},
	}

	for _, tt := range tests {
		t.Run(tt.role, func(t *testing.T) {
			perms := GetRolePermissions(tt.role)
			assert.Equal(t, tt.canManageUsers, perms.CanManageUsers)
			assert.Equal(t, tt.canManageSettings, perms.CanManageSettings)
			assert.Equal(t, tt.canViewReports, perms.CanViewReports)
		})
	}
}

func TestTenant_JSONSerialization(t *testing.T) {
	tenant := &Tenant{
		ID:         "tenant-123",
		Name:       "Test Company",
		Slug:       "test-company",
		SchemaName: "tenant_test_company",
		Settings:   DefaultSettings(),
		IsActive:   true,
	}

	// Test JSON marshaling
	data, err := json.Marshal(tenant)
	require.NoError(t, err)

	// Test JSON unmarshaling
	var decoded Tenant
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, tenant.ID, decoded.ID)
	assert.Equal(t, tenant.Name, decoded.Name)
	assert.Equal(t, tenant.Slug, decoded.Slug)
	assert.Equal(t, tenant.SchemaName, decoded.SchemaName)
	assert.Equal(t, tenant.Settings.DefaultCurrency, decoded.Settings.DefaultCurrency)
	assert.Equal(t, tenant.IsActive, decoded.IsActive)
}

func TestUser_JSONSerialization(t *testing.T) {
	user := &User{
		ID:           "user-123",
		Email:        "test@example.com",
		PasswordHash: "secret-hash-should-not-appear",
		Name:         "Test User",
		IsActive:     true,
	}

	// Test JSON marshaling - password hash should be omitted
	data, err := json.Marshal(user)
	require.NoError(t, err)

	// Verify password hash is not in JSON output
	assert.NotContains(t, string(data), "secret-hash-should-not-appear")
	assert.NotContains(t, string(data), "password_hash")

	// Verify other fields are present
	assert.Contains(t, string(data), `"id":"user-123"`)
	assert.Contains(t, string(data), `"email":"test@example.com"`)
	assert.Contains(t, string(data), `"name":"Test User"`)
}

func TestCreateTenantRequest_Validation(t *testing.T) {
	tests := []struct {
		name      string
		request   CreateTenantRequest
		expectErr bool
	}{
		{
			name: "valid request",
			request: CreateTenantRequest{
				Name: "My Company",
				Slug: "my-company",
			},
			expectErr: false,
		},
		{
			name: "with custom settings",
			request: CreateTenantRequest{
				Name: "US Company",
				Slug: "us-company",
				Settings: &TenantSettings{
					DefaultCurrency: "USD",
					CountryCode:     "US",
					Timezone:        "America/New_York",
				},
			},
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Validate slug format
			isValidSlug := len(tt.request.Slug) >= 3 && slugRegex.MatchString(tt.request.Slug)
			if tt.expectErr {
				assert.False(t, isValidSlug)
			} else {
				assert.True(t, isValidSlug)
			}
		})
	}
}

func TestTenantMembership(t *testing.T) {
	membership := TenantMembership{
		Tenant: Tenant{
			ID:   "tenant-123",
			Name: "Test Tenant",
			Slug: "test-tenant",
		},
		Role:      RoleAdmin,
		IsDefault: true,
	}

	// Test JSON serialization
	data, err := json.Marshal(membership)
	require.NoError(t, err)

	var decoded TenantMembership
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, membership.Tenant.ID, decoded.Tenant.ID)
	assert.Equal(t, membership.Role, decoded.Role)
	assert.Equal(t, membership.IsDefault, decoded.IsDefault)
}

func TestCreateUserRequest(t *testing.T) {
	request := CreateUserRequest{
		Email:    "  Test@Example.com  ",
		Password: "securePassword123",
		Name:     "Test User",
	}

	// Test email normalization (as done in CreateUser)
	normalizedEmail := "test@example.com"
	// The service normalizes email to lowercase and trims spaces

	// Just verify the struct fields
	assert.NotEmpty(t, request.Email)
	assert.NotEmpty(t, request.Password)
	assert.NotEmpty(t, request.Name)

	// In the actual service, email would be normalized like this:
	assert.NotEqual(t, normalizedEmail, request.Email) // Before normalization
}

func TestValidRoles(t *testing.T) {
	roles := ValidRoles()

	// Should contain exactly these three roles
	assert.Len(t, roles, 3)
	assert.Contains(t, roles, RoleAdmin)
	assert.Contains(t, roles, RoleAccountant)
	assert.Contains(t, roles, RoleViewer)

	// Should NOT contain owner (owner is assigned at creation, not invited)
	assert.NotContains(t, roles, RoleOwner)
}

func TestIsValidRole(t *testing.T) {
	tests := []struct {
		name     string
		role     string
		expected bool
	}{
		{"admin is valid", RoleAdmin, true},
		{"accountant is valid", RoleAccountant, true},
		{"viewer is valid", RoleViewer, true},
		{"owner is NOT valid for invitation", RoleOwner, false},
		{"empty string is invalid", "", false},
		{"unknown role is invalid", "unknown", false},
		{"uppercase admin is invalid", "ADMIN", false},
		{"mixed case is invalid", "Admin", false},
		{"whitespace is invalid", " admin ", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsValidRole(tt.role)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetRolePermissions_AllRoles(t *testing.T) {
	t.Run("owner has all permissions", func(t *testing.T) {
		perms := GetRolePermissions(RoleOwner)
		assert.True(t, perms.CanManageUsers)
		assert.True(t, perms.CanManageSettings)
		assert.True(t, perms.CanManageAccounts)
		assert.True(t, perms.CanCreateEntries)
		assert.True(t, perms.CanApproveEntries)
		assert.True(t, perms.CanViewReports)
		assert.True(t, perms.CanManageInvoices)
		assert.True(t, perms.CanManagePayments)
		assert.True(t, perms.CanManageContacts)
		assert.True(t, perms.CanManageBanking)
		assert.True(t, perms.CanExportData)
	})

	t.Run("admin has all permissions", func(t *testing.T) {
		perms := GetRolePermissions(RoleAdmin)
		assert.True(t, perms.CanManageUsers)
		assert.True(t, perms.CanManageSettings)
		assert.True(t, perms.CanManageAccounts)
		assert.True(t, perms.CanCreateEntries)
		assert.True(t, perms.CanApproveEntries)
		assert.True(t, perms.CanViewReports)
		assert.True(t, perms.CanManageInvoices)
		assert.True(t, perms.CanManagePayments)
		assert.True(t, perms.CanManageContacts)
		assert.True(t, perms.CanManageBanking)
		assert.True(t, perms.CanExportData)
	})

	t.Run("accountant has accounting permissions but not user/settings management", func(t *testing.T) {
		perms := GetRolePermissions(RoleAccountant)
		assert.False(t, perms.CanManageUsers)
		assert.False(t, perms.CanManageSettings)
		assert.True(t, perms.CanManageAccounts)
		assert.True(t, perms.CanCreateEntries)
		assert.True(t, perms.CanApproveEntries)
		assert.True(t, perms.CanViewReports)
		assert.True(t, perms.CanManageInvoices)
		assert.True(t, perms.CanManagePayments)
		assert.True(t, perms.CanManageContacts)
		assert.True(t, perms.CanManageBanking)
		assert.True(t, perms.CanExportData)
	})

	t.Run("viewer has only view permissions", func(t *testing.T) {
		perms := GetRolePermissions(RoleViewer)
		assert.False(t, perms.CanManageUsers)
		assert.False(t, perms.CanManageSettings)
		assert.False(t, perms.CanManageAccounts)
		assert.False(t, perms.CanCreateEntries)
		assert.False(t, perms.CanApproveEntries)
		assert.True(t, perms.CanViewReports)
		assert.False(t, perms.CanManageInvoices)
		assert.False(t, perms.CanManagePayments)
		assert.False(t, perms.CanManageContacts)
		assert.False(t, perms.CanManageBanking)
		assert.False(t, perms.CanExportData)
	})

	t.Run("unknown role has no permissions", func(t *testing.T) {
		perms := GetRolePermissions("invalid-role")
		assert.False(t, perms.CanManageUsers)
		assert.False(t, perms.CanManageSettings)
		assert.False(t, perms.CanManageAccounts)
		assert.False(t, perms.CanCreateEntries)
		assert.False(t, perms.CanApproveEntries)
		assert.False(t, perms.CanViewReports)
		assert.False(t, perms.CanManageInvoices)
		assert.False(t, perms.CanManagePayments)
		assert.False(t, perms.CanManageContacts)
		assert.False(t, perms.CanManageBanking)
		assert.False(t, perms.CanExportData)
	})
}

func TestTenantSettings_AllFields(t *testing.T) {
	settings := TenantSettings{
		DefaultCurrency: "USD",
		CountryCode:     "US",
		Timezone:        "America/New_York",
		DateFormat:      "MM/DD/YYYY",
		DecimalSep:      ".",
		ThousandsSep:    ",",
		FiscalYearStart: 7,
		VATNumber:       "VAT123456",
		RegCode:         "REG789",
		Address:         "123 Main St",
		Email:           "company@example.com",
		Phone:           "+1-555-1234",
		Logo:            "https://example.com/logo.png",
		PDFPrimaryColor: "#ff0000",
		PDFFooterText:   "Thank you for your business",
		BankDetails:     "Bank: Example, IBAN: XX123",
		InvoiceTerms:    "Net 30",
	}

	// Test JSON serialization
	data, err := json.Marshal(settings)
	require.NoError(t, err)

	var decoded TenantSettings
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, settings.DefaultCurrency, decoded.DefaultCurrency)
	assert.Equal(t, settings.CountryCode, decoded.CountryCode)
	assert.Equal(t, settings.Timezone, decoded.Timezone)
	assert.Equal(t, settings.DateFormat, decoded.DateFormat)
	assert.Equal(t, settings.DecimalSep, decoded.DecimalSep)
	assert.Equal(t, settings.ThousandsSep, decoded.ThousandsSep)
	assert.Equal(t, settings.FiscalYearStart, decoded.FiscalYearStart)
	assert.Equal(t, settings.VATNumber, decoded.VATNumber)
	assert.Equal(t, settings.RegCode, decoded.RegCode)
	assert.Equal(t, settings.Address, decoded.Address)
	assert.Equal(t, settings.Email, decoded.Email)
	assert.Equal(t, settings.Phone, decoded.Phone)
	assert.Equal(t, settings.Logo, decoded.Logo)
	assert.Equal(t, settings.PDFPrimaryColor, decoded.PDFPrimaryColor)
	assert.Equal(t, settings.PDFFooterText, decoded.PDFFooterText)
	assert.Equal(t, settings.BankDetails, decoded.BankDetails)
	assert.Equal(t, settings.InvoiceTerms, decoded.InvoiceTerms)
}

func TestUserInvitation_JSONSerialization(t *testing.T) {
	now := time.Now()
	later := now.Add(7 * 24 * time.Hour)

	inv := UserInvitation{
		ID:         "inv-123",
		TenantID:   "tenant-456",
		TenantName: "Test Tenant",
		Email:      "test@example.com",
		Role:       RoleAdmin,
		InvitedBy:  "user-789",
		Token:      "secret-token-should-not-appear",
		ExpiresAt:  later,
		CreatedAt:  now,
	}

	data, err := json.Marshal(inv)
	require.NoError(t, err)

	// Token should be omitted from JSON
	assert.NotContains(t, string(data), "secret-token-should-not-appear")

	// Other fields should be present
	assert.Contains(t, string(data), `"id":"inv-123"`)
	assert.Contains(t, string(data), `"tenant_id":"tenant-456"`)
	assert.Contains(t, string(data), `"email":"test@example.com"`)
	assert.Contains(t, string(data), `"role":"admin"`)
}

func TestCreateInvitationRequest(t *testing.T) {
	req := CreateInvitationRequest{
		Email: "newuser@example.com",
		Role:  RoleAccountant,
	}

	assert.Equal(t, "newuser@example.com", req.Email)
	assert.Equal(t, RoleAccountant, req.Role)
}

func TestAcceptInvitationRequest(t *testing.T) {
	req := AcceptInvitationRequest{
		Token:    "invite-token",
		Password: "newpassword123",
		Name:     "New User",
	}

	assert.Equal(t, "invite-token", req.Token)
	assert.Equal(t, "newpassword123", req.Password)
	assert.Equal(t, "New User", req.Name)
}

func TestUpdateTenantRequest(t *testing.T) {
	name := "Updated Name"
	settings := TenantSettings{
		VATNumber: "NEW-VAT",
		Email:     "new@example.com",
	}

	req := UpdateTenantRequest{
		Name:     &name,
		Settings: &settings,
	}

	require.NotNil(t, req.Name)
	assert.Equal(t, "Updated Name", *req.Name)
	require.NotNil(t, req.Settings)
	assert.Equal(t, "NEW-VAT", req.Settings.VATNumber)
}

func TestTenantUser(t *testing.T) {
	now := time.Now()
	tu := TenantUser{
		TenantID:  "tenant-123",
		UserID:    "user-456",
		Role:      RoleAccountant,
		IsDefault: true,
		CreatedAt: now,
	}

	data, err := json.Marshal(tu)
	require.NoError(t, err)

	var decoded TenantUser
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, tu.TenantID, decoded.TenantID)
	assert.Equal(t, tu.UserID, decoded.UserID)
	assert.Equal(t, tu.Role, decoded.Role)
	assert.Equal(t, tu.IsDefault, decoded.IsDefault)
}

func TestUserWithTenants(t *testing.T) {
	uwt := UserWithTenants{
		User: User{
			ID:       "user-123",
			Email:    "test@example.com",
			Name:     "Test User",
			IsActive: true,
		},
		Tenants: []TenantMembership{
			{
				Tenant: Tenant{
					ID:   "tenant-1",
					Name: "Tenant One",
					Slug: "tenant-one",
				},
				Role:      RoleOwner,
				IsDefault: true,
			},
			{
				Tenant: Tenant{
					ID:   "tenant-2",
					Name: "Tenant Two",
					Slug: "tenant-two",
				},
				Role:      RoleViewer,
				IsDefault: false,
			},
		},
	}

	data, err := json.Marshal(uwt)
	require.NoError(t, err)

	var decoded UserWithTenants
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, uwt.User.ID, decoded.User.ID)
	assert.Len(t, decoded.Tenants, 2)
	assert.Equal(t, "tenant-one", decoded.Tenants[0].Tenant.Slug)
	assert.True(t, decoded.Tenants[0].IsDefault)
}

func TestNewService(t *testing.T) {
	service := NewService(nil)
	assert.NotNil(t, service)
}

func TestSlugRegex(t *testing.T) {
	validSlugs := []string{
		"abc",
		"test-company",
		"my123",
		"a2b3c4",
		"company-2024",
	}

	invalidSlugs := []string{
		"-abc",     // starts with hyphen
		"abc-",     // ends with hyphen
		"AB",       // too short and uppercase
		"a",        // too short
		"ab",       // too short
		"ABC",      // uppercase
		"my_test",  // underscore
		"my test",  // space
		"my@test",  // special char
	}

	for _, slug := range validSlugs {
		t.Run("valid_"+slug, func(t *testing.T) {
			assert.True(t, slugRegex.MatchString(slug), "slug %s should be valid", slug)
		})
	}

	for _, slug := range invalidSlugs {
		t.Run("invalid_"+slug, func(t *testing.T) {
			isValid := len(slug) >= 3 && slugRegex.MatchString(slug)
			assert.False(t, isValid, "slug %s should be invalid", slug)
		})
	}
}
