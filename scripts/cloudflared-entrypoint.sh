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
      echo "  Run scripts/setup-tunnel.sh on the host first, then set TUNNEL_TOKEN in .env" >&2
      exit 1
    fi
    echo "[cloudflared-entrypoint] mode=named; connecting tunnel token ${TUNNEL_TOKEN:0:8}…"
    exec cloudflared tunnel --no-autoupdate --config /etc/cloudflared/config.yml run
    ;;

  *)
    echo "[cloudflared-entrypoint] ERROR: unknown TUNNEL_MODE '$MODE' (expected: quick|named)" >&2
    exit 1
    ;;
esac
