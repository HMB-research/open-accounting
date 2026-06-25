package tenant

import (
	"context"
	"fmt"
	"time"

	"github.com/HMB-research/open-accounting/internal/database"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository defines the contract for tenant data access
type Repository interface {
	// Tenant operations
	CreateTenant(ctx context.Context, tenant *Tenant, settingsJSON []byte, ownerID string) error
	GetTenant(ctx context.Context, tenantID string) (*Tenant, error)
	GetTenantBySlug(ctx context.Context, slug string) (*Tenant, error)
	UpdateTenant(ctx context.Context, tenantID, name string, settingsJSON []byte, updatedAt time.Time) error
	UpdateTenantWithPeriodCloseEvent(ctx context.Context, tenantID, name string, settingsJSON []byte, updatedAt time.Time, event *PeriodCloseEvent) error
	DeleteTenant(ctx context.Context, tenantID, schemaName string) error
	CompleteOnboarding(ctx context.Context, tenantID string) error
	ListPeriodCloseEvents(ctx context.Context, tenantID string, limit int) ([]PeriodCloseEvent, error)
	GetLatestCloseEventForPeriod(ctx context.Context, tenantID, periodEndDate string) (*PeriodCloseEvent, error)
	CreateTenantAuditEvent(ctx context.Context, event *TenantAuditEvent) error
	ListTenantAuditEvents(ctx context.Context, tenantID string, limit int) ([]TenantAuditEvent, error)

	// Tenant User operations
	AddUserToTenant(ctx context.Context, tenantID, userID, role string) error
	RemoveUserFromTenant(ctx context.Context, tenantID, userID string) error
	GetUserRole(ctx context.Context, tenantID, userID string) (string, error)
	GetTenantUser(ctx context.Context, tenantID, userID string) (*TenantUser, error)
	ListUserTenants(ctx context.Context, userID string) ([]TenantMembership, error)
	ListTenantUsers(ctx context.Context, tenantID string) ([]TenantUser, error)
	UpdateTenantUserRole(ctx context.Context, tenantID, userID, newRole string) error
	SetTenantUserActive(ctx context.Context, tenantID, userID string, active bool) error
	RemoveTenantUser(ctx context.Context, tenantID, userID string) error

	// User operations
	CreateUser(ctx context.Context, user *User) error
	GetUserByEmail(ctx context.Context, email string) (*User, error)
	GetUserByID(ctx context.Context, userID string) (*User, error)
	UpdateUserPassword(ctx context.Context, userID, passwordHash string, updatedAt time.Time) error

	// Invitation operations
	CreateInvitation(ctx context.Context, inv *UserInvitation) error
	GetInvitationByToken(ctx context.Context, token string) (*UserInvitation, error)
	AcceptInvitation(ctx context.Context, inv *UserInvitation, userID string, password string, name string, createUser bool) error
	ListInvitations(ctx context.Context, tenantID string) ([]UserInvitation, error)
	RevokeInvitation(ctx context.Context, tenantID, invitationID string) error
	CheckUserIsMember(ctx context.Context, tenantID, email string) (bool, error)
}

// Common errors
var (
	ErrTenantNotFound     = fmt.Errorf("tenant not found")
	ErrUserNotFound       = fmt.Errorf("user not found")
	ErrUserNotInTenant    = fmt.Errorf("user not member of tenant")
	ErrInvitationNotFound = fmt.Errorf("invitation not found")
	ErrEmailExists        = fmt.Errorf("email already exists")
)

var newGormDBFromPool = database.NewGormDBFromPool

func NewRepository(db *pgxpool.Pool) *GORMRepository {
	if db == nil {
		return &GORMRepository{}
	}
	gormDB, err := newGormDBFromPool(context.Background(), db)
	if err != nil {
		panic(fmt.Errorf("create tenant GORM repository: %w", err))
	}
	return NewGORMRepository(gormDB)
}
