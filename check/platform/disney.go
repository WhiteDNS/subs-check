package platform

import (
	"fmt"
	"net/http"
	"strings"
)

func CheckDisney(httpClient *http.Client) (bool, error) {
	// Define constants.
	const (
		cookie    = "grant_type=urn%3Aietf%3Aparams%3Aoauth%3Agrant-type%3Atoken-exchange&latitude=0&longitude=0&platform=browser&subject_token=DISNEYASSERTION&subject_token_type=urn%3Abamtech%3Aparams%3Aoauth%3Atoken-type%3Adevice"
		assertion = `{"deviceFamily":"browser","applicationRuntime":"chrome","deviceProfile":"windows","attributes":{}}`
		authBear  = "Bearer ZGlzbmV5JmJyb3dzZXImMS4wLjA.Cu56AgSfBTDag5NiRA81oLHkDZfu5L3CKadnefEAY84"
		userAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/123.0.0.0 Safari/537.36"
	)

	// Step 1: get the assertion token.
	req, err := http.NewRequest("POST", "https://disney.api.edge.bamgrid.com/devices", strings.NewReader(assertion))
	if err != nil {
		return false, err
	}

	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Authorization", authBear)
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	var assertionResp map[string]interface{}
	if err := readJSONPooled(resp.Body, &assertionResp); err != nil {
		return false, err
	}

	assertionToken, ok := assertionResp["assertion"].(string)
	if !ok {
		return false, fmt.Errorf("failed to get assertion token")
	}

	// Step 2: get the access token.
	tokenData := strings.Replace(cookie, "DISNEYASSERTION", assertionToken, 1)
	req, err = http.NewRequest("POST", "https://disney.api.edge.bamgrid.com/token", strings.NewReader(tokenData))
	if err != nil {
		return false, err
	}

	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Authorization", authBear)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err = httpClient.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	var tokenResp map[string]interface{}
	if err := readJSONPooled(resp.Body, &tokenResp); err != nil {
		return false, err
	}

	if errDesc, ok := tokenResp["error_description"].(string); ok && errDesc == "forbidden-location" {
		return false, nil
	}

	refreshToken, ok := tokenResp["refresh_token"].(string)
	if !ok {
		return false, nil
	}

	// Step 3: check the region.
	gqlQuery := fmt.Sprintf(`{"query":"mutation refreshToken($input: RefreshTokenInput!) {refreshToken(refreshToken: $input) {activeSession {sessionId}}}","variables":{"input":{"refreshToken":"%s"}}}`, refreshToken)

	req, err = http.NewRequest("POST", "https://disney.api.edge.bamgrid.com/graph/v1/device/graphql", strings.NewReader(gqlQuery))
	if err != nil {
		return false, err
	}

	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Authorization", authBear)

	resp, err = httpClient.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	var gqlResp map[string]interface{}
	if err := readJSONPooled(resp.Body, &gqlResp); err != nil {
		return false, err
	}

	// Check region information.
	extensions, ok := gqlResp["extensions"].(map[string]interface{})
	if !ok {
		return false, nil
	}

	sdk, ok := extensions["sdk"].(map[string]interface{})
	if !ok {
		return false, nil
	}

	session, ok := sdk["session"].(map[string]interface{})
	if !ok {
		return false, nil
	}

	inSupportedLocation, _ := session["inSupportedLocation"].(bool)

	return inSupportedLocation, nil
}
