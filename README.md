# Racing Game Backend (MVP)

Local-only backend for a multiplayer mobile racing game. The mobile client
(Unity, iOS / Android) is built separately; this repo is the Go-based
REST API, the UDP real-time game server, Postgres persistence, and a
Docker Compose stack that brings it all up locally.

> Status: **MVP backend foundation** — REST API + UDP PING/PONG +
> Postgres + Nginx + Docker Compose are running. Auth, matchmaking, race
> loop, leaderboards, and the Unity contracts are the next milestones.

---

## Architecture

```
                UNITY MOBILE
               iOS / Android
                     |
         +-----------+-----------+
         |                       |
     HTTPS / REST             UDP
     JSON                     Protobuf (future)
         |                       |
         v                       v
    +---------+             +-------------+
    |  NGINX  |             | GAME SERVER |
    |  :80    |             |    GO       |
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

* The REST API and the Game Server are two separate Go binaries built
  from the same module (`cmd/api` and `cmd/game-server`).
* Nginx terminates HTTP and forwards `/api/*` to the REST API.
* The Game Server speaks UDP directly to Unity; today it only handles
  a text `PING → PONG`. The protocol swap to Protobuf is a single
  function change inside `internal/networking/udp_server.go`.
* PostgreSQL is internal-only (not exposed to the host).

---

## Tech stack

| Layer        | Choice                                          |
|--------------|-------------------------------------------------|
| Language     | Go 1.27                                         |
| REST API     | `net/http` + `http.ServeMux` (no third-party router) |
| Database     | PostgreSQL 16                                   |
| DB driver    | `pgx` v5 with `pgxpool`                         |
| Migrations   | `golang-migrate/migrate` (CLI + Docker image)   |
| Real-time    | UDP via `net.UDPConn` + Protobuf (proto3)       |
| API contract | JSON + OpenAPI (planned for next milestone)      |
| Auth         | JWT architecture (planned)                       |
| Logging      | `log/slog` (JSON handler)                       |
| Tests        | Go `testing` + `httptest`                       |
| Reverse proxy| Nginx 1.27                                      |
| Orchestration| Docker Compose v2                                |
| Build images | Multi-stage, `gcr.io/distroless/static-debian12:nonroot` |

---

## Directory structure

```
.
├── cmd/
│   ├── api/            # REST API entry point
│   ├── game-server/    # UDP game server entry point
│   └── udp-client/     # developer CLI: sends PING, prints PONG
├── internal/
│   ├── api/            # HTTP router, handlers, middleware
│   │   ├── handlers/   # /health, stub handlers, future domain handlers
│   │   └── middleware/ # slog request logger, Chain helper
│   ├── auth/           # (planned) JWT middleware, login, guest flow
│   ├── cars/           # (planned) car catalog handlers
│   ├── config/         # env-driven configuration loader
│   ├── database/       # pgxpool wrapper (New, Close, HealthCheck)
│   ├── garage/         # (planned) garage handlers
│   ├── matchmaking/    # (planned) matchmaking handlers
│   ├── networking/     # UDP server primitive + HandlePacket (text today)
│   ├── player/         # (planned) player profile handlers
│   ├── race/           # (planned) race metadata handlers
│   └── tracks/         # (planned) track catalog handlers
├── protocol/
│   ├── game.proto      # proto3 contract: Vector3, PlayerInput, WorldSnapshot, …
│   └── gamepb/         # generated Go bindings (committed)
├── migrations/         # SQL migrations (000001_init.{up,down}.sql)
├── nginx/              # nginx.conf reverse-proxy config
├── scripts/            # migrate.sh, gen-proto.sh
├── specs/              # implementation specs (16 numbered files)
├── docs/               # (planned) OpenAPI spec
├── tests/              # (planned) cross-package integration tests
├── Dockerfile.api          # multi-stage build for the REST API
├── Dockerfile.game-server  # multi-stage build for the UDP game server
├── docker-compose.yml      # 5-service local stack
├── .env.example            # template for environment variables
├── .gitignore
├── .dockerignore
├── go.mod / go.sum
└── README.md (this file)
```

---

## Prerequisites

* **Docker** 24+ with **Compose v2**
* **Go** 1.27+ (only needed for local builds outside Docker or for
  running `go test ./...` directly)
* **protoc** 3+ (only needed to regenerate `protocol/gamepb/game.pb.go`)

The Docker stack does **not** require Go or protoc on the host; the
binaries are built inside the multi-stage Dockerfiles.

---

## Environment variables

Copy `.env.example` to `.env` and fill in real values. The file is
ignored by git — never commit a real `.env`.

| Variable             | Required | Default          | Notes                                     |
|----------------------|----------|------------------|-------------------------------------------|
| `APP_ENV`            | no       | `development`    | `development` skips secret requirement    |
| `LOG_LEVEL`          | no       | `info`           | `debug` \| `info` \| `warn` \| `error`    |
| `API_PORT`           | no       | `8080`           | Container-internal; not published to host |
| `GAME_SERVER_PORT`   | no       | `7000`           | Published to host `7000/udp`              |
| `POSTGRES_HOST`      | no       | `postgres`       | Docker service name                       |
| `POSTGRES_PORT`      | no       | `5432`           |                                           |
| `POSTGRES_DB`        | no       | `racing_game`    |                                           |
| `POSTGRES_USER`      | no       | `racing_user`    |                                           |
| `POSTGRES_PASSWORD`  | **yes**  | —                | Required outside development              |
| `POSTGRES_SSLMODE`   | no       | `disable`        | `disable` for local dev                   |
| `JWT_SECRET`         | **yes**  | —                | Required outside development              |

---

## Build (outside Docker)

```bash
go mod tidy
go build ./cmd/api
go build ./cmd/game-server
go build ./cmd/udp-client
```

---

## Start the stack

```bash
cp .env.example .env       # edit secrets first if you want
docker compose up --build -d
docker compose ps
```

You should see:

```
NAME                 IMAGE                                 STATUS
racing-backend-api   racing-game-backend/api:dev           Up
racing-game-server   racing-game-backend/game-server:dev   Up
racing-nginx         nginx:1.27-alpine                     Up
racing-postgres      postgres:16-alpine                    Up (healthy)
racing-migrate       migrate/migrate:v4.18.1               Exited (0)
```

> The migrate container is one-shot and exits 0 after applying
> migrations. `backend-api` waits for it via
> `service_completed_successfully`.

---

## Stop the stack

```bash
docker compose down            # keep postgres_data volume
docker compose down -v         # also delete the volume (fresh DB)
```

---

## Inspect

```bash
docker compose ps                        # service status
docker compose logs -f backend-api       # follow logs
docker compose logs -f game-server
docker compose logs -f nginx
docker compose exec postgres psql -U racing_user -d racing_game
```

---

## Test the API (via Nginx)

```bash
curl http://localhost:8081/api/v1/health
# {"status":"ok","service":"backend-api","database":"ok"}

curl -i http://localhost:8081/api/v1/auth
# HTTP/1.1 501 Not Implemented
# {"error":"not implemented","path":"/api/v1/auth"}
```

The host port is **8081** (see Troubleshooting if you expected 80).

---

## Test UDP

```bash
go run ./cmd/udp-client --host=localhost --port=7000
# Sent: PING → 127.0.0.1:7000
# Received: PONG
```

The game-server logs each packet on the host:

```bash
docker compose logs -f game-server
```

---

## Run Go tests

```bash
go test ./...
```

Tests cover:

* `internal/config` — env loading, defaults, DSN composition, secret enforcement
* `internal/database` — nil guards, optional live integration (skipped unless `INTEGRATION_POSTGRES_URL` is set)
* `internal/api/handlers` — `/health` JSON shape, stub 501
* `internal/api/middleware` — request logging, middleware ordering
* `internal/api` — full router dispatch (health 200, stubs 501, unknown 404)
* `internal/networking` — `HandlePacket` cases, end-to-end UDP loopback PING/PONG, idempotent close
* `protocol/gamepb` — protobuf round-trip

---

## Migrations

Migrations run automatically on `docker compose up` via the one-shot
`migrate` service. To run them manually:

```bash
go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
POSTGRES_HOST=localhost POSTGRES_PORT=5432 \
POSTGRES_DB=racing_game POSTGRES_USER=racing_user \
POSTGRES_PASSWORD=dev_password ./scripts/migrate.sh up
```

Migration files live in `migrations/` with timestamped names. Adding a
new migration:

```bash
# create files with timestamp prefix
$migrate create -ext sql -dir migrations -seq add_some_table
# edit the .up.sql and .down.sql, commit
docker compose up -d migrate   # one-shot re-run
```

---

## Reset the local database

```bash
docker compose down -v
docker compose up -d
```

The `-v` flag removes the `postgres_data` named volume so the next
start runs migrations against a clean DB.

---

## Protocol Buffers

The wire contract for the Game Server is `protocol/game.proto` (proto3,
package `racing.game.v1`).

### Regenerate Go bindings

```bash
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
./scripts/gen-proto.sh
# → regenerates protocol/gamepb/game.pb.go
```

The generated file is **committed** so contributors don't need `protoc`
locally to build.

### Generate C# bindings for Unity

In your Unity project (or via a CI step):

```bash
# using grpc.tools (includes protoc-gen-csharp)
protoc \
  --csharp_out=Assets/Scripts/Generated \
  --csharp_opt=file_extension=.g.cs \
  protocol/game.proto
```

The Unity developer copies `protocol/game.proto` into the Unity project
and runs the command above.

---

## OpenAPI

A future milestone will add `docs/openapi.yaml` describing every REST
endpoint as the contract for the Unity client. The router already
exposes 501 stubs for all future paths so the URL space is stable.

---

## Ports

| Service        | Container | Host (docker-compose) |
|----------------|-----------|-----------------------|
| Nginx          | 80 TCP    | **8081** TCP          |
| backend-api    | 8080 TCP  | internal only         |
| game-server    | 7000 UDP  | **7000** UDP          |
| postgres       | 5432 TCP  | internal only         |

---

## Troubleshooting

### Port 80 already in use

A system nginx (Homebrew, MAMP, etc.) may be listening on host port
80. The compose file maps nginx to **host 8081** to avoid the conflict.
If you really want port 80, stop the system service first:

```bash
sudo brew services stop nginx   # or your service manager
```

then edit `docker-compose.yml` and change `"8081:80"` back to `"80:80"`.

### `postgres` host not found from your shell

The hostname `postgres` only resolves inside the Docker network. From
your host, use `localhost` (port is internal — only Docker can reach
it; use `docker compose exec postgres psql ...` instead).

### UDP doesn't work on macOS firewall

macOS may prompt the first time an application binds a UDP port below
1024 (we don't, we're on 7000) or accepts inbound UDP. Approve the
prompt when it appears, or explicitly allow the binary:

```
System Settings → Network → Firewall → Allow incoming connections for Docker
```

### `docker compose up` hangs on `migrate`

Postgres is unhealthy. Check logs:

```bash
docker compose logs postgres
```

The most common cause is a missing or wrong `POSTGRES_PASSWORD` in
`.env`.

### Generated `protocol/gamepb/game.pb.go` is stale after editing `.proto`

Regenerate it locally and commit:

```bash
./scripts/gen-proto.sh
```

CI should not regenerate — it uses the committed file.

### `go test ./...` fails because of network/DB

The `internal/database` suite includes an opt-in integration test that
needs a real Postgres:

```bash
INTEGRATION_POSTGRES_URL=postgres://user:pass@localhost:5432/db go test ./...
```

Without that variable the test is skipped, not failed.

---

## Next steps (next milestone)

* JWT-based `/api/v1/auth/login` and `/api/v1/auth/guest`
* Domain handlers for players, cars, garage, tracks
* Matchmaking endpoint that returns `{ matchId, gameServerHost, gameServerPort, gameToken }`
* Protobuf framing on the Game Server (replace `HandlePacket`)
* Per-race goroutines + authoritative tick loop
* Leaderboards and race history persisted to Postgres

See `specs/` for the per-spec breakdown of the **foundation** milestone
that this README documents.