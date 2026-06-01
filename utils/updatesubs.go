package utils

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"log/slog"

	"github.com/beck-8/subs-check/config"
)

// HTTPClient defines the common HTTP client interface.
type httpClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// APIResponse is the API response structure.
type versionResponse struct {
	Version string `json:"version"`
}

type providersResponse struct {
	Providers map[string]struct {
		VehicleType string `json:"vehicleType"`
	} `json:"providers"`
}

// makeRequest handles common HTTP request logic.
func makeRequest(client httpClient, method, url string) ([]byte, error) {
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", config.GlobalConfig.MihomoApiSecret))

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusNoContent {
			return nil, nil
		}
		return nil, fmt.Errorf("API returned non-200 status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	return body, nil
}

func UpdateSubs() {
	if config.GlobalConfig.MihomoApiUrl == "" {
		// slog.Warn("MihomoApiUrl is not configured; skipping update")
		return
	}

	version, err := getVersion(http.DefaultClient)
	if err != nil {
		slog.Error(fmt.Sprintf("Failed to get version: %v", err))
		return
	}

	slog.Info(fmt.Sprintf("Current Mihomo version: %s", version))

	names, err := getNeedUpdateNames(http.DefaultClient)
	if err != nil {
		slog.Error(fmt.Sprintf("Failed to get subscriptions that need updating: %v", err))
		return
	}

	if err := updateSubs(http.DefaultClient, names); err != nil {
		slog.Error(fmt.Sprintf("Failed to update subscriptions: %v", err))
		return
	}
	slog.Info("Subscription update completed")
}

func getVersion(client httpClient) (string, error) {
	url := fmt.Sprintf("%s/version", config.GlobalConfig.MihomoApiUrl)
	body, err := makeRequest(client, http.MethodGet, url)
	if err != nil {
		return "", err
	}

	var version versionResponse
	if err := json.Unmarshal(body, &version); err != nil {
		return "", fmt.Errorf("failed to parse version info: %w", err)
	}
	return version.Version, nil
}

func getNeedUpdateNames(client httpClient) ([]string, error) {
	url := fmt.Sprintf("%s/providers/proxies", config.GlobalConfig.MihomoApiUrl)
	body, err := makeRequest(client, http.MethodGet, url)
	if err != nil {
		return nil, err
	}

	var response providersResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse provider info: %w", err)
	}

	var names []string
	for name, provider := range response.Providers {
		if provider.VehicleType == "HTTP" {
			names = append(names, name)
		}
	}
	return names, nil
}

func updateSubs(client httpClient, names []string) error {
	for _, name := range names {
		url := fmt.Sprintf("%s/providers/proxies/%s", config.GlobalConfig.MihomoApiUrl, name)
		if _, err := makeRequest(client, http.MethodPut, url); err != nil {
			slog.Error(fmt.Sprintf("Failed to update subscription %v: %v", name, err))
		}
		slog.Info(fmt.Sprintf("Successfully updated subscription: %s", name))
	}
	return nil
}
