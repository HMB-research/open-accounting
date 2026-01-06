package accounting

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

// BudgetPeriod represents the budget period for a cost center
type BudgetPeriod string

const (
	BudgetPeriodMonthly   BudgetPeriod = "MONTHLY"
	BudgetPeriodQuarterly BudgetPeriod = "QUARTERLY"
	BudgetPeriodAnnual    BudgetPeriod = "ANNUAL"
)

// CostCenter represents a cost center for expense tracking
type CostCenter struct {
	ID           string           `json:"id"`
	TenantID     string           `json:"tenant_id"`
	Code         string           `json:"code"`
	Name         string           `json:"name"`
	Description  string           `json:"description,omitempty"`
	ParentID     *string          `json:"parent_id,omitempty"`
	IsActive     bool             `json:"is_active"`
	BudgetAmount *decimal.Decimal `json:"budget_amount,omitempty"`
	BudgetPeriod BudgetPeriod     `json:"budget_period"`
	CreatedAt    time.Time        `json:"created_at"`
	UpdatedAt    time.Time        `json:"updated_at"`
	// Computed fields for reports
	Children   []CostCenter     `json:"children,omitempty"`
	TotalSpent *decimal.Decimal `json:"total_spent,omitempty"`
	BudgetUsed *decimal.Decimal `json:"budget_used_percentage,omitempty"`
}

// CostAllocation tracks expense allocations to cost centers
type CostAllocation struct {
	ID                   string           `json:"id"`
	TenantID             string           `json:"tenant_id"`
	CostCenterID         string           `json:"cost_center_id"`
	JournalEntryLineID   string           `json:"journal_entry_line_id"`
	Amount               decimal.Decimal  `json:"amount"`
	AllocationPercentage *decimal.Decimal `json:"allocation_percentage,omitempty"`
	AllocationDate       time.Time        `json:"allocation_date"`
	Notes                string           `json:"notes,omitempty"`
	CreatedAt            time.Time        `json:"created_at"`
	// Joined fields
	CostCenterCode string `json:"cost_center_code,omitempty"`
	CostCenterName string `json:"cost_center_name,omitempty"`
}

// CreateCostCenterRequest is the request to create a cost center
type CreateCostCenterRequest struct {
	Code         string           `json:"code"`
	Name         string           `json:"name"`
	Description  string           `json:"description,omitempty"`
	ParentID     *string          `json:"parent_id,omitempty"`
	IsActive     bool             `json:"is_active"`
	BudgetAmount *decimal.Decimal `json:"budget_amount,omitempty"`
	BudgetPeriod BudgetPeriod     `json:"budget_period,omitempty"`
}

// UpdateCostCenterRequest is the request to update a cost center
type UpdateCostCenterRequest struct {
	Code         string           `json:"code"`
	Name         string           `json:"name"`
	Description  string           `json:"description,omitempty"`
	ParentID     *string          `json:"parent_id,omitempty"`
	IsActive     bool             `json:"is_active"`
	BudgetAmount *decimal.Decimal `json:"budget_amount,omitempty"`
	BudgetPeriod BudgetPeriod     `json:"budget_period,omitempty"`
}

// CostCenterSummary provides expense summary for a cost center
type CostCenterSummary struct {
	CostCenter    CostCenter      `json:"cost_center"`
	TotalExpenses decimal.Decimal `json:"total_expenses"`
	BudgetAmount  decimal.Decimal `json:"budget_amount"`
	BudgetUsed    decimal.Decimal `json:"budget_used_percentage"`
	IsOverBudget  bool            `json:"is_over_budget"`
	PeriodStart   time.Time       `json:"period_start"`
	PeriodEnd     time.Time       `json:"period_end"`
}

// CostCenterReport is a full report across all cost centers
type CostCenterReport struct {
	TenantID      string              `json:"tenant_id"`
	PeriodStart   time.Time           `json:"period_start"`
	PeriodEnd     time.Time           `json:"period_end"`
	GeneratedAt   time.Time           `json:"generated_at"`
	CostCenters   []CostCenterSummary `json:"cost_centers"`
	TotalExpenses decimal.Decimal     `json:"total_expenses"`
	TotalBudget   decimal.Decimal     `json:"total_budget"`
}

// CostCenterRepository defines the interface for cost center data access
type CostCenterRepository interface {
	GetByID(ctx context.Context, schemaName, tenantID, costCenterID string) (*CostCenter, error)
	List(ctx context.Context, schemaName, tenantID string, activeOnly bool) ([]CostCenter, error)
	Create(ctx context.Context, schemaName string, cc *CostCenter) error
	Update(ctx context.Context, schemaName string, cc *CostCenter) error
	Delete(ctx context.Context, schemaName, tenantID, costCenterID string) error
	GetExpensesByPeriod(ctx context.Context, schemaName, tenantID, costCenterID string, start, end time.Time) (decimal.Decimal, error)
}

// CostCenterPostgresRepository implements CostCenterRepository for PostgreSQL
type CostCenterPostgresRepository struct {
	db *pgxpool.Pool
}

// NewCostCenterRepository creates a new cost center repository
func NewCostCenterRepository(db *pgxpool.Pool) *CostCenterPostgresRepository {
	return &CostCenterPostgresRepository{db: db}
}

// ensureCostCenterTables creates tables if they don't exist in the tenant schema
func (r *CostCenterPostgresRepository) ensureCostCenterTables(ctx context.Context, schemaName string) error {
	// Create cost_centers table
	_, err := r.db.Exec(ctx, fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s.cost_centers (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			tenant_id UUID NOT NULL,
			code VARCHAR(20) NOT NULL,
			name VARCHAR(200) NOT NULL,
			description TEXT,
			parent_id UUID,
			is_active BOOLEAN NOT NULL DEFAULT true,
			budget_amount DECIMAL(15,2),
			budget_period VARCHAR(20) DEFAULT 'ANNUAL',
			created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
			UNIQUE(tenant_id, code)
		)
	`, schemaName))
	if err != nil {
		return fmt.Errorf("create cost_centers table: %w", err)
	}

	// Create cost_allocations table
	_, err = r.db.Exec(ctx, fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s.cost_allocations (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			tenant_id UUID NOT NULL,
			cost_center_id UUID NOT NULL,
			journal_entry_line_id UUID NOT NULL,
			amount DECIMAL(15,2) NOT NULL,
			allocation_percentage DECIMAL(5,2),
			allocation_date DATE NOT NULL,
			notes TEXT,
			created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
		)
	`, schemaName))
	if err != nil {
		return fmt.Errorf("create cost_allocations table: %w", err)
	}

	// Create indexes
	_, _ = r.db.Exec(ctx, fmt.Sprintf(`CREATE INDEX IF NOT EXISTS idx_cost_centers_tenant ON %s.cost_centers(tenant_id)`, schemaName))
	_, _ = r.db.Exec(ctx, fmt.Sprintf(`CREATE INDEX IF NOT EXISTS idx_cost_centers_active ON %s.cost_centers(tenant_id, is_active)`, schemaName))
	_, _ = r.db.Exec(ctx, fmt.Sprintf(`CREATE INDEX IF NOT EXISTS idx_cost_allocations_center ON %s.cost_allocations(cost_center_id)`, schemaName))
	_, _ = r.db.Exec(ctx, fmt.Sprintf(`CREATE INDEX IF NOT EXISTS idx_cost_allocations_date ON %s.cost_allocations(allocation_date)`, schemaName))

	return nil
}

// GetByID retrieves a cost center by ID
func (r *CostCenterPostgresRepository) GetByID(ctx context.Context, schemaName, tenantID, costCenterID string) (*CostCenter, error) {
	if err := r.ensureCostCenterTables(ctx, schemaName); err != nil {
		return nil, err
	}

	var cc CostCenter
	var budgetAmount *decimal.Decimal
	err := r.db.QueryRow(ctx, fmt.Sprintf(`
		SELECT id, tenant_id, code, name, COALESCE(description, ''), parent_id, is_active,
		       budget_amount, COALESCE(budget_period, 'ANNUAL'), created_at, updated_at
		FROM %s.cost_centers
		WHERE id = $1 AND tenant_id = $2
	`, schemaName), costCenterID, tenantID).Scan(
		&cc.ID, &cc.TenantID, &cc.Code, &cc.Name, &cc.Description, &cc.ParentID,
		&cc.IsActive, &budgetAmount, &cc.BudgetPeriod, &cc.CreatedAt, &cc.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("cost center not found: %s", costCenterID)
	}
	if err != nil {
		return nil, fmt.Errorf("get cost center: %w", err)
	}
	cc.BudgetAmount = budgetAmount
	return &cc, nil
}

// List retrieves all cost centers for a tenant
func (r *CostCenterPostgresRepository) List(ctx context.Context, schemaName, tenantID string, activeOnly bool) ([]CostCenter, error) {
	if err := r.ensureCostCenterTables(ctx, schemaName); err != nil {
		return nil, err
	}

	query := fmt.Sprintf(`
		SELECT id, tenant_id, code, name, COALESCE(description, ''), parent_id, is_active,
		       budget_amount, COALESCE(budget_period, 'ANNUAL'), created_at, updated_at
		FROM %s.cost_centers
		WHERE tenant_id = $1
	`, schemaName)
	if activeOnly {
		query += " AND is_active = true"
	}
	query += " ORDER BY code"

	rows, err := r.db.Query(ctx, query, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list cost centers: %w", err)
	}
	defer rows.Close()

	var costCenters []CostCenter
	for rows.Next() {
		var cc CostCenter
		var budgetAmount *decimal.Decimal
		if err := rows.Scan(
			&cc.ID, &cc.TenantID, &cc.Code, &cc.Name, &cc.Description, &cc.ParentID,
			&cc.IsActive, &budgetAmount, &cc.BudgetPeriod, &cc.CreatedAt, &cc.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan cost center: %w", err)
		}
		cc.BudgetAmount = budgetAmount
		costCenters = append(costCenters, cc)
	}
	return costCenters, nil
}

// Create creates a new cost center
func (r *CostCenterPostgresRepository) Create(ctx context.Context, schemaName string, cc *CostCenter) error {
	if err := r.ensureCostCenterTables(ctx, schemaName); err != nil {
		return err
	}

	if cc.ID == "" {
		cc.ID = uuid.New().String()
	}
	now := time.Now()
	cc.CreatedAt = now
	cc.UpdatedAt = now

	if cc.BudgetPeriod == "" {
		cc.BudgetPeriod = BudgetPeriodAnnual
	}

	_, err := r.db.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.cost_centers (id, tenant_id, code, name, description, parent_id, is_active, budget_amount, budget_period, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`, schemaName),
		cc.ID, cc.TenantID, cc.Code, cc.Name, cc.Description, cc.ParentID,
		cc.IsActive, cc.BudgetAmount, cc.BudgetPeriod, cc.CreatedAt, cc.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("create cost center: %w", err)
	}
	return nil
}

// Update updates an existing cost center
func (r *CostCenterPostgresRepository) Update(ctx context.Context, schemaName string, cc *CostCenter) error {
	cc.UpdatedAt = time.Now()

	result, err := r.db.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.cost_centers
		SET code = $1, name = $2, description = $3, parent_id = $4, is_active = $5,
		    budget_amount = $6, budget_period = $7, updated_at = $8
		WHERE id = $9 AND tenant_id = $10
	`, schemaName),
		cc.Code, cc.Name, cc.Description, cc.ParentID, cc.IsActive,
		cc.BudgetAmount, cc.BudgetPeriod, cc.UpdatedAt, cc.ID, cc.TenantID,
	)
	if err != nil {
		return fmt.Errorf("update cost center: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("cost center not found: %s", cc.ID)
	}
	return nil
}

// Delete deletes a cost center
func (r *CostCenterPostgresRepository) Delete(ctx context.Context, schemaName, tenantID, costCenterID string) error {
	// First check if there are any child cost centers
	var childCount int
	err := r.db.QueryRow(ctx, fmt.Sprintf(`
		SELECT COUNT(*) FROM %s.cost_centers WHERE parent_id = $1 AND tenant_id = $2
	`, schemaName), costCenterID, tenantID).Scan(&childCount)
	if err != nil {
		return fmt.Errorf("check children: %w", err)
	}
	if childCount > 0 {
		return fmt.Errorf("cannot delete cost center with %d children", childCount)
	}

	// Check for allocations
	var allocationCount int
	err = r.db.QueryRow(ctx, fmt.Sprintf(`
		SELECT COUNT(*) FROM %s.cost_allocations WHERE cost_center_id = $1 AND tenant_id = $2
	`, schemaName), costCenterID, tenantID).Scan(&allocationCount)
	if err != nil {
		return fmt.Errorf("check allocations: %w", err)
	}
	if allocationCount > 0 {
		return fmt.Errorf("cannot delete cost center with %d allocations", allocationCount)
	}

	result, err := r.db.Exec(ctx, fmt.Sprintf(`
		DELETE FROM %s.cost_centers WHERE id = $1 AND tenant_id = $2
	`, schemaName), costCenterID, tenantID)
	if err != nil {
		return fmt.Errorf("delete cost center: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("cost center not found: %s", costCenterID)
	}
	return nil
}

// GetExpensesByPeriod gets total expenses for a cost center in a period
func (r *CostCenterPostgresRepository) GetExpensesByPeriod(ctx context.Context, schemaName, tenantID, costCenterID string, start, end time.Time) (decimal.Decimal, error) {
	var total decimal.Decimal
	err := r.db.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(SUM(amount), 0)
		FROM %s.cost_allocations
		WHERE cost_center_id = $1 AND tenant_id = $2
		  AND allocation_date >= $3 AND allocation_date <= $4
	`, schemaName), costCenterID, tenantID, start, end).Scan(&total)
	if err != nil {
		return decimal.Zero, fmt.Errorf("get expenses: %w", err)
	}
	return total, nil
}

// CostCenterService provides business logic for cost centers
type CostCenterService struct {
	db   *pgxpool.Pool
	repo CostCenterRepository
}

// NewCostCenterService creates a new cost center service
func NewCostCenterService(db *pgxpool.Pool) *CostCenterService {
	return &CostCenterService{
		db:   db,
		repo: NewCostCenterRepository(db),
	}
}

// GetCostCenter retrieves a cost center by ID
func (s *CostCenterService) GetCostCenter(ctx context.Context, schemaName, tenantID, costCenterID string) (*CostCenter, error) {
	return s.repo.GetByID(ctx, schemaName, tenantID, costCenterID)
}

// ListCostCenters retrieves all cost centers for a tenant
func (s *CostCenterService) ListCostCenters(ctx context.Context, schemaName, tenantID string, activeOnly bool) ([]CostCenter, error) {
	return s.repo.List(ctx, schemaName, tenantID, activeOnly)
}

// CreateCostCenter creates a new cost center
func (s *CostCenterService) CreateCostCenter(ctx context.Context, schemaName, tenantID string, req *CreateCostCenterRequest) (*CostCenter, error) {
	if req.Code == "" {
		return nil, fmt.Errorf("cost center code is required")
	}
	if req.Name == "" {
		return nil, fmt.Errorf("cost center name is required")
	}

	cc := &CostCenter{
		TenantID:     tenantID,
		Code:         req.Code,
		Name:         req.Name,
		Description:  req.Description,
		ParentID:     req.ParentID,
		IsActive:     req.IsActive,
		BudgetAmount: req.BudgetAmount,
		BudgetPeriod: req.BudgetPeriod,
	}

	if cc.BudgetPeriod == "" {
		cc.BudgetPeriod = BudgetPeriodAnnual
	}

	if err := s.repo.Create(ctx, schemaName, cc); err != nil {
		return nil, err
	}
	return cc, nil
}

// UpdateCostCenter updates an existing cost center
func (s *CostCenterService) UpdateCostCenter(ctx context.Context, schemaName, tenantID, costCenterID string, req *UpdateCostCenterRequest) (*CostCenter, error) {
	cc, err := s.repo.GetByID(ctx, schemaName, tenantID, costCenterID)
	if err != nil {
		return nil, err
	}

	cc.Code = req.Code
	cc.Name = req.Name
	cc.Description = req.Description
	cc.ParentID = req.ParentID
	cc.IsActive = req.IsActive
	cc.BudgetAmount = req.BudgetAmount
	cc.BudgetPeriod = req.BudgetPeriod

	if err := s.repo.Update(ctx, schemaName, cc); err != nil {
		return nil, err
	}
	return cc, nil
}

// DeleteCostCenter deletes a cost center
func (s *CostCenterService) DeleteCostCenter(ctx context.Context, schemaName, tenantID, costCenterID string) error {
	return s.repo.Delete(ctx, schemaName, tenantID, costCenterID)
}

// GetCostCenterReport generates a report for all cost centers
func (s *CostCenterService) GetCostCenterReport(ctx context.Context, schemaName, tenantID string, start, end time.Time) (*CostCenterReport, error) {
	costCenters, err := s.repo.List(ctx, schemaName, tenantID, true)
	if err != nil {
		return nil, err
	}

	report := &CostCenterReport{
		TenantID:      tenantID,
		PeriodStart:   start,
		PeriodEnd:     end,
		GeneratedAt:   time.Now(),
		CostCenters:   make([]CostCenterSummary, 0, len(costCenters)),
		TotalExpenses: decimal.Zero,
		TotalBudget:   decimal.Zero,
	}

	for _, cc := range costCenters {
		expenses, err := s.repo.GetExpensesByPeriod(ctx, schemaName, tenantID, cc.ID, start, end)
		if err != nil {
			return nil, err
		}

		budget := decimal.Zero
		if cc.BudgetAmount != nil {
			budget = *cc.BudgetAmount
		}

		budgetUsed := decimal.Zero
		isOverBudget := false
		if budget.GreaterThan(decimal.Zero) {
			budgetUsed = expenses.Div(budget).Mul(decimal.NewFromInt(100))
			isOverBudget = expenses.GreaterThan(budget)
		}

		summary := CostCenterSummary{
			CostCenter:    cc,
			TotalExpenses: expenses,
			BudgetAmount:  budget,
			BudgetUsed:    budgetUsed,
			IsOverBudget:  isOverBudget,
			PeriodStart:   start,
			PeriodEnd:     end,
		}
		report.CostCenters = append(report.CostCenters, summary)
		report.TotalExpenses = report.TotalExpenses.Add(expenses)
		report.TotalBudget = report.TotalBudget.Add(budget)
	}

	return report, nil
}

// MockCostCenterRepository is a mock implementation for testing
type MockCostCenterRepository struct {
	CostCenters map[string]*CostCenter
	Allocations map[string][]CostAllocation
}

// NewMockCostCenterRepository creates a new mock repository
func NewMockCostCenterRepository() *MockCostCenterRepository {
	return &MockCostCenterRepository{
		CostCenters: make(map[string]*CostCenter),
		Allocations: make(map[string][]CostAllocation),
	}
}

// GetByID mock implementation
func (m *MockCostCenterRepository) GetByID(_ context.Context, _, tenantID, costCenterID string) (*CostCenter, error) {
	if cc, ok := m.CostCenters[costCenterID]; ok && cc.TenantID == tenantID {
		return cc, nil
	}
	return nil, fmt.Errorf("cost center not found: %s", costCenterID)
}

// List mock implementation
func (m *MockCostCenterRepository) List(_ context.Context, _, tenantID string, activeOnly bool) ([]CostCenter, error) {
	var result []CostCenter
	for _, cc := range m.CostCenters {
		if cc.TenantID == tenantID {
			if activeOnly && !cc.IsActive {
				continue
			}
			result = append(result, *cc)
		}
	}
	return result, nil
}

// Create mock implementation
func (m *MockCostCenterRepository) Create(_ context.Context, _ string, cc *CostCenter) error {
	if cc.ID == "" {
		cc.ID = uuid.New().String()
	}
	m.CostCenters[cc.ID] = cc
	return nil
}

// Update mock implementation
func (m *MockCostCenterRepository) Update(_ context.Context, _ string, cc *CostCenter) error {
	if _, ok := m.CostCenters[cc.ID]; !ok {
		return fmt.Errorf("cost center not found: %s", cc.ID)
	}
	m.CostCenters[cc.ID] = cc
	return nil
}

// Delete mock implementation
func (m *MockCostCenterRepository) Delete(_ context.Context, _, tenantID, costCenterID string) error {
	if cc, ok := m.CostCenters[costCenterID]; ok && cc.TenantID == tenantID {
		delete(m.CostCenters, costCenterID)
		return nil
	}
	return fmt.Errorf("cost center not found: %s", costCenterID)
}

// GetExpensesByPeriod mock implementation
func (m *MockCostCenterRepository) GetExpensesByPeriod(_ context.Context, _, tenantID, costCenterID string, start, end time.Time) (decimal.Decimal, error) {
	total := decimal.Zero
	if allocs, ok := m.Allocations[costCenterID]; ok {
		for _, a := range allocs {
			if a.TenantID == tenantID && !a.AllocationDate.Before(start) && !a.AllocationDate.After(end) {
				total = total.Add(a.Amount)
			}
		}
	}
	return total, nil
}
