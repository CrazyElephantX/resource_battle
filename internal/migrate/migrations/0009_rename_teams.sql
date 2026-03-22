BEGIN;

UPDATE teams SET name = 'Команда A' WHERE name = 'Команда 1';
UPDATE teams SET name = 'Команда B' WHERE name = 'Команда 2';
UPDATE teams SET name = 'Команда C' WHERE name = 'Команда 3';
UPDATE teams SET name = 'Команда D' WHERE name = 'Команда 4';
UPDATE teams SET name = 'Команда E' WHERE name = 'Команда 5';
UPDATE teams SET name = 'Команда F' WHERE name = 'Команда 6';

COMMIT;