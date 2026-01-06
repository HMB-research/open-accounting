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
	Reference     string          `json:"reference,omitempty"`
	SourceType    string          `json:"source_type,omitempty"`
	SourceID      string          `json:"source_id,omitempty"`
	ToWarehouseID string          `json:"to_warehouse_id,omitempty"`
	Notes         string          `json:"notes,omitempty"`
	MovementDate  time.Time       `json:"movement_date"`
	CreatedAt     time.Time       `json:"created_at"`
	CreatedBy     string          `json:"created_by"`
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

// CreateWarehouseRequest represents a request to create a warehouse
type CreateWarehouseRequest struct {
	Code      string `json:"code"`
	Name      string `json:"name"`
	Address   string `json:"address,omitempty"`
	IsDefault bool   `json:"is_default"`
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
	ProductID   string `json:"product_id"`
	WarehouseID string `json:"warehouse_id"`
	Quantity    string `json:"quantity"`
	UnitCost    string `json:"unit_cost,omitempty"`
	Reason      string `json:"reason,omitempty"`
	UserID      string `json:"-"`
}

// TransferStockRequest represents a request to transfer stock between warehouses
type TransferStockRequest struct {
	ProductID       string `json:"product_id"`
	FromWarehouseID string `json:"from_warehouse_id"`
	ToWarehouseID   string `json:"to_warehouse_id"`
	Quantity        string `json:"quantity"`
	Notes           string `json:"notes,omitempty"`
	UserID          string `json:"-"`
}

// ProductFilter represents filters for listing products
type ProductFilter struct {
	ProductType ProductType   `json:"product_type,omitempty"`
	Status      ProductStatus `json:"status,omitempty"`
	CategoryID  string        `json:"category_id,omitempty"`
	Search      string        `json:"search,omitempty"`
	LowStock    bool          `json:"low_stock,omitempty"`
}
