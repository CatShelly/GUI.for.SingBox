#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
arch="${1:-amd64}"
case "$arch" in amd64|arm64) ;; *) echo 'Usage: build-linux.sh [amd64|arm64]' >&2; exit 1;; esac
pnpm --dir frontend install --frozen-lockfile
pnpm --dir frontend build
go test ./...
out="dist/singbox-webui-linux-$arch"
mkdir -p "$out/deploy"
CGO_ENABLED=0 GOOS=linux GOARCH="$arch" go build -trimpath -o "$out/singbox-webui" .
cp deploy/* "$out/deploy/"
cp README.md LICENSE "$out/"
package="singbox-webui-linux-$arch"
tar -C dist -czf "$out.tar.gz" "$package/singbox-webui" "$package/README.md" "$package/LICENSE" "$package/deploy/singbox-webui.service" "$package/deploy/nginx.conf.example"
echo "$out.tar.gz"
