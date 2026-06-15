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
5. **Full-Stack Manifest Support**: Plugins can declare backend, frontend, and database components. Backend hooks and routes can run through loopback HTTP runtimes or supervised package runtimes. Frontend slots support safe declarative cards, links, actions, and operator-bundled registered components.

### Plugin Lifecycle

```
Not Installed → Installed → Enabled ↔ Disabled → Uninstalled
                    ↓
                  Failed
```

- **Not Installed**: Plugin exists in registry but not on this instance
- **Installed**: Plugin code downloaded, awaiting enablement
- **Enabled**: Plugin active for supported capabilities. Declarative frontend slots and navigation are available. Backend hooks and routes are available for `runtime: http` and supervised `runtime: package` executables.
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
  runtime: http                # http = externally managed loopback runtime
  base_url: http://127.0.0.1:9123

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
      label: Expense exceptions
      description: Review expenses that need receipts or approval
      path: /plugins/expense-tracker/exceptions
      kind: card               # card, link, or action
      badge: 4 open
      order: 100
    - name: invoice.sidebar
      component: ExpenseLink.svelte
      label: Related expenses
      path: /plugins/expense-tracker/invoices
      kind: link

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

### Backend Runtime Modes

`backend.runtime` selects how backend hooks and routes are executed:

| Runtime | Status | Required fields | Description |
|---------|--------|-----------------|-------------|
| omitted | Legacy manifest metadata | `package`, `entry` | Preserves old manifests. Hook and route declarations without an executable runtime are rejected during enablement. |
| `http` | Supported | `base_url` | Open Accounting proxies hooks and tenant plugin routes to an operator-managed HTTP process on loopback. `base_url` must use `http` and a loopback host such as `127.0.0.1`, `::1`, or `localhost`. |
| `package` | Supported, conservative supervisor | `package`, `executable` | Starts a plugin-local executable directly, waits for its loopback health endpoint, and proxies hooks and tenant plugin routes to declared handler paths. |

HTTP runtime manifests may keep legacy `package` and `entry` fields for compatibility, but the HTTP runtime uses `base_url` and handler paths. `backend.executable` is not valid with `runtime: http`.

Supervised package runtime manifests use plugin-relative paths only:

```yaml
backend:
  runtime: package
  package: ./backend            # Plugin-relative package directory
  executable: bin/expense-plugin # Slash-separated path inside package

  routes:
    - method: POST
      path: /expenses/import
      handler: /routes/import
```

Package runtime path rules are intentionally narrow:

- `package` and `executable` must be relative to the plugin repository, not absolute paths or URLs.
- Paths must stay inside the plugin package; `..` traversal is rejected.
- Paths must use `/` separators and must not contain whitespace or shell metacharacters.
- `base_url` is only valid for `runtime: http`.

Package runtime process contract:

- Open Accounting executes the resolved file directly with no shell and sets the working directory to `backend.package`.
- The executable must read `OPEN_ACCOUNTING_RUNTIME_ADDR` and listen on that loopback `host:port`.
- The executable must return a 2xx response from `OPEN_ACCOUNTING_RUNTIME_HEALTH_PATH` before hooks or routes are considered available.
- `OPEN_ACCOUNTING_RUNTIME_BASE_URL`, `OPEN_ACCOUNTING_PLUGIN_ID`, and `OPEN_ACCOUNTING_PLUGIN_NAME` are also provided for diagnostics.
- Operators can inspect lifecycle, health, crash/backoff, restart count, and last output through `GET /api/v1/admin/plugins/:id/runtime` or `oa admin plugins runtime status --id <plugin-id>`.
- Operators can restart supervised package runtimes through `POST /api/v1/admin/plugins/:id/runtime/restart` or `oa admin plugins runtime restart --id <plugin-id>`.
- If a package runtime exits unexpectedly after a healthy startup, Open Accounting keeps the crash/backoff status visible, waits for the restart backoff, then starts a replacement runtime and re-registers hooks.
- On unload or disable, Open Accounting unregisters hooks, sends an interrupt to the process, and kills it if it does not stop within the shutdown timeout.

Current limitation: package runtimes are supervised for startup, proxying, shutdown, manual restart, automatic crash restart, and crash/backoff reporting, but operating-system sandbox/resource isolation is not built in yet.

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

Backend event hooks execute through `runtime: http` when the manifest declares a loopback `base_url`, or through `runtime: package` after the supervised executable exposes its health endpoint. The plugin must have `hooks:register` permission. The application sends each hook invocation to the declared handler path on the runtime process.

Tenant outbound webhooks are the supported runtime notification path today. Use `webhooks create`, `webhooks test`, and `webhooks deliveries` in the CLI, or the `/tenants/{tenantId}/webhooks` API, to subscribe external systems to these events with signed HTTP delivery.

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

#### Expense Events
- `expense.created` - Expense claim created
- `expense.submitted` - Expense submitted for approval
- `expense.approved` - Expense approved
- `expense.rejected` - Expense rejected
- `expense.posted` - Expense posted to the ledger

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

#### Webhook Events
- `webhook.test` - Manual webhook endpoint test delivery

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

Frontend slot declarations render safe manifest-defined cards, links, and actions in host slot locations. The runtime uses `label`, `description`, `path`, `kind`, `badge`, and `order` from the manifest; `path` must be an internal application route. Plugin Svelte components are still not dynamically loaded, so `component` remains the stable component identifier and fallback label for future component-runtime work.

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

### Declarative Slot Entries

```yaml
frontend:
  components: ./frontend/components
  slots:
    - name: dashboard.widgets
      component: ExpenseWidget.svelte
      label: Expense exceptions
      description: Review expenses that need receipts or approval
      path: /plugins/expense-tracker/exceptions
      kind: card
      badge: 4 open
      order: 100
```

Supported `kind` values are `card`, `link`, and `action`. Links and actions require an internal `path`; external URLs and protocol-relative URLs are rejected by manifest validation.

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
package main

import (
    "encoding/json"
    "net/http"
)

func main() {
    mux := http.NewServeMux()
    mux.HandleFunc("/hooks/invoice", onInvoiceCreated)
    mux.HandleFunc("/routes/expenses", listExpenses)
    _ = http.ListenAndServe("127.0.0.1:9123", mux)
}

func onInvoiceCreated(w http.ResponseWriter, r *http.Request) {
    var payload struct {
        PluginID string          `json:"plugin_id"`
        Handler  string          `json:"handler"`
        Event    json.RawMessage `json:"event"`
    }
    _ = json.NewDecoder(r.Body).Decode(&payload)
    w.WriteHeader(http.StatusAccepted)
}

func listExpenses(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    _, _ = w.Write([]byte(`{"expenses":[]}`))
}
```

For `runtime: http`, run this process outside Open Accounting and point `backend.base_url` at its loopback address. Open Accounting forwards route requests with tenant and plugin headers and forwards hook payloads to the configured handler paths. Tenant runtime routes are invoked through `/api/v1/tenants/{tenantId}/plugins/{pluginId}/runtime/...` or the CLI `oa plugins runtime invoke --id <plugin-id> --method GET|POST|PUT|PATCH|DELETE --path <route>`. The CLI accepts a raw query string with `--query` and a JSON request body with either `--body-json` or `--body-file`. Successful runtime route responses are returned as the plugin produced them: Open Accounting preserves the runtime status code, forwards non-hop-by-hop response headers, and streams the raw response body instead of wrapping it in a host JSON envelope.

For `runtime: package`, build a self-contained executable inside the plugin repository and declare the containing package directory plus the executable path. Open Accounting launches the executable directly, waits for `OPEN_ACCOUNTING_RUNTIME_HEALTH_PATH`, forwards requests over loopback, reports lifecycle/crash/backoff state, supports manual runtime restart, and automatically restarts after unexpected exits. OS-level sandboxing remains outside the current supervisor.

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
GET    /api/v1/admin/plugins/:id/runtime   # Inspect backend runtime status
POST   /api/v1/admin/plugins/:id/runtime/restart # Restart package runtime
```

Admin plugin and registry endpoints require a tenant-scoped token whose current tenant membership is `owner` or `admin`; membership is rechecked when the request runs.

### Tenant Endpoints

```
GET    /api/v1/tenants/:id/plugins                    # List available plugins
POST   /api/v1/tenants/:id/plugins/:pid/enable        # Enable for tenant
POST   /api/v1/tenants/:id/plugins/:pid/disable       # Disable for tenant
GET    /api/v1/tenants/:id/plugins/:pid/settings      # Get settings
PUT    /api/v1/tenants/:id/plugins/:pid/settings      # Update settings
GET    /api/v1/tenants/:id/plugins/:pid/runtime/*     # Invoke declared runtime route
POST   /api/v1/tenants/:id/plugins/:pid/runtime/*     # Invoke declared runtime route
PUT    /api/v1/tenants/:id/plugins/:pid/runtime/*     # Invoke declared runtime route
PATCH  /api/v1/tenants/:id/plugins/:pid/runtime/*     # Invoke declared runtime route
DELETE /api/v1/tenants/:id/plugins/:pid/runtime/*     # Invoke declared runtime route
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
6. **Runtime Boundaries**: HTTP runtimes must be loopback-only. Package runtimes must declare safe plugin-relative package and executable paths, expose the assigned loopback health endpoint before use, report crash/backoff status, restart automatically after unexpected exits, and are stopped on unload. OS-level sandboxing remains outside the built-in supervisor.

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
- For backend hooks, confirm `backend.runtime: http` and a loopback `backend.base_url` are configured
- Check event type spelling in manifest
- Review handler implementation and runtime process logs
