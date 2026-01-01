package accounting

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockRepository is a mock implementation of RepositoryInterface for testing
type MockRepository struct {
	accounts       map[string]*Account
	journalEntries map[string]*JournalEntry
	accountsByType map[AccountType][]Account
	balances       []AccountBalance
	periodBalances []AccountBalance

	// Error injection
	getAccountErr       error
	listAccountsErr     error
	createAccountErr    error
	getJournalErr       error
	createJournalErr    error
	updateStatusErr     error
	getBalanceErr       error
	trialBalanceErr     error
	periodBalanceErr    error
	voidJournalEntryErr error
}

func NewMockRepository() *MockRepository {
	return &MockRepository{
		accounts:       make(map[string]*Account),
		journalEntries: make(map[string]*JournalEntry),
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

func (m *MockRepository) CreateJournalEntry(ctx context.Context, schemaName string, je *JournalEntry) error {
	if m.createJournalErr != nil {
		return m.createJournalErr
	}
	je.EntryNumber = "JE-00001"
	m.journalEntries[je.ID] = je
	return nil
}

func (m *MockRepository) CreateJournalEntryTx(ctx context.Context, schemaName string, tx pgx.Tx, je *JournalEntry) error {
	return m.CreateJournalEntry(ctx, schemaName, je)
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

func TestNewServiceWithRepo(t *testing.T) {
	// NewServiceWithRepo allows injecting a mock repository for testing
	svc := NewServiceWithRepo(nil, NewMockRepository())
	assert.NotNil(t, svc)
}

func TestService_GetAccount(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	svc := NewServiceWithRepo(nil, repo)
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
	svc := NewServiceWithRepo(nil, repo)
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

func TestService_CreateAccount(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	svc := NewServiceWithRepo(nil, repo)
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

	t.Run("creates account with parent", func(t *testing.T) {
		parentID := "parent-1"
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

func TestService_GetJournalEntry(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	svc := NewServiceWithRepo(nil, repo)
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

func TestService_CreateJournalEntry(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	svc := NewServiceWithRepo(nil, repo)
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
		sourceID := "inv-1"
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

func TestService_PostJournalEntry(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	svc := NewServiceWithRepo(nil, repo)
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
	svc := NewServiceWithRepo(nil, repo)
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
	svc := NewServiceWithRepo(nil, repo)
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
	svc := NewServiceWithRepo(nil, repo)
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
	svc := NewServiceWithRepo(nil, repo)
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
	svc := NewServiceWithRepo(nil, repo)
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
