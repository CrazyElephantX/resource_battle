BEGIN;

ALTER TABLE app_settings
ADD COLUMN IF NOT EXISTS partner3_logo_filename TEXT,
ADD COLUMN IF NOT EXISTS partner3_logo_mime TEXT,
ADD COLUMN IF NOT EXISTS partner3_logo_data BYTEA;

COMMIT;