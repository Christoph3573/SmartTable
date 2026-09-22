-- OpenCode-KI-Chat (Lern-Bereich): Sessions gehören einem Owner (user_id),
-- die opencode-session-id liegt daneben. Nachrichten-Cache für Verlauf.
-- Ownership wird bei jedem Zugriff geprüft (Owner oder Superadmin).
CREATE TABLE opencode_sessions (
    id                SERIAL PRIMARY KEY,
    owner_id          INT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    opencode_id       TEXT NOT NULL,
    title             TEXT NOT NULL DEFAULT 'Lern-Chat',
    workspace         TEXT NOT NULL,
    model_provider    TEXT,
    model_id          TEXT,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE opencode_messages (
    id          SERIAL PRIMARY KEY,
    session_id  INT NOT NULL REFERENCES opencode_sessions(id) ON DELETE CASCADE,
    role        TEXT NOT NULL CHECK (role IN ('user', 'assistant')),
    content     TEXT NOT NULL DEFAULT '',
    tokens      INT NOT NULL DEFAULT 0,
    cost        DOUBLE PRECISION NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Kurzlebige Session-Token für den Backend-MCP-Server: OpenCode meldet sich
-- damit am MCP-Endpunkt (Header X-Session-Token); der Tenant (user_id) kommt
-- aus dieser Tabelle, nie aus Client-Parametern.
CREATE TABLE opencode_session_tokens (
    token      TEXT PRIMARY KEY,
    user_id    INT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_opencode_sessions_owner ON opencode_sessions(owner_id);
CREATE UNIQUE INDEX idx_opencode_sessions_ocid ON opencode_sessions(opencode_id);
CREATE INDEX idx_opencode_messages_session ON opencode_messages(session_id, id);
CREATE INDEX idx_opencode_session_tokens_user ON opencode_session_tokens(user_id);
