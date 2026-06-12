package accounting

import (
	"context"
	"errors"
	"fmt"
	"strings"
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

// ImportCostAllocationsRequest contains CSV payload for cost allocation migration.
type ImportCostAllocationsRequest struct {
	CSVContent string `json:"csv_content"`
	FileName   string `json:"file_name,omitempty"`
}

// ImportCostAllocationsResult summarizes a cost allocation CSV import.
type ImportCostAllocationsResult struct {
	FileName            string                          `json:"file_name,omitempty"`
	RowsProcessed       int                             `json:"rows_processed"`
	AllocationsImported int                             `json:"allocations_imported"`
	RowsSkipped         int                             `json:"rows_skipped"`
	Errors              []ImportCostAllocationsRowError `json:"errors,omitempty"`
}

// ImportCostAllocationsRowError describes a row-level cost allocation import failure.
type ImportCostAllocationsRowError struct {
	Row                int    `json:"row"`
	CostCenterID       string `json:"cost_center_id,omitempty"`
	CostCenterCode     string `json:"cost_center_code,omitempty"`
	JournalEntryLineID string `json:"journal_entry_line_id,omitempty"`
	Message            string `json:"message"`
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

// CreateCostAllocationRequest assigns a journal entry line amount to a cost center.
type CreateCostAllocationRequest struct {
	CostCenterID         string           `json:"cost_center_id"`
	JournalEntryLineID   string           `json:"journal_entry_line_id"`
	Amount               decimal.Decimal  `json:"amount"`
	AllocationPercentage *decimal.Decimal `json:"allocation_percentage,omitempty"`
	AllocationDate       time.Time        `json:"allocation_date"`
	Notes                string           `json:"notes,omitempty"`
}

// CostAllocationFilters filters cost allocations for review and reporting.
type CostAllocationFilters struct {
	CostCenterID       string
	JournalEntryLineID string
	StartDate          *time.Time
	EndDate            *time.Time
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
	CreateAllocation(ctx context.Context, schemaName string, allocation *CostAllocation) error
	ListAllocations(ctx context.Context, schemaName, tenantID string, filters CostAllocationFilters) ([]CostAllocation, error)
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

// CreateAllocation creates a cost allocation row.
func (r *CostCenterGORMRepository) CreateAllocation(ctx context.Context, schemaName string, allocation *CostAllocation) error {
	if allocation.ID == "" {
		allocation.ID = uuid.New().String()
	}
	allocation.CreatedAt = time.Now()

	db, err := r.tenantTable(ctx, schemaName, "cost_allocations")
	if err != nil {
		return fmt.Errorf("qualify cost allocations table: %w", err)
	}
	if err := db.Create(costAllocationToModel(allocation)).Error; err != nil {
		return fmt.Errorf("create cost allocation: %w", err)
	}
	return nil
}

// ListAllocations lists cost allocations with optional cost center and date filters.
func (r *CostCenterGORMRepository) ListAllocations(ctx context.Context, schemaName, tenantID string, filters CostAllocationFilters) ([]CostAllocation, error) {
	allocationsTable, err := r.tenantTable(ctx, schemaName, "cost_allocations")
	if err != nil {
		return nil, fmt.Errorf("qualify cost allocations table: %w", err)
	}
	costCentersTable, err := database.QualifiedTable(schemaName, "cost_centers")
	if err != nil {
		return nil, fmt.Errorf("qualify cost centers table: %w", err)
	}

	query := allocationsTable.
		Select("cost_allocations.*, cost_centers.code AS cost_center_code, cost_centers.name AS cost_center_name").
		Joins("LEFT JOIN "+costCentersTable+" AS cost_centers ON cost_centers.id = cost_allocations.cost_center_id AND cost_centers.tenant_id = cost_allocations.tenant_id").
		Where("cost_allocations.tenant_id = ?", tenantID)
	if strings.TrimSpace(filters.CostCenterID) != "" {
		query = query.Where("cost_allocations.cost_center_id = ?", strings.TrimSpace(filters.CostCenterID))
	}
	if strings.TrimSpace(filters.JournalEntryLineID) != "" {
		query = query.Where("cost_allocations.journal_entry_line_id = ?", strings.TrimSpace(filters.JournalEntryLineID))
	}
	if filters.StartDate != nil {
		query = query.Where("cost_allocations.allocation_date >= ?", *filters.StartDate)
	}
	if filters.EndDate != nil {
		query = query.Where("cost_allocations.allocation_date <= ?", *filters.EndDate)
	}

	var rows []struct {
		models.CostAllocation
		CostCenterCode string
		CostCenterName string
	}
	if err := query.Order("cost_allocations.allocation_date DESC, cost_allocations.created_at DESC").Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("list cost allocations: %w", err)
	}

	allocations := make([]CostAllocation, len(rows))
	for i := range rows {
		allocations[i] = *costAllocationFromModel(&rows[i].CostAllocation)
		allocations[i].CostCenterCode = rows[i].CostCenterCode
		allocations[i].CostCenterName = rows[i].CostCenterName
	}
	return allocations, nil
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

func costAllocationToModel(allocation *CostAllocation) *models.CostAllocation {
	return &models.CostAllocation{
		ID:                   allocation.ID,
		TenantID:             allocation.TenantID,
		CostCenterID:         allocation.CostCenterID,
		JournalEntryLineID:   allocation.JournalEntryLineID,
		Amount:               models.NewDecimal(allocation.Amount),
		AllocationPercentage: costCenterBudgetAmountToModel(allocation.AllocationPercentage),
		AllocationDate:       allocation.AllocationDate,
		Notes:                allocation.Notes,
		CreatedAt:            allocation.CreatedAt,
	}
}

func costAllocationFromModel(allocation *models.CostAllocation) *CostAllocation {
	return &CostAllocation{
		ID:                   allocation.ID,
		TenantID:             allocation.TenantID,
		CostCenterID:         allocation.CostCenterID,
		JournalEntryLineID:   allocation.JournalEntryLineID,
		Amount:               allocation.Amount.Decimal,
		AllocationPercentage: costCenterBudgetAmountFromModel(allocation.AllocationPercentage),
		AllocationDate:       allocation.AllocationDate,
		Notes:                allocation.Notes,
		CreatedAt:            allocation.CreatedAt,
	}
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
	parentID, err := normalizeOptionalCostCenterUUIDPtr(req.ParentID, "parent_id")
	if err != nil {
		return nil, err
	}

	cc := &CostCenter{
		TenantID:     tenantID,
		Code:         req.Code,
		Name:         req.Name,
		Description:  req.Description,
		ParentID:     parentID,
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
	parentID, err := normalizeOptionalCostCenterUUIDPtr(req.ParentID, "parent_id")
	if err != nil {
		return nil, err
	}

	cc.Code = req.Code
	cc.Name = req.Name
	cc.Description = req.Description
	cc.ParentID = parentID
	cc.IsActive = req.IsActive
	cc.BudgetAmount = req.BudgetAmount
	cc.BudgetPeriod = req.BudgetPeriod

	if err := s.repo.Update(ctx, schemaName, cc); err != nil {
		return nil, err
	}
	return cc, nil
}

func normalizeOptionalCostCenterUUIDPtr(value *string, field string) (*string, error) {
	if value == nil {
		return nil, nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil, nil
	}
	parsedID, err := uuid.Parse(trimmed)
	if err != nil {
		return nil, fmt.Errorf("%s must be a valid UUID", field)
	}
	id := parsedID.String()
	return &id, nil
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

// CreateCostAllocation assigns a journal entry line amount to a cost center.
func (s *CostCenterService) CreateCostAllocation(ctx context.Context, schemaName, tenantID string, req *CreateCostAllocationRequest) (*CostAllocation, error) {
	costCenterID := strings.TrimSpace(req.CostCenterID)
	if costCenterID == "" {
		return nil, fmt.Errorf("cost_center_id is required")
	}
	journalEntryLineID := strings.TrimSpace(req.JournalEntryLineID)
	if journalEntryLineID == "" {
		return nil, fmt.Errorf("journal_entry_line_id is required")
	}
	if !req.Amount.GreaterThan(decimal.Zero) {
		return nil, fmt.Errorf("amount must be greater than zero")
	}
	if req.AllocationDate.IsZero() {
		return nil, fmt.Errorf("allocation_date is required")
	}
	if req.AllocationPercentage != nil {
		if req.AllocationPercentage.LessThan(decimal.Zero) || req.AllocationPercentage.GreaterThan(decimal.NewFromInt(100)) {
			return nil, fmt.Errorf("allocation_percentage must be between 0 and 100")
		}
	}
	if _, err := s.repo.GetByID(ctx, schemaName, tenantID, costCenterID); err != nil {
		return nil, err
	}

	allocation := &CostAllocation{
		TenantID:             tenantID,
		CostCenterID:         costCenterID,
		JournalEntryLineID:   journalEntryLineID,
		Amount:               req.Amount,
		AllocationPercentage: req.AllocationPercentage,
		AllocationDate:       req.AllocationDate,
		Notes:                strings.TrimSpace(req.Notes),
	}
	if err := s.repo.CreateAllocation(ctx, schemaName, allocation); err != nil {
		return nil, err
	}
	return allocation, nil
}

// ListCostAllocations returns cost-center allocations for review and automation.
func (s *CostCenterService) ListCostAllocations(ctx context.Context, schemaName, tenantID string, filters CostAllocationFilters) ([]CostAllocation, error) {
	if filters.StartDate != nil && filters.EndDate != nil && filters.EndDate.Before(*filters.StartDate) {
		return nil, fmt.Errorf("end_date must be on or after start_date")
	}
	filters.CostCenterID = strings.TrimSpace(filters.CostCenterID)
	filters.JournalEntryLineID = strings.TrimSpace(filters.JournalEntryLineID)
	return s.repo.ListAllocations(ctx, schemaName, tenantID, filters)
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

// CreateAllocation mock implementation
func (m *MockCostCenterRepository) CreateAllocation(_ context.Context, _ string, allocation *CostAllocation) error {
	if allocation.ID == "" {
		allocation.ID = uuid.New().String()
	}
	if allocation.CreatedAt.IsZero() {
		allocation.CreatedAt = time.Now()
	}
	m.Allocations[allocation.CostCenterID] = append(m.Allocations[allocation.CostCenterID], *allocation)
	return nil
}

// ListAllocations mock implementation
func (m *MockCostCenterRepository) ListAllocations(_ context.Context, _, tenantID string, filters CostAllocationFilters) ([]CostAllocation, error) {
	result := []CostAllocation{}
	for _, allocations := range m.Allocations {
		for _, allocation := range allocations {
			if allocation.TenantID != tenantID {
				continue
			}
			if strings.TrimSpace(filters.CostCenterID) != "" && allocation.CostCenterID != strings.TrimSpace(filters.CostCenterID) {
				continue
			}
			if strings.TrimSpace(filters.JournalEntryLineID) != "" && allocation.JournalEntryLineID != strings.TrimSpace(filters.JournalEntryLineID) {
				continue
			}
			if filters.StartDate != nil && allocation.AllocationDate.Before(*filters.StartDate) {
				continue
			}
			if filters.EndDate != nil && allocation.AllocationDate.After(*filters.EndDate) {
				continue
			}
			if costCenter, ok := m.CostCenters[allocation.CostCenterID]; ok {
				allocation.CostCenterCode = costCenter.Code
				allocation.CostCenterName = costCenter.Name
			}
			result = append(result, allocation)
		}
	}
	return result, nil
}
