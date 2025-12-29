// Package docs contains generated OpenAPI documentation
package docs

// @title Open Accounting API
// @version 1.0
// @description Open-source, multi-tenant accounting software API
// @description
// @description Features:
// @description - Multi-tenant with schema-per-tenant isolation
// @description - Double-entry bookkeeping
// @description - Contacts management (customers/suppliers)
// @description - Invoicing with VAT calculations
// @description - Payment tracking and allocation
// @description - Financial reporting (Trial Balance, Account Balances)

// @contact.name Open Accounting Team
// @contact.url https://github.com/HMB-research/open-accounting
// @contact.email support@openaccounting.io

// @license.name MIT
// @license.url https://opensource.org/licenses/MIT

// @host localhost:8080
// @BasePath /api/v1

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description JWT Bearer token authentication. Format: "Bearer {token}"

// @tag.name Auth
// @tag.description Authentication endpoints (register, login, token refresh)

// @tag.name Users
// @tag.description User profile management

// @tag.name Tenants
// @tag.description Multi-tenant organization management

// @tag.name Accounts
// @tag.description Chart of Accounts management

// @tag.name Journal Entries
// @tag.description Double-entry bookkeeping transactions

// @tag.name Contacts
// @tag.description Customer and supplier management

// @tag.name Invoices
// @tag.description Sales and purchase invoicing

// @tag.name Payments
// @tag.description Payment tracking and allocation

// @tag.name Reports
// @tag.description Financial reports and balances
