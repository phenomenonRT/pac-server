package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLoadMigratesLegacyListenIP(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	legacy := `{
  "listen_ip": "192.168.28.1",
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
}`
	if err := os.WriteFile(path, []byte(legacy), 0o644); err != nil {
		t.Fatalf("write legacy config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	want := []string{"192.168.28.1"}
	if !reflect.DeepEqual(cfg.ListenIPs, want) {
		t.Fatalf("ListenIPs = %#v, want %#v", cfg.ListenIPs, want)
	}
	if cfg.ListenIP != "" {
		t.Fatalf("ListenIP = %q, want empty after migration", cfg.ListenIP)
	}
}

func TestLoadPrefersNewListenIPsOverLegacyField(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	mixed := `{
  "listen_ip": "192.168.28.1",
  "listen_ips": ["10.0.0.5"],
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
}`
	if err := os.WriteFile(path, []byte(mixed), 0o644); err != nil {
		t.Fatalf("write mixed config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	want := []string{"10.0.0.5"}
	if !reflect.DeepEqual(cfg.ListenIPs, want) {
		t.Fatalf("ListenIPs = %#v, want %#v (new field must win over legacy)", cfg.ListenIPs, want)
	}
}

func TestLoadDefaultsToLocalhostWhenNoFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "does-not-exist.json")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	want := []string{"127.0.0.1"}
	if !reflect.DeepEqual(cfg.ListenIPs, want) {
		t.Fatalf("ListenIPs = %#v, want %#v", cfg.ListenIPs, want)
	}
}
