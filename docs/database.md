# Datenbankschema

## Kern-Tabellen

```sql
users           → id, email, password, role (student|teacher|admin), first_name, last_name
classes         → id, name, school_year
class_members   → class_id, user_id
class_teachers  → class_id, user_id, is_home_teacher
subjects        → id, name, short
refresh_tokens  → id, user_id, token, expires_at
```

## Feature-Tabellen

```sql
substitutions        → date, period, class_id, original_teacher_id, sub_teacher_id, room, type
events               → title, start_time, end_time, all_day, type, class_id (NULL = schulweit)
files                → name, path, size, mime_type, uploader_id, class_id, folder_id
file_folders         → name, parent_id (rekursiv), class_id
chat_channels        → type (direct|class|group), class_id
chat_members         → channel_id, user_id, last_read_at
messages             → channel_id, sender_id, content, file_id
homework             → title, due_date, class_id, subject_id, teacher_id
homework_submissions → homework_id, student_id, status (open|submitted|graded), grade
```