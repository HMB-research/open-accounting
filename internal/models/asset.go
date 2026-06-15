package models

import "time"

// AssetCategory represents a fixed-asset category for a tenant.
type AssetCategory struct {
	ID                            string    `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()" json:"id"`
	TenantID                      string    `gorm:"column:tenant_id;type:uuid;not null;index" json:"tenant_id"`
	Name                          string    `gorm:"size:100;not null" json:"name"`
	Description                   string    `gorm:"type:text" json:"description,omitempty"`
	DepreciationMethod            string    `gorm:"column:depreciation_method;size:20;not null;default:'STRAIGHT_LINE'" json:"depreciation_method"`
	DefaultUsefulLifeMonths       int       `gorm:"column:default_useful_life_months;not null;default:60" json:"default_useful_life_months"`
	DefaultResidualValuePercent   Decimal   `gorm:"column:default_residual_value_percent;type:numeric(5,2);not null;default:0" json:"default_residual_value_percent"`
	AssetAccountID                *string   `gorm:"column:asset_account_id;type:uuid" json:"asset_account_id,omitempty"`
	DepreciationExpenseAccountID  *string   `gorm:"column:depreciation_expense_account_id;type:uuid" json:"depreciation_expense_account_id,omitempty"`
	AccumulatedDepreciationAcctID *string   `gorm:"column:accumulated_depreciation_account_id;type:uuid" json:"accumulated_depreciation_account_id,omitempty"`
	CreatedAt                     time.Time `gorm:"not null;default:now()" json:"created_at"`
	UpdatedAt                     time.Time `gorm:"not null;default:now()" json:"updated_at"`
}

// TableName returns the table name for GORM.
func (AssetCategory) TableName() string {
	return "asset_categories"
}

// FixedAsset represents a tenant fixed asset.
type FixedAsset struct {
	ID                            string     `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()" json:"id"`
	TenantID                      string     `gorm:"column:tenant_id;type:uuid;not null;index" json:"tenant_id"`
	AssetNumber                   string     `gorm:"column:asset_number;size:50;not null" json:"asset_number"`
	Name                          string     `gorm:"size:200;not null" json:"name"`
	Description                   string     `gorm:"type:text" json:"description,omitempty"`
	CategoryID                    *string    `gorm:"column:category_id;type:uuid" json:"category_id,omitempty"`
	Status                        string     `gorm:"size:20;not null;default:'ACTIVE'" json:"status"`
	PurchaseDate                  time.Time  `gorm:"column:purchase_date;type:date;not null" json:"purchase_date"`
	PurchaseCost                  Decimal    `gorm:"column:purchase_cost;type:numeric(28,8);not null" json:"purchase_cost"`
	SupplierID                    *string    `gorm:"column:supplier_id;type:uuid" json:"supplier_id,omitempty"`
	InvoiceID                     *string    `gorm:"column:invoice_id;type:uuid" json:"invoice_id,omitempty"`
	SerialNumber                  string     `gorm:"column:serial_number;size:100" json:"serial_number,omitempty"`
	Location                      string     `gorm:"size:200" json:"location,omitempty"`
	DepreciationMethod            string     `gorm:"column:depreciation_method;size:20;not null;default:'STRAIGHT_LINE'" json:"depreciation_method"`
	UsefulLifeMonths              int        `gorm:"column:useful_life_months;not null;default:60" json:"useful_life_months"`
	ResidualValue                 Decimal    `gorm:"column:residual_value;type:numeric(28,8);not null;default:0" json:"residual_value"`
	DepreciationStartDate         *time.Time `gorm:"column:depreciation_start_date;type:date" json:"depreciation_start_date,omitempty"`
	AccumulatedDepreciation       Decimal    `gorm:"column:accumulated_depreciation;type:numeric(28,8);not null;default:0" json:"accumulated_depreciation"`
	BookValue                     Decimal    `gorm:"column:book_value;type:numeric(28,8);not null" json:"book_value"`
	LastDepreciationDate          *time.Time `gorm:"column:last_depreciation_date;type:date" json:"last_depreciation_date,omitempty"`
	DisposalDate                  *time.Time `gorm:"column:disposal_date;type:date" json:"disposal_date,omitempty"`
	DisposalMethod                *string    `gorm:"column:disposal_method;size:20" json:"disposal_method,omitempty"`
	DisposalProceeds              Decimal    `gorm:"column:disposal_proceeds;type:numeric(28,8);default:0" json:"disposal_proceeds"`
	DisposalNotes                 string     `gorm:"column:disposal_notes;type:text" json:"disposal_notes,omitempty"`
	DisposalJournalEntryID        *string    `gorm:"column:disposal_journal_entry_id;type:uuid" json:"disposal_journal_entry_id,omitempty"`
	AssetAccountID                *string    `gorm:"column:asset_account_id;type:uuid" json:"asset_account_id,omitempty"`
	DepreciationExpenseAccountID  *string    `gorm:"column:depreciation_expense_account_id;type:uuid" json:"depreciation_expense_account_id,omitempty"`
	AccumulatedDepreciationAcctID *string    `gorm:"column:accumulated_depreciation_account_id;type:uuid" json:"accumulated_depreciation_account_id,omitempty"`
	CreatedAt                     time.Time  `gorm:"not null;default:now()" json:"created_at"`
	CreatedBy                     string     `gorm:"column:created_by;type:uuid;not null" json:"created_by"`
	UpdatedAt                     time.Time  `gorm:"not null;default:now()" json:"updated_at"`
}

// TableName returns the table name for GORM.
func (FixedAsset) TableName() string {
	return "fixed_assets"
}

// DepreciationEntry represents one depreciation posting for a fixed asset.
type DepreciationEntry struct {
	ID                 string    `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()" json:"id"`
	TenantID           string    `gorm:"column:tenant_id;type:uuid;not null;index" json:"tenant_id"`
	AssetID            string    `gorm:"column:asset_id;type:uuid;not null;index" json:"asset_id"`
	DepreciationDate   time.Time `gorm:"column:depreciation_date;type:date;not null" json:"depreciation_date"`
	PeriodStart        time.Time `gorm:"column:period_start;type:date;not null" json:"period_start"`
	PeriodEnd          time.Time `gorm:"column:period_end;type:date;not null" json:"period_end"`
	DepreciationAmount Decimal   `gorm:"column:depreciation_amount;type:numeric(28,8);not null" json:"depreciation_amount"`
	AccumulatedTotal   Decimal   `gorm:"column:accumulated_total;type:numeric(28,8);not null" json:"accumulated_total"`
	BookValueAfter     Decimal   `gorm:"column:book_value_after;type:numeric(28,8);not null" json:"book_value_after"`
	JournalEntryID     *string   `gorm:"column:journal_entry_id;type:uuid" json:"journal_entry_id,omitempty"`
	Notes              string    `gorm:"type:text" json:"notes,omitempty"`
	CreatedAt          time.Time `gorm:"not null;default:now()" json:"created_at"`
	CreatedBy          string    `gorm:"column:created_by;type:uuid;not null" json:"created_by"`
}

// TableName returns the table name for GORM.
func (DepreciationEntry) TableName() string {
	return "depreciation_entries"
}
