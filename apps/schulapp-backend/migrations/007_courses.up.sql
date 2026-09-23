-- Kurse: pro Benutzer auswählbare Unterrichtskurse.
--
-- Quellen:
--   provider = 'smarttable'    -> externe Kennung subject:<subject_id>
--   provider = 'schoolconnect' -> externe Kennung = Kurskürzel aus dem
--                                 Schülerportal-Stundenplan (z. B. "2m")
--   provider = 'custom'        -> selbst angelegter Kurs (external_key leer)
--
-- selected = false blendet den Kurs im Stundenplan/Vertretungsplan aus.
-- Nur explizit abgewählte Kurse werden gespeichert; fehlt eine Zeile, gilt
-- der Kurs als sichtbar (Opt-out statt Opt-in).
CREATE TABLE courses (
    id           SERIAL PRIMARY KEY,
    owner_id     INT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider     TEXT NOT NULL DEFAULT 'custom'
                 CHECK (provider IN ('smarttable', 'schoolconnect', 'custom')),
    -- NULL bei eigenen Kursen (mehrere erlaubt); bei externen Quellen gesetzt.
    external_key TEXT,
    name         TEXT NOT NULL,
    short        TEXT,
    color        TEXT,
    selected     BOOLEAN NOT NULL DEFAULT true,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (owner_id, provider, external_key)
);

-- Eigene Kursstunden (provider = 'custom'): wiederkehrende Slots wie lessons.
CREATE TABLE course_lessons (
    id          SERIAL PRIMARY KEY,
    course_id   INT NOT NULL REFERENCES courses(id) ON DELETE CASCADE,
    day_of_week INT NOT NULL CHECK (day_of_week BETWEEN 1 AND 5),
    period      INT NOT NULL CHECK (period >= 1),
    room        TEXT,
    UNIQUE (course_id, day_of_week, period)
);

CREATE INDEX idx_courses_owner ON courses(owner_id);
CREATE INDEX idx_course_lessons_course ON course_lessons(course_id);
