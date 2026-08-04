#!/usr/bin/env sh
set -eu

if [ "$(id -u)" -ne 0 ]; then
  echo "Run as root: sudo sh uninstall.sh"
  exit 1
fi

if command -v systemctl >/dev/null 2>&1; then
  systemctl stop pac-server 2>/dev/null || true
  systemctl disable pac-server 2>/dev/null || true
fi

rm -f /etc/systemd/system/pac-server.service
rm -f /usr/local/bin/pac-server

if command -v systemctl >/dev/null 2>&1; then
  systemctl daemon-reload
fi

echo "Removed pac-server binary and service."
echo "Config directory was kept: /etc/pac-server"
