# SchulApp – Implementierung und Betrieb

## Zielbild

SchulApp ist ein rollenbasiertes Schulmanagement-System für Schülerinnen und
Schüler, Lehrkräfte und Administration. Die Anwendung bündelt den Schulalltag
an einem Ort: Klassen, Vertretungen, Termine, Hausaufgaben, Dateien und
Kurs-Chats. Jede Funktion ist über die REST-API an dieselbe PostgreSQL-Datenbank
angebunden; die Oberfläche enthält keine lokalen Demo-Datensätze.

## Funktionen nach Rolle

| Bereich | Schüler | Lehrkraft | Administration |
| --- | --- | --- | --- |
| Dashboard | Persönliche Übersicht | Persönliche Übersicht | Gesamtüberblick |
| Klassen | Zugewiesene Klassen lesen | Eigene Klassen verwalten | Alle Klassen verwalten |
| Vertretungen und Kalender | Relevante Einträge lesen | Einträge für eigene Klassen anlegen und pflegen | Schulweit verwalten |
| Hausaufgaben | Offene Aufgaben und Abgaben | Aufgaben und Abgaben der eigenen Klassen | Alle Aufgaben einsehen |
| Dateien | Dateien der eigenen Klassen lesen | Hochladen und verwalten | Vollzugriff |
| Chat | Eigene Kanäle und Nachrichten | Eigene Kanäle und Nachrichten | Eigene Kanäle und Moderation |
| Benutzer | – | – | Einzelne oder bis zu 200 Konten auf einmal anlegen, ändern und deaktivieren |

Backend-seitig werden Zugriffe nicht nur über die Rolle, sondern zusätzlich
über Klassen- oder Kanalmitgliedschaften geprüft. Ein gültiger Token allein
reicht deshalb nicht, um Daten einer fremden Klasse abzurufen.

## Architektur

```text
Browser
  │  React Router, Zustand, Axios
  ▼
Nginx/Caddy (gleiche Origin)
  ├── /        → Vite-Build (statische Dateien)
  └── /api/*   → Go/Chi-API (:8080)
                     │
                     ▼
                PostgreSQL 16
                     │
       Upload-Volume ◄──────┘
```

Das Frontend ist ein Client-Side-Rendering-Build. Das Go-Backend läuft als
separater Dienst, führt die Geschäftslogik aus und verwaltet Datenbank,
Autorisierung und Upload-Dateien. Docker Compose startet Datenbank, Migration,
Backend und Frontend gemeinsam. Die Ansible-Rolle im Ordner `deploy/` kopiert
den Quellstand auf den Raspberry Pi und startet den Stack reproduzierbar.

## API und Datenmodell

`apps/schulapp-backend/openapi.yaml` ist der verbindliche API-Vertrag. Daraus
entstehen das Go-Serverinterface und die TypeScript-Typen. Bei Änderungen an
Endpunkten gilt deshalb immer diese Reihenfolge:

1. OpenAPI-Spezifikation anpassen.
2. `make generate` ausführen und generierte Dateien einchecken.
3. Handler, SQL-Queries und Frontend-Client implementieren.
4. Backend und Frontend bauen bzw. testen.

Die API gliedert sich in:

- `auth`: Login, Token-Erneuerung, Logout und Profil.
- `users`, `classes`, `subjects`: Stammdaten und Klassenmitgliedschaften. Die
  Administration kann Klassenname und Schuljahr direkt pflegen, Schüler und
  Lehrkräfte zuordnen bzw. entfernen und beim Massenimport neu angelegte
  Konten unmittelbar einer Klasse zuweisen.
- `substitutions`, `events`: Tages- und Terminplanung.
- `homework`, `submissions`: Aufgaben und Abgabestatus.
- `files`, `folders`: Klassenmaterial mit persistentem Upload-Speicher.
- `channels`, `messages`: Mitgliedschaftsgebundene Kurskommunikation. Die
  Oberfläche erstellt Direkt-, Gruppen- und Klassenunterhaltungen; für
  Direkt- und Gruppenchat liefert die API ausschließlich aktive Kontakte aus
  gemeinsamen Klassen und prüft diese Einschränkung auch beim Anlegen.

Die PostgreSQL-Migration `001_initial_schema.up.sql` enthält die Tabellen und
Fremdschlüssel für diese Domänen. SQL-Abfragen liegen in
`internal/db/query/`; `sqlc` erzeugt daraus typsichere Go-Zugriffe.

## Anmeldung und Sicherheit

- Access Tokens sind kurzlebige JWTs und werden ausschließlich im
  Browser-Speicher gehalten.
- Der Refresh Token liegt als HttpOnly-Cookie vor; Axios sendet ihn mit
  `withCredentials` bei der Erneuerung mit.
- Öffentliche Routen sind Login, Refresh, Logout und Health. Alle fachlichen
  Routen verlangen Bearer-Authentifizierung.
- Schreiboperationen prüfen die Rolle und, falls nötig, Eigentümerschaft oder
  Klassenmitgliedschaft.
- Geheimnisse (`JWT_SECRET`, Datenbankpasswort und Deployment-Schlüssel)
  werden über Umgebungsvariablen bzw. GitHub Secrets übergeben und gehören
  nicht in den Client oder ins Repository.

Für Entwicklungsdaten dient `seed/main.go`. Es darf nicht als impliziter
Produktions-Seed verstanden werden.

## Lokale Entwicklung

### Voraussetzungen

- Go 1.24 oder neuer
- Node.js 20 oder neuer
- Docker mit Compose-Plugin für den vollständigen lokalen Stack
- Optional: `oapi-codegen`, `sqlc` und `golang-migrate` für Codegenerierung
  bzw. isolierte Backend-Arbeit

### Stack starten

```bash
docker compose up --build
```

Der Browser ruft den Stack über `http://localhost` auf. PostgreSQL ist absichtlich
nur an `127.0.0.1:5432` gebunden. Der Container `migrate` führt Migrationen vor
dem Backend-Start aus; die Volumes `postgres_data` und `uploads_data` erhalten
Zustand über Container-Neustarts hinweg.

Für getrennte Entwicklung:

```bash
# Terminal 1
docker compose up -d postgres migrate
cd apps/schulapp-backend && go run ./cmd/server

# Terminal 2
cd apps/codeclub-ui && npm run dev
```

Das Frontend erwartet standardmäßig die API unter `http://localhost:8080`; ein
abweichender Wert wird über `VITE_API_BASE` gesetzt. Das Backend erwartet
mindestens `DATABASE_URL` und `JWT_SECRET`; Uploads werden über `UPLOAD_DIR`
in ein persistentes Verzeichnis geschrieben. Siehe
`apps/schulapp-backend/.env.example`.

## Qualitätschecks

```bash
cd apps/schulapp-backend
go vet ./...
go build ./...

cd ../codeclub-ui
npm run lint
npm run build
```

Vor einem Release müssen neben den Builds die zentralen Rollenflüsse geprüft
werden: Login, Token-Refresh, Zugriff auf eigene/fremde Klassen, CRUD für
Termine und Vertretungen, Hausaufgabenabgabe, Datei-Upload/-Download sowie
Kanalmitgliedschaft im Chat.

## Deployment

Ein Push nach `develop` deployt über GitHub Actions auf die Entwicklungs-Pi;
ein Push nach `main` auf die Produktions-Pi. Der Workflow richtet die
Jump-Host-SSH-Konfiguration ein und führt anschließend das Ansible-Playbook
mit den jeweiligen Host-Variablen aus. Die Ansible-Rolle baut den Compose-Stack
auf dem Zielsystem und wartet danach auf einen erfolgreichen API-Login-Check.

Benötigte Secrets sind im Workflow dokumentiert, insbesondere SSH-Schlüssel,
VM-Login, Tunnel-Port und Zielnutzer. Sie dürfen weder in `.env`-Dateien noch
in einer Commit-Historie landen.

## Verzeichnisorientierung

```text
apps/codeclub-ui/             React-Oberfläche, Feature-API-Clients und Router
apps/schulapp-backend/        OpenAPI, Handler, Migrationen und SQL-Queries
deploy/                       Ansible-Rollen, Inventories und Compose-Templates
docs/                         Architektur- und Betriebsdokumentation
docker-compose.yml            Lokaler Gesamtstack
```

Diese Datei ergänzt die vorhandenen Unterlagen in `docs/` und ist die
Orientierung für Entwicklung, Review und Betrieb des vollständigen Projekts.
