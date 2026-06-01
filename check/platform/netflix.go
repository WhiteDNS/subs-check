package platform

import (
	"net/http"
	"regexp"
	"strings"
)

var netflixRe = regexp.MustCompile(`/([a-z]{2})/title/`)

// NetflixResult represents a Netflix check result.
type NetflixResult struct {
	Full          bool   // Full unlock.
	OriginalsOnly bool   // Netflix Originals only.
	Region        string // Region code.
}

// CheckNetflix checks Netflix unlock status.
// 1. Full unlock: a non-original title returns 200/301 and the region is extracted -> NF-US.
// 2. Originals only: a non-original title returns 404 and an original title returns 200 -> NF.
// 3. Blocked: all requests return 403 -> no tag.
func CheckNetflix(httpClient *http.Client) (*NetflixResult, error) {
	result := &NetflixResult{}

	// Title 81280792 is a non-original title with regional availability.
	// Title 70143836 is a Netflix Original.
	nonOriginalStatus := checkNetflixTitle(httpClient, "81280792")
	originalStatus := checkNetflixTitle(httpClient, "70143836")

	if nonOriginalStatus == 200 || nonOriginalStatus == 301 {
		// Non-original title is accessible -> full unlock.
		result.Full = true
		result.Region = getNetflixRegion(httpClient)
	} else if nonOriginalStatus == 404 && (originalStatus == 200 || originalStatus == 301) {
		// Non-original title is 404 but original title is accessible -> Originals only.
		result.OriginalsOnly = true
	}

	return result, nil
}

// checkNetflixTitle checks the HTTP status code for a specific Netflix title.
func checkNetflixTitle(httpClient *http.Client, titleID string) int {
	req, err := http.NewRequest("GET", "https://www.netflix.com/title/"+titleID, nil)
	if err != nil {
		return 0
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36")

	resp, err := httpClient.Do(req)
	if err != nil {
		return 0
	}
	defer resp.Body.Close()

	return resp.StatusCode
}

// getNetflixRegion extracts the region code by visiting a specific title.
func getNetflixRegion(httpClient *http.Client) string {
	req, err := http.NewRequest("GET", "https://www.netflix.com/title/80018499", nil)
	if err != nil {
		return ""
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36")

	// Do not follow redirects; extract the region code from the Location header.
	client := *httpClient
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}

	resp, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	location := resp.Header.Get("Location")
	if location == "" {
		return ""
	}

	// Location format example: https://www.netflix.com/xx/title/80018499.
	matches := netflixRe.FindStringSubmatch(location)
	if len(matches) > 1 {
		return strings.ToUpper(matches[1])
	}

	return ""
}
