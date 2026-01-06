-- Leave/Absence Management for Estonian payroll compliance
-- Migration: 017_leave_management
-- Supports: Annual leave, sick leave, parental leave, study leave, etc.

-- Function to add leave management tables to a tenant schema
CREATE OR REPLACE FUNCTION add_leave_management_tables(schema_name TEXT) RETURNS VOID AS $$
BEGIN
    -- Absence types (Puudumise tüübid)
    EXECUTE format('
        CREATE TABLE IF NOT EXISTS %I.absence_types (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            tenant_id UUID NOT NULL,
            code VARCHAR(50) NOT NULL,
            name VARCHAR(200) NOT NULL,
            name_et VARCHAR(200) NOT NULL,
            description TEXT,

            -- Configuration
            is_paid BOOLEAN DEFAULT true,
            affects_salary BOOLEAN DEFAULT true,
            requires_document BOOLEAN DEFAULT false,
            document_type VARCHAR(100),  -- TK66, medical certificate, etc.

            -- Accrual settings
            default_days_per_year NUMERIC(5, 2) DEFAULT 0,
            max_carryover_days NUMERIC(5, 2) DEFAULT 0,

            -- Estonian regulatory codes
            tsd_code VARCHAR(20),  -- Code for TSD declaration
            emta_code VARCHAR(20), -- EMTA (Tax Board) classification

            is_system BOOLEAN DEFAULT false,  -- System types cannot be deleted
            is_active BOOLEAN DEFAULT true,
            sort_order INTEGER DEFAULT 0,
            created_at TIMESTAMPTZ DEFAULT NOW(),
            updated_at TIMESTAMPTZ DEFAULT NOW(),

            CONSTRAINT absence_types_code_unique UNIQUE (tenant_id, code)
        )
    ', schema_name);

    -- Leave balances per employee per year
    EXECUTE format('
        CREATE TABLE IF NOT EXISTS %I.leave_balances (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            tenant_id UUID NOT NULL,
            employee_id UUID NOT NULL,
            absence_type_id UUID NOT NULL,
            year INTEGER NOT NULL,

            -- Days tracking
            entitled_days NUMERIC(5, 2) DEFAULT 0,    -- Total entitled for year
            carryover_days NUMERIC(5, 2) DEFAULT 0,   -- Carried from previous year
            used_days NUMERIC(5, 2) DEFAULT 0,        -- Already taken
            pending_days NUMERIC(5, 2) DEFAULT 0,     -- Requested but not approved
            remaining_days NUMERIC(5, 2) GENERATED ALWAYS AS (entitled_days + carryover_days - used_days - pending_days) STORED,

            notes TEXT,
            created_at TIMESTAMPTZ DEFAULT NOW(),
            updated_at TIMESTAMPTZ DEFAULT NOW(),

            CONSTRAINT leave_balances_unique UNIQUE (tenant_id, employee_id, absence_type_id, year)
        )
    ', schema_name);

    -- Leave records (individual absence entries)
    EXECUTE format('
        CREATE TABLE IF NOT EXISTS %I.leave_records (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            tenant_id UUID NOT NULL,
            employee_id UUID NOT NULL,
            absence_type_id UUID NOT NULL,

            -- Period
            start_date DATE NOT NULL,
            end_date DATE NOT NULL,

            -- Duration
            total_days NUMERIC(5, 2) NOT NULL,
            working_days NUMERIC(5, 2) NOT NULL,  -- Excluding weekends/holidays

            -- Status workflow
            status VARCHAR(20) DEFAULT ''PENDING'',  -- PENDING, APPROVED, REJECTED, CANCELLED

            -- Documentation
            document_number VARCHAR(100),  -- TK66 number, etc.
            document_date DATE,
            document_url TEXT,

            -- Approval workflow
            requested_at TIMESTAMPTZ DEFAULT NOW(),
            requested_by UUID,
            approved_at TIMESTAMPTZ,
            approved_by UUID,
            rejected_at TIMESTAMPTZ,
            rejected_by UUID,
            rejection_reason TEXT,

            -- Integration with payroll
            payroll_run_id UUID,  -- Link to payroll if affects salary

            notes TEXT,
            created_at TIMESTAMPTZ DEFAULT NOW(),
            updated_at TIMESTAMPTZ DEFAULT NOW(),

            CONSTRAINT leave_records_dates_check CHECK (end_date >= start_date),
            CONSTRAINT leave_records_status_check
                CHECK (status IN (''PENDING'', ''APPROVED'', ''REJECTED'', ''CANCELLED''))
        )
    ', schema_name);

    -- Indexes for performance
    EXECUTE format('
        CREATE INDEX IF NOT EXISTS idx_absence_types_tenant ON %I.absence_types(tenant_id);
        CREATE INDEX IF NOT EXISTS idx_leave_balances_employee ON %I.leave_balances(employee_id);
        CREATE INDEX IF NOT EXISTS idx_leave_balances_year ON %I.leave_balances(year);
        CREATE INDEX IF NOT EXISTS idx_leave_records_employee ON %I.leave_records(employee_id);
        CREATE INDEX IF NOT EXISTS idx_leave_records_dates ON %I.leave_records(start_date, end_date);
        CREATE INDEX IF NOT EXISTS idx_leave_records_status ON %I.leave_records(status);
    ', schema_name, schema_name, schema_name, schema_name, schema_name, schema_name);

    -- Insert default Estonian absence types
    EXECUTE format('
        INSERT INTO %I.absence_types (tenant_id, code, name, name_et, is_paid, affects_salary, requires_document, document_type, default_days_per_year, max_carryover_days, tsd_code, is_system, sort_order)
        VALUES
            (''00000000-0000-0000-0000-000000000000'', ''ANNUAL_LEAVE'', ''Annual Leave'', ''Põhipuhkus'', true, false, false, NULL, 28, 28, NULL, true, 1),
            (''00000000-0000-0000-0000-000000000000'', ''SICK_LEAVE'', ''Sick Leave'', ''Haigusleht'', true, true, true, ''TK66'', 0, 0, ''10'', true, 2),
            (''00000000-0000-0000-0000-000000000000'', ''CHILD_SICK'', ''Child Sick Leave'', ''Lapse hooldusleht'', true, true, true, ''TK66'', 0, 0, ''11'', true, 3),
            (''00000000-0000-0000-0000-000000000000'', ''MATERNITY'', ''Maternity Leave'', ''Rasedus- ja sünnituspuhkus'', true, true, true, ''Medical'', 0, 0, ''12'', true, 4),
            (''00000000-0000-0000-0000-000000000000'', ''PARENTAL'', ''Parental Leave'', ''Vanemapuhkus'', true, true, false, NULL, 0, 0, ''13'', true, 5),
            (''00000000-0000-0000-0000-000000000000'', ''STUDY_LEAVE'', ''Study Leave'', ''Õppepuhkus'', true, false, true, ''School certificate'', 30, 0, NULL, true, 6),
            (''00000000-0000-0000-0000-000000000000'', ''UNPAID'', ''Unpaid Leave'', ''Palgata puhkus'', false, true, false, NULL, 0, 0, NULL, true, 7),
            (''00000000-0000-0000-0000-000000000000'', ''COMP_TIME'', ''Compensatory Time Off'', ''Tasaarvelduspuhkus'', true, false, false, NULL, 0, 0, NULL, true, 8)
        ON CONFLICT (tenant_id, code) DO NOTHING
    ', schema_name);

END;
$$ LANGUAGE plpgsql;

-- Apply to all existing tenant schemas
DO $$
DECLARE
    schema_rec RECORD;
BEGIN
    FOR schema_rec IN
        SELECT schema_name
        FROM information_schema.schemata
        WHERE schema_name LIKE 'tenant_%'
           OR schema_name = 'schema_demo1'
           OR schema_name = 'schema_demo2'
           OR schema_name = 'schema_demo3'
           OR schema_name = 'schema_demo4'
    LOOP
        PERFORM add_leave_management_tables(schema_rec.schema_name);
    END LOOP;
END;
$$;
