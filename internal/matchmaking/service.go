package matchmaking

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/gustavo/racing-game-backend/internal/auth"
)

// MaxPlayersPerMatch is the hard cap on a single race in the MVP.
const MaxPlayersPerMatch = 4

// MatchInfo is the JSON-friendly shape returned to clients.
type MatchInfo struct {
	MatchID         uuid.UUID `json:"matchId"`
	GameServerHost  string    `json:"gameServerHost"`
	GameServerPort  int       `json:"gameServerPort"`
	GameToken       string    `json:"gameToken"`
	ExpiresAt       time.Time `json:"expiresAt"`
}

// Service composes repo + token signer for /matchmaking/join.
type Service struct {
	repo        *Repo
	gameHost    string
	gamePort    int
	tokenSecret string
	log         *slog.Logger
}

// NewService wires a real service.
func NewService(repo *Repo, gameHost string, gamePort int, tokenSecret string, log *slog.Logger) *Service {
	if log == nil {
		log = slog.Default()
	}
	return &Service{
		repo:        repo,
		gameHost:    gameHost,
		gamePort:    gamePort,
		tokenSecret: tokenSecret,
		log:         log,
	}
}

// Join either joins the player to a pending match or mints a new one.
// The MVP "matchmaking" is immediate and stateless (no skill rating),
// so the only stateful part is the player-cap check.
func (s *Service) Join(ctx context.Context, playerID uuid.UUID) (*MatchInfo, error) {
	if s.gameHost == "" || s.gamePort == 0 {
		return nil, ErrNoGameServer
	}

	matchID, err := s.findOrCreateMatch(ctx, playerID)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	token, exp, err := SignGameToken(s.tokenSecret, matchID, playerID, now)
	if err != nil {
		return nil, fmt.Errorf("matchmaking: sign: %w", err)
	}
	if err := s.repo.CreateAssignment(ctx, matchID, playerID, token, exp); err != nil {
		return nil, err
	}

	s.log.Info("matchmaking.join",
		slog.String("player_id", playerID.String()),
		slog.String("match_id", matchID.String()),
		slog.Time("expires_at", exp),
	)
	return &MatchInfo{
		MatchID:        matchID,
		GameServerHost: s.gameHost,
		GameServerPort: s.gamePort,
		GameToken:      token,
		ExpiresAt:      exp,
	}, nil
}

// findOrCreateMatch returns the first pending match (capacity left)
// or creates a brand-new one. Allocation logic for the MVP: the
// newest unconsumed assignment with a slot wins; ties broken by
// created_at DESC for stability.
func (s *Service) findOrCreateMatch(ctx context.Context, playerID uuid.UUID) (uuid.UUID, error) {
	r := s.repo
	now := time.Now()

	rows, err := r.pool.Query(ctx, `
		SELECT match_id FROM matchmaking_assignments
		 WHERE consumed = FALSE AND expires_at > $1
		 GROUP BY match_id
		 HAVING COUNT(*) < $2
		 ORDER BY MAX(created_at) DESC
		 LIMIT 1
	`, now, MaxPlayersPerMatch)
	if err != nil {
		return uuid.Nil, fmt.Errorf("matchmaking: pick: %w", err)
	}
	defer rows.Close()
	if rows.Next() {
		var matchID uuid.UUID
		if err := rows.Scan(&matchID); err != nil {
			return uuid.Nil, err
		}
		// Re-check capacity atomically — cheap double-check.
		n, err := r.CountForMatch(ctx, matchID, now)
		if err != nil {
			return uuid.Nil, err
		}
		if n < MaxPlayersPerMatch {
			return matchID, nil
		}
	}
	return uuid.New(), nil
}

// auth.PlayerIDFromContext is re-exported so handlers live next to the
// service they call.
var _ = auth.PlayerIDFromContext