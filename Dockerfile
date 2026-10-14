FROM alpine:3.22 AS installer

RUN apk add --no-cache bash curl coreutils

ARG TRYNAS_VERSION=v1.0.15

RUN curl -fsSL \
    https://raw.githubusercontent.com/rajibdpi/trynas/main/get.sh \
    -o /tmp/get.sh \
    && bash /tmp/get.sh \
       --version "${TRYNAS_VERSION}" \
       --dir /opt/trynas

FROM alpine:3.22

RUN apk add --no-cache ca-certificates curl tzdata \
    && addgroup -g 10001 trynas \
    && adduser -D -u 10001 -G trynas trynas \
    && mkdir -p /srv/trynas/data /srv/trynas/storage \
    && chown -R 10001:10001 /srv/trynas

COPY --from=installer /opt/trynas/trynas /usr/local/bin/trynas

USER 10001:10001
WORKDIR /srv/trynas

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=60s --retries=3 \
    CMD curl -fsS --max-time 4 http://127.0.0.1:8080/api/health || exit 1

ENTRYPOINT ["/bin/sh", "-c", "umask 077; exec /usr/local/bin/trynas"]