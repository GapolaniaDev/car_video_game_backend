package auth

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repo is the Postgres-backed persistence layer for users + players.
// Constructed once and shared by the auth service.
type Repo struct {
	pool *pgxpool.Pool
}

// NewRepo wraps an existing pgxpool.
func NewRepo(pool *pgxpool.Pool) *Repo { return &Repo{pool: pool} }

// CreateUserWithPlayer inserts a (user, player) pair inside a single
// transaction. Returns ErrEmailTaken when the email already exists.
// Returns the new user and player ids.
func (r *Repo) CreateUserWithPlayer(ctx context.Context, email, passwordHash, displayName string) (uuid.UUID, uuid.UUID, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return uuid.Nil, uuid.Nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	var userID uuid.UUID
	err = tx.QueryRow(ctx, `
		INSERT INTO users (email, password_hash, is_guest)
		VALUES ($1, $2, FALSE)
		RETURNING id
	`, email, passwordHash).Scan(&userID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" { // unique_violation
			return uuid.Nil, uuid.Nil, ErrEmailTaken
		}
		return uuid.Nil, uuid.Nil, fmt.Errorf("insert user: %w", err)
	}

	var playerID uuid.UUID
	err = tx.QueryRow(ctx, `
		INSERT INTO players (user_id, display_name)
		VALUES ($1, $2)
		RETURNING id
	`, userID, displayName).Scan(&playerID)
	if err != nil {
		return uuid.Nil, uuid.Nil, fmt.Errorf("insert player: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return uuid.Nil, uuid.Nil, fmt.Errorf("commit: %w", err)
	}
	return userID, playerID, nil
}

// CreateGuest inserts an anonymous (is_guest=true) user and a player
// with a generated display name. Returns the new ids.
func (r *Repo) CreateGuest(ctx context.Context, displayName string) (uuid.UUID, uuid.UUID, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return uuid.Nil, uuid.Nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	var userID uuid.UUID
	err = tx.QueryRow(ctx, `
		INSERT INTO users (is_guest)
		VALUES (TRUE)
		RETURNING id
	`).Scan(&userID)
	if err != nil {
		return uuid.Nil, uuid.Nil, fmt.Errorf("insert guest user: %w", err)
	}

	var playerID uuid.UUID
	err = tx.QueryRow(ctx, `
		INSERT INTO players (user_id, display_name)
		VALUES ($1, $2)
		RETURNING id
	`, userID, displayName).Scan(&playerID)
	if err != nil {
		return uuid.Nil, uuid.Nil, fmt.Errorf("insert guest player: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return uuid.Nil, uuid.Nil, fmt.Errorf("commit: %w", err)
	}
	return userID, playerID, nil
}

// LookupUserByEmail returns the (user_id, password_hash) pair for an
// email, or ErrInvalidCredentials when the row doesn't exist or the
// account is a guest. The single sentinel prevents enumeration.
func (r *Repo) LookupUserByEmail(ctx context.Context, email string) (uuid.UUID, string, error) {
	var userID uuid.UUID
	var hash *string
	var isGuest bool
	err := r.pool.QueryRow(ctx, `
		SELECT id, password_hash, is_guest
		FROM users
		WHERE email = $1
	`, email).Scan(&userID, &hash, &isGuest)
	if errors.Is(err, pgx.ErrNoRows) || isGuest || hash == nil {
		return uuid.Nil, "", ErrInvalidCredentials
	}
	if err != nil {
		return uuid.Nil, "", fmt.Errorf("lookup user: %w", err)
	}
	return userID, *hash, nil
}

// PlayerByUserID returns the player id associated with a user.
func (r *Repo) PlayerByUserID(ctx context.Context, userID uuid.UUID) (uuid.UUID, error) {
	var playerID uuid.UUID
	err := r.pool.QueryRow(ctx, `
		SELECT id FROM players WHERE user_id = $1
	`, userID).Scan(&playerID)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, ErrPlayerNotFound
	}
	if err != nil {
		return uuid.Nil, fmt.Errorf("lookup player: %w", err)
	}
	return playerID, nil
}

// PlayerByID returns the player id (sanity check) — exists mainly so
// callers can confirm a token's player id still maps to a row.
func (r *Repo) PlayerByID(ctx context.Context, playerID uuid.UUID) error {
	var n int
	err := r.pool.QueryRow(ctx, `SELECT 1 FROM players WHERE id = $1`, playerID).Scan(&n)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrPlayerNotFound
	}
	return err
}

// TouchLastLogin updates the last_login_at column for a user.
func (r *Repo) TouchLastLogin(ctx context.Context, userID uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `UPDATE users SET last_login_at = NOW() WHERE id = $1`, userID)
	return err
}