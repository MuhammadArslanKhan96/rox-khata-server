package ledger_test

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/joho/godotenv"
	"rox-khata/internal/db"
	"rox-khata/internal/ledger"
)

func TestConcurrentTransfers(t *testing.T) {
	// Load environment configurations if .env is available
	_ = godotenv.Load("../../.env")

	// 1. Connect to PostgreSQL
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://postgres:postgres@localhost:5432/rox_khata?sslmode=disable"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := db.InitPool(ctx, dbURL)
	if err != nil {
		t.Skipf("Skipping integration test: database connection failed: %v", err)
	}
	defer pool.Close()

	// 2. Setup: Create schema tables if not exist (ensures test runs in isolation)
	setupSQL := `
	CREATE TABLE IF NOT EXISTS party (
		id BIGSERIAL PRIMARY KEY,
		business_id VARCHAR(255) NOT NULL,
		account_name VARCHAR(255) NOT NULL DEFAULT '',
		account_type VARCHAR(50) NOT NULL DEFAULT 'CUSTOMER',
		phone_number VARCHAR(50) DEFAULT '',
		current_balance NUMERIC(20, 4) NOT NULL DEFAULT 0.0000,
		updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
		CONSTRAINT chk_non_negative_balance CHECK (current_balance >= 0.0000)
	);
	CREATE TABLE IF NOT EXISTS journal_entries (
		id BIGSERIAL PRIMARY KEY,
		business_id VARCHAR(255) NOT NULL,
		from_account_id BIGINT NOT NULL REFERENCES party(id),
		to_account_id BIGINT NOT NULL REFERENCES party(id),
		amount NUMERIC(20, 4) NOT NULL,
		description TEXT NOT NULL,
		created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
		CONSTRAINT chk_positive_amount CHECK (amount > 0.0000),
		CONSTRAINT chk_distinct_accounts CHECK (from_account_id <> to_account_id)
	);
	`
	_, err = pool.Exec(ctx, setupSQL)
	if err != nil {
		t.Fatalf("Failed to setup test schema: %v", err)
	}

	repo := ledger.NewLedgerRepository(pool)
	svc := ledger.NewLedgerService(repo)

	businessID := "test_business_123"

	// 3. Create two accounts with $1000 each
	acc1, err := svc.CreateAccount(ctx, ledger.CreateAccountRequest{
		BusinessID:     businessID,
		InitialBalance: 1000.0,
	})
	if err != nil {
		t.Fatalf("Failed to create account 1: %v", err)
	}

	acc2, err := svc.CreateAccount(ctx, ledger.CreateAccountRequest{
		BusinessID:     businessID,
		InitialBalance: 1000.0,
	})
	if err != nil {
		t.Fatalf("Failed to create account 2: %v", err)
	}

	initialSum := acc1.CurrentBalance + acc2.CurrentBalance
	if initialSum != 2000.0 {
		t.Fatalf("Expected initial sum to be 2000, got %f", initialSum)
	}

	// 4. Fire 100 concurrent transfers:
	// - 50 transfers of $10.0 from acc1 to acc2
	// - 50 transfers of $5.0 from acc2 to acc1
	// Net expected changes:
	// acc1 balance: 1000.0 - (50 * 10.0) + (50 * 5.0) = 1000 - 500 + 250 = 750.0
	// acc2 balance: 1000.0 + (50 * 10.0) - (50 * 5.0) = 1000 + 500 - 250 = 1250.0
	// Sum must remain exactly 2000.0
	numTransfers := 50
	var wg sync.WaitGroup
	wg.Add(numTransfers * 2)

	transferFunc := func(fromID, toID int64, amount float64, desc string) {
		defer wg.Done()
		_, err := svc.Transfer(context.Background(), ledger.TransferRequest{
			BusinessID:    businessID,
			FromAccountID: fromID,
			ToAccountID:   toID,
			Amount:        amount,
			Description:   desc,
		})
		if err != nil {
			t.Errorf("Transfer failed: %v", err)
		}
	}

	// Run concurrent transfers
	for i := 0; i < numTransfers; i++ {
		go transferFunc(acc1.ID, acc2.ID, 10.0, fmt.Sprintf("Transfer 1->2 #%d", i))
		go transferFunc(acc2.ID, acc1.ID, 5.0, fmt.Sprintf("Transfer 2->1 #%d", i))
	}

	wg.Wait()

	// 5. Retrieve final balances
	finalAcc1, err := svc.GetAccount(ctx, acc1.ID)
	if err != nil {
		t.Fatalf("Failed to fetch final account 1: %v", err)
	}
	finalAcc2, err := svc.GetAccount(ctx, acc2.ID)
	if err != nil {
		t.Fatalf("Failed to fetch final account 2: %v", err)
	}

	t.Logf("Final balances: Account 1 = %.2f, Account 2 = %.2f", finalAcc1.CurrentBalance, finalAcc2.CurrentBalance)

	if finalAcc1.CurrentBalance != 750.0 {
		t.Errorf("Expected Account 1 balance to be 750.0, got %f", finalAcc1.CurrentBalance)
	}
	if finalAcc2.CurrentBalance != 1250.0 {
		t.Errorf("Expected Account 2 balance to be 1250.0, got %f", finalAcc2.CurrentBalance)
	}

	finalSum := finalAcc1.CurrentBalance + finalAcc2.CurrentBalance
	if finalSum != 2000.0 {
		t.Errorf("Expected final sum to be 2000, got %f", finalSum)
	}
}
