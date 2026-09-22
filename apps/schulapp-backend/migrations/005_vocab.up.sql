-- Vokabeln (Lern-Bereich): Sets gehören einem Owner, können optional
-- einer Klasse geteilt werden. Karten wandern nach Leitner durch
-- Boxen 1..5 (fällig sofort / +1d / +3d / +7d / +14d).
CREATE TABLE vocab_sets (
    id          SERIAL PRIMARY KEY,
    owner_id    INT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    class_id    INT REFERENCES classes(id) ON DELETE SET NULL,
    title       TEXT NOT NULL,
    description TEXT,
    source_lang TEXT NOT NULL DEFAULT 'de',
    target_lang TEXT NOT NULL DEFAULT 'en',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE vocab_cards (
    id         SERIAL PRIMARY KEY,
    set_id     INT NOT NULL REFERENCES vocab_sets(id) ON DELETE CASCADE,
    front      TEXT NOT NULL,
    back       TEXT NOT NULL,
    hint       TEXT,
    box        INT NOT NULL DEFAULT 1 CHECK (box BETWEEN 1 AND 5),
    due_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_vocab_sets_owner ON vocab_sets(owner_id);
CREATE INDEX idx_vocab_sets_class ON vocab_sets(class_id);
CREATE INDEX idx_vocab_cards_set ON vocab_cards(set_id);
