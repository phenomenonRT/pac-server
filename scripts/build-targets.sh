#!/usr/bin/env sh
set -eu

ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
DIST="$ROOT/dist"
mkdir -p "$DIST"
cd "$ROOT"

build() {
  name="$1"
  goarch="$2"
  goarm="${3:-}"
  gomips="${4:-}"

  echo "Building $name -> $DIST/pac-server-$name"
  if [ -n "$goarm" ]; then
    GOARM="$goarm" export GOARM
  else
    unset GOARM || true
  fi

  if [ -n "$gomips" ]; then
    GOMIPS="$gomips" export GOMIPS
  else
    unset GOMIPS || true
  fi

  CGO_ENABLED=0 GOOS=linux GOARCH="$goarch" \
    go build -buildvcs=false -trimpath -ldflags "-s -w" -o "$DIST/pac-server-$name" ./cmd/pac-server
}

build openwrt-amd64 amd64
build openwrt-386 386
build openwrt-arm64 arm64
build openwrt-armv5 arm 5
build openwrt-armv6 arm 6
build openwrt-armv7 arm 7
build openwrt-mips-softfloat mips "" softfloat
build openwrt-mipsle-softfloat mipsle "" softfloat
build keenetic-mipsle-softfloat mipsle "" softfloat
build keenetic-armv7 arm 7
build keenetic-arm64 arm64
