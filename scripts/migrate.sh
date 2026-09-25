#!/usr/bin/env bash
# scripts/migrate.sh — run migrations manually against a running Postgres.
#
# Usage:
#   POSTGRES_HOST=localhost POSTGRES_PORT=5432 \
#   POSTGRES_DB=racing_game POSTGRES_USER=racing_user \
#   POSTGRES_PASSWORD=dev_password ./scripts/migrate.sh up
#
# Requires the `migrate` CLI to be installed locally:
#   go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest

set -euo pipefail

DIRECTION="${1:-up}"

: "${POSTGRES_HOST:=postgres}"
: "${POSTGRES_PORT:=5432}"
: "${POSTGRES_DB:=racing_game}"
: "${POSTGRES_USER:=racing_user}"
: "${POSTGRES_PASSWORD:?POSTGRES_PASSWORD must be set}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
MIGRATIONS_DIR="$SCRIPT_DIR/../migrations"

DSN="postgres://${POSTGRES_USER}:${POSTGRES_PASSWORD}@${POSTGRES_HOST}:${POSTGRES_PORT}/${POSTGRES_DB}?sslmode=disable"

migrate -path "$MIGRATIONS_DIR" -database "$DSN" "$DIRECTION"