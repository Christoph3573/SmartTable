INSERT INTO users (email, password_hash, first_name, last_name, role)
VALUES ('admin@schule.de', '$2a$12$gD6nu0ThgHpmlIZDY2NT3um1pKZSF7YLNVBKRCprC49UA35WJXhMy', 'Admin', 'Schule', 'admin')
ON CONFLICT (email) DO UPDATE SET password_hash = EXCLUDED.password_hash;
