$ErrorActionPreference = "Stop"

$root = Split-Path -Parent $PSScriptRoot
$dist = Join-Path $root "dist"
New-Item -ItemType Directory -Force -Path $dist | Out-Null

$targets = @(
  @{ Name = "openwrt-amd64"; GOARCH = "amd64" },
  @{ Name = "openwrt-386"; GOARCH = "386" },
  @{ Name = "openwrt-arm64"; GOARCH = "arm64" },
  @{ Name = "openwrt-armv5"; GOARCH = "arm"; GOARM = "5" },
  @{ Name = "openwrt-armv6"; GOARCH = "arm"; GOARM = "6" },
  @{ Name = "openwrt-armv7"; GOARCH = "arm"; GOARM = "7" },
  @{ Name = "openwrt-mips-softfloat"; GOARCH = "mips"; GOMIPS = "softfloat" },
  @{ Name = "openwrt-mipsle-softfloat"; GOARCH = "mipsle"; GOMIPS = "softfloat" },
  @{ Name = "keenetic-mipsle-softfloat"; GOARCH = "mipsle"; GOMIPS = "softfloat" },
  @{ Name = "keenetic-armv7"; GOARCH = "arm"; GOARM = "7" },
  @{ Name = "keenetic-arm64"; GOARCH = "arm64" }
)

foreach ($target in $targets) {
  $env:CGO_ENABLED = "0"
  $env:GOOS = "linux"
  $env:GOARCH = $target.GOARCH

  if ($target.ContainsKey("GOARM")) {
    $env:GOARM = $target.GOARM
  } else {
    Remove-Item Env:\GOARM -ErrorAction SilentlyContinue
  }

  if ($target.ContainsKey("GOMIPS")) {
    $env:GOMIPS = $target.GOMIPS
  } else {
    Remove-Item Env:\GOMIPS -ErrorAction SilentlyContinue
  }

  $out = Join-Path $dist ("pac-server-" + $target.Name)
  Write-Host "Building $($target.Name) -> $out"
  go build -buildvcs=false -trimpath -ldflags "-s -w" -o $out ./cmd/pac-server
}

Remove-Item Env:\GOOS -ErrorAction SilentlyContinue
Remove-Item Env:\GOARCH -ErrorAction SilentlyContinue
Remove-Item Env:\GOARM -ErrorAction SilentlyContinue
Remove-Item Env:\GOMIPS -ErrorAction SilentlyContinue
Remove-Item Env:\GOMIPS64 -ErrorAction SilentlyContinue
Remove-Item Env:\CGO_ENABLED -ErrorAction SilentlyContinue
