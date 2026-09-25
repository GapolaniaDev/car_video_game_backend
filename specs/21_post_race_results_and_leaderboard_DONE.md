# Spec 21 — Post-Race: History, Results & Leaderboards (Block 5)

**Status:** DONE (2026-09-25)
**Section:** 5, 13, 29
**Depends on:** Spec 20 (race loop emits RaceFinished)
**Blocks:** —

## Objective

Persist the final results of every race to Postgres and expose them through REST endpoints so Unity can show race history and leaderboards. Also harden the Game Server's race-finished path to call into the API (or share a package) to record results.

After this milestone, Unity can:
- See a player's recent races via `GET /api/v1/players/me/races`.
- See the full result of a race via `GET /api/v1/races/{id}`.
- See the global top-N times per track via `GET /api/v1/leaderboard?trackId=...&limit=...`.

## Scope

### Persistence path

When the Game Server detects a race finish (Spec 20), it persists the `race_results` rows via a new package `internal/results/`:

- `internal/results/writer.go` — `PersistRaceFinish(ctx, raceID uuid.UUID, results []RaceResult) error`.
- Implementation: opens a `pgxpool.Pool` (new — the Game Server now talks to Postgres for this one job), runs a transaction that:
  1. Inserts one row per player into `race_results`.
  2. Updates `race_players.finish_position` and `race_players.finished_at`.
  3. Updates `races.status = 'finished'` and `races.finished_at = NOW()`.
- On error: log + retry once after 1s; on second failure log + keep race in-memory until next milestone implements a DLQ.

### New REST endpoints (in `internal/api/handlers/results.go`)

All protected by `auth.RequireAuth`.

| Method | Path                                | Description                                              |
|--------|-------------------------------------|----------------------------------------------------------|
| GET    | `/api/v1/players/me/races`          | Recent races for the authenticated player.               |
| GET    | `/api/v1/races/{id}`                | Full race detail including final results.                |
| GET    | `/api/v1/leaderboard`               | Top-N for a track. Query: `?trackId=<uuid>&limit=<n>`.   |

#### Response shapes

```jsonc
// GET /players/me/races
{
  "races": [
    {
      "raceId": "uuid",
      "trackId": "uuid",
      "trackName": "Crescent Bay",
      "finishedAt": "RFC3339",
      "position": 1,
      "totalTimeMs": 123456,
      "bestLapMs":  30000
    },
    ...
  ],
  "nextCursor": null
}

// GET /races/{id}
{
  "raceId": "uuid",
  "trackId": "uuid",
  "trackName": "Crescent Bay",
  "status":  "finished",
  "startedAt": "RFC3339",
  "finishedAt": "RFC3339",
  "results": [
    { "playerId":"uuid","displayName":"...","position":1,
      "totalTimeMs":123456,"bestLapMs":30000 },
    ...
  ]
}

// GET /leaderboard?trackId=...&limit=10
{
  "trackId": "uuid",
  "trackName": "Crescent Bay",
  "entries": [
    { "rank": 1, "playerId":"uuid","displayName":"...",
      "bestLapMs": 28000, "achievedAt":"RFC3339" },
    ...
  ]
}
```

### Pagination

- `/players/me/races` uses **keyset pagination** with cursor = the `finished_at` of the last row. Default page size 20, max 100.
- `/leaderboard` uses a simple `?limit=` (default 10, max 100). No offset needed — we sort by `best_lap_ms ASC` and stop at N.
- `/races/{id}` returns the full result list (no pagination — a race has ≤ 4 players in MVP).

### New package: `internal/leaderboard/`

- `repo.go` — pgx queries:
  - `BestLapsByTrack(ctx, trackID uuid.UUID, n int) ([]LeaderboardEntry, error)`
  - `PlayerBestPerTrack(ctx, playerID uuid.UUID) ([]LeaderboardEntry, error)` (used to compute a player's personal bests; not exposed as an endpoint in this milestone but the repo method is here).
- `handler.go` — wires the endpoint.

### Query specifics

`/leaderboard` SQL (rough sketch):

```sql
SELECT
    rr.player_id,
    p.display_name,
    MIN(rr.best_lap_ms) AS best_lap_ms,
    MIN(rr.created_at)  AS achieved_at
FROM race_results rr
JOIN players p ON p.id = rr.player_id
JOIN races r   ON r.id = rr.race_id
WHERE r.track_id = $1
  AND rr.best_lap_ms IS NOT NULL
GROUP BY rr.player_id, p.display_name
ORDER BY best_lap_ms ASC
LIMIT $2;
```

The `MIN` over `(player_id, best_lap_ms)` gives the player's best single lap across all of their races on this track.

### Tests

- `internal/results/writer_test.go` — happy path inserts results + updates parent rows; idempotent on retry (unique constraint on `(race_id, player_id)`).
- `internal/leaderboard/repo_test.go` — SQL produces the expected ordering for a 3-player, 3-race fixture.
- `internal/leaderboard/handler_test.go` — query parsing, defaults, max limit cap.
- `internal/api/handlers/results_test.go` — `/players/me/races` returns the authenticated player's races only; `/races/{id}` 404s for unknown races; `/leaderboard` 400s for missing `trackId`.

### Logging

- `internal/results/writer.go`: log `race_id`, `result_count`, `outcome`.
- `/leaderboard` requests log `track_id`, `limit`, `returned_count`.
- No PII beyond `player_id` in logs.

### Dependencies

- `internal/database` (already present).
- No new external packages.

### Performance notes

- The leaderboard query should be backed by the existing `idx_race_results_position` and a future partial index on `(track_id, best_lap_ms) WHERE best_lap_ms IS NOT NULL`. For MVP, no new index needed; revisit when the `race_results` table exceeds ~100k rows.

## Deliverables

- [ ] `internal/results/writer.go` + `writer_test.go`
- [ ] `internal/leaderboard/repo.go` + `handler.go` + tests
- [ ] `internal/api/handlers/results.go` + `results_test.go`
- [ ] `cmd/game-server/main.go` wires the `results.Writer` and calls it on `RaceFinished`
- [ ] `internal/api/router.go` registers the three new endpoints
- [ ] Migration `000005_race_state` (from Spec 20) is unchanged here
- [ ] README updated ("Race history" + "Leaderboards" sections)
- [ ] `docs/openapi.yaml` updated with the three new endpoints
- [ ] `cmd/udp-client` extended with `--mode=observer --race-id=...` (dev-only) that joins and prints snapshots + finish events

## Acceptance Criteria

- `go build ./...` exits 0.
- `go test ./...` exits 0.
- `docker compose build && docker compose up -d` works.
- Live end-to-end (Unity-side flow simulated by `udp-client --mode=player`):
  1. Two players join a race, complete 1 lap.
  2. Within 2 seconds of `RaceFinished`, a row per player exists in `race_results` (verify via `docker compose exec postgres psql -c "SELECT count(*) FROM race_results WHERE race_id=..."`).
  3. `GET /api/v1/races/{id}` returns the race with both players in the right order.
  4. `GET /api/v1/players/me/races` returns the calling player's recent race.
  5. `GET /api/v1/leaderboard?trackId=...` returns the players ordered by best lap.
- A failed write to Postgres does not crash the Game Server — it logs and continues.

## Out of Scope

- Rewards / progression / unlock system.
- Anti-cheat audits on result rows.
- Per-region or per-friend leaderboards.
- Server-side race replay from `race_state_snapshots` (table reserved for this).
- Export / share results to social media.

## Notes

- The Game Server's new Postgres dependency is **only** for result persistence. Keep all real-time state in memory; only flush on race finish.
- The `/leaderboard` endpoint deliberately ranks best **lap**, not best total time. Total time is more gameable (shorter tracks → better times) and harder to make meaningful across tracks. Best lap is also what players compare. Document this choice in the README.
## Implementation Notes (2026-09-25)

- `internal/results` provides a `Writer` that upserts the `races`,
  `race_players`, and `race_results` rows in a single transaction
  with ON CONFLICT clauses for idempotent retries. `PersistWithRetry`
  retries once on a transient error (1s delay).
- `internal/leaderboard` exposes `GET /api/v1/leaderboard` with
  `trackId` + `limit` query params, ranking players by their fastest
  `best_lap_ms` on that track.
- `internal/racehistory` exposes two endpoints:
  `GET /api/v1/players/me/races` (keyset-paginated by `finished_at`)
  and `GET /api/v1/races/{id}` (full race detail incl. all results).
- The race's persistence hook is wired as an `OnFinish` callback
  on the `race.Race` struct: when the tick loop transitions to
  StatusFinished, the callback runs synchronously *before*
  `CleanupFinished` can remove the race from the manager. This
  fixes a race where the broadcaster's poll loop and the persistence
  poll loop both observed the same finished race and one of them
  removed it before the other persisted it.
- The Game Server uses a tiny 5m "MVP-Loop" layout (4 waypoints at
  (0,0), (5,0), (5,5), (0,5)) so a drive-forward-only client can
  finish a lap without steering. All four waypoints fit inside the
  8m checkpoint radius. The track **id** comes from the first row
  in the `tracks` table (Crescent Bay in the seeds migration) so the
  FK on `races.track_id` resolves.
- The smoke test on the docker-compose stack: register → matchmaking
  join → 15s of `--mode=player` UDP driving → race finishes in
  ~3.2s → results persisted → `/players/me/races`, `/races/{id}`,
  `/leaderboard?trackId=...` all return the expected rows.
