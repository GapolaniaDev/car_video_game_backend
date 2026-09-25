#!/usr/bin/env bash
# ─────────────────────────────────────────────────────────────────────
# setup-tunnel.sh — bootstrap a Cloudflare named tunnel for the
# racing-game-backend (Spec 22).
#
# What it does (idempotent):
#   1. Verifies `cloudflared` is installed locally.
#   2. Loads `.env` if present.
#   3. Runs `cloudflared tunnel login` (opens a browser once, caches
#      cert.pem) — skipped if cert.pem already exists.
#   4. `cloudflared tunnel create racing-backend` — skipped if the
#      tunnel already exists in the account.
#   5. Creates the DNS CNAMEs api.<domain> and racing.<domain>
#      pointing at the tunnel, either via the Cloudflare API
#      (CLOUDFLARE_API_TOKEN set) or via `cloudflared tunnel
#      route dns`.
#   6. Writes cloudflared/config.yml with the resolved UUID and
#      credentials path.
#   7. Prints the resulting public hostname(s) so the operator can
#      copy them into README / Unity build config.
#
# Usage:
#   ./scripts/setup-tunnel.sh                    # interactive
#   TUNNEL_DOMAIN=example.com ./scripts/setup-tunnel.sh
#
# Exit codes:
#   0  success
#   1  missing prerequisites
#   2  Cloudflare auth not done (cert.pem absent)
#   3  Cloudflare API error
# ─────────────────────────────────────────────────────────────────────

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CRED_DIR="${REPO_ROOT}/cloudflared"
CONFIG_FILE="${CRED_DIR}/config.yml"
TUNNEL_NAME="racing-backend"
LOG_PREFIX="setup-tunnel"

log()  { printf '[%s] %s\n' "${LOG_PREFIX}" "$*"; }
err()  { printf '[%s] ERROR: %s\n' "${LOG_PREFIX}" "$*" >&2; }
die()  { err "$*"; exit "${2:-1}"; }

# ─── 1. Prerequisites ───────────────────────────────────────────────
command -v cloudflared >/dev/null 2>&1 \
  || die "cloudflared is not installed. Get it from https://pkg.cloudflare.com/ or 'brew install cloudflared'."

command -v jq >/dev/null 2>&1 \
  || die "jq is not installed. brew install jq / apt install jq."

mkdir -p "${CRED_DIR}"

# ─── 2. Load .env if present ─────────────────────────────────────────
if [[ -f "${REPO_ROOT}/.env" ]]; then
  log "loading ${REPO_ROOT}/.env"
  set -a
  # shellcheck disable=SC1091
  source "${REPO_ROOT}/.env"
  set +a
fi

: "${TUNNEL_DOMAIN:?TUNNEL_DOMAIN must be set (e.g. example.com)}"

# ─── 3. cloudflared login ────────────────────────────────────────────
if [[ ! -f "${CRED_DIR}/cert.pem" && ! -f "${HOME}/.cloudflared/cert.pem" ]]; then
  log "running 'cloudflared tunnel login' — follow the browser prompt"
  cloudflared tunnel login
else
  log "found existing cert.pem, skipping login"
fi

# cloudflared looks for cert.pem in $HOME/.cloudflared by default.
# Copy our project-local cert.pem into place if we have one and the
# global one is missing.
if [[ -f "${CRED_DIR}/cert.pem" && ! -f "${HOME}/.cloudflared/cert.pem" ]]; then
  mkdir -p "${HOME}/.cloudflared"
  cp "${CRED_DIR}/cert.pem" "${HOME}/.cloudflared/cert.pem"
  log "installed project cert.pem to ~/.cloudflared/"
fi

# ─── 4. Create the tunnel (idempotent) ───────────────────────────────
log "ensuring tunnel '${TUNNEL_NAME}' exists"
if ! cloudflared tunnel info "${TUNNEL_NAME}" >/dev/null 2>&1; then
  cloudflared tunnel create "${TUNNEL_NAME}"
fi

# Resolve the tunnel UUID + credentials path.
TUNNEL_INFO="$(cloudflared tunnel info "${TUNNEL_NAME}" --output json)"
TUNNEL_UUID="$(printf '%s' "${TUNNEL_INFO}" | jq -r '.id')"
[[ -n "${TUNNEL_UUID}" && "${TUNNEL_UUID}" != "null" ]] \
  || die "could not resolve tunnel UUID for '${TUNNEL_NAME}'"
log "tunnel uuid: ${TUNNEL_UUID}"

# Copy the credentials JSON into the project so docker-compose can
# mount it. cloudflared stores it under ~/.cloudflared/<UUID>.json
SRC_CREDS="${HOME}/.cloudflared/${TUNNEL_UUID}.json"
DST_CREDS="${CRED_DIR}/${TUNNEL_UUID}.json"
if [[ -f "${SRC_CREDS}" && ! -f "${DST_CREDS}" ]]; then
  cp "${SRC_CREDS}" "${DST_CREDS}"
  log "copied credentials to ${DST_CREDS}"
fi

# ─── 5. DNS records ──────────────────────────────────────────────────
create_dns_via_cli() {
  local host="$1"
  log "cloudflared tunnel route dns ${TUNNEL_NAME} ${host}"
  cloudflared tunnel route dns "${TUNNEL_NAME}" "${host}" || true
}

create_dns_via_api() {
  local host="$1"
  local fqdn="${host}.${TUNNEL_DOMAIN}"
  local body
  body="$(jq -n \
    --arg type "CNAME" \
    --arg name "${host}" \
    --arg content "${TUNNEL_UUID}.cfargotunnel.com" \
    --argjson proxied true \
    '{type:$type, name:$name, content:$content, proxied:$proxied, ttl:1}')"
  local resp
  resp="$(curl -fsS -X POST \
    -H "Authorization: Bearer ${CLOUDFLARE_API_TOKEN}" \
    -H "Content-Type: application/json" \
    -d "${body}" \
    "https://api.cloudflare.com/client/v4/zones/${CLOUDFLARE_ZONE_ID}/dns_records")" \
    || die "Cloudflare API DNS create failed for ${fqdn}" 3
  local ok
  ok="$(printf '%s' "${resp}" | jq -r '.success')"
  if [[ "${ok}" != "true" ]]; then
    err "Cloudflare API returned: ${resp}"
    die "DNS create failed for ${fqdn}" 3
  fi
}

for host in api racing; do
  if [[ -n "${CLOUDFLARE_API_TOKEN}" && -n "${CLOUDFLARE_ZONE_ID}" ]]; then
    log "creating DNS via Cloudflare API for ${host}.${TUNNEL_DOMAIN}"
    create_dns_via_api "${host}" || true
  else
    log "creating DNS via cloudflared CLI for ${host}.${TUNNEL_DOMAIN}"
    create_dns_via_cli "${host}.${TUNNEL_DOMAIN}"
  fi
done

# ─── 6. Write config.yml ─────────────────────────────────────────────
log "writing ${CONFIG_FILE}"
cat > "${CONFIG_FILE}" <<YAML
# Generated by scripts/setup-tunnel.sh on $(date -u +%Y-%m-%dT%H:%M:%SZ).
# Do not hand-edit the tunnel UUID / credentials-file fields; rerun
# the script if those change.

tunnel: ${TUNNEL_UUID}
credentials-file: /etc/cloudflared/${TUNNEL_UUID}.json

ingress:
  - hostname: api.${TUNNEL_DOMAIN}
    service: http://backend-api:8080
    originRequest:
      noTLSVerify: false
  - hostname: racing.${TUNNEL_DOMAIN}
    service: http://backend-api:8080
  - service: http_status:404

warp-routing:
  enabled: false

metrics: localhost:2000
YAML

# ─── 7. Done ─────────────────────────────────────────────────────────
cat <<EOF

──────────────────────────────────────────────────────────────
 Tunnel ready!

   Tunnel name : ${TUNNEL_NAME}
   Tunnel UUID : ${TUNNEL_UUID}
   Public URLs : https://api.${TUNNEL_DOMAIN}/
                https://racing.${TUNNEL_DOMAIN}/

 Next steps:
   1. Set TUNNEL_MODE=named and TUNNEL_TOKEN in your .env.
      Get the token with:
         cloudflared tunnel token ${TUNNEL_NAME}
   2. Run: TUNNEL_MODE=named docker compose up -d cloudflared
   3. Verify: curl https://api.${TUNNEL_DOMAIN}/api/v1/health

 The UDP Game Server is NOT tunneled. Expose :7000 directly
 on the host (already done by docker-compose) when remote
 players need to reach it.
──────────────────────────────────────────────────────────────
EOF
