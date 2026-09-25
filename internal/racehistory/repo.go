// Package racehistory serves player race history and individual
// race detail through the REST API.
package racehistory

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PlayerRace is a one-line summary of a single race for a player.
type PlayerRace struct {
	RaceID      uuid.UUID  `json:"raceId"`
	TrackID     uuid.UUID  `json:"trackId"`
	TrackName   string     `json:"trackName"`
	FinishedAt  *time.Time `json:"finishedAt,omitempty"`
	Position    int        `json:"position"`
	TotalTimeMs int        `json:"totalTimeMs"`
	BestLapMs   int        `json:"bestLapMs"`
}

// PlayerHistoryPage is the response for /players/me/races.
type PlayerHistoryPage struct {
	Races      []PlayerRace `json:"races"`
	NextCursor *time.Time   `json:"nextCursor,omitempty"`
}

// RaceDetail is the response for /races/{id}.
type RaceDetail struct {
	RaceID     uuid.UUID   `json:"raceId"`
	TrackID    uuid.UUID   `json:"trackId"`
	TrackName  string      `json:"trackName"`
	Status     string      `json:"status"`
	StartedAt  *time.Time  `json:"startedAt,omitempty"`
	FinishedAt *time.Time  `json:"finishedAt,omitempty"`
	Results    []ResultRow `json:"results"`
}

// ResultRow is one player's slice of a RaceDetail.
type ResultRow struct {
	PlayerID    uuid.UUID `json:"playerId"`
	DisplayName string    `json:"displayName"`
	Position    int       `json:"position"`
	TotalTimeMs int       `json:"totalTimeMs"`
	BestLapMs   int       `json:"bestLapMs"`
}

// Repo wraps the queries needed by these endpoints.
type Repo struct {
	pool *pgxpool.Pool
}

func NewRepo(pool *pgxpool.Pool) *Repo { return &Repo{pool: pool} }

// ListByPlayer returns `limit` races finished <= cursor (or all) for
// the player. Page size is clamped to [1, 100].
func (r *Repo) ListByPlayer(ctx context.Context, playerID uuid.UUID, cursor *time.Time, limit int) (*PlayerHistoryPage, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	var rows pgx.Rows
	var err error
	if cursor == nil {
		rows, err = r.pool.Query(ctx, `
			SELECT rr.race_id, r.track_id, t.name, r.finished_at,
			       rr.position, rr.total_time_ms, COALESCE(rr.best_lap_ms, 0)
			  FROM race_results rr
			  JOIN races r   ON r.id = rr.race_id
			  JOIN tracks t  ON t.id = r.track_id
			 WHERE rr.player_id = $1
			   AND r.finished_at IS NOT NULL
			 ORDER BY r.finished_at DESC
			 LIMIT $2
		`, playerID, limit)
	} else {
		rows, err = r.pool.Query(ctx, `
			SELECT rr.race_id, r.track_id, t.name, r.finished_at,
			       rr.position, rr.total_time_ms, COALESCE(rr.best_lap_ms, 0)
			  FROM race_results rr
			  JOIN races r   ON r.id = rr.race_id
			  JOIN tracks t  ON t.id = r.track_id
			 WHERE rr.player_id = $1
			   AND r.finished_at < $2
			 ORDER BY r.finished_at DESC
			 LIMIT $3
		`, playerID, *cursor, limit)
	}
	if err != nil {
		return nil, fmt.Errorf("racehistory: list: %w", err)
	}
	defer rows.Close()

	out := &PlayerHistoryPage{Races: []PlayerRace{}}
	for rows.Next() {
		var pr PlayerRace
		var done *time.Time
		if err := rows.Scan(&pr.RaceID, &pr.TrackID, &pr.TrackName, &done, &pr.Position, &pr.TotalTimeMs, &pr.BestLapMs); err != nil {
			return nil, err
		}
		pr.FinishedAt = done
		out.Races = append(out.Races, pr)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(out.Races) == limit {
		last := out.Races[len(out.Races)-1]
		if last.FinishedAt != nil {
			c := *last.FinishedAt
			out.NextCursor = &c
		}
	}
	return out, nil
}

// GetRace returns the full RaceDetail for a race. Returns
// pgx.ErrNoRows when the race is unknown.
func (r *Repo) GetRace(ctx context.Context, raceID uuid.UUID) (*RaceDetail, error) {
	var d RaceDetail
	var started, finished *time.Time
	err := r.pool.QueryRow(ctx, `
		SELECT r.id, r.track_id, t.name, r.status, r.started_at, r.finished_at
		  FROM races r
		  JOIN tracks t ON t.id = r.track_id
		 WHERE r.id = $1
	`, raceID).Scan(&d.RaceID, &d.TrackID, &d.TrackName, &d.Status, &started, &finished)
	if err != nil {
		return nil, err
	}
	d.StartedAt = started
	d.FinishedAt = finished

	rows, err := r.pool.Query(ctx, `
		SELECT rr.player_id, p.display_name, rr.position, rr.total_time_ms, COALESCE(rr.best_lap_ms, 0)
		  FROM race_results rr
		  JOIN players p ON p.id = rr.player_id
		 WHERE rr.race_id = $1
		 ORDER BY rr.position ASC
	`, raceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var r ResultRow
		if err := rows.Scan(&r.PlayerID, &r.DisplayName, &r.Position, &r.TotalTimeMs, &r.BestLapMs); err != nil {
			return nil, err
		}
		d.Results = append(d.Results, r)
	}
	return &d, rows.Err()
}

// ErrNotFound is a sentinel matching pgx.ErrNoRows.
var ErrNotFound = errors.New("racehistory: not found")

// TranslateError maps pgx.ErrNoRows into our own ErrNotFound.
func TranslateError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}
