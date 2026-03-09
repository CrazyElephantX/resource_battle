BEGIN;

INSERT INTO teams(name) VALUES
  ('Команда 1'),
  ('Команда 2'),
  ('Команда 3'),
  ('Команда 4'),
  ('Команда 5'),
  ('Команда 6')
ON CONFLICT (name) DO NOTHING;

COMMIT;

