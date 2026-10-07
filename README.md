# TryNAS V5 — Native (No Docker)

TryNAS is a lightweight self-hosted **File Browser + NAS Dashboard** using Next.js, Tailwind CSS, shadcn/ui-compatible components, Go, and SQLite.

## Run with one command

With the standalone Linux AMD64 binary built, run from the project root:

```bash
./dist/trynas-linux-amd64 -config .env
```

This starts both the web UI and API using the project's configuration, database
and storage paths. Open **http://localhost:8080** (or the port configured in
`.env`). Press **Ctrl+C** in the terminal to stop the app.

If the binary is missing, build it first with `node scripts/build.mjs`.
See [Standalone application for Windows, Linux and macOS](#standalone-application-for-windows-linux-and-macos)
for other platforms and build requirements. Stop any development backend before
starting the standalone app against the same database.

## Included features

- Responsive NAS dashboard: CPU, RAM, uptime and storage usage
- File/folder browser with list and grid views
- Multi-file upload and drag & drop
- Recursive search
- Create folder, rename, copy, move and ZIP
- Image/video/audio/PDF/text preview
- Downloads
- Favorites in SQLite
- Trash, restore and permanent delete
- Expiring public share links
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

These tools are needed only to build the standalone executable. Running an
already-built executable does not require Go, Node.js or pnpm.

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
ADDR=:8080
COOKIE_SECURE=false
```

For first testing, `STORAGE_ROOT=./storage` is safer. Do not point TryNAS at `/`.

## Data layout

```text
TryNAS/
├── backend/
├── frontend/
├── data/
│   └── trynas.db       # created at runtime
├── storage/            # default development storage
├── .env
└── dist/               # standalone executables
```

Actual NAS files are stored under `STORAGE_ROOT`. SQLite stores application metadata such as users, sessions, favorites, trash metadata, shares and audit logs.

## Security

Use a strong administrator password, expose only the intended storage directory, and do not expose the development servers directly to the public internet. Use a reverse proxy with HTTPS for remote access.

## V5 user management and file controls

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

Select files with checkboxes or **Select all**, then copy, move, favorite, ZIP,
or send them to trash. Bulk actions return individual outcomes and preserve
failed selections for retry. Each ZIP action creates one archive per selected
item. Right-click or use the three-dot button for a keyboard-accessible Radix
context/dropdown menu. Forms, confirmations and previews use Radix dialogs
styled with the app's shadcn-compatible component system. Appearance follows
the system theme initially; the sun/moon button persists your chosen theme.

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
node scripts/build.mjs
```

That creates a binary for your current OS/architecture in `dist/`. Build every
supported 64-bit target with:

```bash
node scripts/build.mjs --all
```

Or build selected targets:

```bash
node scripts/build.mjs --target windows/amd64,linux/amd64,darwin/arm64
```

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
database, stored files or credentials.

### Troubleshoot login

If login reports **Invalid username or password**, check which database the
running instance uses. A fresh database launched without administrator settings
creates username `admin` with password `admin123!`. Configure a strong password
before initializing a new installation.

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
   `node scripts/build.mjs`.
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

Rebuild with `node scripts/build.mjs` after changing the app to include those
changes in the standalone executable.

On Windows/macOS, requests must use the exact path spelling returned by the
file browser so case aliases cannot bypass folder permission rules. Windows
reserved device names, alternate data streams, and invalid filename characters
are rejected.
