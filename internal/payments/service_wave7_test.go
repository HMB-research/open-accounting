package payments

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

func TestPaymentsWave7CreateFailsWhenInvoiceUpdateFails(t *testing.T) {
	repo := NewMockRepository()
	invoiceSvc := &MockInvoiceService{recordPaymentErr: assertablePaymentErr("invoice update failed")}
	service := NewServiceWithRepository(repo, invoiceSvc)
	invoiceID := "invoice-1"

	payment, err := service.Create(context.Background(), "tenant-1", "tenant_demo", &CreatePaymentRequest{
		PaymentType:  PaymentTypeReceived,
		PaymentDate:  time.Date(2026, 6, 25, 0, 0, 0, 0, time.UTC),
		Amount:       decimal.NewFromInt(100),
		Currency:     "EUR",
		ExchangeRate: decimal.NewFromInt(1),
		UserID:       "user-1",
		Allocations: []AllocationRequest{{
			InvoiceID: invoiceID,
			Amount:    decimal.NewFromInt(100),
		}},
	})
	if err == nil || !strings.Contains(err.Error(), "invoice update failed") {
		t.Fatalf("Create() error = %v, want invoice update failure", err)
	}
	if payment != nil {
		t.Fatalf("Create() payment = %#v, want nil on invoice update failure", payment)
	}
	if len(invoiceSvc.recordPaymentCalls) != 1 || invoiceSvc.recordPaymentCalls[0].invoiceID != invoiceID {
		t.Fatalf("RecordPayment calls = %#v", invoiceSvc.recordPaymentCalls)
	}
}

func TestPaymentsWave7ReverseNilRequestRequiresReason(t *testing.T) {
	service := NewServiceWithRepository(NewMockRepository(), nil)

	result, err := service.Reverse(context.Background(), "tenant-1", "tenant_demo", "payment-1", nil)
	if err == nil || !strings.Contains(err.Error(), "reversal reason is required") {
		t.Fatalf("Reverse(nil request) error = %v, want reason validation", err)
	}
	if result != nil {
		t.Fatalf("Reverse(nil request) result = %#v, want nil", result)
	}
}

type assertablePaymentErr string

func (e assertablePaymentErr) Error() string {
	return string(e)
}
