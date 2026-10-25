# TryNAS

A lightweight, self-hosted file browser and NAS dashboard built with Go, SQLite,
and Next.js. Runs as a single executable on Linux, macOS, and Windows.

[Project website](https://rajibdpi.github.io/trynas/) · [Latest release](https://github.com/rajibdpi/trynas/releases/latest)

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

## Run as a service on Linux or macOS

Install with `bash get.sh`, create `~/trynas/.env` as described in
[First start](installation.md#first-start), then run from this checkout:

```bash
bash scripts/install-service.sh
```

Run as your normal account, without sudo. The helper installs a Linux systemd
user unit or macOS launchd LaunchAgent using your actual home directory, then
prints the command to start it. Stop any existing TryNAS service or manually
running instance before starting the new one. See
[service startup](installation.md#production-and-service-startup) for commands,
login behavior, and Linux hosts without systemd.

## Deploy with Coolify Dockerfile

The root `Dockerfile` uses the checked-in `get.sh` to download a checksum-verified GitHub release and
runs a single production container. `.dockerignore` excludes local secrets,
data, and generated files from the build.

Push `Dockerfile`, `.dockerignore`, `get.sh`, and `scripts/docker-entrypoint.sh`
to the branch configured in Coolify and select **Dockerfile** as the build pack
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

In Coolify, add two persistent volume mounts to this application:

| Stable volume name | Container destination |
| --- | --- |
| `trynas-data` | `/data` |
| `trynas-storage` | `/storage` |

Keep both mounts and their source volumes across redeploys. `/data` includes
SQLite and the remote-storage encryption key.
The Dockerfile declares both directories as volumes and sets `DB_PATH` and
`STORAGE_ROOT` to use them by default. **The Dockerfile alone does not guarantee
reuse:** without explicit mounts, replacement containers can get new, empty
anonymous volumes. Use named volumes to reuse the same
storage when replacing a container, for example
`--mount source=trynas-data,target=/data --mount source=trynas-storage,target=/storage`
with `docker run`.

For Docker Compose, the included `compose.yaml` mounts stable named volumes:

```sh
export ADMIN_PASSWORD='<unique-random-password-of-8-to-72-bytes>'
docker compose up -d --build
```

For subsequent updates, run `docker compose build --no-cache` followed by
`docker compose up -d`. Keep the same Docker host and volume names. Set
`PUBLIC_URL` and `COOKIE_SECURE=true` when serving through HTTPS.
Do not use `docker compose down -v` when updating; it deletes these volumes.

If files already disappeared after a deployment, inspect the previous
container's mounts with `docker inspect <old-container> --format '{{json .Mounts}}'`.
The files may still be in its old volume. Back up and reuse that source volume
or migrate its contents while TryNAS is stopped, including the entire `/data`
directory and `/storage`. Creating new named volumes does not recover old data.
For Compose, set `TRYNAS_DATA_VOLUME` and `TRYNAS_STORAGE_VOLUME` to existing
volume names when reusing old volumes.

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

### Archive formats

The file browser can create and extract ZIP, 7z, XZ, BZIP2, GZIP, TAR, and WIM archives. ZIP works out of the box. For other formats, install 7-Zip and make `7zz` or `7z` available on the server PATH; the Docker image includes it. XZ, BZIP2, and GZIP accept one file; use TAR, ZIP, 7z, or WIM for folders and multiple files. Extraction uses a new folder and is limited to 10,000 entries and 1 GiB. Encrypted archives and links are rejected.

Archive creation has no default entry or source-size cap. Set `ARCHIVE_CREATE_MAX_ENTRIES` and `ARCHIVE_CREATE_MAX_BYTES` to positive integers to impose limits; unset them or use `0` for unlimited. File sizes exclude directories. Storage quotas still apply to the resulting archive. These settings affect creation only; extraction retains its separate limits.

## Project website (GitHub Pages)

The static landing page lives in `site/`. After a push to `main` passes all
release checks and publishes its GitHub release, the same workflow deploys the
website to **https://rajibdpi.github.io/trynas/**. Draft tag releases and manual
release runs do not deploy the website. Download buttons use GitHub's latest
stable release links, so they continue to work across version updates.

One-time repository setup: open **Settings → Pages → Build and deployment**
and select **GitHub Actions** as the source. If the `github-pages` environment
has branch restrictions, allow `main`. No custom domain or extra secrets are
required. A Pages deployment failure does not unpublish a successful release;
fix the Pages settings and re-run the failed job.

Preview locally with `python3 -m http.server 8000 --directory site`, then open
http://localhost:8000. Edit `site/index.html` for content and links and
`site/style.css` for styling. Only `site/` is uploaded to Pages; the NAS app,
configuration, and storage are not part of the website.
