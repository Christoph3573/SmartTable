ALTER TABLE homework_submissions ADD COLUMN file_id INT REFERENCES files(id) ON DELETE SET NULL;
