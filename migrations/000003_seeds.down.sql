BEGIN;

-- Remove seeded player_car grants (only the ones pointing at our seed car).
DELETE FROM player_cars WHERE car_id = '11111111-1111-1111-1111-111111111111';

DELETE FROM tracks WHERE name IN ('Crescent Bay', 'Granite Pass');
DELETE FROM cars  WHERE name IN ('Veloce', 'Bruiser', 'Drifter');

COMMIT;