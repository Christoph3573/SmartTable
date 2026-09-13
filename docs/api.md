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
GET    /     ?date_from=YYYY-MM-DD&date_to=YYYY-MM-DD&class_id=
POST   /     Eintrag erstellen (Lehrer/Admin)
PATCH  /:id  Bearbeiten
DELETE /:id  Löschen
```

## Kalender `/api/v1/events`

```
GET    /     ?start_date=&end_date=&class_id=
POST   /     Termin erstellen (Lehrer)
PATCH  /:id  Bearbeiten (Ersteller/Admin)
DELETE /:id  Löschen
```

## Dateien und Ordner

```
GET    /api/v1/classes/:id/folders          Ordnerstruktur
POST   /api/v1/classes/:id/folders          Ordner erstellen
DELETE /api/v1/folders/:id                  Ordner löschen
GET    /api/v1/classes/:id/files            Dateien lesen, optional ?folder_id=
POST   /api/v1/classes/:id/files            Datei hochladen (multipart/form-data)
GET    /api/v1/files/:id                    Datei herunterladen
DELETE /api/v1/files/:id                    Datei löschen
```

## Chat `/api/v1/chat`

```
GET    /chat/contacts                   Erreichbare Personen aus gemeinsamen Klassen
GET    /channels                        Eigene Kanäle
POST   /channels                        Direkt-, Gruppen- oder Klassenchat erstellen
GET    /channels/:id/messages           Nachrichten (Cursor-Pagination)
POST   /channels/:id/messages           Nachricht senden
```

Der Client aktualisiert eine geöffnete Unterhaltung im Zehn-Sekunden-Takt.
Direkt- und Gruppenchat-Empfänger werden serverseitig auf aktive Personen aus
gemeinsamen Klassen begrenzt; Admins können alle aktiven Konten erreichen.

## Hausaufgaben `/api/v1/homework`

```
GET    /api/v1/classes/:id/homework
POST   /api/v1/classes/:id/homework
PATCH  /api/v1/homework/:id
DELETE /api/v1/homework/:id
GET    /api/v1/homework/:id/submissions
POST   /api/v1/homework/:id/submissions
PATCH  /api/v1/submissions/:id           Abgabe benoten
```
