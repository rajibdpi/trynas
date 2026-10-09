#!/usr/bin/env bash
# Install a checksum-verified TryNAS release without Go or Node.js.
set -euo pipefail

fail() { printf 'TryNAS installer: %s\n' "$*" >&2; exit 1; }
usage() {
  cat <<'HELP'
Usage: bash get.sh [--version TAG] [--dir DIRECTORY] [--source RELEASE_DIRECTORY]
Default: latest published release, installed into ~/.local/bin.
--source installs a local build with SHA256SUMS instead of downloading.
HELP
}

version=latest
install_dir="${HOME:?HOME must be set}/.local/bin"
source_dir=
while (($#)); do
  case "$1" in
    --version|--dir|--source)
      (($# >= 2)) && [[ -n "$2" ]] || fail "missing value for $1"
      case "$1" in
        --version) version=$2 ;;
        --dir) install_dir=$2 ;;
        --source) source_dir=$2 ;;
      esac
      shift 2 ;;
    --help|-h) usage; exit 0 ;;
    *) fail "unknown argument: $1" ;;
  esac
done
[[ -z "$source_dir" || "$version" == latest ]] || fail '--source and --version cannot be combined'
[[ "$version" =~ ^[A-Za-z0-9._-]+$ ]] || fail 'invalid release tag'

case "$(uname -s)" in
  Linux) platform=linux ;;
  Darwin) platform=darwin ;;
  MINGW*|MSYS*|CYGWIN*) platform=windows ;;
  *) fail 'unsupported OS; supported: Linux, macOS, Windows with Bash' ;;
esac
native_cpu=$(uname -m)
if [[ "$platform" == windows ]]; then
  native_cpu=${PROCESSOR_ARCHITEW6432:-${PROCESSOR_ARCHITECTURE:-$native_cpu}}
fi
case "$native_cpu" in
  x86_64|amd64|AMD64) architecture=amd64 ;;
  aarch64|arm64|ARM64) architecture=arm64 ;;
  *) fail 'unsupported CPU; supported: AMD64 and ARM64' ;;
esac
asset="trynas-$platform-$architecture"
executable=trynas
if [[ "$platform" == windows ]]; then asset+=.exe; executable+=.exe; fi

# Private download directory; every exit path removes temporary files.
temporary=$(mktemp -d)
staged=
cleanup() {
  rm -rf -- "$temporary"
  if [[ -n "$staged" ]]; then rm -f -- "$staged"; fi
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

fetch() {
  if command -v curl >/dev/null 2>&1; then
    curl --fail --silent --show-error --location --proto '=https' --proto-redir '=https' --connect-timeout 15 --max-time 300 --output "$2" "$1"
  elif command -v wget >/dev/null 2>&1; then
    wget --https-only --timeout=30 --tries=2 -q -O "$2" "$1"
  else
    fail 'install curl or wget to download releases, or use --source'
  fi
}

printf 'Installing TryNAS for %s/%s...\n' "$platform" "$architecture"
if [[ -n "$source_dir" ]]; then
  cp -- "$source_dir/SHA256SUMS" "$temporary/SHA256SUMS"
  cp -- "$source_dir/$asset" "$temporary/$asset"
else
  base=https://github.com/rajibdpi/trynas/releases
  if [[ "$version" == latest ]]; then base+=/latest/download; else base+="/download/$version"; fi
  fetch "$base/SHA256SUMS" "$temporary/SHA256SUMS"
  fetch "$base/$asset" "$temporary/$asset"
fi

expected=$(awk -v name="$asset" 'NF == 2 && ($2 == name || $2 == "*" name) { print tolower($1) }' "$temporary/SHA256SUMS")
[[ "$expected" =~ ^[0-9a-f]{64}$ ]] || fail "missing, invalid, or duplicate SHA256SUMS entry for $asset"
if command -v sha256sum >/dev/null 2>&1; then
  actual=$(sha256sum "$temporary/$asset")
elif command -v shasum >/dev/null 2>&1; then
  actual=$(shasum -a 256 "$temporary/$asset")
else
  fail 'SHA-256 verification requires sha256sum or shasum'
fi
actual=${actual%% *}
[[ "$actual" == "$expected" ]] || fail 'checksum mismatch; existing installation was not replaced'

mkdir -p -- "$install_dir"
# Stage beside the destination so replacement stays on the same filesystem.
staged=$(mktemp "$install_dir/.trynas-install-XXXXXX")
cp -- "$temporary/$asset" "$staged"
chmod 755 "$staged"
[[ ! -d "$install_dir/$executable" ]] || fail 'destination is a directory'
mv -f -- "$staged" "$install_dir/$executable"
staged=
printf 'Installed %s/%s (SHA-256 verified).\n' "$install_dir" "$executable"
printf 'Add %s to PATH. Set ADMIN_PASSWORD before the first launch.\n' "$install_dir"
