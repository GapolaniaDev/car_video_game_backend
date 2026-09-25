-- Seed catalog data + give every existing player a starter car.
--
-- Idempotent: ON CONFLICT (name) DO NOTHING on inserts; the default
-- car grant only targets players that do not already own one.

BEGIN;

-- ─── car catalog ────────────────────────────────────────────────────
INSERT INTO cars (id, name, base_stats) VALUES
    ('11111111-1111-1111-1111-111111111111', 'Veloce',
     '{"top_speed": 220, "acceleration": 12.0, "handling": 0.7}'),
    ('22222222-2222-2222-2222-222222222222', 'Bruiser',
     '{"top_speed": 180, "acceleration": 8.0,  "handling": 0.5}'),
    ('33333333-3333-3333-3333-333333333333', 'Drifter',
     '{"top_speed": 200, "acceleration": 10.0, "handling": 0.9}')
ON CONFLICT (name) DO NOTHING;

-- ─── track catalog ──────────────────────────────────────────────────
INSERT INTO tracks (id, name, layout) VALUES
    ('aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 'Crescent Bay',
     '{"waypoints": [{"x":0,"y":0,"z":0},{"x":1000,"y":0,"z":0},{"x":1000,"y":0,"z":1000},{"x":0,"y":0,"z":1000}]}'),
    ('bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb', 'Granite Pass',
     '{"waypoints": [{"x":0,"y":0,"z":0},{"x":500,"y":0,"z":500},{"x":1000,"y":0,"z":0},{"x":500,"y":0,"z":-500}]}')
ON CONFLICT (name) DO NOTHING;

-- ─── grant every existing player a default Veloce ───────────────────
INSERT INTO player_cars (player_id, car_id, customizations)
SELECT p.id, '11111111-1111-1111-1111-111111111111', '{}'::jsonb
FROM players p
WHERE NOT EXISTS (
    SELECT 1 FROM player_cars pc WHERE pc.player_id = p.id
);

COMMIT;