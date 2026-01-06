-- Cost Centers Migration
-- Enables expense tracking and reporting by department, project, or location

-- Cost centers table
CREATE TABLE IF NOT EXISTS cost_centers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    code VARCHAR(20) NOT NULL,
    name VARCHAR(200) NOT NULL,
    description TEXT,
    parent_id UUID REFERENCES cost_centers(id) ON DELETE SET NULL,
    is_active BOOLEAN NOT NULL DEFAULT true,
    budget_amount DECIMAL(15,2),
    budget_period VARCHAR(20) DEFAULT 'ANNUAL', -- MONTHLY, QUARTERLY, ANNUAL
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    UNIQUE(tenant_id, code)
);

-- Cost allocations for tracking expenses by cost center
CREATE TABLE IF NOT EXISTS cost_allocations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    cost_center_id UUID NOT NULL REFERENCES cost_centers(id) ON DELETE CASCADE,
    journal_entry_line_id UUID NOT NULL,
    amount DECIMAL(15,2) NOT NULL,
    allocation_percentage DECIMAL(5,2),
    allocation_date DATE NOT NULL,
    notes TEXT,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- Indexes for performance
CREATE INDEX IF NOT EXISTS idx_cost_centers_tenant ON cost_centers(tenant_id);
CREATE INDEX IF NOT EXISTS idx_cost_centers_parent ON cost_centers(parent_id);
CREATE INDEX IF NOT EXISTS idx_cost_centers_active ON cost_centers(tenant_id, is_active);
CREATE INDEX IF NOT EXISTS idx_cost_allocations_center ON cost_allocations(cost_center_id);
CREATE INDEX IF NOT EXISTS idx_cost_allocations_date ON cost_allocations(allocation_date);
CREATE INDEX IF NOT EXISTS idx_cost_allocations_journal_line ON cost_allocations(journal_entry_line_id);
