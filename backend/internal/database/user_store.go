package database

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type UserRecord struct {
	Username         string
	Email            string
	PasswordHash     []byte
	Role             string
	Verified         bool
	VerificationCode string
	FailedAttempts   int
	LockedUntil      time.Time
}

type UserStore struct {
	pool *pgxpool.Pool
}

func NewUserStore(pool *pgxpool.Pool) *UserStore {
	if pool == nil {
		return nil
	}
	return &UserStore{pool: pool}
}

func (s *UserStore) EnsureSchema() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	const schema = `
CREATE TABLE IF NOT EXISTS users (
    username TEXT PRIMARY KEY,
    email TEXT NOT NULL,
    password_hash BYTEA NOT NULL,
    role TEXT NOT NULL,
    verified BOOLEAN NOT NULL DEFAULT FALSE,
    verification_code TEXT NOT NULL DEFAULT '',
    failed_attempts INTEGER NOT NULL DEFAULT 0,
    locked_until TIMESTAMP NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

ALTER TABLE users ADD COLUMN IF NOT EXISTS email TEXT NOT NULL DEFAULT '';
ALTER TABLE users ADD COLUMN IF NOT EXISTS password_hash BYTEA NOT NULL DEFAULT '\\x';
ALTER TABLE users ADD COLUMN IF NOT EXISTS role TEXT NOT NULL DEFAULT 'developer';
ALTER TABLE users ADD COLUMN IF NOT EXISTS verified BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE users ADD COLUMN IF NOT EXISTS verification_code TEXT NOT NULL DEFAULT '';
ALTER TABLE users ADD COLUMN IF NOT EXISTS failed_attempts INTEGER NOT NULL DEFAULT 0;
ALTER TABLE users ADD COLUMN IF NOT EXISTS locked_until TIMESTAMP NULL;
ALTER TABLE users ADD COLUMN IF NOT EXISTS created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP;

CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email_unique_lower ON users (lower(email));
`

	_, err := s.pool.Exec(ctx, schema)
	return err
}

func (s *UserStore) GetByUsername(username string) (UserRecord, bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rec := UserRecord{}
	var lockedUntil *time.Time
	err := s.pool.QueryRow(ctx, `
SELECT username, email, password_hash, role, verified, verification_code, failed_attempts, locked_until
FROM users
WHERE username = $1`, username).Scan(
		&rec.Username,
		&rec.Email,
		&rec.PasswordHash,
		&rec.Role,
		&rec.Verified,
		&rec.VerificationCode,
		&rec.FailedAttempts,
		&lockedUntil,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return UserRecord{}, false, nil
		}
		return UserRecord{}, false, err
	}
	if lockedUntil != nil {
		rec.LockedUntil = *lockedUntil
	}
	return rec, true, nil
}

func (s *UserStore) GetByUsernameOrEmail(username, email string) (UserRecord, bool, error) {
	if username != "" {
		rec, found, err := s.GetByUsername(username)
		if err != nil || found {
			return rec, found, err
		}
	}
	if email == "" {
		return UserRecord{}, false, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rec := UserRecord{}
	var lockedUntil *time.Time
	err := s.pool.QueryRow(ctx, `
SELECT username, email, password_hash, role, verified, verification_code, failed_attempts, locked_until
FROM users
WHERE lower(email) = lower($1)
LIMIT 1`, email).Scan(
		&rec.Username,
		&rec.Email,
		&rec.PasswordHash,
		&rec.Role,
		&rec.Verified,
		&rec.VerificationCode,
		&rec.FailedAttempts,
		&lockedUntil,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return UserRecord{}, false, nil
		}
		return UserRecord{}, false, err
	}
	if lockedUntil != nil {
		rec.LockedUntil = *lockedUntil
	}
	return rec, true, nil
}

func (s *UserStore) EmailExists(email string) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var marker int
	err := s.pool.QueryRow(ctx, `SELECT 1 FROM users WHERE lower(email) = lower($1) LIMIT 1`, email).Scan(&marker)
	if err != nil {
		if err == pgx.ErrNoRows {
			return false, nil
		}
		return false, err
	}
	return marker == 1, nil
}

func (s *UserStore) CreateUser(rec UserRecord) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var lockedUntil *time.Time
	if !rec.LockedUntil.IsZero() {
		lockedUntil = &rec.LockedUntil
	}

	_, err := s.pool.Exec(ctx, `
INSERT INTO users (username, email, password_hash, role, verified, verification_code, failed_attempts, locked_until)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		rec.Username,
		rec.Email,
		rec.PasswordHash,
		rec.Role,
		rec.Verified,
		rec.VerificationCode,
		rec.FailedAttempts,
		lockedUntil,
	)
	return err
}

func (s *UserStore) UpdateUser(rec UserRecord) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var lockedUntil *time.Time
	if !rec.LockedUntil.IsZero() {
		lockedUntil = &rec.LockedUntil
	}

	_, err := s.pool.Exec(ctx, `
UPDATE users
SET email = $2,
    password_hash = $3,
    role = $4,
    verified = $5,
    verification_code = $6,
    failed_attempts = $7,
    locked_until = $8
WHERE username = $1`,
		rec.Username,
		rec.Email,
		rec.PasswordHash,
		rec.Role,
		rec.Verified,
		rec.VerificationCode,
		rec.FailedAttempts,
		lockedUntil,
	)
	return err
}
