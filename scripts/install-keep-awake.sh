#!/bin/bash
# ─────────────────────────────────────────────────────────────────────
# install-keep-awake.sh — instala el LaunchAgent que mantiene la
# Mac despierta cuando está enchufada, para que el tunnel siga vivo.
#
# Install: ./scripts/install-keep-awake.sh
# Remove:  ./scripts/install-keep-awake.sh --uninstall
# ─────────────────────────────────────────────────────────────────────

set -eu

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PLIST_NAME="local.tunnel.keep-awake"
PLIST_PATH="$HOME/Library/LaunchAgents/${PLIST_NAME}.plist"
LOG_OUT="/tmp/tunnel-keep-awake.log"
LOG_ERR="/tmp/tunnel-keep-awake.err"

case "${1:-}" in
  --uninstall)
    launchctl unload "$PLIST_PATH" 2>/dev/null || true
    rm -f "$PLIST_PATH"
    pkill -x caffeinate 2>/dev/null || true
    echo "Uninstalled: $PLIST_PATH"
    exit 0
    ;;
esac

# 1) El script de keep-awake debe existir y ser ejecutable
if [[ ! -x "$REPO_ROOT/scripts/keep-awake-on-ac.sh" ]]; then
  echo "ERROR: $REPO_ROOT/scripts/keep-awake-on-ac.sh no es ejecutable" >&2
  echo "  chmod +x scripts/keep-awake-on-ac.sh" >&2
  exit 1
fi

# 2) Escribir el plist
cat > "$PLIST_PATH" <<PLIST
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>${PLIST_NAME}</string>

    <!-- start whenever the user logs in -->
    <key>RunAtLoad</key>
    <true/>

    <!-- restart automatically if it dies -->
    <key>KeepAlive</key>
    <true/>

    <key>ProcessType</key>
    <string>Background</string>

    <key>ProgramArguments</key>
    <array>
        <string>/bin/bash</string>
        <string>${REPO_ROOT}/scripts/keep-awake-on-ac.sh</string>
    </array>

    <key>StandardOutPath</key>
    <string>${LOG_OUT}</string>

    <key>StandardErrorPath</key>
    <string>${LOG_ERR}</string>
</dict>
</plist>
PLIST

# 3) Cargar el LaunchAgent
launchctl unload "$PLIST_PATH" 2>/dev/null || true
launchctl load "$PLIST_PATH"

echo ""
echo "✓ Instalado: $PLIST_PATH"
echo ""
echo "Cómo verificar que está corriendo:"
echo "  launchctl list | grep tunnel-keep-awake"
echo "  tail -f $LOG_OUT"
echo ""
echo "Para desinstalar:"
echo "  $REPO_ROOT/scripts/install-keep-awake.sh --uninstall"
