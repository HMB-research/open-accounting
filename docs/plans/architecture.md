# Open Accounting - System Architecture

> Comprehensive open-source accounting software replicating SmartAccounts functionality

**Technology Stack:** Go + SvelteKit + PostgreSQL
**Multi-tenancy:** Schema-per-tenant
**Precision:** NUMERIC(28,8) for money, NUMERIC(18,10) for exchange rates

---

## Table of Contents

1. [Core Accounting](#section-1-core-accounting)
2. [Purchase & Sales](#section-2-purchase--sales)
3. [VAT Management](#section-3-vat-management)
4. [Payments](#section-4-payments)
5. [Inventory](#section-5-inventory)
6. [Payroll](#section-6-payroll)
7. [E-Invoicing](#section-7-e-invoicing)
8. [Reports](#section-8-reports)
9. [Frontend Architecture](#section-9-frontend-architecture)
10. [Testing Strategy](#section-10-testing-strategy)
11. [Deployment & CI/CD](#section-11-deployment--cicd)

---

## Section 1: Core Accounting

### 1.1 Database Schema

```sql
-- Multi-tenant base
CREATE TABLE tenants (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            VARCHAR(255) NOT NULL,
    schema_name     VARCHAR(63) NOT NULL UNIQUE,
    reg_code        VARCHAR(20),
    vat_number      VARCHAR(20),
    address         JSONB,
    base_currency   CHAR(3) DEFAULT 'EUR',
    fiscal_year_start INTEGER DEFAULT 1,  -- Month (1-12)
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    is_active       BOOLEAN DEFAULT true
);

-- Chart of Accounts
CREATE TABLE accounts (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID NOT NULL REFERENCES tenants(id),
    code            VARCHAR(20) NOT NULL,
    name            VARCHAR(255) NOT NULL,
    account_type    VARCHAR(20) NOT NULL,  -- ASSET, LIABILITY, EQUITY, REVENUE, EXPENSE
    parent_id       UUID REFERENCES accounts(id),
    is_active       BOOLEAN DEFAULT true,
    is_system       BOOLEAN DEFAULT false, -- Cannot be deleted
    description     TEXT,
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(tenant_id, code)
);

CREATE INDEX idx_accounts_tenant ON accounts(tenant_id);
CREATE INDEX idx_accounts_type ON accounts(tenant_id, account_type);

-- Immutable Journal Entries
CREATE TABLE journal_entries (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID NOT NULL REFERENCES tenants(id),
    entry_number    VARCHAR(20) NOT NULL,
    entry_date      DATE NOT NULL,
    description     TEXT NOT NULL,
    reference       VARCHAR(100),
    source_type     VARCHAR(20),           -- MANUAL, INVOICE, PAYMENT, PAYROLL
    source_id       UUID,
    status          VARCHAR(10) DEFAULT 'DRAFT',  -- DRAFT, POSTED, VOIDED
    posted_at       TIMESTAMPTZ,
    posted_by       UUID REFERENCES users(id),
    voided_at       TIMESTAMPTZ,
    voided_by       UUID REFERENCES users(id),
    void_reason     TEXT,
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    created_by      UUID REFERENCES users(id),
    UNIQUE(tenant_id, entry_number)
);

CREATE INDEX idx_journal_entries_tenant_date ON journal_entries(tenant_id, entry_date);
CREATE INDEX idx_journal_entries_source ON journal_entries(tenant_id, source_type, source_id);

-- Journal Entry Lines (Double-Entry)
CREATE TABLE journal_entry_lines (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID NOT NULL REFERENCES tenants(id),
    journal_entry_id UUID NOT NULL REFERENCES journal_entries(id),
    account_id      UUID NOT NULL REFERENCES accounts(id),
    description     TEXT,
    debit_amount    NUMERIC(28,8) NOT NULL DEFAULT 0,
    credit_amount   NUMERIC(28,8) NOT NULL DEFAULT 0,
    currency        CHAR(3) NOT NULL DEFAULT 'EUR',
    exchange_rate   NUMERIC(18,10) DEFAULT 1,
    base_debit      NUMERIC(28,8) NOT NULL DEFAULT 0,  -- In tenant base currency
    base_credit     NUMERIC(28,8) NOT NULL DEFAULT 0,
    CONSTRAINT chk_debit_or_credit CHECK (
        (debit_amount > 0 AND credit_amount = 0) OR
        (credit_amount > 0 AND debit_amount = 0)
    )
);

CREATE INDEX idx_jel_entry ON journal_entry_lines(journal_entry_id);
CREATE INDEX idx_jel_account ON journal_entry_lines(account_id);

-- Users
CREATE TABLE users (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID NOT NULL REFERENCES tenants(id),
    email           VARCHAR(255) NOT NULL,
    password_hash   VARCHAR(255) NOT NULL,
    name            VARCHAR(255) NOT NULL,
    role            VARCHAR(20) NOT NULL DEFAULT 'USER',
    is_active       BOOLEAN DEFAULT true,
    last_login_at   TIMESTAMPTZ,
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(tenant_id, email)
);
```

### 1.2 Core Accounting Types (Go)

```go
package accounting

import (
    "time"
    "github.com/shopspring/decimal"
)

type AccountType string

const (
    AccountTypeAsset     AccountType = "ASSET"
    AccountTypeLiability AccountType = "LIABILITY"
    AccountTypeEquity    AccountType = "EQUITY"
    AccountTypeRevenue   AccountType = "REVENUE"
    AccountTypeExpense   AccountType = "EXPENSE"
)

type JournalEntryStatus string

const (
    StatusDraft  JournalEntryStatus = "DRAFT"
    StatusPosted JournalEntryStatus = "POSTED"
    StatusVoided JournalEntryStatus = "VOIDED"
)

type Account struct {
    ID          string      `json:"id"`
    TenantID    string      `json:"tenant_id"`
    Code        string      `json:"code"`
    Name        string      `json:"name"`
    AccountType AccountType `json:"account_type"`
    ParentID    *string     `json:"parent_id,omitempty"`
    IsActive    bool        `json:"is_active"`
    IsSystem    bool        `json:"is_system"`
    Description string      `json:"description,omitempty"`
    CreatedAt   time.Time   `json:"created_at"`
}

type JournalEntry struct {
    ID          string             `json:"id"`
    TenantID    string             `json:"tenant_id"`
    EntryNumber string             `json:"entry_number"`
    EntryDate   time.Time          `json:"entry_date"`
    Description string             `json:"description"`
    Reference   string             `json:"reference,omitempty"`
    SourceType  string             `json:"source_type,omitempty"`
    SourceID    *string            `json:"source_id,omitempty"`
    Status      JournalEntryStatus `json:"status"`
    Lines       []JournalEntryLine `json:"lines"`
    PostedAt    *time.Time         `json:"posted_at,omitempty"`
    PostedBy    *string            `json:"posted_by,omitempty"`
    CreatedAt   time.Time          `json:"created_at"`
    CreatedBy   string             `json:"created_by"`
}

type JournalEntryLine struct {
    ID             string          `json:"id"`
    JournalEntryID string          `json:"journal_entry_id"`
    AccountID      string          `json:"account_id"`
    Description    string          `json:"description,omitempty"`
    DebitAmount    decimal.Decimal `json:"debit_amount"`
    CreditAmount   decimal.Decimal `json:"credit_amount"`
    Currency       string          `json:"currency"`
    ExchangeRate   decimal.Decimal `json:"exchange_rate"`
    BaseDebit      decimal.Decimal `json:"base_debit"`
    BaseCredit     decimal.Decimal `json:"base_credit"`
}

// Validate ensures debits equal credits
func (je *JournalEntry) Validate() error {
    if len(je.Lines) == 0 {
        return errors.New("journal entry must have at least one line")
    }

    totalDebits := decimal.Zero
    totalCredits := decimal.Zero

    for _, line := range je.Lines {
        totalDebits = totalDebits.Add(line.BaseDebit)
        totalCredits = totalCredits.Add(line.BaseCredit)
    }

    if !totalDebits.Equal(totalCredits) {
        return fmt.Errorf("journal entry does not balance: debits=%s, credits=%s",
            totalDebits, totalCredits)
    }

    if totalDebits.IsZero() {
        return errors.New("journal entry cannot have zero amounts")
    }

    return nil
}
```

### 1.3 Journal Entry Service

```go
package accounting

import (
    "context"
    "fmt"
    "time"

    "github.com/jackc/pgx/v5/pgxpool"
)

type JournalService struct {
    db *pgxpool.Pool
}

func NewJournalService(db *pgxpool.Pool) *JournalService {
    return &JournalService{db: db}
}

func (s *JournalService) CreateEntry(ctx context.Context, tenantID string, req *CreateJournalEntryRequest) (*JournalEntry, error) {
    entry := &JournalEntry{
        ID:          uuid.New().String(),
        TenantID:    tenantID,
        EntryDate:   req.EntryDate,
        Description: req.Description,
        Reference:   req.Reference,
        Status:      StatusDraft,
        Lines:       req.Lines,
        CreatedAt:   time.Now(),
        CreatedBy:   req.UserID,
    }

    // Validate balance
    if err := entry.Validate(); err != nil {
        return nil, err
    }

    tx, err := s.db.Begin(ctx)
    if err != nil {
        return nil, fmt.Errorf("begin transaction: %w", err)
    }
    defer tx.Rollback(ctx)

    // Generate entry number
    var entryNumber string
    err = tx.QueryRow(ctx, `
        SELECT COALESCE(MAX(CAST(SUBSTRING(entry_number FROM 4) AS INTEGER)), 0) + 1
        FROM journal_entries WHERE tenant_id = $1
    `, tenantID).Scan(&entryNumber)
    if err != nil {
        return nil, fmt.Errorf("generate entry number: %w", err)
    }
    entry.EntryNumber = fmt.Sprintf("JE-%05d", entryNumber)

    // Insert entry
    _, err = tx.Exec(ctx, `
        INSERT INTO journal_entries (id, tenant_id, entry_number, entry_date, description, reference, source_type, source_id, status, created_at, created_by)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
    `, entry.ID, entry.TenantID, entry.EntryNumber, entry.EntryDate, entry.Description,
       entry.Reference, entry.SourceType, entry.SourceID, entry.Status, entry.CreatedAt, entry.CreatedBy)
    if err != nil {
        return nil, fmt.Errorf("insert entry: %w", err)
    }

    // Insert lines
    for i := range entry.Lines {
        line := &entry.Lines[i]
        line.ID = uuid.New().String()
        line.JournalEntryID = entry.ID

        _, err = tx.Exec(ctx, `
            INSERT INTO journal_entry_lines (id, tenant_id, journal_entry_id, account_id, description, debit_amount, credit_amount, currency, exchange_rate, base_debit, base_credit)
            VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
        `, line.ID, tenantID, line.JournalEntryID, line.AccountID, line.Description,
           line.DebitAmount, line.CreditAmount, line.Currency, line.ExchangeRate, line.BaseDebit, line.BaseCredit)
        if err != nil {
            return nil, fmt.Errorf("insert line: %w", err)
        }
    }

    if err := tx.Commit(ctx); err != nil {
        return nil, fmt.Errorf("commit: %w", err)
    }

    return entry, nil
}

func (s *JournalService) PostEntry(ctx context.Context, tenantID, entryID, userID string) error {
    result, err := s.db.Exec(ctx, `
        UPDATE journal_entries
        SET status = $1, posted_at = $2, posted_by = $3
        WHERE id = $4 AND tenant_id = $5 AND status = $6
    `, StatusPosted, time.Now(), userID, entryID, tenantID, StatusDraft)

    if err != nil {
        return fmt.Errorf("post entry: %w", err)
    }

    if result.RowsAffected() == 0 {
        return errors.New("entry not found or already posted")
    }

    return nil
}

func (s *JournalService) VoidEntry(ctx context.Context, tenantID, entryID, userID, reason string) (*JournalEntry, error) {
    tx, err := s.db.Begin(ctx)
    if err != nil {
        return nil, err
    }
    defer tx.Rollback(ctx)

    // Get original entry
    original, err := s.getEntryByID(ctx, tx, tenantID, entryID)
    if err != nil {
        return nil, err
    }

    if original.Status != StatusPosted {
        return nil, errors.New("only posted entries can be voided")
    }

    // Mark original as voided
    _, err = tx.Exec(ctx, `
        UPDATE journal_entries
        SET status = $1, voided_at = $2, voided_by = $3, void_reason = $4
        WHERE id = $5 AND tenant_id = $6
    `, StatusVoided, time.Now(), userID, reason, entryID, tenantID)
    if err != nil {
        return nil, err
    }

    // Create reversing entry
    reversal := &JournalEntry{
        ID:          uuid.New().String(),
        TenantID:    tenantID,
        EntryDate:   time.Now(),
        Description: fmt.Sprintf("Reversal of %s: %s", original.EntryNumber, reason),
        Reference:   original.EntryNumber,
        SourceType:  "VOID",
        SourceID:    &original.ID,
        Status:      StatusPosted,
        PostedAt:    ptrTime(time.Now()),
        PostedBy:    &userID,
        CreatedAt:   time.Now(),
        CreatedBy:   userID,
    }

    // Reverse debits and credits
    for _, line := range original.Lines {
        reversal.Lines = append(reversal.Lines, JournalEntryLine{
            AccountID:    line.AccountID,
            Description:  "Reversal",
            DebitAmount:  line.CreditAmount,  // Swap
            CreditAmount: line.DebitAmount,   // Swap
            Currency:     line.Currency,
            ExchangeRate: line.ExchangeRate,
            BaseDebit:    line.BaseCredit,
            BaseCredit:   line.BaseDebit,
        })
    }

    // Insert reversal (similar to CreateEntry)
    // ... insert logic ...

    if err := tx.Commit(ctx); err != nil {
        return nil, err
    }

    return reversal, nil
}
```

### 1.4 Testing Requirements

```go
func TestJournalEntry_MustBalance(t *testing.T) {
    // Test that debits equal credits
    // Test rejection of unbalanced entries
    // Test handling of large numbers (NUMERIC(28,8))
}

func TestJournalEntry_ImmutableWhenPosted(t *testing.T) {
    // Test that posted entries cannot be modified
    // Test void creates reversal instead of deletion
}

func TestAccountBalance_Calculation(t *testing.T) {
    // Test balance calculation from journal entries
    // Test debit-normal vs credit-normal accounts
}
```

---

## Section 2: Purchase & Sales

### 2.1 Database Schema

```sql
-- Partners (Customers and Vendors)
CREATE TABLE partners (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID NOT NULL REFERENCES tenants(id),
    name            VARCHAR(255) NOT NULL,
    reg_code        VARCHAR(20),
    vat_number      VARCHAR(20),
    partner_type    VARCHAR(10) NOT NULL,  -- CUSTOMER, VENDOR, BOTH
    email           VARCHAR(255),
    phone           VARCHAR(50),
    address         JSONB,
    payment_terms_days INTEGER DEFAULT 14,
    credit_limit    NUMERIC(28,8),
    default_account_id UUID REFERENCES accounts(id),
    is_active       BOOLEAN DEFAULT true,
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(tenant_id, reg_code)
);

CREATE INDEX idx_partners_tenant ON partners(tenant_id);
CREATE INDEX idx_partners_type ON partners(tenant_id, partner_type);

-- Invoices (Sales and Purchase)
CREATE TABLE invoices (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID NOT NULL REFERENCES tenants(id),
    invoice_number  VARCHAR(20) NOT NULL,
    invoice_type    VARCHAR(10) NOT NULL,  -- SALES, PURCHASE
    partner_id      UUID NOT NULL REFERENCES partners(id),
    invoice_date    DATE NOT NULL,
    due_date        DATE NOT NULL,
    currency        CHAR(3) NOT NULL DEFAULT 'EUR',
    exchange_rate   NUMERIC(18,10) DEFAULT 1,
    subtotal        NUMERIC(28,8) NOT NULL DEFAULT 0,
    tax_total       NUMERIC(28,8) NOT NULL DEFAULT 0,
    total           NUMERIC(28,8) NOT NULL DEFAULT 0,
    balance_due     NUMERIC(28,8) NOT NULL DEFAULT 0,
    status          VARCHAR(20) DEFAULT 'DRAFT',
    notes           TEXT,
    internal_notes  TEXT,
    journal_entry_id UUID REFERENCES journal_entries(id),
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    created_by      UUID REFERENCES users(id),
    UNIQUE(tenant_id, invoice_number, invoice_type)
);

CREATE INDEX idx_invoices_tenant_date ON invoices(tenant_id, invoice_date);
CREATE INDEX idx_invoices_partner ON invoices(partner_id);
CREATE INDEX idx_invoices_status ON invoices(tenant_id, status);

-- Invoice Lines
CREATE TABLE invoice_lines (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID NOT NULL REFERENCES tenants(id),
    invoice_id      UUID NOT NULL REFERENCES invoices(id) ON DELETE CASCADE,
    line_number     INTEGER NOT NULL,
    item_id         UUID REFERENCES items(id),
    description     TEXT NOT NULL,
    quantity        NUMERIC(28,8) NOT NULL DEFAULT 1,
    unit            VARCHAR(20),
    unit_price      NUMERIC(28,8) NOT NULL,
    discount_percent NUMERIC(5,2) DEFAULT 0,
    discount_amount NUMERIC(28,8) DEFAULT 0,
    line_net        NUMERIC(28,8) NOT NULL,
    vat_rate_id     UUID REFERENCES vat_rates(id),
    vat_rate        NUMERIC(5,2) NOT NULL,
    vat_amount      NUMERIC(28,8) NOT NULL,
    line_total      NUMERIC(28,8) NOT NULL,
    account_id      UUID REFERENCES accounts(id),
    cost_center_id  UUID,
    project_id      UUID,
    UNIQUE(invoice_id, line_number)
);

CREATE INDEX idx_invoice_lines_invoice ON invoice_lines(invoice_id);

-- Items/Products
CREATE TABLE items (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID NOT NULL REFERENCES tenants(id),
    code            VARCHAR(50) NOT NULL,
    name            VARCHAR(255) NOT NULL,
    description     TEXT,
    item_type       VARCHAR(20) NOT NULL,  -- PRODUCT, SERVICE
    unit            VARCHAR(20) DEFAULT 'pcs',
    sales_price     NUMERIC(28,8),
    purchase_price  NUMERIC(28,8),
    sales_vat_rate_id UUID REFERENCES vat_rates(id),
    purchase_vat_rate_id UUID REFERENCES vat_rates(id),
    sales_account_id UUID REFERENCES accounts(id),
    purchase_account_id UUID REFERENCES accounts(id),
    inventory_account_id UUID REFERENCES accounts(id),
    is_active       BOOLEAN DEFAULT true,
    track_inventory BOOLEAN DEFAULT false,
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(tenant_id, code)
);
```

### 2.2 Invoice Service (Go)

```go
package invoicing

import (
    "context"
    "time"

    "github.com/shopspring/decimal"
)

type InvoiceType string

const (
    InvoiceTypeSales    InvoiceType = "SALES"
    InvoiceTypePurchase InvoiceType = "PURCHASE"
)

type InvoiceStatus string

const (
    InvoiceStatusDraft         InvoiceStatus = "DRAFT"
    InvoiceStatusSent          InvoiceStatus = "SENT"
    InvoiceStatusPartiallyPaid InvoiceStatus = "PARTIALLY_PAID"
    InvoiceStatusPaid          InvoiceStatus = "PAID"
    InvoiceStatusCancelled     InvoiceStatus = "CANCELLED"
)

type Invoice struct {
    ID            string          `json:"id"`
    TenantID      string          `json:"tenant_id"`
    InvoiceNumber string          `json:"invoice_number"`
    InvoiceType   InvoiceType     `json:"invoice_type"`
    PartnerID     string          `json:"partner_id"`
    Partner       *Partner        `json:"partner,omitempty"`
    InvoiceDate   time.Time       `json:"invoice_date"`
    DueDate       time.Time       `json:"due_date"`
    Currency      string          `json:"currency"`
    ExchangeRate  decimal.Decimal `json:"exchange_rate"`
    Subtotal      decimal.Decimal `json:"subtotal"`
    TaxTotal      decimal.Decimal `json:"tax_total"`
    Total         decimal.Decimal `json:"total"`
    BalanceDue    decimal.Decimal `json:"balance_due"`
    Status        InvoiceStatus   `json:"status"`
    Lines         []InvoiceLine   `json:"lines"`
    JournalEntryID *string        `json:"journal_entry_id,omitempty"`
    CreatedAt     time.Time       `json:"created_at"`
}

type InvoiceLine struct {
    ID              string          `json:"id"`
    InvoiceID       string          `json:"invoice_id"`
    LineNumber      int             `json:"line_number"`
    ItemID          *string         `json:"item_id,omitempty"`
    Description     string          `json:"description"`
    Quantity        decimal.Decimal `json:"quantity"`
    Unit            string          `json:"unit"`
    UnitPrice       decimal.Decimal `json:"unit_price"`
    DiscountPercent decimal.Decimal `json:"discount_percent"`
    DiscountAmount  decimal.Decimal `json:"discount_amount"`
    LineNet         decimal.Decimal `json:"line_net"`
    VATRateID       string          `json:"vat_rate_id"`
    VATRate         decimal.Decimal `json:"vat_rate"`
    VATAmount       decimal.Decimal `json:"vat_amount"`
    LineTotal       decimal.Decimal `json:"line_total"`
    AccountID       *string         `json:"account_id,omitempty"`
}

// Calculate computes line totals
func (l *InvoiceLine) Calculate() {
    l.LineNet = l.Quantity.Mul(l.UnitPrice)

    if l.DiscountPercent.GreaterThan(decimal.Zero) {
        l.DiscountAmount = l.LineNet.Mul(l.DiscountPercent).Div(decimal.NewFromInt(100))
        l.LineNet = l.LineNet.Sub(l.DiscountAmount)
    }

    l.VATAmount = l.LineNet.Mul(l.VATRate).Div(decimal.NewFromInt(100))
    l.LineTotal = l.LineNet.Add(l.VATAmount)
}

// CalculateTotals aggregates line totals
func (inv *Invoice) CalculateTotals() {
    inv.Subtotal = decimal.Zero
    inv.TaxTotal = decimal.Zero

    for i := range inv.Lines {
        inv.Lines[i].Calculate()
        inv.Subtotal = inv.Subtotal.Add(inv.Lines[i].LineNet)
        inv.TaxTotal = inv.TaxTotal.Add(inv.Lines[i].VATAmount)
    }

    inv.Total = inv.Subtotal.Add(inv.TaxTotal)
    inv.BalanceDue = inv.Total
}

type InvoiceService struct {
    db             *pgxpool.Pool
    journalService *JournalService
}

func (s *InvoiceService) CreateInvoice(ctx context.Context, tenantID string, req *CreateInvoiceRequest) (*Invoice, error) {
    invoice := &Invoice{
        ID:          uuid.New().String(),
        TenantID:    tenantID,
        InvoiceType: req.InvoiceType,
        PartnerID:   req.PartnerID,
        InvoiceDate: req.InvoiceDate,
        DueDate:     req.DueDate,
        Currency:    req.Currency,
        ExchangeRate: req.ExchangeRate,
        Status:      InvoiceStatusDraft,
        Lines:       req.Lines,
        CreatedAt:   time.Now(),
    }

    invoice.CalculateTotals()

    // Generate invoice number
    prefix := "INV"
    if invoice.InvoiceType == InvoiceTypePurchase {
        prefix = "BILL"
    }

    // Insert invoice and lines...
    return invoice, nil
}

func (s *InvoiceService) PostInvoice(ctx context.Context, tenantID, invoiceID, userID string) error {
    invoice, err := s.GetByID(ctx, tenantID, invoiceID)
    if err != nil {
        return err
    }

    if invoice.Status != InvoiceStatusDraft {
        return errors.New("only draft invoices can be posted")
    }

    // Create journal entry
    journalReq := s.buildJournalEntry(invoice)
    entry, err := s.journalService.CreateEntry(ctx, tenantID, journalReq)
    if err != nil {
        return fmt.Errorf("create journal entry: %w", err)
    }

    // Post journal entry
    if err := s.journalService.PostEntry(ctx, tenantID, entry.ID, userID); err != nil {
        return fmt.Errorf("post journal entry: %w", err)
    }

    // Update invoice status
    _, err = s.db.Exec(ctx, `
        UPDATE invoices
        SET status = $1, journal_entry_id = $2
        WHERE id = $3 AND tenant_id = $4
    `, InvoiceStatusSent, entry.ID, invoiceID, tenantID)

    return err
}

func (s *InvoiceService) buildJournalEntry(inv *Invoice) *CreateJournalEntryRequest {
    lines := []JournalEntryLine{}

    if inv.InvoiceType == InvoiceTypeSales {
        // Debit: Accounts Receivable
        lines = append(lines, JournalEntryLine{
            AccountID:   s.getReceivableAccount(inv.TenantID),
            DebitAmount: inv.Total,
        })

        // Credit: Revenue accounts (per line)
        // Credit: VAT payable
        for _, line := range inv.Lines {
            lines = append(lines, JournalEntryLine{
                AccountID:    *line.AccountID,
                CreditAmount: line.LineNet,
            })
        }
        lines = append(lines, JournalEntryLine{
            AccountID:    s.getVATPayableAccount(inv.TenantID),
            CreditAmount: inv.TaxTotal,
        })
    }

    return &CreateJournalEntryRequest{
        EntryDate:   inv.InvoiceDate,
        Description: fmt.Sprintf("Invoice %s - %s", inv.InvoiceNumber, inv.Partner.Name),
        SourceType:  "INVOICE",
        SourceID:    inv.ID,
        Lines:       lines,
    }
}
```

### 2.3 Testing Requirements

```go
func TestInvoice_LineCalculation(t *testing.T) {
    // Test VAT calculation per line
    // Test discount application
    // Test quantity * unit price
}

func TestInvoice_JournalEntryGeneration(t *testing.T) {
    // Test sales invoice creates proper debits/credits
    // Test purchase invoice creates proper debits/credits
}

func TestInvoice_PaymentApplication(t *testing.T) {
    // Test partial payment updates balance
    // Test full payment changes status to PAID
}
```

---

## Section 3: VAT Management

### 3.1 Database Schema

```sql
-- VAT Rates with date ranges
CREATE TABLE vat_rates (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID NOT NULL REFERENCES tenants(id),
    country         CHAR(2) NOT NULL,       -- ISO country code
    category        VARCHAR(20) NOT NULL,   -- STANDARD, REDUCED, ZERO, EXEMPT
    name            VARCHAR(100) NOT NULL,
    rate            NUMERIC(5,2) NOT NULL,
    valid_from      DATE NOT NULL,
    valid_to        DATE,                   -- NULL means currently valid
    is_default      BOOLEAN DEFAULT false,
    account_id      UUID REFERENCES accounts(id),  -- VAT payable/receivable account
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    CONSTRAINT chk_valid_dates CHECK (valid_to IS NULL OR valid_to > valid_from)
);

CREATE INDEX idx_vat_rates_lookup ON vat_rates(tenant_id, country, category, valid_from);

-- VAT Returns
CREATE TABLE vat_returns (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID NOT NULL REFERENCES tenants(id),
    period_start    DATE NOT NULL,
    period_end      DATE NOT NULL,
    status          VARCHAR(20) DEFAULT 'DRAFT',  -- DRAFT, SUBMITTED, ACCEPTED
    sales_vat       NUMERIC(28,8) NOT NULL DEFAULT 0,
    purchase_vat    NUMERIC(28,8) NOT NULL DEFAULT 0,
    net_vat         NUMERIC(28,8) NOT NULL DEFAULT 0,  -- sales_vat - purchase_vat
    submitted_at    TIMESTAMPTZ,
    reference       VARCHAR(50),
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(tenant_id, period_start, period_end)
);

-- Pre-populated EU VAT rates (example data)
INSERT INTO vat_rates (tenant_id, country, category, name, rate, valid_from) VALUES
-- Estonia
(NULL, 'EE', 'STANDARD', 'Standard Rate', 22.00, '2024-01-01'),
(NULL, 'EE', 'STANDARD', 'Standard Rate (old)', 20.00, '2009-07-01'),
(NULL, 'EE', 'REDUCED', 'Reduced Rate', 9.00, '2009-01-01'),
(NULL, 'EE', 'ZERO', 'Zero Rate', 0.00, '2004-05-01'),
-- Germany
(NULL, 'DE', 'STANDARD', 'Regelsteuersatz', 19.00, '2021-01-01'),
(NULL, 'DE', 'REDUCED', 'Ermäßigter Satz', 7.00, '2021-01-01');
```

### 3.2 VAT Service (Go)

```go
package tax

import (
    "context"
    "time"

    "github.com/shopspring/decimal"
)

type VATRate struct {
    ID        string          `json:"id"`
    TenantID  string          `json:"tenant_id"`
    Country   string          `json:"country"`
    Category  string          `json:"category"`
    Name      string          `json:"name"`
    Rate      decimal.Decimal `json:"rate"`
    ValidFrom time.Time       `json:"valid_from"`
    ValidTo   *time.Time      `json:"valid_to,omitempty"`
    IsDefault bool            `json:"is_default"`
    AccountID *string         `json:"account_id,omitempty"`
}

// IsValidOn checks if rate is valid on a specific date
func (v *VATRate) IsValidOn(date time.Time) bool {
    if date.Before(v.ValidFrom) {
        return false
    }
    if v.ValidTo != nil && date.After(*v.ValidTo) {
        return false
    }
    return true
}

type VATService struct {
    db *pgxpool.Pool
}

// GetEffectiveRate returns the VAT rate valid on a specific date
func (s *VATService) GetEffectiveRate(ctx context.Context, tenantID, country, category string, date time.Time) (*VATRate, error) {
    var rate VATRate
    err := s.db.QueryRow(ctx, `
        SELECT id, tenant_id, country, category, name, rate, valid_from, valid_to, is_default, account_id
        FROM vat_rates
        WHERE (tenant_id = $1 OR tenant_id IS NULL)
          AND country = $2
          AND category = $3
          AND valid_from <= $4
          AND (valid_to IS NULL OR valid_to >= $4)
        ORDER BY tenant_id NULLS LAST, valid_from DESC
        LIMIT 1
    `, tenantID, country, category, date).Scan(
        &rate.ID, &rate.TenantID, &rate.Country, &rate.Category,
        &rate.Name, &rate.Rate, &rate.ValidFrom, &rate.ValidTo,
        &rate.IsDefault, &rate.AccountID,
    )

    if err != nil {
        return nil, fmt.Errorf("VAT rate not found for %s/%s on %s: %w",
            country, category, date.Format("2006-01-02"), err)
    }

    return &rate, nil
}

// GetRatesForCountry returns all current rates for a country
func (s *VATService) GetRatesForCountry(ctx context.Context, tenantID, country string) ([]VATRate, error) {
    rows, err := s.db.Query(ctx, `
        SELECT DISTINCT ON (category)
            id, tenant_id, country, category, name, rate, valid_from, valid_to, is_default, account_id
        FROM vat_rates
        WHERE (tenant_id = $1 OR tenant_id IS NULL)
          AND country = $2
          AND valid_from <= CURRENT_DATE
          AND (valid_to IS NULL OR valid_to >= CURRENT_DATE)
        ORDER BY category, tenant_id NULLS LAST, valid_from DESC
    `, tenantID, country)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var rates []VATRate
    for rows.Next() {
        var r VATRate
        if err := rows.Scan(&r.ID, &r.TenantID, &r.Country, &r.Category,
            &r.Name, &r.Rate, &r.ValidFrom, &r.ValidTo, &r.IsDefault, &r.AccountID); err != nil {
            return nil, err
        }
        rates = append(rates, r)
    }

    return rates, nil
}

// CalculateVATReturn generates VAT return for a period
func (s *VATService) CalculateVATReturn(ctx context.Context, tenantID string, periodStart, periodEnd time.Time) (*VATReturn, error) {
    var salesVAT, purchaseVAT decimal.Decimal

    // Sum VAT from sales invoices
    err := s.db.QueryRow(ctx, `
        SELECT COALESCE(SUM(tax_total), 0)
        FROM invoices
        WHERE tenant_id = $1
          AND invoice_type = 'SALES'
          AND status IN ('SENT', 'PARTIALLY_PAID', 'PAID')
          AND invoice_date BETWEEN $2 AND $3
    `, tenantID, periodStart, periodEnd).Scan(&salesVAT)
    if err != nil {
        return nil, err
    }

    // Sum VAT from purchase invoices
    err = s.db.QueryRow(ctx, `
        SELECT COALESCE(SUM(tax_total), 0)
        FROM invoices
        WHERE tenant_id = $1
          AND invoice_type = 'PURCHASE'
          AND status IN ('SENT', 'PARTIALLY_PAID', 'PAID')
          AND invoice_date BETWEEN $2 AND $3
    `, tenantID, periodStart, periodEnd).Scan(&purchaseVAT)
    if err != nil {
        return nil, err
    }

    return &VATReturn{
        TenantID:    tenantID,
        PeriodStart: periodStart,
        PeriodEnd:   periodEnd,
        Status:      "DRAFT",
        SalesVAT:    salesVAT,
        PurchaseVAT: purchaseVAT,
        NetVAT:      salesVAT.Sub(purchaseVAT),
    }, nil
}
```

### 3.3 Key Design Decisions

| Decision | Rationale |
|----------|-----------|
| Date-range validity | VAT rates change on specific government dates, not overnight |
| Tenant override | Tenants can define custom rates that override system defaults |
| Historical accuracy | Invoice VAT is determined by invoice date, not current date |
| NULL valid_to | Currently active rates have no end date |

---

## Section 4: Payments

### 4.1 Database Schema

```sql
-- Bank Accounts
CREATE TABLE bank_accounts (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID NOT NULL REFERENCES tenants(id),
    name            VARCHAR(100) NOT NULL,
    account_number  VARCHAR(50),
    iban            VARCHAR(34),
    bic             VARCHAR(11),
    currency        CHAR(3) DEFAULT 'EUR',
    account_id      UUID NOT NULL REFERENCES accounts(id),  -- GL account
    is_default      BOOLEAN DEFAULT false,
    is_active       BOOLEAN DEFAULT true,
    balance         NUMERIC(28,8) DEFAULT 0,
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(tenant_id, iban)
);

-- Payments
CREATE TABLE payments (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID NOT NULL REFERENCES tenants(id),
    payment_number  VARCHAR(20) NOT NULL,
    payment_date    DATE NOT NULL,
    payment_type    VARCHAR(10) NOT NULL,  -- INCOMING, OUTGOING
    partner_id      UUID REFERENCES partners(id),
    bank_account_id UUID NOT NULL REFERENCES bank_accounts(id),
    amount          NUMERIC(28,8) NOT NULL,
    currency        CHAR(3) NOT NULL DEFAULT 'EUR',
    exchange_rate   NUMERIC(18,10) DEFAULT 1,
    reference       VARCHAR(100),
    description     TEXT,
    status          VARCHAR(20) DEFAULT 'DRAFT',
    journal_entry_id UUID REFERENCES journal_entries(id),
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(tenant_id, payment_number)
);

CREATE INDEX idx_payments_tenant_date ON payments(tenant_id, payment_date);
CREATE INDEX idx_payments_partner ON payments(partner_id);

-- Payment Allocations (link payments to invoices)
CREATE TABLE payment_allocations (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID NOT NULL REFERENCES tenants(id),
    payment_id      UUID NOT NULL REFERENCES payments(id),
    invoice_id      UUID NOT NULL REFERENCES invoices(id),
    allocated_amount NUMERIC(28,8) NOT NULL,
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(payment_id, invoice_id)
);

CREATE INDEX idx_allocations_invoice ON payment_allocations(invoice_id);
```

### 4.2 Payment Service (Go)

```go
package payments

import (
    "context"
    "time"

    "github.com/shopspring/decimal"
)

type PaymentType string

const (
    PaymentTypeIncoming PaymentType = "INCOMING"
    PaymentTypeOutgoing PaymentType = "OUTGOING"
)

type Payment struct {
    ID            string          `json:"id"`
    TenantID      string          `json:"tenant_id"`
    PaymentNumber string          `json:"payment_number"`
    PaymentDate   time.Time       `json:"payment_date"`
    PaymentType   PaymentType     `json:"payment_type"`
    PartnerID     *string         `json:"partner_id,omitempty"`
    BankAccountID string          `json:"bank_account_id"`
    Amount        decimal.Decimal `json:"amount"`
    Currency      string          `json:"currency"`
    ExchangeRate  decimal.Decimal `json:"exchange_rate"`
    Reference     string          `json:"reference,omitempty"`
    Description   string          `json:"description,omitempty"`
    Status        string          `json:"status"`
    Allocations   []PaymentAllocation `json:"allocations,omitempty"`
}

type PaymentAllocation struct {
    ID              string          `json:"id"`
    PaymentID       string          `json:"payment_id"`
    InvoiceID       string          `json:"invoice_id"`
    InvoiceNumber   string          `json:"invoice_number,omitempty"`
    AllocatedAmount decimal.Decimal `json:"allocated_amount"`
}

type PaymentService struct {
    db             *pgxpool.Pool
    journalService *JournalService
    invoiceService *InvoiceService
}

func (s *PaymentService) CreatePayment(ctx context.Context, tenantID string, req *CreatePaymentRequest) (*Payment, error) {
    payment := &Payment{
        ID:            uuid.New().String(),
        TenantID:      tenantID,
        PaymentDate:   req.PaymentDate,
        PaymentType:   req.PaymentType,
        PartnerID:     req.PartnerID,
        BankAccountID: req.BankAccountID,
        Amount:        req.Amount,
        Currency:      req.Currency,
        ExchangeRate:  req.ExchangeRate,
        Reference:     req.Reference,
        Description:   req.Description,
        Status:        "DRAFT",
    }

    // Validate allocations don't exceed payment amount
    totalAllocated := decimal.Zero
    for _, alloc := range req.Allocations {
        totalAllocated = totalAllocated.Add(alloc.Amount)
    }
    if totalAllocated.GreaterThan(payment.Amount) {
        return nil, errors.New("allocations exceed payment amount")
    }

    // Insert payment and allocations...
    return payment, nil
}

func (s *PaymentService) PostPayment(ctx context.Context, tenantID, paymentID, userID string) error {
    tx, err := s.db.Begin(ctx)
    if err != nil {
        return err
    }
    defer tx.Rollback(ctx)

    payment, err := s.getByID(ctx, tx, tenantID, paymentID)
    if err != nil {
        return err
    }

    // Create journal entry
    journalReq := s.buildJournalEntry(payment)
    entry, err := s.journalService.CreateEntryTx(ctx, tx, tenantID, journalReq)
    if err != nil {
        return err
    }

    // Post journal entry
    if err := s.journalService.PostEntryTx(ctx, tx, tenantID, entry.ID, userID); err != nil {
        return err
    }

    // Update invoice balances
    for _, alloc := range payment.Allocations {
        if err := s.invoiceService.ApplyPaymentTx(ctx, tx, tenantID, alloc.InvoiceID, alloc.AllocatedAmount); err != nil {
            return err
        }
    }

    // Update payment status
    _, err = tx.Exec(ctx, `
        UPDATE payments
        SET status = 'POSTED', journal_entry_id = $1
        WHERE id = $2 AND tenant_id = $3
    `, entry.ID, paymentID, tenantID)
    if err != nil {
        return err
    }

    return tx.Commit(ctx)
}

func (s *PaymentService) buildJournalEntry(p *Payment) *CreateJournalEntryRequest {
    lines := []JournalEntryLine{}
    bankAccount := s.getBankGLAccount(p.BankAccountID)

    if p.PaymentType == PaymentTypeIncoming {
        // Debit: Bank
        lines = append(lines, JournalEntryLine{
            AccountID:   bankAccount,
            DebitAmount: p.Amount,
        })
        // Credit: Accounts Receivable (or per allocation)
        lines = append(lines, JournalEntryLine{
            AccountID:    s.getReceivableAccount(p.TenantID),
            CreditAmount: p.Amount,
        })
    } else {
        // Credit: Bank
        lines = append(lines, JournalEntryLine{
            AccountID:    bankAccount,
            CreditAmount: p.Amount,
        })
        // Debit: Accounts Payable
        lines = append(lines, JournalEntryLine{
            AccountID:   s.getPayableAccount(p.TenantID),
            DebitAmount: p.Amount,
        })
    }

    return &CreateJournalEntryRequest{
        EntryDate:   p.PaymentDate,
        Description: fmt.Sprintf("Payment %s", p.PaymentNumber),
        SourceType:  "PAYMENT",
        SourceID:    p.ID,
        Lines:       lines,
    }
}
```

### 4.3 Testing Requirements

```go
func TestPayment_AllocationValidation(t *testing.T) {
    // Test allocations cannot exceed payment amount
    // Test allocations cannot exceed invoice balance
}

func TestPayment_InvoiceBalanceUpdate(t *testing.T) {
    // Test partial payment reduces balance
    // Test full payment marks invoice as PAID
}

func TestPayment_JournalEntryGeneration(t *testing.T) {
    // Test incoming payment debits bank, credits receivable
    // Test outgoing payment credits bank, debits payable
}
```

---

## Section 5: Inventory

### 5.1 Database Schema

```sql
-- Warehouse Locations
CREATE TABLE warehouses (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID NOT NULL REFERENCES tenants(id),
    code            VARCHAR(20) NOT NULL,
    name            VARCHAR(100) NOT NULL,
    address         JSONB,
    is_default      BOOLEAN DEFAULT false,
    is_active       BOOLEAN DEFAULT true,
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(tenant_id, code)
);

-- Inventory Lots (for FIFO costing)
CREATE TABLE inventory_lots (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID NOT NULL REFERENCES tenants(id),
    item_id         UUID NOT NULL REFERENCES items(id),
    warehouse_id    UUID NOT NULL REFERENCES warehouses(id),
    lot_number      VARCHAR(50),
    receipt_date    DATE NOT NULL,
    quantity        NUMERIC(28,8) NOT NULL,
    remaining_qty   NUMERIC(28,8) NOT NULL,
    unit_cost       NUMERIC(28,8) NOT NULL,
    source_type     VARCHAR(20),            -- PURCHASE, ADJUSTMENT, TRANSFER
    source_id       UUID,
    expiry_date     DATE,
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    CONSTRAINT chk_remaining CHECK (remaining_qty >= 0 AND remaining_qty <= quantity)
);

CREATE INDEX idx_lots_item ON inventory_lots(item_id, warehouse_id);
CREATE INDEX idx_lots_fifo ON inventory_lots(item_id, warehouse_id, receipt_date, id);

-- Inventory Movements
CREATE TABLE inventory_movements (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID NOT NULL REFERENCES tenants(id),
    movement_number VARCHAR(20) NOT NULL,
    movement_date   TIMESTAMPTZ NOT NULL,
    movement_type   VARCHAR(20) NOT NULL,   -- RECEIPT, ISSUE, TRANSFER, ADJUSTMENT
    item_id         UUID NOT NULL REFERENCES items(id),
    from_warehouse_id UUID REFERENCES warehouses(id),
    to_warehouse_id UUID REFERENCES warehouses(id),
    quantity        NUMERIC(28,8) NOT NULL,
    unit_cost       NUMERIC(28,8),
    total_cost      NUMERIC(28,8),
    lot_id          UUID REFERENCES inventory_lots(id),
    source_type     VARCHAR(20),
    source_id       UUID,
    notes           TEXT,
    journal_entry_id UUID REFERENCES journal_entries(id),
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    created_by      UUID REFERENCES users(id),
    UNIQUE(tenant_id, movement_number)
);

CREATE INDEX idx_movements_item ON inventory_movements(item_id);
CREATE INDEX idx_movements_date ON inventory_movements(tenant_id, movement_date);

-- Current Stock View
CREATE VIEW current_stock AS
SELECT
    tenant_id,
    item_id,
    warehouse_id,
    SUM(remaining_qty) AS quantity_on_hand,
    SUM(remaining_qty * unit_cost) AS total_value,
    CASE WHEN SUM(remaining_qty) > 0
         THEN SUM(remaining_qty * unit_cost) / SUM(remaining_qty)
         ELSE 0
    END AS avg_unit_cost
FROM inventory_lots
WHERE remaining_qty > 0
GROUP BY tenant_id, item_id, warehouse_id;
```

### 5.2 Inventory Service (Go)

```go
package inventory

import (
    "context"
    "time"

    "github.com/shopspring/decimal"
)

type MovementType string

const (
    MovementTypeReceipt    MovementType = "RECEIPT"
    MovementTypeIssue      MovementType = "ISSUE"
    MovementTypeTransfer   MovementType = "TRANSFER"
    MovementTypeAdjustment MovementType = "ADJUSTMENT"
)

type InventoryLot struct {
    ID           string          `json:"id"`
    TenantID     string          `json:"tenant_id"`
    ItemID       string          `json:"item_id"`
    WarehouseID  string          `json:"warehouse_id"`
    LotNumber    string          `json:"lot_number,omitempty"`
    ReceiptDate  time.Time       `json:"receipt_date"`
    Quantity     decimal.Decimal `json:"quantity"`
    RemainingQty decimal.Decimal `json:"remaining_qty"`
    UnitCost     decimal.Decimal `json:"unit_cost"`
    ExpiryDate   *time.Time      `json:"expiry_date,omitempty"`
}

type InventoryMovement struct {
    ID              string          `json:"id"`
    TenantID        string          `json:"tenant_id"`
    MovementNumber  string          `json:"movement_number"`
    MovementDate    time.Time       `json:"movement_date"`
    MovementType    MovementType    `json:"movement_type"`
    ItemID          string          `json:"item_id"`
    FromWarehouseID *string         `json:"from_warehouse_id,omitempty"`
    ToWarehouseID   *string         `json:"to_warehouse_id,omitempty"`
    Quantity        decimal.Decimal `json:"quantity"`
    UnitCost        decimal.Decimal `json:"unit_cost"`
    TotalCost       decimal.Decimal `json:"total_cost"`
    LotID           *string         `json:"lot_id,omitempty"`
}

type InventoryService struct {
    db             *pgxpool.Pool
    journalService *JournalService
}

// ReceiveStock creates a new inventory lot (FIFO entry point)
func (s *InventoryService) ReceiveStock(ctx context.Context, tenantID string, req *ReceiveStockRequest) (*InventoryMovement, error) {
    tx, err := s.db.Begin(ctx)
    if err != nil {
        return nil, err
    }
    defer tx.Rollback(ctx)

    // Create lot
    lot := &InventoryLot{
        ID:           uuid.New().String(),
        TenantID:     tenantID,
        ItemID:       req.ItemID,
        WarehouseID:  req.WarehouseID,
        LotNumber:    req.LotNumber,
        ReceiptDate:  req.ReceiptDate,
        Quantity:     req.Quantity,
        RemainingQty: req.Quantity,
        UnitCost:     req.UnitCost,
        ExpiryDate:   req.ExpiryDate,
    }

    _, err = tx.Exec(ctx, `
        INSERT INTO inventory_lots (id, tenant_id, item_id, warehouse_id, lot_number, receipt_date, quantity, remaining_qty, unit_cost, source_type, source_id, expiry_date)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
    `, lot.ID, lot.TenantID, lot.ItemID, lot.WarehouseID, lot.LotNumber,
       lot.ReceiptDate, lot.Quantity, lot.RemainingQty, lot.UnitCost,
       req.SourceType, req.SourceID, lot.ExpiryDate)
    if err != nil {
        return nil, err
    }

    // Create movement record
    movement := &InventoryMovement{
        ID:            uuid.New().String(),
        TenantID:      tenantID,
        MovementDate:  req.ReceiptDate,
        MovementType:  MovementTypeReceipt,
        ItemID:        req.ItemID,
        ToWarehouseID: &req.WarehouseID,
        Quantity:      req.Quantity,
        UnitCost:      req.UnitCost,
        TotalCost:     req.Quantity.Mul(req.UnitCost),
        LotID:         &lot.ID,
    }

    // Generate movement number and insert...

    // Create journal entry (Debit Inventory, Credit AP or Cash)
    journalReq := s.buildReceiptJournalEntry(movement, req.ItemID)
    entry, _ := s.journalService.CreateEntryTx(ctx, tx, tenantID, journalReq)
    s.journalService.PostEntryTx(ctx, tx, tenantID, entry.ID, req.UserID)

    return movement, tx.Commit(ctx)
}

// IssueStock removes stock using FIFO costing
func (s *InventoryService) IssueStock(ctx context.Context, tenantID string, req *IssueStockRequest) (*InventoryMovement, decimal.Decimal, error) {
    tx, err := s.db.Begin(ctx)
    if err != nil {
        return nil, decimal.Zero, err
    }
    defer tx.Rollback(ctx)

    // Get lots in FIFO order
    rows, err := tx.Query(ctx, `
        SELECT id, remaining_qty, unit_cost
        FROM inventory_lots
        WHERE tenant_id = $1 AND item_id = $2 AND warehouse_id = $3 AND remaining_qty > 0
        ORDER BY receipt_date ASC, id ASC
        FOR UPDATE
    `, tenantID, req.ItemID, req.WarehouseID)
    if err != nil {
        return nil, decimal.Zero, err
    }
    defer rows.Close()

    remainingToIssue := req.Quantity
    totalCost := decimal.Zero
    lotConsumptions := []struct {
        LotID    string
        Qty      decimal.Decimal
        UnitCost decimal.Decimal
    }{}

    for rows.Next() && remainingToIssue.GreaterThan(decimal.Zero) {
        var lotID string
        var remainingQty, unitCost decimal.Decimal
        if err := rows.Scan(&lotID, &remainingQty, &unitCost); err != nil {
            return nil, decimal.Zero, err
        }

        issueFromLot := decimal.Min(remainingQty, remainingToIssue)
        lotConsumptions = append(lotConsumptions, struct {
            LotID    string
            Qty      decimal.Decimal
            UnitCost decimal.Decimal
        }{lotID, issueFromLot, unitCost})

        totalCost = totalCost.Add(issueFromLot.Mul(unitCost))
        remainingToIssue = remainingToIssue.Sub(issueFromLot)
    }

    if remainingToIssue.GreaterThan(decimal.Zero) {
        return nil, decimal.Zero, fmt.Errorf("insufficient stock: need %s more units", remainingToIssue)
    }

    // Update lot quantities
    for _, lc := range lotConsumptions {
        _, err = tx.Exec(ctx, `
            UPDATE inventory_lots
            SET remaining_qty = remaining_qty - $1
            WHERE id = $2
        `, lc.Qty, lc.LotID)
        if err != nil {
            return nil, decimal.Zero, err
        }
    }

    // Calculate weighted average cost for this issue
    avgCost := totalCost.Div(req.Quantity)

    // Create movement record
    movement := &InventoryMovement{
        ID:              uuid.New().String(),
        TenantID:        tenantID,
        MovementDate:    time.Now(),
        MovementType:    MovementTypeIssue,
        ItemID:          req.ItemID,
        FromWarehouseID: &req.WarehouseID,
        Quantity:        req.Quantity,
        UnitCost:        avgCost,
        TotalCost:       totalCost,
    }

    // Create journal entry (Debit COGS, Credit Inventory)
    journalReq := s.buildIssueJournalEntry(movement, req.ItemID, totalCost)
    entry, _ := s.journalService.CreateEntryTx(ctx, tx, tenantID, journalReq)
    s.journalService.PostEntryTx(ctx, tx, tenantID, entry.ID, req.UserID)

    if err := tx.Commit(ctx); err != nil {
        return nil, decimal.Zero, err
    }

    return movement, totalCost, nil
}

// GetStockLevel returns current stock for an item
func (s *InventoryService) GetStockLevel(ctx context.Context, tenantID, itemID, warehouseID string) (*StockLevel, error) {
    var level StockLevel
    err := s.db.QueryRow(ctx, `
        SELECT
            item_id, warehouse_id,
            COALESCE(SUM(remaining_qty), 0) AS quantity_on_hand,
            COALESCE(SUM(remaining_qty * unit_cost), 0) AS total_value
        FROM inventory_lots
        WHERE tenant_id = $1 AND item_id = $2 AND warehouse_id = $3 AND remaining_qty > 0
        GROUP BY item_id, warehouse_id
    `, tenantID, itemID, warehouseID).Scan(
        &level.ItemID, &level.WarehouseID, &level.QuantityOnHand, &level.TotalValue,
    )

    if err == pgx.ErrNoRows {
        return &StockLevel{ItemID: itemID, WarehouseID: warehouseID}, nil
    }

    return &level, err
}
```

### 5.3 Key Features

| Feature | Description |
|---------|-------------|
| FIFO Costing | First-in-first-out inventory valuation |
| Lot Tracking | Track individual receipt lots with optional expiry |
| Multi-warehouse | Support for multiple storage locations |
| Automatic COGS | Cost of goods sold calculated on issue |
| Journal Integration | All movements create GL entries |

---

## Section 6: Payroll

### 6.1 Database Schema

```sql
-- Employees
CREATE TABLE employees (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID NOT NULL REFERENCES tenants(id),
    employee_number VARCHAR(20) NOT NULL,
    first_name      VARCHAR(100) NOT NULL,
    last_name       VARCHAR(100) NOT NULL,
    personal_code   VARCHAR(20),            -- Estonian isikukood
    email           VARCHAR(255),
    phone           VARCHAR(50),
    address         JSONB,
    bank_account    VARCHAR(34),            -- IBAN
    hire_date       DATE NOT NULL,
    termination_date DATE,
    department_id   UUID,
    position        VARCHAR(100),
    is_active       BOOLEAN DEFAULT true,
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(tenant_id, employee_number),
    UNIQUE(tenant_id, personal_code)
);

-- Salary Agreements
CREATE TABLE salary_agreements (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID NOT NULL REFERENCES tenants(id),
    employee_id     UUID NOT NULL REFERENCES employees(id),
    salary_type     VARCHAR(20) NOT NULL,   -- MONTHLY, HOURLY
    gross_amount    NUMERIC(28,8) NOT NULL,
    currency        CHAR(3) DEFAULT 'EUR',
    valid_from      DATE NOT NULL,
    valid_to        DATE,
    has_pension_ii  BOOLEAN DEFAULT true,   -- II pillar (2%)
    tax_free_income NUMERIC(28,8) DEFAULT 654,  -- Monthly tax-free amount
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    CONSTRAINT chk_valid_salary_dates CHECK (valid_to IS NULL OR valid_to > valid_from)
);

CREATE INDEX idx_salary_employee ON salary_agreements(employee_id);

-- Payroll Runs
CREATE TABLE payroll_runs (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID NOT NULL REFERENCES tenants(id),
    run_number      VARCHAR(20) NOT NULL,
    period_start    DATE NOT NULL,
    period_end      DATE NOT NULL,
    payment_date    DATE NOT NULL,
    status          VARCHAR(20) DEFAULT 'DRAFT',
    total_gross     NUMERIC(28,8) DEFAULT 0,
    total_net       NUMERIC(28,8) DEFAULT 0,
    total_employer_cost NUMERIC(28,8) DEFAULT 0,
    journal_entry_id UUID REFERENCES journal_entries(id),
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    created_by      UUID REFERENCES users(id),
    UNIQUE(tenant_id, run_number)
);

-- Payslips
CREATE TABLE payslips (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID NOT NULL REFERENCES tenants(id),
    payroll_run_id  UUID NOT NULL REFERENCES payroll_runs(id),
    employee_id     UUID NOT NULL REFERENCES employees(id),

    -- Gross pay
    gross_salary    NUMERIC(28,8) NOT NULL,

    -- Employee deductions
    pension_ii      NUMERIC(28,8) DEFAULT 0,  -- 2% employee contribution
    unemployment_ee NUMERIC(28,8) DEFAULT 0,  -- 1.6% employee
    income_tax      NUMERIC(28,8) DEFAULT 0,  -- 20% (after deductions)
    other_deductions NUMERIC(28,8) DEFAULT 0,

    -- Net pay
    net_salary      NUMERIC(28,8) NOT NULL,

    -- Employer contributions
    social_tax      NUMERIC(28,8) DEFAULT 0,  -- 33%
    unemployment_er NUMERIC(28,8) DEFAULT 0,  -- 0.8% employer

    -- Totals
    total_employer_cost NUMERIC(28,8) NOT NULL,

    -- Metadata
    calculation_details JSONB,
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(payroll_run_id, employee_id)
);

CREATE INDEX idx_payslips_employee ON payslips(employee_id);
```

### 6.2 Estonian Payroll Calculator (Go)

```go
package payroll

import (
    "github.com/shopspring/decimal"
)

// Estonian tax rates (2024)
var (
    SocialTaxRate       = decimal.NewFromFloat(0.33)   // 33% employer
    UnemploymentEERate  = decimal.NewFromFloat(0.016)  // 1.6% employee
    UnemploymentERRate  = decimal.NewFromFloat(0.008)  // 0.8% employer
    PensionIIRate       = decimal.NewFromFloat(0.02)   // 2% employee (if opted in)
    IncomeTaxRate       = decimal.NewFromFloat(0.20)   // 20% flat rate

    // Progressive tax-free income thresholds (2024)
    TaxFreeMax          = decimal.NewFromInt(654)      // Max monthly tax-free
    TaxFreePhaseOutStart = decimal.NewFromInt(1200)    // Start reducing
    TaxFreePhaseOutEnd   = decimal.NewFromInt(2100)    // Fully phased out
)

type PayrollInput struct {
    GrossSalary   decimal.Decimal
    HasPensionII  bool
    TaxFreeIncome decimal.Decimal  // Can be customized per employee
}

type PayrollResult struct {
    GrossSalary          decimal.Decimal `json:"gross_salary"`

    // Employee deductions
    PensionII            decimal.Decimal `json:"pension_ii"`
    UnemploymentEmployee decimal.Decimal `json:"unemployment_employee"`
    IncomeTax            decimal.Decimal `json:"income_tax"`
    TotalDeductions      decimal.Decimal `json:"total_deductions"`

    // Net pay
    NetSalary            decimal.Decimal `json:"net_salary"`

    // Employer costs
    SocialTax            decimal.Decimal `json:"social_tax"`
    UnemploymentEmployer decimal.Decimal `json:"unemployment_employer"`
    TotalEmployerCost    decimal.Decimal `json:"total_employer_cost"`
}

type EstonianPayrollCalculator struct{}

func NewEstonianPayrollCalculator() *EstonianPayrollCalculator {
    return &EstonianPayrollCalculator{}
}

func (c *EstonianPayrollCalculator) Calculate(input PayrollInput) PayrollResult {
    gross := input.GrossSalary
    result := PayrollResult{GrossSalary: gross}

    // 1. Unemployment insurance (employee portion)
    result.UnemploymentEmployee = gross.Mul(UnemploymentEERate).Round(2)

    // 2. Funded pension II (if applicable)
    if input.HasPensionII {
        result.PensionII = gross.Mul(PensionIIRate).Round(2)
    }

    // 3. Calculate taxable income
    taxableIncome := gross.Sub(result.UnemploymentEmployee).Sub(result.PensionII)

    // 4. Apply tax-free income
    taxFree := input.TaxFreeIncome
    if taxFree.GreaterThan(TaxFreeMax) {
        taxFree = TaxFreeMax
    }
    taxableIncome = taxableIncome.Sub(taxFree)
    if taxableIncome.LessThan(decimal.Zero) {
        taxableIncome = decimal.Zero
    }

    // 5. Income tax
    result.IncomeTax = taxableIncome.Mul(IncomeTaxRate).Round(2)

    // 6. Total deductions and net salary
    result.TotalDeductions = result.UnemploymentEmployee.Add(result.PensionII).Add(result.IncomeTax)
    result.NetSalary = gross.Sub(result.TotalDeductions)

    // 7. Employer costs
    result.SocialTax = gross.Mul(SocialTaxRate).Round(2)
    result.UnemploymentEmployer = gross.Mul(UnemploymentERRate).Round(2)
    result.TotalEmployerCost = gross.Add(result.SocialTax).Add(result.UnemploymentEmployer)

    return result
}

// CalculateProgressiveTaxFree calculates reduced tax-free income for high earners
func (c *EstonianPayrollCalculator) CalculateProgressiveTaxFree(annualIncome decimal.Decimal) decimal.Decimal {
    monthlyIncome := annualIncome.Div(decimal.NewFromInt(12))

    if monthlyIncome.LessThanOrEqual(TaxFreePhaseOutStart) {
        return TaxFreeMax
    }

    if monthlyIncome.GreaterThanOrEqual(TaxFreePhaseOutEnd) {
        return decimal.Zero
    }

    // Linear phase-out
    range_ := TaxFreePhaseOutEnd.Sub(TaxFreePhaseOutStart)
    excess := monthlyIncome.Sub(TaxFreePhaseOutStart)
    reduction := TaxFreeMax.Mul(excess).Div(range_)

    return TaxFreeMax.Sub(reduction).Round(2)
}
```

### 6.3 Payroll Service

```go
package payroll

import (
    "context"
    "time"
)

type PayrollService struct {
    db         *pgxpool.Pool
    calculator *EstonianPayrollCalculator
    journal    *JournalService
}

func (s *PayrollService) CreatePayrollRun(ctx context.Context, tenantID string, req *CreatePayrollRunRequest) (*PayrollRun, error) {
    run := &PayrollRun{
        ID:          uuid.New().String(),
        TenantID:    tenantID,
        PeriodStart: req.PeriodStart,
        PeriodEnd:   req.PeriodEnd,
        PaymentDate: req.PaymentDate,
        Status:      "DRAFT",
    }

    // Get active employees with salary agreements
    employees, err := s.getActiveEmployeesWithSalary(ctx, tenantID, req.PeriodEnd)
    if err != nil {
        return nil, err
    }

    // Calculate payslips
    for _, emp := range employees {
        input := PayrollInput{
            GrossSalary:   emp.GrossSalary,
            HasPensionII:  emp.HasPensionII,
            TaxFreeIncome: emp.TaxFreeIncome,
        }

        result := s.calculator.Calculate(input)

        payslip := &Payslip{
            ID:                 uuid.New().String(),
            PayrollRunID:       run.ID,
            EmployeeID:         emp.ID,
            GrossSalary:        result.GrossSalary,
            PensionII:          result.PensionII,
            UnemploymentEE:     result.UnemploymentEmployee,
            IncomeTax:          result.IncomeTax,
            NetSalary:          result.NetSalary,
            SocialTax:          result.SocialTax,
            UnemploymentER:     result.UnemploymentEmployer,
            TotalEmployerCost:  result.TotalEmployerCost,
        }

        run.Payslips = append(run.Payslips, payslip)
        run.TotalGross = run.TotalGross.Add(result.GrossSalary)
        run.TotalNet = run.TotalNet.Add(result.NetSalary)
        run.TotalEmployerCost = run.TotalEmployerCost.Add(result.TotalEmployerCost)
    }

    // Save to database...
    return run, nil
}

func (s *PayrollService) PostPayrollRun(ctx context.Context, tenantID, runID, userID string) error {
    run, err := s.getByID(ctx, tenantID, runID)
    if err != nil {
        return err
    }

    // Create journal entry
    // Debit: Salary Expense (gross)
    // Debit: Social Tax Expense
    // Debit: Unemployment Employer Expense
    // Credit: Salary Payable (net)
    // Credit: Tax Payable (income tax + social tax)
    // Credit: Pension Payable
    // Credit: Unemployment Payable

    lines := []JournalEntryLine{
        {AccountID: s.getSalaryExpenseAccount(), DebitAmount: run.TotalGross},
        {AccountID: s.getSocialTaxExpenseAccount(), DebitAmount: s.sumSocialTax(run)},
        {AccountID: s.getUnemploymentExpenseAccount(), DebitAmount: s.sumUnemploymentER(run)},
        {AccountID: s.getSalaryPayableAccount(), CreditAmount: run.TotalNet},
        {AccountID: s.getTaxPayableAccount(), CreditAmount: s.sumTaxes(run)},
        // ... more lines for various payables
    }

    // Create and post journal entry...
    return nil
}
```

### 6.4 Key Features

| Feature | Description |
|---------|-------------|
| Estonian Tax Compliance | Social tax (33%), income tax (20%), unemployment |
| Progressive Tax-Free | Reduced tax-free income for high earners |
| Pension II Support | Optional 2% employee contribution |
| Batch Processing | Process all employees in single payroll run |
| Journal Integration | Automatic GL entries for payroll expenses |

---

## Section 7: E-Invoicing

### 7.1 Database Schema

```sql
-- E-invoice operators
CREATE TABLE einvoice_operators (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID NOT NULL REFERENCES tenants(id),
    code            VARCHAR(50) NOT NULL,   -- 'BILLBERRY', 'FINBITE', 'TELEMA'
    name            VARCHAR(255) NOT NULL,
    api_endpoint    VARCHAR(500),
    api_key_encrypted BYTEA,
    is_active       BOOLEAN DEFAULT true,
    is_default      BOOLEAN DEFAULT false,
    UNIQUE(tenant_id, code)
);

-- Partner e-invoice settings
CREATE TABLE partner_einvoice_settings (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID NOT NULL REFERENCES tenants(id),
    partner_id      UUID NOT NULL REFERENCES partners(id),
    accepts_einvoice BOOLEAN DEFAULT false,
    preferred_format VARCHAR(20) DEFAULT 'EE_1.2',  -- EE_1.2, PEPPOL_BIS_3
    operator_id     UUID REFERENCES einvoice_operators(id),
    einvoice_address VARCHAR(100),
    UNIQUE(tenant_id, partner_id)
);

-- E-invoice transmissions
CREATE TABLE einvoice_transmissions (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID NOT NULL REFERENCES tenants(id),
    invoice_id      UUID NOT NULL REFERENCES invoices(id),
    direction       VARCHAR(10) NOT NULL,   -- 'OUTBOUND', 'INBOUND'
    format          VARCHAR(20) NOT NULL,   -- 'EE_1.2', 'PEPPOL_BIS_3'
    operator_id     UUID REFERENCES einvoice_operators(id),
    file_id         VARCHAR(50),
    transmission_id VARCHAR(100),
    status          VARCHAR(20) NOT NULL DEFAULT 'PENDING',
    xml_content     TEXT NOT NULL,
    xml_hash        VARCHAR(64),
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    sent_at         TIMESTAMPTZ,
    delivered_at    TIMESTAMPTZ,
    error_message   TEXT,
    retry_count     INTEGER DEFAULT 0
);

CREATE INDEX idx_einvoice_invoice ON einvoice_transmissions(invoice_id);
CREATE INDEX idx_einvoice_status ON einvoice_transmissions(tenant_id, status);

-- E-invoice registry cache
CREATE TABLE einvoice_registry_cache (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    reg_code        VARCHAR(20) NOT NULL UNIQUE,
    company_name    VARCHAR(255),
    accepts_einvoice BOOLEAN DEFAULT false,
    operator_code   VARCHAR(50),
    einvoice_address VARCHAR(100),
    last_checked_at TIMESTAMPTZ DEFAULT NOW(),
    expires_at      TIMESTAMPTZ DEFAULT NOW() + INTERVAL '24 hours'
);
```

### 7.2 Estonian E-Invoice XML Generator (Go)

```go
package einvoice

import (
    "encoding/xml"
    "time"

    "github.com/shopspring/decimal"
)

// Estonian e-invoice v1.2 XML structure
type EInvoice struct {
    XMLName xml.Name      `xml:"E_Invoice"`
    Header  Header        `xml:"Header"`
    Invoice []InvoiceDoc  `xml:"Invoice"`
    Footer  Footer        `xml:"Footer"`
}

type Header struct {
    Date           string `xml:"Date"`
    FileId         string `xml:"FileId"`
    Version        string `xml:"Version"`
    SenderId       string `xml:"SenderId"`
    ReceiverId     string `xml:"ReceiverId"`
    ContractId     string `xml:"ContractId,omitempty"`
    PaymentId      string `xml:"PaymentId,omitempty"`
}

type InvoiceDoc struct {
    InvoiceId           string              `xml:"InvoiceId,attr"`
    RegNumber           string              `xml:"InvoiceGlobUniqId,attr,omitempty"`
    SellerRegnumber     string              `xml:"SellerRegnumber,attr"`
    InvoiceParties      InvoiceParties      `xml:"InvoiceParties"`
    InvoiceInformation  InvoiceInformation  `xml:"InvoiceInformation"`
    InvoiceSumGroup     InvoiceSumGroup     `xml:"InvoiceSumGroup"`
    InvoiceItem         []InvoiceItem       `xml:"InvoiceItem"`
    PaymentInfo         PaymentInfo         `xml:"PaymentInfo"`
}

type InvoiceParties struct {
    SellerParty   Party `xml:"SellerParty"`
    BuyerParty    Party `xml:"BuyerParty"`
}

type Party struct {
    Name          string  `xml:"Name"`
    RegNumber     string  `xml:"RegNumber"`
    VATRegNumber  string  `xml:"VATRegNumber,omitempty"`
    ContactData   ContactData `xml:"ContactData,omitempty"`
    AccountInfo   AccountInfo `xml:"AccountInfo,omitempty"`
}

type ContactData struct {
    LegalAddress  Address `xml:"LegalAddress,omitempty"`
    ContactName   string  `xml:"ContactName,omitempty"`
    PhoneNumber   string  `xml:"PhoneNumber,omitempty"`
    E_MailAddress string  `xml:"E-MailAddress,omitempty"`
}

type Address struct {
    PostalAddress1 string `xml:"PostalAddress1,omitempty"`
    City           string `xml:"City,omitempty"`
    PostalCode     string `xml:"PostalCode,omitempty"`
    Country        string `xml:"Country,omitempty"`
}

type AccountInfo struct {
    AccountNumber string `xml:"AccountNumber"`
    IBAN          string `xml:"IBAN,omitempty"`
    BIC           string `xml:"BIC,omitempty"`
    BankName      string `xml:"BankName,omitempty"`
}

type InvoiceInformation struct {
    Type            InvoiceType `xml:"Type"`
    DocumentName    string      `xml:"DocumentName,omitempty"`
    InvoiceNumber   string      `xml:"InvoiceNumber"`
    InvoiceDate     string      `xml:"InvoiceDate"`
    DueDate         string      `xml:"DueDate,omitempty"`
    InvoiceContentText string   `xml:"InvoiceContentText,omitempty"`
}

type InvoiceType struct {
    Type string `xml:",chardata"`
}

type InvoiceSumGroup struct {
    InvoiceSum      string  `xml:"InvoiceSum"`
    TotalVATSum     string  `xml:"TotalVATSum"`
    TotalSum        string  `xml:"TotalSum"`
    Currency        string  `xml:"Currency"`
    TotalToPay      string  `xml:"TotalToPay"`
    VAT             []VAT   `xml:"VAT,omitempty"`
}

type VAT struct {
    VATRate   string `xml:"VATRate,attr"`
    VATSum    string `xml:"VATSum"`
    SumBeforeVAT string `xml:"SumBeforeVAT,omitempty"`
}

type InvoiceItem struct {
    InvoiceItemGroup InvoiceItemGroup `xml:"InvoiceItemGroup"`
}

type InvoiceItemGroup struct {
    ItemEntry []ItemEntry `xml:"ItemEntry"`
}

type ItemEntry struct {
    RowNo           string `xml:"RowNo,omitempty"`
    Description     string `xml:"Description"`
    ItemSum         string `xml:"ItemSum"`
    ItemAmount      string `xml:"ItemAmount,omitempty"`
    ItemUnit        string `xml:"ItemUnit,omitempty"`
    ItemPrice       string `xml:"ItemPrice,omitempty"`
    VATRate         string `xml:"VATRate,omitempty"`
    VATSum          string `xml:"VATSum,omitempty"`
}

type PaymentInfo struct {
    Currency           string `xml:"Currency"`
    PaymentDescription string `xml:"PaymentDescription,omitempty"`
    PaymentTotalSum    string `xml:"PaymentTotalSum"`
    PayerName          string `xml:"PayerName,omitempty"`
    PaymentId          string `xml:"PaymentId,omitempty"`
    PayToAccount       string `xml:"PayToAccount,omitempty"`
    PayToName          string `xml:"PayToName,omitempty"`
    PayDueDate         string `xml:"PayDueDate,omitempty"`
}

type Footer struct {
    TotalNumberInvoices string `xml:"TotalNumberInvoices"`
    TotalAmount         string `xml:"TotalAmount"`
}

// EInvoiceGenerator creates Estonian e-invoice XML
type EInvoiceGenerator struct {
    tenantService *TenantService
}

func (g *EInvoiceGenerator) GenerateEstonianXML(invoice *Invoice, tenant *Tenant) (string, error) {
    doc := &EInvoice{
        Header: Header{
            Date:       time.Now().Format("2006-01-02"),
            FileId:     uuid.New().String(),
            Version:    "1.2",
            SenderId:   tenant.RegCode,
            ReceiverId: invoice.Partner.RegCode,
        },
        Invoice: []InvoiceDoc{g.buildInvoiceDoc(invoice, tenant)},
        Footer: Footer{
            TotalNumberInvoices: "1",
            TotalAmount:         invoice.Total.String(),
        },
    }

    output, err := xml.MarshalIndent(doc, "", "  ")
    if err != nil {
        return "", err
    }

    return xml.Header + string(output), nil
}

func (g *EInvoiceGenerator) buildInvoiceDoc(inv *Invoice, tenant *Tenant) InvoiceDoc {
    doc := InvoiceDoc{
        InvoiceId:       inv.ID,
        SellerRegnumber: tenant.RegCode,
        InvoiceParties: InvoiceParties{
            SellerParty: Party{
                Name:         tenant.Name,
                RegNumber:    tenant.RegCode,
                VATRegNumber: tenant.VATNumber,
            },
            BuyerParty: Party{
                Name:         inv.Partner.Name,
                RegNumber:    inv.Partner.RegCode,
                VATRegNumber: inv.Partner.VATNumber,
            },
        },
        InvoiceInformation: InvoiceInformation{
            Type:          InvoiceType{Type: "DEB"},
            InvoiceNumber: inv.InvoiceNumber,
            InvoiceDate:   inv.InvoiceDate.Format("2006-01-02"),
            DueDate:       inv.DueDate.Format("2006-01-02"),
        },
        InvoiceSumGroup: InvoiceSumGroup{
            InvoiceSum:  inv.Subtotal.String(),
            TotalVATSum: inv.TaxTotal.String(),
            TotalSum:    inv.Total.String(),
            Currency:    inv.Currency,
            TotalToPay:  inv.BalanceDue.String(),
        },
        PaymentInfo: PaymentInfo{
            Currency:        inv.Currency,
            PaymentTotalSum: inv.Total.String(),
            PayDueDate:      inv.DueDate.Format("2006-01-02"),
        },
    }

    // Add line items
    var entries []ItemEntry
    for i, line := range inv.Lines {
        entries = append(entries, ItemEntry{
            RowNo:       fmt.Sprintf("%d", i+1),
            Description: line.Description,
            ItemSum:     line.LineTotal.String(),
            ItemAmount:  line.Quantity.String(),
            ItemUnit:    line.Unit,
            ItemPrice:   line.UnitPrice.String(),
            VATRate:     line.VATRate.String(),
            VATSum:      line.VATAmount.String(),
        })
    }
    doc.InvoiceItem = []InvoiceItem{{
        InvoiceItemGroup: InvoiceItemGroup{ItemEntry: entries},
    }}

    return doc
}
```

### 7.3 E-Invoice Service

```go
package einvoice

import (
    "context"
    "crypto/sha256"
    "encoding/hex"
)

type EInvoiceService struct {
    db        *pgxpool.Pool
    generator *EInvoiceGenerator
    operators map[string]OperatorClient
}

func (s *EInvoiceService) CreateTransmission(ctx context.Context, tenantID string, invoiceID string, format string) (*EInvoiceTransmission, error) {
    invoice, _ := s.getInvoice(ctx, tenantID, invoiceID)
    tenant, _ := s.getTenant(ctx, tenantID)

    var xmlContent string
    var err error

    switch format {
    case "EE_1.2":
        xmlContent, err = s.generator.GenerateEstonianXML(invoice, tenant)
    case "PEPPOL_BIS_3":
        xmlContent, err = s.generator.GeneratePeppolXML(invoice, tenant)
    default:
        return nil, fmt.Errorf("unsupported format: %s", format)
    }

    if err != nil {
        return nil, err
    }

    hash := sha256.Sum256([]byte(xmlContent))

    transmission := &EInvoiceTransmission{
        ID:         uuid.New().String(),
        TenantID:   tenantID,
        InvoiceID:  invoiceID,
        Direction:  "OUTBOUND",
        Format:     format,
        Status:     "PENDING",
        XMLContent: xmlContent,
        XMLHash:    hex.EncodeToString(hash[:]),
        CreatedAt:  time.Now(),
    }

    // Save to database...
    return transmission, nil
}

func (s *EInvoiceService) SendEInvoice(ctx context.Context, tenantID, transmissionID string) error {
    transmission, _ := s.getTransmission(ctx, tenantID, transmissionID)
    operator, _ := s.getOperator(ctx, tenantID, transmission.OperatorID)

    client := s.operators[operator.Code]
    if client == nil {
        return fmt.Errorf("operator not configured: %s", operator.Code)
    }

    result, err := client.Send(ctx, transmission.XMLContent)
    if err != nil {
        // Update with error
        s.updateTransmissionError(ctx, transmissionID, err.Error())
        return err
    }

    // Update with success
    s.updateTransmissionSuccess(ctx, transmissionID, result.TransmissionID)
    return nil
}

func (s *EInvoiceService) CheckPartnerAcceptsEInvoice(ctx context.Context, regCode string) (bool, error) {
    // Check cache first
    var cached EInvoiceRegistryCache
    err := s.db.QueryRow(ctx, `
        SELECT accepts_einvoice FROM einvoice_registry_cache
        WHERE reg_code = $1 AND expires_at > NOW()
    `, regCode).Scan(&cached.AcceptsEInvoice)

    if err == nil {
        return cached.AcceptsEInvoice, nil
    }

    // Query RIK registry (Estonian Business Registry)
    // This would integrate with the actual e-invoice registry API
    accepts := s.queryEInvoiceRegistry(regCode)

    // Cache result
    s.cacheRegistryResult(ctx, regCode, accepts)

    return accepts, nil
}
```

### 7.4 Key Features

| Feature | Description |
|---------|-------------|
| Estonian v1.2 Format | Full EVS 923:2014/AC:2017 compliance |
| Peppol BIS 3.0 | EU standard e-invoice format |
| Multi-operator | Support for Finbite, Billberry, Telema |
| Registry Lookup | Check if partner accepts e-invoices |
| Transmission Tracking | Full audit trail with XML hash |

---

## Section 8: Reports

### 8.1 Database Schema

```sql
-- Fiscal years
CREATE TABLE fiscal_years (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID NOT NULL REFERENCES tenants(id),
    name            VARCHAR(50) NOT NULL,
    start_date      DATE NOT NULL,
    end_date        DATE NOT NULL,
    is_closed       BOOLEAN DEFAULT false,
    closed_at       TIMESTAMPTZ,
    closed_by       UUID REFERENCES users(id),
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(tenant_id, name),
    CHECK(end_date > start_date)
);

-- Account categories for financial statement mapping
CREATE TABLE account_categories (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID NOT NULL REFERENCES tenants(id),
    code            VARCHAR(20) NOT NULL,
    name            VARCHAR(100) NOT NULL,
    statement_type  VARCHAR(20) NOT NULL,      -- 'BALANCE_SHEET', 'INCOME_STATEMENT'
    category_type   VARCHAR(30) NOT NULL,      -- 'ASSET', 'LIABILITY', 'EQUITY', 'REVENUE', 'EXPENSE'
    parent_id       UUID REFERENCES account_categories(id),
    display_order   INTEGER NOT NULL DEFAULT 0,
    UNIQUE(tenant_id, code)
);

-- Saved report configurations
CREATE TABLE report_templates (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID NOT NULL REFERENCES tenants(id),
    report_type     VARCHAR(30) NOT NULL,
    name            VARCHAR(100) NOT NULL,
    config_json     JSONB NOT NULL,
    is_default      BOOLEAN DEFAULT false,
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(tenant_id, report_type, name)
);

-- Report snapshots for audit trail
CREATE TABLE report_snapshots (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID NOT NULL REFERENCES tenants(id),
    report_type     VARCHAR(30) NOT NULL,
    template_id     UUID REFERENCES report_templates(id),
    period_start    DATE NOT NULL,
    period_end      DATE NOT NULL,
    generated_at    TIMESTAMPTZ DEFAULT NOW(),
    generated_by    UUID REFERENCES users(id),
    parameters_json JSONB,
    data_json       JSONB NOT NULL,
    checksum        VARCHAR(64) NOT NULL       -- SHA-256 of data_json
);

CREATE INDEX idx_report_snapshots_tenant_type ON report_snapshots(tenant_id, report_type);
```

### 8.2 Report Types (Go)

```go
package reports

import (
    "time"
    "github.com/shopspring/decimal"
)

type AccountBalance struct {
    AccountID     string          `json:"account_id"`
    AccountCode   string          `json:"account_code"`
    AccountName   string          `json:"account_name"`
    AccountType   string          `json:"account_type"`
    DebitBalance  decimal.Decimal `json:"debit_balance"`
    CreditBalance decimal.Decimal `json:"credit_balance"`
    NetBalance    decimal.Decimal `json:"net_balance"`
}

type TrialBalance struct {
    TenantID     string           `json:"tenant_id"`
    AsOfDate     time.Time        `json:"as_of_date"`
    GeneratedAt  time.Time        `json:"generated_at"`
    Accounts     []AccountBalance `json:"accounts"`
    TotalDebits  decimal.Decimal  `json:"total_debits"`
    TotalCredits decimal.Decimal  `json:"total_credits"`
    IsBalanced   bool             `json:"is_balanced"`
}

type BalanceSheetSection struct {
    Name     string              `json:"name"`
    Items    []BalanceSheetItem  `json:"items"`
    Subtotal decimal.Decimal     `json:"subtotal"`
}

type BalanceSheetItem struct {
    AccountCode string          `json:"account_code"`
    Name        string          `json:"name"`
    Amount      decimal.Decimal `json:"amount"`
    PriorAmount decimal.Decimal `json:"prior_amount,omitempty"`
}

type BalanceSheet struct {
    TenantID                  string              `json:"tenant_id"`
    AsOfDate                  time.Time           `json:"as_of_date"`
    ComparisonDate            *time.Time          `json:"comparison_date,omitempty"`
    GeneratedAt               time.Time           `json:"generated_at"`
    Currency                  string              `json:"currency"`
    Assets                    BalanceSheetSection `json:"assets"`
    Liabilities               BalanceSheetSection `json:"liabilities"`
    Equity                    BalanceSheetSection `json:"equity"`
    TotalAssets               decimal.Decimal     `json:"total_assets"`
    TotalLiabilities          decimal.Decimal     `json:"total_liabilities"`
    TotalEquity               decimal.Decimal     `json:"total_equity"`
    TotalLiabilitiesAndEquity decimal.Decimal     `json:"total_liabilities_and_equity"`
    IsBalanced                bool                `json:"is_balanced"`
}

type IncomeStatementSection struct {
    Name     string                `json:"name"`
    Items    []IncomeStatementItem `json:"items"`
    Subtotal decimal.Decimal       `json:"subtotal"`
}

type IncomeStatementItem struct {
    AccountCode    string          `json:"account_code"`
    Name           string          `json:"name"`
    Amount         decimal.Decimal `json:"amount"`
    PriorAmount    decimal.Decimal `json:"prior_amount,omitempty"`
    PercentOfSales decimal.Decimal `json:"percent_of_sales,omitempty"`
}

type IncomeStatement struct {
    TenantID          string                 `json:"tenant_id"`
    PeriodStart       time.Time              `json:"period_start"`
    PeriodEnd         time.Time              `json:"period_end"`
    GeneratedAt       time.Time              `json:"generated_at"`
    Currency          string                 `json:"currency"`
    Revenue           IncomeStatementSection `json:"revenue"`
    CostOfGoodsSold   IncomeStatementSection `json:"cost_of_goods_sold"`
    GrossProfit       decimal.Decimal        `json:"gross_profit"`
    GrossMargin       decimal.Decimal        `json:"gross_margin_percent"`
    OperatingExpenses IncomeStatementSection `json:"operating_expenses"`
    OperatingIncome   decimal.Decimal        `json:"operating_income"`
    IncomeBeforeTax   decimal.Decimal        `json:"income_before_tax"`
    IncomeTaxExpense  decimal.Decimal        `json:"income_tax_expense"`
    NetIncome         decimal.Decimal        `json:"net_income"`
    NetMargin         decimal.Decimal        `json:"net_margin_percent"`
}

type AgedPartnerBalance struct {
    PartnerID   string          `json:"partner_id"`
    PartnerName string          `json:"partner_name"`
    RegCode     string          `json:"reg_code,omitempty"`
    Current     decimal.Decimal `json:"current"`
    Days1to30   decimal.Decimal `json:"days_1_30"`
    Days31to60  decimal.Decimal `json:"days_31_60"`
    Days61to90  decimal.Decimal `json:"days_61_90"`
    Over90Days  decimal.Decimal `json:"over_90_days"`
    Total       decimal.Decimal `json:"total"`
}

type AgedReport struct {
    TenantID    string               `json:"tenant_id"`
    ReportType  string               `json:"report_type"`
    AsOfDate    time.Time            `json:"as_of_date"`
    GeneratedAt time.Time            `json:"generated_at"`
    Currency    string               `json:"currency"`
    Partners    []AgedPartnerBalance `json:"partners"`
    GrandTotal  decimal.Decimal      `json:"grand_total"`
}
```

### 8.3 Report Service

```go
package reports

import (
    "context"
    "crypto/sha256"
    "encoding/hex"
    "encoding/json"
    "time"

    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/shopspring/decimal"
)

type ReportService struct {
    db *pgxpool.Pool
}

func NewReportService(db *pgxpool.Pool) *ReportService {
    return &ReportService{db: db}
}

func (s *ReportService) GenerateTrialBalance(ctx context.Context, tenantID string, asOfDate time.Time) (*TrialBalance, error) {
    query := `
        WITH account_totals AS (
            SELECT
                a.id AS account_id,
                a.code AS account_code,
                a.name AS account_name,
                a.account_type,
                COALESCE(SUM(jel.debit_amount), 0) AS total_debits,
                COALESCE(SUM(jel.credit_amount), 0) AS total_credits
            FROM accounts a
            LEFT JOIN journal_entry_lines jel ON jel.account_id = a.id
            LEFT JOIN journal_entries je ON je.id = jel.journal_entry_id
            WHERE a.tenant_id = $1
              AND (je.id IS NULL OR (je.entry_date <= $2 AND je.status = 'POSTED'))
            GROUP BY a.id, a.code, a.name, a.account_type
        )
        SELECT
            account_id, account_code, account_name, account_type,
            total_debits, total_credits,
            CASE
                WHEN account_type IN ('ASSET', 'EXPENSE') THEN total_debits - total_credits
                ELSE total_credits - total_debits
            END AS net_balance
        FROM account_totals
        WHERE total_debits != 0 OR total_credits != 0
        ORDER BY account_code
    `

    rows, err := s.db.Query(ctx, query, tenantID, asOfDate)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var accounts []AccountBalance
    totalDebits := decimal.Zero
    totalCredits := decimal.Zero

    for rows.Next() {
        var ab AccountBalance
        if err := rows.Scan(
            &ab.AccountID, &ab.AccountCode, &ab.AccountName, &ab.AccountType,
            &ab.DebitBalance, &ab.CreditBalance, &ab.NetBalance,
        ); err != nil {
            return nil, err
        }
        accounts = append(accounts, ab)
        totalDebits = totalDebits.Add(ab.DebitBalance)
        totalCredits = totalCredits.Add(ab.CreditBalance)
    }

    return &TrialBalance{
        TenantID:     tenantID,
        AsOfDate:     asOfDate,
        GeneratedAt:  time.Now(),
        Accounts:     accounts,
        TotalDebits:  totalDebits,
        TotalCredits: totalCredits,
        IsBalanced:   totalDebits.Equal(totalCredits),
    }, nil
}

func (s *ReportService) GenerateAgedReceivables(ctx context.Context, tenantID string, asOfDate time.Time) (*AgedReport, error) {
    query := `
        SELECT
            p.id AS partner_id,
            p.name AS partner_name,
            p.reg_code,
            COALESCE(SUM(CASE WHEN i.due_date >= $2 THEN i.balance_due ELSE 0 END), 0) AS current_amount,
            COALESCE(SUM(CASE WHEN i.due_date < $2 AND i.due_date >= $2 - INTERVAL '30 days' THEN i.balance_due ELSE 0 END), 0) AS days_1_30,
            COALESCE(SUM(CASE WHEN i.due_date < $2 - INTERVAL '30 days' AND i.due_date >= $2 - INTERVAL '60 days' THEN i.balance_due ELSE 0 END), 0) AS days_31_60,
            COALESCE(SUM(CASE WHEN i.due_date < $2 - INTERVAL '60 days' AND i.due_date >= $2 - INTERVAL '90 days' THEN i.balance_due ELSE 0 END), 0) AS days_61_90,
            COALESCE(SUM(CASE WHEN i.due_date < $2 - INTERVAL '90 days' THEN i.balance_due ELSE 0 END), 0) AS over_90
        FROM invoices i
        JOIN partners p ON p.id = i.partner_id
        WHERE i.tenant_id = $1
          AND i.invoice_type = 'SALES'
          AND i.status IN ('SENT', 'PARTIALLY_PAID')
          AND i.balance_due > 0
        GROUP BY p.id, p.name, p.reg_code
        HAVING SUM(i.balance_due) > 0
        ORDER BY SUM(i.balance_due) DESC
    `

    rows, err := s.db.Query(ctx, query, tenantID, asOfDate)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    report := &AgedReport{
        TenantID:    tenantID,
        ReportType:  "RECEIVABLES",
        AsOfDate:    asOfDate,
        GeneratedAt: time.Now(),
        Currency:    "EUR",
    }

    for rows.Next() {
        var pb AgedPartnerBalance
        if err := rows.Scan(
            &pb.PartnerID, &pb.PartnerName, &pb.RegCode,
            &pb.Current, &pb.Days1to30, &pb.Days31to60,
            &pb.Days61to90, &pb.Over90Days,
        ); err != nil {
            return nil, err
        }
        pb.Total = pb.Current.Add(pb.Days1to30).Add(pb.Days31to60).Add(pb.Days61to90).Add(pb.Over90Days)
        report.Partners = append(report.Partners, pb)
        report.GrandTotal = report.GrandTotal.Add(pb.Total)
    }

    return report, nil
}

func (s *ReportService) SaveReportSnapshot(ctx context.Context, tenantID, reportType string, report interface{}, periodStart, periodEnd time.Time, userID string) (string, error) {
    dataJSON, err := json.Marshal(report)
    if err != nil {
        return "", err
    }

    hash := sha256.Sum256(dataJSON)
    checksum := hex.EncodeToString(hash[:])

    var snapshotID string
    err = s.db.QueryRow(ctx, `
        INSERT INTO report_snapshots (tenant_id, report_type, period_start, period_end, generated_by, data_json, checksum)
        VALUES ($1, $2, $3, $4, $5, $6, $7)
        RETURNING id
    `, tenantID, reportType, periodStart, periodEnd, userID, dataJSON, checksum).Scan(&snapshotID)

    return snapshotID, err
}
```

### 8.4 Key Features

| Feature | Description |
|---------|-------------|
| Trial Balance | All accounts with debit/credit balances |
| Balance Sheet | Assets, Liabilities, Equity as of date |
| Income Statement | Revenue, COGS, Expenses for period |
| Aged AR/AP | Receivables/Payables by aging buckets |
| Report Snapshots | Immutable audit trail with checksums |
| Export Formats | Excel, CSV, PDF |

---

## Section 9: Frontend Architecture

### 9.1 Project Structure

```
src/
├── lib/
│   ├── api/
│   │   ├── client.ts           # HTTP client with auth
│   │   ├── accounts.ts         # Chart of accounts API
│   │   ├── invoices.ts         # Invoice CRUD
│   │   └── reports.ts          # Financial reports
│   ├── components/
│   │   ├── ui/                 # Base UI components
│   │   ├── forms/              # Form components
│   │   ├── reports/            # Report components
│   │   └── layout/             # Layout components
│   ├── stores/
│   │   ├── auth.ts             # Authentication state
│   │   ├── tenant.ts           # Current tenant context
│   │   └── notifications.ts    # Toast notifications
│   ├── utils/
│   │   ├── currency.ts         # Money formatting
│   │   ├── date.ts             # Date utilities
│   │   └── decimal.ts          # Decimal.js wrapper
│   └── types/
│       └── models.ts           # Domain models
├── routes/
│   ├── +layout.svelte
│   ├── +page.svelte            # Dashboard
│   ├── accounting/
│   ├── sales/
│   ├── purchases/
│   ├── inventory/
│   ├── payroll/
│   ├── reports/
│   └── settings/
└── app.html
```

### 9.2 API Client

```typescript
// src/lib/api/client.ts
import { browser } from '$app/environment';
import { goto } from '$app/navigation';
import { auth } from '$lib/stores/auth';
import Decimal from 'decimal.js';

const API_BASE = import.meta.env.VITE_API_URL || '/api/v1';

class ApiError extends Error {
  constructor(
    public status: number,
    public code: string,
    message: string
  ) {
    super(message);
    this.name = 'ApiError';
  }
}

function decimalReviver(key: string, value: unknown): unknown {
  if (typeof value === 'string' && /^-?\d+\.?\d*$/.test(value)) {
    if (key.includes('amount') || key.includes('balance') ||
        key.includes('price') || key.includes('total') ||
        key.includes('rate') || key.includes('quantity')) {
      return new Decimal(value);
    }
  }
  return value;
}

async function request<T>(endpoint: string, options: RequestOptions = {}): Promise<T> {
  const { method = 'GET', body, headers = {} } = options;

  let token: string | null = null;
  if (browser) {
    auth.subscribe(a => token = a.token)();
  }

  const response = await fetch(`${API_BASE}${endpoint}`, {
    method,
    headers: {
      'Content-Type': 'application/json',
      ...(token ? { 'Authorization': `Bearer ${token}` } : {}),
      ...headers
    },
    body: body ? JSON.stringify(body, (_, v) =>
      v instanceof Decimal ? v.toString() : v
    ) : undefined
  });

  if (response.status === 401) {
    auth.logout();
    if (browser) goto('/login');
    throw new ApiError(401, 'UNAUTHORIZED', 'Session expired');
  }

  const text = await response.text();
  const data = text ? JSON.parse(text, decimalReviver) : null;

  if (!response.ok) {
    throw new ApiError(response.status, data?.code || 'UNKNOWN', data?.message || 'Request failed');
  }

  return data as T;
}

export const api = {
  get: <T>(endpoint: string) => request<T>(endpoint),
  post: <T>(endpoint: string, body: unknown) => request<T>(endpoint, { method: 'POST', body }),
  put: <T>(endpoint: string, body: unknown) => request<T>(endpoint, { method: 'PUT', body }),
  patch: <T>(endpoint: string, body: unknown) => request<T>(endpoint, { method: 'PATCH', body }),
  delete: <T>(endpoint: string) => request<T>(endpoint, { method: 'DELETE' })
};
```

### 9.3 Authentication Store

```typescript
// src/lib/stores/auth.ts
import { writable, derived } from 'svelte/store';
import { browser } from '$app/environment';

interface AuthState {
  token: string | null;
  user: User | null;
  loading: boolean;
}

function createAuthStore() {
  const initial: AuthState = {
    token: browser ? localStorage.getItem('token') : null,
    user: browser ? JSON.parse(localStorage.getItem('user') || 'null') : null,
    loading: false
  };

  const { subscribe, set, update } = writable<AuthState>(initial);

  return {
    subscribe,

    async login(email: string, password: string): Promise<void> {
      update(s => ({ ...s, loading: true }));

      try {
        const response = await fetch('/api/v1/auth/login', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ email, password })
        });

        if (!response.ok) {
          const error = await response.json();
          throw new Error(error.message || 'Login failed');
        }

        const { token, user } = await response.json();

        if (browser) {
          localStorage.setItem('token', token);
          localStorage.setItem('user', JSON.stringify(user));
        }

        set({ token, user, loading: false });
      } catch (e) {
        update(s => ({ ...s, loading: false }));
        throw e;
      }
    },

    logout(): void {
      if (browser) {
        localStorage.removeItem('token');
        localStorage.removeItem('user');
      }
      set({ token: null, user: null, loading: false });
    }
  };
}

export const auth = createAuthStore();
export const isAuthenticated = derived(auth, $auth => !!$auth.token);
export const currentUser = derived(auth, $auth => $auth.user);
```

### 9.4 Currency Utilities

```typescript
// src/lib/utils/currency.ts
import Decimal from 'decimal.js';

export function formatMoney(
  amount: Decimal | string | number,
  currency = 'EUR',
  locale = 'et-EE'
): string {
  const value = amount instanceof Decimal
    ? amount.toNumber()
    : typeof amount === 'string'
      ? parseFloat(amount)
      : amount;

  return new Intl.NumberFormat(locale, {
    style: 'currency',
    currency,
    minimumFractionDigits: 2
  }).format(value);
}

export function parseMoneyInput(input: string): Decimal {
  const normalized = input
    .replace(/\s/g, '')
    .replace(/,(?=\d{3})/g, '')
    .replace(',', '.');

  return new Decimal(normalized || '0');
}
```

### 9.5 Key Features

| Feature | Description |
|---------|-------------|
| Type-safe API | Full TypeScript with Decimal.js |
| Reactive Forms | Real-time calculations on invoice lines |
| Auth Store | Persistent login with localStorage |
| Estonian Locale | Currency and number formatting |
| SSR Support | Server-side rendering with SvelteKit |

---

## Section 10: Testing Strategy

### 10.1 Testing Pyramid

```
┌─────────────────────────────────────────────────────────────┐
│                      Testing Pyramid                        │
├─────────────────────────────────────────────────────────────┤
│                          E2E (~50 tests)                    │
│                        (Playwright)                         │
│                                                             │
│                    Integration (~200 tests)                 │
│                      (Go + Testcontainers)                  │
│                                                             │
│                       Unit (~500+ tests)                    │
│                    (Go + Vitest)                            │
└─────────────────────────────────────────────────────────────┘
```

### 10.2 Test Fixtures Factory (Go)

```go
// internal/testutil/fixtures.go
package testutil

import (
    "fmt"
    "time"
    "github.com/google/uuid"
    "github.com/shopspring/decimal"
)

type Factory struct {
    tenantID string
    sequence int
}

func NewFactory(tenantID string) *Factory {
    return &Factory{tenantID: tenantID, sequence: 0}
}

func (f *Factory) nextSeq() int {
    f.sequence++
    return f.sequence
}

type InvoiceBuilder struct {
    invoice Invoice
    lines   []InvoiceLine
}

func (f *Factory) Invoice() *InvoiceBuilder {
    seq := f.nextSeq()
    now := time.Now()
    return &InvoiceBuilder{
        invoice: Invoice{
            ID:            uuid.New().String(),
            TenantID:      f.tenantID,
            InvoiceNumber: fmt.Sprintf("INV-%05d", seq),
            InvoiceType:   "SALES",
            InvoiceDate:   now,
            DueDate:       now.AddDate(0, 0, 14),
            Currency:      "EUR",
            Status:        "DRAFT",
        },
        lines: []InvoiceLine{},
    }
}

func (b *InvoiceBuilder) WithPartner(partnerID string) *InvoiceBuilder {
    b.invoice.PartnerID = partnerID
    return b
}

func (b *InvoiceBuilder) WithLine(description string, qty, unitPrice, vatRate float64) *InvoiceBuilder {
    qtyDec := decimal.NewFromFloat(qty)
    priceDec := decimal.NewFromFloat(unitPrice)
    vatRateDec := decimal.NewFromFloat(vatRate)

    lineNet := qtyDec.Mul(priceDec)
    vatAmount := lineNet.Mul(vatRateDec).Div(decimal.NewFromInt(100))
    lineTotal := lineNet.Add(vatAmount)

    line := InvoiceLine{
        ID:          uuid.New().String(),
        Description: description,
        Quantity:    qtyDec,
        UnitPrice:   priceDec,
        VATRate:     vatRateDec,
        VATAmount:   vatAmount,
        LineTotal:   lineTotal,
    }

    b.lines = append(b.lines, line)
    b.invoice.Subtotal = b.invoice.Subtotal.Add(lineNet)
    b.invoice.TaxTotal = b.invoice.TaxTotal.Add(vatAmount)
    b.invoice.Total = b.invoice.Subtotal.Add(b.invoice.TaxTotal)
    b.invoice.BalanceDue = b.invoice.Total

    return b
}

func (b *InvoiceBuilder) Build() (Invoice, []InvoiceLine) {
    return b.invoice, b.lines
}
```

### 10.3 Unit Tests - Double-Entry

```go
func TestJournalEntry_MustBalance(t *testing.T) {
    tests := []struct {
        name    string
        debits  []decimal.Decimal
        credits []decimal.Decimal
        wantErr bool
    }{
        {
            name:    "balanced entry passes",
            debits:  []decimal.Decimal{decimal.NewFromFloat(100.00)},
            credits: []decimal.Decimal{decimal.NewFromFloat(100.00)},
            wantErr: false,
        },
        {
            name:    "unbalanced entry fails",
            debits:  []decimal.Decimal{decimal.NewFromFloat(100.00)},
            credits: []decimal.Decimal{decimal.NewFromFloat(99.99)},
            wantErr: true,
        },
        {
            name:    "handles large numbers",
            debits:  []decimal.Decimal{decimal.RequireFromString("99999999999999999999.12345678")},
            credits: []decimal.Decimal{decimal.RequireFromString("99999999999999999999.12345678")},
            wantErr: false,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            entry := &JournalEntry{Lines: make([]JournalEntryLine, 0)}

            for _, d := range tt.debits {
                entry.Lines = append(entry.Lines, JournalEntryLine{DebitAmount: d})
            }
            for _, c := range tt.credits {
                entry.Lines = append(entry.Lines, JournalEntryLine{CreditAmount: c})
            }

            err := entry.Validate()
            if tt.wantErr {
                assert.Error(t, err)
            } else {
                assert.NoError(t, err)
            }
        })
    }
}
```

### 10.4 Integration Tests with Testcontainers

```go
//go:build integration

package accounting

import (
    "context"
    "testing"
    "github.com/stretchr/testify/suite"
    "github.com/testcontainers/testcontainers-go/modules/postgres"
)

type JournalIntegrationSuite struct {
    suite.Suite
    container *postgres.PostgresContainer
    db        *pgxpool.Pool
    service   *JournalService
    tenantID  string
}

func TestJournalIntegration(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping integration test")
    }
    suite.Run(t, new(JournalIntegrationSuite))
}

func (s *JournalIntegrationSuite) SetupSuite() {
    ctx := context.Background()
    container, _ := postgres.Run(ctx, "postgres:16-alpine",
        postgres.WithDatabase("testdb"),
        postgres.WithUsername("test"),
        postgres.WithPassword("test"),
    )
    s.container = container

    connStr, _ := container.ConnectionString(ctx, "sslmode=disable")
    s.db, _ = pgxpool.New(ctx, connStr)

    runMigrations(s.db)
    s.tenantID = s.createTestTenant(ctx)
    s.service = NewJournalService(s.db)
}

func (s *JournalIntegrationSuite) TestTrialBalanceAlwaysBalances() {
    ctx := context.Background()

    // Create multiple entries
    for i := 0; i < 10; i++ {
        entry, _ := s.service.CreateEntry(ctx, s.tenantID, &CreateJournalEntryRequest{
            EntryDate:   time.Now(),
            Description: fmt.Sprintf("Entry %d", i),
            Lines: []CreateJournalEntryLine{
                {AccountID: expenseAcct, DebitAmount: decimal.NewFromFloat(float64(i*100 + 50))},
                {AccountID: cashAcct, CreditAmount: decimal.NewFromFloat(float64(i*100 + 50))},
            },
        })
        s.service.PostEntry(ctx, s.tenantID, entry.ID)
    }

    reportService := NewReportService(s.db)
    trialBalance, _ := reportService.GenerateTrialBalance(ctx, s.tenantID, time.Now())

    s.True(trialBalance.IsBalanced)
    s.True(trialBalance.TotalDebits.Equal(trialBalance.TotalCredits))
}
```

### 10.5 E2E Tests with Playwright

```typescript
// e2e/invoices.spec.ts
import { test, expect } from '@playwright/test';

test.describe('Invoice Management', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/login');
    await page.fill('input[name="email"]', 'test@example.com');
    await page.fill('input[name="password"]', 'password');
    await page.click('button[type="submit"]');
    await page.waitForURL('/');
  });

  test('creates a new sales invoice', async ({ page }) => {
    await page.goto('/sales/invoices');
    await page.click('text=New Invoice');
    await page.selectOption('select[name="partner"]', { label: 'Test Customer' });
    await page.fill('input[name="invoiceDate"]', '2024-01-15');
    await page.fill('input[placeholder="Item description"]', 'Consulting services');
    await page.fill('input[name="quantity"]', '10');
    await page.fill('input[name="unitPrice"]', '150');

    await expect(page.locator('.grand-total')).toHaveText('€1,800.00');
    await page.click('button:has-text("Create Invoice")');
    await expect(page).toHaveURL(/\/sales\/invoices\/inv-/);
  });
});
```

### 10.6 Test Commands

```makefile
test-unit:
	go test -short -race -coverprofile=coverage.out ./...

test-integration:
	go test -race -tags=integration -coverprofile=coverage-integration.out ./...

test-frontend:
	cd frontend && npm run test

test-e2e:
	cd frontend && npx playwright test

test-all: test-unit test-integration test-frontend test-e2e
```

### 10.7 Key Testing Principles

| Principle | Implementation |
|-----------|----------------|
| Double-entry always balances | Every journal entry test validates debits = credits |
| Decimal precision | Use `decimal.Decimal` comparisons, never float |
| Date-aware VAT | Test rate selection at boundary dates |
| Immutable audit trail | Verify posted entries reject modification |
| Multi-tenant isolation | Ensure data never leaks between tenants |

---

## Section 11: Deployment & CI/CD

### 11.1 Dockerfiles

```dockerfile
# deploy/docker/Dockerfile.api
FROM golang:1.22-alpine AS builder
WORKDIR /app
RUN apk add --no-cache git ca-certificates tzdata
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s -X main.Version=${VERSION:-dev}" \
    -o /app/bin/api ./cmd/api

FROM alpine:3.19
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=builder /app/bin/api /app/api
COPY --from=builder /app/migrations /app/migrations
RUN adduser -D -u 1000 appuser
USER appuser
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1
ENTRYPOINT ["/app/api"]
```

```dockerfile
# deploy/docker/Dockerfile.frontend
FROM node:20-alpine AS builder
WORKDIR /app
COPY package*.json ./
RUN npm ci
COPY . .
ENV NODE_ENV=production
RUN npm run build

FROM node:20-alpine
WORKDIR /app
RUN adduser -D -u 1000 appuser
COPY --from=builder /app/build /app/build
COPY --from=builder /app/package*.json ./
RUN npm ci --omit=dev
USER appuser
EXPOSE 3000
ENV NODE_ENV=production
CMD ["node", "build"]
```

### 11.2 Docker Compose (Production)

```yaml
# deploy/docker-compose.prod.yml
version: '3.8'

services:
  postgres:
    image: postgres:16-alpine
    restart: unless-stopped
    environment:
      POSTGRES_DB: ${DB_NAME:-openaccounting}
      POSTGRES_USER: ${DB_USER:-openaccounting}
      POSTGRES_PASSWORD: ${DB_PASSWORD:?Database password required}
    volumes:
      - postgres_data:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U ${DB_USER:-openaccounting}"]
      interval: 10s
      timeout: 5s
      retries: 5

  api:
    image: ghcr.io/yourusername/open-accounting-api:${VERSION:-latest}
    restart: unless-stopped
    depends_on:
      postgres:
        condition: service_healthy
    environment:
      DATABASE_URL: postgres://${DB_USER}:${DB_PASSWORD}@postgres:5432/${DB_NAME}?sslmode=disable
      JWT_SECRET: ${JWT_SECRET:?JWT secret required}
    labels:
      - "traefik.enable=true"
      - "traefik.http.routers.api.rule=Host(`${DOMAIN}`) && PathPrefix(`/api`)"
      - "traefik.http.routers.api.tls.certresolver=letsencrypt"

  frontend:
    image: ghcr.io/yourusername/open-accounting-frontend:${VERSION:-latest}
    restart: unless-stopped
    depends_on:
      - api
    environment:
      PUBLIC_API_URL: https://${DOMAIN}/api
    labels:
      - "traefik.enable=true"
      - "traefik.http.routers.frontend.rule=Host(`${DOMAIN}`)"
      - "traefik.http.routers.frontend.tls.certresolver=letsencrypt"

  traefik:
    image: traefik:v3.0
    restart: unless-stopped
    command:
      - "--providers.docker=true"
      - "--entrypoints.websecure.address=:443"
      - "--certificatesresolvers.letsencrypt.acme.email=${ACME_EMAIL}"
      - "--certificatesresolvers.letsencrypt.acme.storage=/letsencrypt/acme.json"
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock:ro
      - letsencrypt:/letsencrypt

volumes:
  postgres_data:
  letsencrypt:
```

### 11.3 One-Line Installer

```bash
#!/usr/bin/env bash
# deploy/scripts/install.sh
# Usage: curl -fsSL https://raw.githubusercontent.com/yourusername/open-accounting/main/deploy/scripts/install.sh | bash

set -euo pipefail

INSTALL_DIR="${INSTALL_DIR:-/opt/open-accounting}"
VERSION="${VERSION:-latest}"
REPO="https://github.com/yourusername/open-accounting"

check_requirements() {
    command -v docker >/dev/null 2>&1 || { echo "Docker required"; exit 1; }
    docker info >/dev/null 2>&1 || { echo "Docker not running"; exit 1; }
}

configure() {
    read -p "Enter your domain: " DOMAIN
    read -p "Enter email for SSL: " ACME_EMAIL
    read -s -p "Database password (Enter for auto): " DB_PASSWORD
    [[ -z "$DB_PASSWORD" ]] && DB_PASSWORD=$(openssl rand -base64 32 | tr -d '/+=' | cut -c1-32)
    JWT_SECRET=$(openssl rand -base64 32 | tr -d '/+=' | cut -c1-32)
}

setup() {
    sudo mkdir -p "$INSTALL_DIR"/{backups,data}
    cd "$INSTALL_DIR"

    curl -fsSL "$REPO/raw/main/deploy/docker-compose.prod.yml" -o docker-compose.yml

    cat > .env << EOF
VERSION=${VERSION}
DOMAIN=${DOMAIN}
ACME_EMAIL=${ACME_EMAIL}
DB_NAME=openaccounting
DB_USER=openaccounting
DB_PASSWORD=${DB_PASSWORD}
JWT_SECRET=${JWT_SECRET}
EOF
    chmod 600 .env

    docker compose pull
    docker compose up -d
}

main() {
    echo "Open Accounting Installer"
    check_requirements
    configure
    setup
    echo "Installation complete: https://${DOMAIN}"
}

main "$@"
```

### 11.4 GitHub Actions CI

```yaml
# .github/workflows/ci.yml
name: CI

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

jobs:
  lint-go:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.22'
      - uses: golangci/golangci-lint-action@v4

  test-go:
    runs-on: ubuntu-latest
    services:
      postgres:
        image: postgres:16-alpine
        env:
          POSTGRES_USER: test
          POSTGRES_PASSWORD: test
          POSTGRES_DB: testdb
        ports:
          - 5432:5432
        options: --health-cmd pg_isready --health-interval 10s
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.22'
      - run: go test -race -coverprofile=coverage.out -short ./...
      - run: go test -race -tags=integration ./...
        env:
          DATABASE_URL: postgres://test:test@localhost:5432/testdb?sslmode=disable

  test-frontend:
    runs-on: ubuntu-latest
    defaults:
      run:
        working-directory: ./frontend
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-node@v4
        with:
          node-version: '20'
      - run: npm ci
      - run: npm run lint
      - run: npm run test:unit

  build:
    needs: [lint-go, test-go, test-frontend]
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: docker/setup-buildx-action@v3
      - uses: docker/build-push-action@v5
        with:
          context: .
          file: deploy/docker/Dockerfile.api
          push: false
          tags: open-accounting-api:test
```

### 11.5 GitHub Actions Release

```yaml
# .github/workflows/release.yml
name: Release

on:
  push:
    tags:
      - 'v*'

jobs:
  build-and-push:
    runs-on: ubuntu-latest
    permissions:
      contents: read
      packages: write
    steps:
      - uses: actions/checkout@v4
      - uses: docker/setup-qemu-action@v3
      - uses: docker/setup-buildx-action@v3
      - uses: docker/login-action@v3
        with:
          registry: ghcr.io
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}

      - name: Build and push API
        uses: docker/build-push-action@v5
        with:
          context: .
          file: deploy/docker/Dockerfile.api
          platforms: linux/amd64,linux/arm64
          push: true
          tags: |
            ghcr.io/${{ github.repository }}-api:${{ github.ref_name }}
            ghcr.io/${{ github.repository }}-api:latest

      - name: Build and push Frontend
        uses: docker/build-push-action@v5
        with:
          context: ./frontend
          file: deploy/docker/Dockerfile.frontend
          platforms: linux/amd64,linux/arm64
          push: true
          tags: |
            ghcr.io/${{ github.repository }}-frontend:${{ github.ref_name }}
            ghcr.io/${{ github.repository }}-frontend:latest
```

### 11.6 Makefile

```makefile
VERSION ?= $(shell git describe --tags --always --dirty)
REGISTRY ?= ghcr.io/yourusername

dev:
	docker compose -f deploy/docker-compose.yml up --build

test:
	go test -race -short ./...

test-integration:
	go test -race -tags=integration ./...

lint:
	golangci-lint run
	cd frontend && npm run lint

build-api:
	docker build -f deploy/docker/Dockerfile.api -t $(REGISTRY)/open-accounting-api:$(VERSION) .

build-frontend:
	docker build -f deploy/docker/Dockerfile.frontend -t $(REGISTRY)/open-accounting-frontend:$(VERSION) ./frontend

build: build-api build-frontend

push: build
	docker push $(REGISTRY)/open-accounting-api:$(VERSION)
	docker push $(REGISTRY)/open-accounting-frontend:$(VERSION)

deploy-prod:
	VERSION=$(VERSION) docker compose -f deploy/docker-compose.prod.yml up -d
```

### 11.7 Deployment Summary

| Component | Purpose |
|-----------|---------|
| `install.sh` | One-line installer: `curl ... \| bash` |
| `upgrade.sh` | Zero-downtime upgrade with backup |
| `ci.yml` | Lint, test, build on every PR |
| `release.yml` | Multi-arch images on tag |
| Docker Compose | Single-server with Traefik + Let's Encrypt |

**One-line deployment:**
```bash
curl -fsSL https://raw.githubusercontent.com/yourusername/open-accounting/main/deploy/scripts/install.sh | bash
```

---

## Appendix: Quick Reference

### Technology Stack
- **Backend:** Go 1.22+
- **Frontend:** SvelteKit + TypeScript
- **Database:** PostgreSQL 16+
- **Precision:** NUMERIC(28,8) for money

### Key Design Decisions
1. Schema-per-tenant for multi-tenancy
2. Immutable journal entries (void creates reversal)
3. FIFO inventory costing with lot tracking
4. Date-aware VAT rates (valid_from/valid_to)
5. Estonian payroll tax compliance
6. Estonian e-invoice v1.2 + Peppol BIS 3.0

### API Authentication
- JWT tokens with tenant context
- HMAC-SHA256 for external integrations

### Testing Requirements
- Unit tests for all calculations
- Integration tests with Testcontainers
- E2E tests with Playwright
- 80%+ code coverage target
