# Frontend Architecture

This document describes the architecture, patterns, and conventions used in the Open Accounting frontend.

## Technology Stack

- **Framework**: SvelteKit 2.x with Svelte 5
- **Language**: TypeScript (strict mode)
- **Styling**: CSS custom properties with component-scoped styles
- **i18n**: Paraglide-JS (compile-time translations)
- **Testing**: Vitest (unit), Playwright (E2E)
- **Math**: Decimal.js for financial calculations

## Directory Structure

```
src/
├── lib/
│   ├── api.ts              # Type-safe API client
│   ├── components/         # Reusable UI components
│   ├── composables/        # Svelte 5 state composables
│   ├── paraglide/          # Generated i18n messages
│   ├── stores/             # Svelte stores (auth)
│   └── utils/              # Utility functions
├── routes/                 # SvelteKit file-based routing
│   ├── +layout.svelte      # Root layout with navigation
│   ├── +page.svelte        # Landing page
│   ├── dashboard/          # Dashboard view
│   ├── invoices/           # Invoice management
│   ├── contacts/           # Contact management
│   ├── accounts/           # Chart of accounts
│   ├── journal/            # Journal entries
│   ├── payments/           # Payment tracking
│   ├── quotes/             # Quote management
│   ├── orders/             # Order management
│   ├── assets/             # Fixed asset tracking
│   ├── inventory/          # Inventory management
│   ├── banking/            # Bank reconciliation
│   ├── employees/          # Employee management
│   ├── payroll/            # Payroll processing
│   ├── settings/           # Application settings
│   └── login/              # Authentication
├── tests/                  # Unit tests
└── paraglide/              # Paraglide config
```

## Core Patterns

### 1. API Client (`src/lib/api.ts`)

The API client provides type-safe methods for all backend endpoints with:

- **Automatic token management**: Access tokens stored in auth store, auto-refresh on expiry
- **Retry logic**: Exponential backoff with jitter for transient failures (5xx, 429)
- **Error handling**: Typed error responses with user-friendly messages

```typescript
// Example usage
const invoices = await api.getInvoices(tenantId, { status: 'SENT' });
const contact = await api.createContact(tenantId, { name: 'Acme Corp' });
```

Key exports:
- `api`: Singleton API client instance
- `buildQuery()`: Query string builder for filtering
- `RetryConfig`, `DEFAULT_RETRY_CONFIG`, `TEST_RETRY_CONFIG`: Retry configuration
- `isRetryableError()`, `calculateBackoffDelay()`: Retry utilities

### 2. Multi-Tenant Architecture

All data operations require a tenant ID passed as a URL query parameter:

```typescript
// URL structure
/invoices?tenant=uuid-here

// API calls include tenant ID
await api.getInvoices(tenantId, filter);
```

The `TenantSelector` component in the navbar allows switching between organizations.

### 3. Svelte 5 Patterns

Components use Svelte 5 runes for reactivity:

```svelte
<script lang="ts">
  // Props with defaults
  let { items = [], loading = false }: Props = $props();

  // Reactive state
  let count = $state(0);

  // Derived values
  let doubled = $derived(count * 2);

  // Bindable props
  let value = $bindable('');

  // Effects for side effects
  $effect(() => {
    if (value) loadData(value);
  });
</script>
```

### 4. Component Library

Reusable components in `src/lib/components/`:

| Component | Purpose |
|-----------|---------|
| `StatusBadge` | Config-driven status indicators |
| `FormModal` | Modal dialog with backdrop and accessibility |
| `ErrorAlert` | Dismissible error/warning/info/success alerts |
| `DateRangeFilter` | Date range picker with presets |
| `PeriodSelector` | Fiscal period selection |
| `ExportButton` | Excel/CSV/PDF export dropdown |
| `ActivityFeed` | Recent activity timeline |
| `TenantSelector` | Organization switcher |
| `LanguageSelector` | Locale switcher (en/et) |
| `ContactFormModal` | Quick contact creation form |
| `LineItemsEditor` | Invoice/quote/order line items table |
| `OnboardingWizard` | New tenant setup wizard |

### 5. Authentication

The auth store (`src/lib/stores/auth.ts`) manages:

- JWT access and refresh tokens
- "Remember me" persistence (localStorage vs sessionStorage)
- Automatic token refresh on API calls

```typescript
import { authStore, isAuthenticated } from '$lib/stores/auth';

// Check auth status
if ($isAuthenticated) {
  // User is logged in
}

// Login
authStore.setTokens(accessToken, refreshToken, rememberMe);

// Logout
authStore.clearTokens();
```

### 6. Internationalization

Paraglide-JS provides compile-time type-safe translations:

```svelte
<script>
  import * as m from '$lib/paraglide/messages.js';
</script>

<h1>{m.dashboard_title()}</h1>
<button>{m.common_save()}</button>
```

Translation files: `messages/en.json`, `messages/et.json`

### 7. Financial Calculations

All money operations use Decimal.js to avoid floating-point errors:

```typescript
import Decimal from 'decimal.js';
import { calculateLineTotal, calculateLinesTotal } from '$lib/utils/formatting';

const lineTotal = calculateLineTotal(line); // Returns Decimal
const total = calculateLinesTotal(lines);   // Returns Decimal
```

### 8. Utility Functions

**`src/lib/utils/formatting.ts`**
- `formatCurrency()` - Format amounts as EUR with proper locale
- `formatDate()` - Format dates for display
- `calculateLineTotal()` - Line item total with VAT and discount
- `calculateLinesTotal()` - Sum of all line items
- `createEmptyLine()` - Factory for new line items

**`src/lib/utils/dates.ts`**
- `calculateDateRange()` - Date range from presets (THIS_MONTH, etc.)
- `getTodayISO()` - Current date in ISO format

**`src/lib/utils/tenant.ts`**
- `getTenantFromUrl()` - Extract tenant ID from URL params

## Page Structure

Each page follows a consistent pattern:

```svelte
<script lang="ts">
  import { page } from '$app/stores';
  import { api } from '$lib/api';
  import * as m from '$lib/paraglide/messages.js';

  // Get tenant from URL
  let tenantId = $derived($page.url.searchParams.get('tenant') || '');

  // State
  let items = $state<Item[]>([]);
  let isLoading = $state(true);
  let error = $state('');

  // Load data on tenant change
  $effect(() => {
    if (tenantId) loadData();
  });

  async function loadData() {
    try {
      isLoading = true;
      items = await api.getItems(tenantId);
    } catch (err) {
      error = err instanceof Error ? err.message : 'Failed to load';
    } finally {
      isLoading = false;
    }
  }
</script>

<div class="page">
  {#if error}
    <ErrorAlert message={error} onDismiss={() => error = ''} />
  {/if}

  {#if isLoading}
    <div class="loading">Loading...</div>
  {:else}
    <!-- Page content -->
  {/if}
</div>
```

## Testing Strategy

### Unit Tests (Vitest)

Location: `src/tests/`

- API client methods
- Utility functions
- Component logic (not rendering due to Svelte 5 limitations)

```bash
bun test              # Run tests
bun run test:coverage # With coverage
```

### E2E Tests (Playwright)

Location: `e2e/`

- Full user flows
- Demo mode testing
- Cross-browser verification

```bash
bun run test:e2e      # Run all E2E tests
bun run test:demo     # Demo mode tests only
```

## CSS Architecture

Uses CSS custom properties for theming:

```css
:root {
  --color-primary: #4f46e5;
  --color-bg: #ffffff;
  --color-text: #1f2937;
  --color-border: #e5e7eb;
  --radius-md: 0.375rem;
  /* ... */
}
```

Component styles are scoped with responsive breakpoints:

```svelte
<style>
  .component {
    padding: 1rem;
  }

  @media (max-width: 768px) {
    .component {
      padding: 0.5rem;
    }
  }
</style>
```

## Error Handling

1. **API errors**: Caught and displayed via `ErrorAlert`
2. **Form validation**: HTML5 validation + custom checks
3. **Network errors**: Retry logic handles transient failures
4. **Global errors**: `+error.svelte` catches unhandled route errors

## Performance Considerations

- **Lazy loading**: Routes are code-split automatically
- **Decimal.js**: Precision over performance for financial data
- **Caching**: Browser caches static assets
- **SSR**: SvelteKit provides server-side rendering where beneficial
