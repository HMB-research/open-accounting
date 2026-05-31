package accounting

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/HMB-research/open-accounting/internal/database"
	"github.com/HMB-research/open-accounting/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
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

// ImportCostCentersRequest contains CSV payload for cost center migration.
type ImportCostCentersRequest struct {
	CSVContent string `json:"csv_content"`
	FileName   string `json:"file_name,omitempty"`
}

// ImportCostCentersResult summarizes a cost center CSV import.
type ImportCostCentersResult struct {
	FileName           string                      `json:"file_name,omitempty"`
	RowsProcessed      int                         `json:"rows_processed"`
	CostCentersCreated int                         `json:"cost_centers_created"`
	RowsSkipped        int                         `json:"rows_skipped"`
	Errors             []ImportCostCentersRowError `json:"errors,omitempty"`
}

// ImportCostCentersRowError describes a row-level cost center import failure.
type ImportCostCentersRowError struct {
	Row     int    `json:"row"`
	Code    string `json:"code,omitempty"`
	Name    string `json:"name,omitempty"`
	Message string `json:"message"`
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

// CostCenterGORMRepository implements CostCenterRepository with the shared ORM layer.
type CostCenterGORMRepository struct {
	db *gorm.DB
}

// NewCostCenterRepository creates a new ORM-backed cost center repository.
func NewCostCenterRepository(db *pgxpool.Pool) *CostCenterGORMRepository {
	if db == nil {
		return &CostCenterGORMRepository{}
	}
	gormDB, err := database.NewGormDBFromPool(context.Background(), db)
	if err != nil {
		panic(fmt.Errorf("create cost center GORM repository: %w", err))
	}
	return NewCostCenterGORMRepository(gormDB)
}

func NewCostCenterGORMRepository(db *gorm.DB) *CostCenterGORMRepository {
	return &CostCenterGORMRepository{db: db}
}

func (r *CostCenterGORMRepository) tenantTable(ctx context.Context, schemaName, tableName string) (*gorm.DB, error) {
	if r.db == nil {
		return nil, fmt.Errorf("cost center repository database is not configured")
	}
	return database.TenantTable(r.db.WithContext(ctx), schemaName, tableName)
}

// GetByID retrieves a cost center by ID
func (r *CostCenterGORMRepository) GetByID(ctx context.Context, schemaName, tenantID, costCenterID string) (*CostCenter, error) {
	db, err := r.tenantTable(ctx, schemaName, "cost_centers")
	if err != nil {
		return nil, fmt.Errorf("qualify cost centers table: %w", err)
	}

	var ccModel models.CostCenter
	err = db.Where("id = ? AND tenant_id = ?", costCenterID, tenantID).First(&ccModel).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("cost center not found: %s", costCenterID)
	}
	if err != nil {
		return nil, fmt.Errorf("get cost center: %w", err)
	}
	return costCenterFromModel(&ccModel), nil
}

// List retrieves all cost centers for a tenant
func (r *CostCenterGORMRepository) List(ctx context.Context, schemaName, tenantID string, activeOnly bool) ([]CostCenter, error) {
	db, err := r.tenantTable(ctx, schemaName, "cost_centers")
	if err != nil {
		return nil, fmt.Errorf("qualify cost centers table: %w", err)
	}

	query := db.Where("tenant_id = ?", tenantID)
	if activeOnly {
		query = query.Where("is_active = ?", true)
	}

	var ccModels []models.CostCenter
	if err := query.Order("code ASC").Find(&ccModels).Error; err != nil {
		return nil, fmt.Errorf("list cost centers: %w", err)
	}

	costCenters := make([]CostCenter, len(ccModels))
	for i := range ccModels {
		costCenters[i] = *costCenterFromModel(&ccModels[i])
	}
	return costCenters, nil
}

// Create creates a new cost center
func (r *CostCenterGORMRepository) Create(ctx context.Context, schemaName string, cc *CostCenter) error {
	if cc.ID == "" {
		cc.ID = uuid.New().String()
	}
	now := time.Now()
	cc.CreatedAt = now
	cc.UpdatedAt = now

	if cc.BudgetPeriod == "" {
		cc.BudgetPeriod = BudgetPeriodAnnual
	}

	db, err := r.tenantTable(ctx, schemaName, "cost_centers")
	if err != nil {
		return fmt.Errorf("qualify cost centers table: %w", err)
	}
	if err := db.Create(costCenterToModel(cc)).Error; err != nil {
		return fmt.Errorf("create cost center: %w", err)
	}
	return nil
}

// Update updates an existing cost center
func (r *CostCenterGORMRepository) Update(ctx context.Context, schemaName string, cc *CostCenter) error {
	cc.UpdatedAt = time.Now()

	db, err := r.tenantTable(ctx, schemaName, "cost_centers")
	if err != nil {
		return fmt.Errorf("qualify cost centers table: %w", err)
	}
	result := db.Where("id = ? AND tenant_id = ?", cc.ID, cc.TenantID).
		Updates(map[string]interface{}{
			"code":          cc.Code,
			"name":          cc.Name,
			"description":   cc.Description,
			"parent_id":     cc.ParentID,
			"is_active":     cc.IsActive,
			"budget_amount": costCenterBudgetAmountToModel(cc.BudgetAmount),
			"budget_period": string(cc.BudgetPeriod),
			"updated_at":    cc.UpdatedAt,
		})
	if result.Error != nil {
		return fmt.Errorf("update cost center: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("cost center not found: %s", cc.ID)
	}
	return nil
}

// Delete deletes a cost center
func (r *CostCenterGORMRepository) Delete(ctx context.Context, schemaName, tenantID, costCenterID string) error {
	childrenTable, err := r.tenantTable(ctx, schemaName, "cost_centers")
	if err != nil {
		return fmt.Errorf("qualify cost centers table: %w", err)
	}

	// First check if there are any child cost centers
	var childCount int64
	if err := childrenTable.Model(&models.CostCenter{}).
		Where("parent_id = ? AND tenant_id = ?", costCenterID, tenantID).
		Count(&childCount).Error; err != nil {
		return fmt.Errorf("check children: %w", err)
	}
	if childCount > 0 {
		return fmt.Errorf("cannot delete cost center with %d children", childCount)
	}

	allocationsTable, err := r.tenantTable(ctx, schemaName, "cost_allocations")
	if err != nil {
		return fmt.Errorf("qualify cost allocations table: %w", err)
	}

	// Check for allocations
	var allocationCount int64
	if err := allocationsTable.Model(&models.CostAllocation{}).
		Where("cost_center_id = ? AND tenant_id = ?", costCenterID, tenantID).
		Count(&allocationCount).Error; err != nil {
		return fmt.Errorf("check allocations: %w", err)
	}
	if allocationCount > 0 {
		return fmt.Errorf("cannot delete cost center with %d allocations", allocationCount)
	}

	costCentersTable, err := r.tenantTable(ctx, schemaName, "cost_centers")
	if err != nil {
		return fmt.Errorf("qualify cost centers table: %w", err)
	}
	result := costCentersTable.Where("id = ? AND tenant_id = ?", costCenterID, tenantID).Delete(&models.CostCenter{})
	if result.Error != nil {
		return fmt.Errorf("delete cost center: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("cost center not found: %s", costCenterID)
	}
	return nil
}

// GetExpensesByPeriod gets total expenses for a cost center in a period
func (r *CostCenterGORMRepository) GetExpensesByPeriod(ctx context.Context, schemaName, tenantID, costCenterID string, start, end time.Time) (decimal.Decimal, error) {
	db, err := r.tenantTable(ctx, schemaName, "cost_allocations")
	if err != nil {
		return decimal.Zero, fmt.Errorf("qualify cost allocations table: %w", err)
	}

	var row struct {
		Total models.Decimal
	}
	if err := db.
		Select("COALESCE(SUM(amount), 0) AS total").
		Where("cost_center_id = ? AND tenant_id = ?", costCenterID, tenantID).
		Where("allocation_date >= ? AND allocation_date <= ?", start, end).
		Scan(&row).Error; err != nil {
		return decimal.Zero, fmt.Errorf("get expenses: %w", err)
	}
	return row.Total.Decimal, nil
}

func costCenterToModel(cc *CostCenter) *models.CostCenter {
	return &models.CostCenter{
		ID:           cc.ID,
		TenantID:     cc.TenantID,
		Code:         cc.Code,
		Name:         cc.Name,
		Description:  cc.Description,
		ParentID:     cc.ParentID,
		IsActive:     cc.IsActive,
		BudgetAmount: costCenterBudgetAmountToModel(cc.BudgetAmount),
		BudgetPeriod: string(cc.BudgetPeriod),
		CreatedAt:    cc.CreatedAt,
		UpdatedAt:    cc.UpdatedAt,
	}
}

func costCenterFromModel(cc *models.CostCenter) *CostCenter {
	return &CostCenter{
		ID:           cc.ID,
		TenantID:     cc.TenantID,
		Code:         cc.Code,
		Name:         cc.Name,
		Description:  cc.Description,
		ParentID:     cc.ParentID,
		IsActive:     cc.IsActive,
		BudgetAmount: costCenterBudgetAmountFromModel(cc.BudgetAmount),
		BudgetPeriod: BudgetPeriod(cc.BudgetPeriod),
		CreatedAt:    cc.CreatedAt,
		UpdatedAt:    cc.UpdatedAt,
	}
}

func costCenterBudgetAmountToModel(amount *decimal.Decimal) *models.Decimal {
	if amount == nil {
		return nil
	}
	value := models.Decimal{Decimal: *amount}
	return &value
}

func costCenterBudgetAmountFromModel(amount *models.Decimal) *decimal.Decimal {
	if amount == nil {
		return nil
	}
	value := amount.Decimal
	return &value
}

// CostCenterService provides business logic for cost centers
type CostCenterService struct {
	repo CostCenterRepository
}

// NewCostCenterService creates a new cost center service with an ORM-backed repository.
func NewCostCenterService(db *pgxpool.Pool) *CostCenterService {
	return &CostCenterService{
		repo: NewCostCenterRepository(db),
	}
}

// NewCostCenterServiceWithRepository creates a new cost center service with a custom repository.
func NewCostCenterServiceWithRepository(repo CostCenterRepository) *CostCenterService {
	return &CostCenterService{repo: repo}
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
	result := []CostCenter{}
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
