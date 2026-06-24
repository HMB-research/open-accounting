package accounting

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/HMB-research/open-accounting/internal/documents"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type yearEndWorkflowRepository struct {
	*MockRepository

	periodBalanceCalls    int
	periodBalanceErrors   map[int]error
	trialBalanceCalls     int
	trialBalanceErrors    map[int]error
	journalByIDCalls      int
	journalByIDErrors     map[int]error
	journalBySourceCalls  int
	journalBySourceErrors map[int]error
}

func (r *yearEndWorkflowRepository) GetPeriodBalances(ctx context.Context, schemaName, tenantID string, startDate, endDate time.Time) ([]AccountBalance, error) {
	r.periodBalanceCalls++
	if err := r.periodBalanceErrors[r.periodBalanceCalls]; err != nil {
		return nil, err
	}
	return r.MockRepository.GetPeriodBalances(ctx, schemaName, tenantID, startDate, endDate)
}

func (r *yearEndWorkflowRepository) GetTrialBalance(ctx context.Context, schemaName, tenantID string, asOfDate time.Time) ([]AccountBalance, error) {
	r.trialBalanceCalls++
	if err := r.trialBalanceErrors[r.trialBalanceCalls]; err != nil {
		return nil, err
	}
	return r.MockRepository.GetTrialBalance(ctx, schemaName, tenantID, asOfDate)
}

func (r *yearEndWorkflowRepository) GetJournalEntryByID(ctx context.Context, schemaName, tenantID, entryID string) (*JournalEntry, error) {
	r.journalByIDCalls++
	if err := r.journalByIDErrors[r.journalByIDCalls]; err != nil {
		return nil, err
	}
	return r.MockRepository.GetJournalEntryByID(ctx, schemaName, tenantID, entryID)
}

func (r *yearEndWorkflowRepository) GetJournalEntryBySource(ctx context.Context, schemaName, tenantID, sourceType, sourceID string) (*JournalEntry, error) {
	r.journalBySourceCalls++
	if err := r.journalBySourceErrors[r.journalBySourceCalls]; err != nil {
		return nil, err
	}
	return r.MockRepository.GetJournalEntryBySource(ctx, schemaName, tenantID, sourceType, sourceID)
}

func newYearEndReadyRepository() *MockRepository {
	repo := NewMockRepository()
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
	return repo
}

func addYearEndCarryForwardEntry(t *testing.T, repo *MockRepository) {
	t.Helper()

	fiscalYearEndDate, err := time.Parse(yearEndDateLayout, "2025-12-31")
	require.NoError(t, err)
	sourceID := yearEndCarryForwardSourceID("tenant-1", fiscalYearEndDate)
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
}

func TestService_GetYearEndCloseStatusEdgeCases(t *testing.T) {
	ctx := context.Background()
	schemaName := "tenant_test"

	tests := []struct {
		name      string
		repo      RepositoryInterface
		periodEnd string
		lockDate  *string
		wantErr   string
	}{
		{
			name:      "rejects invalid period end date",
			repo:      newYearEndReadyRepository(),
			periodEnd: "not-a-date",
			lockDate:  stringPtr("2025-12-31"),
			wantErr:   "period end date must use YYYY-MM-DD",
		},
		{
			name:      "rejects invalid tenant lock date",
			repo:      newYearEndReadyRepository(),
			periodEnd: "2025-12-31",
			lockDate:  stringPtr("not-a-date"),
			wantErr:   "invalid tenant period lock date",
		},
		{
			name: "propagates period balance load errors",
			repo: func() RepositoryInterface {
				repo := newYearEndReadyRepository()
				repo.periodBalanceErr = errors.New("period failed")
				return repo
			}(),
			periodEnd: "2025-12-31",
			lockDate:  stringPtr("2025-12-31"),
			wantErr:   "load fiscal-year balances",
		},
		{
			name: "wraps income statement load errors",
			repo: &yearEndWorkflowRepository{
				MockRepository:      newYearEndReadyRepository(),
				periodBalanceErrors: map[int]error{2: errors.New("income failed")},
			},
			periodEnd: "2025-12-31",
			lockDate:  stringPtr("2025-12-31"),
			wantErr:   "load fiscal-year income statement",
		},
		{
			name: "wraps account list errors",
			repo: func() RepositoryInterface {
				repo := newYearEndReadyRepository()
				repo.listAccountsErr = errors.New("accounts failed")
				return repo
			}(),
			periodEnd: "2025-12-31",
			lockDate:  stringPtr("2025-12-31"),
			wantErr:   "list accounts",
		},
		{
			name: "wraps source lookup errors",
			repo: &yearEndWorkflowRepository{
				MockRepository:        newYearEndReadyRepository(),
				journalBySourceErrors: map[int]error{1: errors.New("source failed")},
			},
			periodEnd: "2025-12-31",
			lockDate:  stringPtr("2025-12-31"),
			wantErr:   "check carry-forward journal",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewServiceWithRepository(tt.repo)

			status, err := svc.GetYearEndCloseStatus(ctx, schemaName, "tenant-1", 1, tt.periodEnd, tt.lockDate)

			require.Error(t, err)
			assert.Nil(t, status)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}

	t.Run("reports non-calendar fiscal years and missing retained earnings readiness", func(t *testing.T) {
		repo := newYearEndReadyRepository()
		delete(repo.accounts, "retained")
		svc := NewServiceWithRepository(repo)

		status, err := svc.GetYearEndCloseStatus(ctx, schemaName, "tenant-1", 7, "2025-06-30", stringPtr("2025-06-30"))

		require.NoError(t, err)
		require.NotNil(t, status)
		assert.Equal(t, "2024/2025", status.FiscalYearLabel)
		assert.Equal(t, "2024-07-01", status.FiscalYearStartDate)
		assert.Equal(t, "2025-06-30", status.FiscalYearEndDate)
		assert.True(t, status.IsFiscalYearEnd)
		assert.False(t, status.HasRetainedEarningsAccount)
		assert.False(t, status.CarryForwardReady)
		assert.Contains(t, remediationCodes(status.RemediationActions), "retained_earnings_account_missing")
	})

	t.Run("allows balanced carry-forward readiness without retained earnings account", func(t *testing.T) {
		repo := newYearEndReadyRepository()
		delete(repo.accounts, "retained")
		repo.periodBalances = []AccountBalance{
			{AccountID: "revenue-1", AccountType: AccountTypeRevenue, NetBalance: decimal.NewFromInt(100)},
			{AccountID: "expense-1", AccountType: AccountTypeExpense, NetBalance: decimal.NewFromInt(100)},
		}
		svc := NewServiceWithRepository(repo)

		status, err := svc.GetYearEndCloseStatus(ctx, schemaName, "tenant-1", 1, "2025-12-31", stringPtr("2025-12-31"))

		require.NoError(t, err)
		assert.False(t, status.HasRetainedEarningsAccount)
		assert.True(t, status.CarryForwardReady)
		assert.NotContains(t, remediationCodes(status.RemediationActions), "retained_earnings_account_missing")
		assert.Contains(t, remediationCodes(status.RemediationActions), "ready_to_post_carry_forward")
	})
}

func TestBuildYearEndCloseRemediationActions(t *testing.T) {
	t.Parallel()

	status := &YearEndCloseStatus{
		PeriodEndDate:              "2025-12-31",
		FiscalYearLabel:            "2025",
		FiscalYearEndDate:          "2025-12-31",
		IsFiscalYearEnd:            true,
		PeriodClosed:               false,
		HasProfitAndLossActivity:   true,
		CarryForwardNeeded:         true,
		HasRetainedEarningsAccount: false,
		NetIncome:                  decimal.NewFromInt(1000),
		ClosePackEvidenceEntityID:  "year-end-close-tenant-2025-12-31",
		ClosePackEvidence: &documents.EvidencePolicyResult{
			EntityType:         documents.EntityTypeYearEndClose,
			EntityID:           "year-end-close-tenant-2025-12-31",
			Compliant:          false,
			TotalCount:         1,
			PendingReviewCount: 1,
			ApprovedCount:      0,
			RejectedCount:      0,
			MissingEvidence:    false,
		},
		InventoryCostingReview: &YearEndInventoryCostingReview{
			ValuationMethod:            "WEIGHTED_AVERAGE",
			BlockingExceptionLineCount: 2,
			NegativeQuantityLineCount:  1,
			MissingCostLineCount:       1,
			Ready:                      false,
		},
	}

	actions := BuildYearEndCloseRemediationActions(status)
	codes := remediationCodes(actions)

	assert.Contains(t, codes, "fiscal_year_not_closed")
	assert.Contains(t, codes, "retained_earnings_account_missing")
	assert.Contains(t, codes, "close_pack_evidence_not_approved")
	assert.Contains(t, codes, "inventory_costing_exceptions")
	assert.NotContains(t, codes, "ready_to_post_carry_forward")
	assert.Equal(t, "accountant", actions[0].OwnerRole)
	assert.Equal(t, "year_end_close", actions[0].WorkspaceQueue)
	assert.Equal(t, "high", actions[0].Priority)
	assert.Equal(t, 1, actions[0].DueInDays)
	assert.Contains(t, actions[0].AssignmentKey, "year-end-close:fiscal-year-not-closed")
	assert.Contains(t, actions[2].Message, "pending")
	assert.Contains(t, actions[3].CLICommand, "weighted-average")

	ready := &YearEndCloseStatus{
		PeriodEndDate:              "2025-12-31",
		FiscalYearEndDate:          "2025-12-31",
		IsFiscalYearEnd:            true,
		PeriodClosed:               true,
		HasProfitAndLossActivity:   true,
		CarryForwardNeeded:         true,
		CarryForwardReady:          true,
		HasRetainedEarningsAccount: true,
		NetIncome:                  decimal.NewFromInt(1000),
	}
	assert.Equal(t, []string{"ready_to_post_carry_forward"}, remediationCodes(BuildYearEndCloseRemediationActions(ready)))

	posted := *ready
	posted.CarryForwardReady = false
	posted.ExistingCarryForward = &JournalEntrySummary{ID: "je-1", EntryNumber: "JE-1"}
	assert.Equal(t, []string{"carry_forward_already_posted"}, remediationCodes(BuildYearEndCloseRemediationActions(&posted)))

	notYearEnd := &YearEndCloseStatus{
		PeriodEndDate:     "2025-11-30",
		FiscalYearEndDate: "2025-12-31",
		IsFiscalYearEnd:   false,
	}
	assert.Equal(t, []string{"period_not_fiscal_year_end"}, remediationCodes(BuildYearEndCloseRemediationActions(notYearEnd)))
}

func TestYearEndCloseRemediationMessageEdgeCases(t *testing.T) {
	t.Parallel()

	assert.Nil(t, BuildYearEndCloseRemediationActions(nil))
	assert.Equal(t, "Approved close-pack evidence is missing.", closePackEvidenceMessage(nil))
	assert.Equal(t, "Approved close-pack evidence is missing.", closePackEvidenceMessage(&documents.EvidencePolicyResult{}))
	assert.Equal(
		t,
		"Close-pack evidence has 2 rejected document(s) and no approved document.",
		closePackEvidenceMessage(&documents.EvidencePolicyResult{TotalCount: 2, RejectedCount: 2}),
	)
	assert.Equal(
		t,
		"Close-pack evidence is not compliant.",
		closePackEvidenceMessage(&documents.EvidencePolicyResult{TotalCount: 1, ApprovedCount: 1, Compliant: false}),
	)

	status := &YearEndCloseStatus{
		FiscalYearEndDate:        "2025-12-31",
		IsFiscalYearEnd:          true,
		PeriodClosed:             true,
		HasProfitAndLossActivity: false,
	}
	actions := BuildYearEndCloseRemediationActions(status)
	assert.Equal(t, []string{"no_profit_and_loss_activity"}, remediationCodes(actions))
	require.Len(t, actions, 1)
	assert.Contains(t, actions[0].AssignmentKey, "2025-12-31")
}

func remediationCodes(actions []YearEndCloseRemediationAction) []string {
	codes := make([]string, 0, len(actions))
	for _, action := range actions {
		codes = append(codes, action.Code)
	}
	return codes
}

func TestService_GetYearEndClosePackErrorPaths(t *testing.T) {
	ctx := context.Background()
	schemaName := "tenant_test"

	tests := []struct {
		name      string
		repo      RepositoryInterface
		periodEnd string
		wantErr   string
	}{
		{
			name:      "returns close status errors",
			repo:      newYearEndReadyRepository(),
			periodEnd: "not-a-date",
			wantErr:   "period end date must use YYYY-MM-DD",
		},
		{
			name:      "rejects non fiscal year end",
			repo:      newYearEndReadyRepository(),
			periodEnd: "2025-11-30",
			wantErr:   "period end date must match the fiscal year end",
		},
		{
			name: "wraps trial balance errors",
			repo: func() RepositoryInterface {
				repo := newYearEndReadyRepository()
				repo.trialBalanceErr = errors.New("trial failed")
				return repo
			}(),
			periodEnd: "2025-12-31",
			wantErr:   "load year-end trial balance",
		},
		{
			name: "wraps balance sheet errors",
			repo: &yearEndWorkflowRepository{
				MockRepository:     newYearEndReadyRepository(),
				trialBalanceErrors: map[int]error{2: errors.New("balance failed")},
			},
			periodEnd: "2025-12-31",
			wantErr:   "load year-end balance sheet",
		},
		{
			name: "wraps final income statement errors",
			repo: &yearEndWorkflowRepository{
				MockRepository:      newYearEndReadyRepository(),
				periodBalanceErrors: map[int]error{3: errors.New("income failed")},
			},
			periodEnd: "2025-12-31",
			wantErr:   "load fiscal-year income statement",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewServiceWithRepository(tt.repo)

			pack, err := svc.GetYearEndClosePack(ctx, schemaName, "tenant-1", 1, tt.periodEnd, stringPtr("2025-12-31"))

			require.Error(t, err)
			assert.Nil(t, pack)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestService_CreateYearEndCarryForwardErrorPaths(t *testing.T) {
	ctx := context.Background()
	schemaName := "tenant_test"
	request := &CreateYearEndCarryForwardRequest{PeriodEndDate: "2025-12-31", UserID: "user-1"}

	tests := []struct {
		name    string
		repo    RepositoryInterface
		req     *CreateYearEndCarryForwardRequest
		lock    *string
		wantErr string
	}{
		{
			name:    "requires request",
			repo:    newYearEndReadyRepository(),
			req:     nil,
			lock:    stringPtr("2025-12-31"),
			wantErr: "request is required",
		},
		{
			name:    "requires user id",
			repo:    newYearEndReadyRepository(),
			req:     &CreateYearEndCarryForwardRequest{PeriodEndDate: "2025-12-31"},
			lock:    stringPtr("2025-12-31"),
			wantErr: "user_id is required",
		},
		{
			name:    "returns close status errors",
			repo:    newYearEndReadyRepository(),
			req:     &CreateYearEndCarryForwardRequest{PeriodEndDate: "not-a-date", UserID: "user-1"},
			lock:    stringPtr("2025-12-31"),
			wantErr: "period end date must use YYYY-MM-DD",
		},
		{
			name: "rejects years without P&L activity",
			repo: func() RepositoryInterface {
				repo := newYearEndReadyRepository()
				repo.periodBalances = nil
				return repo
			}(),
			req:     request,
			lock:    stringPtr("2025-12-31"),
			wantErr: "no revenue or expense activity",
		},
		{
			name: "rejects duplicate carry-forward",
			repo: func() RepositoryInterface {
				repo := newYearEndReadyRepository()
				addYearEndCarryForwardEntry(t, repo)
				return repo
			}(),
			req:     request,
			lock:    stringPtr("2025-12-31"),
			wantErr: "carry-forward already exists",
		},
		{
			name: "requires retained earnings account for imbalanced carry-forward",
			repo: func() RepositoryInterface {
				repo := newYearEndReadyRepository()
				delete(repo.accounts, "retained")
				return repo
			}(),
			req:     request,
			lock:    stringPtr("2025-12-31"),
			wantErr: "retained earnings account is required",
		},
		{
			name: "wraps final balance reload errors",
			repo: &yearEndWorkflowRepository{
				MockRepository:      newYearEndReadyRepository(),
				periodBalanceErrors: map[int]error{3: errors.New("reload failed")},
			},
			req:     request,
			lock:    stringPtr("2025-12-31"),
			wantErr: "load fiscal-year balances",
		},
		{
			name: "wraps journal creation errors",
			repo: func() RepositoryInterface {
				repo := newYearEndReadyRepository()
				repo.createJournalErr = errors.New("create failed")
				return repo
			}(),
			req:     request,
			lock:    stringPtr("2025-12-31"),
			wantErr: "create carry-forward journal entry",
		},
		{
			name: "wraps post errors",
			repo: func() RepositoryInterface {
				repo := newYearEndReadyRepository()
				repo.updateStatusErr = errors.New("post failed")
				return repo
			}(),
			req:     request,
			lock:    stringPtr("2025-12-31"),
			wantErr: "post carry-forward journal entry",
		},
		{
			name: "wraps posted journal reload errors",
			repo: &yearEndWorkflowRepository{
				MockRepository:    newYearEndReadyRepository(),
				journalByIDErrors: map[int]error{2: errors.New("reload journal failed")},
			},
			req:     request,
			lock:    stringPtr("2025-12-31"),
			wantErr: "load carry-forward journal entry",
		},
		{
			name: "returns refreshed status errors",
			repo: &yearEndWorkflowRepository{
				MockRepository:      newYearEndReadyRepository(),
				periodBalanceErrors: map[int]error{4: errors.New("status refresh failed")},
			},
			req:     request,
			lock:    stringPtr("2025-12-31"),
			wantErr: "load fiscal-year balances",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewServiceWithRepository(tt.repo)

			result, err := svc.CreateYearEndCarryForward(ctx, schemaName, "tenant-1", 1, tt.lock, tt.req)

			require.Error(t, err)
			assert.Nil(t, result)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestService_ReverseYearEndCarryForwardErrorPaths(t *testing.T) {
	ctx := context.Background()
	schemaName := "tenant_test"
	request := &ReverseYearEndCarryForwardRequest{PeriodEndDate: "2025-12-31", Reason: "Late accrual", UserID: "user-1"}

	tests := []struct {
		name    string
		repo    RepositoryInterface
		req     *ReverseYearEndCarryForwardRequest
		lock    *string
		wantErr string
	}{
		{
			name:    "requires request",
			repo:    newYearEndReadyRepository(),
			req:     nil,
			lock:    stringPtr("2025-12-31"),
			wantErr: "request is required",
		},
		{
			name:    "requires user id",
			repo:    newYearEndReadyRepository(),
			req:     &ReverseYearEndCarryForwardRequest{PeriodEndDate: "2025-12-31", Reason: "Late accrual"},
			lock:    stringPtr("2025-12-31"),
			wantErr: "user_id is required",
		},
		{
			name:    "returns close status errors",
			repo:    newYearEndReadyRepository(),
			req:     &ReverseYearEndCarryForwardRequest{PeriodEndDate: "not-a-date", Reason: "Late accrual", UserID: "user-1"},
			lock:    stringPtr("2025-12-31"),
			wantErr: "period end date must use YYYY-MM-DD",
		},
		{
			name: "rejects non fiscal year end",
			repo: func() RepositoryInterface {
				repo := newYearEndReadyRepository()
				addYearEndCarryForwardEntry(t, repo)
				return repo
			}(),
			req:     &ReverseYearEndCarryForwardRequest{PeriodEndDate: "2025-11-30", Reason: "Late accrual", UserID: "user-1"},
			lock:    stringPtr("2025-12-31"),
			wantErr: "period end date must match the fiscal year end",
		},
		{
			name: "wraps original journal load errors",
			repo: func() RepositoryInterface {
				base := newYearEndReadyRepository()
				addYearEndCarryForwardEntry(t, base)
				return &yearEndWorkflowRepository{
					MockRepository:    base,
					journalByIDErrors: map[int]error{1: errors.New("load original failed")},
				}
			}(),
			req:     request,
			lock:    stringPtr("2025-12-31"),
			wantErr: "load carry-forward journal entry",
		},
		{
			name: "wraps reversal errors",
			repo: func() RepositoryInterface {
				repo := newYearEndReadyRepository()
				addYearEndCarryForwardEntry(t, repo)
				repo.voidJournalEntryErr = errors.New("void failed")
				return repo
			}(),
			req:     request,
			lock:    stringPtr("2025-12-31"),
			wantErr: "reverse carry-forward journal entry",
		},
		{
			name: "returns refreshed status errors",
			repo: func() RepositoryInterface {
				base := newYearEndReadyRepository()
				addYearEndCarryForwardEntry(t, base)
				return &yearEndWorkflowRepository{
					MockRepository:      base,
					periodBalanceErrors: map[int]error{3: errors.New("status refresh failed")},
				}
			}(),
			req:     request,
			lock:    stringPtr("2025-12-31"),
			wantErr: "load fiscal-year balances",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewServiceWithRepository(tt.repo)

			result, err := svc.ReverseYearEndCarryForward(ctx, schemaName, "tenant-1", 1, tt.lock, tt.req)

			require.Error(t, err)
			assert.Nil(t, result)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}
