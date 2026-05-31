package models

import "time"

// AbsenceType represents a tenant leave/absence type.
type AbsenceType struct {
	ID          string  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	TenantID    string  `gorm:"type:uuid;not null;index" json:"tenant_id"`
	Code        string  `gorm:"size:50;not null" json:"code"`
	Name        string  `gorm:"size:200;not null" json:"name"`
	NameET      string  `gorm:"column:name_et;size:200;not null" json:"name_et"`
	Description *string `gorm:"type:text" json:"description,omitempty"`

	IsPaid           bool    `gorm:"column:is_paid;not null" json:"is_paid"`
	AffectsSalary    bool    `gorm:"column:affects_salary;not null" json:"affects_salary"`
	RequiresDocument bool    `gorm:"column:requires_document;not null" json:"requires_document"`
	DocumentType     *string `gorm:"column:document_type;size:100" json:"document_type,omitempty"`

	DefaultDaysPerYear Decimal `gorm:"column:default_days_per_year;type:numeric(5,2);not null;default:0" json:"default_days_per_year"`
	MaxCarryoverDays   Decimal `gorm:"column:max_carryover_days;type:numeric(5,2);not null;default:0" json:"max_carryover_days"`

	TSDCode  *string `gorm:"column:tsd_code;size:20" json:"tsd_code,omitempty"`
	EMTACode *string `gorm:"column:emta_code;size:20" json:"emta_code,omitempty"`

	IsSystem  bool      `gorm:"column:is_system;not null" json:"is_system"`
	IsActive  bool      `gorm:"column:is_active;not null" json:"is_active"`
	SortOrder int       `gorm:"column:sort_order;not null;default:0" json:"sort_order"`
	CreatedAt time.Time `gorm:"not null;default:now()" json:"created_at"`
	UpdatedAt time.Time `gorm:"not null;default:now()" json:"updated_at"`
}

func (AbsenceType) TableName() string {
	return "absence_types"
}

// LeaveBalance tracks leave entitlement per employee and absence type.
type LeaveBalance struct {
	ID            string `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	TenantID      string `gorm:"type:uuid;not null;index" json:"tenant_id"`
	EmployeeID    string `gorm:"column:employee_id;type:uuid;not null;index" json:"employee_id"`
	AbsenceTypeID string `gorm:"column:absence_type_id;type:uuid;not null;index" json:"absence_type_id"`
	Year          int    `gorm:"not null;index" json:"year"`

	EntitledDays  Decimal `gorm:"column:entitled_days;type:numeric(5,2);not null;default:0" json:"entitled_days"`
	CarryoverDays Decimal `gorm:"column:carryover_days;type:numeric(5,2);not null;default:0" json:"carryover_days"`
	UsedDays      Decimal `gorm:"column:used_days;type:numeric(5,2);not null;default:0" json:"used_days"`
	PendingDays   Decimal `gorm:"column:pending_days;type:numeric(5,2);not null;default:0" json:"pending_days"`
	RemainingDays Decimal `gorm:"column:remaining_days;type:numeric(5,2);->" json:"remaining_days"`

	Notes     *string   `gorm:"type:text" json:"notes,omitempty"`
	CreatedAt time.Time `gorm:"not null;default:now()" json:"created_at"`
	UpdatedAt time.Time `gorm:"not null;default:now()" json:"updated_at"`
}

func (LeaveBalance) TableName() string {
	return "leave_balances"
}

// LeaveRecord represents an individual leave/absence record.
type LeaveRecord struct {
	ID            string `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	TenantID      string `gorm:"type:uuid;not null;index" json:"tenant_id"`
	EmployeeID    string `gorm:"column:employee_id;type:uuid;not null;index" json:"employee_id"`
	AbsenceTypeID string `gorm:"column:absence_type_id;type:uuid;not null;index" json:"absence_type_id"`

	StartDate time.Time `gorm:"column:start_date;type:date;not null" json:"start_date"`
	EndDate   time.Time `gorm:"column:end_date;type:date;not null" json:"end_date"`

	TotalDays   Decimal `gorm:"column:total_days;type:numeric(5,2);not null" json:"total_days"`
	WorkingDays Decimal `gorm:"column:working_days;type:numeric(5,2);not null" json:"working_days"`
	Status      string  `gorm:"size:20;not null;default:'PENDING'" json:"status"`

	DocumentNumber *string    `gorm:"column:document_number;size:100" json:"document_number,omitempty"`
	DocumentDate   *time.Time `gorm:"column:document_date;type:date" json:"document_date,omitempty"`
	DocumentURL    *string    `gorm:"column:document_url;type:text" json:"document_url,omitempty"`

	RequestedAt time.Time  `gorm:"column:requested_at;not null;default:now()" json:"requested_at"`
	RequestedBy *string    `gorm:"column:requested_by;type:uuid" json:"requested_by,omitempty"`
	ApprovedAt  *time.Time `gorm:"column:approved_at" json:"approved_at,omitempty"`
	ApprovedBy  *string    `gorm:"column:approved_by;type:uuid" json:"approved_by,omitempty"`
	RejectedAt  *time.Time `gorm:"column:rejected_at" json:"rejected_at,omitempty"`
	RejectedBy  *string    `gorm:"column:rejected_by;type:uuid" json:"rejected_by,omitempty"`

	RejectionReason *string `gorm:"column:rejection_reason;type:text" json:"rejection_reason,omitempty"`
	PayrollRunID    *string `gorm:"column:payroll_run_id;type:uuid" json:"payroll_run_id,omitempty"`
	Notes           *string `gorm:"type:text" json:"notes,omitempty"`

	CreatedAt time.Time `gorm:"not null;default:now()" json:"created_at"`
	UpdatedAt time.Time `gorm:"not null;default:now()" json:"updated_at"`
}

func (LeaveRecord) TableName() string {
	return "leave_records"
}
