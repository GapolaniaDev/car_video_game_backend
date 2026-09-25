# Multiplayer Racing Game Backend MVP — Implementation Prompt

## Context

Build the initial backend infrastructure for a multiplayer mobile racing game.

- Backend language: **Go**
- Mobile client (separate): Unity / C# for iOS and Android
- Goal: Clean, working, Dockerized backend foundation that runs locally today and can later be deployed to a Linux VPS with minimal architectural changes.

This is an MVP. Prioritize:
- simplicity
- clean architecture
- maintainability
- testability
- networking correctness
- portability
- clear contracts between Unity and backend

Do NOT overengineer.

---

## Architecture

```
                UNITY MOBILE
               iOS / Android
                     |
         +-----------+-----------+
         |                       |
     HTTPS / REST             UDP
     JSON                     Protobuf
         |                       |
         v                       v
    +---------+             +-------------+
    |  NGINX  |             | GAME SERVER |
    | 80/443  |             |    GO       |
    +----+----+             | UDP :7000   |
         |                  +------+------+
         v                         |
    +-------------+                |
    | BACKEND API |                |
    |     GO      |                |
    |    :8080    |                |
    +------+------+                |
           |                       |
           +-----------+-----------+
                       |
                       v
                +-------------+
                | PostgreSQL  |
                |    :5432    |
                +-------------+
```

The REST API and Game Server are two separate Go executables sharing internal Go packages when appropriate.

---

## Tech Stack

- **Backend language:** Go
- **REST API:** Go net/http (lightweight router OK; prefer stdlib)
- **Database:** PostgreSQL
- **PostgreSQL driver:** pgx
- **Migrations:** Lightweight Go-compatible (SQL files preferred)
- **Real-time networking:** Go UDP (`net.UDPConn`)
- **Serialization (real-time):** Protocol Buffers
- **API serialization:** JSON
- **REST API docs:** OpenAPI 3.x
- **Auth:** JWT architecture (don't overbuild)
- **Infra:** Docker + Docker Compose
- **Reverse proxy:** Nginx
- **Logging:** Go `log/slog`
- **Testing:** Go standard `testing` package

Do NOT use: Kubernetes, Redis, Kafka, RabbitMQ, MongoDB, GraphQL, Elasticsearch, OpenSearch, Prometheus, Grafana, service mesh, distributed tracing, multiple databases, multiple repositories, cloud-specific infrastructure, complex microservices.

---

## Project Structure

```
racing-game-backend/
├── cmd/
│   ├── api/main.go
│   └── game-server/main.go
├── internal/
│   ├── config/
│   ├── database/
│   ├── auth/
│   ├── player/
│   ├── cars/
│   ├── garage/
│   ├── tracks/
│   ├── matchmaking/
│   ├── race/
│   └── networking/
├── protocol/game.proto
├── migrations/
├── nginx/nginx.conf
├── scripts/
├── tests/
├── Dockerfile.api
├── Dockerfile.game-server
├── docker-compose.yml
├── go.mod
├── go.sum
├── .env.example
├── .gitignore
└── README.md
```

Single Go monorepo with two executables. Both `cmd/api` and `cmd/game-server` compile independently.

---

## REST API

- Listens internally on **:8080**
- JSON over HTTP, versioned at `/api/v1/`
- Endpoints:
  - `GET /api/v1/health` → `{ "status": "ok", "service": "backend-api" }` (200 OK)
  - Prepared for future: `/api/v1/auth`, `/api/v1/players`, `/api/v1/cars`, `/api/v1/garage`, `/api/v1/tracks`, `/api/v1/matchmaking`, `/api/v1/races`, `/api/v1/leaderboard`

Clean handlers/services/repositories — but no enterprise-style boilerplate.

---

## OpenAPI

- Location: `docs/openapi.yaml`
- OpenAPI 3.x
- Document `GET /api/v1/health` minimum; prepare structure for future endpoints
- Acts as the contract between Go Backend and Unity Client
- Don't generate excessive client/server code

---

## Authentication

- JWT architecture
- Future flow:
  - `POST /api/v1/auth/login` → `{ "accessToken": "...", "playerId": "..." }`
  - `Authorization: Bearer <token>` header
- Future guest: `POST /api/v1/auth/guest`
- Requirements:
  - `JWT_SECRET` from env vars
  - Architecture allows auth middleware
  - No secrets committed

---

## Game Server

- Second Go executable: `cmd/game-server/main.go`
- UDP on **:7000**
- Use `net.ListenUDP`
- MVP: implement **PING → PONG**
- Log: startup, listening address, incoming packets, remote address, errors
- Structure so text protocol can later be replaced with Protobuf
- Do NOT implement racing physics yet

### Future responsibilities (architect only, do not implement now)
- Player connections, match authentication, game token validation
- Race instances, authoritative game state, player inputs, tick loop
- Car positions, rotations, velocity, checkpoints, laps, race timers
- Disconnects, reconnects, race results
- One process eventually manages multiple races
- Goroutines per race in the future

---

## Real-time Model (Authoritative Server)

- Unity sends **PlayerInput** (sequence, throttle, brake, steering)
- Server validates/calculates game state
- Server sends **WorldSnapshot** (tick, cars[])
- Unity is **untrusted** — never design around "my position is X" messages

---

## Protocol Buffers

- `protocol/game.proto`, Proto3
- Compatible with Go and C# generation
- Define: `Vector3`, `Quaternion`, `JoinRaceRequest`, `JoinRaceResponse`, `PlayerInput`, `CarState`, `WorldSnapshot`, `RaceStarting`, `RaceStarted`, `CheckpointPassed`, `LapCompleted`, `RaceFinished`, `RaceResult`, `PlayerDisconnected`
- Include suitable `package` and `go_package` options
- Must compile with `protoc` if tooling is available
- Document how Unity will generate C# classes

---

## PostgreSQL

- Single DB
- pgx from Go
- Hostname inside Docker: `postgres` (NOT localhost)
- Migrations for: `users`, `players`, `cars`, `player_cars`, `tracks`, `races`, `race_players`, `race_results`
- UUID primary keys (unless good reason otherwise)
- Proper FKs, indexes, timestamps, constraints
- Keep schema simple
- **Do NOT** store real-time car positions in PostgreSQL

### Conceptual model
```
users
└── players
    ├── player_cars
    ├── race_players
    └── race_results

cars
└── player_cars

tracks
└── races

races
├── race_players
└── race_results
```

---

## Database Connection

- Reusable package: `internal/database/`
- Connection pooling via `pgxpool`
- API can verify DB connectivity
- Graceful shutdown
- Config from environment variables

---

## Redis

**DO NOT add Redis now.** May be useful later for matchmaking, distributed sessions, lobby state, presence, caching, multi-game-server coordination. Not in Docker Compose yet.

---

## Nginx

- Receives HTTP :80, proxies `/api/` to `backend-api:8080`
- Reachable by Docker service name
- Prepare for HTTPS later
- No TLS certs required for local dev

---

## Docker

Required services:
- nginx
- backend-api
- game-server
- postgres

Internal network: `racing-network`. Use service names for internal DNS.

### Ports
- Nginx: 80 TCP
- Game Server: 7000 UDP
- PostgreSQL: NOT publicly exposed
- Backend API: prefer via Nginx; 8080 may remain internal

External interfaces:
- REST: `http://localhost/api/v1/`
- Real-time: `localhost:7000` UDP

---

## Dockerfiles

- `Dockerfile.api` and `Dockerfile.game-server`
- Multi-stage builds (build stage + minimal runtime)
- Don't ship full Go toolchain in runtime unless necessary

---

## Docker Compose

- `docker-compose.yml`
- Build API + Game Server
- Start PostgreSQL + Nginx
- Internal networking
- Persist PostgreSQL data via named volume `postgres_data`
- Pass env vars
- Expose correct TCP/UDP ports
- Health checks where appropriate
- Data must survive `docker compose restart` and normal recreation

---

## Configuration

- `internal/config/` loads from env vars
- No hardcoded production values
- `.env.example` with:
  ```
  APP_ENV=development
  API_PORT=8080
  GAME_SERVER_PORT=7000
  POSTGRES_HOST=postgres
  POSTGRES_PORT=5432
  POSTGRES_DB=racing_game
  POSTGRES_USER=racing_user
  POSTGRES_PASSWORD=change_me
  JWT_SECRET=change_me
  LOG_LEVEL=debug
  ```
- No real `.env` committed

---

## Logging

- `log/slog`
- API: startup, listening addr, shutdown, DB errors, important HTTP errors
- Game Server: startup, UDP listening addr, incoming packets, remote addr, protocol errors, shutdown
- Don't log secrets/passwords

---

## Graceful Shutdown

Both services handle `SIGINT` and `SIGTERM`:
- REST API stops accepting requests gracefully
- Game Server closes UDP socket
- DB connections close cleanly

---

## Health Checks

- `GET /api/v1/health` may include DB state
- PostgreSQL Docker healthcheck via `pg_isready`
- Docker Compose uses health/dependency config where useful
- Avoid arbitrary long sleep scripts

---

## Local Development

Start: `docker compose up --build -d`
Verify: `docker compose ps` shows all 4 services running
Test REST: `curl http://localhost/api/v1/health`

---

## UDP Test Client

- `cmd/udp-client/main.go`
- Send PING to `localhost:7000`, wait for PONG, print result
- Fail with useful timeout/error if no response

---

## Testing

- Go standard `testing`
- `go test ./...` must succeed
- Minimum tests:
  - API health handler
  - Configuration (where useful)
  - UDP PING/PONG logic (where practical)

---

## Matchmaking — Future API Contract

Prepare for:
```
POST /api/v1/matchmaking/join
→ { "matchId": "uuid", "gameServerHost": "127.0.0.1", "gameServerPort": 7000, "gameToken": "temporary-secure-token" }
```
Unity connects UDP and sends `JoinRaceRequest { match_id, player_id, game_token }`. Game Server validates token. Don't implement complex matchmaking yet.

---

## Expected Future Client Flow

Unity → `POST /auth/login` (JWT) → REST API
Unity → `GET /players/me`, `/garage`, `/tracks` → REST API
Unity → `POST /matchmaking/join` → REST API → `{ matchId, host, port, gameToken }`
Unity → UDP + Protobuf → Game Server → Race
During race: `PlayerInput` → Game Server → `WorldSnapshot` → all players
After race: Game Server → PostgreSQL → `race_results` → REST API exposes results

---

## Real-time Design Rule

REST is for: auth, players, cars, garage, tracks, matchmaking, race history, leaderboards, persistent data
UDP + Protobuf is for: `PlayerInput`, `CarState`, `WorldSnapshot`, race sync, race events

---

## Security Basics

- No committed secrets
- No public PostgreSQL
- Don't trust client race results or positions
- No hardcoded production passwords
- No logged auth secrets
- Authoritative Game Server; untrusted Unity client

---

## Scalability (Today vs Future)

Today: one machine
- Docker → nginx + backend-api + game-server + postgres

Future: split services
- Server 1: REST API
- Server 2-3: Game Servers
- Server 4: PostgreSQL
- Unity contracts (OpenAPI + Protobuf) remain largely unchanged

---

## Development Tunnel Support

Local env must be compatible with dev tunnel exposing REST API. UDP handled separately via VPN/direct connectivity when remote multiplayer testing begins. No tunnel-specific deps in backend.

---

## README

Must explain:
- Architecture, tech stack, directory structure
- Prerequisites (Go version, Docker)
- Environment variables
- Build, start, stop, inspect, logs
- Test REST, test UDP, run Go tests
- Migrations, reset local DB
- Protocol Buffers organization, Unity consumption
- Ports, troubleshooting

---

## Implementation Order

1. Inspect existing repo
2. Initialize/validate Go module
3. Create project directory structure
4. Create configuration package
5. Create PostgreSQL Docker service
6. Create database package using pgxpool
7. Create migrations
8. Create Go REST API
9. Implement `GET /api/v1/health`
10. Create API Dockerfile
11. Create Nginx configuration
12. Verify `http://localhost/api/v1/health`
13. Create Go UDP Game Server
14. Implement PING → PONG
15. Create Game Server Dockerfile
16. Expose `7000/udp`
17. Create Go UDP test client
18. Create `protocol/game.proto`
19. Validate Proto syntax if tooling available
20. Add tests
21. Add graceful shutdown
22. Add health checks
23. Write README
24. Run entire system
25. Fix all errors discovered during verification

---

## Commands That Must Work

- `go mod tidy`
- `go build ./cmd/api`
- `go build ./cmd/game-server`
- `go test ./...`
- `docker compose config`
- `docker compose build`
- `docker compose up -d`
- `docker compose ps`
- `curl http://localhost/api/v1/health`
- `go run ./cmd/udp-client`
- `docker compose logs`

---

## Definition of Done

- Docker Compose starts successfully with nginx, backend-api, game-server, postgres
- `GET http://localhost/api/v1/health` → 200 through Nginx
- UDP Game Server listens on `localhost:7000`
- UDP client: PING → PONG
- PostgreSQL starts healthy; API can connect
- Initial migrations execute successfully
- PostgreSQL data persists across restarts
- `protocol/game.proto` exists
- Go code builds; tests pass
- No real secrets committed
- README has accurate setup instructions

---

## Final Validation

Run and verify:
- `go test ./...`
- `docker compose config`
- `docker compose build`
- `docker compose up -d`
- `docker compose ps`
- REST health endpoint
- UDP PING/PONG
- PostgreSQL connectivity
- `docker compose logs`

Fix compilation, container startup, networking, and migration errors. Don't leave obvious errors.

---

## Final Report

After completion return:
1. Architecture implemented
2. Files created / modified
3. Go packages created
4. Docker services created
5. DB tables/migrations
6. REST endpoints available
7. UDP functionality
8. Protobuf messages
9. Ports exposed
10. Commands executed
11. Test results
12. Docker verification
13. Limitations / unfinished items
14. Recommended next milestone

**Stop after the backend foundation is verified.** Do not continue into complex game functionality.

Immediate objective: working local Go backend environment (Nginx + REST API + UDP Game Server + PostgreSQL + Protobuf + Docker Compose) that can later communicate with Unity and deploy to a Linux VPS.