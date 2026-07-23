package erp

import "time"

// Category represents a product category.
type Category struct {
	ID         int64     `json:"id" db:"id"`
	BusinessID string    `json:"business_id" db:"business_id"`
	Code       string    `json:"code" db:"code"`
	Name       string    `json:"name" db:"name"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
}

// Brand represents a product brand.
type Brand struct {
	ID          int64     `json:"id" db:"id"`
	BusinessID  string    `json:"business_id" db:"business_id"`
	Name        string    `json:"name" db:"name"`
	Description string    `json:"description" db:"description"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
}

// Unit represents a measurement unit.
type Unit struct {
	ID            int64     `json:"id" db:"id"`
	BusinessID    string    `json:"business_id" db:"business_id"`
	Name          string    `json:"name" db:"name"`
	ShortName     string    `json:"short_name" db:"short_name"`
	BaseUnit      *int64    `json:"base_unit,omitempty" db:"base_unit"`
	Operator      string    `json:"operator" db:"operator"`
	OperatorValue float64   `json:"operator_value" db:"operator_value"`
	CreatedAt     time.Time `json:"created_at" db:"created_at"`
}

// Warehouse represents an inventory storage location.
type Warehouse struct {
	ID         int64     `json:"id" db:"id"`
	BusinessID string    `json:"business_id" db:"business_id"`
	Name       string    `json:"name" db:"name"`
	City       string    `json:"city" db:"city"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
}

// Product represents a full ERP product record.
type Product struct {
	ID               string    `json:"id" db:"id"`
	BusinessID       string    `json:"business_id" db:"business_id"`
	Name             string    `json:"name" db:"name"`
	Code             string    `json:"code" db:"code"`
	BarcodeSymbology string    `json:"barcode_symbology" db:"barcode_symbology"`
	CategoryID       int64     `json:"category_id" db:"category_id"`
	BrandID          *int64    `json:"brand_id,omitempty" db:"brand_id"`
	Description      string    `json:"description" db:"description"`
	Type             string    `json:"type" db:"type"`
	UnitID           int64     `json:"unit_id" db:"unit_id"`
	UnitSaleID       int64     `json:"unit_sale_id" db:"unit_sale_id"`
	UnitPurchaseID   int64     `json:"unit_purchase_id" db:"unit_purchase_id"`
	StockAlert       float64   `json:"stock_alert" db:"stock_alert"`
	Cost             float64   `json:"cost" db:"cost"`
	Price            float64   `json:"price" db:"price"`
	WholesalePrice   float64   `json:"wholesale_price" db:"wholesale_price"`
	MinPrice         float64   `json:"min_price" db:"min_price"`
	TaxNet           float64   `json:"tax_net" db:"tax_net"`
	TaxMethod        string    `json:"tax_method" db:"tax_method"`
	DiscountMethod   string    `json:"discount_method" db:"discount_method"`
	Discount         float64   `json:"discount" db:"discount"`
	Points           float64   `json:"points" db:"points"`
	WarrantyPeriod   int       `json:"warranty_period" db:"warranty_period"`
	WarrantyUnit     string    `json:"warranty_unit" db:"warranty_unit"`
	HasGuarantee     bool      `json:"has_guarantee" db:"has_guarantee"`
	WarrantyTerms    string    `json:"warranty_terms" db:"warranty_terms"`
	IsImei           bool      `json:"is_imei" db:"is_imei"`
	NotSelling       bool      `json:"not_selling" db:"not_selling"`
	IsFeatured       bool      `json:"is_featured" db:"is_featured"`
	Image            string    `json:"image" db:"image"`
	CreatedAt        time.Time `json:"created_at" db:"created_at"`
	UpdatedAt        time.Time `json:"updated_at" db:"updated_at"`
}

// ProductWarehouse represents the quantity of a product in a warehouse.
type ProductWarehouse struct {
	ID          int64     `json:"id" db:"id"`
	BusinessID  string    `json:"business_id" db:"business_id"`
	ProductID   string    `json:"product_id" db:"product_id"`
	WarehouseID int64     `json:"warehouse_id" db:"warehouse_id"`
	Qte         float64   `json:"qte" db:"qte"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
}

// WarehouseStockInput represents opening stock quantities input by warehouse ID.
type WarehouseStockInput struct {
	WarehouseID int64   `json:"warehouse_id" binding:"required"`
	Quantity    float64 `json:"quantity" binding:"gte=0"`
}

// CreateProductRequest defines the API request payload for creating a new product.
type CreateProductRequest struct {
	ID               string                `json:"id"` // Optional client UUID
	BusinessID       string                `json:"business_id"`
	Name             string                `json:"name" binding:"required"`
	Code             string                `json:"code" binding:"required"`
	BarcodeSymbology string                `json:"barcode_symbology" binding:"required"`
	CategoryID       int64                 `json:"category_id" binding:"required"`
	BrandID          *int64                `json:"brand_id"`
	Description      string                `json:"description"`
	Type             string                `json:"type" binding:"required"`
	UnitID           int64                 `json:"unit_id" binding:"required"`
	UnitSaleID       int64                 `json:"unit_sale_id" binding:"required"`
	UnitPurchaseID   int64                 `json:"unit_purchase_id" binding:"required"`
	StockAlert       float64               `json:"stock_alert"`
	Cost             float64               `json:"cost" binding:"gte=0"`
	Price            float64               `json:"price" binding:"gte=0"`
	WholesalePrice   float64               `json:"wholesale_price"`
	MinPrice         float64               `json:"min_price"`
	TaxNet           float64               `json:"tax_net"`
	TaxMethod        string                `json:"tax_method" binding:"required"`
	DiscountMethod   string                `json:"discount_method" binding:"required"`
	Discount         float64               `json:"discount"`
	Points           float64               `json:"points"`
	WarrantyPeriod   int                   `json:"warranty_period"`
	WarrantyUnit     string                `json:"warranty_unit"`
	HasGuarantee     bool                  `json:"has_guarantee"`
	WarrantyTerms    string                `json:"warranty_terms"`
	IsImei           bool                  `json:"is_imei"`
	NotSelling       bool                  `json:"not_selling"`
	IsFeatured       bool                  `json:"is_featured"`
	Image            string                `json:"image"`
	OpeningStock     []WarehouseStockInput `json:"opening_stock"`
}

// DropdownDataResponse bundles category, brand, unit, and warehouse lookup options.
type DropdownDataResponse struct {
	Categories []Category  `json:"categories"`
	Brands     []Brand     `json:"brands"`
	Units      []Unit      `json:"units"`
	Warehouses []Warehouse `json:"warehouses"`
}

// APIError represents structured error feedback.
type APIError struct {
	Error   string `json:"error"`
	Details string `json:"details,omitempty"`
}

type CreateCategoryRequest struct {
	BusinessID string `json:"business_id"`
	Code       string `json:"code" binding:"required"`
	Name       string `json:"name" binding:"required"`
}

type CreateBrandRequest struct {
	BusinessID  string `json:"business_id"`
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
}

type CreateUnitRequest struct {
	BusinessID    string  `json:"business_id"`
	Name          string  `json:"name" binding:"required"`
	ShortName     string  `json:"short_name" binding:"required"`
	BaseUnit      *int64  `json:"base_unit"`
	Operator      string  `json:"operator"`
	OperatorValue float64 `json:"operator_value"`
}

type CreateWarehouseRequest struct {
	BusinessID string `json:"business_id"`
	Name       string `json:"name" binding:"required"`
	City       string `json:"city"`
}
