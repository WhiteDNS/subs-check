package platform

import (
	"bytes"
	"net/http"
	"strings"
)

// CheckSpotify checks Spotify unlock status.
// It prefers extracting the region code from the redirected URL path (for example /us/...)
// and falls back to extracting countryCode from the body.
// Returns an alpha-2 region code such as "US"; an empty string means unavailable.
func CheckSpotify(httpClient *http.Client) (string, error) {
	req, err := http.NewRequest("GET", "https://www.spotify.com/api/content/v1/country-selector?platform=web&format=json", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 403 || resp.StatusCode == 451 {
		return "", nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", nil
	}

	// Method 1: extract the region code from the final redirected URL.
	// Spotify redirects to URLs such as https://www.spotify.com/us/... or /jp/...
	finalURL := resp.Request.URL.Path
	if region := extractRegionFromPath(finalURL); region != "" {
		return region, nil
	}

	// Method 2: extract countryCode from the body.
	buf := getPooledBuf()
	defer putPooledBuf(buf)
	if _, err := buf.ReadFrom(resp.Body); err != nil {
		return "", err
	}
	body := buf.Bytes()

	// Check whether the response is blocked.
	if bytes.Contains(bytes.ToLower(body), []byte("not available in your country")) {
		return "", nil
	}

	// Find "countryCode":"XX".
	marker := []byte(`"countryCode":"`)
	if idx := bytes.Index(body, marker); idx != -1 {
		start := idx + len(marker)
		rest := body[start:]
		if end := bytes.Index(rest, []byte(`"`)); end > 0 {
			code := strings.ToUpper(string(rest[:end]))
			if len(code) == 2 {
				return code, nil
			}
		}
	}

	return "", nil
}

// extractRegionFromPath extracts the region code from the first URL path segment.
// Examples: /us/... -> US, /jp/... -> JP, /en-us/... -> EN.
func extractRegionFromPath(path string) string {
	path = strings.TrimPrefix(path, "/")
	if path == "" {
		return ""
	}

	// Take the first path segment.
	segment := path
	if idx := strings.Index(path, "/"); idx != -1 {
		segment = path[:idx]
	}

	// Skip api prefixes, which indicate there was no redirect.
	if segment == "" || segment == "api" {
		return ""
	}

	// For formats such as en-us, take the first half.
	if idx := strings.Index(segment, "-"); idx != -1 {
		segment = segment[:idx]
	}

	if len(segment) == 2 {
		return strings.ToUpper(segment)
	}

	return ""
}
