# Spec 01 — Project Setup & Go Module Init

**Status:** DONE
**Section:** 36 (Steps 1–3)
**Depends on:** —
**Blocks:** all other specs

## Objective

Inspect the existing repository (currently empty), initialize a clean Go module, and create the canonical directory structure for the racing-game-backend monorepo.

## Scope

- Verify the working directory state (no useful existing work to preserve).
- Initialize a Go module with a clean module name (suggested: `github.com/gustavo/racing-game-backend` — adjust if user prefers).
- Create the directory tree:
  ```
  cmd/api/
  cmd/game-server/
  cmd/udp-client/   (created later in Spec 11)
  internal/config/
  internal/database/
  internal/auth/
  internal/player/
  internal/cars/
  internal/garage/
  internal/tracks/
  internal/matchmaking/
  internal/race/
  internal/networking/
  protocol/
  migrations/
  nginx/
  scripts/
  tests/
  docs/
  ```
- Add `.gitignore` (Go, secrets, build artifacts).
- Both `cmd/api` and `cmd/game-server` must compile independently.

## Deliverables

- [ ] `go.mod` exists with module name + Go 1.22+ (or current LTS)
- [ ] Directory tree created
- [ ] `.gitignore` committed (no `.env`, no `*.pb.go` initial binaries, no `vendor/`)
- [ ] `go build ./cmd/api` succeeds (stub main)
- [ ] `go build ./cmd/game-server` succeeds (stub main)
- [ ] `go mod tidy` succeeds

## Acceptance Criteria

- `go mod tidy` exits 0
- `go build ./cmd/api` exits 0
- `go build ./cmd/game-server` exits 0

## Out of Scope

- Real handler logic (Spec 05)
- Real UDP logic (Spec 08)
- Migrations (Spec 04)
- Docker (Spec 15)

## Notes

This is the foundation. If any future spec needs to create a new top-level directory, prefer `internal/<name>/` for non-imported packages and `pkg/` only if explicitly exported.