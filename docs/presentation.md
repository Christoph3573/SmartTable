# SchulApp – Funktionsweise & Status

Diese Datei erklärt, wie der Code funktioniert, und dient als Grundlage für die Präsentation.

## 1. Was die App macht

Rollenbasierter Schulplaner für **Schüler**, **Lehrkräfte** und **Administration** mit:

- Klassen- und Nutzerverwaltung (inkl. Massenimport von bis zu 200 Konten)
- Stundenplan-Fächern
- Vertretungsplan (Ausfall, Vertretung, Raumänderung, Zusatztermin)
- Kalender/Termine (Ferien, Prüfungen, Events)
- Hausaufgaben inkl. Abgabe- und Bewertungsstatus
- Datei-/Ordnerverwaltung pro Klasse
- Klassen-, Gruppen- und Direktchat

## 2. Architektur

```
Browser (React SPA)
   │  fetch/axios, JWT im Access-Token
   ▼
Go-Backend (chi Router, Port konfigurierbar)
   │  SQL über sqlc-generierte Queries
   ▼
PostgreSQL 16
```

- **Frontend:** `apps/codeclub-ui` — React 19 + TypeScript + Vite + Tailwind, Routing über React Router v7, Server-State über React Query, Auth-State über Zustand.
- **Backend:** `apps/schulapp-backend` — Go + chi. Alle Endpunkte sind in `openapi.yaml` spezifiziert; `oapi-codegen` erzeugt daraus die Typen/Interfaces in `internal/api/generated.go`. Die eigentliche Logik steckt in `internal/api/handler/*.go` (2100+ Zeilen, alle Endpunkte real implementiert — die `Unimplemented`-Stubs in `generated.go` sind nur der ungenutzte Codegen-Fallback).
- **Datenbank:** PostgreSQL, Schema in `apps/schulapp-backend/migrations/001_initial_schema.up.sql`, SQL-Queries in `internal/db/query/*.sql`, typsicherer Zugriff über sqlc.

## 3. Auth & Zugriffskontrolle

- Login (`POST /api/v1/auth/login`) prüft E-Mail/Passwort (bcrypt-Hash) und liefert ein JWT (`Claims`: `user_id`, `email`, `role`).
- `internal/middleware/auth.go` validiert das Bearer-Token auf jeder geschützten Route und legt die Claims in den Request-Context.
- Öffentlich ohne Token: `/health`, `/api/v1/health`, `/api/v1/auth/login|refresh|logout`.
- Autorisierung ist zweistufig: Rolle (admin/teacher/student) **und** fachlicher Bezug (Klassen- bzw. Kanalmitgliedschaft) — ein gültiges Token einer fremden Klasse reicht nicht aus, um deren Daten zu lesen (siehe `internal/api/handler/access.go`).
- Login-Endpoint ist zusätzlich per Rate-Limiter gegen Brute-Force geschützt (5 Versuche/Minute, `golang.org/x/time/rate`).

## 4. Was tatsächlich getestet wurde (End-to-End über die laufende API)

| Flow | Ergebnis |
|---|---|
| Login (Admin, Schüler) | ✅ JWT korrekt ausgestellt |
| Klassen lesen/anlegen, Mitglieder/Lehrkräfte zuordnen | ✅ 200/201 |
| Fach anlegen | ✅ 201 |
| Termin anlegen/löschen | ✅ 201/204 |
| Vertretung anlegen | ✅ 201 |
| Hausaufgabe anlegen | ✅ 201 |
| Ordner anlegen | ✅ 201 |
| Datei-Upload (multipart) | ✅ 201 |
| Klassen-Chat anlegen + Nachricht senden/lesen | ✅ 201/200 |
| Rollenschutz: Schüler darf keine Nutzer anlegen | ✅ 403 |
| Zugriffsbindung: Schüler sieht nur eigene Klasse | ✅ korrekt gefiltert |

Backend-Build (`go build ./...`, `go vet ./...`) und Frontend-Build (`tsc -b && vite build`, `npm run lint`) laufen beide ohne Fehler/Warnungen.

## 5. Lokal starten

```bash
# 1) Postgres (Docker)
docker run -d --name schulapp-postgres \
  -e POSTGRES_USER=schulapp -e POSTGRES_PASSWORD=schulapp -e POSTGRES_DB=schulapp \
  -p 127.0.0.1:5434:5432 postgres:16-alpine

# 2) Migrationen
docker run --rm --network host \
  -v $(pwd)/apps/schulapp-backend/migrations:/migrations \
  migrate/migrate:v4.18.2 \
  -path /migrations -database "postgres://schulapp:schulapp@127.0.0.1:5434/schulapp?sslmode=disable" up

# 3) Backend
cd apps/schulapp-backend
DATABASE_URL="postgres://schulapp:schulapp@127.0.0.1:5434/schulapp?sslmode=disable" \
JWT_SECRET="change-me-in-production" \
PORT="8090" \
CORS_ORIGINS="http://localhost:5173" \
go run ./cmd/server

# 4) Seed-Testdaten
DATABASE_URL="postgres://schulapp:schulapp@127.0.0.1:5434/schulapp?sslmode=disable" go run ./seed

# 5) Frontend
cd ../codeclub-ui
VITE_API_BASE="http://localhost:8090" npm run dev
```

`CORS_ORIGINS` ist eine kommagetrennte Liste erlaubter Frontend-Origins (Default: `http://localhost:5173,http://localhost:3000`) — für Zugriff über Tailscale/LAN die jeweilige IP ergänzen, statt sie hart in den Code zu schreiben.

## 6. Test-Zugänge (Seed-Daten, `seed/main.go`)

| Rolle | E-Mail | Passwort |
|---|---|---|
| Admin | admin@schule.de | admin123 |
| Lehrkraft | mueller@schule.de / schmidt@schule.de / weber@schule.de | lehrer123 |
| Schüler | schueler1@schule.de … schueler10@schule.de | schueler123 |

Nur für lokale Entwicklung — kein Produktions-Seed.

## 7. Verzeichnisorientierung

```
apps/codeclub-ui/          React-Frontend
  src/pages/                Eine Seite pro Feature (auth, dashboard, admin, calendar, chat, files, homework, substitutions)
  src/api/                  Axios-Client + generierte Typen
  src/router/                Routing inkl. Protected/RoleRoute-Guards

apps/schulapp-backend/     Go-Backend
  cmd/server/main.go         Einstiegspunkt, Router-Setup, CORS, DB-Verbindung
  internal/api/handler/      Fachliche Logik (Auth, Klassen/Nutzer, Dateien/Chat, Stundenplan/Hausaufgaben, Fächer, Zugriffsprüfung)
  internal/middleware/       JWT-Auth-Middleware
  internal/db/query/         Rohe SQL-Queries (sqlc-Quelle)
  migrations/                 Datenbankschema
  seed/main.go                Entwicklungs-Testdaten
  openapi.yaml                 API-Vertrag (Quelle für Codegenerierung)

deploy/                    Ansible-Rollen für Raspberry-Pi-Deployment
docs/                       Architektur-/Betriebsdokumentation (diese Datei ergänzt implementation.md/overview.md)
```

## 8. Bekannte Lücken für den produktiven Einsatz

- Kein automatisiertes Test-Suite (`_test.go`) für Handler — bisher nur manuelle End-to-End-Prüfung über die laufende API.
- `docker-compose.yml` bindet Backend/Frontend auf feste Ports (80/8080) — bei lokalen Portkonflikten (wie hier: 8081 durch Fremd-Dienst belegt) manuell auf freie Ports ausweichen.
- WebSocket-Chat laut `docs/implementation.md` vorgesehen (`gorilla/websocket`), aktuell läuft Chat über klassisches Request/Response-Polling der REST-Endpunkte — kein Push in Echtzeit.
