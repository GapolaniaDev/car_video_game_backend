# Spec 04 — Database Migrations

**Status:** DONE
**Section:** 12, 13
**Depends on:** Spec 03
**Blocks:** Spec 15

## Objective

Create initial SQL migrations for the persistent data model. Migrations must run automatically on container startup and create the base schema.

## Scope

- Choose a lightweight, Go-compatible migration tool. Recommended: **`golang-migrate/migrate`** invoked from a tiny entrypoint, OR **`pressly/goose`** with embedded SQL files. Pick one and justify in this spec.
- Migration files live in `migrations/` with timestamped names: `000001_init.up.sql`, `000001_init.down.sql`, etc.
- Use UUID primary keys (`uuid` type + `gen_random_uuid()` via `pgcrypto` extension), unless a strong reason exists for another strategy.
- Use proper FKs, indexes, timestamps (`created_at`, `updated_at`), and constraints.

### Tables to create
- `users` — auth identity (id UUID, email unique, password_hash nullable for guest, created_at, updated_at)
- `players` — game profile (id UUID, user_id FK, display_name, created_at, updated_at)
- `cars` — global car catalog (id UUID, name, base_stats JSONB, created_at)
- `player_cars` — ownership (id UUID, player_id FK, car_id FK, acquired_at, customizations JSONB, unique(player_id, car_id))
- `tracks` — track catalog (id UUID, name, layout JSONB, created_at)
- `races` — race metadata (id UUID, track_id FK, status enum, started_at, finished_at, created_at)
- `race_players` — race participation (id UUID, race_id FK, player_id FK, finish_position nullable, finished_at nullable, unique(race_id, player_id))
- `race_results` — final per-player results (id UUID, race_id FK, player_id FK, total_time_ms, best_lap_ms, position, created_at, unique(race_id, player_id))

### Do NOT
- Store real-time car positions or per-tick snapshots in Postgres.

## Deliverables

- [ ] `migrations/000001_init.up.sql`
- [ ] `migrations/000001_init.down.sql`
- [ ] Migrations wired to run on container startup (entrypoint script or `goose` invocation)
- [ ] `scripts/migrate.sh` for manual local runs (optional but useful)

## Acceptance Criteria

- Migration files are valid SQL (parse cleanly)
- `up` migration creates all 8 tables
- `down` migration drops them in FK-safe order
- On a fresh Postgres volume, applying all `up` migrations succeeds
- After migrations, `\dt` in psql lists all 8 tables

## Out of Scope

- Seeds (cars/tracks data can be added later)
- Versioned schema changes after MVP