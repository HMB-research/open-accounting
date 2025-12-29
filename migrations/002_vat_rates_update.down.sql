-- Remove new Estonian VAT rates
DELETE FROM vat_rates WHERE country_code = 'EE' AND rate IN (22.00, 24.00, 13.00) AND rate_type IN ('STANDARD', 'ACCOMMODATION', 'PRESS');

-- Restore old 20% rate to have no end date
UPDATE vat_rates
SET valid_to = NULL
WHERE country_code = 'EE' AND rate = 20.00 AND rate_type = 'STANDARD';
