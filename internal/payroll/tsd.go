package payroll

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// GenerateTSD generates a TSD declaration from a payroll run
func (s *Service) GenerateTSD(ctx context.Context, schemaName, tenantID, payrollRunID string) (*TSDDeclaration, error) {
	// Get the payroll run
	run, err := s.GetPayrollRun(ctx, schemaName, tenantID, payrollRunID)
	if err != nil {
		return nil, err
	}

	if run.Status != PayrollApproved && run.Status != PayrollPaid {
		return nil, fmt.Errorf("payroll run must be APPROVED or PAID to generate TSD")
	}

	// Get payslips with employee data
	payslips, err := s.GetPayslipsWithEmployees(ctx, schemaName, tenantID, payrollRunID)
	if err != nil {
		return nil, err
	}

	if len(payslips) == 0 {
		return nil, fmt.Errorf("no payslips found for this payroll run")
	}

	// Create TSD declaration
	tsd := &TSDDeclaration{
		ID:           uuid.New().String(),
		TenantID:     tenantID,
		PeriodYear:   run.PeriodYear,
		PeriodMonth:  run.PeriodMonth,
		PayrollRunID: payrollRunID,
		Status:       TSDDraft,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	// First pass: calculate totals and create row objects
	rows := make([]TSDRow, 0, len(payslips))
	for _, ps := range payslips {
		if ps.Employee == nil {
			continue
		}

		row := TSDRow{
			ID:             uuid.New().String(),
			TenantID:       tenantID,
			DeclarationID:  tsd.ID,
			EmployeeID:     ps.EmployeeID,
			PersonalCode:   ps.Employee.PersonalCode,
			FirstName:      ps.Employee.FirstName,
			LastName:       ps.Employee.LastName,
			PaymentType:    "10", // Regular salary
			GrossPayment:   ps.GrossSalary,
			BasicExemption: ps.BasicExemptionApplied,
			TaxableAmount:  ps.TaxableIncome,
			IncomeTax:      ps.IncomeTax,
			SocialTax:      ps.SocialTax,
			UnemploymentER: ps.UnemploymentInsuranceER,
			UnemploymentEE: ps.UnemploymentInsuranceEE,
			FundedPension:  ps.FundedPension,
			CreatedAt:      time.Now(),
		}

		rows = append(rows, row)

		// Accumulate totals
		tsd.TotalPayments = tsd.TotalPayments.Add(row.GrossPayment)
		tsd.TotalIncomeTax = tsd.TotalIncomeTax.Add(row.IncomeTax)
		tsd.TotalSocialTax = tsd.TotalSocialTax.Add(row.SocialTax)
		tsd.TotalUnemploymentER = tsd.TotalUnemploymentER.Add(row.UnemploymentER)
		tsd.TotalUnemploymentEE = tsd.TotalUnemploymentEE.Add(row.UnemploymentEE)
		tsd.TotalFundedPension = tsd.TotalFundedPension.Add(row.FundedPension)
	}

	if err := s.repo.WithTransaction(ctx, func(txRepo Repository) error {
		if err := txRepo.DeleteTSDByPeriod(ctx, schemaName, tenantID, run.PeriodYear, run.PeriodMonth); err != nil {
			return fmt.Errorf("delete existing TSD: %w", err)
		}
		if err := txRepo.CreateTSDDeclaration(ctx, schemaName, tsd); err != nil {
			return fmt.Errorf("insert TSD declaration: %w", err)
		}
		if err := txRepo.CreateTSDRows(ctx, schemaName, rows); err != nil {
			return fmt.Errorf("insert TSD rows: %w", err)
		}
		return nil
	}); err != nil {
		return nil, err
	}

	tsd.Rows = rows
	tsd.RemediationActions = BuildTSDRemediationActions(tsd)
	return tsd, nil
}

// GetTSD retrieves a TSD declaration by period
func (s *Service) GetTSD(ctx context.Context, schemaName, tenantID string, year, month int) (*TSDDeclaration, error) {
	tsd, err := s.repo.GetTSD(ctx, schemaName, tenantID, year, month)
	if err == ErrTSDDeclarationNotFound {
		return nil, fmt.Errorf("TSD declaration not found")
	}
	if err != nil {
		return nil, fmt.Errorf("get TSD: %w", err)
	}
	tsd.RemediationActions = BuildTSDRemediationActions(tsd)
	return tsd, nil
}

// GetTSDRows retrieves all rows for a TSD declaration
func (s *Service) GetTSDRows(ctx context.Context, schemaName, tenantID, declarationID string) ([]TSDRow, error) {
	rows, err := s.repo.GetTSDRows(ctx, schemaName, tenantID, declarationID)
	if err != nil {
		return nil, fmt.Errorf("get TSD rows: %w", err)
	}
	return rows, nil
}

// ListTSD lists TSD declarations for a tenant
func (s *Service) ListTSD(ctx context.Context, schemaName, tenantID string, filter TSDListFilter) ([]TSDDeclaration, error) {
	declarations, err := s.repo.ListTSD(ctx, schemaName, tenantID, filter)
	if err != nil {
		return nil, fmt.Errorf("list TSD: %w", err)
	}
	for i := range declarations {
		declarations[i].RemediationActions = BuildTSDRemediationActions(&declarations[i])
	}
	return declarations, nil
}

// GetPayslipsWithEmployees retrieves payslips with employee data
func (s *Service) GetPayslipsWithEmployees(ctx context.Context, schemaName, tenantID, payrollRunID string) ([]Payslip, error) {
	payslips, err := s.repo.GetPayslipsWithEmployees(ctx, schemaName, tenantID, payrollRunID)
	if err != nil {
		return nil, fmt.Errorf("get payslips: %w", err)
	}
	return payslips, nil
}

// CalculateTaxPreview calculates tax preview for a given gross salary
func CalculateTaxPreview(grossSalary decimal.Decimal, applyBasicExemption bool, fundedPensionRate decimal.Decimal) TaxCalculation {
	basicExemption := decimal.Zero
	if applyBasicExemption {
		basicExemption = DefaultBasicExemption
	}
	return CalculateEstonianTaxes(grossSalary, basicExemption, fundedPensionRate)
}
