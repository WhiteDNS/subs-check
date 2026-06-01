package platform

import (
	"net/http"
	"regexp"
	"strings"

	"github.com/biter777/countries"
)

var geminiRe = regexp.MustCompile(`,2,1,200,"([A-Z]{3})"`)

// Gemini blocked region list (alpha-3 codes).
var geminiBlockedCodes = map[string]bool{
	"CHN": true, "RUS": true, "BLR": true, "CUB": true,
	"IRN": true, "PRK": true, "SYR": true, "HKG": true, "MAC": true,
}

// alpha3ToAlpha2 converts an alpha-3 code to alpha-2 using the countries library.
func alpha3ToAlpha2(alpha3 string) string {
	code := strings.ToUpper(alpha3)
	country := countries.ByName(code)
	if country == countries.Unknown {
		return ""
	}
	return country.Alpha2()
}

// CheckGemini checks Google Gemini unlock status.
// Returns an alpha-2 region code such as "US"; an empty string means unavailable.
func CheckGemini(httpClient *http.Client) (string, error) {
	req, err := http.NewRequest("GET", "https://gemini.google.com/", nil)
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

	// Extract the alpha-3 country code.
	matches := geminiRe.FindSubmatch(body)
	if len(matches) <= 1 {
		return "", nil
	}

	alpha3Code := string(matches[1])

	// Check whether it is in the blocked list.
	if geminiBlockedCodes[alpha3Code] {
		return "", nil
	}

	alpha2Code := alpha3ToAlpha2(alpha3Code)
	if alpha2Code == "" {
		return "", nil
	}
	return alpha2Code, nil
}
