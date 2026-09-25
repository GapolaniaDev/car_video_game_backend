# Spec 10 — UDP Test Client

**Status:** pendiente
**Section:** 26
**Depends on:** Spec 08
**Blocks:** Spec 17

## Objective

Provide a small Go utility for testing the Game Server's PING/PONG behavior end-to-end without depending on platform-specific netcat.

## Scope

- `cmd/udp-client/main.go`:
  - Resolve UDP target from flags or env: defaults `localhost:7000`
  - Dial UDP: `net.DialUDP("udp", nil, addr)`
  - Send `[]byte("PING\n")` (or just `"PING"`)
  - Set read deadline (e.g., 3 seconds)
  - Read response, print it
  - If response == `"PONG"` → exit 0
  - Else or on timeout → print useful error and exit 1
- Flags via stdlib `flag` package:
  - `--host` (default `localhost`)
  - `--port` (default `7000`)
  - `--timeout` (default `3s`)

## Deliverables

- [ ] `cmd/udp-client/main.go`
- [ ] `cmd/udp-client/main_test.go` (test the response parser function if extractable)

## Acceptance Criteria

- `go build ./cmd/udp-client` exits 0
- `go run ./cmd/udp-client` against a running game server prints:
  ```
  Sent: PING
  Received: PONG
  ```
  and exits 0
- Against a closed port: prints a clear error and exits non-zero within timeout

## Out of Scope

- Protobuf-based client (future)
- Multiple-message exchanges