BEGIN;

CREATE TABLE IF NOT EXISTS global_resources (
  id INTEGER PRIMARY KEY CHECK (id = 1),
  devs INTEGER NOT NULL DEFAULT 0 CHECK (devs >= 0),
  analysts INTEGER NOT NULL DEFAULT 0 CHECK (analysts >= 0),
  testers INTEGER NOT NULL DEFAULT 0 CHECK (testers >= 0),
  designers INTEGER NOT NULL DEFAULT 0 CHECK (designers >= 0),
  devops INTEGER NOT NULL DEFAULT 0 CHECK (devops >= 0),
  techlead INTEGER NOT NULL DEFAULT 0 CHECK (techlead >= 0),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO global_resources (id, devs, analysts, testers, designers, devops, techlead)
VALUES (1, 0, 0, 0, 0, 0, 0)
ON CONFLICT (id) DO NOTHING;

COMMIT;