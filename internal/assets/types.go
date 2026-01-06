package assets

import (
	"errors"
	"time"

	"github.com/shopspring/decimal"
)

// AssetStatus represents the status of a fixed asset
type AssetStatus string

const (
	AssetStatusDraft    AssetStatus = "DRAFT"
	AssetStatusActive   AssetStatus = "ACTIVE"
	AssetStatusDisposed AssetStatus = "DISPOSED"
	AssetStatusSold     AssetStatus = "SOLD"
)

// DepreciationMethod represents how an asset is depreciated
type DepreciationMethod string

const (
	DepreciationStraightLine     DepreciationMethod = "STRAIGHT_LINE"
	DepreciationDecliningBalance DepreciationMethod = "DECLINING_BALANCE"
	DepreciationUnitsOfProd      DepreciationMethod = "UNITS_OF_PRODUCTION"
)

// DisposalMethod represents how an asset was disposed
type DisposalMethod string

const (
	DisposalSold     DisposalMethod = "SOLD"
	DisposalScrapped DisposalMethod = "SCRAPPED"
	DisposalDonated  DisposalMethod = "DONATED"
	DisposalLost     DisposalMethod = "LOST"
)

// AssetCategory represents a category of fixed assets
type AssetCategory struct {
	ID                            string             `json:"id"`
	TenantID                      string             `json:"tenant_id"`
	Name                          string             `json:"name"`
	Description                   string             `json:"description,omitempty"`
	DepreciationMethod            DepreciationMethod `json:"depreciation_method"`
	DefaultUsefulLifeMonths       int                `json:"default_useful_life_months"`
	DefaultResidualValuePercent   decimal.Decimal    `json:"default_residual_value_percent"`
	AssetAccountID                *string            `json:"asset_account_id,omitempty"`
	DepreciationExpenseAccountID  *string            `json:"depreciation_expense_account_id,omitempty"`
	AccumulatedDepreciationAcctID *string            `json:"accumulated_depreciation_account_id,omitempty"`
	CreatedAt                     time.Time          `json:"created_at"`
	UpdatedAt                     time.Time          `json:"updated_at"`
}

// FixedAsset represents a fixed asset
type FixedAsset struct {
	ID          string         `json:"id"`
	TenantID    string         `json:"tenant_id"`
	AssetNumber string         `json:"asset_number"`
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	CategoryID  *string        `json:"category_id,omitempty"`
	Category    *AssetCategory `json:"category,omitempty"`
	Status      AssetStatus    `json:"status"`

	// Purchase Information
	PurchaseDate time.Time       `json:"purchase_date"`
	PurchaseCost decimal.Decimal `json:"purchase_cost"`
	SupplierID   *string         `json:"supplier_id,omitempty"`
	InvoiceID    *string         `json:"invoice_id,omitempty"`
	SerialNumber string          `json:"serial_number,omitempty"`
	Location     string          `json:"location,omitempty"`

	// Depreciation Settings
	DepreciationMethod    DepreciationMethod `json:"depreciation_method"`
	UsefulLifeMonths      int                `json:"useful_life_months"`
	ResidualValue         decimal.Decimal    `json:"residual_value"`
	DepreciationStartDate *time.Time         `json:"depreciation_start_date,omitempty"`

	// Calculated Values
	AccumulatedDepreciation decimal.Decimal `json:"accumulated_depreciation"`
	BookValue               decimal.Decimal `json:"book_value"`
	LastDepreciationDate    *time.Time      `json:"last_depreciation_date,omitempty"`

	// Disposal Information
	DisposalDate     *time.Time      `json:"disposal_date,omitempty"`
	DisposalMethod   *DisposalMethod `json:"disposal_method,omitempty"`
	DisposalProceeds decimal.Decimal `json:"disposal_proceeds"`
	DisposalNotes    string          `json:"disposal_notes,omitempty"`

	// Account Links
	AssetAccountID                *string `json:"asset_account_id,omitempty"`
	DepreciationExpenseAccountID  *string `json:"depreciation_expense_account_id,omitempty"`
	AccumulatedDepreciationAcctID *string `json:"accumulated_depreciation_account_id,omitempty"`

	CreatedAt time.Time `json:"created_at"`
	CreatedBy string    `json:"created_by"`
	UpdatedAt time.Time `json:"updated_at"`
}

// DepreciationEntry represents a single depreciation record
type DepreciationEntry struct {
	ID                 string          `json:"id"`
	TenantID           string          `json:"tenant_id"`
	AssetID            string          `json:"asset_id"`
	DepreciationDate   time.Time       `json:"depreciation_date"`
	PeriodStart        time.Time       `json:"period_start"`
	PeriodEnd          time.Time       `json:"period_end"`
	DepreciationAmount decimal.Decimal `json:"depreciation_amount"`
	AccumulatedTotal   decimal.Decimal `json:"accumulated_total"`
	BookValueAfter     decimal.Decimal `json:"book_value_after"`
	JournalEntryID     *string         `json:"journal_entry_id,omitempty"`
	Notes              string          `json:"notes,omitempty"`
	CreatedAt          time.Time       `json:"created_at"`
	CreatedBy          string          `json:"created_by"`
}

// AssetMaintenance represents a maintenance record for an asset
type AssetMaintenance struct {
	ID                  string          `json:"id"`
	TenantID            string          `json:"tenant_id"`
	AssetID             string          `json:"asset_id"`
	MaintenanceDate     time.Time       `json:"maintenance_date"`
	MaintenanceType     string          `json:"maintenance_type"`
	Description         string          `json:"description"`
	Cost                decimal.Decimal `json:"cost"`
	PerformedBy         string          `json:"performed_by,omitempty"`
	NextMaintenanceDate *time.Time      `json:"next_maintenance_date,omitempty"`
	InvoiceID           *string         `json:"invoice_id,omitempty"`
	CreatedAt           time.Time       `json:"created_at"`
	CreatedBy           string          `json:"created_by"`
}

// CalculateMonthlyDepreciation calculates the monthly depreciation amount
func (a *FixedAsset) CalculateMonthlyDepreciation() decimal.Decimal {
	if a.UsefulLifeMonths <= 0 {
		return decimal.Zero
	}

	depreciableAmount := a.PurchaseCost.Sub(a.ResidualValue)
	if depreciableAmount.LessThanOrEqual(decimal.Zero) {
		return decimal.Zero
	}

	switch a.DepreciationMethod {
	case DepreciationStraightLine:
		return depreciableAmount.Div(decimal.NewFromInt(int64(a.UsefulLifeMonths))).Round(2)
	case DepreciationDecliningBalance:
		// Double declining balance rate
		yearlyRate := decimal.NewFromFloat(2.0).Div(decimal.NewFromInt(int64(a.UsefulLifeMonths / 12)))
		monthlyRate := yearlyRate.Div(decimal.NewFromInt(12))
		return a.BookValue.Mul(monthlyRate).Round(2)
	default:
		return depreciableAmount.Div(decimal.NewFromInt(int64(a.UsefulLifeMonths))).Round(2)
	}
}

// Validate validates the asset data
func (a *FixedAsset) Validate() error {
	if a.Name == "" {
		return errors.New("asset name is required")
	}
	if a.PurchaseDate.IsZero() {
		return errors.New("purchase date is required")
	}
	if a.PurchaseCost.LessThanOrEqual(decimal.Zero) {
		return errors.New("purchase cost must be positive")
	}
	if a.UsefulLifeMonths <= 0 {
		return errors.New("useful life must be positive")
	}
	if a.ResidualValue.LessThan(decimal.Zero) {
		return errors.New("residual value cannot be negative")
	}
	if a.ResidualValue.GreaterThan(a.PurchaseCost) {
		return errors.New("residual value cannot exceed purchase cost")
	}
	return nil
}

// CreateAssetRequest is the request to create a fixed asset
type CreateAssetRequest struct {
	Name                          string             `json:"name"`
	Description                   string             `json:"description,omitempty"`
	CategoryID                    *string            `json:"category_id,omitempty"`
	PurchaseDate                  time.Time          `json:"purchase_date"`
	PurchaseCost                  decimal.Decimal    `json:"purchase_cost"`
	SupplierID                    *string            `json:"supplier_id,omitempty"`
	SerialNumber                  string             `json:"serial_number,omitempty"`
	Location                      string             `json:"location,omitempty"`
	DepreciationMethod            DepreciationMethod `json:"depreciation_method,omitempty"`
	UsefulLifeMonths              int                `json:"useful_life_months,omitempty"`
	ResidualValue                 decimal.Decimal    `json:"residual_value,omitempty"`
	DepreciationStartDate         *time.Time         `json:"depreciation_start_date,omitempty"`
	AssetAccountID                *string            `json:"asset_account_id,omitempty"`
	DepreciationExpenseAccountID  *string            `json:"depreciation_expense_account_id,omitempty"`
	AccumulatedDepreciationAcctID *string            `json:"accumulated_depreciation_account_id,omitempty"`
	UserID                        string             `json:"-"`
}

// UpdateAssetRequest is the request to update a fixed asset
type UpdateAssetRequest struct {
	Name                          string             `json:"name"`
	Description                   string             `json:"description,omitempty"`
	CategoryID                    *string            `json:"category_id,omitempty"`
	SerialNumber                  string             `json:"serial_number,omitempty"`
	Location                      string             `json:"location,omitempty"`
	DepreciationMethod            DepreciationMethod `json:"depreciation_method,omitempty"`
	UsefulLifeMonths              int                `json:"useful_life_months,omitempty"`
	ResidualValue                 decimal.Decimal    `json:"residual_value,omitempty"`
	AssetAccountID                *string            `json:"asset_account_id,omitempty"`
	DepreciationExpenseAccountID  *string            `json:"depreciation_expense_account_id,omitempty"`
	AccumulatedDepreciationAcctID *string            `json:"accumulated_depreciation_account_id,omitempty"`
}

// DisposeAssetRequest is the request to dispose of an asset
type DisposeAssetRequest struct {
	DisposalDate     time.Time       `json:"disposal_date"`
	DisposalMethod   DisposalMethod  `json:"disposal_method"`
	DisposalProceeds decimal.Decimal `json:"disposal_proceeds,omitempty"`
	DisposalNotes    string          `json:"disposal_notes,omitempty"`
	UserID           string          `json:"-"`
}

// CreateCategoryRequest is the request to create an asset category
type CreateCategoryRequest struct {
	Name                          string             `json:"name"`
	Description                   string             `json:"description,omitempty"`
	DepreciationMethod            DepreciationMethod `json:"depreciation_method,omitempty"`
	DefaultUsefulLifeMonths       int                `json:"default_useful_life_months,omitempty"`
	DefaultResidualValuePercent   decimal.Decimal    `json:"default_residual_value_percent,omitempty"`
	AssetAccountID                *string            `json:"asset_account_id,omitempty"`
	DepreciationExpenseAccountID  *string            `json:"depreciation_expense_account_id,omitempty"`
	AccumulatedDepreciationAcctID *string            `json:"accumulated_depreciation_account_id,omitempty"`
}

// AssetFilter provides filtering options for assets
type AssetFilter struct {
	Status     AssetStatus
	CategoryID string
	Search     string
}
