UPDATE webhook_deliveries
SET response_body = ''
WHERE response_body <> '';
