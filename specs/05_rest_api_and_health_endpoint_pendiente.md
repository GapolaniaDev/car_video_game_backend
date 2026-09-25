# Spec 05 — REST API Foundation + Health Endpoint

**Status:** pendiente
**Section:** 5, 24
**Depends on:** Specs 02, 03
**Blocks:** Specs 06, 13, 14

## Objective

Stand up a clean, versioned HTTP REST API (`/api/v1/`) and implement `GET /api/v1/health`. Architecture should be ready for future domain endpoints without creating enterprise boilerplate.

## Scope

### Server
- `cmd/api/main.go` boots an HTTP server on `:8080` (internal).
- Uses `net/http` from stdlib. May add a tiny router (`http.ServeMux` Go 1.22+ patterns) — **no third-party router unless justified**.
- `internal/api/router.go` builds routes.
- `internal/api/handlers/health.go` implements the health handler.
- Structured logging with `slog`.
- Read timeout / write timeout set sensibly (e.g., 15s / 15s).

### Endpoint
- `GET /api/v1/health`
  - Response 200:
    ```json
    { "status": "ok", "service": "backend-api" }
    ```
  - Optionally include `"database": "ok|down"` by calling `database.HealthCheck(ctx, pool)`. Keep response shape simple.
- Stub routes registered but returning 501 Not Implemented for future endpoints: `/api/v1/auth`, `/api/v1/players`, `/api/v1/cars`, `/api/v1/garage`, `/api/v1/tracks`, `/api/v1/matchmaking`, `/api/v1/races`, `/api/v1/leaderboard`. Just enough to prove the URL space; no business logic yet.

### Middleware
- Request logging middleware (method, path, status, latency) — no auth middleware yet, but the structure should allow adding one.
- Architecture note (in code comment): future JWT middleware will read `Authorization: Bearer <token>` and attach claims to context.

## Deliverables

- [ ] `cmd/api/main.go`
- [ ] `internal/api/router.go`
- [ ] `internal/api/handlers/health.go`
- [ ] `internal/api/middleware/logging.go`
- [ ] `internal/api/handlers/stub.go` for the future endpoint stubs
- [ ] Unit test for health handler (no DB required): returns 200 and correct JSON shape

## Acceptance Criteria

- `go build ./cmd/api` exits 0
- `curl http://localhost:8080/api/v1/health` returns 200 + correct JSON (when run locally outside Docker; inside Docker it's accessed via Nginx in Spec 08)
- Future endpoints respond 501 with a clear `{"error": "not implemented"}` body
- Logs are structured (`slog` JSON or text — pick one and document)

## Out of Scope

- Auth middleware (future milestone)
- Business logic for players/cars/garage/etc.
- HTTPS termination (handled by Nginx in Spec 08)