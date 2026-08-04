package domainlist

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

const baseURL = "https://raw.githubusercontent.com/v2fly/domain-list-community/master/data/"

type Client struct {
	HTTPClient *http.Client
}

func NewClient() Client {
	return Client{
		HTTPClient: &http.Client{Timeout: 15 * time.Second},
	}
}

func (c Client) Fetch(name string) ([]string, error) {
	seen := make(map[string]struct{})
	return c.fetch(name, seen, 0)
}

func RawURL(name string) string {
	return baseURL + sanitizeListName(name)
}

func (c Client) fetch(name string, seenFiles map[string]struct{}, depth int) ([]string, error) {
	if depth > 3 {
		return nil, nil
	}

	name = sanitizeListName(name)
	if name == "" {
		return nil, fmt.Errorf("domain list name is required")
	}
	if _, ok := seenFiles[name]; ok {
		return nil, nil
	}
	seenFiles[name] = struct{}{}

	req, err := http.NewRequest(http.MethodGet, baseURL+name, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "pac-server")

	client := c.HTTPClient
	if client == nil {
		client = NewClient().HTTPClient
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("domain list %q returned %s", name, resp.Status)
	}

	domains, includes, err := parse(resp.Body)
	if err != nil {
		return nil, err
	}

	for _, include := range includes {
		more, err := c.fetch(include, seenFiles, depth+1)
		if err != nil {
			return nil, err
		}
		domains = append(domains, more...)
	}

	return unique(domains), nil
}

func ParseText(value string) []string {
	domains, _, _ := parse(strings.NewReader(value))
	return unique(domains)
}

func parse(r io.Reader) ([]string, []string, error) {
	var domains []string
	var includes []string

	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if idx := strings.Index(line, "#"); idx >= 0 {
			line = strings.TrimSpace(line[:idx])
		}
		if idx := strings.Index(line, "@"); idx >= 0 {
			line = strings.TrimSpace(line[:idx])
		}
		if line == "" {
			continue
		}

		kind, value, ok := strings.Cut(line, ":")
		if !ok {
			domains = append(domains, cleanDomain(line))
			continue
		}

		kind = strings.ToLower(strings.TrimSpace(kind))
		value = strings.TrimSpace(value)
		switch kind {
		case "domain", "full":
			domains = append(domains, cleanDomain(value))
		case "include":
			includes = append(includes, sanitizeListName(value))
		}
	}

	return cleanDomains(domains), includes, scanner.Err()
}

func sanitizeListName(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.TrimPrefix(value, "geosite:")
	re := regexp.MustCompile(`[^a-z0-9.!_-]+`)
	value = re.ReplaceAllString(value, "-")
	return strings.Trim(value, ".-_")
}

func cleanDomain(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.TrimPrefix(value, "domain:")
	value = strings.TrimPrefix(value, "full:")
	value = strings.TrimPrefix(value, ".")
	return value
}

func cleanDomains(values []string) []string {
	clean := make([]string, 0, len(values))
	for _, value := range values {
		value = cleanDomain(value)
		if value == "" || strings.ContainsAny(value, "/*()[]\\") {
			continue
		}
		clean = append(clean, value)
	}
	return clean
}

func unique(values []string) []string {
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = cleanDomain(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}
