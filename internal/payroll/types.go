package payroll

import (
	"time"

	"github.com/shopspring/decimal"
)

// Estonian tax rates for 2025
var (
	IncomeTaxRate              = decimal.NewFromFloat(0.22)   // 22%
	SocialTaxRate              = decimal.NewFromFloat(0.33)   // 33%
	UnemploymentEmployeeRate   = decimal.NewFromFloat(0.016)  // 1.6%
	UnemploymentEmployerRate   = decimal.NewFromFloat(0.008)  // 0.8%
	DefaultBasicExemption      = decimal.NewFromFloat(700.00) // €700/month in 2025
	MinimumSocialTaxBase       = decimal.NewFromFloat(820.00) // €820/month in 2025
	MinimumSocialTax           = decimal.NewFromFloat(270.60) // 33% of €820
	FundedPensionRateDefault   = decimal.NewFromFloat(0.02)   // 2%
	FundedPensionRateIncreased = decimal.NewFromFloat(0.04)   // 4% (optional)
)

// EmploymentType defines the type of employment
type EmploymentType string

const (
	EmploymentFullTime EmploymentType = "FULL_TIME"
	EmploymentPartTime EmploymentType = "PART_TIME"
	EmploymentContract EmploymentType = "CONTRACT"
)

// PayrollStatus defines the status of a payroll run
type PayrollStatus string

const (
	PayrollDraft      PayrollStatus = "DRAFT"
	PayrollCalculated PayrollStatus = "CALCULATED"
	PayrollApproved   PayrollStatus = "APPROVED"
	PayrollPaid       PayrollStatus = "PAID"
	PayrollDeclared   PayrollStatus = "DECLARED"
)

// TSDStatus defines the status of a TSD declaration
type TSDStatus string

const (
	TSDDraft     TSDStatus = "DRAFT"
	TSDSubmitted TSDStatus = "SUBMITTED"
	TSDAccepted  TSDStatus = "ACCEPTED"
	TSDRejected  TSDStatus = "REJECTED"
)

// Employee represents an employee in the payroll system
type Employee struct {
	ID             string `json:"id"`
	TenantID       string `json:"tenant_id"`
	EmployeeNumber string `json:"employee_number,omitempty"`
	FirstName      string `json:"first_name"`
	LastName       string `json:"last_name"`
	PersonalCode   string `json:"personal_code,omitempty"` // Estonian isikukood
	Email          string `json:"email,omitempty"`
	Phone          string `json:"phone,omitempty"`
	Address        string `json:"address,omitempty"`
	BankAccount    string `json:"bank_account,omitempty"` // IBAN

	// Employment details
	StartDate      time.Time      `json:"start_date"`
	EndDate        *time.Time     `json:"end_date,omitempty"`
	Position       string         `json:"position,omitempty"`
	Department     string         `json:"department,omitempty"`
	EmploymentType EmploymentType `json:"employment_type"`

	// Tax settings
	TaxResidency         string          `json:"tax_residency"`
	ApplyBasicExemption  bool            `json:"apply_basic_exemption"`
	BasicExemptionAmount decimal.Decimal `json:"basic_exemption_amount"`
	FundedPensionRate    decimal.Decimal `json:"funded_pension_rate"`

	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// FullName returns the employee's full name
func (e *Employee) FullName() string {
	return e.FirstName + " " + e.LastName
}

// SalaryComponent represents a salary component (base, bonus, etc.)
type SalaryComponent struct {
	ID            string          `json:"id"`
	TenantID      string          `json:"tenant_id"`
	EmployeeID    string          `json:"employee_id"`
	ComponentType string          `json:"component_type"` // BASE_SALARY, BONUS, COMMISSION, BENEFIT, DEDUCTION
	Name          string          `json:"name"`
	Amount        decimal.Decimal `json:"amount"`
	IsTaxable     bool            `json:"is_taxable"`
	IsRecurring   bool            `json:"is_recurring"`
	EffectiveFrom time.Time       `json:"effective_from"`
	EffectiveTo   *time.Time      `json:"effective_to,omitempty"`
	CreatedAt     time.Time       `json:"created_at"`
}

// PayrollRun represents a monthly payroll calculation
type PayrollRun struct {
	ID                string          `json:"id"`
	TenantID          string          `json:"tenant_id"`
	PeriodYear        int             `json:"period_year"`
	PeriodMonth       int             `json:"period_month"`
	Status            PayrollStatus   `json:"status"`
	PaymentDate       *time.Time      `json:"payment_date,omitempty"`
	TotalGross        decimal.Decimal `json:"total_gross"`
	TotalNet          decimal.Decimal `json:"total_net"`
	TotalEmployerCost decimal.Decimal `json:"total_employer_cost"`
	Notes             string          `json:"notes,omitempty"`
	CreatedBy         string          `json:"created_by,omitempty"`
	ApprovedBy        string          `json:"approved_by,omitempty"`
	ApprovedAt        *time.Time      `json:"approved_at,omitempty"`
	CreatedAt         time.Time       `json:"created_at"`
	UpdatedAt         time.Time       `json:"updated_at"`

	// Loaded relations
	Payslips []Payslip `json:"payslips,omitempty"`
}

// Payslip represents an individual employee's payslip
type Payslip struct {
	ID           string `json:"id"`
	TenantID     string `json:"tenant_id"`
	PayrollRunID string `json:"payroll_run_id"`
	EmployeeID   string `json:"employee_id"`

	// Earnings
	GrossSalary   decimal.Decimal `json:"gross_salary"`
	TaxableIncome decimal.Decimal `json:"taxable_income"`

	// Employee deductions (withheld from gross)
	IncomeTax               decimal.Decimal `json:"income_tax"`
	UnemploymentInsuranceEE decimal.Decimal `json:"unemployment_insurance_employee"`
	FundedPension           decimal.Decimal `json:"funded_pension"`
	OtherDeductions         decimal.Decimal `json:"other_deductions"`

	// Net pay (what employee receives)
	NetSalary decimal.Decimal `json:"net_salary"`

	// Employer costs (on top of gross)
	SocialTax               decimal.Decimal `json:"social_tax"`
	UnemploymentInsuranceER decimal.Decimal `json:"unemployment_insurance_employer"`

	// Total cost to employer
	TotalEmployerCost decimal.Decimal `json:"total_employer_cost"`

	// Tax calculation details
	BasicExemptionApplied decimal.Decimal `json:"basic_exemption_applied"`

	PaymentStatus string     `json:"payment_status"`
	PaidAt        *time.Time `json:"paid_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`

	// Loaded relations
	Employee *Employee `json:"employee,omitempty"`
}

// TSDDeclaration represents a TSD tax declaration for a period
type TSDDeclaration struct {
	ID           string `json:"id"`
	TenantID     string `json:"tenant_id"`
	PeriodYear   int    `json:"period_year"`
	PeriodMonth  int    `json:"period_month"`
	PayrollRunID string `json:"payroll_run_id,omitempty"`

	// Totals for the declaration
	TotalPayments       decimal.Decimal `json:"total_payments"`
	TotalIncomeTax      decimal.Decimal `json:"total_income_tax"`
	TotalSocialTax      decimal.Decimal `json:"total_social_tax"`
	TotalUnemploymentER decimal.Decimal `json:"total_unemployment_employer"`
	TotalUnemploymentEE decimal.Decimal `json:"total_unemployment_employee"`
	TotalFundedPension  decimal.Decimal `json:"total_funded_pension"`

	Status        TSDStatus  `json:"status"`
	SubmittedAt   *time.Time `json:"submitted_at,omitempty"`
	EMTAReference string     `json:"emta_reference,omitempty"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// Loaded relations
	Rows []TSDRow `json:"rows,omitempty"`
}

// TSDRow represents a single row in TSD Annex 1 (payments to residents)
type TSDRow struct {
	ID            string `json:"id"`
	TenantID      string `json:"tenant_id"`
	DeclarationID string `json:"declaration_id"`
	EmployeeID    string `json:"employee_id"`

	// Employee identification
	PersonalCode string `json:"personal_code"`
	FirstName    string `json:"first_name"`
	LastName     string `json:"last_name"`

	// Payment details
	PaymentType    string          `json:"payment_type"` // TSD payment type code (e.g., "10" for salary)
	GrossPayment   decimal.Decimal `json:"gross_payment"`
	BasicExemption decimal.Decimal `json:"basic_exemption"`
	TaxableAmount  decimal.Decimal `json:"taxable_amount"`

	// Taxes
	IncomeTax      decimal.Decimal `json:"income_tax"`
	SocialTax      decimal.Decimal `json:"social_tax"`
	UnemploymentER decimal.Decimal `json:"unemployment_insurance_employer"`
	UnemploymentEE decimal.Decimal `json:"unemployment_insurance_employee"`
	FundedPension  decimal.Decimal `json:"funded_pension"`

	CreatedAt time.Time `json:"created_at"`
}

// Request/Response types

// CreateEmployeeRequest is the request to create an employee
type CreateEmployeeRequest struct {
	EmployeeNumber       string          `json:"employee_number,omitempty"`
	FirstName            string          `json:"first_name"`
	LastName             string          `json:"last_name"`
	PersonalCode         string          `json:"personal_code,omitempty"`
	Email                string          `json:"email,omitempty"`
	Phone                string          `json:"phone,omitempty"`
	Address              string          `json:"address,omitempty"`
	BankAccount          string          `json:"bank_account,omitempty"`
	StartDate            time.Time       `json:"start_date"`
	Position             string          `json:"position,omitempty"`
	Department           string          `json:"department,omitempty"`
	EmploymentType       EmploymentType  `json:"employment_type"`
	ApplyBasicExemption  bool            `json:"apply_basic_exemption"`
	BasicExemptionAmount decimal.Decimal `json:"basic_exemption_amount,omitempty"`
	FundedPensionRate    decimal.Decimal `json:"funded_pension_rate,omitempty"`
}

// CreatePayrollRunRequest is the request to create a payroll run
type CreatePayrollRunRequest struct {
	PeriodYear  int        `json:"period_year"`
	PeriodMonth int        `json:"period_month"`
	PaymentDate *time.Time `json:"payment_date,omitempty"`
	Notes       string     `json:"notes,omitempty"`
}

// UpdateEmployeeRequest is the request to update an employee
type UpdateEmployeeRequest struct {
	EmployeeNumber       string           `json:"employee_number,omitempty"`
	FirstName            string           `json:"first_name,omitempty"`
	LastName             string           `json:"last_name,omitempty"`
	PersonalCode         string           `json:"personal_code,omitempty"`
	Email                string           `json:"email,omitempty"`
	Phone                string           `json:"phone,omitempty"`
	Address              string           `json:"address,omitempty"`
	BankAccount          string           `json:"bank_account,omitempty"`
	EndDate              *time.Time       `json:"end_date,omitempty"`
	Position             string           `json:"position,omitempty"`
	Department           string           `json:"department,omitempty"`
	EmploymentType       EmploymentType   `json:"employment_type,omitempty"`
	ApplyBasicExemption  *bool            `json:"apply_basic_exemption,omitempty"`
	BasicExemptionAmount *decimal.Decimal `json:"basic_exemption_amount,omitempty"`
	FundedPensionRate    *decimal.Decimal `json:"funded_pension_rate,omitempty"`
	IsActive             *bool            `json:"is_active,omitempty"`
}

// TaxCalculation holds the breakdown of taxes for a salary
type TaxCalculation struct {
	GrossSalary    decimal.Decimal `json:"gross_salary"`
	BasicExemption decimal.Decimal `json:"basic_exemption"`
	TaxableIncome  decimal.Decimal `json:"taxable_income"`

	// Employee deductions
	IncomeTax       decimal.Decimal `json:"income_tax"`
	UnemploymentEE  decimal.Decimal `json:"unemployment_employee"`
	FundedPension   decimal.Decimal `json:"funded_pension"`
	TotalDeductions decimal.Decimal `json:"total_deductions"`

	NetSalary decimal.Decimal `json:"net_salary"`

	// Employer costs
	SocialTax         decimal.Decimal `json:"social_tax"`
	UnemploymentER    decimal.Decimal `json:"unemployment_employer"`
	TotalEmployerCost decimal.Decimal `json:"total_employer_cost"`
}

// CalculateEstonianTaxes calculates all Estonian payroll taxes
func CalculateEstonianTaxes(grossSalary decimal.Decimal, basicExemption decimal.Decimal, fundedPensionRate decimal.Decimal) TaxCalculation {
	calc := TaxCalculation{
		GrossSalary:    grossSalary,
		BasicExemption: basicExemption,
	}

	// 1. Calculate unemployment insurance (employee) - from gross
	calc.UnemploymentEE = grossSalary.Mul(UnemploymentEmployeeRate).Round(2)

	// 2. Calculate funded pension (II pillar) - from gross
	calc.FundedPension = grossSalary.Mul(fundedPensionRate).Round(2)

	// 3. Calculate taxable income (gross - basic exemption)
	calc.TaxableIncome = grossSalary.Sub(basicExemption)
	if calc.TaxableIncome.IsNegative() {
		calc.TaxableIncome = decimal.Zero
	}

	// 4. Calculate income tax (22% of taxable income)
	calc.IncomeTax = calc.TaxableIncome.Mul(IncomeTaxRate).Round(2)

	// 5. Total employee deductions
	calc.TotalDeductions = calc.IncomeTax.Add(calc.UnemploymentEE).Add(calc.FundedPension)

	// 6. Net salary
	calc.NetSalary = grossSalary.Sub(calc.TotalDeductions)

	// 7. Employer costs
	// Social tax (33% of gross, minimum €270.60)
	calc.SocialTax = grossSalary.Mul(SocialTaxRate).Round(2)
	if calc.SocialTax.LessThan(MinimumSocialTax) && grossSalary.GreaterThan(decimal.Zero) {
		calc.SocialTax = MinimumSocialTax
	}

	// Unemployment insurance (employer) - 0.8%
	calc.UnemploymentER = grossSalary.Mul(UnemploymentEmployerRate).Round(2)

	// Total employer cost = gross + social tax + unemployment (employer)
	calc.TotalEmployerCost = grossSalary.Add(calc.SocialTax).Add(calc.UnemploymentER)

	return calc
}
