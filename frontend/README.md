# Open Accounting Frontend

SvelteKit-based web application for Open Accounting.

## Quick Start

```bash
# Install dependencies
npm install

# Compile translations (required before first run)
npm run paraglide

# Start development server
npm run dev

# Access at http://localhost:5173
```

## Prerequisites

- Node.js 22+
- npm 10+
- Running backend API at http://localhost:8080

## Available Scripts

| Script | Description |
|--------|-------------|
| `npm run dev` | Start development server with HMR |
| `npm run build` | Build for production |
| `npm run preview` | Preview production build |
| `npm run check` | TypeScript type checking |
| `npm run check:watch` | Watch mode type checking |
| `npm run lint` | Run ESLint |
| `npm run format` | Format code with Prettier |
| `npm run test` | Run unit tests (Vitest) |
| `npm run test:watch` | Watch mode unit tests |
| `npm run test:coverage` | Run tests with coverage report |
| `npm run test:e2e` | Run E2E tests (Playwright) |
| `npm run test:e2e:ui` | E2E tests with UI |
| `npm run test:e2e:debug` | Debug E2E tests |

## Project Structure

```
frontend/
├── src/
│   ├── lib/
│   │   ├── api.ts              # API client (191 methods, 177 types)
│   │   ├── components/         # Reusable UI components
│   │   │   ├── StatusBadge.svelte      # Generic status badge
│   │   │   ├── TenantSelector.svelte   # Multi-tenant selector
│   │   │   ├── ErrorAlert.svelte       # Error display component
│   │   │   ├── FormModal.svelte        # Reusable modal wrapper
│   │   │   ├── LineItemsEditor.svelte  # Invoice line items
│   │   │   ├── DateRangeFilter.svelte  # Date filtering
│   │   │   ├── ExportButton.svelte     # Excel/PDF export
│   │   │   ├── PeriodSelector.svelte   # Fiscal period selector
│   │   │   ├── ActivityFeed.svelte     # Recent activity list
│   │   │   ├── ContactFormModal.svelte # Contact CRUD modal
│   │   │   ├── LanguageSelector.svelte # i18n switcher
│   │   │   └── OnboardingWizard.svelte # Initial setup wizard
│   │   ├── stores/             # Svelte stores (auth, tenant)
│   │   ├── utils/              # Utility functions
│   │   └── paraglide/          # Generated translations (do not edit)
│   │
│   └── routes/                 # SvelteKit file-based routing
│       ├── +layout.svelte      # Root layout with navbar
│       ├── dashboard/          # Dashboard with charts
│       ├── invoices/           # Invoice management
│       ├── quotes/             # Quote management
│       ├── orders/             # Order management
│       ├── payments/           # Payment tracking
│       ├── contacts/           # Customer/supplier management
│       ├── accounts/           # Chart of accounts
│       ├── journal/            # Journal entries
│       ├── reports/            # Financial reports
│       ├── banking/            # Bank reconciliation
│       ├── assets/             # Fixed asset tracking
│       ├── tax/                # Tax compliance (KMD)
│       ├── payroll/            # Payroll management
│       ├── employees/          # Employee management
│       └── settings/           # Company settings
│
├── e2e/                        # Playwright E2E tests
│   └── demo/                   # Demo environment tests
├── messages/                   # Translation source files
│   ├── en.json                 # English
│   └── et.json                 # Estonian
└── static/                     # Static assets
```

## Technology Stack

| Technology | Version | Purpose |
|------------|---------|---------|
| **SvelteKit** | 2.49+ | Full-stack framework |
| **Svelte** | 5.16+ | Component framework (runes) |
| **Vite** | 7.3+ | Build tool |
| **TypeScript** | 5.7+ | Type safety |
| **Paraglide-JS** | 2.7+ | Compile-time i18n |
| **Chart.js** | 4.5+ | Dashboard charts |
| **Decimal.js** | 10.4+ | Financial precision |
| **ExcelJS** | 4.4+ | Excel exports |
| **Vitest** | 4.0+ | Unit testing |
| **Playwright** | 1.57+ | E2E testing |

## Svelte 5 Runes

This project uses Svelte 5 with runes for reactivity:

```svelte
<script lang="ts">
  // Props with $props()
  let { value, onChange }: Props = $props();

  // Reactive state with $state()
  let count = $state(0);

  // Derived values with $derived()
  let doubled = $derived(count * 2);

  // Bindable props with $bindable()
  let { open = $bindable() }: Props = $props();
</script>
```

## Component Library

### StatusBadge

Generic status badge with configurable styling:

```svelte
<script>
  import StatusBadge from '$lib/components/StatusBadge.svelte';

  const invoiceConfig = {
    DRAFT: { class: 'secondary', label: 'Draft' },
    SENT: { class: 'info', label: 'Sent' },
    PAID: { class: 'success', label: 'Paid' },
    OVERDUE: { class: 'danger', label: 'Overdue' }
  };
</script>

<StatusBadge status="PAID" config={invoiceConfig} />
```

### TenantSelector

Multi-tenant company selector (appears in navbar):

```svelte
<script>
  import TenantSelector from '$lib/components/TenantSelector.svelte';
</script>

<TenantSelector {tenantId} {tenants} />
```

### ErrorAlert

Dismissible error display:

```svelte
<script>
  import ErrorAlert from '$lib/components/ErrorAlert.svelte';
  let error = $state('');
</script>

<ErrorAlert bind:error />
```

## API Client

The API client (`src/lib/api.ts`) provides typed methods for all backend endpoints:

```typescript
import { api, type Invoice, type Contact } from '$lib/api';

// List invoices with filtering
const invoices = await api.listInvoices(tenantId, { status: 'SENT' });

// Create a contact
const contact = await api.createContact(tenantId, {
  name: 'Acme Corp',
  email: 'billing@acme.com',
  is_customer: true
});

// Authentication is handled automatically via JWT
// Token refresh on 401 errors is built-in
```

## Testing

### Unit Tests (Vitest)

```bash
# Run all unit tests
npm run test

# Watch mode
npm run test:watch

# With coverage
npm run test:coverage
```

Test files: `src/**/*.test.ts`

### E2E Tests (Playwright)

```bash
# Run E2E tests
npm run test:e2e

# With UI
npm run test:e2e:ui

# Debug mode
npm run test:e2e:debug
```

Test files: `e2e/**/*.spec.ts`

E2E tests run against the demo environment with seeded data.

## Internationalization

Translations are managed with Paraglide-JS (compile-time i18n):

1. **Source files**: `messages/en.json`, `messages/et.json`
2. **Compile**: `npm run paraglide`
3. **Generated**: `src/lib/paraglide/messages.js`

Usage in components:

```svelte
<script>
  import * as m from '$lib/paraglide/messages.js';
</script>

<h1>{m.dashboard_title()}</h1>
<p>{m.invoices_count({ count: 5 })}</p>
```

Adding new translations:

1. Add key to `messages/en.json`
2. Add key to `messages/et.json`
3. Run `npm run paraglide`
4. Import and use in component

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `VITE_API_URL` | Backend API URL | `http://localhost:8080` |

## Code Style

- **Formatting**: Prettier with svelte-plugin
- **Linting**: ESLint with TypeScript rules
- **Components**: PascalCase filenames
- **Functions**: camelCase
- **Types**: PascalCase interfaces

```bash
# Format all files
npm run format

# Check linting
npm run lint
```

## Building for Production

```bash
# Build
npm run build

# Preview build
npm run preview

# Output in 'build/' directory
```

The production build uses `@sveltejs/adapter-node` for server deployment.

## Common Issues

### "Translations not found"

Run `npm run paraglide` to compile translations before starting dev server.

### "API connection refused"

Ensure the backend is running at http://localhost:8080.

### "Type errors in generated files"

Run `npm run check` to regenerate SvelteKit types.

## Related Documentation

- [Main README](../README.md) - Project overview
- [API Documentation](../docs/API.md) - Backend API reference
- [Architecture](../docs/ARCHITECTURE.md) - System design
