package plugin

// PermissionCategory represents a category of permissions
type PermissionCategory string

const (
	CategoryDataAccess PermissionCategory = "data"
	CategorySystem     PermissionCategory = "system"
	CategoryDatabase   PermissionCategory = "database"
	CategoryDangerous  PermissionCategory = "dangerous"
)

// PermissionRisk represents the risk level of a permission
type PermissionRisk string

const (
	RiskLow      PermissionRisk = "low"
	RiskMedium   PermissionRisk = "medium"
	RiskHigh     PermissionRisk = "high"
	RiskCritical PermissionRisk = "critical"
)

// Permission defines a plugin permission
type Permission struct {
	Name        string             `json:"name"`
	Category    PermissionCategory `json:"category"`
	Risk        PermissionRisk     `json:"risk"`
	Description string             `json:"description"`
}

// AllPermissions is the registry of all available permissions
var AllPermissions = map[string]Permission{
	// Data Access - Low Risk
	"contacts:read": {
		Name:        "contacts:read",
		Category:    CategoryDataAccess,
		Risk:        RiskLow,
		Description: "Read contact information",
	},
	"contacts:write": {
		Name:        "contacts:write",
		Category:    CategoryDataAccess,
		Risk:        RiskLow,
		Description: "Create and modify contacts",
	},
	"invoices:read": {
		Name:        "invoices:read",
		Category:    CategoryDataAccess,
		Risk:        RiskLow,
		Description: "Read invoices",
	},
	"invoices:write": {
		Name:        "invoices:write",
		Category:    CategoryDataAccess,
		Risk:        RiskMedium,
		Description: "Create and modify invoices",
	},
	"payments:read": {
		Name:        "payments:read",
		Category:    CategoryDataAccess,
		Risk:        RiskLow,
		Description: "Read payment records",
	},
	"payments:write": {
		Name:        "payments:write",
		Category:    CategoryDataAccess,
		Risk:        RiskMedium,
		Description: "Record payments",
	},
	"accounts:read": {
		Name:        "accounts:read",
		Category:    CategoryDataAccess,
		Risk:        RiskLow,
		Description: "Read chart of accounts",
	},
	"accounts:write": {
		Name:        "accounts:write",
		Category:    CategoryDataAccess,
		Risk:        RiskMedium,
		Description: "Modify chart of accounts",
	},
	"employees:read": {
		Name:        "employees:read",
		Category:    CategoryDataAccess,
		Risk:        RiskLow,
		Description: "Read employee information",
	},
	"employees:write": {
		Name:        "employees:write",
		Category:    CategoryDataAccess,
		Risk:        RiskMedium,
		Description: "Manage employees",
	},
	"payroll:read": {
		Name:        "payroll:read",
		Category:    CategoryDataAccess,
		Risk:        RiskMedium,
		Description: "Read payroll data",
	},
	"payroll:write": {
		Name:        "payroll:write",
		Category:    CategoryDataAccess,
		Risk:        RiskHigh,
		Description: "Manage payroll",
	},
	"banking:read": {
		Name:        "banking:read",
		Category:    CategoryDataAccess,
		Risk:        RiskMedium,
		Description: "Read bank account data",
	},
	"banking:write": {
		Name:        "banking:write",
		Category:    CategoryDataAccess,
		Risk:        RiskHigh,
		Description: "Manage bank transactions",
	},

	// System - Medium Risk
	"email:send": {
		Name:        "email:send",
		Category:    CategorySystem,
		Risk:        RiskMedium,
		Description: "Send emails on behalf of tenant",
	},
	"storage:read": {
		Name:        "storage:read",
		Category:    CategorySystem,
		Risk:        RiskLow,
		Description: "Read stored files",
	},
	"storage:write": {
		Name:        "storage:write",
		Category:    CategorySystem,
		Risk:        RiskMedium,
		Description: "Upload and store files",
	},
	"pdf:generate": {
		Name:        "pdf:generate",
		Category:    CategorySystem,
		Risk:        RiskLow,
		Description: "Generate PDF documents",
	},
	"settings:read": {
		Name:        "settings:read",
		Category:    CategorySystem,
		Risk:        RiskLow,
		Description: "Read tenant settings",
	},
	"settings:write": {
		Name:        "settings:write",
		Category:    CategorySystem,
		Risk:        RiskMedium,
		Description: "Modify tenant settings",
	},

	// Database - High Risk
	"database:migrate": {
		Name:        "database:migrate",
		Category:    CategoryDatabase,
		Risk:        RiskHigh,
		Description: "Run database migrations in tenant schema",
	},
	"database:query": {
		Name:        "database:query",
		Category:    CategoryDatabase,
		Risk:        RiskHigh,
		Description: "Execute SQL queries in tenant schema",
	},

	// Dangerous - Critical Risk
	"hooks:register": {
		Name:        "hooks:register",
		Category:    CategoryDangerous,
		Risk:        RiskCritical,
		Description: "Listen to system events",
	},
	"routes:register": {
		Name:        "routes:register",
		Category:    CategoryDangerous,
		Risk:        RiskCritical,
		Description: "Add custom API endpoints",
	},
	"admin:access": {
		Name:        "admin:access",
		Category:    CategoryDangerous,
		Risk:        RiskCritical,
		Description: "Access admin functions",
	},
}

// ValidatePermission checks if a permission name is valid
func ValidatePermission(name string) bool {
	_, exists := AllPermissions[name]
	return exists
}

// ValidatePermissions checks if all permission names are valid
func ValidatePermissions(names []string) (invalid []string) {
	for _, name := range names {
		if !ValidatePermission(name) {
			invalid = append(invalid, name)
		}
	}
	return invalid
}

// GetPermission returns a permission by name
func GetPermission(name string) (Permission, bool) {
	p, exists := AllPermissions[name]
	return p, exists
}

// GetPermissionsByCategory returns all permissions in a category
func GetPermissionsByCategory(category PermissionCategory) []Permission {
	var result []Permission
	for _, p := range AllPermissions {
		if p.Category == category {
			result = append(result, p)
		}
	}
	return result
}

// GetPermissionsByRisk returns all permissions at or above a risk level
func GetPermissionsByRisk(minRisk PermissionRisk) []Permission {
	riskOrder := map[PermissionRisk]int{
		RiskLow:      1,
		RiskMedium:   2,
		RiskHigh:     3,
		RiskCritical: 4,
	}

	minRiskLevel := riskOrder[minRisk]
	var result []Permission
	for _, p := range AllPermissions {
		if riskOrder[p.Risk] >= minRiskLevel {
			result = append(result, p)
		}
	}
	return result
}

// HasDangerousPermissions checks if any permissions are in the dangerous category
func HasDangerousPermissions(permissions []string) bool {
	for _, name := range permissions {
		if p, exists := AllPermissions[name]; exists {
			if p.Category == CategoryDangerous {
				return true
			}
		}
	}
	return false
}

// GetHighestRiskLevel returns the highest risk level among the given permissions
func GetHighestRiskLevel(permissions []string) PermissionRisk {
	riskOrder := map[PermissionRisk]int{
		RiskLow:      1,
		RiskMedium:   2,
		RiskHigh:     3,
		RiskCritical: 4,
	}

	highest := RiskLow
	for _, name := range permissions {
		if p, exists := AllPermissions[name]; exists {
			if riskOrder[p.Risk] > riskOrder[highest] {
				highest = p.Risk
			}
		}
	}
	return highest
}

// PermissionSummary provides a summary of requested permissions
type PermissionSummary struct {
	Total        int            `json:"total"`
	ByCategory   map[string]int `json:"by_category"`
	ByRisk       map[string]int `json:"by_risk"`
	HighestRisk  PermissionRisk `json:"highest_risk"`
	HasDangerous bool           `json:"has_dangerous"`
	InvalidCount int            `json:"invalid_count"`
	InvalidNames []string       `json:"invalid_names,omitempty"`
}

// SummarizePermissions provides a summary of the given permissions
func SummarizePermissions(permissions []string) PermissionSummary {
	summary := PermissionSummary{
		Total:      len(permissions),
		ByCategory: make(map[string]int),
		ByRisk:     make(map[string]int),
	}

	for _, name := range permissions {
		if p, exists := AllPermissions[name]; exists {
			summary.ByCategory[string(p.Category)]++
			summary.ByRisk[string(p.Risk)]++
		} else {
			summary.InvalidCount++
			summary.InvalidNames = append(summary.InvalidNames, name)
		}
	}

	summary.HighestRisk = GetHighestRiskLevel(permissions)
	summary.HasDangerous = HasDangerousPermissions(permissions)

	return summary
}
