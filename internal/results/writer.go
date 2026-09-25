// Package results persists the final outcome of a race to Postgres
// so the REST API can serve history and leaderboards.
//
// The Writer is invoked by the Game Server's tick loop when a race
// transitions to StatusFinished. It is idempotent on retry thanks to
// the unique (race_id, player_id) constraint on race_results.
package results

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Entry is the per-player result of one race.
type Entry struct {
	PlayerID    uuid.UUID
	Position    int
	TotalTimeMs int
	BestLapMs   int
}

// WriteRequest is what the race loop hands to the writer.
type WriteRequest struct {
	RaceID     uuid.UUID
	TrackID    uuid.UUID
	StartedAt  time.Time
	FinishedAt time.Time
	Results    []Entry
}

// Writer is the persistence layer.
type Writer struct {
	pool *pgxpool.Pool
	log  *slog.Logger
}

// NewWriter constructs a writer.
func NewWriter(pool *pgxpool.Pool, log *slog.Logger) *Writer {
	if log == nil {
		log = slog.Default()
	}
	return &Writer{pool: pool, log: log}
}

// PersistRaceFinish inserts the race row (idempotent if missing),
// updates race_players + race_results, and flips the race to
// 'finished'. It returns immediately on the first error but the
// caller is expected to retry once before giving up.
func (w *Writer) PersistRaceFinish(ctx context.Context, req WriteRequest) error {
	if req.RaceID == uuid.Nil {
		return errors.New("results: race_id required")
	}
	if req.TrackID == uuid.Nil {
		return errors.New("results: track_id required")
	}
	if len(req.Results) == 0 {
		return errors.New("results: empty results")
	}

	tx, err := w.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Upsert the race row so the FK constraints succeed. The Game
	// Server only knows races in memory; this row anchors it in the
	// DB on first finish.
	startedAt := req.StartedAt
	if startedAt.IsZero() {
		startedAt = time.Now()
	}
	finishedAt := req.FinishedAt
	if finishedAt.IsZero() {
		finishedAt = time.Now()
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO races (id, track_id, status, started_at, finished_at)
		VALUES ($1, $2, 'finished', $3, $4)
		ON CONFLICT (id) DO UPDATE SET
			status = 'finished',
			finished_at = EXCLUDED.finished_at
	`, req.RaceID, req.TrackID, startedAt, finishedAt); err != nil {
		return fmt.Errorf("upsert races: %w", err)
	}

	for _, r := range req.Results {
		if r.PlayerID == uuid.Nil {
			continue
		}
		// Upsert race_players so FK constraints succeed.
		if _, err := tx.Exec(ctx, `
			INSERT INTO race_players (race_id, player_id, finish_position, finished_at)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (race_id, player_id) DO UPDATE SET
				finish_position = EXCLUDED.finish_position,
				finished_at     = EXCLUDED.finished_at
		`, req.RaceID, r.PlayerID, r.Position, finishedAt); err != nil {
			return fmt.Errorf("upsert race_players (%s): %w", r.PlayerID, err)
		}

		// Idempotent insert of the result row.
		if _, err := tx.Exec(ctx, `
			INSERT INTO race_results (race_id, player_id, position, total_time_ms, best_lap_ms)
			VALUES ($1, $2, $3, $4, NULLIF($5, 0))
			ON CONFLICT (race_id, player_id) DO UPDATE SET
				position      = EXCLUDED.position,
				total_time_ms = EXCLUDED.total_time_ms,
				best_lap_ms   = EXCLUDED.best_lap_ms
		`, req.RaceID, r.PlayerID, r.Position, r.TotalTimeMs, r.BestLapMs); err != nil {
			return fmt.Errorf("upsert race_results (%s): %w", r.PlayerID, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	w.log.Info("results.persist.ok",
		slog.String("race_id", req.RaceID.String()),
		slog.Int("result_count", len(req.Results)),
	)
	return nil
}

// PersistWithRetry invokes PersistRaceFinish once; on error it sleeps
// 1s and retries once. The MVP "DLQ" is just logging at that point —
// in-memory state is not retained past a process restart.
func (w *Writer) PersistWithRetry(ctx context.Context, req WriteRequest) {
	if err := w.PersistRaceFinish(ctx, req); err == nil {
		return
	} else {
		w.log.Warn("results.persist.retry",
			slog.String("race_id", req.RaceID.String()),
			slog.String("err", err.Error()),
		)
	}
	select {
	case <-ctx.Done():
		return
	case <-time.After(1 * time.Second):
	}
	if err := w.PersistRaceFinish(ctx, req); err != nil {
		w.log.Error("results.persist.failed",
			slog.String("race_id", req.RaceID.String()),
			slog.String("err", err.Error()),
		)
	}
}
