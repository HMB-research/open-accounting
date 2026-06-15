package models

import "time"

// CostCenter represents a tenant cost center for budget tracking.
type CostCenter struct {
	ID           string    `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	TenantID     string    `gorm:"column:tenant_id;type:uuid;not null;index" json:"tenant_id"`
	Code         string    `gorm:"size:20;not null;uniqueIndex:idx_cost_center_tenant_code" json:"code"`
	Name         string    `gorm:"size:200;not null" json:"name"`
	Description  string    `gorm:"type:text" json:"description,omitempty"`
	ParentID     *string   `gorm:"column:parent_id;type:uuid;index" json:"parent_id,omitempty"`
	IsActive     bool      `gorm:"column:is_active;not null;default:true;index" json:"is_active"`
	BudgetAmount *Decimal  `gorm:"column:budget_amount;type:numeric(15,2)" json:"budget_amount,omitempty"`
	BudgetPeriod string    `gorm:"column:budget_period;size:20;default:'ANNUAL'" json:"budget_period"`
	CreatedAt    time.Time `gorm:"not null;default:now()" json:"created_at"`
	UpdatedAt    time.Time `gorm:"not null;default:now()" json:"updated_at"`
}

// TableName returns the table name for GORM.
func (CostCenter) TableName() string {
	return "cost_centers"
}

// CostAllocation represents an amount allocated to a cost center.
type CostAllocation struct {
	ID                   string    `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	TenantID             string    `gorm:"column:tenant_id;type:uuid;not null;index" json:"tenant_id"`
	CostCenterID         string    `gorm:"column:cost_center_id;type:uuid;not null;index" json:"cost_center_id"`
	JournalEntryLineID   string    `gorm:"column:journal_entry_line_id;type:uuid;not null;index" json:"journal_entry_line_id"`
	Amount               Decimal   `gorm:"type:numeric(15,2);not null" json:"amount"`
	AllocationPercentage *Decimal  `gorm:"column:allocation_percentage;type:numeric(5,2)" json:"allocation_percentage,omitempty"`
	AllocationDate       time.Time `gorm:"column:allocation_date;type:date;not null;index" json:"allocation_date"`
	Notes                string    `gorm:"type:text" json:"notes,omitempty"`
	CreatedAt            time.Time `gorm:"not null;default:now()" json:"created_at"`
}

// TableName returns the table name for GORM.
func (CostAllocation) TableName() string {
	return "cost_allocations"
}
