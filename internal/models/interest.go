package models

import "time"

// InvoiceInterest stores a calculated late-payment interest snapshot.
type InvoiceInterest struct {
	ID                string    `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	InvoiceID         string    `gorm:"column:invoice_id;type:uuid;not null;index" json:"invoice_id"`
	CalculatedAt      time.Time `gorm:"column:calculated_at;not null;index" json:"calculated_at"`
	DaysOverdue       int       `gorm:"column:days_overdue;not null" json:"days_overdue"`
	PrincipalAmount   Decimal   `gorm:"column:principal_amount;type:numeric(15,2);not null" json:"principal_amount"`
	InterestRate      Decimal   `gorm:"column:interest_rate;type:numeric(8,6);not null" json:"interest_rate"`
	InterestAmount    Decimal   `gorm:"column:interest_amount;type:numeric(15,2);not null" json:"interest_amount"`
	TotalWithInterest Decimal   `gorm:"column:total_with_interest;type:numeric(15,2);not null" json:"total_with_interest"`
	CreatedAt         time.Time `gorm:"not null;default:now()" json:"created_at"`
}

// TableName returns the table name for GORM.
func (InvoiceInterest) TableName() string {
	return "invoice_interest"
}
