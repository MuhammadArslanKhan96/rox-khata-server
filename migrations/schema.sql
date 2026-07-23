-- migrations/schema.sql
-- Rox Khata - Complete Database Schema
-- Covers: Ledger Accounts, Journal Entries, Items, Banks, Team Members

-- Enable uuid-ossp extension for UUID support
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- ============================================================
-- 1. PARTY TABLE
--    Represents customers, suppliers (and virtual system accounts)
-- ============================================================
CREATE TABLE IF NOT EXISTS party (
    id               VARCHAR(255) PRIMARY KEY,   -- UUID from Android (matches Room entity)
    business_id      VARCHAR(255) NOT NULL,       -- Tenant phone number / business identifier
    account_name     VARCHAR(255) NOT NULL,
    account_type     VARCHAR(50)  NOT NULL CHECK (account_type IN ('CUSTOMER', 'SUPPLIER')),
    phone_number     VARCHAR(50),
    current_balance  NUMERIC(20, 4) NOT NULL DEFAULT 0.0000,
    backend_id       BIGINT,                      -- Optional backend-assigned integer ID for sync
    last_updated_at  BIGINT NOT NULL DEFAULT 0,   -- Epoch millis (matches Android Room)
    created_at       TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_party_business_id    ON party(business_id);
CREATE INDEX IF NOT EXISTS idx_party_account_type   ON party(business_id, account_type);

-- ============================================================
-- 2. JOURNAL ENTRIES
--    Append-only financial transaction log per party
-- ============================================================
CREATE TABLE IF NOT EXISTS journal_entries (
    id                   VARCHAR(255) PRIMARY KEY,  -- UUID from Android
    business_id          VARCHAR(255) NOT NULL,
    account_id           VARCHAR(255) NOT NULL REFERENCES party(id) ON DELETE CASCADE,
    amount               NUMERIC(20, 4) NOT NULL CHECK (amount > 0.0000),
    transaction_type     VARCHAR(50)   NOT NULL CHECK (transaction_type IN ('CREDIT', 'CASH_RECEIVED')),
    description          TEXT NOT NULL DEFAULT '',
    is_synced            BOOLEAN NOT NULL DEFAULT FALSE,
    backend_transfer_id  BIGINT,
    created_at           BIGINT NOT NULL,           -- Epoch millis (matches Android Room)
    synced_at            TIMESTAMP WITH TIME ZONE
);

CREATE INDEX IF NOT EXISTS idx_journal_entries_business_id ON journal_entries(business_id);
CREATE INDEX IF NOT EXISTS idx_journal_entries_account_id  ON journal_entries(account_id);
CREATE INDEX IF NOT EXISTS idx_journal_entries_created_at  ON journal_entries(created_at DESC);

-- ============================================================
-- 3. ITEMS (Product/Service Catalog)
-- ============================================================
CREATE TABLE IF NOT EXISTS items (
    id               VARCHAR(255) PRIMARY KEY,
    business_id      VARCHAR(255) NOT NULL,
    name             VARCHAR(255) NOT NULL,
    purchase_price   NUMERIC(20, 4) NOT NULL DEFAULT 0.0000,
    sale_price       NUMERIC(20, 4) NOT NULL DEFAULT 0.0000,
    stock_quantity   INTEGER NOT NULL DEFAULT 0,
    created_at       TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_items_business_id ON items(business_id);

-- ============================================================
-- 4. BANKS / WALLETS
-- ============================================================
CREATE TABLE IF NOT EXISTS banks (
    id           VARCHAR(255) PRIMARY KEY,
    business_id  VARCHAR(255) NOT NULL,
    bank_name    VARCHAR(255) NOT NULL,
    account_no   VARCHAR(255) NOT NULL DEFAULT '',
    balance      NUMERIC(20, 4) NOT NULL DEFAULT 0.0000,
    created_at   TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_banks_business_id ON banks(business_id);

-- ============================================================
-- 5. STAFF MEMBERS
-- ============================================================
CREATE TABLE IF NOT EXISTS staff (
    id           VARCHAR(255) PRIMARY KEY,
    business_id  VARCHAR(255) NOT NULL,
    name         VARCHAR(255) NOT NULL,
    phone        VARCHAR(50)  NOT NULL,
    role         VARCHAR(100) NOT NULL DEFAULT 'Staff',
    created_at   TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_staff_business_id ON staff(business_id);

-- ============================================================
-- 6. TENANTS / BUSINESSES
--    Top-level business registry keyed by owner phone number
-- ============================================================
CREATE TABLE IF NOT EXISTS tenants (
    phone                   VARCHAR(50) PRIMARY KEY,          -- Owner phone = business/tenant ID
    business_name           VARCHAR(255) NOT NULL DEFAULT 'My Business',
    email                   VARCHAR(255) UNIQUE,              -- Owner email (must be unique)
    is_verified             BOOLEAN NOT NULL DEFAULT FALSE,
    verification_code       VARCHAR(10),
    verification_expires_at TIMESTAMP WITH TIME ZONE,
    created_at              TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- ============================================================
-- 7. USERS
--    Users registered under a business tenant
-- ============================================================
CREATE TABLE IF NOT EXISTS users (
    id           SERIAL PRIMARY KEY,
    tenant_phone VARCHAR(50) NOT NULL REFERENCES tenants(phone) ON DELETE CASCADE,
    username     VARCHAR(255) NOT NULL,
    email        VARCHAR(255) UNIQUE,
    phone        VARCHAR(50) NOT NULL UNIQUE,
    role         VARCHAR(50) NOT NULL DEFAULT 'STAFF',
    created_at   TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- ============================================================
-- 8. ERP TABLES (Category, Brand, Unit, Warehouse, Products, ProductWarehouse)
-- ============================================================
CREATE TABLE IF NOT EXISTS categories (
    id               SERIAL PRIMARY KEY,
    business_id      VARCHAR(255) NOT NULL,
    code             VARCHAR(192) NOT NULL,
    name             VARCHAR(192) NOT NULL,
    created_at       TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_categories_code ON categories(business_id, code);

CREATE TABLE IF NOT EXISTS brands (
    id               SERIAL PRIMARY KEY,
    business_id      VARCHAR(255) NOT NULL,
    name             VARCHAR(192) NOT NULL,
    description      VARCHAR(255),
    created_at       TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_brands_name ON brands(business_id, name);

CREATE TABLE IF NOT EXISTS units (
    id               SERIAL PRIMARY KEY,
    business_id      VARCHAR(255) NOT NULL,
    name             VARCHAR(192) NOT NULL,
    short_name       VARCHAR(192) NOT NULL,
    base_unit        INTEGER,
    operator         VARCHAR(10) NOT NULL DEFAULT '*',
    operator_value   NUMERIC(20, 4) NOT NULL DEFAULT 1.0000,
    created_at       TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_units_name ON units(business_id, name);

CREATE TABLE IF NOT EXISTS warehouses (
    id               SERIAL PRIMARY KEY,
    business_id      VARCHAR(255) NOT NULL,
    name             VARCHAR(192) NOT NULL,
    city             VARCHAR(192),
    created_at       TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_warehouses_name ON warehouses(business_id, name);

CREATE TABLE IF NOT EXISTS products (
    id                   VARCHAR(255) PRIMARY KEY, -- UUID from client or server
    business_id          VARCHAR(255) NOT NULL,
    name                 VARCHAR(255) NOT NULL,
    code                 VARCHAR(192) NOT NULL,
    barcode_symbology    VARCHAR(192) NOT NULL DEFAULT 'Code 128',
    category_id          INTEGER NOT NULL REFERENCES categories(id) ON DELETE CASCADE,
    brand_id             INTEGER REFERENCES brands(id) ON DELETE SET NULL,
    description          TEXT,
    type                 VARCHAR(192) NOT NULL DEFAULT 'Standard Product',
    unit_id              INTEGER NOT NULL REFERENCES units(id) ON DELETE CASCADE,
    unit_sale_id         INTEGER NOT NULL REFERENCES units(id) ON DELETE CASCADE,
    unit_purchase_id     INTEGER NOT NULL REFERENCES units(id) ON DELETE CASCADE,
    stock_alert          NUMERIC(20, 4) NOT NULL DEFAULT 0.0000,
    cost                 NUMERIC(20, 4) NOT NULL DEFAULT 0.0000,
    price                NUMERIC(20, 4) NOT NULL DEFAULT 0.0000,
    wholesale_price      NUMERIC(20, 4) NOT NULL DEFAULT 0.0000,
    min_price            NUMERIC(20, 4) NOT NULL DEFAULT 0.0000,
    tax_net              NUMERIC(20, 4) NOT NULL DEFAULT 0.0000,
    tax_method           VARCHAR(192) NOT NULL DEFAULT 'Exclusive',
    discount_method      VARCHAR(192) NOT NULL DEFAULT 'Percent',
    discount             NUMERIC(20, 4) NOT NULL DEFAULT 0.0000,
    points               NUMERIC(20, 4) NOT NULL DEFAULT 0.0000,
    warranty_period      INTEGER NOT NULL DEFAULT 0,
    warranty_unit        VARCHAR(50) NOT NULL DEFAULT 'Months',
    has_guarantee        BOOLEAN NOT NULL DEFAULT FALSE,
    warranty_terms       TEXT,
    is_imei              BOOLEAN NOT NULL DEFAULT FALSE,
    not_selling          BOOLEAN NOT NULL DEFAULT FALSE,
    is_featured          BOOLEAN NOT NULL DEFAULT FALSE,
    image                TEXT,
    created_at           TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at           TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_products_code ON products(business_id, code);

CREATE TABLE IF NOT EXISTS product_warehouse (
    id                   SERIAL PRIMARY KEY,
    business_id          VARCHAR(255) NOT NULL,
    product_id           VARCHAR(255) NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    warehouse_id         INTEGER NOT NULL REFERENCES warehouses(id) ON DELETE CASCADE,
    qte                  NUMERIC(20, 4) NOT NULL DEFAULT 0.0000,
    created_at           TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_product_warehouse_uniq ON product_warehouse(product_id, warehouse_id);

