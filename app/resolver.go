package app

import (
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"strings"

	"github.com/beck-8/subs-check/config"
	"github.com/metacubex/mihomo/component/resolver"
	"github.com/metacubex/mihomo/dns"
)

// defaultBootstrapNameservers is the fallback when default-nameserver is empty; entries must be plain IPs.
var defaultBootstrapNameservers = []string{
	"223.5.5.5",
	"119.29.29.29",
}

// initResolver wires mihomo's global resolver based on user config.
// Call after loadConfig() and before any proxy.DialContext.
//
// Fallback chain when Enable=true:
//
//	default-nameserver → defaultBootstrapNameservers
//	nameserver         → default-nameserver
//	proxy-server-nameserver → nameserver
func initResolver() error {
	c := &config.GlobalConfig.DNS

	// IPv6 toggle applies regardless of Enable — a user can flip on v6 without replacing the resolver.
	resolver.DisableIPv6 = !c.IPv6

	if !c.Enable {
		slog.Info("DNS resolver uses mihomo defaults", "ipv6", c.IPv6)
		return nil
	}

	if len(c.DefaultNameserver) == 0 {
		c.DefaultNameserver = defaultBootstrapNameservers
	}
	valid, err := validateBootstrapIPs(c.DefaultNameserver)
	if err != nil {
		return err
	}
	c.DefaultNameserver = valid
	if len(c.Nameserver) == 0 {
		c.Nameserver = c.DefaultNameserver
	}
	if len(c.ProxyServerNameserver) == 0 {
		c.ProxyServerNameserver = c.Nameserver
	}

	main, err := parseNameservers(c.Nameserver, "nameserver")
	if err != nil {
		return err
	}
	proxySrv, err := parseNameservers(c.ProxyServerNameserver, "proxy-server-nameserver")
	if err != nil {
		return err
	}
	def, err := parseNameservers(c.DefaultNameserver, "default-nameserver")
	if err != nil {
		return err
	}

	rs := dns.NewResolver(dns.Config{
		Main:        main,
		Default:     def,
		ProxyServer: proxySrv,
		IPv6:        c.IPv6,
	})

	resolver.DefaultResolver = rs.Resolver
	resolver.ProxyServerHostResolver = rs.ProxyResolver

	slog.Info("DNS resolver initialized",
		"nameserver", len(main),
		"proxy-server", len(proxySrv),
		"default", len(def),
		"ipv6", c.IPv6)
	return nil
}

// parseNameservers converts string URLs into dns.NameServer. Bare IP becomes UDP:53.
// Supports: udp://, tcp://, tls://, https://, http://, quic://.
// Invalid entries are warn-skipped; an error is returned only when all entries are invalid.
// fieldName is used in log warnings to point users at the offending config field.
func parseNameservers(servers []string, fieldName string) ([]dns.NameServer, error) {
	out := make([]dns.NameServer, 0, len(servers))
	for _, s := range servers {
		// Bare IP or host[:port] gets the udp:// prefix.
		raw := s
		if !strings.Contains(s, "://") {
			s = "udp://" + s
		}
		u, err := url.Parse(s)
		if err != nil {
			slog.Warn(fieldName+" skipped invalid item", "value", raw, "reason", err)
			continue
		}
		ns := dns.NameServer{}
		switch u.Scheme {
		case "udp":
			ns.Addr = hostPort(u.Host, "53")
		case "tcp":
			ns.Net = "tcp"
			ns.Addr = hostPort(u.Host, "53")
		case "tls":
			ns.Net = "tls"
			ns.Addr = hostPort(u.Host, "853")
		case "https", "http":
			ns.Net = "https"
			defPort := "443"
			if u.Scheme == "http" {
				defPort = "80"
			}
			cleaned := url.URL{Scheme: u.Scheme, Host: hostPort(u.Host, defPort), Path: u.Path}
			ns.Addr = cleaned.String()
		case "quic":
			ns.Net = "quic"
			ns.Addr = hostPort(u.Host, "853")
		default:
			slog.Warn(fieldName+" skipped unsupported scheme", "value", raw, "scheme", u.Scheme)
			continue
		}
		out = append(out, ns)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("%s has no valid entries; at least one valid entry is required", fieldName)
	}
	return out, nil
}

// validateBootstrapIPs filters default-nameserver entries to those that are literal IPs,
// warning about (and dropping) invalid ones. Returns an error only when nothing remains.
// Accepts: "1.1.1.1", "1.1.1.1:5353", "::1", "[::1]:5353", etc.
// Hostnames are rejected — bootstrap can't depend on DNS to resolve itself.
func validateBootstrapIPs(servers []string) ([]string, error) {
	valid := make([]string, 0, len(servers))
	for _, ns := range servers {
		host := ns
		// SplitHostPort handles both IPv4:port and bracketed IPv6:port.
		if h, _, err := net.SplitHostPort(ns); err == nil {
			host = h
		}
		// Bare bracketed IPv6 like "[::1]" without port.
		host = strings.TrimPrefix(strings.TrimSuffix(host, "]"), "[")
		if net.ParseIP(host) == nil {
			slog.Warn("default-nameserver skipped invalid IP", "value", ns)
			continue
		}
		valid = append(valid, ns)
	}
	if len(valid) == 0 {
		return nil, fmt.Errorf("default-nameserver has no valid entries; at least one valid IP is required")
	}
	return valid, nil
}

func hostPort(host, defPort string) string {
	if host == "" {
		return ":" + defPort
	}
	// IPv6 literal already has its own [::1] wrapping; just check for a trailing :port.
	if idx := strings.LastIndex(host, ":"); idx > strings.LastIndex(host, "]") {
		return host
	}
	return host + ":" + defPort
}
