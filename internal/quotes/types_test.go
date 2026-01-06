package quotes

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQuoteLineCalculate(t *testing.T) {
	tests := []struct {
		name         string
		line         QuoteLine
		wantSubtotal string
		wantVAT      string
		wantTotal    string
	}{
		{
			name: "simple calculation no discount",
			line: QuoteLine{
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
			line: QuoteLine{
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
			line: QuoteLine{
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
			line: QuoteLine{
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
			line: QuoteLine{
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

func TestQuoteCalculate(t *testing.T) {
	tests := []struct {
		name         string
		lines        []QuoteLine
		wantSubtotal string
		wantVAT      string
		wantTotal    string
	}{
		{
			name: "single line",
			lines: []QuoteLine{
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
			lines: []QuoteLine{
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
			wantSubtotal: "280",
			wantVAT:      "61.6",
			wantTotal:    "341.6",
		},
		{
			name:         "empty lines",
			lines:        []QuoteLine{},
			wantSubtotal: "0",
			wantVAT:      "0",
			wantTotal:    "0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			quote := &Quote{Lines: tt.lines}
			quote.Calculate()

			wantSubtotal, _ := decimal.NewFromString(tt.wantSubtotal)
			wantVAT, _ := decimal.NewFromString(tt.wantVAT)
			wantTotal, _ := decimal.NewFromString(tt.wantTotal)

			assert.True(t, quote.Subtotal.Equal(wantSubtotal), "subtotal: got %s, want %s", quote.Subtotal, wantSubtotal)
			assert.True(t, quote.VATAmount.Equal(wantVAT), "VAT: got %s, want %s", quote.VATAmount, wantVAT)
			assert.True(t, quote.Total.Equal(wantTotal), "total: got %s, want %s", quote.Total, wantTotal)
		})
	}
}

func TestQuoteValidate(t *testing.T) {
	validLine := QuoteLine{
		Description:     "Test product",
		Quantity:        decimal.NewFromInt(1),
		UnitPrice:       decimal.NewFromFloat(100.00),
		DiscountPercent: decimal.Zero,
		VATRate:         decimal.NewFromInt(22),
		LineNumber:      1,
	}

	futureDate := time.Now().AddDate(0, 1, 0)

	tests := []struct {
		name    string
		quote   Quote
		wantErr string
	}{
		{
			name: "valid quote",
			quote: Quote{
				ContactID:  "contact-123",
				QuoteDate:  time.Now(),
				ValidUntil: &futureDate,
				Lines:      []QuoteLine{validLine},
			},
			wantErr: "",
		},
		{
			name: "valid quote without valid_until",
			quote: Quote{
				ContactID:  "contact-123",
				QuoteDate:  time.Now(),
				ValidUntil: nil,
				Lines:      []QuoteLine{validLine},
			},
			wantErr: "",
		},
		{
			name: "no lines",
			quote: Quote{
				ContactID: "contact-123",
				QuoteDate: time.Now(),
				Lines:     []QuoteLine{},
			},
			wantErr: "quote must have at least one line",
		},
		{
			name: "missing contact",
			quote: Quote{
				ContactID: "",
				QuoteDate: time.Now(),
				Lines:     []QuoteLine{validLine},
			},
			wantErr: "contact is required",
		},
		{
			name: "missing quote date",
			quote: Quote{
				ContactID: "contact-123",
				QuoteDate: time.Time{},
				Lines:     []QuoteLine{validLine},
			},
			wantErr: "quote date is required",
		},
		{
			name: "valid_until before quote_date",
			quote: Quote{
				ContactID:  "contact-123",
				QuoteDate:  time.Now(),
				ValidUntil: func() *time.Time { t := time.Now().AddDate(0, 0, -1); return &t }(),
				Lines:      []QuoteLine{validLine},
			},
			wantErr: "valid until date cannot be before quote date",
		},
		{
			name: "missing line description",
			quote: Quote{
				ContactID: "contact-123",
				QuoteDate: time.Now(),
				Lines: []QuoteLine{
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
			quote: Quote{
				ContactID: "contact-123",
				QuoteDate: time.Now(),
				Lines: []QuoteLine{
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
			quote: Quote{
				ContactID: "contact-123",
				QuoteDate: time.Now(),
				Lines: []QuoteLine{
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
			quote: Quote{
				ContactID: "contact-123",
				QuoteDate: time.Now(),
				Lines: []QuoteLine{
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
			quote: Quote{
				ContactID: "contact-123",
				QuoteDate: time.Now(),
				Lines: []QuoteLine{
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
			quote: Quote{
				ContactID: "contact-123",
				QuoteDate: time.Now(),
				Lines: []QuoteLine{
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
			quote: Quote{
				ContactID: "contact-123",
				QuoteDate: time.Now(),
				Lines: []QuoteLine{
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.quote.Validate()

			if tt.wantErr == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			}
		})
	}
}

func TestQuoteValidateFixesLineNumbers(t *testing.T) {
	quote := Quote{
		ContactID: "contact-123",
		QuoteDate: time.Now(),
		Lines: []QuoteLine{
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

	err := quote.Validate()
	require.NoError(t, err)

	assert.Equal(t, 1, quote.Lines[0].LineNumber)
	assert.Equal(t, 2, quote.Lines[1].LineNumber)
}

func TestQuoteIsExpired(t *testing.T) {
	yesterday := time.Now().AddDate(0, 0, -1)
	tomorrow := time.Now().AddDate(0, 0, 1)

	tests := []struct {
		name    string
		quote   Quote
		expired bool
	}{
		{
			name: "no valid_until - never expires",
			quote: Quote{
				ValidUntil: nil,
				Status:     QuoteStatusSent,
			},
			expired: false,
		},
		{
			name: "valid_until in future - not expired",
			quote: Quote{
				ValidUntil: &tomorrow,
				Status:     QuoteStatusSent,
			},
			expired: false,
		},
		{
			name: "valid_until in past - expired",
			quote: Quote{
				ValidUntil: &yesterday,
				Status:     QuoteStatusSent,
			},
			expired: true,
		},
		{
			name: "valid_until in past but already converted - not expired",
			quote: Quote{
				ValidUntil: &yesterday,
				Status:     QuoteStatusConverted,
			},
			expired: false,
		},
		{
			name: "valid_until in past but accepted - not expired",
			quote: Quote{
				ValidUntil: &yesterday,
				Status:     QuoteStatusAccepted,
			},
			expired: false,
		},
		{
			name: "valid_until in past and draft - expired",
			quote: Quote{
				ValidUntil: &yesterday,
				Status:     QuoteStatusDraft,
			},
			expired: true,
		},
		{
			name: "valid_until in past and rejected - expired",
			quote: Quote{
				ValidUntil: &yesterday,
				Status:     QuoteStatusRejected,
			},
			expired: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.quote.IsExpired()
			assert.Equal(t, tt.expired, result)
		})
	}
}

func TestQuoteStatusConstants(t *testing.T) {
	assert.Equal(t, QuoteStatus("DRAFT"), QuoteStatusDraft)
	assert.Equal(t, QuoteStatus("SENT"), QuoteStatusSent)
	assert.Equal(t, QuoteStatus("ACCEPTED"), QuoteStatusAccepted)
	assert.Equal(t, QuoteStatus("REJECTED"), QuoteStatusRejected)
	assert.Equal(t, QuoteStatus("EXPIRED"), QuoteStatusExpired)
	assert.Equal(t, QuoteStatus("CONVERTED"), QuoteStatusConverted)
}
