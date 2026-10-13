# Deploy TryNAS to Coolify with Docker Compose

`docker-compose.yml` builds the root Dockerfile, which generates the frontend
before embedding it in the Go executable. It runs one `trynas` service on
internal port 8080 with persistent database and file volumes.

## Configure the application

1. Push `docker-compose.yml`, `Dockerfile`, and `.dockerignore` to your Git
   repository. Select a branch or commit containing all three files; older
   release tags may not contain them.
2. Create a Git-backed Coolify application and select **Docker Compose** as
   the build pack. For an existing application, open **General > Build pipeline**
   and change the build strategy to **Compose**.
3. Set **Base Directory** to `/` and **Docker Compose Location** to
   `/docker-compose.yml`. Clear custom build/start commands, save, and
   load/reload the Compose configuration.
4. Set the following variables in Coolify:

   ```dotenv
   ADMIN_USER=admin
   ADMIN_PASSWORD=<unique-random-password-of-8-to-72-bytes>
   CORS_ORIGIN=https://nas.tryplusit.com
   TRYNAS_VERSION=v1.0.15
   ```

   Replace the password placeholder and use the version matching your source.
   The password is required at runtime and initializes the first administrator
   only. Existing accounts retain their passwords. Enter it in Coolify;
   do not commit it or pass it as a build argument. `TRYNAS_VERSION` is an
   optional build argument and defaults to `dev`.
5. Under **Domains**, choose service **trynas** and enter
   `https://nas.tryplusit.com:8080`. The suffix selects the internal container
   port; your public URL remains **https://nas.tryplusit.com**. If your Coolify
   version has a separate internal-port field, set it to `8080` and enter
   `https://nas.tryplusit.com` as the domain.
6. Add a DNS A record for `nas.tryplusit.com` pointing to the Coolify server's
   public IPv4 address. Coolify handles HTTPS through its proxy.
7. Review the named volumes already defined in Compose:
   - `trynas-data` mounted at `/data`: SQLite, the remote-storage encryption
     key, and optional `.env` saved by server settings.
   - `trynas-storage` mounted at `/storage`: user files and trash.

   Keep both volumes on redeploy and back them up together. For an existing
   installation, restore its data into these volumes before starting.
   The container runs as UID/GID `10001:10001`; restored files and existing
   bind mounts must be writable by that user. Do not delete volumes on redeploy.
8. Deploy and verify `https://nas.tryplusit.com/api/health` returns `ok: true`.
   Sign in and confirm an upload survives a redeploy.

Compose sets `COOKIE_SECURE=true`, `ADDR=0.0.0.0:8080`,
`DB_PATH=/data/trynas.db`, and `STORAGE_ROOT=/storage`. The proxy reaches the
service through the container network; no public host port is needed.
The frontend uses the same origin as the API.

Use one instance. Compose replaces the service during updates, so expect brief
downtime. Keep preview deployments isolated from production volumes.
Environment variables override settings saved in `/data/.env`.
`TRUSTED_PROXIES` is optional; set only actual proxy socket IPs when forwarded
client IPs are needed. `PROXY_UPLOAD_LIMIT_MB` defaults to 10240; configure
upstream proxy size limits and timeouts to support your intended uploads.

## Troubleshoot the missing frontend error

`pattern all:dist: no matching files found` alongside `railpack-plan.json`
means the deployment still uses Railpack. Save **Docker Compose** as the build
strategy, load `/docker-compose.yml`, and start a new deployment. The new log
should build the Dockerfile frontend stage with `pnpm build` before Go.

## Validate locally

Create a private environment file containing `ADMIN_PASSWORD`, then run:

```bash
docker compose --env-file /path/to/private-runtime.env config --quiet
docker compose --env-file /path/to/private-runtime.env build
```

This service is configured for the Coolify HTTPS proxy. Port 8080 is exposed
internally and is not published for local HTTP access.

Reference: [Coolify Docker Compose applications](https://coolify.io/docs/applications/builds/docker-compose).
