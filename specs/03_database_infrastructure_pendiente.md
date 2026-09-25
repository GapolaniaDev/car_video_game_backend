# Spec 03 — Database Infrastructure (Postgres Service + pgxpool)

**Status:** pendiente
**Section:** 12, 14, 17, 20
**Depends on:** Spec 02
**Blocks:** Specs 04, 05, 15

## Objective

Provide a reusable PostgreSQL connection layer using `pgxpool`, plus a working Postgres service definition that Docker Compose can spin up.

## Scope

### Go side — `internal/database/`
- Add `pgxpool` dependency.
- Function `New(ctx context.Context, cfg *config.Config) (*pgxpool.Pool, error)`.
- Function `Ping(ctx, pool)` for connectivity verification.
- Function `Close(pool)` for graceful shutdown.
- Function `HealthCheck(ctx, pool) error` for the `/health` endpoint to call.
- Use connection pooling defaults appropriate for MVP (no exotic tuning).
- Log lifecycle events via `slog` (no secrets).

### Docker side
- Postgres image: `postgres:16-alpine`.
- Environment: `POSTGRES_DB`, `POSTGRES_USER`, `POSTGRES_PASSWORD`.
- Healthcheck using `pg_isready -U "$POSTGRES_USER" -d "$POSTGRES_DB"`.
- Named volume `postgres_data` mounted to `/var/lib/postgresql/data`.
- NOT publicly exposed (no `ports:` mapping in `docker-compose.yml`; service network only).
- Internal network `racing-network`.

> NOTE: For this spec, only the Postgres service definition is written. The full `docker-compose.yml` is assembled in Spec 15. Until then, store the service block in a scratch file or inline in the spec's notes.

## Deliverables

- [ ] `internal/database/database.go`
- [ ] `internal/database/database_test.go` (test that opening with bad config fails; opening with valid config succeeds when DB available — use `Skip` if no DB in CI).
- [ ] Postgres service block prepared (will be merged into `docker-compose.yml` in Spec 15)

## Acceptance Criteria

- `go build ./...` succeeds
- `go test ./internal/database/...` passes (or skips cleanly)
- No secrets in logs

## Out of Scope

- Migrations (Spec 04)
- Schema definitions (Spec 04)
- Full docker-compose orchestration (Spec 15)