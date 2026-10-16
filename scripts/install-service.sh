#!/usr/bin/env bash
# Register a per-user service; intentionally does not start the application.
set -euo pipefail
umask 077
script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
: "${HOME:?HOME must be set}"
[[ $HOME == /* ]] || { echo 'HOME must be an absolute path.' >&2; exit 1; }
if [[ ${1:-} == --help ]]; then
  echo 'Usage: bash scripts/install-service.sh (run as the account that will run TryNAS)'
  exit 0
fi
[[ $# == 0 ]] || { echo 'Unexpected arguments; use --help.' >&2; exit 1; }
[[ -x "$HOME/.local/bin/trynas" ]] || { echo 'Install TryNAS into ~/.local/bin first using get.sh.' >&2; exit 1; }
[[ -f "$HOME/trynas/.env" ]] || { echo 'Create ~/trynas/.env first; see installation.md.' >&2; exit 1; }

xml_escape() {
  local value=$1
  value=${value//&/\&amp;}
  value=${value//</\&lt;}
  value=${value//>/\&gt;}
  printf '%s' "$value"
}

case $(uname -s) in
  Linux)
    command -v systemctl >/dev/null 2>&1 || {
      echo 'This Linux host does not use systemd. Configure its supervisor to run:' >&2
      printf 'cd %q && exec %q -config %q\n' "$HOME/trynas" "$HOME/.local/bin/trynas" "$HOME/trynas/.env" >&2
      exit 1
    }
    unit_dir=${XDG_CONFIG_HOME:-$HOME/.config}/systemd/user
    mkdir -p "$unit_dir"
    cp "$script_dir/trynas.service" "$unit_dir/trynas.service"
    systemctl --user daemon-reload
    echo 'Installed. Start with: systemctl --user enable --now trynas.service'
    echo 'Logs: journalctl --user -u trynas.service -f'
    echo 'For startup before login: sudo loginctl enable-linger "$(id -un)"'
    ;;
  Darwin)
    agent_dir=$HOME/Library/LaunchAgents
    log_dir=$HOME/Library/Logs/TryNAS
    mkdir -p "$agent_dir" "$log_dir"
    escaped_home=$(xml_escape "$HOME")
    plist=$agent_dir/com.trynas.agent.plist
    cat > "$plist" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
  <key>Label</key><string>com.trynas.agent</string>
  <key>ProgramArguments</key><array>
    <string>$escaped_home/.local/bin/trynas</string>
    <string>-config</string><string>$escaped_home/trynas/.env</string>
  </array>
  <key>WorkingDirectory</key><string>$escaped_home/trynas</string>
  <key>RunAtLoad</key><true/>
  <key>KeepAlive</key><dict><key>SuccessfulExit</key><false/></dict>
  <key>ThrottleInterval</key><integer>5</integer>
  <key>ExitTimeOut</key><integer>30</integer>
  <key>Umask</key><integer>63</integer>
  <key>StandardOutPath</key><string>$escaped_home/Library/Logs/TryNAS/stdout.log</string>
  <key>StandardErrorPath</key><string>$escaped_home/Library/Logs/TryNAS/stderr.log</string>
</dict></plist>
EOF
    plutil -lint "$plist"
    echo 'Installed. Start from a logged-in macOS session with:'
    echo 'launchctl bootstrap "gui/$(id -u)" "$HOME/Library/LaunchAgents/com.trynas.agent.plist"'
    echo 'If already loaded, use: launchctl kickstart -k "gui/$(id -u)/com.trynas.agent"'
    echo 'Starts automatically at login. Logs: ~/Library/Logs/TryNAS/'
    ;;
  *) echo 'Service installation supports Linux (systemd) and macOS (launchd).' >&2; exit 1 ;;
esac
