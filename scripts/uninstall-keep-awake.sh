#!/bin/bash
# wrapper para que sea fácil de recordar
exec "$(dirname "${BASH_SOURCE[0]}")/install-keep-awake.sh" --uninstall
