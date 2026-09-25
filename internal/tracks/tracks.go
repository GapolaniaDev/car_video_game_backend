// Package tracks exposes the global track catalog (read-only).
package tracks

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Track is the JSON-friendly representation of the tracks table.
type Track struct {
	TrackID   uuid.UUID       `json:"trackId"`
	Name      string          `json:"name"`
	Layout    json.RawMessage `json:"layout"`
	CreatedAt time.Time       `json:"createdAt"`
}

// Repo holds read queries.
type Repo struct {
	pool *pgxpool.Pool
}

func NewRepo(pool *pgxpool.Pool) *Repo { return &Repo{pool: pool} }

// List returns every track in catalog order.
func (r *Repo) List(ctx context.Context) ([]Track, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, name, layout, created_at FROM tracks ORDER BY name
	`)
	if err != nil {
		return nil, fmt.Errorf("list tracks: %w", err)
	}
	defer rows.Close()

	var out []Track
	for rows.Next() {
		var t Track
		if err := rows.Scan(&t.TrackID, &t.Name, &t.Layout, &t.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// GetByID returns a single track.
func (r *Repo) GetByID(ctx context.Context, id uuid.UUID) (*Track, error) {
	var t Track
	err := r.pool.QueryRow(ctx, `
		SELECT id, name, layout, created_at FROM tracks WHERE id = $1
	`, id).Scan(&t.TrackID, &t.Name, &t.Layout, &t.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, pgx.ErrNoRows
		}
		return nil, err
	}
	return &t, nil
}