package race

import (
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
)

// Track is an in-memory racing track: a series of 3D waypoints plus a
// closed lap loop. The first waypoint is the start/finish line.
type Track struct {
	ID        uuid.UUID       `json:"trackId"`
	Name      string          `json:"name"`
	Waypoints []Waypoint      `json:"waypoints"`
	// spawn_positions is a flat list of {x,y,z} used to seed
	// players. Empty maps all players to origin (deterministic tests
	// rely on this).
	Spawns []Vec3 `json:"spawns,omitempty"`
}

// Waypoint is one checkpoint on the track. In MVP all waypoints are
// pass-throughs; the renderer would interpret them physically.
type Waypoint struct {
	X float32 `json:"x"`
	Y float32 `json:"y"`
	Z float32 `json:"z"`
}

// Vec3 is a minimal 3D vector used by both Track and PlayerState.
type Vec3 struct {
	X float32 `json:"x"`
	Y float32 `json:"y"`
	Z float32 `json:"z"`
}

// ParseTrackLayout unmarshals a track's layout blob (stored in the
// tracks.layout column) into a Track. Used by the Game Server
// bootstrap which loads track metadata from Postgres.
func ParseTrackLayout(id uuid.UUID, name string, raw json.RawMessage) (*Track, error) {
	var t Track
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &t); err != nil {
			return nil, fmt.Errorf("race: parse track layout: %w", err)
		}
	}
	t.ID = id
	t.Name = name
	if len(t.Waypoints) == 0 {
		return nil, fmt.Errorf("race: track %s has no waypoints", name)
	}
	return &t, nil
}

// Len returns the number of waypoints.
func (t *Track) Len() int { return len(t.Waypoints) }

// NextCheckpoint returns the next checkpoint index the player should
// hit after `prev`, wrapping 0..Len()-1.
func (t *Track) NextCheckpoint(prev int) int {
	return (prev + 1) % t.Len()
}
