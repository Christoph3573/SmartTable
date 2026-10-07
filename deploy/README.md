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
| `roles/schulapp_docker/` | Synchronisiert die Quellen (Backend, Frontend, SchoolConnect-Sidecar) und führt Docker Compose aus |
| `roles/schoolconnect.systemd.disabled/` | Archivierte alte Rolle (SchoolConnect-v0.1.0-Binary als systemd-Service) — nicht mehr im Deploy |
| `roles/ssh_tunnel/` | Stellt den Reverse-Tunnel zur Code-Club-VM her |

## SchoolConnect (Data-Provider, v0.3.0 Multi-Tenant als Sidecar)

SchoolConnect läuft als Compose-Service `schoolconnect`
(`apps/schoolconnect/Dockerfile` lädt das v0.3.0-Release-Binary für die
Build-Architektur — `linux/arm64` auf dem Pi, `linux/amd64` sonst — und
startet `schoolconnect serve` mit `SC_REQUIRE_TENANT=true`): nur
internes Netz (kein `ports:`, kein Tunnel/Caddy nötig, der Zugriff
läuft ausschließlich über das SmartTable-Backend via
`http://schoolconnect:8081`).

- Variablen: siehe `roles/schulapp_docker/defaults/main.yml`
  (`sc_tenant_shared_secret` für die optionale HMAC-Signatur von
  `X-SC-Tenant` — muss auf Backend- und Sidecar-Seite identisch sein,
  sonst leer — und `sc_log_level`).
- Tenant-Trennung: Das Backend setzt `X-SC-Tenant` aus der JWT-`user_id`
  (nie aus Client-Parametern); Credentials + Login-Sessions sind pro
  App-Benutzer isoliert, Logout trifft nur die eigene Session.
  Persistenz im Volume `sc_creds` (`SCHOOLCONNECT_CONFIG_DIR=/data`).
- User-Logins (Schülerportal/mebis/ByCS) laufen pro Benutzer über die App
  (Einstellungen → SchoolConnect) — die SchoolConnect-Runtime verwaltet
  ihre Sessions pro Tenant selbst, im Backend wird nichts gespeichert.
- Smoke-Test: `GET /api` des Sidecars (tenantlos) + Backend-Login müssen
  funktionieren; zwei App-User mit eigenem Schul-Login dürfen sich nicht
  vermischen.

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
