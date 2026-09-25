# Spec 19 — Matchmaking & UDP Protobuf Framing (Block 3)

**Status:** pendiente
**Section:** 8, 9, 10, 11, 28
**Depends on:** Spec 17 (JWT), Spec 18 (OpenAPI shapes for tokens)
**Blocks:** Spec 20

## Objective

Make the Game Server understand the Protobuf wire protocol defined in `protocol/game.proto`, validate the `game_token` on `JoinRaceRequest`, and add the `POST /api/v1/matchmaking/join` endpoint that issues a short-lived token plus a match id.

After this milestone, Unity can:
1. Authenticate (Spec 17).
2. `POST /api/v1/matchmaking/join` → receive `{ matchId, gameServerHost, gameServerPort, gameToken }`.
3. Open UDP to the Game Server.
4. Send `JoinRaceRequest { match_id, player_id, game_token }`.
5. Receive `JoinRaceResponse { ok=true, race_id, initial_tick }` from the server.

## Scope

### New package: `internal/matchmaking/`

- `service.go` — `Join(ctx, playerID, userID) (MatchInfo, error)`. For the MVP, "matchmaking" is **immediate**: as soon as a player asks, the service either (a) finds an existing pending race on the Game Server pool with < 4 players, or (b) creates a new one. No skill rating, no waiting queue. The service signs a short-lived `game_token` and returns routing info.
- `token.go` — signs and parses `game_token`. Uses **HMAC-SHA256** with a dedicated secret `GAME_TOKEN_SECRET` (separate from `JWT_SECRET`). Format: base64url(payload) + "." + base64url(HMAC). Payload = `{ match_id, player_id, exp }`. Lifetime 5 minutes.
- `repo.go` — persists match assignments in a new `matchmaking_assignments` table (see migration).
- `handler.go` — `POST /api/v1/matchmaking/join`. Protected by `auth.RequireAuth`. Body `{}`. Response `{ matchId, gameServerHost, gameServerPort, gameToken }`.
- `errors.go` — sentinels: `ErrNoGameServer`, `ErrRaceFull`, `ErrTokenInvalid`.

### New migration: `000004_matchmaking.up.sql`

```sql
CREATE TABLE matchmaking_assignments (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    match_id    UUID        NOT NULL,
    player_id   UUID        NOT NULL REFERENCES players(id) ON DELETE CASCADE,
    race_id     UUID,                  -- NULL until the Game Server allocates a race
    game_token  TEXT        NOT NULL,
    expires_at  TIMESTAMPTZ NOT NULL,
    consumed    BOOLEAN     NOT NULL DEFAULT FALSE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_match_assign_player ON matchmaking_assignments(player_id);
CREATE INDEX idx_match_assign_match  ON matchmaking_assignments(match_id);
```

Down migration drops the table.

### Game Server changes (`internal/networking/`, `cmd/game-server/`)

The text-protocol `HandlePacket` is **replaced** by a Protobuf-aware dispatcher:

- `internal/protoframing/framing.go` — small length-prefixed framing helper (4-byte big-endian length + payload). Matches what Unity will send.
- `internal/networking/dispatcher.go` — given a decoded protobuf message, returns the appropriate reply:
  - `JoinRaceRequest` → validate `game_token` (call into `internal/matchmaking/VerifyAndConsume` over HTTP or via a shared package — see "Architecture note" below) → reply `JoinRaceResponse`.
  - For all other messages: reply with an error `JoinRaceResponse{ ok=false, error="must JoinRaceRequest first" }` until the player is registered.
- `internal/networking/server.go` — replace `HandlePacket([]byte) []byte` with a `ServePacket([]byte, *net.UDPAddr) []byte` that:
  1. Reads the 4-byte length prefix.
  2. Unmarshals as `JoinRaceRequest` first (for now — full message-type dispatch arrives in Spec 20).
  3. Calls dispatcher.
  4. Marshals reply + length-prefixes it.
- `cmd/game-server/main.go` — load `GAME_TOKEN_SECRET` from config (shared secret with the API).

> Architecture note: token validation is the **only** cross-process dependency. Two options:
> (a) The Game Server calls a tiny internal HTTP endpoint on the API like `POST /internal/matchmaking/verify`. Adds a network hop.
> (b) Both services share `internal/matchmaking.VerifyAndConsume` which uses the same HMAC secret + checks the local `matchmaking_assignments` row in Postgres. Adds a DB dependency to the Game Server.
>
> **Decision for MVP:** option (b). The Game Server already does not depend on Postgres, so we add it. The `game_token` is self-validating via HMAC; the DB lookup just confirms the assignment exists and has not been consumed.

### Service health and routing info

- `GAME_SERVER_PUBLIC_HOST` env var, default `localhost` (configurable for prod where it would be the public DNS or a tunnel).
- `GAME_SERVER_PUBLIC_PORT` env var, default `7000`.
- The matchmaking service returns these in the response so Unity knows where to dial.

### Tests

- `internal/matchmaking/token_test.go` — sign + verify, expiry, wrong secret, replay-after-consume.
- `internal/matchmaking/handler_test.go` — handler returns the expected JSON shape; requires live DB (skip otherwise).
- `internal/networking/framing_test.go` — round-trip a `JoinRaceRequest` byte stream through the new dispatcher.
- `internal/networking/dispatcher_test.go` — happy path, expired token, wrong token, missing token. Use a mock or stub of the verifier for unit tests.

### Dependencies

- The existing `protocol/gamepb` package (no new dep).
- `github.com/google/uuid` (likely already in tree).
- No new HTTP libraries.

### Logging

- Matchmaking service logs: `player_id`, `match_id`, `race_id` (if known), `outcome`.
- Game Server logs: inbound message type (always `JoinRaceRequest` this milestone), outcome, remote addr. No payloads (tokens).

## Deliverables

- [ ] `migrations/000004_matchmaking.{up,down}.sql`
- [ ] `internal/matchmaking/{service,token,repo,handler,errors,handler_test}.go`
- [ ] `internal/protoframing/framing.go` + `framing_test.go`
- [ ] `internal/networking/dispatcher.go` + `dispatcher_test.go`
- [ ] `internal/networking/server.go` updated (replace text `HandlePacket`)
- [ ] `cmd/game-server/main.go` wired to load `GAME_TOKEN_SECRET`
- [ ] `internal/api/router.go` registers the new `/matchmaking/join` handler
- [ ] `internal/config/config.go` parses the new vars
- [ ] `.env.example` updated
- [ ] README updated (Matchmaking section + new game_token mechanism)
- [ ] `docs/openapi.yaml` updated with `/matchmaking/join`
- [ ] Old `cmd/udp-client` extended: `--mode=protobuf` sends a length-prefixed `JoinRaceRequest` against a mock token (best-effort / dev-only)

## Acceptance Criteria

- `go build ./...` exits 0.
- `go test ./...` exits 0.
- `docker compose build && docker compose up -d` runs the new migration.
- Live flow:
  - `POST /api/v1/auth/login` → token.
  - `POST /api/v1/matchmaking/join` with that token → `{ matchId, gameServerHost, gameServerPort, gameToken }`.
  - From the host, `go run ./cmd/udp-client --mode=protobuf --port=7000 --game-token=<...> --match-id=<...> --player-id=<...>` returns `Received: ok=true ...`.
  - Sending the same request twice → second time fails (token consumed).
  - Sending a request with a tampered token → `ok=false, error="invalid token"`.
- Logs show `match_id`, `player_id`, outcome — never the token itself.

## Out of Scope

- Real matchmaking algorithms (skill rating, wait queues, party support). MVP is "join and play".
- Per-race state, tick loop, physics. That is Spec 20.
- Race results persistence. That is Spec 21.
- Multiple Game Server processes. The MVP runs one; `GAME_SERVER_PUBLIC_HOST/PORT` is single-valued.

## Notes

- `game_token` and JWT are **deliberately separate**. JWT is the identity layer (REST, long-lived). `game_token` is the session layer (UDP, short-lived, single-use). Don't try to reuse the JWT over UDP.
- HMAC-SHA256 is enough for the MVP; switch to ECDSA or NaCl later if replay-attack resistance requires a stateful server-side nonce table. For MVP the `consumed` flag in Postgres plus the short TTL is sufficient.
---

## Done (2026-09-25)

Implemented on branch `feature/spec-19-matchmaking-protobuf` → merged into `develop`.

**Deliverables shipped**
- `migrations/000004_matchmaking.{up,down}.sql`
- `internal/matchmaking/{service,token,repo,handler,errors,crypto_test,token_test,token_testhelpers_test}.go`
- `internal/protoframing/{framing.go,framing_test.go}`: 4-byte BE length-prefix, 64 KiB cap
- `internal/networking/{dispatcher.go,dispatcher_test.go}` + `server.go` (replaces text `udp_server.go`)
- `cmd/game-server/main.go`: opens DB, wires dispatcher
- `cmd/udp-client/main.go`: --mode=protobuf sends a length-prefixed JoinRaceRequest
- `internal/api/router.go`: registers `POST /api/v1/matchmaking/join`
- `cmd/api/main.go`: builds Matchmaking service
- `docker-compose.yml`: `game-server` joins the network, depends on Postgres+migrate, both services receive `GAME_TOKEN_SECRET`
- `docs/openapi.yaml`: new path and `MatchInfo` schema
- `.env.example`: documents `GAME_SERVER_PUBLIC_HOST/PORT`

**Live verification**
- Register → mint match → `game_token` row in `matchmaking_assignments`
- UDP first join → `ok=true raceId=… initialTick=…`
- Replay same `game_token` → `ok=false error="matchmaking: token already consumed"`
- Tampered token (last char flipped) → `ok=false error="matchmaking: invalid token"`
