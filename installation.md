# Installation

TryNAS runs as one executable containing its web UI, API, and SQLite driver.
Linux, macOS, and Windows are supported on AMD64 and ARM64. Running an installed
binary requires no Go, Node.js, pnpm, or separate database server.

This guide uses the standalone-binary approach illustrated by
[File Browser's installation guide](https://github.com/filebrowser/filebrowser/blob/master/docs/installation.md),
with TryNAS's own release assets and first-start configuration.

## Choose an installation method

| Method | Requirements |
| --- | --- |
| Native Windows installer | Windows PowerShell 5.1+ or PowerShell 7+ |
| Bash download installer | Bash, curl or wget, and sha256sum or shasum |
| Manual binary installation | A matching release binary and `SHA256SUMS` |
| Go installer from a published release | Go 1.27.1 or compatible toolchain and a source checkout |
| Go installer from a local build | Go and an existing release directory |
| Build from source | Go, Node.js 24+, and pnpm 10+ |

All installers detect your platform, verifies SHA-256, and installs only the
executable. It does not edit configuration, create accounts, or start a service.
Stop the running app before upgrading. Installation does not reset existing data.

## Download script for Linux and macOS

No Go installation or source checkout is required. Install the latest release with
one command:

```bash
curl -fsSL https://raw.githubusercontent.com/rajibdpi/trynas/main/get.sh | bash
```

To select a release tag or installation directory, pass arguments to Bash:

```bash
curl -fsSL https://raw.githubusercontent.com/rajibdpi/trynas/main/get.sh | bash -s -- --version v1.0.0 --dir ./bin
```

Alternatively, download the installer, then run it:

```bash
curl -fsSL https://raw.githubusercontent.com/rajibdpi/trynas/main/get.sh -o get.sh
bash get.sh
```

Use the branch containing `get.sh` if your checkout is published on a different
branch. These remote commands work after the script is pushed and release assets
are published. From the current checkout you can already install a local build:

```bash
bash get.sh --source ./dist
```

The default destination is `~/.local/bin`. Set a custom directory or release tag:

```bash
bash get.sh --dir ./bin
bash get.sh --version v1.0.0
bash get.sh --help
```

Use an actual published tag in place of `v1.0.0`. The script also supports Windows
with Git Bash/MSYS/Cygwin on AMD64 or ARM64; native PowerShell users can use `get.ps1` below. `--source` and `--version` cannot be
combined. No automatic sudo, service registration, or file capabilities are used.

The script detects the OS and CPU, uses GitHub's latest-release download redirect
without an API request, verifies the binary against `SHA256SUMS`, and cleans up
temporary files. It requires curl or wget only for remote downloads. Add the
installation directory to PATH and follow [First start](#first-start).

## Native Windows installer

Use Windows PowerShell 5.1 or PowerShell 7+; Go and Node.js are not required.
Install the latest release and add it to your user PATH with one command:

```powershell
& ([scriptblock]::Create((Invoke-WebRequest -UseBasicParsing -Uri https://raw.githubusercontent.com/rajibdpi/trynas/main/get.ps1).Content)) -AddToPath
```

Alternatively, download the script and run it in your current session:

```powershell
Invoke-WebRequest -UseBasicParsing -Uri https://raw.githubusercontent.com/rajibdpi/trynas/main/get.ps1 -OutFile get.ps1
.\get.ps1 -AddToPath
```

As with `get.sh`, use the branch containing the installer after it is published.
Your PowerShell execution policy must permit the downloaded script under your
organization's rules. The installer does not change execution policy.

From a source checkout, install local release files:

```powershell
.\get.ps1 -Source .\dist -AddToPath
```

Optional parameters:

```powershell
.\get.ps1 -Version v1.0.0 -InstallDir "$HOME\bin"
Get-Help .\get.ps1 -Detailed
```

Use an actual published tag. `-Source` and `-Version` cannot be combined.
The script detects native AMD64/ARM64 Windows, verifies SHA-256, and installs
`trynas.exe` into `%LOCALAPPDATA%\TryNAS\bin` by default. Administrator rights
are not needed for this per-user location. `-AddToPath` updates only your user
PATH and the current session; omit it to leave PATH unchanged. Stop a running
TryNAS instance before upgrading. Failed checksum validation preserves the old
binary, and temporary download/staging files are cleaned up.

### Supported installer platforms

| Platform | CPU | Recommended installer | Default directory |
| --- | --- | --- | --- |
| Linux | AMD64, ARM64 | `get.sh` | `~/.local/bin` |
| macOS | Intel AMD64, Apple ARM64 | `get.sh` | `~/.local/bin` |
| Windows | AMD64, ARM64 | `get.ps1` | `%LOCALAPPDATA%\TryNAS\bin` |
| Windows Git Bash/MSYS/Cygwin | AMD64, ARM64 | `get.sh` | `~/.local/bin` |

The Go installer remains available on all six targets. BSD, 32-bit CPUs, and
Android/Termux are not advertised because the release builder does not produce
those binaries. TryNAS releases use raw binaries, so no archive extraction is
needed. Neither script grants privileged-port capabilities or registers services.

## Automatic installation with Go

From a TryNAS source checkout, install the latest published GitHub release:

```bash
go run ./scripts/install
```

Choose a release tag or installation directory:

```bash
go run ./scripts/install --version v1.0.0
go run ./scripts/install --dir ./bin
go run ./scripts/install --help
```

`v1.0.0` is an example; use an actual published tag. Downloads require a release
in [rajibdpi/trynas](https://github.com/rajibdpi/trynas/releases) containing the
matching binary and `SHA256SUMS`. If assets have not been published, use a local
build instead. The installer does not publish releases.

### Linux and macOS

The default destination is `~/.local/bin/trynas`. Add it to your shell's PATH:

```bash
export PATH="$HOME/.local/bin:$PATH"
```

Add the same line to your shell startup file for future terminals. Then follow
[First start](#first-start). For a local release:

```bash
go run ./scripts/install --source ./dist
```

### Windows

Run the same Go command in PowerShell from the checkout:

```powershell
go run ./scripts/install
```

The default destination is `%LOCALAPPDATA%\TryNAS\bin\trynas.exe`. To use it
in the current PowerShell session:

```powershell
$env:Path = "$env:LOCALAPPDATA\TryNAS\bin;$env:Path"
```

Add that directory to your user PATH through Windows Environment Variables to
make it permanent. For local releases:

```powershell
go run ./scripts/install --source .\dist
```

## Manual installation

Download your binary and `SHA256SUMS` from the same release, or obtain them from
a local `dist` build. Keep the original filename while verifying the checksum.

| Platform | AMD64 | ARM64 |
| --- | --- | --- |
| Linux | `trynas-linux-amd64` | `trynas-linux-arm64` |
| macOS | `trynas-darwin-amd64` | `trynas-darwin-arm64` |
| Windows | `trynas-windows-amd64.exe` | `trynas-windows-arm64.exe` |

On Linux, compare the hash printed by `sha256sum trynas-linux-amd64` with its
entry in `SHA256SUMS`. On macOS, use `shasum -a 256 trynas-darwin-arm64`.
Select the filename matching your CPU. On Windows:

```powershell
Get-FileHash .\trynas-windows-amd64.exe -Algorithm SHA256
Get-Content .\SHA256SUMS
```

After verifying, copy the binary into a directory on PATH and rename it to
`trynas` (`trynas.exe` on Windows). On Unix, make it executable with
`chmod 755 trynas`. You can also run it directly without installing it on PATH.
SHA-256 verification detects damaged or mismatched assets; it relies on obtaining
the binary and manifest from a trusted release.

## Build and install from source

From the project root:

```bash
go run ./scripts/build
go run ./scripts/install --source ./dist
```

Build all supported platforms with `go run ./scripts/build --all`, or selected
ones with `go run ./scripts/build --target linux/arm64,windows/amd64`.
`--skip-frontend` reuses `frontend/dist` and requires only Go, but does not include
new frontend edits. See [Build requirements](README.md#build-requirements).

## GitHub releases

Every push to `main` builds and publishes a release with all six binaries,
checksums, and installers. Release builds contain no administrator credentials. Set a unique runtime
`ADMIN_PASSWORD` before starting each fresh installation.
Automatic releases start at `v1.0.0`, increment the highest stable tag’s patch version,
and become the latest release. Reruns reuse an existing stable tag for the same commit.

The workflow also supports version-tag draft releases.
Commit the workflow, source, installers, and this guide before creating a tag:

```bash
git tag v1.0.0
git push origin v1.0.0
```

Use your actual next version. A `vMAJOR.MINOR.PATCH` tag triggers backend tests,
frontend type checks, installer tests on Linux/macOS/Windows, a fresh frontend
build, and all six Go binaries. GitHub Actions creates a **draft** release with:

- Six platform binaries and `SHA256SUMS`.
- `get.sh`, `get.ps1`, and `installation.md`.

Review the draft under the repository's Releases page and publish it when ready.
Alternatively, open **Actions → GitHub release → Run workflow**, enter an existing
version tag, and select **Publish the release after validation**. Leave it unchecked
to prepare a draft. Manual dispatch requires the workflow on the default branch.
GitHub Actions must be enabled; the publishing job uses the built-in `GITHUB_TOKEN`
with `contents: write`, so no personal access token is required.

Tags such as `v1.0.0-rc.1` produce prereleases. Latest-download installers use the
latest published stable release; use `--version` or `-Version` for prereleases.
Drafts do not provide public download assets. A rerun can replace draft assets,
but the workflow refuses to overwrite a published release: create a new tag.

Local build and installation commands do not publish releases. Tag pushes and
explicit publishing through the workflow are the release actions.

## Persistent storage with Coolify Dockerfile deployments

The Dockerfile declares `VOLUME ["/data", "/storage"]`, but this declaration
alone does not ensure that replacement containers reuse the same storage.
Configure two mounts in your Coolify application's **Persistent Storage**:

| Storage | Destination Path in container |
| --- | --- |
| Persistent volume for the database and encryption key | `/data` |
| Persistent volume for uploaded files | `/storage` |

Keep the same mounts across every redeploy. Set runtime `DB_PATH=/data/trynas.db`
and `STORAGE_ROOT=/storage`. The container runs as UID/GID `10001:10001`;
both mounted directories must be readable and writable by that account.
See [Coolify's persistent storage guide](https://coolify.io/docs/applications/configuration/persistent-storage).

If you already have data, reuse its existing volumes or bind mount its existing
absolute server directory paths at these destinations. New empty volumes do not
automatically recover files from older volumes. Preserve the database, encryption
key, and uploaded files together when recovering an installation.

Before changing mounts, inspect the old and current containers on the deployment
server (replace `CONTAINER` with the actual name or ID):

```sh
docker ps -a
docker inspect CONTAINER --format '{{json .Mounts}}'
docker volume ls
```

Compare the mount sources for `/data` and `/storage`. If they changed, locate and
back up the original storage before reconnecting it. Stop TryNAS before copying
its database. Do not delete old containers or prune volumes until the existing
data has been located and backed up. If the original storage was deleted, restore
from a backup. A Dockerfile change cannot recover deleted storage.

See [Dockerfile deployment settings](README.md#deploy-with-coolify-dockerfile)
for the remaining runtime configuration.

## First start

Create a permanent working directory for configuration and data. Always run the
app from this directory, or use absolute paths in the configuration.

On Linux or macOS:

```bash
mkdir -p ~/trynas
cd ~/trynas
```

On Windows PowerShell:

```powershell
New-Item -ItemType Directory -Force "$HOME\TryNAS" | Out-Null
Set-Location "$HOME\TryNAS"
```

Create `.env` in this directory with your own password:

```dotenv
ADMIN_USER=admin
ADMIN_PASSWORD=replace-with-your-own-unique-password
ADDR=127.0.0.1:8080
DB_PATH=./data/trynas.db
STORAGE_ROOT=./storage
COOKIE_SECURE=false
```

Use an administrator password of 8–72 bytes. Release builds do not generate,
embed, or print an administrator password. Restrict access to `.env`; on Unix:

```bash
chmod 600 .env
```

On Windows, restrict the file's permissions to the account running TryNAS using
the file's Security settings. Start from the configured working directory:

```bash
trynas --config .env
```

Open **http://localhost:8080** and sign in with your configured credentials.
Press **Ctrl+C** to stop. The first launch creates the database and administrator.
`ADMIN_USER` and `ADMIN_PASSWORD` initialize an empty database only; editing them
later does not change existing credentials. Change an existing password through
administrator user management. Password hashes in SQLite cannot reveal the
original password. Do not delete the database to resolve login problems.

## Production and service startup

The default listener is local-only. For network access, choose the listener and
firewall rules deliberately. Use HTTPS through a reverse proxy for production;
set `COOKIE_SECURE=true` and `CORS_ORIGIN` to the exact HTTPS frontend origin.
Bind the backend to loopback when the proxy runs on the same host.

After installing the binary with `get.sh` and creating `~/trynas/.env`,
run from a source checkout as the account that will run TryNAS (without sudo):

```bash
bash scripts/install-service.sh
```

This separate, opt-in helper registers the service without starting it or
modifying your configuration. It expects `~/.local/bin/trynas` and
`~/trynas/.env`; these paths work with custom home directories and either
supported CPU architecture. Stop any old system service or manually running
instance before starting the new service to avoid duplicate processes.

### Linux with systemd

The helper installs `trynas.service` into
`${XDG_CONFIG_HOME:-$HOME/.config}/systemd/user`. This is a **user unit**;
its `%h` specifier follows the service manager's home directory. Do not install
it as a system unit in `/etc/systemd/system`.

```bash
systemctl --user enable --now trynas.service
systemctl --user status trynas.service
journalctl --user -u trynas.service -f
```

To run before login and remain running after logout:

```bash
sudo loginctl enable-linger "$(id -un)"
```

An active systemd user manager is required. Linux distributions using OpenRC,
runit, or another init system must configure that supervisor separately, using
the following command and `~/trynas` as the working directory:

```bash
"$HOME/.local/bin/trynas" -config "$HOME/trynas/.env"
```

### macOS with launchd

The helper generates `~/Library/LaunchAgents/com.trynas.agent.plist` with
absolute paths for your account. Start it from a logged-in desktop session:

```bash
launchctl bootstrap "gui/$(id -u)" "$HOME/Library/LaunchAgents/com.trynas.agent.plist"
launchctl print "gui/$(id -u)/com.trynas.agent"
```

The agent starts at login, restarts after failures, and writes logs to
`~/Library/Logs/TryNAS/`. It runs during your login session, rather than before
login. To restart an already loaded agent:

```bash
launchctl kickstart -k "gui/$(id -u)/com.trynas.agent"
```

After changing its plist, unload it with the following command, then bootstrap
it again:

```bash
launchctl bootout "gui/$(id -u)" "$HOME/Library/LaunchAgents/com.trynas.agent.plist"
```

The service account needs read access to its executable and configuration, and
write access to its database and storage. The download installers do not
register services automatically.

## Upgrade or remove

Stop TryNAS, back up configuration and database, then rerun the installer for the
new release or replace the binary after checksum verification. Restart using the
same working directory and configuration. A running executable is not updated
until restarted. Windows may refuse replacement while it is running.

To uninstall, stop the app and remove the installed executable. Remove any
service you registered separately. Configuration, the database, and stored files
remain in their configured locations; keep them unless you intend to delete data.

## Development

From the source checkout:

```bash
go run ./scripts/dev
```

This starts the frontend on port 3000 and the API on port 8080 by default.
Node.js and pnpm are required. See [Run development servers](README.md#run-development-servers)
for environment configuration, flags, and shutdown behavior.

## Installer verification for maintainers

Run the Go installer and Bash platform/checksum tests from the project root:

```bash
go test -count=1 ./scripts/install
```

The Bash tests simulate all six platform names and verify that modified payloads
cannot replace an existing installation. If PowerShell is available, the Go test
also launches the PowerShell test script. On Windows, run it directly with:

```powershell
powershell -NoProfile -File scripts/install/powershell_test.ps1
```

Those tests cover AMD64/ARM64 selection, installation into paths with spaces,
replacement, checksum mismatch, duplicate manifest entries, and unchanged PATH
without `-AddToPath`. Live release downloads and actual target machines still
need verification before publishing a release.


## HTTPS and deployment security

For direct HTTPS, set `TLS_CERT_FILE` and `TLS_KEY_FILE` to the certificate chain
and private key paths, then start TryNAS normally. Secure cookies and HSTS are
set automatically on direct HTTPS requests. Protect the private key and bind
`ADDR` deliberately. Existing local HTTP development remains supported.

For a reverse proxy, keep TryNAS on a loopback/private listener, terminate HTTPS
at the proxy, set `COOKIE_SECURE=true`, and set `TRUSTED_PROXIES` to the proxy's
exact socket IP(s) or narrowly scoped CIDRs. The proxy must replace forwarded
headers supplied by clients. Configure only the intended HTTPS origins in
`CORS_ORIGIN`. Restrict backend access with the host firewall; trusting a proxy
header does not encrypt the backend connection. Do not expose development HTTP
to untrusted networks.

Fresh installations require a unique runtime `ADMIN_PASSWORD`. Release binaries
contain no shared bootstrap password. Existing database accounts are unchanged.
Rotate passwords on installations that used older shared build defaults: new
binaries do not invalidate an old password automatically. Startup credential
logging remains enabled; restrict access to the terminal and service journal.

Uploads retain the `path`, `files`, and `file` multipart fields. Send `path`
before file contents for early authorization; other field orders still work.
Uploads stream within the user's remaining quota and the 10 GiB request limit,
with up to two active uploads and 10,000 files per request. A busy server returns
429 so clients can retry. Permission and quota checks run again before commit.
Keep sufficient free space on the storage filesystem for the bounded staging
area. Upload staging no longer consumes the OS multipart temporary directory.

`pnpm security:check` verifies the local `sprintf-js` precision patch before
excluding GHSA-hp3w-g68c-fv3c from the registry audit. The registry cannot detect
local patches, so an unfiltered audit still lists the upstream package version.
No other advisories are excluded. Release CI also runs `govulncheck` and pins
GitHub Actions to immutable commits.

Password-protected shares prompt on every new visit, including a browser that
unlocked the link earlier. Reopening the page revokes that browser's download
grant. Downloads and QR codes remain available during the current unlocked
visit; other visitors are unaffected. Unlocked pages relock when restored from
browser history. The share URL never contains an unlock credential.

Administrators can change **Upload proxy limit (MiB)** under Settings → Server
configuration. It saves `PROXY_UPLOAD_LIMIT_MB` to the server configuration file
and applies when the frontend restarts. The default is 10,240 MiB and the valid
range is 1–10,240 MiB. This limit includes multipart framing and all files in one
request; backend quotas and the 10 GiB backend cap still apply. A separately
started frontend reads the project-root `.env`; set `TRYNAS_CONFIG_FILE` when using
a custom configuration file. An explicit `PROXY_UPLOAD_LIMIT_MB` environment
variable takes precedence over the saved value.

File uploads handle name conflicts as follows: matching names with different
byte sizes receive numbered names such as `report(1).txt`, `report(2).txt`.
Matching names and byte sizes open a dialog with Keep existing (skip), Keep both
(numbered copy), and Replace. Equal byte sizes do not imply equal contents.
The same rules apply to files inside a local folder upload. Destination permissions
and quota are rechecked at commit; replacement failure restores the original.

Normal users are restricted to their explicitly assigned home folder and its
recursive contents. Root or outside-folder permission grants cannot widen that
boundary. Administrator permissions for normal users apply only inside their
home; administrators retain full storage access. Direct browser links outside
the home return to the user's home, and the backend independently rejects outside
paths. Legacy accounts without homes are provisioned with username folders when
the app starts; files are not moved. Invalid or conflicting legacy home assignments
remain inaccessible rather than inheriting another folder's permissions.
