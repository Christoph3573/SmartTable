# Deployment-Infrastruktur

Ansible-Playbooks und Rollen für den SmartTable-Deploy auf den Raspberry Pis.

## Inhalt

| Pfad | Zweck |
|------|--------|
| `inventory/hosts` | Zielhosts in der Gruppe `webservers` |
| `inventory/group_vars/all.yml` | Gemeinsame Variablen für App, Docker und Caddy |
| `inventory/host_vars/<host>/vars.yml` | Host-spezifische Ports, Pfade und Secrets |
| `deploy.yml` | Startet den vollständigen Compose-Stack und den SSH-Tunnel |
| `provision.yml` | Richtet die Betriebssystem-Basis des Hosts ein |
| `roles/schulapp_docker/` | Synchronisiert die Quellen und führt Docker Compose aus |
| `roles/schoolconnect/` | Installiert das SchoolConnect-v0.1.0-Binary und betreibt `serve` als systemd-Service |
| `roles/ssh_tunnel/` | Stellt den Reverse-Tunnel zur Code-Club-VM her |

## SchoolConnect (Data-Provider)

Die Rolle `schoolconnect` lädt das Release-Binary v0.1.0 von GitHub
(`schoolconnect-linux-arm64` für den Pi) nach `/opt/schoolconnect/` und
startet `schoolconnect serve` als systemd-Service auf
`127.0.0.1:8081` (nur Loopback — kein Tunnel/Caddy nötig, der Zugriff
läuft ausschließlich über das SmartTable-Backend).

- Variablen: siehe `roles/schoolconnect/defaults/main.yml`
  (`schoolconnect_version`, `schoolconnect_port`, optional
  `schoolconnect_sha256` für die Checksummenprüfung und
  `schoolconnect_env_secrets` für Plugin-Secrets wie
  `SCHUELERPORTAL_SECRET`).
- Das Backend erreicht SchoolConnect über `SCHOOLCONNECT_BASE_URL`
  (aus dem Container via `host.docker.internal`, siehe
  `docker-compose.yml.j2`); der direkte Systemd-Deploy nutzt
  `http://127.0.0.1:8081`.
- User-Logins (Schülerportal/mebis/ByCS) laufen pro Aufruf über die App
  (Einstellungen → SchoolConnect) — die SchoolConnect-Runtime verwaltet
  ihre Sessions selbst, im Backend wird nichts gespeichert.
- Smoke-Tests der Rolle: `schoolconnect list` + `GET /api` müssen 200 liefern.

## Schnellstart

1. Zielhost und Variablen im Inventory prüfen.
2. Auf dem Laptop die benötigten Collections installieren:

   ```bash
   pip install ansible
   ansible-galaxy collection install ansible.posix community.docker community.general
   ```

3. Einmalig die Hostbasis einrichten:

   ```bash
   ansible-playbook provision.yml -i inventory
   ```

4. Den Anwendungsstack ausrollen:

   ```bash
   ansible-playbook deploy.yml -i inventory
   ```

---

Siehe auch: [`../README.md`](../README.md) und
[`../docs/implementation.md`](../docs/implementation.md).
