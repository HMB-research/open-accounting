package accounting

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockRepository is a mock implementation of RepositoryInterface for testing
type MockRepository struct {
	accounts       map[string]*Account
	journalEntries map[string]*JournalEntry
	templates      map[string]*JournalEntryTemplate
	accountsByType map[AccountType][]Account
	balances       []AccountBalance
	periodBalances []AccountBalance

	listJournalSchemaName string
	listJournalTenantID   string
	listJournalLimit      int

	// Error injection
	getAccountErr       error
	listAccountsErr     error
	createAccountErr    error
	updateAccountErr    error
	getJournalErr       error
	createJournalErr    error
	updateStatusErr     error
	getBalanceErr       error
	trialBalanceErr     error
	periodBalanceErr    error
	voidJournalEntryErr error
	listTemplatesErr    error
	getTemplateErr      error
	dueTemplatesErr     error
	updateTemplateErr   error
	dueTemplateIDs      []string
}

type repositoryWithoutJournalTemplates struct {
	RepositoryInterface
}

func NewMockRepository() *MockRepository {
	return &MockRepository{
		accounts:       make(map[string]*Account),
		journalEntries: make(map[string]*JournalEntry),
		templates:      make(map[string]*JournalEntryTemplate),
		accountsByType: make(map[AccountType][]Account),
	}
}

func (m *MockRepository) GetAccountByID(ctx context.Context, schemaName, tenantID, accountID string) (*Account, error) {
	if m.getAccountErr != nil {
		return nil, m.getAccountErr
	}
	a, ok := m.accounts[accountID]
	if !ok || a.TenantID != tenantID {
		return nil, errors.New("account not found")
	}
	return a, nil
}

func (m *MockRepository) ListAccounts(ctx context.Context, schemaName, tenantID string, activeOnly bool) ([]Account, error) {
	if m.listAccountsErr != nil {
		return nil, m.listAccountsErr
	}
	var result []Account
	for _, a := range m.accounts {
		if a.TenantID != tenantID {
			continue
		}
		if activeOnly && !a.IsActive {
			continue
		}
		result = append(result, *a)
	}
	return result, nil
}

func (m *MockRepository) CreateAccount(ctx context.Context, schemaName string, a *Account) error {
	if m.createAccountErr != nil {
		return m.createAccountErr
	}
	m.accounts[a.ID] = a
	return nil
}

func (m *MockRepository) UpdateAccount(ctx context.Context, schemaName string, a *Account) error {
	if m.updateAccountErr != nil {
		return m.updateAccountErr
	}
	if _, ok := m.accounts[a.ID]; !ok {
		return errors.New("account not found")
	}
	m.accounts[a.ID] = a
	return nil
}

func (m *MockRepository) ListJournalEntries(ctx context.Context, schemaName, tenantID string, limit int) ([]JournalEntry, error) {
	m.listJournalSchemaName = schemaName
	m.listJournalTenantID = tenantID
	m.listJournalLimit = limit

	if m.getJournalErr != nil {
		return nil, m.getJournalErr
	}

	result := make([]JournalEntry, 0, len(m.journalEntries))
	for _, entry := range m.journalEntries {
		if entry.TenantID != tenantID {
			continue
		}
		result = append(result, *entry)
	}
	return result, nil
}

func (m *MockRepository) GetJournalEntryByID(ctx context.Context, schemaName, tenantID, entryID string) (*JournalEntry, error) {
	if m.getJournalErr != nil {
		return nil, m.getJournalErr
	}
	je, ok := m.journalEntries[entryID]
	if !ok || je.TenantID != tenantID {
		return nil, errors.New("journal entry not found")
	}
	return je, nil
}

func (m *MockRepository) GetJournalEntryBySource(ctx context.Context, schemaName, tenantID, sourceType, sourceID string) (*JournalEntry, error) {
	if m.getJournalErr != nil {
		return nil, m.getJournalErr
	}
	for _, entry := range m.journalEntries {
		if entry.TenantID != tenantID || entry.SourceType != sourceType || entry.Status == StatusVoided || entry.SourceID == nil || *entry.SourceID != sourceID {
			continue
		}
		return entry, nil
	}
	return nil, nil
}

func (m *MockRepository) CreateJournalEntry(ctx context.Context, schemaName string, je *JournalEntry) error {
	if m.createJournalErr != nil {
		return m.createJournalErr
	}
	je.EntryNumber = "JE-00001"
	m.journalEntries[je.ID] = je
	return nil
}

func (m *MockRepository) CreateJournalEntryTemplate(ctx context.Context, schemaName string, template *JournalEntryTemplate) error {
	if m.createJournalErr != nil {
		return m.createJournalErr
	}
	m.templates[template.ID] = template
	return nil
}

func (m *MockRepository) ListJournalEntryTemplates(ctx context.Context, schemaName, tenantID string, activeOnly bool) ([]JournalEntryTemplate, error) {
	if m.listTemplatesErr != nil {
		return nil, m.listTemplatesErr
	}
	result := make([]JournalEntryTemplate, 0, len(m.templates))
	for _, template := range m.templates {
		if template.TenantID != tenantID {
			continue
		}
		if activeOnly && !template.IsActive {
			continue
		}
		result = append(result, *template)
	}
	return result, nil
}

func (m *MockRepository) GetJournalEntryTemplateByID(ctx context.Context, schemaName, tenantID, templateID string) (*JournalEntryTemplate, error) {
	if m.getTemplateErr != nil {
		return nil, m.getTemplateErr
	}
	template, ok := m.templates[templateID]
	if !ok || template.TenantID != tenantID {
		return nil, errors.New("journal entry template not found")
	}
	return template, nil
}

func (m *MockRepository) GetDueJournalEntryTemplateIDs(ctx context.Context, schemaName, tenantID string, asOfDate time.Time) ([]string, error) {
	if m.dueTemplatesErr != nil {
		return nil, m.dueTemplatesErr
	}
	if m.dueTemplateIDs != nil {
		return append([]string(nil), m.dueTemplateIDs...), nil
	}
	var ids []string
	for _, template := range m.templates {
		if template.TenantID != tenantID || !template.IsActive || !template.IsRecurring() || template.NextGenerationDate == nil {
			continue
		}
		if template.NextGenerationDate.After(asOfDate) {
			continue
		}
		if template.EndDate != nil && template.NextGenerationDate.After(*template.EndDate) {
			continue
		}
		ids = append(ids, template.ID)
	}
	return ids, nil
}

func (m *MockRepository) UpdateJournalEntryTemplateAfterGeneration(ctx context.Context, schemaName, tenantID, templateID string, nextDate time.Time, generatedAt time.Time) error {
	if m.updateTemplateErr != nil {
		return m.updateTemplateErr
	}
	template, ok := m.templates[templateID]
	if !ok || template.TenantID != tenantID {
		return errors.New("journal entry template not found")
	}
	template.NextGenerationDate = &nextDate
	template.LastGeneratedAt = &generatedAt
	template.GeneratedCount++
	template.UpdatedAt = generatedAt
	return nil
}

func (m *MockRepository) UpdateJournalEntryStatus(ctx context.Context, schemaName, tenantID, entryID string, status JournalEntryStatus, userID string) error {
	if m.updateStatusErr != nil {
		return m.updateStatusErr
	}
	je, ok := m.journalEntries[entryID]
	if !ok || je.TenantID != tenantID {
		return errors.New("entry not found or invalid status transition")
	}
	je.Status = status
	return nil
}

func (m *MockRepository) GetAccountBalance(ctx context.Context, schemaName, tenantID, accountID string, asOfDate time.Time) (decimal.Decimal, error) {
	if m.getBalanceErr != nil {
		return decimal.Zero, m.getBalanceErr
	}
	return decimal.NewFromFloat(1000), nil
}

func (m *MockRepository) GetTrialBalance(ctx context.Context, schemaName, tenantID string, asOfDate time.Time) ([]AccountBalance, error) {
	if m.trialBalanceErr != nil {
		return nil, m.trialBalanceErr
	}
	return m.balances, nil
}

func (m *MockRepository) GetPeriodBalances(ctx context.Context, schemaName, tenantID string, startDate, endDate time.Time) ([]AccountBalance, error) {
	if m.periodBalanceErr != nil {
		return nil, m.periodBalanceErr
	}
	return m.periodBalances, nil
}

func (m *MockRepository) VoidJournalEntry(ctx context.Context, schemaName, tenantID, entryID, userID, reason string, reversal *JournalEntry) error {
	if m.voidJournalEntryErr != nil {
		return m.voidJournalEntryErr
	}
	// Mark original entry as voided in mock
	if je, ok := m.journalEntries[entryID]; ok && je.TenantID == tenantID {
		je.Status = StatusVoided
	}
	// Store the reversal entry
	if reversal != nil {
		reversal.EntryNumber = "JE-00002"
		m.journalEntries[reversal.ID] = reversal
	}
	return nil
}

func TestNewServiceWithRepository(t *testing.T) {
	// NewServiceWithRepository allows injecting a mock repository for testing
	svc := NewServiceWithRepository(NewMockRepository())
	assert.NotNil(t, svc)
}

func TestService_GetAccount(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	svc := NewServiceWithRepository(repo)
	schemaName := "tenant_test"

	account := &Account{
		ID:          "acc-1",
		TenantID:    "tenant-1",
		Code:        "1000",
		Name:        "Cash",
		AccountType: AccountTypeAsset,
		IsActive:    true,
	}
	repo.accounts[account.ID] = account

	t.Run("returns account when found", func(t *testing.T) {
		result, err := svc.GetAccount(ctx, schemaName, "tenant-1", "acc-1")
		require.NoError(t, err)
		assert.Equal(t, "acc-1", result.ID)
		assert.Equal(t, "Cash", result.Name)
	})

	t.Run("returns error when not found", func(t *testing.T) {
		_, err := svc.GetAccount(ctx, schemaName, "tenant-1", "nonexistent")
		assert.Error(t, err)
	})

	t.Run("returns error when wrong tenant", func(t *testing.T) {
		_, err := svc.GetAccount(ctx, schemaName, "tenant-2", "acc-1")
		assert.Error(t, err)
	})

	t.Run("propagates repository error", func(t *testing.T) {
		repo.getAccountErr = errors.New("database error")
		_, err := svc.GetAccount(ctx, schemaName, "tenant-1", "acc-1")
		assert.Error(t, err)
		repo.getAccountErr = nil
	})
}

func TestService_ListAccounts(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	svc := NewServiceWithRepository(repo)
	schemaName := "tenant_test"

	repo.accounts["acc-1"] = &Account{ID: "acc-1", TenantID: "tenant-1", IsActive: true}
	repo.accounts["acc-2"] = &Account{ID: "acc-2", TenantID: "tenant-1", IsActive: false}
	repo.accounts["acc-3"] = &Account{ID: "acc-3", TenantID: "tenant-2", IsActive: true}

	t.Run("lists all accounts for tenant", func(t *testing.T) {
		result, err := svc.ListAccounts(ctx, schemaName, "tenant-1", false)
		require.NoError(t, err)
		assert.Len(t, result, 2)
	})

	t.Run("lists only active accounts", func(t *testing.T) {
		result, err := svc.ListAccounts(ctx, schemaName, "tenant-1", true)
		require.NoError(t, err)
		assert.Len(t, result, 1)
	})

	t.Run("propagates repository error", func(t *testing.T) {
		repo.listAccountsErr = errors.New("database error")
		_, err := svc.ListAccounts(ctx, schemaName, "tenant-1", false)
		assert.Error(t, err)
		repo.listAccountsErr = nil
	})
}

func TestService_GetAccountHierarchy(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	svc := NewServiceWithRepository(repo)
	schemaName := "tenant_test"

	parentID := "acc-1000"
	repo.accounts["acc-4000"] = &Account{ID: "acc-4000", TenantID: "tenant-1", Code: "4000", Name: "Revenue", AccountType: AccountTypeRevenue, IsActive: true}
	repo.accounts["acc-1100"] = &Account{ID: "acc-1100", TenantID: "tenant-1", Code: "1100", Name: "Bank", AccountType: AccountTypeAsset, ParentID: &parentID, IsActive: true}
	repo.accounts["acc-1000"] = &Account{ID: "acc-1000", TenantID: "tenant-1", Code: "1000", Name: "Assets", AccountType: AccountTypeAsset, IsActive: true}
	repo.accounts["acc-1200"] = &Account{ID: "acc-1200", TenantID: "tenant-1", Code: "1200", Name: "Receivables", AccountType: AccountTypeAsset, ParentID: &parentID, IsActive: false}
	repo.accounts["acc-other"] = &Account{ID: "acc-other", TenantID: "tenant-2", Code: "9999", Name: "Other", AccountType: AccountTypeExpense, IsActive: true}

	t.Run("flattens parent child accounts in code order", func(t *testing.T) {
		result, err := svc.GetAccountHierarchy(ctx, schemaName, "tenant-1", false)
		require.NoError(t, err)
		require.Len(t, result, 4)

		assert.Equal(t, []string{"1000", "1100", "1200", "4000"}, []string{result[0].Code, result[1].Code, result[2].Code, result[3].Code})
		assert.Equal(t, 0, result[0].Depth)
		assert.True(t, result[0].HasChildren)
		assert.Equal(t, 1, result[1].Depth)
		assert.Equal(t, "1000", result[1].ParentCode)
		assert.Equal(t, "Assets", result[1].ParentName)
		assert.Equal(t, "1000/1100", result[1].Path)
		assert.Equal(t, "1000/1200", result[2].Path)
	})

	t.Run("applies active only filter before building hierarchy", func(t *testing.T) {
		result, err := svc.GetAccountHierarchy(ctx, schemaName, "tenant-1", true)
		require.NoError(t, err)
		require.Len(t, result, 3)
		assert.Equal(t, []string{"1000", "1100", "4000"}, []string{result[0].Code, result[1].Code, result[2].Code})
	})

	t.Run("propagates repository error", func(t *testing.T) {
		repo.listAccountsErr = errors.New("database error")
		_, err := svc.GetAccountHierarchy(ctx, schemaName, "tenant-1", false)
		assert.Error(t, err)
		repo.listAccountsErr = nil
	})
}

func TestBuildAccountHierarchy(t *testing.T) {
	t.Run("sorts duplicate codes by name then id and treats missing parent as root", func(t *testing.T) {
		missingParentID := "missing-parent"

		result := BuildAccountHierarchy([]Account{
			{ID: "acc-b", Code: "1000", Name: "Cash B", AccountType: AccountTypeAsset, IsActive: true},
			{ID: "acc-a2", Code: "1000", Name: "Cash A", AccountType: AccountTypeAsset, IsActive: true},
			{ID: "orphan", Code: "0500", Name: "Orphan", AccountType: AccountTypeAsset, ParentID: &missingParentID, IsActive: true},
			{ID: "acc-a1", Code: "1000", Name: "Cash A", AccountType: AccountTypeAsset, IsActive: true},
		})

		require.Len(t, result, 4)
		assert.Equal(t, []string{"orphan", "acc-a1", "acc-a2", "acc-b"}, []string{result[0].ID, result[1].ID, result[2].ID, result[3].ID})
		assert.Equal(t, 0, result[0].Depth)
		assert.Empty(t, result[0].ParentCode)
		assert.Empty(t, result[0].ParentName)
		assert.Equal(t, "0500", result[0].Path)
		assert.False(t, result[0].HasChildren)
	})

	t.Run("does not recurse forever when parent references form a cycle", func(t *testing.T) {
		parentA := "acc-a"
		parentB := "acc-b"

		result := BuildAccountHierarchy([]Account{
			{ID: "acc-a", Code: "1000", Name: "Assets", AccountType: AccountTypeAsset, ParentID: &parentB, IsActive: true},
			{ID: "acc-b", Code: "1100", Name: "Bank", AccountType: AccountTypeAsset, ParentID: &parentA, IsActive: true},
		})

		require.Len(t, result, 2)
		assert.Equal(t, []string{"acc-a", "acc-b"}, []string{result[0].ID, result[1].ID})
		assert.Equal(t, 0, result[0].Depth)
		assert.Empty(t, result[0].ParentCode)
		assert.Equal(t, "1000", result[0].Path)
		assert.True(t, result[0].HasChildren)
		assert.Equal(t, 1, result[1].Depth)
		assert.Equal(t, "1000", result[1].ParentCode)
		assert.Equal(t, "Assets", result[1].ParentName)
		assert.Equal(t, "1000/1100", result[1].Path)
		assert.True(t, result[1].HasChildren)
	})
}

func TestService_CreateAccount(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	svc := NewServiceWithRepository(repo)
	schemaName := "tenant_test"

	t.Run("creates account with generated ID", func(t *testing.T) {
		req := &CreateAccountRequest{
			Code:        "1000",
			Name:        "Cash",
			AccountType: AccountTypeAsset,
			Description: "Cash account",
		}

		result, err := svc.CreateAccount(ctx, schemaName, "tenant-1", req)
		require.NoError(t, err)
		assert.NotEmpty(t, result.ID)
		assert.Equal(t, "tenant-1", result.TenantID)
		assert.Equal(t, "1000", result.Code)
		assert.Equal(t, "Cash", result.Name)
		assert.Equal(t, AccountTypeAsset, result.AccountType)
		assert.True(t, result.IsActive)
		assert.False(t, result.IsSystem)
		assert.False(t, result.CreatedAt.IsZero())
	})

	t.Run("creates account with explicit normalized ID and blank parent", func(t *testing.T) {
		id := "  11111111-1111-4111-8111-111111111111  "
		blankParentID := "  "
		req := &CreateAccountRequest{
			ID:          id,
			Code:        "  1020  ",
			Name:        "  Clearing  ",
			AccountType: AccountTypeAsset,
			ParentID:    &blankParentID,
			Description: "  Suspense clearing account  ",
		}

		result, err := svc.CreateAccount(ctx, schemaName, "tenant-1", req)
		require.NoError(t, err)
		assert.Equal(t, "11111111-1111-4111-8111-111111111111", result.ID)
		assert.Equal(t, "1020", result.Code)
		assert.Equal(t, "Clearing", result.Name)
		assert.Equal(t, "Suspense clearing account", result.Description)
		assert.Nil(t, result.ParentID)
	})

	t.Run("creates account with parent", func(t *testing.T) {
		parentID := "11111111-1111-1111-1111-111111111111"
		req := &CreateAccountRequest{
			Code:        "1010",
			Name:        "Petty Cash",
			AccountType: AccountTypeAsset,
			ParentID:    &parentID,
		}

		result, err := svc.CreateAccount(ctx, schemaName, "tenant-1", req)
		require.NoError(t, err)
		assert.Equal(t, &parentID, result.ParentID)
	})

	t.Run("rejects missing required fields", func(t *testing.T) {
		_, err := svc.CreateAccount(ctx, schemaName, "tenant-1", &CreateAccountRequest{
			Code:        "   ",
			Name:        "Missing type",
			AccountType: "",
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "code, name, and account_type are required")
	})

	t.Run("rejects invalid account type", func(t *testing.T) {
		_, err := svc.CreateAccount(ctx, schemaName, "tenant-1", &CreateAccountRequest{
			Code:        "1999",
			Name:        "Unsupported",
			AccountType: AccountType("unsupported"),
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid account_type")
	})

	t.Run("rejects invalid explicit id", func(t *testing.T) {
		_, err := svc.CreateAccount(ctx, schemaName, "tenant-1", &CreateAccountRequest{
			ID:          "legacy-id",
			Code:        "1998",
			Name:        "Legacy",
			AccountType: AccountTypeAsset,
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "id must be a valid UUID")
	})

	t.Run("rejects invalid parent id", func(t *testing.T) {
		parentID := "legacy-parent"
		req := &CreateAccountRequest{
			Code:        "1010",
			Name:        "Petty Cash",
			AccountType: AccountTypeAsset,
			ParentID:    &parentID,
		}

		_, err := svc.CreateAccount(ctx, schemaName, "tenant-1", req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "parent_id must be a valid UUID")
	})

	t.Run("propagates repository error", func(t *testing.T) {
		repo.createAccountErr = errors.New("database error")
		req := &CreateAccountRequest{
			Code:        "1000",
			Name:        "Cash",
			AccountType: AccountTypeAsset,
		}
		_, err := svc.CreateAccount(ctx, schemaName, "tenant-1", req)
		assert.Error(t, err)
		repo.createAccountErr = nil
	})
}

func TestService_UpdateAndDeactivateAccount(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	svc := NewServiceWithRepository(repo)
	schemaName := "tenant_test"

	repo.accounts["custom"] = &Account{
		ID:          "custom",
		TenantID:    "tenant-1",
		Code:        "6100",
		Name:        "Old Expense",
		AccountType: AccountTypeExpense,
		IsActive:    true,
		IsSystem:    false,
		Description: "Old description",
		CreatedAt:   time.Now(),
	}
	repo.accounts["system"] = &Account{
		ID:          "system",
		TenantID:    "tenant-1",
		Code:        "1000",
		Name:        "Cash",
		AccountType: AccountTypeAsset,
		IsActive:    true,
		IsSystem:    true,
		CreatedAt:   time.Now(),
	}

	t.Run("updates editable account", func(t *testing.T) {
		result, err := svc.UpdateAccount(ctx, schemaName, "tenant-1", "custom", &UpdateAccountRequest{
			Code:        "6150",
			Name:        "Updated Expense",
			AccountType: AccountTypeExpense,
			Description: "Updated description",
		})
		require.NoError(t, err)
		assert.Equal(t, "6150", result.Code)
		assert.Equal(t, "Updated Expense", result.Name)
		assert.Equal(t, "Updated description", repo.accounts["custom"].Description)
		assert.True(t, repo.accounts["custom"].IsActive)
	})

	t.Run("returns get errors on update", func(t *testing.T) {
		_, err := svc.UpdateAccount(ctx, schemaName, "tenant-1", "missing", &UpdateAccountRequest{
			Code:        "6150",
			Name:        "Missing",
			AccountType: AccountTypeExpense,
		})
		assert.Error(t, err)
	})

	t.Run("rejects system account update", func(t *testing.T) {
		_, err := svc.UpdateAccount(ctx, schemaName, "tenant-1", "system", &UpdateAccountRequest{
			Code:        "1010",
			Name:        "Changed",
			AccountType: AccountTypeAsset,
		})
		assert.ErrorIs(t, err, ErrSystemAccountImmutable)
		assert.Equal(t, "1000", repo.accounts["system"].Code)
	})

	t.Run("rejects invalid update request", func(t *testing.T) {
		_, err := svc.UpdateAccount(ctx, schemaName, "tenant-1", "custom", &UpdateAccountRequest{
			Code:        "",
			Name:        "Invalid",
			AccountType: AccountTypeExpense,
		})
		assert.Error(t, err)
	})

	t.Run("rejects invalid update account type", func(t *testing.T) {
		_, err := svc.UpdateAccount(ctx, schemaName, "tenant-1", "custom", &UpdateAccountRequest{
			Code:        "6150",
			Name:        "Updated Expense",
			AccountType: AccountType("unsupported"),
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid account_type")
	})

	t.Run("rejects invalid parent id update", func(t *testing.T) {
		parentID := "legacy-parent"
		_, err := svc.UpdateAccount(ctx, schemaName, "tenant-1", "custom", &UpdateAccountRequest{
			Code:        "6150",
			Name:        "Updated Expense",
			AccountType: AccountTypeExpense,
			ParentID:    &parentID,
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "parent_id must be a valid UUID")
	})

	t.Run("propagates update repository errors", func(t *testing.T) {
		repo.updateAccountErr = errors.New("update failed")
		_, err := svc.UpdateAccount(ctx, schemaName, "tenant-1", "custom", &UpdateAccountRequest{
			Code:        "6150",
			Name:        "Updated Expense",
			AccountType: AccountTypeExpense,
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "update failed")
		repo.updateAccountErr = nil
	})

	t.Run("deactivates editable account", func(t *testing.T) {
		result, err := svc.DeactivateAccount(ctx, schemaName, "tenant-1", "custom")
		require.NoError(t, err)
		assert.False(t, result.IsActive)
		assert.False(t, repo.accounts["custom"].IsActive)
	})

	t.Run("returns get errors on deactivation", func(t *testing.T) {
		_, err := svc.DeactivateAccount(ctx, schemaName, "tenant-1", "missing")
		assert.Error(t, err)
	})

	t.Run("rejects system account deactivation", func(t *testing.T) {
		_, err := svc.DeactivateAccount(ctx, schemaName, "tenant-1", "system")
		assert.ErrorIs(t, err, ErrSystemAccountImmutable)
		assert.True(t, repo.accounts["system"].IsActive)
	})

	t.Run("propagates deactivation update errors", func(t *testing.T) {
		repo.accounts["deactivate-error"] = &Account{
			ID:          "deactivate-error",
			TenantID:    "tenant-1",
			Code:        "6200",
			Name:        "Temporary Expense",
			AccountType: AccountTypeExpense,
			IsActive:    true,
			IsSystem:    false,
		}
		repo.updateAccountErr = errors.New("deactivate failed")

		_, err := svc.DeactivateAccount(ctx, schemaName, "tenant-1", "deactivate-error")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "deactivate failed")
		repo.updateAccountErr = nil
	})
}

func TestService_GetJournalEntry(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	svc := NewServiceWithRepository(repo)
	schemaName := "tenant_test"

	entry := &JournalEntry{
		ID:          "je-1",
		TenantID:    "tenant-1",
		EntryNumber: "JE-00001",
		Status:      StatusDraft,
	}
	repo.journalEntries[entry.ID] = entry

	t.Run("returns entry when found", func(t *testing.T) {
		result, err := svc.GetJournalEntry(ctx, schemaName, "tenant-1", "je-1")
		require.NoError(t, err)
		assert.Equal(t, "je-1", result.ID)
	})

	t.Run("returns error when not found", func(t *testing.T) {
		_, err := svc.GetJournalEntry(ctx, schemaName, "tenant-1", "nonexistent")
		assert.Error(t, err)
	})
}

func TestService_ListJournalEntries(t *testing.T) {
	ctx := context.Background()
	schemaName := "tenant_test"

	t.Run("lists tenant journal entries", func(t *testing.T) {
		repo := NewMockRepository()
		svc := NewServiceWithRepository(repo)

		repo.journalEntries["je-1"] = &JournalEntry{
			ID:          "je-1",
			TenantID:    "tenant-1",
			EntryNumber: "JE-00001",
			Status:      StatusDraft,
		}
		repo.journalEntries["je-2"] = &JournalEntry{
			ID:          "je-2",
			TenantID:    "tenant-1",
			EntryNumber: "JE-00002",
			Status:      StatusPosted,
		}
		repo.journalEntries["other-tenant"] = &JournalEntry{
			ID:          "other-tenant",
			TenantID:    "tenant-2",
			EntryNumber: "JE-99999",
			Status:      StatusPosted,
		}

		result, err := svc.ListJournalEntries(ctx, schemaName, "tenant-1", 25)

		require.NoError(t, err)
		require.Len(t, result, 2)
		assert.ElementsMatch(t, []string{"je-1", "je-2"}, []string{result[0].ID, result[1].ID})
		assert.Equal(t, schemaName, repo.listJournalSchemaName)
		assert.Equal(t, "tenant-1", repo.listJournalTenantID)
		assert.Equal(t, 25, repo.listJournalLimit)
	})

	t.Run("returns repository errors", func(t *testing.T) {
		repo := NewMockRepository()
		svc := NewServiceWithRepository(repo)
		repo.getJournalErr = errors.New("journal list failed")

		result, err := svc.ListJournalEntries(ctx, schemaName, "tenant-1", 10)

		require.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "journal list failed")
		assert.Equal(t, schemaName, repo.listJournalSchemaName)
		assert.Equal(t, "tenant-1", repo.listJournalTenantID)
		assert.Equal(t, 10, repo.listJournalLimit)
	})
}

func TestService_CreateJournalEntry(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	svc := NewServiceWithRepository(repo)
	schemaName := "tenant_test"

	t.Run("creates balanced journal entry", func(t *testing.T) {
		req := &CreateJournalEntryRequest{
			EntryDate:   time.Now(),
			Description: "Test entry",
			Lines: []CreateJournalEntryLineReq{
				{AccountID: "acc-1", DebitAmount: decimal.NewFromFloat(100)},
				{AccountID: "acc-2", CreditAmount: decimal.NewFromFloat(100)},
			},
			UserID: "user-1",
		}

		result, err := svc.CreateJournalEntry(ctx, schemaName, "tenant-1", req)
		require.NoError(t, err)
		assert.NotEmpty(t, result.ID)
		assert.Equal(t, "tenant-1", result.TenantID)
		assert.Equal(t, StatusDraft, result.Status)
		assert.Equal(t, "user-1", result.CreatedBy)
		assert.Len(t, result.Lines, 2)
	})

	t.Run("preserves requested journal line IDs", func(t *testing.T) {
		lineID1 := "11111111-1111-4111-8111-111111111111"
		lineID2 := "22222222-2222-4222-8222-222222222222"
		req := &CreateJournalEntryRequest{
			EntryDate:   time.Now(),
			Description: "Historical entry",
			Lines: []CreateJournalEntryLineReq{
				{LineID: " " + lineID1 + " ", AccountID: "acc-1", DebitAmount: decimal.NewFromFloat(100)},
				{LineID: lineID2, AccountID: "acc-2", CreditAmount: decimal.NewFromFloat(100)},
			},
			UserID: "user-1",
		}

		result, err := svc.CreateJournalEntry(ctx, schemaName, "tenant-1", req)
		require.NoError(t, err)
		require.Len(t, result.Lines, 2)
		assert.Equal(t, lineID1, result.Lines[0].ID)
		assert.Equal(t, lineID2, result.Lines[1].ID)
	})

	t.Run("applies default currency EUR", func(t *testing.T) {
		req := &CreateJournalEntryRequest{
			EntryDate:   time.Now(),
			Description: "Test entry",
			Lines: []CreateJournalEntryLineReq{
				{AccountID: "acc-1", DebitAmount: decimal.NewFromFloat(100)},
				{AccountID: "acc-2", CreditAmount: decimal.NewFromFloat(100)},
			},
			UserID: "user-1",
		}

		result, err := svc.CreateJournalEntry(ctx, schemaName, "tenant-1", req)
		require.NoError(t, err)
		assert.Equal(t, "EUR", result.Lines[0].Currency)
	})

	t.Run("applies default exchange rate 1", func(t *testing.T) {
		req := &CreateJournalEntryRequest{
			EntryDate:   time.Now(),
			Description: "Test entry",
			Lines: []CreateJournalEntryLineReq{
				{AccountID: "acc-1", DebitAmount: decimal.NewFromFloat(100)},
				{AccountID: "acc-2", CreditAmount: decimal.NewFromFloat(100)},
			},
			UserID: "user-1",
		}

		result, err := svc.CreateJournalEntry(ctx, schemaName, "tenant-1", req)
		require.NoError(t, err)
		assert.True(t, result.Lines[0].ExchangeRate.Equal(decimal.NewFromInt(1)))
	})

	t.Run("calculates base amounts with exchange rate", func(t *testing.T) {
		req := &CreateJournalEntryRequest{
			EntryDate:   time.Now(),
			Description: "USD entry",
			Lines: []CreateJournalEntryLineReq{
				{AccountID: "acc-1", DebitAmount: decimal.NewFromFloat(100), Currency: "USD", ExchangeRate: decimal.NewFromFloat(0.92)},
				{AccountID: "acc-2", CreditAmount: decimal.NewFromFloat(100), Currency: "USD", ExchangeRate: decimal.NewFromFloat(0.92)},
			},
			UserID: "user-1",
		}

		result, err := svc.CreateJournalEntry(ctx, schemaName, "tenant-1", req)
		require.NoError(t, err)
		assert.True(t, result.Lines[0].BaseDebit.Equal(decimal.NewFromFloat(92)))
		assert.Equal(t, "USD", result.Lines[0].Currency)
	})

	t.Run("rejects non-positive exchange rate", func(t *testing.T) {
		req := &CreateJournalEntryRequest{
			EntryDate:   time.Now(),
			Description: "Bad FX entry",
			Lines: []CreateJournalEntryLineReq{
				{AccountID: "acc-1", DebitAmount: decimal.NewFromFloat(100), Currency: "USD", ExchangeRate: decimal.NewFromFloat(-0.92)},
				{AccountID: "acc-2", CreditAmount: decimal.NewFromFloat(100), Currency: "USD", ExchangeRate: decimal.NewFromFloat(-0.92)},
			},
			UserID: "user-1",
		}

		_, err := svc.CreateJournalEntry(ctx, schemaName, "tenant-1", req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "exchange_rate must be positive")
	})

	t.Run("rejects invalid journal line ID", func(t *testing.T) {
		req := &CreateJournalEntryRequest{
			EntryDate:   time.Now(),
			Description: "Bad line ID entry",
			Lines: []CreateJournalEntryLineReq{
				{LineID: "legacy-line", AccountID: "acc-1", DebitAmount: decimal.NewFromFloat(100)},
				{AccountID: "acc-2", CreditAmount: decimal.NewFromFloat(100)},
			},
			UserID: "user-1",
		}

		_, err := svc.CreateJournalEntry(ctx, schemaName, "tenant-1", req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "line_id must be a valid UUID")
	})

	t.Run("rejects duplicate journal line IDs", func(t *testing.T) {
		lineID := "11111111-1111-4111-8111-111111111111"
		req := &CreateJournalEntryRequest{
			EntryDate:   time.Now(),
			Description: "Duplicate line ID entry",
			Lines: []CreateJournalEntryLineReq{
				{LineID: lineID, AccountID: "acc-1", DebitAmount: decimal.NewFromFloat(100)},
				{LineID: " " + lineID + " ", AccountID: "acc-2", CreditAmount: decimal.NewFromFloat(100)},
			},
			UserID: "user-1",
		}

		_, err := svc.CreateJournalEntry(ctx, schemaName, "tenant-1", req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "duplicate line_id")
	})

	t.Run("rejects unbalanced entry", func(t *testing.T) {
		req := &CreateJournalEntryRequest{
			EntryDate:   time.Now(),
			Description: "Unbalanced entry",
			Lines: []CreateJournalEntryLineReq{
				{AccountID: "acc-1", DebitAmount: decimal.NewFromFloat(100)},
				{AccountID: "acc-2", CreditAmount: decimal.NewFromFloat(50)},
			},
			UserID: "user-1",
		}

		_, err := svc.CreateJournalEntry(ctx, schemaName, "tenant-1", req)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "validation failed")
	})

	t.Run("rejects entry with no lines", func(t *testing.T) {
		req := &CreateJournalEntryRequest{
			EntryDate:   time.Now(),
			Description: "Empty entry",
			Lines:       []CreateJournalEntryLineReq{},
			UserID:      "user-1",
		}

		_, err := svc.CreateJournalEntry(ctx, schemaName, "tenant-1", req)
		assert.Error(t, err)
	})

	t.Run("rejects zero amount entry", func(t *testing.T) {
		req := &CreateJournalEntryRequest{
			EntryDate:   time.Now(),
			Description: "Zero entry",
			Lines: []CreateJournalEntryLineReq{
				{AccountID: "acc-1", DebitAmount: decimal.Zero},
				{AccountID: "acc-2", CreditAmount: decimal.Zero},
			},
			UserID: "user-1",
		}

		_, err := svc.CreateJournalEntry(ctx, schemaName, "tenant-1", req)
		assert.Error(t, err)
	})

	t.Run("sets source type and ID", func(t *testing.T) {
		sourceID := "11111111-1111-1111-1111-111111111111"
		req := &CreateJournalEntryRequest{
			EntryDate:   time.Now(),
			Description: "Invoice entry",
			SourceType:  "INVOICE",
			SourceID:    &sourceID,
			Lines: []CreateJournalEntryLineReq{
				{AccountID: "acc-1", DebitAmount: decimal.NewFromFloat(100)},
				{AccountID: "acc-2", CreditAmount: decimal.NewFromFloat(100)},
			},
			UserID: "user-1",
		}

		result, err := svc.CreateJournalEntry(ctx, schemaName, "tenant-1", req)
		require.NoError(t, err)
		assert.Equal(t, "INVOICE", result.SourceType)
		assert.Equal(t, &sourceID, result.SourceID)
	})

	t.Run("ignores blank source ID", func(t *testing.T) {
		sourceID := " \t "
		req := &CreateJournalEntryRequest{
			EntryDate:   time.Now(),
			Description: "Manual adjustment",
			SourceType:  "MANUAL",
			SourceID:    &sourceID,
			Lines: []CreateJournalEntryLineReq{
				{AccountID: "acc-1", DebitAmount: decimal.NewFromFloat(100)},
				{AccountID: "acc-2", CreditAmount: decimal.NewFromFloat(100)},
			},
			UserID: "user-1",
		}

		result, err := svc.CreateJournalEntry(ctx, schemaName, "tenant-1", req)
		require.NoError(t, err)
		assert.Equal(t, "MANUAL", result.SourceType)
		assert.Nil(t, result.SourceID)
	})

	t.Run("rejects invalid source ID", func(t *testing.T) {
		sourceID := "legacy-invoice"
		req := &CreateJournalEntryRequest{
			EntryDate:   time.Now(),
			Description: "Invoice entry",
			SourceType:  "INVOICE",
			SourceID:    &sourceID,
			Lines: []CreateJournalEntryLineReq{
				{AccountID: "acc-1", DebitAmount: decimal.NewFromFloat(100)},
				{AccountID: "acc-2", CreditAmount: decimal.NewFromFloat(100)},
			},
			UserID: "user-1",
		}

		_, err := svc.CreateJournalEntry(ctx, schemaName, "tenant-1", req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "source_id must be a valid UUID")
	})

	t.Run("sets evidence requirement", func(t *testing.T) {
		req := &CreateJournalEntryRequest{
			EntryDate:        time.Now(),
			Description:      "Evidence controlled adjustment",
			RequiresEvidence: true,
			Lines: []CreateJournalEntryLineReq{
				{AccountID: "acc-1", DebitAmount: decimal.NewFromFloat(100)},
				{AccountID: "acc-2", CreditAmount: decimal.NewFromFloat(100)},
			},
			UserID: "user-1",
		}

		result, err := svc.CreateJournalEntry(ctx, schemaName, "tenant-1", req)
		require.NoError(t, err)
		assert.True(t, result.RequiresEvidence)
	})

	t.Run("propagates repository error", func(t *testing.T) {
		repo.createJournalErr = errors.New("database error")
		req := &CreateJournalEntryRequest{
			EntryDate:   time.Now(),
			Description: "Test entry",
			Lines: []CreateJournalEntryLineReq{
				{AccountID: "acc-1", DebitAmount: decimal.NewFromFloat(100)},
				{AccountID: "acc-2", CreditAmount: decimal.NewFromFloat(100)},
			},
			UserID: "user-1",
		}
		_, err := svc.CreateJournalEntry(ctx, schemaName, "tenant-1", req)
		assert.Error(t, err)
		repo.createJournalErr = nil
	})
}

func TestService_JournalEntryTemplates(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	svc := NewServiceWithRepository(repo)
	schemaName := "tenant_test"

	template, err := svc.CreateJournalEntryTemplate(ctx, schemaName, "tenant-1", &CreateJournalEntryTemplateRequest{
		Name:             "Monthly rent accrual",
		Description:      "Monthly rent accrual",
		Reference:        "RENT",
		RequiresEvidence: false,
		Lines: []CreateJournalEntryLineReq{
			{AccountID: "rent-expense", Description: "Rent expense", DebitAmount: decimal.RequireFromString("500.00")},
			{AccountID: "accruals", Description: "Accrued rent", CreditAmount: decimal.RequireFromString("500.00")},
		},
		UserID: "user-1",
	})
	require.NoError(t, err)
	assert.NotEmpty(t, template.ID)
	assert.Equal(t, "Monthly rent accrual", template.Name)
	assert.Equal(t, 2, template.LineCount)

	templates, err := svc.ListJournalEntryTemplates(ctx, schemaName, "tenant-1", true)
	require.NoError(t, err)
	require.Len(t, templates, 1)
	assert.Equal(t, template.ID, templates[0].ID)

	entry, err := svc.ApplyJournalEntryTemplate(ctx, schemaName, "tenant-1", template.ID, &ApplyJournalEntryTemplateRequest{
		EntryDate:   time.Date(2026, 4, 30, 0, 0, 0, 0, time.UTC),
		Description: "April rent accrual",
		Reference:   "RENT-APR",
		Post:        true,
		UserID:      "user-1",
	})
	require.NoError(t, err)
	assert.Equal(t, StatusPosted, entry.Status)
	assert.Equal(t, SourceTypeJournalTemplate, entry.SourceType)
	require.NotNil(t, entry.SourceID)
	assert.Equal(t, template.ID, *entry.SourceID)
	assert.Equal(t, "April rent accrual", entry.Description)
	assert.Equal(t, "RENT-APR", entry.Reference)
}

func TestService_JournalEntryTemplateValidation(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	svc := NewServiceWithRepository(repo)
	schemaName := "tenant_test"

	_, err := svc.CreateJournalEntryTemplate(ctx, schemaName, "tenant-1", &CreateJournalEntryTemplateRequest{
		Name:        "Unbalanced",
		Description: "Unbalanced",
		Lines: []CreateJournalEntryLineReq{
			{AccountID: "expense", DebitAmount: decimal.RequireFromString("500.00")},
			{AccountID: "accruals", CreditAmount: decimal.RequireFromString("400.00")},
		},
		UserID: "user-1",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "validation failed")

	_, err = svc.CreateJournalEntryTemplate(ctx, schemaName, "tenant-1", &CreateJournalEntryTemplateRequest{
		Name:        "Bad FX template",
		Description: "Bad FX template",
		Lines: []CreateJournalEntryLineReq{
			{AccountID: "expense", DebitAmount: decimal.RequireFromString("100.00"), Currency: "USD", ExchangeRate: decimal.RequireFromString("-0.92")},
			{AccountID: "accruals", CreditAmount: decimal.RequireFromString("100.00"), Currency: "USD", ExchangeRate: decimal.RequireFromString("-0.92")},
		},
		UserID: "user-1",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exchange_rate must be positive")

	template, err := svc.CreateJournalEntryTemplate(ctx, schemaName, "tenant-1", &CreateJournalEntryTemplateRequest{
		Name:             "Evidence controlled",
		Description:      "Evidence controlled",
		RequiresEvidence: true,
		Lines: []CreateJournalEntryLineReq{
			{AccountID: "expense", DebitAmount: decimal.RequireFromString("100.00")},
			{AccountID: "accruals", CreditAmount: decimal.RequireFromString("100.00")},
		},
		UserID: "user-1",
	})
	require.NoError(t, err)

	_, err = svc.ApplyJournalEntryTemplate(ctx, schemaName, "tenant-1", template.ID, &ApplyJournalEntryTemplateRequest{
		EntryDate: time.Date(2026, 4, 30, 0, 0, 0, 0, time.UTC),
		Post:      true,
		UserID:    "user-1",
	})
	require.ErrorIs(t, err, ErrTemplateEvidenceAutoPost)
}

func TestService_RecurringJournalEntryTemplateGeneration(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	svc := NewServiceWithRepository(repo)
	schemaName := "tenant_test"
	startDate := time.Date(2026, 4, 30, 0, 0, 0, 0, time.UTC)

	template, err := svc.CreateJournalEntryTemplate(ctx, schemaName, "tenant-1", &CreateJournalEntryTemplateRequest{
		Name:        "Monthly depreciation",
		Description: "Monthly depreciation",
		Frequency:   JournalEntryTemplateFrequencyMonthly,
		StartDate:   &startDate,
		Lines: []CreateJournalEntryLineReq{
			{AccountID: "depreciation-expense", DebitAmount: decimal.RequireFromString("250.00")},
			{AccountID: "accumulated-depreciation", CreditAmount: decimal.RequireFromString("250.00")},
		},
		UserID: "user-1",
	})
	require.NoError(t, err)
	require.NotNil(t, template.NextGenerationDate)
	assert.Equal(t, "2026-04-30", template.NextGenerationDate.Format("2006-01-02"))

	result, err := svc.GenerateJournalEntryTemplate(ctx, schemaName, "tenant-1", template.ID, &GenerateJournalEntryTemplateRequest{
		UserID: "user-1",
	})
	require.NoError(t, err)
	assert.Equal(t, "generated", result.Status)
	assert.Equal(t, "JE-00001", result.GeneratedEntryNumber)
	require.NotNil(t, result.NextGenerationDate)
	assert.Equal(t, "2026-05-30", result.NextGenerationDate.Format("2006-01-02"))
	assert.Equal(t, 1, repo.templates[template.ID].GeneratedCount)

	dueResults, err := svc.GenerateDueJournalEntryTemplates(ctx, schemaName, "tenant-1", &GenerateDueJournalEntryTemplatesRequest{
		AsOfDate: result.NextGenerationDate,
		UserID:   "user-1",
	})
	require.NoError(t, err)
	require.Len(t, dueResults, 1)
	assert.Equal(t, "generated", dueResults[0].Status)
}

func TestJournalEntryTemplateScheduleHelpers(t *testing.T) {
	from := time.Date(2026, 1, 15, 12, 30, 0, 0, time.UTC)

	nextDateCases := []struct {
		name      string
		frequency JournalEntryTemplateFrequency
		want      string
	}{
		{name: "weekly", frequency: JournalEntryTemplateFrequencyWeekly, want: "2026-01-22"},
		{name: "biweekly", frequency: JournalEntryTemplateFrequencyBiweekly, want: "2026-01-29"},
		{name: "monthly", frequency: JournalEntryTemplateFrequencyMonthly, want: "2026-02-15"},
		{name: "quarterly", frequency: JournalEntryTemplateFrequencyQuarterly, want: "2026-04-15"},
		{name: "yearly", frequency: JournalEntryTemplateFrequencyYearly, want: "2027-01-15"},
		{name: "default monthly", frequency: JournalEntryTemplateFrequency("CUSTOM"), want: "2026-02-15"},
	}

	for _, tt := range nextDateCases {
		t.Run("calculate next date "+tt.name, func(t *testing.T) {
			template := &JournalEntryTemplate{Frequency: tt.frequency}

			nextDate := template.CalculateNextDate(from)

			assert.Equal(t, tt.want, nextDate.Format("2006-01-02"))
		})
	}

	assert.True(t, isValidJournalEntryTemplateFrequency(JournalEntryTemplateFrequencyWeekly))
	assert.True(t, isValidJournalEntryTemplateFrequency(JournalEntryTemplateFrequencyYearly))
	assert.False(t, isValidJournalEntryTemplateFrequency(JournalEntryTemplateFrequency("CUSTOM")))

	startDate := time.Date(2026, 4, 30, 15, 45, 0, 0, time.UTC)
	nextDate := time.Date(2026, 5, 31, 0, 0, 0, 0, time.UTC)
	endDate := time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC)
	beforeStart := time.Date(2026, 4, 29, 0, 0, 0, 0, time.UTC)

	scheduleCases := []struct {
		name    string
		setup   func() *JournalEntryTemplate
		wantErr string
	}{
		{
			name: "invalid frequency",
			setup: func() *JournalEntryTemplate {
				return &JournalEntryTemplate{Frequency: JournalEntryTemplateFrequency("CUSTOM"), StartDate: &startDate, NextGenerationDate: &nextDate}
			},
			wantErr: "invalid journal entry template frequency",
		},
		{
			name: "missing start",
			setup: func() *JournalEntryTemplate {
				return &JournalEntryTemplate{Frequency: JournalEntryTemplateFrequencyMonthly, NextGenerationDate: &nextDate}
			},
			wantErr: "start_date is required",
		},
		{
			name: "end before start",
			setup: func() *JournalEntryTemplate {
				return &JournalEntryTemplate{Frequency: JournalEntryTemplateFrequencyMonthly, StartDate: &startDate, EndDate: &beforeStart, NextGenerationDate: &startDate}
			},
			wantErr: "end_date cannot be before start_date",
		},
		{
			name: "missing next",
			setup: func() *JournalEntryTemplate {
				return &JournalEntryTemplate{Frequency: JournalEntryTemplateFrequencyMonthly, StartDate: &startDate}
			},
			wantErr: "next_generation_date is required",
		},
		{
			name: "next before start",
			setup: func() *JournalEntryTemplate {
				return &JournalEntryTemplate{Frequency: JournalEntryTemplateFrequencyMonthly, StartDate: &startDate, NextGenerationDate: &beforeStart}
			},
			wantErr: "next_generation_date cannot be before start_date",
		},
	}

	for _, tt := range scheduleCases {
		t.Run("validates schedule "+tt.name, func(t *testing.T) {
			err := validateJournalEntryTemplateSchedule(tt.setup())

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}

	t.Run("rejects missing account ids", func(t *testing.T) {
		err := validateJournalEntryTemplate(&JournalEntryTemplate{
			Lines: []JournalEntryTemplateLine{
				{DebitAmount: decimal.RequireFromString("10.00"), ExchangeRate: decimal.NewFromInt(1)},
				{AccountID: "accruals", CreditAmount: decimal.RequireFromString("10.00"), ExchangeRate: decimal.NewFromInt(1)},
			},
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "line account_id is required")
	})

	t.Run("returns schedule validation errors", func(t *testing.T) {
		err := validateJournalEntryTemplate(&JournalEntryTemplate{
			Frequency: JournalEntryTemplateFrequency("CUSTOM"),
			Lines: []JournalEntryTemplateLine{
				{AccountID: "expense", DebitAmount: decimal.RequireFromString("10.00"), ExchangeRate: decimal.NewFromInt(1)},
				{AccountID: "accruals", CreditAmount: decimal.RequireFromString("10.00"), ExchangeRate: decimal.NewFromInt(1)},
			},
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid journal entry template frequency")
	})

	recurringTemplate := &JournalEntryTemplate{
		Frequency:          JournalEntryTemplateFrequencyMonthly,
		StartDate:          &startDate,
		EndDate:            &endDate,
		NextGenerationDate: &nextDate,
	}

	t.Run("uses requested recurring date", func(t *testing.T) {
		requested := time.Date(2026, 6, 15, 18, 10, 0, 0, time.UTC)

		entryDate, err := recurringTemplateEntryDate(recurringTemplate, &requested)

		require.NoError(t, err)
		assert.Equal(t, "2026-06-15", entryDate.Format("2006-01-02"))
		assert.Equal(t, 0, entryDate.Hour())
	})

	t.Run("falls back to start date without next date", func(t *testing.T) {
		template := &JournalEntryTemplate{
			Frequency: JournalEntryTemplateFrequencyMonthly,
			StartDate: &startDate,
		}

		entryDate, err := recurringTemplateEntryDate(template, nil)

		require.NoError(t, err)
		assert.Equal(t, "2026-04-30", entryDate.Format("2006-01-02"))
	})

	t.Run("rejects non recurring template", func(t *testing.T) {
		_, err := recurringTemplateEntryDate(&JournalEntryTemplate{}, nil)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "not recurring")
	})

	t.Run("rejects recurring template without a date", func(t *testing.T) {
		_, err := recurringTemplateEntryDate(&JournalEntryTemplate{Frequency: JournalEntryTemplateFrequencyMonthly}, nil)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "no next generation date")
	})

	t.Run("rejects requested dates after the end date", func(t *testing.T) {
		requested := time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)

		_, err := recurringTemplateEntryDate(recurringTemplate, &requested)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "ended on 2026-12-31")
	})
}

func TestService_JournalEntryTemplateErrorPaths(t *testing.T) {
	ctx := context.Background()
	schemaName := "tenant_test"
	tenantID := "tenant-1"
	userID := "user-1"
	startDate := time.Date(2026, 4, 30, 0, 0, 0, 0, time.UTC)

	balancedLines := func(amount string) []CreateJournalEntryLineReq {
		return []CreateJournalEntryLineReq{
			{AccountID: "expense", Description: "Expense", DebitAmount: decimal.RequireFromString(amount)},
			{AccountID: "accruals", Description: "Accrual", CreditAmount: decimal.RequireFromString(amount)},
		}
	}
	createTemplate := func(t *testing.T, repo *MockRepository, req *CreateJournalEntryTemplateRequest) *JournalEntryTemplate {
		t.Helper()
		svc := NewServiceWithRepository(repo)
		template, err := svc.CreateJournalEntryTemplate(ctx, schemaName, tenantID, req)
		require.NoError(t, err)
		return template
	}

	t.Run("rejects missing name and lines", func(t *testing.T) {
		svc := NewServiceWithRepository(NewMockRepository())

		_, err := svc.CreateJournalEntryTemplate(ctx, schemaName, tenantID, &CreateJournalEntryTemplateRequest{
			Lines:  balancedLines("10.00"),
			UserID: userID,
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "name is required")

		_, err = svc.CreateJournalEntryTemplate(ctx, schemaName, tenantID, &CreateJournalEntryTemplateRequest{
			Name:   "Incomplete",
			Lines:  balancedLines("10.00")[:1],
			UserID: userID,
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "at least two lines are required")
	})

	t.Run("rejects repositories without template support", func(t *testing.T) {
		svc := NewServiceWithRepository(repositoryWithoutJournalTemplates{RepositoryInterface: NewMockRepository()})

		_, err := svc.CreateJournalEntryTemplate(ctx, schemaName, tenantID, &CreateJournalEntryTemplateRequest{
			Name:   "Unsupported",
			Lines:  balancedLines("10.00"),
			UserID: userID,
		})
		require.ErrorIs(t, err, errJournalEntryTemplatesUnsupported)

		_, err = svc.ListJournalEntryTemplates(ctx, schemaName, tenantID, true)
		require.ErrorIs(t, err, errJournalEntryTemplatesUnsupported)

		_, err = svc.GetJournalEntryTemplate(ctx, schemaName, tenantID, "template-1")
		require.ErrorIs(t, err, errJournalEntryTemplatesUnsupported)

		_, err = svc.GenerateJournalEntryTemplate(ctx, schemaName, tenantID, "template-1", &GenerateJournalEntryTemplateRequest{
			UserID: userID,
		})
		require.ErrorIs(t, err, errJournalEntryTemplatesUnsupported)

		_, err = svc.GenerateDueJournalEntryTemplates(ctx, schemaName, tenantID, &GenerateDueJournalEntryTemplatesRequest{
			UserID: userID,
		})
		require.ErrorIs(t, err, errJournalEntryTemplatesUnsupported)
	})

	t.Run("wraps template repository errors", func(t *testing.T) {
		repo := NewMockRepository()
		repo.createJournalErr = errors.New("insert failed")
		svc := NewServiceWithRepository(repo)

		_, err := svc.CreateJournalEntryTemplate(ctx, schemaName, tenantID, &CreateJournalEntryTemplateRequest{
			Name:   "Repository failure",
			Lines:  balancedLines("10.00"),
			UserID: userID,
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "create journal entry template")
		assert.Contains(t, err.Error(), "insert failed")

		repo = NewMockRepository()
		repo.listTemplatesErr = errors.New("list failed")
		svc = NewServiceWithRepository(repo)
		_, err = svc.ListJournalEntryTemplates(ctx, schemaName, tenantID, true)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "list failed")

		repo = NewMockRepository()
		repo.getTemplateErr = errors.New("get failed")
		svc = NewServiceWithRepository(repo)
		_, err = svc.GetJournalEntryTemplate(ctx, schemaName, tenantID, "template-1")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "get failed")
	})

	t.Run("applies template defaults to a draft entry", func(t *testing.T) {
		repo := NewMockRepository()
		template := createTemplate(t, repo, &CreateJournalEntryTemplateRequest{
			Name:        "Monthly accrual",
			Description: "Default description",
			Reference:   "ACCRUAL",
			Lines:       balancedLines("10.00"),
			UserID:      userID,
		})
		svc := NewServiceWithRepository(repo)

		entry, err := svc.ApplyJournalEntryTemplate(ctx, schemaName, tenantID, template.ID, &ApplyJournalEntryTemplateRequest{
			UserID: userID,
		})

		require.NoError(t, err)
		assert.Equal(t, StatusDraft, entry.Status)
		assert.False(t, entry.EntryDate.IsZero())
		assert.Equal(t, "Default description", entry.Description)
		assert.Equal(t, "ACCRUAL", entry.Reference)
	})

	t.Run("rejects inactive templates", func(t *testing.T) {
		repo := NewMockRepository()
		template := createTemplate(t, repo, &CreateJournalEntryTemplateRequest{
			Name:   "Inactive",
			Lines:  balancedLines("10.00"),
			UserID: userID,
		})
		repo.templates[template.ID].IsActive = false
		svc := NewServiceWithRepository(repo)

		_, err := svc.ApplyJournalEntryTemplate(ctx, schemaName, tenantID, template.ID, &ApplyJournalEntryTemplateRequest{
			UserID: userID,
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "journal entry template is inactive")
	})

	t.Run("propagates missing template errors on apply", func(t *testing.T) {
		svc := NewServiceWithRepository(NewMockRepository())

		_, err := svc.ApplyJournalEntryTemplate(ctx, schemaName, tenantID, "missing-template", &ApplyJournalEntryTemplateRequest{
			UserID: userID,
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "journal entry template not found")
	})

	t.Run("propagates apply create and post errors", func(t *testing.T) {
		repo := NewMockRepository()
		template := createTemplate(t, repo, &CreateJournalEntryTemplateRequest{
			Name:   "Apply create failure",
			Lines:  balancedLines("10.00"),
			UserID: userID,
		})
		repo.createJournalErr = errors.New("entry insert failed")
		svc := NewServiceWithRepository(repo)

		_, err := svc.ApplyJournalEntryTemplate(ctx, schemaName, tenantID, template.ID, &ApplyJournalEntryTemplateRequest{
			UserID: userID,
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "entry insert failed")

		repo = NewMockRepository()
		template = createTemplate(t, repo, &CreateJournalEntryTemplateRequest{
			Name:   "Apply post failure",
			Lines:  balancedLines("10.00"),
			UserID: userID,
		})
		repo.updateStatusErr = errors.New("post failed")
		svc = NewServiceWithRepository(repo)

		_, err = svc.ApplyJournalEntryTemplate(ctx, schemaName, tenantID, template.ID, &ApplyJournalEntryTemplateRequest{
			Post:   true,
			UserID: userID,
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "post failed")
	})

	t.Run("rejects locked ended and non recurring generation", func(t *testing.T) {
		repo := NewMockRepository()
		nonRecurring := createTemplate(t, repo, &CreateJournalEntryTemplateRequest{
			Name:   "Manual only",
			Lines:  balancedLines("10.00"),
			UserID: userID,
		})
		svc := NewServiceWithRepository(repo)

		_, err := svc.GenerateJournalEntryTemplate(ctx, schemaName, tenantID, nonRecurring.ID, &GenerateJournalEntryTemplateRequest{
			UserID: userID,
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not recurring")

		recurring := createTemplate(t, repo, &CreateJournalEntryTemplateRequest{
			Name:      "Locked recurring",
			Frequency: JournalEntryTemplateFrequencyMonthly,
			StartDate: &startDate,
			Lines:     balancedLines("10.00"),
			UserID:    userID,
		})
		_, err = svc.GenerateJournalEntryTemplate(ctx, schemaName, tenantID, recurring.ID, &GenerateJournalEntryTemplateRequest{
			PeriodLockDate: &startDate,
			UserID:         userID,
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "period locked through 2026-04-30")

		endDate := startDate
		nextAfterEnd := startDate.AddDate(0, 1, 0)
		ended := createTemplate(t, repo, &CreateJournalEntryTemplateRequest{
			Name:               "Ended recurring",
			Frequency:          JournalEntryTemplateFrequencyMonthly,
			StartDate:          &startDate,
			EndDate:            &endDate,
			NextGenerationDate: &nextAfterEnd,
			Lines:              balancedLines("10.00"),
			UserID:             userID,
		})
		_, err = svc.GenerateJournalEntryTemplate(ctx, schemaName, tenantID, ended.ID, &GenerateJournalEntryTemplateRequest{
			UserID: userID,
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "ended on 2026-04-30")
	})

	t.Run("wraps generation apply and schedule update errors", func(t *testing.T) {
		repo := NewMockRepository()
		template := createTemplate(t, repo, &CreateJournalEntryTemplateRequest{
			Name:      "Generate apply failure",
			Frequency: JournalEntryTemplateFrequencyMonthly,
			StartDate: &startDate,
			Lines:     balancedLines("10.00"),
			UserID:    userID,
		})
		repo.createJournalErr = errors.New("entry insert failed")
		svc := NewServiceWithRepository(repo)

		_, err := svc.GenerateJournalEntryTemplate(ctx, schemaName, tenantID, template.ID, &GenerateJournalEntryTemplateRequest{
			UserID: userID,
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "entry insert failed")

		repo = NewMockRepository()
		template = createTemplate(t, repo, &CreateJournalEntryTemplateRequest{
			Name:      "Generate update failure",
			Frequency: JournalEntryTemplateFrequencyMonthly,
			StartDate: &startDate,
			Lines:     balancedLines("10.00"),
			UserID:    userID,
		})
		repo.updateTemplateErr = errors.New("schedule update failed")
		svc = NewServiceWithRepository(repo)

		_, err = svc.GenerateJournalEntryTemplate(ctx, schemaName, tenantID, template.ID, &GenerateJournalEntryTemplateRequest{
			UserID: userID,
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "update recurring journal template")
		assert.Contains(t, err.Error(), "schedule update failed")
	})

	t.Run("reports due generation list and per-template errors", func(t *testing.T) {
		repo := NewMockRepository()
		repo.dueTemplatesErr = errors.New("query failed")
		svc := NewServiceWithRepository(repo)

		_, err := svc.GenerateDueJournalEntryTemplates(ctx, schemaName, tenantID, &GenerateDueJournalEntryTemplatesRequest{
			UserID: userID,
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "list due recurring journal templates")
		assert.Contains(t, err.Error(), "query failed")

		repo = NewMockRepository()
		repo.dueTemplateIDs = []string{"missing-template"}
		svc = NewServiceWithRepository(repo)

		results, err := svc.GenerateDueJournalEntryTemplates(ctx, schemaName, tenantID, &GenerateDueJournalEntryTemplatesRequest{
			UserID: userID,
		})
		require.NoError(t, err)
		require.Len(t, results, 1)
		assert.Equal(t, "missing-template", results[0].TemplateID)
		assert.Equal(t, "error", results[0].Status)
		assert.Contains(t, results[0].Error, "journal entry template not found")
	})
}

func TestService_PostJournalEntry(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	svc := NewServiceWithRepository(repo)
	schemaName := "tenant_test"

	t.Run("posts draft entry", func(t *testing.T) {
		entry := &JournalEntry{
			ID:       "je-1",
			TenantID: "tenant-1",
			Status:   StatusDraft,
			Lines: []JournalEntryLine{
				{DebitAmount: decimal.NewFromFloat(100), BaseDebit: decimal.NewFromFloat(100)},
				{CreditAmount: decimal.NewFromFloat(100), BaseCredit: decimal.NewFromFloat(100)},
			},
		}
		repo.journalEntries[entry.ID] = entry

		err := svc.PostJournalEntry(ctx, schemaName, "tenant-1", "je-1", "user-1")
		require.NoError(t, err)
		assert.Equal(t, StatusPosted, entry.Status)
	})

	t.Run("rejects non-draft entry", func(t *testing.T) {
		entry := &JournalEntry{
			ID:       "je-2",
			TenantID: "tenant-1",
			Status:   StatusPosted,
			Lines: []JournalEntryLine{
				{DebitAmount: decimal.NewFromFloat(100), BaseDebit: decimal.NewFromFloat(100)},
				{CreditAmount: decimal.NewFromFloat(100), BaseCredit: decimal.NewFromFloat(100)},
			},
		}
		repo.journalEntries[entry.ID] = entry

		err := svc.PostJournalEntry(ctx, schemaName, "tenant-1", "je-2", "user-1")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "only draft entries can be posted")
	})

	t.Run("returns error when entry not found", func(t *testing.T) {
		err := svc.PostJournalEntry(ctx, schemaName, "tenant-1", "nonexistent", "user-1")
		assert.Error(t, err)
	})

	t.Run("propagates repository error", func(t *testing.T) {
		entry := &JournalEntry{
			ID:       "je-3",
			TenantID: "tenant-1",
			Status:   StatusDraft,
			Lines: []JournalEntryLine{
				{DebitAmount: decimal.NewFromFloat(100), BaseDebit: decimal.NewFromFloat(100)},
				{CreditAmount: decimal.NewFromFloat(100), BaseCredit: decimal.NewFromFloat(100)},
			},
		}
		repo.journalEntries[entry.ID] = entry
		repo.updateStatusErr = errors.New("database error")

		err := svc.PostJournalEntry(ctx, schemaName, "tenant-1", "je-3", "user-1")
		assert.Error(t, err)
		repo.updateStatusErr = nil
	})

	t.Run("rejects unbalanced entry", func(t *testing.T) {
		entry := &JournalEntry{
			ID:       "je-4",
			TenantID: "tenant-1",
			Status:   StatusDraft,
			Lines: []JournalEntryLine{
				{DebitAmount: decimal.NewFromFloat(100), BaseDebit: decimal.NewFromFloat(100)},
				{CreditAmount: decimal.NewFromFloat(50), BaseCredit: decimal.NewFromFloat(50)}, // Unbalanced
			},
		}
		repo.journalEntries[entry.ID] = entry

		err := svc.PostJournalEntry(ctx, schemaName, "tenant-1", "je-4", "user-1")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "entry validation failed")
	})
}

func TestService_VoidJournalEntry(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	svc := NewServiceWithRepository(repo)
	schemaName := "tenant_test"

	t.Run("voids posted entry and creates reversal", func(t *testing.T) {
		entry := &JournalEntry{
			ID:          "je-1",
			TenantID:    "tenant-1",
			EntryNumber: "JE-00001",
			Status:      StatusPosted,
			Lines: []JournalEntryLine{
				{ID: "line-1", AccountID: "acc-1", DebitAmount: decimal.NewFromFloat(100), CreditAmount: decimal.Zero, Currency: "EUR", ExchangeRate: decimal.NewFromInt(1), BaseDebit: decimal.NewFromFloat(100), BaseCredit: decimal.Zero},
				{ID: "line-2", AccountID: "acc-2", DebitAmount: decimal.Zero, CreditAmount: decimal.NewFromFloat(100), Currency: "EUR", ExchangeRate: decimal.NewFromInt(1), BaseDebit: decimal.Zero, BaseCredit: decimal.NewFromFloat(100)},
			},
		}
		repo.journalEntries[entry.ID] = entry

		reversal, err := svc.VoidJournalEntry(ctx, schemaName, "tenant-1", "je-1", "user-1", "Test void")
		require.NoError(t, err)
		assert.NotNil(t, reversal)
		assert.NotEmpty(t, reversal.ID)
		assert.Equal(t, "tenant-1", reversal.TenantID)
		assert.Equal(t, StatusPosted, reversal.Status)
		assert.Equal(t, "VOID", reversal.SourceType)
		assert.Contains(t, reversal.Description, "Reversal of JE-00001")
		assert.Len(t, reversal.Lines, 2)
		// Check reversal lines swap debits and credits
		assert.True(t, reversal.Lines[0].CreditAmount.Equal(decimal.NewFromFloat(100)))
		assert.True(t, reversal.Lines[0].DebitAmount.Equal(decimal.Zero))
	})

	t.Run("rejects non-posted entry", func(t *testing.T) {
		entry := &JournalEntry{
			ID:       "je-draft",
			TenantID: "tenant-1",
			Status:   StatusDraft,
		}
		repo.journalEntries[entry.ID] = entry

		_, err := svc.VoidJournalEntry(ctx, schemaName, "tenant-1", "je-draft", "user-1", "Test void")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "only posted entries can be voided")
	})

	t.Run("returns error when entry not found", func(t *testing.T) {
		_, err := svc.VoidJournalEntry(ctx, schemaName, "tenant-1", "nonexistent", "user-1", "Test void")
		assert.Error(t, err)
	})

	t.Run("propagates repository error", func(t *testing.T) {
		entry := &JournalEntry{
			ID:       "je-2",
			TenantID: "tenant-1",
			Status:   StatusPosted,
			Lines: []JournalEntryLine{
				{AccountID: "acc-1", DebitAmount: decimal.NewFromFloat(100), BaseDebit: decimal.NewFromFloat(100)},
				{AccountID: "acc-2", CreditAmount: decimal.NewFromFloat(100), BaseCredit: decimal.NewFromFloat(100)},
			},
		}
		repo.journalEntries[entry.ID] = entry
		repo.voidJournalEntryErr = errors.New("database error")

		_, err := svc.VoidJournalEntry(ctx, schemaName, "tenant-1", "je-2", "user-1", "Test void")
		assert.Error(t, err)
		repo.voidJournalEntryErr = nil
	})
}

func TestService_GetAccountBalance(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	svc := NewServiceWithRepository(repo)
	schemaName := "tenant_test"

	t.Run("returns balance", func(t *testing.T) {
		balance, err := svc.GetAccountBalance(ctx, schemaName, "tenant-1", "acc-1", time.Now())
		require.NoError(t, err)
		assert.True(t, balance.Equal(decimal.NewFromFloat(1000)))
	})

	t.Run("propagates repository error", func(t *testing.T) {
		repo.getBalanceErr = errors.New("database error")
		_, err := svc.GetAccountBalance(ctx, schemaName, "tenant-1", "acc-1", time.Now())
		assert.Error(t, err)
		repo.getBalanceErr = nil
	})
}

func TestService_GetTrialBalance(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	svc := NewServiceWithRepository(repo)
	schemaName := "tenant_test"

	repo.balances = []AccountBalance{
		{AccountID: "acc-1", AccountCode: "1000", AccountName: "Cash", AccountType: AccountTypeAsset, DebitBalance: decimal.NewFromFloat(5000), CreditBalance: decimal.Zero},
		{AccountID: "acc-2", AccountCode: "2000", AccountName: "Payables", AccountType: AccountTypeLiability, DebitBalance: decimal.Zero, CreditBalance: decimal.NewFromFloat(3000)},
		{AccountID: "acc-3", AccountCode: "3000", AccountName: "Capital", AccountType: AccountTypeEquity, DebitBalance: decimal.Zero, CreditBalance: decimal.NewFromFloat(2000)},
	}

	t.Run("returns trial balance", func(t *testing.T) {
		result, err := svc.GetTrialBalance(ctx, schemaName, "tenant-1", time.Now())
		require.NoError(t, err)
		assert.Equal(t, "tenant-1", result.TenantID)
		assert.Len(t, result.Accounts, 3)
		assert.True(t, result.TotalDebits.Equal(decimal.NewFromFloat(5000)))
		assert.True(t, result.TotalCredits.Equal(decimal.NewFromFloat(5000)))
		assert.True(t, result.IsBalanced)
	})

	t.Run("propagates repository error", func(t *testing.T) {
		repo.trialBalanceErr = errors.New("database error")
		_, err := svc.GetTrialBalance(ctx, schemaName, "tenant-1", time.Now())
		assert.Error(t, err)
		repo.trialBalanceErr = nil
	})
}

func TestService_GetBalanceSheet(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	svc := NewServiceWithRepository(repo)
	schemaName := "tenant_test"

	t.Run("generates balanced balance sheet", func(t *testing.T) {
		repo.balances = []AccountBalance{
			{AccountID: "acc-1", AccountCode: "1000", AccountName: "Cash", AccountType: AccountTypeAsset, NetBalance: decimal.NewFromFloat(10000)},
			{AccountID: "acc-2", AccountCode: "2000", AccountName: "Payables", AccountType: AccountTypeLiability, NetBalance: decimal.NewFromFloat(-3000)},
			{AccountID: "acc-3", AccountCode: "3000", AccountName: "Capital", AccountType: AccountTypeEquity, NetBalance: decimal.NewFromFloat(-5000)},
			{AccountID: "acc-4", AccountCode: "4000", AccountName: "Revenue", AccountType: AccountTypeRevenue, NetBalance: decimal.NewFromFloat(-4000)},
			{AccountID: "acc-5", AccountCode: "5000", AccountName: "Expenses", AccountType: AccountTypeExpense, NetBalance: decimal.NewFromFloat(2000)},
		}

		result, err := svc.GetBalanceSheet(ctx, schemaName, "tenant-1", time.Now())
		require.NoError(t, err)
		assert.Equal(t, "tenant-1", result.TenantID)
		assert.Len(t, result.Assets, 1)
		assert.Len(t, result.Liabilities, 1)
		assert.Len(t, result.Equity, 1)
		assert.True(t, result.TotalAssets.Equal(decimal.NewFromFloat(10000)))
		assert.True(t, result.TotalLiabilities.Equal(decimal.NewFromFloat(3000)))
		// RetainedEarnings = Revenue (4000) - Expenses (2000) = 2000
		assert.True(t, result.RetainedEarnings.Equal(decimal.NewFromFloat(2000)))
		// TotalEquity = Capital (5000) + RetainedEarnings (2000) = 7000
		assert.True(t, result.TotalEquity.Equal(decimal.NewFromFloat(7000)))
		assert.True(t, result.IsBalanced) // 10000 = 3000 + 7000
	})

	t.Run("propagates repository error", func(t *testing.T) {
		repo.trialBalanceErr = errors.New("database error")
		_, err := svc.GetBalanceSheet(ctx, schemaName, "tenant-1", time.Now())
		assert.Error(t, err)
		repo.trialBalanceErr = nil
	})
}

func TestService_GetIncomeStatement(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	svc := NewServiceWithRepository(repo)
	schemaName := "tenant_test"

	t.Run("generates income statement", func(t *testing.T) {
		repo.periodBalances = []AccountBalance{
			{AccountID: "acc-1", AccountCode: "4000", AccountName: "Sales", AccountType: AccountTypeRevenue, NetBalance: decimal.NewFromFloat(-10000)},
			{AccountID: "acc-2", AccountCode: "4100", AccountName: "Services", AccountType: AccountTypeRevenue, NetBalance: decimal.NewFromFloat(-5000)},
			{AccountID: "acc-3", AccountCode: "5000", AccountName: "COGS", AccountType: AccountTypeExpense, NetBalance: decimal.NewFromFloat(6000)},
			{AccountID: "acc-4", AccountCode: "5100", AccountName: "Wages", AccountType: AccountTypeExpense, NetBalance: decimal.NewFromFloat(3000)},
		}

		startDate := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
		endDate := time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC)

		result, err := svc.GetIncomeStatement(ctx, schemaName, "tenant-1", startDate, endDate)
		require.NoError(t, err)
		assert.Equal(t, "tenant-1", result.TenantID)
		assert.Equal(t, startDate, result.StartDate)
		assert.Equal(t, endDate, result.EndDate)
		assert.Len(t, result.Revenue, 2)
		assert.Len(t, result.Expenses, 2)
		assert.True(t, result.TotalRevenue.Equal(decimal.NewFromFloat(15000)))
		assert.True(t, result.TotalExpenses.Equal(decimal.NewFromFloat(9000)))
		assert.True(t, result.NetIncome.Equal(decimal.NewFromFloat(6000)))
	})

	t.Run("propagates repository error", func(t *testing.T) {
		repo.periodBalanceErr = errors.New("database error")
		_, err := svc.GetIncomeStatement(ctx, schemaName, "tenant-1", time.Now(), time.Now())
		assert.Error(t, err)
		repo.periodBalanceErr = nil
	})
}

func TestTrialBalanceIsBalanced(t *testing.T) {
	t.Run("balanced when debits equal credits", func(t *testing.T) {
		tb := &TrialBalance{
			TotalDebits:  decimal.NewFromFloat(5000),
			TotalCredits: decimal.NewFromFloat(5000),
		}
		tb.IsBalanced = tb.TotalDebits.Equal(tb.TotalCredits)
		assert.True(t, tb.IsBalanced)
	})

	t.Run("not balanced when debits differ from credits", func(t *testing.T) {
		tb := &TrialBalance{
			TotalDebits:  decimal.NewFromFloat(5000),
			TotalCredits: decimal.NewFromFloat(4000),
		}
		tb.IsBalanced = tb.TotalDebits.Equal(tb.TotalCredits)
		assert.False(t, tb.IsBalanced)
	})
}

func TestCreateAccountRequest(t *testing.T) {
	parentID := "parent-1"
	req := CreateAccountRequest{
		Code:        "1000",
		Name:        "Cash",
		AccountType: AccountTypeAsset,
		ParentID:    &parentID,
		Description: "Main cash account",
	}

	assert.Equal(t, "1000", req.Code)
	assert.Equal(t, "Cash", req.Name)
	assert.Equal(t, AccountTypeAsset, req.AccountType)
	assert.Equal(t, &parentID, req.ParentID)
	assert.Equal(t, "Main cash account", req.Description)
}

func TestService_GetYearEndCloseStatus(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	svc := NewServiceWithRepository(repo)

	repo.accounts["retained"] = &Account{
		ID:          "retained",
		TenantID:    "tenant-1",
		Code:        "3200",
		Name:        "Retained Earnings",
		AccountType: AccountTypeEquity,
		IsActive:    true,
	}
	repo.periodBalances = []AccountBalance{
		{
			AccountID:     "revenue-1",
			AccountCode:   "4100",
			AccountName:   "Sales Revenue",
			AccountType:   AccountTypeRevenue,
			DebitBalance:  decimal.Zero,
			CreditBalance: decimal.NewFromInt(1000),
			NetBalance:    decimal.NewFromInt(1000),
		},
		{
			AccountID:     "expense-1",
			AccountCode:   "5100",
			AccountName:   "Salary Expenses",
			AccountType:   AccountTypeExpense,
			DebitBalance:  decimal.NewFromInt(400),
			CreditBalance: decimal.Zero,
			NetBalance:    decimal.NewFromInt(400),
		},
	}

	status, err := svc.GetYearEndCloseStatus(ctx, "tenant_test", "tenant-1", 1, "2025-12-31", stringPtr("2025-12-31"))

	require.NoError(t, err)
	require.NotNil(t, status)
	assert.Equal(t, "2025", status.FiscalYearLabel)
	assert.Equal(t, "2025-01-01", status.FiscalYearStartDate)
	assert.Equal(t, "2025-12-31", status.FiscalYearEndDate)
	assert.Equal(t, "2026-01-01", status.CarryForwardDate)
	assert.True(t, status.IsFiscalYearEnd)
	assert.True(t, status.PeriodClosed)
	assert.True(t, status.HasProfitAndLossActivity)
	assert.True(t, status.CarryForwardNeeded)
	assert.True(t, status.CarryForwardReady)
	assert.True(t, status.HasRetainedEarningsAccount)
	require.NotNil(t, status.RetainedEarningsAccount)
	assert.Equal(t, "retained", status.RetainedEarningsAccount.ID)
	assert.True(t, status.NetIncome.Equal(decimal.NewFromInt(600)))
	assert.Nil(t, status.ExistingCarryForward)
}

func TestService_GetYearEndCloseStatusDetectsExistingCarryForward(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	svc := NewServiceWithRepository(repo)

	fiscalYearEndDate, err := time.Parse(yearEndDateLayout, "2025-12-31")
	require.NoError(t, err)
	sourceID := yearEndCarryForwardSourceID("tenant-1", fiscalYearEndDate)

	repo.accounts["retained"] = &Account{
		ID:          "retained",
		TenantID:    "tenant-1",
		Code:        "3200",
		Name:        "Retained Earnings",
		AccountType: AccountTypeEquity,
		IsActive:    true,
	}
	repo.periodBalances = []AccountBalance{
		{
			AccountID:     "revenue-1",
			AccountCode:   "4100",
			AccountName:   "Sales Revenue",
			AccountType:   AccountTypeRevenue,
			CreditBalance: decimal.NewFromInt(200),
			NetBalance:    decimal.NewFromInt(200),
		},
	}
	repo.journalEntries["carry-forward"] = &JournalEntry{
		ID:          "carry-forward",
		TenantID:    "tenant-1",
		EntryNumber: "JE-00042",
		EntryDate:   fiscalYearEndDate.AddDate(0, 0, 1),
		Description: "Year-end carry-forward",
		Reference:   "CF-20251231",
		SourceType:  SourceTypeYearEndCarryForward,
		SourceID:    &sourceID,
		Status:      StatusPosted,
	}

	status, err := svc.GetYearEndCloseStatus(ctx, "tenant_test", "tenant-1", 1, "2025-12-31", stringPtr("2025-12-31"))

	require.NoError(t, err)
	assert.False(t, status.CarryForwardNeeded)
	assert.False(t, status.CarryForwardReady)
	require.NotNil(t, status.ExistingCarryForward)
	assert.Equal(t, "carry-forward", status.ExistingCarryForward.ID)
	assert.Equal(t, "JE-00042", status.ExistingCarryForward.EntryNumber)
}

func TestService_GetYearEndClosePack(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	svc := NewServiceWithRepository(repo)

	repo.accounts["retained"] = &Account{
		ID:          "retained",
		TenantID:    "tenant-1",
		Code:        "3200",
		Name:        "Retained Earnings",
		AccountType: AccountTypeEquity,
		IsActive:    true,
	}
	repo.balances = []AccountBalance{
		{
			AccountID:    "asset-1",
			AccountCode:  "1000",
			AccountName:  "Bank",
			AccountType:  AccountTypeAsset,
			DebitBalance: decimal.NewFromInt(1000),
			NetBalance:   decimal.NewFromInt(1000),
		},
		{
			AccountID:     "equity-1",
			AccountCode:   "3000",
			AccountName:   "Equity",
			AccountType:   AccountTypeEquity,
			CreditBalance: decimal.NewFromInt(1000),
			NetBalance:    decimal.NewFromInt(-1000),
		},
	}
	repo.periodBalances = []AccountBalance{
		{
			AccountID:     "revenue-1",
			AccountCode:   "4100",
			AccountName:   "Sales Revenue",
			AccountType:   AccountTypeRevenue,
			CreditBalance: decimal.NewFromInt(1000),
			NetBalance:    decimal.NewFromInt(1000),
		},
		{
			AccountID:    "expense-1",
			AccountCode:  "5100",
			AccountName:  "Salary Expenses",
			AccountType:  AccountTypeExpense,
			DebitBalance: decimal.NewFromInt(400),
			NetBalance:   decimal.NewFromInt(400),
		},
	}

	pack, err := svc.GetYearEndClosePack(ctx, "tenant_test", "tenant-1", 1, "2025-12-31", stringPtr("2025-12-31"))

	require.NoError(t, err)
	require.NotNil(t, pack.Status)
	require.NotNil(t, pack.TrialBalance)
	require.NotNil(t, pack.BalanceSheet)
	require.NotNil(t, pack.IncomeStatement)
	assert.Equal(t, "2025", pack.Status.FiscalYearLabel)
	assert.True(t, pack.TrialBalance.IsBalanced)
	assert.True(t, pack.BalanceSheet.TotalAssets.Equal(decimal.NewFromInt(1000)))
	assert.True(t, pack.IncomeStatement.NetIncome.Equal(decimal.NewFromInt(600)))
}

func TestService_CreateYearEndCarryForward(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	svc := NewServiceWithRepository(repo)

	repo.accounts["retained"] = &Account{
		ID:          "retained",
		TenantID:    "tenant-1",
		Code:        "3200",
		Name:        "Retained Earnings",
		AccountType: AccountTypeEquity,
		IsActive:    true,
	}
	repo.periodBalances = []AccountBalance{
		{
			AccountID:     "revenue-1",
			AccountCode:   "4100",
			AccountName:   "Sales Revenue",
			AccountType:   AccountTypeRevenue,
			CreditBalance: decimal.NewFromInt(1000),
			NetBalance:    decimal.NewFromInt(1000),
		},
		{
			AccountID:    "expense-1",
			AccountCode:  "5100",
			AccountName:  "Salary Expenses",
			AccountType:  AccountTypeExpense,
			DebitBalance: decimal.NewFromInt(400),
			NetBalance:   decimal.NewFromInt(400),
		},
	}

	result, err := svc.CreateYearEndCarryForward(ctx, "tenant_test", "tenant-1", 1, stringPtr("2025-12-31"), &CreateYearEndCarryForwardRequest{
		PeriodEndDate: "2025-12-31",
		UserID:        "user-1",
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.JournalEntry)
	assert.Equal(t, SourceTypeYearEndCarryForward, result.JournalEntry.SourceType)
	assert.Equal(t, StatusPosted, result.JournalEntry.Status)
	assert.Equal(t, "2026-01-01", result.JournalEntry.EntryDate.Format(yearEndDateLayout))
	require.Len(t, result.JournalEntry.Lines, 3)
	assert.True(t, result.JournalEntry.Lines[0].DebitAmount.Equal(decimal.NewFromInt(1000)))
	assert.True(t, result.JournalEntry.Lines[1].CreditAmount.Equal(decimal.NewFromInt(400)))
	assert.Equal(t, "retained", result.JournalEntry.Lines[2].AccountID)
	assert.True(t, result.JournalEntry.Lines[2].CreditAmount.Equal(decimal.NewFromInt(600)))
	require.NotNil(t, result.Status)
	require.NotNil(t, result.Status.ExistingCarryForward)
	assert.Equal(t, result.JournalEntry.ID, result.Status.ExistingCarryForward.ID)
}

func TestService_CreateYearEndCarryForwardRequiresClosedYear(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	svc := NewServiceWithRepository(repo)

	repo.accounts["retained"] = &Account{
		ID:          "retained",
		TenantID:    "tenant-1",
		Code:        "3200",
		Name:        "Retained Earnings",
		AccountType: AccountTypeEquity,
		IsActive:    true,
	}
	repo.periodBalances = []AccountBalance{
		{
			AccountID:     "revenue-1",
			AccountCode:   "4100",
			AccountName:   "Sales Revenue",
			AccountType:   AccountTypeRevenue,
			CreditBalance: decimal.NewFromInt(250),
			NetBalance:    decimal.NewFromInt(250),
		},
	}

	_, err := svc.CreateYearEndCarryForward(ctx, "tenant_test", "tenant-1", 1, stringPtr("2025-11-30"), &CreateYearEndCarryForwardRequest{
		PeriodEndDate: "2025-12-31",
		UserID:        "user-1",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "fiscal year must be closed")
}

func TestService_CreateYearEndCarryForwardRejectsNonYearEndDate(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	svc := NewServiceWithRepository(repo)

	repo.periodBalances = []AccountBalance{
		{
			AccountID:     "revenue-1",
			AccountCode:   "4100",
			AccountName:   "Sales Revenue",
			AccountType:   AccountTypeRevenue,
			CreditBalance: decimal.NewFromInt(100),
			NetBalance:    decimal.NewFromInt(100),
		},
	}

	_, err := svc.CreateYearEndCarryForward(ctx, "tenant_test", "tenant-1", 1, stringPtr("2025-11-30"), &CreateYearEndCarryForwardRequest{
		PeriodEndDate: "2025-11-30",
		UserID:        "user-1",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "must match the fiscal year end")
}

func TestService_ReverseYearEndCarryForward(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	svc := NewServiceWithRepository(repo)

	fiscalYearEndDate, err := time.Parse(yearEndDateLayout, "2025-12-31")
	require.NoError(t, err)
	sourceID := yearEndCarryForwardSourceID("tenant-1", fiscalYearEndDate)

	repo.accounts["retained"] = &Account{
		ID:          "retained",
		TenantID:    "tenant-1",
		Code:        "3200",
		Name:        "Retained Earnings",
		AccountType: AccountTypeEquity,
		IsActive:    true,
	}
	repo.periodBalances = []AccountBalance{
		{
			AccountID:     "revenue-1",
			AccountCode:   "4100",
			AccountName:   "Sales Revenue",
			AccountType:   AccountTypeRevenue,
			CreditBalance: decimal.NewFromInt(1000),
			NetBalance:    decimal.NewFromInt(1000),
		},
	}
	repo.journalEntries["carry-forward"] = &JournalEntry{
		ID:          "carry-forward",
		TenantID:    "tenant-1",
		EntryNumber: "JE-00042",
		EntryDate:   fiscalYearEndDate.AddDate(0, 0, 1),
		Description: "Year-end carry-forward",
		Reference:   "CF-20251231",
		SourceType:  SourceTypeYearEndCarryForward,
		SourceID:    &sourceID,
		Status:      StatusPosted,
		Lines: []JournalEntryLine{
			{
				AccountID:    "revenue-1",
				DebitAmount:  decimal.NewFromInt(1000),
				BaseDebit:    decimal.NewFromInt(1000),
				Currency:     "EUR",
				ExchangeRate: decimal.NewFromInt(1),
			},
			{
				AccountID:    "retained",
				CreditAmount: decimal.NewFromInt(1000),
				BaseCredit:   decimal.NewFromInt(1000),
				Currency:     "EUR",
				ExchangeRate: decimal.NewFromInt(1),
			},
		},
	}

	result, err := svc.ReverseYearEndCarryForward(ctx, "tenant_test", "tenant-1", 1, stringPtr("2025-12-31"), &ReverseYearEndCarryForwardRequest{
		PeriodEndDate: "2025-12-31",
		Reason:        "Late supplier accrual",
		UserID:        "user-1",
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.ReversalJournalEntry)
	assert.Equal(t, SourceTypeYearEndCarryForwardReversal, result.ReversalJournalEntry.SourceType)
	assert.Equal(t, "carry-forward", *result.ReversalJournalEntry.SourceID)
	assert.Equal(t, "2026-01-01", result.ReversalJournalEntry.EntryDate.Format(yearEndDateLayout))
	assert.Equal(t, StatusVoided, repo.journalEntries["carry-forward"].Status)
	require.Len(t, result.ReversalJournalEntry.Lines, 2)
	assert.True(t, result.ReversalJournalEntry.Lines[0].CreditAmount.Equal(decimal.NewFromInt(1000)))
	assert.True(t, result.ReversalJournalEntry.Lines[1].DebitAmount.Equal(decimal.NewFromInt(1000)))
	require.NotNil(t, result.Status)
	assert.Nil(t, result.Status.ExistingCarryForward)
	assert.True(t, result.Status.CarryForwardNeeded)
}

func TestService_ReverseYearEndCarryForwardRequiresExistingCarryForward(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	svc := NewServiceWithRepository(repo)

	repo.periodBalances = []AccountBalance{
		{
			AccountID:     "revenue-1",
			AccountCode:   "4100",
			AccountName:   "Sales Revenue",
			AccountType:   AccountTypeRevenue,
			CreditBalance: decimal.NewFromInt(100),
			NetBalance:    decimal.NewFromInt(100),
		},
	}

	_, err := svc.ReverseYearEndCarryForward(ctx, "tenant_test", "tenant-1", 1, stringPtr("2025-12-31"), &ReverseYearEndCarryForwardRequest{
		PeriodEndDate: "2025-12-31",
		Reason:        "Late supplier accrual",
		UserID:        "user-1",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "carry-forward does not exist")
}

func TestService_ReverseYearEndCarryForwardRequiresReason(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	svc := NewServiceWithRepository(repo)

	_, err := svc.ReverseYearEndCarryForward(ctx, "tenant_test", "tenant-1", 1, stringPtr("2025-12-31"), &ReverseYearEndCarryForwardRequest{
		PeriodEndDate: "2025-12-31",
		UserID:        "user-1",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "reason is required")
}

func TestCreateJournalEntryRequest(t *testing.T) {
	sourceID := "inv-1"
	req := CreateJournalEntryRequest{
		EntryDate:   time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC),
		Description: "Invoice payment",
		Reference:   "INV-001",
		SourceType:  "INVOICE",
		SourceID:    &sourceID,
		Lines: []CreateJournalEntryLineReq{
			{AccountID: "acc-1", DebitAmount: decimal.NewFromFloat(100)},
			{AccountID: "acc-2", CreditAmount: decimal.NewFromFloat(100)},
		},
		UserID: "user-1",
	}

	assert.Equal(t, "Invoice payment", req.Description)
	assert.Equal(t, "INV-001", req.Reference)
	assert.Equal(t, "INVOICE", req.SourceType)
	assert.Len(t, req.Lines, 2)
}

func stringPtr(value string) *string {
	return &value
}
