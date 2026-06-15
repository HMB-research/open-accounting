package models

import "time"

// Order represents a tenant sales order.
type Order struct {
	ID                   string     `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()" json:"id"`
	TenantID             string     `gorm:"column:tenant_id;type:uuid;not null;index" json:"tenant_id"`
	OrderNumber          string     `gorm:"column:order_number;size:50;not null" json:"order_number"`
	ContactID            string     `gorm:"column:contact_id;type:uuid;not null;index" json:"contact_id"`
	OrderDate            time.Time  `gorm:"column:order_date;type:date;not null" json:"order_date"`
	ExpectedDelivery     *time.Time `gorm:"column:expected_delivery;type:date" json:"expected_delivery,omitempty"`
	Status               string     `gorm:"size:20;not null;default:'PENDING'" json:"status"`
	Currency             string     `gorm:"size:3;not null;default:'EUR'" json:"currency"`
	ExchangeRate         Decimal    `gorm:"column:exchange_rate;type:numeric(18,10);not null;default:1" json:"exchange_rate"`
	Subtotal             Decimal    `gorm:"type:numeric(28,8);not null;default:0" json:"subtotal"`
	VATAmount            Decimal    `gorm:"column:vat_amount;type:numeric(28,8);not null;default:0" json:"vat_amount"`
	Total                Decimal    `gorm:"type:numeric(28,8);not null;default:0" json:"total"`
	Notes                string     `gorm:"type:text" json:"notes,omitempty"`
	QuoteID              *string    `gorm:"column:quote_id;type:uuid" json:"quote_id,omitempty"`
	ConvertedToInvoiceID *string    `gorm:"column:converted_to_invoice_id;type:uuid" json:"converted_to_invoice_id,omitempty"`
	CreatedAt            time.Time  `gorm:"not null;default:now()" json:"created_at"`
	CreatedBy            string     `gorm:"column:created_by;type:uuid;not null" json:"created_by"`
	UpdatedAt            time.Time  `gorm:"not null;default:now()" json:"updated_at"`
}

// TableName returns the table name for GORM.
func (Order) TableName() string {
	return "orders"
}

// OrderLine represents a line item on a tenant sales order.
type OrderLine struct {
	ID              string  `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()" json:"id"`
	TenantID        string  `gorm:"column:tenant_id;type:uuid;not null;index" json:"tenant_id"`
	OrderID         string  `gorm:"column:order_id;type:uuid;not null;index" json:"order_id"`
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
func (OrderLine) TableName() string {
	return "order_lines"
}

// OrderStockReservation stores a tenant order stock reservation.
type OrderStockReservation struct {
	ID          string     `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	TenantID    string     `gorm:"column:tenant_id;type:uuid;not null;uniqueIndex:idx_order_stock_reservation_unique" json:"tenant_id"`
	OrderID     string     `gorm:"column:order_id;type:uuid;not null;uniqueIndex:idx_order_stock_reservation_unique" json:"order_id"`
	ProductID   string     `gorm:"column:product_id;type:uuid;not null;uniqueIndex:idx_order_stock_reservation_unique" json:"product_id"`
	WarehouseID string     `gorm:"column:warehouse_id;type:uuid;not null;uniqueIndex:idx_order_stock_reservation_unique" json:"warehouse_id"`
	Quantity    Decimal    `gorm:"type:numeric(15,3);not null;default:0" json:"quantity"`
	Status      string     `gorm:"size:20;not null;default:'RESERVED'" json:"status"`
	Reason      *string    `gorm:"type:text" json:"reason,omitempty"`
	CreatedAt   time.Time  `gorm:"not null;default:now()" json:"created_at"`
	CreatedBy   *string    `gorm:"column:created_by;type:uuid" json:"created_by,omitempty"`
	UpdatedAt   time.Time  `gorm:"not null;default:now()" json:"updated_at"`
	ReleasedAt  *time.Time `gorm:"column:released_at" json:"released_at,omitempty"`
	ReleasedBy  *string    `gorm:"column:released_by;type:uuid" json:"released_by,omitempty"`
}

// TableName returns the table name for GORM.
func (OrderStockReservation) TableName() string {
	return "order_stock_reservations"
}
