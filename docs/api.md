# API-Endpunkte

## Auth `/api/v1/auth`

```
POST  /login      E-Mail + Passwort → Access + Refresh Token
POST  /refresh    Refresh Token → neues Access Token
POST  /logout     Refresh Token invalidieren
GET   /me         Eigenes Profil
PATCH /me         Eigenes Profil aktualisieren
```

## Benutzer `/api/v1/users` (Admin)

```
GET    /          Alle Benutzer
POST   /          Benutzer erstellen
POST   /bulk       Bis zu 200 Benutzer atomar erstellen
PATCH  /:id       Benutzer bearbeiten
DELETE /:id       Benutzer deaktivieren
```

`POST /bulk` erwartet ein Array von Benutzerobjekten mit E-Mail, Vorname,
Nachname, Rolle und Startpasswort. Scheitert ein Datensatz (beispielsweise an
einer bereits verwendeten E-Mail-Adresse), wird kein Benutzer aus diesem
Aufruf angelegt. Das UI kann die erfolgreich angelegten Schüler und Lehrkräfte
im selben Arbeitsschritt einer ausgewählten Klasse zuordnen.

## Klassen `/api/v1/classes`

```
GET    /                      Klassen (eigene für Schüler/Lehrer)
POST   /                      Klasse erstellen (Admin)
PATCH  /:id                   Klassenname und Schuljahr ändern (Admin)
DELETE /:id                   Klasse löschen (Admin)
GET    /:id/members            Schüler der Klasse auflisten
POST   /:id/members           Schüler hinzufügen
DELETE /:id/members/:uid      Schüler entfernen
GET    /:id/teachers           Lehrkräfte der Klasse auflisten
POST   /:id/teachers          Lehrer zuweisen
DELETE /:id/teachers/:uid     Lehrkraft entfernen
```

## Vertretungsplan `/api/v1/substitutions`

```
GET    /     ?date=YYYY-MM-DD&classId=
POST   /     Eintrag erstellen (Lehrer/Admin)
PATCH  /:id  Bearbeiten
DELETE /:id  Löschen
```

## Kalender `/api/v1/events`

```
GET    /     ?from=&to=&classId=
POST   /     Termin erstellen (Lehrer)
PATCH  /:id  Bearbeiten (Ersteller/Admin)
DELETE /:id  Löschen
```

## Dateien `/api/v1/files`

```
GET    /folders         Ordnerstruktur
POST   /folders         Ordner erstellen
POST   /upload          Datei hochladen (multipart/form-data)
GET    /:id/download    Datei herunterladen (Auth erforderlich)
DELETE /:id             Datei löschen
```

## Chat `/api/v1/chat`

```
GET    /channels                        Eigene Kanäle
POST   /channels                        Kanal/DM erstellen
GET    /channels/:id/messages           Nachrichten (Cursor-Pagination)
POST   /channels/:id/messages           Nachricht senden
PATCH  /channels/:id/read               Gelesen-Markierung
```

**WebSocket Events:** `send_message` → `new_message`, `typing` → `user_typing`, `mark_read`

## Hausaufgaben `/api/v1/homework`

```
GET    /                   ?classId=&subjectId=&upcoming=true
POST   /                   Aufgabe erstellen (Lehrer)
GET    /:id                Details
PATCH  /:id                Bearbeiten
POST   /:id/submissions    Abgabe einreichen (Schüler)
GET    /:id/submissions    Alle Abgaben (Lehrer)
PATCH  /:id/submissions/:sid  Bewerten (Lehrer)
```
