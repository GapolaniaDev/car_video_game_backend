# Spec 06 — API Dockerfile

**Status:** pendiente
**Section:** 19
**Depends on:** Spec 05
**Blocks:** Spec 15

## Objective

Create an optimized, multi-stage `Dockerfile.api` that produces a small runtime image for the REST API binary.

## Scope

### Build stage
- Base: `golang:1.22-alpine` (or current LTS)
- Set `CGO_ENABLED=0` for a static binary
- Copy `go.mod` + `go.sum` first; run `go mod download`
- Copy the rest of the source
- Build with `go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api`

### Runtime stage
- Base: `gcr.io/distroless/static-debian:nonroot` OR `alpine:latest` (justify choice — prefer distroless)
- Copy the binary
- Run as non-root user
- `EXPOSE 8080`
- `ENTRYPOINT ["/api"]`

### Size / hygiene
- No Go toolchain in runtime
- No `.git`, no `migrations/` if not needed at runtime (migrations run via dedicated step — see Spec 15)
- `.dockerignore` excludes `.git`, `.env`, `*.md` (except README in build context if useful), `vendor/`, `.claude/`

## Deliverables

- [ ] `Dockerfile.api`
- [ ] `.dockerignore`

## Acceptance Criteria

- `docker build -f Dockerfile.api -t racing-api:test .` succeeds
- Image runs: `docker run --rm racing-api:test --help` or just exits cleanly when stopped
- Image size is reasonable (< 30 MB for distroless static)
- Final image has no Go compiler installed

## Out of Scope

- Game Server image (Spec 10)
- docker-compose orchestration (Spec 15)