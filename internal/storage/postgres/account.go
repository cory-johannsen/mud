package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

// Role constants for account privilege levels.
const (
	RolePlayer = "player"
	RoleEditor = "editor"
	RoleAdmin  = "admin"
)

// ValidRole reports whether role is a recognised privilege level.
func ValidRole(role string) bool {
	switch role {
	case RolePlayer, RoleEditor, RoleAdmin:
		return true
	}
	return false
}

// ErrInvalidRole is returned when an unrecognised role string is supplied.
var ErrInvalidRole = errors.New("invalid role")

// Account represents a player account in the database.
type Account struct {
	ID           int64
	Username     string
	PasswordHash string
	Role         string
	CreatedAt    time.Time
}

// ErrAccountNotFound is returned when an account lookup yields no results.
var ErrAccountNotFound = errors.New("account not found")

// ErrAccountExists is returned when attempting to create a duplicate username.
var ErrAccountExists = errors.New("account already exists")

// ErrInvalidCredentials is returned when authentication fails.
var ErrInvalidCredentials = errors.New("invalid credentials")

// AccountRepository provides account persistence operations.
type AccountRepository struct {
	db *pgxpool.Pool
}

// NewAccountRepository creates an AccountRepository backed by the given pool.
//
// Precondition: db must be a valid, open connection pool.
func NewAccountRepository(db *pgxpool.Pool) *AccountRepository {
	return &AccountRepository{db: db}
}

// Create inserts a new account with a bcrypt-hashed password.
//
// Precondition: username must be non-empty; password must be non-empty.
// Postcondition: Returns the created Account with ID and CreatedAt set,
// or ErrAccountExists if the username is taken.
func (r *AccountRepository) Create(ctx context.Context, username, password string) (Account, error) {
	hash, err := HashPassword(password)
	if err != nil {
		return Account{}, fmt.Errorf("hashing password: %w", err)
	}

	var acct Account
	err = r.db.QueryRow(ctx,
		`INSERT INTO accounts (username, password_hash)
		 VALUES ($1, $2)
		 RETURNING id, username, password_hash, role, created_at`,
		username, hash,
	).Scan(&acct.ID, &acct.Username, &acct.PasswordHash, &acct.Role, &acct.CreatedAt)
	if err != nil {
		if isDuplicateKeyError(err) {
			return Account{}, ErrAccountExists
		}
		return Account{}, fmt.Errorf("inserting account: %w", err)
	}

	return acct, nil
}

// Authenticate verifies credentials and returns the matching account.
//
// Precondition: username and password must be non-empty.
// Postcondition: Returns the Account if credentials are valid,
// ErrAccountNotFound if the username doesn't exist,
// or ErrInvalidCredentials if the password is wrong.
func (r *AccountRepository) Authenticate(ctx context.Context, username, password string) (Account, error) {
	var acct Account
	err := r.db.QueryRow(ctx,
		`SELECT id, username, password_hash, role, created_at
		 FROM accounts WHERE username = $1`,
		username,
	).Scan(&acct.ID, &acct.Username, &acct.PasswordHash, &acct.Role, &acct.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Account{}, ErrAccountNotFound
		}
		return Account{}, fmt.Errorf("querying account: %w", err)
	}

	if !CheckPassword(password, acct.PasswordHash) {
		return Account{}, ErrInvalidCredentials
	}

	return acct, nil
}

// GetByUsername retrieves an account by username.
//
// Precondition: username must be non-empty.
// Postcondition: Returns the Account or ErrAccountNotFound.
func (r *AccountRepository) GetByUsername(ctx context.Context, username string) (Account, error) {
	var acct Account
	err := r.db.QueryRow(ctx,
		`SELECT id, username, password_hash, role, created_at
		 FROM accounts WHERE username = $1`,
		username,
	).Scan(&acct.ID, &acct.Username, &acct.PasswordHash, &acct.Role, &acct.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Account{}, ErrAccountNotFound
		}
		return Account{}, fmt.Errorf("querying account: %w", err)
	}
	return acct, nil
}

// SetRole updates the role for the given account.
//
// Precondition: role must be a valid role string (use ValidRole to check).
// Postcondition: The account's role is updated, or ErrInvalidRole / ErrAccountNotFound is returned.
func (r *AccountRepository) SetRole(ctx context.Context, accountID int64, role string) error {
	if !ValidRole(role) {
		return ErrInvalidRole
	}

	tag, err := r.db.Exec(ctx,
		`UPDATE accounts SET role = $1 WHERE id = $2`,
		role, accountID,
	)
	if err != nil {
		return fmt.Errorf("updating role: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrAccountNotFound
	}
	return nil
}

// HashPassword creates a bcrypt hash of the given password.
//
// Precondition: password must be non-empty.
// Postcondition: Returns a bcrypt hash string.
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// CheckPassword compares a plaintext password against a bcrypt hash.
//
// Postcondition: Returns true if password matches the hash.
func CheckPassword(password, hash string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// isDuplicateKeyError checks if a pgx error is a unique constraint violation.
func isDuplicateKeyError(err error) bool {
	// pgx wraps PostgreSQL errors; check for SQLSTATE 23505 (unique_violation)
	var pgErr interface{ SQLState() string }
	if errors.As(err, &pgErr) {
		return pgErr.SQLState() == "23505"
	}
	return false
}
