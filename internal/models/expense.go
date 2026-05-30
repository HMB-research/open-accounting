package models

import "time"

// Expense represents a tenant-scoped expense claim (GORM model).
type Expense struct {
	ID               string     `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()" json:"id"`
	TenantID         string     `gorm:"column:tenant_id;type:uuid;not null;index" json:"tenant_id"`
	ExpenseNumber    string     `gorm:"column:expense_number;size:30;not null" json:"expense_number"`
	ExpenseDate      time.Time  `gorm:"column:expense_date;type:date;not null" json:"expense_date"`
	Merchant         string     `gorm:"size:200;not null" json:"merchant"`
	Description      string     `gorm:"type:text;not null;default:''" json:"description,omitempty"`
	EmployeeID       *string    `gorm:"column:employee_id;type:uuid" json:"employee_id,omitempty"`
	ContactID        *string    `gorm:"column:contact_id;type:uuid" json:"contact_id,omitempty"`
	ExpenseAccountID string     `gorm:"column:expense_account_id;type:uuid;not null" json:"expense_account_id"`
	PaymentAccountID string     `gorm:"column:payment_account_id;type:uuid;not null" json:"payment_account_id"`
	Amount           Decimal    `gorm:"type:numeric(28,8);not null" json:"amount"`
	Currency         string     `gorm:"size:3;not null;default:'EUR'" json:"currency"`
	ExchangeRate     Decimal    `gorm:"column:exchange_rate;type:numeric(18,10);not null;default:1" json:"exchange_rate"`
	BaseAmount       Decimal    `gorm:"column:base_amount;type:numeric(28,8);not null" json:"base_amount"`
	RequiresReceipt  bool       `gorm:"column:requires_receipt;not null;default:true" json:"requires_receipt"`
	Status           string     `gorm:"size:20;not null;default:'DRAFT'" json:"status"`
	JournalEntryID   *string    `gorm:"column:journal_entry_id;type:uuid" json:"journal_entry_id,omitempty"`
	SubmittedAt      *time.Time `gorm:"column:submitted_at" json:"submitted_at,omitempty"`
	SubmittedBy      *string    `gorm:"column:submitted_by;type:uuid" json:"submitted_by,omitempty"`
	ApprovedAt       *time.Time `gorm:"column:approved_at" json:"approved_at,omitempty"`
	ApprovedBy       *string    `gorm:"column:approved_by;type:uuid" json:"approved_by,omitempty"`
	RejectedAt       *time.Time `gorm:"column:rejected_at" json:"rejected_at,omitempty"`
	RejectedBy       *string    `gorm:"column:rejected_by;type:uuid" json:"rejected_by,omitempty"`
	RejectionReason  string     `gorm:"column:rejection_reason;type:text" json:"rejection_reason,omitempty"`
	PostedAt         *time.Time `gorm:"column:posted_at" json:"posted_at,omitempty"`
	PostedBy         *string    `gorm:"column:posted_by;type:uuid" json:"posted_by,omitempty"`
	CreatedAt        time.Time  `gorm:"not null;default:now()" json:"created_at"`
	CreatedBy        string     `gorm:"column:created_by;type:uuid;not null" json:"created_by"`
	UpdatedAt        time.Time  `gorm:"not null;default:now()" json:"updated_at"`
}

// TableName returns the table name for GORM.
func (Expense) TableName() string {
	return "expenses"
}
