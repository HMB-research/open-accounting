-- =============================================================================
-- Migration 012: Fix add_payroll_tables percent escapes
-- The comments in format() strings contained unescaped % characters
-- which are interpreted as format specifiers. This fixes them.
-- =============================================================================

-- Recreate add_payroll_tables with escaped percent signs in comments
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
            personal_code VARCHAR(20),
            email VARCHAR(255),
            phone VARCHAR(50),
            address TEXT,
            bank_account VARCHAR(50),

            start_date DATE NOT NULL,
            end_date DATE,
            position VARCHAR(200),
            department VARCHAR(200),
            employment_type VARCHAR(20) DEFAULT ''FULL_TIME'',

            tax_residency VARCHAR(2) DEFAULT ''EE'',
            apply_basic_exemption BOOLEAN DEFAULT true,
            basic_exemption_amount NUMERIC(10, 2) DEFAULT 700.00,
            funded_pension_rate NUMERIC(5, 2) DEFAULT 0.00,

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
            component_type VARCHAR(50) NOT NULL,
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
            status VARCHAR(20) DEFAULT ''DRAFT'',
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

            gross_salary NUMERIC(18, 2) NOT NULL,
            taxable_income NUMERIC(18, 2) NOT NULL,

            income_tax NUMERIC(18, 2) DEFAULT 0,
            unemployment_insurance_employee NUMERIC(18, 2) DEFAULT 0,
            funded_pension NUMERIC(18, 2) DEFAULT 0,
            other_deductions NUMERIC(18, 2) DEFAULT 0,

            net_salary NUMERIC(18, 2) NOT NULL,

            social_tax NUMERIC(18, 2) DEFAULT 0,
            unemployment_insurance_employer NUMERIC(18, 2) DEFAULT 0,

            total_employer_cost NUMERIC(18, 2) NOT NULL,

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

            total_payments NUMERIC(18, 2) DEFAULT 0,
            total_income_tax NUMERIC(18, 2) DEFAULT 0,
            total_social_tax NUMERIC(18, 2) DEFAULT 0,
            total_unemployment_employer NUMERIC(18, 2) DEFAULT 0,
            total_unemployment_employee NUMERIC(18, 2) DEFAULT 0,
            total_funded_pension NUMERIC(18, 2) DEFAULT 0,

            status VARCHAR(20) DEFAULT ''DRAFT'',
            submitted_at TIMESTAMPTZ,
            emta_reference VARCHAR(100),

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

            personal_code VARCHAR(20) NOT NULL,
            first_name VARCHAR(100) NOT NULL,
            last_name VARCHAR(100) NOT NULL,

            payment_type VARCHAR(10) DEFAULT ''10'',
            gross_payment NUMERIC(18, 2) NOT NULL,
            basic_exemption NUMERIC(18, 2) DEFAULT 0,
            taxable_amount NUMERIC(18, 2) NOT NULL,

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

    -- Add foreign keys (using exception handling since IF NOT EXISTS is not supported for constraints)
    BEGIN
        EXECUTE format('ALTER TABLE %I.salary_components ADD CONSTRAINT fk_salary_employee
            FOREIGN KEY (employee_id) REFERENCES %I.employees(id) ON DELETE CASCADE', schema_name, schema_name);
    EXCEPTION WHEN duplicate_object THEN NULL;
    END;

    BEGIN
        EXECUTE format('ALTER TABLE %I.payslips ADD CONSTRAINT fk_payslip_run
            FOREIGN KEY (payroll_run_id) REFERENCES %I.payroll_runs(id) ON DELETE CASCADE', schema_name, schema_name);
    EXCEPTION WHEN duplicate_object THEN NULL;
    END;

    BEGIN
        EXECUTE format('ALTER TABLE %I.payslips ADD CONSTRAINT fk_payslip_employee
            FOREIGN KEY (employee_id) REFERENCES %I.employees(id)', schema_name, schema_name);
    EXCEPTION WHEN duplicate_object THEN NULL;
    END;

    BEGIN
        EXECUTE format('ALTER TABLE %I.tsd_declarations ADD CONSTRAINT fk_tsd_payroll
            FOREIGN KEY (payroll_run_id) REFERENCES %I.payroll_runs(id)', schema_name, schema_name);
    EXCEPTION WHEN duplicate_object THEN NULL;
    END;

    BEGIN
        EXECUTE format('ALTER TABLE %I.tsd_rows ADD CONSTRAINT fk_tsd_row_declaration
            FOREIGN KEY (declaration_id) REFERENCES %I.tsd_declarations(id) ON DELETE CASCADE', schema_name, schema_name);
    EXCEPTION WHEN duplicate_object THEN NULL;
    END;

    BEGIN
        EXECUTE format('ALTER TABLE %I.tsd_rows ADD CONSTRAINT fk_tsd_row_employee
            FOREIGN KEY (employee_id) REFERENCES %I.employees(id)', schema_name, schema_name);
    EXCEPTION WHEN duplicate_object THEN NULL;
    END;
END;
$$ LANGUAGE plpgsql;
