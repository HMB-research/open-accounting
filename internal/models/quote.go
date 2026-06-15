package models

import "time"

// Quote represents a tenant sales quote.
type Quote struct {
	ID                   string     `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()" json:"id"`
	TenantID             string     `gorm:"column:tenant_id;type:uuid;not null;index" json:"tenant_id"`
	QuoteNumber          string     `gorm:"column:quote_number;size:50;not null" json:"quote_number"`
	ContactID            string     `gorm:"column:contact_id;type:uuid;not null;index" json:"contact_id"`
	QuoteDate            time.Time  `gorm:"column:quote_date;type:date;not null" json:"quote_date"`
	ValidUntil           *time.Time `gorm:"column:valid_until;type:date" json:"valid_until,omitempty"`
	Status               string     `gorm:"size:20;not null;default:'DRAFT'" json:"status"`
	Currency             string     `gorm:"size:3;not null;default:'EUR'" json:"currency"`
	ExchangeRate         Decimal    `gorm:"column:exchange_rate;type:numeric(18,10);not null;default:1" json:"exchange_rate"`
	Subtotal             Decimal    `gorm:"type:numeric(28,8);not null;default:0" json:"subtotal"`
	VATAmount            Decimal    `gorm:"column:vat_amount;type:numeric(28,8);not null;default:0" json:"vat_amount"`
	Total                Decimal    `gorm:"type:numeric(28,8);not null;default:0" json:"total"`
	Notes                string     `gorm:"type:text" json:"notes,omitempty"`
	ConvertedToOrderID   *string    `gorm:"column:converted_to_order_id;type:uuid" json:"converted_to_order_id,omitempty"`
	ConvertedToInvoiceID *string    `gorm:"column:converted_to_invoice_id;type:uuid" json:"converted_to_invoice_id,omitempty"`
	CreatedAt            time.Time  `gorm:"not null;default:now()" json:"created_at"`
	CreatedBy            string     `gorm:"column:created_by;type:uuid;not null" json:"created_by"`
	UpdatedAt            time.Time  `gorm:"not null;default:now()" json:"updated_at"`
}

// TableName returns the table name for GORM.
func (Quote) TableName() string {
	return "quotes"
}

// QuoteLine represents a line item on a tenant sales quote.
type QuoteLine struct {
	ID              string  `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()" json:"id"`
	TenantID        string  `gorm:"column:tenant_id;type:uuid;not null;index" json:"tenant_id"`
	QuoteID         string  `gorm:"column:quote_id;type:uuid;not null;index" json:"quote_id"`
	LineNumber      int     `gorm:"column:line_number;not null" json:"line_number"`
	Description     string  `gorm:"type:text;not null" json:"description"`
	Quantity        Decimal `gorm:"type:numeric(18,6);not null;default:1" json:"quantity"`
	Unit            string  `gorm:"size:20" json:"unit,omitempty"`
	UnitPrice       Decimal `gorm:"column:unit_price;type:numeric(28,8);not null" json:"unit_price"`
	DiscountPercent Decimal `gorm:"column:discount_percent;type:numeric(5,2);not null;default:0" json:"discount_percent"`
	VATRate         Decimal `gorm:"column:vat_rate;type:numeric(5,2);not null" json:"vat_rate"`
	LineSubtotal    Decimal `gorm:"column:line_subtotal;type:numeric(28,8);not null" json:"line_subtotal"`
	LineVAT         Decimal `gorm:"column:line_vat;type:numeric(28,8);not null" json:"line_vat"`
	LineTotal       Decimal `gorm:"column:line_total;type:numeric(28,8);not null" json:"line_total"`
	ProductID       *string `gorm:"column:product_id;type:uuid" json:"product_id,omitempty"`
}

// TableName returns the table name for GORM.
func (QuoteLine) TableName() string {
	return "quote_lines"
}
