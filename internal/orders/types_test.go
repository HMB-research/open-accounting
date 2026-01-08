package orders

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOrderLine_Calculate(t *testing.T) {
	t.Run("calculates line totals without discount", func(t *testing.T) {
		line := OrderLine{
			Quantity:        decimal.NewFromInt(2),
			UnitPrice:       decimal.NewFromFloat(100.00),
			DiscountPercent: decimal.Zero,
			VATRate:         decimal.NewFromInt(20),
		}

		line.Calculate()

		assert.True(t, line.LineSubtotal.Equal(decimal.NewFromFloat(200.00)))
		assert.True(t, line.LineVAT.Equal(decimal.NewFromFloat(40.00)))
		assert.True(t, line.LineTotal.Equal(decimal.NewFromFloat(240.00)))
	})

	t.Run("calculates line totals with discount", func(t *testing.T) {
		line := OrderLine{
			Quantity:        decimal.NewFromInt(10),
			UnitPrice:       decimal.NewFromFloat(50.00),
			DiscountPercent: decimal.NewFromInt(10), // 10% discount
			VATRate:         decimal.NewFromInt(20),
		}

		line.Calculate()

		// 10 * 50 = 500, 10% discount = 450
		assert.True(t, line.LineSubtotal.Equal(decimal.NewFromFloat(450.00)))
		// 450 * 20% = 90
		assert.True(t, line.LineVAT.Equal(decimal.NewFromFloat(90.00)))
		// 450 + 90 = 540
		assert.True(t, line.LineTotal.Equal(decimal.NewFromFloat(540.00)))
	})

	t.Run("rounds to 2 decimal places", func(t *testing.T) {
		line := OrderLine{
			Quantity:        decimal.NewFromFloat(3),
			UnitPrice:       decimal.NewFromFloat(33.33),
			DiscountPercent: decimal.Zero,
			VATRate:         decimal.NewFromInt(20),
		}

		line.Calculate()

		// 3 * 33.33 = 99.99
		assert.True(t, line.LineSubtotal.Equal(decimal.NewFromFloat(99.99)))
	})
}

func TestOrder_Calculate(t *testing.T) {
	t.Run("calculates totals from lines", func(t *testing.T) {
		order := Order{
			Lines: []OrderLine{
				{
					Quantity:        decimal.NewFromInt(2),
					UnitPrice:       decimal.NewFromFloat(100.00),
					DiscountPercent: decimal.Zero,
					VATRate:         decimal.NewFromInt(20),
				},
				{
					Quantity:        decimal.NewFromInt(5),
					UnitPrice:       decimal.NewFromFloat(50.00),
					DiscountPercent: decimal.Zero,
					VATRate:         decimal.NewFromInt(20),
				},
			},
		}

		order.Calculate()

		// Line 1: 200 subtotal, 40 VAT
		// Line 2: 250 subtotal, 50 VAT
		// Total: 450 subtotal, 90 VAT, 540 total
		assert.True(t, order.Subtotal.Equal(decimal.NewFromFloat(450.00)))
		assert.True(t, order.VATAmount.Equal(decimal.NewFromFloat(90.00)))
		assert.True(t, order.Total.Equal(decimal.NewFromFloat(540.00)))
	})

	t.Run("handles empty lines", func(t *testing.T) {
		order := Order{
			Lines: []OrderLine{},
		}

		order.Calculate()

		assert.True(t, order.Subtotal.IsZero())
		assert.True(t, order.VATAmount.IsZero())
		assert.True(t, order.Total.IsZero())
	})
}

func TestOrder_Validate(t *testing.T) {
	validOrder := func() Order {
		return Order{
			ContactID: "contact-1",
			OrderDate: time.Now(),
			Lines: []OrderLine{
				{
					Description:     "Test item",
					Quantity:        decimal.NewFromInt(1),
					UnitPrice:       decimal.NewFromFloat(100.00),
					VATRate:         decimal.NewFromInt(20),
					DiscountPercent: decimal.Zero,
				},
			},
		}
	}

	t.Run("valid order passes validation", func(t *testing.T) {
		order := validOrder()
		err := order.Validate()
		require.NoError(t, err)
	})

	t.Run("returns error when no lines", func(t *testing.T) {
		order := validOrder()
		order.Lines = []OrderLine{}

		err := order.Validate()
		assert.EqualError(t, err, "order must have at least one line")
	})

	t.Run("returns error when contact missing", func(t *testing.T) {
		order := validOrder()
		order.ContactID = ""

		err := order.Validate()
		assert.EqualError(t, err, "contact is required")
	})

	t.Run("returns error when order date missing", func(t *testing.T) {
		order := validOrder()
		order.OrderDate = time.Time{}

		err := order.Validate()
		assert.EqualError(t, err, "order date is required")
	})

	t.Run("returns error when line description empty", func(t *testing.T) {
		order := validOrder()
		order.Lines[0].Description = ""

		err := order.Validate()
		assert.EqualError(t, err, "line description is required")
	})

	t.Run("returns error when quantity zero or negative", func(t *testing.T) {
		order := validOrder()
		order.Lines[0].Quantity = decimal.Zero

		err := order.Validate()
		assert.EqualError(t, err, "line quantity must be positive")
	})

	t.Run("returns error when unit price negative", func(t *testing.T) {
		order := validOrder()
		order.Lines[0].UnitPrice = decimal.NewFromFloat(-1)

		err := order.Validate()
		assert.EqualError(t, err, "line unit price cannot be negative")
	})

	t.Run("returns error when VAT rate negative", func(t *testing.T) {
		order := validOrder()
		order.Lines[0].VATRate = decimal.NewFromFloat(-1)

		err := order.Validate()
		assert.EqualError(t, err, "line VAT rate cannot be negative")
	})

	t.Run("returns error when discount below 0", func(t *testing.T) {
		order := validOrder()
		order.Lines[0].DiscountPercent = decimal.NewFromFloat(-5)

		err := order.Validate()
		assert.EqualError(t, err, "line discount must be between 0 and 100")
	})

	t.Run("returns error when discount above 100", func(t *testing.T) {
		order := validOrder()
		order.Lines[0].DiscountPercent = decimal.NewFromInt(101)

		err := order.Validate()
		assert.EqualError(t, err, "line discount must be between 0 and 100")
	})

	t.Run("corrects line numbers during validation", func(t *testing.T) {
		order := validOrder()
		order.Lines[0].LineNumber = 99

		err := order.Validate()
		require.NoError(t, err)
		assert.Equal(t, 1, order.Lines[0].LineNumber)
	})
}
