package race

import (
	"github.com/gustavo/racing-game-backend/protocol/gamepb"
)

// WorldSnapshot is the per-tick payload broadcast to all players in
// the race. Currently a 1:1 mapping to gamepb.WorldSnapshot.
type WorldSnapshot struct {
	Tick uint64
	Cars []SnapshotCar
}

// SnapshotCar is the per-vehicle projection the broadcaster hands to
// Unity (matches gamepb.CarState).
type SnapshotCar struct {
	PlayerID string
	Position gamepb.Vector3
	Rotation gamepb.Quaternion
	Velocity gamepb.Vector3
}

// buildWorldSnapshot composes a per-tick snapshot. Caller holds the
// race read lock (RLock).
func buildWorldSnapshot(r *Race, tick uint64) *WorldSnapshot {
	if r.status != StatusInProgress {
		// We still emit a snapshot during StatusStarting so clients
		// can render the countdown scene; during StatusPending/Finished
		// we skip to keep traffic low.
		if r.status != StatusStarting {
			return nil
		}
	}
	out := &WorldSnapshot{Tick: tick, Cars: make([]SnapshotCar, 0, len(r.players))}
	for _, p := range r.players {
		out.Cars = append(out.Cars, SnapshotCar{
			PlayerID: p.PlayerID.String(),
			Position: gamepb.Vector3{X: p.Position.X, Y: p.Position.Y, Z: p.Position.Z},
			Rotation: gamepb.Quaternion{Y: 0, W: 1}, // placeholder heading-as-quat
			Velocity: gamepb.Vector3{X: float32(p.Velocity), Y: 0, Z: 0},
		})
	}
	return out
}
