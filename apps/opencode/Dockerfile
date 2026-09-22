# OpenCode-Sidecar (KI-Lernchat, Agent-Runtime).
# Lädt das Release-Binary für die Build-Architektur (linux/arm64 = Pi,
# linux/amd64 = Standard-Server, jeweils .tar.gz mit Binary `opencode`)
# und startet `opencode serve --hostname 0.0.0.0 --port 8082`.
#
# Basis: debian-slim — das Standard-Release-Binary ist gegen glibc
# gelinkt (Interpreter /lib/ld-linux-*, verifiziert via `file`), auf
# Alpine (musl) startet es nicht (exit 127).
#
# Arch-Erkennung via `uname -m` (nicht via $TARGETARCH): TARGETARCH ist
# ein BuildKit-Builtin und ist beim klassischen Builder auf dem Pi leer —
# dann wurde immer das x64-Binary geladen, auf ARM also falsche Arch
# (exec format → exit 127 beim `--version`-Check). `uname -m` greift
# immer, TARGETARCH bleibt nur als Fallback.
#
# Multi-Tenancy: EIN gemeinsamer Service für alle User. Das Go-Backend ist
# die Security-Grenze — es legt pro User einen Workspace
# /workspaces/<user_id> an (Volume `workspaces`, gemountet auf /workspaces)
# und ruft die opencode-REST-API mit ?directory=/workspaces/<user_id> auf
# (directory-scoped: MCP-Registrierung + Sessions sind pro Directory
# isoliert, verifiziert gegen opencode 1.17.20). Der Browser spricht NIE
# direkt mit diesem Service (kein ports:, nur internes Compose-Netz).
#
# Modell-Credentials: OPENROUTER_API_KEY (oder ein anderer Provider-Key)
# kommt als Env aus .env/Ansible-Vault — der Container nutzt ihn global
# für alle Tenant-Sessions. OPENCODE_VERSION pinnt das Release.
FROM debian:12-slim

ARG OPENCODE_VERSION=v1.18.32
ARG TARGETARCH

RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates wget tar && \
    rm -rf /var/lib/apt/lists && \
    MACHINE="$(uname -m)" && \
    if [ "$MACHINE" = "aarch64" ] || [ "$TARGETARCH" = "arm64" ]; then OC_ARCH="arm64"; else OC_ARCH="x64"; fi && \
    echo "Downloading opencode ${OPENCODE_VERSION} for linux/${OC_ARCH} (uname -m=${MACHINE}, TARGETARCH=${TARGETARCH})" && \
    wget -O /tmp/opencode.tar.gz \
      "https://github.com/anomalyco/opencode/releases/download/${OPENCODE_VERSION}/opencode-linux-${OC_ARCH}.tar.gz" && \
    tar xzf /tmp/opencode.tar.gz -C /usr/local/bin && \
    rm /tmp/opencode.tar.gz && \
    chmod +x /usr/local/bin/opencode && \
    /usr/local/bin/opencode --version

ENV OPENCODE_PORT=8082

VOLUME /workspaces
EXPOSE 8082

# Kein API-Passwort: Der Service ist nur im Compose-Netz erreichbar, das
# Backend ist die Auth-Grenze (JWT + Ownership-Checks). Workspaces werden
# vom Backend angelegt (Volume `workspaces` ↔ /workspaces).
CMD ["sh", "-c", "mkdir -p /workspaces && exec opencode serve --hostname 0.0.0.0 --port ${OPENCODE_PORT}"]
