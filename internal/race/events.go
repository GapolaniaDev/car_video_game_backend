package race

import (
	"time"

	"github.com/google/uuid"
)

// Event is anything that falls out of the tick loop and must be
// observed by the broadcaster. The private eventMarker() method is
// the only thing that binds event types together.
type Event interface {
	eventMarker()
}

// CheckpointPassedEvt is emitted when a player crosses a waypoint.
type CheckpointPassedEvt struct {
	PlayerID uuid.UUID
	Index    int
	Tick     uint64
}

// LapCompletedEvt is emitted when a player completes a lap.
type LapCompletedEvt struct {
	PlayerID uuid.UUID
	Lap       int
	LapTimeMs int
	Tick      uint64
}

// RaceFinishedEvt is emitted when the race transitions to StatusFinished.
type RaceFinishedEvt struct {
	RaceID uuid.UUID
	// Results are 1-indexed (winner first).
	Results []RaceResult
}

// RaceResult is the public-facing shape; matches protobuf RaceResult.
type RaceResult struct {
	PlayerID    uuid.UUID
	Position    int
	TotalTimeMs int
	BestLapMs   int
}

// PlayerJoinedEvt is informational for the broadcaster.
type PlayerJoinedEvt struct {
	PlayerID uuid.UUID
	At        time.Time
}

// PlayerLeftEvt tells the broadcaster to drop the player's address.
type PlayerLeftEvt struct {
	PlayerID uuid.UUID
	Reason   string
}

func (CheckpointPassedEvt) eventMarker() {}
func (LapCompletedEvt) eventMarker()    {}
func (RaceFinishedEvt) eventMarker()    {}
func (PlayerJoinedEvt) eventMarker()    {}
func (PlayerLeftEvt) eventMarker()      {}

// PlayerInput is a slightly trimmed version of the protobuf input
// message — we don't need the sequence number inside the race
// package.
type PlayerInput struct {
	Throttle float32
	Brake    float32
	Steering float32
}

// PlayerState is the authoritative per-player state inside Race.
type PlayerState struct {
	PlayerID uuid.UUID

	Position Vec3
	Velocity float64
	Heading  float64

	LastInput     PlayerInput
	LastInputTick uint64

	Lap            int
	LastCheckpoint int // index of the last waypoint the player has crossed
	NextCheckpoint int // index of the next waypoint they need to cross (mirrors LastCheckpoint before cross)

	Finished   bool
	FinishPos  int
	FinishTime time.Time

	LapStartedAt time.Time
	BestLapMs    int
	StartedAt    time.Time
}