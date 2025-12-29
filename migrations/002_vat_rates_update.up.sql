-- Add new Estonian VAT rates for 2024-2025

-- Standard rate increased to 22% (Jan 1, 2024)
INSERT INTO vat_rates (id, country_code, rate_type, rate, valid_from, valid_to, description)
VALUES
    (gen_random_uuid(), 'EE', 'STANDARD', 22.00, '2024-01-01', '2025-06-30', 'Estonian standard VAT 22%'),
    (gen_random_uuid(), 'EE', 'STANDARD', 24.00, '2025-07-01', NULL, 'Estonian standard VAT 24%'),
    (gen_random_uuid(), 'EE', 'ACCOMMODATION', 13.00, '2025-01-01', NULL, 'Accommodation services VAT 13%'),
    (gen_random_uuid(), 'EE', 'PRESS', 9.00, '2025-01-01', NULL, 'Press publications VAT 9%')
ON CONFLICT DO NOTHING;

-- Update old 20% rate to have end date
UPDATE vat_rates
SET valid_to = '2023-12-31'
WHERE country_code = 'EE' AND rate = 20.00 AND rate_type = 'STANDARD' AND valid_to IS NULL;
