-- Matchmaking session tokens (block 3).
-- Each row is one assignment of a player to a (pending) match. The
-- `game_token` is HMAC-signed by the API and verified locally by the
-- Game Server using the same secret. `consumed` flips true after the
-- Game Server accepts the JoinRaceRequest, blocking replay.
CREATE TABLE matchmaking_assignments (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    match_id    UUID        NOT NULL,
    player_id   UUID        NOT NULL REFERENCES players(id) ON DELETE CASCADE,
    race_id     UUID,                  -- NULL until the Game Server allocates a race
    game_token  TEXT        NOT NULL,
    expires_at  TIMESTAMPTZ NOT NULL,
    consumed    BOOLEAN     NOT NULL DEFAULT FALSE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_match_assign_player ON matchmaking_assignments(player_id);
CREATE INDEX idx_match_assign_match  ON matchmaking_assignments(match_id);
