-- Payroll module for Estonian tax compliance (TSD)
-- Migration: 007_payroll

-- Function to add payroll tables to a tenant schema
CREATE OR REPLACE FUNCTION add_payroll_tables(schema_name TEXT) RETURNS VOID AS $$
BEGIN
    -- Employees table
    EXECUTE format('
        CREATE TABLE IF NOT EXISTS %I.employees (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            tenant_id UUID NOT NULL,
            employee_number VARCHAR(50),
            first_name VARCHAR(100) NOT NULL,
            last_name VARCHAR(100) NOT NULL,
            personal_code VARCHAR(20),  -- Estonian isikukood
            email VARCHAR(255),
            phone VARCHAR(50),
            address TEXT,
            bank_account VARCHAR(50),   -- IBAN for salary payment

            -- Employment details
            start_date DATE NOT NULL,
            end_date DATE,
            position VARCHAR(200),
            department VARCHAR(200),
            employment_type VARCHAR(20) DEFAULT ''FULL_TIME'',  -- FULL_TIME, PART_TIME, CONTRACT

            -- Tax settings
            tax_residency VARCHAR(2) DEFAULT ''EE'',
            apply_basic_exemption BOOLEAN DEFAULT true,
            basic_exemption_amount NUMERIC(10, 2) DEFAULT 700.00,  -- 2025: €700/month
            funded_pension_rate NUMERIC(5, 2) DEFAULT 0.00,  -- II pillar: 0%, 2%, or 4%

            is_active BOOLEAN DEFAULT true,
            created_at TIMESTAMPTZ DEFAULT NOW(),
            updated_at TIMESTAMPTZ DEFAULT NOW(),

            CONSTRAINT employees_employment_type_check
                CHECK (employment_type IN (''FULL_TIME'', ''PART_TIME'', ''CONTRACT''))
        )
    ', schema_name);

    -- Salary components (base salary, bonuses, etc.)
    EXECUTE format('
        CREATE TABLE IF NOT EXISTS %I.salary_components (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            tenant_id UUID NOT NULL,
            employee_id UUID NOT NULL,
            component_type VARCHAR(50) NOT NULL,  -- BASE_SALARY, BONUS, COMMISSION, BENEFIT, DEDUCTION
            name VARCHAR(200) NOT NULL,
            amount NUMERIC(18, 2) NOT NULL,
            is_taxable BOOLEAN DEFAULT true,
            is_recurring BOOLEAN DEFAULT true,
            effective_from DATE NOT NULL,
            effective_to DATE,
            created_at TIMESTAMPTZ DEFAULT NOW()
        )
    ', schema_name);

    -- Monthly payroll runs
    EXECUTE format('
        CREATE TABLE IF NOT EXISTS %I.payroll_runs (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            tenant_id UUID NOT NULL,
            period_year INTEGER NOT NULL,
            period_month INTEGER NOT NULL,
            status VARCHAR(20) DEFAULT ''DRAFT'',  -- DRAFT, CALCULATED, APPROVED, PAID, DECLARED
            payment_date DATE,
            total_gross NUMERIC(18, 2) DEFAULT 0,
            total_net NUMERIC(18, 2) DEFAULT 0,
            total_employer_cost NUMERIC(18, 2) DEFAULT 0,
            notes TEXT,
            created_by UUID,
            approved_by UUID,
            approved_at TIMESTAMPTZ,
            created_at TIMESTAMPTZ DEFAULT NOW(),
            updated_at TIMESTAMPTZ DEFAULT NOW(),

            CONSTRAINT payroll_runs_period_unique UNIQUE (tenant_id, period_year, period_month),
            CONSTRAINT payroll_runs_status_check
                CHECK (status IN (''DRAFT'', ''CALCULATED'', ''APPROVED'', ''PAID'', ''DECLARED''))
        )
    ', schema_name);

    -- Individual payslips within a payroll run
    EXECUTE format('
        CREATE TABLE IF NOT EXISTS %I.payslips (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            tenant_id UUID NOT NULL,
            payroll_run_id UUID NOT NULL,
            employee_id UUID NOT NULL,

            -- Earnings
            gross_salary NUMERIC(18, 2) NOT NULL,
            taxable_income NUMERIC(18, 2) NOT NULL,

            -- Employee deductions (withheld)
            income_tax NUMERIC(18, 2) DEFAULT 0,           -- 22%
            unemployment_insurance_employee NUMERIC(18, 2) DEFAULT 0,  -- 1.6%
            funded_pension NUMERIC(18, 2) DEFAULT 0,       -- 2% or 4%
            other_deductions NUMERIC(18, 2) DEFAULT 0,

            -- Net pay
            net_salary NUMERIC(18, 2) NOT NULL,

            -- Employer costs (on top of gross)
            social_tax NUMERIC(18, 2) DEFAULT 0,           -- 33%
            unemployment_insurance_employer NUMERIC(18, 2) DEFAULT 0,  -- 0.8%

            -- Total employer cost
            total_employer_cost NUMERIC(18, 2) NOT NULL,

            -- Tax calculation details (for TSD)
            basic_exemption_applied NUMERIC(18, 2) DEFAULT 0,

            payment_status VARCHAR(20) DEFAULT ''PENDING'',
            paid_at TIMESTAMPTZ,

            created_at TIMESTAMPTZ DEFAULT NOW(),

            CONSTRAINT payslips_payment_status_check
                CHECK (payment_status IN (''PENDING'', ''PAID'', ''CANCELLED''))
        )
    ', schema_name);

    -- TSD declarations
    EXECUTE format('
        CREATE TABLE IF NOT EXISTS %I.tsd_declarations (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            tenant_id UUID NOT NULL,
            period_year INTEGER NOT NULL,
            period_month INTEGER NOT NULL,
            payroll_run_id UUID,

            -- TSD totals
            total_payments NUMERIC(18, 2) DEFAULT 0,       -- All taxable payments
            total_income_tax NUMERIC(18, 2) DEFAULT 0,
            total_social_tax NUMERIC(18, 2) DEFAULT 0,
            total_unemployment_employer NUMERIC(18, 2) DEFAULT 0,
            total_unemployment_employee NUMERIC(18, 2) DEFAULT 0,
            total_funded_pension NUMERIC(18, 2) DEFAULT 0,

            status VARCHAR(20) DEFAULT ''DRAFT'',  -- DRAFT, SUBMITTED, ACCEPTED, REJECTED
            submitted_at TIMESTAMPTZ,
            emta_reference VARCHAR(100),  -- Reference from e-MTA after submission

            created_at TIMESTAMPTZ DEFAULT NOW(),
            updated_at TIMESTAMPTZ DEFAULT NOW(),

            CONSTRAINT tsd_declarations_period_unique UNIQUE (tenant_id, period_year, period_month),
            CONSTRAINT tsd_declarations_status_check
                CHECK (status IN (''DRAFT'', ''SUBMITTED'', ''ACCEPTED'', ''REJECTED''))
        )
    ', schema_name);

    -- TSD declaration rows (Annex 1 - payments to residents)
    EXECUTE format('
        CREATE TABLE IF NOT EXISTS %I.tsd_rows (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            tenant_id UUID NOT NULL,
            declaration_id UUID NOT NULL,
            employee_id UUID NOT NULL,

            -- Employee identification
            personal_code VARCHAR(20) NOT NULL,
            first_name VARCHAR(100) NOT NULL,
            last_name VARCHAR(100) NOT NULL,

            -- Payment details
            payment_type VARCHAR(10) DEFAULT ''10'',  -- TSD payment type code
            gross_payment NUMERIC(18, 2) NOT NULL,
            basic_exemption NUMERIC(18, 2) DEFAULT 0,
            taxable_amount NUMERIC(18, 2) NOT NULL,

            -- Taxes
            income_tax NUMERIC(18, 2) DEFAULT 0,
            social_tax NUMERIC(18, 2) DEFAULT 0,
            unemployment_insurance_employer NUMERIC(18, 2) DEFAULT 0,
            unemployment_insurance_employee NUMERIC(18, 2) DEFAULT 0,
            funded_pension NUMERIC(18, 2) DEFAULT 0,

            created_at TIMESTAMPTZ DEFAULT NOW()
        )
    ', schema_name);

    -- Create indexes
    EXECUTE format('CREATE INDEX IF NOT EXISTS idx_%I_employees_active ON %I.employees(tenant_id, is_active)',
        replace(schema_name, 'tenant_', '') || '_emp', schema_name);
    EXECUTE format('CREATE INDEX IF NOT EXISTS idx_%I_payroll_runs_period ON %I.payroll_runs(tenant_id, period_year, period_month)',
        replace(schema_name, 'tenant_', '') || '_pr', schema_name);
    EXECUTE format('CREATE INDEX IF NOT EXISTS idx_%I_payslips_run ON %I.payslips(payroll_run_id)',
        replace(schema_name, 'tenant_', '') || '_ps', schema_name);
    EXECUTE format('CREATE INDEX IF NOT EXISTS idx_%I_tsd_period ON %I.tsd_declarations(tenant_id, period_year, period_month)',
        replace(schema_name, 'tenant_', '') || '_tsd', schema_name);

    -- Add foreign keys
    EXECUTE format('ALTER TABLE %I.salary_components ADD CONSTRAINT fk_salary_employee
        FOREIGN KEY (employee_id) REFERENCES %I.employees(id) ON DELETE CASCADE', schema_name, schema_name);
    EXECUTE format('ALTER TABLE %I.payslips ADD CONSTRAINT fk_payslip_run
        FOREIGN KEY (payroll_run_id) REFERENCES %I.payroll_runs(id) ON DELETE CASCADE', schema_name, schema_name);
    EXECUTE format('ALTER TABLE %I.payslips ADD CONSTRAINT fk_payslip_employee
        FOREIGN KEY (employee_id) REFERENCES %I.employees(id)', schema_name, schema_name);
    EXECUTE format('ALTER TABLE %I.tsd_declarations ADD CONSTRAINT fk_tsd_payroll
        FOREIGN KEY (payroll_run_id) REFERENCES %I.payroll_runs(id)', schema_name, schema_name);
    EXECUTE format('ALTER TABLE %I.tsd_rows ADD CONSTRAINT fk_tsd_row_declaration
        FOREIGN KEY (declaration_id) REFERENCES %I.tsd_declarations(id) ON DELETE CASCADE', schema_name, schema_name);
    EXECUTE format('ALTER TABLE %I.tsd_rows ADD CONSTRAINT fk_tsd_row_employee
        FOREIGN KEY (employee_id) REFERENCES %I.employees(id)', schema_name, schema_name);
END;
$$ LANGUAGE plpgsql;

-- Apply to existing tenant schemas
DO $$
DECLARE
    schema_record RECORD;
BEGIN
    FOR schema_record IN
        SELECT schema_name FROM tenants WHERE is_active = true
    LOOP
        PERFORM add_payroll_tables(schema_record.schema_name);
    END LOOP;
END $$;

-- Update create_tenant_schema to include payroll tables
CREATE OR REPLACE FUNCTION create_tenant_schema(schema_name TEXT) RETURNS VOID AS $$
BEGIN
    -- Create the schema
    EXECUTE format('CREATE SCHEMA IF NOT EXISTS %I', schema_name);

    -- Create core accounting tables (existing)
    PERFORM create_accounting_tables(schema_name);

    -- Create payroll tables
    PERFORM add_payroll_tables(schema_name);
END;
$$ LANGUAGE plpgsql;
