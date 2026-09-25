// Package garage exposes a player's owned cars.
package garage

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Entry is a row of the player's garage.
type Entry struct {
	PlayerCarID    uuid.UUID       `json:"playerCarId"`
	CarID          uuid.UUID       `json:"carId"`
	Name           string          `json:"name"`
	AcquiredAt     time.Time       `json:"acquiredAt"`
	Customizations json.RawMessage `json:"customizations"`
}

// Repo holds garage queries.
type Repo struct {
	pool *pgxpool.Pool
}

func NewRepo(pool *pgxpool.Pool) *Repo { return &Repo{pool: pool} }

// ListByPlayer returns every (player_car, car) row owned by a player.
func (r *Repo) ListByPlayer(ctx context.Context, playerID uuid.UUID) ([]Entry, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT pc.id, pc.car_id, c.name, pc.acquired_at, pc.customizations
		FROM player_cars pc
		JOIN cars c ON c.id = pc.car_id
		WHERE pc.player_id = $1
		ORDER BY pc.acquired_at ASC
	`, playerID)
	if err != nil {
		return nil, fmt.Errorf("list garage: %w", err)
	}
	defer rows.Close()

	var out []Entry
	for rows.Next() {
		var e Entry
		if err := rows.Scan(&e.PlayerCarID, &e.CarID, &e.Name, &e.AcquiredAt, &e.Customizations); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}