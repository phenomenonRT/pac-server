package pac

import (
	"encoding/json"
	"fmt"
	"strings"

	"pac-server/internal/config"
)

func Generate(profile config.PACProfile) (string, error) {
	directDomains, err := json.Marshal(profile.DirectDomains)
	if err != nil {
		return "", err
	}

	proxyDomains, err := json.Marshal(profile.ProxyDomains)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf(`function FindProxyForURL(url, host) {
  var directDomains = %s;
  var proxyDomains = %s;
  var proxy = %q;
  var fallback = %q;

  host = (host || "").toLowerCase();

  if (isPlainHostName(host) || isInNet(host, "127.0.0.0", "255.0.0.0")) {
    return "DIRECT";
  }

  if (matchesDomain(host, directDomains)) {
    return "DIRECT";
  }

  if (matchesDomain(host, proxyDomains)) {
    return proxy;
  }

  return fallback === "PROXY" ? proxy : fallback;
}

function matchesDomain(host, domains) {
  for (var i = 0; i < domains.length; i++) {
    var domain = domains[i];
    if (host === domain || dnsDomainIs(host, "." + domain)) {
      return true;
    }
  }
  return false;
}
`, directDomains, proxyDomains, strings.TrimSpace(profile.ProxyString()), strings.TrimSpace(profile.Fallback)), nil
}
