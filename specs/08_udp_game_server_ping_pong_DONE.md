# Spec 08 — UDP Game Server (PING → PONG)

**Status:** DONE
**Section:** 8, 22, 23
**Depends on:** Spec 02
**Blocks:** Specs 10, 11, 14

## Objective

Implement a second Go executable that listens on UDP `:7000`, responds to `PING` with `PONG`, and is structured so the text protocol can later be replaced by Protocol Buffers.

## Scope

### Server
- `cmd/game-server/main.go`:
  - Load config via `internal/config`
  - Open UDP socket: `net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4zero, Port: cfg.GameServerPort})`
  - Read loop: handle each datagram
  - **MVP behavior:** if payload is `"PING"` (trim whitespace), reply `"PONG"` to the same remote address
  - Log: startup, listening address, each packet (type, remote addr, payload length), errors, shutdown
  - Graceful shutdown on `SIGINT` / `SIGTERM`: stop read loop, close socket
  - Structured logs via `slog`; no payload content logged (length only) to keep noise down and avoid future secret leakage

### Code structure
- `internal/networking/udp_server.go` — small abstraction `UDPConn` wrapper with a `ReadLoop` and `Reply`. This is where Protobuf will plug in later.
- Keep the protocol swap-out clean: a single `HandlePacket(payload []byte, from *net.UDPAddr) []byte` function that returns a reply. Today: string compare. Future: Protobuf unmarshal/marshal.

## Deliverables

- [ ] `cmd/game-server/main.go`
- [ ] `internal/networking/udp_server.go`
- [ ] Unit test for `HandlePacket`: `[]byte("PING")` → `[]byte("PONG")`; unknown payload → `nil` or `[]byte{}`

## Acceptance Criteria

- `go build ./cmd/game-server` exits 0
- `go test ./internal/networking/...` passes
- When run locally on `:7000`, sending a UDP `PING` from `nc -u localhost 7000` (or the future client) returns `PONG`
- Logs show startup, listening address, and structured per-packet entries

## Out of Scope

- Protobuf framing (Spec 12)
- Race loops, player sessions, matchmaking tokens
- TCP fallback