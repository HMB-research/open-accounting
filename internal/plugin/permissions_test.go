package plugin

import (
	"testing"
)

func TestValidatePermission(t *testing.T) {
	tests := []struct {
		name       string
		permission string
		expected   bool
	}{
		// Valid permissions
		{"Valid contacts:read", "contacts:read", true},
		{"Valid contacts:write", "contacts:write", true},
		{"Valid invoices:read", "invoices:read", true},
		{"Valid invoices:write", "invoices:write", true},
		{"Valid payments:read", "payments:read", true},
		{"Valid payments:write", "payments:write", true},
		{"Valid accounts:read", "accounts:read", true},
		{"Valid accounts:write", "accounts:write", true},
		{"Valid employees:read", "employees:read", true},
		{"Valid employees:write", "employees:write", true},
		{"Valid payroll:read", "payroll:read", true},
		{"Valid payroll:write", "payroll:write", true},
		{"Valid banking:read", "banking:read", true},
		{"Valid banking:write", "banking:write", true},
		{"Valid email:send", "email:send", true},
		{"Valid storage:read", "storage:read", true},
		{"Valid storage:write", "storage:write", true},
		{"Valid pdf:generate", "pdf:generate", true},
		{"Valid settings:read", "settings:read", true},
		{"Valid settings:write", "settings:write", true},
		{"Valid database:migrate", "database:migrate", true},
		{"Valid database:query", "database:query", true},
		{"Valid hooks:register", "hooks:register", true},
		{"Valid routes:register", "routes:register", true},
		{"Valid admin:access", "admin:access", true},

		// Invalid permissions
		{"Invalid empty", "", false},
		{"Invalid unknown", "unknown:permission", false},
		{"Invalid typo", "contact:read", false},
		{"Invalid case", "CONTACTS:READ", false},
		{"Invalid format", "contacts", false},
		{"Invalid special chars", "contacts:read!", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidatePermission(tt.permission)
			if result != tt.expected {
				t.Errorf("ValidatePermission(%q) = %v, want %v", tt.permission, result, tt.expected)
			}
		})
	}
}

func TestValidatePermissions(t *testing.T) {
	tests := []struct {
		name        string
		permissions []string
		wantInvalid []string
	}{
		{
			name:        "All valid",
			permissions: []string{"contacts:read", "invoices:write", "email:send"},
			wantInvalid: nil,
		},
		{
			name:        "One invalid",
			permissions: []string{"contacts:read", "invalid:perm", "email:send"},
			wantInvalid: []string{"invalid:perm"},
		},
		{
			name:        "Multiple invalid",
			permissions: []string{"bad:one", "contacts:read", "bad:two"},
			wantInvalid: []string{"bad:one", "bad:two"},
		},
		{
			name:        "Empty list",
			permissions: []string{},
			wantInvalid: nil,
		},
		{
			name:        "Nil list",
			permissions: nil,
			wantInvalid: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidatePermissions(tt.permissions)
			if len(result) != len(tt.wantInvalid) {
				t.Errorf("ValidatePermissions() returned %d invalid, want %d", len(result), len(tt.wantInvalid))
				return
			}
			for i, inv := range result {
				if inv != tt.wantInvalid[i] {
					t.Errorf("ValidatePermissions() invalid[%d] = %q, want %q", i, inv, tt.wantInvalid[i])
				}
			}
		})
	}
}

func TestGetPermission(t *testing.T) {
	tests := []struct {
		name       string
		permission string
		wantExists bool
		wantRisk   PermissionRisk
	}{
		{"Low risk permission", "contacts:read", true, RiskLow},
		{"Medium risk permission", "invoices:write", true, RiskMedium},
		{"High risk permission", "payroll:write", true, RiskHigh},
		{"Critical risk permission", "hooks:register", true, RiskCritical},
		{"Non-existent", "fake:permission", false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			perm, exists := GetPermission(tt.permission)
			if exists != tt.wantExists {
				t.Errorf("GetPermission(%q) exists = %v, want %v", tt.permission, exists, tt.wantExists)
			}
			if exists && perm.Risk != tt.wantRisk {
				t.Errorf("GetPermission(%q) risk = %v, want %v", tt.permission, perm.Risk, tt.wantRisk)
			}
		})
	}
}

func TestGetPermissionsByCategory(t *testing.T) {
	tests := []struct {
		name     string
		category PermissionCategory
		minCount int
	}{
		{"Data access permissions", CategoryDataAccess, 10},
		{"System permissions", CategorySystem, 5},
		{"Database permissions", CategoryDatabase, 2},
		{"Dangerous permissions", CategoryDangerous, 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			perms := GetPermissionsByCategory(tt.category)
			if len(perms) < tt.minCount {
				t.Errorf("GetPermissionsByCategory(%q) returned %d permissions, want at least %d",
					tt.category, len(perms), tt.minCount)
			}
			for _, p := range perms {
				if p.Category != tt.category {
					t.Errorf("GetPermissionsByCategory(%q) returned permission with category %q",
						tt.category, p.Category)
				}
			}
		})
	}
}

func TestGetPermissionsByRisk(t *testing.T) {
	tests := []struct {
		name    string
		minRisk PermissionRisk
	}{
		{"Low risk minimum", RiskLow},
		{"Medium risk minimum", RiskMedium},
		{"High risk minimum", RiskHigh},
		{"Critical risk minimum", RiskCritical},
	}

	riskOrder := map[PermissionRisk]int{
		RiskLow:      1,
		RiskMedium:   2,
		RiskHigh:     3,
		RiskCritical: 4,
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			perms := GetPermissionsByRisk(tt.minRisk)
			for _, p := range perms {
				if riskOrder[p.Risk] < riskOrder[tt.minRisk] {
					t.Errorf("GetPermissionsByRisk(%q) returned permission %q with lower risk %q",
						tt.minRisk, p.Name, p.Risk)
				}
			}
		})
	}
}

func TestHasDangerousPermissions(t *testing.T) {
	tests := []struct {
		name        string
		permissions []string
		expected    bool
	}{
		{
			name:        "No dangerous permissions",
			permissions: []string{"contacts:read", "invoices:write", "email:send"},
			expected:    false,
		},
		{
			name:        "Has hooks:register",
			permissions: []string{"contacts:read", "hooks:register"},
			expected:    true,
		},
		{
			name:        "Has routes:register",
			permissions: []string{"routes:register", "invoices:read"},
			expected:    true,
		},
		{
			name:        "Has admin:access",
			permissions: []string{"admin:access"},
			expected:    true,
		},
		{
			name:        "Empty list",
			permissions: []string{},
			expected:    false,
		},
		{
			name:        "Only invalid permissions",
			permissions: []string{"invalid:perm"},
			expected:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := HasDangerousPermissions(tt.permissions)
			if result != tt.expected {
				t.Errorf("HasDangerousPermissions(%v) = %v, want %v",
					tt.permissions, result, tt.expected)
			}
		})
	}
}

func TestGetHighestRiskLevel(t *testing.T) {
	tests := []struct {
		name        string
		permissions []string
		expected    PermissionRisk
	}{
		{
			name:        "All low risk",
			permissions: []string{"contacts:read", "invoices:read"},
			expected:    RiskLow,
		},
		{
			name:        "Mixed with medium",
			permissions: []string{"contacts:read", "invoices:write"},
			expected:    RiskMedium,
		},
		{
			name:        "Mixed with high",
			permissions: []string{"contacts:read", "payroll:write"},
			expected:    RiskHigh,
		},
		{
			name:        "Mixed with critical",
			permissions: []string{"contacts:read", "hooks:register"},
			expected:    RiskCritical,
		},
		{
			name:        "Empty list",
			permissions: []string{},
			expected:    RiskLow,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetHighestRiskLevel(tt.permissions)
			if result != tt.expected {
				t.Errorf("GetHighestRiskLevel(%v) = %v, want %v",
					tt.permissions, result, tt.expected)
			}
		})
	}
}

func TestSummarizePermissions(t *testing.T) {
	tests := []struct {
		name        string
		permissions []string
		wantTotal   int
		wantInvalid int
	}{
		{
			name:        "Valid permissions",
			permissions: []string{"contacts:read", "invoices:write", "hooks:register"},
			wantTotal:   3,
			wantInvalid: 0,
		},
		{
			name:        "Some invalid",
			permissions: []string{"contacts:read", "invalid:one", "invalid:two"},
			wantTotal:   3,
			wantInvalid: 2,
		},
		{
			name:        "Empty",
			permissions: []string{},
			wantTotal:   0,
			wantInvalid: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			summary := SummarizePermissions(tt.permissions)
			if summary.Total != tt.wantTotal {
				t.Errorf("SummarizePermissions() Total = %d, want %d", summary.Total, tt.wantTotal)
			}
			if summary.InvalidCount != tt.wantInvalid {
				t.Errorf("SummarizePermissions() InvalidCount = %d, want %d", summary.InvalidCount, tt.wantInvalid)
			}
		})
	}
}

func TestPermissionRiskCategories(t *testing.T) {
	// Verify specific permissions have correct risk levels
	expectations := map[string]PermissionRisk{
		// Low risk
		"contacts:read":  RiskLow,
		"contacts:write": RiskLow,
		"invoices:read":  RiskLow,
		"accounts:read":  RiskLow,
		"storage:read":   RiskLow,
		"pdf:generate":   RiskLow,

		// Medium risk
		"invoices:write": RiskMedium,
		"payments:write": RiskMedium,
		"email:send":     RiskMedium,
		"payroll:read":   RiskMedium,
		"banking:read":   RiskMedium,

		// High risk
		"payroll:write":    RiskHigh,
		"banking:write":    RiskHigh,
		"database:migrate": RiskHigh,
		"database:query":   RiskHigh,

		// Critical risk
		"hooks:register":  RiskCritical,
		"routes:register": RiskCritical,
		"admin:access":    RiskCritical,
	}

	for perm, expectedRisk := range expectations {
		t.Run(perm, func(t *testing.T) {
			p, exists := GetPermission(perm)
			if !exists {
				t.Fatalf("Permission %q not found", perm)
			}
			if p.Risk != expectedRisk {
				t.Errorf("Permission %q has risk %q, want %q", perm, p.Risk, expectedRisk)
			}
		})
	}
}
