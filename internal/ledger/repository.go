package ledger

import (
	"context"
	"errors"

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

// LedgerRepository defines interface for DB operations.
type LedgerRepository interface {
	GetPool() *pgxpool.Pool
	CreateAccount(ctx context.Context, db DBTX, businessID string, initialBalance float64, accountName string, accountType string, phoneNumber string) (*LedgerAccount, error)
	GetAccount(ctx context.Context, db DBTX, id int64) (*LedgerAccount, error)
	GetAccountForUpdate(ctx context.Context, db DBTX, id int64) (*LedgerAccount, error)
	UpdateAccountBalance(ctx context.Context, db DBTX, id int64, newBalance float64) error
	CreateJournalEntry(ctx context.Context, db DBTX, businessID string, fromAccountID, toAccountID int64, amount float64, description string) (*JournalEntry, error)
	GetJournalEntries(ctx context.Context, businessID string, accountID int64) ([]JournalEntry, error)
	UpsertItem(ctx context.Context, db DBTX, item ItemSyncRequest) error
	UpsertBank(ctx context.Context, db DBTX, bank BankSyncRequest) error
	UpsertTeamMember(ctx context.Context, db DBTX, member TeamMemberSyncRequest) error
	UpsertTenant(ctx context.Context, db DBTX, phone string, name string) error
	RegisterTenant(ctx context.Context, db DBTX, phone string, email string, name string) error
	GetItems(ctx context.Context, businessID string) ([]ItemSyncRequest, error)
	GetBanks(ctx context.Context, businessID string) ([]BankSyncRequest, error)
	GetTeamMembers(ctx context.Context, businessID string) ([]TeamMemberSyncRequest, error)
	GetAccounts(ctx context.Context, businessID string) ([]LedgerAccount, error)
}

type postgresLedgerRepository struct {
	pool *pgxpool.Pool
}

// NewLedgerRepository creates a new postgresLedgerRepository instance.
func NewLedgerRepository(pool *pgxpool.Pool) LedgerRepository {
	return &postgresLedgerRepository{pool: pool}
}

func (r *postgresLedgerRepository) GetPool() *pgxpool.Pool {
	return r.pool
}

// CreateAccount inserts a new account in the database.
func (r *postgresLedgerRepository) CreateAccount(ctx context.Context, db DBTX, businessID string, initialBalance float64, accountName string, accountType string, phoneNumber string) (*LedgerAccount, error) {
	// Auto-upsert tenant
	_ = r.UpsertTenant(ctx, db, businessID, "My Business")

	// Check if account already exists for this business with same name
	if accountName != "" {
		var existing LedgerAccount
		checkQuery := `
			SELECT backend_id, business_id, current_balance, created_at, account_name, account_type, COALESCE(phone_number, '')
			FROM party
			WHERE business_id = $1 AND account_name = $2 LIMIT 1
		`
		err := db.QueryRow(ctx, checkQuery, businessID, accountName).Scan(
			&existing.ID, &existing.BusinessID, &existing.CurrentBalance, &existing.UpdatedAt,
			&existing.AccountName, &existing.AccountType, &existing.PhoneNumber,
		)
		if err == nil {
			return &existing, nil
		}
	}

	uId := uuid.New().String()
	query := `
		INSERT INTO party (id, business_id, current_balance, account_name, account_type, phone_number, last_updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, 0)
		RETURNING backend_id, business_id, current_balance, created_at, account_name, account_type, COALESCE(phone_number, '')
	`
	row := db.QueryRow(ctx, query, uId, businessID, initialBalance, accountName, accountType, phoneNumber)
	
	var account LedgerAccount
	err := row.Scan(&account.ID, &account.BusinessID, &account.CurrentBalance, &account.UpdatedAt, &account.AccountName, &account.AccountType, &account.PhoneNumber)
	if err != nil {
		return nil, err
	}
	return &account, nil
}

// GetAccount retrieves a ledger account by id without locking.
func (r *postgresLedgerRepository) GetAccount(ctx context.Context, db DBTX, id int64) (*LedgerAccount, error) {
	query := `
		SELECT backend_id, business_id, current_balance, created_at, account_name, account_type, COALESCE(phone_number, '')
		FROM party
		WHERE backend_id = $1
	`
	row := db.QueryRow(ctx, query, id)
	
	var account LedgerAccount
	err := row.Scan(&account.ID, &account.BusinessID, &account.CurrentBalance, &account.UpdatedAt, &account.AccountName, &account.AccountType, &account.PhoneNumber)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil // Return nil, nil when account is not found
		}
		return nil, err
	}
	return &account, nil
}

// GetAccountForUpdate locks the account row using SELECT ... FOR UPDATE.
func (r *postgresLedgerRepository) GetAccountForUpdate(ctx context.Context, db DBTX, id int64) (*LedgerAccount, error) {
	query := `
		SELECT backend_id, business_id, current_balance, created_at, account_name, account_type, COALESCE(phone_number, '')
		FROM party
		WHERE backend_id = $1
		FOR UPDATE
	`
	row := db.QueryRow(ctx, query, id)
	
	var account LedgerAccount
	err := row.Scan(&account.ID, &account.BusinessID, &account.CurrentBalance, &account.UpdatedAt, &account.AccountName, &account.AccountType, &account.PhoneNumber)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &account, nil
}

// UpdateAccountBalance updates the balance of the specified account.
func (r *postgresLedgerRepository) UpdateAccountBalance(ctx context.Context, db DBTX, id int64, newBalance float64) error {
	query := `
		UPDATE party
		SET current_balance = $1
		WHERE backend_id = $2
	`
	_, err := db.Exec(ctx, query, newBalance, id)
	return err
}

// CreateJournalEntry inserts an immutable record into the journal_entries table.
func (r *postgresLedgerRepository) CreateJournalEntry(ctx context.Context, db DBTX, businessID string, fromAccountID, toAccountID int64, amount float64, description string) (*JournalEntry, error) {
	query := `
		INSERT INTO journal_entries (business_id, from_account_id, to_account_id, amount, description, created_at)
		VALUES ($1, $2, $3, $4, $5, CURRENT_TIMESTAMP)
		RETURNING id, business_id, from_account_id, to_account_id, amount, description, created_at
	`
	row := db.QueryRow(ctx, query, businessID, fromAccountID, toAccountID, amount, description)
	
	var entry JournalEntry
	err := row.Scan(&entry.ID, &entry.BusinessID, &entry.FromAccountID, &entry.ToAccountID, &entry.Amount, &entry.Description, &entry.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &entry, nil
}

// GetJournalEntries returns the transaction ledger history for a business and account.
func (r *postgresLedgerRepository) GetJournalEntries(ctx context.Context, businessID string, accountID int64) ([]JournalEntry, error) {
	query := `
		SELECT id, business_id, from_account_id, to_account_id, amount, description, created_at
		FROM journal_entries
		WHERE business_id = $1 AND (from_account_id = $2 OR to_account_id = $2)
		ORDER BY created_at DESC
	`
	// This does not require locking, can run directly on the pool
	rows, err := r.pool.Query(ctx, query, businessID, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var entries []JournalEntry
	for rows.Next() {
		var entry JournalEntry
		err := rows.Scan(&entry.ID, &entry.BusinessID, &entry.FromAccountID, &entry.ToAccountID, &entry.Amount, &entry.Description, &entry.CreatedAt)
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	
	return entries, rows.Err()
}

// UpsertItem inserts or updates an item on conflict.
func (r *postgresLedgerRepository) UpsertItem(ctx context.Context, db DBTX, item ItemSyncRequest) error {
	_ = r.UpsertTenant(ctx, db, item.BusinessID, "My Business")

	query := `
		INSERT INTO items (id, business_id, name, purchase_price, sale_price, stock_quantity)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (id) DO UPDATE SET
			name = EXCLUDED.name,
			purchase_price = EXCLUDED.purchase_price,
			sale_price = EXCLUDED.sale_price,
			stock_quantity = EXCLUDED.stock_quantity
	`
	_, err := db.Exec(ctx, query, item.ID, item.BusinessID, item.Name, item.PurchasePrice, item.SalePrice, item.StockQuantity)
	return err
}

// UpsertBank inserts or updates a bank account on conflict.
func (r *postgresLedgerRepository) UpsertBank(ctx context.Context, db DBTX, bank BankSyncRequest) error {
	_ = r.UpsertTenant(ctx, db, bank.BusinessID, "My Business")

	query := `
		INSERT INTO banks (id, business_id, bank_name, account_no, balance)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (id) DO UPDATE SET
			bank_name = EXCLUDED.bank_name,
			account_no = EXCLUDED.account_no,
			balance = EXCLUDED.balance
	`
	_, err := db.Exec(ctx, query, bank.ID, bank.BusinessID, bank.BankName, bank.AccountNo, bank.Balance)
	return err
}

// UpsertTeamMember inserts or updates a team member on conflict.
func (r *postgresLedgerRepository) UpsertTeamMember(ctx context.Context, db DBTX, member TeamMemberSyncRequest) error {
	_ = r.UpsertTenant(ctx, db, member.BusinessID, "My Business")

	query := `
		INSERT INTO staff (id, business_id, name, phone, role)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (id) DO UPDATE SET
			name = EXCLUDED.name,
			phone = EXCLUDED.phone,
			role = EXCLUDED.role
	`
	_, err := db.Exec(ctx, query, member.ID, member.BusinessID, member.Name, member.Phone, member.Role)
	return err
}

// UpsertTenant inserts or ignores a tenant and registers their owner user in the database.
func (r *postgresLedgerRepository) UpsertTenant(ctx context.Context, db DBTX, phone string, name string) error {
	if name == "" {
		name = "My Business"
	}
	queryTenant := `
		INSERT INTO tenants (phone, business_name, created_at)
		VALUES ($1, $2, CURRENT_TIMESTAMP)
		ON CONFLICT (phone) DO UPDATE SET
			business_name = CASE WHEN tenants.business_name = 'My Business' AND EXCLUDED.business_name != 'My Business' THEN EXCLUDED.business_name ELSE tenants.business_name END
	`
	_, err := db.Exec(ctx, queryTenant, phone, name)
	if err != nil {
		return err
	}

	queryUser := `
		INSERT INTO users (tenant_phone, username, phone, role)
		VALUES ($1, 'Owner', $1, 'OWNER')
		ON CONFLICT (phone) DO NOTHING
	`
	_, err = db.Exec(ctx, queryUser, phone)
	return err
}

// RegisterTenant creates a new business tenant and owner user, enforcing phone and email uniqueness.
func (r *postgresLedgerRepository) RegisterTenant(ctx context.Context, db DBTX, phone string, email string, name string) error {
	queryTenant := `
		INSERT INTO tenants (phone, business_name, email, created_at)
		VALUES ($1, $2, $3, CURRENT_TIMESTAMP)
		ON CONFLICT (phone) DO UPDATE SET
			business_name = EXCLUDED.business_name,
			email = EXCLUDED.email
	`
	_, err := db.Exec(ctx, queryTenant, phone, name, email)
	if err != nil {
		return err
	}

	queryUser := `
		INSERT INTO users (tenant_phone, username, email, phone, role)
		VALUES ($1, 'Owner', $2, $1, 'OWNER')
		ON CONFLICT (phone) DO NOTHING
	`
	_, err = db.Exec(ctx, queryUser, phone, email)
	return err
}

func (r *postgresLedgerRepository) GetItems(ctx context.Context, businessID string) ([]ItemSyncRequest, error) {
	query := `SELECT id, business_id, name, purchase_price, sale_price, stock_quantity FROM items WHERE business_id = $1`
	rows, err := r.pool.Query(ctx, query, businessID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []ItemSyncRequest
	for rows.Next() {
		var item ItemSyncRequest
		if err := rows.Scan(&item.ID, &item.BusinessID, &item.Name, &item.PurchasePrice, &item.SalePrice, &item.StockQuantity); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (r *postgresLedgerRepository) GetBanks(ctx context.Context, businessID string) ([]BankSyncRequest, error) {
	query := `SELECT id, business_id, bank_name, account_no, balance FROM banks WHERE business_id = $1`
	rows, err := r.pool.Query(ctx, query, businessID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var banks []BankSyncRequest
	for rows.Next() {
		var bank BankSyncRequest
		if err := rows.Scan(&bank.ID, &bank.BusinessID, &bank.BankName, &bank.AccountNo, &bank.Balance); err != nil {
			return nil, err
		}
		banks = append(banks, bank)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return banks, nil
}

func (r *postgresLedgerRepository) GetTeamMembers(ctx context.Context, businessID string) ([]TeamMemberSyncRequest, error) {
	query := `SELECT id, business_id, name, phone, role FROM staff WHERE business_id = $1`
	rows, err := r.pool.Query(ctx, query, businessID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var members []TeamMemberSyncRequest
	for rows.Next() {
		var member TeamMemberSyncRequest
		if err := rows.Scan(&member.ID, &member.BusinessID, &member.Name, &member.Phone, &member.Role); err != nil {
			return nil, err
		}
		members = append(members, member)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return members, nil
}

func (r *postgresLedgerRepository) GetAccounts(ctx context.Context, businessID string) ([]LedgerAccount, error) {
	query := `SELECT backend_id, business_id, current_balance, created_at, account_name, account_type, COALESCE(phone_number, '') FROM party WHERE business_id = $1`
	rows, err := r.pool.Query(ctx, query, businessID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var accounts []LedgerAccount
	for rows.Next() {
		var account LedgerAccount
		if err := rows.Scan(&account.ID, &account.BusinessID, &account.CurrentBalance, &account.UpdatedAt, &account.AccountName, &account.AccountType, &account.PhoneNumber); err != nil {
			return nil, err
		}
		accounts = append(accounts, account)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return accounts, nil
}
