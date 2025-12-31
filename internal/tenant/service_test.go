package tenant

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Mock Repository
// =============================================================================

type MockRepository struct {
	tenants     map[string]*Tenant
	users       map[string]*User
	tenantUsers map[string][]TenantUser
	invitations map[string]*UserInvitation

	// Error injection
	createTenantErr         error
	getTenantErr            error
	getTenantBySlugErr      error
	updateTenantErr         error
	deleteTenantErr         error
	completeOnboardingErr   error
	addUserToTenantErr      error
	removeUserFromTenantErr error
	getUserRoleErr          error
	listUserTenantsErr      error
	listTenantUsersErr      error
	updateTenantUserRoleErr error
	removeTenantUserErr     error
	createUserErr           error
	getUserByEmailErr       error
	getUserByIDErr          error
	createInvitationErr     error
	getInvitationByTokenErr error
	acceptInvitationErr     error
	listInvitationsErr      error
	revokeInvitationErr     error
	checkUserIsMemberErr    error
	userIsMember            bool
}

func NewMockRepository() *MockRepository {
	return &MockRepository{
		tenants:     make(map[string]*Tenant),
		users:       make(map[string]*User),
		tenantUsers: make(map[string][]TenantUser),
		invitations: make(map[string]*UserInvitation),
	}
}

func (m *MockRepository) CreateTenant(ctx context.Context, tenant *Tenant, settingsJSON []byte, ownerID string) error {
	if m.createTenantErr != nil {
		return m.createTenantErr
	}
	m.tenants[tenant.ID] = tenant
	if ownerID != "" {
		m.tenantUsers[tenant.ID] = append(m.tenantUsers[tenant.ID], TenantUser{
			TenantID:  tenant.ID,
			UserID:    ownerID,
			Role:      RoleOwner,
			IsDefault: true,
			CreatedAt: time.Now(),
		})
	}
	return nil
}

func (m *MockRepository) GetTenant(ctx context.Context, tenantID string) (*Tenant, error) {
	if m.getTenantErr != nil {
		return nil, m.getTenantErr
	}
	t, ok := m.tenants[tenantID]
	if !ok {
		return nil, ErrTenantNotFound
	}
	return t, nil
}

func (m *MockRepository) GetTenantBySlug(ctx context.Context, slug string) (*Tenant, error) {
	if m.getTenantBySlugErr != nil {
		return nil, m.getTenantBySlugErr
	}
	for _, t := range m.tenants {
		if t.Slug == slug {
			return t, nil
		}
	}
	return nil, ErrTenantNotFound
}

func (m *MockRepository) UpdateTenant(ctx context.Context, tenantID, name string, settingsJSON []byte, updatedAt time.Time) error {
	if m.updateTenantErr != nil {
		return m.updateTenantErr
	}
	t, ok := m.tenants[tenantID]
	if ok {
		t.Name = name
		t.UpdatedAt = updatedAt
	}
	return nil
}

func (m *MockRepository) DeleteTenant(ctx context.Context, tenantID, schemaName string) error {
	if m.deleteTenantErr != nil {
		return m.deleteTenantErr
	}
	delete(m.tenants, tenantID)
	delete(m.tenantUsers, tenantID)
	return nil
}

func (m *MockRepository) CompleteOnboarding(ctx context.Context, tenantID string) error {
	if m.completeOnboardingErr != nil {
		return m.completeOnboardingErr
	}
	if t, ok := m.tenants[tenantID]; ok {
		t.OnboardingCompleted = true
	}
	return nil
}

func (m *MockRepository) AddUserToTenant(ctx context.Context, tenantID, userID, role string) error {
	if m.addUserToTenantErr != nil {
		return m.addUserToTenantErr
	}
	m.tenantUsers[tenantID] = append(m.tenantUsers[tenantID], TenantUser{
		TenantID:  tenantID,
		UserID:    userID,
		Role:      role,
		IsDefault: false,
		CreatedAt: time.Now(),
	})
	return nil
}

func (m *MockRepository) RemoveUserFromTenant(ctx context.Context, tenantID, userID string) error {
	if m.removeUserFromTenantErr != nil {
		return m.removeUserFromTenantErr
	}
	users := m.tenantUsers[tenantID]
	for i, u := range users {
		if u.UserID == userID {
			m.tenantUsers[tenantID] = append(users[:i], users[i+1:]...)
			return nil
		}
	}
	return ErrUserNotInTenant
}

func (m *MockRepository) GetUserRole(ctx context.Context, tenantID, userID string) (string, error) {
	if m.getUserRoleErr != nil {
		return "", m.getUserRoleErr
	}
	for _, u := range m.tenantUsers[tenantID] {
		if u.UserID == userID {
			return u.Role, nil
		}
	}
	return "", ErrUserNotInTenant
}

func (m *MockRepository) ListUserTenants(ctx context.Context, userID string) ([]TenantMembership, error) {
	if m.listUserTenantsErr != nil {
		return nil, m.listUserTenantsErr
	}
	var memberships []TenantMembership
	for tenantID, users := range m.tenantUsers {
		for _, u := range users {
			if u.UserID == userID {
				if t, ok := m.tenants[tenantID]; ok {
					memberships = append(memberships, TenantMembership{
						Tenant:    *t,
						Role:      u.Role,
						IsDefault: u.IsDefault,
					})
				}
			}
		}
	}
	return memberships, nil
}

func (m *MockRepository) ListTenantUsers(ctx context.Context, tenantID string) ([]TenantUser, error) {
	if m.listTenantUsersErr != nil {
		return nil, m.listTenantUsersErr
	}
	return m.tenantUsers[tenantID], nil
}

func (m *MockRepository) UpdateTenantUserRole(ctx context.Context, tenantID, userID, newRole string) error {
	if m.updateTenantUserRoleErr != nil {
		return m.updateTenantUserRoleErr
	}
	for i := range m.tenantUsers[tenantID] {
		if m.tenantUsers[tenantID][i].UserID == userID {
			m.tenantUsers[tenantID][i].Role = newRole
			return nil
		}
	}
	return nil
}

func (m *MockRepository) RemoveTenantUser(ctx context.Context, tenantID, userID string) error {
	if m.removeTenantUserErr != nil {
		return m.removeTenantUserErr
	}
	users := m.tenantUsers[tenantID]
	for i, u := range users {
		if u.UserID == userID {
			m.tenantUsers[tenantID] = append(users[:i], users[i+1:]...)
			return nil
		}
	}
	return nil
}

func (m *MockRepository) CreateUser(ctx context.Context, user *User) error {
	if m.createUserErr != nil {
		return m.createUserErr
	}
	m.users[user.ID] = user
	return nil
}

func (m *MockRepository) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	if m.getUserByEmailErr != nil {
		return nil, m.getUserByEmailErr
	}
	for _, u := range m.users {
		if u.Email == email {
			return u, nil
		}
	}
	return nil, ErrUserNotFound
}

func (m *MockRepository) GetUserByID(ctx context.Context, userID string) (*User, error) {
	if m.getUserByIDErr != nil {
		return nil, m.getUserByIDErr
	}
	u, ok := m.users[userID]
	if !ok {
		return nil, ErrUserNotFound
	}
	return u, nil
}

func (m *MockRepository) CreateInvitation(ctx context.Context, inv *UserInvitation) error {
	if m.createInvitationErr != nil {
		return m.createInvitationErr
	}
	m.invitations[inv.Token] = inv
	return nil
}

func (m *MockRepository) GetInvitationByToken(ctx context.Context, token string) (*UserInvitation, error) {
	if m.getInvitationByTokenErr != nil {
		return nil, m.getInvitationByTokenErr
	}
	inv, ok := m.invitations[token]
	if !ok {
		return nil, ErrInvitationNotFound
	}
	return inv, nil
}

func (m *MockRepository) AcceptInvitation(ctx context.Context, inv *UserInvitation, userID string, password string, name string, createUser bool) error {
	if m.acceptInvitationErr != nil {
		return m.acceptInvitationErr
	}
	if createUser {
		m.users[userID] = &User{
			ID:           userID,
			Email:        inv.Email,
			PasswordHash: password,
			Name:         name,
			IsActive:     true,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		}
	}
	m.tenantUsers[inv.TenantID] = append(m.tenantUsers[inv.TenantID], TenantUser{
		TenantID:  inv.TenantID,
		UserID:    userID,
		Role:      inv.Role,
		IsDefault: false,
		CreatedAt: time.Now(),
	})
	now := time.Now()
	inv.AcceptedAt = &now
	return nil
}

func (m *MockRepository) ListInvitations(ctx context.Context, tenantID string) ([]UserInvitation, error) {
	if m.listInvitationsErr != nil {
		return nil, m.listInvitationsErr
	}
	var result []UserInvitation
	for _, inv := range m.invitations {
		if inv.TenantID == tenantID && inv.AcceptedAt == nil && time.Now().Before(inv.ExpiresAt) {
			result = append(result, *inv)
		}
	}
	return result, nil
}

func (m *MockRepository) RevokeInvitation(ctx context.Context, tenantID, invitationID string) error {
	if m.revokeInvitationErr != nil {
		return m.revokeInvitationErr
	}
	for token, inv := range m.invitations {
		if inv.ID == invitationID && inv.TenantID == tenantID && inv.AcceptedAt == nil {
			delete(m.invitations, token)
			return nil
		}
	}
	return errors.New("invitation not found or already accepted")
}

func (m *MockRepository) CheckUserIsMember(ctx context.Context, tenantID, email string) (bool, error) {
	if m.checkUserIsMemberErr != nil {
		return false, m.checkUserIsMemberErr
	}
	return m.userIsMember, nil
}

// Helper to add test data
func (m *MockRepository) AddTestTenant(t *Tenant) {
	m.tenants[t.ID] = t
}

func (m *MockRepository) AddTestUser(u *User) {
	m.users[u.ID] = u
}

func (m *MockRepository) AddTestTenantUser(tu TenantUser) {
	m.tenantUsers[tu.TenantID] = append(m.tenantUsers[tu.TenantID], tu)
}

// =============================================================================
// Service Tests
// =============================================================================

func TestNewServiceWithRepository(t *testing.T) {
	repo := NewMockRepository()
	svc := NewServiceWithRepository(repo)

	assert.NotNil(t, svc)
	assert.NotNil(t, svc.repo)
}

func TestService_CreateTenant(t *testing.T) {
	tests := []struct {
		name       string
		req        *CreateTenantRequest
		setupRepo  func(*MockRepository)
		wantErr    bool
		errContain string
	}{
		{
			name: "valid tenant creation",
			req: &CreateTenantRequest{
				Name: "Test Company",
				Slug: "test-company",
			},
			wantErr: false,
		},
		{
			name: "with custom settings",
			req: &CreateTenantRequest{
				Name: "US Company",
				Slug: "us-company",
				Settings: &TenantSettings{
					DefaultCurrency: "USD",
					CountryCode:     "US",
				},
			},
			wantErr: false,
		},
		{
			name: "with owner ID",
			req: &CreateTenantRequest{
				Name:    "My Company",
				Slug:    "my-company",
				OwnerID: "user-123",
			},
			wantErr: false,
		},
		{
			name: "slug too short",
			req: &CreateTenantRequest{
				Name: "Test",
				Slug: "ab",
			},
			wantErr:    true,
			errContain: "slug must be between 3 and 50 characters",
		},
		{
			name: "slug with invalid characters",
			req: &CreateTenantRequest{
				Name: "Test",
				Slug: "Test_Company",
			},
			wantErr:    true,
			errContain: "slug must contain only lowercase letters",
		},
		{
			name: "repository error",
			req: &CreateTenantRequest{
				Name: "Test",
				Slug: "test-company",
			},
			setupRepo: func(m *MockRepository) {
				m.createTenantErr = errors.New("db error")
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := NewMockRepository()
			if tt.setupRepo != nil {
				tt.setupRepo(repo)
			}
			svc := NewServiceWithRepository(repo)

			tenant, err := svc.CreateTenant(context.Background(), tt.req)

			if tt.wantErr {
				require.Error(t, err)
				if tt.errContain != "" {
					assert.Contains(t, err.Error(), tt.errContain)
				}
				return
			}

			require.NoError(t, err)
			require.NotNil(t, tenant)
			assert.Equal(t, tt.req.Name, tenant.Name)
			assert.Equal(t, tt.req.Slug, tenant.Slug)
			assert.True(t, tenant.IsActive)
			assert.NotEmpty(t, tenant.ID)
		})
	}
}

func TestService_GetTenant(t *testing.T) {
	tests := []struct {
		name      string
		tenantID  string
		setupRepo func(*MockRepository)
		wantErr   bool
	}{
		{
			name:     "existing tenant",
			tenantID: "tenant-123",
			setupRepo: func(m *MockRepository) {
				m.AddTestTenant(&Tenant{
					ID:       "tenant-123",
					Name:     "Test Company",
					Slug:     "test-company",
					IsActive: true,
				})
			},
			wantErr: false,
		},
		{
			name:     "not found",
			tenantID: "nonexistent",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := NewMockRepository()
			if tt.setupRepo != nil {
				tt.setupRepo(repo)
			}
			svc := NewServiceWithRepository(repo)

			tenant, err := svc.GetTenant(context.Background(), tt.tenantID)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, tenant)
			assert.Equal(t, tt.tenantID, tenant.ID)
		})
	}
}

func TestService_GetTenantBySlug(t *testing.T) {
	repo := NewMockRepository()
	repo.AddTestTenant(&Tenant{
		ID:       "tenant-123",
		Name:     "Test Company",
		Slug:     "test-company",
		IsActive: true,
	})
	svc := NewServiceWithRepository(repo)

	t.Run("existing slug", func(t *testing.T) {
		tenant, err := svc.GetTenantBySlug(context.Background(), "test-company")
		require.NoError(t, err)
		assert.Equal(t, "tenant-123", tenant.ID)
	})

	t.Run("not found", func(t *testing.T) {
		_, err := svc.GetTenantBySlug(context.Background(), "nonexistent")
		require.Error(t, err)
	})
}

func TestService_UpdateTenant(t *testing.T) {
	tests := []struct {
		name      string
		tenantID  string
		req       *UpdateTenantRequest
		setupRepo func(*MockRepository)
		wantErr   bool
	}{
		{
			name:     "update name",
			tenantID: "tenant-123",
			req: &UpdateTenantRequest{
				Name: strPtr("Updated Name"),
			},
			setupRepo: func(m *MockRepository) {
				m.AddTestTenant(&Tenant{
					ID:       "tenant-123",
					Name:     "Original Name",
					Slug:     "test",
					Settings: DefaultSettings(),
				})
			},
			wantErr: false,
		},
		{
			name:     "update settings",
			tenantID: "tenant-123",
			req: &UpdateTenantRequest{
				Settings: &TenantSettings{
					VATNumber: "VAT123",
					Email:     "new@example.com",
				},
			},
			setupRepo: func(m *MockRepository) {
				m.AddTestTenant(&Tenant{
					ID:       "tenant-123",
					Name:     "Test",
					Slug:     "test",
					Settings: DefaultSettings(),
				})
			},
			wantErr: false,
		},
		{
			name:     "update all settings fields",
			tenantID: "tenant-123",
			req: &UpdateTenantRequest{
				Name: strPtr("Updated Company"),
				Settings: &TenantSettings{
					VATNumber:       "EE123456789",
					RegCode:         "12345678",
					Address:         "123 Main St",
					Email:           "company@example.com",
					Phone:           "+372 555 1234",
					Logo:            "logo.png",
					PDFPrimaryColor: "#FF0000",
					PDFFooterText:   "Thank you for your business",
					BankDetails:     "EE123456789012345678",
					InvoiceTerms:    "Net 30",
					Timezone:        "Europe/Tallinn",
					DateFormat:      "DD.MM.YYYY",
					DecimalSep:      ",",
					ThousandsSep:    " ",
					FiscalYearStart: 7,
				},
			},
			setupRepo: func(m *MockRepository) {
				m.AddTestTenant(&Tenant{
					ID:       "tenant-123",
					Name:     "Test",
					Slug:     "test",
					Settings: DefaultSettings(),
				})
			},
			wantErr: false,
		},
		{
			name:     "update with empty name is ignored",
			tenantID: "tenant-123",
			req: &UpdateTenantRequest{
				Name: strPtr(""),
			},
			setupRepo: func(m *MockRepository) {
				m.AddTestTenant(&Tenant{
					ID:       "tenant-123",
					Name:     "Original Name",
					Slug:     "test",
					Settings: DefaultSettings(),
				})
			},
			wantErr: false,
		},
		{
			name:     "update with nil settings",
			tenantID: "tenant-123",
			req: &UpdateTenantRequest{
				Name:     strPtr("Updated"),
				Settings: nil,
			},
			setupRepo: func(m *MockRepository) {
				m.AddTestTenant(&Tenant{
					ID:       "tenant-123",
					Name:     "Original",
					Slug:     "test",
					Settings: DefaultSettings(),
				})
			},
			wantErr: false,
		},
		{
			name:     "repository update error",
			tenantID: "tenant-123",
			req: &UpdateTenantRequest{
				Name: strPtr("Updated"),
			},
			setupRepo: func(m *MockRepository) {
				m.AddTestTenant(&Tenant{
					ID:       "tenant-123",
					Name:     "Test",
					Slug:     "test",
					Settings: DefaultSettings(),
				})
				m.updateTenantErr = errors.New("db error")
			},
			wantErr: true,
		},
		{
			name:     "tenant not found",
			tenantID: "nonexistent",
			req: &UpdateTenantRequest{
				Name: strPtr("Updated"),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := NewMockRepository()
			if tt.setupRepo != nil {
				tt.setupRepo(repo)
			}
			svc := NewServiceWithRepository(repo)

			tenant, err := svc.UpdateTenant(context.Background(), tt.tenantID, tt.req)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, tenant)
		})
	}
}

func TestService_DeleteTenant(t *testing.T) {
	repo := NewMockRepository()
	repo.AddTestTenant(&Tenant{
		ID:         "tenant-123",
		Name:       "Test",
		Slug:       "test",
		SchemaName: "tenant_test",
	})
	svc := NewServiceWithRepository(repo)

	t.Run("delete existing", func(t *testing.T) {
		err := svc.DeleteTenant(context.Background(), "tenant-123")
		require.NoError(t, err)

		// Verify deleted
		_, err = svc.GetTenant(context.Background(), "tenant-123")
		require.Error(t, err)
	})

	t.Run("delete nonexistent", func(t *testing.T) {
		err := svc.DeleteTenant(context.Background(), "nonexistent")
		require.Error(t, err)
	})
}

func TestService_CompleteOnboarding(t *testing.T) {
	repo := NewMockRepository()
	repo.AddTestTenant(&Tenant{
		ID:                  "tenant-123",
		OnboardingCompleted: false,
	})
	svc := NewServiceWithRepository(repo)

	err := svc.CompleteOnboarding(context.Background(), "tenant-123")
	require.NoError(t, err)

	tenant, _ := repo.GetTenant(context.Background(), "tenant-123")
	assert.True(t, tenant.OnboardingCompleted)
}

func TestService_ListUserTenants(t *testing.T) {
	repo := NewMockRepository()
	repo.AddTestTenant(&Tenant{ID: "tenant-1", Name: "Tenant 1", Slug: "t1", IsActive: true})
	repo.AddTestTenant(&Tenant{ID: "tenant-2", Name: "Tenant 2", Slug: "t2", IsActive: true})
	repo.AddTestTenantUser(TenantUser{TenantID: "tenant-1", UserID: "user-1", Role: RoleOwner, IsDefault: true})
	repo.AddTestTenantUser(TenantUser{TenantID: "tenant-2", UserID: "user-1", Role: RoleAdmin})

	svc := NewServiceWithRepository(repo)

	memberships, err := svc.ListUserTenants(context.Background(), "user-1")
	require.NoError(t, err)
	assert.Len(t, memberships, 2)
}

func TestService_AddUserToTenant(t *testing.T) {
	repo := NewMockRepository()
	svc := NewServiceWithRepository(repo)

	err := svc.AddUserToTenant(context.Background(), "tenant-1", "user-1", RoleAdmin)
	require.NoError(t, err)

	users, _ := repo.ListTenantUsers(context.Background(), "tenant-1")
	assert.Len(t, users, 1)
	assert.Equal(t, RoleAdmin, users[0].Role)
}

func TestService_RemoveUserFromTenant(t *testing.T) {
	repo := NewMockRepository()
	repo.AddTestTenantUser(TenantUser{TenantID: "tenant-1", UserID: "user-1", Role: RoleAdmin})
	svc := NewServiceWithRepository(repo)

	t.Run("remove existing user", func(t *testing.T) {
		err := svc.RemoveUserFromTenant(context.Background(), "tenant-1", "user-1")
		require.NoError(t, err)
	})

	t.Run("user not in tenant", func(t *testing.T) {
		err := svc.RemoveUserFromTenant(context.Background(), "tenant-1", "user-999")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "user not found in tenant")
	})
}

func TestService_GetUserRole(t *testing.T) {
	repo := NewMockRepository()
	repo.AddTestTenantUser(TenantUser{TenantID: "tenant-1", UserID: "user-1", Role: RoleAccountant})
	svc := NewServiceWithRepository(repo)

	t.Run("existing user", func(t *testing.T) {
		role, err := svc.GetUserRole(context.Background(), "tenant-1", "user-1")
		require.NoError(t, err)
		assert.Equal(t, RoleAccountant, role)
	})

	t.Run("user not in tenant", func(t *testing.T) {
		_, err := svc.GetUserRole(context.Background(), "tenant-1", "user-999")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "user not member of tenant")
	})
}

func TestService_CreateUser(t *testing.T) {
	t.Run("valid user", func(t *testing.T) {
		repo := NewMockRepository()
		svc := NewServiceWithRepository(repo)
		user, err := svc.CreateUser(context.Background(), &CreateUserRequest{
			Email:    "  Test@Example.COM  ",
			Password: "password123",
			Name:     "Test User",
		})
		require.NoError(t, err)
		assert.Equal(t, "test@example.com", user.Email) // normalized
		assert.NotEmpty(t, user.PasswordHash)
	})

	t.Run("duplicate email", func(t *testing.T) {
		repo := NewMockRepository()
		repo.createUserErr = ErrEmailExists
		svc := NewServiceWithRepository(repo)
		_, err := svc.CreateUser(context.Background(), &CreateUserRequest{
			Email:    "test@example.com",
			Password: "password",
			Name:     "Test",
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "email already exists")
	})

	t.Run("repository error", func(t *testing.T) {
		repo := NewMockRepository()
		repo.createUserErr = errors.New("database connection failed")
		svc := NewServiceWithRepository(repo)
		_, err := svc.CreateUser(context.Background(), &CreateUserRequest{
			Email:    "test@example.com",
			Password: "password",
			Name:     "Test",
		})
		require.Error(t, err)
	})
}

func TestService_GetUserByEmail(t *testing.T) {
	repo := NewMockRepository()
	repo.AddTestUser(&User{
		ID:    "user-123",
		Email: "test@example.com",
		Name:  "Test User",
	})
	svc := NewServiceWithRepository(repo)

	t.Run("existing user", func(t *testing.T) {
		user, err := svc.GetUserByEmail(context.Background(), "test@example.com")
		require.NoError(t, err)
		assert.Equal(t, "user-123", user.ID)
	})

	t.Run("not found", func(t *testing.T) {
		_, err := svc.GetUserByEmail(context.Background(), "nonexistent@example.com")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "user not found")
	})
}

func TestService_GetUserByID(t *testing.T) {
	repo := NewMockRepository()
	repo.AddTestUser(&User{ID: "user-123", Email: "test@example.com"})
	svc := NewServiceWithRepository(repo)

	t.Run("existing user", func(t *testing.T) {
		user, err := svc.GetUserByID(context.Background(), "user-123")
		require.NoError(t, err)
		assert.Equal(t, "test@example.com", user.Email)
	})

	t.Run("not found", func(t *testing.T) {
		_, err := svc.GetUserByID(context.Background(), "nonexistent")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "user not found")
	})
}

func TestService_CreateInvitation(t *testing.T) {
	tests := []struct {
		name       string
		tenantID   string
		invitedBy  string
		req        *CreateInvitationRequest
		setupRepo  func(*MockRepository)
		wantErr    bool
		errContain string
	}{
		{
			name:      "valid invitation",
			tenantID:  "tenant-1",
			invitedBy: "user-1",
			req: &CreateInvitationRequest{
				Email: "newuser@example.com",
				Role:  RoleAdmin,
			},
			wantErr: false,
		},
		{
			name:      "invalid role",
			tenantID:  "tenant-1",
			invitedBy: "user-1",
			req: &CreateInvitationRequest{
				Email: "test@example.com",
				Role:  "invalid",
			},
			wantErr:    true,
			errContain: "invalid role",
		},
		{
			name:      "empty email",
			tenantID:  "tenant-1",
			invitedBy: "user-1",
			req: &CreateInvitationRequest{
				Email: "   ",
				Role:  RoleAdmin,
			},
			wantErr:    true,
			errContain: "email is required",
		},
		{
			name:      "user already member",
			tenantID:  "tenant-1",
			invitedBy: "user-1",
			req: &CreateInvitationRequest{
				Email: "existing@example.com",
				Role:  RoleAdmin,
			},
			setupRepo: func(m *MockRepository) {
				m.userIsMember = true
			},
			wantErr:    true,
			errContain: "already a member",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := NewMockRepository()
			if tt.setupRepo != nil {
				tt.setupRepo(repo)
			}
			svc := NewServiceWithRepository(repo)

			inv, err := svc.CreateInvitation(context.Background(), tt.tenantID, tt.invitedBy, tt.req)

			if tt.wantErr {
				require.Error(t, err)
				if tt.errContain != "" {
					assert.Contains(t, err.Error(), tt.errContain)
				}
				return
			}

			require.NoError(t, err)
			require.NotNil(t, inv)
			assert.NotEmpty(t, inv.Token)
			assert.Equal(t, tt.tenantID, inv.TenantID)
		})
	}
}

func TestService_GetInvitationByToken(t *testing.T) {
	repo := NewMockRepository()
	validInv := &UserInvitation{
		ID:        "inv-1",
		TenantID:  "tenant-1",
		Email:     "test@example.com",
		Role:      RoleAdmin,
		Token:     "valid-token",
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}
	repo.invitations["valid-token"] = validInv
	svc := NewServiceWithRepository(repo)

	t.Run("valid token", func(t *testing.T) {
		inv, err := svc.GetInvitationByToken(context.Background(), "valid-token")
		require.NoError(t, err)
		assert.Equal(t, "inv-1", inv.ID)
	})

	t.Run("not found", func(t *testing.T) {
		_, err := svc.GetInvitationByToken(context.Background(), "invalid")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invitation not found")
	})

	t.Run("expired", func(t *testing.T) {
		expiredInv := &UserInvitation{
			ID:        "inv-2",
			Token:     "expired-token",
			ExpiresAt: time.Now().Add(-24 * time.Hour),
		}
		repo.invitations["expired-token"] = expiredInv

		_, err := svc.GetInvitationByToken(context.Background(), "expired-token")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "expired")
	})

	t.Run("already accepted", func(t *testing.T) {
		now := time.Now()
		acceptedInv := &UserInvitation{
			ID:         "inv-3",
			Token:      "accepted-token",
			ExpiresAt:  time.Now().Add(24 * time.Hour),
			AcceptedAt: &now,
		}
		repo.invitations["accepted-token"] = acceptedInv

		_, err := svc.GetInvitationByToken(context.Background(), "accepted-token")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "already accepted")
	})
}

func TestService_AcceptInvitation(t *testing.T) {
	tests := []struct {
		name       string
		req        *AcceptInvitationRequest
		setupRepo  func(*MockRepository)
		wantErr    bool
		errContain string
	}{
		{
			name: "new user accepts",
			req: &AcceptInvitationRequest{
				Token:    "valid-token",
				Password: "password123",
				Name:     "New User",
			},
			setupRepo: func(m *MockRepository) {
				m.invitations["valid-token"] = &UserInvitation{
					ID:        "inv-1",
					TenantID:  "tenant-1",
					Email:     "new@example.com",
					Role:      RoleAdmin,
					Token:     "valid-token",
					ExpiresAt: time.Now().Add(24 * time.Hour),
				}
				m.AddTestTenant(&Tenant{ID: "tenant-1", Name: "Test", Slug: "test"})
			},
			wantErr: false,
		},
		{
			name: "existing user accepts",
			req: &AcceptInvitationRequest{
				Token: "valid-token",
			},
			setupRepo: func(m *MockRepository) {
				m.invitations["valid-token"] = &UserInvitation{
					ID:        "inv-1",
					TenantID:  "tenant-1",
					Email:     "existing@example.com",
					Role:      RoleAdmin,
					Token:     "valid-token",
					ExpiresAt: time.Now().Add(24 * time.Hour),
				}
				m.AddTestTenant(&Tenant{ID: "tenant-1", Name: "Test", Slug: "test"})
				m.AddTestUser(&User{ID: "user-1", Email: "existing@example.com"})
			},
			wantErr: false,
		},
		{
			name: "new user without password",
			req: &AcceptInvitationRequest{
				Token: "valid-token",
				Name:  "New User",
			},
			setupRepo: func(m *MockRepository) {
				m.invitations["valid-token"] = &UserInvitation{
					ID:        "inv-1",
					TenantID:  "tenant-1",
					Email:     "new@example.com",
					Role:      RoleAdmin,
					Token:     "valid-token",
					ExpiresAt: time.Now().Add(24 * time.Hour),
				}
			},
			wantErr:    true,
			errContain: "password and name are required",
		},
		{
			name: "new user without name",
			req: &AcceptInvitationRequest{
				Token:    "valid-token",
				Password: "password123",
			},
			setupRepo: func(m *MockRepository) {
				m.invitations["valid-token"] = &UserInvitation{
					ID:        "inv-1",
					TenantID:  "tenant-1",
					Email:     "new@example.com",
					Role:      RoleAdmin,
					Token:     "valid-token",
					ExpiresAt: time.Now().Add(24 * time.Hour),
				}
			},
			wantErr:    true,
			errContain: "password and name are required",
		},
		{
			name: "invalid token",
			req: &AcceptInvitationRequest{
				Token:    "invalid-token",
				Password: "password123",
				Name:     "New User",
			},
			wantErr:    true,
			errContain: "invitation not found",
		},
		{
			name: "repository accept error",
			req: &AcceptInvitationRequest{
				Token:    "valid-token",
				Password: "password123",
				Name:     "New User",
			},
			setupRepo: func(m *MockRepository) {
				m.invitations["valid-token"] = &UserInvitation{
					ID:        "inv-1",
					TenantID:  "tenant-1",
					Email:     "new@example.com",
					Role:      RoleAdmin,
					Token:     "valid-token",
					ExpiresAt: time.Now().Add(24 * time.Hour),
				}
				m.acceptInvitationErr = errors.New("database error")
			},
			wantErr: true,
		},
		{
			name: "get user by email error",
			req: &AcceptInvitationRequest{
				Token:    "valid-token",
				Password: "password123",
				Name:     "New User",
			},
			setupRepo: func(m *MockRepository) {
				m.invitations["valid-token"] = &UserInvitation{
					ID:        "inv-1",
					TenantID:  "tenant-1",
					Email:     "new@example.com",
					Role:      RoleAdmin,
					Token:     "valid-token",
					ExpiresAt: time.Now().Add(24 * time.Hour),
				}
				m.getUserByEmailErr = errors.New("database error")
			},
			wantErr:    true,
			errContain: "check user",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := NewMockRepository()
			if tt.setupRepo != nil {
				tt.setupRepo(repo)
			}
			svc := NewServiceWithRepository(repo)

			membership, err := svc.AcceptInvitation(context.Background(), tt.req)

			if tt.wantErr {
				require.Error(t, err)
				if tt.errContain != "" {
					assert.Contains(t, err.Error(), tt.errContain)
				}
				return
			}

			require.NoError(t, err)
			require.NotNil(t, membership)
		})
	}
}

func TestService_AcceptInvitation_GetTenantError(t *testing.T) {
	repo := NewMockRepository()
	// Setup valid invitation
	repo.invitations["valid-token"] = &UserInvitation{
		ID:        "inv-1",
		TenantID:  "tenant-1",
		Email:     "existing@example.com",
		Role:      RoleAdmin,
		Token:     "valid-token",
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}
	// Add existing user
	repo.AddTestUser(&User{ID: "user-1", Email: "existing@example.com"})
	// Don't add tenant - GetTenant will fail
	repo.getTenantErr = errors.New("tenant not found")

	svc := NewServiceWithRepository(repo)

	_, err := svc.AcceptInvitation(context.Background(), &AcceptInvitationRequest{
		Token: "valid-token",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "get tenant")
}

func TestService_CreateInvitation_CheckMemberError(t *testing.T) {
	repo := NewMockRepository()
	repo.checkUserIsMemberErr = errors.New("database error")
	svc := NewServiceWithRepository(repo)

	_, err := svc.CreateInvitation(context.Background(), "tenant-1", "user-1", &CreateInvitationRequest{
		Email: "test@example.com",
		Role:  RoleAdmin,
	})
	require.Error(t, err)
}

func TestService_CreateInvitation_RepositoryError(t *testing.T) {
	repo := NewMockRepository()
	repo.createInvitationErr = errors.New("database error")
	svc := NewServiceWithRepository(repo)

	_, err := svc.CreateInvitation(context.Background(), "tenant-1", "user-1", &CreateInvitationRequest{
		Email: "test@example.com",
		Role:  RoleAdmin,
	})
	require.Error(t, err)
}

func TestService_GetInvitationByToken_RepositoryError(t *testing.T) {
	repo := NewMockRepository()
	repo.getInvitationByTokenErr = errors.New("database error")
	svc := NewServiceWithRepository(repo)

	_, err := svc.GetInvitationByToken(context.Background(), "token")
	require.Error(t, err)
}

func TestService_RemoveTenantUser_RepositoryError(t *testing.T) {
	repo := NewMockRepository()
	repo.AddTestTenantUser(TenantUser{TenantID: "tenant-1", UserID: "user-1", Role: RoleAdmin})
	repo.getUserRoleErr = errors.New("database error")
	svc := NewServiceWithRepository(repo)

	err := svc.RemoveTenantUser(context.Background(), "tenant-1", "user-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "check user role")
}

func TestService_UpdateTenantUserRole_RepositoryError(t *testing.T) {
	repo := NewMockRepository()
	repo.AddTestTenantUser(TenantUser{TenantID: "tenant-1", UserID: "user-1", Role: RoleAdmin})
	repo.getUserRoleErr = errors.New("database error")
	svc := NewServiceWithRepository(repo)

	err := svc.UpdateTenantUserRole(context.Background(), "tenant-1", "user-1", RoleViewer)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "check current role")
}

func TestService_ListInvitations(t *testing.T) {
	repo := NewMockRepository()
	repo.invitations["token-1"] = &UserInvitation{
		ID:        "inv-1",
		TenantID:  "tenant-1",
		Email:     "test1@example.com",
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}
	repo.invitations["token-2"] = &UserInvitation{
		ID:        "inv-2",
		TenantID:  "tenant-1",
		Email:     "test2@example.com",
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}
	svc := NewServiceWithRepository(repo)

	invitations, err := svc.ListInvitations(context.Background(), "tenant-1")
	require.NoError(t, err)
	assert.Len(t, invitations, 2)
}

func TestService_RevokeInvitation(t *testing.T) {
	repo := NewMockRepository()
	repo.invitations["token-1"] = &UserInvitation{
		ID:        "inv-1",
		TenantID:  "tenant-1",
		Token:     "token-1",
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}
	svc := NewServiceWithRepository(repo)

	err := svc.RevokeInvitation(context.Background(), "tenant-1", "inv-1")
	require.NoError(t, err)

	// Verify deleted
	invitations, _ := svc.ListInvitations(context.Background(), "tenant-1")
	assert.Len(t, invitations, 0)
}

func TestService_RemoveTenantUser(t *testing.T) {
	tests := []struct {
		name       string
		tenantID   string
		userID     string
		setupRepo  func(*MockRepository)
		wantErr    bool
		errContain string
	}{
		{
			name:     "remove admin user",
			tenantID: "tenant-1",
			userID:   "user-1",
			setupRepo: func(m *MockRepository) {
				m.AddTestTenantUser(TenantUser{TenantID: "tenant-1", UserID: "user-1", Role: RoleAdmin})
			},
			wantErr: false,
		},
		{
			name:     "cannot remove owner",
			tenantID: "tenant-1",
			userID:   "user-owner",
			setupRepo: func(m *MockRepository) {
				m.AddTestTenantUser(TenantUser{TenantID: "tenant-1", UserID: "user-owner", Role: RoleOwner})
			},
			wantErr:    true,
			errContain: "cannot remove owner",
		},
		{
			name:     "user not in tenant",
			tenantID: "tenant-1",
			userID:   "nonexistent",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := NewMockRepository()
			if tt.setupRepo != nil {
				tt.setupRepo(repo)
			}
			svc := NewServiceWithRepository(repo)

			err := svc.RemoveTenantUser(context.Background(), tt.tenantID, tt.userID)

			if tt.wantErr {
				require.Error(t, err)
				if tt.errContain != "" {
					assert.Contains(t, err.Error(), tt.errContain)
				}
				return
			}

			require.NoError(t, err)
		})
	}
}

func TestService_UpdateTenantUserRole(t *testing.T) {
	tests := []struct {
		name       string
		tenantID   string
		userID     string
		newRole    string
		setupRepo  func(*MockRepository)
		wantErr    bool
		errContain string
	}{
		{
			name:     "update to admin",
			tenantID: "tenant-1",
			userID:   "user-1",
			newRole:  RoleAdmin,
			setupRepo: func(m *MockRepository) {
				m.AddTestTenantUser(TenantUser{TenantID: "tenant-1", UserID: "user-1", Role: RoleViewer})
			},
			wantErr: false,
		},
		{
			name:       "invalid role",
			tenantID:   "tenant-1",
			userID:     "user-1",
			newRole:    "invalid",
			wantErr:    true,
			errContain: "invalid role",
		},
		{
			name:     "cannot change owner role",
			tenantID: "tenant-1",
			userID:   "user-owner",
			newRole:  RoleAdmin,
			setupRepo: func(m *MockRepository) {
				m.AddTestTenantUser(TenantUser{TenantID: "tenant-1", UserID: "user-owner", Role: RoleOwner})
			},
			wantErr:    true,
			errContain: "cannot change owner role",
		},
		{
			name:     "user not in tenant",
			tenantID: "tenant-1",
			userID:   "nonexistent",
			newRole:  RoleAdmin,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := NewMockRepository()
			if tt.setupRepo != nil {
				tt.setupRepo(repo)
			}
			svc := NewServiceWithRepository(repo)

			err := svc.UpdateTenantUserRole(context.Background(), tt.tenantID, tt.userID, tt.newRole)

			if tt.wantErr {
				require.Error(t, err)
				if tt.errContain != "" {
					assert.Contains(t, err.Error(), tt.errContain)
				}
				return
			}

			require.NoError(t, err)
		})
	}
}

func TestService_ListTenantUsers(t *testing.T) {
	repo := NewMockRepository()
	repo.AddTestTenantUser(TenantUser{TenantID: "tenant-1", UserID: "user-1", Role: RoleOwner})
	repo.AddTestTenantUser(TenantUser{TenantID: "tenant-1", UserID: "user-2", Role: RoleAdmin})
	repo.AddTestTenantUser(TenantUser{TenantID: "tenant-1", UserID: "user-3", Role: RoleViewer})

	svc := NewServiceWithRepository(repo)

	users, err := svc.ListTenantUsers(context.Background(), "tenant-1")
	require.NoError(t, err)
	assert.Len(t, users, 3)
}

// Helper function for string pointers
func strPtr(s string) *string {
	return &s
}
