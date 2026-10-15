FROM alpine:3.22 AS installer

RUN apk add --no-cache bash curl coreutils

# Use the latest published release unless a specific tag is supplied.
# Build with --no-cache to refresh the download when using latest.
ARG TRYNAS_VERSION=latest

RUN curl -fsSL \
    https://raw.githubusercontent.com/rajibdpi/trynas/main/get.sh \
    -o /tmp/get.sh \
    && bash /tmp/get.sh \
       --version "${TRYNAS_VERSION}" \
       --dir /opt/trynas

FROM alpine:3.22

RUN apk add --no-cache ca-certificates curl tzdata 7zip \
    && addgroup -g 10001 trynas \
    && adduser -D -u 10001 -G trynas trynas \
    && mkdir -p /srv/trynas/data /srv/trynas/storage /data /storage \
    && chown -R 10001:10001 /srv/trynas /data /storage

COPY --from=installer /opt/trynas/trynas /usr/local/bin/trynas
COPY --chmod=755 scripts/docker-entrypoint.sh /usr/local/bin/docker-entrypoint.sh

USER 10001:10001
WORKDIR /srv/trynas

ENV DB_PATH=/data/trynas.db \
    STORAGE_ROOT=/storage

# Mount the SAME named volumes or host directories here on every deployment.
# VOLUME alone creates anonymous volumes that replacements may not reuse.
# /data holds the database and encryption key; /storage holds uploaded files.
VOLUME ["/data", "/storage"]

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=60s --retries=3 \
    CMD curl -fsS --max-time 4 http://127.0.0.1:8080/api/health || exit 1

ENTRYPOINT ["/usr/local/bin/docker-entrypoint.sh"]
