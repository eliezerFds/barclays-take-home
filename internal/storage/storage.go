package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

var (
	ErrUserNotFound       = errors.New("user not found")
	ErrFailedToCreateUser = errors.New("failed to create user")
	ErrDuplicateUser      = errors.New("user already exists")

	ErrAccountNotFound       = errors.New("account not found")
	ErrFailedToCreateAccount = errors.New("failed to create account")

	ErrTransactionNotFound       = errors.New("transaction not found")
	ErrFailedToCreateTransaction = errors.New("failed to create transaction")
	ErrInsufficientFunds         = errors.New("insufficient funds")
)

type Storage struct {
	db *sqlx.DB
}

func New(db *sqlx.DB) *Storage {
	return &Storage{db: db}
}

type User struct {
	ID              string    `db:"id"`
	Name            string    `db:"name"`
	AddressLine1    string    `db:"address_line1"`
	AddressLine2    string    `db:"address_line2"`
	AddressLine3    string    `db:"address_line3"`
	AddressTown     string    `db:"address_town"`
	AddressCounty   string    `db:"address_county"`
	AddressPostcode string    `db:"address_postcode"`
	PhoneNumber     string    `db:"phone_number"`
	Email           string    `db:"email"`
	PasswordHash    string    `db:"password_hash"`
	CreatedAt       time.Time `db:"created_at"`
	UpdatedAt       time.Time `db:"updated_at"`
}

type CreateUserParams struct {
	Name            string
	AddressLine1    string
	AddressLine2    string
	AddressLine3    string
	AddressTown     string
	AddressCounty   string
	AddressPostcode string
	PhoneNumber     string
	Email           string
	PasswordHash    string
}

func (s *Storage) CreateUser(ctx context.Context, params CreateUserParams) (*User, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("failed to generate user id: %w", err)
	}
	userID := "usr-" + strings.ReplaceAll(id.String(), "-", "")
	now := time.Now().UTC()

	var user User
	err = s.db.QueryRowxContext(ctx, `
		INSERT INTO users (id, name, address_line1, address_line2, address_line3, address_town, address_county, address_postcode, phone_number, email, password_hash, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		RETURNING id, name, address_line1, address_line2, address_line3, address_town, address_county, address_postcode, phone_number, email, password_hash, created_at, updated_at`,
		userID, params.Name, params.AddressLine1, params.AddressLine2, params.AddressLine3,
		params.AddressTown, params.AddressCounty, params.AddressPostcode,
		params.PhoneNumber, params.Email, params.PasswordHash, now, now,
	).StructScan(&user)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return nil, ErrDuplicateUser
		}
		return nil, errors.Join(ErrFailedToCreateUser, err)
	}

	return &user, nil
}

func (s *Storage) GetUserByID(ctx context.Context, id string) (*User, error) {
	var user User
	err := s.db.QueryRowxContext(ctx, `
		SELECT id, name, address_line1, address_line2, address_line3, address_town, address_county, address_postcode, phone_number, email, password_hash, created_at, updated_at
		FROM users WHERE id = ?`, id).StructScan(&user)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return &user, nil
}

func (s *Storage) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	var user User
	err := s.db.QueryRowxContext(ctx, `
		SELECT id, name, address_line1, address_line2, address_line3, address_town, address_county, address_postcode, phone_number, email, password_hash, created_at, updated_at
		FROM users WHERE email = ?`, email).StructScan(&user)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return &user, nil
}

// --- Accounts ---

type Account struct {
	AccountNumber string    `db:"account_number"`
	SortCode      string    `db:"sort_code"`
	Name          string    `db:"name"`
	AccountType   string    `db:"account_type"`
	Balance       int64     `db:"balance"`
	Currency      string    `db:"currency"`
	UserID        string    `db:"user_id"`
	CreatedAt     time.Time `db:"created_at"`
	UpdatedAt     time.Time `db:"updated_at"`
}

type CreateAccountParams struct {
	Name        string
	AccountType string
	UserID      string
}

func (s *Storage) GetAccountByNumber(ctx context.Context, accountNumber string) (*Account, error) {
	var account Account
	err := s.db.QueryRowxContext(ctx, `
		SELECT account_number, sort_code, name, account_type, balance, currency, user_id, created_at, updated_at
		FROM accounts WHERE account_number = ?`, accountNumber).StructScan(&account)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrAccountNotFound
		}
		return nil, err
	}
	return &account, nil
}

// --- Transactions ---

type Transaction struct {
	ID            string    `db:"id"`
	AccountNumber string    `db:"account_number"`
	Amount        int64     `db:"amount"`
	Currency      string    `db:"currency"`
	Type          string    `db:"type"`
	Reference     string    `db:"reference"`
	CreatedAt     time.Time `db:"created_at"`
}

type CreateTransactionParams struct {
	AccountNumber string
	Amount        int64
	Currency      string
	Type          string
	Reference     string
}

func (s *Storage) CreateTransaction(ctx context.Context, params CreateTransactionParams) (*Transaction, error) {
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, errors.Join(ErrFailedToCreateTransaction, err)
	}
	defer tx.Rollback()

	now := time.Now().UTC()
	var res sql.Result
	if params.Type == "deposit" {
		res, err = tx.ExecContext(ctx,
			`UPDATE accounts SET balance = balance + ?, updated_at = ? WHERE account_number = ?`,
			params.Amount, now, params.AccountNumber)
	} else {
		// AND balance >= ? makes the check and update atomic — no separate SELECT needed.
		res, err = tx.ExecContext(ctx,
			`UPDATE accounts SET balance = balance - ?, updated_at = ? WHERE account_number = ? AND balance >= ?`,
			params.Amount, now, params.AccountNumber, params.Amount)
	}
	if err != nil {
		return nil, errors.Join(ErrFailedToCreateTransaction, err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return nil, errors.Join(ErrFailedToCreateTransaction, err)
	}
	if rows == 0 {
		if params.Type == "withdrawal" {
			return nil, ErrInsufficientFunds
		}
		return nil, ErrAccountNotFound
	}

	id, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("failed to generate transaction id: %w", err)
	}
	transactionID := "tan-" + strings.ReplaceAll(id.String(), "-", "")

	var transaction Transaction
	err = tx.QueryRowxContext(ctx, `
		INSERT INTO transactions (id, account_number, amount, currency, type, reference, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		RETURNING id, account_number, amount, currency, type, reference, created_at`,
		transactionID, params.AccountNumber, params.Amount, params.Currency, params.Type, params.Reference, now,
	).StructScan(&transaction)
	if err != nil {
		return nil, errors.Join(ErrFailedToCreateTransaction, err)
	}

	if err = tx.Commit(); err != nil {
		return nil, errors.Join(ErrFailedToCreateTransaction, err)
	}

	return &transaction, nil
}

func (s *Storage) GetTransactionByID(ctx context.Context, accountNumber, transactionID string) (*Transaction, error) {
	var transaction Transaction
	err := s.db.QueryRowxContext(ctx, `
		SELECT id, account_number, amount, currency, type, reference, created_at
		FROM transactions WHERE id = ? AND account_number = ?`, transactionID, accountNumber).StructScan(&transaction)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrTransactionNotFound
		}
		return nil, err
	}
	return &transaction, nil
}

func (s *Storage) CreateAccount(ctx context.Context, params CreateAccountParams) (*Account, error) {
	now := time.Now().UTC()

	for range 10 {
		accountNumber := fmt.Sprintf("01%06d", rand.Intn(1_000_000))

		var account Account
		err := s.db.QueryRowxContext(ctx, `
			INSERT INTO accounts (account_number, name, account_type, user_id, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?)
			RETURNING account_number, sort_code, name, account_type, balance, currency, user_id, created_at, updated_at`,
			accountNumber, params.Name, params.AccountType, params.UserID, now, now,
		).StructScan(&account)
		if err == nil {
			return &account, nil
		}
		if !strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return nil, errors.Join(ErrFailedToCreateAccount, err)
		}
	}

	return nil, ErrFailedToCreateAccount
}
