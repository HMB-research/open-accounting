-- Rollback migration 002: Remove 2025 VAT rates and restore 22% as current
-- BACKWARD COMPATIBILITY: This restores the state before migration 002

-- Remove new rate types added in migration 002
DELETE FROM vat_rates
WHERE country_code = 'EE'
  AND rate_type IN ('ACCOMMODATION', 'PRESS');

-- Remove 24% standard rate
DELETE FROM vat_rates
WHERE country_code = 'EE'
  AND rate = 24.00
  AND rate_type = 'STANDARD'
  AND valid_from = '2025-07-01';

-- Restore 22% rate as current (remove end date)
UPDATE vat_rates
SET valid_to = NULL
WHERE country_code = 'EE'
  AND rate = 22.00
  AND rate_type = 'STANDARD'
  AND valid_from = '2024-01-01';
