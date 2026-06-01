package platform

import (
	"bytes"
	"fmt"
	"net/http"

	"github.com/metacubex/mihomo/common/convert"
)

func CheckIPRisk(httpClient *http.Client, ip string) (string, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("https://scamalytics.com/ip/%s", ip), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", convert.RandUserAgent())
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		// Read the response body.
		buf := getPooledBuf()
		defer putPooledBuf(buf)
		if _, err := buf.ReadFrom(resp.Body); err != nil {
			return "", err
		}
		body := buf.Bytes()
		marker := []byte("IP Fraud Risk API")
		apiIndex := bytes.Index(body, marker)
		if apiIndex == -1 {
			return "", fmt.Errorf("IP Fraud Risk API not found")
		}
		// Start from the content after "IP Fraud Risk API".
		contentAfterAPI := body[apiIndex+len(marker):]
		// Split by line.
		lines := bytes.Split(contentAfterAPI, []byte("\n"))

		if len(lines) < 7 {
			return "", fmt.Errorf("invalid IP Fraud Risk API response format")
		}
		var score, rist []byte
		{
			score = bytes.TrimSpace(lines[4])
			tmp := bytes.Split(score, []byte(":"))
			if len(tmp) < 2 {
				return "", fmt.Errorf("invalid IP Fraud Risk API response format")
			}
			score = bytes.ReplaceAll(tmp[1], []byte(`"`), nil)
			score = bytes.ReplaceAll(score, []byte(","), nil)

			rist = bytes.TrimSpace(lines[5])
			tmp = bytes.Split(rist, []byte(":"))
			if len(tmp) < 2 {
				return "", fmt.Errorf("invalid IP Fraud Risk API response format")
			}
			rist = bytes.ReplaceAll(tmp[1], []byte(`"`), nil)
			rist = bytes.ReplaceAll(rist, []byte(","), nil)
		}

		if len(score) > 0 && len(rist) > 0 {
			// return fmt.Sprintf("%s%% %s", score, rist), nil
			return fmt.Sprintf("%s%%", score), nil
		}
	}
	return "", nil
}
