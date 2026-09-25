-- Reverse the initial schema. Tables are dropped in FK-safe order.

BEGIN;

DROP TABLE IF EXISTS race_results    CASCADE;
DROP TABLE IF EXISTS race_players    CASCADE;
DROP TABLE IF EXISTS races           CASCADE;
DROP TABLE IF EXISTS tracks          CASCADE;
DROP TABLE IF EXISTS player_cars     CASCADE;
DROP TABLE IF EXISTS cars            CASCADE;
DROP TABLE IF EXISTS players         CASCADE;
DROP TABLE IF EXISTS users           CASCADE;

DROP FUNCTION IF EXISTS set_updated_at();

COMMIT;