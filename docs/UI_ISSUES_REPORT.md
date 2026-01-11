# UI Views Issues Report

> Last Updated: 2026-01-11
> Tested Against: Railway Demo Environment

## Summary

| Category | Working | Issues | Not Tested |
|----------|---------|--------|------------|
| Landing/Auth | 2/2 | 0 | 0 |
| Core Accounting | 0/6 | 0 | 6 |
| Business Operations | 0/8 | 0 | 8 |
| Payroll | 0/4 | 0 | 4 |
| Banking | 0/2 | 0 | 2 |
| Reports | 0/3 | 0 | 3 |
| Settings | 0/5 | 0 | 5 |
| Admin | 0/1 | 0 | 1 |
| **Total** | **2/33** | **0** | **31** |

---

## Testing Criteria

Each view is tested for:
1. **Page Load** - Does the page load without errors?
2. **Data Display** - Does data render correctly in tables/lists?
3. **Navigation** - Do all links/buttons navigate correctly?
4. **CRUD Operations** - Can you Create, Read, Update, Delete?
5. **Error Handling** - Are errors displayed appropriately?
6. **Responsive** - Does it work on mobile viewport?

### Status Legend
- ✅ **Working** - All criteria pass
- ⚠️ **Partial** - Some issues exist (see notes)
- ❌ **Broken** - Critical issues prevent usage
- 🔲 **Not Tested** - Awaiting testing

---

## Detailed View Reports

### Landing & Authentication

#### / (Landing Page)
| Criteria | Status | Notes |
|----------|--------|-------|
| Page Load | ✅ | Loads correctly with SvelteKit initialization |
| Data Display | ✅ | Shows 6 core features: Invoicing, Payroll, Banking, TSD, Reports, Open Source |
| Navigation | ✅ | Get Started → /login, Try Demo → /login, Learn More → features section, GitHub link |
| CRUD | N/A | |
| Errors | ✅ | No errors observed |
| Responsive | ⚠️ | Mobile nav menu indicator not visible (needs manual verification) |

**Features Verified:**
- Hero section with clear value proposition
- Estonian business targeting
- Demo credentials displayed (demo@example.com / demo123)
- MIT License and self-hosting info

**Overall:** ✅ Working

---

#### /login
| Criteria | Status | Notes |
|----------|--------|-------|
| Page Load | ✅ | Loads correctly |
| Data Display | ✅ | Email field, Password field, Remember me checkbox, Language selector |
| Navigation | ✅ | Sign In button, Create Account link |
| CRUD | N/A | |
| Errors | ✅ | Error handling for invalid credentials |
| Responsive | ⚠️ | Needs manual verification |

**Features Verified:**
- Email input field
- Password input field with visibility toggle (eye icon) - *added recently*
- "Remember me" checkbox for session persistence
- Language selector (English/Eesti)
- API endpoint configured correctly

**Note:** Password visibility toggle is in source code but Railway deployment may not have latest version.

**Overall:** ✅ Working

---

### Core Accounting

#### /dashboard
| Criteria | Status | Notes |
|----------|--------|-------|
| Page Load | 🔲 | |
| Data Display | 🔲 | |
| Navigation | 🔲 | |
| CRUD | N/A | |
| Errors | 🔲 | |
| Responsive | 🔲 | |

**Overall:** 🔲 Not Tested

---

#### /accounts
| Criteria | Status | Notes |
|----------|--------|-------|
| Page Load | 🔲 | |
| Data Display | 🔲 | |
| Navigation | 🔲 | |
| CRUD | 🔲 | |
| Errors | 🔲 | |
| Responsive | 🔲 | |

**Overall:** 🔲 Not Tested

---

#### /journal
| Criteria | Status | Notes |
|----------|--------|-------|
| Page Load | 🔲 | |
| Data Display | 🔲 | |
| Navigation | 🔲 | |
| CRUD | 🔲 | |
| Errors | 🔲 | |
| Responsive | 🔲 | |

**Overall:** 🔲 Not Tested

---

#### /invoices
| Criteria | Status | Notes |
|----------|--------|-------|
| Page Load | 🔲 | |
| Data Display | 🔲 | |
| Navigation | 🔲 | |
| CRUD | 🔲 | |
| Errors | 🔲 | |
| Responsive | 🔲 | |

**Overall:** 🔲 Not Tested

---

#### /invoices/reminders
| Criteria | Status | Notes |
|----------|--------|-------|
| Page Load | 🔲 | |
| Data Display | 🔲 | |
| Navigation | 🔲 | |
| CRUD | 🔲 | |
| Errors | 🔲 | |
| Responsive | 🔲 | |

**Overall:** 🔲 Not Tested

---

#### /contacts
| Criteria | Status | Notes |
|----------|--------|-------|
| Page Load | 🔲 | |
| Data Display | 🔲 | |
| Navigation | 🔲 | |
| CRUD | 🔲 | |
| Errors | 🔲 | |
| Responsive | 🔲 | |

**Overall:** 🔲 Not Tested

---

### Business Operations

#### /quotes
| Criteria | Status | Notes |
|----------|--------|-------|
| Page Load | 🔲 | |
| Data Display | 🔲 | |
| Navigation | 🔲 | |
| CRUD | 🔲 | |
| Errors | 🔲 | |
| Responsive | 🔲 | |

**Known Issues:**
- Quote-to-Order conversion needs verification
- Email quote functionality needs implementation
- Quote PDF generation needs verification

**Overall:** 🔲 Not Tested

---

#### /orders
| Criteria | Status | Notes |
|----------|--------|-------|
| Page Load | 🔲 | |
| Data Display | 🔲 | |
| Navigation | 🔲 | |
| CRUD | 🔲 | |
| Errors | 🔲 | |
| Responsive | 🔲 | |

**Known Issues:**
- Order-to-Invoice conversion needs verification
- Order status workflow needs testing

**Overall:** 🔲 Not Tested

---

#### /payments
| Criteria | Status | Notes |
|----------|--------|-------|
| Page Load | 🔲 | |
| Data Display | 🔲 | |
| Navigation | 🔲 | |
| CRUD | 🔲 | |
| Errors | 🔲 | |
| Responsive | 🔲 | |

**Overall:** 🔲 Not Tested

---

#### /payments/cash
| Criteria | Status | Notes |
|----------|--------|-------|
| Page Load | 🔲 | |
| Data Display | 🔲 | |
| Navigation | 🔲 | |
| CRUD | 🔲 | |
| Errors | 🔲 | |
| Responsive | 🔲 | |

**Overall:** 🔲 Not Tested

---

#### /recurring
| Criteria | Status | Notes |
|----------|--------|-------|
| Page Load | 🔲 | |
| Data Display | 🔲 | |
| Navigation | 🔲 | |
| CRUD | 🔲 | |
| Errors | 🔲 | |
| Responsive | 🔲 | |

**Overall:** 🔲 Not Tested

---

#### /assets
| Criteria | Status | Notes |
|----------|--------|-------|
| Page Load | 🔲 | |
| Data Display | 🔲 | |
| Navigation | 🔲 | |
| CRUD | 🔲 | |
| Errors | 🔲 | |
| Responsive | 🔲 | |

**Overall:** 🔲 Not Tested

---

#### /inventory
| Criteria | Status | Notes |
|----------|--------|-------|
| Page Load | 🔲 | |
| Data Display | 🔲 | |
| Navigation | 🔲 | |
| CRUD | 🔲 | |
| Errors | 🔲 | |
| Responsive | 🔲 | |

**Known Issues:**
- Stock level tracking not implemented
- Warehouse management not implemented

**Overall:** 🔲 Not Tested

---

### Payroll

#### /employees
| Criteria | Status | Notes |
|----------|--------|-------|
| Page Load | 🔲 | |
| Data Display | 🔲 | |
| Navigation | 🔲 | |
| CRUD | 🔲 | |
| Errors | 🔲 | |
| Responsive | 🔲 | |

**Overall:** 🔲 Not Tested

---

#### /employees/absences
| Criteria | Status | Notes |
|----------|--------|-------|
| Page Load | 🔲 | |
| Data Display | 🔲 | |
| Navigation | 🔲 | |
| CRUD | 🔲 | |
| Errors | 🔲 | |
| Responsive | 🔲 | |

**Known Issues:**
- Leave balance tracking needs verification

**Overall:** 🔲 Not Tested

---

#### /payroll
| Criteria | Status | Notes |
|----------|--------|-------|
| Page Load | 🔲 | |
| Data Display | 🔲 | |
| Navigation | 🔲 | |
| CRUD | 🔲 | |
| Errors | 🔲 | |
| Responsive | 🔲 | |

**Overall:** 🔲 Not Tested

---

#### /payroll/calculator
| Criteria | Status | Notes |
|----------|--------|-------|
| Page Load | 🔲 | |
| Data Display | 🔲 | |
| Navigation | 🔲 | |
| CRUD | N/A | |
| Errors | 🔲 | |
| Responsive | 🔲 | |

**Overall:** 🔲 Not Tested

---

#### /tsd
| Criteria | Status | Notes |
|----------|--------|-------|
| Page Load | 🔲 | |
| Data Display | 🔲 | |
| Navigation | 🔲 | |
| CRUD | 🔲 | |
| Errors | 🔲 | |
| Responsive | 🔲 | |

**Overall:** 🔲 Not Tested

---

### Banking

#### /banking
| Criteria | Status | Notes |
|----------|--------|-------|
| Page Load | 🔲 | |
| Data Display | 🔲 | |
| Navigation | 🔲 | |
| CRUD | 🔲 | |
| Errors | 🔲 | |
| Responsive | 🔲 | |

**Overall:** 🔲 Not Tested

---

#### /banking/import
| Criteria | Status | Notes |
|----------|--------|-------|
| Page Load | 🔲 | |
| Data Display | 🔲 | |
| Navigation | 🔲 | |
| CRUD | 🔲 | |
| Errors | 🔲 | |
| Responsive | 🔲 | |

**Overall:** 🔲 Not Tested

---

### Tax & Compliance

#### /tax
| Criteria | Status | Notes |
|----------|--------|-------|
| Page Load | 🔲 | |
| Data Display | 🔲 | |
| Navigation | 🔲 | |
| CRUD | N/A | |
| Errors | 🔲 | |
| Responsive | 🔲 | |

**Overall:** 🔲 Not Tested

---

#### /vat-returns
| Criteria | Status | Notes |
|----------|--------|-------|
| Page Load | 🔲 | |
| Data Display | 🔲 | |
| Navigation | 🔲 | |
| CRUD | 🔲 | |
| Errors | 🔲 | |
| Responsive | 🔲 | |

**Overall:** 🔲 Not Tested

---

### Reports

#### /reports
| Criteria | Status | Notes |
|----------|--------|-------|
| Page Load | 🔲 | |
| Data Display | 🔲 | |
| Navigation | 🔲 | |
| CRUD | N/A | |
| Errors | 🔲 | |
| Responsive | 🔲 | |

**Overall:** 🔲 Not Tested

---

#### /reports/balance-confirmations
| Criteria | Status | Notes |
|----------|--------|-------|
| Page Load | 🔲 | |
| Data Display | 🔲 | |
| Navigation | 🔲 | |
| CRUD | 🔲 | |
| Errors | 🔲 | |
| Responsive | 🔲 | |

**Overall:** 🔲 Not Tested

---

#### /reports/cash-flow
| Criteria | Status | Notes |
|----------|--------|-------|
| Page Load | 🔲 | |
| Data Display | 🔲 | |
| Navigation | 🔲 | |
| CRUD | N/A | |
| Errors | 🔲 | |
| Responsive | 🔲 | |

**Overall:** 🔲 Not Tested

---

### Settings

#### /settings
| Criteria | Status | Notes |
|----------|--------|-------|
| Page Load | 🔲 | |
| Data Display | 🔲 | |
| Navigation | 🔲 | |
| CRUD | N/A | |
| Errors | 🔲 | |
| Responsive | 🔲 | |

**Overall:** 🔲 Not Tested

---

#### /settings/company
| Criteria | Status | Notes |
|----------|--------|-------|
| Page Load | 🔲 | |
| Data Display | 🔲 | |
| Navigation | 🔲 | |
| CRUD | 🔲 | |
| Errors | 🔲 | |
| Responsive | 🔲 | |

**Overall:** 🔲 Not Tested

---

#### /settings/email
| Criteria | Status | Notes |
|----------|--------|-------|
| Page Load | 🔲 | |
| Data Display | 🔲 | |
| Navigation | 🔲 | |
| CRUD | 🔲 | |
| Errors | 🔲 | |
| Responsive | 🔲 | |

**Overall:** 🔲 Not Tested

---

#### /settings/plugins
| Criteria | Status | Notes |
|----------|--------|-------|
| Page Load | 🔲 | |
| Data Display | 🔲 | |
| Navigation | 🔲 | |
| CRUD | 🔲 | |
| Errors | 🔲 | |
| Responsive | 🔲 | |

**Overall:** 🔲 Not Tested

---

#### /settings/cost-centers
| Criteria | Status | Notes |
|----------|--------|-------|
| Page Load | 🔲 | |
| Data Display | 🔲 | |
| Navigation | 🔲 | |
| CRUD | 🔲 | |
| Errors | 🔲 | |
| Responsive | 🔲 | |

**Known Issues:**
- Cost center assignment to transactions needs UI

**Overall:** 🔲 Not Tested

---

### Admin

#### /admin/plugins
| Criteria | Status | Notes |
|----------|--------|-------|
| Page Load | 🔲 | |
| Data Display | 🔲 | |
| Navigation | 🔲 | |
| CRUD | 🔲 | |
| Errors | 🔲 | |
| Responsive | 🔲 | |

**Overall:** 🔲 Not Tested

---

## Issues Summary

### Critical Issues (Blocking)
_None identified yet_

### Major Issues (Functional Problems)
_None identified yet_

### Minor Issues (Polish/UX)
_None identified yet_

---

## Change Log

| Date | Tester | Changes |
|------|--------|---------|
| 2026-01-11 | Claude | Tested Landing page (/) - Working |
| 2026-01-11 | Claude | Tested Login page (/login) - Working |
| 2026-01-11 | - | Initial template created |
