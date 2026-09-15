-- Stundenplan: wiederkehrende wöchentliche Unterrichtsslots.
CREATE TABLE lessons (
    id           SERIAL PRIMARY KEY,
    class_id     INT NOT NULL REFERENCES classes(id) ON DELETE CASCADE,
    subject_id   INT NOT NULL REFERENCES subjects(id),
    teacher_id   INT NOT NULL REFERENCES users(id),
    day_of_week  INT NOT NULL CHECK (day_of_week BETWEEN 1 AND 5), -- 1=Montag..5=Freitag
    period       INT NOT NULL CHECK (period >= 1),
    room         TEXT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    -- Ein Klassen-Slot (Tag+Stunde) darf nicht doppelt belegt werden.
    UNIQUE (class_id, day_of_week, period)
);

CREATE INDEX idx_lessons_class ON lessons(class_id);
CREATE INDEX idx_lessons_teacher ON lessons(teacher_id);
