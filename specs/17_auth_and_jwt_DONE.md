# Spec 17 — Authentication & JWT (Block 1)

**Status:** DONE
**Section:** 7, 29
**Depends on:** Specs 02 (config), 04 (migrations), 05 (router), 13 (graceful shutdown)
**Blocks:** Specs 18, 19

## Objective

Replace the `/api/v1/auth*` 501 stubs with real endpoints that issue JWTs, and add JWT-validation middleware that future domain handlers can opt into. After this milestone, Unity can create or recover a player account and receive a token it can use to call protected endpoints.

## Scope

### Database changes

New migration `000002_auth.up.sql`:

- Add column `users.last_login_at TIMESTAMPTZ` (nullable).
- Add column `players.avatar_url TEXT` (nullable) — small addition we keep needing everywhere; cheap to add now.
- No structural changes to existing tables; only `ALTER TABLE … ADD COLUMN …`.

Down migration drops both columns.

### Configuration additions (no breaking changes)

- New env var `JWT_ACCESS_TTL` (default `"1h"`) — token lifetime for `accessToken`.
- New env var `JWT_ISSUER` (default `"racing-game-backend"`).
- The existing `JWT_SECRET` becomes mandatory (it was already required outside dev).

### New package: `internal/auth/`

- `password.go` — bcrypt helpers: `HashPassword(plain string) (string, error)`, `VerifyPassword(hash, plain string) error`. Cost factor 12.
- `jwt.go` — `IssueAccessToken(playerID, userID string) (string, error)` and `ParseAccessToken(raw string) (*Claims, error)`.
  - Library: `github.com/golang-jwt/jwt/v5` (single small dep).
  - Claims: `sub` = playerID, `uid` = userID, `iss` = JWT_ISSUER, `iat`, `exp`.
- `middleware.go` — `RequireAuth(log *slog.Logger) func(http.Handler) http.Handler`:
  - Reads `Authorization: Bearer <token>`.
  - Validates signature, expiry, issuer.
  - On success, attaches claims to `r.Context()` via a typed key.
  - On failure, returns 401 with `{"error":"unauthorized"}`.
- `errors.go` — sentinel errors `ErrInvalidCredentials`, `ErrEmailTaken`, `ErrPlayerNotFound`, `ErrUnauthorized`.

### New handlers (`internal/api/handlers/auth.go`)

- `POST /api/v1/auth/register`
  - Body: `{ "email": "...", "password": "...", "displayName": "..." }`
  - 200: `{ "accessToken": "...", "playerId": "uuid" }`
  - 400 if email invalid or password < 8 chars.
  - 409 if email already exists.
- `POST /api/v1/auth/login`
  - Body: `{ "email": "...", "password": "..." }`
  - 200: `{ "accessToken": "...", "playerId": "uuid" }`
  - 401 on bad credentials (single generic message — no user-enumeration leak).
  - Updates `users.last_login_at = NOW()`.
- `POST /api/v1/auth/guest`
  - Body: none.
  - Creates a `users` row with `is_guest=true`, no email, no password.
  - Creates a sibling `players` row with `display_name = "Guest-<6 hex>"`.
  - 200: `{ "accessToken": "...", "playerId": "uuid" }`.

### Router wiring (`internal/api/router.go`)

- Remove `/api/v1/auth`, `/api/v1/auth/login`, `/api/v1/auth/guest` from the stub list.
- Register the three real handlers at:
  - `POST /api/v1/auth/register`
  - `POST /api/v1/auth/login`
  - `POST /api/v1/auth/guest`
- Add a `protected` helper used in Spec 18: `protected := middleware.RequireAuth(log)` then wrap handlers.

### Repository layer (`internal/auth/repo.go`)

Thin wrapper around `*database.Pool`:

- `CreateUserWithPlayer(ctx, email, passwordHash, displayName string) (userID, playerID uuid.UUID, err error)`
- `LookupUserByEmail(ctx, email string) (userID, passwordHash uuid.UUID, err error)`
- `CreateGuest(ctx) (userID, playerID uuid.UUID, err error)`
- `PlayerByID(ctx, id uuid.UUID) (*Player, error)`

Use plain `database/sql`-style queries via `pgxpool.Pool.Query` / `Exec`. No ORM.

### Service layer (`internal/auth/service.go`)

Composes repo + password + JWT:

- `Register(ctx, email, password, displayName) (token, playerID, error)`
- `Login(ctx, email, password) (token, playerID, error)`
- `Guest(ctx) (token, playerID, error)`

### Validation helpers

- Email: simple regex `^[^\s@]+@[^\s@]+\.[^\s@]+$`.
- Password: min 8 chars, max 128 chars.
- Display name: 1–32 chars, trim whitespace.

### Tests (stdlib only)

- `internal/auth/password_test.go` — hash/verify round-trip; wrong password fails.
- `internal/auth/jwt_test.go` — issue + parse happy path; expired token rejected; wrong signature rejected.
- `internal/auth/middleware_test.go` — missing header → 401; valid token → 200; expired → 401.
- `internal/auth/handlers_test.go` — register / login / guest happy paths and the main error cases (409, 401). Use `t.Skip` if no live DB.
- `internal/api/middleware/chain_test.go` (extend existing) — auth middleware composes with logging middleware in the right order.

### Logging

- Never log `password`, `password_hash`, raw tokens.
- Log: register/login/guest outcomes with `user_id`, `player_id`, `remote_addr`, outcome (success/failure reason).

### Dependencies

Add (single): `github.com/golang-jwt/jwt/v5` and `golang.org/x/crypto/bcrypt`.

## Deliverables

- [ ] `migrations/000002_auth.up.sql` and `…down.sql`
- [ ] `internal/auth/password.go` + `password_test.go`
- [ ] `internal/auth/jwt.go` + `jwt_test.go`
- [ ] `internal/auth/middleware.go` + `middleware_test.go`
- [ ] `internal/auth/repo.go`
- [ ] `internal/auth/service.go`
- [ ] `internal/auth/errors.go`
- [ ] `internal/auth/handlers.go` + `handlers_test.go`
- [ ] Router wired to real handlers
- [ ] `.env.example` updated with new vars
- [ ] `internal/config/config.go` updated to parse them
- [ ] README "Auth" section updated

## Acceptance Criteria

- `go build ./...` exits 0.
- `go test ./...` exits 0 (incl. new auth tests; DB-dependent ones skipped cleanly without a live DB).
- `docker compose build && docker compose up -d` works without removing data (the new migration runs forward).
- Live:
  - `POST /api/v1/auth/register` with valid body returns 200 + token; second call with same email returns 409.
  - `POST /api/v1/auth/login` with right creds returns 200 + token; with wrong password returns 401 with generic message.
  - `POST /api/v1/auth/guest` with no body returns 200 + token.
  - `curl -H "Authorization: Bearer <token>" http://localhost:8081/api/v1/players/me` (a protected route registered in Spec 18) returns 200. Without the header returns 401.
- Passwords are stored as bcrypt hashes; the SQL `SELECT password_hash FROM users LIMIT 1` never shows plaintext.
- Logs do not contain passwords or raw tokens.

## Out of Scope

- Refresh tokens, password reset, email verification → future milestone.
- "Remember me", rate limiting per IP → future milestone.
- OAuth / social login → future milestone.
- Spec 18 domain handlers (next block).

## Notes

- Use UUID type from `github.com/google/uuid` (already in dependency tree via pgx).
- Keep this spec focused on auth plumbing. The next spec wires the same middleware onto the protected domain handlers.