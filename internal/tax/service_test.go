package tax

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockRepository implements Repository for testing
type MockRepository struct {
	ensureSchemaErr        error
	queryVATDataResult     []VATAggregateRow
	queryVATDataErr        error
	saveDeclarationErr     error
	getDeclarationResult   *KMDDeclaration
	getDeclarationErr      error
	listDeclarationsResult []KMDDeclaration
	listDeclarationsErr    error
	savedDeclarations      []*KMDDeclaration
}

func (m *MockRepository) EnsureSchema(ctx context.Context, schemaName string) error {
	return m.ensureSchemaErr
}

func (m *MockRepository) QueryVATData(ctx context.Context, schemaName, tenantID string, startDate, endDate time.Time) ([]VATAggregateRow, error) {
	if m.queryVATDataErr != nil {
		return nil, m.queryVATDataErr
	}
	return m.queryVATDataResult, nil
}

func (m *MockRepository) SaveDeclaration(ctx context.Context, schemaName string, decl *KMDDeclaration) error {
	if m.saveDeclarationErr != nil {
		return m.saveDeclarationErr
	}
	m.savedDeclarations = append(m.savedDeclarations, decl)
	return nil
}

func (m *MockRepository) GetDeclaration(ctx context.Context, schemaName, tenantID string, year, month int) (*KMDDeclaration, error) {
	if m.getDeclarationErr != nil {
		return nil, m.getDeclarationErr
	}
	return m.getDeclarationResult, nil
}

func (m *MockRepository) ListDeclarations(ctx context.Context, schemaName, tenantID string) ([]KMDDeclaration, error) {
	if m.listDeclarationsErr != nil {
		return nil, m.listDeclarationsErr
	}
	return m.listDeclarationsResult, nil
}

func TestService_AggregateVATByCode(t *testing.T) {
	rows := aggregateVATByCode([]VATEntry{
		{VATCode: "1", TaxBase: 1000, TaxAmount: 220, IsOutput: true},
		{VATCode: "1", TaxBase: 500, TaxAmount: 110, IsOutput: true},
		{VATCode: "4", TaxBase: 300, TaxAmount: 66, IsOutput: false},
	})

	assert.Len(t, rows, 2)

	// Find row 1 (output VAT)
	var row1 *KMDRow
	for i := range rows {
		if rows[i].Code == "1" {
			row1 = &rows[i]
			break
		}
	}
	assert.NotNil(t, row1)
	assert.Equal(t, "1500", row1.TaxBase.String())
	assert.Equal(t, "330", row1.TaxAmount.String())
}

func TestMapVATRateToKMDCode(t *testing.T) {
	tests := []struct {
		rate     decimal.Decimal
		isOutput bool
		expected string
	}{
		{decimal.NewFromInt(22), true, KMDRow1},
		{decimal.NewFromInt(24), true, KMDRow1},
		{decimal.NewFromInt(20), true, KMDRow1},
		{decimal.NewFromInt(13), true, KMDRow21},
		{decimal.NewFromInt(9), true, KMDRow2},
		{decimal.NewFromInt(0), true, KMDRow3},
		{decimal.NewFromInt(22), false, KMDRow4},
	}

	for _, tt := range tests {
		result := mapVATRateToKMDCode(tt.rate, tt.isOutput)
		assert.Equal(t, tt.expected, result, "rate=%v, isOutput=%v", tt.rate, tt.isOutput)
	}
}

func TestGetKMDRowDescription(t *testing.T) {
	desc := getKMDRowDescription(KMDRow1)
	assert.Contains(t, desc, "standard")

	desc = getKMDRowDescription(KMDRow4)
	assert.Contains(t, desc, "Input VAT")

	desc = getKMDRowDescription("unknown")
	assert.Equal(t, "Unknown", desc)
}

func TestGetKMDRowDescription_AllCodes(t *testing.T) {
	tests := []struct {
		code        string
		shouldExist bool
	}{
		{KMDRow1, true},
		{KMDRow2, true},
		{KMDRow21, true},
		{KMDRow3, true},
		{KMDRow31, true},
		{KMDRow4, true},
		{KMDRow5, true},
		{KMDRow6, true},
		{"nonexistent", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			desc := getKMDRowDescription(tt.code)
			if tt.shouldExist {
				assert.NotEqual(t, "Unknown", desc, "code %s should have a description", tt.code)
			} else {
				assert.Equal(t, "Unknown", desc, "code %s should return Unknown", tt.code)
			}
		})
	}
}

func TestMapVATRateToKMDCode_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		rate     decimal.Decimal
		isOutput bool
		expected string
	}{
		// Boundary testing for rates >= 20 goes to Row1
		{"22% output", decimal.NewFromInt(22), true, KMDRow1},
		{"20% output", decimal.NewFromInt(20), true, KMDRow1},
		{"24% output", decimal.NewFromInt(24), true, KMDRow1},
		{"25% output", decimal.NewFromInt(25), true, KMDRow1},
		// Specific rates
		{"13% output", decimal.NewFromInt(13), true, KMDRow21},
		{"9% output", decimal.NewFromInt(9), true, KMDRow2},
		{"0% output", decimal.NewFromInt(0), true, KMDRow3},
		// Default case: any other rate goes to Row1
		{"5% output (default case)", decimal.NewFromInt(5), true, KMDRow1},
		{"19% output (default case)", decimal.NewFromInt(19), true, KMDRow1},
		{"8.5% output (default case)", decimal.NewFromFloat(8.5), true, KMDRow1},
		{"15% output (default case)", decimal.NewFromInt(15), true, KMDRow1},
		// Input VAT always goes to Row4
		{"22% input", decimal.NewFromInt(22), false, KMDRow4},
		{"0% input", decimal.NewFromInt(0), false, KMDRow4},
		{"9% input", decimal.NewFromInt(9), false, KMDRow4},
		{"13% input", decimal.NewFromInt(13), false, KMDRow4},
		// Decimal rates
		{"22.0% output", decimal.NewFromFloat(22.0), true, KMDRow1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mapVATRateToKMDCode(tt.rate, tt.isOutput)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestAggregateVATByCode_EmptyInput(t *testing.T) {
	rows := aggregateVATByCode([]VATEntry{})
	assert.Len(t, rows, 0)
}

func TestAggregateVATByCode_MultipleEntriesSameCode(t *testing.T) {
	entries := []VATEntry{
		{VATCode: "1", TaxBase: 100, TaxAmount: 22, IsOutput: true},
		{VATCode: "1", TaxBase: 200, TaxAmount: 44, IsOutput: true},
		{VATCode: "1", TaxBase: 300, TaxAmount: 66, IsOutput: true},
	}

	rows := aggregateVATByCode(entries)

	assert.Len(t, rows, 1)
	assert.Equal(t, "600", rows[0].TaxBase.String())
	assert.Equal(t, "132", rows[0].TaxAmount.String())
}

func TestAggregateVATByCode_MixedCodes(t *testing.T) {
	entries := []VATEntry{
		{VATCode: "1", TaxBase: 1000, TaxAmount: 220, IsOutput: true},
		{VATCode: "2", TaxBase: 500, TaxAmount: 45, IsOutput: true},
		{VATCode: "4", TaxBase: 300, TaxAmount: 66, IsOutput: false},
		{VATCode: "1", TaxBase: 500, TaxAmount: 110, IsOutput: true},
		{VATCode: "4", TaxBase: 200, TaxAmount: 44, IsOutput: false},
	}

	rows := aggregateVATByCode(entries)

	assert.Len(t, rows, 3)

	// Find each row and verify
	rowMap := make(map[string]KMDRow)
	for _, row := range rows {
		rowMap[row.Code] = row
	}

	row1 := rowMap["1"]
	assert.Equal(t, "1500", row1.TaxBase.String())
	assert.Equal(t, "330", row1.TaxAmount.String())

	row2 := rowMap["2"]
	assert.Equal(t, "500", row2.TaxBase.String())
	assert.Equal(t, "45", row2.TaxAmount.String())

	row4 := rowMap["4"]
	assert.Equal(t, "500", row4.TaxBase.String())
	assert.Equal(t, "110", row4.TaxAmount.String())
}

func TestAggregateVATByCode_DecimalAmounts(t *testing.T) {
	entries := []VATEntry{
		{VATCode: "1", TaxBase: 100.50, TaxAmount: 22.11, IsOutput: true},
		{VATCode: "1", TaxBase: 200.25, TaxAmount: 44.055, IsOutput: true},
	}

	rows := aggregateVATByCode(entries)

	assert.Len(t, rows, 1)
	expected := decimal.NewFromFloat(100.50 + 200.25)
	assert.True(t, rows[0].TaxBase.Equal(expected))
}

// Service tests using MockRepository

func TestService_GenerateKMD_Success(t *testing.T) {
	repo := &MockRepository{
		queryVATDataResult: []VATAggregateRow{
			{VATRate: decimal.NewFromInt(22), IsOutput: true, TaxBase: decimal.NewFromInt(1000), TaxAmount: decimal.NewFromInt(220)},
			{VATRate: decimal.NewFromInt(22), IsOutput: false, TaxBase: decimal.NewFromInt(500), TaxAmount: decimal.NewFromInt(110)},
		},
	}
	svc := NewServiceWithRepository(repo)

	req := &CreateKMDRequest{Year: 2024, Month: 1}
	decl, err := svc.GenerateKMD(context.Background(), "tenant-1", "test_schema", req)

	require.NoError(t, err)
	require.NotNil(t, decl)
	assert.Equal(t, 2024, decl.Year)
	assert.Equal(t, 1, decl.Month)
	assert.Equal(t, "DRAFT", decl.Status)
	assert.Equal(t, "220", decl.TotalOutputVAT.String())
	assert.Equal(t, "110", decl.TotalInputVAT.String())
	assert.Len(t, decl.Rows, 2)
	assert.Len(t, repo.savedDeclarations, 1)
}

func TestService_GenerateKMD_EnsureSchemaError(t *testing.T) {
	repo := &MockRepository{
		ensureSchemaErr: errors.New("schema error"),
	}
	svc := NewServiceWithRepository(repo)

	req := &CreateKMDRequest{Year: 2024, Month: 1}
	_, err := svc.GenerateKMD(context.Background(), "tenant-1", "test_schema", req)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "ensure schema")
}

func TestService_GenerateKMD_QueryVATDataError(t *testing.T) {
	repo := &MockRepository{
		queryVATDataErr: errors.New("query error"),
	}
	svc := NewServiceWithRepository(repo)

	req := &CreateKMDRequest{Year: 2024, Month: 1}
	_, err := svc.GenerateKMD(context.Background(), "tenant-1", "test_schema", req)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "query VAT data")
}

func TestService_GenerateKMD_SaveDeclarationError(t *testing.T) {
	repo := &MockRepository{
		queryVATDataResult: []VATAggregateRow{},
		saveDeclarationErr: errors.New("save error"),
	}
	svc := NewServiceWithRepository(repo)

	req := &CreateKMDRequest{Year: 2024, Month: 1}
	_, err := svc.GenerateKMD(context.Background(), "tenant-1", "test_schema", req)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "save declaration")
}

func TestService_GenerateKMD_EmptyVATData(t *testing.T) {
	repo := &MockRepository{
		queryVATDataResult: []VATAggregateRow{},
	}
	svc := NewServiceWithRepository(repo)

	req := &CreateKMDRequest{Year: 2024, Month: 1}
	decl, err := svc.GenerateKMD(context.Background(), "tenant-1", "test_schema", req)

	require.NoError(t, err)
	require.NotNil(t, decl)
	assert.True(t, decl.TotalOutputVAT.IsZero())
	assert.True(t, decl.TotalInputVAT.IsZero())
	assert.Len(t, decl.Rows, 0)
}

func TestService_GetKMD_Success(t *testing.T) {
	now := time.Now()
	repo := &MockRepository{
		getDeclarationResult: &KMDDeclaration{
			ID:             "decl-1",
			TenantID:       "tenant-1",
			Year:           2024,
			Month:          1,
			Status:         "DRAFT",
			TotalOutputVAT: decimal.NewFromInt(220),
			TotalInputVAT:  decimal.NewFromInt(110),
			CreatedAt:      now,
			UpdatedAt:      now,
		},
	}
	svc := NewServiceWithRepository(repo)

	decl, err := svc.GetKMD(context.Background(), "tenant-1", "test_schema", "2024", "1")

	require.NoError(t, err)
	require.NotNil(t, decl)
	assert.Equal(t, "decl-1", decl.ID)
	assert.Equal(t, 2024, decl.Year)
	assert.Equal(t, 1, decl.Month)
}

func TestService_GetKMD_InvalidYear(t *testing.T) {
	repo := &MockRepository{}
	svc := NewServiceWithRepository(repo)

	_, err := svc.GetKMD(context.Background(), "tenant-1", "test_schema", "invalid", "1")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid year")
}

func TestService_GetKMD_InvalidMonth(t *testing.T) {
	repo := &MockRepository{}
	svc := NewServiceWithRepository(repo)

	_, err := svc.GetKMD(context.Background(), "tenant-1", "test_schema", "2024", "invalid")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid month")
}

func TestService_GetKMD_NotFound(t *testing.T) {
	repo := &MockRepository{
		getDeclarationErr: errors.New("not found"),
	}
	svc := NewServiceWithRepository(repo)

	_, err := svc.GetKMD(context.Background(), "tenant-1", "test_schema", "2024", "1")

	require.Error(t, err)
}

func TestService_ListKMD_Success(t *testing.T) {
	now := time.Now()
	repo := &MockRepository{
		listDeclarationsResult: []KMDDeclaration{
			{ID: "decl-1", TenantID: "tenant-1", Year: 2024, Month: 2, Status: "DRAFT", CreatedAt: now, UpdatedAt: now},
			{ID: "decl-2", TenantID: "tenant-1", Year: 2024, Month: 1, Status: "SUBMITTED", CreatedAt: now, UpdatedAt: now},
		},
	}
	svc := NewServiceWithRepository(repo)

	declarations, err := svc.ListKMD(context.Background(), "tenant-1", "test_schema")

	require.NoError(t, err)
	assert.Len(t, declarations, 2)
	assert.Equal(t, "decl-1", declarations[0].ID)
	assert.Equal(t, "decl-2", declarations[1].ID)
}

func TestService_ListKMD_Empty(t *testing.T) {
	repo := &MockRepository{
		listDeclarationsResult: []KMDDeclaration{},
	}
	svc := NewServiceWithRepository(repo)

	declarations, err := svc.ListKMD(context.Background(), "tenant-1", "test_schema")

	require.NoError(t, err)
	assert.Len(t, declarations, 0)
}

func TestService_ListKMD_Error(t *testing.T) {
	repo := &MockRepository{
		listDeclarationsErr: errors.New("list error"),
	}
	svc := NewServiceWithRepository(repo)

	_, err := svc.ListKMD(context.Background(), "tenant-1", "test_schema")

	require.Error(t, err)
}

func TestService_EnsureSchema_Success(t *testing.T) {
	repo := &MockRepository{}
	svc := NewServiceWithRepository(repo)

	err := svc.EnsureSchema(context.Background(), "test_schema")

	require.NoError(t, err)
}

func TestService_EnsureSchema_Error(t *testing.T) {
	repo := &MockRepository{
		ensureSchemaErr: errors.New("schema error"),
	}
	svc := NewServiceWithRepository(repo)

	err := svc.EnsureSchema(context.Background(), "test_schema")

	require.Error(t, err)
}

// TestNewService_WithNilPool tests the NewService constructor with a nil pool
func TestNewService_WithNilPool(t *testing.T) {
	// NewService should create a service with nil pool (won't panic until used)
	svc := NewService(nil)
	require.NotNil(t, svc)
	assert.NotNil(t, svc.repo)
}
