# Smart Table

`apps/codeclub-ui` (React 19 + Vite + TS) und `apps/schulapp-backend` (Go + chi + Postgres) — Roadmap und Architektur-Docs in [`docs/`](docs/).

**Deploy:** Push nach `develop` deployt auf **pi-christoph** (Dev), Push nach `main` auf **pi-emanuel** (Prod). Jeder Host fährt den kompletten Stack als Docker Compose (eigene Container/Netz/Volumes/Ports — siehe `deploy/roles/schulapp_docker/`), damit sich Dev/Prod und andere Dienste auf dem Host nicht in die Quere kommen.

Pipeline pro Deploy: Quellen per Ansible syncen → Docker-Images bauen (Frontend, Backend, SchoolConnect-Sidecar) → `docker compose up` → Migrationen (`migrate`-One-Shot) → Backend-Healthcheck → Frontend-nginx auf `127.0.0.1:<tunnel_local_port>` → per autossh-Tunnel öffentlich erreichbar.

Rollout ist bewusst inkrementell nach der 8-Phasen-Roadmap (siehe `docs/`) — jede Phase bleibt für sich deploybar und wird einzeln verifiziert, bevor die nächste startet.

## Architektur-Übersicht

### Request-Flow

```
Browser ──https──► Tunnel-VM (codeclub.check24.fun)
                        │  autossh Reverse-Tunnel
                        ▼
              Raspberry Pi (127.0.0.1:<tunnel_local_port>)
                        │
                  [frontend/nginx]
                   /api/* │  / (SPA)
                          ▼
                  [backend :8080] (Go + chi, JWT)
                   ┌──────┴──────┐
                   ▼             ▼
          [postgres :5432]  [schoolconnect :8081]
           System-of-       nur internes Netz,
           Record           kein Host-Port
```

### Wie Frontend und Backend zusammenhängen

- Das **Frontend** (`apps/codeclub-ui`) wird als statische SPA gebaut (`vite build`) und vom `frontend`-Container per nginx ausgeliefert. `VITE_API_BASE=""`, also laufen alle API-Calls relativ (`/api/v1/...`) und landen über `location /api/ { proxy_pass http://backend:8080/api/; }` beim Backend — kein CORS-Problem, keine absolute Backend-URL im Build.
- Das **Backend** (`apps/schulapp-backend`) hört auf `:8080` und bedient alles unter `/api/v1/*` (Auth mit JWT + HttpOnly-Refresh-Cookie, Stundenplan, Vertretungen, Hausaufgaben, Chat, Kalender, Dateien, Vokabeln) plus `GET /health` und den Chat-WebSocket `GET /ws?token=...`.
- **Chat** ist live: Das Frontend hält einen WebSocket (`useChatSocket`) offen und schreibt eingehende Nachrichten direkt in den React-Query-Cache; ungelesene Zähler kommen aus `GET /api/v1/channels`.
- **SchoolConnect** (optionaler Data-Provider für Stundenplan/Vertretungen/Hausaufgaben, plus LehrplanPLUS für den Lernplan) läuft als Sidecar `schoolconnect serve` (v0.3.0, `SC_REQUIRE_TENANT=true`) **nur im internen Compose-Netz**. Das Frontend redet nie direkt mit ihm — es geht immer über `GET/POST /api/v1/integrations/schoolconnect/...` ans Backend, und das Backend setzt `X-SC-Tenant` aus der JWT-`user_id` (pro App-Benutzer isoliert, optional HMAC via `SC_TENANT_SHARED_SECRET`). Chat/Kalender/Dateien bleiben immer bei SmartTable.
- **Lern-Bereich:** Die Sidebar trennt **Dashboard** (eigener Reiter), **Schule** (Stundenplan, Vertretungsplan, Kalender, Dateien, Hausaufgaben, Chat) und **Lernen** (Lernplan aus Stundenplan-Fächern + LehrplanPLUS, Vokabeln mit Leitner-Abfrage in `vocab_sets`/`vocab_cards`, KI-Chat). Der **KI-Chat** spricht später mit `opencode serve` als eigenem Compose-Service (`opencode`, nur internes Netz) über `POST /api/v1/integrations/opencode/chat` — bis dahin antwortet die Seite lokal (fällige Vokabeln + nächste Hausaufgaben); der Service ist in beiden Compose-Files vorbereitet, aber auskommentiert.
- **Datenhaltung:** Postgres ist System-of-Record (User, Klassen, Stundenpläne, Chat, Dateien-Metadaten, Vokabelsets/-karten). Uploads liegen im Volume `uploads_data` (`UPLOAD_DIR=/app/uploads`), SchoolConnect-Credentials/Sessions im Volume `sc_creds` (`/data`), getrennt pro Tenant.

### Docker-Container (ein Compose-Stack pro Host)

| Container | Image/Build | Ports (Host) | Aufgabe |
|-----------|-------------|--------------|---------|
| `postgres` | `postgres:16-alpine` | `127.0.0.1:<postgres_host_port> (25432) → 5432` | Datenbank, Volume `postgres_data`, Healthcheck `pg_isready` |
| `migrate` | `migrate/migrate:v4.18.2` | keine | One-Shot: fährt SQL aus `apps/schulapp-backend/migrations` hoch (`service_completed_successfully`), danach exited |
| `backend` | `apps/schulapp-backend/Dockerfile` | keine (nur intern `:8080`) | Go-API; braucht `DATABASE_URL`, `JWT_SECRET`, `SCHOOLCONNECT_BASE_URL=http://schoolconnect:8081`; wartet auf `migrate` + `schoolconnect` |
| `schoolconnect` | `apps/schoolconnect/Dockerfile` (Release-Binary v0.3.0) | keine (nur intern `:8081`) | Externe Schulplattformen (Schülerportal, mebis, ByCS, LehrplanPLUS); Volume `sc_creds` → `/data`; `SC_REQUIRE_TENANT=true` |
| `opencode` (später, aktuell auskommentiert) | `apps/opencode/Dockerfile` (noch anzulegen) | keine (nur intern `:8082`) | `opencode serve` für den KI-Lernchat; Volume `opencode_data` → `/data`; Proxy `POST /api/v1/integrations/opencode/chat` noch zu verdrahten |
| `frontend` | `apps/codeclub-ui/Dockerfile` (node-build → `nginx:alpine`) | `127.0.0.1:<tunnel_local_port> → 80` | Liefert `dist/` aus, proxied `/api/` ans Backend |

Volumes: `postgres_data` (DB), `uploads_data` (Datei-Uploads), `sc_creds` (SchoolConnect-Sessions pro Tenant), später `opencode_data`. Lokal: `docker compose up -d` (Frontend dann auf `http://localhost/`); Prod/Dev: identischer Stack, nur andere Ports/Secrets pro Host (`deploy/inventory/host_vars/<host>/vars.yml`).

## Deployment-Infrastruktur

Die Ansible-Playbooks und Rollen für den Raspberry-Pi-Deploy liegen in [`deploy/`](deploy/).

### Architektur (Host / Tunnel)

```
Laptop  ──ansible──►  Raspberry Pi ──autossh──►  codeclub.check24.fun
                            │                    (öffentlich erreichbar)
              [frontend :<tunnel_local_port>] ◄── Reverse-Tunnel
                            │
        [backend :8080] [postgres] [schoolconnect :8081]
        (nur Compose-intern, kein öffentlicher Port)
```

### Rollen

| Rolle | Was sie tut |
|-------|------------|
| `schulapp_docker` | Synct Backend-/Frontend-/SchoolConnect-Quellen, rendert `docker-compose.yml` + `.env` und fährt den Compose-Stack (inkl. Migrationen) hoch |
| `ssh_tunnel` | Installiert autossh, richtet den Reverse-Tunnel zur Code-Club-VM als systemd-Service ein |
| `schoolconnect.systemd.disabled` | Archivierte alte Rolle (SchoolConnect-v0.1.0-Binary als systemd-Service) — nicht mehr im Deploy |

### Schnellstart

```bash
cd deploy/

# Inventory anpassen
# inventory/hosts  →  ansible_host / ansible_user auf den eigenen Pi setzen

# Abhängigkeiten
pip install ansible
ansible-galaxy collection install ansible.posix community.docker community.general

# Deployen (GitHub Actions macht das automatisch:
# develop → pi-christoph, main → pi-emanuel)
ansible-playbook deploy.yml -i inventory
```

### Voraussetzungen

- Ansible ≥ 2.14 (+ Collections `ansible.posix`, `community.docker`, `community.general`)
- SSH-Zugang zum Raspberry Pi (Key-Auth empfohlen)
- Auf dem Pi: Debian/Ubuntu-basiertes OS mit `apt`
- SSH-Key `~/.ssh/id_tunnel` (bzw. `tunnel_user`-Home) für den Tunnel bereits auf dem Pi hinterlegt

### Projektstruktur

```
deploy/
├── deploy.yml                    # Haupt-Playbook (schulapp_docker + ssh_tunnel)
├── provision.yml                 # Optionales zweites Playbook-Gerüst
├── inventory/
│   ├── hosts                     # Host-Gruppe webservers
│   ├── group_vars/all.yml        # Gemeinsame Variablen
│   └── host_vars/<host>/
│       └── vars.yml              # Ports, Secrets, Tunnel-Ziele (pro Host)
├── roles/
│   ├── schulapp_docker/          # Compose-Stack (Backend, Frontend, Postgres, SchoolConnect)
│   ├── schoolconnect.systemd.disabled/  # Archivierte alte SchoolConnect-Rolle
│   └── ssh_tunnel/               # autossh Reverse-Tunnel
```

### Secrets

Keine neuen GitHub Secrets nötig — der Deploy läuft über die bereits vorhandenen SSH-Secrets (`SSH_PRIVATE_KEY`, `VM_HOST`, `VM_USER`, `PI_TUNNEL_PORT`, `PI_USER`, `EMANUEL_*`). `jwt_secret` und `postgres_password` liegen direkt in `inventory/host_vars/<host>/vars.yml` (pro Host unterschiedlich, nicht geteilt). Kein `ansible-vault` im Einsatz.

## Lokale Entwicklung

```bash
docker compose up -d
```
