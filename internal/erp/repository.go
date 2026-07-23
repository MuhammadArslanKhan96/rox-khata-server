package erp

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DBTX represents database operations available on both *pgxpool.Pool and pgx.Tx.
type DBTX interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type ERPRepository interface {
	GetPool() *pgxpool.Pool
	SaveProduct(ctx context.Context, db DBTX, req CreateProductRequest) (*Product, error)
	GetDropdownData(ctx context.Context, db DBTX, businessID string) (*DropdownDataResponse, error)
	EnsureDefaultDropdownData(ctx context.Context, db DBTX, businessID string) error
	SaveCategory(ctx context.Context, db DBTX, req CreateCategoryRequest) (*Category, error)
	SaveBrand(ctx context.Context, db DBTX, req CreateBrandRequest) (*Brand, error)
	SaveUnit(ctx context.Context, db DBTX, req CreateUnitRequest) (*Unit, error)
	SaveWarehouse(ctx context.Context, db DBTX, req CreateWarehouseRequest) (*Warehouse, error)
}

type postgresERPRepository struct {
	pool *pgxpool.Pool
}

func NewERPRepository(pool *pgxpool.Pool) ERPRepository {
	return &postgresERPRepository{pool: pool}
}

func (r *postgresERPRepository) GetPool() *pgxpool.Pool {
	return r.pool
}

func (r *postgresERPRepository) SaveProduct(ctx context.Context, db DBTX, req CreateProductRequest) (*Product, error) {
	// 1. Resolve UUID ID
	pID := req.ID
	if pID == "" {
		pID = uuid.New().String()
	}

	// 2. Auto-upsert tenant
	r.upsertTenant(ctx, db, req.BusinessID, "My Business")

	// 3. Insert Product
	queryProduct := `
		INSERT INTO products (
			id, business_id, name, code, barcode_symbology, category_id, brand_id, description,
			type, unit_id, unit_sale_id, unit_purchase_id, stock_alert, cost, price,
			wholesale_price, min_price, tax_net, tax_method, discount_method, discount,
			points, warranty_period, warranty_unit, has_guarantee, warranty_terms,
			is_imei, not_selling, is_featured, image, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8,
			$9, $10, $11, $12, $13, $14, $15,
			$16, $17, $18, $19, $20, $21,
			$22, $23, $24, $25, $26,
			$27, $28, $29, $30, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
		)
		ON CONFLICT (id) DO UPDATE SET
			name = EXCLUDED.name,
			code = EXCLUDED.code,
			barcode_symbology = EXCLUDED.barcode_symbology,
			category_id = EXCLUDED.category_id,
			brand_id = EXCLUDED.brand_id,
			description = EXCLUDED.description,
			type = EXCLUDED.type,
			unit_id = EXCLUDED.unit_id,
			unit_sale_id = EXCLUDED.unit_sale_id,
			unit_purchase_id = EXCLUDED.unit_purchase_id,
			stock_alert = EXCLUDED.stock_alert,
			cost = EXCLUDED.cost,
			price = EXCLUDED.price,
			wholesale_price = EXCLUDED.wholesale_price,
			min_price = EXCLUDED.min_price,
			tax_net = EXCLUDED.tax_net,
			tax_method = EXCLUDED.tax_method,
			discount_method = EXCLUDED.discount_method,
			discount = EXCLUDED.discount,
			points = EXCLUDED.points,
			warranty_period = EXCLUDED.warranty_period,
			warranty_unit = EXCLUDED.warranty_unit,
			has_guarantee = EXCLUDED.has_guarantee,
			warranty_terms = EXCLUDED.warranty_terms,
			is_imei = EXCLUDED.is_imei,
			not_selling = EXCLUDED.not_selling,
			is_featured = EXCLUDED.is_featured,
			image = EXCLUDED.image,
			updated_at = CURRENT_TIMESTAMP
		RETURNING 
			id, business_id, name, code, barcode_symbology, category_id, brand_id, description,
			type, unit_id, unit_sale_id, unit_purchase_id, stock_alert, cost, price,
			wholesale_price, min_price, tax_net, tax_method, discount_method, discount,
			points, warranty_period, warranty_unit, has_guarantee, warranty_terms,
			is_imei, not_selling, is_featured, image, created_at, updated_at
	`

	var p Product
	row := db.QueryRow(ctx, queryProduct,
		pID, req.BusinessID, req.Name, req.Code, req.BarcodeSymbology, req.CategoryID, req.BrandID, req.Description,
		req.Type, req.UnitID, req.UnitSaleID, req.UnitPurchaseID, req.StockAlert, req.Cost, req.Price,
		req.WholesalePrice, req.MinPrice, req.TaxNet, req.TaxMethod, req.DiscountMethod, req.Discount,
		req.Points, req.WarrantyPeriod, req.WarrantyUnit, req.HasGuarantee, req.WarrantyTerms,
		req.IsImei, req.NotSelling, req.IsFeatured, req.Image,
	)

	err := row.Scan(
		&p.ID, &p.BusinessID, &p.Name, &p.Code, &p.BarcodeSymbology, &p.CategoryID, &p.BrandID, &p.Description,
		&p.Type, &p.UnitID, &p.UnitSaleID, &p.UnitPurchaseID, &p.StockAlert, &p.Cost, &p.Price,
		&p.WholesalePrice, &p.MinPrice, &p.TaxNet, &p.TaxMethod, &p.DiscountMethod, &p.Discount,
		&p.Points, &p.WarrantyPeriod, &p.WarrantyUnit, &p.HasGuarantee, &p.WarrantyTerms,
		&p.IsImei, &p.NotSelling, &p.IsFeatured, &p.Image, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to save product row: %w", err)
	}

	// 4. Save Product Warehouse Quantities (Opening Stock)
	if len(req.OpeningStock) > 0 {
		for _, stock := range req.OpeningStock {
			queryStock := `
				INSERT INTO product_warehouse (business_id, product_id, warehouse_id, qte)
				VALUES ($1, $2, $3, $4)
				ON CONFLICT (product_id, warehouse_id) DO UPDATE SET
					qte = EXCLUDED.qte
			`
			_, err = db.Exec(ctx, queryStock, req.BusinessID, p.ID, stock.WarehouseID, stock.Quantity)
			if err != nil {
				return nil, fmt.Errorf("failed to save stock for warehouse %d: %w", stock.WarehouseID, err)
			}
		}
	}

	return &p, nil
}

func (r *postgresERPRepository) GetDropdownData(ctx context.Context, db DBTX, businessID string) (*DropdownDataResponse, error) {
	// Auto-upsert tenant and seed default dropdowns if empty
	r.upsertTenant(ctx, db, businessID, "My Business")
	_ = r.EnsureDefaultDropdownData(ctx, db, businessID)

	var res DropdownDataResponse

	// Categories
	catRows, err := db.Query(ctx, `SELECT id, business_id, code, name, created_at FROM categories WHERE business_id = $1 ORDER BY name ASC`, businessID)
	if err == nil {
		defer catRows.Close()
		for catRows.Next() {
			var cat Category
			if err := catRows.Scan(&cat.ID, &cat.BusinessID, &cat.Code, &cat.Name, &cat.CreatedAt); err == nil {
				res.Categories = append(res.Categories, cat)
			}
		}
	}

	// Brands
	brandRows, err := db.Query(ctx, `SELECT id, business_id, name, COALESCE(description, ''), created_at FROM brands WHERE business_id = $1 ORDER BY name ASC`, businessID)
	if err == nil {
		defer brandRows.Close()
		for brandRows.Next() {
			var brand Brand
			if err := brandRows.Scan(&brand.ID, &brand.BusinessID, &brand.Name, &brand.Description, &brand.CreatedAt); err == nil {
				res.Brands = append(res.Brands, brand)
			}
		}
	}

	// Units
	unitRows, err := db.Query(ctx, `SELECT id, business_id, name, short_name, base_unit, operator, operator_value, created_at FROM units WHERE business_id = $1 ORDER BY name ASC`, businessID)
	if err == nil {
		defer unitRows.Close()
		for unitRows.Next() {
			var unit Unit
			var baseUnit sql.NullInt64
			if err := unitRows.Scan(&unit.ID, &unit.BusinessID, &unit.Name, &unit.ShortName, &baseUnit, &unit.Operator, &unit.OperatorValue, &unit.CreatedAt); err == nil {
				if baseUnit.Valid {
					unit.BaseUnit = &baseUnit.Int64
				}
				res.Units = append(res.Units, unit)
			}
		}
	}

	// Warehouses
	whRows, err := db.Query(ctx, `SELECT id, business_id, name, COALESCE(city, ''), created_at FROM warehouses WHERE business_id = $1 ORDER BY name ASC`, businessID)
	if err == nil {
		defer whRows.Close()
		for whRows.Next() {
			var wh Warehouse
			if err := whRows.Scan(&wh.ID, &wh.BusinessID, &wh.Name, &wh.City, &wh.CreatedAt); err == nil {
				res.Warehouses = append(res.Warehouses, wh)
			}
		}
	}

	// Initialize empty lists instead of null
	if res.Categories == nil {
		res.Categories = []Category{}
	}
	if res.Brands == nil {
		res.Brands = []Brand{}
	}
	if res.Units == nil {
		res.Units = []Unit{}
	}
	if res.Warehouses == nil {
		res.Warehouses = []Warehouse{}
	}

	return &res, nil
}

func (r *postgresERPRepository) EnsureDefaultDropdownData(ctx context.Context, db DBTX, businessID string) error {
	var count int


	// 2. Brands
	_ = db.QueryRow(ctx, "SELECT COUNT(*) FROM brands WHERE business_id = $1", businessID).Scan(&count)
	if count == 0 {
		brands := []Brand{
			{Name: "No Brand", Description: "No specific brand"},
			{Name: "Samsung", Description: "Samsung electronics"},
			{Name: "Apple", Description: "Apple Inc."},
			{Name: "Nestle", Description: "Nestle food products"},
		}
		for _, b := range brands {
			_, _ = db.Exec(ctx, "INSERT INTO brands (business_id, name, description) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING", businessID, b.Name, b.Description)
		}
	}

	// 3. Units
	_ = db.QueryRow(ctx, "SELECT COUNT(*) FROM units WHERE business_id = $1", businessID).Scan(&count)
	if count == 0 {
		units := []Unit{
			{Name: "Piece", ShortName: "pc", Operator: "*", OperatorValue: 1.0},
			{Name: "Kilogram", ShortName: "kg", Operator: "*", OperatorValue: 1.0},
			{Name: "Box", ShortName: "box", Operator: "*", OperatorValue: 1.0},
			{Name: "Dozen", ShortName: "dz", Operator: "*", OperatorValue: 12.0},
		}
		for _, u := range units {
			_, _ = db.Exec(ctx, "INSERT INTO units (business_id, name, short_name, operator, operator_value) VALUES ($1, $2, $3, $4, $5) ON CONFLICT DO NOTHING", businessID, u.Name, u.ShortName, u.Operator, u.OperatorValue)
		}
	}

	// 4. Warehouses (No default warehouses seeded)


	return nil
}

func (r *postgresERPRepository) SaveCategory(ctx context.Context, db DBTX, req CreateCategoryRequest) (*Category, error) {
	r.upsertTenant(ctx, db, req.BusinessID, "My Business")
	query := `
		INSERT INTO categories (business_id, code, name, created_at)
		VALUES ($1, $2, $3, CURRENT_TIMESTAMP)
		ON CONFLICT (business_id, code) DO UPDATE SET name = EXCLUDED.name
		RETURNING id, business_id, code, name, created_at
	`
	var cat Category
	err := db.QueryRow(ctx, query, req.BusinessID, req.Code, req.Name).Scan(&cat.ID, &cat.BusinessID, &cat.Code, &cat.Name, &cat.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to save category: %w", err)
	}
	return &cat, nil
}

func (r *postgresERPRepository) SaveBrand(ctx context.Context, db DBTX, req CreateBrandRequest) (*Brand, error) {
	r.upsertTenant(ctx, db, req.BusinessID, "My Business")
	query := `
		INSERT INTO brands (business_id, name, description, created_at)
		VALUES ($1, $2, $3, CURRENT_TIMESTAMP)
		ON CONFLICT (business_id, name) DO UPDATE SET description = EXCLUDED.description
		RETURNING id, business_id, name, description, created_at
	`
	var b Brand
	err := db.QueryRow(ctx, query, req.BusinessID, req.Name, req.Description).Scan(&b.ID, &b.BusinessID, &b.Name, &b.Description, &b.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to save brand: %w", err)
	}
	return &b, nil
}

func (r *postgresERPRepository) SaveUnit(ctx context.Context, db DBTX, req CreateUnitRequest) (*Unit, error) {
	r.upsertTenant(ctx, db, req.BusinessID, "My Business")
	operator := req.Operator
	if operator == "" {
		operator = "*"
	}
	opVal := req.OperatorValue
	if opVal <= 0 {
		opVal = 1.0
	}
	query := `
		INSERT INTO units (business_id, name, short_name, base_unit, operator, operator_value, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, CURRENT_TIMESTAMP)
		ON CONFLICT (business_id, name) DO UPDATE SET
			short_name = EXCLUDED.short_name,
			base_unit = EXCLUDED.base_unit,
			operator = EXCLUDED.operator,
			operator_value = EXCLUDED.operator_value
		RETURNING id, business_id, name, short_name, base_unit, operator, operator_value, created_at
	`
	var u Unit
	var baseUnit sql.NullInt64
	err := db.QueryRow(ctx, query, req.BusinessID, req.Name, req.ShortName, req.BaseUnit, operator, opVal).
		Scan(&u.ID, &u.BusinessID, &u.Name, &u.ShortName, &baseUnit, &u.Operator, &u.OperatorValue, &u.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to save unit: %w", err)
	}
	if baseUnit.Valid {
		u.BaseUnit = &baseUnit.Int64
	}
	return &u, nil
}

func (r *postgresERPRepository) SaveWarehouse(ctx context.Context, db DBTX, req CreateWarehouseRequest) (*Warehouse, error) {
	r.upsertTenant(ctx, db, req.BusinessID, "My Business")
	query := `
		INSERT INTO warehouses (business_id, name, city, created_at)
		VALUES ($1, $2, $3, CURRENT_TIMESTAMP)
		ON CONFLICT (business_id, name) DO UPDATE SET city = EXCLUDED.city
		RETURNING id, business_id, name, city, created_at
	`
	var wh Warehouse
	err := db.QueryRow(ctx, query, req.BusinessID, req.Name, req.City).Scan(&wh.ID, &wh.BusinessID, &wh.Name, &wh.City, &wh.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to save warehouse: %w", err)
	}
	return &wh, nil
}

func (r *postgresERPRepository) upsertTenant(ctx context.Context, db DBTX, phone string, name string) {
	queryTenant := `
		INSERT INTO tenants (phone, business_name, created_at)
		VALUES ($1, $2, CURRENT_TIMESTAMP)
		ON CONFLICT (phone) DO NOTHING
	`
	_, _ = db.Exec(ctx, queryTenant, phone, name)

	queryUser := `
		INSERT INTO users (tenant_phone, username, phone, role)
		VALUES ($1, 'Owner', $1, 'OWNER')
		ON CONFLICT (phone) DO NOTHING
	`
	_, _ = db.Exec(ctx, queryUser, phone)
}
