# SchoolConnect-Sidecar (v0.3.0 Multi-Tenant).
# Lädt das Release-Binary für die Build-Architektur (linux/arm64 = Pi,
# linux/amd64 = Standard-Server, Assets heißen schoolconnect-linux-amd64
# bzw. schoolconnect-linux-arm64 — KEIN separates x64-Asset wie bei
# opencode) und startet `schoolconnect serve`.
# Mandanten-Trennung: SC_REQUIRE_TENANT=true verlangt X-SC-Tenant für
# Login-Plugins (/healthz, /api-Index und öffentliche Plugins bleiben frei).
#
# Arch-Erkennung via `uname -m` (nicht via $TARGETARCH): TARGETARCH ist
# ein BuildKit-Builtin und ist beim klassischen Builder auf dem Pi leer —
# dann wurde die URL .../schoolconnect-linux- (ohne Suffix) gezogen und
# der Build brach ab. `uname -m` greift immer, TARGETARCH nur als Fallback.
FROM alpine:3.21

ARG SC_VERSION=v0.3.0
ARG TARGETARCH

RUN apk add --no-cache ca-certificates wget && \
    MACHINE="$(uname -m)" && \
    if [ "$MACHINE" = "aarch64" ] || [ "$MACHINE" = "arm64" ] || [ "$TARGETARCH" = "arm64" ]; then SC_ARCH="arm64"; else SC_ARCH="amd64"; fi && \
    echo "Downloading schoolconnect ${SC_VERSION} for linux/${SC_ARCH} (uname -m=${MACHINE}, TARGETARCH=${TARGETARCH})" && \
    wget -O /usr/local/bin/schoolconnect \
      "https://github.com/Christoph3573/SchoolConnect/releases/download/${SC_VERSION}/schoolconnect-linux-${SC_ARCH}" && \
    chmod +x /usr/local/bin/schoolconnect && \
    /usr/local/bin/schoolconnect list

ENV REST_ADDR=:8081 \
    SCHOOLCONNECT_CONFIG_DIR=/data \
    SC_REQUIRE_TENANT=true \
    LOG_LEVEL=info

VOLUME /data
EXPOSE 8081

HEALTHCHECK --interval=15s --timeout=5s --retries=3 \
  CMD wget -qO- http://127.0.0.1:8081/healthz > /dev/null || exit 1

CMD ["schoolconnect", "serve"]
