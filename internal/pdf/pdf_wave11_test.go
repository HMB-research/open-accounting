package pdf

import (
	"errors"
	"testing"

	"github.com/johnfercher/maroto/v2/pkg/core"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"

	"github.com/HMB-research/open-accounting/internal/payroll"
)

func stubWave11GenerateMarotoPDF(t *testing.T, err error) {
	t.Helper()
	original := generateMarotoPDF
	generateMarotoPDF = func(core.Maroto) (core.Document, error) {
		return nil, err
	}
	t.Cleanup(func() {
		generateMarotoPDF = original
	})
}

func TestWave11PDFGenerationErrors(t *testing.T) {
	expectedErr := errors.New("generate failed")
	stubWave11GenerateMarotoPDF(t, expectedErr)

	service := NewService()
	tnant := createTestTenant()

	invoiceBytes, err := service.GenerateInvoicePDF(createTestInvoice(), tnant, DefaultPDFSettings())
	require.Nil(t, invoiceBytes)
	require.ErrorIs(t, err, expectedErr)
	require.Contains(t, err.Error(), "failed to generate PDF")

	quoteBytes, err := service.GenerateQuotePDF(createTestQuote(), tnant, DefaultPDFSettings())
	require.Nil(t, quoteBytes)
	require.ErrorIs(t, err, expectedErr)
	require.Contains(t, err.Error(), "failed to generate PDF")

	payslipBytes, err := service.GeneratePayslipPDF(&payroll.Payslip{
		ID:            "payslip-1",
		TenantID:      "tenant-1",
		PayrollRunID:  "run-1",
		EmployeeID:    "employee-1",
		GrossSalary:   decimal.NewFromInt(1000),
		TaxableIncome: decimal.NewFromInt(1000),
		NetSalary:     decimal.NewFromInt(800),
		PaymentStatus: "PENDING",
		Employee:      &payroll.Employee{FirstName: "Mari", LastName: "Maasikas"},
	}, &payroll.PayrollRun{PeriodYear: 2026, PeriodMonth: 6}, tnant)
	require.Nil(t, payslipBytes)
	require.ErrorIs(t, err, expectedErr)
	require.Contains(t, err.Error(), "failed to generate payslip PDF")
}
