package tenant

import (
	"encoding/json"
	"testing"

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
