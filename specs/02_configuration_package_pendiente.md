# Spec 02 — Configuration Package

**Status:** pendiente
**Section:** 21
**Depends on:** Spec 01
**Blocks:** Specs 03, 05, 08, 15

## Objective

Implement a centralized, env-driven configuration loader used by both Go executables. No hardcoded production values anywhere.

## Scope

- Create `internal/config/config.go` exposing a `Config` struct loaded from environment variables.
- Provide a `Load() (*Config, error)` function.
- Provide a `MustLoad() *Config` for entry-point convenience.
- Variables (minimum):
  - `APP_ENV` (default: `development`)
  - `API_PORT` (default: `8080`)
  - `GAME_SERVER_PORT` (default: `7000`)
  - `POSTGRES_HOST` (default: `postgres`)
  - `POSTGRES_PORT` (default: `5432`)
  - `POSTGRES_DB` (default: `racing_game`)
  - `POSTGRES_USER` (default: `racing_user`)
  - `POSTGRES_PASSWORD` (required, no default)
  - `POSTGRES_SSLMODE` (default: `disable` in dev)
  - `JWT_SECRET` (required, no default)
  - `LOG_LEVEL` (default: `debug`, accepted: `debug|info|warn|error`)
- Use stdlib only (`os`, `strconv`, `fmt`). Do not add a config framework.
- Return clear errors when required vars are missing (e.g., `JWT_SECRET` in production-ish envs).
- Create `.env.example` matching the variables above with placeholder values.
- Add a small unit test confirming defaults + required-var errors.

## Deliverables

- [ ] `internal/config/config.go`
- [ ] `.env.example` at repo root
- [ ] `internal/config/config_test.go`

## Acceptance Criteria

- `go test ./internal/config/...` passes
- Loading with no env vars fails when `JWT_SECRET` and `POSTGRES_PASSWORD` are missing (in non-dev mode)
- Loading with full env vars succeeds and returns a populated `Config`
- No secret values appear in log output

## Out of Scope

- Live `.env` loading (handled by Docker Compose `env_file`)
- Hot-reload of config