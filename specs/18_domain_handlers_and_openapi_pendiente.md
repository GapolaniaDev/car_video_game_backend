# Spec 18 — Domain Handlers & OpenAPI (Block 2)

**Status:** pendiente
**Section:** 5, 6, 29
**Depends on:** Spec 17 (auth + JWT middleware)
**Blocks:** Spec 19

## Objective

Replace the 501 stubs under `/api/v1/players`, `/api/v1/cars`, `/api/v1/garage`, `/api/v1/tracks` with real, JWT-protected handlers backed by Postgres. Ship the OpenAPI 3.x spec as the contract Unity will consume.

After this milestone, Unity can:
- Authenticate (Spec 17) → call `GET /players/me`, `GET /cars`, `GET /garage`, `GET /tracks` and receive real data.

## Scope

### Database changes

Migration `000003_seeds.up.sql`:

- Insert a small **cars** seed (3 models): `Veloce`, `Bruiser`, `Drifter`. Each with a representative `base_stats` JSON (`{ "top_speed": …, "acceleration": …, "handling": … }`).
- Insert a small **tracks** seed (2 tracks): `Crescent Bay`, `Granite Pass`. Each with a `layout` JSON describing waypoints (`{ "waypoints": [ { "x":..,"y":..,"z":.. }, … ] }`).
- Add a default car to every existing player via `INSERT INTO player_cars ... SELECT id, <seed_car_id> FROM players WHERE ...` so freshly-authenticated guests immediately see something in their garage.

`000003_seeds.down.sql` removes those seeds (idempotent: delete by name).

### Domain packages

Each domain lives in its own `internal/<name>/` package:

- `internal/player/` — repo + service + handler
- `internal/cars/` — repo (read-only catalog) + handler
- `internal/garage/` — repo + handler
- `internal/tracks/` — repo (read-only catalog) + handler

Each follows the same shape:

```
repo.go      // pgx queries
service.go   // business rules, no HTTP types
handler.go   // http.HandlerFunc, decodes JSON, calls service, writes JSON
handler_test.go
```

No business logic in handlers. No HTTP types in services. No SQL in services.

### Endpoints

All routes under `/api/v1/`. All routes other than `/health` and `/auth/*` are protected by `auth.RequireAuth` middleware.

| Method | Path                       | Auth | Description                                       |
|--------|----------------------------|------|---------------------------------------------------|
| GET    | `/api/v1/players/me`       | yes  | Returns the authenticated player profile.         |
| GET    | `/api/v1/cars`             | yes  | Catalog of all car models.                        |
| GET    | `/api/v1/cars/{id}`        | yes  | One car by UUID.                                  |
| GET    | `/api/v1/garage`           | yes  | Cars owned by the authenticated player.           |
| GET    | `/api/v1/tracks`           | yes  | Catalog of all tracks.                            |
| GET    | `/api/v1/tracks/{id}`      | yes  | One track by UUID.                                |

### Response shapes

```jsonc
// GET /players/me
{
  "playerId": "uuid",
  "userId":   "uuid",
  "displayName": "...",
  "isGuest":  true,
  "createdAt": "RFC3339",
  "lastLoginAt": "RFC3339 | null"
}

// GET /cars
[{ "carId":"uuid", "name":"Veloce", "baseStats":{...} }, …]

// GET /garage
[{ "playerCarId":"uuid", "carId":"uuid", "name":"Veloce",
   "acquiredAt":"RFC3339", "customizations":{} }, …]

// GET /tracks
[{ "trackId":"uuid", "name":"Crescent Bay", "layout":{...} }, …]
```

### Errors

- 400: malformed UUID or missing path parameter.
- 401: handled by middleware.
- 404: when an `id` route resolves nothing.
- 500: logged with full request context, response body is generic `{"error":"internal"}`.

### OpenAPI 3.x

- Location: `docs/openapi.yaml`.
- Covers **every** route currently registered by the router (health + auth + players + cars + garage + tracks + the still-501 matchmaking/races/leaderboard stubs), with the appropriate `2xx` / `4xx` / `5xx` responses.
- Schemas named in `components.schemas`: `Player`, `Car`, `PlayerCar`, `Track`, `ErrorResponse`, `AuthRequest`, `AuthResponse`, `HealthResponse`.
- A `BearerAuth` security scheme under `components.securitySchemes` referenced from the protected endpoints.
- Top-level `info.title = "Racing Game Backend API"`, `version: "0.1.0"`.
- Server entry: `http://localhost:8081/api/v1` (via Nginx).

### Router wiring (`internal/api/router.go`)

- Group protected routes behind `protected := middleware.RequireAuth(log)`.
- Domain handlers take a small `DomainDeps` struct that contains `*database.Pool` and `*slog.Logger`.
- The 501 stubs for `/matchmaking`, `/races`, `/leaderboard` remain (their real implementations are Specs 19, 20, 21).

### Logging

- One log line per request as today.
- Domain handlers additionally log outcome with `player_id`, `route`, `latency`.
- Never log full response bodies (may contain player PII).

### Tests

- Each domain gets at minimum:
  - one happy-path unit test against a real DB (or `t.Skip` if `INTEGRATION_POSTGRES_URL` is unset),
  - one `httptest` test for the handler (success and the 401 path).
- `docs/openapi.yaml` validation: smoke test that loads the YAML and asserts every path in the router has a matching entry.

### Dependencies

- `github.com/google/uuid` (already transitively present via pgx, but add explicit import).
- `gopkg.in/yaml.v3` for the OpenAPI smoke test parser.

## Deliverables

- [ ] `migrations/000003_seeds.{up,down}.sql`
- [ ] `internal/player/{repo,service,handler,handler_test}.go`
- [ ] `internal/cars/{repo,handler,handler_test}.go`
- [ ] `internal/garage/{repo,handler,handler_test}.go`
- [ ] `internal/tracks/{repo,handler,handler_test}.go`
- [ ] `internal/api/router.go` wired with protected groups
- [ ] `docs/openapi.yaml` covering every registered route
- [ ] OpenAPI smoke test
- [ ] README updated with new endpoints and a link to the OpenAPI doc
- [ ] `.env.example` unchanged (no new vars in this spec)

## Acceptance Criteria

- `go build ./...` exits 0.
- `go test ./...` exits 0; DB tests skip cleanly without a live DB.
- `docker compose build && docker compose up -d` runs the new migration and seeds.
- Live (after acquiring a token via Spec 17):
  - `GET /api/v1/players/me` returns 200 + a Player JSON.
  - `GET /api/v1/cars` returns the 3 seeded cars.
  - `GET /api/v1/garage` returns at least 1 car for the authenticated player (from the seed).
  - `GET /api/v1/tracks` returns the 2 seeded tracks.
  - Any of the above without `Authorization` returns 401.
- `docs/openapi.yaml` is valid YAML and contains every route the router exposes today.

## Out of Scope

- Writing to the car catalog or track catalog (read-only for MVP).
- Garage mutations (acquire / sell / customize). Future milestone.
- Player profile edits (display name, avatar). Future milestone.
- Pagination — catalog responses are small for MVP; revisit when car/track count grows.

## Notes

- Reuse the `internal/auth.RequireAuth` middleware. Add `auth.PlayerIDFromContext(ctx) (uuid.UUID, bool)` helper so handlers can read the authenticated player without parsing claims themselves.
- Keep JSON tags explicit on every response struct; do not rely on default `MarshalJSON` behavior.
- The OpenAPI YAML must be human-edited (not generated). It is the contract — it should reflect intent, not whatever Go happens to marshal.