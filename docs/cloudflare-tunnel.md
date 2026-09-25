# Cloudflare Tunnel (Spec 22)

This stack ships an optional [`cloudflared`](https://github.com/cloudflare/cloudflared)
container that wraps the REST API in a Cloudflare Tunnel. UDP traffic
for the Game Server is **not** tunneled — the Game Server keeps its
own direct exposure on `:7000/udp`.

The tunnel container supports two modes, controlled by `TUNNEL_MODE`
in `.env`:

| Mode     | When to use                                                                                         | DNS / account needed? |
| -------- | --------------------------------------------------------------------------------------------------- | --------------------- |
| `quick`  | Local dev, CI, demos, "let me show my teammate on Discord for 5 minutes"                           | No                    |
| `named`  | Staging, production, anywhere you want a stable hostname like `api.example.com`                    | Yes                   |

UDP Game Server traffic is **not** affected by either mode.

---

## Quick tunnel (`TUNNEL_MODE=quick`)

Zero-setup: no Cloudflare account, no DNS, no credentials. Cloudflare
hands you an ephemeral `*.trycloudflare.com` URL on every startup.

```sh
# 1. Make sure .env exists with default values
cp .env.example .env

# 2. Start the stack
docker compose up -d backend-api cloudflared

# 3. Watch the cloudflared log for your URL
docker logs -f racing-cloudflared
# …wait for the line containing trycloudflare.com…
# https://something-random.trycloudflare.com

# 4. Hit it
URL=$(docker logs racing-cloudflared | grep -oE 'https://[a-z0-9-]+\.trycloudflare\.com' | head -1)
curl -sS "$URL/api/v1/health"
# → {"status":"ok","service":"backend-api","database":"ok"}
```

The URL changes every time `cloudflared` restarts. Don't bake it into
client builds — point Unity at it only for ephemeral testing.

---

## Named tunnel (`TUNNEL_MODE=named`)

Persistent tunnel attached to a real Cloudflare account + zone. DNS
records live in your zone and survive restarts.

### Prerequisites

1. A Cloudflare account with at least one zone (a domain you control).
2. A DNS-record **Edit** API token. Create one in the Cloudflare
   dashboard under **My Profile → API Tokens → Create Token →
   Edit zone DNS**. Limit the token to the specific zone.

### One-time host setup

Run the bootstrap script on the host (not inside Docker):

```sh
export TUNNEL_DOMAIN=example.com
export CLOUDFLARE_ACCOUNT_ID=…   # optional, not used yet
export CLOUDFLARE_ZONE_ID=…      # required if using the API
export CLOUDFLARE_API_TOKEN=…    # required if using the API

./scripts/setup-tunnel.sh
```

The script will:

1. Verify `cloudflared` and `jq` are installed on the host.
2. Run `cloudflared tunnel login` — opens a browser, captures
   `cert.pem`. Re-run safely; skipped if cert.pem already exists.
3. Create the tunnel `racing-backend` (skipped if it already exists).
4. Create CNAMEs `api.<domain>` and `racing.<domain>` pointing at the
   tunnel, either via the Cloudflare API (when
   `CLOUDFLARE_API_TOKEN` is set) or via
   `cloudflared tunnel route dns`.
5. Render `cloudflared/config.yml` with the resolved UUID.
6. Print the public URLs and the `cloudflared tunnel token …`
   command you need to run to mint a TUNNEL_TOKEN.

### Boot the tunnel

```sh
# 1. Run the bootstrap output:
cloudflared tunnel token racing-backend
# → paste into .env as TUNNEL_TOKEN=…

# 2. Set mode + domain in .env
TUNNEL_MODE=named
TUNNEL_DOMAIN=example.com

# 3. Bring it up
docker compose up -d cloudflared
docker logs -f racing-cloudflared
```

You should see the tunnel connect (no `trycloudflare.com` URL this
time — it's using your real hostname).

### Verify

```sh
curl -sS https://api.example.com/api/v1/health
# → {"status":"ok","service":"backend-api","database":"ok"}
```

---

## What the tunnel exposes

| Hostname                       | Service             | Notes                              |
| ------------------------------ | ------------------- | ---------------------------------- |
| `api.<TUNNEL_DOMAIN>`          | `backend-api:8080`  | REST API                           |
| `racing.<TUNNEL_DOMAIN>`       | `backend-api:8080`  | Reserved for the SPA frontend      |
| anything else                  | `404`               | Clean edge-level 404               |

UDP `:7000` is unaffected — clients reach the Game Server via
`GAME_SERVER_PUBLIC_HOST` / `GAME_SERVER_PUBLIC_PORT`, which is
whatever DNS/IP you give them for the host itself (NOT a
Cloudflare URL).

---

## Troubleshooting

| Symptom                                                      | Cause / fix                                                                                                |
| ------------------------------------------------------------ | ---------------------------------------------------------------------------------------------------------- |
| Container exits with "unknown TUNNEL_MODE"                   | `.env` has a typo or `TUNNEL_MODE` is unset → defaults to `quick`. Fix the env var.                       |
| `named` mode: "tunnel token missing"                         | Run `cloudflared tunnel token racing-backend` and paste into `.env` as `TUNNEL_TOKEN`.                    |
| `named` mode: "no such file cert.pem"                        | `scripts/setup-tunnel.sh` was not run, or was run on a different host.                                     |
| Quick mode prints no `trycloudflare.com` URL                 | Container can't reach `backend-api:8080` → check `docker logs racing-backend-api`.                        |
| `403 Forbidden` on every request                              | The host running `cloudflared` is in the Cloudflare dashboard's **Tunnel** list as **Inactive**. Check DNS. |
| UDP `:7000` not reachable from a remote client              | Expected — tunnels do not carry UDP. Open `:7000/udp` on the host firewall or use Cloudflare Spectrum (paid). |

---

## Why is the Game Server NOT tunneled?

Cloudflare Tunnel carries TCP and HTTP/HTTPS only. For UDP you have
two options:

1. **Direct exposure** (what we do): open `:7000/udp` on the host.
   Free, simple, works for the MVP.
2. **Cloudflare Spectrum** (paid): a separate Cloudflare product
   that proxies UDP. Document as a follow-up when we need DDoS
   protection on the Game Server port.

For the multiplayer racing MVP, option 1 is enough.

---

## Files

| File                                  | Purpose                                                                   |
| ------------------------------------- | ------------------------------------------------------------------------- |
| `docker-compose.yml`                  | `cloudflared` service definition.                                         |
| `scripts/cloudflared-entrypoint.sh`   | Picks `tunnel --url` vs `tunnel run` based on `TUNNEL_MODE`.              |
| `scripts/setup-tunnel.sh`             | One-shot host bootstrap for `named` mode.                                 |
| `cloudflared/config.yml`              | Ingress rules (rendered by `setup-tunnel.sh` for `named` mode).           |
| `.env.example`                        | Documents `TUNNEL_MODE`, `TUNNEL_DOMAIN`, `TUNNEL_TOKEN`, etc.           |
| `.gitignore`                          | Excludes `cloudflared/*.json` and `cert.pem` from VCS.                    |
| `tests/cloudflared_config_test.sh`    | Smoke test for the config + entrypoint.                                   |
