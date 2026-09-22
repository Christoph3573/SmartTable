# OpenCode-Sidecar (KI-Lernchat, Agent-Runtime).
# Lädt das Release-Binary für die Build-Architektur (linux/arm64 = Pi,
# linux/amd64 = Standard-Server, jeweils .tar.gz mit Binary `opencode`)
# und startet `opencode serve --hostname 0.0.0.0 --port 8082`.
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
FROM alpine:3.21

ARG OPENCODE_VERSION=v1.18.32
ARG TARGETARCH

RUN apk add --no-cache ca-certificates wget tar && \
    if [ "$TARGETARCH" = "arm64" ]; then OC_ARCH="arm64"; else OC_ARCH="x64"; fi && \
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
