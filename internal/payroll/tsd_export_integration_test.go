//go:build integration

package payroll

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/HMB-research/open-accounting/internal/testutil"
	"github.com/shopspring/decimal"
)

func setupTSDForExportTest(t *testing.T) (*Service, *testutil.TestTenant, *TSDDeclaration) {
	t.Helper()

	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	userID := testutil.CreateTestUser(t, pool, "tsd-export-test@example.com")
	testutil.AddUserToTenant(t, pool, tenant.ID, userID, "admin")

	service := NewService(pool)
	ctx := context.Background()

	// Ensure payroll schema
	_, err := pool.Exec(ctx, "SELECT add_payroll_tables($1)", tenant.SchemaName)
	if err != nil {
		t.Fatalf("Failed to add payroll tables: %v", err)
	}

	// Create employee with personal code
	employee, err := service.CreateEmployee(ctx, tenant.SchemaName, tenant.ID, &CreateEmployeeRequest{
		EmployeeNumber:       "EMP-EXPORT-001",
		FirstName:            "Katrin",
		LastName:             "Kask",
		PersonalCode:         "48903125678",
		Email:                "katrin@example.com",
		StartDate:            time.Now().AddDate(-1, 0, 0),
		Position:             "Manager",
		EmploymentType:       EmploymentFullTime,
		ApplyBasicExemption:  true,
		BasicExemptionAmount: DefaultBasicExemption,
		FundedPensionRate:    FundedPensionRateDefault,
	})
	if err != nil {
		t.Fatalf("CreateEmployee failed: %v", err)
	}

	// Set salary
	err = service.SetBaseSalary(ctx, tenant.SchemaName, tenant.ID, employee.ID, decimal.NewFromFloat(4000.00), time.Now().AddDate(0, -1, 0))
	if err != nil {
		t.Fatalf("SetBaseSalary failed: %v", err)
	}

	// Create payroll run
	now := time.Now()
	run, err := service.CreatePayrollRun(ctx, tenant.SchemaName, tenant.ID, userID, &CreatePayrollRunRequest{
		PeriodYear:  now.Year(),
		PeriodMonth: int(now.Month()),
	})
	if err != nil {
		t.Fatalf("CreatePayrollRun failed: %v", err)
	}

	// Calculate and approve
	_, err = service.CalculatePayroll(ctx, tenant.SchemaName, tenant.ID, run.ID)
	if err != nil {
		t.Fatalf("CalculatePayroll failed: %v", err)
	}

	err = service.ApprovePayrollRun(ctx, tenant.SchemaName, tenant.ID, run.ID, userID)
	if err != nil {
		t.Fatalf("ApprovePayrollRun failed: %v", err)
	}

	// Generate TSD
	tsd, err := service.GenerateTSD(ctx, tenant.SchemaName, tenant.ID, run.ID)
	if err != nil {
		t.Fatalf("GenerateTSD failed: %v", err)
	}

	return service, tenant, tsd
}

func TestService_ExportTSDToXML(t *testing.T) {
	service, tenant, tsd := setupTSDForExportTest(t)
	ctx := context.Background()

	company := TSDCompanyInfo{
		RegistryCode: "12345678",
		Name:         "Test Company OÜ",
	}

	xmlData, err := service.ExportTSDToXML(ctx, tenant.SchemaName, tenant.ID, tsd.PeriodYear, tsd.PeriodMonth, company)
	if err != nil {
		t.Fatalf("ExportTSDToXML failed: %v", err)
	}

	xmlStr := string(xmlData)

	// Verify XML header
	if !strings.HasPrefix(xmlStr, "<?xml version=") {
		t.Error("expected XML declaration at start")
	}

	// Verify root element
	if !strings.Contains(xmlStr, "<tpiDeklaratsioon") {
		t.Error("expected tpiDeklaratsioon root element")
	}

	// Verify namespace
	if !strings.Contains(xmlStr, "xmlns=\"http://www.emta.ee/xml/tsd\"") {
		t.Error("expected e-MTA namespace")
	}

	// Verify company info
	if !strings.Contains(xmlStr, company.RegistryCode) {
		t.Error("expected company registry code in XML")
	}
	if !strings.Contains(xmlStr, company.Name) {
		t.Error("expected company name in XML")
	}

	// Verify period
	expectedPeriod := time.Now().Format("200601")[:6]
	if !strings.Contains(xmlStr, expectedPeriod) {
		t.Errorf("expected period %s in XML", expectedPeriod)
	}

	// Verify Annex 1 structure
	if !strings.Contains(xmlStr, "<dpiLisa1") {
		t.Error("expected dpiLisa1 element (Annex 1)")
	}
	if !strings.Contains(xmlStr, "<l1Rida") {
		t.Error("expected l1Rida element (Annex 1 row)")
	}

	// Verify employee data in XML
	if !strings.Contains(xmlStr, "48903125678") { // Personal code
		t.Error("expected personal code in XML")
	}
	if !strings.Contains(xmlStr, "Katrin") {
		t.Error("expected first name in XML")
	}
	if !strings.Contains(xmlStr, "Kask") {
		t.Error("expected last name in XML")
	}
}

func TestService_ExportTSDToCSV(t *testing.T) {
	service, tenant, tsd := setupTSDForExportTest(t)
	ctx := context.Background()

	csvData, err := service.ExportTSDToCSV(ctx, tenant.SchemaName, tenant.ID, tsd.PeriodYear, tsd.PeriodMonth)
	if err != nil {
		t.Fatalf("ExportTSDToCSV failed: %v", err)
	}

	csvStr := string(csvData)
	lines := strings.Split(csvStr, "\n")

	// Verify header
	if len(lines) < 2 {
		t.Fatal("expected at least header and one data row")
	}

	header := lines[0]
	expectedHeaders := []string{
		"row_number", "personal_code", "first_name", "last_name",
		"payment_type", "gross_payment", "basic_exemption", "taxable_amount",
		"income_tax", "social_tax", "unemployment_ee", "unemployment_er", "funded_pension",
	}
	for _, h := range expectedHeaders {
		if !strings.Contains(header, h) {
			t.Errorf("expected header to contain %s", h)
		}
	}

	// Verify data row
	dataRow := lines[1]
	if !strings.Contains(dataRow, "48903125678") { // Personal code
		t.Error("expected personal code in CSV data")
	}
	if !strings.Contains(dataRow, "Katrin") {
		t.Error("expected first name in CSV data")
	}
	if !strings.Contains(dataRow, "Kask") {
		t.Error("expected last name in CSV data")
	}

	// Verify semicolon delimiter
	if !strings.Contains(dataRow, ";") {
		t.Error("expected semicolon delimiter in CSV")
	}
}

func TestService_GetTSDSummary(t *testing.T) {
	service, tenant, tsd := setupTSDForExportTest(t)
	ctx := context.Background()

	summary, err := service.GetTSDSummary(ctx, tenant.SchemaName, tenant.ID, tsd.PeriodYear, tsd.PeriodMonth)
	if err != nil {
		t.Fatalf("GetTSDSummary failed: %v", err)
	}

	// Verify summary fields
	now := time.Now()
	expectedPeriod := now.Format("2006-01")
	if summary.Period != expectedPeriod {
		t.Errorf("expected period %s, got %s", expectedPeriod, summary.Period)
	}

	if summary.EmployeeCount != 1 {
		t.Errorf("expected 1 employee, got %d", summary.EmployeeCount)
	}

	if summary.TotalGrossPayments.IsZero() {
		t.Error("expected non-zero total gross payments")
	}

	if summary.TotalTaxes.IsZero() {
		t.Error("expected non-zero total taxes")
	}

	if summary.TotalEmployerCosts.IsZero() {
		t.Error("expected non-zero total employer costs")
	}

	if summary.Status != TSDDraft {
		t.Errorf("expected status %s, got %s", TSDDraft, summary.Status)
	}
}

func TestService_MarkTSDSubmitted(t *testing.T) {
	service, tenant, tsd := setupTSDForExportTest(t)
	ctx := context.Background()

	emtaReference := "EMTA-2025-TEST-123"

	err := service.MarkTSDSubmitted(ctx, tenant.SchemaName, tenant.ID, tsd.ID, emtaReference)
	if err != nil {
		t.Fatalf("MarkTSDSubmitted failed: %v", err)
	}

	// Verify status changed
	updated, err := service.GetTSD(ctx, tenant.SchemaName, tenant.ID, tsd.PeriodYear, tsd.PeriodMonth)
	if err != nil {
		t.Fatalf("GetTSD failed: %v", err)
	}

	if updated.Status != TSDSubmitted {
		t.Errorf("expected status %s, got %s", TSDSubmitted, updated.Status)
	}

	if updated.EMTAReference != emtaReference {
		t.Errorf("expected EMTA reference %s, got %s", emtaReference, updated.EMTAReference)
	}

	if updated.SubmittedAt == nil {
		t.Error("expected submitted_at to be set")
	}
}

func TestService_MarkTSDAccepted(t *testing.T) {
	service, tenant, tsd := setupTSDForExportTest(t)
	ctx := context.Background()

	// First mark as submitted
	err := service.MarkTSDSubmitted(ctx, tenant.SchemaName, tenant.ID, tsd.ID, "EMTA-TEST")
	if err != nil {
		t.Fatalf("MarkTSDSubmitted failed: %v", err)
	}

	// Then mark as accepted
	err = service.MarkTSDAccepted(ctx, tenant.SchemaName, tenant.ID, tsd.ID)
	if err != nil {
		t.Fatalf("MarkTSDAccepted failed: %v", err)
	}

	// Verify status
	updated, err := service.GetTSD(ctx, tenant.SchemaName, tenant.ID, tsd.PeriodYear, tsd.PeriodMonth)
	if err != nil {
		t.Fatalf("GetTSD failed: %v", err)
	}

	if updated.Status != TSDAccepted {
		t.Errorf("expected status %s, got %s", TSDAccepted, updated.Status)
	}
}

func TestService_MarkTSDRejected(t *testing.T) {
	service, tenant, tsd := setupTSDForExportTest(t)
	ctx := context.Background()

	// First mark as submitted
	err := service.MarkTSDSubmitted(ctx, tenant.SchemaName, tenant.ID, tsd.ID, "EMTA-TEST")
	if err != nil {
		t.Fatalf("MarkTSDSubmitted failed: %v", err)
	}

	// Then mark as rejected
	err = service.MarkTSDRejected(ctx, tenant.SchemaName, tenant.ID, tsd.ID)
	if err != nil {
		t.Fatalf("MarkTSDRejected failed: %v", err)
	}

	// Verify status
	updated, err := service.GetTSD(ctx, tenant.SchemaName, tenant.ID, tsd.PeriodYear, tsd.PeriodMonth)
	if err != nil {
		t.Fatalf("GetTSD failed: %v", err)
	}

	if updated.Status != TSDRejected {
		t.Errorf("expected status %s, got %s", TSDRejected, updated.Status)
	}
}

