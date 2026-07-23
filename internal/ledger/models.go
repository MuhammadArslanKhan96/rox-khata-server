package ledger

import (
	"time"
)

// LedgerAccount represents a business-specific account holding a financial balance.
type LedgerAccount struct {
	ID             int64     `json:"id"`
	BusinessID     string    `json:"business_id"`
	CurrentBalance float64   `json:"current_balance"`
	UpdatedAt      time.Time `json:"updated_at"`
	AccountName    string    `json:"account_name"`
	AccountType    string    `json:"account_type"`
	PhoneNumber    string    `json:"phone_number"`
}

// JournalEntry represents an immutable record of money moved between two accounts.
type JournalEntry struct {
	ID            int64     `json:"id"`
	BusinessID    string    `json:"business_id"`
	FromAccountID int64     `json:"from_account_id"`
	ToAccountID   int64     `json:"to_account_id"`
	Amount        float64   `json:"amount"`
	Description   string    `json:"description"`
	CreatedAt     time.Time `json:"created_at"`
}

// CreateAccountRequest defines the schema for creating a new ledger account.
type CreateAccountRequest struct {
	BusinessID     string  `json:"business_id" binding:"required,min=1,max=255"`
	InitialBalance float64 `json:"initial_balance" binding:"gte=0"`
	AccountName    string  `json:"account_name" binding:"required"`
	AccountType    string  `json:"account_type" binding:"required"`
	PhoneNumber    string  `json:"phone_number"`
}

// RegisterTenantRequest defines the schema for registering a new tenant.
type RegisterTenantRequest struct {
	Phone        string `json:"phone" binding:"required"`
	Email        string `json:"email" binding:"required"`
	BusinessName string `json:"business_name" binding:"required"`
}

// LoginRequest defines schema for user authentication.
type LoginRequest struct {
	Login    string `json:"login" binding:"required"`
	Password string `json:"password"`
}

// LoginResponse defines schema for successful user authentication.
type LoginResponse struct {
	Status       string `json:"status"`
	Message      string `json:"message"`
	TenantPhone  string `json:"tenant_phone"`
	BusinessName string `json:"business_name"`
	Email        string `json:"email"`
	Role         string `json:"role"`
}

// TransferRequest defines the schema for creating a new ledger transfer.
type TransferRequest struct {
	BusinessID    string  `json:"business_id" binding:"required,min=1,max=255"`
	FromAccountID int64   `json:"from_account_id" binding:"required"`
	ToAccountID   int64   `json:"to_account_id" binding:"required"`
	Amount        float64 `json:"amount" binding:"required,gt=0"`
	Description   string  `json:"description" binding:"required,min=1,max=1000"`
}

// APIError represents structured error feedback returned to the client.
type APIError struct {
	Error   string `json:"error"`
	Details string `json:"details,omitempty"`
}

// ItemSyncRequest represents the item details to sync.
type ItemSyncRequest struct {
	ID            string  `json:"id" binding:"required"`
	BusinessID    string  `json:"business_id" binding:"required"`
	Name          string  `json:"name" binding:"required"`
	PurchasePrice float64 `json:"purchase_price"`
	SalePrice     float64 `json:"sale_price"`
	StockQuantity int     `json:"stock_quantity"`
}

// BankSyncRequest represents the bank details to sync.
type BankSyncRequest struct {
	ID         string  `json:"id" binding:"required"`
	BusinessID string  `json:"business_id" binding:"required"`
	BankName   string  `json:"bank_name" binding:"required"`
	AccountNo  string  `json:"account_no"`
	Balance    float64 `json:"balance"`
}

// TeamMemberSyncRequest represents the team member details to sync.
type TeamMemberSyncRequest struct {
	ID         string `json:"id" binding:"required"`
	BusinessID string `json:"business_id" binding:"required"`
	Name       string `json:"name" binding:"required"`
	Phone      string `json:"phone" binding:"required"`
	Role       string `json:"role"`
}
