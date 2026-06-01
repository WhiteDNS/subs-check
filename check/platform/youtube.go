package platform

import (
	"bytes"
	"net/http"
	"regexp"
)

// Find INNERTUBE_CONTEXT_GL in the body and extract the region code.
var youtubeRe = regexp.MustCompile(`"INNERTUBE_CONTEXT_GL"\s*:\s*"([^"]+)"`)

func CheckYoutube(httpClient *http.Client) (string, error) {
	// Create request.
	req, err := http.NewRequest("GET", "https://www.youtube.com/premium", nil)
	if err != nil {
		return "", err
	}

	// Add headers.
	req.Header.Set("accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7")
	req.Header.Set("accept-language", "zh-CN,zh;q=0.9")
	req.Header.Set("sec-ch-ua", `"Chromium";v="131", "Not_A Brand";v="24", "Google Chrome";v="131"`)
	req.Header.Set("sec-ch-ua-platform", `"Windows"`)
	req.Header.Set("sec-fetch-dest", "document")
	req.Header.Set("sec-fetch-mode", "navigate")
	req.Header.Set("sec-fetch-site", "none")
	req.Header.Set("user-agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36")

	// Send request.
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// Read response body.
	buf := getPooledBuf()
	defer putPooledBuf(buf)
	if _, err := buf.ReadFrom(resp.Body); err != nil {
		return "", err
	}
	body := buf.Bytes()

	// Blocked in China.
	if bytes.Contains(body, []byte("www.google.cn")) {
		return "CN", nil
	}

	if bytes.Contains(body, []byte("Premium is not available in your country")) {
		return "", nil
	}

	// Check for the blocked-in-China marker before detecting the location.
	match := youtubeRe.FindSubmatch(body)
	if len(match) > 1 {
		if region := string(match[1]); region != "" {
			return region, nil
		}
	}

	return "", nil
}
