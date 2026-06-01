package proxies

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	"log/slog"

	"github.com/metacubex/mihomo/common/convert"
)

type geoResult struct {
	loc string
	ip  string
}

// This needs a non-CF IPv4 API without strict rate limits because IPv6 entries
// missing from the database may fall back to US. CF APIs cannot be the only
// source because CF nodes without proxyip must still be kept.
// GetProxyCountry queries all IP lookup endpoints in parallel and returns the
// best result by priority.
func GetProxyCountry(httpClient *http.Client) (loc string, ip string) {
	// Order represents priority; lower index means higher quality.
	checkers := []func(*http.Client) (string, string){
		GetMe, GetIpinfo, GetCFProxy, GetEdgeOneProxy,
	}

	results := make([]geoResult, len(checkers))
	var wg sync.WaitGroup

	for idx, fn := range checkers {
		wg.Add(1)
		go func(i int, f func(*http.Client) (string, string)) {
			defer wg.Done()
			l, p := f(httpClient)
			results[i] = geoResult{l, p}
		}(idx, fn)
	}

	wg.Wait()

	// Return the first successful result by priority.
	for _, res := range results {
		if res.loc != "" && res.ip != "" {
			return res.loc, res.ip
		}
	}
	return
}

// GetEdgeOneProxy gets geolocation through Tencent EdgeOne.
func GetEdgeOneProxy(httpClient *http.Client) (loc string, ip string) {
	type GeoResponse struct {
		Eo struct {
			Geo struct {
				CountryCodeAlpha2 string `json:"countryCodeAlpha2"`
			} `json:"geo"`
			ClientIp string `json:"clientIp"`
		} `json:"eo"`
	}

	url := "https://functions-geolocation.edgeone.app/geo"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		slog.Debug(fmt.Sprintf("Failed to create request: %s", err))
		return
	}
	req.Header.Set("User-Agent", convert.RandUserAgent())
	resp, err := httpClient.Do(req)
	if err != nil {
		slog.Debug(fmt.Sprintf("edgeone failed to get node location: %s", err))
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		slog.Debug(fmt.Sprintf("edgeone returned non-200 status code: %v", resp.StatusCode))
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		slog.Debug(fmt.Sprintf("edgeone failed to read node location: %s", err))
		return
	}

	var eo GeoResponse
	err = json.Unmarshal(body, &eo)
	if err != nil {
		slog.Debug(fmt.Sprintf("Failed to parse edgeone JSON: %v", err))
		return
	}

	return eo.Eo.Geo.CountryCodeAlpha2, eo.Eo.ClientIp
}

// GetCFProxy gets geolocation through Cloudflare cdn-cgi/trace.
// Limitation: CF nodes need proxyip egress to access CF-protected sites, so
// trace returns the proxyip egress location, not the node's real exit location.
func GetCFProxy(httpClient *http.Client) (loc string, ip string) {
	url := "https://www.cloudflare.com/cdn-cgi/trace"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		slog.Debug(fmt.Sprintf("Failed to create request: %s", err))
		return
	}
	req.Header.Set("User-Agent", convert.RandUserAgent())
	resp, err := httpClient.Do(req)
	if err != nil {
		slog.Debug(fmt.Sprintf("cf failed to get node location: %s", err))
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		slog.Debug(fmt.Sprintf("cf returned non-200 status code: %v", resp.StatusCode))
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		slog.Debug(fmt.Sprintf("cf failed to read node location: %s", err))
		return
	}

	// Parse the response text to find loc=XX
	for _, line := range strings.Split(string(body), "\n") {
		if strings.HasPrefix(line, "loc=") {
			loc = strings.TrimPrefix(line, "loc=")
		}
		if strings.HasPrefix(line, "ip=") {
			ip = strings.TrimPrefix(line, "ip=")
		}
	}
	return
}

// GetIPSB gets geolocation through ip.sb.
func GetIPSB(httpClient *http.Client) (loc string, ip string) {
	type GeoIPData struct {
		IP      string `json:"ip"`
		Country string `json:"country_code"`
	}

	url := "https://api.ip.sb/geoip"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		slog.Debug(fmt.Sprintf("Failed to create request: %s", err))
		return
	}
	req.Header.Set("User-Agent", convert.RandUserAgent())
	resp, err := httpClient.Do(req)
	if err != nil {
		slog.Debug(fmt.Sprintf("ip.sb failed to get node location: %s", err))
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		slog.Debug(fmt.Sprintf("ip.sb returned non-200 status code: %v", resp.StatusCode))
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		slog.Debug(fmt.Sprintf("ip.sb failed to read node location: %s", err))
		return
	}

	var geo GeoIPData
	err = json.Unmarshal(body, &geo)
	if err != nil {
		slog.Debug(fmt.Sprintf("Failed to parse ip.sb JSON: %v", err))
		return
	}

	return geo.Country, geo.IP
}

func GetMe(httpClient *http.Client) (loc string, ip string) {
	type GeoIPData struct {
		IP      string `json:"ip"`
		Country string `json:"country_code"`
	}

	url := "https://ip.122911.xyz/api/ipinfo"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		slog.Debug(fmt.Sprintf("Failed to create request: %s", err))
		return
	}
	req.Header.Set("User-Agent", "subs-check (https://github.com/beck-8/subs-check)")
	resp, err := httpClient.Do(req)
	if err != nil {
		slog.Debug(fmt.Sprintf("me failed to get node location: %s", err))
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		slog.Debug(fmt.Sprintf("me returned non-200 status code: %v", resp.StatusCode))
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		slog.Debug(fmt.Sprintf("me failed to read node location: %s", err))
		return
	}

	var geo GeoIPData
	err = json.Unmarshal(body, &geo)
	if err != nil {
		slog.Debug(fmt.Sprintf("Failed to parse me JSON: %v", err))
		return
	}

	return geo.Country, geo.IP
}

func GetIpinfo(httpClient *http.Client) (loc string, ip string) {
	type GeoIPData struct {
		IP      string `json:"ip"`
		Country string `json:"country_code"`
	}

	url := string([]byte{104, 116, 116, 112, 115, 58, 47, 47, 97, 112, 105, 46, 105, 112,
		105, 110, 102, 111, 46, 105, 111, 47, 108, 105, 116, 101, 47, 109, 101, 63, 116,
		111, 107, 101, 110, 61, 48, 57, 48, 102, 54, 54, 55, 55, 57, 55, 51, 51, 98, 102})
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		slog.Debug(fmt.Sprintf("Failed to create request: %s", err))
		return
	}
	req.Header.Set("User-Agent", "subs-check (https://github.com/beck-8/subs-check)")
	resp, err := httpClient.Do(req)
	if err != nil {
		slog.Debug(fmt.Sprintf("Ipinfo failed to get node location: %s", err))
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		slog.Debug(fmt.Sprintf("Ipinfo returned non-200 status code: %v", resp.StatusCode))
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		slog.Debug(fmt.Sprintf("Ipinfo failed to read node location: %s", err))
		return
	}

	var geo GeoIPData
	err = json.Unmarshal(body, &geo)
	if err != nil {
		slog.Debug(fmt.Sprintf("Failed to parse Ipinfo JSON: %v", err))
		return
	}

	return geo.Country, geo.IP
}
