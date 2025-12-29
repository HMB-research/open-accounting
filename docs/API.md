# API Reference

Complete API reference for Open Accounting. Interactive documentation available at `/swagger/` when running the server.

## Base URL

```
http://localhost:8080/api/v1
```

## Authentication

All endpoints (except `/auth/*` and `/invitations/*`) require a Bearer token:

```bash
curl -H "Authorization: Bearer <access_token>" \
     https://api.example.com/api/v1/me
```

### Register

Create a new user account.

```http
POST /auth/register
Content-Type: application/json

{
  "email": "user@example.com",
  "password": "securepassword",
  "name": "John Doe"
}
```

**Response (201 Created):**
```json
{
  "id": "uuid",
  "email": "user@example.com",
  "name": "John Doe"
}
```

### Login

Authenticate and receive JWT tokens.

```http
POST /auth/login
Content-Type: application/json

{
  "email": "user@example.com",
  "password": "securepassword",
  "tenant_id": "uuid"  // Optional: login directly to a tenant
}
```

**Response (200 OK):**
```json
{
  "access_token": "eyJhbGc...",
  "refresh_token": "eyJhbGc...",
  "token_type": "Bearer",
  "expires_in": 900,
  "user": {
    "id": "uuid",
    "email": "user@example.com",
    "name": "John Doe"
  }
}
```

### Refresh Token

Exchange refresh token for new access token.

```http
POST /auth/refresh
Content-Type: application/json

{
  "refresh_token": "eyJhbGc...",
  "tenant_id": "uuid"  // Optional: switch tenant context
}
```

---

## User Endpoints

### Get Current User

```http
GET /me
Authorization: Bearer <token>
```

**Response:**
```json
{
  "id": "uuid",
  "email": "user@example.com",
  "name": "John Doe",
  "created_at": "2025-01-01T00:00:00Z"
}
```

### List User's Tenants

```http
GET /me/tenants
Authorization: Bearer <token>
```

**Response:**
```json
[
  {
    "tenant": {
      "id": "uuid",
      "name": "My Company",
      "slug": "my-company"
    },
    "role": "owner",
    "is_default": true
  }
]
```

---

## Tenant Endpoints

### Create Tenant

```http
POST /tenants
Authorization: Bearer <token>
Content-Type: application/json

{
  "name": "Acme Corp",
  "slug": "acme-corp",
  "settings": {
    "default_currency": "EUR",
    "country_code": "EE",
    "timezone": "Europe/Tallinn"
  }
}
```

### Get Tenant

```http
GET /tenants/{tenantId}
Authorization: Bearer <token>
```

---

## Accounts (Chart of Accounts)

### List Accounts

```http
GET /tenants/{tenantId}/accounts
Authorization: Bearer <token>
```

**Query Parameters:**
- `active_only` (bool): Filter for active accounts

**Response:**
```json
[
  {
    "id": "uuid",
    "code": "1000",
    "name": "Cash",
    "account_type": "ASSET",
    "parent_id": null,
    "is_active": true,
    "balance": "1000.00"
  }
]
```

### Create Account

```http
POST /tenants/{tenantId}/accounts
Authorization: Bearer <token>
Content-Type: application/json

{
  "code": "1010",
  "name": "Petty Cash",
  "account_type": "ASSET",
  "parent_id": "uuid",  // Optional
  "description": "Office petty cash"
}
```

**Account Types:** `ASSET`, `LIABILITY`, `EQUITY`, `REVENUE`, `EXPENSE`

---

## Journal Entries

### Create Journal Entry

```http
POST /tenants/{tenantId}/journal-entries
Authorization: Bearer <token>
Content-Type: application/json

{
  "entry_date": "2025-01-15",
  "description": "Office supplies purchase",
  "reference": "INV-001",
  "lines": [
    {
      "account_id": "uuid",
      "debit": "100.00",
      "credit": "0.00",
      "description": "Office supplies"
    },
    {
      "account_id": "uuid",
      "debit": "0.00",
      "credit": "100.00",
      "description": "Payment from cash"
    }
  ]
}
```

**Note:** Debits must equal credits.

### Post Journal Entry

Finalize a draft entry (makes it immutable).

```http
POST /tenants/{tenantId}/journal-entries/{entryId}/post
Authorization: Bearer <token>
```

### Void Journal Entry

Creates a reversal entry.

```http
POST /tenants/{tenantId}/journal-entries/{entryId}/void
Authorization: Bearer <token>
Content-Type: application/json

{
  "reason": "Duplicate entry"
}
```

---

## Contacts

### List Contacts

```http
GET /tenants/{tenantId}/contacts
Authorization: Bearer <token>
```

**Query Parameters:**
- `type` (string): `customer`, `supplier`, or `both`
- `search` (string): Search by name or email

### Create Contact

```http
POST /tenants/{tenantId}/contacts
Authorization: Bearer <token>
Content-Type: application/json

{
  "name": "ABC Supplier",
  "type": "supplier",
  "email": "contact@abc.com",
  "phone": "+372 555 1234",
  "vat_number": "EE123456789",
  "address": "123 Main St, Tallinn"
}
```

---

## Invoices

### List Invoices

```http
GET /tenants/{tenantId}/invoices
Authorization: Bearer <token>
```

**Query Parameters:**
- `type` (string): `SALES` or `PURCHASE`
- `status` (string): `DRAFT`, `SENT`, `PAID`, `VOID`
- `contact_id` (uuid): Filter by contact

### Create Invoice

```http
POST /tenants/{tenantId}/invoices
Authorization: Bearer <token>
Content-Type: application/json

{
  "invoice_type": "SALES",
  "contact_id": "uuid",
  "issue_date": "2025-01-15",
  "due_date": "2025-01-29",
  "currency": "EUR",
  "lines": [
    {
      "description": "Consulting services",
      "quantity": 10,
      "unit_price": "100.00",
      "vat_rate": "22.00"
    }
  ]
}
```

### Download Invoice PDF

```http
GET /tenants/{tenantId}/invoices/{invoiceId}/pdf
Authorization: Bearer <token>
```

Returns `application/pdf` file.

---

## Payments

### Create Payment

```http
POST /tenants/{tenantId}/payments
Authorization: Bearer <token>
Content-Type: application/json

{
  "payment_type": "RECEIVED",
  "contact_id": "uuid",
  "account_id": "uuid",
  "amount": "1220.00",
  "payment_date": "2025-01-20",
  "reference": "BANK-001"
}
```

**Payment Types:** `RECEIVED`, `MADE`

### Allocate Payment to Invoice

```http
POST /tenants/{tenantId}/payments/{paymentId}/allocate
Authorization: Bearer <token>
Content-Type: application/json

{
  "invoice_id": "uuid",
  "amount": "1220.00"
}
```

---

## User Management

### Invite User

```http
POST /tenants/{tenantId}/invitations
Authorization: Bearer <token>
Content-Type: application/json

{
  "email": "newuser@example.com",
  "role": "accountant"
}
```

**Roles:** `admin`, `accountant`, `viewer`

### Accept Invitation (Public)

```http
POST /invitations/accept
Content-Type: application/json

{
  "token": "invitation-token",
  "password": "newpassword",  // Required for new users
  "name": "New User"          // Required for new users
}
```

### List Tenant Users

```http
GET /tenants/{tenantId}/users
Authorization: Bearer <token>
```

### Update User Role

```http
PUT /tenants/{tenantId}/users/{userId}/role
Authorization: Bearer <token>
Content-Type: application/json

{
  "role": "admin"
}
```

---

## Reports

### Trial Balance

```http
GET /tenants/{tenantId}/reports/trial-balance
Authorization: Bearer <token>
```

**Query Parameters:**
- `as_of_date` (string): Date in YYYY-MM-DD format

### Account Balance

```http
GET /tenants/{tenantId}/reports/account-balance/{accountId}
Authorization: Bearer <token>
```

### Receivables Aging

```http
GET /tenants/{tenantId}/reports/aging/receivables
Authorization: Bearer <token>
```

---

## Estonian Tax (KMD)

### Generate KMD Declaration

```http
POST /tenants/{tenantId}/tax/kmd
Authorization: Bearer <token>
Content-Type: application/json

{
  "year": 2025,
  "month": 1
}
```

### Export KMD to XML

```http
GET /tenants/{tenantId}/tax/kmd/{year}/{month}/xml
Authorization: Bearer <token>
```

Returns `application/xml` file compatible with Estonian e-MTA.

---

## Error Responses

All errors return JSON with an `error` field:

```json
{
  "error": "Description of what went wrong"
}
```

### Common Status Codes

| Code | Meaning |
|------|---------|
| 400 | Bad Request - Invalid input |
| 401 | Unauthorized - Missing/invalid token |
| 403 | Forbidden - Insufficient permissions |
| 404 | Not Found - Resource doesn't exist |
| 500 | Internal Server Error |
