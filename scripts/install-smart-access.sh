#!/usr/bin/env bash
set -euo pipefail
# Installs a scoped policy for read-only SMART commands, not full sudo access.
if [[ $EUID -ne 0 ]]; then
  echo 'Run: sudo ./scripts/install-smart-access.sh YOUR_API_USERNAME' >&2
  exit 1
fi
api_user="${1:-${SUDO_USER:-}}"
if [[ ! "$api_user" =~ ^[a-z_][a-z0-9_.-]*\$?$ ]] || ! id "$api_user" >/dev/null 2>&1; then
  echo 'Supply the existing Linux account that runs the API.' >&2
  exit 1
fi
smart_binary=/usr/sbin/smartctl
if [[ ! -x "$smart_binary" || $(stat -c %u "$smart_binary") != 0 ]]; then
  echo 'Install smartmontools with root-owned /usr/sbin/smartctl first.' >&2
  exit 1
fi
policy_tmp=$(mktemp)
trap 'rm -f "$policy_tmp"' EXIT
{
  echo '# TryNAS: only fixed read-only SMART queries are permitted.'
  printf '%s ALL=(root) NOPASSWD: %s --scan-open -j\n' "$api_user" "$smart_binary"
  for device in /dev/sda /dev/sdb /dev/sdc /dev/nvme0; do
    printf '%s ALL=(root) NOPASSWD: %s -a -j %s\n' "$api_user" "$smart_binary" "$device"
    if [[ $device == /dev/nvme* ]]; then types=(nvme); else types=(scsi sat ata); fi
    for device_type in "${types[@]}"; do
      printf '%s ALL=(root) NOPASSWD: %s -a -j %s -d %s\n' "$api_user" "$smart_binary" "$device" "$device_type"
    done
  done
} > "$policy_tmp"
visudo -cf "$policy_tmp"
install -o root -g root -m 0440 "$policy_tmp" /etc/sudoers.d/trynas-smartctl
printf 'Installed read-only SMART access for %s on sda, sdb, sdc and nvme0.\n' "$api_user"
echo 'Set SMART_USE_SUDO=true in .env and restart the API, then refresh Disk health.'
