# Spec 14 — Docker Compose Orchestration

**Status:** pendiente
**Section:** 17, 18, 20, 25
**Depends on:** Specs 03, 04, 06, 07, 09, 13
**Blocks:** Spec 17

## Objective

Wire everything together into a single `docker-compose.yml` that brings up Nginx + REST API + Game Server + Postgres on a shared internal network, with correct port exposure and persistent Postgres data.

## Scope

### Top-level
- `name: racing-game-backend`
- `networks: { racing-network: { driver: bridge } }`
- `volumes: { postgres_data: {} }`

### Services

**postgres**
- image: `postgres:16-alpine`
- env: `POSTGRES_DB`, `POSTGRES_USER`, `POSTGRES_PASSWORD` (from `.env` via `env_file`)
- volume: `postgres_data:/var/lib/postgresql/data`
- healthcheck: `pg_isready -U $POSTGRES_USER -d $POSTGRES_DB`
- networks: `racing-network`
- no `ports:` (internal only)

**backend-api**
- build: `context: .` `dockerfile: Dockerfile.api`
- env_file: `.env`
- environment: also forward `POSTGRES_HOST=postgres` etc.
- depends_on: `postgres: { condition: service_healthy }`
- networks: `racing-network`
- (Optional) `expose: ["8080"]` for debugging — not published to host
- healthcheck: `wget -qO- http://localhost:8080/api/v1/health || exit 1` (interval 10s, retries 3, start_period 15s) — adjust based on whether distroless has wget (it does NOT). Use a tiny sidecar or skip; if skipped, document.
- **Migrations:** a one-shot `migrate` step runs on startup before the API comes up. Options:
  - Separate `migrate` service that depends on `postgres: service_healthy`, runs the migration binary, then exits. `backend-api` depends_on `migrate: service_completed_successfully`.
  - OR include a small `entrypoint.sh` in the API image that runs migrations and execs the binary. Choose **the separate service approach** to keep the API image minimal.

**migrate** (one-shot)
- same build context as API
- command: runs the migrator (`goose` or `migrate`) against `$POSTGRES_HOST:5432`
- depends_on: `postgres: { condition: service_healthy }`

**game-server**
- build: `context: .` `dockerfile: Dockerfile.game-server`
- env_file: `.env`
- networks: `racing-network`
- ports: `"7000:7000/udp"` (host UDP exposure)
- (no healthcheck — UDP)

**nginx**
- image: `nginx:1.27-alpine`
- volumes: `./nginx/nginx.conf:/etc/nginx/nginx.conf:ro`
- ports: `"80:80"`
- depends_on: `backend-api: { condition: service_started }` (or healthy if API healthcheck works)
- networks: `racing-network`

## Deliverables

- [ ] `docker-compose.yml`
- [ ] `migrations/` already populated (Spec 04)
- [ ] Decision documented: distroless + API healthcheck approach

## Acceptance Criteria

- `docker compose config` exits 0 and prints valid config
- `docker compose build` succeeds
- `docker compose up -d` starts all 4 services + the migrate step
- `docker compose ps` shows everything `running` (or `exited 0` for migrate)
- `docker compose restart` preserves Postgres data

## Out of Scope

- Production deployment manifests
- TLS / certs
- Reverse proxy on the host