# Plugin System Documentation

Open Accounting supports a plugin marketplace that allows extending functionality through community-developed modules. This document covers plugin architecture, development, and deployment.

## Table of Contents

- [Overview](#overview)
- [Architecture](#architecture)
- [Plugin Manifest](#plugin-manifest)
- [Permission System](#permission-system)
- [Event Hooks](#event-hooks)
- [UI Extension Points](#ui-extension-points)
- [Plugin Development](#plugin-development)
- [Plugin Distribution](#plugin-distribution)
- [API Reference](#api-reference)

## Overview

### Key Principles

1. **Open Source Only**: Plugins must be publicly available on GitHub or GitLab
2. **Git-Based Distribution**: Plugins are cloned from repositories, no package registry
3. **Two-Level Enablement**: Installed instance-wide by admins, enabled per-tenant by users
4. **Permission-Based Security**: Plugins declare required permissions, users approve them
5. **Full-Stack Support**: Plugins can include backend, frontend, and database components

### Plugin Lifecycle

```
Not Installed → Installed → Enabled ↔ Disabled → Uninstalled
                    ↓
                  Failed
```

- **Not Installed**: Plugin exists in registry but not on this instance
- **Installed**: Plugin code downloaded, awaiting enablement
- **Enabled**: Plugin active, hooks registered, routes available
- **Disabled**: Plugin present but inactive
- **Failed**: Plugin encountered an error during loading

## Architecture

### Database Schema

```sql
-- Plugin registries (marketplace sources)
plugin_registries (
    id, name, url, description, is_official, is_active, last_synced_at
)

-- Installed plugins (instance-wide)
plugins (
    id, name, display_name, version, repository_url, repository_type,
    state, granted_permissions, manifest, installed_at
)

-- Per-tenant enablement
tenant_plugins (
    id, tenant_id, plugin_id, is_enabled, settings, enabled_at
)

-- Migration tracking
plugin_migrations (
    id, plugin_id, version, filename, applied_at, checksum
)
```

### File Structure

```
internal/plugin/
├── types.go          # Core data types
├── permissions.go    # Permission registry and validation
├── manifest.go       # YAML manifest parsing
├── service.go        # Main service (install, enable, etc.)
├── git.go            # Repository operations
├── hooks.go          # Event system
└── migration.go      # Database migration runner

frontend/src/lib/plugins/
├── manager.ts        # Plugin state management
├── Slot.svelte       # UI extension point component
└── index.ts          # Public exports
```

## Plugin Manifest

Every plugin must have a `plugin.yaml` file in the repository root:

```yaml
# Required metadata
name: expense-tracker           # Unique identifier (lowercase, hyphens)
display_name: Expense Tracker   # Human-readable name
version: 1.0.0                  # Semantic version

# Optional metadata
description: Track employee expenses with receipt scanning
author: Your Name
license: MIT
homepage: https://github.com/you/expense-tracker
min_app_version: 1.0.0

# Required permissions (see Permission System section)
permissions:
  - invoices:read
  - invoices:write
  - email:send
  - hooks:register

# Backend configuration (optional)
backend:
  package: ./backend           # Go package path
  entry: NewService            # Service constructor function

  hooks:                       # Event subscriptions
    - event: invoice.created
      handler: OnInvoiceCreated
    - event: payment.received
      handler: OnPaymentReceived

  routes:                      # API endpoints
    - method: GET
      path: /expenses
      handler: ListExpenses
    - method: POST
      path: /expenses
      handler: CreateExpense

# Frontend configuration (optional)
frontend:
  components: ./frontend/components  # Svelte component directory

  navigation:                  # Menu items to add
    - label: Expenses
      icon: receipt
      path: /expenses
      position: after:invoices  # Position hint

  slots:                       # UI injection points
    - name: dashboard.widgets
      component: ExpenseWidget.svelte
    - name: invoice.sidebar
      component: ExpenseLink.svelte

# Database configuration (optional)
database:
  migrations: ./migrations     # SQL migration directory

# Tenant settings schema (optional, JSON Schema format)
settings:
  type: object
  properties:
    receipt_required:
      type: boolean
      default: true
      description: Require receipt upload for expenses
    approval_threshold:
      type: number
      default: 100
      description: Expenses above this amount require approval
```

## Permission System

### Permission Categories

| Category | Risk Level | Description |
|----------|------------|-------------|
| Data Access | Low-Medium | Read/write business data |
| System | Medium | Use system services |
| Database | High | Direct database access |
| Dangerous | Critical | System-level changes |

### Available Permissions

#### Data Access
| Permission | Risk | Description |
|------------|------|-------------|
| `contacts:read` | Low | Read contact information |
| `contacts:write` | Low | Create and modify contacts |
| `invoices:read` | Low | Read invoices |
| `invoices:write` | Medium | Create and modify invoices |
| `payments:read` | Low | Read payment records |
| `payments:write` | Medium | Record payments |
| `accounts:read` | Low | Read chart of accounts |
| `accounts:write` | Medium | Modify chart of accounts |
| `employees:read` | Low | Read employee data |
| `employees:write` | Medium | Modify employee records |

#### System
| Permission | Risk | Description |
|------------|------|-------------|
| `email:send` | Medium | Send emails via system |
| `storage:read` | Low | Read stored files |
| `storage:write` | Medium | Upload and store files |
| `pdf:generate` | Low | Generate PDF documents |

#### Database
| Permission | Risk | Description |
|------------|------|-------------|
| `database:migrate` | High | Run SQL migrations |
| `database:query` | High | Execute SQL queries |

#### Dangerous
| Permission | Risk | Description |
|------------|------|-------------|
| `hooks:register` | Critical | Listen to system events |
| `routes:register` | Critical | Add API endpoints |
| `admin:access` | Critical | Access admin functions |

### Permission Approval Flow

1. Admin installs plugin from repository
2. System displays required permissions with risk levels
3. Admin reviews and approves specific permissions
4. Plugin is enabled with granted permissions
5. Tenants can enable plugin for their organization

## Event Hooks

Plugins can subscribe to system events to react to changes:

### Available Events

#### Invoice Events
- `invoice.created` - New invoice created
- `invoice.sent` - Invoice sent to customer
- `invoice.paid` - Invoice marked as paid
- `invoice.voided` - Invoice voided

#### Payment Events
- `payment.received` - Payment recorded
- `payment.allocated` - Payment allocated to invoice

#### Contact Events
- `contact.created` - New contact created
- `contact.updated` - Contact modified
- `contact.deleted` - Contact removed

#### Journal Entry Events
- `journal_entry.created` - Entry created
- `journal_entry.posted` - Entry posted
- `journal_entry.voided` - Entry voided

#### Recurring Events
- `recurring.created` - Recurring invoice setup
- `recurring.generated` - Invoice generated from template
- `recurring.stopped` - Recurring stopped

#### Banking Events
- `bank_transaction.imported` - Transactions imported
- `bank_transaction.matched` - Transaction matched
- `reconciliation.completed` - Reconciliation finished

#### Payroll Events
- `payroll.calculated` - Payroll run calculated
- `payroll.approved` - Payroll approved
- `employee.created` - New employee added

#### Tenant Events
- `tenant.created` - New tenant registered
- `tenant.updated` - Tenant settings changed

#### Email Events
- `email.sent` - Email sent successfully
- `email.failed` - Email delivery failed

### Event Payload Structure

```go
type Event struct {
    Type     string          `json:"type"`      // Event type
    TenantID uuid.UUID       `json:"tenant_id"` // Tenant context
    Data     json.RawMessage `json:"data"`      // Event-specific data
    Time     time.Time       `json:"time"`      // Event timestamp
}
```

## UI Extension Points

### Available Slots

| Slot Name | Location | Description |
|-----------|----------|-------------|
| `dashboard.widgets` | Dashboard | Widget cards area |
| `dashboard.actions` | Dashboard | Quick action buttons |
| `invoice.sidebar` | Invoice detail | Sidebar content |
| `invoice.actions` | Invoice detail | Action buttons |
| `contact.sidebar` | Contact detail | Sidebar content |
| `payment.sidebar` | Payment detail | Sidebar content |
| `settings.tabs` | Settings page | Additional tabs |
| `reports.custom` | Reports page | Custom report options |
| `header.actions` | Global header | Near logout button |

### Using Slots in Frontend

```svelte
<script>
  import { Slot } from '$lib/plugins';
</script>

<!-- In your page component -->
<Slot name="dashboard.widgets" props={{ tenantId }} />
```

### Navigation Positioning

Use position hints to control where navigation items appear:

- `after:invoices` - After the Invoices menu item
- `before:reports` - Before the Reports menu item
- `100` - Numeric position (lower = earlier)

## Plugin Development

### Repository Structure

```
my-plugin/
├── plugin.yaml           # Required: Plugin manifest
├── README.md             # Required: Documentation
├── LICENSE               # Required: Open source license
├── backend/
│   ├── service.go        # Main service
│   ├── handlers.go       # HTTP handlers
│   └── types.go          # Data types
├── frontend/
│   ├── components/
│   │   └── MyWidget.svelte
│   └── routes/
│       └── my-feature/
│           └── +page.svelte
└── migrations/
    ├── 001_create_tables.up.sql
    └── 001_create_tables.down.sql
```

### Backend Development

```go
// backend/service.go
package myplugin

import (
    "context"
    "github.com/jackc/pgx/v5/pgxpool"
)

type Service struct {
    pool *pgxpool.Pool
}

// NewService is the entry point called by the plugin system
func NewService(pool *pgxpool.Pool) *Service {
    return &Service{pool: pool}
}

// OnInvoiceCreated handles the invoice.created event
func (s *Service) OnInvoiceCreated(ctx context.Context, event plugin.Event) error {
    // Process event
    return nil
}

// ListExpenses handles GET /expenses
func (s *Service) ListExpenses(w http.ResponseWriter, r *http.Request) {
    // Handle request
}
```

### Frontend Development

```svelte
<!-- frontend/components/MyWidget.svelte -->
<script lang="ts">
  import { api } from '$lib/api';

  let { tenantId } = $props<{ tenantId: string }>();
  let data = $state([]);

  $effect(() => {
    // Load data from plugin API
  });
</script>

<div class="widget">
  <h3>My Widget</h3>
  <!-- Widget content -->
</div>
```

### Database Migrations

```sql
-- migrations/001_create_expenses.up.sql
CREATE TABLE IF NOT EXISTS expenses (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    employee_id UUID,
    amount DECIMAL(15,2) NOT NULL,
    description TEXT,
    receipt_url TEXT,
    status VARCHAR(20) DEFAULT 'pending',
    created_at TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX idx_expenses_tenant ON expenses(tenant_id);
```

## Plugin Distribution

### Creating a Registry

1. Create a GitHub/GitLab repository
2. Add a `plugins.yaml` file:

```yaml
version: 1
plugins:
  - name: expense-tracker
    display_name: Expense Tracker
    description: Track employee expenses
    repository: https://github.com/you/expense-tracker
    version: 1.0.0
    author: Your Name
    license: MIT
    tags: [expenses, receipts, hr]

  - name: time-tracking
    display_name: Time Tracking
    description: Track billable hours
    repository: https://github.com/you/time-tracking
    version: 2.0.0
    author: Your Name
    license: MIT
    tags: [time, billing, hours]
```

### Adding a Registry

Admins can add custom registries:

1. Go to Admin → Plugin Marketplace
2. Click "Registries" tab
3. Click "Add Registry"
4. Enter the GitHub/GitLab repository URL

### Requirements for Plugins

- Public GitHub or GitLab repository
- Valid `plugin.yaml` manifest
- LICENSE file (OSI-approved license required)
- README.md documentation

## API Reference

### Admin Endpoints

```
GET    /api/v1/admin/plugin-registries     # List registries
POST   /api/v1/admin/plugin-registries     # Add registry
DELETE /api/v1/admin/plugin-registries/:id # Remove registry
POST   /api/v1/admin/plugin-registries/:id/sync # Sync registry

GET    /api/v1/admin/plugins               # List installed plugins
GET    /api/v1/admin/plugins/search?q=     # Search plugins
GET    /api/v1/admin/plugins/permissions   # List all permissions
POST   /api/v1/admin/plugins/install       # Install plugin
GET    /api/v1/admin/plugins/:id           # Get plugin details
DELETE /api/v1/admin/plugins/:id           # Uninstall plugin
POST   /api/v1/admin/plugins/:id/enable    # Enable with permissions
POST   /api/v1/admin/plugins/:id/disable   # Disable plugin
```

### Tenant Endpoints

```
GET    /api/v1/tenants/:id/plugins                    # List available plugins
POST   /api/v1/tenants/:id/plugins/:pid/enable        # Enable for tenant
POST   /api/v1/tenants/:id/plugins/:pid/disable       # Disable for tenant
GET    /api/v1/tenants/:id/plugins/:pid/settings      # Get settings
PUT    /api/v1/tenants/:id/plugins/:pid/settings      # Update settings
```

### Request/Response Examples

#### Install Plugin
```bash
POST /api/v1/admin/plugins/install
{
  "repository_url": "https://github.com/user/my-plugin"
}

Response:
{
  "id": "uuid",
  "name": "my-plugin",
  "display_name": "My Plugin",
  "version": "1.0.0",
  "state": "installed",
  "manifest": {...}
}
```

#### Enable Plugin
```bash
POST /api/v1/admin/plugins/:id/enable
{
  "permissions": ["invoices:read", "hooks:register"]
}

Response:
{
  "id": "uuid",
  "name": "my-plugin",
  "state": "enabled",
  "granted_permissions": ["invoices:read", "hooks:register"]
}
```

## Security Considerations

1. **Repository Validation**: Only public GitHub/GitLab repos accepted
2. **License Required**: OSI-approved license file mandatory
3. **Permission Review**: Admins must approve each permission
4. **Risk Warnings**: High-risk permissions highlighted in UI
5. **Tenant Isolation**: Plugin data scoped to tenant schemas
6. **No Code Execution**: Plugins run in same process (future: sandboxing)

## Troubleshooting

### Plugin Won't Install
- Verify repository is public
- Check plugin.yaml syntax
- Ensure LICENSE file exists
- Check network connectivity

### Plugin Won't Enable
- Review required permissions
- Check for dependency conflicts
- Look for migration errors in logs

### UI Components Not Showing
- Verify slot names match exactly
- Check plugin is enabled for tenant
- Reload page after enabling

### Events Not Firing
- Confirm hooks:register permission granted
- Check event type spelling in manifest
- Review handler implementation
