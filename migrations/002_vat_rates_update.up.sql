-- Add new Estonian VAT rates for 2024-2025
-- BACKWARD COMPATIBILITY: Historical rates are preserved for bookkeeping
-- Old rates get valid_to dates set, but records remain for historical lookups
--
-- Historical VAT rates preserved:
--   - 20% STANDARD (2009-07-01 to 2023-12-31) - from initial schema
--   - 22% STANDARD (2024-01-01 to 2025-06-30) - updated below
--   - 9% REDUCED  (2009-07-01 to 2023-12-31) - from initial schema
--   - 13% REDUCED (2024-01-01 to present)    - from initial schema

-- Add new rate types for 2025 (these are new categories, not replacing old ones)
INSERT INTO vat_rates (id, country_code, rate_type, rate, name, valid_from, valid_to)
VALUES
    -- New standard rate 24% effective July 1, 2025
    (gen_random_uuid(), 'EE', 'STANDARD', 24.00, 'Standard rate 24%', '2025-07-01', NULL),
    -- Accommodation services at 13% (new category)
    (gen_random_uuid(), 'EE', 'ACCOMMODATION', 13.00, 'Accommodation services 13%', '2025-01-01', NULL),
    -- Press/periodicals at 9% (new category, distinct from old REDUCED)
    (gen_random_uuid(), 'EE', 'PRESS', 9.00, 'Press and periodicals 9%', '2025-01-01', NULL)
ON CONFLICT DO NOTHING;

-- Set end date for current 22% standard rate (preserves historical record)
-- This does NOT delete the rate - it remains available for historical lookups
UPDATE vat_rates
SET valid_to = '2025-06-30'
WHERE country_code = 'EE'
  AND rate = 22.00
  AND rate_type = 'STANDARD'
  AND valid_to IS NULL;
