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

No Go installation or source checkout is required. Download the installer, then run it:

```bash
curl -fsSL https://raw.githubusercontent.com/rajibdpi/trynas/master/get.sh -o get.sh
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
Download the script and run it in your current session:

```powershell
Invoke-WebRequest -UseBasicParsing -Uri https://raw.githubusercontent.com/rajibdpi/trynas/master/get.ps1 -OutFile get.ps1
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

Release maintainers must upload the matching `trynas-*` files and `SHA256SUMS`
as assets on the same GitHub release tag for remote installation to work.
Publish all six binaries when offering installation for every supported platform.
Builds and local installation do not automatically publish anything.

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

For Linux systemd, follow [Install on Ubuntu with systemd](README.md#install-on-ubuntu-with-systemd).
The supplied unit has example user and project paths: update `User`, `Group`,
`WorkingDirectory`, `ExecStart`, and `RequiresMountsFor` for your installation.
The service account needs read access to the executable/configuration and write
access to the configured database and storage. The installer does not register
systemd, launchd, or Windows services automatically.

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
