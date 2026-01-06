package orders

import (
	"errors"
	"time"

	"github.com/shopspring/decimal"

	"github.com/HMB-research/open-accounting/internal/contacts"
)

// OrderStatus represents the status of an order
type OrderStatus string

const (
	OrderStatusPending    OrderStatus = "PENDING"
	OrderStatusConfirmed  OrderStatus = "CONFIRMED"
	OrderStatusProcessing OrderStatus = "PROCESSING"
	OrderStatusShipped    OrderStatus = "SHIPPED"
	OrderStatusDelivered  OrderStatus = "DELIVERED"
	OrderStatusCanceled   OrderStatus = "CANCELED"
)

// Order represents a sales order
type Order struct {
	ID                   string            `json:"id"`
	TenantID             string            `json:"tenant_id"`
	OrderNumber          string            `json:"order_number"`
	ContactID            string            `json:"contact_id"`
	Contact              *contacts.Contact `json:"contact,omitempty"`
	OrderDate            time.Time         `json:"order_date"`
	ExpectedDelivery     *time.Time        `json:"expected_delivery,omitempty"`
	Status               OrderStatus       `json:"status"`
	Currency             string            `json:"currency"`
	ExchangeRate         decimal.Decimal   `json:"exchange_rate"`
	Subtotal             decimal.Decimal   `json:"subtotal"`
	VATAmount            decimal.Decimal   `json:"vat_amount"`
	Total                decimal.Decimal   `json:"total"`
	Notes                string            `json:"notes,omitempty"`
	QuoteID              *string           `json:"quote_id,omitempty"`
	ConvertedToInvoiceID *string           `json:"converted_to_invoice_id,omitempty"`
	Lines                []OrderLine       `json:"lines"`
	CreatedAt            time.Time         `json:"created_at"`
	CreatedBy            string            `json:"created_by"`
	UpdatedAt            time.Time         `json:"updated_at"`
}

// OrderLine represents a line item on an order
type OrderLine struct {
	ID              string          `json:"id"`
	TenantID        string          `json:"tenant_id"`
	OrderID         string          `json:"order_id"`
	LineNumber      int             `json:"line_number"`
	Description     string          `json:"description"`
	Quantity        decimal.Decimal `json:"quantity"`
	Unit            string          `json:"unit,omitempty"`
	UnitPrice       decimal.Decimal `json:"unit_price"`
	DiscountPercent decimal.Decimal `json:"discount_percent"`
	VATRate         decimal.Decimal `json:"vat_rate"`
	LineSubtotal    decimal.Decimal `json:"line_subtotal"`
	LineVAT         decimal.Decimal `json:"line_vat"`
	LineTotal       decimal.Decimal `json:"line_total"`
	ProductID       *string         `json:"product_id,omitempty"`
}

// Calculate computes the line totals
func (l *OrderLine) Calculate() {
	// Subtotal = quantity * unit_price * (1 - discount/100)
	grossAmount := l.Quantity.Mul(l.UnitPrice)
	discountAmount := grossAmount.Mul(l.DiscountPercent).Div(decimal.NewFromInt(100))
	l.LineSubtotal = grossAmount.Sub(discountAmount).Round(2)

	// VAT = subtotal * vat_rate/100
	l.LineVAT = l.LineSubtotal.Mul(l.VATRate).Div(decimal.NewFromInt(100)).Round(2)

	// Total = subtotal + VAT
	l.LineTotal = l.LineSubtotal.Add(l.LineVAT)
}

// Calculate computes the order totals from lines
func (o *Order) Calculate() {
	o.Subtotal = decimal.Zero
	o.VATAmount = decimal.Zero
	o.Total = decimal.Zero

	for i := range o.Lines {
		o.Lines[i].Calculate()
		o.Subtotal = o.Subtotal.Add(o.Lines[i].LineSubtotal)
		o.VATAmount = o.VATAmount.Add(o.Lines[i].LineVAT)
		o.Total = o.Total.Add(o.Lines[i].LineTotal)
	}
}

// Validate validates the order
func (o *Order) Validate() error {
	if len(o.Lines) == 0 {
		return errors.New("order must have at least one line")
	}

	if o.ContactID == "" {
		return errors.New("contact is required")
	}

	if o.OrderDate.IsZero() {
		return errors.New("order date is required")
	}

	for i, line := range o.Lines {
		if line.Description == "" {
			return errors.New("line description is required")
		}
		if line.Quantity.LessThanOrEqual(decimal.Zero) {
			return errors.New("line quantity must be positive")
		}
		if line.UnitPrice.LessThan(decimal.Zero) {
			return errors.New("line unit price cannot be negative")
		}
		if line.VATRate.LessThan(decimal.Zero) {
			return errors.New("line VAT rate cannot be negative")
		}
		if line.DiscountPercent.LessThan(decimal.Zero) || line.DiscountPercent.GreaterThan(decimal.NewFromInt(100)) {
			return errors.New("line discount must be between 0 and 100")
		}
		// Ensure line numbers are sequential
		if line.LineNumber != i+1 {
			o.Lines[i].LineNumber = i + 1
		}
	}

	return nil
}

// CreateOrderRequest is the request to create an order
type CreateOrderRequest struct {
	ContactID        string                   `json:"contact_id"`
	OrderDate        time.Time                `json:"order_date"`
	ExpectedDelivery *time.Time               `json:"expected_delivery,omitempty"`
	Currency         string                   `json:"currency,omitempty"`
	ExchangeRate     decimal.Decimal          `json:"exchange_rate,omitempty"`
	Notes            string                   `json:"notes,omitempty"`
	QuoteID          *string                  `json:"quote_id,omitempty"`
	Lines            []CreateOrderLineRequest `json:"lines"`
	UserID           string                   `json:"-"`
}

// CreateOrderLineRequest is a line in the create order request
type CreateOrderLineRequest struct {
	Description     string          `json:"description"`
	Quantity        decimal.Decimal `json:"quantity"`
	Unit            string          `json:"unit,omitempty"`
	UnitPrice       decimal.Decimal `json:"unit_price"`
	DiscountPercent decimal.Decimal `json:"discount_percent,omitempty"`
	VATRate         decimal.Decimal `json:"vat_rate"`
	ProductID       *string         `json:"product_id,omitempty"`
}

// UpdateOrderRequest is the request to update an order
type UpdateOrderRequest struct {
	ContactID        string                   `json:"contact_id"`
	OrderDate        time.Time                `json:"order_date"`
	ExpectedDelivery *time.Time               `json:"expected_delivery,omitempty"`
	Currency         string                   `json:"currency,omitempty"`
	ExchangeRate     decimal.Decimal          `json:"exchange_rate,omitempty"`
	Notes            string                   `json:"notes,omitempty"`
	Lines            []CreateOrderLineRequest `json:"lines"`
}

// OrderFilter provides filtering options
type OrderFilter struct {
	Status    OrderStatus
	ContactID string
	FromDate  *time.Time
	ToDate    *time.Time
	Search    string
}
