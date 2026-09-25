#!/bin/bash
# ─────────────────────────────────────────────────────────────────────
# keep-awake-on-ac.sh — corre caffeinate solo cuando la Mac está
# enchufada a corriente. Cuando la desenchufás (batería), la Mac
# puede dormir normal y no drena batería.
#
# LaunchAgent en ~/Library/LaunchAgents/local.tunnel.keep-awake.plist
# corre este script al login.
# ─────────────────────────────────────────────────────────────────────

CYCLE_SECONDS=30

while true; do
  # pmset -g batt muestra "AC power" o "Battery power"
  POWER=$(pmset -g batt | grep -oE '(AC power|Battery power)' | head -1)

  if [ "$POWER" = "AC power" ]; then
    # Prevent display + system sleep. Quit after 1 cycle so a new
    # caffeinate takes over; prevents multiple stacking.
    pkill -x caffeinate 2>/dev/null
    /usr/bin/caffeinate -dis &
    CAFF_PID=$!
    sleep "$CYCLE_SECONDS"
    kill "$CAFF_PID" 2>/dev/null
  else
    # On battery: stop caffeinate so the Mac can sleep normally.
    pkill -x caffeinate 2>/dev/null
    sleep "$CYCLE_SECONDS"
  fi
done
