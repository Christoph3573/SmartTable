DROP INDEX IF EXISTS idx_join_requests_student;
DROP INDEX IF EXISTS idx_join_requests_class;
DROP TABLE IF EXISTS class_join_requests;

ALTER TABLE users DROP CONSTRAINT IF EXISTS users_role_check;
UPDATE users SET role = 'admin' WHERE role = 'superadmin';
ALTER TABLE users ADD CONSTRAINT users_role_check CHECK (role IN ('student', 'teacher', 'admin'));

ALTER TABLE classes ALTER COLUMN school_id DROP NOT NULL;
ALTER TABLE classes DROP COLUMN IF EXISTS school_id;
ALTER TABLE users DROP COLUMN IF EXISTS school_id;

DELETE FROM schools WHERE name = 'Musterschule';
DROP TABLE IF EXISTS schools;
