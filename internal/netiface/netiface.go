// Package netiface discovers local network interfaces and their IPv4
// addresses so the web UI can offer them as a listen-address picker instead
// of requiring the user to type an IP by hand.
package netiface

import (
	"net"
	"sort"
	"strings"
)

// Option is a single selectable "listen on this address" choice.
type Option struct {
	// Label is shown to the user, e.g. "eth0 (192.168.1.1)".
	Label string
	// IP is the value actually stored/used as listen_ip.
	IP string
}

// List returns the IPv4 addresses of all up network interfaces, plus a
// trailing "all interfaces" (0.0.0.0) option. It never returns an error;
// if interfaces cannot be enumerated it simply returns fewer options.
func List() []Option {
	options := make([]Option, 0, 8)
	seen := make(map[string]struct{})

	ifaces, err := net.Interfaces()
	if err == nil {
		for _, iface := range ifaces {
			if iface.Flags&net.FlagUp == 0 {
				continue
			}

			addrs, err := iface.Addrs()
			if err != nil {
				continue
			}

			for _, addr := range addrs {
				ip := extractIPv4(addr)
				if ip == "" {
					continue
				}
				if _, ok := seen[ip]; ok {
					continue
				}
				seen[ip] = struct{}{}

				label := iface.Name + " (" + ip + ")"
				if ip == "127.0.0.1" {
					label = iface.Name + " (" + ip + ", только это устройство)"
				}
				options = append(options, Option{Label: label, IP: ip})
			}
		}
	}

	sort.Slice(options, func(i, j int) bool {
		if options[i].IP == "127.0.0.1" {
			return true
		}
		if options[j].IP == "127.0.0.1" {
			return false
		}
		return options[i].IP < options[j].IP
	})

	if _, ok := seen["0.0.0.0"]; !ok {
		options = append(options, Option{
			Label: "0.0.0.0 (все интерфейсы, включая WAN)",
			IP:    "0.0.0.0",
		})
	}

	return options
}

// WithCurrent ensures cfgIP is present in the returned options, adding it
// (marked as current) when it does not match any detected interface, e.g.
// after moving a config file between machines.
func WithCurrent(options []Option, cfgIPs []string) []Option {
	have := make(map[string]struct{}, len(options))
	for _, opt := range options {
		have[opt.IP] = struct{}{}
	}

	missing := make([]Option, 0)
	for _, cfgIP := range cfgIPs {
		cfgIP = strings.TrimSpace(cfgIP)
		if cfgIP == "" {
			continue
		}
		if _, ok := have[cfgIP]; ok {
			continue
		}
		have[cfgIP] = struct{}{}
		missing = append(missing, Option{Label: cfgIP + " (текущий, интерфейс не обнаружен)", IP: cfgIP})
	}

	if len(missing) == 0 {
		return options
	}
	return append(missing, options...)
}

func extractIPv4(addr net.Addr) string {
	var ip net.IP
	switch v := addr.(type) {
	case *net.IPNet:
		ip = v.IP
	case *net.IPAddr:
		ip = v.IP
	default:
		return ""
	}

	ip4 := ip.To4()
	if ip4 == nil {
		return ""
	}
	return ip4.String()
}
