package quotes

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQuoteLine_Calculate(t *testing.T) {
	t.Run("calculates line totals without discount", func(t *testing.T) {
		line := QuoteLine{
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
		line := QuoteLine{
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
		line := QuoteLine{
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

func TestQuote_Calculate(t *testing.T) {
	t.Run("calculates totals from lines", func(t *testing.T) {
		quote := Quote{
			Lines: []QuoteLine{
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

		quote.Calculate()

		// Line 1: 200 subtotal, 40 VAT
		// Line 2: 250 subtotal, 50 VAT
		// Total: 450 subtotal, 90 VAT, 540 total
		assert.True(t, quote.Subtotal.Equal(decimal.NewFromFloat(450.00)))
		assert.True(t, quote.VATAmount.Equal(decimal.NewFromFloat(90.00)))
		assert.True(t, quote.Total.Equal(decimal.NewFromFloat(540.00)))
	})

	t.Run("handles empty lines", func(t *testing.T) {
		quote := Quote{
			Lines: []QuoteLine{},
		}

		quote.Calculate()

		assert.True(t, quote.Subtotal.IsZero())
		assert.True(t, quote.VATAmount.IsZero())
		assert.True(t, quote.Total.IsZero())
	})
}

func TestQuote_Validate(t *testing.T) {
	validQuote := func() Quote {
		return Quote{
			ContactID: "contact-1",
			QuoteDate: time.Now(),
			Lines: []QuoteLine{
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

	t.Run("valid quote passes validation", func(t *testing.T) {
		quote := validQuote()
		err := quote.Validate()
		require.NoError(t, err)
	})

	t.Run("returns error when no lines", func(t *testing.T) {
		quote := validQuote()
		quote.Lines = []QuoteLine{}

		err := quote.Validate()
		assert.EqualError(t, err, "quote must have at least one line")
	})

	t.Run("returns error when contact missing", func(t *testing.T) {
		quote := validQuote()
		quote.ContactID = ""

		err := quote.Validate()
		assert.EqualError(t, err, "contact is required")
	})

	t.Run("returns error when quote date missing", func(t *testing.T) {
		quote := validQuote()
		quote.QuoteDate = time.Time{}

		err := quote.Validate()
		assert.EqualError(t, err, "quote date is required")
	})

	t.Run("returns error when valid until before quote date", func(t *testing.T) {
		quote := validQuote()
		validUntil := quote.QuoteDate.AddDate(0, 0, -1)
		quote.ValidUntil = &validUntil

		err := quote.Validate()
		assert.EqualError(t, err, "valid until date cannot be before quote date")
	})

	t.Run("returns error when line description empty", func(t *testing.T) {
		quote := validQuote()
		quote.Lines[0].Description = ""

		err := quote.Validate()
		assert.EqualError(t, err, "line description is required")
	})

	t.Run("returns error when quantity zero or negative", func(t *testing.T) {
		quote := validQuote()
		quote.Lines[0].Quantity = decimal.Zero

		err := quote.Validate()
		assert.EqualError(t, err, "line quantity must be positive")
	})

	t.Run("returns error when unit price negative", func(t *testing.T) {
		quote := validQuote()
		quote.Lines[0].UnitPrice = decimal.NewFromFloat(-1)

		err := quote.Validate()
		assert.EqualError(t, err, "line unit price cannot be negative")
	})

	t.Run("returns error when VAT rate negative", func(t *testing.T) {
		quote := validQuote()
		quote.Lines[0].VATRate = decimal.NewFromFloat(-1)

		err := quote.Validate()
		assert.EqualError(t, err, "line VAT rate cannot be negative")
	})

	t.Run("returns error when discount below 0", func(t *testing.T) {
		quote := validQuote()
		quote.Lines[0].DiscountPercent = decimal.NewFromFloat(-5)

		err := quote.Validate()
		assert.EqualError(t, err, "line discount must be between 0 and 100")
	})

	t.Run("returns error when discount above 100", func(t *testing.T) {
		quote := validQuote()
		quote.Lines[0].DiscountPercent = decimal.NewFromInt(101)

		err := quote.Validate()
		assert.EqualError(t, err, "line discount must be between 0 and 100")
	})

	t.Run("corrects line numbers during validation", func(t *testing.T) {
		quote := validQuote()
		quote.Lines[0].LineNumber = 99

		err := quote.Validate()
		require.NoError(t, err)
		assert.Equal(t, 1, quote.Lines[0].LineNumber)
	})
}

func TestQuote_IsExpired(t *testing.T) {
	t.Run("returns false when no valid until date", func(t *testing.T) {
		quote := Quote{
			ValidUntil: nil,
			Status:     QuoteStatusDraft,
		}

		assert.False(t, quote.IsExpired())
	})

	t.Run("returns false when valid until is in future", func(t *testing.T) {
		future := time.Now().AddDate(0, 0, 30)
		quote := Quote{
			ValidUntil: &future,
			Status:     QuoteStatusDraft,
		}

		assert.False(t, quote.IsExpired())
	})

	t.Run("returns true when valid until is in past", func(t *testing.T) {
		past := time.Now().AddDate(0, 0, -1)
		quote := Quote{
			ValidUntil: &past,
			Status:     QuoteStatusDraft,
		}

		assert.True(t, quote.IsExpired())
	})

	t.Run("returns false when converted even if past valid date", func(t *testing.T) {
		past := time.Now().AddDate(0, 0, -1)
		quote := Quote{
			ValidUntil: &past,
			Status:     QuoteStatusConverted,
		}

		assert.False(t, quote.IsExpired())
	})

	t.Run("returns false when accepted even if past valid date", func(t *testing.T) {
		past := time.Now().AddDate(0, 0, -1)
		quote := Quote{
			ValidUntil: &past,
			Status:     QuoteStatusAccepted,
		}

		assert.False(t, quote.IsExpired())
	})
}
