-- Таблица запросов ресурсов команд в спринте
CREATE TABLE IF NOT EXISTS round_resource_requests (
    id BIGSERIAL PRIMARY KEY,
    round_id BIGINT NOT NULL REFERENCES rounds(id) ON DELETE CASCADE,
    team_id BIGINT NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    devs INTEGER NOT NULL DEFAULT 0 CHECK (devs >= 0),
    analysts INTEGER NOT NULL DEFAULT 0 CHECK (analysts >= 0),
    testers INTEGER NOT NULL DEFAULT 0 CHECK (testers >= 0),
    designers INTEGER NOT NULL DEFAULT 0 CHECK (designers >= 0),
    devops INTEGER NOT NULL DEFAULT 0 CHECK (devops >= 0),
    techlead INTEGER NOT NULL DEFAULT 0 CHECK (techlead >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(round_id, team_id)
);

-- Индекс для быстрого поиска по раунду (идемпотентный)
CREATE INDEX IF NOT EXISTS idx_round_resource_requests_round_id ON round_resource_requests(round_id);