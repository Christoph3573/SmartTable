INSERT INTO users (email, password_hash, first_name, last_name, role)
VALUES ('admin@schule.de', '$2b$12$KTjhK7pKccXE6cjtjNR1fOh9lpZtxqM9a5QgX0M6hV7wJo9EQLxpu', 'Admin', 'Schule', 'admin')
ON CONFLICT (email) DO NOTHING;
