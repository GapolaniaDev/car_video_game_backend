-- Race state is authoritative on the Game Server. Postgres only sees
-- final results. This table is reserved for future replay/debug
-- tooling — Spec 20 does not write to it.
CREATE TABLE race_state_snapshots (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    race_id      UUID NOT NULL REFERENCES races(id) ON DELETE CASCADE,
    tick         BIGINT NOT NULL,
    snapshot     JSONB NOT NULL,
    captured_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_race_snap_race_id ON race_state_snapshots(race_id);
