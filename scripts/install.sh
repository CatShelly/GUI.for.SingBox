#!/usr/bin/env bash
set -euo pipefail
umask 077

# Usage: sudo bash install.sh [listen-address:port]
repo='CatShelly/GUI.for.SingBox'
core_version='1.14.4'
install_dir='/opt/singbox-webui'
service='singbox-webui.service'
unit="/etc/systemd/system/$service"
webui_binary="$install_dir/singbox-webui"
core_dir="$install_dir/data/sing-box"
core_binary="$core_dir/sing-box"

die() { printf 'Error: %s\n' "$*" >&2; exit 1; }

normalize_listen() {
  local address_pattern='^(\[[0-9a-fA-F:]+\]|[a-zA-Z0-9][a-zA-Z0-9.-]*):([0-9]{1,5})$'
  [[ "$1" =~ $address_pattern ]] || die 'Invalid listen address. Examples: 0.0.0.0:12500 or [::]:12500'
  local port=$((10#${BASH_REMATCH[2]}))
  ((port >= 1 && port <= 65535)) || die 'Port must be between 1 and 65535.'
  printf '%s:%s\n' "${BASH_REMATCH[1]}" "$port"
}

[[ $# -le 1 ]] || die 'Usage: install.sh [listen-address:port]'
requested_listen=$(normalize_listen "${1:-0.0.0.0:9090}")
[[ $(uname -s) == Linux ]] || die 'Linux is required.'
[[ $EUID -eq 0 ]] || die 'Run this script with sudo or as root.'
for command in curl tar sha256sum sed grep install mktemp systemctl flock cp mv readlink; do
  command -v "$command" >/dev/null || die "Required command not found: $command"
done
[[ -d /run/systemd/system ]] || die 'A running systemd installation is required.'
case "$(uname -m)" in
  x86_64)
    arch=amd64
    core_checksum='699a1b510aa4396b2176d32d1181920399ebf1e267835da6f3b598d0f7c24968'
    ;;
  aarch64|arm64)
    arch=arm64
    core_checksum='436441f76ee2f7fe21b5b2d426d3cb261a480455a704865789ad533e4273ec26'
    ;;
  *) die 'Only Linux amd64 and arm64 are supported.' ;;
esac

exec 9>/run/singbox-webui-install.lock
flock -n 9 || die 'Another installation or upgrade is in progress.'

listen="$requested_listen"
upgrade=0
if [[ -e "$install_dir" || -L "$install_dir" || -e "$unit" || -L "$unit" ]] ||
  systemctl cat "$service" >/dev/null 2>&1; then
  [[ -d "$install_dir" && ! -L "$install_dir" && -f "$webui_binary" && ! -L "$webui_binary" &&
     -d "$core_dir" && ! -L "$core_dir" && ! -L "$install_dir/data" &&
     -f "$unit" && ! -L "$unit" ]] || die 'Existing files do not match a one-click installation.'
  [[ $(systemctl show --property=FragmentPath --value "$service") == "$unit" &&
     $(systemctl show --property=WorkingDirectory --value "$service") == "$install_dir" ]] ||
    die 'Existing systemd service uses a different installation layout.'
  exec_start=$(systemctl show --property=ExecStart --value "$service")
  data_dir_pattern='[[:space:]]-data-dir(=|[[:space:]])[.]/?([[:space:]]|$)'
  listen_pattern='[[:space:]]-listen(=|[[:space:]])([^[:space:];]+)'
  [[ "$exec_start" == *"argv[]=$webui_binary "* && "$exec_start" =~ $data_dir_pattern &&
     "$exec_start" =~ $listen_pattern ]] || die 'Cannot identify the existing service command.'
  listen=$(normalize_listen "${BASH_REMATCH[2]}")
  [[ $# -eq 0 || "$requested_listen" == "$listen" ]] ||
    die "Existing service listens on $listen. Run the script without an address to upgrade."
  [[ ! -L "$core_binary" && ( ! -e "$core_binary" || -f "$core_binary" ) ]] ||
    die 'Existing stable core is not a regular file.'
  upgrade=1
fi

host=${listen%:*}
port=${listen##*:}
probe_host="$host"
display_host="$host"
case "$host" in
  0.0.0.0) probe_host=127.0.0.1; display_host='<server IP>' ;;
  '[::]') probe_host='[::1]'; display_host='<server IP>' ;;
esac
probe_url="http://$probe_host:$port/api/session"
webui_url="http://$display_host:$port"

tmp=$(mktemp -d)
webui_stage=''
core_stage=''
rollback_needed=0
was_active=0
had_core=0
expect_core=0

cleanup() {
  local status=$? restored=1
  if ((rollback_needed)); then
    printf 'Upgrade failed. Restoring previous executables.\n' >&2
    systemctl stop "$service" || restored=0
    cp -p -- "$webui_stage/previous" "$webui_stage/restore" &&
      mv -fT -- "$webui_stage/restore" "$webui_binary" || restored=0
    if ((had_core)); then
      cp -p -- "$core_stage/previous" "$core_stage/restore" &&
        mv -fT -- "$core_stage/restore" "$core_binary" || restored=0
    else
      rm -f -- "$core_binary" || restored=0
    fi
    if ((restored && was_active)); then
      systemctl reset-failed "$service" >/dev/null 2>&1 || true
      systemctl start "$service" && wait_ready || restored=0
    fi
    if ((!restored)); then
      printf 'Rollback incomplete. Backups are in %s and %s.\n' "$webui_stage" "$core_stage" >&2
      rm -rf -- "$tmp"
      return 1
    fi
  fi
  rm -rf -- "$tmp"
  [[ -z "$webui_stage" ]] || rm -rf -- "$webui_stage"
  [[ -z "$core_stage" ]] || rm -rf -- "$core_stage"
  return "$status"
}

core_running() {
  local pid='' executable
  [[ -r "$core_dir/pid.txt" ]] || return 1
  IFS= read -r pid < "$core_dir/pid.txt" || [[ -n "$pid" ]] || return 1
  [[ "$pid" =~ ^[1-9][0-9]*$ ]] || return 1
  executable=$(readlink "/proc/$pid/exe") || return 1
  executable=${executable% (deleted)}
  [[ "$executable" == "$core_binary" || "$executable" == "$core_binary-latest" ]]
}

wait_ready() {
  local attempt
  for ((attempt=0; attempt<30; attempt++)); do
    if systemctl is-active --quiet "$service" &&
      curl --fail --silent --noproxy '*' --max-time 2 "$probe_url" >/dev/null &&
      { ((!expect_core)) || core_running; }; then
      return 0
    fi
    sleep 1
  done
  return 1
}

trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
download() { curl --fail --location --retry 3 --connect-timeout 20 --max-time 600 "$1" -o "$2"; }
release="https://github.com/$repo/releases/latest/download"
package="singbox-webui-linux-$arch"
printf 'Downloading latest WebUI and sing-box %s (%s)...\n' "$core_version" "$arch"
download "$release/$package.tar.gz" "$tmp/$package.tar.gz"
download "$release/SHA256SUMS" "$tmp/SHA256SUMS"
checksum=$(grep -E "^[0-9a-fA-F]{64}  $package\\.tar\\.gz$" "$tmp/SHA256SUMS") || die 'WebUI checksum not found.'
(cd "$tmp" && printf '%s\n' "$checksum" | sha256sum --check --status) || die 'WebUI checksum verification failed.'
core_package="sing-box-$core_version-linux-$arch"
download "https://github.com/SagerNet/sing-box/releases/download/v$core_version/$core_package.tar.gz" "$tmp/core.tar.gz"
(cd "$tmp" && printf '%s  core.tar.gz\n' "$core_checksum" | sha256sum --check --status) ||
  die 'sing-box checksum verification failed.'
tar --no-same-owner --no-same-permissions -xzf "$tmp/$package.tar.gz" -C "$tmp"
tar --no-same-owner --no-same-permissions -xzf "$tmp/core.tar.gz" -C "$tmp"
[[ -f "$tmp/$package/singbox-webui" && -f "$tmp/$core_package/sing-box" ]] || die 'Release archive is missing its executable.'
chmod +x "$tmp/$core_package/sing-box"
"$tmp/$core_package/sing-box" version

readiness_error='Service did not become ready. Check: journalctl -u singbox-webui -n 50 --no-pager'
if ((upgrade)); then
  webui_stage=$(mktemp -d "$install_dir/.upgrade.XXXXXX")
  core_stage=$(mktemp -d "$core_dir/.upgrade.XXXXXX")
  install -m 0755 "$tmp/$package/singbox-webui" "$webui_stage/new"
  install -m 0755 "$tmp/$core_package/sing-box" "$core_stage/new"
  cp -p -- "$webui_binary" "$webui_stage/previous"
  if [[ -f "$core_binary" ]]; then
    cp -p -- "$core_binary" "$core_stage/previous"
    had_core=1
  fi
  if systemctl is-active --quiet "$service"; then
    was_active=1
    if core_running; then
      expect_core=1
    fi
  fi

  rollback_needed=1
  systemctl stop "$service"
  mv -fT -- "$webui_stage/new" "$webui_binary"
  mv -fT -- "$core_stage/new" "$core_binary"
  if ((was_active)); then
    systemctl start "$service"
    wait_ready || die "$readiness_error"
  fi
  rollback_needed=0

  printf '\nUpgrade complete.\nWebUI: %s\n' "$webui_url"
  if ((!was_active)); then
    printf 'Service is stopped. Start it with: sudo systemctl start %s\n' "$service"
  fi
  exit 0
fi

install -d -m 0755 "$install_dir"
install -m 0755 "$tmp/$package/singbox-webui" "$webui_binary"
install -d -m 0700 "$install_dir/data" "$core_dir"
install -m 0755 "$tmp/$core_package/sing-box" "$core_binary"
for file in README.md LICENSE; do
  install -m 0644 "$tmp/$package/$file" "$install_dir/$file"
done
cat > "$unit" <<UNIT
[Unit]
Description=SingBox WebUI
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=root
WorkingDirectory=$install_dir
ExecStart=$webui_binary -listen $listen -data-dir ./
Restart=on-failure
RestartSec=3
TimeoutStopSec=30
KillMode=control-group
UMask=0077

[Install]
WantedBy=multi-user.target
UNIT
chmod 0644 "$unit"
systemctl daemon-reload
systemctl enable --now "$service"
wait_ready || die "$readiness_error"

password=$(sed -nE 's/^webuiPassword: "([0-9a-f]{48})"$/\1/p' "$install_dir/data/user.yaml")
[[ -n "$password" ]] || die "Initial password not found in $install_dir/data/user.yaml."
printf '\nInstallation complete.\nWebUI: %s\nPassword: %s\n' "$webui_url" "$password"
printf 'Change webuiPassword in %s/data/user.yaml, then run:\n  sudo systemctl restart singbox-webui\n' "$install_dir"
