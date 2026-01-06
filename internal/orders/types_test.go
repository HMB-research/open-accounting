package orders

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOrderLineCalculate(t *testing.T) {
	tests := []struct {
		name             string
		line             OrderLine
		wantSubtotal     string
		wantVAT          string
		wantTotal        string
	}{
		{
			name: "simple calculation no discount",
			line: OrderLine{
				Quantity:        decimal.NewFromInt(2),
				UnitPrice:       decimal.NewFromFloat(100.00),
				DiscountPercent: decimal.Zero,
				VATRate:         decimal.NewFromInt(22),
			},
			wantSubtotal: "200",
			wantVAT:      "44",
			wantTotal:    "244",
		},
		{
			name: "with 10% discount",
			line: OrderLine{
				Quantity:        decimal.NewFromInt(1),
				UnitPrice:       decimal.NewFromFloat(100.00),
				DiscountPercent: decimal.NewFromInt(10),
				VATRate:         decimal.NewFromInt(22),
			},
			wantSubtotal: "90",
			wantVAT:      "19.8",
			wantTotal:    "109.8",
		},
		{
			name: "zero VAT rate",
			line: OrderLine{
				Quantity:        decimal.NewFromFloat(5.5),
				UnitPrice:       decimal.NewFromFloat(20.00),
				DiscountPercent: decimal.Zero,
				VATRate:         decimal.Zero,
			},
			wantSubtotal: "110",
			wantVAT:      "0",
			wantTotal:    "110",
		},
		{
			name: "fractional quantities",
			line: OrderLine{
				Quantity:        decimal.NewFromFloat(2.5),
				UnitPrice:       decimal.NewFromFloat(10.00),
				DiscountPercent: decimal.Zero,
				VATRate:         decimal.NewFromInt(20),
			},
			wantSubtotal: "25",
			wantVAT:      "5",
			wantTotal:    "30",
		},
		{
			name: "100% discount",
			line: OrderLine{
				Quantity:        decimal.NewFromInt(1),
				UnitPrice:       decimal.NewFromFloat(100.00),
				DiscountPercent: decimal.NewFromInt(100),
				VATRate:         decimal.NewFromInt(22),
			},
			wantSubtotal: "0",
			wantVAT:      "0",
			wantTotal:    "0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.line.Calculate()

			wantSubtotal, _ := decimal.NewFromString(tt.wantSubtotal)
			wantVAT, _ := decimal.NewFromString(tt.wantVAT)
			wantTotal, _ := decimal.NewFromString(tt.wantTotal)

			assert.True(t, tt.line.LineSubtotal.Equal(wantSubtotal), "subtotal: got %s, want %s", tt.line.LineSubtotal, wantSubtotal)
			assert.True(t, tt.line.LineVAT.Equal(wantVAT), "VAT: got %s, want %s", tt.line.LineVAT, wantVAT)
			assert.True(t, tt.line.LineTotal.Equal(wantTotal), "total: got %s, want %s", tt.line.LineTotal, wantTotal)
		})
	}
}

func TestOrderCalculate(t *testing.T) {
	tests := []struct {
		name         string
		lines        []OrderLine
		wantSubtotal string
		wantVAT      string
		wantTotal    string
	}{
		{
			name: "single line",
			lines: []OrderLine{
				{
					Quantity:        decimal.NewFromInt(1),
					UnitPrice:       decimal.NewFromFloat(100.00),
					DiscountPercent: decimal.Zero,
					VATRate:         decimal.NewFromInt(22),
				},
			},
			wantSubtotal: "100",
			wantVAT:      "22",
			wantTotal:    "122",
		},
		{
			name: "multiple lines",
			lines: []OrderLine{
				{
					Quantity:        decimal.NewFromInt(2),
					UnitPrice:       decimal.NewFromFloat(50.00),
					DiscountPercent: decimal.Zero,
					VATRate:         decimal.NewFromInt(22),
				},
				{
					Quantity:        decimal.NewFromInt(1),
					UnitPrice:       decimal.NewFromFloat(200.00),
					DiscountPercent: decimal.NewFromInt(10),
					VATRate:         decimal.NewFromInt(22),
				},
			},
			wantSubtotal: "280",    // 100 + 180
			wantVAT:      "61.6",   // 22 + 39.6
			wantTotal:    "341.6",  // 122 + 219.6
		},
		{
			name:         "empty lines",
			lines:        []OrderLine{},
			wantSubtotal: "0",
			wantVAT:      "0",
			wantTotal:    "0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			order := &Order{Lines: tt.lines}
			order.Calculate()

			wantSubtotal, _ := decimal.NewFromString(tt.wantSubtotal)
			wantVAT, _ := decimal.NewFromString(tt.wantVAT)
			wantTotal, _ := decimal.NewFromString(tt.wantTotal)

			assert.True(t, order.Subtotal.Equal(wantSubtotal), "subtotal: got %s, want %s", order.Subtotal, wantSubtotal)
			assert.True(t, order.VATAmount.Equal(wantVAT), "VAT: got %s, want %s", order.VATAmount, wantVAT)
			assert.True(t, order.Total.Equal(wantTotal), "total: got %s, want %s", order.Total, wantTotal)
		})
	}
}

func TestOrderValidate(t *testing.T) {
	validLine := OrderLine{
		Description:     "Test product",
		Quantity:        decimal.NewFromInt(1),
		UnitPrice:       decimal.NewFromFloat(100.00),
		DiscountPercent: decimal.Zero,
		VATRate:         decimal.NewFromInt(22),
		LineNumber:      1,
	}

	tests := []struct {
		name    string
		order   Order
		wantErr string
	}{
		{
			name: "valid order",
			order: Order{
				ContactID: "contact-123",
				OrderDate: time.Now(),
				Lines:     []OrderLine{validLine},
			},
			wantErr: "",
		},
		{
			name: "no lines",
			order: Order{
				ContactID: "contact-123",
				OrderDate: time.Now(),
				Lines:     []OrderLine{},
			},
			wantErr: "order must have at least one line",
		},
		{
			name: "missing contact",
			order: Order{
				ContactID: "",
				OrderDate: time.Now(),
				Lines:     []OrderLine{validLine},
			},
			wantErr: "contact is required",
		},
		{
			name: "missing order date",
			order: Order{
				ContactID: "contact-123",
				OrderDate: time.Time{},
				Lines:     []OrderLine{validLine},
			},
			wantErr: "order date is required",
		},
		{
			name: "missing line description",
			order: Order{
				ContactID: "contact-123",
				OrderDate: time.Now(),
				Lines: []OrderLine{
					{
						Description:     "",
						Quantity:        decimal.NewFromInt(1),
						UnitPrice:       decimal.NewFromFloat(100.00),
						DiscountPercent: decimal.Zero,
						VATRate:         decimal.NewFromInt(22),
					},
				},
			},
			wantErr: "line description is required",
		},
		{
			name: "zero quantity",
			order: Order{
				ContactID: "contact-123",
				OrderDate: time.Now(),
				Lines: []OrderLine{
					{
						Description:     "Test",
						Quantity:        decimal.Zero,
						UnitPrice:       decimal.NewFromFloat(100.00),
						DiscountPercent: decimal.Zero,
						VATRate:         decimal.NewFromInt(22),
					},
				},
			},
			wantErr: "line quantity must be positive",
		},
		{
			name: "negative quantity",
			order: Order{
				ContactID: "contact-123",
				OrderDate: time.Now(),
				Lines: []OrderLine{
					{
						Description:     "Test",
						Quantity:        decimal.NewFromInt(-1),
						UnitPrice:       decimal.NewFromFloat(100.00),
						DiscountPercent: decimal.Zero,
						VATRate:         decimal.NewFromInt(22),
					},
				},
			},
			wantErr: "line quantity must be positive",
		},
		{
			name: "negative unit price",
			order: Order{
				ContactID: "contact-123",
				OrderDate: time.Now(),
				Lines: []OrderLine{
					{
						Description:     "Test",
						Quantity:        decimal.NewFromInt(1),
						UnitPrice:       decimal.NewFromInt(-100),
						DiscountPercent: decimal.Zero,
						VATRate:         decimal.NewFromInt(22),
					},
				},
			},
			wantErr: "line unit price cannot be negative",
		},
		{
			name: "negative VAT rate",
			order: Order{
				ContactID: "contact-123",
				OrderDate: time.Now(),
				Lines: []OrderLine{
					{
						Description:     "Test",
						Quantity:        decimal.NewFromInt(1),
						UnitPrice:       decimal.NewFromFloat(100.00),
						DiscountPercent: decimal.Zero,
						VATRate:         decimal.NewFromInt(-22),
					},
				},
			},
			wantErr: "line VAT rate cannot be negative",
		},
		{
			name: "negative discount",
			order: Order{
				ContactID: "contact-123",
				OrderDate: time.Now(),
				Lines: []OrderLine{
					{
						Description:     "Test",
						Quantity:        decimal.NewFromInt(1),
						UnitPrice:       decimal.NewFromFloat(100.00),
						DiscountPercent: decimal.NewFromInt(-10),
						VATRate:         decimal.NewFromInt(22),
					},
				},
			},
			wantErr: "line discount must be between 0 and 100",
		},
		{
			name: "discount over 100%",
			order: Order{
				ContactID: "contact-123",
				OrderDate: time.Now(),
				Lines: []OrderLine{
					{
						Description:     "Test",
						Quantity:        decimal.NewFromInt(1),
						UnitPrice:       decimal.NewFromFloat(100.00),
						DiscountPercent: decimal.NewFromInt(150),
						VATRate:         decimal.NewFromInt(22),
					},
				},
			},
			wantErr: "line discount must be between 0 and 100",
		},
		{
			name: "fixes line numbers",
			order: Order{
				ContactID: "contact-123",
				OrderDate: time.Now(),
				Lines: []OrderLine{
					{
						Description:     "First",
						Quantity:        decimal.NewFromInt(1),
						UnitPrice:       decimal.NewFromFloat(100.00),
						DiscountPercent: decimal.Zero,
						VATRate:         decimal.NewFromInt(22),
						LineNumber:      5, // Wrong number
					},
					{
						Description:     "Second",
						Quantity:        decimal.NewFromInt(1),
						UnitPrice:       decimal.NewFromFloat(50.00),
						DiscountPercent: decimal.Zero,
						VATRate:         decimal.NewFromInt(22),
						LineNumber:      10, // Wrong number
					},
				},
			},
			wantErr: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.order.Validate()

			if tt.wantErr == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			}
		})
	}
}

func TestOrderValidateFixesLineNumbers(t *testing.T) {
	order := Order{
		ContactID: "contact-123",
		OrderDate: time.Now(),
		Lines: []OrderLine{
			{
				Description:     "First",
				Quantity:        decimal.NewFromInt(1),
				UnitPrice:       decimal.NewFromFloat(100.00),
				DiscountPercent: decimal.Zero,
				VATRate:         decimal.NewFromInt(22),
				LineNumber:      99,
			},
			{
				Description:     "Second",
				Quantity:        decimal.NewFromInt(1),
				UnitPrice:       decimal.NewFromFloat(50.00),
				DiscountPercent: decimal.Zero,
				VATRate:         decimal.NewFromInt(22),
				LineNumber:      100,
			},
		},
	}

	err := order.Validate()
	require.NoError(t, err)

	// Validate should fix line numbers to 1, 2
	assert.Equal(t, 1, order.Lines[0].LineNumber)
	assert.Equal(t, 2, order.Lines[1].LineNumber)
}

func TestOrderStatusConstants(t *testing.T) {
	assert.Equal(t, OrderStatus("PENDING"), OrderStatusPending)
	assert.Equal(t, OrderStatus("CONFIRMED"), OrderStatusConfirmed)
	assert.Equal(t, OrderStatus("PROCESSING"), OrderStatusProcessing)
	assert.Equal(t, OrderStatus("SHIPPED"), OrderStatusShipped)
	assert.Equal(t, OrderStatus("DELIVERED"), OrderStatusDelivered)
	assert.Equal(t, OrderStatus("CANCELED"), OrderStatusCanceled)
}
