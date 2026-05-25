package storage

import (
	"context"
	"testing"

	"barclays/database/migrations"
	_ "github.com/glebarez/go-sqlite"
	"github.com/jmoiron/sqlx"
	migrate "github.com/rubenv/sql-migrate"
)

func newTestDB(t *testing.T) *Storage {
	t.Helper()
	db, err := sqlx.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	mg := migrations.GetMigrationSource()
	migrate.SetTable("migrations")
	if _, err = migrate.Exec(db.DB, "sqlite3", mg, migrate.Up); err != nil {
		t.Fatal(err)
	}
	return New(db)
}

var baseUserParams = CreateUserParams{
	Name:            "Test User",
	AddressLine1:    "1 Test Street",
	AddressTown:     "London",
	AddressCounty:   "Greater London",
	AddressPostcode: "SW1A 1AA",
	PhoneNumber:     "+447911123456",
	Email:           "test@example.com",
	PasswordHash:    "hashedpassword",
}

// --- Users ---

func TestCreateUser(t *testing.T) {
	repo := newTestDB(t)
	ctx := context.Background()

	user, err := repo.CreateUser(ctx, baseUserParams)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user.ID == "" {
		t.Error("expected non-empty ID")
	}
	if user.Email != baseUserParams.Email {
		t.Errorf("email: got %q, want %q", user.Email, baseUserParams.Email)
	}
	if user.Name != baseUserParams.Name {
		t.Errorf("name: got %q, want %q", user.Name, baseUserParams.Name)
	}
}

func TestCreateUser_DuplicateEmail(t *testing.T) {
	repo := newTestDB(t)
	ctx := context.Background()

	if _, err := repo.CreateUser(ctx, baseUserParams); err != nil {
		t.Fatalf("first create: %v", err)
	}
	if _, err := repo.CreateUser(ctx, baseUserParams); err != ErrDuplicateUser {
		t.Errorf("expected ErrDuplicateUser, got %v", err)
	}
}

func TestGetUserByID(t *testing.T) {
	repo := newTestDB(t)
	ctx := context.Background()

	created, _ := repo.CreateUser(ctx, baseUserParams)

	user, err := repo.GetUserByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user.ID != created.ID {
		t.Errorf("id: got %q, want %q", user.ID, created.ID)
	}
}

func TestGetUserByID_NotFound(t *testing.T) {
	repo := newTestDB(t)
	ctx := context.Background()

	if _, err := repo.GetUserByID(ctx, "usr-doesnotexist"); err != ErrUserNotFound {
		t.Errorf("expected ErrUserNotFound, got %v", err)
	}
}

func TestGetUserByEmail(t *testing.T) {
	repo := newTestDB(t)
	ctx := context.Background()

	repo.CreateUser(ctx, baseUserParams)

	user, err := repo.GetUserByEmail(ctx, baseUserParams.Email)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user.Email != baseUserParams.Email {
		t.Errorf("email: got %q, want %q", user.Email, baseUserParams.Email)
	}
}

func TestGetUserByEmail_NotFound(t *testing.T) {
	repo := newTestDB(t)
	ctx := context.Background()

	if _, err := repo.GetUserByEmail(ctx, "nobody@example.com"); err != ErrUserNotFound {
		t.Errorf("expected ErrUserNotFound, got %v", err)
	}
}

// --- Accounts ---

func seedUser(t *testing.T, repo *Storage) *User {
	t.Helper()
	user, err := repo.CreateUser(context.Background(), baseUserParams)
	if err != nil {
		t.Fatalf("seedUser: %v", err)
	}
	return user
}

func seedAccount(t *testing.T, repo *Storage, userID string) *Account {
	t.Helper()
	account, err := repo.CreateAccount(context.Background(), CreateAccountParams{
		Name:        "Test Account",
		AccountType: "personal",
		UserID:      userID,
	})
	if err != nil {
		t.Fatalf("seedAccount: %v", err)
	}
	return account
}

func TestCreateAccount(t *testing.T) {
	repo := newTestDB(t)
	user := seedUser(t, repo)

	account, err := repo.CreateAccount(context.Background(), CreateAccountParams{
		Name:        "My Account",
		AccountType: "personal",
		UserID:      user.ID,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if account.AccountNumber == "" {
		t.Error("expected non-empty account number")
	}
	if account.Balance != 0 {
		t.Errorf("balance: got %d, want 0", account.Balance)
	}
	if account.Currency != "GBP" {
		t.Errorf("currency: got %q, want GBP", account.Currency)
	}
	if account.SortCode != "10-10-10" {
		t.Errorf("sort code: got %q, want 10-10-10", account.SortCode)
	}
}

func TestGetAccountByNumber(t *testing.T) {
	repo := newTestDB(t)
	user := seedUser(t, repo)
	created := seedAccount(t, repo, user.ID)

	account, err := repo.GetAccountByNumber(context.Background(), created.AccountNumber)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if account.AccountNumber != created.AccountNumber {
		t.Errorf("accountNumber: got %q, want %q", account.AccountNumber, created.AccountNumber)
	}
	if account.UserID != user.ID {
		t.Errorf("userID: got %q, want %q", account.UserID, user.ID)
	}
}

func TestGetAccountByNumber_NotFound(t *testing.T) {
	repo := newTestDB(t)

	if _, err := repo.GetAccountByNumber(context.Background(), "01999999"); err != ErrAccountNotFound {
		t.Errorf("expected ErrAccountNotFound, got %v", err)
	}
}

// --- Transactions ---

func TestCreateTransaction_Deposit(t *testing.T) {
	repo := newTestDB(t)
	ctx := context.Background()
	user := seedUser(t, repo)
	account := seedAccount(t, repo, user.ID)

	tx, err := repo.CreateTransaction(ctx, CreateTransactionParams{
		AccountNumber: account.AccountNumber,
		Amount:        5000,
		Currency:      "GBP",
		Type:          "deposit",
		Reference:     "salary",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tx.ID == "" {
		t.Error("expected non-empty transaction ID")
	}
	if tx.Amount != 5000 {
		t.Errorf("amount: got %d, want 5000", tx.Amount)
	}
	if tx.Reference != "salary" {
		t.Errorf("reference: got %q, want salary", tx.Reference)
	}

	updated, _ := repo.GetAccountByNumber(ctx, account.AccountNumber)
	if updated.Balance != 5000 {
		t.Errorf("balance after deposit: got %d, want 5000", updated.Balance)
	}
}

func TestCreateTransaction_Withdrawal(t *testing.T) {
	repo := newTestDB(t)
	ctx := context.Background()
	user := seedUser(t, repo)
	account := seedAccount(t, repo, user.ID)

	repo.CreateTransaction(ctx, CreateTransactionParams{
		AccountNumber: account.AccountNumber,
		Amount:        10000,
		Currency:      "GBP",
		Type:          "deposit",
	})

	_, err := repo.CreateTransaction(ctx, CreateTransactionParams{
		AccountNumber: account.AccountNumber,
		Amount:        4050,
		Currency:      "GBP",
		Type:          "withdrawal",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	updated, _ := repo.GetAccountByNumber(ctx, account.AccountNumber)
	if updated.Balance != 5950 {
		t.Errorf("balance after withdrawal: got %d, want 5950", updated.Balance)
	}
}

func TestCreateTransaction_InsufficientFunds(t *testing.T) {
	repo := newTestDB(t)
	ctx := context.Background()
	user := seedUser(t, repo)
	account := seedAccount(t, repo, user.ID)

	_, err := repo.CreateTransaction(ctx, CreateTransactionParams{
		AccountNumber: account.AccountNumber,
		Amount:        100,
		Currency:      "GBP",
		Type:          "withdrawal",
	})
	if err != ErrInsufficientFunds {
		t.Errorf("expected ErrInsufficientFunds, got %v", err)
	}

	updated, _ := repo.GetAccountByNumber(ctx, account.AccountNumber)
	if updated.Balance != 0 {
		t.Errorf("balance should still be 0, got %d", updated.Balance)
	}
}

func TestCreateTransaction_BalanceConsistency(t *testing.T) {
	repo := newTestDB(t)
	ctx := context.Background()
	user := seedUser(t, repo)
	account := seedAccount(t, repo, user.ID)

	steps := []struct {
		txType  string
		amount  int64
		wantBal int64
	}{
		{"deposit", 10000, 10000},
		{"withdrawal", 3000, 7000},
		{"deposit", 500, 7500},
		{"withdrawal", 7500, 0},
	}

	for _, s := range steps {
		_, err := repo.CreateTransaction(ctx, CreateTransactionParams{
			AccountNumber: account.AccountNumber,
			Amount:        s.amount,
			Currency:      "GBP",
			Type:          s.txType,
		})
		if err != nil {
			t.Fatalf("%s %d: %v", s.txType, s.amount, err)
		}
		updated, _ := repo.GetAccountByNumber(ctx, account.AccountNumber)
		if updated.Balance != s.wantBal {
			t.Errorf("after %s %d: balance got %d, want %d", s.txType, s.amount, updated.Balance, s.wantBal)
		}
	}
}

func TestGetTransactionByID(t *testing.T) {
	repo := newTestDB(t)
	ctx := context.Background()
	user := seedUser(t, repo)
	account := seedAccount(t, repo, user.ID)

	repo.CreateTransaction(ctx, CreateTransactionParams{
		AccountNumber: account.AccountNumber,
		Amount:        5000,
		Currency:      "GBP",
		Type:          "deposit",
	})
	created, _ := repo.CreateTransaction(ctx, CreateTransactionParams{
		AccountNumber: account.AccountNumber,
		Amount:        1234,
		Currency:      "GBP",
		Type:          "withdrawal",
		Reference:     "coffee",
	})

	tx, err := repo.GetTransactionByID(ctx, account.AccountNumber, created.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tx.ID != created.ID {
		t.Errorf("id: got %q, want %q", tx.ID, created.ID)
	}
	if tx.Amount != 1234 {
		t.Errorf("amount: got %d, want 1234", tx.Amount)
	}
	if tx.Reference != "coffee" {
		t.Errorf("reference: got %q, want coffee", tx.Reference)
	}
}

func TestGetTransactionByID_NotFound(t *testing.T) {
	repo := newTestDB(t)
	ctx := context.Background()
	user := seedUser(t, repo)
	account := seedAccount(t, repo, user.ID)

	if _, err := repo.GetTransactionByID(ctx, account.AccountNumber, "tan-doesnotexist"); err != ErrTransactionNotFound {
		t.Errorf("expected ErrTransactionNotFound, got %v", err)
	}
}

func TestGetTransactionByID_WrongAccount(t *testing.T) {
	repo := newTestDB(t)
	ctx := context.Background()

	params2 := baseUserParams
	params2.Email = "other@example.com"
	user1 := seedUser(t, repo)
	user2, _ := repo.CreateUser(ctx, params2)

	account1 := seedAccount(t, repo, user1.ID)
	account2 := seedAccount(t, repo, user2.ID)

	repo.CreateTransaction(ctx, CreateTransactionParams{
		AccountNumber: account1.AccountNumber,
		Amount:        1000,
		Currency:      "GBP",
		Type:          "deposit",
	})
	tx, _ := repo.CreateTransaction(ctx, CreateTransactionParams{
		AccountNumber: account1.AccountNumber,
		Amount:        500,
		Currency:      "GBP",
		Type:          "withdrawal",
	})

	if _, err := repo.GetTransactionByID(ctx, account2.AccountNumber, tx.ID); err != ErrTransactionNotFound {
		t.Errorf("expected ErrTransactionNotFound for wrong account, got %v", err)
	}
}
