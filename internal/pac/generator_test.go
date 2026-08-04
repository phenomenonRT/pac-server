package pac

import (
	"strings"
	"testing"

	"pac-server/internal/config"
)

func TestGenerateIncludesProxyAndDomains(t *testing.T) {
	got, err := Generate(config.Config{
		Profiles: []config.PACProfile{{
			Proxy:         "SOCKS5 127.0.0.1:1080",
			Fallback:      "DIRECT",
			DirectDomains: []string{"localhost"},
			ProxyDomains:  []string{"example.com"},
		}},
	}.DefaultProfile())
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	for _, want := range []string{
		`"SOCKS5 127.0.0.1:1080"`,
		`"localhost"`,
		`"example.com"`,
		"FindProxyForURL",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("Generate() missing %q in:\n%s", want, got)
		}
	}
}
