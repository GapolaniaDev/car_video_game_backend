// Package cars exposes the global car catalog (read-only).
package cars

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

// Car is the JSON-friendly representation of the cars table.
type Car struct {
	CarID      uuid.UUID       `json:"carId"`
	Name       string          `json:"name"`
	BaseStats  json.RawMessage `json:"baseStats"`
	CreatedAt  time.Time       `json:"createdAt"`
}

// Repo holds the read queries.
type Repo struct {
	pool *pgxpool.Pool
}

func NewRepo(pool *pgxpool.Pool) *Repo { return &Repo{pool: pool} }

// List returns every car in catalog order.
func (r *Repo) List(ctx context.Context) ([]Car, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, name, base_stats, created_at FROM cars ORDER BY name
	`)
	if err != nil {
		return nil, fmt.Errorf("list cars: %w", err)
	}
	defer rows.Close()

	var out []Car
	for rows.Next() {
		var c Car
		if err := rows.Scan(&c.CarID, &c.Name, &c.BaseStats, &c.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// GetByID returns a single car or pgx.ErrNoRows.
func (r *Repo) GetByID(ctx context.Context, id uuid.UUID) (*Car, error) {
	var c Car
	err := r.pool.QueryRow(ctx, `
		SELECT id, name, base_stats, created_at FROM cars WHERE id = $1
	`, id).Scan(&c.CarID, &c.Name, &c.BaseStats, &c.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, pgx.ErrNoRows
		}
		return nil, err
	}
	return &c, nil
}