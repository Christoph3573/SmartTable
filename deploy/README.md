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
| `roles/ssh_tunnel/` | Stellt den Reverse-Tunnel zur Code-Club-VM her |

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
