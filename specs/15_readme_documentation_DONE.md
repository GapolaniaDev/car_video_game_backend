# Spec 15 — README Documentation

**Status:** DONE
**Section:** 35
**Depends on:** All previous specs
**Blocks:** Spec 17

## Objective

Produce a `README.md` that lets any developer get the whole stack running locally with minimal prior context, and clearly documents how Unity will consume the contracts.

## Scope

Sections required:

1. **Project overview** — one paragraph: multiplayer racing game backend, MVP.
2. **Architecture** — diagram (ASCII) showing Unity → Nginx → REST API → Postgres, Unity → UDP → Game Server.
3. **Tech stack** — Go, PostgreSQL, pgx, Protocol Buffers, Nginx, Docker Compose, slog.
4. **Directory structure** — tree with short descriptions.
5. **Prerequisites** — Go 1.22+, Docker, Docker Compose, optional `protoc`.
6. **Environment variables** — table of every var in `.env.example` with description.
7. **Build** — `go mod tidy`, `go build ./cmd/api`, `go build ./cmd/game-server`.
8. **Start the stack** — `docker compose up --build -d`.
9. **Stop the stack** — `docker compose down`.
10. **Inspect** — `docker compose ps`, `docker compose logs -f <service>`.
11. **Test REST** — `curl http://localhost/api/v1/health`.
12. **Test UDP** — `go run ./cmd/udp-client`.
13. **Run Go tests** — `go test ./...`.
14. **Migrations** — how they run automatically via the `migrate` service, plus manual `goose`/`migrate` commands.
15. **Reset local database** — `docker compose down -v` (destroys volume).
16. **Protocol Buffers** — file location, how to regenerate Go code, how Unity generates C# (document protoc + `grpc.tools` for Unity, or pointing Unity to the .proto via a copy).
17. **Ports** — table: 80 (nginx), 7000/udp (game server), 5432 internal only (postgres), 8080 internal only (api).
19. **Troubleshooting** — common issues: port already in use, migrations failing because Postgres not healthy, UDP not reachable on macOS due to firewall.
20. **Next steps** — pointer to the next milestone (auth, matchmaking, race loop).

## Deliverables

- [ ] `README.md`

## Acceptance Criteria

- A new developer with only Docker + Go installed can follow README and bring up the stack in < 10 minutes
- README explicitly mentions the future Unity integration

## Out of Scope

- Generated API reference (OpenAPI doc referenced separately)
- Deployment guide