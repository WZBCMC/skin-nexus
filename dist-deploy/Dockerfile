# SkinNexus runtime-only image (deploy variant): artifacts are cross-compiled
# on the dev machine and copied in — no compilation on the 2C/3.5G box.
# The repo's top-level Dockerfile is the multi-stage source build; this one
# exercises the same runtime contract (binary + web/dist + migrations + /data
# volume + /healthz + auto-migrate).
FROM alpine:3.20
# CN network: use aliyun mirror (dl-cdn index fetch is unreliable from China);
# wget comes from busybox in the base image (healthcheck).
RUN sed -i 's|dl-cdn.alpinelinux.org|mirrors.aliyun.com|g' /etc/apk/repositories \
 && apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY skin-nexus /app/skin-nexus
COPY web-dist /app/web/dist
COPY migrations /app/migrations
RUN chmod +x /app/skin-nexus

ENV SKIN_LISTEN=:8080 \
    SKIN_DATA_DIR=/data \
    SKIN_MIGRATIONS_DIR=/app/migrations \
    SKIN_AUTO_MIGRATE=true

VOLUME /data
HEALTHCHECK --interval=30s --timeout=3s --retries=3 \
  CMD wget -q -O /dev/null http://127.0.0.1:8080/healthz || exit 1

ENTRYPOINT ["/app/skin-nexus"]
