#!/bin/sh
# ─────────────────────────────────────────────────────────────────────
# cloudflared-entrypoint.sh — pick the right cloudflared command
# based on TUNNEL_MODE (Spec 22).
#
# Modes:
#   quick  — `cloudflared tunnel --url …` — no token, no DNS.
#            Prints a `*.trycloudflare.com` URL on stdout within a
#            few seconds. Used during dev/demos. The ingress in
#            /etc/cloudflared/config.yml is ignored in this mode.
#   named  — `cloudflared tunnel run` with TUNNEL_TOKEN. Requires
#            scripts/setup-tunnel.sh to have been run on the host.
#
# This script is mounted into the cloudflared container at
# /entrypoint.sh and replaces the image's default ENTRYPOINT.
# ─────────────────────────────────────────────────────────────────────

set -eu

MODE="${TUNNEL_MODE:-quick}"
BACKEND="${BACKEND_HOST:-backend-api}:8080"

case "$MODE" in
  quick)
    echo "[cloudflared-entrypoint] mode=quick; starting quick tunnel to ${BACKEND}"
    exec cloudflared tunnel --no-autoupdate --config /etc/cloudflared/config.yml --url "http://${BACKEND}"
    ;;

  named)
    if [ -z "${TUNNEL_TOKEN:-}" ]; then
      echo "[cloudflared-entrypoint] ERROR: TUNNEL_MODE=named requires TUNNEL_TOKEN" >&2
      echo "  Mint one in the Cloudflare dashboard (Zero Trust -> Networks" >&2
      echo "  -> Tunnels -> your tunnel -> 'Configure' -> Docker -> copy" >&2
      echo "  the --token value into TUNNEL_TOKEN in .env)." >&2
      exit 1
    fi
    echo "[cloudflared-entrypoint] mode=named; connecting token ${TUNNEL_TOKEN:0:8}…"
    # --token is a flag of the `run` subcommand, not a global flag
    # of `tunnel`. The token is self-contained: cloudflared
    # authenticates against Cloudflare's API and pulls the
    # credentials itself. We don't need credentials-file or a
    # static tunnel UUID in config.yml.
    exec cloudflared tunnel --no-autoupdate run --token "${TUNNEL_TOKEN}"
    ;;

  *)
    echo "[cloudflared-entrypoint] ERROR: unknown TUNNEL_MODE '$MODE' (expected: quick|named)" >&2
    exit 1
    ;;
esac
