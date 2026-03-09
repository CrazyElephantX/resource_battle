BEGIN;

INSERT INTO rounds(number, name) VALUES
  (1, 'Раунд 1'),
  (2, 'Раунд 2'),
  (3, 'Раунд 3'),
  (4, 'Раунд 4')
ON CONFLICT (number) DO UPDATE SET name = EXCLUDED.name;

COMMIT;

