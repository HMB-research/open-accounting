package inventory

import (
	"fmt"
	"time"

	"github.com/shopspring/decimal"
)

// ProductType defines the type of product
type ProductType string

const (
	ProductTypeGoods   ProductType = "GOODS"
	ProductTypeService ProductType = "SERVICE"
)

// ProductStatus defines the status of a product
type ProductStatus string

const (
	ProductStatusActive   ProductStatus = "ACTIVE"
	ProductStatusInactive ProductStatus = "INACTIVE"
)

// MovementType defines the type of inventory movement
type MovementType string

const (
	MovementTypeIn         MovementType = "IN"
	MovementTypeOut        MovementType = "OUT"
	MovementTypeAdjustment MovementType = "ADJUSTMENT"
	MovementTypeTransfer   MovementType = "TRANSFER"
)

// Product represents a product or service
type Product struct {
	ID                 string          `json:"id"`
	TenantID           string          `json:"tenant_id"`
	Code               string          `json:"code"`
	Name               string          `json:"name"`
	Description        string          `json:"description,omitempty"`
	ProductType        ProductType     `json:"product_type"`
	CategoryID         string          `json:"category_id,omitempty"`
	Unit               string          `json:"unit,omitempty"`
	PurchasePrice      decimal.Decimal `json:"purchase_price"`
	SalesPrice         decimal.Decimal `json:"sales_price"`
	VATRate            decimal.Decimal `json:"vat_rate"`
	MinStockLevel      decimal.Decimal `json:"min_stock_level"`
	CurrentStock       decimal.Decimal `json:"current_stock"`
	ReorderPoint       decimal.Decimal `json:"reorder_point"`
	SaleAccountID      string          `json:"sale_account_id,omitempty"`
	PurchaseAccountID  string          `json:"purchase_account_id,omitempty"`
	InventoryAccountID string          `json:"inventory_account_id,omitempty"`
	TrackInventory     bool            `json:"track_inventory"`
	IsActive           bool            `json:"is_active"`
	Barcode            string          `json:"barcode,omitempty"`
	SupplierID         string          `json:"supplier_id,omitempty"`
	LeadTimeDays       int             `json:"lead_time_days"`
	CreatedAt          time.Time       `json:"created_at"`
	UpdatedAt          time.Time       `json:"updated_at"`
}

// Validate validates the product
func (p *Product) Validate() error {
	if p.Name == "" {
		return fmt.Errorf("product name is required")
	}
	if p.ProductType == "" {
		return fmt.Errorf("product type is required")
	}
	if p.ProductType != ProductTypeGoods && p.ProductType != ProductTypeService {
		return fmt.Errorf("invalid product type: %s", p.ProductType)
	}
	if p.SalesPrice.IsNegative() {
		return fmt.Errorf("sales price cannot be negative")
	}
	return nil
}

// ProductCategory represents a category for products
type ProductCategory struct {
	ID          string    `json:"id"`
	TenantID    string    `json:"tenant_id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	ParentID    string    `json:"parent_id,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Warehouse represents a warehouse or storage location
type Warehouse struct {
	ID        string    `json:"id"`
	TenantID  string    `json:"tenant_id"`
	Code      string    `json:"code"`
	Name      string    `json:"name"`
	Address   string    `json:"address,omitempty"`
	IsDefault bool      `json:"is_default"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// StockLevel represents the stock level of a product in a warehouse
type StockLevel struct {
	ID           string          `json:"id"`
	TenantID     string          `json:"tenant_id"`
	ProductID    string          `json:"product_id"`
	WarehouseID  string          `json:"warehouse_id"`
	Quantity     decimal.Decimal `json:"quantity"`
	ReservedQty  decimal.Decimal `json:"reserved_qty"`
	AvailableQty decimal.Decimal `json:"available_qty"`
	LastUpdated  time.Time       `json:"last_updated"`
}

// InventoryLotReservation records stock reserved against a tracked lot, serial, or expiry position.
type InventoryLotReservation struct {
	ID           string          `json:"id"`
	TenantID     string          `json:"tenant_id"`
	ProductID    string          `json:"product_id"`
	WarehouseID  string          `json:"warehouse_id"`
	LotNumber    string          `json:"lot_number,omitempty"`
	SerialNumber string          `json:"serial_number,omitempty"`
	ExpiryDate   string          `json:"expiry_date,omitempty"`
	Quantity     decimal.Decimal `json:"quantity"`
	Reason       string          `json:"reason,omitempty"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
	CreatedBy    string          `json:"created_by,omitempty"`
}

// InventoryMovement represents a movement of inventory
type InventoryMovement struct {
	ID            string          `json:"id"`
	TenantID      string          `json:"tenant_id"`
	ProductID     string          `json:"product_id"`
	WarehouseID   string          `json:"warehouse_id"`
	MovementType  MovementType    `json:"movement_type"`
	Quantity      decimal.Decimal `json:"quantity"`
	UnitCost      decimal.Decimal `json:"unit_cost"`
	TotalCost     decimal.Decimal `json:"total_cost"`
	LotNumber     string          `json:"lot_number,omitempty"`
	SerialNumber  string          `json:"serial_number,omitempty"`
	ExpiryDate    string          `json:"expiry_date,omitempty"`
	Reference     string          `json:"reference,omitempty"`
	SourceType    string          `json:"source_type,omitempty"`
	SourceID      string          `json:"source_id,omitempty"`
	ToWarehouseID string          `json:"to_warehouse_id,omitempty"`
	Notes         string          `json:"notes,omitempty"`
	MovementDate  time.Time       `json:"movement_date"`
	CreatedAt     time.Time       `json:"created_at"`
	CreatedBy     string          `json:"created_by"`
}

const (
	// InventoryValuationMethodStandardCost values stock using each product purchase price.
	InventoryValuationMethodStandardCost = "STANDARD_COST"
	// InventoryValuationMethodWeightedAverage values stock using weighted average inbound movement cost.
	InventoryValuationMethodWeightedAverage = "WEIGHTED_AVERAGE"
	// InventoryValuationMethodFIFO values ending stock from the newest remaining inbound layers.
	InventoryValuationMethodFIFO = "FIFO"
)

// InventoryValuationLine represents one valued product/warehouse stock position.
type InventoryValuationLine struct {
	ProductID      string          `json:"product_id"`
	ProductCode    string          `json:"product_code"`
	ProductName    string          `json:"product_name"`
	WarehouseID    string          `json:"warehouse_id,omitempty"`
	WarehouseCode  string          `json:"warehouse_code,omitempty"`
	WarehouseName  string          `json:"warehouse_name,omitempty"`
	Quantity       decimal.Decimal `json:"quantity"`
	ReservedQty    decimal.Decimal `json:"reserved_qty"`
	AvailableQty   decimal.Decimal `json:"available_qty"`
	UnitCost       decimal.Decimal `json:"unit_cost"`
	InventoryValue decimal.Decimal `json:"inventory_value"`
}

// InventoryValuationReport summarizes valued on-hand stock for tracked goods.
type InventoryValuationReport struct {
	TenantID        string                   `json:"tenant_id"`
	WarehouseID     string                   `json:"warehouse_id,omitempty"`
	ValuationMethod string                   `json:"valuation_method"`
	Lines           []InventoryValuationLine `json:"lines"`
	TotalQuantity   decimal.Decimal          `json:"total_quantity"`
	TotalReserved   decimal.Decimal          `json:"total_reserved"`
	TotalAvailable  decimal.Decimal          `json:"total_available"`
	TotalValue      decimal.Decimal          `json:"total_value"`
	GeneratedAt     time.Time                `json:"generated_at"`
}

// InventoryLotLine represents one lot, serial, expiry, or untracked stock position.
type InventoryLotLine struct {
	ProductID        string          `json:"product_id"`
	ProductCode      string          `json:"product_code"`
	ProductName      string          `json:"product_name"`
	WarehouseID      string          `json:"warehouse_id,omitempty"`
	WarehouseCode    string          `json:"warehouse_code,omitempty"`
	WarehouseName    string          `json:"warehouse_name,omitempty"`
	LotNumber        string          `json:"lot_number,omitempty"`
	SerialNumber     string          `json:"serial_number,omitempty"`
	ExpiryDate       string          `json:"expiry_date,omitempty"`
	Quantity         decimal.Decimal `json:"quantity"`
	UnitCost         decimal.Decimal `json:"unit_cost"`
	InventoryValue   decimal.Decimal `json:"inventory_value"`
	LastMovementDate time.Time       `json:"last_movement_date"`
}

// InventoryLotReport summarizes on-hand stock by lot, serial, and expiry metadata.
type InventoryLotReport struct {
	TenantID      string             `json:"tenant_id"`
	ProductID     string             `json:"product_id,omitempty"`
	WarehouseID   string             `json:"warehouse_id,omitempty"`
	IncludeEmpty  bool               `json:"include_empty"`
	Lines         []InventoryLotLine `json:"lines"`
	TotalQuantity decimal.Decimal    `json:"total_quantity"`
	TotalValue    decimal.Decimal    `json:"total_value"`
	GeneratedAt   time.Time          `json:"generated_at"`
}

// CreateProductRequest represents a request to create a product
type CreateProductRequest struct {
	Code               string `json:"code,omitempty"`
	Name               string `json:"name"`
	Description        string `json:"description,omitempty"`
	ProductType        string `json:"product_type"`
	CategoryID         string `json:"category_id,omitempty"`
	Unit               string `json:"unit,omitempty"`
	PurchasePrice      string `json:"purchase_price,omitempty"`
	SalesPrice         string `json:"sales_price"`
	VATRate            string `json:"vat_rate,omitempty"`
	MinStockLevel      string `json:"min_stock_level,omitempty"`
	ReorderPoint       string `json:"reorder_point,omitempty"`
	SaleAccountID      string `json:"sale_account_id,omitempty"`
	PurchaseAccountID  string `json:"purchase_account_id,omitempty"`
	InventoryAccountID string `json:"inventory_account_id,omitempty"`
	TrackInventory     bool   `json:"track_inventory"`
	Barcode            string `json:"barcode,omitempty"`
	SupplierID         string `json:"supplier_id,omitempty"`
	LeadTimeDays       int    `json:"lead_time_days,omitempty"`
}

// ImportProductsRequest contains CSV payload for product master migration.
type ImportProductsRequest struct {
	CSVContent string `json:"csv_content"`
	FileName   string `json:"file_name,omitempty"`
}

// ImportProductsResult summarizes a product CSV import.
type ImportProductsResult struct {
	FileName        string                   `json:"file_name,omitempty"`
	RowsProcessed   int                      `json:"rows_processed"`
	ProductsCreated int                      `json:"products_created"`
	RowsSkipped     int                      `json:"rows_skipped"`
	Errors          []ImportProductsRowError `json:"errors,omitempty"`
}

// ImportProductsRowError describes a row-level product import failure.
type ImportProductsRowError struct {
	Row     int    `json:"row"`
	Code    string `json:"code,omitempty"`
	Name    string `json:"name,omitempty"`
	Message string `json:"message"`
}

// UpdateProductRequest represents a request to update a product
type UpdateProductRequest struct {
	Name               string `json:"name"`
	Description        string `json:"description,omitempty"`
	CategoryID         string `json:"category_id,omitempty"`
	Unit               string `json:"unit,omitempty"`
	PurchasePrice      string `json:"purchase_price,omitempty"`
	SalesPrice         string `json:"sales_price"`
	VATRate            string `json:"vat_rate,omitempty"`
	MinStockLevel      string `json:"min_stock_level,omitempty"`
	ReorderPoint       string `json:"reorder_point,omitempty"`
	SaleAccountID      string `json:"sale_account_id,omitempty"`
	PurchaseAccountID  string `json:"purchase_account_id,omitempty"`
	InventoryAccountID string `json:"inventory_account_id,omitempty"`
	TrackInventory     bool   `json:"track_inventory"`
	IsActive           bool   `json:"is_active"`
	Barcode            string `json:"barcode,omitempty"`
	SupplierID         string `json:"supplier_id,omitempty"`
	LeadTimeDays       int    `json:"lead_time_days,omitempty"`
}

// CreateCategoryRequest represents a request to create a category
type CreateCategoryRequest struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	ParentID    string `json:"parent_id,omitempty"`
}

// ImportProductCategoriesRequest contains CSV payload for product category migration.
type ImportProductCategoriesRequest struct {
	CSVContent string `json:"csv_content"`
	FileName   string `json:"file_name,omitempty"`
}

// ImportProductCategoriesResult summarizes a product category CSV import.
type ImportProductCategoriesResult struct {
	FileName          string                            `json:"file_name,omitempty"`
	RowsProcessed     int                               `json:"rows_processed"`
	CategoriesCreated int                               `json:"categories_created"`
	RowsSkipped       int                               `json:"rows_skipped"`
	Errors            []ImportProductCategoriesRowError `json:"errors,omitempty"`
}

// ImportProductCategoriesRowError describes a row-level product category import failure.
type ImportProductCategoriesRowError struct {
	Row     int    `json:"row"`
	Name    string `json:"name,omitempty"`
	Message string `json:"message"`
}

// CreateWarehouseRequest represents a request to create a warehouse
type CreateWarehouseRequest struct {
	Code      string `json:"code"`
	Name      string `json:"name"`
	Address   string `json:"address,omitempty"`
	IsDefault bool   `json:"is_default"`
}

// ImportWarehousesRequest contains CSV payload for warehouse master migration.
type ImportWarehousesRequest struct {
	CSVContent string `json:"csv_content"`
	FileName   string `json:"file_name,omitempty"`
}

// ImportWarehousesResult summarizes a warehouse CSV import.
type ImportWarehousesResult struct {
	FileName          string                     `json:"file_name,omitempty"`
	RowsProcessed     int                        `json:"rows_processed"`
	WarehousesCreated int                        `json:"warehouses_created"`
	RowsSkipped       int                        `json:"rows_skipped"`
	Errors            []ImportWarehousesRowError `json:"errors,omitempty"`
}

// ImportWarehousesRowError describes a row-level warehouse import failure.
type ImportWarehousesRowError struct {
	Row     int    `json:"row"`
	Code    string `json:"code,omitempty"`
	Name    string `json:"name,omitempty"`
	Message string `json:"message"`
}

// UpdateWarehouseRequest represents a request to update a warehouse
type UpdateWarehouseRequest struct {
	Name      string `json:"name"`
	Address   string `json:"address,omitempty"`
	IsDefault bool   `json:"is_default"`
	IsActive  bool   `json:"is_active"`
}

// AdjustStockRequest represents a request to adjust stock
type AdjustStockRequest struct {
	ProductID    string `json:"product_id"`
	WarehouseID  string `json:"warehouse_id"`
	Quantity     string `json:"quantity"`
	UnitCost     string `json:"unit_cost,omitempty"`
	LotNumber    string `json:"lot_number,omitempty"`
	SerialNumber string `json:"serial_number,omitempty"`
	ExpiryDate   string `json:"expiry_date,omitempty"`
	Reason       string `json:"reason,omitempty"`
	UserID       string `json:"-"`
}

// ImportStockAdjustmentsRequest contains CSV payload for stock adjustment migration.
type ImportStockAdjustmentsRequest struct {
	CSVContent string `json:"csv_content"`
	FileName   string `json:"file_name,omitempty"`
	UserID     string `json:"-"`
}

// ImportStockAdjustmentsResult summarizes a stock adjustment CSV import.
type ImportStockAdjustmentsResult struct {
	FileName            string                           `json:"file_name,omitempty"`
	RowsProcessed       int                              `json:"rows_processed"`
	AdjustmentsImported int                              `json:"adjustments_imported"`
	RowsSkipped         int                              `json:"rows_skipped"`
	Errors              []ImportStockAdjustmentsRowError `json:"errors,omitempty"`
}

// ImportStockAdjustmentsRowError describes a row-level stock import failure.
type ImportStockAdjustmentsRowError struct {
	Row          int    `json:"row"`
	ProductRef   string `json:"product_ref,omitempty"`
	WarehouseRef string `json:"warehouse_ref,omitempty"`
	Quantity     string `json:"quantity,omitempty"`
	Message      string `json:"message"`
}

// TransferStockRequest represents a request to transfer stock between warehouses
type TransferStockRequest struct {
	ProductID       string `json:"product_id"`
	FromWarehouseID string `json:"from_warehouse_id"`
	ToWarehouseID   string `json:"to_warehouse_id"`
	Quantity        string `json:"quantity"`
	LotNumber       string `json:"lot_number,omitempty"`
	SerialNumber    string `json:"serial_number,omitempty"`
	ExpiryDate      string `json:"expiry_date,omitempty"`
	Notes           string `json:"notes,omitempty"`
	UserID          string `json:"-"`
}

// StockReservationRequest represents a request to reserve or release warehouse stock.
type StockReservationRequest struct {
	ProductID    string `json:"product_id"`
	WarehouseID  string `json:"warehouse_id"`
	Quantity     string `json:"quantity"`
	LotNumber    string `json:"lot_number,omitempty"`
	SerialNumber string `json:"serial_number,omitempty"`
	ExpiryDate   string `json:"expiry_date,omitempty"`
	Reason       string `json:"reason,omitempty"`
	UserID       string `json:"-"`
}

// IssueStockRequest represents a request to consume sellable stock from one warehouse.
type IssueStockRequest struct {
	ProductID                string `json:"product_id"`
	WarehouseID              string `json:"warehouse_id"`
	Quantity                 string `json:"quantity"`
	LotNumber                string `json:"lot_number,omitempty"`
	SerialNumber             string `json:"serial_number,omitempty"`
	ExpiryDate               string `json:"expiry_date,omitempty"`
	Reference                string `json:"reference,omitempty"`
	SourceType               string `json:"source_type,omitempty"`
	SourceID                 string `json:"source_id,omitempty"`
	Reason                   string `json:"reason,omitempty"`
	CostOfGoodsSoldAccountID string `json:"cost_of_goods_sold_account_id,omitempty"`
	InventoryAccountID       string `json:"inventory_account_id,omitempty"`
	PostToLedger             bool   `json:"post_to_ledger,omitempty"`
	UserID                   string `json:"-"`
}

// InventoryIssueAccountingLine is a suggested accounting line for a costed stock issue.
type InventoryIssueAccountingLine struct {
	Role         string          `json:"role"`
	AccountID    string          `json:"account_id"`
	Description  string          `json:"description,omitempty"`
	DebitAmount  decimal.Decimal `json:"debit_amount"`
	CreditAmount decimal.Decimal `json:"credit_amount"`
	Currency     string          `json:"currency"`
}

// InventoryIssueAccounting contains accounting-ready COGS lines for a stock issue.
type InventoryIssueAccounting struct {
	SourceType  string                         `json:"source_type,omitempty"`
	SourceID    string                         `json:"source_id,omitempty"`
	Reference   string                         `json:"reference,omitempty"`
	Description string                         `json:"description,omitempty"`
	Posted      bool                           `json:"posted,omitempty"`
	JournalID   string                         `json:"journal_entry_id,omitempty"`
	JournalNo   string                         `json:"journal_entry_number,omitempty"`
	Lines       []InventoryIssueAccountingLine `json:"lines"`
}

// IssueStockResult summarizes created stock issue movements and their cost.
type IssueStockResult struct {
	ProductID   string                    `json:"product_id"`
	WarehouseID string                    `json:"warehouse_id"`
	Quantity    decimal.Decimal           `json:"quantity"`
	UnitCost    decimal.Decimal           `json:"unit_cost"`
	TotalCost   decimal.Decimal           `json:"total_cost"`
	Movements   []InventoryMovement       `json:"movements"`
	StockLevel  *StockLevel               `json:"stock_level,omitempty"`
	Accounting  *InventoryIssueAccounting `json:"accounting,omitempty"`
}

// ProductFilter represents filters for listing products
type ProductFilter struct {
	ProductType ProductType   `json:"product_type,omitempty"`
	Status      ProductStatus `json:"status,omitempty"`
	CategoryID  string        `json:"category_id,omitempty"`
	Search      string        `json:"search,omitempty"`
	LowStock    bool          `json:"low_stock,omitempty"`
}
