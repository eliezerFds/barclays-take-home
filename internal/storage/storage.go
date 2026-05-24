package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

var (
	ErrUserNotFound       = errors.New("user not found")
	ErrFailedToCreateUser = errors.New("failed to create user")
	ErrDuplicateUser      = errors.New("user already exists")
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
