package payroll

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/HMB-research/open-accounting/internal/database"
	"github.com/HMB-research/open-accounting/internal/models"
	"gorm.io/gorm"
)

// AbsenceGORMRepository implements AbsenceRepository with the shared ORM layer.
type AbsenceGORMRepository struct {
	db *gorm.DB
}

// NewAbsenceGORMRepository creates an ORM-backed absence repository.
func NewAbsenceGORMRepository(db *gorm.DB) *AbsenceGORMRepository {
	return &AbsenceGORMRepository{db: db}
}

func (r *AbsenceGORMRepository) dbWithContext(ctx context.Context) (*gorm.DB, error) {
	if r.db == nil {
		return nil, fmt.Errorf("absence repository database is not configured")
	}
	return r.db.WithContext(ctx), nil
}

func (r *AbsenceGORMRepository) tenantTable(ctx context.Context, schemaName, tableName string) (*gorm.DB, error) {
	db, err := r.dbWithContext(ctx)
	if err != nil {
		return nil, err
	}
	return database.TenantTable(db, schemaName, tableName)
}

func (r *AbsenceGORMRepository) tenantTableName(schemaName, tableName string) (string, error) {
	return database.QualifiedTable(schemaName, tableName)
}

// ListEmployees returns employees for a tenant.
func (r *AbsenceGORMRepository) ListEmployees(ctx context.Context, schemaName, tenantID string, activeOnly bool) ([]Employee, error) {
	if _, err := r.dbWithContext(ctx); err != nil {
		return nil, err
	}
	return NewGORMRepository(r.db).ListEmployees(ctx, schemaName, tenantID, activeOnly)
}

// ListAbsenceTypes returns all absence types for a tenant.
func (r *AbsenceGORMRepository) ListAbsenceTypes(ctx context.Context, schemaName, tenantID string, activeOnly bool) ([]AbsenceType, error) {
	db, err := r.tenantTable(ctx, schemaName, "absence_types")
	if err != nil {
		return nil, err
	}

	query := db.Where("tenant_id = ?", tenantID)
	if activeOnly {
		query = query.Where("is_active = ?", true)
	}

	var typeModels []models.AbsenceType
	if err := query.Order("sort_order, name").Find(&typeModels).Error; err != nil {
		return nil, fmt.Errorf("list absence types: %w", err)
	}

	types := make([]AbsenceType, len(typeModels))
	for i := range typeModels {
		types[i] = *modelToAbsenceType(&typeModels[i])
	}
	return types, nil
}

// GetAbsenceType retrieves an absence type by ID.
func (r *AbsenceGORMRepository) GetAbsenceType(ctx context.Context, schemaName, tenantID, typeID string) (*AbsenceType, error) {
	db, err := r.tenantTable(ctx, schemaName, "absence_types")
	if err != nil {
		return nil, err
	}

	var typeModel models.AbsenceType
	err = db.Where("tenant_id = ? AND id = ?", tenantID, typeID).First(&typeModel).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrAbsenceTypeNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get absence type: %w", err)
	}

	return modelToAbsenceType(&typeModel), nil
}

// GetAbsenceTypeByCode retrieves an absence type by code.
func (r *AbsenceGORMRepository) GetAbsenceTypeByCode(ctx context.Context, schemaName, tenantID, code string) (*AbsenceType, error) {
	db, err := r.tenantTable(ctx, schemaName, "absence_types")
	if err != nil {
		return nil, err
	}

	var typeModel models.AbsenceType
	err = db.Where("tenant_id = ? AND code = ?", tenantID, code).First(&typeModel).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrAbsenceTypeNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get absence type by code: %w", err)
	}

	return modelToAbsenceType(&typeModel), nil
}

// GetLeaveBalance retrieves a specific leave balance.
func (r *AbsenceGORMRepository) GetLeaveBalance(ctx context.Context, schemaName, tenantID, employeeID, absenceTypeID string, year int) (*LeaveBalance, error) {
	db, err := r.tenantTable(ctx, schemaName, "leave_balances")
	if err != nil {
		return nil, err
	}

	var balanceModel models.LeaveBalance
	err = db.Where(
		"tenant_id = ? AND employee_id = ? AND absence_type_id = ? AND year = ?",
		tenantID, employeeID, absenceTypeID, year,
	).First(&balanceModel).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrLeaveBalanceNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get leave balance: %w", err)
	}

	return modelToLeaveBalance(&balanceModel), nil
}

// ListLeaveBalances returns leave balances for an employee.
func (r *AbsenceGORMRepository) ListLeaveBalances(ctx context.Context, schemaName, tenantID, employeeID string, year int) ([]LeaveBalance, error) {
	db, err := r.dbWithContext(ctx)
	if err != nil {
		return nil, err
	}
	leaveBalancesTable, err := r.tenantTableName(schemaName, "leave_balances")
	if err != nil {
		return nil, err
	}
	absenceTypesTable, err := r.tenantTableName(schemaName, "absence_types")
	if err != nil {
		return nil, err
	}

	var rows []leaveBalanceWithAbsenceTypeRow
	if err := db.Table(leaveBalancesTable+" AS lb").
		Select(`
			lb.*,
			at.code AS absence_type_code,
			at.name AS absence_type_name,
			at.name_et AS absence_type_name_et
		`).
		Joins("JOIN "+absenceTypesTable+" AS at ON at.id = lb.absence_type_id").
		Where("lb.tenant_id = ? AND lb.employee_id = ? AND lb.year = ?", tenantID, employeeID, year).
		Order("at.sort_order, at.name").
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("list leave balances: %w", err)
	}

	balances := make([]LeaveBalance, len(rows))
	for i := range rows {
		balance := *modelToLeaveBalance(&rows[i].LeaveBalance)
		balance.AbsenceType = &AbsenceType{
			ID:     balance.AbsenceTypeID,
			Code:   rows[i].AbsenceTypeCode,
			Name:   rows[i].AbsenceTypeName,
			NameET: rows[i].AbsenceTypeNameET,
		}
		balances[i] = balance
	}
	return balances, nil
}

// CreateLeaveBalance inserts a new leave balance.
func (r *AbsenceGORMRepository) CreateLeaveBalance(ctx context.Context, schemaName string, balance *LeaveBalance) error {
	db, err := r.tenantTable(ctx, schemaName, "leave_balances")
	if err != nil {
		return err
	}

	if err := db.Create(leaveBalanceToModel(balance)).Error; err != nil {
		return fmt.Errorf("create leave balance: %w", err)
	}
	return nil
}

// UpdateLeaveBalance updates a leave balance.
func (r *AbsenceGORMRepository) UpdateLeaveBalance(ctx context.Context, schemaName string, balance *LeaveBalance) error {
	db, err := r.tenantTable(ctx, schemaName, "leave_balances")
	if err != nil {
		return err
	}

	query := db.Where("id = ?", balance.ID)
	if balance.TenantID != "" {
		query = query.Where("tenant_id = ?", balance.TenantID)
	}
	result := query.Updates(map[string]interface{}{
		"entitled_days":  models.Decimal{Decimal: balance.EntitledDays},
		"carryover_days": models.Decimal{Decimal: balance.CarryoverDays},
		"used_days":      models.Decimal{Decimal: balance.UsedDays},
		"pending_days":   models.Decimal{Decimal: balance.PendingDays},
		"notes":          stringPtrIfNotBlank(balance.Notes),
		"updated_at":     balance.UpdatedAt,
	})
	if result.Error != nil {
		return fmt.Errorf("update leave balance: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrLeaveBalanceNotFound
	}
	return nil
}

// CreateLeaveRecord inserts a new leave record.
func (r *AbsenceGORMRepository) CreateLeaveRecord(ctx context.Context, schemaName string, record *LeaveRecord) error {
	db, err := r.tenantTable(ctx, schemaName, "leave_records")
	if err != nil {
		return err
	}

	if err := db.Create(leaveRecordToModel(record)).Error; err != nil {
		return fmt.Errorf("create leave record: %w", err)
	}
	return nil
}

// GetLeaveRecord retrieves a leave record by ID.
func (r *AbsenceGORMRepository) GetLeaveRecord(ctx context.Context, schemaName, tenantID, recordID string) (*LeaveRecord, error) {
	db, err := r.tenantTable(ctx, schemaName, "leave_records")
	if err != nil {
		return nil, err
	}

	var recordModel models.LeaveRecord
	err = db.Where("tenant_id = ? AND id = ?", tenantID, recordID).First(&recordModel).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrLeaveRecordNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get leave record: %w", err)
	}

	return modelToLeaveRecord(&recordModel), nil
}

// ListLeaveRecords returns leave records for a tenant/employee.
func (r *AbsenceGORMRepository) ListLeaveRecords(ctx context.Context, schemaName, tenantID, employeeID string, year int) ([]LeaveRecord, error) {
	db, err := r.dbWithContext(ctx)
	if err != nil {
		return nil, err
	}
	leaveRecordsTable, err := r.tenantTableName(schemaName, "leave_records")
	if err != nil {
		return nil, err
	}
	absenceTypesTable, err := r.tenantTableName(schemaName, "absence_types")
	if err != nil {
		return nil, err
	}

	query := db.Table(leaveRecordsTable+" AS lr").
		Select(`
			lr.*,
			at.code AS absence_type_code,
			at.name AS absence_type_name,
			at.name_et AS absence_type_name_et
		`).
		Joins("JOIN "+absenceTypesTable+" AS at ON at.id = lr.absence_type_id").
		Where("lr.tenant_id = ?", tenantID)
	if employeeID != "" {
		query = query.Where("lr.employee_id = ?", employeeID)
	}
	if year > 0 {
		startOfYear := time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC)
		startOfNextYear := time.Date(year+1, 1, 1, 0, 0, 0, 0, time.UTC)
		query = query.Where("lr.start_date >= ? AND lr.start_date < ?", startOfYear, startOfNextYear)
	}

	var rows []leaveRecordWithAbsenceTypeRow
	if err := query.Order("lr.start_date DESC").Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("list leave records: %w", err)
	}

	records := make([]LeaveRecord, len(rows))
	for i := range rows {
		record := *modelToLeaveRecord(&rows[i].LeaveRecord)
		record.AbsenceType = &AbsenceType{
			ID:     record.AbsenceTypeID,
			Code:   rows[i].AbsenceTypeCode,
			Name:   rows[i].AbsenceTypeName,
			NameET: rows[i].AbsenceTypeNameET,
		}
		records[i] = record
	}
	return records, nil
}

// UpdateLeaveRecord updates a leave record.
func (r *AbsenceGORMRepository) UpdateLeaveRecord(ctx context.Context, schemaName string, record *LeaveRecord) error {
	db, err := r.tenantTable(ctx, schemaName, "leave_records")
	if err != nil {
		return err
	}

	query := db.Where("id = ?", record.ID)
	if record.TenantID != "" {
		query = query.Where("tenant_id = ?", record.TenantID)
	}
	result := query.Updates(map[string]interface{}{
		"status":           string(record.Status),
		"approved_at":      record.ApprovedAt,
		"approved_by":      stringPtrIfNotBlank(record.ApprovedBy),
		"rejected_at":      record.RejectedAt,
		"rejected_by":      stringPtrIfNotBlank(record.RejectedBy),
		"rejection_reason": stringPtrIfNotBlank(record.RejectionReason),
		"updated_at":       record.UpdatedAt,
	})
	if result.Error != nil {
		return fmt.Errorf("update leave record: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrLeaveRecordNotFound
	}
	return nil
}

type leaveBalanceWithAbsenceTypeRow struct {
	models.LeaveBalance
	AbsenceTypeCode   string `gorm:"column:absence_type_code"`
	AbsenceTypeName   string `gorm:"column:absence_type_name"`
	AbsenceTypeNameET string `gorm:"column:absence_type_name_et"`
}

type leaveRecordWithAbsenceTypeRow struct {
	models.LeaveRecord
	AbsenceTypeCode   string `gorm:"column:absence_type_code"`
	AbsenceTypeName   string `gorm:"column:absence_type_name"`
	AbsenceTypeNameET string `gorm:"column:absence_type_name_et"`
}

func modelToAbsenceType(m *models.AbsenceType) *AbsenceType {
	return &AbsenceType{
		ID:                 m.ID,
		TenantID:           m.TenantID,
		Code:               m.Code,
		Name:               m.Name,
		NameET:             m.NameET,
		Description:        stringValue(m.Description),
		IsPaid:             m.IsPaid,
		AffectsSalary:      m.AffectsSalary,
		RequiresDocument:   m.RequiresDocument,
		DocumentType:       stringValue(m.DocumentType),
		DefaultDaysPerYear: m.DefaultDaysPerYear.Decimal,
		MaxCarryoverDays:   m.MaxCarryoverDays.Decimal,
		TSDCode:            stringValue(m.TSDCode),
		EMTACode:           stringValue(m.EMTACode),
		IsSystem:           m.IsSystem,
		IsActive:           m.IsActive,
		SortOrder:          m.SortOrder,
		CreatedAt:          m.CreatedAt,
		UpdatedAt:          m.UpdatedAt,
	}
}

func absenceTypeToModel(t *AbsenceType) *models.AbsenceType {
	return &models.AbsenceType{
		ID:                 t.ID,
		TenantID:           t.TenantID,
		Code:               t.Code,
		Name:               t.Name,
		NameET:             t.NameET,
		Description:        stringPtrIfNotBlank(t.Description),
		IsPaid:             t.IsPaid,
		AffectsSalary:      t.AffectsSalary,
		RequiresDocument:   t.RequiresDocument,
		DocumentType:       stringPtrIfNotBlank(t.DocumentType),
		DefaultDaysPerYear: models.Decimal{Decimal: t.DefaultDaysPerYear},
		MaxCarryoverDays:   models.Decimal{Decimal: t.MaxCarryoverDays},
		TSDCode:            stringPtrIfNotBlank(t.TSDCode),
		EMTACode:           stringPtrIfNotBlank(t.EMTACode),
		IsSystem:           t.IsSystem,
		IsActive:           t.IsActive,
		SortOrder:          t.SortOrder,
		CreatedAt:          t.CreatedAt,
		UpdatedAt:          t.UpdatedAt,
	}
}

func modelToLeaveBalance(m *models.LeaveBalance) *LeaveBalance {
	return &LeaveBalance{
		ID:            m.ID,
		TenantID:      m.TenantID,
		EmployeeID:    m.EmployeeID,
		AbsenceTypeID: m.AbsenceTypeID,
		Year:          m.Year,
		EntitledDays:  m.EntitledDays.Decimal,
		CarryoverDays: m.CarryoverDays.Decimal,
		UsedDays:      m.UsedDays.Decimal,
		PendingDays:   m.PendingDays.Decimal,
		RemainingDays: m.RemainingDays.Decimal,
		Notes:         stringValue(m.Notes),
		CreatedAt:     m.CreatedAt,
		UpdatedAt:     m.UpdatedAt,
	}
}

func leaveBalanceToModel(b *LeaveBalance) *models.LeaveBalance {
	return &models.LeaveBalance{
		ID:            b.ID,
		TenantID:      b.TenantID,
		EmployeeID:    b.EmployeeID,
		AbsenceTypeID: b.AbsenceTypeID,
		Year:          b.Year,
		EntitledDays:  models.Decimal{Decimal: b.EntitledDays},
		CarryoverDays: models.Decimal{Decimal: b.CarryoverDays},
		UsedDays:      models.Decimal{Decimal: b.UsedDays},
		PendingDays:   models.Decimal{Decimal: b.PendingDays},
		Notes:         stringPtrIfNotBlank(b.Notes),
		CreatedAt:     b.CreatedAt,
		UpdatedAt:     b.UpdatedAt,
	}
}

func modelToLeaveRecord(m *models.LeaveRecord) *LeaveRecord {
	return &LeaveRecord{
		ID:              m.ID,
		TenantID:        m.TenantID,
		EmployeeID:      m.EmployeeID,
		AbsenceTypeID:   m.AbsenceTypeID,
		StartDate:       m.StartDate,
		EndDate:         m.EndDate,
		TotalDays:       m.TotalDays.Decimal,
		WorkingDays:     m.WorkingDays.Decimal,
		Status:          LeaveStatus(m.Status),
		DocumentNumber:  stringValue(m.DocumentNumber),
		DocumentDate:    m.DocumentDate,
		DocumentURL:     stringValue(m.DocumentURL),
		RequestedAt:     m.RequestedAt,
		RequestedBy:     stringValue(m.RequestedBy),
		ApprovedAt:      m.ApprovedAt,
		ApprovedBy:      stringValue(m.ApprovedBy),
		RejectedAt:      m.RejectedAt,
		RejectedBy:      stringValue(m.RejectedBy),
		RejectionReason: stringValue(m.RejectionReason),
		PayrollRunID:    stringValue(m.PayrollRunID),
		Notes:           stringValue(m.Notes),
		CreatedAt:       m.CreatedAt,
		UpdatedAt:       m.UpdatedAt,
	}
}

func leaveRecordToModel(r *LeaveRecord) *models.LeaveRecord {
	return &models.LeaveRecord{
		ID:              r.ID,
		TenantID:        r.TenantID,
		EmployeeID:      r.EmployeeID,
		AbsenceTypeID:   r.AbsenceTypeID,
		StartDate:       r.StartDate,
		EndDate:         r.EndDate,
		TotalDays:       models.Decimal{Decimal: r.TotalDays},
		WorkingDays:     models.Decimal{Decimal: r.WorkingDays},
		Status:          string(r.Status),
		DocumentNumber:  stringPtrIfNotBlank(r.DocumentNumber),
		DocumentDate:    r.DocumentDate,
		DocumentURL:     stringPtrIfNotBlank(r.DocumentURL),
		RequestedAt:     r.RequestedAt,
		RequestedBy:     stringPtrIfNotBlank(r.RequestedBy),
		ApprovedAt:      r.ApprovedAt,
		ApprovedBy:      stringPtrIfNotBlank(r.ApprovedBy),
		RejectedAt:      r.RejectedAt,
		RejectedBy:      stringPtrIfNotBlank(r.RejectedBy),
		RejectionReason: stringPtrIfNotBlank(r.RejectionReason),
		PayrollRunID:    stringPtrIfNotBlank(r.PayrollRunID),
		Notes:           stringPtrIfNotBlank(r.Notes),
		CreatedAt:       r.CreatedAt,
		UpdatedAt:       r.UpdatedAt,
	}
}
