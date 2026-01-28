# Product Requirement Prompt (PRP): Quotes, Orders & Fixed Assets View Fixes

## Executive Summary

- **Feature Name**: Quotes, Orders & Fixed Assets View Fixes
- **Version**: 1.0
- **Date**: 2026-01-08
- **Author**: Claude
- **Status**: Draft
- **Brief Description**: Fix issues preventing Quotes, Orders, and Fixed Assets views from displaying data in the demo environment, and add comprehensive E2E test coverage.

## Problem Statement

### Current Situation
E2E testing of the Quotes (`/quotes`), Orders (`/orders`), and Fixed Assets (`/assets`) views reveals that all three views:
- Load successfully with proper UI structure
- Display heading, filter controls, and "New" button correctly
- Show **empty states** instead of demo data
- Have **no dedicated E2E test coverage**

### Impact
- **Users**: Demo visitors cannot see these important accounting features working with real data
- **Quality**: No automated regression testing for these views
- **Business**: Incomplete demo experience reduces confidence in the product

### Opportunity
- Improve demo experience by showing realistic quote/order/asset workflows
- Add E2E coverage to prevent regressions
- Identify and fix data pipeline issues

### Constraints
- Must maintain backward compatibility with existing API contracts
- Must work within multi-tenant schema architecture
- Must not break existing demo seeding process

## Goals & Objectives

### Primary Goal
Ensure Quotes, Orders, and Fixed Assets views display seeded demo data and have comprehensive E2E test coverage.

### Secondary Goals
1. Identify root cause of missing data
2. Add navigation links in sidebar for these views
3. Improve demo seed data with realistic business scenarios

### Success Metrics
- [ ] All three views display demo data (not empty states)
- [ ] E2E tests pass for all three views
- [ ] API endpoints return correct data for demo tenant
- [ ] Views accessible from navigation sidebar

### Key Performance Indicators (KPIs)
- 100% E2E test pass rate for views
- Demo data visible within 3 seconds of page load
- Zero console errors when loading views

## User Stories & Use Cases

### Target Users
- **Demo visitors**: Evaluating the product
- **Accountants**: Managing quotes, orders, and fixed assets
- **Developers**: Testing and maintaining the views

### User Stories

#### US1: View Quotes List
**As a** demo user,
**I want to** see a list of sample quotes when I navigate to /quotes,
**So that** I can understand how quote management works.

**Acceptance Criteria:**
- Page displays at least 4 quotes with different statuses (DRAFT, SENT, ACCEPTED, CONVERTED)
- Quotes show customer name, amount, date, and status badge
- Filter by status works correctly
- "New Quote" button is visible

#### US2: View Orders List
**As a** demo user,
**I want to** see a list of sample orders when I navigate to /orders,
**So that** I can understand the order lifecycle.

**Acceptance Criteria:**
- Page displays at least 3 orders with different statuses (PENDING, CONFIRMED, PROCESSING)
- Orders show customer name, amount, expected delivery, status
- Filter by status works correctly
- Order actions (confirm, process, ship, deliver) are visible for appropriate statuses

#### US3: View Fixed Assets List
**As a** demo user,
**I want to** see fixed asset inventory when I navigate to /assets,
**So that** I can understand asset depreciation tracking.

**Acceptance Criteria:**
- Page displays at least 6 assets with different statuses (DRAFT, ACTIVE, DISPOSED)
- Assets show name, category, purchase cost, book value, depreciation status
- Filter by status works correctly
- Asset actions (activate, depreciate, dispose) are visible

### Use Case Scenarios

#### UC1: Demo User Explores Quote to Order Workflow
1. User logs in as demo@example.com
2. Navigates to /quotes
3. Sees quotes in various statuses
4. Notes one quote is "CONVERTED" status
5. Navigates to /orders
6. Sees corresponding order linked from converted quote

#### UC2: Accountant Reviews Asset Depreciation
1. User navigates to /assets
2. Selects an "ACTIVE" asset
3. Clicks "View History" to see depreciation entries
4. Reviews accumulated depreciation and book value

### Edge Cases
- Empty tenant (no quotes/orders/assets) should show appropriate empty states
- Very long lists should paginate properly
- Dates in different timezones should display correctly

## Functional Requirements

### P0 - Critical (Must Have)

| ID | Requirement | Status |
|----|-------------|--------|
| FR1 | API `/tenants/{id}/quotes` returns seeded demo data | Not Working |
| FR2 | API `/tenants/{id}/orders` returns seeded demo data | Not Working |
| FR3 | API `/tenants/{id}/assets` returns seeded demo data | Not Working |
| FR4 | Quotes view displays data table (not empty state) | Not Working |
| FR5 | Orders view displays data table (not empty state) | Not Working |
| FR6 | Assets view displays data table (not empty state) | Not Working |

### P1 - Important (Should Have)

| ID | Requirement | Status |
|----|-------------|--------|
| FR7 | E2E test for Quotes view data display | Missing |
| FR8 | E2E test for Orders view data display | Missing |
| FR9 | E2E test for Assets view data display | Missing |
| FR10 | Navigation sidebar links to these views | Check Required |
| FR11 | E2E test for create quote flow | Missing |
| FR12 | E2E test for create order flow | Missing |
| FR13 | E2E test for create asset flow | Missing |

### P2 - Nice to Have

| ID | Requirement | Status |
|----|-------------|--------|
| FR14 | Quote to Order conversion E2E test | Missing |
| FR15 | Asset depreciation recording E2E test | Missing |
| FR16 | Asset disposal E2E test | Missing |

### Dependencies
- Demo seed SQL must execute correctly (`scripts/demo-seed.sql`)
- Migrations 014 (quotes/orders) and 015 (fixed assets) must be applied
- Tenant schema functions must create required tables

## Non-Functional Requirements

### Performance
- Views should load within 2 seconds
- API responses should return within 500ms
- No N+1 query problems

### Security
- All endpoints require authentication
- Tenant isolation must be enforced
- No cross-tenant data leakage

### Reliability
- E2E tests should have <5% flakiness rate
- API should handle missing data gracefully

### Usability
- Empty states should have helpful messages
- Loading states should be visible
- Error messages should be actionable

## Technical Specifications

### Architecture Overview

```
Frontend (Svelte 5)           Backend (Go)              Database (PostgreSQL)
┌─────────────────┐          ┌─────────────────┐       ┌─────────────────────┐
│ /quotes page    │──HTTP───▶│ GET /quotes     │──SQL─▶│ tenant_xxx.quotes   │
│ /orders page    │          │ GET /orders     │       │ tenant_xxx.orders   │
│ /assets page    │          │ GET /assets     │       │ tenant_xxx.fixed_   │
└─────────────────┘          └─────────────────┘       │ assets              │
                                                        └─────────────────────┘
```

### Technology Stack
- **Frontend**: Svelte 5 with Runes, TypeScript, Paraglide i18n
- **Backend**: Go 1.22+, Gin framework
- **Database**: PostgreSQL 15+ with multi-tenant schemas
- **Testing**: Playwright for E2E

### Data Models

#### Quotes Schema (from migration 014)
```sql
CREATE TABLE quotes (
    id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL,
    quote_number VARCHAR(50) NOT NULL,
    contact_id UUID NOT NULL,
    quote_date DATE NOT NULL,
    valid_until DATE,
    status quote_status NOT NULL DEFAULT 'DRAFT',
    subtotal DECIMAL(15,2) NOT NULL DEFAULT 0,
    vat_amount DECIMAL(15,2) NOT NULL DEFAULT 0,
    total DECIMAL(15,2) NOT NULL DEFAULT 0,
    notes TEXT,
    converted_to_order_id UUID,
    created_by UUID,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);
```

#### Orders Schema (from migration 014)
```sql
CREATE TABLE orders (
    id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL,
    order_number VARCHAR(50) NOT NULL,
    contact_id UUID NOT NULL,
    order_date DATE NOT NULL,
    expected_delivery DATE,
    status order_status NOT NULL DEFAULT 'PENDING',
    subtotal DECIMAL(15,2) NOT NULL DEFAULT 0,
    vat_amount DECIMAL(15,2) NOT NULL DEFAULT 0,
    total DECIMAL(15,2) NOT NULL DEFAULT 0,
    notes TEXT,
    quote_id UUID,
    created_by UUID,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);
```

#### Fixed Assets Schema (from migration 015)
```sql
CREATE TABLE fixed_assets (
    id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL,
    asset_number VARCHAR(50) NOT NULL,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    category_id UUID,
    purchase_date DATE NOT NULL,
    purchase_cost DECIMAL(15,2) NOT NULL,
    residual_value DECIMAL(15,2) DEFAULT 0,
    useful_life_months INTEGER NOT NULL,
    depreciation_method depreciation_method_type NOT NULL,
    depreciation_start_date DATE,
    book_value DECIMAL(15,2) NOT NULL,
    accumulated_depreciation DECIMAL(15,2) DEFAULT 0,
    status asset_status NOT NULL DEFAULT 'DRAFT',
    ...
);
```

### Integration Points
- Contacts API (for customer/supplier lookup)
- Accounting module (for depreciation journal entries)
- Invoice module (quote to invoice conversion)

### Technical Constraints
- Must use schema-qualified queries for multi-tenant isolation
- Must handle tenant context from JWT middleware
- Must support Railway deployment environment

## Implementation Plan

### Phase 1: Root Cause Analysis
**Objective**: Identify why demo data isn't displaying

**Tasks**:
1. [ ] Check if migrations 014 and 015 ran successfully on demo environment
2. [ ] Verify `add_quotes_and_orders_tables()` and `add_fixed_assets_tables()` functions create tables in tenant schema
3. [ ] Test API endpoints locally with proper tenant context
4. [ ] Check handler schema qualification in repository layer
5. [ ] Review demo seed SQL execution order

**Deliverables**:
- Root cause documented
- Fix identified

### Phase 2: Backend Fixes
**Objective**: Fix API endpoints to return correct data

**Tasks**:
1. [ ] Fix quotes repository query (if needed)
2. [ ] Fix orders repository query (if needed)
3. [ ] Fix assets repository query (if needed)
4. [ ] Add integration tests for repository layer
5. [ ] Test endpoints return seeded data

**Deliverables**:
- API returns correct demo data
- Integration tests pass

### Phase 3: E2E Test Implementation
**Objective**: Add comprehensive E2E test coverage

**Tasks**:
1. [ ] Create `frontend/e2e/demo/quotes.spec.ts`
2. [ ] Create `frontend/e2e/demo/orders.spec.ts`
3. [ ] Create `frontend/e2e/demo/fixed-assets.spec.ts`
4. [ ] Add tests for data display
5. [ ] Add tests for filtering
6. [ ] Add tests for CRUD operations
7. [ ] Run full E2E suite to verify

**Deliverables**:
- 3 new E2E test files
- All tests passing

### Phase 4: Navigation & Polish
**Objective**: Ensure views are accessible and polished

**Tasks**:
1. [ ] Add sidebar navigation links (if missing)
2. [ ] Verify mobile responsiveness
3. [ ] Update demo-all-views.spec.ts to include these views
4. [ ] Document any remaining issues

**Deliverables**:
- Views accessible from navigation
- Comprehensive E2E coverage

## Risk Assessment

### Technical Risks

| Risk | Probability | Impact | Mitigation |
|------|-------------|--------|------------|
| Schema tables not created in tenant schema | Medium | High | Verify migration and seed functions |
| Repository using wrong schema qualifier | Medium | High | Add debug logging, review query patterns |
| Demo seed SQL syntax errors | Low | High | Test seed SQL locally first |
| E2E test flakiness | Medium | Medium | Use proper wait conditions |

### Business Risks
- Demo experience degraded until fixed
- Potential customer confusion about missing features

### Mitigation Strategies
1. Test all changes locally before deploying to demo
2. Add debug endpoints to verify data state
3. Use explicit waits in E2E tests
4. Document any known limitations

## Success Criteria & Acceptance Tests

### Acceptance Criteria

| Criteria | Test Method |
|----------|-------------|
| Quotes page shows at least 4 quotes | E2E test assertion |
| Orders page shows at least 3 orders | E2E test assertion |
| Assets page shows at least 6 assets | E2E test assertion |
| All status filters work correctly | E2E test interactions |
| Create flows work without errors | E2E test workflows |
| No console errors on page load | E2E test console monitoring |

### Test Scenarios

**TS1**: Demo Login and View Quotes
```
1. Navigate to /login
2. Login as demo@example.com
3. Navigate to /quotes
4. Verify table has >= 4 rows
5. Verify each row has: quote number, status badge, customer, date, total
```

**TS2**: Filter Orders by Status
```
1. Navigate to /orders (logged in)
2. Select "Confirmed" from status filter
3. Verify only CONFIRMED orders display
4. Select "All Orders"
5. Verify all orders display
```

**TS3**: Asset Depreciation History
```
1. Navigate to /assets (logged in)
2. Find an "Active" asset
3. Click "View History"
4. Verify depreciation modal shows entries
5. Close modal
```

### Quality Gates
- [ ] All P0 requirements passing
- [ ] E2E tests achieving >95% pass rate
- [ ] No high-severity bugs
- [ ] Code review approved

## Documentation & Training

### Documentation Needs
- [ ] Update `docs/demo-e2e-testing.md` with new test files
- [ ] Add E2E test examples for CRUD operations
- [ ] Document any API changes

### Knowledge Transfer
- E2E test patterns documented in test files
- Investigation findings recorded in this PRP

## Post-Launch Considerations

### Monitoring
- Track E2E test pass rates in CI
- Monitor for demo environment errors

### Maintenance
- Keep E2E tests updated with UI changes
- Update demo seed data periodically

### Future Enhancements
- Add quote-to-invoice conversion
- Add order fulfillment workflow
- Add bulk asset depreciation
- Add asset import from CSV

## Files to Investigate/Modify

### Backend (Go)
- `internal/quotes/repository.go` - Query methods
- `internal/orders/repository.go` - Query methods
- `internal/assets/repository.go` - Query methods
- `cmd/api/handlers_business.go` - API handlers
- `scripts/demo-seed.sql` - Seed data

### Frontend (Svelte)
- `frontend/src/routes/quotes/+page.svelte` - Already implemented
- `frontend/src/routes/orders/+page.svelte` - Already implemented
- `frontend/src/routes/assets/+page.svelte` - Already implemented
- `frontend/src/routes/+layout.svelte` - Navigation

### Tests
- `frontend/e2e/demo/quotes.spec.ts` - To be created
- `frontend/e2e/demo/orders.spec.ts` - To be created
- `frontend/e2e/demo/fixed-assets.spec.ts` - To be created

## Appendix: E2E Test Results (2026-01-08)

```
Test Results:
- Quotes view: Heading=true, Table=false, EmptyState=true, NewButton=true
- Orders view: Heading=true, Table=false, EmptyState=true, NewButton=true
- Assets view: Heading=true, Table=false, EmptyState=true, NewButton=true

Conclusion: Views work but API returns no data for demo tenant.
```
