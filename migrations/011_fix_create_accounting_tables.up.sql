-- =============================================================================
-- Migration 011: Fix create_accounting_tables function
-- This function was referenced in migration 007 but never created.
-- It extracts the core accounting table creation from the original
-- create_tenant_schema function.
-- =============================================================================

-- Create the missing create_accounting_tables function
CREATE OR REPLACE FUNCTION create_accounting_tables(schema_name TEXT) RETURNS VOID AS $$
BEGIN
    -- Set search path for this function
    EXECUTE format('SET search_path TO %I', schema_name);

    -- =========================================================================
    -- CHART OF ACCOUNTS
    -- =========================================================================

    EXECUTE format('
        CREATE TABLE IF NOT EXISTS %I.accounts (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            tenant_id UUID NOT NULL,
            code VARCHAR(20) NOT NULL,
            name VARCHAR(255) NOT NULL,
            account_type VARCHAR(20) NOT NULL CHECK (account_type IN (''ASSET'', ''LIABILITY'', ''EQUITY'', ''REVENUE'', ''EXPENSE'')),
            parent_id UUID REFERENCES %I.accounts(id),
            is_active BOOLEAN NOT NULL DEFAULT true,
            is_system BOOLEAN NOT NULL DEFAULT false,
            description TEXT,
            created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
            UNIQUE (tenant_id, code)
        )
    ', schema_name, schema_name);

    EXECUTE format('CREATE INDEX IF NOT EXISTS idx_%s_accounts_tenant ON %I.accounts(tenant_id)',
        replace(schema_name, '-', '_'), schema_name);
    EXECUTE format('CREATE INDEX IF NOT EXISTS idx_%s_accounts_type ON %I.accounts(account_type)',
        replace(schema_name, '-', '_'), schema_name);
    EXECUTE format('CREATE INDEX IF NOT EXISTS idx_%s_accounts_parent ON %I.accounts(parent_id)',
        replace(schema_name, '-', '_'), schema_name);

    -- =========================================================================
    -- FISCAL YEARS
    -- =========================================================================

    EXECUTE format('
        CREATE TABLE IF NOT EXISTS %I.fiscal_years (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            tenant_id UUID NOT NULL,
            name VARCHAR(100) NOT NULL,
            start_date DATE NOT NULL,
            end_date DATE NOT NULL,
            is_closed BOOLEAN NOT NULL DEFAULT false,
            closed_at TIMESTAMPTZ,
            closed_by UUID,
            created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
            UNIQUE (tenant_id, start_date),
            CHECK (end_date > start_date)
        )
    ', schema_name);

    -- =========================================================================
    -- JOURNAL ENTRIES
    -- =========================================================================

    EXECUTE format('
        CREATE TABLE IF NOT EXISTS %I.journal_entries (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            tenant_id UUID NOT NULL,
            entry_number VARCHAR(20) NOT NULL,
            entry_date DATE NOT NULL,
            description TEXT NOT NULL,
            reference VARCHAR(100),
            source_type VARCHAR(50),
            source_id UUID,
            status VARCHAR(20) NOT NULL DEFAULT ''DRAFT'' CHECK (status IN (''DRAFT'', ''POSTED'', ''VOIDED'')),
            posted_at TIMESTAMPTZ,
            posted_by UUID,
            voided_at TIMESTAMPTZ,
            voided_by UUID,
            void_reason TEXT,
            created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
            created_by UUID NOT NULL,
            UNIQUE (tenant_id, entry_number)
        )
    ', schema_name);

    EXECUTE format('CREATE INDEX IF NOT EXISTS idx_%s_je_tenant ON %I.journal_entries(tenant_id)',
        replace(schema_name, '-', '_'), schema_name);
    EXECUTE format('CREATE INDEX IF NOT EXISTS idx_%s_je_date ON %I.journal_entries(entry_date)',
        replace(schema_name, '-', '_'), schema_name);
    EXECUTE format('CREATE INDEX IF NOT EXISTS idx_%s_je_status ON %I.journal_entries(status)',
        replace(schema_name, '-', '_'), schema_name);
    EXECUTE format('CREATE INDEX IF NOT EXISTS idx_%s_je_source ON %I.journal_entries(source_type, source_id)',
        replace(schema_name, '-', '_'), schema_name);

    -- =========================================================================
    -- JOURNAL ENTRY LINES
    -- =========================================================================

    EXECUTE format('
        CREATE TABLE IF NOT EXISTS %I.journal_entry_lines (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            tenant_id UUID NOT NULL,
            journal_entry_id UUID NOT NULL REFERENCES %I.journal_entries(id) ON DELETE CASCADE,
            account_id UUID NOT NULL REFERENCES %I.accounts(id),
            description TEXT,
            debit_amount NUMERIC(28,8) NOT NULL DEFAULT 0,
            credit_amount NUMERIC(28,8) NOT NULL DEFAULT 0,
            currency CHAR(3) NOT NULL DEFAULT ''EUR'',
            exchange_rate NUMERIC(18,10) NOT NULL DEFAULT 1,
            base_debit NUMERIC(28,8) NOT NULL DEFAULT 0,
            base_credit NUMERIC(28,8) NOT NULL DEFAULT 0,
            CHECK (debit_amount >= 0 AND credit_amount >= 0),
            CHECK (NOT (debit_amount > 0 AND credit_amount > 0))
        )
    ', schema_name, schema_name, schema_name);

    EXECUTE format('CREATE INDEX IF NOT EXISTS idx_%s_jel_entry ON %I.journal_entry_lines(journal_entry_id)',
        replace(schema_name, '-', '_'), schema_name);
    EXECUTE format('CREATE INDEX IF NOT EXISTS idx_%s_jel_account ON %I.journal_entry_lines(account_id)',
        replace(schema_name, '-', '_'), schema_name);

    -- =========================================================================
    -- CONTACTS (Customers/Suppliers)
    -- =========================================================================

    EXECUTE format('
        CREATE TABLE IF NOT EXISTS %I.contacts (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            tenant_id UUID NOT NULL,
            code VARCHAR(20),
            name VARCHAR(255) NOT NULL,
            contact_type VARCHAR(20) NOT NULL CHECK (contact_type IN (''CUSTOMER'', ''SUPPLIER'', ''BOTH'')),
            reg_code VARCHAR(50),
            vat_number VARCHAR(50),
            email VARCHAR(255),
            phone VARCHAR(50),
            address_line1 VARCHAR(255),
            address_line2 VARCHAR(255),
            city VARCHAR(100),
            postal_code VARCHAR(20),
            country_code CHAR(2) NOT NULL DEFAULT ''EE'',
            payment_terms_days INTEGER NOT NULL DEFAULT 14,
            credit_limit NUMERIC(28,8),
            default_account_id UUID REFERENCES %I.accounts(id),
            is_active BOOLEAN NOT NULL DEFAULT true,
            notes TEXT,
            created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
            updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
        )
    ', schema_name, schema_name);

    EXECUTE format('CREATE INDEX IF NOT EXISTS idx_%s_contacts_tenant ON %I.contacts(tenant_id)',
        replace(schema_name, '-', '_'), schema_name);
    EXECUTE format('CREATE INDEX IF NOT EXISTS idx_%s_contacts_type ON %I.contacts(contact_type)',
        replace(schema_name, '-', '_'), schema_name);
    EXECUTE format('CREATE INDEX IF NOT EXISTS idx_%s_contacts_reg ON %I.contacts(reg_code)',
        replace(schema_name, '-', '_'), schema_name);

    -- =========================================================================
    -- INVOICES
    -- =========================================================================

    EXECUTE format('
        CREATE TABLE IF NOT EXISTS %I.invoices (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            tenant_id UUID NOT NULL,
            invoice_number VARCHAR(50) NOT NULL,
            invoice_type VARCHAR(20) NOT NULL CHECK (invoice_type IN (''SALES'', ''PURCHASE'', ''CREDIT_NOTE'')),
            contact_id UUID NOT NULL REFERENCES %I.contacts(id),
            issue_date DATE NOT NULL,
            due_date DATE NOT NULL,
            currency CHAR(3) NOT NULL DEFAULT ''EUR'',
            exchange_rate NUMERIC(18,10) NOT NULL DEFAULT 1,
            subtotal NUMERIC(28,8) NOT NULL DEFAULT 0,
            vat_amount NUMERIC(28,8) NOT NULL DEFAULT 0,
            total NUMERIC(28,8) NOT NULL DEFAULT 0,
            base_subtotal NUMERIC(28,8) NOT NULL DEFAULT 0,
            base_vat_amount NUMERIC(28,8) NOT NULL DEFAULT 0,
            base_total NUMERIC(28,8) NOT NULL DEFAULT 0,
            amount_paid NUMERIC(28,8) NOT NULL DEFAULT 0,
            status VARCHAR(20) NOT NULL DEFAULT ''DRAFT'' CHECK (status IN (''DRAFT'', ''SENT'', ''PARTIALLY_PAID'', ''PAID'', ''OVERDUE'', ''VOIDED'')),
            reference VARCHAR(100),
            notes TEXT,
            journal_entry_id UUID REFERENCES %I.journal_entries(id),
            einvoice_sent_at TIMESTAMPTZ,
            einvoice_id VARCHAR(100),
            created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
            created_by UUID NOT NULL,
            updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
            UNIQUE (tenant_id, invoice_number, invoice_type)
        )
    ', schema_name, schema_name, schema_name);

    EXECUTE format('CREATE INDEX IF NOT EXISTS idx_%s_inv_tenant ON %I.invoices(tenant_id)',
        replace(schema_name, '-', '_'), schema_name);
    EXECUTE format('CREATE INDEX IF NOT EXISTS idx_%s_inv_contact ON %I.invoices(contact_id)',
        replace(schema_name, '-', '_'), schema_name);
    EXECUTE format('CREATE INDEX IF NOT EXISTS idx_%s_inv_status ON %I.invoices(status)',
        replace(schema_name, '-', '_'), schema_name);
    EXECUTE format('CREATE INDEX IF NOT EXISTS idx_%s_inv_date ON %I.invoices(issue_date)',
        replace(schema_name, '-', '_'), schema_name);
    EXECUTE format('CREATE INDEX IF NOT EXISTS idx_%s_inv_due ON %I.invoices(due_date)',
        replace(schema_name, '-', '_'), schema_name);

    -- =========================================================================
    -- INVOICE LINES
    -- =========================================================================

    EXECUTE format('
        CREATE TABLE IF NOT EXISTS %I.invoice_lines (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            tenant_id UUID NOT NULL,
            invoice_id UUID NOT NULL REFERENCES %I.invoices(id) ON DELETE CASCADE,
            line_number INTEGER NOT NULL,
            description TEXT NOT NULL,
            quantity NUMERIC(18,6) NOT NULL DEFAULT 1,
            unit VARCHAR(20),
            unit_price NUMERIC(28,8) NOT NULL,
            discount_percent NUMERIC(5,2) NOT NULL DEFAULT 0,
            vat_rate NUMERIC(5,2) NOT NULL,
            line_subtotal NUMERIC(28,8) NOT NULL,
            line_vat NUMERIC(28,8) NOT NULL,
            line_total NUMERIC(28,8) NOT NULL,
            account_id UUID REFERENCES %I.accounts(id),
            product_id UUID,
            UNIQUE (invoice_id, line_number)
        )
    ', schema_name, schema_name, schema_name);

    EXECUTE format('CREATE INDEX IF NOT EXISTS idx_%s_invl_invoice ON %I.invoice_lines(invoice_id)',
        replace(schema_name, '-', '_'), schema_name);

    -- =========================================================================
    -- PAYMENTS
    -- =========================================================================

    EXECUTE format('
        CREATE TABLE IF NOT EXISTS %I.payments (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            tenant_id UUID NOT NULL,
            payment_number VARCHAR(50) NOT NULL,
            payment_type VARCHAR(20) NOT NULL CHECK (payment_type IN (''RECEIVED'', ''MADE'')),
            contact_id UUID REFERENCES %I.contacts(id),
            payment_date DATE NOT NULL,
            amount NUMERIC(28,8) NOT NULL,
            currency CHAR(3) NOT NULL DEFAULT ''EUR'',
            exchange_rate NUMERIC(18,10) NOT NULL DEFAULT 1,
            base_amount NUMERIC(28,8) NOT NULL,
            payment_method VARCHAR(50),
            bank_account VARCHAR(50),
            reference VARCHAR(100),
            notes TEXT,
            journal_entry_id UUID REFERENCES %I.journal_entries(id),
            created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
            created_by UUID NOT NULL,
            UNIQUE (tenant_id, payment_number)
        )
    ', schema_name, schema_name, schema_name);

    EXECUTE format('CREATE INDEX IF NOT EXISTS idx_%s_pay_tenant ON %I.payments(tenant_id)',
        replace(schema_name, '-', '_'), schema_name);
    EXECUTE format('CREATE INDEX IF NOT EXISTS idx_%s_pay_contact ON %I.payments(contact_id)',
        replace(schema_name, '-', '_'), schema_name);
    EXECUTE format('CREATE INDEX IF NOT EXISTS idx_%s_pay_date ON %I.payments(payment_date)',
        replace(schema_name, '-', '_'), schema_name);

    -- =========================================================================
    -- PAYMENT ALLOCATIONS
    -- =========================================================================

    EXECUTE format('
        CREATE TABLE IF NOT EXISTS %I.payment_allocations (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            tenant_id UUID NOT NULL,
            payment_id UUID NOT NULL REFERENCES %I.payments(id) ON DELETE CASCADE,
            invoice_id UUID NOT NULL REFERENCES %I.invoices(id),
            amount NUMERIC(28,8) NOT NULL,
            created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
        )
    ', schema_name, schema_name, schema_name);

    EXECUTE format('CREATE INDEX IF NOT EXISTS idx_%s_pa_payment ON %I.payment_allocations(payment_id)',
        replace(schema_name, '-', '_'), schema_name);
    EXECUTE format('CREATE INDEX IF NOT EXISTS idx_%s_pa_invoice ON %I.payment_allocations(invoice_id)',
        replace(schema_name, '-', '_'), schema_name);

    -- =========================================================================
    -- PRODUCTS
    -- =========================================================================

    EXECUTE format('
        CREATE TABLE IF NOT EXISTS %I.products (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            tenant_id UUID NOT NULL,
            code VARCHAR(50) NOT NULL,
            name VARCHAR(255) NOT NULL,
            description TEXT,
            product_type VARCHAR(20) NOT NULL DEFAULT ''GOODS'' CHECK (product_type IN (''GOODS'', ''SERVICE'')),
            unit VARCHAR(20),
            purchase_price NUMERIC(28,8),
            sale_price NUMERIC(28,8),
            vat_rate NUMERIC(5,2) NOT NULL DEFAULT 22,
            purchase_account_id UUID REFERENCES %I.accounts(id),
            sale_account_id UUID REFERENCES %I.accounts(id),
            inventory_account_id UUID REFERENCES %I.accounts(id),
            track_inventory BOOLEAN NOT NULL DEFAULT false,
            is_active BOOLEAN NOT NULL DEFAULT true,
            created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
            updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
            UNIQUE (tenant_id, code)
        )
    ', schema_name, schema_name, schema_name, schema_name);

    EXECUTE format('CREATE INDEX IF NOT EXISTS idx_%s_prod_tenant ON %I.products(tenant_id)',
        replace(schema_name, '-', '_'), schema_name);

    -- =========================================================================
    -- INVENTORY LOTS (FIFO tracking)
    -- =========================================================================

    EXECUTE format('
        CREATE TABLE IF NOT EXISTS %I.inventory_lots (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            tenant_id UUID NOT NULL,
            product_id UUID NOT NULL REFERENCES %I.products(id),
            lot_number VARCHAR(50),
            quantity NUMERIC(18,6) NOT NULL,
            remaining_quantity NUMERIC(18,6) NOT NULL,
            unit_cost NUMERIC(28,8) NOT NULL,
            received_date DATE NOT NULL,
            expiry_date DATE,
            source_type VARCHAR(50),
            source_id UUID,
            created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
        )
    ', schema_name, schema_name);

    EXECUTE format('CREATE INDEX IF NOT EXISTS idx_%s_lot_product ON %I.inventory_lots(product_id)',
        replace(schema_name, '-', '_'), schema_name);
    EXECUTE format('CREATE INDEX IF NOT EXISTS idx_%s_lot_remaining ON %I.inventory_lots(remaining_quantity) WHERE remaining_quantity > 0',
        replace(schema_name, '-', '_'), schema_name);

    -- =========================================================================
    -- INVENTORY MOVEMENTS
    -- =========================================================================

    EXECUTE format('
        CREATE TABLE IF NOT EXISTS %I.inventory_movements (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            tenant_id UUID NOT NULL,
            product_id UUID NOT NULL REFERENCES %I.products(id),
            lot_id UUID REFERENCES %I.inventory_lots(id),
            movement_type VARCHAR(20) NOT NULL CHECK (movement_type IN (''IN'', ''OUT'', ''ADJUSTMENT'')),
            quantity NUMERIC(18,6) NOT NULL,
            unit_cost NUMERIC(28,8) NOT NULL,
            total_cost NUMERIC(28,8) NOT NULL,
            source_type VARCHAR(50),
            source_id UUID,
            notes TEXT,
            created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
            created_by UUID NOT NULL
        )
    ', schema_name, schema_name, schema_name);

    EXECUTE format('CREATE INDEX IF NOT EXISTS idx_%s_inv_mov_product ON %I.inventory_movements(product_id)',
        replace(schema_name, '-', '_'), schema_name);
    EXECUTE format('CREATE INDEX IF NOT EXISTS idx_%s_inv_mov_date ON %I.inventory_movements(created_at)',
        replace(schema_name, '-', '_'), schema_name);

    -- =========================================================================
    -- NOTE: Employee, contract, payroll tables are created by add_payroll_tables()
    -- function (migration 007). Do NOT duplicate them here to avoid schema conflicts.
    -- =========================================================================

    -- =========================================================================
    -- E-INVOICE LOG
    -- =========================================================================

    EXECUTE format('
        CREATE TABLE IF NOT EXISTS %I.einvoice_log (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            tenant_id UUID NOT NULL,
            invoice_id UUID NOT NULL REFERENCES %I.invoices(id),
            direction VARCHAR(10) NOT NULL CHECK (direction IN (''OUTBOUND'', ''INBOUND'')),
            format VARCHAR(20) NOT NULL,
            status VARCHAR(20) NOT NULL,
            external_id VARCHAR(100),
            xml_content TEXT,
            error_message TEXT,
            sent_at TIMESTAMPTZ,
            received_at TIMESTAMPTZ,
            created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
        )
    ', schema_name, schema_name);

    EXECUTE format('CREATE INDEX IF NOT EXISTS idx_%s_einv_invoice ON %I.einvoice_log(invoice_id)',
        replace(schema_name, '-', '_'), schema_name);

    -- =========================================================================
    -- BANK ACCOUNTS
    -- =========================================================================

    EXECUTE format('
        CREATE TABLE IF NOT EXISTS %I.bank_accounts (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            tenant_id UUID NOT NULL,
            name VARCHAR(100) NOT NULL,
            account_number VARCHAR(50) NOT NULL,
            bank_name VARCHAR(100),
            swift_code VARCHAR(11),
            currency CHAR(3) NOT NULL DEFAULT ''EUR'',
            gl_account_id UUID REFERENCES %I.accounts(id),
            is_default BOOLEAN NOT NULL DEFAULT false,
            is_active BOOLEAN NOT NULL DEFAULT true,
            created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
        )
    ', schema_name, schema_name);

    EXECUTE format('CREATE INDEX IF NOT EXISTS idx_%s_bank_tenant ON %I.bank_accounts(tenant_id)',
        replace(schema_name, '-', '_'), schema_name);

    -- =========================================================================
    -- BANK TRANSACTIONS
    -- =========================================================================

    EXECUTE format('
        CREATE TABLE IF NOT EXISTS %I.bank_transactions (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            tenant_id UUID NOT NULL,
            bank_account_id UUID NOT NULL REFERENCES %I.bank_accounts(id),
            transaction_date DATE NOT NULL,
            value_date DATE,
            amount NUMERIC(28,8) NOT NULL,
            currency CHAR(3) NOT NULL DEFAULT ''EUR'',
            description TEXT,
            reference VARCHAR(100),
            counterparty_name VARCHAR(255),
            counterparty_account VARCHAR(50),
            status VARCHAR(20) NOT NULL DEFAULT ''UNMATCHED'' CHECK (status IN (''UNMATCHED'', ''MATCHED'', ''RECONCILED'')),
            matched_payment_id UUID REFERENCES %I.payments(id),
            journal_entry_id UUID REFERENCES %I.journal_entries(id),
            imported_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
            external_id VARCHAR(100)
        )
    ', schema_name, schema_name, schema_name, schema_name);

    EXECUTE format('CREATE INDEX IF NOT EXISTS idx_%s_bt_bank ON %I.bank_transactions(bank_account_id)',
        replace(schema_name, '-', '_'), schema_name);
    EXECUTE format('CREATE INDEX IF NOT EXISTS idx_%s_bt_date ON %I.bank_transactions(transaction_date)',
        replace(schema_name, '-', '_'), schema_name);
    EXECUTE format('CREATE INDEX IF NOT EXISTS idx_%s_bt_status ON %I.bank_transactions(status)',
        replace(schema_name, '-', '_'), schema_name);

    -- =========================================================================
    -- AUDIT LOG
    -- =========================================================================

    EXECUTE format('
        CREATE TABLE IF NOT EXISTS %I.audit_log (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            tenant_id UUID NOT NULL,
            user_id UUID NOT NULL,
            action VARCHAR(50) NOT NULL,
            entity_type VARCHAR(50) NOT NULL,
            entity_id UUID NOT NULL,
            old_values JSONB,
            new_values JSONB,
            ip_address INET,
            user_agent TEXT,
            created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
        )
    ', schema_name);

    EXECUTE format('CREATE INDEX IF NOT EXISTS idx_%s_audit_entity ON %I.audit_log(entity_type, entity_id)',
        replace(schema_name, '-', '_'), schema_name);
    EXECUTE format('CREATE INDEX IF NOT EXISTS idx_%s_audit_user ON %I.audit_log(user_id)',
        replace(schema_name, '-', '_'), schema_name);
    EXECUTE format('CREATE INDEX IF NOT EXISTS idx_%s_audit_date ON %I.audit_log(created_at)',
        replace(schema_name, '-', '_'), schema_name);

    -- Reset search path
    RESET search_path;
END;
$$ LANGUAGE plpgsql;
