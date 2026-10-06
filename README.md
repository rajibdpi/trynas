# TryNAS V5 — Native (No Docker)

TryNAS is a lightweight self-hosted **File Browser + NAS Dashboard** using Next.js, Tailwind CSS, shadcn/ui-compatible components, Go, and SQLite.

## Native package policy

This package intentionally contains **no Dockerfile and no docker-compose.yml**. Run the Go API and Next.js frontend directly on Linux/macOS/WSL. Docker can be added later as a separate deployment layer.

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

## Requirements

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
CORS_ORIGIN=http://localhost:3000
NEXT_PUBLIC_API_URL=http://localhost:8080
COOKIE_SECURE=false
```

For first testing, `STORAGE_ROOT=./storage` is safer. Do not point TryNAS at `/`.

## Run backend

From the project root:

```bash
./run-backend.sh
```

API: `http://localhost:8080`

The first startup creates the SQLite database and bootstrap administrator.
Sign in with `ADMIN_USER` and `ADMIN_PASSWORD` from `.env`. These values only
initialize the administrator when the database has no users; editing `.env`
does not change an existing account's password.

## Run frontend

Open a second terminal:

```bash
./run-frontend.sh
```

Web UI: `http://localhost:3000`

Or manually:

```bash
cd frontend
pnpm install
pnpm dev
```

## Production native deployment

Build the frontend:

```bash
cd frontend
pnpm install
pnpm build
pnpm start
```

Build the backend:

```bash
cd backend
go mod download
go build -o trynas-api .
./trynas-api
```

For production, run both processes with systemd and place Nginx/Caddy/Traefik in front with HTTPS. Set `COOKIE_SECURE=true` after HTTPS is enabled.

## Data layout

```text
TryNAS/
├── backend/
├── frontend/
├── data/
│   └── trynas.db       # created at runtime
├── storage/            # default development storage
├── .env
├── run-backend.sh
└── run-frontend.sh
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

Validation:

```bash
cd backend
go test -race ./...
cd ../frontend
pnpm exec tsc --noEmit
pnpm build
```

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
