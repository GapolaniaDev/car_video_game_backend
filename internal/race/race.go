package race

import (
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
)

// JoinMsg is sent from the dispatcher to the race when a player
// finishes the JoinRaceRequest handshake.
type JoinMsg struct {
	PlayerID uuid.UUID
	CarSpec  CarSpec
}

// LeaveMsg is sent when a player's UDP address drops or they
// explicitly leave.
type LeaveMsg struct {
	PlayerID uuid.UUID
	Reason   string
}

// InputMsg is a thin wrapper carrying sequence numbers so the race
// can ack + ack-with-jitter in the future (MVP keeps the latest only).
type InputMsg struct {
	PlayerID uuid.UUID
	Input    PlayerInput
	Sequence uint32
}

// Race is the authoritative state machine for one race. All mutation
// goes through channels; external goroutines never touch fields
// directly. Run() drives the tick loop until StatusFinished and then
// closes Done.
type Race struct {
	ID         uuid.UUID
	Track      *Track
	StartTick  uint64
	TickRateHz int
	MaxLaps    int
	MaxPlayers int

	mu      sync.RWMutex
	status  RaceStatus
	players map[uuid.UUID]*PlayerState
	specs   map[uuid.UUID]CarSpec

	// ordering for finish position. finishedOrder is appended to
	// in the order players cross the line. Together with finishPos
	// (assigned on append) it yields the final ranking.
	finishedOrder []uuid.UUID

	CreatedAt     time.Time
	StartRealTime time.Time
	FinishTime    time.Time

	Events []Event // drained by broadcaster after each tick

	JoinMsg      chan JoinMsg
	LeaveMsg     chan LeaveMsg
	InputCh      chan InputMsg
	Done         chan struct{}
	SnapshotOut  chan WorldSnapshot
	closeOnce    sync.Once

	log *slog.Logger
}

// New constructs a race. The race is NOT auto-started — call Run in
// its own goroutine.
func New(id uuid.UUID, track *Track, tickHz, maxLaps, maxPlayers int, log *slog.Logger) *Race {
	if log == nil {
		log = slog.Default()
	}
	if tickHz <= 0 {
		tickHz = DefaultTickHz
	}
	if maxLaps <= 0 {
		maxLaps = DefaultMaxLaps
	}
	if maxPlayers <= 0 {
		maxPlayers = DefaultMaxPlayers
	}
	return &Race{
		ID:         id,
		Track:      track,
		TickRateHz: tickHz,
		MaxLaps:    maxLaps,
		MaxPlayers: maxPlayers,
		status:     StatusPending,
		players:    map[uuid.UUID]*PlayerState{},
		specs:      map[uuid.UUID]CarSpec{},
		CreatedAt:  time.Now(),
		Events:     nil,
		JoinMsg:    make(chan JoinMsg, 8),
		LeaveMsg:   make(chan LeaveMsg, 8),
		InputCh:    make(chan InputMsg, 256),
		Done:       make(chan struct{}),
		SnapshotOut: make(chan WorldSnapshot, 4),
		log:        log,
	}
}

// Status returns the current RaceStatus under the mutex.
func (r *Race) Status() RaceStatus {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.status
}

// PlayersSnapshot returns a shallow copy of all current players and
// their state. Used by the broadcaster to compose WorldSnapshot.
func (r *Race) PlayersSnapshot() []*PlayerState {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*PlayerState, 0, len(r.players))
	for _, p := range r.players {
		out = append(out, p)
	}
	return out
}

// AddPlayer registers a player in the race. Used during the
// JoinRaceRequest handshake.
func (r *Race) AddPlayer(playerID uuid.UUID, spec CarSpec) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.status == StatusFinished {
		return false
	}
	if len(r.players) >= r.MaxPlayers {
		return false
	}
	if _, ok := r.players[playerID]; ok {
		return true
	}
	r.players[playerID] = &PlayerState{
		PlayerID:       playerID,
		Position:       spawn(r.Track, len(r.players)),
		NextCheckpoint: 1 % lenOr1(r.Track.Waypoints),
		Lap:            0,
		StartedAt:      time.Now(),
		LapStartedAt:   time.Now(),
	}
	r.specs[playerID] = spec
	return true
}

// RemovePlayer drops a player. Safe to call multiple times.
func (r *Race) RemovePlayer(playerID uuid.UUID) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.players, playerID)
	delete(r.specs, playerID)
}

// ApplyInput enqueues a player input. Non-blocking; drops on full
// channel (caller decides how to react — MVP logs).
func (r *Race) ApplyInput(m InputMsg) {
	select {
	case r.InputCh <- m:
	default:
		r.log.Debug("race.input.drop",
			slog.String("race_id", r.ID.String()),
			slog.String("player_id", m.PlayerID.String()),
		)
	}
}

// EnqueueJoin enqueues a JoinMsg from the dispatcher.
func (r *Race) EnqueueJoin(m JoinMsg) {
	select {
	case r.JoinMsg <- m:
	default:
	}
}

// EnqueueLeave enqueues a LeaveMsg.
func (r *Race) EnqueueLeave(m LeaveMsg) {
	select {
	case r.LeaveMsg <- m:
	default:
	}
}

// FinalResults returns the ordered RaceResults (winner first).
func (r *Race) FinalResults() []RaceResult {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]RaceResult, 0, len(r.finishedOrder))
	for i, pid := range r.finishedOrder {
		p, ok := r.players[pid]
		if !ok {
			continue
		}
		total := int(p.FinishTime.Sub(p.StartedAt) / time.Millisecond)
		out = append(out, RaceResult{
			PlayerID:    pid,
			Position:    i + 1,
			TotalTimeMs: total,
			BestLapMs:   p.BestLapMs,
		})
	}
	return out
}

// --- helpers -------------------------------------------------------------

func spawn(t *Track, idx int) Vec3 {
	if t == nil || len(t.Spawns) == 0 {
		// Fallback: fan out along the +x axis so multi-player tests
		// don't all start at exactly the same point.
		return Vec3{X: float32(idx) * 2, Y: 0, Z: 0}
	}
	return t.Spawns[idx%len(t.Spawns)]
}

func lenOr1(ws []Waypoint) int {
	if len(ws) == 0 {
		return 1
	}
	return len(ws)
}
