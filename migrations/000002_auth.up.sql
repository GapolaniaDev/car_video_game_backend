-- Auth-related schema additions.
--
--  * users.last_login_at — populated on every successful login.
--  * players.avatar_url — optional cosmetic URL.

BEGIN;

ALTER TABLE users
    ADD COLUMN IF NOT EXISTS last_login_at TIMESTAMPTZ;

ALTER TABLE players
    ADD COLUMN IF NOT EXISTS avatar_url TEXT;

COMMIT;