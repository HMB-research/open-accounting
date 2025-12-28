package invoicing

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInvoiceLine_Calculate(t *testing.T) {
	t.Run("simple calculation without discount", func(t *testing.T) {
		line := InvoiceLine{
			Quantity:        decimal.NewFromInt(2),
			UnitPrice:       decimal.NewFromFloat(100),
			DiscountPercent: decimal.Zero,
			VATRate:         decimal.NewFromInt(22),
		}

		line.Calculate()

		assert.True(t, line.LineSubtotal.Equal(decimal.NewFromFloat(200)))
		assert.True(t, line.LineVAT.Equal(decimal.NewFromFloat(44)))
		assert.True(t, line.LineTotal.Equal(decimal.NewFromFloat(244)))
	})

	t.Run("calculation with discount", func(t *testing.T) {
		line := InvoiceLine{
			Quantity:        decimal.NewFromInt(10),
			UnitPrice:       decimal.NewFromFloat(50),
			DiscountPercent: decimal.NewFromInt(10), // 10% discount
			VATRate:         decimal.NewFromInt(22),
		}

		line.Calculate()

		// 10 * 50 = 500, minus 10% = 450
		assert.True(t, line.LineSubtotal.Equal(decimal.NewFromFloat(450)))
		// 450 * 22% = 99
		assert.True(t, line.LineVAT.Equal(decimal.NewFromFloat(99)))
		// 450 + 99 = 549
		assert.True(t, line.LineTotal.Equal(decimal.NewFromFloat(549)))
	})

	t.Run("fractional quantities", func(t *testing.T) {
		line := InvoiceLine{
			Quantity:        decimal.NewFromFloat(1.5),
			UnitPrice:       decimal.NewFromFloat(33.33),
			DiscountPercent: decimal.Zero,
			VATRate:         decimal.NewFromInt(9),
		}

		line.Calculate()

		// 1.5 * 33.33 = 49.995, rounded to 50.00
		assert.True(t, line.LineSubtotal.Equal(decimal.NewFromFloat(50)))
		// 50 * 9% = 4.50
		assert.True(t, line.LineVAT.Equal(decimal.NewFromFloat(4.50)))
		// 50 + 4.50 = 54.50
		assert.True(t, line.LineTotal.Equal(decimal.NewFromFloat(54.50)))
	})

	t.Run("zero VAT", func(t *testing.T) {
		line := InvoiceLine{
			Quantity:        decimal.NewFromInt(1),
			UnitPrice:       decimal.NewFromFloat(100),
			DiscountPercent: decimal.Zero,
			VATRate:         decimal.Zero, // 0% VAT (export, etc.)
		}

		line.Calculate()

		assert.True(t, line.LineSubtotal.Equal(decimal.NewFromFloat(100)))
		assert.True(t, line.LineVAT.IsZero())
		assert.True(t, line.LineTotal.Equal(decimal.NewFromFloat(100)))
	})
}

func TestInvoice_Calculate(t *testing.T) {
	t.Run("multiple lines", func(t *testing.T) {
		inv := Invoice{
			ExchangeRate: decimal.NewFromInt(1),
			Lines: []InvoiceLine{
				{
					Quantity:        decimal.NewFromInt(2),
					UnitPrice:       decimal.NewFromFloat(100),
					DiscountPercent: decimal.Zero,
					VATRate:         decimal.NewFromInt(22),
				},
				{
					Quantity:        decimal.NewFromInt(1),
					UnitPrice:       decimal.NewFromFloat(50),
					DiscountPercent: decimal.Zero,
					VATRate:         decimal.NewFromInt(22),
				},
			},
		}

		inv.Calculate()

		// Line 1: 200 + 44 = 244
		// Line 2: 50 + 11 = 61
		assert.True(t, inv.Subtotal.Equal(decimal.NewFromFloat(250)))
		assert.True(t, inv.VATAmount.Equal(decimal.NewFromFloat(55)))
		assert.True(t, inv.Total.Equal(decimal.NewFromFloat(305)))
	})

	t.Run("with exchange rate", func(t *testing.T) {
		inv := Invoice{
			ExchangeRate: decimal.NewFromFloat(0.92), // USD to EUR
			Lines: []InvoiceLine{
				{
					Quantity:        decimal.NewFromInt(1),
					UnitPrice:       decimal.NewFromFloat(100),
					DiscountPercent: decimal.Zero,
					VATRate:         decimal.NewFromInt(22),
				},
			},
		}

		inv.Calculate()

		assert.True(t, inv.Subtotal.Equal(decimal.NewFromFloat(100)))
		assert.True(t, inv.VATAmount.Equal(decimal.NewFromFloat(22)))
		assert.True(t, inv.Total.Equal(decimal.NewFromFloat(122)))

		// Base amounts in EUR
		assert.True(t, inv.BaseSubtotal.Equal(decimal.NewFromFloat(92)))
		assert.True(t, inv.BaseVATAmount.Equal(decimal.NewFromFloat(20.24)))
		assert.True(t, inv.BaseTotal.Equal(decimal.NewFromFloat(112.24)))
	})
}

func TestInvoice_Validate(t *testing.T) {
	baseInvoice := func() *Invoice {
		return &Invoice{
			ContactID: "contact-123",
			IssueDate: time.Now(),
			DueDate:   time.Now().AddDate(0, 0, 14),
			Lines: []InvoiceLine{
				{
					Description: "Test item",
					Quantity:    decimal.NewFromInt(1),
					UnitPrice:   decimal.NewFromFloat(100),
					VATRate:     decimal.NewFromInt(22),
				},
			},
		}
	}

	t.Run("valid invoice", func(t *testing.T) {
		inv := baseInvoice()
		err := inv.Validate()
		assert.NoError(t, err)
	})

	t.Run("no lines", func(t *testing.T) {
		inv := baseInvoice()
		inv.Lines = []InvoiceLine{}

		err := inv.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "at least one line")
	})

	t.Run("no contact", func(t *testing.T) {
		inv := baseInvoice()
		inv.ContactID = ""

		err := inv.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "contact")
	})

	t.Run("no issue date", func(t *testing.T) {
		inv := baseInvoice()
		inv.IssueDate = time.Time{}

		err := inv.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "issue date")
	})

	t.Run("due date before issue date", func(t *testing.T) {
		inv := baseInvoice()
		inv.DueDate = inv.IssueDate.AddDate(0, 0, -1)

		err := inv.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "due date")
	})

	t.Run("empty line description", func(t *testing.T) {
		inv := baseInvoice()
		inv.Lines[0].Description = ""

		err := inv.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "description")
	})

	t.Run("zero quantity", func(t *testing.T) {
		inv := baseInvoice()
		inv.Lines[0].Quantity = decimal.Zero

		err := inv.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "quantity")
	})

	t.Run("negative quantity", func(t *testing.T) {
		inv := baseInvoice()
		inv.Lines[0].Quantity = decimal.NewFromInt(-1)

		err := inv.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "quantity")
	})

	t.Run("negative unit price", func(t *testing.T) {
		inv := baseInvoice()
		inv.Lines[0].UnitPrice = decimal.NewFromInt(-100)

		err := inv.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unit price")
	})

	t.Run("negative VAT rate", func(t *testing.T) {
		inv := baseInvoice()
		inv.Lines[0].VATRate = decimal.NewFromInt(-1)

		err := inv.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "VAT rate")
	})

	t.Run("discount over 100", func(t *testing.T) {
		inv := baseInvoice()
		inv.Lines[0].DiscountPercent = decimal.NewFromInt(101)

		err := inv.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "discount")
	})

	t.Run("negative discount", func(t *testing.T) {
		inv := baseInvoice()
		inv.Lines[0].DiscountPercent = decimal.NewFromInt(-5)

		err := inv.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "discount")
	})
}

func TestInvoice_AmountDue(t *testing.T) {
	t.Run("unpaid invoice", func(t *testing.T) {
		inv := Invoice{
			Total:      decimal.NewFromFloat(122),
			AmountPaid: decimal.Zero,
		}

		assert.True(t, inv.AmountDue().Equal(decimal.NewFromFloat(122)))
	})

	t.Run("partially paid", func(t *testing.T) {
		inv := Invoice{
			Total:      decimal.NewFromFloat(122),
			AmountPaid: decimal.NewFromFloat(50),
		}

		assert.True(t, inv.AmountDue().Equal(decimal.NewFromFloat(72)))
	})

	t.Run("fully paid", func(t *testing.T) {
		inv := Invoice{
			Total:      decimal.NewFromFloat(122),
			AmountPaid: decimal.NewFromFloat(122),
		}

		assert.True(t, inv.AmountDue().IsZero())
	})

	t.Run("overpaid", func(t *testing.T) {
		inv := Invoice{
			Total:      decimal.NewFromFloat(100),
			AmountPaid: decimal.NewFromFloat(120),
		}

		// Returns negative when overpaid
		assert.True(t, inv.AmountDue().Equal(decimal.NewFromFloat(-20)))
	})
}

func TestInvoice_IsPaid(t *testing.T) {
	t.Run("not paid", func(t *testing.T) {
		inv := Invoice{
			Total:      decimal.NewFromFloat(100),
			AmountPaid: decimal.NewFromFloat(50),
		}
		assert.False(t, inv.IsPaid())
	})

	t.Run("exactly paid", func(t *testing.T) {
		inv := Invoice{
			Total:      decimal.NewFromFloat(100),
			AmountPaid: decimal.NewFromFloat(100),
		}
		assert.True(t, inv.IsPaid())
	})

	t.Run("overpaid", func(t *testing.T) {
		inv := Invoice{
			Total:      decimal.NewFromFloat(100),
			AmountPaid: decimal.NewFromFloat(150),
		}
		assert.True(t, inv.IsPaid())
	})
}

func TestInvoice_IsOverdue(t *testing.T) {
	t.Run("not overdue - future due date", func(t *testing.T) {
		inv := Invoice{
			Total:      decimal.NewFromFloat(100),
			AmountPaid: decimal.Zero,
			DueDate:    time.Now().AddDate(0, 0, 7),
			Status:     StatusSent,
		}
		assert.False(t, inv.IsOverdue())
	})

	t.Run("overdue - past due date", func(t *testing.T) {
		inv := Invoice{
			Total:      decimal.NewFromFloat(100),
			AmountPaid: decimal.Zero,
			DueDate:    time.Now().AddDate(0, 0, -7),
			Status:     StatusSent,
		}
		assert.True(t, inv.IsOverdue())
	})

	t.Run("not overdue - paid", func(t *testing.T) {
		inv := Invoice{
			Total:      decimal.NewFromFloat(100),
			AmountPaid: decimal.NewFromFloat(100),
			DueDate:    time.Now().AddDate(0, 0, -7),
			Status:     StatusPaid,
		}
		assert.False(t, inv.IsOverdue())
	})

	t.Run("not overdue - voided", func(t *testing.T) {
		inv := Invoice{
			Total:      decimal.NewFromFloat(100),
			AmountPaid: decimal.Zero,
			DueDate:    time.Now().AddDate(0, 0, -7),
			Status:     StatusVoided,
		}
		assert.False(t, inv.IsOverdue())
	})
}

func TestInvoiceType_Values(t *testing.T) {
	assert.Equal(t, "SALES", string(InvoiceTypeSales))
	assert.Equal(t, "PURCHASE", string(InvoiceTypePurchase))
	assert.Equal(t, "CREDIT_NOTE", string(InvoiceTypeCreditNote))
}

func TestInvoiceStatus_Values(t *testing.T) {
	assert.Equal(t, "DRAFT", string(StatusDraft))
	assert.Equal(t, "SENT", string(StatusSent))
	assert.Equal(t, "PARTIALLY_PAID", string(StatusPartiallyPaid))
	assert.Equal(t, "PAID", string(StatusPaid))
	assert.Equal(t, "OVERDUE", string(StatusOverdue))
	assert.Equal(t, "VOIDED", string(StatusVoided))
}

func TestInvoice_LineNumbersSequential(t *testing.T) {
	inv := Invoice{
		ContactID: "contact-123",
		IssueDate: time.Now(),
		DueDate:   time.Now().AddDate(0, 0, 14),
		Lines: []InvoiceLine{
			{Description: "Item 1", Quantity: decimal.NewFromInt(1), UnitPrice: decimal.NewFromFloat(100)},
			{Description: "Item 2", Quantity: decimal.NewFromInt(1), UnitPrice: decimal.NewFromFloat(200)},
			{Description: "Item 3", Quantity: decimal.NewFromInt(1), UnitPrice: decimal.NewFromFloat(300)},
		},
	}

	// Validate should fix line numbers
	err := inv.Validate()
	assert.NoError(t, err)

	assert.Equal(t, 1, inv.Lines[0].LineNumber)
	assert.Equal(t, 2, inv.Lines[1].LineNumber)
	assert.Equal(t, 3, inv.Lines[2].LineNumber)
}

func TestCreateInvoiceRequest_Defaults(t *testing.T) {
	req := CreateInvoiceRequest{
		InvoiceType: InvoiceTypeSales,
		ContactID:   "contact-123",
		IssueDate:   time.Now(),
		DueDate:     time.Now().AddDate(0, 0, 14),
		Lines: []CreateInvoiceLineRequest{
			{
				Description: "Service",
				Quantity:    decimal.NewFromInt(1),
				UnitPrice:   decimal.NewFromFloat(500),
				VATRate:     decimal.NewFromInt(22),
			},
		},
	}

	assert.Equal(t, InvoiceTypeSales, req.InvoiceType)
	assert.Equal(t, "contact-123", req.ContactID)
	assert.Len(t, req.Lines, 1)
	assert.True(t, req.Lines[0].Quantity.Equal(decimal.NewFromInt(1)))
}
