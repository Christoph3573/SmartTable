# Datenbankschema

## Rollen-Hierarchie

```
superadmin → school_admin → teacher → student
```

- **superadmin** (Plattform): Schulen + Schul-Admins anlegen, alles.
- **school_admin**: Lehrer der eigenen Schule anlegen, Klassen anlegen, Beitrittsanfragen der eigenen Schule freigeben.
- **teacher**: Klassen der eigenen Schule anlegen, Anfragen eigener Klassen freigeben.
- **student**: öffentliche Registrierung, Schule + Klasse wählen, Beitrittsanfrage stellen.

## Kern-Tabellen

```sql
schools         → id, name UNIQUE
users           → id, email, password, role (student|teacher|school_admin|superadmin), first_name, last_name, school_id → schools (NULL bei superadmin)
classes         → id, name, school_year, school_id → schools NOT NULL
class_members   → class_id, user_id
class_teachers  → class_id, user_id, is_home_teacher
class_join_requests → id, class_id, student_id, status (pending|approved|rejected), decided_by, UNIQUE(class_id, student_id)
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