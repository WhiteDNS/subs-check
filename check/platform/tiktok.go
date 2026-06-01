package platform

import (
	"net/http"
	"regexp"
)

var tiktokRe = regexp.MustCompile(`"region"\s*:\s*"([A-Z]{2})"`)

func CheckTikTok(httpClient *http.Client) (string, error) {
	req, err := http.NewRequest("GET", "https://www.tiktok.com/", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36")
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", nil
	}

	buf := getPooledBuf()
	defer putPooledBuf(buf)
	if _, err := buf.ReadFrom(resp.Body); err != nil {
		return "", err
	}
	body := buf.Bytes()

	// Use a regex to match "region":"XX".
	matches := tiktokRe.FindSubmatch(body)
	if len(matches) >= 2 {
		return string(matches[1]), nil
	}
	return "", nil
}
