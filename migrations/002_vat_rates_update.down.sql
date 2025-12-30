-- Remove new Estonian VAT rates added in migration 002
DELETE FROM vat_rates WHERE country_code = 'EE' AND rate = 24.00 AND rate_type = 'STANDARD';
DELETE FROM vat_rates WHERE country_code = 'EE' AND rate_type IN ('ACCOMMODATION', 'PRESS');

-- Restore 22% rate to have no end date
UPDATE vat_rates
SET valid_to = NULL
WHERE country_code = 'EE' AND rate = 22.00 AND rate_type = 'STANDARD';
