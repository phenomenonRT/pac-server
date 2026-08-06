# PAC Server

Go server for creating, editing, and serving multiple PAC files.

## Run Locally

```powershell
go run ./cmd/pac-server
```

Default address:

```text
127.0.0.1:81
```

Useful URLs:

- `http://127.0.0.1:81/` - web UI
- `http://127.0.0.1:81/lists` - v2fly domain list loader
- `http://127.0.0.1:81/settings` - listen IP and port settings
- `http://127.0.0.1:81/proxy.pac` - first PAC profile
- `http://127.0.0.1:81/pac/default.pac` - `default` PAC profile

## Install on Linux

Use one command after the first GitHub Release exists:

```sh
curl -fsSL https://raw.githubusercontent.com/phenomenonRT/pac-server/main/install.sh | sudo sh
```

Install a specific version:

```sh
curl -fsSL https://raw.githubusercontent.com/phenomenonRT/pac-server/main/install.sh | sudo VERSION=v1.0.0 sh
```

Uninstall:

```sh
curl -fsSL https://raw.githubusercontent.com/phenomenonRT/pac-server/main/uninstall.sh | sudo sh
```

The installer:

- detects Linux architecture;
- downloads a release archive;
- installs `pac-server` to `/usr/local/bin/pac-server`;
- creates `/etc/pac-server/config.json` if it does not exist;
- creates and starts `pac-server.service` on systemd systems.

Service commands:

```sh
sudo systemctl status pac-server
sudo systemctl restart pac-server
sudo journalctl -u pac-server -f
```

## Configuration

Default config path:

```text
/etc/pac-server/config.json
```

Local development config path:

```text
config.json
```

Override config path:

```sh
PAC_CONFIG=/path/to/config.json pac-server
```

Example:

```json
{
  "listen_ips": ["127.0.0.1"],
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
```

Keep `listen_ips` limited to `["127.0.0.1"]` for local-only access. On a router, add the LAN IP too, for example `["127.0.0.1", "192.168.1.1"]`, to reach the UI from both the device and the LAN. Do not add `0.0.0.0` unless you intentionally want to listen on every interface. The server opens one listener per address in `listen_ips`, all serving the same profiles. Old configs with a single `listen_ip` string still load and are migrated automatically. Pick interfaces on the `/settings` page (checkboxes, multiple allowed); changes take effect after a restart.

## GitHub Releases

The workflow at `.github/workflows/release.yml` builds release archives when you push a tag:

```sh
git tag v1.0.0
git push origin v1.0.0
```

Publish checklist:

1. Create a GitHub repository.
2. Push this project to `main` or `master`.
3. Push a tag like `v1.0.0`.
4. Open GitHub Actions and wait for the `Release` workflow.
5. Check the generated GitHub Release assets.

You can also run the `Release` workflow manually from GitHub Actions and enter a tag like `v1.0.0`.

It builds:

- `linux-amd64`
- `linux-386`
- `linux-arm64`
- `linux-armv5`
- `linux-armv6`
- `linux-armv7`
- `linux-mips-softfloat`
- `linux-mipsle-softfloat`

Release files are published automatically to GitHub Releases with SHA256 files.
