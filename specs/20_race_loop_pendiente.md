# Spec 20 — Race Loop (Block 4)

**Status:** pendiente
**Section:** 9, 10, 29, 30
**Depends on:** Spec 19 (matchmaking + Protobuf framing + game_token validation)
**Blocks:** Spec 21

## Objective

Add the authoritative race state machine inside the Game Server: per-race goroutines, a tick loop, simple car physics, checkpoint/lap detection, race timer, and a "snapshot + events" broadcast back to every player in the race.

After this milestone, Unity can send `PlayerInput` over UDP and receive `WorldSnapshot`, `CheckpointPassed`, `LapCompleted`, and `RaceFinished` messages until the race ends.

## Scope

### New package: `internal/race/`

Lives inside the Game Server binary only.

```
race.go        // Race struct + state machine
physics.go     // toy car kinematics: position, velocity, rotation
track.go       // in-memory track representation + checkpoint detection
state.go       // enums / constants for race status
events.go      // event bus / queue per race
manager.go     // RaceManager: lookup by race_id, create, list
```

#### Race struct

```
type Race struct {
    ID         uuid.UUID
    Track      *Track
    Status     RaceStatus   // pending, starting, in_progress, finished
    Players    map[uuid.UUID]*PlayerState
    StartTick  uint64
    TickRateHz int          // 30 by default
    StartTime  time.Time
    FinishTime time.Time

    events     chan Event   // buffered, drained on broadcast
    in         chan InputMsg
    join       chan JoinMsg
    leave      chan LeaveMsg
    done       chan struct{}
}
```

#### PlayerState

```
type PlayerState struct {
    PlayerID      uuid.UUID
    Car           CarState           // current authoritative state
    LastInput     PlayerInput        // last received input (for ack)
    LastInputTick uint32
    Lap           int
    LastCheckpoint int
    Finished      bool
    FinishPos     int
}
```

#### RaceManager

- `NewRace(trackID uuid.UUID, maxPlayers int) *Race`
- `Get(id uuid.UUID) (*Race, bool)`
- `List() []*Race`
- `Join(raceID, playerID) error`
- `Leave(raceID, playerID) error`
- `ApplyInput(playerID uuid.UUID, in PlayerInput)` — non-blocking enqueue
- `Run(race *Race)` — drives the tick loop until `Race.Status == finished`.

### Physics (`physics.go`)

A deliberately simple kinematic model so we can iterate fast:

```
accel     = (throttle - brake) * MAX_ACCEL        // m/s²
turn      = steering * MAX_STEER_RATE             // rad/s
velocity += accel * dt
heading  += turn   * dt
position += velocity * dt, rotated by heading
```

No collision with other cars in MVP. No terrain. The point is a working loop, not realism. Car `base_stats` from the catalog (loaded at race start) scale `MAX_ACCEL` / `MAX_STEER_RATE`.

### Tick loop

- `Run` goroutine per race.
- `dt = 1 / TickRateHz` (e.g. 33ms @ 30Hz).
- Each tick:
  1. Drain all pending `InputMsg` from `in`.
  2. Advance physics for each non-finished player.
  3. Detect checkpoint crossings; emit `CheckpointPassed`.
  4. Detect lap completions; emit `LapCompleted`.
  5. Compose a new `WorldSnapshot`; broadcast to all current players.
  6. Check finish condition (all players finished OR `FinishTime` elapsed); if so, transition to `finished`, emit `RaceFinished`, close `done`.

### Broadcast

- `internal/networking/broadcast.go` keeps a `map[playerID] *net.UDPAddr` per race.
- On tick, marshal `WorldSnapshot` (and any events accumulated during the tick) once per player (today: same payload to everyone; future: per-player visibility).
- The existing UDP write loop in `networking.UDPServer` becomes a thin per-conn dispatcher: it forwards bytes received on its socket into the appropriate race's `in`/`join`/`leave` channels based on which player is associated with the remote address. A simple `addr → playerID` table is maintained for the lifetime of each UDP connection.

### Race events

Events are typed:

```
type Event interface { eventMarker() }
type CheckpointPassedEvt struct { PlayerID uuid.UUID; Index uint32; Tick uint64 }
type LapCompletedEvt    struct { PlayerID uuid.UUID; Lap uint32; LapTimeMs uint32 }
type RaceFinishedEvt    struct { RaceID uuid.UUID; Results []RaceResult }
```

The race produces these; the broadcaster consumes them and emits Protobuf messages.

### New migration: `000005_race_state.up.sql`

```sql
-- Race state is authoritative on the Game Server. Postgres only sees
-- the final result. This migration is mostly a forward-compatible
-- schema in case we ever want to persist intermediate state (e.g. for
-- replay).

CREATE TABLE race_state_snapshots (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    race_id      UUID NOT NULL REFERENCES races(id) ON DELETE CASCADE,
    tick         BIGINT NOT NULL,
    snapshot     JSONB NOT NULL,
    captured_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_race_snap_race_id ON race_state_snapshots(race_id);
```

Not used by Spec 20; reserved for future replay/debug tooling.

Down migration drops the table.

### Dispatcher updates

`internal/networking/dispatcher.go` extends to handle, **once the player is registered** in a race:

- `PlayerInput` → enqueue into the right race's `in` channel.
- All other message types → ignored or `JoinRaceResponse{ ok=false, error="..." }`.

Until a player has successfully joined, only `JoinRaceRequest` is accepted.

### Race starting handshake

When the last needed player joins a race (or a 5-second auto-start timer fires after the first join), the race transitions `pending → starting`. The race then sends:

```
RaceStarting { start_tick, countdown_ms }
```

after `countdown_ms`, sends `RaceStarted { start_tick }` and transitions to `in_progress`.

For the MVP the countdown is fixed at 3 seconds; configurable later.

### Tests

- `internal/race/physics_test.go` — sanity checks on the kinematic formulas.
- `internal/race/track_test.go` — checkpoint detection around a circular path.
- `internal/race/manager_test.go` — `NewRace`, `Join`, `Leave`, `ApplyInput` happy paths.
- `internal/race/race_test.go` — `Run` end-to-end: 2 players, 1 lap, finish in correct order, both `WorldSnapshot` and `RaceFinished` emitted.
- `internal/networking/dispatcher_test.go` — extend with `PlayerInput` test (player joined first, input arrives → no error).
- All race tests should be deterministic: fixed tick rate, fixed dt, no time-based sleeps beyond what's required by the test.

### Logging

- Per race: lifecycle events (created, started, finished, all-players-left).
- Per tick: not logged (would be too noisy at 30Hz); only exceptions and finish events.
- Per broadcast error: `player_id`, `race_id`, `error`.

## Deliverables

- [ ] `migrations/000005_race_state.{up,down}.sql`
- [ ] `internal/race/{race,physics,track,state,events,manager,manager_test,race_test,physics_test,track_test}.go`
- [ ] `internal/networking/dispatcher.go` updated for `PlayerInput`
- [ ] `internal/networking/broadcast.go` + `broadcast_test.go`
- [ ] `cmd/game-server/main.go` wires `RaceManager` into the dispatcher
- [ ] `cmd/udp-client` extended with `--mode=player --player-id=... --race-id=...` that joins a race and prints the first few snapshots (dev-only)
- [ ] README updated ("Race loop" section)
- [ ] `docs/openapi.yaml` updated to note that the matchmaking endpoint now returns a `race_id` after Spec 19 already shipped

## Acceptance Criteria

- `go build ./...` exits 0.
- `go test ./...` exits 0. The end-to-end race test passes deterministically.
- `docker compose build && docker compose up -d` runs the new migration.
- Live flow (with 2+ clients):
  1. Each Unity-side `PlayerInput` produces a `WorldSnapshot` broadcast visible to all players in the race.
  2. Crossing a checkpoint emits `CheckpointPassed` to that player.
  3. Completing the lap emits `LapCompleted`.
  4. When all players finish, `RaceFinished` is emitted with the right ordering.
  5. After the race ends, no further inputs are processed; subsequent packets are dropped with a logged warning.
- A race that has zero players (everyone left) is reaped within 30 seconds and its memory released.

## Out of Scope

- Collision detection between cars.
- Off-track penalties / respawn.
- Boost pads, drift bonuses, AI opponents.
- Reconnect during a race (a player who dropped cannot rejoin mid-race in this milestone — Spec 21+).
- Spectator / relay server.

## Notes

- The Game Server is now a **stateful process**. Restarting it loses in-flight races. Document this in the README troubleshooting section.
- Keep the `Race` struct immutable from outside; all mutations go through channels. This is what makes it cheap to reason about correctness.
- Tick rate defaults to 30Hz but is configurable via `RACE_TICK_HZ` env var. Document the inverse relationship between tick rate and CPU usage.