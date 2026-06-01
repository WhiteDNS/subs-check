package platform

import (
	"net/http"
	"regexp"
)

var claudeRe = regexp.MustCompile(`loc=([A-Z]{2})`)

// Claude blocked region list (alpha-2 codes).
var claudeBlockedRegions = map[string]bool{
	"AF": true, "BY": true, "CN": true, "CU": true, "HK": true,
	"IR": true, "KP": true, "MO": true, "RU": true, "SY": true,
}

// CheckClaude checks Claude unlock status.
// It extracts the region code through cdn-cgi/trace and filters with the blocked list.
// Returns an alpha-2 region code such as "US"; an empty string means unavailable.
func CheckClaude(httpClient *http.Client) (string, error) {
	req, err := http.NewRequest("GET", "https://claude.ai/cdn-cgi/trace", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	buf := getPooledBuf()
	defer putPooledBuf(buf)
	if _, err := buf.ReadFrom(resp.Body); err != nil {
		return "", err
	}
	body := buf.Bytes()

	matches := claudeRe.FindSubmatch(body)
	if len(matches) <= 1 {
		return "", nil
	}

	region := string(matches[1])
	if claudeBlockedRegions[region] {
		return "", nil
	}

	return region, nil
}
