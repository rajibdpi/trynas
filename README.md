# TryNAS V5 — Native (No Docker)

TryNAS is a lightweight self-hosted **File Browser + NAS Dashboard** using Next.js, Tailwind CSS, shadcn/ui-compatible components, Go, and SQLite.

For platform-specific setup, verified binary installation, and upgrades, see
[Installation](installation.md), including Go-free `get.sh` (Linux/macOS) and `get.ps1` (Windows) installers.

## Run development servers

Install the [build requirements](#build-requirements), then run these commands
from the project root:

```bash
cp .env.example .env
# Edit .env and set a unique ADMIN_PASSWORD before the first launch.
chmod 600 .env
go run ./scripts/dev
```

Open **http://localhost:3000** for the Next.js frontend. The Go API listens on
**http://localhost:8080** by default. The Go command loads `.env`, installs
frontend dependencies with `pnpm install --frozen-lockfile` if `node_modules`
is missing, compiles the backend, and starts both servers.

Press **Ctrl+C** to stop both servers and their child processes. If either
server exits, the other stops automatically. Stop existing TryNAS servers first.
Frontend edits use Next.js hot reload; restart the command after Go backend edits.
The command leaves `frontend/.env.local` unchanged.

For the default development ports, configure:

```dotenv
ADDR=127.0.0.1:8080
CORS_ORIGIN=http://localhost:3000
NEXT_PUBLIC_API_URL=http://localhost:8080
COOKIE_SECURE=false
```

Backend flags override environment values. For example:

```bash
go run ./scripts/dev --config .env --addr 127.0.0.1:8081
go run ./scripts/dev --help
```

When changing the backend port, also update `NEXT_PUBLIC_API_URL` to match.
The frontend uses port 3000. `--root` and `--db` override storage and database
paths. The existing `./run-dev.sh` command also forwards to this Go runner.

## Run the standalone app

With the standalone Linux AMD64 binary built, run from the project root:

```bash
./dist/trynas-linux-amd64 -config .env
```

This starts both the web UI and API using the project's configuration, database
and storage paths. Open **http://localhost:8080** (or the port configured in
`.env`). Press **Ctrl+C** in the terminal to stop the app.

If the binary is missing, build it first with `go run ./scripts/build`.
See [Standalone application for Windows, Linux and macOS](#standalone-application-for-windows-linux-and-macos)
for other platforms and build requirements. Stop any development backend before
starting the standalone app against the same database.

## Install on Ubuntu with systemd

The supplied `scripts/trynas.service` runs the standalone Linux AMD64 app as
`rajib` from `/home/rajib/trynas`, loads that directory's `.env`, restarts after
failure, and starts at boot. It serves both the UI and API; a separate frontend
service is not needed.

If your installation differs, edit `User`, `Group`, `WorkingDirectory`,
`ExecStart`, and `RequiresMountsFor` in the unit before installing it. On ARM64,
use `trynas-linux-arm64` in `ExecStart`. The service account must have access to
the binary and configuration, and write access to the database and storage
directories. For storage on a separate mount, add its absolute mount path to
`RequiresMountsFor` so systemd mounts it before starting TryNAS.

1. Open the project directory:

   ```bash
   cd /home/rajib/trynas
   ```

2. Create the configuration only if `.env` does not already exist, then edit it:

   ```bash
   if [ ! -f .env ]; then cp .env.example .env; fi
   nano .env
   chmod 600 .env
   ```

   Set `ADMIN_USER` and a strong `ADMIN_PASSWORD` before the first launch.
   Check `ADDR`, `DB_PATH`, and `STORAGE_ROOT`; relative data and storage paths
   resolve from the service's `WorkingDirectory`. See [Configure](#configure)
   for configuration examples. Existing database accounts keep their passwords.

3. Check that the standalone binary is available:

   ```bash
   test -x dist/trynas-linux-amd64
   ```

   If it is missing, install the [build requirements](#build-requirements) and
   run `go run ./scripts/build`. Use `dist/trynas-linux-arm64` on ARM64 and update
   the unit's `ExecStart` accordingly.

4. Review `scripts/trynas.service` and adjust its account and paths if needed.
   Stop any manually started TryNAS or development backend before starting the
   service against the same database.

Install the unit and enable startup at boot:

```bash
sudo install -o root -g root -m 0644 scripts/trynas.service /etc/systemd/system/trynas.service
sudo systemctl daemon-reload
sudo systemctl enable --now trynas.service
sudo systemctl status trynas.service
```

The status should show `active (running)`. If startup fails, inspect the logs:

```bash
sudo journalctl -u trynas.service -n 100 --no-pager
```

Check that the configured service account can read and execute the binary,
read and secure `.env`, and write to the configured database and storage
directories. If the port is already in use, stop the existing server or change
`ADDR` in `.env`. After correcting repeated startup failures, run:

```bash
sudo systemctl reset-failed trynas.service
sudo systemctl start trynas.service
```

Open **http://localhost:8080**, or the address configured by `ADDR` in `.env`.
Use these commands to view logs and manage the app:

```bash
sudo journalctl -u trynas.service -f
sudo systemctl restart trynas.service
sudo systemctl stop trynas.service
```

Restart after changing `.env`. Before rebuilding or replacing the executable,
stop the service, then start it again with `sudo systemctl start trynas.service`.
After editing `scripts/trynas.service`, reinstall it and reload systemd:

```bash
sudo install -o root -g root -m 0644 scripts/trynas.service /etc/systemd/system/trynas.service
sudo systemctl daemon-reload
sudo systemctl restart trynas.service
```

To prevent startup at boot and stop the app, run
`sudo systemctl disable --now trynas.service`.

## Included features

- Responsive NAS dashboard with live CPU, memory, storage and network traffic graphs
- Fixed navbar with background blur, file search and a profile/logout dropdown
- Collapsible sidebar with icon-only navigation
- File/folder browser with list and grid views
- Double-click to open and Ctrl-click to select files and folders
- Multi-file upload and drag & drop
- Recursive search
- Create folder, rename, copy, move and ZIP
- Image/video/audio/PDF/text preview
- Downloads
- Favorites in SQLite
- Trash, restore and permanent delete
- File and folder share links with optional passwords, exact expiry times and downloadable QR codes
- SQLite-backed revocable sessions
- Audit/activity log
- Filesystem path-traversal protection
- Tailwind CSS + shadcn/ui-compatible UI

## Stack

- Frontend: Next.js 16.3.8, React, TypeScript, Tailwind CSS, shadcn/ui-compatible primitives, Lucide
- Backend: Go 1.27.1 target, chi, gopsutil
- Database: SQLite in WAL mode
- Storage: native local/Linux filesystem

## Build requirements

These tools are needed for development and full release builds. Running an
already-built executable does not require Go, Node.js or pnpm. Backend-only
builds with `--skip-frontend` need Go and existing `frontend/dist` assets.

- Go 1.27.1 or compatible current Go toolchain
- Node.js 24+
- pnpm 10+
- Linux is recommended for NAS use

Ubuntu example prerequisites:

```bash
sudo apt update
sudo apt install -y build-essential sqlite3
corepack enable
```

Install Go separately from the official Go distribution if your Ubuntu repository has an older release.

## Configure

```bash
cp .env.example .env
nano .env
```

Important values:

```env
ADMIN_USER=admin
ADMIN_PASSWORD=use-a-long-random-password
DB_PATH=./data/trynas.db
STORAGE_ROOT=/mnt/storage
ADDR=127.0.0.1:8080
COOKIE_SECURE=false
```

For first testing, `STORAGE_ROOT=./storage` is safer. Do not point TryNAS at `/`.

## Data layout

```text
TryNAS/
├── main.go             # standalone entry point
├── go.mod
├── go.sum
├── cmd/                # command execution and server startup
│   ├── root.go         # Execute entry point and configuration loading
│   └── server.go       # storage initialization and HTTP server
├── scripts/
│   ├── build/          # Go release builder
│   └── dev/            # Go development server runner
├── app/                # dependency wiring and HTTP routes
├── core/               # shared state, models, and helpers
├── files/              # file operations (other features also have root folders)
├── providers/          # storage provider code
├── frontend/           # all UI source, Go static handler, and embedded assets
│   ├── modules/        # frontend feature folders
│   ├── frontend.go     # Go handler for the embedded web UI
│   └── dist/           # generated static assets for standalone builds
├── data/
│   └── trynas.db       # created at runtime
├── storage/            # default development storage
├── .env
└── dist/               # standalone executables
```

Actual NAS files are stored under `STORAGE_ROOT`. SQLite stores application metadata such as users, sessions, favorites, trash metadata, shares and audit logs.

## Module architecture

The source follows the feature-oriented folder organization used by
[File Browser](https://github.com/filebrowser/filebrowser), adapted to TryNAS's
Go backend and Next.js frontend. Backend feature packages live in project-root folders; frontend features live
under `frontend/modules/`.

| Module | Backend responsibility | Frontend responsibility |
| --- | --- | --- |
| `auth` | Login, logout, sessions, browser-origin middleware, throttling | Login panel |
| `files` | Browsing, search, downloads, previews, uploads, copy/move, ZIP | File browser, previews, folder picker, file icons |
| `users` | Accounts, user homes, quotas, profiles | Users and quotas panel |
| `permissions` | Folder grants, access checks, request guards | Folder permissions panel |
| `share` | Share links, passwords, expiry, public pages, QR codes, archives | Shared links panel |
| `providers` (frontend: `storage`) | Provider connections, encryption, transfers, attached storage | Provider settings and attached storage browser |
| `favorites` | Saved favorites | Favorites panel |
| `trash` | Trash listing, deletion, restore, purge | Trash panel |
| `settings` | Application preferences | Application settings panel |
| `metrics` | System, network, SMART monitoring | System and disk health panels, graphs |
| `activity` | Audit recording and listing | Activity panel |

`main.go` calls `cmd.Execute()`, which loads configuration and starts the
server. `cmd/server.go` initializes storage and the database, then creates the
application. `app/` creates the feature modules, wires their dependencies,
and registers HTTP routes. Startup errors return to `main.go` for logging
and a nonzero exit status.
`core/` contains shared state, models, path helpers, and security
helpers. Each feature's `module.go` declares the operations it needs through a
`Dependencies` interface, implemented by the application. Feature packages do
not import the application or each other. All modules use the same database,
storage paths, and locks so quotas and file mutations remain coordinated.
`config/` loads configuration; `database/` opens SQLite and
applies migrations; `frontend/frontend.go` serves the embedded static UI.
The `providers/` package contains storage integration code; the root `storage/`
folder remains the default NAS data directory.

Inside `frontend/`, `app/` contains the route entry points and layout,
`modules/workspace` composes the feature panels, `modules/dashboard` contains
the overview, and `modules/shared` contains the API client, types, helpers,
notifications, and shared dialogs. Reusable UI primitives remain in
`components/ui/`.

Use `go run ./scripts/dev` for development. The release builder
generates the Next.js static export in `frontend/dist/` and embeds
that folder directly in the standalone executable. Rebuild after source changes:

```bash
go run ./scripts/build
```

Run backend regression tests and frontend type checks from the project root:

```bash
go test ./...
(cd frontend && pnpm exec tsc --noEmit)
```

## Security

The backend defaults to `127.0.0.1:8080` for local access. For LAN or remote
access, configure the listener explicitly and use HTTPS through a reverse proxy
with `COOKIE_SECURE=true`. The application does not trust forwarded client-IP
headers; authentication throttling uses the socket peer address. Behind a proxy,
that limit is shared by clients using the same proxy connection address.

Configuration and database files are restricted to the service account. New
sessions store UTC timestamps, and legacy session timestamps are normalized on
startup. Password attempts are limited to 10 per account/share and 30 per socket
IP in five minutes. Browser mutations reject untrusted origins, and JSON API
posts require `application/json`.

Public ZIP downloads allow two concurrent requests, up to 1 GiB of source data
and ZIP output, and 10,000 archive entries. Requests have a five-minute deadline;
HTTP transfers have a 15-minute timeout. ZIP delivery releases the mutation lock
before sending the archive. Larger selections must be split into smaller shares.

Use a strong administrator password, expose only the intended storage directory, and do not expose the development servers directly to the public internet. Use a reverse proxy with HTTPS for remote access.

## V5 user management and file controls

**Dashboard**, **System & network**, **Activity**, **Users & quotas**,
**Folder permissions**, **Settings** and **Disk health** are available only to
administrators. Normal users land on **Files** and see Files, Favorites,
Shared links and Trash. Restricted pages redirect them to Files, and the
corresponding backend APIs reject non-admin access.

Administrators have **Users & quotas**, **Folder permissions**, and **Disk health**
sections. Creating an account with an 8–72 byte password also creates a home folder named
after its username and grants that user write access. Usernames must be single
folder names, and existing storage items cannot be adopted as new home folders.
Other folders require explicit grants. Editing an existing username does not
rename its home folder. A blank permission
path means storage root. Read grants allow browsing, preview, download, copying
and sharing; write grants also allow uploads and changes. Copying and creating
ZIPs require write access at the destination. The nearest explicit rule applies
to descendants; `none` denies access and removing a rule restores inheritance.
Parents of granted folders can be browsed to reach them, but unrelated files
are hidden. Downloads, search, recursive operations, and public links enforce
permissions. Revoking a share owner's access also disables their public links.
Administrators always have full access. Remove explicit rules inside a folder
before moving or deleting that folder, then reapply them at its new location.
Password resets revoke the user's sessions. The last administrator is protected.

Quotas are per user: `0` means unlimited. The UI accepts GiB and the API stores
bytes. Files uploaded, copied, or generated as ZIPs through TryNAS are owned by
the acting user and charged to their quota. Renames and moves preserve ownership.
Trash remains charged until permanent deletion. Administrators can manage all
trash entries. Existing storage files remain unassigned and are not retroactively
charged. Direct filesystem changes are outside this accounting; use TryNAS for
managed files. Account deletion requires its owned files to be removed first.
Quota checks and mutations are serialized within the API process; run one API
instance against each database/storage root.

Uploads use measured transfer progress, a processing phase, and cancellation
while transferring. Each request is limited to 10 GiB. A failed permission,
quota, duplicate-name, or destination-conflict check installs none of that
request's files. Existing files are never overwritten. Symbolic links and
reserved `.trynas-*` paths are blocked. Temporary upload data is cleaned up.

## Browse and select files

Choose **Files** in the sidebar or open **/files** to access the All files view
directly. The dashboard remains at **/**.

In list and grid views, use these controls:

| Action | Behavior |
| --- | --- |
| Click an item | Leave selection unchanged |
| Ctrl-click | Add or remove an individual file or folder from the selection |
| Shift-click | Select a range from the last clicked item |
| Ctrl + Shift-click | Add a range to the existing selection |
| Double-click | Open a folder or preview a file |

Use **Ctrl-click** to select individual items. Once an item is selected,
**Select all** appears to select all visible items. **Shift-click** selects a
range in the displayed order; **Ctrl + Shift-click** adds that range to the
selection. Plain click sets the range anchor without changing selection.
Selection clears when changing folders or search terms.
Focus an item's name and press **Enter** or **Space**
to open it with the keyboard. Favorites also open on double-click.

Use the search field beside **Upload** in the file browser header, or enter at
least two characters in the navbar search and press **Enter** to open results.
Search includes descendants of the current folder. List/grid toggles, Folder
and Refresh controls are also in the file browser header.

Select items, then copy, move, favorite, ZIP, or send them to trash.
Bulk actions return individual outcomes and preserve
failed selections for retry. Each ZIP action creates one archive per selected
item. Right-click or use the three-dot button for a keyboard-accessible Radix
context/dropdown menu. Forms, confirmations and previews use Radix dialogs
styled with the app's shadcn-compatible component system. Appearance follows
the system theme initially; the navbar's sun/moon button persists your chosen theme.

The navbar stays fixed with a blurred background. Its panel toggle collapses
the desktop sidebar to icons or expands it; on mobile it opens or closes
navigation. The account dropdown offers **Profile** and **Logout**.

Action notifications appear as shared toasts at the bottom right, including
copied links, uploads, moves and deletions. Toasts dismiss automatically or with
the close button; hovering or focusing a toast pauses dismissal.

Dashboard cards show rolling 60-second graphs. CPU, memory and storage update
every five seconds; network traffic updates every three seconds, with separate
download and upload lines. Hover over graph points for measured values. Storage
shows your quota usage, with an automatic bytes-used scale for unlimited quotas.

## Settings

Administrators can open **Settings** from the sidebar or visit **/settings**.
Save the workspace app name, default list/grid file view and default share expiry
(Never, 1, 7, 15 or 30 days). Settings persist in SQLite and supply defaults for
all users; existing share links retain their own expiry. Other users pick up
changed defaults when they reload the app.

The page also displays the active storage root, listen address, frontend origin,
secure-cookie mode and SMART sudo setting. Change those values in `.env` and
restart the backend. Passwords and other secrets are not displayed on this page.

## Share files and folders

Choose **Share link** from a file or folder's menu. Under **Expires at**, choose
**Never**, **1 day**, **7 days**, **15 days**, **30 days**, or **Custom**. Custom
shows a date/time picker in your local timezone, up to 365 days ahead. Optionally
set an 8–72 byte password; leave it blank for access through the link alone.

Recipients open the shared page and enter the password when required. The page
lists shared files and folders, including readable folder contents, with names,
types and individual download buttons. Files
download directly; folders download as ZIP archives containing only items the
share owner can read. Reserved paths and symbolic links are excluded. The page
also offers a downloadable QR code for the link. Protected shares remain
password-protected when opened through their QR code.

To share a selection with one link, **Ctrl-click** the files and folders, then
choose **Share** in the selection toolbar. **Select all** can include every
visible item. Set one password and expiry for the entire link. Recipients use
**Download all (ZIP)** for multiple items. Selected folders include their
readable contents; overlapping selections are included once. A link supports
up to 500 selected items. **Shared links** lists the items included in each link.

**Shared links** shows each link's expiry in your local time and whether it is
password protected. Revoking a link, reaching its expiry or removing the owner's
access blocks further downloads. Existing links retain their original settings.

## Hardware monitoring

**System & network** displays actual interface traffic in bytes per second,
sampled every second and refreshed in the UI every three seconds. This measures
traffic, not theoretical link capacity; loopback interfaces are excluded.

**Disk health** uses smartmontools JSON output, with a one-minute cache, bounded
command timeouts, and up to sixteen scanned devices. Install it on the NAS host:

```bash
sudo apt install smartmontools
smartctl --scan-open -j
smartctl -a -j /dev/sda
```

Use your actual device from the scan. Configure the API service account's device
access according to your operating system; TryNAS runs SMART commands directly by default. Optional scoped sudo access
is described below. SMART health, temperature, power-on hours and NVMe wear are
shown when available. Missing smartctl, unsupported controllers, virtual disks,
or insufficient device access show unavailable data rather than a healthy
reading. SMART indicates drive-reported condition and does not replace backups.

### SMART permission denied

If the disk cards report `Permission denied`, the API's Linux account cannot
open the devices. The supplied installer grants only specific read-only SMART
commands for `/dev/sda`, `/dev/sdb`, `/dev/sdc` and `/dev/nvme0`, plus disk scanning.
It does not grant a root shell, arbitrary smartctl flags, or disk write access.
From the project root, install the policy with your API's Linux username:

```bash
sudo ./scripts/install-smart-access.sh rajib
```

The script validates the sudoers policy with `visudo` before installing it at
`/etc/sudoers.d/trynas-smartctl`. Set `SMART_USE_SUDO=true` in `.env` and restart
the backend. It then invokes `sudo -n` (never prompts for or stores a password).
Use Disk health's refresh button to bypass the one-minute cache. Verify access:

```bash
sudo -n /usr/sbin/smartctl -a -j /dev/nvme0 -d nvme
```

Adapt the installer allowlist if your devices have different names or controller
types. To undo, remove `/etc/sudoers.d/trynas-smartctl` with administrator access,
set `SMART_USE_SUDO=false`, and restart the backend.

## Standalone application for Windows, Linux and macOS

Build once to embed the complete frontend in the Go executable. On the machine
that builds releases, install Go, Node.js and pnpm, then run from the project root:

```bash
go run ./scripts/build
```

Release binaries contain no default administrator password. Set `ADMIN_PASSWORD`
(8–72 bytes) at runtime before initializing a fresh database. `ADMIN_USER` defaults
to `admin`. Building does not print or write credentials, and rebuilding does not
reset existing accounts.

The Go release command rebuilds the frontend and backend. It clears `.next`
and `frontend/dist`, generates a fresh static export, and embeds that export
in each Go executable. Development output stays in `frontend/.next-dev`.
The default target is the current operating system and architecture.

| Command | Result |
| --- | --- |
| `go run ./scripts/build` | Full release build for the current machine |
| `go run ./scripts/build --all` | Full builds for all six supported targets |
| `go run ./scripts/build --target linux/arm64,windows/amd64` | Full builds for selected targets |
| `go run ./scripts/build --skip-frontend` | Rebuild Go using existing exported UI assets |
| `go run ./scripts/build --help` | Show build options |

`--skip-frontend` can also be combined with `--target` or `--all`. It does not
include frontend source edits; use a full build when the UI changes. `--all`
and `--target` cannot be used together. Supported targets are Linux, Windows,
and macOS (`darwin`), each on `amd64` and `arm64`.

Binaries are written to the root `dist/` directory, alongside `SHA256SUMS`.
A subset build preserves other target binaries and includes their existing
checksums. Rebuild all targets before distributing a complete updated release.
Each existing binary is replaced only after its new build succeeds.

Building does not update an already running app. Stop the development backend
or old release, then run the newly built executable **from the project root**
to use the same `.env`, database, and storage as dev mode. For Linux amd64:

```bash
./dist/trynas-linux-amd64
```

Launching from inside `dist/` uses that directory for relative configuration and
data paths, which can show a different set of users, settings, and files. Other
platform binaries are refreshed only when included in the build targets; use
`--all` to update all six release executables.

Release filenames:

| Platform | Intel/AMD 64-bit | ARM64 |
| --- | --- | --- |
| Windows | `trynas-windows-amd64.exe` | `trynas-windows-arm64.exe` |
| Linux | `trynas-linux-amd64` | `trynas-linux-arm64` |
| macOS | `trynas-darwin-amd64` | `trynas-darwin-arm64` |

`dist/SHA256SUMS` contains hashes for the built binaries. Each executable embeds
the HTML, JavaScript, CSS and icons, and uses a pure-Go SQLite driver. The target
machine does not need Node.js, pnpm, Go, a separate frontend server, or an external
SQLite installation. Existing SQLite databases remain usable; stop the old API
before opening the same database with the new executable.

Run the matching binary from the project root using the commands below.
Relative `DB_PATH` and `STORAGE_ROOT` values resolve from the terminal's current
working directory. Always launch from the same directory to keep your existing
database, storage and accounts.

Existing process environment variables override `.env`; CLI flags override both.
Use `-help` to view supported flags. Set `ADMIN_USER` and `ADMIN_PASSWORD` in
`.env` before the first launch. Building does **not** bundle your `.env`,
database, stored files, or administrator credentials.

### Troubleshoot login

If login reports **Invalid username or password**, check which database the
running instance uses. A fresh database uses runtime `ADMIN_PASSWORD` (8–72 bytes) when supplied,
and rejects missing passwords or the example placeholder. Set administrator
credentials before initializing a new installation.

`ADMIN_USER` and `ADMIN_PASSWORD` only initialize an empty database. Adding or
editing `.env` after the first launch does not reset an existing account. Use
the original configuration and database to keep your existing login, or sign in
with another administrator and update the user's password in user management.
Do not delete the database to fix login: it also contains users, permissions,
shares, and other application records.

### Start and restart the standalone app

Run these commands from the project root to use its `.env`, database and storage.
Choose the binary matching your operating system and CPU:

| Platform | Start command (AMD64) |
| --- | --- |
| Linux | `./dist/trynas-linux-amd64 -config .env` |
| macOS | `./dist/trynas-darwin-amd64 -config .env` |
| Windows PowerShell | `.\dist\trynas-windows-amd64.exe -config .env` |

For ARM64, substitute `arm64` for `amd64` (including Apple Silicon Macs).
Open **http://localhost:8080**, or the port configured by `ADDR` or `-addr`.

To restart:

1. Press **Ctrl+C** in the terminal running the app and wait for it to exit.
2. If updating the app, replace the executable or rebuild it with
   `go run ./scripts/build`.
3. Run the same start command from the same working directory, with the same
   configuration and data paths.
4. Refresh the browser.

Restart after changing runtime configuration in `.env`. Existing account
passwords remain stored in the database. Keep the server in its terminal so you
can stop it with **Ctrl+C**; closing the browser does not stop the server.

### Stop or change the server port

Press **Ctrl+C** in the terminal running TryNAS. Start only one server against
a given database.

If startup reports `bind: address already in use`, stop the existing server or
choose another port:

```bash
./dist/trynas-linux-amd64 -config .env -addr :8090
```

Then open **http://localhost:8090**. Rebuilding an executable does not stop or
restart an already running server; stop it and launch the new binary.

SMART monitoring is optional and still needs smartmontools plus disk/controller
access on the target host. `scripts/install-smart-access.sh` and
`SMART_USE_SUDO=true` are Linux setup options; Windows and macOS require their own
appropriate device access. Unsupported SMART fields remain unavailable.
Cross-built Windows/macOS binaries are unsigned; signing/notarization for public
distribution is a separate release step. macOS Finder launches and Windows GUI
launches are not managed installers; the app runs as a terminal/server process.

Rebuild with `go run ./scripts/build` after changing the app to include those
changes in the standalone executable.

On Windows/macOS, requests must use the exact path spelling returned by the
file browser so case aliases cannot bypass folder permission rules. Windows
reserved device names, alternate data streams, and invalid filename characters
are rejected.

## Storage providers and sync

Administrators can open **Settings → Storage providers** to add Google Drive,
SMB/CIFS, SFTP, and Amazon S3 or S3-compatible connections. The app embeds
[rclone Go backends](https://pkg.go.dev/github.com/rclone/rclone/backend); no
rclone executable is needed on the TryNAS server.

1. Choose a provider, name the connection, and enter the remote directory and credentials.
2. Click **Test** to verify access to that directory.
3. Choose the local folder relative to the TryNAS storage root (empty means the root).
4. Choose **Local → Remote** or **Remote → Local**, then run a preview before transferring.

**Copy** updates files while keeping extra destination files. **Mirror** makes
the destination match the source and can delete destination files; running it
requires the deletion confirmation checkbox. These are one-way operations in
either direction, not automatic two-way conflict resolution. Files and subfolders
are transferred recursively, including empty directories where supported.

For Google Drive, run `rclone authorize drive` on your computer and paste the
returned OAuth token JSON. If you use your own OAuth client, supply the same
client ID and secret in TryNAS. See [Google Drive setup](https://rclone.org/drive/).
For SMB, use `share/path` as the remote directory. For SFTP, supply a password or
private key and a trusted `known_hosts` file path on the TryNAS server; host-key
verification is required. See [SFTP setup](https://rclone.org/sftp/).
For S3, use `bucket/path`, provide access keys, and set the region and endpoint as
needed by your provider.

Connections are encrypted with AES-GCM in the database. The generated
`.trynas-storage.key` beside the database has mode `0600`; back up this key
securely along with the database, because losing it makes saved connections
unreadable. Credentials are never returned by the connections API.

One transfer runs at a time, with a 24-hour deadline. Status, transferred bytes,
file counts, and cancellation are available in Settings. The most recent job is
saved; jobs interrupted by a server restart must be started again. Transfers hold
the file-mutation lock to prevent overlapping app writes, and exclude internal
`.trynas-*` folders. Local symlinks are rejected. Copying files can overwrite
existing destination content; cancellation does not roll back completed changes.

### Attach providers as storage

In **Settings → Storage providers**, click **Attach as storage** on a saved
connection. Administrators can then open **All files** and use the **Storage**
selector to switch between local storage and attached providers. Attachments
persist across restarts. **Detach storage** removes the provider from the file
browser without deleting its files or its saved connection.

Attached storage supports direct folder browsing, downloads, uploads, new
folders, and permanent file/folder deletion. Double-click opens a folder or
downloads a file; Ctrl/Cmd-click selects, and Shift-click selects a range.
Search filters the current remote folder. Browser uploads refuse to overwrite
an existing file. Remote deletion requires confirmation and does not use the
local trash. Local permissions, quotas, favorites, and public share links apply
to local storage; attached providers currently remain administrator-only.

Operations use the provider's configured remote directory as their root. One
direct remote operation runs at a time, and sync jobs cannot overlap direct
remote operations. Provider operations have a 15-minute request deadline;
failures may leave completed uploads or deletions in place.

### Backend production behavior

SQLite uses WAL with at most eight reusable connections. Covering indexes speed
up per-user quota totals, and quota reads use one database query. System readings
are cached for one second; app settings are cached for 30 seconds and updated
immediately after saves. API and share responses use `Cache-Control: no-store`;
fingerprinted frontend assets retain their long-lived browser cache.

Use HTTPS for production access. Session cookies are automatically secure with
direct TLS. When terminating TLS at a reverse proxy, set `COOKIE_SECURE=true`
and the exact frontend origin in `CORS_ORIGIN`. The app does not trust arbitrary
forwarded headers to decide cookie security.
