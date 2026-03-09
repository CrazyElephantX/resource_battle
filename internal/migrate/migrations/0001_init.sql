BEGIN;

CREATE TABLE IF NOT EXISTS teams (
  id BIGSERIAL PRIMARY KEY,
  name TEXT NOT NULL UNIQUE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS tasks (
  id BIGSERIAL PRIMARY KEY,
  title TEXT NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  points INTEGER NOT NULL,
  repeatable BOOLEAN NOT NULL DEFAULT false,
  active BOOLEAN NOT NULL DEFAULT true,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS tasks_active_idx ON tasks(active);

CREATE TABLE IF NOT EXISTS rounds (
  id BIGSERIAL PRIMARY KEY,
  number INTEGER NOT NULL UNIQUE CHECK (number > 0),
  name TEXT NOT NULL DEFAULT '',
  started_at TIMESTAMPTZ,
  ended_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS completions (
  id BIGSERIAL PRIMARY KEY,
  team_id BIGINT NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
  round_id BIGINT NOT NULL REFERENCES rounds(id) ON DELETE CASCADE,
  task_id BIGINT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
  count INTEGER NOT NULL DEFAULT 1 CHECK (count >= 1),
  completed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (team_id, round_id, task_id)
);

CREATE INDEX IF NOT EXISTS completions_team_idx ON completions(team_id);
CREATE INDEX IF NOT EXISTS completions_round_idx ON completions(round_id);
CREATE INDEX IF NOT EXISTS completions_task_idx ON completions(task_id);

COMMIT;

