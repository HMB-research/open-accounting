package payments

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
)

func TestPaymentType_Values(t *testing.T) {
	assert.Equal(t, "RECEIVED", string(PaymentTypeReceived))
	assert.Equal(t, "MADE", string(PaymentTypeMade))
}

func TestPayment_TotalAllocated(t *testing.T) {
	t.Run("no allocations", func(t *testing.T) {
		payment := Payment{
			Amount:      decimal.NewFromFloat(1000),
			Allocations: []PaymentAllocation{},
		}

		assert.True(t, payment.TotalAllocated().IsZero())
	})

	t.Run("single allocation", func(t *testing.T) {
		payment := Payment{
			Amount: decimal.NewFromFloat(1000),
			Allocations: []PaymentAllocation{
				{Amount: decimal.NewFromFloat(500)},
			},
		}

		assert.True(t, payment.TotalAllocated().Equal(decimal.NewFromFloat(500)))
	})

	t.Run("multiple allocations", func(t *testing.T) {
		payment := Payment{
			Amount: decimal.NewFromFloat(1000),
			Allocations: []PaymentAllocation{
				{Amount: decimal.NewFromFloat(300)},
				{Amount: decimal.NewFromFloat(250)},
				{Amount: decimal.NewFromFloat(150)},
			},
		}

		assert.True(t, payment.TotalAllocated().Equal(decimal.NewFromFloat(700)))
	})

	t.Run("fully allocated", func(t *testing.T) {
		payment := Payment{
			Amount: decimal.NewFromFloat(500),
			Allocations: []PaymentAllocation{
				{Amount: decimal.NewFromFloat(200)},
				{Amount: decimal.NewFromFloat(300)},
			},
		}

		assert.True(t, payment.TotalAllocated().Equal(decimal.NewFromFloat(500)))
	})
}

func TestPayment_UnallocatedAmount(t *testing.T) {
	t.Run("no allocations", func(t *testing.T) {
		payment := Payment{
			Amount:      decimal.NewFromFloat(1000),
			Allocations: []PaymentAllocation{},
		}

		assert.True(t, payment.UnallocatedAmount().Equal(decimal.NewFromFloat(1000)))
	})

	t.Run("partially allocated", func(t *testing.T) {
		payment := Payment{
			Amount: decimal.NewFromFloat(1000),
			Allocations: []PaymentAllocation{
				{Amount: decimal.NewFromFloat(300)},
				{Amount: decimal.NewFromFloat(200)},
			},
		}

		assert.True(t, payment.UnallocatedAmount().Equal(decimal.NewFromFloat(500)))
	})

	t.Run("fully allocated", func(t *testing.T) {
		payment := Payment{
			Amount: decimal.NewFromFloat(500),
			Allocations: []PaymentAllocation{
				{Amount: decimal.NewFromFloat(500)},
			},
		}

		assert.True(t, payment.UnallocatedAmount().IsZero())
	})
}

func TestPayment_Defaults(t *testing.T) {
	payment := Payment{
		ID:            "pay-123",
		TenantID:      "tenant-456",
		PaymentNumber: "PMT-00001",
		PaymentType:   PaymentTypeReceived,
		Amount:        decimal.NewFromFloat(500),
		Currency:      "EUR",
		ExchangeRate:  decimal.NewFromInt(1),
	}

	assert.Equal(t, "pay-123", payment.ID)
	assert.Equal(t, "PMT-00001", payment.PaymentNumber)
	assert.Equal(t, PaymentTypeReceived, payment.PaymentType)
	assert.Equal(t, "EUR", payment.Currency)
	assert.True(t, payment.Amount.Equal(decimal.NewFromFloat(500)))
}

func TestPayment_WithExchangeRate(t *testing.T) {
	payment := Payment{
		Amount:       decimal.NewFromFloat(100),
		Currency:     "USD",
		ExchangeRate: decimal.NewFromFloat(0.92),
		BaseAmount:   decimal.NewFromFloat(92),
	}

	assert.True(t, payment.Amount.Equal(decimal.NewFromFloat(100)))
	assert.True(t, payment.ExchangeRate.Equal(decimal.NewFromFloat(0.92)))
	assert.True(t, payment.BaseAmount.Equal(decimal.NewFromFloat(92)))
}

func TestPaymentAllocation_Fields(t *testing.T) {
	now := time.Now()
	alloc := PaymentAllocation{
		ID:        "alloc-123",
		TenantID:  "tenant-456",
		PaymentID: "pay-789",
		InvoiceID: "inv-012",
		Amount:    decimal.NewFromFloat(250),
		CreatedAt: now,
	}

	assert.Equal(t, "alloc-123", alloc.ID)
	assert.Equal(t, "tenant-456", alloc.TenantID)
	assert.Equal(t, "pay-789", alloc.PaymentID)
	assert.Equal(t, "inv-012", alloc.InvoiceID)
	assert.True(t, alloc.Amount.Equal(decimal.NewFromFloat(250)))
	assert.Equal(t, now, alloc.CreatedAt)
}

func TestCreatePaymentRequest_Minimal(t *testing.T) {
	req := CreatePaymentRequest{
		PaymentType: PaymentTypeReceived,
		Amount:      decimal.NewFromFloat(1000),
		PaymentDate: time.Now(),
	}

	assert.Equal(t, PaymentTypeReceived, req.PaymentType)
	assert.True(t, req.Amount.Equal(decimal.NewFromFloat(1000)))
	assert.Nil(t, req.ContactID)
	assert.Equal(t, "", req.Currency)
}

func TestCreatePaymentRequest_Full(t *testing.T) {
	contactID := "contact-123"
	req := CreatePaymentRequest{
		PaymentType:   PaymentTypeMade,
		ContactID:     &contactID,
		PaymentDate:   time.Now(),
		Amount:        decimal.NewFromFloat(5000),
		Currency:      "USD",
		ExchangeRate:  decimal.NewFromFloat(0.92),
		PaymentMethod: "BANK_TRANSFER",
		BankAccount:   "EE123456789012345678",
		Reference:     "REF-001",
		Notes:         "Quarterly payment",
		Allocations: []AllocationRequest{
			{InvoiceID: "inv-001", Amount: decimal.NewFromFloat(2000)},
			{InvoiceID: "inv-002", Amount: decimal.NewFromFloat(3000)},
		},
	}

	assert.Equal(t, PaymentTypeMade, req.PaymentType)
	assert.Equal(t, "contact-123", *req.ContactID)
	assert.Equal(t, "USD", req.Currency)
	assert.Equal(t, "BANK_TRANSFER", req.PaymentMethod)
	assert.Len(t, req.Allocations, 2)
}

func TestAllocationRequest_Fields(t *testing.T) {
	alloc := AllocationRequest{
		InvoiceID: "inv-123",
		Amount:    decimal.NewFromFloat(500),
	}

	assert.Equal(t, "inv-123", alloc.InvoiceID)
	assert.True(t, alloc.Amount.Equal(decimal.NewFromFloat(500)))
}

func TestPaymentFilter_Fields(t *testing.T) {
	now := time.Now()
	filter := PaymentFilter{
		PaymentType: PaymentTypeReceived,
		ContactID:   "contact-123",
		FromDate:    &now,
		ToDate:      &now,
	}

	assert.Equal(t, PaymentTypeReceived, filter.PaymentType)
	assert.Equal(t, "contact-123", filter.ContactID)
	assert.Equal(t, now, *filter.FromDate)
	assert.Equal(t, now, *filter.ToDate)
}

func TestPayment_PaymentMade(t *testing.T) {
	payment := Payment{
		ID:            "pay-out-123",
		PaymentNumber: "OUT-00001",
		PaymentType:   PaymentTypeMade,
		Amount:        decimal.NewFromFloat(2500),
		Currency:      "EUR",
		PaymentMethod: "WIRE",
	}

	assert.Equal(t, PaymentTypeMade, payment.PaymentType)
	assert.Equal(t, "OUT-00001", payment.PaymentNumber)
	assert.Equal(t, "WIRE", payment.PaymentMethod)
}

func TestPayment_PrecisionHandling(t *testing.T) {
	t.Run("high precision amounts", func(t *testing.T) {
		payment := Payment{
			Amount: decimal.RequireFromString("12345.67890123"),
			Allocations: []PaymentAllocation{
				{Amount: decimal.RequireFromString("1234.56789012")},
				{Amount: decimal.RequireFromString("2345.67890123")},
			},
		}

		expected := decimal.RequireFromString("3580.24679135")
		assert.True(t, payment.TotalAllocated().Equal(expected))

		unallocatedExpected := decimal.RequireFromString("8765.43210988")
		assert.True(t, payment.UnallocatedAmount().Equal(unallocatedExpected))
	})
}

func TestPayment_EmptyAllocations(t *testing.T) {
	payment := Payment{
		Amount: decimal.NewFromFloat(1000),
	}

	// Allocations is nil, not empty slice
	assert.True(t, payment.TotalAllocated().IsZero())
	assert.True(t, payment.UnallocatedAmount().Equal(decimal.NewFromFloat(1000)))
}
