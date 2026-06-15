package payments

import (
	"fmt"
	"strconv"
	"strings"
)

// PaymentNumberPrefix returns the generated number prefix for a payment type.
func PaymentNumberPrefix(paymentType PaymentType) string {
	if paymentType == PaymentTypeMade {
		return "OUT"
	}
	return "PMT"
}

// FormatPaymentNumber formats a generated payment number for a sequence value.
func FormatPaymentNumber(paymentType PaymentType, sequence int) string {
	return fmt.Sprintf("%s-%05d", PaymentNumberPrefix(paymentType), sequence)
}

// NextPaymentNumberSequence returns the next generated sequence from existing numbers.
func NextPaymentNumberSequence(paymentNumbers []string, paymentType PaymentType) int {
	prefix := PaymentNumberPrefix(paymentType)
	maxSeq := 0
	for _, paymentNumber := range paymentNumbers {
		seq, ok := paymentNumberSequence(paymentNumber, prefix)
		if ok && seq > maxSeq {
			maxSeq = seq
		}
	}
	return maxSeq + 1
}

func paymentNumberSequence(paymentNumber, prefix string) (int, bool) {
	sequenceText, ok := strings.CutPrefix(strings.TrimSpace(paymentNumber), prefix+"-")
	if !ok || sequenceText == "" {
		return 0, false
	}
	for i, char := range sequenceText {
		if char < '0' || char > '9' {
			sequenceText = sequenceText[:i]
			break
		}
	}
	if sequenceText == "" {
		return 0, false
	}
	sequence, err := strconv.Atoi(sequenceText)
	if err != nil {
		return 0, false
	}
	return sequence, true
}
