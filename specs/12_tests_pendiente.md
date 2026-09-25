# Spec 12 — Tests

**Status:** pendiente
**Section:** 27
**Depends on:** Specs 02, 05, 08
**Blocks:** Spec 17

## Objective

Provide minimum-viable Go unit tests so that `go test ./...` exits 0 and the critical paths have coverage.

## Scope

- `internal/config/config_test.go`:
  - Default values loaded when only required env vars set
  - Missing required var returns error
- `internal/api/handlers/health_test.go`:
  - Health handler returns 200 and correct JSON shape
  - Test with and without DB (use `sqlmock` or skip if DB unavailable)
- `internal/networking/udp_server_test.go`:
  - `HandlePacket([]byte("PING"))` returns `[]byte("PONG")`
  - Unknown payload returns nothing / empty
- `cmd/udp-client/main_test.go` (optional): test any extracted helpers.

### Testing constraints
- Use only Go's standard `testing` package + `httptest`.
- For DB tests, may use `github.com/DATA-DOG/go-sqlmock` (single tiny dependency) OR `t.Skip` when no DB available. Justify choice in code comments.
- Tests must run in any environment; don't hard-depend on Docker running.

## Deliverables

- [ ] All test files listed above
- [ ] `go test ./...` exits 0

## Acceptance Criteria

- `go test ./...` passes locally and in any CI environment without Docker
- No flaky tests
- Coverage of at least the three critical paths (health, config, PING/PONG)

## Out of Scope

- Integration tests against a live Postgres
- Load tests
- End-to-end Docker-based tests (those are in Spec 17)