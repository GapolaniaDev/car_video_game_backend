# Spec 11 — Protocol Buffers (`protocol/game.proto`)

**Status:** pendiente
**Section:** 11
**Depends on:** Spec 01
**Blocks:** Spec 17 (validation only)

## Objective

Define the real-time wire contract between Unity and the Game Server using Protocol Buffers (proto3). Make the file compile with `protoc` if tooling is available; document how to generate Go and C# code.

## Scope

- `protocol/game.proto`:
  - `syntax = "proto3";`
  - `option go_package = "github.com/gustavo/racing-game-backend/protocol/gamepb";`
  - Messages:
    - `Vector3 { float x = 1; float y = 2; float z = 3; }`
    - `Quaternion { float x = 1; float y = 2; float z = 3; float w = 4; }`
    - `PlayerInput { uint32 sequence = 1; float throttle = 2; float brake = 3; float steering = 4; }`
    - `CarState { string player_id = 1; Vector3 position = 2; Quaternion rotation = 3; Vector3 velocity = 4; }`
    - `WorldSnapshot { uint64 tick = 1; repeated CarState cars = 2; }`
    - `JoinRaceRequest { string match_id = 1; string player_id = 2; string game_token = 3; }`
    - `JoinRaceResponse { bool ok = 1; string error = 2; string race_id = 3; uint64 initial_tick = 4; }`
    - `RaceStarting { uint64 start_tick = 1; uint32 countdown_ms = 2; }`
    - `RaceStarted { uint64 start_tick = 1; }`
    - `CheckpointPassed { string player_id = 1; uint32 checkpoint_index = 2; uint64 tick = 3; }`
    - `LapCompleted { string player_id = 1; uint32 lap = 2; uint32 lap_time_ms = 3; }`
    - `RaceFinished { string race_id = 1; repeated RaceResult results = 2; }`
    - `RaceResult { string player_id = 1; uint32 position = 2; uint32 total_time_ms = 3; uint32 best_lap_ms = 4; }`
    - `PlayerDisconnected { string player_id = 1; string reason = 2; }`

- README section documenting:
  - How to install `protoc`, `protoc-gen-go`
  - Command: `protoc --go_out=. --go_opt=paths=source_relative protocol/game.proto`
  - For Unity / C#: use `grpc.tools` or `protoc-gen-csharp` with the same `.proto` file

### Generated code policy
- Do NOT commit generated `.pb.go` files to MVP if the toolchain is unstable on the developer machine — but DO commit them if generation works, so the build doesn't require `protoc` on every developer's machine.
- Decision: commit generated Go code under `protocol/gamepb/`. Document regeneration in README.

## Deliverables

- [ ] `protocol/game.proto`
- [ ] Generated `protocol/gamepb/game.pb.go` (committed)
- [ ] README "Protocol Buffers" section

## Acceptance Criteria

- `protoc --version` works OR a documented note explains the file is hand-validated (syntax, package, message shapes)
- All 13 messages present
- Generated Go code compiles (`go build ./...`)
- Message names match the spec exactly

## Out of Scope

- gRPC service definitions (not needed for raw UDP framing)
- Field options like `deprecated` or custom validators