package ledger

import (
	"context"
	"errors"
	"fmt"
	"log"

	"rox-khata/internal/email"
)

// Standard Business Rule Errors
var (
	ErrAccountNotFound      = errors.New("account not found")
	ErrTenantMismatch       = errors.New("account does not belong to the specified business")
	ErrInsufficientFunds    = errors.New("insufficient funds in source account")
	ErrSameAccountTransfer  = errors.New("cannot transfer funds to the same account")
	ErrInvalidAmount        = errors.New("transfer amount must be greater than zero")
)

// LedgerService interface defines the business operations.
type LedgerService interface {
	CreateAccount(ctx context.Context, req CreateAccountRequest) (*LedgerAccount, error)
	RegisterTenant(ctx context.Context, req RegisterTenantRequest) error
	VerifyEmail(ctx context.Context, req VerifyEmailRequest) error
	ResendVerificationCode(ctx context.Context, req ResendCodeRequest) error
	GetAccount(ctx context.Context, id int64) (*LedgerAccount, error)
	Transfer(ctx context.Context, req TransferRequest) (*JournalEntry, error)
	GetStatement(ctx context.Context, businessID string, accountID int64) ([]JournalEntry, error)
	SyncItem(ctx context.Context, req ItemSyncRequest) error
	SyncBank(ctx context.Context, req BankSyncRequest) error
	SyncTeamMember(ctx context.Context, req TeamMemberSyncRequest) error
	GetItems(ctx context.Context, businessID string) ([]ItemSyncRequest, error)
	GetBanks(ctx context.Context, businessID string) ([]BankSyncRequest, error)
	GetTeamMembers(ctx context.Context, businessID string) ([]TeamMemberSyncRequest, error)
	GetAccounts(ctx context.Context, businessID string) ([]LedgerAccount, error)
	LoginTenant(ctx context.Context, req LoginRequest) (*LoginResponse, error)
}

type ledgerService struct {
	repo LedgerRepository
}

// NewLedgerService creates a new LedgerService.
func NewLedgerService(repo LedgerRepository) LedgerService {
	return &ledgerService{repo: repo}
}

func (s *ledgerService) RegisterTenant(ctx context.Context, req RegisterTenantRequest) error {
	otpCode, err := s.repo.RegisterTenant(ctx, s.repo.GetPool(), req.Phone, req.Email, req.BusinessName, req.Password)
	if err != nil {
		return err
	}

	// Dispatch email verification OTP asynchronously
	go func() {
		mailer := email.NewMailer()
		if err := mailer.SendVerificationEmail(req.Email, req.BusinessName, otpCode); err != nil {
			log.Printf("[Warning] Failed to send verification email to %s: %v", req.Email, err)
		} else {
			log.Printf("[Email Success] Verification OTP [%s] sent to %s", otpCode, req.Email)
		}
	}()

	return nil
}

func (s *ledgerService) VerifyEmail(ctx context.Context, req VerifyEmailRequest) error {
	return s.repo.VerifyTenantEmail(ctx, req.Email, req.Code)
}

func (s *ledgerService) ResendVerificationCode(ctx context.Context, req ResendCodeRequest) error {
	otpCode, businessName, err := s.repo.ResendVerificationCode(ctx, req.Email)
	if err != nil {
		return err
	}

	go func() {
		mailer := email.NewMailer()
		if err := mailer.SendVerificationEmail(req.Email, businessName, otpCode); err != nil {
			log.Printf("[Warning] Failed to resend verification email to %s: %v", req.Email, err)
		} else {
			log.Printf("[Email Success] Resent Verification OTP [%s] to %s", otpCode, req.Email)
		}
	}()

	return nil
}

func (s *ledgerService) CreateAccount(ctx context.Context, req CreateAccountRequest) (*LedgerAccount, error) {
	// Execute CreateAccount directly on the pool (no tx needed for single insert)
	return s.repo.CreateAccount(ctx, s.repo.GetPool(), req.BusinessID, req.InitialBalance, req.AccountName, req.AccountType, req.PhoneNumber)
}

func (s *ledgerService) GetAccount(ctx context.Context, id int64) (*LedgerAccount, error) {
	account, err := s.repo.GetAccount(ctx, s.repo.GetPool(), id)
	if err != nil {
		return nil, err
	}
	if account == nil {
		return nil, ErrAccountNotFound
	}
	return account, nil
}

func (s *ledgerService) GetStatement(ctx context.Context, businessID string, accountID int64) ([]JournalEntry, error) {
	// First verify the account exists and belongs to the tenant
	account, err := s.GetAccount(ctx, accountID)
	if err != nil {
		return nil, err
	}
	if account.BusinessID != businessID {
		return nil, ErrTenantMismatch
	}

	return s.repo.GetJournalEntries(ctx, businessID, accountID)
}

// Transfer performs a safe concurrent transfer inside an atomic transaction.
func (s *ledgerService) Transfer(ctx context.Context, req TransferRequest) (*JournalEntry, error) {
	if req.FromAccountID == req.ToAccountID {
		return nil, ErrSameAccountTransfer
	}
	if req.Amount <= 0 {
		return nil, ErrInvalidAmount
	}

	// 1. Begin atomic database transaction
	tx, err := s.repo.GetPool().Begin(ctx)
	if err != nil {
		log.Printf("[Error] Failed to begin database transaction: %v", err)
		return nil, fmt.Errorf("transaction error: %w", err)
	}

	// 2. Fail-Safe Rollback: ensure rollback is called if commit does not occur
	defer func() {
		// Rollback is a safe no-op if the transaction has already been committed
		if rollbackErr := tx.Rollback(ctx); rollbackErr != nil && !errors.Is(rollbackErr, context.Canceled) {
			// Do not log if transaction is already committed/closed
			// pgx Rollback returns ErrTxClosed if transaction is already closed, which is safe to ignore
		}
	}()

	// 3. Resolve locking order (Deterministic Locking) to prevent deadlocks.
	// We lock the account with the smaller ID first, then the larger ID.
	var fromAccount, toAccount *LedgerAccount
	if req.FromAccountID < req.ToAccountID {
		// Lock smaller ID first
		fromAccount, err = s.repo.GetAccountForUpdate(ctx, tx, req.FromAccountID)
		if err != nil {
			return nil, fmt.Errorf("failed to lock source account %d: %w", req.FromAccountID, err)
		}
		
		toAccount, err = s.repo.GetAccountForUpdate(ctx, tx, req.ToAccountID)
		if err != nil {
			return nil, fmt.Errorf("failed to lock destination account %d: %w", req.ToAccountID, err)
		}
	} else {
		// Lock smaller ID first (which is destination account in this branch)
		toAccount, err = s.repo.GetAccountForUpdate(ctx, tx, req.ToAccountID)
		if err != nil {
			return nil, fmt.Errorf("failed to lock destination account %d: %w", req.ToAccountID, err)
		}

		fromAccount, err = s.repo.GetAccountForUpdate(ctx, tx, req.FromAccountID)
		if err != nil {
			return nil, fmt.Errorf("failed to lock source account %d: %w", req.FromAccountID, err)
		}
	}

	// 4. Validate Account Existence
	if fromAccount == nil {
		return nil, fmt.Errorf("source account %d: %w", req.FromAccountID, ErrAccountNotFound)
	}
	if toAccount == nil {
		return nil, fmt.Errorf("destination account %d: %w", req.ToAccountID, ErrAccountNotFound)
	}

	// 5. Tenant Isolation Checks
	if fromAccount.BusinessID != req.BusinessID {
		return nil, fmt.Errorf("source account: %w", ErrTenantMismatch)
	}
	if toAccount.BusinessID != req.BusinessID {
		return nil, fmt.Errorf("destination account: %w", ErrTenantMismatch)
	}

	// 6. Balance validation (Double-spending & Overdraft check)
	if fromAccount.CurrentBalance < req.Amount {
		return nil, fmt.Errorf("account %d has balance %.4f: %w", fromAccount.ID, fromAccount.CurrentBalance, ErrInsufficientFunds)
	}

	// 7. Calculate new balances
	newFromBalance := fromAccount.CurrentBalance - req.Amount
	newToBalance := toAccount.CurrentBalance + req.Amount

	// 8. Update accounts in the database
	err = s.repo.UpdateAccountBalance(ctx, tx, fromAccount.ID, newFromBalance)
	if err != nil {
		log.Printf("[Error] Failed to update source account balance: %v", err)
		return nil, fmt.Errorf("failed to update source balance: %w", err)
	}

	err = s.repo.UpdateAccountBalance(ctx, tx, toAccount.ID, newToBalance)
	if err != nil {
		log.Printf("[Error] Failed to update destination account balance: %v", err)
		return nil, fmt.Errorf("failed to update destination balance: %w", err)
	}

	// 9. Insert the immutable audit record (Append-Only)
	entry, err := s.repo.CreateJournalEntry(ctx, tx, req.BusinessID, req.FromAccountID, req.ToAccountID, req.Amount, req.Description)
	if err != nil {
		log.Printf("[Error] Failed to insert journal entry: %v", err)
		return nil, fmt.Errorf("failed to insert journal entry: %w", err)
	}

	// 10. Commit transaction
	err = tx.Commit(ctx)
	if err != nil {
		log.Printf("[Error] Failed to commit database transaction: %v", err)
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return entry, nil
}

func (s *ledgerService) SyncItem(ctx context.Context, req ItemSyncRequest) error {
	return s.repo.UpsertItem(ctx, s.repo.GetPool(), req)
}

func (s *ledgerService) SyncBank(ctx context.Context, req BankSyncRequest) error {
	return s.repo.UpsertBank(ctx, s.repo.GetPool(), req)
}

func (s *ledgerService) SyncTeamMember(ctx context.Context, req TeamMemberSyncRequest) error {
	return s.repo.UpsertTeamMember(ctx, s.repo.GetPool(), req)
}

func (s *ledgerService) GetItems(ctx context.Context, businessID string) ([]ItemSyncRequest, error) {
	return s.repo.GetItems(ctx, businessID)
}

func (s *ledgerService) GetBanks(ctx context.Context, businessID string) ([]BankSyncRequest, error) {
	return s.repo.GetBanks(ctx, businessID)
}

func (s *ledgerService) GetTeamMembers(ctx context.Context, businessID string) ([]TeamMemberSyncRequest, error) {
	return s.repo.GetTeamMembers(ctx, businessID)
}

func (s *ledgerService) GetAccounts(ctx context.Context, businessID string) ([]LedgerAccount, error) {
	return s.repo.GetAccounts(ctx, businessID)
}

func (s *ledgerService) LoginTenant(ctx context.Context, req LoginRequest) (*LoginResponse, error) {
	return s.repo.AuthenticateTenant(ctx, req.Login, req.Password)
}
