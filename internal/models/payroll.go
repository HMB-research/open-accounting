package models

import (
	"time"
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

// Employee represents an employee in the payroll system (GORM model)
type Employee struct {
	ID             string `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	TenantID       string `gorm:"type:uuid;not null;index" json:"tenant_id"`
	EmployeeNumber string `gorm:"column:employee_number;size:50" json:"employee_number,omitempty"`
	FirstName      string `gorm:"column:first_name;size:100;not null" json:"first_name"`
	LastName       string `gorm:"column:last_name;size:100;not null" json:"last_name"`
	PersonalCode   string `gorm:"column:personal_code;size:20" json:"personal_code,omitempty"`
	Email          string `gorm:"size:255" json:"email,omitempty"`
	Phone          string `gorm:"size:50" json:"phone,omitempty"`
	Address        string `gorm:"type:text" json:"address,omitempty"`
	BankAccount    string `gorm:"column:bank_account;size:50" json:"bank_account,omitempty"`

	StartDate      time.Time      `gorm:"column:start_date;type:date;not null" json:"start_date"`
	EndDate        *time.Time     `gorm:"column:end_date;type:date" json:"end_date,omitempty"`
	Position       string         `gorm:"size:100" json:"position,omitempty"`
	Department     string         `gorm:"size:100" json:"department,omitempty"`
	EmploymentType EmploymentType `gorm:"column:employment_type;size:20;not null" json:"employment_type"`

	TaxResidency         string  `gorm:"column:tax_residency;size:10;default:'EE'" json:"tax_residency"`
	ApplyBasicExemption  bool    `gorm:"column:apply_basic_exemption;not null;default:true" json:"apply_basic_exemption"`
	BasicExemptionAmount Decimal `gorm:"column:basic_exemption_amount;type:numeric(28,8);not null;default:0" json:"basic_exemption_amount"`
	FundedPensionRate    Decimal `gorm:"column:funded_pension_rate;type:numeric(5,4);not null;default:0.02" json:"funded_pension_rate"`

	IsActive  bool      `gorm:"column:is_active;not null;default:true" json:"is_active"`
	CreatedAt time.Time `gorm:"not null;default:now()" json:"created_at"`
	UpdatedAt time.Time `gorm:"not null;default:now()" json:"updated_at"`
}

// TableName returns the table name for GORM
func (Employee) TableName() string {
	return "employees"
}

// SalaryComponent represents a salary component (GORM model)
type SalaryComponent struct {
	ID            string     `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	TenantID      string     `gorm:"type:uuid;not null;index" json:"tenant_id"`
	EmployeeID    string     `gorm:"column:employee_id;type:uuid;not null;index" json:"employee_id"`
	ComponentType string     `gorm:"column:component_type;size:50;not null" json:"component_type"`
	Name          string     `gorm:"size:100;not null" json:"name"`
	Amount        Decimal    `gorm:"type:numeric(28,8);not null" json:"amount"`
	IsTaxable     bool       `gorm:"column:is_taxable;not null;default:true" json:"is_taxable"`
	IsRecurring   bool       `gorm:"column:is_recurring;not null;default:true" json:"is_recurring"`
	EffectiveFrom time.Time  `gorm:"column:effective_from;type:date;not null" json:"effective_from"`
	EffectiveTo   *time.Time `gorm:"column:effective_to;type:date" json:"effective_to,omitempty"`
	CreatedAt     time.Time  `gorm:"not null;default:now()" json:"created_at"`

	// Relations
	Employee *Employee `gorm:"foreignKey:EmployeeID" json:"employee,omitempty"`
}

// TableName returns the table name for GORM
func (SalaryComponent) TableName() string {
	return "salary_components"
}

// PayrollRun represents a monthly payroll calculation (GORM model)
type PayrollRun struct {
	ID                string        `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	TenantID          string        `gorm:"type:uuid;not null;index" json:"tenant_id"`
	PeriodYear        int           `gorm:"column:period_year;not null" json:"period_year"`
	PeriodMonth       int           `gorm:"column:period_month;not null" json:"period_month"`
	Status            PayrollStatus `gorm:"size:20;not null;default:'DRAFT'" json:"status"`
	PaymentDate       *time.Time    `gorm:"column:payment_date;type:date" json:"payment_date,omitempty"`
	TotalGross        Decimal       `gorm:"column:total_gross;type:numeric(28,8);not null;default:0" json:"total_gross"`
	TotalNet          Decimal       `gorm:"column:total_net;type:numeric(28,8);not null;default:0" json:"total_net"`
	TotalEmployerCost Decimal       `gorm:"column:total_employer_cost;type:numeric(28,8);not null;default:0" json:"total_employer_cost"`
	Notes             string        `gorm:"type:text" json:"notes,omitempty"`
	CreatedBy         string        `gorm:"column:created_by;type:uuid" json:"created_by,omitempty"`
	ApprovedBy        string        `gorm:"column:approved_by;type:uuid" json:"approved_by,omitempty"`
	ApprovedAt        *time.Time    `gorm:"column:approved_at" json:"approved_at,omitempty"`
	CreatedAt         time.Time     `gorm:"not null;default:now()" json:"created_at"`
	UpdatedAt         time.Time     `gorm:"not null;default:now()" json:"updated_at"`

	// Relations
	Payslips []Payslip `gorm:"foreignKey:PayrollRunID" json:"payslips,omitempty"`
}

// TableName returns the table name for GORM
func (PayrollRun) TableName() string {
	return "payroll_runs"
}

// Payslip represents an individual employee's payslip (GORM model)
type Payslip struct {
	ID           string `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	TenantID     string `gorm:"type:uuid;not null;index" json:"tenant_id"`
	PayrollRunID string `gorm:"column:payroll_run_id;type:uuid;not null;index" json:"payroll_run_id"`
	EmployeeID   string `gorm:"column:employee_id;type:uuid;not null;index" json:"employee_id"`

	GrossSalary             Decimal `gorm:"column:gross_salary;type:numeric(28,8);not null;default:0" json:"gross_salary"`
	TaxableIncome           Decimal `gorm:"column:taxable_income;type:numeric(28,8);not null;default:0" json:"taxable_income"`
	IncomeTax               Decimal `gorm:"column:income_tax;type:numeric(28,8);not null;default:0" json:"income_tax"`
	UnemploymentInsuranceEE Decimal `gorm:"column:unemployment_insurance_employee;type:numeric(28,8);not null;default:0" json:"unemployment_insurance_employee"`
	FundedPension           Decimal `gorm:"column:funded_pension;type:numeric(28,8);not null;default:0" json:"funded_pension"`
	OtherDeductions         Decimal `gorm:"column:other_deductions;type:numeric(28,8);not null;default:0" json:"other_deductions"`
	NetSalary               Decimal `gorm:"column:net_salary;type:numeric(28,8);not null;default:0" json:"net_salary"`
	SocialTax               Decimal `gorm:"column:social_tax;type:numeric(28,8);not null;default:0" json:"social_tax"`
	UnemploymentInsuranceER Decimal `gorm:"column:unemployment_insurance_employer;type:numeric(28,8);not null;default:0" json:"unemployment_insurance_employer"`
	TotalEmployerCost       Decimal `gorm:"column:total_employer_cost;type:numeric(28,8);not null;default:0" json:"total_employer_cost"`
	BasicExemptionApplied   Decimal `gorm:"column:basic_exemption_applied;type:numeric(28,8);not null;default:0" json:"basic_exemption_applied"`

	PaymentStatus string     `gorm:"column:payment_status;size:20;default:'PENDING'" json:"payment_status"`
	PaidAt        *time.Time `gorm:"column:paid_at" json:"paid_at,omitempty"`
	CreatedAt     time.Time  `gorm:"not null;default:now()" json:"created_at"`

	// Relations
	PayrollRun *PayrollRun `gorm:"foreignKey:PayrollRunID" json:"payroll_run,omitempty"`
	Employee   *Employee   `gorm:"foreignKey:EmployeeID" json:"employee,omitempty"`
}

// TableName returns the table name for GORM
func (Payslip) TableName() string {
	return "payslips"
}
