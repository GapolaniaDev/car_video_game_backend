package matchmaking

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repo is the persistence layer for matchmaking_assignments.
type Repo struct {
	pool        *pgxpool.Pool
	tokenSecret string // set via WithSecret for the Game Server path
}

func NewRepo(pool *pgxpool.Pool) *Repo { return &Repo{pool: pool} }

// CreateAssignment inserts a new row for the (match, player) tuple.
// Tokens are unique per (player, match) but the unique constraint lives
// in our application code (not the DB) for MVP simplicity.
func (r *Repo) CreateAssignment(ctx context.Context, matchID, playerID uuid.UUID, token string, exp time.Time) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO matchmaking_assignments (match_id, player_id, game_token, expires_at)
		VALUES ($1, $2, $3, $4)
	`, matchID, playerID, token, exp)
	if err != nil {
		return fmt.Errorf("matchmaking: insert assignment: %w", err)
	}
	return nil
}

// AssignmentIsFresh implements TokenStore.
func (r *Repo) AssignmentIsFresh(matchID, playerID uuid.UUID, rawToken string, now time.Time) (bool, error) {
	var consumed bool
	var exp time.Time
	err := r.pool.QueryRow(context.Background(), `
		SELECT consumed, expires_at FROM matchmaking_assignments
		 WHERE match_id = $1 AND player_id = $2 AND game_token = $3
	`, matchID, playerID, rawToken).Scan(&consumed, &exp)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("matchmaking: lookup assignment: %w", err)
	}
	if consumed {
		return false, nil
	}
	if now.After(exp) {
		return false, nil
	}
	if !VerifyHMAC(r.tokenSecret, rawToken) {
		return false, nil
	}
	return true, nil
}

// SetTokenSecret stores the secret used for HMAC checks on the Repo.
// Called by both API and Game Server constructors.
func (r *Repo) SetTokenSecret(secret string) { r.tokenSecret = secret }

// MarkConsumed implements TokenStore.
func (r *Repo) MarkConsumed(matchID, playerID uuid.UUID, rawToken string) error {
	_, err := r.pool.Exec(context.Background(), `
		UPDATE matchmaking_assignments SET consumed = TRUE
		 WHERE match_id = $1 AND player_id = $2 AND game_token = $3
	`, matchID, playerID, rawToken)
	if err != nil {
		return fmt.Errorf("matchmaking: mark consumed: %w", err)
	}
	return nil
}

// VerifyAndConsume satisfies networking.Verifier. It re-checks the
// HMAC, expiry, existence, and flips consumed=true in one shot.
func (r *Repo) VerifyAndConsume(ctx context.Context, matchID, playerID uuid.UUID, rawToken string) error {
	var consumed bool
	var exp time.Time
	err := r.pool.QueryRow(ctx, `
		SELECT consumed, expires_at FROM matchmaking_assignments
		 WHERE match_id = $1 AND player_id = $2 AND game_token = $3
	`, matchID, playerID, rawToken).Scan(&consumed, &exp)
	if errors.Is(err, pgx.ErrNoRows) {
		// Also try a pure HMAC + exp check so the Game Server can
		// reject obvious forgeries even when the row is gone.
		if _, verr := VerifyGameToken(r.tokenSecret, rawToken, nil, time.Now()); verr != nil {
			if verr == ErrTokenExpired {
				return ErrTokenExpired
			}
			return ErrTokenInvalid
		}
		return ErrTokenInvalid
	}
	if err != nil {
		return fmt.Errorf("matchmaking: verify: %w", err)
	}
	if consumed {
		return ErrTokenConsumed
	}
	if time.Now().After(exp) {
		return ErrTokenExpired
	}
	if !VerifyHMAC(r.tokenSecret, rawToken) {
		return ErrTokenInvalid
	}
	if _, err := r.pool.Exec(ctx, `
		UPDATE matchmaking_assignments SET consumed = TRUE
		 WHERE match_id = $1 AND player_id = $2 AND game_token = $3 AND consumed = FALSE
	`, matchID, playerID, rawToken); err != nil {
		return fmt.Errorf("matchmaking: consume: %w", err)
	}
	return nil
}

// WithSecret supplies the HMAC secret used by VerifyAndConsume. The
// Game Server constructs the Repo with a no-Auth method and binds the
// secret via this setter before the dispatcher runs.
func (r *Repo) WithSecret(secret string) *Repo {
	r.tokenSecret = secret
	return r
}

// CountForMatch returns how many active (unconsumed, unexpired)
// assignments exist for a match — used to enforce the max-player cap.
func (r *Repo) CountForMatch(ctx context.Context, matchID uuid.UUID, now time.Time) (int, error) {
	var n int
	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM matchmaking_assignments
		 WHERE match_id = $1
		   AND consumed = FALSE
		   AND expires_at > $2
	`, matchID, now).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("matchmaking: count: %w", err)
	}
	return n, nil
}
