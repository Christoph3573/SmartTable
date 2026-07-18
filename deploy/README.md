# Deployment-Infrastruktur

Ansible-Playbooks und Rollen für den Smart-Table-Deploy auf dem Raspberry Pi.

## Inhalt

| Pfad | Zweck |
|------|--------|
| `site/` | Statische Website (Template für Teilnehmende) |
| `inventory/hosts` | Host-Gruppe `webservers` (INI wie in `.ansible`) |
| `inventory/group_vars/all.yml` | Leere Vorlage für gemeinsame Variablen |
| `inventory/host_vars/<host>/vars.yml` | Leere Vorlage für host-spezifische Variablen |
| `inventory/host_vars/<host>/vault.yml` | Optionale Secrets-Datei (ansible-vault) |
| `roles/example_role/` | Nur Strukturbeispiel, keine fertige Rolle |
| `deploy.yml` | Minimales Playbook mit TODO-Task |
| `provision.yml` | Optionales zweites Playbook-Gerüst |
| `ansible.cfg` | Standard-Inventory und Optionen |

## Schnellstart

1. `inventory/hosts`: Hosteintrag (`ansible_host`, `ansible_user`) an deinen Pi anpassen.
2. Optional `inventory/host_vars/pi-example/vars.yml` für host-spezifische Werte nutzen.
3. Auf dem Laptop: `pip install ansible`, dann im **diesem** Verzeichnis:

4. Danach erweitert ihr das Playbook/optional eine Rolle gemeinsam um echte Tasks.

Vollständige Session-Anleitung: [03-session-webserver-ansible.md](../docs/anleitungen/03-session-webserver-ansible.md)

---

*Siehe auch: [`../README.md`](../README.md) für die Übersicht über das gesamte Smart-Table-Projekt.*
