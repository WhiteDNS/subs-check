package proxies

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	u "net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/beck-8/subs-check/config"
	"github.com/beck-8/subs-check/utils"
	"github.com/metacubex/mihomo/common/convert"
	"github.com/metacubex/mihomo/component/resolver"
	"github.com/samber/lo"
	"gopkg.in/yaml.v3"
)

type subEntry struct {
	url    string
	source string
}

const (
	defaultProxyBatchSize   = 5000
	failedSubIgnoreFile     = "failed-sub-urls.yaml"
	failedSubIgnoreDuration = 72 * time.Hour
)

var fetchSubData = GetDateFromSubs

func GetProxies() ([]map[string]any, error) {
	var proxies []map[string]any
	err := ForEachProxyBatch(0, func(batch []map[string]any) error {
		proxies = append(proxies, batch...)
		return nil
	})
	return proxies, err
}

func ForEachProxyBatch(batchSize int, handle func([]map[string]any) error) error {
	if batchSize <= 0 {
		batchSize = defaultProxyBatchSize
	}

	// Resolve local and remote subscription lists.
	subUrls, localNum, remoteNum := resolveSubUrls()
	slog.Info("Subscription URL count", "local", localNum, "remote", remoteNum, "total", len(subUrls))

	if len(config.GlobalConfig.NodeType) > 0 {
		slog.Info("Filtering only user-configured protocols", "type", config.GlobalConfig.NodeType)
	}

	var wg sync.WaitGroup
	// Subscription-fetch concurrency is decoupled from the alive-check
	// concurrency: most users want dozens of cheap HTTP fetches in parallel
	// even when the alive-check pool is tuned large. Falls back to 20 if
	// the user hasn't set it (zero value).
	subFetchConcurrency := config.GlobalConfig.SubUrlsConcurrent
	if subFetchConcurrency <= 0 {
		subFetchConcurrency = 20
	}
	concurrentLimit := make(chan struct{}, subFetchConcurrency) // Limit concurrency.
	ignores := loadFailedSubIgnores(time.Now())
	defer ignores.save()
	var ignoreMu sync.Mutex

	var (
		batch    = make([]map[string]any, 0, batchSize)
		batchMu  sync.Mutex
		handleMu sync.Mutex
		stopOnce sync.Once
		errMu    sync.Mutex
		retErr   error
		done     = make(chan struct{})
	)
	stop := func(err error) {
		if err != nil {
			errMu.Lock()
			if retErr == nil {
				retErr = err
			}
			errMu.Unlock()
		}
		stopOnce.Do(func() { close(done) })
	}
	currentErr := func() error {
		errMu.Lock()
		defer errMu.Unlock()
		return retErr
	}
	flush := func(items []map[string]any) error {
		if len(items) == 0 {
			return nil
		}
		select {
		case <-done:
			return currentErr()
		default:
		}
		handleMu.Lock()
		defer handleMu.Unlock()
		select {
		case <-done:
			return currentErr()
		default:
		}
		err := handle(items)
		if err != nil {
			stop(err)
		}
		return err
	}
	addProxy := func(proxy map[string]any) error {
		select {
		case <-done:
			return currentErr()
		default:
		}

		batchMu.Lock()
		batch = append(batch, proxy)
		if len(batch) < batchSize {
			batchMu.Unlock()
			return nil
		}
		items := batch
		batch = make([]map[string]any, 0, batchSize)
		batchMu.Unlock()
		return flush(items)
	}

	// Start workers.
fetchLoop:
	for idx, subUrl := range subUrls {
		select {
		case <-done:
			break fetchLoop
		default:
		}
		if ignores.ignored(subUrl.url, time.Now()) {
			slog.Warn("Subscription URL is temporarily ignored after recent failure", "source", subUrl.source, "url", subUrl.url)
			continue
		}

		wg.Add(1)
		select {
		case concurrentLimit <- struct{}{}: // Acquire token.
		case <-done:
			wg.Done()
			break fetchLoop
		}

		go func(e subEntry) {
			defer wg.Done()
			defer func() { <-concurrentLimit }() // Release token.

			url := utils.WarpUrl(e.url)
			data, err := fetchSubData(url)
			if err != nil {
				slog.Error("Failed to fetch subscription URL; ignoring for 3 days", "source", e.source, "url", e.url, "err", err)
				ignoreMu.Lock()
				ignores.ignore(e.url, time.Now().Add(failedSubIgnoreDuration))
				ignoreMu.Unlock()
				return
			}

			count, err := forEachProxyFromSubData(url, data, addProxy)
			if err != nil {
				if currentErr() != nil {
					return
				}
				slog.Error("Failed to parse proxy", "source", e.source, "url", url, "err", err)
				return
			}
			slog.Debug("Fetched subscription URL", "source", e.source, "url", url, "count", count)
		}(subUrls[idx])
	}

	// Wait for all workers to finish.
	wg.Wait()

	if err := currentErr(); err != nil {
		return err
	}

	batchMu.Lock()
	items := batch
	batch = nil
	batchMu.Unlock()
	if err := flush(items); err != nil {
		return err
	}

	return currentErr()
}

func forEachProxyFromSubData(url string, data []byte, yield func(map[string]any) error) (int, error) {
	var tag string
	if d, err := u.Parse(url); err == nil {
		tag = d.Fragment
	}

	var con map[string]any
	if err := yaml.Unmarshal(data, &con); err != nil {
		proxyList, err := convert.ConvertsV2Ray(data)
		if err != nil {
			return 0, err
		}
		count := 0
		for _, proxy := range proxyList {
			if !prepareProxy(proxy, url, tag) {
				continue
			}
			count++
			if err := yield(proxy); err != nil {
				return count, err
			}
		}
		return count, nil
	}

	proxyInterface, ok := con["proxies"]
	if !ok || proxyInterface == nil {
		return 0, errors.New("subscription has no proxies")
	}
	proxyList, ok := proxyInterface.([]any)
	if !ok {
		return 0, errors.New("subscription proxies field is invalid")
	}

	count := 0
	for _, proxy := range proxyList {
		proxyMap, ok := proxy.(map[string]any)
		if !ok {
			continue
		}
		if !prepareProxy(proxyMap, url, tag) {
			continue
		}
		count++
		if err := yield(proxyMap); err != nil {
			return count, err
		}
	}
	return count, nil
}

func prepareProxy(proxy map[string]any, url, tag string) bool {
	if t, ok := proxy["type"].(string); ok {
		if len(config.GlobalConfig.NodeType) > 0 && !lo.Contains(config.GlobalConfig.NodeType, t) {
			return false
		}
		switch t {
		case "hysteria2", "hy2":
			if _, ok := proxy["obfs_password"]; ok {
				proxy["obfs-password"] = proxy["obfs_password"]
				delete(proxy, "obfs_password")
			}
		}
	}

	proxy["sub_url"] = url
	if tag != "" {
		proxy["sub_tag"] = tag
	}
	return true
}

type failedSubIgnores struct {
	path    string
	entries map[string]time.Time
	dirty   bool
}

func loadFailedSubIgnores(now time.Time) *failedSubIgnores {
	ignores := &failedSubIgnores{
		path:    failedSubIgnorePath(),
		entries: make(map[string]time.Time),
	}
	data, err := os.ReadFile(ignores.path)
	if err != nil {
		return ignores
	}

	var raw map[string]string
	if err := yaml.Unmarshal(data, &raw); err != nil {
		slog.Warn("Failed to parse failed subscription ignore list; starting fresh", "path", ignores.path, "err", err)
		ignores.dirty = true
		return ignores
	}
	for url, value := range raw {
		expires, err := time.Parse(time.RFC3339, value)
		if err != nil || !expires.After(now) {
			ignores.dirty = true
			continue
		}
		ignores.entries[url] = expires
	}
	return ignores
}

func (i *failedSubIgnores) ignored(url string, now time.Time) bool {
	expires, ok := i.entries[url]
	if !ok {
		return false
	}
	if expires.After(now) {
		return true
	}
	delete(i.entries, url)
	i.dirty = true
	return false
}

func (i *failedSubIgnores) ignore(url string, expires time.Time) {
	i.entries[url] = expires
	i.dirty = true
}

func (i *failedSubIgnores) save() {
	if !i.dirty {
		return
	}
	if err := os.MkdirAll(filepath.Dir(i.path), 0755); err != nil {
		slog.Warn("Failed to create failed subscription ignore directory", "path", i.path, "err", err)
		return
	}
	if len(i.entries) == 0 {
		if err := os.Remove(i.path); err != nil && !os.IsNotExist(err) {
			slog.Warn("Failed to remove empty failed subscription ignore list", "path", i.path, "err", err)
		}
		return
	}

	raw := make(map[string]string, len(i.entries))
	for url, expires := range i.entries {
		raw[url] = expires.Format(time.RFC3339)
	}
	data, err := yaml.Marshal(raw)
	if err != nil {
		slog.Warn("Failed to serialize failed subscription ignore list", "path", i.path, "err", err)
		return
	}
	if err := os.WriteFile(i.path, data, 0644); err != nil {
		slog.Warn("Failed to save failed subscription ignore list", "path", i.path, "err", err)
	}
}

func failedSubIgnorePath() string {
	if config.GlobalConfig.OutputDir != "" {
		return filepath.Join(config.GlobalConfig.OutputDir, failedSubIgnoreFile)
	}
	return filepath.Join(utils.GetExecutablePath(), "output", failedSubIgnoreFile)
}

// from 3k
// resolveSubUrls merges local and remote subscription lists and deduplicates them.
func resolveSubUrls() ([]subEntry, int, int) {
	// Counts.
	var localNum, remoteNum int
	localNum = len(config.GlobalConfig.SubUrls)

	entries := make([]subEntry, 0, len(config.GlobalConfig.SubUrls))
	// Local config.
	for _, u := range config.GlobalConfig.SubUrls {
		entries = append(entries, subEntry{url: u, source: "local config"})
	}

	// Remote lists.
	if len(config.GlobalConfig.SubUrlsRemote) != 0 {
		for _, d := range config.GlobalConfig.SubUrlsRemote {
			if remote, err := fetchRemoteSubUrls(utils.WarpUrl(d)); err != nil {
				slog.Warn("Failed to fetch remote subscription list; ignored", "url", d, "err", err)
			} else {
				remoteNum += len(remote)
				for _, u := range remote {
					entries = append(entries, subEntry{url: u, source: d})
				}
			}
		}
	}

	// Normalize and deduplicate.
	seen := make(map[string]struct{}, len(entries))
	out := make([]subEntry, 0, len(entries))
	for _, e := range entries {
		s := strings.TrimSpace(e.url)
		if s == "" || strings.HasPrefix(s, "#") { // Skip blank lines and comments.
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, subEntry{url: s, source: e.source})
	}
	return out, localNum, remoteNum
}

// fetchRemoteSubUrls reads a subscription URL list from a remote URL.
// Supports two formats:
// 1) plain text separated by newlines, with # comments and blank lines
// 2) YAML/JSON string array
func fetchRemoteSubUrls(listURL string) ([]string, error) {
	if listURL == "" {
		return nil, errors.New("empty list url")
	}
	data, err := GetDateFromSubs(listURL)
	if err != nil {
		return nil, err
	}

	// Prefer parsing as a string array, compatible with YAML/JSON.
	var arr []string
	if err := yaml.Unmarshal(data, &arr); err == nil && len(arr) > 0 {
		return arr, nil
	}

	// Fall back to line-based parsing.
	res := make([]string, 0, 16)
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		res = append(res, line)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return res, nil
}

// GetDateFromSubs fetches data from a subscription URL.
func GetDateFromSubs(subUrl string) ([]byte, error) {
	maxRetries := config.GlobalConfig.SubUrlsReTry
	// Retry interval.
	retryInterval := config.GlobalConfig.SubUrlsRetryInterval
	if retryInterval == 0 {
		retryInterval = 1
	}
	// Timeout.
	timeout := config.GlobalConfig.SubUrlsTimeout
	if timeout == 0 {
		timeout = 10
	}
	var lastErr error

	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
	// Route DNS through the configured mihomo resolver so subscription domains aren't leaked to system DNS.
	// Only when user enabled custom DNS — keeps default behavior unchanged for existing users.
	if config.GlobalConfig.DNS.Enable {
		transport.DialContext = newMihomoDialer(time.Duration(timeout) * time.Second)
	}
	client := &http.Client{
		Timeout:   time.Duration(timeout) * time.Second,
		Transport: transport,
	}

	for i := range maxRetries {
		if i > 0 {
			time.Sleep(time.Duration(retryInterval) * time.Second)
		}

		req, err := http.NewRequest("GET", subUrl, nil)
		if err != nil {
			lastErr = err
			continue
		}

		if config.GlobalConfig.SubUrlsGetUA == "random" {
			req.Header.Set("User-Agent", convert.RandUserAgent())
		} else {
			req.Header.Set("User-Agent", config.GlobalConfig.SubUrlsGetUA)
		}

		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		if resp.StatusCode != 200 {
			resp.Body.Close()
			lastErr = fmt.Errorf("status code: %d", resp.StatusCode)
			continue
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = fmt.Errorf("failed to read response data: %w", err)
			continue
		}
		return body, nil
	}

	return nil, fmt.Errorf("failed after %d retries: %w", maxRetries, lastErr)
}

// newMihomoDialer returns a DialContext that resolves via mihomo's global resolver
// (DoH when configured) then dials the resulting IP directly, avoiding the OS DNS path.
// dialTimeout is shared with the request-level timeout from SubUrlsTimeout.
func newMihomoDialer(dialTimeout time.Duration) func(ctx context.Context, network, addr string) (net.Conn, error) {
	d := &net.Dialer{Timeout: dialTimeout}
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, err
		}
		// If addr is already an IP, skip resolution.
		if ip := net.ParseIP(host); ip != nil {
			return d.DialContext(ctx, network, addr)
		}
		ips, err := resolver.LookupIP(ctx, host)
		if err != nil {
			return nil, fmt.Errorf("resolve %s: %w", host, err)
		}
		if len(ips) == 0 {
			return nil, fmt.Errorf("no ip for %s", host)
		}
		// Try each IP in turn, returning on the first successful connection.
		var dialErr error
		for _, ip := range ips {
			conn, err := d.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
			if err == nil {
				return conn, nil
			}
			dialErr = err
		}
		return nil, fmt.Errorf("dial %s: %w", host, dialErr)
	}
}
