package race

import (
	"context"
	"errors"
	"log/slog"
	"math"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Manager owns the set of in-flight races.
type Manager struct {
	mu         sync.RWMutex
	races      map[uuid.UUID]*Race
	byMatch    map[uuid.UUID]uuid.UUID // match_id -> race_id
	trackFn    TrackLookup
	onFinish   func(*Race)
	tickHz     int
	maxLaps    int
	maxPlayers int
	log        *slog.Logger
}

// TrackLookup returns the track for a given match.
type TrackLookup func(matchID uuid.UUID) (*Track, error)

// NewManager constructs an empty manager. onFinish is invoked once per
// race, the moment the race transitions to StatusFinished. Pass nil to
// skip the hook.
func NewManager(tickHz, maxLaps, maxPlayers int, lookup TrackLookup, onFinish func(*Race), log *slog.Logger) *Manager {
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
	return &Manager{
		races:      map[uuid.UUID]*Race{},
		byMatch:    map[uuid.UUID]uuid.UUID{},
		trackFn:    lookup,
		onFinish:   onFinish,
		tickHz:     tickHz,
		maxLaps:    maxLaps,
		maxPlayers: maxPlayers,
		log:        log,
	}
}

// Get returns a race by ID.
func (m *Manager) Get(id uuid.UUID) (*Race, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	r, ok := m.races[id]
	return r, ok
}

// ByMatch returns the race currently associated with a match.
func (m *Manager) ByMatch(matchID uuid.UUID) (*Race, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	rid, ok := m.byMatch[matchID]
	if !ok {
		return nil, false
	}
	return m.races[rid], true
}

// List returns a snapshot of every race this manager currently knows.
func (m *Manager) List() []*Race {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]*Race, 0, len(m.races))
	for _, r := range m.races {
		out = append(out, r)
	}
	return out
}

// PlayerRace returns the race this player belongs to (best-effort).
func (m *Manager) PlayerRace(playerID uuid.UUID) (*Race, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, r := range m.races {
		r.mu.RLock()
		if _, ok := r.players[playerID]; ok {
			r.mu.RUnlock()
			return r, true
		}
		r.mu.RUnlock()
	}
	return nil, false
}

// NewRace creates a race for the given match and starts its tick loop.
func (m *Manager) NewRace(ctx context.Context, matchID uuid.UUID) (*Race, error) {
	track, err := m.trackFn(matchID)
	if err != nil {
		return nil, err
	}
	r := New(uuid.New(), track, m.tickHz, m.maxLaps, m.maxPlayers, m.log)
	r.OnFinish = m.onFinish
	m.mu.Lock()
	m.races[r.ID] = r
	m.byMatch[matchID] = r.ID
	m.mu.Unlock()

	m.log.Info("race.created",
		slog.String("race_id", r.ID.String()),
		slog.String("match_id", matchID.String()),
		slog.String("track", track.Name),
	)
	go r.Run(ctx)
	return r, nil
}

// SetOnFinish swaps the per-race OnFinish callback. Safe to call
// before any races exist (which is the typical wiring pattern). Use
// this to inject the persistence callback after the manager has been
// constructed but before races start.
func (m *Manager) SetOnFinish(cb func(*Race)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onFinish = cb
}

// RegisterPlayer routes a JoinRaceRequest to the right race.
func (m *Manager) RegisterPlayer(ctx context.Context, matchID, playerID uuid.UUID) (*Race, error) {
	r, ok := m.ByMatch(matchID)
	if !ok {
		created, err := m.NewRace(ctx, matchID)
		if err != nil {
			return nil, err
		}
		r = created
	}
	if !r.AddPlayer(playerID, DefaultSpec()) {
		return nil, errors.New("race: could not add player (full or finished)")
	}
	return r, nil
}

// CleanupFinished drops finished races from the manager. Game Server
// invokes this periodically.
func (m *Manager) CleanupFinished() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, r := range m.races {
		if r.Status() == StatusFinished {
			delete(m.races, id)
		}
	}
}

// Run is the per-race tick loop.
func (r *Race) Run(ctx context.Context) {
	dt := time.Second / time.Duration(r.TickRateHz)
	ticker := time.NewTicker(dt)
	defer ticker.Stop()

	tickIdx := uint64(0)
	started := false
	var lastEmpty time.Time

	finishRace := func() {
		r.mu.Lock()
		if r.status == StatusFinished {
			r.mu.Unlock()
			return
		}
		r.status = StatusFinished
		r.FinishTime = time.Now()
		results := r.finalResultsLocked()
		r.Events = append(r.Events, RaceFinishedEvt{
			RaceID:  r.ID,
			Results: results,
		})
		cb := r.OnFinish
		r.mu.Unlock()
		if cb != nil {
			cb(r)
		}
		r.closeOnce.Do(func() { close(r.Done) })
	}

	for {
		select {
		case <-ctx.Done():
			finishRace()
			return
		case <-r.Done:
			return
		default:
		}

		// Drain control messages non-blockingly.
		for {
			select {
			case m := <-r.JoinMsg:
				r.AddPlayer(m.PlayerID, m.CarSpec)
				r.mu.Lock()
				r.Events = append(r.Events, PlayerJoinedEvt{PlayerID: m.PlayerID, At: time.Now()})
				r.mu.Unlock()
			default:
				goto joinsDone
			}
		}
	joinsDone:
		for {
			select {
			case m := <-r.LeaveMsg:
				r.RemovePlayer(m.PlayerID)
				r.mu.Lock()
				r.Events = append(r.Events, PlayerLeftEvt{PlayerID: m.PlayerID, Reason: m.Reason})
				r.mu.Unlock()
			default:
				goto leavesDone
			}
		}
	leavesDone:

		r.mu.Lock()
		playerCount := len(r.players)
		statusNow := r.status

		// Auto-start when the first player joins.
		if statusNow == StatusPending && playerCount > 0 {
			r.status = StatusStarting
			r.StartRealTime = time.Now()
			r.Events = append(r.Events, RaceStartingEvt{})
		}
		_ = statusNow
		if r.status == StatusStarting && time.Since(r.StartRealTime) >= time.Duration(CountdownMillis)*time.Millisecond {
			r.status = StatusInProgress
			r.StartTick = tickIdx
			started = true
		}

		// Drain inputs.
		for {
			select {
			case m := <-r.InputCh:
				r.applyInputLocked(m)
			default:
				goto inputsDone
			}
		}
	inputsDone:

		// Physics & checkpoint detection.
		if r.status == StatusInProgress {
			r.advanceTickLocked(dt.Seconds(), tickIdx)
		}
		snapshot := buildWorldSnapshot(r, tickIdx)
		events := append([]Event(nil), r.Events...)
		r.Events = nil
		r.mu.Unlock()

		if snapshot != nil {
			select {
			case r.SnapshotOut <- *snapshot:
			default:
			}
		}
		_ = events
		_ = math.Sqrt // keep referenced

		// Empty-race reap.
		r.mu.RLock()
		playerCountNow := len(r.players)
		r.mu.RUnlock()
		if playerCountNow == 0 {
			if lastEmpty.IsZero() {
				lastEmpty = time.Now()
			} else if time.Since(lastEmpty) > EmptyRaceReapAfter {
				r.log.Info("race.reaped.empty", slog.String("race_id", r.ID.String()))
				finishRace()
				return
			}
		} else {
			lastEmpty = time.Time{}
		}

		// Finish condition.
		if started && allPlayersFinished(r) {
			finishRace()
			return
		}

		tickIdx++
		select {
		case <-ticker.C:
		case <-ctx.Done():
			finishRace()
			return
		case <-r.Done:
			return
		}
	}
}

func nonBlocking(_ chan JoinMsg) bool { return true }

// RaceStartingEvt is emitted when the countdown begins.
type RaceStartingEvt struct{}

func (RaceStartingEvt) eventMarker() {}

func allPlayersFinished(r *Race) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if len(r.players) == 0 {
		return false
	}
	for _, p := range r.players {
		if !p.Finished {
			return false
		}
	}
	return true
}

// applyInputLocked requires r.mu held.
func (r *Race) applyInputLocked(m InputMsg) {
	p, ok := r.players[m.PlayerID]
	if !ok {
		return
	}
	if p.Finished {
		return
	}
	p.LastInput = m.Input
	p.LastInputTick = uint64(m.Sequence)
}

// advanceTickLocked requires r.mu held.
func (r *Race) advanceTickLocked(dt float64, tick uint64) {
	const cpRadius = 8.0
	for _, p := range r.players {
		if p.Finished {
			continue
		}
		spec, ok := r.specs[p.PlayerID]
		if !ok {
			spec = DefaultSpec()
		}
		Step(p, p.LastInput, spec, dt)
		// Checkpoint detection.
		if len(r.Track.Waypoints) == 0 {
			continue
		}
		idx := p.NextCheckpoint % len(r.Track.Waypoints)
		wp := r.Track.Waypoints[idx]
		dx := float64(p.Position.X) - float64(wp.X)
		dz := float64(p.Position.Z) - float64(wp.Z)
		if math.Hypot(dx, dz) < cpRadius {
			// The next-checkpoint pointer advances by exactly one.
			wasStartLine := idx == 0
			p.LastCheckpoint = idx
			p.NextCheckpoint = (idx + 1) % len(r.Track.Waypoints)
			r.Events = append(r.Events, CheckpointPassedEvt{
				PlayerID: p.PlayerID, Index: idx, Tick: tick,
			})
			// Lap completion is recognised when the player crosses
			// the start/finish line (waypoint 0) after at least one
			// other checkpoint has been touched.
			if wasStartLine && p.Lap > 0 {
				now := time.Now()
				lapMs := int(now.Sub(p.LapStartedAt) / time.Millisecond)
				if p.BestLapMs == 0 || lapMs < p.BestLapMs {
					p.BestLapMs = lapMs
				}
				p.Lap++
				p.LapStartedAt = now
				r.Events = append(r.Events, LapCompletedEvt{
					PlayerID: p.PlayerID, Lap: p.Lap, LapTimeMs: lapMs, Tick: tick,
				})
				if p.Lap >= r.MaxLaps {
					p.Finished = true
					p.FinishTime = now
					p.FinishPos = len(r.finishedOrder) + 1
					r.finishedOrder = append(r.finishedOrder, p.PlayerID)
				}
			} else if wasStartLine && p.Lap == 0 {
				// First time crossing the start line counts as the
				// first lap already in progress; bump Lap so the
				// next crossing closes it.
				p.Lap = 1
			}
		}
	}
}

// finalResultsLocked requires r.mu held.
func (r *Race) finalResultsLocked() []RaceResult {
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
