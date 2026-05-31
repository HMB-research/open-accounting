package banking

import (
	"github.com/HMB-research/open-accounting/internal/payments"
	"github.com/shopspring/decimal"
)

func paymentTypeForTransactionAmount(amount decimal.Decimal) payments.PaymentType {
	if amount.IsNegative() {
		return payments.PaymentTypeMade
	}
	return payments.PaymentTypeReceived
}
