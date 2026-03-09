BEGIN;

-- tasks.title is not unique, so we seed via "insert if missing" + update.
INSERT INTO tasks(title, description, points, repeatable, active)
SELECT 'Плюс балл', 'Повторяемая задача: добавляет 1 балл', 1, true, true
WHERE NOT EXISTS (SELECT 1 FROM tasks WHERE title = 'Плюс балл');

INSERT INTO tasks(title, description, points, repeatable, active)
SELECT 'Минус балл', 'Повторяемая задача: отнимает 1 балл', -1, true, true
WHERE NOT EXISTS (SELECT 1 FROM tasks WHERE title = 'Минус балл');

UPDATE tasks
SET description = 'Повторяемая задача: добавляет 1 балл',
    points = 1,
    repeatable = true,
    active = true,
    updated_at = now()
WHERE title = 'Плюс балл';

UPDATE tasks
SET description = 'Повторяемая задача: отнимает 1 балл',
    points = -1,
    repeatable = true,
    active = true,
    updated_at = now()
WHERE title = 'Минус балл';

COMMIT;

