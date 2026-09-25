# Spec 09 — Game Server Dockerfile

**Status:** DONE
**Section:** 19
**Depends on:** Spec 08
**Blocks:** Spec 15

## Objective

Create an optimized, multi-stage `Dockerfile.game-server` that produces a small runtime image for the UDP Game Server binary.

## Scope

### Build stage
- Base: `golang:1.22-alpine` (or current LTS)
- `CGO_ENABLED=0`
- `go mod download`
- Build: `go build -trimpath -ldflags="-s -w" -o /out/game-server ./cmd/game-server`

### Runtime stage
- Base: `gcr.io/distroless/static-debian:nonroot` (preferred for true minimal image) OR `alpine:latest`
- Copy binary
- `EXPOSE 7000/udp`
- Run as non-root
- `ENTRYPOINT ["/game-server"]`

### Notes
- Distroless has no shell, so for distroless images do NOT add a `HEALTHCHECK` line — handle healthchecks via docker-compose spec 15 (or accept that UDP has no healthcheck).
- If using alpine, may add a simple UDP liveness check in docker-compose.

## Deliverables

- [ ] `Dockerfile.game-server`
- [ ] `.dockerignore` already shared with Spec 06

## Acceptance Criteria

- `docker build -f Dockerfile.game-server -t racing-gs:test .` succeeds
- Image runs without error
- Image size is reasonable (< 30 MB)

## Out of Scope

- docker-compose orchestration (Spec 15)