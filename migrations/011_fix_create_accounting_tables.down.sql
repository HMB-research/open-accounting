-- =============================================================================
-- Migration 011 Down: Remove create_accounting_tables function
-- =============================================================================

DROP FUNCTION IF EXISTS create_accounting_tables(TEXT);
