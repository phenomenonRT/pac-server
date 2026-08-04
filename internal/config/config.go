package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"regexp"
	"strconv"
	"strings"
)

type Config struct {
	ListenIP   string `json:"listen_ip"`
	ListenPort int    `json:"listen_port"`

	ListenAddr string `json:"listen_addr,omitempty"`

	Profiles []PACProfile `json:"profiles"`

	Proxy         string   `json:"proxy,omitempty"`
	Fallback      string   `json:"fallback,omitempty"`
	DirectDomains []string `json:"direct_domains,omitempty"`
	ProxyDomains  []string `json:"proxy_domains,omitempty"`
}

type PACProfile struct {
	Name          string   `json:"name"`
	Slug          string   `json:"slug"`
	ProxyType     string   `json:"proxy_type"`
	ProxyHost     string   `json:"proxy_host"`
	ProxyPort     int      `json:"proxy_port"`
	Proxy         string   `json:"proxy"`
	Fallback      string   `json:"fallback"`
	DirectDomains []string `json:"direct_domains"`
	ProxyDomains  []string `json:"proxy_domains"`
}

func Load(path string) (Config, error) {
	cfg := Default()

	if data, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(data, &cfg); err != nil {
			return Config{}, fmt.Errorf("parse %s: %w", path, err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return Config{}, fmt.Errorf("read %s: %w", path, err)
	}

	applyEnv(&cfg)
	normalize(&cfg)

	if err := validate(cfg); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func Save(path string, cfg Config) error {
	normalize(&cfg)
	if err := validate(cfg); err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg.forSave(), "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	return os.WriteFile(path, data, 0o644)
}

func Default() Config {
	return Config{
		ListenIP:   "127.0.0.1",
		ListenPort: 81,
		Profiles: []PACProfile{
			{
				Name:          "Default",
				Slug:          "default",
				ProxyType:     "SOCKS5",
				ProxyHost:     "127.0.0.1",
				ProxyPort:     1080,
				Proxy:         "SOCKS5 127.0.0.1:1080",
				Fallback:      "DIRECT",
				DirectDomains: []string{"localhost", "local", "lan"},
				ProxyDomains:  []string{},
			},
		},
	}
}

func (cfg Config) Address() string {
	ip := strings.TrimSpace(cfg.ListenIP)
	if ip == "" {
		ip = "127.0.0.1"
	}
	return net.JoinHostPort(ip, strconv.Itoa(cfg.ListenPort))
}

func (cfg Config) DefaultProfile() PACProfile {
	if len(cfg.Profiles) == 0 {
		return Default().Profiles[0]
	}
	return cfg.Profiles[0]
}

func (cfg Config) FindProfile(slug string) (PACProfile, bool) {
	for _, profile := range cfg.Profiles {
		if profile.Slug == slug {
			return profile, true
		}
	}
	return PACProfile{}, false
}

func NewProfile(name, slug, proxyType, proxyHost, proxyPort, fallback, directDomains, proxyDomains string) PACProfile {
	if slug == "" {
		slug = name
	}

	port, _ := strconv.Atoi(strings.TrimSpace(proxyPort))

	return PACProfile{
		Name:          strings.TrimSpace(name),
		Slug:          Slugify(slug),
		ProxyType:     NormalizeProxyType(proxyType),
		ProxyHost:     strings.TrimSpace(proxyHost),
		ProxyPort:     port,
		Fallback:      strings.TrimSpace(fallback),
		DirectDomains: SplitList(directDomains),
		ProxyDomains:  SplitList(proxyDomains),
	}
}

func NewSettings(listenIP, listenPort string) (string, int) {
	port, _ := strconv.Atoi(strings.TrimSpace(listenPort))
	return strings.TrimSpace(listenIP), port
}

func NormalizeProxyType(value string) string {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "DIRECT":
		return "DIRECT"
	case "SOCKS":
		return "SOCKS"
	case "SOCKS5":
		return "SOCKS5"
	case "HTTPS":
		return "HTTPS"
	case "PROXY", "HTTP":
		return "PROXY"
	default:
		return "SOCKS5"
	}
}

func (profile PACProfile) ProxyAddress() string {
	if profile.ProxyType == "DIRECT" {
		return "DIRECT"
	}
	return strings.TrimSpace(net.JoinHostPort(profile.ProxyHost, strconv.Itoa(profile.ProxyPort)))
}

func (profile PACProfile) ProxyString() string {
	if profile.ProxyType == "" && strings.TrimSpace(profile.Proxy) != "" {
		return strings.TrimSpace(profile.Proxy)
	}
	if profile.ProxyType == "DIRECT" {
		return "DIRECT"
	}
	return profile.ProxyType + " " + profile.ProxyAddress()
}

func SplitList(value string) []string {
	value = strings.ReplaceAll(value, "\r\n", ",")
	value = strings.ReplaceAll(value, "\n", ",")
	return cleanList(strings.Split(value, ","))
}

func JoinList(values []string) string {
	return strings.Join(cleanList(values), "\n")
}

func Slugify(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.TrimSuffix(value, ".pac")

	re := regexp.MustCompile(`[^a-z0-9_-]+`)
	value = re.ReplaceAllString(value, "-")
	value = strings.Trim(value, "-_")
	if value == "" {
		return "default"
	}
	return value
}

func applyEnv(cfg *Config) {
	if value := os.Getenv("PAC_ADDR"); value != "" {
		cfg.ListenAddr = value
	}
	if value := os.Getenv("PAC_PROXY"); value != "" {
		ensureLegacyProfile(cfg)
		cfg.Profiles[0].Proxy = value
	}
	if value := os.Getenv("PAC_DIRECT_DOMAINS"); value != "" {
		ensureLegacyProfile(cfg)
		cfg.Profiles[0].DirectDomains = SplitList(value)
	}
	if value := os.Getenv("PAC_PROXY_DOMAINS"); value != "" {
		ensureLegacyProfile(cfg)
		cfg.Profiles[0].ProxyDomains = SplitList(value)
	}
}

func normalize(cfg *Config) {
	cfg.ListenIP = strings.TrimSpace(cfg.ListenIP)
	if cfg.ListenAddr != "" {
		cfg.ListenIP, cfg.ListenPort = parseListenAddr(cfg.ListenAddr)
	}
	if cfg.ListenIP == "" {
		cfg.ListenIP = "127.0.0.1"
	}
	if cfg.ListenPort == 0 {
		cfg.ListenPort = 81
	}

	if len(cfg.Profiles) == 0 && cfg.Proxy != "" {
		cfg.Profiles = []PACProfile{{
			Name:          "Default",
			Slug:          "default",
			Proxy:         cfg.Proxy,
			Fallback:      cfg.Fallback,
			DirectDomains: cfg.DirectDomains,
			ProxyDomains:  cfg.ProxyDomains,
		}}
	}
	if len(cfg.Profiles) == 0 {
		cfg.Profiles = Default().Profiles
	}

	seen := make(map[string]int, len(cfg.Profiles))
	for i := range cfg.Profiles {
		profile := &cfg.Profiles[i]
		profile.Name = strings.TrimSpace(profile.Name)
		profile.Slug = Slugify(profile.Slug)
		profile.Proxy = strings.TrimSpace(profile.Proxy)
		if profile.Proxy != "" && (profile.ProxyHost == "" || profile.ProxyPort == 0) {
			profile.ProxyType, profile.ProxyHost, profile.ProxyPort = parseProxy(profile.Proxy)
		}
		profile.ProxyType = NormalizeProxyType(profile.ProxyType)
		profile.ProxyHost = strings.TrimSpace(profile.ProxyHost)
		profile.Fallback = strings.TrimSpace(profile.Fallback)
		profile.DirectDomains = cleanList(profile.DirectDomains)
		profile.ProxyDomains = cleanList(profile.ProxyDomains)

		if profile.Name == "" {
			profile.Name = profile.Slug
		}
		if profile.Proxy == "" {
			if profile.ProxyHost == "" {
				profile.ProxyHost = "127.0.0.1"
			}
			if profile.ProxyPort == 0 && profile.ProxyType != "DIRECT" {
				profile.ProxyPort = 1080
			}
			profile.Proxy = profile.ProxyString()
		}
		profile.Proxy = profile.ProxyString()
		if profile.Fallback == "" {
			profile.Fallback = "DIRECT"
		}

		base := profile.Slug
		if count := seen[base]; count > 0 {
			profile.Slug = fmt.Sprintf("%s-%d", base, count+1)
		}
		seen[base]++
	}

	cfg.ListenAddr = ""
	cfg.Proxy = ""
	cfg.Fallback = ""
	cfg.DirectDomains = nil
	cfg.ProxyDomains = nil
}

func validate(cfg Config) error {
	if len(cfg.Profiles) == 0 {
		return errors.New("at least one PAC profile is required")
	}

	seen := make(map[string]struct{}, len(cfg.Profiles))
	for _, profile := range cfg.Profiles {
		if profile.Name == "" {
			return errors.New("profile name must not be empty")
		}
		if profile.Slug == "" {
			return errors.New("profile slug must not be empty")
		}
		if profile.Proxy == "" {
			return fmt.Errorf("profile %q proxy must not be empty", profile.Name)
		}
		if profile.ProxyType != "DIRECT" && profile.ProxyHost == "" {
			return fmt.Errorf("profile %q proxy host must not be empty", profile.Name)
		}
		if profile.ProxyType != "DIRECT" && profile.ProxyPort <= 0 {
			return fmt.Errorf("profile %q proxy port must be greater than zero", profile.Name)
		}
		if _, ok := seen[profile.Slug]; ok {
			return fmt.Errorf("profile slug %q is duplicated", profile.Slug)
		}
		seen[profile.Slug] = struct{}{}
	}

	return nil
}

func (cfg Config) forSave() Config {
	cfg.ListenAddr = ""
	cfg.Proxy = ""
	cfg.Fallback = ""
	cfg.DirectDomains = nil
	cfg.ProxyDomains = nil
	return cfg
}

func parseListenAddr(value string) (string, int) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "127.0.0.1", 81
	}

	host, portText, err := net.SplitHostPort(value)
	if err != nil {
		if strings.HasPrefix(value, ":") {
			port, _ := strconv.Atoi(strings.TrimPrefix(value, ":"))
			return "127.0.0.1", port
		}
		return value, 81
	}
	port, _ := strconv.Atoi(portText)
	if host == "" {
		host = "127.0.0.1"
	}
	return host, port
}

func parseProxy(value string) (string, string, int) {
	fields := strings.Fields(value)
	if len(fields) == 0 {
		return "SOCKS5", "127.0.0.1", 1080
	}

	proxyType := NormalizeProxyType(fields[0])
	if proxyType == "DIRECT" {
		return "DIRECT", "", 0
	}

	address := ""
	if len(fields) > 1 {
		address = fields[1]
	}
	host, portText, err := net.SplitHostPort(address)
	if err != nil {
		return proxyType, address, 1080
	}
	port, _ := strconv.Atoi(portText)
	return proxyType, host, port
}

func ensureLegacyProfile(cfg *Config) {
	if len(cfg.Profiles) == 0 {
		cfg.Profiles = Default().Profiles
	}
}

func cleanList(values []string) []string {
	clean := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))

	for _, value := range values {
		item := strings.ToLower(strings.TrimSpace(value))
		item = strings.TrimPrefix(item, ".")
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		clean = append(clean, item)
	}

	return clean
}
