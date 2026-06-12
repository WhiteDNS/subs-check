package check

import (
	"context"
	"net"
	"net/netip"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/beck-8/subs-check/config"
	"github.com/metacubex/mihomo/component/resolver"
)

var (
	// Official Cloudflare IP ranges, checked 2026-06-12:
	// https://www.cloudflare.com/ips-v4 and https://www.cloudflare.com/ips-v6
	cloudflarePrefixes = mustParseCloudflarePrefixes([]string{
		"173.245.48.0/20",
		"103.21.244.0/22",
		"103.22.200.0/22",
		"103.31.4.0/22",
		"141.101.64.0/18",
		"108.162.192.0/18",
		"190.93.240.0/20",
		"188.114.96.0/20",
		"197.234.240.0/22",
		"198.41.128.0/17",
		"162.158.0.0/15",
		"104.16.0.0/13",
		"104.24.0.0/14",
		"172.64.0.0/13",
		"131.0.72.0/22",
		"2400:cb00::/32",
		"2606:4700::/32",
		"2803:f800::/32",
		"2405:b500::/32",
		"2405:8100::/32",
		"2a06:98c0::/29",
		"2c0f:f248::/32",
	})

	cloudflareHostnameSuffixes = []string{
		"cloudflare.com",
		"cloudflare.net",
		"workers.dev",
		"pages.dev",
		"trycloudflare.com",
	}

	cloudflareHostCache sync.Map
)

func mustParseCloudflarePrefixes(values []string) []netip.Prefix {
	prefixes := make([]netip.Prefix, 0, len(values))
	for _, value := range values {
		prefixes = append(prefixes, netip.MustParsePrefix(value))
	}
	return prefixes
}

func isCloudflareFrontedProxy(proxy map[string]any) bool {
	if proxy == nil {
		return false
	}
	server, _ := proxy["server"].(string)
	return isCloudflareFrontedHost(server)
}

func isCloudflareFrontedHost(raw string) bool {
	host := normalizeEndpointHost(raw)
	if host == "" {
		return false
	}

	if cached, ok := cloudflareHostCache.Load(host); ok {
		return cached.(bool)
	}

	ok := isCloudflareKnownHostname(host) || isCloudflareIPLiteral(host) || resolvesOnlyToCloudflare(host)
	cloudflareHostCache.Store(host, ok)
	return ok
}

func normalizeEndpointHost(raw string) string {
	host := strings.TrimSpace(raw)
	host = strings.Trim(host, `"'`)
	if host == "" {
		return ""
	}

	if parsed, err := url.Parse(host); err == nil && parsed.Host != "" {
		host = parsed.Host
	}
	if split, _, err := net.SplitHostPort(host); err == nil {
		host = split
	}
	host = strings.TrimPrefix(host, "[")
	host = strings.TrimSuffix(host, "]")
	if beforePath, _, ok := strings.Cut(host, "/"); ok {
		host = beforePath
	}
	host = strings.TrimSuffix(host, ".")
	return strings.ToLower(strings.TrimSpace(host))
}

func isCloudflareKnownHostname(host string) bool {
	for _, suffix := range cloudflareHostnameSuffixes {
		if host == suffix || strings.HasSuffix(host, "."+suffix) {
			return true
		}
	}
	return false
}

func isCloudflareIPLiteral(host string) bool {
	addr, err := netip.ParseAddr(host)
	if err != nil {
		return false
	}
	return isCloudflareAddr(addr)
}

func isCloudflareAddr(addr netip.Addr) bool {
	addr = addr.Unmap()
	for _, prefix := range cloudflarePrefixes {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}

func resolvesOnlyToCloudflare(host string) bool {
	if net.ParseIP(host) != nil || !strings.Contains(host, ".") {
		return false
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	addrs, err := lookupEndpointAddrs(ctx, host)
	if err != nil || len(addrs) == 0 {
		return false
	}
	for _, addr := range addrs {
		if !isCloudflareAddr(addr) {
			return false
		}
	}
	return true
}

func lookupEndpointAddrs(ctx context.Context, host string) ([]netip.Addr, error) {
	if config.GlobalConfig.DNS.Enable {
		return resolver.LookupIP(ctx, host)
	}

	ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil || len(ips) == 0 {
		return nil, err
	}
	addrs := make([]netip.Addr, 0, len(ips))
	for _, ip := range ips {
		addr, ok := netip.AddrFromSlice(ip.IP)
		if ok {
			addrs = append(addrs, addr)
		}
	}
	return addrs, nil
}
