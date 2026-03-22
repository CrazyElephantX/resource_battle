BEGIN;

ALTER TABLE qr_codes
ADD COLUMN updated_at TIMESTAMPTZ NOT NULL DEFAULT now();

-- Обновляем существующие записи, чтобы updated_at совпадало с created_at
UPDATE qr_codes
SET updated_at = created_at
WHERE updated_at IS NULL;

COMMIT;