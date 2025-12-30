-- Add new Estonian VAT rates for 2024-2025
-- Note: The vat_rates table uses 'name' column, not 'description'

-- Standard rate increased to 24% (July 1, 2025)
INSERT INTO vat_rates (id, country_code, rate_type, rate, name, valid_from, valid_to)
VALUES
    (gen_random_uuid(), 'EE', 'STANDARD', 24.00, 'Standard rate 24%', '2025-07-01', NULL),
    (gen_random_uuid(), 'EE', 'ACCOMMODATION', 13.00, 'Accommodation services', '2025-01-01', NULL),
    (gen_random_uuid(), 'EE', 'PRESS', 9.00, 'Press publications', '2025-01-01', NULL)
ON CONFLICT DO NOTHING;

-- Update 22% rate to have end date when 24% takes effect
UPDATE vat_rates
SET valid_to = '2025-06-30'
WHERE country_code = 'EE' AND rate = 22.00 AND rate_type = 'STANDARD' AND valid_to IS NULL;
