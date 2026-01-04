# Demo Data Reset E2E Testing Design

## Overview

Comprehensive E2E testing strategy to verify demo data availability and reset functionality. Tests validate that seeded data exists, has correct counts/values, and that the reset mechanism properly restores data after modifications.

## Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Verification level | Existence + Integrity + Reset | Comprehensive coverage |
| Reset trigger timing | On-demand in specific tests | Avoids slowing entire suite |
| Test file location | Dedicated `reset.spec.ts` | Centralized, maintainable |
| Modification approach | Hybrid (counts + API mod + reset) | Thorough but not flaky |
| Parallelization | Worker-scoped reset | Maintains parallel benefits |
| Reset granularity | Per-user API parameter | Clean isolation per worker |
| Data verification method | API-based | Fast, reliable, no UI flakiness |
| Status response format | Counts + key identifiers | Catches quantity and corruption issues |

---

## Part 1: API Changes

### 1.1 Modify `/api/demo/reset` Endpoint

Add optional `user` query parameter to reset individual demo users:

| Request | Behavior |
|---------|----------|
| `POST /api/demo/reset` | Resets all 3 users (existing behavior) |
| `POST /api/demo/reset?user=1` | Resets only demo1 |
| `POST /api/demo/reset?user=2` | Resets only demo2 |
| `POST /api/demo/reset?user=3` | Resets only demo3 |

**Implementation in `cmd/api/handlers.go`:**
- Parse `user` query parameter
- Filter `demoUsers` slice to only include specified user
- If parameter invalid or out of range, return 400 error
- If parameter absent, reset all users (backward compatible)

### 1.2 New `/api/demo/status` Endpoint

Returns entity counts and key identifiers for verification.

**Request:**
```
GET /api/demo/status?user=1
Header: X-Demo-Secret: <secret>
```

**Response:**
```json
{
  "user": 1,
  "accounts": {
    "count": 28,
    "keys": ["Cash", "Bank Account EUR", "Accounts Receivable", "Accounts Payable"]
  },
  "contacts": {
    "count": 10,
    "keys": ["TechCorp Solutions", "Global Industries", "Baltic Shipping"]
  },
  "invoices": {
    "count": 5,
    "keys": ["INV1-2024-001", "INV1-2024-002", "INV1-2025-001"]
  },
  "employees": {
    "count": 3,
    "keys": ["John Doe", "Jane Smith", "Bob Wilson"]
  },
  "payments": {
    "count": 3,
    "keys": ["PAY1-2024-001", "PAY1-2024-002"]
  },
  "journalEntries": {
    "count": 2,
    "keys": ["JE1-2024-001", "JE1-2024-002"]
  },
  "bankAccounts": {
    "count": 2,
    "keys": ["Main EUR Account", "USD Account"]
  },
  "recurringInvoices": {
    "count": 1,
    "keys": ["Monthly Hosting Fee"]
  },
  "payrollRuns": {
    "count": 2,
    "keys": ["2024-11", "2024-12"]
  },
  "tsdDeclarations": {
    "count": 2,
    "keys": ["2024-11", "2024-12"]
  }
}
```

**Security:** Protected by same `X-Demo-Secret` header as reset endpoint.

---

## Part 2: Test Implementation

### 2.1 New File: `frontend/e2e/demo/reset.spec.ts`

```typescript
import { test, expect } from '@playwright/test';
import { getDemoCredentials } from './utils';
import {
  triggerDemoReset,
  getDemoStatus,
  createTestInvoice,
  deleteInvoice,
  EXPECTED_DEMO_DATA
} from './api';

test.describe('Demo Data Reset Verification', () => {

  test.describe('Initial State Verification', () => {

    test('has correct account count and key accounts', async ({}, testInfo) => {
      const userNum = (testInfo.parallelIndex % 3) + 1;
      const status = await getDemoStatus(userNum);

      expect(status.accounts.count).toBe(EXPECTED_DEMO_DATA.accounts.count);
      for (const key of EXPECTED_DEMO_DATA.accounts.keys) {
        expect(status.accounts.keys).toContain(key);
      }
    });

    test('has correct contact count and key contacts', async ({}, testInfo) => {
      const userNum = (testInfo.parallelIndex % 3) + 1;
      const status = await getDemoStatus(userNum);

      expect(status.contacts.count).toBe(EXPECTED_DEMO_DATA.contacts.count);
      for (const key of EXPECTED_DEMO_DATA.contacts.keys) {
        expect(status.contacts.keys).toContain(key);
      }
    });

    test('has correct invoice count and key invoices', async ({}, testInfo) => {
      const userNum = (testInfo.parallelIndex % 3) + 1;
      const status = await getDemoStatus(userNum);

      expect(status.invoices.count).toBe(EXPECTED_DEMO_DATA.invoices.count);
      for (const key of EXPECTED_DEMO_DATA.invoices.keys) {
        expect(status.invoices.keys).toContain(key);
      }
    });

    test('has correct employee count and key employees', async ({}, testInfo) => {
      const userNum = (testInfo.parallelIndex % 3) + 1;
      const status = await getDemoStatus(userNum);

      expect(status.employees.count).toBe(EXPECTED_DEMO_DATA.employees.count);
      for (const key of EXPECTED_DEMO_DATA.employees.keys) {
        expect(status.employees.keys).toContain(key);
      }
    });

    test('has correct payment count and key payments', async ({}, testInfo) => {
      const userNum = (testInfo.parallelIndex % 3) + 1;
      const status = await getDemoStatus(userNum);

      expect(status.payments.count).toBe(EXPECTED_DEMO_DATA.payments.count);
    });

    test('has correct journal entry count', async ({}, testInfo) => {
      const userNum = (testInfo.parallelIndex % 3) + 1;
      const status = await getDemoStatus(userNum);

      expect(status.journalEntries.count).toBe(EXPECTED_DEMO_DATA.journalEntries.count);
    });

    test('has correct bank account count', async ({}, testInfo) => {
      const userNum = (testInfo.parallelIndex % 3) + 1;
      const status = await getDemoStatus(userNum);

      expect(status.bankAccounts.count).toBe(EXPECTED_DEMO_DATA.bankAccounts.count);
    });

    test('has correct recurring invoice count', async ({}, testInfo) => {
      const userNum = (testInfo.parallelIndex % 3) + 1;
      const status = await getDemoStatus(userNum);

      expect(status.recurringInvoices.count).toBe(EXPECTED_DEMO_DATA.recurringInvoices.count);
    });

    test('has correct payroll run count', async ({}, testInfo) => {
      const userNum = (testInfo.parallelIndex % 3) + 1;
      const status = await getDemoStatus(userNum);

      expect(status.payrollRuns.count).toBe(EXPECTED_DEMO_DATA.payrollRuns.count);
    });

    test('has correct TSD declaration count', async ({}, testInfo) => {
      const userNum = (testInfo.parallelIndex % 3) + 1;
      const status = await getDemoStatus(userNum);

      expect(status.tsdDeclarations.count).toBe(EXPECTED_DEMO_DATA.tsdDeclarations.count);
    });

  });

  test.describe('Reset Functionality', () => {

    test('reset restores data after API modification', async ({}, testInfo) => {
      const userNum = (testInfo.parallelIndex % 3) + 1;

      // Get initial state
      const initialStatus = await getDemoStatus(userNum);
      const initialInvoiceCount = initialStatus.invoices.count;

      // Create a test invoice via API
      const testInvoiceId = await createTestInvoice(userNum);

      // Verify invoice was created
      const modifiedStatus = await getDemoStatus(userNum);
      expect(modifiedStatus.invoices.count).toBe(initialInvoiceCount + 1);

      // Trigger reset for this user only
      await triggerDemoReset(userNum);

      // Verify data is restored
      const restoredStatus = await getDemoStatus(userNum);
      expect(restoredStatus.invoices.count).toBe(initialInvoiceCount);
      expect(restoredStatus.invoices.keys).not.toContain(testInvoiceId);
    });

    test('reset is idempotent', async ({}, testInfo) => {
      const userNum = (testInfo.parallelIndex % 3) + 1;

      // Reset twice
      await triggerDemoReset(userNum);
      const statusAfterFirst = await getDemoStatus(userNum);

      await triggerDemoReset(userNum);
      const statusAfterSecond = await getDemoStatus(userNum);

      // Should produce identical state
      expect(statusAfterSecond).toEqual(statusAfterFirst);
    });

  });

});
```

### 2.2 New File: `frontend/e2e/demo/api.ts`

```typescript
import { DEMO_API_URL } from './utils';

const DEMO_SECRET = process.env.DEMO_RESET_SECRET || '';

export interface EntityStatus {
  count: number;
  keys: string[];
}

export interface DemoStatus {
  user: number;
  accounts: EntityStatus;
  contacts: EntityStatus;
  invoices: EntityStatus;
  employees: EntityStatus;
  payments: EntityStatus;
  journalEntries: EntityStatus;
  bankAccounts: EntityStatus;
  recurringInvoices: EntityStatus;
  payrollRuns: EntityStatus;
  tsdDeclarations: EntityStatus;
}

/**
 * Expected demo data counts and key identifiers.
 * Update these when seed data changes.
 */
export const EXPECTED_DEMO_DATA = {
  accounts: {
    count: 28,
    keys: ['Cash', 'Bank Account EUR', 'Accounts Receivable', 'Accounts Payable']
  },
  contacts: {
    count: 10,
    keys: ['TechCorp Solutions', 'Global Industries']
  },
  invoices: {
    count: 5,
    keys: ['INV1-2024-001', 'INV1-2024-002']
  },
  employees: {
    count: 3,
    keys: ['John Doe', 'Jane Smith']
  },
  payments: {
    count: 3,
    keys: ['PAY1-2024-001']
  },
  journalEntries: {
    count: 2,
    keys: ['JE1-2024-001']
  },
  bankAccounts: {
    count: 2,
    keys: ['Main EUR Account']
  },
  recurringInvoices: {
    count: 1,
    keys: ['Monthly Hosting Fee']
  },
  payrollRuns: {
    count: 2,
    keys: ['2024-11', '2024-12']
  },
  tsdDeclarations: {
    count: 2,
    keys: ['2024-11', '2024-12']
  }
};

/**
 * Trigger demo reset for a specific user
 */
export async function triggerDemoReset(userNum: number): Promise<void> {
  const response = await fetch(`${DEMO_API_URL}/api/demo/reset?user=${userNum}`, {
    method: 'POST',
    headers: {
      'X-Demo-Secret': DEMO_SECRET,
    },
  });

  if (!response.ok) {
    throw new Error(`Demo reset failed: ${response.status} ${await response.text()}`);
  }
}

/**
 * Get demo status (counts and key identifiers) for a specific user
 */
export async function getDemoStatus(userNum: number): Promise<DemoStatus> {
  const response = await fetch(`${DEMO_API_URL}/api/demo/status?user=${userNum}`, {
    headers: {
      'X-Demo-Secret': DEMO_SECRET,
    },
  });

  if (!response.ok) {
    throw new Error(`Demo status failed: ${response.status} ${await response.text()}`);
  }

  return response.json();
}

/**
 * Create a test invoice via API (for reset testing)
 */
export async function createTestInvoice(userNum: number): Promise<string> {
  // Implementation depends on existing invoice API
  // Returns the created invoice ID/number
  const response = await fetch(`${DEMO_API_URL}/api/invoices`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'X-Demo-Secret': DEMO_SECRET,
      'X-Tenant-ID': `b0000000-0000-0000-000${userNum}-000000000001`,
    },
    body: JSON.stringify({
      invoice_type: 'sales',
      contact_id: `d${userNum}000000-0000-0000-0001-000000000001`, // First contact
      issue_date: new Date().toISOString().split('T')[0],
      due_date: new Date(Date.now() + 30*24*60*60*1000).toISOString().split('T')[0],
      lines: [{
        description: 'Test item for reset verification',
        quantity: 1,
        unit_price: 100,
        vat_rate: 20,
      }],
    }),
  });

  if (!response.ok) {
    throw new Error(`Create invoice failed: ${response.status} ${await response.text()}`);
  }

  const data = await response.json();
  return data.invoice_number;
}
```

---

## Part 3: Implementation Steps

1. **Backend: Add `user` parameter to reset endpoint**
   - Parse query parameter in `DemoReset` handler
   - Filter `demoUsers` slice based on parameter
   - Add validation for parameter value (1-3)

2. **Backend: Add `/api/demo/status` endpoint**
   - Query counts from each tenant table
   - Query key identifiers (names, codes, numbers)
   - Return structured JSON response
   - Protect with `X-Demo-Secret` header

3. **Frontend: Create `frontend/e2e/demo/api.ts`**
   - Implement `triggerDemoReset()` function
   - Implement `getDemoStatus()` function
   - Implement `createTestInvoice()` function
   - Define `EXPECTED_DEMO_DATA` constant

4. **Frontend: Create `frontend/e2e/demo/reset.spec.ts`**
   - Initial state verification tests (10 tests)
   - Reset functionality tests (2 tests)

5. **Update CI workflow**
   - Ensure `DEMO_RESET_SECRET` is available in test environment
   - Add reset tests to demo test suite

6. **Verify expected counts**
   - Review seed SQL to confirm actual counts
   - Update `EXPECTED_DEMO_DATA` with accurate values

---

## Files to Create/Modify

| File | Action |
|------|--------|
| `cmd/api/handlers.go` | Modify `DemoReset`, add `DemoStatus` handler |
| `cmd/api/main.go` | Add route for `/api/demo/status` |
| `frontend/e2e/demo/api.ts` | Create new file |
| `frontend/e2e/demo/reset.spec.ts` | Create new file |
| `.github/workflows/e2e-demo.yml` | Ensure secret is passed (if not already) |
