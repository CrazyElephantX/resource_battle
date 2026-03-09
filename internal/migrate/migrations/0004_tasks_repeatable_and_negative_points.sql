BEGIN;

-- Allow negative points (e.g. "Минус балл")
ALTER TABLE tasks DROP CONSTRAINT IF EXISTS tasks_points_check;

-- Special tasks can be done multiple times.
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS repeatable BOOLEAN NOT NULL DEFAULT false;

-- Completions may represent multiple occurrences for repeatable tasks.
ALTER TABLE completions ADD COLUMN IF NOT EXISTS count INTEGER NOT NULL DEFAULT 1;
ALTER TABLE completions DROP CONSTRAINT IF EXISTS completions_count_check;
ALTER TABLE completions ADD CONSTRAINT completions_count_check CHECK (count >= 1);

COMMIT;

