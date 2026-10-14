# TryNAS

A lightweight, self-hosted file browser and NAS dashboard built with Go, SQLite,
and Next.js. Runs as a single executable on Linux, macOS, and Windows.

## Features

- Browse, search, upload, download, preview, and organize files.
- Share files and folders with passwords, expiry times, and QR codes.
- Manage users, storage quotas, folder permissions, and remote storage providers.
- Restore deleted files from trash and keep favorites.
- Monitor CPU, memory, storage, network traffic, and activity logs.

## Install

On Linux or macOS:

```bash
curl -fsSL https://raw.githubusercontent.com/rajibdpi/trynas/main/get.sh | bash
```

For Windows, first-start configuration, and upgrades, see the
[installation guide](installation.md). Release binaries include the UI and API;
Go, Node.js, and a separate database server are not required to run them.

## Build requirements

For development or building from source: **Go 1.27.1**, **Node.js 24+**, and
**pnpm 10+**.

## Run development servers

From the project root:

```bash
cp .env.example .env
# Edit .env: set a unique ADMIN_PASSWORD before the first launch.
chmod 600 .env
go run ./scripts/dev
```

Open **http://localhost:3000**. The API runs at **http://localhost:8080**.
The command installs frontend dependencies if needed and starts both servers.
Press **Ctrl+C** to stop them; restart after Go code changes.

Set `STORAGE_ROOT` to your file directory and `DB_PATH` to your SQLite database
path in `.env`. Defaults are `./storage` and `./data/trynas.db`. Log in with
`ADMIN_USER` and `ADMIN_PASSWORD`; these initialize the first administrator.
Existing accounts keep their passwords.

## Build and run

After configuring `.env`:

```bash
go run ./scripts/build
# Linux AMD64 example; use the binary matching your platform in dist/.
./dist/trynas-linux-amd64 -config .env
```

The build creates a standalone executable for your machine. Open
**http://localhost:8080**, or the address configured by `ADDR`.
Stop development servers before using the same database.

## Install on Ubuntu with systemd

Build the Linux binary, then edit [scripts/trynas.service](scripts/trynas.service)
to match your service account, project directory, storage mounts, and binary.
The supplied unit uses `rajib`, `/home/rajib/trynas`, and Linux AMD64.

```bash
sudo install -o root -g root -m 0644 scripts/trynas.service /etc/systemd/system/trynas.service
sudo systemctl daemon-reload
sudo systemctl enable --now trynas.service
sudo systemctl status trynas.service
```

The service account needs read access to `.env` and write access to the database
and storage directories. Stop any manually running instance first. View logs
with `sudo journalctl -u trynas.service -f`.

## Deploy with Coolify Dockerfile

The root `Dockerfile` downloads a checksum-verified GitHub release and
runs a single production container. `.dockerignore` excludes local secrets,
data, and generated files from the build.

Push both files and select **Dockerfile** as the build pack in Coolify
(**General > Build pipeline > Build strategy** for an existing application).
Set base directory `/` and Dockerfile location `/Dockerfile`, then save.
Clear custom build/start commands.
Expose port `8080` and set the domain to `https://nas.tryplusit.com`.
Configure these runtime variables in Coolify:

```dotenv
ADMIN_USER=admin
PUBLIC_URL=https://nas.tryplusit.com
ADMIN_PASSWORD=<unique-random-password-of-8-to-72-bytes>
ADDR=0.0.0.0:8080
COOKIE_SECURE=true
CORS_ORIGIN=https://nas.tryplusit.com
DB_PATH=/data/trynas.db
STORAGE_ROOT=/storage
```

Mount persistent storage at `/data` and `/storage` and keep both across
redeploys. `/data` includes SQLite and the remote-storage encryption key.
The Dockerfile declares both directories as volumes and sets `DB_PATH` and
`STORAGE_ROOT` to use them by default. Use named volumes to reuse the same
storage when replacing a container, for example
`--mount source=trynas-data,target=/data --mount source=trynas-storage,target=/storage`
with `docker run`.
The container runs as UID/GID `10001:10001`; existing mounts must be writable
by that user. Use one instance and disable overlapping deployments using
consistent container naming. The image includes a health check at `/api/health`.
After the health endpoint responds successfully, Docker logs print
`TryNAS is ready. Open: https://nas.tryplusit.com` using `PUBLIC_URL`.
This setting only controls the printed address; configure the domain in Coolify
as described above. Without `PUBLIC_URL`, the printed address is `http://localhost:8080`.
The initial admin password belongs
in Coolify's runtime settings, never in Git or build variables.

The build argument `TRYNAS_VERSION` defaults to `latest`, which downloads the
latest published GitHub release. The release workflow publishes a new release
after successful builds and validation on `main`. Rebuild after publication
with Docker caching disabled to fetch the updated binary:

```sh
docker build --no-cache -t trynas:latest .
```

In Coolify, disable build caching when redeploying to refresh `latest`.
To install a specific release instead, set `TRYNAS_VERSION` to its tag:

```sh
docker build --build-arg TRYNAS_VERSION=v1.0.15 -t trynas:v1.0.15 .
```

An existing running container keeps its installed version until you rebuild
and redeploy it. Deploy the commit containing the Dockerfile and verify
`https://nas.tryplusit.com/api/health`.

See the [Coolify Dockerfile guide](https://coolify.io/docs/applications/builds/dockerfile).

## Documentation

- [Installation and upgrades](installation.md)
- [Development commands and project structure](CONTRIBUTING.md)
