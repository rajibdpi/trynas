# Deploy TryNAS to Coolify

The root Dockerfile builds the frontend into the Go executable and runs one
container on port 8080. Node.js is only needed during the image build.

## Configure the application

1. Push these deployment files to your Git repository, then create a Coolify
   application from `rajibdpi/trynas`. For a production release, select the
   release tag or its exact commit rather than a moving development branch.
2. Select **Dockerfile** as the build pack, set the base directory to `/`, and
   the Dockerfile location to `/Dockerfile`. Set **Ports Exposes** to `8080`.
   No custom build or start command is needed.
3. Set your domain, for example `https://nas.example.com`, and point its DNS to
   the Coolify server. Coolify terminates HTTPS and forwards traffic to port 8080.
4. Add the following **runtime** environment variables. Replace both placeholders:

   ```dotenv
   ADMIN_USER=admin
   ADMIN_PASSWORD=<unique-random-password-of-8-to-72-bytes>
   CORS_ORIGIN=https://nas.example.com
   COOKIE_SECURE=true
   ADDR=0.0.0.0:8080
   DB_PATH=/data/trynas.db
   STORAGE_ROOT=/storage
   ```

   Enter the password directly in Coolify; do not commit it or make it a build
   argument. It initializes the first administrator only. Existing accounts
   retain their passwords. The frontend uses the same origin as the API, so
   `NEXT_PUBLIC_API_URL` is not needed.
5. Add two **persistent storage** volumes with destination paths `/data` and
   `/storage`. `/data` holds SQLite, the remote-storage encryption key, and
   the optional `.env` saved by server settings. `/storage` holds user files
   and trash. Back up both volumes together. Keep the same volumes on redeploy.
   The container runs as UID/GID `10001:10001`; existing volumes or host bind
   mounts must be writable by that user.
6. Optionally set Docker build argument `TRYNAS_VERSION` to your release tag
   (for example `v1.0.15`) so `/api/health` reports the release version.
   Otherwise the image reports `dev`.
7. Use one application instance and disable overlapping/rolling deployments
   for this SQLite-backed application. In Coolify versions that use a
   **Consistent Container Name** setting to prevent rolling updates, enable it.
   Expect brief downtime during redeploys. Disable preview deployments that
   would share the production volumes.
8. Deploy. The image includes a health check for `/api/health`. Confirm that
   `https://nas.example.com/api/health` returns `ok: true`, then sign in and
   verify an upload survives a redeploy.

Coolify environment variables override settings stored in `/data/.env`.
Manage the listener, storage paths, HTTPS cookies, and origin in Coolify.
If you need forwarded client IPs, set `TRUSTED_PROXIES` to the actual proxy
socket IPs only; it can be left unset for same-origin HTTPS access.
The upload setting defaults to 10240 MiB. Configure any upstream proxy's
request-size limit and timeout to accommodate your intended upload sizes.

## Local container check

```bash
docker build --build-arg TRYNAS_VERSION=v1.0.15 -t trynas:production .
# Create a private runtime env file containing ADMIN_USER and ADMIN_PASSWORD.
docker run --rm --name trynas-check --env-file /path/to/private-runtime.env \
  -e COOKIE_SECURE=false -e CORS_ORIGIN=http://localhost:8080 \
  -p 127.0.0.1:8080:8080 \
  -v trynas-check-data:/data -v trynas-check-storage:/storage \
  trynas:production
```

Open `http://localhost:8080/api/health` and `http://localhost:8080`.
For production HTTPS, keep `COOKIE_SECURE=true`.

Reference: [Coolify Dockerfile deployments](https://coolify.io/docs/applications/builds/dockerfile),
[health checks](https://coolify.io/docs/applications/configuration/health-checks),
and [rolling updates](https://coolify.io/docs/applications/deployments/rolling-updates).
