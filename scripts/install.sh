#!/usr/bin/env bash
set -euo pipefail
umask 077

# Usage: sudo bash install.sh [listen-address:port]
repo='CatShelly/GUI.for.SingBox'
listen="${1:-0.0.0.0:9090}"
core_version='1.14.1'
install_dir='/opt/singbox-webui'
unit='/etc/systemd/system/singbox-webui.service'

die() { printf 'Error: %s\n' "$*" >&2; exit 1; }
[[ $# -le 1 ]] || die 'Usage: install.sh [listen-address:port]'
address_pattern='^(\[[0-9a-fA-F:]+\]|[a-zA-Z0-9][a-zA-Z0-9.-]*):([0-9]{1,5})$'
[[ "$listen" =~ $address_pattern ]] || die 'Invalid listen address. Examples: 0.0.0.0:12500 or [::]:12500'
host="${BASH_REMATCH[1]}"
port=$((10#${BASH_REMATCH[2]}))
((port >= 1 && port <= 65535)) || die 'Port must be between 1 and 65535.'
listen="$host:$port"
probe_host="$host"
display_host="$host"
case "$host" in
  0.0.0.0) probe_host=127.0.0.1; display_host='<server IP>' ;;
  '[::]') probe_host='[::1]'; display_host='<server IP>' ;;
esac
probe_url="http://$probe_host:$port/api/session"
webui_url="http://$display_host:$port"
[[ $(uname -s) == Linux ]] || die 'Linux is required.'
[[ $EUID -eq 0 ]] || die 'Run this script with sudo or as root.'
for command in curl tar sha256sum sed grep install mktemp systemctl; do
  command -v "$command" >/dev/null || die "Required command not found: $command"
done
[[ -d /run/systemd/system ]] || die 'A running systemd installation is required.'
case "$(uname -m)" in
  x86_64) arch=amd64 ;;
  aarch64|arm64) arch=arm64 ;;
  *) die 'Only Linux amd64 and arm64 are supported.' ;;
esac
[[ ! -e "$install_dir" && ! -L "$install_dir" ]] || die "$install_dir already exists; installation stopped to preserve existing data."
[[ ! -e "$unit" && ! -L "$unit" ]] || die "$unit already exists."
if systemctl cat singbox-webui.service >/dev/null 2>&1; then
  die 'A singbox-webui service is already installed.'
fi

tmp=$(mktemp -d)
trap 'rm -rf -- "$tmp"' EXIT
download() { curl --fail --location --retry 3 --connect-timeout 20 --max-time 600 "$1" -o "$2"; }
release="https://github.com/$repo/releases/latest/download"
package="singbox-webui-linux-$arch"
printf 'Downloading latest WebUI and sing-box %s (%s)...\n' "$core_version" "$arch"
download "$release/$package.tar.gz" "$tmp/$package.tar.gz"
download "$release/SHA256SUMS" "$tmp/SHA256SUMS"
checksum=$(grep -E "^[0-9a-fA-F]{64}  $package\.tar\.gz$" "$tmp/SHA256SUMS") || die 'WebUI checksum not found.'
(cd "$tmp" && printf '%s\n' "$checksum" | sha256sum --check --status) || die 'WebUI checksum verification failed.'
core_package="sing-box-$core_version-linux-$arch"
download "https://github.com/SagerNet/sing-box/releases/download/v$core_version/$core_package.tar.gz" "$tmp/core.tar.gz"
tar -xzf "$tmp/$package.tar.gz" -C "$tmp"
tar -xzf "$tmp/core.tar.gz" -C "$tmp"
[[ -f "$tmp/$package/singbox-webui" && -f "$tmp/$core_package/sing-box" ]] || die 'Release archive is missing its executable.'
chmod +x "$tmp/$core_package/sing-box"
"$tmp/$core_package/sing-box" version

install -d -m 0755 "$install_dir"
install -m 0755 "$tmp/$package/singbox-webui" "$install_dir/singbox-webui"
install -d -m 0700 "$install_dir/data" "$install_dir/data/sing-box"
install -m 0755 "$tmp/$core_package/sing-box" "$install_dir/data/sing-box/sing-box"
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
WorkingDirectory=/opt/singbox-webui
ExecStart=/opt/singbox-webui/singbox-webui -listen $listen -data-dir ./
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
systemctl enable --now singbox-webui.service

password=''
for ((attempt=0; attempt<30; attempt++)); do
  if systemctl is-active --quiet singbox-webui.service &&
    curl --fail --silent --noproxy '*' --max-time 2 "$probe_url" >/dev/null; then
    if [[ -f "$install_dir/data/user.yaml" ]]; then
      password=$(sed -nE 's/^webuiPassword: "([0-9a-f]{48})"$/\1/p' "$install_dir/data/user.yaml")
      [[ -n "$password" ]] && break
    fi
  fi
  sleep 1
done
[[ -n "$password" ]] || die 'Service did not become ready. Check: journalctl -u singbox-webui -n 50 --no-pager'
printf '\nInstallation complete.\nWebUI: %s\nPassword: %s\n' "$webui_url" "$password"
printf 'Change webuiPassword in %s/data/user.yaml, then run:\n  sudo systemctl restart singbox-webui\n' "$install_dir"
