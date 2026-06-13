package payroll

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/shopspring/decimal"
)

func TestValidatePersonalCode(t *testing.T) {
	tests := []struct {
		name     string
		code     string
		expected bool
	}{
		// Valid codes with correct checksums
		{"Valid male born 1980", "38001010009", true},   // Checksum: 9
		{"Valid female born 1990", "49001010012", true}, // Checksum: 2
		{"Valid code with checksum 0", "37605030299", true},

		// Test case that triggers weights2 calculation (first checksum % 11 == 10)
		// Code 17605030336: weights1 sum = 1*1+7*2+6*3+0*4+5*5+0*6+3*7+0*8+3*9+3*1 = 1+14+18+0+25+0+21+0+27+3 = 109
		// 109 % 11 = 10, so weights2 is used
		// weights2 sum = 1*3+7*4+6*5+0*6+5*7+0*8+3*9+0*1+3*2+3*3 = 3+28+30+0+35+0+27+0+6+9 = 138
		// 138 % 11 = 6, so checksum = 6
		{"Valid with weights2", "17605030336", true},

		// Test case where weights2 also gives 10 (checksum becomes 0)
		// This code: 10001010080 triggers both weight sums % 11 == 10
		// weights1: 1+0+0+0+5+0+7+0+0+0 = 13 *wait* let me recalc
		// Actually: 1*1+0*2+0*3+0*4+1*5+0*6+1*7+0*8+0*9+8*1 = 1+0+0+0+5+0+7+0+0+8 = 21, 21%11=10
		// weights2: 1*3+0*4+0*5+0*6+1*7+0*8+1*9+0*1+0*2+8*3 = 3+0+0+0+7+0+9+0+0+24 = 43, 43%11=10 -> checksum=0
		{"Valid with double checksum 10", "10001010080", true},

		// Invalid codes - wrong length
		{"Too short", "3800101000", false},
		{"Too long", "380010100012", false},
		{"Empty", "", false},

		// Invalid codes - non-numeric
		{"Contains letter", "3800101000a", false},
		{"Contains space", "3800101 001", false},
		{"Contains special char", "38001-10001", false},

		// Invalid codes - wrong checksum
		{"Wrong checksum", "38001010001", false},
		{"Wrong checksum 2", "38001010003", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ValidatePersonalCode(tt.code); got != tt.expected {
				t.Errorf("ValidatePersonalCode(%q) = %v, want %v", tt.code, got, tt.expected)
			}
		})
	}
}

func TestValidatePersonalCode_KnownValidCodes(t *testing.T) {
	// These are structurally valid Estonian personal codes (not real people)
	validCodes := []string{
		"38001010009", // Male born 1980-01-01, checksum 9
		"49001010012", // Female born 1990-01-01, checksum 2
		"37605030299", // Male born 1976-05-03, checksum 9
	}

	for _, code := range validCodes {
		if !ValidatePersonalCode(code) {
			t.Errorf("Expected %q to be valid", code)
		}
	}
}

func TestFormatDecimal(t *testing.T) {
	tests := []struct {
		input    decimal.Decimal
		expected string
	}{
		{decimal.NewFromFloat(100.00), "100.00"},
		{decimal.NewFromFloat(100.123), "100.12"},
		{decimal.NewFromFloat(100.126), "100.13"},
		{decimal.NewFromFloat(0), "0.00"},
		{decimal.NewFromFloat(-50.50), "-50.50"},
	}

	for _, tt := range tests {
		if got := formatDecimal(tt.input); got != tt.expected {
			t.Errorf("formatDecimal(%v) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestFormatDecimalIfPositive(t *testing.T) {
	tests := []struct {
		input    decimal.Decimal
		expected string
	}{
		{decimal.NewFromFloat(100.00), "100.00"},
		{decimal.NewFromFloat(0.01), "0.01"},
		{decimal.NewFromFloat(0), ""},
		{decimal.NewFromFloat(-50.50), ""},
	}

	for _, tt := range tests {
		if got := formatDecimalIfPositive(tt.input); got != tt.expected {
			t.Errorf("formatDecimalIfPositive(%v) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestGenerateTSDFilename(t *testing.T) {
	tests := []struct {
		registryCode string
		year         int
		month        int
		format       string
	}{
		{"12345678", 2025, 1, "xml"},
		{"12345678", 2025, 12, "csv"},
		{"87654321", 2024, 6, "xml"},
	}

	for _, tt := range tests {
		filename := GenerateTSDFilename(tt.registryCode, tt.year, tt.month, tt.format)

		// Check that the filename contains expected parts
		if len(filename) == 0 {
			t.Error("Filename should not be empty")
		}

		// Filename format: TSD_REGCODE_YYYYMM_DATE.FORMAT
		expectedPrefix := "TSD_" + tt.registryCode
		if filename[:len(expectedPrefix)] != expectedPrefix {
			t.Errorf("Filename should start with %q, got %q", expectedPrefix, filename)
		}
	}
}

func TestPaymentTypeCodes(t *testing.T) {
	// Verify payment type codes are correctly defined per Estonian TSD specification
	expectedCodes := map[string]string{
		"PaymentTypeSalary":        "10",
		"PaymentTypeVacationPay":   "11",
		"PaymentTypeSickPay":       "12",
		"PaymentTypeBonus":         "13",
		"PaymentTypeTermination":   "14",
		"PaymentTypeBoard":         "21",
		"PaymentTypeContract":      "22",
		"PaymentTypeRoyalties":     "30",
		"PaymentTypeRent":          "40",
		"PaymentTypeInterest":      "50",
		"PaymentTypeDividends":     "60",
		"PaymentTypePension":       "70",
		"PaymentTypeBenefit":       "80",
		"PaymentTypeFringeBenefit": "42",
	}

	if PaymentTypeSalary != expectedCodes["PaymentTypeSalary"] {
		t.Errorf("PaymentTypeSalary = %q, want %q", PaymentTypeSalary, expectedCodes["PaymentTypeSalary"])
	}

	if PaymentTypeVacationPay != expectedCodes["PaymentTypeVacationPay"] {
		t.Errorf("PaymentTypeVacationPay = %q, want %q", PaymentTypeVacationPay, expectedCodes["PaymentTypeVacationPay"])
	}

	if PaymentTypeSickPay != expectedCodes["PaymentTypeSickPay"] {
		t.Errorf("PaymentTypeSickPay = %q, want %q", PaymentTypeSickPay, expectedCodes["PaymentTypeSickPay"])
	}

	if PaymentTypeBonus != expectedCodes["PaymentTypeBonus"] {
		t.Errorf("PaymentTypeBonus = %q, want %q", PaymentTypeBonus, expectedCodes["PaymentTypeBonus"])
	}

	if PaymentTypeBoard != expectedCodes["PaymentTypeBoard"] {
		t.Errorf("PaymentTypeBoard = %q, want %q", PaymentTypeBoard, expectedCodes["PaymentTypeBoard"])
	}

	if PaymentTypeContract != expectedCodes["PaymentTypeContract"] {
		t.Errorf("PaymentTypeContract = %q, want %q", PaymentTypeContract, expectedCodes["PaymentTypeContract"])
	}
}

func TestTSDStatusValues(t *testing.T) {
	// Verify TSD status enum values
	if TSDDraft != "DRAFT" {
		t.Errorf("TSDDraft = %q, want DRAFT", TSDDraft)
	}

	if TSDSubmitted != "SUBMITTED" {
		t.Errorf("TSDSubmitted = %q, want SUBMITTED", TSDSubmitted)
	}

	if TSDAccepted != "ACCEPTED" {
		t.Errorf("TSDAccepted = %q, want ACCEPTED", TSDAccepted)
	}

	if TSDRejected != "REJECTED" {
		t.Errorf("TSDRejected = %q, want REJECTED", TSDRejected)
	}
}

func TestPayrollStatusValues(t *testing.T) {
	// Verify payroll status enum values
	statuses := []struct {
		status   PayrollStatus
		expected string
	}{
		{PayrollDraft, "DRAFT"},
		{PayrollCalculated, "CALCULATED"},
		{PayrollApproved, "APPROVED"},
		{PayrollPaid, "PAID"},
		{PayrollDeclared, "DECLARED"},
	}

	for _, tt := range statuses {
		if string(tt.status) != tt.expected {
			t.Errorf("Status = %q, want %q", tt.status, tt.expected)
		}
	}
}

func TestCalculateTaxPreview(t *testing.T) {
	tests := []struct {
		name                string
		grossSalary         decimal.Decimal
		applyBasicExemption bool
		fundedPensionRate   decimal.Decimal
		expectedTaxable     decimal.Decimal
		expectedIncomeTax   decimal.Decimal
	}{
		{
			name:                "With basic exemption",
			grossSalary:         decimal.NewFromFloat(2000.00),
			applyBasicExemption: true,
			fundedPensionRate:   decimal.NewFromFloat(0.02),
			expectedTaxable:     decimal.NewFromFloat(1300.00), // 2000 - 700
			expectedIncomeTax:   decimal.NewFromFloat(286.00),  // 22% of 1300
		},
		{
			name:                "Without basic exemption",
			grossSalary:         decimal.NewFromFloat(2000.00),
			applyBasicExemption: false,
			fundedPensionRate:   decimal.NewFromFloat(0.02),
			expectedTaxable:     decimal.NewFromFloat(2000.00),
			expectedIncomeTax:   decimal.NewFromFloat(440.00), // 22% of 2000
		},
		{
			name:                "Zero salary with exemption",
			grossSalary:         decimal.Zero,
			applyBasicExemption: true,
			fundedPensionRate:   decimal.NewFromFloat(0.02),
			expectedTaxable:     decimal.Zero,
			expectedIncomeTax:   decimal.Zero,
		},
		{
			name:                "Low salary below exemption",
			grossSalary:         decimal.NewFromFloat(500.00),
			applyBasicExemption: true,
			fundedPensionRate:   decimal.NewFromFloat(0.02),
			expectedTaxable:     decimal.Zero, // 500 - 700 = negative, so 0
			expectedIncomeTax:   decimal.Zero,
		},
		{
			name:                "Increased pension rate",
			grossSalary:         decimal.NewFromFloat(3000.00),
			applyBasicExemption: true,
			fundedPensionRate:   decimal.NewFromFloat(0.04),
			expectedTaxable:     decimal.NewFromFloat(2300.00), // 3000 - 700
			expectedIncomeTax:   decimal.NewFromFloat(506.00),  // 22% of 2300
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calc := CalculateTaxPreview(tt.grossSalary, tt.applyBasicExemption, tt.fundedPensionRate)

			if !calc.TaxableIncome.Equal(tt.expectedTaxable) {
				t.Errorf("TaxableIncome = %s, want %s", calc.TaxableIncome, tt.expectedTaxable)
			}

			if !calc.IncomeTax.Equal(tt.expectedIncomeTax) {
				t.Errorf("IncomeTax = %s, want %s", calc.IncomeTax, tt.expectedIncomeTax)
			}

			// Verify basic exemption was correctly applied
			if tt.applyBasicExemption {
				if !calc.BasicExemption.Equal(DefaultBasicExemption) {
					t.Errorf("BasicExemption = %s, want %s", calc.BasicExemption, DefaultBasicExemption)
				}
			} else {
				if !calc.BasicExemption.IsZero() {
					t.Errorf("BasicExemption = %s, want zero", calc.BasicExemption)
				}
			}

			// Verify funded pension calculation
			expectedPension := tt.grossSalary.Mul(tt.fundedPensionRate).Round(2)
			if !calc.FundedPension.Equal(expectedPension) {
				t.Errorf("FundedPension = %s, want %s", calc.FundedPension, expectedPension)
			}
		})
	}
}

func TestEmploymentTypeValues(t *testing.T) {
	// Verify employment type enum values
	types := []struct {
		empType  EmploymentType
		expected string
	}{
		{EmploymentFullTime, "FULL_TIME"},
		{EmploymentPartTime, "PART_TIME"},
		{EmploymentContract, "CONTRACT"},
	}

	for _, tt := range types {
		if string(tt.empType) != tt.expected {
			t.Errorf("EmploymentType = %q, want %q", tt.empType, tt.expected)
		}
	}
}

func TestServiceGenerateTSDWithMockRepository(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	service := NewServiceWithRepository(repo, &MockUUIDGenerator{prefix: "tsd"})

	repo.PayrollRuns["run-1"] = &PayrollRun{
		ID:          "run-1",
		TenantID:    "tenant-1",
		PeriodYear:  2025,
		PeriodMonth: 1,
		Status:      PayrollApproved,
	}
	repo.Employees["emp-1"] = &Employee{
		ID:           "emp-1",
		TenantID:     "tenant-1",
		FirstName:    "Mari",
		LastName:     "Maasikas",
		PersonalCode: "38001010009",
	}
	repo.Payslips = []Payslip{{
		ID:                      "pay-1",
		TenantID:                "tenant-1",
		PayrollRunID:            "run-1",
		EmployeeID:              "emp-1",
		GrossSalary:             decimal.NewFromInt(2000),
		BasicExemptionApplied:   decimal.NewFromInt(700),
		TaxableIncome:           decimal.NewFromInt(1300),
		IncomeTax:               decimal.NewFromInt(286),
		SocialTax:               decimal.NewFromInt(660),
		UnemploymentInsuranceEE: decimal.NewFromInt(32),
		UnemploymentInsuranceER: decimal.NewFromInt(16),
		FundedPension:           decimal.NewFromInt(40),
		NetSalary:               decimal.NewFromInt(1642),
		TotalEmployerCost:       decimal.NewFromInt(2676),
	}}
	repo.TSDDeclarations["old-tsd"] = &TSDDeclaration{
		ID:          "old-tsd",
		TenantID:    "tenant-1",
		PeriodYear:  2025,
		PeriodMonth: 1,
		Status:      TSDSubmitted,
	}

	tsd, err := service.GenerateTSD(ctx, "tenant_schema", "tenant-1", "run-1")
	if err != nil {
		t.Fatalf("GenerateTSD returned error: %v", err)
	}

	if tsd.PayrollRunID != "run-1" || tsd.Status != TSDDraft {
		t.Fatalf("unexpected TSD metadata: run=%q status=%q", tsd.PayrollRunID, tsd.Status)
	}
	requireDecimalEqual(t, tsd.TotalPayments, decimal.NewFromInt(2000))
	requireDecimalEqual(t, tsd.TotalIncomeTax, decimal.NewFromInt(286))
	requireDecimalEqual(t, tsd.TotalSocialTax, decimal.NewFromInt(660))
	requireDecimalEqual(t, tsd.TotalUnemploymentEE, decimal.NewFromInt(32))
	requireDecimalEqual(t, tsd.TotalUnemploymentER, decimal.NewFromInt(16))
	requireDecimalEqual(t, tsd.TotalFundedPension, decimal.NewFromInt(40))
	if len(tsd.Rows) != 1 {
		t.Fatalf("expected one TSD row, got %d", len(tsd.Rows))
	}
	row := tsd.Rows[0]
	if row.EmployeeID != "emp-1" || row.PersonalCode != "38001010009" || row.PaymentType != PaymentTypeSalary {
		t.Fatalf("unexpected row identity: %+v", row)
	}
	if _, ok := repo.TSDDeclarations["old-tsd"]; ok {
		t.Fatalf("expected GenerateTSD to delete existing declaration for the period")
	}
	if _, ok := repo.TSDDeclarations[tsd.ID]; !ok {
		t.Fatalf("expected generated declaration to be persisted")
	}
}

func TestServiceGenerateTSDSkipsPayslipsWithoutEmployee(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	service := NewServiceWithRepository(repo, &MockUUIDGenerator{prefix: "tsd"})

	repo.PayrollRuns["run-1"] = &PayrollRun{
		ID:          "run-1",
		TenantID:    "tenant-1",
		PeriodYear:  2025,
		PeriodMonth: 1,
		Status:      PayrollApproved,
	}
	repo.Employees["emp-1"] = &Employee{
		ID:           "emp-1",
		TenantID:     "tenant-1",
		FirstName:    "Mari",
		LastName:     "Maasikas",
		PersonalCode: "38001010009",
	}
	repo.Payslips = []Payslip{
		{
			ID:                      "pay-1",
			TenantID:                "tenant-1",
			PayrollRunID:            "run-1",
			EmployeeID:              "emp-1",
			GrossSalary:             decimal.NewFromInt(1000),
			IncomeTax:               decimal.NewFromInt(220),
			SocialTax:               decimal.NewFromInt(330),
			UnemploymentInsuranceEE: decimal.NewFromInt(16),
			UnemploymentInsuranceER: decimal.NewFromInt(8),
			FundedPension:           decimal.NewFromInt(20),
		},
		{
			ID:                      "pay-missing-employee",
			TenantID:                "tenant-1",
			PayrollRunID:            "run-1",
			EmployeeID:              "missing-emp",
			GrossSalary:             decimal.NewFromInt(500),
			IncomeTax:               decimal.NewFromInt(110),
			SocialTax:               decimal.NewFromInt(165),
			UnemploymentInsuranceEE: decimal.NewFromInt(8),
			UnemploymentInsuranceER: decimal.NewFromInt(4),
			FundedPension:           decimal.NewFromInt(10),
		},
	}

	tsd, err := service.GenerateTSD(ctx, "tenant_schema", "tenant-1", "run-1")
	if err != nil {
		t.Fatalf("GenerateTSD returned error: %v", err)
	}

	if len(tsd.Rows) != 1 {
		t.Fatalf("expected only the payslip with employee data to become a row, got %d", len(tsd.Rows))
	}
	if tsd.Rows[0].EmployeeID != "emp-1" {
		t.Fatalf("unexpected generated row: %+v", tsd.Rows[0])
	}
	requireDecimalEqual(t, tsd.TotalPayments, decimal.NewFromInt(1000))
	requireDecimalEqual(t, tsd.TotalIncomeTax, decimal.NewFromInt(220))
	requireDecimalEqual(t, tsd.TotalSocialTax, decimal.NewFromInt(330))
	requireDecimalEqual(t, tsd.TotalUnemploymentEE, decimal.NewFromInt(16))
	requireDecimalEqual(t, tsd.TotalUnemploymentER, decimal.NewFromInt(8))
	requireDecimalEqual(t, tsd.TotalFundedPension, decimal.NewFromInt(20))
}

func TestServiceGenerateTSDAllowsPaidPayrollRun(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	service := NewServiceWithRepository(repo, &MockUUIDGenerator{prefix: "tsd"})
	seedApprovedPayrollRunWithPayslip(repo)
	repo.PayrollRuns["run-1"].Status = PayrollPaid

	tsd, err := service.GenerateTSD(ctx, "tenant_schema", "tenant-1", "run-1")
	if err != nil {
		t.Fatalf("GenerateTSD returned error: %v", err)
	}

	if tsd.PayrollRunID != "run-1" || len(tsd.Rows) != 1 {
		t.Fatalf("unexpected generated TSD: %+v", tsd)
	}
	requireDecimalEqual(t, tsd.TotalPayments, decimal.NewFromInt(1000))
}

func TestServiceGenerateTSDErrors(t *testing.T) {
	ctx := context.Background()

	t.Run("wraps payroll run lookup failures", func(t *testing.T) {
		repo := NewMockRepository()
		service := NewServiceWithRepository(repo, &MockUUIDGenerator{prefix: "tsd"})
		repo.GetPayrollRunErr = errors.New("lookup failed")

		_, err := service.GenerateTSD(ctx, "tenant_schema", "tenant-1", "run-1")
		if err == nil || !strings.Contains(err.Error(), "get payroll run") {
			t.Fatalf("expected payroll run lookup error, got %v", err)
		}
	})

	t.Run("rejects unapproved payroll run", func(t *testing.T) {
		repo := NewMockRepository()
		service := NewServiceWithRepository(repo, &MockUUIDGenerator{prefix: "tsd"})
		repo.PayrollRuns["run-1"] = &PayrollRun{
			ID:       "run-1",
			TenantID: "tenant-1",
			Status:   PayrollCalculated,
		}

		_, err := service.GenerateTSD(ctx, "tenant_schema", "tenant-1", "run-1")
		if err == nil || !strings.Contains(err.Error(), "APPROVED or PAID") {
			t.Fatalf("expected status validation error, got %v", err)
		}
	})

	t.Run("rejects payroll run without payslips", func(t *testing.T) {
		repo := NewMockRepository()
		service := NewServiceWithRepository(repo, &MockUUIDGenerator{prefix: "tsd"})
		repo.PayrollRuns["run-1"] = &PayrollRun{
			ID:       "run-1",
			TenantID: "tenant-1",
			Status:   PayrollApproved,
		}

		_, err := service.GenerateTSD(ctx, "tenant_schema", "tenant-1", "run-1")
		if err == nil || !strings.Contains(err.Error(), "no payslips") {
			t.Fatalf("expected missing payslips error, got %v", err)
		}
	})

	t.Run("wraps payslip lookup failures", func(t *testing.T) {
		repo := NewMockRepository()
		service := NewServiceWithRepository(repo, &MockUUIDGenerator{prefix: "tsd"})
		repo.PayrollRuns["run-1"] = &PayrollRun{
			ID:       "run-1",
			TenantID: "tenant-1",
			Status:   PayrollApproved,
		}
		repo.GetPayslipsErr = errors.New("lookup failed")

		_, err := service.GenerateTSD(ctx, "tenant_schema", "tenant-1", "run-1")
		if err == nil || !strings.Contains(err.Error(), "get payslips") {
			t.Fatalf("expected payslip lookup error, got %v", err)
		}
	})

	t.Run("wraps delete existing declaration failures", func(t *testing.T) {
		repo := NewMockRepository()
		service := NewServiceWithRepository(repo, &MockUUIDGenerator{prefix: "tsd"})
		seedApprovedPayrollRunWithPayslip(repo)
		repo.DeleteTSDErr = errors.New("delete failed")

		_, err := service.GenerateTSD(ctx, "tenant_schema", "tenant-1", "run-1")
		if err == nil || !strings.Contains(err.Error(), "delete existing TSD") {
			t.Fatalf("expected delete existing TSD error, got %v", err)
		}
	})

	t.Run("wraps declaration create failures", func(t *testing.T) {
		repo := NewMockRepository()
		service := NewServiceWithRepository(repo, &MockUUIDGenerator{prefix: "tsd"})
		seedApprovedPayrollRunWithPayslip(repo)
		repo.CreateTSDErr = errors.New("create failed")

		_, err := service.GenerateTSD(ctx, "tenant_schema", "tenant-1", "run-1")
		if err == nil || !strings.Contains(err.Error(), "insert TSD declaration") {
			t.Fatalf("expected declaration insert error, got %v", err)
		}
	})

	t.Run("wraps repository write failures", func(t *testing.T) {
		repo := NewMockRepository()
		service := NewServiceWithRepository(repo, &MockUUIDGenerator{prefix: "tsd"})
		seedApprovedPayrollRunWithPayslip(repo)
		repo.CreateTSDRowsErr = errors.New("insert failed")

		_, err := service.GenerateTSD(ctx, "tenant_schema", "tenant-1", "run-1")
		if err == nil || !strings.Contains(err.Error(), "insert TSD rows") {
			t.Fatalf("expected row insert error, got %v", err)
		}
	})
}

func TestServiceExportTSDToXMLAndCSV(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	service := NewServiceWithRepository(repo, &MockUUIDGenerator{prefix: "tsd"})
	seedTSDForExport(repo)

	xmlData, err := service.ExportTSDToXML(ctx, "tenant_schema", "tenant-1", 2025, 1, TSDCompanyInfo{
		RegistryCode: "12345678",
		Name:         "Acme OU",
	})
	if err != nil {
		t.Fatalf("ExportTSDToXML returned error: %v", err)
	}
	xmlText := string(xmlData)
	for _, expected := range []string{
		"<dpiPeriood>202501</dpiPeriood>",
		"<rpiMkIsikKood>12345678</rpiMkIsikKood>",
		"<l1Isikukood>38001010009</l1Isikukood>",
		"<l1Mv>700.00</l1Mv>",
		"<l1TkmTootja>16.00</l1TkmTootja>",
	} {
		if !strings.Contains(xmlText, expected) {
			t.Fatalf("expected XML to contain %q\n%s", expected, xmlText)
		}
	}

	csvData, err := service.ExportTSDToCSV(ctx, "tenant_schema", "tenant-1", 2025, 1)
	if err != nil {
		t.Fatalf("ExportTSDToCSV returned error: %v", err)
	}
	csvText := string(csvData)
	if !strings.Contains(csvText, "row_number;personal_code;first_name;last_name;payment_type") {
		t.Fatalf("CSV header missing: %s", csvText)
	}
	expectedRow := "1;38001010009;Mari;Maasikas;10;2000.00;700.00;1300.00;286.00;660.00;32.00;16.00;40.00"
	if !strings.Contains(csvText, expectedRow) {
		t.Fatalf("expected CSV row %q\n%s", expectedRow, csvText)
	}
}

func TestServiceExportTSDToXMLWithoutRows(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	service := NewServiceWithRepository(repo, &MockUUIDGenerator{prefix: "tsd"})

	repo.TSDDeclarations["tsd-1"] = &TSDDeclaration{
		ID:                  "tsd-1",
		TenantID:            "tenant-1",
		PeriodYear:          2025,
		PeriodMonth:         2,
		Status:              TSDDraft,
		TotalPayments:       decimal.NewFromInt(1000),
		TotalIncomeTax:      decimal.NewFromInt(220),
		TotalSocialTax:      decimal.NewFromInt(330),
		TotalUnemploymentEE: decimal.NewFromInt(16),
		TotalUnemploymentER: decimal.NewFromInt(8),
		TotalFundedPension:  decimal.NewFromInt(20),
	}

	xmlData, err := service.ExportTSDToXML(ctx, "tenant_schema", "tenant-1", 2025, 2, TSDCompanyInfo{
		RegistryCode: "12345678",
		Name:         "Acme OU",
	})
	if err != nil {
		t.Fatalf("ExportTSDToXML returned error: %v", err)
	}

	xmlText := string(xmlData)
	for _, expected := range []string{
		"<dpiPeriood>202502</dpiPeriood>",
		"<dpsMaksedKokku>1000.00</dpsMaksedKokku>",
		"<dpsTm>220.00</dpsTm>",
	} {
		if !strings.Contains(xmlText, expected) {
			t.Fatalf("expected XML to contain %q\n%s", expected, xmlText)
		}
	}
	for _, unexpected := range []string{
		"<dpiLisa1>",
		"<l1Rida>",
		"<l1Isikukood>",
	} {
		if strings.Contains(xmlText, unexpected) {
			t.Fatalf("expected XML not to contain %q\n%s", unexpected, xmlText)
		}
	}
}

func TestServiceTSDQuerySummaryAndStatusMarkers(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	service := NewServiceWithRepository(repo, &MockUUIDGenerator{prefix: "tsd"})
	seedTSDForExport(repo)

	rows, err := service.GetTSDRows(ctx, "tenant_schema", "tenant-1", "tsd-1")
	if err != nil {
		t.Fatalf("GetTSDRows returned error: %v", err)
	}
	if len(rows) != 1 || rows[0].EmployeeID != "emp-1" {
		t.Fatalf("unexpected TSD rows: %+v", rows)
	}

	declarations, err := service.ListTSD(ctx, "tenant_schema", "tenant-1", TSDListFilter{Year: 2025, Month: 1})
	if err != nil {
		t.Fatalf("ListTSD returned error: %v", err)
	}
	if len(declarations) != 1 || declarations[0].ID != "tsd-1" {
		t.Fatalf("unexpected declarations: %+v", declarations)
	}

	summary, err := service.GetTSDSummary(ctx, "tenant_schema", "tenant-1", 2025, 1)
	if err != nil {
		t.Fatalf("GetTSDSummary returned error: %v", err)
	}
	if summary.Period != "2025-01" || summary.EmployeeCount != 1 || summary.Status != TSDDraft {
		t.Fatalf("unexpected summary metadata: %+v", summary)
	}
	requireDecimalEqual(t, summary.TotalGrossPayments, decimal.NewFromInt(2000))
	requireDecimalEqual(t, summary.TotalTaxes, decimal.NewFromInt(358))
	requireDecimalEqual(t, summary.TotalEmployerCosts, decimal.NewFromInt(676))

	if err := service.MarkTSDSubmitted(ctx, "tenant_schema", "tenant-1", "tsd-1", "EMTA-REF-1"); err != nil {
		t.Fatalf("MarkTSDSubmitted returned error: %v", err)
	}
	if repo.TSDDeclarations["tsd-1"].Status != TSDSubmitted ||
		repo.TSDDeclarations["tsd-1"].EMTAReference != "EMTA-REF-1" ||
		repo.TSDDeclarations["tsd-1"].SubmittedAt == nil {
		t.Fatalf("submission marker was not persisted: %+v", repo.TSDDeclarations["tsd-1"])
	}

	if err := service.MarkTSDAccepted(ctx, "tenant_schema", "tenant-1", "tsd-1"); err != nil {
		t.Fatalf("MarkTSDAccepted returned error: %v", err)
	}
	if repo.TSDDeclarations["tsd-1"].Status != TSDAccepted {
		t.Fatalf("expected accepted status, got %q", repo.TSDDeclarations["tsd-1"].Status)
	}

	if err := service.MarkTSDRejected(ctx, "tenant_schema", "tenant-1", "tsd-1"); err != nil {
		t.Fatalf("MarkTSDRejected returned error: %v", err)
	}
	if repo.TSDDeclarations["tsd-1"].Status != TSDRejected {
		t.Fatalf("expected rejected status, got %q", repo.TSDDeclarations["tsd-1"].Status)
	}
}

func TestServiceTSDErrorWrapping(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	service := NewServiceWithRepository(repo, &MockUUIDGenerator{prefix: "tsd"})

	repo.GetTSDErr = errors.New("lookup failed")
	if _, err := service.ExportTSDToXML(ctx, "tenant_schema", "tenant-1", 2025, 1, TSDCompanyInfo{}); err == nil ||
		!strings.Contains(err.Error(), "get TSD") {
		t.Fatalf("expected XML export lookup error, got %v", err)
	}
	if _, err := service.ExportTSDToCSV(ctx, "tenant_schema", "tenant-1", 2025, 1); err == nil ||
		!strings.Contains(err.Error(), "get TSD") {
		t.Fatalf("expected CSV export lookup error, got %v", err)
	}
	if _, err := service.GetTSDSummary(ctx, "tenant_schema", "tenant-1", 2025, 1); err == nil ||
		!strings.Contains(err.Error(), "get TSD") {
		t.Fatalf("expected summary lookup error, got %v", err)
	}

	repo.GetTSDErr = ErrTSDDeclarationNotFound
	if _, err := service.GetTSD(ctx, "tenant_schema", "tenant-1", 2025, 1); err == nil ||
		!strings.Contains(err.Error(), "TSD declaration not found") {
		t.Fatalf("expected not found error, got %v", err)
	}

	repo.GetTSDErr = nil
	repo.GetPayslipsErr = errors.New("payslips failed")
	if _, err := service.GetPayslipsWithEmployees(ctx, "tenant_schema", "tenant-1", "run-1"); err == nil ||
		!strings.Contains(err.Error(), "get payslips") {
		t.Fatalf("expected payslip lookup error, got %v", err)
	}

	repo.GetPayslipsErr = nil
	repo.GetTSDRowsErr = errors.New("rows failed")
	if _, err := service.GetTSDRows(ctx, "tenant_schema", "tenant-1", "tsd-1"); err == nil ||
		!strings.Contains(err.Error(), "get TSD rows") {
		t.Fatalf("expected row lookup error, got %v", err)
	}

	repo.GetTSDRowsErr = nil
	repo.ListTSDErr = errors.New("list failed")
	if _, err := service.ListTSD(ctx, "tenant_schema", "tenant-1", TSDListFilter{}); err == nil ||
		!strings.Contains(err.Error(), "list TSD") {
		t.Fatalf("expected list error, got %v", err)
	}

	repo.ListTSDErr = nil
	repo.MarkTSDSubmittedErr = errors.New("submit failed")
	if err := service.MarkTSDSubmitted(ctx, "tenant_schema", "tenant-1", "tsd-1", "ref"); err == nil ||
		!strings.Contains(err.Error(), "mark TSD submitted") {
		t.Fatalf("expected submitted marker error, got %v", err)
	}

	repo.MarkTSDSubmittedErr = nil
	repo.UpdateTSDStatusErr = errors.New("status failed")
	if err := service.MarkTSDAccepted(ctx, "tenant_schema", "tenant-1", "tsd-1"); err == nil ||
		!strings.Contains(err.Error(), "mark TSD accepted") {
		t.Fatalf("expected accepted marker error, got %v", err)
	}
	if err := service.MarkTSDRejected(ctx, "tenant_schema", "tenant-1", "tsd-1"); err == nil ||
		!strings.Contains(err.Error(), "mark TSD rejected") {
		t.Fatalf("expected rejected marker error, got %v", err)
	}
}

func seedApprovedPayrollRunWithPayslip(repo *MockRepository) {
	repo.PayrollRuns["run-1"] = &PayrollRun{
		ID:          "run-1",
		TenantID:    "tenant-1",
		PeriodYear:  2025,
		PeriodMonth: 1,
		Status:      PayrollApproved,
	}
	repo.Employees["emp-1"] = &Employee{
		ID:        "emp-1",
		TenantID:  "tenant-1",
		FirstName: "Mari",
		LastName:  "Maasikas",
	}
	repo.Payslips = []Payslip{{
		ID:           "pay-1",
		TenantID:     "tenant-1",
		PayrollRunID: "run-1",
		EmployeeID:   "emp-1",
		GrossSalary:  decimal.NewFromInt(1000),
	}}
}

func seedTSDForExport(repo *MockRepository) {
	repo.TSDDeclarations["tsd-1"] = &TSDDeclaration{
		ID:                  "tsd-1",
		TenantID:            "tenant-1",
		PeriodYear:          2025,
		PeriodMonth:         1,
		Status:              TSDDraft,
		TotalPayments:       decimal.NewFromInt(2000),
		TotalIncomeTax:      decimal.NewFromInt(286),
		TotalSocialTax:      decimal.NewFromInt(660),
		TotalUnemploymentEE: decimal.NewFromInt(32),
		TotalUnemploymentER: decimal.NewFromInt(16),
		TotalFundedPension:  decimal.NewFromInt(40),
	}
	repo.TSDRows["tsd-1"] = []TSDRow{{
		ID:             "row-1",
		TenantID:       "tenant-1",
		DeclarationID:  "tsd-1",
		EmployeeID:     "emp-1",
		PersonalCode:   "38001010009",
		FirstName:      "Mari",
		LastName:       "Maasikas",
		PaymentType:    PaymentTypeSalary,
		GrossPayment:   decimal.NewFromInt(2000),
		BasicExemption: decimal.NewFromInt(700),
		TaxableAmount:  decimal.NewFromInt(1300),
		IncomeTax:      decimal.NewFromInt(286),
		SocialTax:      decimal.NewFromInt(660),
		UnemploymentEE: decimal.NewFromInt(32),
		UnemploymentER: decimal.NewFromInt(16),
		FundedPension:  decimal.NewFromInt(40),
	}}
}

func requireDecimalEqual(t *testing.T, got, want decimal.Decimal) {
	t.Helper()
	if !got.Equal(want) {
		t.Fatalf("decimal mismatch: got %s, want %s", got, want)
	}
}
