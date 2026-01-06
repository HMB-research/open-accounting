package payroll

import (
	"time"

	"github.com/shopspring/decimal"
)

// LeaveStatus defines the status of a leave request
type LeaveStatus string

const (
	LeavePending  LeaveStatus = "PENDING"
	LeaveApproved LeaveStatus = "APPROVED"
	LeaveRejected LeaveStatus = "REJECTED"
	LeaveCanceled LeaveStatus = "CANCELED"
)

// AbsenceType represents a type of leave/absence
type AbsenceType struct {
	ID          string `json:"id"`
	TenantID    string `json:"tenant_id"`
	Code        string `json:"code"`
	Name        string `json:"name"`
	NameET      string `json:"name_et"` // Estonian name
	Description string `json:"description,omitempty"`

	// Configuration
	IsPaid           bool   `json:"is_paid"`
	AffectsSalary    bool   `json:"affects_salary"`
	RequiresDocument bool   `json:"requires_document"`
	DocumentType     string `json:"document_type,omitempty"` // TK66, medical certificate, etc.

	// Accrual settings
	DefaultDaysPerYear decimal.Decimal `json:"default_days_per_year"`
	MaxCarryoverDays   decimal.Decimal `json:"max_carryover_days"`

	// Estonian regulatory codes
	TSDCode  string `json:"tsd_code,omitempty"`  // Code for TSD declaration
	EMTACode string `json:"emta_code,omitempty"` // EMTA (Tax Board) classification

	IsSystem  bool      `json:"is_system"` // System types cannot be deleted
	IsActive  bool      `json:"is_active"`
	SortOrder int       `json:"sort_order"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// LeaveBalance tracks leave entitlement per employee per year
type LeaveBalance struct {
	ID            string `json:"id"`
	TenantID      string `json:"tenant_id"`
	EmployeeID    string `json:"employee_id"`
	AbsenceTypeID string `json:"absence_type_id"`
	Year          int    `json:"year"`

	// Days tracking
	EntitledDays  decimal.Decimal `json:"entitled_days"`  // Total entitled for year
	CarryoverDays decimal.Decimal `json:"carryover_days"` // Carried from previous year
	UsedDays      decimal.Decimal `json:"used_days"`      // Already taken
	PendingDays   decimal.Decimal `json:"pending_days"`   // Requested but not approved
	RemainingDays decimal.Decimal `json:"remaining_days"` // Calculated: entitled + carryover - used - pending

	Notes     string    `json:"notes,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// Loaded relations
	AbsenceType *AbsenceType `json:"absence_type,omitempty"`
}

// LeaveRecord represents an individual absence entry
type LeaveRecord struct {
	ID            string `json:"id"`
	TenantID      string `json:"tenant_id"`
	EmployeeID    string `json:"employee_id"`
	AbsenceTypeID string `json:"absence_type_id"`

	// Period
	StartDate time.Time `json:"start_date"`
	EndDate   time.Time `json:"end_date"`

	// Duration
	TotalDays   decimal.Decimal `json:"total_days"`   // Calendar days
	WorkingDays decimal.Decimal `json:"working_days"` // Excluding weekends/holidays

	// Status workflow
	Status LeaveStatus `json:"status"`

	// Documentation
	DocumentNumber string     `json:"document_number,omitempty"` // TK66 number, etc.
	DocumentDate   *time.Time `json:"document_date,omitempty"`
	DocumentURL    string     `json:"document_url,omitempty"`

	// Approval workflow
	RequestedAt     time.Time  `json:"requested_at"`
	RequestedBy     string     `json:"requested_by,omitempty"`
	ApprovedAt      *time.Time `json:"approved_at,omitempty"`
	ApprovedBy      string     `json:"approved_by,omitempty"`
	RejectedAt      *time.Time `json:"rejected_at,omitempty"`
	RejectedBy      string     `json:"rejected_by,omitempty"`
	RejectionReason string     `json:"rejection_reason,omitempty"`

	// Integration with payroll
	PayrollRunID string `json:"payroll_run_id,omitempty"`

	Notes     string    `json:"notes,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// Loaded relations
	AbsenceType *AbsenceType `json:"absence_type,omitempty"`
	Employee    *Employee    `json:"employee,omitempty"`
}

// Request types

// CreateLeaveRecordRequest is the request to create a leave record
type CreateLeaveRecordRequest struct {
	EmployeeID     string          `json:"employee_id"`
	AbsenceTypeID  string          `json:"absence_type_id"`
	StartDate      time.Time       `json:"start_date"`
	EndDate        time.Time       `json:"end_date"`
	TotalDays      decimal.Decimal `json:"total_days"`
	WorkingDays    decimal.Decimal `json:"working_days"`
	DocumentNumber string          `json:"document_number,omitempty"`
	DocumentDate   *time.Time      `json:"document_date,omitempty"`
	Notes          string          `json:"notes,omitempty"`
}

// ApproveLeaveRequest is the request to approve a leave record
type ApproveLeaveRequest struct {
	ApprovedBy string `json:"approved_by"`
}

// RejectLeaveRequest is the request to reject a leave record
type RejectLeaveRequest struct {
	RejectedBy string `json:"rejected_by"`
	Reason     string `json:"reason"`
}

// UpdateLeaveBalanceRequest is the request to update a leave balance
type UpdateLeaveBalanceRequest struct {
	EntitledDays  *decimal.Decimal `json:"entitled_days,omitempty"`
	CarryoverDays *decimal.Decimal `json:"carryover_days,omitempty"`
	Notes         string           `json:"notes,omitempty"`
}
