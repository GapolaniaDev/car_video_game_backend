// Package race implements the authoritative race loop: per-race
// goroutines, deterministic tick loop, toy kinematics, checkpoint
// detection and event broadcasting.
//
// Lives in the Game Server only.
package race

import "time"

// RaceStatus is the state-machine for a race's lifecycle.
type RaceStatus int

const (
	// StatusPending — created, no players yet.
	StatusPending RaceStatus = iota
	// StatusStarting — countdown in progress.
	StatusStarting
	// StatusInProgress — race is live.
	StatusInProgress
	// StatusFinished — race over, no more inputs accepted.
	StatusFinished
)

// String renders a RaceStatus for logs.
func (s RaceStatus) String() string {
	switch s {
	case StatusPending:
		return "pending"
	case StatusStarting:
		return "starting"
	case StatusInProgress:
		return "in_progress"
	case StatusFinished:
		return "finished"
	default:
		return "unknown"
	}
}

// Defaults for the tick loop. Configurable via env in a future spec.
const (
	DefaultTickHz       = 30
	DefaultMaxPlayers   = 4
	DefaultMaxLaps      = 1
	CountdownMillis     = 3_000
	EmptyRaceReapAfter  = 30 * time.Second
)
