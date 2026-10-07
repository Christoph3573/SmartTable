# SchoolConnect als wählbarer Data-Provider — Plan (kompakt)

**Ziel:** Der User wählt in den Einstellungen die Datenquelle für
Stundenplan, Vertretungsplan und Hausaufgaben: `SmartTable` (eigenes
System, Default) oder `SchoolConnect` (Schülerportal & Co. via Proxy).
Chat, Kalender, Dateien bleiben immer SmartTable.

**Prinzip:** Keine zweite User-Verwaltung in SchoolConnect. SmartTable
bleibt System-of-Record (JWT). SchoolConnect wird nur
**mandantenfähig**: Session-Trennung pro App-User via Tenant-Key,
den das Backend aus dem JWT setzt.

```text
Frontend (Provider-Toggle, localStorage)
  │  JWT
  ▼
Backend (:8080, Compose) ── X-SC-Tenant: <user_id> ──▶ SC-Sidecar (:8081, nur internes Netz)
  │                                                          tenant → plugin → creds + Cookie-Jar
  ▼
PostgreSQL / Uploads
```

---

## 1. SchoolConnect ändern (Upstream, v0.1.0 → v0.2.0)

Problem heute: Store-Key ist nur `plugin-id`
(`~/.config/schoolconnect/credentials.json`), ein Cookie-Jar pro
Plugin-Prozess, REST-Adapter leitet keinen Tenant weiter → global
geteilt, Last-Login-wins.

**1a. `internal/core/session/session.go` — Store pro Tenant**
- Format: `map[tenant]map[plugin]map[key]value` statt `map[plugin]creds`.
- Migration: altes Format einmalig unter Tenant `_legacy` übernehmen.
- Signaturen: `Get(tenant, plugin)`, `Set(tenant, plugin, creds)`,
  `Delete(tenant, plugin)`. Datei-Rechte 0600/0700 behalten,
  read-through bleibt (CLI/REST/MCP teilen den Stand).

**1b. `internal/app/runtime.go` — Tenant durchreichen**
- `Authenticate(ctx, tenant, plugin, args)`,
  `Logout(ctx, tenant, plugin)`,
  `Call(ctx, tenant, plugin, fn, args)`,
  `StoredKeys(tenant, plugin)`, `PreviewMerge(tenant, …)`,
  `mergeCreds(tenant, …)`.
- Merge-Reihenfolge unverändert: Defaults < Store(tenant) < Env < Args.
- Antwort-Envelopes unverändert (nur Key-Namen, nie Secrets).

**1c. `plugins/schuelerportal|mebis|bycs-drive/plugin.go` — Jars pro Tenant**
- Heute: ein `client.Jar` + `authed/schule/email/secret` pro Prozess.
- Neu: `sessions map[tenant]*sess` mit eigenem `cookiejar` je Tenant
  (`mu` global + pro Eintrag).
- `ensureAuth/login/getAPI/doAPI` bekommen `tenant`; Re-Login (401/419)
  nutzt nur Tenant-eigene Creds. `logout` löscht Store **und**
  In-Memory-Session des Tenants.

**1d. `internal/adapters/rest/rest.go` — Tenant-Header lesen**
- Header `X-SC-Tenant: <smarttable-user-id>` auswerten
  (trimmen, validieren `^[A-Za-z0-9-_:]{1,128}$`).
- An Runtime übergeben für `auth/logout/call`.
- Fehlt der Header → `401 {code: tenant_required}`.
- Ausnahmen tenant-los: `GET /api` (Index), `/healthz`,
  Plugin `lernplan-bayern` (kein Login).
- Optional: `X-SC-Tenant-Sig` (HMAC mit Shared Secret) prüfen.

**1e. `internal/core/config` — zwei neue Envs**
- `SC_REQUIRE_TENANT=true`, `SC_TENANT_SHARED_SECRET=<secret>`.
  `REST_ADDR`, `SCHOOLCONNECT_CONFIG_DIR`, `LOG_LEVEL` bleiben.
- `cmd/schoolconnect/main.go`: kein Logikumbau, nur Config-Check +
  Log „tenant-mode on".

**Explizit nicht bauen:** keine `users`-Tabelle, kein JWT, keine
Rollen/Registrierung. Das bleibt in SmartTable.

## 2. SmartTable ändern (dieses Repo)

- **Backend** (`handler/schoolconnect.go`, `cmd/server/main.go`):
  `schoolConnectDo` setzt `X-SC-Tenant` aus JWT-`user_id` (aus
  `claims`, nie aus Client-Param) + optional HMAC. Default
  `SCHOOLCONNECT_BASE_URL=http://schoolconnect:8081`. `status`
  pro User abfragen. Doku „pro User isoliert".
- **Compose** (`docker-compose.yml` + `deploy/roles/schulapp_docker/…`):
  neuer Service `schoolconnect` (Image v0.2.0, **kein** `ports:`,
  Volume `sc_creds`), Backend hängt davon ab,
  `extra_hosts host.docker.internal` entfernen. systemd-Rolle
  `schoolconnect` aus `deploy.yml` entfernen/archivieren.
- **Frontend:** bereits fertig (`settingsStore`, `schoolconnect.ts`,
  `SettingsDialog`, Pages mit `enabled: isExternal`, 401→Settings,
  schreibgeschützt). Nur Texte auf „pro Benutzer isoliert" ziehen.

## 3. Akzeptanz

1. Zwei App-User gleichzeitig mit eigenem Schülerportal-Login —
   keine Vermischung, Logout trifft nur die eigene Session.
2. SC nur intern erreichbar (`http://schoolconnect:8081`), kein
   Host-Gateway mehr.
3. Provider-Umschalter ohne Reload; SC down → Hinweis + Fallback
   auf SmartTable.

## 4. Reihenfolge

1. Upstream v0.2.0 (Tenant-Isolation, Abschnitt 1).
2. Backend + Compose auf Sidecar umstellen (Abschnitt 2).
3. Lokal mit 2 Usern verifizieren, dann Deploy-Templates + Doku
   (`setup.md`, `api.md`, `deploy/README.md`) angleichen.
