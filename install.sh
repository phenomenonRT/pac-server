#!/usr/bin/env sh
set -eu

REPO="${REPO:-phenomenonRT/pac-server}"
VERSION="${VERSION:-latest}"
BIN_NAME="pac-server"
if [ -n "${INSTALL_DIR:-}" ]; then
  INSTALL_DIR="$INSTALL_DIR"
elif [ -d /opt/etc/init.d ]; then
  INSTALL_DIR="/opt/bin"
elif [ -f /etc/openwrt_release ]; then
  INSTALL_DIR="/usr/bin"
else
  INSTALL_DIR="/usr/local/bin"
fi
if [ -n "${CONFIG_DIR:-}" ]; then
  CONFIG_DIR="$CONFIG_DIR"
elif [ -d /opt/etc/init.d ]; then
  CONFIG_DIR="/opt/etc/pac-server"
else
  CONFIG_DIR="/etc/pac-server"
fi
SERVICE_FILE="/etc/systemd/system/pac-server.service"
OPENWRT_SERVICE_FILE="/etc/init.d/pac-server"
ENTWARE_SERVICE_FILE="/opt/etc/init.d/S99pac-server"

need_root() {
  if [ "$(id -u)" -ne 0 ]; then
    echo "Run as root: sudo sh install.sh"
    exit 1
  fi
}

detect_target() {
  machine="$(uname -m)"
  case "$machine" in
    x86_64|amd64) echo "linux-amd64" ;;
    i386|i686) echo "linux-386" ;;
    aarch64|arm64) echo "linux-arm64" ;;
    armv7l|armv7*) echo "linux-armv7" ;;
    armv6l|armv6*) echo "linux-armv6" ;;
    armv5l|armv5*) echo "linux-armv5" ;;
    mips) echo "linux-mips-softfloat" ;;
    mipsel|mipsle) echo "linux-mipsle-softfloat" ;;
    *)
      echo "Unsupported architecture: $machine" >&2
      exit 1
      ;;
  esac
}

download_url() {
  target="$1"
  if [ "$VERSION" = "latest" ]; then
    echo "https://github.com/$REPO/releases/latest/download/pac-server-$target.tar.gz"
  else
    echo "https://github.com/$REPO/releases/download/$VERSION/pac-server-$target.tar.gz"
  fi
}

fetch() {
  url="$1"
  out="$2"
  if command -v curl >/dev/null 2>&1; then
    curl -fL "$url" -o "$out"
  elif command -v wget >/dev/null 2>&1; then
    wget -O "$out" "$url"
  else
    echo "curl or wget is required" >&2
    exit 1
  fi
}

install_service() {
  cat > "$SERVICE_FILE" <<UNIT
[Unit]
Description=PAC Server
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=root
Group=root
Environment=PAC_CONFIG=$CONFIG_DIR/config.json
ExecStart=$INSTALL_DIR/pac-server
Restart=on-failure
RestartSec=3

[Install]
WantedBy=multi-user.target
UNIT
}

install_openwrt_service() {
  cat > "$OPENWRT_SERVICE_FILE" <<OPENWRT_INIT
#!/bin/sh /etc/rc.common

START=95
STOP=10
USE_PROCD=1

start_service() {
  procd_open_instance
  procd_set_param command $INSTALL_DIR/pac-server
  procd_set_param env PAC_CONFIG=$CONFIG_DIR/config.json
  procd_set_param respawn 3600 5 5
  procd_close_instance
}
OPENWRT_INIT
  chmod 0755 "$OPENWRT_SERVICE_FILE"
}

install_entware_service() {
  mkdir -p /opt/etc/init.d
  cat > "$ENTWARE_SERVICE_FILE" <<ENTWARE_INIT
#!/bin/sh

ENABLED=yes
PROCS=pac-server
ARGS=""
PREARGS="env PAC_CONFIG=$CONFIG_DIR/config.json"
DESC="PAC Server"
PATH=/opt/sbin:/opt/bin:/usr/sbin:/usr/bin:/sbin:/bin

. /opt/etc/init.d/rc.func
ENTWARE_INIT
  chmod 0755 "$ENTWARE_SERVICE_FILE"
}

install_config() {
  mkdir -p "$CONFIG_DIR"
  if [ ! -f "$CONFIG_DIR/config.json" ]; then
    cat > "$CONFIG_DIR/config.json" <<'JSON'
{
  "listen_ip": "127.0.0.1",
  "listen_port": 81,
  "profiles": [
    {
      "name": "Default",
      "slug": "default",
      "proxy_type": "SOCKS5",
      "proxy_host": "127.0.0.1",
      "proxy_port": 1080,
      "proxy": "SOCKS5 127.0.0.1:1080",
      "fallback": "DIRECT",
      "direct_domains": ["localhost", "local", "lan"],
      "proxy_domains": []
    }
  ]
}
JSON
  fi
}

port_from_config() {
  if [ -f "$CONFIG_DIR/config.json" ]; then
    grep -o '"listen_port"[[:space:]]*:[[:space:]]*[0-9]*' "$CONFIG_DIR/config.json" \
      | head -n1 | grep -o '[0-9]*$'
  fi
}

# Открывает доступ к порту pac-server только с loopback (lo) и LAN-моста (br0),
# WAN не трогает. Работает только там, где есть uci (OpenWrt / Keenetic NDMS uci-shim).
install_openwrt_firewall() {
  if ! command -v uci >/dev/null 2>&1; then
    return
  fi

  port="$(port_from_config)"
  if [ -z "${port:-}" ]; then
    port=81
  fi

  for entry in "pac_server_lo:lo" "pac_server_lan:br0"; do
    name="${entry%%:*}"
    dev="${entry#*:}"
    uci set firewall."$name"="rule"
    uci set firewall."$name".name="PAC Server ($dev)"
    uci set firewall."$name".target='ACCEPT'
    uci set firewall."$name".proto='tcp'
    uci set firewall."$name".dest_port="$port"
    uci set firewall."$name".device="$dev"
    uci set firewall."$name".direction='in'
  done

  uci commit firewall
  /etc/init.d/firewall reload >/dev/null 2>&1 || true
  echo "Firewall: открыт TCP/$port для интерфейсов lo и br0 (WAN не тронут)"
}

main() {
  need_root
  target="$(detect_target)"
  tmp_dir="$(mktemp -d)"
  archive="$tmp_dir/$BIN_NAME.tar.gz"
  url="$(download_url "$target")"

  echo "Downloading $url"
  fetch "$url" "$archive"

  tar -xzf "$archive" -C "$tmp_dir"
  if ! mkdir -p "$INSTALL_DIR" 2>/dev/null || ! cp "$tmp_dir/$BIN_NAME" "$INSTALL_DIR/$BIN_NAME" 2>/dev/null; then
    if [ -d /opt/bin ]; then
      INSTALL_DIR="/opt/bin"
      mkdir -p "$INSTALL_DIR"
      cp "$tmp_dir/$BIN_NAME" "$INSTALL_DIR/$BIN_NAME"
    else
      echo "Cannot write to $INSTALL_DIR and /opt/bin does not exist." >&2
      echo "Set writable install dir manually, for example: INSTALL_DIR=/opt/bin sh install.sh" >&2
      exit 1
    fi
  fi
  chmod 0755 "$INSTALL_DIR/$BIN_NAME"
  install_config

  if command -v systemctl >/dev/null 2>&1; then
    install_service
    systemctl daemon-reload
    systemctl enable pac-server
    systemctl restart pac-server
    systemctl status pac-server --no-pager || true
  elif [ -f /etc/openwrt_release ] && [ -d /etc/init.d ]; then
    install_openwrt_service
    install_openwrt_firewall
    "$OPENWRT_SERVICE_FILE" enable
    "$OPENWRT_SERVICE_FILE" restart
    echo "OpenWrt service installed: $OPENWRT_SERVICE_FILE"
  elif [ -d /opt/etc/init.d ]; then
    install_entware_service
    install_openwrt_firewall
    "$ENTWARE_SERVICE_FILE" start || true
    echo "Entware service installed: $ENTWARE_SERVICE_FILE"
  else
    echo "systemd not found. Binary installed to $INSTALL_DIR/$BIN_NAME"
  fi

  rm -rf "$tmp_dir"
  echo "Installed. Config: $CONFIG_DIR/config.json"
}

main "$@"
