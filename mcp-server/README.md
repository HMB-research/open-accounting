# Open Accounting MCP Server

Model Context Protocol (MCP) server for Open Accounting, enabling AI assistants to interact with accounting features programmatically.

## Installation

```bash
cd mcp-server
npm install
npm run build
```

## Configuration

Set environment variables:
- `OPEN_ACCOUNTING_API_URL` - API base URL (default: `http://localhost:8080`)
- `OPEN_ACCOUNTING_API_TOKEN` - JWT or API token for authentication

## Usage with Claude Code

Add to your Claude Code configuration:

```bash
claude mcp add open-accounting -- npx tsx /path/to/mcp-server/src/index.ts
```

Or after building:

```bash
claude mcp add open-accounting -- node /path/to/mcp-server/dist/index.js
```

## Available Tools

| Tool | Description |
|------|-------------|
| `list_invoices` | List invoices with optional filters |
| `create_invoice` | Create a new invoice |
| `get_account_balance` | Get account balance as of date |
| `generate_report` | Generate financial reports |
| `list_contacts` | List customers and vendors |
| `record_payment` | Record a payment |
| `get_chart_of_accounts` | Get chart of accounts |
| `reset_demo_data` | Reset demo data (demo only) |

## Development

```bash
npm run dev  # Run with tsx (hot reload)
npm run build  # Build TypeScript
npm start  # Run built version
```
