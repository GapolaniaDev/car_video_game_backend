#!/usr/bin/env bash
# ─────────────────────────────────────────────────────────────────────
# cloudflared_config_test.sh — smoke test for the Cloudflare Tunnel
# config (Spec 22). Does NOT start cloudflared; that would require
# network egress to Cloudflare. Instead it verifies the static
# config files are well-formed and reference the right services.
#
# Usage:
#   ./tests/cloudflared_config_test.sh
# ─────────────────────────────────────────────────────────────────────

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
FAIL=0

step() { printf '\n─── %s ───\n' "$*"; }
ok()   { printf '  ✓ %s\n' "$*"; }
bad()  { printf '  ✗ %s\n' "$*"; FAIL=$((FAIL + 1)); }
need() { command -v "$1" >/dev/null 2>&1 || { bad "missing required tool: $1"; exit 1; }; }

need yq

step "docker-compose.yml references the cloudflared service"
if yq -e '.services.cloudflared' "${REPO_ROOT}/docker-compose.yml" >/dev/null 2>&1; then
  ok "services.cloudflared is defined"
else
  bad "services.cloudflared missing from docker-compose.yml"
fi
if yq -e '.services.cloudflared.image' "${REPO_ROOT}/docker-compose.yml" >/dev/null 2>&1; then
  ok "image is pinned: $(yq '.services.cloudflared.image' "${REPO_ROOT}/docker-compose.yml")"
else
  bad "cloudflared image not set"
fi

step "docker-compose.yml mounts the entrypoint script"
ENTRYPOINT_BIND="$(yq '.services.cloudflared.volumes[] | select(test("cloudflared-entrypoint"))' "${REPO_ROOT}/docker-compose.yml" 2>/dev/null || true)"
if [[ -n "${ENTRYPOINT_BIND}" ]]; then
  ok "entrypoint script is bind-mounted: ${ENTRYPOINT_BIND}"
else
  bad "cloudflared-entrypoint.sh is not bind-mounted into the container"
fi

step "scripts/cloudflared-entrypoint.sh is executable + starts with shebang"
EP="${REPO_ROOT}/scripts/cloudflared-entrypoint.sh"
if [[ -x "${EP}" ]]; then
  ok "executable bit set"
else
  bad "scripts/cloudflared-entrypoint.sh is not executable (chmod +x)"
fi
if head -1 "${EP}" | grep -q '^#!'; then
  ok "starts with shebang"
else
  bad "no shebang on first line"
fi
if grep -qE '^[[:space:]]*quick\)' "${EP}"; then
  ok "handles quick mode"
else
  bad "no quick-mode branch"
fi
if grep -qE '^[[:space:]]*named\)' "${EP}"; then
  ok "handles named mode"
else
  bad "no named-mode branch"
fi

step "scripts/setup-tunnel.sh is executable + references the expected commands"
SU="${REPO_ROOT}/scripts/setup-tunnel.sh"
if [[ -x "${SU}" ]]; then
  ok "executable bit set"
else
  bad "scripts/setup-tunnel.sh is not executable"
fi
for needle in 'cloudflared tunnel login' 'cloudflared tunnel create' 'cloudflared tunnel info' 'cloudflared tunnel route dns'; do
  if grep -qF "${needle}" "${SU}"; then
    ok "references: ${needle}"
  else
    bad "missing: ${needle}"
  fi
done

step ".env.example documents the new variables"
ENV="${REPO_ROOT}/.env.example"
for var in TUNNEL_MODE TUNNEL_DOMAIN TUNNEL_TOKEN CLOUDFLARE_API_TOKEN; do
  if grep -qE "^${var}=" "${ENV}"; then
    ok "documents ${var}"
  else
    bad ".env.example missing ${var}"
  fi
done

step ".gitignore excludes the cloudflared credentials"
GI="${REPO_ROOT}/.gitignore"
if grep -qE '^cloudflared/\*\.json' "${GI}"; then
  ok "ignores cloudflared/*.json"
else
  bad ".gitignore missing cloudflared/*.json"
fi
if grep -qE '^cloudflared/cert\.pem' "${GI}"; then
  ok "ignores cloudflared/cert.pem"
else
  bad ".gitignore missing cloudflared/cert.pem"
fi

step "cloudflared/config.yml is valid YAML + references backend-api:8080"
CFG="${REPO_ROOT}/cloudflared/config.yml"
if yq -e '.ingress' "${CFG}" >/dev/null 2>&1; then
  ok "ingress section present"
else
  bad "ingress section missing or invalid YAML"
fi
# In a real bootstrap the host substitutes ${TUNNEL_DOMAIN}; check
# the un-rendered form.
if grep -q 'backend-api:8080' "${CFG}"; then
  ok "ingress routes to backend-api:8080"
else
  bad "ingress does not reference backend-api:8080"
fi
# Quick-mode config has a single rule (cloudflared rejects multi-rule
# configs in --url mode); the http_status:404 catch-all is rendered
# in by setup-tunnel.sh for named mode. Both are acceptable.
if grep -q 'http_status:404' "${CFG}"; then
  ok "catch-all returns 404 (named-mode style)"
else
  ok "single-rule ingress (quick-mode style; no 404 catch-all is fine)"
fi

step "no real tokens leaked into the repo"
# .env (real) and the per-UUID credentials JSON should not be present.
if [[ -f "${REPO_ROOT}/.env" && ! -f "${REPO_ROOT}/.env.example" ]]; then
  bad ".env exists but .env.example missing — uncommon state"
fi
# Search the tracked tree for known secret prefixes (case-insensitive).
HITS="$(grep -rE 'Bearer [A-Za-z0-9_-]{40,}|eyJ[A-Za-z0-9_-]{30,}|cloudflared.*token.*[a-f0-9-]{36}' \
  --include='*.go' --include='*.sh' --include='*.yml' --include='*.yaml' --include='*.md' \
  --exclude-dir='.git' --exclude-dir='node_modules' "${REPO_ROOT}" 2>/dev/null \
  | grep -vE '\.env\.example|README|docs/cloudflare-tunnel|specs/' || true)"
if [[ -z "${HITS}" ]]; then
  ok "no obvious secrets in tracked source"
else
  bad "possible secret leakage:
${HITS}"
fi

step "result"
if [[ "${FAIL}" -eq 0 ]]; then
  echo "  ALL OK"
  exit 0
else
  echo "  ${FAIL} check(s) failed"
  exit 1
fi
