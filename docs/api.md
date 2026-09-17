# API-Endpunkte

## Auth `/api/v1/auth`

```
POST  /login      E-Mail + Passwort → Access + Refresh Token
POST  /refresh    Refresh Token → neues Access Token
POST  /logout     Refresh Token invalidieren
POST  /register   Öffentliche Schüler-Registrierung (school_id + optional requested_class_id → 201 + ggf. Beitrittsanfrage)
GET   /me         Eigenes Profil
PATCH /me         Eigenes Profil aktualisieren
```

## Öffentlich (ohne Auth, für Registrierung)

```
GET /public/schools            Schulen auflisten
GET /public/classes?school_id= Klassen einer Schule auflisten
```

## Schulen `/api/v1/schools`

```
GET  /    Schulen (superadmin: alle; Personal: eigene)
POST /    Schule anlegen (nur superadmin)
```

## Zugehörigkeit

```
GET /me/membership   { school, classes[], pending[] } für Banner/Join-Flow
```

## Benutzer `/api/v1/users` (superadmin: alle Rollen; school_admin: nur teacher eigener Schule)

```
GET    /          Alle Benutzer (Scope: eigene Schule für Schul-Admins)
POST   /          Benutzer erstellen
POST   /bulk       Bis zu 200 Benutzer atomar erstellen
PATCH  /:id       Benutzer bearbeiten (nur superadmin ändert Rolle/Passwort)
DELETE /:id       Benutzer deaktivieren
```

`POST /bulk` erwartet ein Array von Benutzerobjekten mit E-Mail, Vorname,
Nachname, Rolle und Startpasswort. Scheitert ein Datensatz (beispielsweise an
einer bereits verwendeten E-Mail-Adresse), wird kein Benutzer aus diesem
Aufruf angelegt. Das UI kann die erfolgreich angelegten Schüler und Lehrkräfte
im selben Arbeitsschritt einer ausgewählten Klasse zuordnen.

## Klassen `/api/v1/classes`

```
GET    /                      Klassen (Schüler/Lehrer: eigene; school_admin: eigene Schule; superadmin: alle)
POST   /                      Klasse erstellen (superadmin|school_admin|teacher; school_id aus Profil)
PATCH  /:id                   Klassenname und Schuljahr ändern (Klassenverwaltung)
DELETE /:id                   Klasse löschen (superadmin|school_admin eigener Schule)
GET    /:id/members            Schüler der Klasse auflisten
POST   /:id/members           Schüler hinzufügen
DELETE /:id/members/:uid      Schüler entfernen
GET    /:id/teachers           Lehrkräfte der Klasse auflisten
POST   /:id/teachers          Lehrer zuweisen (superadmin|school_admin)
DELETE /:id/teachers/:uid     Lehrkraft entfernen
```

## Beitrittsanfragen

```
POST /classes/:id/join-requests  Anfrage stellen (Schüler der Klassenschule, idempotent)
GET  /classes/:id/join-requests  Offene Anfragen (Lehrer eigener Klasse, school_admin eigener Schule)
POST /join-requests/:id/approve  Freigeben (Transaktion → class_members)
POST /join-requests/:id/reject   Ablehnen
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

## SchoolConnect-Integration `/api/v1/integrations/schoolconnect`

Proxy auf die SchoolConnect-REST-API v0.1.0 (`schoolconnect serve`,
Binary + systemd-Service via Ansible-Rolle `schoolconnect`). Eigenes
JWT-Auth bleibt davor; nur lesende Plugin-Funktionen sind freigegeben
(Allowlist im Backend, `fetch`/Dateidownloads ausgenommen).

```
GET    /status                      Erreichbarkeit + Pluginliste ({reachable, plugins[]})
POST   /{plugin}/auth               Plugin-Login (Credentials im Body, Antwort ohne Secret-Werte)
POST   /{plugin}/logout             Plugin-Session verwerfen
GET    /{plugin}/{function}         Lesende Funktion (Query-Params werden gereicht)
POST   /{plugin}/{function}         Lesende Funktion (JSON-Body wird gereicht)
```

Freigegeben: `schuelerportal.{profil,stundenplan,hausaufgaben,vertretungsplan}`,
`mebis.{courses,abschnitte,inhalt}`, `bycs-drive.{spaces,list}`,
`lernplan-bayern.{search,details}` (kein Login nötig).
