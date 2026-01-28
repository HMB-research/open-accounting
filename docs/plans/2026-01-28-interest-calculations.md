# Late Payment Interest Calculations

**Goal:** Add automatic interest/penalty calculations for overdue invoices.

## Database Changes

### Migration 022: Add interest settings and tracking

```sql
-- Add to tenant_settings or create new table
ALTER TABLE {schema}.tenant_settings ADD COLUMN IF NOT EXISTS
  late_payment_interest_rate DECIMAL(5,4) DEFAULT 0.0005; -- 0.05% per day = ~18% annually

-- Track calculated interest per invoice
CREATE TABLE {schema}.invoice_interest (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  invoice_id UUID NOT NULL REFERENCES {schema}.invoices(id),
  calculated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  days_overdue INTEGER NOT NULL,
  principal_amount DECIMAL(15,2) NOT NULL,
  interest_rate DECIMAL(5,4) NOT NULL,
  interest_amount DECIMAL(15,2) NOT NULL,
  total_with_interest DECIMAL(15,2) NOT NULL
);
```

## Backend Changes

### Task 1: Add interest rate to tenant settings
- File: `internal/tenant/types.go` - add `LatePaymentInterestRate` field
- File: `internal/tenant/repository.go` - fetch/update rate

### Task 2: Create interest calculation service
- File: `internal/invoicing/interest_service.go`
- Calculate: `interest = outstanding × rate × days_overdue`
- Methods:
  - `CalculateInterest(invoice, asOfDate) → InterestResult`
  - `GetTotalWithInterest(invoiceID) → amount`

### Task 3: Extend invoice queries
- Add `interest_amount` and `total_with_interest` to invoice responses
- Calculate on-the-fly or cache in `invoice_interest` table

### Task 4: Update reminder emails
- Include interest amount in template data
- Show: "Outstanding: €X + €Y interest = €Z total"

### Task 5: Add API endpoint
- `GET /invoices/{id}/interest` - get current interest calculation
- `GET /tenants/{id}/settings/interest` - get/set interest rate

### Task 6: Frontend
- Settings page: interest rate configuration
- Invoice detail: show interest breakdown
- Reminder preview: show calculated amounts

## Email Template Update

```
Outstanding amount: {{outstanding_amount}} {{currency}}
Days overdue: {{days_overdue}}
Interest ({{interest_rate}}% daily): {{interest_amount}} {{currency}}
─────────────────────────
Total due: {{total_with_interest}} {{currency}}
```

## Estonian Law Reference
- Standard commercial interest: ~0.02-0.05% per day
- Must be specified in invoice terms
- Cannot exceed usury limits
