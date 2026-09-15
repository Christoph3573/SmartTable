-- Schulen + Beitrittsanfragen (Schul-Hierarchie)
CREATE TABLE schools (
    id         SERIAL PRIMARY KEY,
    name       TEXT NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO schools (name) VALUES ('Musterschule') ON CONFLICT (name) DO NOTHING;

ALTER TABLE classes ADD COLUMN school_id INT REFERENCES schools(id);
ALTER TABLE users ADD COLUMN school_id INT REFERENCES schools(id);

-- Bestand der Musterschule zuordnen
UPDATE classes SET school_id = (SELECT id FROM schools WHERE name = 'Musterschule') WHERE school_id IS NULL;
UPDATE users SET school_id = (SELECT id FROM schools WHERE name = 'Musterschule') WHERE school_id IS NULL;

-- Klassen gehören immer zu einer Schule
ALTER TABLE classes ALTER COLUMN school_id SET NOT NULL;

-- Rollen erweitern: admin -> superadmin, neu: school_admin
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_role_check;
UPDATE users SET role = 'superadmin' WHERE role = 'admin';
ALTER TABLE users ADD CONSTRAINT users_role_check CHECK (role IN ('student', 'teacher', 'school_admin', 'superadmin'));

-- Superadmins gehören zu keiner Schule
UPDATE users SET school_id = NULL WHERE role = 'superadmin';

CREATE TABLE class_join_requests (
    id         SERIAL PRIMARY KEY,
    class_id   INT NOT NULL REFERENCES classes(id) ON DELETE CASCADE,
    student_id INT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status     TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'approved', 'rejected')),
    decided_by INT REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    decided_at TIMESTAMPTZ,
    UNIQUE (class_id, student_id)
);

CREATE INDEX idx_join_requests_class ON class_join_requests(class_id, status);
CREATE INDEX idx_join_requests_student ON class_join_requests(student_id, status);
