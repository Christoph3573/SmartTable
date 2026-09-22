# SchoolConnect-Sidecar (v0.3.0 Multi-Tenant).
# Lädt das Release-Binary für die Build-Architektur (linux/arm64 = Pi,
# linux/amd64 = Standard-Server) und startet `schoolconnect serve`.
# Mandanten-Trennung: SC_REQUIRE_TENANT=true verlangt X-SC-Tenant für
# Login-Plugins (/healthz, /api-Index und öffentliche Plugins bleiben frei).
FROM alpine:3.21

ARG SC_VERSION=v0.3.0
ARG TARGETARCH

RUN apk add --no-cache ca-certificates wget && \
    wget -O /usr/local/bin/schoolconnect \
      "https://github.com/Christoph3573/SchoolConnect/releases/download/${SC_VERSION}/schoolconnect-linux-${TARGETARCH}" && \
    chmod +x /usr/local/bin/schoolconnect && \
    /usr/local/bin/schoolconnect list

ENV REST_ADDR=:8081 \
    SCHOOLCONNECT_CONFIG_DIR=/data \
    SC_REQUIRE_TENANT=true \
    LOG_LEVEL=info

VOLUME /data
EXPOSE 8081

CMD ["schoolconnect", "serve"]
