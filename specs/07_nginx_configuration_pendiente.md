# Spec 07 — Nginx Configuration

**Status:** pendiente
**Section:** 16
**Depends on:** Specs 05, 06
**Blocks:** Spec 15

## Objective

Configure Nginx as a reverse proxy that exposes `/api/` to the Go REST API on the internal Docker network.

## Scope

- `nginx/nginx.conf` with:
  - `worker_processes auto`
  - `events { worker_connections 1024; }`
  - `http` block listening on port 80
  - `upstream backend_api { server backend-api:8080; }`
  - `server` block with `location /api/ { proxy_pass http://backend_api; proxy_set_header Host $host; proxy_set_header X-Real-IP $remote_addr; proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for; proxy_set_header X-Forwarded-Proto $scheme; }`
- Add common proxy timeouts and buffering settings.
- Comment out TLS server block but leave a clearly marked place for it.
- No TLS certificates required for local dev.

## Deliverables

- [ ] `nginx/nginx.conf`

## Acceptance Criteria

- `docker compose config` (Spec 15) parses the Nginx service block
- Manual: starting Nginx + a mock backend on `backend-api:8080` and `curl http://localhost/api/v1/health` returns 200 from the backend
- No TLS-required directives active

## Out of Scope

- Certbot / Let's Encrypt (future)
- Rate limiting, gzip tuning, etc. (future)