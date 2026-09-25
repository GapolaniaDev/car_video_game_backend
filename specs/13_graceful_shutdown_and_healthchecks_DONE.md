# Spec 13 — Graceful Shutdown & Health Checks

**Status:** DONE
**Section:** 23, 24
**Depends on:** Specs 05, 08
**Blocks:** Specs 15, 17

## Objective

Make both Go services shut down cleanly on container signals, and surface healthcheck endpoints/info that Docker Compose can rely on.

## Scope

### Graceful shutdown (both services)
- Listen for `SIGINT` and `SIGTERM`.
- Cancel a root `context.Context` with a reasonable timeout (e.g., 10s).
- API: `http.Server.Shutdown(ctx)` — stop accepting new connections, finish in-flight ones.
- Game Server: close UDP socket, exit read loop.
- Database: `pgxpool.Pool.Close()` from the API process.
- Log shutdown progress.

### Health checks
- `GET /api/v1/health` already returns basic status; extend to optionally call `db.Ping(ctx)` and report `"database": "ok" | "down"` (best effort, short timeout).
- For Docker healthcheck of the API: probe `GET /api/v1/health` every 10s, 3 retries, 30s start period.
- For Docker healthcheck of Postgres: `pg_isready -U $POSTGRES_USER -d $POSTGRES_DB` (defined in Spec 03 already; formalized in compose in Spec 15).
- Game Server has no HTTP — healthcheck handled via docker-compose UDP probe (or omitted, given UDP healthchecks are flaky). Document the choice.

## Deliverables

- [ ] Updated `cmd/api/main.go` with signal handling and `http.Server.Shutdown`
- [ ] Updated `cmd/game-server/main.go` with signal handling
- [ ] Updated `internal/api/handlers/health.go` reporting DB state
- [ ] Healthcheck sections in the future `docker-compose.yml`

## Acceptance Criteria

- `docker compose down` (after Spec 15) shows clean shutdowns in logs (no "force killed" entries)
- `docker stop` of an API container finishes within ~10s with the timeout chosen
- API health endpoint returns 200 with `{"status":"ok","service":"backend-api","database":"ok"}` when DB is up

## Out of Scope

- Hot reload
- Connection draining beyond the timeout