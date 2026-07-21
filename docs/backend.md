# Backend — SchulApp

## Architektur

```
Browser ── HTTPS ──> Nginx ──> /api/* ──> Go Backend (:8080) ──> PostgreSQL
                         └──> / (statisch) ──> Frontend (dist/)
```

Das Backend ist eine RESTful JSON API in **Go 1.24+** mit Chi-Router, JWT-Auth und PostgreSQL.

---

## OpenAPI-first Pipeline

Die `openapi.yaml` ist die **einzige Quelle der Wahrheit**:

```
                    ┌─ oapi-codegen ──→ schulapp-backend/internal/api/generated.go
openapi.yaml ──>    │                     (ServerInterface + Typen + Router)
   make generate    ├─ sqlc ─────────→ schulapp-backend/internal/db/generated/
                    │                     (typsichere DB-Queries)
                    └─ openapi-typescript → codeclub-ui/src/api/generated/types.ts
                                            (TypeScript-Typen)
```

**Workflow bei einem neuen Endpunkt:**

1. Endpunkt + Schema in `openapi.yaml` beschreiben
2. `make generate` (oapi-codegen + sqlc + TypeScript)
3. Generierte Methode auf `handler.Server` implementieren
4. `go vet ./...` + `go build ./...`

---

## Projektstruktur

```
apps/schulapp-backend/
├── cmd/server/main.go              # Entrypoint, Dependency Wiring
├── openapi.yaml                    # API-Spezifikation (Quelle der Wahrheit)
├── oapi-codegen.yaml               # Konfiguration für oapi-codegen
├── sqlc.yaml                       # Konfiguration für sqlc
├── Makefile                        # make generate, make run, make migrate
├── Dockerfile                      # Multi-Stage Build für Produktion
├── api_test.http                   # Manuelle REST-Tests (VS Code)
│
├── internal/
│   ├── api/
│   │   └── generated.go            # oapi-codegen Output — nicht manuell editieren
│   ├── api/handler/
│   │   ├── auth.go                 # Login, Refresh, Logout, Me, UpdateMe
│   │   ├── subjects.go             # subjects CRUD (Beispiel-Endpunkt)
│   │   ├── health.go               # Health-Check
│   │   └── helpers.go              # writeJSON / writeError
│   ├── db/
│   │   ├── query/                  # SQL-Queries für sqlc
│   │   │   ├── user.sql
│   │   │   ├── class.sql
│   │   │   ├── refresh_token.sql
│   │   │   ├── subject.sql
│   │   │   ├── substitution.sql
│   │   │   ├── event.sql
│   │   │   ├── homework.sql
│   │   │   ├── file.sql
│   │   │   └── chat.sql
│   │   └── generated/              # sqlc Output — nicht manuell editieren
│   │       ├── db.go, models.go
│   │       └── *.sql.go
│   └── middleware/
│       ├── auth.go                 # JWT validieren, Claims in Context
│       └── role.go                 # Rollen-Prüfung (student/teacher/admin)
│
├── migrations/                     # SQL-Migrationen (golang-migrate)
├── seed/
│   └── main.go                     # Testdaten: 1 Admin, 3 Lehrer, 10 Schüler
└── uploads/                        # Lokaler Datei-Speicher
```

---

## Handler implementieren

Jeder Handler lebt in einer eigenen Datei in `internal/api/handler/`. Das `Server`-Struct embeded `api.Unimplemented`, damit die Implementierung bei einer künftigen Erweiterung des OpenAPI-Vertrags kompiliert. Alle derzeit beschriebenen Endpunkte werden jedoch von konkreten Handlern überschrieben.

### Beispiel: Subjects

```go
package handler

import (
	"encoding/json"
	"net/http"
	"strings"

	"schulapp/internal/api"
	appmw "schulapp/internal/middleware"
)

// GET /api/v1/subjects
func (h *Server) GetApiV1Subjects(w http.ResponseWriter, r *http.Request) {
	// 1. Auth prüfen
	claims := appmw.GetClaims(r)
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "nicht autorisiert")
		return
	}

	// 2. DB-Query
	rows, err := h.DB.QueryContext(r.Context(),
		`SELECT id, name, short FROM subjects ORDER BY name`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Datenbankfehler")
		return
	}
	defer rows.Close()

	// 3. Ergebnis parsen
	subjects := []api.Subject{}
	for rows.Next() {
		var s api.Subject
		if err := rows.Scan(&s.Id, &s.Name, &s.Short); err != nil {
			writeError(w, http.StatusInternalServerError, "Fehler beim Lesen")
			return
		}
		subjects = append(subjects, s)
	}

	// 4. JSON-Antwort
	writeJSON(w, http.StatusOK, subjects)
}

// POST /api/v1/subjects
func (h *Server) PostApiV1Subjects(w http.ResponseWriter, r *http.Request) {
	claims := appmw.GetClaims(r)
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "nicht autorisiert")
		return
	}

	var req api.CreateSubjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "ungültiges JSON")
		return
	}
	if req.Name == "" || req.Short == "" {
		writeError(w, http.StatusBadRequest, "name und short dürfen nicht leer sein")
		return
	}

	var subject api.Subject
	err := h.DB.QueryRowContext(r.Context(),
		`INSERT INTO subjects (name, short) VALUES ($1, $2)
		 RETURNING id, name, short`,
		req.Name, req.Short,
	).Scan(&subject.Id, &subject.Name, &subject.Short)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Datenbankfehler")
		return
	}

	writeJSON(w, http.StatusCreated, subject)
}
```

---

## Fehlerfälle

| Fall | Status | Beispiel |
|---|---|---|
| Ungültiges JSON | `400` | `POST /auth/login` mit `{` |
| Validierungsfehler | `400` | `POST /subjects` mit leerem `name` |
| Falscher Login | `401` | Falsches Passwort |
| Fehlender/abgelaufener Token | `401` | `GET /auth/me` ohne `Authorization` |
| Rate Limit | `429` | 6. Login-Versuch innerhalb 1 Minute |
| DB-Fehler | `500` | Datenbank nicht erreichbar |
| Nicht implementiert | `501` | Nur bei einem künftig hinzugefügten, noch nicht überschriebenen OpenAPI-Endpunkt |

Jeder Handler folgt dem gleichen Muster:

```go
func (h *Server) MeinHandler(w http.ResponseWriter, r *http.Request) {
	// 1. Auth
	claims := appmw.GetClaims(r)
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "nicht autorisiert")
		return
	}

	// 2. Request parsen
	var req api.MeinRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "ungültiges JSON")
		return
	}

	// 3. Fachlogik + DB
	// ...

	// 4. Response
	writeJSON(w, http.StatusOK, result)
}
```

---

## Authentifizierung

Der JWT-Access-Token (15min) wird im `Authorization: Bearer <token>` Header gesendet. Der Refresh-Token (7d) liegt als HttpOnly-Cookie (`refresh_token`).

Der Auth-Check passiert **im Handler**, nicht als Middleware:

```go
claims := appmw.GetClaims(r)
if claims == nil {
    writeError(w, http.StatusUnauthorized, "nicht autorisiert")
    return
}
// claims.UserID, claims.Email, claims.Role verfügbar
```

Nur öffentliche Endpunkte (`login`, `refresh`, `logout`, `health`) lassen den Check weg.

---

## Manuelles Testen mit curl

```bash
# Health
curl -s https://example.com/health | jq .

# Login — Token holen
TOKEN=$(curl -s https://example.com/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"admin@schule.de","password":"admin123"}' | jq -r '.access_token')

# Eigenes Profil
curl -s https://example.com/api/v1/auth/me \
  -H "Authorization: Bearer $TOKEN" | jq .

# Subjects (leer)
curl -s https://example.com/api/v1/subjects \
  -H "Authorization: Bearer $TOKEN" | jq .

# Fach anlegen
curl -s -X POST https://example.com/api/v1/subjects \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"name":"Mathematik","short":"MA"}' | jq .
```

Alternativ die Datei `api_test.http` im VS Code REST Client öffnen und auf "Send Request" klicken.

---

## Migrationen

Neue Migration anlegen:

```bash
migrate create -ext sql -dir migrations -seq beschreibung
```

Enthält `up` (Schema-Änderung) und `down` (Rückgängig). Ausführen:

```bash
make migrate       # up
make migrate-down  # down
```

---

## Deployment (Ansible)

Der Deploy läuft über GitHub Actions → Ansible → Raspberry Pi:

1. Repo wird auf den Pi kopiert
2. `docker compose up --build` baut und startet alle Container
3. Migrationen laufen vor dem Backend-Start

Der Docker-Build läuft **auf dem Pi**. `make generate` wird dort **nicht** aufgerufen — die generierten Dateien müssen committed sein und werden mitkopiert.

---

## Abhängigkeiten

| Tool | Zweck | Installation |
|---|---|---|
| Go 1.24+ | Compiler | `apt install golang-1.24-go` |
| oapi-codegen | Go-Code aus OpenAPI | `go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest` |
| sqlc | Go-Code aus SQL | `go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest` |
| golang-migrate | DB-Migrationen | `go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest` |
