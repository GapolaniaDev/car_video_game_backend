// Package leaderboard serves the per-track "best lap" ranking.
package leaderboard

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Entry is one row in a leaderboard response.
type Entry struct {
	Rank        int        `json:"rank"`
	PlayerID    uuid.UUID  `json:"playerId"`
	DisplayName string     `json:"displayName"`
	BestLapMs   int        `json:"bestLapMs"`
	AchievedAt  *time.Time `json:"achievedAt,omitempty"`
}

// Track is the catalog metadata returned alongside entries.
type Track struct {
	TrackID uuid.UUID `json:"trackId"`
	Name    string    `json:"trackName"`
}

// Repo holds the leaderboard queries.
type Repo struct {
	pool *pgxpool.Pool
}

func NewRepo(pool *pgxpool.Pool) *Repo { return &Repo{pool: pool} }

// LoadTrack returns the catalog entry for a track id.
func (r *Repo) LoadTrack(ctx context.Context, id uuid.UUID) (*Track, error) {
	var t Track
	err := r.pool.QueryRow(ctx, `SELECT id, name FROM tracks WHERE id = $1`, id).Scan(&t.TrackID, &t.Name)
	if err != nil {
		return nil, fmt.Errorf("leaderboard: load track: %w", err)
	}
	return &t, nil
}

// BestLapsByTrack returns the top-N best laps on `trackID` across all
// players. Empty `n` returns the default (10).
func (r *Repo) BestLapsByTrack(ctx context.Context, trackID uuid.UUID, n int) ([]Entry, error) {
	if n <= 0 {
		n = 10
	}
	rows, err := r.pool.Query(ctx, `
		SELECT
			rr.player_id,
			p.display_name,
			MIN(rr.best_lap_ms) AS best_lap_ms,
			MIN(rr.created_at)  AS achieved_at
		FROM race_results rr
		JOIN players p ON p.id = rr.player_id
		JOIN races   r ON r.id = rr.race_id
		WHERE r.track_id = $1
		  AND rr.best_lap_ms IS NOT NULL
		  AND rr.best_lap_ms > 0
		GROUP BY rr.player_id, p.display_name
		ORDER BY best_lap_ms ASC
		LIMIT $2
	`, trackID, n)
	if err != nil {
		return nil, fmt.Errorf("leaderboard: best laps: %w", err)
	}
	defer rows.Close()

	var out []Entry
	rank := 1
	for rows.Next() {
		var e Entry
		var ach *time.Time
		if err := rows.Scan(&e.PlayerID, &e.DisplayName, &e.BestLapMs, &ach); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		e.AchievedAt = ach
		e.Rank = rank
		rank++
		out = append(out, e)
	}
	return out, rows.Err()
}

// PlayerBestPerTrack returns the player's personal best lap on each
// track they have raced on.
func (r *Repo) PlayerBestPerTrack(ctx context.Context, playerID uuid.UUID) (map[uuid.UUID]int, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT r.track_id, MIN(rr.best_lap_ms)
		  FROM race_results rr
		  JOIN races r ON r.id = rr.race_id
		 WHERE rr.player_id = $1
		   AND rr.best_lap_ms IS NOT NULL
		   AND rr.best_lap_ms > 0
		 GROUP BY r.track_id
	`, playerID)
	if err != nil {
		return nil, fmt.Errorf("leaderboard: player bests: %w", err)
	}
	defer rows.Close()

	out := map[uuid.UUID]int{}
	for rows.Next() {
		var track uuid.UUID
		var lap int
		if err := rows.Scan(&track, &lap); err != nil {
			return nil, err
		}
		out[track] = lap
	}
	return out, rows.Err()
}
