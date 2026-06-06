package tax

import (
	"context"
	"testing"
	"time"

	"github.com/HMB-research/open-accounting/internal/database"
	"github.com/HMB-research/open-accounting/internal/testutil"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

func TestRepository_TenantBootstrapCreatesKMDTables(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	ctx := context.Background()

	for _, tableName := range []string{"kmd_declarations", "kmd_rows"} {
		table, err := database.QualifiedTable(tenant.SchemaName, tableName)
		if err != nil {
			t.Fatalf("qualified %s table: %v", tableName, err)
		}

		var count int
		if err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM "+table).Scan(&count); err != nil {
			t.Fatalf("%s table not created by tenant bootstrap: %v", tableName, err)
		}
	}

	uniqueIndex := "idx_kmd_declarations_tenant"
	var exists bool
	err := pool.QueryRow(ctx, "SELECT to_regclass($1) IS NOT NULL", tenant.SchemaName+"."+uniqueIndex).Scan(&exists)
	if err != nil {
		t.Fatalf("query KMD declaration tenant index: %v", err)
	}
	if !exists {
		t.Fatalf("KMD declaration tenant index %s missing", uniqueIndex)
	}
}

func TestRepository_SaveDeclaration(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	repo := newTaxGORMRepository(t, pool)
	ctx := context.Background()

	// Create KMD declaration
	now := time.Now()
	decl := &KMDDeclaration{
		ID:             uuid.New().String(),
		TenantID:       tenant.ID,
		Year:           now.Year(),
		Month:          int(now.Month()),
		TotalOutputVAT: decimal.NewFromFloat(1000),
		TotalInputVAT:  decimal.NewFromFloat(300),
		Status:         "DRAFT",
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	err := repo.SaveDeclaration(ctx, tenant.SchemaName, decl)
	if err != nil {
		t.Fatalf("SaveDeclaration failed: %v", err)
	}

	// Verify it was created
	retrieved, err := repo.GetDeclaration(ctx, tenant.SchemaName, tenant.ID, now.Year(), int(now.Month()))
	if err != nil {
		t.Fatalf("GetDeclaration failed: %v", err)
	}

	if retrieved == nil {
		t.Fatal("expected KMD declaration, got nil")
	}

	if !retrieved.TotalOutputVAT.Equal(decl.TotalOutputVAT) {
		t.Errorf("expected TotalOutputVAT %s, got %s", decl.TotalOutputVAT, retrieved.TotalOutputVAT)
	}
	if !retrieved.TotalInputVAT.Equal(decl.TotalInputVAT) {
		t.Errorf("expected TotalInputVAT %s, got %s", decl.TotalInputVAT, retrieved.TotalInputVAT)
	}
}

func TestRepository_ListDeclarations(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	repo := newTaxGORMRepository(t, pool)
	ctx := context.Background()

	// Create multiple KMD declarations
	now := time.Now()
	for i := 1; i <= 3; i++ {
		decl := &KMDDeclaration{
			ID:             uuid.New().String(),
			TenantID:       tenant.ID,
			Year:           now.Year(),
			Month:          i,
			TotalOutputVAT: decimal.NewFromFloat(float64(i * 100)),
			TotalInputVAT:  decimal.NewFromFloat(float64(i * 30)),
			Status:         "DRAFT",
			CreatedAt:      now,
			UpdatedAt:      now,
		}

		err := repo.SaveDeclaration(ctx, tenant.SchemaName, decl)
		if err != nil {
			t.Fatalf("SaveDeclaration for month %d failed: %v", i, err)
		}
	}

	// List all declarations
	declarations, err := repo.ListDeclarations(ctx, tenant.SchemaName, tenant.ID)
	if err != nil {
		t.Fatalf("ListDeclarations failed: %v", err)
	}

	if len(declarations) != 3 {
		t.Errorf("expected 3 declarations, got %d", len(declarations))
	}
}

func TestRepository_QueryKMDINFData(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	repo := newTaxGORMRepository(t, pool)
	ctx := context.Background()

	salesContactID := uuid.New().String()
	purchaseContactID := uuid.New().String()
	smallContactID := uuid.New().String()
	foreignContactID := uuid.New().String()
	_, err := pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.contacts (id, tenant_id, name, contact_type, reg_code, country_code, is_active, created_at, updated_at)
		VALUES
			($1, $5, 'Sales Partner', 'CUSTOMER', '12345678', 'EE', true, NOW(), NOW()),
			($2, $5, 'Purchase Partner', 'SUPPLIER', '87654321', 'EE', true, NOW(), NOW()),
			($3, $5, 'Small Partner', 'CUSTOMER', '11111111', 'EE', true, NOW(), NOW()),
			($4, $5, 'Foreign Partner', 'CUSTOMER', '22222222', 'FI', true, NOW(), NOW())
	`, salesContactID, purchaseContactID, smallContactID, foreignContactID, tenant.ID)
	if err != nil {
		t.Fatalf("insert contacts: %v", err)
	}

	insertInvoice := func(number, invoiceType, contactID string, issueDate time.Time, subtotal, vat, total decimal.Decimal) {
		t.Helper()
		_, err := pool.Exec(ctx, `
			INSERT INTO `+tenant.SchemaName+`.invoices (
				id, tenant_id, invoice_number, invoice_type, contact_id, issue_date, due_date,
				currency, exchange_rate, subtotal, vat_amount, total,
				base_subtotal, base_vat_amount, base_total, amount_paid, status,
				created_by, created_at, updated_at
			) VALUES ($1, $2, $3, $4, $5, $6, $6, 'EUR', 1, $7, $8, $9, $7, $8, $9, 0, 'SENT', $10, NOW(), NOW())
		`, uuid.New().String(), tenant.ID, number, invoiceType, contactID, issueDate, subtotal, vat, total, uuid.New().String())
		if err != nil {
			t.Fatalf("insert invoice %s: %v", number, err)
		}
	}

	periodDate := time.Date(2026, 3, 5, 0, 0, 0, 0, time.UTC)
	insertInvoice("S-1", "SALES", salesContactID, periodDate, decimal.NewFromInt(600), decimal.NewFromInt(132), decimal.NewFromInt(732))
	insertInvoice("S-2", "SALES", salesContactID, periodDate.AddDate(0, 0, 1), decimal.NewFromInt(500), decimal.NewFromInt(110), decimal.NewFromInt(610))
	insertInvoice("P-1", "PURCHASE", purchaseContactID, periodDate, decimal.NewFromInt(1200), decimal.NewFromInt(264), decimal.NewFromInt(1464))
	insertInvoice("SMALL-1", "SALES", smallContactID, periodDate, decimal.NewFromInt(500), decimal.NewFromInt(110), decimal.NewFromInt(610))
	insertInvoice("FOREIGN-1", "SALES", foreignContactID, periodDate, decimal.NewFromInt(2000), decimal.NewFromInt(440), decimal.NewFromInt(2440))

	rows, err := repo.QueryKMDINFData(
		ctx,
		tenant.SchemaName,
		tenant.ID,
		time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
		decimal.NewFromInt(1000),
	)
	if err != nil {
		t.Fatalf("QueryKMDINFData failed: %v", err)
	}

	if len(rows) != 3 {
		t.Fatalf("expected 3 KMD INF rows, got %d", len(rows))
	}
	if rows[0].Part != KMDINFPartSales || rows[0].ContactName != "Sales Partner" {
		t.Fatalf("expected sales partner first, got %#v", rows[0])
	}
	if !rows[0].PartnerPeriodTaxableAmount.Equal(decimal.NewFromInt(1100)) {
		t.Fatalf("expected sales partner period amount 1100, got %s", rows[0].PartnerPeriodTaxableAmount)
	}
	if rows[2].Part != KMDINFPartPurchases || rows[2].ContactName != "Purchase Partner" {
		t.Fatalf("expected purchase partner third, got %#v", rows[2])
	}
}

func TestRepository_QueryEUVATOSSData(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	repo := newTaxGORMRepository(t, pool)
	ctx := context.Background()

	deContactID := uuid.New().String()
	fiContactID := uuid.New().String()
	eeContactID := uuid.New().String()
	noContactID := uuid.New().String()
	_, err := pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.contacts (id, tenant_id, name, contact_type, vat_number, country_code, is_active, created_at, updated_at)
		VALUES
			($1, $5, 'German Consumer', 'CUSTOMER', '', 'DE', true, NOW(), NOW()),
			($2, $5, 'Finnish VAT Customer', 'CUSTOMER', 'FI12345678', 'FI', true, NOW(), NOW()),
			($3, $5, 'Estonian Customer', 'CUSTOMER', '', 'EE', true, NOW(), NOW()),
			($4, $5, 'Norwegian Customer', 'CUSTOMER', '', 'NO', true, NOW(), NOW())
	`, deContactID, fiContactID, eeContactID, noContactID, tenant.ID)
	if err != nil {
		t.Fatalf("insert contacts: %v", err)
	}

	insertInvoiceLine := func(number, contactID, status string, issueDate time.Time, vatRate, subtotal, vat, total decimal.Decimal) {
		t.Helper()
		invoiceID := uuid.New().String()
		_, err := pool.Exec(ctx, `
			INSERT INTO `+tenant.SchemaName+`.invoices (
				id, tenant_id, invoice_number, invoice_type, contact_id, issue_date, due_date,
				currency, exchange_rate, subtotal, vat_amount, total,
				base_subtotal, base_vat_amount, base_total, amount_paid, status,
				created_by, created_at, updated_at
			) VALUES ($1, $2, $3, 'SALES', $4, $5, $5, 'EUR', 1, $6, $7, $8, $6, $7, $8, 0, $9, $10, NOW(), NOW())
		`, invoiceID, tenant.ID, number, contactID, issueDate, subtotal, vat, total, status, uuid.New().String())
		if err != nil {
			t.Fatalf("insert invoice %s: %v", number, err)
		}
		_, err = pool.Exec(ctx, `
			INSERT INTO `+tenant.SchemaName+`.invoice_lines (
				id, tenant_id, invoice_id, line_number, description, quantity, unit_price,
				discount_percent, vat_rate, vat_treatment, line_subtotal, line_vat, line_total
			) VALUES ($1, $2, $3, 1, 'OSS sale', 1, $4, 0, $5, 'STANDARD', $6, $7, $8)
		`, uuid.New().String(), tenant.ID, invoiceID, subtotal, vatRate, subtotal, vat, total)
		if err != nil {
			t.Fatalf("insert invoice line %s: %v", number, err)
		}
	}

	q1Date := time.Date(2026, 2, 5, 0, 0, 0, 0, time.UTC)
	insertInvoiceLine("DE-1", deContactID, "SENT", q1Date, decimal.NewFromInt(19), decimal.NewFromInt(100), decimal.NewFromInt(19), decimal.NewFromInt(119))
	insertInvoiceLine("FI-1", fiContactID, "SENT", q1Date, decimal.NewFromInt(24), decimal.NewFromInt(200), decimal.NewFromInt(48), decimal.NewFromInt(248))
	insertInvoiceLine("EE-1", eeContactID, "SENT", q1Date, decimal.NewFromInt(22), decimal.NewFromInt(100), decimal.NewFromInt(22), decimal.NewFromInt(122))
	insertInvoiceLine("NO-1", noContactID, "SENT", q1Date, decimal.NewFromInt(25), decimal.NewFromInt(100), decimal.NewFromInt(25), decimal.NewFromInt(125))
	insertInvoiceLine("DE-Q2", deContactID, "SENT", time.Date(2026, 4, 5, 0, 0, 0, 0, time.UTC), decimal.NewFromInt(19), decimal.NewFromInt(100), decimal.NewFromInt(19), decimal.NewFromInt(119))
	insertInvoiceLine("DE-DRAFT", deContactID, "DRAFT", q1Date, decimal.NewFromInt(19), decimal.NewFromInt(100), decimal.NewFromInt(19), decimal.NewFromInt(119))

	rows, err := repo.QueryEUVATOSSData(
		ctx,
		tenant.SchemaName,
		tenant.ID,
		time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
		false,
	)
	if err != nil {
		t.Fatalf("QueryEUVATOSSData failed: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected one B2C OSS row, got %d: %#v", len(rows), rows)
	}
	if rows[0].CountryCode != "DE" || !rows[0].VATAmount.Equal(decimal.NewFromInt(19)) {
		t.Fatalf("expected German OSS row with 19 VAT, got %#v", rows[0])
	}

	rows, err = repo.QueryEUVATOSSData(
		ctx,
		tenant.SchemaName,
		tenant.ID,
		time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
		true,
	)
	if err != nil {
		t.Fatalf("QueryEUVATOSSData include B2B failed: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected B2C and VAT-registered EU rows, got %d: %#v", len(rows), rows)
	}
	if rows[1].CountryCode != "FI" || !rows[1].TaxableAmount.Equal(decimal.NewFromInt(200)) {
		t.Fatalf("expected Finnish include-B2B row, got %#v", rows[1])
	}
}

func TestRepository_QueryVATDataIncludesReverseChargePurchases(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	repo := newTaxGORMRepository(t, pool)
	ctx := context.Background()

	contactID := uuid.New().String()
	userID := uuid.New().String()
	invoiceID := uuid.New().String()
	_, err := pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.contacts (id, tenant_id, name, contact_type, reg_code, country_code, is_active, created_at, updated_at)
		VALUES ($1, $2, 'EU Supplier', 'SUPPLIER', 'DE123456789', 'DE', true, NOW(), NOW())
	`, contactID, tenant.ID)
	if err != nil {
		t.Fatalf("insert contact: %v", err)
	}

	periodDate := time.Date(2026, 3, 5, 0, 0, 0, 0, time.UTC)
	_, err = pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.invoices (
			id, tenant_id, invoice_number, invoice_type, contact_id, issue_date, due_date,
			currency, exchange_rate, subtotal, vat_amount, total,
			base_subtotal, base_vat_amount, base_total, amount_paid, status,
			created_by, created_at, updated_at
		) VALUES ($1, $2, 'BILL-RC-1', 'PURCHASE', $3, $4, $4, 'EUR', 1, 100, 0, 100, 100, 0, 100, 0, 'SENT', $5, NOW(), NOW())
	`, invoiceID, tenant.ID, contactID, periodDate, userID)
	if err != nil {
		t.Fatalf("insert invoice: %v", err)
	}
	_, err = pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.invoice_lines (
			id, tenant_id, invoice_id, line_number, description, quantity, unit_price,
			discount_percent, vat_rate, vat_treatment, line_subtotal, line_vat, line_total
		) VALUES ($1, $2, $3, 1, 'EU service', 1, 100, 0, 22, 'REVERSE_CHARGE', 100, 0, 100)
	`, uuid.New().String(), tenant.ID, invoiceID)
	if err != nil {
		t.Fatalf("insert invoice line: %v", err)
	}

	rows, err := repo.QueryVATData(
		ctx,
		tenant.SchemaName,
		tenant.ID,
		time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 3, 31, 23, 59, 59, 0, time.UTC),
	)
	if err != nil {
		t.Fatalf("QueryVATData failed: %v", err)
	}

	byOutput := make(map[bool]VATAggregateRow)
	for _, row := range rows {
		byOutput[row.IsOutput] = row
	}
	output, ok := byOutput[true]
	if !ok {
		t.Fatal("expected reverse-charge output VAT row")
	}
	input, ok := byOutput[false]
	if !ok {
		t.Fatal("expected reverse-charge input VAT row")
	}
	if !output.TaxBase.Equal(decimal.NewFromInt(100)) || !output.TaxAmount.Equal(decimal.NewFromInt(22)) {
		t.Fatalf("unexpected output row: %#v", output)
	}
	if !input.TaxBase.Equal(decimal.NewFromInt(100)) || !input.TaxAmount.Equal(decimal.NewFromInt(22)) {
		t.Fatalf("unexpected input row: %#v", input)
	}
}

func TestRepository_GetDeclaration_NotFound(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	repo := newTaxGORMRepository(t, pool)
	ctx := context.Background()

	// Try to get non-existent KMD
	decl, err := repo.GetDeclaration(ctx, tenant.SchemaName, tenant.ID, 2099, 12)
	if err != nil {
		t.Fatalf("GetDeclaration failed: %v", err)
	}

	if decl != nil {
		t.Error("expected nil for non-existent KMD, got a declaration")
	}
}

func TestRepository_SaveDeclarationWithRows(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	repo := newTaxGORMRepository(t, pool)
	ctx := context.Background()

	// Create KMD declaration with rows
	now := time.Now()
	decl := &KMDDeclaration{
		ID:             uuid.New().String(),
		TenantID:       tenant.ID,
		Year:           now.Year(),
		Month:          6, // Use a different month to avoid conflicts
		TotalOutputVAT: decimal.NewFromFloat(2200),
		TotalInputVAT:  decimal.NewFromFloat(500),
		Status:         "DRAFT",
		Rows: []KMDRow{
			{Code: KMDRow1, Description: "Standard rate sales (22%)", TaxBase: decimal.NewFromFloat(10000), TaxAmount: decimal.NewFromFloat(2200)},
			{Code: KMDRow4, Description: "Input VAT on domestic purchases", TaxBase: decimal.NewFromFloat(2272.73), TaxAmount: decimal.NewFromFloat(500)},
		},
		CreatedAt: now,
		UpdatedAt: now,
	}

	err := repo.SaveDeclaration(ctx, tenant.SchemaName, decl)
	if err != nil {
		t.Fatalf("SaveDeclaration with rows failed: %v", err)
	}

	// Retrieve and verify rows
	retrieved, err := repo.GetDeclaration(ctx, tenant.SchemaName, tenant.ID, now.Year(), 6)
	if err != nil {
		t.Fatalf("GetDeclaration failed: %v", err)
	}

	if retrieved == nil {
		t.Fatal("expected KMD declaration, got nil")
	}

	if len(retrieved.Rows) != 2 {
		t.Errorf("expected 2 rows, got %d", len(retrieved.Rows))
	}

	// Verify row content
	if len(retrieved.Rows) > 0 {
		row1 := retrieved.Rows[0]
		if row1.Code != KMDRow1 {
			t.Errorf("expected row code %s, got %s", KMDRow1, row1.Code)
		}
		if !row1.TaxAmount.Equal(decimal.NewFromFloat(2200)) {
			t.Errorf("expected tax amount 2200, got %s", row1.TaxAmount)
		}
	}
}

func TestRepository_QueryVATData(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	repo := newTaxGORMRepository(t, pool)
	ctx := context.Background()

	// Create revenue account (for output VAT) with unique code
	revenueAccountID := uuid.New().String()
	uniqueCode1 := "49" + uuid.New().String()[:8]
	_, err := pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.accounts
		(id, tenant_id, code, name, account_type, is_active, created_at)
		VALUES ($1, $2, $3, 'Sales Revenue', 'REVENUE', true, NOW())
	`, revenueAccountID, tenant.ID, uniqueCode1)
	if err != nil {
		t.Fatalf("Failed to create revenue account: %v", err)
	}

	// Create expense account (for input VAT) with unique code
	expenseAccountID := uuid.New().String()
	uniqueCode2 := "59" + uuid.New().String()[:8]
	_, err = pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.accounts
		(id, tenant_id, code, name, account_type, is_active, created_at)
		VALUES ($1, $2, $3, 'Operating Expenses', 'EXPENSE', true, NOW())
	`, expenseAccountID, tenant.ID, uniqueCode2)
	if err != nil {
		t.Fatalf("Failed to create expense account: %v", err)
	}

	// Create a POSTED journal entry
	journalEntryID := uuid.New().String()
	createdBy := uuid.New().String()
	entryDate := time.Now().AddDate(0, 0, -5)
	_, err = pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.journal_entries
		(id, tenant_id, entry_number, entry_date, reference, description, status, created_at, created_by)
		VALUES ($1, $2, 'JE-00001', $3, 'VAT-TEST-001', 'VAT Test Entry', 'POSTED', NOW(), $4)
	`, journalEntryID, tenant.ID, entryDate, createdBy)
	if err != nil {
		t.Fatalf("Failed to create journal entry: %v", err)
	}

	// Create journal entry lines with VAT rates
	// Revenue line with 22% VAT (output VAT) - credit for revenue
	revenueLine := uuid.New().String()
	_, err = pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.journal_entry_lines
		(id, tenant_id, journal_entry_id, account_id, description, debit_amount, credit_amount, vat_rate)
		VALUES ($1, $2, $3, $4, 'Revenue with 22% VAT', 0, 1000, 22)
	`, revenueLine, tenant.ID, journalEntryID, revenueAccountID)
	if err != nil {
		t.Fatalf("Failed to create revenue journal line: %v", err)
	}

	// Expense line with 9% VAT (input VAT) - debit for expense
	expenseLine := uuid.New().String()
	_, err = pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.journal_entry_lines
		(id, tenant_id, journal_entry_id, account_id, description, debit_amount, credit_amount, vat_rate)
		VALUES ($1, $2, $3, $4, 'Expense with 9% VAT', 500, 0, 9)
	`, expenseLine, tenant.ID, journalEntryID, expenseAccountID)
	if err != nil {
		t.Fatalf("Failed to create expense journal line: %v", err)
	}

	// Query VAT data
	startDate := entryDate.AddDate(0, 0, -1)
	endDate := time.Now()
	vatData, err := repo.QueryVATData(ctx, tenant.SchemaName, tenant.ID, startDate, endDate)
	if err != nil {
		t.Fatalf("QueryVATData failed: %v", err)
	}

	// Should have aggregated VAT data
	if len(vatData) == 0 {
		t.Error("expected VAT data rows, got none")
	}

	// Log results for debugging
	t.Logf("Found %d VAT aggregate rows", len(vatData))
	for i, row := range vatData {
		t.Logf("Row %d: VATRate=%s, IsOutput=%v, TaxBase=%s, TaxAmount=%s",
			i, row.VATRate, row.IsOutput, row.TaxBase, row.TaxAmount)
	}

	// Verify we have output VAT (from revenue account)
	var hasOutputVAT bool
	for _, row := range vatData {
		if row.IsOutput {
			hasOutputVAT = true
			if !row.VATRate.Equal(decimal.NewFromInt(22)) {
				t.Errorf("expected 22%% VAT rate for output, got %s", row.VATRate)
			}
		}
	}

	if !hasOutputVAT {
		t.Error("expected at least one output VAT row")
	}
}

func TestRepository_QueryVATData_Empty(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	repo := newTaxGORMRepository(t, pool)
	ctx := context.Background()

	// Query VAT data for period with no entries
	startDate := time.Now().AddDate(-1, 0, 0)
	endDate := time.Now().AddDate(-1, 1, 0)
	vatData, err := repo.QueryVATData(ctx, tenant.SchemaName, tenant.ID, startDate, endDate)
	if err != nil {
		t.Fatalf("QueryVATData failed: %v", err)
	}

	// Should return empty slice (not error) for no data
	if len(vatData) != 0 {
		t.Errorf("expected 0 VAT data rows for empty period, got %d", len(vatData))
	}
}

func TestRepository_SaveDeclaration_Update(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	repo := newTaxGORMRepository(t, pool)
	ctx := context.Background()

	now := time.Now()

	// Create initial declaration
	decl := &KMDDeclaration{
		ID:             uuid.New().String(),
		TenantID:       tenant.ID,
		Year:           now.Year(),
		Month:          7, // Use unique month
		TotalOutputVAT: decimal.NewFromFloat(1000),
		TotalInputVAT:  decimal.NewFromFloat(200),
		Status:         "DRAFT",
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	err := repo.SaveDeclaration(ctx, tenant.SchemaName, decl)
	if err != nil {
		t.Fatalf("SaveDeclaration (initial) failed: %v", err)
	}

	// Update with new values (upsert)
	decl.TotalOutputVAT = decimal.NewFromFloat(1500)
	decl.TotalInputVAT = decimal.NewFromFloat(350)
	decl.Status = "SUBMITTED"
	decl.UpdatedAt = time.Now()

	err = repo.SaveDeclaration(ctx, tenant.SchemaName, decl)
	if err != nil {
		t.Fatalf("SaveDeclaration (update) failed: %v", err)
	}

	// Verify update
	retrieved, err := repo.GetDeclaration(ctx, tenant.SchemaName, tenant.ID, now.Year(), 7)
	if err != nil {
		t.Fatalf("GetDeclaration failed: %v", err)
	}

	if retrieved.Status != "SUBMITTED" {
		t.Errorf("expected status SUBMITTED, got %s", retrieved.Status)
	}
	if !retrieved.TotalOutputVAT.Equal(decimal.NewFromFloat(1500)) {
		t.Errorf("expected TotalOutputVAT 1500, got %s", retrieved.TotalOutputVAT)
	}
}

func TestRepository_SaveDeclaration_UpdateWithDifferentIDReplacesRows(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	repo := newTaxGORMRepository(t, pool)
	ctx := context.Background()

	now := time.Now().UTC()
	originalID := uuid.New().String()
	decl := &KMDDeclaration{
		ID:             originalID,
		TenantID:       tenant.ID,
		Year:           now.Year(),
		Month:          8,
		TotalOutputVAT: decimal.NewFromFloat(220),
		TotalInputVAT:  decimal.NewFromFloat(0),
		Status:         "DRAFT",
		Rows: []KMDRow{{
			Code:        KMDRow1,
			Description: "Taxable sales",
			TaxBase:     decimal.NewFromFloat(1000),
			TaxAmount:   decimal.NewFromFloat(220),
		}},
		CreatedAt: now,
		UpdatedAt: now,
	}

	err := repo.SaveDeclaration(ctx, tenant.SchemaName, decl)
	if err != nil {
		t.Fatalf("SaveDeclaration (initial) failed: %v", err)
	}

	submittedAt := now.AddDate(0, 1, 0)
	replacement := &KMDDeclaration{
		ID:             uuid.New().String(),
		TenantID:       tenant.ID,
		Year:           now.Year(),
		Month:          8,
		TotalOutputVAT: decimal.NewFromFloat(0),
		TotalInputVAT:  decimal.NewFromFloat(80),
		Status:         "ACCEPTED",
		SubmittedAt:    &submittedAt,
		Rows: []KMDRow{{
			Code:        KMDRow4,
			Description: "Input VAT",
			TaxBase:     decimal.NewFromFloat(363.64),
			TaxAmount:   decimal.NewFromFloat(80),
		}},
		CreatedAt: now,
		UpdatedAt: now,
	}

	err = repo.SaveDeclaration(ctx, tenant.SchemaName, replacement)
	if err != nil {
		t.Fatalf("SaveDeclaration (replacement) failed: %v", err)
	}
	if replacement.ID != originalID {
		t.Fatalf("expected replacement ID to reuse existing declaration ID %s, got %s", originalID, replacement.ID)
	}

	retrieved, err := repo.GetDeclaration(ctx, tenant.SchemaName, tenant.ID, now.Year(), 8)
	if err != nil {
		t.Fatalf("GetDeclaration failed: %v", err)
	}
	if retrieved == nil {
		t.Fatal("expected KMD declaration, got nil")
	}
	if retrieved.Status != "ACCEPTED" {
		t.Errorf("expected status ACCEPTED, got %s", retrieved.Status)
	}
	if retrieved.SubmittedAt == nil {
		t.Fatal("expected submitted_at to be persisted")
	}
	if len(retrieved.Rows) != 1 || retrieved.Rows[0].Code != KMDRow4 {
		t.Fatalf("expected old rows to be replaced with row 4, got %#v", retrieved.Rows)
	}
}

func TestRepository_ListDeclarations_Empty(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	repo := newTaxGORMRepository(t, pool)
	ctx := context.Background()

	// List declarations for tenant with no declarations
	declarations, err := repo.ListDeclarations(ctx, tenant.SchemaName, tenant.ID)
	if err != nil {
		t.Fatalf("ListDeclarations returned error: %v", err)
	}

	// Should return empty slice (not nil)
	if declarations == nil {
		t.Error("expected empty slice, got nil")
	}
	if len(declarations) != 0 {
		t.Errorf("expected 0 declarations, got %d", len(declarations))
	}
}

func TestNewService(t *testing.T) {
	pool := testutil.SetupTestDB(t)

	// Test the NewService constructor
	service := NewService(pool)
	if service == nil {
		t.Fatal("NewService returned nil")
	}

	// Verify service has a valid repo
	if service.repo == nil {
		t.Error("service.repo is nil")
	}
}

func newTaxGORMRepository(t *testing.T, pool *pgxpool.Pool) *GORMRepository {
	t.Helper()

	gormDB, err := database.NewGormDBFromPool(context.Background(), pool)
	if err != nil {
		t.Fatalf("create gorm repository: %v", err)
	}
	return NewGORMRepository(gormDB)
}
