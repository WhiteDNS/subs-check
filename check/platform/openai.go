package platform

import (
	"bytes"
	"net/http"
	"regexp"
)

var openaiRe = regexp.MustCompile(`loc=([A-Z]{2})`)

// OpenAIResult represents an OpenAI check result.
type OpenAIResult struct {
	Full   bool   // Client available (cookies + client both passed).
	Web    bool   // Web only (one check passed).
	Region string // Region code.
}

// CheckOpenAI checks ChatGPT unlock status.
// 1. cookies + client both pass -> GPT⁺-US.
// 2. only one check passes -> GPT-US.
// 3. neither passes -> no tag.
func CheckOpenAI(httpClient *http.Client) *OpenAIResult {
	result := &OpenAIResult{}

	cookiesOK := checkCookies(httpClient)
	clientOK := checkClient(httpClient)

	if cookiesOK && clientOK {
		result.Full = true
	} else if cookiesOK || clientOK {
		result.Web = true
	} else {
		return result
	}

	result.Region = getOpenAIRegion(httpClient)
	return result
}

// getOpenAIRegion extracts the region code through Cloudflare cdn-cgi/trace.
func getOpenAIRegion(httpClient *http.Client) string {
	req, err := http.NewRequest("GET", "https://chat.openai.com/cdn-cgi/trace", nil)
	if err != nil {
		return ""
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36")

	resp, err := httpClient.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	buf := getPooledBuf()
	defer putPooledBuf(buf)
	if _, err := buf.ReadFrom(resp.Body); err != nil {
		return ""
	}
	body := buf.Bytes()

	matches := openaiRe.FindSubmatch(body)
	if len(matches) > 1 {
		return string(matches[1])
	}
	return ""
}

// checkCookies checks network access through cookies.
func checkCookies(httpClient *http.Client) bool {
	req, err := http.NewRequest("GET", "https://api.openai.com/compliance/cookie_requirements", nil)
	if err != nil {
		return false
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36")
	resp, err := httpClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	buf := getPooledBuf()
	defer putPooledBuf(buf)
	if _, err := buf.ReadFrom(resp.Body); err != nil {
		return false
	}
	body := buf.Bytes()

	return !bytes.Contains(bytes.ToLower(body), []byte("unsupported_country"))
}

// checkClient checks app availability by simulating client access.
func checkClient(httpClient *http.Client) bool {
	req, err := http.NewRequest("GET", "https://ios.chat.openai.com", nil)
	if err != nil {
		return false
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (iPhone; CPU iPhone OS 16_6_0 like Mac OS X) AppleWebKit/537.36 (KHTML, like Gecko) Mobile/16G29 ChatGPT/3.0")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Requested-With", "com.openai.chatgpt")
	req.Header.Set("Referer", "https://chat.openai.com/")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Origin", "https://chat.openai.com")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	req.Header.Set("sec-ch-ua-mobile", "?1")

	resp, err := httpClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	buf := getPooledBuf()
	defer putPooledBuf(buf)
	if _, err := buf.ReadFrom(resp.Body); err != nil {
		return false
	}
	body := buf.Bytes()

	bodyLower := bytes.ToLower(body)
	return !bytes.Contains(bodyLower, []byte("unsupported_country")) && !bytes.Contains(bodyLower, []byte("vpn"))
}
