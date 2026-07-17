CREATE TABLE users (
    id            SERIAL PRIMARY KEY,
    email         TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    first_name    TEXT NOT NULL,
    last_name     TEXT NOT NULL,
    role          TEXT NOT NULL CHECK (role IN ('student', 'teacher', 'admin')),
    active        BOOLEAN NOT NULL DEFAULT true,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE classes (
    id          SERIAL PRIMARY KEY,
    name        TEXT NOT NULL,
    school_year TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE class_members (
    class_id INT NOT NULL REFERENCES classes(id) ON DELETE CASCADE,
    user_id  INT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    PRIMARY KEY (class_id, user_id)
);

CREATE TABLE class_teachers (
    class_id        INT NOT NULL REFERENCES classes(id) ON DELETE CASCADE,
    user_id         INT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    is_home_teacher BOOLEAN NOT NULL DEFAULT false,
    PRIMARY KEY (class_id, user_id)
);

CREATE TABLE subjects (
    id    SERIAL PRIMARY KEY,
    name  TEXT NOT NULL,
    short TEXT NOT NULL
);

CREATE TABLE refresh_tokens (
    id         SERIAL PRIMARY KEY,
    user_id    INT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token      TEXT NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE substitutions (
    id                  SERIAL PRIMARY KEY,
    date                DATE NOT NULL,
    period              INT NOT NULL,
    class_id            INT REFERENCES classes(id),
    subject_id          INT REFERENCES subjects(id),
    original_teacher_id INT REFERENCES users(id),
    sub_teacher_id      INT REFERENCES users(id),
    room                TEXT,
    type                TEXT NOT NULL CHECK (type IN ('substitution', 'cancellation', 'room_change', 'extra')),
    note                TEXT,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE events (
    id         SERIAL PRIMARY KEY,
    title      TEXT NOT NULL,
    start_time TIMESTAMPTZ NOT NULL,
    end_time   TIMESTAMPTZ NOT NULL,
    all_day    BOOLEAN NOT NULL DEFAULT false,
    type       TEXT NOT NULL CHECK (type IN ('holiday', 'exam', 'event', 'other')),
    class_id   INT REFERENCES classes(id),
    creator_id INT REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE file_folders (
    id         SERIAL PRIMARY KEY,
    name       TEXT NOT NULL,
    parent_id  INT REFERENCES file_folders(id),
    class_id   INT REFERENCES classes(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE files (
    id          SERIAL PRIMARY KEY,
    name        TEXT NOT NULL,
    path        TEXT NOT NULL,
    size        BIGINT NOT NULL,
    mime_type   TEXT NOT NULL,
    uploader_id INT REFERENCES users(id),
    class_id    INT REFERENCES classes(id),
    folder_id   INT REFERENCES file_folders(id),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE chat_channels (
    id         SERIAL PRIMARY KEY,
    name       TEXT,
    type       TEXT NOT NULL CHECK (type IN ('direct', 'class', 'group')),
    class_id   INT REFERENCES classes(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE chat_members (
    channel_id   INT NOT NULL REFERENCES chat_channels(id) ON DELETE CASCADE,
    user_id      INT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    last_read_at TIMESTAMPTZ,
    PRIMARY KEY (channel_id, user_id)
);

CREATE TABLE messages (
    id         SERIAL PRIMARY KEY,
    channel_id INT NOT NULL REFERENCES chat_channels(id) ON DELETE CASCADE,
    sender_id  INT NOT NULL REFERENCES users(id),
    content    TEXT NOT NULL,
    file_id    INT REFERENCES files(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE homework (
    id          SERIAL PRIMARY KEY,
    title       TEXT NOT NULL,
    description TEXT,
    due_date    DATE NOT NULL,
    class_id    INT NOT NULL REFERENCES classes(id),
    subject_id  INT REFERENCES subjects(id),
    teacher_id  INT NOT NULL REFERENCES users(id),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE homework_submissions (
    id          SERIAL PRIMARY KEY,
    homework_id INT NOT NULL REFERENCES homework(id) ON DELETE CASCADE,
    student_id  INT NOT NULL REFERENCES users(id),
    status      TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'submitted', 'graded')),
    grade       NUMERIC(4,1),
    submitted_at TIMESTAMPTZ,
    graded_at   TIMESTAMPTZ,
    UNIQUE (homework_id, student_id)
);

CREATE INDEX idx_substitutions_date ON substitutions(date);
CREATE INDEX idx_substitutions_class ON substitutions(class_id);
CREATE INDEX idx_events_time ON events(start_time, end_time);
CREATE INDEX idx_messages_channel ON messages(channel_id, created_at);
CREATE INDEX idx_homework_class ON homework(class_id, due_date);
CREATE INDEX idx_refresh_tokens_token ON refresh_tokens(token);
