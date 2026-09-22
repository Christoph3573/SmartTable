# Installation & Deployment

## Voraussetzungen

- Go 1.22+
- Node.js 20+
- PostgreSQL (lokal oder Docker)
- `oapi-codegen`: `go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest`
- `sqlc`: `go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest`
- `golang-migrate`: `go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest`

## Backend

```bash
cd schulapp-backend

go mod tidy
make generate
make migrate
go run seed/main.go
go run cmd/server/main.go
```

Umgebungsvariablen (`.env`):
```env
DATABASE_URL=postgres://user:pass@localhost:5432/schulapp
JWT_SECRET=your-secret-here
PORT=8080
UPLOAD_DIR=./uploads
SCHOOLCONNECT_BASE_URL=http://schoolconnect:8081
# Optional: HMAC-Signatur für X-SC-Tenant (muss zum Sidecar passen).
SC_TENANT_SHARED_SECRET=
```

## SchoolConnect (optionaler Data-Provider)

Das Backend proxied lesende Aufrufe an die SchoolConnect-REST-API v0.3.0
(`GET /api/v1/integrations/schoolconnect/...`, siehe `docs/api.md`).
SchoolConnect läuft als Sidecar-Service `schoolconnect` im Compose-Netz
(nur internes Netz, kein Host-Port) mit `SC_REQUIRE_TENANT=true`; der
Proxy setzt `X-SC-Tenant` aus der JWT-`user_id` — Credentials +
Login-Sessions sind pro App-Benutzer isoliert (Logout trifft nur die
eigene Session). Ohne laufenden Sidecar meldet `GET .../status` schlicht
`{"reachable": false}` und die App bleibt auf dem SmartTable-Provider.

```bash
# Lokal ohne Compose (manueller Sidecar aus dem v0.3.0-Release):
# https://github.com/Christoph3573/SchoolConnect/releases/tag/v0.3.0
SC_REQUIRE_TENANT=true REST_ADDR=:8081 schoolconnect serve
# ... und SCHOOLCONNECT_BASE_URL=http://127.0.0.1:8081 setzen.
```

Der Deploy (`deploy/deploy.yml` → Rolle `schulapp_docker`) baut und
startet den Sidecar automatisch aus `apps/schoolconnect/Dockerfile` —
dort ist nichts manuell zu tun. Die alte systemd-Rolle ist archiviert
unter `deploy/roles/schoolconnect.systemd.disabled/`.

## Frontend

```bash
cd codeclub-ui

npx @openapitools/openapi-generator-cli generate \
  -i ../schulapp-backend/openapi.yaml \
  -g typescript-axios -o src/api/generated

npm install axios zustand @tanstack/react-query react-router-dom

npm run dev       # Entwicklung
npm run build     # Produktion → dist/
```

---

## Backend — Mini-Checkliste

- [ ] `GET /health` antwortet
- [ ] `GET /api/v1/items` liefert echte Daten
- [ ] `POST /api/v1/items` nimmt Daten an
- [ ] Datenbankanbindung funktioniert
- [ ] Fehlerfälle: ungültige Eingaben, nicht gefundene Daten, kaputte DB-Verbindung

---

## Deployment-Checkliste

```
Frontend (statische Dateien)   →  dist/ per Webserver ausliefern
Backend (Go Binary)            →  go build → einzelne Binary starten
Datenbank                      →  PostgreSQL-Dienst bereitstellen
```

- [ ] Frontend gebaut (`npm run build`) und erreichbar?
- [ ] Backend läuft als Prozess auf dem Server?
- [ ] Frontend kennt die richtige Backend-URL (Umgebungsvariable)?
- [ ] Backend kann die Datenbank erreichen?
- [ ] Daten bleiben nach einem Neustart erhalten?
- [ ] Ansible-Playbook deployt reproduzierbar?
- [ ] GitHub Action ruft das Playbook auf?
- [ ] Zugangsdaten als GitHub Secrets hinterlegt — nicht im Repository?
- [ ] Kurze README erklärt das automatisierte Deployment?