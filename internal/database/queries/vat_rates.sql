-- name: GetVATRate :one
SELECT * FROM vat_rates
WHERE id = $1;

-- name: GetCurrentVATRate :one
SELECT * FROM vat_rates
WHERE country_code = $1
  AND rate_type = $2
  AND valid_from <= $3
  AND (valid_to IS NULL OR valid_to >= $3)
LIMIT 1;

-- name: ListVATRates :many
SELECT * FROM vat_rates
WHERE country_code = $1
ORDER BY rate_type, valid_from DESC;

-- name: ListCurrentVATRates :many
SELECT * FROM vat_rates
WHERE country_code = $1
  AND valid_from <= CURRENT_DATE
  AND (valid_to IS NULL OR valid_to >= CURRENT_DATE)
ORDER BY rate_type;

-- name: CreateVATRate :one
INSERT INTO vat_rates (country_code, rate_type, rate, name, valid_from, valid_to)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: UpdateVATRate :one
UPDATE vat_rates
SET rate = $2, name = $3, valid_to = $4
WHERE id = $1
RETURNING *;
