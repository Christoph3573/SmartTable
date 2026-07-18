# Smart Table

`apps/codeclub-ui` (React 19 + Vite + TS) und `apps/schulapp-backend` (Go + chi + Postgres) — Roadmap und Architektur-Docs in [`docs/`](docs/).

**Deploy:** Push nach `develop` deployt auf **pi-christoph** (Dev), Push nach `main` auf **pi-emanuel** (Prod). Jeder Host hat eine komplett eigenständige Postgres-Instanz (eigener Docker-Container/Netz/Volume/Port/DB-User — siehe `deploy/roles/postgres_docker/`), damit sich Dev/Prod und ggf. andere Dienste auf dem Host nicht in die Quere kommen.

Pipeline pro Deploy: Frontend bauen (`vite build`) → Backend für die Pi-Architektur cross-compilen (Go) → Ansible: Postgres-Container hoch → Migrationen laufen lassen → Go-Binary als systemd-Service deployen → Caddy liefert `dist/` aus und reverse-proxied `/api/*` aufs Backend.

Rollout ist bewusst inkrementell nach der 8-Phasen-Roadmap (siehe `docs/`) — jede Phase bleibt für sich deploybar und wird einzeln verifiziert, bevor die nächste startet.

## Deployment-Infrastruktur

Die Ansible-Playbooks und Rollen für den Raspberry-Pi-Deploy liegen in [`deploy/`](deploy/).

### Architektur

```
Laptop  ──ansible──►  Raspberry Pi
                           │
                    [Caddy :80]
                    [autossh] ──tunnel──►  smart-table.example.com
                                                │
                                           öffentlich erreichbar
```

### Rollen

| Rolle | Was sie tut |
|-------|------------|
| `caddy` | Installiert Caddy, deployt Caddyfile + statische Site nach `/var/www/mysite/` |
| `ssh_tunnel` | Installiert autossh, richtet einen systemd-Service als Reverse-Tunnel ein |

### Schnellstart

```bash
cd deploy/

# Inventory anpassen
# inventory/hosts  →  ansible_host / ansible_user auf den eigenen Pi setzen

# Abhängigkeiten
pip install ansible

# Deployen
ansible-playbook deploy.yml
```

### Voraussetzungen

- Ansible ≥ 2.14
- SSH-Zugang zum Raspberry Pi (Key-Auth empfohlen)
- Auf dem Pi: Debian/Ubuntu-basiertes OS mit `apt`
- SSH-Key `/home/pi/.ssh/id_tunnel` für den Tunnel bereits auf dem Pi hinterlegt

### Projektstruktur

```
deploy/
├── deploy.yml                    # Haupt-Playbook (caddy + ssh_tunnel)
├── provision.yml                 # Optionales zweites Playbook-Gerüst
├── inventory/
│   ├── hosts                     # Host-Gruppe webservers
│   ├── group_vars/all.yml        # Gemeinsame Variablen
│   └── host_vars/pi-example/
│       ├── vars.yml              # Host-spezifische Variablen
│       └── vault.yml             # Secrets (ansible-vault)
├── roles/
│   ├── caddy/                    # Caddy-Webserver
│   └── ssh_tunnel/               # autossh Reverse-Tunnel
└── site/                         # Statische Website (Template)
```

### Secrets

Keine neuen GitHub Secrets nötig — der Deploy läuft über die bereits vorhandenen SSH-Secrets (`SSH_PRIVATE_KEY`, `VM_HOST`, `VM_USER`, `PI_TUNNEL_PORT`, `PI_USER`, `EMANUEL_*`). `jwt_secret` und `postgres_password` liegen direkt in `inventory/host_vars/<host>/vars.yml` (pro Host unterschiedlich, nicht geteilt). Kein `ansible-vault` im Einsatz.

## Lokale Entwicklung

```bash
docker compose up -d
```
