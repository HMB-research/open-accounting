package scheduler

import "context"

// TenantInfo contains minimal tenant information for scheduling
type TenantInfo struct {
	ID             string
	SchemaName     string
	CompanyName    string // Needed for email templates
	PeriodLockDate string
}

// Repository defines the interface for scheduler data access
type Repository interface {
	ListActiveTenants(ctx context.Context) ([]TenantInfo, error)
}
