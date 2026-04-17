# CHECK24 Code Club

Begleitmaterial und Ansible-Setups für den internen Code Club. Jede Session bekommt ein eigenes Verzeichnis mit Gerüst, Rollen und Anleitung.

## Sessions

| Session | Thema | Verzeichnis |
|---------|-------|-------------|
| 03 | Webserver + Ansible | [`session-03-webserver-ansible/`](session-03-webserver-ansible/) |

## Session 03 — Webserver + Ansible

Ziel: Eine statische Website per Ansible auf einem Raspberry Pi deployen und über einen SSH-Reverse-Tunnel ins Internet bringen.

### Architektur

```
Laptop  ──ansible──►  Raspberry Pi
                           │
                    [Caddy :80]
                    [autossh] ──tunnel──►  codeclub.check24.fun
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
cd session-03-webserver-ansible/

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
session-03-webserver-ansible/
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

### Secrets mit ansible-vault

```bash
# Vault-Datei verschlüsseln
ansible-vault encrypt inventory/host_vars/pi-example/vault.yml

# Playbook mit Vault ausführen
ansible-playbook deploy.yml --ask-vault-pass
```
