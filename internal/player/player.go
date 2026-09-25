// Package player exposes the authenticated player's own profile
// (GET /players/me). Other player actions live in future milestones.
package player

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gustavo/racing-game-backend/internal/auth"
)

// Profile is the JSON-friendly shape returned by handlers.
type Profile struct {
	PlayerID    uuid.UUID  `json:"playerId"`
	UserID      uuid.UUID  `json:"userId"`
	DisplayName string     `json:"displayName"`
	AvatarURL   *string    `json:"avatarUrl,omitempty"`
	IsGuest     bool       `json:"isGuest"`
	CreatedAt   time.Time  `json:"createdAt"`
	LastLoginAt *time.Time `json:"lastLoginAt,omitempty"`
}

// Repo is the persistence layer.
type Repo struct {
	pool *pgxpool.Pool
}

func NewRepo(pool *pgxpool.Pool) *Repo { return &Repo{pool: pool} }

// GetByID fetches a profile for the given player id.
func (r *Repo) GetByID(ctx context.Context, playerID uuid.UUID) (*Profile, error) {
	var p Profile
	var lastLogin *time.Time
	err := r.pool.QueryRow(ctx, `
		SELECT p.id, p.user_id, p.display_name, p.avatar_url,
		       u.is_guest, p.created_at, u.last_login_at
		FROM players p
		JOIN users u ON u.id = p.user_id
		WHERE p.id = $1
	`, playerID).Scan(
		&p.PlayerID, &p.UserID, &p.DisplayName, &p.AvatarURL,
		&p.IsGuest, &p.CreatedAt, &lastLogin,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, auth.ErrPlayerNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get player: %w", err)
	}
	p.LastLoginAt = lastLogin
	return &p, nil
}