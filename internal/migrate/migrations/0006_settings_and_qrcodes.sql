BEGIN;

CREATE TABLE IF NOT EXISTS app_settings (
  id INTEGER PRIMARY KEY,
  partner_logo_filename TEXT,
  partner_logo_mime TEXT,
  partner_logo_data BYTEA,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO app_settings(id)
VALUES (1)
ON CONFLICT (id) DO NOTHING;

CREATE TABLE IF NOT EXISTS qr_codes (
  id BIGSERIAL PRIMARY KEY,
  kind TEXT NOT NULL, -- 'author' or 'partner'
  title TEXT NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  image_filename TEXT,
  image_mime TEXT,
  image_data BYTEA NOT NULL,
  active BOOLEAN NOT NULL DEFAULT true,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS qr_codes_kind_active_idx
  ON qr_codes(kind, active);

COMMIT;

