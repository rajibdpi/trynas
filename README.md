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

The root `Dockerfile` builds the frontend, embeds it in the Go server, and
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
ADMIN_PASSWORD=<unique-random-password-of-8-to-72-bytes>
ADDR=0.0.0.0:8080
COOKIE_SECURE=true
CORS_ORIGIN=https://nas.tryplusit.com
DB_PATH=/data/trynas.db
STORAGE_ROOT=/storage
```

Mount persistent storage at `/data` and `/storage` and keep both across
redeploys. `/data` includes SQLite and the remote-storage encryption key.
The container runs as UID/GID `10001:10001`; existing mounts must be writable
by that user. Use one instance and disable overlapping deployments using
consistent container naming. The image includes a health check at `/api/health`.
The initial admin password belongs
in Coolify's runtime settings, never in Git or build variables.

Optionally set build argument `TRYNAS_VERSION` to your release tag; it defaults
to `dev`. Deploy the commit containing the Dockerfile and verify
`https://nas.tryplusit.com/api/health`.

See the [Coolify Dockerfile guide](https://coolify.io/docs/applications/builds/dockerfile).

## Documentation

- [Installation and upgrades](installation.md)
- [Development commands and project structure](CONTRIBUTING.md)
