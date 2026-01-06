-- Rollback Cost Centers Migration

DROP INDEX IF EXISTS idx_cost_allocations_journal_line;
DROP INDEX IF EXISTS idx_cost_allocations_date;
DROP INDEX IF EXISTS idx_cost_allocations_center;
DROP INDEX IF EXISTS idx_cost_centers_active;
DROP INDEX IF EXISTS idx_cost_centers_parent;
DROP INDEX IF EXISTS idx_cost_centers_tenant;

DROP TABLE IF EXISTS cost_allocations;
DROP TABLE IF EXISTS cost_centers;
