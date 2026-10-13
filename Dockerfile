# syntax=docker/dockerfile:1
FROM node:24-bookworm-slim AS frontend
WORKDIR /src/frontend
RUN npm install --global pnpm@12.5.1
COPY frontend/package.json frontend/pnpm-lock.yaml frontend/pnpm-workspace.yaml ./
COPY frontend/patches ./patches
RUN pnpm install --frozen-lockfile
COPY frontend/ ./
ENV TRYNAS_EMBEDDED_BUILD=true NEXT_PUBLIC_EMBEDDED=true NEXT_PUBLIC_API_URL="" NEXT_TELEMETRY_DISABLED=1
RUN pnpm build

FROM golang:1.27.2-bookworm AS backend
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend /src/frontend/dist ./frontend/dist
ARG TRYNAS_VERSION=dev
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X github.com/rajibdpi/trynas/core.Version=${TRYNAS_VERSION}" -o /out/trynas .

FROM debian:bookworm-slim AS runtime
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates curl \
    && rm -rf /var/lib/apt/lists/* \
    && groupadd --gid 10001 trynas \
    && useradd --uid 10001 --gid trynas --no-create-home trynas \
    && mkdir /data /storage && chown trynas:trynas /data /storage
COPY --from=backend /out/trynas /usr/local/bin/trynas
WORKDIR /data
ENV ADDR=0.0.0.0:8080 DB_PATH=/data/trynas.db STORAGE_ROOT=/storage COOKIE_SECURE=true
USER 10001:10001
EXPOSE 8080
VOLUME ["/data", "/storage"]
HEALTHCHECK --interval=30s --timeout=5s --start-period=30s --retries=3 \
    CMD curl --fail --silent http://127.0.0.1:8080/api/health > /dev/null || exit 1
ENTRYPOINT ["/usr/local/bin/trynas"]
