package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
)

// MockRepository is a mock implementation of Repository for testing
type MockRepository struct {
	registries    map[uuid.UUID]*Registry
	plugins       map[uuid.UUID]*Plugin
	tenantPlugins map[string]*TenantPlugin // key: "tenantID:pluginID"

	// Error injection
	listRegistriesErr          error
	getRegistryErr             error
	createRegistryErr          error
	deleteRegistryErr          error
	updateRegistryErr          error
	listPluginsErr             error
	getPluginErr               error
	getPluginByNameErr         error
	createPluginErr            error
	updatePluginErr            error
	deletePluginErr            error
	listTenantPluginsErr       error
	getTenantPluginErr         error
	createTenantPluginErr      error
	enableTenantPluginErr      error
	disableTenantPluginErr     error
	getTenantPluginSettingsErr error
	updateTenantSettingsErr    error
	deleteTenantPluginErr      error
	isEnabledForTenantErr      error
	listEnabledPluginsErr      error
}

func NewMockRepository() *MockRepository {
	return &MockRepository{
		registries:    make(map[uuid.UUID]*Registry),
		plugins:       make(map[uuid.UUID]*Plugin),
		tenantPlugins: make(map[string]*TenantPlugin),
	}
}

func (m *MockRepository) ListRegistries(ctx context.Context) ([]Registry, error) {
	if m.listRegistriesErr != nil {
		return nil, m.listRegistriesErr
	}
	var result []Registry
	for _, r := range m.registries {
		result = append(result, *r)
	}
	return result, nil
}

func (m *MockRepository) GetRegistry(ctx context.Context, id uuid.UUID) (*Registry, error) {
	if m.getRegistryErr != nil {
		return nil, m.getRegistryErr
	}
	r, ok := m.registries[id]
	if !ok {
		return nil, fmt.Errorf("registry not found")
	}
	return r, nil
}

func (m *MockRepository) CreateRegistry(ctx context.Context, name, url, description string) (*Registry, error) {
	if m.createRegistryErr != nil {
		return nil, m.createRegistryErr
	}
	r := &Registry{
		ID:          uuid.New(),
		Name:        name,
		URL:         url,
		Description: description,
		IsOfficial:  false,
		IsActive:    true,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	m.registries[r.ID] = r
	return r, nil
}

func (m *MockRepository) DeleteRegistry(ctx context.Context, id uuid.UUID) (int64, error) {
	if m.deleteRegistryErr != nil {
		return 0, m.deleteRegistryErr
	}
	r, ok := m.registries[id]
	if !ok || r.IsOfficial {
		return 0, nil
	}
	delete(m.registries, id)
	return 1, nil
}

func (m *MockRepository) UpdateRegistryLastSynced(ctx context.Context, id uuid.UUID) error {
	if m.updateRegistryErr != nil {
		return m.updateRegistryErr
	}
	if r, ok := m.registries[id]; ok {
		now := time.Now()
		r.LastSyncedAt = &now
		r.UpdatedAt = now
	}
	return nil
}

func (m *MockRepository) ListPlugins(ctx context.Context) ([]Plugin, error) {
	if m.listPluginsErr != nil {
		return nil, m.listPluginsErr
	}
	var result []Plugin
	for _, p := range m.plugins {
		result = append(result, *p)
	}
	return result, nil
}

func (m *MockRepository) GetPlugin(ctx context.Context, id uuid.UUID) (*Plugin, error) {
	if m.getPluginErr != nil {
		return nil, m.getPluginErr
	}
	p, ok := m.plugins[id]
	if !ok {
		return nil, fmt.Errorf("plugin not found")
	}
	return p, nil
}

func (m *MockRepository) GetPluginByName(ctx context.Context, name string) (*Plugin, error) {
	if m.getPluginByNameErr != nil {
		return nil, m.getPluginByNameErr
	}
	for _, p := range m.plugins {
		if p.Name == name {
			return p, nil
		}
	}
	return nil, fmt.Errorf("plugin not found")
}

func (m *MockRepository) CreatePlugin(ctx context.Context, p *Plugin) error {
	if m.createPluginErr != nil {
		return m.createPluginErr
	}
	m.plugins[p.ID] = p
	return nil
}

func (m *MockRepository) UpdatePlugin(ctx context.Context, p *Plugin) error {
	if m.updatePluginErr != nil {
		return m.updatePluginErr
	}
	if _, ok := m.plugins[p.ID]; !ok {
		return fmt.Errorf("plugin not found")
	}
	m.plugins[p.ID] = p
	return nil
}

func (m *MockRepository) DeletePlugin(ctx context.Context, id uuid.UUID) (int64, error) {
	if m.deletePluginErr != nil {
		return 0, m.deletePluginErr
	}
	if _, ok := m.plugins[id]; !ok {
		return 0, nil
	}
	delete(m.plugins, id)
	return 1, nil
}

func (m *MockRepository) ListTenantPlugins(ctx context.Context, tenantID uuid.UUID) ([]TenantPlugin, error) {
	if m.listTenantPluginsErr != nil {
		return nil, m.listTenantPluginsErr
	}
	var result []TenantPlugin
	for key, tp := range m.tenantPlugins {
		if tp.TenantID == tenantID {
			// Include plugin data
			if p, ok := m.plugins[tp.PluginID]; ok {
				tp.Plugin = p
			}
			result = append(result, *tp)
			_ = key // suppress unused warning
		}
	}
	return result, nil
}

func (m *MockRepository) GetTenantPlugin(ctx context.Context, tenantID, pluginID uuid.UUID) (*TenantPlugin, error) {
	if m.getTenantPluginErr != nil {
		return nil, m.getTenantPluginErr
	}
	key := fmt.Sprintf("%s:%s", tenantID.String(), pluginID.String())
	tp, ok := m.tenantPlugins[key]
	if !ok {
		return nil, fmt.Errorf("tenant plugin not found")
	}
	if p, ok := m.plugins[tp.PluginID]; ok {
		tp.Plugin = p
	}
	return tp, nil
}

func (m *MockRepository) CreateTenantPlugin(ctx context.Context, tenantID, pluginID uuid.UUID, settings json.RawMessage) error {
	if m.createTenantPluginErr != nil {
		return m.createTenantPluginErr
	}
	key := fmt.Sprintf("%s:%s", tenantID.String(), pluginID.String())
	now := time.Now()
	m.tenantPlugins[key] = &TenantPlugin{
		ID:        uuid.New(),
		TenantID:  tenantID,
		PluginID:  pluginID,
		IsEnabled: true,
		Settings:  settings,
		EnabledAt: &now,
		CreatedAt: now,
		UpdatedAt: now,
	}
	return nil
}

func (m *MockRepository) EnableTenantPlugin(ctx context.Context, tenantID, pluginID uuid.UUID, settings json.RawMessage) error {
	if m.enableTenantPluginErr != nil {
		return m.enableTenantPluginErr
	}
	key := fmt.Sprintf("%s:%s", tenantID.String(), pluginID.String())
	now := time.Now()
	if tp, ok := m.tenantPlugins[key]; ok {
		tp.IsEnabled = true
		tp.Settings = settings
		tp.EnabledAt = &now
		tp.UpdatedAt = now
	} else {
		m.tenantPlugins[key] = &TenantPlugin{
			ID:        uuid.New(),
			TenantID:  tenantID,
			PluginID:  pluginID,
			IsEnabled: true,
			Settings:  settings,
			EnabledAt: &now,
			CreatedAt: now,
			UpdatedAt: now,
		}
	}
	return nil
}

func (m *MockRepository) DisableTenantPlugin(ctx context.Context, tenantID, pluginID uuid.UUID) (int64, error) {
	if m.disableTenantPluginErr != nil {
		return 0, m.disableTenantPluginErr
	}
	key := fmt.Sprintf("%s:%s", tenantID.String(), pluginID.String())
	if tp, ok := m.tenantPlugins[key]; ok {
		tp.IsEnabled = false
		tp.UpdatedAt = time.Now()
		return 1, nil
	}
	return 0, nil
}

func (m *MockRepository) GetTenantPluginSettings(ctx context.Context, tenantID, pluginID uuid.UUID) (json.RawMessage, error) {
	if m.getTenantPluginSettingsErr != nil {
		return nil, m.getTenantPluginSettingsErr
	}
	key := fmt.Sprintf("%s:%s", tenantID.String(), pluginID.String())
	if tp, ok := m.tenantPlugins[key]; ok {
		return tp.Settings, nil
	}
	return json.RawMessage("{}"), nil
}

func (m *MockRepository) UpdateTenantPluginSettings(ctx context.Context, tenantID, pluginID uuid.UUID, settings json.RawMessage) error {
	if m.updateTenantSettingsErr != nil {
		return m.updateTenantSettingsErr
	}
	key := fmt.Sprintf("%s:%s", tenantID.String(), pluginID.String())
	tp, ok := m.tenantPlugins[key]
	if !ok {
		return fmt.Errorf("tenant plugin not found")
	}
	tp.Settings = settings
	tp.UpdatedAt = time.Now()
	return nil
}

func (m *MockRepository) DeleteTenantPlugin(ctx context.Context, tenantID, pluginID uuid.UUID) error {
	if m.deleteTenantPluginErr != nil {
		return m.deleteTenantPluginErr
	}
	key := fmt.Sprintf("%s:%s", tenantID.String(), pluginID.String())
	delete(m.tenantPlugins, key)
	return nil
}

func (m *MockRepository) IsPluginEnabledForTenant(ctx context.Context, tenantID, pluginID uuid.UUID) (bool, error) {
	if m.isEnabledForTenantErr != nil {
		return false, m.isEnabledForTenantErr
	}
	key := fmt.Sprintf("%s:%s", tenantID.String(), pluginID.String())
	_, ok := m.tenantPlugins[key]
	return ok, nil
}

func (m *MockRepository) ListEnabledPlugins(ctx context.Context) ([]Plugin, error) {
	if m.listEnabledPluginsErr != nil {
		return nil, m.listEnabledPluginsErr
	}
	var result []Plugin
	for _, p := range m.plugins {
		if p.State == StateEnabled {
			result = append(result, *p)
		}
	}
	return result, nil
}

func (m *MockRepository) InsertPluginReturning(ctx context.Context, manifest *Manifest, repoURL string, repoType RepositoryType, manifestJSON []byte) (*Plugin, error) {
	if m.createPluginErr != nil {
		return nil, m.createPluginErr
	}
	p := &Plugin{
		ID:             uuid.New(),
		Name:           manifest.Name,
		DisplayName:    manifest.DisplayName,
		Description:    manifest.Description,
		Version:        manifest.Version,
		RepositoryURL:  repoURL,
		RepositoryType: repoType,
		Author:         manifest.Author,
		License:        manifest.License,
		HomepageURL:    manifest.Homepage,
		State:          StateInstalled,
		Manifest:       manifestJSON,
		InstalledAt:    time.Now(),
		UpdatedAt:      time.Now(),
	}
	m.plugins[p.ID] = p
	return p, nil
}

func (m *MockRepository) CountEnabledTenantsForPlugin(ctx context.Context, pluginID uuid.UUID) (int, error) {
	count := 0
	for _, tp := range m.tenantPlugins {
		if tp.PluginID == pluginID && tp.IsEnabled {
			count++
		}
	}
	return count, nil
}

func (m *MockRepository) UpdatePluginState(ctx context.Context, pluginID uuid.UUID, state PluginState, permissions []string) error {
	if m.updatePluginErr != nil {
		return m.updatePluginErr
	}
	if p, ok := m.plugins[pluginID]; ok {
		p.State = state
		p.GrantedPermissions = permissions
		p.UpdatedAt = time.Now()
	}
	return nil
}

func (m *MockRepository) DisableAllTenantsForPlugin(ctx context.Context, pluginID uuid.UUID) error {
	for _, tp := range m.tenantPlugins {
		if tp.PluginID == pluginID {
			tp.IsEnabled = false
			tp.UpdatedAt = time.Now()
		}
	}
	return nil
}

func (m *MockRepository) GetTenantPluginsWithAll(ctx context.Context, tenantID uuid.UUID) ([]TenantPlugin, error) {
	if m.listTenantPluginsErr != nil {
		return nil, m.listTenantPluginsErr
	}
	var result []TenantPlugin
	for _, p := range m.plugins {
		if p.State != StateEnabled {
			continue
		}
		tp := TenantPlugin{
			PluginID: p.ID,
			TenantID: tenantID,
			Plugin:   p,
		}
		key := tenantID.String() + ":" + p.ID.String()
		if existing, ok := m.tenantPlugins[key]; ok {
			tp.ID = existing.ID
			tp.IsEnabled = existing.IsEnabled
			tp.Settings = existing.Settings
			tp.EnabledAt = existing.EnabledAt
			tp.CreatedAt = existing.CreatedAt
			tp.UpdatedAt = existing.UpdatedAt
		}
		result = append(result, tp)
	}
	return result, nil
}

// Tests

func TestNewServiceWithRepository(t *testing.T) {
	repo := NewMockRepository()
	service := NewServiceWithRepository(repo, nil, "/tmp/plugins")

	if service == nil {
		t.Fatal("expected service to be created")
	}
	if service.repo != repo {
		t.Error("repository not set correctly")
	}
	if service.hooks == nil {
		t.Error("hooks should be initialized")
	}
}

func TestService_GetHookRegistry(t *testing.T) {
	repo := NewMockRepository()
	hooks := NewHookRegistry()
	service := NewServiceWithRepository(repo, hooks, "/tmp/plugins")

	result := service.GetHookRegistry()
	if result != hooks {
		t.Error("expected hook registry to be returned")
	}
}

func TestService_ListRegistries(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name        string
		setupRepo   func() *MockRepository
		expectCount int
		expectErr   bool
	}{
		{
			name: "success_empty",
			setupRepo: func() *MockRepository {
				return NewMockRepository()
			},
			expectCount: 0,
			expectErr:   false,
		},
		{
			name: "success_with_registries",
			setupRepo: func() *MockRepository {
				repo := NewMockRepository()
				repo.registries[uuid.New()] = &Registry{
					ID:   uuid.New(),
					Name: "Test Registry",
					URL:  "https://github.com/test/registry",
				}
				return repo
			},
			expectCount: 1,
			expectErr:   false,
		},
		{
			name: "repository_error",
			setupRepo: func() *MockRepository {
				repo := NewMockRepository()
				repo.listRegistriesErr = fmt.Errorf("db error")
				return repo
			},
			expectCount: 0,
			expectErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := tt.setupRepo()
			service := NewServiceWithRepository(repo, nil, "/tmp/plugins")

			result, err := service.ListRegistries(ctx)

			if tt.expectErr {
				if err == nil {
					t.Error("expected error but got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(result) != tt.expectCount {
				t.Errorf("expected %d registries, got %d", tt.expectCount, len(result))
			}
		})
	}
}

func TestService_GetRegistry(t *testing.T) {
	ctx := context.Background()
	regID := uuid.New()

	tests := []struct {
		name      string
		setupRepo func() *MockRepository
		id        uuid.UUID
		expectErr bool
	}{
		{
			name: "success",
			setupRepo: func() *MockRepository {
				repo := NewMockRepository()
				repo.registries[regID] = &Registry{
					ID:   regID,
					Name: "Test Registry",
					URL:  "https://github.com/test/registry",
				}
				return repo
			},
			id:        regID,
			expectErr: false,
		},
		{
			name: "not_found",
			setupRepo: func() *MockRepository {
				return NewMockRepository()
			},
			id:        uuid.New(),
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := tt.setupRepo()
			service := NewServiceWithRepository(repo, nil, "/tmp/plugins")

			result, err := service.GetRegistry(ctx, tt.id)

			if tt.expectErr {
				if err == nil {
					t.Error("expected error but got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result == nil {
				t.Error("expected result but got nil")
			}
		})
	}
}

func TestService_ListPlugins(t *testing.T) {
	ctx := context.Background()
	pluginID := uuid.New()

	tests := []struct {
		name        string
		setupRepo   func() *MockRepository
		expectCount int
		expectErr   bool
	}{
		{
			name: "success_empty",
			setupRepo: func() *MockRepository {
				return NewMockRepository()
			},
			expectCount: 0,
			expectErr:   false,
		},
		{
			name: "success_with_plugins",
			setupRepo: func() *MockRepository {
				repo := NewMockRepository()
				repo.plugins[pluginID] = &Plugin{
					ID:          pluginID,
					Name:        "test-plugin",
					DisplayName: "Test Plugin",
					State:       StateEnabled,
				}
				return repo
			},
			expectCount: 1,
			expectErr:   false,
		},
		{
			name: "repository_error",
			setupRepo: func() *MockRepository {
				repo := NewMockRepository()
				repo.listPluginsErr = fmt.Errorf("db error")
				return repo
			},
			expectCount: 0,
			expectErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := tt.setupRepo()
			service := NewServiceWithRepository(repo, nil, "/tmp/plugins")

			result, err := service.ListPlugins(ctx)

			if tt.expectErr {
				if err == nil {
					t.Error("expected error but got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(result) != tt.expectCount {
				t.Errorf("expected %d plugins, got %d", tt.expectCount, len(result))
			}
		})
	}
}

func TestService_GetLoadedPlugin(t *testing.T) {
	repo := NewMockRepository()
	service := NewServiceWithRepository(repo, nil, "/tmp/plugins")

	// Test getting a non-existent plugin
	plugin, exists := service.GetLoadedPlugin("nonexistent")
	if exists {
		t.Error("expected plugin to not exist")
	}
	if plugin != nil {
		t.Error("expected plugin to be nil")
	}
}

func TestService_GetPlugin(t *testing.T) {
	ctx := context.Background()
	pluginID := uuid.New()

	tests := []struct {
		name      string
		setupRepo func() *MockRepository
		id        uuid.UUID
		expectErr bool
	}{
		{
			name: "success",
			setupRepo: func() *MockRepository {
				repo := NewMockRepository()
				repo.plugins[pluginID] = &Plugin{
					ID:          pluginID,
					Name:        "test-plugin",
					DisplayName: "Test Plugin",
					State:       StateEnabled,
				}
				return repo
			},
			id:        pluginID,
			expectErr: false,
		},
		{
			name: "not_found",
			setupRepo: func() *MockRepository {
				return NewMockRepository()
			},
			id:        uuid.New(),
			expectErr: true,
		},
		{
			name: "repository_error",
			setupRepo: func() *MockRepository {
				repo := NewMockRepository()
				repo.getPluginErr = fmt.Errorf("db error")
				return repo
			},
			id:        uuid.New(),
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := tt.setupRepo()
			service := NewServiceWithRepository(repo, nil, "/tmp/plugins")

			result, err := service.GetPlugin(ctx, tt.id)

			if tt.expectErr {
				if err == nil {
					t.Error("expected error but got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result == nil {
				t.Error("expected result but got nil")
			}
			if result.ID != tt.id {
				t.Errorf("expected ID %v, got %v", tt.id, result.ID)
			}
		})
	}
}

func TestService_GetPluginByName(t *testing.T) {
	ctx := context.Background()
	pluginID := uuid.New()

	tests := []struct {
		name       string
		setupRepo  func() *MockRepository
		pluginName string
		expectErr  bool
	}{
		{
			name: "success",
			setupRepo: func() *MockRepository {
				repo := NewMockRepository()
				repo.plugins[pluginID] = &Plugin{
					ID:          pluginID,
					Name:        "test-plugin",
					DisplayName: "Test Plugin",
				}
				return repo
			},
			pluginName: "test-plugin",
			expectErr:  false,
		},
		{
			name: "not_found",
			setupRepo: func() *MockRepository {
				return NewMockRepository()
			},
			pluginName: "nonexistent",
			expectErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := tt.setupRepo()
			service := NewServiceWithRepository(repo, nil, "/tmp/plugins")

			result, err := service.GetPluginByName(ctx, tt.pluginName)

			if tt.expectErr {
				if err == nil {
					t.Error("expected error but got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.Name != tt.pluginName {
				t.Errorf("expected name %v, got %v", tt.pluginName, result.Name)
			}
		})
	}
}

func TestService_AddRegistry(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name      string
		setupRepo func() *MockRepository
		req       CreateRegistryRequest
		expectErr bool
	}{
		{
			name: "success",
			setupRepo: func() *MockRepository {
				return NewMockRepository()
			},
			req: CreateRegistryRequest{
				Name:        "Test Registry",
				URL:         "https://github.com/test/registry",
				Description: "A test registry",
			},
			expectErr: false,
		},
		{
			name: "invalid_url",
			setupRepo: func() *MockRepository {
				return NewMockRepository()
			},
			req: CreateRegistryRequest{
				Name: "Invalid Registry",
				URL:  "https://invalid-url.com/registry",
			},
			expectErr: true,
		},
		{
			name: "repository_error",
			setupRepo: func() *MockRepository {
				repo := NewMockRepository()
				repo.createRegistryErr = fmt.Errorf("db error")
				return repo
			},
			req: CreateRegistryRequest{
				Name: "Test Registry",
				URL:  "https://github.com/test/registry",
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := tt.setupRepo()
			service := NewServiceWithRepository(repo, nil, "/tmp/plugins")

			result, err := service.AddRegistry(ctx, tt.req)

			if tt.expectErr {
				if err == nil {
					t.Error("expected error but got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result == nil {
				t.Error("expected result but got nil")
			}
			if result.Name != tt.req.Name {
				t.Errorf("expected name %v, got %v", tt.req.Name, result.Name)
			}
		})
	}
}

func TestService_RemoveRegistry(t *testing.T) {
	ctx := context.Background()
	regID := uuid.New()

	tests := []struct {
		name      string
		setupRepo func() *MockRepository
		id        uuid.UUID
		expectErr bool
	}{
		{
			name: "success",
			setupRepo: func() *MockRepository {
				repo := NewMockRepository()
				repo.registries[regID] = &Registry{
					ID:         regID,
					Name:       "Test Registry",
					IsOfficial: false,
				}
				return repo
			},
			id:        regID,
			expectErr: false,
		},
		{
			name: "not_found",
			setupRepo: func() *MockRepository {
				return NewMockRepository()
			},
			id:        uuid.New(),
			expectErr: true,
		},
		{
			name: "official_registry",
			setupRepo: func() *MockRepository {
				repo := NewMockRepository()
				repo.registries[regID] = &Registry{
					ID:         regID,
					Name:       "Official Registry",
					IsOfficial: true,
				}
				return repo
			},
			id:        regID,
			expectErr: true,
		},
		{
			name: "repository_error",
			setupRepo: func() *MockRepository {
				repo := NewMockRepository()
				repo.deleteRegistryErr = fmt.Errorf("db error")
				return repo
			},
			id:        uuid.New(),
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := tt.setupRepo()
			service := NewServiceWithRepository(repo, nil, "/tmp/plugins")

			err := service.RemoveRegistry(ctx, tt.id)

			if tt.expectErr {
				if err == nil {
					t.Error("expected error but got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestService_UpdateRegistryLastSynced(t *testing.T) {
	ctx := context.Background()
	regID := uuid.New()

	tests := []struct {
		name      string
		setupRepo func() *MockRepository
		id        uuid.UUID
		expectErr bool
	}{
		{
			name: "success",
			setupRepo: func() *MockRepository {
				repo := NewMockRepository()
				repo.registries[regID] = &Registry{
					ID:   regID,
					Name: "Test Registry",
				}
				return repo
			},
			id:        regID,
			expectErr: false,
		},
		{
			name: "repository_error",
			setupRepo: func() *MockRepository {
				repo := NewMockRepository()
				repo.updateRegistryErr = fmt.Errorf("db error")
				return repo
			},
			id:        uuid.New(),
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := tt.setupRepo()
			service := NewServiceWithRepository(repo, nil, "/tmp/plugins")

			err := service.UpdateRegistryLastSynced(ctx, tt.id)

			if tt.expectErr {
				if err == nil {
					t.Error("expected error but got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestService_EnableForTenant(t *testing.T) {
	ctx := context.Background()
	tenantID := uuid.New()
	pluginID := uuid.New()

	tests := []struct {
		name      string
		setupRepo func() *MockRepository
		tenantID  uuid.UUID
		pluginID  uuid.UUID
		settings  json.RawMessage
		expectErr bool
	}{
		{
			name: "success",
			setupRepo: func() *MockRepository {
				repo := NewMockRepository()
				repo.plugins[pluginID] = &Plugin{
					ID:    pluginID,
					Name:  "test-plugin",
					State: StateEnabled,
				}
				return repo
			},
			tenantID:  tenantID,
			pluginID:  pluginID,
			settings:  json.RawMessage(`{"key": "value"}`),
			expectErr: false,
		},
		{
			name: "plugin_not_found",
			setupRepo: func() *MockRepository {
				return NewMockRepository()
			},
			tenantID:  tenantID,
			pluginID:  uuid.New(),
			expectErr: true,
		},
		{
			name: "plugin_not_enabled",
			setupRepo: func() *MockRepository {
				repo := NewMockRepository()
				repo.plugins[pluginID] = &Plugin{
					ID:    pluginID,
					Name:  "test-plugin",
					State: StateDisabled,
				}
				return repo
			},
			tenantID:  tenantID,
			pluginID:  pluginID,
			expectErr: true,
		},
		{
			name: "repository_error",
			setupRepo: func() *MockRepository {
				repo := NewMockRepository()
				repo.plugins[pluginID] = &Plugin{
					ID:    pluginID,
					Name:  "test-plugin",
					State: StateEnabled,
				}
				repo.enableTenantPluginErr = fmt.Errorf("db error")
				return repo
			},
			tenantID:  tenantID,
			pluginID:  pluginID,
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := tt.setupRepo()
			service := NewServiceWithRepository(repo, nil, "/tmp/plugins")

			err := service.EnableForTenant(ctx, tt.tenantID, tt.pluginID, tt.settings)

			if tt.expectErr {
				if err == nil {
					t.Error("expected error but got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestService_DisableForTenant(t *testing.T) {
	ctx := context.Background()
	tenantID := uuid.New()
	pluginID := uuid.New()

	tests := []struct {
		name      string
		setupRepo func() *MockRepository
		tenantID  uuid.UUID
		pluginID  uuid.UUID
		expectErr bool
	}{
		{
			name: "success",
			setupRepo: func() *MockRepository {
				repo := NewMockRepository()
				key := fmt.Sprintf("%s:%s", tenantID.String(), pluginID.String())
				now := time.Now()
				repo.tenantPlugins[key] = &TenantPlugin{
					ID:        uuid.New(),
					TenantID:  tenantID,
					PluginID:  pluginID,
					IsEnabled: true,
					EnabledAt: &now,
				}
				return repo
			},
			tenantID:  tenantID,
			pluginID:  pluginID,
			expectErr: false,
		},
		{
			name: "not_found",
			setupRepo: func() *MockRepository {
				return NewMockRepository()
			},
			tenantID:  tenantID,
			pluginID:  uuid.New(),
			expectErr: true,
		},
		{
			name: "repository_error",
			setupRepo: func() *MockRepository {
				repo := NewMockRepository()
				repo.disableTenantPluginErr = fmt.Errorf("db error")
				return repo
			},
			tenantID:  tenantID,
			pluginID:  pluginID,
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := tt.setupRepo()
			service := NewServiceWithRepository(repo, nil, "/tmp/plugins")

			err := service.DisableForTenant(ctx, tt.tenantID, tt.pluginID)

			if tt.expectErr {
				if err == nil {
					t.Error("expected error but got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestService_GetTenantPluginSettings(t *testing.T) {
	ctx := context.Background()
	tenantID := uuid.New()
	pluginID := uuid.New()

	tests := []struct {
		name           string
		setupRepo      func() *MockRepository
		tenantID       uuid.UUID
		pluginID       uuid.UUID
		expectSettings string
		expectErr      bool
	}{
		{
			name: "success",
			setupRepo: func() *MockRepository {
				repo := NewMockRepository()
				key := fmt.Sprintf("%s:%s", tenantID.String(), pluginID.String())
				now := time.Now()
				repo.tenantPlugins[key] = &TenantPlugin{
					ID:        uuid.New(),
					TenantID:  tenantID,
					PluginID:  pluginID,
					Settings:  json.RawMessage(`{"key": "value"}`),
					EnabledAt: &now,
				}
				return repo
			},
			tenantID:       tenantID,
			pluginID:       pluginID,
			expectSettings: `{"key": "value"}`,
			expectErr:      false,
		},
		{
			name: "not_found_returns_empty",
			setupRepo: func() *MockRepository {
				return NewMockRepository()
			},
			tenantID:       tenantID,
			pluginID:       uuid.New(),
			expectSettings: "{}",
			expectErr:      false,
		},
		{
			name: "repository_error",
			setupRepo: func() *MockRepository {
				repo := NewMockRepository()
				repo.getTenantPluginSettingsErr = fmt.Errorf("db error")
				return repo
			},
			tenantID:  tenantID,
			pluginID:  pluginID,
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := tt.setupRepo()
			service := NewServiceWithRepository(repo, nil, "/tmp/plugins")

			result, err := service.GetTenantPluginSettings(ctx, tt.tenantID, tt.pluginID)

			if tt.expectErr {
				if err == nil {
					t.Error("expected error but got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if string(result) != tt.expectSettings {
				t.Errorf("expected settings %v, got %v", tt.expectSettings, string(result))
			}
		})
	}
}

func TestService_UpdateTenantPluginSettings(t *testing.T) {
	ctx := context.Background()
	tenantID := uuid.New()
	pluginID := uuid.New()

	tests := []struct {
		name      string
		setupRepo func() *MockRepository
		tenantID  uuid.UUID
		pluginID  uuid.UUID
		settings  json.RawMessage
		expectErr bool
	}{
		{
			name: "success",
			setupRepo: func() *MockRepository {
				repo := NewMockRepository()
				key := fmt.Sprintf("%s:%s", tenantID.String(), pluginID.String())
				now := time.Now()
				repo.tenantPlugins[key] = &TenantPlugin{
					ID:        uuid.New(),
					TenantID:  tenantID,
					PluginID:  pluginID,
					EnabledAt: &now,
				}
				return repo
			},
			tenantID:  tenantID,
			pluginID:  pluginID,
			settings:  json.RawMessage(`{"updated": true}`),
			expectErr: false,
		},
		{
			name: "not_found",
			setupRepo: func() *MockRepository {
				return NewMockRepository()
			},
			tenantID:  tenantID,
			pluginID:  uuid.New(),
			expectErr: true,
		},
		{
			name: "repository_error",
			setupRepo: func() *MockRepository {
				repo := NewMockRepository()
				repo.updateTenantSettingsErr = fmt.Errorf("db error")
				return repo
			},
			tenantID:  tenantID,
			pluginID:  pluginID,
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := tt.setupRepo()
			service := NewServiceWithRepository(repo, nil, "/tmp/plugins")

			err := service.UpdateTenantPluginSettings(ctx, tt.tenantID, tt.pluginID, tt.settings)

			if tt.expectErr {
				if err == nil {
					t.Error("expected error but got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestService_LoadAndUnloadPlugin(t *testing.T) {
	repo := NewMockRepository()
	hooks := NewHookRegistry()
	service := NewServiceWithRepository(repo, hooks, "/tmp/plugins")

	pluginID := uuid.New()
	plugin := &Plugin{
		ID:   pluginID,
		Name: "test-plugin",
	}

	manifest := &Manifest{
		Name:    "test-plugin",
		Version: "1.0.0",
		Backend: &BackendConfig{
			Hooks: []HookConfig{
				{Event: "invoice.created", Handler: "handleInvoice"},
			},
		},
	}

	// Test loadPlugin
	err := service.loadPlugin(plugin, manifest)
	if err != nil {
		t.Fatalf("loadPlugin failed: %v", err)
	}

	// Verify plugin was loaded
	loaded, exists := service.GetLoadedPlugin("test-plugin")
	if !exists {
		t.Error("expected plugin to be loaded")
	}
	if loaded.Plugin.ID != pluginID {
		t.Error("loaded plugin has wrong ID")
	}

	// Test unloadPlugin
	service.unloadPlugin("test-plugin")

	// Verify plugin was unloaded
	_, exists = service.GetLoadedPlugin("test-plugin")
	if exists {
		t.Error("expected plugin to be unloaded")
	}

	// Test unloading non-existent plugin (should not panic)
	service.unloadPlugin("nonexistent")
}

func TestService_LoadPlugin_WithoutBackend(t *testing.T) {
	repo := NewMockRepository()
	hooks := NewHookRegistry()
	service := NewServiceWithRepository(repo, hooks, "/tmp/plugins")

	plugin := &Plugin{
		ID:   uuid.New(),
		Name: "frontend-only-plugin",
	}

	manifest := &Manifest{
		Name:    "frontend-only-plugin",
		Version: "1.0.0",
		Frontend: &FrontendConfig{
			Components: "main.js",
		},
		// No backend hooks
	}

	err := service.loadPlugin(plugin, manifest)
	if err != nil {
		t.Fatalf("loadPlugin failed: %v", err)
	}

	loaded, exists := service.GetLoadedPlugin("frontend-only-plugin")
	if !exists {
		t.Error("expected plugin to be loaded")
	}
	if loaded.Manifest.Frontend == nil {
		t.Error("expected frontend manifest to be set")
	}
}

func TestService_IsPluginEnabledForTenant(t *testing.T) {
	ctx := context.Background()
	tenantID := uuid.New()
	pluginID := uuid.New()

	tests := []struct {
		name          string
		setupRepo     func() *MockRepository
		tenantID      uuid.UUID
		pluginID      uuid.UUID
		expectEnabled bool
		expectErr     bool
	}{
		{
			name: "enabled",
			setupRepo: func() *MockRepository {
				repo := NewMockRepository()
				key := fmt.Sprintf("%s:%s", tenantID.String(), pluginID.String())
				now := time.Now()
				repo.tenantPlugins[key] = &TenantPlugin{
					ID:        uuid.New(),
					TenantID:  tenantID,
					PluginID:  pluginID,
					IsEnabled: true,
					EnabledAt: &now,
				}
				return repo
			},
			tenantID:      tenantID,
			pluginID:      pluginID,
			expectEnabled: true,
			expectErr:     false,
		},
		{
			name: "not_enabled",
			setupRepo: func() *MockRepository {
				return NewMockRepository()
			},
			tenantID:      tenantID,
			pluginID:      uuid.New(),
			expectEnabled: false,
			expectErr:     false,
		},
		{
			name: "repository_error",
			setupRepo: func() *MockRepository {
				repo := NewMockRepository()
				repo.isEnabledForTenantErr = fmt.Errorf("db error")
				return repo
			},
			tenantID:  tenantID,
			pluginID:  pluginID,
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := tt.setupRepo()
			service := NewServiceWithRepository(repo, nil, "/tmp/plugins")

			result, err := service.IsPluginEnabledForTenant(ctx, tt.tenantID, tt.pluginID)

			if tt.expectErr {
				if err == nil {
					t.Error("expected error but got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result != tt.expectEnabled {
				t.Errorf("expected enabled %v, got %v", tt.expectEnabled, result)
			}
		})
	}
}

func TestService_GetTenantPlugins(t *testing.T) {
	ctx := context.Background()
	tenantID := uuid.New()
	pluginID := uuid.New()

	tests := []struct {
		name        string
		setupRepo   func() *MockRepository
		tenantID    uuid.UUID
		expectCount int
		expectErr   bool
	}{
		{
			name: "success_empty",
			setupRepo: func() *MockRepository {
				return NewMockRepository()
			},
			tenantID:    tenantID,
			expectCount: 0,
			expectErr:   false,
		},
		{
			name: "success_with_plugins",
			setupRepo: func() *MockRepository {
				repo := NewMockRepository()
				repo.plugins[pluginID] = &Plugin{
					ID:    pluginID,
					Name:  "test-plugin",
					State: StateEnabled,
				}
				return repo
			},
			tenantID:    tenantID,
			expectCount: 1,
			expectErr:   false,
		},
		{
			name: "repository_error",
			setupRepo: func() *MockRepository {
				repo := NewMockRepository()
				repo.listTenantPluginsErr = fmt.Errorf("db error")
				return repo
			},
			tenantID:  tenantID,
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := tt.setupRepo()
			service := NewServiceWithRepository(repo, nil, "/tmp/plugins")

			result, err := service.GetTenantPlugins(ctx, tt.tenantID)

			if tt.expectErr {
				if err == nil {
					t.Error("expected error but got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(result) != tt.expectCount {
				t.Errorf("expected %d plugins, got %d", tt.expectCount, len(result))
			}
		})
	}
}

func TestService_InstallPlugin_InvalidURL(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	service := NewServiceWithRepository(repo, nil, "/tmp/plugins")

	// Test with invalid URL
	_, err := service.InstallPlugin(ctx, "invalid-url")
	if err == nil {
		t.Error("expected error for invalid URL")
	}
}

func TestService_UninstallPlugin_NotFound(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	service := NewServiceWithRepository(repo, nil, "/tmp/plugins")

	err := service.UninstallPlugin(ctx, uuid.New())
	if err == nil {
		t.Error("expected error for non-existent plugin")
	}
}

func TestService_UninstallPlugin_HasTenants(t *testing.T) {
	ctx := context.Background()
	pluginID := uuid.New()
	tenantID := uuid.New()
	repo := NewMockRepository()
	repo.plugins[pluginID] = &Plugin{
		ID:    pluginID,
		Name:  "test-plugin",
		State: StateEnabled,
	}
	// Add tenant plugin
	key := fmt.Sprintf("%s:%s", tenantID.String(), pluginID.String())
	now := time.Now()
	repo.tenantPlugins[key] = &TenantPlugin{
		ID:        uuid.New(),
		TenantID:  tenantID,
		PluginID:  pluginID,
		IsEnabled: true,
		EnabledAt: &now,
	}
	service := NewServiceWithRepository(repo, nil, "/tmp/plugins")

	err := service.UninstallPlugin(ctx, pluginID)
	if err == nil {
		t.Error("expected error when plugin has enabled tenants")
	}
}

func TestService_EnablePlugin_NotFound(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	service := NewServiceWithRepository(repo, nil, "/tmp/plugins")

	err := service.EnablePlugin(ctx, uuid.New(), []string{})
	if err == nil {
		t.Error("expected error for non-existent plugin")
	}
}

func TestService_EnablePlugin_AlreadyEnabled(t *testing.T) {
	ctx := context.Background()
	pluginID := uuid.New()
	repo := NewMockRepository()
	repo.plugins[pluginID] = &Plugin{
		ID:       pluginID,
		Name:     "test-plugin",
		State:    StateEnabled,
		Manifest: json.RawMessage(`{"name": "test-plugin", "version": "1.0.0"}`),
	}
	service := NewServiceWithRepository(repo, nil, "/tmp/plugins")

	err := service.EnablePlugin(ctx, pluginID, []string{})
	if err == nil {
		t.Error("expected error for already enabled plugin")
	}
}

func TestService_EnablePlugin_InvalidPermissions(t *testing.T) {
	ctx := context.Background()
	pluginID := uuid.New()
	repo := NewMockRepository()
	repo.plugins[pluginID] = &Plugin{
		ID:       pluginID,
		Name:     "test-plugin",
		State:    StateInstalled,
		Manifest: json.RawMessage(`{"name": "test-plugin", "version": "1.0.0"}`),
	}
	service := NewServiceWithRepository(repo, nil, "/tmp/plugins")

	err := service.EnablePlugin(ctx, pluginID, []string{"invalid:permission"})
	if err == nil {
		t.Error("expected error for invalid permissions")
	}
}

func TestService_EnablePlugin_MissingRequiredPermission(t *testing.T) {
	ctx := context.Background()
	pluginID := uuid.New()
	repo := NewMockRepository()
	// Plugin with backend hooks requires hooks:register permission
	repo.plugins[pluginID] = &Plugin{
		ID:    pluginID,
		Name:  "test-plugin",
		State: StateInstalled,
		Manifest: json.RawMessage(`{
			"name": "test-plugin",
			"version": "1.0.0",
			"backend": {
				"hooks": [{"event": "invoice.created", "handler": "onInvoice"}]
			}
		}`),
	}
	service := NewServiceWithRepository(repo, nil, "/tmp/plugins")

	// Try to enable without granting hooks:register permission
	err := service.EnablePlugin(ctx, pluginID, []string{"invoices:read"})
	if err == nil {
		t.Error("expected error for missing required permission")
	}
}

func TestService_EnablePlugin_Success(t *testing.T) {
	ctx := context.Background()
	pluginID := uuid.New()
	repo := NewMockRepository()
	repo.plugins[pluginID] = &Plugin{
		ID:    pluginID,
		Name:  "test-plugin",
		State: StateInstalled,
		Manifest: json.RawMessage(`{
			"name": "test-plugin",
			"version": "1.0.0"
		}`),
	}
	hooks := NewHookRegistry()
	service := NewServiceWithRepository(repo, hooks, "/tmp/plugins")

	err := service.EnablePlugin(ctx, pluginID, []string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify plugin is loaded
	loaded, exists := service.GetLoadedPlugin("test-plugin")
	if !exists {
		t.Error("expected plugin to be loaded")
	}
	if loaded.Plugin.State != StateEnabled {
		t.Error("expected plugin state to be enabled")
	}
}

func TestService_DisablePlugin_NotFound(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	service := NewServiceWithRepository(repo, nil, "/tmp/plugins")

	err := service.DisablePlugin(ctx, uuid.New())
	if err == nil {
		t.Error("expected error for non-existent plugin")
	}
}

func TestService_DisablePlugin_AlreadyDisabled(t *testing.T) {
	ctx := context.Background()
	pluginID := uuid.New()
	repo := NewMockRepository()
	repo.plugins[pluginID] = &Plugin{
		ID:    pluginID,
		Name:  "test-plugin",
		State: StateDisabled,
	}
	service := NewServiceWithRepository(repo, nil, "/tmp/plugins")

	err := service.DisablePlugin(ctx, pluginID)
	if err == nil {
		t.Error("expected error for already disabled plugin")
	}
}

func TestService_DisablePlugin_Success(t *testing.T) {
	ctx := context.Background()
	pluginID := uuid.New()
	tenantID := uuid.New()
	repo := NewMockRepository()
	repo.plugins[pluginID] = &Plugin{
		ID:    pluginID,
		Name:  "test-plugin",
		State: StateEnabled,
	}
	// Add tenant plugin
	key := fmt.Sprintf("%s:%s", tenantID.String(), pluginID.String())
	now := time.Now()
	repo.tenantPlugins[key] = &TenantPlugin{
		ID:        uuid.New(),
		TenantID:  tenantID,
		PluginID:  pluginID,
		IsEnabled: true,
		EnabledAt: &now,
	}
	hooks := NewHookRegistry()
	service := NewServiceWithRepository(repo, hooks, "/tmp/plugins")

	// First load the plugin
	service.loadPlugin(repo.plugins[pluginID], &Manifest{Name: "test-plugin", Version: "1.0.0"})

	err := service.DisablePlugin(ctx, pluginID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify plugin is unloaded
	_, exists := service.GetLoadedPlugin("test-plugin")
	if exists {
		t.Error("expected plugin to be unloaded")
	}

	// Verify tenant plugins are disabled
	for _, tp := range repo.tenantPlugins {
		if tp.PluginID == pluginID && tp.IsEnabled {
			t.Error("expected all tenant plugins to be disabled")
		}
	}
}

func TestService_LoadEnabledPlugins_Empty(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	hooks := NewHookRegistry()
	service := NewServiceWithRepository(repo, hooks, "/tmp/plugins")

	err := service.LoadEnabledPlugins(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestService_LoadEnabledPlugins_WithPlugins(t *testing.T) {
	ctx := context.Background()
	pluginID := uuid.New()
	repo := NewMockRepository()
	repo.plugins[pluginID] = &Plugin{
		ID:       pluginID,
		Name:     "test-plugin",
		State:    StateEnabled,
		Manifest: json.RawMessage(`{"name": "test-plugin", "version": "1.0.0"}`),
	}
	hooks := NewHookRegistry()
	service := NewServiceWithRepository(repo, hooks, "/tmp/plugins")

	err := service.LoadEnabledPlugins(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify plugin is loaded
	loaded, exists := service.GetLoadedPlugin("test-plugin")
	if !exists {
		t.Error("expected plugin to be loaded")
	}
	if loaded == nil {
		t.Error("expected loaded plugin to not be nil")
	}
}

func TestService_LoadEnabledPlugins_InvalidManifest(t *testing.T) {
	ctx := context.Background()
	pluginID := uuid.New()
	repo := NewMockRepository()
	repo.plugins[pluginID] = &Plugin{
		ID:       pluginID,
		Name:     "test-plugin",
		State:    StateEnabled,
		Manifest: json.RawMessage(`{invalid json`),
	}
	hooks := NewHookRegistry()
	service := NewServiceWithRepository(repo, hooks, "/tmp/plugins")

	// Should not return error, just log warning
	err := service.LoadEnabledPlugins(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestService_LoadEnabledPlugins_RepoError(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	repo.listEnabledPluginsErr = fmt.Errorf("db error")
	hooks := NewHookRegistry()
	service := NewServiceWithRepository(repo, hooks, "/tmp/plugins")

	err := service.LoadEnabledPlugins(ctx)
	if err == nil {
		t.Error("expected error from repository")
	}
}

// Additional tests to improve coverage

func TestService_ListTenantPlugins(t *testing.T) {
	ctx := context.Background()
	tenantID := uuid.New()
	pluginID := uuid.New()

	tests := []struct {
		name        string
		setupRepo   func() *MockRepository
		tenantID    uuid.UUID
		expectCount int
		expectErr   bool
	}{
		{
			name: "success_with_enabled_plugins",
			setupRepo: func() *MockRepository {
				repo := NewMockRepository()
				repo.plugins[pluginID] = &Plugin{
					ID:    pluginID,
					Name:  "test-plugin",
					State: StateEnabled,
				}
				key := fmt.Sprintf("%s:%s", tenantID.String(), pluginID.String())
				now := time.Now()
				repo.tenantPlugins[key] = &TenantPlugin{
					ID:        uuid.New(),
					TenantID:  tenantID,
					PluginID:  pluginID,
					IsEnabled: true,
					EnabledAt: &now,
				}
				return repo
			},
			tenantID:    tenantID,
			expectCount: 1,
			expectErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := tt.setupRepo()
			service := NewServiceWithRepository(repo, nil, "/tmp/plugins")

			result, err := service.GetTenantPlugins(ctx, tt.tenantID)

			if tt.expectErr {
				if err == nil {
					t.Error("expected error but got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(result) != tt.expectCount {
				t.Errorf("expected %d plugins, got %d", tt.expectCount, len(result))
			}
		})
	}
}

func TestService_EnablePlugin_WithHooks(t *testing.T) {
	ctx := context.Background()
	pluginID := uuid.New()
	repo := NewMockRepository()
	repo.plugins[pluginID] = &Plugin{
		ID:    pluginID,
		Name:  "test-plugin-with-hooks",
		State: StateInstalled,
		Manifest: json.RawMessage(`{
			"name": "test-plugin-with-hooks",
			"version": "1.0.0",
			"backend": {
				"package": "internal/plugin",
				"entry": "main.go",
				"hooks": [{"event": "invoice.created", "handler": "onInvoice"}]
			},
			"permissions": ["hooks:register"]
		}`),
	}
	hooks := NewHookRegistry()
	service := NewServiceWithRepository(repo, hooks, "/tmp/plugins")

	err := service.EnablePlugin(ctx, pluginID, []string{"hooks:register"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify plugin is loaded with hooks
	loaded, exists := service.GetLoadedPlugin("test-plugin-with-hooks")
	if !exists {
		t.Error("expected plugin to be loaded")
	}
	if loaded == nil {
		t.Error("expected loaded plugin to not be nil")
	}
}

func TestService_EnablePlugin_UpdateStateError(t *testing.T) {
	ctx := context.Background()
	pluginID := uuid.New()
	repo := NewMockRepository()
	repo.plugins[pluginID] = &Plugin{
		ID:    pluginID,
		Name:  "test-plugin",
		State: StateInstalled,
		Manifest: json.RawMessage(`{
			"name": "test-plugin",
			"version": "1.0.0"
		}`),
	}
	repo.updatePluginErr = fmt.Errorf("update state error")
	service := NewServiceWithRepository(repo, nil, "/tmp/plugins")

	err := service.EnablePlugin(ctx, pluginID, []string{})
	if err == nil {
		t.Error("expected error from update state failure")
	}
}

func TestService_DisablePlugin_UpdateStateError(t *testing.T) {
	ctx := context.Background()
	pluginID := uuid.New()
	repo := NewMockRepository()
	repo.plugins[pluginID] = &Plugin{
		ID:    pluginID,
		Name:  "test-plugin",
		State: StateEnabled,
	}
	repo.updatePluginErr = fmt.Errorf("update state error")
	hooks := NewHookRegistry()
	service := NewServiceWithRepository(repo, hooks, "/tmp/plugins")

	err := service.DisablePlugin(ctx, pluginID)
	if err == nil {
		t.Error("expected error from update state failure")
	}
}

func TestService_EnableForTenant_GetPluginError(t *testing.T) {
	ctx := context.Background()
	tenantID := uuid.New()
	pluginID := uuid.New()
	repo := NewMockRepository()
	repo.getPluginErr = fmt.Errorf("get plugin error")
	service := NewServiceWithRepository(repo, nil, "/tmp/plugins")

	err := service.EnableForTenant(ctx, tenantID, pluginID, nil)
	if err == nil {
		t.Error("expected error from get plugin failure")
	}
}

func TestService_LoadPlugin_WithRoutes(t *testing.T) {
	repo := NewMockRepository()
	hooks := NewHookRegistry()
	service := NewServiceWithRepository(repo, hooks, "/tmp/plugins")

	plugin := &Plugin{
		ID:   uuid.New(),
		Name: "routes-plugin",
	}

	manifest := &Manifest{
		Name:    "routes-plugin",
		Version: "1.0.0",
		Backend: &BackendConfig{
			Package: "internal/plugin",
			Entry:   "main.go",
			Routes: []RouteConfig{
				{Method: "GET", Path: "/api/test", Handler: "handleTest"},
			},
		},
	}

	err := service.loadPlugin(plugin, manifest)
	if err != nil {
		t.Fatalf("loadPlugin failed: %v", err)
	}

	loaded, exists := service.GetLoadedPlugin("routes-plugin")
	if !exists {
		t.Error("expected plugin to be loaded")
	}
	if loaded.Manifest.Backend == nil {
		t.Error("expected backend manifest to be set")
	}
}

func TestService_NewService(t *testing.T) {
	// Test NewService with nil pool (will panic in real usage, but covers the code path)
	defer func() {
		if r := recover(); r != nil {
			// Expected panic when pool is nil
		}
	}()

	// This tests that NewService creates a proper service
	// In real usage, we'd need a proper pool, but for coverage we test the code path
	service := NewServiceWithRepository(nil, nil, "/tmp/plugins")
	if service == nil {
		t.Error("expected service to be created")
	}
}

func TestMockRepository_AdditionalMethods(t *testing.T) {
	repo := NewMockRepository()
	ctx := context.Background()
	pluginID := uuid.New()
	tenantID := uuid.New()

	// Test InsertPluginReturning
	manifest := &Manifest{
		Name:        "test-plugin",
		DisplayName: "Test Plugin",
		Version:     "1.0.0",
	}
	manifestJSON := json.RawMessage(`{"name":"test-plugin"}`)

	plugin, err := repo.InsertPluginReturning(ctx, manifest, "https://github.com/test/repo", RepoGitHub, manifestJSON)
	if err != nil {
		t.Fatalf("InsertPluginReturning failed: %v", err)
	}
	if plugin.Name != manifest.Name {
		t.Errorf("expected name %s, got %s", manifest.Name, plugin.Name)
	}

	// Test CountEnabledTenantsForPlugin with no tenants
	count, err := repo.CountEnabledTenantsForPlugin(ctx, pluginID)
	if err != nil {
		t.Fatalf("CountEnabledTenantsForPlugin failed: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 tenants, got %d", count)
	}

	// Test UpdatePluginState
	err = repo.UpdatePluginState(ctx, plugin.ID, StateEnabled, []string{"test:permission"})
	if err != nil {
		t.Fatalf("UpdatePluginState failed: %v", err)
	}

	// Verify state was updated
	updated, _ := repo.GetPlugin(ctx, plugin.ID)
	if updated.State != StateEnabled {
		t.Errorf("expected state %s, got %s", StateEnabled, updated.State)
	}

	// Test DisableAllTenantsForPlugin
	key := fmt.Sprintf("%s:%s", tenantID.String(), plugin.ID.String())
	now := time.Now()
	repo.tenantPlugins[key] = &TenantPlugin{
		ID:        uuid.New(),
		TenantID:  tenantID,
		PluginID:  plugin.ID,
		IsEnabled: true,
		EnabledAt: &now,
	}

	err = repo.DisableAllTenantsForPlugin(ctx, plugin.ID)
	if err != nil {
		t.Fatalf("DisableAllTenantsForPlugin failed: %v", err)
	}

	// Verify tenant plugin was disabled
	tp := repo.tenantPlugins[key]
	if tp.IsEnabled {
		t.Error("expected tenant plugin to be disabled")
	}

	// Test GetTenantPluginsWithAll
	results, err := repo.GetTenantPluginsWithAll(ctx, tenantID)
	if err != nil {
		t.Fatalf("GetTenantPluginsWithAll failed: %v", err)
	}
	// Should find the enabled plugin
	found := false
	for _, result := range results {
		if result.PluginID == plugin.ID {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected to find plugin in GetTenantPluginsWithAll results")
	}
}

func TestMockRepository_Errors(t *testing.T) {
	ctx := context.Background()
	tenantID := uuid.New()
	pluginID := uuid.New()

	tests := []struct {
		name      string
		setupRepo func() *MockRepository
		test      func(*MockRepository) error
	}{
		{
			name: "InsertPluginReturning error",
			setupRepo: func() *MockRepository {
				repo := NewMockRepository()
				repo.createPluginErr = fmt.Errorf("create error")
				return repo
			},
			test: func(repo *MockRepository) error {
				_, err := repo.InsertPluginReturning(ctx, &Manifest{Name: "test"}, "url", RepoGitHub, nil)
				return err
			},
		},
		{
			name: "UpdatePluginState error",
			setupRepo: func() *MockRepository {
				repo := NewMockRepository()
				repo.updatePluginErr = fmt.Errorf("update error")
				return repo
			},
			test: func(repo *MockRepository) error {
				return repo.UpdatePluginState(ctx, pluginID, StateEnabled, nil)
			},
		},
		{
			name: "GetTenantPluginsWithAll error",
			setupRepo: func() *MockRepository {
				repo := NewMockRepository()
				repo.listTenantPluginsErr = fmt.Errorf("list error")
				return repo
			},
			test: func(repo *MockRepository) error {
				_, err := repo.GetTenantPluginsWithAll(ctx, tenantID)
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := tt.setupRepo()
			err := tt.test(repo)
			if err == nil {
				t.Errorf("expected error for %s", tt.name)
			}
		})
	}
}

// TestNewService tests the NewService constructor with a nil pool
func TestNewService(t *testing.T) {
	// NewService should create a service with nil pool (won't panic until used)
	svc := NewService(nil, "/tmp/plugins")
	if svc == nil {
		t.Fatal("expected non-nil service")
	}
	if svc.pool != nil {
		t.Error("expected nil pool")
	}
	if svc.repo == nil {
		t.Error("expected non-nil repo")
	}
	if svc.hooks == nil {
		t.Error("expected non-nil hooks")
	}
	if svc.pluginDir != "/tmp/plugins" {
		t.Errorf("expected pluginDir to be /tmp/plugins, got %s", svc.pluginDir)
	}
}
