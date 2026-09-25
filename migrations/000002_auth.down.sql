BEGIN;

ALTER TABLE users
    DROP COLUMN IF EXISTS last_login_at;

ALTER TABLE players
    DROP COLUMN IF EXISTS avatar_url;

COMMIT;