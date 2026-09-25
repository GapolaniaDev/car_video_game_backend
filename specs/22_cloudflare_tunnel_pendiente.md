# Spec 22 — Cloudflare Tunnel for public access

**Status:** pendiente
**Section:** — (post-MVP; deployment / DevOps)
**Depends on:** Spec 14 (docker-compose stack), Spec 18 (REST + OpenAPI)
**Blocks:** —

## Objective

Expose the local docker-compose stack behind a Cloudflare Tunnel so
external clients (the Unity client, mobile, web admin) can reach the
REST API over HTTPS via a stable hostname, without opening inbound
firewall ports on the host. Support two modes:

1. **Quick tunnel** (dev / CI / demos): `cloudflared tunnel --url …`
   generates an ephemeral `*.trycloudflare.com` URL — zero
   credentials, zero DNS, zero config. Use during development or
   when a teammate wants to point a remote client at your laptop.

2. **Named tunnel** (staging / production): persistent tunnel
   connected to a real Cloudflare account + zone, ingress rules
   route by hostname to the right docker-compose service.

Out of scope:

- **UDP Game Server** — `cloudflared` does not carry UDP traffic.
  The Game Server keeps its existing direct `7000:7000/udp` host
  publish. (Cloudflare Spectrum is the paid UDP tunnel option;
  document it as a follow-up rather than implement it here.)
- Service auth via Cloudflare Access.
- Multi-region load balancing.
- Migrating the API off the existing `nginx` reverse proxy.

## Why

Today the stack is reachable only via `localhost` or via the host's
own IP. A remote player can't connect without a VPN, an SSH tunnel,
or punching a hole in the firewall. Cloudflare Tunnel flips the
model: the server opens an outbound connection to Cloudflare's edge
and Cloudflare forwards inbound HTTPS to us. No inbound firewall
rules, free TLS via Cloudflare's certificates, free DDoS shield.

## Architecture

```
   Internet                  Cloudflare Edge                 Our host
 ┌──────────┐   HTTPS    ┌───────────────────┐  outbound  ┌────────────────┐
 │ Unity /  │ ─────────▶ │ api.example.com   │ ─────────▶ │ cloudflared    │
 │ web      │            │   → nginx (in-vm) │            │   (container)  │
 │ client   │            │                   │            │     │          │
 └──────────┘            │ racing.example    │            │     ▼          │
                         │   → SPA (future)  │            │  backend-api   │
                         └───────────────────┘            │   :8080        │
                                                          │                │
   UDP client ────────── (direct) ──────────────────────▶│ game-server    │
   (port 7000)                                           │   :7000/udp    │
                                                          └────────────────┘
```

The `cloudflared` container joins `racing-network`. Ingress rules
forward `api.<domain>` to `http://backend-api:8080` (bypassing the
local nginx, which is useful for production; nginx stays in the
compose file for local dev where there's no tunnel).

## Files / changes

### 1. `docker-compose.yml`

Add a new service `cloudflared` (always present, behavior controlled
by env vars):

```yaml
cloudflared:
  image: cloudflare/cloudflared:2025.4.1
  container_name: racing-cloudflared
  command: >-
    tunnel --config /etc/cloudflared/config.yml --no-autoupdate
    ${TUNNEL_MODE_FLAGS}
  environment:
    TUNNEL_TOKEN: ${TUNNEL_TOKEN:-}
  volumes:
    - ./cloudflared:/etc/cloudflared:ro
  networks: [racing-network]
  depends_on:
    backend-api: { condition: service_started }
  restart: unless-stopped
```

`TUNNEL_MODE_FLAGS` is built in `.env` to one of:

- **quick**: `run --url http://backend-api:8080` — quick tunnel mode
- **named**: `run` — uses the credentials file + token in
  `cloudflared/`

### 2. `.env.example` (additions)

```
# Cloudflare Tunnel
TUNNEL_MODE=quick                 # quick | named
TUNNEL_DOMAIN=                    # api.example.com (named mode)
TUNNEL_TOKEN=                     # from `cloudflared tunnel token …`
CLOUDFLARE_ACCOUNT_ID=            # for setup-tunnel.sh
CLOUDFLARE_ZONE_ID=               # for setup-tunnel.sh
CLOUDFLARE_API_TOKEN=             # for setup-tunnel.sh (DNS edit perms)
```

### 3. `cloudflared/config.yml` (new)

Used only in `named` mode:

```yaml
tunnel: <TUNNEL_UUID>
credentials-file: /etc/cloudflared/<TUNNEL_UUID>.json

ingress:
  - hostname: api.${TUNNEL_DOMAIN}
    service: http://backend-api:8080
  - hostname: racing.${TUNNEL_DOMAIN}
    service: http://backend-api:8080
  - service: http_status:404
```

### 4. `scripts/setup-tunnel.sh` (new)

Bootstrap helper for `named` mode. Wraps `cloudflared` + `curl`
calls to Cloudflare's API so the dev doesn't need to learn the
Cloudflare dashboard:

1. Check `cloudflared` is installed locally.
2. `cloudflared tunnel login` (opens browser, captures cert).
3. `cloudflared tunnel create racing-backend` → writes
   `cloudflared/<UUID>.json` and prints the tunnel UUID + token.
4. `cloudflared tunnel route dns racing-backend api.$TUNNEL_DOMAIN`
   (creates the CNAME in Cloudflare's DNS).
5. Generate `cloudflared/config.yml` from the env vars.
6. Print the resulting `https://api.$TUNNEL_DOMAIN/`.

Idempotent: rerunning with an existing tunnel just regenerates the
config and prints the URL.

### 5. `docs/cloudflare-tunnel.md` (new)

End-user docs covering both modes, with copy-pasteable commands.

### 6. `README.md`

Add a "Public access via Cloudflare Tunnel" section pointing to
`docs/cloudflare-tunnel.md`.

## Implementation steps

1. Add `cloudflared` service to `docker-compose.yml` with mode-aware
   command. Verify it starts in `quick` mode without any env vars
   and prints a `*.trycloudflare.com` URL to the container log.
2. Create `cloudflared/config.yml` and `scripts/setup-tunnel.sh`.
   Make `setup-tunnel.sh` executable + `shellcheck`-clean.
3. Add the new env vars to `.env.example`. Do **not** commit real
   credentials — `.env` stays in `.gitignore`.
4. Write `docs/cloudflare-tunnel.md` with both modes documented.
5. Add a `tests/cloudflared_config_test.sh` smoke test that:
   - validates `config.yml` parses with `yq` if available
   - asserts the required env vars are referenced
   - asserts no real tokens are committed

## Acceptance criteria

- [ ] `docker compose up cloudflared` (with `TUNNEL_MODE=quick`)
      reaches `service_healthy` and logs a `trycloudflare.com` URL.
- [ ] An external client can hit
      `https://<ephemeral>.trycloudflare.com/api/v1/health` and get
      `{ "status": "ok", … }`.
- [ ] `TUNNEL_MODE=named` + `TUNNEL_TOKEN=…` boots without printing
      any auth errors and connects to the configured tunnel.
- [ ] `scripts/setup-tunnel.sh` is idempotent and exits 0 on a
      second run with the same args.
- [ ] The UDP Game Server is **not** routed through the tunnel —
      its `7000:7000/udp` host publish is unchanged.
- [ ] `docs/cloudflare-tunnel.md` documents both modes, lists the
      required env vars, and links to Cloudflare's docs for the
      account-side setup (zone, API token permissions).
- [ ] Real credentials never appear in `git log` —
      `.gitignore` covers `.env` and `cloudflared/*.json`.

## Verification

- **Quick mode smoke test:**
  ```
  TUNNEL_MODE=quick docker compose up -d cloudflared
  docker logs -f racing-cloudflared  # wait for trycloudflare.com URL
  URL=$(docker logs racing-cloudflared | grep -oE 'https://[a-z0-9-]+\.trycloudflare\.com' | head -1)
  curl -sS "$URL/api/v1/health"
  ```
  Expected: `200 {"status":"ok","service":"backend-api","database":"ok"}`.

- **Named mode smoke test** (requires a real Cloudflare account +
  domain — only run in staging, not in CI):
  ```
  ./scripts/setup-tunnel.sh
  TUNNEL_MODE=named docker compose up -d cloudflared
  curl -sS https://api.$TUNNEL_DOMAIN/api/v1/health
  ```

- **UDP untouched:** `nmap -sU -p 7000 localhost` still shows the
  port open after the tunnel is up.

## Risks

- **Tunnel hijack via stolen token**: `TUNNEL_TOKEN` is a bearer
  secret. Document this and ensure `.env` is gitignored. Rotate the
  token if it leaks.
- **Cloudflare account outage** = public API outage. Document.
- **Quick tunnels change URL on every restart** — Unity clients
  that hard-coded a trycloudflare URL will break. Document that
  quick mode is for dev/demos only.
- **`cloudflared` version drift**: pin the image tag.

## Notes

- Cloudflare's UDP story is "use Spectrum" (paid) or "expose the
  port directly." We pick the latter because (a) it's free, (b) the
  Game Server has no need for DDoS protection at MVP scale, and
  (c) most racing-game backends expose their UDP port via a CDN
  edge only at very high scale.
- If/when we add Unity Cloud Code or another Unity-hosted relay,
  we can revisit: Cloudflare Spectrum for UDP, or run our own
  anycast relay.
- The `cloudflared` container can be removed from the compose file
  for teammates who only develop locally — they can keep using
  `localhost:8081` and the existing `nginx` reverse proxy.
